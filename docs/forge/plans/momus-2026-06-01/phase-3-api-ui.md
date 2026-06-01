## Phase 3 — API + UI（review-config GET/PUT + 设置面）

**Goal:** `/api/forge/review-config` GET/PUT（按 scope 配 reviewer agent + enabled）+ 设置面 UI。

**Depends-on:** Phase 0　**Unblocks:** Phase 4
**Completion gate:** handler 编译 + 路由；core/views/web typecheck 绿。

> 比 F1/F2 简单：每 scope 单一 config（GET 当前、PUT upsert），非列表 CRUD。UI 复用
> `agentListOptions`（F1 探查确认存在）做 reviewer 下拉。

---

### Task 3.1: handler（GET/PUT）

**Files:**
- Create: `server/internal/handler/forge_review.go`
- Modify: `server/cmd/server/router.go`

- [ ] **Step 1: 写 handler**

`server/internal/handler/forge_review.go`：
```go
package handler

import (
	"encoding/json"
	"net/http"

	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

type ForgeReviewConfigBody struct {
	ProjectID       string `json:"project_id,omitempty"` // empty = workspace-level
	ReviewerAgentID string `json:"reviewer_agent_id"`
	Enabled         bool   `json:"enabled"`
}

func (h *Handler) GetForgeReviewConfig(w http.ResponseWriter, r *http.Request) {
	wsID := h.resolveWorkspaceID(r)
	if wsID == "" {
		writeError(w, http.StatusBadRequest, "workspace_id is required")
		return
	}
	var projParam pgtype.UUID
	if pid := r.URL.Query().Get("project_id"); pid != "" {
		p, ok := parseUUIDOrBadRequest(w, pid, "project_id")
		if !ok {
			return
		}
		projParam = p
	}
	cfg, err := h.Queries.GetReviewConfigByScope(r.Context(), db.GetReviewConfigByScopeParams{
		WorkspaceID: parseUUID(wsID),
		ProjectID:   projParam,
	})
	if err != nil {
		writeJSON(w, http.StatusOK, ForgeReviewConfigBody{}) // none configured
		return
	}
	out := ForgeReviewConfigBody{ReviewerAgentID: uuidToString(cfg.ReviewerAgentID), Enabled: cfg.Enabled}
	if cfg.ProjectID.Valid {
		out.ProjectID = uuidToString(cfg.ProjectID)
	}
	writeJSON(w, http.StatusOK, out)
}

func (h *Handler) PutForgeReviewConfig(w http.ResponseWriter, r *http.Request) {
	wsID := h.resolveWorkspaceID(r)
	if wsID == "" {
		writeError(w, http.StatusBadRequest, "workspace_id is required")
		return
	}
	userID, ok := requireUserID(w, r)
	if !ok {
		return
	}
	var req ForgeReviewConfigBody
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	reviewerID, ok := parseUUIDOrBadRequest(w, req.ReviewerAgentID, "reviewer_agent_id")
	if !ok {
		return
	}
	if req.ProjectID != "" {
		projID, ok := parseUUIDOrBadRequest(w, req.ProjectID, "project_id")
		if !ok {
			return
		}
		cfg, err := h.Queries.UpsertProjectReviewConfig(r.Context(), db.UpsertProjectReviewConfigParams{
			WorkspaceID: parseUUID(wsID), ProjectID: projID, ReviewerAgentID: reviewerID,
			Enabled: req.Enabled, CreatedBy: parseUUID(userID),
		})
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to save review config")
			return
		}
		writeJSON(w, http.StatusOK, ForgeReviewConfigBody{ProjectID: uuidToString(cfg.ProjectID), ReviewerAgentID: uuidToString(cfg.ReviewerAgentID), Enabled: cfg.Enabled})
		return
	}
	cfg, err := h.Queries.UpsertWorkspaceReviewConfig(r.Context(), db.UpsertWorkspaceReviewConfigParams{
		WorkspaceID: parseUUID(wsID), ReviewerAgentID: reviewerID, Enabled: req.Enabled, CreatedBy: parseUUID(userID),
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to save review config")
		return
	}
	writeJSON(w, http.StatusOK, ForgeReviewConfigBody{ReviewerAgentID: uuidToString(cfg.ReviewerAgentID), Enabled: cfg.Enabled})
}
```
> 实现时按 sqlc 生成的 params 字段名核对（`GetReviewConfigByScopeParams` 的 ProjectID 用
> `sqlc.narg` 生成可空 pgtype.UUID）。若 Upsert 的 `ON CONFLICT WHERE` 生成有问题，退回
> get-then-create/update 两步。

- [ ] **Step 2: 路由**（workspace-scoped 组，`/api/forge/checks` 旁）

```go
r.Route("/api/forge/review-config", func(r chi.Router) {
	r.Get("/", h.GetForgeReviewConfig)
	r.Put("/", h.PutForgeReviewConfig)
})
```

- [ ] **Step 3: 编译 + commit**

Run: `wsl -d Ubuntu -- bash -lc "cd /mnt/d/shulex_work/forge/server && go build ./... 2>&1 | tail -8"`
Expected: 通过。
```bash
git add server/internal/handler/forge_review.go server/cmd/server/router.go
git commit -m "feat(forge): review-config GET/PUT API + routes"
```

---

### Task 3.2: 前端（设置面，复用 agent 下拉）

**Files:**
- Create: `packages/core/types/forge-review.ts`（`ForgeReviewConfig` {project_id?, reviewer_agent_id, enabled}）
- Modify: `packages/core/types/index.ts`（加 export）
- Modify: `packages/core/api/client.ts`（`getForgeReviewConfig(projectId?)` + `putForgeReviewConfig(body)`）
- Modify: `packages/core/workspace/queries.ts`（`forgeReviewConfig` key + `forgeReviewConfigOptions(wsId, projectId?)`）
- Create: `packages/views/forge-review/forge-review-page.tsx`（reviewer agent 下拉 + enabled 开关 + 保存；用 `agentListOptions(wsId)` 列 agent 供选）
- Create: `packages/views/forge-review/index.ts`
- Modify: `packages/views/package.json`（加 `"./forge-review"` export）
- Create: `apps/web/app/[workspaceSlug]/(dashboard)/forge-review/page.tsx`

- [ ] **Step 1: 接线**

镜像 F1/F2 的 core 接线（types/client/queries）。view：`useQuery(forgeReviewConfigOptions(wsId))`
取当前配置 + `useQuery(agentListOptions(wsId))` 列 agent；`<select>` 选 reviewer + 开关；
保存走 `putForgeReviewConfig` mutation + invalidate。zod 解析遵循 CLAUDE.md API 兼容规则。

- [ ] **Step 2: typecheck**

Run (Windows): `cd D:\shulex_work\forge; corepack pnpm --filter "@multica/core" --filter "@multica/views" --filter "@multica/web" typecheck 2>&1 | Select-Object -Last 20`
Expected: 三包 Done。

- [ ] **Step 3: Commit**

```bash
git add packages/core/types/forge-review.ts packages/core/types/index.ts packages/core/api/client.ts packages/core/workspace/queries.ts packages/views/forge-review/ packages/views/package.json "apps/web/app/[workspaceSlug]/(dashboard)/forge-review/page.tsx"
git commit -m "feat(forge): review-config UI — reviewer agent picker + toggle"
```

---

## Phase 3 完成检查
- [ ] review-config GET/PUT handler + 路由编译通过
- [ ] 前端三包 typecheck 绿
