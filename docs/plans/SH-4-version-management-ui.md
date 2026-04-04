# SH-4 -- Version Management UI Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a version management system that groups tasks into release versions, tracks version status through PLANNING -> RELEASED lifecycle, detects file conflicts between tasks in the same version, and provides a release flow with git tag creation.

**Architecture:** New `version` module in forge-core (model/repository/service/handler), new migration for `pipeline.versions` and `pipeline.version_tasks` tables, two new frontend pages (version list + version detail), and integration with the existing task and pipeline modules. Conflict detection runs on `touched_files` overlap between tasks in the same version.

**Tech Stack:** Go 1.22 + pgx + Gin, Next.js 15 + shadcn/ui + Tailwind CSS 4

**Dependencies:** S7 (DevOps/pipeline schema), S9 (task_nodes with files field)

**Duration:** 2 days

---

## File Structure

### Go Backend

```
forge-core/
+-- migrations/
|   +-- 016_versions.sql                       # NEW: versions + version_tasks tables
+-- internal/module/version/
|   +-- model.go                               # NEW: Version, VersionTask structs
|   +-- repository.go                          # NEW: CRUD + conflict detection queries
|   +-- service.go                             # NEW: version lifecycle + release logic
|   +-- handler.go                             # NEW: HTTP endpoints
+-- internal/router/router.go                  # MODIFY: register version routes
+-- cmd/forge-core/main.go                     # MODIFY: initialize version module
```

### Frontend

```
forge-portal/
+-- app/(dashboard)/projects/[id]/
|   +-- versions/page.tsx                      # NEW: version list page
|   +-- versions/[vid]/page.tsx                # NEW: version detail page
+-- components/versions/
|   +-- create-version-dialog.tsx              # NEW: create version dialog
|   +-- version-status-badge.tsx               # NEW: status badge with colors
|   +-- version-task-list.tsx                  # NEW: tasks in a version with conflict indicators
|   +-- release-confirmation-dialog.tsx        # NEW: release confirmation with tag preview
+-- components/project-sidebar.tsx             # MODIFY: add "Versions" nav item
+-- lib/versions.ts                            # NEW: version API client + types
```

---

## Day 1: Version List Page + Backend CRUD

### Task 1: Database Migration

**Files:**
- Create: `forge-core/migrations/016_versions.sql`

- [ ] **Step 1: Create migration file**

`forge-core/migrations/016_versions.sql`:

```sql
-- SH-4: Version management for release grouping
CREATE TABLE IF NOT EXISTS pipeline.versions (
    id              BIGSERIAL PRIMARY KEY,
    tenant_id       BIGINT NOT NULL,
    project_id      BIGINT NOT NULL REFERENCES engine.projects(id),
    version_number  VARCHAR(50) NOT NULL,            -- e.g. v1.0, v1.1, v2.0
    description     TEXT,
    status          VARCHAR(20) NOT NULL DEFAULT 'PLANNING',  -- PLANNING / IN_PROGRESS / TESTING / RELEASED / CANCELLED
    tag_name        VARCHAR(100),                    -- git tag name when released
    tag_sha         VARCHAR(40),                     -- git tag commit SHA
    released_at     TIMESTAMPTZ,
    released_by     BIGINT,
    created_by      BIGINT NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_versions_project ON pipeline.versions(project_id);
CREATE UNIQUE INDEX IF NOT EXISTS idx_versions_project_number ON pipeline.versions(project_id, version_number);

-- Junction table: which tasks belong to which version
CREATE TABLE IF NOT EXISTS pipeline.version_tasks (
    id              BIGSERIAL PRIMARY KEY,
    version_id      BIGINT NOT NULL REFERENCES pipeline.versions(id) ON DELETE CASCADE,
    task_id         BIGINT NOT NULL REFERENCES engine.tasks(id),
    added_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(version_id, task_id)
);

CREATE INDEX IF NOT EXISTS idx_version_tasks_version ON pipeline.version_tasks(version_id);
CREATE INDEX IF NOT EXISTS idx_version_tasks_task ON pipeline.version_tasks(task_id);
```

- [ ] **Step 2: Verify build**

```bash
cd forge-core && go build ./cmd/forge-core
```

- [ ] **Step 3: Commit**

```bash
git add forge-core/migrations/016_versions.sql
git commit -m "feat(sh4): add versions and version_tasks tables for release management"
```

---

### Task 2: Go Backend -- Version Module

**Files:**
- Create: `forge-core/internal/module/version/model.go`
- Create: `forge-core/internal/module/version/repository.go`
- Create: `forge-core/internal/module/version/service.go`
- Create: `forge-core/internal/module/version/handler.go`
- Modify: `forge-core/internal/router/router.go`
- Modify: `forge-core/cmd/forge-core/main.go`

**IMPORTANT**: Read `forge-core/internal/module/pipeline/` (model.go, repository.go, service.go, handler.go) to follow the existing module pattern.

- [ ] **Step 1: Create model.go**

```go
package version

import "time"

// Status constants
const (
    StatusPlanning   = "PLANNING"
    StatusInProgress = "IN_PROGRESS"
    StatusTesting    = "TESTING"
    StatusReleased   = "RELEASED"
    StatusCancelled  = "CANCELLED"
)

type Version struct {
    ID            int64      `json:"id"`
    TenantID      int64      `json:"tenantId"`
    ProjectID     int64      `json:"projectId"`
    VersionNumber string     `json:"versionNumber"`
    Description   *string    `json:"description,omitempty"`
    Status        string     `json:"status"`
    TagName       *string    `json:"tagName,omitempty"`
    TagSHA        *string    `json:"tagSha,omitempty"`
    ReleasedAt    *time.Time `json:"releasedAt,omitempty"`
    ReleasedBy    *int64     `json:"releasedBy,omitempty"`
    CreatedBy     int64      `json:"createdBy"`
    CreatedAt     time.Time  `json:"createdAt"`
    UpdatedAt     time.Time  `json:"updatedAt"`
    // Computed fields (not stored)
    TaskCount     int        `json:"taskCount"`
    CompletedCount int       `json:"completedCount"`
}

type VersionTask struct {
    VersionID int64     `json:"versionId"`
    TaskID    int64     `json:"taskId"`
    AddedAt   time.Time `json:"addedAt"`
    // Joined fields from engine.tasks
    TaskTitle  *string `json:"taskTitle,omitempty"`
    TaskStatus string  `json:"taskStatus,omitempty"`
}

type FileConflict struct {
    FilePath string  `json:"filePath"`
    TaskIDs  []int64 `json:"taskIds"`
    TaskTitles []string `json:"taskTitles"`
}

type CreateVersionRequest struct {
    VersionNumber string `json:"versionNumber" binding:"required,max=50"`
    Description   string `json:"description"`
}

type AddTasksRequest struct {
    TaskIDs []int64 `json:"taskIds" binding:"required,min=1"`
}

type ReleaseRequest struct {
    TagName string `json:"tagName"` // optional override, defaults to version_number
}

type VersionDetailResponse struct {
    Version   Version       `json:"version"`
    Tasks     []VersionTask `json:"tasks"`
    Conflicts []FileConflict `json:"conflicts"`
}

type VersionListResponse struct {
    Versions []Version `json:"versions"`
    Total    int64     `json:"total"`
}
```

- [ ] **Step 2: Create repository.go**

```go
// Create -- insert a new version
func (r *Repository) Create(ctx context.Context, v *Version) error

// ListByProject -- list versions for a project, ordered by created_at DESC
func (r *Repository) ListByProject(ctx context.Context, projectID int64) ([]Version, error)

// GetByID -- get version with task counts
func (r *Repository) GetByID(ctx context.Context, versionID int64) (*Version, error)

// UpdateStatus -- update version status
func (r *Repository) UpdateStatus(ctx context.Context, versionID int64, status string) error

// SetReleaseInfo -- set tag_name, tag_sha, released_at, released_by
func (r *Repository) SetReleaseInfo(ctx context.Context, versionID int64, tagName, tagSHA string, releasedBy int64) error

// AddTasks -- add tasks to a version
func (r *Repository) AddTasks(ctx context.Context, versionID int64, taskIDs []int64) error

// RemoveTask -- remove a task from a version
func (r *Repository) RemoveTask(ctx context.Context, versionID int64, taskID int64) error

// ListTasks -- list tasks in a version with joined task info
func (r *Repository) ListTasks(ctx context.Context, versionID int64) ([]VersionTask, error)

// DetectConflicts -- find file overlaps between tasks in a version
func (r *Repository) DetectConflicts(ctx context.Context, versionID int64) ([]FileConflict, error)
```

Key SQL for `ListByProject` (with task counts):
```sql
SELECT v.*,
    COALESCE(tc.task_count, 0) as task_count,
    COALESCE(tc.completed_count, 0) as completed_count
FROM pipeline.versions v
LEFT JOIN (
    SELECT vt.version_id,
        COUNT(*) as task_count,
        COUNT(*) FILTER (WHERE t.status = 'COMPLETED') as completed_count
    FROM pipeline.version_tasks vt
    JOIN engine.tasks t ON t.id = vt.task_id
    GROUP BY vt.version_id
) tc ON tc.version_id = v.id
WHERE v.project_id = $1
ORDER BY v.created_at DESC
```

Key SQL for `DetectConflicts` (cross-reference task_nodes files):
```sql
SELECT tn1.task_id AS task_id_a, tn2.task_id AS task_id_b,
    jsonb_array_elements_text(tn1.files) AS file_path
FROM pipeline.version_tasks vt1
JOIN engine.task_nodes tn1 ON tn1.task_id = vt1.task_id
JOIN pipeline.version_tasks vt2 ON vt2.version_id = vt1.version_id AND vt2.task_id > vt1.task_id
JOIN engine.task_nodes tn2 ON tn2.task_id = vt2.task_id
WHERE vt1.version_id = $1
AND tn1.files ?| ARRAY(SELECT jsonb_array_elements_text(tn2.files))
```

Note: The conflict query uses task_nodes.files JSONB arrays from S9. If task_nodes do not exist for some tasks, those tasks are simply excluded from conflict detection.

- [ ] **Step 3: Create service.go**

```go
type Service struct {
    repo      *Repository
    projectRepo ProjectGetter // interface to get project info
    ghAdapter   GitHubAdapter  // interface for tag creation
}

// CreateVersion -- validate version_number uniqueness, insert
func (s *Service) CreateVersion(ctx context.Context, projectID, tenantID, userID int64, req CreateVersionRequest) (*Version, error)

// ListVersions -- list with task counts
func (s *Service) ListVersions(ctx context.Context, projectID int64) ([]Version, error)

// GetVersionDetail -- version + tasks + conflicts
func (s *Service) GetVersionDetail(ctx context.Context, versionID int64) (*VersionDetailResponse, error)

// AddTasks -- add tasks, trigger conflict re-check
func (s *Service) AddTasks(ctx context.Context, versionID int64, taskIDs []int64) error

// RemoveTask -- remove task from version
func (s *Service) RemoveTask(ctx context.Context, versionID, taskID int64) error

// ReleaseVersion -- check all tasks COMPLETED, create git tag, update status
func (s *Service) ReleaseVersion(ctx context.Context, versionID, userID int64, req ReleaseRequest) error
```

`ReleaseVersion` logic:
1. Get version + tasks
2. Check all tasks have status `COMPLETED` -- return error if any not completed
3. Determine tag name: `req.TagName` or `v.VersionNumber`
4. Call GitHub adapter: `CreateTag(ctx, owner, repo, tagName, defaultBranch)`
5. Update version: status=RELEASED, tag_name, tag_sha, released_at=now, released_by=userID

- [ ] **Step 4: Create handler.go**

8 endpoints:
```go
// GET /api/projects/:id/versions -- list versions
func (h *Handler) ListVersions(c *gin.Context)

// POST /api/projects/:id/versions -- create version
func (h *Handler) CreateVersion(c *gin.Context)

// GET /api/projects/:id/versions/:vid -- get version detail
func (h *Handler) GetVersionDetail(c *gin.Context)

// PUT /api/projects/:id/versions/:vid/status -- update status
func (h *Handler) UpdateStatus(c *gin.Context)

// POST /api/projects/:id/versions/:vid/tasks -- add tasks
func (h *Handler) AddTasks(c *gin.Context)

// DELETE /api/projects/:id/versions/:vid/tasks/:taskId -- remove task
func (h *Handler) RemoveTask(c *gin.Context)

// POST /api/projects/:id/versions/:vid/release -- trigger release
func (h *Handler) ReleaseVersion(c *gin.Context)

// GET /api/projects/:id/versions/:vid/conflicts -- get conflicts only
func (h *Handler) GetConflicts(c *gin.Context)
```

- [ ] **Step 5: Register routes in router.go**

Add `VersionHandler *version.Handler` to `Deps` struct, then:

```go
// Versions
if deps.VersionHandler != nil {
    protected.GET("/projects/:id/versions", deps.VersionHandler.ListVersions)
    protected.POST("/projects/:id/versions", deps.VersionHandler.CreateVersion)
    protected.GET("/projects/:id/versions/:vid", deps.VersionHandler.GetVersionDetail)
    protected.PUT("/projects/:id/versions/:vid/status", deps.VersionHandler.UpdateStatus)
    protected.POST("/projects/:id/versions/:vid/tasks", deps.VersionHandler.AddTasks)
    protected.DELETE("/projects/:id/versions/:vid/tasks/:taskId", deps.VersionHandler.RemoveTask)
    protected.POST("/projects/:id/versions/:vid/release", deps.VersionHandler.ReleaseVersion)
    protected.GET("/projects/:id/versions/:vid/conflicts", deps.VersionHandler.GetConflicts)
}
```

- [ ] **Step 6: Initialize module in main.go**

Follow the pattern of other modules: create repository -> service -> handler -> inject into Deps.

- [ ] **Step 7: Verify build**

```bash
cd forge-core && go build ./cmd/forge-core
```

- [ ] **Step 8: Commit**

```bash
git add forge-core/
git commit -m "feat(sh4): add version module with CRUD, conflict detection, and release flow"
```

---

### Task 3: Frontend -- Version List Page

**Files:**
- Create: `forge-portal/lib/versions.ts`
- Create: `forge-portal/components/versions/version-status-badge.tsx`
- Create: `forge-portal/components/versions/create-version-dialog.tsx`
- Create: `forge-portal/app/(dashboard)/projects/[id]/versions/page.tsx`
- Modify: `forge-portal/components/project-sidebar.tsx`

- [ ] **Step 1: Create API client lib/versions.ts**

```typescript
import { api } from "./api";

export interface Version {
  id: number;
  tenantId: number;
  projectId: number;
  versionNumber: string;
  description?: string;
  status: string;
  tagName?: string;
  tagSha?: string;
  releasedAt?: string;
  releasedBy?: number;
  createdBy: number;
  createdAt: string;
  updatedAt: string;
  taskCount: number;
  completedCount: number;
}

export interface VersionTask {
  versionId: number;
  taskId: number;
  addedAt: string;
  taskTitle?: string;
  taskStatus: string;
}

export interface FileConflict {
  filePath: string;
  taskIds: number[];
  taskTitles: string[];
}

export interface VersionDetail {
  version: Version;
  tasks: VersionTask[];
  conflicts: FileConflict[];
}

export const VERSION_STATUS_CONFIG: Record<string, { label: string; color: string; bgClass: string }> = {
  PLANNING:    { label: "Planning",    color: "text-blue-400",   bgClass: "bg-blue-500/20" },
  IN_PROGRESS: { label: "In Progress", color: "text-purple-400", bgClass: "bg-purple-500/20" },
  TESTING:     { label: "Testing",     color: "text-amber-400",  bgClass: "bg-amber-500/20" },
  RELEASED:    { label: "Released",    color: "text-green-400",  bgClass: "bg-green-500/20" },
  CANCELLED:   { label: "Cancelled",   color: "text-gray-400",   bgClass: "bg-gray-500/20" },
};

export async function listVersions(projectId: number): Promise<Version[]> {
  const res = await api.get<{ versions: Version[] }>(`/projects/${projectId}/versions`);
  return res.versions || [];
}

export async function createVersion(projectId: number, data: { versionNumber: string; description: string }): Promise<Version> {
  return api.post(`/projects/${projectId}/versions`, data);
}

export async function getVersionDetail(projectId: number, versionId: number): Promise<VersionDetail> {
  return api.get(`/projects/${projectId}/versions/${versionId}`);
}

export async function addTasksToVersion(projectId: number, versionId: number, taskIds: number[]): Promise<void> {
  return api.post(`/projects/${projectId}/versions/${versionId}/tasks`, { taskIds });
}

export async function removeTaskFromVersion(projectId: number, versionId: number, taskId: number): Promise<void> {
  return api.delete(`/projects/${projectId}/versions/${versionId}/tasks/${taskId}`);
}

export async function releaseVersion(projectId: number, versionId: number, tagName?: string): Promise<void> {
  return api.post(`/projects/${projectId}/versions/${versionId}/release`, { tagName });
}
```

- [ ] **Step 2: Create VersionStatusBadge component**

`forge-portal/components/versions/version-status-badge.tsx`:

Uses `VERSION_STATUS_CONFIG` to render a colored Badge (shadcn/ui Badge component):
- PLANNING = blue dot + "Planning"
- IN_PROGRESS = purple dot + "In Progress"
- TESTING = amber dot + "Testing"
- RELEASED = green dot + "Released"
- CANCELLED = gray dot + "Cancelled"

- [ ] **Step 3: Create CreateVersionDialog**

`forge-portal/components/versions/create-version-dialog.tsx`:

- shadcn Dialog with form
- Version number input: prefix "v" + two number fields (major.minor) -- or free text with validation `^v\d+\.\d+(\.\d+)?$`
- Description textarea
- Submit calls `createVersion()` then refreshes parent
- Pattern: follow `create-project-dialog.tsx` for dialog/form structure

- [ ] **Step 4: Create version list page**

`forge-portal/app/(dashboard)/projects/[id]/versions/page.tsx`:

```
+---------------------------------------------------------------+
|  Version Management                    [+ Create Version]      |
+---------------------------------------------------------------+
| Version     | Status      | Tasks     | Progress    | Created  |
|-------------|-------------|-----------|-------------|----------|
| v1.2        | IN_PROGRESS | 5 tasks   | ====--  60% | 2d ago   |
| v1.1        | RELEASED    | 3 tasks   | ======  100%| 5d ago   |
| v1.0        | RELEASED    | 8 tasks   | ======  100%| 2w ago   |
+---------------------------------------------------------------+
```

Table columns:
- Version number (link to detail page)
- Status badge (VersionStatusBadge component)
- Task count
- Progress bar (completedCount / taskCount)
- Created date (relative time)

Row click navigates to `/projects/[id]/versions/[vid]`.

- [ ] **Step 5: Update ProjectSidebar**

Read `forge-portal/components/project-sidebar.tsx`, add navigation item:

```tsx
{ icon: Tags, label: "Versions", href: `/projects/${projectId}/versions` },
```

Place after "Changes" / "PR" and before "Settings". Use `Tags` icon from lucide-react.

- [ ] **Step 6: Verify frontend build**

```bash
cd forge-portal && npm run build
```

- [ ] **Step 7: Commit**

```bash
git add forge-portal/
git commit -m "feat(sh4): add version list page with status badges and progress tracking"
```

---

## Day 2: Version Detail Page + Release Flow

### Task 4: Frontend -- Version Detail Page

**Files:**
- Create: `forge-portal/components/versions/version-task-list.tsx`
- Create: `forge-portal/components/versions/release-confirmation-dialog.tsx`
- Create: `forge-portal/app/(dashboard)/projects/[id]/versions/[vid]/page.tsx`

- [ ] **Step 1: Create VersionTaskList component**

`forge-portal/components/versions/version-task-list.tsx`:

Displays tasks within a version as a table/list:
- Columns: task title, status badge, conflict indicator, added date
- Conflict indicator: if task appears in `conflicts[]`, show AlertTriangle icon (amber) with tooltip listing conflicting files
- Status colors: reuse task status colors from existing task components
- "Remove from version" button (trash icon, requires confirmation)
- "Add Tasks" button at top to add existing project tasks

- [ ] **Step 2: Create ReleaseConfirmationDialog**

`forge-portal/components/versions/release-confirmation-dialog.tsx`:

shadcn AlertDialog with:
- Version number prominently displayed
- Git tag preview: "This will create git tag `v1.2` on branch `main`"
- Tag name override input (optional)
- Warning if any tasks not COMPLETED (with list of incomplete tasks)
- Conflicts summary (if any file overlaps exist)
- "Release Version" button -- disabled if incomplete tasks exist
- Loading state during release API call
- On success: show toast + redirect to version list

- [ ] **Step 3: Create version detail page**

`forge-portal/app/(dashboard)/projects/[id]/versions/[vid]/page.tsx`:

Layout:
```
+---------------------------------------------------------------+
| <- Back to versions                                            |
|                                                                |
| v1.2                                 [Status Badge]            |
| Description: Feature batch for Q2 release                      |
|                                                                |
+---------------------------------------------------------------+
| [Conflicts Warning Card]   (only if conflicts.length > 0)     |
| ! 2 file conflicts detected between tasks                     |
|   - src/service/user.go: Task #12, Task #15                   |
|   - src/model/user.go: Task #12, Task #18                     |
+---------------------------------------------------------------+
|                                                                |
| Tasks (5)                              [+ Add Tasks]           |
| +-----------------------------------------------------------+ |
| | VersionTaskList component                                  | |
| +-----------------------------------------------------------+ |
|                                                                |
| [Release Version]  (disabled until all tasks COMPLETED)        |
+---------------------------------------------------------------+
```

Components used:
- `VersionStatusBadge` for status
- `VersionTaskList` for task list
- `ReleaseConfirmationDialog` triggered by "Release Version" button
- Conflict warning cards: for each `FileConflict`, show the file path and which tasks overlap
- Breadcrumb navigation: Projects > {name} > Versions > v1.2

- [ ] **Step 4: Verify frontend build**

```bash
cd forge-portal && npm run build
```

- [ ] **Step 5: Commit**

```bash
git add forge-portal/
git commit -m "feat(sh4): add version detail page with conflict cards and release flow"
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

- [ ] **Step 3: End-to-end verification checklist**

1. Start all services
2. Navigate to a project with completed tasks
3. Click "Versions" in sidebar
4. Create a new version "v1.0" with description
5. Navigate to version detail page
6. Add tasks to the version
7. Verify task count and progress bar update
8. If tasks have overlapping files in task_nodes, verify conflict warning appears
9. Complete all tasks -> "Release Version" button becomes enabled
10. Click release -> confirm dialog shows git tag preview
11. Confirm release -> version status changes to RELEASED
12. Verify git tag was created on GitHub (check via GitHub adapter)

- [ ] **Step 4: Commit**

```bash
git add -A
git commit -m "feat(sh4): complete version management UI with release flow and conflict detection"
```

---

## Data Structures

### API Endpoints

| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/projects/:id/versions` | List versions with task counts |
| POST | `/api/projects/:id/versions` | Create new version |
| GET | `/api/projects/:id/versions/:vid` | Version detail + tasks + conflicts |
| PUT | `/api/projects/:id/versions/:vid/status` | Update version status |
| POST | `/api/projects/:id/versions/:vid/tasks` | Add tasks to version |
| DELETE | `/api/projects/:id/versions/:vid/tasks/:taskId` | Remove task from version |
| POST | `/api/projects/:id/versions/:vid/release` | Release version (create tag) |
| GET | `/api/projects/:id/versions/:vid/conflicts` | Get file conflicts only |

### Database Schema

```
pipeline.versions
+-- id (BIGSERIAL PK)
+-- tenant_id (BIGINT NOT NULL)
+-- project_id (BIGINT NOT NULL -> engine.projects)
+-- version_number (VARCHAR(50) NOT NULL, UNIQUE per project)
+-- description (TEXT)
+-- status (VARCHAR(20): PLANNING/IN_PROGRESS/TESTING/RELEASED/CANCELLED)
+-- tag_name (VARCHAR(100))
+-- tag_sha (VARCHAR(40))
+-- released_at (TIMESTAMPTZ)
+-- released_by (BIGINT)
+-- created_by (BIGINT NOT NULL)
+-- created_at, updated_at (TIMESTAMPTZ)

pipeline.version_tasks
+-- id (BIGSERIAL PK)
+-- version_id (BIGINT -> pipeline.versions, CASCADE)
+-- task_id (BIGINT -> engine.tasks)
+-- added_at (TIMESTAMPTZ)
+-- UNIQUE(version_id, task_id)
```

---

## Acceptance Criteria

- [ ] Create version with version number (v{major}.{minor}) and description
- [ ] Version list shows status badge, task count, progress bar, created date
- [ ] Version detail shows tasks with their statuses
- [ ] File conflict detection works (tasks with overlapping files in task_nodes show warnings)
- [ ] "Release Version" button disabled until all tasks COMPLETED
- [ ] Release confirmation dialog shows git tag preview
- [ ] Release creates git tag on GitHub via adapter
- [ ] Version status transitions: PLANNING -> IN_PROGRESS -> TESTING -> RELEASED
- [ ] Sidebar navigation includes "Versions" entry
- [ ] `go build` + `npm run build` pass
