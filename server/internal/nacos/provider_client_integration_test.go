//go:build integration

package nacos

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"testing"
	"time"
)

// getProviderWithRetry handles Nacos's eventual consistency: a freshly
// published config can lag the publish call by a moment.
func getProviderWithRetry(t *testing.T, c *ProviderClient, ns, name string) ProviderShape {
	t.Helper()
	var lastErr error
	for i := 0; i < 10; i++ {
		p, err := c.GetProvider(context.Background(), ns, name, "stable")
		if err == nil {
			return p
		}
		lastErr = err
		time.Sleep(300 * time.Millisecond)
	}
	t.Fatalf("get %q after retries: %v", name, lastErr)
	return ProviderShape{}
}

// deleteProviderConfig is a best-effort teardown for the unique-named test
// config (the config center's delete API is not part of ProviderQuerier).
func deleteProviderConfig(c *ProviderClient, ns, name string) {
	q := url.Values{"dataId": {name}, "groupName": {providerGroup}, "namespaceId": {ns}}
	_, _ = c.do(context.Background(), http.MethodDelete, "/nacos/v3/admin/cs/config", q, nil)
}

// Run against a live Nacos 3.x config center:
//   NACOS_TEST_ADDR=http://127.0.0.1:8848 go test -tags integration ./internal/nacos/ -run TestProviderClient -v
func testProviderClient(t *testing.T) *ProviderClient {
	addr := os.Getenv("NACOS_TEST_ADDR")
	if addr == "" {
		addr = "http://127.0.0.1:8848"
	}
	return NewProviderClient(addr, "nacos", "nacos")
}

func TestProviderClientRoundTrip(t *testing.T) {
	c := testProviderClient(t)
	ctx := context.Background()
	name := fmt.Sprintf("router-it-%d", time.Now().UnixNano()) // unique per run
	defer deleteProviderConfig(c, "shared", name)

	if err := c.RegisterProvider(ctx, "shared", ProviderShape{
		Name:      name,
		Version:   "1.0.0",
		Protocol:  "anthropic",
		BaseURL:   "https://router.example/api",
		AuthKey:   "ROUTER_API_KEY",
		Lifecycle: "published",
		Models: []ProviderModel{
			{ID: "claude-opus", Label: "Opus", Default: true},
		},
	}); err != nil {
		t.Fatalf("register: %v", err)
	}

	got := getProviderWithRetry(t, c, "shared", name)
	if got.Name != name {
		t.Fatalf("name mismatch: %q", got.Name)
	}
	if got.Protocol != "anthropic" {
		t.Fatalf("protocol mismatch: %q", got.Protocol)
	}
	if got.BaseURL != "https://router.example/api" {
		t.Fatalf("base_url mismatch: %q", got.BaseURL)
	}
	if got.AuthKey != "ROUTER_API_KEY" {
		t.Fatalf("auth_key mismatch: %q", got.AuthKey)
	}
	if got.Namespace != "shared" {
		t.Fatalf("namespace not tagged: %q", got.Namespace)
	}

	list, err := c.ListProviders(ctx, "shared")
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	found := false
	for _, p := range list {
		if p.Name == name {
			found = true
		}
	}
	if !found {
		t.Fatalf("registered provider %q not in list", name)
	}
}
