# Design — pair_pipeline Production Wire-up (TODOS #5)

> **Type:** Design spec (brainstorming output, pre-plan)
> **Date:** 2026-04-08
> **Branch:** main
> **Status:** DESIGN READY — awaiting user review before writing execution plan
> **Related:** `docs/TODOS.md` P1 #5, `docs/technical-design.md §3.7 Known Leftovers`, `docs/plans/2026-04-07-agent-base-loop-reduction.md` (the PR that shipped pair_pipeline)

## Context

The agent base loop — `LLM → coder → BuildVerify → reviewer → fix loop` — was shipped end-to-end on 2026-04-08 in `docs/plans/2026-04-07-agent-base-loop-reduction.md`. It runs against real DASHSCOPE and passes `pytest -m e2e` in 4.13s.

**Gap:** The production HTTP path (`/api/projects/:id/agent/chat` → forge-core → ai-worker `POST /api/run`) still calls the bare `QueryEngine.submit_message(message)` at `ai-worker/src/api_server.py:182`. `pair_pipeline.run_pair_pipeline` has ZERO production callers — only the e2e test imports it directly. Users typing in the Agent Terminal in the browser get single-shot LLM with no fix loop, no build verification, and no reviewer.

This spec covers wiring `pair_pipeline` into `/api/run` so that chat requests on projects with a cloned workspace run through the full base loop.

## Goals

1. When a chat message lands on a project whose workspace directory exists on disk, run it through `pair_pipeline` (coder → BuildVerify → reviewer → fix loop).
2. When the workspace is absent (project not yet cloned, ad-hoc session, or the chat is not code-related), fall back to the existing `QueryEngine.submit_message` path — no regression for chat-like interactions.
3. No breaking changes to the SSE event protocol, Redis stream schema, PG `agent_messages` table, or frontend hydration logic.
4. Zero impact on the existing `pytest -m e2e` test (which imports `pair_pipeline` directly) and on the existing G1-G4 handler tests in forge-core.

## Non-goals

- **Not** fixing the known concurrent-session-write bug (`_sessions` dict has no lock — same session_id can receive two parallel `/api/run` calls, both mutate the same `QueryEngine`). Logged as a follow-up P2.
- **Not** fixing the mid-stream crash recovery gap (ai-worker container restart mid-pair_pipeline loses events). Requires Temporal workflow wrapping, out of scope.
- **Not** implementing LLM-based intent classification (routing decision is purely structural: does workspace_path exist on disk?).
- **Not** pre-loading `code_files` from the workspace into the initial pipeline context. The coder QueryEngine has `Read`/`Glob`/`Grep` tools and is responsible for fetching context it needs. First-version simplicity.
- **Not** refactoring `_create_engine` to use the `ModelRouter` multi-purpose routing (Purpose.GENERATE vs Purpose.REVIEW gets the same model today). Just passes the `purpose` parameter through; model selection strategy is a follow-up.

## Current state (verified by reading source 2026-04-08)

### Production call path

```
Browser
  │ POST /api/projects/:id/agent/chat {message}
  ▼
forge-core/internal/module/agent/handler.go::Chat
  ├─ ① authorize (tenant ownership, G2 contract — pinned by tests)
  ├─ ② persist user_message to engine.agent_messages (G1 contract)
  └─ ③ service.SubmitMessage(ctx, projectID, req)
        │
        ▼
forge-core/internal/module/agent/service.go::SubmitMessage:49-98
  ├─ Construct aiRunRequest {session_id, project_id, message, model, system_prompt, correlation_id}
  └─ HTTP POST {aiWorkerURL}/api/run (10s timeout, fire-and-forget)
        │
        ▼
ai-worker/src/api_server.py::run_agent:54-79
  ├─ Get or create QueryEngine via _sessions dict
  └─ asyncio.create_task(_run_and_publish(engine, session_id, message, correlation_id))
        │
        ▼ (background)
ai-worker/src/api_server.py::_run_and_publish:143-214
  ├─ Write user_message to Redis XADD + PG insert
  └─ async for event in engine.submit_message(message):   ◄── THE LINE
        Redis XADD(stream_key, event_data)
        PG insert into engine.agent_messages
```

The `◄── THE LINE` (at `api_server.py:182`) is where the work happens. Swapping it for `pair_pipeline` is the main change.

### What's already compatible

The `StreamEvent` protocol is already designed for pair_pipeline:
- `FixLoopStarted` / `FixLoopCompleted` events exist in `stream_events.py`
- `SessionComplete` event exists with `files_created`, `files_modified`, `build_status`, `duration_ms`, `tokens_total`, `cost_usd`
- `_serialize_event` at `api_server.py:276-340` already handles all of them and maps them to Redis stream fields
- The frontend consumes these events but never receives them today because `QueryEngine.submit_message` doesn't emit them

**This is the key insight:** the protocol, the Redis path, the PG path, and the frontend are all already wired for `pair_pipeline`. Only the "who produces the events" seam is wrong.

### pair_pipeline.run_pair_pipeline signature

```python
async def run_pair_pipeline(
    config: PairPipelineConfig,      # project_dir, build_command, build_timeout, max_cycles, ...
    coder_engine: Any,               # QueryEngine instance
    reviewer_engine: Any,            # QueryEngine instance
    initial_prompt: str,
    code_files: Optional[Dict[str, str]] = None,
) -> AsyncIterator[Any]:             # yields StreamEvents AND CycleResult/PairPipelineResult
```

It's an `AsyncIterator`, which is a drop-in replacement for `engine.submit_message(message)`. The only wrinkle: it yields TWO kinds of things — `StreamEvent` subclasses (which `_serialize_event` handles) AND `CycleResult` / `PairPipelineResult` dataclasses (which `_serialize_event` falls through to `type: "unknown"`).

Current `stream_events.py` has 10 independent dataclasses with no common base class — adding one is part of this spec.

### Workspace filesystem

`forge-core/internal/workspace/manager.go` is already a production component:
- `ProjectDir(tenantID, projectID)` → `/data/forge/workspaces/tenant-{t}/project-{p}/repo`
- `EnsureClone` / `CreateWorktree` / file ops
- Currently used by `internal/temporal/activity/build_activities.go` (Temporal task workflow)
- `agent.Handler` / `agent.Service` does NOT currently depend on `workspace.Manager` — needs to be injected in `cmd/forge-core/main.go`

The ai-worker is a separate container; it does not share the forge-core host's filesystem by default. A docker-compose volume mount is required so both sides see `/data/forge/workspaces`.

## Approach

### Key decisions (all confirmed in brainstorm)

| # | Decision | Rationale |
|---|----------|-----------|
| 1 | **Routing rule:** `workspace_path` present and on-disk → `pair_pipeline`, otherwise `QueryEngine`. No LLM classifier, no frontend toggle, no unconditional pair_pipeline. | Structural, zero-latency router; chat-only interactions stay cheap. |
| 2 | **Filesystem bridge:** docker-compose volume mount; both `forge-core` and `forge-ai-worker` see `/data/forge/workspaces`. `FORGE_WORKSPACE_ROOT` env variable aligns them. | Closest to production SaaS architecture; reuses existing `workspace.Manager` layout. |
| 3 | **Non-StreamEvent yields:** filter in `_run_and_publish` via `isinstance(event, StreamEvent)`. `CycleResult` / `PairPipelineResult` are logged, not emitted. | Simplest; pair_pipeline already emits `SessionComplete` for summary so we don't lose UX value. |
| 4 | **Initial `code_files`:** empty dict (`{}`); coder QueryEngine uses its Read/Glob/Grep tools to fetch context on demand. | Keeps `aiRunRequest` flat; matches e2e test behavior; no forge-core-side file parsing. |
| 5 | **Where workspace_path is computed:** forge-core (which already has `workspace.Manager`). ai-worker is stateless and has no DB access. | Clean layering; ai-worker doesn't need to read forge-core's projects table. |

### Data flow (target state)

```
Browser
  │ POST /projects/:id/agent/chat {message}
  ▼
forge-core/handler.go::Chat (unchanged for auth, G1, G2)
  └─ ③ service.SubmitMessage(ctx, projectID, req)
        │
        ▼
forge-core/service.go::SubmitMessage (MODIFIED)
  ├─ ③.1 [NEW] workspace_path := ""
  ├─ ③.2 [NEW] if s.wsManager != nil:
  │         candidate := wsManager.ProjectDir(tenantID, projectID)
  │         if _, err := os.Stat(filepath.Join(candidate, ".git")); err == nil {
  │             workspace_path = candidate
  │         }
  ├─ ③.3 [NEW] aiRunRequest.WorkspacePath = workspace_path
  └─ ③.4 HTTP POST /api/run (unchanged protocol)
        │
        ▼
ai-worker/api_server.py::_run_and_publish (MODIFIED)
  ├─ ⑤.1 Write user_message to Redis + PG (unchanged)
  └─ ⑤.2 [NEW] async for event in _route_and_stream(req, ...):
          Redis XADD(stream_key, event_data)
          PG insert into agent_messages

ai-worker/api_server.py::_route_and_stream (NEW)
  ├─ if req.workspace_path and os.path.isdir(req.workspace_path):
  │     coder    = _create_engine(req, purpose=Purpose.GENERATE)
  │     reviewer = _create_engine(req, purpose=Purpose.REVIEW)
  │     config   = PairPipelineConfig(project_dir=Path(req.workspace_path))
  │     async for event in run_pair_pipeline(config, coder, reviewer, req.message):
  │         if isinstance(event, StreamEvent):
  │             yield event
  └─ else:
        async for event in _sessions[session_id].submit_message(req.message):
            yield event
```

## Contract changes

### HTTP contract: `aiRunRequest` / `RunRequest`

**Go (`forge-core/internal/module/agent/service.go`):**
```go
type aiRunRequest struct {
    SessionID     string `json:"session_id,omitempty"`
    ProjectID     int64  `json:"project_id"`
    WorkspacePath string `json:"workspace_path,omitempty"`  // NEW, nullable
    Message       string `json:"message"`
    Model         string `json:"model,omitempty"`
    SystemPrompt  string `json:"system_prompt,omitempty"`
    CorrelationID string `json:"correlation_id,omitempty"`
}
```

**Python (`ai-worker/src/api_server.py`):**
```python
class RunRequest(BaseModel):
    session_id: Optional[str] = None
    project_id: int
    workspace_path: Optional[str] = None  # NEW, nullable
    message: str
    model: Optional[str] = None
    system_prompt: Optional[str] = None
    correlation_id: Optional[str] = None
```

Backwards compatible: nullable field, empty/missing → falls back to QueryEngine path.

### forge-core DI: `workspace.Manager` → `agent.Service`

**Go (`forge-core/internal/module/agent/service.go`):**
```go
type Service struct {
    aiWorkerURL string
    httpClient  *http.Client
    wsManager   *workspace.Manager  // NEW, may be nil
}

func NewService(aiWorkerURL string, wsManager *workspace.Manager) *Service {
    return &Service{
        aiWorkerURL: aiWorkerURL,
        httpClient:  &http.Client{Timeout: 10 * time.Second},
        wsManager:   wsManager,
    }
}
```

**Go (`cmd/forge-core/main.go`):** pass existing `wsManager` instance to `agent.NewService` as the second argument. The `wsManager` is already constructed for `build_activities.go`.

Backwards compatibility: `wsManager == nil` is tolerated; `SubmitMessage` checks for nil and falls through to `workspace_path = ""`. This means existing `agent.NewService` callers in handler_test.go can pass `nil` and their tests continue to work.

### ai-worker: `_create_engine` gains `purpose` parameter

**Python (`ai-worker/src/api_server.py`):**
```python
from src.models.router import Purpose

def _create_engine(req: RunRequest, purpose: Purpose = Purpose.GENERATE) -> Any:
    ...
    api_client = ModelRouterAdapter(router, purpose=purpose)

    if purpose == Purpose.REVIEW:
        default_prompt = (
            "You are a strict code reviewer. Respond with exactly one of: "
            "APPROVE / REVISE <changes> / REJECT <reason>."
        )
    else:
        default_prompt = "You are a helpful AI coding assistant."
    system_prompt = req.system_prompt or default_prompt
    ...
```

Backwards compatible: `purpose=Purpose.GENERATE` default preserves existing behavior for all callers that don't pass it.

### StreamEvent base class

**Python (`ai-worker/src/openharness/engine/stream_events.py`):**
```python
class StreamEvent:
    """Marker base class for all stream events emitted by pipeline iterators."""
    pass

@dataclass
class AssistantTextDelta(StreamEvent):
    ...

# ... all 10 existing event dataclasses inherit from StreamEvent
```

Zero runtime impact (no methods). Enables `isinstance(event, StreamEvent)` type narrowing in `_run_and_publish` and `_route_and_stream`.

### Unchanged contracts (explicit)

- `StreamEvent` payload shapes — no field renames or additions
- Redis stream key format `agent:stream:{session_id}`
- `engine.agent_messages` PG schema
- `/api/projects/:id/agent/chat` Go HTTP endpoint shape
- `/api/projects/:id/agent/stream` Go SSE endpoint shape
- `QueryEngine.submit_message` interface — the fallback path uses it verbatim

## Components (file manifest)

### forge-core (Go)

| File | Action | LoC delta |
|------|--------|-----------|
| `internal/module/agent/service.go` | Constructor accepts `*workspace.Manager` (nilable); `SubmitMessage` computes `WorkspacePath` when wsManager non-nil and `.git` exists | +25 |
| `cmd/forge-core/main.go` | Pass existing `wsManager` to `agent.NewService` | +1 |
| `internal/module/agent/service_test.go` or `handler_test.go` | 2 new tests: workspace_path populated when repo exists / empty when missing | +30 |
| `internal/module/agent/handler_test.go` | Backfill `nil` as second arg to all `NewService` constructions (expected ~5 sites from G1-G4 tests) | +5 |

### ai-worker (Python)

| File | Action | LoC delta |
|------|--------|-----------|
| `src/openharness/engine/stream_events.py` | Add `class StreamEvent: pass` base class; all 10 event dataclasses inherit from it | +3 |
| `src/api_server.py` | `RunRequest` gains `workspace_path`; `_create_engine` gains `purpose` parameter; extract new `_route_and_stream(req, session_id, ...)` iterator; `_run_and_publish` consumes `_route_and_stream` instead of `engine.submit_message` directly | +60 |
| `tests/openharness/test_stream_events_base.py` (new) | 1 test asserting all stream event classes are `StreamEvent` instances | +15 |
| `tests/test_api_server_route.py` (new) | 5 tests covering the routing matrix (empty/valid/missing workspace, StreamEvent filtering, error propagation) | +120 |

### Infrastructure (docker-compose)

| File | Action | LoC delta |
|------|--------|-----------|
| `docker-compose.dev.yml` | `forge-ai-worker` service: add `volumes: - ${FORGE_WORKSPACE_ROOT:-./workspaces}:/data/forge/workspaces` + `environment: - FORGE_WORKSPACE_ROOT=/data/forge/workspaces` | +4 |
| `ai-worker/.env` (existing) | Add `FORGE_WORKSPACE_ROOT=/data/forge/workspaces` (container-internal path) | +1 |
| `forge-core/.env.example` or docs | Document `FORGE_WORKSPACE_ROOT` expectation for local dev (Windows: `D:/forge-workspaces`, Linux: `/data/forge/workspaces`) | +2 |

## Error handling

### Failure modes (catalogued)

| # | Failure | Detected at | Behavior | User-visible effect |
|---|---------|-------------|----------|---------------------|
| 1 | Project not cloned (`.git` missing) | forge-core `os.Stat` | `workspace_path = ""` → fallback to QueryEngine | Chat works, no fix loop |
| 2 | `wsManager == nil` (dev env without config) | forge-core nil check | Same as 1 | Same as 1 |
| 3 | `os.Stat` non-ENOENT error (permission, etc.) | forge-core | WARN log, `workspace_path = ""` | Same as 1, with log signal |
| 4 | `workspace_path` non-empty but directory missing in ai-worker (volume misconfigured) | ai-worker `os.path.isdir` | Fallback to QueryEngine, WARN log | Same as 1, with log signal |
| 5 | `workspace_path` exists but no language marker (`go.mod`, `package.json`) | pair_pipeline `detect_language` returns None | `build_command=None`, pipeline skips BuildVerify, runs coder + reviewer only | Code generated + reviewed, no fix loop (no build to fail) |
| 6 | Build command timeout | `BuildVerifyHook.run` | Treated as build failure → enters fix loop | Fix loop starts |
| 7 | 3 fix loop cycles exhausted | pair_pipeline main loop | Emit `ErrorEvent` + `SessionComplete` with `build_status="failed"` | Error banner + summary card |
| 8 | `pair_pipeline` raises unexpected exception | outer `try/except` in `_run_and_publish` | Emit `ErrorEvent` to Redis + PG | Error banner |

### Design choice: fallback over fail-fast

When `workspace_path` is non-empty but invalid (case 4), the router **falls back** to `QueryEngine` rather than raising. Reason: misconfigured volume mount is an ops issue, not a user issue. Failing the chat would make the tool unusable until ops fixes it; falling back keeps chat functional with degraded capability and logs a WARN for monitoring.

**Test binding:** `test_route_nonexistent_workspace_falls_back` in `tests/test_api_server_route.py` pins this behavior.

### Known issues (not fixed in this PR, documented for follow-up)

- **Concurrent writes to same session:** `_sessions` dict is unlocked; two concurrent `/api/run` calls with the same `session_id` both mutate the same `QueryEngine`. Pre-existing bug, logged as new P2 in TODOS.md.
- **Mid-stream crash recovery:** If ai-worker container restarts mid-pair_pipeline, partial events are in Redis + PG but no `SessionComplete`. Frontend SSE stream appears cut. Requires Temporal workflow wrapping, out of scope.
- **Multi-model routing:** `ModelRouter` is currently ignored for `Purpose.GENERATE` vs `Purpose.REVIEW` distinction (both end up on same model). Logged as P2 enhancement.

## Testing

### New tests (7 total)

| Test | Scope | Assertion |
|------|-------|-----------|
| `TestAgentService_PassesWorkspacePath_WhenRepoExists` | forge-core unit | Service constructs aiRunRequest with `WorkspacePath == wsManager.ProjectDir(...)` when `.git` exists |
| `TestAgentService_PassesEmpty_WhenRepoMissing` | forge-core unit | `WorkspacePath == ""` when `os.Stat(.git)` fails |
| `test_stream_events_base_class` | ai-worker unit | All 10 event dataclasses satisfy `isinstance(ev, StreamEvent)` |
| `test_route_empty_workspace_uses_queryengine` | ai-worker unit | Empty `workspace_path` → mock QueryEngine called exactly once, mock pair_pipeline not called |
| `test_route_valid_workspace_uses_pair_pipeline` | ai-worker unit | Valid `workspace_path` → mock pair_pipeline called, mock QueryEngine not called |
| `test_route_nonexistent_workspace_falls_back` | ai-worker unit | `workspace_path` set but `os.path.isdir` false → fallback to QueryEngine, WARN log emitted |
| `test_route_filters_non_stream_events` | ai-worker unit | pair_pipeline yields `CycleResult` → not emitted to Redis; `StreamEvent` count == Redis `XADD` calls |
| `test_pair_pipeline_exception_emits_error_event` | ai-worker unit | pair_pipeline raises → `ErrorEvent` appears on Redis stream |

### Regression check

| Existing test | Risk | Mitigation |
|---|---|---|
| `handler_test.go` G1-G4 (dual-storage + cross-tenant) | `NewService` signature changed | Pass `nil` as second arg; service internals tolerate nil wsManager |
| `pytest -m e2e` (pair_pipeline direct import) | Does not go through api_server.py at all | No change; test should stay green |
| `go test ./...` | Service constructor change propagates | All touched sites updated in Phase 4 |
| Existing QueryEngine-path api_server tests (if any) | `_run_and_publish` refactored to call `_route_and_stream` | Tests verifying engine.submit_message behavior continue through the empty-workspace fallback path |

### Manual e2e verification (Phase 5)

1. Harvey creates a minimal Go module at `D:/forge-workspaces/tenant-1/project-999/repo/` with `go.mod` + `main.go` (one-time setup).
2. forge-core recognizes project 999 (existing project creation flow).
3. `curl -X POST http://localhost:8080/api/projects/999/agent/chat -H "Authorization: Bearer <JWT>" -d '{"message":"add a Hello(name string) string function to main.go"}'`
4. Expected SSE event sequence:
   - `thinking_started` (label="Generating code")
   - `text_delta` (stream of coder output)
   - `thinking_stopped`
   - `thinking_started` (label="Running go build ./...")
   - `thinking_stopped`
   - `thinking_started` (label="Reviewing code")
   - `text_delta` (reviewer output, "APPROVE")
   - `turn_complete`
   - `session_complete` with `build_status="passed"`, `files_modified >= 1`
5. `psql -c "SELECT event_type FROM engine.agent_messages WHERE session_id=<sid> ORDER BY id"` shows the full event sequence durably stored.

If the sequence does not include `session_complete` with `build_status="passed"`, the PR is not merge-ready.

## Execution phases (for writing-plans skill)

> This section is a scaffold for the writing-plans skill to expand into a TASK-level plan. Phase ordering is intentional: refactor (no functional change) → new feature gated behind workspace_path → infra (volume mount) → Go side → manual e2e gate.

| Phase | Goal | LoC | Commits | CC time | Gate |
|-------|------|-----|---------|---------|------|
| 0 | Prerequisite validation: confirm docker volume mount feasible on Windows | 0 | 0 | ~20 min | Manual: `docker exec forge-ai-worker ls <mounted_path>` |
| 1 | `StreamEvent` base class refactor (zero functional change) | ~18 | 1 | ~15 min | `pytest ai-worker/tests/` green |
| 2 | ai-worker router + `Purpose` support + isinstance filter (behind unused `workspace_path`) | ~200 | 1 | ~1h | All new ai-worker unit tests pass; existing tests still green |
| 3 | docker-compose volume mount + env alignment | ~7 | 1 | ~10 min | `docker exec forge-ai-worker ls /data/forge/workspaces` returns forge-core-created directory |
| 4 | forge-core `workspace.Manager` injection into `agent.Service` + `SubmitMessage` populates `WorkspacePath` | ~60 | 1 | ~30 min | `go test ./internal/module/agent/... ./cmd/...` green + `go build` succeeds |
| 5 | **Integration gate:** real HTTP e2e against a real Go workspace (manual) | 0 | 0 | ~20 min | `session_complete` event with `build_status="passed"` received; PG has full trace |
| 6 | Docs: update `docs/technical-design.md §3.7`, `docs/TODOS.md`, `/checkpoint` | ~15 | 1 | ~15 min | Commit 5 pushed |

**Total:** ~300 LoC across 5 commits, ~2h CC time, ~7h human time (mostly waiting for gates and debugging volume mount).

**Success criteria for merge:**
1. All gates in Phases 0-4 green
2. Phase 5 manual e2e received `session_complete` with `build_status="passed"`
3. Phase 5 PG verification shows full event history stored
4. `pytest -m e2e` and `go test ./...` still green
5. No changes to frontend code required

## Open questions (for the writing-plans skill to resolve)

- Should Phase 4 unit tests live in `service_test.go` (new) or `handler_test.go` (existing)? The wsManager logic is in service.go, so service_test.go is more appropriate.
- Should `WorkspacePath` be validated (absolute, no `..`, within `FORGE_WORKSPACE_ROOT`) in forge-core or ai-worker? Defense in depth says both. writing-plans skill should detail.
- For Phase 0, what minimal "seed project" should Harvey create to validate the volume mount? A one-file Go module is probably enough.

## References

- Previous PR (shipped pair_pipeline): `docs/plans/2026-04-07-agent-base-loop-reduction.md`
- Known leftovers: `docs/technical-design.md §3.7`
- TODOS source: `docs/TODOS.md` P1 #5
- pair_pipeline source: `ai-worker/src/openharness/engine/pair_pipeline.py`
- api_server source: `ai-worker/src/api_server.py`
- Current router target: `ai-worker/src/api_server.py:182`
- workspace.Manager source: `forge-core/internal/workspace/manager.go`
