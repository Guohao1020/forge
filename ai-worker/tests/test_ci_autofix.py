import pytest
from unittest.mock import AsyncMock, MagicMock
from src.openharness.hooks.builtin.ci_autofix_hook import (
    CIAutoFixHook,
    CIAutoFixConfig,
    CIAutoFixResult,
    CIStatus,
)


@pytest.mark.asyncio
async def test_ci_passes_first_try():
    ci = AsyncMock()
    ci.poll_status.return_value = CIStatus.SUCCESS

    hook = CIAutoFixHook(CIAutoFixConfig(max_retries=3))
    result = await hook.run(ci_provider=ci, fix_engine=MagicMock())

    assert result.success
    assert result.attempts == 1
    assert result.ci_status == CIStatus.SUCCESS


@pytest.mark.asyncio
async def test_ci_fails_then_fix_passes():
    from src.openharness.engine.messages import ConversationMessage, TextBlock
    from src.openharness.engine.stream_events import AssistantTurnComplete
    from src.openharness.api.usage import UsageSnapshot

    # First poll: failure, second poll: success
    ci = AsyncMock()
    ci.poll_status.side_effect = [CIStatus.FAILURE, CIStatus.SUCCESS]
    ci.get_logs.return_value = "Error: test failed\nassert 1 == 2"
    ci.push_fix.return_value = None

    # Mock fix engine
    fix_engine = MagicMock()
    fix_msg = ConversationMessage(
        role="assistant",
        content=[TextBlock(text="Fixed the test")],
    )

    async def fix_submit(prompt):
        yield AssistantTurnComplete(
            message=fix_msg,
            usage=UsageSnapshot(input_tokens=10, output_tokens=5),
        )
    fix_engine.submit_message = fix_submit

    hook = CIAutoFixHook(CIAutoFixConfig(max_retries=3, poll_interval_seconds=0, poll_timeout_seconds=1))
    result = await hook.run(ci_provider=ci, fix_engine=fix_engine)

    assert result.success
    assert result.attempts == 2
    assert result.fixed_on_attempt == 2


@pytest.mark.asyncio
async def test_circuit_breaker_exhausted():
    ci = AsyncMock()
    ci.poll_status.return_value = CIStatus.FAILURE
    ci.get_logs.return_value = "persistent error"
    ci.push_fix.return_value = None

    fix_engine = MagicMock()

    async def fix_submit(prompt):
        from src.openharness.engine.messages import ConversationMessage, TextBlock
        from src.openharness.engine.stream_events import AssistantTurnComplete
        from src.openharness.api.usage import UsageSnapshot
        yield AssistantTurnComplete(
            message=ConversationMessage(role="assistant", content=[TextBlock(text="fix")]),
            usage=UsageSnapshot(input_tokens=5, output_tokens=5),
        )
    fix_engine.submit_message = fix_submit

    hook = CIAutoFixHook(CIAutoFixConfig(
        max_retries=2,
        poll_interval_seconds=0,
        poll_timeout_seconds=1,
    ))
    result = await hook.run(ci_provider=ci, fix_engine=fix_engine)

    assert not result.success
    assert result.attempts == 2
    assert "exhausted" in result.last_error.lower()


@pytest.mark.asyncio
async def test_ci_polling_error():
    ci = AsyncMock()
    ci.poll_status.side_effect = ConnectionError("CI unreachable")

    hook = CIAutoFixHook(CIAutoFixConfig(max_retries=3, poll_interval_seconds=0, poll_timeout_seconds=1))
    result = await hook.run(ci_provider=ci, fix_engine=MagicMock())

    assert not result.success
    assert result.ci_status == CIStatus.ERROR
    assert "polling error" in result.last_error.lower()
