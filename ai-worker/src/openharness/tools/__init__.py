"""OpenHarness Tool Infrastructure — BaseTool, ToolRegistry, context tools, file tools, exec tools."""

from src.openharness.tools.base import (
    BaseTool,
    SimpleTool,
    ToolExecutionContext,
    ToolItem,
    ToolRegistry,
    ToolResult,
)
from src.openharness.tools.context_tools import register_context_tools
from src.openharness.tools.file_tools import (
    EditFileTool,
    GlobTool,
    GrepTool,
    ListDirectoryTool,
    ReadFileTool,
    WriteFileTool,
    register_file_tools,
)
from src.openharness.tools.phase_tool import SetPhaseInput, SetPhaseTool
from src.openharness.tools.bash_tool import BashInput, BashTool, register_exec_tools
from src.openharness.tools.workspace_path import PathEscapeError, WorkspacePath


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
    "BashInput",
    "BashTool",
    "EditFileTool",
    "GlobTool",
    "GrepTool",
    "ListDirectoryTool",
    "PathEscapeError",
    "ReadFileTool",
    "SetPhaseInput",
    "SetPhaseTool",
    "SimpleTool",
    "ToolExecutionContext",
    "ToolItem",
    "ToolRegistry",
    "ToolResult",
    "WorkspacePath",
    "WriteFileTool",
    "create_default_tool_registry",
    "register_context_tools",
    "register_exec_tools",
    "register_file_tools",
]
