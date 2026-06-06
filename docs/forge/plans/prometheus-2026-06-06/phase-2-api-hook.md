# Phase 2 — `/api/llm-providers/*` + 派发解析钩子 + agent provider_ref API

依赖:Phase 0(适配)、Phase 1(resolver)。产出:provider 目录 API(workspace 鉴权 + owner/admin)
+ claim 处 `resolveAgentProvider` 把 provider_ref 合并进 `custom_env`/`custom_args`,daemon 零改动。
镜像 N1 的 `mcp_registry.go` + `daemon.go` 钩子。复用同包已有的 `namespaceAllowed` / `sharedMCPNamespace`。

---

### Task 2.1: provider 目录 handler

**Files:**
- Create: `server/internal/handler/llm_registry.go` + `llm_registry_test.go`
- Modify: `server/internal/handler/handler.go`(Handler 加 `Providers nacos.ProviderQuerier` 字段)
- Modify: `server/cmd/server/router.go`(注册路由 + `NACOS_SERVER_ADDR` 时装配)

- [ ] **Step 1: handler**(对齐 `mcp_registry.go`:`resolveWorkspaceID` + `workspaceMember`;
  注册/lifecycle 走 `requireWorkspaceRole(...,"owner","admin")`)

```go
// server/internal/handler/llm_registry.go
package handler

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/multica-ai/multica/server/internal/nacos"
)

// providersReady writes 503 + returns false when no provider adapter is wired.
func (h *Handler) providersReady(w http.ResponseWriter) bool {
	if h.Providers == nil {
		writeError(w, http.StatusServiceUnavailable, "llm provider registry not configured")
		return false
	}
	return true
}

// GET /api/llm-providers — workspace ns + shared, merged + namespace-tagged.
func (h *Handler) ListLLMProviders(w http.ResponseWriter, r *http.Request) {
	wsID := h.resolveWorkspaceID(r)
	if wsID == "" {
		writeError(w, http.StatusBadRequest, "workspace_id is required")
		return
	}
	if _, ok := h.workspaceMember(w, r, wsID); !ok {
		return
	}
	if !h.providersReady(w) {
		return
	}
	out := []nacos.ProviderShape{}
	for _, ns := range []string{wsID, sharedMCPNamespace} {
		list, err := h.Providers.ListProviders(r.Context(), ns)
		if err != nil {
			continue // degrade
		}
		for i := range list {
			list[i].Namespace = ns
		}
		out = append(out, list...)
	}
	writeJSON(w, http.StatusOK, map[string]any{"providers": out})
}

// GET /api/llm-providers/{name}
func (h *Handler) GetLLMProvider(w http.ResponseWriter, r *http.Request) {
	wsID := h.resolveWorkspaceID(r)
	if wsID == "" {
		writeError(w, http.StatusBadRequest, "workspace_id is required")
		return
	}
	if _, ok := h.workspaceMember(w, r, wsID); !ok {
		return
	}
	if !h.providersReady(w) {
		return
	}
	name := chi.URLParam(r, "name")
	ns := r.URL.Query().Get("namespace")
	if ns == "" {
		ns = wsID
	}
	if !namespaceAllowed(ns, wsID) {
		writeError(w, http.StatusForbidden, "namespace not allowed")
		return
	}
	p, err := h.Providers.GetProvider(r.Context(), ns, name, r.URL.Query().Get("ref"))
	if err != nil {
		writeError(w, http.StatusNotFound, "provider not found")
		return
	}
	writeJSON(w, http.StatusOK, p)
}

type registerProviderRequest struct {
	Namespace string              `json:"namespace"`
	Provider  nacos.ProviderShape `json:"provider"`
}

// POST /api/llm-providers — owner/admin.
func (h *Handler) RegisterLLMProvider(w http.ResponseWriter, r *http.Request) {
	wsID := h.resolveWorkspaceID(r)
	if wsID == "" {
		writeError(w, http.StatusBadRequest, "workspace_id is required")
		return
	}
	if _, ok := h.requireWorkspaceRole(w, r, wsID, "workspace not found", "owner", "admin"); !ok {
		return
	}
	if !h.providersReady(w) {
		return
	}
	var req registerProviderRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	ns := req.Namespace
	if ns == "" {
		ns = wsID
	}
	if !namespaceAllowed(ns, wsID) {
		writeError(w, http.StatusForbidden, "namespace not allowed")
		return
	}
	if req.Provider.Name == "" || req.Provider.Version == "" || req.Provider.Protocol == "" {
		writeError(w, http.StatusBadRequest, "provider name, version and protocol are required")
		return
	}
	if err := h.Providers.RegisterProvider(r.Context(), ns, req.Provider); err != nil {
		writeError(w, http.StatusBadGateway, "nacos unavailable")
		return
	}
	writeJSON(w, http.StatusCreated, map[string]string{"status": "registered"})
}

type providerLifecycleRequest struct {
	Version   string `json:"version"`
	Lifecycle string `json:"lifecycle"`
}

// PUT /api/llm-providers/{name}/lifecycle — owner/admin.
func (h *Handler) SetLLMProviderLifecycle(w http.ResponseWriter, r *http.Request) {
	wsID := h.resolveWorkspaceID(r)
	if wsID == "" {
		writeError(w, http.StatusBadRequest, "workspace_id is required")
		return
	}
	if _, ok := h.requireWorkspaceRole(w, r, wsID, "workspace not found", "owner", "admin"); !ok {
		return
	}
	if !h.providersReady(w) {
		return
	}
	name := chi.URLParam(r, "name")
	var req providerLifecycleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	ns := r.URL.Query().Get("namespace")
	if ns == "" {
		ns = wsID
	}
	if !namespaceAllowed(ns, wsID) {
		writeError(w, http.StatusForbidden, "namespace not allowed")
		return
	}
	if err := h.Providers.SetProviderLifecycle(r.Context(), ns, name, req.Version, req.Lifecycle); err != nil {
		writeError(w, http.StatusBadGateway, "nacos unavailable")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
```

- [ ] **Step 2: Handler 字段**(`handler.go`,`Nacos` 字段旁加)

```go
	// Providers backs the Forge LLM-provider catalog (/api/llm-providers/*) and
	// the dispatch-time provider_ref resolver. nil when NACOS_SERVER_ADDR unset.
	Providers nacos.ProviderQuerier
```

- [ ] **Step 3: 路由 + 装配**(`router.go`,在 `h.Nacos` 装配旁)

```go
// router.go — 认证后的 /api 子路由里(挨着 /api/mcp-registry/servers):
r.Route("/api/llm-providers", func(r chi.Router) {
	r.Get("/", h.ListLLMProviders)
	r.Post("/", h.RegisterLLMProvider)
	r.Get("/{name}", h.GetLLMProvider)
	r.Put("/{name}/lifecycle", h.SetLLMProviderLifecycle)
})
// 装配(NACOS_SERVER_ADDR 存在的 if 块里,h.Nacos 赋值之后):
h.Providers = nacos.NewCachedProviderQuerier(nacos.NewProviderClient(addr, idKey, idVal))
```

- [ ] **Step 4: handler 测**(`llm_registry_test.go`,镜像 `mcp_registry_test.go`:`fakeProviderCatalog`
  + `withFakeProviders` 存取 `testHandler.Providers`)。覆盖:list 合并+标注 ns 200 / 非成员 404 /
  未配置(`Providers=nil`)503 / 非 owner 注册 403 / 缺字段 400 / 越界 namespace 403。

```go
// 关键脚手架(其余同 mcp_registry_test.go)
type fakeProviderCatalog struct{ items map[string]nacos.ProviderShape }
func pk(ns, name string) string { return ns + "|" + name }
func (f *fakeProviderCatalog) ListProviders(_ context.Context, ns string) ([]nacos.ProviderShape, error) {
	var out []nacos.ProviderShape
	for k, p := range f.items { if strings.HasPrefix(k, ns+"|") { out = append(out, p) } }
	return out, nil
}
func (f *fakeProviderCatalog) GetProvider(_ context.Context, ns, name, _ string) (nacos.ProviderShape, error) {
	p, ok := f.items[pk(ns, name)]; if !ok { return nacos.ProviderShape{}, errors.New("nf") }; return p, nil
}
func (f *fakeProviderCatalog) RegisterProvider(_ context.Context, ns string, p nacos.ProviderShape) error {
	if f.items == nil { f.items = map[string]nacos.ProviderShape{} }; f.items[pk(ns, p.Name)] = p; return nil
}
func (f *fakeProviderCatalog) SetProviderLifecycle(context.Context, string, string, string, string) error { return nil }
func withFakeProviders(t *testing.T, f nacos.ProviderQuerier) {
	t.Helper(); prev := testHandler.Providers; testHandler.Providers = f
	t.Cleanup(func() { testHandler.Providers = prev })
}
```

- [ ] **Step 5: `CGO_ENABLED=0 go build ./... && go test ./internal/handler/ -run LLMProvider`**(一次性 PG,同 N1)→ 绿。
- [ ] **Step 6: Commit** — `git commit -am "feat(llm-registry): provider catalog API (list/get/register/lifecycle) + auth"`

---

### Task 2.2: `resolveAgentProvider` 钩子 + agent provider_ref API

**Files:**
- Modify: `server/internal/handler/llm_registry.go`(加 `resolveAgentProvider` 方法 + 测)
- Modify: `server/internal/handler/daemon.go`(claim 处调钩子)
- Modify: `server/internal/handler/agent.go`(AgentResponse + agentToResponse + UpdateAgent + 请求结构)

- [ ] **Step 1: 钩子方法**(`llm_registry.go`)

```go
// resolveAgentProvider merges the agent's provider_ref resolution into its
// custom_env/custom_args at dispatch. No-op when no Nacos / no provider_ref.
// Precedence (escape hatch): agent's explicit custom_env wins on key conflict;
// provider args first then agent args (Codex -c later wins); agent.model wins,
// else the provider default.
func (h *Handler) resolveAgentProvider(
	ctx context.Context, agentID string, providerRef []byte,
	customEnv map[string]string, customArgs []string, model string,
) (map[string]string, []string, string) {
	if h.Providers == nil || len(providerRef) == 0 || bytes.Equal(bytes.TrimSpace(providerRef), []byte("null")) {
		return customEnv, customArgs, model
	}
	var pref nacos.ProviderRef
	if err := json.Unmarshal(providerRef, &pref); err != nil || pref.Name == "" {
		return customEnv, customArgs, model
	}
	res, warns, err := providerresolve.Resolve(ctx, h.Providers, providerresolve.Input{
		Ref: &pref, Secrets: providerresolve.MapSecrets(customEnv), Model: model,
	})
	for _, msg := range warns {
		slog.Warn("provider resolve", "agent_id", agentID, "warning", msg)
	}
	if err != nil {
		return customEnv, customArgs, model
	}
	// env: provider first, agent explicit wins.
	mergedEnv := map[string]string{}
	for k, v := range res.Env {
		mergedEnv[k] = v
	}
	for k, v := range customEnv {
		mergedEnv[k] = v
	}
	// args: provider first, agent after.
	mergedArgs := append(append([]string{}, res.Args...), customArgs...)
	resolvedModel := model
	if resolvedModel == "" {
		resolvedModel = res.Model
	}
	return mergedEnv, mergedArgs, resolvedModel
}
```
(imports 在 `llm_registry.go` 顶部加 `bytes`、`context`、`log/slog`、`providerresolve`。)

- [ ] **Step 2: claim 调用点**(`daemon.go` `ClaimTaskByRuntime`,在 `resolveAgentMcpConfig` 之后、
  装 `TaskAgentData` 之前;`model` 来自 `agent.Model.String`)

```go
customEnv, customArgs, agentModel := h.resolveAgentProvider(
	r.Context(), uuidToString(agent.ID), agent.ProviderRef, customEnv, customArgs, agent.Model.String)
// 用 customEnv / customArgs / agentModel 装 TaskAgentData(Model 字段用 agentModel)
```

- [ ] **Step 3: agent provider_ref API**(同 N1 mcp_refs 做法)
  - `AgentResponse` 加 `ProviderRef json.RawMessage \`json:"provider_ref"\``;`agentToResponse` 映射
    (`a.ProviderRef` 为空 → `json.RawMessage("null")`,**非密、不脱敏**)。
  - `updateAgentRequest` 加 `ProviderRef *json.RawMessage \`json:"provider_ref"\``;`UpdateAgent` 经
    rawFields:`null` → 清空(`params.ProviderRef` 不设 = COALESCE 不动?**注意**:provider_ref 需支持
    清空,用专门处理——`rawProviderRef, ok := rawFields["provider_ref"]; if ok { params.ProviderRef = append([]byte(nil), rawProviderRef...) }`,`null` 字面量原样写入即清空)。
  - `CreateAgentParams` 无需带(创建默认 NULL)。

- [ ] **Step 4: 钩子单测**(`llm_registry_test.go`)

```go
func TestResolveAgentProvider_MergesEnvAgentWins(t *testing.T) {
	withFakeProviders(t, &fakeProviderCatalog{items: map[string]nacos.ProviderShape{
		pk(testWorkspaceID, "router"): {Name: "router", Version: "1", Protocol: "anthropic",
			BaseURL: "https://catalog", AuthKey: "ROUTER_API_KEY", Lifecycle: "published"},
	}})
	pref, _ := json.Marshal(nacos.ProviderRef{Namespace: testWorkspaceID, Name: "router", Ref: "stable"})
	env, _, _ := testHandler.resolveAgentProvider(context.Background(), "a1", pref,
		map[string]string{"ROUTER_API_KEY": "sek", "ANTHROPIC_BASE_URL": "https://override"}, nil, "")
	if env["ANTHROPIC_AUTH_TOKEN"] != "sek" {
		t.Fatalf("secret inject: %v", env)
	}
	if env["ANTHROPIC_BASE_URL"] != "https://override" {
		t.Fatalf("agent explicit env must win: %v", env["ANTHROPIC_BASE_URL"])
	}
}

func TestResolveAgentProvider_NilRefUnchanged(t *testing.T) {
	withFakeProviders(t, &fakeProviderCatalog{})
	env, args, model := testHandler.resolveAgentProvider(context.Background(), "a1", []byte("null"),
		map[string]string{"X": "1"}, []string{"-a"}, "m")
	if env["X"] != "1" || len(args) != 1 || model != "m" {
		t.Fatalf("null provider_ref must be unchanged: %v %v %v", env, args, model)
	}
}
```

- [ ] **Step 5: `go build ./... && go test ./internal/handler/ -run 'LLMProvider|ResolveAgentProvider|UpdateAgent'`**(一次性 PG)→ 绿。
- [ ] **Step 6: Commit** — `git commit -am "feat(claim): resolve provider_ref -> merged env/args at dispatch (daemon unchanged)"`
