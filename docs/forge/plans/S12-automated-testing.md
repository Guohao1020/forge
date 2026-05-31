# S12 -- Automated Test Execution Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Execute AI-generated tests in real containers (K8s Jobs or Docker), run four test layers in sequence (unit -> API -> integration -> regression), stream test logs to frontend via SSE, enforce coverage gates, and implement an AI-powered auto-fix loop when tests fail (analyze failure logs -> generate fix -> re-run, max 3 rounds).

**Architecture:** Build a `forge-task-runner` Docker image containing Go 1.22, Node 20, and Python 3.12 runtimes. Test execution happens as a Temporal activity that creates a K8s Job (with Docker fallback on dev). Tests run in 4 layers; each layer must pass before the next begins. Test logs stream to Redis pub/sub, which SSE handler reads and pushes to the frontend. Coverage results are collected and checked against a configurable threshold. On failure, AI analyzes logs and generates a fix, then re-runs (max 3 attempts).

**Tech Stack:** Go 1.22 + client-go + Docker SDK, K8s Jobs (k3s/minikube dev, ACK prod), Redis pub/sub, Python (AI fix agent), Next.js + SSE

**Dependencies:** S11' (code generation with lint), Infra (K8s or Docker available), engine.test_results table (migration 012 already exists)

**Duration:** 3 days

---

## File Structure

### Docker

```
forge-task-runner/
+-- Dockerfile                         # NEW: multi-runtime test runner image
+-- scripts/
|   +-- run-tests.sh                   # NEW: test execution orchestrator script
```

### Go Backend

```
forge-core/
+-- internal/temporal/
|   +-- activity/test_activities.go    # NEW: K8s Job / Docker test execution activity
|   +-- workflow/task_workflow.go       # MODIFY: add real TEST step with 4-layer execution
+-- internal/module/testresult/
|   +-- model.go                       # MODIFY: add layer progression fields
|   +-- repository.go                  # MODIFY: add batch create for multi-layer results
|   +-- handler.go                     # MODIFY: add test log streaming endpoint
|   +-- service.go                     # MODIFY: add coverage gate check
+-- internal/router/router.go         # MODIFY: register test log streaming route
```

### Python AI Worker

```
ai-worker/src/
+-- agents/test_fixer.py               # NEW: agent that analyzes test failures and generates fixes
+-- activities/test_fix.py             # NEW: test failure analysis + fix generation activity
```

### Frontend

```
forge-portal/
+-- components/tasks/
|   +-- test-results-panel.tsx         # NEW: test results display with 4-layer progress
|   +-- test-log-viewer.tsx            # NEW: streaming test log viewer
|   +-- coverage-gauge.tsx             # NEW: coverage percentage gauge
|   +-- task-workspace.tsx             # MODIFY: render test results in TEST step
```

---

## Day 1: Docker Image + Test Execution Activity

### Task 1: forge-task-runner Docker Image

**Files:**
- Create: `forge-task-runner/Dockerfile`
- Create: `forge-task-runner/scripts/run-tests.sh`

- [ ] **Step 1: Create Dockerfile**

`forge-task-runner/Dockerfile`:

```dockerfile
# forge-task-runner: multi-runtime test execution image
FROM ubuntu:22.04 AS base

# Install common tools
RUN apt-get update && apt-get install -y \
    git curl wget ca-certificates \
    && rm -rf /var/lib/apt/lists/*

# Go 1.22
RUN wget -q https://go.dev/dl/go1.22.5.linux-amd64.tar.gz \
    && tar -C /usr/local -xzf go1.22.5.linux-amd64.tar.gz \
    && rm go1.22.5.linux-amd64.tar.gz
ENV PATH="/usr/local/go/bin:${PATH}"
ENV GOPATH="/root/go"

# Node 20
RUN curl -fsSL https://deb.nodesource.com/setup_20.x | bash - \
    && apt-get install -y nodejs \
    && npm install -g npm@latest

# Python 3.12
RUN apt-get update && apt-get install -y \
    python3.12 python3.12-venv python3-pip \
    && rm -rf /var/lib/apt/lists/*
RUN ln -sf /usr/bin/python3.12 /usr/bin/python

# Lint tools (for post-test validation)
RUN go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
RUN npm install -g eslint

# Test runner script
COPY scripts/run-tests.sh /usr/local/bin/run-tests
RUN chmod +x /usr/local/bin/run-tests

WORKDIR /workspace
ENTRYPOINT ["/usr/local/bin/run-tests"]
```

- [ ] **Step 2: Create test runner script**

`forge-task-runner/scripts/run-tests.sh`:

```bash
#!/bin/bash
set -e

# Arguments:
# $1 = repo clone URL
# $2 = branch name
# $3 = test layer (UNIT|API|INTEGRATION|E2E)
# $4 = project type (go|node|python)
# $5 = Redis URL for log streaming
# $6 = task ID

REPO_URL=$1
BRANCH=$2
LAYER=$3
PROJECT_TYPE=$4
REDIS_URL=$5
TASK_ID=$6

log() {
    echo "[$(date -u '+%Y-%m-%dT%H:%M:%SZ')] $1"
    # Publish to Redis for SSE streaming
    if [ -n "$REDIS_URL" ]; then
        redis-cli -u "$REDIS_URL" PUBLISH "test:${TASK_ID}" "$1" 2>/dev/null || true
    fi
}

log "Cloning repository..."
git clone --depth 1 --branch "$BRANCH" "$REPO_URL" /workspace/repo
cd /workspace/repo

log "Installing dependencies..."
case $PROJECT_TYPE in
    go)
        go mod download
        ;;
    node)
        npm ci
        ;;
    python)
        pip install -r requirements.txt 2>/dev/null || true
        ;;
esac

log "Running $LAYER tests..."
case $PROJECT_TYPE in
    go)
        case $LAYER in
            UNIT)
                go test -v -count=1 -coverprofile=coverage.out ./... 2>&1 | tee test-output.log
                ;;
            API)
                go test -v -count=1 -tags=api -coverprofile=coverage.out ./... 2>&1 | tee test-output.log
                ;;
            INTEGRATION)
                go test -v -count=1 -tags=integration -coverprofile=coverage.out ./... 2>&1 | tee test-output.log
                ;;
            E2E)
                go test -v -count=1 -tags=e2e -coverprofile=coverage.out ./... 2>&1 | tee test-output.log
                ;;
        esac
        # Parse coverage
        go tool cover -func=coverage.out | tail -1 | awk '{print $NF}' > coverage-pct.txt
        ;;
    node)
        case $LAYER in
            UNIT)
                npx jest --coverage --ci 2>&1 | tee test-output.log
                ;;
            API|INTEGRATION)
                npx jest --testPathPattern="(api|integration)" --coverage --ci 2>&1 | tee test-output.log
                ;;
            E2E)
                npx jest --testPathPattern="e2e" --coverage --ci 2>&1 | tee test-output.log
                ;;
        esac
        ;;
    python)
        case $LAYER in
            UNIT)
                python -m pytest tests/unit/ -v --cov --cov-report=term 2>&1 | tee test-output.log
                ;;
            API|INTEGRATION)
                python -m pytest tests/integration/ -v --cov --cov-report=term 2>&1 | tee test-output.log
                ;;
        esac
        ;;
esac

# Output results as JSON
EXIT_CODE=${PIPESTATUS[0]}
log "Tests completed with exit code: $EXIT_CODE"

# Parse test counts from output
TOTAL=$(grep -c "^---" test-output.log 2>/dev/null || echo "0")
PASSED=$(grep -c "PASS" test-output.log 2>/dev/null || echo "0")
FAILED=$(grep -c "FAIL" test-output.log 2>/dev/null || echo "0")

cat > /workspace/results.json << EOF
{
    "layer": "$LAYER",
    "exit_code": $EXIT_CODE,
    "total": $TOTAL,
    "passed": $PASSED,
    "failed": $FAILED,
    "coverage_pct": "$(cat coverage-pct.txt 2>/dev/null || echo '0')",
    "log_file": "test-output.log"
}
EOF

exit $EXIT_CODE
```

- [ ] **Step 3: Build image locally**

```bash
cd forge-task-runner && docker build -t forge-task-runner:latest .
```

- [ ] **Step 4: Commit**

```bash
git add forge-task-runner/
git commit -m "feat(s12): create forge-task-runner Docker image with Go/Node/Python runtimes"
```

---

### Task 2: Go Backend -- Test Execution Activity

**Files:**
- Create: `forge-core/internal/temporal/activity/test_activities.go`
- Modify: `forge-core/internal/module/testresult/model.go`
- Modify: `forge-core/internal/module/testresult/repository.go`
- Modify: `forge-core/internal/module/testresult/service.go`

**IMPORTANT**: Read `forge-core/internal/module/testresult/` files and `forge-core/internal/temporal/activity/task_activities.go` first.

- [ ] **Step 1: Create test execution activity**

`forge-core/internal/temporal/activity/test_activities.go`:

```go
package activity

import (
    "context"
    "encoding/json"
    "fmt"
    "log/slog"
    "time"

    "github.com/jackc/pgx/v5/pgxpool"
)

type TestActivities struct {
    db     *pgxpool.Pool
    k8s    K8sClient      // interface -- real or mock
    docker DockerClient   // fallback when K8s unavailable
}

type TestLayerInput struct {
    TaskID      int64  `json:"taskId"`
    ProjectID   int64  `json:"projectId"`
    TenantID    int64  `json:"tenantId"`
    RepoURL     string `json:"repoUrl"`
    Branch      string `json:"branch"`
    Layer       string `json:"layer"`       // UNIT, API, INTEGRATION, E2E
    ProjectType string `json:"projectType"` // go, node, python
    RedisURL    string `json:"redisUrl"`
}

type TestLayerResult struct {
    Layer       string  `json:"layer"`
    Passed      bool    `json:"passed"`
    TotalCases  int     `json:"totalCases"`
    PassedCount int     `json:"passedCount"`
    FailedCount int     `json:"failedCount"`
    CoveragePct float64 `json:"coveragePct"`
    DurationMs  int64   `json:"durationMs"`
    LogOutput   string  `json:"logOutput"`
}

// RunTestLayer executes one test layer in a K8s Job or Docker container
func (a *TestActivities) RunTestLayer(ctx context.Context, input TestLayerInput) (*TestLayerResult, error) {
    start := time.Now()
    slog.Info("running test layer", "task_id", input.TaskID, "layer", input.Layer)

    // K8s Job spec
    jobSpec := buildK8sJobSpec(input)

    var result *TestLayerResult
    var err error

    // Try K8s first, fall back to Docker
    if a.k8s != nil {
        result, err = a.runInK8s(ctx, jobSpec, input)
    }
    if err != nil || a.k8s == nil {
        if err != nil {
            slog.Warn("K8s execution failed, falling back to Docker", "error", err)
        }
        result, err = a.runInDocker(ctx, input)
    }

    if result != nil {
        result.DurationMs = time.Since(start).Milliseconds()
    }

    // Save to test_results table
    a.saveTestResult(ctx, input, result)

    return result, err
}
```

K8s Job spec builder:
```go
func buildK8sJobSpec(input TestLayerInput) *K8sJobSpec {
    return &K8sJobSpec{
        Name:    fmt.Sprintf("test-%d-%s-%d", input.TaskID, input.Layer, time.Now().Unix()),
        Image:   "forge-task-runner:latest",
        Args:    []string{input.RepoURL, input.Branch, input.Layer, input.ProjectType, input.RedisURL, fmt.Sprint(input.TaskID)},
        Timeout: 30 * time.Minute,
        Resources: ResourceRequirements{
            RequestCPU:    "500m",
            RequestMemory: "1Gi",
            LimitCPU:      "2",
            LimitMemory:   "4Gi",
        },
        TTLAfterFinished: 5 * time.Minute,
    }
}
```

- [ ] **Step 2: Enhance testresult model**

In `forge-core/internal/module/testresult/model.go`, add:

```go
// Layer constants
const (
    LayerUnit        = "UNIT"
    LayerAPI         = "API"
    LayerIntegration = "INTEGRATION"
    LayerE2E         = "E2E"
)

// Layer order for sequential execution
var LayerOrder = []string{LayerUnit, LayerAPI, LayerIntegration, LayerE2E}

// TestRunSummary aggregates results across all layers for a task
type TestRunSummary struct {
    TaskID        int64              `json:"taskId"`
    OverallStatus string             `json:"overallStatus"` // PASSED / FAILED / PARTIAL
    TotalCases    int                `json:"totalCases"`
    PassedCases   int                `json:"passedCases"`
    FailedCases   int                `json:"failedCases"`
    CoveragePct   float64            `json:"coveragePct"`
    Layers        []TestLayerSummary `json:"layers"`
}

type TestLayerSummary struct {
    Layer       string  `json:"layer"`
    Status      string  `json:"status"`
    TotalCases  int     `json:"totalCases"`
    Passed      int     `json:"passed"`
    Failed      int     `json:"failed"`
    CoveragePct float64 `json:"coveragePct"`
    DurationMs  int64   `json:"durationMs"`
}
```

- [ ] **Step 3: Add batch result creation to repository**

```go
// CreateBatchResults saves results for multiple test layers
func (r *Repository) CreateBatchResults(ctx context.Context, taskID int64, results []TestLayerResult) error

// GetRunSummary returns aggregated test results for a task
func (r *Repository) GetRunSummary(ctx context.Context, taskID int64) (*TestRunSummary, error)
```

- [ ] **Step 4: Add coverage gate to service**

```go
// CheckCoverageGate checks if coverage meets the configured threshold
func (s *Service) CheckCoverageGate(ctx context.Context, taskID int64, threshold float64) (bool, float64, error) {
    summary, err := s.repo.GetRunSummary(ctx, taskID)
    if err != nil {
        return false, 0, err
    }
    return summary.CoveragePct >= threshold, summary.CoveragePct, nil
}
```

Default threshold: 60% (configurable per project in project settings).

- [ ] **Step 5: Verify build**

```bash
cd forge-core && go build ./cmd/forge-core
```

- [ ] **Step 6: Commit**

```bash
git add forge-core/
git commit -m "feat(s12): add test execution activity with K8s Job + Docker fallback"
```

---

## Day 2: Workflow Integration + AI Auto-Fix Loop

### Task 3: Workflow -- 4-Layer Sequential Execution + Auto-Fix

**Files:**
- Modify: `forge-core/internal/temporal/workflow/task_workflow.go`
- Create: `ai-worker/src/agents/test_fixer.py`
- Create: `ai-worker/src/activities/test_fix.py`
- Modify: `ai-worker/src/worker.py`

- [ ] **Step 1: Add 4-layer test execution to workflow**

In `task_workflow.go`, replace the existing mock TEST step with real execution:

```go
// ---- Step: TEST (4-layer sequential execution) ----
err = workflow.ExecuteActivity(localCtx, "ExecuteStep", activity.StepInput{
    TaskID: input.TaskID, StepType: "TEST", TaskStatus: "TESTING", Duration: 0,
}).Get(ctx, nil)

layers := []string{"UNIT", "API", "INTEGRATION", "E2E"}
var testResults []map[string]interface{}
allPassed := true

for _, layer := range layers {
    layerInput := map[string]interface{}{
        "taskId":      input.TaskID,
        "projectId":   input.ProjectID,
        "tenantId":    input.TenantID,
        "repoUrl":     input.RepoURL,
        "branch":      input.Branch,
        "layer":       layer,
        "projectType": input.ProjectType,
        "redisUrl":    input.RedisURL,
    }

    var layerResult map[string]interface{}
    err = workflow.ExecuteActivity(testCtx, "RunTestLayer", layerInput).Get(ctx, &layerResult)
    if err != nil {
        slog.Warn("test layer failed", "layer", layer, "error", err)
        allPassed = false
        testResults = append(testResults, map[string]interface{}{
            "layer": layer, "passed": false, "error": err.Error(),
        })
        break // Stop sequential execution on failure
    }

    passed, _ := layerResult["passed"].(bool)
    testResults = append(testResults, layerResult)
    if !passed {
        allPassed = false
        break // Previous layer must pass before next
    }
}
```

- [ ] **Step 2: Add AI auto-fix loop**

After test failure, attempt AI-powered fix (max 3 rounds):

```go
if !allPassed {
    // AI auto-fix loop: max 3 attempts
    for attempt := 1; attempt <= 3; attempt++ {
        slog.Info("attempting AI test fix", "task_id", input.TaskID, "attempt", attempt)

        // Get the last failed layer's log output
        lastFailed := testResults[len(testResults)-1]
        failedLayer, _ := lastFailed["layer"].(string)
        logOutput, _ := lastFailed["logOutput"].(string)

        fixInput := map[string]interface{}{
            "task_id":      input.TaskID,
            "project_id":   input.ProjectID,
            "failed_layer": failedLayer,
            "test_log":     logOutput,
            "code_files":   generateResult["files"],
        }

        var fixResult map[string]interface{}
        err = workflow.ExecuteActivity(aiCtx, "analyze_test_failure", fixInput).Get(ctx, &fixResult)
        if err != nil {
            slog.Warn("AI fix generation failed", "error", err)
            break
        }

        // Replace code with fixed version
        if fixedFiles, ok := fixResult["files"].([]interface{}); ok {
            generateResult["files"] = fixedFiles
            // Save updated code
            _ = workflow.ExecuteActivity(localCtx, "SaveStepOutput",
                input.TaskID, "GENERATE", generateResult).Get(ctx, nil)
        }

        // Push fixed code to branch
        _ = workflow.ExecuteActivity(localCtx, "PushCode",
            input.TaskID, fixResult).Get(ctx, nil)

        // Re-run failed layer
        var rerunResult map[string]interface{}
        err = workflow.ExecuteActivity(testCtx, "RunTestLayer",
            map[string]interface{}{
                "taskId": input.TaskID, "layer": failedLayer,
                // ... other fields ...
            }).Get(ctx, &rerunResult)

        if err == nil {
            passed, _ := rerunResult["passed"].(bool)
            if passed {
                allPassed = true
                slog.Info("AI fix succeeded", "task_id", input.TaskID, "attempt", attempt)
                break
            }
        }
    }
}
```

- [ ] **Step 3: Create TestFixerAgent**

`ai-worker/src/agents/test_fixer.py`:

```python
TEST_FIXER_SYSTEM_PROMPT = """You are a debugging expert. You are given failing test output and the code that failed. Your task is to analyze the test failure and fix the code.

## Rules
1. Only fix the code to make the failing tests pass
2. Do NOT modify the test files
3. Do NOT change the API contract or data model
4. Explain each fix briefly
5. If the test itself seems wrong, say so but still try to make the code pass

## Output Format
IMPORTANT: You MUST respond with ONLY a JSON object.
{"files": [{"path": "...", "content": "...", "language": "..."}], "analysis": "Root cause: ...", "fixes": ["description of fix 1", "fix 2"]}
"""

class TestFixerAgent(BaseAgent):
    purpose = Purpose.GENERATE

    def _build_system_prompt(self, context: ProjectContext) -> str:
        return TEST_FIXER_SYSTEM_PROMPT
```

- [ ] **Step 4: Create test fix activity**

`ai-worker/src/activities/test_fix.py`:

```python
@dataclass
class TestFixInput:
    task_id: int
    project_id: int
    failed_layer: str
    test_log: str
    code_files: List[Dict[str, Any]]

@dataclass
class TestFixOutput:
    files: List[Dict[str, Any]]
    analysis: str
    fixes: List[str]
    tokens_used: int = 0

@activity.defn(name="analyze_test_failure")
async def analyze_test_failure_activity(input: TestFixInput) -> TestFixOutput:
    builder = ContextBuilder()
    try:
        ctx = await builder.build(input.project_id, purpose="code-generation")

        user_prompt = f"## Failed Test Layer: {input.failed_layer}\n\n"
        user_prompt += f"## Test Output (FAILURE)\n```\n{input.test_log[-5000:]}\n```\n\n"  # Last 5k chars
        user_prompt += "## Current Code\n"
        for f in input.code_files[:10]:
            user_prompt += f"\n### {f.get('path', '?')}\n```{f.get('language', '')}\n{f.get('content', '')}\n```\n"
        user_prompt += "\nAnalyze the test failure and fix the code. Do NOT modify test files."

        router = ModelRouter()
        agent = TestFixerAgent(router)
        result = await agent.run(user_prompt, ctx)

        return TestFixOutput(
            files=result.structured.get("files", []),
            analysis=result.structured.get("analysis", ""),
            fixes=result.structured.get("fixes", []),
            tokens_used=result.tokens_used,
        )
    finally:
        await builder.close()
```

- [ ] **Step 5: Register in worker.py**

```python
from src.activities.test_fix import analyze_test_failure_activity
```

- [ ] **Step 6: Verify build**

```bash
cd forge-core && go build ./cmd/forge-core
cd ai-worker && python -c "from src.activities.test_fix import analyze_test_failure_activity; print('OK')"
```

- [ ] **Step 7: Commit**

```bash
git add forge-core/ ai-worker/
git commit -m "feat(s12): add 4-layer test execution with AI auto-fix loop (max 3 rounds)"
```

---

## Day 3: Frontend Test Results UI + Quality Gate

### Task 4: Frontend -- Test Results Panel

**Files:**
- Create: `forge-portal/components/tasks/test-results-panel.tsx`
- Create: `forge-portal/components/tasks/test-log-viewer.tsx`
- Create: `forge-portal/components/tasks/coverage-gauge.tsx`
- Modify: `forge-portal/components/tasks/task-workspace.tsx`
- Modify: `forge-core/internal/module/testresult/handler.go`
- Modify: `forge-core/internal/router/router.go`

- [ ] **Step 1: Add test log streaming endpoint**

In `testresult/handler.go`, add an SSE endpoint for test logs:

```go
// GET /api/projects/:id/tasks/:taskId/test-logs -- SSE stream of test execution logs
func (h *Handler) StreamTestLogs(c *gin.Context) {
    taskID, _ := strconv.ParseInt(c.Param("taskId"), 10, 64)
    // Subscribe to Redis pub/sub channel "test:{taskId}"
    // Stream messages as SSE events
    // ... follows same pattern as existing task SSE handler ...
}
```

Register in `router.go`:
```go
protected.GET("/projects/:id/tasks/:taskId/test-logs", deps.TestResultHandler.StreamTestLogs)
```

- [ ] **Step 2: Create CoverageGauge component**

`forge-portal/components/tasks/coverage-gauge.tsx`:

Circular gauge showing coverage percentage:
- Color: green (>= 80%), amber (>= 60%), red (< 60%)
- Center text: "78.5%"
- Label below: "Code Coverage"
- Threshold line indicator (shows the configured minimum)

- [ ] **Step 3: Create TestLogViewer component**

`forge-portal/components/tasks/test-log-viewer.tsx`:

Scrollable log viewer:
- Connects to SSE endpoint `/api/projects/:id/tasks/:taskId/test-logs`
- Auto-scrolls to bottom as new lines arrive
- Color-coded lines: green for PASS, red for FAIL, gray for info
- Monospace font (Geist Mono)
- "Copy All" button
- Max height with scroll

- [ ] **Step 4: Create TestResultsPanel component**

`forge-portal/components/tasks/test-results-panel.tsx`:

```
+-------------------------------------------------------------+
| Test Results                         Overall: PASSED          |
+-------------------------------------------------------------+
|                                                               |
| Layer Progress:                                               |
| [UNIT ====== PASS] [API ====== PASS] [INTEG === FAIL] [E2E] |
|                                                               |
| +------ UNIT Tests ----------------------------------------+ |
| | 12 passed, 0 failed, 0 skipped          78.5% coverage  | |
| | Duration: 3.2s                                           | |
| +----------------------------------------------------------+ |
|                                                               |
| +------ API Tests -----------------------------------------+ |
| | 8 passed, 0 failed                      65.2% coverage  | |
| | Duration: 5.1s                                           | |
| +----------------------------------------------------------+ |
|                                                               |
| +------ Integration Tests ------ FAILED -------------------+ |
| | 3 passed, 2 failed                                       | |
| | Duration: 12.4s                                          | |
| | [View Failure Logs]                                      | |
| +----------------------------------------------------------+ |
|                                                               |
| AI Auto-Fix: Attempt 1/3 - Fixed 2 issues                   |
| AI Auto-Fix: Attempt 2/3 - All tests pass                   |
|                                                               |
| [Coverage Gauge: 72.1%]  Threshold: 60% -- PASSED            |
|                                                               |
| [View Full Logs]                                              |
+-------------------------------------------------------------+
```

Props: `testSummary: TestRunSummary`, `projectId: number`, `taskId: number`

- Layer progress bar: 4 segments, each colored by status (green=pass, red=fail, gray=pending)
- Per-layer card with pass/fail counts, coverage, duration
- Failed layer shows "View Failure Logs" button -> expands to TestLogViewer
- AI fix attempts shown as timeline items
- Coverage gauge at bottom
- Coverage gate: "PASSED" / "FAILED" vs threshold

- [ ] **Step 5: Integrate into task-workspace.tsx**

In the TEST step rendering:

```tsx
if (stepType === "TEST") {
  if (status === "RUNNING") {
    return (
      <div className="space-y-4">
        <div className="flex items-center gap-3">
          <Loader2 className="h-6 w-6 animate-spin text-purple-400" />
          <span>Running automated tests...</span>
        </div>
        <TestLogViewer projectId={projectId} taskId={taskId} />
      </div>
    );
  }
  if (status === "COMPLETED" && output) {
    return <TestResultsPanel testSummary={output} projectId={projectId} taskId={taskId} />;
  }
}
```

- [ ] **Step 6: Verify frontend build**

```bash
cd forge-portal && npm run build
```

- [ ] **Step 7: Commit**

```bash
git add forge-portal/ forge-core/
git commit -m "feat(s12): add test results panel with 4-layer progress, coverage gauge, and log streaming"
```

---

### Task 5: Build Verification + End-to-End Testing

- [ ] **Step 1: Go build**

```bash
cd forge-core && go build ./cmd/forge-core
```

- [ ] **Step 2: Frontend build**

```bash
cd forge-portal && npm run build
```

- [ ] **Step 3: Build task runner image**

```bash
cd forge-task-runner && docker build -t forge-task-runner:latest .
```

- [ ] **Step 4: End-to-end verification**

1. Start all services + Docker/k3s
2. Create a task for a Go project with tests
3. Workflow runs through ANALYZE -> PLAN -> TEST_WRITING -> GENERATE
4. TEST step begins:
   - UNIT layer runs -> logs stream to frontend
   - If UNIT passes -> API layer runs
   - If API passes -> INTEGRATION layer runs
   - Each layer result saved to test_results table
5. Coverage gate check: compare against threshold
6. If any layer fails:
   - AI analyzes failure log
   - Generates fix -> re-pushes code
   - Re-runs failed layer (up to 3 attempts)
7. Frontend shows:
   - Layer progress bar (4 segments)
   - Per-layer results with pass/fail counts
   - Coverage gauge
   - AI fix attempt timeline
   - Streaming log viewer

- [ ] **Step 5: Commit**

```bash
git add -A
git commit -m "feat(s12): complete automated test execution with 4-layer pipeline and AI auto-fix"
```

---

## Acceptance Criteria

- [ ] forge-task-runner Docker image contains Go 1.22, Node 20, Python 3.12
- [ ] Test execution runs in K8s Job (or Docker fallback in dev)
- [ ] 4 test layers execute sequentially: UNIT -> API -> INTEGRATION -> E2E
- [ ] Layer N+1 only runs if Layer N passes
- [ ] Test logs stream to frontend via Redis -> SSE
- [ ] Results stored in engine.test_results table (per layer)
- [ ] Coverage gate: configurable threshold (default 60%), blocks on failure
- [ ] AI auto-fix: on test failure, analyze logs -> generate fix -> re-run (max 3 rounds)
- [ ] Frontend shows layer progress, pass/fail counts, coverage gauge
- [ ] Streaming log viewer with color-coded output
- [ ] K8s Job resources: 500m-2CPU, 1-4Gi memory, 30min timeout
- [ ] `go build` + `npm run build` + Docker build pass
