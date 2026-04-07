import pytest
from src.openharness.permissions.modes import PermissionMode
from src.openharness.permissions.checker import PermissionChecker, PermissionDecision


def test_read_only_always_allowed():
    checker = PermissionChecker(mode=PermissionMode.DEFAULT)
    d = checker.evaluate("file_read", is_read_only=True)
    assert d.allowed
    assert not d.requires_confirmation


def test_mutating_requires_confirmation():
    checker = PermissionChecker(mode=PermissionMode.DEFAULT)
    d = checker.evaluate("bash", is_read_only=False)
    assert not d.allowed
    assert d.requires_confirmation


def test_full_auto_allows_all():
    checker = PermissionChecker(mode=PermissionMode.FULL_AUTO)
    d = checker.evaluate("bash", is_read_only=False)
    assert d.allowed
    assert not d.requires_confirmation


def test_denied_tool():
    checker = PermissionChecker(mode=PermissionMode.FULL_AUTO, denied_tools=["bash"])
    d = checker.evaluate("bash", is_read_only=False)
    assert not d.allowed
    assert "denied" in d.reason.lower()


def test_allowed_tool_overrides_default():
    checker = PermissionChecker(mode=PermissionMode.DEFAULT, allowed_tools=["bash"])
    d = checker.evaluate("bash", is_read_only=False)
    assert d.allowed


def test_denied_takes_precedence_over_allowed():
    """If a tool is in both allowed and denied, denied wins."""
    checker = PermissionChecker(
        mode=PermissionMode.FULL_AUTO,
        allowed_tools=["bash"],
        denied_tools=["bash"],
    )
    d = checker.evaluate("bash", is_read_only=False)
    assert not d.allowed


def test_unknown_tool_default_mode():
    checker = PermissionChecker(mode=PermissionMode.DEFAULT)
    d = checker.evaluate("some_new_tool", is_read_only=False)
    assert not d.allowed
    assert d.requires_confirmation


def test_decision_is_frozen():
    d = PermissionDecision(allowed=True)
    with pytest.raises(AttributeError):
        d.allowed = False
