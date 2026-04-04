# SH-1 — Harness 基座层（ContextCache + Agent Loop + ModelRouter tools）

## 目标

将 AI Worker 从"单次 LLM 调用"升级为"多轮工具调用引擎"：

1. **ContextCache** — Redis 缓存项目上下文，同一 Workflow 内多个 Activity 共享，避免重复 API 调用
2. **Agent Loop** — BaseAgent.run() 从 single-shot 变为 iterative tool-use loop（最多 5 轮）
3. **ModelRouter tools** — chat()/chat_stream() 支持 tools 参数，处理 tool_use stop_reason

完成后，Forge AI 引擎具备 Harness Engineering 的核心能力基座：按需拉取上下文 + 多轮推理 + 工具调用。

## 前置依赖

- 无外部依赖，仅修改 ai-worker 内部文件
- Redis 已部署（docker-compose.dev.yml，端口 6379，密码 forge_redis_2026）
- 现有 6 个 Activity 正常运行（analyze, plan, test_writing, generate, review, profile）

## 工期

3 天

---

## Day 1 — ContextCache（Redis 缓存 + 并行上下文构建）

### 1.1 新建 `ai-worker/src/context/cache.py`

```python
"""Redis-backed context cache for Temporal workflows.

Key format: ctx:{workflow_id}
TTL: 30 minutes (matches typical workflow duration)
Serialization: JSON via dataclasses_json or manual dict conversion
"""
from __future__ import annotations

import json
import logging
import time
from typing import Optional

import redis.asyncio as aioredis

from src.config import settings
from src.context.builder import ContextBuilder, ProjectContext

logger = logging.getLogger(__name__)

# Cache TTL: 30 minutes
CACHE_TTL_SECONDS = 30 * 60


def _cache_key(workflow_id: str) -> str:
    return f"ctx:{workflow_id}"


def _serialize_context(ctx: ProjectContext) -> str:
    """Serialize ProjectContext to JSON string."""
    return json.dumps({
        "project_name": ctx.project_name,
        "project_description": ctx.project_description,
        "tech_stack": ctx.tech_stack,
        "coding_standards": ctx.coding_standards,
        "review_rules": ctx.review_rules,
        "prompt_template_system": ctx.prompt_template_system,
        "prompt_template_user": ctx.prompt_template_user,
        "project_profiles": ctx.project_profiles,
        # conversation_history is NOT cached — it changes per activity
    }, ensure_ascii=False)


def _deserialize_context(data: str) -> ProjectContext:
    """Deserialize JSON string to ProjectContext."""
    d = json.loads(data)
    return ProjectContext(
        project_name=d.get("project_name", ""),
        project_description=d.get("project_description", ""),
        tech_stack=d.get("tech_stack", {}),
        coding_standards=d.get("coding_standards", []),
        review_rules=d.get("review_rules", []),
        prompt_template_system=d.get("prompt_template_system", ""),
        prompt_template_user=d.get("prompt_template_user", ""),
        project_profiles=d.get("project_profiles", {}),
    )


class ContextCache:
    """Redis-backed cache for ProjectContext, scoped to a Temporal workflow."""

    def __init__(self) -> None:
        self._redis: Optional[aioredis.Redis] = None
        self._builder = ContextBuilder()

    async def _get_redis(self) -> Optional[aioredis.Redis]:
        if self._redis is None:
            try:
                self._redis = aioredis.from_url(
                    f"redis://:{settings.redis_password}@{settings.redis_host}:{settings.redis_port}",
                    decode_responses=True,
                )
            except Exception as e:
                logger.warning("Redis unavailable for context cache: %s", e)
        return self._redis

    async def get_or_build(
        self,
        workflow_id: str,
        project_id: int,
        purpose: str,
        conversation_history: list[dict] | None = None,
    ) -> ProjectContext:
        """Return cached context or build fresh. conversation_history is never cached."""
        start = time.monotonic()

        # Try cache first
        redis = await self._get_redis()
        if redis and workflow_id:
            try:
                cached = await redis.get(_cache_key(workflow_id))
                if cached:
                    ctx = _deserialize_context(cached)
                    ctx.conversation_history = conversation_history or []
                    elapsed = int((time.monotonic() - start) * 1000)
                    logger.info(
                        "context cache HIT: workflow=%s latency=%dms",
                        workflow_id, elapsed,
                    )
                    return ctx
            except Exception as e:
                logger.warning("context cache read failed: %s", e)

        # Cache miss — build from APIs (parallel)
        ctx = await self._builder.build_parallel(
            project_id=project_id,
            purpose=purpose,
            conversation_history=conversation_history,
        )

        # Store in cache
        if redis and workflow_id:
            try:
                await redis.set(
                    _cache_key(workflow_id),
                    _serialize_context(ctx),
                    ex=CACHE_TTL_SECONDS,
                )
                logger.info("context cache SET: workflow=%s", workflow_id)
            except Exception as e:
                logger.warning("context cache write failed: %s", e)

        elapsed = int((time.monotonic() - start) * 1000)
        logger.info(
            "context cache MISS: workflow=%s latency=%dms",
            workflow_id, elapsed,
        )
        return ctx

    async def invalidate(self, workflow_id: str) -> None:
        """Explicitly invalidate cached context (e.g., after profile scan)."""
        redis = await self._get_redis()
        if redis:
            try:
                await redis.delete(_cache_key(workflow_id))
            except Exception:
                pass

    async def close(self) -> None:
        await self._builder.close()
        if self._redis:
            await self._redis.aclose()
```

### 1.2 修改 `ai-worker/src/context/builder.py` — 添加 `build_parallel()` 方法

在 `ContextBuilder` 类中添加新方法，使用 `asyncio.gather` 并行拉取 4 个 API：

```python
# 在 ContextBuilder 类内部，build() 方法之后新增：

async def build_parallel(
    self,
    project_id: int,
    purpose: str,
    conversation_history: list[dict] | None = None,
) -> ProjectContext:
    """Build context with parallel API calls (4x faster than serial)."""
    import asyncio

    ctx = ProjectContext()
    ctx.conversation_history = conversation_history or []

    # Define all fetchers
    async def fetch_project():
        try:
            resp = await self._client.get(f"/api/projects/{project_id}")
            if resp.status_code == 200:
                data = resp.json().get("data", {})
                ctx.project_name = data.get("name", "")
                ctx.project_description = data.get("description", "")
                ctx.tech_stack = data.get("techStack") or {}
        except Exception as e:
            logger.warning(f"Failed to fetch project {project_id}: {e}")

    async def fetch_specs():
        try:
            resp = await self._client.get(f"/api/specs/effective/{project_id}")
            if resp.status_code == 200:
                data = resp.json().get("data", {})
                standards = data.get("standards", [])
                ctx.coding_standards = [
                    s.get("content", "") for s in standards if s.get("content")
                ]
                ctx.review_rules = data.get("rules", [])
        except Exception as e:
            logger.warning(f"Failed to fetch specs for project {project_id}: {e}")

    async def fetch_prompts():
        try:
            resp = await self._client.get(
                "/api/specs/prompts", params={"purpose": purpose}
            )
            if resp.status_code == 200:
                data = resp.json().get("data", {})
                items = data.get("items", [])
                template = None
                for item in items:
                    if item.get("isDefault"):
                        template = item
                        break
                if not template and items:
                    template = items[0]
                if template:
                    ctx.prompt_template_system = template.get("systemPrompt", "")
                    ctx.prompt_template_user = template.get("userTemplate", "")
        except Exception as e:
            logger.warning(f"Failed to fetch prompt template for {purpose}: {e}")

    async def fetch_profiles():
        try:
            resp = await self._client.get(f"/api/projects/{project_id}/profiles")
            if resp.status_code == 200:
                data = resp.json().get("data", {})
                profiles_list = data.get("profiles", [])
                for p in profiles_list:
                    key = p.get("profileKey", "")
                    value = p.get("profileValue", {})
                    if key and value:
                        ctx.project_profiles[key] = value
        except Exception as e:
            logger.warning(f"Failed to fetch project profiles for {project_id}: {e}")

    # Fire all 4 in parallel
    await asyncio.gather(
        fetch_project(),
        fetch_specs(),
        fetch_prompts(),
        fetch_profiles(),
        return_exceptions=True,
    )

    return ctx
```

原有的 `build()` 方法保持不变（向后兼容）。`build_parallel()` 是新的推荐入口。

### 1.3 修改所有 6 个 Activity — 添加 `workflow_id` 参数，使用 ContextCache

每个 Activity 的 Input dataclass 添加可选字段 `workflow_id`：

**`ai-worker/src/activities/analyze.py`** — 修改 AnalyzeInput 和 activity 函数：

```python
@dataclass
class AnalyzeInput:
    project_id: int
    task_id: int
    requirement: str
    conversation_history: Optional[List[Dict[str, Any]]] = None
    workflow_id: Optional[str] = None  # NEW: for context caching
```

修改 `analyze_requirement_activity`：

```python
@activity.defn(name="analyze_requirement")
async def analyze_requirement_activity(input: AnalyzeInput) -> AnalyzeOutput:
    logger.info(f"Analyzing requirement for task {input.task_id}")
    cache = ContextCache()
    try:
        ctx = await cache.get_or_build(
            workflow_id=input.workflow_id or "",
            project_id=input.project_id,
            purpose="requirement-analysis",
            conversation_history=input.conversation_history,
        )
        # ... rest unchanged ...
    finally:
        await cache.close()
```

**同样的模式应用到其他 5 个 Activity 文件：**

| 文件 | Input 类 | 添加字段 |
|------|----------|----------|
| `activities/plan.py` | PlanInput | `workflow_id: Optional[str] = None` |
| `activities/test_writing.py` | TestWritingInput | `workflow_id: Optional[str] = None` |
| `activities/generate.py` | GenerateInput | `workflow_id: Optional[str] = None` |
| `activities/review.py` | ReviewInput | `workflow_id: Optional[str] = None` |
| `activities/profile.py` | ScanProfileInput | `workflow_id: Optional[str] = None` |

每个 activity 函数体内：将 `ContextBuilder()` + `builder.build()` 替换为 `ContextCache()` + `cache.get_or_build()`，`finally` 块改为 `await cache.close()`。

### 1.4 修改 Go workflow — 传递 workflow_id

修改 `forge-core/internal/temporal/workflow/task_workflow.go` 中所有 AI activity 调用，在 map 中添加 `"workflow_id"` 字段：

```go
// 在 TaskWorkflow 函数开头获取 workflow ID
workflowID := workflow.GetInfo(ctx).WorkflowExecution.ID

// 每个 AI activity 调用都加上 workflow_id，例如：
err = workflow.ExecuteActivity(aiCtx, "plan_task", map[string]interface{}{
    "task_id":             input.TaskID,
    "tenant_id":           input.TenantID,
    "project_id":          input.ProjectID,
    "requirement_summary": input.Requirement,
    "workflow_id":         workflowID,  // NEW
}).Get(ctx, &planResult)
```

同样修改 `TaskExecutionWorkflow`、`PlanOnlyWorkflow`、`AnalyzeRequirementWorkflow` 中的所有 AI activity map。

---

## Day 2 — Agent Loop（多轮工具调用循环）

### 2.1 修改 `ai-worker/src/agents/base.py` — AgentResult 增加字段

```python
@dataclass
class AgentResult:
    content: str              # Raw text response
    structured: dict          # Parsed JSON data
    tokens_used: int
    model: str
    provider: str
    latency_ms: int
    tool_calls_count: int = 0  # NEW: number of tool calls executed
    parse_failed: bool = False # NEW: JSON parse failed flag
```

### 2.2 修改 `ai-worker/src/agents/base.py` — BaseAgent.run() 增加 tool-use loop

```python
import asyncio
import hashlib
from src.models.client import LLMResponse

class BaseAgent:
    purpose: Purpose = Purpose.ANALYZE
    tools: list[dict] | None = None          # NEW: subclass sets this
    max_tool_rounds: int = 5                 # NEW: max iterations
    tool_timeout_seconds: float = 10.0       # NEW: per-tool timeout

    def __init__(self, router: ModelRouter) -> None:
        self.router = router

    def _build_system_prompt(self, context: ProjectContext) -> str:
        return context.to_system_prompt()

    def _build_messages(self, user_input: str, context: ProjectContext) -> list[dict]:
        messages = []
        for msg in context.conversation_history:
            messages.append(
                {"role": msg.get("role", "user"), "content": msg.get("content", "")}
            )
        messages.append({"role": "user", "content": user_input})
        return messages

    def _get_tool_executor(self) -> "ContextToolExecutor | None":
        """Subclasses override to provide a tool executor. Default: None."""
        return None

    async def run(
        self,
        user_input: str,
        context: ProjectContext,
        project_id: int = 0,
    ) -> AgentResult:
        """Run agent with optional iterative tool-use loop.

        When self.tools is None, behaves identically to the original
        single-shot implementation (backward compatible).
        When self.tools is set, enters a loop:
          1. Call LLM with tools
          2. If LLM returns tool_use, execute tools, append results, repeat
          3. If LLM returns text (end_turn), exit loop
          4. Max `max_tool_rounds` iterations
        """
        system = self._build_system_prompt(context)
        messages = self._build_messages(user_input, context)

        # --- Fast path: no tools, single-shot (backward compatible) ---
        if not self.tools:
            response: LLMResponse = await self.router.chat(
                system=system, messages=messages, purpose=self.purpose
            )
            structured = self._parse_json(response.content)
            parse_failed = (structured == {} and response.content.strip() != "{}")
            return AgentResult(
                content=response.content,
                structured=structured,
                tokens_used=response.input_tokens + response.output_tokens,
                model=response.model,
                provider=response.provider,
                latency_ms=response.latency_ms,
                parse_failed=parse_failed,
            )

        # --- Tool-use loop ---
        executor = self._get_tool_executor()
        total_tokens = 0
        total_latency = 0
        tool_calls_count = 0
        last_model = ""
        last_provider = ""
        dedup_cache: set[str] = set()  # hash of (tool_name, args) to skip repeats

        for round_idx in range(self.max_tool_rounds):
            response: LLMResponse = await self.router.chat(
                system=system,
                messages=messages,
                purpose=self.purpose,
                tools=self.tools,
            )
            total_tokens += response.input_tokens + response.output_tokens
            total_latency += response.latency_ms
            last_model = response.model
            last_provider = response.provider

            # Check if response contains tool calls
            if not response.tool_calls:
                # No tool calls — final text response
                structured = self._parse_json(response.content)
                parse_failed = (structured == {} and response.content.strip() != "{}")
                return AgentResult(
                    content=response.content,
                    structured=structured,
                    tokens_used=total_tokens,
                    model=last_model,
                    provider=last_provider,
                    latency_ms=total_latency,
                    tool_calls_count=tool_calls_count,
                    parse_failed=parse_failed,
                )

            # Process tool calls
            # Append assistant message with tool_use blocks
            messages.append(response.to_assistant_message())

            tool_results = []
            for tc in response.tool_calls:
                # Dedup: skip if same tool+args already called
                call_hash = hashlib.md5(
                    f"{tc.name}:{json.dumps(tc.arguments, sort_keys=True)}".encode()
                ).hexdigest()
                if call_hash in dedup_cache:
                    logger.info("skipping duplicate tool call: %s", tc.name)
                    tool_results.append({
                        "type": "tool_result",
                        "tool_use_id": tc.id,
                        "content": "(duplicate call skipped)",
                    })
                    continue
                dedup_cache.add(call_hash)

                # Execute with timeout
                if executor:
                    try:
                        result_str = await asyncio.wait_for(
                            executor.execute(tc, context, project_id),
                            timeout=self.tool_timeout_seconds,
                        )
                    except asyncio.TimeoutError:
                        result_str = f"Tool call timed out after {self.tool_timeout_seconds}s"
                        logger.warning("tool timeout: %s", tc.name)
                    except Exception as e:
                        result_str = f"Tool execution error: {e}"
                        logger.warning("tool error: %s %s", tc.name, e)
                else:
                    result_str = "No tool executor configured"

                tool_results.append({
                    "type": "tool_result",
                    "tool_use_id": tc.id,
                    "content": result_str,
                })
                tool_calls_count += 1

            # Append tool results as user message
            messages.append({"role": "user", "content": tool_results})

            # Token budget check
            if total_tokens > 150_000:
                logger.warning(
                    "token budget exceeded (%d), forcing final response",
                    total_tokens,
                )
                break

        # Exhausted rounds — force a final non-tool call
        logger.warning("max tool rounds (%d) reached, making final call without tools", self.max_tool_rounds)
        response = await self.router.chat(
            system=system, messages=messages, purpose=self.purpose
        )
        total_tokens += response.input_tokens + response.output_tokens
        total_latency += response.latency_ms
        structured = self._parse_json(response.content)
        parse_failed = (structured == {} and response.content.strip() != "{}")
        return AgentResult(
            content=response.content,
            structured=structured,
            tokens_used=total_tokens,
            model=response.model,
            provider=response.provider,
            latency_ms=total_latency,
            tool_calls_count=tool_calls_count,
            parse_failed=parse_failed,
        )

    def _parse_json(self, text: str) -> dict:
        # ... existing implementation unchanged ...
```

### 2.3 修改 `ai-worker/src/models/client.py` — LLMResponse 增加 tool_calls 支持

```python
@dataclass
class ToolCall:
    """A tool call from the LLM response."""
    id: str
    name: str
    arguments: dict


@dataclass
class LLMResponse:
    content: str
    model: str
    provider: str
    input_tokens: int
    output_tokens: int
    latency_ms: int
    tool_calls: list[ToolCall] | None = None  # NEW
    stop_reason: str = ""                      # NEW: "end_turn" | "tool_use"

    def to_assistant_message(self) -> dict:
        """Convert to assistant message format for conversation history.

        For Anthropic-style: returns content blocks (text + tool_use).
        For OpenAI-style: returns message with tool_calls.
        """
        if not self.tool_calls:
            return {"role": "assistant", "content": self.content}

        # Anthropic format: content blocks
        blocks = []
        if self.content:
            blocks.append({"type": "text", "text": self.content})
        for tc in self.tool_calls:
            blocks.append({
                "type": "tool_use",
                "id": tc.id,
                "name": tc.name,
                "input": tc.arguments,
            })
        return {"role": "assistant", "content": blocks}
```

---

## Day 3 — ModelRouter tools 支持

### 3.1 修改 `ai-worker/src/models/router.py` — chat() 添加 tools 参数

```python
async def chat(
    self,
    system: str,
    messages: list[dict[str, Any]],
    purpose: Purpose = Purpose.GENERATE,
    tools: list[dict] | None = None,  # NEW
) -> LLMResponse:
    """Route a chat request through the fallback chain for the given purpose."""
    chain = ROUTING_RULES[purpose]
    errors: list[str] = []

    for provider, model in chain:
        api_key = self._get_api_key(provider)
        if not api_key:
            logger.debug("Skipping %s: no API key configured", provider)
            continue

        breaker_key = f"{provider}:{model}"
        breaker = self._get_breaker(breaker_key)
        if not breaker.is_available():
            logger.debug("Skipping %s/%s: circuit breaker open", provider, model)
            continue

        # Skip providers that don't support tools
        if tools and provider not in _TOOLS_CAPABLE_PROVIDERS:
            logger.debug("Skipping %s: tools not supported", provider)
            continue

        try:
            caller = PROVIDER_CALLERS[provider]
            call_kwargs: dict[str, Any] = {}
            if purpose == Purpose.ANALYZE and provider in ("dashscope", "openai", "deepseek"):
                call_kwargs["response_format"] = {"type": "json_object"}
            if tools:
                call_kwargs["tools"] = tools
            response = await caller(api_key, model, system, messages, **call_kwargs)
            breaker.record_success()
            logger.info(
                "LLM call succeeded: provider=%s model=%s latency=%dms tools=%s",
                provider, model, response.latency_ms, bool(tools),
            )
            return response
        except Exception as exc:
            breaker.record_failure()
            error_msg = f"{provider}/{model}: {exc}"
            errors.append(error_msg)
            logger.warning("LLM call failed: %s", error_msg)

    raise RuntimeError(
        f"All models failed for purpose={purpose.value}: {'; '.join(errors)}"
    )

# Providers known to support tools
_TOOLS_CAPABLE_PROVIDERS = {"anthropic", "openai", "dashscope", "deepseek"}
```

同样修改 `chat_stream()` 添加 `tools` 参数（streaming + tools 暂不实现，传 tools 时 fallback 到非流式）。

### 3.2 修改 `ai-worker/src/models/client.py` — 所有 provider caller 支持 tools

**Anthropic (`call_anthropic`)**:

```python
async def call_anthropic(
    api_key: str,
    model: str,
    system: str,
    messages: list[dict[str, Any]],
    tools: list[dict] | None = None,  # NEW
    **kwargs: Any,
) -> LLMResponse:
    """Call Anthropic Claude API with optional tools."""
    client = anthropic.AsyncAnthropic(api_key=api_key)
    start = time.monotonic()

    create_kwargs: dict[str, Any] = {
        "model": model,
        "max_tokens": MAX_TOKENS,
        "system": system,
        "messages": messages,
    }
    if tools:
        # Convert to Anthropic tool format
        create_kwargs["tools"] = _to_anthropic_tools(tools)

    response = await client.messages.create(**create_kwargs)
    latency_ms = int((time.monotonic() - start) * 1000)

    # Extract content and tool_calls
    content = ""
    tool_calls = []
    for block in response.content:
        if block.type == "text":
            content += block.text
        elif block.type == "tool_use":
            tool_calls.append(ToolCall(
                id=block.id,
                name=block.name,
                arguments=block.input if isinstance(block.input, dict) else {},
            ))

    return LLMResponse(
        content=content,
        model=response.model,
        provider="anthropic",
        input_tokens=response.usage.input_tokens,
        output_tokens=response.usage.output_tokens,
        latency_ms=latency_ms,
        tool_calls=tool_calls if tool_calls else None,
        stop_reason=response.stop_reason or "",
    )


def _to_anthropic_tools(tools: list[dict]) -> list[dict]:
    """Convert generic tool format to Anthropic's tool format."""
    result = []
    for t in tools:
        result.append({
            "name": t["name"],
            "description": t.get("description", ""),
            "input_schema": t.get("parameters", t.get("input_schema", {"type": "object", "properties": {}})),
        })
    return result
```

**OpenAI-compatible (`_call_openai_compatible`)**:

```python
async def _call_openai_compatible(
    api_key: str,
    model: str,
    system: str,
    messages: list[dict[str, Any]],
    provider: str,
    base_url: Optional[str] = None,
    response_format: Optional[Dict[str, Any]] = None,
    tools: list[dict] | None = None,  # NEW
) -> LLMResponse:
    """Shared implementation for OpenAI-compatible APIs."""
    kwargs: dict[str, Any] = {"api_key": api_key}
    if base_url:
        kwargs["base_url"] = base_url
    client = openai.AsyncOpenAI(**kwargs)

    full_messages = [{"role": "system", "content": system}] + messages
    create_kwargs: dict[str, Any] = {
        "model": model,
        "max_tokens": MAX_TOKENS,
        "messages": full_messages,
    }
    if response_format:
        create_kwargs["response_format"] = response_format
    if tools:
        create_kwargs["tools"] = _to_openai_tools(tools)

    start = time.monotonic()
    response = await client.chat.completions.create(**create_kwargs)
    latency_ms = int((time.monotonic() - start) * 1000)
    choice = response.choices[0]
    usage = response.usage

    # Extract tool calls from OpenAI format
    tool_calls = None
    if choice.message.tool_calls:
        tool_calls = []
        for tc in choice.message.tool_calls:
            try:
                args = json.loads(tc.function.arguments)
            except (json.JSONDecodeError, TypeError):
                args = {}
            tool_calls.append(ToolCall(
                id=tc.id,
                name=tc.function.name,
                arguments=args,
            ))

    return LLMResponse(
        content=choice.message.content or "",
        model=response.model,
        provider=provider,
        input_tokens=usage.prompt_tokens if usage else 0,
        output_tokens=usage.completion_tokens if usage else 0,
        latency_ms=latency_ms,
        tool_calls=tool_calls,
        stop_reason=choice.finish_reason or "",
    )


def _to_openai_tools(tools: list[dict]) -> list[dict]:
    """Convert generic tool format to OpenAI's function calling format."""
    result = []
    for t in tools:
        result.append({
            "type": "function",
            "function": {
                "name": t["name"],
                "description": t.get("description", ""),
                "parameters": t.get("parameters", t.get("input_schema", {"type": "object", "properties": {}})),
            },
        })
    return result
```

**更新各 provider 的签名转发 tools**:

```python
async def call_openai(
    api_key: str, model: str, system: str, messages: list[dict[str, Any]],
    response_format=None, tools=None,  # ADD tools
) -> LLMResponse:
    return await _call_openai_compatible(
        api_key, model, system, messages, "openai",
        response_format=response_format, tools=tools,
    )

async def call_dashscope(
    api_key: str, model: str, system: str, messages: list[dict[str, Any]],
    response_format=None, tools=None,  # ADD tools
) -> LLMResponse:
    return await _call_openai_compatible(
        api_key, model, system, messages, "dashscope",
        base_url="https://dashscope.aliyuncs.com/compatible-mode/v1",
        response_format=response_format, tools=tools,
    )

async def call_deepseek(
    api_key: str, model: str, system: str, messages: list[dict[str, Any]],
    response_format=None, tools=None,  # ADD tools
) -> LLMResponse:
    return await _call_openai_compatible(
        api_key, model, system, messages, "deepseek",
        base_url="https://api.deepseek.com",
        response_format=response_format, tools=tools,
    )
```

---

## 数据结构

### ContextCache Redis Key

```
Key:    ctx:{workflow_id}
Value:  JSON string of ProjectContext (excluding conversation_history)
TTL:    1800 seconds (30 minutes)
```

### ToolCall dataclass

```python
@dataclass
class ToolCall:
    id: str           # Unique ID from provider (e.g., "toolu_01abc...")
    name: str         # Tool function name
    arguments: dict   # Parsed arguments
```

### AgentResult (updated)

```python
@dataclass
class AgentResult:
    content: str
    structured: dict
    tokens_used: int
    model: str
    provider: str
    latency_ms: int
    tool_calls_count: int = 0
    parse_failed: bool = False
```

---

## 验收标准

1. **ContextCache**
   - 同一 workflow_id 第二次调用 `get_or_build()` 命中缓存，延迟 < 5ms（vs 首次 200-500ms）
   - Redis 不可用时 graceful fallback 到直接 build，无异常抛出
   - TTL 30 分钟后自动过期

2. **Agent Loop**
   - `tools=None` 时行为与改动前完全一致（现有 6 个 agent 无影响）
   - `tools` 非空时，LLM 返回 tool_use 触发循环，最终返回文本
   - 重复 tool call 被 dedup 跳过
   - 单次 tool 执行超过 10s 返回 timeout 错误，不阻塞循环
   - 超过 5 轮后强制 final call（无 tools），避免无限循环
   - token 超过 150k 提前退出

3. **ModelRouter tools**
   - Anthropic provider: tools 原生传递，tool_use 响应正确解析为 ToolCall 列表
   - OpenAI/DashScope/DeepSeek: tools 转为 function calling 格式，tool_calls 正确解析
   - 不支持 tools 的 provider 自动跳过，尝试链中下一个
   - `chat_stream()` 传 tools 时 fallback 到非流式（暂不支持 streaming + tools）

---

## 质量验证

### 测试用例

```python
# tests/test_context_cache.py

import pytest
from unittest.mock import AsyncMock, patch

@pytest.mark.asyncio
async def test_cache_hit():
    """Second call with same workflow_id should return cached context."""
    cache = ContextCache()
    # First call — cache miss, builds from APIs
    ctx1 = await cache.get_or_build("wf-1", project_id=1, purpose="test")
    # Second call — cache hit
    ctx2 = await cache.get_or_build("wf-1", project_id=1, purpose="test")
    assert ctx2.project_name == ctx1.project_name
    await cache.close()

@pytest.mark.asyncio
async def test_cache_miss_redis_down():
    """When Redis is unavailable, should fallback to builder."""
    with patch("src.context.cache.aioredis") as mock_redis:
        mock_redis.from_url.side_effect = ConnectionError("refused")
        cache = ContextCache()
        ctx = await cache.get_or_build("wf-2", project_id=1, purpose="test")
        assert isinstance(ctx, ProjectContext)
        await cache.close()

@pytest.mark.asyncio
async def test_conversation_history_not_cached():
    """conversation_history should not be in cache, always from input."""
    cache = ContextCache()
    ctx1 = await cache.get_or_build("wf-3", 1, "test", conversation_history=[{"role": "user", "content": "hi"}])
    ctx2 = await cache.get_or_build("wf-3", 1, "test", conversation_history=[{"role": "user", "content": "bye"}])
    assert ctx2.conversation_history == [{"role": "user", "content": "bye"}]
    await cache.close()


# tests/test_agent_loop.py

@pytest.mark.asyncio
async def test_agent_no_tools_backward_compatible():
    """Agent with tools=None should behave like original single-shot."""
    router = MockRouter(response=LLMResponse(content='{"status": "ok"}', ...))
    agent = BaseAgent(router)
    result = await agent.run("test input", ProjectContext())
    assert result.structured == {"status": "ok"}
    assert result.tool_calls_count == 0

@pytest.mark.asyncio
async def test_agent_tool_loop():
    """Agent with tools should loop until LLM returns text."""
    # Mock router returns tool_use first, then text
    router = MockRouter(responses=[
        LLMResponse(content="", tool_calls=[ToolCall(id="1", name="read_file", arguments={"path": "main.go"})], ...),
        LLMResponse(content='{"files": []}', tool_calls=None, ...),
    ])
    agent = BaseAgent(router)
    agent.tools = [{"name": "read_file", ...}]
    result = await agent.run("generate code", ProjectContext())
    assert result.tool_calls_count == 1
    assert result.structured == {"files": []}

@pytest.mark.asyncio
async def test_agent_dedup_tool_calls():
    """Duplicate tool calls should be skipped."""
    router = MockRouter(responses=[
        LLMResponse(content="", tool_calls=[
            ToolCall(id="1", name="read_file", arguments={"path": "a.go"}),
            ToolCall(id="2", name="read_file", arguments={"path": "a.go"}),  # dup
        ], ...),
        LLMResponse(content='{"ok": true}', tool_calls=None, ...),
    ])
    agent = BaseAgent(router)
    agent.tools = [{"name": "read_file", ...}]
    result = await agent.run("test", ProjectContext())
    assert result.tool_calls_count == 1  # only 1 actual execution

@pytest.mark.asyncio
async def test_agent_max_rounds():
    """Agent should exit after max_tool_rounds."""
    # Router always returns tool_use
    router = MockRouter(always_tool_use=True, final_response=LLMResponse(content='{}', ...))
    agent = BaseAgent(router)
    agent.tools = [{"name": "read_file", ...}]
    agent.max_tool_rounds = 3
    result = await agent.run("test", ProjectContext())
    # Should have exited after 3 rounds + 1 final call
    assert result.tool_calls_count <= 3


# tests/test_model_router_tools.py

@pytest.mark.asyncio
async def test_anthropic_tools_passthrough():
    """Tools should be passed to Anthropic in native format."""
    tools = [{"name": "query_db", "description": "Query DB schema", "parameters": {"type": "object", "properties": {}}}]
    with patch("src.models.client.anthropic.AsyncAnthropic") as mock:
        mock_instance = mock.return_value
        mock_instance.messages.create = AsyncMock(return_value=MockAnthropicResponse())
        response = await call_anthropic("key", "model", "sys", [], tools=tools)
        # Verify tools were passed
        call_args = mock_instance.messages.create.call_args
        assert "tools" in call_args.kwargs

@pytest.mark.asyncio
async def test_openai_tools_format():
    """Tools should be converted to OpenAI function calling format."""
    tools = [{"name": "query_db", "description": "Query DB", "parameters": {"type": "object", "properties": {}}}]
    converted = _to_openai_tools(tools)
    assert converted[0]["type"] == "function"
    assert converted[0]["function"]["name"] == "query_db"
```

### 回归验证

运行现有任务（无 tools）确保行为完全不变：

```bash
cd ai-worker && python -m pytest tests/ -v
```

验证 ContextCache 性能提升：

```bash
# 在同一 workflow 中连续调用 plan → test_writing → generate
# 预期：plan 耗时 ~300ms（cache miss），test_writing/generate 耗时 < 10ms（cache hit）
```
