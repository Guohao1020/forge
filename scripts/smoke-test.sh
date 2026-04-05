#!/usr/bin/env bash
# Smoke test — verifies core API endpoints are responsive.
# Usage: bash scripts/smoke-test.sh [BASE_URL]
#
# Requires: curl, jq
# Default BASE_URL: http://localhost:8080

set -euo pipefail

BASE="${1:-http://localhost:8080}"
PASS=0
FAIL=0
TOKEN=""

pass() { echo "  ✓ $1"; PASS=$((PASS + 1)); }
fail() { echo "  ✗ $1: $2"; FAIL=$((FAIL + 1)); }

check() {
  local name="$1" method="$2" path="$3" expected_status="$4"
  local headers=(-H "Content-Type: application/json")
  if [ -n "$TOKEN" ]; then
    headers+=(-H "Authorization: Bearer $TOKEN")
  fi

  local status
  status=$(curl -s -o /dev/null -w "%{http_code}" -X "$method" "${headers[@]}" "$BASE$path" 2>/dev/null || echo "000")

  if [ "$status" = "$expected_status" ]; then
    pass "$name (${status})"
  else
    fail "$name" "expected ${expected_status}, got ${status}"
  fi
}

check_json() {
  local name="$1" method="$2" path="$3" body="$4" expected_status="$5"
  local headers=(-H "Content-Type: application/json")
  if [ -n "$TOKEN" ]; then
    headers+=(-H "Authorization: Bearer $TOKEN")
  fi

  local status
  status=$(curl -s -o /dev/null -w "%{http_code}" -X "$method" "${headers[@]}" -d "$body" "$BASE$path" 2>/dev/null || echo "000")

  if [ "$status" = "$expected_status" ]; then
    pass "$name (${status})"
  else
    fail "$name" "expected ${expected_status}, got ${status}"
  fi
}

echo "=== Forge Smoke Test ==="
echo "Target: $BASE"
echo ""

echo "--- Public Endpoints ---"
check "Health check" GET "/health" 200
check "Prometheus metrics" GET "/metrics" 200
check "System info" GET "/api/system/info" 200

echo ""
echo "--- Auth ---"
# Login to get token
RESPONSE=$(curl -s -X POST -H "Content-Type: application/json" \
  -d '{"username":"admin","password":"admin123"}' \
  "$BASE/api/auth/login" 2>/dev/null || echo '{}')

TOKEN=$(echo "$RESPONSE" | jq -r '.data.token // empty' 2>/dev/null || echo "")
if [ -n "$TOKEN" ]; then
  pass "Login (got token)"
else
  fail "Login" "no token returned"
  echo ""
  echo "=== Cannot continue without auth token ==="
  echo "Results: $PASS passed, $FAIL failed"
  exit 1
fi

check "Auth me" GET "/api/auth/me" 200

echo ""
echo "--- Projects ---"
check "List projects" GET "/api/projects" 200
check "Search" GET "/api/search?q=test" 200
check "Activity feed" GET "/api/activity" 200

echo ""
echo "--- Settings ---"
check "List settings" GET "/api/settings" 200
check "Admin metrics" GET "/api/admin/metrics" 200
check "Admin users" GET "/api/admin/users" 200
check "Admin costs" GET "/api/admin/costs" 200
check "Admin budget" GET "/api/admin/budget" 200

echo ""
echo "--- Specs ---"
check "List standards" GET "/api/specs/standards" 200
check "List prompts" GET "/api/specs/prompts" 200
check "List rules" GET "/api/specs/rules" 200

echo ""
echo "=== Results: $PASS passed, $FAIL failed ==="

if [ "$FAIL" -gt 0 ]; then
  exit 1
fi
