"""SetPhaseTool -- signal a 7-step workflow phase transition.

Architecturally the first tool in the T2 set to yield a StreamEvent
mid-execution. SetPhaseTool extends BaseTool directly (not
SimpleTool) so it can yield a typed PhaseChanged event before its
final ToolResult. Spec section 4.11 explicitly calls out why SimpleTool
doesn't fit: SimpleTool only yields a ToolResult, and 'sniff the
tool name in query.py to decide whether to emit PhaseChanged' is
the hardcoded-special-case anti-pattern the Phase 2 refactor was
designed to eliminate.

The 7-phase Literal set is enforced at Pydantic input validation,
so an invalid phase value never reaches execute() -- the agent gets
a ValidationError in the ToolResultBlock path instead, which it
can see and retry.
"""

from __future__ import annotations

from typing import Literal

from pydantic import BaseModel, Field

from ..engine.stream_events import PhaseChanged
from .base import BaseTool, ToolExecutionContext, ToolResult


# The 7 phases of the Variant B workflow ribbon. The frontend's
# step-ribbon.tsx component hard-codes the same 7 labels.
Phase = Literal[
    "Analyze", "Plan", "Generate", "Build", "Test", "Review", "Deploy"
]


class SetPhaseInput(BaseModel):
    phase: Phase = Field(
        ...,
        description=(
            "The phase to transition to. Must be exactly one of: "
            "Analyze, Plan, Generate, Build, Test, Review, Deploy. "
            "The UI step ribbon will highlight this phase. You can "
            "go backwards (e.g. Build -> Generate to fix a compile "
            "error)."
        ),
    )


class SetPhaseTool(BaseTool):
    name = "set_phase"
    description = (
        "Signal which phase you're currently in. The UI step ribbon "
        "will highlight that phase. Available phases: Analyze "
        "(understanding requirements and code), Plan (deciding "
        "changes), Generate (writing code), Build (compiling), Test "
        "(running tests), Review (verifying own work), Deploy "
        "(committing or preparing deployment). Call this when you "
        "start a new phase. You can go backwards (e.g., Build -> "
        "Generate to fix a compile error)."
    )
    input_model = SetPhaseInput

    def is_read_only(self, arguments: BaseModel) -> bool:
        # set_phase doesn't touch the filesystem or run subprocesses.
        # It's a pure UI signal.
        return True

    async def execute(self, arguments: SetPhaseInput, context: ToolExecutionContext):
        yield PhaseChanged(phase=arguments.phase)
        yield ToolResult(output=f"Phase set to {arguments.phase}")
