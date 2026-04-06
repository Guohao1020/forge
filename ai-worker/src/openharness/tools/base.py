"""Tool abstractions -- BaseTool, ToolRegistry, ToolResult."""

from __future__ import annotations

from abc import ABC, abstractmethod
from dataclasses import dataclass, field
from pathlib import Path
from typing import Any

from pydantic import BaseModel


@dataclass
class ToolExecutionContext:
    """Runtime context passed to every tool invocation."""

    cwd: Path
    metadata: dict[str, Any] = field(default_factory=dict)


@dataclass(frozen=True)
class ToolResult:
    """Immutable result returned by a tool execution."""

    output: str
    is_error: bool = False
    metadata: dict[str, Any] = field(default_factory=dict)


class BaseTool(ABC):
    """Abstract base for all OpenHarness tools."""

    name: str
    description: str
    input_model: type[BaseModel]

    @abstractmethod
    async def execute(
        self, arguments: BaseModel, context: ToolExecutionContext
    ) -> ToolResult: ...

    def is_read_only(self, arguments: BaseModel) -> bool:
        """Whether this invocation is purely read-only (safe to run in parallel)."""
        return False

    def to_api_schema(self) -> dict[str, Any]:
        """Serialize to Anthropic-compatible tool definition."""
        return {
            "name": self.name,
            "description": self.description,
            "input_schema": self.input_model.model_json_schema(),
        }


class ToolRegistry:
    """Registry that holds named tool instances."""

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
