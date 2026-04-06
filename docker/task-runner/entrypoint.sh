#!/usr/bin/env bash
set -e

echo "=== Forge Task Runner ==="
echo "TASK_ID: $TASK_ID"
echo "FRAMEWORK: $FRAMEWORK"
echo "FORGE_API_URL: $FORGE_API_URL"

# Check if files are mounted via ConfigMap
if [ "$FILES_MOUNTED" = "true" ] && [ -d "/workspace/files" ]; then
  echo "Files mounted via ConfigMap, restoring to workspace..."
  cd /workspace
  # Only process real files (skip K8s ConfigMap symlink dirs like ..data, ..2026_xxx)
  find /workspace/files -maxdepth 1 -type l -o -type f | while read -r f; do
    fname=$(basename "$f")
    # Skip K8s internal files (start with ..)
    [[ "$fname" == ..* ]] && continue
    # Restore path: __ becomes /, gen__ prefix means generated code
    if [[ "$fname" == gen__* ]]; then
      restored=$(echo "${fname#gen__}" | sed 's/__/\//g')
    else
      restored=$(echo "$fname" | sed 's/__/\//g')
    fi
    mkdir -p "$(dirname "$restored")"
    cp "$f" "$restored"
    echo "  Restored: $restored"
  done
  echo "Files restored from ConfigMap."
else
  # Fallback: fetch from API (original behavior)
  echo "Fetching generated code and test files from API..."

  TASK_DATA=$(curl -sf -H "Authorization: Bearer $FORGE_API_TOKEN" \
    "$FORGE_API_URL/api/internal/tasks/$TASK_ID/steps" 2>/dev/null || echo '{}')

  if [ "$TASK_DATA" = "{}" ] || [ -z "$TASK_DATA" ]; then
    echo "ERROR: Could not fetch task data from forge-core"
    TASK_DATA=$(curl -sf -H "Authorization: Bearer $FORGE_API_TOKEN" \
      "$FORGE_API_URL/api/projects/0/tasks/$TASK_ID" 2>/dev/null || echo '{}')
  fi

  GENERATE_OUTPUT=$(echo "$TASK_DATA" | jq -r '.data.steps[] | select(.step_type == "GENERATE") | .output // empty' 2>/dev/null || echo '')
  TEST_OUTPUT=$(echo "$TASK_DATA" | jq -r '.data.steps[] | select(.step_type == "TEST_WRITING") | .output // empty' 2>/dev/null || echo '')

  mkdir -p /workspace/src /workspace/test

  if [ -n "$GENERATE_OUTPUT" ]; then
    echo "$GENERATE_OUTPUT" | jq -c '.files[]?' 2>/dev/null | while read -r file; do
      filepath=$(echo "$file" | jq -r '.path')
      content=$(echo "$file" | jq -r '.content')
      mkdir -p "/workspace/$(dirname "$filepath")"
      echo "$content" > "/workspace/$filepath"
      echo "  Written: $filepath"
    done
  fi

  if [ -n "$TEST_OUTPUT" ]; then
    echo "$TEST_OUTPUT" | jq -c '.test_files[]?' 2>/dev/null | while read -r file; do
      filepath=$(echo "$file" | jq -r '.path')
      content=$(echo "$file" | jq -r '.content')
      mkdir -p "/workspace/$(dirname "$filepath")"
      echo "$content" > "/workspace/$filepath"
      echo "  Written (test): $filepath"
    done
  fi
fi

echo "Files in workspace:"
find /workspace -maxdepth 4 -type f \( -name '*.ts' -o -name '*.tsx' -o -name '*.js' -o -name '*.py' -o -name '*.go' \) | grep -v node_modules | head -20

# Detect framework and run tests
RESULT_FILE="/tmp/test-results.json"
PASSED=0
FAILED=0
TOTAL=0
COVERAGE=0
STATUS="UNKNOWN"

cd /workspace

case "$FRAMEWORK" in
  jest|vitest|"")
    # Node.js project — initialize if needed
    if [ ! -f "package.json" ]; then
      cat > package.json <<'PKGJSON'
{"name":"forge-test","private":true}
PKGJSON
    fi

    # Install Jest with TypeScript/JSX support
    npm install --save-dev jest @types/jest ts-jest typescript \
      @testing-library/react @testing-library/jest-dom react react-dom @types/react 2>/dev/null || true

    # Create jest.config.js with ts-jest for TSX support
    cat > jest.config.js <<'JESTCFG'
module.exports = {
  testEnvironment: 'jsdom',
  transform: {
    '^.+\\.tsx?$': ['ts-jest', { tsconfig: { jsx: 'react-jsx', esModuleInterop: true, module: 'commonjs' } }],
  },
  moduleFileExtensions: ['ts', 'tsx', 'js', 'jsx', 'json'],
  testMatch: ['**/*.test.ts', '**/*.test.tsx', '**/*.spec.ts', '**/*.spec.tsx'],
  moduleNameMapper: {
    '^@/(.*)$': '<rootDir>/$1',
  },
};
JESTCFG

    # Create minimal tsconfig if missing
    if [ ! -f "tsconfig.json" ]; then
      cat > tsconfig.json <<'TSCFG'
{"compilerOptions":{"target":"es2020","module":"commonjs","jsx":"react-jsx","esModuleInterop":true,"strict":false,"moduleResolution":"node","baseUrl":".","paths":{"@/*":["./*"]}}}
TSCFG
    fi

    # Run tests
    npx jest --json --outputFile=/tmp/jest-results.json --passWithNoTests --forceExit --no-cache 2>&1 || true

    if [ -f "/tmp/jest-results.json" ]; then
      PASSED=$(jq '.numPassedTests // 0' /tmp/jest-results.json)
      FAILED=$(jq '.numFailedTests // 0' /tmp/jest-results.json)
      TOTAL=$((PASSED + FAILED))
      STATUS=$([ "$FAILED" -eq 0 ] && echo "PASSED" || echo "FAILED")
      cp /tmp/jest-results.json "$RESULT_FILE"
    else
      STATUS="ERROR"
      echo '{"error":"jest did not produce output"}' > "$RESULT_FILE"
    fi
    ;;

  pytest)
    pytest --json-report --json-report-file=/tmp/pytest-results.json \
           --tb=short -q 2>&1 || true

    if [ -f "/tmp/pytest-results.json" ]; then
      PASSED=$(jq '.summary.passed // 0' /tmp/pytest-results.json)
      FAILED=$(jq '.summary.failed // 0' /tmp/pytest-results.json)
      TOTAL=$((PASSED + FAILED))
      STATUS=$([ "$FAILED" -eq 0 ] && echo "PASSED" || echo "FAILED")
      cp /tmp/pytest-results.json "$RESULT_FILE"
    else
      STATUS="ERROR"
    fi
    ;;

  go)
    go test -json ./... 2>&1 | tee /tmp/go-test.json || true
    PASSED=$(grep '"Test":' /tmp/go-test.json | grep '"Action":"pass"' | wc -l)
    FAILED=$(grep '"Test":' /tmp/go-test.json | grep '"Action":"fail"' | wc -l)
    TOTAL=$((PASSED + FAILED))
    STATUS=$([ "$FAILED" -eq 0 ] && echo "PASSED" || echo "FAILED")
    ;;

  *)
    echo "Unknown framework: $FRAMEWORK"
    STATUS="SKIPPED"
    ;;
esac

echo ""
echo "=== Test Results ==="
echo "Status: $STATUS"
echo "Total: $TOTAL, Passed: $PASSED, Failed: $FAILED"

# Report results back to forge-core
REPORT=$(jq -n \
  --arg status "$STATUS" \
  --argjson total "${TOTAL:-0}" \
  --argjson passed "${PASSED:-0}" \
  --argjson failed "${FAILED:-0}" \
  --argjson coverage "${COVERAGE:-0}" \
  --arg framework "$FRAMEWORK" \
  '{status: $status, total: $total, passed: $passed, failed: $failed, coverage_pct: $coverage, framework: $framework, k8s: true}')

echo "Reporting results to forge-core..."
curl -sf -X POST "$FORGE_API_URL/api/internal/tasks/$TASK_ID/test-results" \
  -H "Authorization: Bearer $FORGE_API_TOKEN" \
  -H "Content-Type: application/json" \
  -d "$REPORT" 2>/dev/null || echo "Warning: Could not report results to forge-core"

echo "=== Task Runner Complete ==="

# Exit with appropriate code
[ "$STATUS" = "PASSED" ] || [ "$STATUS" = "SKIPPED" ] && exit 0 || exit 1
