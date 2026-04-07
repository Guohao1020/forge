"""API client protocol and event types for the QueryEngine abstraction layer."""

from __future__ import annotations

from dataclasses import dataclass, field
from typing import Any, AsyncIterator, Dict, List, Optional, Protocol

from ..engine.messages import ConversationMessage
from .usage import UsageSnapshot


@dataclass(frozen=True)
class ApiMessageRequest:
    model: str
    messages: List[ConversationMessage]
    system_prompt: Optional[str] = None
    max_tokens: int = 4096
    tools: Optional[List[Dict[str, Any]]] = None


@dataclass(frozen=True)
class ApiTextDeltaEvent:
    text: str


@dataclass(frozen=True)
class ApiToolUseStartEvent:
    tool_use_id: str
    name: str


@dataclass(frozen=True)
class ApiMessageCompleteEvent:
    message: ConversationMessage
    usage: UsageSnapshot
    stop_reason: Optional[str] = None


ApiStreamEvent = Any  # Union of the above event types


class SupportsStreamingMessages(Protocol):
    def stream_message(
        self, request: ApiMessageRequest,
    ) -> AsyncIterator[ApiStreamEvent]:
        ...
