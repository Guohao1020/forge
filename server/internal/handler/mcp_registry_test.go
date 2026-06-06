package handler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/multica-ai/multica/server/internal/nacos"
)

// fakeCatalog is an in-memory NacosQuerier for handler tests — no live Nacos.
type fakeCatalog struct {
	servers map[string]nacos.MCPServerShape // key: ns|name
	listErr error
}

func nsKey(ns, name string) string { return ns + "|" + name }

func (f *fakeCatalog) ListMCPServers(_ context.Context, ns string) ([]nacos.MCPServerShape, error) {
	if f.listErr != nil {
		return nil, f.listErr
	}
	var out []nacos.MCPServerShape
	for k, s := range f.servers {
		if strings.HasPrefix(k, ns+"|") {
			out = append(out, s)
		}
	}
	return out, nil
}

func (f *fakeCatalog) GetMCPServer(_ context.Context, ns, name, _ string) (nacos.MCPServerShape, error) {
	s, ok := f.servers[nsKey(ns, name)]
	if !ok {
		return nacos.MCPServerShape{}, errors.New("not found")
	}
	return s, nil
}

func (f *fakeCatalog) RegisterMCPServer(_ context.Context, ns string, s nacos.MCPServerShape) error {
	if f.servers == nil {
		f.servers = map[string]nacos.MCPServerShape{}
	}
	f.servers[nsKey(ns, s.Name)] = s
	return nil
}

func (f *fakeCatalog) SetMCPLifecycle(_ context.Context, ns, name, _, lifecycle string) error {
	s, ok := f.servers[nsKey(ns, name)]
	if !ok {
		return errors.New("not found")
	}
	s.Lifecycle = lifecycle
	f.servers[nsKey(ns, name)] = s
	return nil
}

// withFakeNacos installs a fake catalog on the shared test handler for the
// duration of one test and restores the previous value afterward.
func withFakeNacos(t *testing.T, f nacos.NacosQuerier) {
	t.Helper()
	prev := testHandler.Nacos
	testHandler.Nacos = f
	t.Cleanup(func() { testHandler.Nacos = prev })
}

// createMCPTestMember inserts a user + member with the given role and returns
// the user id. Cleaned up at test end.
func createMCPTestMember(t *testing.T, role string) string {
	t.Helper()
	email := fmt.Sprintf("mcp-reg-%s-%d@multica.ai", role, time.Now().UnixNano())
	var userID string
	if err := testPool.QueryRow(context.Background(),
		`INSERT INTO "user" (name, email) VALUES ($1, $2) RETURNING id`,
		"MCP Reg "+role, email,
	).Scan(&userID); err != nil {
		t.Fatalf("create user: %v", err)
	}
	if _, err := testPool.Exec(context.Background(),
		`INSERT INTO member (workspace_id, user_id, role) VALUES ($1, $2, $3)`,
		testWorkspaceID, userID, role,
	); err != nil {
		t.Fatalf("create member: %v", err)
	}
	t.Cleanup(func() { testPool.Exec(context.Background(), `DELETE FROM "user" WHERE id = $1`, userID) })
	return userID
}

// createMCPNonMemberUser inserts a user with NO membership in the test workspace.
func createMCPNonMemberUser(t *testing.T) string {
	t.Helper()
	email := fmt.Sprintf("mcp-reg-nonmember-%d@multica.ai", time.Now().UnixNano())
	var userID string
	if err := testPool.QueryRow(context.Background(),
		`INSERT INTO "user" (name, email) VALUES ($1, $2) RETURNING id`,
		"MCP Reg NonMember", email,
	).Scan(&userID); err != nil {
		t.Fatalf("create user: %v", err)
	}
	t.Cleanup(func() { testPool.Exec(context.Background(), `DELETE FROM "user" WHERE id = $1`, userID) })
	return userID
}

func publishedStdio(name string) nacos.MCPServerShape {
	return nacos.MCPServerShape{
		Name: name, Version: "1.0.0", Transport: "stdio",
		Command: name + "-mcp", Args: []string{"--port", "1"},
		EnvKeys: []string{"VOC_API_KEY"}, Lifecycle: "published",
	}
}

func TestMCPCatalog_ListMergesWorkspaceAndSharedNamespaces(t *testing.T) {
	withFakeNacos(t, &fakeCatalog{servers: map[string]nacos.MCPServerShape{
		nsKey(testWorkspaceID, "voc"):     publishedStdio("voc"),
		nsKey(sharedMCPNamespace, "fetch"): publishedStdio("fetch"),
	}})

	w := httptest.NewRecorder()
	testHandler.ListMCPCatalog(w, newRequestAsUser(testUserID, "GET", "/api/mcp-registry/servers", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("ListMCPCatalog: expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var resp struct {
		Servers []nacos.MCPServerShape `json:"servers"`
	}
	json.NewDecoder(w.Body).Decode(&resp)
	ns := map[string]string{}
	for _, s := range resp.Servers {
		ns[s.Name] = s.Namespace
	}
	if _, ok := ns["voc"]; !ok {
		t.Fatalf("expected workspace server voc, got %+v", resp.Servers)
	}
	if _, ok := ns["fetch"]; !ok {
		t.Fatalf("expected shared server fetch, got %+v", resp.Servers)
	}
	// Each entry must be tagged with its origin namespace so the picker can
	// build a correct MCPRef (workspace-local vs shared).
	if ns["voc"] != testWorkspaceID {
		t.Fatalf("voc namespace = %q, want workspace id %q", ns["voc"], testWorkspaceID)
	}
	if ns["fetch"] != sharedMCPNamespace {
		t.Fatalf("fetch namespace = %q, want %q", ns["fetch"], sharedMCPNamespace)
	}
}

func TestMCPCatalog_NonMemberRejected(t *testing.T) {
	withFakeNacos(t, &fakeCatalog{})
	nonMember := createMCPNonMemberUser(t)

	w := httptest.NewRecorder()
	testHandler.ListMCPCatalog(w, newRequestAsUser(nonMember, "GET", "/api/mcp-registry/servers", nil))
	if w.Code != http.StatusNotFound {
		t.Fatalf("non-member ListMCPCatalog: expected 404, got %d: %s", w.Code, w.Body.String())
	}
}

func TestMCPCatalog_NotConfiguredReturns503(t *testing.T) {
	withFakeNacos(t, nil) // simulate NACOS_SERVER_ADDR unset

	w := httptest.NewRecorder()
	testHandler.ListMCPCatalog(w, newRequestAsUser(testUserID, "GET", "/api/mcp-registry/servers", nil))
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("unconfigured ListMCPCatalog: expected 503, got %d: %s", w.Code, w.Body.String())
	}
}

func TestMCPCatalog_RegisterRequiresOwnerOrAdmin(t *testing.T) {
	fake := &fakeCatalog{}
	withFakeNacos(t, fake)
	member := createMCPTestMember(t, "member")

	body := map[string]any{"server": map[string]any{
		"name": "voc", "version": "1.0.0", "transport": "stdio", "command": "voc-mcp", "lifecycle": "published",
	}}

	// plain member → 403
	w := httptest.NewRecorder()
	testHandler.RegisterMCPCatalogServer(w, newRequestAsUser(member, "POST", "/api/mcp-registry/servers", body))
	if w.Code != http.StatusForbidden {
		t.Fatalf("member register: expected 403, got %d: %s", w.Code, w.Body.String())
	}

	// owner (testUserID) → 201 + recorded in catalog
	w = httptest.NewRecorder()
	testHandler.RegisterMCPCatalogServer(w, newRequestAsUser(testUserID, "POST", "/api/mcp-registry/servers", body))
	if w.Code != http.StatusCreated {
		t.Fatalf("owner register: expected 201, got %d: %s", w.Code, w.Body.String())
	}
	if _, ok := fake.servers[nsKey(testWorkspaceID, "voc")]; !ok {
		t.Fatalf("owner register did not persist server into catalog: %+v", fake.servers)
	}
}

func TestMCPCatalog_RegisterRejectsMissingNameOrVersion(t *testing.T) {
	withFakeNacos(t, &fakeCatalog{})
	body := map[string]any{"server": map[string]any{"name": "voc"}} // no version

	w := httptest.NewRecorder()
	testHandler.RegisterMCPCatalogServer(w, newRequestAsUser(testUserID, "POST", "/api/mcp-registry/servers", body))
	if w.Code != http.StatusBadRequest {
		t.Fatalf("register without version: expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestMCPCatalog_GetRejectsForeignNamespace(t *testing.T) {
	withFakeNacos(t, &fakeCatalog{servers: map[string]nacos.MCPServerShape{
		nsKey(testWorkspaceID, "voc"): publishedStdio("voc"),
	}})
	foreignNS := "00000000-0000-0000-0000-0000000000ff"

	req := newRequestAsUser(testUserID, "GET", "/api/mcp-registry/servers/voc?namespace="+foreignNS, nil)
	req = withURLParam(req, "name", "voc")
	w := httptest.NewRecorder()
	testHandler.GetMCPCatalogServer(w, req)
	if w.Code != http.StatusForbidden {
		t.Fatalf("foreign-namespace get: expected 403, got %d: %s", w.Code, w.Body.String())
	}
}

// --- resolveAgentMcpConfig (dispatch hook) ---

func TestResolveAgentMcpConfig_ResolvesRefsWithSecrets(t *testing.T) {
	withFakeNacos(t, &fakeCatalog{servers: map[string]nacos.MCPServerShape{
		nsKey(testWorkspaceID, "voc"): publishedStdio("voc"),
	}})
	refs, _ := json.Marshal([]nacos.MCPRef{{Namespace: testWorkspaceID, Name: "voc", Ref: "stable"}})

	out := testHandler.resolveAgentMcpConfig(
		context.Background(), "agent-1", refs, nil,
		map[string]string{"VOC_API_KEY": "secret-123"},
	)

	var parsed struct {
		McpServers map[string]struct {
			Command string            `json:"command"`
			Env     map[string]string `json:"env"`
		} `json:"mcpServers"`
	}
	if err := json.Unmarshal(out, &parsed); err != nil {
		t.Fatalf("unmarshal resolved config: %v (%s)", err, string(out))
	}
	voc, ok := parsed.McpServers["voc"]
	if !ok {
		t.Fatalf("resolved config missing voc server: %s", string(out))
	}
	if voc.Command != "voc-mcp" {
		t.Fatalf("voc command = %q, want voc-mcp", voc.Command)
	}
	if voc.Env["VOC_API_KEY"] != "secret-123" {
		t.Fatalf("voc env VOC_API_KEY = %q, want secret-123 (secret must be injected from custom_env)", voc.Env["VOC_API_KEY"])
	}
}

func TestResolveAgentMcpConfig_EmptyRefsReturnsInlineUnchanged(t *testing.T) {
	withFakeNacos(t, &fakeCatalog{})
	inline := json.RawMessage(`{"mcpServers":{"inline":{"command":"x"}}}`)

	out := testHandler.resolveAgentMcpConfig(context.Background(), "agent-1", []byte("[]"), inline, nil)
	if string(out) != string(inline) {
		t.Fatalf("empty refs must return inline unchanged; got %s", string(out))
	}
}

func TestResolveAgentMcpConfig_NoNacosReturnsInlineUnchanged(t *testing.T) {
	withFakeNacos(t, nil) // no adapter wired
	refs, _ := json.Marshal([]nacos.MCPRef{{Namespace: testWorkspaceID, Name: "voc", Ref: "stable"}})
	inline := json.RawMessage(`{"mcpServers":{"inline":{"command":"x"}}}`)

	out := testHandler.resolveAgentMcpConfig(context.Background(), "agent-1", refs, inline, nil)
	if string(out) != string(inline) {
		t.Fatalf("no Nacos must return inline unchanged; got %s", string(out))
	}
}
