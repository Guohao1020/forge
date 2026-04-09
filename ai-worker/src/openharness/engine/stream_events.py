from __future__ import annotations

from dataclasses import dataclass

from ..api.usage import UsageSnapshot
from .messages import ConversationMessage


class StreamEvent:
    """Marker base class for all stream events emitted by pipeline iterators.

    Used by api_server._route_and_stream to filter out non-event yields
    (such as CycleResult or PairPipelineResult) via isinstance checks.
    Empty on purpose: no shared state, no methods.
    """
    pass


@dataclass(frozen=True)
class AssistantTextDelta(StreamEvent):
    """Streaming text chunk from the assistant."""
    text: str


@dataclass(frozen=True)
class AssistantTurnComplete(StreamEvent):
    """Emitted when an assistant turn finishes."""
    message: ConversationMessage
    usage: UsageSnapshot


@dataclass(frozen=True)
class ToolExecutionStarted(StreamEvent):
    """Emitted when a tool begins executing."""
    tool_name: str
    tool_input: dict


@dataclass(frozen=True)
class ToolExecutionCompleted(StreamEvent):
    """Emitted when a tool finishes executing."""
    tool_name: str
    output: str
    is_error: bool


@dataclass(frozen=True)
class ErrorEvent(StreamEvent):
    """Emitted on error during engine execution."""
    message: str
    recoverable: bool


@dataclass(frozen=True)
class ThinkingStarted(StreamEvent):
    """Emitted when the agent starts a sustained thinking/tool phase.

    Frontend renders this as a pulsing indicator under the current AI
    message. Optional `label` overrides the default "Thinking..." text
    (e.g. "Running read_file...", "Analyzing project...").
    """
    label: str = "Thinking"


@dataclass(frozen=True)
class ThinkingStopped(StreamEvent):
    """Emitted when the agent finishes a thinking/tool phase."""
    pass


@dataclass(frozen=True)
class FixLoopStarted(StreamEvent):
    """Emitted when the pair pipeline enters a build-fix cycle.

    `cycle` is 1-indexed, `max_cycles` is the configured limit, `errors`
    is a best-effort count of compilation errors in the last build output.
    """
    cycle: int
    max_cycles: int
    errors: int = 0


@dataclass(frozen=True)
class FixLoopCompleted(StreamEvent):
    """Emitted when a fix-loop cycle finishes (successful build or
    reviewer decision)."""
    cycle: int
    success: bool


@dataclass(frozen=True)
class SessionComplete(StreamEvent):
    """Emitted when an agent turn finishes, regardless of success/failure.

    Carries the aggregate stats the SummaryCard needs.
    """
    files_created: int
    files_modified: int
    build_status: str  # "passed" | "failed" | "skipped"
    duration_ms: int
    tokens_total: int
    cost_usd: float
