# SH-2 — 上下文工具层（5 个 Context Tools + Agent 接入）

## 目标

为 AI Agent 提供 5 个结构化工具，让 LLM 按需拉取项目上下文（而非在 system prompt 里塞入全部画像数据）：

| 工具 | 用途 | 数据来源 |
|------|------|----------|
| `query_api_catalog` | 查询 API 接口清单 | project_profiles["api_catalog"] |
| `query_db_schema` | 查询数据库结构 | project_profiles["db_schema"] |
| `query_business_rules` | 查询业务规则 | project_profiles["business_rules"] |
| `query_module_graph` | 查询模块依赖图 | project_profiles["module_graph"] |
| `read_project_file` | 读取项目文件内容 | forge-core GET /api/projects/:id/code/file |

完成后：Coder Agent 可以在编码过程中按需查询 API 清单、DB Schema、模块依赖，而不是在 system prompt 中被动接收全部数据（节省 60%+ token，同时更精准）。

## 前置依赖

- **SH-1 完成** — BaseAgent 支持 tools 参数 + tool-use loop + ModelRouter tools 传递
- 项目画像已存在（profile scan activity 已实现）
- forge-core 文件读取 API 已存在（`GET /api/projects/:id/code/file?path=xxx`）

## 工期

2 天

---

## Day 1 — 工具定义与执行器

### 1.1 新建 `ai-worker/src/context/tools.py`

```python
"""Context tools for AI agents — 5 structured tools for on-demand context retrieval.

Tools query project profiles (stored in DB from profile scan) or read project files
via forge-core API. They are called by the Agent Loop (SH-1) when the LLM requests them.

Design principle: Coding standards and prompt templates stay in system prompt (always needed).
Project profiles move to tools (pulled on demand by LLM).
"""
from __future__ import annotations

import json
import logging
from typing import Any

import httpx

from src.config import settings
from src.context.builder import ProjectContext

logger = logging.getLogger(__name__)

# Max content size returned per tool call (chars)
MAX_TOOL_RESULT_CHARS = 20_000

# ============================================================================
# Tool Definitions (JSON Schema format, compatible with Anthropic + OpenAI)
# ============================================================================

TOOL_QUERY_API_CATALOG = {
    "name": "query_api_catalog",
    "description": (
        "查询项目的 API 接口清单。返回 REST 端点列表，包含路径、方法、参数和响应格式。"
        "当你需要了解项目有哪些 API、某个模块的接口签名、或判断接口是否已存在时调用此工具。"
        "支持关键词过滤，例如传入 'user' 只返回用户相关的 API。"
    ),
    "parameters": {
        "type": "object",
        "properties": {
            "keyword": {
                "type": "string",
                "description": "过滤关键词（可选），只返回包含此关键词的 API 条目。例如 'user', 'order', 'auth'",
            },
        },
        "required": [],
    },
}

TOOL_QUERY_DB_SCHEMA = {
    "name": "query_db_schema",
    "description": (
        "查询项目的数据库表结构。返回表名、字段名、字段类型、索引和外键约束。"
        "当你需要了解数据模型、编写 SQL、设计新表或理解字段关系时调用此工具。"
        "支持关键词过滤，例如传入 'user' 只返回用户相关的表。"
    ),
    "parameters": {
        "type": "object",
        "properties": {
            "keyword": {
                "type": "string",
                "description": "过滤关键词（可选），只返回包含此关键词的表。例如 'user', 'order', 'product'",
            },
        },
        "required": [],
    },
}

TOOL_QUERY_BUSINESS_RULES = {
    "name": "query_business_rules",
    "description": (
        "查询项目的业务规则。返回验证规则、权限策略、工作流约束等业务逻辑。"
        "当你需要理解业务约束（如订单状态流转、权限校验、数据校验规则）时调用此工具。"
        "支持关键词过滤。"
    ),
    "parameters": {
        "type": "object",
        "properties": {
            "keyword": {
                "type": "string",
                "description": "过滤关键词（可选）。例如 'permission', 'validate', 'workflow'",
            },
        },
        "required": [],
    },
}

TOOL_QUERY_MODULE_GRAPH = {
    "name": "query_module_graph",
    "description": (
        "查询项目的模块依赖关系图。返回模块列表、模块间的导入关系和依赖方向。"
        "当你需要了解项目架构、判断代码应放在哪个模块、或评估修改的影响范围时调用此工具。"
        "支持关键词过滤。"
    ),
    "parameters": {
        "type": "object",
        "properties": {
            "keyword": {
                "type": "string",
                "description": "过滤关键词（可选）。例如 'auth', 'payment', 'handler'",
            },
        },
        "required": [],
    },
}

TOOL_READ_PROJECT_FILE = {
    "name": "read_project_file",
    "description": (
        "读取项目仓库中某个文件的内容。返回文件的完整文本（最多 20KB，超出截断）。"
        "当你需要查看现有代码实现、参考已有模式、或理解某个文件的具体逻辑时调用此工具。"
        "你需要知道文件的路径。可以先用 query_module_graph 了解项目结构。"
    ),
    "parameters": {
        "type": "object",
        "properties": {
            "path": {
                "type": "string",
                "description": "文件在仓库中的相对路径。例如 'src/main.go', 'internal/handler/user.go', 'package.json'",
            },
        },
        "required": ["path"],
    },
}

# All 5 tools
CONTEXT_TOOLS: list[dict] = [
    TOOL_QUERY_API_CATALOG,
    TOOL_QUERY_DB_SCHEMA,
    TOOL_QUERY_BUSINESS_RULES,
    TOOL_QUERY_MODULE_GRAPH,
    TOOL_READ_PROJECT_FILE,
]

# Profile-based tools (no API call needed, just query local data)
_PROFILE_TOOL_MAP: dict[str, str] = {
    "query_api_catalog": "api_catalog",
    "query_db_schema": "db_schema",
    "query_business_rules": "business_rules",
    "query_module_graph": "module_graph",
}


def build_profile_availability_hint(context: ProjectContext) -> str:
    """Generate a one-line hint for the system prompt showing which profiles are available.

    Example: "可用画像: api_catalog(52条), db_schema(12表), module_graph(8模块)"
    This helps the LLM decide which tools are worth calling.
    """
    available = []
    dimension_labels = {
        "api_catalog": "API",
        "db_schema": "表结构",
        "module_graph": "模块",
        "business_rules": "规则",
        "architecture": "架构",
    }
    for key, value in context.project_profiles.items():
        label = dimension_labels.get(key, key)
        # Try to count items
        count = ""
        if isinstance(value, dict):
            items = value.get("items", value.get("tables", value.get("modules", value.get("rules", []))))
            if isinstance(items, list):
                count = f"({len(items)}条)"
            elif isinstance(value, dict) and len(value) > 0:
                count = f"({len(value)}项)"
        elif isinstance(value, list):
            count = f"({len(value)}条)"
        available.append(f"{label}{count}")

    if not available:
        return "项目画像: 暂无（建议先执行画像扫描）"
    return "可用画像: " + ", ".join(available) + "。使用查询工具按需获取详情。"


# ============================================================================
# Tool Executor
# ============================================================================

class ContextToolExecutor:
    """Executes context tools called by the Agent Loop.

    Profile-query tools: search in-memory ProjectContext.project_profiles
    File-read tool: calls forge-core API
    """

    def __init__(self) -> None:
        self._http_client: httpx.AsyncClient | None = None

    async def _get_client(self) -> httpx.AsyncClient:
        if self._http_client is None:
            self._http_client = httpx.AsyncClient(
                base_url=settings.forge_api_url,
                headers={"Authorization": f"Bearer {settings.forge_api_token}"},
                timeout=10.0,
            )
        return self._http_client

    async def execute(
        self,
        tool_call: Any,  # ToolCall dataclass from client.py
        context: ProjectContext,
        project_id: int,
    ) -> str:
        """Execute a single tool call and return the result as a string."""
        name = tool_call.name
        args = tool_call.arguments or {}

        logger.info("executing tool: %s args=%s project=%d", name, args, project_id)

        # Profile-based tools
        if name in _PROFILE_TOOL_MAP:
            profile_key = _PROFILE_TOOL_MAP[name]
            return self._query_profile(context, profile_key, args.get("keyword"))

        # File read tool
        if name == "read_project_file":
            path = args.get("path", "")
            if not path:
                return "Error: 'path' parameter is required"
            return await self._read_file(project_id, path)

        return f"Unknown tool: {name}"

    def _query_profile(
        self,
        context: ProjectContext,
        profile_key: str,
        keyword: str | None,
    ) -> str:
        """Query a project profile dimension with optional keyword filtering."""
        profile_data = context.project_profiles.get(profile_key)
        if not profile_data:
            return f"画像维度 '{profile_key}' 暂无数据。请先执行项目画像扫描。"

        # If no keyword, return full profile (truncated)
        if not keyword:
            result = json.dumps(profile_data, ensure_ascii=False, indent=2)
            if len(result) > MAX_TOOL_RESULT_CHARS:
                result = result[:MAX_TOOL_RESULT_CHARS] + "\n... (截断，请使用 keyword 过滤)"
            return result

        # Keyword filtering — search through the profile structure
        keyword_lower = keyword.lower()
        filtered = self._filter_profile(profile_data, keyword_lower)

        if not filtered:
            return f"未找到包含 '{keyword}' 的条目。"

        result = json.dumps(filtered, ensure_ascii=False, indent=2)
        if len(result) > MAX_TOOL_RESULT_CHARS:
            result = result[:MAX_TOOL_RESULT_CHARS] + "\n... (截断)"
        return result

    def _filter_profile(self, data: Any, keyword: str) -> Any:
        """Recursively filter profile data by keyword."""
        if isinstance(data, dict):
            # Check if this dict matches
            data_str = json.dumps(data, ensure_ascii=False).lower()
            if keyword in data_str:
                # If dict has "items", "tables", "modules", "rules" — filter the list
                for list_key in ("items", "tables", "modules", "rules", "endpoints", "apis"):
                    if list_key in data and isinstance(data[list_key], list):
                        filtered_list = [
                            item for item in data[list_key]
                            if keyword in json.dumps(item, ensure_ascii=False).lower()
                        ]
                        if filtered_list:
                            return {**data, list_key: filtered_list}
                return data
            return None

        elif isinstance(data, list):
            filtered = [
                item for item in data
                if keyword in json.dumps(item, ensure_ascii=False).lower()
            ]
            return filtered if filtered else None

        elif isinstance(data, str):
            return data if keyword in data.lower() else None

        return None

    async def _read_file(self, project_id: int, path: str) -> str:
        """Read a file from the project repository via forge-core API."""
        try:
            client = await self._get_client()
            resp = await client.get(
                f"/api/projects/{project_id}/code/file",
                params={"path": path},
            )
            if resp.status_code == 404:
                return f"文件不存在: {path}"
            if resp.status_code != 200:
                return f"读取文件失败: HTTP {resp.status_code}"

            data = resp.json().get("data", {})
            content = data.get("content", "")

            if not content:
                return f"文件为空: {path}"

            # Truncate large files
            if len(content) > MAX_TOOL_RESULT_CHARS:
                content = content[:MAX_TOOL_RESULT_CHARS] + f"\n... (文件截断，原始大小: {len(content)} chars)"

            return f"=== {path} ===\n{content}"

        except httpx.TimeoutException:
            return f"读取文件超时: {path}"
        except Exception as e:
            return f"读取文件异常: {path} — {e}"

    async def close(self) -> None:
        if self._http_client:
            await self._http_client.aclose()
```

---

## Day 2 — Agent 接入（每个 Agent 配置工具集）

### 2.1 修改 `ai-worker/src/agents/coder.py` — 全部 5 个工具

```python
from src.context.tools import (
    CONTEXT_TOOLS,
    ContextToolExecutor,
    build_profile_availability_hint,
)

class CoderAgent(BaseAgent):
    purpose = Purpose.GENERATE
    tools = CONTEXT_TOOLS  # All 5 tools

    def __init__(self, router: ModelRouter) -> None:
        super().__init__(router)
        self._tool_executor = ContextToolExecutor()

    def _get_tool_executor(self) -> ContextToolExecutor:
        return self._tool_executor

    def _build_system_prompt(self, context: ProjectContext) -> str:
        base = CODER_SYSTEM_PROMPT

        # Inject language constraints (unchanged)
        if context.tech_stack:
            lang_constraints = _build_language_constraints(context.tech_stack)
            if lang_constraints:
                base += f"\n\n{lang_constraints}"

        # Coding standards STAY in system prompt (always needed)
        if context.coding_standards:
            base += "\n\n## Coding Standards (MUST follow)\n"
            for std in context.coding_standards:
                base += std + "\n"

        # Profile availability hint (replaces full profile injection)
        hint = build_profile_availability_hint(context)
        base += f"\n\n## {hint}"
        base += "\n编码标准已包含在上方。项目画像数据请通过工具按需查询。"

        return base
```

### 2.2 修改 `ai-worker/src/agents/planner.py` — 3 个工具

```python
from src.context.tools import (
    TOOL_QUERY_API_CATALOG,
    TOOL_QUERY_MODULE_GRAPH,
    TOOL_READ_PROJECT_FILE,
    ContextToolExecutor,
    build_profile_availability_hint,
)

class PlannerAgent(BaseAgent):
    purpose = Purpose.PLAN
    tools = [TOOL_QUERY_API_CATALOG, TOOL_QUERY_MODULE_GRAPH, TOOL_READ_PROJECT_FILE]

    def __init__(self, router: ModelRouter) -> None:
        super().__init__(router)
        self._tool_executor = ContextToolExecutor()

    def _get_tool_executor(self) -> ContextToolExecutor:
        return self._tool_executor

    def _build_system_prompt(self, context: ProjectContext) -> str:
        base = PLANNER_SYSTEM_PROMPT
        # Coding standards in system prompt
        if context.coding_standards:
            base += "\n\n## Coding Standards\n"
            for std in context.coding_standards:
                base += std + "\n"
        # Profile hint
        hint = build_profile_availability_hint(context)
        base += f"\n\n## {hint}"
        base += "\n使用 query_api_catalog 了解现有 API，使用 query_module_graph 了解模块结构，使用 read_project_file 查看具体代码。"
        return base
```

### 2.3 修改 `ai-worker/src/agents/test_writer.py` — 3 个工具

```python
from src.context.tools import (
    TOOL_QUERY_DB_SCHEMA,
    TOOL_QUERY_API_CATALOG,
    TOOL_READ_PROJECT_FILE,
    ContextToolExecutor,
    build_profile_availability_hint,
)

class TestWriterAgent(BaseAgent):
    purpose = Purpose.TEST_WRITING
    tools = [TOOL_QUERY_DB_SCHEMA, TOOL_QUERY_API_CATALOG, TOOL_READ_PROJECT_FILE]

    def __init__(self, router: ModelRouter) -> None:
        super().__init__(router)
        self._tool_executor = ContextToolExecutor()

    def _get_tool_executor(self) -> ContextToolExecutor:
        return self._tool_executor

    def _build_system_prompt(self, context: ProjectContext) -> str:
        base = TEST_WRITER_SYSTEM_PROMPT
        # Tech stack for framework selection (unchanged)
        tech = context.tech_stack
        if tech:
            frameworks = tech.get("frameworks", [])
            languages = tech.get("languages", {})
            if frameworks or languages:
                base += f"\n\n## Detected Tech Stack\nLanguages: {languages}\nFrameworks: {frameworks}"
        # Coding standards
        if context.coding_standards:
            base += "\n\n## Coding Standards\n"
            for std in context.coding_standards:
                base += std + "\n"
        # Profile hint
        hint = build_profile_availability_hint(context)
        base += f"\n\n## {hint}"
        base += "\n使用 query_db_schema 了解数据模型，使用 query_api_catalog 了解接口签名，使用 read_project_file 查看测试样板。"
        return base
```

### 2.4 修改 `ai-worker/src/agents/reviewer.py` — 2 个工具

```python
from src.context.tools import (
    TOOL_READ_PROJECT_FILE,
    TOOL_QUERY_BUSINESS_RULES,
    ContextToolExecutor,
    build_profile_availability_hint,
)

class ReviewerAgent(BaseAgent):
    purpose = Purpose.REVIEW
    tools = [TOOL_READ_PROJECT_FILE, TOOL_QUERY_BUSINESS_RULES]

    def __init__(self, router: ModelRouter) -> None:
        super().__init__(router)
        self._tool_executor = ContextToolExecutor()

    def _get_tool_executor(self) -> ContextToolExecutor:
        return self._tool_executor

    def _build_system_prompt(self, context: ProjectContext) -> str:
        base = REVIEWER_SYSTEM_PROMPT
        if context.review_rules:
            base += "\n\n## Review Rules to Check\n"
            for rule in context.review_rules:
                name = rule.get("name", "")
                category = rule.get("category", "")
                severity = rule.get("severity", "")
                base += f"- [{severity}] {category}: {name}\n"
        # Coding standards (always needed for review)
        if context.coding_standards:
            base += "\n\n## Coding Standards (MUST enforce)\n"
            for std in context.coding_standards:
                base += std + "\n"
        # Profile hint
        hint = build_profile_availability_hint(context)
        base += f"\n\n## {hint}"
        base += "\n使用 read_project_file 查看原有代码对比，使用 query_business_rules 验证业务逻辑正确性。"
        return base
```

### 2.5 `ai-worker/src/agents/analyst.py` — 无工具（不变）

AnalystAgent 不使用工具：需求分析阶段不需要查代码，只需要对话理解。

```python
class AnalystAgent(BaseAgent):
    purpose = Purpose.ANALYZE
    # tools = None  (default, no tools)

    def _build_system_prompt(self, context: ProjectContext) -> str:
        base = ANALYST_SYSTEM_PROMPT
        project_context = context.to_system_prompt()
        if project_context:
            base += f"\n\n## Project Context\n{project_context}"
        return base
```

### 2.6 修改 Activity 传递 `project_id` 到 Agent.run()

每个 Activity 调用 `agent.run()` 时需要传 `project_id`，供工具执行器读取文件：

```python
# activities/generate.py — 修改 fallback 非流式调用
result = await agent.run(user_prompt, ctx, project_id=input.project_id)

# activities/plan.py
result = await agent.run(summary, ctx, project_id=input.project_id)

# activities/test_writing.py
result = await agent.run(user_prompt, ctx, project_id=input.project_id)

# activities/review.py
result = await agent.run(user_prompt, ctx, project_id=input.project_id)
```

### 2.7 Agent 工具配置总结

| Agent | tools | 理由 |
|-------|-------|------|
| `AnalystAgent` | `None` | 需求分析不需要代码上下文 |
| `PlannerAgent` | `[query_api_catalog, query_module_graph, read_project_file]` | 规划需要了解 API 和模块结构，判断文件位置 |
| `TestWriterAgent` | `[query_db_schema, query_api_catalog, read_project_file]` | 测试需要了解数据模型和接口签名 |
| `CoderAgent` | `CONTEXT_TOOLS` (全部 5 个) | 编码需要全部上下文维度 |
| `ReviewerAgent` | `[read_project_file, query_business_rules]` | 审查需要对比原有代码，验证业务规则 |

---

## 数据结构

### Tool Definition Schema（通用格式，兼容 Anthropic + OpenAI）

```json
{
  "name": "query_api_catalog",
  "description": "查询项目的 API 接口清单...",
  "parameters": {
    "type": "object",
    "properties": {
      "keyword": {
        "type": "string",
        "description": "过滤关键词"
      }
    },
    "required": []
  }
}
```

### ContextToolExecutor 执行流

```
Agent Loop (base.py)
  ├── LLM 返回 tool_use: query_db_schema(keyword="user")
  ├── BaseAgent 调用 executor.execute(tool_call, context, project_id)
  │   └── ContextToolExecutor._query_profile(context, "db_schema", "user")
  │       └── 从 context.project_profiles["db_schema"] 中过滤含 "user" 的条目
  │       └── 返回 JSON 字符串
  ├── 结果作为 tool_result 追加到 messages
  └── 下一轮 LLM 调用...
```

---

## API 设计

本切片不新增 API。所有工具调用都在 ai-worker 进程内完成：
- Profile 查询：直接从 ProjectContext.project_profiles 内存查询
- 文件读取：调用已有的 `GET /api/projects/:id/code/file?path=xxx`

---

## 验收标准

1. **5 个工具定义**
   - 每个工具有完整的 name、description、parameters（JSON Schema）
   - Description 用中文，清晰说明何时调用
   - parameters.required 只在必要时设置（read_project_file 的 path 必填，其余可选）

2. **Profile 查询工具**
   - 无 keyword 参数时返回完整画像（截断至 20KB）
   - 有 keyword 时只返回匹配条目
   - 画像不存在时返回友好提示"暂无数据，请先执行画像扫描"

3. **文件读取工具**
   - 正确调用 forge-core API
   - 文件不存在返回 404 友好提示
   - 大文件截断至 20KB 并标注

4. **Agent 接入**
   - CoderAgent 有全部 5 个工具
   - PlannerAgent 有 3 个工具
   - TestWriterAgent 有 3 个工具
   - ReviewerAgent 有 2 个工具
   - AnalystAgent 无工具（不变）
   - 所有 Agent 的 system prompt 中保留 coding standards
   - system prompt 中不再注入完整 project_profiles，改为 availability hint

5. **质量对比**
   - 对同一任务，分别用旧模式（全量注入）和新模式（工具按需）运行
   - 新模式的 Review score 应 >= 旧模式（不降分）
   - 新模式的 token 消耗应减少 30%+ （因为不注入全部画像）

---

## 质量验证

### 测试用例

```python
# tests/test_context_tools.py

import pytest
from src.context.tools import ContextToolExecutor, CONTEXT_TOOLS, build_profile_availability_hint
from src.context.builder import ProjectContext
from unittest.mock import AsyncMock

def _make_context_with_profiles() -> ProjectContext:
    """Create a ProjectContext with sample profile data."""
    return ProjectContext(
        project_profiles={
            "api_catalog": {
                "items": [
                    {"method": "GET", "path": "/api/users", "description": "List users"},
                    {"method": "POST", "path": "/api/users", "description": "Create user"},
                    {"method": "GET", "path": "/api/orders", "description": "List orders"},
                ]
            },
            "db_schema": {
                "tables": [
                    {"name": "users", "columns": [{"name": "id", "type": "BIGINT"}, {"name": "email", "type": "VARCHAR"}]},
                    {"name": "orders", "columns": [{"name": "id", "type": "BIGINT"}, {"name": "user_id", "type": "BIGINT"}]},
                ]
            },
            "module_graph": {
                "modules": [
                    {"name": "auth", "depends_on": []},
                    {"name": "user", "depends_on": ["auth"]},
                    {"name": "order", "depends_on": ["user"]},
                ]
            },
        }
    )


class TestContextToolExecutor:
    """Test the ContextToolExecutor class."""

    @pytest.mark.asyncio
    async def test_query_api_catalog_no_keyword(self):
        """Should return full API catalog."""
        from dataclasses import dataclass
        @dataclass
        class FakeToolCall:
            name: str = "query_api_catalog"
            arguments: dict = None
            id: str = "1"
        tc = FakeToolCall(arguments={})
        executor = ContextToolExecutor()
        ctx = _make_context_with_profiles()
        result = await executor.execute(tc, ctx, project_id=1)
        assert "/api/users" in result
        assert "/api/orders" in result
        await executor.close()

    @pytest.mark.asyncio
    async def test_query_api_catalog_with_keyword(self):
        """Should filter by keyword."""
        from dataclasses import dataclass
        @dataclass
        class FakeToolCall:
            name: str = "query_api_catalog"
            arguments: dict = None
            id: str = "1"
        tc = FakeToolCall(arguments={"keyword": "user"})
        executor = ContextToolExecutor()
        ctx = _make_context_with_profiles()
        result = await executor.execute(tc, ctx, project_id=1)
        assert "user" in result.lower()
        assert "order" not in result.lower()  # Filtered out
        await executor.close()

    @pytest.mark.asyncio
    async def test_query_empty_profile(self):
        """Should return friendly message when profile is empty."""
        from dataclasses import dataclass
        @dataclass
        class FakeToolCall:
            name: str = "query_db_schema"
            arguments: dict = None
            id: str = "1"
        tc = FakeToolCall(arguments={})
        executor = ContextToolExecutor()
        ctx = ProjectContext()  # No profiles
        result = await executor.execute(tc, ctx, project_id=1)
        assert "暂无数据" in result
        await executor.close()

    @pytest.mark.asyncio
    async def test_read_project_file_success(self):
        """Should return file content from forge-core API."""
        from dataclasses import dataclass
        @dataclass
        class FakeToolCall:
            name: str = "read_project_file"
            arguments: dict = None
            id: str = "1"
        tc = FakeToolCall(arguments={"path": "main.go"})
        executor = ContextToolExecutor()
        # Mock HTTP client
        executor._http_client = AsyncMock()
        mock_resp = AsyncMock()
        mock_resp.status_code = 200
        mock_resp.json.return_value = {"data": {"content": "package main\n\nfunc main() {}"}}
        executor._http_client.get = AsyncMock(return_value=mock_resp)

        result = await executor.execute(tc, ProjectContext(), project_id=1)
        assert "package main" in result
        assert "main.go" in result

    @pytest.mark.asyncio
    async def test_read_project_file_not_found(self):
        """Should return 404 message."""
        from dataclasses import dataclass
        @dataclass
        class FakeToolCall:
            name: str = "read_project_file"
            arguments: dict = None
            id: str = "1"
        tc = FakeToolCall(arguments={"path": "nonexistent.go"})
        executor = ContextToolExecutor()
        executor._http_client = AsyncMock()
        mock_resp = AsyncMock()
        mock_resp.status_code = 404
        executor._http_client.get = AsyncMock(return_value=mock_resp)

        result = await executor.execute(tc, ProjectContext(), project_id=1)
        assert "不存在" in result


class TestProfileAvailabilityHint:
    def test_hint_with_profiles(self):
        ctx = _make_context_with_profiles()
        hint = build_profile_availability_hint(ctx)
        assert "API" in hint
        assert "3条" in hint  # 3 API items
        assert "表结构" in hint

    def test_hint_without_profiles(self):
        ctx = ProjectContext()
        hint = build_profile_availability_hint(ctx)
        assert "暂无" in hint


class TestToolDefinitions:
    def test_all_tools_have_required_fields(self):
        for tool in CONTEXT_TOOLS:
            assert "name" in tool
            assert "description" in tool
            assert "parameters" in tool
            assert tool["parameters"]["type"] == "object"
            assert "properties" in tool["parameters"]

    def test_read_project_file_requires_path(self):
        assert "path" in TOOL_READ_PROJECT_FILE["parameters"]["required"]

    def test_profile_tools_keyword_optional(self):
        for tool in [TOOL_QUERY_API_CATALOG, TOOL_QUERY_DB_SCHEMA, TOOL_QUERY_BUSINESS_RULES, TOOL_QUERY_MODULE_GRAPH]:
            assert tool["parameters"]["required"] == []
```

### 端到端回归验证

```bash
# 对同一项目、同一需求，对比旧模式和新模式
# 旧模式（SH-2 前）：全量画像注入 system prompt
# 新模式（SH-2 后）：工具按需查询

# 检查：
# 1. Review score >= 旧模式
# 2. Token 消耗减少 30%+
# 3. 生成代码无回归（文件结构、import 正确性）
```
