# Forge Platform

**AI-driven Harness Engineering Platform** — Non-technical users describe requirements in natural language, AI generates production-grade code through a constrained 7-stage pipeline with automated testing, review, and deployment.

> **"规范即灵魂"** — AI is the executor; engineering standards are the soul.

## Architecture

```
Users (Web / IM / CLI)
    → forge-portal (Next.js :3000)
    → forge-core (Go API :8080)
    → Temporal Server (:7233)
    → AI Worker (Python, LangGraph)
        ├── ContextCache (Redis)
        ├── Agent Loop (5-round tool use)
        ├── 5 Context Tools (on-demand knowledge)
        └── 6 AI Agents (Analyst → Planner → TestWriter → Coder → Reviewer → Profiler)
    → LLM Providers (DashScope / Claude / GPT / DeepSeek)
```

## Services

| Service | Stack | Port | Description |
|---------|-------|------|-------------|
| forge-core | Go + Gin + PostgreSQL | 8080 | Unified API (~97 endpoints, 22 resource groups) |
| ai-worker | Python + LangGraph + Temporal | — | AI agents (analyze, plan, test, generate, review, profile) |
| forge-portal | Next.js 15 + shadcn/ui + Tailwind | 3000 | Web workbench (29 pages, 64 components) |
| forge-bot | Go + Gin | 8085 | IM bot (DingTalk webhook, 6 card templates) |
| Prometheus | — | 9090 | Metrics collection |
| Grafana | — | 3001 | 3 dashboards (health, AI perf, tasks) |
| Loki + Promtail | — | 3100 | Log aggregation |

## Quick Start

```bash
# Prerequisites: Docker, Go 1.26+, Python 3.12+, Node.js 20+

# Start infrastructure (PostgreSQL, Redis, Temporal, Grafana, Prometheus, Loki)
docker compose -f docker-compose.dev.yml up -d

# Start services
cd forge-core && go run ./cmd/forge-core    # API :8080
cd forge-portal && npm run dev              # Frontend :3000
cd forge-bot && go run ./cmd/forge-bot      # IM bot :8085

# Run tests
make test                   # 155 Go + 120 Python + TypeScript + ESLint
make smoke-test             # 16 API endpoint smoke test
make bench                  # 12 Go benchmarks

# Login: admin / admin123
# Web UI: http://localhost:3000
# API: http://localhost:8080
# Grafana: http://localhost:3001 (admin / forge_grafana_2026)
# Temporal UI: http://localhost:8233
# Health: http://localhost:8080/health
```

## Key Features

- **7-Stage AI Pipeline**: Requirements → Task Split → Test-First → Code Gen → Review → Testing → Deployment
- **Harness Engineering**: ContextCache + Agent Loop + Context Tools (learn-claude-code patterns)
- **Version Management**: Multi-requirement parallel development with 3-layer conflict detection
- **Project Type Detection**: Auto-detect Web/Mobile/Desktop/API/Library + branch strategy
- **AI Recommendations**: Structured option cards with pros/cons/risk when multiple approaches exist
- **Specs Center**: Versioned coding standards mechanically injected into every AI generation
- **Multi-Model Routing**: DashScope → Claude → GPT → DeepSeek with circuit breaker fallback

## Testing

```bash
make test          # 155 Go + 120 Python + TypeScript + ESLint
make test-go       # Go unit tests (155 tests, 18 packages)
make test-python   # Python tests with coverage (120+ tests)
make bench         # 12 Go benchmarks
make smoke-test    # 16 API endpoint checks
make coverage      # Go coverage report
```

## Documentation

| Document | Description |
|----------|-------------|
| [PRD](docs/PRD.md) (v6.0) | Product requirements — 20 functional modules |
| [Technical Design](docs/technical-design.md) (v3.0) | Architecture, Harness Engineering, data models |
| [Product Design](docs/product-design.md) (v3.0) | UI/UX specs, page designs, visual system |
| [Milestone Plan](docs/milestone-plan.md) (v6.0) | Phase 1-3 roadmap, 40+ execution plans |
| [Harness Design](docs/plans/harness-engineering-design.md) | L1/L2/L3 architecture (ContextCache, Tools, Orchestrator) |
| [CHANGELOG](CHANGELOG.md) | Release notes |
| [CONTRIBUTING](CONTRIBUTING.md) | Development workflow |
| [STATUS](STATUS.md) | Current system state |

## Releases

| Version | Date | Description |
|---------|------|-------------|
| v0.4.2 | 2026-04-05 | Phase 3 Complete + Platform Hardening (155 tests, 97 endpoints) |
| v0.3.0 | 2026-04-05 | Phase 3: RBAC + Metrics + Cost + Constraint Engine |
| v0.2.0 | 2026-04-05 | Phase 2: Harness Engineering (ContextCache, Agent Loop, Tools) |
| v0.1.0 | 2026-04-02 | Phase 1: Minimum Closed Loop |

## Platform Stats

```
API Endpoints:    ~97 across 22 resource groups
Go Tests:         155 (0 failures) + 12 benchmarks
Frontend:         29 pages, 64 components
Middleware:       11 layers (security, rate limit, timeout, metrics, etc.)
Docker Services:  10 (infra + observability)
Migrations:       21
Release Tags:     12
```
