"""Tool abstractions — BaseTool, SimpleTool, ToolRegistry, ToolResult, ToolExecutionContext.

BaseTool is an async-generator-shaped abstraction: subclasses yield zero
or more StreamEvent instances during execution (to report progress,
thinking indicators, phase transitions, etc.) and then yield exactly
one ToolResult as the final value. The agent loop consumes the
generator, forwards StreamEvents to its own stream, and uses the
ToolResult to build the ToolResultBlock that goes back to the model.

SimpleTool is a convenience subclass for the common case of "tool
completes in one step, no mid-execution events". Subclasses override
_execute_simple and get the async-generator wrapping for free.

This file deliberately imports StreamEvent from engine.stream_events —
stream_events.py is a leaf module (no tools imports) so no cycle.
"""

from __future__ import annotations

from abc import ABC, abstractmethod
from collections.abc import AsyncIterator
from dataclasses import dataclass, field
from pathlib import Path
from typing import Any, Union

from pydantic import BaseModel

from ..engine.stream_events import StreamEvent


@dataclass
class ToolExecutionContext:
    """Runtime context passed to every tool invocation.

    cwd is the resolved workspace root for the current session. Tools
    that accept paths (read_file, write_file, etc.) must resolve them
    relative to cwd via WorkspacePath.resolve().
    """

    cwd: Path
    metadata: dict[str, Any] = field(default_factory=dict)


@dataclass(frozen=True)
class ToolResult:
    """Immutable result returned by a tool execution.

    Tools must yield exactly one of these as the final item in their
    execute() generator. is_error=True signals that the tool caught a
    recoverable failure (invalid input, file not found, etc.) — the
    agent sees this as a ToolResultBlock(is_error=True) and can react.
    Uncaught Python exceptions are a bug; tools should catch them
    internally and return ToolResult(is_error=True, output=...).
    """

    output: str
    is_error: bool = False
    metadata: dict[str, Any] = field(default_factory=dict)


# The item type yielded by BaseTool.execute. A stream of zero or more
# StreamEvents, terminated by exactly one ToolResult.
ToolItem = Union[StreamEvent, ToolResult]


class BaseTool(ABC):
    """Abstract base for all OpenHarness tools.

    Subclasses MUST implement execute() as an async generator. They
    yield StreamEvents freely during execution and yield ToolResult
    exactly once as the final item.

    For the common case of "simple tool with no mid-execution events",
    subclass SimpleTool instead — it handles the generator wrapping.
    """

    name: str
    description: str
    input_model: type[BaseModel]

    @abstractmethod
    def execute(
        self,
        arguments: BaseModel,
        context: ToolExecutionContext,
    ) -> AsyncIterator[ToolItem]:
        """Run the tool. Yields zero or more StreamEvents followed by
        exactly one ToolResult. Must not raise on expected errors —
        return ToolResult(is_error=True, output=...) instead.

        This is a method that returns an AsyncIterator, which in Python
        means: subclasses define it with 'async def' and 'yield', and
        callers use 'async for item in tool.execute(...)'.
        """
        ...

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


class SimpleTool(BaseTool):
    """Convenience subclass for tools that don't emit mid-execution events.

    Subclasses override _execute_simple() which returns a ToolResult
    directly (no yield). SimpleTool.execute wraps it in a one-shot
    async generator so the BaseTool contract is preserved without
    every subclass having to write `yield await self._execute_simple(...)`
    boilerplate.

    Example:
        class MyTool(SimpleTool):
            name = "my_tool"
            description = "..."
            input_model = MyInput

            async def _execute_simple(self, arguments, context):
                return ToolResult(output="done")
    """

    @abstractmethod
    async def _execute_simple(
        self,
        arguments: BaseModel,
        context: ToolExecutionContext,
    ) -> ToolResult:
        """Run the tool and return a ToolResult. No yielding."""
        ...

    async def execute(
        self,
        arguments: BaseModel,
        context: ToolExecutionContext,
    ) -> AsyncIterator[ToolItem]:
        """Adapt _execute_simple into the async-generator contract."""
        result = await self._execute_simple(arguments, context)
        yield result


class ToolRegistry:
    """Registry that holds named tool instances.

    Not thread-safe — intended to be built once at engine construction
    and read concurrently thereafter.
    """

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
