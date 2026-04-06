from __future__ import annotations

from dataclasses import dataclass
from typing import Union

from ..api.usage import UsageSnapshot
from .messages import ConversationMessage


@dataclass(frozen=True)
class AssistantTextDelta:
    """Streaming text chunk from the assistant."""
    text: str


@dataclass(frozen=True)
class AssistantTurnComplete:
    """Emitted when an assistant turn finishes."""
    message: ConversationMessage
    usage: UsageSnapshot


@dataclass(frozen=True)
class ToolExecutionStarted:
    """Emitted when a tool begins executing."""
    tool_name: str
    tool_input: dict


@dataclass(frozen=True)
class ToolExecutionCompleted:
    """Emitted when a tool finishes executing."""
    tool_name: str
    output: str
    is_error: bool


@dataclass(frozen=True)
class ErrorEvent:
    """Emitted on error during engine execution."""
    message: str
    recoverable: bool


StreamEvent = Union[
    AssistantTextDelta,
    AssistantTurnComplete,
    ToolExecutionStarted,
    ToolExecutionCompleted,
    ErrorEvent,
]
