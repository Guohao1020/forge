"""Tests for RequestClarificationTool — the meta-tool that pauses the
agent mid-turn to ask the user a clarifying question.

Spec reference: §2.9.2.d, §4.1 contract.
"""

import asyncio
import os
from pathlib import Path
from unittest.mock import patch

import pytest
from pydantic import ValidationError

from src.openharness.engine.agent_hooks import (
    ClarificationCoordinator,
    ClarificationTimeout,
)
from src.openharness.engine.stream_events import ClarificationRequested
from src.openharness.tools.base import ToolExecutionContext, ToolResult


@pytest.fixture
def coordinator():
    return ClarificationCoordinator()


@pytest.fixture
def tool_ctx(tmp_path, coordinator):
    return ToolExecutionContext(
        cwd=tmp_path,
        tool_use_id="toolu_test_123",
        clarification_coordinator=coordinator,
    )


@pytest.fixture
def clarification_tool():
    from src.openharness.tools.interaction_tools import RequestClarificationTool
    return RequestClarificationTool()


async def _collect(tool, arguments, ctx):
    """Collect all yielded items from a BaseTool.execute() async generator."""
    items = []
    async for item in tool.execute(arguments, ctx):
        items.append(item)
    return items


@pytest.mark.asyncio
async def test_happy_path_yields_clarification_then_result(
    clarification_tool, tool_ctx, coordinator,
):
    """Tool yields ClarificationRequested, then ToolResult with the delivered response."""
    from src.openharness.tools.interaction_tools import ClarificationInput

    async def _deliver():
        await asyncio.sleep(0.05)
        coordinator.deliver("toolu_test_123", "use TypeScript please")

    asyncio.create_task(_deliver())

    items = await _collect(
        clarification_tool,
        ClarificationInput(question="What language should I use?"),
        tool_ctx,
    )

    assert len(items) == 2
    assert isinstance(items[0], ClarificationRequested)
    assert items[0].question == "What language should I use?"
    assert items[0].tool_use_id == "toolu_test_123"
    assert isinstance(items[1], ToolResult)
    assert items[1].output == "use TypeScript please"
    assert not items[1].is_error


@pytest.mark.asyncio
async def test_timeout_raises_clarification_timeout(
    clarification_tool, tool_ctx,
):
    """When no response arrives within timeout, ClarificationTimeout is raised."""
    from src.openharness.tools.interaction_tools import ClarificationInput

    with patch.dict(os.environ, {"FORGE_CLARIFICATION_TIMEOUT_SECONDS": "0.2"}):
        # Re-import to pick up the patched env
        import importlib
        import src.openharness.tools.interaction_tools as mod
        importlib.reload(mod)
        tool = mod.RequestClarificationTool()

        with pytest.raises(ClarificationTimeout) as exc_info:
            await _collect(
                tool,
                mod.ClarificationInput(question="What language?"),
                tool_ctx,
            )

        assert exc_info.value.tool_use_id == "toolu_test_123"


@pytest.mark.asyncio
async def test_cancellation_propagates(
    clarification_tool, tool_ctx,
):
    """CancelledError propagates cleanly (session teardown)."""
    from src.openharness.tools.interaction_tools import ClarificationInput

    async def _cancel_after_delay():
        await asyncio.sleep(0.05)
        task.cancel()

    async def _run():
        return await _collect(
            clarification_tool,
            ClarificationInput(question="What language?"),
            tool_ctx,
        )

    task = asyncio.create_task(_run())
    asyncio.create_task(_cancel_after_delay())

    with pytest.raises(asyncio.CancelledError):
        await task


def test_empty_question_rejected():
    """Empty question string must be rejected by Pydantic validation."""
    from src.openharness.tools.interaction_tools import ClarificationInput
    with pytest.raises(ValidationError):
        ClarificationInput(question="")


def test_oversized_question_rejected():
    """Question exceeding 4 KiB must be rejected."""
    from src.openharness.tools.interaction_tools import ClarificationInput
    with pytest.raises(ValidationError):
        ClarificationInput(question="x" * 4097)


def test_valid_question_accepted():
    """Question within limits is accepted."""
    from src.openharness.tools.interaction_tools import ClarificationInput
    inp = ClarificationInput(question="What language?")
    assert inp.question == "What language?"
