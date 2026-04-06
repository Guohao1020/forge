"""Context tools -- on-demand knowledge retrieval for AI Agents.

Migrated from src.context.tools to the BaseTool interface. Five read-only
tools that query project profiles (API catalog, DB schema, business rules,
module graph) or read source files from the project repo.
"""

from __future__ import annotations

import json
import logging
from typing import Any

import httpx
from pydantic import BaseModel, Field

from src.config import settings
from src.openharness.tools.base import (
    BaseTool,
    ToolExecutionContext,
    ToolRegistry,
    ToolResult,
)

logger = logging.getLogger(__name__)


# ---------------------------------------------------------------------------
# Pydantic input models
# ---------------------------------------------------------------------------


class QueryApiCatalogInput(BaseModel):
    keyword: str = Field(
        ..., description="搜索关键词（API 路径、handler 名称、相关实体名）"
    )


class QueryDbSchemaInput(BaseModel):
    table_name: str = Field(
        ..., description="表名或关键词（如 'users' 或 'order'）"
    )


class QueryBusinessRulesInput(BaseModel):
    domain: str = Field(
        ..., description="业务域关键词（如 'user'、'payment'、'order'）"
    )


class QueryModuleGraphInput(BaseModel):
    module_name: str = Field(
        ..., description="模块名或关键词（如 'auth'、'user'、'service'）"
    )


class ReadProjectFileInput(BaseModel):
    path: str = Field(
        ..., description="文件路径（如 'internal/module/user/service.go'）"
    )


# ---------------------------------------------------------------------------
# Shared search helper
# ---------------------------------------------------------------------------


def _search_profile(
    profiles: dict[str, Any],
    dimension: str,
    keyword: str,
    items_key: str,
    match_fields: list[str],
) -> str:
    """Search within a profile dimension by keyword matching.

    Args:
        profiles: Full project profiles dict.
        dimension: Profile dimension key (e.g. "api_catalog").
        keyword: Search keyword (lowercased by caller).
        items_key: Key within the dimension data that holds the list of items.
        match_fields: Fields used for display (not currently filtered, but
            kept for forward-compatibility with field-level search).

    Returns:
        JSON string of matching items or a human-readable message.
    """
    profile_data = profiles.get(dimension, {})
    if not profile_data:
        return (
            f"No {dimension} data in project profiles. "
            f"Profile scan may not have run yet. Use read_project_file to inspect code directly."
        )

    items = profile_data.get(items_key, [])
    if not items:
        return f"{dimension} data is empty ({items_key} list is empty)."

    keyword_lower = keyword.lower()
    if keyword_lower:
        results = [
            item
            for item in items
            if keyword_lower in json.dumps(item, ensure_ascii=False).lower()
        ]
    else:
        results = items

    if not results:
        return (
            f"No {dimension} data matching '{keyword}'. "
            f"Total entries: {len(items)}."
        )

    truncation_note = ""
    if len(results) > 20:
        results = results[:20]
        truncation_note = (
            f"\n\n... showing first 20 of {len(items)} matching entries"
        )

    return json.dumps(results, ensure_ascii=False, indent=2) + truncation_note


# ---------------------------------------------------------------------------
# Tool implementations
# ---------------------------------------------------------------------------


class QueryApiCatalogTool(BaseTool):
    name = "query_api_catalog"
    description = (
        "查询项目的 API 接口清单。当你需要了解现有 API 路径、HTTP 方法、请求参数、"
        "返回值格式时调用此工具。传入关键词过滤（如 'user' 或 '/api/orders'）。"
    )
    input_model = QueryApiCatalogInput

    def __init__(self, profiles: dict[str, Any]) -> None:
        self._profiles = profiles

    async def execute(
        self, arguments: QueryApiCatalogInput, context: ToolExecutionContext
    ) -> ToolResult:
        output = _search_profile(
            self._profiles,
            "api_catalog",
            arguments.keyword,
            "endpoints",
            ["path", "method", "handler"],
        )
        return ToolResult(output=output)

    def is_read_only(self, arguments: BaseModel) -> bool:
        return True


class QueryDbSchemaTool(BaseTool):
    name = "query_db_schema"
    description = (
        "查询项目的数据库表结构（字段名、类型、索引、外键关系）。"
        "当你需要设计数据模型、编写 SQL、或理解已有表结构时调用。"
    )
    input_model = QueryDbSchemaInput

    def __init__(self, profiles: dict[str, Any]) -> None:
        self._profiles = profiles

    async def execute(
        self, arguments: QueryDbSchemaInput, context: ToolExecutionContext
    ) -> ToolResult:
        output = _search_profile(
            self._profiles,
            "db_schema",
            arguments.table_name,
            "tables",
            ["name", "columns"],
        )
        return ToolResult(output=output)

    def is_read_only(self, arguments: BaseModel) -> bool:
        return True


class QueryBusinessRulesTool(BaseTool):
    name = "query_business_rules"
    description = (
        "查询项目的业务规则约束。例如'积分不能为负'、'订单超时30分钟自动取消'。"
        "当你需要了解业务逻辑边界、验证规则、工作流约束时调用。"
    )
    input_model = QueryBusinessRulesInput

    def __init__(self, profiles: dict[str, Any]) -> None:
        self._profiles = profiles

    async def execute(
        self, arguments: QueryBusinessRulesInput, context: ToolExecutionContext
    ) -> ToolResult:
        output = _search_profile(
            self._profiles,
            "business_rules",
            arguments.domain,
            "rules",
            ["domain", "rule", "source"],
        )
        return ToolResult(output=output)

    def is_read_only(self, arguments: BaseModel) -> bool:
        return True


class QueryModuleGraphTool(BaseTool):
    name = "query_module_graph"
    description = (
        "查询项目的模块依赖关系图。了解代码组织结构、模块边界、import 路径。"
        "当你需要确定新代码应放在哪个模块、或了解模块间调用关系时调用。"
    )
    input_model = QueryModuleGraphInput

    def __init__(self, profiles: dict[str, Any]) -> None:
        self._profiles = profiles

    async def execute(
        self, arguments: QueryModuleGraphInput, context: ToolExecutionContext
    ) -> ToolResult:
        output = _search_profile(
            self._profiles,
            "module_graph",
            arguments.module_name,
            "modules",
            ["name", "path", "depends_on"],
        )
        return ToolResult(output=output)

    def is_read_only(self, arguments: BaseModel) -> bool:
        return True


class ReadProjectFileTool(BaseTool):
    name = "read_project_file"
    description = (
        "读取项目仓库中的源代码文件。当你需要参考现有代码实现风格、"
        "理解已有逻辑、或确认接口定义时调用。返回文件内容（最多 20000 字符）。"
    )
    input_model = ReadProjectFileInput

    def __init__(self, project_id: int) -> None:
        self._project_id = project_id

    async def execute(
        self, arguments: ReadProjectFileInput, context: ToolExecutionContext
    ) -> ToolResult:
        path = arguments.path
        if not path:
            return ToolResult(output="Error: file path is required", is_error=True)

        try:
            async with httpx.AsyncClient(timeout=10) as client:
                resp = await client.get(
                    f"{settings.forge_api_url}/api/projects/{self._project_id}/code/file",
                    params={"path": path, "ref": "main"},
                    headers={
                        "Authorization": f"Bearer {settings.forge_api_token}"
                    },
                )
        except Exception as e:
            return ToolResult(
                output=f"Cannot connect to Forge API: {e}", is_error=True
            )

        if resp.status_code == 200:
            data = resp.json().get("data", {})
            content = data.get("content", "")
            if not content:
                return ToolResult(output=f"File {path} is empty or does not exist.")
            if len(content) > 20000:
                return ToolResult(
                    output=(
                        content[:20000]
                        + f"\n\n... [truncated, showing first 20000 of {len(content)} chars]"
                    )
                )
            return ToolResult(output=content)
        elif resp.status_code == 404:
            return ToolResult(
                output=f"File {path} not found. Check path is correct.",
                is_error=True,
            )
        else:
            return ToolResult(
                output=f"Failed to read {path} (HTTP {resp.status_code})",
                is_error=True,
            )

    def is_read_only(self, arguments: BaseModel) -> bool:
        return True


# ---------------------------------------------------------------------------
# Registration helper
# ---------------------------------------------------------------------------


def register_context_tools(
    registry: ToolRegistry,
    profiles: dict[str, Any],
    project_id: int,
) -> None:
    """Register all five context tools into the given registry."""
    registry.register(QueryApiCatalogTool(profiles))
    registry.register(QueryDbSchemaTool(profiles))
    registry.register(QueryBusinessRulesTool(profiles))
    registry.register(QueryModuleGraphTool(profiles))
    registry.register(ReadProjectFileTool(project_id))
