package mcpresolve

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/multica-ai/multica/server/internal/nacos"
)

// fakeQ returns canned shapes keyed by name; names in failNames fail.
type fakeQ struct {
	shapes    map[string]nacos.MCPServerShape
	failNames map[string]bool
}

func (f *fakeQ) GetMCPServer(_ context.Context, _ string, name string, _ string) (nacos.MCPServerShape, error) {
	if f.failNames[name] {
		return nacos.MCPServerShape{}, errors.New("not found")
	}
	return f.shapes[name], nil
}
func (f *fakeQ) ListMCPServers(context.Context, string) ([]nacos.MCPServerShape, error) { return nil, nil }
func (f *fakeQ) RegisterMCPServer(context.Context, string, nacos.MCPServerShape) error  { return nil }
func (f *fakeQ) SetMCPLifecycle(context.Context, string, string, string, string) error  { return nil }

func mcpServers(t *testing.T, raw json.RawMessage) map[string]map[string]any {
	t.Helper()
	var out struct {
		McpServers map[string]map[string]any `json:"mcpServers"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("bad json: %v", err)
	}
	return out.McpServers
}

func TestResolve_StdioWithSecret(t *testing.T) {
	q := &fakeQ{shapes: map[string]nacos.MCPServerShape{
		"voc": {Name: "voc", Version: "1.0.0", Transport: "stdio", Command: "voc-mcp",
			Args: []string{"--port", "1"}, EnvKeys: []string{"VOC_API_KEY"}, Lifecycle: "published"},
	}}
	in := Input{
		Refs:    []nacos.MCPRef{{Namespace: "ws1", Name: "voc", Ref: "stable"}},
		Secrets: MapSecrets{"VOC_API_KEY": "sekret"},
	}
	raw, warns, err := ResolveMCP(context.Background(), q, in)
	if err != nil {
		t.Fatal(err)
	}
	if len(warns) != 0 {
		t.Fatalf("unexpected warns: %v", warns)
	}
	srv := mcpServers(t, raw)["voc"]
	if srv["command"] != "voc-mcp" {
		t.Fatalf("command: %v", srv["command"])
	}
	env := srv["env"].(map[string]any)
	if env["VOC_API_KEY"] != "sekret" {
		t.Fatalf("secret not injected: %v", env)
	}
}

func TestResolve_MissingRefSkipped(t *testing.T) {
	q := &fakeQ{
		shapes:    map[string]nacos.MCPServerShape{"ok": {Name: "ok", Version: "1", Transport: "stdio", Command: "x", Lifecycle: "published"}},
		failNames: map[string]bool{"gone": true},
	}
	raw, warns, err := ResolveMCP(context.Background(), q, Input{Refs: []nacos.MCPRef{
		{Namespace: "ws1", Name: "gone", Ref: "stable"}, {Namespace: "ws1", Name: "ok", Ref: "stable"},
	}, Secrets: MapSecrets{}})
	if err != nil {
		t.Fatal(err)
	}
	if len(warns) != 1 {
		t.Fatalf("want 1 warn, got %v", warns)
	}
	if _, ok := mcpServers(t, raw)["ok"]; !ok {
		t.Fatal("ok should survive")
	}
	if _, ok := mcpServers(t, raw)["gone"]; ok {
		t.Fatal("gone should be skipped")
	}
}

func TestResolve_InlineOverridesByName(t *testing.T) {
	q := &fakeQ{shapes: map[string]nacos.MCPServerShape{
		"voc": {Name: "voc", Version: "1", Transport: "stdio", Command: "catalog", Lifecycle: "published"},
	}}
	in := Input{
		Refs:      []nacos.MCPRef{{Namespace: "ws1", Name: "voc", Ref: "stable"}},
		InlineMCP: json.RawMessage(`{"mcpServers":{"voc":{"command":"local-override"}}}`),
		Secrets:   MapSecrets{},
	}
	raw, _, err := ResolveMCP(context.Background(), q, in)
	if err != nil {
		t.Fatal(err)
	}
	if mcpServers(t, raw)["voc"]["command"] != "local-override" {
		t.Fatal("inline should win")
	}
}

func TestResolve_MissingSecretWarns(t *testing.T) {
	q := &fakeQ{shapes: map[string]nacos.MCPServerShape{
		"voc": {Name: "voc", Version: "1", Transport: "stdio", Command: "x", EnvKeys: []string{"K"}, Lifecycle: "published"},
	}}
	_, warns, err := ResolveMCP(context.Background(), q, Input{
		Refs: []nacos.MCPRef{{Namespace: "ws1", Name: "voc", Ref: "stable"}}, Secrets: MapSecrets{},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(warns) != 1 {
		t.Fatalf("want missing-secret warn, got %v", warns)
	}
}

func TestResolve_Remote(t *testing.T) {
	q := &fakeQ{shapes: map[string]nacos.MCPServerShape{
		"api": {Name: "api", Version: "1", Transport: "sse", URL: "https://x/mcp", HeaderKeys: []string{"AUTH"}, Lifecycle: "published"},
	}}
	raw, _, err := ResolveMCP(context.Background(), q, Input{
		Refs: []nacos.MCPRef{{Namespace: "ws1", Name: "api", Ref: "stable"}}, Secrets: MapSecrets{"AUTH": "t"},
	})
	if err != nil {
		t.Fatal(err)
	}
	srv := mcpServers(t, raw)["api"]
	if srv["url"] != "https://x/mcp" {
		t.Fatal("url")
	}
	if srv["headers"].(map[string]any)["AUTH"] != "t" {
		t.Fatal("header secret")
	}
}
