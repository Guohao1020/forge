"""Tests for SetPhaseTool.

SetPhaseTool is the smallest tool in the T2 set but architecturally
important as the first tool to yield a StreamEvent (PhaseChanged)
before its ToolResult. These tests verify the contract end-to-end.
"""

import pytest
from pydantic import ValidationError

from src.openharness.engine.stream_events import PhaseChanged, StreamEvent
from src.openharness.tools.base import ToolExecutionContext, ToolResult


@pytest.fixture
def phase_tool():
    from src.openharness.tools.phase_tool import SetPhaseTool
    return SetPhaseTool()


@pytest.fixture
def tool_ctx(tmp_path):
    return ToolExecutionContext(cwd=tmp_path)


async def _collect(tool, arguments, ctx):
    items = []
    async for item in tool.execute(arguments, ctx):
        items.append(item)
    return items


@pytest.mark.asyncio
async def test_set_phase_emits_phase_changed_then_result(phase_tool, tool_ctx):
    from src.openharness.tools.phase_tool import SetPhaseInput

    items = await _collect(
        phase_tool,
        SetPhaseInput(phase="Analyze"),
        tool_ctx,
    )
    # Expected order: PhaseChanged, then ToolResult
    assert len(items) == 2
    assert isinstance(items[0], PhaseChanged)
    assert items[0].phase == "Analyze"
    assert isinstance(items[1], ToolResult)
    assert not items[1].is_error
    assert "Analyze" in items[1].output


@pytest.mark.asyncio
@pytest.mark.parametrize("phase", [
    "Analyze", "Plan", "Generate", "Build", "Test", "Review", "Deploy",
])
async def test_set_phase_accepts_all_seven_phases(phase_tool, tool_ctx, phase):
    from src.openharness.tools.phase_tool import SetPhaseInput

    items = await _collect(phase_tool, SetPhaseInput(phase=phase), tool_ctx)
    phase_evt = next(i for i in items if isinstance(i, PhaseChanged))
    assert phase_evt.phase == phase


def test_set_phase_rejects_invalid_phase_at_input_validation():
    """Pydantic should reject invalid phase values before execute runs."""
    from src.openharness.tools.phase_tool import SetPhaseInput

    with pytest.raises(ValidationError):
        SetPhaseInput(phase="Analyse")  # British spelling -- wrong
    with pytest.raises(ValidationError):
        SetPhaseInput(phase="")
    with pytest.raises(ValidationError):
        SetPhaseInput(phase="Debugging")  # not in the Literal set


@pytest.mark.asyncio
async def test_set_phase_is_read_only(phase_tool):
    from src.openharness.tools.phase_tool import SetPhaseInput

    args = SetPhaseInput(phase="Review")
    assert phase_tool.is_read_only(args) is True


@pytest.mark.asyncio
async def test_set_phase_contract_single_tool_result(phase_tool, tool_ctx):
    """The BaseTool contract says 'exactly one ToolResult'. This test
    verifies SetPhaseTool doesn't accidentally yield two ToolResults
    (e.g., from a copy-paste mistake in execute)."""
    from src.openharness.tools.phase_tool import SetPhaseInput

    items = await _collect(phase_tool, SetPhaseInput(phase="Generate"), tool_ctx)
    tool_results = [i for i in items if isinstance(i, ToolResult)]
    assert len(tool_results) == 1
    # The ToolResult must be LAST
    assert isinstance(items[-1], ToolResult)
