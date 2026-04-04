# Contributing to Forge

## Development Setup

```bash
# Prerequisites: Docker, Go 1.26+, Python 3.9+, Node.js 20+

# 1. Clone and start infrastructure
git clone git@codeup.aliyun.com:68956a307e0dbda9ae2cf005/voc/shulex-forge.git
cd shulex-forge
make dev            # or: bash scripts/dev-start.sh

# 2. Run tests
make test           # All 190 tests + coverage

# 3. Open in browser
open http://localhost:3000   # Login: admin / admin123
```

## Project Structure

```
forge/
├── forge-core/         Go API Server (port 8080)
├── ai-worker/          Python AI Worker (Temporal activities)
├── forge-portal/       Next.js Frontend (port 3000)
├── docker/             Docker configs (task-runner, postgres)
├── docs/               All documentation
│   ├── PRD.md          Product requirements
│   ├── technical-design.md  Architecture
│   ├── product-design.md    UI/UX specs
│   ├── milestone-plan.md    Roadmap
│   └── plans/          Execution plans (20+ docs)
└── scripts/            Dev/test/deploy scripts
```

## Workflow

This project uses **Claude Code Opus** as the primary coding agent, with Harvey as the product owner and decision maker.

### Making Changes

1. Read the relevant execution plan in `docs/plans/`
2. Implement changes following existing patterns
3. Run `make test` before committing
4. Use Conventional Commits: `feat:`, `fix:`, `docs:`, `test:`, `chore:`
5. Push to `main` (trunk-based development)

### Key Conventions

- **Go**: Gin framework, modular monolith, `Result[T]` responses, constructor injection
- **Python**: Temporal activities, LangGraph agents, asyncio, dataclasses
- **Frontend**: Next.js 15 App Router, shadcn/ui, Tailwind CSS 4, Zustand
- **Database**: PostgreSQL multi-schema (auth/engine/specs/pipeline), pgvector
- **Workflow**: Temporal for all async operations

### Running Individual Services

```bash
make dev              # Start everything
bash scripts/dev-start.sh core    # Only forge-core
bash scripts/dev-start.sh worker  # Only AI worker
bash scripts/dev-start.sh portal  # Only frontend
```

### Testing

```bash
make test             # All tests
make test-go          # Go only (76 tests)
make test-python      # Python only (103 tests)
make test-ts          # TypeScript check
make test-api         # API integration (11 tests, needs forge-core running)
```

### Docker

```bash
make docker           # Build all images
make deploy           # Full stack via docker-compose.prod.yml
make deploy-down      # Stop production stack
```
