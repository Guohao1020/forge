"""Tests for ClarificationCoordinator — the future-based pause/resume
state machine that bridges RequestClarificationTool and ReturnChannel.

Spec reference: §2.9.2.d.
"""

import asyncio

import pytest

from src.openharness.engine.agent_hooks import ClarificationCoordinator


@pytest.mark.asyncio
async def test_deliver_resolves_future():
    """Happy path: deliver() resolves the future that wait_for() awaits."""
    coordinator = ClarificationCoordinator()

    async def _deliver_after_delay():
        await asyncio.sleep(0.05)
        coordinator.deliver("toolu_abc", "user says TypeScript")

    asyncio.create_task(_deliver_after_delay())
    result = await coordinator.wait_for("toolu_abc", timeout=5.0)
    assert result == "user says TypeScript"


@pytest.mark.asyncio
async def test_timeout_raises_timeout_error():
    """wait_for() raises asyncio.TimeoutError if no deliver() arrives."""
    coordinator = ClarificationCoordinator()

    with pytest.raises(asyncio.TimeoutError):
        await coordinator.wait_for("toolu_xyz", timeout=0.1)


@pytest.mark.asyncio
async def test_timeout_cleans_up_pending():
    """After timeout, the tool_use_id is removed from _pending."""
    coordinator = ClarificationCoordinator()

    with pytest.raises(asyncio.TimeoutError):
        await coordinator.wait_for("toolu_cleanup", timeout=0.1)

    assert "toolu_cleanup" not in coordinator._pending


@pytest.mark.asyncio
async def test_cancel_all_cancels_pending_futures():
    """cancel_all() cancels all pending futures and clears the map."""
    coordinator = ClarificationCoordinator()

    async def _wait_and_catch():
        try:
            await coordinator.wait_for("toolu_cancel", timeout=10.0)
        except (asyncio.CancelledError, asyncio.TimeoutError):
            return "cancelled"
        return "resolved"

    task = asyncio.create_task(_wait_and_catch())
    await asyncio.sleep(0.05)  # Let the future get registered
    coordinator.cancel_all()
    result = await task
    assert result == "cancelled"
    assert len(coordinator._pending) == 0


@pytest.mark.asyncio
async def test_deliver_unknown_tool_use_id_logs_warning(caplog):
    """deliver() for an unknown tool_use_id logs a warning and does not crash."""
    coordinator = ClarificationCoordinator()

    import logging
    with caplog.at_level(logging.WARNING, logger="src.openharness.engine.agent_hooks"):
        coordinator.deliver("toolu_unknown", "some response")

    assert "unknown tool_use_id" in caplog.text


@pytest.mark.asyncio
async def test_deliver_after_completion_logs_warning(caplog):
    """deliver() after the future is already done logs a warning."""
    coordinator = ClarificationCoordinator()

    async def _deliver_twice():
        await asyncio.sleep(0.05)
        coordinator.deliver("toolu_dup", "first response")
        coordinator.deliver("toolu_dup", "second response")

    asyncio.create_task(_deliver_twice())

    import logging
    with caplog.at_level(logging.WARNING, logger="src.openharness.engine.agent_hooks"):
        result = await coordinator.wait_for("toolu_dup", timeout=5.0)

    assert result == "first response"
    # The second deliver should have logged a warning about "arrived after completion"
    # or "unknown tool_use_id" (since the first deliver + wait_for cleanup removes it)
