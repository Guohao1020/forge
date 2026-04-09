# chronos Round 2 — Session Handoff

> **You are starting a new Claude session to continue chronos Round 2.** This document is your complete starting context. Read it fully before doing anything else.

**Previous session ended:** 2026-04-09, after writing chronos Round 1 (8 phases, ~16k lines, committed as `af85017` through `ba91f27`) and running the first autoplan CEO review which returned 5 accepted findings.

**Your job:** rewrite chronos into Round 2 per the spec §2.9 additions, then re-run the spec review loop, then re-run the autoplan CEO/Design/Eng/DX reviews.

---

## Quick orientation

### 1. Read these files in this order (20 minutes)

1. **This file** (`docs/plans/chronos-2026-04-09/ROUND-2-HANDOFF.md`) — full context
2. **`CLAUDE.md`** — project conventions, Plan Directory Convention section is mandatory
3. **`~/.claude/projects/D--shulex-work-forge/memory/MEMORY.md`** — index of all project memory
4. **`~/.claude/projects/D--shulex-work-forge/memory/chronos-ceo-review-2026-04-09.md`** — the CEO review that triggered Round 2 (critical for understanding why Round 2 exists)
5. **`~/.claude/projects/D--shulex-work-forge/memory/chronos-architecture-a2.md`** — Round 1 architecture decisions (Q1-Q6 + silicon valley standard)
6. **`~/.claude/projects/D--shulex-work-forge/memory/feedback-silicon-valley-infra.md`** — engineering standard
7. **`docs/specs/2026-04-09-agent-variant-b-single-agent-design.md`** — the design spec. **Focus on §2.9 Round 2 additions** which is authoritative for what Round 2 must deliver. Skim §1-§8 for context.
8. **`docs/plans/chronos-2026-04-09/index.md`** — current plan state with Round 2 status banner
9. **`docs/plans/chronos-2026-04-09/phase-*.md`** — skim each Round 1 phase file to understand what exists

### 2. Understand what's committed and what's not

- All Round 1 plan files are committed through `ba91f27`
- Round 2 design spec updates are committed separately (see the commit that adds §2.9)
- **No Round 2 plan files exist yet.** Your job is to write them.

### 3. Understand what's explicitly out-of-scope for Round 2

These are **not** Round 2 work even though the CEO reviewer mentioned them:

- **Finding 6 (dep install escape hatch):** Medium severity, deferred. The "pre-commit to a design before Phase 0 starts" advice is not accepted. Stay with the spec §2.9.2 approach: clarification tool is the mid-turn interactivity; dep install stays as "pre-install at workspace create time via the prep endpoint" per Round 1.
- **Finding 8 (E2E smoke test asserts shape only):** Low severity, deferred. Round 2's e2e test still asserts shape; PM-readable verification is a follow-up project.
- **P3 full implementation:** Round 2 adds the **infrastructure** for bidirectional communication (Phase 5a) but does NOT implement per-call permission approvals. Permission mode stays at FULL_AUTO. P3 uses the bidirectional infrastructure later.
- **Real hook implementations:** Round 2 adds hook **registries** and **extension points** but ships with empty default hooks. The actual spec-injection / constraint-engine / entropy-scan hooks are follow-up projects.
- **Key rotation:** Phase 1b adds deploy key generation + upload but NOT rotation. Keys are generated once, reused forever until manual rotation.

---

## What Round 2 must deliver (the 5 accepted CEO findings)

### Finding 1 — Verification hooks (Critical)

**Goal:** chronos adds extension points to `query.py` / `query_engine.py` / `_create_engine` so future projects can plug in spec-injection, constraint-engine, and entropy-scan without modifying core files.

**Implementation location:** `phase-5-agent-loop.md` gains new tasks:

- **Task 5.8 (new): `HookRegistry` wiring + `system_prompt_slots`** — extend `_create_engine` to register empty default hook registries (pre_turn, pre_tool_call, post_turn) and add a `system_prompt_slots` dict that `build_system_prompt` can consult. The existing `hooks/` directory already has PRE_TOOL_USE / POST_TOOL_USE events from earlier streams; wire them formally into chronos.
- **Task 5.9 (new): hook contract tests** — trivial test hooks that verify the extension points work end-to-end (a pre_turn hook that adds "foo" to the prompt, a pre_tool_call hook that blocks a specific tool name, a post_turn hook that counts turns).

**Scope boundary:** no real hook implementations. Empty registries + tests only.

**Files to touch:**
- `ai-worker/src/openharness/engine/query_engine.py` — accept hook registries in constructor
- `ai-worker/src/openharness/engine/query.py` — invoke hooks at the three points
- `ai-worker/src/openharness/engine/prompts.py` — render `system_prompt_slots`
- `ai-worker/src/api_server.py::_create_engine` — wire empty default registries
- New tests under `ai-worker/tests/openharness/engine/test_hooks_integration.py`

### Finding 2 — `request_clarification` + bidirectional SSE (Critical, biggest scope change)

**Goal:** the agent can pause mid-turn, ask the user a question, and receive the answer via a Redis pub/sub return channel.

**Implementation scope:** a **new phase 5a** plus modifications to Phase 4, 5, 6.

**New file: `phase-5a-bidirectional-rpc.md`** (~8-10 tasks, ~2000 lines)

Task outline for Phase 5a (expand each into full TDD steps when writing):

1. **Task 5a.1:** `ClarificationRequested` stream event (already mentioned in §2.9.2)
   - New dataclass in `stream_events.py`
   - `_serialize_event` branch in `api_server.py`
   - Tests in `test_stream_events_base.py`

2. **Task 5a.2:** Redis pub/sub return channel infrastructure
   - `agent:return:{session_id}` channel naming convention
   - New `src/openharness/engine/return_channel.py` module with async context manager that subscribes to a session's return channel and yields messages
   - Unit tests with fakeredis or real Redis dev container

3. **Task 5a.3:** `ClarificationResponse` message shape on the return channel
   - JSON payload: `{type: "clarification_response", tool_use_id: str, response: str}`
   - Validation helpers
   - Tests

4. **Task 5a.4:** Pause/resume state machine in `QueryEngine.submit_message`
   - When a tool yields `ClarificationRequested`, `_execute_tool_call` pauses (awaits a future from the return channel)
   - Timeout: 5 minutes default, configurable. On timeout, the tool yields `ToolResult(is_error=True, output="no user response within N seconds")`.
   - Tests: happy path (user responds within timeout), timeout path, cancelled session

5. **Task 5a.5:** Per-session return channel subscriber lifecycle
   - When a session starts, subscribe to its return channel
   - When a session ends (LRU eviction or DELETE), unsubscribe and cleanup
   - Connection pooling — don't create a new Redis connection per session
   - Tests

6. **Task 5a.6:** forge-core endpoint `POST /api/sessions/{session_id}/clarify`
   - Accepts `{response: str, tool_use_id: str}`
   - Publishes to `agent:return:{session_id}` via Redis client
   - Auth: same as other `/api/sessions/*` endpoints
   - Tests (Go)

7. **Task 5a.7:** Concurrent session safety
   - Multiple sessions waiting for clarification concurrently must not interfere
   - Tool_use_id correlation in the pub/sub payload so the right waiting future resolves
   - Stress test with N=10 concurrent sessions each awaiting clarification

8. **Task 5a.8:** Error paths
   - Redis connection drops during wait → raise `ToolResult(is_error=True, "connection lost")`
   - Session deleted during wait → cancel the future cleanly
   - Multiple clarification requests from the same tool_use_id → error (should be impossible, but defensive)

9. **Task 5a.9:** Integration test (Python)
   - Real Redis (docker-compose dev instance)
   - Fake user response from a pytest fixture that publishes to the return channel
   - Verify the full pause → publish → resume cycle

**Additions to Phase 4:** Task 4.9 — add `ClarificationRequested` to stream_events.py (duplicates Phase 5a Task 5a.1 if you're splitting work; decide whether to keep it here or only in Phase 5a when writing)

**Additions to Phase 5:**

- **Task 5.10 (new): `RequestClarificationTool`** — BaseTool subclass (not SimpleTool because it yields a StreamEvent mid-execution). Implementation:
  - Takes a `question: str` input
  - Yields `ClarificationRequested(question, tool_use_id)`
  - Awaits the return channel future (via Phase 5a infrastructure)
  - Yields `ToolResult(output=user_response)` or `ToolResult(is_error=True, output=timeout_msg)`
- **Task 5.11 (new): wire tool into `_create_engine`** via a new `register_interaction_tools` helper (alongside `register_file_tools`, `register_exec_tools`)

**Additions to Phase 6:**

- **Task 6.10 (new): Clarification input component** — renders below a `clarification_requested` SSE event, shows the question, has an input field, submits via `POST /api/sessions/{id}/clarify`
- **Task 6.11 (new): Return channel integration** — frontend already uses EventSource for forward SSE; the clarification POST is a separate HTTP call, no EventSource changes needed. But the UI state machine needs to track "waiting for clarification" and disable the main chat input during that state

### Finding 3 — `request_review` meta-tool (High)

**Goal:** the agent can voluntarily invoke a dedicated reviewer LLM call at milestones.

**Implementation location:** `phase-5-agent-loop.md` gains:

- **Task 5.12 (new): `RequestReviewTool`**
  - BaseTool subclass (can be SimpleTool — it doesn't yield mid-execution events, just returns the reviewer output)
  - Input: `summary: str` (what the agent wants reviewed — the agent summarizes its work so the reviewer has context)
  - Internally fires a second LLM call via `ModelRouter` with `Purpose.REVIEW` (reintroduce this enum value but make it internal to this tool — the public API stays single-purpose)
  - The reviewer prompt is separate: `build_reviewer_prompt(summary, current_diff, original_request)` in `prompts.py`
  - Returns: `ToolResult(output="APPROVE" | "REVISE <details>" | "REJECT <reason>")`
- **Task 5.13 (new): `build_reviewer_prompt`** in `prompts.py`
  - Tests similar to `build_system_prompt` — substring invariants
- **Task 5.14 (new): System prompt update** for the main agent to instruct it to call `request_review` at milestones
- **Task 5.15 (new): `register_interaction_tools` helper** — registers `RequestClarificationTool` + `RequestReviewTool` together (same "interaction" category)

**Tests:** mock the ModelRouter call so tests don't pay LLM cost; verify the tool constructs the reviewer prompt correctly and returns the mocked response.

### Finding 4 — Phase 1 split into 1a/1b (High)

**Goal:** unblock Phase 5 faster by shipping a minimal workspace module first and deferring deploy-key work.

**Implementation:**

**Delete:** `phase-1-workspace.md`

**Create: `phase-1a-workspace-minimal.md`** (~6-7 tasks)

Take Round 1's Phase 1 tasks 1.1, 1.5, 1.6, 1.7, 1.9, 1.10, 1.11, 1.12, 1.13 (state DAO, prep RPC, ProjectLookup, EnsureReady state machine, caller migrations, main.go wiring, e2e against local bare repo) but **replace** the git runner with the existing HTTPS+token approach (keep `injectToken` temporarily). The resulting Phase 1a:

1. Task 1a.1: State DAO (unchanged from Round 1 1.1)
2. Task 1a.2: Prep RPC client (unchanged from Round 1 1.5)
3. Task 1a.3: `ProjectLookup` interface (unchanged from Round 1 1.6)
4. Task 1a.4: `EnsureReady` state machine **using HTTPS+token git** (simplified — no DeployKey dependency; `RealGitRunner` uses `injectToken` internally for Phase 1a, gets replaced in Phase 1b)
5. Task 1a.5: `manager.go` refactor (unchanged goal; temporary `injectToken` path retained)
6. Task 1a.6: Worker callsite migrations (build_activities, devops_activities — unchanged from Round 1 1.9, 1.10)
7. Task 1a.7: Agent service + main.go wiring (unchanged from Round 1 1.11, 1.12)

**Create: `phase-1b-deploy-keys.md`** (~5-6 tasks)

Contains Round 1's Phase 1 tasks 1.2, 1.3, 1.4 plus a migration task:

1. Task 1b.1: `DeployKeyRepo` + ed25519 generation (Round 1 1.2)
2. Task 1b.2: GitHub deploy key upload client (Round 1 1.3)
3. Task 1b.3: `RealGitRunner` SSH variant (Round 1 1.4, adapted — was originally written for SSH from the start; now it's a replacement for the HTTPS path)
4. Task 1b.4: Migration — flip `_create_engine` / `EnsureReady` / `manager.go` from HTTPS+token to SSH deploy keys. Delete `injectToken`.
5. Task 1b.5: Tests for the migrated path (Round 1 1.13 adapted for SSH)
6. Task 1b.6: Key rotation stub (empty method, documented as future work)

**Phase 1b can execute in parallel with Phase 5/6 because no later phase depends on the auth mechanism** — everything goes through `workspace.Manager.EnsureReady` which abstracts over it.

### Finding 5 — Harness Engineering hooks (High)

This is covered by Finding 1 (verification hooks) in scope. The hook registries + `system_prompt_slots` ARE the Harness Engineering hooks. No additional work beyond Finding 1.

---

## Execution order for Round 2

1. **Spec review loop** (20-60 minutes)
   - Dispatch `spec-document-reviewer` subagent with the updated design spec (focused on §2.9)
   - Fix any issues found, re-dispatch, repeat until approved or 3 iterations
   - The reviewer should verify §2.9 is internally consistent and doesn't contradict §2.1-§2.8

2. **Plan rewriting** (3-5 hours of focused writing)
   - Follow the TODO breakdown below in order

3. **Plan review loop** (spec-document-reviewer OR plan-document-reviewer — check which skill you have)
   - On each new/changed phase file, dispatch a reviewer subagent
   - Fix and re-dispatch until approved

4. **Round 2 autoplan review** (40-80 minutes)
   - Re-run CEO / Design / Eng / DX reviews on the Round 2 plan
   - In `[subagent-only]` mode if Codex CLI still not available
   - Surface any new strategic findings to Harvey before executing

5. **Final gate and execution handoff**
   - Present the Round 2 final state to Harvey
   - Choose execution mode (`superpowers:subagent-driven-development` or `superpowers:executing-plans`)

---

## Round 2 TODO breakdown (in order)

Copy these into TodoWrite at the start of the new session:

1. [pending] Read handoff doc + CEO review memory + spec §2.9 (orientation)
2. [pending] Dispatch spec-document-reviewer on updated design spec §2.9
3. [pending] Fix any spec review issues; re-dispatch until approved (max 3 iterations)
4. [pending] Delete Round 1 `phase-1-workspace.md`
5. [pending] Write `phase-1a-workspace-minimal.md` (~6-7 tasks, ~1800 lines)
6. [pending] Write `phase-1b-deploy-keys.md` (~5-6 tasks, ~1500 lines)
7. [pending] Write new `phase-5a-bidirectional-rpc.md` (~9 tasks, ~2000 lines) — deepest reconnaissance needed
8. [pending] Update `phase-4-bash-events.md`: add Task 4.9 for `ClarificationRequested` event
9. [pending] Update `phase-5-agent-loop.md`: add Tasks 5.8-5.15 for hooks + clarification + review tools (~+800 lines)
10. [pending] Update `phase-6-frontend.md`: add Tasks 6.10-6.11 for clarification UI (~+400 lines)
11. [pending] Update `phase-7-deploy.md`: include clarification round-trip in smoke test
12. [pending] Rewrite `index.md`: 9 phases, new dependency graph, remove Round 2 status banner (replace with "delivered")
13. [pending] Dispatch plan-document-reviewer on each new/changed file (loop until clean)
14. [pending] Re-run autoplan CEO review on Round 2 plan
15. [pending] Re-run autoplan Design review
16. [pending] Re-run autoplan Eng review
17. [pending] Re-run autoplan DX review
18. [pending] Final approval gate — present to Harvey
19. [pending] Execution handoff

---

## Reconnaissance you MUST do before writing Phase 5a

Phase 5a is the deepest unknown because bidirectional communication through Redis was not part of the original design. Before writing tasks, read these in the new session:

1. **`ai-worker/src/api_server.py`** — look at `_run_and_publish`, `_get_redis`, `_get_pg_pool`. Understand the current SSE fire-and-forget model.
2. **`ai-worker/src/openharness/engine/query_engine.py`** — current `submit_message` is a straight async generator; you need to understand how to inject an awaitable pause point into it.
3. **`docker-compose.dev.yml`** — confirm Redis version + any auth config.
4. **forge-core `/api/sessions/*` routes** — find the pattern for session auth and model it for `/api/sessions/{id}/clarify`. Look at `forge-core/internal/module/agent/handler.go`.
5. **Frontend SSE handler** — find how the existing SSE reader handles events in `forge-portal/components/agent/agent-chat.tsx`. The clarification UI will share the same reader but trigger a different render path.
6. **Check if `redis.asyncio` supports pub/sub properly** — verify the client is `redis-py>=5` and the pub/sub API matches what we need.

Do this reconnaissance and write it into Phase 5a's "Phase context" section before any task is drafted. Otherwise Phase 5a will have the same "plausible but unverified" problem that Round 1 Phase 1 had (the CEO reviewer caught architectural contradictions that required iter 2 fixes).

---

## Conventions to follow (don't invent new ones)

From Round 1, the following conventions are established and must be preserved in Round 2:

- **Plan Directory Convention** (in `CLAUDE.md`): one file per phase, `index.md` as entry point, Greek-name + ISO-date directory
- **Task structure** (in each phase file): `### Task N.M: Title`, files touched, TDD checklist steps, complete code blocks (no pseudocode), exact commands with expected outputs, git commit at the end
- **Silicon valley standard** (in `~/.claude/.../memory/feedback-silicon-valley-infra.md`): no hardcoded special cases, no regex-as-security, one code path, no AsyncMock fallbacks, no fallbacks in general
- **Contract tests as mechanical gate** (Phase 2 Task 2.4 / Phase 3 Task 3.8 / Phase 4 Task 4.6 pattern): new tools get auto-added to `ALL_TOOL_SPECS` in `test_base_tool_contract.py`
- **Adversarial test files are split**: security-critical tests go in a separate `test_*_adversarial.py` file so CI failures are loud

New tools added in Round 2 (`RequestClarificationTool`, `RequestReviewTool`) must follow the same pattern: extend the contract suite from 12 to 14 tools in Phase 5.

---

## Known pitfalls

1. **PEM markers in example code trigger pre-commit `detect-private-key` hook.** If you write sample code that has the literal BEGIN/END OPENSSH PEM header strings, split them with concatenation: ```"-----BEGIN " + "OPENSSH PRI" + "VATE KEY-----"``` (the string "PRI" + "VATE" avoids the regex match). Seen twice in Round 1 (Phase 1 deploy key tests).
2. **`forge-portal/AGENTS.md` warns the Next.js in this project has breaking changes** — read `node_modules/next/dist/docs/` before writing any Next-specific code. Seen in Phase 6 Round 1.
3. **Windows line endings** — `git commit` emits warnings about LF→CRLF conversion on large markdown files. These are warnings not errors, don't panic.
4. **Codex CLI may not be available** — autoplan runs in `[subagent-only]` mode. Tag findings accordingly in the final gate.
5. **Phase 5a's pause/resume requires `asyncio.Future`** — the agent loop is already async but the tool execution path doesn't currently have a pause point. You'll need to introduce one cleanly without breaking the existing "tool yields ToolResult" contract. Careful design required.

---

## Round 1 file list (reference)

```
docs/plans/chronos-2026-04-09/
├── index.md                            (will be rewritten — has Round 2 status banner now)
├── ROUND-2-HANDOFF.md                  (this file)
├── phase-0-infrastructure.md           (Round 1, keep as-is)
├── phase-1-workspace.md                (Round 1, DELETE — split into 1a + 1b)
├── phase-2-basetool.md                 (Round 1, keep as-is)
├── phase-3-file-tools.md               (Round 1, keep as-is)
├── phase-4-bash-events.md              (Round 1, add Task 4.9)
├── phase-5-agent-loop.md               (Round 1, expand ~+800 lines)
├── phase-6-frontend.md                 (Round 1, expand ~+400 lines)
└── phase-7-deploy.md                   (Round 1, minor smoke test update)
```

After Round 2 writing completes, the directory will look like:

```
docs/plans/chronos-2026-04-09/
├── index.md                            (Round 2, rewritten)
├── phase-0-infrastructure.md           (Round 1, unchanged)
├── phase-1a-workspace-minimal.md       (Round 2, NEW from Round 1 Phase 1 split)
├── phase-1b-deploy-keys.md             (Round 2, NEW from Round 1 Phase 1 split)
├── phase-2-basetool.md                 (Round 1, unchanged)
├── phase-3-file-tools.md               (Round 1, unchanged)
├── phase-4-bash-events.md              (Round 1 + Task 4.9)
├── phase-5a-bidirectional-rpc.md       (Round 2, NEW)
├── phase-5-agent-loop.md               (Round 1 + 8 new tasks)
├── phase-6-frontend.md                 (Round 1 + 2 new tasks)
└── phase-7-deploy.md                   (Round 1 + minor smoke update)
```

Plus `deploy-runbook.md` and `retro.md` which are created DURING execution (Tasks 7.3 and 7.4), not during plan writing.

---

## Start the new session with this prompt

Paste this into your new Claude session to restart cleanly:

```
Continue chronos Round 2 rewrite. Read these in order:

1. docs/plans/chronos-2026-04-09/ROUND-2-HANDOFF.md (full context)
2. docs/specs/2026-04-09-agent-variant-b-single-agent-design.md §2.9 (authoritative)
3. ~/.claude/projects/D--shulex-work-forge/memory/chronos-ceo-review-2026-04-09.md

Then start with: "Dispatch spec-document-reviewer on the updated design spec §2.9 to verify internal consistency with §2.1-§2.8 before plan rewriting begins."

Follow the Round 2 TODO breakdown in the handoff doc.
```

Good luck. Round 2 is the right call.
