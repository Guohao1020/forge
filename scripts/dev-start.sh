#!/bin/bash
# Forge Development Environment — Quick Start
#
# Starts all services needed for local development:
# 1. Infrastructure (PostgreSQL+pgvector, Redis, Temporal)
# 2. Go API Server (forge-core)
# 3. Python AI Worker (ai-worker)
# 4. Next.js Frontend (forge-portal)
#
# Prerequisites:
#   - Docker + Docker Compose
#   - Go 1.22+
#   - Python 3.9+ with pip
#   - Node.js 20+ with npm
#   - DashScope API key (or other LLM provider key)
#
# Usage:
#   bash scripts/dev-start.sh          # Start everything
#   bash scripts/dev-start.sh infra    # Start only infrastructure
#   bash scripts/dev-start.sh core     # Start only forge-core
#   bash scripts/dev-start.sh worker   # Start only AI worker
#   bash scripts/dev-start.sh portal   # Start only frontend

set -euo pipefail
cd "$(dirname "$0")/.."
ROOT=$(pwd)

MODE=${1:-all}

start_infra() {
    echo "=== Starting Infrastructure ==="
    docker compose -f docker-compose.dev.yml up -d postgres redis temporal
    echo "Waiting for PostgreSQL to be healthy..."
    docker compose -f docker-compose.dev.yml exec -T postgres pg_isready -U forge -q 2>/dev/null
    echo "Infrastructure ready."
    echo "  PostgreSQL: localhost:5432 (forge/forge_dev_2026)"
    echo "  Redis:      localhost:6379 (password: forge_redis_2026)"
    echo "  Temporal:   localhost:7233"
    echo "  Temporal UI: http://localhost:8233"
}

start_core() {
    echo "=== Building forge-core ==="
    cd "$ROOT/forge-core"
    go build ./cmd/forge-core
    echo "=== Starting forge-core ==="
    ./forge-core.exe &
    sleep 3
    echo "forge-core running on http://localhost:8080"
    echo "  Login: admin / admin123"
}

start_worker() {
    echo "=== Starting AI Worker ==="
    cd "$ROOT/ai-worker"

    # Check for API key
    if [ -z "${DASHSCOPE_API_KEY:-}" ]; then
        DASHSCOPE_API_KEY=$(grep DASHSCOPE_API_KEY .env 2>/dev/null | cut -d= -f2 || echo "")
    fi
    if [ -z "$DASHSCOPE_API_KEY" ]; then
        echo "WARNING: DASHSCOPE_API_KEY not set. AI features will not work."
        echo "Set it via: export DASHSCOPE_API_KEY=sk-..."
    fi

    # Update FORGE_API_TOKEN with fresh login token
    TOKEN=$(curl -s -X POST http://localhost:8080/api/auth/login \
        -H "Content-Type: application/json" \
        -d '{"username":"admin","password":"admin123"}' 2>/dev/null \
        | python3 -c "import sys,json; print(json.load(sys.stdin)['data']['token'])" 2>/dev/null || echo "")
    if [ -n "$TOKEN" ]; then
        sed -i "s|FORGE_API_TOKEN=.*|FORGE_API_TOKEN=$TOKEN|" .env 2>/dev/null || true
        echo "Updated FORGE_API_TOKEN in .env"
    fi

    DASHSCOPE_API_KEY=${DASHSCOPE_API_KEY} python -B -m src.worker &
    sleep 3
    echo "AI Worker running on Temporal queue 'ai-worker'"
}

start_portal() {
    echo "=== Starting Frontend ==="
    cd "$ROOT/forge-portal"
    npm run dev &
    sleep 5
    echo "Frontend running on http://localhost:3000"
}

case $MODE in
    infra)
        start_infra
        ;;
    core)
        start_core
        ;;
    worker)
        start_worker
        ;;
    portal)
        start_portal
        ;;
    all)
        start_infra
        echo ""
        start_core
        echo ""
        start_worker
        echo ""
        start_portal
        echo ""
        echo "=== All services started ==="
        echo ""
        echo "  Web UI:      http://localhost:3000"
        echo "  API:         http://localhost:8080"
        echo "  Temporal UI: http://localhost:8233"
        echo ""
        echo "  Login: admin / admin123"
        echo ""
        echo "To run tests: bash scripts/test-api.sh"
        echo "To stop:      bash scripts/dev-stop.sh"
        ;;
    *)
        echo "Usage: $0 [all|infra|core|worker|portal]"
        exit 1
        ;;
esac
