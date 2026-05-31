# SX-3 -- Wire Profile Scanning (ProfilerAgent -> Temporal -> Auto-trigger)

**Duration**: 1 day
**Priority**: P1 -- Profile data enriches all AI context; without it, code generation is blind to project structure
**Dependencies**: None (can be done in parallel with SX-1/SX-2)

---

## 1. Goal

Wire the profile scan button to actually scan the project's codebase via ProfilerAgent, and auto-trigger a scan when a project is first imported. Currently `TriggerScan` returns a fake `{"status": "scan_queued"}`.

## 2. Current State Analysis

### 2.1 What Exists

**Go side (forge-core)**:

- `profile/handler.go` (line 48-56): `TriggerScan` handler -- parses project ID, returns `{"status": "scan_queued"}` with a TODO comment
- `profile/service.go` (line 11-48): `Service` with `ListProfiles`, `GetProfile`, `SaveProfile` -- all working, backed by PostgreSQL
- `profile/repository.go` (line 60-69): `Upsert` method -- INSERT ON CONFLICT UPDATE, working
- `profile/model.go`: `ProfileEntry` struct with `ProjectID`, `ProfileKey`, `ProfileValue` (JSONB), `Version`, `ScannedAt`
- Route registered: `POST /api/projects/:id/profiles/scan` (via router.go)

**Python side (ai-worker)**:

- `activities/profile.py` (line 159-267): `scan_project_profile_activity` -- fully implemented, tested activity that:
  1. Fetches file tree from forge-core API (`GET /api/projects/{id}/code/tree`)
  2. Selects relevant files per dimension (api_catalog, db_schema, module_graph, architecture, business_rules)
  3. Fetches file contents via forge-core API (`GET /api/projects/{id}/code/file?path=...`)
  4. Runs `ProfilerAgent` per dimension (profiler.py)
  5. Saves results back via `PUT /api/projects/{id}/profiles/{key}` -- **this endpoint does not exist yet**
- `activities/profile.py` (line 16-22): `ALL_DIMENSIONS = ["api_catalog", "db_schema", "module_graph", "architecture", "business_rules"]`
- `agents/profiler.py` (line 75-90): `ProfilerAgent` -- builds system prompt with dimension-specific instructions, calls LLM, returns structured JSON

**What is missing**:

1. **No Temporal workflow** to wrap the `scan_project_profile_activity` -- need a `ProfileScanWorkflow`
2. **TriggerScan handler does not start any workflow** -- just returns fake response
3. **No auto-trigger on import** -- `project/service.go ImportFromGitHub` (line 275-305) only triggers `DetectTechStack`
4. **No PUT endpoint for saving profiles** -- `_save_profile` in profile.py (line 134-156) calls `PUT /api/projects/{id}/profiles/{key}` which is not registered
5. **Service struct has no Temporal client** -- `profile/service.go` only has a `repo` field

### 2.2 Data Flow to Wire

```
Frontend "Scan" button
  -> POST /api/projects/:id/profiles/scan
  -> profile.Handler.TriggerScan()
  -> profile.Service.TriggerScan()
  -> temporalClient.ExecuteWorkflow("ProfileScanWorkflow", ...)
  -> [Temporal] ProfileScanWorkflow runs on "forge-task-queue"
  -> [Temporal] calls scan_project_profile activity on "ai-worker" queue
  -> [Python] ProfilerAgent analyzes code per dimension
  -> [Python] saves results back via PUT /api/projects/:id/profiles/:key
  -> [SSE] broadcasts scan progress
```

## 3. Implementation Steps

### Step 3.1: Add Profile Save Endpoint (20 min)

The Python activity needs to save profile results back to forge-core. Add the PUT endpoint.

**File**: `forge-core/internal/module/profile/handler.go`

Add new handler:

```go
func (h *Handler) SaveProfile(c *gin.Context) {
    projectID, err := strconv.ParseInt(c.Param("id"), 10, 64)
    if err != nil {
        response.Fail(c, http.StatusBadRequest, "invalid project id")
        return
    }
    key := c.Param("key")
    if key == "" {
        response.Fail(c, http.StatusBadRequest, "profile key is required")
        return
    }

    var req struct {
        ProfileValue json.RawMessage `json:"profileValue" binding:"required"`
    }
    if err := c.ShouldBindJSON(&req); err != nil {
        response.Fail(c, http.StatusBadRequest, "invalid request: "+err.Error())
        return
    }

    entry, err := h.svc.SaveProfile(c.Request.Context(), projectID, key, req.ProfileValue)
    if err != nil {
        response.Fail(c, http.StatusInternalServerError, err.Error())
        return
    }
    response.OK(c, entry)
}
```

Note: `SaveProfile` already exists in `profile/service.go` (line 38-48) and `repository.go Upsert` (line 60-69). We just need the handler and route.

**File**: `forge-core/internal/router/router.go`

Add the route in the protected projects group. Find the existing profile routes and add:

```go
// Profile routes (find existing group with ListProfiles, GetProfile, TriggerScan)
projectGroup.PUT("/profiles/:key", deps.ProfileHandler.SaveProfile)
```

Need to add `import "encoding/json"` to handler.go if not already present.

### Step 3.2: Create ProfileScanWorkflow (30 min)

**File**: `forge-core/internal/temporal/workflow/profile_workflow.go` (new file)

```go
package workflow

import (
    "time"

    "go.temporal.io/sdk/temporal"
    "go.temporal.io/sdk/workflow"
)

// ProfileScanInput matches the Python ScanProfileInput dataclass.
type ProfileScanInput struct {
    ProjectID int64    `json:"project_id"`
    UserID    int64    `json:"user_id"`
    Keys      []string `json:"keys,omitempty"` // nil = all dimensions
}

// ProfileScanWorkflow orchestrates a project profile scan.
// The actual scanning runs on the ai-worker Python queue.
func ProfileScanWorkflow(ctx workflow.Context, input ProfileScanInput) (map[string]interface{}, error) {
    logger := workflow.GetLogger(ctx)
    logger.Info("ProfileScanWorkflow started", "project_id", input.ProjectID)

    aiCtx := workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
        TaskQueue:           "ai-worker",
        StartToCloseTimeout: 10 * time.Minute, // Profile scan can be slow (5 dimensions * LLM calls)
        RetryPolicy: &temporal.RetryPolicy{
            MaximumAttempts:    2,
            InitialInterval:    5 * time.Second,
            BackoffCoefficient: 2.0,
        },
    })

    var result map[string]interface{}
    err := workflow.ExecuteActivity(aiCtx, "scan_project_profile", input).Get(ctx, &result)
    if err != nil {
        logger.Error("profile scan activity failed", "error", err)
        return nil, err
    }

    logger.Info("ProfileScanWorkflow completed", "project_id", input.ProjectID)
    return result, nil
}
```

### Step 3.3: Register Workflow in Worker (10 min)

**File**: `forge-core/internal/temporal/worker.go`

Add workflow registration after the existing registrations (around line 32):

```go
w.RegisterWorkflowWithOptions(wf.ProfileScanWorkflow, workflow.RegisterOptions{
    Name: "ProfileScanWorkflow",
})
```

### Step 3.4: Wire TriggerScan to Temporal (30 min)

**File**: `forge-core/internal/module/profile/service.go`

Add Temporal client to the service struct:

```go
import (
    "context"
    "encoding/json"
    "errors"
    "fmt"
    "log/slog"

    "github.com/jackc/pgx/v5"
    "go.temporal.io/sdk/client"
)

type Service struct {
    repo           *Repository
    temporalClient client.Client // nil if Temporal unavailable
}

func NewService(repo *Repository, tc client.Client) *Service {
    return &Service{repo: repo, temporalClient: tc}
}
```

Add TriggerScan method:

```go
// TriggerScan starts a ProfileScanWorkflow via Temporal.
func (s *Service) TriggerScan(ctx context.Context, projectID, userID int64, keys []string) (string, error) {
    if s.temporalClient == nil {
        return "", fmt.Errorf("temporal not available")
    }

    workflowID := fmt.Sprintf("profile-scan-%d", projectID)
    input := map[string]interface{}{
        "project_id": projectID,
        "user_id":    userID,
        "keys":       keys,
    }

    we, err := s.temporalClient.ExecuteWorkflow(ctx, client.StartWorkflowOptions{
        ID:        workflowID,
        TaskQueue: "forge-task-queue",
    }, "ProfileScanWorkflow", input)
    if err != nil {
        return "", fmt.Errorf("start profile scan workflow: %w", err)
    }

    slog.Info("profile scan triggered", "project_id", projectID, "workflow_id", we.GetID())
    return we.GetID(), nil
}
```

**File**: `forge-core/internal/module/profile/handler.go`

Update TriggerScan handler:

```go
func (h *Handler) TriggerScan(c *gin.Context) {
    projectID, err := strconv.ParseInt(c.Param("id"), 10, 64)
    if err != nil {
        response.Fail(c, http.StatusBadRequest, "invalid project id")
        return
    }

    userID, _ := c.Get("user_id")

    // Parse optional request body for specific dimensions
    var req struct {
        Keys []string `json:"keys"` // optional, empty = full scan
    }
    _ = c.ShouldBindJSON(&req) // OK if no body -- means full scan

    workflowID, err := h.svc.TriggerScan(c.Request.Context(), projectID, userID.(int64), req.Keys)
    if err != nil {
        response.Fail(c, http.StatusInternalServerError, "failed to trigger scan: "+err.Error())
        return
    }

    response.OK(c, gin.H{
        "status":      "scan_started",
        "workflow_id": workflowID,
    })
}
```

### Step 3.5: Update Service Constructor Call Site (15 min)

Find where `profile.NewService` is called and pass the Temporal client.

**File**: `forge-core/cmd/forge-core/main.go` (or wherever service initialization happens)

Search for `profile.NewService(` and update:

```go
// Before:
profileSvc := profile.NewService(profileRepo)

// After:
profileSvc := profile.NewService(profileRepo, temporalClient)
```

If the constructor is in a dependency injection file, find it:

```bash
grep -r "profile.NewService" forge-core/
```

### Step 3.6: Add Auto-trigger on Project Import (30 min)

**File**: `forge-core/internal/module/project/service.go`

In `ImportFromGitHub` (line 275-305), after the async `DetectTechStack` goroutine (line 292-296), add profile scan trigger:

```go
// Existing code (line 292-296):
go func(pid int64) {
    bgCtx := context.Background()
    if err := s.DetectTechStack(bgCtx, pid, tenantID, userID); err != nil {
        slog.Warn("tech stack detection failed", "project_id", pid, "error", err)
    }
}(brief.ID)

// NEW: Also trigger profile scan after import
go func(pid int64) {
    bgCtx := context.Background()
    if err := s.TriggerProfileScan(bgCtx, pid, userID); err != nil {
        slog.Warn("auto profile scan failed", "project_id", pid, "error", err)
    }
}(brief.ID)
```

Add `TriggerProfileScan` method to project service. This requires the project service to have access to Temporal client OR to the profile service.

**Option A**: Add Temporal client to project service (simpler):

```go
type Service struct {
    repo           *Repository
    authSvc        AuthTokenProvider
    ws             WorkspaceProvider
    temporalClient client.Client // add this
}

func NewService(repo *Repository, authSvc AuthTokenProvider, ws WorkspaceProvider, tc client.Client) *Service {
    return &Service{repo: repo, authSvc: authSvc, ws: ws, temporalClient: tc}
}

func (s *Service) TriggerProfileScan(ctx context.Context, projectID, userID int64) error {
    if s.temporalClient == nil {
        return nil // skip silently
    }
    workflowID := fmt.Sprintf("profile-scan-%d-import", projectID)
    _, err := s.temporalClient.ExecuteWorkflow(ctx, client.StartWorkflowOptions{
        ID:        workflowID,
        TaskQueue: "forge-task-queue",
    }, "ProfileScanWorkflow", map[string]interface{}{
        "project_id": projectID,
        "user_id":    userID,
    })
    if err != nil {
        return fmt.Errorf("start profile scan: %w", err)
    }
    slog.Info("auto-triggered profile scan on import", "project_id", projectID)
    return nil
}
```

**Option B**: Have the profile handler/service expose a method and inject it (more decoupled but more wiring).

Recommend Option A for simplicity.

### Step 3.7: Register Python Activity in Worker (5 min)

**File**: `ai-worker/src/main.py` (or wherever the Temporal worker is set up)

Verify `scan_project_profile` activity is registered:

```python
from src.activities.profile import scan_project_profile_activity

# In the worker setup:
worker = Worker(
    client,
    task_queue="ai-worker",
    activities=[
        analyze_requirement_activity,
        plan_task_activity,
        generate_code_activity,
        generate_test_cases_activity,
        review_code_activity,
        scan_project_profile_activity,  # <-- ensure this is listed
    ],
)
```

Check by searching:

```bash
grep -r "scan_project_profile" ai-worker/src/
```

### Step 3.8: Verify Integration - ContextBuilder Loads Profiles (15 min)

The `ContextBuilder.build()` already fetches profiles (builder.py line 133-145):

```python
# Fetch project profiles (AI memory) -- GET /api/projects/{project_id}/profiles
resp = await self._client.get(f"/api/projects/{project_id}/profiles")
if resp.status_code == 200:
    data = resp.json().get("data", {})
    profiles_list = data.get("profiles", [])
    for p in profiles_list:
        key = p.get("profileKey", "")
        value = p.get("profileValue", {})
        if key and value:
            ctx.project_profiles[key] = value
```

And `to_system_prompt()` already renders profiles (builder.py line 46-67):

```python
if self.project_profiles:
    profile_labels = {
        "api_catalog": "API endpoint catalog",
        "db_schema": "Database schema",
        "module_graph": "Module dependency graph",
        ...
    }
    # Renders each profile as a labeled section
```

So once profiles are saved by the scan activity, all subsequent AI calls automatically include them. No additional wiring needed.

### Step 3.9: Frontend -- Show Real Scan Progress (45 min)

**Location**: Find the profile page component. It is likely at `forge-portal/app/(dashboard)/projects/[id]/profile/page.tsx` or similar.

```bash
find forge-portal -name "*.tsx" | xargs grep -l "TriggerScan\|profiles/scan\|profile.*scan" 2>/dev/null
```

The scan button should:

1. Call `POST /api/projects/${projectId}/profiles/scan`
2. Show loading state with dimension-by-dimension progress
3. On completion, refresh the profile list

```typescript
// In the profile page component:
const [isScanning, setIsScanning] = useState(false);

const handleScan = async () => {
  setIsScanning(true);
  try {
    await api.post(`/projects/${projectId}/profiles/scan`);
    // Poll for results or use SSE
    // Simple approach: wait a few seconds then refresh
    setTimeout(async () => {
      await refetchProfiles();
      setIsScanning(false);
    }, 30000); // Profile scan takes ~30s for 5 dimensions
  } catch {
    setIsScanning(false);
    toast.error("Failed to start profile scan");
  }
};
```

Better approach: poll the profiles endpoint until new `scannedAt` timestamps appear.

## 4. Files Modified

| File | Change | Type |
|------|--------|------|
| `forge-core/internal/temporal/workflow/profile_workflow.go` | New ProfileScanWorkflow | NEW |
| `forge-core/internal/temporal/worker.go` | Register ProfileScanWorkflow | MODIFY |
| `forge-core/internal/module/profile/service.go` | Add temporalClient, TriggerScan method | MODIFY |
| `forge-core/internal/module/profile/handler.go` | Wire TriggerScan to service, add SaveProfile handler | MODIFY |
| `forge-core/internal/router/router.go` | Add PUT /profiles/:key route | MODIFY |
| `forge-core/internal/module/project/service.go` | Add TriggerProfileScan, call on import | MODIFY |
| `forge-core/cmd/forge-core/main.go` | Update NewService calls with Temporal client | MODIFY |
| Frontend profile page | Show real scan progress | MODIFY |

## 5. Acceptance Criteria

- [ ] `POST /api/projects/:id/profiles/scan` starts a real Temporal workflow (returns workflow_id)
- [ ] Temporal Web UI (localhost:8233) shows ProfileScanWorkflow running/completed
- [ ] After scan completes, `GET /api/projects/:id/profiles` returns 5 dimensions (api_catalog, db_schema, module_graph, architecture, business_rules)
- [ ] Each profile dimension has non-empty `profileValue` with structured JSON
- [ ] Importing a new GitHub project automatically triggers profile scan (check AI Worker logs)
- [ ] ContextBuilder includes profiles in system prompt for all agents (check logs for "profiles=5" or similar)
- [ ] Frontend scan button shows loading state and displays results after completion
- [ ] Profile data persists across requests (saved in PostgreSQL, not just in memory)

## 6. Risks and Rollback

| Risk | Mitigation |
|------|------------|
| Profile scan takes too long (5 dimensions x 30s each) | Workflow timeout is 10 min; dimensions run sequentially but can be parallelized later |
| File tree fetch fails (project not cloned yet) | Activity falls back gracefully: `output.errors["_general"] = "Could not fetch file tree"` (profile.py line 181-182) |
| LLM returns unparseable profile JSON | BaseAgent._parse_json has robust fallback; worst case returns empty dict which is skipped |
| PUT /profiles/:key auth issue for AI Worker | Same fix as SX-2: service token or internal API group |
| Auto-scan on import races with clone | DetectTechStack also needs the repo; both use goroutines; profile scan will fail gracefully if tree is not ready, can be manually retriggered |
| Workflow ID collision on re-scan | Use `fmt.Sprintf("profile-scan-%d", projectID)` -- Temporal will reject duplicate; use `WorkflowIDReusePolicy` to allow re-running |

---

**Estimated actual work**: 4-5 hours (new workflow + wiring + frontend updates)
