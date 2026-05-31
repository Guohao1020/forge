# SH-3a — 版本数据模型 + CRUD API

## 目标

引入版本管理模型，让多个任务可以归属到同一个版本（如 v1.2.0），为后续的 VersionOrchestrator（SH-3b）并发协调奠定数据基础：

1. **project_versions 表** — 版本主表（id, version, status, description, released_at）
2. **tasks 表扩展** — 添加 version_id、conflict_status、blocked_by、touched_files 字段
3. **Go module** — module/version/ 完整 CRUD（model, repository, service, handler）
4. **前端 API 客户端** — forge-portal/lib/versions.ts

## 前置依赖

- 无外部依赖（纯数据模型层，不依赖 SH-1/SH-2）
- PostgreSQL engine schema 已存在
- forge-core Router 模式已建立

## 工期

2 天

---

## Day 1 — 数据库迁移 + Go 模型层

### 1.1 新建迁移文件 `forge-core/migrations/016_project_versions.sql`

```sql
-- ============================================================
-- 016_project_versions.sql
-- Version management for coordinating multiple tasks
-- ============================================================

-- 1. Version table
CREATE TABLE IF NOT EXISTS engine.project_versions (
    id              BIGSERIAL PRIMARY KEY,
    tenant_id       BIGINT NOT NULL,
    project_id      BIGINT NOT NULL REFERENCES engine.projects(id) ON DELETE CASCADE,
    version         VARCHAR(50) NOT NULL,  -- semver: v1.0.0, v1.2.0-rc.1
    status          VARCHAR(20) NOT NULL DEFAULT 'PLANNING',
    description     TEXT DEFAULT '',
    released_at     TIMESTAMPTZ,
    created_by      BIGINT NOT NULL REFERENCES auth.users(id),
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    -- Unique constraint: one version string per project
    CONSTRAINT uq_project_version UNIQUE (project_id, version)
);

-- Indexes
CREATE INDEX IF NOT EXISTS idx_project_versions_project ON engine.project_versions(project_id);
CREATE INDEX IF NOT EXISTS idx_project_versions_tenant ON engine.project_versions(tenant_id);
CREATE INDEX IF NOT EXISTS idx_project_versions_status ON engine.project_versions(status);

-- 2. Extend tasks table for version coordination
ALTER TABLE engine.tasks
    ADD COLUMN IF NOT EXISTS version_id       BIGINT REFERENCES engine.project_versions(id) ON DELETE SET NULL,
    ADD COLUMN IF NOT EXISTS conflict_status   VARCHAR(20) DEFAULT 'NONE',
    ADD COLUMN IF NOT EXISTS blocked_by        BIGINT[] DEFAULT '{}',
    ADD COLUMN IF NOT EXISTS touched_files     TEXT[] DEFAULT '{}';

CREATE INDEX IF NOT EXISTS idx_tasks_version_id ON engine.tasks(version_id);
CREATE INDEX IF NOT EXISTS idx_tasks_conflict_status ON engine.tasks(conflict_status);

-- Comment documentation
COMMENT ON TABLE engine.project_versions IS '项目版本管理 — 多任务协调的版本容器';
COMMENT ON COLUMN engine.project_versions.version IS '语义化版本号，如 v1.0.0, v1.2.0-rc.1';
COMMENT ON COLUMN engine.project_versions.status IS 'PLANNING / ACTIVE / FROZEN / RELEASED / CANCELLED';
COMMENT ON COLUMN engine.tasks.version_id IS '关联版本，NULL 表示独立任务';
COMMENT ON COLUMN engine.tasks.conflict_status IS 'NONE / WARNING / BLOCKED / RESOLVED';
COMMENT ON COLUMN engine.tasks.blocked_by IS '被哪些任务 ID 阻塞（文件冲突）';
COMMENT ON COLUMN engine.tasks.touched_files IS '此任务修改的文件路径列表';
```

### 1.2 新建 `forge-core/internal/module/version/model.go`

```go
package version

import (
	"time"
)

// Version status constants
const (
	StatusPlanning  = "PLANNING"   // 版本规划中，可添加任务
	StatusActive    = "ACTIVE"     // 版本开发中，任务执行中
	StatusFrozen    = "FROZEN"     // 版本冻结，不可添加新任务
	StatusReleased  = "RELEASED"   // 版本已发布
	StatusCancelled = "CANCELLED"  // 版本已取消
)

// Conflict status for tasks within a version
const (
	ConflictNone     = "NONE"     // 无冲突
	ConflictWarning  = "WARNING"  // 潜在冲突（同 package/directory）
	ConflictBlocked  = "BLOCKED"  // 确认冲突，等待前序任务完成
	ConflictResolved = "RESOLVED" // 冲突已解决
)

// ProjectVersion represents a version that groups multiple tasks.
type ProjectVersion struct {
	ID          int64      `json:"id" db:"id"`
	TenantID    int64      `json:"tenantId" db:"tenant_id"`
	ProjectID   int64      `json:"projectId" db:"project_id"`
	Version     string     `json:"version" db:"version"`
	Status      string     `json:"status" db:"status"`
	Description string     `json:"description" db:"description"`
	ReleasedAt  *time.Time `json:"releasedAt,omitempty" db:"released_at"`
	CreatedBy   int64      `json:"createdBy" db:"created_by"`
	CreatedAt   time.Time  `json:"createdAt" db:"created_at"`
	UpdatedAt   time.Time  `json:"updatedAt" db:"updated_at"`
}

// VersionTask is a task with version-specific fields.
type VersionTask struct {
	ID             int64    `json:"id"`
	Title          *string  `json:"title,omitempty"`
	Status         string   `json:"status"`
	ConflictStatus string   `json:"conflictStatus"`
	BlockedBy      []int64  `json:"blockedBy"`
	TouchedFiles   []string `json:"touchedFiles"`
}

// ----- Request/Response DTOs -----

type CreateVersionRequest struct {
	Version     string `json:"version" binding:"required,min=1,max=50"`
	Description string `json:"description" binding:"max=2000"`
}

type UpdateVersionRequest struct {
	Description *string `json:"description,omitempty" binding:"omitempty,max=2000"`
	Status      *string `json:"status,omitempty" binding:"omitempty,oneof=PLANNING ACTIVE FROZEN CANCELLED"`
}

type ReleaseVersionRequest struct {
	Tag     string `json:"tag,omitempty"`     // Git tag name, defaults to version string
	Message string `json:"message,omitempty"` // Git tag message
}

type VersionResponse struct {
	Version ProjectVersion `json:"version"`
	Tasks   []VersionTask  `json:"tasks,omitempty"`
	Stats   *VersionStats  `json:"stats,omitempty"`
}

type VersionStats struct {
	TotalTasks     int `json:"totalTasks"`
	CompletedTasks int `json:"completedTasks"`
	FailedTasks    int `json:"failedTasks"`
	ActiveTasks    int `json:"activeTasks"`
	BlockedTasks   int `json:"blockedTasks"`
}

type VersionListResponse struct {
	Versions []VersionResponse `json:"versions"`
	Total    int64             `json:"total"`
}
```

### 1.3 新建 `forge-core/internal/module/version/repository.go`

```go
package version

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

// Create inserts a new project version.
func (r *Repository) Create(ctx context.Context, v *ProjectVersion) error {
	return r.db.QueryRow(ctx,
		`INSERT INTO engine.project_versions (tenant_id, project_id, version, status, description, created_by)
		 VALUES ($1, $2, $3, $4, $5, $6)
		 RETURNING id, created_at, updated_at`,
		v.TenantID, v.ProjectID, v.Version, v.Status, v.Description, v.CreatedBy,
	).Scan(&v.ID, &v.CreatedAt, &v.UpdatedAt)
}

// GetByID returns a version by ID.
func (r *Repository) GetByID(ctx context.Context, id int64) (*ProjectVersion, error) {
	v := &ProjectVersion{}
	err := r.db.QueryRow(ctx,
		`SELECT id, tenant_id, project_id, version, status, description, released_at, created_by, created_at, updated_at
		 FROM engine.project_versions WHERE id = $1`, id,
	).Scan(&v.ID, &v.TenantID, &v.ProjectID, &v.Version, &v.Status, &v.Description,
		&v.ReleasedAt, &v.CreatedBy, &v.CreatedAt, &v.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return v, nil
}

// ListByProject returns versions for a project with pagination.
func (r *Repository) ListByProject(ctx context.Context, projectID int64, page, pageSize int) ([]ProjectVersion, int64, error) {
	var total int64
	err := r.db.QueryRow(ctx,
		`SELECT COUNT(*) FROM engine.project_versions WHERE project_id = $1`, projectID,
	).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("count versions: %w", err)
	}

	offset := (page - 1) * pageSize
	rows, err := r.db.Query(ctx,
		`SELECT id, tenant_id, project_id, version, status, description, released_at, created_by, created_at, updated_at
		 FROM engine.project_versions
		 WHERE project_id = $1
		 ORDER BY created_at DESC
		 LIMIT $2 OFFSET $3`,
		projectID, pageSize, offset,
	)
	if err != nil {
		return nil, 0, fmt.Errorf("list versions: %w", err)
	}
	defer rows.Close()

	var versions []ProjectVersion
	for rows.Next() {
		var v ProjectVersion
		if err := rows.Scan(&v.ID, &v.TenantID, &v.ProjectID, &v.Version, &v.Status,
			&v.Description, &v.ReleasedAt, &v.CreatedBy, &v.CreatedAt, &v.UpdatedAt); err != nil {
			return nil, 0, fmt.Errorf("scan version: %w", err)
		}
		versions = append(versions, v)
	}
	return versions, total, nil
}

// Update updates mutable fields of a version.
func (r *Repository) Update(ctx context.Context, id int64, desc *string, status *string) error {
	if desc != nil {
		_, err := r.db.Exec(ctx,
			`UPDATE engine.project_versions SET description = $2, updated_at = NOW() WHERE id = $1`,
			id, *desc)
		if err != nil {
			return fmt.Errorf("update description: %w", err)
		}
	}
	if status != nil {
		_, err := r.db.Exec(ctx,
			`UPDATE engine.project_versions SET status = $2, updated_at = NOW() WHERE id = $1`,
			id, *status)
		if err != nil {
			return fmt.Errorf("update status: %w", err)
		}
	}
	return nil
}

// Release marks a version as RELEASED with timestamp.
func (r *Repository) Release(ctx context.Context, id int64) error {
	_, err := r.db.Exec(ctx,
		`UPDATE engine.project_versions SET status = 'RELEASED', released_at = NOW(), updated_at = NOW() WHERE id = $1`,
		id)
	return err
}

// GetTasksByVersion returns tasks belonging to a version with conflict info.
func (r *Repository) GetTasksByVersion(ctx context.Context, versionID int64) ([]VersionTask, error) {
	rows, err := r.db.Query(ctx,
		`SELECT id, title, status, conflict_status, blocked_by, touched_files
		 FROM engine.tasks
		 WHERE version_id = $1
		 ORDER BY id ASC`,
		versionID,
	)
	if err != nil {
		return nil, fmt.Errorf("get tasks by version: %w", err)
	}
	defer rows.Close()

	var tasks []VersionTask
	for rows.Next() {
		var t VersionTask
		if err := rows.Scan(&t.ID, &t.Title, &t.Status, &t.ConflictStatus, &t.BlockedBy, &t.TouchedFiles); err != nil {
			return nil, fmt.Errorf("scan version task: %w", err)
		}
		tasks = append(tasks, t)
	}
	return tasks, nil
}

// GetVersionStats computes task statistics for a version.
func (r *Repository) GetVersionStats(ctx context.Context, versionID int64) (*VersionStats, error) {
	stats := &VersionStats{}
	err := r.db.QueryRow(ctx,
		`SELECT
			COUNT(*),
			COUNT(*) FILTER (WHERE status = 'COMPLETED'),
			COUNT(*) FILTER (WHERE status = 'FAILED'),
			COUNT(*) FILTER (WHERE status NOT IN ('COMPLETED', 'FAILED', 'CANCELLED')),
			COUNT(*) FILTER (WHERE conflict_status = 'BLOCKED')
		 FROM engine.tasks WHERE version_id = $1`,
		versionID,
	).Scan(&stats.TotalTasks, &stats.CompletedTasks, &stats.FailedTasks,
		&stats.ActiveTasks, &stats.BlockedTasks)
	if err != nil {
		if err == pgx.ErrNoRows {
			return stats, nil
		}
		return nil, fmt.Errorf("get version stats: %w", err)
	}
	return stats, nil
}

// UpdateTaskConflict updates the conflict status of a task.
func (r *Repository) UpdateTaskConflict(ctx context.Context, taskID int64, status string, blockedBy []int64) error {
	_, err := r.db.Exec(ctx,
		`UPDATE engine.tasks SET conflict_status = $2, blocked_by = $3, updated_at = NOW() WHERE id = $1`,
		taskID, status, blockedBy,
	)
	return err
}

// UpdateTaskTouchedFiles sets the list of files a task has modified.
func (r *Repository) UpdateTaskTouchedFiles(ctx context.Context, taskID int64, files []string) error {
	_, err := r.db.Exec(ctx,
		`UPDATE engine.tasks SET touched_files = $2, updated_at = NOW() WHERE id = $1`,
		taskID, files,
	)
	return err
}
```

---

## Day 2 — Service + Handler + Router + 前端 API 客户端

### 2.1 新建 `forge-core/internal/module/version/service.go`

```go
package version

import (
	"context"
	"fmt"
	"log/slog"
	"regexp"
)

// semverRegex validates version strings like v1.0.0, v1.2.3-rc.1, 1.0.0
var semverRegex = regexp.MustCompile(`^v?\d+\.\d+\.\d+(-[a-zA-Z0-9.]+)?$`)

type Service struct {
	repo *Repository
}

func NewService(repo *Repository) *Service {
	return &Service{repo: repo}
}

// Create validates and creates a new version.
func (s *Service) Create(ctx context.Context, tenantID, projectID, userID int64, req *CreateVersionRequest) (*VersionResponse, error) {
	// Validate version format
	if !semverRegex.MatchString(req.Version) {
		return nil, fmt.Errorf("版本号格式无效，请使用语义化版本号（如 v1.0.0, v1.2.3-rc.1）")
	}

	v := &ProjectVersion{
		TenantID:    tenantID,
		ProjectID:   projectID,
		Version:     req.Version,
		Status:      StatusPlanning,
		Description: req.Description,
		CreatedBy:   userID,
	}

	if err := s.repo.Create(ctx, v); err != nil {
		return nil, fmt.Errorf("创建版本失败: %w", err)
	}

	slog.Info("version created", "id", v.ID, "version", v.Version, "project_id", projectID)
	return &VersionResponse{Version: *v}, nil
}

// GetByID returns a version with tasks and stats.
func (s *Service) GetByID(ctx context.Context, id int64) (*VersionResponse, error) {
	v, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("版本不存在: %w", err)
	}

	tasks, err := s.repo.GetTasksByVersion(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("获取版本任务失败: %w", err)
	}

	stats, err := s.repo.GetVersionStats(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("获取版本统计失败: %w", err)
	}

	return &VersionResponse{
		Version: *v,
		Tasks:   tasks,
		Stats:   stats,
	}, nil
}

// List returns versions for a project.
func (s *Service) List(ctx context.Context, projectID int64, page, pageSize int) (*VersionListResponse, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	versions, total, err := s.repo.ListByProject(ctx, projectID, page, pageSize)
	if err != nil {
		return nil, fmt.Errorf("获取版本列表失败: %w", err)
	}

	// Enrich each version with stats
	result := make([]VersionResponse, 0, len(versions))
	for i := range versions {
		stats, _ := s.repo.GetVersionStats(ctx, versions[i].ID)
		result = append(result, VersionResponse{
			Version: versions[i],
			Stats:   stats,
		})
	}

	return &VersionListResponse{
		Versions: result,
		Total:    total,
	}, nil
}

// Update modifies a version's description or status.
func (s *Service) Update(ctx context.Context, id int64, req *UpdateVersionRequest) error {
	// Validate status transitions
	if req.Status != nil {
		v, err := s.repo.GetByID(ctx, id)
		if err != nil {
			return fmt.Errorf("版本不存在: %w", err)
		}
		if err := validateStatusTransition(v.Status, *req.Status); err != nil {
			return err
		}
	}
	return s.repo.Update(ctx, id, req.Description, req.Status)
}

// Release marks a version as released after validating all tasks are done.
func (s *Service) Release(ctx context.Context, id int64, req *ReleaseVersionRequest) error {
	v, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return fmt.Errorf("版本不存在: %w", err)
	}

	if v.Status == StatusReleased {
		return fmt.Errorf("版本已发布")
	}
	if v.Status == StatusCancelled {
		return fmt.Errorf("版本已取消，无法发布")
	}

	// Check all tasks are completed
	stats, err := s.repo.GetVersionStats(ctx, id)
	if err != nil {
		return fmt.Errorf("获取版本统计失败: %w", err)
	}
	if stats.ActiveTasks > 0 {
		return fmt.Errorf("版本中还有 %d 个进行中的任务，无法发布", stats.ActiveTasks)
	}
	if stats.BlockedTasks > 0 {
		return fmt.Errorf("版本中还有 %d 个被阻塞的任务，无法发布", stats.BlockedTasks)
	}

	// TODO(SH-3b): Trigger git tag creation via Temporal activity
	slog.Info("version released", "id", id, "version", v.Version)

	return s.repo.Release(ctx, id)
}

// validateStatusTransition checks if a status change is allowed.
func validateStatusTransition(current, target string) error {
	allowed := map[string][]string{
		StatusPlanning:  {StatusActive, StatusCancelled},
		StatusActive:    {StatusFrozen, StatusCancelled},
		StatusFrozen:    {StatusActive, StatusCancelled},  // Can unfreeze
		StatusReleased:  {},                                // Terminal state
		StatusCancelled: {},                                // Terminal state
	}
	targets, ok := allowed[current]
	if !ok {
		return fmt.Errorf("未知的当前状态: %s", current)
	}
	for _, t := range targets {
		if t == target {
			return nil
		}
	}
	return fmt.Errorf("不允许从 %s 变更为 %s", current, target)
}
```

### 2.2 新建 `forge-core/internal/module/version/handler.go`

```go
package version

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/shulex/forge/forge-core/internal/pkg/response"
)

type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

// POST /api/projects/:id/versions
func (h *Handler) Create(c *gin.Context) {
	projectID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.Fail(c, http.StatusBadRequest, "无效的项目ID")
		return
	}

	var req CreateVersionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, "请输入版本号")
		return
	}

	tenantID, _ := c.Get("tenant_id")
	userID, _ := c.Get("user_id")

	result, err := h.service.Create(c.Request.Context(),
		tenantID.(int64), projectID, userID.(int64), &req)
	if err != nil {
		response.Fail(c, http.StatusBadRequest, err.Error())
		return
	}

	response.OK(c, result)
}

// GET /api/projects/:id/versions
func (h *Handler) List(c *gin.Context) {
	projectID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.Fail(c, http.StatusBadRequest, "无效的项目ID")
		return
	}

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))

	result, err := h.service.List(c.Request.Context(), projectID, page, pageSize)
	if err != nil {
		response.Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	response.OK(c, result)
}

// GET /api/projects/:id/versions/:versionId
func (h *Handler) GetByID(c *gin.Context) {
	versionID, err := strconv.ParseInt(c.Param("versionId"), 10, 64)
	if err != nil {
		response.Fail(c, http.StatusBadRequest, "无效的版本ID")
		return
	}

	result, err := h.service.GetByID(c.Request.Context(), versionID)
	if err != nil {
		response.Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	response.OK(c, result)
}

// PUT /api/projects/:id/versions/:versionId
func (h *Handler) Update(c *gin.Context) {
	versionID, err := strconv.ParseInt(c.Param("versionId"), 10, 64)
	if err != nil {
		response.Fail(c, http.StatusBadRequest, "无效的版本ID")
		return
	}

	var req UpdateVersionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, "请求参数无效")
		return
	}

	if err := h.service.Update(c.Request.Context(), versionID, &req); err != nil {
		response.Fail(c, http.StatusBadRequest, err.Error())
		return
	}
	response.OK(c, gin.H{"message": "版本已更新"})
}

// POST /api/projects/:id/versions/:versionId/release
func (h *Handler) Release(c *gin.Context) {
	versionID, err := strconv.ParseInt(c.Param("versionId"), 10, 64)
	if err != nil {
		response.Fail(c, http.StatusBadRequest, "无效的版本ID")
		return
	}

	var req ReleaseVersionRequest
	_ = c.ShouldBindJSON(&req) // Optional body

	if err := h.service.Release(c.Request.Context(), versionID, &req); err != nil {
		response.Fail(c, http.StatusBadRequest, err.Error())
		return
	}
	response.OK(c, gin.H{"message": "版本已发布"})
}
```

### 2.3 修改 `forge-core/internal/router/router.go` — 注册版本路由

在 Deps 结构体添加字段：

```go
type Deps struct {
	// ... existing fields ...
	VersionHandler *version.Handler  // NEW
}
```

在 `Setup()` 函数的 protected 路由组内添加：

```go
// Versions
if deps.VersionHandler != nil {
    protected.POST("/projects/:id/versions", deps.VersionHandler.Create)
    protected.GET("/projects/:id/versions", deps.VersionHandler.List)
    protected.GET("/projects/:id/versions/:versionId", deps.VersionHandler.GetByID)
    protected.PUT("/projects/:id/versions/:versionId", deps.VersionHandler.Update)
    protected.POST("/projects/:id/versions/:versionId/release", deps.VersionHandler.Release)
}
```

### 2.4 修改 `forge-core/internal/module/task/model.go` — 扩展 Task 结构体

添加版本协调相关字段：

```go
type Task struct {
	ID             int64      `json:"id"`
	TenantID       int64      `json:"tenant_id"`
	ProjectID      int64      `json:"project_id"`
	Title          *string    `json:"title,omitempty"`
	Requirement    string     `json:"requirement"`
	Source         string     `json:"source"`
	Status         string     `json:"status"`
	WorkflowID     *string    `json:"workflow_id,omitempty"`
	WorkflowRunID  *string    `json:"workflow_run_id,omitempty"`
	RiskLevel      *string    `json:"risk_level,omitempty"`
	RiskScore      *int       `json:"risk_score,omitempty"`
	BranchName     *string    `json:"branch_name,omitempty"`
	FilesChanged   *int       `json:"files_changed,omitempty"`
	LinesAdded     *int       `json:"lines_added,omitempty"`
	LinesDeleted   *int       `json:"lines_deleted,omitempty"`
	PrNumber       *int       `json:"pr_number,omitempty" db:"pr_number"`
	MrUrl          *string    `json:"mr_url,omitempty" db:"mr_url"`
	ReviewScore    *int       `json:"review_score,omitempty" db:"review_score"`
	VersionID      *int64     `json:"version_id,omitempty" db:"version_id"`           // NEW
	ConflictStatus string     `json:"conflict_status" db:"conflict_status"`            // NEW
	BlockedBy      []int64    `json:"blocked_by,omitempty" db:"blocked_by"`            // NEW
	TouchedFiles   []string   `json:"touched_files,omitempty" db:"touched_files"`      // NEW
	CreatedBy      int64      `json:"created_by"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
	CompletedAt    *time.Time `json:"completed_at,omitempty"`
}
```

### 2.5 新建 `forge-portal/lib/versions.ts` — 前端 API 客户端

```typescript
import { apiClient } from "./api-client";

// Types
export interface ProjectVersion {
  id: number;
  tenantId: number;
  projectId: number;
  version: string;
  status: "PLANNING" | "ACTIVE" | "FROZEN" | "RELEASED" | "CANCELLED";
  description: string;
  releasedAt: string | null;
  createdBy: number;
  createdAt: string;
  updatedAt: string;
}

export interface VersionTask {
  id: number;
  title: string | null;
  status: string;
  conflictStatus: "NONE" | "WARNING" | "BLOCKED" | "RESOLVED";
  blockedBy: number[];
  touchedFiles: string[];
}

export interface VersionStats {
  totalTasks: number;
  completedTasks: number;
  failedTasks: number;
  activeTasks: number;
  blockedTasks: number;
}

export interface VersionResponse {
  version: ProjectVersion;
  tasks?: VersionTask[];
  stats?: VersionStats;
}

export interface VersionListResponse {
  versions: VersionResponse[];
  total: number;
}

// API functions

export async function createVersion(
  projectId: number,
  data: { version: string; description?: string }
): Promise<VersionResponse> {
  const res = await apiClient.post(`/projects/${projectId}/versions`, data);
  return res.data.data;
}

export async function listVersions(
  projectId: number,
  page = 1,
  pageSize = 20
): Promise<VersionListResponse> {
  const res = await apiClient.get(`/projects/${projectId}/versions`, {
    params: { page, page_size: pageSize },
  });
  return res.data.data;
}

export async function getVersion(
  projectId: number,
  versionId: number
): Promise<VersionResponse> {
  const res = await apiClient.get(
    `/projects/${projectId}/versions/${versionId}`
  );
  return res.data.data;
}

export async function updateVersion(
  projectId: number,
  versionId: number,
  data: { description?: string; status?: string }
): Promise<void> {
  await apiClient.put(`/projects/${projectId}/versions/${versionId}`, data);
}

export async function releaseVersion(
  projectId: number,
  versionId: number,
  data?: { tag?: string; message?: string }
): Promise<void> {
  await apiClient.post(
    `/projects/${projectId}/versions/${versionId}/release`,
    data || {}
  );
}
```

---

## 数据结构

### project_versions 表

| 字段 | 类型 | 约束 | 说明 |
|------|------|------|------|
| id | BIGSERIAL | PK | |
| tenant_id | BIGINT | NOT NULL | 多租户隔离 |
| project_id | BIGINT | FK → engine.projects | |
| version | VARCHAR(50) | NOT NULL, UNIQUE(project_id, version) | 语义化版本号 |
| status | VARCHAR(20) | NOT NULL, DEFAULT 'PLANNING' | 版本状态 |
| description | TEXT | DEFAULT '' | 版本描述 |
| released_at | TIMESTAMPTZ | NULLABLE | 发布时间 |
| created_by | BIGINT | FK → auth.users | |
| created_at | TIMESTAMPTZ | NOT NULL, DEFAULT NOW() | |
| updated_at | TIMESTAMPTZ | NOT NULL, DEFAULT NOW() | |

### tasks 表新增字段

| 字段 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| version_id | BIGINT | NULL | FK → project_versions，NULL = 独立任务 |
| conflict_status | VARCHAR(20) | 'NONE' | NONE/WARNING/BLOCKED/RESOLVED |
| blocked_by | BIGINT[] | '{}' | 阻塞此任务的任务 ID 列表 |
| touched_files | TEXT[] | '{}' | 此任务修改的文件路径列表 |

### 版本状态流转

```
PLANNING ──→ ACTIVE ──→ FROZEN ──→ ACTIVE (unfreeze)
    │            │          │
    │            │          └──→ CANCELLED
    │            └──→ CANCELLED
    └──→ CANCELLED

ACTIVE ──→ RELEASED (terminal, all tasks completed)
```

---

## API 设计

| 方法 | 路径 | 说明 | 请求体 | 响应 |
|------|------|------|--------|------|
| POST | `/api/projects/:id/versions` | 创建版本 | `{version, description?}` | `VersionResponse` |
| GET | `/api/projects/:id/versions` | 版本列表 | query: `page, page_size` | `VersionListResponse` |
| GET | `/api/projects/:id/versions/:versionId` | 版本详情（含任务列表 + 统计） | - | `VersionResponse` |
| PUT | `/api/projects/:id/versions/:versionId` | 更新版本 | `{description?, status?}` | `{message}` |
| POST | `/api/projects/:id/versions/:versionId/release` | 发布版本 | `{tag?, message?}` | `{message}` |

---

## 验收标准

1. **数据库迁移**
   - `016_project_versions.sql` 可成功执行（幂等，使用 IF NOT EXISTS）
   - project_versions 表创建正确，约束生效（version + project_id 唯一）
   - tasks 表新字段 version_id / conflict_status / blocked_by / touched_files 添加成功
   - 现有 tasks 数据不受影响（新字段有默认值）

2. **CRUD API**
   - POST 创建版本：版本号格式校验（拒绝 "abc"，接受 "v1.0.0"）
   - GET 列表：分页正确，含统计信息
   - GET 详情：返回版本信息 + 关联任务列表 + 统计
   - PUT 更新：状态流转校验（PLANNING→ACTIVE 允许，RELEASED→ACTIVE 拒绝）
   - POST release：检查所有任务已完成，否则返回明确错误

3. **前端客户端**
   - TypeScript 类型定义完整
   - 5 个 API 函数可正常调用

---

## 质量验证

### Go 单元测试

```go
// internal/module/version/service_test.go

func TestValidateVersionFormat(t *testing.T) {
	tests := []struct {
		version string
		valid   bool
	}{
		{"v1.0.0", true},
		{"1.0.0", true},
		{"v1.2.3-rc.1", true},
		{"v1.2.3-beta", true},
		{"abc", false},
		{"v1", false},
		{"v1.0", false},
		{"", false},
	}
	for _, tt := range tests {
		result := semverRegex.MatchString(tt.version)
		if result != tt.valid {
			t.Errorf("version=%q expected valid=%v got %v", tt.version, tt.valid, result)
		}
	}
}

func TestValidateStatusTransition(t *testing.T) {
	tests := []struct {
		from, to string
		wantErr  bool
	}{
		{"PLANNING", "ACTIVE", false},
		{"PLANNING", "CANCELLED", false},
		{"ACTIVE", "FROZEN", false},
		{"ACTIVE", "CANCELLED", false},
		{"FROZEN", "ACTIVE", false},   // unfreeze
		{"RELEASED", "ACTIVE", true},  // terminal
		{"CANCELLED", "ACTIVE", true}, // terminal
		{"PLANNING", "RELEASED", true}, // must go through ACTIVE
	}
	for _, tt := range tests {
		err := validateStatusTransition(tt.from, tt.to)
		if (err != nil) != tt.wantErr {
			t.Errorf("from=%s to=%s wantErr=%v gotErr=%v", tt.from, tt.to, tt.wantErr, err)
		}
	}
}
```

### HTTP API 手工测试

```bash
# 创建版本
curl -X POST http://localhost:8080/api/projects/1/versions \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"version": "v1.0.0", "description": "第一个版本"}'

# 列表
curl http://localhost:8080/api/projects/1/versions \
  -H "Authorization: Bearer $TOKEN"

# 详情
curl http://localhost:8080/api/projects/1/versions/1 \
  -H "Authorization: Bearer $TOKEN"

# 更新状态
curl -X PUT http://localhost:8080/api/projects/1/versions/1 \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"status": "ACTIVE"}'

# 发布（失败：有活跃任务）
curl -X POST http://localhost:8080/api/projects/1/versions/1/release \
  -H "Authorization: Bearer $TOKEN"
# 预期: 400 "版本中还有 N 个进行中的任务，无法发布"
```
