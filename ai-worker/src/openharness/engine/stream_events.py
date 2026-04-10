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
    tool_use_id: str
    tool_name: str
    tool_input: dict


@dataclass(frozen=True)
class ToolExecutionCompleted(StreamEvent):
    """Emitted when a tool finishes executing."""
    tool_use_id: str
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
class PhaseChanged(StreamEvent):
    """Emitted by SetPhaseTool to signal a phase transition in the
    7-step workflow shown by the frontend ribbon.

    Valid phases: "Analyze", "Plan", "Generate", "Build", "Test",
    "Review", "Deploy". The agent calls set_phase with the new
    phase name and this event is yielded verbatim to the frontend.

    The literal set is NOT enforced at this dataclass level -- it's
    a Pydantic Literal type on SetPhaseInput (phase_tool.py), which
    catches invalid phases at tool-input-validation time before
    the event is ever constructed.
    """
    phase: str


@dataclass(frozen=True)
class ClarificationRequested(StreamEvent):
    """Agent paused mid-tool-execution to ask the user a clarifying question.

    Emitted by RequestClarificationTool (Phase 5a) when the agent needs
    more information from the user. The tool awaits a response on the
    session's Redis return channel (agent:return:{session_id}) and
    yields ToolResult(output=<user_response>) once the response arrives.

    tool_use_id threads through from the surrounding ToolExecutionStarted
    event so the frontend can correlate the input form with the right
    tool card, and the backend can correlate the return-channel
    ClarificationResponse with the right pending future.
    """
    question: str
    tool_use_id: str


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
