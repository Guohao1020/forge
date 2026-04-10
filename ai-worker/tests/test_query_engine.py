import pytest
from unittest.mock import AsyncMock, MagicMock
from src.openharness.engine.query_engine import QueryEngine
from src.openharness.engine.messages import ConversationMessage, TextBlock
from src.openharness.engine.stream_events import AssistantTextDelta, AssistantTurnComplete
from src.openharness.tools.base import SimpleTool, ToolRegistry, ToolResult
from src.openharness.api.usage import UsageSnapshot


@pytest.mark.asyncio
async def test_submit_message_simple():
    mock_client = AsyncMock()
    mock_msg = ConversationMessage(
        role="assistant", content=[TextBlock(text="Hello!")],
    )
    mock_usage = UsageSnapshot(input_tokens=10, output_tokens=5)

    async def mock_stream(request):
        from src.openharness.api.client import ApiTextDeltaEvent, ApiMessageCompleteEvent
        yield ApiTextDeltaEvent(text="Hello!")
        yield ApiMessageCompleteEvent(
            message=mock_msg, usage=mock_usage, stop_reason="end_turn",
        )

    mock_client.stream_message = mock_stream

    engine = QueryEngine(
        api_client=mock_client,
        tool_registry=ToolRegistry(),
        model="test",
        system_prompt="You are helpful.",
    )
    events = []
    async for event in engine.submit_message("Hi"):
        events.append(event)

    assert any(isinstance(e, AssistantTextDelta) for e in events)
    assert any(isinstance(e, AssistantTurnComplete) for e in events)
    assert engine.total_usage.total_tokens == 15


def test_engine_clear():
    engine = QueryEngine(
        api_client=MagicMock(),
        tool_registry=ToolRegistry(),
        model="test",
        system_prompt="test",
    )
    engine.clear()
    assert len(engine.messages) == 0


def test_engine_set_system_prompt():
    engine = QueryEngine(
        api_client=MagicMock(),
        tool_registry=ToolRegistry(),
        model="test",
        system_prompt="original",
    )
    engine.set_system_prompt("updated")
    assert engine._system_prompt == "updated"


def test_engine_set_model():
    engine = QueryEngine(
        api_client=MagicMock(),
        tool_registry=ToolRegistry(),
        model="original",
        system_prompt="test",
    )
    engine.set_model("new-model")
    assert engine._model == "new-model"


@pytest.mark.asyncio
async def test_submit_message_with_tool_call():
    """Test that tool calls are executed and results fed back."""
    from src.openharness.api.client import (
        ApiTextDeltaEvent, ApiMessageCompleteEvent,
    )
    from src.openharness.engine.messages import ToolUseBlock, ToolResultBlock
    from src.openharness.tools.base import SimpleTool, ToolResult, ToolExecutionContext
    from pydantic import BaseModel
    from pathlib import Path

    class EchoInput(BaseModel):
        text: str

    class EchoTool(SimpleTool):
        name = "echo"
        description = "Echoes text"
        input_model = EchoInput

        def is_read_only(self, arguments):
            return True

        async def _execute_simple(self, arguments, context):
            return ToolResult(output=f"Echo: {arguments.text}")

    registry = ToolRegistry()
    registry.register(EchoTool())

    call_count = 0

    async def mock_stream(request):
        nonlocal call_count
        call_count += 1
        if call_count == 1:
            # First call: AI wants to use a tool
            msg = ConversationMessage(
                role="assistant",
                content=[ToolUseBlock(id="t1", name="echo", input={"text": "hello"})],
            )
            yield ApiMessageCompleteEvent(
                message=msg,
                usage=UsageSnapshot(input_tokens=10, output_tokens=5),
                stop_reason="tool_use",
            )
        else:
            # Second call: AI produces final response
            msg = ConversationMessage(
                role="assistant",
                content=[TextBlock(text="Done!")],
            )
            yield ApiTextDeltaEvent(text="Done!")
            yield ApiMessageCompleteEvent(
                message=msg,
                usage=UsageSnapshot(input_tokens=20, output_tokens=10),
                stop_reason="end_turn",
            )

    mock_client = AsyncMock()
    mock_client.stream_message = mock_stream

    engine = QueryEngine(
        api_client=mock_client,
        tool_registry=registry,
        model="test",
        system_prompt="test",
    )

    events = []
    async for event in engine.submit_message("Use echo"):
        events.append(event)

    from src.openharness.engine.stream_events import (
        ToolExecutionStarted, ToolExecutionCompleted,
    )
    assert any(isinstance(e, ToolExecutionStarted) for e in events)
    assert any(isinstance(e, ToolExecutionCompleted) for e in events)
    assert any(isinstance(e, AssistantTextDelta) for e in events)
    assert call_count == 2
    # Total usage across both API calls
    assert engine.total_usage.total_tokens == 45
