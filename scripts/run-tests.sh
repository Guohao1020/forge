#!/bin/bash
# Run all Forge tests (unit + integration)
set -euo pipefail
cd "$(dirname "$0")/.."

PASS=0
FAIL=0

echo "=== Go Unit Tests ==="
cd forge-core
if go test ./internal/module/version/... ./internal/temporal/workflow/... -count=1 2>&1 | tail -3; then
    PASS=$((PASS + 1))
else
    FAIL=$((FAIL + 1))
fi

echo ""
echo "=== Go Project Detector Tests ==="
if go test ./internal/module/project/... -run TestDetect -count=1 2>&1 | tail -3; then
    PASS=$((PASS + 1))
else
    FAIL=$((FAIL + 1))
fi

echo ""
echo "=== Go Build ==="
if go build ./cmd/forge-core 2>&1; then
    echo "Build: PASS"
    PASS=$((PASS + 1))
else
    echo "Build: FAIL"
    FAIL=$((FAIL + 1))
fi

cd ..

echo ""
echo "=== Python Unit Tests ==="
cd ai-worker
if python -m pytest tests/ --cov=src --cov-report=term --tb=short -q 2>&1 | tail -10; then
    PASS=$((PASS + 1))
else
    FAIL=$((FAIL + 1))
fi
cd ..

echo ""
echo "=== TypeScript Type Check ==="
cd forge-portal
if npx tsc --noEmit --pretty false 2>&1 | head -3; then
    echo "TypeScript: PASS"
    PASS=$((PASS + 1))
else
    echo "TypeScript: FAIL"
    FAIL=$((FAIL + 1))
fi
cd ..

echo ""
echo "=== API Integration Tests ==="
if curl -s http://localhost:8080/api/auth/me 2>&1 | grep -q "登录"; then
    if bash scripts/test-api.sh 2>&1 | tail -3; then
        PASS=$((PASS + 1))
    else
        FAIL=$((FAIL + 1))
    fi
else
    echo "SKIP: forge-core not running (start with: bash scripts/dev-start.sh core)"
fi

echo ""
echo "=================================="
echo "Test suites: $PASS passed, $FAIL failed"
echo "=================================="
