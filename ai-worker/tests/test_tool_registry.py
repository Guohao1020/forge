"""Tests for OpenHarness tool abstractions and context tools."""

import pytest
from pathlib import Path

from pydantic import BaseModel

from src.openharness.tools.base import (
    BaseTool,
    SimpleTool,
    ToolExecutionContext,
    ToolRegistry,
    ToolResult,
)
from src.openharness.tools.context_tools import (
    register_context_tools,
    _search_profile,
)


# ---------------------------------------------------------------------------
# Fixture: EchoTool for base-layer tests
# ---------------------------------------------------------------------------


class EchoInput(BaseModel):
    text: str


class EchoTool(SimpleTool):
    name = "echo"
    description = "Echo input text"
    input_model = EchoInput

    async def _execute_simple(
        self, arguments: EchoInput, context: ToolExecutionContext
    ) -> ToolResult:
        return ToolResult(output=arguments.text)

    def is_read_only(self, arguments: EchoInput) -> bool:
        return True


# ---------------------------------------------------------------------------
# ToolRegistry tests
# ---------------------------------------------------------------------------


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
    items = []
    async for item in tool.execute(EchoInput(text="hello"), ctx):
        items.append(item)
    assert len(items) == 1
    result = items[0]
    assert result.output == "hello"
    assert not result.is_error


def test_tool_read_only():
    tool = EchoTool()
    assert tool.is_read_only(EchoInput(text="x")) is True


# ---------------------------------------------------------------------------
# Context tools registration tests
# ---------------------------------------------------------------------------


def test_register_context_tools():
    registry = ToolRegistry()
    profiles = {
        "api_catalog": {
            "endpoints": [{"path": "/api/users", "method": "GET"}]
        }
    }
    register_context_tools(registry, profiles, project_id=1)
    assert registry.get("query_api_catalog") is not None
    assert registry.get("query_db_schema") is not None
    assert registry.get("query_business_rules") is not None
    assert registry.get("query_module_graph") is not None
    assert registry.get("read_project_file") is not None
    assert len(registry.list_tools()) == 5


# ---------------------------------------------------------------------------
# _search_profile tests
# ---------------------------------------------------------------------------


def test_search_profile_found():
    profiles = {
        "api_catalog": {
            "endpoints": [
                {
                    "path": "/api/users",
                    "method": "GET",
                    "handler": "listUsers",
                },
                {
                    "path": "/api/orders",
                    "method": "POST",
                    "handler": "createOrder",
                },
            ]
        }
    }
    result = _search_profile(
        profiles, "api_catalog", "user", "endpoints", ["path"]
    )
    assert "users" in result


def test_search_profile_empty():
    result = _search_profile(
        {}, "api_catalog", "user", "endpoints", ["path"]
    )
    assert "No api_catalog data" in result


def test_search_profile_no_match():
    profiles = {
        "api_catalog": {
            "endpoints": [
                {"path": "/api/orders", "method": "POST"},
            ]
        }
    }
    result = _search_profile(
        profiles, "api_catalog", "zzzzz", "endpoints", ["path"]
    )
    assert "No api_catalog data matching" in result


def test_search_profile_empty_items():
    profiles = {"db_schema": {"tables": []}}
    result = _search_profile(
        profiles, "db_schema", "user", "tables", ["name"]
    )
    assert "empty" in result


def test_search_profile_no_keyword_returns_all():
    profiles = {
        "module_graph": {
            "modules": [
                {"name": "auth", "path": "/auth"},
                {"name": "user", "path": "/user"},
            ]
        }
    }
    result = _search_profile(
        profiles, "module_graph", "", "modules", ["name"]
    )
    assert "auth" in result
    assert "user" in result
