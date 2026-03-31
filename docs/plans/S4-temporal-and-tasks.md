# S4 — Temporal 集成 + 任务管理 + Kanban + SSE 实时更新

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 集成 Temporal 工作流引擎，实现任务创建 → 骨架 Workflow 自动流转 → Kanban 看板展示 → SSE 实时推送的完整闭环。

**Architecture:** forge-core 新增 task module + Temporal client/worker（嵌入 forge-core 进程），forge-portal 新增 Kanban 看板页 + 任务详情页 + SSE 实时更新。Temporal Server 通过 Docker Compose 运行。

**Tech Stack:** Go 1.22 + Gin + Temporal Go SDK, Next.js 15 (App Router) + TypeScript + Tailwind CSS 4 + shadcn/ui, PostgreSQL 16, Redis 7, Temporal 1.24

**Depends on:** S1 (auth + login + dashboard), S2 (project CRUD + pages), S3 (GitHub OAuth + repo sync)

---

## 前置说明

### 本切片交付后你可以做什么

1. `docker compose -f docker-compose.dev.yml up -d` 启动 PostgreSQL + Redis + Temporal + Temporal UI
2. `cd forge-core && go run ./cmd/forge-core` 启动后端（包含嵌入式 Temporal Worker）
3. `cd forge-portal && npm run dev` 启动前端
4. 浏览器打开 `http://localhost:3000`，登录后选择一个项目
5. 进入项目的 "Tasks" 页面，点击 "New Task" 创建任务
6. 看到任务出现在 Kanban 看板的 "Submitted" 列
7. 任务自动流转（Analyzing → Planning → Generating → Reviewing → Testing → Deploying → Completed）
8. 点击任务卡片进入详情页，看到步骤时间线实时更新
9. 访问 `http://localhost:8233` 可在 Temporal UI 中查看 Workflow 执行详情

### 与后续切片的关系

S4 的 Temporal Workflow 是**骨架**，每个步骤只是 sleep + 状态更新。后续切片替换为真实活动：
- S6 (AI Worker): 替换 ANALYZING/PLANNING/GENERATING 步骤为 LangGraph AI 调用
- S7 (DevOps Worker): 替换 TESTING/DEPLOYING 步骤为真实 CI/CD 流水线
- S8 (Constraint Worker): 替换 REVIEWING 步骤为真实 Lint/Security 扫描

---

## 文件结构

### forge-core 新增/修改

```
forge-core/
├── internal/
│   ├── module/
│   │   └── task/
│   │       ├── model.go               # 数据模型 + DTO
│   │       ├── repository.go           # 数据库操作
│   │       ├── service.go              # 业务逻辑（含 Temporal 触发）
│   │       ├── handler.go              # HTTP handler (CRUD)
│   │       └── sse.go                  # SSE 实时推送 handler
│   ├── temporal/
│   │   ├── client.go                   # Temporal client 封装
│   │   ├── worker.go                   # Temporal worker 启动
│   │   ├── workflow/
│   │   │   └── task_workflow.go        # 任务骨架 Workflow
│   │   └── activity/
│   │       └── task_activities.go      # 任务步骤 Activities
│   └── router/
│       └── router.go                   # (修改) 注册 task 路由
├── cmd/
│   └── forge-core/
│       └── main.go                     # (修改) 初始化 Temporal + task module
├── migrations/
│   └── 004_init_tasks.sql              # tasks + task_steps DDL
└── go.mod                              # (修改) 添加 Temporal SDK 依赖
```

### forge-portal 新增

```
forge-portal/
├── app/
│   └── (dashboard)/
│       └── projects/
│           └── [id]/
│               └── tasks/
│                   ├── page.tsx                # Kanban 看板页
│                   └── [taskId]/
│                       └── page.tsx            # 任务详情页
├── components/
│   ├── tasks/
│   │   ├── kanban-board.tsx            # Kanban 看板组件
│   │   ├── task-card.tsx               # 任务卡片组件
│   │   ├── create-task-dialog.tsx      # 创建任务对话框
│   │   ├── task-detail.tsx             # 任务详情组件
│   │   └── step-timeline.tsx           # 步骤时间线组件
│   └── ui/
│       └── (shadcn 按需添加)
├── lib/
│   ├── tasks.ts                        # Task API 调用封装
│   └── use-task-stream.ts              # SSE Hook
```

### 根目录修改

```
docker-compose.dev.yml                  # (修改) 添加 Temporal + Temporal UI
```

---

## Task 1: Docker Compose 添加 Temporal

**Files:**
- Modify: `docker-compose.dev.yml`

- [ ] **Step 1: 在 docker-compose.dev.yml 中添加 Temporal 服务**

在现有的 `services` 下（postgres、redis 之后）追加 `temporal` 和 `temporal-ui` 两个服务：

```yaml
  temporal:
    image: temporalio/auto-setup:1.24
    container_name: forge-temporal
    environment:
      - DB=postgresql
      - DB_PORT=5432
      - POSTGRES_USER=forge
      - POSTGRES_PWD=forge_dev_2026
      - POSTGRES_SEEDS=postgres
      - DYNAMIC_CONFIG_FILE_PATH=config/dynamicconfig/development-sql.yaml
    ports:
      - "7233:7233"
    depends_on:
      postgres:
        condition: service_healthy

  temporal-ui:
    image: temporalio/ui:2.26.2
    container_name: forge-temporal-ui
    environment:
      - TEMPORAL_ADDRESS=temporal:7233
      - TEMPORAL_CORS_ORIGINS=http://localhost:3000
    ports:
      - "8233:8080"
    depends_on:
      - temporal
```

> **注意**: Temporal auto-setup 镜像会自动在 PostgreSQL 中创建 `temporal` 和 `temporal_visibility` 数据库。S1 的 `docker/postgres/init.sql` 已预建了 `forge_temporal` 数据库，但 auto-setup 使用自己的数据库名。两者不冲突。

- [ ] **Step 2: 启动并验证 Temporal**

```bash
docker compose -f docker-compose.dev.yml up -d
```

**验证**：
```bash
# Temporal gRPC 端口可达
docker exec forge-temporal tctl cluster health
# 预期: SERVING

# Temporal UI 可访问
curl -s http://localhost:8233 | head -5
# 预期: HTML 页面内容

# 原有服务仍然正常
docker exec forge-postgres psql -U forge -d forge_main -c "SELECT 1;"
docker exec forge-redis redis-cli -a forge_redis_2026 ping
```

- [ ] **Step 3: Commit**

```bash
git add docker-compose.dev.yml
git commit -m "infra: add Temporal Server and Temporal UI to dev environment"
```

---

## Task 2: 任务数据库迁移

**Files:**
- Create: `forge-core/migrations/004_init_tasks.sql`

- [ ] **Step 1: 创建 tasks 迁移脚本**

`forge-core/migrations/004_init_tasks.sql`：

```sql
-- Task status lifecycle:
-- SUBMITTED → ANALYZING → PLANNING → GENERATING → REVIEWING → TESTING → DEPLOYING → COMPLETED
-- Any step can transition to FAILED

-- Tasks table
CREATE TABLE IF NOT EXISTS engine.tasks (
    id              BIGSERIAL PRIMARY KEY,
    tenant_id       BIGINT NOT NULL,
    project_id      BIGINT NOT NULL REFERENCES engine.projects(id),
    title           VARCHAR(500),
    requirement     TEXT NOT NULL,
    source          VARCHAR(20) NOT NULL DEFAULT 'WEB',
    status          VARCHAR(30) NOT NULL DEFAULT 'SUBMITTED',
    workflow_id     VARCHAR(200),
    workflow_run_id VARCHAR(200),
    risk_level      VARCHAR(10),
    risk_score      INT,
    branch_name     VARCHAR(200),
    files_changed   INT,
    lines_added     INT,
    lines_deleted   INT,
    created_by      BIGINT NOT NULL REFERENCES auth.users(id),
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    completed_at    TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_tasks_project_id ON engine.tasks(project_id);
CREATE INDEX IF NOT EXISTS idx_tasks_tenant_id ON engine.tasks(tenant_id);
CREATE INDEX IF NOT EXISTS idx_tasks_status ON engine.tasks(status);
CREATE INDEX IF NOT EXISTS idx_tasks_created_by ON engine.tasks(created_by);
CREATE INDEX IF NOT EXISTS idx_tasks_workflow_id ON engine.tasks(workflow_id);

-- Task steps table
CREATE TABLE IF NOT EXISTS engine.task_steps (
    id              BIGSERIAL PRIMARY KEY,
    task_id         BIGINT NOT NULL REFERENCES engine.tasks(id) ON DELETE CASCADE,
    name            VARCHAR(200) NOT NULL,
    step_type       VARCHAR(30) NOT NULL,
    status          VARCHAR(20) NOT NULL DEFAULT 'PENDING',
    input           JSONB,
    output          JSONB,
    error           JSONB,
    attempt         INT NOT NULL DEFAULT 1,
    max_attempts    INT NOT NULL DEFAULT 3,
    started_at      TIMESTAMPTZ,
    completed_at    TIMESTAMPTZ,
    duration_ms     BIGINT
);

CREATE INDEX IF NOT EXISTS idx_task_steps_task_id ON engine.task_steps(task_id);
CREATE INDEX IF NOT EXISTS idx_task_steps_status ON engine.task_steps(status);
```

- [ ] **Step 2: 验证迁移执行**

```bash
# 重启后端触发迁移
cd forge-core && go run ./cmd/forge-core

# 验证表已创建
docker exec forge-postgres psql -U forge -d forge_main -c "\dt engine.*"
# 预期: 包含 tasks 和 task_steps 表

docker exec forge-postgres psql -U forge -d forge_main -c "\di engine.*"
# 预期: 包含所有索引
```

- [ ] **Step 3: Commit**

```bash
git add forge-core/migrations/004_init_tasks.sql
git commit -m "feat: add tasks and task_steps database migration"
```

---

## Task 3: Task Module — Model + Repository + Service + Handler

**Files:**
- Create: `forge-core/internal/module/task/model.go`
- Create: `forge-core/internal/module/task/repository.go`
- Create: `forge-core/internal/module/task/service.go`
- Create: `forge-core/internal/module/task/handler.go`
- Modify: `forge-core/internal/router/router.go`
- Modify: `forge-core/cmd/forge-core/main.go`

- [ ] **Step 1: 创建 model.go — 数据模型 + DTO**

`forge-core/internal/module/task/model.go`：

```go
package task

import (
	"time"
)

// Task status constants
const (
	StatusSubmitted  = "SUBMITTED"
	StatusAnalyzing  = "ANALYZING"
	StatusPlanning   = "PLANNING"
	StatusGenerating = "GENERATING"
	StatusReviewing  = "REVIEWING"
	StatusTesting    = "TESTING"
	StatusDeploying  = "DEPLOYING"
	StatusCompleted  = "COMPLETED"
	StatusFailed     = "FAILED"
)

// Step status constants
const (
	StepPending   = "PENDING"
	StepRunning   = "RUNNING"
	StepCompleted = "COMPLETED"
	StepFailed    = "FAILED"
	StepSkipped   = "SKIPPED"
)

// Step type constants (maps to workflow steps)
const (
	StepTypeAnalyze  = "ANALYZE"
	StepTypePlan     = "PLAN"
	StepTypeGenerate = "GENERATE"
	StepTypeReview   = "REVIEW"
	StepTypeTest     = "TEST"
	StepTypeDeploy   = "DEPLOY"
)

// Task source constants
const (
	SourceWeb  = "WEB"
	SourceIM   = "IM"
	SourceAPI  = "API"
)

// AllSteps defines the default step sequence for a task workflow
var AllSteps = []struct {
	Name     string
	StepType string
}{
	{"需求分析", StepTypeAnalyze},
	{"方案规划", StepTypePlan},
	{"代码生成", StepTypeGenerate},
	{"代码审查", StepTypeReview},
	{"自动化测试", StepTypeTest},
	{"部署发布", StepTypeDeploy},
}

// DB models

type Task struct {
	ID            int64      `json:"id"`
	TenantID      int64      `json:"tenant_id"`
	ProjectID     int64      `json:"project_id"`
	Title         *string    `json:"title,omitempty"`
	Requirement   string     `json:"requirement"`
	Source        string     `json:"source"`
	Status        string     `json:"status"`
	WorkflowID    *string    `json:"workflow_id,omitempty"`
	WorkflowRunID *string    `json:"workflow_run_id,omitempty"`
	RiskLevel     *string    `json:"risk_level,omitempty"`
	RiskScore     *int       `json:"risk_score,omitempty"`
	BranchName    *string    `json:"branch_name,omitempty"`
	FilesChanged  *int       `json:"files_changed,omitempty"`
	LinesAdded    *int       `json:"lines_added,omitempty"`
	LinesDeleted  *int       `json:"lines_deleted,omitempty"`
	CreatedBy     int64      `json:"created_by"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
	CompletedAt   *time.Time `json:"completed_at,omitempty"`
}

type TaskStep struct {
	ID          int64      `json:"id"`
	TaskID      int64      `json:"task_id"`
	Name        string     `json:"name"`
	StepType    string     `json:"step_type"`
	Status      string     `json:"status"`
	Input       *string    `json:"input,omitempty"`
	Output      *string    `json:"output,omitempty"`
	Error       *string    `json:"error,omitempty"`
	Attempt     int        `json:"attempt"`
	MaxAttempts int        `json:"max_attempts"`
	StartedAt   *time.Time `json:"started_at,omitempty"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`
	DurationMs  *int64     `json:"duration_ms,omitempty"`
}

// Request DTOs

type CreateTaskRequest struct {
	Title       string `json:"title"`
	Requirement string `json:"requirement" binding:"required"`
}

// Response DTOs

type TaskResponse struct {
	Task  Task       `json:"task"`
	Steps []TaskStep `json:"steps,omitempty"`
}

type TaskListResponse struct {
	Tasks []Task `json:"tasks"`
	Total int64  `json:"total"`
}

// SSE event DTO

type TaskProgressEvent struct {
	Type     string `json:"type"`
	TaskID   int64  `json:"task_id"`
	Status   string `json:"status"`
	StepType string `json:"step_type,omitempty"`
	StepName string `json:"step_name,omitempty"`
	Progress int    `json:"progress"`
}
```

- [ ] **Step 2: 创建 repository.go — 数据库操作**

`forge-core/internal/module/task/repository.go`：

```go
package task

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

func (r *Repository) Create(ctx context.Context, t *Task) error {
	err := r.db.QueryRow(ctx,
		`INSERT INTO engine.tasks (tenant_id, project_id, title, requirement, source, status, created_by)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)
		 RETURNING id, created_at, updated_at`,
		t.TenantID, t.ProjectID, t.Title, t.Requirement, t.Source, t.Status, t.CreatedBy,
	).Scan(&t.ID, &t.CreatedAt, &t.UpdatedAt)
	if err != nil {
		return fmt.Errorf("create task: %w", err)
	}
	return nil
}

func (r *Repository) FindByID(ctx context.Context, taskID int64) (*Task, error) {
	t := &Task{}
	err := r.db.QueryRow(ctx,
		`SELECT id, tenant_id, project_id, title, requirement, source, status,
		        workflow_id, workflow_run_id, risk_level, risk_score,
		        branch_name, files_changed, lines_added, lines_deleted,
		        created_by, created_at, updated_at, completed_at
		 FROM engine.tasks WHERE id = $1`,
		taskID,
	).Scan(&t.ID, &t.TenantID, &t.ProjectID, &t.Title, &t.Requirement, &t.Source, &t.Status,
		&t.WorkflowID, &t.WorkflowRunID, &t.RiskLevel, &t.RiskScore,
		&t.BranchName, &t.FilesChanged, &t.LinesAdded, &t.LinesDeleted,
		&t.CreatedBy, &t.CreatedAt, &t.UpdatedAt, &t.CompletedAt)
	if err != nil {
		return nil, fmt.Errorf("find task: %w", err)
	}
	return t, nil
}

func (r *Repository) ListByProject(ctx context.Context, projectID int64, status string, offset, limit int) ([]Task, int64, error) {
	// Count query
	countSQL := `SELECT COUNT(*) FROM engine.tasks WHERE project_id = $1`
	args := []interface{}{projectID}
	argIdx := 2

	if status != "" {
		countSQL += fmt.Sprintf(" AND status = $%d", argIdx)
		args = append(args, status)
		argIdx++
	}

	var total int64
	if err := r.db.QueryRow(ctx, countSQL, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count tasks: %w", err)
	}

	// List query
	listSQL := `SELECT id, tenant_id, project_id, title, requirement, source, status,
	                   workflow_id, workflow_run_id, risk_level, risk_score,
	                   branch_name, files_changed, lines_added, lines_deleted,
	                   created_by, created_at, updated_at, completed_at
	            FROM engine.tasks WHERE project_id = $1`

	listArgs := []interface{}{projectID}
	listArgIdx := 2

	if status != "" {
		listSQL += fmt.Sprintf(" AND status = $%d", listArgIdx)
		listArgs = append(listArgs, status)
		listArgIdx++
	}

	listSQL += " ORDER BY created_at DESC"
	listSQL += fmt.Sprintf(" LIMIT $%d OFFSET $%d", listArgIdx, listArgIdx+1)
	listArgs = append(listArgs, limit, offset)

	rows, err := r.db.Query(ctx, listSQL, listArgs...)
	if err != nil {
		return nil, 0, fmt.Errorf("list tasks: %w", err)
	}
	defer rows.Close()

	var tasks []Task
	for rows.Next() {
		var t Task
		if err := rows.Scan(&t.ID, &t.TenantID, &t.ProjectID, &t.Title, &t.Requirement, &t.Source, &t.Status,
			&t.WorkflowID, &t.WorkflowRunID, &t.RiskLevel, &t.RiskScore,
			&t.BranchName, &t.FilesChanged, &t.LinesAdded, &t.LinesDeleted,
			&t.CreatedBy, &t.CreatedAt, &t.UpdatedAt, &t.CompletedAt); err != nil {
			return nil, 0, fmt.Errorf("scan task: %w", err)
		}
		tasks = append(tasks, t)
	}
	return tasks, total, nil
}

func (r *Repository) UpdateStatus(ctx context.Context, taskID int64, status string) error {
	_, err := r.db.Exec(ctx,
		`UPDATE engine.tasks SET status = $1, updated_at = NOW() WHERE id = $2`,
		status, taskID,
	)
	return err
}

func (r *Repository) UpdateWorkflowIDs(ctx context.Context, taskID int64, workflowID, runID string) error {
	_, err := r.db.Exec(ctx,
		`UPDATE engine.tasks SET workflow_id = $1, workflow_run_id = $2, updated_at = NOW() WHERE id = $3`,
		workflowID, runID, taskID,
	)
	return err
}

func (r *Repository) MarkCompleted(ctx context.Context, taskID int64, status string) error {
	_, err := r.db.Exec(ctx,
		`UPDATE engine.tasks SET status = $1, completed_at = NOW(), updated_at = NOW() WHERE id = $2`,
		status, taskID,
	)
	return err
}

// Task step operations

func (r *Repository) CreateSteps(ctx context.Context, taskID int64, steps []struct {
	Name     string
	StepType string
}) error {
	for _, s := range steps {
		_, err := r.db.Exec(ctx,
			`INSERT INTO engine.task_steps (task_id, name, step_type, status)
			 VALUES ($1, $2, $3, $4)`,
			taskID, s.Name, s.StepType, StepPending,
		)
		if err != nil {
			return fmt.Errorf("create step %s: %w", s.Name, err)
		}
	}
	return nil
}

func (r *Repository) GetStepsByTaskID(ctx context.Context, taskID int64) ([]TaskStep, error) {
	rows, err := r.db.Query(ctx,
		`SELECT id, task_id, name, step_type, status, input, output, error,
		        attempt, max_attempts, started_at, completed_at, duration_ms
		 FROM engine.task_steps WHERE task_id = $1 ORDER BY id`,
		taskID,
	)
	if err != nil {
		return nil, fmt.Errorf("get steps: %w", err)
	}
	defer rows.Close()

	var steps []TaskStep
	for rows.Next() {
		var s TaskStep
		if err := rows.Scan(&s.ID, &s.TaskID, &s.Name, &s.StepType, &s.Status,
			&s.Input, &s.Output, &s.Error,
			&s.Attempt, &s.MaxAttempts, &s.StartedAt, &s.CompletedAt, &s.DurationMs); err != nil {
			return nil, fmt.Errorf("scan step: %w", err)
		}
		steps = append(steps, s)
	}
	return steps, nil
}

func (r *Repository) UpdateStepStatus(ctx context.Context, taskID int64, stepType, status string) error {
	_, err := r.db.Exec(ctx,
		`UPDATE engine.task_steps
		 SET status = $1,
		     started_at = CASE WHEN $1 = 'RUNNING' AND started_at IS NULL THEN NOW() ELSE started_at END,
		     completed_at = CASE WHEN $1 IN ('COMPLETED', 'FAILED', 'SKIPPED') THEN NOW() ELSE completed_at END,
		     duration_ms = CASE WHEN $1 IN ('COMPLETED', 'FAILED', 'SKIPPED') AND started_at IS NOT NULL
		                        THEN EXTRACT(EPOCH FROM (NOW() - started_at)) * 1000
		                        ELSE duration_ms END
		 WHERE task_id = $2 AND step_type = $3`,
		status, taskID, stepType,
	)
	return err
}

func (r *Repository) UpdateStepError(ctx context.Context, taskID int64, stepType string, errJSON string) error {
	_, err := r.db.Exec(ctx,
		`UPDATE engine.task_steps SET error = $1::jsonb WHERE task_id = $2 AND step_type = $3`,
		errJSON, taskID, stepType,
	)
	return err
}
```

- [ ] **Step 3: 创建 service.go — 业务逻辑**

`forge-core/internal/module/task/service.go`：

注意：Service 依赖 Temporal client 来启动 Workflow，但 Temporal 相关代码在 Task 4 创建。此处先用接口解耦。

```go
package task

import (
	"context"
	"fmt"
	"log/slog"
)

// WorkflowStarter 是 Temporal client 的抽象接口，由 Task 4 实现
type WorkflowStarter interface {
	StartTaskWorkflow(ctx context.Context, taskID int64, tenantID int64, projectID int64) (workflowID string, runID string, err error)
}

type Service struct {
	repo            *Repository
	workflowStarter WorkflowStarter
}

func NewService(repo *Repository, ws WorkflowStarter) *Service {
	return &Service{
		repo:            repo,
		workflowStarter: ws,
	}
}

func (s *Service) CreateTask(ctx context.Context, tenantID, projectID, userID int64, req *CreateTaskRequest) (*TaskResponse, error) {
	// Build title: use provided title or first 50 chars of requirement
	title := req.Title
	if title == "" {
		title = req.Requirement
		if len(title) > 50 {
			title = title[:50] + "..."
		}
	}

	t := &Task{
		TenantID:    tenantID,
		ProjectID:   projectID,
		Title:       &title,
		Requirement: req.Requirement,
		Source:      SourceWeb,
		Status:      StatusSubmitted,
		CreatedBy:   userID,
	}

	if err := s.repo.Create(ctx, t); err != nil {
		return nil, fmt.Errorf("create task: %w", err)
	}

	// Create default steps
	if err := s.repo.CreateSteps(ctx, t.ID, AllSteps); err != nil {
		return nil, fmt.Errorf("create steps: %w", err)
	}

	// Start Temporal workflow
	if s.workflowStarter != nil {
		workflowID, runID, err := s.workflowStarter.StartTaskWorkflow(ctx, t.ID, tenantID, projectID)
		if err != nil {
			slog.Error("failed to start workflow", "task_id", t.ID, "error", err)
			// Don't fail the task creation — task remains SUBMITTED, can be retried
		} else {
			_ = s.repo.UpdateWorkflowIDs(ctx, t.ID, workflowID, runID)
			t.WorkflowID = &workflowID
			t.WorkflowRunID = &runID
		}
	}

	steps, _ := s.repo.GetStepsByTaskID(ctx, t.ID)

	return &TaskResponse{
		Task:  *t,
		Steps: steps,
	}, nil
}

func (s *Service) GetTask(ctx context.Context, taskID int64) (*TaskResponse, error) {
	t, err := s.repo.FindByID(ctx, taskID)
	if err != nil {
		return nil, fmt.Errorf("get task: %w", err)
	}

	steps, err := s.repo.GetStepsByTaskID(ctx, taskID)
	if err != nil {
		return nil, fmt.Errorf("get steps: %w", err)
	}

	return &TaskResponse{
		Task:  *t,
		Steps: steps,
	}, nil
}

func (s *Service) ListTasks(ctx context.Context, projectID int64, status string, page, pageSize int) (*TaskListResponse, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	offset := (page - 1) * pageSize

	tasks, total, err := s.repo.ListByProject(ctx, projectID, status, offset, pageSize)
	if err != nil {
		return nil, fmt.Errorf("list tasks: %w", err)
	}

	if tasks == nil {
		tasks = []Task{}
	}

	return &TaskListResponse{
		Tasks: tasks,
		Total: total,
	}, nil
}
```

- [ ] **Step 4: 创建 handler.go — HTTP handler**

`forge-core/internal/module/task/handler.go`：

```go
package task

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

// POST /api/projects/:id/tasks
func (h *Handler) CreateTask(c *gin.Context) {
	projectID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.Fail(c, http.StatusBadRequest, "无效的项目ID")
		return
	}

	var req CreateTaskRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, "请输入需求描述")
		return
	}

	tenantID, _ := c.Get("tenant_id")
	userID, _ := c.Get("user_id")

	result, err := h.service.CreateTask(c.Request.Context(),
		tenantID.(int64), projectID, userID.(int64), &req)
	if err != nil {
		response.Fail(c, http.StatusInternalServerError, "创建任务失败")
		return
	}

	response.OK(c, result)
}

// GET /api/projects/:id/tasks
func (h *Handler) ListTasks(c *gin.Context) {
	projectID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.Fail(c, http.StatusBadRequest, "无效的项目ID")
		return
	}

	status := c.Query("status")
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "50"))

	result, err := h.service.ListTasks(c.Request.Context(), projectID, status, page, pageSize)
	if err != nil {
		response.Fail(c, http.StatusInternalServerError, "获取任务列表失败")
		return
	}

	response.OK(c, result)
}

// GET /api/projects/:id/tasks/:taskId
func (h *Handler) GetTask(c *gin.Context) {
	taskID, err := strconv.ParseInt(c.Param("taskId"), 10, 64)
	if err != nil {
		response.Fail(c, http.StatusBadRequest, "无效的任务ID")
		return
	}

	result, err := h.service.GetTask(c.Request.Context(), taskID)
	if err != nil {
		response.Fail(c, http.StatusInternalServerError, "获取任务详情失败")
		return
	}

	response.OK(c, result)
}
```

- [ ] **Step 5: 更新 router.go 注册 task 路由**

修改 `forge-core/internal/router/router.go`，在 Deps 中添加 TaskHandler，注册 task 路由：

```go
package router

import (
	"github.com/gin-gonic/gin"
	"github.com/shulex/forge/forge-core/internal/middleware"
	"github.com/shulex/forge/forge-core/internal/module/auth"
	"github.com/shulex/forge/forge-core/internal/module/task"
)

type Deps struct {
	AuthHandler *auth.Handler
	AuthService *auth.Service
	TaskHandler *task.Handler
	TaskSSE     *task.SSEHandler // 在 Task 5 中添加
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

			// Task routes (S4)
			if deps.TaskHandler != nil {
				protected.POST("/projects/:id/tasks", deps.TaskHandler.CreateTask)
				protected.GET("/projects/:id/tasks", deps.TaskHandler.ListTasks)
				protected.GET("/projects/:id/tasks/:taskId", deps.TaskHandler.GetTask)
			}

			// SSE routes (S4 — Task 5)
			if deps.TaskSSE != nil {
				protected.GET("/stream/tasks/:taskId", deps.TaskSSE.Stream)
			}
		}
	}

	return r
}
```

> **注意**: `deps.TaskSSE` 在 Task 5 中实现。这里先用 nil check 保护，避免 Task 3 编译失败。

- [ ] **Step 6: 更新 main.go 组装 task 模块**

修改 `forge-core/cmd/forge-core/main.go`，在 auth 模块初始化之后、router.Setup 之前添加：

```go
// Task module (S4)
// WorkflowStarter will be set in Task 4 when Temporal is initialized
taskRepo := task.NewRepository(db)
taskService := task.NewService(taskRepo, nil) // nil = no Temporal yet, set in Task 4
taskHandler := task.NewHandler(taskService)

// Router
r := router.Setup(&router.Deps{
	AuthHandler: authHandler,
	AuthService: authService,
	TaskHandler: taskHandler,
	TaskSSE:     nil, // Set in Task 5
})
```

- [ ] **Step 7: 安装依赖并验证编译**

```bash
cd forge-core
go mod tidy
go build ./cmd/forge-core
```

- [ ] **Step 8: 启动并测试 CRUD API**

```bash
cd forge-core && go run ./cmd/forge-core
```

**测试**（需要先登录拿 token，且需要有项目数据）：
```bash
# 登录
TOKEN=$(curl -s -X POST http://localhost:8080/api/auth/login \
  -H "Content-Type: application/json" \
  -d '{"username":"admin","password":"admin123"}' | jq -r '.data.token')

# 创建任务（假设 project_id = 1 存在）
curl -X POST http://localhost:8080/api/projects/1/tasks \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  -d '{"requirement":"实现用户注册功能，支持手机号和邮箱注册"}'
# 预期: {"code":0,"data":{"task":{"id":1,"status":"SUBMITTED",...},"steps":[...]}}

# 查看任务列表
curl http://localhost:8080/api/projects/1/tasks \
  -H "Authorization: Bearer $TOKEN"
# 预期: {"code":0,"data":{"tasks":[...],"total":1}}

# 查看任务详情
curl http://localhost:8080/api/projects/1/tasks/1 \
  -H "Authorization: Bearer $TOKEN"
# 预期: {"code":0,"data":{"task":{...},"steps":[6 pending steps]}}
```

- [ ] **Step 9: Commit**

```bash
git add forge-core/internal/module/task/ forge-core/internal/router/router.go
git commit -m "feat: implement task module with CRUD API and step tracking"
```

---

## Task 4: Temporal Client + Workflow + Activities

**Files:**
- Create: `forge-core/internal/temporal/client.go`
- Create: `forge-core/internal/temporal/worker.go`
- Create: `forge-core/internal/temporal/workflow/task_workflow.go`
- Create: `forge-core/internal/temporal/activity/task_activities.go`
- Modify: `forge-core/cmd/forge-core/main.go`
- Modify: `forge-core/go.mod`

- [ ] **Step 1: 安装 Temporal Go SDK**

```bash
cd forge-core
go get go.temporal.io/sdk@latest
go mod tidy
```

- [ ] **Step 2: 创建 client.go — Temporal client 封装**

`forge-core/internal/temporal/client.go`：

```go
package temporal

import (
	"context"
	"fmt"
	"log/slog"

	"go.temporal.io/sdk/client"
)

const (
	TaskQueueName = "forge-task-queue"
	Namespace     = "default"
)

type Client struct {
	inner client.Client
}

func NewClient(ctx context.Context, hostPort string) (*Client, error) {
	c, err := client.Dial(client.Options{
		HostPort:  hostPort,
		Namespace: Namespace,
	})
	if err != nil {
		return nil, fmt.Errorf("dial temporal: %w", err)
	}

	slog.Info("temporal connected", "host", hostPort)
	return &Client{inner: c}, nil
}

func (c *Client) Inner() client.Client {
	return c.inner
}

func (c *Client) Close() {
	c.inner.Close()
}

// StartTaskWorkflow implements task.WorkflowStarter interface
func (c *Client) StartTaskWorkflow(ctx context.Context, taskID int64, tenantID int64, projectID int64) (string, string, error) {
	workflowID := fmt.Sprintf("task-%d", taskID)

	options := client.StartWorkflowOptions{
		ID:        workflowID,
		TaskQueue: TaskQueueName,
	}

	// Import the workflow function reference — we use a string-based approach
	// to avoid circular dependency. The workflow is registered in the worker.
	we, err := c.inner.ExecuteWorkflow(ctx, options, "TaskWorkflow", TaskWorkflowInput{
		TaskID:    taskID,
		TenantID:  tenantID,
		ProjectID: projectID,
	})
	if err != nil {
		return "", "", fmt.Errorf("start task workflow: %w", err)
	}

	slog.Info("workflow started", "workflow_id", we.GetID(), "run_id", we.GetRunID(), "task_id", taskID)
	return we.GetID(), we.GetRunID(), nil
}

// TaskWorkflowInput is shared between client and workflow
type TaskWorkflowInput struct {
	TaskID    int64 `json:"task_id"`
	TenantID  int64 `json:"tenant_id"`
	ProjectID int64 `json:"project_id"`
}
```

- [ ] **Step 3: 创建 task_activities.go — 步骤 Activity 实现**

`forge-core/internal/temporal/activity/task_activities.go`：

每个 Activity 做三件事：(1) 标记步骤为 RUNNING，(2) sleep 模拟工作，(3) 标记步骤为 COMPLETED，(4) 更新任务状态。

```go
package activity

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type TaskActivities struct {
	db *pgxpool.Pool
}

func NewTaskActivities(db *pgxpool.Pool) *TaskActivities {
	return &TaskActivities{db: db}
}

type StepInput struct {
	TaskID     int64  `json:"task_id"`
	StepType   string `json:"step_type"`
	TaskStatus string `json:"task_status"` // The task-level status to set (e.g., ANALYZING)
	Duration   int    `json:"duration"`    // Simulated work duration in seconds
}

type StepOutput struct {
	TaskID   int64  `json:"task_id"`
	StepType string `json:"step_type"`
	Status   string `json:"status"`
}

// ExecuteStep is the generic skeleton activity for all workflow steps.
// In future slices, each step type gets its own real activity.
func (a *TaskActivities) ExecuteStep(ctx context.Context, input StepInput) (*StepOutput, error) {
	slog.Info("step started", "task_id", input.TaskID, "step", input.StepType, "status", input.TaskStatus)

	// 1. Update task status
	_, err := a.db.Exec(ctx,
		`UPDATE engine.tasks SET status = $1, updated_at = NOW() WHERE id = $2`,
		input.TaskStatus, input.TaskID,
	)
	if err != nil {
		return nil, fmt.Errorf("update task status: %w", err)
	}

	// 2. Mark step as RUNNING
	_, err = a.db.Exec(ctx,
		`UPDATE engine.task_steps
		 SET status = 'RUNNING', started_at = NOW()
		 WHERE task_id = $1 AND step_type = $2`,
		input.TaskID, input.StepType,
	)
	if err != nil {
		return nil, fmt.Errorf("mark step running: %w", err)
	}

	// 3. Simulate work
	time.Sleep(time.Duration(input.Duration) * time.Second)

	// 4. Mark step as COMPLETED
	_, err = a.db.Exec(ctx,
		`UPDATE engine.task_steps
		 SET status = 'COMPLETED',
		     completed_at = NOW(),
		     duration_ms = EXTRACT(EPOCH FROM (NOW() - started_at)) * 1000
		 WHERE task_id = $1 AND step_type = $2`,
		input.TaskID, input.StepType,
	)
	if err != nil {
		return nil, fmt.Errorf("mark step completed: %w", err)
	}

	slog.Info("step completed", "task_id", input.TaskID, "step", input.StepType)

	return &StepOutput{
		TaskID:   input.TaskID,
		StepType: input.StepType,
		Status:   "COMPLETED",
	}, nil
}

// CompleteTask marks the task as COMPLETED. Called at the end of the workflow.
func (a *TaskActivities) CompleteTask(ctx context.Context, taskID int64) error {
	_, err := a.db.Exec(ctx,
		`UPDATE engine.tasks SET status = 'COMPLETED', completed_at = NOW(), updated_at = NOW() WHERE id = $1`,
		taskID,
	)
	if err != nil {
		return fmt.Errorf("complete task: %w", err)
	}
	slog.Info("task completed", "task_id", taskID)
	return nil
}

// FailTask marks the task as FAILED. Called when any step fails.
func (a *TaskActivities) FailTask(ctx context.Context, taskID int64, errMsg string) error {
	_, err := a.db.Exec(ctx,
		`UPDATE engine.tasks SET status = 'FAILED', completed_at = NOW(), updated_at = NOW() WHERE id = $1`,
		taskID,
	)
	if err != nil {
		return fmt.Errorf("fail task: %w", err)
	}
	slog.Error("task failed", "task_id", taskID, "error", errMsg)
	return nil
}
```

- [ ] **Step 4: 创建 task_workflow.go — 骨架 Workflow**

`forge-core/internal/temporal/workflow/task_workflow.go`：

```go
package workflow

import (
	"time"

	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"

	temporalpkg "github.com/shulex/forge/forge-core/internal/temporal"
	"github.com/shulex/forge/forge-core/internal/temporal/activity"
)

// Step definitions with simulated durations
var workflowSteps = []activity.StepInput{
	{StepType: "ANALYZE", TaskStatus: "ANALYZING", Duration: 2},
	{StepType: "PLAN", TaskStatus: "PLANNING", Duration: 2},
	{StepType: "GENERATE", TaskStatus: "GENERATING", Duration: 3},
	{StepType: "REVIEW", TaskStatus: "REVIEWING", Duration: 2},
	{StepType: "TEST", TaskStatus: "TESTING", Duration: 2},
	{StepType: "DEPLOY", TaskStatus: "DEPLOYING", Duration: 2},
}

// TaskWorkflow is the skeleton workflow that transitions through all steps.
// Each step is a Temporal Activity that simulates work with a sleep.
// Future slices replace individual steps with real AI/DevOps/Constraint activities.
func TaskWorkflow(ctx workflow.Context, input temporalpkg.TaskWorkflowInput) error {
	logger := workflow.GetLogger(ctx)
	logger.Info("TaskWorkflow started", "task_id", input.TaskID)

	// Activity options: 60s timeout per step, 3 retries
	actCtx := workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
		StartToCloseTimeout: 60 * time.Second,
		RetryPolicy: &temporal.RetryPolicy{
			MaximumAttempts: 3,
		},
	})

	// Execute each step sequentially
	for _, step := range workflowSteps {
		stepInput := activity.StepInput{
			TaskID:     input.TaskID,
			StepType:   step.StepType,
			TaskStatus: step.TaskStatus,
			Duration:   step.Duration,
		}

		var result activity.StepOutput
		err := workflow.ExecuteActivity(actCtx, "ExecuteStep", stepInput).Get(ctx, &result)
		if err != nil {
			logger.Error("step failed", "task_id", input.TaskID, "step", step.StepType, "error", err)

			// Mark task as failed
			_ = workflow.ExecuteActivity(actCtx, "FailTask", input.TaskID, err.Error()).Get(ctx, nil)
			return err
		}
	}

	// All steps completed — mark task as done
	err := workflow.ExecuteActivity(actCtx, "CompleteTask", input.TaskID).Get(ctx, nil)
	if err != nil {
		logger.Error("failed to complete task", "task_id", input.TaskID, "error", err)
		return err
	}

	logger.Info("TaskWorkflow completed", "task_id", input.TaskID)
	return nil
}
```

- [ ] **Step 5: 创建 worker.go — Temporal Worker 启动**

`forge-core/internal/temporal/worker.go`：

```go
package temporal

import (
	"log/slog"

	"github.com/jackc/pgx/v5/pgxpool"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/worker"

	"github.com/shulex/forge/forge-core/internal/temporal/activity"
	"github.com/shulex/forge/forge-core/internal/temporal/workflow"
)

// StartWorker creates and starts a Temporal worker in a goroutine.
// The worker is embedded in forge-core for Phase 1 (no separate worker process).
func StartWorker(c client.Client, db *pgxpool.Pool) (worker.Worker, error) {
	w := worker.New(c, TaskQueueName, worker.Options{})

	// Register workflow
	w.RegisterWorkflowWithOptions(workflow.TaskWorkflow, worker.RegisterWorkflowOptions{
		Name: "TaskWorkflow",
	})

	// Register activities
	activities := activity.NewTaskActivities(db)
	w.RegisterActivityWithOptions(activities.ExecuteStep, worker.RegisterActivityOptions{
		Name: "ExecuteStep",
	})
	w.RegisterActivityWithOptions(activities.CompleteTask, worker.RegisterActivityOptions{
		Name: "CompleteTask",
	})
	w.RegisterActivityWithOptions(activities.FailTask, worker.RegisterActivityOptions{
		Name: "FailTask",
	})

	// Start worker in background
	if err := w.Start(); err != nil {
		return nil, err
	}

	slog.Info("temporal worker started", "queue", TaskQueueName)
	return w, nil
}
```

- [ ] **Step 6: 更新 config.go 添加 Temporal 配置**

修改 `forge-core/internal/config/config.go`，在 Config struct 中添加：

```go
type Config struct {
	ServerPort     string
	DatabaseURL    string
	RedisAddr      string
	RedisPassword  string
	JWTSecret      string
	JWTExpireHours int
	TemporalAddr   string // 新增
}

func Load() *Config {
	return &Config{
		ServerPort:     getEnv("SERVER_PORT", "8080"),
		DatabaseURL:    getEnv("DATABASE_URL", "postgres://forge:forge_dev_2026@localhost:5432/forge_main?sslmode=disable"),
		RedisAddr:      getEnv("REDIS_ADDR", "localhost:6379"),
		RedisPassword:  getEnv("REDIS_PASSWORD", "forge_redis_2026"),
		JWTSecret:      getEnv("JWT_SECRET", "forge-dev-secret-key-change-in-production"),
		JWTExpireHours: 8,
		TemporalAddr:   getEnv("TEMPORAL_ADDR", "localhost:7233"), // 新增
	}
}
```

- [ ] **Step 7: 更新 main.go 初始化 Temporal + 注入 WorkflowStarter**

修改 `forge-core/cmd/forge-core/main.go`，在 database/redis 连接之后添加 Temporal 初始化，并将 Temporal client 注入到 task service：

```go
package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/shulex/forge/forge-core/internal/config"
	"github.com/shulex/forge/forge-core/internal/middleware"
	"github.com/shulex/forge/forge-core/internal/module/auth"
	"github.com/shulex/forge/forge-core/internal/module/task"
	"github.com/shulex/forge/forge-core/internal/pkg/database"
	forgeRedis "github.com/shulex/forge/forge-core/internal/pkg/redis"
	"github.com/shulex/forge/forge-core/internal/router"
	forgeTemporal "github.com/shulex/forge/forge-core/internal/temporal"
)

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo})))

	cfg := config.Load()
	ctx := context.Background()

	// Database
	db, err := database.NewPool(ctx, cfg.DatabaseURL)
	if err != nil {
		slog.Error("failed to connect database", "error", err)
		os.Exit(1)
	}
	defer db.Close()

	// Redis
	rdb, err := forgeRedis.NewClient(ctx, cfg.RedisAddr, cfg.RedisPassword)
	if err != nil {
		slog.Error("failed to connect redis", "error", err)
		os.Exit(1)
	}
	defer rdb.Close()

	// Migrations
	if err := database.RunMigrations(ctx, db, "migrations"); err != nil {
		slog.Error("failed to run migrations", "error", err)
		os.Exit(1)
	}

	// Temporal
	tc, err := forgeTemporal.NewClient(ctx, cfg.TemporalAddr)
	if err != nil {
		slog.Error("failed to connect temporal", "error", err)
		os.Exit(1)
	}
	defer tc.Close()

	// Start embedded Temporal worker
	tw, err := forgeTemporal.StartWorker(tc.Inner(), db)
	if err != nil {
		slog.Error("failed to start temporal worker", "error", err)
		os.Exit(1)
	}
	defer tw.Stop()

	// Auth module
	authRepo := auth.NewRepository(db)
	authService := auth.NewService(authRepo, cfg.JWTSecret, cfg.JWTExpireHours)
	authHandler := auth.NewHandler(authService)

	// Task module — inject Temporal client as WorkflowStarter
	taskRepo := task.NewRepository(db)
	taskService := task.NewService(taskRepo, tc)
	taskHandler := task.NewHandler(taskService)

	// SSE handler (Task 5)
	taskSSE := task.NewSSEHandler(taskRepo)

	// Router
	r := router.Setup(&router.Deps{
		AuthHandler: authHandler,
		AuthService: authService,
		TaskHandler: taskHandler,
		TaskSSE:     taskSSE,
	})

	// Graceful shutdown
	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		<-sigCh
		slog.Info("shutting down...")
		tw.Stop()
		tc.Close()
		db.Close()
		rdb.Close()
		os.Exit(0)
	}()

	slog.Info("forge-core starting", "port", cfg.ServerPort)
	if err := r.Run(":" + cfg.ServerPort); err != nil {
		slog.Error("server failed", "error", err)
		os.Exit(1)
	}
}
```

> **注意**: 此 main.go 是完整版，包含 Task 5 的 SSEHandler。实现时如果 Task 5 还没做，先用 `TaskSSE: nil` 占位。

- [ ] **Step 8: 安装依赖并验证编译**

```bash
cd forge-core
go mod tidy
go build ./cmd/forge-core
```

- [ ] **Step 9: 启动并测试 Workflow**

确保 Docker Compose（含 Temporal）已启动：

```bash
docker compose -f docker-compose.dev.yml up -d
cd forge-core && go run ./cmd/forge-core
```

**测试完整流程**：
```bash
# 登录
TOKEN=$(curl -s -X POST http://localhost:8080/api/auth/login \
  -H "Content-Type: application/json" \
  -d '{"username":"admin","password":"admin123"}' | jq -r '.data.token')

# 创建任务（触发 Workflow）
curl -X POST http://localhost:8080/api/projects/1/tasks \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  -d '{"requirement":"实现用户注册功能，支持手机号和邮箱注册"}'
# 预期: task 有 workflow_id 和 workflow_run_id

# 等待 ~13 秒后查看任务状态
sleep 15
curl http://localhost:8080/api/projects/1/tasks/1 \
  -H "Authorization: Bearer $TOKEN"
# 预期: status = "COMPLETED", 所有 steps 的 status = "COMPLETED", duration_ms > 0

# 查看 Temporal UI
# 浏览器打开 http://localhost:8233
# 应该能看到 task-1 workflow 和完整的执行历史
```

- [ ] **Step 10: Commit**

```bash
git add forge-core/internal/temporal/ forge-core/internal/config/config.go forge-core/cmd/forge-core/main.go forge-core/go.mod forge-core/go.sum
git commit -m "feat: integrate Temporal with skeleton task workflow and embedded worker"
```

---

## Task 5: SSE 实时推送端点

**Files:**
- Create: `forge-core/internal/module/task/sse.go`
- Modify: `forge-core/internal/router/router.go` (如果 Task 3 未预注册)

- [ ] **Step 1: 创建 sse.go — SSE handler**

`forge-core/internal/module/task/sse.go`：

SSE 端点通过轮询数据库获取任务状态变化，每秒推送一次。客户端在任务到达终态时断开连接。

```go
package task

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
)

type SSEHandler struct {
	repo *Repository
}

func NewSSEHandler(repo *Repository) *SSEHandler {
	return &SSEHandler{repo: repo}
}

// Stream handles GET /api/stream/tasks/:taskId
// Sends SSE events as the task progresses through steps.
func (h *SSEHandler) Stream(c *gin.Context) {
	taskID, err := strconv.ParseInt(c.Param("taskId"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid task ID"})
		return
	}

	// Set SSE headers
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no") // Disable nginx buffering

	// Track the last known status to only send changes
	lastStatus := ""
	lastStepStatuses := map[string]string{}

	// Flush helper
	flusher, ok := c.Writer.(http.Flusher)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "streaming not supported"})
		return
	}

	ctx := c.Request.Context()

	// Send initial state
	h.sendFullState(c, flusher, taskID)

	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			slog.Info("SSE client disconnected", "task_id", taskID)
			return
		case <-ticker.C:
			// Fetch current task state
			t, err := h.repo.FindByID(ctx, taskID)
			if err != nil {
				slog.Error("SSE: failed to find task", "task_id", taskID, "error", err)
				return
			}

			steps, err := h.repo.GetStepsByTaskID(ctx, taskID)
			if err != nil {
				slog.Error("SSE: failed to get steps", "task_id", taskID, "error", err)
				return
			}

			// Check if anything changed
			changed := t.Status != lastStatus
			if !changed {
				for _, s := range steps {
					if prev, ok := lastStepStatuses[s.StepType]; !ok || prev != s.Status {
						changed = true
						break
					}
				}
			}

			if !changed {
				// Send heartbeat to keep connection alive
				fmt.Fprintf(c.Writer, ": heartbeat\n\n")
				flusher.Flush()
				continue
			}

			// Send update
			lastStatus = t.Status
			for _, s := range steps {
				lastStepStatuses[s.StepType] = s.Status
			}

			// Calculate progress percentage
			completedSteps := 0
			runningStep := ""
			runningStepName := ""
			for _, s := range steps {
				if s.Status == StepCompleted {
					completedSteps++
				}
				if s.Status == StepRunning {
					runningStep = s.StepType
					runningStepName = s.Name
				}
			}
			totalSteps := len(steps)
			progress := 0
			if totalSteps > 0 {
				progress = (completedSteps * 100) / totalSteps
			}

			event := TaskProgressEvent{
				Type:     "TASK_PROGRESS",
				TaskID:   taskID,
				Status:   t.Status,
				StepType: runningStep,
				StepName: runningStepName,
				Progress: progress,
			}

			data, _ := json.Marshal(event)
			fmt.Fprintf(c.Writer, "event: TASK_PROGRESS\ndata: %s\n\n", string(data))

			// Also send full step details
			stepsData, _ := json.Marshal(map[string]interface{}{
				"type":  "STEPS_UPDATE",
				"steps": steps,
			})
			fmt.Fprintf(c.Writer, "event: STEPS_UPDATE\ndata: %s\n\n", string(stepsData))

			flusher.Flush()

			// Close connection when task reaches terminal state
			if t.Status == StatusCompleted || t.Status == StatusFailed {
				// Send final event
				finalEvent := map[string]interface{}{
					"type":    "TASK_COMPLETE",
					"task_id": taskID,
					"status":  t.Status,
				}
				finalData, _ := json.Marshal(finalEvent)
				fmt.Fprintf(c.Writer, "event: TASK_COMPLETE\ndata: %s\n\n", string(finalData))
				flusher.Flush()
				return
			}
		}
	}
}

// sendFullState sends the complete current state when client first connects
func (h *SSEHandler) sendFullState(c *gin.Context, flusher http.Flusher, taskID int64) {
	t, err := h.repo.FindByID(c.Request.Context(), taskID)
	if err != nil {
		return
	}

	steps, err := h.repo.GetStepsByTaskID(c.Request.Context(), taskID)
	if err != nil {
		return
	}

	fullState := map[string]interface{}{
		"type":  "FULL_STATE",
		"task":  t,
		"steps": steps,
	}
	data, _ := json.Marshal(fullState)
	fmt.Fprintf(c.Writer, "event: FULL_STATE\ndata: %s\n\n", string(data))
	flusher.Flush()
}
```

- [ ] **Step 2: 验证 SSE 端点**

```bash
cd forge-core && go run ./cmd/forge-core
```

**测试 SSE**（需要两个终端）：

终端 1 — 监听 SSE：
```bash
TOKEN=$(curl -s -X POST http://localhost:8080/api/auth/login \
  -H "Content-Type: application/json" \
  -d '{"username":"admin","password":"admin123"}' | jq -r '.data.token')

# 先创建任务
TASK_ID=$(curl -s -X POST http://localhost:8080/api/projects/1/tasks \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  -d '{"requirement":"测试SSE实时推送"}' | jq -r '.data.task.id')

# 连接 SSE（会看到实时事件流）
curl -N -H "Authorization: Bearer $TOKEN" \
  -H "Accept: text/event-stream" \
  http://localhost:8080/api/stream/tasks/$TASK_ID
# 预期: 每秒收到 TASK_PROGRESS 和 STEPS_UPDATE 事件
# 约 13 秒后收到 TASK_COMPLETE 事件，连接自动关闭
```

- [ ] **Step 3: Commit**

```bash
git add forge-core/internal/module/task/sse.go
git commit -m "feat: add SSE endpoint for real-time task progress streaming"
```

---

## Task 6: 前端 — Task Kanban 看板 + 创建任务对话框

**Files:**
- Create: `forge-portal/lib/tasks.ts`
- Create: `forge-portal/components/tasks/kanban-board.tsx`
- Create: `forge-portal/components/tasks/task-card.tsx`
- Create: `forge-portal/components/tasks/create-task-dialog.tsx`
- Create: `forge-portal/app/(dashboard)/projects/[id]/tasks/page.tsx`
- 按需安装 shadcn/ui 组件: `dialog`, `textarea`, `badge`, `scroll-area`

- [ ] **Step 1: 安装 shadcn/ui 组件**

```bash
cd forge-portal
npx shadcn@latest add dialog textarea badge scroll-area
```

- [ ] **Step 2: 创建 tasks.ts — API 调用封装**

`forge-portal/lib/tasks.ts`：

```typescript
import { api } from "./api";

export interface Task {
  id: number;
  tenant_id: number;
  project_id: number;
  title?: string;
  requirement: string;
  source: string;
  status: string;
  workflow_id?: string;
  workflow_run_id?: string;
  risk_level?: string;
  risk_score?: number;
  branch_name?: string;
  files_changed?: number;
  lines_added?: number;
  lines_deleted?: number;
  created_by: number;
  created_at: string;
  updated_at: string;
  completed_at?: string;
}

export interface TaskStep {
  id: number;
  task_id: number;
  name: string;
  step_type: string;
  status: string;
  input?: string;
  output?: string;
  error?: string;
  attempt: number;
  max_attempts: number;
  started_at?: string;
  completed_at?: string;
  duration_ms?: number;
}

export interface TaskDetail {
  task: Task;
  steps: TaskStep[];
}

export interface TaskListResult {
  tasks: Task[];
  total: number;
}

export const TASK_STATUSES = [
  "SUBMITTED",
  "ANALYZING",
  "PLANNING",
  "GENERATING",
  "REVIEWING",
  "TESTING",
  "DEPLOYING",
  "COMPLETED",
  "FAILED",
] as const;

export type TaskStatus = (typeof TASK_STATUSES)[number];

export const STATUS_LABELS: Record<string, string> = {
  SUBMITTED: "已提交",
  ANALYZING: "分析中",
  PLANNING: "规划中",
  GENERATING: "生成中",
  REVIEWING: "审查中",
  TESTING: "测试中",
  DEPLOYING: "部署中",
  COMPLETED: "已完成",
  FAILED: "失败",
};

export const STATUS_COLORS: Record<string, string> = {
  SUBMITTED: "#8888A0",
  ANALYZING: "#3B82F6",
  PLANNING: "#3B82F6",
  GENERATING: "#3B82F6",
  REVIEWING: "#3B82F6",
  TESTING: "#3B82F6",
  DEPLOYING: "#3B82F6",
  COMPLETED: "#10B981",
  FAILED: "#EF4444",
};

// Kanban columns — group statuses into columns
export const KANBAN_COLUMNS = [
  { key: "SUBMITTED", label: "已提交", statuses: ["SUBMITTED"] },
  { key: "IN_PROGRESS", label: "进行中", statuses: ["ANALYZING", "PLANNING", "GENERATING", "REVIEWING", "TESTING", "DEPLOYING"] },
  { key: "COMPLETED", label: "已完成", statuses: ["COMPLETED"] },
  { key: "FAILED", label: "失败", statuses: ["FAILED"] },
];

export async function createTask(projectId: number, requirement: string, title?: string): Promise<TaskDetail> {
  return api.post<TaskDetail>(`/projects/${projectId}/tasks`, { requirement, title });
}

export async function listTasks(projectId: number, status?: string, page = 1, pageSize = 50): Promise<TaskListResult> {
  const params = new URLSearchParams();
  if (status) params.set("status", status);
  params.set("page", String(page));
  params.set("page_size", String(pageSize));
  return api.get<TaskListResult>(`/projects/${projectId}/tasks?${params.toString()}`);
}

export async function getTask(projectId: number, taskId: number): Promise<TaskDetail> {
  return api.get<TaskDetail>(`/projects/${projectId}/tasks/${taskId}`);
}
```

- [ ] **Step 3: 创建 task-card.tsx — 任务卡片组件**

`forge-portal/components/tasks/task-card.tsx`：

```tsx
"use client";

import { Task, STATUS_LABELS, STATUS_COLORS } from "@/lib/tasks";
import { Badge } from "@/components/ui/badge";
import { Clock, GitBranch } from "lucide-react";
import Link from "next/link";

interface TaskCardProps {
  task: Task;
  projectId: number;
}

export function TaskCard({ task, projectId }: TaskCardProps) {
  const statusColor = STATUS_COLORS[task.status] || "#8888A0";
  const statusLabel = STATUS_LABELS[task.status] || task.status;

  const timeAgo = getTimeAgo(task.created_at);

  return (
    <Link href={`/projects/${projectId}/tasks/${task.id}`}>
      <div
        className="group relative rounded-lg border border-[var(--border)] bg-[var(--surface-1)] p-3
                   hover:border-[var(--border-glow)] hover:bg-[var(--surface-2)]
                   transition-all cursor-pointer"
        style={{ borderLeftWidth: "2px", borderLeftColor: statusColor }}
      >
        {/* Title */}
        <h4 className="text-sm font-medium text-[var(--text-primary)] line-clamp-1 mb-1">
          {task.title || "Untitled Task"}
        </h4>

        {/* Requirement snippet */}
        <p className="text-xs text-[var(--text-secondary)] line-clamp-2 mb-2">
          {task.requirement}
        </p>

        {/* Footer */}
        <div className="flex items-center justify-between">
          <Badge
            variant="outline"
            className="text-[10px] px-1.5 py-0"
            style={{ color: statusColor, borderColor: statusColor }}
          >
            {statusLabel}
          </Badge>

          <div className="flex items-center gap-2 text-[10px] text-[var(--text-muted)]">
            {task.branch_name && (
              <span className="flex items-center gap-0.5">
                <GitBranch className="w-3 h-3" />
                {task.branch_name}
              </span>
            )}
            <span className="flex items-center gap-0.5">
              <Clock className="w-3 h-3" />
              {timeAgo}
            </span>
          </div>
        </div>
      </div>
    </Link>
  );
}

function getTimeAgo(dateStr: string): string {
  const now = new Date();
  const date = new Date(dateStr);
  const diff = Math.floor((now.getTime() - date.getTime()) / 1000);

  if (diff < 60) return "刚刚";
  if (diff < 3600) return `${Math.floor(diff / 60)}分钟前`;
  if (diff < 86400) return `${Math.floor(diff / 3600)}小时前`;
  return `${Math.floor(diff / 86400)}天前`;
}
```

- [ ] **Step 4: 创建 create-task-dialog.tsx — 创建任务对话框**

`forge-portal/components/tasks/create-task-dialog.tsx`：

```tsx
"use client";

import { useState } from "react";
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from "@/components/ui/dialog";
import { Textarea } from "@/components/ui/textarea";
import { Plus, Loader2 } from "lucide-react";
import { createTask } from "@/lib/tasks";

interface CreateTaskDialogProps {
  projectId: number;
  onCreated: () => void;
}

export function CreateTaskDialog({ projectId, onCreated }: CreateTaskDialogProps) {
  const [open, setOpen] = useState(false);
  const [requirement, setRequirement] = useState("");
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState("");

  const handleSubmit = async () => {
    if (!requirement.trim()) {
      setError("请输入需求描述");
      return;
    }

    setLoading(true);
    setError("");

    try {
      await createTask(projectId, requirement.trim());
      setRequirement("");
      setOpen(false);
      onCreated();
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : "创建任务失败");
    } finally {
      setLoading(false);
    }
  };

  return (
    <Dialog open={open} onOpenChange={setOpen}>
      <DialogTrigger asChild>
        <button
          className="flex items-center gap-2 rounded-lg bg-[var(--primary)] px-4 py-2 text-sm font-medium
                     text-white hover:bg-[var(--primary-hover)] transition-colors"
        >
          <Plus className="w-4 h-4" />
          新建任务
        </button>
      </DialogTrigger>
      <DialogContent className="bg-[var(--surface-1)] border-[var(--border)] max-w-lg">
        <DialogHeader>
          <DialogTitle className="text-[var(--text-primary)]">新建任务</DialogTitle>
        </DialogHeader>
        <div className="space-y-4 mt-2">
          <div>
            <label className="text-sm text-[var(--text-secondary)] mb-1 block">
              需求描述
            </label>
            <Textarea
              placeholder="用自然语言描述你的需求，例如：实现用户注册功能，支持手机号和邮箱注册，需要发送验证码..."
              value={requirement}
              onChange={(e) => setRequirement(e.target.value)}
              className="min-h-[120px] bg-[var(--input-bg)] border-[var(--border)] text-[var(--text-primary)]
                         placeholder:text-[var(--text-muted)] resize-none focus:border-[var(--primary)]"
              disabled={loading}
            />
          </div>

          {error && (
            <p className="text-sm text-[var(--error)]">{error}</p>
          )}

          <div className="flex justify-end gap-2">
            <button
              onClick={() => setOpen(false)}
              className="rounded-lg px-4 py-2 text-sm text-[var(--text-secondary)]
                         hover:text-[var(--text-primary)] hover:bg-[var(--surface-2)] transition-colors"
              disabled={loading}
            >
              取消
            </button>
            <button
              onClick={handleSubmit}
              disabled={loading || !requirement.trim()}
              className="flex items-center gap-2 rounded-lg bg-[var(--primary)] px-4 py-2 text-sm font-medium
                         text-white hover:bg-[var(--primary-hover)] transition-colors
                         disabled:opacity-50 disabled:cursor-not-allowed"
            >
              {loading && <Loader2 className="w-4 h-4 animate-spin" />}
              {loading ? "创建中..." : "创建任务"}
            </button>
          </div>
        </div>
      </DialogContent>
    </Dialog>
  );
}
```

- [ ] **Step 5: 创建 kanban-board.tsx — Kanban 看板组件**

`forge-portal/components/tasks/kanban-board.tsx`：

```tsx
"use client";

import { Task, KANBAN_COLUMNS, STATUS_COLORS } from "@/lib/tasks";
import { TaskCard } from "./task-card";
import { ScrollArea } from "@/components/ui/scroll-area";

interface KanbanBoardProps {
  tasks: Task[];
  projectId: number;
}

export function KanbanBoard({ tasks, projectId }: KanbanBoardProps) {
  return (
    <div className="flex gap-4 h-[calc(100vh-200px)] overflow-x-auto pb-4">
      {KANBAN_COLUMNS.map((column) => {
        const columnTasks = tasks.filter((t) =>
          column.statuses.includes(t.status)
        );
        const accentColor = STATUS_COLORS[column.statuses[0]] || "#8888A0";

        return (
          <div
            key={column.key}
            className="flex-shrink-0 w-[300px] flex flex-col rounded-lg border border-[var(--border)] bg-[var(--background)]"
          >
            {/* Column header */}
            <div className="flex items-center gap-2 px-3 py-2 border-b border-[var(--border)]">
              <div
                className="w-2 h-2 rounded-full"
                style={{ backgroundColor: accentColor }}
              />
              <span className="text-sm font-medium text-[var(--text-primary)]">
                {column.label}
              </span>
              <span className="text-xs text-[var(--text-muted)] ml-auto">
                {columnTasks.length}
              </span>
            </div>

            {/* Column body */}
            <ScrollArea className="flex-1 p-2">
              <div className="space-y-2">
                {columnTasks.length === 0 ? (
                  <p className="text-xs text-[var(--text-muted)] text-center py-8">
                    暂无任务
                  </p>
                ) : (
                  columnTasks.map((task) => (
                    <TaskCard key={task.id} task={task} projectId={projectId} />
                  ))
                )}
              </div>
            </ScrollArea>
          </div>
        );
      })}
    </div>
  );
}
```

- [ ] **Step 6: 创建 tasks page.tsx — Kanban 页面**

`forge-portal/app/(dashboard)/projects/[id]/tasks/page.tsx`：

```tsx
"use client";

import { useEffect, useState, useCallback } from "react";
import { useParams } from "next/navigation";
import { listTasks, Task } from "@/lib/tasks";
import { KanbanBoard } from "@/components/tasks/kanban-board";
import { CreateTaskDialog } from "@/components/tasks/create-task-dialog";
import { Loader2, LayoutGrid } from "lucide-react";

export default function TasksPage() {
  const params = useParams();
  const projectId = Number(params.id);

  const [tasks, setTasks] = useState<Task[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");

  const fetchTasks = useCallback(async () => {
    try {
      const result = await listTasks(projectId);
      setTasks(result.tasks);
      setError("");
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : "加载失败");
    } finally {
      setLoading(false);
    }
  }, [projectId]);

  useEffect(() => {
    fetchTasks();

    // Auto-refresh every 3 seconds to pick up status changes
    const interval = setInterval(fetchTasks, 3000);
    return () => clearInterval(interval);
  }, [fetchTasks]);

  if (loading) {
    return (
      <div className="flex items-center justify-center h-64">
        <Loader2 className="w-8 h-8 animate-spin text-[var(--primary)]" />
      </div>
    );
  }

  return (
    <div className="p-6">
      {/* Header */}
      <div className="flex items-center justify-between mb-6">
        <div className="flex items-center gap-3">
          <LayoutGrid className="w-5 h-5 text-[var(--primary)]" />
          <h1 className="text-xl font-semibold text-[var(--text-primary)]">任务看板</h1>
          <span className="text-sm text-[var(--text-muted)]">
            {tasks.length} 个任务
          </span>
        </div>
        <CreateTaskDialog projectId={projectId} onCreated={fetchTasks} />
      </div>

      {error && (
        <div className="mb-4 rounded-lg border border-[var(--error)] bg-[var(--error)]/10 p-3 text-sm text-[var(--error)]">
          {error}
        </div>
      )}

      {/* Kanban */}
      <KanbanBoard tasks={tasks} projectId={projectId} />
    </div>
  );
}
```

- [ ] **Step 7: 验证 Kanban 页面**

```bash
cd forge-portal && npm run dev
```

**验证**：
1. 浏览器打开 `http://localhost:3000/projects/1/tasks`
2. 应该看到四列 Kanban 看板（已提交、进行中、已完成、失败），都显示 "暂无任务"
3. 点击 "新建任务"，输入需求，点击 "创建任务"
4. 任务卡片出现在 "已提交" 列
5. 几秒后自动流转到 "进行中" 列（因为 Workflow 在执行）
6. 约 13 秒后任务出现在 "已完成" 列

- [ ] **Step 8: Commit**

```bash
git add forge-portal/lib/tasks.ts forge-portal/components/tasks/ forge-portal/app/\(dashboard\)/projects/\[id\]/tasks/
git commit -m "feat: add task kanban board with create dialog and auto-refresh"
```

---

## Task 7: 前端 — 任务详情页 + 步骤时间线

**Files:**
- Create: `forge-portal/components/tasks/task-detail.tsx`
- Create: `forge-portal/components/tasks/step-timeline.tsx`
- Create: `forge-portal/app/(dashboard)/projects/[id]/tasks/[taskId]/page.tsx`

- [ ] **Step 1: 创建 step-timeline.tsx — 步骤时间线组件**

`forge-portal/components/tasks/step-timeline.tsx`：

```tsx
"use client";

import { TaskStep } from "@/lib/tasks";
import { CheckCircle2, Circle, Loader2, XCircle, MinusCircle } from "lucide-react";

interface StepTimelineProps {
  steps: TaskStep[];
}

const stepIcons: Record<string, React.ReactNode> = {
  PENDING: <Circle className="w-5 h-5 text-[var(--text-muted)]" />,
  RUNNING: <Loader2 className="w-5 h-5 text-[var(--info)] animate-spin" />,
  COMPLETED: <CheckCircle2 className="w-5 h-5 text-[var(--success)]" />,
  FAILED: <XCircle className="w-5 h-5 text-[var(--error)]" />,
  SKIPPED: <MinusCircle className="w-5 h-5 text-[var(--text-muted)]" />,
};

const stepColors: Record<string, string> = {
  PENDING: "var(--text-muted)",
  RUNNING: "var(--info)",
  COMPLETED: "var(--success)",
  FAILED: "var(--error)",
  SKIPPED: "var(--text-muted)",
};

export function StepTimeline({ steps }: StepTimelineProps) {
  return (
    <div className="relative">
      {steps.map((step, index) => {
        const isLast = index === steps.length - 1;
        const lineColor = step.status === "COMPLETED" ? "var(--success)" : "var(--border)";

        return (
          <div key={step.id} className="flex gap-3">
            {/* Icon + connecting line */}
            <div className="flex flex-col items-center">
              <div className="flex-shrink-0 z-10">
                {stepIcons[step.status] || stepIcons.PENDING}
              </div>
              {!isLast && (
                <div
                  className="w-0.5 flex-1 min-h-[32px]"
                  style={{ backgroundColor: lineColor }}
                />
              )}
            </div>

            {/* Content */}
            <div className="pb-6 flex-1">
              <div className="flex items-center justify-between">
                <h4
                  className="text-sm font-medium"
                  style={{ color: stepColors[step.status] || "var(--text-primary)" }}
                >
                  {step.name}
                </h4>
                {step.duration_ms != null && step.duration_ms > 0 && (
                  <span className="text-xs text-[var(--text-muted)]">
                    {formatDuration(step.duration_ms)}
                  </span>
                )}
              </div>

              {/* Status text */}
              <p className="text-xs text-[var(--text-muted)] mt-0.5">
                {getStatusText(step)}
              </p>

              {/* Running pulse animation */}
              {step.status === "RUNNING" && (
                <div className="mt-2 h-1 rounded-full bg-[var(--surface-2)] overflow-hidden">
                  <div className="h-full w-1/3 rounded-full bg-[var(--info)] animate-pulse" />
                </div>
              )}

              {/* Error display */}
              {step.status === "FAILED" && step.error && (
                <div className="mt-2 rounded border border-[var(--error)]/30 bg-[var(--error)]/5 p-2 text-xs text-[var(--error)]">
                  {step.error}
                </div>
              )}
            </div>
          </div>
        );
      })}
    </div>
  );
}

function formatDuration(ms: number): string {
  if (ms < 1000) return `${ms}ms`;
  const seconds = Math.floor(ms / 1000);
  if (seconds < 60) return `${seconds}s`;
  const minutes = Math.floor(seconds / 60);
  const remainingSeconds = seconds % 60;
  return `${minutes}m ${remainingSeconds}s`;
}

function getStatusText(step: TaskStep): string {
  switch (step.status) {
    case "PENDING":
      return "等待执行";
    case "RUNNING":
      return step.started_at
        ? `执行中 — 开始于 ${new Date(step.started_at).toLocaleTimeString()}`
        : "执行中";
    case "COMPLETED":
      return step.completed_at
        ? `完成于 ${new Date(step.completed_at).toLocaleTimeString()}`
        : "已完成";
    case "FAILED":
      return `第 ${step.attempt}/${step.max_attempts} 次尝试失败`;
    case "SKIPPED":
      return "已跳过";
    default:
      return "";
  }
}
```

- [ ] **Step 2: 创建 task-detail.tsx — 任务详情组件**

`forge-portal/components/tasks/task-detail.tsx`：

```tsx
"use client";

import { TaskDetail as TaskDetailType, STATUS_LABELS, STATUS_COLORS } from "@/lib/tasks";
import { Badge } from "@/components/ui/badge";
import { Clock, GitBranch, FileCode, ArrowLeft } from "lucide-react";
import Link from "next/link";

interface TaskDetailProps {
  detail: TaskDetailType;
  projectId: number;
}

export function TaskDetailView({ detail, projectId }: TaskDetailProps) {
  const { task } = detail;
  const statusColor = STATUS_COLORS[task.status] || "#8888A0";
  const statusLabel = STATUS_LABELS[task.status] || task.status;

  return (
    <div className="space-y-6">
      {/* Back link */}
      <Link
        href={`/projects/${projectId}/tasks`}
        className="inline-flex items-center gap-1 text-sm text-[var(--text-secondary)] hover:text-[var(--text-primary)] transition-colors"
      >
        <ArrowLeft className="w-4 h-4" />
        返回看板
      </Link>

      {/* Task header */}
      <div className="rounded-lg border border-[var(--border)] bg-[var(--surface-1)] p-6">
        <div className="flex items-start justify-between mb-4">
          <div>
            <h1 className="text-lg font-semibold text-[var(--text-primary)] mb-1">
              {task.title || "Untitled Task"}
            </h1>
            <p className="text-sm text-[var(--text-muted)]">
              Task #{task.id}
            </p>
          </div>
          <Badge
            variant="outline"
            className="text-sm px-3 py-1"
            style={{ color: statusColor, borderColor: statusColor }}
          >
            {statusLabel}
          </Badge>
        </div>

        {/* Requirement */}
        <div className="mb-4">
          <h3 className="text-sm font-medium text-[var(--text-secondary)] mb-1">需求描述</h3>
          <p className="text-sm text-[var(--text-primary)] whitespace-pre-wrap leading-relaxed">
            {task.requirement}
          </p>
        </div>

        {/* Metadata grid */}
        <div className="grid grid-cols-2 md:grid-cols-4 gap-4 pt-4 border-t border-[var(--border)]">
          <MetaItem
            icon={<Clock className="w-4 h-4" />}
            label="创建时间"
            value={new Date(task.created_at).toLocaleString()}
          />
          {task.completed_at && (
            <MetaItem
              icon={<Clock className="w-4 h-4" />}
              label="完成时间"
              value={new Date(task.completed_at).toLocaleString()}
            />
          )}
          {task.branch_name && (
            <MetaItem
              icon={<GitBranch className="w-4 h-4" />}
              label="分支"
              value={task.branch_name}
            />
          )}
          {task.files_changed != null && (
            <MetaItem
              icon={<FileCode className="w-4 h-4" />}
              label="文件变更"
              value={`${task.files_changed} files (+${task.lines_added || 0} -${task.lines_deleted || 0})`}
            />
          )}
        </div>

        {/* Workflow ID */}
        {task.workflow_id && (
          <div className="mt-4 pt-4 border-t border-[var(--border)]">
            <span className="text-xs text-[var(--text-muted)]">
              Workflow: {task.workflow_id}
            </span>
          </div>
        )}
      </div>
    </div>
  );
}

function MetaItem({ icon, label, value }: { icon: React.ReactNode; label: string; value: string }) {
  return (
    <div className="flex items-start gap-2">
      <span className="text-[var(--text-muted)] mt-0.5">{icon}</span>
      <div>
        <p className="text-xs text-[var(--text-muted)]">{label}</p>
        <p className="text-sm text-[var(--text-primary)]">{value}</p>
      </div>
    </div>
  );
}
```

- [ ] **Step 3: 创建 task detail page.tsx**

`forge-portal/app/(dashboard)/projects/[id]/tasks/[taskId]/page.tsx`：

```tsx
"use client";

import { useEffect, useState, useCallback } from "react";
import { useParams } from "next/navigation";
import { getTask, TaskDetail } from "@/lib/tasks";
import { TaskDetailView } from "@/components/tasks/task-detail";
import { StepTimeline } from "@/components/tasks/step-timeline";
import { Loader2 } from "lucide-react";

export default function TaskDetailPage() {
  const params = useParams();
  const projectId = Number(params.id);
  const taskId = Number(params.taskId);

  const [detail, setDetail] = useState<TaskDetail | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");

  const fetchDetail = useCallback(async () => {
    try {
      const result = await getTask(projectId, taskId);
      setDetail(result);
      setError("");
      return result;
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : "加载失败");
      return null;
    } finally {
      setLoading(false);
    }
  }, [projectId, taskId]);

  useEffect(() => {
    fetchDetail();

    // Poll for updates until task is in terminal state
    const interval = setInterval(async () => {
      const result = await fetchDetail();
      if (result && (result.task.status === "COMPLETED" || result.task.status === "FAILED")) {
        clearInterval(interval);
      }
    }, 1000);

    return () => clearInterval(interval);
  }, [fetchDetail]);

  if (loading) {
    return (
      <div className="flex items-center justify-center h-64">
        <Loader2 className="w-8 h-8 animate-spin text-[var(--primary)]" />
      </div>
    );
  }

  if (error || !detail) {
    return (
      <div className="p-6">
        <div className="rounded-lg border border-[var(--error)] bg-[var(--error)]/10 p-4 text-[var(--error)]">
          {error || "任务不存在"}
        </div>
      </div>
    );
  }

  return (
    <div className="p-6">
      <div className="grid grid-cols-1 lg:grid-cols-3 gap-6">
        {/* Left: Task info */}
        <div className="lg:col-span-2">
          <TaskDetailView detail={detail} projectId={projectId} />
        </div>

        {/* Right: Step timeline */}
        <div>
          <div className="rounded-lg border border-[var(--border)] bg-[var(--surface-1)] p-4">
            <h3 className="text-sm font-medium text-[var(--text-primary)] mb-4">
              执行步骤
            </h3>
            <StepTimeline steps={detail.steps} />
          </div>
        </div>
      </div>
    </div>
  );
}
```

- [ ] **Step 4: 验证任务详情页**

```bash
cd forge-portal && npm run dev
```

**验证**：
1. 先创建一个任务（通过 Kanban 页面或 curl）
2. 在 Kanban 页面点击任务卡片
3. 应该跳转到 `/projects/1/tasks/1`
4. 左侧显示任务信息（标题、需求、状态、时间）
5. 右侧显示步骤时间线，步骤从灰色（等待）→ 蓝色旋转（执行中）→ 绿色对勾（完成）实时更新
6. 正在执行的步骤有脉冲进度条动画
7. 所有步骤完成后，任务状态显示 "已完成"

- [ ] **Step 5: Commit**

```bash
git add forge-portal/components/tasks/task-detail.tsx forge-portal/components/tasks/step-timeline.tsx forge-portal/app/\(dashboard\)/projects/\[id\]/tasks/\[taskId\]/
git commit -m "feat: add task detail page with step timeline and real-time updates"
```

---

## Task 8: 前端 SSE 集成（替换轮询）

**Files:**
- Create: `forge-portal/lib/use-task-stream.ts`
- Modify: `forge-portal/app/(dashboard)/projects/[id]/tasks/[taskId]/page.tsx`

- [ ] **Step 1: 创建 use-task-stream.ts — SSE Hook**

`forge-portal/lib/use-task-stream.ts`：

```typescript
"use client";

import { useEffect, useRef, useCallback, useState } from "react";
import { Task, TaskStep } from "./tasks";

interface TaskStreamState {
  task: Task | null;
  steps: TaskStep[];
  connected: boolean;
  progress: number;
  currentStep: string;
}

interface TaskProgressEvent {
  type: string;
  task_id: number;
  status: string;
  step_type: string;
  step_name: string;
  progress: number;
}

export function useTaskStream(taskId: number | null) {
  const [state, setState] = useState<TaskStreamState>({
    task: null,
    steps: [],
    connected: false,
    progress: 0,
    currentStep: "",
  });
  const eventSourceRef = useRef<EventSource | null>(null);
  const reconnectTimeoutRef = useRef<ReturnType<typeof setTimeout> | null>(null);

  const connect = useCallback(() => {
    if (!taskId) return;

    const token = localStorage.getItem("forge_token");
    if (!token) return;

    // EventSource doesn't support custom headers, so pass token as query param
    // The backend SSE handler also accepts ?token= for EventSource compatibility
    const url = `/api/stream/tasks/${taskId}?token=${encodeURIComponent(token)}`;

    const es = new EventSource(url);
    eventSourceRef.current = es;

    es.onopen = () => {
      setState((prev) => ({ ...prev, connected: true }));
    };

    // Handle FULL_STATE event (initial state on connect)
    es.addEventListener("FULL_STATE", (event) => {
      try {
        const data = JSON.parse(event.data);
        setState((prev) => ({
          ...prev,
          task: data.task,
          steps: data.steps || [],
        }));
      } catch {
        // ignore parse errors
      }
    });

    // Handle TASK_PROGRESS event
    es.addEventListener("TASK_PROGRESS", (event) => {
      try {
        const data: TaskProgressEvent = JSON.parse(event.data);
        setState((prev) => ({
          ...prev,
          progress: data.progress,
          currentStep: data.step_name || data.step_type,
          task: prev.task ? { ...prev.task, status: data.status } : null,
        }));
      } catch {
        // ignore parse errors
      }
    });

    // Handle STEPS_UPDATE event
    es.addEventListener("STEPS_UPDATE", (event) => {
      try {
        const data = JSON.parse(event.data);
        if (data.steps) {
          setState((prev) => ({ ...prev, steps: data.steps }));
        }
      } catch {
        // ignore parse errors
      }
    });

    // Handle TASK_COMPLETE event
    es.addEventListener("TASK_COMPLETE", (event) => {
      try {
        const data = JSON.parse(event.data);
        setState((prev) => ({
          ...prev,
          task: prev.task ? { ...prev.task, status: data.status } : null,
          progress: 100,
          connected: false,
        }));
      } catch {
        // ignore parse errors
      }
      es.close();
    });

    es.onerror = () => {
      es.close();
      setState((prev) => ({ ...prev, connected: false }));

      // Auto-reconnect after 3 seconds
      reconnectTimeoutRef.current = setTimeout(() => {
        connect();
      }, 3000);
    };
  }, [taskId]);

  useEffect(() => {
    connect();

    return () => {
      if (eventSourceRef.current) {
        eventSourceRef.current.close();
      }
      if (reconnectTimeoutRef.current) {
        clearTimeout(reconnectTimeoutRef.current);
      }
    };
  }, [connect]);

  const disconnect = useCallback(() => {
    if (eventSourceRef.current) {
      eventSourceRef.current.close();
      eventSourceRef.current = null;
    }
    if (reconnectTimeoutRef.current) {
      clearTimeout(reconnectTimeoutRef.current);
      reconnectTimeoutRef.current = null;
    }
    setState((prev) => ({ ...prev, connected: false }));
  }, []);

  return { ...state, disconnect };
}
```

- [ ] **Step 2: 更新 SSE handler 支持 query param token**

由于 `EventSource` 不支持自定义 header，需要在 SSE handler 中支持从 query string 读取 token。

修改 `forge-core/internal/module/task/sse.go`，在 `Stream` 方法开头、设置 SSE headers 之前添加 token 提取逻辑：

```go
// In the Stream method, add this at the very top (before SSE headers):

// Support token from query param for EventSource (which can't set headers)
// The JWTAuth middleware handles the normal Authorization header case.
// For SSE, the frontend passes ?token=xxx since EventSource can't set headers.
```

同时修改 `forge-core/internal/router/router.go`，将 SSE 路由移到一个特殊的中间件组，支持从 query param 提取 token：

```go
// 在 router.go 中，SSE 路由单独注册，使用修改版的 auth 中间件

// SSE routes — support token from query param
stream := api.Group("/stream")
stream.Use(middleware.JWTAuthWithQueryToken(deps.AuthService))
{
    if deps.TaskSSE != nil {
        stream.GET("/tasks/:taskId", deps.TaskSSE.Stream)
    }
}
```

在 `forge-core/internal/middleware/auth.go` 中添加新的中间件：

```go
// JWTAuthWithQueryToken is like JWTAuth but also accepts token from ?token= query param.
// This is needed for SSE (EventSource) which cannot set custom headers.
func JWTAuthWithQueryToken(authService *auth.Service) gin.HandlerFunc {
    return func(c *gin.Context) {
        // Try Authorization header first
        tokenString := ""
        header := c.GetHeader("Authorization")
        if header != "" && strings.HasPrefix(header, "Bearer ") {
            tokenString = strings.TrimPrefix(header, "Bearer ")
        }

        // Fallback to query param
        if tokenString == "" {
            tokenString = c.Query("token")
        }

        if tokenString == "" {
            response.Fail(c, http.StatusUnauthorized, "请先登录")
            c.Abort()
            return
        }

        claims, err := authService.ValidateToken(c.Request.Context(), tokenString)
        if err != nil {
            response.Fail(c, http.StatusUnauthorized, "登录已过期，请重新登录")
            c.Abort()
            return
        }

        c.Set("user_id", claims.UserID)
        c.Set("tenant_id", claims.TenantID)
        c.Set("username", claims.Username)
        c.Set("token_jti", claims.ID)
        c.Next()
    }
}
```

- [ ] **Step 3: 更新任务详情页使用 SSE**

修改 `forge-portal/app/(dashboard)/projects/[id]/tasks/[taskId]/page.tsx`，用 SSE Hook 替换轮询：

```tsx
"use client";

import { useEffect, useState, useCallback } from "react";
import { useParams } from "next/navigation";
import { getTask, TaskDetail } from "@/lib/tasks";
import { useTaskStream } from "@/lib/use-task-stream";
import { TaskDetailView } from "@/components/tasks/task-detail";
import { StepTimeline } from "@/components/tasks/step-timeline";
import { Loader2, Wifi, WifiOff } from "lucide-react";

export default function TaskDetailPage() {
  const params = useParams();
  const projectId = Number(params.id);
  const taskId = Number(params.taskId);

  const [detail, setDetail] = useState<TaskDetail | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");

  // SSE stream for real-time updates
  const stream = useTaskStream(taskId);

  // Initial fetch
  const fetchDetail = useCallback(async () => {
    try {
      const result = await getTask(projectId, taskId);
      setDetail(result);
      setError("");
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : "加载失败");
    } finally {
      setLoading(false);
    }
  }, [projectId, taskId]);

  useEffect(() => {
    fetchDetail();
  }, [fetchDetail]);

  // Merge SSE updates into detail state
  const mergedDetail: TaskDetail | null = detail
    ? {
        task: stream.task
          ? { ...detail.task, status: stream.task.status, completed_at: stream.task.completed_at }
          : detail.task,
        steps: stream.steps.length > 0 ? stream.steps : detail.steps,
      }
    : null;

  if (loading) {
    return (
      <div className="flex items-center justify-center h-64">
        <Loader2 className="w-8 h-8 animate-spin text-[var(--primary)]" />
      </div>
    );
  }

  if (error || !mergedDetail) {
    return (
      <div className="p-6">
        <div className="rounded-lg border border-[var(--error)] bg-[var(--error)]/10 p-4 text-[var(--error)]">
          {error || "任务不存在"}
        </div>
      </div>
    );
  }

  return (
    <div className="p-6">
      {/* SSE connection indicator */}
      <div className="flex items-center gap-1.5 mb-2 text-xs text-[var(--text-muted)]">
        {stream.connected ? (
          <>
            <Wifi className="w-3 h-3 text-[var(--success)]" />
            <span>实时更新中</span>
            {stream.currentStep && (
              <span className="text-[var(--info)]">
                — {stream.currentStep} ({stream.progress}%)
              </span>
            )}
          </>
        ) : (
          <>
            <WifiOff className="w-3 h-3" />
            <span>
              {mergedDetail.task.status === "COMPLETED" || mergedDetail.task.status === "FAILED"
                ? "任务已结束"
                : "连接断开，尝试重连..."}
            </span>
          </>
        )}
      </div>

      <div className="grid grid-cols-1 lg:grid-cols-3 gap-6">
        {/* Left: Task info */}
        <div className="lg:col-span-2">
          <TaskDetailView detail={mergedDetail} projectId={projectId} />
        </div>

        {/* Right: Step timeline */}
        <div>
          <div className="rounded-lg border border-[var(--border)] bg-[var(--surface-1)] p-4">
            <h3 className="text-sm font-medium text-[var(--text-primary)] mb-4">
              执行步骤
            </h3>
            <StepTimeline steps={mergedDetail.steps} />
          </div>
        </div>
      </div>
    </div>
  );
}
```

- [ ] **Step 4: 验证 SSE 集成**

```bash
# 确保后端和前端都在运行
cd forge-core && go run ./cmd/forge-core
cd forge-portal && npm run dev
```

**验证**：
1. 创建一个新任务
2. 立即点击任务卡片进入详情页
3. 右上角应显示 "实时更新中" 和绿色 WiFi 图标
4. 步骤时间线实时更新：
   - "需求分析" 显示蓝色旋转图标 + 脉冲进度条
   - 2 秒后变为绿色对勾，"方案规划" 开始旋转
   - 依次类推，每个步骤 2-3 秒完成
5. 所有步骤完成后：
   - 任务状态变为 "已完成"
   - 连接指示器变为 "任务已结束"
   - SSE 连接自动关闭
6. 打开浏览器开发者工具 Network 面板，应该能看到 EventSource 连接和事件流

- [ ] **Step 5: Commit**

```bash
git add forge-portal/lib/use-task-stream.ts forge-portal/app/\(dashboard\)/projects/\[id\]/tasks/\[taskId\]/page.tsx forge-core/internal/middleware/auth.go forge-core/internal/router/router.go
git commit -m "feat: integrate SSE for real-time task progress updates on detail page"
```

---

## 验证清单

完成所有 8 个 Task 后，执行以下端到端验证：

```bash
# 1. 启动基础设施
docker compose -f docker-compose.dev.yml up -d

# 2. 验证 Temporal 健康
curl -s http://localhost:8233 | head -1  # Temporal UI

# 3. 启动后端
cd forge-core && go run ./cmd/forge-core
# 预期日志: database connected, redis connected, temporal connected, temporal worker started

# 4. 启动前端
cd forge-portal && npm run dev

# 5. 浏览器测试
# - 登录: http://localhost:3000/login (admin/admin123)
# - 导航到项目任务页: http://localhost:3000/projects/1/tasks
# - 创建任务 → 看 Kanban 看板自动刷新
# - 点击任务 → 看详情页步骤实时更新
# - Temporal UI: http://localhost:8233 → 查看 workflow 执行历史

# 6. API 测试
TOKEN=$(curl -s -X POST http://localhost:8080/api/auth/login \
  -H "Content-Type: application/json" \
  -d '{"username":"admin","password":"admin123"}' | jq -r '.data.token')

# 创建任务
curl -X POST http://localhost:8080/api/projects/1/tasks \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  -d '{"requirement":"测试端到端流程"}'

# 查看任务列表
curl http://localhost:8080/api/projects/1/tasks \
  -H "Authorization: Bearer $TOKEN"

# SSE 流
curl -N -H "Authorization: Bearer $TOKEN" \
  http://localhost:8080/api/stream/tasks/1
```
