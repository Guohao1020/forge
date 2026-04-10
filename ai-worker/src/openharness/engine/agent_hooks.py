"""Agent hooks — exception hierarchy, clarification coordinator, and
(later in Phase 5 Task 5.8) hook registry + protocols.

This module is the first thing appended to by later phases. Import
order:
  - Task 5a.1: SessionHaltError, ClarificationTimeout, ReturnChannelError
  - Task 5a.2: ClarificationCoordinator
  - Task 5.8:  AgentHookRegistry, PreTurnHook, PreToolCallHook,
               PostTurnHook, PromptSlotFiller, PreToolCallBlock,
               AgentHookContext
"""

from __future__ import annotations

import asyncio
import logging

logger = logging.getLogger(__name__)


# ---------------------------------------------------------------------------
# Exception hierarchy (spec §2.9.2.d)
# ---------------------------------------------------------------------------


class SessionHaltError(Exception):
    """Base class for errors that halt the session rather than being
    translated to ToolResult(is_error=True). See §4.1 BaseTool contract
    and §2.9.2.f timeout policy."""


class ClarificationTimeout(SessionHaltError):
    """Raised when the user does not respond to a clarification request
    within the configured timeout window."""

    def __init__(self, tool_use_id: str, timeout_seconds: float) -> None:
        super().__init__(
            f"clarification timeout after {timeout_seconds}s "
            f"(tool_use_id={tool_use_id})"
        )
        self.tool_use_id = tool_use_id
        self.timeout_seconds = timeout_seconds


class ReturnChannelError(SessionHaltError):
    """Raised when the Redis return channel is lost mid-wait."""


# ---------------------------------------------------------------------------
# Clarification coordinator (spec §2.9.2.d)
# ---------------------------------------------------------------------------


class ClarificationCoordinator:
    """Future-based pause/resume state machine for request_clarification.

    One instance per session, owned by QueryEngine. The coordinator
    bridges RequestClarificationTool (which calls wait_for) and
    ReturnChannel (which calls deliver).

    _pending maps tool_use_id -> asyncio.Future[str]. A tool registers
    a future via wait_for(); the ReturnChannel listener resolves it
    via deliver(). Sequential tool execution per §2.9.2.h means at
    most one future is pending at a time, but the dict supports
    multiple for correctness.
    """

    def __init__(self) -> None:
        self._pending: dict[str, asyncio.Future[str]] = {}

    async def wait_for(self, tool_use_id: str, timeout: float) -> str:
        """Register a future for tool_use_id and await it with timeout.

        Returns the user's response string on success. Raises
        asyncio.TimeoutError on timeout. Always cleans up _pending
        on exit (success, timeout, or cancellation).
        """
        fut: asyncio.Future[str] = asyncio.get_running_loop().create_future()
        self._pending[tool_use_id] = fut
        try:
            return await asyncio.wait_for(fut, timeout=timeout)
        finally:
            self._pending.pop(tool_use_id, None)

    def deliver(self, tool_use_id: str, response: str) -> None:
        """Resolve a pending future with the user's response.

        Logs a warning and returns silently if:
        - tool_use_id is not in _pending (stale response or replay)
        - the future is already done (duplicate delivery)
        """
        fut = self._pending.get(tool_use_id)
        if fut is None:
            logger.warning(
                "clarification response for unknown tool_use_id: %s",
                tool_use_id,
            )
            return
        if fut.done():
            logger.warning(
                "clarification response arrived after completion: %s",
                tool_use_id,
            )
            return
        fut.set_result(response)

    def cancel_all(self) -> None:
        """Cancel all pending futures and clear the map.

        Called by QueryEngine.close() and by ReturnChannel on
        connection failure.
        """
        for fut in list(self._pending.values()):
            if not fut.done():
                fut.cancel()
        self._pending.clear()
