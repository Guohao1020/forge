# S2 — 项目管理 CRUD + 项目详情布局

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 在 S1 登录闭环基础上，交付完整的项目 CRUD（创建/查看/编辑/删除/收藏），包括项目大厅卡片列表、项目详情布局和设置页面。

**Architecture:** forge-core 新增 project module（model/repository/service/handler），遵循 auth module 的 Constructor Injection 模式。前端在 (dashboard) route group 下新增创建对话框、项目卡片网格、项目详情 Layout 和设置页。engine schema 存储项目和收藏数据。

**Tech Stack:** Go 1.22 + Gin + pgxpool, Next.js 15 (App Router) + TypeScript + Tailwind CSS 4 + shadcn/ui + Lucide Icons

---

## 前置说明

### 依赖 S1 交付物

本切片假设以下 S1 产物已就位：
- Docker Compose (PostgreSQL + Redis) 正常运行
- forge-core Go API Server 可编译启动（auth module 工作正常）
- forge-portal Next.js 前端可启动（login page + empty project hall + dashboard layout + sidebar + topbar）
- `forge-core/internal/pkg/response/response.go` — Result[T] 统一响应
- `forge-core/internal/pkg/database/postgres.go` — PostgreSQL 连接池
- `forge-core/internal/pkg/database/migrate.go` — 迁移执行器
- `forge-core/internal/middleware/auth.go` — JWT 鉴权中间件
- `forge-core/internal/router/router.go` — 路由注册（Deps 依赖注入模式）
- `forge-portal/lib/api.ts` — fetch wrapper（自动带 token、401 跳转）
- `forge-portal/lib/auth.tsx` — AuthProvider + useAuth hook
- `forge-portal/components/sidebar.tsx` — 侧边栏
- `forge-portal/components/topbar.tsx` — 顶栏

### 本切片交付后你可以做什么

1. 在项目大厅看到项目卡片列表（含搜索、收藏筛选）
2. 点击"创建新项目"弹出对话框，填写名称/描述/默认分支后创建项目
3. 收藏/取消收藏项目，收藏的项目排在前面
4. 点击项目卡片进入项目详情页，看到项目级侧边栏（概览/任务/变更/测试/部署/设置）
5. 在设置页编辑项目名称/描述/默认分支
6. 在设置页危险区删除项目（软删除）

---

## 文件结构

### forge-core 新增/修改

```
forge-core/
├── migrations/
│   └── 002_init_engine.sql                    # engine schema DDL (projects + project_stars)
├── internal/
│   ├── module/
│   │   └── project/
│   │       ├── model.go                       # 数据模型 + DTO
│   │       ├── repository.go                  # 数据库操作
│   │       ├── service.go                     # 业务逻辑
│   │       └── handler.go                     # HTTP handler
│   └── router/
│       └── router.go                          # 修改：注册 project 路由
├── cmd/
│   └── forge-core/
│       └── main.go                            # 修改：组装 project 模块
```

### forge-portal 新增/修改

```
forge-portal/
├── app/
│   └── (dashboard)/
│       └── projects/
│           ├── page.tsx                       # 修改：从空状态 → 真实项目列表
│           └── [id]/
│               ├── layout.tsx                 # 项目详情 Layout（项目级侧边栏）
│               ├── page.tsx                   # 概览页
│               ├── tasks/page.tsx             # 任务占位页
│               ├── changes/page.tsx           # 变更占位页
│               ├── tests/page.tsx             # 测试占位页
│               ├── deploy/page.tsx            # 部署占位页
│               └── settings/page.tsx          # 设置页
├── components/
│   ├── project-card.tsx                       # 项目卡片组件
│   ├── create-project-dialog.tsx              # 创建项目对话框
│   └── project-sidebar.tsx                    # 项目级侧边栏
├── lib/
│   └── api.ts                                 # 不变（S1 已有）
```

---

## Task 1: Engine Schema 数据库迁移

**Files:**
- Create: `forge-core/migrations/002_init_engine.sql`

- [ ] **Step 1: 创建 engine schema 迁移脚本**

`forge-core/migrations/002_init_engine.sql`：

```sql
-- Projects
CREATE TABLE IF NOT EXISTS engine.projects (
    id              BIGSERIAL PRIMARY KEY,
    tenant_id       BIGINT NOT NULL REFERENCES auth.tenants(id),
    name            VARCHAR(200) NOT NULL,
    description     TEXT,
    status          VARCHAR(20) NOT NULL DEFAULT 'ACTIVE',
    code_platform   VARCHAR(50),
    code_repo_url   TEXT,
    code_credential VARCHAR(100),
    profile         JSONB DEFAULT '{}',
    profile_version INT DEFAULT 0,
    profile_updated TIMESTAMPTZ,
    default_branch  VARCHAR(100) DEFAULT 'main',
    ai_model        VARCHAR(50),
    risk_threshold  INT DEFAULT 90,
    auto_merge      BOOLEAN DEFAULT TRUE,
    created_by      BIGINT REFERENCES auth.users(id),
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(tenant_id, name)
);

-- Project stars (user favorites)
CREATE TABLE IF NOT EXISTS engine.project_stars (
    id              BIGSERIAL PRIMARY KEY,
    user_id         BIGINT NOT NULL REFERENCES auth.users(id),
    project_id      BIGINT NOT NULL REFERENCES engine.projects(id),
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(user_id, project_id)
);

-- Indexes for common queries
CREATE INDEX IF NOT EXISTS idx_projects_tenant_id ON engine.projects(tenant_id);
CREATE INDEX IF NOT EXISTS idx_projects_tenant_status ON engine.projects(tenant_id, status);
CREATE INDEX IF NOT EXISTS idx_project_stars_user_id ON engine.project_stars(user_id);
CREATE INDEX IF NOT EXISTS idx_project_stars_project_id ON engine.project_stars(project_id);
```

- [ ] **Step 2: 验证迁移执行**

```bash
# 确保 Docker 基础设施运行中
docker compose -f docker-compose.dev.yml up -d

# 启动后端（会自动执行迁移）
cd forge-core && go run ./cmd/forge-core

# 验证表已创建
docker exec forge-postgres psql -U forge -d forge_main -c "\dt engine.*"
# 预期: engine.projects 和 engine.project_stars 两张表
```

- [ ] **Step 3: Commit**

```bash
git add forge-core/migrations/002_init_engine.sql
git commit -m "feat: add engine schema migration with projects and project_stars tables"
```

---

## Task 2: Project Module — 后端 Model + Repository

**Files:**
- Create: `forge-core/internal/module/project/model.go`
- Create: `forge-core/internal/module/project/repository.go`

- [ ] **Step 1: 创建 model.go — 数据模型 + DTO**

`forge-core/internal/module/project/model.go`：

```go
package project

import "time"

// DB model
type Project struct {
	ID             int64      `json:"id"`
	TenantID       int64      `json:"tenant_id"`
	Name           string     `json:"name"`
	Description    *string    `json:"description,omitempty"`
	Status         string     `json:"status"`
	CodePlatform   *string    `json:"code_platform,omitempty"`
	CodeRepoURL    *string    `json:"code_repo_url,omitempty"`
	DefaultBranch  string     `json:"default_branch"`
	AIModel        *string    `json:"ai_model,omitempty"`
	RiskThreshold  int        `json:"risk_threshold"`
	AutoMerge      bool       `json:"auto_merge"`
	CreatedBy      *int64     `json:"created_by,omitempty"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
}

// ProjectWithStar is a project with star status for the current user
type ProjectWithStar struct {
	Project
	Starred bool `json:"starred"`
}

// Request DTOs
type CreateProjectRequest struct {
	Name          string  `json:"name" binding:"required,max=200"`
	Description   *string `json:"description"`
	DefaultBranch string  `json:"default_branch"`
	CodePlatform  *string `json:"code_platform"`
}

type UpdateProjectRequest struct {
	Name          *string `json:"name" binding:"omitempty,max=200"`
	Description   *string `json:"description"`
	DefaultBranch *string `json:"default_branch"`
}

type ListProjectsQuery struct {
	Search    string `form:"search"`
	Starred   bool   `form:"starred"`
	Page      int    `form:"page"`
	PageSize  int    `form:"page_size"`
}

// Response DTOs
type ProjectListResponse struct {
	Items      []ProjectWithStar `json:"items"`
	Total      int64             `json:"total"`
	Page       int               `json:"page"`
	PageSize   int               `json:"page_size"`
}
```

- [ ] **Step 2: 创建 repository.go — 数据库操作**

`forge-core/internal/module/project/repository.go`：

```go
package project

import (
	"context"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

func (r *Repository) Create(ctx context.Context, tenantID int64, userID int64, req *CreateProjectRequest) (*Project, error) {
	defaultBranch := req.DefaultBranch
	if defaultBranch == "" {
		defaultBranch = "main"
	}

	p := &Project{}
	err := r.db.QueryRow(ctx,
		`INSERT INTO engine.projects (tenant_id, name, description, default_branch, code_platform, created_by)
		 VALUES ($1, $2, $3, $4, $5, $6)
		 RETURNING id, tenant_id, name, description, status, code_platform, code_repo_url,
		           default_branch, ai_model, risk_threshold, auto_merge, created_by, created_at, updated_at`,
		tenantID, req.Name, req.Description, defaultBranch, req.CodePlatform, userID,
	).Scan(&p.ID, &p.TenantID, &p.Name, &p.Description, &p.Status, &p.CodePlatform, &p.CodeRepoURL,
		&p.DefaultBranch, &p.AIModel, &p.RiskThreshold, &p.AutoMerge, &p.CreatedBy, &p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("create project: %w", err)
	}
	return p, nil
}

func (r *Repository) FindByID(ctx context.Context, tenantID int64, projectID int64) (*Project, error) {
	p := &Project{}
	err := r.db.QueryRow(ctx,
		`SELECT id, tenant_id, name, description, status, code_platform, code_repo_url,
		        default_branch, ai_model, risk_threshold, auto_merge, created_by, created_at, updated_at
		 FROM engine.projects
		 WHERE id = $1 AND tenant_id = $2 AND status != 'ARCHIVED'`,
		projectID, tenantID,
	).Scan(&p.ID, &p.TenantID, &p.Name, &p.Description, &p.Status, &p.CodePlatform, &p.CodeRepoURL,
		&p.DefaultBranch, &p.AIModel, &p.RiskThreshold, &p.AutoMerge, &p.CreatedBy, &p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("find project: %w", err)
	}
	return p, nil
}

func (r *Repository) List(ctx context.Context, tenantID int64, userID int64, q *ListProjectsQuery) ([]ProjectWithStar, int64, error) {
	// Build WHERE clause
	conditions := []string{"p.tenant_id = $1", "p.status != 'ARCHIVED'"}
	args := []interface{}{tenantID}
	argIdx := 2

	if q.Search != "" {
		conditions = append(conditions, fmt.Sprintf("(p.name ILIKE $%d OR p.description ILIKE $%d)", argIdx, argIdx))
		args = append(args, "%"+q.Search+"%")
		argIdx++
	}

	if q.Starred {
		conditions = append(conditions, fmt.Sprintf("ps.user_id = $%d", argIdx))
		args = append(args, userID)
		argIdx++
	}

	where := strings.Join(conditions, " AND ")

	// Join for star info
	joinClause := fmt.Sprintf("LEFT JOIN engine.project_stars ps ON p.id = ps.project_id AND ps.user_id = $%d", argIdx)
	args = append(args, userID)
	argIdx++

	// Count total
	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM engine.projects p %s WHERE %s", joinClause, where)
	var total int64
	err := r.db.QueryRow(ctx, countQuery, args...).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("count projects: %w", err)
	}

	// Fetch page — starred projects first, then by updated_at desc
	offset := (q.Page - 1) * q.PageSize
	listQuery := fmt.Sprintf(
		`SELECT p.id, p.tenant_id, p.name, p.description, p.status, p.code_platform, p.code_repo_url,
		        p.default_branch, p.ai_model, p.risk_threshold, p.auto_merge, p.created_by, p.created_at, p.updated_at,
		        CASE WHEN ps.user_id IS NOT NULL THEN TRUE ELSE FALSE END AS starred
		 FROM engine.projects p
		 %s
		 WHERE %s
		 ORDER BY starred DESC, p.updated_at DESC
		 LIMIT $%d OFFSET $%d`,
		joinClause, where, argIdx, argIdx+1,
	)
	args = append(args, q.PageSize, offset)

	rows, err := r.db.Query(ctx, listQuery, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("list projects: %w", err)
	}
	defer rows.Close()

	var items []ProjectWithStar
	for rows.Next() {
		var pw ProjectWithStar
		err := rows.Scan(&pw.ID, &pw.TenantID, &pw.Name, &pw.Description, &pw.Status,
			&pw.CodePlatform, &pw.CodeRepoURL, &pw.DefaultBranch, &pw.AIModel,
			&pw.RiskThreshold, &pw.AutoMerge, &pw.CreatedBy, &pw.CreatedAt, &pw.UpdatedAt,
			&pw.Starred)
		if err != nil {
			return nil, 0, fmt.Errorf("scan project: %w", err)
		}
		items = append(items, pw)
	}

	return items, total, nil
}

func (r *Repository) Update(ctx context.Context, tenantID int64, projectID int64, req *UpdateProjectRequest) (*Project, error) {
	// Build SET clause dynamically
	sets := []string{"updated_at = NOW()"}
	args := []interface{}{}
	argIdx := 1

	if req.Name != nil {
		sets = append(sets, fmt.Sprintf("name = $%d", argIdx))
		args = append(args, *req.Name)
		argIdx++
	}
	if req.Description != nil {
		sets = append(sets, fmt.Sprintf("description = $%d", argIdx))
		args = append(args, *req.Description)
		argIdx++
	}
	if req.DefaultBranch != nil {
		sets = append(sets, fmt.Sprintf("default_branch = $%d", argIdx))
		args = append(args, *req.DefaultBranch)
		argIdx++
	}

	setClause := strings.Join(sets, ", ")

	args = append(args, projectID, tenantID)
	query := fmt.Sprintf(
		`UPDATE engine.projects SET %s WHERE id = $%d AND tenant_id = $%d AND status != 'ARCHIVED'
		 RETURNING id, tenant_id, name, description, status, code_platform, code_repo_url,
		           default_branch, ai_model, risk_threshold, auto_merge, created_by, created_at, updated_at`,
		setClause, argIdx, argIdx+1,
	)

	p := &Project{}
	err := r.db.QueryRow(ctx, query, args...).Scan(
		&p.ID, &p.TenantID, &p.Name, &p.Description, &p.Status, &p.CodePlatform, &p.CodeRepoURL,
		&p.DefaultBranch, &p.AIModel, &p.RiskThreshold, &p.AutoMerge, &p.CreatedBy, &p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("update project: %w", err)
	}
	return p, nil
}

func (r *Repository) Delete(ctx context.Context, tenantID int64, projectID int64) error {
	result, err := r.db.Exec(ctx,
		`UPDATE engine.projects SET status = 'ARCHIVED', updated_at = NOW()
		 WHERE id = $1 AND tenant_id = $2 AND status != 'ARCHIVED'`,
		projectID, tenantID,
	)
	if err != nil {
		return fmt.Errorf("delete project: %w", err)
	}
	if result.RowsAffected() == 0 {
		return fmt.Errorf("project not found")
	}
	return nil
}

func (r *Repository) Star(ctx context.Context, userID int64, projectID int64) error {
	_, err := r.db.Exec(ctx,
		`INSERT INTO engine.project_stars (user_id, project_id)
		 VALUES ($1, $2)
		 ON CONFLICT (user_id, project_id) DO NOTHING`,
		userID, projectID,
	)
	if err != nil {
		return fmt.Errorf("star project: %w", err)
	}
	return nil
}

func (r *Repository) Unstar(ctx context.Context, userID int64, projectID int64) error {
	_, err := r.db.Exec(ctx,
		`DELETE FROM engine.project_stars WHERE user_id = $1 AND project_id = $2`,
		userID, projectID,
	)
	if err != nil {
		return fmt.Errorf("unstar project: %w", err)
	}
	return nil
}

func (r *Repository) IsStarred(ctx context.Context, userID int64, projectID int64) (bool, error) {
	var exists bool
	err := r.db.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM engine.project_stars WHERE user_id = $1 AND project_id = $2)`,
		userID, projectID,
	).Scan(&exists)
	return exists, err
}
```

- [ ] **Step 3: 验证编译**

```bash
cd forge-core && go build ./cmd/forge-core
# 预期: 编译成功无报错
```

- [ ] **Step 4: Commit**

```bash
git add forge-core/internal/module/project/model.go forge-core/internal/module/project/repository.go
git commit -m "feat: add project module model and repository layer"
```

---

## Task 3: Project Module — 后端 Service + Handler + 路由注册

**Files:**
- Create: `forge-core/internal/module/project/service.go`
- Create: `forge-core/internal/module/project/handler.go`
- Modify: `forge-core/internal/router/router.go`
- Modify: `forge-core/cmd/forge-core/main.go`

- [ ] **Step 1: 创建 service.go — 业务逻辑**

`forge-core/internal/module/project/service.go`：

```go
package project

import (
	"context"
	"errors"
)

type Service struct {
	repo *Repository
}

func NewService(repo *Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) Create(ctx context.Context, tenantID int64, userID int64, req *CreateProjectRequest) (*Project, error) {
	if req.Name == "" {
		return nil, errors.New("项目名称不能为空")
	}
	return s.repo.Create(ctx, tenantID, userID, req)
}

func (s *Service) Get(ctx context.Context, tenantID int64, projectID int64) (*Project, error) {
	p, err := s.repo.FindByID(ctx, tenantID, projectID)
	if err != nil {
		return nil, err
	}
	if p == nil {
		return nil, errors.New("项目不存在")
	}
	return p, nil
}

func (s *Service) List(ctx context.Context, tenantID int64, userID int64, q *ListProjectsQuery) (*ProjectListResponse, error) {
	// Normalize pagination
	if q.Page < 1 {
		q.Page = 1
	}
	if q.PageSize < 1 || q.PageSize > 100 {
		q.PageSize = 20
	}

	items, total, err := s.repo.List(ctx, tenantID, userID, q)
	if err != nil {
		return nil, err
	}

	if items == nil {
		items = []ProjectWithStar{}
	}

	return &ProjectListResponse{
		Items:    items,
		Total:    total,
		Page:     q.Page,
		PageSize: q.PageSize,
	}, nil
}

func (s *Service) Update(ctx context.Context, tenantID int64, projectID int64, req *UpdateProjectRequest) (*Project, error) {
	p, err := s.repo.Update(ctx, tenantID, projectID, req)
	if err != nil {
		return nil, err
	}
	if p == nil {
		return nil, errors.New("项目不存在")
	}
	return p, nil
}

func (s *Service) Delete(ctx context.Context, tenantID int64, projectID int64) error {
	return s.repo.Delete(ctx, tenantID, projectID)
}

func (s *Service) Star(ctx context.Context, userID int64, projectID int64) error {
	return s.repo.Star(ctx, userID, projectID)
}

func (s *Service) Unstar(ctx context.Context, userID int64, projectID int64) error {
	return s.repo.Unstar(ctx, userID, projectID)
}
```

- [ ] **Step 2: 创建 handler.go — HTTP handlers**

`forge-core/internal/module/project/handler.go`：

```go
package project

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

// getTenantID extracts tenant_id from JWT context (set by auth middleware)
func getTenantID(c *gin.Context) int64 {
	v, _ := c.Get("tenant_id")
	return v.(int64)
}

// getUserID extracts user_id from JWT context (set by auth middleware)
func getUserID(c *gin.Context) int64 {
	v, _ := c.Get("user_id")
	return v.(int64)
}

// getProjectID parses :id param from URL
func getProjectID(c *gin.Context) (int64, bool) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.Fail(c, http.StatusBadRequest, "无效的项目 ID")
		return 0, false
	}
	return id, true
}

// POST /api/projects
func (h *Handler) Create(c *gin.Context) {
	var req CreateProjectRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, "请填写项目名称")
		return
	}

	p, err := h.service.Create(c.Request.Context(), getTenantID(c), getUserID(c), &req)
	if err != nil {
		// Check for unique constraint violation
		if isUniqueViolation(err) {
			response.Fail(c, http.StatusConflict, "项目名称已存在")
			return
		}
		response.Fail(c, http.StatusInternalServerError, "创建项目失败: "+err.Error())
		return
	}

	response.OK(c, p)
}

// GET /api/projects
func (h *Handler) List(c *gin.Context) {
	var q ListProjectsQuery
	if err := c.ShouldBindQuery(&q); err != nil {
		response.Fail(c, http.StatusBadRequest, "参数错误")
		return
	}

	result, err := h.service.List(c.Request.Context(), getTenantID(c), getUserID(c), &q)
	if err != nil {
		response.Fail(c, http.StatusInternalServerError, "获取项目列表失败")
		return
	}

	response.OK(c, result)
}

// GET /api/projects/:id
func (h *Handler) Get(c *gin.Context) {
	projectID, ok := getProjectID(c)
	if !ok {
		return
	}

	p, err := h.service.Get(c.Request.Context(), getTenantID(c), projectID)
	if err != nil {
		response.Fail(c, http.StatusNotFound, err.Error())
		return
	}

	response.OK(c, p)
}

// PUT /api/projects/:id
func (h *Handler) Update(c *gin.Context) {
	projectID, ok := getProjectID(c)
	if !ok {
		return
	}

	var req UpdateProjectRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, "参数错误")
		return
	}

	p, err := h.service.Update(c.Request.Context(), getTenantID(c), projectID, &req)
	if err != nil {
		if isUniqueViolation(err) {
			response.Fail(c, http.StatusConflict, "项目名称已存在")
			return
		}
		response.Fail(c, http.StatusInternalServerError, "更新项目失败: "+err.Error())
		return
	}

	response.OK(c, p)
}

// DELETE /api/projects/:id
func (h *Handler) Delete(c *gin.Context) {
	projectID, ok := getProjectID(c)
	if !ok {
		return
	}

	if err := h.service.Delete(c.Request.Context(), getTenantID(c), projectID); err != nil {
		response.Fail(c, http.StatusInternalServerError, "删除项目失败: "+err.Error())
		return
	}

	response.OK(c, nil)
}

// POST /api/projects/:id/star
func (h *Handler) Star(c *gin.Context) {
	projectID, ok := getProjectID(c)
	if !ok {
		return
	}

	if err := h.service.Star(c.Request.Context(), getUserID(c), projectID); err != nil {
		response.Fail(c, http.StatusInternalServerError, "收藏失败")
		return
	}

	response.OK(c, nil)
}

// DELETE /api/projects/:id/star
func (h *Handler) Unstar(c *gin.Context) {
	projectID, ok := getProjectID(c)
	if !ok {
		return
	}

	if err := h.service.Unstar(c.Request.Context(), getUserID(c), projectID); err != nil {
		response.Fail(c, http.StatusInternalServerError, "取消收藏失败")
		return
	}

	response.OK(c, nil)
}

// isUniqueViolation checks if the error is a PostgreSQL unique violation (23505)
func isUniqueViolation(err error) bool {
	if err == nil {
		return false
	}
	return contains(err.Error(), "23505") || contains(err.Error(), "unique constraint")
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchString(s, substr)
}

func searchString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
```

- [ ] **Step 3: 更新 router.go — 注册 project 路由**

修改 `forge-core/internal/router/router.go`，在 `Deps` struct 中添加 `ProjectHandler`，在 protected group 中注册 project 路由：

```go
package router

import (
	"github.com/gin-gonic/gin"
	"github.com/shulex/forge/forge-core/internal/middleware"
	"github.com/shulex/forge/forge-core/internal/module/auth"
	"github.com/shulex/forge/forge-core/internal/module/project"
)

type Deps struct {
	AuthHandler    *auth.Handler
	AuthService    *auth.Service
	ProjectHandler *project.Handler
}

func Setup(deps *Deps) *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(middleware.CORS())

	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})

	api := r.Group("/api")
	{
		// Public routes
		api.POST("/auth/login", deps.AuthHandler.Login)

		// Protected routes
		protected := api.Group("")
		protected.Use(middleware.JWTAuth(deps.AuthService))
		{
			protected.POST("/auth/logout", deps.AuthHandler.Logout)
			protected.GET("/auth/me", deps.AuthHandler.Me)

			// Project routes
			protected.POST("/projects", deps.ProjectHandler.Create)
			protected.GET("/projects", deps.ProjectHandler.List)
			protected.GET("/projects/:id", deps.ProjectHandler.Get)
			protected.PUT("/projects/:id", deps.ProjectHandler.Update)
			protected.DELETE("/projects/:id", deps.ProjectHandler.Delete)
			protected.POST("/projects/:id/star", deps.ProjectHandler.Star)
			protected.DELETE("/projects/:id/star", deps.ProjectHandler.Unstar)
		}
	}

	return r
}
```

- [ ] **Step 4: 更新 main.go — 组装 project 模块**

修改 `forge-core/cmd/forge-core/main.go`，在 auth module 组装之后、router.Setup 之前添加：

```go
// 新增 import
import "github.com/shulex/forge/forge-core/internal/module/project"

// 在 authHandler := auth.NewHandler(authService) 之后添加:

// Project module
projectRepo := project.NewRepository(db)
projectService := project.NewService(projectRepo)
projectHandler := project.NewHandler(projectService)

// 修改 router.Setup 调用:
r := router.Setup(&router.Deps{
    AuthHandler:    authHandler,
    AuthService:    authService,
    ProjectHandler: projectHandler,
})
```

- [ ] **Step 5: 编译并验证 API**

```bash
cd forge-core
go mod tidy
go build ./cmd/forge-core
```

启动后端并测试：

```bash
cd forge-core && go run ./cmd/forge-core
```

```bash
# 先登录获取 token
TOKEN=$(curl -s -X POST http://localhost:8080/api/auth/login \
  -H "Content-Type: application/json" \
  -d '{"username":"admin","password":"admin123"}' | jq -r '.data.token')

# 创建项目
curl -X POST http://localhost:8080/api/projects \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  -d '{"name":"My First Project","description":"测试项目","default_branch":"main"}'
# 预期: {"code":0,"message":"ok","data":{"id":1,"name":"My First Project",...}}

# 列表项目
curl http://localhost:8080/api/projects \
  -H "Authorization: Bearer $TOKEN"
# 预期: {"code":0,"message":"ok","data":{"items":[...],"total":1,"page":1,"page_size":20}}

# 获取单个项目
curl http://localhost:8080/api/projects/1 \
  -H "Authorization: Bearer $TOKEN"
# 预期: {"code":0,"message":"ok","data":{"id":1,"name":"My First Project",...}}

# 收藏项目
curl -X POST http://localhost:8080/api/projects/1/star \
  -H "Authorization: Bearer $TOKEN"
# 预期: {"code":0,"message":"ok"}

# 列表（含收藏状态）
curl http://localhost:8080/api/projects \
  -H "Authorization: Bearer $TOKEN"
# 预期: items[0].starred = true

# 更新项目
curl -X PUT http://localhost:8080/api/projects/1 \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  -d '{"name":"Updated Project Name"}'
# 预期: {"code":0,"message":"ok","data":{"id":1,"name":"Updated Project Name",...}}

# 取消收藏
curl -X DELETE http://localhost:8080/api/projects/1/star \
  -H "Authorization: Bearer $TOKEN"
# 预期: {"code":0,"message":"ok"}

# 删除项目（软删除）
curl -X DELETE http://localhost:8080/api/projects/1 \
  -H "Authorization: Bearer $TOKEN"
# 预期: {"code":0,"message":"ok"}

# 再次列表（已删除的项目不应出现）
curl http://localhost:8080/api/projects \
  -H "Authorization: Bearer $TOKEN"
# 预期: {"code":0,"message":"ok","data":{"items":[],"total":0,...}}

# 重复名称测试
curl -X POST http://localhost:8080/api/projects \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  -d '{"name":"Test Project"}'
curl -X POST http://localhost:8080/api/projects \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  -d '{"name":"Test Project"}'
# 预期第二次: {"code":-1,"message":"项目名称已存在"}
```

- [ ] **Step 6: Commit**

```bash
git add forge-core/internal/module/project/service.go forge-core/internal/module/project/handler.go
git add forge-core/internal/router/router.go forge-core/cmd/forge-core/main.go
git commit -m "feat: add project CRUD + star/unstar API endpoints with route registration"
```

---

## Task 4: 前端 — 创建项目对话框

**Files:**
- Install shadcn/ui components: `dialog`, `textarea`, `select`
- Create: `forge-portal/components/create-project-dialog.tsx`

- [ ] **Step 1: 安装 shadcn/ui 组件**

```bash
cd forge-portal
npx shadcn@latest add dialog textarea select
```

- [ ] **Step 2: 创建 CreateProjectDialog 组件**

`forge-portal/components/create-project-dialog.tsx`：

```tsx
"use client";

import { useState } from "react";
import { api } from "@/lib/api";
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from "@/components/ui/dialog";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Textarea } from "@/components/ui/textarea";
import { Label } from "@/components/ui/label";
import { Plus } from "lucide-react";

interface CreateProjectDialogProps {
  onCreated: () => void;
}

export function CreateProjectDialog({ onCreated }: CreateProjectDialogProps) {
  const [open, setOpen] = useState(false);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState("");

  const [name, setName] = useState("");
  const [description, setDescription] = useState("");
  const [defaultBranch, setDefaultBranch] = useState("main");

  function resetForm() {
    setName("");
    setDescription("");
    setDefaultBranch("main");
    setError("");
  }

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    if (!name.trim()) {
      setError("请输入项目名称");
      return;
    }

    setLoading(true);
    setError("");

    try {
      await api.post("/projects", {
        name: name.trim(),
        description: description.trim() || null,
        default_branch: defaultBranch.trim() || "main",
      });
      setOpen(false);
      resetForm();
      onCreated();
    } catch (err: unknown) {
      const message = err instanceof Error ? err.message : "创建失败，请重试";
      setError(message);
    } finally {
      setLoading(false);
    }
  }

  return (
    <Dialog open={open} onOpenChange={(v) => { setOpen(v); if (!v) resetForm(); }}>
      <DialogTrigger asChild>
        <Button
          className="gap-2"
          style={{ background: "var(--primary)", boxShadow: "0 0 20px rgba(139, 92, 246, 0.3)" }}
        >
          <Plus size={16} />
          创建新项目
        </Button>
      </DialogTrigger>
      <DialogContent
        className="sm:max-w-[520px] border"
        style={{
          background: "var(--surface-1)",
          borderColor: "var(--border)",
          color: "var(--text-primary)",
        }}
      >
        <DialogHeader>
          <DialogTitle style={{ color: "var(--text-primary)" }}>创建新项目</DialogTitle>
        </DialogHeader>

        <form onSubmit={handleSubmit} className="space-y-5 mt-4">
          <div className="space-y-2">
            <Label style={{ color: "var(--text-secondary)" }}>
              项目名称 <span style={{ color: "var(--error)" }}>*</span>
            </Label>
            <Input
              value={name}
              onChange={(e) => setName(e.target.value)}
              placeholder="输入项目名称"
              maxLength={200}
              className="h-10 border"
              style={{
                background: "var(--input-bg)",
                borderColor: "var(--border)",
                color: "var(--text-primary)",
              }}
            />
          </div>

          <div className="space-y-2">
            <Label style={{ color: "var(--text-secondary)" }}>
              项目描述
            </Label>
            <Textarea
              value={description}
              onChange={(e) => setDescription(e.target.value)}
              placeholder="简要描述项目用途（可选）"
              rows={3}
              className="border resize-none"
              style={{
                background: "var(--input-bg)",
                borderColor: "var(--border)",
                color: "var(--text-primary)",
              }}
            />
          </div>

          <div className="space-y-2">
            <Label style={{ color: "var(--text-secondary)" }}>
              默认分支
            </Label>
            <Input
              value={defaultBranch}
              onChange={(e) => setDefaultBranch(e.target.value)}
              placeholder="main"
              className="h-10 border"
              style={{
                background: "var(--input-bg)",
                borderColor: "var(--border)",
                color: "var(--text-primary)",
              }}
            />
          </div>

          <div className="space-y-2">
            <Label style={{ color: "var(--text-secondary)" }}>
              代码平台
            </Label>
            <div
              className="h-10 flex items-center px-3 rounded-md border text-sm"
              style={{
                background: "var(--input-bg)",
                borderColor: "var(--border)",
                color: "var(--text-muted)",
              }}
            >
              暂未接入 — 将在后续版本支持 GitHub / Codeup
            </div>
          </div>

          {error && (
            <p className="text-sm" style={{ color: "var(--error)" }}>
              {error}
            </p>
          )}

          <div className="flex justify-end gap-3 pt-2">
            <Button
              type="button"
              variant="outline"
              onClick={() => { setOpen(false); resetForm(); }}
              className="border"
              style={{ borderColor: "var(--border)", color: "var(--text-secondary)" }}
            >
              取消
            </Button>
            <Button
              type="submit"
              disabled={loading || !name.trim()}
              style={{ background: "var(--primary)" }}
            >
              {loading ? "创建中..." : "创建项目"}
            </Button>
          </div>
        </form>
      </DialogContent>
    </Dialog>
  );
}
```

- [ ] **Step 3: 验证对话框渲染**

先启动前端查看对话框是否正确渲染（此时还没接入项目大厅页面，下一 Task 会接入）：

```bash
cd forge-portal && npm run dev
```

可临时在 projects/page.tsx 中 import 并渲染 `<CreateProjectDialog onCreated={() => {}} />` 确认样式无误。

- [ ] **Step 4: Commit**

```bash
git add forge-portal/components/create-project-dialog.tsx
git add forge-portal/components/ui/  # 新增的 shadcn 组件
git commit -m "feat: add create project dialog component with form validation"
```

---

## Task 5: 前端 — 项目卡片 + 项目大厅改造

**Files:**
- Install shadcn/ui component: `badge`
- Create: `forge-portal/components/project-card.tsx`
- Modify: `forge-portal/app/(dashboard)/projects/page.tsx`

- [ ] **Step 1: 安装 shadcn/ui badge 组件**

```bash
cd forge-portal
npx shadcn@latest add badge
```

- [ ] **Step 2: 创建 ProjectCard 组件**

`forge-portal/components/project-card.tsx`：

```tsx
"use client";

import { useRouter } from "next/navigation";
import { Star, GitBranch, Clock } from "lucide-react";
import { api } from "@/lib/api";

interface ProjectCardProps {
  project: {
    id: number;
    name: string;
    description?: string;
    status: string;
    code_platform?: string;
    default_branch: string;
    starred: boolean;
    updated_at: string;
  };
  onStarToggle: () => void;
}

function formatTime(dateStr: string): string {
  const date = new Date(dateStr);
  const now = new Date();
  const diff = now.getTime() - date.getTime();
  const minutes = Math.floor(diff / 60000);
  const hours = Math.floor(diff / 3600000);
  const days = Math.floor(diff / 86400000);

  if (minutes < 1) return "刚刚";
  if (minutes < 60) return `${minutes} 分钟前`;
  if (hours < 24) return `${hours} 小时前`;
  if (days < 30) return `${days} 天前`;
  return date.toLocaleDateString("zh-CN");
}

export function ProjectCard({ project, onStarToggle }: ProjectCardProps) {
  const router = useRouter();

  async function handleStarClick(e: React.MouseEvent) {
    e.stopPropagation();
    try {
      if (project.starred) {
        await api.delete(`/projects/${project.id}/star`);
      } else {
        await api.post(`/projects/${project.id}/star`);
      }
      onStarToggle();
    } catch {
      // silently fail
    }
  }

  return (
    <div
      className="group relative p-5 rounded-xl border cursor-pointer transition-all duration-200 hover:border-[rgba(139,92,246,0.3)] hover:shadow-[0_0_20px_rgba(139,92,246,0.08)]"
      style={{
        background: "var(--surface-1)",
        borderColor: "rgba(255, 255, 255, 0.06)",
      }}
      onClick={() => router.push(`/projects/${project.id}`)}
    >
      {/* Star button */}
      <button
        onClick={handleStarClick}
        className="absolute top-4 right-4 p-1.5 rounded-lg transition-colors hover:bg-[rgba(255,255,255,0.05)]"
        title={project.starred ? "取消收藏" : "收藏"}
      >
        <Star
          size={16}
          className={project.starred ? "fill-[#F59E0B] text-[#F59E0B]" : "text-[var(--text-muted)]"}
        />
      </button>

      {/* Project name */}
      <h3
        className="text-base font-medium mb-1.5 pr-8 truncate"
        style={{ color: "var(--text-primary)" }}
      >
        {project.name}
      </h3>

      {/* Description */}
      <p
        className="text-sm mb-4 line-clamp-2 min-h-[2.5rem]"
        style={{ color: "var(--text-secondary)" }}
      >
        {project.description || "暂无描述"}
      </p>

      {/* Footer metadata */}
      <div className="flex items-center gap-4 text-xs" style={{ color: "var(--text-muted)" }}>
        <div className="flex items-center gap-1">
          <GitBranch size={12} />
          <span>{project.default_branch}</span>
        </div>
        {project.code_platform && (
          <div className="flex items-center gap-1">
            <span className="capitalize">{project.code_platform}</span>
          </div>
        )}
        <div className="flex items-center gap-1 ml-auto">
          <Clock size={12} />
          <span>{formatTime(project.updated_at)}</span>
        </div>
      </div>
    </div>
  );
}
```

- [ ] **Step 3: 改造项目大厅页面 — 从空状态到真实列表**

替换 `forge-portal/app/(dashboard)/projects/page.tsx`：

```tsx
"use client";

import { useCallback, useEffect, useState } from "react";
import { api } from "@/lib/api";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { FolderOpen, GitBranch, Search, Star } from "lucide-react";
import { CreateProjectDialog } from "@/components/create-project-dialog";
import { ProjectCard } from "@/components/project-card";

interface ProjectItem {
  id: number;
  name: string;
  description?: string;
  status: string;
  code_platform?: string;
  default_branch: string;
  starred: boolean;
  updated_at: string;
}

interface ProjectListData {
  items: ProjectItem[];
  total: number;
  page: number;
  page_size: number;
}

export default function ProjectsPage() {
  const [data, setData] = useState<ProjectListData | null>(null);
  const [loading, setLoading] = useState(true);
  const [search, setSearch] = useState("");
  const [starredOnly, setStarredOnly] = useState(false);
  const [page, setPage] = useState(1);

  const fetchProjects = useCallback(async () => {
    setLoading(true);
    try {
      const params = new URLSearchParams();
      if (search) params.set("search", search);
      if (starredOnly) params.set("starred", "true");
      params.set("page", String(page));
      params.set("page_size", "20");

      const result = await api.get<ProjectListData>(`/projects?${params.toString()}`);
      setData(result);
    } catch {
      // handled by api.ts (401 redirect etc.)
    } finally {
      setLoading(false);
    }
  }, [search, starredOnly, page]);

  useEffect(() => {
    fetchProjects();
  }, [fetchProjects]);

  // Debounced search
  const [searchInput, setSearchInput] = useState("");
  useEffect(() => {
    const timer = setTimeout(() => {
      setSearch(searchInput);
      setPage(1);
    }, 300);
    return () => clearTimeout(timer);
  }, [searchInput]);

  const totalPages = data ? Math.ceil(data.total / data.page_size) : 0;
  const hasProjects = data && data.items.length > 0;
  const isEmpty = data && data.total === 0 && !search && !starredOnly;

  return (
    <div>
      {/* Header */}
      <div className="flex items-center justify-between mb-6">
        <h1 className="text-2xl font-semibold tracking-tight" style={{ color: "var(--text-primary)" }}>
          项目大厅
        </h1>
        <div className="flex gap-3">
          <Button
            variant="outline"
            className="gap-2 border"
            style={{ borderColor: "var(--border)", color: "var(--text-secondary)" }}
            disabled
          >
            <GitBranch size={16} />
            接入代码平台
          </Button>
          <CreateProjectDialog onCreated={fetchProjects} />
        </div>
      </div>

      {/* Search + Filter bar (hidden when no projects at all) */}
      {!isEmpty && (
        <div className="flex items-center gap-3 mb-5">
          <div className="relative flex-1 max-w-sm">
            <Search
              size={16}
              className="absolute left-3 top-1/2 -translate-y-1/2"
              style={{ color: "var(--text-muted)" }}
            />
            <Input
              value={searchInput}
              onChange={(e) => setSearchInput(e.target.value)}
              placeholder="搜索项目名称..."
              className="pl-9 h-9 border"
              style={{
                background: "var(--input-bg)",
                borderColor: "var(--border)",
                color: "var(--text-primary)",
              }}
            />
          </div>
          <Button
            variant="outline"
            size="sm"
            className={`gap-1.5 border transition-colors ${
              starredOnly ? "border-[#F59E0B]/30 text-[#F59E0B]" : ""
            }`}
            style={
              starredOnly
                ? { borderColor: "rgba(245, 158, 11, 0.3)", color: "#F59E0B", background: "rgba(245, 158, 11, 0.05)" }
                : { borderColor: "var(--border)", color: "var(--text-secondary)" }
            }
            onClick={() => { setStarredOnly(!starredOnly); setPage(1); }}
          >
            <Star size={14} className={starredOnly ? "fill-[#F59E0B]" : ""} />
            收藏
          </Button>
        </div>
      )}

      {/* Loading state */}
      {loading && !data && (
        <div
          className="flex items-center justify-center py-24 rounded-xl border"
          style={{ background: "var(--surface-1)", borderColor: "var(--border)" }}
        >
          <p className="text-sm" style={{ color: "var(--text-muted)" }}>加载中...</p>
        </div>
      )}

      {/* Empty state — no projects exist at all */}
      {isEmpty && (
        <div
          className="flex flex-col items-center justify-center py-24 rounded-xl border"
          style={{ background: "var(--surface-1)", borderColor: "var(--border)" }}
        >
          <div
            className="w-16 h-16 rounded-2xl flex items-center justify-center mb-4"
            style={{ background: "rgba(139, 92, 246, 0.1)" }}
          >
            <FolderOpen size={32} style={{ color: "var(--primary)" }} />
          </div>
          <h3 className="text-lg font-medium mb-2" style={{ color: "var(--text-primary)" }}>
            还没有项目
          </h3>
          <p className="text-sm mb-6" style={{ color: "var(--text-secondary)" }}>
            接入代码平台同步已有项目，或创建一个新项目开始
          </p>
          <div className="flex gap-3">
            <Button
              variant="outline"
              className="gap-2 border"
              style={{ borderColor: "var(--border)", color: "var(--text-secondary)" }}
              disabled
            >
              <GitBranch size={16} />
              接入代码平台
            </Button>
            <CreateProjectDialog onCreated={fetchProjects} />
          </div>
        </div>
      )}

      {/* No search results */}
      {data && data.items.length === 0 && !isEmpty && (
        <div
          className="flex flex-col items-center justify-center py-16 rounded-xl border"
          style={{ background: "var(--surface-1)", borderColor: "var(--border)" }}
        >
          <Search size={32} style={{ color: "var(--text-muted)" }} className="mb-3" />
          <p className="text-sm" style={{ color: "var(--text-secondary)" }}>
            未找到匹配的项目
          </p>
        </div>
      )}

      {/* Project cards grid */}
      {hasProjects && (
        <>
          <div className="grid grid-cols-1 md:grid-cols-2 xl:grid-cols-3 gap-4">
            {data.items.map((project) => (
              <ProjectCard
                key={project.id}
                project={project}
                onStarToggle={fetchProjects}
              />
            ))}
          </div>

          {/* Pagination */}
          {totalPages > 1 && (
            <div className="flex items-center justify-center gap-2 mt-6">
              <Button
                variant="outline"
                size="sm"
                disabled={page <= 1}
                onClick={() => setPage(page - 1)}
                className="border"
                style={{ borderColor: "var(--border)", color: "var(--text-secondary)" }}
              >
                上一页
              </Button>
              <span className="text-sm px-3" style={{ color: "var(--text-secondary)" }}>
                {page} / {totalPages}
              </span>
              <Button
                variant="outline"
                size="sm"
                disabled={page >= totalPages}
                onClick={() => setPage(page + 1)}
                className="border"
                style={{ borderColor: "var(--border)", color: "var(--text-secondary)" }}
              >
                下一页
              </Button>
            </div>
          )}
        </>
      )}
    </div>
  );
}
```

- [ ] **Step 4: 验证项目大厅**

确保后端运行中：

```bash
# Terminal 1: 后端
cd forge-core && go run ./cmd/forge-core

# Terminal 2: 前端
cd forge-portal && npm run dev
```

**验证清单**：

| # | 操作 | 预期结果 |
|---|------|---------|
| 1 | 登录后看到项目大厅 | 空状态卡片（还没有项目） |
| 2 | 点击"创建新项目" | 弹出对话框 |
| 3 | 不填名称点创建 | 错误提示"请输入项目名称" |
| 4 | 填写名称/描述后创建 | 对话框关闭，项目卡片出现 |
| 5 | 创建第二个项目 | 卡片网格展示两个项目 |
| 6 | 点击收藏星标 | 星标变为黄色填充 |
| 7 | 点击"收藏"筛选按钮 | 只显示收藏的项目 |
| 8 | 搜索框输入关键字 | 列表实时过滤 |
| 9 | 创建重复名称项目 | 显示"项目名称已存在"错误 |

- [ ] **Step 5: Commit**

```bash
git add forge-portal/components/project-card.tsx
git add forge-portal/components/ui/  # badge 组件
git add forge-portal/app/\(dashboard\)/projects/page.tsx
git commit -m "feat: implement project hall with project cards, search, star filter and create dialog"
```

---

## Task 6: 前端 — 项目详情布局 + 项目级侧边栏

**Files:**
- Create: `forge-portal/components/project-sidebar.tsx`
- Create: `forge-portal/app/(dashboard)/projects/[id]/layout.tsx`
- Create: `forge-portal/app/(dashboard)/projects/[id]/page.tsx`

- [ ] **Step 1: 创建项目级侧边栏组件**

`forge-portal/components/project-sidebar.tsx`：

```tsx
"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";
import {
  ArrowLeft,
  LayoutDashboard,
  ListTodo,
  GitPullRequest,
  TestTube,
  Rocket,
  Settings,
} from "lucide-react";

interface ProjectSidebarProps {
  projectId: string;
  projectName: string;
}

export function ProjectSidebar({ projectId, projectName }: ProjectSidebarProps) {
  const pathname = usePathname();
  const basePath = `/projects/${projectId}`;

  const navItems = [
    { href: basePath, label: "概览", icon: LayoutDashboard, exact: true },
    { href: `${basePath}/tasks`, label: "任务", icon: ListTodo },
    { href: `${basePath}/changes`, label: "变更", icon: GitPullRequest },
    { href: `${basePath}/tests`, label: "测试", icon: TestTube },
    { href: `${basePath}/deploy`, label: "部署", icon: Rocket },
    { href: `${basePath}/settings`, label: "设置", icon: Settings },
  ];

  function isActive(item: typeof navItems[0]) {
    if (item.exact) {
      return pathname === item.href;
    }
    return pathname.startsWith(item.href);
  }

  return (
    <aside
      className="w-60 h-screen flex flex-col border-r"
      style={{ background: "var(--surface-1)", borderColor: "var(--border)" }}
    >
      {/* Back to projects */}
      <div className="h-14 flex items-center px-4 border-b" style={{ borderColor: "var(--border)" }}>
        <Link
          href="/projects"
          className="flex items-center gap-2 text-sm transition-colors hover:text-[var(--text-primary)]"
          style={{ color: "var(--text-secondary)" }}
        >
          <ArrowLeft size={16} />
          返回项目大厅
        </Link>
      </div>

      {/* Project name */}
      <div className="px-4 py-3 border-b" style={{ borderColor: "var(--border)" }}>
        <h2
          className="text-sm font-medium truncate"
          style={{ color: "var(--text-primary)" }}
          title={projectName}
        >
          {projectName}
        </h2>
      </div>

      {/* Nav items */}
      <nav className="flex-1 p-3 space-y-1">
        {navItems.map((item) => {
          const active = isActive(item);
          return (
            <Link
              key={item.href}
              href={item.href}
              className={`flex items-center gap-3 px-3 py-2 rounded-lg text-sm transition-colors ${
                active
                  ? "text-[var(--text-primary)]"
                  : "text-[var(--text-secondary)] hover:text-[var(--text-primary)] hover:bg-[rgba(255,255,255,0.03)]"
              }`}
              style={active ? { background: "rgba(139, 92, 246, 0.1)" } : {}}
            >
              <item.icon size={18} />
              {item.label}
            </Link>
          );
        })}
      </nav>
    </aside>
  );
}
```

- [ ] **Step 2: 创建项目详情 Layout**

`forge-portal/app/(dashboard)/projects/[id]/layout.tsx`：

这个 Layout 替换了 dashboard layout 的默认侧边栏，改为项目级侧边栏：

```tsx
"use client";

import { useEffect, useState } from "react";
import { useParams } from "next/navigation";
import { api } from "@/lib/api";
import { Topbar } from "@/components/topbar";
import { ProjectSidebar } from "@/components/project-sidebar";

interface ProjectInfo {
  id: number;
  name: string;
  description?: string;
  status: string;
  default_branch: string;
}

export default function ProjectLayout({ children }: { children: React.ReactNode }) {
  const params = useParams();
  const projectId = params.id as string;
  const [project, setProject] = useState<ProjectInfo | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");

  useEffect(() => {
    async function fetchProject() {
      try {
        const data = await api.get<ProjectInfo>(`/projects/${projectId}`);
        setProject(data);
      } catch {
        setError("项目不存在或无权访问");
      } finally {
        setLoading(false);
      }
    }
    fetchProject();
  }, [projectId]);

  if (loading) {
    return (
      <div className="flex h-screen" style={{ background: "var(--background)" }}>
        <div className="w-60 border-r" style={{ background: "var(--surface-1)", borderColor: "var(--border)" }} />
        <div className="flex-1 flex items-center justify-center">
          <p className="text-sm" style={{ color: "var(--text-muted)" }}>加载中...</p>
        </div>
      </div>
    );
  }

  if (error || !project) {
    return (
      <div className="flex h-screen" style={{ background: "var(--background)" }}>
        <div className="w-60 border-r" style={{ background: "var(--surface-1)", borderColor: "var(--border)" }} />
        <div className="flex-1 flex items-center justify-center">
          <p className="text-sm" style={{ color: "var(--error)" }}>{error || "项目不存在"}</p>
        </div>
      </div>
    );
  }

  return (
    <div className="flex h-screen" style={{ background: "var(--background)" }}>
      <ProjectSidebar projectId={projectId} projectName={project.name} />
      <div className="flex-1 flex flex-col overflow-hidden">
        <Topbar />
        <main className="flex-1 overflow-auto p-6">
          {children}
        </main>
      </div>
    </div>
  );
}
```

**重要**: 这个 layout 有自己的 sidebar + topbar，不需要外层 dashboard layout 的 sidebar。需要在 Next.js App Router 中正确处理嵌套 — `(dashboard)/layout.tsx` 会包裹 `projects/[id]/layout.tsx`。

为了避免双侧边栏，需要修改 `(dashboard)/layout.tsx` 让它检测是否在项目详情页：

修改 `forge-portal/app/(dashboard)/layout.tsx`：

```tsx
"use client";

import { useAuth } from "@/lib/auth";
import { useRouter, usePathname } from "next/navigation";
import { useEffect } from "react";
import { Sidebar } from "@/components/sidebar";
import { Topbar } from "@/components/topbar";

export default function DashboardLayout({ children }: { children: React.ReactNode }) {
  const { user, loading } = useAuth();
  const router = useRouter();
  const pathname = usePathname();

  useEffect(() => {
    if (!loading && !user) {
      router.push("/login");
    }
  }, [user, loading, router]);

  if (loading) {
    return (
      <div className="min-h-screen flex items-center justify-center" style={{ background: "var(--background)" }}>
        <div className="text-sm" style={{ color: "var(--text-muted)" }}>加载中...</div>
      </div>
    );
  }

  if (!user) return null;

  // Project detail pages have their own layout (project sidebar + topbar)
  // Match /projects/<id> and any sub-paths, but NOT /projects alone
  const isProjectDetail = /^\/projects\/\d+/.test(pathname);
  if (isProjectDetail) {
    return <>{children}</>;
  }

  return (
    <div className="flex h-screen" style={{ background: "var(--background)" }}>
      <Sidebar />
      <div className="flex-1 flex flex-col overflow-hidden">
        <Topbar />
        <main className="flex-1 overflow-auto p-6">
          {children}
        </main>
      </div>
    </div>
  );
}
```

- [ ] **Step 3: 创建概览页**

`forge-portal/app/(dashboard)/projects/[id]/page.tsx`：

```tsx
"use client";

import { useEffect, useState } from "react";
import { useParams } from "next/navigation";
import { api } from "@/lib/api";
import { GitBranch, Clock, Info, ListTodo, TestTube, Rocket } from "lucide-react";

interface ProjectDetail {
  id: number;
  name: string;
  description?: string;
  status: string;
  code_platform?: string;
  default_branch: string;
  risk_threshold: number;
  auto_merge: boolean;
  created_at: string;
  updated_at: string;
}

function formatDate(dateStr: string): string {
  return new Date(dateStr).toLocaleDateString("zh-CN", {
    year: "numeric",
    month: "long",
    day: "numeric",
    hour: "2-digit",
    minute: "2-digit",
  });
}

export default function ProjectOverviewPage() {
  const params = useParams();
  const projectId = params.id as string;
  const [project, setProject] = useState<ProjectDetail | null>(null);

  useEffect(() => {
    async function fetchProject() {
      try {
        const data = await api.get<ProjectDetail>(`/projects/${projectId}`);
        setProject(data);
      } catch {
        // error handled by layout
      }
    }
    fetchProject();
  }, [projectId]);

  if (!project) return null;

  const placeholders = [
    { icon: ListTodo, label: "任务", desc: "AI 任务将在后续版本中启用" },
    { icon: TestTube, label: "测试", desc: "四层自动化测试将在后续版本中启用" },
    { icon: Rocket, label: "部署", desc: "自动化部署将在后续版本中启用" },
  ];

  return (
    <div className="space-y-6">
      <h1 className="text-2xl font-semibold tracking-tight" style={{ color: "var(--text-primary)" }}>
        概览
      </h1>

      {/* Project info card */}
      <div
        className="p-6 rounded-xl border"
        style={{ background: "var(--surface-1)", borderColor: "rgba(255, 255, 255, 0.06)" }}
      >
        <div className="flex items-start justify-between mb-4">
          <div>
            <h2 className="text-lg font-medium" style={{ color: "var(--text-primary)" }}>
              {project.name}
            </h2>
            <p className="text-sm mt-1" style={{ color: "var(--text-secondary)" }}>
              {project.description || "暂无描述"}
            </p>
          </div>
          <span
            className="px-2.5 py-0.5 rounded-full text-xs font-medium"
            style={{
              background: "rgba(16, 185, 129, 0.1)",
              color: "#10B981",
            }}
          >
            {project.status}
          </span>
        </div>

        <div className="grid grid-cols-2 md:grid-cols-4 gap-4 pt-4 border-t" style={{ borderColor: "var(--border)" }}>
          <div>
            <div className="flex items-center gap-1.5 text-xs mb-1" style={{ color: "var(--text-muted)" }}>
              <GitBranch size={12} />
              默认分支
            </div>
            <p className="text-sm" style={{ color: "var(--text-primary)" }}>{project.default_branch}</p>
          </div>
          <div>
            <div className="flex items-center gap-1.5 text-xs mb-1" style={{ color: "var(--text-muted)" }}>
              <Info size={12} />
              代码平台
            </div>
            <p className="text-sm" style={{ color: "var(--text-primary)" }}>
              {project.code_platform || "未接入"}
            </p>
          </div>
          <div>
            <div className="flex items-center gap-1.5 text-xs mb-1" style={{ color: "var(--text-muted)" }}>
              <Clock size={12} />
              创建时间
            </div>
            <p className="text-sm" style={{ color: "var(--text-primary)" }}>{formatDate(project.created_at)}</p>
          </div>
          <div>
            <div className="flex items-center gap-1.5 text-xs mb-1" style={{ color: "var(--text-muted)" }}>
              <Clock size={12} />
              最后更新
            </div>
            <p className="text-sm" style={{ color: "var(--text-primary)" }}>{formatDate(project.updated_at)}</p>
          </div>
        </div>
      </div>

      {/* Placeholder cards for future features */}
      <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
        {placeholders.map((item) => (
          <div
            key={item.label}
            className="p-5 rounded-xl border flex flex-col items-center justify-center text-center py-10"
            style={{
              background: "var(--surface-1)",
              borderColor: "rgba(255, 255, 255, 0.06)",
            }}
          >
            <div
              className="w-10 h-10 rounded-xl flex items-center justify-center mb-3"
              style={{ background: "rgba(139, 92, 246, 0.08)" }}
            >
              <item.icon size={20} style={{ color: "var(--primary)" }} />
            </div>
            <h3 className="text-sm font-medium mb-1" style={{ color: "var(--text-primary)" }}>
              {item.label}
            </h3>
            <p className="text-xs" style={{ color: "var(--text-muted)" }}>
              {item.desc}
            </p>
          </div>
        ))}
      </div>
    </div>
  );
}
```

- [ ] **Step 4: 验证项目详情布局**

```bash
cd forge-portal && npm run dev
```

**验证清单**：

| # | 操作 | 预期结果 |
|---|------|---------|
| 1 | 在项目大厅点击一个项目卡片 | 跳转到 /projects/[id]，看到项目级侧边栏 |
| 2 | 侧边栏显示 | "返回项目大厅" + 项目名 + 6 个导航项 |
| 3 | 概览页显示 | 项目信息卡片 + 3 个占位卡片 |
| 4 | 点击"返回项目大厅" | 回到 /projects |
| 5 | 侧边栏"概览"高亮 | 紫色背景 |

- [ ] **Step 5: Commit**

```bash
git add forge-portal/components/project-sidebar.tsx
git add forge-portal/app/\(dashboard\)/projects/\[id\]/layout.tsx
git add forge-portal/app/\(dashboard\)/projects/\[id\]/page.tsx
git add forge-portal/app/\(dashboard\)/layout.tsx
git commit -m "feat: add project detail layout with project-scoped sidebar and overview page"
```

---

## Task 7: 前端 — 项目占位子页面

**Files:**
- Create: `forge-portal/app/(dashboard)/projects/[id]/tasks/page.tsx`
- Create: `forge-portal/app/(dashboard)/projects/[id]/changes/page.tsx`
- Create: `forge-portal/app/(dashboard)/projects/[id]/tests/page.tsx`
- Create: `forge-portal/app/(dashboard)/projects/[id]/deploy/page.tsx`

- [ ] **Step 1: 创建通用占位页面生成函数**

为了避免重复代码，每个占位页面结构相同但内容不同。创建以下四个文件：

`forge-portal/app/(dashboard)/projects/[id]/tasks/page.tsx`：

```tsx
import { ListTodo } from "lucide-react";

export default function TasksPage() {
  return (
    <div className="flex flex-col items-center justify-center py-32">
      <div
        className="w-16 h-16 rounded-2xl flex items-center justify-center mb-4"
        style={{ background: "rgba(139, 92, 246, 0.08)" }}
      >
        <ListTodo size={32} style={{ color: "var(--primary)" }} />
      </div>
      <h2 className="text-lg font-medium mb-2" style={{ color: "var(--text-primary)" }}>
        任务管理
      </h2>
      <p className="text-sm" style={{ color: "var(--text-muted)" }}>
        AI 任务创建与执行将在后续版本中启用
      </p>
    </div>
  );
}
```

`forge-portal/app/(dashboard)/projects/[id]/changes/page.tsx`：

```tsx
import { GitPullRequest } from "lucide-react";

export default function ChangesPage() {
  return (
    <div className="flex flex-col items-center justify-center py-32">
      <div
        className="w-16 h-16 rounded-2xl flex items-center justify-center mb-4"
        style={{ background: "rgba(139, 92, 246, 0.08)" }}
      >
        <GitPullRequest size={32} style={{ color: "var(--primary)" }} />
      </div>
      <h2 className="text-lg font-medium mb-2" style={{ color: "var(--text-primary)" }}>
        变更管理
      </h2>
      <p className="text-sm" style={{ color: "var(--text-muted)" }}>
        代码变更与 Pull Request 管理将在后续版本中启用
      </p>
    </div>
  );
}
```

`forge-portal/app/(dashboard)/projects/[id]/tests/page.tsx`：

```tsx
import { TestTube } from "lucide-react";

export default function TestsPage() {
  return (
    <div className="flex flex-col items-center justify-center py-32">
      <div
        className="w-16 h-16 rounded-2xl flex items-center justify-center mb-4"
        style={{ background: "rgba(139, 92, 246, 0.08)" }}
      >
        <TestTube size={32} style={{ color: "var(--primary)" }} />
      </div>
      <h2 className="text-lg font-medium mb-2" style={{ color: "var(--text-primary)" }}>
        测试中心
      </h2>
      <p className="text-sm" style={{ color: "var(--text-muted)" }}>
        四层自动化测试将在后续版本中启用
      </p>
    </div>
  );
}
```

`forge-portal/app/(dashboard)/projects/[id]/deploy/page.tsx`：

```tsx
import { Rocket } from "lucide-react";

export default function DeployPage() {
  return (
    <div className="flex flex-col items-center justify-center py-32">
      <div
        className="w-16 h-16 rounded-2xl flex items-center justify-center mb-4"
        style={{ background: "rgba(139, 92, 246, 0.08)" }}
      >
        <Rocket size={32} style={{ color: "var(--primary)" }} />
      </div>
      <h2 className="text-lg font-medium mb-2" style={{ color: "var(--text-primary)" }}>
        部署管理
      </h2>
      <p className="text-sm" style={{ color: "var(--text-muted)" }}>
        自动化部署与灰度发布将在后续版本中启用
      </p>
    </div>
  );
}
```

- [ ] **Step 2: 验证占位页面**

```bash
cd forge-portal && npm run dev
```

**验证**：进入一个项目详情页，依次点击侧边栏的"任务"、"变更"、"测试"、"部署"链接，每个都应该显示对应的占位内容。侧边栏对应项高亮。

- [ ] **Step 3: Commit**

```bash
git add forge-portal/app/\(dashboard\)/projects/\[id\]/tasks/
git add forge-portal/app/\(dashboard\)/projects/\[id\]/changes/
git add forge-portal/app/\(dashboard\)/projects/\[id\]/tests/
git add forge-portal/app/\(dashboard\)/projects/\[id\]/deploy/
git commit -m "feat: add placeholder pages for tasks, changes, tests, and deploy"
```

---

## Task 8: 前端 — 项目设置页（编辑 + 删除）

**Files:**
- Install shadcn/ui component: `alert-dialog`, `separator`
- Create: `forge-portal/app/(dashboard)/projects/[id]/settings/page.tsx`

- [ ] **Step 1: 安装 shadcn/ui 组件**

```bash
cd forge-portal
npx shadcn@latest add alert-dialog separator
```

- [ ] **Step 2: 创建项目设置页**

`forge-portal/app/(dashboard)/projects/[id]/settings/page.tsx`：

```tsx
"use client";

import { useEffect, useState } from "react";
import { useParams, useRouter } from "next/navigation";
import { api, ApiError } from "@/lib/api";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Textarea } from "@/components/ui/textarea";
import { Label } from "@/components/ui/label";
import { Separator } from "@/components/ui/separator";
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
  AlertDialogTrigger,
} from "@/components/ui/alert-dialog";
import { Save, Trash2 } from "lucide-react";

interface ProjectDetail {
  id: number;
  name: string;
  description?: string;
  default_branch: string;
}

export default function ProjectSettingsPage() {
  const params = useParams();
  const router = useRouter();
  const projectId = params.id as string;

  const [project, setProject] = useState<ProjectDetail | null>(null);
  const [loading, setLoading] = useState(true);

  // Form state
  const [name, setName] = useState("");
  const [description, setDescription] = useState("");
  const [defaultBranch, setDefaultBranch] = useState("");
  const [saving, setSaving] = useState(false);
  const [saveError, setSaveError] = useState("");
  const [saveSuccess, setSaveSuccess] = useState(false);

  // Delete state
  const [deleting, setDeleting] = useState(false);
  const [deleteConfirmName, setDeleteConfirmName] = useState("");

  useEffect(() => {
    async function fetchProject() {
      try {
        const data = await api.get<ProjectDetail>(`/projects/${projectId}`);
        setProject(data);
        setName(data.name);
        setDescription(data.description || "");
        setDefaultBranch(data.default_branch);
      } catch {
        // handled by layout
      } finally {
        setLoading(false);
      }
    }
    fetchProject();
  }, [projectId]);

  async function handleSave(e: React.FormEvent) {
    e.preventDefault();
    if (!name.trim()) {
      setSaveError("项目名称不能为空");
      return;
    }

    setSaving(true);
    setSaveError("");
    setSaveSuccess(false);

    try {
      const updated = await api.put<ProjectDetail>(`/projects/${projectId}`, {
        name: name.trim(),
        description: description.trim(),
        default_branch: defaultBranch.trim() || "main",
      });
      setProject(updated);
      setSaveSuccess(true);
      setTimeout(() => setSaveSuccess(false), 3000);
    } catch (err: unknown) {
      const message = err instanceof ApiError ? err.message : "保存失败，请重试";
      setSaveError(message);
    } finally {
      setSaving(false);
    }
  }

  async function handleDelete() {
    setDeleting(true);
    try {
      await api.delete(`/projects/${projectId}`);
      router.push("/projects");
    } catch {
      setDeleting(false);
    }
  }

  if (loading || !project) return null;

  const hasChanges =
    name !== project.name ||
    description !== (project.description || "") ||
    defaultBranch !== project.default_branch;

  return (
    <div className="max-w-2xl space-y-8">
      <h1 className="text-2xl font-semibold tracking-tight" style={{ color: "var(--text-primary)" }}>
        项目设置
      </h1>

      {/* General settings */}
      <div
        className="p-6 rounded-xl border"
        style={{ background: "var(--surface-1)", borderColor: "rgba(255, 255, 255, 0.06)" }}
      >
        <h2 className="text-base font-medium mb-5" style={{ color: "var(--text-primary)" }}>
          基本信息
        </h2>

        <form onSubmit={handleSave} className="space-y-5">
          <div className="space-y-2">
            <Label style={{ color: "var(--text-secondary)" }}>
              项目名称 <span style={{ color: "var(--error)" }}>*</span>
            </Label>
            <Input
              value={name}
              onChange={(e) => setName(e.target.value)}
              maxLength={200}
              className="h-10 border"
              style={{
                background: "var(--input-bg)",
                borderColor: "var(--border)",
                color: "var(--text-primary)",
              }}
            />
          </div>

          <div className="space-y-2">
            <Label style={{ color: "var(--text-secondary)" }}>
              项目描述
            </Label>
            <Textarea
              value={description}
              onChange={(e) => setDescription(e.target.value)}
              rows={3}
              className="border resize-none"
              style={{
                background: "var(--input-bg)",
                borderColor: "var(--border)",
                color: "var(--text-primary)",
              }}
            />
          </div>

          <div className="space-y-2">
            <Label style={{ color: "var(--text-secondary)" }}>
              默认分支
            </Label>
            <Input
              value={defaultBranch}
              onChange={(e) => setDefaultBranch(e.target.value)}
              className="h-10 border"
              style={{
                background: "var(--input-bg)",
                borderColor: "var(--border)",
                color: "var(--text-primary)",
              }}
            />
          </div>

          {saveError && (
            <p className="text-sm" style={{ color: "var(--error)" }}>
              {saveError}
            </p>
          )}

          {saveSuccess && (
            <p className="text-sm" style={{ color: "var(--success)" }}>
              保存成功
            </p>
          )}

          <div className="flex justify-end">
            <Button
              type="submit"
              disabled={saving || !hasChanges || !name.trim()}
              className="gap-2"
              style={{
                background: hasChanges ? "var(--primary)" : undefined,
                opacity: hasChanges ? 1 : 0.5,
              }}
            >
              <Save size={16} />
              {saving ? "保存中..." : "保存修改"}
            </Button>
          </div>
        </form>
      </div>

      {/* Danger zone */}
      <div
        className="p-6 rounded-xl border"
        style={{
          background: "var(--surface-1)",
          borderColor: "rgba(239, 68, 68, 0.2)",
        }}
      >
        <h2 className="text-base font-medium mb-2" style={{ color: "var(--error)" }}>
          危险区域
        </h2>
        <p className="text-sm mb-5" style={{ color: "var(--text-secondary)" }}>
          删除项目后，项目数据将被归档且不可恢复。请谨慎操作。
        </p>

        <Separator className="mb-5" style={{ background: "var(--border)" }} />

        <div className="flex items-center justify-between">
          <div>
            <p className="text-sm font-medium" style={{ color: "var(--text-primary)" }}>
              删除项目
            </p>
            <p className="text-xs mt-0.5" style={{ color: "var(--text-muted)" }}>
              将项目状态标记为 ARCHIVED
            </p>
          </div>

          <AlertDialog>
            <AlertDialogTrigger asChild>
              <Button
                variant="outline"
                className="gap-2 border"
                style={{
                  borderColor: "rgba(239, 68, 68, 0.3)",
                  color: "var(--error)",
                }}
              >
                <Trash2 size={16} />
                删除项目
              </Button>
            </AlertDialogTrigger>
            <AlertDialogContent
              style={{
                background: "var(--surface-1)",
                borderColor: "var(--border)",
                color: "var(--text-primary)",
              }}
            >
              <AlertDialogHeader>
                <AlertDialogTitle style={{ color: "var(--text-primary)" }}>
                  确认删除项目
                </AlertDialogTitle>
                <AlertDialogDescription style={{ color: "var(--text-secondary)" }}>
                  此操作不可撤销。请输入项目名称 <strong style={{ color: "var(--text-primary)" }}>{project.name}</strong> 以确认删除。
                </AlertDialogDescription>
              </AlertDialogHeader>

              <Input
                value={deleteConfirmName}
                onChange={(e) => setDeleteConfirmName(e.target.value)}
                placeholder={project.name}
                className="h-10 border mt-2"
                style={{
                  background: "var(--input-bg)",
                  borderColor: "var(--border)",
                  color: "var(--text-primary)",
                }}
              />

              <AlertDialogFooter>
                <AlertDialogCancel
                  className="border"
                  style={{ borderColor: "var(--border)", color: "var(--text-secondary)" }}
                  onClick={() => setDeleteConfirmName("")}
                >
                  取消
                </AlertDialogCancel>
                <AlertDialogAction
                  disabled={deleteConfirmName !== project.name || deleting}
                  onClick={handleDelete}
                  style={{
                    background: deleteConfirmName === project.name ? "var(--error)" : undefined,
                    opacity: deleteConfirmName === project.name ? 1 : 0.5,
                  }}
                >
                  {deleting ? "删除中..." : "确认删除"}
                </AlertDialogAction>
              </AlertDialogFooter>
            </AlertDialogContent>
          </AlertDialog>
        </div>
      </div>
    </div>
  );
}
```

- [ ] **Step 3: 验证设置页**

```bash
cd forge-portal && npm run dev
```

**验证清单**：

| # | 操作 | 预期结果 |
|---|------|---------|
| 1 | 进入项目详情，点侧边栏"设置" | 看到设置页，表单预填项目信息 |
| 2 | 修改项目名称后点"保存修改" | 绿色成功提示 |
| 3 | 不修改任何字段 | "保存修改"按钮灰色禁用 |
| 4 | 改为已存在的项目名称 | 显示"项目名称已存在"错误 |
| 5 | 点击"删除项目" | 弹出确认对话框 |
| 6 | 不输入项目名称 | "确认删除"按钮灰色禁用 |
| 7 | 输入正确项目名称后确认删除 | 跳转回项目大厅，该项目消失 |

- [ ] **Step 4: Commit**

```bash
git add forge-portal/app/\(dashboard\)/projects/\[id\]/settings/
git add forge-portal/components/ui/  # alert-dialog, separator
git commit -m "feat: implement project settings page with edit and delete functionality"
```

---

## Task 9: 端到端验证 + 清理

**Files:**
- 无新文件，只做验证和可能的微调

- [ ] **Step 1: 完整重启验证**

停止所有服务，从零启动：

```bash
# 重建数据库
docker compose -f docker-compose.dev.yml down -v
docker compose -f docker-compose.dev.yml up -d

# 等待 PostgreSQL 就绪（约 5 秒）

# 启动后端
cd forge-core && go run ./cmd/forge-core
# 预期: migration 001 和 002 都应用成功

# 启动前端
cd forge-portal && npm run dev
```

- [ ] **Step 2: 完整用户流程验证**

| # | 操作 | 预期结果 |
|---|------|---------|
| 1 | 打开 http://localhost:3000 | 重定向到 /login |
| 2 | 用 admin / admin123 登录 | 进入项目大厅 |
| 3 | 看到空状态 | "还没有项目" + 创建按钮 |
| 4 | 点击"创建新项目" | 弹出对话框 |
| 5 | 创建项目 "E-Commerce Backend" | 卡片出现 |
| 6 | 创建项目 "Mobile App" | 两张卡片 |
| 7 | 收藏 "Mobile App" | 星标变黄色 |
| 8 | 点击"收藏"筛选 | 只显示 "Mobile App" |
| 9 | 关闭收藏筛选 | 显示所有项目 |
| 10 | 搜索 "ecom" | 只显示 "E-Commerce Backend" |
| 11 | 清空搜索 | 显示所有项目 |
| 12 | 点击 "E-Commerce Backend" | 进入项目详情概览页 |
| 13 | 侧边栏显示项目名 + 6 个导航 | 概览高亮 |
| 14 | 点击"任务"/"变更"/"测试"/"部署" | 分别看到对应占位页 |
| 15 | 点击"设置" | 看到设置表单 |
| 16 | 改名为 "EC Shop Backend"，保存 | 成功提示，侧边栏项目名更新（需刷新页面） |
| 17 | 点击"删除项目"，输入名称确认 | 跳转回项目大厅，项目消失 |
| 18 | 点击"返回项目大厅" | 回到 /projects |
| 19 | 登出 | 回到 /login |
| 20 | 直接访问 /projects/1 | 跳转到 /login |

- [ ] **Step 3: 后端 API 验证（可选，用 curl 补充测试）**

```bash
TOKEN=$(curl -s -X POST http://localhost:8080/api/auth/login \
  -H "Content-Type: application/json" \
  -d '{"username":"admin","password":"admin123"}' | jq -r '.data.token')

# 未登录访问
curl http://localhost:8080/api/projects
# 预期: {"code":-1,"message":"请先登录"}

# 带 token 访问
curl http://localhost:8080/api/projects -H "Authorization: Bearer $TOKEN"
# 预期: {"code":0,"message":"ok","data":{"items":[...],...}}
```

- [ ] **Step 4: 确认编译无警告**

```bash
# Go
cd forge-core && go vet ./...
# 预期: 无输出（无警告）

# Next.js
cd forge-portal && npm run build
# 预期: 编译成功
```

- [ ] **Step 5: Final Commit（如有微调）**

```bash
# 如果前面验证发现小问题需要修复，统一提交
git add -A
git commit -m "fix: address S2 integration issues found during end-to-end testing"
```

如果一切正常无需修复，则跳过此步。
