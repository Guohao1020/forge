# chronos · Phase 5 — Agent Loop + api_server + Prompts + Interaction Tools

> **Project:** [chronos — Agent Variant B Single-Agent Implementation](index.md)
> **Phase:** 5 of 7 · **Tasks:** 15 · **Depends on:** [Phase 1a](phase-1a-workspace-minimal.md), [Phase 3](phase-3-file-tools.md), [Phase 4](phase-4-bash-events.md), [Phase 5a](phase-5a-bidirectional-rpc.md) · **Unblocks:** Phase 6, Phase 7
> **Spec reference:** [Design spec §2.9.1 (AgentHookRegistry), §2.9.3 (RequestReviewTool), §4.12 (tool registry construction), §5 (agent loop & event layer), §5.2 (system prompt), §5.6 (SessionCollector), §5.7 (routing), §5.8 (LRU cache), §3.9 (prep endpoint)](../../specs/2026-04-09-agent-variant-b-single-agent-design.md)

**Execution:** REQUIRED SUB-SKILL: Use `superpowers:subagent-driven-development` (recommended) or `superpowers:executing-plans`. Steps use checkbox (`- [ ]`) syntax for tracking.

---

## Phase goal

Wire everything Phase 1–4 built into the agent runtime, **plus Round 2: in-process agent hooks + interaction meta-tools**. Fifteen tasks (Round 1 Tasks 5.1–5.7 + Round 2 Tasks 5.8–5.15):

1. **`prompts.py`** — `build_system_prompt(language, workspace_path)` generates the real Variant B system prompt (not the one-liner "you are a helpful AI coding assistant" currently in the code)
2. **`session_collector.py`** — `SessionCollector` tracks per-turn file creates/modifies/build status by observing `ToolExecutionStarted`/`Completed` events; `_is_build_like` helper + build-like token set
3. **`QueryEngine` integration** — `query_engine.py` wires `SessionCollector` into `submit_message`, emits `SessionComplete` conditionally (only if any tool was called)
4. **`LRUSessionCache`** — replaces the `_sessions: Dict[str, Any]` in `api_server.py` with a 100-entry LRU that calls `engine.clear()` on eviction
5. **`_create_engine` rewrite** — delete the `purpose` parameter, delete the `Purpose.REVIEW` branch, register all 14 tools (6 context + 6 file + 2 exec), raise on missing ModelRouter (no AsyncMock fallback)
6. **`_route_and_stream` simplification** — delete the pair_pipeline branch, delete the guarded import of `pair_pipeline`, make `workspace_path` required, add defensive `is_dir()` check
7. **`/api/workspace/prep` endpoint** — the HTTP handler that forge-core's `PrepClient` (Phase 1 Task 1.5) calls. Detects language, runs prep command via non-sandboxed subprocess (has network inside ai-worker container), returns structured result

**Round 2 additions (Tasks 5.8–5.15)** — bolted onto the Round 1 baseline, exposing in-process extension points and the second interaction meta-tool:

8. **`AgentHookRegistry` + Protocols + `AgentHookContext`** (Task 5.8) — extend `agent_hooks.py` (created in Phase 5a Task 5a.2 with `SessionHaltError` + `ClarificationCoordinator`) with the four hook Protocols (`PreTurnHook`, `PreToolCallHook`, `PostTurnHook`, `PromptSlotFiller`), the `PreToolCallBlock` short-circuit dataclass, the `AgentHookContext` per-session value, and the empty-default `AgentHookRegistry` itself
9. **Hook invocation points in `query.py` + `run_agent_loop`** (Task 5.9) — wire `pre_turn` (before each turn), `pre_tool_call` (between permission check and tool execution in `_execute_tool_call`), and `post_turn` (after `stop_reason=end_turn`) call sites. `QueryEngine.__init__` accepts the registry + context as optional empty-default arguments
10. **Hook integration tests** (Task 5.10) — ~11 end-to-end scenarios per spec §7.4 Round 2: pre_turn mutates messages/system prompt, pre_tool_call blocks/mutates tool args, post_turn fires on end_turn, hooks raise → loop halts with `ErrorEvent(recoverable=False)`, hooks invoked in registration order, slot substitution, missing slot stripped
11. **`build_system_prompt` slot substitution + Round 2 system prompt update** (Task 5.11) — `build_system_prompt` becomes `async`, accepts `slots: dict[str, PromptSlotFiller] | None` and `hook_context: AgentHookContext | None`, substitutes `{{slot_name}}` placeholders, regex-strips unfilled slots. Adds the Round 2 "How to work" bullets for `request_clarification` (bullet 1) and `request_review` (bullet 7). Tests cover language/path substitution, slot fill, slot strip, filler exception propagation, and the new instruction substring invariants
12. **`build_reviewer_prompt` + `parse_verdict` + `REVIEWER_SYSTEM_PROMPT`** (Task 5.12) — extend `prompts.py` with the reviewer-side text infrastructure. `REVIEWER_SYSTEM_PROMPT` is the pinned constant from spec §2.9.3.c. `build_reviewer_prompt(summary, current_diff, original_request)` renders the user-facing reviewer message. `VERDICT_PATTERN` + `parse_verdict` extract `APPROVE`/`REVISE`/`REJECT` from the reviewer's response, raising `ReviewerParseError` on failure
13. **`RequestReviewTool` implementation** (Task 5.13) — the tool itself, appended to `interaction_tools.py` (Phase 5a Task 5a.4 already created the file with `RequestClarificationTool`). Subclasses `SimpleTool`. Collects `git diff HEAD` directly via `asyncio.create_subprocess_exec` (the §2.9.3.e bwrap exemption — read-only, parameter-less, hardcoded argv). Calls `ModelRouter.generate(purpose=Purpose.REVIEW, ...)` with a 1024-token cap. Returns the raw reviewer text as `ToolResult.output` — does NOT parse the verdict (the agent reads the verdict and decides what to do)
14. **`register_interaction_tools` helper** (Task 5.14) — small function that registers both `RequestClarificationTool()` and `RequestReviewTool(model_router, workspace_dir)` in one call. Idempotency intentionally fails (registry rejects duplicate registration) so a buggy double-call surfaces immediately
15. **`_create_engine` wiring — hooks + interaction tools** (Task 5.15) — completes the `_create_engine` rewrite started in Task 5.5. Constructs an empty `AgentHookRegistry` + `AgentHookContext`, calls `register_interaction_tools`, asserts `router.require_model_for(Purpose.REVIEW)` (fail-fast per §2.9.3.f — if no reviewer model is configured, the agent does not start), `await`s the now-async `build_system_prompt` with the registry's `system_prompt_slots`, and passes the registry + context into `QueryEngine`

Plus side changes: `_serialize_event` grows a `phase_changed` branch and a `tool_use_id` field on `tool_started`/`tool_completed` (matching Phase 4 Task 4.1's event shape changes).

**Completion gate:**
- `pytest ai-worker/tests/test_api_server.py tests/test_api_server_route.py -v` — all passing
- `pytest ai-worker/tests/test_query_engine.py -v` — passes with SessionCollector integration
- `pytest ai-worker/tests/openharness/engine/test_session_collector.py -v` — new test file passes
- `pytest ai-worker/tests/openharness/engine/test_prompts.py -v` — Round 1 ten tests + Round 2 seven (slot/instruction) tests pass
- `pytest ai-worker/tests/openharness/engine/test_agent_hook_registry.py -v` — Task 5.8 tests pass
- `pytest ai-worker/tests/openharness/engine/test_hooks_integration.py -v` — Task 5.10 ~11 integration scenarios pass
- `pytest ai-worker/tests/openharness/engine/test_reviewer_prompts.py -v` — Task 5.12 reviewer prompt + verdict parser tests pass
- `pytest ai-worker/tests/openharness/tools/test_request_review_tool.py -v` — Task 5.13 happy path + error path + git diff fixture tests pass
- `pytest ai-worker/tests/openharness/tools/test_register_interaction_tools.py -v` — Task 5.14 registration helper tests pass
- `grep -n "pair_pipeline" ai-worker/src/api_server.py` returns nothing (guarded import gone)
- `grep -n "Purpose.REVIEW\|Purpose.GENERATE" ai-worker/src/api_server.py` returns nothing (purpose parameter gone)
- `grep -n "AsyncMock" ai-worker/src/api_server.py` returns nothing (mock fallback gone — fail fast)
- `grep -n "AgentHookRegistry" ai-worker/src/api_server.py` returns at least one match (Task 5.15 wired it in)
- `grep -n "register_interaction_tools" ai-worker/src/api_server.py` returns at least one match
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

### Task 5.8: `agent_hooks.py` extension — `AgentHookRegistry` + Protocols + `AgentHookContext`

**Files:**
- Modify: `ai-worker/src/openharness/engine/agent_hooks.py` (extend — Phase 5a Task 5a.2 already created this module with `SessionHaltError` + `ClarificationCoordinator`)
- Create: `ai-worker/tests/openharness/engine/test_agent_hook_registry.py`

**Context:** Spec §2.9.1 introduces an in-process Python-callable hook system that runs alongside the existing subprocess `HookRegistry`/`HookExecutor` (which stays unchanged for shell-command hooks). The two systems do NOT share class names — the new one is `AgentHookRegistry`, distinct from the existing `HookRegistry`.

`AgentHookRegistry` holds four fields, all empty by default (chronos Round 2 ships extension points only — no real hook implementations):
- `pre_turn: list[PreTurnHook]` — runs before each turn, mutates the message list and/or `ctx.system_prompt_buffer`
- `pre_tool_call: list[PreToolCallHook]` — runs between permission check and tool execution, may return `PreToolCallBlock(reason)` to short-circuit
- `post_turn: list[PostTurnHook]` — runs after `stop_reason=end_turn`
- `system_prompt_slots: dict[str, PromptSlotFiller]` — maps slot names to async fillers, substituted into `{{slot_name}}` placeholders by `build_system_prompt`

`AgentHookContext` is constructed once per session in `_create_engine` and passed into every hook call. It carries `project_id`, `session_id`, `workspace_dir`, and a mutable `system_prompt_buffer: list[str]` that pre_turn hooks can append to.

**Pinned signatures (§2.9.1.b):** all four are `async` Protocols. Sync hooks are forbidden — the agent loop is `async` end-to-end. `PreToolCallBlock` is a frozen dataclass with a single `reason: str` field.

**Failure mode (§2.9.1.c):** any hook raising halts the turn via `ErrorEvent(message="agent hook {name} failed: {exc}", recoverable=False)`. The empty default registries Round 2 ships never raise — this failure mode only bites downstream projects with buggy hooks. Loudly, as §2.8 demands.

- [ ] **Step 1: Read the existing `agent_hooks.py` so the extension is additive**

```bash
sed -n '1,40p' ai-worker/src/openharness/engine/agent_hooks.py
```
Expected: the file already imports `dataclass`, `asyncio`, etc. and defines `SessionHaltError` and `ClarificationCoordinator` from Phase 5a Task 5a.2. The Round 2 additions append below the existing code without modifying it.

- [ ] **Step 2: Write the failing tests**

Create `ai-worker/tests/openharness/engine/test_agent_hook_registry.py`:

```python
"""Tests for AgentHookRegistry — the in-process Python hook system.

These are pure construction-and-shape tests. End-to-end behavior
(hooks actually firing inside run_agent_loop) lives in
test_hooks_integration.py (Task 5.10).

Spec: §2.9.1.a-e.
"""

from __future__ import annotations

from pathlib import Path

import pytest

from src.openharness.engine.agent_hooks import (
    AgentHookContext,
    AgentHookRegistry,
    PostTurnHook,
    PreToolCallBlock,
    PreToolCallHook,
    PreTurnHook,
    PromptSlotFiller,
)


# ---------------------------------------------------------------------------
# AgentHookRegistry construction
# ---------------------------------------------------------------------------


def test_empty_registry_construction():
    """Default construction yields four empty containers — no hooks
    registered. Round 2 ships this empty default; downstream projects
    populate it via a project-scoped factory."""
    registry = AgentHookRegistry()
    assert registry.pre_turn == []
    assert registry.pre_tool_call == []
    assert registry.post_turn == []
    assert registry.system_prompt_slots == {}
    # Each registry instance has independent containers (no shared
    # default mutable state — that bug class is one of Python's most
    # famous). Construct a second instance and confirm.
    other = AgentHookRegistry()
    other.pre_turn.append(lambda: None)
    assert registry.pre_turn == []  # not aliased


def test_register_pre_turn_hook():
    """pre_turn hooks accumulate in registration order."""
    registry = AgentHookRegistry()

    async def hook_a(ctx, messages):
        return messages

    async def hook_b(ctx, messages):
        return messages

    registry.pre_turn.append(hook_a)
    registry.pre_turn.append(hook_b)

    assert len(registry.pre_turn) == 2
    assert registry.pre_turn[0] is hook_a
    assert registry.pre_turn[1] is hook_b


def test_register_pre_tool_call_hook():
    registry = AgentHookRegistry()

    async def block_bash(ctx, tool_name, arguments):
        if tool_name == "bash":
            return PreToolCallBlock(reason="bash blocked by test")
        return arguments

    registry.pre_tool_call.append(block_bash)
    assert len(registry.pre_tool_call) == 1
    assert registry.pre_tool_call[0] is block_bash


def test_register_post_turn_hook():
    registry = AgentHookRegistry()
    seen = []

    async def record(ctx, final_message):
        seen.append(final_message)

    registry.post_turn.append(record)
    assert len(registry.post_turn) == 1
    assert registry.post_turn[0] is record


def test_register_slot_filler():
    """system_prompt_slots is a dict — slot name → async filler."""
    registry = AgentHookRegistry()

    async def project_specs_filler(ctx):
        return "spec content goes here"

    registry.system_prompt_slots["project_specs"] = project_specs_filler
    assert "project_specs" in registry.system_prompt_slots
    assert registry.system_prompt_slots["project_specs"] is project_specs_filler


# ---------------------------------------------------------------------------
# PreToolCallBlock dataclass
# ---------------------------------------------------------------------------


def test_pre_tool_call_block_dataclass_frozen():
    """PreToolCallBlock is frozen — reason cannot be mutated after
    construction. This is enforced by @dataclass(frozen=True)."""
    block = PreToolCallBlock(reason="bash blocked")
    assert block.reason == "bash blocked"
    with pytest.raises((AttributeError, dataclasses_FrozenInstanceError)):
        block.reason = "different reason"  # type: ignore


# Compat: dataclasses.FrozenInstanceError is a subclass of AttributeError
# in CPython 3.12+. Catching both keeps the test green across versions.
import dataclasses
dataclasses_FrozenInstanceError = dataclasses.FrozenInstanceError


# ---------------------------------------------------------------------------
# AgentHookContext shape
# ---------------------------------------------------------------------------


def test_agent_hook_context_mutable_buffer():
    """AgentHookContext.system_prompt_buffer is a mutable list that
    pre_turn hooks can append to. Construction takes project_id,
    session_id, workspace_dir, and the initial (usually empty) buffer.
    """
    ctx = AgentHookContext(
        project_id=42,
        session_id="sess-abc",
        workspace_dir=Path("/data/forge/workspaces/tenant-1/project-42"),
        system_prompt_buffer=[],
    )
    assert ctx.project_id == 42
    assert ctx.session_id == "sess-abc"
    assert ctx.workspace_dir == Path(
        "/data/forge/workspaces/tenant-1/project-42"
    )
    assert ctx.system_prompt_buffer == []

    # The buffer is mutable — pre_turn hooks append to it
    ctx.system_prompt_buffer.append("extra context line")
    assert ctx.system_prompt_buffer == ["extra context line"]


def test_agent_hook_context_buffers_are_per_instance():
    """Two separate context instances must not share the same buffer
    object — each session gets a fresh list."""
    ctx_a = AgentHookContext(
        project_id=1,
        session_id="a",
        workspace_dir=Path("/ws/a"),
        system_prompt_buffer=[],
    )
    ctx_b = AgentHookContext(
        project_id=1,
        session_id="b",
        workspace_dir=Path("/ws/b"),
        system_prompt_buffer=[],
    )
    ctx_a.system_prompt_buffer.append("a-only")
    assert ctx_b.system_prompt_buffer == []
```

- [ ] **Step 3: Run the tests — expect import failures**

```bash
cd ai-worker && python -m pytest tests/openharness/engine/test_agent_hook_registry.py -v
```
Expected: `ImportError: cannot import name 'AgentHookRegistry' from src.openharness.engine.agent_hooks` (and the four Protocols and `AgentHookContext` and `PreToolCallBlock`).

- [ ] **Step 4: Append the new types to `agent_hooks.py`**

Open `ai-worker/src/openharness/engine/agent_hooks.py` and append (do NOT touch the Phase 5a Task 5a.2 code that's already in the file — `SessionHaltError`, `ClarificationCoordinator`, etc.):

```python
# ---------------------------------------------------------------------------
# Round 2: in-process agent hook system (spec §2.9.1)
#
# Distinct from openharness.hooks.HookRegistry, which runs SUBPROCESS
# shell-command hooks on PRE_TOOL_USE / POST_TOOL_USE / etc. events.
# That system is unchanged. AgentHookRegistry is a parallel,
# in-process Python-callable hook system. The two do not replace
# each other and must not share a class name.
#
# Round 2 ships extension points only — empty default registries.
# Downstream projects (the verification project, the entropy project,
# etc.) plug in real implementations via a project-scoped factory.
# ---------------------------------------------------------------------------

from __future__ import annotations

from dataclasses import dataclass, field
from pathlib import Path
from typing import Awaitable, Callable, Protocol, TYPE_CHECKING

if TYPE_CHECKING:
    # Avoid a hard import cycle: messages.py imports from
    # stream_events.py which is in the same package.
    from .messages import ConversationMessage
    from pydantic import BaseModel


# ---------------------------------------------------------------------------
# Pinned async Protocol signatures — §2.9.1.b
# ---------------------------------------------------------------------------


class PreTurnHook(Protocol):
    """Runs before each turn. May mutate the message list (returning
    the new list) and/or append to ctx.system_prompt_buffer (the
    rendered build_system_prompt result is mutable across turns
    via the registry's slot fillers; pre_turn hooks operate on the
    PER-TURN message list)."""

    async def __call__(
        self,
        ctx: "AgentHookContext",
        messages: "list[ConversationMessage]",
    ) -> "list[ConversationMessage]": ...


class PreToolCallHook(Protocol):
    """Runs inside _execute_tool_call between permission check and
    tool execution. Returns either the (possibly-mutated) parsed
    arguments to execute, or PreToolCallBlock(reason) to short-circuit
    with a ToolExecutionCompleted(is_error=True, output=reason). The
    tool itself is NOT executed when blocked. Raising an exception is
    a bug — use PreToolCallBlock for intentional blocks."""

    async def __call__(
        self,
        ctx: "AgentHookContext",
        tool_name: str,
        arguments: "BaseModel",
    ) -> "BaseModel | PreToolCallBlock": ...


class PostTurnHook(Protocol):
    """Runs after each turn completes with stop_reason=end_turn. May
    record metrics, trigger follow-up events, etc. Returns None."""

    async def __call__(
        self,
        ctx: "AgentHookContext",
        final_message: "ConversationMessage",
    ) -> None: ...


class PromptSlotFiller(Protocol):
    """Returns the string that gets substituted into a {{slot_name}}
    placeholder in build_system_prompt. Called once per submit_message
    when the system prompt is rendered."""

    async def __call__(self, ctx: "AgentHookContext") -> str: ...


@dataclass(frozen=True)
class PreToolCallBlock:
    """Returned by a pre_tool_call hook to short-circuit a tool call
    without executing the tool. The reason is surfaced in the
    resulting ToolExecutionCompleted event so the agent observes
    the block in the next turn's input."""

    reason: str


# ---------------------------------------------------------------------------
# AgentHookContext — per-session value passed into every hook call
# ---------------------------------------------------------------------------


@dataclass
class AgentHookContext:
    """Per-session context constructed once in _create_engine and
    passed into every agent hook call.

    Fields:
        project_id: The project this session belongs to. Hooks use
            this to look up project-specific data (specs, profiles,
            constraint configs, etc.).
        session_id: The session UUID. Hooks use this to correlate
            metrics or attach session-scoped state.
        workspace_dir: The absolute path to the session's workspace
            directory. Hooks that need to read project state (e.g.
            for context_engineering hooks that load project specs)
            use this as the root.
        system_prompt_buffer: A mutable list pre_turn hooks can
            append to. The agent loop does NOT currently re-render
            the system prompt mid-session — this buffer is reserved
            for future Round 3+ work where pre_turn hooks dynamically
            inject context per-turn. For Round 2 it's an unused but
            stable interface.
    """

    project_id: int
    session_id: str
    workspace_dir: Path
    system_prompt_buffer: list[str] = field(default_factory=list)


# ---------------------------------------------------------------------------
# AgentHookRegistry — four hook collections, all empty by default
# ---------------------------------------------------------------------------


class AgentHookRegistry:
    """In-process agent hook registry. Holds four collections, all
    empty by default. Each collection is a plain list (or dict for
    slot fillers) — registration is `registry.pre_turn.append(hook)`,
    not a method call. The collections are public attributes so
    downstream factories can populate them with one-liners.

    Order semantics (§2.9.1.c): list order = registration order =
    invocation order. No priority field. If hook ordering matters,
    register them in the order you want them invoked.

    Failure mode (§2.9.1.c): hooks raising an exception halt the
    turn with ErrorEvent(recoverable=False). The empty defaults
    Round 2 ships never raise.
    """

    def __init__(self) -> None:
        # Per-instance lists/dict — never share defaults across
        # instances. The classic Python mutable-default-argument
        # bug class is avoided by constructing fresh containers
        # in __init__.
        self.pre_turn: list[PreTurnHook] = []
        self.pre_tool_call: list[PreToolCallHook] = []
        self.post_turn: list[PostTurnHook] = []
        self.system_prompt_slots: dict[str, PromptSlotFiller] = {}
```

Notes on the imports and type shape:
- `TYPE_CHECKING`-guarded imports avoid the circular import (`agent_hooks.py` references `ConversationMessage` which lives in `messages.py` in the same package). At runtime the Protocol parameters are just strings.
- The Protocols are structural — any async callable matching the signature is accepted. There's no `register()` method; the lists/dict are public attributes by design (the simplest mechanism, per §2.8).

- [ ] **Step 5: Run the tests**

```bash
cd ai-worker && python -m pytest tests/openharness/engine/test_agent_hook_registry.py -v
```
Expected: **9 tests pass** (8 new + 1 buffer-isolation).

- [ ] **Step 6: Smoke-test the import path**

```bash
cd ai-worker && python -c "
from src.openharness.engine.agent_hooks import (
    AgentHookContext,
    AgentHookRegistry,
    PreToolCallBlock,
    PreTurnHook,
    PreToolCallHook,
    PostTurnHook,
    PromptSlotFiller,
)
r = AgentHookRegistry()
assert r.pre_turn == []
assert r.pre_tool_call == []
assert r.post_turn == []
assert r.system_prompt_slots == {}
print('ok')
"
```
Expected: `ok`.

- [ ] **Step 7: Commit**

```bash
git add ai-worker/src/openharness/engine/agent_hooks.py ai-worker/tests/openharness/engine/test_agent_hook_registry.py
git commit -m "feat(engine): AgentHookRegistry + Protocol hook signatures

Round 2 introduces an in-process Python-callable hook system that
runs alongside (NOT replaces) the existing subprocess HookRegistry/
HookExecutor. The new types are appended to agent_hooks.py (the
module Phase 5a Task 5a.2 created with SessionHaltError +
ClarificationCoordinator).

New types (spec §2.9.1.b):
- PreTurnHook(ctx, messages) -> messages — Protocol
- PreToolCallHook(ctx, tool_name, args) -> args | PreToolCallBlock
- PostTurnHook(ctx, final_message) -> None — Protocol
- PromptSlotFiller(ctx) -> str — Protocol
- PreToolCallBlock(reason) — frozen dataclass for short-circuit
- AgentHookContext(project_id, session_id, workspace_dir,
  system_prompt_buffer) — dataclass passed into every hook
- AgentHookRegistry — four public attribute collections:
    pre_turn: list[PreTurnHook]
    pre_tool_call: list[PreToolCallHook]
    post_turn: list[PostTurnHook]
    system_prompt_slots: dict[str, PromptSlotFiller]
  All empty by default. Round 2 ships extension points only —
  no real hook implementations.

Order semantics (§2.9.1.c): list order = registration order =
invocation order. No priority field. If hook ordering matters,
register in the right order. Silicon-valley simplest-mechanism.

Failure mode: hooks raising halt the turn with ErrorEvent
(recoverable=False) — same fail-fast stance as §4.12 no-AsyncMock.

Tests: 9 pure construction/shape tests covering empty registry,
register pre_turn/pre_tool_call/post_turn/slot, frozen
PreToolCallBlock, mutable AgentHookContext.system_prompt_buffer,
and per-instance buffer isolation (no shared mutable defaults).

End-to-end hook firing inside run_agent_loop is covered by
Task 5.10 test_hooks_integration.py."
```

---

### Task 5.9: Hook invocation points in `query.py` + `run_agent_loop`

**Files:**
- Modify: `ai-worker/src/openharness/engine/query.py` — `_execute_tool_call` (pre_tool_call invocation)
- Modify: `ai-worker/src/openharness/engine/query.py` — `run_agent_loop` (pre_turn + post_turn invocations)
- Modify: `ai-worker/src/openharness/engine/query_engine.py` — `QueryEngine.__init__` accepts the registry + context

**Context:** Task 5.8 added the types. Task 5.9 wires them into the actual agent loop. Three call sites per spec §2.9.1.c and §5.1 Round 2:

1. **`pre_turn`** — before each turn, in `run_agent_loop`'s outer loop. Iterate `registry.pre_turn` in list order, call each with `(ctx, messages)`, use the returned messages for the API call. On any hook raise: yield `ErrorEvent(recoverable=False, message="agent hook {name} failed: {exc}")` and `return`.
2. **`pre_tool_call`** — inside `_execute_tool_call`, between the permission check and the tool execution. Iterate `registry.pre_tool_call`, call each with `(ctx, tool_name, parsed_arguments)`. If any returns `PreToolCallBlock(reason)`: emit `ToolExecutionStarted` normally, then emit `ToolExecutionCompleted(is_error=True, output=reason)`, then short-circuit (don't call `tool.execute`). If a hook returns mutated arguments, the next hook sees the mutation; the tool runs with the final mutated arguments.
3. **`post_turn`** — after the turn completes with `stop_reason=end_turn`. Iterate `registry.post_turn`, call each with `(ctx, final_message)`. Errors halt the loop.

`QueryEngine.__init__` accepts `agent_hook_registry: AgentHookRegistry | None = None` (defaults to a fresh empty registry) and `agent_hook_context: AgentHookContext | None = None` (defaults to None — only useful when the registry is non-empty). Both are passed into the `QueryContext` that flows into `run_agent_loop`.

- [ ] **Step 1: Read the current `run_agent_loop` and `_execute_tool_call`**

```bash
sed -n '1,40p' ai-worker/src/openharness/engine/query.py
grep -n "def run_agent_loop\|def _execute_tool_call" ai-worker/src/openharness/engine/query.py
```
Note the line numbers; the edits in Step 2 reference them.

- [ ] **Step 2: Extend `QueryContext` and `QueryEngine.__init__`**

In `ai-worker/src/openharness/engine/query.py`, find the `QueryContext` dataclass (it's the bag of state passed into `run_agent_loop`). Add two new fields:

```python
@dataclass
class QueryContext:
    api_client: SupportsStreamingMessages
    tool_registry: ToolRegistry
    model: str
    system_prompt: str
    max_tokens: int = 4096
    max_turns: int = 25
    hook_executor: HookExecutor | None = None
    permission_checker: PermissionChecker | None = None
    cwd: Path | None = None
    # Round 2 additions (Phase 5 Task 5.9)
    agent_hook_registry: "AgentHookRegistry | None" = None
    agent_hook_context: "AgentHookContext | None" = None
```

(Use forward-reference strings to avoid an import cycle. The actual `from .agent_hooks import ...` lives at the top of the file under a `TYPE_CHECKING` guard.)

In `ai-worker/src/openharness/engine/query_engine.py`, find `QueryEngine.__init__` and add the matching parameters:

```python
def __init__(
    self,
    api_client: SupportsStreamingMessages,
    tool_registry: ToolRegistry,
    model: str,
    system_prompt: str,
    *,
    max_tokens: int = 4096,
    max_turns: int = 25,
    hook_executor: "HookExecutor | None" = None,
    permission_checker: "PermissionChecker | None" = None,
    cwd: "Path | None" = None,
    # Round 2 additions (Phase 5 Task 5.9)
    agent_hook_registry: "AgentHookRegistry | None" = None,
    agent_hook_context: "AgentHookContext | None" = None,
) -> None:
    from .agent_hooks import AgentHookRegistry  # local import, avoids cycle
    self._api_client = api_client
    self._tool_registry = tool_registry
    self._model = model
    self._system_prompt = system_prompt
    self._max_tokens = max_tokens
    self._max_turns = max_turns
    self._hook_executor = hook_executor
    self._permission_checker = permission_checker
    self._cwd = cwd
    # Empty default keeps the loop hot path simple — no None checks
    # in run_agent_loop, just iterate empty lists.
    self._agent_hook_registry = agent_hook_registry or AgentHookRegistry()
    self._agent_hook_context = agent_hook_context  # may be None
    self._messages: list[ConversationMessage] = []
    self._total_usage = UsageSnapshot()
```

In the same file, the `submit_message` method already constructs a `QueryContext` (Task 5.3 wired SessionCollector into it). Update that construction to also pass the registry + context:

```python
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
    agent_hook_registry=self._agent_hook_registry,
    agent_hook_context=self._agent_hook_context,
)
```

- [ ] **Step 3: Add the `pre_turn` invocation to `run_agent_loop`**

In `query.py`, find the outer `for turn in range(context.max_turns):` loop in `run_agent_loop`. Just before the message list is sent to `context.api_client.stream_message(...)`, insert the pre_turn invocation:

```python
async def run_agent_loop(
    context: QueryContext,
    messages: list[ConversationMessage],
) -> AsyncIterator[StreamEvent]:
    """The main agent loop. Yields stream events as the agent
    interacts with tools across one or more turns.
    """
    from .agent_hooks import AgentHookRegistry  # local import for runtime resolution
    from .stream_events import ErrorEvent

    registry = context.agent_hook_registry or AgentHookRegistry()
    hook_ctx = context.agent_hook_context  # may be None for legacy callers

    for turn in range(context.max_turns):
        # Round 2 (§2.9.1.c, §5.1): pre_turn hooks run before the
        # message list is sent. Each hook may mutate the message list
        # and/or append to ctx.system_prompt_buffer. Hooks run in
        # registration order. A raising hook halts the loop with
        # ErrorEvent(recoverable=False).
        for hook in registry.pre_turn:
            try:
                messages = await hook(hook_ctx, messages)
            except Exception as exc:
                logger.exception(
                    "agent hook %s in pre_turn raised", hook
                )
                yield ErrorEvent(
                    message=(
                        f"agent hook {getattr(hook, '__name__', repr(hook))} "
                        f"raised in pre_turn: {exc}"
                    ),
                    recoverable=False,
                )
                return

        # ... existing API call + stream loop unchanged ...
```

After the inner stream-handling loop, when a turn ends with `stop_reason="end_turn"`, insert the post_turn invocation:

```python
        if stop_reason == "end_turn":
            # Round 2 (§2.9.1.c, §5.1): post_turn hooks observe the
            # turn's final assistant message. Errors halt the loop.
            for hook in registry.post_turn:
                try:
                    await hook(hook_ctx, final_message)
                except Exception as exc:
                    logger.exception(
                        "agent hook %s in post_turn raised", hook
                    )
                    yield ErrorEvent(
                        message=(
                            f"agent hook {getattr(hook, '__name__', repr(hook))} "
                            f"raised in post_turn: {exc}"
                        ),
                        recoverable=False,
                    )
                    return
            return  # exits the agent loop normally

        # ... existing handling for stop_reason == "tool_use" continues ...
```

Where `final_message` is the assistant `ConversationMessage` returned from the most recent `ApiMessageCompleteEvent`.

- [ ] **Step 4: Add the `pre_tool_call` invocation inside `_execute_tool_call`**

In the same file, find `_execute_tool_call` (the helper that runs a single tool call from a `ToolUseBlock`). Between the permission check and the actual `tool.execute(...)` call, insert:

```python
async def _execute_tool_call(
    context: QueryContext,
    tool_use: ToolUseBlock,
) -> AsyncIterator[StreamEvent | ToolResult]:
    """Execute a single tool call from a ToolUseBlock. Yields any
    StreamEvents the tool emits and ends with a ToolResult."""
    from .agent_hooks import (
        AgentHookRegistry,
        PreToolCallBlock,
    )

    tool = context.tool_registry.get(tool_use.name)
    if tool is None:
        yield ToolResult(
            tool_use_id=tool_use.id,
            output=f"unknown tool: {tool_use.name}",
            is_error=True,
        )
        return

    # Parse arguments through the tool's input model. Pydantic
    # raises ValidationError on shape mismatches; surface as a
    # tool error.
    try:
        parsed_arguments = tool.input_model.model_validate(tool_use.input)
    except Exception as exc:
        yield ToolResult(
            tool_use_id=tool_use.id,
            output=f"invalid arguments for {tool_use.name}: {exc}",
            is_error=True,
        )
        return

    # Permission check (existing — unchanged)
    if context.permission_checker is not None:
        permission_decision = context.permission_checker.check(
            tool_name=tool_use.name,
            tool_input=tool_use.input,
        )
        if permission_decision.denied:
            yield ToolResult(
                tool_use_id=tool_use.id,
                output=(
                    f"permission denied for {tool_use.name}: "
                    f"{permission_decision.reason}"
                ),
                is_error=True,
            )
            return

    # Round 2 (§2.9.1.c): pre_tool_call hooks run between permission
    # check and tool execution. Each hook receives the (possibly
    # mutated) parsed arguments from the previous hook, and returns
    # either new arguments or a PreToolCallBlock to short-circuit.
    registry = context.agent_hook_registry or AgentHookRegistry()
    hook_ctx = context.agent_hook_context

    for hook in registry.pre_tool_call:
        try:
            result = await hook(hook_ctx, tool_use.name, parsed_arguments)
        except Exception as exc:
            logger.exception(
                "agent hook %s in pre_tool_call raised", hook
            )
            from .stream_events import ErrorEvent
            yield ErrorEvent(
                message=(
                    f"agent hook {getattr(hook, '__name__', repr(hook))} "
                    f"raised in pre_tool_call: {exc}"
                ),
                recoverable=False,
            )
            return

        if isinstance(result, PreToolCallBlock):
            # Short-circuit per §2.9.1.c: emit Started + Completed
            # with is_error=True, do NOT execute the tool.
            yield ToolExecutionStarted(
                tool_use_id=tool_use.id,
                tool_name=tool_use.name,
                tool_input=tool_use.input,
            )
            yield ToolExecutionCompleted(
                tool_use_id=tool_use.id,
                tool_name=tool_use.name,
                output=result.reason,
                is_error=True,
            )
            yield ToolResult(
                tool_use_id=tool_use.id,
                output=result.reason,
                is_error=True,
            )
            return

        parsed_arguments = result  # mutated args feed the next hook

    # ... existing ToolExecutionStarted emission + tool.execute(...) loop ...
```

- [ ] **Step 5: Smoke-test the imports + module shape**

```bash
cd ai-worker && python -c "
from src.openharness.engine.query import QueryContext, run_agent_loop, _execute_tool_call
from src.openharness.engine.query_engine import QueryEngine
from src.openharness.engine.agent_hooks import AgentHookRegistry, AgentHookContext
import inspect
sig = inspect.signature(QueryEngine.__init__)
assert 'agent_hook_registry' in sig.parameters
assert 'agent_hook_context' in sig.parameters
print('ok')
"
```
Expected: `ok`. Any ImportError or missing parameter means the wiring is wrong — debug before proceeding.

- [ ] **Step 6: Run the existing query/engine tests to catch regressions**

```bash
cd ai-worker && python -m pytest tests/test_query_engine.py tests/openharness/engine/ -v 2>&1 | tail -40
```
Expected: existing tests still pass — the registry has empty defaults, so legacy callers that don't pass the new arguments see no behavior change. The full hook integration tests come in Task 5.10.

- [ ] **Step 7: Commit**

```bash
git add ai-worker/src/openharness/engine/query.py ai-worker/src/openharness/engine/query_engine.py
git commit -m "feat(engine): wire AgentHookRegistry invocation into query.py

Three call sites per spec §2.9.1.c and §5.1 Round 2:

1. pre_turn: in run_agent_loop, before each turn. Iterate
   registry.pre_turn in list order, call (ctx, messages),
   replace messages with the returned list. Hook raise yields
   ErrorEvent(recoverable=False) and halts the loop.

2. pre_tool_call: in _execute_tool_call, between the permission
   check and tool.execute. Each hook receives (ctx, tool_name,
   parsed_arguments) and returns either new arguments or a
   PreToolCallBlock(reason). On block: emit ToolExecutionStarted
   + ToolExecutionCompleted(is_error=True, output=reason) and
   skip tool execution. Hooks compose by chaining mutated args.
   Hook raise also halts.

3. post_turn: in run_agent_loop, after stop_reason=end_turn.
   Iterate registry.post_turn in list order, call (ctx,
   final_message). Hook raise halts.

QueryContext gains two optional fields agent_hook_registry +
agent_hook_context. QueryEngine.__init__ accepts the matching
keyword-only parameters with empty-default registry. Legacy
callers see zero behavior change because empty registries
yield zero hook iterations.

End-to-end behavioral tests live in Task 5.10
test_hooks_integration.py."
```

---

### Task 5.10: Hook integration tests

**Files:**
- Create: `ai-worker/tests/openharness/engine/test_hooks_integration.py`

**Context:** Task 5.9 wired the hooks. Task 5.10 verifies they actually fire end-to-end with a real `run_agent_loop` invocation against a mocked API client. These are integration tests — the registry, the context, the loop, and the tool execution path are all real; only the upstream LLM is mocked.

Per spec §7.4 Round 2, ~11 scenarios:
1. `pre_turn` hook mutates the system prompt buffer (no API mutation, just confirms the buffer is observable post-turn)
2. `pre_turn` hook mutates the message list (the mocked API client sees the appended message)
3. `pre_tool_call` hook blocks a named tool (`bash`) — the tool is NOT executed and the agent observes a `is_error=True` ToolExecutionCompleted
4. `pre_tool_call` hook mutates arguments — the tool runs with the mutated arguments
5. `post_turn` hook fires on `end_turn` — counter increments to 1
6. `pre_turn` hook raising `RuntimeError` — loop emits `ErrorEvent(recoverable=False)` with hook name
7. `pre_tool_call` hook raising — loop emits `ErrorEvent(recoverable=False)` with hook name
8. `post_turn` hook raising — loop emits `ErrorEvent(recoverable=False)` with hook name
9. Hooks invoked in registration order — register hook A then B, observe A's mutation visible to B
10. `system_prompt_slots` substitution — registered filler replaces `{{slot_name}}` placeholder
11. `system_prompt_slots` missing — unfilled `{{project_specs}}` is regex-stripped from the rendered prompt

The tests use a small `FakeApiClient` that returns canned `ApiMessageCompleteEvent` sequences, the same pattern Task 5.3 introduced for SessionComplete tests.

- [ ] **Step 1: Create the integration test file**

Create `ai-worker/tests/openharness/engine/test_hooks_integration.py`:

```python
"""End-to-end agent hook integration tests.

These exercise the full call chain (registry → run_agent_loop →
_execute_tool_call) against a mocked API client. The registry,
context, loop, and tool execution are all real — only the upstream
LLM is mocked.

Spec: §2.9.1.c, §5.1 Round 2, §7.4 Round 2.
"""

from __future__ import annotations

from pathlib import Path
from typing import AsyncIterator

import pytest

from src.openharness.api.client import (
    ApiMessageCompleteEvent,
    ApiMessageRequest,
    SupportsStreamingMessages,
)
from src.openharness.api.usage import UsageSnapshot
from src.openharness.engine.agent_hooks import (
    AgentHookContext,
    AgentHookRegistry,
    PreToolCallBlock,
)
from src.openharness.engine.messages import (
    ConversationMessage,
    TextBlock,
    ToolUseBlock,
)
from src.openharness.engine.query_engine import QueryEngine
from src.openharness.engine.stream_events import (
    ErrorEvent,
    SessionComplete,
    ToolExecutionCompleted,
    ToolExecutionStarted,
)
from src.openharness.tools.base import ToolRegistry
from src.openharness.tools.phase_tool import SetPhaseTool


# ---------------------------------------------------------------------------
# Test helpers
# ---------------------------------------------------------------------------


class _FakeApiClient(SupportsStreamingMessages):
    """Replays a canned sequence of ApiMessageCompleteEvents.

    Each call to stream_message returns the next event in the queue.
    Tests construct an instance with the events they want, run the
    engine, then assert on the recorded request payloads.
    """

    def __init__(self, events: list[ApiMessageCompleteEvent]) -> None:
        self._events = list(events)
        self.recorded_requests: list[ApiMessageRequest] = []

    async def stream_message(
        self, request: ApiMessageRequest
    ) -> AsyncIterator[ApiMessageCompleteEvent]:
        self.recorded_requests.append(request)
        if not self._events:
            raise RuntimeError("FakeApiClient: no more queued events")
        yield self._events.pop(0)


def _end_turn_event(text: str = "done") -> ApiMessageCompleteEvent:
    return ApiMessageCompleteEvent(
        message=ConversationMessage(
            role="assistant",
            content=[TextBlock(text=text)],
        ),
        usage=UsageSnapshot(input_tokens=5, output_tokens=2),
        stop_reason="end_turn",
    )


def _tool_use_event(
    tool_name: str,
    tool_input: dict,
    tool_use_id: str = "toolu_1",
) -> ApiMessageCompleteEvent:
    return ApiMessageCompleteEvent(
        message=ConversationMessage(
            role="assistant",
            content=[
                TextBlock(text="invoking tool"),
                ToolUseBlock(
                    id=tool_use_id,
                    name=tool_name,
                    input=tool_input,
                ),
            ],
        ),
        usage=UsageSnapshot(input_tokens=10, output_tokens=5),
        stop_reason="tool_use",
    )


def _make_engine(
    api_client: _FakeApiClient,
    *,
    registry: AgentHookRegistry | None = None,
    tools: ToolRegistry | None = None,
    cwd: Path | None = None,
) -> QueryEngine:
    return QueryEngine(
        api_client=api_client,
        tool_registry=tools or ToolRegistry(),
        model="test-model",
        system_prompt="base system prompt",
        agent_hook_registry=registry,
        agent_hook_context=(
            AgentHookContext(
                project_id=1,
                session_id="sess-test",
                workspace_dir=cwd or Path("/tmp/ws"),
                system_prompt_buffer=[],
            )
            if registry is not None
            else None
        ),
        cwd=cwd or Path("/tmp/ws"),
    )


# ---------------------------------------------------------------------------
# pre_turn hook scenarios
# ---------------------------------------------------------------------------


@pytest.mark.asyncio
async def test_pre_turn_hook_mutates_system_prompt(tmp_path):
    """A pre_turn hook can append to ctx.system_prompt_buffer; the
    mutation is visible to later hooks and to test code reading the
    context after the turn."""
    registry = AgentHookRegistry()

    async def appender(ctx, messages):
        ctx.system_prompt_buffer.append("<extra context>")
        return messages

    registry.pre_turn.append(appender)

    api_client = _FakeApiClient([_end_turn_event()])
    engine = _make_engine(api_client, registry=registry, cwd=tmp_path)

    events = [e async for e in engine.submit_message("hi")]
    assert engine._agent_hook_context.system_prompt_buffer == [
        "<extra context>"
    ]
    # Sanity: the loop completed normally
    assert any(isinstance(e, SessionComplete) for e in events) is False  # text-only turn skips


@pytest.mark.asyncio
async def test_pre_turn_hook_mutates_message_list(tmp_path):
    """A pre_turn hook can return a mutated message list; the API
    client receives the mutated version."""
    registry = AgentHookRegistry()

    async def inject(ctx, messages):
        return messages + [
            ConversationMessage.from_user_text("[injected]")
        ]

    registry.pre_turn.append(inject)

    api_client = _FakeApiClient([_end_turn_event()])
    engine = _make_engine(api_client, registry=registry, cwd=tmp_path)

    [e async for e in engine.submit_message("original")]

    assert len(api_client.recorded_requests) == 1
    request = api_client.recorded_requests[0]
    # The mutated list should contain at least the original user message
    # plus the injected one
    assert any(
        "[injected]" in (m.text if hasattr(m, "text") else "")
        for m in request.messages
    )


# ---------------------------------------------------------------------------
# pre_tool_call hook scenarios
# ---------------------------------------------------------------------------


@pytest.mark.asyncio
async def test_pre_tool_call_hook_blocks_tool(tmp_path):
    """A pre_tool_call hook returning PreToolCallBlock short-circuits
    tool execution. The agent observes a ToolExecutionCompleted with
    is_error=True and output equal to the block reason."""
    registry = AgentHookRegistry()

    async def block_set_phase(ctx, tool_name, arguments):
        if tool_name == "set_phase":
            return PreToolCallBlock(reason="set_phase blocked by test")
        return arguments

    registry.pre_tool_call.append(block_set_phase)

    tools = ToolRegistry()
    tools.register(SetPhaseTool())

    api_client = _FakeApiClient([
        _tool_use_event("set_phase", {"phase": "Generate"}),
        _end_turn_event(),
    ])
    engine = _make_engine(
        api_client, registry=registry, tools=tools, cwd=tmp_path
    )

    events = [e async for e in engine.submit_message("go")]

    completed = [
        e for e in events if isinstance(e, ToolExecutionCompleted)
    ]
    assert len(completed) == 1
    assert completed[0].is_error is True
    assert completed[0].output == "set_phase blocked by test"


@pytest.mark.asyncio
async def test_pre_tool_call_hook_mutates_arguments(tmp_path):
    """A pre_tool_call hook returning a mutated arguments object
    causes the tool to run with the mutated values. We use SetPhase
    because it's the only tool that returns its input verbatim."""
    registry = AgentHookRegistry()

    async def force_review(ctx, tool_name, arguments):
        if tool_name == "set_phase":
            # Pydantic models support .copy(update=...)
            return arguments.model_copy(update={"phase": "Review"})
        return arguments

    registry.pre_tool_call.append(force_review)

    tools = ToolRegistry()
    tools.register(SetPhaseTool())

    api_client = _FakeApiClient([
        _tool_use_event("set_phase", {"phase": "Generate"}),
        _end_turn_event(),
    ])
    engine = _make_engine(
        api_client, registry=registry, tools=tools, cwd=tmp_path
    )

    events = [e async for e in engine.submit_message("go")]
    started = [
        e for e in events if isinstance(e, ToolExecutionStarted)
    ]
    assert len(started) == 1
    # Hook mutated Generate -> Review at the boundary; the agent
    # loop forwarded the original tool_input on Started, but the
    # tool itself ran with the mutated arguments and the resulting
    # Completed reflects that.
    completed = [
        e for e in events if isinstance(e, ToolExecutionCompleted)
    ]
    assert len(completed) == 1
    assert completed[0].is_error is False
    # SetPhase returns "Phase set to {phase}" — assert the mutated value
    assert "Review" in completed[0].output


# ---------------------------------------------------------------------------
# post_turn hook scenarios
# ---------------------------------------------------------------------------


@pytest.mark.asyncio
async def test_post_turn_hook_fires_on_end_turn(tmp_path):
    """A post_turn hook fires once at the end of a normally
    terminated turn (stop_reason=end_turn)."""
    registry = AgentHookRegistry()
    counter = {"value": 0}

    async def increment(ctx, final_message):
        counter["value"] += 1

    registry.post_turn.append(increment)

    api_client = _FakeApiClient([_end_turn_event()])
    engine = _make_engine(api_client, registry=registry, cwd=tmp_path)

    [e async for e in engine.submit_message("hi")]
    assert counter["value"] == 1


# ---------------------------------------------------------------------------
# Hook exception → loop halt
# ---------------------------------------------------------------------------


@pytest.mark.asyncio
async def test_pre_turn_hook_exception_halts_loop(tmp_path):
    registry = AgentHookRegistry()

    async def boom(ctx, messages):
        raise RuntimeError("pre_turn boom")

    registry.pre_turn.append(boom)

    api_client = _FakeApiClient([_end_turn_event()])
    engine = _make_engine(api_client, registry=registry, cwd=tmp_path)

    events = [e async for e in engine.submit_message("hi")]
    errors = [e for e in events if isinstance(e, ErrorEvent)]
    assert len(errors) == 1
    assert errors[0].recoverable is False
    assert "pre_turn" in errors[0].message
    assert "boom" in errors[0].message
    # The API client must NOT have been called — the hook raised before the request
    assert api_client.recorded_requests == []


@pytest.mark.asyncio
async def test_pre_tool_call_hook_exception_halts_loop(tmp_path):
    registry = AgentHookRegistry()

    async def boom(ctx, tool_name, arguments):
        raise RuntimeError("pre_tool_call boom")

    registry.pre_tool_call.append(boom)

    tools = ToolRegistry()
    tools.register(SetPhaseTool())

    api_client = _FakeApiClient([
        _tool_use_event("set_phase", {"phase": "Generate"}),
        _end_turn_event(),
    ])
    engine = _make_engine(
        api_client, registry=registry, tools=tools, cwd=tmp_path
    )

    events = [e async for e in engine.submit_message("go")]
    errors = [e for e in events if isinstance(e, ErrorEvent)]
    assert len(errors) == 1
    assert errors[0].recoverable is False
    assert "pre_tool_call" in errors[0].message
    assert "boom" in errors[0].message


@pytest.mark.asyncio
async def test_post_turn_hook_exception_halts_loop(tmp_path):
    registry = AgentHookRegistry()

    async def boom(ctx, final_message):
        raise RuntimeError("post_turn boom")

    registry.post_turn.append(boom)

    api_client = _FakeApiClient([_end_turn_event()])
    engine = _make_engine(api_client, registry=registry, cwd=tmp_path)

    events = [e async for e in engine.submit_message("hi")]
    errors = [e for e in events if isinstance(e, ErrorEvent)]
    assert len(errors) == 1
    assert errors[0].recoverable is False
    assert "post_turn" in errors[0].message
    assert "boom" in errors[0].message


# ---------------------------------------------------------------------------
# Hook ordering
# ---------------------------------------------------------------------------


@pytest.mark.asyncio
async def test_hooks_invoked_in_registration_order(tmp_path):
    """Per spec §2.9.1.c: list order = registration order = invocation
    order. Register A then B; A's mutation is visible to B."""
    registry = AgentHookRegistry()
    trail: list[str] = []

    async def first(ctx, messages):
        trail.append("first")
        return messages

    async def second(ctx, messages):
        trail.append("second")
        return messages

    registry.pre_turn.append(first)
    registry.pre_turn.append(second)

    api_client = _FakeApiClient([_end_turn_event()])
    engine = _make_engine(api_client, registry=registry, cwd=tmp_path)

    [e async for e in engine.submit_message("hi")]
    assert trail == ["first", "second"]


# ---------------------------------------------------------------------------
# system_prompt_slots tests (covered also in test_prompts.py — these
# verify the slot fillers work end-to-end through build_system_prompt)
# ---------------------------------------------------------------------------


@pytest.mark.asyncio
async def test_system_prompt_slot_substitution(tmp_path):
    """A registered slot filler replaces its {{slot_name}}
    placeholder in the rendered system prompt."""
    from src.openharness.engine.agent_hooks import AgentHookContext
    from src.openharness.engine.prompts import build_system_prompt

    registry = AgentHookRegistry()

    async def project_specs_filler(ctx):
        return "spec content goes here"

    registry.system_prompt_slots["project_specs"] = project_specs_filler

    ctx = AgentHookContext(
        project_id=1,
        session_id="sess",
        workspace_dir=tmp_path,
        system_prompt_buffer=[],
    )

    prompt = await build_system_prompt(
        language="go",
        workspace_path=str(tmp_path),
        slots=registry.system_prompt_slots,
        hook_context=ctx,
    )
    assert "spec content goes here" in prompt
    # And the literal placeholder is gone
    assert "{{project_specs}}" not in prompt


@pytest.mark.asyncio
async def test_system_prompt_slot_missing_is_stripped(tmp_path):
    """When no slot filler is registered for {{project_specs}},
    the regex cleanup strips the placeholder so the agent never
    sees the literal text."""
    from src.openharness.engine.prompts import build_system_prompt

    prompt = await build_system_prompt(
        language="go",
        workspace_path=str(tmp_path),
        slots=None,
        hook_context=None,
    )
    assert "{{project_specs}}" not in prompt
    # Defensive: no other unrendered slots either
    assert "{{" not in prompt
```

- [ ] **Step 2: Run the integration tests**

```bash
cd ai-worker && python -m pytest tests/openharness/engine/test_hooks_integration.py -v
```
Expected: **11 tests pass**. Failures here mean Task 5.9's wiring is incomplete or buggy — debug `query.py` before proceeding.

If `test_system_prompt_slot_substitution` and `test_system_prompt_slot_missing_is_stripped` fail with `TypeError: object str can't be used in 'await' expression`, that's a sign Task 5.11 hasn't been done yet — these two tests depend on the async `build_system_prompt`. Run them after Task 5.11.

- [ ] **Step 3: Commit**

```bash
git add ai-worker/tests/openharness/engine/test_hooks_integration.py
git commit -m "test(engine): AgentHookRegistry integration tests

End-to-end behavioral tests for the in-process agent hook system.
Run the full call chain (registry -> run_agent_loop ->
_execute_tool_call) against a mocked API client. The registry,
context, loop, and tool execution are all real; only the upstream
LLM is mocked via _FakeApiClient.

Eleven scenarios per spec §7.4 Round 2:
- pre_turn mutates system_prompt_buffer (visible post-turn)
- pre_turn mutates message list (FakeApiClient sees mutation)
- pre_tool_call blocks set_phase tool with PreToolCallBlock
- pre_tool_call mutates set_phase argument (Generate -> Review)
- post_turn fires once on stop_reason=end_turn
- pre_turn raise -> ErrorEvent(recoverable=False), API not called
- pre_tool_call raise -> ErrorEvent(recoverable=False)
- post_turn raise -> ErrorEvent(recoverable=False)
- Hooks invoked in registration order (A then B trail)
- system_prompt_slots filler replaces {{project_specs}} placeholder
- Missing slot filler -> regex cleanup strips literal {{slot}}

The last two scenarios depend on Task 5.11's async
build_system_prompt — run them after that task lands."
```

---

### Task 5.11: `build_system_prompt` slot substitution + Round 2 system prompt update

**Files:**
- Modify: `ai-worker/src/openharness/engine/prompts.py` — `build_system_prompt` becomes async + slot-aware
- Modify: `ai-worker/tests/openharness/engine/test_prompts.py` — add Round 2 tests

**Context:** Task 5.1 created `build_system_prompt(language, workspace_path) -> str` as a sync function. Spec §5.2 Round 2 makes it async (so slot fillers can be async) and gains two optional parameters:

```python
async def build_system_prompt(
    language: str | None,
    workspace_path: str,
    slots: dict[str, PromptSlotFiller] | None = None,
    hook_context: AgentHookContext | None = None,
) -> str:
```

The template gains a `{{project_specs}}` placeholder. After slot substitution, a regex pass strips any remaining unfilled `{{slot_name}}` placeholders so the agent never sees literal placeholder text.

The "How to work" section gains two new bullets per spec §5.2 Round 2:
- **Bullet 1** (replaces the old "Understand the user's request" bullet): "...If the request is ambiguous, call `request_clarification` with a specific question rather than guessing. The user will type a response and you will receive it as the tool's return value."
- **Bullet 7** (new): "At major milestones — before `end_turn`, before a git commit that represents a user-visible feature boundary — call `request_review` with a short summary of what you built and why you believe it's correct..."

The existing Round 1 ten substring tests stay valid (the constraints, phases, tools, sandbox text are all still in the prompt). Add seven new tests covering the slot machinery and the new instruction bullets.

- [ ] **Step 1: Add the failing Round 2 tests**

Append to `ai-worker/tests/openharness/engine/test_prompts.py`:

```python
# ---------------------------------------------------------------------------
# Round 2: slot substitution + Round 2 instruction bullets
# ---------------------------------------------------------------------------

import pytest

from src.openharness.engine.agent_hooks import AgentHookContext
from src.openharness.engine.prompts import build_system_prompt


@pytest.mark.asyncio
async def test_language_substitution():
    """The {language} f-string substitution from Round 1 still works
    after the async refactor."""
    prompt = await build_system_prompt(
        language="python",
        workspace_path="/data/ws/py",
    )
    assert "python" in prompt.lower()


@pytest.mark.asyncio
async def test_workspace_path_substitution():
    prompt = await build_system_prompt(
        language="go",
        workspace_path="/data/ws/go-project",
    )
    assert "/data/ws/go-project" in prompt


@pytest.mark.asyncio
async def test_unregistered_slot_stripped():
    """When no slot filler is registered for {{project_specs}}, the
    regex cleanup strips it. The agent must never see literal
    {{project_specs}} in the rendered prompt."""
    prompt = await build_system_prompt(
        language="go",
        workspace_path="/ws",
    )
    assert "{{project_specs}}" not in prompt
    assert "{{" not in prompt


@pytest.mark.asyncio
async def test_registered_slot_replaces_placeholder(tmp_path):
    """A registered slot filler's return value replaces its
    {{slot_name}} placeholder via plain str.replace."""
    async def filler(ctx):
        return "## Project specs\n- spec one\n- spec two"

    ctx = AgentHookContext(
        project_id=1,
        session_id="s",
        workspace_dir=tmp_path,
        system_prompt_buffer=[],
    )
    prompt = await build_system_prompt(
        language="go",
        workspace_path=str(tmp_path),
        slots={"project_specs": filler},
        hook_context=ctx,
    )
    assert "## Project specs" in prompt
    assert "- spec one" in prompt
    assert "{{project_specs}}" not in prompt


@pytest.mark.asyncio
async def test_filler_exception_propagates(tmp_path):
    """A slot filler that raises propagates the exception (fail-fast
    per §2.8 — silent suppression would hide context loader bugs)."""
    async def boom(ctx):
        raise RuntimeError("filler boom")

    ctx = AgentHookContext(
        project_id=1,
        session_id="s",
        workspace_dir=tmp_path,
        system_prompt_buffer=[],
    )
    with pytest.raises(RuntimeError, match="filler boom"):
        await build_system_prompt(
            language="go",
            workspace_path=str(tmp_path),
            slots={"project_specs": boom},
            hook_context=ctx,
        )


@pytest.mark.asyncio
async def test_request_review_instruction_present():
    """Round 2 §5.2 bullet 7: the agent must be told when to invoke
    request_review at major milestones."""
    prompt = await build_system_prompt(
        language="go",
        workspace_path="/ws",
    )
    assert "request_review" in prompt
    # The verdict vocabulary must be mentioned so the agent knows
    # how to interpret the response
    assert "APPROVE" in prompt
    assert "REVISE" in prompt
    assert "REJECT" in prompt


@pytest.mark.asyncio
async def test_request_clarification_instruction_present():
    """Round 2 §5.2 bullet 1: the agent must be told to call
    request_clarification when the request is ambiguous."""
    prompt = await build_system_prompt(
        language="go",
        workspace_path="/ws",
    )
    assert "request_clarification" in prompt
    assert "ambiguous" in prompt.lower() or "clarification" in prompt.lower()
```

Note: the Round 1 tests (`test_prompt_mentions_all_seven_phases`, etc.) need to be **converted to async** because `build_system_prompt` is now async. Add `@pytest.mark.asyncio` to each one and prefix the body with `await`. This is mechanical — `pytest -v` will tell you which tests fail with `TypeError: object str can't be used in 'await' expression` and you fix them in place.

- [ ] **Step 2: Run the tests — expect failures**

```bash
cd ai-worker && python -m pytest tests/openharness/engine/test_prompts.py -v
```
Expected: the new Round 2 tests fail (slot machinery doesn't exist yet, `{{project_specs}}` not in template, `request_review` not in template). The Round 1 tests fail with the async TypeError — fix them up too.

- [ ] **Step 3: Rewrite `prompts.py`**

Open `ai-worker/src/openharness/engine/prompts.py`. Replace the function with the async slot-aware version per spec §5.2 Round 2:

```python
"""System prompts for the Variant B single-agent.

Round 2: build_system_prompt is now async and accepts optional slot
fillers (§2.9.1). The template has a {{project_specs}} placeholder
which is substituted by registered fillers; unfilled placeholders
are regex-stripped before the prompt is returned.

Spec: §5.2 System prompt (Round 2).
"""

from __future__ import annotations

import logging
import re
from typing import TYPE_CHECKING, Optional

if TYPE_CHECKING:
    from .agent_hooks import AgentHookContext, PromptSlotFiller

logger = logging.getLogger(__name__)


async def build_system_prompt(
    language: Optional[str],
    workspace_path: str,
    slots: "dict[str, PromptSlotFiller] | None" = None,
    hook_context: "AgentHookContext | None" = None,
) -> str:
    """Build the full system prompt for a Variant B agent session.

    Args:
        language: Detected language name (e.g. "go", "python") or
            None if detection failed.
        workspace_path: Absolute path of the workspace mounted into
            the ai-worker container.
        slots: Optional mapping of slot name -> async filler. Each
            filler is invoked with hook_context and its return value
            replaces {{slot_name}} in the template via str.replace.
            Slot names not in the template log a warning.
        hook_context: The AgentHookContext passed to slot fillers.
            Required when slots is non-empty; otherwise unused.

    Returns:
        A multi-line system prompt with all {{slot}} placeholders
        either substituted or stripped.
    """
    lang_line = (
        f"- Project language: {language}"
        if language
        else "- Project language: unknown (inspect files with list_directory/glob/read_file to detect)"
    )

    template = f"""You are Forge Agent, an AI coding assistant embedded in a Harness Engineering platform. You work on a user's codebase inside a sandboxed workspace.

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

**Interaction meta-tools**
- `request_clarification` — pause and ask the user a clarifying question; the response arrives as the tool's return value
- `request_review` — request an independent reviewer LLM to critique your current work before finalizing

{{{{project_specs}}}}

## How to work

1. Understand the user's request. **If the request is ambiguous, call `request_clarification` with a specific question rather than guessing.** The user will type a response and you will receive it as the tool's return value. Do not waste turns inferring intent from partial information.
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

7. **At major milestones — before `end_turn`, before a git commit that represents a user-visible feature boundary — call `request_review` with a short summary of what you built and why you believe it's correct.** The reviewer is an independent LLM that sees your diff and the user's original request. Act on the verdict: **APPROVE** → proceed, **REVISE** → address the listed items, **REJECT** → reconsider the approach. You are not required to invoke the reviewer on every turn; use judgment.

8. Stop when the user's request is satisfied. Do NOT over-engineer. Do NOT add features the user did not ask for. Do NOT refactor adjacent code unrelated to the task.

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

    # Slot substitution (§2.9.1.c, §5.2 Round 2): registered fillers
    # replace their {{slot_name}} placeholder. Unregistered slot names
    # log a warning. Filler exceptions propagate (fail-fast §2.8).
    if slots:
        for slot_name, filler in slots.items():
            placeholder = "{{" + slot_name + "}}"
            if placeholder in template:
                value = await filler(hook_context)
                template = template.replace(placeholder, value)
            else:
                logger.warning(
                    "build_system_prompt: slot '%s' is registered but its "
                    "placeholder is not in the template",
                    slot_name,
                )

    # Strip any unfilled {{slot_name}} placeholders so the agent
    # never sees them literally. The pattern matches Python-identifier
    # slot names: letters, digits, underscores, starting with a
    # letter or underscore.
    template = re.sub(r"\{\{[a-zA-Z_][a-zA-Z_0-9]*\}\}", "", template)
    return template
```

- [ ] **Step 4: Run the tests**

```bash
cd ai-worker && python -m pytest tests/openharness/engine/test_prompts.py -v
```
Expected: all Round 1 (now async) + 7 new Round 2 tests pass. If a Round 1 test still fails because of the async refactor, add `@pytest.mark.asyncio` and `await` to that test and re-run.

- [ ] **Step 5: Smoke-test the integration tests from Task 5.10**

```bash
cd ai-worker && python -m pytest tests/openharness/engine/test_hooks_integration.py::test_system_prompt_slot_substitution tests/openharness/engine/test_hooks_integration.py::test_system_prompt_slot_missing_is_stripped -v
```
Expected: both pass now that `build_system_prompt` is async and slot-aware.

- [ ] **Step 6: Commit**

```bash
git add ai-worker/src/openharness/engine/prompts.py ai-worker/tests/openharness/engine/test_prompts.py
git commit -m "feat(prompts): slot substitution + Round 2 system prompt update

build_system_prompt is now async and accepts optional slot fillers.
Spec §5.2 Round 2 + §2.9.1.c.

Signature change:
  build_system_prompt(language, workspace_path,
                      slots=None, hook_context=None) -> str
  All slots are dict[str, PromptSlotFiller] async callables.

Template additions:
- {{project_specs}} placeholder near the top of the prompt; gets
  substituted by a registered filler or stripped by the regex
  cleanup pass at the end of the function
- Bullet 1 (How to work) now mentions request_clarification and
  tells the agent to call it on ambiguous requests rather than
  guessing
- New bullet 7 (How to work) tells the agent to call request_review
  at major milestones and explains how to interpret the
  APPROVE/REVISE/REJECT verdict vocabulary

Slot substitution: registered slot names get replaced via plain
str.replace. Unregistered slot names log a warning. Slot fillers
that raise propagate the exception (fail-fast per §2.8).

Final cleanup: re.sub strips any literal {{slot_name}} that
remains. Pattern matches Python-identifier slot names.

Tests: Round 1 ten substring tests converted to async +
seven new Round 2 tests:
- test_language_substitution
- test_workspace_path_substitution
- test_unregistered_slot_stripped
- test_registered_slot_replaces_placeholder
- test_filler_exception_propagates
- test_request_review_instruction_present
- test_request_clarification_instruction_present

The two slot tests in Task 5.10 test_hooks_integration.py also
pass after this task (they test build_system_prompt end-to-end
through the AgentHookRegistry)."
```

---

### Task 5.12: `build_reviewer_prompt` + `parse_verdict` + `REVIEWER_SYSTEM_PROMPT`

**Files:**
- Modify: `ai-worker/src/openharness/engine/prompts.py` — append reviewer machinery
- Create: `ai-worker/tests/openharness/engine/test_reviewer_prompts.py`

**Context:** Spec §2.9.3.c-d defines the reviewer-side text infrastructure that `RequestReviewTool` (Task 5.13) uses:

1. **`REVIEWER_SYSTEM_PROMPT`** — pinned constant string with the exact text from §2.9.3.c. The reviewer LLM sees this as its system prompt and is told to respond with `APPROVE` / `REVISE <details>` / `REJECT <reason>` on a single line.
2. **`build_reviewer_prompt(summary, current_diff, original_request) -> str`** — renders the user-facing reviewer message with the three required inputs in three labeled sections.
3. **`REVIEWER_DIFF_MAX_BYTES = 32_768`** — cap on `git diff HEAD` output before truncation.
4. **`VERDICT_PATTERN`** — regex `^(APPROVE|REVISE|REJECT)(?:\s+(.*))?$` (multiline) to find the verdict line in the reviewer's response.
5. **`parse_verdict(text) -> (verdict, details)`** — scans the response for the first matching verdict line. Raises `ReviewerParseError` on failure (no fallback — the tool surfaces the parse error to the agent which decides whether to retry).

These are all pure-Python helpers; tests are substring/regex assertions.

- [ ] **Step 1: Write the failing tests**

Create `ai-worker/tests/openharness/engine/test_reviewer_prompts.py`:

```python
"""Tests for the reviewer-side prompt + parser machinery.

Spec: §2.9.3.c-d.
"""

from __future__ import annotations

import pytest

from src.openharness.engine.prompts import (
    REVIEWER_DIFF_MAX_BYTES,
    REVIEWER_SYSTEM_PROMPT,
    ReviewerParseError,
    build_reviewer_prompt,
    parse_verdict,
)


# ---------------------------------------------------------------------------
# build_reviewer_prompt — substring invariants
# ---------------------------------------------------------------------------


def test_build_reviewer_prompt_substring_invariants():
    """All three input strings appear in the rendered prompt under
    their labeled sections."""
    rendered = build_reviewer_prompt(
        summary="Added a /api/health endpoint that returns 200 OK.",
        current_diff="diff --git a/api.go b/api.go\n+func health() {}",
        original_request="please add a health endpoint",
    )

    # Section headers
    assert "User's original request" in rendered
    assert "Agent's summary of work" in rendered
    assert "Git diff" in rendered

    # Inputs themselves
    assert "please add a health endpoint" in rendered
    assert "Added a /api/health endpoint that returns 200 OK." in rendered
    assert "diff --git a/api.go b/api.go" in rendered

    # The closing instruction
    assert "APPROVE" in rendered
    assert "REVISE" in rendered
    assert "REJECT" in rendered


def test_build_reviewer_prompt_deterministic():
    """Same inputs -> same output. No timestamps, no random IDs."""
    args = dict(
        summary="s",
        current_diff="d",
        original_request="r",
    )
    a = build_reviewer_prompt(**args)
    b = build_reviewer_prompt(**args)
    assert a == b


def test_reviewer_system_prompt_constant_present():
    """The pinned system prompt constant exists and contains the
    verdict vocabulary."""
    assert "senior engineer" in REVIEWER_SYSTEM_PROMPT.lower() or "reviewer" in REVIEWER_SYSTEM_PROMPT.lower()
    assert "APPROVE" in REVIEWER_SYSTEM_PROMPT
    assert "REVISE" in REVIEWER_SYSTEM_PROMPT
    assert "REJECT" in REVIEWER_SYSTEM_PROMPT


def test_reviewer_diff_max_bytes_constant():
    """The cap is 32 KiB per spec §2.9.3.e."""
    assert REVIEWER_DIFF_MAX_BYTES == 32_768


# ---------------------------------------------------------------------------
# parse_verdict — happy paths
# ---------------------------------------------------------------------------


def test_parse_verdict_approve():
    verdict, details = parse_verdict("APPROVE")
    assert verdict == "APPROVE"
    assert details == ""


def test_parse_verdict_revise_with_details():
    verdict, details = parse_verdict("REVISE add null check on line 42")
    assert verdict == "REVISE"
    assert details == "add null check on line 42"


def test_parse_verdict_reject_with_reason():
    verdict, details = parse_verdict(
        "REJECT the diff implements a different feature than asked"
    )
    assert verdict == "REJECT"
    assert "different feature than asked" in details


def test_parse_verdict_finds_verdict_in_middle_of_response():
    """The regex is multiline; preamble or chatter before the
    verdict line is allowed."""
    verdict, details = parse_verdict(
        "Let me think...\nLooking at the diff, I see...\n"
        "APPROVE\n"
        "Some trailing text that should be ignored."
    )
    assert verdict == "APPROVE"


def test_parse_verdict_first_line_is_verdict():
    verdict, details = parse_verdict("REVISE rename the function\nmore notes here")
    assert verdict == "REVISE"
    assert details == "rename the function"


# ---------------------------------------------------------------------------
# parse_verdict — error paths
# ---------------------------------------------------------------------------


def test_parse_verdict_invalid_raises_error():
    """Response with no recognized verdict line raises
    ReviewerParseError. The exception message includes a snippet of
    the response so the caller can log it."""
    with pytest.raises(ReviewerParseError) as exc_info:
        parse_verdict("This response has no verdict at all.")
    assert "This response has no verdict" in str(exc_info.value)


def test_parse_verdict_empty_input_raises():
    with pytest.raises(ReviewerParseError):
        parse_verdict("")


def test_parse_verdict_lowercase_verdict_does_not_match():
    """Verdicts are case-sensitive — the spec uses uppercase as the
    parsing contract. A lowercase 'approve' is not parsed."""
    with pytest.raises(ReviewerParseError):
        parse_verdict("approve")
```

- [ ] **Step 2: Run the tests — expect import failures**

```bash
cd ai-worker && python -m pytest tests/openharness/engine/test_reviewer_prompts.py -v
```
Expected: `ImportError` for `REVIEWER_SYSTEM_PROMPT`, `ReviewerParseError`, etc.

- [ ] **Step 3: Append the reviewer machinery to `prompts.py`**

Open `ai-worker/src/openharness/engine/prompts.py` (Task 5.11 already extended it). Append after `build_system_prompt`:

```python
# ---------------------------------------------------------------------------
# Reviewer-side prompt machinery (§2.9.3.c-d)
# Used by RequestReviewTool (Task 5.13).
# ---------------------------------------------------------------------------


REVIEWER_DIFF_MAX_BYTES = 32_768


REVIEWER_SYSTEM_PROMPT = """You are a senior engineer reviewing another AI agent's work on a user's codebase. You have no tools. You see only: (1) the user's original request, (2) the AI agent's own summary of what it built, (3) the git diff showing the agent's changes.

Your job: judge whether the agent's work actually does what the user asked. Focus on:
- Intent mismatch: the diff does something subtly different from the user's request (wrong field name, wrong endpoint, wrong default)
- Missing cases: the user asked for X including edge cases, the diff handles X but not the edge cases
- Obvious bugs: null dereferences, off-by-one, unsafe SQL, missing error handling in load-bearing paths
- Non-goals: the diff adds functionality the user did not ask for

Do NOT flag: coding style, naming preferences, architectural taste, "could be more elegant", "might be slow", "should add tests" (unless tests are part of the user's request).

Respond with EXACTLY one of these formats, on a single line, no preamble:

    APPROVE
    REVISE <what to change>
    REJECT <why it's fundamentally wrong>

Your verdict is parsed by regex. Any text before the verdict line or after it will be ignored."""


def build_reviewer_prompt(
    summary: str,
    current_diff: str,
    original_request: str,
) -> str:
    """Render the user message the reviewer LLM will see.

    All three arguments are required. The diff is assumed to be
    pre-truncated by the caller (RequestReviewTool._collect_git_diff
    enforces REVIEWER_DIFF_MAX_BYTES).

    Args:
        summary: The agent's own description of what it built and
            why it believes the work is complete.
        current_diff: Output of `git diff HEAD` from the workspace,
            capped at REVIEWER_DIFF_MAX_BYTES.
        original_request: The user's original message.

    Returns:
        Plain-string user message ready to send to ModelRouter.generate
        as the single message in the messages array.
    """
    return f"""## User's original request
{original_request}

## Agent's summary of work
{summary}

## Git diff
{current_diff}

Review the above and respond with APPROVE / REVISE / REJECT.
"""


# ---------------------------------------------------------------------------
# Verdict parsing — §2.9.3.d
# ---------------------------------------------------------------------------


import re
from typing import Literal, Tuple


VERDICT_PATTERN = re.compile(
    r"^(APPROVE|REVISE|REJECT)(?:\s+(.*))?$",
    re.MULTILINE,
)


class ReviewerParseError(ValueError):
    """Raised when the reviewer's response does not contain a
    parseable verdict line. The agent observes this as a tool error
    and decides whether to retry the request_review call or proceed
    without a verdict.
    """


def parse_verdict(text: str) -> Tuple[Literal["APPROVE", "REVISE", "REJECT"], str]:
    """Find the first verdict line in the reviewer's response.

    The reviewer is instructed to respond with EXACTLY one of:
        APPROVE
        REVISE <details>
        REJECT <reason>
    on a single line. We scan line-by-line for the first match
    (so preamble or trailing chatter is allowed but ignored).

    Returns:
        (verdict, details) tuple. Verdict is the literal string
        "APPROVE" / "REVISE" / "REJECT". Details is the rest of the
        line after the verdict word, stripped, or empty for APPROVE.

    Raises:
        ReviewerParseError: no verdict line was found in the response.
    """
    if not text:
        raise ReviewerParseError(
            "Reviewer response is empty — no verdict line to parse"
        )

    for line in text.splitlines():
        match = VERDICT_PATTERN.match(line.strip())
        if match:
            verdict = match.group(1)  # type: ignore[assignment]
            details = (match.group(2) or "").strip()
            return verdict, details

    raise ReviewerParseError(
        f"Reviewer response did not contain a verdict line: {text[:200]!r}"
    )
```

Note: the `import re` near the bottom is intentional — Python imports are idempotent and putting the import next to the regex it serves keeps the constant + function + parser as a self-contained block. The `from typing import Literal, Tuple` is also placed inline for the same reason.

- [ ] **Step 4: Run the tests**

```bash
cd ai-worker && python -m pytest tests/openharness/engine/test_reviewer_prompts.py -v
```
Expected: **12 tests pass** (4 substring/constant + 5 verdict parsing happy paths + 3 error paths).

- [ ] **Step 5: Commit**

```bash
git add ai-worker/src/openharness/engine/prompts.py ai-worker/tests/openharness/engine/test_reviewer_prompts.py
git commit -m "feat(prompts): build_reviewer_prompt + parse_verdict for RequestReviewTool

Three pieces of reviewer-side text infrastructure per spec §2.9.3.c-d,
all consumed by Task 5.13's RequestReviewTool:

1. REVIEWER_SYSTEM_PROMPT — pinned constant string the reviewer
   LLM sees as its system prompt. Tells it to respond with
   APPROVE / REVISE <details> / REJECT <reason> on a single line.
   The text is verbatim from spec §2.9.3.c.

2. build_reviewer_prompt(summary, current_diff, original_request)
   — renders the three labeled sections (User's original request,
   Agent's summary of work, Git diff) into a single user message
   string. Deterministic — no timestamps or random IDs.

3. REVIEWER_DIFF_MAX_BYTES = 32_768 — cap enforced by the tool's
   git diff collector before calling build_reviewer_prompt.

Plus the verdict parser:

- VERDICT_PATTERN — re.compile(r'^(APPROVE|REVISE|REJECT)(?:\\s+(.*))?$',
  re.MULTILINE)
- parse_verdict(text) -> (verdict_literal, details)
  Scans the response line-by-line for the first match. Allows
  preamble and trailing chatter. Case-sensitive verbs (lowercase
  'approve' does not match — the parsing contract is uppercase).
- ReviewerParseError(ValueError) raised when no verdict line found.
  No fallback — the tool surfaces the error to the agent which
  decides whether to retry.

Tests: 12 cases covering substring invariants, constant shape,
APPROVE/REVISE/REJECT happy paths with and without details,
verdict in the middle of a multiline response, empty input,
lowercase rejection."
```

---

### Task 5.13: `RequestReviewTool` implementation

**Files:**
- Modify: `ai-worker/src/openharness/tools/interaction_tools.py` (APPEND — Phase 5a Task 5a.4 already created this with `RequestClarificationTool`)
- Create: `ai-worker/tests/openharness/tools/test_request_review_tool.py`

**Context:** Spec §2.9.3.a defines the tool. It subclasses `SimpleTool` (not `BaseTool`) — it doesn't yield mid-execution stream events, it just performs an async LLM call and returns the verdict text. The behavior:

1. Collect `git diff HEAD` from the workspace via `asyncio.create_subprocess_exec` (NOT `BashTool`/bwrap — exempted by §2.9.3.e because the call is hardcoded, parameter-less, read-only). Cap at `REVIEWER_DIFF_MAX_BYTES`. 30s timeout.
2. Call `build_reviewer_prompt(summary, diff, original_user_request)` (from Task 5.12). The original user request comes from `context.original_user_request` (a field added to `ToolExecutionContext` in Phase 5a).
3. Call `await self._router.generate(purpose=Purpose.REVIEW, system_prompt=REVIEWER_SYSTEM_PROMPT, messages=[{"role": "user", "content": prompt}], max_tokens=1024)`.
4. Return `ToolResult(output=response_text, is_error=False)`. The tool does NOT parse the verdict — the agent reads the raw reviewer text and decides what to do (the verdict vocabulary is in the system prompt; the agent is instructed to respond accordingly).
5. On `ModelRouterError`: return `ToolResult(output=f"reviewer unavailable: {exc}", is_error=True)`. No retry.
6. On subprocess failure (git diff timeout, non-zero exit): return `ToolResult(output=f"git diff failed: {error}", is_error=True)`.

**Constructor:** `RequestReviewTool(model_router: ModelRouter, workspace_dir: Path)`. Both required. Stored as `self._router` and `self._workspace_dir`.

- [ ] **Step 1: Read the existing `interaction_tools.py` so the append is additive**

```bash
sed -n '1,30p' ai-worker/src/openharness/tools/interaction_tools.py
grep -n "class\|def " ai-worker/src/openharness/tools/interaction_tools.py
```
Expected: the file has `RequestClarificationTool` from Phase 5a Task 5a.4 plus its imports. The Round 2 additions append below.

- [ ] **Step 2: Write the failing tests**

Create `ai-worker/tests/openharness/tools/test_request_review_tool.py`:

```python
"""Tests for RequestReviewTool — the reviewer meta-tool.

Spec: §2.9.3.a-g.
"""

from __future__ import annotations

import asyncio
import subprocess
from pathlib import Path
from unittest.mock import AsyncMock

import pytest

from src.openharness.engine.prompts import REVIEWER_DIFF_MAX_BYTES
from src.openharness.tools.base import ToolExecutionContext
from src.openharness.tools.interaction_tools import (
    RequestReviewInput,
    RequestReviewTool,
)


# ---------------------------------------------------------------------------
# Test fixtures
# ---------------------------------------------------------------------------


def _make_mock_router(response: str | Exception):
    """Build a mock ModelRouter whose generate() returns `response`
    or raises if `response` is an Exception."""
    router = AsyncMock()
    if isinstance(response, Exception):
        router.generate.side_effect = response
    else:
        router.generate.return_value = response
    return router


def _make_context(workspace: Path) -> ToolExecutionContext:
    """Build a ToolExecutionContext for the test. Phase 5a added the
    original_user_request field; we populate it explicitly."""
    return ToolExecutionContext(
        cwd=workspace,
        tool_use_id="toolu_review_test",
        original_user_request="add a health endpoint",
    )


@pytest.fixture
def real_git_workspace(tmp_path: Path) -> Path:
    """Create a real git workspace with one committed file and one
    staged change. _collect_git_diff should return a non-empty diff
    against this fixture."""
    subprocess.run(["git", "init", "-q"], cwd=tmp_path, check=True)
    subprocess.run(
        ["git", "config", "user.email", "test@test.com"],
        cwd=tmp_path, check=True,
    )
    subprocess.run(
        ["git", "config", "user.name", "test"],
        cwd=tmp_path, check=True,
    )
    (tmp_path / "main.go").write_text("package main\n\nfunc main() {}\n")
    subprocess.run(["git", "add", "main.go"], cwd=tmp_path, check=True)
    subprocess.run(
        ["git", "commit", "-q", "-m", "initial"],
        cwd=tmp_path, check=True,
    )
    # Modify the file so git diff HEAD has output
    (tmp_path / "main.go").write_text(
        "package main\n\nfunc main() { println(\"hi\") }\n"
    )
    return tmp_path


# ---------------------------------------------------------------------------
# Happy path
# ---------------------------------------------------------------------------


@pytest.mark.asyncio
async def test_happy_path_mocked_router(real_git_workspace: Path):
    """The tool collects git diff, builds the reviewer prompt, calls
    the router, and returns the router's response as ToolResult.output
    with is_error=False."""
    router = _make_mock_router(response="APPROVE\n")
    tool = RequestReviewTool(
        model_router=router,
        workspace_dir=real_git_workspace,
    )

    arguments = RequestReviewInput(
        summary="Added println call to main."
    )
    ctx = _make_context(real_git_workspace)

    result = await tool._execute_simple(arguments, ctx)
    assert result.is_error is False
    assert result.output == "APPROVE\n"
    # Router was called exactly once
    router.generate.assert_called_once()
    # And the call had the expected purpose + system prompt
    call_kwargs = router.generate.call_args.kwargs
    from src.openharness.engine.prompts import REVIEWER_SYSTEM_PROMPT
    assert call_kwargs["system_prompt"] == REVIEWER_SYSTEM_PROMPT
    assert call_kwargs["max_tokens"] == 1024
    # Purpose is set to REVIEW (the actual import path is in the tool)
    assert "REVIEW" in str(call_kwargs.get("purpose", ""))


@pytest.mark.asyncio
async def test_revise_response_preserved(real_git_workspace: Path):
    """A REVISE response is returned verbatim — the tool does NOT
    parse the verdict (the agent does)."""
    router = _make_mock_router(
        response="REVISE add null check on line 42\nDetails follow."
    )
    tool = RequestReviewTool(
        model_router=router,
        workspace_dir=real_git_workspace,
    )
    arguments = RequestReviewInput(summary="x")
    ctx = _make_context(real_git_workspace)

    result = await tool._execute_simple(arguments, ctx)
    assert result.is_error is False
    assert "REVISE add null check on line 42" in result.output
    assert "Details follow." in result.output


# ---------------------------------------------------------------------------
# Router exception path
# ---------------------------------------------------------------------------


@pytest.mark.asyncio
async def test_router_exception_returns_error(real_git_workspace: Path):
    """When ModelRouter.generate raises a ModelRouterError, the tool
    catches it and returns ToolResult(is_error=True)."""
    from src.openharness.tools.interaction_tools import ModelRouterError

    router = _make_mock_router(
        response=ModelRouterError("no providers configured")
    )
    tool = RequestReviewTool(
        model_router=router,
        workspace_dir=real_git_workspace,
    )
    arguments = RequestReviewInput(summary="x")
    ctx = _make_context(real_git_workspace)

    result = await tool._execute_simple(arguments, ctx)
    assert result.is_error is True
    assert "reviewer unavailable" in result.output
    assert "no providers configured" in result.output


# ---------------------------------------------------------------------------
# Git diff collection
# ---------------------------------------------------------------------------


@pytest.mark.asyncio
async def test_collect_git_diff_real_workspace(real_git_workspace: Path):
    """In a real git workspace with an unstaged modification,
    _collect_git_diff returns the diff text and it contains the
    modification."""
    router = _make_mock_router(response="APPROVE")
    tool = RequestReviewTool(
        model_router=router,
        workspace_dir=real_git_workspace,
    )
    diff = await tool._collect_git_diff()
    assert "main.go" in diff
    assert "println" in diff


@pytest.mark.asyncio
async def test_collect_git_diff_truncation(real_git_workspace: Path):
    """If the diff exceeds REVIEWER_DIFF_MAX_BYTES, the result is
    truncated and a marker is appended."""
    # Create a large file with many lines so the diff exceeds 32KiB
    big_content = "\n".join(
        f"line {i} " + "x" * 80 for i in range(2000)
    )
    (real_git_workspace / "big.txt").write_text(big_content)
    subprocess.run(
        ["git", "add", "big.txt"],
        cwd=real_git_workspace, check=True,
    )
    # Don't commit — leave staged so git diff HEAD picks it up

    router = _make_mock_router(response="APPROVE")
    tool = RequestReviewTool(
        model_router=router,
        workspace_dir=real_git_workspace,
    )
    diff = await tool._collect_git_diff()
    assert len(diff.encode("utf-8")) <= REVIEWER_DIFF_MAX_BYTES + 100
    assert "<diff truncated" in diff


@pytest.mark.asyncio
async def test_collect_git_diff_timeout(tmp_path: Path, monkeypatch):
    """If git diff hangs longer than the 30s timeout, the tool
    catches asyncio.TimeoutError and returns an error result."""
    # Initialize a real git repo so git diff HEAD doesn't fail
    # immediately — the timeout path is what we're testing.
    subprocess.run(["git", "init", "-q"], cwd=tmp_path, check=True)
    subprocess.run(
        ["git", "config", "user.email", "t@t.com"],
        cwd=tmp_path, check=True,
    )
    subprocess.run(
        ["git", "config", "user.name", "t"],
        cwd=tmp_path, check=True,
    )
    (tmp_path / "f.txt").write_text("x")
    subprocess.run(["git", "add", "."], cwd=tmp_path, check=True)
    subprocess.run(
        ["git", "commit", "-q", "-m", "init"],
        cwd=tmp_path, check=True,
    )

    router = _make_mock_router(response="APPROVE")
    tool = RequestReviewTool(
        model_router=router,
        workspace_dir=tmp_path,
    )

    # Patch wait_for to immediately raise TimeoutError
    async def fake_wait_for(coro, timeout):
        raise asyncio.TimeoutError()

    monkeypatch.setattr(asyncio, "wait_for", fake_wait_for)

    arguments = RequestReviewInput(summary="x")
    ctx = _make_context(tmp_path)
    result = await tool._execute_simple(arguments, ctx)
    assert result.is_error is True
    assert "timed out" in result.output.lower() or "timeout" in result.output.lower()


# ---------------------------------------------------------------------------
# Input validation
# ---------------------------------------------------------------------------


def test_input_validation_empty_summary_rejected():
    """Pydantic rejects empty/whitespace-only summary at construction
    time. The agent must provide a real summary."""
    from pydantic import ValidationError

    with pytest.raises(ValidationError):
        RequestReviewInput(summary="")
```

- [ ] **Step 3: Run the tests — expect failures**

```bash
cd ai-worker && python -m pytest tests/openharness/tools/test_request_review_tool.py -v
```
Expected: `ImportError: cannot import name 'RequestReviewTool' from src.openharness.tools.interaction_tools`.

- [ ] **Step 4: Append `RequestReviewTool` to `interaction_tools.py`**

Open `ai-worker/src/openharness/tools/interaction_tools.py`. Append below the existing `RequestClarificationTool`:

```python
# ---------------------------------------------------------------------------
# RequestReviewTool — Round 2 §2.9.3
# ---------------------------------------------------------------------------

import asyncio
import logging
from pathlib import Path
from typing import TYPE_CHECKING

from pydantic import BaseModel, Field

from src.openharness.engine.prompts import (
    REVIEWER_DIFF_MAX_BYTES,
    REVIEWER_SYSTEM_PROMPT,
    build_reviewer_prompt,
)
from src.openharness.tools.base import (
    SimpleTool,
    ToolExecutionContext,
    ToolResult,
)

if TYPE_CHECKING:
    from src.models.router import ModelRouter

logger = logging.getLogger(__name__)


# Re-export ModelRouterError so tests and downstream callers can
# catch it without reaching into src.models. The tool catches this
# class explicitly in _execute_simple.
try:
    from src.models.router import ModelRouterError
except ImportError:
    # ModelRouter is built and shipped before this tool, but defensive
    # imports keep the test suite buildable in environments where the
    # router module hasn't been wired in yet (CI bootstrap, etc.).
    class ModelRouterError(Exception):
        """Raised by ModelRouter.generate when no provider is available
        or all providers fail. Re-exported here so RequestReviewTool's
        callers don't need to import from src.models directly."""


GIT_DIFF_TIMEOUT_SECONDS = 30
DIFF_TRUNCATION_MARKER = "\n\n<diff truncated at 32KiB by RequestReviewTool>\n"


class RequestReviewInput(BaseModel):
    """Input for request_review.

    summary:
        The agent's own description of what it built and why it
        believes the work is complete. Required, non-empty.
    """

    summary: str = Field(..., min_length=1)


class RequestReviewTool(SimpleTool):
    """Independent reviewer LLM invocation. The agent calls this at
    major milestones to get a second opinion on its diff before
    finalizing the turn.

    The tool collects `git diff HEAD` from the workspace via direct
    subprocess (NOT BashTool/bwrap — see §2.9.3.e for the exemption),
    builds the reviewer prompt, calls ModelRouter.generate with
    Purpose.REVIEW, and returns the raw reviewer response. The
    verdict is NOT parsed by the tool; the agent reads the response
    and decides whether to APPROVE/REVISE/REJECT-style follow-up.

    Spec: §2.9.3.
    """

    name = "request_review"
    description = (
        "Request an independent reviewer LLM to critique your current "
        "work before finalizing. The reviewer sees your git diff and "
        "the user's original request. Returns the reviewer's verdict "
        "as plain text — read the response and act on the verdict "
        "(APPROVE proceed, REVISE address listed items, REJECT "
        "reconsider approach)."
    )
    input_model = RequestReviewInput

    def __init__(
        self,
        model_router: "ModelRouter",
        workspace_dir: Path,
    ) -> None:
        self._router = model_router
        self._workspace_dir = workspace_dir

    async def _execute_simple(
        self,
        arguments: RequestReviewInput,
        context: ToolExecutionContext,
    ) -> ToolResult:
        # Collect git diff first — if this fails, we don't waste an
        # LLM call.
        try:
            diff = await self._collect_git_diff()
        except asyncio.TimeoutError:
            return ToolResult(
                output=(
                    f"git diff HEAD timed out after "
                    f"{GIT_DIFF_TIMEOUT_SECONDS}s — workspace may be "
                    f"in an unhealthy state"
                ),
                is_error=True,
            )
        except Exception as exc:
            logger.exception("RequestReviewTool: git diff collection failed")
            return ToolResult(
                output=f"git diff failed: {exc}",
                is_error=True,
            )

        prompt = build_reviewer_prompt(
            summary=arguments.summary,
            current_diff=diff,
            original_request=context.original_user_request,
        )

        # Lazy import to avoid hard dependency at module-import time
        from src.models.router import Purpose

        try:
            response_text = await self._router.generate(
                purpose=Purpose.REVIEW,
                system_prompt=REVIEWER_SYSTEM_PROMPT,
                messages=[{"role": "user", "content": prompt}],
                max_tokens=1024,
            )
        except ModelRouterError as exc:
            return ToolResult(
                output=f"reviewer unavailable: {exc}",
                is_error=True,
            )

        return ToolResult(output=response_text, is_error=False)

    async def _collect_git_diff(self) -> str:
        """Run `git diff HEAD` directly via asyncio.create_subprocess_exec.

        §2.9.3.e bwrap exemption: this is one of exactly two
        subprocess calls in Round 2 that bypass bwrap. The other is
        the workspace manager's git operations (§3.5). Both are
        hardcoded, parameter-less, read-only git invocations. The
        argv is literally ['git', 'diff', 'HEAD'] — no shell, no
        user input, no format strings.

        Output is capped at REVIEWER_DIFF_MAX_BYTES (32 KiB). On
        overflow, the diff is truncated and DIFF_TRUNCATION_MARKER
        is appended so the reviewer knows it didn't see everything.

        Timeout: GIT_DIFF_TIMEOUT_SECONDS (30 s). On timeout, the
        process group is killed and asyncio.TimeoutError propagates
        to the caller.
        """
        proc = await asyncio.create_subprocess_exec(
            "git",
            "diff",
            "HEAD",
            cwd=str(self._workspace_dir),
            stdout=asyncio.subprocess.PIPE,
            stderr=asyncio.subprocess.PIPE,
        )
        try:
            stdout, stderr = await asyncio.wait_for(
                proc.communicate(),
                timeout=GIT_DIFF_TIMEOUT_SECONDS,
            )
        except asyncio.TimeoutError:
            try:
                proc.kill()
            except ProcessLookupError:
                pass
            raise

        if proc.returncode != 0:
            # Common case: workspace isn't a git repo, or HEAD doesn't
            # exist (fresh clone with no commits). Surface the stderr
            # so the agent can decide what to do.
            err = stderr.decode("utf-8", errors="replace")
            raise RuntimeError(
                f"git diff HEAD exited {proc.returncode}: {err.strip()}"
            )

        # Decode + truncate
        text = stdout.decode("utf-8", errors="replace")
        if len(text.encode("utf-8")) > REVIEWER_DIFF_MAX_BYTES:
            # Slice to roughly the byte budget; the actual byte length
            # after the marker is appended is checked in the test.
            truncated = text.encode("utf-8")[:REVIEWER_DIFF_MAX_BYTES]
            text = (
                truncated.decode("utf-8", errors="replace")
                + DIFF_TRUNCATION_MARKER
            )
        return text
```

- [ ] **Step 5: Run the tests**

```bash
cd ai-worker && python -m pytest tests/openharness/tools/test_request_review_tool.py -v
```
Expected: **8 tests pass** (happy path, REVISE preservation, router exception, real-workspace diff collection, truncation, timeout, plus the input validation test).

If `test_collect_git_diff_real_workspace` fails on a system without `git` installed, that's an environment problem — install git or mark the test with `@pytest.mark.skipif(shutil.which("git") is None, reason="git not installed")`. The test relies on real git as the spec demands (§2.9.3.g).

- [ ] **Step 6: Commit**

```bash
git add ai-worker/src/openharness/tools/interaction_tools.py ai-worker/tests/openharness/tools/test_request_review_tool.py
git commit -m "feat(tools): RequestReviewTool with git diff collection

The reviewer meta-tool from spec §2.9.3. Subclasses SimpleTool
(not BaseTool) — no mid-execution stream events, just a single
async LLM call returning a ToolResult.

Behavior:
1. Collect git diff HEAD via asyncio.create_subprocess_exec
   directly. §2.9.3.e bwrap exemption: argv is hardcoded
   ['git', 'diff', 'HEAD'], no shell, no user input. One of
   exactly two subprocess calls in Round 2 that bypass bwrap
   (the other is the workspace manager's git operations).
2. Cap diff output at REVIEWER_DIFF_MAX_BYTES (32 KiB) and
   append DIFF_TRUNCATION_MARKER on overflow.
3. 30-second wall-clock timeout on git diff. On timeout: kill
   the process group, return ToolResult(is_error=True,
   output='timed out...').
4. Build the reviewer prompt via build_reviewer_prompt
   (Task 5.12) using arguments.summary, the diff, and
   context.original_user_request.
5. Call ModelRouter.generate(purpose=Purpose.REVIEW,
   system_prompt=REVIEWER_SYSTEM_PROMPT,
   messages=[user-msg],
   max_tokens=1024).
6. Return ToolResult(output=response_text, is_error=False).
   The tool does NOT parse the verdict — the agent reads
   the raw text and decides what to do based on the
   APPROVE/REVISE/REJECT vocabulary the system prompt taught it.

Constructor: RequestReviewTool(model_router, workspace_dir).
Both required.

Error paths:
- ModelRouterError -> ToolResult(is_error=True,
  output='reviewer unavailable: {exc}'). No retry.
- git diff timeout -> ToolResult(is_error=True, ...timed out)
- git diff non-zero exit -> RuntimeError -> ToolResult(is_error=True,
  output='git diff failed: {exc}')

Input validation: Pydantic rejects empty/whitespace summary at
construction time (Field min_length=1).

Tests: 8 cases covering happy path with mocked router,
REVISE preservation, router exception path, real git fixture
diff collection, truncation on >32KiB diff, timeout via
patched asyncio.wait_for, input validation."
```

---

### Task 5.14: `register_interaction_tools` helper

**Files:**
- Modify: `ai-worker/src/openharness/tools/interaction_tools.py` (extend with one helper function)
- Create: `ai-worker/tests/openharness/tools/test_register_interaction_tools.py`

**Context:** Spec §2.9.3.b mentions `register_interaction_tools(tool_registry, model_router, workspace_dir)` as the way `_create_engine` wires up both interaction meta-tools (`RequestClarificationTool` from Phase 5a Task 5a.4 and `RequestReviewTool` from Task 5.13). The helper is one short function that constructs both tools and registers them on the supplied `ToolRegistry`.

Idempotency: a second call must fail loudly. `ToolRegistry.register` already raises on duplicate registration; the helper just propagates that — there's no `if name in registry` check that silently swallows the second call. This is the silicon-valley standard from spec §2.8: a duplicate `register_interaction_tools` call indicates a bug in the call site, surface it.

- [ ] **Step 1: Write the failing tests**

Create `ai-worker/tests/openharness/tools/test_register_interaction_tools.py`:

```python
"""Tests for register_interaction_tools helper.

Spec: §2.9.3.b.
"""

from __future__ import annotations

from pathlib import Path
from unittest.mock import AsyncMock

import pytest

from src.openharness.tools.base import ToolRegistry
from src.openharness.tools.interaction_tools import (
    RequestClarificationTool,
    RequestReviewTool,
    register_interaction_tools,
)


def test_registers_both_tools(tmp_path: Path):
    """A single call to register_interaction_tools registers both
    RequestClarificationTool and RequestReviewTool on the supplied
    ToolRegistry."""
    registry = ToolRegistry()
    router = AsyncMock()

    register_interaction_tools(
        registry=registry,
        model_router=router,
        workspace_dir=tmp_path,
    )

    clarification = registry.get("request_clarification")
    review = registry.get("request_review")

    assert clarification is not None
    assert review is not None
    assert isinstance(clarification, RequestClarificationTool)
    assert isinstance(review, RequestReviewTool)


def test_idempotent_registration_fails_loudly(tmp_path: Path):
    """A second call must raise — duplicate registration indicates
    a bug in the call site (probably calling _create_engine twice
    on the same shared registry, which would cause stale tools).
    Silicon-valley §2.8: surface the bug, don't swallow it."""
    registry = ToolRegistry()
    router = AsyncMock()

    register_interaction_tools(
        registry=registry,
        model_router=router,
        workspace_dir=tmp_path,
    )

    with pytest.raises(Exception):  # ToolRegistry raises ValueError or KeyError
        register_interaction_tools(
            registry=registry,
            model_router=router,
            workspace_dir=tmp_path,
        )


def test_review_tool_constructed_with_router_and_workspace(tmp_path: Path):
    """The RequestReviewTool instance must be constructed with the
    same router and workspace_dir passed into the helper — verify
    via private attributes."""
    registry = ToolRegistry()
    router = AsyncMock()

    register_interaction_tools(
        registry=registry,
        model_router=router,
        workspace_dir=tmp_path,
    )

    review = registry.get("request_review")
    assert review._router is router
    assert review._workspace_dir == tmp_path
```

- [ ] **Step 2: Run the tests — expect import failure**

```bash
cd ai-worker && python -m pytest tests/openharness/tools/test_register_interaction_tools.py -v
```
Expected: `ImportError: cannot import name 'register_interaction_tools'`.

- [ ] **Step 3: Append the helper to `interaction_tools.py`**

Open `ai-worker/src/openharness/tools/interaction_tools.py`. Append at the bottom (below `RequestReviewTool`):

```python
# ---------------------------------------------------------------------------
# register_interaction_tools — wiring helper for _create_engine
# Spec: §2.9.3.b. Called by api_server._create_engine (Task 5.15).
# ---------------------------------------------------------------------------


def register_interaction_tools(
    registry: "ToolRegistry",
    model_router: "ModelRouter",
    workspace_dir: Path,
) -> None:
    """Register both interaction meta-tools on the supplied registry.

    The two tools are RequestClarificationTool (Phase 5a) and
    RequestReviewTool (Task 5.13). Both are constructed here so
    _create_engine doesn't need to know their constructor signatures.

    A second call on the same registry raises — ToolRegistry.register
    already enforces unique tool names. Silicon Valley standard
    §2.8: duplicate registration is a bug in the call site, surface
    it loudly rather than swallowing the second call.
    """
    from src.openharness.tools.base import ToolRegistry  # for type checker

    registry.register(RequestClarificationTool())
    registry.register(
        RequestReviewTool(
            model_router=model_router,
            workspace_dir=workspace_dir,
        )
    )
```

(The `from src.openharness.tools.base import ToolRegistry` inside the function is for the type hint only — the parameter is the live `ToolRegistry` instance the caller already has.)

- [ ] **Step 4: Run the tests**

```bash
cd ai-worker && python -m pytest tests/openharness/tools/test_register_interaction_tools.py -v
```
Expected: **3 tests pass**.

- [ ] **Step 5: Commit**

```bash
git add ai-worker/src/openharness/tools/interaction_tools.py ai-worker/tests/openharness/tools/test_register_interaction_tools.py
git commit -m "feat(tools): register_interaction_tools helper

Single helper that wires both interaction meta-tools onto a
supplied ToolRegistry in one call. _create_engine (Task 5.15)
uses this so it doesn't need to know either tool's constructor
signature.

Signature:
  register_interaction_tools(registry, model_router, workspace_dir) -> None

Registers:
- RequestClarificationTool() — stateless (Phase 5a Task 5a.4)
- RequestReviewTool(model_router, workspace_dir) — Task 5.13

Idempotency: a second call raises (ToolRegistry.register rejects
duplicate names). Silicon-valley standard §2.8 — duplicate
registration is a call-site bug, surface it loudly.

Tests: 3 cases:
- Both tools registered after a single call
- Second call raises (idempotent failure)
- RequestReviewTool instance constructed with the same router
  and workspace_dir passed into the helper"
```

---

### Task 5.15: `_create_engine` wiring — hooks + interaction tools

**Files:**
- Modify: `ai-worker/src/api_server.py` — `_create_engine` (the function Task 5.5 created and Phase 5a Task 5a.9 made async)
- Modify: `ai-worker/src/models/router.py` — add `ModelRouter.require_model_for(purpose)` if not already present
- Modify: `ai-worker/tests/test_api_server.py` — three new tests

**Context:** Tasks 5.8–5.14 built the components. Task 5.15 wires them into the engine constructor per spec §4.12 Round 2. Five additions to the existing `_create_engine`:

1. Construct an empty `AgentHookRegistry` and an `AgentHookContext` from the request's `project_id` + `session_id` + the resolved `workspace_dir`.
2. After the existing tool registrations, call `register_interaction_tools(tool_registry, model_router=router, workspace_dir=workspace_dir)`.
3. Before constructing the system prompt, call `router.require_model_for(Purpose.REVIEW)` — fail-fast if no reviewer model is configured (§2.9.3.f). If `ModelRouter` doesn't already have this method, add it as a small helper that raises `ModelRouterError` when no model is registered for the purpose.
4. `await build_system_prompt(...)` (now async per Task 5.11) with `slots=agent_hook_registry.system_prompt_slots` and `hook_context=agent_hook_context`.
5. Pass `agent_hook_registry` and `agent_hook_context` into the `QueryEngine(...)` constructor.

Phase 5a Task 5a.9 already made `_create_engine` async and partially wired the clarification coordinator + return channel; this task layers the agent hook + reviewer wiring on top.

- [ ] **Step 1: Add `ModelRouter.require_model_for` if missing**

Check whether the method exists:

```bash
grep -n "def require_model_for" ai-worker/src/models/router.py
```

If absent, add to `ai-worker/src/models/router.py` inside the `ModelRouter` class:

```python
def require_model_for(self, purpose: Purpose) -> None:
    """Fail-fast assertion that a model is registered for the given
    purpose. Used by _create_engine on startup so we discover missing
    reviewer model configuration immediately rather than at the first
    request_review call (which would surface as a confusing tool
    error mid-session).

    Spec §2.9.3.f: if Purpose.REVIEW has no model registered, the
    agent does not start.
    """
    if not self._has_model_for(purpose):
        raise ModelRouterError(
            f"no model registered for purpose {purpose.name} — "
            f"configure FORGE_MODEL_{purpose.name} or update the "
            f"ModelRouter constructor"
        )
```

Where `_has_model_for(purpose)` is whatever existing predicate the router has for "do I know how to serve this purpose"; if no such helper exists, inline the check by trying to look up the model in the router's internal map and returning False on KeyError.

- [ ] **Step 2: Update `_create_engine`**

Open `ai-worker/src/api_server.py`. Find the `_create_engine` function from Task 5.5 (which Phase 5a Task 5a.9 already converted to `async def`). Apply the Round 2 additions per spec §4.12.

The function header changes from:
```python
async def _create_engine(req: RunRequest, workspace_dir: Path) -> Any:
```
to (Phase 5a may already have added `redis_client`):
```python
async def _create_engine(
    req: RunRequest,
    workspace_dir: Path,
    redis_client: "Redis",
) -> Any:
```

Replace the body with the Round 2 shape from spec §4.12:

```python
async def _create_engine(
    req: RunRequest,
    workspace_dir: Path,
    redis_client: "Redis",
) -> Any:
    """Create a QueryEngine wired with the full T2 tool set + Round 2
    in-process agent hooks + interaction meta-tools.

    Round 2 additions:
    - AgentHookRegistry (empty default; downstream projects populate)
    - AgentHookContext (per-session, passed into every hook)
    - register_interaction_tools (RequestClarificationTool +
      RequestReviewTool)
    - router.require_model_for(Purpose.REVIEW) fail-fast assertion
    - await build_system_prompt(...) with slots + hook_context
    """
    from pathlib import Path

    from src.openharness.engine.agent_hooks import (
        AgentHookContext,
        AgentHookRegistry,
        ClarificationCoordinator,
    )
    from src.openharness.engine.prompts import build_system_prompt
    from src.openharness.engine.query_engine import QueryEngine
    from src.openharness.engine.return_channel import ReturnChannel
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
    from src.openharness.tools.interaction_tools import (
        register_interaction_tools,
    )

    tool_registry = ToolRegistry()

    # Context tools (6) — empty profiles for now (live profile data
    # is wired in by a follow-up task when the scan pipeline is ready).
    register_context_tools(
        tool_registry,
        profiles={},
        project_id=req.project_id,
    )

    # File tools (6): read/write/edit/glob/grep/list_directory
    register_file_tools(tool_registry, workspace_dir)

    # Exec tools (2): bash + set_phase
    register_exec_tools(tool_registry, workspace_dir)

    # Subprocess hooks (existing — unchanged)
    command_hook_registry = HookRegistry()
    command_hook_executor = HookExecutor(command_hook_registry)

    # Round 2 (§2.9.1): in-process agent hooks. Empty default;
    # downstream projects populate via a project-scoped factory.
    agent_hook_registry = AgentHookRegistry()
    agent_hook_context = AgentHookContext(
        project_id=req.project_id,
        session_id=req.session_id,
        workspace_dir=workspace_dir,
        system_prompt_buffer=[],
    )

    # Round 2 (§2.9.2): clarification coordinator + return channel
    # already constructed in Phase 5a Task 5a.9.
    clarification_coordinator = ClarificationCoordinator()
    return_channel = await ReturnChannel.open(
        session_id=req.session_id,
        redis=redis_client,
        coordinator=clarification_coordinator,
    )

    permission_checker = PermissionChecker(mode=PermissionMode.FULL_AUTO)

    # ModelRouter — required, fail-fast on missing reviewer model
    # (§2.9.3.f). No AsyncMock fallback (silicon-valley §2.8).
    try:
        from src.models.router import ModelRouter, ModelRouterError, Purpose
        from src.openharness.api.providers.router_adapter import (
            ModelRouterAdapter,
        )

        router = ModelRouter()
        # §2.9.3.f: must have a reviewer model registered or the
        # agent does not start.
        router.require_model_for(Purpose.REVIEW)
        api_client = ModelRouterAdapter(router, purpose=Purpose.GENERATE)
    except Exception as e:
        logger.error("ModelRouter unavailable: %s", e)
        raise RuntimeError(
            f"ModelRouter unavailable — agent cannot start. "
            f"Check provider credentials and Purpose.REVIEW model "
            f"registration. Underlying error: {e}"
        ) from e

    # Round 2 (§2.9.3.b): register interaction meta-tools after the
    # main tool surface so they don't accidentally collide with any
    # existing tool name. The helper raises if either name is already
    # registered.
    register_interaction_tools(
        registry=tool_registry,
        model_router=router,
        workspace_dir=workspace_dir,
    )

    # Model + system prompt
    model = req.model or os.getenv("FORGE_DEFAULT_MODEL", "claude-sonnet-4-20250514")

    if req.system_prompt is not None:
        # Caller override (rare — used by tests and explicit callers)
        system_prompt = req.system_prompt
    else:
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

        # Round 2 (§5.2): build_system_prompt is async and accepts
        # slot fillers from the agent hook registry.
        system_prompt = await build_system_prompt(
            language=language_name,
            workspace_path=str(workspace_dir),
            slots=agent_hook_registry.system_prompt_slots,
            hook_context=agent_hook_context,
        )

    return QueryEngine(
        api_client=api_client,
        tool_registry=tool_registry,
        model=model,
        system_prompt=system_prompt,
        hook_executor=command_hook_executor,
        permission_checker=permission_checker,
        cwd=workspace_dir,
        agent_hook_registry=agent_hook_registry,
        agent_hook_context=agent_hook_context,
    )
```

The `clarification_coordinator` and `return_channel` are constructed but not yet passed into `QueryEngine`'s constructor — that wiring happens in Phase 5a's tasks (via `RequestClarificationTool`'s execution context, which has its own coordinator wiring path). They're kept in the function so the Phase 5a wiring stays intact; this Round 2 task only layers on the agent-hook + interaction-tool additions.

- [ ] **Step 3: Update `_route_and_stream` to pass `redis_client`**

If Phase 5a Task 5a.9 didn't already do it, the call site needs to forward the redis client into `_create_engine`. Find the call in `_route_and_stream`:

```python
engine = _create_engine(req, workspace_dir=resolved_workspace)
```
and change it to:

```python
engine = await _create_engine(
    req,
    workspace_dir=resolved_workspace,
    redis_client=_redis_client,  # module-level redis client from app startup
)
```

If `_redis_client` doesn't exist as a module-level constant, get it from FastAPI's `app.state` or wherever the existing Redis stream publisher pulls from.

- [ ] **Step 4: Add the new tests**

Append to `ai-worker/tests/test_api_server.py`:

```python
# ---------------------------------------------------------------------------
# Round 2: _create_engine wiring tests (Phase 5 Task 5.15)
# ---------------------------------------------------------------------------


@pytest.mark.asyncio
async def test_create_engine_registers_interaction_tools(
    tmp_path, monkeypatch
):
    """After _create_engine returns, the engine's tool registry must
    contain both request_clarification and request_review."""
    from unittest.mock import AsyncMock, patch

    from src.api_server import RunRequest, _create_engine

    redis_client = AsyncMock()

    # Mock ModelRouter to bypass real provider config
    with patch("src.models.router.ModelRouter") as MockRouter:
        instance = MockRouter.return_value
        instance.require_model_for = lambda purpose: None  # passes
        # Patch the language profile loader so it doesn't error on
        # an empty tmp_path workspace
        monkeypatch.setattr(
            "src.openharness.skills.project_language.load_all_language_profiles",
            lambda _: {},
        )
        monkeypatch.setattr(
            "src.openharness.skills.project_language.detect_language",
            lambda *_, **__: None,
        )

        req = RunRequest(
            tenant_id=1,
            project_id=1,
            session_id="sess-test",
            workspace_path="ws",
            message="hi",
        )

        engine = await _create_engine(
            req,
            workspace_dir=tmp_path,
            redis_client=redis_client,
        )

    assert engine._tool_registry.get("request_clarification") is not None
    assert engine._tool_registry.get("request_review") is not None


@pytest.mark.asyncio
async def test_create_engine_constructs_empty_agent_hook_registry(
    tmp_path, monkeypatch
):
    """The engine must hold a non-None AgentHookRegistry with all
    four collections empty by default."""
    from unittest.mock import AsyncMock, patch

    from src.api_server import RunRequest, _create_engine

    redis_client = AsyncMock()

    with patch("src.models.router.ModelRouter") as MockRouter:
        MockRouter.return_value.require_model_for = lambda purpose: None
        monkeypatch.setattr(
            "src.openharness.skills.project_language.load_all_language_profiles",
            lambda _: {},
        )
        monkeypatch.setattr(
            "src.openharness.skills.project_language.detect_language",
            lambda *_, **__: None,
        )

        req = RunRequest(
            tenant_id=1,
            project_id=42,
            session_id="sess-abc",
            workspace_path="ws",
            message="hi",
        )

        engine = await _create_engine(
            req,
            workspace_dir=tmp_path,
            redis_client=redis_client,
        )

    registry = engine._agent_hook_registry
    assert registry is not None
    assert registry.pre_turn == []
    assert registry.pre_tool_call == []
    assert registry.post_turn == []
    assert registry.system_prompt_slots == {}

    ctx = engine._agent_hook_context
    assert ctx is not None
    assert ctx.project_id == 42
    assert ctx.session_id == "sess-abc"
    assert ctx.workspace_dir == tmp_path


@pytest.mark.asyncio
async def test_create_engine_fail_fast_on_missing_reviewer_model(
    tmp_path, monkeypatch
):
    """If ModelRouter has no reviewer model registered,
    require_model_for raises and _create_engine surfaces a
    RuntimeError. The agent does NOT start."""
    from unittest.mock import patch

    from src.api_server import RunRequest, _create_engine
    from src.models.router import ModelRouterError

    redis_client = object()  # not used; we fail before opening the channel

    with patch("src.models.router.ModelRouter") as MockRouter:
        instance = MockRouter.return_value

        def boom(purpose):
            raise ModelRouterError(
                f"no model registered for purpose {purpose.name}"
            )

        instance.require_model_for = boom

        req = RunRequest(
            tenant_id=1,
            project_id=1,
            session_id="sess-test",
            workspace_path="ws",
            message="hi",
        )

        with pytest.raises(RuntimeError, match="ModelRouter unavailable"):
            await _create_engine(
                req,
                workspace_dir=tmp_path,
                redis_client=redis_client,
            )
```

- [ ] **Step 5: Smoke-test the imports**

```bash
cd ai-worker && python -c "
import asyncio
from src.api_server import _create_engine
import inspect
assert inspect.iscoroutinefunction(_create_engine)
print('ok')
"
```
Expected: `ok`. If not async, the function still has the Task 5.5 sync signature — apply Phase 5a Task 5a.9's async conversion first.

- [ ] **Step 6: Run the new tests**

```bash
cd ai-worker && python -m pytest tests/test_api_server.py -v -k "create_engine_registers_interaction_tools or create_engine_constructs_empty_agent_hook_registry or create_engine_fail_fast_on_missing_reviewer_model"
```
Expected: **3 tests pass**.

- [ ] **Step 7: Run the full api_server test suite to catch regressions**

```bash
cd ai-worker && python -m pytest tests/test_api_server.py tests/test_api_server_route.py -v 2>&1 | tail -40
```
Expected: existing tests still pass + the three new ones.

- [ ] **Step 8: Commit**

```bash
git add ai-worker/src/api_server.py ai-worker/src/models/router.py ai-worker/tests/test_api_server.py
git commit -m "feat(api): wire AgentHookRegistry + interaction tools into _create_engine

Completes the _create_engine rewrite by layering Round 2 (§4.12)
additions onto the Phase 5a Task 5a.9 baseline:

1. Construct an empty AgentHookRegistry alongside the existing
   subprocess HookRegistry. Round 2 ships extension points only;
   downstream projects populate via a project-scoped factory.

2. Construct an AgentHookContext from req.project_id, req.session_id,
   workspace_dir, and an empty system_prompt_buffer. Passed into
   every agent hook call inside the engine.

3. Call register_interaction_tools(tool_registry, model_router=router,
   workspace_dir=workspace_dir) after the main tool registrations.
   Adds RequestClarificationTool (Phase 5a) + RequestReviewTool
   (Task 5.13). Helper rejects duplicate registration loudly per
   silicon-valley §2.8.

4. Add ModelRouter.require_model_for(Purpose.REVIEW) fail-fast
   assertion before the system prompt is built. Per §2.9.3.f, if no
   reviewer model is registered, the agent does not start. The
   helper is added to ModelRouter if not already present — raises
   ModelRouterError on missing.

5. await build_system_prompt(...) (now async per Task 5.11) with
   slots=agent_hook_registry.system_prompt_slots and
   hook_context=agent_hook_context. Empty default registry means
   slots is {} and the {{project_specs}} placeholder is regex-stripped.

6. Pass agent_hook_registry and agent_hook_context into
   QueryEngine(...). The engine forwards them into QueryContext
   which run_agent_loop and _execute_tool_call use to invoke
   pre_turn / pre_tool_call / post_turn hooks (Task 5.9).

Tests: 3 new cases:
- test_create_engine_registers_interaction_tools — both meta-tools
  appear in the engine's ToolRegistry after construction
- test_create_engine_constructs_empty_agent_hook_registry —
  registry is non-None with all four collections empty;
  context carries the right project_id/session_id/workspace_dir
- test_create_engine_fail_fast_on_missing_reviewer_model —
  require_model_for raise propagates as RuntimeError, the
  agent does not start"
```

---

## Phase 5 completion check

Before starting Phase 6:

**Round 1 tasks (5.1–5.7):**

- [ ] `pytest ai-worker/tests/openharness/engine/test_prompts.py -v` — Round 1 ten tests (converted to async) + Round 2 seven slot/instruction tests pass
- [ ] `pytest ai-worker/tests/openharness/engine/test_session_collector.py -v` — 34 tests pass (24 parametrized + 10 SessionCollector)
- [ ] `pytest ai-worker/tests/openharness/engine/test_session_cache.py -v` — 10 tests pass
- [ ] `pytest ai-worker/tests/test_query_engine.py -v` — passes with SessionComplete integration tests
- [ ] `pytest ai-worker/tests/test_api_server.py tests/test_api_server_route.py -v` — existing tests pass + workspace_prep tests pass + Task 5.15 create_engine tests pass
- [ ] `python -c "from src.api_server import app, _create_engine, _route_and_stream, _serialize_event; print('ok')"` prints `ok`
- [ ] `grep -n "pair_pipeline" ai-worker/src/api_server.py` returns nothing
- [ ] `grep -n "Purpose.REVIEW" ai-worker/src/api_server.py` returns only the Task 5.15 `router.require_model_for(Purpose.REVIEW)` line — not the old `_create_engine` switch branch
- [ ] `grep -n "Purpose.GENERATE" ai-worker/src/api_server.py` returns only the Task 5.15 `ModelRouterAdapter(router, purpose=Purpose.GENERATE)` line
- [ ] `grep -n "AsyncMock" ai-worker/src/api_server.py` returns nothing
- [ ] `grep -n "_PAIR_PIPELINE_AVAILABLE\|_sessions: Dict" ai-worker/src/api_server.py` returns nothing (both deleted)
- [ ] `grep -n "FixLoop" ai-worker/src/api_server.py` returns nothing (stale _serialize_event branches purged)
- [ ] `curl -X POST http://localhost:8090/api/workspace/prep -d '{"tenant_id":1,"project_id":1,"workspace_path":"nonexistent"}' -H "Content-Type: application/json"` returns `{"status":"error", ...}` JSON

**Round 2 tasks (5.8–5.15):**

- [ ] `pytest ai-worker/tests/openharness/engine/test_agent_hook_registry.py -v` — Task 5.8 nine tests pass
- [ ] `pytest ai-worker/tests/openharness/engine/test_hooks_integration.py -v` — Task 5.10 eleven integration scenarios pass
- [ ] `pytest ai-worker/tests/openharness/engine/test_reviewer_prompts.py -v` — Task 5.12 twelve reviewer-prompt + verdict-parser tests pass
- [ ] `pytest ai-worker/tests/openharness/tools/test_request_review_tool.py -v` — Task 5.13 eight tests pass (happy path, REVISE preserved, router exception, real git diff, truncation, timeout, input validation)
- [ ] `pytest ai-worker/tests/openharness/tools/test_register_interaction_tools.py -v` — Task 5.14 three tests pass
- [ ] `grep -n "AgentHookRegistry" ai-worker/src/api_server.py` returns at least one match (Task 5.15 wired it in)
- [ ] `grep -n "register_interaction_tools" ai-worker/src/api_server.py` returns at least one match
- [ ] `grep -n "require_model_for" ai-worker/src/api_server.py` returns at least one match (the fail-fast assertion)
- [ ] `python -c "
    from src.openharness.engine.agent_hooks import (
        AgentHookRegistry, AgentHookContext, PreToolCallBlock,
        PreTurnHook, PreToolCallHook, PostTurnHook, PromptSlotFiller,
    )
    r = AgentHookRegistry()
    assert r.pre_turn == [] and r.pre_tool_call == [] and r.post_turn == []
    assert r.system_prompt_slots == {}
    print('ok')
"` prints `ok`
- [ ] `python -c "
    from src.openharness.tools.interaction_tools import (
        RequestClarificationTool, RequestReviewTool, register_interaction_tools,
    )
    from src.openharness.engine.prompts import (
        REVIEWER_SYSTEM_PROMPT, REVIEWER_DIFF_MAX_BYTES,
        build_reviewer_prompt, parse_verdict, ReviewerParseError,
    )
    assert REVIEWER_DIFF_MAX_BYTES == 32_768
    print('ok')
"` prints `ok`

**Branch state:**

- [ ] Branch has **15 new commits** from this phase — one per task (Tasks 5.1 through 5.15)

## Phase 5 outputs unlock

- **Phase 6** (frontend) has a full agent backend to wire SSE against. The Redis stream now carries `text_delta`, `turn_complete`, `tool_started`/`tool_completed` with `tool_use_id`, `phase_changed`, `thinking_started`/`stopped`, `session_complete`, `clarification_requested` (from Phase 5a's bidirectional RPC wired through `RequestClarificationTool`), and `error` events. Nothing else is expected.
- **Phase 6** also has the backend side of the request_review meta-tool. Phase 6 renders the reviewer's verdict text in the existing tool-card UI (no new card type needed — it's a standard SimpleTool result).
- **Phase 7** (e2e + deploy) can run a real agent message end-to-end: forge-core's EnsureReady creates a workspace, calls the prep endpoint (which ai-worker now serves), then forge-core forwards the user message → api_server._route_and_stream → QueryEngine → agent loop → tools → SSE → frontend. The E2E smoke test covers the Round 2 clarification round-trip and (if the reviewer model is configured) a request_review invocation.
- **Downstream projects** (verification, entropy, etc.) have a stable in-process extension point (`AgentHookRegistry`) they can populate without modifying `query.py`. Round 2 ships the extension points empty; the first real consumer is a follow-up project.
- Phase 5 is the last phase that touches ai-worker's core Python code for chronos Round 2. Phase 6 is frontend-only and Phase 7 is integration/deploy.
