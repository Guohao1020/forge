# Phase 1 — `mcpresolve` 解析器(纯逻辑,TDD,mock)

依赖:Phase 0(`nacos.NacosQuerier` 接口 + 类型)。产出:`agent.mcp_refs` → 有效 `mcp_config` 的
纯函数,mock 单测全绿,**不依赖真 Nacos**。对称 `internal/forge/checks`、`internal/forge/standards`。

---

### Task 1.1: 解析器 + 首个失败测试(单 stdio ref + 注 secret)

**Files:**
- Create: `server/internal/mcpresolve/resolve.go`
- Create: `server/internal/mcpresolve/resolve_test.go`

- [ ] **Step 1: 写失败测试**

```go
// server/internal/mcpresolve/resolve_test.go
package mcpresolve

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/multica-ai/multica/server/internal/nacos"
)

// fakeQ returns canned shapes keyed by name; err names fail.
type fakeQ struct{ shapes map[string]nacos.MCPServerShape; failNames map[string]bool }

func (f *fakeQ) GetMCPServer(_ context.Context, _ , name, _ string) (nacos.MCPServerShape, error) {
	if f.failNames[name] { return nacos.MCPServerShape{}, errors.New("not found") }
	return f.shapes[name], nil
}
func (f *fakeQ) ListMCPServers(context.Context, string) ([]nacos.MCPServerShape, error) { return nil, nil }
func (f *fakeQ) RegisterMCPServer(context.Context, string, nacos.MCPServerShape) error { return nil }
func (f *fakeQ) SetMCPLifecycle(context.Context, string, string, string, string) error { return nil }

func mcpServers(t *testing.T, raw json.RawMessage) map[string]map[string]any {
	t.Helper()
	var out struct{ McpServers map[string]map[string]any `json:"mcpServers"` }
	if err := json.Unmarshal(raw, &out); err != nil { t.Fatalf("bad json: %v", err) }
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
	if err != nil { t.Fatal(err) }
	if len(warns) != 0 { t.Fatalf("unexpected warns: %v", warns) }
	srv := mcpServers(t, raw)["voc"]
	if srv["command"] != "voc-mcp" { t.Fatalf("command: %v", srv["command"]) }
	env := srv["env"].(map[string]any)
	if env["VOC_API_KEY"] != "sekret" { t.Fatalf("secret not injected: %v", env) }
}
```

- [ ] **Step 2: 跑确认失败** — `cd server && go test ./internal/mcpresolve/ -run TestResolve_StdioWithSecret -v` → FAIL（undefined）。

- [ ] **Step 3: 实现**

```go
// server/internal/mcpresolve/resolve.go
package mcpresolve

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/multica-ai/multica/server/internal/nacos"
)

// SecretSource resolves a secret KEY (named in the catalog shape) to its value.
// Backed by the agent's custom_env. Never comes from Nacos.
type SecretSource interface{ Get(key string) (string, bool) }

type MapSecrets map[string]string

func (m MapSecrets) Get(k string) (string, bool) { v, ok := m[k]; return v, ok }

type Input struct {
	Refs      []nacos.MCPRef
	InlineMCP json.RawMessage // existing agent.mcp_config: {"mcpServers": {...}}
	Secrets   SecretSource
}

// ResolveMCP builds the effective mcp_config from catalog refs + inline config.
// Best-effort: a bad ref is skipped with a warning, never fails the whole resolve.
func ResolveMCP(ctx context.Context, q nacos.NacosQuerier, in Input) (json.RawMessage, []string, error) {
	servers := map[string]json.RawMessage{}
	var warnings []string

	for _, ref := range in.Refs {
		shape, err := q.GetMCPServer(ctx, ref.Namespace, ref.Name, ref.Ref)
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("mcp %s/%s@%s skipped: %v", ref.Namespace, ref.Name, ref.Ref, err))
			continue
		}
		if shape.Lifecycle != "published" {
			warnings = append(warnings, fmt.Sprintf("mcp %s@%s is %q; skipped", shape.Name, shape.Version, shape.Lifecycle))
			continue
		}
		entry, w := buildEntry(shape, in.Secrets)
		warnings = append(warnings, w...)
		servers[shape.Name] = entry
	}

	// inline mcp_config overrides cataloged entries by name (local override).
	if len(in.InlineMCP) > 0 {
		var inline struct {
			McpServers map[string]json.RawMessage `json:"mcpServers"`
		}
		if err := json.Unmarshal(in.InlineMCP, &inline); err == nil {
			for name, cfg := range inline.McpServers {
				servers[name] = cfg
			}
		}
	}

	out, err := json.Marshal(map[string]any{"mcpServers": servers})
	if err != nil {
		return nil, warnings, err
	}
	return out, warnings, nil
}

func buildEntry(s nacos.MCPServerShape, secrets SecretSource) (json.RawMessage, []string) {
	var warnings []string
	m := map[string]any{}
	if s.Transport == "stdio" {
		m["command"] = s.Command
		if len(s.Args) > 0 {
			m["args"] = s.Args
		}
		if len(s.EnvKeys) > 0 {
			env := map[string]string{}
			for _, k := range s.EnvKeys {
				v, ok := secrets.Get(k)
				if !ok {
					warnings = append(warnings, fmt.Sprintf("mcp %s: secret %q missing from agent env", s.Name, k))
				}
				env[k] = v
			}
			m["env"] = env
		}
	} else { // "sse" | "http"
		m["url"] = s.URL
		if len(s.HeaderKeys) > 0 {
			headers := map[string]string{}
			for _, k := range s.HeaderKeys {
				v, ok := secrets.Get(k)
				if !ok {
					warnings = append(warnings, fmt.Sprintf("mcp %s: secret %q missing from agent env", s.Name, k))
				}
				headers[k] = v
			}
			m["headers"] = headers
		}
	}
	b, _ := json.Marshal(m)
	return b, warnings
}
```

- [ ] **Step 4: 跑确认通过** — `go test ./internal/mcpresolve/ -run TestResolve_StdioWithSecret -v` → PASS。

- [ ] **Step 5: Commit** — `git add server/internal/mcpresolve/ && git commit -m "feat(mcpresolve): resolve mcp_refs -> effective mcp_config (stdio+secret)"`

---

### Task 1.2: 边界用例(缺 ref 跳过 / 内联覆盖 / 缺 secret / remote)

**Files:** Modify `server/internal/mcpresolve/resolve_test.go`

- [ ] **Step 1: 加四个测试**

```go
func TestResolve_MissingRefSkipped(t *testing.T) {
	q := &fakeQ{
		shapes:    map[string]nacos.MCPServerShape{"ok": {Name: "ok", Version: "1", Transport: "stdio", Command: "x", Lifecycle: "published"}},
		failNames: map[string]bool{"gone": true},
	}
	raw, warns, err := ResolveMCP(context.Background(), q, Input{Refs: []nacos.MCPRef{
		{Namespace: "ws1", Name: "gone", Ref: "stable"}, {Namespace: "ws1", Name: "ok", Ref: "stable"},
	}, Secrets: MapSecrets{}})
	if err != nil { t.Fatal(err) }
	if len(warns) != 1 { t.Fatalf("want 1 warn, got %v", warns) }
	if _, ok := mcpServers(t, raw)["ok"]; !ok { t.Fatal("ok should survive") }
	if _, ok := mcpServers(t, raw)["gone"]; ok { t.Fatal("gone should be skipped") }
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
	if err != nil { t.Fatal(err) }
	if mcpServers(t, raw)["voc"]["command"] != "local-override" { t.Fatal("inline should win") }
}

func TestResolve_MissingSecretWarns(t *testing.T) {
	q := &fakeQ{shapes: map[string]nacos.MCPServerShape{
		"voc": {Name: "voc", Version: "1", Transport: "stdio", Command: "x", EnvKeys: []string{"K"}, Lifecycle: "published"},
	}}
	_, warns, err := ResolveMCP(context.Background(), q, Input{
		Refs: []nacos.MCPRef{{Namespace: "ws1", Name: "voc", Ref: "stable"}}, Secrets: MapSecrets{},
	})
	if err != nil { t.Fatal(err) }
	if len(warns) != 1 { t.Fatalf("want missing-secret warn, got %v", warns) }
}

func TestResolve_Remote(t *testing.T) {
	q := &fakeQ{shapes: map[string]nacos.MCPServerShape{
		"api": {Name: "api", Version: "1", Transport: "sse", URL: "https://x/mcp", HeaderKeys: []string{"AUTH"}, Lifecycle: "published"},
	}}
	raw, _, err := ResolveMCP(context.Background(), q, Input{
		Refs: []nacos.MCPRef{{Namespace: "ws1", Name: "api", Ref: "stable"}}, Secrets: MapSecrets{"AUTH": "t"},
	})
	if err != nil { t.Fatal(err) }
	srv := mcpServers(t, raw)["api"]
	if srv["url"] != "https://x/mcp" { t.Fatal("url") }
	if srv["headers"].(map[string]any)["AUTH"] != "t" { t.Fatal("header secret") }
}
```

- [ ] **Step 2: 跑全包测试** — `go test ./internal/mcpresolve/ -v` → 全 PASS。
- [ ] **Step 3: `go vet ./internal/mcpresolve/`** → 干净。
- [ ] **Step 4: Commit** — `git commit -am "test(mcpresolve): edge cases (skip/override/missing-secret/remote)"`
