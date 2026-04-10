"""SessionCollector — per-turn stats tracking for SessionComplete.

QueryEngine.submit_message instantiates one SessionCollector per
turn and calls observe() for each event yielded by the agent loop.
At the end of the turn, should_emit_summary() decides whether to
yield a SessionComplete event (skipped if no tools were called),
and the accumulated fields populate the event.

Correlation between Started and Completed events uses the
tool_use_id field added to both in Phase 4 Task 4.1 — no
positional ordering assumptions.

Spec: §5.6 Session complete logic.
"""

from __future__ import annotations

from typing import Dict

from .stream_events import (
    StreamEvent,
    ToolExecutionCompleted,
    ToolExecutionStarted,
)


# Build/test runner first tokens. A bash command whose first
# whitespace-separated token (after './' stripping) is in this set
# counts as build-like for build_status tracking.
_BUILD_LIKE_FIRST_TOKENS = frozenset({
    "go",         # go build, go test, go vet
    "mvn",        # mvn compile, mvn test
    "gradle",     # gradle build
    "gradlew",    # ./gradlew build (stripped to 'gradlew')
    "npm",        # npm run build, npm test
    "pnpm",
    "yarn",
    "pytest",
    "jest",
    "cargo",      # cargo build, cargo test
    "make",
    "ctest",
    "dotnet",     # dotnet build
    "tsc",        # TypeScript compile
    "javac",
    "rustc",
    "python",     # python -m pytest, python setup.py test
    "python3",
    "ruff",       # linting counts as "build-like" for status purposes
    "eslint",
    "mypy",
    "pyright",
    "gofmt",
    "golint",
    "staticcheck",
})


def _is_build_like(command: str) -> bool:
    """Return True if the first token of `command` is a known build/
    test/lint runner. Used to decide whether a bash call should
    update SessionCollector.last_build_status.
    """
    stripped = command.strip()
    if not stripped:
        return False
    first = stripped.split()[0]
    # './gradlew' -> 'gradlew', './mvnw' -> 'mvnw'
    if "/" in first:
        first = first.rsplit("/", 1)[-1]
    return first in _BUILD_LIKE_FIRST_TOKENS


class SessionCollector:
    """Per-turn stats observer.

    Attributes:
        files_created: Count of successful write_file calls this turn.
        files_modified: Count of successful edit_file calls this turn.
        tool_call_count: Total tool calls attempted (success or error).
        last_build_status: "passed" | "failed" | "skipped". Updated
            by the most recent build-like bash call; "skipped" if no
            build-like bash was called this turn.
    """

    def __init__(self) -> None:
        self.files_created: int = 0
        self.files_modified: int = 0
        self.tool_call_count: int = 0
        self.last_build_status: str = "skipped"
        # tool_use_id -> command, populated on ToolExecutionStarted
        # for bash calls, consumed on ToolExecutionCompleted
        self._pending_bash: Dict[str, str] = {}

    def observe(self, event: StreamEvent) -> None:
        """Update internal state from a single stream event.

        Non-tool events (AssistantTextDelta, AssistantTurnComplete,
        PhaseChanged, ThinkingStarted, etc.) are silently ignored.
        """
        if isinstance(event, ToolExecutionStarted):
            # Stash bash commands so the matching Completed event
            # can look them up by tool_use_id for build-like detection.
            if event.tool_name == "bash":
                command = event.tool_input.get("command", "")
                self._pending_bash[event.tool_use_id] = command
            return

        if isinstance(event, ToolExecutionCompleted):
            self.tool_call_count += 1

            if event.tool_name == "write_file" and not event.is_error:
                self.files_created += 1
            elif event.tool_name == "edit_file" and not event.is_error:
                self.files_modified += 1
            elif event.tool_name == "bash":
                command = self._pending_bash.pop(event.tool_use_id, "")
                if _is_build_like(command):
                    self.last_build_status = (
                        "passed" if not event.is_error else "failed"
                    )
            return

        # All other event types: no-op

    def should_emit_summary(self) -> bool:
        """Whether SessionComplete should be emitted at the end of
        this turn. Returns False if the turn had zero tool calls
        (agent only replied with text) — in that case a SummaryCard
        with '0 files created' would be noise, so we skip it.
        """
        return self.tool_call_count > 0
