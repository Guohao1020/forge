#!/bin/bash
# Forge Health Check — Verify all services are running correctly
# Usage: bash scripts/health-check.sh

set -euo pipefail

PASS=0
FAIL=0

check() {
    local name=$1
    local cmd=$2
    if eval "$cmd" > /dev/null 2>&1; then
        echo "  OK: $name"
        PASS=$((PASS + 1))
    else
        echo "  FAIL: $name"
        FAIL=$((FAIL + 1))
    fi
}

echo "=== Forge Health Check ==="
echo ""

echo "Infrastructure:"
check "PostgreSQL" "docker exec forge-postgres pg_isready -U forge -q"
check "Redis" "docker exec forge-redis redis-cli -a forge_redis_2026 ping 2>/dev/null | grep -q PONG"
check "Temporal" "docker exec forge-temporal tctl cluster health 2>/dev/null || docker ps --format '{{.Names}}' | grep -q forge-temporal"

echo ""
echo "Services:"
check "forge-core API" "curl -s http://localhost:8080/api/auth/me 2>/dev/null | grep -q '登录\|code'"
check "forge-core login" "curl -s -X POST http://localhost:8080/api/auth/login -H 'Content-Type: application/json' -d '{\"username\":\"admin\",\"password\":\"admin123\"}' 2>/dev/null | grep -q token"

echo ""
echo "Frontend (optional — may not be running):"
if curl -s http://localhost:3000 -o /dev/null -w '%{http_code}' 2>/dev/null | grep -qE '200|307'; then
    echo "  OK: forge-portal"
    PASS=$((PASS + 1))
else
    echo "  SKIP: forge-portal (not running, start with: npm run dev)"
fi

echo ""
echo "=================================="
echo "Health: $PASS OK, $FAIL FAIL"
echo "=================================="

[ $FAIL -eq 0 ] || exit 1
