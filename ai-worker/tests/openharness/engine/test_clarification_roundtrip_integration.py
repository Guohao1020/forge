"""Integration test — real Redis round-trip for clarification.

Requires docker-compose dev Redis on localhost:6379.
Skips cleanly if Redis is not available.

Spec reference: §2.9.2 full lifecycle.
"""

import asyncio
import json
import os

import pytest

# Try to import redis.asyncio — skip entire module if not available
try:
    import redis.asyncio as aioredis
    HAS_AIOREDIS = True
except ImportError:
    HAS_AIOREDIS = False

from src.openharness.engine.agent_hooks import ClarificationCoordinator
from src.openharness.engine.return_channel import ReturnChannel

pytestmark = [
    pytest.mark.skipif(not HAS_AIOREDIS, reason="redis.asyncio not installed"),
    pytest.mark.integration,
]

REDIS_URL = os.environ.get("FORGE_REDIS_URL", "redis://:forge_redis_2026@localhost:6379/0")


async def _redis_available() -> bool:
    """Check if Redis is reachable."""
    try:
        client = aioredis.from_url(REDIS_URL, decode_responses=False)
        await client.ping()
        await client.aclose()
        return True
    except Exception:
        return False


@pytest.fixture
async def real_redis():
    """Provide a real Redis client. Skip if not available."""
    if not await _redis_available():
        pytest.skip("Redis not available at " + REDIS_URL)
    client = aioredis.from_url(REDIS_URL, decode_responses=False)
    yield client
    await client.aclose()


@pytest.mark.asyncio
async def test_full_clarification_roundtrip(real_redis):
    """Complete pause -> publish -> resume cycle with real Redis.

    1. Create ClarificationCoordinator
    2. Open ReturnChannel on real Redis
    3. Background task publishes a clarification_response after 100ms
    4. coordinator.wait_for() resolves with the published response
    5. Close ReturnChannel
    6. Verify cleanup (no lingering subscriptions)
    """
    session_id = "sess_integration_roundtrip"
    tool_use_id = "toolu_integration_001"
    expected_response = "use Python with pytest"

    coordinator = ClarificationCoordinator()
    rc = await ReturnChannel.open(session_id, real_redis, coordinator)

    try:
        # Background task: publish response after 100ms
        async def _publish_response():
            await asyncio.sleep(0.1)
            msg = json.dumps({
                "type": "clarification_response",
                "session_id": session_id,
                "tool_use_id": tool_use_id,
                "response": expected_response,
            })
            # Use a separate client for publishing (real Redis allows this)
            pub_client = aioredis.from_url(REDIS_URL, decode_responses=False)
            await pub_client.publish(f"agent:return:{session_id}", msg)
            await pub_client.aclose()

        asyncio.create_task(_publish_response())

        # Wait for the response
        result = await coordinator.wait_for(tool_use_id, timeout=5.0)
        assert result == expected_response

    finally:
        await rc.close()

    # Verify cleanup — no pending futures
    assert len(coordinator._pending) == 0


@pytest.mark.asyncio
async def test_roundtrip_with_timeout(real_redis):
    """Timeout path: no publisher -> TimeoutError."""
    session_id = "sess_integration_timeout"
    tool_use_id = "toolu_integration_timeout"

    coordinator = ClarificationCoordinator()
    rc = await ReturnChannel.open(session_id, real_redis, coordinator)

    try:
        with pytest.raises(asyncio.TimeoutError):
            await coordinator.wait_for(tool_use_id, timeout=0.3)
    finally:
        await rc.close()

    assert len(coordinator._pending) == 0


@pytest.mark.asyncio
async def test_roundtrip_close_during_wait(real_redis):
    """Closing the channel while a wait is pending cancels the future."""
    session_id = "sess_integration_close"
    tool_use_id = "toolu_integration_close"

    coordinator = ClarificationCoordinator()
    rc = await ReturnChannel.open(session_id, real_redis, coordinator)

    async def _wait_and_catch():
        try:
            await coordinator.wait_for(tool_use_id, timeout=30.0)
        except (asyncio.CancelledError, asyncio.TimeoutError):
            return "cancelled"
        return "resolved"

    task = asyncio.create_task(_wait_and_catch())
    await asyncio.sleep(0.1)
    await rc.close()

    result = await task
    assert result == "cancelled"
    assert len(coordinator._pending) == 0


@pytest.mark.asyncio
async def test_multiple_sequential_clarifications(real_redis):
    """Two clarifications in the same session, resolved sequentially."""
    session_id = "sess_integration_multi"

    coordinator = ClarificationCoordinator()
    rc = await ReturnChannel.open(session_id, real_redis, coordinator)

    try:
        pub_client = aioredis.from_url(REDIS_URL, decode_responses=False)

        # First clarification
        async def _publish_first():
            await asyncio.sleep(0.1)
            msg = json.dumps({
                "type": "clarification_response",
                "session_id": session_id,
                "tool_use_id": "toolu_q1",
                "response": "TypeScript",
            })
            await pub_client.publish(f"agent:return:{session_id}", msg)

        asyncio.create_task(_publish_first())
        result1 = await coordinator.wait_for("toolu_q1", timeout=5.0)
        assert result1 == "TypeScript"

        # Second clarification (sequential — per §2.9.2.h)
        async def _publish_second():
            await asyncio.sleep(0.1)
            msg = json.dumps({
                "type": "clarification_response",
                "session_id": session_id,
                "tool_use_id": "toolu_q2",
                "response": "Jest",
            })
            await pub_client.publish(f"agent:return:{session_id}", msg)

        asyncio.create_task(_publish_second())
        result2 = await coordinator.wait_for("toolu_q2", timeout=5.0)
        assert result2 == "Jest"

        await pub_client.aclose()
    finally:
        await rc.close()
