# Phase 2 — MCP 目录 API + 派发解析钩子

依赖:Phase 0(适配)、Phase 1(resolver)。产出:`/api/mcp-registry/*`(workspace 鉴权 + owner/admin)
+ claim 处一行钩子把 `mcp_refs` 解析成有效 `mcp_config`,daemon 零改动。

---

### Task 2.1: MCP 目录 handler

**Files:**
- Create: `server/internal/handler/mcp_registry.go` + `mcp_registry_test.go`
- Modify: `server/internal/handler/handler.go`(Handler 结构加 `Nacos nacos.NacosQuerier` 字段)
- Modify: `server/cmd/server/router.go`(注册路由)
- Modify: `server/cmd/server/main.go`(装配:`nacos.NewCachedQuerier(nacos.NewClient(env...))` 注入 Handler)

- [ ] **Step 1: handler**(对齐既有 `forge_checks.go` 的鉴权姿态:`resolveWorkspaceID` + 成员校验;
  注册/lifecycle 走 owner/admin)

```go
// server/internal/handler/mcp_registry.go
package handler

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/multica-ai/multica/server/internal/nacos"
)

// GET /api/mcp-registry/servers — list catalog (workspace ns + shared).
func (h *Handler) ListMCPServers(w http.ResponseWriter, r *http.Request) {
	wsID, ok := h.resolveWorkspaceID(r)
	if !ok { writeError(w, http.StatusBadRequest, "workspace required"); return }
	if _, ok := h.workspaceMember(w, r, wsID); !ok { return }
	out := []nacos.MCPServerShape{}
	for _, ns := range []string{wsID, "shared"} {
		list, err := h.Nacos.ListMCPServers(r.Context(), ns)
		if err != nil { continue } // degrade: partial/empty on Nacos down
		out = append(out, list...)
	}
	writeJSON(w, http.StatusOK, map[string]any{"servers": out})
}

// GET /api/mcp-registry/servers/{name}
func (h *Handler) GetMCPServer(w http.ResponseWriter, r *http.Request) {
	wsID, ok := h.resolveWorkspaceID(r); if !ok { writeError(w, http.StatusBadRequest, "workspace required"); return }
	if _, ok := h.workspaceMember(w, r, wsID); !ok { return }
	name := chi.URLParam(r, "name")
	ns := r.URL.Query().Get("namespace"); if ns == "" { ns = wsID }
	if ns != wsID && ns != "shared" { writeError(w, http.StatusForbidden, "namespace not allowed"); return }
	s, err := h.Nacos.GetMCPServer(r.Context(), ns, name, r.URL.Query().Get("ref"))
	if err != nil { writeError(w, http.StatusNotFound, "not found"); return }
	writeJSON(w, http.StatusOK, s)
}

type registerMCPRequest struct {
	Namespace string                `json:"namespace"`
	Server    nacos.MCPServerShape  `json:"server"`
}

// POST /api/mcp-registry/servers — owner/admin only.
func (h *Handler) RegisterMCPServer(w http.ResponseWriter, r *http.Request) {
	wsID, ok := h.resolveWorkspaceID(r); if !ok { writeError(w, http.StatusBadRequest, "workspace required"); return }
	if _, ok := h.workspaceOwnerOrAdmin(w, r, wsID); !ok { return }
	var req registerMCPRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil { writeError(w, http.StatusBadRequest, "bad body"); return }
	ns := req.Namespace; if ns == "" { ns = wsID }
	if ns != wsID && ns != "shared" { writeError(w, http.StatusForbidden, "namespace not allowed"); return }
	if req.Server.Name == "" || req.Server.Version == "" { writeError(w, http.StatusBadRequest, "name+version required"); return }
	if err := h.Nacos.RegisterMCPServer(r.Context(), ns, req.Server); err != nil { writeError(w, http.StatusBadGateway, "nacos unavailable"); return }
	writeJSON(w, http.StatusCreated, map[string]string{"status": "registered"})
}

type lifecycleRequest struct{ Version, Lifecycle string }

// PUT /api/mcp-registry/servers/{name}/lifecycle — owner/admin only.
func (h *Handler) SetMCPLifecycle(w http.ResponseWriter, r *http.Request) {
	wsID, ok := h.resolveWorkspaceID(r); if !ok { writeError(w, http.StatusBadRequest, "workspace required"); return }
	if _, ok := h.workspaceOwnerOrAdmin(w, r, wsID); !ok { return }
	name := chi.URLParam(r, "name")
	var req lifecycleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil { writeError(w, http.StatusBadRequest, "bad body"); return }
	ns := r.URL.Query().Get("namespace"); if ns == "" { ns = wsID }
	if ns != wsID && ns != "shared" { writeError(w, http.StatusForbidden, "namespace not allowed"); return }
	if err := h.Nacos.SetMCPLifecycle(r.Context(), ns, name, req.Version, req.Lifecycle); err != nil { writeError(w, http.StatusBadGateway, "nacos unavailable"); return }
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
```

> `workspaceOwnerOrAdmin` 若不存在,沿用 F1/F4 加固用的 owner/admin 成员校验同款 helper(见
> `validateAutopilotAssignee` 邻近的成员角色检查);复用,勿新造。

- [ ] **Step 2: 路由 + 装配**

```go
// router.go,认证后的 /api 子路由里:
r.Get("/mcp-registry/servers", h.ListMCPServers)
r.Get("/mcp-registry/servers/{name}", h.GetMCPServer)
r.Post("/mcp-registry/servers", h.RegisterMCPServer)
r.Put("/mcp-registry/servers/{name}/lifecycle", h.SetMCPLifecycle)
// main.go: h.Nacos = nacos.NewCachedQuerier(nacos.NewClient(os.Getenv("NACOS_SERVER_ADDR"), os.Getenv("NACOS_USERNAME"), os.Getenv("NACOS_PASSWORD")))
```

- [ ] **Step 3: handler 测**(mock Nacos + 鉴权:非成员 401、非 owner 注册 403、坏 namespace 403、
  正常 list 200)。用既有 handler 测脚手架建 fixture workspace/member。

- [ ] **Step 4: `go build ./... && go test ./internal/handler/ -run MCP -v`** → 绿。
- [ ] **Step 5: Commit** — `git commit -am "feat(mcp-registry): catalog API (list/get/register/lifecycle) + auth"`

---

### Task 2.2: claim 处解析钩子(有效 mcp_config)

**Files:**
- Modify: 构建给 daemon 的 task/claim 响应处(设 `mcp_config` 的那一行;对齐 F1 claim 注入、
  F2 `GetForgeChecks` 的服务端解析位置)。grep `McpConfig` / `mcp_config` 在 `internal/handler` /
  `internal/service` 找到 task→daemon 的赋值点。

- [ ] **Step 1: 在该点调 resolver,替换/合并 mcp_config**

```go
// 伪代码,插在"把 agent.mcp_config 放进给 daemon 的 task 载荷"之前/处:
secrets := mcpresolve.MapSecrets(decodeCustomEnv(agent.CustomEnv)) // agent.custom_env -> map[string]string
var refs []nacos.MCPRef
_ = json.Unmarshal(agent.McpRefs, &refs)
effective, warns, err := mcpresolve.ResolveMCP(ctx, h.Nacos, mcpresolve.Input{
	Refs: refs, InlineMCP: agent.McpConfig, Secrets: secrets,
})
if err == nil && len(refs) > 0 {
	taskMcpConfig = effective      // 有效配置 = 解析自 Nacos + 注密 + 合并内联
}
for _, w := range warns { slog.Warn("mcp resolve", "warning", w) }
// refs 为空时保持原 agent.mcp_config 不变(完全向后兼容)
```

- [ ] **Step 2: 关键不变量**:`mcp_refs` 为空 → 行为与今天**完全一致**(用原 `mcp_config`);
  daemon/CLI 侧零改动(还是从同样字段读)。Nacos 解析失败 → 已被 `CachedQuerier` 降级吞掉,
  最坏是某 ref 被跳过 + warn,不阻塞派发。

- [ ] **Step 3: 服务层单测**:造一个带 `mcp_refs` + `custom_env` 的 agent + mock Nacos,断言
  task 载荷里的 `mcp_config` = 解析后的有效配置;`mcp_refs` 空时等于原 `mcp_config`。

- [ ] **Step 4: `go test ./internal/...`** 相关包绿。
- [ ] **Step 5: Commit** — `git commit -am "feat(claim): resolve mcp_refs -> effective mcp_config at dispatch (daemon unchanged)"`
