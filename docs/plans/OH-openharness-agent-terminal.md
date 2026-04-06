# OpenHarness Agent Terminal — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 将 Forge 任务模块从固定 7 步流水线重构为 OpenHarness 对话式 Agent Terminal。AI 生成的代码必须编译通过才能推送。去掉 Temporal，改用 Python HTTP API + QueryEngine 进程内循环。前端改为 Chat + 动态步骤指示器 + 代码面板，支持 Light/Dark 双主题。

**Architecture:** Python QueryEngine (OpenHarness Engine) → Redis Pub/Sub → Go SSE Handler → Next.js Agent Terminal。所有硬编码的编译命令、Dockerfile 模板、lint 规则等外部化为 Skill YAML。Worker/Reviewer Pair Pipeline 实现验证驱动的迭代循环。

**Tech Stack:** Python 3.12 + Pydantic + FastAPI + asyncio + Redis | Go 1.22 + Gin | Next.js 15 + shadcn/ui + Tailwind CSS 4

**前置条件:** Tasks 1-2 已完成（消息模型 + 工具注册表，commits d5e4802 + 2ded47c）

**设计规格:** `docs/product-design.md` §4 (Agent Terminal) + §9.2 (双主题色彩体系)

**参考:** [OpenHarness](https://github.com/HKUDS/OpenHarness) | [OpenSwarm](https://github.com/unohee/OpenSwarm)

---

## Phase A: Harness 引擎核心（Python）

> 目标: 完成 OpenHarness 10 子系统中的 5 个核心模块，让 Agent Loop 可以独立运行和测试。
> 依赖: Tasks 1-2 已完成
> 产出: QueryEngine 可以接收消息、调用工具、执行钩子、返回流式事件

### Task 1: HookManager — 生命周期钩子

**Files:**
- Create: `ai-worker/src/openharness/hooks/__init__.py`
- Create: `ai-worker/src/openharness/hooks/events.py`
- Create: `ai-worker/src/openharness/hooks/schemas.py`
- Create: `ai-worker/src/openharness/hooks/executor.py`
- Create: `ai-worker/src/openharness/hooks/loader.py`
- Test: `ai-worker/tests/test_hook_executor.py`

- [ ] **Step 1: Write failing tests**

```python
# tests/test_hook_executor.py
import pytest
from src.openharness.hooks.events import HookEvent
from src.openharness.hooks.executor import HookExecutor, HookResult, AggregatedHookResult
from src.openharness.hooks.loader import HookRegistry
from src.openharness.hooks.schemas import CommandHookDefinition

def test_hook_event_values():
    assert HookEvent.PRE_TOOL_USE == "pre_tool_use"
    assert HookEvent.POST_TOOL_USE == "post_tool_use"
    assert HookEvent.POST_GENERATION == "post_generation"

def test_hook_registry_register_and_get():
    registry = HookRegistry()
    hook = CommandHookDefinition(command="echo ok")
    registry.register(HookEvent.PRE_TOOL_USE, hook)
    hooks = registry.get(HookEvent.PRE_TOOL_USE)
    assert len(hooks) == 1

def test_hook_result_not_blocked():
    result = HookResult(hook_type="command", success=True, output="ok")
    assert not result.blocked

def test_aggregated_blocked():
    r1 = HookResult(hook_type="command", success=True, output="ok")
    r2 = HookResult(hook_type="command", success=True, output="denied",
                    blocked=True, reason="forbidden")
    agg = AggregatedHookResult(results=[r1, r2])
    assert agg.blocked
    assert agg.reason == "forbidden"

def test_aggregated_not_blocked():
    agg = AggregatedHookResult(results=[
        HookResult(hook_type="command", success=True, output="ok")
    ])
    assert not agg.blocked
```

- [ ] **Step 2: Run to verify fails**

Run: `cd ai-worker && python -m pytest tests/test_hook_executor.py -v`
Expected: FAIL with ModuleNotFoundError

- [ ] **Step 3: Implement events.py**

```python
# src/openharness/hooks/events.py
from enum import Enum

class HookEvent(str, Enum):
    PRE_TOOL_USE = "pre_tool_use"
    POST_TOOL_USE = "post_tool_use"
    PRE_GENERATION = "pre_generation"
    POST_GENERATION = "post_generation"
    POST_PUSH = "post_push"
    POST_CI = "post_ci"
```

- [ ] **Step 4: Implement schemas.py**

```python
# src/openharness/hooks/schemas.py
from __future__ import annotations
from typing import Any
from pydantic import BaseModel

class HookMatcher(BaseModel):
    tool_name: str | None = None
    agent_name: str | None = None

    def matches(self, payload: dict[str, Any]) -> bool:
        if self.tool_name and payload.get("tool_name") != self.tool_name:
            return False
        if self.agent_name and payload.get("agent_name") != self.agent_name:
            return False
        return True

class CommandHookDefinition(BaseModel):
    type: str = "command"
    command: str
    timeout_seconds: int = 30
    matcher: dict[str, str] | None = None
    block_on_failure: bool = False
```

- [ ] **Step 5: Implement loader.py**

```python
# src/openharness/hooks/loader.py
from __future__ import annotations
from .events import HookEvent
from .schemas import CommandHookDefinition

HookDefinition = CommandHookDefinition

class HookRegistry:
    def __init__(self) -> None:
        self._hooks: dict[HookEvent, list[HookDefinition]] = {}

    def register(self, event: HookEvent, hook: HookDefinition) -> None:
        self._hooks.setdefault(event, []).append(hook)

    def get(self, event: HookEvent) -> list[HookDefinition]:
        return self._hooks.get(event, [])
```

- [ ] **Step 6: Implement executor.py**

```python
# src/openharness/hooks/executor.py
from __future__ import annotations
import asyncio
import json
import os
import logging
from dataclasses import dataclass, field
from typing import Any
from .events import HookEvent
from .loader import HookRegistry
from .schemas import HookMatcher

logger = logging.getLogger(__name__)

@dataclass(frozen=True)
class HookResult:
    hook_type: str
    success: bool
    output: str
    blocked: bool = False
    reason: str = ""
    metadata: dict[str, Any] = field(default_factory=dict)

@dataclass
class AggregatedHookResult:
    results: list[HookResult]

    @property
    def blocked(self) -> bool:
        return any(r.blocked for r in self.results)

    @property
    def reason(self) -> str:
        for r in self.results:
            if r.blocked:
                return r.reason
        return ""

class HookExecutor:
    def __init__(self, registry: HookRegistry) -> None:
        self._registry = registry

    async def execute(self, event: HookEvent, payload: dict[str, Any]) -> AggregatedHookResult:
        hooks = self._registry.get(event)
        results = []
        for hook in hooks:
            if hook.matcher:
                m = HookMatcher(**hook.matcher)
                if not m.matches(payload):
                    continue
            result = await self._run_hook(hook, payload)
            results.append(result)
            if result.blocked:
                break
        return AggregatedHookResult(results=results)

    async def _run_hook(self, hook, payload: dict[str, Any]) -> HookResult:
        if hook.type == "command":
            return await self._run_command_hook(hook, payload)
        return HookResult(hook_type=hook.type, success=False, output="Unknown hook type")

    async def _run_command_hook(self, hook, payload: dict[str, Any]) -> HookResult:
        env = os.environ.copy()
        env["FORGE_HOOK_EVENT"] = str(payload.get("event", ""))
        env["FORGE_HOOK_PAYLOAD"] = json.dumps(payload, default=str)
        try:
            proc = await asyncio.create_subprocess_shell(
                hook.command,
                stdout=asyncio.subprocess.PIPE,
                stderr=asyncio.subprocess.PIPE,
                env=env,
            )
            stdout, stderr = await asyncio.wait_for(
                proc.communicate(), timeout=hook.timeout_seconds
            )
            output = stdout.decode() if stdout else ""
            success = proc.returncode == 0
            blocked = not success and hook.block_on_failure
            return HookResult(
                hook_type="command", success=success, output=output,
                blocked=blocked,
                reason=stderr.decode() if blocked else "",
            )
        except asyncio.TimeoutError:
            return HookResult(
                hook_type="command", success=False, output="",
                blocked=hook.block_on_failure, reason="Hook timed out",
            )
```

- [ ] **Step 7: Create __init__.py**

```python
# src/openharness/hooks/__init__.py
from .events import HookEvent
from .executor import HookExecutor, HookResult, AggregatedHookResult
from .loader import HookRegistry
```

- [ ] **Step 8: Run tests**

Run: `cd ai-worker && python -m pytest tests/test_hook_executor.py -v`
Expected: All PASS

- [ ] **Step 9: Commit**

```bash
git add ai-worker/src/openharness/hooks/ ai-worker/tests/test_hook_executor.py
git commit -m "feat(harness): HookManager — lifecycle hooks with command executor"
```

---

### Task 2: SkillLoader — Markdown 技能加载

**Files:**
- Create: `ai-worker/src/openharness/skills/__init__.py`
- Create: `ai-worker/src/openharness/skills/types.py`
- Create: `ai-worker/src/openharness/skills/loader.py`
- Create: `ai-worker/src/openharness/skills/registry.py`
- Create: `ai-worker/skills/analyze.md`
- Create: `ai-worker/skills/generate.md`
- Create: `ai-worker/skills/review.md`
- Create: `ai-worker/skills/plan.md`
- Create: `ai-worker/skills/test-writing.md`
- Create: `ai-worker/skills/profile.md`
- Test: `ai-worker/tests/test_skill_loader.py`

- [ ] **Step 1: Write failing tests**

```python
# tests/test_skill_loader.py
import pytest
from src.openharness.skills.types import SkillDefinition
from src.openharness.skills.loader import parse_skill_markdown
from src.openharness.skills.registry import SkillRegistry

def test_parse_skill_with_frontmatter():
    content = "---\nname: code-gen\ndescription: Generate code\npurpose: generate\ntools: [query_api_catalog]\n---\n\nYou are a code expert.\n"
    name, desc, metadata = parse_skill_markdown("fallback", content)
    assert name == "code-gen"
    assert desc == "Generate code"
    assert metadata["purpose"] == "generate"

def test_parse_skill_without_frontmatter():
    content = "# My Skill\n\nDescription here.\n"
    name, desc, metadata = parse_skill_markdown("my-skill", content)
    assert name == "my-skill"

def test_skill_registry():
    registry = SkillRegistry()
    skill = SkillDefinition(name="test", description="Test", content="# Test", source="test")
    registry.register(skill)
    assert registry.get("test") is skill
    assert registry.get("nope") is None
    assert len(registry.list_skills()) == 1
```

- [ ] **Step 2: Run to verify fails**

Run: `cd ai-worker && python -m pytest tests/test_skill_loader.py -v`

- [ ] **Step 3: Implement types.py + loader.py + registry.py**

```python
# src/openharness/skills/types.py
from dataclasses import dataclass

@dataclass(frozen=True)
class SkillDefinition:
    name: str
    description: str
    content: str
    source: str
    path: str | None = None
    metadata: dict | None = None
```

```python
# src/openharness/skills/loader.py
from __future__ import annotations
import re
from pathlib import Path
import yaml
from .types import SkillDefinition
from .registry import SkillRegistry

def parse_skill_markdown(default_name: str, content: str) -> tuple[str, str, dict]:
    metadata = {}
    body = content
    fm_match = re.match(r"^---\s*\n(.*?)\n---\s*\n", content, re.DOTALL)
    if fm_match:
        try:
            metadata = yaml.safe_load(fm_match.group(1)) or {}
        except yaml.YAMLError:
            pass
        body = content[fm_match.end():]
    name = metadata.get("name", default_name)
    description = metadata.get("description", "")
    if not description:
        h1 = re.search(r"^#\s+(.+)$", body, re.MULTILINE)
        if h1:
            description = h1.group(1).strip()
    return name, description, metadata

def load_skills_from_dir(directory: str | Path) -> list[SkillDefinition]:
    d = Path(directory)
    if not d.exists():
        return []
    skills = []
    for f in sorted(d.glob("*.md")):
        content = f.read_text(encoding="utf-8")
        name, desc, metadata = parse_skill_markdown(f.stem, content)
        skills.append(SkillDefinition(
            name=name, description=desc, content=content,
            source="file", path=str(f), metadata=metadata,
        ))
    return skills

def load_skill_registry(skills_dir: str | Path = "skills/") -> SkillRegistry:
    registry = SkillRegistry()
    for skill in load_skills_from_dir(skills_dir):
        registry.register(skill)
    return registry
```

```python
# src/openharness/skills/registry.py
from __future__ import annotations
from .types import SkillDefinition

class SkillRegistry:
    def __init__(self) -> None:
        self._skills: dict[str, SkillDefinition] = {}

    def register(self, skill: SkillDefinition) -> None:
        self._skills[skill.name] = skill

    def get(self, name: str) -> SkillDefinition | None:
        return self._skills.get(name)

    def list_skills(self) -> list[SkillDefinition]:
        return sorted(self._skills.values(), key=lambda s: s.name)
```

- [ ] **Step 4: Create 6 skill markdown files**

Extract the FULL system prompt constant from each agent file and save as markdown with YAML frontmatter.

| Skill file | Source | Lines |
|------------|--------|-------|
| `skills/analyze.md` | `ANALYST_SYSTEM_PROMPT` from `agents/analyst.py:7-118` | ~111 lines |
| `skills/generate.md` | `CODER_SYSTEM_PROMPT` from `agents/coder.py:7-113` | ~106 lines |
| `skills/plan.md` | `PLANNER_SYSTEM_PROMPT` from `agents/planner.py:7-37` | ~30 lines |
| `skills/review.md` | `REVIEWER_SYSTEM_PROMPT` from `agents/reviewer.py:7-69` | ~62 lines |
| `skills/test-writing.md` | `TEST_WRITER_SYSTEM_PROMPT` from `agents/test_writer.py:7-25` | ~18 lines |
| `skills/profile.md` | `PROFILER_SYSTEM_PROMPT` + `DIMENSION_PROMPTS` from `agents/profiler.py:7-72` | ~65 lines |

Each file format:
```markdown
---
name: requirement-analysis
description: Progressive requirement clarification
purpose: analyze
tools: []
---

[FULL SYSTEM PROMPT TEXT HERE - copy every line from the agent's constant]
```

**IMPORTANT**: Each agent's `_build_system_prompt()` adds dynamic content (language constraints, confirmed requirements, review rules, tech stack). These dynamic injections stay in the agent Python code. Only the base prompt constant moves to markdown.

- [ ] **Step 5: Run tests**

Run: `cd ai-worker && python -m pytest tests/test_skill_loader.py -v`

- [ ] **Step 6: Commit**

```bash
git add ai-worker/src/openharness/skills/ ai-worker/skills/ ai-worker/tests/test_skill_loader.py
git commit -m "feat(harness): SkillLoader + 6 agent skill markdown files"
```

---

### Task 3: PermissionChecker — 工具权限

**Files:**
- Create: `ai-worker/src/openharness/permissions/__init__.py`
- Create: `ai-worker/src/openharness/permissions/modes.py`
- Create: `ai-worker/src/openharness/permissions/checker.py`
- Test: `ai-worker/tests/test_permission_checker.py`

- [ ] **Step 1: Write failing tests**

```python
# tests/test_permission_checker.py
import pytest
from src.openharness.permissions.modes import PermissionMode
from src.openharness.permissions.checker import PermissionChecker

def test_read_only_always_allowed():
    checker = PermissionChecker(mode=PermissionMode.DEFAULT)
    assert checker.evaluate("file_read", is_read_only=True).allowed

def test_mutating_requires_confirmation():
    checker = PermissionChecker(mode=PermissionMode.DEFAULT)
    d = checker.evaluate("bash", is_read_only=False)
    assert not d.allowed
    assert d.requires_confirmation

def test_full_auto():
    checker = PermissionChecker(mode=PermissionMode.FULL_AUTO)
    assert checker.evaluate("bash", is_read_only=False).allowed

def test_denied_tool():
    checker = PermissionChecker(mode=PermissionMode.FULL_AUTO, denied_tools=["bash"])
    assert not checker.evaluate("bash", is_read_only=False).allowed
```

- [ ] **Step 2: Run to verify fails**

Run: `cd ai-worker && python -m pytest tests/test_permission_checker.py -v`

- [ ] **Step 3: Implement modes.py + checker.py**

```python
# src/openharness/permissions/modes.py
from enum import Enum

class PermissionMode(str, Enum):
    DEFAULT = "default"
    FULL_AUTO = "full_auto"
```

```python
# src/openharness/permissions/checker.py
from __future__ import annotations
from dataclasses import dataclass
from .modes import PermissionMode

@dataclass(frozen=True)
class PermissionDecision:
    allowed: bool
    requires_confirmation: bool = False
    reason: str = ""

class PermissionChecker:
    def __init__(self, mode: PermissionMode = PermissionMode.DEFAULT,
                 allowed_tools: list[str] | None = None,
                 denied_tools: list[str] | None = None) -> None:
        self._mode = mode
        self._allowed = set(allowed_tools or [])
        self._denied = set(denied_tools or [])

    def evaluate(self, tool_name: str, *, is_read_only: bool = False, **kw) -> PermissionDecision:
        if tool_name in self._denied:
            return PermissionDecision(allowed=False, reason=f"Tool '{tool_name}' denied")
        if tool_name in self._allowed:
            return PermissionDecision(allowed=True)
        if is_read_only:
            return PermissionDecision(allowed=True)
        if self._mode == PermissionMode.FULL_AUTO:
            return PermissionDecision(allowed=True)
        return PermissionDecision(allowed=False, requires_confirmation=True,
                                  reason="Mutating tool requires confirmation")
```

- [ ] **Step 4: Run tests, commit**

Run: `cd ai-worker && python -m pytest tests/test_permission_checker.py -v`

```bash
git add ai-worker/src/openharness/permissions/ ai-worker/tests/test_permission_checker.py
git commit -m "feat(harness): PermissionChecker with mode-based authorization"
```

---

### Task 4: QueryEngine + Agent Loop

**Files:**
- Create: `ai-worker/src/openharness/api/client.py`
- Create: `ai-worker/src/openharness/engine/query.py`
- Create: `ai-worker/src/openharness/engine/query_engine.py`
- Test: `ai-worker/tests/test_query_engine.py`

- [ ] **Step 1: Write failing test**

```python
# tests/test_query_engine.py
import pytest
from unittest.mock import AsyncMock, MagicMock
from src.openharness.engine.query_engine import QueryEngine
from src.openharness.engine.messages import ConversationMessage, TextBlock
from src.openharness.engine.stream_events import AssistantTextDelta, AssistantTurnComplete
from src.openharness.tools.base import ToolRegistry
from src.openharness.api.usage import UsageSnapshot

@pytest.mark.asyncio
async def test_submit_message_simple():
    mock_client = AsyncMock()
    mock_msg = ConversationMessage(role="assistant", content=[TextBlock(text="Hello!")])
    mock_usage = UsageSnapshot(input_tokens=10, output_tokens=5)

    async def mock_stream(request):
        from src.openharness.api.client import ApiTextDeltaEvent, ApiMessageCompleteEvent
        yield ApiTextDeltaEvent(text="Hello!")
        yield ApiMessageCompleteEvent(message=mock_msg, usage=mock_usage, stop_reason="end_turn")

    mock_client.stream_message = mock_stream

    engine = QueryEngine(
        api_client=mock_client, tool_registry=ToolRegistry(),
        model="test", system_prompt="You are helpful.",
    )
    events = []
    async for event in engine.submit_message("Hi"):
        events.append(event)

    assert any(isinstance(e, AssistantTextDelta) for e in events)
    assert any(isinstance(e, AssistantTurnComplete) for e in events)
    assert engine.total_usage.total_tokens == 15

def test_engine_clear():
    engine = QueryEngine(
        api_client=MagicMock(), tool_registry=ToolRegistry(),
        model="test", system_prompt="test",
    )
    engine.clear()
    assert len(engine.messages) == 0
```

- [ ] **Step 2: Run to verify fails**

Run: `cd ai-worker && python -m pytest tests/test_query_engine.py -v`

- [ ] **Step 3: Implement api/client.py (protocol + event types)**

```python
# src/openharness/api/client.py
from __future__ import annotations
from dataclasses import dataclass, field
from typing import Any, AsyncIterator, Protocol
from ..engine.messages import ConversationMessage
from .usage import UsageSnapshot

@dataclass(frozen=True)
class ApiMessageRequest:
    model: str
    messages: list[ConversationMessage]
    system_prompt: str | None = None
    max_tokens: int = 4096
    tools: list[dict[str, Any]] | None = None

@dataclass(frozen=True)
class ApiTextDeltaEvent:
    text: str

@dataclass(frozen=True)
class ApiMessageCompleteEvent:
    message: ConversationMessage
    usage: UsageSnapshot
    stop_reason: str | None = None

ApiStreamEvent = ApiTextDeltaEvent | ApiMessageCompleteEvent

class SupportsStreamingMessages(Protocol):
    async def stream_message(self, request: ApiMessageRequest) -> AsyncIterator[ApiStreamEvent]: ...
```

- [ ] **Step 4: Implement query.py (core agent loop)**

The core loop: stream API call, detect tool_use, execute tools (single sequentially, multiple concurrently via asyncio.gather), append results, continue until no tool calls or max turns.

Key behaviors:
- Pre-tool hook: `hook_executor.execute(PRE_TOOL_USE, payload)` — blocked = return error ToolResultBlock
- Tool lookup: `tool_registry.get(name)` — not found = error
- Input validation: `tool.input_model.model_validate(input)` — fail = error
- Tool execution: `tool.execute(parsed, ToolExecutionContext)` — exception = error
- Post-tool hook: `hook_executor.execute(POST_TOOL_USE, payload)`
- Events yielded: AssistantTextDelta, AssistantTurnComplete, ToolExecutionStarted, ToolExecutionCompleted, ErrorEvent
- MaxTurnsExceeded raised if loop exhausted

(See full implementation in `docs/superpowers/plans/2026-04-06-openharness-refactor.md` Task 5, Step 4)

- [ ] **Step 5: Implement query_engine.py (stateful wrapper)**

QueryEngine owns: `_messages`, `_cost_tracker`, `_api_client`, `_tool_registry`, `_hook_executor`. Methods: `submit_message(prompt)` → AsyncIterator[StreamEvent], `clear()`, `set_system_prompt()`, `set_model()`, properties: `messages`, `total_usage`.

(See full implementation in `docs/superpowers/plans/2026-04-06-openharness-refactor.md` Task 5, Step 5)

- [ ] **Step 6: Run tests, commit**

Run: `cd ai-worker && python -m pytest tests/test_query_engine.py -v`

```bash
git add ai-worker/src/openharness/api/client.py ai-worker/src/openharness/engine/query.py \
       ai-worker/src/openharness/engine/query_engine.py ai-worker/tests/test_query_engine.py
git commit -m "feat(harness): QueryEngine + Agent Loop with tool execution and hooks"
```

---

### Task 5: ModelRouterAdapter — 桥接现有路由

**Files:**
- Create: `ai-worker/src/openharness/api/providers/__init__.py`
- Create: `ai-worker/src/openharness/api/providers/router_adapter.py`
- Test: `ai-worker/tests/test_api_providers.py`

- [ ] **Step 1: Write failing test**

```python
# tests/test_api_providers.py
import pytest
from unittest.mock import AsyncMock, MagicMock, patch
from src.openharness.api.providers.router_adapter import ModelRouterAdapter
from src.openharness.api.client import ApiMessageRequest, ApiTextDeltaEvent, ApiMessageCompleteEvent
from src.openharness.engine.messages import ConversationMessage
from src.models.router import ModelRouter, Purpose

@pytest.mark.asyncio
async def test_router_adapter_yields_events():
    router = ModelRouter()
    mock_resp = MagicMock()
    mock_resp.content = "Hello"
    mock_resp.model = "test"
    mock_resp.provider = "test"
    mock_resp.input_tokens = 10
    mock_resp.output_tokens = 5
    mock_resp.latency_ms = 100
    mock_resp.stop_reason = "end_turn"
    mock_resp.tool_calls = []
    mock_resp.raw_content = None

    with patch.object(router, 'chat', return_value=mock_resp):
        adapter = ModelRouterAdapter(router, purpose=Purpose.GENERATE)
        request = ApiMessageRequest(
            model="test", messages=[ConversationMessage.from_user_text("hi")],
            system_prompt="test",
        )
        events = []
        async for event in adapter.stream_message(request):
            events.append(event)

        assert len(events) == 2
        assert isinstance(events[0], ApiTextDeltaEvent)
        assert isinstance(events[1], ApiMessageCompleteEvent)
        assert events[1].usage.total_tokens == 15
```

- [ ] **Step 2: Run to verify fails, implement, run to verify passes**

(Implementation: wrap `ModelRouter.chat()` call → yield `ApiTextDeltaEvent` + `ApiMessageCompleteEvent`. Convert messages between OpenHarness format and router format. See full code in `docs/superpowers/plans/2026-04-06-openharness-refactor.md` Task 7.)

- [ ] **Step 3: Commit**

```bash
git add ai-worker/src/openharness/api/providers/ ai-worker/tests/test_api_providers.py
git commit -m "feat(harness): ModelRouterAdapter bridges existing router to QueryEngine"
```

---

### Task 6: Harness Bootstrap + Integration Test

**Files:**
- Modify: `ai-worker/src/worker.py` — 添加 Harness 初始化
- Create: `ai-worker/tests/test_integration_harness.py`

- [ ] **Step 1: Add Harness bootstrap to worker.py**

```python
# Add after imports in worker.py:
try:
    from src.openharness.skills.loader import load_skill_registry
    from src.openharness.tools.base import ToolRegistry
    from src.openharness.hooks.loader import HookRegistry
    from src.openharness.hooks.executor import HookExecutor

    skill_registry = load_skill_registry("skills/")
    tool_registry = ToolRegistry()
    hook_registry = HookRegistry()
    hook_executor = HookExecutor(hook_registry)
    logger.info("Harness initialized: %d skills, %d tools",
                len(skill_registry.list_skills()), len(tool_registry.list_tools()))
except ImportError as e:
    logger.warning("Harness not available: %s", e)
```

- [ ] **Step 2: Write integration tests**

```python
# tests/test_integration_harness.py
import pytest
from unittest.mock import AsyncMock
from src.openharness.tools.base import BaseTool, ToolRegistry, ToolResult, ToolExecutionContext
from src.openharness.hooks.events import HookEvent
from src.openharness.hooks.loader import HookRegistry
from src.openharness.hooks.executor import HookExecutor, HookResult, AggregatedHookResult
from src.openharness.engine.messages import ConversationMessage, TextBlock, ToolResultBlock
from src.openharness.engine.query import QueryContext, _execute_tool_call
from src.openharness.api.usage import UsageSnapshot
from src.openharness.permissions.checker import PermissionChecker
from src.openharness.permissions.modes import PermissionMode
from pydantic import BaseModel
from pathlib import Path

class UpperInput(BaseModel):
    text: str

class UpperTool(BaseTool):
    name = "upper"
    description = "Uppercase text"
    input_model = UpperInput
    def is_read_only(self, arguments): return True
    async def execute(self, arguments, context):
        return ToolResult(output=arguments.text.upper())

@pytest.mark.asyncio
async def test_tool_to_engine_pipeline():
    registry = ToolRegistry()
    registry.register(UpperTool())
    result = await _execute_tool_call(
        context=QueryContext(api_client=AsyncMock(), tool_registry=registry,
                           model="test", system_prompt="test"),
        tool_name="upper", tool_use_id="t1", tool_input={"text": "hello"},
    )
    assert result.content == "HELLO"
    assert not result.is_error

@pytest.mark.asyncio
async def test_hook_blocks_tool():
    registry = ToolRegistry()
    registry.register(UpperTool())
    executor = HookExecutor(HookRegistry())
    original = executor.execute
    async def blocking(event, payload):
        if event == HookEvent.PRE_TOOL_USE:
            return AggregatedHookResult(results=[
                HookResult(hook_type="test", success=False, output="",
                          blocked=True, reason="Forbidden")])
        return AggregatedHookResult(results=[])
    executor.execute = blocking

    result = await _execute_tool_call(
        context=QueryContext(api_client=AsyncMock(), tool_registry=registry,
                           model="test", system_prompt="test", hook_executor=executor),
        tool_name="upper", tool_use_id="t1", tool_input={"text": "hello"},
    )
    assert result.is_error
    assert "BLOCKED" in result.content

@pytest.mark.asyncio
async def test_unknown_tool_error():
    result = await _execute_tool_call(
        context=QueryContext(api_client=AsyncMock(), tool_registry=ToolRegistry(),
                           model="test", system_prompt="test"),
        tool_name="nope", tool_use_id="t1", tool_input={},
    )
    assert result.is_error
    assert "Unknown" in result.content

def test_permission_integration():
    auto = PermissionChecker(mode=PermissionMode.FULL_AUTO)
    default = PermissionChecker(mode=PermissionMode.DEFAULT)
    assert auto.evaluate("bash", is_read_only=False).allowed
    assert not default.evaluate("bash", is_read_only=False).allowed
    assert default.evaluate("bash", is_read_only=False).requires_confirmation
```

- [ ] **Step 3: Run full test suite**

Run: `cd ai-worker && python -m pytest tests/ -v --tb=short`
Expected: All tests pass (existing + new)

- [ ] **Step 4: Commit**

```bash
git add ai-worker/src/worker.py ai-worker/tests/test_integration_harness.py
git commit -m "feat(harness): bootstrap in worker + integration tests for full pipeline"
```

---

## Phase B: Platform Skill 体系 + HTTP API

> 目标: 把硬编码的编译命令/Dockerfile 模板/lint 规则外部化为 Skill YAML。建立 Python HTTP API 替代 Temporal。
> 依赖: Phase A 完成
> 产出: `POST /api/run` 端点可接收消息、运行 QueryEngine、通过 Redis 推送事件

### Task 7: Platform Skills YAML

**Files:**
- Create: `forge-core/skills/detect.yaml`
- Create: `forge-core/skills/dockerfile.yaml`
- Create: `forge-core/skills/agent-loop.yaml`
- Create: `ai-worker/src/openharness/skills/project_skills.py`
- Modify: `forge-core/internal/module/profile/model.go` — 新增 skill 维度常量
- Test: `ai-worker/tests/test_project_skills.py`

(完整实现见 `docs/superpowers/plans/2026-04-06-openharness-refactor.md` Task 15)

- [ ] **Step 1: Create detect.yaml** — 技术栈检测规则 (4 语言: Go, TypeScript, Python, Java)
- [ ] **Step 2: Create dockerfile.yaml** — 4 个 Dockerfile 模板
- [ ] **Step 3: Create agent-loop.yaml** — Agent Loop 默认参数 + 按 purpose 覆盖
- [ ] **Step 4: Implement ProjectSkillLoader** — 从 profile API 读取 skill 维度
- [ ] **Step 5: Add skill dimension constants to Go model** — `KeyBuildSkill` 等 7 个新常量
- [ ] **Step 6: Write tests, commit**

```bash
git commit -m "feat(harness): Platform Skills YAML + ProjectSkillLoader"
```

---

### Task 8: BuildVerifyHook — 编译验证

**Files:**
- Create: `ai-worker/src/openharness/hooks/builtin/__init__.py`
- Create: `ai-worker/src/openharness/hooks/builtin/build_verify_hook.py`
- Test: `ai-worker/tests/test_build_verify.py`

(完整实现见 `docs/superpowers/plans/2026-04-06-openharness-refactor.md` Task 12 Step 3)

- [ ] **Step 1: Implement BuildVerifyHook** — POST_GENERATION hook, 从 build_skill 读命令, 临时目录编译, 失败返回错误日志
- [ ] **Step 2: Write tests**
- [ ] **Step 3: Commit**

```bash
git commit -m "feat(harness): BuildVerifyHook — real compilation verification"
```

---

### Task 9: Python HTTP API Server

**Files:**
- Create: `ai-worker/src/api_server.py`
- Modify: `ai-worker/requirements.txt` — 添加 fastapi, uvicorn

(完整实现见 `docs/superpowers/plans/2026-04-06-openharness-refactor.md` Task 13 Step 1)

- [ ] **Step 1: Implement FastAPI server** — `/api/run` (POST, runs QueryEngine), `/api/sessions/{id}` (DELETE), `/health` (GET)
- [ ] **Step 2: Add fastapi + uvicorn to requirements**
- [ ] **Step 3: Test locally** — `cd ai-worker && python -m src.api_server` + `curl localhost:8090/health`
- [ ] **Step 4: Commit**

```bash
git commit -m "feat(harness): FastAPI HTTP server — replaces Temporal worker"
```

---

### Task 10: Go Backend — Agent Chat Handler

**Files:**
- Create: `forge-core/internal/module/agent/model.go`
- Create: `forge-core/internal/module/agent/handler.go`
- Create: `forge-core/internal/module/agent/service.go`
- Modify: `forge-core/internal/router/router.go`

(完整实现见 `docs/superpowers/plans/2026-04-06-openharness-refactor.md` Task 9)

- [ ] **Step 1: Create model.go** — ChatRequest, AgentSession, StreamEventType
- [ ] **Step 2: Create handler.go** — POST `/projects/:id/agent/chat`, GET `/projects/:id/agent/stream` (SSE via Redis sub)
- [ ] **Step 3: Create service.go** — HTTP POST to `ai-worker:8090/api/run`
- [ ] **Step 4: Register routes in router.go**
- [ ] **Step 5: Build and test** — `cd forge-core && go build ./cmd/forge-core`
- [ ] **Step 6: Commit**

```bash
git commit -m "feat(core): agent chat API — Go handler + SSE via Redis pub/sub"
```

---

## Phase C: 前端 Agent Terminal

> 目标: 实现新版任务页面 — Chat + 动态步骤指示器 + 代码面板，支持 Light/Dark 双主题
> 依赖: Phase B 完成（需要后端 API 可用）
> 设计规格: `docs/product-design.md` §4.2 (Agent Terminal 布局) + §9.2 (双主题色彩体系)

### Task 11: 双主题色彩体系

**Files:**
- Modify: `forge-portal/app/globals.css` — CSS 变量双主题
- Create: `forge-portal/components/theme-toggle.tsx` — 主题切换按钮
- Modify: `forge-portal/app/layout.tsx` — data-theme 属性

- [ ] **Step 1: Add CSS variables for light + dark themes**

按 `docs/product-design.md` §9.2 的 token 表添加 `:root` (light) 和 `[data-theme="dark"]` 两套变量。品牌色从紫色 (#8B5CF6) 改为蓝色 (#2563eb light / #3b82f6 dark)。

- [ ] **Step 2: Create theme toggle component**
- [ ] **Step 3: Test both themes visually**
- [ ] **Step 4: Commit**

```bash
git commit -m "style: dual theme (light/dark) with blue brand color system"
```

---

### Task 12: Agent Terminal 页面

**Files:**
- Create: `forge-portal/app/(dashboard)/projects/[id]/agent/page.tsx`
- Create: `forge-portal/components/agent/agent-chat.tsx`
- Create: `forge-portal/components/agent/tool-execution.tsx`
- Create: `forge-portal/components/agent/step-ribbon.tsx`
- Create: `forge-portal/components/agent/build-card.tsx`
- Create: `forge-portal/components/agent/code-panel.tsx`
- Modify: `forge-portal/components/sidebar.tsx`

(TSX 实现见 `docs/superpowers/plans/2026-04-06-openharness-refactor.md` Task 10)

- [ ] **Step 1: Create page.tsx** — 路由 + 页面容器
- [ ] **Step 2: Create step-ribbon.tsx** — 动态步骤胶囊标签 (done/active/pending/failed 状态, cycle badge)
- [ ] **Step 3: Create agent-chat.tsx** — SSE 连接 + 消息列表 + 流式文本 + 状态栏 + 输入框
- [ ] **Step 4: Create tool-execution.tsx** — 可折叠工具调用卡片
- [ ] **Step 5: Create build-card.tsx** — 编译验证卡片 (绿色通过 / 红色失败 + 日志)
- [ ] **Step 6: Create code-panel.tsx** — 右侧代码面板 (文件 tab + 语法高亮 + diff)
- [ ] **Step 7: Add Agent nav item to sidebar**
- [ ] **Step 8: Test with dev server** — `npm run dev` → http://localhost:3000/projects/1/agent
- [ ] **Step 9: Commit**

```bash
git commit -m "feat(portal): Agent Terminal — chat + step ribbon + code panel + dual theme"
```

---

## Phase D: AI Engineering Loop

> 目标: 实现验证驱动的 Coder → Build → Review 迭代循环 + 推送后 CI 自动修复
> 依赖: Phase A-B 完成
> 来源: OpenSwarm Worker/Reviewer Pair Pipeline

### Task 13: Pair Pipeline

**Files:**
- Create: `ai-worker/src/openharness/engine/pair_pipeline.py`
- Test: `ai-worker/tests/test_pair_pipeline.py`

(实现见 `docs/superpowers/plans/2026-04-06-openharness-refactor.md` Task 16)

- [ ] **Step 1: Implement ReviewDecision enum + PairPipelineConfig + PairPipelineResult**
- [ ] **Step 2: Implement run_pair_pipeline()** — Coder → BuildVerify → Review → loop
- [ ] **Step 3: Write tests**
- [ ] **Step 4: Commit**

```bash
git commit -m "feat(harness): Pair Pipeline — Coder/Reviewer iteration with build verify"
```

---

### Task 14: CI Auto-fix Hook

**Files:**
- Create: `ai-worker/src/openharness/hooks/builtin/ci_autofix_hook.py`

(实现见 `docs/superpowers/plans/2026-04-06-openharness-refactor.md` Task 17)

- [ ] **Step 1: Implement CIAutoFixHook** — POST_PUSH hook, poll CI status, fetch logs, generate fix, push
- [ ] **Step 2: Commit**

```bash
git commit -m "feat(harness): CI auto-fix hook — monitor CI + repair failures"
```

---

## Verification

### End-to-End Smoke Test

1. `docker compose -f docker-compose.dev.yml up -d` (PostgreSQL, Redis)
2. `cd forge-core && go run ./cmd/forge-core`
3. `cd ai-worker && python -m src.api_server`
4. `cd forge-portal && npm run dev`
5. 创建项目 → 自动检测 build_skill
6. 进入 Agent Terminal → 发消息
7. 验证: 流式文本 + 工具卡片 + 编译验证卡片
8. 验证: 编译失败时 AI 自动修复并重试
9. 验证: Light/Dark 主题切换正常

### Unit Test Coverage

```bash
cd ai-worker && python -m pytest tests/ -v --cov=src/openharness --cov-report=term-missing
```

Target: 80%+ coverage on `src/openharness/` modules.

### Phase 完成标准

| Phase | 完成标志 |
|-------|---------|
| A | `python -m pytest tests/` 全部通过, QueryEngine 可以 submit_message + 收到 StreamEvent |
| B | `curl localhost:8090/api/run` 返回 Agent 响应, Go SSE 端点可推送事件 |
| C | 浏览器 `localhost:3000/projects/1/agent` 可以发消息并看到流式回复 |
| D | 编译失败时自动修复循环可见 (Build Failed → AI Fix → Build Pass) |
