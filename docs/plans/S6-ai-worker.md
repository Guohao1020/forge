# S6 — AI Worker + 需求对话 + 端到端代码生成

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 接入 Python AI Worker，替换 S4 的骨架 Workflow，实现从自然语言需求到 AI 分析、规划、代码生成、Review 的完整闭环。用户可以在"需求对话"页面输入需求，AI 多轮对话后生成确认卡片，确认后 Temporal Workflow 驱动真正的 AI 代码生成。

**Architecture:** ai-worker (Python 3.12 + LangGraph) 注册 Temporal Activities。forge-core (Go) 发起 Workflow，通过 Temporal 调用 Python Activities。forge-portal (Next.js) 提供对话界面和实时流式输出。

**Tech Stack:** Python 3.12 + LangGraph + Temporal SDK, Claude claude-sonnet-4-20250514 (primary) + GPT-4o (fallback), SSE streaming, Redis pub/sub (stream relay)

**Prior slices dependency:** S1 (auth), S2 (project), S3 (GitHub OAuth), S4 (Temporal + task CRUD), S5 (specs center)

---

## 前置说明

### 本切片交付后你可以做什么

1. 进入某个项目，点击"新建需求"进入需求对话页面
2. 输入自然语言需求（如"创建一个用户管理 REST API"）
3. AI 分析需求，提出澄清问题（多轮对话）
4. AI 生成需求确认卡片（摘要 + 任务拆解 + 工时评估）
5. 点击"确认"，真正的 Temporal Workflow 开始执行
6. 实时看到 AI 分析、规划、生成代码的流式输出
7. AI 生成代码并提交到 GitHub 仓库的特性分支
8. 任务详情页显示真实步骤进度和 AI 输出内容

### 与 S4 的关系

S4 创建了骨架 Temporal Workflow（Go，mock 延迟）。本切片：
- **替换** S4 的 mock Workflow 为真正调用 Python AI Activities 的 Workflow
- **新增** conversations 表和对话 API
- **新增** ai-worker Python 服务
- **保留** S4 的 task CRUD API 和 kanban 页面

### Phase 1 简化策略

为了快速实现端到端闭环，Phase 1 做以下简化：
- **非真正 token 流式**：Activity 返回完整响应，Go 端以 SSE 推送（未来改 Redis pub/sub 真流式）
- **单文件生成**：AI 一次生成一个文件的代码（未来支持多文件并行）
- **简化 Review**：AI 自检代码质量，不调用外部 Lint 工具（S7+ 接入 constraint-worker）
- **GitHub 提交**：通过 GitHub API 直接创建 commit（不做本地 clone）
- **成本仅记录**：model_calls 表记录每次 AI 调用的 token/费用/延迟，但不做预算限制（Phase 2 启用）
- **无风险两阶段评估**：Phase 1 用简单规则（文件数 + 变更类型）单次打标，不做规划后初评 + Review 后终评

---

## 文件结构

### ai-worker（Python AI Worker — 新建）

```
ai-worker/
├── pyproject.toml                     # 项目元数据 + 依赖
├── requirements.txt                   # pip 依赖（同步自 pyproject.toml）
├── .env.example                       # 环境变量模板
├── src/
│   ├── __init__.py
│   ├── worker.py                      # Temporal Worker 入口
│   ├── config.py                      # 配置加载
│   ├── workflows/
│   │   ├── __init__.py
│   │   └── task_workflow.py           # TaskWorkflow 编排（LangGraph state machine）
│   ├── activities/
│   │   ├── __init__.py
│   │   ├── analyze.py                 # 需求分析 Activity
│   │   ├── plan.py                    # 任务规划 Activity
│   │   ├── generate.py                # 代码生成 Activity
│   │   └── review.py                  # AI Review Activity
│   ├── agents/
│   │   ├── __init__.py
│   │   ├── analyst.py                 # 分析师 Agent（需求理解 + 澄清问题）
│   │   ├── planner.py                 # 规划师 Agent（任务拆解 + 评估）
│   │   ├── coder.py                   # 编码 Agent（代码生成）
│   │   └── reviewer.py               # 审查 Agent（代码审查）
│   ├── models/
│   │   ├── __init__.py
│   │   ├── router.py                  # 多模型路由（Claude → GPT 降级链）
│   │   └── client.py                  # LLM Client 封装
│   └── context/
│       ├── __init__.py
│       └── builder.py                 # Context Builder（规范 + 项目画像 + 代码）
```

### forge-core（Go — 修改）

```
forge-core/
├── internal/
│   ├── module/
│   │   └── task/
│   │       ├── handler.go             # 修改：新增对话 API 路由
│   │       ├── conversation_handler.go # 新增：对话 Handler
│   │       ├── conversation_service.go # 新增：对话 Service
│   │       ├── conversation_repo.go    # 新增：对话 Repository
│   │       ├── model.go               # 修改：新增 Conversation model
│   │       ├── service.go             # 修改：confirm 触发真实 Workflow
│   │       └── workflow.go            # 修改：替换 mock 为真实 Workflow
│   └── router/
│       └── router.go                  # 修改：注册对话路由
├── migrations/
│   └── 006_conversations.sql          # 新增：conversations 表 + model_calls 表
```

### forge-portal（Next.js — 修改/新增）

```
forge-portal/
├── app/
│   └── (dashboard)/
│       └── projects/
│           └── [id]/
│               ├── tasks/
│               │   ├── new/
│               │   │   └── page.tsx   # 新增：需求对话页面
│               │   └── [taskId]/
│               │       └── page.tsx   # 修改：实时步骤 + AI 输出
│               └── layout.tsx         # 可能修改：侧边栏加入"新建需求"
├── components/
│   ├── chat/
│   │   ├── chat-panel.tsx             # 新增：对话面板
│   │   ├── message-bubble.tsx         # 新增：消息气泡
│   │   ├── confirmation-card.tsx      # 新增：需求确认卡片
│   │   └── thinking-indicator.tsx     # 新增：AI 思考中动画
│   └── task/
│       ├── step-timeline.tsx          # 新增：真实步骤时间线
│       └── step-output.tsx            # 新增：步骤 AI 输出展示
├── lib/
│   ├── api/
│   │   └── conversation.ts            # 新增：对话 API 客户端
│   └── hooks/
│       └── use-sse.ts                 # 新增：SSE hook
```

---

## Task 1: Python 项目骨架 + 依赖配置

**Files:**
- Create: `ai-worker/pyproject.toml`
- Create: `ai-worker/requirements.txt`
- Create: `ai-worker/src/__init__.py`
- Create: `ai-worker/src/config.py`
- Create: `ai-worker/.env.example`
- Create: `ai-worker/.gitignore`

- [ ] **Step 1: 创建 pyproject.toml**

`ai-worker/pyproject.toml`:

```toml
[project]
name = "forge-ai-worker"
version = "0.1.0"
description = "Forge AI Worker — LangGraph + Temporal Activities"
requires-python = ">=3.12"
dependencies = [
    "temporalio>=1.7.0",
    "langgraph>=0.2.0",
    "langchain-core>=0.3.0",
    "langchain-anthropic>=0.3.0",
    "langchain-openai>=0.3.0",
    "httpx>=0.27.0",
    "pydantic>=2.9.0",
    "pydantic-settings>=2.6.0",
    "redis>=5.2.0",
    "structlog>=24.4.0",
]

[project.optional-dependencies]
dev = [
    "pytest>=8.3.0",
    "pytest-asyncio>=0.24.0",
    "ruff>=0.8.0",
]

[tool.ruff]
target-version = "py312"
line-length = 120

[tool.pytest.ini_options]
asyncio_mode = "auto"
```

- [ ] **Step 2: 创建 requirements.txt**

`ai-worker/requirements.txt`:

```txt
temporalio>=1.7.0
langgraph>=0.2.0
langchain-core>=0.3.0
langchain-anthropic>=0.3.0
langchain-openai>=0.3.0
httpx>=0.27.0
pydantic>=2.9.0
pydantic-settings>=2.6.0
redis>=5.2.0
structlog>=24.4.0
```

- [ ] **Step 3: 创建 .env.example**

`ai-worker/.env.example`:

```bash
# Temporal
TEMPORAL_HOST=localhost:7233
TEMPORAL_NAMESPACE=default
TEMPORAL_TASK_QUEUE=ai-worker

# LLM Providers
ANTHROPIC_API_KEY=sk-ant-xxx
OPENAI_API_KEY=sk-xxx

# Model Configuration
PRIMARY_MODEL=claude-sonnet-4-20250514
FALLBACK_MODEL=gpt-4o

# Forge Core API (for fetching specs, project profile)
FORGE_CORE_URL=http://localhost:8080

# Redis (for stream relay)
REDIS_URL=redis://:forge_redis_2026@localhost:6379/0

# Logging
LOG_LEVEL=INFO
```

- [ ] **Step 4: 创建 config.py**

`ai-worker/src/config.py`:

```python
from pydantic_settings import BaseSettings


class Settings(BaseSettings):
    """AI Worker configuration — loaded from environment variables."""

    # Temporal
    temporal_host: str = "localhost:7233"
    temporal_namespace: str = "default"
    temporal_task_queue: str = "ai-worker"

    # LLM Providers
    anthropic_api_key: str = ""
    openai_api_key: str = ""

    # Model Configuration
    primary_model: str = "claude-sonnet-4-20250514"
    fallback_model: str = "gpt-4o"
    max_tokens: int = 4096

    # Forge Core API
    forge_core_url: str = "http://localhost:8080"

    # Redis
    redis_url: str = "redis://:forge_redis_2026@localhost:6379/0"

    # Logging
    log_level: str = "INFO"

    model_config = {"env_file": ".env", "env_file_encoding": "utf-8"}


settings = Settings()
```

- [ ] **Step 5: 创建 __init__.py 和 .gitignore**

`ai-worker/src/__init__.py`:
```python
```

`ai-worker/.gitignore`:
```
__pycache__/
*.pyc
.env
.venv/
venv/
dist/
*.egg-info/
.ruff_cache/
.pytest_cache/
```

- [ ] **Step 6: 验证 Python 环境**

```bash
cd ai-worker
python -m venv .venv
source .venv/bin/activate  # Windows: .venv\Scripts\activate
pip install -r requirements.txt
python -c "from src.config import settings; print(f'Config loaded: temporal={settings.temporal_host}')"
```

预期输出：`Config loaded: temporal=localhost:7233`

- [ ] **Step 7: Commit**

```bash
git add ai-worker/
git commit -m "feat(s6): scaffold Python AI Worker project with dependencies and config"
```

---

## Task 2: 多模型路由 + LLM Client

**Files:**
- Create: `ai-worker/src/models/__init__.py`
- Create: `ai-worker/src/models/client.py`
- Create: `ai-worker/src/models/router.py`

- [ ] **Step 1: 创建 LLM Client 封装**

`ai-worker/src/models/__init__.py`:
```python
```

`ai-worker/src/models/client.py`:

封装 Anthropic 和 OpenAI 的统一接口。Phase 1 使用 langchain 的 ChatModel 抽象。

```python
from __future__ import annotations

import structlog
from langchain_anthropic import ChatAnthropic
from langchain_openai import ChatOpenAI
from langchain_core.language_models import BaseChatModel

from src.config import settings

logger = structlog.get_logger()


def create_anthropic_client() -> BaseChatModel:
    """Create Claude client via langchain-anthropic."""
    return ChatAnthropic(
        model=settings.primary_model,
        api_key=settings.anthropic_api_key,
        max_tokens=settings.max_tokens,
        temperature=0.3,
    )


def create_openai_client() -> BaseChatModel:
    """Create GPT client via langchain-openai."""
    return ChatOpenAI(
        model=settings.fallback_model,
        api_key=settings.openai_api_key,
        max_tokens=settings.max_tokens,
        temperature=0.3,
    )
```

- [ ] **Step 2: 创建多模型路由器**

`ai-worker/src/models/router.py`:

根据 purpose 选择最佳模型，失败自动降级。

```python
from __future__ import annotations

from enum import Enum
from typing import Sequence

import structlog
from langchain_core.language_models import BaseChatModel
from langchain_core.messages import BaseMessage

from src.config import settings
from src.models.client import create_anthropic_client, create_openai_client

logger = structlog.get_logger()


class Purpose(str, Enum):
    """AI task purpose — determines model selection priority."""
    ANALYZE = "analyze"
    PLAN = "plan"
    GENERATE = "generate"
    REVIEW = "review"


class ModelRouter:
    """Multi-model router with automatic fallback.

    Primary: Claude (best for code generation + analysis)
    Fallback: GPT-4o (reliable backup)
    """

    def __init__(self) -> None:
        self._clients: dict[str, BaseChatModel] = {}
        self._init_clients()

    def _init_clients(self) -> None:
        """Initialize available model clients based on configured API keys."""
        if settings.anthropic_api_key:
            try:
                self._clients["anthropic"] = create_anthropic_client()
                logger.info("model_client_ready", provider="anthropic", model=settings.primary_model)
            except Exception as e:
                logger.warning("model_client_init_failed", provider="anthropic", error=str(e))

        if settings.openai_api_key:
            try:
                self._clients["openai"] = create_openai_client()
                logger.info("model_client_ready", provider="openai", model=settings.fallback_model)
            except Exception as e:
                logger.warning("model_client_init_failed", provider="openai", error=str(e))

        if not self._clients:
            raise RuntimeError("No LLM providers configured. Set ANTHROPIC_API_KEY or OPENAI_API_KEY.")

    def _get_provider_order(self, purpose: Purpose) -> list[str]:
        """Get provider priority order based on purpose.

        All purposes prefer Claude first, GPT-4o as fallback.
        Future: could customize per purpose (e.g., prefer GPT for certain tasks).
        """
        return ["anthropic", "openai"]

    async def chat(
        self,
        messages: Sequence[BaseMessage],
        purpose: Purpose,
        max_tokens: int | None = None,
    ) -> str:
        """Route chat request to the best available model with fallback.

        Args:
            messages: LangChain message sequence
            purpose: Task purpose (analyze/plan/generate/review)
            max_tokens: Override max tokens (None = use default)

        Returns:
            Model response text

        Raises:
            RuntimeError: If all providers fail
        """
        provider_order = self._get_provider_order(purpose)
        last_error: Exception | None = None

        for provider in provider_order:
            client = self._clients.get(provider)
            if client is None:
                continue

            try:
                logger.info("model_request", provider=provider, purpose=purpose.value, msg_count=len(messages))

                kwargs = {}
                if max_tokens is not None:
                    kwargs["max_tokens"] = max_tokens

                response = await client.ainvoke(messages, **kwargs)
                content = response.content if isinstance(response.content, str) else str(response.content)

                logger.info(
                    "model_response",
                    provider=provider,
                    purpose=purpose.value,
                    response_len=len(content),
                )
                return content

            except Exception as e:
                last_error = e
                logger.warning(
                    "model_request_failed",
                    provider=provider,
                    purpose=purpose.value,
                    error=str(e),
                )
                continue

        raise RuntimeError(f"All model providers failed. Last error: {last_error}")


# Module-level singleton
_router: ModelRouter | None = None


def get_router() -> ModelRouter:
    """Get or create the global ModelRouter singleton."""
    global _router
    if _router is None:
        _router = ModelRouter()
    return _router
```

- [ ] **Step 3: 验证模型路由**

```bash
cd ai-worker
source .venv/bin/activate
python -c "
from src.models.router import ModelRouter, Purpose
print('ModelRouter imported successfully')
print('Purposes:', [p.value for p in Purpose])
"
```

预期：成功导入，列出 4 个 purpose。（实际调用需要 API key，开发时手动测试）

- [ ] **Step 4: Commit**

```bash
git add ai-worker/src/models/
git commit -m "feat(s6): add multi-model router with Claude/GPT fallback chain"
```

---

## Task 3: Context Builder

**Files:**
- Create: `ai-worker/src/context/__init__.py`
- Create: `ai-worker/src/context/builder.py`

- [ ] **Step 1: 创建 Context Builder**

`ai-worker/src/context/__init__.py`:
```python
```

`ai-worker/src/context/builder.py`:

Context Builder 从 forge-core API 拉取项目画像和规范，组装 AI 的完整上下文窗口。

```python
from __future__ import annotations

from dataclasses import dataclass, field

import httpx
import structlog

from src.config import settings

logger = structlog.get_logger()

# Token budget: reserve tokens for output, use rest for context
MAX_CONTEXT_TOKENS = 180_000  # Claude claude-sonnet-4-20250514 has 200k context
RESERVED_OUTPUT_TOKENS = 8_000
MAX_INPUT_TOKENS = MAX_CONTEXT_TOKENS - RESERVED_OUTPUT_TOKENS


@dataclass
class ProjectContext:
    """Assembled context for AI to work with."""
    project_name: str = ""
    project_description: str = ""
    tech_stack: str = ""
    coding_standards: list[str] = field(default_factory=list)
    prompt_template: str = ""
    relevant_files: dict[str, str] = field(default_factory=dict)  # filename -> content
    conversation_history: list[dict] = field(default_factory=list)

    def to_system_prompt(self) -> str:
        """Assemble into a single system prompt string."""
        parts: list[str] = []

        parts.append("# Project Context")
        if self.project_name:
            parts.append(f"**Project:** {self.project_name}")
        if self.project_description:
            parts.append(f"**Description:** {self.project_description}")
        if self.tech_stack:
            parts.append(f"**Tech Stack:** {self.tech_stack}")

        if self.coding_standards:
            parts.append("\n# Coding Standards")
            for std in self.coding_standards:
                parts.append(f"- {std}")

        if self.prompt_template:
            parts.append(f"\n# Instructions\n{self.prompt_template}")

        if self.relevant_files:
            parts.append("\n# Relevant Code Files")
            for filename, content in self.relevant_files.items():
                parts.append(f"\n## {filename}\n```\n{content}\n```")

        return "\n".join(parts)


class ContextBuilder:
    """Builds AI context by fetching project profile and specs from forge-core."""

    def __init__(self) -> None:
        self._http = httpx.AsyncClient(
            base_url=settings.forge_core_url,
            timeout=30.0,
        )

    async def build(
        self,
        project_id: int,
        purpose: str,
        requirement: str = "",
        conversation_history: list[dict] | None = None,
    ) -> ProjectContext:
        """Build context for a specific project and purpose.

        Args:
            project_id: Forge project ID
            purpose: analyze/plan/generate/review
            requirement: The user requirement text
            conversation_history: Prior conversation messages

        Returns:
            Assembled ProjectContext
        """
        ctx = ProjectContext()
        ctx.conversation_history = conversation_history or []

        # Fetch project profile
        await self._load_project(ctx, project_id)

        # Fetch coding standards from specs center
        await self._load_standards(ctx, project_id)

        # Load purpose-specific prompt template
        await self._load_prompt_template(ctx, project_id, purpose)

        # TODO: In future, load relevant code files from GitHub
        # await self._load_relevant_files(ctx, project_id, requirement)

        logger.info(
            "context_built",
            project_id=project_id,
            purpose=purpose,
            standards_count=len(ctx.coding_standards),
            system_prompt_len=len(ctx.to_system_prompt()),
        )

        return ctx

    async def _load_project(self, ctx: ProjectContext, project_id: int) -> None:
        """Fetch project profile from forge-core."""
        try:
            resp = await self._http.get(f"/api/projects/{project_id}")
            if resp.status_code == 200:
                data = resp.json().get("data", {})
                ctx.project_name = data.get("name", "")
                ctx.project_description = data.get("description", "")
                profile = data.get("profile", {})
                if isinstance(profile, dict):
                    ctx.tech_stack = profile.get("techStack", "")
        except Exception as e:
            logger.warning("load_project_failed", project_id=project_id, error=str(e))

    async def _load_standards(self, ctx: ProjectContext, project_id: int) -> None:
        """Fetch effective coding standards from specs center."""
        try:
            resp = await self._http.get(f"/api/projects/{project_id}/specs/standards")
            if resp.status_code == 200:
                data = resp.json().get("data", [])
                for std in data:
                    if std.get("status") == "ACTIVE":
                        ctx.coding_standards.append(
                            f"[{std.get('category', 'general')}] {std.get('name', '')}: {std.get('content', '')}"
                        )
        except Exception as e:
            logger.warning("load_standards_failed", project_id=project_id, error=str(e))

    async def _load_prompt_template(self, ctx: ProjectContext, project_id: int, purpose: str) -> None:
        """Fetch prompt template for the given purpose from specs center."""
        try:
            resp = await self._http.get(
                f"/api/projects/{project_id}/specs/prompts",
                params={"purpose": purpose},
            )
            if resp.status_code == 200:
                data = resp.json().get("data", [])
                if data:
                    # Use the first matching template
                    ctx.prompt_template = data[0].get("content", "")
        except Exception as e:
            logger.warning("load_prompt_template_failed", project_id=project_id, error=str(e))

    async def close(self) -> None:
        """Close HTTP client."""
        await self._http.aclose()
```

- [ ] **Step 2: 验证**

```bash
cd ai-worker
source .venv/bin/activate
python -c "
from src.context.builder import ContextBuilder, ProjectContext
ctx = ProjectContext(project_name='test', tech_stack='Java + Spring Boot')
print(ctx.to_system_prompt()[:200])
print('ContextBuilder imported OK')
"
```

- [ ] **Step 3: Commit**

```bash
git add ai-worker/src/context/
git commit -m "feat(s6): add context builder for assembling AI context from specs and project profile"
```

---

## Task 4: AI Agents（分析/规划/生成/审查）

**Files:**
- Create: `ai-worker/src/agents/__init__.py`
- Create: `ai-worker/src/agents/analyst.py`
- Create: `ai-worker/src/agents/planner.py`
- Create: `ai-worker/src/agents/coder.py`
- Create: `ai-worker/src/agents/reviewer.py`

- [ ] **Step 1: 创建 Agent __init__**

`ai-worker/src/agents/__init__.py`:
```python
```

- [ ] **Step 2: 创建分析师 Agent**

`ai-worker/src/agents/analyst.py`:

分析师负责理解需求、提出澄清问题、生成需求确认卡片。

```python
from __future__ import annotations

import json

import structlog
from langchain_core.messages import SystemMessage, HumanMessage

from src.models.router import get_router, Purpose
from src.context.builder import ProjectContext

logger = structlog.get_logger()

ANALYST_SYSTEM_PROMPT = """You are a senior requirement analyst for the Forge platform.
Your job is to understand user requirements and produce a structured analysis.

## Your responsibilities:
1. Understand the user's natural language requirement
2. Identify ambiguities and ask clarifying questions (if needed)
3. Produce a structured requirement summary

## Output format:
When you have enough information, respond with a JSON block:

```json
{
  "status": "clarify" | "confirmed",
  "questions": ["question1", "question2"],  // only if status=clarify
  "summary": "Brief summary of the requirement",
  "scope": "What this requirement covers",
  "assumptions": ["assumption1", "assumption2"],
  "technical_hints": ["hint1", "hint2"]
}
```

If the requirement is clear enough to proceed, set status to "confirmed".
If you need more information, set status to "clarify" and list your questions.

Always respond in the same language as the user's input.
Keep your analysis concise and actionable."""


async def analyze_requirement(
    requirement: str,
    project_context: ProjectContext,
    conversation_history: list[dict] | None = None,
) -> dict:
    """Analyze a user requirement and return structured analysis.

    Args:
        requirement: User's natural language requirement
        project_context: Assembled project context
        conversation_history: Previous messages in the conversation

    Returns:
        Dict with status, summary, questions, etc.
    """
    router = get_router()

    messages = [
        SystemMessage(content=ANALYST_SYSTEM_PROMPT + "\n\n" + project_context.to_system_prompt()),
    ]

    # Add conversation history
    if conversation_history:
        for msg in conversation_history:
            if msg["role"] == "user":
                messages.append(HumanMessage(content=msg["content"]))
            elif msg["role"] == "assistant":
                from langchain_core.messages import AIMessage
                messages.append(AIMessage(content=msg["content"]))

    # Add current requirement
    messages.append(HumanMessage(content=requirement))

    response = await router.chat(messages, Purpose.ANALYZE)

    # Try to parse structured JSON from response
    result = _parse_analysis_response(response)
    result["raw_response"] = response

    logger.info(
        "requirement_analyzed",
        status=result.get("status", "unknown"),
        has_questions=bool(result.get("questions")),
    )

    return result


def _parse_analysis_response(response: str) -> dict:
    """Extract structured JSON from the analyst's response."""
    # Try to find JSON block in response
    try:
        # Look for ```json ... ``` block
        if "```json" in response:
            start = response.index("```json") + 7
            end = response.index("```", start)
            json_str = response[start:end].strip()
            return json.loads(json_str)
        # Try parsing the whole response as JSON
        return json.loads(response)
    except (json.JSONDecodeError, ValueError):
        # If no structured output, treat as a clarification response
        return {
            "status": "clarify",
            "questions": [],
            "summary": response,
        }
```

- [ ] **Step 3: 创建规划师 Agent**

`ai-worker/src/agents/planner.py`:

规划师负责将分析结果拆解为可执行的任务计划。

```python
from __future__ import annotations

import json

import structlog
from langchain_core.messages import SystemMessage, HumanMessage

from src.models.router import get_router, Purpose
from src.context.builder import ProjectContext

logger = structlog.get_logger()

PLANNER_SYSTEM_PROMPT = """You are a senior software architect and project planner.
Given a requirement analysis, you create an actionable implementation plan.

## Your responsibilities:
1. Break down the requirement into specific implementation tasks
2. Estimate effort for each task
3. Identify files to create or modify
4. Define the implementation order

## Output format:
Respond with a JSON block:

```json
{
  "title": "Short task title",
  "summary": "Brief description of what will be implemented",
  "tasks": [
    {
      "order": 1,
      "title": "Task title",
      "description": "What this task does",
      "files": ["path/to/file1.java", "path/to/file2.java"],
      "estimate_hours": 2,
      "type": "create" | "modify"
    }
  ],
  "total_estimate_hours": 8,
  "risk_level": "LOW" | "MEDIUM" | "HIGH",
  "risk_factors": ["factor1", "factor2"],
  "branch_name": "feature/xxx"
}
```

Always respond in the same language as the input.
Be specific about file paths based on the project's tech stack."""


async def plan_task(
    analysis: dict,
    requirement: str,
    project_context: ProjectContext,
) -> dict:
    """Create an implementation plan from requirement analysis.

    Args:
        analysis: Structured analysis from analyst agent
        requirement: Original user requirement
        project_context: Assembled project context

    Returns:
        Dict with title, tasks, estimates, risk level, etc.
    """
    router = get_router()

    messages = [
        SystemMessage(content=PLANNER_SYSTEM_PROMPT + "\n\n" + project_context.to_system_prompt()),
        HumanMessage(content=f"""## Requirement
{requirement}

## Analysis
{json.dumps(analysis, ensure_ascii=False, indent=2)}

Please create a detailed implementation plan."""),
    ]

    response = await router.chat(messages, Purpose.PLAN)

    result = _parse_plan_response(response)
    result["raw_response"] = response

    logger.info(
        "task_planned",
        task_count=len(result.get("tasks", [])),
        total_hours=result.get("total_estimate_hours", 0),
        risk_level=result.get("risk_level", "UNKNOWN"),
    )

    return result


def _parse_plan_response(response: str) -> dict:
    """Extract structured JSON from the planner's response."""
    try:
        if "```json" in response:
            start = response.index("```json") + 7
            end = response.index("```", start)
            json_str = response[start:end].strip()
            return json.loads(json_str)
        return json.loads(response)
    except (json.JSONDecodeError, ValueError):
        return {
            "title": "Implementation Plan",
            "summary": response[:500],
            "tasks": [],
            "total_estimate_hours": 0,
            "risk_level": "MEDIUM",
        }
```

- [ ] **Step 4: 创建编码 Agent**

`ai-worker/src/agents/coder.py`:

编码 Agent 根据计划生成代码。Phase 1 一次生成一个文件。

```python
from __future__ import annotations

import json

import structlog
from langchain_core.messages import SystemMessage, HumanMessage

from src.models.router import get_router, Purpose
from src.context.builder import ProjectContext

logger = structlog.get_logger()

CODER_SYSTEM_PROMPT = """You are a senior software engineer. You write clean, production-ready code.

## Rules:
1. Follow all coding standards provided in the project context
2. Write complete, compilable code — no TODOs or placeholders
3. Include proper error handling
4. Add concise comments for complex logic
5. Follow the project's existing patterns and conventions

## Output format:
For each file, respond with:

```json
{
  "files": [
    {
      "path": "relative/path/to/file.ext",
      "content": "full file content here",
      "action": "create" | "modify",
      "description": "What this file does"
    }
  ],
  "commit_message": "feat: brief description of changes"
}
```

Generate ALL files needed for the task. Each file must be complete and functional."""


async def generate_code(
    plan: dict,
    requirement: str,
    project_context: ProjectContext,
    task_index: int | None = None,
) -> dict:
    """Generate code based on the implementation plan.

    Args:
        plan: Implementation plan from planner agent
        requirement: Original user requirement
        project_context: Assembled project context
        task_index: If set, only generate code for this specific subtask

    Returns:
        Dict with files list and commit message
    """
    router = get_router()

    # Build task-specific prompt
    task_detail = ""
    if task_index is not None and plan.get("tasks"):
        if 0 <= task_index < len(plan["tasks"]):
            task = plan["tasks"][task_index]
            task_detail = f"""
## Current Task (#{task.get('order', task_index + 1)})
**Title:** {task.get('title', '')}
**Description:** {task.get('description', '')}
**Files:** {', '.join(task.get('files', []))}
**Type:** {task.get('type', 'create')}
"""
    else:
        # Generate all at once
        task_detail = f"""
## Full Implementation Plan
{json.dumps(plan.get('tasks', []), ensure_ascii=False, indent=2)}
"""

    messages = [
        SystemMessage(content=CODER_SYSTEM_PROMPT + "\n\n" + project_context.to_system_prompt()),
        HumanMessage(content=f"""## Requirement
{requirement}

## Plan Summary
{plan.get('summary', '')}

{task_detail}

Generate the complete code for the specified task(s)."""),
    ]

    response = await router.chat(messages, Purpose.GENERATE, max_tokens=8192)

    result = _parse_code_response(response)
    result["raw_response"] = response

    logger.info(
        "code_generated",
        file_count=len(result.get("files", [])),
    )

    return result


def _parse_code_response(response: str) -> dict:
    """Extract structured JSON from the coder's response."""
    try:
        if "```json" in response:
            start = response.index("```json") + 7
            end = response.index("```", start)
            json_str = response[start:end].strip()
            return json.loads(json_str)
        return json.loads(response)
    except (json.JSONDecodeError, ValueError):
        # Fallback: try to extract code blocks
        return {
            "files": [],
            "commit_message": "feat: generated code",
            "raw_text": response,
        }
```

- [ ] **Step 5: 创建审查 Agent**

`ai-worker/src/agents/reviewer.py`:

```python
from __future__ import annotations

import json

import structlog
from langchain_core.messages import SystemMessage, HumanMessage

from src.models.router import get_router, Purpose
from src.context.builder import ProjectContext

logger = structlog.get_logger()

REVIEWER_SYSTEM_PROMPT = """You are a senior code reviewer. You review AI-generated code for quality and correctness.

## Review checklist:
1. **Correctness**: Does the code implement the requirement correctly?
2. **Completeness**: Are all edge cases handled? Any missing files?
3. **Standards compliance**: Does it follow the project's coding standards?
4. **Security**: Any security vulnerabilities (SQL injection, XSS, etc.)?
5. **Error handling**: Are errors properly caught and handled?
6. **Performance**: Any obvious performance issues?

## Output format:
```json
{
  "passed": true | false,
  "score": 85,
  "summary": "Brief review summary",
  "findings": [
    {
      "severity": "critical" | "major" | "minor" | "info",
      "file": "path/to/file.ext",
      "line": 42,
      "message": "Description of the issue",
      "suggestion": "How to fix it"
    }
  ],
  "fix_instructions": "If not passed, describe what needs to be fixed"
}
```

Be strict but fair. Score 0-100. Pass threshold is 70."""


async def review_code(
    generated_code: dict,
    plan: dict,
    requirement: str,
    project_context: ProjectContext,
) -> dict:
    """Review AI-generated code.

    Args:
        generated_code: Code generation result from coder agent
        plan: Implementation plan
        requirement: Original requirement
        project_context: Project context with coding standards

    Returns:
        Dict with passed, score, findings, etc.
    """
    router = get_router()

    # Build code summary for review
    files_summary = ""
    for f in generated_code.get("files", []):
        files_summary += f"\n### {f.get('path', 'unknown')}\n```\n{f.get('content', '')[:3000]}\n```\n"

    if not files_summary:
        files_summary = generated_code.get("raw_text", generated_code.get("raw_response", "No code generated"))[:5000]

    messages = [
        SystemMessage(content=REVIEWER_SYSTEM_PROMPT + "\n\n" + project_context.to_system_prompt()),
        HumanMessage(content=f"""## Requirement
{requirement}

## Plan
{plan.get('summary', '')}

## Generated Code
{files_summary}

Please review the generated code thoroughly."""),
    ]

    response = await router.chat(messages, Purpose.REVIEW)

    result = _parse_review_response(response)
    result["raw_response"] = response

    logger.info(
        "code_reviewed",
        passed=result.get("passed", False),
        score=result.get("score", 0),
        findings_count=len(result.get("findings", [])),
    )

    return result


def _parse_review_response(response: str) -> dict:
    """Extract structured JSON from the reviewer's response."""
    try:
        if "```json" in response:
            start = response.index("```json") + 7
            end = response.index("```", start)
            json_str = response[start:end].strip()
            return json.loads(json_str)
        return json.loads(response)
    except (json.JSONDecodeError, ValueError):
        return {
            "passed": True,
            "score": 70,
            "summary": response[:500],
            "findings": [],
        }
```

- [ ] **Step 6: 验证**

```bash
cd ai-worker
source .venv/bin/activate
python -c "
from src.agents.analyst import analyze_requirement
from src.agents.planner import plan_task
from src.agents.coder import generate_code
from src.agents.reviewer import review_code
print('All agents imported successfully')
"
```

- [ ] **Step 7: Commit**

```bash
git add ai-worker/src/agents/
git commit -m "feat(s6): add AI agents — analyst, planner, coder, reviewer"
```

---

## Task 5: Temporal Activities（Python 端）

**Files:**
- Create: `ai-worker/src/activities/__init__.py`
- Create: `ai-worker/src/activities/analyze.py`
- Create: `ai-worker/src/activities/plan.py`
- Create: `ai-worker/src/activities/generate.py`
- Create: `ai-worker/src/activities/review.py`
- Create: `ai-worker/src/worker.py`

- [ ] **Step 1: 创建 Activities __init__**

`ai-worker/src/activities/__init__.py`:
```python
```

- [ ] **Step 2: 创建 AnalyzeRequirement Activity**

`ai-worker/src/activities/analyze.py`:

每个 Activity 是一个 Temporal Activity Function，接收 dataclass 输入，返回 dataclass 输出。

```python
from __future__ import annotations

from dataclasses import dataclass

import structlog
from temporalio import activity

from src.agents.analyst import analyze_requirement
from src.context.builder import ContextBuilder

logger = structlog.get_logger()


@dataclass
class AnalyzeInput:
    """Input for the analyze requirement activity."""
    project_id: int
    task_id: int
    requirement: str
    conversation_history: list[dict] | None = None


@dataclass
class AnalyzeOutput:
    """Output from the analyze requirement activity."""
    status: str           # "clarify" or "confirmed"
    summary: str
    questions: list[str]
    assumptions: list[str]
    technical_hints: list[str]
    raw_response: str


@activity.defn
async def analyze_requirement_activity(input: AnalyzeInput) -> AnalyzeOutput:
    """Temporal Activity: Analyze a user requirement.

    Called by the Go TaskWorkflow via Temporal cross-language Activity invocation.
    """
    activity.logger.info(f"Analyzing requirement for task {input.task_id}")

    builder = ContextBuilder()
    try:
        ctx = await builder.build(
            project_id=input.project_id,
            purpose="analyze",
            requirement=input.requirement,
            conversation_history=input.conversation_history,
        )

        result = await analyze_requirement(
            requirement=input.requirement,
            project_context=ctx,
            conversation_history=input.conversation_history,
        )

        return AnalyzeOutput(
            status=result.get("status", "confirmed"),
            summary=result.get("summary", ""),
            questions=result.get("questions", []),
            assumptions=result.get("assumptions", []),
            technical_hints=result.get("technical_hints", []),
            raw_response=result.get("raw_response", ""),
        )
    finally:
        await builder.close()
```

- [ ] **Step 3: 创建 PlanTask Activity**

`ai-worker/src/activities/plan.py`:

```python
from __future__ import annotations

from dataclasses import dataclass, field

import structlog
from temporalio import activity

from src.agents.planner import plan_task
from src.context.builder import ContextBuilder

logger = structlog.get_logger()


@dataclass
class PlanInput:
    """Input for the plan task activity."""
    project_id: int
    task_id: int
    requirement: str
    analysis: dict


@dataclass
class TaskItem:
    """A single task in the implementation plan."""
    order: int
    title: str
    description: str
    files: list[str]
    estimate_hours: float
    type: str  # create | modify


@dataclass
class PlanOutput:
    """Output from the plan task activity."""
    title: str
    summary: str
    tasks: list[dict]  # Use dict for Temporal serialization simplicity
    total_estimate_hours: float
    risk_level: str
    risk_factors: list[str]
    branch_name: str
    raw_response: str


@activity.defn
async def plan_task_activity(input: PlanInput) -> PlanOutput:
    """Temporal Activity: Create implementation plan from requirement analysis."""
    activity.logger.info(f"Planning task {input.task_id}")

    builder = ContextBuilder()
    try:
        ctx = await builder.build(
            project_id=input.project_id,
            purpose="plan",
            requirement=input.requirement,
        )

        result = await plan_task(
            analysis=input.analysis,
            requirement=input.requirement,
            project_context=ctx,
        )

        return PlanOutput(
            title=result.get("title", ""),
            summary=result.get("summary", ""),
            tasks=result.get("tasks", []),
            total_estimate_hours=result.get("total_estimate_hours", 0),
            risk_level=result.get("risk_level", "MEDIUM"),
            risk_factors=result.get("risk_factors", []),
            branch_name=result.get("branch_name", f"feature/task-{input.task_id}"),
            raw_response=result.get("raw_response", ""),
        )
    finally:
        await builder.close()
```

- [ ] **Step 4: 创建 GenerateCode Activity**

`ai-worker/src/activities/generate.py`:

```python
from __future__ import annotations

from dataclasses import dataclass

import structlog
from temporalio import activity

from src.agents.coder import generate_code
from src.context.builder import ContextBuilder

logger = structlog.get_logger()


@dataclass
class GenerateInput:
    """Input for the code generation activity."""
    project_id: int
    task_id: int
    requirement: str
    plan: dict
    task_index: int | None = None  # Generate specific subtask or all


@dataclass
class GeneratedFile:
    """A single generated file."""
    path: str
    content: str
    action: str  # create | modify
    description: str


@dataclass
class GenerateOutput:
    """Output from the code generation activity."""
    files: list[dict]  # Dict for Temporal serialization
    commit_message: str
    raw_response: str


@activity.defn
async def generate_code_activity(input: GenerateInput) -> GenerateOutput:
    """Temporal Activity: Generate code based on implementation plan."""
    activity.logger.info(f"Generating code for task {input.task_id}")

    builder = ContextBuilder()
    try:
        ctx = await builder.build(
            project_id=input.project_id,
            purpose="generate",
            requirement=input.requirement,
        )

        result = await generate_code(
            plan=input.plan,
            requirement=input.requirement,
            project_context=ctx,
            task_index=input.task_index,
        )

        return GenerateOutput(
            files=result.get("files", []),
            commit_message=result.get("commit_message", "feat: generated code"),
            raw_response=result.get("raw_response", ""),
        )
    finally:
        await builder.close()
```

- [ ] **Step 5: 创建 ReviewCode Activity**

`ai-worker/src/activities/review.py`:

```python
from __future__ import annotations

from dataclasses import dataclass

import structlog
from temporalio import activity

from src.agents.reviewer import review_code
from src.context.builder import ContextBuilder

logger = structlog.get_logger()


@dataclass
class ReviewInput:
    """Input for the code review activity."""
    project_id: int
    task_id: int
    requirement: str
    plan: dict
    generated_code: dict


@dataclass
class ReviewFinding:
    """A single review finding."""
    severity: str  # critical | major | minor | info
    file: str
    line: int
    message: str
    suggestion: str


@dataclass
class ReviewOutput:
    """Output from the code review activity."""
    passed: bool
    score: int
    summary: str
    findings: list[dict]  # Dict for Temporal serialization
    fix_instructions: str
    raw_response: str


@activity.defn
async def review_code_activity(input: ReviewInput) -> ReviewOutput:
    """Temporal Activity: Review AI-generated code."""
    activity.logger.info(f"Reviewing code for task {input.task_id}")

    builder = ContextBuilder()
    try:
        ctx = await builder.build(
            project_id=input.project_id,
            purpose="review",
            requirement=input.requirement,
        )

        result = await review_code(
            generated_code=input.generated_code,
            plan=input.plan,
            requirement=input.requirement,
            project_context=ctx,
        )

        return ReviewOutput(
            passed=result.get("passed", True),
            score=result.get("score", 0),
            summary=result.get("summary", ""),
            findings=result.get("findings", []),
            fix_instructions=result.get("fix_instructions", ""),
            raw_response=result.get("raw_response", ""),
        )
    finally:
        await builder.close()
```

- [ ] **Step 6: 创建 Temporal Worker 入口**

`ai-worker/src/worker.py`:

```python
"""Temporal Worker entry point for the AI Worker service.

Registers all AI activities and starts polling the ai-worker task queue.
"""
from __future__ import annotations

import asyncio
import sys

import structlog
from temporalio.client import Client
from temporalio.worker import Worker

from src.config import settings
from src.activities.analyze import analyze_requirement_activity
from src.activities.plan import plan_task_activity
from src.activities.generate import generate_code_activity
from src.activities.review import review_code_activity

structlog.configure(
    processors=[
        structlog.dev.ConsoleRenderer() if settings.log_level == "DEBUG"
        else structlog.processors.JSONRenderer(),
    ],
    wrapper_class=structlog.make_filtering_bound_logger(
        getattr(structlog, settings.log_level, structlog.INFO),
    ),
)

logger = structlog.get_logger()


async def main() -> None:
    """Start the Temporal Worker."""
    logger.info(
        "ai_worker_starting",
        temporal_host=settings.temporal_host,
        namespace=settings.temporal_namespace,
        task_queue=settings.temporal_task_queue,
    )

    # Connect to Temporal server
    client = await Client.connect(
        settings.temporal_host,
        namespace=settings.temporal_namespace,
    )

    # Register all activities and start worker
    worker = Worker(
        client,
        task_queue=settings.temporal_task_queue,
        activities=[
            analyze_requirement_activity,
            plan_task_activity,
            generate_code_activity,
            review_code_activity,
        ],
    )

    logger.info("ai_worker_ready", task_queue=settings.temporal_task_queue)

    await worker.run()


def run() -> None:
    """Entry point wrapper."""
    try:
        asyncio.run(main())
    except KeyboardInterrupt:
        logger.info("ai_worker_shutdown")
        sys.exit(0)


if __name__ == "__main__":
    run()
```

- [ ] **Step 7: 验证 Worker 导入**

```bash
cd ai-worker
source .venv/bin/activate
python -c "
from src.worker import main
print('Worker imports OK — all activities registered')
"
```

注意：实际运行 worker 需要 Temporal Server 可用：
```bash
python -m src.worker
# 预期: ai_worker_starting → 连接 Temporal → ai_worker_ready
# 如果 Temporal 不可用，会报连接错误（正常）
```

- [ ] **Step 8: Commit**

```bash
git add ai-worker/src/activities/ ai-worker/src/worker.py
git commit -m "feat(s6): add Temporal activities and worker entry point for AI pipeline"
```

---

## Task 6: Go 端 — 对话 API + 替换骨架 Workflow

**Files:**
- Create: `forge-core/migrations/006_conversations.sql`- Create: `forge-core/internal/module/task/conversation_handler.go`
- Create: `forge-core/internal/module/task/conversation_service.go`
- Create: `forge-core/internal/module/task/conversation_repo.go`
- Modify: `forge-core/internal/module/task/model.go` — 新增 Conversation model
- Modify: `forge-core/internal/module/task/workflow.go` — 替换 mock 为真实 Workflow
- Modify: `forge-core/internal/module/task/handler.go` — 新增 confirm endpoint
- Modify: `forge-core/internal/router/router.go` — 注册对话路由

**Important:** 查看 forge-core 中已有的迁移文件编号，使用下一个序号。查看已有的 handler/service/repository 模式，保持一致。

- [ ] **Step 1: 创建 conversations 迁移**

创建迁移文件 `forge-core/migrations/006_conversations.sql`（替换 X 为实际下一个序号）：

```sql
-- Conversation messages for AI dialog
CREATE TABLE IF NOT EXISTS engine.conversations (
    id              BIGSERIAL PRIMARY KEY,
    task_id         BIGINT NOT NULL REFERENCES engine.tasks(id),
    role            VARCHAR(20) NOT NULL,   -- user / assistant / system
    content         TEXT NOT NULL,
    metadata        JSONB DEFAULT '{}',     -- tool_calls, token counts, model info
    tokens_used     INT DEFAULT 0,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_conversations_task_id ON engine.conversations(task_id);
CREATE INDEX idx_conversations_created_at ON engine.conversations(task_id, created_at);

-- AI model call records (for cost tracking and observability)
CREATE TABLE IF NOT EXISTS engine.model_calls (
    id              BIGSERIAL PRIMARY KEY,
    tenant_id       BIGINT NOT NULL,
    task_id         BIGINT REFERENCES engine.tasks(id),
    step_id         BIGINT REFERENCES engine.task_steps(id),
    model           VARCHAR(50) NOT NULL,       -- claude-sonnet-4, gpt-4o, etc.
    provider        VARCHAR(30) NOT NULL,       -- anthropic, openai, dashscope
    purpose         VARCHAR(30) NOT NULL,       -- ANALYZE, GENERATE, REVIEW, FIX
    input_tokens    INT NOT NULL DEFAULT 0,
    output_tokens   INT NOT NULL DEFAULT 0,
    total_tokens    INT NOT NULL DEFAULT 0,
    cost_cents      INT NOT NULL DEFAULT 0,     -- 费用(分), Phase 1 记录但不限制
    latency_ms      INT NOT NULL DEFAULT 0,
    status          VARCHAR(10) NOT NULL,       -- SUCCESS, FAILED, TIMEOUT
    error_code      VARCHAR(50),
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_model_calls_task_id ON engine.model_calls(task_id);
CREATE INDEX idx_model_calls_tenant_created ON engine.model_calls(tenant_id, created_at);
```

> **说明**: `model_calls` 表在 Phase 1 仅记录数据，不做预算控制。Phase 2 将基于此表实现 Token 预算硬限和租户月度成本报表。

- [ ] **Step 2: 新增 Conversation Model**

在 `forge-core/internal/module/task/model.go` 中添加：

```go
// Conversation represents a message in the AI dialog
type Conversation struct {
    ID         int64           `json:"id"`
    TaskID     int64           `json:"taskId"`
    Role       string          `json:"role"`       // user / assistant / system
    Content    string          `json:"content"`
    Metadata   json.RawMessage `json:"metadata"`
    TokensUsed int             `json:"tokensUsed"`
    CreatedAt  time.Time       `json:"createdAt"`
}

// SendMessageRequest is the request body for sending a chat message
type SendMessageRequest struct {
    Content string `json:"content" binding:"required"`
}

// ConfirmPlanRequest is the request body for confirming the AI plan
type ConfirmPlanRequest struct {
    // Empty for now; plan is already stored in task.task_graph
}

// ConversationResponse wraps conversation data for SSE streaming
type ConversationResponse struct {
    Type    string      `json:"type"`    // message / thinking / error / done
    Content interface{} `json:"content"`
}
```

- [ ] **Step 3: 创建 Conversation Repository**

`forge-core/internal/module/task/conversation_repo.go`:

```go
package task

import (
    "context"
    "encoding/json"
    "time"

    "github.com/jackc/pgx/v5/pgxpool"
)

type ConversationRepository struct {
    db *pgxpool.Pool
}

func NewConversationRepository(db *pgxpool.Pool) *ConversationRepository {
    return &ConversationRepository{db: db}
}

func (r *ConversationRepository) Create(ctx context.Context, conv *Conversation) error {
    query := `
        INSERT INTO engine.conversations (task_id, role, content, metadata, tokens_used, created_at)
        VALUES ($1, $2, $3, $4, $5, $6)
        RETURNING id`

    if conv.Metadata == nil {
        conv.Metadata = json.RawMessage("{}")
    }
    if conv.CreatedAt.IsZero() {
        conv.CreatedAt = time.Now()
    }

    return r.db.QueryRow(ctx, query,
        conv.TaskID, conv.Role, conv.Content, conv.Metadata, conv.TokensUsed, conv.CreatedAt,
    ).Scan(&conv.ID)
}

func (r *ConversationRepository) ListByTaskID(ctx context.Context, taskID int64) ([]Conversation, error) {
    query := `
        SELECT id, task_id, role, content, metadata, tokens_used, created_at
        FROM engine.conversations
        WHERE task_id = $1
        ORDER BY created_at ASC`

    rows, err := r.db.Query(ctx, query, taskID)
    if err != nil {
        return nil, err
    }
    defer rows.Close()

    var conversations []Conversation
    for rows.Next() {
        var c Conversation
        if err := rows.Scan(&c.ID, &c.TaskID, &c.Role, &c.Content, &c.Metadata, &c.TokensUsed, &c.CreatedAt); err != nil {
            return nil, err
        }
        conversations = append(conversations, c)
    }
    return conversations, rows.Err()
}
```

- [ ] **Step 4: 创建 Conversation Service**

`forge-core/internal/module/task/conversation_service.go`:

Service 负责：保存用户消息 → 调用 Temporal Activity（分析） → 保存 AI 回复 → 返回。

```go
package task

import (
    "context"
    "encoding/json"
    "fmt"
    "log/slog"

    "go.temporal.io/sdk/client"
)

type ConversationService struct {
    convRepo       *ConversationRepository
    taskRepo       *Repository  // existing task repository from S4
    temporalClient client.Client
}

func NewConversationService(
    convRepo *ConversationRepository,
    taskRepo *Repository,
    temporalClient client.Client,
) *ConversationService {
    return &ConversationService{
        convRepo:       convRepo,
        taskRepo:       taskRepo,
        temporalClient: temporalClient,
    }
}

// SendMessage handles a user message in the requirement dialog.
// Saves the user message, calls AI analysis, saves AI response, returns it.
func (s *ConversationService) SendMessage(ctx context.Context, taskID int64, content string) (*Conversation, error) {
    // 1. Get task to verify it exists and get project context
    task, err := s.taskRepo.GetByID(ctx, taskID)
    if err != nil {
        return nil, fmt.Errorf("get task: %w", err)
    }

    // 2. Save user message
    userMsg := &Conversation{
        TaskID:  taskID,
        Role:    "user",
        Content: content,
    }
    if err := s.convRepo.Create(ctx, userMsg); err != nil {
        return nil, fmt.Errorf("save user message: %w", err)
    }

    // 3. Get conversation history
    history, err := s.convRepo.ListByTaskID(ctx, taskID)
    if err != nil {
        return nil, fmt.Errorf("get history: %w", err)
    }

    // Convert to simple format for Temporal
    historyMsgs := make([]map[string]interface{}, len(history))
    for i, h := range history {
        historyMsgs[i] = map[string]interface{}{
            "role":    h.Role,
            "content": h.Content,
        }
    }

    // 4. Call AI analysis via Temporal Activity
    // Phase 1: Execute activity directly (synchronous, no streaming)
    analyzeInput := map[string]interface{}{
        "project_id":            task.ProjectID,
        "task_id":               taskID,
        "requirement":           content,
        "conversation_history":  historyMsgs,
    }

    var analyzeResult map[string]interface{}

    // Execute the analyze activity on the ai-worker task queue
    future := s.temporalClient.ExecuteWorkflow(ctx, client.StartWorkflowOptions{
        ID:        fmt.Sprintf("analyze-%d-%d", taskID, len(history)),
        TaskQueue: "ai-worker",
    }, "AnalyzeRequirementWorkflow", analyzeInput)

    if err := future.Get(ctx, &analyzeResult); err != nil {
        slog.Error("analyze activity failed", "error", err, "taskID", taskID)
        // Save error as assistant message
        errMsg := &Conversation{
            TaskID:  taskID,
            Role:    "assistant",
            Content: fmt.Sprintf("分析失败，请重试。错误: %s", err.Error()),
        }
        _ = s.convRepo.Create(ctx, errMsg)
        return errMsg, nil
    }

    // 5. Save AI response
    aiContent := ""
    if raw, ok := analyzeResult["raw_response"].(string); ok {
        aiContent = raw
    } else if summary, ok := analyzeResult["summary"].(string); ok {
        aiContent = summary
    }

    metadata, _ := json.Marshal(analyzeResult)
    aiMsg := &Conversation{
        TaskID:   taskID,
        Role:     "assistant",
        Content:  aiContent,
        Metadata: metadata,
    }
    if err := s.convRepo.Create(ctx, aiMsg); err != nil {
        return nil, fmt.Errorf("save ai message: %w", err)
    }

    // 6. If analysis is confirmed, update task with analysis data
    if status, ok := analyzeResult["status"].(string); ok && status == "confirmed" {
        analysisJSON, _ := json.Marshal(analyzeResult)
        if err := s.taskRepo.UpdateAnalysis(ctx, taskID, analysisJSON); err != nil {
            slog.Warn("failed to update task analysis", "error", err, "taskID", taskID)
        }
    }

    return aiMsg, nil
}

// GetHistory returns all conversation messages for a task.
func (s *ConversationService) GetHistory(ctx context.Context, taskID int64) ([]Conversation, error) {
    return s.convRepo.ListByTaskID(ctx, taskID)
}

// ConfirmPlan confirms the AI plan and triggers the full generation workflow.
func (s *ConversationService) ConfirmPlan(ctx context.Context, taskID int64) error {
    task, err := s.taskRepo.GetByID(ctx, taskID)
    if err != nil {
        return fmt.Errorf("get task: %w", err)
    }

    // Update task status to PLAN_CONFIRMED
    if err := s.taskRepo.UpdateStatus(ctx, taskID, "PLAN_CONFIRMED"); err != nil {
        return fmt.Errorf("update status: %w", err)
    }

    // Start the full generation workflow via Temporal
    workflowInput := map[string]interface{}{
        "project_id":  task.ProjectID,
        "task_id":     taskID,
        "requirement": task.Requirement,
        "analysis":    task.Analysis,
        "task_graph":  task.TaskGraph,
    }

    workflowOptions := client.StartWorkflowOptions{
        ID:        fmt.Sprintf("task-generate-%d", taskID),
        TaskQueue: "ai-worker",
    }

    run, err := s.temporalClient.ExecuteWorkflow(ctx, workflowOptions, "TaskGenerationWorkflow", workflowInput)
    if err != nil {
        return fmt.Errorf("start workflow: %w", err)
    }

    // Save workflow ID to task
    if err := s.taskRepo.UpdateWorkflowID(ctx, taskID, run.GetID(), run.GetRunID()); err != nil {
        slog.Warn("failed to save workflow ID", "error", err, "taskID", taskID)
    }

    slog.Info("generation workflow started", "taskID", taskID, "workflowID", run.GetID())
    return nil
}
```

**Note:** 上面的代码引用了 `taskRepo.UpdateAnalysis`、`taskRepo.UpdateWorkflowID` 等方法。实现时需要检查 S4 已有的 task repository，如果没有这些方法就添加。类似地，`task.ProjectID`、`task.Analysis`、`task.TaskGraph` 等字段需要确认 S4 的 task model 是否已有。

- [ ] **Step 5: 创建 Conversation Handler**

`forge-core/internal/module/task/conversation_handler.go`:

```go
package task

import (
    "fmt"
    "io"
    "log/slog"
    "net/http"
    "strconv"

    "github.com/gin-gonic/gin"
    "github.com/shulex/forge/forge-core/internal/pkg/response"
)

type ConversationHandler struct {
    service *ConversationService
}

func NewConversationHandler(service *ConversationService) *ConversationHandler {
    return &ConversationHandler{service: service}
}

// SendMessage handles POST /api/projects/:id/conversations/:taskId/messages
// Phase 1: Returns full AI response (no streaming).
// Phase 2+: Will stream via SSE.
func (h *ConversationHandler) SendMessage(c *gin.Context) {
    taskID, err := strconv.ParseInt(c.Param("taskId"), 10, 64)
    if err != nil {
        response.Fail(c, http.StatusBadRequest, "invalid task ID")
        return
    }

    var req SendMessageRequest
    if err := c.ShouldBindJSON(&req); err != nil {
        response.Fail(c, http.StatusBadRequest, "invalid request body")
        return
    }

    // Phase 1: Synchronous response (not SSE yet)
    // In Phase 2, this will switch to SSE streaming
    aiMsg, err := h.service.SendMessage(c.Request.Context(), taskID, req.Content)
    if err != nil {
        slog.Error("send message failed", "error", err, "taskID", taskID)
        response.Fail(c, http.StatusInternalServerError, "failed to process message")
        return
    }

    response.OK(c, aiMsg)
}

// SendMessageSSE handles POST /api/projects/:id/conversations/:taskId/messages/stream
// SSE endpoint for streaming AI responses.
func (h *ConversationHandler) SendMessageSSE(c *gin.Context) {
    taskID, err := strconv.ParseInt(c.Param("taskId"), 10, 64)
    if err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": "invalid task ID"})
        return
    }

    var req SendMessageRequest
    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
        return
    }

    // Set SSE headers
    c.Header("Content-Type", "text/event-stream")
    c.Header("Cache-Control", "no-cache")
    c.Header("Connection", "keep-alive")

    // Phase 1: Call AI synchronously, then write full response as SSE events
    // This gives the frontend an SSE-compatible interface it can keep when we add true streaming
    c.SSEvent("thinking", `{"type":"thinking","content":"AI 正在分析..."}`)
    c.Writer.Flush()

    aiMsg, err := h.service.SendMessage(c.Request.Context(), taskID, req.Content)
    if err != nil {
        c.SSEvent("error", fmt.Sprintf(`{"type":"error","content":"%s"}`, err.Error()))
        c.Writer.Flush()
        return
    }

    c.SSEvent("message", fmt.Sprintf(`{"type":"message","content":%q}`, aiMsg.Content))
    c.Writer.Flush()

    // Send metadata (analysis result if available)
    if len(aiMsg.Metadata) > 0 && string(aiMsg.Metadata) != "{}" {
        c.SSEvent("metadata", string(aiMsg.Metadata))
        c.Writer.Flush()
    }

    c.SSEvent("done", `{"type":"done"}`)
    c.Writer.Flush()

    // Keep connection briefly to ensure client receives all events
    _, _ = io.WriteString(c.Writer, "\n")
    c.Writer.Flush()
}

// GetHistory handles GET /api/projects/:id/conversations/:taskId/messages
func (h *ConversationHandler) GetHistory(c *gin.Context) {
    taskID, err := strconv.ParseInt(c.Param("taskId"), 10, 64)
    if err != nil {
        response.Fail(c, http.StatusBadRequest, "invalid task ID")
        return
    }

    messages, err := h.service.GetHistory(c.Request.Context(), taskID)
    if err != nil {
        slog.Error("get history failed", "error", err, "taskID", taskID)
        response.Fail(c, http.StatusInternalServerError, "failed to get conversation history")
        return
    }

    response.OK(c, messages)
}

// ConfirmPlan handles POST /api/projects/:id/tasks/:taskId/confirm
func (h *ConversationHandler) ConfirmPlan(c *gin.Context) {
    taskID, err := strconv.ParseInt(c.Param("taskId"), 10, 64)
    if err != nil {
        response.Fail(c, http.StatusBadRequest, "invalid task ID")
        return
    }

    if err := h.service.ConfirmPlan(c.Request.Context(), taskID); err != nil {
        slog.Error("confirm plan failed", "error", err, "taskID", taskID)
        response.Fail(c, http.StatusInternalServerError, "failed to confirm plan")
        return
    }

    response.OK(c, gin.H{"message": "Plan confirmed, generation workflow started"})
}
```

- [ ] **Step 6: 修改 Go Workflow — 替换 mock 为真实 AI Workflow**

找到 S4 的 `workflow.go`（或等价文件），替换骨架 mock。如果 S4 中 Workflow 是在 Go 中定义的，修改为调用 Python 端 Activities。

**注意**：Temporal 跨语言 Activity 调用需要 Activity 名称匹配。Python 端的 Activity 函数名（如 `analyze_requirement_activity`）需要和 Go 端的 `ExecuteActivity` 调用名一致。

在 `forge-core/internal/module/task/workflow.go` 中，定义或修改主 Workflow：

```go
package task

import (
    "fmt"
    "time"

    "go.temporal.io/sdk/temporal"
    "go.temporal.io/sdk/workflow"
)

// TaskGenerationWorkflow is the main workflow for AI code generation.
// It orchestrates: Plan → Generate → Review → (Fix if needed) → Commit
//
// This workflow runs in the Go worker but calls Python AI Worker activities
// via the "ai-worker" task queue for cross-language activity invocation.
func TaskGenerationWorkflow(ctx workflow.Context, input map[string]interface{}) (map[string]interface{}, error) {
    logger := workflow.GetLogger(ctx)
    taskID := int64(input["task_id"].(float64))
    projectID := int64(input["project_id"].(float64))
    requirement := input["requirement"].(string)

    // Activity options for AI worker activities (longer timeout for LLM calls)
    aiActivityOptions := workflow.ActivityOptions{
        TaskQueue:              "ai-worker",
        StartToCloseTimeout:    5 * time.Minute,
        HeartbeatTimeout:       30 * time.Second,
        RetryPolicy: &temporal.RetryPolicy{
            InitialInterval:    5 * time.Second,
            BackoffCoefficient: 2.0,
            MaximumAttempts:    3,
        },
    }
    aiCtx := workflow.WithActivityOptions(ctx, aiActivityOptions)

    // Local activity options for status updates (runs in Go worker)
    localOptions := workflow.LocalActivityOptions{
        StartToCloseTimeout: 10 * time.Second,
    }
    localCtx := workflow.WithLocalActivityOptions(ctx, localOptions)

    result := map[string]interface{}{}

    // --- Step 1: Plan (if not already planned) ---
    logger.Info("TaskGenerationWorkflow: planning", "taskID", taskID)
    _ = workflow.ExecuteLocalActivity(localCtx, UpdateTaskStatusActivity, taskID, "PLANNING").Get(ctx, nil)

    planInput := map[string]interface{}{
        "project_id":  projectID,
        "task_id":     taskID,
        "requirement": requirement,
        "analysis":    input["analysis"],
    }

    var planOutput map[string]interface{}
    if err := workflow.ExecuteActivity(aiCtx, "plan_task_activity", planInput).Get(ctx, &planOutput); err != nil {
        _ = workflow.ExecuteLocalActivity(localCtx, UpdateTaskStatusActivity, taskID, "FAILED").Get(ctx, nil)
        return nil, fmt.Errorf("plan task: %w", err)
    }
    result["plan"] = planOutput

    // --- Step 2: Generate Code ---
    logger.Info("TaskGenerationWorkflow: generating", "taskID", taskID)
    _ = workflow.ExecuteLocalActivity(localCtx, UpdateTaskStatusActivity, taskID, "GENERATING").Get(ctx, nil)

    generateInput := map[string]interface{}{
        "project_id":  projectID,
        "task_id":     taskID,
        "requirement": requirement,
        "plan":        planOutput,
    }

    var generateOutput map[string]interface{}
    if err := workflow.ExecuteActivity(aiCtx, "generate_code_activity", generateInput).Get(ctx, &generateOutput); err != nil {
        _ = workflow.ExecuteLocalActivity(localCtx, UpdateTaskStatusActivity, taskID, "FAILED").Get(ctx, nil)
        return nil, fmt.Errorf("generate code: %w", err)
    }
    result["generated_code"] = generateOutput

    // --- Step 3: Review Code (with retry loop) ---
    const maxReviewAttempts = 3
    var reviewOutput map[string]interface{}

    for attempt := 1; attempt <= maxReviewAttempts; attempt++ {
        logger.Info("TaskGenerationWorkflow: reviewing", "taskID", taskID, "attempt", attempt)
        _ = workflow.ExecuteLocalActivity(localCtx, UpdateTaskStatusActivity, taskID, "REVIEWING").Get(ctx, nil)

        reviewInput := map[string]interface{}{
            "project_id":     projectID,
            "task_id":        taskID,
            "requirement":    requirement,
            "plan":           planOutput,
            "generated_code": generateOutput,
        }

        if err := workflow.ExecuteActivity(aiCtx, "review_code_activity", reviewInput).Get(ctx, &reviewOutput); err != nil {
            logger.Warn("review failed", "error", err, "attempt", attempt)
            if attempt == maxReviewAttempts {
                _ = workflow.ExecuteLocalActivity(localCtx, UpdateTaskStatusActivity, taskID, "FAILED").Get(ctx, nil)
                return nil, fmt.Errorf("review code: %w", err)
            }
            continue
        }

        // Check if review passed
        passed, _ := reviewOutput["passed"].(bool)
        if passed {
            break
        }

        // Review failed — regenerate code with fix instructions
        if attempt < maxReviewAttempts {
            logger.Info("TaskGenerationWorkflow: regenerating after review", "taskID", taskID, "attempt", attempt)
            _ = workflow.ExecuteLocalActivity(localCtx, UpdateTaskStatusActivity, taskID, "GENERATING").Get(ctx, nil)

            // Add fix instructions to generate input
            generateInput["fix_instructions"] = reviewOutput["fix_instructions"]
            if err := workflow.ExecuteActivity(aiCtx, "generate_code_activity", generateInput).Get(ctx, &generateOutput); err != nil {
                if attempt+1 >= maxReviewAttempts {
                    _ = workflow.ExecuteLocalActivity(localCtx, UpdateTaskStatusActivity, taskID, "FAILED").Get(ctx, nil)
                    return nil, fmt.Errorf("regenerate code: %w", err)
                }
            }
            result["generated_code"] = generateOutput
        }
    }
    result["review"] = reviewOutput

    // --- Step 4: Commit to GitHub ---
    // TODO (S7): Call devops-worker to commit code to branch
    // For Phase 1, mark as completed. Code is in the workflow result.
    logger.Info("TaskGenerationWorkflow: completed", "taskID", taskID)
    _ = workflow.ExecuteLocalActivity(localCtx, UpdateTaskStatusActivity, taskID, "COMPLETED").Get(ctx, nil)

    return result, nil
}

// UpdateTaskStatusActivity is a local activity that updates task status in the database.
func UpdateTaskStatusActivity(ctx workflow.Context, taskID int64, status string) error {
    // This will be registered as a local activity in the Go worker.
    // Implementation uses the task repository to update status.
    // The actual implementation needs access to the repository; see worker registration.
    return nil
}
```

**Important implementation note:** The `UpdateTaskStatusActivity` placeholder above needs to be implemented as a proper local activity with database access. During implementation, create a proper activity struct or closure that has access to the task repository. For example:

```go
// In the Go Temporal worker registration, create a closure:
statusUpdater := func(ctx context.Context, taskID int64, status string) error {
    return taskRepo.UpdateStatus(ctx, taskID, status)
}
// Register: worker.RegisterActivity(statusUpdater)
```

- [ ] **Step 7: 注册对话路由**

在 `forge-core/internal/router/router.go` 中添加对话相关路由：

```go
// Inside the authenticated API group (after existing task routes):

// Conversation / Requirement Dialog
convGroup := api.Group("/projects/:id/conversations")
{
    convGroup.POST("/:taskId/messages", convHandler.SendMessage)
    convGroup.POST("/:taskId/messages/stream", convHandler.SendMessageSSE)
    convGroup.GET("/:taskId/messages", convHandler.GetHistory)
}

// Plan confirmation
api.POST("/projects/:id/tasks/:taskId/confirm", convHandler.ConfirmPlan)
```

- [ ] **Step 8: 验证编译**

```bash
cd forge-core
go build ./cmd/forge-core
# 预期: 编译通过
```

- [ ] **Step 9: 运行迁移验证**

```bash
docker compose -f docker-compose.dev.yml up -d
cd forge-core && go run ./cmd/forge-core
# 检查 conversations 表是否创建成功
docker exec forge-postgres psql -U forge -d forge_main -c "\dt engine.*"
# 预期: 包含 conversations 表
```

- [ ] **Step 10: Commit**

```bash
git add forge-core/
git commit -m "feat(s6): add conversation API, replace skeleton workflow with real AI workflow"
```

---

## Task 7: 前端 — 需求对话页面

**Files:**
- Create: `forge-portal/app/(dashboard)/projects/[id]/tasks/new/page.tsx`
- Create: `forge-portal/components/chat/chat-panel.tsx`
- Create: `forge-portal/components/chat/message-bubble.tsx`
- Create: `forge-portal/components/chat/confirmation-card.tsx`
- Create: `forge-portal/components/chat/thinking-indicator.tsx`
- Create: `forge-portal/lib/api/conversation.ts`
- Create: `forge-portal/lib/hooks/use-sse.ts`

- [ ] **Step 1: 创建 SSE Hook**

`forge-portal/lib/hooks/use-sse.ts`:

```typescript
"use client";

import { useState, useCallback, useRef } from "react";

interface SSEEvent {
  type: string;
  content: string;
}

interface UseSSEOptions {
  onMessage?: (event: SSEEvent) => void;
  onThinking?: () => void;
  onDone?: () => void;
  onError?: (error: string) => void;
}

export function useSSE(options: UseSSEOptions) {
  const [isStreaming, setIsStreaming] = useState(false);
  const abortRef = useRef<AbortController | null>(null);

  const send = useCallback(
    async (url: string, body: object) => {
      setIsStreaming(true);
      abortRef.current = new AbortController();

      try {
        const token = localStorage.getItem("token");
        const response = await fetch(url, {
          method: "POST",
          headers: {
            "Content-Type": "application/json",
            Authorization: `Bearer ${token}`,
          },
          body: JSON.stringify(body),
          signal: abortRef.current.signal,
        });

        if (!response.ok) {
          throw new Error(`HTTP ${response.status}`);
        }

        const reader = response.body?.getReader();
        if (!reader) throw new Error("No response body");

        const decoder = new TextDecoder();
        let buffer = "";

        while (true) {
          const { done, value } = await reader.read();
          if (done) break;

          buffer += decoder.decode(value, { stream: true });
          const lines = buffer.split("\n");
          buffer = lines.pop() || "";

          for (const line of lines) {
            if (line.startsWith("event:")) {
              const eventType = line.slice(6).trim();
              // Next line should be data:
              continue;
            }
            if (line.startsWith("data:")) {
              const data = line.slice(5).trim();
              try {
                const parsed = JSON.parse(data);
                switch (parsed.type) {
                  case "thinking":
                    options.onThinking?.();
                    break;
                  case "message":
                    options.onMessage?.(parsed);
                    break;
                  case "error":
                    options.onError?.(parsed.content);
                    break;
                  case "done":
                    options.onDone?.();
                    break;
                }
              } catch {
                // Not JSON, might be raw SSE
              }
            }
          }
        }
      } catch (err) {
        if ((err as Error).name !== "AbortError") {
          options.onError?.((err as Error).message);
        }
      } finally {
        setIsStreaming(false);
      }
    },
    [options]
  );

  const cancel = useCallback(() => {
    abortRef.current?.abort();
  }, []);

  return { send, cancel, isStreaming };
}
```

- [ ] **Step 2: 创建 Conversation API 客户端**

`forge-portal/lib/api/conversation.ts`:

```typescript
const API_BASE = "/api";

interface Message {
  id: number;
  taskId: number;
  role: "user" | "assistant" | "system";
  content: string;
  metadata?: Record<string, unknown>;
  tokensUsed: number;
  createdAt: string;
}

export async function getConversationHistory(
  projectId: number,
  taskId: number
): Promise<Message[]> {
  const token = localStorage.getItem("token");
  const res = await fetch(
    `${API_BASE}/projects/${projectId}/conversations/${taskId}/messages`,
    {
      headers: { Authorization: `Bearer ${token}` },
    }
  );
  const json = await res.json();
  return json.data || [];
}

export async function sendMessage(
  projectId: number,
  taskId: number,
  content: string
): Promise<Message> {
  const token = localStorage.getItem("token");
  const res = await fetch(
    `${API_BASE}/projects/${projectId}/conversations/${taskId}/messages`,
    {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
        Authorization: `Bearer ${token}`,
      },
      body: JSON.stringify({ content }),
    }
  );
  const json = await res.json();
  return json.data;
}

export async function confirmPlan(
  projectId: number,
  taskId: number
): Promise<void> {
  const token = localStorage.getItem("token");
  await fetch(
    `${API_BASE}/projects/${projectId}/tasks/${taskId}/confirm`,
    {
      method: "POST",
      headers: { Authorization: `Bearer ${token}` },
    }
  );
}

export function getStreamURL(projectId: number, taskId: number): string {
  return `${API_BASE}/projects/${projectId}/conversations/${taskId}/messages/stream`;
}

export type { Message };
```

- [ ] **Step 3: 创建消息气泡组件**

`forge-portal/components/chat/message-bubble.tsx`:

```tsx
"use client";

import { cn } from "@/lib/utils";

interface MessageBubbleProps {
  role: "user" | "assistant" | "system";
  content: string;
  timestamp?: string;
}

export function MessageBubble({ role, content, timestamp }: MessageBubbleProps) {
  const isUser = role === "user";

  return (
    <div className={cn("flex gap-3 mb-4", isUser ? "justify-end" : "justify-start")}>
      {/* Avatar */}
      {!isUser && (
        <div className="w-8 h-8 rounded-full bg-[#8B5CF6]/20 flex items-center justify-center flex-shrink-0">
          <span className="text-[#8B5CF6] text-xs font-bold">AI</span>
        </div>
      )}

      {/* Bubble */}
      <div
        className={cn(
          "max-w-[75%] rounded-xl px-4 py-3 text-sm leading-relaxed",
          isUser
            ? "bg-[#1E1E2E] text-zinc-200"      // Surface-2 for user
            : "bg-[#12121A] text-zinc-300 border-l-2 border-[#8B5CF6]"  // Surface-1 + purple border for AI
        )}
      >
        {/* Render markdown-like content */}
        <div className="whitespace-pre-wrap break-words">
          {content.split("```").map((block, i) => {
            if (i % 2 === 1) {
              // Code block
              return (
                <pre
                  key={i}
                  className="bg-[#0A0A12] rounded-lg p-3 my-2 overflow-x-auto text-xs font-mono"
                >
                  <code>{block.replace(/^\w+\n/, "")}</code>
                </pre>
              );
            }
            return <span key={i}>{block}</span>;
          })}
        </div>

        {/* Timestamp */}
        {timestamp && (
          <div className="text-[10px] text-zinc-500 mt-1">
            {new Date(timestamp).toLocaleTimeString()}
          </div>
        )}
      </div>

      {/* User avatar */}
      {isUser && (
        <div className="w-8 h-8 rounded-full bg-zinc-700 flex items-center justify-center flex-shrink-0">
          <span className="text-zinc-300 text-xs font-bold">U</span>
        </div>
      )}
    </div>
  );
}
```

- [ ] **Step 4: 创建 AI 思考中动画**

`forge-portal/components/chat/thinking-indicator.tsx`:

```tsx
"use client";

export function ThinkingIndicator() {
  return (
    <div className="flex gap-3 mb-4 justify-start">
      <div className="w-8 h-8 rounded-full bg-[#8B5CF6]/20 flex items-center justify-center flex-shrink-0">
        <span className="text-[#8B5CF6] text-xs font-bold">AI</span>
      </div>
      <div className="bg-[#12121A] border-l-2 border-[#8B5CF6] rounded-xl px-4 py-3">
        <div className="flex gap-1.5 items-center">
          <span className="text-xs text-zinc-500 mr-2">AI 正在分析</span>
          <div className="w-2 h-2 rounded-full bg-[#8B5CF6] animate-pulse" style={{ animationDelay: "0ms" }} />
          <div className="w-2 h-2 rounded-full bg-[#8B5CF6] animate-pulse" style={{ animationDelay: "300ms" }} />
          <div className="w-2 h-2 rounded-full bg-[#8B5CF6] animate-pulse" style={{ animationDelay: "600ms" }} />
        </div>
      </div>
    </div>
  );
}
```

- [ ] **Step 5: 创建需求确认卡片组件**

`forge-portal/components/chat/confirmation-card.tsx`:

```tsx
"use client";

import { Button } from "@/components/ui/button";
import { CheckCircle, Edit, XCircle, Clock, AlertTriangle } from "lucide-react";

interface TaskItem {
  order: number;
  title: string;
  description: string;
  files: string[];
  estimate_hours: number;
  type: string;
}

interface PlanData {
  title: string;
  summary: string;
  tasks: TaskItem[];
  total_estimate_hours: number;
  risk_level: string;
  risk_factors: string[];
  branch_name: string;
}

interface ConfirmationCardProps {
  plan: PlanData;
  onConfirm: () => void;
  onModify: () => void;
  onCancel: () => void;
  isConfirming?: boolean;
}

const riskColors: Record<string, string> = {
  LOW: "text-emerald-400",
  MEDIUM: "text-amber-400",
  HIGH: "text-red-400",
};

export function ConfirmationCard({
  plan,
  onConfirm,
  onModify,
  onCancel,
  isConfirming = false,
}: ConfirmationCardProps) {
  return (
    <div className="my-4 rounded-xl overflow-hidden border border-zinc-800">
      {/* Purple gradient top border */}
      <div className="h-1 bg-gradient-to-r from-[#8B5CF6] to-[#6D28D9]" />

      <div className="bg-[#12121A] p-5 space-y-4">
        {/* Title */}
        <div>
          <h3 className="text-lg font-semibold text-zinc-100">{plan.title}</h3>
          <p className="text-sm text-zinc-400 mt-1">{plan.summary}</p>
        </div>

        {/* Task Breakdown */}
        <div>
          <h4 className="text-sm font-medium text-zinc-300 mb-2">任务拆解</h4>
          <div className="space-y-2">
            {plan.tasks.map((task) => (
              <div
                key={task.order}
                className="flex items-start gap-3 bg-[#0A0A12] rounded-lg p-3"
              >
                <span className="text-[#8B5CF6] font-mono text-xs mt-0.5">
                  #{task.order}
                </span>
                <div className="flex-1 min-w-0">
                  <div className="text-sm text-zinc-200">{task.title}</div>
                  <div className="text-xs text-zinc-500 mt-0.5">{task.description}</div>
                  {task.files.length > 0 && (
                    <div className="text-xs text-zinc-600 mt-1 font-mono truncate">
                      {task.files.join(", ")}
                    </div>
                  )}
                </div>
                <div className="flex items-center gap-1 text-xs text-zinc-500 flex-shrink-0">
                  <Clock className="w-3 h-3" />
                  {task.estimate_hours}h
                </div>
              </div>
            ))}
          </div>
        </div>

        {/* Meta info */}
        <div className="flex items-center gap-6 text-xs text-zinc-500">
          <div className="flex items-center gap-1">
            <Clock className="w-3.5 h-3.5" />
            预计 {plan.total_estimate_hours} 小时
          </div>
          <div className={`flex items-center gap-1 ${riskColors[plan.risk_level] || "text-zinc-500"}`}>
            <AlertTriangle className="w-3.5 h-3.5" />
            风险: {plan.risk_level}
          </div>
          <div className="font-mono text-zinc-600">
            {plan.branch_name}
          </div>
        </div>

        {/* Risk factors */}
        {plan.risk_factors && plan.risk_factors.length > 0 && (
          <div className="text-xs text-zinc-500">
            <span className="text-zinc-400">风险因素: </span>
            {plan.risk_factors.join("; ")}
          </div>
        )}

        {/* Action buttons */}
        <div className="flex gap-3 pt-2">
          <Button
            onClick={onConfirm}
            disabled={isConfirming}
            className="bg-[#8B5CF6] hover:bg-[#7C3AED] text-white"
          >
            <CheckCircle className="w-4 h-4 mr-1.5" />
            {isConfirming ? "启动中..." : "确认执行"}
          </Button>
          <Button
            onClick={onModify}
            variant="outline"
            className="border-zinc-700 text-zinc-300 hover:bg-zinc-800"
          >
            <Edit className="w-4 h-4 mr-1.5" />
            修改需求
          </Button>
          <Button
            onClick={onCancel}
            variant="ghost"
            className="text-zinc-500 hover:text-zinc-300"
          >
            <XCircle className="w-4 h-4 mr-1.5" />
            取消
          </Button>
        </div>
      </div>
    </div>
  );
}
```

- [ ] **Step 6: 创建对话面板组件**

`forge-portal/components/chat/chat-panel.tsx`:

```tsx
"use client";

import { useState, useRef, useEffect, useCallback } from "react";
import { Send } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Textarea } from "@/components/ui/textarea";
import { MessageBubble } from "./message-bubble";
import { ThinkingIndicator } from "./thinking-indicator";
import { ConfirmationCard } from "./confirmation-card";
import {
  sendMessage,
  confirmPlan,
  getConversationHistory,
  type Message,
} from "@/lib/api/conversation";

interface ChatPanelProps {
  projectId: number;
  taskId: number;
  onConfirmed?: () => void;
}

export function ChatPanel({ projectId, taskId, onConfirmed }: ChatPanelProps) {
  const [messages, setMessages] = useState<Message[]>([]);
  const [input, setInput] = useState("");
  const [isThinking, setIsThinking] = useState(false);
  const [isConfirming, setIsConfirming] = useState(false);
  const [planData, setPlanData] = useState<any>(null);
  const messagesEndRef = useRef<HTMLDivElement>(null);
  const textareaRef = useRef<HTMLTextAreaElement>(null);

  // Load conversation history on mount
  useEffect(() => {
    async function load() {
      try {
        const history = await getConversationHistory(projectId, taskId);
        setMessages(history);

        // Check if last assistant message has a confirmed analysis
        const lastAssistant = [...history].reverse().find((m) => m.role === "assistant");
        if (lastAssistant?.metadata) {
          const meta = lastAssistant.metadata as any;
          if (meta.status === "confirmed" || meta.title) {
            // There's a plan in metadata
            setPlanData(meta);
          }
        }
      } catch {
        // First message — no history yet
      }
    }
    load();
  }, [projectId, taskId]);

  // Auto-scroll to bottom
  useEffect(() => {
    messagesEndRef.current?.scrollIntoView({ behavior: "smooth" });
  }, [messages, isThinking]);

  const handleSend = useCallback(async () => {
    const content = input.trim();
    if (!content || isThinking) return;

    setInput("");
    setIsThinking(true);

    // Optimistic: add user message immediately
    const userMsg: Message = {
      id: Date.now(),
      taskId,
      role: "user",
      content,
      tokensUsed: 0,
      createdAt: new Date().toISOString(),
    };
    setMessages((prev) => [...prev, userMsg]);

    try {
      // Phase 1: synchronous call (not SSE yet)
      const aiMsg = await sendMessage(projectId, taskId, content);
      setMessages((prev) => [...prev, aiMsg]);

      // Check if AI returned a confirmed analysis or plan
      if (aiMsg.metadata) {
        const meta = aiMsg.metadata as any;
        if (meta.status === "confirmed" || meta.title) {
          setPlanData(meta);
        }
      }
    } catch (err) {
      const errMsg: Message = {
        id: Date.now(),
        taskId,
        role: "assistant",
        content: `出错了: ${(err as Error).message}`,
        tokensUsed: 0,
        createdAt: new Date().toISOString(),
      };
      setMessages((prev) => [...prev, errMsg]);
    } finally {
      setIsThinking(false);
    }
  }, [input, isThinking, projectId, taskId]);

  const handleKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === "Enter" && !e.shiftKey) {
      e.preventDefault();
      handleSend();
    }
  };

  const handleConfirm = async () => {
    setIsConfirming(true);
    try {
      await confirmPlan(projectId, taskId);
      onConfirmed?.();
    } catch (err) {
      alert(`确认失败: ${(err as Error).message}`);
    } finally {
      setIsConfirming(false);
    }
  };

  const handleModify = () => {
    setPlanData(null);
    textareaRef.current?.focus();
  };

  const handleCancel = () => {
    // Navigate back to task list
    window.history.back();
  };

  return (
    <div className="flex flex-col h-full">
      {/* Messages area */}
      <div className="flex-1 overflow-y-auto px-6 py-4 space-y-1">
        {messages.length === 0 && !isThinking && (
          <div className="flex items-center justify-center h-full text-zinc-600 text-sm">
            描述你的需求，AI 将帮你分析和规划实现方案
          </div>
        )}

        {messages.map((msg) => (
          <MessageBubble
            key={msg.id}
            role={msg.role as "user" | "assistant"}
            content={msg.content}
            timestamp={msg.createdAt}
          />
        ))}

        {isThinking && <ThinkingIndicator />}

        {/* Confirmation card */}
        {planData && planData.tasks && (
          <ConfirmationCard
            plan={planData}
            onConfirm={handleConfirm}
            onModify={handleModify}
            onCancel={handleCancel}
            isConfirming={isConfirming}
          />
        )}

        <div ref={messagesEndRef} />
      </div>

      {/* Input area */}
      <div className="border-t border-zinc-800 px-6 py-4 bg-[#0A0A12]">
        <div className="flex gap-3 items-end">
          <Textarea
            ref={textareaRef}
            value={input}
            onChange={(e) => setInput(e.target.value)}
            onKeyDown={handleKeyDown}
            placeholder="描述你的需求... (Enter 发送, Shift+Enter 换行)"
            className="flex-1 min-h-[44px] max-h-[200px] resize-none bg-[#12121A] border-zinc-800 text-zinc-200 placeholder:text-zinc-600 focus:border-[#8B5CF6] focus:ring-[#8B5CF6]/20"
            rows={1}
            disabled={isThinking}
          />
          <Button
            onClick={handleSend}
            disabled={!input.trim() || isThinking}
            className="bg-[#8B5CF6] hover:bg-[#7C3AED] text-white h-[44px] px-4"
          >
            <Send className="w-4 h-4" />
          </Button>
        </div>
      </div>
    </div>
  );
}
```

- [ ] **Step 7: 创建需求对话页面**

`forge-portal/app/(dashboard)/projects/[id]/tasks/new/page.tsx`:

```tsx
"use client";

import { useState, useEffect } from "react";
import { useParams, useRouter } from "next/navigation";
import { ArrowLeft, MessageSquarePlus } from "lucide-react";
import { Button } from "@/components/ui/button";
import { ChatPanel } from "@/components/chat/chat-panel";

export default function NewTaskPage() {
  const params = useParams();
  const router = useRouter();
  const projectId = Number(params.id);

  // Phase 1: Create a task in SUBMITTED status when page loads,
  // then use that taskId for the conversation.
  // In production, this would be a proper "draft task" concept.
  const [taskId, setTaskId] = useState<number | null>(null);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    async function createDraftTask() {
      try {
        const token = localStorage.getItem("token");
        const res = await fetch("/api/tasks", {
          method: "POST",
          headers: {
            "Content-Type": "application/json",
            Authorization: `Bearer ${token}`,
          },
          body: JSON.stringify({
            projectId,
            requirement: "", // Will be filled via conversation
            source: "WEB",
          }),
        });
        const json = await res.json();
        if (json.data?.id) {
          setTaskId(json.data.id);
        }
      } catch (err) {
        console.error("Failed to create draft task:", err);
      } finally {
        setLoading(false);
      }
    }
    createDraftTask();
  }, [projectId]);

  const handleConfirmed = () => {
    // Navigate to task detail page after confirmation
    if (taskId) {
      router.push(`/projects/${projectId}/tasks/${taskId}`);
    }
  };

  return (
    <div className="flex flex-col h-full">
      {/* Header */}
      <div className="flex items-center gap-3 px-6 py-4 border-b border-zinc-800">
        <Button
          variant="ghost"
          size="icon"
          onClick={() => router.back()}
          className="text-zinc-400 hover:text-zinc-200"
        >
          <ArrowLeft className="w-5 h-5" />
        </Button>
        <MessageSquarePlus className="w-5 h-5 text-[#8B5CF6]" />
        <h1 className="text-lg font-semibold text-zinc-100">需求对话</h1>
        <span className="text-xs text-zinc-600">
          描述你想要的功能，AI 将分析需求并生成实现方案
        </span>
      </div>

      {/* Chat area */}
      <div className="flex-1 overflow-hidden">
        {loading ? (
          <div className="flex items-center justify-center h-full text-zinc-500">
            初始化对话...
          </div>
        ) : taskId ? (
          <ChatPanel
            projectId={projectId}
            taskId={taskId}
            onConfirmed={handleConfirmed}
          />
        ) : (
          <div className="flex items-center justify-center h-full text-red-400">
            创建任务失败，请返回重试
          </div>
        )}
      </div>
    </div>
  );
}
```

- [ ] **Step 8: 验证前端编译**

```bash
cd forge-portal
npm run build
# 预期: 编译通过，无 TypeScript 错误
```

如果有 shadcn/ui 组件缺失（如 `Textarea`），需要先安装：
```bash
npx shadcn@latest add textarea
```

- [ ] **Step 9: Commit**

```bash
git add forge-portal/
git commit -m "feat(s6): add requirement dialog page with chat UI and confirmation card"
```

---

## Task 8: 前端 — 任务详情更新（实时步骤 + AI 输出）

**Files:**
- Create: `forge-portal/components/task/step-timeline.tsx`
- Create: `forge-portal/components/task/step-output.tsx`
- Modify: `forge-portal/app/(dashboard)/projects/[id]/tasks/[taskId]/page.tsx`

- [ ] **Step 1: 创建步骤时间线组件**

`forge-portal/components/task/step-timeline.tsx`:

```tsx
"use client";

import { cn } from "@/lib/utils";
import {
  Search,
  ListChecks,
  Code2,
  ShieldCheck,
  CheckCircle2,
  Loader2,
  XCircle,
  Clock,
} from "lucide-react";

interface Step {
  name: string;
  stepType: string;
  status: string; // PENDING | RUNNING | COMPLETED | FAILED
  output?: Record<string, unknown>;
  startedAt?: string;
  completedAt?: string;
  durationMs?: number;
}

interface StepTimelineProps {
  steps: Step[];
  activeStep?: string;
  onStepClick?: (stepType: string) => void;
}

const stepIcons: Record<string, typeof Search> = {
  ANALYZE: Search,
  PLAN: ListChecks,
  GENERATE: Code2,
  REVIEW: ShieldCheck,
};

const statusStyles: Record<string, { dot: string; text: string; line: string }> = {
  PENDING: {
    dot: "bg-zinc-700 border-zinc-600",
    text: "text-zinc-500",
    line: "bg-zinc-800",
  },
  RUNNING: {
    dot: "bg-[#8B5CF6] border-[#8B5CF6] animate-pulse",
    text: "text-zinc-200",
    line: "bg-[#8B5CF6]/30",
  },
  COMPLETED: {
    dot: "bg-emerald-500 border-emerald-500",
    text: "text-zinc-300",
    line: "bg-emerald-500/30",
  },
  FAILED: {
    dot: "bg-red-500 border-red-500",
    text: "text-red-400",
    line: "bg-red-500/30",
  },
};

function StatusIcon({ status }: { status: string }) {
  switch (status) {
    case "RUNNING":
      return <Loader2 className="w-3.5 h-3.5 animate-spin text-[#8B5CF6]" />;
    case "COMPLETED":
      return <CheckCircle2 className="w-3.5 h-3.5 text-emerald-400" />;
    case "FAILED":
      return <XCircle className="w-3.5 h-3.5 text-red-400" />;
    default:
      return <Clock className="w-3.5 h-3.5 text-zinc-600" />;
  }
}

export function StepTimeline({ steps, activeStep, onStepClick }: StepTimelineProps) {
  return (
    <div className="space-y-0">
      {steps.map((step, index) => {
        const styles = statusStyles[step.status] || statusStyles.PENDING;
        const Icon = stepIcons[step.stepType] || Search;
        const isActive = activeStep === step.stepType;
        const isLast = index === steps.length - 1;

        return (
          <div key={step.stepType} className="flex gap-3">
            {/* Timeline column */}
            <div className="flex flex-col items-center">
              <div
                className={cn(
                  "w-8 h-8 rounded-full border-2 flex items-center justify-center",
                  styles.dot
                )}
              >
                <StatusIcon status={step.status} />
              </div>
              {!isLast && <div className={cn("w-0.5 flex-1 min-h-[24px]", styles.line)} />}
            </div>

            {/* Content column */}
            <div
              className={cn(
                "flex-1 pb-6 cursor-pointer",
                isActive && "bg-[#12121A] -mx-3 px-3 rounded-lg"
              )}
              onClick={() => onStepClick?.(step.stepType)}
            >
              <div className="flex items-center gap-2">
                <Icon className={cn("w-4 h-4", styles.text)} />
                <span className={cn("text-sm font-medium", styles.text)}>{step.name}</span>
              </div>

              {step.durationMs && step.status === "COMPLETED" && (
                <div className="text-xs text-zinc-600 mt-0.5">
                  耗时 {(step.durationMs / 1000).toFixed(1)}s
                </div>
              )}
            </div>
          </div>
        );
      })}
    </div>
  );
}
```

- [ ] **Step 2: 创建步骤输出展示组件**

`forge-portal/components/task/step-output.tsx`:

```tsx
"use client";

interface StepOutputProps {
  stepType: string;
  output: Record<string, unknown> | null;
  status: string;
}

export function StepOutput({ stepType, output, status }: StepOutputProps) {
  if (status === "PENDING") {
    return (
      <div className="text-sm text-zinc-600 italic">
        等待执行...
      </div>
    );
  }

  if (status === "RUNNING") {
    return (
      <div className="text-sm text-zinc-400">
        <div className="flex items-center gap-2">
          <div className="w-2 h-2 rounded-full bg-[#8B5CF6] animate-pulse" />
          正在执行...
        </div>
      </div>
    );
  }

  if (!output) {
    return <div className="text-sm text-zinc-600">无输出数据</div>;
  }

  // Render based on step type
  switch (stepType) {
    case "ANALYZE":
      return <AnalyzeOutput data={output} />;
    case "PLAN":
      return <PlanOutput data={output} />;
    case "GENERATE":
      return <GenerateOutput data={output} />;
    case "REVIEW":
      return <ReviewOutput data={output} />;
    default:
      return <pre className="text-xs text-zinc-400 overflow-auto">{JSON.stringify(output, null, 2)}</pre>;
  }
}

function AnalyzeOutput({ data }: { data: Record<string, unknown> }) {
  return (
    <div className="space-y-3 text-sm">
      {data.summary && (
        <div>
          <h4 className="text-zinc-400 text-xs font-medium mb-1">摘要</h4>
          <p className="text-zinc-300">{String(data.summary)}</p>
        </div>
      )}
      {Array.isArray(data.assumptions) && data.assumptions.length > 0 && (
        <div>
          <h4 className="text-zinc-400 text-xs font-medium mb-1">前提假设</h4>
          <ul className="list-disc list-inside text-zinc-400 text-xs space-y-0.5">
            {data.assumptions.map((a: string, i: number) => (
              <li key={i}>{a}</li>
            ))}
          </ul>
        </div>
      )}
    </div>
  );
}

function PlanOutput({ data }: { data: Record<string, unknown> }) {
  const tasks = Array.isArray(data.tasks) ? data.tasks : [];
  return (
    <div className="space-y-3 text-sm">
      {data.summary && <p className="text-zinc-300">{String(data.summary)}</p>}
      {tasks.length > 0 && (
        <div className="space-y-1.5">
          {tasks.map((t: any, i: number) => (
            <div key={i} className="bg-[#0A0A12] rounded p-2 text-xs">
              <span className="text-[#8B5CF6] font-mono">#{t.order || i + 1}</span>{" "}
              <span className="text-zinc-300">{t.title}</span>
              {t.estimate_hours && (
                <span className="text-zinc-600 ml-2">{t.estimate_hours}h</span>
              )}
            </div>
          ))}
        </div>
      )}
    </div>
  );
}

function GenerateOutput({ data }: { data: Record<string, unknown> }) {
  const files = Array.isArray(data.files) ? data.files : [];
  return (
    <div className="space-y-3 text-sm">
      <div className="text-zinc-400 text-xs">
        生成 {files.length} 个文件
      </div>
      {files.map((f: any, i: number) => (
        <div key={i} className="bg-[#0A0A12] rounded overflow-hidden">
          <div className="px-3 py-1.5 bg-zinc-900 text-xs font-mono text-zinc-400 border-b border-zinc-800">
            {f.path} <span className="text-zinc-600">({f.action})</span>
          </div>
          <pre className="p-3 text-xs text-zinc-300 overflow-x-auto max-h-[300px]">
            <code>{f.content?.slice(0, 2000)}{f.content?.length > 2000 ? "\n... (truncated)" : ""}</code>
          </pre>
        </div>
      ))}
    </div>
  );
}

function ReviewOutput({ data }: { data: Record<string, unknown> }) {
  const passed = data.passed as boolean;
  const score = data.score as number;
  const findings = Array.isArray(data.findings) ? data.findings : [];

  return (
    <div className="space-y-3 text-sm">
      <div className="flex items-center gap-4">
        <div className={`text-lg font-bold ${passed ? "text-emerald-400" : "text-red-400"}`}>
          {passed ? "PASSED" : "FAILED"}
        </div>
        <div className="text-zinc-400">
          Score: <span className="text-zinc-200 font-mono">{score}/100</span>
        </div>
      </div>
      {data.summary && <p className="text-zinc-400">{String(data.summary)}</p>}
      {findings.length > 0 && (
        <div className="space-y-1.5">
          {findings.map((f: any, i: number) => (
            <div key={i} className="bg-[#0A0A12] rounded p-2 text-xs flex gap-2">
              <span
                className={`font-medium ${
                  f.severity === "critical"
                    ? "text-red-400"
                    : f.severity === "major"
                    ? "text-amber-400"
                    : "text-zinc-500"
                }`}
              >
                {f.severity}
              </span>
              <span className="text-zinc-400">{f.message}</span>
            </div>
          ))}
        </div>
      )}
    </div>
  );
}
```

- [ ] **Step 3: 修改任务详情页**

修改 `forge-portal/app/(dashboard)/projects/[id]/tasks/[taskId]/page.tsx`。

这个页面需要：
- 从 API 获取 task 详情和 steps 数据
- 用 StepTimeline 展示真实步骤进度
- 点击步骤展示 StepOutput（AI 输出内容）
- 轮询或 SSE 获取实时更新（Phase 1 用轮询）

```tsx
"use client";

import { useState, useEffect, useCallback } from "react";
import { useParams, useRouter } from "next/navigation";
import { ArrowLeft, RefreshCw } from "lucide-react";
import { Button } from "@/components/ui/button";
import { StepTimeline } from "@/components/task/step-timeline";
import { StepOutput } from "@/components/task/step-output";

interface Task {
  id: number;
  title: string;
  requirement: string;
  status: string;
  branchName?: string;
  createdAt: string;
}

interface TaskStep {
  id: number;
  name: string;
  stepType: string;
  status: string;
  output: Record<string, unknown> | null;
  startedAt?: string;
  completedAt?: string;
  durationMs?: number;
}

export default function TaskDetailPage() {
  const params = useParams();
  const router = useRouter();
  const projectId = Number(params.id);
  const taskId = Number(params.taskId);

  const [task, setTask] = useState<Task | null>(null);
  const [steps, setSteps] = useState<TaskStep[]>([]);
  const [activeStep, setActiveStep] = useState<string>("");
  const [loading, setLoading] = useState(true);

  const fetchData = useCallback(async () => {
    const token = localStorage.getItem("token");
    const headers = { Authorization: `Bearer ${token}` };

    try {
      const [taskRes, stepsRes] = await Promise.all([
        fetch(`/api/tasks/${taskId}`, { headers }),
        fetch(`/api/tasks/${taskId}/steps`, { headers }),
      ]);

      const taskJson = await taskRes.json();
      const stepsJson = await stepsRes.json();

      if (taskJson.data) setTask(taskJson.data);
      if (stepsJson.data) {
        setSteps(stepsJson.data);
        // Auto-select the running or last completed step
        const running = stepsJson.data.find((s: TaskStep) => s.status === "RUNNING");
        if (running) {
          setActiveStep(running.stepType);
        } else if (!activeStep) {
          const completed = [...stepsJson.data].reverse().find((s: TaskStep) => s.status === "COMPLETED");
          if (completed) setActiveStep(completed.stepType);
        }
      }
    } catch (err) {
      console.error("Failed to fetch task data:", err);
    } finally {
      setLoading(false);
    }
  }, [taskId, activeStep]);

  // Initial load
  useEffect(() => {
    fetchData();
  }, [fetchData]);

  // Auto-refresh while task is in progress
  useEffect(() => {
    if (!task) return;
    const inProgress = ["ANALYZING", "PLANNING", "GENERATING", "REVIEWING"].includes(task.status);
    if (!inProgress) return;

    const interval = setInterval(fetchData, 3000); // Poll every 3s
    return () => clearInterval(interval);
  }, [task, fetchData]);

  const activeStepData = steps.find((s) => s.stepType === activeStep);

  const statusLabels: Record<string, string> = {
    SUBMITTED: "已提交",
    ANALYZING: "分析中",
    PLANNING: "规划中",
    PLAN_CONFIRMED: "方案已确认",
    GENERATING: "生成中",
    REVIEWING: "审查中",
    COMPLETED: "已完成",
    FAILED: "失败",
    CANCELLED: "已取消",
  };

  if (loading) {
    return (
      <div className="flex items-center justify-center h-full text-zinc-500">
        加载中...
      </div>
    );
  }

  if (!task) {
    return (
      <div className="flex items-center justify-center h-full text-red-400">
        任务不存在
      </div>
    );
  }

  return (
    <div className="flex flex-col h-full">
      {/* Header */}
      <div className="flex items-center justify-between px-6 py-4 border-b border-zinc-800">
        <div className="flex items-center gap-3">
          <Button
            variant="ghost"
            size="icon"
            onClick={() => router.back()}
            className="text-zinc-400 hover:text-zinc-200"
          >
            <ArrowLeft className="w-5 h-5" />
          </Button>
          <div>
            <h1 className="text-lg font-semibold text-zinc-100">
              {task.title || `任务 #${task.id}`}
            </h1>
            <div className="flex items-center gap-3 text-xs text-zinc-500">
              <span className={task.status === "COMPLETED" ? "text-emerald-400" : task.status === "FAILED" ? "text-red-400" : "text-[#8B5CF6]"}>
                {statusLabels[task.status] || task.status}
              </span>
              {task.branchName && (
                <span className="font-mono text-zinc-600">{task.branchName}</span>
              )}
            </div>
          </div>
        </div>
        <Button variant="ghost" size="icon" onClick={fetchData} className="text-zinc-400">
          <RefreshCw className="w-4 h-4" />
        </Button>
      </div>

      {/* Content: two columns */}
      <div className="flex-1 flex overflow-hidden">
        {/* Left: Step Timeline */}
        <div className="w-[280px] border-r border-zinc-800 p-4 overflow-y-auto">
          <h3 className="text-xs font-medium text-zinc-500 mb-3 uppercase tracking-wider">
            执行步骤
          </h3>
          <StepTimeline
            steps={steps}
            activeStep={activeStep}
            onStepClick={setActiveStep}
          />
        </div>

        {/* Right: Step Output */}
        <div className="flex-1 p-6 overflow-y-auto">
          {activeStepData ? (
            <div>
              <h3 className="text-sm font-medium text-zinc-300 mb-3">
                {activeStepData.name}
              </h3>
              <StepOutput
                stepType={activeStepData.stepType}
                output={activeStepData.output}
                status={activeStepData.status}
              />
            </div>
          ) : (
            <div className="text-sm text-zinc-600">
              {steps.length === 0
                ? "任务尚未开始执行"
                : "选择左侧步骤查看详情"
              }
            </div>
          )}

          {/* Requirement context */}
          <div className="mt-8 pt-6 border-t border-zinc-800">
            <h4 className="text-xs font-medium text-zinc-500 mb-2 uppercase tracking-wider">
              原始需求
            </h4>
            <p className="text-sm text-zinc-400 whitespace-pre-wrap">
              {task.requirement || "(通过对话提交)"}
            </p>
          </div>
        </div>
      </div>
    </div>
  );
}
```

- [ ] **Step 4: 验证前端编译**

```bash
cd forge-portal
npm run build
# 预期: 编译通过
```

- [ ] **Step 5: Commit**

```bash
git add forge-portal/
git commit -m "feat(s6): update task detail page with real step timeline and AI output display"
```

---

## 端到端验证

完成全部 8 个 Task 后，按以下步骤验证端到端闭环：

### 准备

```bash
# 1. 启动基础设施
docker compose -f docker-compose.dev.yml up -d

# 2. 启动 forge-core
cd forge-core && go run ./cmd/forge-core

# 3. 启动 AI Worker (需要 API key)
cd ai-worker
source .venv/bin/activate
export ANTHROPIC_API_KEY=sk-ant-xxx  # 你的 API key
python -m src.worker

# 4. 启动前端
cd forge-portal && npm run dev
```

### 测试流程

1. 浏览器打开 `http://localhost:3000`，登录 `admin / admin123`
2. 进入某个项目（S2 创建的）
3. 点击"新建需求"进入对话页面
4. 输入：`创建一个用户管理的 REST API，包含 CRUD 操作和分页查询`
5. 等待 AI 回复（分析需求，可能提出澄清问题）
6. 如果 AI 提问，回答后再等待
7. AI 返回确认卡片 → 检查任务拆解和评估
8. 点击"确认执行"
9. 跳转到任务详情页，观察：
   - 步骤时间线从 PLANNING → GENERATING → REVIEWING 推进
   - 每个步骤有 AI 输出内容
   - 最终状态为 COMPLETED
10. 检查生成的代码文件列表和内容

### 验证要点

- [ ] 对话消息正确保存和显示（用户在右，AI 在左）
- [ ] AI 能理解简单需求并提出合理的澄清问题
- [ ] 确认卡片正确展示任务拆解和评估
- [ ] 确认后 Temporal Workflow 正确启动
- [ ] 任务详情页实时显示步骤进度
- [ ] AI 生成的代码基本合理（不要求完美）
- [ ] Review 步骤能检测到基本问题
- [ ] 如果 Review 失败，能自动重新生成并重新 Review
- [ ] 全流程无崩溃，错误有合理提示

---

## 已知限制 (Phase 1)

以下限制将在后续切片中解决：

| 限制 | 计划解决 |
|------|---------|
| 非真正 token 流式输出 | S7+: Redis pub/sub 真流式 |
| GitHub 提交未实现 | S7: devops-worker 集成 |
| 无外部 Lint/安全扫描 | S7+: constraint-worker 集成 |
| 简单的 JSON 解析（无 retry） | S8: 增强结构化输出解析 |
| 无 token 预算管理 | S8: 计费 + 限额 |
| 无多文件并行生成 | S9: 并行 Activity |
| Context Builder 从 API 拉取较慢 | S8: 缓存层 |
| 轮询刷新步骤状态 | S7: WebSocket 或 SSE 推送 |
