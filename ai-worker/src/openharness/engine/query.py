"""Core agent loop — stream API call, detect tool_use, execute tools, continue.

Flow:
  User message → API call → stream response
    → if stop_reason == "end_turn": done
    → if stop_reason == "tool_use":
        for each tool call:
          1. Pre-tool hook (if blocked → error ToolResultBlock)
          2. Permission check (if denied → error ToolResultBlock)
          3. Tool lookup (not found → error)
          4. Input validation (fail → error)
          5. Tool execution (exception → error)
          6. Post-tool hook
        Append tool results → API call → loop
    → if max_turns exceeded: raise MaxTurnsExceeded
"""

from __future__ import annotations

import asyncio
import logging
from dataclasses import dataclass, field
from pathlib import Path
from typing import Any, AsyncIterator, Dict, List, Optional

from ..api.client import (
    ApiMessageCompleteEvent,
    ApiMessageRequest,
    ApiStreamEvent,
    ApiTextDeltaEvent,
    SupportsStreamingMessages,
)
from ..api.usage import UsageSnapshot
from ..hooks.events import HookEvent
from ..hooks.executor import HookExecutor
from ..permissions.checker import PermissionChecker
from ..tools.base import BaseTool, ToolExecutionContext, ToolRegistry, ToolResult
from .messages import ConversationMessage, TextBlock, ToolResultBlock, ToolUseBlock
from .stream_events import (
    AssistantTextDelta,
    AssistantTurnComplete,
    ErrorEvent,
    StreamEvent,
    ToolExecutionCompleted,
    ToolExecutionStarted,
)

logger = logging.getLogger(__name__)


class MaxTurnsExceeded(Exception):
    pass


@dataclass
class QueryContext:
    """Runtime context for a single agent loop invocation."""

    api_client: SupportsStreamingMessages
    tool_registry: ToolRegistry
    model: str
    system_prompt: str
    max_tokens: int = 4096
    max_turns: int = 25
    hook_executor: Optional[HookExecutor] = None
    permission_checker: Optional[PermissionChecker] = None
    cwd: Path = field(default_factory=Path.cwd)


async def _execute_tool_call(
    context: QueryContext,
    tool_name: str,
    tool_use_id: str,
    tool_input: Dict[str, Any],
) -> ToolResultBlock:
    """Execute a single tool call with hooks and permission checks."""

    # 1. Pre-tool hook
    if context.hook_executor:
        payload = {"tool_name": tool_name, "tool_input": tool_input}
        hook_result = await context.hook_executor.execute(HookEvent.PRE_TOOL_USE, payload)
        if hook_result.blocked:
            reasons = hook_result.all_reasons
            return ToolResultBlock(
                tool_use_id=tool_use_id,
                content=f"BLOCKED by hook: {'; '.join(reasons)}",
                is_error=True,
            )

    # 2. Permission check
    if context.permission_checker:
        tool_obj = context.tool_registry.get(tool_name)
        is_ro = tool_obj.is_read_only(tool_input) if tool_obj else False
        decision = context.permission_checker.evaluate(tool_name, is_read_only=is_ro)
        if not decision.allowed:
            return ToolResultBlock(
                tool_use_id=tool_use_id,
                content=f"Permission denied: {decision.reason}",
                is_error=True,
            )

    # 3. Tool lookup
    tool = context.tool_registry.get(tool_name)
    if not tool:
        return ToolResultBlock(
            tool_use_id=tool_use_id,
            content=f"Unknown tool: {tool_name}",
            is_error=True,
        )

    # 4. Input validation
    try:
        parsed = tool.input_model.model_validate(tool_input)
    except Exception as e:
        return ToolResultBlock(
            tool_use_id=tool_use_id,
            content=f"Invalid input: {e}",
            is_error=True,
        )

    # 5. Tool execution
    try:
        exec_ctx = ToolExecutionContext(cwd=context.cwd)
        result = await tool.execute(parsed, exec_ctx)
    except Exception as e:
        logger.exception("Tool execution failed: %s", tool_name)
        return ToolResultBlock(
            tool_use_id=tool_use_id,
            content=f"Tool execution error: {e}",
            is_error=True,
        )

    # 6. Post-tool hook
    if context.hook_executor:
        payload = {
            "tool_name": tool_name,
            "tool_input": tool_input,
            "tool_output": result.output,
            "is_error": result.is_error,
        }
        await context.hook_executor.execute(HookEvent.POST_TOOL_USE, payload)

    return ToolResultBlock(
        tool_use_id=tool_use_id,
        content=result.output,
        is_error=result.is_error,
    )


async def run_agent_loop(
    context: QueryContext,
    messages: List[ConversationMessage],
) -> AsyncIterator[StreamEvent]:
    """Run the agent loop: stream API, execute tools, repeat until done.

    Yields StreamEvents as they occur. Modifies messages in-place.
    """
    turn = 0

    while turn < context.max_turns:
        turn += 1

        # Build API request
        request = ApiMessageRequest(
            model=context.model,
            messages=messages,
            system_prompt=context.system_prompt,
            max_tokens=context.max_tokens,
            tools=context.tool_registry.to_api_schema() or None,
        )

        # Stream API response
        assistant_message: Optional[ConversationMessage] = None
        usage: Optional[UsageSnapshot] = None
        stop_reason: Optional[str] = None

        async for event in context.api_client.stream_message(request):
            if isinstance(event, ApiTextDeltaEvent):
                yield AssistantTextDelta(text=event.text)
            elif isinstance(event, ApiMessageCompleteEvent):
                assistant_message = event.message
                usage = event.usage
                stop_reason = event.stop_reason

        if assistant_message is None:
            yield ErrorEvent(message="No response from API", recoverable=False)
            return

        # Append assistant message to history
        messages.append(assistant_message)

        yield AssistantTurnComplete(
            message=assistant_message,
            usage=usage or UsageSnapshot(),
        )

        # Check if we need to execute tools
        tool_uses = assistant_message.tool_uses
        if not tool_uses or stop_reason == "end_turn":
            return

        # Execute tool calls
        tool_results: List[ToolResultBlock] = []
        for tu in tool_uses:
            yield ToolExecutionStarted(tool_name=tu.name, tool_input=tu.input)
            result = await _execute_tool_call(
                context=context,
                tool_name=tu.name,
                tool_use_id=tu.id,
                tool_input=tu.input,
            )
            tool_results.append(result)
            yield ToolExecutionCompleted(
                tool_name=tu.name,
                output=result.content,
                is_error=result.is_error,
            )

        # Append tool results as user message
        messages.append(ConversationMessage(
            role="user",
            content=tool_results,  # type: ignore[arg-type]
        ))

    # Max turns exceeded
    yield ErrorEvent(
        message=f"Agent reached maximum iterations ({context.max_turns})",
        recoverable=False,
    )
