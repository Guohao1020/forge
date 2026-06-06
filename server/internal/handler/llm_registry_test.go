package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/multica-ai/multica/server/internal/nacos"
)

// fakeProviderCatalog is an in-memory ProviderQuerier for handler tests — no
// live Nacos. Keyed by "ns|name" via pk (distinct from the MCP test's nsKey to
// avoid a same-package redeclaration).
type fakeProviderCatalog struct {
	items   map[string]nacos.ProviderShape
	listErr error
}

func pk(ns, name string) string { return ns + "|" + name }

func (f *fakeProviderCatalog) ListProviders(_ context.Context, ns string) ([]nacos.ProviderShape, error) {
	if f.listErr != nil {
		return nil, f.listErr
	}
	var out []nacos.ProviderShape
	for k, p := range f.items {
		if strings.HasPrefix(k, ns+"|") {
			out = append(out, p)
		}
	}
	return out, nil
}

func (f *fakeProviderCatalog) GetProvider(_ context.Context, ns, name, _ string) (nacos.ProviderShape, error) {
	p, ok := f.items[pk(ns, name)]
	if !ok {
		return nacos.ProviderShape{}, errors.New("not found")
	}
	return p, nil
}

func (f *fakeProviderCatalog) RegisterProvider(_ context.Context, ns string, p nacos.ProviderShape) error {
	if f.items == nil {
		f.items = map[string]nacos.ProviderShape{}
	}
	f.items[pk(ns, p.Name)] = p
	return nil
}

func (f *fakeProviderCatalog) SetProviderLifecycle(_ context.Context, ns, name, _, lifecycle string) error {
	p, ok := f.items[pk(ns, name)]
	if !ok {
		return errors.New("not found")
	}
	p.Lifecycle = lifecycle
	f.items[pk(ns, name)] = p
	return nil
}

// withFakeProviders installs a fake provider catalog on the shared test handler
// for the duration of one test and restores the previous value afterward.
func withFakeProviders(t *testing.T, f nacos.ProviderQuerier) {
	t.Helper()
	prev := testHandler.Providers
	testHandler.Providers = f
	t.Cleanup(func() { testHandler.Providers = prev })
}

func publishedProvider(name string) nacos.ProviderShape {
	return nacos.ProviderShape{
		Name: name, Version: "1", Protocol: "anthropic",
		BaseURL: "https://catalog", AuthKey: "ROUTER_API_KEY", Lifecycle: "published",
	}
}

func TestLLMProviders_ListMergesWorkspaceAndShared(t *testing.T) {
	withFakeProviders(t, &fakeProviderCatalog{items: map[string]nacos.ProviderShape{
		pk(testWorkspaceID, "router"):     publishedProvider("router"),
		pk(sharedMCPNamespace, "fallback"): publishedProvider("fallback"),
	}})

	w := httptest.NewRecorder()
	testHandler.ListLLMProviders(w, newRequestAsUser(testUserID, "GET", "/api/llm-providers", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("ListLLMProviders: expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var resp struct {
		Providers []nacos.ProviderShape `json:"providers"`
	}
	json.NewDecoder(w.Body).Decode(&resp)
	ns := map[string]string{}
	for _, p := range resp.Providers {
		ns[p.Name] = p.Namespace
	}
	if _, ok := ns["router"]; !ok {
		t.Fatalf("expected workspace provider router, got %+v", resp.Providers)
	}
	if _, ok := ns["fallback"]; !ok {
		t.Fatalf("expected shared provider fallback, got %+v", resp.Providers)
	}
	// Each entry must be tagged with its origin namespace so the picker can
	// build a correct ProviderRef (workspace-local vs shared).
	if ns["router"] != testWorkspaceID {
		t.Fatalf("router namespace = %q, want workspace id %q", ns["router"], testWorkspaceID)
	}
	if ns["fallback"] != sharedMCPNamespace {
		t.Fatalf("fallback namespace = %q, want %q", ns["fallback"], sharedMCPNamespace)
	}
}

func TestLLMProviders_NonMemberRejected(t *testing.T) {
	withFakeProviders(t, &fakeProviderCatalog{})
	nonMember := createMCPNonMemberUser(t)

	w := httptest.NewRecorder()
	testHandler.ListLLMProviders(w, newRequestAsUser(nonMember, "GET", "/api/llm-providers", nil))
	if w.Code != http.StatusNotFound {
		t.Fatalf("non-member ListLLMProviders: expected 404, got %d: %s", w.Code, w.Body.String())
	}
}

func TestLLMProviders_NotConfiguredReturns503(t *testing.T) {
	withFakeProviders(t, nil) // simulate NACOS_SERVER_ADDR unset

	w := httptest.NewRecorder()
	testHandler.ListLLMProviders(w, newRequestAsUser(testUserID, "GET", "/api/llm-providers", nil))
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("unconfigured ListLLMProviders: expected 503, got %d: %s", w.Code, w.Body.String())
	}
}

func TestLLMProviders_RegisterRequiresOwnerOrAdmin(t *testing.T) {
	fake := &fakeProviderCatalog{}
	withFakeProviders(t, fake)
	member := createMCPTestMember(t, "member")

	body := map[string]any{"provider": map[string]any{
		"name": "router", "version": "1", "protocol": "anthropic",
		"base_url": "https://catalog", "auth_key": "ROUTER_API_KEY", "lifecycle": "published",
	}}

	// plain member → 403
	w := httptest.NewRecorder()
	testHandler.RegisterLLMProvider(w, newRequestAsUser(member, "POST", "/api/llm-providers", body))
	if w.Code != http.StatusForbidden {
		t.Fatalf("member register: expected 403, got %d: %s", w.Code, w.Body.String())
	}

	// owner (testUserID) → 201 + recorded in catalog
	w = httptest.NewRecorder()
	testHandler.RegisterLLMProvider(w, newRequestAsUser(testUserID, "POST", "/api/llm-providers", body))
	if w.Code != http.StatusCreated {
		t.Fatalf("owner register: expected 201, got %d: %s", w.Code, w.Body.String())
	}
	if _, ok := fake.items[pk(testWorkspaceID, "router")]; !ok {
		t.Fatalf("owner register did not persist provider into catalog: %+v", fake.items)
	}
}

func TestLLMProviders_RegisterRejectsMissingFields(t *testing.T) {
	withFakeProviders(t, &fakeProviderCatalog{})
	// name + version present but no protocol → 400
	body := map[string]any{"provider": map[string]any{"name": "router", "version": "1"}}

	w := httptest.NewRecorder()
	testHandler.RegisterLLMProvider(w, newRequestAsUser(testUserID, "POST", "/api/llm-providers", body))
	if w.Code != http.StatusBadRequest {
		t.Fatalf("register without protocol: expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestLLMProviders_GetRejectsForeignNamespace(t *testing.T) {
	withFakeProviders(t, &fakeProviderCatalog{items: map[string]nacos.ProviderShape{
		pk(testWorkspaceID, "router"): publishedProvider("router"),
	}})
	foreignNS := "00000000-0000-0000-0000-0000000000ff"

	req := newRequestAsUser(testUserID, "GET", "/api/llm-providers/router?namespace="+foreignNS, nil)
	req = withURLParam(req, "name", "router")
	w := httptest.NewRecorder()
	testHandler.GetLLMProvider(w, req)
	if w.Code != http.StatusForbidden {
		t.Fatalf("foreign-namespace get: expected 403, got %d: %s", w.Code, w.Body.String())
	}
}

// --- resolveAgentProviderRef (dispatch hook) ---

func TestResolveAgentProvider_MergesEnvAgentWins(t *testing.T) {
	withFakeProviders(t, &fakeProviderCatalog{items: map[string]nacos.ProviderShape{
		pk(testWorkspaceID, "router"): {Name: "router", Version: "1", Protocol: "anthropic",
			BaseURL: "https://catalog", AuthKey: "ROUTER_API_KEY", Lifecycle: "published"},
	}})
	pref, _ := json.Marshal(nacos.ProviderRef{Namespace: testWorkspaceID, Name: "router", Ref: "stable"})
	env, _, _ := testHandler.resolveAgentProviderRef(context.Background(), "a1", pref,
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
	env, args, model := testHandler.resolveAgentProviderRef(context.Background(), "a1", []byte("null"),
		map[string]string{"X": "1"}, []string{"-a"}, "m")
	if env["X"] != "1" || len(args) != 1 || model != "m" {
		t.Fatalf("null provider_ref must be unchanged: %v %v %v", env, args, model)
	}
}
