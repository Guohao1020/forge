<div align="center">

# ⚒️ Forge

### AI-driven Harness Engineering Platform

**Describe a feature in plain language. Get production-grade code — clarified, test-driven, reviewed, and deployed.**

<br/>

[![Go](https://img.shields.io/badge/Go-1.26-00ADD8?logo=go&logoColor=white)](https://go.dev)
[![Python](https://img.shields.io/badge/Python-3.12-3776AB?logo=python&logoColor=white)](https://www.python.org)
[![Next.js](https://img.shields.io/badge/Next.js-16-000000?logo=nextdotjs&logoColor=white)](https://nextjs.org)
[![React](https://img.shields.io/badge/React-19-61DAFB?logo=react&logoColor=black)](https://react.dev)
[![Temporal](https://img.shields.io/badge/Temporal-Workflows-000000?logo=temporal&logoColor=white)](https://temporal.io)
[![PostgreSQL](https://img.shields.io/badge/PostgreSQL-16-4169E1?logo=postgresql&logoColor=white)](https://www.postgresql.org)

[![Tests](https://img.shields.io/badge/tests-745%20passing-3fb950)](#testing)
[![Status](https://img.shields.io/badge/status-production--grade-2563eb)](#roadmap)
[![Architecture](https://img.shields.io/badge/agent-A2%20single--agent%20loop-8957e5)](#the-a2-agent)

**English** · [中文](README.zh-CN.md)

</div>

---

> ### *规范即灵魂* — **Standards are the soul.**
> The AI is only the executor. Decades of distilled engineering standards are what guarantee the code.

Forge lets a product manager who has never written a line of code describe what they want — *"I need a coupon system,"* *"add a notification channel"* — and have an AI agent build it, end to end, inside a **harness**: a constrained environment of versioned standards, mechanical verification, and observable feedback. Not a prototype generator. A production pipeline.

## How it works

It starts the moment someone types a sentence.

The agent **doesn't** start writing code. First it steps back and asks what you're really trying to do — surfacing ambiguity, flagging technical risk, and resolving unknowns *before* a single file is touched. When more than one reasonable approach exists, it doesn't guess: it lays out the options as structured cards — trade-offs, risks, and a recommendation grounded in your project's own context — and lets you pick.

Once the requirement is locked, a **single Cursor-style agent** takes over and runs one continuous tool-use loop. It writes the tests first. It reads your existing code with the same file tools a senior engineer would reach for — glob, grep, read, edit. It queries your project's living profile — the API catalog, the database schema, the module graph, the business rules — so the code it generates fits the architecture instead of fighting it. It runs commands inside a locked-down sandbox, watches them fail, fixes itself, and runs them again. Every step is announced through a `set_phase` tool, so the UI shows a live ribbon of *exactly* where the agent is: clarifying → test-first → generating → reviewing → testing → deploying.

Every standard in the **Specs Center** — Java, SQL, Redis, naming, security — is mechanically injected into the agent's context on every run. The agent can't *forget* a convention, because the convention is in the prompt, not in someone's memory.

When the work is done, low-risk changes flow straight through review, merge, and deploy. High-risk ones stop and wait for a human. Either way, every token the agent emitted is streamed live and durably recorded — so you can watch it think, and replay it later.

That's the core of the system. Standards constrain it, verification proves it, observability shows it.

## Architecture

```
                       Users  (Web · IM · CLI)
                              │
                  Traefik  (TLS · JWT · rate-limit · canary)   ← prod gateway
                              │
        ┌─────────────────────┴─────────────────────┐
        │            forge-core  (Go :8080)          │   Modular monolith, 18 modules
        │  auth · project · task · specs · pipeline  │   ~99 endpoints / 22 groups
        │  profile · version · cost · entropy · …     │
        └─────────────────────┬─────────────────────┘
                              │
            ┌─────────────────┼──────────────────────┐
            ▼                 ▼                        ▼
   ai-worker (Py :8090)   Temporal (:7233)     forge-bot (Go :8085)
   ── A2 single agent ──   stateful workflows   DingTalk · 6 cards
   • QueryEngine loop      (versions, entropy
     (≤25 tool rounds)      scans, orchestration)
   • 15-tool toolbelt
   • bwrap sandbox  (no-net, namespace-isolated)
   • LRU session cache + hooks + permission guard
   • dual-storage events → Redis Streams + Postgres
   • ModelRouter: qwen3 → Claude → GPT → DeepSeek
                 (circuit-breaker fallback)
            │
            ▼
   forge-portal (Next.js :3000)        Observability
   29+ pages · 87 components           Grafana · Prometheus · Loki · Promtail
```

> **AI replaces middleware.** Claude/qwen stand in for SonarQube (code quality), MeterSphere (test platform), and Elasticsearch (search). See the [Technical Design](docs/technical-design.md) §13.6.

## The A2 Agent

The heart of Forge is a single autonomous agent — not a rigid pipeline of hand-offs. (The earlier six-agent pipeline, *pair_pipeline*, has been retired.)

| Capability | What it means |
|------------|---------------|
| **Single-agent tool-use loop** | One `QueryEngine` session drives the whole job through up to 25 rounds of reasoning + tool calls, Cursor-style. |
| **15-tool toolbelt** | 6 file tools (`read` · `write` · `edit` · `glob` · `grep` · `ls`), sandboxed `bash`, `set_phase`, 5 project-context query tools, 2 human-in-the-loop tools (`clarify` · `request_review`). |
| **bwrap sandbox** | Every shell command runs inside Bubblewrap with `--unshare-all` — no network, the workspace as the *only* read-write mount, env allow-list, output caps, timeouts, and process-group kill. |
| **Live + durable events** | Each event is dual-written to Redis Streams (hot SSE buffer) **and** Postgres `agent_messages` (replayable history). Sessions rehydrate from PG, then subscribe to Redis. |
| **Self-healing model routing** | `ModelRouter` falls qwen3 → Claude → GPT → DeepSeek with a circuit breaker (3 failures → open, 30s window, 60s recovery). |
| **Guard rails** | Per-call permission checks + a hook registry wrap every tool invocation; an `LRUSessionCache` bounds memory. |

## Quick Start

```bash
# Prerequisites: Docker, Go 1.26+, Python 3.12+, Node.js 20+

# One command: infra → migrations → ai-worker image → forge-core → health checks
bash scripts/dev-deploy.sh

# Frontend (separate terminal)
cd forge-portal && npm run dev
```

Then open the workbench:

| | URL | Credentials |
|---|-----|-------------|
| **Web UI** | http://localhost:3000 | `admin` / `admin123` |
| API | http://localhost:8080 | — |
| Health | http://localhost:8080/health | — |
| Grafana | http://localhost:3001 | `admin` / `forge_grafana_2026` |
| Temporal UI | http://localhost:8233 | — |

<details>
<summary>Manual steps (if the script fails)</summary>

```bash
# 1. Infrastructure
docker compose -f docker-compose.dev.yml up -d postgres redis temporal

# 2. Migrations (idempotent)
docker compose -f docker-compose.dev.yml exec -T postgres \
  psql -U forge -d forge_main < forge-core/migrations/025_workspaces.sql

# 3. Rebuild + restart ai-worker (code lives in the image, not a volume)
docker compose -f docker-compose.dev.yml build --no-cache ai-worker
docker compose -f docker-compose.dev.yml up -d --force-recreate ai-worker

# 4. forge-core
cd forge-core && go build ./cmd/forge-core
FORGE_SECRETS_MASTER_KEY=$(python -c "import base64,os; print(base64.b64encode(os.urandom(32)).decode())") ./forge-core &
```

See [CLAUDE.md](CLAUDE.md) for gotchas (master-key rotation, workspace soft-fail).
</details>

## Services

| Service | Stack | Port | Description |
|---------|-------|------|-------------|
| **forge-core** | Go · Gin · PostgreSQL | 8080 | Unified API — ~99 endpoints across 22 resource groups, 18 modules |
| **ai-worker** | Python · FastAPI · LangGraph · Temporal | 8090 | A2 single agent (QueryEngine + 15 tools + bwrap sandbox) |
| **forge-portal** | Next.js 16 · React 19 · shadcn/ui · Tailwind 4 | 3000 | Web workbench — 29+ pages, 87 components |
| **forge-bot** | Go · Gin | 8085 | IM bot — DingTalk webhook, 6 card templates |
| **Temporal** | — | 7233 | Stateful workflow engine (UI :8233) |
| **Grafana** | — | 3001 | 3 dashboards (health, AI perf, tasks) |
| **Prometheus** | — | 9090 | Metrics collection |
| **Loki + Promtail** | — | 3100 | Log aggregation |

## Philosophy

Forge is built on **Harness Engineering** — the discipline (originated at OpenAI) of designing the *environment* an AI agent runs in so it stays reliable at scale. Three pillars:

- **Context Engineering** — an agent can only see what's in the repo. Standards, project profiles, and prompt templates are versioned and mechanically injected, never left to memory.
- **Architectural Constraints** — conventions are *enforced*, not documented. Linters, structural tests, AI review, and quality gates make the wrong thing hard to do.
- **Entropy Management** — AI faithfully copies patterns, including bad ones. Continuous quality scans, auto-fixes, and trend tracking fight the rot.

And four working principles:

- **Standards are the soul** — the agent obeys the Specs Center, always.
- **Test-first** — tests are written before code, in the target project's native framework.
- **Risk up front** — ambiguity and technical risk are resolved during requirements, not in production.
- **Evidence over claims** — nothing is "done" until mechanical verification says so.

## Testing

```bash
make test          # full suite: Go + Python + TypeScript + ESLint
make test-go       # 404 Go tests (390 core + 14 bot), 18 packages
make test-python   # 341 Python tests across 47 files
make bench         # 22 Go benchmarks
make smoke-test    # API endpoint smoke test
make coverage      # Go coverage report
```

## Documentation

| Document | Description |
|----------|-------------|
| [PRD](docs/PRD.md) | Product requirements — vision, 20 functional modules, business rules |
| [Technical Design](docs/technical-design.md) | Architecture, Harness Engineering, data models, three-phase plan |
| [Product Design](docs/product-design.md) | UI/UX specs, page designs, the "Dense Engineering" visual system |
| [Milestone Plan](docs/milestone-plan.md) | Phase 1–3 delivery roadmap |
| [Harness Design](docs/plans/harness-engineering-design.md) | L1/L2/L3 architecture (ContextCache, Tools, Orchestrator) |
| [Coding Standards](docs/references/coding-standards.md) | The standards baseline injected into the agent |
| [CHANGELOG](CHANGELOG.md) · [CONTRIBUTING](CONTRIBUTING.md) · [CLAUDE.md](CLAUDE.md) | Releases · workflow · developer guide |

## Roadmap

Forge is delivered as a **production-grade enterprise system** in three engineering phases — not an MVP. Phasing controls scope and cadence, never quality.

- **Phase 1 — Foundation & core engine** ✅ — infrastructure, the six Harness components, the full AI engine, external adapters.
- **Phase 2 — Constraint loop & enterprise capabilities** ✅ — constraint engine, entropy management, complete auth, cost control, IM bot.
- **Phase 3 — Observability loop & operational maturity** ✅ — full-stack monitoring, runtime feedback, gray release, quality dashboards.
- **A2 — Single-agent rewrite** ✅ — Cursor-style agent + unified tool-use loop, bwrap sandbox, SSH deploy keys, live/durable session collection. *(pair_pipeline retired.)*

## Platform Stats

```
Backend API:      ~99 endpoints · 22 resource groups · 18 modules
Go Tests:         404 (390 core + 14 bot) + 22 benchmarks
Python Tests:     341 across 47 files
Frontend:         29+ pages · 87 components
Agent Tools:      15  (6 file · bash · set_phase · 5 context · 2 interaction)
Migrations:       26
Docker Services:  10  (infra + observability)
Release Tags:     46  (latest v1.1.3)
```

<div align="center">
<br/>

**⚒️ Forge** — *规范即灵魂*

</div>
