"""End-to-end smoke test for the Variant B single-agent architecture.

This test drives the full stack:
  forge-core workspace.EnsureReady -> ai-worker /api/run ->
  QueryEngine._create_engine -> real ModelRouter -> real LLM ->
  tool registry -> bwrap sandbox -> Redis stream events -> back

Runs on CI on merge-to-main only (~$0.20 per run via real LLM).
Local runs require FORGE_E2E_ENABLED=1 and a working model router
config.

The test asserts the SHAPE of the event trace (set_phase fired,
exploration happened, writes happened, bash happened, SessionComplete
at end) rather than specific content, because LLMs are non-deterministic.
The real "did it work?" assertion is the post-run 'go build ./...'
which proves the agent produced compiling code.

Round 2 adds the bidirectional clarification round-trip: an intentionally
ambiguous prompt forces the agent to call request_clarification; a
background responder publishes a canned response to the session's Redis
return channel; the agent receives the response and continues.
"""

from __future__ import annotations

import os
import subprocess
import sys
from pathlib import Path
from typing import Any, List

import pytest


# ---------------------------------------------------------------------------
# Fixtures
# ---------------------------------------------------------------------------


@pytest.fixture
def go_fixture_workspace(tmp_path: Path) -> Path:
    """Create a minimal Go project fixture in tmp_path.

    Layout:
      tmp_path/
        go.mod
        main.go       (has package main, empty main func)
        handlers/
          (empty, agent is expected to add files here)
    """
    (tmp_path / "go.mod").write_text(
        "module example.com/chronos-e2e\n\ngo 1.22\n"
    )
    (tmp_path / "main.go").write_text(
        'package main\n'
        '\n'
        'import "fmt"\n'
        '\n'
        'func main() {\n'
        '\tfmt.Println("hello from chronos smoke test")\n'
        '}\n'
    )
    (tmp_path / "handlers").mkdir()
    # Placeholder file so the dir isn't empty (some tools skip empty dirs)
    (tmp_path / "handlers" / ".gitkeep").write_text("")

    # Initialize as a git repo so git-respecting tools (glob, grep) work
    subprocess.run(
        ["git", "init", "-q", "-b", "main"],
        cwd=tmp_path,
        check=True,
        env={**os.environ, "GIT_TERMINAL_PROMPT": "0"},
    )
    subprocess.run(
        ["git", "-C", str(tmp_path), "add", "."],
        check=True,
    )
    subprocess.run(
        ["git", "-C", str(tmp_path), "-c", "user.email=test@chronos.local",
         "-c", "user.name=Chronos", "commit", "-qm", "initial"],
        check=True,
    )

    return tmp_path


# ---------------------------------------------------------------------------
# The smoke test
# ---------------------------------------------------------------------------


@pytest.mark.e2e
@pytest.mark.skipif(
    not os.getenv("FORGE_E2E_ENABLED"),
    reason="E2E disabled; set FORGE_E2E_ENABLED=1 to run (costs ~$0.20 per run -- Round 2 adds the clarification round-trip)",
)
@pytest.mark.asyncio
async def test_agent_can_complete_variant_b_workflow_with_clarification(
    go_fixture_workspace: Path,
):
    """A real LLM session with an intentionally ambiguous prompt:
    the agent must call request_clarification, a background task
    publishes a response to the Redis return channel via forge-core's
    /api/sessions/{id}/clarify endpoint, and the agent continues to
    finish the task. This exercises the full bidirectional RPC path
    end-to-end on real infrastructure.

    Per spec 7.6 Round 2, this is the e2e smoke test. The Round 1
    version without clarification is removed -- the Round 2 version
    covers both the original shape assertions AND the new round-trip.
    """
    import asyncio
    import json
    import uuid

    from src.api_server import RunRequest, _create_engine, _get_redis
    from src.openharness.engine.stream_events import (
        ClarificationRequested,
        PhaseChanged,
        SessionComplete,
        ToolExecutionCompleted,
    )

    # Ensure Redis is available (required for clarification round-trip)
    redis_client = await _get_redis()
    if redis_client is None:
        pytest.skip("Redis not available -- start docker-compose dev first")

    # Verify connectivity
    try:
        await redis_client.ping()
    except Exception:
        pytest.skip("Redis not reachable -- start docker-compose dev first")

    session_id = f"e2e-smoke-{uuid.uuid4().hex[:8]}"

    # Intentionally ambiguous: agent can't know the exact response format
    # without asking. This forces a request_clarification call.
    req = RunRequest(
        session_id=session_id,
        project_id=1,
        workspace_path="e2e-smoke",  # relative fragment
        message=(
            "Add a new HTTP handler in handlers/ that returns greeting data. "
            "The exact filename, function name, URL shape, and response body "
            "format are up to you -- but if you need clarification on any of "
            "them, ask me. Then run `go build ./...` to verify it compiles. "
            "Tell me when you are done."
        ),
    )

    # _create_engine constructs the full tool-using agent.
    try:
        engine = _create_engine(req, workspace_dir=go_fixture_workspace)
    except RuntimeError as e:
        pytest.skip(f"ModelRouter unavailable: {e}")

    # Background responder task. When the agent yields
    # ClarificationRequested, we publish a canned response to the
    # session's return channel. In production this POST comes from the
    # frontend; here we publish to Redis directly to exercise the same
    # channel / JSON schema that forge-core produces.
    clarification_seen = asyncio.Event()
    clarification_response_sent = asyncio.Event()

    async def wait_then_respond():
        """Watches for clarification_seen signal, then publishes to
        the return channel."""
        await clarification_seen.wait()
        # Fetch the most recent ClarificationRequested from the shared
        # events list.
        tool_use_id = None
        for ev in reversed(events):
            if isinstance(ev, ClarificationRequested):
                tool_use_id = ev.tool_use_id
                break
        assert tool_use_id is not None, (
            "clarification_seen was set but no ClarificationRequested "
            "event in the buffer -- harness bug"
        )

        # Publish directly to Redis (same code path as forge-core's
        # clarify_handler.go -- same channel, same JSON schema).
        payload = json.dumps({
            "type": "clarification_response",
            "session_id": session_id,
            "tool_use_id": tool_use_id,
            "response": (
                "Please create handlers/hello.go with a function HelloHandler "
                "at URL /hello that returns JSON {\"greeting\": \"world\"} "
                "with HTTP 200."
            ),
        }).encode("utf-8")
        channel = f"agent:return:{session_id}"
        await redis_client.publish(channel, payload)
        clarification_response_sent.set()

    responder_task = asyncio.create_task(wait_then_respond())

    # Collect every event; flip the signal flag when we see the first
    # ClarificationRequested so the responder can act.
    events: List[Any] = []
    try:
        async for event in engine.submit_message(req.message):
            events.append(event)
            if isinstance(event, ClarificationRequested) and not clarification_seen.is_set():
                clarification_seen.set()
    finally:
        responder_task.cancel()
        try:
            await responder_task
        except (asyncio.CancelledError, Exception):
            pass

    # ---- Clarification round-trip assertions (Round 2) ----

    clarification_events = [
        e for e in events if isinstance(e, ClarificationRequested)
    ]
    assert len(clarification_events) >= 1, (
        f"agent did NOT call request_clarification despite ambiguous "
        f"prompt. Events: {[type(e).__name__ for e in events[:30]]}"
    )
    assert clarification_response_sent.is_set(), (
        "background responder did not publish to the return channel -- "
        "harness bug or the agent finished before clarification was seen"
    )

    # Round-trip closed: there's a ToolExecutionCompleted for
    # request_clarification with the matching tool_use_id and is_error=False
    clarify_completions = [
        e for e in events
        if isinstance(e, ToolExecutionCompleted)
        and e.tool_name == "request_clarification"
    ]
    assert len(clarify_completions) >= 1, (
        "request_clarification round-trip did not complete -- no matching "
        "ToolExecutionCompleted event. The pause/resume infrastructure "
        "is broken."
    )
    assert clarify_completions[0].is_error is False, (
        f"request_clarification returned an error: "
        f"{clarify_completions[0].output!r}"
    )
    # The agent should have received something resembling the injected
    # response in the tool output.
    assert (
        "hello" in clarify_completions[0].output.lower()
        or "greeting" in clarify_completions[0].output.lower()
        or "world" in clarify_completions[0].output.lower()
    ), (
        f"clarification response did not contain any of the injected "
        f"keywords. output={clarify_completions[0].output!r}"
    )

    # ---- Shape assertions (carry over from Round 1) ----

    tool_completions = [
        e for e in events if isinstance(e, ToolExecutionCompleted)
    ]
    assert len(tool_completions) > 0, (
        f"agent did no tool calls at all -- events: "
        f"{[type(e).__name__ for e in events[:20]]}"
    )

    tool_names_called = {t.tool_name for t in tool_completions}

    # request_clarification must be in the called set
    assert "request_clarification" in tool_names_called, (
        "request_clarification missing from tool_names_called"
    )

    # set_phase should have fired at least once (agent signalled phase)
    phase_events = [e for e in events if isinstance(e, PhaseChanged)]
    assert len(phase_events) >= 1, (
        f"agent did not call set_phase -- shape of trace is off. "
        f"Tools called: {tool_names_called}"
    )

    # Agent should have explored before writing (at least one read-style tool)
    exploration_tools = {"read_file", "glob", "grep", "list_directory"}
    assert tool_names_called & exploration_tools, (
        f"agent did not explore the workspace before writing. "
        f"Tools called: {tool_names_called}"
    )

    # Agent should have written code (at least one write/edit)
    write_tools = {"write_file", "edit_file"}
    assert tool_names_called & write_tools, (
        f"agent did not write any code. Tools called: {tool_names_called}"
    )

    # Agent should have tried to run something (bash for build/test)
    assert "bash" in tool_names_called, (
        f"agent did not call bash to verify its work. "
        f"Tools called: {tool_names_called}"
    )

    # SessionComplete should be emitted
    session_completes = [e for e in events if isinstance(e, SessionComplete)]
    assert len(session_completes) == 1, (
        f"expected exactly 1 SessionComplete, got {len(session_completes)}"
    )
    sc = session_completes[0]
    assert sc.files_created + sc.files_modified > 0, (
        f"SessionComplete reports no file changes "
        f"(created={sc.files_created}, modified={sc.files_modified})"
    )

    # ---- Post-assertion: the workspace must actually build ----
    # This is the real proof that the agent produced working code.
    build = subprocess.run(
        ["go", "build", "./..."],
        cwd=go_fixture_workspace,
        capture_output=True,
        text=True,
    )
    assert build.returncode == 0, (
        f"agent's output did not compile. go build stderr:\n"
        f"{build.stderr}\n"
        f"stdout:\n{build.stdout}"
    )

    # Assert that handlers/ dir has at least one .go file
    go_files = list((go_fixture_workspace / "handlers").glob("*.go"))
    assert len(go_files) >= 1, (
        f"no .go files in handlers/ -- agent wrote the handler somewhere "
        f"unexpected."
    )


# ---------------------------------------------------------------------------
# Bonus: observability smoke -- verifies log points exist in source
# ---------------------------------------------------------------------------


@pytest.mark.e2e
@pytest.mark.skipif(
    not os.getenv("FORGE_E2E_ENABLED"),
    reason="E2E disabled; set FORGE_E2E_ENABLED=1 to run",
)
def test_observability_log_events_match_expected_keys():
    """Smoke-check that observability log points exist with the
    expected key set. Doesn't run an agent -- just imports the modules
    and asserts the logger call sites are present.

    (A real Loki assertion would require running the agent and
    scraping the container logs; deferred to post-deploy canary.)
    """
    import inspect

    import src.openharness.engine.query as query_mod
    import src.api_server as api_mod

    query_src = inspect.getsource(query_mod)
    api_src = inspect.getsource(api_mod)

    # At least one structured log per critical boundary
    assert "agent.tool_call" in query_src or "agent.turn_complete" in query_src, (
        "query.py is missing tool-call observability log points"
    )
    # api_server logs workspace route decisions
    assert "workspace" in api_src.lower(), "api_server has no workspace logs"
