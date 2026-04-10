"""Tests for SessionHaltError handling in _execute_tool_call.

Verifies that ClarificationTimeout and ReturnChannelError are caught
by the SessionHaltError handler (not the generic except Exception)
and produce ToolResultBlock(is_error=True) with "session halted" content.

Spec reference: §4.1 updated contract, §2.9.2.f.
"""

import asyncio
from pathlib import Path
from unittest.mock import AsyncMock, MagicMock

import pytest

from src.openharness.engine.agent_hooks import (
    ClarificationTimeout,
    ReturnChannelError,
    SessionHaltError,
)
from src.openharness.engine.query import QueryContext, _execute_tool_call
from src.openharness.engine.stream_events import ErrorEvent
from src.openharness.engine.messages import ToolResultBlock
from src.openharness.tools.base import BaseTool, ToolExecutionContext, ToolResult, ToolRegistry


class _HaltingTool(BaseTool):
    """Test tool that raises a SessionHaltError subclass."""

    name = "halting_tool"
    description = "A tool that halts the session."

    class _Input:
        @classmethod
        def model_validate(cls, data):
            return cls()

        @classmethod
        def model_json_schema(cls):
            return {"type": "object", "properties": {}}

    input_model = _Input

    def __init__(self, error: Exception) -> None:
        self._error = error

    async def execute(self, arguments, context):
        raise self._error
        yield  # Make this an async generator (unreachable but needed for type)


def _make_context(tool: BaseTool) -> QueryContext:
    registry = ToolRegistry()
    registry.register(tool)
    return QueryContext(
        api_client=MagicMock(),
        tool_registry=registry,
        model="test-model",
        system_prompt="test prompt",
        cwd=Path("/tmp/test"),
    )


async def _collect_items(context, tool_name, tool_use_id, tool_input):
    """Collect all items yielded by _execute_tool_call."""
    items = []
    async for item in _execute_tool_call(context, tool_name, tool_use_id, tool_input):
        items.append(item)
    return items


@pytest.mark.asyncio
async def test_clarification_timeout_yields_error_result():
    """ClarificationTimeout produces ToolResultBlock(is_error=True)
    with 'session halted' in content."""
    error = ClarificationTimeout("toolu_timeout", 600.0)
    tool = _HaltingTool(error)
    ctx = _make_context(tool)

    items = await _collect_items(ctx, "halting_tool", "toolu_timeout", {})

    # Should yield exactly one ToolResultBlock
    result_blocks = [i for i in items if isinstance(i, ToolResultBlock)]
    assert len(result_blocks) == 1
    result = result_blocks[0]
    assert result.is_error
    assert "session halted" in result.content.lower()
    assert "ClarificationTimeout" in result.content


@pytest.mark.asyncio
async def test_return_channel_error_yields_error_result():
    """ReturnChannelError produces the same terminal pattern."""
    error = ReturnChannelError("Redis connection lost")
    tool = _HaltingTool(error)
    ctx = _make_context(tool)

    items = await _collect_items(ctx, "halting_tool", "toolu_rce", {})

    result_blocks = [i for i in items if isinstance(i, ToolResultBlock)]
    assert len(result_blocks) == 1
    result = result_blocks[0]
    assert result.is_error
    assert "session halted" in result.content.lower()
    assert "ReturnChannelError" in result.content


@pytest.mark.asyncio
async def test_generic_exception_still_caught():
    """Non-SessionHaltError exceptions are still caught by the generic handler."""
    tool = _HaltingTool(RuntimeError("kaboom"))
    ctx = _make_context(tool)

    items = await _collect_items(ctx, "halting_tool", "toolu_generic", {})

    result_blocks = [i for i in items if isinstance(i, ToolResultBlock)]
    assert len(result_blocks) == 1
    result = result_blocks[0]
    assert result.is_error
    assert "kaboom" in result.content
