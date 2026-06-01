## Phase 0 — 数据层（迁移 114 + sqlc）

**Goal:** `forge_entropy_scan` 表 + sqlc 查询（CRUD + `GetEntropyScanByAutopilot` + `ListOpenEntropyFindings`）。
**Depends-on:** 无　**Unblocks:** Phase 1, Phase 3
**Completion gate:** 迁移 up/down 可逆；`make sqlc`（WSL `sqlc generate`）生成；`go build ./...` 通过。

> 环境沿用 F1/F2/F3。Go 命令走 `wsl -d Ubuntu -- bash -lc "cd /mnt/d/shulex_work/forge/server && go ..."`。
> 迁移号：F3 用 113，本切片用 **114**（实施时复查 `ls server/migrations | tail` 确认最大号）。

---

### Task 0.1: 迁移文件

**Files:**
- Create: `server/migrations/114_forge_entropy_scan.up.sql`
- Create: `server/migrations/114_forge_entropy_scan.down.sql`

- [ ] **Step 1: up**

`server/migrations/114_forge_entropy_scan.up.sql`：
```sql
-- Forge F4: entropy scan config (sidecar, forge_ prefix). Defines a periodic
-- whole-repo quality scan for a scope. Forge manages a backing Autopilot
-- (schedule trigger = cron, execution_mode = create_issue, assignee = scanner
-- agent); autopilot_id links to it. The dispatch hook reverse-looks-up this row
-- by autopilot_id to compose the scanner agent's brief (F1 + F2 + custom + dedup).
CREATE TABLE forge_entropy_scan (
    id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id      UUID NOT NULL REFERENCES workspace(id) ON DELETE CASCADE,
    project_id        UUID REFERENCES project(id) ON DELETE CASCADE, -- NULL = workspace-level
    name              TEXT NOT NULL,
    scanner_agent_id  UUID NOT NULL REFERENCES agent(id) ON DELETE CASCADE,
    custom_focus      TEXT NOT NULL DEFAULT '',
    include_standards BOOLEAN NOT NULL DEFAULT TRUE,
    include_checks    BOOLEAN NOT NULL DEFAULT TRUE,
    cron_expression   TEXT NOT NULL,
    timezone          TEXT NOT NULL DEFAULT 'UTC',
    enabled           BOOLEAN NOT NULL DEFAULT TRUE,
    autopilot_id      UUID REFERENCES autopilot(id) ON DELETE SET NULL,
    created_by        UUID,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_forge_entropy_scan_ws ON forge_entropy_scan(workspace_id);
CREATE UNIQUE INDEX idx_forge_entropy_scan_autopilot
    ON forge_entropy_scan(autopilot_id) WHERE autopilot_id IS NOT NULL;
```

- [ ] **Step 2: down**

`server/migrations/114_forge_entropy_scan.down.sql`：
```sql
DROP TABLE IF EXISTS forge_entropy_scan;
```

- [ ] **Step 3: Commit**

```bash
git add server/migrations/114_forge_entropy_scan.up.sql server/migrations/114_forge_entropy_scan.down.sql
git commit -m "feat(forge): migration 114 forge_entropy_scan"
```

---

### Task 0.2: sqlc 查询

**Files:**
- Create: `server/pkg/db/queries/forge_entropy.sql`

- [ ] **Step 1: 写查询**

`server/pkg/db/queries/forge_entropy.sql`：
```sql
-- Forge F4: entropy scan config CRUD + dispatch-hook reverse lookup + dedup list.

-- name: CreateEntropyScan :one
INSERT INTO forge_entropy_scan (
    workspace_id, project_id, name, scanner_agent_id, custom_focus,
    include_standards, include_checks, cron_expression, timezone, enabled, created_by
) VALUES ($1, sqlc.narg('project_id'), $2, $3, $4, $5, $6, $7, $8, $9, $10)
RETURNING *;

-- name: SetEntropyScanAutopilot :exec
UPDATE forge_entropy_scan SET autopilot_id = $2, updated_at = now() WHERE id = $1;

-- name: GetEntropyScan :one
SELECT * FROM forge_entropy_scan WHERE id = $1 AND workspace_id = $2;

-- name: GetEntropyScanByAutopilot :one
SELECT * FROM forge_entropy_scan WHERE autopilot_id = $1;

-- name: ListEntropyScans :many
SELECT * FROM forge_entropy_scan
WHERE workspace_id = $1
  AND project_id IS NOT DISTINCT FROM sqlc.narg('project_id')
ORDER BY created_at DESC;

-- name: UpdateEntropyScan :one
UPDATE forge_entropy_scan SET
    name = $3, scanner_agent_id = $4, custom_focus = $5, include_standards = $6,
    include_checks = $7, cron_expression = $8, timezone = $9, enabled = $10, updated_at = now()
WHERE id = $1 AND workspace_id = $2
RETURNING *;

-- name: DeleteEntropyScan :one
DELETE FROM forge_entropy_scan WHERE id = $1 AND workspace_id = $2
RETURNING autopilot_id;

-- name: ListOpenEntropyFindings :many
-- Dedup list: open (non-terminal) issues carrying the 'forge-entropy' label in
-- this scope. project_id narg NULL = whole workspace.
SELECT i.id, i.number, i.title
FROM issue i
JOIN issue_to_label il ON il.issue_id = i.id
JOIN issue_label l ON l.id = il.label_id
WHERE i.workspace_id = $1
  AND (sqlc.narg('project_id')::uuid IS NULL OR i.project_id = sqlc.narg('project_id'))
  AND l.name = 'forge-entropy'
  AND i.status NOT IN ('done', 'cancelled')
ORDER BY i.created_at DESC
LIMIT 100;
```

- [ ] **Step 2: 生成 + 校验**

Run: `wsl -d Ubuntu -- bash -lc "cd /mnt/d/shulex_work/forge/server && sqlc generate && go build ./... 2>&1 | tail -8"`
Expected: 生成 `ForgeEntropyScan` struct（含 `WorkspaceID/ProjectID/Name/ScannerAgentID/CustomFocus/IncludeStandards/IncludeChecks/CronExpression/Timezone/Enabled/AutopilotID pgtype.UUID`）、
`CreateEntropyScanParams`、`GetEntropyScanByAutopilotRow`/返回 `ForgeEntropyScan`、`ListOpenEntropyFindingsRow`（ID/Number/Title）、
`ListEntropyScansParams`、`UpdateEntropyScanParams`；`go build` 通过。

> 若 `ListOpenEntropyFindings` 的 label join 因实际 schema 列名报错，按生成错误校正表/列名
> （`issue_to_label(issue_id,label_id)`、`issue_label(id,name)` 已核实存在）；去重是软保证，
> 查询失败在 Phase 1 的 `ResolveBrief` 内被吞掉、退化为空清单，不阻断。

- [ ] **Step 3: Commit**

```bash
git add server/pkg/db/queries/forge_entropy.sql server/pkg/db/generated/
git commit -m "feat(forge): sqlc queries for forge_entropy_scan + dedup findings"
```

---

## Phase 0 完成检查
- [ ] 迁移 114 up/down 可逆
- [ ] sqlc 生成 scan CRUD + `GetEntropyScanByAutopilot` + `ListOpenEntropyFindings`，编译通过
