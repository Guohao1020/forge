#!/usr/bin/env bash
# Forge local dev deployment script
# Usage: bash scripts/dev-deploy.sh
#
# Prerequisites:
#   - Docker Desktop running
#   - DASHSCOPE_API_KEY in ai-worker/.env (qwen models)
#
# What it does:
#   1. Starts PostgreSQL + Redis (if not running)
#   2. Applies database migrations
#   3. Rebuilds ai-worker image (with latest code)
#   4. Restarts ai-worker container
#   5. Builds forge-core binary
#   6. Starts forge-core (background)
#   7. Health checks all services
#
# Ports:
#   forge-core    :8080
#   ai-worker     :8090
#   forge-portal  :3000  (start separately: cd forge-portal && npm run dev)
#   PostgreSQL    :5432
#   Redis         :6379
#   Temporal      :7233 / Web UI :8233

set -euo pipefail
cd "$(git rev-parse --show-toplevel)"

COMPOSE_FILE="docker-compose.dev.yml"

echo "=== Step 1: Start infrastructure ==="
docker compose -f "$COMPOSE_FILE" up -d postgres redis temporal
echo "Waiting for PostgreSQL to be healthy..."
for i in {1..30}; do
  docker compose -f "$COMPOSE_FILE" exec -T postgres pg_isready -U forge >/dev/null 2>&1 && break
  sleep 1
done
docker compose -f "$COMPOSE_FILE" exec -T postgres pg_isready -U forge
echo "Redis:"
docker compose -f "$COMPOSE_FILE" exec -T redis redis-cli -a forge_redis_2026 ping 2>/dev/null | grep -v Warning

echo ""
echo "=== Step 2: Apply database migrations ==="
# Only apply if tables don't exist yet (idempotent)
TABLES=$(docker compose -f "$COMPOSE_FILE" exec -T postgres \
  psql -U forge -d forge_main -t -c "SELECT count(*) FROM information_schema.tables WHERE table_schema='engine' AND table_name IN ('workspaces','project_deploy_keys')" 2>/dev/null | tr -d ' ')

if [ "$TABLES" -lt 2 ]; then
  echo "Applying migrations..."
  for f in forge-core/migrations/025_workspaces.sql forge-core/migrations/026_project_deploy_keys.sql; do
    if [ -f "$f" ]; then
      docker compose -f "$COMPOSE_FILE" exec -T postgres psql -U forge -d forge_main < "$f" 2>/dev/null || true
      echo "  Applied: $f"
    fi
  done
else
  echo "Migrations already applied (workspaces + project_deploy_keys exist)"
fi

echo ""
echo "=== Step 3: Rebuild ai-worker image ==="
docker compose -f "$COMPOSE_FILE" build --no-cache ai-worker

echo ""
echo "=== Step 4: Restart ai-worker ==="
docker compose -f "$COMPOSE_FILE" up -d --force-recreate ai-worker
echo "Waiting for ai-worker to be ready..."
for i in {1..20}; do
  curl -sf http://localhost:8090/health >/dev/null 2>&1 && break
  sleep 1
done

echo ""
echo "=== Step 5: Build forge-core ==="
cd forge-core && go build ./cmd/forge-core && cd ..
echo "forge-core binary built OK"

echo ""
echo "=== Step 6: Start forge-core ==="
# Kill any existing forge-core process
taskkill //F //IM forge-core.exe 2>/dev/null || true
# Generate FORGE_SECRETS_MASTER_KEY if not set
if [ -z "${FORGE_SECRETS_MASTER_KEY:-}" ]; then
  export FORGE_SECRETS_MASTER_KEY=$(python -c "import base64,os; print(base64.b64encode(os.urandom(32)).decode())")
  echo "Generated FORGE_SECRETS_MASTER_KEY (ephemeral, regenerated each deploy)"
fi
cd forge-core
nohup ./forge-core > ../forge-core.log 2>&1 &
FORGE_CORE_PID=$!
cd ..
echo "forge-core started (PID: $FORGE_CORE_PID), log: forge-core.log"
sleep 3

echo ""
echo "=== Step 7: Health checks ==="
echo -n "forge-core :8080 ... "
curl -sf http://localhost:8080/health | python -c "import sys,json; d=json.load(sys.stdin); print(f'status={d[\"status\"]} db={d[\"database\"]} redis={d[\"redis\"]}')" 2>/dev/null || echo "FAIL"

echo -n "ai-worker  :8090 ... "
curl -sf http://localhost:8090/health | python -c "import sys,json; d=json.load(sys.stdin); print(f'status={d[\"status\"]} sessions={d[\"sessions\"]}')" 2>/dev/null || echo "FAIL"

echo -n "PostgreSQL :5432 ... "
docker compose -f "$COMPOSE_FILE" exec -T postgres pg_isready -U forge 2>/dev/null && true || echo "FAIL"

echo -n "Redis      :6379 ... "
docker compose -f "$COMPOSE_FILE" exec -T redis redis-cli -a forge_redis_2026 ping 2>/dev/null | grep -v Warning

echo ""
echo "=== Smoke test ==="
echo -n "ai-worker /api/run ... "
RESULT=$(curl -sf -X POST http://localhost:8090/api/run \
  -H "Content-Type: application/json" \
  -d '{"session_id":"deploy-smoke","project_id":1,"message":"hello"}' 2>&1)
echo "$RESULT" | python -c "import sys,json; d=json.load(sys.stdin); print(f'status={d[\"status\"]}')" 2>/dev/null || echo "FAIL: $RESULT"

echo -n "ai-worker /api/workspace/prep ... "
curl -sf -X POST http://localhost:8090/api/workspace/prep \
  -H "Content-Type: application/json" \
  -d '{"tenant_id":1,"project_id":1,"workspace_path":"nonexistent"}' 2>/dev/null | \
  python -c "import sys,json; d=json.load(sys.stdin); print(f'status={d[\"status\"]}')" || echo "FAIL"

echo ""
echo "════════════════════════════════════════════"
echo "  Deploy complete!"
echo "  "
echo "  Frontend: cd forge-portal && npm run dev"
echo "  Logs:     tail -f forge-core.log"
echo "  Logs:     docker compose -f $COMPOSE_FILE logs -f ai-worker"
echo "  Stop:     taskkill //F //IM forge-core.exe"
echo "  Stop:     docker compose -f $COMPOSE_FILE down"
echo "════════════════════════════════════════════"
