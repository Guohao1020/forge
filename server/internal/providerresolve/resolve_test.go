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

func TestResolve_CodexArgsAndEnv(t *testing.T) {
	q := &fakePQ{shapes: map[string]nacos.ProviderShape{
		"router": {Name: "router", Version: "1", Protocol: "codex-router",
			BaseURL: "https://r/v1", AuthKey: "ROUTER_API_KEY", Lifecycle: "published"},
	}}
	res, warns, err := Resolve(context.Background(), q, Input{
		Ref: ref("router"), Secrets: MapSecrets{"ROUTER_API_KEY": "sek"},
	})
	if err != nil || len(warns) != 0 {
		t.Fatalf("err=%v warns=%v", err, warns)
	}
	want := []string{
		"-c", "model_provider=router",
		"-c", "model_providers.router.base_url=https://r/v1",
		"-c", "model_providers.router.wire_api=responses",
		"-c", "model_providers.router.env_key=ROUTER_API_KEY",
	}
	if len(res.Args) != len(want) {
		t.Fatalf("args len: %v", res.Args)
	}
	for i := range want {
		if res.Args[i] != want[i] {
			t.Fatalf("arg %d = %q, want %q", i, res.Args[i], want[i])
		}
	}
	if res.Env["ROUTER_API_KEY"] != "sek" {
		t.Fatalf("key env not injected: %v", res.Env)
	}
}

func TestResolve_MissingSecretWarns(t *testing.T) {
	q := &fakePQ{shapes: map[string]nacos.ProviderShape{
		"router": {Name: "router", Version: "1", Protocol: "anthropic",
			BaseURL: "https://r", AuthKey: "ROUTER_API_KEY", Lifecycle: "published"},
	}}
	_, warns, err := Resolve(context.Background(), q, Input{Ref: ref("router"), Secrets: MapSecrets{}})
	if err != nil {
		t.Fatal(err)
	}
	if len(warns) != 1 {
		t.Fatalf("want missing-secret warn, got %v", warns)
	}
}

func TestResolve_OfflineSkipped(t *testing.T) {
	q := &fakePQ{shapes: map[string]nacos.ProviderShape{
		"router": {Name: "router", Version: "1", Protocol: "anthropic", Lifecycle: "offline"},
	}}
	res, warns, _ := Resolve(context.Background(), q, Input{Ref: ref("router"), Secrets: MapSecrets{}})
	if len(warns) != 1 || len(res.Env) != 0 {
		t.Fatalf("offline should skip: env=%v warns=%v", res.Env, warns)
	}
}

func TestResolve_UnknownProtocolSkipped(t *testing.T) {
	q := &fakePQ{shapes: map[string]nacos.ProviderShape{
		"router": {Name: "router", Version: "1", Protocol: "bedrock", Lifecycle: "published"},
	}}
	res, warns, _ := Resolve(context.Background(), q, Input{Ref: ref("router"), Secrets: MapSecrets{}})
	if len(warns) != 1 || len(res.Env) != 0 || len(res.Args) != 0 {
		t.Fatalf("unknown protocol should skip: %+v warns=%v", res, warns)
	}
}

func TestResolve_ExplicitModelWins(t *testing.T) {
	q := &fakePQ{shapes: map[string]nacos.ProviderShape{
		"router": {Name: "router", Version: "1", Protocol: "anthropic", BaseURL: "https://r",
			AuthKey: "K", Lifecycle: "published", Models: []nacos.ProviderModel{{ID: "def", Default: true}}},
	}}
	res, _, _ := Resolve(context.Background(), q, Input{Ref: ref("router"), Secrets: MapSecrets{"K": "v"}, Model: "explicit"})
	if res.Model != "explicit" || res.Env["ANTHROPIC_MODEL"] != "explicit" {
		t.Fatalf("explicit model should win: %q / %q", res.Model, res.Env["ANTHROPIC_MODEL"])
	}
}

func TestResolve_NilRefNoop(t *testing.T) {
	res, warns, err := Resolve(context.Background(), &fakePQ{}, Input{Ref: nil})
	if err != nil || warns != nil || len(res.Env) != 0 || len(res.Args) != 0 {
		t.Fatalf("nil ref must be a clean no-op: %+v warns=%v err=%v", res, warns, err)
	}
}
