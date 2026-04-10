"""Integration tests — verify the full pipeline: tool → hooks → permissions → engine."""

import pytest
from unittest.mock import AsyncMock
from pydantic import BaseModel
from pathlib import Path

from src.openharness.tools.base import SimpleTool, ToolRegistry, ToolResult, ToolExecutionContext
from src.openharness.hooks.events import HookEvent
from src.openharness.hooks.loader import HookRegistry
from src.openharness.hooks.executor import HookExecutor, HookResult, AggregatedHookResult
from src.openharness.engine.messages import ConversationMessage, TextBlock, ToolResultBlock
from src.openharness.engine.query import QueryContext, _execute_tool_call
from src.openharness.api.usage import UsageSnapshot
from src.openharness.permissions.checker import PermissionChecker
from src.openharness.permissions.modes import PermissionMode


class UpperInput(BaseModel):
    text: str


class UpperTool(SimpleTool):
    name = "upper"
    description = "Uppercase text"
    input_model = UpperInput

    def is_read_only(self, arguments):
        return True

    async def _execute_simple(self, arguments, context):
        return ToolResult(output=arguments.text.upper())


class FailingInput(BaseModel):
    text: str


class FailingTool(SimpleTool):
    name = "fail"
    description = "Always fails"
    input_model = FailingInput

    async def _execute_simple(self, arguments, context):
        raise RuntimeError("Intentional failure")


async def _collect_final_block(context, tool_name, tool_use_id, tool_input):
    """Helper: consume the _execute_tool_call async generator and return the ToolResultBlock."""
    final = None
    async for item in _execute_tool_call(
        context=context,
        tool_name=tool_name,
        tool_use_id=tool_use_id,
        tool_input=tool_input,
    ):
        if isinstance(item, ToolResultBlock):
            final = item
    assert final is not None, "_execute_tool_call yielded no ToolResultBlock"
    return final


@pytest.mark.asyncio
async def test_tool_to_engine_pipeline():
    registry = ToolRegistry()
    registry.register(UpperTool())
    result = await _collect_final_block(
        context=QueryContext(
            api_client=AsyncMock(), tool_registry=registry,
            model="test", system_prompt="test",
        ),
        tool_name="upper", tool_use_id="t1", tool_input={"text": "hello"},
    )
    assert result.content == "HELLO"
    assert not result.is_error


@pytest.mark.asyncio
async def test_hook_blocks_tool():
    registry = ToolRegistry()
    registry.register(UpperTool())
    hook_registry = HookRegistry()
    executor = HookExecutor(hook_registry)

    # Monkey-patch to simulate blocking
    original = executor.execute

    async def blocking(event, payload):
        if event == HookEvent.PRE_TOOL_USE:
            return AggregatedHookResult(results=[
                HookResult(
                    hook_type="test", success=False, output="",
                    blocked=True, reason="Forbidden",
                ),
            ])
        return AggregatedHookResult(results=[])
    executor.execute = blocking

    result = await _collect_final_block(
        context=QueryContext(
            api_client=AsyncMock(), tool_registry=registry,
            model="test", system_prompt="test",
            hook_executor=executor,
        ),
        tool_name="upper", tool_use_id="t1", tool_input={"text": "hello"},
    )
    assert result.is_error
    assert "BLOCKED" in result.content


@pytest.mark.asyncio
async def test_unknown_tool_error():
    result = await _collect_final_block(
        context=QueryContext(
            api_client=AsyncMock(), tool_registry=ToolRegistry(),
            model="test", system_prompt="test",
        ),
        tool_name="nope", tool_use_id="t1", tool_input={},
    )
    assert result.is_error
    assert "Unknown" in result.content


@pytest.mark.asyncio
async def test_tool_execution_error():
    registry = ToolRegistry()
    registry.register(FailingTool())
    result = await _collect_final_block(
        context=QueryContext(
            api_client=AsyncMock(), tool_registry=registry,
            model="test", system_prompt="test",
        ),
        tool_name="fail", tool_use_id="t1", tool_input={"text": "hello"},
    )
    assert result.is_error
    assert "error" in result.content.lower()


@pytest.mark.asyncio
async def test_permission_denies_tool():
    registry = ToolRegistry()
    registry.register(UpperTool())
    checker = PermissionChecker(
        mode=PermissionMode.FULL_AUTO,
        denied_tools=["upper"],
    )
    result = await _collect_final_block(
        context=QueryContext(
            api_client=AsyncMock(), tool_registry=registry,
            model="test", system_prompt="test",
            permission_checker=checker,
        ),
        tool_name="upper", tool_use_id="t1", tool_input={"text": "hello"},
    )
    assert result.is_error
    assert "denied" in result.content.lower()


def test_permission_integration():
    auto = PermissionChecker(mode=PermissionMode.FULL_AUTO)
    default = PermissionChecker(mode=PermissionMode.DEFAULT)
    assert auto.evaluate("bash", is_read_only=False).allowed
    assert not default.evaluate("bash", is_read_only=False).allowed
    assert default.evaluate("bash", is_read_only=False).requires_confirmation


def test_skill_loading():
    from src.openharness.skills.loader import load_skill_registry
    registry = load_skill_registry("skills/")
    skills = registry.list_skills()
    assert len(skills) >= 6, f"Expected 6+ skills, got {len(skills)}: {[s.name for s in skills]}"
