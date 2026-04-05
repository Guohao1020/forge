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
check "forge-core health" "curl -sf http://localhost:8080/health"
check "forge-core metrics" "curl -sf http://localhost:8080/metrics | grep -q forge_http"
check "forge-core system" "curl -sf http://localhost:8080/api/system/info | grep -q version"
check "forge-core login" "curl -s -X POST http://localhost:8080/api/auth/login -H 'Content-Type: application/json' -d '{\"username\":\"admin\",\"password\":\"admin123\"}' 2>/dev/null | grep -q token"

echo ""
echo "Observability (optional):"
check "Grafana" "curl -sf http://localhost:3001/api/health 2>/dev/null"
check "Prometheus" "curl -sf http://localhost:9090/-/ready 2>/dev/null"

echo ""
echo "Frontend (optional):"
if curl -s http://localhost:3000 -o /dev/null -w '%{http_code}' 2>/dev/null | grep -qE '200|307'; then
    echo "  OK: forge-portal"
    PASS=$((PASS + 1))
else
    echo "  SKIP: forge-portal (start with: cd forge-portal && npm run dev)"
fi

echo ""
echo "IM Bot (optional):"
if curl -sf http://localhost:8085/health 2>/dev/null | grep -q ok; then
    echo "  OK: forge-bot"
    PASS=$((PASS + 1))
else
    echo "  SKIP: forge-bot (start with: cd forge-bot && go run ./cmd/forge-bot)"
fi

echo ""
echo "=================================="
echo "Health: $PASS OK, $FAIL FAIL"
echo "=================================="

[ $FAIL -eq 0 ] || exit 1
