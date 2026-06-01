## Phase 0 — 数据层（迁移 112 + sqlc）

**Goal:** `forge_check` sidecar 表 + sqlc 查询。
**Depends-on:** 无（WSL2 Go 工具链 F1 已建）　**Unblocks:** Phase 1, Phase 3
**Completion gate:** 迁移 up/down 可逆；`make sqlc` 生成 Forge check 查询；`go build ./...` 通过。

> 环境：沿用 F1。Go 命令走 `wsl -d Ubuntu -- bash -lc "cd /mnt/d/shulex_work/forge/server && go ..."`。

---

### Task 0.1: 迁移文件

> F1 用 111。F2 用 `112`（实现时 `ls server/migrations/ | grep -oE '^[0-9]+' | sort -n | tail -1` 复查）。

**Files:**
- Create: `server/migrations/112_forge_checks.up.sql`
- Create: `server/migrations/112_forge_checks.down.sql`

- [ ] **Step 1: 写 up 迁移**

`server/migrations/112_forge_checks.up.sql`：
```sql
-- Forge F2: verification gate checks (sidecar, forge_ prefix).
-- A check = name + shell command run in the task workdir after the agent
-- session ends. Non-zero exit = failure → task blocked + comment. Scope:
-- workspace-level (project_id NULL) + project-level (additive — all run).
CREATE TABLE forge_check (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL REFERENCES workspace(id) ON DELETE CASCADE,
    project_id   UUID REFERENCES project(id) ON DELETE CASCADE,
    name         TEXT NOT NULL,
    command      TEXT NOT NULL,
    enabled      BOOLEAN NOT NULL DEFAULT TRUE,
    created_by   UUID,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX uq_forge_check_ws
    ON forge_check (workspace_id, name) WHERE project_id IS NULL;
CREATE UNIQUE INDEX uq_forge_check_proj
    ON forge_check (project_id, name) WHERE project_id IS NOT NULL;
CREATE INDEX idx_forge_check_ws ON forge_check (workspace_id);
```

- [ ] **Step 2: 写 down 迁移**

`server/migrations/112_forge_checks.down.sql`：
```sql
DROP TABLE IF EXISTS forge_check;
```

- [ ] **Step 3: Commit**

```bash
git add server/migrations/112_forge_checks.up.sql server/migrations/112_forge_checks.down.sql
git commit -m "feat(forge): migration 112 forge_check"
```

---

### Task 0.2: sqlc 查询 + 生成

**Files:**
- Create: `server/pkg/db/queries/forge_checks.sql`
- Modify (generated): `server/pkg/db/generated/*` (via `make sqlc`)

- [ ] **Step 1: 写查询文件**

`server/pkg/db/queries/forge_checks.sql`：
```sql
-- Forge F2: verification checks

-- name: ListWorkspaceChecks :many
-- Workspace-level checks (project_id IS NULL), enabled only.
SELECT * FROM forge_check
WHERE workspace_id = $1 AND project_id IS NULL AND enabled = TRUE
ORDER BY name;

-- name: ListProjectChecks :many
-- Project-level checks for a project, enabled only.
SELECT * FROM forge_check
WHERE project_id = $1 AND enabled = TRUE
ORDER BY name;

-- name: ListChecksByWorkspace :many
-- All checks in a workspace (both scopes), for the list API.
SELECT * FROM forge_check
WHERE workspace_id = $1
ORDER BY project_id NULLS FIRST, name;

-- name: GetForgeCheck :one
SELECT * FROM forge_check WHERE id = $1 AND workspace_id = $2;

-- name: CreateForgeCheck :one
INSERT INTO forge_check (workspace_id, project_id, name, command, enabled, created_by)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;

-- name: UpdateForgeCheck :one
UPDATE forge_check SET
    name = COALESCE(sqlc.narg('name'), name),
    command = COALESCE(sqlc.narg('command'), command),
    enabled = COALESCE(sqlc.narg('enabled'), enabled),
    updated_at = now()
WHERE id = $1 AND workspace_id = $2
RETURNING *;

-- name: DeleteForgeCheck :exec
DELETE FROM forge_check WHERE id = $1 AND workspace_id = $2;
```

- [ ] **Step 2: 生成 + 校验**

Run:
```bash
wsl -d Ubuntu -- bash -lc "cd /mnt/d/shulex_work/forge/server && sqlc generate && go build ./... 2>&1 | tail -5"
```
Expected: 生成 `ForgeCheck` struct + `ListWorkspaceChecks`/`CreateForgeCheck` 等方法；`go build` 通过。

- [ ] **Step 3: Commit**

```bash
git add server/pkg/db/queries/forge_checks.sql server/pkg/db/generated/
git commit -m "feat(forge): sqlc queries for forge_check"
```

---

## Phase 0 完成检查
- [ ] 迁移 112 up/down 可逆
- [ ] sqlc 生成 Forge check 代码，编译通过
