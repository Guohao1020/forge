import pytest
from unittest.mock import AsyncMock, MagicMock, patch
from src.openharness.api.providers.router_adapter import ModelRouterAdapter
from src.openharness.api.client import (
    ApiMessageRequest, ApiTextDeltaEvent, ApiMessageCompleteEvent,
)
from src.openharness.engine.messages import ConversationMessage
from src.openharness.api.usage import UsageSnapshot


@pytest.mark.asyncio
async def test_router_adapter_yields_events():
    mock_router = MagicMock()
    mock_resp = MagicMock()
    mock_resp.content = "Hello"
    mock_resp.model = "test"
    mock_resp.provider = "test"
    mock_resp.input_tokens = 10
    mock_resp.output_tokens = 5
    mock_resp.latency_ms = 100
    mock_resp.stop_reason = "end_turn"
    mock_resp.tool_calls = []
    mock_resp.raw_content = None

    mock_router.chat = AsyncMock(return_value=mock_resp)

    from src.models.router import Purpose
    adapter = ModelRouterAdapter(mock_router, purpose=Purpose.GENERATE)
    request = ApiMessageRequest(
        model="test",
        messages=[ConversationMessage.from_user_text("hi")],
        system_prompt="test",
    )
    events = []
    async for event in adapter.stream_message(request):
        events.append(event)

    assert len(events) == 2
    assert isinstance(events[0], ApiTextDeltaEvent)
    assert events[0].text == "Hello"
    assert isinstance(events[1], ApiMessageCompleteEvent)
    assert events[1].usage.total_tokens == 15
    assert events[1].stop_reason == "end_turn"


@pytest.mark.asyncio
async def test_router_adapter_with_tool_calls():
    mock_router = MagicMock()
    mock_resp = MagicMock()
    mock_resp.content = ""
    mock_resp.model = "test"
    mock_resp.provider = "test"
    mock_resp.input_tokens = 20
    mock_resp.output_tokens = 15
    mock_resp.latency_ms = 200
    mock_resp.stop_reason = "tool_use"
    tc_mock = MagicMock()
    tc_mock.id = "t1"
    tc_mock.name = "file_read"
    tc_mock.input = {"path": "/tmp/test"}
    mock_resp.tool_calls = [tc_mock]
    mock_resp.raw_content = None

    mock_router.chat = AsyncMock(return_value=mock_resp)

    from src.models.router import Purpose
    adapter = ModelRouterAdapter(mock_router, purpose=Purpose.GENERATE)
    request = ApiMessageRequest(
        model="test",
        messages=[ConversationMessage.from_user_text("read file")],
        system_prompt="test",
        tools=[{"name": "file_read", "description": "read file", "input_schema": {}}],
    )
    events = []
    async for event in adapter.stream_message(request):
        events.append(event)

    # Should have a complete event with tool use blocks in the message
    assert len(events) >= 1
    complete_evt = [e for e in events if isinstance(e, ApiMessageCompleteEvent)]
    assert len(complete_evt) == 1
    assert complete_evt[0].stop_reason == "tool_use"
    assert len(complete_evt[0].message.tool_uses) == 1


@pytest.mark.asyncio
async def test_router_adapter_empty_response():
    mock_router = MagicMock()
    mock_resp = MagicMock()
    mock_resp.content = ""
    mock_resp.model = "test"
    mock_resp.provider = "test"
    mock_resp.input_tokens = 5
    mock_resp.output_tokens = 0
    mock_resp.latency_ms = 50
    mock_resp.stop_reason = "end_turn"
    mock_resp.tool_calls = []
    mock_resp.raw_content = None

    mock_router.chat = AsyncMock(return_value=mock_resp)

    from src.models.router import Purpose
    adapter = ModelRouterAdapter(mock_router, purpose=Purpose.GENERATE)
    request = ApiMessageRequest(
        model="test",
        messages=[ConversationMessage.from_user_text("hi")],
        system_prompt="test",
    )
    events = []
    async for event in adapter.stream_message(request):
        events.append(event)

    # Should still get a complete event even with empty content
    complete = [e for e in events if isinstance(e, ApiMessageCompleteEvent)]
    assert len(complete) == 1
