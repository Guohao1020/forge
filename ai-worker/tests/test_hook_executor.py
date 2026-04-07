import pytest
from src.openharness.hooks.events import HookEvent
from src.openharness.hooks.executor import HookExecutor, HookResult, AggregatedHookResult
from src.openharness.hooks.loader import HookRegistry
from src.openharness.hooks.schemas import CommandHookDefinition


def test_hook_event_values():
    assert HookEvent.PRE_TOOL_USE == "pre_tool_use"
    assert HookEvent.POST_TOOL_USE == "post_tool_use"
    assert HookEvent.POST_GENERATION == "post_generation"


def test_hook_registry_register_and_get():
    registry = HookRegistry()
    hook = CommandHookDefinition(command="echo ok")
    registry.register(HookEvent.PRE_TOOL_USE, hook)
    hooks = registry.get(HookEvent.PRE_TOOL_USE)
    assert len(hooks) == 1


def test_hook_registry_get_empty():
    registry = HookRegistry()
    assert registry.get(HookEvent.POST_GENERATION) == []


def test_hook_result_not_blocked():
    result = HookResult(hook_type="command", success=True, output="ok")
    assert not result.blocked


def test_hook_result_blocked():
    result = HookResult(
        hook_type="command", success=False, output="denied",
        blocked=True, reason="forbidden",
    )
    assert result.blocked
    assert result.reason == "forbidden"


def test_aggregated_blocked():
    r1 = HookResult(hook_type="command", success=True, output="ok")
    r2 = HookResult(
        hook_type="command", success=True, output="denied",
        blocked=True, reason="forbidden",
    )
    agg = AggregatedHookResult(results=[r1, r2])
    assert agg.blocked
    assert agg.reason == "forbidden"


def test_aggregated_not_blocked():
    agg = AggregatedHookResult(results=[
        HookResult(hook_type="command", success=True, output="ok"),
    ])
    assert not agg.blocked
    assert agg.reason == ""


def test_aggregated_all_reasons():
    """All blocked reasons should be collected, not just the first."""
    r1 = HookResult(hook_type="command", success=False, output="",
                    blocked=True, reason="security")
    r2 = HookResult(hook_type="command", success=False, output="",
                    blocked=True, reason="build failed")
    agg = AggregatedHookResult(results=[r1, r2])
    assert agg.blocked
    reasons = agg.all_reasons
    assert "security" in reasons
    assert "build failed" in reasons


@pytest.mark.asyncio
async def test_executor_runs_command_hook():
    registry = HookRegistry()
    hook = CommandHookDefinition(command="echo hello")
    registry.register(HookEvent.POST_GENERATION, hook)
    executor = HookExecutor(registry)
    result = await executor.execute(HookEvent.POST_GENERATION, {})
    assert not result.blocked
    assert len(result.results) == 1
    assert result.results[0].success


@pytest.mark.asyncio
async def test_executor_skips_on_matcher_mismatch():
    registry = HookRegistry()
    hook = CommandHookDefinition(
        command="echo matched",
        matcher={"tool_name": "bash"},
    )
    registry.register(HookEvent.PRE_TOOL_USE, hook)
    executor = HookExecutor(registry)
    result = await executor.execute(HookEvent.PRE_TOOL_USE, {"tool_name": "file_read"})
    assert len(result.results) == 0


@pytest.mark.asyncio
async def test_executor_matches_tool_name():
    registry = HookRegistry()
    hook = CommandHookDefinition(
        command="echo matched",
        matcher={"tool_name": "bash"},
    )
    registry.register(HookEvent.PRE_TOOL_USE, hook)
    executor = HookExecutor(registry)
    result = await executor.execute(HookEvent.PRE_TOOL_USE, {"tool_name": "bash"})
    assert len(result.results) == 1
    assert result.results[0].success


@pytest.mark.asyncio
async def test_executor_timeout():
    registry = HookRegistry()
    hook = CommandHookDefinition(command="sleep 10", timeout_seconds=1)
    registry.register(HookEvent.POST_GENERATION, hook)
    executor = HookExecutor(registry)
    result = await executor.execute(HookEvent.POST_GENERATION, {})
    assert len(result.results) == 1
    assert not result.results[0].success
    assert "timed out" in result.results[0].reason.lower()


@pytest.mark.asyncio
async def test_executor_block_on_failure():
    registry = HookRegistry()
    hook = CommandHookDefinition(
        command="exit 1",
        block_on_failure=True,
    )
    registry.register(HookEvent.POST_GENERATION, hook)
    executor = HookExecutor(registry)
    result = await executor.execute(HookEvent.POST_GENERATION, {})
    assert result.blocked


@pytest.mark.asyncio
async def test_executor_not_found_command():
    """FileNotFoundError should be caught, not crash."""
    registry = HookRegistry()
    hook = CommandHookDefinition(command=["__nonexistent_binary_xyz__", "arg1"])
    registry.register(HookEvent.POST_GENERATION, hook)
    executor = HookExecutor(registry)
    result = await executor.execute(HookEvent.POST_GENERATION, {})
    assert len(result.results) == 1
    assert not result.results[0].success
