# Phase 1 — `providerresolve` 解析器 + anthropic/codex 两映射器(纯逻辑,TDD,mock)

依赖:Phase 0(`nacos.ProviderQuerier` 接口 + 类型)。产出:`provider_ref` → `(env, args, model)`
的纯函数 + 每协议一个映射器,mock 单测全绿,**不依赖真 Nacos**。镜像 N1 `internal/mcpresolve`。

---

### Task 1.1: 解析器 + anthropic 映射器 + 首个失败测试

**Files:**
- Create: `server/internal/providerresolve/resolve.go`
- Create: `server/internal/providerresolve/anthropic.go`
- Create: `server/internal/providerresolve/codex.go`
- Create: `server/internal/providerresolve/resolve_test.go`

- [ ] **Step 1: 写失败测试**(anthropic:env 出 `ANTHROPIC_*`,密值从 secrets 注入)

```go
// server/internal/providerresolve/resolve_test.go
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
func (f *fakePQ) SetProviderLifecycle(context.Context, string, string, string, string) error { return nil }

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
```

- [ ] **Step 2: 跑确认失败** — `cd server && go test ./internal/providerresolve/ -run TestResolve_Anthropic -v` → FAIL（undefined）。

- [ ] **Step 3: 实现 resolver + anthropic 映射器**

```go
// server/internal/providerresolve/resolve.go
package providerresolve

import (
	"context"
	"fmt"

	"github.com/multica-ai/multica/server/internal/nacos"
)

// SecretSource resolves a secret KEY (named in the provider shape) to its value.
// Backed by agent.custom_env. Never comes from Nacos.
type SecretSource interface{ Get(key string) (string, bool) }

type MapSecrets map[string]string

func (m MapSecrets) Get(k string) (string, bool) { v, ok := m[k]; return v, ok }

// Mapper translates a provider shape into the env + args a given runtime needs.
type Mapper interface {
	Protocol() string
	Map(shape nacos.ProviderShape, secrets SecretSource, model string) (env map[string]string, args []string, warnings []string)
}

type Input struct {
	Ref     *nacos.ProviderRef // agent.provider_ref (nil = none)
	Secrets SecretSource       // agent.custom_env
	Model   string             // agent.model
}

type Result struct {
	Env   map[string]string
	Args  []string
	Model string // resolved default when in.Model was empty
}

var mappers = map[string]Mapper{}

func register(m Mapper) { mappers[m.Protocol()] = m }

func init() {
	register(anthropicMapper{})
	register(codexRouterMapper{})
}

// Resolve looks up the provider shape and runs the protocol's mapper. Best-effort:
// a missing/offline provider or unknown protocol is skipped with a warning and an
// empty Result (caller keeps the agent's existing env/args). Never errors on a
// bad ref — only a programming error returns err.
func Resolve(ctx context.Context, q nacos.ProviderQuerier, in Input) (Result, []string, error) {
	if in.Ref == nil {
		return Result{}, nil, nil
	}
	shape, err := q.GetProvider(ctx, in.Ref.Namespace, in.Ref.Name, in.Ref.Ref)
	if err != nil {
		return Result{}, []string{fmt.Sprintf("provider %s/%s@%s skipped: %v", in.Ref.Namespace, in.Ref.Name, in.Ref.Ref, err)}, nil
	}
	if shape.Lifecycle != "published" {
		return Result{}, []string{fmt.Sprintf("provider %s@%s is %q; skipped", shape.Name, shape.Version, shape.Lifecycle)}, nil
	}
	m, ok := mappers[shape.Protocol]
	if !ok {
		return Result{}, []string{fmt.Sprintf("provider %s: unknown protocol %q; skipped", shape.Name, shape.Protocol)}, nil
	}
	model := in.Model
	if model == "" {
		model = defaultModel(shape)
	}
	env, args, warns := m.Map(shape, in.Secrets, model)
	return Result{Env: env, Args: args, Model: model}, warns, nil
}

func defaultModel(s nacos.ProviderShape) string {
	for _, mm := range s.Models {
		if mm.Default {
			return mm.ID
		}
	}
	return ""
}
```

```go
// server/internal/providerresolve/anthropic.go
package providerresolve

import (
	"fmt"

	"github.com/multica-ai/multica/server/internal/nacos"
)

// anthropicMapper → Claude env. Mirrors what live execution wired by hand:
// ANTHROPIC_BASE_URL / ANTHROPIC_AUTH_TOKEN / ANTHROPIC_MODEL.
type anthropicMapper struct{}

func (anthropicMapper) Protocol() string { return "anthropic" }

func (anthropicMapper) Map(s nacos.ProviderShape, secrets SecretSource, model string) (map[string]string, []string, []string) {
	var warns []string
	env := map[string]string{"ANTHROPIC_BASE_URL": s.BaseURL}
	v, ok := secrets.Get(s.AuthKey)
	if !ok {
		warns = append(warns, fmt.Sprintf("provider %s: secret %q missing from agent env", s.Name, s.AuthKey))
	}
	env["ANTHROPIC_AUTH_TOKEN"] = v
	if model != "" {
		env["ANTHROPIC_MODEL"] = model
	}
	return env, nil, warns
}
```

```go
// server/internal/providerresolve/codex.go
package providerresolve

import (
	"fmt"

	"github.com/multica-ai/multica/server/internal/nacos"
)

// codexRouterMapper → Codex custom_args (-c model_providers.<name>.*) + the key
// env var (codex reads the key via env_key). argv tokens are SEPARATE slice
// elements (each "-c" and its "key=value" are distinct args) so the daemon
// forwards them verbatim to the CLI argv.
type codexRouterMapper struct{}

func (codexRouterMapper) Protocol() string { return "codex-router" }

func (codexRouterMapper) Map(s nacos.ProviderShape, secrets SecretSource, _ string) (map[string]string, []string, []string) {
	var warns []string
	wire := s.WireAPI
	if wire == "" {
		wire = "responses"
	}
	args := []string{
		"-c", "model_provider=" + s.Name,
		"-c", "model_providers." + s.Name + ".base_url=" + s.BaseURL,
		"-c", "model_providers." + s.Name + ".wire_api=" + wire,
		"-c", "model_providers." + s.Name + ".env_key=" + s.AuthKey,
	}
	v, ok := secrets.Get(s.AuthKey)
	if !ok {
		warns = append(warns, fmt.Sprintf("provider %s: secret %q missing from agent env", s.Name, s.AuthKey))
	}
	env := map[string]string{s.AuthKey: v}
	return env, args, warns
}
```

- [ ] **Step 4: 跑确认通过** — `go test ./internal/providerresolve/ -run TestResolve_Anthropic -v` → PASS。
- [ ] **Step 5: Commit** — `git add server/internal/providerresolve/ && git commit -m "feat(providerresolve): resolve provider_ref -> env/args (anthropic + codex mappers)"`

---

### Task 1.2: 边界用例(codex args / 缺 secret / offline 跳过 / 未知 protocol / 显式 model 赢 / nil ref)

**Files:** Modify `server/internal/providerresolve/resolve_test.go`

- [ ] **Step 1: 加六个测试**

```go
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
```

- [ ] **Step 2: 跑全包测试** — `go test ./internal/providerresolve/ -v` → 全 PASS。
- [ ] **Step 3: `go vet ./internal/providerresolve/`** → 干净。
- [ ] **Step 4: Commit** — `git commit -am "test(providerresolve): edge cases (codex/missing-secret/offline/unknown-protocol/model/nil)"`
