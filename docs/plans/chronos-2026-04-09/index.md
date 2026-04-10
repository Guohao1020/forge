# chronos — Agent Variant B Single-Agent Implementation

> **Code name:** chronos (Greek personification of time). This refactor "folds time" — collapsing the pair_pipeline-era workaround into the A2-era real agent architecture in one single branch.

**Started:** 2026-04-09
**Status:** ✅ **Round 2 plan delivered** — 9 phases, ~76 tasks, ~21k lines. Round 1 was reviewed by autoplan CEO subagent on 2026-04-09 and received 5 accepted strategic findings (see `~/.claude/projects/D--shulex-work-forge/memory/chronos-ceo-review-2026-04-09.md`). Round 2 applies all 5 findings and passed a second spec review iteration with only minor call-site fixes that have been applied.
**Branch:** `feat/agent-variant-b-single-agent` (off `main`)

---

## Round 2 delta summary

Round 2 (2026-04-09) adds five strategic pieces that Round 1 lacked:

1. **Verification hooks in the agent loop** (spec §2.9.1) — `AgentHookRegistry` with `pre_turn` / `pre_tool_call` / `post_turn` extension points + `system_prompt_slots`. Empty by default in chronos; downstream projects (spec injection, constraint engine, entropy scan) populate them without modifying `query.py`. Landed in Phase 5 Tasks 5.8–5.11.
2. **`request_clarification` meta-tool + bidirectional SSE** (spec §2.9.2) — agent can pause mid-turn, ask the user a question via a new `ClarificationRequested` stream event, and receive the response via a Redis pub/sub return channel (`agent:return:{session_id}`). Timeout = 10 minutes → session halts (no fallbacks). Landed as a new Phase 5a plus additions to Phases 4, 5, 6, 7.
3. **`request_review` meta-tool** (spec §2.9.3) — agent can voluntarily invoke a dedicated reviewer LLM call at major milestones. Reviewer sees the current `git diff HEAD` + user's original request, returns `APPROVE | REVISE <what> | REJECT <why>`. Single agent still, plus an optional meta-tool. Landed in Phase 5 Tasks 5.12–5.14.
4. **Phase 1 split into 1a (minimal) + 1b (deploy keys)** (spec §2.9.4) — Phase 1a ships workspace module with HTTPS+token auth (retains the existing `injectToken` helper), unblocking Phase 5 immediately. Phase 1b ships SSH deploy keys + GitHub upload + key rotation stub, in parallel with Phases 2–7 or after Phase 5 — no later phase depends on auth mechanism. Phase 1b gates public deployment (§2.9.4.d).
5. **Harness Engineering hooks** — covered by item #1 (the same `AgentHookRegistry` is the Harness Engineering extension surface).

**Silicon Valley standard (§2.8) maintained throughout:** no fallbacks, no AsyncMock, no hardcoded special cases, one code path. Clarification timeout halts the session — it does not return `is_error=True` and let the agent continue. `Purpose.REVIEW` enum value is retained only for `RequestReviewTool`'s internal `ModelRouter` call — `_create_engine`'s Round 1 switch branch stays deleted.

---

## Goal

Rebuild Forge's AI agent pipeline around a single tool-using agent that can actually drive the Variant B Cursor-style UI, replacing the current `pair_pipeline` workaround that uses regex-extracted code blocks. Add the Round 2 strategic additions above (verification hooks, clarification round-trip, optional reviewer, phase-1 split) so the result is differentiated from Cursor/Claude Code for Forge's PM/ops user base — not just a rebuilt Cursor clone.

## Architecture (A2 + Round 2 additions)

Kill `pair_pipeline.py`. Give the agent a real tool surface (`read` / `write` / `edit` / `glob` / `grep` / `list_directory` / `bash` + `set_phase` + `request_clarification` + `request_review`). Bash runs inside bubblewrap (no network, cwd locked, env whitelist). Workspace is one long-lived clone per project. `BaseTool` is an async-generator contract so tools can emit mid-execution stream events without hardcoded special cases. Agent hooks are in-process Python callables on `AgentHookRegistry`; clarification pause/resume is an `asyncio.Future` on `ClarificationCoordinator` resolved by a `ReturnChannel` subscriber listening to Redis pub/sub.

Auth evolves over two phases:

- **Phase 1a:** HTTPS+token git auth via the existing `injectToken` helper. Ships workspace module state machine, prep RPC, caller migrations, and the `EnsureReady` API.
- **Phase 1b:** replaces HTTPS+token with per-project ed25519 SSH deploy keys + AES-GCM storage + GitHub deploy-key upload. Hard cutover — `injectToken` deleted. Runs in parallel with Phases 2–7.

## Tech Stack

Go 1.22 (forge-core workspace module + secrets service + clarify handler), Python 3.12 (ai-worker tools + agent loop + return channel), bubblewrap (Linux namespace sandbox), ripgrep, pathspec (Python glob), AES-GCM (Go `crypto/cipher`), ed25519 (Go `crypto/ed25519`), FastAPI, `redis.asyncio` (pub/sub), Next.js 16 + React 19 + shadcn/ui (forge-portal).

## Design Spec

- **[Design spec (3941 lines after Round 2 updates)](../../specs/2026-04-09-agent-variant-b-single-agent-design.md)** — the authoritative source for every "why" decision in this plan. §2.9 contains the Round 2 strategic additions (verification hooks, bidirectional RPC, request_review, phase-1 split).
- Spec review history:
  - Round 1: 3 iterations through spec-document-reviewer, approved (commits `849d70a` → `d0828c2` → `351f9e4`).
  - Round 2: 2 iterations. Iteration 1 returned 15 findings (3 critical, 7 high-priority, 5 recommendations); all fixed. Iteration 2 returned "Approved" with 3 minor call-site inconsistencies (async `_create_engine` not awaited in §5.7, async `build_system_prompt` not awaited in §4.12, `BaseTool` contract tightening for `SessionHaltError`); all fixed.

## Decision chain (Q1–Q6 + Round 2 additions)

| # | Topic | Decision |
|---|---|---|
| Q1 | Scope | **A** — only agent interaction layer |
| Q2 | Architecture | **A2** — kill pair_pipeline, single agent + tool-use loop |
| Q3 | Tool surface | **T2 + Round 2 meta-tools** — read/write/edit/glob/grep/list_directory/bash + set_phase + request_clarification + request_review |
| Q3a | File paths | Direct workspace ops (no HTTP indirection) |
| Q3b | Bash sandbox | **bubblewrap**, denylist only as UX hint |
| Q4 | Permissions | **P1 → P3**: FULL_AUTO now, bidirectional RPC reserved for future P3 |
| Q5.1 | Step Ribbon | Dynamic via `set_phase` meta-tool |
| Q5.2 | Code Panel | Read-only preview shell |
| Q5.3 | Build Card | Deleted — unified `bash` tool card |
| Q5.4 | Summary Card | Retained, `end_turn` triggers |
| Q5.5 | Fix Loop Banner | Deleted events; frontend visual detection |
| Q5.6 | Thinking Indicator | Repurposed to bash execution waits |
| Q6 | Workspace | **W1** — long-lived per project, lazy-create |
| Q6a | Clone creds | **Phase 1a:** HTTPS+token (temporary). **Phase 1b:** ed25519 SSH deploy keys (hard cutover) |
| Q6b | Resync | `git reset --hard` on new session |
| R2.1 | Verification hooks | `AgentHookRegistry` in-process Python callables, empty default, 4 extension points (pre_turn / pre_tool_call / post_turn / system_prompt_slots) |
| R2.2 | Bidirectional SSE | Redis pub/sub return channel `agent:return:{session_id}`, `ClarificationCoordinator` owns futures, 10-min timeout → session halts |
| R2.3 | Reviewer meta-tool | `request_review` constructs its own `ModelRouter` call with `Purpose.REVIEW`, reviewer prompt lives in `prompts.py` with pinned system prompt + verdict parser |
| R2.4 | Phase 1 split | **Phase 1a** HTTPS+token (unblocks Phase 5); **Phase 1b** SSH deploy keys (gates public deployment) |
| — | Engineering std | **Silicon-valley grade**: no compromises, no debt, no regex-as-security, no AsyncMock fallbacks, one code path |

## Phases (9 phases, Round 2)

| Phase | Name | Tasks | Depends on | Unblocks | Status | File |
|---|---|---|---|---|---|---|
| 0 | Infrastructure & Plumbing | 6 | — | 1a, 2 | ✅ written (Round 1, unchanged) | [phase-0-infrastructure.md](phase-0-infrastructure.md) |
| 1a | Workspace Minimal (Go + HTTPS+Token) | 8 | 0 | 5 | ✅ written (Round 2 NEW) | [phase-1a-workspace-minimal.md](phase-1a-workspace-minimal.md) |
| 1b | Deploy Keys (Go + SSH + GitHub API) | 6 | 1a | public deployment | ✅ written (Round 2 NEW) | [phase-1b-deploy-keys.md](phase-1b-deploy-keys.md) |
| 2 | BaseTool Refactor + WorkspacePath | 6 | 0 | 3, 4 | ✅ written (Round 1, unchanged) | [phase-2-basetool.md](phase-2-basetool.md) |
| 3 | T2 File Tools | 8 | 2 | 5 | ✅ written (Round 1, unchanged) | [phase-3-file-tools.md](phase-3-file-tools.md) |
| 4 | Bash + SetPhase + Stream Events | 9 | 2 | 5, 5a | ✅ written (Round 1 + Round 2 Task 4.9 for `ClarificationRequested`) | [phase-4-bash-events.md](phase-4-bash-events.md) |
| 5a | Bidirectional RPC (Redis Pub/Sub Return Channel) | 9 | 0, 4 | 5 | ✅ written (Round 2 NEW) | [phase-5a-bidirectional-rpc.md](phase-5a-bidirectional-rpc.md) |
| 5 | Agent Loop + api_server + Prompts + Interaction Tools | 15 | 1a, 3, 4, 5a | 6 | ✅ written (Round 1 Tasks 5.1–5.7 + Round 2 Tasks 5.8–5.15 for hooks, clarification tool, review tool) | [phase-5-agent-loop.md](phase-5-agent-loop.md) |
| 6 | Frontend Changes + Clarification UI | 11 | 5 | 7 | ✅ written (Round 1 Tasks 6.1–6.9 + Round 2 Tasks 6.10–6.11 for clarification input component) | [phase-6-frontend.md](phase-6-frontend.md) |
| 7 | E2E Smoke (with clarification round-trip) + Deploy | 4 | 6, 5a | production deployment | ✅ written (Round 1 + Round 2 smoke test rewrite) | [phase-7-deploy.md](phase-7-deploy.md) |

**Total:** ~76 tasks across 9 phases (6 + 8 + 6 + 6 + 8 + 9 + 9 + 15 + 11 + 4 = 82 if you count the 1a/1b split as two phases; ~76 if you count the 6-tasks-added-in-Round-2 delta from the Round 1 baseline of 58).

**Hard rule:** phases execute respecting the dependency graph (below). Each phase's tests must be green before its downstream phases begin. A2 is a breaking refactor — partial states are worse than useless. Phase 1b is the one exception: it can run in parallel with Phases 2–7 because no later phase depends on auth mechanism.

**Commit frequency:** one commit per task minimum. Every task ends with a HEREDOC git commit command.

## Phase dependency graph

```
          Phase 0 (Infra)
             │
    ┌────────┼────────┬────────────┐
    │        │        │            │
  Phase 1a Phase 2   │          (later,
 (Workspace│        │           parallel)
  minimal) │        │              │
    │      │        │          Phase 1b
    │    ┌─┴──┐     │         (deploy keys)
    │  Phase 3 Phase 4         │
    │ (Files) (Bash/Events)    │ (gates public deploy;
    │    │     │               │  not needed for later
    │    │  Phase 5a            │  phases to proceed)
    │    │  (Bidi RPC)         │
    │    │     │               │
    └────┴─────┴──┐            │
                  │            │
               Phase 5         │
               (Agent loop     │
               + hooks +       │
               interaction     │
               tools)          │
                  │            │
               Phase 6         │
               (Frontend +     │
               clarification   │
               UI)             │
                  │            │
               Phase 7 ────────┘
               (E2E smoke
                + deploy)
```

- **Phase 3 and Phase 4** both depend on Phase 2 and are otherwise independent — they CAN run in parallel if multiple workers split the job. Single-agent execution runs them sequentially.
- **Phase 5a (Bidirectional RPC)** depends on Phase 0 (Redis infrastructure) and Phase 4 (Task 4.9 `ClarificationRequested` event) and is otherwise independent. Phase 5 Tasks 5.10–5.11 (`RequestClarificationTool` + `register_interaction_tools`) depend on Phase 5a's `ClarificationCoordinator` being available, so Phase 5 cannot finish before Phase 5a does.
- **Phase 1b (Deploy Keys)** is the off-critical-path exception. It depends on Phase 1a but NOT on any later phase, and Phases 2–7 do NOT depend on Phase 1b. Phase 1b can be scheduled in parallel with Phases 2–7 or after Phase 5, as long as it lands before public production deployment (§2.9.4.d).

## Completion gates

Each phase has a "completion check" section at the bottom of its file listing the gates that must be green before the next phase starts. Do not advance prematurely.

Round 2 adds these cross-cutting gates that span multiple phases:

- Bidirectional RPC: `pytest ai-worker/tests/openharness/engine/test_return_channel_*.py` passes against real Redis (docker-compose dev), plus the adversarial suite (9 tests) from Phase 5a Task 5a.8
- Agent hooks: `pytest ai-worker/tests/openharness/engine/test_hooks_integration.py` passes with 9+ integration scenarios from Phase 5 Task 5.10
- Reviewer tool: `pytest ai-worker/tests/openharness/tools/test_request_review_tool.py` passes with mocked `ModelRouter.generate` + real `git diff` fixture from Phase 5 Task 5.13
- E2E clarification round-trip: `pytest ai-worker/tests/e2e/test_variant_b_smoke.py -m e2e` with `FORGE_E2E_ENABLED=1` completes successfully, asserting both the Round 1 shape invariants AND the Round 2 clarification round-trip closure

## Execution modes

Two supported execution modes after plan approval:

1. **Subagent-Driven (recommended)** — dispatch a fresh subagent per task, review between tasks (see `superpowers:subagent-driven-development`). Well-suited to Round 2 because the dependency graph has parallelism (Phase 1b // Phases 2–7; Phase 3 // Phase 4).
2. **Inline Execution** — execute tasks in the current session with checkpoints (see `superpowers:executing-plans`). Simpler, single-threaded, no parallelism.

## Cross-references

- **Design spec:** [`../../specs/2026-04-09-agent-variant-b-single-agent-design.md`](../../specs/2026-04-09-agent-variant-b-single-agent-design.md) — the authoritative "why" for every decision. §2.9 is the Round 2 additions.
- **CEO review findings:** `~/.claude/projects/D--shulex-work-forge/memory/chronos-ceo-review-2026-04-09.md` — the 5 findings that triggered Round 2.
- **Silicon Valley standard:** `~/.claude/projects/D--shulex-work-forge/memory/feedback-silicon-valley-infra.md` — the engineering standard.
- **Round 2 handoff doc:** [ROUND-2-HANDOFF.md](ROUND-2-HANDOFF.md) — the session-to-session handoff that set up Round 2. Kept as a historical record.

---

<!-- AUTONOMOUS DECISION LOG -->
## /autoplan Review — Decision Audit Trail (2026-04-10)

**Review mode:** SELECTIVE EXPANSION | **Voices:** Claude subagent only (Codex CLI unavailable)
**Phases run:** CEO → Design → Eng → DX (all 4)

### Auto-Decisions

| # | Phase | Decision | Classification | Principle | Rationale | Rejected |
|---|---|---|---|---|---|---|
| 1 | CEO | Add runtime prerequisite check to Phase 0 | Mechanical | P1 | Disk full, rg missing, bwrap missing not gated | — |
| 2 | CEO | Maintain 82-task scope | Mechanical | P6 | Agent loop is table-stakes; hooks populated in 二期 | Reduce scope |
| 3 | CEO | Maintain Silicon Valley quality bar | Mechanical | P6 | User instruction (Harvey's explicit mandate) | Lower bar |
| 4 | CEO | **TASTE: Clarification timeout UX** | Taste | — | 10-min hard halt vs async-friendly design | See gate |
| 5 | CEO | Note competitive landscape gap for Harvey | Mechanical | P3 | Documentation, not scope change | — |
| 6 | CEO | **TASTE: Dogfood gate before deploy** | Taste | — | PM user test vs premature for architecture phase | See gate |
| 7 | Design | Add clarification hydration on reconnect (Task 6.10) | Mechanical | P1 | Critical: user loses clarification form on tab sleep | — |
| 8 | Design | Add countdown timer during 10-min wait (Task 6.10) | Mechanical | P1 | Critical: user has no visibility into timeout clock | — |
| 9 | Design | **TASTE: humanLabel on tool cards for PMs** | Taste | — | "Reading auth handler" vs raw file paths | See gate |
| 10 | Design | Add elapsed time on running tool cards (Task 6.5) | Mechanical | P1 | Running bash shows no progress for 45s+ commands | — |
| 11 | Design | Add ThinkingIndicator in clarification "submitted" state | Mechanical | P5 | 3 lines of JSX, removes anxiety gap | — |
| 12 | Design | **TASTE: Timeout recovery UX (countdown + new session btn)** | Taste | — | Good UX vs scope creep | See gate |
| 13 | Design | Add responsive test for ClarificationInput at 375px | Mechanical | P1 | Mobile viewport untested | — |
| 14 | Design | Add autoFocus + aria-live on ClarificationInput | Mechanical | P5 | a11y: screen readers and keyboard users | — |
| 15 | Design | **TASTE: Background tab notification for clarification** | Taste | — | Notification API vs permission complexity | See gate |
| 16-24 | Design | Batch polish: fix loop count, phase labels, skeleton, etc. | Mechanical | P3 | 9 medium findings, all valid sub-task notes | — |
| 25 | Eng | Verify Phase 2 Task 2.6 handles async-generator propagation | Mechanical | P5 | Subagent flagged; plan already covers in Task 2.6 | — |
| 26 | Eng | Verify LRU eviction calls engine.close() not clear() | Mechanical | P1 | Plan's Phase 5a Task 5a.9 already addresses | — |
| 27 | Eng | Verify TimeoutError→ClarificationTimeout in Task 5a.4 | Mechanical | P5 | Already in spec §2.9.2.d pinned code | — |
| 28 | Eng | Add task.add_done_callback for fire-and-forget exceptions | Mechanical | P1 | Swallowed exceptions leave sessions silently dead | — |
| 29 | Eng | Verify multi-tool-use clarification blocking test exists | Mechanical | P1 | Spec §7.4 already has this test | — |
| 30 | Eng | Add Redis reconnection failure test to Phase 5a | Mechanical | P1 | Gap in adversarial suite | — |
| 31 | Eng | Add rate limiting to /clarify endpoint (5 req/sess/min) | Mechanical | P1 | DoS vector on Redis via authenticated flooding | — |
| 32 | Eng | Flag Task 2.6 as highest-risk in plan | Mechanical | P5 | Generator-exception interaction is the hinge | — |
| 33-40 | Eng | Batch: SessionContext, hook ordering, response size, pool size, clearenv, error sanitize, prompt rebuild, git config | Mechanical | P3/P5 | 8 medium findings, notes for relevant tasks | — |
| 41 | DX | Verify hook Protocol classes are runtime_checkable | Mechanical | P5 | Extension surface must be typed | — |
| 42 | DX | Pin SessionHaltError message format in tests | Mechanical | P5 | Operator grep-ability requires exact format | — |
| 43 | DX | Add structured log points for clarification lifecycle | Mechanical | P1 | Spec §7.8 already defines them; ensure plan references | — |
| 44-52 | DX | Batch post-chronos: ToolResult enforcement, Any→Protocol, guide, HOOK_FAIL_OPEN, hook priority, error codes, metrics, is_alive, auto-register | Mechanical | P3 | 9 medium findings for TODOS.md | — |

### Taste Decisions for Final Gate

| # | Title | Phase | Recommendation | Alternative |
|---|---|---|---|---|
| 4 | Clarification timeout UX | CEO | Accept 10-min hard halt (configurable via env var) | Async-friendly: agent continues with guess, marks as unverified |
| 6 | Dogfood gate before deploy | CEO | Add PM user test to Phase 7 | Skip: premature for architecture build-out |
| 9 | humanLabel on tool cards | Design | Add human-readable action summaries | Keep raw file paths (developer-focused) |
| 12 | Timeout recovery UX | Design | Add 8-min countdown warning + "start new session" button | Bare "session ended" banner (current spec) |
| 15 | Background tab notification | Design | Use document.title change when clarification pending | Skip: adds permission complexity |

## GSTACK REVIEW REPORT

| Review | Trigger | Why | Runs | Status | Findings |
|--------|---------|-----|------|--------|----------|
| CEO Review | `/plan-ceo-review` | Scope & strategy | 1 | complete | 6 findings (1 CRITICAL, 3 HIGH, 2 MEDIUM) |
| Design Review | `/plan-design-review` | UI/UX gaps | 1 | complete | 20 findings (2 CRITICAL, 8 HIGH, 8 MEDIUM, 2 LOW) |
| Eng Review | `/plan-eng-review` | Architecture & tests | 1 | complete | 16 findings (1 CRITICAL*, 8 HIGH, 7 MEDIUM) |
| DX Review | `/plan-devex-review` | Developer experience | 1 | complete | 12 findings (3 HIGH, 9 MEDIUM) |
| Codex Review | `codex exec` | Independent 2nd opinion | 0 | unavailable | Codex CLI not installed |

*Eng CRITICAL (1.1: _execute_tool_call drops StreamEvents) verified as already addressed in Phase 2 Task 2.6.

**VERDICT:** 4/4 reviews complete [subagent-only]. 52 auto-decisions logged. 5 taste decisions surfaced for user approval. Plan is structurally sound with well-scoped mechanical improvements.
