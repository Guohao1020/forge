package providerresolve

import (
	"context"
	"errors"
	"testing"

	"github.com/multica-ai/multica/server/internal/nacos"
)

// fakePQ returns one shape; failNames make GetProvider error.
type fakePQ struct {
	shapes    map[string]nacos.ProviderShape
	failNames map[string]bool
}

func (f *fakePQ) GetProvider(_ context.Context, _, name, _ string) (nacos.ProviderShape, error) {
	if f.failNames[name] {
		return nacos.ProviderShape{}, errors.New("not found")
	}
	return f.shapes[name], nil
}
func (f *fakePQ) ListProviders(context.Context, string) ([]nacos.ProviderShape, error) { return nil, nil }
func (f *fakePQ) RegisterProvider(context.Context, string, nacos.ProviderShape) error  { return nil }
func (f *fakePQ) SetProviderLifecycle(context.Context, string, string, string, string) error {
	return nil
}

func ref(name string) *nacos.ProviderRef {
	return &nacos.ProviderRef{Namespace: "ws1", Name: name, Ref: "stable"}
}

func TestResolve_AnthropicEnvWithSecret(t *testing.T) {
	q := &fakePQ{shapes: map[string]nacos.ProviderShape{
		"router": {Name: "router", Version: "1.0.0", Protocol: "anthropic",
			BaseURL: "https://r/api", AuthKey: "ROUTER_API_KEY", Lifecycle: "published",
			Models: []nacos.ProviderModel{{ID: "claude-sonnet-4-6", Default: true}}},
	}}
	res, warns, err := Resolve(context.Background(), q, Input{
		Ref: ref("router"), Secrets: MapSecrets{"ROUTER_API_KEY": "sekret"}, Model: "",
	})
	if err != nil || len(warns) != 0 {
		t.Fatalf("err=%v warns=%v", err, warns)
	}
	if res.Env["ANTHROPIC_BASE_URL"] != "https://r/api" {
		t.Fatalf("base_url: %v", res.Env["ANTHROPIC_BASE_URL"])
	}
	if res.Env["ANTHROPIC_AUTH_TOKEN"] != "sekret" {
		t.Fatalf("secret not injected: %v", res.Env["ANTHROPIC_AUTH_TOKEN"])
	}
	if res.Model != "claude-sonnet-4-6" {
		t.Fatalf("default model not picked: %q", res.Model)
	}
	if res.Env["ANTHROPIC_MODEL"] != "claude-sonnet-4-6" {
		t.Fatalf("ANTHROPIC_MODEL: %q", res.Env["ANTHROPIC_MODEL"])
	}
}
