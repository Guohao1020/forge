#!/bin/bash
# Forge API Integration Test Script
# Requires: forge-core running on :8080, valid admin credentials
#
# Usage: bash scripts/test-api.sh

set -euo pipefail

BASE_URL="http://localhost:8080"
PASS=0
FAIL=0

# Login
TOKEN=$(curl -s -X POST "$BASE_URL/api/auth/login" \
  -H "Content-Type: application/json" \
  -d '{"username":"admin","password":"admin123"}' | python3 -c "import sys,json; print(json.load(sys.stdin)['data']['token'])")

if [ -z "$TOKEN" ]; then
  echo "FATAL: Login failed"
  exit 1
fi
echo "Logged in as admin"

# Helper: run a test
run_test() {
  local name=$1
  local expected=$2
  local actual=$3

  if echo "$actual" | grep -q "$expected"; then
    echo "  PASS: $name"
    PASS=$((PASS + 1))
  else
    echo "  FAIL: $name (expected '$expected' in response)"
    echo "    Got: $actual"
    FAIL=$((FAIL + 1))
  fi
}

auth() {
  echo "Authorization: Bearer $TOKEN"
}

echo ""
echo "=== Auth ==="
RESP=$(curl -s "$BASE_URL/api/auth/me" -H "$(auth)")
run_test "GET /auth/me" "admin" "$RESP"

echo ""
echo "=== Projects ==="
RESP=$(curl -s "$BASE_URL/api/projects" -H "$(auth)")
run_test "GET /projects" "projects" "$RESP"

echo ""
echo "=== Versions ==="
RESP=$(curl -s -X POST "$BASE_URL/api/projects/25/versions" \
  -H "$(auth)" -H "Content-Type: application/json" \
  -d "{\"version\":\"v99.0.$(date +%s)\",\"description\":\"test\"}")
run_test "POST /versions (create)" "PLANNING" "$RESP"

RESP=$(curl -s "$BASE_URL/api/projects/25/versions" -H "$(auth)")
run_test "GET /versions (list)" "versions" "$RESP"

RESP=$(curl -s -X POST "$BASE_URL/api/projects/25/versions" \
  -H "$(auth)" -H "Content-Type: application/json" \
  -d '{"version":"bad","description":"test"}')
run_test "POST /versions (invalid format)" "invalid version format" "$RESP"

echo ""
echo "=== Tasks ==="
RESP=$(curl -s -X POST "$BASE_URL/api/projects/25/tasks" \
  -H "$(auth)" -H "Content-Type: application/json" \
  -d '{"requirement":"API test task"}')
run_test "POST /tasks (create)" "SUBMITTED" "$RESP"

echo ""
echo "=== Specs ==="
RESP=$(curl -s "$BASE_URL/api/specs/standards" -H "$(auth)")
run_test "GET /specs/standards" "items" "$RESP"

RESP=$(curl -s "$BASE_URL/api/specs/effective/25" -H "$(auth)")
run_test "GET /specs/effective" "standards" "$RESP"

echo ""
echo "=== Profiles ==="
RESP=$(curl -s "$BASE_URL/api/projects/25/profiles" -H "$(auth)")
run_test "GET /profiles" "profiles" "$RESP"

RESP=$(curl -s -X POST "$BASE_URL/api/projects/25/profiles/scan" -H "$(auth)")
run_test "POST /profiles/scan" "scan_started" "$RESP"

echo ""
echo "=== Detection ==="
RESP=$(curl -s -X POST "$BASE_URL/api/projects/25/detect" -H "$(auth)")
run_test "POST /detect" "detection_started" "$RESP"

echo ""
echo "=================================="
echo "Results: $PASS passed, $FAIL failed"
echo "=================================="

if [ $FAIL -gt 0 ]; then
  exit 1
fi
