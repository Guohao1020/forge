# chronos · Phase 7 — E2E Smoke + Deploy

> **Project:** [chronos — Agent Variant B Single-Agent Implementation](index.md)
> **Phase:** 7 of 9 (Round 2) · **Tasks:** 4 · **Depends on:** [Phase 6](phase-6-frontend.md) + [Phase 5a](phase-5a-bidirectional-rpc.md) · **Unblocks:** production deployment
> **Spec reference:** [Design spec §7.6 (E2E smoke test — Round 2 includes clarification round-trip) + §8 (Deployment and rollout) + §2.9.2 (bidirectional RPC, now exercised by the smoke test)](../../specs/2026-04-09-agent-variant-b-single-agent-design.md)

**Execution:** REQUIRED SUB-SKILL: Use `superpowers:subagent-driven-development` (recommended) or `superpowers:executing-plans`. Steps use checkbox (`- [ ]`) syntax for tracking.

---

## Phase goal

Land the final three things that take chronos from "all code written and unit-tested" to "running in production":

1. **Real-LLM E2E smoke test** (`test_variant_b_smoke.py`) — drives the full stack end-to-end: forge-core → ai-worker → bwrap → real LLM → SSE → observable stream events **including a bidirectional clarification round-trip** (spec §2.9.2 / §7.6 Round 2). An intentionally ambiguous user prompt forces the agent to call `request_clarification`; a background responder publishes a canned response to the session's Redis return channel; the agent receives the response and continues. Runs on CI on merge to main (~$0.20 per run — the clarification adds one extra LLM turn vs Round 1's ~$0.10). Asserts the shape of the event trace plus the round-trip closure (matching `tool_use_id` between `ClarificationRequested` and `ToolExecutionCompleted`).
2. **Observability log points** — structured JSON logs at every critical boundary (workspace lifecycle, tool calls, agent turn complete, bash denylist hits). No dashboards, no alerts — just the raw data in Loki so post-deploy debugging has signal.
3. **Deployment runbook** — the three-step deploy (schema + image rebuild → new code + smoke → delete legacy) as an executable checklist, plus a post-deploy verification checklist.
4. **Session memory + retro** — save a chronos project memory file documenting what landed, what was deferred, what the next team/session needs to know.

**Completion gate:**
- `pytest ai-worker/tests/e2e/test_variant_b_smoke.py -m e2e` completes successfully with `FORGE_E2E_ENABLED=1` (skips otherwise)
- `grep -c "agent_event\|agent_tool_call\|agent_turn_complete\|workspace_ensure_ready\|bash_denylist_hit" ai-worker/src/ forge-core/internal/ 2>/dev/null` ≥ 10 matches (observability is wired)
- `docs/plans/chronos-2026-04-09/deploy-runbook.md` exists with the three-step procedure and post-deploy verification checklist
- `~/.claude/projects/D--shulex-work-forge/memory/chronos-delivery-2026-04-09.md` exists with a session summary
- Manual smoke (if Harvey is shepherding the deploy himself): go through the runbook steps one by one, watching for errors
- Branch has 4 new commits from this phase

## Why this phase matters

Phase 0–6 shipped the parts and wired them together. Phase 7 **proves it works against a real LLM** and **writes down how to ship it**. Without this phase:

- A latent bug in the cross-service handoffs (forge-core → ai-worker workspace_path, Redis stream ordering, SSE reconnect) wouldn't be caught until a real user hit it
- There's no observability, so the first production bug is a game of print-statement whack-a-mole
- The deploy procedure lives in Harvey's head, which is fine if Harvey deploys it but breaks the moment he hands off or the moment he forgets a step in 3 weeks
- Future sessions in this codebase would have to re-derive everything from the plan files, which takes an hour; a project memory file takes 5 minutes to write and 30 seconds to read

**Silicon-valley rules for this phase:**
- **The e2e smoke test uses a REAL LLM call** — not a mock. The spec §7.6 explicitly rejects mock-LLM e2e because mocks hide precisely the failures e2e is meant to catch (prompt format mismatches, tool schema serialization bugs, streaming parse issues). ~$0.10 per run is cheap.
- **Observability is structured JSON from day one** — not printf. Every log point is a `logger.info("event_name", extra={...})` call with a stable key set. Loki can query on those keys; grep can't query on "some string with variables embedded".
- **Deploy runbook is executable, not descriptive.** Every step has a copy-pasteable command and a pass/fail check. No prose like "update the database". It says: `PGPASSWORD=... psql -h localhost -d forge -f forge-core/migrations/025_workspaces.sql` and then `psql -c "\d engine.workspaces"` to verify.

---

### Task 7.1: Real-LLM E2E smoke test (Round 2 — includes clarification round-trip)

**Files:**
- Create: `ai-worker/tests/e2e/test_variant_b_smoke.py`
- Modify: `ai-worker/tests/conftest.py` (add `e2e` marker registration if not present; also add a `redis_client` pytest fixture that connects to the docker-compose dev Redis and skips the test on connection failure)
- Modify: `ai-worker/pytest.ini` or `pyproject.toml` (register the `e2e` marker)

**Context:** A single test function that drives the full stack **including the Round 2 bidirectional clarification round-trip**. Skipped by default (`FORGE_E2E_ENABLED` env check) so CI on regular PRs doesn't pay the LLM cost; runs on merge to main via a separate workflow. Also requires a running Redis (docker-compose dev instance) — the test skips cleanly if Redis is unreachable.

The test:
1. Sets up a small fixture Go project in a temp workspace
2. Constructs a `QueryEngine` via `await _create_engine(...)` (Phase 5 Task 5.15 updated this to be async and accept a `redis_client` argument)
3. Submits an **intentionally ambiguous** prompt designed to force the agent to call `request_clarification` (e.g. "Add an HTTP handler that returns greeting data — exact filename, function name, URL shape, and response body format are up to you, but if you need clarification on any of them, ask me")
4. Spawns a background responder task that watches the event stream for a `ClarificationRequested` event and then publishes a canned response to the session's return channel (`agent:return:{session_id}`) via Redis. This simulates what forge-core's `/api/sessions/{id}/clarify` handler does in production.
5. Collects every event from the stream into a list
6. Asserts on the Round 2 clarification round-trip:
   - `ClarificationRequested` event appears (agent called `request_clarification`)
   - The responder task successfully published to the return channel
   - A matching `ToolExecutionCompleted` event with `tool_name=="request_clarification"` and matching `tool_use_id` appears in the stream, with `is_error=False`
   - The tool's output contains keywords from the injected response (defense-in-depth check that the response actually round-tripped)
7. Asserts on the SHAPE of the event trace (carry over from Round 1):
   - `PhaseChanged` events appear (agent called set_phase)
   - At least one of `read_file` / `glob` / `grep` / `list_directory` was called (agent explored before writing)
   - At least one of `write_file` / `edit_file` was called (agent made changes)
   - At least one `bash` call was made (agent verified with a build or test)
   - `SessionComplete` is the last event (or close to it)
8. **Post-assertion**: actually build the resulting workspace with `go build ./...` — the code the agent wrote must actually compile. This is the real test.

Why shape-only assertions: real LLMs are non-deterministic. Asserting that `bash` was called is meaningful; asserting that the agent called `glob "*.go"` specifically is flaky. The post-assertion (`go build` succeeds) is what proves the outcome.

**Round 2 cost:** ~$0.20 per run (up from Round 1's ~$0.10). The extra LLM turn is the agent processing the clarification response and continuing.

**Round 2 test harness note:** The background responder publishes directly to Redis via `redis_client.publish(channel, payload)` rather than going through forge-core's HTTP endpoint. This is deliberate — it exercises the same Redis channel / JSON schema that forge-core produces, without requiring the Go binary to be running during the Python test. The forge-core endpoint is tested separately in Phase 5a Task 5a.6 via Go unit tests against a mocked Redis client. The full "frontend → forge-core HTTP → Redis publish → ai-worker subscriber" path is exercised by the Phase 6 frontend tests + this smoke test's Redis publish step together.

- [ ] **Step 1: Find the existing e2e test infrastructure if any**

```bash
ls ai-worker/tests/e2e/
grep -n "markers\|e2e" ai-worker/pytest.ini ai-worker/pyproject.toml 2>/dev/null
```
Expected: `tests/e2e/` directory exists from earlier streams; may already have a pytest marker registered. If not, add it in Step 2.

- [ ] **Step 2: Register the e2e marker**

Check if the `e2e` marker is registered in `ai-worker/pytest.ini` or `ai-worker/pyproject.toml`. If not, add to `pytest.ini`:

```ini
[pytest]
markers =
    e2e: end-to-end tests requiring real LLM and workspace (slow, costly)
```

Or to `pyproject.toml`:

```toml
[tool.pytest.ini_options]
markers = [
    "e2e: end-to-end tests requiring real LLM and workspace (slow, costly)",
]
```

- [ ] **Step 3: Create the e2e test file**

Create `ai-worker/tests/e2e/test_variant_b_smoke.py`:

```python
"""End-to-end smoke test for the Variant B single-agent architecture.

This test drives the full stack:
  forge-core workspace.EnsureReady → ai-worker /api/run →
  QueryEngine._create_engine → real ModelRouter → real LLM →
  tool registry → bwrap sandbox → Redis stream events → back

Runs on CI on merge-to-main only (~$0.10 per run via real LLM).
Local runs require FORGE_E2E_ENABLED=1 and a working model router
config.

The test asserts the SHAPE of the event trace (set_phase fired,
exploration happened, writes happened, bash happened, SessionComplete
at end) rather than specific content, because LLMs are non-deterministic.
The real "did it work?" assertion is the post-run 'go build ./...'
which proves the agent produced compiling code.
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

    # Initialize as a git repo so git-respecting tools (glob, grep) work sanely
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
    reason="E2E disabled; set FORGE_E2E_ENABLED=1 to run (costs ~$0.20 per run — Round 2 adds the clarification round-trip)",
)
@pytest.mark.asyncio
async def test_agent_can_complete_variant_b_workflow_with_clarification(
    go_fixture_workspace: Path,
    redis_client,  # pytest fixture wired to docker-compose dev Redis
):
    """A real LLM session with an intentionally ambiguous prompt:
    the agent must call request_clarification, a background task
    publishes a response to the Redis return channel via forge-core's
    /api/sessions/{id}/clarify endpoint, and the agent continues to
    finish the task. This exercises the full bidirectional RPC path
    end-to-end on real infrastructure.

    Per spec §7.6 Round 2, this is the e2e smoke test. The Round 1
    version without clarification is removed — the Round 2 version
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
            "format are up to you — but if you need clarification on any of "
            "them, ask me. Then run `go build ./...` to verify it compiles. "
            "Tell me when you are done."
        ),
    )

    # _create_engine is async in Round 2 (Phase 5a Task 5a.9 added
    # the await + redis_client parameter). Pass the real Redis client
    # so the ReturnChannel subscriber comes up.
    redis_client_for_engine = await _get_redis()
    assert redis_client_for_engine is not None, (
        "Redis is required for the e2e smoke test — start docker-compose "
        "dev first. No fallback (spec §2.8)."
    )
    engine = await _create_engine(
        req,
        workspace_dir=go_fixture_workspace,
        redis_client=redis_client_for_engine,
    )

    # Background responder task. When the agent yields
    # ClarificationRequested, we publish a canned response to the
    # session's return channel. In production this POST comes from the
    # frontend; here we use forge-core's /api/sessions/{id}/clarify
    # endpoint directly to exercise the real HTTP path, OR publish to
    # Redis directly if a forge-core HTTP client isn't wired into the
    # test harness.
    clarification_seen = asyncio.Event()
    clarification_response_sent = asyncio.Event()

    async def wait_then_respond():
        """Watches the forward event stream indirectly — actually we
        just sleep briefly and then publish to the return channel when
        we see the agent is in a waiting state. The real mechanism is
        the listener sees the ClarificationRequested event in the main
        task's `events` list and signals clarification_seen."""
        await clarification_seen.wait()
        # Fetch the most recent ClarificationRequested from the shared
        # events list. The main task sets clarification_seen before any
        # pending wait, so there's always at least one event by now.
        tool_use_id = None
        for ev in reversed(events):
            if isinstance(ev, ClarificationRequested):
                tool_use_id = ev.tool_use_id
                break
        assert tool_use_id is not None, (
            "clarification_seen was set but no ClarificationRequested "
            "event in the buffer — harness bug"
        )

        # Publish directly to Redis (simulating forge-core's /clarify
        # endpoint publish). This is the same code path forge-core uses
        # in clarify_handler.go — same channel, same JSON schema.
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
        "background responder did not publish to the return channel — "
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
        "request_clarification round-trip did not complete — no matching "
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
        f"agent did no tool calls at all — events: "
        f"{[type(e).__name__ for e in events[:20]]}"
    )

    tool_names_called = {t.tool_name for t in tool_completions}

    # request_clarification must be in the called set (enforced above,
    # this is a belt-and-suspenders for readability)
    assert "request_clarification" in tool_names_called, (
        "request_clarification missing from tool_names_called — "
        "contradicts the earlier clarification_events check"
    )

    # set_phase should have fired at least once (agent signalled phase)
    phase_events = [e for e in events if isinstance(e, PhaseChanged)]
    assert len(phase_events) >= 1, (
        f"agent did not call set_phase — shape of trace is off. "
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

    # SessionComplete should be emitted (tool_call_count > 0 → collector
    # fires the summary at the end of the turn)
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

    # Optional: assert that handlers/hello.go was actually created.
    # Don't hard-assert the exact filename because the agent might
    # choose handlers/http_hello.go or similar — just check the
    # handlers/ dir has something Go-ish in it.
    go_files = list((go_fixture_workspace / "handlers").glob("*.go"))
    assert len(go_files) >= 1, (
        f"no .go files in handlers/ — agent wrote the handler somewhere "
        f"unexpected. Workspace layout:\n"
        f"{subprocess.run(['find', str(go_fixture_workspace), '-name', '*.go'], capture_output=True, text=True).stdout}"
    )


# ---------------------------------------------------------------------------
# Bonus: observability smoke — parses Loki-shaped logs from the agent run
# ---------------------------------------------------------------------------


@pytest.mark.e2e
@pytest.mark.skipif(
    not os.getenv("FORGE_E2E_ENABLED"),
    reason="E2E disabled; set FORGE_E2E_ENABLED=1 to run",
)
def test_observability_log_events_match_expected_keys():
    """Smoke-check that observability log points exist with the
    expected key set. Doesn't run an agent — just imports the modules
    and asserts the logger call sites are present.

    (A real Loki assertion would require running the agent and
    scraping the container logs; deferred to post-deploy canary.)
    """
    import src.api_server  # noqa: F401
    import src.openharness.engine.query  # noqa: F401

    # Grep the imported source for structured log points. This is a
    # weak test — it just catches accidental deletion of log points
    # during refactors. A full observability test lives in the
    # post-deploy canary procedure (runbook Step 7).
    import inspect
    import src.openharness.engine.query as query_mod
    import src.api_server as api_mod

    query_src = inspect.getsource(query_mod)
    api_src = inspect.getsource(api_mod)

    # At least one structured log per critical boundary
    assert "agent_event" in query_src or "tool_call" in query_src, (
        "query.py is missing tool-call observability log points"
    )
    # api_server logs workspace route decisions
    assert "workspace" in api_src.lower(), "api_server has no workspace logs"
```

- [ ] **Step 4: Register the test is discoverable**

```bash
cd ai-worker && python -m pytest tests/e2e/test_variant_b_smoke.py --collect-only 2>&1 | tail -10
```
Expected: pytest finds the tests and shows them as "skipped" (because `FORGE_E2E_ENABLED` isn't set in the dev environment).

- [ ] **Step 5: Dry-run the test with mocks (optional)**

If you want to validate the test structure without paying for an LLM call, temporarily mock `_create_engine` to return a stub. But don't commit that mock — delete it after the dry run. The committed test is real-LLM only.

- [ ] **Step 6: Commit**

```bash
git add ai-worker/tests/e2e/test_variant_b_smoke.py ai-worker/pytest.ini ai-worker/pyproject.toml 2>/dev/null
git commit -m "test(e2e): real-LLM smoke test for Variant B single-agent

test_agent_can_complete_variant_b_workflow drives the full stack
end-to-end:
  RunRequest -> _create_engine -> real ModelRouter -> real LLM ->
  tool registry -> bwrap sandbox -> stream events -> back

The test seeds a minimal Go project fixture in a tmp_path (main.go
+ empty handlers/ dir + git init), asks the agent to add a
HelloHandler to handlers/, and then:

Shape assertions (non-deterministic-safe):
- at least one tool completion
- at least one set_phase (PhaseChanged event)
- at least one read/glob/grep/list_directory (exploration)
- at least one write_file or edit_file
- at least one bash call
- exactly one SessionComplete at the end with >0 file changes

Post-assertion (the real proof):
- 'go build ./...' on the resulting workspace succeeds

Shape-only assertions handle LLM non-determinism. 'go build'
passing is what proves the agent actually worked.

Second test test_observability_log_events_match_expected_keys
grep-inspects the imported modules for critical log points.
Weak check, but catches accidental deletion during refactors.

Marked @pytest.mark.e2e and skipped unless FORGE_E2E_ENABLED=1.
CI runs this on merge-to-main only (~\$0.10 per run)."
```

---

### Task 7.2: Observability log points at critical boundaries

**Files:**
- Modify: `ai-worker/src/openharness/engine/query.py` — add `agent.tool_call` and `agent.turn_complete` log points
- Modify: `ai-worker/src/openharness/tools/bash_tool.py` — add `agent.bash_denylist_hit` log point
- Modify: `forge-core/internal/workspace/ensure.go` — add `workspace.ensure_ready` log point
- Modify: `forge-core/internal/module/agent/service.go` — add `agent.session_start` log point

**Context:** Spec §7.8 lists the events to log. Each is a single structured log call with a stable key set. No dashboards, no alerts — just emit the data to stdout (Python) or slog (Go) and let the existing Loki/Grafana scraping pick it up. The goal is to give post-deploy debugging a signal — "no tool_call events for session X" is diagnosable; "the agent did nothing" is not.

Log points:

| Location | Event name | Keys |
|---|---|---|
| `query.py:_execute_tool_call` end | `agent.tool_call` | session_id, correlation_id, tool_name, tool_use_id, duration_ms, is_error, input_size, output_size |
| `query.py:run_agent_loop` on end_turn | `agent.turn_complete` | session_id, turn_count, tool_count, total_tokens |
| `bash_tool.py:_intent_denylist_check` on hit | `agent.bash_denylist_hit` | reason, command_prefix |
| `ensure.go:EnsureReady` on each state transition | `workspace.ensure_ready` | tenant_id, project_id, result (cloned/resynced/error), duration_ms |
| `agent/service.go:SubmitMessage` start | `agent.session_start` | session_id, tenant_id, project_id, workspace_path |

- [ ] **Step 1: Add `agent.tool_call` log point in `query.py`**

At the end of `_execute_tool_call` (after the ToolResultBlock yield, before `return`), add:

```python
# Structured observability — emitted after every tool call, success
# or error. Post-deploy debugging can query Loki for specific
# session_ids / tool_names.
logger.info(
    "agent.tool_call",
    extra={
        "event": "agent.tool_call",
        "tool_name": tool_name,
        "tool_use_id": tool_use_id,
        "is_error": tool_result.is_error,
        "input_size_bytes": len(str(tool_input).encode("utf-8")),
        "output_size_bytes": len(tool_result.output.encode("utf-8")),
    },
)
```

The `session_id` and `correlation_id` aren't in scope here (they're in `api_server._run_and_publish`). That's OK — the `_serialize_event` call in `_run_and_publish` logs them separately, and Loki queries can join by timestamp. Don't plumb session_id through the agent loop just for logging; it creates coupling that isn't worth it.

- [ ] **Step 2: Add `agent.turn_complete` log point in `query.py`**

At the end of `run_agent_loop` (where stop_reason == "end_turn" returns), add:

```python
if not tool_uses or stop_reason == "end_turn":
    logger.info(
        "agent.turn_complete",
        extra={
            "event": "agent.turn_complete",
            "turn_count": turn,
            "stop_reason": stop_reason,
        },
    )
    return
```

- [ ] **Step 3: Add `agent.bash_denylist_hit` log point in `bash_tool.py`**

In the `execute` method of `BashTool`, inside the denylist-rejection branch:

```python
reason = _intent_denylist_check(arguments.command)
if reason:
    # Observability — how often does the agent hit the denylist
    # with which commands? Helps tune the denylist over time.
    logger.info(
        "agent.bash_denylist_hit",
        extra={
            "event": "agent.bash_denylist_hit",
            "reason": reason,
            "command_prefix": arguments.command[:60],
        },
    )
    yield ToolResult(
        is_error=True,
        output=(
            f"Command rejected: {reason}\n\n"
            f"$ {arguments.command}"
        ),
    )
    return
```

Make sure `logger` is imported at the top of `bash_tool.py`:

```python
import logging
logger = logging.getLogger(__name__)
```

- [ ] **Step 4: Add `workspace.ensure_ready` log point in `ensure.go`**

In the workspace `EnsureReady` method, at each terminal state (ready, error), add a `slog.Info` call:

```go
// At the end of a successful fresh install
slog.Info("workspace.ensure_ready",
    "event", "workspace.ensure_ready",
    "tenant_id", tenantID,
    "project_id", projectID,
    "result", "cloned",
    "duration_ms", time.Since(startTime).Milliseconds(),
)

// At the end of a successful resync
slog.Info("workspace.ensure_ready",
    "event", "workspace.ensure_ready",
    "tenant_id", tenantID,
    "project_id", projectID,
    "result", "resynced",
    "duration_ms", time.Since(startTime).Milliseconds(),
)

// At each error marking
slog.Error("workspace.ensure_ready",
    "event", "workspace.ensure_ready",
    "tenant_id", tenantID,
    "project_id", projectID,
    "result", "error",
    "reason", reason,
)
```

You'll need to add `startTime := time.Now()` at the start of `EnsureReady` to compute duration.

- [ ] **Step 5: Add `agent.session_start` log point in `agent/service.go`**

At the start of `SubmitMessage`, after any guards:

```go
slog.Info("agent.session_start",
    "event", "agent.session_start",
    "session_id", req.SessionID,
    "tenant_id", tenantID,
    "project_id", projectID,
    "workspace_path", body.WorkspacePath,  // may be empty if workspace not ready
)
```

- [ ] **Step 6: Smoke check the log points compile and run**

```bash
cd ai-worker && python -c "from src.openharness.engine import query; from src.openharness.tools import bash_tool; print('ok')"
cd forge-core && go build ./... && echo ok
```
Expected: both `ok` prints.

- [ ] **Step 7: Commit**

```bash
git add ai-worker/src/openharness/engine/query.py ai-worker/src/openharness/tools/bash_tool.py forge-core/internal/workspace/ensure.go forge-core/internal/module/agent/service.go
git commit -m "feat(observability): structured log points at critical boundaries

Spec §7.8 observability minimum: structured JSON logs emitted at
every critical handoff so post-deploy debugging has signal. No
dashboards, no alerts — just the raw data.

Five log points added:

Python (ai-worker):
- query.py _execute_tool_call — 'agent.tool_call' event per tool
  call with tool_name, tool_use_id, is_error, io sizes
- query.py run_agent_loop — 'agent.turn_complete' on end_turn
  with turn_count and stop_reason
- bash_tool.py execute — 'agent.bash_denylist_hit' when the
  denylist rejects a command, with reason and command prefix.
  Helps tune the denylist over time.

Go (forge-core):
- workspace/ensure.go — 'workspace.ensure_ready' at each terminal
  state (cloned/resynced/error) with tenant+project+duration
- agent/service.go — 'agent.session_start' at the start of
  SubmitMessage with session/tenant/project/workspace_path

session_id is NOT plumbed through the agent loop — it's in scope
at api_server._run_and_publish and _serialize_event, so Loki
queries can join those log lines by timestamp rather than
coupling the agent loop to ambient session state.

This is the minimum viable observability for Phase 7. Dashboards
and alerts are deferred — the runbook post-deploy verification
(Task 7.3) uses these log points to confirm everything is wired."
```

---

### Task 7.3: Deploy runbook

**Files:**
- Create: `docs/plans/chronos-2026-04-09/deploy-runbook.md`

**Context:** The three-step deploy (spec §8) written as an executable checklist with copy-pasteable commands and pass/fail checks at every step. Also covers the post-deploy verification and the rollback procedure.

- [ ] **Step 1: Write the runbook**

Create `docs/plans/chronos-2026-04-09/deploy-runbook.md`:

````markdown
# chronos Deploy Runbook

> **When:** after all Phase 0–6 code has landed on `feat/agent-variant-b-single-agent` and CI is green
> **Who:** Harvey (or whoever is shepherding the deploy)
> **Rollback:** `git reset --hard <pre-deploy-sha>` + `docker-compose restart`
> **Estimated time:** 15–30 minutes for the deploy, +5 minutes for verification

---

## Pre-deploy checklist

- [ ] All Phase 0–6 completion checks are green (revisit each phase file's "Phase N completion check" section)
- [ ] `git status` in the repo is clean
- [ ] Capture the current deployed SHA for rollback:
  ```bash
  PRE_DEPLOY_SHA=$(git rev-parse HEAD)
  echo "$PRE_DEPLOY_SHA" > /tmp/chronos-rollback-sha
  ```
- [ ] Confirm `FORGE_SECRETS_MASTER_KEY` is set in the forge-core deployment environment (Phase 0 Task 0.6 introduced this env var — missing it makes workspace module fall back to legacy mode with a loud warning)
- [ ] Confirm `docker-compose.dev.yml` (or prod equivalent) has the ai-worker container configured with the Phase 0 Dockerfile changes (bubblewrap + ripgrep)
- [ ] Local `docker compose build forge-ai-worker` succeeds and the built image has `bwrap` + `rg` on PATH (verify: `docker run --rm <image> bash -c 'bwrap --version && rg --version'`)

## Step 1 — Database migrations + image rebuild

1. **Apply migration 025 (workspaces table):**
   ```bash
   docker compose -f docker-compose.dev.yml exec -T postgres \
     psql -U forge -d forge -f - < forge-core/migrations/025_workspaces.sql
   ```
   **Verify:**
   ```bash
   docker compose -f docker-compose.dev.yml exec -T postgres \
     psql -U forge -d forge -c "\d engine.workspaces"
   ```
   Expected: 10 columns, status CHECK constraint (`pending|ready|error`).

2. **Apply migration 026 (project_deploy_keys table):**
   ```bash
   docker compose -f docker-compose.dev.yml exec -T postgres \
     psql -U forge -d forge -f - < forge-core/migrations/026_project_deploy_keys.sql
   ```
   **Verify:**
   ```bash
   docker compose -f docker-compose.dev.yml exec -T postgres \
     psql -U forge -d forge -c "\d engine.project_deploy_keys"
   ```

3. **Cleanup stale agent_messages rows** (spec §2.6 "no backward compatibility"):
   ```bash
   docker compose -f docker-compose.dev.yml exec -T postgres \
     psql -U forge -d forge -c "DELETE FROM engine.agent_messages WHERE event_type IN ('fix_loop_started', 'fix_loop_completed');"
   ```
   Or for a full wipe (if the team prefers a clean slate):
   ```bash
   docker compose -f docker-compose.dev.yml exec -T postgres \
     psql -U forge -d forge -c "TRUNCATE engine.agent_messages;"
   ```

4. **Rebuild ai-worker image:**
   ```bash
   docker compose -f docker-compose.dev.yml build forge-ai-worker
   ```

5. **Verify bubblewrap and ripgrep are in the built image:**
   ```bash
   docker compose -f docker-compose.dev.yml run --rm forge-ai-worker \
     bash -c 'bwrap --version && rg --version'
   ```
   Expected: both binaries print version strings. If either fails, debug the Dockerfile before continuing.

6. **Build forge-core binary:**
   ```bash
   cd forge-core && go build ./cmd/forge-core
   ```
   Expected: clean build, no errors. Binary at `forge-core/forge-core`.

## Step 2 — Deploy new code and smoke-test

1. **Bring up the new stack:**
   ```bash
   docker compose -f docker-compose.dev.yml up -d
   ```
   Wait ~10 seconds for containers to settle.

2. **Confirm ai-worker is responsive:**
   ```bash
   curl -sf http://localhost:8090/health | jq
   ```
   Expected: `{"status":"ok","sessions":0,"version":"1.0.0"}` or similar.

3. **Confirm forge-core is responsive:**
   ```bash
   curl -sf http://localhost:8080/health | jq
   ```
   Expected: some ok response.

4. **Smoke-test the workspace prep endpoint** (Phase 5 Task 5.7):
   ```bash
   curl -sf -X POST http://localhost:8090/api/workspace/prep \
     -H "Content-Type: application/json" \
     -d '{"tenant_id":1,"project_id":1,"workspace_path":"nonexistent"}' | jq
   ```
   Expected: `{"status":"error","error":"workspace directory does not exist: ..."}`. This confirms the endpoint is wired and responds in the expected shape.

5. **Full E2E smoke** (optional but recommended):
   ```bash
   cd ai-worker && FORGE_E2E_ENABLED=1 python -m pytest tests/e2e/test_variant_b_smoke.py -v
   ```
   Expected: test passes in ~2 minutes, costs ~$0.10 via real LLM call. If the test fails:
   - Read the stderr carefully — it prints which shape assertion failed
   - Check the ai-worker container logs: `docker compose logs forge-ai-worker --tail 200`
   - Check the Redis stream: `docker compose exec redis redis-cli XLEN agent:stream:e2e-smoke-1`

6. **Manual smoke via the browser:**
   - Open `http://localhost:3000` (or wherever forge-portal is served)
   - Navigate to a project with a real GitHub repo configured
   - Click into the agent page
   - Send a small message: "What files are in the src/ directory?"
   - Expected UI behavior:
     - Step ribbon shows 7 phases in "pending" state initially
     - SSE stream arrives; at least one tool card renders (probably `list_directory` or `glob`)
     - If the agent calls `set_phase`, the ribbon updates (but no extra tool card for set_phase — that's the `hideCard` flag working)
     - At turn end, a SummaryCard shows files_created / files_modified / duration / tokens

## Step 3 — Delete legacy code

Only after Step 2 smoke test passes.

1. **Delete the pair_pipeline files** (Phase 0 Task 0.3 already deleted the core files; this is a final sweep):
   ```bash
   grep -rn "pair_pipeline\|PairPipeline\|FixLoopStarted\|FixLoopCompleted" ai-worker/ forge-core/ forge-portal/ 2>/dev/null
   ```
   Expected: zero matches in code (may appear in comments or git history references, which is fine).

2. **Delete the build-card frontend component** (Phase 6 Task 6.1 already deleted it; final sweep):
   ```bash
   ls forge-portal/components/agent/build-card.tsx 2>/dev/null && echo "STILL EXISTS — delete it" || echo "ok, deleted"
   ```

3. **Commit any final cleanup** if the sweeps found anything:
   ```bash
   git add -A && git commit -m "chore: final pair_pipeline cleanup post-chronos deploy"
   ```

## Post-deploy verification checklist

Use these to confirm the deploy is healthy. If any fails, consider rolling back.

- [ ] `curl -sf http://localhost:8090/health | jq .status` returns `"ok"`
- [ ] `curl -sf http://localhost:8080/health | jq .status` returns `"ok"`
- [ ] `docker compose logs forge-ai-worker --tail 50 | grep -i error` returns nothing
- [ ] `docker compose logs forge-core --tail 50 | grep -i error` returns nothing
- [ ] A test agent message (via UI or API) produces events in Redis:
  ```bash
  docker compose exec redis redis-cli KEYS "agent:stream:*"
  ```
  Expected: at least one stream key after a test message.
- [ ] The Redis stream events have the expected shape:
  ```bash
  docker compose exec redis redis-cli XRANGE agent:stream:<session_id> - +
  ```
  Expected events: `text_delta`, `tool_started`, `tool_completed`, `phase_changed` (if agent called set_phase), `session_complete`. NO `fix_loop_started` / `fix_loop_completed` should appear.
- [ ] Tool cards in the UI render with the new formatters — bash cards show `▶ command · exit code`, file tool cards show path + status
- [ ] `set_phase` calls don't render a tool card (hideCard working)
- [ ] Step ribbon updates as the agent calls `set_phase`
- [ ] Observability: `docker compose logs forge-ai-worker 2>&1 | grep -c "agent.tool_call"` returns > 0 after a real agent run

## Rollback procedure

If the deploy goes wrong:

1. **Restore the previous code:**
   ```bash
   PRE_DEPLOY_SHA=$(cat /tmp/chronos-rollback-sha)
   git reset --hard "$PRE_DEPLOY_SHA"
   ```

2. **Rebuild and restart:**
   ```bash
   docker compose -f docker-compose.dev.yml build forge-ai-worker
   cd forge-core && go build ./cmd/forge-core
   docker compose -f docker-compose.dev.yml up -d
   ```

3. **Database migrations are additive** — they don't need to be rolled back. `engine.workspaces` and `engine.project_deploy_keys` just stay present but unused if the old code is restored. This is the intended design (spec §8 rollback section).

4. **Report the failure:** note which step failed, what the error was, and attach the last 200 lines of both ai-worker and forge-core logs to an incident document so the next retry isn't blind.

## Known deferred items

These are NOT in chronos's scope — if the post-deploy verification exposes them, don't panic:

- **Profile data is empty.** Phase 5 Task 5.5 registers context tools with `profiles={}` — profile scan pipeline integration is a follow-up. The `query_api_catalog` etc. tools will return "no data" until profile scan is wired. The agent can still work fine using file tools + bash.
- **Cost USD is 0.0 in SessionComplete.** Phase 5 Task 5.3 defers cost tracking — UsageSnapshot doesn't carry per-turn cost. Future: add `total_cost_usd` field or compute in api_server.
- **No dashboards or alerts.** Phase 7 Task 7.2 landed structured log points but not Grafana dashboards. Post-deploy canary in the runbook uses `docker compose logs | grep` — good enough for the first week.
- **No distributed tracing / OTel.** Not in chronos scope.

## Success criteria

The deploy is considered successful when:

1. All steps in this runbook completed without errors
2. The post-deploy verification checklist is fully green
3. At least one real user message (from Harvey or a test user) completes end-to-end and produces the expected Variant B UI behavior
4. 24 hours have passed without a rollback

Once #4 is hit, chronos is considered shipped. Update the project memory (Task 7.4) and close out the plan.
````

- [ ] **Step 2: Commit the runbook**

```bash
git add docs/plans/chronos-2026-04-09/deploy-runbook.md
git commit -m "docs(chronos): deploy runbook with three-step procedure

Executable checklist for shipping chronos to production.
Three phases matching spec §8:

1. Migrations + image rebuild — apply 025/026, cleanup stale
   agent_messages rows, rebuild ai-worker with bwrap+ripgrep,
   rebuild forge-core binary. Each step has a copy-pasteable
   command and a verification query.

2. Deploy new code + smoke test — docker compose up, health
   check both services, smoke-test /api/workspace/prep, run
   the E2E real-LLM test (costs ~\$0.10), manual browser smoke.

3. Delete legacy code — final grep sweeps to confirm pair_pipeline
   and build-card are fully gone.

Plus:
- Pre-deploy checklist (Phase completion checks, secrets env,
  image verification)
- Post-deploy verification checklist (11 items covering health,
  logs, Redis stream shape, UI behavior, observability)
- Rollback procedure (git reset + rebuild; migrations are
  additive and don't need rollback per spec §8)
- Known deferred items (empty profiles, cost=0.0, no dashboards,
  no OTel — all explicit out-of-scope notes)
- Success criteria: 24 hours without rollback = shipped

Runbook lives inside the chronos plan directory so the plan,
code, and deploy procedure are all co-located."
```

---

### Task 7.4: Session memory + retro

**Files:**
- Create: `~/.claude/projects/D--shulex-work-forge/memory/chronos-delivery-2026-04-09.md`
- Modify: `~/.claude/projects/D--shulex-work-forge/memory/MEMORY.md` (add index entry)
- Create: `docs/plans/chronos-2026-04-09/retro.md` (in-repo retro for the team)

**Context:** Two outputs — a personal/session memory entry in `~/.claude/projects/.../memory/` and an in-repo retrospective in the chronos plan directory. The memory entry is for Claude (future sessions) and the in-repo retro is for the team (Harvey).

Both summarize: what shipped, what was deferred, what was learned, what the next team needs to know.

- [ ] **Step 1: Write the session memory file**

Create `~/.claude/projects/D--shulex-work-forge/memory/chronos-delivery-2026-04-09.md`:

```markdown
---
name: chronos delivery
description: Summary of the chronos plan delivery (Variant B single-agent) — what shipped, what deferred, what's load-bearing
type: project
---

**chronos** is the Greek-mythology-named plan that rebuilt Forge's AI agent pipeline from the pair_pipeline workaround into a real A2 single-agent architecture. Design spec at `docs/specs/2026-04-09-agent-variant-b-single-agent-design.md`. Plan directory at `docs/plans/chronos-2026-04-09/` with 8 files (index + 7 phases + deploy runbook + retro).

**Why:** Phase 0-6 pair_pipeline was a hollow-pipeline workaround — it had the Variant B UI shell on the frontend but the backend ran a fixed Coder→Build→Reviewer sequence that couldn't actually use tools (tool registry was empty, agent loop existed but was bypassed). chronos replaced the workaround with a real tool-using single agent.

**How to apply:**

- When referencing "the agent" in this codebase, it means the post-chronos single agent: one QueryEngine, one system prompt, 14 tools (6 context + 6 file + 2 exec = bash + set_phase), bubblewrap sandbox for bash, SSH deploy keys per project.
- `workspace.Manager.EnsureReady(ctx, tenant, project, forceSync)` is the single entry point for workspace lifecycle. Replaces `EnsureClone` which is deleted. All git operations happen in forge-core, not ai-worker — prompt injection in ai-worker cannot exfiltrate deploy keys.
- `BaseTool.execute` is an `AsyncIterator[StreamEvent | ToolResult]`. Tools yield 0+ StreamEvents during execution, then exactly 1 ToolResult. Use `SimpleTool` as a convenience base for tools that don't emit mid-execution events.
- `WorkspacePath.resolve(root, user_path)` is the type-level path sandbox. Never do raw path joins in tool code.
- Stream events: `PhaseChanged`, `ToolExecutionStarted/Completed` both carry `tool_use_id`. `FixLoopStarted/Completed` are **deleted** — frontend detects fix loops visually via `detectFixLoopStart`.
- `_create_engine(req, workspace_dir)` raises on ModelRouter failure. No AsyncMock fallback. Fail fast.
- `LRUSessionCache(max_size=100)` replaces the unbounded `_sessions` dict. Eviction calls `engine.clear()`.
- Observability: structured JSON logs at `agent.tool_call`, `agent.turn_complete`, `agent.bash_denylist_hit`, `workspace.ensure_ready`, `agent.session_start`.

**Deferred (not in chronos scope — known gaps):**

- Profile data is empty (`profiles={}` in `_create_engine`). Profile scan pipeline integration is a follow-up. Context tools return "no data" until this lands.
- `SessionComplete.cost_usd` is hardcoded to 0.0. UsageSnapshot doesn't carry per-turn cost. Future: plumb through ModelRouter.
- No dashboards or alerts. Observability is raw JSON logs only.
- P3 permission mode (per-call approval for writes/bash) is reserved in the design but not implemented. Phase 7 ships P1 (FULL_AUTO).
- Dependency install mid-session is NOT supported. bwrap sandbox has no network. Future: either a special elevated sandbox_prep tool or a P3-gated --share-net toggle.
- Code panel is a read-only preview shell. Inline editing + diff rendering deferred.

**Silicon-valley rules enforced during chronos** (Harvey's explicit standard):

- No hardcoded special cases in the agent loop (`if tool_name == 'bash'` was the anti-pattern we avoided — tools yield their own events)
- No regex denylist as security boundary (bubblewrap is the actual sandbox; denylist is a UX hint that gives agents clean error messages for common mistakes like `sudo`)
- No parallel code paths (one bwrap code path, one ripgrep dependency, no Python fallback, no AsyncMock fallback)
- No memory leaks (LRU session cache has a hard bound)
- Contract-as-mechanical-gate (`test_base_tool_contract.py` parametrized over every registered tool class — new tool auto-gets coverage)

**Test counts by phase (rough totals):**

- Phase 0: 9 secrets service tests
- Phase 1: ~40 workspace module tests (state DAO, deploy keys, git, prep, ensure, lookup)
- Phase 2: 35 tool layer contract + path tests
- Phase 3: ~53 file tool tests + 40 parametrized contract (4×10 tools)
- Phase 4: 11 phase + 17 bash + 13 adversarial + 48 parametrized contract (4×12)
- Phase 5: 10 prompt + 34 SessionCollector + 10 LRU + query/api_server updates
- Phase 6: ~30 frontend vitest cases across step-ribbon, tool-formatters, tool-execution, code-panel, thinking-indicator, agent-chat
- Phase 7: 2 e2e (real-LLM and observability grep)

**Commit count:** ~58 planned commits across 7 phases (one per task minimum).

**Key file changes:**
- Deleted: `ai-worker/src/openharness/engine/pair_pipeline.py`, `ai-worker/src/openharness/hooks/builtin/{build_verify,ci_autofix}_hook.py`, `forge-portal/components/agent/build-card.tsx`, four test files
- New: `forge-core/internal/secrets/`, `forge-core/internal/workspace/{state,deploy_keys,git,prep,lookup,ensure,github_deploy_keys}.go`, migrations 025/026, `ai-worker/src/openharness/engine/{prompts,session_collector,session_cache}.py`, `ai-worker/src/openharness/tools/{workspace_path,file_tools,bash_tool,phase_tool}.py`, `forge-portal/components/agent/` minor additions
- Modified heavily: `ai-worker/src/api_server.py`, `ai-worker/src/openharness/engine/{query,query_engine,stream_events}.py`, `ai-worker/src/openharness/tools/{base,context_tools}.py`, `forge-core/internal/workspace/manager.go`, `forge-core/cmd/forge-core/main.go`, `forge-portal/components/agent/{agent-chat,step-ribbon,tool-formatters,tool-execution,code-panel,thinking-indicator}.tsx`

2026-04-09 delivery session.
```

- [ ] **Step 2: Add an entry to MEMORY.md**

Append to `~/.claude/projects/D--shulex-work-forge/memory/MEMORY.md`:

```markdown
- [Chronos Delivery 2026-04-09](chronos-delivery-2026-04-09.md) — Variant B single-agent landed: 14 tools, bwrap sandbox, SSH deploy keys, SessionCollector, LRU cache, visual fix-loop detection
```

- [ ] **Step 3: Write the in-repo retro**

Create `docs/plans/chronos-2026-04-09/retro.md`:

```markdown
# chronos — Delivery Retro

**Date:** 2026-04-09
**Delivery:** Variant B single-agent architecture (A2)
**Plan:** [index.md](index.md)
**Spec:** [../../specs/2026-04-09-agent-variant-b-single-agent-design.md](../../specs/2026-04-09-agent-variant-b-single-agent-design.md)

## What shipped

Seven phases across ~58 tasks. Every phase has a completion-gate checklist and atomic commits — the plan is resumable from any commit.

**Phase 0** (Infra): bubblewrap + ripgrep in ai-worker image, pathspec dep, deleted pair_pipeline files, 2 DB migrations, AES-GCM secrets service.

**Phase 1** (Workspace Go module): state DAO with advisory lock, ed25519 deploy keys, GitHub deploy-key upload client, SSH-aware git wrapper, prep RPC client, EnsureReady state machine, 3 caller migrations (build_activities, devops_activities, agent/service), main.go wiring.

**Phase 2** (Tool contract): `BaseTool.execute` refactored to async generator, `SimpleTool` convenience subclass, `WorkspacePath` sandbox type, context_tools migrated, parametrized contract tests, adversarial path suite, query.py adapted in the same phase so the build stayed green.

**Phase 3** (File tools): 6 `SimpleTool` subclasses (read/write/edit/glob/grep/list_directory), `register_file_tools` helper, contract suite extended to 10 tools.

**Phase 4** (Bash + SetPhase + events): `PhaseChanged` added, `tool_use_id` added to tool events, `FixLoop*` deleted, `SetPhaseTool` (first BaseTool subclass yielding StreamEvents), `BashTool` with full bubblewrap sandbox, 13 P0 adversarial tests.

**Phase 5** (Agent loop): `build_system_prompt` (real Variant B prompt), `SessionCollector`, `LRUSessionCache`, `_create_engine` rewrite registering all 14 tools, `_route_and_stream` simplification, `/api/workspace/prep` endpoint.

**Phase 6** (Frontend): BuildCard deleted, pair_pipeline state purged from agent-chat, step-ribbon rewritten for dynamic phases, phase_changed wired end-to-end, tool-formatters updated for 8 new tools, tool-execution respects hideCard, code-panel downgraded to read-only, thinking-indicator relocated, `detectFixLoopStart` visual heuristic.

**Phase 7** (Deploy): Real-LLM e2e smoke test, observability log points at 5 boundaries, deploy runbook, retro + project memory.

## What we learned

**Silicon-valley standard enforcement works.** Harvey's explicit rule "no compromises, no debt, no hardcoded special cases" shaped every design decision. The result: zero `if tool_name == 'bash'` branches in the agent loop, zero fallback paths in critical code (bwrap has no fallback, ripgrep has no fallback, ModelRouter has no AsyncMock fallback), contract-as-mechanical-gate on the tool protocol.

**The async-generator BaseTool refactor was the right call.** Originally Phase 2 was going to be 5 tasks with a known build-red period; we added Task 2.6 to adapt `query.py` inside Phase 2 so the build stayed green across all subsequent phases. Slight scope increase per phase, much cleaner handoff between phases.

**Spec-first pays off.** 3 rounds of spec review (~40 min each) caught: the bubblewrap `--share-net=false` bug (that flag doesn't exist and would have silently re-enabled network — a security regression shipped to prod); the `workspace.Manager` collision with the already-wired Go module (would have created two parallel workspace managers); the project.NewService description being factually wrong (I assumed it called EnsureClone, but the code was a WorkspaceProvider interface that only exposes ProjectDir). All three would have been painful to discover mid-implementation.

**Writing plans in multi-file directory format works.** Old monolithic 4426-line plan file became unreviewable after Phase 1. Splitting into `docs/plans/chronos-2026-04-09/phase-N-*.md` + `index.md` meant each phase could be reviewed, executed, committed, and rolled back independently. Phase 1's 3622 lines is the outlier (big Go module) but each subsequent phase stays in the 1500-2700 line range, which is digestible.

**Deferred by design, not by accident.** Each phase's completion check calls out what's NOT in scope — profile data wiring, per-turn cost, dashboards, P3 permissions, dependency install mid-session, code panel diff rendering. Writing these down explicitly stopped scope creep and gave the runbook a clean "known deferred items" section.

## What we'd do differently

**Phase 2 was originally 5 tasks and became 6.** Good call in retrospect (build stayed green) but a signal that rigid phase sizing at plan-writing time doesn't always match execution reality. For future plans: plan a rough task count per phase and allow ±2 during execution without panicking.

**Adversarial tests for BashTool landed in Phase 4 Task 4.5, not Phase 4 Task 4.3.** This means BashTool itself and its adversarial suite were in different commits. In retrospect, writing BashTool without its adversarial tests meant the tool was "done" but not "safe" for ~1 commit cycle. Future: land security tests in the same commit as the security-critical code they gate.

**The prep endpoint and the prep client landed ~4 phases apart** (Phase 1 Task 1.5 for the Go client, Phase 5 Task 5.7 for the Python endpoint). During execution, an integrator could reasonably think Phase 1 is "done" even though its prep client has no server to call. The plan documentation flags this but the phase separation was dictated by the dependency graph (Python side depends on Phase 2-4 BaseTool contract). Not a real problem since both land in the same branch, but worth noting.

## What the next session needs to know

See the [session memory file](../../../../.claude/projects/D--shulex-work-forge/memory/chronos-delivery-2026-04-09.md) for the canonical list of load-bearing invariants and known deferrals. Short list:

1. Tool protocol is `BaseTool.execute -> AsyncIterator[StreamEvent | ToolResult]`. Use `SimpleTool` unless you need mid-execution events.
2. Every file-operating tool validates paths via `WorkspacePath.resolve`. Never raw `pathlib.Path` join.
3. BashTool runs in bwrap. No network. 100 KB output cap. 600s max timeout. Denylist is a UX filter, not a security boundary.
4. `_create_engine` raises on ModelRouter failure. If you see the agent start with fake responses, it's a bug — check you didn't re-introduce an AsyncMock fallback somewhere.
5. The 14 tools are registered via 3 helpers: `register_context_tools`, `register_file_tools`, `register_exec_tools`. Keep new tools in the appropriate group.
6. `FixLoopStarted/Completed` are gone. If you need loop visibility, extend `detectFixLoopStart` in `agent-chat.tsx`.
7. `SessionComplete` is conditional: only emitted when `tool_call_count > 0`. Text-only turns don't get a SummaryCard.
```

- [ ] **Step 4: Commit both files**

```bash
# In-repo files first
git add docs/plans/chronos-2026-04-09/retro.md
git commit -m "docs(chronos): delivery retro + in-repo summary

7-phase delivery retro covering what shipped, what we learned,
what we'd do differently, and the invariants the next session
needs to know.

Key learnings:
- silicon-valley standard enforcement produced a real result
  (no hardcoded special cases, no fallback code paths, contract-
  as-mechanical-gate)
- Phase 2 Task 2.6 (query.py adapt) was the right call —
  kept main green across all later phases
- 3 rounds of spec review caught one critical bug (bwrap
  --share-net=false doesn't exist), one architectural collision
  (Go workspace.Manager already wired), and one factual error
  (project.NewService doesn't call EnsureClone)
- multi-file plan directory format beat the monolithic plan
  after Phase 1's 4426-line unreviewability
- deferred-by-design scope cuts stopped creep and gave the
  runbook a clean 'known deferred' section

Retro lives in the plan directory so future team members
reading chronos can see what the team learned, not just what
the team built."

# Then the session memory (not in repo, user-local ~/.claude/)
# Only execute this if the executing agent has write access to ~/.claude/
# Otherwise, the memory file already exists on this host from Step 1 and
# doesn't need re-committing.
```

- [ ] **Step 5: Update the index.md to mark Phase 7 complete**

Edit `docs/plans/chronos-2026-04-09/index.md`:

Change `**Status:** In progress (Phase 0, 1, 2, 3, 4, 5, 6 written; Phase 7 pending)` to `**Status:** ✅ DELIVERED 2026-04-09 — all 7 phases written, deploy runbook in place, retro complete`.

Change Phase 7's row from `⏳ pending` to `✅ written` with a link to `phase-7-deploy.md`.

- [ ] **Step 6: Final commit**

```bash
git add docs/plans/chronos-2026-04-09/index.md
git commit -m "docs(chronos): mark delivery complete in index

All 7 phases written. Plan directory is ready for execution
via superpowers:subagent-driven-development or superpowers:
executing-plans. Deploy runbook landed in Task 7.3, retro
landed in Task 7.4, session memory landed in the parallel
project memory store.

chronos is now in the 'plan written, ready to execute' state."
```

---

## Phase 7 completion check

Before declaring chronos shipped:

- [ ] `pytest ai-worker/tests/e2e/test_variant_b_smoke.py -m e2e --collect-only` lists 2 tests (skipped without `FORGE_E2E_ENABLED`)
- [ ] With `FORGE_E2E_ENABLED=1`, the real-LLM test passes on a fixture Go project
- [ ] `grep -rn 'agent\.tool_call\|agent\.turn_complete\|agent\.bash_denylist_hit\|workspace\.ensure_ready\|agent\.session_start' ai-worker/src/ forge-core/internal/` returns ≥ 5 matches
- [ ] `docs/plans/chronos-2026-04-09/deploy-runbook.md` exists and is ≥ 200 lines
- [ ] `docs/plans/chronos-2026-04-09/retro.md` exists
- [ ] `~/.claude/projects/D--shulex-work-forge/memory/chronos-delivery-2026-04-09.md` exists (session memory)
- [ ] MEMORY.md index has the chronos entry
- [ ] `docs/plans/chronos-2026-04-09/index.md` Phase 7 row shows `✅ written`
- [ ] Branch has **4 new commits** from this phase

## Phase 7 outputs unlock

- **The deploy can be run.** Follow `deploy-runbook.md` step by step.
- **Future sessions have context.** The session memory file + in-repo retro give any agent (or Harvey) a 5-minute read to understand what chronos delivered and what's load-bearing.
- **Observability is wired.** Structured log points mean the first production bug is diagnosable without re-instrumenting the code.

---

## chronos project wrap-up

After Phase 7 lands, the chronos plan directory contains:

```
docs/plans/chronos-2026-04-09/
├── index.md                          (project overview, phase table, status)
├── phase-0-infrastructure.md         (6 tasks)
├── phase-1-workspace.md              (13 tasks)
├── phase-2-basetool.md               (6 tasks)
├── phase-3-file-tools.md             (8 tasks)
├── phase-4-bash-events.md            (8 tasks)
├── phase-5-agent-loop.md             (7 tasks)
├── phase-6-frontend.md               (9 tasks)
├── phase-7-deploy.md                 (4 tasks)  ← this file
├── deploy-runbook.md                 (executable deploy checklist)
└── retro.md                          (delivery retrospective)
```

Total: **~61 tasks across 7 phases, ~16,000 lines of plan content, 11 files in the chronos directory.**

Every task is TDD-structured with bite-sized steps (2–5 minutes each), explicit file paths, complete code blocks (no pseudocode), and exact commands with expected outputs. The plan is designed to be executed by a fresh agent that has never seen the chronos design — every dependency is spelled out, every breaking change is scoped to a single task, every file change is reviewable in isolation.

chronos is **ready to ship**.
