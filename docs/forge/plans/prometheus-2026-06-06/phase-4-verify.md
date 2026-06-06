# Phase 4 — 绕凭证集成验收 + seed + 文档

依赖:Phase 0–3。产出:源码栈上"注册→设 ref→解析出 env/args"端到端实测(不跑活体)+ seed + 文档。
镜像 N1 phase-4。

---

### Task 4.1: 绕凭证集成实测(源码构建栈)

**Files:**
- Create: `server/internal/providerresolve/verify_e2e_test.go`(`//go:build integration`)

- [ ] **Step 1: 端到端测试**(对真 Nacos 配置中心:注册 provider → 经 `CachedProviderQuerier` 解析 →
  断言 anthropic 出 `ANTHROPIC_*`/codex 出 `-c model_providers.*`,密值从模拟 custom_env 注入)

```go
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

// Run: NACOS_TEST_ADDR=http://127.0.0.1:8848 go test -tags integration ./internal/providerresolve/ -run TestEndToEnd -v
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

	// eventual consistency: retry until visible
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

	// degradation: cache warmed by the resolve above; a second resolve still serves.
	r2, _, err := Resolve(ctx, q, Input{
		Ref:     &nacos.ProviderRef{Namespace: "shared", Name: name, Ref: "stable"},
		Secrets: MapSecrets{"ROUTER_API_KEY": "secret-xyz"},
	})
	if err != nil || r2.Env["ANTHROPIC_BASE_URL"] == "" {
		t.Fatalf("second resolve (cache-warm) failed: %v %v", err, r2.Env)
	}
}
```

- [ ] **Step 2: 跑**(Nacos 起着)`NACOS_TEST_ADDR=http://127.0.0.1:8848 go test -tags integration ./internal/providerresolve/ -run TestEndToEnd -v` → PASS。
- [ ] **Step 3: 记录证据**:把"注册→设 ref→解析出的 env/args + 降级走缓存"贴进 plan DoD 勾选。
- [ ] **Step 4: Commit** — `git commit -am "test(providerresolve): creds-free e2e — register->ref->resolve + secret injection"`

> 真 Nacos-down 降级属手动 runbook(`docker stop forge-build-nacos-1` → 再解析走缓存);机制已由
> `provider_cache_test.go` 单测 + 本测暖缓存第二次 resolve 覆盖,Go 测不控 docker。

---

### Task 4.2: seed 一个示例 provider

**Files:**
- Create: `scripts/seed-provider.sh`(照 `scripts/seed-mcp.sh`,POST 配置中心)

- [ ] **Step 1**:往 `shared` namespace seed 一个示例 provider(`flatkey-router`,protocol=anthropic,
  base_url=`<ROUTER_BASE_URL>` 占位,auth_key=`ROUTER_API_KEY`,published,models 列几个别名)。
  **只 seed shape,不 seed 密钥值 / 不写真 base_url**(占位符)。POST `/nacos/v3/admin/cs/config`
  (照 `REST-providers.md` 的 form 字段),带 identity header。

- [ ] **Step 2**:`bash scripts/seed-provider.sh` → `curl` list 确认 `flatkey-router` 在 shared 出现。
- [ ] **Step 3: Commit** — `git commit -am "chore(provider): seed sample LLM provider into shared namespace"`

---

### Task 4.3: 文档 / 记忆

**Files:**
- Modify: `docs/forge/specs/nacos-integration/README.md`(N2 状态 → done;§6 实现状态表加 N2 行)
- Modify: `docs/forge/specs/nacos-integration/N2-provider-model-registry.md`(§11 把实测出的配置中心
  REST 决议补上,链 `REST-providers.md`)
- Modify: 项目记忆 `memory/`(加 N2 done 条目:配置中心存 provider、两映射器、claim 钩子合并、daemon 零改动)

- [ ] **Step 1**:更新上述文档,**不写明文凭证**(占位符)。
- [ ] **Step 2: Commit** — `git commit -m "docs(nacos): N2 done — update spec status + memory"`

---

### Task 4.4: 全量门禁

- [ ] **Step 1: Go** — `cd server && CGO_ENABLED=0 go build ./... && go vet ./internal/nacos/ ./internal/providerresolve/ ./internal/handler/ && go test ./internal/nacos/ ./internal/providerresolve/`(无 DB 部分);handler 测对一次性 PG 跑 `go test ./internal/handler/ -run 'LLMProvider|ResolveAgentProvider'`。
- [ ] **Step 2: TS** — `pnpm typecheck`(core/views/web 绿;docs 包 fumadocs 缺 node_modules 是预存环境问题,忽略)+ `pnpm --filter @multica/core test` + provider-picker 测。
- [ ] **Step 3**:有红就修到全绿。
- [ ] **Step 4**:`finishing-a-development-branch` 收尾(合并/PR 由你定)。

---

## DoD 回填(实测后勾)

- [ ] Nacos 配置中心可 publish/get/list provider;`REST-providers.md` 记录实测接口
- [ ] `agent.provider_ref` 迁移 117 可逆 + sqlc
- [ ] `providerresolve` 全单测绿(anthropic env / codex args / 缺 secret / offline / 未知 protocol / 显式 model / nil ref / 降级)
- [ ] `/api/llm-providers/*` + 鉴权 + 单测;claim 钩子(ref 空 = 原行为;agent 覆盖赢;daemon 零改动)
- [ ] 前端目录 + provider 选择器 + 模型 picker 收敛 + zod;三包 typecheck 绿
- [ ] 绕凭证 e2e:注册→设 ref→解析出 env/args(secret 注入、默认 model);停 Nacos→走缓存
