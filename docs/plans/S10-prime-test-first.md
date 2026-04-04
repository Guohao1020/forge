# S10' -- Test-First System Enhanced Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Upgrade the TestWriterAgent to use context tools (query_db_schema, query_api_catalog) for generating tests against real project structure, add a test preview UI where users can review/approve test cases before code generation begins, and inject approved tests as hard constraints into CoderAgent.

**Architecture:** TestWriterAgent queries project profiles (db_schema, api_catalog) via ContextBuilder to write tests referencing real table names, column types, and API endpoints. Before code generation, workflow pauses at a "test approval" checkpoint. Frontend renders syntax-highlighted test code with Shiki, and user clicks "Approve Tests & Generate Code" to continue. Approved test files are then injected into CoderAgent's prompt as "your code MUST make these tests pass."

**Tech Stack:** Python 3.12 (TestWriterAgent), Go 1.22 (workflow checkpoint), Next.js + Shiki + shadcn/ui

**Dependencies:** S10 (base test-first system, already complete), S16 (project profiles for context data)

**Duration:** 2 days

---

## File Structure

### Python AI Worker

```
ai-worker/src/
+-- agents/test_writer.py           # MODIFY: query db_schema + api_catalog context tools
+-- activities/test_writing.py      # MODIFY: inject profile data into test generation
```

### Go Backend

```
forge-core/
+-- internal/module/task/
|   +-- model.go                    # MODIFY: add StatusTestApproval constant
|   +-- handler.go                  # MODIFY: add ApproveTests endpoint
|   +-- service.go                  # MODIFY: add ApproveTests logic (signal workflow)
+-- internal/temporal/
|   +-- workflow/task_workflow.go    # MODIFY: add approval checkpoint after TEST_WRITING
+-- internal/router/router.go       # MODIFY: register approve-tests route
```

### Frontend

```
forge-portal/
+-- components/tasks/
|   +-- test-preview.tsx            # NEW: test case preview with Shiki + approve button
|   +-- task-workspace.tsx          # MODIFY: render test-preview for TEST_WRITING approval state
+-- lib/tasks.ts                    # MODIFY: add approveTests API function
```

---

## Day 1: TestWriterAgent with Context Tools + Approval Checkpoint

### Task 1: TestWriterAgent -- Context Tool Integration

**Files:**
- Modify: `ai-worker/src/agents/test_writer.py`
- Modify: `ai-worker/src/activities/test_writing.py`

**IMPORTANT**: Read `ai-worker/src/agents/test_writer.py`, `ai-worker/src/activities/test_writing.py`, `ai-worker/src/context/builder.py` first.

- [ ] **Step 1: Enhance TestWriterAgent prompt with schema awareness**

In `ai-worker/src/agents/test_writer.py`, update `TEST_WRITER_SYSTEM_PROMPT`:

```python
TEST_WRITER_SYSTEM_PROMPT = """You are a senior test engineer. Your task is to write test cases BEFORE the implementation code exists. These tests define the expected behavior.

## Context-Aware Testing Rules
1. Select test framework based on the project's tech stack:
   - Go -> use "testing" package (go test)
   - Java -> use JUnit 5
   - Python -> use pytest
   - JavaScript/TypeScript -> use Jest/Vitest
   - If unknown -> use pytest as default
2. When DB schema is available:
   - Reference REAL table names and column types in tests
   - Test database operations against actual schema constraints (NOT NULL, UNIQUE, FK)
   - Include tests for migration compatibility
3. When API catalog is available:
   - Write API integration tests against existing endpoint patterns
   - Test HTTP status codes, request/response shapes, error cases
   - Ensure new endpoints don't conflict with existing ones
4. For each task in the plan, write corresponding test cases
5. Cover: happy path (2+), edge cases (1+), error cases (1+) per function
6. Tests should be compilable/runnable before implementation (use interfaces/mocks)
7. Use descriptive test names: TestCreateUser_WithDuplicateEmail_ReturnsConflict

## Output Format
IMPORTANT: You MUST respond with ONLY a JSON object. No explanations, no markdown.

{"test_files": [{"path": "tests/user_test.go", "content": "package tests\\n\\nimport ...", "language": "go", "framework": "go_test", "covers_task": 1, "test_count": 3}], "total_test_count": 6, "framework": "go_test", "coverage_targets": ["UserService.Create", "UserService.Delete"]}
"""
```

- [ ] **Step 2: Inject profile data into test generation activity**

In `ai-worker/src/activities/test_writing.py`, update the user prompt construction:

```python
@activity.defn(name="generate_test_cases")
async def generate_test_cases_activity(input: TestWritingInput) -> TestWritingOutput:
    logger.info(f"Generating test cases for task {input.task_id}")
    builder = ContextBuilder()
    try:
        ctx = await builder.build(input.project_id, purpose="code-generation")

        user_prompt = ""
        if input.requirement_summary:
            user_prompt += f"## Requirement\n{input.requirement_summary}\n\n"

        # Inject DB schema from project profiles
        if ctx.project_profiles:
            db_schema = ctx.project_profiles.get("db_schema", {})
            if db_schema:
                tables = db_schema.get("tables", [])
                if tables:
                    schema_str = json.dumps(tables[:15], indent=2, ensure_ascii=False)
                    user_prompt += f"## Database Schema (REAL -- reference these in tests)\n{schema_str}\n\n"

            api_catalog = ctx.project_profiles.get("api_catalog", {})
            if api_catalog:
                endpoints = api_catalog.get("endpoints", [])
                if endpoints:
                    api_str = json.dumps(endpoints[:20], indent=2, ensure_ascii=False)
                    user_prompt += f"## Existing API Endpoints (test new endpoints against this pattern)\n{api_str}\n\n"

        if input.plan:
            tasks = input.plan.get("tasks", [])
            if tasks:
                user_prompt += f"## Implementation Plan\n{json.dumps(tasks, indent=2, ensure_ascii=False)}\n\n"

        user_prompt += "\nGenerate test cases for the implementation tasks above. Write tests that will validate the code BEFORE it is written. Use REAL table/column names from the schema above."

        router = ModelRouter()
        agent = TestWriterAgent(router)
        result = await agent.run(user_prompt, ctx)

        return TestWritingOutput(
            test_files=result.structured.get("test_files", []),
            test_count=result.structured.get("total_test_count", 0),
            framework=result.structured.get("framework", ""),
            coverage_targets=result.structured.get("coverage_targets", []),
            tokens_used=result.tokens_used,
            model=result.model,
            provider=result.provider,
            latency_ms=result.latency_ms,
        )
    finally:
        await builder.close()
```

- [ ] **Step 3: Verify imports**

```bash
cd ai-worker && python -c "from src.activities.test_writing import generate_test_cases_activity; print('OK')"
```

- [ ] **Step 4: Commit**

```bash
git add ai-worker/
git commit -m "feat(s10'): enhance TestWriterAgent with db_schema and api_catalog context injection"
```

---

### Task 2: Workflow Approval Checkpoint

**Files:**
- Modify: `forge-core/internal/module/task/model.go`
- Modify: `forge-core/internal/module/task/handler.go`
- Modify: `forge-core/internal/module/task/service.go`
- Modify: `forge-core/internal/temporal/workflow/task_workflow.go`
- Modify: `forge-core/internal/router/router.go`

**IMPORTANT**: Read `task_workflow.go` first to understand the existing workflow structure.

- [ ] **Step 1: Add StatusTestApproval to model.go**

```go
const (
    // ... existing statuses ...
    StatusTestApproval = "TEST_APPROVAL"   // NEW: waiting for user to approve test cases
)
```

- [ ] **Step 2: Add workflow signal for test approval**

In `task_workflow.go`, after the TEST_WRITING step completes and SaveStepOutput is called, add a signal wait:

```go
// ---- After TEST_WRITING step output saved ----

// Update task status to TEST_APPROVAL (waiting for user)
_ = workflow.ExecuteActivity(localCtx, "UpdateTaskStatus", input.TaskID, "TEST_APPROVAL").Get(ctx, nil)

// Notify frontend via SSE that tests are ready for review
_ = workflow.ExecuteActivity(localCtx, "SendSSEEvent", input.TaskID, "test_approval_needed", testResult).Get(ctx, nil)

// Wait for user approval signal
var testApproved bool
approvalCh := workflow.GetSignalChannel(ctx, "approve_tests")
approvalCh.Receive(ctx, &testApproved)

if !testApproved {
    // User rejected tests -- could re-generate or skip
    slog.Warn("test cases rejected by user", "task_id", input.TaskID)
    // Continue without test constraints
    testResult = map[string]interface{}{}
}

// Continue to GENERATE step...
```

Signal name: `approve_tests`
Signal payload: `bool` (true = approved, false = rejected)

- [ ] **Step 3: Add ApproveTests handler**

In `handler.go`:

```go
// POST /api/projects/:id/tasks/:taskId/approve-tests
func (h *Handler) ApproveTests(c *gin.Context) {
    taskID, _ := strconv.ParseInt(c.Param("taskId"), 10, 64)
    var req struct {
        Approved bool `json:"approved"`
    }
    if err := c.ShouldBindJSON(&req); err != nil {
        response.Fail(c, http.StatusBadRequest, "invalid request")
        return
    }
    err := h.service.ApproveTests(c.Request.Context(), taskID, req.Approved)
    if err != nil {
        response.Fail(c, http.StatusInternalServerError, "failed to approve tests: "+err.Error())
        return
    }
    response.OK(c, gin.H{"approved": req.Approved})
}
```

- [ ] **Step 4: Add ApproveTests service method**

In `service.go`:

```go
func (s *Service) ApproveTests(ctx context.Context, taskID int64, approved bool) error {
    task, err := s.repo.GetByID(ctx, taskID)
    if err != nil {
        return err
    }
    if task.WorkflowID == nil || task.WorkflowRunID == nil {
        return fmt.Errorf("task has no active workflow")
    }
    // Signal the Temporal workflow
    return s.temporal.SignalWorkflow(ctx, *task.WorkflowID, *task.WorkflowRunID, "approve_tests", approved)
}
```

- [ ] **Step 5: Register route**

In `router.go`, add inside the tasks group:

```go
protected.POST("/projects/:id/tasks/:taskId/approve-tests", deps.TaskHandler.ApproveTests)
```

- [ ] **Step 6: Verify build**

```bash
cd forge-core && go build ./cmd/forge-core
```

- [ ] **Step 7: Commit**

```bash
git add forge-core/
git commit -m "feat(s10'): add test approval checkpoint in workflow with Temporal signal"
```

---

## Day 2: Test Preview UI

### Task 3: Frontend -- Test Preview Component

**Files:**
- Create: `forge-portal/components/tasks/test-preview.tsx`
- Modify: `forge-portal/components/tasks/task-workspace.tsx`
- Modify: `forge-portal/lib/tasks.ts`

**IMPORTANT**: Read `task-workspace.tsx` and the existing `ShikiCodeViewer` component first.

- [ ] **Step 1: Add approveTests to lib/tasks.ts**

```typescript
export async function approveTests(
  projectId: number,
  taskId: number,
  approved: boolean
): Promise<void> {
  return api.post(`/projects/${projectId}/tasks/${taskId}/approve-tests`, { approved });
}
```

- [ ] **Step 2: Create TestPreview component**

`forge-portal/components/tasks/test-preview.tsx`:

```
+-------------------------------------------------------------+
| Test Cases Preview                    go_test | 6 tests      |
+-------------------------------------------------------------+
|                                                               |
| +--- tests/user_service_test.go (covers task #1) ----------+|
| |                                                           ||
| |  func TestCreateUser_WithValidInput_ReturnsUser(t *...) { ||
| |    // ... (Shiki syntax highlighted)                      ||
| |  }                                                        ||
| |                                                           ||
| |  func TestCreateUser_WithDuplicateEmail_ReturnsErr(t ...) ||
| |    // ...                                                 ||
| |  }                                                        ||
| +-----------------------------------------------------------+|
|                                                               |
| +--- tests/user_handler_test.go (covers task #2) -----------+|
| |  // ... syntax highlighted test code                      ||
| +-----------------------------------------------------------+|
|                                                               |
| Coverage Targets: UserService.Create, UserService.Delete      |
|                                                               |
| [Reject & Regenerate]        [Approve Tests & Generate Code] |
+-------------------------------------------------------------+
```

Component structure:
- Props: `testOutput: TestWritingOutput`, `projectId: number`, `taskId: number`, `onApproved: () => void`
- Header: framework badge + test count badge
- For each `test_file` in output:
  - Collapsible card with file path as title
  - "covers task #N" badge
  - `ShikiCodeViewer` for test code, with language from `test_file.language`
- Coverage targets list (chips)
- Two action buttons:
  - "Reject & Regenerate" (secondary variant) -- calls `approveTests(projectId, taskId, false)`
  - "Approve Tests & Generate Code" (primary, purple) -- calls `approveTests(projectId, taskId, true)`
- Loading state during API call
- After approval, show checkmark and transition to code generation

- [ ] **Step 3: Integrate into task-workspace.tsx**

In `task-workspace.tsx`, find the TEST_WRITING step rendering logic and add the approval state:

```tsx
if (stepType === "TEST_WRITING") {
  if (status === "RUNNING") {
    return (
      <div className="flex flex-col items-center justify-center h-full gap-3">
        <Loader2 className="h-8 w-8 animate-spin text-purple-400" />
        <p className="text-muted-foreground">AI is generating test cases...</p>
      </div>
    );
  }
  // NEW: approval state
  if (taskStatus === "TEST_APPROVAL" && output) {
    return (
      <TestPreview
        testOutput={output}
        projectId={projectId}
        taskId={taskId}
        onApproved={() => {
          // Refresh task data to see GENERATING status
          refetchTask();
        }}
      />
    );
  }
  if (status === "COMPLETED" && output) {
    // ... existing completed view (read-only, no buttons) ...
  }
}
```

Key UX flow:
1. TEST_WRITING step RUNNING -> spinner
2. TEST_WRITING step COMPLETED + task status TEST_APPROVAL -> TestPreview with buttons
3. User clicks "Approve" -> workflow continues to GENERATE
4. Task status changes to GENERATING -> normal code generation view

- [ ] **Step 4: Verify frontend build**

```bash
cd forge-portal && npm run build
```

- [ ] **Step 5: Commit**

```bash
git add forge-portal/
git commit -m "feat(s10'): add test preview UI with Shiki syntax highlighting and approval flow"
```

---

### Task 4: Build Verification + End-to-End Testing

- [ ] **Step 1: Go build**

```bash
cd forge-core && go build ./cmd/forge-core
```

- [ ] **Step 2: Frontend build**

```bash
cd forge-portal && npm run build
```

- [ ] **Step 3: Rebuild AI Worker**

```bash
docker compose -f docker-compose.dev.yml up -d --build ai-worker
```

- [ ] **Step 4: End-to-end verification**

1. Restart all services
2. Ensure a project has profile data (run profile scan first)
3. Create a new task with a requirement
4. Workflow runs: ANALYZE -> PLAN -> TEST_WRITING
5. After TEST_WRITING completes:
   - Task status = TEST_APPROVAL
   - Frontend shows TestPreview component
   - Test code references real table names from db_schema
   - Test code follows project's framework (go test for Go project)
6. Click "Approve Tests & Generate Code"
   - Workflow receives signal, continues to GENERATE
   - Task status changes to GENERATING
   - Generated code includes test file references
7. Verify: if user clicks "Reject", workflow continues without test constraints

- [ ] **Step 5: Commit**

```bash
git add -A
git commit -m "feat(s10'): complete test-first system with context tools and approval checkpoint"
```

---

## Data Structures

### API Endpoints (new)

| Method | Path | Description |
|--------|------|-------------|
| POST | `/api/projects/:id/tasks/:taskId/approve-tests` | Approve or reject test cases |

### Workflow Signal

| Signal | Channel | Payload | Effect |
|--------|---------|---------|--------|
| Test Approval | `approve_tests` | `bool` | true=continue with tests, false=skip tests |

### SSE Event (new)

| Event | Data | When |
|-------|------|------|
| `test_approval_needed` | TestWritingOutput JSON | After TEST_WRITING completes, before GENERATE |

---

## Acceptance Criteria

- [ ] TestWriterAgent queries real db_schema and generates tests with actual table/column names
- [ ] TestWriterAgent queries api_catalog and generates API tests matching existing patterns
- [ ] Workflow pauses at TEST_APPROVAL after tests are generated
- [ ] Frontend shows TestPreview with Shiki-highlighted test code
- [ ] User can approve tests -> workflow continues to code generation
- [ ] User can reject tests -> workflow continues without test constraints
- [ ] Approved test cases are injected into CoderAgent prompt
- [ ] Test file counts and framework detected correctly
- [ ] Coverage targets listed in preview
- [ ] `go build` + `npm run build` + ai-worker rebuild pass
