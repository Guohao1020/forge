"""StreamEvent marker base class contract.

Every stream event dataclass emitted by the pipeline must inherit from
StreamEvent so `isinstance(event, StreamEvent)` filtering works in
api_server._route_and_stream.
"""

from __future__ import annotations

import pytest

from src.openharness.engine.stream_events import (
    AssistantTextDelta,
    AssistantTurnComplete,
    ErrorEvent,
    FixLoopCompleted,
    FixLoopStarted,
    SessionComplete,
    StreamEvent,
    ThinkingStarted,
    ThinkingStopped,
    ToolExecutionCompleted,
    ToolExecutionStarted,
)


ALL_EVENT_CLASSES = [
    AssistantTextDelta,
    AssistantTurnComplete,
    ToolExecutionStarted,
    ToolExecutionCompleted,
    ErrorEvent,
    ThinkingStarted,
    ThinkingStopped,
    FixLoopStarted,
    FixLoopCompleted,
    SessionComplete,
]


@pytest.mark.parametrize("cls", ALL_EVENT_CLASSES)
def test_stream_event_classes_inherit_from_stream_event(cls):
    """All 10 event dataclasses must be StreamEvent subclasses."""
    assert issubclass(cls, StreamEvent), (
        f"{cls.__name__} must inherit from StreamEvent so "
        f"isinstance-based filtering in _route_and_stream works."
    )
