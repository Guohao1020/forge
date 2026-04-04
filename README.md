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
| forge-core | Go + Gin + PostgreSQL | 8080 | Unified API server (auth, projects, tasks, versions, specs, pipeline) |
| ai-worker | Python + LangGraph + Temporal | — | AI agents (analyze, plan, test, generate, review, profile scan) |
| forge-portal | Next.js 15 + shadcn/ui + Tailwind | 3000 | Web workbench ("Deep Space Command Center" dark theme) |

## Quick Start

```bash
# Prerequisites: Docker, Go 1.26+, Python 3.9+, Node.js 20+

# Option 1: Full stack via Docker
make deploy

# Option 2: Local development
make dev                    # Start infrastructure + all services
make test                   # Run 190 tests + coverage
bash scripts/health-check.sh  # Verify everything is healthy

# Login: admin / admin123
# Web UI: http://localhost:3000
# API: http://localhost:8080
# Temporal UI: http://localhost:8233
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
make test          # All 190 tests (76 Go + 103 Python + 11 API)
make test-go       # Go unit tests only
make test-python   # Python unit tests with coverage
make test-api      # API integration tests (needs forge-core running)
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
| v0.2.0 | 2026-04-05 | Phase 2: Harness Engineering (50 commits, 190 tests) |
| v0.1.0 | 2026-04-02 | Phase 1: Minimum Closed Loop (57 tasks, 7 slices) |
