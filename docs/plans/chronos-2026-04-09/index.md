# chronos — Agent Variant B Single-Agent Implementation

> **Code name:** chronos (Greek personification of time). This refactor "folds time" — collapsing the pair_pipeline-era workaround into the A2-era real agent architecture in one single branch.

**Started:** 2026-04-09
**Status:** ✅ All 7 phases written — ready for execution via `superpowers:subagent-driven-development` or `superpowers:executing-plans`
**Branch:** `feat/agent-variant-b-single-agent` (off `main`)

---

## Goal

Rebuild Forge's AI agent pipeline around a single tool-using agent that can actually drive the Variant B Cursor-style UI, replacing the current `pair_pipeline` workaround that uses regex-extracted code blocks.

## Architecture (A2)

Kill `pair_pipeline.py`. Give the agent a real tool surface (`read` / `write` / `edit` / `glob` / `grep` / `list_directory` / `bash` + `set_phase` meta-tool). Bash runs inside bubblewrap (no network, cwd locked, env whitelist). Workspace is one long-lived clone per project, authenticated via project-level ed25519 SSH deploy keys. `BaseTool` refactors to an async-generator contract so tools can emit mid-execution stream events without hardcoded special cases in the agent loop.

## Tech Stack

Go 1.22 (forge-core workspace module + secrets service), Python 3.12 (ai-worker tools + agent loop), bubblewrap (Linux namespace sandbox), ripgrep, pathspec (Python glob), AES-GCM (Go `crypto/cipher`), ed25519 (Go `crypto/ed25519`), FastAPI, Next.js 16 + React 19 (forge-portal).

## Design Spec

- **[Design spec (2344 lines)](../../specs/2026-04-09-agent-variant-b-single-agent-design.md)** — the authoritative source for every "why" decision in this plan.
- Spec review history: 3 iterations through spec-document-reviewer, approved (commits `849d70a` → `d0828c2` → `351f9e4`).

## Decision chain (Q1–Q6 + silicon-valley standard)

| # | Topic | Decision |
|---|---|---|
| Q1 | Scope | **A** — only agent interaction layer |
| Q2 | Architecture | **A2** — kill pair_pipeline, single agent + tool-use loop |
| Q3 | Tool surface | **T2** — read/write/edit/glob/grep/list_directory/bash |
| Q3a | File paths | Direct workspace ops (no HTTP indirection) |
| Q3b | Bash sandbox | **bubblewrap**, denylist only as UX hint |
| Q4 | Permissions | **P1 → P3**: FULL_AUTO now, P3 interface reserved |
| Q5.1 | Step Ribbon | Dynamic via `set_phase` meta-tool |
| Q5.2 | Code Panel | Read-only preview shell |
| Q5.3 | Build Card | Deleted — unified `bash` tool card |
| Q5.4 | Summary Card | Retained, `end_turn` triggers |
| Q5.5 | Fix Loop Banner | Deleted events; frontend visual detection |
| Q5.6 | Thinking Indicator | Repurposed to bash execution waits |
| Q6 | Workspace | **W1** — long-lived per project, lazy-create |
| Q6a | Clone creds | SSH deploy keys (project-level, ed25519) |
| Q6b | Resync | `git reset --hard` on new session |
| — | Engineering std | **Silicon-valley grade**: no compromises, no debt, no regex-as-security, one code path |

## Phases

| Phase | Name | Tasks | Depends on | Status | File |
|---|---|---|---|---|---|
| 0 | Infrastructure & Plumbing | 6 | — | ✅ written | [phase-0-infrastructure.md](phase-0-infrastructure.md) |
| 1 | Workspace Module (Go) | 13 | 0 | ✅ written | [phase-1-workspace.md](phase-1-workspace.md) |
| 2 | BaseTool Refactor + WorkspacePath | 6 | 0 | ✅ written | [phase-2-basetool.md](phase-2-basetool.md) |
| 3 | T2 File Tools | 8 | 2 | ✅ written | [phase-3-file-tools.md](phase-3-file-tools.md) |
| 4 | Bash + SetPhase + Stream Events | 8 | 2 | ✅ written | [phase-4-bash-events.md](phase-4-bash-events.md) |
| 5 | Agent Loop + api_server + Prompts | 7 | 1, 3, 4 | ✅ written | [phase-5-agent-loop.md](phase-5-agent-loop.md) |
| 6 | Frontend Changes | 9 | 5 | ✅ written | [phase-6-frontend.md](phase-6-frontend.md) |
| 7 | E2E Smoke + Deploy | 4 | 6 | ✅ written | [phase-7-deploy.md](phase-7-deploy.md) |

**Total:** ~58 tasks across 8 phases.

**Hard rule:** phases execute in strict order. Each phase's tests must be green before the next begins. A2 is a breaking refactor — partial states are worse than useless.

**Commit frequency:** one commit per task minimum.

## Phase dependency graph

```
         Phase 0 (Infra)
            │
    ┌───────┼───────┐
    │       │       │
  Phase 1 Phase 2   │
 (Workspace)(BaseTool)
    │       │
    │    ┌──┴──┐
    │  Phase 3 Phase 4
    │ (Files) (Bash/Events)
    │    │     │
    └────┴─────┘
          │
       Phase 5
       (Agent loop)
          │
       Phase 6
       (Frontend)
          │
       Phase 7
       (E2E + Deploy)
```

Phase 3 and Phase 4 both depend on Phase 2 and are otherwise independent — they CAN run in parallel if multiple workers split the job. Single-agent execution runs them sequentially.

## Completion gates

Each phase has a "completion check" section at the bottom of its file listing the gates that must be green before the next phase starts. Do not advance prematurely.

## Execution modes

Two supported execution modes after plan approval:

1. **Subagent-Driven (recommended)** — dispatch a fresh subagent per task, review between tasks (see `superpowers:subagent-driven-development`)
2. **Inline Execution** — execute tasks in the current session with checkpoints (see `superpowers:executing-plans`)
