# chronos -- Delivery Retro

**Date:** 2026-04-10
**Delivery:** Variant B single-agent architecture (A2)
**Plan:** [index.md](index.md)
**Spec:** [../../specs/2026-04-09-agent-variant-b-single-agent-design.md](../../specs/2026-04-09-agent-variant-b-single-agent-design.md)

## What shipped

Seven phases across ~76 tasks. Every phase has a completion-gate checklist and atomic commits -- the plan is resumable from any commit.

**Phase 0** (Infra): bubblewrap + ripgrep in ai-worker image, pathspec dep, deleted pair_pipeline files, 2 DB migrations, AES-GCM secrets service.

**Phase 1a** (Workspace Minimal): state DAO with advisory lock, HTTPS+token git wrapper, prep RPC client, EnsureReady state machine, caller migrations (build_activities, devops_activities, agent/service), main.go wiring.

**Phase 1b** (Deploy Keys): ed25519 deploy keys, GitHub deploy-key upload client, SSH-aware git wrapper, key rotation stub.

**Phase 2** (Tool contract): `BaseTool.execute` refactored to async generator, `SimpleTool` convenience subclass, `WorkspacePath` sandbox type, context_tools migrated, parametrized contract tests, adversarial path suite, query.py adapted in the same phase so the build stayed green.

**Phase 3** (File tools): 6 `SimpleTool` subclasses (read/write/edit/glob/grep/list_directory), `register_file_tools` helper, contract suite extended to 10 tools.

**Phase 4** (Bash + SetPhase + events): `PhaseChanged` added, `tool_use_id` added to tool events, `FixLoop*` deleted, `SetPhaseTool` (first BaseTool subclass yielding StreamEvents), `BashTool` with full bubblewrap sandbox, 13 P0 adversarial tests.

**Phase 5a** (Bidirectional RPC): `ClarificationCoordinator` with asyncio.Future, `ReturnChannelSubscriber` on Redis pub/sub, `ClarificationRequested` event, 10-min timeout -> session halt.

**Phase 5** (Agent loop): `build_system_prompt` (real Variant B prompt), `SessionCollector`, `LRUSessionCache`, `_create_engine` rewrite registering all 14 tools, `_route_and_stream` simplification, `/api/workspace/prep` endpoint, `AgentHookRegistry` with 4 extension points, `RequestClarificationTool`, `RequestReviewTool`.

**Phase 6** (Frontend): BuildCard deleted, pair_pipeline state purged from agent-chat, step-ribbon rewritten for dynamic phases, phase_changed wired end-to-end, tool-formatters updated for 8 new tools, tool-execution respects hideCard, code-panel downgraded to read-only, thinking-indicator relocated, `detectFixLoopStart` visual heuristic, clarification input component.

**Phase 7** (Deploy): Real-LLM e2e smoke test with clarification round-trip, observability log points at 5 boundaries, deploy runbook, retro + project memory.

## What we learned

**Silicon-valley standard enforcement works.** Harvey's explicit rule "no compromises, no debt, no hardcoded special cases" shaped every design decision. The result: zero `if tool_name == 'bash'` branches in the agent loop, zero fallback paths in critical code (bwrap has no fallback, ripgrep has no fallback, ModelRouter has no AsyncMock fallback), contract-as-mechanical-gate on the tool protocol.

**The async-generator BaseTool refactor was the right call.** Originally Phase 2 was going to be 5 tasks with a known build-red period; we added Task 2.6 to adapt `query.py` inside Phase 2 so the build stayed green across all subsequent phases. Slight scope increase per phase, much cleaner handoff between phases.

**Spec-first pays off.** 3 rounds of spec review caught: the bubblewrap `--share-net=false` bug (that flag doesn't exist and would have silently re-enabled network -- a security regression shipped to prod); the `workspace.Manager` collision with the already-wired Go module (would have created two parallel workspace managers); the project.NewService description being factually wrong. All three would have been painful to discover mid-implementation.

**Writing plans in multi-file directory format works.** Old monolithic plan files became unreviewable after Phase 1. Splitting into `docs/plans/chronos-2026-04-09/phase-N-*.md` + `index.md` meant each phase could be reviewed, executed, committed, and rolled back independently.

**Deferred by design, not by accident.** Each phase's completion check calls out what's NOT in scope -- profile data wiring, per-turn cost, dashboards, P3 permissions, dependency install mid-session, code panel diff rendering. Writing these down explicitly stopped scope creep and gave the runbook a clean "known deferred items" section.

## What we'd do differently

**Phase 2 was originally 5 tasks and became 6.** Good call in retrospect (build stayed green) but a signal that rigid phase sizing at plan-writing time doesn't always match execution reality. For future plans: plan a rough task count per phase and allow +/-2 during execution without panicking.

**Adversarial tests for BashTool landed in Phase 4 Task 4.5, not Phase 4 Task 4.3.** This means BashTool itself and its adversarial suite were in different commits. In retrospect, writing BashTool without its adversarial tests meant the tool was "done" but not "safe" for ~1 commit cycle. Future: land security tests in the same commit as the security-critical code they gate.

**The prep endpoint and the prep client landed ~4 phases apart** (Phase 1 Task 1.5 for the Go client, Phase 5 Task 5.7 for the Python endpoint). During execution, an integrator could reasonably think Phase 1 is "done" even though its prep client has no server to call. The plan documentation flags this but the phase separation was dictated by the dependency graph.

## What the next session needs to know

See the session memory file at `~/.claude/projects/D--shulex-work-forge/memory/chronos-delivery-2026-04-09.md` for the canonical list of load-bearing invariants and known deferrals. Short list:

1. Tool protocol is `BaseTool.execute -> AsyncIterator[StreamEvent | ToolResult]`. Use `SimpleTool` unless you need mid-execution events.
2. Every file-operating tool validates paths via `WorkspacePath.resolve`. Never raw `pathlib.Path` join.
3. BashTool runs in bwrap. No network. 100 KB output cap. 600s max timeout. Denylist is a UX filter, not a security boundary.
4. `_create_engine` raises on ModelRouter failure. If you see the agent start with fake responses, it's a bug -- check you didn't re-introduce an AsyncMock fallback somewhere.
5. The 14 tools are registered via 3 helpers: `register_context_tools`, `register_file_tools`, `register_exec_tools`. Keep new tools in the appropriate group.
6. `FixLoopStarted/Completed` are gone. If you need loop visibility, extend `detectFixLoopStart` in `agent-chat.tsx`.
7. `SessionComplete` is conditional: only emitted when `tool_call_count > 0`. Text-only turns don't get a SummaryCard.
