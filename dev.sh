#!/bin/bash
# Forge Dev Startup Script (Linux/macOS)
# Usage: ./dev.sh

set -e

# Colors
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

echo -e "${GREEN}=== Forge Dev Environment ===${NC}"

# 1. Start infrastructure
echo -e "${YELLOW}[1/4] Starting infrastructure (PostgreSQL + Redis + Temporal)...${NC}"
docker compose -f docker-compose.dev.yml up -d
echo "Waiting for Temporal to be ready..."
sleep 10

# 2. Load env
if [ -f .env ]; then
  export $(grep -v '^#' .env | xargs)
fi

# 3. Start forge-core
echo -e "${YELLOW}[2/4] Starting forge-core (Go API Server)...${NC}"
cd forge-core
go build -o ./forge-core ./cmd/forge-core && \
  ./forge-core &
CORE_PID=$!
cd ..
sleep 3

# 4. Start forge-portal
echo -e "${YELLOW}[3/4] Starting forge-portal (Next.js)...${NC}"
cd forge-portal
npm run dev &
PORTAL_PID=$!
cd ..

echo -e "${GREEN}[4/4] All services started!${NC}"
echo ""
echo "  Frontend:     http://localhost:3000"
echo "  API Server:   http://localhost:8080"
echo "  Temporal UI:  http://localhost:8233"
echo "  PostgreSQL:   localhost:5432"
echo "  Redis:        localhost:6379"
echo ""
echo "  Login: admin / admin123"
echo ""
echo "Press Ctrl+C to stop all services."

trap "kill $CORE_PID $PORTAL_PID 2>/dev/null; docker compose -f docker-compose.dev.yml down; exit" INT TERM
wait
