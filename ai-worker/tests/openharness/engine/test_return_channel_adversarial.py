"""Adversarial return channel tests — 9 edge cases from spec §7.1.

P0 gate: ALL 9 tests must pass. A single failure blocks Phase 5a.

These tests verify that malicious or buggy publishers cannot crash
the agent, steal data from other sessions, or cause undefined behavior.
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


def _msg(
    session_id: str = "sess_adv",
    tool_use_id: str = "toolu_adv",
    response: str = "answer",
    msg_type: str = "clarification_response",
) -> str:
    return json.dumps({
        "type": msg_type,
        "session_id": session_id,
        "tool_use_id": tool_use_id,
        "response": response,
    })


@pytest.fixture
async def redis_client():
    client = fakeredis_aio.FakeRedis()
    yield client
    await client.aclose()


# ---- Test 1: Wrong session_id -> discarded, no crash ----

@pytest.mark.asyncio
async def test_adv_wrong_session_id_discarded(redis_client, caplog):
    """Message with wrong session_id is silently discarded."""
    coordinator = ClarificationCoordinator()
    rc = await ReturnChannel.open("sess_adv", redis_client, coordinator)
    try:
        async def _publish():
            await asyncio.sleep(0.05)
            await redis_client.publish(
                "agent:return:sess_adv",
                _msg(session_id="sess_WRONG"),
            )

        asyncio.create_task(_publish())

        with caplog.at_level(logging.WARNING):
            with pytest.raises(asyncio.TimeoutError):
                await coordinator.wait_for("toolu_adv", timeout=0.3)

        assert "mismatch" in caplog.text.lower() or "wrong" in caplog.text.lower()
    finally:
        await rc.close()


# ---- Test 2: Wrong message type -> discarded, no crash ----

@pytest.mark.asyncio
async def test_adv_wrong_message_type_discarded(redis_client, caplog):
    """Message with wrong type field is discarded."""
    coordinator = ClarificationCoordinator()
    rc = await ReturnChannel.open("sess_adv", redis_client, coordinator)
    try:
        async def _publish():
            await asyncio.sleep(0.05)
            await redis_client.publish(
                "agent:return:sess_adv",
                _msg(msg_type="permission_grant"),
            )

        asyncio.create_task(_publish())

        with caplog.at_level(logging.WARNING):
            with pytest.raises(asyncio.TimeoutError):
                await coordinator.wait_for("toolu_adv", timeout=0.3)

        assert "type" in caplog.text.lower()
    finally:
        await rc.close()


# ---- Test 3: Unknown tool_use_id -> discarded, no crash ----

@pytest.mark.asyncio
async def test_adv_unknown_tool_use_id_discarded(redis_client, caplog):
    """Message with tool_use_id not in pending map is discarded."""
    coordinator = ClarificationCoordinator()
    rc = await ReturnChannel.open("sess_adv", redis_client, coordinator)
    try:
        async def _publish():
            await asyncio.sleep(0.05)
            await redis_client.publish(
                "agent:return:sess_adv",
                _msg(tool_use_id="toolu_NOT_PENDING"),
            )

        asyncio.create_task(_publish())

        with caplog.at_level(logging.WARNING):
            with pytest.raises(asyncio.TimeoutError):
                await coordinator.wait_for("toolu_adv", timeout=0.3)

        assert "unknown" in caplog.text.lower()
    finally:
        await rc.close()


# ---- Test 4: Malformed JSON -> discarded, no crash ----

@pytest.mark.asyncio
async def test_adv_malformed_json_discarded(redis_client, caplog):
    """Non-JSON payload is silently discarded."""
    coordinator = ClarificationCoordinator()
    rc = await ReturnChannel.open("sess_adv", redis_client, coordinator)
    try:
        async def _publish():
            await asyncio.sleep(0.05)
            await redis_client.publish(
                "agent:return:sess_adv",
                b"this is not json {{{",
            )

        asyncio.create_task(_publish())

        with caplog.at_level(logging.WARNING):
            with pytest.raises(asyncio.TimeoutError):
                await coordinator.wait_for("toolu_adv", timeout=0.3)

        assert "json" in caplog.text.lower() or "malformed" in caplog.text.lower()
    finally:
        await rc.close()


# ---- Test 5: Response after timeout -> discarded (future already done) ----

@pytest.mark.asyncio
async def test_adv_response_after_timeout_discarded(redis_client, caplog):
    """Response arriving after the future timed out is discarded."""
    coordinator = ClarificationCoordinator()
    rc = await ReturnChannel.open("sess_adv", redis_client, coordinator)
    try:
        # Wait with very short timeout
        with pytest.raises(asyncio.TimeoutError):
            await coordinator.wait_for("toolu_late", timeout=0.1)

        # Now publish a response for the timed-out tool_use_id
        with caplog.at_level(logging.WARNING):
            await redis_client.publish(
                "agent:return:sess_adv",
                _msg(tool_use_id="toolu_late"),
            )
            await asyncio.sleep(0.1)  # Let listener process

        # The coordinator should have logged "unknown tool_use_id"
        # because the future was cleaned up on timeout
        assert "unknown" in caplog.text.lower() or len(coordinator._pending) == 0
    finally:
        await rc.close()


# ---- Test 6: Duplicate response for same tool_use_id -> second discarded ----

@pytest.mark.asyncio
async def test_adv_duplicate_response_second_discarded(redis_client):
    """Only the first response is delivered; the second is discarded."""
    coordinator = ClarificationCoordinator()
    rc = await ReturnChannel.open("sess_adv", redis_client, coordinator)
    try:
        async def _publish_twice():
            await asyncio.sleep(0.05)
            await redis_client.publish(
                "agent:return:sess_adv",
                _msg(tool_use_id="toolu_dup", response="first"),
            )
            await asyncio.sleep(0.05)
            await redis_client.publish(
                "agent:return:sess_adv",
                _msg(tool_use_id="toolu_dup", response="second"),
            )

        asyncio.create_task(_publish_twice())
        result = await coordinator.wait_for("toolu_dup", timeout=5.0)
        assert result == "first"
    finally:
        await rc.close()


# ---- Test 7: Empty response string -> accepted (valid per spec) ----

@pytest.mark.asyncio
async def test_adv_empty_response_accepted(redis_client):
    """Empty string response is valid — user hit enter with nothing."""
    coordinator = ClarificationCoordinator()
    rc = await ReturnChannel.open("sess_adv", redis_client, coordinator)
    try:
        async def _publish():
            await asyncio.sleep(0.05)
            await redis_client.publish(
                "agent:return:sess_adv",
                _msg(tool_use_id="toolu_empty", response=""),
            )

        asyncio.create_task(_publish())
        result = await coordinator.wait_for("toolu_empty", timeout=5.0)
        assert result == ""
    finally:
        await rc.close()


# ---- Test 8: Response at exactly 4 KiB limit -> accepted ----

@pytest.mark.asyncio
async def test_adv_response_at_4kb_limit_accepted(redis_client):
    """4096-byte response is valid — the limit is at the forge-core endpoint."""
    coordinator = ClarificationCoordinator()
    rc = await ReturnChannel.open("sess_adv", redis_client, coordinator)
    try:
        big_response = "x" * 4096

        async def _publish():
            await asyncio.sleep(0.05)
            await redis_client.publish(
                "agent:return:sess_adv",
                _msg(tool_use_id="toolu_4kb", response=big_response),
            )

        asyncio.create_task(_publish())
        result = await coordinator.wait_for("toolu_4kb", timeout=5.0)
        assert result == big_response
        assert len(result) == 4096
    finally:
        await rc.close()


# ---- Test 9: Concurrent 10 sessions each awaiting clarification ----

@pytest.mark.asyncio
async def test_adv_concurrent_10_sessions(redis_client):
    """10 sessions, each awaiting clarification, all resolve independently."""
    sessions = []
    for i in range(10):
        coord = ClarificationCoordinator()
        sid = f"sess_conc_{i}"
        rc = await ReturnChannel.open(sid, redis_client, coord)
        sessions.append((sid, coord, rc))

    try:
        # Publish a response for each session
        async def _publish_all():
            await asyncio.sleep(0.1)
            for sid, _, _ in sessions:
                msg = json.dumps({
                    "type": "clarification_response",
                    "session_id": sid,
                    "tool_use_id": f"toolu_{sid}",
                    "response": f"answer_for_{sid}",
                })
                await redis_client.publish(f"agent:return:{sid}", msg)

        asyncio.create_task(_publish_all())

        # Wait for all 10 to resolve
        results = await asyncio.gather(*[
            coord.wait_for(f"toolu_{sid}", timeout=5.0)
            for sid, coord, _ in sessions
        ])

        for i, result in enumerate(results):
            assert result == f"answer_for_sess_conc_{i}"
    finally:
        for _, _, rc in sessions:
            await rc.close()
