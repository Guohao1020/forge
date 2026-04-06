#!/usr/bin/env bash
set -e

# Colors
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m'

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"
cd "$PROJECT_ROOT"

# Track PIDs for cleanup
PIDS=()
cleanup() {
    echo -e "\n${YELLOW}Stopping services...${NC}"
    for pid in "${PIDS[@]}"; do
        kill "$pid" 2>/dev/null || true
    done
    echo -e "${GREEN}All services stopped.${NC}"
    exit 0
}
trap cleanup SIGINT SIGTERM

wait_for() {
    local url=$1 name=$2 max=${3:-60}
    local i=0
    while [ $i -lt $max ]; do
        if curl -sf "$url" > /dev/null 2>&1; then
            echo -e "  ${GREEN}✓${NC} $name ready"
            return 0
        fi
        sleep 2
        i=$((i + 2))
    done
    echo -e "  ${RED}✗${NC} $name failed to start (timeout ${max}s)"
    return 1
}

echo -e "${GREEN}=== Forge Platform — Starting All Services ===${NC}"
echo ""

# 1. Infrastructure
echo -e "${YELLOW}[1/5] Starting infrastructure (PostgreSQL, Redis, Temporal)...${NC}"
docker compose -f docker-compose.dev.yml up -d postgres redis temporal temporal-ui 2>&1 | tail -3
wait_for "http://localhost:8233" "Temporal UI" 60

# 2. forge-core
echo -e "${YELLOW}[2/5] Starting forge-core...${NC}"
cd forge-core
go run ./cmd/forge-core > /tmp/forge-core.log 2>&1 &
PIDS+=($!)
cd "$PROJECT_ROOT"
wait_for "http://localhost:8080/health" "forge-core" 30

# 3. ai-worker
echo -e "${YELLOW}[3/5] Starting ai-worker...${NC}"
cd ai-worker
python -m src.worker > /tmp/ai-worker.log 2>&1 &
PIDS+=($!)
cd "$PROJECT_ROOT"
sleep 3
echo -e "  ${GREEN}✓${NC} ai-worker started"

# 4. forge-portal
echo -e "${YELLOW}[4/5] Starting forge-portal...${NC}"
cd forge-portal
npm run dev > /tmp/forge-portal.log 2>&1 &
PIDS+=($!)
cd "$PROJECT_ROOT"
wait_for "http://localhost:3000" "forge-portal" 30

# 5. Summary
echo ""
echo -e "${GREEN}=== All Services Running ===${NC}"
echo -e "  forge-core:    http://localhost:8080"
echo -e "  forge-portal:  http://localhost:3000"
echo -e "  Temporal UI:   http://localhost:8233"
echo -e "  PostgreSQL:    localhost:5432"
echo -e "  Redis:         localhost:6379"
echo ""
echo -e "${YELLOW}Logs:${NC}"
echo -e "  forge-core:   /tmp/forge-core.log"
echo -e "  ai-worker:    /tmp/ai-worker.log"
echo -e "  forge-portal: /tmp/forge-portal.log"
echo ""
echo -e "Press ${RED}Ctrl+C${NC} to stop all services."
echo ""

# Wait forever
wait
