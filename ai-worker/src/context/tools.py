"""
Context Tools — On-demand knowledge retrieval for AI Agents.

Based on learn-claude-code s05: "Load knowledge when you need it, not upfront."

Instead of stuffing all project profiles (API catalog, DB schema, module graph,
business rules) into the system prompt, we expose them as tools that the Agent
can query when needed. This:
  1. Reduces system prompt from ~8000 to ~3000 tokens
  2. Gives Agent precise, filtered information instead of everything
  3. Enables reading actual source files from the project repo

Usage:
    from src.context.tools import CONTEXT_TOOLS, ContextToolExecutor
    executor = ContextToolExecutor(context, project_id)
    result = await agent.run(user_input, context, tools=CONTEXT_TOOLS, tool_executor=executor)
"""

from __future__ import annotations

import json
import logging
from typing import Any

import httpx

from src.config import settings
from src.context.builder import ProjectContext

logger = logging.getLogger(__name__)

# ---------------------------------------------------------------------------
# Tool Definitions (Anthropic format — converted to OpenAI format by router)
# ---------------------------------------------------------------------------

CONTEXT_TOOLS = [
    {
        "name": "query_api_catalog",
        "description": (
            "查询项目的 API 接口清单。当你需要了解现有 API 路径、HTTP 方法、请求参数、"
            "返回值格式时调用此工具。传入关键词过滤（如 'user' 或 '/api/orders'）。"
        ),
        "input_schema": {
            "type": "object",
            "properties": {
                "keyword": {
                    "type": "string",
                    "description": "搜索关键词（API 路径、handler 名称、相关实体名）",
                }
            },
            "required": ["keyword"],
        },
    },
    {
        "name": "query_db_schema",
        "description": (
            "查询项目的数据库表结构（字段名、类型、索引、外键关系）。"
            "当你需要设计数据模型、编写 SQL、或理解已有表结构时调用。"
        ),
        "input_schema": {
            "type": "object",
            "properties": {
                "table_name": {
                    "type": "string",
                    "description": "表名或关键词（如 'users' 或 'order'）",
                }
            },
            "required": ["table_name"],
        },
    },
    {
        "name": "query_business_rules",
        "description": (
            "查询项目的业务规则约束。例如'积分不能为负'、'订单超时30分钟自动取消'。"
            "当你需要了解业务逻辑边界、验证规则、工作流约束时调用。"
        ),
        "input_schema": {
            "type": "object",
            "properties": {
                "domain": {
                    "type": "string",
                    "description": "业务域关键词（如 'user'、'payment'、'order'）",
                }
            },
            "required": ["domain"],
        },
    },
    {
        "name": "query_module_graph",
        "description": (
            "查询项目的模块依赖关系图。了解代码组织结构、模块边界、import 路径。"
            "当你需要确定新代码应放在哪个模块、或了解模块间调用关系时调用。"
        ),
        "input_schema": {
            "type": "object",
            "properties": {
                "module_name": {
                    "type": "string",
                    "description": "模块名或关键词（如 'auth'、'user'、'service'）",
                }
            },
            "required": ["module_name"],
        },
    },
    {
        "name": "read_project_file",
        "description": (
            "读取项目仓库中的源代码文件。当你需要参考现有代码实现风格、"
            "理解已有逻辑、或确认接口定义时调用。返回文件内容（最多 20000 字符）。"
        ),
        "input_schema": {
            "type": "object",
            "properties": {
                "path": {
                    "type": "string",
                    "description": "文件路径（如 'internal/module/user/service.go'）",
                }
            },
            "required": ["path"],
        },
    },
]


def build_profile_availability_hint(profiles: dict) -> str:
    """
    Generate a one-line hint for the system prompt showing which profile
    dimensions have data. Lets the Agent skip querying empty dimensions.

    Example output:
        "Available profile data: api_catalog (23 endpoints), db_schema (12 tables), business_rules (8 rules), module_graph (empty)"
    """
    dimension_labels = {
        "api_catalog": "endpoints",
        "db_schema": "tables",
        "module_graph": "modules",
        "architecture": "components",
        "business_rules": "rules",
        "coding_habits": "patterns",
        "quality_trends": "metrics",
    }

    parts = []
    for key, label in dimension_labels.items():
        data = profiles.get(key, {})
        if not data:
            parts.append(f"{key} (empty)")
        else:
            # Try to count items in the profile data
            count = _count_profile_items(key, data)
            if count > 0:
                parts.append(f"{key} ({count} {label})")
            else:
                parts.append(f"{key} (available)")

    if not parts:
        return "Project profiles not yet scanned. Use read_project_file to inspect code directly."

    return "Available profile data: " + ", ".join(parts)


def _count_profile_items(key: str, data: Any) -> int:
    """Count the number of items in a profile dimension."""
    if isinstance(data, dict):
        if key == "api_catalog":
            return len(data.get("endpoints", []))
        elif key == "db_schema":
            return len(data.get("tables", []))
        elif key == "module_graph":
            return len(data.get("modules", []))
        elif key == "business_rules":
            return len(data.get("rules", []))
        elif key == "architecture":
            return len(data.get("services", []))
    return 0


# ---------------------------------------------------------------------------
# Tool Executor
# ---------------------------------------------------------------------------

class ContextToolExecutor:
    """
    Executes context query tools using cached ProjectContext and forge-core API.

    Tool calls from the Agent are routed here. Profile queries use the
    already-cached ProjectContext (no extra HTTP calls). File reads go
    through the forge-core code browsing API.
    """

    def __init__(self, context: ProjectContext, project_id: int) -> None:
        self.context = context
        self.project_id = project_id

    async def execute(self, tool_call: dict) -> str:
        """Execute a tool call and return the result as a string."""
        name = tool_call.get("name", "")
        args = tool_call.get("input", {})

        handlers = {
            "query_api_catalog": self._query_api_catalog,
            "query_db_schema": self._query_db_schema,
            "query_business_rules": self._query_business_rules,
            "query_module_graph": self._query_module_graph,
            "read_project_file": self._read_project_file,
        }

        handler = handlers.get(name)
        if not handler:
            return f"Unknown tool: {name}"

        try:
            return await handler(args)
        except Exception as e:
            logger.warning("Tool %s execution error: %s", name, e)
            return f"Tool execution error: {str(e)}"

    async def _query_api_catalog(self, args: dict) -> str:
        keyword = args.get("keyword", "").lower()
        return self._search_profile("api_catalog", keyword, "endpoints", ["path", "method", "handler"])

    async def _query_db_schema(self, args: dict) -> str:
        keyword = args.get("table_name", "").lower()
        return self._search_profile("db_schema", keyword, "tables", ["name", "columns"])

    async def _query_business_rules(self, args: dict) -> str:
        keyword = args.get("domain", "").lower()
        return self._search_profile("business_rules", keyword, "rules", ["domain", "rule", "source"])

    async def _query_module_graph(self, args: dict) -> str:
        keyword = args.get("module_name", "").lower()
        return self._search_profile("module_graph", keyword, "modules", ["name", "path", "depends_on"])

    def _search_profile(
        self,
        dimension: str,
        keyword: str,
        items_key: str,
        match_fields: list[str],
    ) -> str:
        """Search within a profile dimension by keyword matching."""
        profile_data = self.context.project_profiles.get(dimension, {})
        if not profile_data:
            return (
                f"项目画像中没有 {dimension} 数据。"
                f"可能尚未执行画像扫描。你可以使用 read_project_file 直接读取源代码。"
            )

        items = profile_data.get(items_key, [])
        if not items:
            return f"{dimension} 数据为空（{items_key} 列表为空）。"

        # Filter by keyword
        if keyword:
            results = []
            for item in items:
                item_str = json.dumps(item, ensure_ascii=False).lower()
                if keyword in item_str:
                    results.append(item)
        else:
            results = items

        if not results:
            return f"未找到与 '{keyword}' 相关的 {dimension} 数据。可用条目共 {len(items)} 个。"

        # Truncate if too many results
        if len(results) > 20:
            results = results[:20]
            truncation_note = f"\n\n... 显示前 20 条（共 {len(items)} 条匹配）"
        else:
            truncation_note = ""

        return json.dumps(results, ensure_ascii=False, indent=2) + truncation_note

    async def _read_project_file(self, args: dict) -> str:
        """Read a source file from the project repo via forge-core API."""
        path = args.get("path", "")
        if not path:
            return "Error: file path is required"

        async with httpx.AsyncClient(timeout=10) as client:
            try:
                resp = await client.get(
                    f"{settings.forge_api_url}/api/projects/{self.project_id}/code/file",
                    params={"path": path, "ref": "main"},
                    headers={"Authorization": f"Bearer {settings.forge_api_token}"},
                )
            except Exception as e:
                return f"无法连接到 Forge API: {e}"

        if resp.status_code == 200:
            data = resp.json().get("data", {})
            content = data.get("content", "")
            if not content:
                return f"文件 {path} 为空或不存在。"
            if len(content) > 20000:
                return (
                    content[:20000]
                    + f"\n\n... [文件截断，仅显示前 20000 字符，完整大小 {len(content)} 字符]"
                )
            return content
        elif resp.status_code == 404:
            return f"文件 {path} 不存在。请检查路径是否正确。"
        else:
            return f"读取文件 {path} 失败（HTTP {resp.status_code}）"
