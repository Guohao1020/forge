# S9 — 任务拆分增强 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpents:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 将 AI 的任务拆分从简单有序列表升级为 DAG 依赖图 + 需求追溯 + 工时估算，并在前端可视化展示任务依赖关系和进度。

**Architecture:** 增强 PlannerAgent prompt 输出 DAG 结构（含依赖关系 + 需求映射 + 工时估算），新增 task_nodes DB 表存储拆分结果，PlanOutputCard 升级为可交互的 DAG 图。代码生成阶段暂不按 DAG 并行（保持串行生成），仅在展示层体现依赖关系。

**Tech Stack:** Python 3.12 (PlannerAgent), Go 1.22 + pgx (task_nodes 表), Next.js + React (DAG 可视化)

**Dependencies:** S8 (需求澄清, 已完成)

---

## File Structure

### Python AI Worker
```
ai-worker/src/
├── agents/planner.py              # MODIFY: 增强 prompt，输出 DAG 结构
└── activities/plan.py              # MODIFY: PlanOutput 增加 DAG 字段
```

### Go 后端
```
forge-core/
├── migrations/
│   └── 011_task_nodes.sql          # NEW: task_nodes 表
├── internal/module/task/
│   ├── model.go                    # MODIFY: 添加 TaskNode struct
│   ├── repository.go               # MODIFY: 添加 TaskNode CRUD
│   ├── service.go                  # MODIFY: 保存/查询 task nodes
│   └── handler.go                  # MODIFY: 添加 task nodes API
└── internal/router/router.go       # MODIFY: 注册新路由
```

### 前端
```
forge-portal/
├── components/tasks/
│   ├── plan-output-card.tsx        # MODIFY: 升级为 DAG 可视化
│   └── dag-task-list.tsx           # NEW: DAG 任务列表组件（带依赖线和状态）
└── lib/tasks.ts                    # MODIFY: 添加 TaskNode 类型
```

---

## Task 1: 数据库迁移 — task_nodes 表

**Files:**
- Create: `forge-core/migrations/011_task_nodes.sql`

- [ ] **Step 1: 创建迁移文件**

`forge-core/migrations/011_task_nodes.sql`:

```sql
-- S9: Task decomposition nodes for DAG-based planning
CREATE TABLE IF NOT EXISTS engine.task_nodes (
    id              BIGSERIAL PRIMARY KEY,
    task_id         BIGINT NOT NULL REFERENCES engine.tasks(id),
    node_order      INT NOT NULL,
    title           TEXT NOT NULL,
    description     TEXT,
    node_type       VARCHAR(20) NOT NULL DEFAULT 'BACKEND',  -- BACKEND / FRONTEND / SCHEMA / CONFIG / TEST
    status          VARCHAR(20) NOT NULL DEFAULT 'PENDING',   -- PENDING / READY / RUNNING / COMPLETED / SKIPPED
    depends_on      JSONB NOT NULL DEFAULT '[]',              -- array of node_order integers
    files           JSONB NOT NULL DEFAULT '[]',              -- array of file paths
    estimate_hours  DECIMAL(4,1),
    requirement_ref TEXT,                                     -- traceability: which part of requirement this covers
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_task_nodes_task ON engine.task_nodes(task_id);
CREATE UNIQUE INDEX IF NOT EXISTS idx_task_nodes_order ON engine.task_nodes(task_id, node_order);
```

- [ ] **Step 2: 验证构建**

```bash
cd forge-core && go build ./cmd/forge-core
```

- [ ] **Step 3: Commit**

```bash
git add forge-core/migrations/011_task_nodes.sql
git commit -m "feat(s9): add task_nodes table for DAG-based task decomposition"
```

---

## Task 2: Go 后端 — TaskNode model + repository + API

**Files:**
- Modify: `forge-core/internal/module/task/model.go`
- Modify: `forge-core/internal/module/task/repository.go`
- Modify: `forge-core/internal/module/task/handler.go`
- Modify: `forge-core/internal/router/router.go`

**重要**: 先完整读取 `task/model.go`、`task/repository.go`、`task/handler.go`、`router.go` 了解现有模式。

- [ ] **Step 1: 添加 TaskNode struct 到 model.go**

```go
// TaskNode represents a sub-task in the DAG decomposition
type TaskNode struct {
	ID             int64           `json:"id"`
	TaskID         int64           `json:"taskId"`
	NodeOrder      int             `json:"nodeOrder"`
	Title          string          `json:"title"`
	Description    *string         `json:"description,omitempty"`
	NodeType       string          `json:"nodeType"`
	Status         string          `json:"status"`
	DependsOn      json.RawMessage `json:"dependsOn"`
	Files          json.RawMessage `json:"files"`
	EstimateHours  *float64        `json:"estimateHours,omitempty"`
	RequirementRef *string         `json:"requirementRef,omitempty"`
	CreatedAt      time.Time       `json:"createdAt"`
	UpdatedAt      time.Time       `json:"updatedAt"`
}

type TaskNodeListResponse struct {
	Nodes []TaskNode `json:"nodes"`
}
```

- [ ] **Step 2: 添加 repository 方法**

```go
// CreateNodes — batch insert task nodes from plan output
func (r *Repository) CreateNodes(ctx context.Context, taskID int64, nodes []TaskNode) error

// GetNodesByTaskID — list all nodes for a task
func (r *Repository) GetNodesByTaskID(ctx context.Context, taskID int64) ([]TaskNode, error)

// UpdateNodeStatus — update a single node's status
func (r *Repository) UpdateNodeStatus(ctx context.Context, nodeID int64, status string) error

// DeleteNodesByTaskID — clear nodes (for re-planning)
func (r *Repository) DeleteNodesByTaskID(ctx context.Context, taskID int64) error
```

CreateNodes 使用 batch insert:
```sql
INSERT INTO engine.task_nodes (task_id, node_order, title, description, node_type, status, depends_on, files, estimate_hours, requirement_ref)
VALUES ($1, $2, $3, $4, $5, 'PENDING', $6, $7, $8, $9)
```

GetNodesByTaskID:
```sql
SELECT id, task_id, node_order, title, description, node_type, status, depends_on, files, estimate_hours, requirement_ref, created_at, updated_at
FROM engine.task_nodes WHERE task_id = $1 ORDER BY node_order
```

- [ ] **Step 3: 添加 handler 端点**

```go
// GET /api/projects/:id/tasks/:taskId/nodes — 获取任务的 DAG 节点
func (h *Handler) ListTaskNodes(c *gin.Context)

// POST /api/projects/:id/tasks/:taskId/nodes — 保存 DAG 节点（内部使用）
func (h *Handler) SaveTaskNodes(c *gin.Context)
```

- [ ] **Step 4: 注册路由**

在 `router.go` protected group 中添加：
```go
protected.GET("/projects/:id/tasks/:taskId/nodes", deps.TaskHandler.ListTaskNodes)
```

- [ ] **Step 5: 验证构建**

```bash
cd forge-core && go build ./cmd/forge-core
```

- [ ] **Step 6: Commit**

```bash
git add forge-core/internal/module/task/ forge-core/internal/router/router.go
git commit -m "feat(s9): add TaskNode model, repository, and API for DAG decomposition"
```

---

## Task 3: PlannerAgent 增强 — DAG 输出 + 需求追溯

**Files:**
- Modify: `ai-worker/src/agents/planner.py`
- Modify: `ai-worker/src/activities/plan.py`

- [ ] **Step 1: 增强 PlannerAgent prompt**

读取 `ai-worker/src/agents/planner.py`，重写 PLANNER_SYSTEM_PROMPT:

```python
PLANNER_SYSTEM_PROMPT = """You are a senior software architect. Your task is to decompose a requirement into a DAG (Directed Acyclic Graph) of implementation tasks.

## Rules
1. Each task should be completable by modifying 1-3 files
2. Specify dependencies explicitly — which tasks must complete before this one can start
3. Map each task back to which part of the requirement it addresses
4. Estimate effort in hours (0.5, 1, 2, 4, 8)
5. Identify task type: BACKEND, FRONTEND, SCHEMA, CONFIG, TEST
6. Tasks with no dependencies can run in parallel
7. Never create circular dependencies

## Output Format
IMPORTANT: You MUST respond with ONLY a JSON object. No explanations, no markdown.

{"title": "Feature title", "tasks": [{"order": 1, "title": "Create database migration", "description": "Add users table with id, name, email columns", "type": "SCHEMA", "files": ["migrations/001_users.sql"], "depends_on": [], "estimate_hours": 0.5, "requirement_ref": "用户注册功能"}, {"order": 2, "title": "Implement user service", "description": "CRUD operations for users", "type": "BACKEND", "files": ["service/user.go", "model/user.go"], "depends_on": [1], "estimate_hours": 2, "requirement_ref": "用户注册功能"}], "risk_level": "LOW", "risk_factors": [], "total_estimate_hours": 2.5, "parallel_tracks": 2}
"""
```

关键变化：
- `depends_on: int[]` — 依赖的 task order 数组
- `requirement_ref: string` — 追溯到需求的哪个部分
- `description: string` — 每个任务的详细描述
- `total_estimate_hours: float` — 总工时
- `parallel_tracks: int` — 可并行的轨道数

- [ ] **Step 2: 更新 PlanOutput**

在 `ai-worker/src/activities/plan.py` 中扩展 PlanOutput:

```python
@dataclass
class PlanOutput:
    title: str
    tasks: List[Dict[str, Any]]  # now includes depends_on, requirement_ref, description
    risk_level: str
    risk_factors: List[str]
    total_estimate_hours: float = 0
    parallel_tracks: int = 1
    tokens_used: int = 0
    model: str = ""
    provider: str = ""
    latency_ms: int = 0
```

提取新字段：
```python
return PlanOutput(
    ...
    total_estimate_hours=result.structured.get("total_estimate_hours", 0),
    parallel_tracks=result.structured.get("parallel_tracks", 1),
)
```

- [ ] **Step 3: 验证 imports**

```bash
cd ai-worker && python -c "from src.activities.plan import plan_task_activity; print('OK')"
```

- [ ] **Step 4: Commit**

```bash
git add ai-worker/src/agents/planner.py ai-worker/src/activities/plan.py
git commit -m "feat(s9): enhance PlannerAgent with DAG decomposition and requirement traceability"
```

---

## Task 4: Workflow 集成 — 保存 DAG 节点

**Files:**
- Modify: `forge-core/internal/temporal/activity/task_activities.go`
- Modify: `forge-core/internal/temporal/workflow/task_workflow.go`
- Modify: `forge-core/internal/temporal/worker.go`

- [ ] **Step 1: 添加 SaveTaskNodes activity**

在 `task_activities.go` 中添加：

```go
// SaveTaskNodes persists the DAG nodes from plan output
func (a *TaskActivities) SaveTaskNodes(ctx context.Context, taskID int64, nodes []map[string]interface{}) error {
    // Delete existing nodes (re-planning case)
    _, _ = a.db.Exec(ctx, `DELETE FROM engine.task_nodes WHERE task_id = $1`, taskID)

    for _, n := range nodes {
        order, _ := n["order"].(float64)
        title, _ := n["title"].(string)
        desc, _ := n["description"].(string)
        nodeType, _ := n["type"].(string)
        if nodeType == "" { nodeType = "BACKEND" }

        depsJSON, _ := json.Marshal(n["depends_on"])
        filesJSON, _ := json.Marshal(n["files"])
        estHours, _ := n["estimate_hours"].(float64)
        reqRef, _ := n["requirement_ref"].(string)

        _, err := a.db.Exec(ctx,
            `INSERT INTO engine.task_nodes (task_id, node_order, title, description, node_type, depends_on, files, estimate_hours, requirement_ref)
             VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`,
            taskID, int(order), title, desc, nodeType, string(depsJSON), string(filesJSON), estHours, reqRef,
        )
        if err != nil {
            slog.Warn("failed to save task node", "task_id", taskID, "order", order, "error", err)
        }
    }
    slog.Info("task nodes saved", "task_id", taskID, "count", len(nodes))
    return nil
}
```

- [ ] **Step 2: 在 workflow 中调用 SaveTaskNodes**

在 `task_workflow.go` 的 PLAN 步骤完成后（SaveStepOutput 之后），添加：

```go
// Save DAG nodes if plan has tasks
if tasks, ok := planResult["tasks"].([]interface{}); ok && len(tasks) > 0 {
    taskNodes := make([]map[string]interface{}, 0, len(tasks))
    for _, t := range tasks {
        if node, ok := t.(map[string]interface{}); ok {
            taskNodes = append(taskNodes, node)
        }
    }
    _ = workflow.ExecuteActivity(localCtx, "SaveTaskNodes", input.TaskID, taskNodes).Get(ctx, nil)
}
```

- [ ] **Step 3: 注册 activity 到 worker**

在 `worker.go` 中注册 `SaveTaskNodes`。

- [ ] **Step 4: 验证构建**

```bash
cd forge-core && go build ./cmd/forge-core
```

- [ ] **Step 5: Commit**

```bash
git add forge-core/internal/temporal/
git commit -m "feat(s9): save DAG task nodes from plan output via Temporal workflow"
```

---

## Task 5: 前端 — DAG 任务可视化

**Files:**
- Create: `forge-portal/components/tasks/dag-task-list.tsx`
- Modify: `forge-portal/components/tasks/plan-output-card.tsx`
- Modify: `forge-portal/lib/tasks.ts`

- [ ] **Step 1: 添加 TaskNode 类型到 tasks.ts**

```typescript
export interface TaskNode {
  id: number;
  taskId: number;
  nodeOrder: number;
  title: string;
  description?: string;
  nodeType: string;
  status: string;
  dependsOn: number[];
  files: string[];
  estimateHours?: number;
  requirementRef?: string;
}

export async function getTaskNodes(projectId: number, taskId: number): Promise<TaskNode[]> {
  const res = await api.get<{ nodes: TaskNode[] }>(
    `/projects/${projectId}/tasks/${taskId}/nodes`
  );
  return res.nodes || [];
}
```

- [ ] **Step 2: 创建 DAG 任务列表组件**

`forge-portal/components/tasks/dag-task-list.tsx`:

不用复杂的图形库，用带缩进和依赖线的列表展示：

```
┌─ 1. 创建数据库迁移 ──────── SCHEMA · 0.5h ──── ✅ │
│     └─ 需求映射: "用户注册功能"                      │
│     └─ 文件: migrations/001_users.sql               │
│                                                      │
├─ 2. 实现用户 Service ──── BACKEND · 2h ──── ⬚     │
│     └─ 依赖: [1]                                    │
│     └─ 需求映射: "用户注册功能"                      │
│                                                      │
├─ 3. 实现用户 API ──────── BACKEND · 1h ──── ⬚     │
│     └─ 依赖: [2]                                    │
│                                                      │
└─ 4. 编写测试 ──────────── TEST · 1h ──── ⬚        │
      └─ 依赖: [2, 3]                                │
```

每个节点显示：
- 序号 + 标题 + 类型 badge + 工时估算
- 依赖关系（depends_on → 显示为 "依赖: [1, 2]"）
- 需求追溯（requirement_ref）
- 状态图标（PENDING/READY/RUNNING/COMPLETED）
- 可展开查看文件列表和描述

底部统计：总工时 X 小时 · 可并行 N 条

- [ ] **Step 3: 升级 PlanOutputCard**

读取 `plan-output-card.tsx`，修改为：
- 如果有 DAG 数据（tasks 带 depends_on），渲染 `DagTaskList`
- 如果是旧格式（无 depends_on），保持原有的简单列表
- 顶部显示总工时和并行轨道数
- 保留风险等级 badge 和风险因素列表

- [ ] **Step 4: 验证前端构建**

```bash
cd forge-portal && npm run build
```

- [ ] **Step 5: Commit**

```bash
git add forge-portal/
git commit -m "feat(s9): add DAG task visualization with dependencies and effort tracking"
```

---

## Task 6: 构建验证 + Docker 重建

- [ ] **Step 1: Go 构建**

```bash
cd forge-core && go build ./cmd/forge-core
```

- [ ] **Step 2: 前端构建**

```bash
cd forge-portal && npm run build
```

- [ ] **Step 3: 重建 AI Worker**

```bash
docker compose -f docker-compose.dev.yml up -d --build ai-worker
```

- [ ] **Step 4: 端到端验证**

1. 重启 Go 后端（新 binary）
2. 创建新任务 → AI 分析 → 确认
3. PLAN 步骤完成后：
   - DB 中 task_nodes 表有记录
   - API `GET /projects/:id/tasks/:taskId/nodes` 返回 DAG 数据
   - 前端 PlanOutputCard 显示依赖关系图
   - 每个节点有工时估算和需求追溯
4. 验证 DAG 无环（AI 不应该生成循环依赖）

- [ ] **Step 5: Commit**

```bash
git add -A
git commit -m "feat(s9): complete DAG task decomposition with dependency tracking"
```

---

## 验收标准

- [ ] AI 规划输出包含 `depends_on` 依赖关系（不再是纯顺序列表）
- [ ] AI 规划输出包含 `requirement_ref` 需求追溯
- [ ] AI 规划输出包含 `estimate_hours` 工时估算
- [ ] task_nodes 表正确存储 DAG 数据
- [ ] API 返回 DAG 节点列表
- [ ] 前端显示带依赖关系的任务列表（序号 + 依赖 + 工时 + 状态）
- [ ] 前端显示总工时和可并行轨道数
- [ ] `go build` + `npm run build` + ai-worker 重建通过
