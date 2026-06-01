## Phase 0 — 数据层（迁移 113 + sqlc + review-task query）

**Goal:** `forge_review_config` 表 + sqlc 查询 + 建 review 任务的 `CreateForgeReviewTask`。
**Depends-on:** 无　**Unblocks:** Phase 1, Phase 3
**Completion gate:** 迁移 up/down 可逆；`make sqlc` 生成；`go build ./...` 通过。

> 环境沿用 F1/F2。Go 命令走 `wsl -d Ubuntu -- bash -lc "cd /mnt/d/shulex_work/forge/server && go ..."`。

---

### Task 0.1: 迁移文件

> F2 用 112。F3 用 `113`（实现时复查最大号）。

**Files:**
- Create: `server/migrations/113_forge_review_config.up.sql`
- Create: `server/migrations/113_forge_review_config.down.sql`

- [ ] **Step 1: up**

`server/migrations/113_forge_review_config.up.sql`：
```sql
-- Forge F3: AI review config (sidecar, forge_ prefix). Designates the reviewer
-- agent for a scope. After a coding task completes, Forge enqueues a review
-- task for this agent against the same issue, reusing the coder's workdir.
CREATE TABLE forge_review_config (
    id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id      UUID NOT NULL REFERENCES workspace(id) ON DELETE CASCADE,
    project_id        UUID REFERENCES project(id) ON DELETE CASCADE,
    reviewer_agent_id UUID NOT NULL REFERENCES agent(id) ON DELETE CASCADE,
    enabled           BOOLEAN NOT NULL DEFAULT TRUE,
    created_by        UUID,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX uq_forge_review_ws   ON forge_review_config (workspace_id) WHERE project_id IS NULL;
CREATE UNIQUE INDEX uq_forge_review_proj ON forge_review_config (project_id)   WHERE project_id IS NOT NULL;
```

- [ ] **Step 2: down**

`server/migrations/113_forge_review_config.down.sql`：
```sql
DROP TABLE IF EXISTS forge_review_config;
```

- [ ] **Step 3: Commit**

```bash
git add server/migrations/113_forge_review_config.up.sql server/migrations/113_forge_review_config.down.sql
git commit -m "feat(forge): migration 113 forge_review_config"
```

---

### Task 0.2: sqlc 查询

**Files:**
- Create: `server/pkg/db/queries/forge_review.sql`

- [ ] **Step 1: 写查询**

`server/pkg/db/queries/forge_review.sql`：
```sql
-- Forge F3: review config + review task creation

-- name: GetWorkspaceReviewConfig :one
SELECT * FROM forge_review_config
WHERE workspace_id = $1 AND project_id IS NULL AND enabled = TRUE;

-- name: GetProjectReviewConfig :one
SELECT * FROM forge_review_config
WHERE project_id = $1 AND enabled = TRUE;

-- name: GetReviewConfigByScope :one
-- For the GET API: workspace-level when project_id is NULL arg, else project.
SELECT * FROM forge_review_config
WHERE workspace_id = $1
  AND project_id IS NOT DISTINCT FROM sqlc.narg('project_id');

-- name: UpsertWorkspaceReviewConfig :one
INSERT INTO forge_review_config (workspace_id, project_id, reviewer_agent_id, enabled, created_by)
VALUES ($1, NULL, $2, $3, $4)
ON CONFLICT (workspace_id) WHERE project_id IS NULL
DO UPDATE SET reviewer_agent_id = EXCLUDED.reviewer_agent_id, enabled = EXCLUDED.enabled, updated_at = now()
RETURNING *;

-- name: UpsertProjectReviewConfig :one
INSERT INTO forge_review_config (workspace_id, project_id, reviewer_agent_id, enabled, created_by)
VALUES ($1, $2, $3, $4, $5)
ON CONFLICT (project_id) WHERE project_id IS NOT NULL
DO UPDATE SET reviewer_agent_id = EXCLUDED.reviewer_agent_id, enabled = EXCLUDED.enabled, updated_at = now()
RETURNING *;

-- name: CreateForgeReviewTask :one
-- Enqueue a review task for the reviewer agent against the same issue, reusing
-- the coder's workdir. The forge_review marker in context prevents review loops.
INSERT INTO agent_task_queue (
    agent_id, runtime_id, issue_id, parent_task_id, work_dir, context, status, priority
)
VALUES ($1, $2, $3, $4, $5, $6, 'queued', $7)
RETURNING *;
```
> 注：`ON CONFLICT ... WHERE` 对 partial unique index 的 upsert，pg 支持（conflict_target 带
> index predicate）。若 sqlc 生成有问题，退回 handler 内 get-then-create/update 两步。

- [ ] **Step 2: 生成 + 校验**

Run: `wsl -d Ubuntu -- bash -lc "cd /mnt/d/shulex_work/forge/server && sqlc generate && go build ./... 2>&1 | tail -8"`
Expected: 生成 `ForgeReviewConfig` struct + `CreateForgeReviewTaskParams`（AgentID/RuntimeID/IssueID/
ParentTaskID/WorkDir/Context/Priority）+ config 查询；`go build` 通过。

- [ ] **Step 3: Commit**

```bash
git add server/pkg/db/queries/forge_review.sql server/pkg/db/generated/
git commit -m "feat(forge): sqlc queries for forge_review_config + review task"
```

---

## Phase 0 完成检查
- [ ] 迁移 113 up/down 可逆
- [ ] sqlc 生成 review config + CreateForgeReviewTask，编译通过
