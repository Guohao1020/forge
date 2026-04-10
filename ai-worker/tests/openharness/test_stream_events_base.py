"""Smoke tests for stream_events dataclasses.

Full tool-level behavior lives in the per-tool test files. This
file just verifies the dataclasses construct correctly, hold the
right fields, and don't accidentally stop being frozen.
"""

from __future__ import annotations

import pytest

from src.openharness.engine.stream_events import (
    AssistantTextDelta,
    AssistantTurnComplete,
    ClarificationRequested,
    ErrorEvent,
    PhaseChanged,
    SessionComplete,
    StreamEvent,
    ThinkingStarted,
    ThinkingStopped,
    ToolExecutionCompleted,
    ToolExecutionStarted,
)


def test_all_events_subclass_stream_event():
    """Every public event dataclass must subclass StreamEvent so
    the agent loop's isinstance filter in _execute_tool_call works.
    """
    for cls in (
        AssistantTextDelta,
        AssistantTurnComplete,
        ClarificationRequested,
        ErrorEvent,
        PhaseChanged,
        SessionComplete,
        ThinkingStarted,
        ThinkingStopped,
        ToolExecutionCompleted,
        ToolExecutionStarted,
    ):
        assert issubclass(cls, StreamEvent), f"{cls.__name__} does not subclass StreamEvent"


def test_phase_changed_construction():
    evt = PhaseChanged(phase="Analyze")
    assert evt.phase == "Analyze"


def test_phase_changed_is_frozen():
    evt = PhaseChanged(phase="Build")
    with pytest.raises(Exception):  # FrozenInstanceError or AttributeError
        evt.phase = "Test"  # type: ignore[misc]


def test_tool_execution_started_has_tool_use_id():
    evt = ToolExecutionStarted(
        tool_use_id="toolu_abc123",
        tool_name="read_file",
        tool_input={"path": "foo.go"},
    )
    assert evt.tool_use_id == "toolu_abc123"
    assert evt.tool_name == "read_file"
    assert evt.tool_input == {"path": "foo.go"}


def test_tool_execution_completed_has_tool_use_id():
    evt = ToolExecutionCompleted(
        tool_use_id="toolu_abc123",
        tool_name="read_file",
        output="hello",
        is_error=False,
    )
    assert evt.tool_use_id == "toolu_abc123"


def test_fix_loop_events_are_gone():
    """FixLoopStarted and FixLoopCompleted were deleted in Phase 4.
    Importing them should fail -- this test documents the removal
    and breaks if anyone adds them back without thinking."""
    from src.openharness.engine import stream_events
    assert not hasattr(stream_events, "FixLoopStarted")
    assert not hasattr(stream_events, "FixLoopCompleted")


def test_thinking_started_has_label():
    evt = ThinkingStarted(label="Running go build")
    assert evt.label == "Running go build"


def test_thinking_stopped_constructs():
    evt = ThinkingStopped()
    assert isinstance(evt, StreamEvent)


def test_clarification_requested_construction():
    evt = ClarificationRequested(
        question="What test framework should I use?",
        tool_use_id="toolu_abc123",
    )
    assert evt.question == "What test framework should I use?"
    assert evt.tool_use_id == "toolu_abc123"


def test_clarification_requested_is_frozen():
    evt = ClarificationRequested(question="x", tool_use_id="toolu_xyz")
    with pytest.raises(Exception):  # FrozenInstanceError or AttributeError
        evt.question = "y"  # type: ignore[misc]


def test_clarification_requested_subclasses_stream_event():
    assert issubclass(ClarificationRequested, StreamEvent)
