# chronos · Phase 5 — Agent Loop + api_server + Prompts

> **Project:** [chronos — Agent Variant B Single-Agent Implementation](index.md)
> **Phase:** 5 of 7 · **Tasks:** 7 · **Depends on:** [Phase 1](phase-1-workspace.md), [Phase 3](phase-3-file-tools.md), [Phase 4](phase-4-bash-events.md) · **Unblocks:** Phase 6, Phase 7
> **Spec reference:** [Design spec §4.12 (tool registry construction), §5 (agent loop & event layer), §5.2 (system prompt), §5.6 (SessionCollector), §5.7 (routing), §5.8 (LRU cache), §3.9 (prep endpoint)](../../specs/2026-04-09-agent-variant-b-single-agent-design.md)

**Execution:** REQUIRED SUB-SKILL: Use `superpowers:subagent-driven-development` (recommended) or `superpowers:executing-plans`. Steps use checkbox (`- [ ]`) syntax for tracking.

---

## Phase goal

Wire everything Phase 1–4 built into the agent runtime. Seven tasks:

1. **`prompts.py`** — `build_system_prompt(language, workspace_path)` generates the real Variant B system prompt (not the one-liner "you are a helpful AI coding assistant" currently in the code)
2. **`session_collector.py`** — `SessionCollector` tracks per-turn file creates/modifies/build status by observing `ToolExecutionStarted`/`Completed` events; `_is_build_like` helper + build-like token set
3. **`QueryEngine` integration** — `query_engine.py` wires `SessionCollector` into `submit_message`, emits `SessionComplete` conditionally (only if any tool was called)
4. **`LRUSessionCache`** — replaces the `_sessions: Dict[str, Any]` in `api_server.py` with a 100-entry LRU that calls `engine.clear()` on eviction
5. **`_create_engine` rewrite** — delete the `purpose` parameter, delete the `Purpose.REVIEW` branch, register all 14 tools (6 context + 6 file + 2 exec), raise on missing ModelRouter (no AsyncMock fallback)
6. **`_route_and_stream` simplification** — delete the pair_pipeline branch, delete the guarded import of `pair_pipeline`, make `workspace_path` required, add defensive `is_dir()` check
7. **`/api/workspace/prep` endpoint** — the HTTP handler that forge-core's `PrepClient` (Phase 1 Task 1.5) calls. Detects language, runs prep command via non-sandboxed subprocess (has network inside ai-worker container), returns structured result

Plus side changes: `_serialize_event` grows a `phase_changed` branch and a `tool_use_id` field on `tool_started`/`tool_completed` (matching Phase 4 Task 4.1's event shape changes).

**Completion gate:**
- `pytest ai-worker/tests/test_api_server.py tests/test_api_server_route.py -v` — all passing
- `pytest ai-worker/tests/test_query_engine.py -v` — passes with SessionCollector integration
- `pytest ai-worker/tests/openharness/engine/test_session_collector.py -v` — new test file passes
- `pytest ai-worker/tests/openharness/engine/test_prompts.py -v` — new test file passes
- `grep -n "pair_pipeline" ai-worker/src/api_server.py` returns nothing (guarded import gone)
- `grep -n "Purpose.REVIEW\|Purpose.GENERATE" ai-worker/src/api_server.py` returns nothing (purpose parameter gone)
- `grep -n "AsyncMock" ai-worker/src/api_server.py` returns nothing (mock fallback gone — fail fast)
- `curl -X POST http://localhost:8090/api/workspace/prep -d '{"tenant_id":1,"project_id":1,"workspace_path":"test"}' -H "Content-Type: application/json"` returns JSON shape `{status, language, command, ...}` (or an error status for a fake workspace)
- `docker compose -f docker-compose.dev.yml exec forge-ai-worker python -c "from src.api_server import _create_engine; print('ok')"` prints `ok` (no import errors)
- Manual smoke: send an agent message and watch the Redis stream — expect `text_delta`, `tool_started/completed` with `tool_use_id`, optional `phase_changed`, optional `thinking_started/stopped`, `session_complete` at the end

## Why this phase matters

Phase 1–4 built the parts. Phase 5 **assembles the car**. Until Phase 5 lands:
- Phase 2–4 tools exist but aren't registered in any engine
- `query_engine.py`'s `submit_message` yields events but doesn't emit `SessionComplete`
- `api_server.py` still routes to `pair_pipeline` (which Phase 0 deleted — the import is guarded and falls through, so the server currently runs but with an empty tool registry, the old hollow pipeline)
- forge-core's `PrepClient` has nowhere to POST — the endpoint doesn't exist

Phase 5 closes all four gaps in one phase so Phase 6 (frontend) can consume a working agent and Phase 7 (e2e) can exercise it end-to-end.

**Silicon-valley rules for this phase:**
- **No AsyncMock fallback.** If `ModelRouter` fails to initialize, raise. A silent mock that returns fake data is the worst kind of debt — it hides real failures until the e2e test stage, where they're expensive to diagnose.
- **No `purpose` parameter.** The pair_pipeline era had `Purpose.GENERATE` and `Purpose.REVIEW` branching system prompts. A2 is single-agent — one purpose, one prompt. Kill the parameter, kill the branch, kill the `Purpose` import.
- **LRU cache has a hard upper bound.** Unbounded `_sessions: Dict` leaks memory across long-running processes. 100 is an arbitrary ceiling; if ops hits it, swap for Redis-backed session storage. For now, bounded + loud eviction logs.
- **Prep endpoint has a strict timeout.** Dependency install is slow but not infinite. 10 minutes is the cap; beyond that the prep is hung and we error out so forge-core's PrepClient can move on (and the agent starts anyway, matching spec §3.9's "prep is a non-blocking soft failure").

---

### Task 5.1: `build_system_prompt` — the real Variant B system prompt

**Files:**
- Create: `ai-worker/src/openharness/engine/prompts.py`
- Create: `ai-worker/tests/openharness/engine/__init__.py`
- Create: `ai-worker/tests/openharness/engine/test_prompts.py`

**Context:** The current system prompt in `api_server.py:_create_engine` is literally `"You are a helpful AI coding assistant."` That's fine for smoke-testing the wiring but useless for real agent behavior — LLMs need to know what tools they have, how to use them, what the phase structure is, and what the sandbox constraints are.

Spec §5.2 lays out the full prompt structure. Port it into a testable helper:

```python
def build_system_prompt(language: str | None, workspace_path: str) -> str:
    ...
```

The function is pure (no I/O) so testing is trivial: assert that the returned string contains key phrases.

**Prompt design rules** (from spec):
- Mention every tool by name with a one-line "when to use"
- List the 7 phases explicitly and tell the agent to use `set_phase` at transitions
- Explain the sandbox constraints (no network, workspace-scoped paths, 120s default timeout)
- Include `edit_file` prefer-over-`write_file` guidance
- Tell the agent to strip line numbers from `read_file` output before passing to `edit_file`
- Tell the agent to stop when the user's request is satisfied — no over-engineering, no unrequested refactors
- Terse output style: the UI shows tool cards, so text explanations focus on "why" not "what"

- [ ] **Step 1: Create the test directory and failing tests**

```bash
mkdir -p ai-worker/tests/openharness/engine
touch ai-worker/tests/openharness/engine/__init__.py
```

Create `ai-worker/tests/openharness/engine/test_prompts.py`:

```python
"""Tests for build_system_prompt.

The system prompt is just a string, so tests are assertions over
substring presence. Goal: if someone accidentally drops the
'set_phase' instruction or the 'no network' constraint, the test
catches it before the agent silently starts misbehaving.
"""

from src.openharness.engine.prompts import build_system_prompt


def test_prompt_mentions_all_seven_phases():
    prompt = build_system_prompt(language="go", workspace_path="/ws/project")
    for phase in ("Analyze", "Plan", "Generate", "Build", "Test", "Review", "Deploy"):
        assert phase in prompt, f"phase {phase} missing from system prompt"


def test_prompt_mentions_set_phase_tool():
    prompt = build_system_prompt(language="python", workspace_path="/ws/p")
    assert "set_phase" in prompt


def test_prompt_mentions_all_six_file_tools():
    prompt = build_system_prompt(language=None, workspace_path="/ws/p")
    for tool in ("read_file", "write_file", "edit_file", "glob", "grep", "list_directory"):
        assert tool in prompt, f"tool {tool} missing from system prompt"


def test_prompt_mentions_bash_and_sandbox_constraints():
    prompt = build_system_prompt(language="go", workspace_path="/ws/p")
    assert "bash" in prompt
    assert "no network" in prompt.lower() or "NO network" in prompt
    assert "120" in prompt  # default timeout


def test_prompt_mentions_edit_file_preference():
    """The agent should prefer edit_file over write_file for small changes."""
    prompt = build_system_prompt(language="go", workspace_path="/ws/p")
    # Some variation of "prefer edit_file" or "use edit_file for small"
    assert "edit_file" in prompt
    assert "prefer" in prompt.lower() or "preferred" in prompt.lower()


def test_prompt_mentions_line_number_stripping():
    """read_file returns content with line-number prefix; agent must
    strip before passing into edit_file."""
    prompt = build_system_prompt(language="go", workspace_path="/ws/p")
    assert "line number" in prompt.lower() or "line-number" in prompt.lower()
    assert "strip" in prompt.lower()


def test_prompt_with_known_language():
    prompt = build_system_prompt(language="python", workspace_path="/data/ws/py-project")
    assert "python" in prompt.lower()
    assert "/data/ws/py-project" in prompt


def test_prompt_with_unknown_language():
    """Language=None should still produce a valid prompt — it just
    doesn't make a language-specific claim."""
    prompt = build_system_prompt(language=None, workspace_path="/ws/p")
    assert "unknown" in prompt.lower() or "inspect" in prompt.lower()
    assert "/ws/p" in prompt


def test_prompt_tells_agent_to_stop_when_done():
    prompt = build_system_prompt(language="go", workspace_path="/ws/p")
    assert "stop" in prompt.lower() or "done" in prompt.lower()
    # Explicit anti-overengineering guidance
    assert "over-engineer" in prompt.lower() or "not ask" in prompt.lower()


def test_prompt_is_nonempty_and_reasonable_length():
    prompt = build_system_prompt(language="go", workspace_path="/ws/p")
    # Expect 1000-5000 chars — enough to cover tools + phases +
    # constraints, not so much that token budget explodes
    assert 1000 < len(prompt) < 6000, f"prompt length {len(prompt)} is outside 1000-6000"
```

- [ ] **Step 2: Run tests — expect import failure**

```bash
cd ai-worker && python -m pytest tests/openharness/engine/test_prompts.py -v
```
Expected: `ModuleNotFoundError: src.openharness.engine.prompts`.

- [ ] **Step 3: Implement `prompts.py`**

Create `ai-worker/src/openharness/engine/prompts.py`:

```python
"""System prompts for the Variant B single-agent.

The prompt is a single large f-string assembled by build_system_prompt.
Kept in a dedicated module (rather than inline in api_server.py) so
tests can assert substring invariants — if a future refactor drops
the 'set_phase' instruction or the 'no network' sandbox constraint,
the tests in test_prompts.py catch it before the agent silently
starts misbehaving.

Spec: §5.2 System prompt.
"""

from __future__ import annotations

from typing import Optional


def build_system_prompt(language: Optional[str], workspace_path: str) -> str:
    """Build the full system prompt for a Variant B agent session.

    Args:
        language: Detected language name (e.g. "go", "python", "java")
            or None if detection failed / unknown project layout.
        workspace_path: Absolute path where the agent's workspace is
            mounted inside the ai-worker container. Usually
            /data/forge/workspaces/tenant-N/project-N/repo.

    Returns:
        A multi-line system prompt, typically 2000-3000 characters,
        describing the environment, tools, phases, constraints, and
        style guide for the agent.
    """
    lang_line = (
        f"- Project language: {language}"
        if language
        else "- Project language: unknown (inspect files with list_directory/glob/read_file to detect)"
    )

    return f"""You are Forge Agent, an AI coding assistant embedded in a Harness Engineering platform. You work on a user's codebase inside a sandboxed workspace.

## Your environment
- Workspace root: {workspace_path}
{lang_line}
- Sandbox: no network access, cwd locked to workspace, bash timeout 120s default (max 600s)
- You operate with full-auto permissions in this release — no per-call human approval. Be deliberate.

## Available tools

**File reading & search**
- `read_file` — read a file or a line range; output has cat -n-style line-number prefixes
- `glob` — find files by pattern (**/*.go, src/**/*.{{ts,tsx}}, etc.)
- `grep` — search file contents with regex (ripgrep under the hood, fast on large trees)
- `list_directory` — one-level directory listing (dirs first, then files)

**File writing**
- `write_file` — create a new file or overwrite an existing one (parent dirs auto-created)
- `edit_file` — exact-string replacement; preferred over write_file for small changes (less error-prone)

**Execution**
- `bash` — run a shell command in the sandbox (build, test, lint, git inspection)

**Workflow signaling**
- `set_phase` — signal which workflow phase you're currently in (updates the UI step ribbon)

## How to work

1. Understand the user's request. If it's ambiguous, ask for clarification before acting.
2. Before making changes, read the relevant existing code. Use glob/grep to find things. Use read_file to see exact content.
3. Signal your phase with `set_phase`. The 7 phases are:
   - **Analyze** — understanding requirements and current code
   - **Plan** — deciding what to change
   - **Generate** — writing or editing code
   - **Build** — compiling / running build commands
   - **Test** — running tests
   - **Review** — verifying your own work
   - **Deploy** — committing or preparing for deployment

   You may skip phases (trivial change: straight to Generate) and you may go backwards (Build failed → back to Generate to fix). Call `set_phase` whenever you transition to a different phase so the UI ribbon stays accurate.

4. For code changes, **prefer `edit_file` (exact string replacement) over `write_file` (full file overwrite)**. `write_file` is appropriate when creating a new file or when an `edit_file` would be more disruptive than a rewrite.

5. When you pass content into `edit_file`'s `old_string`, **strip the line-number prefix** that `read_file` added. The prefix is right-aligned in a 6-character field followed by a tab: `"     1\\tpackage main"`. The `old_string` must contain the literal source text `"package main"`, NOT the prefixed form. If `edit_file` reports "old_string not found", this is usually the cause — use `read_file` first, copy the exact source text without the line-number field.

6. After code changes, run build/test with `bash` to verify. If the build fails, read the error, fix the code, and build again. You can iterate freely within a turn.

7. Stop when the user's request is satisfied. Do NOT over-engineer. Do NOT add features the user did not ask for. Do NOT refactor adjacent code unrelated to the task.

## Constraints

- **File operations stay inside the workspace.** Any path escape (absolute paths, `..` traversal) is rejected at the tool boundary with a PathEscapeError.
- **No network access.** Do not attempt `npm install`, `go mod download`, `pip install`, `curl`, `wget`, or similar — they will fail inside the sandbox. Dependencies are pre-installed when the workspace is created. If you need a dependency that isn't available, tell the user so they can add it at the project level.
- **bash commands time out at 120 seconds** by default; pass `timeout` up to 600 seconds for slower operations like large test suites. On timeout, the whole process group is killed.
- **Do not attempt destructive git operations** (`reset --hard`, `push --force`, `branch -D`) unless the user explicitly asks.

## Output style

- Be terse. The UI shows every tool call you make as a card — the user can see WHAT you did. Use text to explain WHY.
- Don't narrate obvious actions. "Let me read the file" is noise; just read it.
- When a build fails, don't announce "I'll fix this" — just fix it.
- When you're done, say what you did in one or two sentences max, then stop.
"""
```

- [ ] **Step 4: Run the prompt tests**

```bash
cd ai-worker && python -m pytest tests/openharness/engine/test_prompts.py -v
```
Expected: **10 tests pass**. If the length test fails because the prompt is too long/short, adjust the prompt or the bounds — but don't drop content to pass the test.

- [ ] **Step 5: Commit**

```bash
git add ai-worker/src/openharness/engine/prompts.py ai-worker/tests/openharness/engine/
git commit -m "feat(engine): build_system_prompt for Variant B single-agent

Dedicated prompts.py module with a single public function
build_system_prompt(language, workspace_path) that returns the
full 7-phase system prompt describing:

- environment (workspace path, language, sandbox constraints)
- all 8 tools by name with one-line 'when to use' guidance
- the 7-phase workflow with set_phase transition rules
- edit_file-over-write_file preference
- the line-number-prefix stripping rule for passing read_file
  output into edit_file
- no-network / no-destructive-git constraints
- terse output style (UI shows tool cards, text explains 'why')

Tests are substring invariants: if someone accidentally drops
a key instruction (set_phase mention, line-number-stripping rule,
no-network constraint) the tests catch it before the agent
silently starts misbehaving.

Length bounded to 1000-6000 chars: enough to cover everything,
not so much that per-turn token cost is noticeable.

This replaces 'You are a helpful AI coding assistant.' which is
the current one-liner in _create_engine. Phase 5 Task 5.5 wires
it in."
```

---

### Task 5.2: `SessionCollector` — per-turn stats tracking

**Files:**
- Create: `ai-worker/src/openharness/engine/session_collector.py`
- Create: `ai-worker/tests/openharness/engine/test_session_collector.py`

**Context:** The `SessionComplete` event (yielded at the end of a successful `submit_message` turn) needs to carry `files_created`, `files_modified`, `build_status`, and token totals. Computing these requires observing the stream of events as they pass through `QueryEngine.submit_message`. The `SessionCollector` class is a small observer: you call `observe(event)` for each event yielded by the agent loop, and at the end you call `should_emit_summary()` and read the accumulated fields.

Key rules from spec §5.6:

1. **Build-like detection**: a bash command is "build-like" if its first token (after stripping `./` prefix) is in `_BUILD_LIKE_FIRST_TOKENS` (go, mvn, gradle, gradlew, npm, pnpm, yarn, pytest, cargo, make, ctest, dotnet, tsc, javac). The most-recent build-like bash call determines `build_status`.
2. **Stats by tool**: `write_file` success → `files_created += 1`. `edit_file` success → `files_modified += 1`. Other tools don't increment.
3. **tool_use_id correlation**: Phase 4 added `tool_use_id` to both `ToolExecutionStarted` and `ToolExecutionCompleted`. The collector stashes bash commands by id on Started and looks them up on Completed — no positional ordering assumptions.
4. **Skip summary on zero tool calls**: if the agent only answered with text (no tool calls in this turn), `should_emit_summary()` returns False. The frontend would show a confusing SummaryCard with `0 files created` otherwise.

- [ ] **Step 1: Write the failing tests**

Create `ai-worker/tests/openharness/engine/test_session_collector.py`:

```python
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
```

- [ ] **Step 2: Run tests — expect module missing**

```bash
cd ai-worker && python -m pytest tests/openharness/engine/test_session_collector.py -v
```
Expected: `ModuleNotFoundError`.

- [ ] **Step 3: Implement `session_collector.py`**

Create `ai-worker/src/openharness/engine/session_collector.py`:

```python
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
```

- [ ] **Step 4: Run tests**

```bash
cd ai-worker && python -m pytest tests/openharness/engine/test_session_collector.py -v
```
Expected: approximately **34 tests pass** (24 parametrized `_is_build_like` + ~10 SessionCollector).

- [ ] **Step 5: Commit**

```bash
git add ai-worker/src/openharness/engine/session_collector.py ai-worker/tests/openharness/engine/
git commit -m "feat(engine): SessionCollector tracks per-turn stats

Small observer class instantiated once per submit_message() turn.
Records files_created (successful write_file count), files_modified
(successful edit_file count), tool_call_count, and last_build_status
('passed'|'failed'|'skipped' based on most recent build-like bash).

Build-like detection: _BUILD_LIKE_FIRST_TOKENS frozenset of known
runner binaries (go, mvn, gradle, npm, pytest, cargo, make, ...)
plus ./gradlew-style path stripping. Non-build bash calls (ls,
git status, grep) don't update build_status.

Correlation of Started/Completed pairs uses tool_use_id from
Phase 4 Task 4.1 — no positional assumptions. A _pending_bash
dict stashes commands on Started and pops on Completed.

should_emit_summary() returns False when tool_call_count == 0,
so agent turns that just answered with text don't emit a confusing
'0 files created' SummaryCard.

Tests: 24 parametrized _is_build_like cases + 10 SessionCollector
behavior tests (write/edit counts, failed ops don't count, build
status override sequence, interleaved tool_use_id correlation,
non-tool events ignored, AssistantTurnComplete is a no-op)."
```

---

### Task 5.3: Integrate `SessionCollector` into `QueryEngine.submit_message`

**Files:**
- Modify: `ai-worker/src/openharness/engine/query_engine.py`
- Modify: `ai-worker/tests/test_query_engine.py`

**Context:** Wire `SessionCollector` into `QueryEngine.submit_message` so it:
1. Instantiates a collector at the start of each turn
2. Calls `collector.observe(event)` for each event from the agent loop
3. Measures wall-clock duration
4. Captures per-turn usage delta (total usage at end minus at start)
5. Emits `SessionComplete` conditionally based on `collector.should_emit_summary()`

The `SessionComplete` dataclass (already in `stream_events.py`) has fields: `files_created`, `files_modified`, `build_status`, `duration_ms`, `tokens_total`, `cost_usd`.

Right now `submit_message` is ~30 lines. This task adds ~25 lines for the collector wiring.

- [ ] **Step 1: Update `submit_message` in `query_engine.py`**

Read the current file first:

```bash
sed -n '68,95p' ai-worker/src/openharness/engine/query_engine.py
```

Replace the `submit_message` method with:

```python
    async def submit_message(self, prompt: str) -> AsyncIterator[StreamEvent]:
        """Submit a user message and yield stream events from the agent loop.

        Tracks per-turn stats via SessionCollector and emits a
        SessionComplete event at the end IF any tools were called
        during the turn. Pure-text turns (agent just replied without
        invoking a tool) skip the SessionComplete to avoid a
        confusing '0 files created' UI card.
        """
        import time

        from .session_collector import SessionCollector
        from .stream_events import SessionComplete

        start_ts = time.monotonic()
        collector = SessionCollector()
        prior_usage = self._total_usage

        # Add user message to history
        user_msg = ConversationMessage.from_user_text(prompt)
        self._messages.append(user_msg)

        # Build context
        context = QueryContext(
            api_client=self._api_client,
            tool_registry=self._tool_registry,
            model=self._model,
            system_prompt=self._system_prompt,
            max_tokens=self._max_tokens,
            max_turns=self._max_turns,
            hook_executor=self._hook_executor,
            permission_checker=self._permission_checker,
            cwd=self._cwd,
        )

        # Run agent loop — forward every event and let the collector
        # observe a copy. The collector is pure observation; it does
        # not mutate or suppress events.
        async for event in run_agent_loop(context, self._messages):
            collector.observe(event)
            if isinstance(event, AssistantTurnComplete):
                self._total_usage = UsageSnapshot(
                    input_tokens=self._total_usage.input_tokens + event.usage.input_tokens,
                    output_tokens=self._total_usage.output_tokens + event.usage.output_tokens,
                )
            yield event

        # End-of-turn SessionComplete, if the turn did any work.
        if collector.should_emit_summary():
            duration_ms = int((time.monotonic() - start_ts) * 1000)
            turn_input_tokens = self._total_usage.input_tokens - prior_usage.input_tokens
            turn_output_tokens = self._total_usage.output_tokens - prior_usage.output_tokens
            tokens_total = turn_input_tokens + turn_output_tokens

            # Cost is not currently tracked on UsageSnapshot — it's
            # computed downstream by the model router. For now we
            # report 0.0 and rely on the api_server's separate cost
            # tracking (if any) to populate the Redis stream event
            # correctly. Future: add total_cost_usd to UsageSnapshot.
            cost_usd = 0.0

            yield SessionComplete(
                files_created=collector.files_created,
                files_modified=collector.files_modified,
                build_status=collector.last_build_status,
                duration_ms=duration_ms,
                tokens_total=tokens_total,
                cost_usd=cost_usd,
            )
```

Note: the import of `SessionCollector` and `SessionComplete` is local (inside the method) rather than at the top of the file. This avoids an import cycle — `session_collector.py` imports from `stream_events.py` which is in the same package as `query_engine.py`, and we want to keep `query_engine.py`'s top-level imports unchanged so this task is a minimal diff.

Also add `SessionComplete` to the imports at the top of `stream_events.py` if not already present (it is — defined in Phase 0 / baseline).

- [ ] **Step 2: Add an integration test to test_query_engine.py**

Append to `ai-worker/tests/test_query_engine.py`:

```python
# ---------------------------------------------------------------------------
# SessionComplete emission (added in Phase 5 Task 5.3)
# ---------------------------------------------------------------------------


@pytest.mark.asyncio
async def test_query_engine_emits_session_complete_after_tool_calls(tmp_path):
    """A turn that invokes a tool should yield SessionComplete at the end."""
    from src.openharness.api.client import (
        ApiMessageCompleteEvent,
        ApiMessageRequest,
        ApiStreamEvent,
        SupportsStreamingMessages,
    )
    from src.openharness.api.usage import UsageSnapshot
    from src.openharness.engine.messages import (
        ConversationMessage,
        TextBlock,
        ToolUseBlock,
    )
    from src.openharness.engine.query_engine import QueryEngine
    from src.openharness.engine.stream_events import SessionComplete
    from src.openharness.tools.base import ToolRegistry
    from src.openharness.tools.phase_tool import SetPhaseTool

    # Fake API client: first call returns a tool_use, second call returns end_turn
    class FakeClient:
        def __init__(self):
            self.calls = 0

        async def stream_message(self, request: ApiMessageRequest):
            self.calls += 1
            if self.calls == 1:
                # Agent requests set_phase
                msg = ConversationMessage(
                    role="assistant",
                    content=[
                        TextBlock(text="starting"),
                        ToolUseBlock(
                            id="toolu_1",
                            name="set_phase",
                            input={"phase": "Generate"},
                        ),
                    ],
                )
                yield ApiMessageCompleteEvent(
                    message=msg,
                    usage=UsageSnapshot(input_tokens=10, output_tokens=5),
                    stop_reason="tool_use",
                )
            else:
                # Agent finishes with text only
                msg = ConversationMessage(
                    role="assistant",
                    content=[TextBlock(text="done")],
                )
                yield ApiMessageCompleteEvent(
                    message=msg,
                    usage=UsageSnapshot(input_tokens=15, output_tokens=8),
                    stop_reason="end_turn",
                )

    registry = ToolRegistry()
    registry.register(SetPhaseTool())

    engine = QueryEngine(
        api_client=FakeClient(),
        tool_registry=registry,
        model="test-model",
        system_prompt="test",
        cwd=tmp_path,
    )

    events = []
    async for event in engine.submit_message("do something"):
        events.append(event)

    # Must contain a SessionComplete
    completions = [e for e in events if isinstance(e, SessionComplete)]
    assert len(completions) == 1, f"expected 1 SessionComplete, got {len(completions)}"
    sc = completions[0]
    assert sc.files_created == 0  # set_phase doesn't touch files
    assert sc.files_modified == 0
    assert sc.build_status == "skipped"  # no bash call
    assert sc.duration_ms >= 0
    assert sc.tokens_total == (10 + 5 + 15 + 8)


@pytest.mark.asyncio
async def test_query_engine_skips_session_complete_for_text_only_turn(tmp_path):
    """A turn with zero tool calls should NOT emit SessionComplete."""
    from src.openharness.api.client import ApiMessageCompleteEvent, ApiMessageRequest
    from src.openharness.api.usage import UsageSnapshot
    from src.openharness.engine.messages import ConversationMessage, TextBlock
    from src.openharness.engine.query_engine import QueryEngine
    from src.openharness.engine.stream_events import SessionComplete
    from src.openharness.tools.base import ToolRegistry

    class FakeClient:
        async def stream_message(self, request):
            msg = ConversationMessage(
                role="assistant",
                content=[TextBlock(text="hello")],
            )
            yield ApiMessageCompleteEvent(
                message=msg,
                usage=UsageSnapshot(input_tokens=5, output_tokens=2),
                stop_reason="end_turn",
            )

    engine = QueryEngine(
        api_client=FakeClient(),
        tool_registry=ToolRegistry(),
        model="test",
        system_prompt="test",
        cwd=tmp_path,
    )

    events = []
    async for event in engine.submit_message("hi"):
        events.append(event)

    completions = [e for e in events if isinstance(e, SessionComplete)]
    assert len(completions) == 0, "SessionComplete should not emit for text-only turns"
```

- [ ] **Step 3: Run tests**

```bash
cd ai-worker && python -m pytest tests/test_query_engine.py -v 2>&1 | tail -30
```
Expected: existing tests still pass + the two new SessionComplete tests pass.

If existing tests fail because they construct a `QueryEngine` then iterate events without expecting a `SessionComplete` at the end: update those tests to tolerate the new event type (just filter it out when asserting on the event list).

- [ ] **Step 4: Commit**

```bash
git add ai-worker/src/openharness/engine/query_engine.py ai-worker/tests/test_query_engine.py
git commit -m "feat(engine): SessionCollector integration in QueryEngine

QueryEngine.submit_message now instantiates a SessionCollector
at the start of each turn, calls observe() for every event from
the agent loop, and emits a SessionComplete at the end IF any
tools were called (tool_call_count > 0).

Fields populated:
- files_created, files_modified, build_status from the collector
- duration_ms from a time.monotonic() span
- tokens_total from the per-turn usage delta (total at end minus
  total at start), ignoring any cross-turn reuse
- cost_usd is 0.0 for now — UsageSnapshot doesn't carry cost,
  downstream cost tracking in api_server would need to populate
  this during _serialize_event if cost UI is a priority

Zero-tool turns (agent just answered with text) do NOT emit
SessionComplete. The UI would otherwise show a confusing card
with '0 files created' for every 'hi how are you' interaction.

Tests: two end-to-end cases with a FakeClient — one tool-use
turn that gets a SessionComplete, one text-only turn that
doesn't.

SessionCollector + SessionComplete imports are local to the
method so the top-level imports of query_engine.py stay
minimal; this is intentional — the diff is just the wiring."
```

---

### Task 5.4: `LRUSessionCache` — bounded session storage

**Files:**
- Create: `ai-worker/src/openharness/engine/session_cache.py`
- Create: `ai-worker/tests/openharness/engine/test_session_cache.py`

**Context:** `api_server.py:54` currently has `_sessions: Dict[str, Any] = {}`. This is unbounded — session IDs accumulate forever. In production that's a memory leak. Replace with a proper LRU cache with a hard upper bound of 100 entries.

The cache operates on `QueryEngine` instances (the type hint is `Any` because api_server.py imports are lazy). On eviction, the evicted engine's `clear()` method is called so its message history is released. `put()` returns the old engine if one was evicted so the caller can log it.

Interface:
```python
class LRUSessionCache:
    def __init__(self, max_size: int = 100) -> None: ...
    def get(self, session_id: str) -> QueryEngine | None: ...
    def put(self, session_id: str, engine: QueryEngine) -> None: ...
    def pop(self, session_id: str) -> QueryEngine | None: ...
    def __len__(self) -> int: ...
```

Using `collections.OrderedDict` is the standard Python idiom — `move_to_end` on access gives LRU semantics for free.

- [ ] **Step 1: Write the failing tests**

Create `ai-worker/tests/openharness/engine/test_session_cache.py`:

```python
"""Tests for LRUSessionCache — bounded session storage for api_server."""

from unittest.mock import MagicMock

import pytest

from src.openharness.engine.session_cache import LRUSessionCache


def _make_engine():
    """Fake engine — just needs a clear() method for eviction."""
    return MagicMock(clear=MagicMock())


def test_empty_cache_get_returns_none():
    cache = LRUSessionCache(max_size=3)
    assert cache.get("missing") is None
    assert len(cache) == 0


def test_put_then_get():
    cache = LRUSessionCache(max_size=3)
    engine = _make_engine()
    cache.put("s1", engine)
    assert cache.get("s1") is engine
    assert len(cache) == 1


def test_put_same_session_twice_does_not_grow():
    cache = LRUSessionCache(max_size=3)
    e1 = _make_engine()
    e2 = _make_engine()
    cache.put("s1", e1)
    cache.put("s1", e2)  # same id, new engine
    assert len(cache) == 1
    assert cache.get("s1") is e2


def test_lru_eviction_order():
    cache = LRUSessionCache(max_size=3)
    e1, e2, e3, e4 = [_make_engine() for _ in range(4)]

    cache.put("s1", e1)
    cache.put("s2", e2)
    cache.put("s3", e3)
    assert len(cache) == 3

    # Inserting a 4th should evict the oldest (s1)
    cache.put("s4", e4)
    assert len(cache) == 3
    assert cache.get("s1") is None
    assert cache.get("s2") is e2
    assert cache.get("s3") is e3
    assert cache.get("s4") is e4


def test_eviction_calls_engine_clear():
    """Evicted engines get their clear() method called so message
    history is released."""
    cache = LRUSessionCache(max_size=2)
    e1, e2, e3 = [_make_engine() for _ in range(3)]

    cache.put("s1", e1)
    cache.put("s2", e2)
    cache.put("s3", e3)  # evicts s1

    e1.clear.assert_called_once()
    e2.clear.assert_not_called()
    e3.clear.assert_not_called()


def test_get_refreshes_lru_position():
    """Accessing an entry via get() should move it to the most-
    recently-used position, preventing eviction on the next put."""
    cache = LRUSessionCache(max_size=3)
    e1, e2, e3, e4 = [_make_engine() for _ in range(4)]

    cache.put("s1", e1)
    cache.put("s2", e2)
    cache.put("s3", e3)

    # Touch s1 — now s2 is the LRU
    assert cache.get("s1") is e1

    # Insert s4 — should evict s2, not s1
    cache.put("s4", e4)
    assert cache.get("s1") is e1  # still there
    assert cache.get("s2") is None  # evicted
    assert cache.get("s3") is e3
    assert cache.get("s4") is e4


def test_pop_removes_without_calling_clear():
    """pop() is for explicit session deletion via DELETE /api/sessions/{id}
    — the caller decides whether to clear the engine."""
    cache = LRUSessionCache(max_size=3)
    e1 = _make_engine()
    cache.put("s1", e1)

    popped = cache.pop("s1")
    assert popped is e1
    assert cache.get("s1") is None
    e1.clear.assert_not_called()


def test_pop_missing_returns_none():
    cache = LRUSessionCache(max_size=3)
    assert cache.pop("nonexistent") is None


def test_put_with_max_size_1():
    """Edge case: max_size=1 means every put evicts the previous."""
    cache = LRUSessionCache(max_size=1)
    e1, e2 = _make_engine(), _make_engine()

    cache.put("s1", e1)
    cache.put("s2", e2)

    assert cache.get("s1") is None
    assert cache.get("s2") is e2
    e1.clear.assert_called_once()


def test_refreshing_same_id_does_not_trigger_eviction():
    """Putting an existing id with a new engine replaces in place —
    no eviction happens even at cache capacity."""
    cache = LRUSessionCache(max_size=2)
    e1, e2, e3 = [_make_engine() for _ in range(3)]

    cache.put("s1", e1)
    cache.put("s2", e2)
    # Replace s1 with e3 — no eviction
    cache.put("s1", e3)
    assert len(cache) == 2
    assert cache.get("s1") is e3
    assert cache.get("s2") is e2
    # e1 was REPLACED, not evicted — its clear() should NOT be called.
    # (This is a judgment call; some implementations would clear the
    # replaced engine. For simplicity we don't — the caller who put()s
    # a replacement owns the lifecycle of the old instance.)
    # Actually let's require clear to be called on replacement too:
    e1.clear.assert_called_once()
```

**Note**: the last test asserts that `clear()` IS called on replacement. That's the safer choice — it prevents message history leaks when a session is re-created (e.g., after a server restart reusing old session IDs). Make the implementation match.

- [ ] **Step 2: Run tests — expect import failure**

```bash
cd ai-worker && python -m pytest tests/openharness/engine/test_session_cache.py -v
```

- [ ] **Step 3: Implement `session_cache.py`**

Create `ai-worker/src/openharness/engine/session_cache.py`:

```python
"""LRUSessionCache — bounded in-memory store for QueryEngine instances.

Replaces the unbounded `_sessions: Dict[str, Any] = {}` in
api_server.py. Bounds memory growth at `max_size` entries (default
100) by evicting the least-recently-used session when capacity is
reached. Evicted engines get their clear() method called so
message history is released.

Not thread-safe. api_server.py is single-process async; for true
multi-process session storage, swap this for a Redis-backed cache
and keep the same interface.

Spec: §5.8 Session cache (LRU).
"""

from __future__ import annotations

import logging
from collections import OrderedDict
from typing import Any, Optional

logger = logging.getLogger(__name__)


class LRUSessionCache:
    """LRU cache mapping session_id to QueryEngine.

    Interface:
      - get(session_id) -> engine or None; refreshes LRU position
      - put(session_id, engine); evicts oldest if at capacity, or
        replaces in place if session_id already present
      - pop(session_id) -> engine or None; explicit delete, does NOT
        call clear() on the returned engine (caller owns lifecycle)
      - __len__() -> current entry count

    Eviction (from put() overflow) and replacement (from put() with
    an existing id) both call engine.clear() on the departing engine
    so message history is released. pop() does not, because the
    caller is explicitly taking ownership and will clear it themselves
    or keep it alive.
    """

    def __init__(self, max_size: int = 100) -> None:
        if max_size < 1:
            raise ValueError(f"max_size must be >= 1, got {max_size}")
        self._max_size = max_size
        self._cache: OrderedDict[str, Any] = OrderedDict()

    def get(self, session_id: str) -> Optional[Any]:
        """Return the cached engine for session_id, or None. If
        found, refresh its LRU position (most recently used)."""
        if session_id in self._cache:
            self._cache.move_to_end(session_id)
            return self._cache[session_id]
        return None

    def put(self, session_id: str, engine: Any) -> None:
        """Insert or replace a session. Calls clear() on any
        departing engine (replaced or evicted)."""
        if session_id in self._cache:
            # In-place replacement — clear the old engine first
            old = self._cache[session_id]
            try:
                old.clear()
            except Exception as e:
                logger.warning(
                    "LRUSessionCache: clear() on replaced engine raised: %s",
                    e,
                )
            self._cache[session_id] = engine
            self._cache.move_to_end(session_id)
            return

        self._cache[session_id] = engine
        # Move-to-end is redundant for a fresh insert (OrderedDict
        # inserts at end), but explicit is fine.
        self._cache.move_to_end(session_id)

        # Enforce max_size
        while len(self._cache) > self._max_size:
            oldest_id, oldest_engine = self._cache.popitem(last=False)
            try:
                oldest_engine.clear()
            except Exception as e:
                logger.warning(
                    "LRUSessionCache: clear() on evicted engine raised: %s",
                    e,
                )
            logger.info(
                "LRUSessionCache: evicted session %s (size was %d)",
                oldest_id,
                self._max_size,
            )

    def pop(self, session_id: str) -> Optional[Any]:
        """Remove a session explicitly. Returns the engine if it
        existed, else None. Does NOT call clear() — the caller owns
        the returned engine's lifecycle."""
        return self._cache.pop(session_id, None)

    def __len__(self) -> int:
        return len(self._cache)
```

- [ ] **Step 4: Run tests**

```bash
cd ai-worker && python -m pytest tests/openharness/engine/test_session_cache.py -v
```
Expected: **10 tests pass**.

- [ ] **Step 5: Commit**

```bash
git add ai-worker/src/openharness/engine/session_cache.py ai-worker/tests/openharness/engine/test_session_cache.py
git commit -m "feat(engine): LRUSessionCache bounded at 100 entries

Replaces api_server.py's unbounded _sessions: Dict[str, Any]
with an OrderedDict-backed LRU cache. Default bound is 100
sessions; configurable via constructor.

Semantics:
- get(id) returns engine or None; refreshes LRU position
- put(id, engine) replaces in place OR evicts oldest on overflow;
  BOTH paths call engine.clear() on the departing engine so
  message history is released
- pop(id) removes explicitly; does NOT call clear (caller owns
  lifecycle — used by DELETE /api/sessions/{id})
- __len__() returns current entry count

Eviction logs at INFO. clear() exceptions are caught and logged
at WARNING so a buggy engine can't take down the cache.

Not thread-safe. api_server.py is single-process async. For
multi-process session storage, swap the implementation for a
Redis-backed cache and keep the interface stable.

Tests: 10 cases covering empty get, put/get, repeat-put, LRU
eviction order, clear-on-eviction, get-refreshes-position,
pop-no-clear, pop-missing, max_size=1 edge, replacement-calls-clear.

Phase 5 Task 5.6 wires this into api_server.py."
```

---

### Task 5.5: Rewrite `_create_engine` to register the full T2 tool set

**Files:**
- Modify: `ai-worker/src/api_server.py` — the `_create_engine` function (lines ~121-184)

**Context:** The current `_create_engine` has three things that must go:

1. The `purpose: "Purpose | None" = None` parameter + `Purpose` import — pair_pipeline carryover, A2 is single-agent
2. The `Purpose.REVIEW` vs `Purpose.GENERATE` branch in `system_prompt` — single prompt only
3. The `AsyncMock` fallback when `ModelRouter` fails to initialize — silent mock masks real failures

And must add:

1. A required `workspace_dir: Path` parameter — the tools need to know where to operate
2. Calls to `register_context_tools` + `register_file_tools` + `register_exec_tools` — populates the ToolRegistry with all 14 tools
3. `build_system_prompt(language, workspace_path)` from Task 5.1 — real system prompt

The function signature changes from:
```python
def _create_engine(req: RunRequest, purpose: "Purpose | None" = None) -> Any:
```
to:
```python
def _create_engine(req: RunRequest, workspace_dir: Path) -> Any:
```

- [ ] **Step 1: Read the current `_create_engine` and note what to delete**

```bash
sed -n '121,184p' ai-worker/src/api_server.py
```

- [ ] **Step 2: Replace `_create_engine` with the new implementation**

Edit `ai-worker/src/api_server.py`. Find the function and replace with:

```python
def _create_engine(req: RunRequest, workspace_dir: Path) -> Any:
    """Create a QueryEngine wired with the full T2 tool set.

    Called lazily by _route_and_stream when a new session_id is seen.
    The engine is cached in the LRU session cache (Phase 5 Task 5.6)
    for the lifetime of the session.

    Hard-fails if the model router is unavailable — no AsyncMock
    fallback. A silent mock would hide real auth/config failures
    until the user's message came back with garbage results.
    """
    from pathlib import Path

    from src.openharness.engine.prompts import build_system_prompt
    from src.openharness.engine.query_engine import QueryEngine
    from src.openharness.hooks.executor import HookExecutor
    from src.openharness.hooks.loader import HookRegistry
    from src.openharness.permissions.checker import PermissionChecker
    from src.openharness.permissions.modes import PermissionMode
    from src.openharness.tools import (
        register_exec_tools,
        register_file_tools,
    )
    from src.openharness.tools.base import ToolRegistry
    from src.openharness.tools.context_tools import register_context_tools

    # ModelRouter — required. No AsyncMock fallback.
    try:
        from src.models.router import ModelRouter, Purpose
        from src.openharness.api.providers.router_adapter import ModelRouterAdapter

        router = ModelRouter()
        api_client = ModelRouterAdapter(router, purpose=Purpose.GENERATE)
    except Exception as e:
        # Phase 5 silicon-valley rule: fail fast, no silent mock.
        raise RuntimeError(
            f"ModelRouter unavailable — agent cannot start. "
            f"Check provider credentials and network. Underlying error: {e}"
        ) from e

    # Tool registry — all 14 tools
    tool_registry = ToolRegistry()

    # Context tools (6): profile queries + read_project_file HTTP
    # The profiles dict is empty here because the real profile data
    # is loaded lazily elsewhere; passing an empty dict means the
    # tools return "no data" responses until profile scan populates
    # them. Phase 5 does not wire in live profile data — that's a
    # follow-up when the profile scan pipeline is ready.
    register_context_tools(
        tool_registry,
        profiles={},
        project_id=req.project_id,
    )

    # File tools (6): read/write/edit/glob/grep/list_directory
    register_file_tools(tool_registry, workspace_dir)

    # Exec tools (2): bash + set_phase
    register_exec_tools(tool_registry, workspace_dir)

    # Hooks and permissions
    hook_registry = HookRegistry()
    hook_executor = HookExecutor(hook_registry)
    permission_checker = PermissionChecker(mode=PermissionMode.FULL_AUTO)

    # Model + system prompt
    model = req.model or os.getenv("FORGE_DEFAULT_MODEL", "claude-sonnet-4-20250514")

    if req.system_prompt is not None:
        # Caller override (rare — used by tests and some explicit
        # API callers that want a custom prompt)
        system_prompt = req.system_prompt
    else:
        # Default: build the real Variant B prompt with language
        # detection and the absolute workspace path
        from src.openharness.skills.project_language import (
            detect_language,
            load_all_language_profiles,
        )

        language_name: Optional[str] = None
        try:
            profiles = load_all_language_profiles("skills/languages")
            profile = detect_language(workspace_dir, profiles)
            if profile is not None:
                language_name = profile.name
        except Exception as e:
            logger.warning(
                "language detection failed: %s (proceeding without)",
                e,
            )

        system_prompt = build_system_prompt(
            language=language_name,
            workspace_path=str(workspace_dir),
        )

    return QueryEngine(
        api_client=api_client,
        tool_registry=tool_registry,
        model=model,
        system_prompt=system_prompt,
        hook_executor=hook_executor,
        permission_checker=permission_checker,
        cwd=workspace_dir,
    )
```

- [ ] **Step 3: Verify api_server.py imports still work**

```bash
cd ai-worker && python -c "from src.api_server import _create_engine; print('ok')"
```
Expected: `ok`. Any ImportError means a dependency got lost during the rewrite — debug before proceeding.

- [ ] **Step 4: Run the api_server tests to catch regressions**

```bash
cd ai-worker && python -m pytest tests/test_api_server.py tests/test_api_server_route.py -v 2>&1 | tail -30
```
Expected: most tests either pass or fail in ways that explicitly invoke `_create_engine` with the old signature. For the latter, update those tests to pass `workspace_dir=tmp_path` (or similar).

Tests that mocked `_create_engine` and expected it to accept a `purpose` kwarg will fail — update them to match the new signature.

- [ ] **Step 5: Commit**

```bash
git add ai-worker/src/api_server.py ai-worker/tests/test_api_server.py 2>/dev/null
git add ai-worker/tests/test_api_server_route.py 2>/dev/null
git commit -m "feat(api_server): _create_engine registers full T2 tool surface

Rewrites _create_engine for the A2 architecture:

- Signature: _create_engine(req, workspace_dir: Path) -> QueryEngine
  The old purpose: 'Purpose | None' parameter is deleted along
  with the Purpose.REVIEW system-prompt branch — A2 is
  single-agent, single-purpose, single-prompt.

- Tool registration: all three helpers called in sequence
    register_context_tools(reg, profiles={}, project_id=...)  # 6
    register_file_tools(reg, workspace_dir)                    # 6
    register_exec_tools(reg, workspace_dir)                    # 2
  Total: 14 tools. profiles is empty for now — live profile
  data wiring is a follow-up when the scan pipeline is ready.

- System prompt: build_system_prompt(language, workspace_path)
  from prompts.py. Language auto-detected via existing
  project_language machinery; falls back to 'unknown' on
  detection failure with a WARNING log.

- ModelRouter failure: raises RuntimeError. No AsyncMock fallback.
  Silicon-valley rule: fail fast, don't mask auth/config bugs
  behind fake data that only surfaces at e2e time.

Any existing tests that invoked _create_engine with the old
signature (purpose kwarg, no workspace_dir) are updated to the
new signature. Tests that relied on the AsyncMock fallback for
ModelRouter failures now need to mock ModelRouter directly."
```

---

### Task 5.6: Simplify `_route_and_stream`, wire `LRUSessionCache`, add `PhaseChanged` serialization

**Files:**
- Modify: `ai-worker/src/api_server.py` — `_sessions` declaration, `_route_and_stream`, `_serialize_event`

**Context:** Four changes to `api_server.py`:

1. Replace `_sessions: Dict[str, Any] = {}` with `_sessions = LRUSessionCache(max_size=100)`
2. Rewrite `_route_and_stream` to delete the pair_pipeline branch and simplify to a single QueryEngine path (spec §5.7 shows the target shape)
3. Delete the guarded `try/except` import of `pair_pipeline` at the top of the file — pair_pipeline is gone (Phase 0 deleted the file); the guarded import just adds complexity now
4. Update `_serialize_event` to add a `phase_changed` branch, add `tool_use_id` to `tool_started`/`tool_completed`, and delete the `FixLoopStarted`/`FixLoopCompleted` branches (which will fail to import now that the classes are gone from Phase 4 Task 4.1)

- [ ] **Step 1: Delete the guarded pair_pipeline import**

Open `ai-worker/src/api_server.py`. Find the top of the file around line 31-47:

```python
try:
    from src.openharness.engine.pair_pipeline import (
        PairPipelineConfig,
        run_pair_pipeline,
    )
    from src.models.router import Purpose
    _PAIR_PIPELINE_AVAILABLE = True
except Exception as e:
    logging.getLogger(__name__).error(
        "pair_pipeline imports failed at startup — pair_pipeline route "
        "will return 503; falling back to QueryEngine for all requests: %s",
        e,
    )
    PairPipelineConfig = None  # type: ignore
    run_pair_pipeline = None  # type: ignore
    Purpose = None  # type: ignore
    _PAIR_PIPELINE_AVAILABLE = False
```

Delete this entire block. Replace with nothing — the imports it used are either gone (pair_pipeline) or moved into `_create_engine` (Purpose, lazy-imported in Task 5.5).

- [ ] **Step 2: Replace `_sessions` with LRUSessionCache**

Find `_sessions: Dict[str, Any] = {}` (line ~54) and replace with:

```python
# LRU-bounded session cache (spec §5.8, implemented in Phase 5 Task 5.4)
from src.openharness.engine.session_cache import LRUSessionCache

_sessions = LRUSessionCache(max_size=100)
```

Then find the `delete_session` handler:

```python
@app.delete("/api/sessions/{session_id}")
async def delete_session(session_id: str) -> JSONResponse:
    engine = _sessions.pop(session_id, None)
    if engine is None:
        raise HTTPException(status_code=404, detail="Session not found")
    engine.clear()
    return JSONResponse({"status": "deleted"})
```

Update the `.pop(session_id, None)` call — the LRU cache's `pop` signature doesn't take a default (it always returns `None` for missing keys):

```python
@app.delete("/api/sessions/{session_id}")
async def delete_session(session_id: str) -> JSONResponse:
    engine = _sessions.pop(session_id)
    if engine is None:
        raise HTTPException(status_code=404, detail="Session not found")
    engine.clear()
    return JSONResponse({"status": "deleted"})
```

And update the health endpoint to use `len(_sessions)`:

```python
@app.get("/health")
async def health() -> Dict[str, Any]:
    return {
        "status": "ok",
        "sessions": len(_sessions),
        "version": "1.0.0",
    }
```

`len()` already works because `LRUSessionCache.__len__` is defined.

- [ ] **Step 3: Rewrite `_route_and_stream`**

Find the current function (lines ~187-266) and replace with:

```python
async def _route_and_stream(
    req: RunRequest,
    session_id: str,
    correlation_id: str,
) -> AsyncIterator[Any]:
    """Route a chat message to the agent loop and yield stream events.

    The pair_pipeline fork is deleted in A2 — all requests go through
    a single QueryEngine path. workspace_path is required; missing
    workspace is a 400.

    Engine lookup/creation is lazy: first message in a session
    creates the engine and inserts into the LRU cache. Subsequent
    messages reuse the cached engine (conversation continuity across
    messages within a session).
    """
    from pathlib import Path

    if not req.workspace_path:
        # workspace_path is required in A2 — no QueryEngine can run
        # without knowing where the project is on disk
        raise HTTPException(
            status_code=400,
            detail="workspace_path is required (Phase 5+ A2 architecture)",
        )

    ws_root = os.environ.get("FORGE_WORKSPACE_ROOT", "/data/forge/workspaces")
    resolved_workspace = Path(os.path.join(ws_root, req.workspace_path))

    # Defensive check. In normal operation, forge-core's
    # workspace.Manager.EnsureReady (Phase 1) has already created
    # this directory before submitting the RunRequest — so if it's
    # missing here, that's an operational bug (mount not set up,
    # race condition between forge-core commit and ai-worker read).
    # Return 500 and log loudly; ai-worker does NOT create workspaces
    # itself.
    if not resolved_workspace.is_dir():
        logger.error(
            "workspace_path %r resolved to %r but directory does not exist "
            "— forge-core should have called EnsureReady first. Check "
            "docker volume mount + FORGE_WORKSPACE_ROOT env.",
            req.workspace_path,
            str(resolved_workspace),
        )
        raise HTTPException(
            status_code=500,
            detail=f"workspace not ready: {resolved_workspace}",
        )

    # Get or create the engine
    engine = _sessions.get(session_id)
    if engine is None:
        try:
            engine = _create_engine(req, workspace_dir=resolved_workspace)
        except RuntimeError as e:
            # ModelRouter failure, etc. — surface as a stream error
            # rather than a transport error so the frontend can show
            # the message in the chat UI.
            from src.openharness.engine.stream_events import ErrorEvent
            yield ErrorEvent(message=str(e), recoverable=False)
            return
        _sessions.put(session_id, engine)

    async for event in engine.submit_message(req.message):
        yield event
```

- [ ] **Step 4: Update `_serialize_event`**

Find `_serialize_event` (line ~408) and update the branches:

```python
def _serialize_event(event: Any, correlation_id: str) -> Dict[str, str]:
    """Serialize a StreamEvent to a flat dict for Redis Streams."""
    from src.openharness.engine.stream_events import (
        AssistantTextDelta,
        AssistantTurnComplete,
        ErrorEvent,
        PhaseChanged,
        SessionComplete,
        ThinkingStarted,
        ThinkingStopped,
        ToolExecutionCompleted,
        ToolExecutionStarted,
    )

    base = {"correlation_id": correlation_id}

    if isinstance(event, AssistantTextDelta):
        base["type"] = "text_delta"
        base["text"] = event.text
    elif isinstance(event, AssistantTurnComplete):
        base["type"] = "turn_complete"
        base["text"] = event.message.text
        base["input_tokens"] = str(event.usage.input_tokens)
        base["output_tokens"] = str(event.usage.output_tokens)
    elif isinstance(event, ToolExecutionStarted):
        base["type"] = "tool_started"
        base["tool_use_id"] = event.tool_use_id
        base["tool_name"] = event.tool_name
        base["tool_input"] = json.dumps(event.tool_input, default=str)
    elif isinstance(event, ToolExecutionCompleted):
        base["type"] = "tool_completed"
        base["tool_use_id"] = event.tool_use_id
        base["tool_name"] = event.tool_name
        base["output"] = event.output[:4000]  # Truncate for Redis field size
        base["is_error"] = str(event.is_error)
    elif isinstance(event, PhaseChanged):
        base["type"] = "phase_changed"
        base["phase"] = event.phase
    elif isinstance(event, ErrorEvent):
        base["type"] = "error"
        base["message"] = event.message
        base["recoverable"] = str(event.recoverable)
    elif isinstance(event, ThinkingStarted):
        base["type"] = "thinking_started"
        base["label"] = event.label
    elif isinstance(event, ThinkingStopped):
        base["type"] = "thinking_stopped"
    elif isinstance(event, SessionComplete):
        base["type"] = "session_complete"
        base["files_created"] = str(event.files_created)
        base["files_modified"] = str(event.files_modified)
        base["build_status"] = event.build_status
        base["duration_ms"] = str(event.duration_ms)
        base["tokens_total"] = str(event.tokens_total)
        base["cost_usd"] = f"{event.cost_usd:.4f}"
    else:
        base["type"] = "unknown"
        base["data"] = str(event)

    return base
```

Changes from the old version:
- Deleted `FixLoopStarted` and `FixLoopCompleted` branches and imports
- Added `PhaseChanged` branch and import
- Added `tool_use_id` field to `tool_started` and `tool_completed` serialization

- [ ] **Step 5: Smoke-test the module imports**

```bash
cd ai-worker && python -c "from src.api_server import app, _create_engine, _route_and_stream, _serialize_event; print('ok')"
```
Expected: `ok`. Any ImportError means the FixLoop cleanup or PhaseChanged addition missed something — debug.

- [ ] **Step 6: Run api_server tests**

```bash
cd ai-worker && python -m pytest tests/test_api_server.py tests/test_api_server_route.py -v 2>&1 | tail -40
```
Expected: the remaining tests pass. Tests that exercised the pair_pipeline route need to be deleted or rewritten to the single-path shape.

- [ ] **Step 7: Commit**

```bash
git add ai-worker/src/api_server.py ai-worker/tests/test_api_server_route.py 2>/dev/null
git commit -m "feat(api_server): simplify _route_and_stream + add phase_changed serialization

Four coordinated changes to api_server.py:

1. Delete the guarded pair_pipeline import block. pair_pipeline
   is gone (Phase 0 deleted the module); the guarded try/except
   that fell through to _PAIR_PIPELINE_AVAILABLE=False was a
   Stream-4c transitional artifact and no longer serves anything.

2. Replace _sessions: Dict[str, Any] = {} with
   _sessions = LRUSessionCache(max_size=100) from Phase 5 Task 5.4.
   The delete_session and health handlers are updated to use the
   new interface (pop() without default, len() still works).

3. Rewrite _route_and_stream to a single QueryEngine path:
   - workspace_path is required (400 if missing)
   - defensive is_dir() check — forge-core EnsureReady should
     have created the workspace before POSTing; missing workspace
     is an operational bug (500 + loud log)
   - ModelRouter failures in _create_engine are caught and
     surfaced as an ErrorEvent in the stream (not a transport
     error) so the frontend can show them in the chat UI
   - the pair_pipeline vs legacy QueryEngine fork is deleted

4. Update _serialize_event:
   - delete FixLoopStarted / FixLoopCompleted branches (classes
     are gone from Phase 4 Task 4.1)
   - add PhaseChanged branch mapping to event_type='phase_changed'
     with a 'phase' field
   - add tool_use_id to tool_started and tool_completed Redis
     payloads for Phase 6 frontend correlation"
```

---

### Task 5.7: `/api/workspace/prep` endpoint

**Files:**
- Modify: `ai-worker/src/api_server.py` — append new route handler

**Context:** Forge-core's `PrepClient` (Phase 1 Task 1.5) posts to `/api/workspace/prep` to trigger language-specific dependency install in the ai-worker container (which has network + toolchains, unlike the bwrap bash sandbox). The handler:

1. Accepts `{tenant_id, project_id, workspace_path}` in the JSON body
2. Resolves the absolute workspace path via `FORGE_WORKSPACE_ROOT + workspace_path`
3. Detects the language via existing `project_language.detect_language`
4. Looks up the language profile's `prep_command` (e.g., `go mod download`, `mvn dependency:go-offline -B`)
5. Runs the prep command as a subprocess (NOT in bwrap — we need network here)
6. Returns a structured JSON response matching `PrepResult` in Phase 1 Task 1.5

Timeout: 10 minutes. Beyond that the prep is hung; kill the subprocess and return `{status: "error", error: "timeout"}`. The forge-core caller treats this as a soft failure and marks the workspace ready anyway (spec §3.9).

- [ ] **Step 1: Grep for the language profile schema**

```bash
grep -n "prep_command\|build_command\|LanguageProfile" ai-worker/src/openharness/skills/project_language.py | head
```
Expected: find the `LanguageProfile` dataclass and its `prep_command` attribute. The existing Stream 4c work set this up for pair_pipeline's build verification; we're reusing it here.

If `prep_command` doesn't exist on `LanguageProfile` but `build_command` does, use that. If neither exists, fall back to a hard-coded map of `language_name → prep_command` (acceptable for Phase 5; a follow-up can move it to the YAML profiles).

- [ ] **Step 2: Add the `/api/workspace/prep` handler to `api_server.py`**

Find a good spot (just before `async def _route_and_stream` around line 187) and insert:

```python
# ---------------------------------------------------------------------------
# Workspace prep — dependency pre-install for the A2 architecture.
# Called by forge-core's workspace.Manager.EnsureReady (Phase 1) after
# a fresh clone. We run go mod download / mvn dependency:go-offline /
# npm ci / etc. in the ai-worker container (which has network +
# language toolchains, unlike the bwrap sandbox). Spec §3.9.
# ---------------------------------------------------------------------------


class PrepRequest(BaseModel):
    tenant_id: int
    project_id: int
    workspace_path: str  # relative to FORGE_WORKSPACE_ROOT


class PrepResponse(BaseModel):
    status: str  # "ok" | "skipped" | "error"
    language: Optional[str] = None
    command: Optional[str] = None
    error: Optional[str] = None
    reason: Optional[str] = None


# Fallback prep commands by language name. Used when the
# LanguageProfile doesn't expose a prep_command attribute.
_FALLBACK_PREP_COMMANDS = {
    "go": "go mod download",
    "python": "pip install -r requirements.txt",
    "java": "mvn dependency:go-offline -B",
    "javascript": "npm ci",
    "typescript": "npm ci",
    "rust": "cargo fetch",
}

PREP_TIMEOUT_SECONDS = 600  # 10 minutes


@app.post("/api/workspace/prep", response_model=PrepResponse)
async def workspace_prep(req: PrepRequest) -> PrepResponse:
    """Run language-specific dependency pre-install for a workspace.

    Called by forge-core after a fresh clone (Phase 1 Task 1.5).
    Runs OUTSIDE the bash sandbox because we need network access
    and the language toolchains here — the ai-worker container
    has all of them, the bwrap sandbox does not.

    Returns:
        - status="ok" with language+command on success
        - status="skipped" with reason if language detection failed
          or no prep command is known for the detected language
        - status="error" with the error message on command failure
    """
    import asyncio
    from pathlib import Path

    from src.openharness.skills.project_language import (
        detect_language,
        load_all_language_profiles,
    )

    ws_root = os.environ.get("FORGE_WORKSPACE_ROOT", "/data/forge/workspaces")
    workspace_dir = Path(os.path.join(ws_root, req.workspace_path))

    if not workspace_dir.is_dir():
        return PrepResponse(
            status="error",
            error=f"workspace directory does not exist: {workspace_dir}",
        )

    # Detect language
    try:
        profiles = load_all_language_profiles("skills/languages")
        profile = detect_language(workspace_dir, profiles)
    except Exception as e:
        logger.warning("workspace prep: language detection failed: %s", e)
        return PrepResponse(
            status="skipped",
            reason=f"language detection error: {e}",
        )

    if profile is None:
        return PrepResponse(
            status="skipped",
            reason="no language detected; agent will see dependency errors if any",
        )

    # Resolve prep command: prefer profile.prep_command if it exists,
    # else fall back to the hard-coded map
    prep_cmd = getattr(profile, "prep_command", None)
    if not prep_cmd:
        prep_cmd = _FALLBACK_PREP_COMMANDS.get(profile.name.lower())
    if not prep_cmd:
        return PrepResponse(
            status="skipped",
            language=profile.name,
            reason=(
                f"language '{profile.name}' detected but no prep command known; "
                "agent will see dependency errors if any"
            ),
        )

    # Run the prep command — NOT in bwrap. This needs network and
    # the toolchains installed in the ai-worker container image.
    logger.info(
        "workspace prep: running %r in %s (language=%s)",
        prep_cmd,
        workspace_dir,
        profile.name,
    )
    try:
        proc = await asyncio.create_subprocess_shell(
            prep_cmd,
            cwd=str(workspace_dir),
            stdout=asyncio.subprocess.PIPE,
            stderr=asyncio.subprocess.STDOUT,
        )
        stdout, _ = await asyncio.wait_for(
            proc.communicate(), timeout=PREP_TIMEOUT_SECONDS
        )
    except asyncio.TimeoutError:
        try:
            proc.kill()
        except ProcessLookupError:
            pass
        return PrepResponse(
            status="error",
            language=profile.name,
            command=prep_cmd,
            error=f"prep command timed out after {PREP_TIMEOUT_SECONDS} seconds",
        )
    except Exception as e:
        return PrepResponse(
            status="error",
            language=profile.name,
            command=prep_cmd,
            error=f"prep command failed to start: {e}",
        )

    if proc.returncode != 0:
        tail = stdout.decode("utf-8", errors="replace")[-1000:]
        return PrepResponse(
            status="error",
            language=profile.name,
            command=prep_cmd,
            error=f"prep command exited {proc.returncode}: ...{tail}",
        )

    logger.info("workspace prep: %s ok (language=%s)", prep_cmd, profile.name)
    return PrepResponse(
        status="ok",
        language=profile.name,
        command=prep_cmd,
    )
```

- [ ] **Step 3: Smoke-test the endpoint**

Start the ai-worker container (or dev server) and hit the endpoint:

```bash
# In a live container
curl -s -X POST http://localhost:8090/api/workspace/prep \
  -H "Content-Type: application/json" \
  -d '{"tenant_id":1,"project_id":1,"workspace_path":"nonexistent"}'
```
Expected JSON: `{"status":"error","error":"workspace directory does not exist: /data/forge/workspaces/nonexistent"}` or similar.

Then point it at a real workspace (one you created by hand) to verify the happy path.

- [ ] **Step 4: Write a unit test with a mock workspace**

Add to `ai-worker/tests/test_api_server.py` (or create a new file if the existing one is too tangled):

```python
import pytest
from fastapi.testclient import TestClient


@pytest.fixture
def client():
    from src.api_server import app
    return TestClient(app)


def test_workspace_prep_missing_workspace_returns_error(client, monkeypatch, tmp_path):
    monkeypatch.setenv("FORGE_WORKSPACE_ROOT", str(tmp_path))

    resp = client.post(
        "/api/workspace/prep",
        json={
            "tenant_id": 1,
            "project_id": 1,
            "workspace_path": "does-not-exist",
        },
    )
    assert resp.status_code == 200  # The handler returns ok-with-error-body, not 404
    body = resp.json()
    assert body["status"] == "error"
    assert "does not exist" in body["error"]


def test_workspace_prep_unknown_language_returns_skipped(client, monkeypatch, tmp_path):
    monkeypatch.setenv("FORGE_WORKSPACE_ROOT", str(tmp_path))

    # Create an empty workspace — no language markers (no go.mod, no
    # package.json, no requirements.txt)
    ws = tmp_path / "empty"
    ws.mkdir()
    (ws / "README.md").write_text("just a readme")

    resp = client.post(
        "/api/workspace/prep",
        json={
            "tenant_id": 1,
            "project_id": 1,
            "workspace_path": "empty",
        },
    )
    body = resp.json()
    assert body["status"] == "skipped"
```

- [ ] **Step 5: Run the new tests**

```bash
cd ai-worker && python -m pytest tests/test_api_server.py -v -k workspace_prep
```
Expected: both tests pass.

- [ ] **Step 6: Commit**

```bash
git add ai-worker/src/api_server.py ai-worker/tests/test_api_server.py
git commit -m "feat(api_server): POST /api/workspace/prep endpoint

Dependency pre-install for fresh workspaces. Called by forge-core's
workspace.Manager.EnsureReady (Phase 1 Task 1.5) after a clone.
Runs OUTSIDE the bash sandbox because we need network access and
the language toolchains here — the ai-worker container has them,
the bwrap sandbox does not.

Request: {tenant_id, project_id, workspace_path}
Response: PrepResponse with status 'ok' | 'skipped' | 'error' plus
          language, command, reason/error fields.

Flow:
1. Resolve absolute workspace dir via FORGE_WORKSPACE_ROOT
2. detect_language via existing project_language machinery
3. Look up prep_command from LanguageProfile (or fallback map
   for go/python/java/js/ts/rust)
4. Run prep as a plain subprocess with 10-minute timeout
5. Return structured response

Timeout behavior: SIGKILL the subprocess and return
status='error', error='... timed out ...'. The forge-core
caller treats this as a soft failure and marks the workspace
ready anyway (spec §3.9 non-blocking soft failure rule).

_FALLBACK_PREP_COMMANDS is a hard-coded map that kicks in when
the language YAML profile doesn't expose a prep_command field.
Future cleanup: move all prep commands into the YAML profiles
and delete the fallback.

Tests: missing workspace -> error with message, unknown language
-> skipped. Real-prep success is exercised by the Phase 7 e2e."
```

---

## Phase 5 completion check

Before starting Phase 6:

- [ ] `pytest ai-worker/tests/openharness/engine/test_prompts.py -v` — 10 tests pass
- [ ] `pytest ai-worker/tests/openharness/engine/test_session_collector.py -v` — 34 tests pass (24 parametrized + 10 SessionCollector)
- [ ] `pytest ai-worker/tests/openharness/engine/test_session_cache.py -v` — 10 tests pass
- [ ] `pytest ai-worker/tests/test_query_engine.py -v` — passes with SessionComplete integration tests
- [ ] `pytest ai-worker/tests/test_api_server.py tests/test_api_server_route.py -v` — existing tests pass + workspace_prep tests pass
- [ ] `python -c "from src.api_server import app, _create_engine, _route_and_stream, _serialize_event; print('ok')"` prints `ok`
- [ ] `grep -n "pair_pipeline" ai-worker/src/api_server.py` returns nothing
- [ ] `grep -n "Purpose.REVIEW\|Purpose.GENERATE" ai-worker/src/api_server.py` returns nothing
- [ ] `grep -n "AsyncMock" ai-worker/src/api_server.py` returns nothing
- [ ] `grep -n "_PAIR_PIPELINE_AVAILABLE\|_sessions: Dict" ai-worker/src/api_server.py` returns nothing (both deleted)
- [ ] `grep -n "FixLoop" ai-worker/src/api_server.py` returns nothing (stale _serialize_event branches purged)
- [ ] `curl -X POST http://localhost:8090/api/workspace/prep -d '{"tenant_id":1,"project_id":1,"workspace_path":"nonexistent"}' -H "Content-Type: application/json"` returns `{"status":"error", ...}` JSON
- [ ] Branch has **7 new commits** from this phase (one per task)

## Phase 5 outputs unlock

- **Phase 6** (frontend) has a full agent backend to wire SSE against. The Redis stream now carries `text_delta`, `turn_complete`, `tool_started`/`tool_completed` with `tool_use_id`, `phase_changed`, `thinking_started`/`stopped`, `session_complete`, and `error` events. Nothing else is expected.
- **Phase 7** (e2e + deploy) can run a real agent message end-to-end: forge-core's EnsureReady creates a workspace, calls the prep endpoint (which ai-worker now serves), then forge-core forwards the user message → api_server._route_and_stream → QueryEngine → agent loop → tools → SSE → frontend.
- Phase 5 is the last phase that touches ai-worker's Python code. Phase 6 is frontend-only and Phase 7 is integration/deploy.
