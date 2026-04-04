# SX-4 -- End-to-End Quality Validation + S15 Wrap-up

**Duration**: 1-2 days
**Priority**: P0 -- This is the "ship it" gate; nothing launches without a verified run
**Dependencies**: SX-1, SX-2, SX-3 (all three must be done first)

---

## 1. Goal

Execute a complete end-to-end flow on a real GitHub project. Record every step: inputs, outputs, model used, tokens consumed, time taken. Fix any issues found. Also close out remaining S15 (code browsing) items.

## 2. Preparation

### 2.1 Create Demo GitHub Repository

Create a minimal but realistic Go project for testing the full pipeline.

**Repository**: `forge-demo-api` (private, under your GitHub account)

**Structure**:

```
forge-demo-api/
  go.mod
  go.sum
  cmd/
    server/
      main.go          # HTTP server with Gin, starts on :9090
  internal/
    handler/
      user_handler.go  # GET /users, POST /users
    service/
      user_service.go  # Business logic
    model/
      user.go          # User struct
    repository/
      user_repo.go     # PostgreSQL repository
  migrations/
    001_create_users.sql
  Dockerfile
  README.md
```

**Files content** (keep minimal -- ~200 lines total):

`go.mod`:
```
module github.com/<you>/forge-demo-api

go 1.22

require (
    github.com/gin-gonic/gin v1.9.1
    github.com/jackc/pgx/v5 v5.5.0
)
```

`cmd/server/main.go`:
```go
package main

import (
    "log"
    "github.com/<you>/forge-demo-api/internal/handler"
    "github.com/gin-gonic/gin"
)

func main() {
    r := gin.Default()
    h := handler.NewUserHandler(nil) // nil repo for demo
    r.GET("/users", h.List)
    r.POST("/users", h.Create)
    log.Fatal(r.Run(":9090"))
}
```

`internal/model/user.go`:
```go
package model

import "time"

type User struct {
    ID        int64     `json:"id"`
    Name      string    `json:"name"`
    Email     string    `json:"email"`
    CreatedAt time.Time `json:"created_at"`
}
```

`internal/handler/user_handler.go`:
```go
package handler

import (
    "net/http"
    "github.com/gin-gonic/gin"
)

type UserHandler struct {}

func NewUserHandler(repo interface{}) *UserHandler {
    return &UserHandler{}
}

func (h *UserHandler) List(c *gin.Context) {
    c.JSON(http.StatusOK, gin.H{"users": []interface{}{}})
}

func (h *UserHandler) Create(c *gin.Context) {
    c.JSON(http.StatusCreated, gin.H{"message": "created"})
}
```

`migrations/001_create_users.sql`:
```sql
CREATE TABLE users (
    id BIGSERIAL PRIMARY KEY,
    name VARCHAR(100) NOT NULL,
    email VARCHAR(200) UNIQUE NOT NULL,
    created_at TIMESTAMP DEFAULT NOW()
);
```

Push to GitHub. This gives the AI enough context to work with (Go project, Gin framework, PostgreSQL, REST API pattern) but is small enough for fast scanning.

### 2.2 Setup Coding Standards

Before the test run, create at least one coding standard via the Specs Center (verified in SX-2):

```bash
TOKEN=$(curl -s -X POST http://localhost:8080/api/auth/login \
  -H "Content-Type: application/json" \
  -d '{"username":"admin","password":"admin123"}' | jq -r '.data.token')

curl -X POST http://localhost:8080/api/specs/standards \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Go API Standards",
    "category": "API",
    "scope": "COMPANY",
    "scopeId": 0,
    "content": "## Go API Standards\n1. All handlers must validate input and return structured error responses\n2. Use middleware for authentication -- never check auth in handlers directly\n3. Repository functions must accept context.Context as first parameter\n4. All database queries must use parameterized queries (no string concatenation)\n5. HTTP responses must use consistent JSON wrapper: {\"code\": 0, \"data\": ..., \"message\": \"\"}"
  }'
```

### 2.3 Ensure All Services Running

```bash
# Terminal 1: PostgreSQL + Redis + Temporal
docker compose -f docker-compose.dev.yml up -d

# Terminal 2: forge-core
cd forge-core && go run ./cmd/forge-core

# Terminal 3: ai-worker
cd ai-worker && python -m src.main

# Terminal 4: forge-portal
cd forge-portal && npm run dev

# Verify
curl http://localhost:8080/health     # {"status":"ok"}
curl http://localhost:3000            # Next.js page
# Check Temporal Web UI at http://localhost:8233
```

## 3. End-to-End Test Execution

### Step 3.1: Import Project (5 min)

**Action**: In the frontend (localhost:3000), go to Projects page, click "Import from GitHub", select `forge-demo-api`.

**Record**:
| Metric | Value |
|--------|-------|
| Import time | ___ seconds |
| Tech stack detected | Go, Docker (expected) |
| Profile scan auto-triggered | Yes/No |

**Verify**:
- Project appears in project list
- Code browser shows correct file tree
- Profile page shows scan results (if auto-trigger works from SX-3)

### Step 3.2: Profile Scan (5 min)

If auto-scan did not trigger, manually scan:

**Action**: Go to project profile page, click "Scan".

**Record**:
| Metric | Value |
|--------|-------|
| Scan start time | ___ |
| Scan completion time | ___ |
| Dimensions scanned | ___ / 5 |
| Dimensions failed | ___ |
| Model used | ___ (check ai-worker logs) |
| Total tokens | ___ |

**Verify profile content**:
```bash
curl -s http://localhost:8080/api/projects/<id>/profiles \
  -H "Authorization: Bearer $TOKEN" | jq '.data.profiles[] | {key: .profileKey, valueSize: (.profileValue | tostring | length)}'
```

Expected:
```json
{"key": "api_catalog", "valueSize": ...}
{"key": "db_schema", "valueSize": ...}
{"key": "module_graph", "valueSize": ...}
{"key": "architecture", "valueSize": ...}
{"key": "business_rules", "valueSize": ...}
```

### Step 3.3: Create Task + Requirement Analysis (10-15 min)

**Action**: Create a new task with requirement: "Add a search endpoint for users that supports filtering by name and email, with pagination"

**Record for EACH conversation turn**:
| Turn | User Input | AI Status | AI Phase | Time (s) | Model | Tokens |
|------|-----------|-----------|----------|----------|-------|--------|
| 1 | (requirement) | clarify | understanding | ___ | ___ | ___ |
| 2 | (user reply) | clarify | scenario | ___ | ___ | ___ |
| 3 | (user reply) | clarify | constraints | ___ | ___ | ___ |
| 4 | (user reply) | confirmed | - | ___ | ___ | ___ |

**Verify at each turn**:
- Response is in Chinese
- Single question (not multiple)
- Options are provided
- Frontend renders OptionButtons
- Phase progresses (understanding -> scenario -> constraints -> confirmed)

**Verify at confirmation**:
- ConfirmationCard appears with full requirements summary
- functional_requirements list is populated
- acceptance_criteria list is populated
- risks are identified (can be empty for simple tasks)

### Step 3.4: Plan Generation (5-10 min)

**Action**: Click "Confirm" on the ConfirmationCard.

**Record**:
| Metric | Value |
|--------|-------|
| Plan generation time | ___ seconds |
| Model used | ___ |
| Tokens used | ___ |
| Number of task nodes | ___ |
| Total estimated hours | ___ |
| Risk level | ___ |

**Verify**:
- PlanReviewCard appears with task breakdown
- Each task has: title, type (BACKEND/FRONTEND/etc), files list, estimate_hours
- Dependencies are logical (e.g., model -> repository -> handler)
- DAG task list shows on the task workspace panel

### Step 3.5: Plan Approval + Test Writing (5-10 min)

**Action**: Click "Approve" on the PlanReviewCard.

**Record**:
| Step | Time (s) | Model | Tokens | Output Summary |
|------|----------|-------|--------|---------------|
| TEST_WRITING | ___ | ___ | ___ | ___ test files, ___ total tests |

**Verify**:
- Test cases are generated for the Go project
- Test file paths are correct (e.g., `internal/handler/user_handler_test.go`)
- Tests use correct framework (testing + testify or httptest)
- Test cases cover the search/filter/pagination logic from the requirement

### Step 3.6: Code Generation (5-10 min)

**Record**:
| Metric | Value |
|--------|-------|
| Generation time | ___ seconds |
| Model used | ___ |
| Tokens used | ___ |
| Files generated | ___ |
| Lines added | ___ |
| Lines deleted | ___ |

**Verify generated code**:
- Files match the plan (correct paths)
- Code follows the coding standard injected in SX-2 (error handling, parameterized queries, etc.)
- Code is syntactically valid Go
- Streaming code view shows real-time token output (if streaming is wired)

### Step 3.7: Code Review (5-10 min)

**Record**:
| Metric | Value |
|--------|-------|
| Review time | ___ seconds |
| Model used | ___ |
| Score | ___ / 100 |
| Passed | Yes/No |
| Findings count | ___ (ERROR: ___, WARNING: ___, INFO: ___) |
| Review attempts | ___ / 3 |

**Verify**:
- Review checks coding standards (findings with rule "STANDARD/" prefix)
- Review checks lint rules (findings with rule "LINT/" prefix)
- If failed, auto-fix loop triggers and generates corrected code
- ReviewReportCard shows findings on the task workspace

### Step 3.8: Test Execution (2-5 min)

**Record**:
| Metric | Value |
|--------|-------|
| Test execution time | ___ ms |
| Framework | ___ |
| Total tests | ___ |
| Passed | ___ |
| Failed | ___ |
| Coverage | ___ % |
| K8s job | Yes/No (likely No/mock) |

### Step 3.9: Deploy (Push + PR) (2-5 min)

**Record**:
| Metric | Value |
|--------|-------|
| Branch name | feature/YYYY-MM-DD/... |
| Files pushed | ___ |
| PR number | ___ |
| PR URL | ___ |
| Preview URL | ___ (mock) |

**Verify**:
- Branch was created on GitHub
- PR was created with correct title and description
- PR contains exactly the files from code generation
- Branch name follows the pattern: `feature/{date}/{tenantId}/{userId}/{taskId}-{slug}`
- StepTimeline shows all steps as COMPLETED

### Step 3.10: Full Metrics Summary

Fill out the complete metrics table:

| Step | Duration | Model | Provider | Tokens In | Tokens Out | Cost Est. |
|------|----------|-------|----------|-----------|------------|-----------|
| Profile Scan (5 dims) | | | | | | |
| Analysis (4 turns) | | | | | | |
| Plan | | | | | | |
| Test Writing | | | | | | |
| Code Generation | | | | | | |
| Review (1 attempt) | | | | | | |
| Test Execution | | | | | | |
| Deploy | | | | | | |
| **TOTAL** | | | | | | |

## 4. Expected Issues and Fixes

Based on codebase analysis, these are the most likely issues:

### Issue 4.1: AI Worker auth failure on internal API calls

**Symptom**: ContextBuilder logs show 401 on `/api/specs/effective/` or `/api/projects/`
**Fix**: Set `FORGE_API_TOKEN` in `ai-worker/.env` (see SX-2 Step 3.7)

### Issue 4.2: Profile scan activity not registered in Python worker

**Symptom**: Temporal shows "activity not registered" error for `scan_project_profile`
**Fix**: Add `scan_project_profile_activity` to the activities list in `ai-worker/src/main.py`

### Issue 4.3: Generated code references non-existent packages

**Symptom**: Code generation creates files that import packages not in go.mod
**Fix**: This is expected for AI-generated code. The generated code would need `go mod tidy` in a real build environment. Note for future: add a post-generation step that validates imports.

### Issue 4.4: Review is too strict / too lenient

**Symptom**: Review always passes (score 95+) even with obvious issues, OR always fails
**Fix**: Tune the `REVIEWER_SYSTEM_PROMPT` in `ai-worker/src/agents/reviewer.py`. Pass threshold is `score >= 80 AND zero ERROR findings`.

### Issue 4.5: PR creation fails -- no GitHub token for service user

**Symptom**: Deploy step fails with "no GitHub token available"
**Fix**: Ensure the user who created the task has a connected GitHub account (OAuth flow completed). The `DevOpsActivities.PushToGitHub` uses the creator's token.

### Issue 4.6: Branch naming collision

**Symptom**: Push fails with "branch already exists"
**Fix**: The branch name includes taskId which should be unique. If re-running the same task, delete the old branch first or use a suffix.

### Issue 4.7: File tree empty for newly imported project

**Symptom**: Profile scan or code generation gets empty file tree
**Fix**: The local workspace clone may not have completed yet. Wait and retry, or ensure `workspace.Manager` clones on import.

## 5. S15 Remaining Work

### 5.1 File Search (Ctrl+P) -- Not Implemented

**Current state**: Code browser at `forge-portal/app/(dashboard)/projects/[id]/code/page.tsx` has a file tree but no search.

**Implementation**:

**File**: `forge-portal/components/code-browser/file-search-dialog.tsx` (new)

Create a command palette-style dialog:

```tsx
// Fuzzy file search dialog triggered by Ctrl+P
// Uses the already-loaded file tree data
// Filters files as user types
// Clicking a file navigates to it in the code viewer

interface FileSearchDialogProps {
  files: string[];        // Flat list of file paths from API
  open: boolean;
  onOpenChange: (open: boolean) => void;
  onSelect: (path: string) => void;
}
```

Key implementation details:
- Listen for `Ctrl+P` / `Cmd+P` keyboard shortcut globally on the code browser page
- Use the file tree data already fetched by `GET /api/projects/:id/code/tree`
- Implement fuzzy matching: split query into characters, check if they appear in order in the file path
- Show top 20 results ranked by match quality
- Highlight matching characters in results
- Use shadcn `Dialog` or `Command` component (if available)

**Files to modify**:
- `forge-portal/components/code-browser/file-search-dialog.tsx` (NEW)
- `forge-portal/app/(dashboard)/projects/[id]/code/page.tsx` (add keyboard listener + dialog)

### 5.2 Large File Truncation

**Current state**: `forge-portal/components/code-preview/code-viewer.tsx` renders all content regardless of size. Very large files (>10000 lines) cause browser lag.

**Fix**: Add truncation with a "Show full file" button.

**File**: `forge-portal/components/code-preview/code-viewer.tsx` or `shiki-code-viewer.tsx`

```tsx
const MAX_LINES = 5000;
const lines = content.split('\n');
const isTruncated = lines.length > MAX_LINES;
const displayContent = isTruncated
  ? lines.slice(0, MAX_LINES).join('\n') + '\n\n// ... truncated (' + lines.length + ' total lines)'
  : content;
```

Also handle the API side: `forge-core/internal/module/project/service.go` line 398-418 (`GetCodeFile`) already has no size limit. The `_fetch_file_content` in the Python activity (profile.py line 116-131) has a 50KB limit. Make the Go API truncate similarly for the frontend:

```go
// In GetCodeFile, add truncation for very large files
const maxFileSize = 500_000 // 500KB for frontend display
if len(content) > maxFileSize {
    content = content[:maxFileSize] + "\n\n// ... file truncated (original size: " + strconv.Itoa(len(content)) + " bytes)"
}
```

### 5.3 Empty State Handling

**Current state**: Several pages show nothing when data is empty.

**Check and fix these locations**:

| Component | Empty State Needed | File |
|-----------|-------------------|------|
| Code browser -- no repo connected | "Connect GitHub to browse code" | `code/page.tsx` |
| Profile page -- no scans yet | "Click Scan to analyze your project" | `profile/page.tsx` |
| Task list -- no tasks | "Create your first task" | Tasks list page |
| Conversation -- no messages | Already handled (chat-panel.tsx line 84-86) | OK |
| Code preview -- no step selected | Already handled | OK |

Each empty state should have:
- A descriptive message explaining what goes here
- An icon (from Lucide)
- A CTA button if applicable

## 6. Success Criteria

### End-to-End Validation
- [ ] Complete pipeline executed: import -> scan -> analyze -> plan -> test -> generate -> review -> deploy
- [ ] Every step produced meaningful output (no empty results, no placeholder data)
- [ ] Total time from task creation to PR: under 15 minutes
- [ ] All AI responses in Chinese (for analysis conversation)
- [ ] Generated code is syntactically valid and follows injected coding standards
- [ ] Review caught at least one issue (proves review is not rubber-stamping)
- [ ] PR was created on GitHub with correct branch naming
- [ ] Full metrics table filled out with real data
- [ ] No unhandled errors in forge-core, ai-worker, or frontend console logs

### S15 Wrap-up
- [ ] File search (Ctrl+P) works in code browser
- [ ] Large files are truncated with "show more" option
- [ ] Empty states are handled for code browser, profile page, and task list

## 7. Risks and Rollback

| Risk | Mitigation |
|------|------------|
| One step fails and blocks the whole pipeline | Each step can be retried independently; Temporal provides workflow history for debugging |
| LLM API rate limits during full run | Fallback chain means if qwen3-max is rate-limited, it falls back to claude-sonnet-4, then gpt-4o |
| Total token cost is high for full pipeline | Estimate: ~200k tokens total. At qwen3 pricing this is ~$0.50; at Claude pricing ~$3. Budget is acceptable for validation. |
| Generated code does not compile | This is expected -- compilation check is a future constraint worker feature. For now, manual inspection is sufficient. |
| Demo repo is too simple to test edge cases | The demo repo is intentionally simple for the first validation. Complex repos will be tested in Phase 2b. |

---

**Estimated actual work**: 6-8 hours (3-4h for the E2E run + recording, 2-3h for S15 items, 1h for documentation)
