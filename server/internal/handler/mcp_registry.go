package handler

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/multica-ai/multica/server/internal/mcpresolve"
	"github.com/multica-ai/multica/server/internal/nacos"
)

// Forge iris (N1): the MCP server catalog. Read endpoints are workspace-member
// gated; writes (register / lifecycle) are owner/admin only. The catalog lives
// in Nacos (source of truth) and stores SHAPE ONLY — env_keys / header_keys name
// the secrets an agent must supply via custom_env; values never touch Nacos.
//
// Namespacing: every workspace reads its own namespace plus the cross-workspace
// "shared" namespace. A caller can only ever address those two; any other
// namespaceId is rejected so a member cannot read or write another workspace's
// catalog by guessing its id.
const sharedMCPNamespace = "shared"

// namespaceAllowed reports whether ns is addressable by a caller scoped to wsID:
// the workspace's own namespace or the shared one.
func namespaceAllowed(ns, wsID string) bool {
	return ns == wsID || ns == sharedMCPNamespace
}

// nacosReady writes 503 and returns false when no Nacos adapter is wired
// (NACOS_SERVER_ADDR unset). Keeps every catalog endpoint from nil-panicking
// on deployments that don't run Nacos.
func (h *Handler) nacosReady(w http.ResponseWriter) bool {
	if h.Nacos == nil {
		writeError(w, http.StatusServiceUnavailable, "mcp registry not configured")
		return false
	}
	return true
}

// resolveAgentMcpConfig returns the effective mcp_config handed to the daemon
// for a claimed task. When the agent references catalog MCP servers (mcp_refs),
// they are resolved against Nacos and merged with the agent's inline config,
// with secret VALUES injected from custom_env (the catalog stores shape only).
//
// It is a pure no-op — returning inlineMcpConfig untouched — when the agent has
// no mcp_refs OR no Nacos adapter is wired. That makes dispatch byte-identical
// to pre-iris behavior for every agent that doesn't opt into the catalog, and
// keeps deployments without Nacos completely unaffected. A Nacos outage is
// already absorbed by CachedQuerier (degrades to last-known shape); a hard
// resolve error falls back to the inline config rather than blocking dispatch.
func (h *Handler) resolveAgentMcpConfig(ctx context.Context, agentID string, mcpRefs []byte, inlineMcpConfig json.RawMessage, customEnv map[string]string) json.RawMessage {
	if h.Nacos == nil || len(mcpRefs) == 0 {
		return inlineMcpConfig
	}
	var refs []nacos.MCPRef
	if err := json.Unmarshal(mcpRefs, &refs); err != nil || len(refs) == 0 {
		return inlineMcpConfig
	}
	effective, warns, err := mcpresolve.ResolveMCP(ctx, h.Nacos, mcpresolve.Input{
		Refs:      refs,
		InlineMCP: inlineMcpConfig,
		Secrets:   mcpresolve.MapSecrets(customEnv),
	})
	for _, msg := range warns {
		slog.Warn("mcp resolve", "agent_id", agentID, "warning", msg)
	}
	if err != nil {
		return inlineMcpConfig
	}
	return effective
}

// ListMCPCatalog handles GET /api/mcp-registry/servers — lists the workspace
// namespace and the shared namespace, merged. Degrades to a partial/empty list
// if Nacos is unreachable rather than failing the request.
func (h *Handler) ListMCPCatalog(w http.ResponseWriter, r *http.Request) {
	wsID := h.resolveWorkspaceID(r)
	if wsID == "" {
		writeError(w, http.StatusBadRequest, "workspace_id is required")
		return
	}
	if _, ok := h.workspaceMember(w, r, wsID); !ok {
		return
	}
	if !h.nacosReady(w) {
		return
	}
	out := []nacos.MCPServerShape{}
	for _, ns := range []string{wsID, sharedMCPNamespace} {
		list, err := h.Nacos.ListMCPServers(r.Context(), ns)
		if err != nil {
			continue // degrade: a down namespace contributes nothing, never 500s
		}
		out = append(out, list...)
	}
	writeJSON(w, http.StatusOK, map[string]any{"servers": out})
}

// GetMCPCatalogServer handles GET /api/mcp-registry/servers/{name}.
func (h *Handler) GetMCPCatalogServer(w http.ResponseWriter, r *http.Request) {
	wsID := h.resolveWorkspaceID(r)
	if wsID == "" {
		writeError(w, http.StatusBadRequest, "workspace_id is required")
		return
	}
	if _, ok := h.workspaceMember(w, r, wsID); !ok {
		return
	}
	if !h.nacosReady(w) {
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
	s, err := h.Nacos.GetMCPServer(r.Context(), ns, name, r.URL.Query().Get("ref"))
	if err != nil {
		writeError(w, http.StatusNotFound, "mcp server not found")
		return
	}
	writeJSON(w, http.StatusOK, s)
}

type registerMCPRequest struct {
	Namespace string               `json:"namespace"`
	Server    nacos.MCPServerShape `json:"server"`
}

// RegisterMCPCatalogServer handles POST /api/mcp-registry/servers — owner/admin.
func (h *Handler) RegisterMCPCatalogServer(w http.ResponseWriter, r *http.Request) {
	wsID := h.resolveWorkspaceID(r)
	if wsID == "" {
		writeError(w, http.StatusBadRequest, "workspace_id is required")
		return
	}
	if _, ok := h.requireWorkspaceRole(w, r, wsID, "workspace not found", "owner", "admin"); !ok {
		return
	}
	if !h.nacosReady(w) {
		return
	}
	var req registerMCPRequest
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
	if req.Server.Name == "" || req.Server.Version == "" {
		writeError(w, http.StatusBadRequest, "server name and version are required")
		return
	}
	if err := h.Nacos.RegisterMCPServer(r.Context(), ns, req.Server); err != nil {
		writeError(w, http.StatusBadGateway, "nacos unavailable")
		return
	}
	writeJSON(w, http.StatusCreated, map[string]string{"status": "registered"})
}

type lifecycleRequest struct {
	Version   string `json:"version"`
	Lifecycle string `json:"lifecycle"`
}

// SetMCPCatalogLifecycle handles PUT /api/mcp-registry/servers/{name}/lifecycle
// — owner/admin. Flips a cataloged version between published and offline.
func (h *Handler) SetMCPCatalogLifecycle(w http.ResponseWriter, r *http.Request) {
	wsID := h.resolveWorkspaceID(r)
	if wsID == "" {
		writeError(w, http.StatusBadRequest, "workspace_id is required")
		return
	}
	if _, ok := h.requireWorkspaceRole(w, r, wsID, "workspace not found", "owner", "admin"); !ok {
		return
	}
	if !h.nacosReady(w) {
		return
	}
	name := chi.URLParam(r, "name")
	var req lifecycleRequest
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
	if err := h.Nacos.SetMCPLifecycle(r.Context(), ns, name, req.Version, req.Lifecycle); err != nil {
		writeError(w, http.StatusBadGateway, "nacos unavailable")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
