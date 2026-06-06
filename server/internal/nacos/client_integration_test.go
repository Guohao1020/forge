//go:build integration

package nacos

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"
)

// getWithRetry handles Nacos's eventual consistency: a freshly registered
// server's name->version record can lag the register call by a moment.
func getWithRetry(t *testing.T, c *Client, ns, name string) MCPServerShape {
	t.Helper()
	var lastErr error
	for i := 0; i < 10; i++ {
		s, err := c.GetMCPServer(context.Background(), ns, name, "stable")
		if err == nil {
			return s
		}
		lastErr = err
		time.Sleep(300 * time.Millisecond)
	}
	t.Fatalf("get %q after retries: %v", name, lastErr)
	return MCPServerShape{}
}

// Run against a live Nacos 3.x:
//   NACOS_TEST_ADDR=http://127.0.0.1:18848 go test -tags integration ./internal/nacos/ -run TestClient -v
func testClient(t *testing.T) *Client {
	addr := os.Getenv("NACOS_TEST_ADDR")
	if addr == "" {
		addr = "http://127.0.0.1:18848"
	}
	return NewClient(addr, "nacos", "nacos")
}

func TestClientRoundTrip(t *testing.T) {
	c := testClient(t)
	ctx := context.Background()
	name := fmt.Sprintf("iris-it-%d", time.Now().UnixNano()) // unique per run (register is create-only)

	if err := c.RegisterMCPServer(ctx, "public", MCPServerShape{
		Name: name, Version: "1.0.0", Transport: "stdio",
		Command: "voc-mcp", Args: []string{"--port", "1"}, EnvKeys: []string{"VOC_API_KEY"},
	}); err != nil {
		t.Fatalf("register: %v", err)
	}

	got := getWithRetry(t, c, "public", name)
	if got.Command != "voc-mcp" {
		t.Fatalf("command mismatch: %q", got.Command)
	}
	if len(got.EnvKeys) != 1 || got.EnvKeys[0] != "VOC_API_KEY" {
		t.Fatalf("env_keys mismatch: %v", got.EnvKeys)
	}
	if got.Transport != "stdio" || got.Lifecycle != "published" {
		t.Fatalf("transport/lifecycle mismatch: %+v", got)
	}

	list, err := c.ListMCPServers(ctx, "public")
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	found := false
	for _, s := range list {
		if s.Name == name {
			found = true
		}
	}
	if !found {
		t.Fatalf("registered server %q not in list", name)
	}
}
