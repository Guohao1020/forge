"""OpenHarness Tool Infrastructure — BaseTool, ToolRegistry, context tools."""

from src.openharness.tools.base import (
    BaseTool,
    SimpleTool,
    ToolExecutionContext,
    ToolItem,
    ToolRegistry,
    ToolResult,
)
from src.openharness.tools.context_tools import register_context_tools


def create_default_tool_registry(
    profiles: dict,
    project_id: int,
) -> ToolRegistry:
    """Create a ToolRegistry pre-loaded with the standard context tools."""
    registry = ToolRegistry()
    register_context_tools(registry, profiles, project_id)
    return registry


__all__ = [
    "BaseTool",
    "SimpleTool",
    "ToolExecutionContext",
    "ToolItem",
    "ToolRegistry",
    "ToolResult",
    "create_default_tool_registry",
    "register_context_tools",
]
