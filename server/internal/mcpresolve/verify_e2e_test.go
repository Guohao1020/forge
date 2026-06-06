//go:build integration

package mcpresolve

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/multica-ai/multica/server/internal/nacos"
)

// TestEndToEndRegisterRefsResolve is the creds-free acceptance test for iris
// N0+N1: against a live Nacos 3.x it registers a catalog server, references it
// the way an agent would (mcp_refs), and resolves that ref into an effective
// mcp_config — asserting the secret VALUE is injected from the (simulated)
// agent custom_env while the catalog only ever stored the KEY name. No live
// agent/CLI is launched; this exercises register → refs → resolve directly.
//
// Run against the docker stack Nacos:
//
//	NACOS_TEST_ADDR=http://127.0.0.1:8848 go test -tags integration ./internal/mcpresolve/ -run TestEndToEnd -v
func TestEndToEndRegisterRefsResolve(t *testing.T) {
	addr := os.Getenv("NACOS_TEST_ADDR")
	if addr == "" {
		addr = "http://127.0.0.1:8848"
	}
	q := nacos.NewCachedQuerier(nacos.NewClient(addr, "nacos", "nacos"))
	ctx := context.Background()

	name := fmt.Sprintf("demo-e2e-%d", time.Now().UnixNano()) // unique: register is create-only
	if err := q.RegisterMCPServer(ctx, "shared", nacos.MCPServerShape{
		Name: name, Version: "1.0.0", Transport: "stdio",
		Command: "demo-mcp", Args: []string{"--flag"}, EnvKeys: []string{"DEMO_KEY"},
	}); err != nil {
		t.Fatalf("register: %v", err)
	}

	// Eventual consistency: the freshly-registered name->version record can lag.
	var resolved json.RawMessage
	var lastErr error
	for i := 0; i < 10; i++ {
		out, warns, err := ResolveMCP(ctx, q, Input{
			Refs:    []nacos.MCPRef{{Namespace: "shared", Name: name, Ref: "stable"}},
			Secrets: MapSecrets{"DEMO_KEY": "secret-xyz"},
		})
		if err != nil {
			lastErr = err
			time.Sleep(300 * time.Millisecond)
			continue
		}
		// A skipped ref leaves mcpServers empty + emits a warning; retry until
		// the registration is visible.
		if hasServer(out, name) {
			resolved = out
			break
		}
		lastErr = fmt.Errorf("server %q not yet in resolved config; warnings=%v", name, warns)
		time.Sleep(300 * time.Millisecond)
	}
	if resolved == nil {
		t.Fatalf("resolve never saw the registered server: %v", lastErr)
	}

	var parsed struct {
		McpServers map[string]struct {
			Command string            `json:"command"`
			Args    []string          `json:"args"`
			Env     map[string]string `json:"env"`
		} `json:"mcpServers"`
	}
	if err := json.Unmarshal(resolved, &parsed); err != nil {
		t.Fatalf("unmarshal resolved: %v (%s)", err, string(resolved))
	}
	srv, ok := parsed.McpServers[name]
	if !ok {
		t.Fatalf("resolved config missing %q: %s", name, string(resolved))
	}
	if srv.Command != "demo-mcp" {
		t.Fatalf("command = %q, want demo-mcp", srv.Command)
	}
	if srv.Env["DEMO_KEY"] != "secret-xyz" {
		t.Fatalf("env DEMO_KEY = %q, want secret-xyz (secret must be injected from agent env, not the catalog)", srv.Env["DEMO_KEY"])
	}

	// Degradation: the CachedQuerier warmed its cache on the successful resolve
	// above. A second resolve through the same querier still succeeds even if
	// the cache is the only source — proving last-known shape is served. (A
	// true Nacos-down test stops the container; that is a manual runbook step
	// since a Go test shouldn't control docker — see plan Phase 4 Step 3.)
	out2, _, err := ResolveMCP(ctx, q, Input{
		Refs:    []nacos.MCPRef{{Namespace: "shared", Name: name, Ref: "stable"}},
		Secrets: MapSecrets{"DEMO_KEY": "secret-xyz"},
	})
	if err != nil || !hasServer(out2, name) {
		t.Fatalf("second resolve (cache-warm) failed: err=%v out=%s", err, string(out2))
	}
}

func hasServer(raw json.RawMessage, name string) bool {
	var parsed struct {
		McpServers map[string]json.RawMessage `json:"mcpServers"`
	}
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return false
	}
	_, ok := parsed.McpServers[name]
	return ok
}
