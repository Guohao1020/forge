# pair_pipeline Production Wire-up — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Wire `pair_pipeline.run_pair_pipeline` into the production HTTP path (`/api/projects/:id/agent/chat` → forge-core → ai-worker `/api/run`) so chat messages on projects with a cloned workspace run through the full coder → BuildVerify → reviewer → fix loop. Chat on projects without a workspace continues to fall back to the single-shot QueryEngine path.

**Architecture:** Approach 2 from the design spec — forge-core computes `workspace_path` via its existing `workspace.Manager` and passes it in the `aiRunRequest` HTTP body. ai-worker's `_run_and_publish` gains a `_route_and_stream` iterator that switches between pair_pipeline and QueryEngine based on whether `workspace_path` is set and the directory exists on disk. A docker-compose volume mount makes the forge-core-owned workspace directory visible inside the ai-worker container.

**Tech Stack:** Go 1.22 + Gin (forge-core), Python 3.12 + FastAPI + LangGraph (ai-worker), Redis Streams (SSE), PostgreSQL (durable history), Docker Compose (dev env).

**Spec:** [`docs/plans/2026-04-08-pair-pipeline-production-wire-design.md`](./2026-04-08-pair-pipeline-production-wire-design.md)

**Follows:** [`docs/plans/2026-04-07-agent-base-loop-reduction.md`](./2026-04-07-agent-base-loop-reduction.md) (the PR that shipped pair_pipeline as an e2e-only capability)

---

## 🚨 LATE-BREAKING PROTOCOL AMENDMENT (added 2026-04-09 during Phase 0)

Phase 0 Task 0.2 Step 6 revealed a cross-filesystem path-prefix problem:
**forge-core runs on the Windows host** and its `workspace.Manager.ProjectDir()`
returns an absolute Windows path like `D:/forge-workspaces/tenant-1/project-999/repo`.
**ai-worker runs in a Linux Docker container** and sees that same directory at
`/data/forge/workspaces/tenant-1/project-999/repo` via the volume mount.
`os.path.isdir("D:/...")` inside the Linux container will always return False,
no matter how correct the volume mount is.

**Corrected protocol — `workspace_path` is a RELATIVE path, not absolute:**

Wherever the plan (and the original spec) show code that puts an absolute path
into `workspace_path` or calls `os.path.isdir(req.workspace_path)` directly,
implementers MUST instead:

### forge-core side (service.go::SubmitMessage)

Replace the "put absolute path" logic with a relative fragment derived from
`tenant_id` and `project_id`:

```go
// Populate workspace_path as a RELATIVE fragment. forge-core and
// ai-worker each join it to their own FORGE_WORKSPACE_ROOT so the
// protocol works across the host/container filesystem split.
if s.wsManager != nil && tenantID > 0 {
    absDir := s.wsManager.ProjectDir(tenantID, projectID)
    gitDir := filepath.Join(absDir, ".git")
    if _, err := os.Stat(gitDir); err == nil {
        // Reduce to relative fragment; the format matches
        // workspace.Manager.ProjectDir deterministically.
        body.WorkspacePath = fmt.Sprintf("tenant-%d/project-%d/repo", tenantID, projectID)
    } else if !os.IsNotExist(err) {
        slog.Warn("agent service: unexpected stat error on .git dir, treating as missing",
            "tenant_id", tenantID,
            "project_id", projectID,
            "path", gitDir,
            "error", err,
        )
    }
}
```

Note: the existence check still uses the ABSOLUTE host path (correct — forge-core
lives on the host and that's the path it can stat). Only the value SENT over HTTP
is relative.

### ai-worker side (api_server.py::_route_and_stream)

The router must join `workspace_path` with ai-worker's own `FORGE_WORKSPACE_ROOT`
env var (set to `/data/forge/workspaces` inside the container via docker-compose):

```python
# Decide routing
use_pair_pipeline = False
resolved_workspace: Optional[str] = None  # absolute path inside the container
if req.workspace_path:
    ws_root = os.environ.get("FORGE_WORKSPACE_ROOT", "/data/forge/workspaces")
    resolved_workspace = os.path.join(ws_root, req.workspace_path)
    if os.path.isdir(resolved_workspace):
        use_pair_pipeline = True
    else:
        logger.warning(
            "workspace_path %r resolved to %r but directory does not exist "
            "— falling back to QueryEngine (check docker volume mount + "
            "FORGE_WORKSPACE_ROOT env)",
            req.workspace_path,
            resolved_workspace,
        )
```

And when calling pair_pipeline, pass the RESOLVED (absolute container-side) path:

```python
config = PairPipelineConfig(project_dir=Path(resolved_workspace))
```

### Tests must reflect the amendment

- **Task 2.1 `test_run_request_accepts_workspace_path`**: the example value should
  be `"tenant-1/project-1/repo"`, not `"/data/forge/workspaces/tenant-1/project-1/repo"`.
- **Task 2.3a `test_route_nonexistent_workspace_falls_back`**: use
  `workspace_path="bogus/tenant-999/project-999/repo"` (something that won't resolve
  under any reasonable FORGE_WORKSPACE_ROOT).
- **Task 2.3b `test_route_valid_workspace_uses_pair_pipeline`**: set
  `monkeypatch.setenv("FORGE_WORKSPACE_ROOT", str(tmp_path))` and pass
  `workspace_path="repo"`, then `workspace = tmp_path / "repo"; workspace.mkdir()`
  — so the test drives the resolved path through the env var rather than hardcoding
  an absolute path.
- **Task 3.1 `TestSubmitMessage_PassesWorkspacePath_WhenRepoExists`**: assert
  `captured.WorkspacePath == "tenant-1/project-42/repo"`, not
  `captured.WorkspacePath == projectDir`.

### Why this amendment

1. The original plan assumed forge-core and ai-worker share a filesystem view. They
   don't — forge-core is on the host, ai-worker is in a container with a volume mount.
2. Relative path keeps the protocol portable: future deployments where forge-core
   also moves into docker-compose (TODOS Dev Experience #3) will not require any
   protocol change, because both sides will still use relative + their own env var.
3. The alternative (ai-worker doing path rewriting) would encode host-specific
   knowledge into ai-worker code, which is the wrong layer.

**Implementer subagents**: when you see absolute-path code in the detailed Task
2.3a/b/c or Task 3.1 sections below, MIRROR the amendment above. If you're ever
unsure, the amendment is authoritative.

---

## Pre-Flight Checklist

Before starting Phase 0, verify:

- [ ] Current branch is `main`, working tree clean except expected artifacts (`.claude/settings.local.json` drift, plan docs, `docs/plans/2026-04-08-pair-pipeline-production-wire-design.md`). Run `git status --short`.
- [ ] `forge-ai-worker` container is up and healthy: `docker compose -f docker-compose.dev.yml ps forge-ai-worker` shows `Up ... (healthy)`.
- [ ] `pytest -m e2e` in `ai-worker/` still passes against `DASHSCOPE_API_KEY` (sanity check that the prior PR is still alive).
- [ ] forge-core binary is responsive: `curl -s http://localhost:8080/health` returns 200.

If any check fails, fix the environment before writing code.

---

## File Structure

### Files created

| Path | Purpose |
|------|---------|
| `ai-worker/tests/openharness/test_stream_events_base.py` | One test asserting all `stream_events.py` dataclasses inherit from the new `StreamEvent` base class. |
| `ai-worker/tests/test_api_server_route.py` | Six tests covering `_route_and_stream` routing matrix: empty workspace, valid workspace, nonexistent workspace, non-StreamEvent filtering, exception propagation, Purpose injection. |
| `forge-core/internal/module/agent/service_test.go` | Two new tests asserting `SubmitMessage` sends `WorkspacePath` correctly based on disk state. |

### Files modified

| Path | What changes |
|------|--------------|
| `ai-worker/src/openharness/engine/stream_events.py` | Add empty `class StreamEvent: pass` base class; make all 10 existing event dataclasses inherit from it. No runtime behavior change. |
| `ai-worker/src/api_server.py` | `RunRequest` gains `workspace_path`; `_create_engine` gains `purpose` parameter with a REVIEW system prompt branch; extract new `_route_and_stream(req, session_id, correlation_id)` iterator; `_run_and_publish` calls `_route_and_stream` instead of `engine.submit_message`. |
| `forge-core/internal/module/agent/service.go` | Constructor accepts `*workspace.Manager` (nilable); `SubmitMessage` signature gains `tenantID int64` parameter; method computes `WorkspacePath` when wsManager non-nil, tenantID non-zero, and `.git` exists on disk. |
| `forge-core/internal/module/agent/handler.go` | Both `SubmitMessage` call sites updated: legacy path (L151) passes `0` for tenantID; dual-storage path (L223) passes the real `tenantID` from `currentUser(c)`. |
| `forge-core/internal/module/agent/handler_test.go` | The single `newTestService` helper (1 call site) gets `nil` as the second argument. |
| `forge-core/cmd/forge-core/main.go` | L245 passes existing `workspaceMgr` to `agent.NewService`. |
| `docker-compose.dev.yml` | `forge-ai-worker` service: add `volumes` entry for workspace root, add `FORGE_WORKSPACE_ROOT=/data/forge/workspaces` environment variable. |
| `ai-worker/.env` | Add `FORGE_WORKSPACE_ROOT=/data/forge/workspaces` (container-internal path). |
| `docs/TODOS.md` | Move #5 from pending to done; add 3 new P2 entries (session concurrency lock, mid-stream crash recovery, ModelRouter Purpose differentiation). |
| `docs/technical-design.md §3.7` | Update "Known Leftovers" to reflect that `pair_pipeline` is now on the production path. |

---

## Phase 0 — Prerequisite validation (no code, just verification)

**Goal:** Confirm that docker volume mount works on Harvey's Windows host BEFORE writing any code. This is a GATE — if volume mount is broken, the whole PR is blocked until it's fixed.

### Task 0.1: Create a seed workspace directory on the host

**Files:**
- Create on host (not in repo): a minimal Go project at the location that `workspace.Manager` would produce.

The default `FORGE_WORKSPACE_ROOT` is `./workspaces` in `config.go:65`, meaning relative to forge-core's CWD. For Harvey's dev env on Windows this likely resolves to `D:/shulex_work/forge/forge-core/workspaces/`. We want a path that is writable from both the Windows host and reachable inside the Linux docker container via a volume mount.

Choice: use `D:/forge-workspaces/` as the host path. This is outside the git repo (safer), short, and easy to mount. We'll set `FORGE_WORKSPACE_ROOT=D:/forge-workspaces` later for forge-core and `/data/forge/workspaces` inside the container.

- [ ] **Step 1: Create the seed directory structure on the host**

```bash
mkdir -p "D:/forge-workspaces/tenant-1/project-999/repo"
cd "D:/forge-workspaces/tenant-1/project-999/repo"
git init -b main
```

Expected: directory exists, `git init` reports "Initialized empty Git repository".

- [ ] **Step 2: Seed with a one-file Go module**

```bash
cat > go.mod <<'EOF'
module forge-test/project-999

go 1.22
EOF

cat > main.go <<'EOF'
package main

import "fmt"

func main() {
    fmt.Println("hello from forge workspace")
}
EOF

git add go.mod main.go
git commit -m "seed: initial Go module for pair_pipeline e2e"
```

Expected: git commit succeeds, `ls -la .git` confirms repo exists.

- [ ] **Step 3: Verify `go build` works locally (sanity check the seed)**

```bash
go build ./...
```

Expected: no output (success), no binary produced (not in a `cmd/` subdir).

### Task 0.2: Add the docker-compose volume mount (GATE)

**Files:**
- Modify: `D:\shulex_work\forge\docker-compose.dev.yml` — `forge-ai-worker` service

- [ ] **Step 1: Read current `forge-ai-worker` service definition**

```bash
grep -A 20 "forge-ai-worker:" docker-compose.dev.yml
```

Identify the existing `volumes:` block (if any) and `environment:` block.

- [ ] **Step 2: Add volume mount + env variable**

Modify the `forge-ai-worker` service to add:

```yaml
    volumes:
      - ${FORGE_WORKSPACE_ROOT_HOST:-./workspaces}:/data/forge/workspaces
    environment:
      FORGE_WORKSPACE_ROOT: /data/forge/workspaces
```

If `volumes:` or `environment:` already exist, append these entries. If not, add the blocks. Keep existing entries intact.

Note: `FORGE_WORKSPACE_ROOT_HOST` is a new variable used only in docker-compose.dev.yml to let Harvey point the mount at `D:/forge-workspaces` without changing code. Default `./workspaces` preserves existing behavior for other contributors.

- [ ] **Step 3: Tell Harvey to set the host env var and restart forge-ai-worker**

Do not run this yourself — ask Harvey to:

```bash
export FORGE_WORKSPACE_ROOT_HOST="D:/forge-workspaces"
# (or set it in a .env file at repo root that docker-compose picks up)
docker compose -f docker-compose.dev.yml up -d forge-ai-worker --force-recreate
```

- [ ] **Step 4: GATE — verify the volume mount is visible inside the container**

```bash
docker exec forge-ai-worker ls -la /data/forge/workspaces/tenant-1/project-999/repo/
```

Expected output must include `go.mod` and `main.go`.

If this fails: STOP. The volume mount is not working and no further Phase will help. Debug until the expected output appears.

- [ ] **Step 5: GATE — verify forge-ai-worker has a `go` toolchain (or explicitly accept the degraded mode)**

```bash
docker exec -w /data/forge/workspaces/tenant-1/project-999/repo forge-ai-worker sh -c "which go && go build ./..."
```

Two possible outcomes, each with a different implication for Phase 4's success criterion:

**Outcome A: `go` is present, `go build` succeeds.** Phase 4's success criterion is `session_complete` with `build_status="passed"`. Proceed.

**Outcome B: `go` is absent (`which go` returns nothing or errors).** `pair_pipeline`'s `BuildVerifyHook` will either skip BuildVerify entirely (language detection returns None) or fail the build. Phase 4's success criterion must be **degraded** to `session_complete` with `build_status="skipped"` OR `build_status="failed"` — what we're proving is end-to-end plumbing (pair_pipeline ran, emitted events, written to PG), NOT that the LLM actually produced compilable Go code. Installing `go` in the ai-worker image is a separate concern (TODOS #1, workspace sandbox with toolchain containers).

The implementer MUST pick A or B before continuing to Phase 1. Document the choice in the Phase 4 gate.

- [ ] **Step 6: GATE — verify forge-core's `cfg.WorkspaceRoot` is aligned with the docker mount**

The ai-worker container mounts `${FORGE_WORKSPACE_ROOT_HOST:-./workspaces}` at `/data/forge/workspaces`. forge-core writes into `cfg.WorkspaceRoot` read from the env var `FORGE_WORKSPACE_ROOT` (default `./workspaces` relative to forge-core's CWD). For `wsManager.ProjectDir()` on the forge-core side and the path the ai-worker sees inside the container to refer to the same bytes on disk, these two must be aligned.

Check Harvey's forge-core environment:

```bash
# Inspect the environment forge-core is running with
cat forge-core/.env 2>/dev/null | grep FORGE_WORKSPACE_ROOT
# Or, if forge-core is spawned via CC:
echo $FORGE_WORKSPACE_ROOT
```

Expected: the value resolves to the SAME Windows host path as `FORGE_WORKSPACE_ROOT_HOST` in docker-compose (e.g., both = `D:/forge-workspaces`).

If not aligned: set `FORGE_WORKSPACE_ROOT=D:/forge-workspaces` in `forge-core/.env` and restart forge-core in Phase 4 Task 4.1. Failure to align means forge-core writes to one directory, ai-worker reads from another, and Phase 4 fails with "workspace_path exists but directory empty" — a confusing failure mode.

---

## Phase 1 — StreamEvent base class refactor (no functional change)

**Goal:** Replace the existing `StreamEvent = Union[...]` type alias in `stream_events.py` with an empty `class StreamEvent: pass` marker base class, and make all 10 event dataclasses inherit from it. This unblocks `isinstance(event, StreamEvent)` filtering in Phase 2. Runtime behavior is unchanged — Python type annotations are not enforced at runtime, so existing `AsyncIterator[StreamEvent]` annotations in `query.py` / `query_engine.py` continue to work with the new class-based name.

**Current state (verified 2026-04-09):** `ai-worker/src/openharness/engine/stream_events.py:96` already has `StreamEvent = Union[AssistantTextDelta, ..., SessionComplete]`. This Union is imported as a TYPE (not an instance source) by:
- `query_engine.py:19`: `from .stream_events import AssistantTurnComplete, StreamEvent`
- `query.py`: type annotation only
- `engine/__init__.py`: re-export

Python does not care at runtime whether `StreamEvent` is a Union or a class for type annotation purposes — both are valid in `-> AsyncIterator[StreamEvent]`. But **`isinstance(x, StreamEvent)` fails on a Union** (`TypeError: issubclass() arg 2 must be a class, a tuple of classes, or a union`). Phase 1's job is to swap the definition so `isinstance` works.

### Task 1.1: Replace the Union with a class-based `StreamEvent`

**Files:**
- Modify: `D:\shulex_work\forge\ai-worker\src\openharness\engine\stream_events.py`

- [ ] **Step 1: Write the failing test**

Create `D:\shulex_work\forge\ai-worker\tests\openharness\test_stream_events_base.py`:

```python
"""StreamEvent marker base class contract.

Every stream event dataclass emitted by the pipeline must inherit from
StreamEvent so `isinstance(event, StreamEvent)` filtering works in
api_server._route_and_stream.
"""

from __future__ import annotations

import pytest

from src.openharness.engine.stream_events import (
    AssistantTextDelta,
    AssistantTurnComplete,
    ErrorEvent,
    FixLoopCompleted,
    FixLoopStarted,
    SessionComplete,
    StreamEvent,
    ThinkingStarted,
    ThinkingStopped,
    ToolExecutionCompleted,
    ToolExecutionStarted,
)


ALL_EVENT_CLASSES = [
    AssistantTextDelta,
    AssistantTurnComplete,
    ToolExecutionStarted,
    ToolExecutionCompleted,
    ErrorEvent,
    ThinkingStarted,
    ThinkingStopped,
    FixLoopStarted,
    FixLoopCompleted,
    SessionComplete,
]


@pytest.mark.parametrize("cls", ALL_EVENT_CLASSES)
def test_stream_event_classes_inherit_from_stream_event(cls):
    """All 10 event dataclasses must be StreamEvent subclasses."""
    assert issubclass(cls, StreamEvent), (
        f"{cls.__name__} must inherit from StreamEvent so "
        f"isinstance-based filtering in _route_and_stream works."
    )
```

- [ ] **Step 2: Run test to verify it fails**

```bash
cd D:/shulex_work/forge/ai-worker
pytest tests/openharness/test_stream_events_base.py -v
```

Expected: the import of `StreamEvent` SUCCEEDS (it's already the Union alias), but `issubclass(cls, StreamEvent)` raises `TypeError: issubclass() arg 2 must be a class, a tuple of classes, or a union`. All 10 parametrized cases FAIL with that TypeError.

- [ ] **Step 3: Replace the Union with a class-based base + make all dataclasses inherit**

Edit `D:\shulex_work\forge\ai-worker\src\openharness\engine\stream_events.py`.

**Step 3a:** At the top of the file, right before the first `@dataclass`, add:

```python
class StreamEvent:
    """Marker base class for all stream events emitted by pipeline iterators.

    Used by api_server._route_and_stream to filter out non-event yields
    (such as CycleResult or PairPipelineResult) via isinstance checks.
    Empty on purpose: no shared state, no methods.
    """
    pass
```

**Step 3b:** Change each of the 10 existing event dataclasses to inherit from `StreamEvent`:

```python
@dataclass
class AssistantTextDelta(StreamEvent):
    ...

@dataclass
class AssistantTurnComplete(StreamEvent):
    ...

# ...repeat for all 10: ToolExecutionStarted, ToolExecutionCompleted,
# ErrorEvent, ThinkingStarted, ThinkingStopped, FixLoopStarted,
# FixLoopCompleted, SessionComplete
```

Do NOT rename any class. Do NOT add fields. Just add the base class to each `class X(StreamEvent)` line.

**Step 3c:** DELETE the existing `StreamEvent = Union[...]` type alias at line ~96 entirely. The new class-based `StreamEvent` name now shadows the old Union. `typing.Union` may no longer be imported — if nothing else in the file uses it, drop the `Union` import from the `typing` line at the top.

**Step 3d:** Verify existing consumers of `StreamEvent` as a type annotation still work. Do NOT edit them — just confirm by reading these files that the name `StreamEvent` is still importable:
- `ai-worker/src/openharness/engine/query_engine.py:19` — `from .stream_events import AssistantTurnComplete, StreamEvent`
- `ai-worker/src/openharness/engine/query.py` (grep for `StreamEvent`)
- `ai-worker/src/openharness/engine/__init__.py` (grep for `StreamEvent`)

Python does not care at runtime whether a type annotation is a Union or a class — both work in `AsyncIterator[StreamEvent]`. Static type checkers (mypy/pyright) MAY warn about narrowing, but that's acceptable in this codebase (we don't run pyright in CI).

- [ ] **Step 4: Run the test to verify it passes**

```bash
pytest tests/openharness/test_stream_events_base.py -v
```

Expected: 10 parametrized cases all PASS.

- [ ] **Step 5: Smoke-import the downstream modules that reference `StreamEvent`**

```bash
python -c "from src.openharness.engine.query_engine import QueryEngine; print('ok')"
python -c "from src.openharness.engine import query; print('ok')"
python -c "from src.openharness.engine import StreamEvent; print(type(StreamEvent))"
```

Expected: all three succeed, the third prints `<class 'type'>` (confirming `StreamEvent` is now a class, not a Union).

If any import fails with `ImportError: cannot import name 'Union'`, you dropped the `typing.Union` import prematurely — restore it.

- [ ] **Step 6: Run the full ai-worker unit suite to verify no regression**

```bash
pytest tests/ -m "not e2e" -q
```

Expected: same green/pass count as before Phase 1 + the 10 new parametrized cases. Zero failures.

- [ ] **Step 7: Commit**

```bash
git add ai-worker/src/openharness/engine/stream_events.py ai-worker/tests/openharness/test_stream_events_base.py
git commit -m "refactor(ai-worker): StreamEvent marker base class for type narrowing

Replaces the existing StreamEvent = Union[...] type alias with an empty
StreamEvent marker base class and makes all 10 event dataclasses in
stream_events.py inherit from it. Runtime behavior unchanged — Python
type annotations are not enforced at runtime, so downstream consumers
of AsyncIterator[StreamEvent] (query_engine.py, query.py) continue to
work without edits.

Enables isinstance(event, StreamEvent) filtering in
api_server._route_and_stream so non-event yields from pair_pipeline
(CycleResult, PairPipelineResult) can be dropped cleanly.

Preparation for TODOS #5 (pair_pipeline production wire-up)."
```

---

## Phase 2 — ai-worker router + Purpose support (feature gated by workspace_path)

**Goal:** Teach `api_server.py` to route chat messages either to `pair_pipeline` (when `workspace_path` is set and the directory exists) or to the existing `QueryEngine.submit_message` path (otherwise). Extend `_create_engine` to support creating a reviewer engine with a different system prompt. This phase lands the new capability but it stays dormant until Phase 4 populates `workspace_path` from forge-core.

### Task 2.1: Extend `RunRequest` with `workspace_path`

**Files:**
- Modify: `D:\shulex_work\forge\ai-worker\src\api_server.py`

- [ ] **Step 1: Write the failing test**

Create `D:\shulex_work\forge\ai-worker\tests\test_api_server_route.py`:

```python
"""Routing contract for api_server._route_and_stream.

_route_and_stream decides between the pair_pipeline path and the legacy
QueryEngine path based on whether RunRequest.workspace_path is set and
the directory exists on disk. This test file pins that contract.
"""

from __future__ import annotations

import asyncio
import pytest
from unittest.mock import AsyncMock, MagicMock, patch

from src.api_server import RunRequest


def test_run_request_accepts_workspace_path():
    """RunRequest must accept an optional workspace_path field."""
    req = RunRequest(
        project_id=1,
        message="hello",
        workspace_path="/data/forge/workspaces/tenant-1/project-1/repo",
    )
    assert req.workspace_path == "/data/forge/workspaces/tenant-1/project-1/repo"


def test_run_request_workspace_path_is_optional():
    """Legacy callers that don't set workspace_path must still work."""
    req = RunRequest(project_id=1, message="hello")
    assert req.workspace_path is None
```

- [ ] **Step 2: Run test to verify it fails**

```bash
cd D:/shulex_work/forge/ai-worker
pytest tests/test_api_server_route.py::test_run_request_accepts_workspace_path -v
```

Expected: Pydantic `ValidationError` — extra field `workspace_path` not permitted.

- [ ] **Step 3: Add the `workspace_path` field**

In `D:\shulex_work\forge\ai-worker\src\api_server.py`, modify the `RunRequest` class:

```python
class RunRequest(BaseModel):
    session_id: Optional[str] = None
    project_id: int
    workspace_path: Optional[str] = None
    message: str
    model: Optional[str] = None
    system_prompt: Optional[str] = None
    correlation_id: Optional[str] = None
```

- [ ] **Step 4: Run the tests to verify they pass**

```bash
pytest tests/test_api_server_route.py -v
```

Expected: both tests PASS.

- [ ] **Step 5: Commit**

```bash
git add ai-worker/src/api_server.py ai-worker/tests/test_api_server_route.py
git commit -m "feat(ai-worker): add workspace_path to RunRequest

Optional field, nullable. Dormant until _route_and_stream is added
in the next commit. Callers (forge-core agent.Service) will populate
this in a subsequent commit."
```

### Task 2.2: Extend `_create_engine` with `purpose` parameter

**Files:**
- Modify: `D:\shulex_work\forge\ai-worker\src\api_server.py` — `_create_engine` function

- [ ] **Step 1: Write the failing test**

Append to `D:\shulex_work\forge\ai-worker\tests\test_api_server_route.py`:

```python
from src.api_server import _create_engine
from src.models.router import Purpose


def test_create_engine_defaults_to_generate_purpose():
    """Backwards compat: _create_engine called without purpose should
    still work, defaulting to Purpose.GENERATE."""
    req = RunRequest(project_id=1, message="hello")
    engine = _create_engine(req)
    assert engine is not None
    # QueryEngine stores the resolved system_prompt in a private attribute
    # `_system_prompt` (verified at query_engine.py:40). Reach through it
    # for the assertion — keeping the coupling explicit.
    assert "helpful" in engine._system_prompt.lower()


def test_create_engine_accepts_review_purpose():
    """New capability: _create_engine called with Purpose.REVIEW should
    produce an engine configured with the reviewer system prompt."""
    req = RunRequest(project_id=1, message="hello")
    engine = _create_engine(req, purpose=Purpose.REVIEW)
    assert engine is not None
    # The reviewer prompt must include "APPROVE" and "REVISE" as literal
    # tokens because pair_pipeline's _parse_review_decision matches on
    # those exact prefixes. This is a load-bearing contract.
    assert "APPROVE" in engine._system_prompt
    assert "REVISE" in engine._system_prompt


def test_create_engine_user_prompt_overrides_purpose_default():
    """If the caller provides system_prompt explicitly, it wins over the
    purpose-derived default. This is how the legacy single-shot path
    keeps its own prompt when it gets wired through the new router."""
    req = RunRequest(
        project_id=1,
        message="hello",
        system_prompt="Custom prompt for this caller only.",
    )
    engine = _create_engine(req, purpose=Purpose.REVIEW)
    assert engine._system_prompt == "Custom prompt for this caller only."
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
pytest tests/test_api_server_route.py::test_create_engine_accepts_review_purpose -v
```

Expected: TypeError — `_create_engine() got an unexpected keyword argument 'purpose'`.

- [ ] **Step 3: Extend `_create_engine`**

In `D:\shulex_work\forge\ai-worker\src\api_server.py`, replace the existing `_create_engine` function (lines ~100-140):

```python
def _create_engine(req: RunRequest, purpose: "Purpose | None" = None) -> Any:
    """Create a QueryEngine for a new session.

    purpose controls the ModelRouter routing and the default system
    prompt. Purpose.GENERATE (default) gets the coder prompt;
    Purpose.REVIEW gets the reviewer prompt used by pair_pipeline's
    reviewer engine.
    """
    from src.openharness.engine.query_engine import QueryEngine
    from src.openharness.tools.base import ToolRegistry
    from src.openharness.hooks.loader import HookRegistry
    from src.openharness.hooks.executor import HookExecutor
    from src.openharness.permissions.checker import PermissionChecker
    from src.openharness.permissions.modes import PermissionMode
    from src.models.router import Purpose as _Purpose

    if purpose is None:
        purpose = _Purpose.GENERATE

    # Try to load model router adapter
    try:
        from src.models.router import ModelRouter
        from src.openharness.api.providers.router_adapter import ModelRouterAdapter
        router = ModelRouter()
        api_client = ModelRouterAdapter(router, purpose=purpose)
    except Exception as e:
        logger.warning("ModelRouter not available, using mock: %s", e)
        from unittest.mock import AsyncMock
        api_client = AsyncMock()

    # Load registries
    tool_registry = ToolRegistry()
    hook_registry = HookRegistry()
    hook_executor = HookExecutor(hook_registry)
    permission_checker = PermissionChecker(mode=PermissionMode.FULL_AUTO)

    model = req.model or os.getenv("FORGE_DEFAULT_MODEL", "claude-sonnet-4-20250514")

    if req.system_prompt is not None:
        system_prompt = req.system_prompt
    elif purpose == _Purpose.REVIEW:
        system_prompt = (
            "You are a strict code reviewer. You MUST respond with exactly "
            "one of these three forms:\n"
            "- APPROVE (if the code is correct and production-ready)\n"
            "- REVISE <specific changes needed>\n"
            "- REJECT <reason why the approach is fundamentally wrong>\n"
            "Be terse. Do not ramble."
        )
    else:
        system_prompt = "You are a helpful AI coding assistant."

    return QueryEngine(
        api_client=api_client,
        tool_registry=tool_registry,
        model=model,
        system_prompt=system_prompt,
        hook_executor=hook_executor,
        permission_checker=permission_checker,
    )
```

Note: the `"Purpose | None"` forward-reference string in the type hint avoids a top-of-file import that would fail at test collection time if models.router is not importable. The import happens inside the function body.

- [ ] **Step 4: Run tests to verify they pass**

```bash
pytest tests/test_api_server_route.py -v
```

Expected: all four tests in the file PASS.

- [ ] **Step 5: Run the full ai-worker suite to verify no regression**

```bash
pytest tests/ -m "not e2e" -q
```

Expected: still green.

- [ ] **Step 6: Commit**

```bash
git add ai-worker/src/api_server.py ai-worker/tests/test_api_server_route.py
git commit -m "feat(ai-worker): _create_engine supports Purpose parameter

Defaults to Purpose.GENERATE preserving all existing callers.
Purpose.REVIEW gets a reviewer-specific system prompt for use by
pair_pipeline's reviewer engine in the next commit.

Prep for TODOS #5."
```

### Task 2.3a: Skeleton `_route_and_stream` with legacy fallback only

**Files:**
- Modify: `D:\shulex_work\forge\ai-worker\src\api_server.py` — introduce `_route_and_stream` that only handles the empty-workspace fallback path.

This slice introduces the iterator seam without touching pair_pipeline logic yet. Tests prove the empty-workspace path is preserved byte-for-byte.

- [ ] **Step 1: Write the failing tests**

Append to `D:\shulex_work\forge\ai-worker\tests\test_api_server_route.py`:

```python
import os
from pathlib import Path

from src.api_server import _route_and_stream
from src.openharness.engine.stream_events import (
    AssistantTextDelta,
    SessionComplete,
    StreamEvent,
)


# Fake events emitted by mock iterators — shared across 2.3a/b/c tests
def _fake_query_engine_iter():
    """Mimic engine.submit_message(): yield a single AssistantTextDelta."""
    async def _gen():
        yield AssistantTextDelta(text="hello from QueryEngine")
    return _gen()


@pytest.mark.asyncio
async def test_route_empty_workspace_uses_queryengine(monkeypatch):
    """workspace_path is None → QueryEngine path yields its events through."""
    req = RunRequest(project_id=1, message="hello")

    qe_called = {"v": 0}

    def fake_qe(r, purpose=None):
        qe_called["v"] += 1
        mock = MagicMock()
        mock.submit_message = MagicMock(return_value=_fake_query_engine_iter())
        return mock

    monkeypatch.setattr("src.api_server._create_engine", fake_qe)

    events = []
    async for ev in _route_and_stream(req, "sid-1", "corr-1"):
        events.append(ev)

    assert qe_called["v"] == 1, "QueryEngine must be created when workspace is empty"
    assert len(events) == 1
    assert isinstance(events[0], AssistantTextDelta)


@pytest.mark.asyncio
async def test_route_nonexistent_workspace_falls_back(monkeypatch, caplog):
    """workspace_path is set but the directory does not exist → fallback
    to QueryEngine. Misconfigured volume mount must not fail the chat;
    fall back and log a WARN."""
    req = RunRequest(
        project_id=1,
        message="hello",
        workspace_path="/nonexistent/path/that/does/not/exist",
    )

    qe_called = {"v": 0}

    def fake_qe(r, purpose=None):
        qe_called["v"] += 1
        mock = MagicMock()
        mock.submit_message = MagicMock(return_value=_fake_query_engine_iter())
        return mock

    monkeypatch.setattr("src.api_server._create_engine", fake_qe)

    import logging
    with caplog.at_level(logging.WARNING):
        events = []
        async for ev in _route_and_stream(req, "sid-1", "corr-1"):
            events.append(ev)

    assert qe_called["v"] == 1
    # WARN log must mention the misconfigured path
    assert any("workspace_path" in r.message for r in caplog.records)
```

Note: in 2.3a we are NOT yet testing the pair_pipeline path. Task 2.3b adds that. This slice only proves that `_route_and_stream` exists as an async iterator, that the empty-workspace case delegates to QueryEngine identically to today, and that a misconfigured workspace_path produces a logged fallback (which is what production will do on day one before Phase 3 ships, and on day N when a volume mount is broken).

- [ ] **Step 2: Run the tests to verify they fail**

```bash
pytest tests/test_api_server_route.py -v
```

Expected: the 2 new routing tests FAIL with `ImportError: cannot import name '_route_and_stream' from 'src.api_server'`.

- [ ] **Step 3: Add `_route_and_stream` skeleton with legacy-fallback only**

In `D:\shulex_work\forge\ai-worker\src\api_server.py`:

a. Verify `os` is already imported (it should be). Do NOT add `Path` yet — we'll add it in Task 2.3b when we actually reference it.

b. After `_create_engine` (around line 140), add the new iterator. In Task 2.3a we only handle the empty-workspace fallback. The pair_pipeline branch is stubbed as a comment and will be filled in by Task 2.3b:

```python
async def _route_and_stream(
    req: RunRequest,
    session_id: str,
    correlation_id: str,
):
    """Route a chat message to either pair_pipeline (when workspace_path
    is set and the directory exists) or the legacy QueryEngine path.

    Async generator. Yields only StreamEvent instances. Exceptions
    propagate to the caller (_run_and_publish) which turns them into
    ErrorEvent.
    """
    # Decide routing
    use_pair_pipeline = False
    if req.workspace_path:
        if os.path.isdir(req.workspace_path):
            use_pair_pipeline = True
        else:
            logger.warning(
                "workspace_path is set but directory does not exist: %s "
                "— falling back to QueryEngine (check docker volume mount)",
                req.workspace_path,
            )

    if not use_pair_pipeline:
        # Legacy path: single-shot QueryEngine. Reuse session engine
        # from _sessions when present (continuity across messages) or
        # create a fresh one on first message.
        engine = _sessions.get(session_id)
        if engine is None:
            engine = _create_engine(req)
            _sessions[session_id] = engine
        async for event in engine.submit_message(req.message):
            yield event
        return

    # TODO(Task 2.3b): pair_pipeline branch. Will be filled in by the
    # next commit. Until then, fall through to QueryEngine when this
    # branch is reached — the test_route_valid_workspace tests expect
    # this branch to be wired by 2.3b.
    raise NotImplementedError(
        "pair_pipeline branch not implemented yet (TODO Task 2.3b)"
    )
```

c. Do NOT touch `_run_and_publish` yet. Task 2.3c will refactor its signature and make it call `_route_and_stream`. In 2.3a, `_route_and_stream` exists as a standalone function but nothing in production calls it — only our new tests do.

- [ ] **Step 4: Run the 2.3a tests to verify they pass**

```bash
pytest tests/test_api_server_route.py::test_route_empty_workspace_uses_queryengine tests/test_api_server_route.py::test_route_nonexistent_workspace_falls_back -v
```

Expected: both tests PASS. The other tests in the file (from Tasks 2.1 and 2.2) still PASS. The `_route_and_stream` function exists, delegates to QueryEngine for empty workspace, logs WARN + falls back when workspace_path is bogus.

- [ ] **Step 5: Commit**

```bash
git add ai-worker/src/api_server.py ai-worker/tests/test_api_server_route.py
git commit -m "feat(ai-worker): _route_and_stream skeleton with legacy-only path

Introduces the _route_and_stream async iterator seam. Today it only
handles the empty-workspace fallback: delegate to the existing
session-cached QueryEngine.submit_message path. pair_pipeline branch
raises NotImplementedError pending Task 2.3b.

Tests cover the two paths that are wired now: empty workspace_path
(QueryEngine path) and nonexistent workspace_path (fallback + WARN
log). The valid-workspace path is left for the next commit.

_run_and_publish is untouched in this commit — Task 2.3c refactors
its signature separately for a clean review diff.

Refs TODOS #5."
```

### Task 2.3b: Fill in the `_route_and_stream` pair_pipeline branch

**Files:**
- Modify: `D:\shulex_work\forge\ai-worker\src\api_server.py` — replace the `NotImplementedError` branch with real pair_pipeline wiring.

- [ ] **Step 1: Write the failing tests**

Append to `D:\shulex_work\forge\ai-worker\tests\test_api_server_route.py`:

```python
def _fake_pair_pipeline_iter_with_non_event():
    """Mimic run_pair_pipeline: yield a StreamEvent, a non-StreamEvent
    (simulating CycleResult), and a final SessionComplete."""
    from dataclasses import dataclass

    @dataclass
    class FakeCycleResult:
        cycle: int = 1

    async def _gen():
        yield AssistantTextDelta(text="hello from pair_pipeline")
        yield FakeCycleResult()
        yield SessionComplete(
            files_created=1,
            files_modified=0,
            build_status="passed",
            duration_ms=100,
            tokens_total=50,
            cost_usd=0.001,
        )
    return _gen()


@pytest.mark.asyncio
async def test_route_valid_workspace_uses_pair_pipeline(tmp_path, monkeypatch):
    """workspace_path is a real directory → pair_pipeline path, 2 engines created."""
    workspace = tmp_path / "repo"
    workspace.mkdir()

    req = RunRequest(
        project_id=1,
        message="add a hello function",
        workspace_path=str(workspace),
    )

    qe_calls = []  # list of Purpose values

    def fake_qe(r, purpose=None):
        qe_calls.append(purpose)
        mock = MagicMock()
        mock.submit_message = MagicMock(return_value=_fake_query_engine_iter())
        return mock

    pipeline_called = {"v": 0}

    def fake_pipeline(config, coder_engine, reviewer_engine, initial_prompt, code_files=None):
        pipeline_called["v"] += 1
        assert str(config.project_dir) == str(workspace), \
            "PairPipelineConfig.project_dir must be the workspace_path"
        return _fake_pair_pipeline_iter_with_non_event()

    monkeypatch.setattr("src.api_server._create_engine", fake_qe)
    monkeypatch.setattr("src.api_server.run_pair_pipeline", fake_pipeline)

    events = []
    async for ev in _route_and_stream(req, "sid-1", "corr-1"):
        events.append(ev)

    assert pipeline_called["v"] == 1, "pair_pipeline must be called"
    assert len(qe_calls) == 2, "Two engines created: coder + reviewer"
    # The non-StreamEvent (FakeCycleResult) must have been filtered out
    assert len(events) == 2, f"Expected 2 filtered events, got {len(events)}: {events}"
    assert all(isinstance(e, StreamEvent) for e in events)


@pytest.mark.asyncio
async def test_route_filters_non_stream_events(tmp_path, monkeypatch):
    """CycleResult / PairPipelineResult yields must not reach the caller."""
    workspace = tmp_path / "repo"
    workspace.mkdir()
    req = RunRequest(project_id=1, message="add code", workspace_path=str(workspace))

    def fake_qe(r, purpose=None):
        mock = MagicMock()
        mock.submit_message = MagicMock(return_value=_fake_query_engine_iter())
        return mock

    def fake_pipeline(*args, **kwargs):
        return _fake_pair_pipeline_iter_with_non_event()

    monkeypatch.setattr("src.api_server._create_engine", fake_qe)
    monkeypatch.setattr("src.api_server.run_pair_pipeline", fake_pipeline)

    events = []
    async for ev in _route_and_stream(req, "sid-1", "corr-1"):
        events.append(ev)

    # Fake pipeline yields 3 items: 1 StreamEvent, 1 non-event, 1 StreamEvent
    # After filtering: 2 StreamEvents
    assert len(events) == 2
    assert all(isinstance(e, StreamEvent) for e in events)
```

- [ ] **Step 2: Run the new tests to verify they fail**

```bash
pytest tests/test_api_server_route.py::test_route_valid_workspace_uses_pair_pipeline tests/test_api_server_route.py::test_route_filters_non_stream_events -v
```

Expected: both FAIL with `NotImplementedError: pair_pipeline branch not implemented yet (TODO Task 2.3b)`.

- [ ] **Step 3: Add the `Path` import + module-level imports for pair_pipeline**

In `D:\shulex_work\forge\ai-worker\src\api_server.py`, at the top:

```python
from pathlib import Path
```

Also, to make `monkeypatch.setattr("src.api_server.run_pair_pipeline", ...)` work in tests, `run_pair_pipeline` must be importable at module level. Add at the top (near the other imports):

```python
from src.openharness.engine.pair_pipeline import (
    PairPipelineConfig,
    run_pair_pipeline,
)
from src.openharness.engine.stream_events import StreamEvent
from src.models.router import Purpose
```

- [ ] **Step 4: Replace the NotImplementedError branch with real pair_pipeline wiring**

In `_route_and_stream`, replace the block:

```python
    # TODO(Task 2.3b): pair_pipeline branch. Will be filled in by the
    # next commit. Until then, fall through to QueryEngine when this
    # branch is reached — the test_route_valid_workspace tests expect
    # this branch to be wired by 2.3b.
    raise NotImplementedError(
        "pair_pipeline branch not implemented yet (TODO Task 2.3b)"
    )
```

with:

```python
    # pair_pipeline path: two engines, coder + reviewer, differentiated
    # by Purpose. PairPipelineConfig.project_dir is the workspace on
    # disk where BuildVerify will run `go build` / `mvn` / etc.
    coder = _create_engine(req, purpose=Purpose.GENERATE)
    reviewer = _create_engine(req, purpose=Purpose.REVIEW)
    config = PairPipelineConfig(project_dir=Path(req.workspace_path))

    async for item in run_pair_pipeline(
        config=config,
        coder_engine=coder,
        reviewer_engine=reviewer,
        initial_prompt=req.message,
        code_files=None,
    ):
        if isinstance(item, StreamEvent):
            yield item
        # Non-StreamEvent yields (CycleResult, PairPipelineResult) are
        # informational for direct callers of run_pair_pipeline (like
        # the e2e test). HTTP callers get the event stream only.
```

- [ ] **Step 5: Run the new tests to verify they pass**

```bash
pytest tests/test_api_server_route.py -v
```

Expected: all routing tests PASS — the two 2.3a tests + the two new 2.3b tests = 4 routing tests green. Also the earlier 2.1 + 2.2 tests still green.

- [ ] **Step 6: Run the full ai-worker unit suite + e2e**

```bash
pytest tests/ -m "not e2e" -q
pytest tests/ -m e2e -v
```

Expected: no regression in either.

- [ ] **Step 7: Commit**

```bash
git add ai-worker/src/api_server.py ai-worker/tests/test_api_server_route.py
git commit -m "feat(ai-worker): _route_and_stream pair_pipeline branch

Fills in the TODO from the previous commit: when workspace_path
resolves to an on-disk directory, _route_and_stream constructs a
coder + reviewer QueryEngine pair via Purpose.GENERATE/REVIEW,
builds a PairPipelineConfig with the workspace as project_dir,
and delegates to run_pair_pipeline. Non-StreamEvent yields
(CycleResult / PairPipelineResult) are filtered via isinstance.

Still not wired through _run_and_publish — the next commit
refactors that signature.

Refs TODOS #5."
```

### Task 2.3c: Refactor `_run_and_publish` to use `_route_and_stream`

**Files:**
- Modify: `D:\shulex_work\forge\ai-worker\src\api_server.py` — `_run_and_publish` signature + `run_agent` caller

- [ ] **Step 1: Write the failing test**

Append to `D:\shulex_work\forge\ai-worker\tests\test_api_server_route.py`:

```python
def _fake_pair_pipeline_that_raises():
    async def _gen():
        yield AssistantTextDelta(text="before crash")
        raise RuntimeError("pipeline exploded")
    return _gen()


@pytest.mark.asyncio
async def test_route_pair_pipeline_exception_propagates(tmp_path, monkeypatch):
    """Exceptions in pair_pipeline propagate from _route_and_stream so
    _run_and_publish can catch them and emit an ErrorEvent to Redis/PG."""
    workspace = tmp_path / "repo"
    workspace.mkdir()
    req = RunRequest(project_id=1, message="add code", workspace_path=str(workspace))

    def fake_qe(r, purpose=None):
        mock = MagicMock()
        mock.submit_message = MagicMock(return_value=_fake_query_engine_iter())
        return mock

    def fake_pipeline(*args, **kwargs):
        return _fake_pair_pipeline_that_raises()

    monkeypatch.setattr("src.api_server._create_engine", fake_qe)
    monkeypatch.setattr("src.api_server.run_pair_pipeline", fake_pipeline)

    events = []
    with pytest.raises(RuntimeError, match="pipeline exploded"):
        async for ev in _route_and_stream(req, "sid-1", "corr-1"):
            events.append(ev)

    # Events yielded BEFORE the exception must have been observable
    assert len(events) == 1
    assert events[0].text == "before crash"
```

- [ ] **Step 2: Run the test to verify it passes immediately**

Wait — this test should already pass because `_route_and_stream` just delegates to whatever `run_pair_pipeline` yields, and exceptions propagate naturally from async generators. Run:

```bash
pytest tests/test_api_server_route.py::test_route_pair_pipeline_exception_propagates -v
```

Expected: PASS. This is a contract-pinning test, not a driver for new code.

- [ ] **Step 3: Refactor `_run_and_publish` signature**

In `D:\shulex_work\forge\ai-worker\src\api_server.py`, replace the current signature:

```python
async def _run_and_publish(
    engine: Any, session_id: str, message: str, correlation_id: str,
) -> None:
```

with:

```python
async def _run_and_publish(
    req: RunRequest, session_id: str, correlation_id: str,
) -> None:
```

Inside the function body:
- Replace references to `message` with `req.message`
- Delete the `engine` parameter usage (the old `async for event in engine.submit_message(message)` line)
- Replace it with `async for event in _route_and_stream(req, session_id, correlation_id)`

The new try block:

```python
    try:
        async for event in _route_and_stream(req, session_id, correlation_id):
            event_data = _serialize_event(event, correlation_id)
            redis_id: Optional[str] = None
            if redis_client:
                try:
                    redis_id = await redis_client.xadd(
                        stream_key,
                        event_data,
                        maxlen=settings.agent_stream_maxlen,
                        approximate=True,
                    )
                except Exception as e:
                    logger.error("Redis XADD failed: %s", e)
            await _persist_message(pg_pool, session_id, redis_id, event_data)
    except Exception as e:
        logger.exception("Agent run failed for session %s", session_id)
        error_data = {
            "type": "error",
            "message": str(e),
            "correlation_id": correlation_id,
        }
        # ... rest unchanged (err_redis_id, _persist_message) ...
```

- [ ] **Step 4: Update `run_agent` to match the new signature**

In `run_agent` (around line 54-79), the current code:

```python
    # Get or create session engine
    engine = _sessions.get(session_id)
    if engine is None:
        engine = _create_engine(req)
        _sessions[session_id] = engine

    # Fire-and-forget: run in background task
    asyncio.create_task(
        _run_and_publish(engine, session_id, req.message, correlation_id),
    )
```

becomes:

```python
    # Engine is created lazily inside _route_and_stream on the legacy
    # path. pair_pipeline creates its own fresh pair per invocation.
    # _sessions caching for the legacy path happens in _route_and_stream.
    asyncio.create_task(
        _run_and_publish(req, session_id, correlation_id),
    )
```

Delete the eager `_sessions.get / _create_engine` block — it's now done inside `_route_and_stream` on demand.

- [ ] **Step 5: Run the full ai-worker unit suite + e2e**

```bash
pytest tests/ -m "not e2e" -q
pytest tests/ -m e2e -v
```

Expected: all green. Key regressions to watch:
- `test_api_server.py` or similar tests that assert `_sessions` population on the eager path — adjust them to assert lazy population via `_route_and_stream` instead.
- e2e test imports pair_pipeline directly, so unaffected.

- [ ] **Step 6: Commit**

```bash
git add ai-worker/src/api_server.py ai-worker/tests/test_api_server_route.py
git commit -m "refactor(ai-worker): _run_and_publish calls _route_and_stream

Drops the eager engine creation in run_agent; _route_and_stream
now handles session engine lookup/creation on the legacy path.
_run_and_publish loses its engine+message positional args and
takes the RunRequest directly.

After this commit the pair_pipeline branch is reachable from the
production HTTP path — it just still does nothing until forge-core
starts populating workspace_path in Phase 3.

Contract test added: exceptions from pair_pipeline propagate through
_route_and_stream so the outer try/except in _run_and_publish can
turn them into ErrorEvent for Redis/PG.

Refs TODOS #5."
```

---

## Phase 3 — forge-core workspace_path injection

**Goal:** Teach forge-core's `agent.Service.SubmitMessage` to compute `workspace_path` using the existing `workspace.Manager` and pass it in `aiRunRequest`. Tenant ID flow is added to the Service signature because `wsManager.ProjectDir` needs it.

### Task 3.1: Extend `agent.Service` with wsManager + tenantID flow

**Files:**
- Modify: `D:\shulex_work\forge\forge-core\internal\module\agent\service.go`

- [ ] **Step 1: Write the failing tests**

Create `D:\shulex_work\forge\forge-core\internal\module\agent\service_test.go`:

```go
package agent

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/shulex/forge/forge-core/internal/workspace"
)

// captureAIWorker returns an httptest server that records the last
// aiRunRequest it received. Useful for asserting what forge-core sent.
func captureAIWorker(t *testing.T) (*httptest.Server, *aiRunRequest) {
	t.Helper()
	captured := &aiRunRequest{}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/run" {
			http.NotFound(w, r)
			return
		}
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, captured)
		resp := aiRunResponse{
			SessionID:     "sid-captured",
			Status:        "accepted",
			CorrelationID: "corr-captured",
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	t.Cleanup(srv.Close)
	return srv, captured
}

// TestSubmitMessage_PassesWorkspacePath_WhenRepoExists asserts that
// when wsManager is non-nil, tenantID is positive, and the resolved
// ProjectDir contains a .git directory, SubmitMessage sends the
// workspace_path to the ai-worker.
func TestSubmitMessage_PassesWorkspacePath_WhenRepoExists(t *testing.T) {
	// Create a fake workspace root with a .git marker
	root := t.TempDir()
	projectDir := filepath.Join(root, "tenant-1", "project-42", "repo")
	if err := os.MkdirAll(filepath.Join(projectDir, ".git"), 0o755); err != nil {
		t.Fatalf("mkdir .git: %v", err)
	}

	wsManager := workspace.NewManager(root)
	srv, captured := captureAIWorker(t)

	svc := NewService(srv.URL, wsManager)
	req := ChatRequest{Message: "hello"}

	resp, err := svc.SubmitMessage(context.Background(), 1, 42, req)
	if err != nil {
		t.Fatalf("SubmitMessage: %v", err)
	}
	if resp == nil {
		t.Fatalf("nil resp")
	}

	want := projectDir
	if captured.WorkspacePath != want {
		t.Errorf("WorkspacePath = %q, want %q", captured.WorkspacePath, want)
	}
}

// TestSubmitMessage_EmptyWorkspacePath_WhenRepoMissing asserts the
// fallback: when the project has not been cloned (no .git), we send
// an empty workspace_path so the ai-worker falls back to QueryEngine.
func TestSubmitMessage_EmptyWorkspacePath_WhenRepoMissing(t *testing.T) {
	root := t.TempDir() // empty — no tenant-1/project-42/repo at all

	wsManager := workspace.NewManager(root)
	srv, captured := captureAIWorker(t)

	svc := NewService(srv.URL, wsManager)
	req := ChatRequest{Message: "hello"}

	_, err := svc.SubmitMessage(context.Background(), 1, 42, req)
	if err != nil {
		t.Fatalf("SubmitMessage: %v", err)
	}

	if captured.WorkspacePath != "" {
		t.Errorf("WorkspacePath = %q, want empty string (project not cloned)", captured.WorkspacePath)
	}
}

// TestSubmitMessage_EmptyWorkspacePath_WhenWsManagerNil asserts that
// nil wsManager is tolerated (legacy dev boot, handler_test fixtures)
// and produces an empty workspace_path.
func TestSubmitMessage_EmptyWorkspacePath_WhenWsManagerNil(t *testing.T) {
	srv, captured := captureAIWorker(t)

	svc := NewService(srv.URL, nil)
	req := ChatRequest{Message: "hello"}

	_, err := svc.SubmitMessage(context.Background(), 1, 42, req)
	if err != nil {
		t.Fatalf("SubmitMessage: %v", err)
	}

	if captured.WorkspacePath != "" {
		t.Errorf("WorkspacePath = %q, want empty (nil wsManager)", captured.WorkspacePath)
	}
}

// TestSubmitMessage_EmptyWorkspacePath_WhenTenantZero asserts that
// when the caller does not know the tenant (legacy Chat fallback
// path in handler.go L151), tenantID=0 produces an empty
// workspace_path — we don't want a bogus "tenant-0" path.
func TestSubmitMessage_EmptyWorkspacePath_WhenTenantZero(t *testing.T) {
	root := t.TempDir()
	// Even if some directory happens to exist at tenant-0/..., we
	// must NOT use it — tenantID=0 is a sentinel meaning "unknown".
	projectDir := filepath.Join(root, "tenant-0", "project-42", "repo")
	_ = os.MkdirAll(filepath.Join(projectDir, ".git"), 0o755)

	wsManager := workspace.NewManager(root)
	srv, captured := captureAIWorker(t)

	svc := NewService(srv.URL, wsManager)
	req := ChatRequest{Message: "hello"}

	_, err := svc.SubmitMessage(context.Background(), 0, 42, req)
	if err != nil {
		t.Fatalf("SubmitMessage: %v", err)
	}

	if captured.WorkspacePath != "" {
		t.Errorf("WorkspacePath = %q, want empty (tenantID=0 sentinel)", captured.WorkspacePath)
	}
}
```

Note: this test file uses `aiRunRequest` and `aiRunResponse` which are package-private types. Because we're in the same package (`agent`), that's fine.

- [ ] **Step 2: Run tests to verify they fail**

```bash
cd D:/shulex_work/forge/forge-core
go test ./internal/module/agent/... -run TestSubmitMessage -v
```

Expected: compile error. `NewService` currently takes 1 arg; tests pass 2. `SubmitMessage` currently takes 3 args; tests pass 4. `aiRunRequest` has no `WorkspacePath` field.

- [ ] **Step 3: Update `aiRunRequest`**

In `D:\shulex_work\forge\forge-core\internal\module\agent\service.go`, update the `aiRunRequest` struct:

```go
type aiRunRequest struct {
	SessionID     string `json:"session_id,omitempty"`
	ProjectID     int64  `json:"project_id"`
	WorkspacePath string `json:"workspace_path,omitempty"`
	Message       string `json:"message"`
	Model         string `json:"model,omitempty"`
	SystemPrompt  string `json:"system_prompt,omitempty"`
	CorrelationID string `json:"correlation_id,omitempty"`
}
```

- [ ] **Step 4: Update `Service` struct + `NewService`**

Replace the `Service` struct and `NewService` function:

```go
// Service handles communication with the Python AI worker.
type Service struct {
	aiWorkerURL string
	httpClient  *http.Client
	wsManager   *workspace.Manager // optional; nil tolerated for dev / tests
}

// NewService creates a new agent service.
// wsManager may be nil — in that case, workspace_path is always empty
// and the ai-worker falls back to the QueryEngine chat path.
func NewService(aiWorkerURL string, wsManager *workspace.Manager) *Service {
	return &Service{
		aiWorkerURL: aiWorkerURL,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
		wsManager: wsManager,
	}
}
```

Add the import for `workspace`:

```go
import (
    // ... existing ...
    "github.com/shulex/forge/forge-core/internal/workspace"
)
```

- [ ] **Step 5: Update `SubmitMessage` signature + body**

Replace the `SubmitMessage` method:

```go
// SubmitMessage sends a message to the AI worker (fire-and-forget).
// The AI worker runs either pair_pipeline (if workspace_path resolves
// to an on-disk repo) or QueryEngine (fallback) asynchronously and
// publishes events to Redis.
//
// tenantID may be 0 when the caller (legacy Chat path without
// dual-storage) does not know the tenant. In that case workspace_path
// is left empty regardless of wsManager state — we do NOT synthesize
// a tenant-0 directory lookup.
func (s *Service) SubmitMessage(ctx context.Context, tenantID, projectID int64, req ChatRequest) (*ChatResponse, error) {
	body := aiRunRequest{
		SessionID:     req.SessionID,
		ProjectID:     projectID,
		Message:       req.Message,
		Model:         req.Model,
		SystemPrompt:  req.SystemPrompt,
		CorrelationID: req.CorrelationID,
	}

	// Populate workspace_path when we have a non-nil wsManager, a real
	// tenant, and a cloned repo on disk. Anything else leaves
	// workspace_path empty → ai-worker QueryEngine fallback.
	if s.wsManager != nil && tenantID > 0 {
		candidate := s.wsManager.ProjectDir(tenantID, projectID)
		gitDir := filepath.Join(candidate, ".git")
		if _, err := os.Stat(gitDir); err == nil {
			body.WorkspacePath = candidate
		} else if !os.IsNotExist(err) {
			slog.Warn("agent service: unexpected stat error on .git dir, treating as missing",
				"tenant_id", tenantID,
				"project_id", projectID,
				"path", gitDir,
				"error", err,
			)
		}
	}

	jsonBody, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	url := fmt.Sprintf("%s/api/run", s.aiWorkerURL)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := s.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("call ai-worker: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("ai-worker returned %d: %s", resp.StatusCode, string(respBody))
	}

	var aiResp aiRunResponse
	if err := json.NewDecoder(resp.Body).Decode(&aiResp); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	slog.Info("agent message submitted",
		"session_id", aiResp.SessionID,
		"correlation_id", aiResp.CorrelationID,
		"project_id", projectID,
		"workspace_path", body.WorkspacePath,
	)

	return &ChatResponse{
		SessionID:     aiResp.SessionID,
		Status:        aiResp.Status,
		CorrelationID: aiResp.CorrelationID,
	}, nil
}
```

Add missing imports:

```go
import (
    // ... existing ...
    "os"
    "path/filepath"
)
```

- [ ] **Step 6: Run the new service tests to verify they pass**

```bash
go test ./internal/module/agent/... -run TestSubmitMessage -v
```

Expected: 4 new tests all PASS.

- [ ] **Step 7: Update `handler.go` to pass tenantID**

Two call sites. The first is the legacy path (L150-158) — the caller does not have tenantID, so pass `0`:

```go
	if h.chat == nil {
		resp, err := h.service.SubmitMessage(c.Request.Context(), 0, projectID, req)
		// ... unchanged ...
	}
```

The second is the dual-storage path (L223) — tenantID is available from `currentUser(c)` above (already captured as `tenantID` variable). Update:

```go
	resp, err := h.service.SubmitMessage(ctx, tenantID, projectID, req)
```

- [ ] **Step 8: Update all existing handler tests to match the new signatures**

Any test in `handler_test.go` that constructs `agent.NewService(...)` needs `nil` as the second argument. Run this to find them:

```bash
grep -n "agent.NewService\|NewService(" internal/module/agent/handler_test.go
```

For each hit, add `nil` as the second argument. Example:

Before:
```go
svc := NewService(srv.URL)
```

After:
```go
svc := NewService(srv.URL, nil)
```

Any test call to `svc.SubmitMessage(ctx, projectID, req)` needs to become `svc.SubmitMessage(ctx, 0, projectID, req)` (tenantID=0 for legacy-path tests).

```bash
grep -n "SubmitMessage(" internal/module/agent/handler_test.go
```

- [ ] **Step 9: Run the full agent package test suite**

```bash
go test ./internal/module/agent/... -v
```

Expected: all tests green, including the G1-G4 tests from the previous PR.

- [ ] **Step 10: Commit**

```bash
git add forge-core/internal/module/agent/service.go \
        forge-core/internal/module/agent/service_test.go \
        forge-core/internal/module/agent/handler.go \
        forge-core/internal/module/agent/handler_test.go
git commit -m "feat(agent): compute workspace_path for pair_pipeline routing

agent.Service now accepts a *workspace.Manager (nilable) and computes
workspace_path by calling wsManager.ProjectDir(tenantID, projectID)
and verifying the .git directory exists on disk. Empty string when
wsManager is nil, tenantID is 0, or the repo is not cloned — which
the ai-worker treats as 'use the legacy QueryEngine path'.

SubmitMessage signature gains tenantID. handler.go legacy path passes
0 (unknown tenant); dual-storage path passes the authenticated
tenantID from currentUser(c).

Refs TODOS #5."
```

### Task 3.2: Wire wsManager into agent.NewService in main.go

**Files:**
- Modify: `D:\shulex_work\forge\forge-core\cmd\forge-core\main.go` — L245

- [ ] **Step 1: Write a quick smoke test (compile + run)**

There is no dedicated main_test for this — the compile gate catches the change. Instead:

```bash
cd D:/shulex_work/forge/forge-core
go build ./cmd/forge-core
```

Expected: compile FAILS with "not enough arguments in call to agent.NewService".

- [ ] **Step 2: Update the call site**

In `D:\shulex_work\forge\forge-core\cmd\forge-core\main.go` around line 245, change:

```go
	agentSvc := agent.NewService(cfg.AIWorkerURL)
```

to:

```go
	agentSvc := agent.NewService(cfg.AIWorkerURL, workspaceMgr)
```

Note: `workspaceMgr` is already constructed at L122 in this file. No new allocation.

- [ ] **Step 3: Run go build to verify it compiles**

```bash
go build ./cmd/forge-core
```

Expected: success, `forge-core.exe` or `forge-core` binary produced at the repo root (and ignored by .gitignore from Phase 0's TODOS #4).

- [ ] **Step 4: Run the full forge-core test suite**

```bash
go test ./...
```

Expected: all green. This catches any test outside the agent package that accidentally constructs `agent.NewService(...)` (shouldn't exist, but the compile gate ensures it).

- [ ] **Step 5: Commit**

```bash
git add forge-core/cmd/forge-core/main.go
git commit -m "feat(agent): wire workspaceMgr into agent.NewService at bootstrap

workspaceMgr is already constructed for the project and temporal
activity modules at L122. Reusing the same instance here means the
dev env automatically picks up FORGE_WORKSPACE_ROOT for agent chat
routing as well.

Completes the forge-core side of TODOS #5."
```

---

## Phase 4 — Integration GATE (manual end-to-end)

**Goal:** The PR is not merge-ready until a real HTTP chat request on a real cloned project flows through the full pair_pipeline loop and reaches `session_complete` with `build_status="passed"`. This is the success criterion from the design spec.

This is a manual gate performed by Harvey — CC assists but does not own the terminal that runs forge-core on the host.

### Task 4.1: Restart forge-core with new binary

- [ ] **Step 1: Ask Harvey to rebuild and restart forge-core**

```bash
cd D:/shulex_work/forge
go build -o forge-core/forge-core-new.exe ./forge-core/cmd/forge-core
# Stop old forge-core
powershell "Stop-Process -Id <OLD_PID>"
# Start new forge-core
./forge-core/forge-core-new.exe
```

Confirm the server is up:
```bash
curl -s http://localhost:8080/health
```

Expected: 200 OK.

- [ ] **Step 2: Confirm forge-ai-worker still sees the mounted workspace**

```bash
docker exec forge-ai-worker ls /data/forge/workspaces/tenant-1/project-999/repo/go.mod
```

Expected: file listed (still the seed from Phase 0).

### Task 4.2: Create a project-999 DB row if it does not exist

The forge-core project module requires a row in `engine.projects` for project_id=999. If it's not there, `handler.Chat` will return 404 before even reaching the ai-worker.

- [ ] **Step 1: Check if project 999 exists in PG**

```bash
docker exec forge-postgres psql -U forge -d forge -c "SELECT id, tenant_id FROM engine.projects WHERE id=999;"
```

If the row exists and `tenant_id=1`, skip to Task 4.3.

- [ ] **Step 2: If the row is missing, insert a minimal test project**

```bash
docker exec forge-postgres psql -U forge -d forge -c "
INSERT INTO engine.projects (id, tenant_id, name, repo_url, default_branch, created_by, created_at, updated_at)
VALUES (999, 1, 'pair-pipeline-e2e-seed', 'file://D:/forge-workspaces/tenant-1/project-999/repo', 'main', 1, NOW(), NOW())
ON CONFLICT (id) DO NOTHING;
"
```

Note: if there are NOT-NULL columns the schema requires beyond these, add them. The goal is just to make project_id=999 a valid target for the agent handler.

### Task 4.3: Send a real chat message and observe the SSE stream

- [ ] **Step 1: Obtain an admin JWT**

```bash
curl -s -X POST http://localhost:8080/api/auth/login \
     -H "Content-Type: application/json" \
     -d '{"username":"admin","password":"admin123"}'
```

Expected: JSON with a `token` field. Save it as `$TOKEN`.

- [ ] **Step 2: Open the SSE stream in one terminal window**

```bash
curl -N -H "Authorization: Bearer $TOKEN" \
     "http://localhost:8080/api/projects/999/agent/stream?session_id=e2e-pair-$(date +%s)"
```

Leave this running.

- [ ] **Step 3: In another terminal, send the chat message**

```bash
SID="e2e-pair-$(date +%s)"
curl -X POST -H "Authorization: Bearer $TOKEN" \
     -H "Content-Type: application/json" \
     "http://localhost:8080/api/projects/999/agent/chat" \
     -d "{\"session_id\":\"$SID\",\"message\":\"add a Hello(name string) string function to main.go that returns 'Hello, '+name\"}"
```

Expected: 202 Accepted response with the session_id echoed.

- [ ] **Step 4: GATE — watch the SSE stream for the pair_pipeline event sequence**

The terminal from Step 2 should produce events in this approximate order:

1. `user_message` (echo of the message)
2. `thinking_started` with `label="Generating code"`
3. `text_delta` events (streaming coder output)
4. `turn_complete`
5. `thinking_stopped`
6. `thinking_started` with `label="Running go build ./..."` (iff Phase 0 Outcome A — `go` is in the ai-worker image)
7. `thinking_stopped`
8. `thinking_started` with `label="Reviewing code"`
9. `text_delta` events (streaming reviewer output)
10. `turn_complete`
11. `thinking_stopped`
12. `session_complete`

**The success criterion depends on which Phase 0 outcome you recorded:**

| Phase 0 outcome | Phase 4 success criterion |
|-----------------|---------------------------|
| **A — `go` is in the ai-worker image** | `session_complete` arrives AND `build_status="passed"` (the full fix loop ran and converged, OR coder produced compilable code on the first pass). `build_status="failed"` after 3 cycles is NOT a pass — it means the LLM produced bad code and the pipeline gave up; debug with a simpler prompt like "add a `// hello` comment to main.go". |
| **B — `go` is NOT in the ai-worker image** | `session_complete` arrives AND `build_status` is either `"skipped"` (BuildVerify short-circuited because language detection couldn't locate a usable toolchain) OR `"failed"` with build output containing `"go: not found"` (pipeline tried to run `go build`, failed immediately). In either case the pipeline plumbing ran end-to-end; the build validation gap is a separate TODO #1 concern. |

**If `session_complete` never arrives at all, the PR is not ready.** Debug and retry.

Common failure modes and fixes:

| Symptom | Cause | Fix |
|---------|-------|-----|
| 404 from chat endpoint | project_id=999 not in DB | Run Task 4.2 Step 2 |
| Stream shows only QueryEngine-style events (no FixLoop / SessionComplete) | workspace_path empty in aiRunRequest | Check `slog.Info` log line in forge-core for `workspace_path` field; if empty, debug `wsManager.ProjectDir` return value and `os.Stat` result |
| pair_pipeline starts but BuildVerify is skipped | Language detection didn't match | Check `detect_language` log output; ensure `go.mod` is present inside the container's view of the path |
| `session_complete` arrives but `build_status="failed"` after 3 fix cycles (Outcome A) | LLM produced bad code | Retry with a simpler prompt. If it still fails, inspect the `text_delta` events for what the coder was writing — it may be a prompt-tuning issue |
| Container cannot run `go build` (Outcome B) | ai-worker image has no go toolchain | Expected in Outcome B; accept the degraded criterion. Installing go in the ai-worker image is TODOS #1 territory |

- [ ] **Step 5: Verify PG durable history**

```bash
docker exec forge-postgres psql -U forge -d forge -c "
SELECT event_type, role, LEFT(COALESCE(content, ''), 60) AS preview
FROM engine.agent_messages
WHERE session_id = '$SID'
ORDER BY id;"
```

Expected: a row count >= the number of events observed in the SSE stream (allow for a small Redis-vs-PG race). The sequence should start with `user_message` and include at least `thinking_started`, `text_delta`, `turn_complete`, `session_complete` events.

### Task 4.4: Save a success checkpoint

- [ ] **Step 1: Run `/checkpoint save`**

```bash
# In Claude Code:
/checkpoint save pair-pipeline-production-wired
```

This captures the working state with the successful e2e verification.

---

## Phase 5 — Documentation

**Goal:** Update TODOS.md, technical-design.md, and (optionally) the README to reflect the new capability.

### Task 5.1: Update TODOS.md

**Files:**
- Modify: `D:\shulex_work\forge\docs\TODOS.md`

- [ ] **Step 1: Read current state of TODOS.md**

```bash
grep -n "pair_pipeline\|#5\|Workspace sandbox" docs/TODOS.md
```

- [ ] **Step 2: Mark #5 as DONE and add the 3 new P2 items**

Edit `docs/TODOS.md`:

a. Find the #5 "Wire api_server.py /api/run to pair_pipeline" entry (or equivalent) and change its status to DONE with a date and commit reference.

b. Add a new P2 section (or append to existing) with these three items:

```markdown
### P2 — ai-worker followups from pair_pipeline production wire-up (2026-04-09)

- **Session concurrency lock in api_server.py `_sessions` dict.** Two concurrent `/api/run` calls with the same session_id both mutate the same cached QueryEngine. Pre-existing bug, not introduced by the pair_pipeline wire-up, but now more visible because pair_pipeline holds state longer. Fix: per-session asyncio.Lock or reject the second request with 429.

- **Mid-stream crash recovery.** If forge-ai-worker restarts while a pair_pipeline is running, partial events land in Redis + PG without a trailing `session_complete` event. Frontend SSE stream appears to hang. Requires wrapping pair_pipeline in a Temporal workflow (or equivalent durable orchestration). Out of scope for this PR, but important before marketplace or multi-tenant SaaS.

- **ModelRouter Purpose differentiation.** `Purpose.GENERATE` and `Purpose.REVIEW` currently resolve to the same underlying model in most ModelRouter configurations. For the Harness "reviewer as a different model" contract to have teeth, the router should assign a smaller/cheaper model to REVIEW and a stronger one to GENERATE. Fix is a config file change plus an e2e test asserting the two engines end up with different models.
```

### Task 5.2: Update technical-design.md §3.7

**Files:**
- Modify: `D:\shulex_work\forge\docs\technical-design.md`

- [ ] **Step 1: Find §3.7 "Known Leftovers"**

```bash
grep -n "Known Leftovers\|§3.7\|3\.7" docs/technical-design.md
```

- [ ] **Step 2: Update the pair_pipeline entry**

Find the existing text describing how `api_server.py /api/run` uses single-shot QueryEngine instead of pair_pipeline. Replace it with:

```markdown
**pair_pipeline production wire-up — RESOLVED 2026-04-09** (see `docs/plans/2026-04-09-pair-pipeline-production-wire.md`)

`api_server._route_and_stream` now dispatches chat messages to either
`pair_pipeline.run_pair_pipeline` (when forge-core passes a non-empty
`workspace_path` that exists on disk) or the legacy single-shot
`QueryEngine.submit_message` path. Routing is structural: no LLM
classifier, no frontend toggle. Chat-like interactions (projects
without a cloned workspace) continue to get the cheap single-shot
path; code-work interactions (projects with a cloned repo) get the
full coder → BuildVerify → reviewer → fix loop.
```

### Task 5.3: Final commit

- [ ] **Step 1: Commit the documentation updates**

```bash
git add docs/TODOS.md docs/technical-design.md
git commit -m "docs: TODOS #5 done + pair_pipeline production wire-up followups

Marks TODOS P1 #5 (pair_pipeline production wire-up) as DONE and
adds three new P2 followups discovered during implementation:
session concurrency lock, mid-stream crash recovery, ModelRouter
Purpose differentiation.

Updates technical-design.md §3.7 to reflect the new routing
behavior: structural (workspace_path existence) dispatch between
pair_pipeline and QueryEngine paths.

Refs docs/plans/2026-04-09-pair-pipeline-production-wire.md"
```

---

## Phase 6 — Push and checkpoint (optional, depends on Harvey's preference)

- [ ] **Step 1: Run final test suite sweep**

```bash
cd D:/shulex_work/forge
go test ./... 2>&1 | tail -20
cd ai-worker && pytest tests/ -m "not e2e" -q && pytest tests/ -m e2e -v
```

Expected: all green.

- [ ] **Step 2: Review commit log**

```bash
git log --oneline origin/main..HEAD
```

Expected sequence:
1. `refactor(ai-worker): StreamEvent marker base class for type narrowing` (Phase 1)
2. `feat(ai-worker): add workspace_path to RunRequest` (Task 2.1)
3. `feat(ai-worker): _create_engine supports Purpose parameter` (Task 2.2)
4. `feat(ai-worker): _route_and_stream skeleton with legacy-only path` (Task 2.3a)
5. `feat(ai-worker): _route_and_stream pair_pipeline branch` (Task 2.3b)
6. `refactor(ai-worker): _run_and_publish calls _route_and_stream` (Task 2.3c)
7. `feat(agent): compute workspace_path for pair_pipeline routing` (Task 3.1)
8. `feat(agent): wire workspaceMgr into agent.NewService at bootstrap` (Task 3.2)
9. `chore(docker): mount workspace volume into forge-ai-worker` (Phase 0 Task 0.2 — commit when Phase 0 completes, not after Phase 5)
10. `docs: TODOS #5 done + pair_pipeline production wire-up followups` (Task 5.3)

Ten commits. Phase 0's docker-compose change lands immediately after Phase 0 passes; it is NOT deferred to the end. This ensures the volume mount is reviewable on its own and the Phase 1-2 test work lands on top of a working mount. Commit order is: 9 → 1 → 2 → 3 → 4 → 5 → 6 → 7 → 8 → 10.

**Alternative ordering (fewer commits, denser review diff):** If Harvey prefers, Tasks 2.1 / 2.2 can be folded into Task 2.3a (commit 4 absorbs commits 2-3), reducing total to 8 commits. The current ordering is TDD-pure (each task lands a failing test first, then code), but the fold-in is reasonable because 2.1-2.3a are all incremental changes to the same function. Decide before pushing.

- [ ] **Step 3: Ask Harvey if he wants to push**

Do not push automatically. Show the commit sequence and ask explicitly.

- [ ] **Step 4: If approved, push**

```bash
git push origin main
```

---

## Success Criteria (copy from spec for clarity)

All of the following must be true before the PR is considered merged:

1. ✅ All TDD tests in Phases 1, 2, 3 are green
2. ✅ `go test ./...` in forge-core is green
3. ✅ `pytest tests/ -m "not e2e"` in ai-worker is green (no regression)
4. ✅ `pytest tests/ -m e2e` in ai-worker is green (pair_pipeline direct-import path still works)
5. ✅ Phase 4 manual e2e:
   - Real HTTP chat request reaches `session_complete` event
   - `build_status="passed"` if Phase 0 recorded Outcome A (go toolchain present), OR
   - `build_status` is `"skipped"`/`"failed with go: not found"` if Phase 0 recorded Outcome B (go toolchain absent — the plumbing worked but BuildVerify could not run; installing go is TODOS #1)
6. ✅ Phase 4 PG verification: `engine.agent_messages` for the test session has a full event trail matching the SSE stream
7. ✅ No changes to frontend source code required
8. ✅ Documentation updated: TODOS.md, technical-design.md §3.7
9. ✅ `.gitignore` still correctly ignores the forge-core build artifacts (the TODOS #4 .gitignore change landed 2026-04-08 in this same session — verify with `git check-ignore -v forge-core/forge-core-new.exe` returning a match from `forge-core/.gitignore:6`)

---

## Open questions (for the reviewer or implementer to flag)

- **Should tenantID=0 be rejected as an error instead of being a sentinel?** Current design: tenantID=0 → empty workspace_path → QueryEngine fallback (legacy behavior). Alternative: reject at service layer with an explicit "missing tenant" error. The sentinel approach is less invasive for the legacy path and was chosen for that reason, but if the reviewer prefers strict validation, changing it is localized to `service.go::SubmitMessage`.

- **Should `_route_and_stream` clear `_sessions[session_id]` when routing to pair_pipeline?** Current design: pair_pipeline does not use the session cache; it creates fresh coder+reviewer engines per invocation. This means if the same session alternates between chat (QueryEngine, cached) and code work (pair_pipeline, fresh), the cached QueryEngine persists across both. Is that a leak? Probably fine — it's at worst ~100KB of idle state per session.

- **Should the docker-compose volume be committed as a breaking change or kept opt-in?** Current design uses `${FORGE_WORKSPACE_ROOT_HOST:-./workspaces}`, which defaults to `./workspaces` relative to the compose file. If Harvey's setup uses `D:/forge-workspaces`, he sets the env var locally. This is non-breaking for other contributors who run forge-core out-of-compose. If in the future forge-core moves into compose (TODOS Dev Experience item), the mount becomes mandatory.

---

## Files touched summary (for reviewer)

### Created (3)
- `ai-worker/tests/openharness/test_stream_events_base.py` (~30 lines)
- `ai-worker/tests/test_api_server_route.py` (~300 lines including all 10 test cases across Tasks 2.1, 2.2, 2.3a, 2.3b, 2.3c)
- `forge-core/internal/module/agent/service_test.go` (~150 lines including 4 test cases)

### Modified (9)
- `ai-worker/src/openharness/engine/stream_events.py` (+3 lines, no semantic change)
- `ai-worker/src/api_server.py` (+60 / -20 lines: RunRequest field, _create_engine purpose param, _route_and_stream extraction, _run_and_publish signature)
- `forge-core/internal/module/agent/service.go` (+40 / -5 lines: wsManager field, tenantID param, WorkspacePath computation)
- `forge-core/internal/module/agent/handler.go` (+2 / -2 lines: SubmitMessage call sites)
- `forge-core/internal/module/agent/handler_test.go` (+~5 lines: nil wsManager in fixtures, tenantID=0 in test calls)
- `forge-core/cmd/forge-core/main.go` (+0 / -0 lines: single arg added to NewService call)
- `docker-compose.dev.yml` (+4 lines: volumes + environment)
- `ai-worker/.env` (+1 line: FORGE_WORKSPACE_ROOT)
- `docs/TODOS.md` (+15 lines: DONE marker + 3 new P2 entries)
- `docs/technical-design.md` (+8 lines: §3.7 update)

**Total: ~300 net new LoC, 10 commits (or 8 with the fold-in ordering), ~2h CC time, ~1-2d human time.**
