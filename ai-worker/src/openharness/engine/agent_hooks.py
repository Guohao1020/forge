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


# ---------------------------------------------------------------------------
# Round 2: in-process agent hook system (spec §2.9.1)
#
# Distinct from openharness.hooks.HookRegistry, which runs SUBPROCESS
# shell-command hooks on PRE_TOOL_USE / POST_TOOL_USE / etc. events.
# That system is unchanged. AgentHookRegistry is a parallel,
# in-process Python-callable hook system. The two do not replace
# each other and must not share a class name.
#
# Round 2 ships extension points only — empty default registries.
# Downstream projects (the verification project, the entropy project,
# etc.) plug in real implementations via a project-scoped factory.
# ---------------------------------------------------------------------------

from dataclasses import dataclass, field
from pathlib import Path
from typing import Awaitable, Callable, Protocol, TYPE_CHECKING

if TYPE_CHECKING:
    # Avoid a hard import cycle: messages.py imports from
    # stream_events.py which is in the same package.
    from .messages import ConversationMessage
    from pydantic import BaseModel


# ---------------------------------------------------------------------------
# Pinned async Protocol signatures — §2.9.1.b
# ---------------------------------------------------------------------------


class PreTurnHook(Protocol):
    """Runs before each turn. May mutate the message list (returning
    the new list) and/or append to ctx.system_prompt_buffer (the
    rendered build_system_prompt result is mutable across turns
    via the registry's slot fillers; pre_turn hooks operate on the
    PER-TURN message list)."""

    async def __call__(
        self,
        ctx: "AgentHookContext",
        messages: "list[ConversationMessage]",
    ) -> "list[ConversationMessage]": ...


class PreToolCallHook(Protocol):
    """Runs inside _execute_tool_call between permission check and
    tool execution. Returns either the (possibly-mutated) parsed
    arguments to execute, or PreToolCallBlock(reason) to short-circuit
    with a ToolExecutionCompleted(is_error=True, output=reason). The
    tool itself is NOT executed when blocked. Raising an exception is
    a bug — use PreToolCallBlock for intentional blocks."""

    async def __call__(
        self,
        ctx: "AgentHookContext",
        tool_name: str,
        arguments: "BaseModel",
    ) -> "BaseModel | PreToolCallBlock": ...


class PostTurnHook(Protocol):
    """Runs after each turn completes with stop_reason=end_turn. May
    record metrics, trigger follow-up events, etc. Returns None."""

    async def __call__(
        self,
        ctx: "AgentHookContext",
        final_message: "ConversationMessage",
    ) -> None: ...


class PromptSlotFiller(Protocol):
    """Returns the string that gets substituted into a {{slot_name}}
    placeholder in build_system_prompt. Called once per submit_message
    when the system prompt is rendered."""

    async def __call__(self, ctx: "AgentHookContext") -> str: ...


@dataclass(frozen=True)
class PreToolCallBlock:
    """Returned by a pre_tool_call hook to short-circuit a tool call
    without executing the tool. The reason is surfaced in the
    resulting ToolExecutionCompleted event so the agent observes
    the block in the next turn's input."""

    reason: str


# ---------------------------------------------------------------------------
# AgentHookContext — per-session value passed into every hook call
# ---------------------------------------------------------------------------


@dataclass
class AgentHookContext:
    """Per-session context constructed once in _create_engine and
    passed into every agent hook call.

    Fields:
        project_id: The project this session belongs to. Hooks use
            this to look up project-specific data (specs, profiles,
            constraint configs, etc.).
        session_id: The session UUID. Hooks use this to correlate
            metrics or attach session-scoped state.
        workspace_dir: The absolute path to the session's workspace
            directory. Hooks that need to read project state (e.g.
            for context_engineering hooks that load project specs)
            use this as the root.
        system_prompt_buffer: A mutable list pre_turn hooks can
            append to. The agent loop does NOT currently re-render
            the system prompt mid-session — this buffer is reserved
            for future Round 3+ work where pre_turn hooks dynamically
            inject context per-turn. For Round 2 it's an unused but
            stable interface.
    """

    project_id: int
    session_id: str
    workspace_dir: Path
    system_prompt_buffer: list[str] = field(default_factory=list)


# ---------------------------------------------------------------------------
# AgentHookRegistry — four hook collections, all empty by default
# ---------------------------------------------------------------------------


class AgentHookRegistry:
    """In-process agent hook registry. Holds four collections, all
    empty by default. Each collection is a plain list (or dict for
    slot fillers) — registration is `registry.pre_turn.append(hook)`,
    not a method call. The collections are public attributes so
    downstream factories can populate them with one-liners.

    Order semantics (§2.9.1.c): list order = registration order =
    invocation order. No priority field. If hook ordering matters,
    register them in the order you want them invoked.

    Failure mode (§2.9.1.c): hooks raising an exception halt the
    turn with ErrorEvent(recoverable=False). The empty defaults
    Round 2 ships never raise.
    """

    def __init__(self) -> None:
        # Per-instance lists/dict — never share defaults across
        # instances. The classic Python mutable-default-argument
        # bug class is avoided by constructing fresh containers
        # in __init__.
        self.pre_turn: list[PreTurnHook] = []
        self.pre_tool_call: list[PreToolCallHook] = []
        self.post_turn: list[PostTurnHook] = []
        self.system_prompt_slots: dict[str, PromptSlotFiller] = {}
