//go:build integration

package providerresolve

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/multica-ai/multica/server/internal/nacos"
)

// TestEndToEndRegisterRefResolve 是 prometheus N2 的免凭证验收测试:对一个活体
// Nacos 3.x 配置中心,注册一条 provider(只存 auth_key 的 KEY 名),再像 agent 那样
// 用 provider_ref 引用它,resolve 成生效的 env / args / model —— 断言 secret 的
// VALUE 是从(模拟的)agent custom_env 注入的,而 Nacos 里只存了 KEY 名。
// 不拉起真实 agent / CLI;直接走 register → ref → resolve。
//
// 对 docker 栈里的 Nacos 跑:
//
//	NACOS_TEST_ADDR=http://127.0.0.1:8848 go test -tags integration ./internal/providerresolve/ -run TestEndToEnd -v
func TestEndToEndRegisterRefResolve(t *testing.T) {
	addr := os.Getenv("NACOS_TEST_ADDR")
	if addr == "" {
		addr = "http://127.0.0.1:8848"
	}
	q := nacos.NewCachedProviderQuerier(nacos.NewProviderClient(addr, "nacos", "nacos"))
	ctx := context.Background()
	name := fmt.Sprintf("router-e2e-%d", time.Now().UnixNano())

	if err := q.RegisterProvider(ctx, "shared", nacos.ProviderShape{
		Name: name, Version: "1.0.0", Protocol: "anthropic",
		BaseURL: "https://router.example/api", AuthKey: "ROUTER_API_KEY", Lifecycle: "published",
		Models: []nacos.ProviderModel{{ID: "claude-sonnet-4-6", Default: true}},
	}); err != nil {
		t.Fatalf("register: %v", err)
	}

	// 最终一致性:刚发布的 provider 可能短暂不可见,轮询直到 resolve 看到它。
	var res Result
	var lastErr error
	for i := 0; i < 10; i++ {
		r, warns, err := Resolve(ctx, q, Input{
			Ref:     &nacos.ProviderRef{Namespace: "shared", Name: name, Ref: "stable"},
			Secrets: MapSecrets{"ROUTER_API_KEY": "secret-xyz"},
		})
		if err == nil && r.Env["ANTHROPIC_BASE_URL"] != "" {
			res = r
			break
		}
		lastErr = fmt.Errorf("not visible yet: warns=%v err=%v", warns, err)
		time.Sleep(300 * time.Millisecond)
	}
	if res.Env["ANTHROPIC_BASE_URL"] != "https://router.example/api" {
		t.Fatalf("resolve never saw provider: %v", lastErr)
	}
	if res.Env["ANTHROPIC_AUTH_TOKEN"] != "secret-xyz" {
		t.Fatalf("secret must be injected from agent env: %q", res.Env["ANTHROPIC_AUTH_TOKEN"])
	}
	if res.Model != "claude-sonnet-4-6" {
		t.Fatalf("default model: %q", res.Model)
	}

	// 退化:CachedProviderQuerier 已在上面成功 resolve 时 warm 了缓存。第二次
	// resolve 即便只剩缓存这一个来源也应成功 —— 证明 last-known shape 仍可派发。
	r2, _, err := Resolve(ctx, q, Input{
		Ref:     &nacos.ProviderRef{Namespace: "shared", Name: name, Ref: "stable"},
		Secrets: MapSecrets{"ROUTER_API_KEY": "secret-xyz"},
	})
	if err != nil || r2.Env["ANTHROPIC_BASE_URL"] == "" {
		t.Fatalf("second resolve (cache-warm) failed: %v %v", err, r2.Env)
	}
}
