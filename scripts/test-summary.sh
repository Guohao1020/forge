#!/usr/bin/env bash
# test-summary.sh — Generate a comprehensive test report
# Usage: bash scripts/test-summary.sh

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
ROOT_DIR="$(dirname "$SCRIPT_DIR")"

echo "=========================================="
echo "  Forge Platform — Test Summary Report"
echo "=========================================="
echo ""

# Go tests
echo "--- Go (forge-core) ---"
cd "$ROOT_DIR/forge-core"
GO_CORE=$(go test ./internal/... -count=1 -v 2>&1 | grep -cE '^--- PASS' || echo 0)
GO_PKGS=$(go test ./internal/... -count=1 2>&1 | grep -c '^ok' || echo 0)
GO_FAIL=$(go test ./internal/... -count=1 2>&1 | grep -c 'FAIL$' || echo 0)
echo "  Tests:    $GO_CORE"
echo "  Packages: $GO_PKGS"
echo "  Failures: $GO_FAIL"

echo ""
echo "--- Go (forge-bot) ---"
cd "$ROOT_DIR/forge-bot"
GO_BOT=$(go test ./... -count=1 -v 2>&1 | grep -cE '^--- PASS' || echo 0)
echo "  Tests:    $GO_BOT"

echo ""
echo "--- Python (ai-worker) ---"
cd "$ROOT_DIR/ai-worker"
PY_COUNT=$(python -m pytest tests/ --tb=no -q 2>&1 | grep -oP '\d+ passed' | grep -oP '\d+' || echo 0)
echo "  Tests:    $PY_COUNT"

echo ""
echo "--- Go Benchmarks ---"
cd "$ROOT_DIR/forge-core"
BENCH=$(go test ./internal/... -bench=. -run='^$' -count=1 2>&1 | grep -c 'Benchmark' || echo 0)
echo "  Count:    $BENCH"

TOTAL=$((GO_CORE + GO_BOT + PY_COUNT))
echo ""
echo "=========================================="
echo "  TOTAL: $TOTAL tests, $GO_FAIL failures"
echo "  Go: $((GO_CORE + GO_BOT)) | Python: $PY_COUNT | Benchmarks: $BENCH"
echo "=========================================="
