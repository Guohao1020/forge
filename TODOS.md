# TODOS

Deferred items from reviews and retros. Format: one-line what + why + context + dependency.

## Security (P0 — Marketplace prerequisite)

- [ ] **Workspace sandbox isolation — P0 gate before Marketplace** — AI-generated code goes through `BuildVerifyHook` which runs `go build` (or `mvn`, `npm install`, `cargo build`, etc.) against the LLM's output. Every one of these tools executes code as a side effect of compilation: Go runs `init()` functions and `go:generate` directives, npm runs `postinstall` scripts, Maven runs build plugins, Cargo runs `build.rs`. The current implementation drops the LLM's files into a host subdirectory and shells out — meaning ANY malicious snippet from the LLM (jailbreak, prompt injection, or just a hallucinated dependency that resolves to a typosquat) is **remote code execution on the host running ai-worker**. This is acceptable for a single-developer self-hosted setup; it is **not** acceptable for Marketplace (users sharing skills) which turns the build hook into a supply-chain attack vector. Mitigation: run `BuildVerifyHook.run()` inside a disposable sandbox — `docker run --rm --network=none --cap-drop=ALL --read-only --tmpfs /work -v <generated>:/work:ro <toolchain-image> <build_command>`, or `firejail`/`gVisor`/`bwrap` equivalents on Linux. Pin a per-language toolchain image (`golang:1.22-alpine`, `node:20-alpine`, etc.) so the host's Go install is irrelevant. **Effort:** M (人 ~3天 / CC ~3h). **Acceptance:** ai-worker tests/e2e still passes; a malicious test fixture that tries to write to `/etc/passwd` from inside `init()` is blocked; sandbox failure modes (no network, OOM, timeout) all surface as `BuildVerifyResult(success=False)` with actionable messages. **Depends on:** decision about Linux vs cross-platform sandbox (Docker is portable but adds 2-5s per build; bubblewrap is faster but Linux-only). **Why P0 but deferred:** unblocks Marketplace, but base loop reduction (2026-04-08 PR) needs to ship first to prove the loop even works before hardening it.

## Refactor (P1 — pre-OpenHarness cleanup)

- [ ] **Migrate `src/activities/` and 4 test files off `src/agents/*.py`, then delete `src/agents/`** — the 7 files under `ai-worker/src/agents/` (`base.py`, `analyst.py`, `coder.py`, `planner.py`, `profiler.py`, `reviewer.py`, `test_writer.py`) are NOT orphans despite the 2026-04-06 cross-model learning that flagged them. `grep -rn "from src.agents"` finds 6 importers in `src/activities/` (`analyze.py`, `generate.py`, `plan.py`, `profile.py`, `review.py`, `test_writing.py`) and 4 test files (`test_agents.py`, `test_agents_extended.py`, `test_agent_loop.py`, `test_analyze_flow.py`). They were the Temporal-activities-era agent implementations. The OpenHarness `QueryEngine` (`src/openharness/engine/query_engine.py`) is meant to replace them, but the activities haven't been migrated yet — they still call into `BaseAgent`. Plan to migrate: (1) port `src/activities/*.py` to use `QueryEngine` directly instead of `BaseAgent`, (2) port the agent test suites to use `QueryEngine` mocks, (3) `git rm -r src/agents/`, (4) update `src/worker.py` activity registration. **Effort:** L (人 ~5天 / CC ~2h). **Why P1 not P0:** `src/activities/` is the Temporal pathway, which the new HTTP `/api/run` path bypasses entirely; the activities are still wired but no longer the critical path. **Depends on:** confirming whether any existing Temporal workflow still creates analyze/generate/plan/profile/review/test_writing activities (if yes, those workflows have to migrate first). **Context:** found during 2026-04-08 base-loop-reduction TASK 12 — plan said "delete the orphans" but they aren't orphans yet.

## UI Alignment

- [ ] **Compare `components/agent/build-card.tsx` against Variant B mockup lines 505-554** — red border, `BUILD FAILED` title, `exit code + duration` meta, multi-line log with `.error-line` red highlight. Not yet inspected in 2026-04-07 eng review; Section 2 was already at 5 decisions and build-card was deferred to avoid scope creep. Pick up alongside the `code-panel` shiki rewrite since both touch code-view styling. **Depends on:** Variant B token rename (Section 2.2 finding, `--text-error`/`--bg-error` must exist).

## Dev Experience

- [ ] **Move `forge-core` into `docker-compose.dev.yml`** — currently `forge-core` runs as a host process spawned by Harvey's IDE/terminal, while `ai-worker` / `postgres` / `redis` / `temporal` all run in compose. This split causes three real frictions: (1) Windows file lock prevents `go build -o forge-core.exe` from overwriting the running binary, forcing the `forge-core-new.exe` workaround, (2) CC cannot safely restart `forge-core` after backend changes because it doesn't know the spawn context, so plan TASK 9-style "rebuild + Harvey manually restart" steps recur in every base-loop change, (3) the host process cannot use `host.docker.internal` symmetry — the rest of the compose graph reaches it via `host.docker.internal:8080` instead of by service name. Putting `forge-core` in compose makes restart = `docker compose restart forge-core`, eliminates the file-lock workaround, and lets CC drive backend integration testing end-to-end without manual handoffs.
  Effort: S (人 ~2h / CC ~30min). Acceptance: `docker compose up -d` brings the whole stack including `forge-core`, `curl http://localhost:8080/api/healthz` returns 200, and the existing host launch path is removed from `forge-core/cmd/forge-core/main.go` docs (or kept as an alternate dev mode). **Depends on:** decision about whether forge-core needs source hot-reload (`air` inside the container) or accepts cold restarts on every code change. **Out of scope for** the agent base loop reduction PR — surfaced during 2026-04-08 TASK 9 prep when explaining `forge-core-new.exe`.

- [x] **Add `forge-core/forge-core-new.exe` to `.gitignore`** — ✅ DONE 2026-04-09 in commit `62dc9dd`. Two-line fix added `/forge-core-new` and `/forge-core-new.exe` to `forge-core/.gitignore`. Verified with `git check-ignore -v forge-core/forge-core-new.exe`. The compose migration above is independent and remains pending.

## Known Issues — pair_pipeline production wire-up followups (P2)

Surfaced during the 2026-04-09 `/api/run → _route_and_stream → pair_pipeline` wire-up
(plan `docs/plans/2026-04-09-pair-pipeline-production-wire.md`). The happy path is
working end-to-end as of commit `280b88f` and was verified against project-999 on
2026-04-09 12:49 local. The items below are real issues found during or after
Phase 4 — none block the current PR, but they should all get addressed before we
promote the chat path to "GA" for customers.

- [ ] **Session concurrency lock** — `POST /api/projects/:id/agent/chat` does not
  serialize concurrent requests for the same `session_id`. If a user fires two
  chat requests against the same session in parallel (double-click, network
  retry, two browser tabs), both hit `_run_and_publish` and both spin up
  pair_pipeline runs against the same workspace directory — producing interleaved
  writes to Redis streams, possibly interleaved Edit tool calls racing on
  `main.go`, and an `agent_messages` history that looks like two conversations
  woven together. Fix: per-session advisory lock (Redis `SETNX` with TTL or a
  Postgres advisory lock keyed on session_id) held for the duration of
  `_route_and_stream`. Reject the second request with 409 and a clear error.
  **Effort:** S (人 ~4h / CC ~30min). **Acceptance:** a parallel-POST test
  submits 2 messages to the same session_id simultaneously; exactly one gets
  202, the other gets 409; Redis stream has no interleaved events. **Depends
  on:** decision about whether 409 should expose a `retry_after` hint based on
  the first request's running duration.

- [ ] **Mid-stream crash recovery** — when ai-worker's `_run_and_publish`
  background task crashes partway through a pair_pipeline run (OOM, connection
  reset from the LLM, a rare Python exception inside `run_pair_pipeline`), the
  SSE client sees a half-finished stream: typically some `text_delta` events,
  no `session_complete`, and eventually only heartbeats. There is no explicit
  `session_failed` or `session_aborted` event. The PG dual-storage side is
  equally confused — `user_message` is persisted but no `error` event is
  written. Downstream frontend has to time out on its own. Fix: wrap
  `_run_and_publish` body in a try/except that emits a synthetic `ErrorEvent`
  with `recoverable=false` to Redis before the task dies, and persist the same
  event to `agent_messages`. **Effort:** S (人 ~3h / CC ~20min). **Acceptance:**
  a pytest that raises `RuntimeError("boom")` from inside a stubbed
  `run_pair_pipeline` sees the ErrorEvent on the Redis stream and in PG.
  **Depends on:** nothing.

- [ ] **ModelRouter Purpose differentiation** — `_create_engine(req,
  purpose=Purpose.GENERATE)` and `_create_engine(req, purpose=Purpose.REVIEW)`
  both currently route to the same underlying Claude model. The Purpose
  parameter is plumbed through but ModelRouter does not branch on it yet. In
  practice this means coder and reviewer are the same LLM with different system
  prompts — which works but misses the whole point of separation of concerns.
  Fix: extend ModelRouter to honor Purpose → optionally select a cheaper/faster
  model for REVIEW (e.g. Sonnet for GENERATE, Haiku for REVIEW) with an env
  override per tenant. **Effort:** M (人 ~1天 / CC ~1h). **Acceptance:** a
  Purpose-based e2e test shows two different model_call entries for the same
  pair_pipeline run, and a configuration toggle lets an operator override the
  REVIEW model per-tenant. **Depends on:** model pricing data for the routing
  heuristic, cost dashboard updates to distinguish coder vs reviewer spend.

- [ ] **ai-worker logging is invisible under uvicorn** — `ai-worker/src/api_server.py`
  calls `logging.basicConfig(...)` inside `if __name__ == "__main__":` (line 524).
  `start.sh` launches the API via `uvicorn src.api_server:app` which means
  `__main__` is uvicorn, not api_server, so basicConfig never runs. Every
  `logger.info(...)` call in api_server.py — including the critical
  `pair_pipeline route: session=... workspace=...` diagnostic at line 248 that
  the Phase 4 plan told operators to grep for — is silently dropped. Today we
  only see `print()` calls and uvicorn's own access log. I chased an imaginary
  bug for 20 minutes because of this. Fix: move `logging.basicConfig(...)` to
  module top-level, or configure uvicorn via `--log-config` to use the same
  JSON formatter. **Effort:** XS (人 ~30min / CC ~5min). **Acceptance:** after
  fix, `docker logs forge-ai-worker | grep "pair_pipeline route"` shows a line
  for every pair_pipeline chat we send; the JSON log format (`{"time", "level",
  "msg"}`) matches what's documented as the ai-worker log shape. **Depends on:**
  nothing. Discovered 2026-04-09 during Phase 4 diagnostics.

- [ ] **ai-worker PG pool has no cold-start retry** — if `forge-postgres` comes
  up AFTER ai-worker's first connect attempt (common when docker compose
  starts the whole stack in dependency order but ai-worker races ahead), the
  PG pool init at `api_server.py` ~line 515 fails with `[Errno 111] Connection
  refused`, prints `"PG pool not available — agent history will rely on Redis
  only"`, and the code path never tries again for the lifetime of the
  container. Result: `agent_messages` dual-storage silently degrades to
  Redis-only, so `/api/projects/:id/agent/sessions/:sid/messages` returns only
  the forge-core-side `user_message` rows and zero assistant/tool events. Very
  confusing when debugging. Fix: retry PG pool init on first use of the pool,
  OR add a background task that periodically reconciles the pool. **Effort:**
  S (人 ~4h / CC ~30min). **Acceptance:** pytest that simulates postgres
  unavailable on first connect and available on second — second connect
  succeeds and persists an event to PG. **Depends on:** nothing. Discovered
  2026-04-09 during Phase 4 dual-storage verification.

- [ ] **LLM doesn't actually call Edit under pair_pipeline when `code_files=None`** —
  during Phase 4 verification against project-999 the LLM produced a correct
  `Hello(name string) string` implementation as a markdown code block in its
  response message but never invoked the Edit tool to write to `main.go`.
  Result: `files_modified=0`, BuildVerify trivially passed, SSE `session_complete`
  arrived with `build_status=passed` — but the workspace is unchanged on disk.
  Root cause suspected in the coder system prompt: when `code_files=None` is
  passed to `run_pair_pipeline`, the prompt does not strongly enough instruct
  the LLM to read the existing file (Read/Glob/Grep) AND then call Edit. The
  LLM's default behavior when asked to "add a function" is to write code in
  chat, which is wrong for this pipeline. Fix: review and strengthen the
  coder prompt template for the `code_files=None` branch, and add a
  pair_pipeline acceptance test that asserts `files_modified >= 1` when the
  initial_prompt is an edit request and the workspace has a matching target
  file. **Effort:** M (人 ~1天 / CC ~1h). **Acceptance:** post-fix, the same
  Phase 4 e2e chat against project-999 produces `files_modified=1` and
  `main.go` on disk has the `Hello` function. **Depends on:** coder prompt
  authoring conventions; may interact with the reviewer prompt.

## Brand Cleanup

- [ ] **Sweep the codebase for "深空指挥中心" purple-brand residue** — the old purple brand (`#8B5CF6`, `#7C3AED`, aurora background, "深空指挥中心" visual language, `accent-glow` shadow effects) was replaced by Variant B blue (`#2563eb` light / `#3b82f6` dark, Cursor/VS Code density). Residue already found in `step-ribbon.tsx:30` (shadow-glow) and `CLAUDE.md` Frontend section. Grep candidates:
  - `grep -ri "8B5CF6\|8b5cf6\|7C3AED\|7c3aed" --include="*.tsx" --include="*.ts" --include="*.css" --include="*.md"`
  - `grep -ri "深空\|指挥中心\|aurora\|forge purple\|Forge Purple" .`
  - `grep -ri "accent-glow\|shadow-glow" --include="*.tsx"`
  Update `CLAUDE.md` Frontend section to replace "Brand color: Forge Purple #8B5CF6" and "深空指挥中心" visual language with Variant B ("Dense Engineering, Cursor/VS Code inspired, blue #2563eb/#3b82f6"). Update `docs/product-design.md` brand sections. **Context:** Variant B was approved in `~/.gstack/projects/voc-shulex-forge/designs/agent-terminal-shotgun-20260406/approved.json` on 2026-04-06; design-doc section on brand shift exists. This is documentation debt, not a bug, but inconsistency risks new code drifting back toward purple.
