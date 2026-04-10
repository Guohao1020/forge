"""Interaction meta-tools — tools that pause the agent to interact with
the user (clarification) or with a reviewer LLM (review).

RequestClarificationTool lives here. RequestReviewTool is added in
Phase 5 Task 5.13.

Spec reference: §2.9.2.d (request_clarification).
"""

from __future__ import annotations

import asyncio
import logging
import os
from collections.abc import AsyncIterator
from typing import Union

from pydantic import BaseModel, Field, field_validator

from ..engine.agent_hooks import ClarificationTimeout
from ..engine.stream_events import ClarificationRequested
from .base import BaseTool, ToolExecutionContext, ToolResult

logger = logging.getLogger(__name__)

# Default: 10 minutes. Configurable via env for testing.
CLARIFICATION_TIMEOUT_SECONDS: float = float(
    os.environ.get("FORGE_CLARIFICATION_TIMEOUT_SECONDS", "600")
)


class ClarificationInput(BaseModel):
    """Input schema for request_clarification."""

    question: str = Field(
        ...,
        min_length=1,
        max_length=4096,
        description="The clarifying question to ask the user.",
    )

    @field_validator("question")
    @classmethod
    def question_not_blank(cls, v: str) -> str:
        if not v.strip():
            raise ValueError("question must not be blank")
        return v


class RequestClarificationTool(BaseTool):
    """Meta-tool that pauses the agent mid-turn to ask the user a
    clarifying question.

    Extends BaseTool (not SimpleTool) because it yields a
    ClarificationRequested StreamEvent before its terminal ToolResult.
    The pause is an asyncio.Future await — no thread is blocked.

    On timeout, raises ClarificationTimeout (a SessionHaltError) which
    _execute_tool_call catches and translates to
    ErrorEvent(recoverable=False) + terminal ToolResultBlock(is_error=True).
    """

    name = "request_clarification"
    description = (
        "Ask the user a clarifying question and wait for their response. "
        "Use when the request is ambiguous or missing critical details. "
        "The agent pauses until the user responds (timeout: 10 minutes)."
    )
    input_model = ClarificationInput

    async def execute(
        self,
        arguments: BaseModel,
        context: ToolExecutionContext,
    ) -> AsyncIterator[Union[ClarificationRequested, ToolResult]]:
        tool_use_id = context.tool_use_id
        if tool_use_id is None:
            raise RuntimeError(
                "RequestClarificationTool requires tool_use_id on context"
            )

        coordinator = context.clarification_coordinator
        if coordinator is None:
            raise RuntimeError(
                "RequestClarificationTool requires clarification_coordinator on context"
            )

        # 1. Yield the clarification event — frontend renders the input box
        yield ClarificationRequested(
            question=arguments.question,
            tool_use_id=tool_use_id,
        )

        # 2. Await the user's response via the coordinator
        try:
            response = await coordinator.wait_for(
                tool_use_id,
                timeout=CLARIFICATION_TIMEOUT_SECONDS,
            )
        except asyncio.TimeoutError:
            # Fail-fast: halt the session per §2.9.2.f
            raise ClarificationTimeout(tool_use_id, CLARIFICATION_TIMEOUT_SECONDS)
        except asyncio.CancelledError:
            # Session is being torn down; propagate cleanly
            raise

        # 3. Yield the terminal ToolResult with the user's answer
        yield ToolResult(output=response, is_error=False)

    def is_read_only(self, arguments: BaseModel) -> bool:
        return True
