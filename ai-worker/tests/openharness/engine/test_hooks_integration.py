"""End-to-end agent hook integration tests.

These exercise the full call chain (registry -> run_agent_loop ->
_execute_tool_call) against a mocked API client. The registry,
context, loop, and tool execution are all real — only the upstream
LLM is mocked.

Spec: §2.9.1.c, §5.1 Round 2, §7.4 Round 2.
"""

from __future__ import annotations

from pathlib import Path
from typing import AsyncIterator

import pytest

from src.openharness.api.client import (
    ApiMessageCompleteEvent,
    ApiMessageRequest,
    SupportsStreamingMessages,
)
from src.openharness.api.usage import UsageSnapshot
from src.openharness.engine.agent_hooks import (
    AgentHookContext,
    AgentHookRegistry,
    PreToolCallBlock,
)
from src.openharness.engine.messages import (
    ConversationMessage,
    TextBlock,
    ToolUseBlock,
)
from src.openharness.engine.query_engine import QueryEngine
from src.openharness.engine.stream_events import (
    ErrorEvent,
    SessionComplete,
    ToolExecutionCompleted,
    ToolExecutionStarted,
)
from src.openharness.tools.base import ToolRegistry
from src.openharness.tools.phase_tool import SetPhaseTool


# ---------------------------------------------------------------------------
# Test helpers
# ---------------------------------------------------------------------------


class _FakeApiClient:
    """Replays a canned sequence of ApiMessageCompleteEvents."""

    def __init__(self, events: list[ApiMessageCompleteEvent]) -> None:
        self._events = list(events)
        self.recorded_requests: list[ApiMessageRequest] = []

    async def stream_message(self, request: ApiMessageRequest):
        self.recorded_requests.append(request)
        if not self._events:
            raise RuntimeError("FakeApiClient: no more queued events")
        yield self._events.pop(0)


def _end_turn_event(text: str = "done") -> ApiMessageCompleteEvent:
    return ApiMessageCompleteEvent(
        message=ConversationMessage(
            role="assistant",
            content=[TextBlock(text=text)],
        ),
        usage=UsageSnapshot(input_tokens=5, output_tokens=2),
        stop_reason="end_turn",
    )


def _tool_use_event(
    tool_name: str,
    tool_input: dict,
    tool_use_id: str = "toolu_1",
) -> ApiMessageCompleteEvent:
    return ApiMessageCompleteEvent(
        message=ConversationMessage(
            role="assistant",
            content=[
                TextBlock(text="invoking tool"),
                ToolUseBlock(
                    id=tool_use_id,
                    name=tool_name,
                    input=tool_input,
                ),
            ],
        ),
        usage=UsageSnapshot(input_tokens=10, output_tokens=5),
        stop_reason="tool_use",
    )


def _make_engine(
    api_client: _FakeApiClient,
    *,
    registry: AgentHookRegistry | None = None,
    tools: ToolRegistry | None = None,
    cwd: Path | None = None,
) -> QueryEngine:
    return QueryEngine(
        api_client=api_client,
        tool_registry=tools or ToolRegistry(),
        model="test-model",
        system_prompt="base system prompt",
        agent_hook_registry=registry,
        agent_hook_context=(
            AgentHookContext(
                project_id=1,
                session_id="sess-test",
                workspace_dir=cwd or Path("/tmp/ws"),
                system_prompt_buffer=[],
            )
            if registry is not None
            else None
        ),
        cwd=cwd or Path("/tmp/ws"),
    )


# ---------------------------------------------------------------------------
# pre_turn hook scenarios
# ---------------------------------------------------------------------------


@pytest.mark.asyncio
async def test_pre_turn_hook_mutates_system_prompt(tmp_path):
    """A pre_turn hook can append to ctx.system_prompt_buffer."""
    registry = AgentHookRegistry()

    async def appender(ctx, messages):
        ctx.system_prompt_buffer.append("<extra context>")
        return messages

    registry.pre_turn.append(appender)

    api_client = _FakeApiClient([_end_turn_event()])
    engine = _make_engine(api_client, registry=registry, cwd=tmp_path)

    events = [e async for e in engine.submit_message("hi")]
    assert engine._agent_hook_context.system_prompt_buffer == [
        "<extra context>"
    ]


@pytest.mark.asyncio
async def test_pre_turn_hook_mutates_message_list(tmp_path):
    """A pre_turn hook can return a mutated message list."""
    registry = AgentHookRegistry()

    async def inject(ctx, messages):
        return messages + [
            ConversationMessage.from_user_text("[injected]")
        ]

    registry.pre_turn.append(inject)

    api_client = _FakeApiClient([_end_turn_event()])
    engine = _make_engine(api_client, registry=registry, cwd=tmp_path)

    [e async for e in engine.submit_message("original")]

    assert len(api_client.recorded_requests) == 1
    request = api_client.recorded_requests[0]
    # Check that the injected message is in the messages sent to the API
    texts = []
    for m in request.messages:
        if hasattr(m, "text"):
            texts.append(m.text)
        elif hasattr(m, "content"):
            for block in (m.content if isinstance(m.content, list) else [m.content]):
                if hasattr(block, "text"):
                    texts.append(block.text)
    assert any("[injected]" in t for t in texts)


# ---------------------------------------------------------------------------
# pre_tool_call hook scenarios
# ---------------------------------------------------------------------------


@pytest.mark.asyncio
async def test_pre_tool_call_hook_blocks_tool(tmp_path):
    """A pre_tool_call hook returning PreToolCallBlock short-circuits
    tool execution."""
    registry = AgentHookRegistry()

    async def block_set_phase(ctx, tool_name, arguments):
        if tool_name == "set_phase":
            return PreToolCallBlock(reason="set_phase blocked by test")
        return arguments

    registry.pre_tool_call.append(block_set_phase)

    tools = ToolRegistry()
    tools.register(SetPhaseTool())

    api_client = _FakeApiClient([
        _tool_use_event("set_phase", {"phase": "Generate"}),
        _end_turn_event(),
    ])
    engine = _make_engine(
        api_client, registry=registry, tools=tools, cwd=tmp_path
    )

    events = [e async for e in engine.submit_message("go")]

    completed = [
        e for e in events if isinstance(e, ToolExecutionCompleted)
    ]
    assert len(completed) == 1
    assert completed[0].is_error is True
    assert completed[0].output == "set_phase blocked by test"


@pytest.mark.asyncio
async def test_pre_tool_call_hook_mutates_arguments(tmp_path):
    """A pre_tool_call hook returning a mutated arguments object
    causes the tool to run with the mutated values."""
    registry = AgentHookRegistry()

    async def force_review(ctx, tool_name, arguments):
        if tool_name == "set_phase":
            return arguments.model_copy(update={"phase": "Review"})
        return arguments

    registry.pre_tool_call.append(force_review)

    tools = ToolRegistry()
    tools.register(SetPhaseTool())

    api_client = _FakeApiClient([
        _tool_use_event("set_phase", {"phase": "Generate"}),
        _end_turn_event(),
    ])
    engine = _make_engine(
        api_client, registry=registry, tools=tools, cwd=tmp_path
    )

    events = [e async for e in engine.submit_message("go")]
    completed = [
        e for e in events if isinstance(e, ToolExecutionCompleted)
    ]
    assert len(completed) == 1
    assert completed[0].is_error is False
    assert "Review" in completed[0].output


# ---------------------------------------------------------------------------
# post_turn hook scenarios
# ---------------------------------------------------------------------------


@pytest.mark.asyncio
async def test_post_turn_hook_fires_on_end_turn(tmp_path):
    """A post_turn hook fires once at the end of a normally
    terminated turn (stop_reason=end_turn)."""
    registry = AgentHookRegistry()
    counter = {"value": 0}

    async def increment(ctx, final_message):
        counter["value"] += 1

    registry.post_turn.append(increment)

    api_client = _FakeApiClient([_end_turn_event()])
    engine = _make_engine(api_client, registry=registry, cwd=tmp_path)

    [e async for e in engine.submit_message("hi")]
    assert counter["value"] == 1


# ---------------------------------------------------------------------------
# Hook exception -> loop halt
# ---------------------------------------------------------------------------


@pytest.mark.asyncio
async def test_pre_turn_hook_exception_halts_loop(tmp_path):
    registry = AgentHookRegistry()

    async def boom(ctx, messages):
        raise RuntimeError("pre_turn boom")

    registry.pre_turn.append(boom)

    api_client = _FakeApiClient([_end_turn_event()])
    engine = _make_engine(api_client, registry=registry, cwd=tmp_path)

    events = [e async for e in engine.submit_message("hi")]
    errors = [e for e in events if isinstance(e, ErrorEvent)]
    assert len(errors) == 1
    assert errors[0].recoverable is False
    assert "pre_turn" in errors[0].message
    assert "boom" in errors[0].message
    # The API client must NOT have been called
    assert api_client.recorded_requests == []


@pytest.mark.asyncio
async def test_pre_tool_call_hook_exception_halts_loop(tmp_path):
    registry = AgentHookRegistry()

    async def boom(ctx, tool_name, arguments):
        raise RuntimeError("pre_tool_call boom")

    registry.pre_tool_call.append(boom)

    tools = ToolRegistry()
    tools.register(SetPhaseTool())

    api_client = _FakeApiClient([
        _tool_use_event("set_phase", {"phase": "Generate"}),
        _end_turn_event(),
    ])
    engine = _make_engine(
        api_client, registry=registry, tools=tools, cwd=tmp_path
    )

    events = [e async for e in engine.submit_message("go")]
    errors = [e for e in events if isinstance(e, ErrorEvent)]
    assert len(errors) == 1
    assert errors[0].recoverable is False
    assert "pre_tool_call" in errors[0].message
    assert "boom" in errors[0].message


@pytest.mark.asyncio
async def test_post_turn_hook_exception_halts_loop(tmp_path):
    registry = AgentHookRegistry()

    async def boom(ctx, final_message):
        raise RuntimeError("post_turn boom")

    registry.post_turn.append(boom)

    api_client = _FakeApiClient([_end_turn_event()])
    engine = _make_engine(api_client, registry=registry, cwd=tmp_path)

    events = [e async for e in engine.submit_message("hi")]
    errors = [e for e in events if isinstance(e, ErrorEvent)]
    assert len(errors) == 1
    assert errors[0].recoverable is False
    assert "post_turn" in errors[0].message
    assert "boom" in errors[0].message


# ---------------------------------------------------------------------------
# Hook ordering
# ---------------------------------------------------------------------------


@pytest.mark.asyncio
async def test_hooks_invoked_in_registration_order(tmp_path):
    """Per spec §2.9.1.c: list order = registration order = invocation
    order. Register A then B; A's mutation is visible to B."""
    registry = AgentHookRegistry()
    trail: list[str] = []

    async def first(ctx, messages):
        trail.append("first")
        return messages

    async def second(ctx, messages):
        trail.append("second")
        return messages

    registry.pre_turn.append(first)
    registry.pre_turn.append(second)

    api_client = _FakeApiClient([_end_turn_event()])
    engine = _make_engine(api_client, registry=registry, cwd=tmp_path)

    [e async for e in engine.submit_message("hi")]
    assert trail == ["first", "second"]


# ---------------------------------------------------------------------------
# system_prompt_slots tests
# ---------------------------------------------------------------------------


@pytest.mark.asyncio
async def test_system_prompt_slot_substitution(tmp_path):
    """A registered slot filler replaces its {{slot_name}}
    placeholder in the rendered system prompt."""
    from src.openharness.engine.agent_hooks import AgentHookContext
    from src.openharness.engine.prompts import build_system_prompt

    registry = AgentHookRegistry()

    async def project_specs_filler(ctx):
        return "spec content goes here"

    registry.system_prompt_slots["project_specs"] = project_specs_filler

    ctx = AgentHookContext(
        project_id=1,
        session_id="sess",
        workspace_dir=tmp_path,
        system_prompt_buffer=[],
    )

    prompt = await build_system_prompt(
        language="go",
        workspace_path=str(tmp_path),
        slots=registry.system_prompt_slots,
        hook_context=ctx,
    )
    assert "spec content goes here" in prompt
    assert "{{project_specs}}" not in prompt


@pytest.mark.asyncio
async def test_system_prompt_slot_missing_is_stripped(tmp_path):
    """When no slot filler is registered for {{project_specs}},
    the regex cleanup strips the placeholder."""
    from src.openharness.engine.prompts import build_system_prompt

    prompt = await build_system_prompt(
        language="go",
        workspace_path=str(tmp_path),
        slots=None,
        hook_context=None,
    )
    assert "{{project_specs}}" not in prompt
    assert "{{" not in prompt
