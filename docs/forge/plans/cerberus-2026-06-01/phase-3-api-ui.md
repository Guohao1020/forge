## Phase 3 — API + UI（checks CRUD + views）

**Goal:** `/api/forge/checks` CRUD + `packages/views/forge-checks/` 列表/编辑。紧密镜像 F1
forge-standards（commit `b09f2ff4`）。

**Depends-on:** Phase 0（sqlc）　**Unblocks:** Phase 4
**Completion gate:** handler 编译 + 路由；core/views/web typecheck 绿。

> 模板：F1 的 `server/internal/handler/forge_standards.go`、`packages/views/forge-standards/`、
> `packages/core/{types,api,workspace}` 的 forge-standard 接线。check 比 standard 简单
> （字段只有 name/command/project_id/enabled）。

---

### Task 3.1: handler（CRUD）

**Files:**
- Create: `server/internal/handler/forge_checks.go`
- Modify: `server/cmd/server/router.go`

- [ ] **Step 1: 写 handler（镜像 forge_standards.go，字段精简）**

`server/internal/handler/forge_checks.go`：
```go
package handler

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

type ForgeCheckRequest struct {
	ProjectID string `json:"project_id,omitempty"` // empty = workspace-level
	Name      string `json:"name"`
	Command   string `json:"command"`
	Enabled   *bool  `json:"enabled,omitempty"`
}

type ForgeCheckResponse struct {
	ID          string `json:"id"`
	WorkspaceID string `json:"workspace_id"`
	ProjectID   string `json:"project_id,omitempty"`
	Name        string `json:"name"`
	Command     string `json:"command"`
	Enabled     bool   `json:"enabled"`
}

func forgeCheckToResponse(c db.ForgeCheck) ForgeCheckResponse {
	r := ForgeCheckResponse{
		ID: uuidToString(c.ID), WorkspaceID: uuidToString(c.WorkspaceID),
		Name: c.Name, Command: c.Command, Enabled: c.Enabled,
	}
	if c.ProjectID.Valid {
		r.ProjectID = uuidToString(c.ProjectID)
	}
	return r
}

func (h *Handler) ListForgeChecks(w http.ResponseWriter, r *http.Request) {
	wsID := h.resolveWorkspaceID(r)
	if wsID == "" {
		writeError(w, http.StatusBadRequest, "workspace_id is required")
		return
	}
	rows, err := h.Queries.ListChecksByWorkspace(r.Context(), parseUUID(wsID))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list checks")
		return
	}
	out := make([]ForgeCheckResponse, len(rows))
	for i, c := range rows {
		out[i] = forgeCheckToResponse(c)
	}
	writeJSON(w, http.StatusOK, out)
}

func (h *Handler) CreateForgeCheck(w http.ResponseWriter, r *http.Request) {
	wsID := h.resolveWorkspaceID(r)
	if wsID == "" {
		writeError(w, http.StatusBadRequest, "workspace_id is required")
		return
	}
	userID, ok := requireUserID(w, r)
	if !ok {
		return
	}
	var req ForgeCheckRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Name == "" || req.Command == "" {
		writeError(w, http.StatusBadRequest, "name and command are required")
		return
	}
	var projID pgtype.UUID
	if req.ProjectID != "" {
		p, valid := parseUUIDOrBadRequest(w, req.ProjectID, "project_id")
		if !valid {
			return
		}
		projID = p
	}
	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}
	c, err := h.Queries.CreateForgeCheck(r.Context(), db.CreateForgeCheckParams{
		WorkspaceID: parseUUID(wsID), ProjectID: projID, Name: req.Name,
		Command: req.Command, Enabled: enabled, CreatedBy: parseUUID(userID),
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create check")
		return
	}
	writeJSON(w, http.StatusCreated, forgeCheckToResponse(c))
}

func (h *Handler) loadForgeCheck(w http.ResponseWriter, r *http.Request) (db.ForgeCheck, bool) {
	wsID := h.resolveWorkspaceID(r)
	if wsID == "" {
		writeError(w, http.StatusBadRequest, "workspace_id is required")
		return db.ForgeCheck{}, false
	}
	id, ok := parseUUIDOrBadRequest(w, chi.URLParam(r, "id"), "check id")
	if !ok {
		return db.ForgeCheck{}, false
	}
	c, err := h.Queries.GetForgeCheck(r.Context(), db.GetForgeCheckParams{ID: id, WorkspaceID: parseUUID(wsID)})
	if err != nil {
		writeError(w, http.StatusNotFound, "check not found")
		return db.ForgeCheck{}, false
	}
	return c, true
}

func (h *Handler) UpdateForgeCheck(w http.ResponseWriter, r *http.Request) {
	existing, ok := h.loadForgeCheck(w, r)
	if !ok {
		return
	}
	var req ForgeCheckRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	params := db.UpdateForgeCheckParams{ID: existing.ID, WorkspaceID: existing.WorkspaceID}
	if req.Name != "" {
		params.Name = pgtype.Text{String: req.Name, Valid: true}
	}
	if req.Command != "" {
		params.Command = pgtype.Text{String: req.Command, Valid: true}
	}
	if req.Enabled != nil {
		params.Enabled = pgtype.Bool{Bool: *req.Enabled, Valid: true}
	}
	c, err := h.Queries.UpdateForgeCheck(r.Context(), params)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update check")
		return
	}
	writeJSON(w, http.StatusOK, forgeCheckToResponse(c))
}

func (h *Handler) DeleteForgeCheck(w http.ResponseWriter, r *http.Request) {
	c, ok := h.loadForgeCheck(w, r)
	if !ok {
		return
	}
	if err := h.Queries.DeleteForgeCheck(r.Context(), db.DeleteForgeCheckParams{ID: c.ID, WorkspaceID: c.WorkspaceID}); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to delete check")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
```

- [ ] **Step 2: 注册路由**（workspace-scoped 组，`/api/forge/standards` 旁）

```go
r.Route("/api/forge/checks", func(r chi.Router) {
	r.Get("/", h.ListForgeChecks)
	r.Post("/", h.CreateForgeCheck)
	r.Route("/{id}", func(r chi.Router) {
		r.Get("/", func(w http.ResponseWriter, r *http.Request) {
			if c, ok := h.loadForgeCheck(w, r); ok {
				writeJSON(w, http.StatusOK, forgeCheckToResponse(c))
			}
		})
		r.Put("/", h.UpdateForgeCheck)
		r.Delete("/", h.DeleteForgeCheck)
	})
})
```

- [ ] **Step 3: 编译 + commit**

Run: `wsl -d Ubuntu -- bash -lc "cd /mnt/d/shulex_work/forge/server && go build ./... 2>&1 | tail -8"`
Expected: 通过。
```bash
git add server/internal/handler/forge_checks.go server/cmd/server/router.go
git commit -m "feat(forge): checks CRUD API + routes"
```

---

### Task 3.2: 前端（镜像 forge-standards）

**Files:**
- Create: `packages/core/types/forge-check.ts`（`ForgeCheck` {id, workspace_id, project_id?, name, command, enabled} + `ForgeCheckInput`）
- Modify: `packages/core/types/index.ts`（加 `export type { ForgeCheck, ForgeCheckInput } from "./forge-check";`）
- Modify: `packages/core/api/client.ts`（import + `listForgeChecks/createForgeCheck/updateForgeCheck/deleteForgeCheck` 方法，镜像 forge standards 方法，路径 `/api/forge/checks`）
- Modify: `packages/core/workspace/queries.ts`（`forgeChecks` key + `forgeCheckListOptions`）
- Create: `packages/views/forge-checks/forge-checks-page.tsx`（列表 + name/command 表单，镜像 forge-standards-page.tsx，去掉 core/detail/tags，改成单 command Textarea）
- Create: `packages/views/forge-checks/index.ts`（`export { ForgeChecksPage } from "./forge-checks-page";`）
- Modify: `packages/views/package.json`（exports 加 `"./forge-checks": "./forge-checks/index.ts"`）
- Create: `apps/web/app/[workspaceSlug]/(dashboard)/forge-checks/page.tsx`（`export { ForgeChecksPage as default } from "@multica/views/forge-checks";`）

- [ ] **Step 1: 照 F1 forge-standards 的八处接线一一对应**

逐文件对照 F1（`git show b09f2ff4 --stat` 看清单），把 standard→check、core_content/detail_content/
profile_tags 三字段→单 command 字段。view 表单：name（Input）+ command（Textarea）+ scope。

- [ ] **Step 2: typecheck**

Run（Windows，node_modules 已装）:
```powershell
cd D:\shulex_work\forge; corepack pnpm --filter "@multica/core" --filter "@multica/views" --filter "@multica/web" typecheck 2>&1 | Select-Object -Last 20
```
Expected: 三包 Done。

- [ ] **Step 3: Commit**

```bash
git add packages/core/types/forge-check.ts packages/core/types/index.ts packages/core/api/client.ts packages/core/workspace/queries.ts packages/views/forge-checks/ packages/views/package.json "apps/web/app/[workspaceSlug]/(dashboard)/forge-checks/page.tsx"
git commit -m "feat(forge): checks UI — types, api client, query options, list+editor, web route"
```

---

## Phase 3 完成检查
- [ ] checks CRUD handler + 路由编译通过
- [ ] 前端三包 typecheck 绿
