## Phase 0 — Build env + 数据层（迁移 + sqlc）

**Goal:** 建立 Go 构建/测试环境；创建 `forge_standard` + `forge_project_profile` 迁移；
写 sqlc 查询并生成代码。

**Depends-on:** 无　**Unblocks:** Phase 1, Phase 3
**Completion gate:** 迁移可 up/down；`make sqlc` 生成 Forge 查询的 Go 代码；`go build ./...` 通过。

---

### Task 0.1: 建立 Go 构建/测试环境

- [ ] **Step 1: WSL2 装 Go 1.26 + pnpm（用户级，免 sudo）**

Run（WSL2 Ubuntu）:
```bash
cd ~ && curl -fsSL https://go.dev/dl/go1.26.1.linux-amd64.tar.gz -o /tmp/go.tgz
mkdir -p ~/.local && tar -C ~/.local -xzf /tmp/go.tgz   # → ~/.local/go
echo 'export PATH=$HOME/.local/go/bin:$HOME/go/bin:$PATH' >> ~/.bashrc
export PATH=$HOME/.local/go/bin:$HOME/go/bin:$PATH
curl -fsSL https://get.pnpm.io/install.sh | sh -        # → ~/.local/share/pnpm
go version && ~/.local/share/pnpm/pnpm --version
```
Expected: `go version go1.26.1 linux/amd64`；pnpm ≥ 10.28。

- [ ] **Step 2: 装 sqlc + 确认仓库可访问**

Run（WSL2，仓库在 /mnt/d 或 cp 到 ~）:
```bash
go install github.com/sqlc-dev/sqlc/cmd/sqlc@latest
cd <forge-repo>/server && go build ./... 2>&1 | tail -5
```
Expected: sqlc 装好；`go build ./...` 通过（确认基线编译干净）。

---

### Task 0.2: 创建迁移文件

> Multica 迁移到 `110_*`。Forge 用 `111`（实现时 `ls server/migrations/ | sort | tail` 复查最大号）。

**Files:**
- Create: `server/migrations/111_forge_standards.up.sql`
- Create: `server/migrations/111_forge_standards.down.sql`

- [ ] **Step 1: 写 up 迁移**

`server/migrations/111_forge_standards.up.sql`：
```sql
-- Forge F1: spec-center Standards (sidecar tables, forge_ prefix).
-- A standard = categorized + profile-tagged coding convention with a mandatory
-- core part (injected into agent instructions) and a detailed part (compiled
-- into an on-demand skill). Scope: workspace-level (project_id NULL) overridden
-- by project-level. Forge-owned; does not touch Multica's project/skill tables.

CREATE TABLE forge_standard (
    id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id   UUID NOT NULL REFERENCES workspace(id) ON DELETE CASCADE,
    project_id     UUID REFERENCES project(id) ON DELETE CASCADE,
    name           TEXT NOT NULL,
    category       TEXT NOT NULL,
    profile_tags   TEXT[] NOT NULL DEFAULT '{}',
    core_content   TEXT NOT NULL DEFAULT '',
    detail_content TEXT NOT NULL DEFAULT '',
    enabled        BOOLEAN NOT NULL DEFAULT TRUE,
    created_by     UUID,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Override key uniqueness: (category,name) unique within each scope.
CREATE UNIQUE INDEX uq_forge_standard_ws
    ON forge_standard (workspace_id, category, name) WHERE project_id IS NULL;
CREATE UNIQUE INDEX uq_forge_standard_proj
    ON forge_standard (project_id, category, name) WHERE project_id IS NOT NULL;
CREATE INDEX idx_forge_standard_ws ON forge_standard (workspace_id);

CREATE TABLE forge_project_profile (
    project_id   UUID PRIMARY KEY REFERENCES project(id) ON DELETE CASCADE,
    tags         TEXT[] NOT NULL DEFAULT '{}',
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);
```

- [ ] **Step 2: 写 down 迁移**

`server/migrations/111_forge_standards.down.sql`：
```sql
DROP TABLE IF EXISTS forge_project_profile;
DROP TABLE IF EXISTS forge_standard;
```

- [ ] **Step 3: 应用迁移验证 up/down**

Run（对 selfhost 的 postgres，或本地测试库）:
```bash
cd <forge-repo>/server && make migrate-up 2>&1 | tail -3
# 验证表存在
psql "$DATABASE_URL" -c "\d forge_standard" | head -20
make migrate-down 2>&1 | tail -3 && make migrate-up 2>&1 | tail -3
```
Expected: up 后两表存在；down 干净回滚；再 up 成功（幂等可逆）。

- [ ] **Step 4: Commit**

```bash
git add server/migrations/111_forge_standards.up.sql server/migrations/111_forge_standards.down.sql
git commit -m "feat(forge): migration 111 forge_standard + forge_project_profile"
```

---

### Task 0.3: sqlc 查询 + 生成

**Files:**
- Create: `server/pkg/db/queries/forge_standards.sql`
- Modify (generated): `server/pkg/db/generated/*` (via `make sqlc`)

- [ ] **Step 1: 写查询文件**

`server/pkg/db/queries/forge_standards.sql`：
```sql
-- Forge F1: standards + project profile

-- name: ListWorkspaceStandards :many
-- Workspace-level standards (project_id IS NULL), enabled only. Used by resolve.
SELECT * FROM forge_standard
WHERE workspace_id = $1 AND project_id IS NULL AND enabled = TRUE
ORDER BY category, name;

-- name: ListProjectStandards :many
-- Project-level overrides for a specific project, enabled only.
SELECT * FROM forge_standard
WHERE project_id = $1 AND enabled = TRUE
ORDER BY category, name;

-- name: ListStandardsByWorkspace :many
-- All standards in a workspace (both scopes), for the list API.
SELECT * FROM forge_standard
WHERE workspace_id = $1
ORDER BY project_id NULLS FIRST, category, name;

-- name: GetForgeStandard :one
SELECT * FROM forge_standard WHERE id = $1 AND workspace_id = $2;

-- name: CreateForgeStandard :one
INSERT INTO forge_standard
    (workspace_id, project_id, name, category, profile_tags, core_content, detail_content, enabled, created_by)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
RETURNING *;

-- name: UpdateForgeStandard :one
UPDATE forge_standard SET
    name = COALESCE(sqlc.narg('name'), name),
    category = COALESCE(sqlc.narg('category'), category),
    profile_tags = COALESCE(sqlc.narg('profile_tags'), profile_tags),
    core_content = COALESCE(sqlc.narg('core_content'), core_content),
    detail_content = COALESCE(sqlc.narg('detail_content'), detail_content),
    enabled = COALESCE(sqlc.narg('enabled'), enabled),
    updated_at = now()
WHERE id = $1 AND workspace_id = $2
RETURNING *;

-- name: DeleteForgeStandard :exec
DELETE FROM forge_standard WHERE id = $1 AND workspace_id = $2;

-- name: GetForgeProjectProfile :one
SELECT * FROM forge_project_profile WHERE project_id = $1;

-- name: UpsertForgeProjectProfile :one
INSERT INTO forge_project_profile (project_id, tags, updated_at)
VALUES ($1, $2, now())
ON CONFLICT (project_id) DO UPDATE SET tags = EXCLUDED.tags, updated_at = now()
RETURNING *;
```

- [ ] **Step 2: 生成 sqlc 代码**

Run:
```bash
cd <forge-repo>/server && make sqlc 2>&1 | tail -5
go build ./... 2>&1 | tail -5
```
Expected: `make sqlc` 无错；生成 `pkg/db/generated` 里的 `ForgeStandard`/`ForgeProjectProfile`
struct 与 `ListWorkspaceStandards` 等方法；`go build ./...` 通过。

- [ ] **Step 3: Commit**

```bash
git add server/pkg/db/queries/forge_standards.sql server/pkg/db/generated/
git commit -m "feat(forge): sqlc queries for forge_standard + forge_project_profile"
```

---

## Phase 0 完成检查
- [ ] WSL2 Go 1.26 + pnpm + sqlc 就位，`go build ./...` 通过
- [ ] 迁移 111 up/down 可逆，两表创建
- [ ] sqlc 生成 Forge 查询代码，编译通过
