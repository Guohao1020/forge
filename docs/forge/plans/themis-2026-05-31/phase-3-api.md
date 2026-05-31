## Phase 3 — API（Standards CRUD + project profile）

**Goal:** `/api/forge/standards` CRUD + `/api/forge/projects/{id}/profile` GET/PUT，沿用
Multica 的 handler/sqlc/路由模式 + workspace 隔离。

**Depends-on:** Phase 0（sqlc 方法）　**Unblocks:** Phase 4
**Completion gate:** create→list 往返测试通过；`go build ./...` 通过；路由注册。

---

### Task 3.1: 请求/响应类型 + handler

**Files:**
- Create: `server/internal/handler/forge_standards.go`

- [ ] **Step 1: 写 handler 文件**

`server/internal/handler/forge_standards.go`：
```go
package handler

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/middleware"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

type ForgeStandardRequest struct {
	ProjectID     string   `json:"project_id,omitempty"` // empty = workspace-level
	Name          string   `json:"name"`
	Category      string   `json:"category"`
	ProfileTags   []string `json:"profile_tags"`
	CoreContent   string   `json:"core_content"`
	DetailContent string   `json:"detail_content"`
	Enabled       *bool    `json:"enabled,omitempty"`
}

type ForgeStandardResponse struct {
	ID            string   `json:"id"`
	WorkspaceID   string   `json:"workspace_id"`
	ProjectID     string   `json:"project_id,omitempty"`
	Name          string   `json:"name"`
	Category      string   `json:"category"`
	ProfileTags   []string `json:"profile_tags"`
	CoreContent   string   `json:"core_content"`
	DetailContent string   `json:"detail_content"`
	Enabled       bool     `json:"enabled"`
}

func forgeStandardToResponse(s db.ForgeStandard) ForgeStandardResponse {
	r := ForgeStandardResponse{
		ID: uuidToString(s.ID), WorkspaceID: uuidToString(s.WorkspaceID),
		Name: s.Name, Category: s.Category, ProfileTags: s.ProfileTags,
		CoreContent: s.CoreContent, DetailContent: s.DetailContent, Enabled: s.Enabled,
	}
	if s.ProjectID.Valid {
		r.ProjectID = uuidToString(s.ProjectID)
	}
	return r
}

func (h *Handler) ListForgeStandards(w http.ResponseWriter, r *http.Request) {
	wsID := h.resolveWorkspaceID(r)
	if wsID == "" {
		writeError(w, http.StatusBadRequest, "workspace_id is required")
		return
	}
	rows, err := h.Queries.ListStandardsByWorkspace(r.Context(), parseUUID(wsID))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list standards")
		return
	}
	out := make([]ForgeStandardResponse, len(rows))
	for i, s := range rows {
		out[i] = forgeStandardToResponse(s)
	}
	writeJSON(w, http.StatusOK, out)
}

func (h *Handler) CreateForgeStandard(w http.ResponseWriter, r *http.Request) {
	wsID := h.resolveWorkspaceID(r)
	if wsID == "" {
		writeError(w, http.StatusBadRequest, "workspace_id is required")
		return
	}
	userID, ok := requireUserID(w, r)
	if !ok {
		return
	}
	var req ForgeStandardRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Name == "" || req.Category == "" {
		writeError(w, http.StatusBadRequest, "name and category are required")
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
	if req.ProfileTags == nil {
		req.ProfileTags = []string{}
	}
	s, err := h.Queries.CreateForgeStandard(r.Context(), db.CreateForgeStandardParams{
		WorkspaceID: parseUUID(wsID), ProjectID: projID, Name: req.Name, Category: req.Category,
		ProfileTags: req.ProfileTags, CoreContent: req.CoreContent, DetailContent: req.DetailContent,
		Enabled: enabled, CreatedBy: parseUUID(userID),
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create standard")
		return
	}
	writeJSON(w, http.StatusCreated, forgeStandardToResponse(s))
}

func (h *Handler) loadForgeStandard(w http.ResponseWriter, r *http.Request) (db.ForgeStandard, bool) {
	wsID := h.resolveWorkspaceID(r)
	id, ok := parseUUIDOrBadRequest(w, chi.URLParam(r, "id"), "standard id")
	if !ok {
		return db.ForgeStandard{}, false
	}
	s, err := h.Queries.GetForgeStandard(r.Context(), db.GetForgeStandardParams{ID: id, WorkspaceID: parseUUID(wsID)})
	if err != nil {
		writeError(w, http.StatusNotFound, "standard not found")
		return db.ForgeStandard{}, false
	}
	return s, true
}

func (h *Handler) GetForgeStandard(w http.ResponseWriter, r *http.Request) {
	s, ok := h.loadForgeStandard(w, r)
	if !ok {
		return
	}
	writeJSON(w, http.StatusOK, forgeStandardToResponse(s))
}

func (h *Handler) UpdateForgeStandard(w http.ResponseWriter, r *http.Request) {
	existing, ok := h.loadForgeStandard(w, r)
	if !ok {
		return
	}
	var req ForgeStandardRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	params := db.UpdateForgeStandardParams{ID: existing.ID, WorkspaceID: existing.WorkspaceID}
	if req.Name != "" {
		params.Name = pgtype.Text{String: req.Name, Valid: true}
	}
	if req.Category != "" {
		params.Category = pgtype.Text{String: req.Category, Valid: true}
	}
	if req.ProfileTags != nil {
		params.ProfileTags = req.ProfileTags
	}
	params.CoreContent = pgtype.Text{String: req.CoreContent, Valid: true}
	params.DetailContent = pgtype.Text{String: req.DetailContent, Valid: true}
	if req.Enabled != nil {
		params.Enabled = pgtype.Bool{Bool: *req.Enabled, Valid: true}
	}
	s, err := h.Queries.UpdateForgeStandard(r.Context(), params)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update standard")
		return
	}
	writeJSON(w, http.StatusOK, forgeStandardToResponse(s))
}

func (h *Handler) DeleteForgeStandard(w http.ResponseWriter, r *http.Request) {
	s, ok := h.loadForgeStandard(w, r)
	if !ok {
		return
	}
	if err := h.Queries.DeleteForgeStandard(r.Context(), db.DeleteForgeStandardParams{ID: s.ID, WorkspaceID: s.WorkspaceID}); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to delete standard")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ---- project profile ----

type ForgeProfileRequest struct {
	Tags []string `json:"tags"`
}

func (h *Handler) GetForgeProjectProfile(w http.ResponseWriter, r *http.Request) {
	projID, ok := parseUUIDOrBadRequest(w, chi.URLParam(r, "id"), "project id")
	if !ok {
		return
	}
	prof, err := h.Queries.GetForgeProjectProfile(r.Context(), projID)
	if err != nil {
		writeJSON(w, http.StatusOK, ForgeProfileRequest{Tags: []string{}}) // none yet
		return
	}
	writeJSON(w, http.StatusOK, ForgeProfileRequest{Tags: prof.Tags})
}

func (h *Handler) PutForgeProjectProfile(w http.ResponseWriter, r *http.Request) {
	projID, ok := parseUUIDOrBadRequest(w, chi.URLParam(r, "id"), "project id")
	if !ok {
		return
	}
	var req ForgeProfileRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Tags == nil {
		req.Tags = []string{}
	}
	prof, err := h.Queries.UpsertForgeProjectProfile(r.Context(), db.UpsertForgeProjectProfileParams{ProjectID: projID, Tags: req.Tags})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to save profile")
		return
	}
	writeJSON(w, http.StatusOK, ForgeProfileRequest{Tags: prof.Tags})
}

var _ = middleware.ResolveWorkspaceIDFromRequest // ensure middleware import used
```
> 实现时核对生成的 params struct 字段名（sqlc 按列名生成，如 `CreateForgeStandardParams`、
> `UpdateForgeStandardParams`）。`UpdateForgeStandard` 用 `sqlc.narg` → 生成 `pgtype.Text`/
> `pgtype.Bool` 可空参数；若 `profile_tags` narg 生成为 `[]string`（非空切片即覆盖），按实际类型用。

- [ ] **Step 2: 编译验证**

Run: `cd <forge-repo>/server && go build ./... 2>&1 | tail -8`
Expected: 编译通过（按生成 params 字段名修正后）。

---

### Task 3.2: 注册路由

**Files:**
- Modify: `server/cmd/server/router.go`（workspace-scoped 组内，参照 `/api/skills` 块）

- [ ] **Step 1: 在 skills 路由块附近加 forge 路由**

```go
r.Route("/api/forge/standards", func(r chi.Router) {
	r.Get("/", h.ListForgeStandards)
	r.Post("/", h.CreateForgeStandard)
	r.Route("/{id}", func(r chi.Router) {
		r.Get("/", h.GetForgeStandard)
		r.Put("/", h.UpdateForgeStandard)
		r.Delete("/", h.DeleteForgeStandard)
	})
})
r.Route("/api/forge/projects/{id}/profile", func(r chi.Router) {
	r.Get("/", h.GetForgeProjectProfile)
	r.Put("/", h.PutForgeProjectProfile)
})
```
> 放进与 `/api/skills` 同一个经过 `RequireWorkspaceMember` 中间件的 group。

- [ ] **Step 2: 编译验证**

Run: `cd <forge-repo>/server && go build ./... 2>&1 | tail -5`
Expected: 通过。

- [ ] **Step 3: Commit**

```bash
git add server/internal/handler/forge_standards.go server/cmd/server/router.go
git commit -m "feat(forge): standards + project-profile CRUD API"
```

---

### Task 3.3: create→list 往返测试

**Files:**
- Create: `server/internal/handler/forge_standards_test.go`

- [ ] **Step 1: 写测试（沿用 Multica handler 测试的 test-DB fixture 模式）**

参照仓库现有 `*_test.go`（如 `skill` 的 handler 测试）建一个 workspace + user fixture，
然后：`POST /api/forge/standards`（category=api,name=rest,core="X"）→ 断言 201；
`GET /api/forge/standards` → 断言返回含该 standard。复用现有测试 helper（建 test request、
注入 workspace context、用真实测试 DB）。

> 具体 fixture/helper 名照搬同包现有 handler 测试；本步交付一个能 `go test ./internal/handler/ -run ForgeStandard` 通过的往返用例。

- [ ] **Step 2: 跑测试**

Run: `cd <forge-repo>/server && go test ./internal/handler/ -run ForgeStandard 2>&1 | tail -8`
Expected: PASS。

- [ ] **Step 3: Commit**

```bash
git add server/internal/handler/forge_standards_test.go
git commit -m "test(forge): standards API create→list round-trip"
```

---

## Phase 3 完成检查
- [ ] CRUD + profile handler 编译通过
- [ ] 路由注册在 workspace-scoped group
- [ ] create→list 往返测试通过
