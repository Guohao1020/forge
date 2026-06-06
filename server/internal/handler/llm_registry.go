package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/multica-ai/multica/server/internal/nacos"
	"github.com/multica-ai/multica/server/internal/providerresolve"
)

// Forge prometheus (N2): the LLM-provider catalog. Mirrors mcp_registry.go.
// Read endpoints are workspace-member gated; writes (register / lifecycle) are
// owner/admin only. The catalog lives in Nacos (source of truth) and stores
// SHAPE ONLY — auth_key NAMES the secret an agent must supply via custom_env;
// the value never touches Nacos. Namespacing reuses the same wsID + "shared"
// rule as the MCP catalog (see namespaceAllowed / sharedMCPNamespace), so a
// caller can only ever address its own workspace's providers and the shared set.

// providersReady writes 503 and returns false when no provider adapter is wired
// (NACOS_SERVER_ADDR unset). Keeps every provider endpoint from nil-panicking
// on deployments that don't run Nacos.
func (h *Handler) providersReady(w http.ResponseWriter) bool {
	if h.Providers == nil {
		writeError(w, http.StatusServiceUnavailable, "llm provider registry not configured")
		return false
	}
	return true
}

// ListLLMProviders handles GET /api/llm-providers — lists the workspace
// namespace and the shared namespace, merged. Degrades to a partial/empty list
// if Nacos is unreachable rather than failing the request.
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
			continue // degrade: a down namespace contributes nothing, never 500s
		}
		for i := range list {
			list[i].Namespace = ns // tag origin so the picker can build a ProviderRef
		}
		out = append(out, list...)
	}
	writeJSON(w, http.StatusOK, map[string]any{"providers": out})
}

// GetLLMProvider handles GET /api/llm-providers/{name}.
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

// RegisterLLMProvider handles POST /api/llm-providers — owner/admin.
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

// SetLLMProviderLifecycle handles PUT /api/llm-providers/{name}/lifecycle —
// owner/admin. Flips a cataloged provider version between published and offline.
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

// resolveAgentProviderRef merges the agent's provider_ref resolution into its
// custom_env / custom_args at dispatch time. It is a pure no-op — returning the
// inputs untouched — when the agent has no provider_ref OR no Nacos adapter is
// wired, so dispatch stays byte-identical to pre-prometheus behavior for every
// agent that doesn't opt into the provider catalog.
//
// Precedence (the agent's explicit config is the escape hatch):
//   - env: provider-resolved env is applied first, then the agent's explicit
//     custom_env overwrites on key conflict — an operator can always pin a
//     base URL / token by hand.
//   - args: provider args come first, the agent's custom_args after (so a
//     later Codex `-c` override from the agent wins).
//   - model: the agent's model wins; the provider default fills an empty model.
//
// A resolve error or unknown protocol is absorbed by providerresolve (warns +
// empty Result), and a hard error here falls back to the unchanged inputs
// rather than blocking dispatch. Named with a -Ref suffix to avoid colliding
// with the existing resolveAgentProvider (runtime-provider lookup for
// thinking_level validation) in agent.go.
func (h *Handler) resolveAgentProviderRef(
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
		Ref:     &pref,
		Secrets: providerresolve.MapSecrets(customEnv),
		Model:   model,
	})
	for _, msg := range warns {
		slog.Warn("provider resolve", "agent_id", agentID, "warning", msg)
	}
	if err != nil {
		return customEnv, customArgs, model
	}
	mergedEnv := map[string]string{}
	for k, v := range res.Env {
		mergedEnv[k] = v
	}
	for k, v := range customEnv { // agent explicit env wins (escape hatch)
		mergedEnv[k] = v
	}
	mergedArgs := append(append([]string{}, res.Args...), customArgs...)
	resolvedModel := model
	if resolvedModel == "" {
		resolvedModel = res.Model
	}
	return mergedEnv, mergedArgs, resolvedModel
}
