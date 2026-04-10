"""Tests for ReturnChannel — Redis pub/sub subscriber lifecycle.

Uses fakeredis for unit tests. The integration test in
test_clarification_roundtrip_integration.py uses real Redis.

Spec reference: §2.9.2.c.
"""

import asyncio
import json
import logging

import pytest

try:
    import fakeredis.aioredis as fakeredis_aio
    HAS_FAKEREDIS = True
except ImportError:
    HAS_FAKEREDIS = False

from src.openharness.engine.agent_hooks import ClarificationCoordinator
from src.openharness.engine.return_channel import ReturnChannel

pytestmark = pytest.mark.skipif(
    not HAS_FAKEREDIS,
    reason="fakeredis not installed",
)

SESSION_ID = "sess_test_return_channel"
CHANNEL = f"agent:return:{SESSION_ID}"


@pytest.fixture
def coordinator():
    return ClarificationCoordinator()


@pytest.fixture
async def redis_client():
    client = fakeredis_aio.FakeRedis()
    yield client
    await client.aclose()


@pytest.fixture
async def return_channel(redis_client, coordinator):
    rc = await ReturnChannel.open(SESSION_ID, redis_client, coordinator)
    yield rc
    await rc.close()


def _make_message(
    session_id: str = SESSION_ID,
    tool_use_id: str = "toolu_abc",
    response: str = "user answer",
    msg_type: str = "clarification_response",
) -> str:
    return json.dumps({
        "type": msg_type,
        "session_id": session_id,
        "tool_use_id": tool_use_id,
        "response": response,
    })


@pytest.mark.asyncio
async def test_open_and_close_lifecycle(redis_client, coordinator):
    """Open creates a subscriber; close tears it down cleanly."""
    rc = await ReturnChannel.open(SESSION_ID, redis_client, coordinator)
    assert rc is not None
    await rc.close()
    # Double close should not raise
    await rc.close()


@pytest.mark.asyncio
async def test_message_delivery(redis_client, coordinator, return_channel):
    """A valid clarification_response message resolves the coordinator future."""
    async def _publish_after_delay():
        await asyncio.sleep(0.1)
        await redis_client.publish(CHANNEL, _make_message())

    asyncio.create_task(_publish_after_delay())
    result = await coordinator.wait_for("toolu_abc", timeout=5.0)
    assert result == "user answer"


@pytest.mark.asyncio
async def test_wrong_session_id_discarded(redis_client, coordinator, return_channel, caplog):
    """Message with wrong session_id is discarded with a warning."""
    async def _publish():
        await asyncio.sleep(0.1)
        await redis_client.publish(
            CHANNEL,
            _make_message(session_id="sess_wrong"),
        )

    asyncio.create_task(_publish())
    with caplog.at_level(logging.WARNING):
        with pytest.raises(asyncio.TimeoutError):
            await coordinator.wait_for("toolu_abc", timeout=0.3)

    assert "session_id mismatch" in caplog.text or "wrong session" in caplog.text.lower()


@pytest.mark.asyncio
async def test_wrong_type_discarded(redis_client, coordinator, return_channel, caplog):
    """Message with wrong type field is discarded."""
    async def _publish():
        await asyncio.sleep(0.1)
        await redis_client.publish(
            CHANNEL,
            _make_message(msg_type="permission_response"),
        )

    asyncio.create_task(_publish())
    with caplog.at_level(logging.WARNING):
        with pytest.raises(asyncio.TimeoutError):
            await coordinator.wait_for("toolu_abc", timeout=0.3)

    assert "type" in caplog.text.lower()


@pytest.mark.asyncio
async def test_malformed_json_discarded(redis_client, coordinator, return_channel, caplog):
    """Malformed JSON is discarded with a warning."""
    async def _publish():
        await asyncio.sleep(0.1)
        await redis_client.publish(CHANNEL, b"not json at all {{{")

    asyncio.create_task(_publish())
    with caplog.at_level(logging.WARNING):
        with pytest.raises(asyncio.TimeoutError):
            await coordinator.wait_for("toolu_abc", timeout=0.3)

    assert "json" in caplog.text.lower() or "malformed" in caplog.text.lower()


@pytest.mark.asyncio
async def test_unknown_tool_use_id_discarded(redis_client, coordinator, return_channel, caplog):
    """Message with tool_use_id not in pending is discarded."""
    async def _publish():
        await asyncio.sleep(0.1)
        await redis_client.publish(
            CHANNEL,
            _make_message(tool_use_id="toolu_unknown"),
        )

    asyncio.create_task(_publish())
    with caplog.at_level(logging.WARNING):
        with pytest.raises(asyncio.TimeoutError):
            await coordinator.wait_for("toolu_abc", timeout=0.3)

    assert "unknown" in caplog.text.lower()


@pytest.mark.asyncio
async def test_close_cancels_pending_futures(redis_client, coordinator):
    """Closing the return channel cancels all pending coordinator futures."""
    rc = await ReturnChannel.open(SESSION_ID, redis_client, coordinator)

    async def _wait_and_catch():
        try:
            await coordinator.wait_for("toolu_pending", timeout=30.0)
        except (asyncio.CancelledError, asyncio.TimeoutError):
            return "cancelled"
        return "resolved"

    task = asyncio.create_task(_wait_and_catch())
    await asyncio.sleep(0.05)
    await rc.close()
    result = await task
    assert result == "cancelled"


@pytest.mark.asyncio
async def test_close_is_idempotent(redis_client, coordinator):
    """Calling close() twice does not raise."""
    rc = await ReturnChannel.open(SESSION_ID, redis_client, coordinator)
    await rc.close()
    await rc.close()  # Must not raise
