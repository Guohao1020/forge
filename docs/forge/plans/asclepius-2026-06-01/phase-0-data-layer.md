## Phase 0 — 数据层（迁移 115 + sqlc）

**Goal:** `forge_entropy_scan.auto_fix` 列 + 新 sidecar `forge_fix_pr` + sqlc（CreateFixPR 幂等；auto_fix 并入 entropy CRUD）。
**Depends-on:** 无　**Unblocks:** Phase 1, 2, 3
**Completion gate:** 迁移 up/down 可逆；`sqlc generate` 生成；`go build ./...` 通过。

> Go/sqlc 走 `wsl -d Ubuntu -- bash -lc "cd /mnt/d/shulex_work/forge/server && ..."`。WSL git 无 identity →
> `git commit` 用原生 Windows git(无 `--no-verify`)。迁移号:F4 用 114,本切片用 **115**(实施时 `ls migrations | tail` 复查最大号)。

---

### Task 0.1: 迁移文件

**Files:**
- Create: `server/migrations/115_forge_fix_pr.up.sql`
- Create: `server/migrations/115_forge_fix_pr.down.sql`

- [ ] **Step 1: up**

`server/migrations/115_forge_fix_pr.up.sql`：
```sql
-- Forge F4b: self-healing loop. Per-scan auto_fix flag enables the scanner agent
-- to fix what it safely can and open a PR. forge_fix_pr records the PR the agent
-- opened against its issue (self-contained; does NOT touch the GitHub-App-coupled
-- github_pull_request table).
ALTER TABLE forge_entropy_scan ADD COLUMN auto_fix BOOLEAN NOT NULL DEFAULT FALSE;

CREATE TABLE forge_fix_pr (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL REFERENCES workspace(id) ON DELETE CASCADE,
    issue_id     UUID NOT NULL REFERENCES issue(id) ON DELETE CASCADE,
    task_id      UUID NOT NULL REFERENCES agent_task_queue(id) ON DELETE CASCADE,
    pr_url       TEXT NOT NULL,
    branch       TEXT NOT NULL DEFAULT '',
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_forge_fix_pr_issue ON forge_fix_pr(issue_id);
CREATE UNIQUE INDEX idx_forge_fix_pr_task_url ON forge_fix_pr(task_id, pr_url);
```

- [ ] **Step 2: down**

`server/migrations/115_forge_fix_pr.down.sql`：
```sql
DROP TABLE IF EXISTS forge_fix_pr;
ALTER TABLE forge_entropy_scan DROP COLUMN IF EXISTS auto_fix;
```

- [ ] **Step 3: Commit**

```bash
git add server/migrations/115_forge_fix_pr.up.sql server/migrations/115_forge_fix_pr.down.sql
git commit -m "feat(forge): migration 115 auto_fix column + forge_fix_pr"
```

---

### Task 0.2: sqlc — forge_fix_pr 查询 + auto_fix 并入 entropy CRUD

**Files:**
- Create: `server/pkg/db/queries/forge_fix_pr.sql`
- Modify: `server/pkg/db/queries/forge_entropy.sql`（CreateEntropyScan + UpdateEntropyScan 加 auto_fix）

- [ ] **Step 1: 新查询文件**

`server/pkg/db/queries/forge_fix_pr.sql`：
```sql
-- Forge F4b: record the PR a coding agent opened against its issue (idempotent).

-- name: CreateFixPR :one
INSERT INTO forge_fix_pr (workspace_id, issue_id, task_id, pr_url, branch)
VALUES ($1, $2, $3, $4, $5)
ON CONFLICT (task_id, pr_url) DO NOTHING
RETURNING *;

-- name: ListFixPRsByIssue :many
SELECT * FROM forge_fix_pr WHERE issue_id = $1 ORDER BY created_at DESC;
```
> `:one` + `ON CONFLICT DO NOTHING`:冲突时不返回行 → 生成的 `CreateFixPR` 返回 `pgx.ErrNoRows`。
> Phase 2 用它区分"已录入(幂等跳过、不重复评论)"。

- [ ] **Step 2: 给 entropy CRUD 加 auto_fix**

编辑 `server/pkg/db/queries/forge_entropy.sql`：

`CreateEntropyScan` 改为(列表 + 值各加 auto_fix=$11):
```sql
-- name: CreateEntropyScan :one
INSERT INTO forge_entropy_scan (
    workspace_id, project_id, name, scanner_agent_id, custom_focus,
    include_standards, include_checks, cron_expression, timezone, enabled, created_by, auto_fix
) VALUES ($1, sqlc.narg('project_id'), $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
RETURNING *;
```

`UpdateEntropyScan` 改为(SET 加 `auto_fix = $11`):
```sql
-- name: UpdateEntropyScan :one
UPDATE forge_entropy_scan SET
    name = $3, scanner_agent_id = $4, custom_focus = $5, include_standards = $6,
    include_checks = $7, cron_expression = $8, timezone = $9, enabled = $10, auto_fix = $11, updated_at = now()
WHERE id = $1 AND workspace_id = $2
RETURNING *;
```
（其余 query 不动；`ListEntropyScans`/`GetEntropyScan`/`GetEntropyScanByAutopilot` 是 `SELECT *`,auto_fix 自动带出。）

- [ ] **Step 3: 生成 + 校验**

Run: `wsl -d Ubuntu -- bash -lc "cd /mnt/d/shulex_work/forge/server && sqlc generate && go build ./... 2>&1 | tail -10"`
Expected: 生成 `ForgeFixPr` struct（ID/WorkspaceID/IssueID/TaskID pgtype.UUID、PrUrl string、Branch string、CreatedAt）、
`CreateFixPRParams{WorkspaceID, IssueID, TaskID, PrUrl, Branch}`、`ListFixPRsByIssue`；
`CreateEntropyScanParams` / `UpdateEntropyScanParams` 各新增 `AutoFix bool`；`ForgeEntropyScan` struct 多 `AutoFix bool`。
`go build ./...` 通过(此时 handler 还没传 auto_fix,但 Create/UpdateEntropyScanParams 多一个字段不影响编译——字段零值;Phase 3 再透传)。

> 若 `go build` 因 `CreateEntropyScanParams` 多字段在 handler 处报"missing field"——不会,Go struct 字面量允许省略字段(零值)。
> 但若 handler 用了**位置**字面量(无字段名)才会报错——F4 handler 用的是带字段名的字面量,安全。

- [ ] **Step 4: Commit**

```bash
git add server/pkg/db/queries/forge_fix_pr.sql server/pkg/db/queries/forge_entropy.sql server/pkg/db/generated/
git commit -m "feat(forge): sqlc forge_fix_pr + auto_fix in entropy CRUD"
```

---

## Phase 0 完成检查
- [ ] 迁移 115 up/down 可逆（auto_fix 列 + forge_fix_pr 表）
- [ ] sqlc 生成 `ForgeFixPr` + `CreateFixPR`(幂等) + `AutoFix` 进 entropy params/struct，编译通过
