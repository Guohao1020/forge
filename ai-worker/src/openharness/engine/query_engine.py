"""QueryEngine — stateful wrapper around the agent loop.

Owns: messages, cost tracker, api client, tool registry, hook executor, permissions.
Provides: submit_message() -> AsyncIterator[StreamEvent], clear(), set_system_prompt().
"""

from __future__ import annotations

from pathlib import Path
from typing import Any, AsyncIterator, Dict, List, Optional

from ..api.client import SupportsStreamingMessages
from ..api.usage import UsageSnapshot
from ..hooks.executor import HookExecutor
from ..permissions.checker import PermissionChecker
from ..tools.base import ToolRegistry
from .messages import ConversationMessage, TextBlock
from .query import QueryContext, run_agent_loop
from .stream_events import AssistantTurnComplete, StreamEvent


class QueryEngine:
    """Stateful engine that manages conversation history and runs agent loops."""

    def __init__(
        self,
        api_client: SupportsStreamingMessages,
        tool_registry: ToolRegistry,
        model: str,
        system_prompt: str,
        max_tokens: int = 4096,
        max_turns: int = 25,
        hook_executor: Optional[HookExecutor] = None,
        permission_checker: Optional[PermissionChecker] = None,
        cwd: Optional[Path] = None,
    ) -> None:
        self._api_client = api_client
        self._tool_registry = tool_registry
        self._model = model
        self._system_prompt = system_prompt
        self._max_tokens = max_tokens
        self._max_turns = max_turns
        self._hook_executor = hook_executor
        self._permission_checker = permission_checker
        self._cwd = cwd or Path.cwd()

        self._messages: List[ConversationMessage] = []
        self._total_usage = UsageSnapshot()

    @property
    def messages(self) -> List[ConversationMessage]:
        return self._messages

    @property
    def total_usage(self) -> UsageSnapshot:
        return self._total_usage

    def clear(self) -> None:
        self._messages.clear()
        self._total_usage = UsageSnapshot()

    def set_system_prompt(self, prompt: str) -> None:
        self._system_prompt = prompt

    def set_model(self, model: str) -> None:
        self._model = model

    async def submit_message(self, prompt: str) -> AsyncIterator[StreamEvent]:
        """Submit a user message and yield stream events from the agent loop."""
        # Add user message to history
        user_msg = ConversationMessage.from_user_text(prompt)
        self._messages.append(user_msg)

        # Build context
        context = QueryContext(
            api_client=self._api_client,
            tool_registry=self._tool_registry,
            model=self._model,
            system_prompt=self._system_prompt,
            max_tokens=self._max_tokens,
            max_turns=self._max_turns,
            hook_executor=self._hook_executor,
            permission_checker=self._permission_checker,
            cwd=self._cwd,
        )

        # Run agent loop and accumulate usage
        async for event in run_agent_loop(context, self._messages):
            if isinstance(event, AssistantTurnComplete):
                self._total_usage = UsageSnapshot(
                    input_tokens=self._total_usage.input_tokens + event.usage.input_tokens,
                    output_tokens=self._total_usage.output_tokens + event.usage.output_tokens,
                )
            yield event
