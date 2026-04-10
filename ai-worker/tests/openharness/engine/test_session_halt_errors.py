"""Tests for the SessionHaltError exception hierarchy.

These exceptions halt the session rather than being translated to
ToolResult(is_error=True). See spec §2.9.2.d and §4.1 BaseTool
contract update.
"""

import pytest

from src.openharness.engine.agent_hooks import (
    ClarificationTimeout,
    ReturnChannelError,
    SessionHaltError,
)


class TestSessionHaltErrorHierarchy:
    """Verify subclass relationships — _execute_tool_call catches
    SessionHaltError as a family, not individual subclasses."""

    def test_session_halt_error_is_exception(self):
        assert issubclass(SessionHaltError, Exception)

    def test_clarification_timeout_is_session_halt_error(self):
        assert issubclass(ClarificationTimeout, SessionHaltError)

    def test_return_channel_error_is_session_halt_error(self):
        assert issubclass(ReturnChannelError, SessionHaltError)


class TestClarificationTimeoutConstruction:
    """Verify ClarificationTimeout stores and exposes its fields."""

    def test_construction_and_attributes(self):
        err = ClarificationTimeout(
            tool_use_id="toolu_01HY_abc",
            timeout_seconds=600.0,
        )
        assert err.tool_use_id == "toolu_01HY_abc"
        assert err.timeout_seconds == 600.0

    def test_str_representation_contains_tool_use_id(self):
        err = ClarificationTimeout(
            tool_use_id="toolu_xyz",
            timeout_seconds=300.0,
        )
        msg = str(err)
        assert "toolu_xyz" in msg
        assert "300" in msg

    def test_str_representation_contains_timeout(self):
        err = ClarificationTimeout(
            tool_use_id="toolu_abc",
            timeout_seconds=600.0,
        )
        assert "600" in str(err)


class TestReturnChannelError:
    """Verify ReturnChannelError is usable as a standalone exception."""

    def test_construction_with_message(self):
        err = ReturnChannelError("Redis connection lost")
        assert "Redis connection lost" in str(err)

    def test_construction_empty(self):
        err = ReturnChannelError()
        assert isinstance(err, SessionHaltError)
