# Forge AI Worker — OpenHarness 架构重构计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 将 Forge AI Worker 从固定管线架构重构为 OpenHarness 风格的可编程 Harness 架构，并增加 Web 端 Claude Code 交互能力（流式对话、工具可视化、多轮 Agent Loop）。

**Architecture:** 参照 HKUDS/OpenHarness 的核心子系统设计（Engine + Tools + Skills + Hooks + Permissions），将其架构映射到 Forge 的 Temporal + Go + Python 技术栈上。保留 Temporal 作为进程间编排器，在 Python 层建立可编程 Harness 基础设施。前端增加实时 Agent 交互界面。MCP/Coordinator/Plugins/Memory 作为后续扩展，本期不实现。

**Dependencies added:** `pydantic>=2.0.0`, `pyyaml>=6.0`

**Tech Stack:** Python 3.12 + Pydantic + asyncio + Temporal SDK + Redis Pub/Sub + SSE + Next.js + shadcn/ui

---

## File Structure

```
ai-worker/
  src/
    openharness/                     # 新增：Harness 基础设施（命名致敬 OpenHarness）
      __init__.py                    # 公共 API 导出
      engine/
        __init__.py
        query_engine.py              # QueryEngine — 有状态会话引擎
        query.py                     # run_query() — 核心 Agent Loop
        messages.py                  # ConversationMessage, ContentBlock 类型
        stream_events.py             # StreamEvent 事件类型
        cost_tracker.py              # 累计 Token 成本追踪
      tools/
        __init__.py                  # create_default_tool_registry()
        base.py                      # BaseTool, ToolRegistry, ToolResult
        context_tools.py             # 5 个现有 Context 工具迁移
      hooks/
        __init__.py
        events.py                    # HookEvent 枚举
        schemas.py                   # Hook 定义模型
        executor.py                  # HookExecutor 执行引擎
        loader.py                    # HookRegistry 加载
        builtin/
          __init__.py
          constraint_hook.py         # ReviewRule 约束执行
          cost_hook.py               # 成本追踪钩子
          logging_hook.py            # 结构化日志钩子
      skills/
        __init__.py
        types.py                     # SkillDefinition
        loader.py                    # Markdown 解析 + 加载
        registry.py                  # SkillRegistry
      permissions/
        __init__.py
        checker.py                   # PermissionChecker
        modes.py                     # PermissionMode 枚举
      api/
        __init__.py
        client.py                    # SupportsStreamingMessages 协议 + UsageSnapshot
        providers/
          __init__.py
          anthropic.py               # Anthropic 客户端
          openai_compat.py           # OpenAI 兼容客户端（DashScope/DeepSeek）
    agents/                          # 保留：Agent 特化（改为薄封装）
      base.py                        # 改造：委托给 QueryEngine
      analyst.py                     # 改造：从 Skill 加载 prompt
      coder.py
      planner.py
      reviewer.py
      test_writer.py
      profiler.py
    activities/                      # 保留：Temporal Activities
      analyze.py                     # 改造：使用 QueryEngine
      generate.py
      plan.py
      profile.py
      review.py
    context/                         # 保留：向后兼容
      builder.py                     # 改造：集成 SkillLoader
      tools.py                       # 改造：委托给 ToolRegistry
      cache.py                       # 保留
    models/                          # 改造：集成到 api/ 模块
      client.py                      # 改造：实现 SupportsStreamingMessages
      router.py                      # 改造：使用 api/ 客户端
    worker.py                        # 改造：启动 Harness 基础设施
  skills/                            # 新增：Agent 技能 Markdown 文件
    analyze.md
    generate.md
    review.md
    plan.md
    test-writing.md
    profile.md
  tests/
    test_tool_registry.py            # 新增
    test_hook_executor.py            # 新增
    test_query_engine.py             # 新增
    test_skill_loader.py             # 新增
    test_permission_checker.py       # 新增
    test_api_providers.py             # 新增
    test_agent_loop.py               # 改造
    test_analyze_flow.py             # 改造

forge-portal/                        # 前端改造
  app/(dashboard)/projects/[id]/
    agent/                           # 新增：Agent 交互页面
      page.tsx                       # Claude Code 风格交互界面
  components/
    agent/                           # 新增：Agent UI 组件
      agent-chat.tsx                 # 流式对话界面
      tool-execution.tsx             # 工具执行可视化
      agent-status.tsx               # Agent 状态面板
      permission-dialog.tsx          # 权限确认对话框
```

---

### Task 1: Engine — 消息模型 + 流式事件

**Files:**
- Create: `ai-worker/src/openharness/__init__.py`
- Create: `ai-worker/src/openharness/engine/__init__.py`
- Create: `ai-worker/src/openharness/engine/messages.py`
- Create: `ai-worker/src/openharness/engine/stream_events.py`
- Create: `ai-worker/src/openharness/engine/cost_tracker.py`
- Create: `ai-worker/src/openharness/api/__init__.py`
- Create: `ai-worker/src/openharness/api/usage.py`
- Test: `ai-worker/tests/test_messages.py`

- [ ] **Step 1: Write failing test for ConversationMessage**

```python
# tests/test_messages.py
import pytest
from src.openharness.engine.messages import (
    TextBlock, ToolUseBlock, ToolResultBlock, ConversationMessage
)

def test_text_block_creation():
    block = TextBlock(text="hello")
    assert block.type == "text"
    assert block.text == "hello"

def test_tool_use_block_creation():
    block = ToolUseBlock(name="bash", input={"command": "ls"})
    assert block.type == "tool_use"
    assert block.name == "bash"
    assert block.id.startswith("toolu_")

def test_conversation_message_from_user():
    msg = ConversationMessage.from_user_text("hello")
    assert msg.role == "user"
    assert msg.text == "hello"
    assert len(msg.content) == 1

def test_conversation_message_tool_uses():
    msg = ConversationMessage(role="assistant", content=[
        TextBlock(text="I'll run that"),
        ToolUseBlock(name="bash", input={"command": "ls"}),
    ])
    assert len(msg.tool_uses) == 1
    assert msg.tool_uses[0].name == "bash"

def test_tool_result_block():
    block = ToolResultBlock(tool_use_id="toolu_abc", content="file1.py\nfile2.py")
    assert block.type == "tool_result"
    assert not block.is_error

def test_message_to_api_param():
    msg = ConversationMessage.from_user_text("test")
    param = msg.to_api_param()
    assert param["role"] == "user"
    assert isinstance(param["content"], list)
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd ai-worker && python -m pytest tests/test_messages.py -v`
Expected: FAIL with ModuleNotFoundError

- [ ] **Step 3: Implement messages.py**

```python
# src/openharness/engine/messages.py
"""Conversation message types — Anthropic-compatible content blocks."""

from __future__ import annotations

from typing import Any, Literal
from uuid import uuid4

from pydantic import BaseModel, Field

class TextBlock(BaseModel):
    type: Literal["text"] = "text"
    text: str

class ToolUseBlock(BaseModel):
    type: Literal["tool_use"] = "tool_use"
    id: str = Field(default_factory=lambda: f"toolu_{uuid4().hex[:24]}")
    name: str
    input: dict[str, Any] = Field(default_factory=dict)

class ToolResultBlock(BaseModel):
    type: Literal["tool_result"] = "tool_result"
    tool_use_id: str
    content: str
    is_error: bool = False

ContentBlock = TextBlock | ToolUseBlock | ToolResultBlock

class ConversationMessage(BaseModel):
    role: Literal["user", "assistant"]
    content: list[ContentBlock] = Field(default_factory=list)

    @classmethod
    def from_user_text(cls, text: str) -> "ConversationMessage":
        return cls(role="user", content=[TextBlock(text=text)])

    @property
    def text(self) -> str:
        return "".join(b.text for b in self.content if isinstance(b, TextBlock))

    @property
    def tool_uses(self) -> list[ToolUseBlock]:
        return [b for b in self.content if isinstance(b, ToolUseBlock)]

    def to_api_param(self) -> dict[str, Any]:
        return {
            "role": self.role,
            "content": [b.model_dump() for b in self.content],
        }
```

- [ ] **Step 4: Create package __init__ files**

```python
# src/openharness/__init__.py
"""Forge Harness Infrastructure — inspired by OpenHarness (HKUDS)."""

# src/openharness/engine/__init__.py
from .messages import ConversationMessage, TextBlock, ToolUseBlock, ToolResultBlock
from .stream_events import StreamEvent
from .cost_tracker import CostTracker
```

- [ ] **Step 5: Implement stream_events.py**

```python
# src/openharness/engine/stream_events.py
"""Stream events emitted by the query engine."""

from __future__ import annotations
from dataclasses import dataclass, field
from typing import Any
from .messages import ConversationMessage

@dataclass(frozen=True)
class AssistantTextDelta:
    text: str

@dataclass(frozen=True)
class AssistantTurnComplete:
    message: ConversationMessage
    usage: UsageSnapshot

@dataclass(frozen=True)
class ToolExecutionStarted:
    tool_name: str
    tool_input: dict[str, Any]

@dataclass(frozen=True)
class ToolExecutionCompleted:
    tool_name: str
    output: str
    is_error: bool = False

@dataclass(frozen=True)
class ErrorEvent:
    message: str
    recoverable: bool = True

StreamEvent = (
    AssistantTextDelta
    | AssistantTurnComplete
    | ToolExecutionStarted
    | ToolExecutionCompleted
    | ErrorEvent
)
```

- [ ] **Step 6: Implement usage.py + cost_tracker.py**

```python
# src/openharness/api/__init__.py
# src/openharness/api/usage.py
from pydantic import BaseModel

class UsageSnapshot(BaseModel):
    input_tokens: int = 0
    output_tokens: int = 0

    @property
    def total_tokens(self) -> int:
        return self.input_tokens + self.output_tokens

# src/openharness/engine/cost_tracker.py
from ..api.usage import UsageSnapshot

class CostTracker:
    def __init__(self) -> None:
        self._usage = UsageSnapshot()

    def add(self, usage: UsageSnapshot) -> None:
        self._usage = UsageSnapshot(
            input_tokens=self._usage.input_tokens + usage.input_tokens,
            output_tokens=self._usage.output_tokens + usage.output_tokens,
        )

    @property
    def total(self) -> UsageSnapshot:
        return self._usage

    def reset(self) -> None:
        self._usage = UsageSnapshot()
```

- [ ] **Step 7: Run tests to verify pass**

Run: `cd ai-worker && python -m pytest tests/test_messages.py -v`
Expected: All PASS

- [ ] **Step 8: Commit**

```bash
git add ai-worker/src/openharness/ ai-worker/tests/test_messages.py
git commit -m "feat(harness): message models, stream events, cost tracker — OpenHarness engine foundation"
```

---

### Task 2: Tools — BaseTool + ToolRegistry + 迁移 ContextTools

**Files:**
- Create: `ai-worker/src/openharness/tools/__init__.py`
- Create: `ai-worker/src/openharness/tools/base.py`
- Create: `ai-worker/src/openharness/tools/context_tools.py`
- Test: `ai-worker/tests/test_tool_registry.py`

- [ ] **Step 1: Write failing test for ToolRegistry**

```python
# tests/test_tool_registry.py
import pytest
from src.openharness.tools.base import BaseTool, ToolRegistry, ToolResult, ToolExecutionContext
from pydantic import BaseModel
from pathlib import Path

class EchoInput(BaseModel):
    text: str

class EchoTool(BaseTool):
    name = "echo"
    description = "Echo input text"
    input_model = EchoInput

    async def execute(self, arguments: EchoInput, context: ToolExecutionContext) -> ToolResult:
        return ToolResult(output=arguments.text)

    def is_read_only(self, arguments: EchoInput) -> bool:
        return True

@pytest.mark.asyncio
async def test_registry_register_and_get():
    registry = ToolRegistry()
    tool = EchoTool()
    registry.register(tool)
    assert registry.get("echo") is tool
    assert registry.get("nonexistent") is None

def test_registry_list_tools():
    registry = ToolRegistry()
    registry.register(EchoTool())
    tools = registry.list_tools()
    assert len(tools) == 1
    assert tools[0].name == "echo"

def test_registry_to_api_schema():
    registry = ToolRegistry()
    registry.register(EchoTool())
    schemas = registry.to_api_schema()
    assert len(schemas) == 1
    assert schemas[0]["name"] == "echo"
    assert "input_schema" in schemas[0]

@pytest.mark.asyncio
async def test_tool_execution():
    tool = EchoTool()
    ctx = ToolExecutionContext(cwd=Path("."))
    result = await tool.execute(EchoInput(text="hello"), ctx)
    assert result.output == "hello"
    assert not result.is_error

def test_tool_read_only():
    tool = EchoTool()
    assert tool.is_read_only(EchoInput(text="x")) is True
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd ai-worker && python -m pytest tests/test_tool_registry.py -v`
Expected: FAIL

- [ ] **Step 3: Implement base.py**

```python
# src/openharness/tools/base.py
"""Tool abstractions — BaseTool, ToolRegistry, ToolResult."""

from __future__ import annotations

from abc import ABC, abstractmethod
from dataclasses import dataclass, field
from pathlib import Path
from typing import Any

from pydantic import BaseModel


@dataclass
class ToolExecutionContext:
    cwd: Path
    metadata: dict[str, Any] = field(default_factory=dict)


@dataclass(frozen=True)
class ToolResult:
    output: str
    is_error: bool = False
    metadata: dict[str, Any] = field(default_factory=dict)


class BaseTool(ABC):
    name: str
    description: str
    input_model: type[BaseModel]

    @abstractmethod
    async def execute(self, arguments: BaseModel, context: ToolExecutionContext) -> ToolResult:
        ...

    def is_read_only(self, arguments: BaseModel) -> bool:
        return False

    def to_api_schema(self) -> dict[str, Any]:
        return {
            "name": self.name,
            "description": self.description,
            "input_schema": self.input_model.model_json_schema(),
        }


class ToolRegistry:
    def __init__(self) -> None:
        self._tools: dict[str, BaseTool] = {}

    def register(self, tool: BaseTool) -> None:
        self._tools[tool.name] = tool

    def get(self, name: str) -> BaseTool | None:
        return self._tools.get(name)

    def list_tools(self) -> list[BaseTool]:
        return list(self._tools.values())

    def to_api_schema(self) -> list[dict[str, Any]]:
        return [t.to_api_schema() for t in self._tools.values()]
```

- [ ] **Step 4: Run tests to verify pass**

Run: `cd ai-worker && python -m pytest tests/test_tool_registry.py -v`
Expected: All PASS

- [ ] **Step 5: Migrate 5 ContextTools to BaseTool subclasses**

```python
# src/openharness/tools/context_tools.py
"""Forge context query tools — migrated from src/context/tools.py."""

from __future__ import annotations
import json
import logging
from typing import Any

import httpx
from pydantic import BaseModel

from .base import BaseTool, ToolExecutionContext, ToolResult
from src.config import settings

logger = logging.getLogger(__name__)


class QueryApiCatalogInput(BaseModel):
    keyword: str

class QueryApiCatalogTool(BaseTool):
    name = "query_api_catalog"
    description = "查询项目的 API 接口清单。传入关键词过滤。"
    input_model = QueryApiCatalogInput

    def __init__(self, profiles: dict):
        self._profiles = profiles

    def is_read_only(self, arguments: QueryApiCatalogInput) -> bool:
        return True

    async def execute(self, arguments: QueryApiCatalogInput, context: ToolExecutionContext) -> ToolResult:
        return ToolResult(output=_search_profile(
            self._profiles, "api_catalog", arguments.keyword.lower(),
            "endpoints", ["path", "method", "handler"]
        ))


class QueryDbSchemaInput(BaseModel):
    table_name: str

class QueryDbSchemaTool(BaseTool):
    name = "query_db_schema"
    description = "查询项目的数据库表结构。"
    input_model = QueryDbSchemaInput

    def __init__(self, profiles: dict):
        self._profiles = profiles

    def is_read_only(self, arguments: QueryDbSchemaInput) -> bool:
        return True

    async def execute(self, arguments: QueryDbSchemaInput, context: ToolExecutionContext) -> ToolResult:
        return ToolResult(output=_search_profile(
            self._profiles, "db_schema", arguments.table_name.lower(),
            "tables", ["name", "columns"]
        ))


class QueryBusinessRulesInput(BaseModel):
    domain: str

class QueryBusinessRulesTool(BaseTool):
    name = "query_business_rules"
    description = "查询项目的业务规则约束。"
    input_model = QueryBusinessRulesInput

    def __init__(self, profiles: dict):
        self._profiles = profiles

    def is_read_only(self, arguments: QueryBusinessRulesInput) -> bool:
        return True

    async def execute(self, arguments: QueryBusinessRulesInput, context: ToolExecutionContext) -> ToolResult:
        return ToolResult(output=_search_profile(
            self._profiles, "business_rules", arguments.domain.lower(),
            "rules", ["domain", "rule", "source"]
        ))


class QueryModuleGraphInput(BaseModel):
    module_name: str

class QueryModuleGraphTool(BaseTool):
    name = "query_module_graph"
    description = "查询项目的模块依赖关系图。"
    input_model = QueryModuleGraphInput

    def __init__(self, profiles: dict):
        self._profiles = profiles

    def is_read_only(self, arguments: QueryModuleGraphInput) -> bool:
        return True

    async def execute(self, arguments: QueryModuleGraphInput, context: ToolExecutionContext) -> ToolResult:
        return ToolResult(output=_search_profile(
            self._profiles, "module_graph", arguments.module_name.lower(),
            "modules", ["name", "path", "depends_on"]
        ))


class ReadProjectFileInput(BaseModel):
    path: str

class ReadProjectFileTool(BaseTool):
    name = "read_project_file"
    description = "读取项目仓库中的源代码文件。"
    input_model = ReadProjectFileInput

    def __init__(self, project_id: int):
        self._project_id = project_id

    def is_read_only(self, arguments: ReadProjectFileInput) -> bool:
        return True

    async def execute(self, arguments: ReadProjectFileInput, context: ToolExecutionContext) -> ToolResult:
        if not arguments.path:
            return ToolResult(output="Error: file path is required", is_error=True)
        async with httpx.AsyncClient(timeout=10) as client:
            try:
                resp = await client.get(
                    f"{settings.forge_api_url}/api/projects/{self._project_id}/code/file",
                    params={"path": arguments.path, "ref": "main"},
                    headers={"Authorization": f"Bearer {settings.forge_api_token}"},
                )
            except Exception as e:
                return ToolResult(output=f"API connection error: {e}", is_error=True)
        if resp.status_code == 200:
            content = resp.json().get("data", {}).get("content", "")
            if len(content) > 20000:
                content = content[:20000] + f"\n\n... [truncated at 20000 chars]"
            return ToolResult(output=content or "File is empty")
        elif resp.status_code == 404:
            return ToolResult(output=f"File {arguments.path} not found", is_error=True)
        else:
            return ToolResult(output=f"HTTP {resp.status_code}", is_error=True)


def register_context_tools(registry, profiles: dict, project_id: int) -> None:
    """Register all 5 context query tools into the registry."""
    registry.register(QueryApiCatalogTool(profiles))
    registry.register(QueryDbSchemaTool(profiles))
    registry.register(QueryBusinessRulesTool(profiles))
    registry.register(QueryModuleGraphTool(profiles))
    registry.register(ReadProjectFileTool(project_id))


def _search_profile(profiles: dict, dimension: str, keyword: str, items_key: str, match_fields: list[str]) -> str:
    """Search within a profile dimension by keyword matching."""
    data = profiles.get(dimension, {})
    if not data:
        return f"No {dimension} data available. Use read_project_file to inspect code directly."
    items = data.get(items_key, [])
    if not items:
        return f"{dimension} data is empty."
    if keyword:
        results = [i for i in items if keyword in json.dumps(i, ensure_ascii=False).lower()]
    else:
        results = items
    if not results:
        return f"No {dimension} data matching '{keyword}'. Total: {len(items)} entries."
    if len(results) > 20:
        results = results[:20]
    return json.dumps(results, ensure_ascii=False, indent=2)
```

- [ ] **Step 6: Run all tests**

Run: `cd ai-worker && python -m pytest tests/test_tool_registry.py -v`
Expected: All PASS

- [ ] **Step 7: Commit**

```bash
git add ai-worker/src/openharness/tools/ ai-worker/tests/test_tool_registry.py
git commit -m "feat(harness): BaseTool + ToolRegistry + context tools migration"
```

---

### Task 3: Hooks — HookEvent + HookExecutor + 内置约束钩子

**Files:**
- Create: `ai-worker/src/openharness/hooks/__init__.py`
- Create: `ai-worker/src/openharness/hooks/events.py`
- Create: `ai-worker/src/openharness/hooks/schemas.py`
- Create: `ai-worker/src/openharness/hooks/executor.py`
- Create: `ai-worker/src/openharness/hooks/loader.py`
- Create: `ai-worker/src/openharness/hooks/builtin/__init__.py`
- Create: `ai-worker/src/openharness/hooks/builtin/constraint_hook.py`
- Create: `ai-worker/src/openharness/hooks/builtin/cost_hook.py`
- Create: `ai-worker/src/openharness/hooks/builtin/logging_hook.py`
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

def test_hook_registry_register():
    registry = HookRegistry()
    hook = CommandHookDefinition(command="echo ok", matcher={"tool_name": "bash"})
    registry.register(HookEvent.PRE_TOOL_USE, hook)
    hooks = registry.get(HookEvent.PRE_TOOL_USE)
    assert len(hooks) == 1

def test_hook_result_not_blocked():
    result = HookResult(hook_type="command", success=True, output="ok")
    assert not result.blocked

def test_aggregated_result_blocked():
    r1 = HookResult(hook_type="command", success=True, output="ok")
    r2 = HookResult(hook_type="command", success=True, output="denied", blocked=True, reason="forbidden pattern")
    agg = AggregatedHookResult(results=[r1, r2])
    assert agg.blocked
    assert agg.reason == "forbidden pattern"

def test_aggregated_result_not_blocked():
    r1 = HookResult(hook_type="command", success=True, output="ok")
    agg = AggregatedHookResult(results=[r1])
    assert not agg.blocked
```

- [ ] **Step 2: Run test to verify fails**

Run: `cd ai-worker && python -m pytest tests/test_hook_executor.py -v`

- [ ] **Step 3: Implement events.py + schemas.py**

```python
# src/openharness/hooks/events.py
from enum import Enum

class HookEvent(str, Enum):
    PRE_TOOL_USE = "pre_tool_use"
    POST_TOOL_USE = "post_tool_use"
    PRE_GENERATION = "pre_generation"
    POST_GENERATION = "post_generation"

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

- [ ] **Step 4: Implement executor.py + loader.py**

```python
# src/openharness/hooks/loader.py
from __future__ import annotations
from .events import HookEvent
from .schemas import CommandHookDefinition

HookDefinition = CommandHookDefinition  # Union type for future expansion

class HookRegistry:
    def __init__(self) -> None:
        self._hooks: dict[HookEvent, list[HookDefinition]] = {}

    def register(self, event: HookEvent, hook: HookDefinition) -> None:
        self._hooks.setdefault(event, []).append(hook)

    def get(self, event: HookEvent) -> list[HookDefinition]:
        return self._hooks.get(event, [])

# src/openharness/hooks/executor.py
from __future__ import annotations
from dataclasses import dataclass, field
from typing import Any
from .events import HookEvent
from .loader import HookRegistry

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
                from .schemas import HookMatcher
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
        import asyncio, json, os
        env = os.environ.copy()
        env["FORGE_HOOK_EVENT"] = payload.get("event", "")
        env["FORGE_HOOK_PAYLOAD"] = json.dumps(payload)
        try:
            proc = await asyncio.create_subprocess_shell(
                hook.command, stdout=asyncio.subprocess.PIPE,
                stderr=asyncio.subprocess.PIPE, env=env,
            )
            stdout, stderr = await asyncio.wait_for(
                proc.communicate(), timeout=hook.timeout_seconds
            )
            output = stdout.decode() if stdout else ""
            success = proc.returncode == 0
            blocked = not success and hook.block_on_failure
            return HookResult(
                hook_type="command", success=success, output=output,
                blocked=blocked, reason=stderr.decode() if blocked else "",
            )
        except asyncio.TimeoutError:
            return HookResult(
                hook_type="command", success=False, output="",
                blocked=hook.block_on_failure, reason="Hook timed out",
            )
```

- [ ] **Step 5: Run tests**

Run: `cd ai-worker && python -m pytest tests/test_hook_executor.py -v`

- [ ] **Step 6: Implement built-in constraint hook**

```python
# src/openharness/hooks/builtin/constraint_hook.py
"""ReviewRule constraint enforcement hook.

Fetches ReviewRules from forge-core API, then executes them against
generated code:
- PATTERN → regex match
- AST → reserved for tree-sitter (future)
- AI_CHECK → reserved for AI review (future)
"""

from __future__ import annotations
import json, re, logging
from dataclasses import dataclass
from typing import Any

import httpx
from ..events import HookEvent
from ..executor import HookResult
from src.config import settings

logger = logging.getLogger(__name__)

@dataclass
class Violation:
    rule_name: str
    severity: str  # ERROR, WARNING, INFO
    message: str
    file_path: str | None = None
    line: int | None = None

class ConstraintEnforcementHook:
    """POST_GENERATION hook that checks code against ReviewRules."""

    event = HookEvent.POST_GENERATION
    priority = 10

    async def execute(self, payload: dict[str, Any]) -> HookResult:
        project_id = payload.get("project_id")
        code_content = payload.get("code_content", "")
        if not project_id or not code_content:
            return HookResult(hook_type="constraint", success=True, output="No code to check")

        rules = await self._fetch_rules(project_id)
        if not rules:
            return HookResult(hook_type="constraint", success=True, output="No rules configured")

        violations = []
        for rule in rules:
            if rule.get("rule_type") == "PATTERN":
                violations.extend(self._check_pattern(rule, code_content))

        if not violations:
            return HookResult(hook_type="constraint", success=True, output="All constraints passed")

        errors = [v for v in violations if v.severity == "ERROR"]
        output = self._format_violations(violations)

        return HookResult(
            hook_type="constraint",
            success=len(errors) == 0,
            output=output,
            blocked=len(errors) > 0,
            reason=f"{len(errors)} ERROR violations found" if errors else "",
        )

    async def _fetch_rules(self, project_id: int) -> list[dict]:
        try:
            async with httpx.AsyncClient(timeout=5) as client:
                resp = await client.get(
                    f"{settings.forge_api_url}/api/specs/effective/{project_id}",
                    headers={"Authorization": f"Bearer {settings.forge_api_token}"},
                )
                if resp.status_code == 200:
                    data = resp.json().get("data", {})
                    return [r for r in data.get("rules", []) if r.get("enabled", True)]
        except Exception as e:
            logger.warning("Failed to fetch review rules: %s", e)
        return []

    def _check_pattern(self, rule: dict, code: str) -> list[Violation]:
        definition = rule.get("definition", {})
        pattern = definition.get("pattern", "")
        if not pattern:
            return []
        violations = []
        for i, line in enumerate(code.split("\n"), 1):
            if re.search(pattern, line):
                violations.append(Violation(
                    rule_name=rule.get("name", "unknown"),
                    severity=rule.get("severity", "WARNING"),
                    message=definition.get("message", f"Pattern '{pattern}' matched"),
                    line=i,
                ))
        return violations

    def _format_violations(self, violations: list[Violation]) -> str:
        lines = []
        for v in violations:
            loc = f"line {v.line}" if v.line else ""
            lines.append(f"[{v.severity}] {v.rule_name}: {v.message} {loc}".strip())
        return "\n".join(lines)
```

- [ ] **Step 7: Commit**

```bash
git add ai-worker/src/openharness/hooks/ ai-worker/tests/test_hook_executor.py
git commit -m "feat(harness): HookExecutor + constraint enforcement hook"
```

---

### Task 4: Skills — Markdown 技能加载器

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
from pathlib import Path
from src.openharness.skills.types import SkillDefinition
from src.openharness.skills.loader import parse_skill_markdown
from src.openharness.skills.registry import SkillRegistry

def test_parse_skill_with_frontmatter():
    content = """---
name: code-generation
description: Generate production code
purpose: generate
tools: [query_api_catalog, read_project_file]
---

You are a code generation expert.

## Rules
1. Write complete code
"""
    name, desc, metadata = parse_skill_markdown("fallback", content)
    assert name == "code-generation"
    assert desc == "Generate production code"
    assert metadata["purpose"] == "generate"
    assert "query_api_catalog" in metadata["tools"]

def test_parse_skill_without_frontmatter():
    content = """# My Skill

This is a skill description.

## Details
More content here.
"""
    name, desc, metadata = parse_skill_markdown("my-skill", content)
    assert name == "my-skill"

def test_skill_registry():
    registry = SkillRegistry()
    skill = SkillDefinition(
        name="test", description="Test skill",
        content="# Test\nContent", source="test",
    )
    registry.register(skill)
    assert registry.get("test") is skill
    assert registry.get("nonexistent") is None
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
    source: str  # "builtin", "user", "plugin"
    path: str | None = None
    metadata: dict | None = None

# src/openharness/skills/loader.py
from __future__ import annotations
import re, yaml
from pathlib import Path
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
    skills = []
    d = Path(directory)
    if not d.exists():
        return skills
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

Extract the existing hardcoded system prompts from each agent into `ai-worker/skills/` as markdown files. Each file has frontmatter (name, purpose, tools) + the full system prompt as markdown body.

- [ ] **Step 5: Run tests**

Run: `cd ai-worker && python -m pytest tests/test_skill_loader.py -v`

- [ ] **Step 6: Commit**

```bash
git add ai-worker/src/openharness/skills/ ai-worker/skills/ ai-worker/tests/test_skill_loader.py
git commit -m "feat(harness): SkillLoader + 6 agent skill markdown files"
```

---

### Task 5: Engine — QueryEngine + Agent Loop

**Files:**
- Create: `ai-worker/src/openharness/engine/query_engine.py`
- Create: `ai-worker/src/openharness/engine/query.py`
- Create: `ai-worker/src/openharness/api/client.py`
- Create: `ai-worker/src/openharness/api/providers/__init__.py`
- Create: `ai-worker/src/openharness/api/providers/anthropic.py`
- Create: `ai-worker/src/openharness/api/providers/openai_compat.py`
- Test: `ai-worker/tests/test_query_engine.py`

- [ ] **Step 1: Write failing test for QueryEngine**

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
    """Test that a simple message produces stream events."""
    mock_client = AsyncMock()
    # Mock returns a simple text response with no tool calls
    mock_msg = ConversationMessage(role="assistant", content=[TextBlock(text="Hello!")])
    mock_usage = UsageSnapshot(input_tokens=10, output_tokens=5)

    async def mock_stream(request):
        from src.openharness.api.client import ApiTextDeltaEvent, ApiMessageCompleteEvent
        yield ApiTextDeltaEvent(text="Hello!")
        yield ApiMessageCompleteEvent(message=mock_msg, usage=mock_usage, stop_reason="end_turn")

    mock_client.stream_message = mock_stream

    engine = QueryEngine(
        api_client=mock_client,
        tool_registry=ToolRegistry(),
        model="test-model",
        system_prompt="You are helpful.",
    )

    events = []
    async for event in engine.submit_message("Hi"):
        events.append(event)

    assert len(events) >= 2
    assert isinstance(events[0], AssistantTextDelta)
    assert isinstance(events[-1], AssistantTurnComplete)
    assert engine.total_usage.total_tokens == 15

def test_engine_clear():
    engine = QueryEngine(
        api_client=MagicMock(),
        tool_registry=ToolRegistry(),
        model="test",
        system_prompt="test",
    )
    engine.clear()
    assert len(engine.messages) == 0
```

- [ ] **Step 2: Run to verify fails**

Run: `cd ai-worker && python -m pytest tests/test_query_engine.py -v`

- [ ] **Step 3: Implement api/client.py with protocol**

```python
# src/openharness/api/client.py
"""API client protocol and event types."""

from __future__ import annotations
from dataclasses import dataclass
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
    async def stream_message(self, request: ApiMessageRequest) -> AsyncIterator[ApiStreamEvent]:
        ...
```

- [ ] **Step 4: Implement query.py — the core agent loop**

```python
# src/openharness/engine/query.py
"""Core agent loop — run_query() implements the OpenHarness engine pattern."""

from __future__ import annotations
import asyncio, logging
from dataclasses import dataclass
from typing import Any, AsyncIterator

from .messages import ConversationMessage, ToolResultBlock
from .stream_events import (
    StreamEvent, AssistantTextDelta, AssistantTurnComplete,
    ToolExecutionStarted, ToolExecutionCompleted, ErrorEvent,
)
from ..api.client import ApiMessageRequest, ApiTextDeltaEvent, ApiMessageCompleteEvent, SupportsStreamingMessages
from ..api.usage import UsageSnapshot
from ..tools.base import ToolRegistry, ToolExecutionContext
from ..hooks.executor import HookExecutor
from ..hooks.events import HookEvent
from pathlib import Path

logger = logging.getLogger(__name__)

@dataclass
class QueryContext:
    api_client: SupportsStreamingMessages
    tool_registry: ToolRegistry
    model: str
    system_prompt: str
    max_tokens: int = 4096
    max_turns: int = 20
    hook_executor: HookExecutor | None = None
    cwd: Path = Path(".")

class MaxTurnsExceeded(RuntimeError):
    def __init__(self, max_turns: int):
        self.max_turns = max_turns
        super().__init__(f"Exceeded {max_turns} turns")

async def run_query(
    context: QueryContext,
    messages: list[ConversationMessage],
) -> AsyncIterator[tuple[StreamEvent, UsageSnapshot | None]]:
    for turn in range(context.max_turns):
        request = ApiMessageRequest(
            model=context.model,
            messages=messages,
            system_prompt=context.system_prompt,
            max_tokens=context.max_tokens,
            tools=context.tool_registry.to_api_schema() or None,
        )

        final_message = None
        usage = None

        try:
            async for event in context.api_client.stream_message(request):
                if isinstance(event, ApiTextDeltaEvent):
                    yield AssistantTextDelta(text=event.text), None
                elif isinstance(event, ApiMessageCompleteEvent):
                    final_message = event.message
                    usage = event.usage
                    messages.append(final_message)
                    yield AssistantTurnComplete(message=final_message, usage=usage), usage
        except Exception as e:
            yield ErrorEvent(message=str(e)), None
            return

        if final_message is None:
            return

        tool_calls = final_message.tool_uses
        if not tool_calls:
            return

        tool_results = []
        if len(tool_calls) == 1:
            tc = tool_calls[0]
            yield ToolExecutionStarted(tool_name=tc.name, tool_input=tc.input), None
            result = await _execute_tool_call(context, tc.name, tc.id, tc.input)
            yield ToolExecutionCompleted(tool_name=tc.name, output=result.content, is_error=result.is_error), None
            tool_results.append(result)
        else:
            for tc in tool_calls:
                yield ToolExecutionStarted(tool_name=tc.name, tool_input=tc.input), None
            results = await asyncio.gather(*[
                _execute_tool_call(context, tc.name, tc.id, tc.input)
                for tc in tool_calls
            ])
            for tc, result in zip(tool_calls, results):
                yield ToolExecutionCompleted(tool_name=tc.name, output=result.content, is_error=result.is_error), None
            tool_results = list(results)

        messages.append(ConversationMessage(role="user", content=tool_results))

    raise MaxTurnsExceeded(context.max_turns)

async def _execute_tool_call(
    context: QueryContext,
    tool_name: str,
    tool_use_id: str,
    tool_input: dict[str, Any],
) -> ToolResultBlock:
    # Pre-tool hook
    if context.hook_executor:
        hook_result = await context.hook_executor.execute(
            HookEvent.PRE_TOOL_USE,
            {"tool_name": tool_name, "tool_input": tool_input},
        )
        if hook_result.blocked:
            return ToolResultBlock(tool_use_id=tool_use_id, content=f"BLOCKED: {hook_result.reason}", is_error=True)

    tool = context.tool_registry.get(tool_name)
    if tool is None:
        return ToolResultBlock(tool_use_id=tool_use_id, content=f"Unknown tool: {tool_name}", is_error=True)

    try:
        parsed = tool.input_model.model_validate(tool_input)
    except Exception as e:
        return ToolResultBlock(tool_use_id=tool_use_id, content=f"Input validation error: {e}", is_error=True)

    try:
        exec_ctx = ToolExecutionContext(cwd=context.cwd)
        result = await tool.execute(parsed, exec_ctx)
    except Exception as e:
        result_content = f"Tool execution error: {e}"
        logger.warning("Tool %s failed: %s", tool_name, e)
        return ToolResultBlock(tool_use_id=tool_use_id, content=result_content, is_error=True)

    # Post-tool hook
    if context.hook_executor:
        await context.hook_executor.execute(
            HookEvent.POST_TOOL_USE,
            {"tool_name": tool_name, "tool_input": tool_input, "tool_output": result.output, "tool_is_error": result.is_error},
        )

    return ToolResultBlock(tool_use_id=tool_use_id, content=result.output, is_error=result.is_error)
```

- [ ] **Step 5: Implement query_engine.py**

```python
# src/openharness/engine/query_engine.py
"""QueryEngine — stateful conversation engine."""

from __future__ import annotations
from typing import AsyncIterator

from .messages import ConversationMessage
from .stream_events import StreamEvent
from .cost_tracker import CostTracker
from .query import QueryContext, run_query
from ..api.client import SupportsStreamingMessages
from ..api.usage import UsageSnapshot
from ..tools.base import ToolRegistry
from ..hooks.executor import HookExecutor
from pathlib import Path

class QueryEngine:
    def __init__(
        self,
        api_client: SupportsStreamingMessages,
        tool_registry: ToolRegistry,
        model: str,
        system_prompt: str,
        max_tokens: int = 4096,
        max_turns: int = 20,
        hook_executor: HookExecutor | None = None,
        cwd: Path = Path("."),
    ) -> None:
        self._api_client = api_client
        self._tool_registry = tool_registry
        self._model = model
        self._system_prompt = system_prompt
        self._max_tokens = max_tokens
        self._max_turns = max_turns
        self._hook_executor = hook_executor
        self._cwd = cwd
        self._messages: list[ConversationMessage] = []
        self._cost_tracker = CostTracker()

    @property
    def messages(self) -> list[ConversationMessage]:
        return list(self._messages)

    @property
    def total_usage(self) -> UsageSnapshot:
        return self._cost_tracker.total

    def clear(self) -> None:
        self._messages.clear()
        self._cost_tracker.reset()

    def set_system_prompt(self, prompt: str) -> None:
        self._system_prompt = prompt

    def set_model(self, model: str) -> None:
        self._model = model

    async def submit_message(self, prompt: str) -> AsyncIterator[StreamEvent]:
        self._messages.append(ConversationMessage.from_user_text(prompt))
        context = QueryContext(
            api_client=self._api_client,
            tool_registry=self._tool_registry,
            model=self._model,
            system_prompt=self._system_prompt,
            max_tokens=self._max_tokens,
            max_turns=self._max_turns,
            hook_executor=self._hook_executor,
            cwd=self._cwd,
        )
        async for event, usage in run_query(context, self._messages):
            if usage:
                self._cost_tracker.add(usage)
            yield event
```

- [ ] **Step 6: Run tests**

Run: `cd ai-worker && python -m pytest tests/test_query_engine.py -v`

- [ ] **Step 7: Commit**

```bash
git add ai-worker/src/openharness/engine/ ai-worker/src/openharness/api/ ai-worker/tests/test_query_engine.py
git commit -m "feat(harness): QueryEngine + Agent Loop + API client protocol"
```

---

### Task 6: Permissions — PermissionChecker

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
from src.openharness.permissions.checker import PermissionChecker, PermissionDecision

def test_read_only_always_allowed():
    checker = PermissionChecker(mode=PermissionMode.DEFAULT)
    decision = checker.evaluate("file_read", is_read_only=True)
    assert decision.allowed

def test_mutating_requires_confirmation():
    checker = PermissionChecker(mode=PermissionMode.DEFAULT)
    decision = checker.evaluate("bash", is_read_only=False)
    assert not decision.allowed
    assert decision.requires_confirmation

def test_full_auto_allows_everything():
    checker = PermissionChecker(mode=PermissionMode.FULL_AUTO)
    decision = checker.evaluate("bash", is_read_only=False)
    assert decision.allowed

def test_denied_tool():
    checker = PermissionChecker(mode=PermissionMode.FULL_AUTO, denied_tools=["bash"])
    decision = checker.evaluate("bash", is_read_only=False)
    assert not decision.allowed
```

- [ ] **Step 2: Run to verify fails, implement, run to verify passes**

- [ ] **Step 3: Implement modes.py + checker.py**

```python
# src/openharness/permissions/modes.py
from enum import Enum

class PermissionMode(str, Enum):
    DEFAULT = "default"
    FULL_AUTO = "full_auto"

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
    def __init__(
        self,
        mode: PermissionMode = PermissionMode.DEFAULT,
        allowed_tools: list[str] | None = None,
        denied_tools: list[str] | None = None,
    ) -> None:
        self._mode = mode
        self._allowed = set(allowed_tools or [])
        self._denied = set(denied_tools or [])

    def evaluate(self, tool_name: str, *, is_read_only: bool = False, **kwargs) -> PermissionDecision:
        if tool_name in self._denied:
            return PermissionDecision(allowed=False, reason=f"Tool '{tool_name}' is denied")
        if tool_name in self._allowed:
            return PermissionDecision(allowed=True)
        if is_read_only:
            return PermissionDecision(allowed=True)
        if self._mode == PermissionMode.FULL_AUTO:
            return PermissionDecision(allowed=True)
        return PermissionDecision(allowed=False, requires_confirmation=True, reason="Mutating tool requires confirmation")
```

- [ ] **Step 4: Commit**

```bash
git add ai-worker/src/openharness/permissions/ ai-worker/tests/test_permission_checker.py
git commit -m "feat(harness): PermissionChecker with mode-based authorization"
```

---

### Task 7: API Providers — 适配现有 ModelRouter

**Files:**
- Create: `ai-worker/src/openharness/api/providers/anthropic.py`
- Create: `ai-worker/src/openharness/api/providers/openai_compat.py`
- Modify: `ai-worker/src/models/client.py` — 实现 SupportsStreamingMessages
- Modify: `ai-worker/src/models/router.py` — 委托给新 API clients
- Test: `ai-worker/tests/test_api_providers.py`

- [ ] **Step 1: Write failing test for Anthropic provider**

- [ ] **Step 2: Implement providers wrapping existing callers**

Wrap the existing `call_anthropic()`, `call_openai()`, `call_dashscope()`, `call_deepseek()` functions into classes implementing `SupportsStreamingMessages`. The ModelRouter continues to handle fallback chains and circuit breakers.

- [ ] **Step 3: Run tests, commit**

```bash
git commit -m "feat(harness): API provider adapters implementing SupportsStreamingMessages"
```

---

### Task 8: Agent 改造 — 委托给 QueryEngine

**Files:**
- Modify: `ai-worker/src/agents/base.py` — BaseAgent 使用 QueryEngine
- Modify: `ai-worker/src/agents/analyst.py` — 从 Skill 加载 prompt
- Modify: `ai-worker/src/agents/coder.py`
- Modify: `ai-worker/src/agents/reviewer.py`
- Modify: `ai-worker/src/agents/planner.py`
- Modify: `ai-worker/src/agents/test_writer.py`
- Modify: `ai-worker/src/agents/profiler.py`
- Modify: `ai-worker/src/activities/analyze.py`
- Modify: `ai-worker/src/activities/generate.py`
- Modify: `ai-worker/src/activities/plan.py`
- Modify: `ai-worker/src/activities/review.py`
- Modify: `ai-worker/src/activities/profile.py`
- Modify: `ai-worker/src/worker.py`
- Test: `ai-worker/tests/test_agent_loop.py`

- [ ] **Step 1: Refactor BaseAgent to use QueryEngine internally**

BaseAgent becomes a thin wrapper: builds QueryEngine with the right skill prompt, tool registry, and hooks, then calls `engine.submit_message()`. The existing `run()` method signature stays the same for backward compatibility — Activities don't change.

- [ ] **Step 2: Update each Agent to load prompt from SkillLoader**

Replace hardcoded system prompts with `skill_registry.get("generate").content` etc.

- [ ] **Step 3: Update worker.py to bootstrap Harness**

```python
# worker.py — new initialization
from src.openharness.tools import create_default_tool_registry
from src.openharness.hooks.loader import HookRegistry
from src.openharness.hooks.executor import HookExecutor
from src.openharness.skills.loader import load_skill_registry

# At startup:
skill_registry = load_skill_registry("skills/")
tool_registry = create_default_tool_registry()
hook_registry = HookRegistry()
hook_executor = HookExecutor(hook_registry)
```

- [ ] **Step 4: Run existing tests to verify backward compatibility**

Run: `cd ai-worker && python -m pytest tests/ -v`
Expected: All existing tests still pass

- [ ] **Step 5: Commit**

```bash
git commit -m "refactor(harness): agents delegate to QueryEngine, skills loaded from markdown"
```

---

### Task 9: 后端 — Agent Chat API + SSE 推送（原 Task 12，前置于前端）

**Files:**
- Create: `forge-portal/app/(dashboard)/projects/[id]/agent/page.tsx`
- Create: `forge-portal/components/agent/agent-chat.tsx`
- Create: `forge-portal/components/agent/tool-execution.tsx`
- Create: `forge-portal/components/agent/agent-status.tsx`
- Modify: `forge-portal/components/sidebar.tsx` — 添加 Agent 导航项

- [ ] **Step 1: Create Agent chat page with SSE streaming**

A full-page chat interface that:
- Sends messages to a new `/api/projects/:id/agent/chat` endpoint
- Receives SSE stream of `StreamEvent` types
- Renders `AssistantTextDelta` as streaming text
- Renders `ToolExecutionStarted/Completed` as collapsible tool cards
- Shows token usage in status bar

- [ ] **Step 2: Create tool execution visualization component**

Each tool call renders as a card showing:
- Tool name + input args
- Execution status (running spinner / completed check / error)
- Collapsible output panel
- Time elapsed

- [ ] **Step 3: Create agent status panel**

Shows: current model, total tokens used, tool calls made, active turn number.

- [ ] **Step 4: Add Agent nav item to sidebar**

- [ ] **Step 5: Commit**

```bash
git commit -m "feat(portal): Claude Code-style agent interaction page with streaming"
```

---

### Task 10: 前端 — Claude Code 风格 Agent 交互界面

**Files:**
- Create: `forge-core/internal/module/agent/handler.go`
- Create: `forge-core/internal/module/agent/service.go`
- Modify: `forge-core/internal/router/router.go` — 注册新路由
- Modify: `forge-core/internal/temporal/worker.go` — 注册新 Workflow

- [ ] **Step 1: Create Agent chat handler**

```
POST /api/projects/:id/agent/chat
  Body: { "message": "..." }
  Response: SSE stream

GET /api/projects/:id/agent/stream
  Response: SSE stream of StreamEvent
```

The handler starts a Temporal workflow that runs the full QueryEngine loop, streaming events back via Redis Pub/Sub → SSE.

- [ ] **Step 2: Create Agent chat service**

Manages conversation state (message history) in PostgreSQL. Each project has an active agent session.

- [ ] **Step 3: Run tests, commit**

```bash
git commit -m "feat(core): agent chat API with SSE streaming for web-based Claude Code experience"
```

---

### Task 11: 集成测试 — 端到端验证

**Files:**
- Create: `ai-worker/tests/test_integration_harness.py`

- [ ] **Step 1: Write integration test: Tool → Hook → Permission → Result**

Test that the full pipeline works: register tool, register hook, create engine, submit message, verify events.

- [ ] **Step 2: Write integration test: Skill loading + Agent execution**

Verify that an agent loads its prompt from a skill file and produces correct output format.

- [ ] **Step 3: Write integration test: Constraint enforcement**

Create a PATTERN rule, generate code that violates it, verify the hook blocks.

- [ ] **Step 4: Run full test suite**

Run: `cd ai-worker && python -m pytest tests/ -v --tb=short`

- [ ] **Step 5: Commit**

```bash
git commit -m "test(harness): integration tests for tool→hook→permission→engine pipeline"
```

---

## Verification

### End-to-End Smoke Test

1. Start `docker compose -f docker-compose.dev.yml up -d` (PostgreSQL, Redis, Temporal)
2. Start forge-core: `cd forge-core && go run ./cmd/forge-core`
3. Start ai-worker: `cd ai-worker && python -m src.worker`
4. Start portal: `cd forge-portal && npm run dev`
5. Navigate to project → Agent page
6. Send a message → verify streaming text appears
7. Verify tool calls render as cards with expandable output
8. Verify token usage updates in status bar

### Unit Test Coverage

```bash
cd ai-worker && python -m pytest tests/ -v --cov=src/openharness --cov-report=term-missing
```

Target: 80%+ coverage on `src/openharness/` modules.
