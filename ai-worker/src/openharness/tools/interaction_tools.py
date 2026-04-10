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


# ---------------------------------------------------------------------------
# RequestReviewTool — Round 2 §2.9.3
# ---------------------------------------------------------------------------


from pathlib import Path
from typing import TYPE_CHECKING

from src.openharness.engine.prompts import (
    REVIEWER_DIFF_MAX_BYTES,
    REVIEWER_SYSTEM_PROMPT,
    build_reviewer_prompt,
)
from src.openharness.tools.base import (
    SimpleTool,
)

if TYPE_CHECKING:
    from src.models.router import ModelRouter


# Re-export ModelRouterError so tests and downstream callers can
# catch it without reaching into src.models. The tool catches this
# class explicitly in _execute_simple.
try:
    from src.models.router import ModelRouterError
except ImportError:
    class ModelRouterError(Exception):  # type: ignore[no-redef]
        """Raised by ModelRouter.generate when no provider is available
        or all providers fail. Re-exported here so RequestReviewTool's
        callers don't need to import from src.models directly."""


GIT_DIFF_TIMEOUT_SECONDS = 30
DIFF_TRUNCATION_MARKER = "\n\n<diff truncated at 32KiB by RequestReviewTool>\n"


class RequestReviewInput(BaseModel):
    """Input for request_review.

    summary:
        The agent's own description of what it built and why it
        believes the work is complete. Required, non-empty.
    """

    summary: str = Field(..., min_length=1)


class RequestReviewTool(SimpleTool):
    """Independent reviewer LLM invocation. The agent calls this at
    major milestones to get a second opinion on its diff before
    finalizing the turn.

    The tool collects `git diff HEAD` from the workspace via direct
    subprocess (NOT BashTool/bwrap — see §2.9.3.e for the exemption),
    builds the reviewer prompt, calls ModelRouter.generate with
    Purpose.REVIEW, and returns the raw reviewer response. The
    verdict is NOT parsed by the tool; the agent reads the response
    and decides whether to APPROVE/REVISE/REJECT-style follow-up.

    Spec: §2.9.3.
    """

    name = "request_review"
    description = (
        "Request an independent reviewer LLM to critique your current "
        "work before finalizing. The reviewer sees your git diff and "
        "the user's original request. Returns the reviewer's verdict "
        "as plain text — read the response and act on the verdict "
        "(APPROVE proceed, REVISE address listed items, REJECT "
        "reconsider approach)."
    )
    input_model = RequestReviewInput

    def __init__(
        self,
        model_router: "ModelRouter",
        workspace_dir: Path,
    ) -> None:
        self._router = model_router
        self._workspace_dir = workspace_dir

    async def _execute_simple(
        self,
        arguments: RequestReviewInput,
        context: ToolExecutionContext,
    ) -> ToolResult:
        # Collect git diff first — if this fails, we don't waste an
        # LLM call.
        try:
            diff = await self._collect_git_diff()
        except asyncio.TimeoutError:
            return ToolResult(
                output=(
                    f"git diff HEAD timed out after "
                    f"{GIT_DIFF_TIMEOUT_SECONDS}s — workspace may be "
                    f"in an unhealthy state"
                ),
                is_error=True,
            )
        except Exception as exc:
            logger.exception("RequestReviewTool: git diff collection failed")
            return ToolResult(
                output=f"git diff failed: {exc}",
                is_error=True,
            )

        prompt = build_reviewer_prompt(
            summary=arguments.summary,
            current_diff=diff,
            original_request=context.original_user_request or "(no original request available)",
        )

        # Lazy import to avoid hard dependency at module-import time
        from src.models.router import Purpose

        try:
            response = await self._router.chat(
                system=REVIEWER_SYSTEM_PROMPT,
                messages=[{"role": "user", "content": prompt}],
                purpose=Purpose.REVIEW,
                max_tokens=1024,
            )
            response_text = getattr(response, "content", str(response)) or str(response)
        except ModelRouterError as exc:
            return ToolResult(
                output=f"reviewer unavailable: {exc}",
                is_error=True,
            )

        return ToolResult(output=response_text, is_error=False)

    async def _collect_git_diff(self) -> str:
        """Run `git diff HEAD` directly via asyncio.create_subprocess_exec.

        §2.9.3.e bwrap exemption: this is one of exactly two
        subprocess calls in Round 2 that bypass bwrap. The other is
        the workspace manager's git operations (§3.5). Both are
        hardcoded, parameter-less, read-only git invocations. The
        argv is literally ['git', 'diff', 'HEAD'] — no shell, no
        user input, no format strings.

        Output is capped at REVIEWER_DIFF_MAX_BYTES (32 KiB). On
        overflow, the diff is truncated and DIFF_TRUNCATION_MARKER
        is appended so the reviewer knows it didn't see everything.

        Timeout: GIT_DIFF_TIMEOUT_SECONDS (30 s). On timeout, the
        process group is killed and asyncio.TimeoutError propagates
        to the caller.
        """
        proc = await asyncio.create_subprocess_exec(
            "git",
            "diff",
            "HEAD",
            cwd=str(self._workspace_dir),
            stdout=asyncio.subprocess.PIPE,
            stderr=asyncio.subprocess.PIPE,
        )
        try:
            stdout, stderr = await asyncio.wait_for(
                proc.communicate(),
                timeout=GIT_DIFF_TIMEOUT_SECONDS,
            )
        except asyncio.TimeoutError:
            try:
                proc.kill()
            except ProcessLookupError:
                pass
            raise

        if proc.returncode != 0:
            err = stderr.decode("utf-8", errors="replace")
            raise RuntimeError(
                f"git diff HEAD exited {proc.returncode}: {err.strip()}"
            )

        # Decode + truncate
        text = stdout.decode("utf-8", errors="replace")
        if len(text.encode("utf-8")) > REVIEWER_DIFF_MAX_BYTES:
            truncated = text.encode("utf-8")[:REVIEWER_DIFF_MAX_BYTES]
            text = (
                truncated.decode("utf-8", errors="replace")
                + DIFF_TRUNCATION_MARKER
            )
        return text

    def is_read_only(self, arguments: BaseModel) -> bool:
        return True


# ---------------------------------------------------------------------------
# register_interaction_tools — wiring helper for _create_engine
# Spec: §2.9.3.b. Called by api_server._create_engine (Task 5.15).
# ---------------------------------------------------------------------------


def register_interaction_tools(
    registry: "ToolRegistry",
    model_router: "ModelRouter",
    workspace_dir: Path,
) -> None:
    """Register both interaction meta-tools on the supplied registry.

    The two tools are RequestClarificationTool (Phase 5a) and
    RequestReviewTool (Task 5.13). Both are constructed here so
    _create_engine doesn't need to know their constructor signatures.

    A second call on the same registry raises — ToolRegistry.register
    already enforces unique tool names. Silicon Valley standard
    §2.8: duplicate registration is a bug in the call site, surface
    it loudly rather than swallowing the second call.
    """
    from src.openharness.tools.base import ToolRegistry as _ToolRegistry  # type hint

    registry.register(RequestClarificationTool())
    registry.register(
        RequestReviewTool(
            model_router=model_router,
            workspace_dir=workspace_dir,
        )
    )
