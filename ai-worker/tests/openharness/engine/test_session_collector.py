"""Tests for SessionCollector — per-turn stats observation."""

import pytest

from src.openharness.engine.session_collector import (
    _is_build_like,
    SessionCollector,
)
from src.openharness.engine.stream_events import (
    AssistantTextDelta,
    AssistantTurnComplete,
    ToolExecutionCompleted,
    ToolExecutionStarted,
)


# ---------------------------------------------------------------------------
# _is_build_like
# ---------------------------------------------------------------------------


@pytest.mark.parametrize("command,expected", [
    # Build-like
    ("go build ./...", True),
    ("go test -v", True),
    ("mvn compile", True),
    ("mvn test", True),
    ("gradle build", True),
    ("./gradlew build", True),
    ("npm run build", True),
    ("yarn test", True),
    ("pnpm test", True),
    ("pytest tests/", True),
    ("cargo build", True),
    ("make test", True),
    ("ctest -v", True),
    ("dotnet build", True),
    ("tsc -p .", True),
    ("javac Main.java", True),
    # Not build-like
    ("ls -la", False),
    ("cat README.md", False),
    ("git status", False),
    ("git log --oneline", False),
    ("grep TODO *.go", False),
    ("find . -name '*.txt'", False),
    ("", False),
    ("   ", False),
])
def test_is_build_like(command, expected):
    assert _is_build_like(command) is expected


# ---------------------------------------------------------------------------
# SessionCollector
# ---------------------------------------------------------------------------


def test_fresh_collector_has_zero_counts():
    c = SessionCollector()
    assert c.files_created == 0
    assert c.files_modified == 0
    assert c.tool_call_count == 0
    assert c.last_build_status == "skipped"


def test_collector_counts_write_file():
    c = SessionCollector()
    c.observe(ToolExecutionStarted(
        tool_use_id="t1",
        tool_name="write_file",
        tool_input={"path": "foo.py", "content": "x"},
    ))
    c.observe(ToolExecutionCompleted(
        tool_use_id="t1",
        tool_name="write_file",
        output="Wrote 1 line to foo.py",
        is_error=False,
    ))
    assert c.files_created == 1
    assert c.files_modified == 0
    assert c.tool_call_count == 1


def test_collector_counts_edit_file():
    c = SessionCollector()
    c.observe(ToolExecutionStarted(
        tool_use_id="t1",
        tool_name="edit_file",
        tool_input={"path": "foo.py", "old_string": "a", "new_string": "b"},
    ))
    c.observe(ToolExecutionCompleted(
        tool_use_id="t1",
        tool_name="edit_file",
        output="Replaced in foo.py (+1 -1 lines)",
        is_error=False,
    ))
    assert c.files_modified == 1
    assert c.files_created == 0


def test_collector_does_not_count_failed_writes():
    c = SessionCollector()
    c.observe(ToolExecutionStarted(
        tool_use_id="t1",
        tool_name="write_file",
        tool_input={"path": "x.py", "content": "x"},
    ))
    c.observe(ToolExecutionCompleted(
        tool_use_id="t1",
        tool_name="write_file",
        output="Path escape: absolute path not allowed",
        is_error=True,
    ))
    assert c.files_created == 0
    # But still counts as a tool call attempted
    assert c.tool_call_count == 1


def test_collector_builds_status_from_last_build_command():
    c = SessionCollector()
    # First bash: a non-build-like git status — does NOT update build_status
    c.observe(ToolExecutionStarted(
        tool_use_id="t1",
        tool_name="bash",
        tool_input={"command": "git status"},
    ))
    c.observe(ToolExecutionCompleted(
        tool_use_id="t1",
        tool_name="bash",
        output="clean",
        is_error=False,
    ))
    assert c.last_build_status == "skipped"

    # Second bash: go build, success
    c.observe(ToolExecutionStarted(
        tool_use_id="t2",
        tool_name="bash",
        tool_input={"command": "go build ./..."},
    ))
    c.observe(ToolExecutionCompleted(
        tool_use_id="t2",
        tool_name="bash",
        output="OK",
        is_error=False,
    ))
    assert c.last_build_status == "passed"

    # Third bash: go test, failure — should override passed to failed
    c.observe(ToolExecutionStarted(
        tool_use_id="t3",
        tool_name="bash",
        tool_input={"command": "go test ./..."},
    ))
    c.observe(ToolExecutionCompleted(
        tool_use_id="t3",
        tool_name="bash",
        output="FAIL",
        is_error=True,
    ))
    assert c.last_build_status == "failed"


def test_collector_should_emit_summary_requires_tool_calls():
    c = SessionCollector()
    # No tool calls at all — agent just answered with text
    c.observe(AssistantTextDelta(text="Hello"))
    assert c.should_emit_summary() is False

    # One tool call — summary should emit
    c.observe(ToolExecutionStarted(
        tool_use_id="t1",
        tool_name="read_file",
        tool_input={"path": "foo.py"},
    ))
    c.observe(ToolExecutionCompleted(
        tool_use_id="t1",
        tool_name="read_file",
        output="...",
        is_error=False,
    ))
    assert c.should_emit_summary() is True


def test_collector_correlates_bash_by_tool_use_id():
    """Interleaved tool calls must correlate by tool_use_id, not
    positional ordering. Simulate two overlapping bash calls
    (which can't actually happen in sequential agent loop, but
    the implementation should be robust anyway)."""
    c = SessionCollector()
    c.observe(ToolExecutionStarted(
        tool_use_id="t1",
        tool_name="bash",
        tool_input={"command": "go build"},
    ))
    c.observe(ToolExecutionStarted(
        tool_use_id="t2",
        tool_name="bash",
        tool_input={"command": "ls"},
    ))
    c.observe(ToolExecutionCompleted(
        tool_use_id="t2",  # ls completes first
        tool_name="bash",
        output="files",
        is_error=False,
    ))
    c.observe(ToolExecutionCompleted(
        tool_use_id="t1",  # go build completes second
        tool_name="bash",
        output="OK",
        is_error=False,
    ))
    # Only go build updates build_status (ls is not build-like)
    assert c.last_build_status == "passed"


def test_collector_ignores_non_tool_events():
    c = SessionCollector()
    c.observe(AssistantTextDelta(text="thinking..."))
    assert c.tool_call_count == 0
    assert c.files_created == 0


def test_collector_turn_complete_is_noop():
    """AssistantTurnComplete is observed by QueryEngine for usage
    accumulation; SessionCollector doesn't care about it."""
    from src.openharness.api.usage import UsageSnapshot
    from src.openharness.engine.messages import ConversationMessage, TextBlock

    c = SessionCollector()
    msg = ConversationMessage(role="assistant", content=[TextBlock(text="done")])
    c.observe(AssistantTurnComplete(message=msg, usage=UsageSnapshot()))
    # No state change
    assert c.tool_call_count == 0
    assert c.should_emit_summary() is False
