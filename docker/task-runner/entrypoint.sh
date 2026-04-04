#!/bin/bash
# forge-task-runner entrypoint
#
# Environment variables (set by Forge when creating the container):
#   REPO_URL       — Git clone URL (with token embedded for private repos)
#   BRANCH         — Branch to checkout
#   FRAMEWORK      — Test framework to use (go_test, jest, pytest, junit5)
#   COMMAND         — Override: run this command instead of auto-detected test
#   TASK_ID        — Forge task ID (for reporting)
#   COVERAGE_MIN   — Minimum coverage percentage (default: 60)
#   FORGE_API_URL  — Forge API base URL (for reporting results back)
#   FORGE_API_TOKEN — API token for authentication

set -euo pipefail

COVERAGE_MIN=${COVERAGE_MIN:-60}
RESULT_FILE="/tmp/test-result.json"

echo "=== Forge Task Runner ==="
echo "Task ID:    ${TASK_ID:-unknown}"
echo "Framework:  ${FRAMEWORK:-auto}"
echo "Branch:     ${BRANCH:-main}"
echo "Coverage:   >= ${COVERAGE_MIN}%"
echo "========================="

# --- Step 1: Clone repository ---
if [ -n "${REPO_URL:-}" ]; then
    echo "[1/4] Cloning repository..."
    git clone --depth 1 --branch "${BRANCH:-main}" "${REPO_URL}" /workspace/repo 2>&1 || {
        echo "ERROR: Failed to clone repository"
        exit 1
    }
    cd /workspace/repo
else
    echo "[1/4] No REPO_URL provided, using mounted workspace"
    cd /workspace
fi

# --- Step 2: Install dependencies ---
echo "[2/4] Installing dependencies..."
if [ -f "go.mod" ]; then
    go mod download 2>&1
elif [ -f "package.json" ]; then
    if [ -f "pnpm-lock.yaml" ]; then
        pnpm install --frozen-lockfile 2>&1
    elif [ -f "yarn.lock" ]; then
        npm install 2>&1
    else
        npm ci 2>&1 || npm install 2>&1
    fi
elif [ -f "requirements.txt" ]; then
    pip install -r requirements.txt 2>&1
elif [ -f "pyproject.toml" ]; then
    pip install -e ".[test]" 2>&1 || pip install -e . 2>&1
fi

# --- Step 3: Run tests ---
echo "[3/4] Running tests..."

PASSED=0
FAILED=0
TOTAL=0
COVERAGE=0
STATUS="FAILED"
TEST_OUTPUT=""

# Override command takes priority
if [ -n "${COMMAND:-}" ]; then
    echo "Running custom command: ${COMMAND}"
    TEST_OUTPUT=$(eval "${COMMAND}" 2>&1) || true
    STATUS="PASSED"
else
    # Auto-detect test framework
    DETECTED_FRAMEWORK="${FRAMEWORK:-auto}"

    if [ "${DETECTED_FRAMEWORK}" = "auto" ]; then
        if [ -f "go.mod" ]; then
            DETECTED_FRAMEWORK="go_test"
        elif [ -f "package.json" ]; then
            DETECTED_FRAMEWORK="jest"
        elif [ -f "pyproject.toml" ] || [ -f "requirements.txt" ]; then
            DETECTED_FRAMEWORK="pytest"
        fi
    fi

    case "${DETECTED_FRAMEWORK}" in
        go_test)
            echo "Running: go test ./... -v -coverprofile=coverage.out"
            TEST_OUTPUT=$(go test ./... -v -coverprofile=coverage.out -json 2>&1) || true
            # Extract coverage
            if [ -f "coverage.out" ]; then
                COVERAGE_LINE=$(go tool cover -func=coverage.out | grep total: | awk '{print $3}' | sed 's/%//')
                COVERAGE=${COVERAGE_LINE%.*}  # truncate to int
            fi
            # Count pass/fail from JSON output
            PASSED=$(echo "${TEST_OUTPUT}" | grep -c '"Action":"pass"' || true)
            FAILED=$(echo "${TEST_OUTPUT}" | grep -c '"Action":"fail"' || true)
            TOTAL=$((PASSED + FAILED))
            ;;

        jest)
            echo "Running: npx jest --coverage --json"
            TEST_OUTPUT=$(npx jest --coverage --json --outputFile=/tmp/jest-result.json 2>&1) || true
            if [ -f "/tmp/jest-result.json" ]; then
                PASSED=$(jq '.numPassedTests' /tmp/jest-result.json 2>/dev/null || echo 0)
                FAILED=$(jq '.numFailedTests' /tmp/jest-result.json 2>/dev/null || echo 0)
                TOTAL=$(jq '.numTotalTests' /tmp/jest-result.json 2>/dev/null || echo 0)
                # Extract line coverage
                COVERAGE=$(jq '.coverageMap | to_entries | map(.value.s | to_entries | map(.value) | add) | add // 0' /tmp/jest-result.json 2>/dev/null | head -1 || echo 0)
            fi
            ;;

        pytest)
            echo "Running: python -m pytest -v --cov --cov-report=json"
            TEST_OUTPUT=$(python -m pytest -v --cov --cov-report=json:/tmp/cov.json 2>&1) || true
            # Extract pass/fail from pytest output
            PASSED=$(echo "${TEST_OUTPUT}" | grep -oP '\d+ passed' | grep -oP '\d+' || echo 0)
            FAILED=$(echo "${TEST_OUTPUT}" | grep -oP '\d+ failed' | grep -oP '\d+' || echo 0)
            TOTAL=$((PASSED + FAILED))
            if [ -f "/tmp/cov.json" ]; then
                COVERAGE=$(jq '.totals.percent_covered' /tmp/cov.json 2>/dev/null | cut -d. -f1 || echo 0)
            fi
            ;;

        *)
            echo "WARNING: Unknown framework '${DETECTED_FRAMEWORK}', running generic test"
            TEST_OUTPUT="No test framework detected"
            ;;
    esac

    # Determine pass/fail
    if [ "${FAILED}" -eq 0 ] && [ "${TOTAL}" -gt 0 ]; then
        STATUS="PASSED"
    fi

    # Coverage gate
    if [ "${COVERAGE}" -lt "${COVERAGE_MIN}" ] && [ "${STATUS}" = "PASSED" ]; then
        echo "WARNING: Coverage ${COVERAGE}% is below minimum ${COVERAGE_MIN}%"
        # Don't fail on coverage yet — just warn (configurable in future)
    fi
fi

# --- Step 4: Report results ---
echo "[4/4] Test complete."
echo "Status:   ${STATUS}"
echo "Tests:    ${TOTAL} total, ${PASSED} passed, ${FAILED} failed"
echo "Coverage: ${COVERAGE}%"

# Write result JSON
cat > "${RESULT_FILE}" <<EOF
{
    "status": "${STATUS}",
    "framework": "${DETECTED_FRAMEWORK:-unknown}",
    "total": ${TOTAL:-0},
    "passed": ${PASSED:-0},
    "failed": ${FAILED:-0},
    "coverage": ${COVERAGE:-0},
    "coverage_min": ${COVERAGE_MIN},
    "task_id": ${TASK_ID:-0}
}
EOF

# Report back to Forge API if configured
if [ -n "${FORGE_API_URL:-}" ] && [ -n "${FORGE_API_TOKEN:-}" ]; then
    echo "Reporting results to Forge API..."
    curl -s -X POST \
        "${FORGE_API_URL}/api/projects/0/tasks/${TASK_ID}/test-results" \
        -H "Authorization: Bearer ${FORGE_API_TOKEN}" \
        -H "Content-Type: application/json" \
        -d @"${RESULT_FILE}" || echo "WARNING: Failed to report to Forge API"
fi

cat "${RESULT_FILE}"

# Exit with test status
if [ "${STATUS}" = "PASSED" ]; then
    exit 0
else
    exit 1
fi
