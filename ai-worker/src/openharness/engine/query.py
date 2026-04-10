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
from collections.abc import AsyncIterator
from typing import Any, Dict, List, Optional

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
from .agent_hooks import SessionHaltError
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
    clarification_coordinator: Any = None
    original_user_request: Optional[str] = None
    # Round 2 additions (Phase 5 Task 5.9)
    agent_hook_registry: Any = None  # AgentHookRegistry
    agent_hook_context: Any = None  # AgentHookContext


async def _execute_tool_call(
    context: QueryContext,
    tool_name: str,
    tool_use_id: str,
    tool_input: Dict[str, Any],
    pre_parsed_arguments: Any = None,
) -> AsyncIterator[Any]:
    """Execute a single tool call with hooks and permission checks.

    Consumes the tool's async-generator execute() and yields:
      - zero or more StreamEvents (forwarded as-is to the caller)
      - exactly one ToolResultBlock as the final item

    Hook failures and permission denials short-circuit with a single
    ToolResultBlock(is_error=True) and no StreamEvents.

    pre_parsed_arguments: if provided (from pre_tool_call hook
    mutation), skip re-validation and use these directly.
    """

    # 1. Pre-tool hook (subprocess hooks — distinct from agent hooks)
    if context.hook_executor:
        payload = {"tool_name": tool_name, "tool_input": tool_input}
        hook_result = await context.hook_executor.execute(HookEvent.PRE_TOOL_USE, payload)
        if hook_result.blocked:
            reasons = hook_result.all_reasons
            yield ToolResultBlock(
                tool_use_id=tool_use_id,
                content=f"BLOCKED by hook: {'; '.join(reasons)}",
                is_error=True,
            )
            return

    # 2. Permission check
    if context.permission_checker:
        tool_obj = context.tool_registry.get(tool_name)
        is_ro = tool_obj.is_read_only(tool_input) if tool_obj else False
        decision = context.permission_checker.evaluate(tool_name, is_read_only=is_ro)
        if not decision.allowed:
            yield ToolResultBlock(
                tool_use_id=tool_use_id,
                content=f"Permission denied: {decision.reason}",
                is_error=True,
            )
            return

    # 3. Tool lookup
    tool = context.tool_registry.get(tool_name)
    if not tool:
        yield ToolResultBlock(
            tool_use_id=tool_use_id,
            content=f"Unknown tool: {tool_name}",
            is_error=True,
        )
        return

    # 4. Input validation (skip if pre_parsed from agent hook mutation)
    if pre_parsed_arguments is not None:
        parsed = pre_parsed_arguments
    else:
        try:
            parsed = tool.input_model.model_validate(tool_input)
        except Exception as e:
            yield ToolResultBlock(
                tool_use_id=tool_use_id,
                content=f"Invalid input: {e}",
                is_error=True,
            )
            return

    # 5. Tool execution — consume the async generator
    exec_ctx = ToolExecutionContext(
        cwd=context.cwd,
        tool_use_id=tool_use_id,
        clarification_coordinator=getattr(context, "clarification_coordinator", None),
        original_user_request=getattr(context, "original_user_request", None),
    )
    tool_result: ToolResult | None = None
    try:
        async for item in tool.execute(parsed, exec_ctx):
            if isinstance(item, ToolResult):
                if tool_result is not None:
                    raise RuntimeError(
                        f"tool {tool_name} yielded multiple ToolResults"
                    )
                tool_result = item
            elif isinstance(item, StreamEvent):
                # Forward mid-execution events up to run_agent_loop
                yield item
            else:
                raise TypeError(
                    f"tool {tool_name} yielded unexpected type: {type(item).__name__}"
                )
    except SessionHaltError as halt:
        logger.error(
            "Session halted by %s during tool %s: %s",
            type(halt).__name__,
            tool_name,
            halt,
        )
        yield ToolResultBlock(
            tool_use_id=tool_use_id,
            content=f"session halted: {type(halt).__name__}: {halt}",
            is_error=True,
        )
        return
    except Exception as e:
        logger.exception("Tool execution failed: %s", tool_name)
        yield ToolResultBlock(
            tool_use_id=tool_use_id,
            content=f"Tool execution error: {e}",
            is_error=True,
        )
        return

    if tool_result is None:
        yield ToolResultBlock(
            tool_use_id=tool_use_id,
            content=f"Tool {tool_name} did not yield a ToolResult",
            is_error=True,
        )
        return

    # 6. Post-tool hook
    if context.hook_executor:
        payload = {
            "tool_name": tool_name,
            "tool_input": tool_input,
            "tool_output": tool_result.output,
            "is_error": tool_result.is_error,
        }
        await context.hook_executor.execute(HookEvent.POST_TOOL_USE, payload)

    # Structured observability -- emitted after every tool call, success
    # or error. Post-deploy debugging can query Loki for specific
    # tool_names / tool_use_ids.
    logger.info(
        "agent.tool_call",
        extra={
            "event": "agent.tool_call",
            "tool_name": tool_name,
            "tool_use_id": tool_use_id,
            "is_error": tool_result.is_error,
            "input_size_bytes": len(str(tool_input).encode("utf-8")),
            "output_size_bytes": len(tool_result.output.encode("utf-8")),
        },
    )

    yield ToolResultBlock(
        tool_use_id=tool_use_id,
        content=tool_result.output,
        is_error=tool_result.is_error,
    )


async def run_agent_loop(
    context: QueryContext,
    messages: List[ConversationMessage],
) -> AsyncIterator[StreamEvent]:
    """Run the agent loop: stream API, execute tools, repeat until done.

    Yields StreamEvents as they occur. Modifies messages in-place.
    """
    from .agent_hooks import AgentHookRegistry, PreToolCallBlock

    registry = context.agent_hook_registry or AgentHookRegistry()
    hook_ctx = context.agent_hook_context  # may be None for legacy callers

    turn = 0

    while turn < context.max_turns:
        turn += 1

        # Round 2 (§2.9.1.c): pre_turn hooks run before the API call.
        for hook in registry.pre_turn:
            try:
                messages = await hook(hook_ctx, messages)
            except Exception as exc:
                logger.exception("agent hook %s in pre_turn raised", hook)
                yield ErrorEvent(
                    message=(
                        f"agent hook {getattr(hook, '__name__', repr(hook))} "
                        f"raised in pre_turn: {exc}"
                    ),
                    recoverable=False,
                )
                return

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
            logger.info(
                "agent.turn_complete",
                extra={
                    "event": "agent.turn_complete",
                    "turn_count": turn,
                    "stop_reason": stop_reason,
                },
            )
            # Round 2 (§2.9.1.c): post_turn hooks fire on end_turn.
            for hook in registry.post_turn:
                try:
                    await hook(hook_ctx, assistant_message)
                except Exception as exc:
                    logger.exception("agent hook %s in post_turn raised", hook)
                    yield ErrorEvent(
                        message=(
                            f"agent hook {getattr(hook, '__name__', repr(hook))} "
                            f"raised in post_turn: {exc}"
                        ),
                        recoverable=False,
                    )
                    return
            return

        # Execute tool calls
        tool_results: List[ToolResultBlock] = []
        for tu in tool_uses:
            # Round 2: pre_tool_call hooks run before tool execution.
            tool = context.tool_registry.get(tu.name)
            if tool is not None:
                try:
                    parsed = tool.input_model.model_validate(tu.input)
                except Exception:
                    parsed = None
            else:
                parsed = None

            blocked = False
            if parsed is not None:
                for hook in registry.pre_tool_call:
                    try:
                        result = await hook(hook_ctx, tu.name, parsed)
                    except Exception as exc:
                        logger.exception("agent hook %s in pre_tool_call raised", hook)
                        yield ErrorEvent(
                            message=(
                                f"agent hook {getattr(hook, '__name__', repr(hook))} "
                                f"raised in pre_tool_call: {exc}"
                            ),
                            recoverable=False,
                        )
                        return

                    if isinstance(result, PreToolCallBlock):
                        # Short-circuit: emit Started + Completed with error
                        yield ToolExecutionStarted(
                            tool_use_id=tu.id, tool_name=tu.name, tool_input=tu.input
                        )
                        tool_results.append(ToolResultBlock(
                            tool_use_id=tu.id,
                            content=result.reason,
                            is_error=True,
                        ))
                        yield ToolExecutionCompleted(
                            tool_use_id=tu.id,
                            tool_name=tu.name,
                            output=result.reason,
                            is_error=True,
                        )
                        blocked = True
                        break
                    parsed = result  # mutated args feed the next hook

            if blocked:
                continue

            yield ToolExecutionStarted(tool_use_id=tu.id, tool_name=tu.name, tool_input=tu.input)

            # _execute_tool_call is an async generator that yields
            # zero or more StreamEvents followed by exactly one
            # ToolResultBlock. Forward StreamEvents upstream; capture
            # the ToolResultBlock as the tool's final result.
            final_block: ToolResultBlock | None = None
            async for item in _execute_tool_call(
                context=context,
                tool_name=tu.name,
                tool_use_id=tu.id,
                tool_input=tu.input,
                pre_parsed_arguments=parsed,
            ):
                if isinstance(item, ToolResultBlock):
                    final_block = item
                else:
                    # Mid-execution StreamEvent (e.g., ThinkingStarted
                    # from BashTool in Phase 4). Pass through.
                    yield item

            assert final_block is not None, (
                f"_execute_tool_call yielded no ToolResultBlock for {tu.name}"
            )
            tool_results.append(final_block)
            yield ToolExecutionCompleted(
                tool_use_id=tu.id,
                tool_name=tu.name,
                output=final_block.content,
                is_error=final_block.is_error,
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
