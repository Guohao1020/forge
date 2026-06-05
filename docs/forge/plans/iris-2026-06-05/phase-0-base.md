# Phase 0 — Nacos 基座 + `agent.mcp_refs` + `NacosQuerier` 接口 + 适配实现

依赖:无。产出:Nacos 容器跑起来 + 实测接口记录 + DB 列 + 适配包(接口 + 缓存降级 + REST 实现)。

---

### Task 0.1: Nacos 3.x 容器进 self-host 栈

**Files:**
- Modify: `docker-compose.selfhost.yml`(加 `nacos` 服务)
- Modify: `docker-compose.selfhost.build.yml`(若需 dev 覆盖端口/env)

- [ ] **Step 1: 加 nacos 服务**(standalone + 开鉴权)

```yaml
# docker-compose.selfhost.yml,services: 下加
  nacos:
    image: nacos/nacos-server:v3.0.1
    environment:
      MODE: standalone
      NACOS_AUTH_ENABLE: "true"
      NACOS_AUTH_IDENTITY_KEY: nacos
      NACOS_AUTH_IDENTITY_VALUE: nacos
      NACOS_AUTH_TOKEN: ${NACOS_AUTH_TOKEN:-SecretKey012345678901234567890123456789012345678901234567890123456789}
    ports:
      - "127.0.0.1:${NACOS_PORT:-8848}:8848"
      - "127.0.0.1:9848:9848"
    volumes:
      - nacos_data:/home/nacos/data
    restart: unless-stopped

# volumes: 下加
  nacos_data:
```

- [ ] **Step 2: 起栈、验证 Nacos 活着**

Run: `docker compose -f docker-compose.selfhost.yml -f docker-compose.selfhost.build.yml -p forge-build up -d nacos`
然后 `curl -s http://127.0.0.1:8848/nacos/`(预期:200 / 控制台 HTML)。
> 注意端口:本机 8080 已被 `tagging-orchestration` 占;Nacos 用 8848,不冲突。

- [ ] **Step 3: Commit**

```bash
git add docker-compose.selfhost.yml docker-compose.selfhost.build.yml
git commit -m "feat(nacos): add Nacos 3.x to selfhost stack"
```

---

### Task 0.2: 探针 — 实测 AI Registry 的 MCP REST 接口

**Files:**
- Create: `server/internal/nacos/REST.md`(实测接口记录,供适配实现照抄)

- [ ] **Step 1: 取鉴权 token**

```bash
curl -s -X POST 'http://127.0.0.1:8848/nacos/v1/auth/login' \
  -d 'username=nacos&password=nacos'   # 预期返回 {"accessToken":"...","tokenTtl":...}
```

- [ ] **Step 2: 探 MCP 资源端点**(列/注册/详情)。AI Registry 在 Nacos 3.x 的 MCP 端点形如
  `/nacos/v3/admin/ai/mcp`(实测确认;若控制台 Network 面板能看到更准)。逐个 `curl` 记录:
  注册一个 dummy MCP server、列出、按 name 取详情、看版本字段。

```bash
TOKEN=<accessToken>
# 列(实测确认 path 与 query)
curl -s "http://127.0.0.1:8848/nacos/v3/admin/ai/mcp/list?namespaceId=public&accessToken=$TOKEN"
# 注册(实测确认 body schema)
curl -s -X POST "http://127.0.0.1:8848/nacos/v3/admin/ai/mcp?accessToken=$TOKEN" \
  -H 'Content-Type: application/json' -d '{ ... }'
```

- [ ] **Step 3: 把实测的 path / query / request body / response body 原样记进 `REST.md`**
  (含:list、get、register、set-lifecycle 的确切形状;namespace 参数名;版本/tag 字段名)。

- [ ] **Step 4: Commit**

```bash
git add server/internal/nacos/REST.md
git commit -m "docs(nacos): record measured AI Registry MCP REST API"
```

> 之后 Task 0.5 的 REST 实现**照 `REST.md` 填**具体 path/payload。接口(0.4)与上层(Phase 1/2)
> 不依赖此实测,可并行先做。

---

### Task 0.3: `agent.mcp_refs` 迁移 + sqlc

**Files:**
- Create: `server/migrations/NNN_agent_mcp_refs.up.sql` / `.down.sql`(NNN = 下一个迁移号)
- Modify: `server/pkg/db/queries/agent.sql`(读写 `mcp_refs`)

- [ ] **Step 1: 迁移**

```sql
-- up
ALTER TABLE agent ADD COLUMN mcp_refs JSONB NOT NULL DEFAULT '[]';
-- down
ALTER TABLE agent DROP COLUMN mcp_refs;
```

- [ ] **Step 2: 跑迁移 up/down/up 验证可逆**

Run: `cd server && go run ./cmd/migrate up && go run ./cmd/migrate down && go run ./cmd/migrate up`
Expected: 无错;`\d agent` 有 `mcp_refs jsonb not null default '[]'`。

- [ ] **Step 3: 在 `agent.sql` 的 GetAgent / UpdateAgent / CreateAgent 查询里带上 `mcp_refs` 列**
  (跟现有 `mcp_config` 并列),然后 `make sqlc` 重生成。

Run: `make sqlc` → 确认 `db.Agent` 多了 `McpRefs []byte`。

- [ ] **Step 4: Commit**

```bash
git add server/migrations/NNN_agent_mcp_refs.* server/pkg/db/queries/agent.sql server/pkg/db/generated/
git commit -m "feat(agent): add mcp_refs column + sqlc"
```

---

### Task 0.4: `NacosQuerier` 接口 + MCP 类型(纯类型,无外部依赖)

**Files:**
- Create: `server/internal/nacos/types.go`
- Create: `server/internal/nacos/querier.go`

- [ ] **Step 1: 类型 + 接口**

```go
// server/internal/nacos/types.go
package nacos

// MCPServerShape is the catalog entry WITHOUT secret values.
// env_keys / header_keys name the secrets; values are injected later from Multica.
type MCPServerShape struct {
	Name      string            `json:"name"`
	Version   string            `json:"version"`
	Transport string            `json:"transport"` // "stdio" | "sse" | "http"
	Command   string            `json:"command,omitempty"`
	Args      []string          `json:"args,omitempty"`
	EnvKeys   []string          `json:"env_keys,omitempty"`
	URL       string            `json:"url,omitempty"`
	HeaderKeys []string         `json:"header_keys,omitempty"`
	Lifecycle string            `json:"lifecycle"` // "published" | "offline" | "draft"
	Tools     []string          `json:"tools,omitempty"`
}

// MCPRef is what an agent stores (agent.mcp_refs element).
type MCPRef struct {
	Namespace string `json:"namespace"` // workspace id or "shared"
	Name      string `json:"name"`
	Ref       string `json:"ref"` // a version or a tag ("stable"/"latest")
}
```

```go
// server/internal/nacos/querier.go
package nacos

import "context"

// NacosQuerier is the seam the resolver + handler depend on. The real adapter
// (client.go) implements it over the Nacos REST API; tests use a fake.
type NacosQuerier interface {
	ListMCPServers(ctx context.Context, namespace string) ([]MCPServerShape, error)
	GetMCPServer(ctx context.Context, namespace, name, ref string) (MCPServerShape, error)
	RegisterMCPServer(ctx context.Context, namespace string, s MCPServerShape) error
	SetMCPLifecycle(ctx context.Context, namespace, name, version, lifecycle string) error
}
```

- [ ] **Step 2: `go build ./internal/nacos/`** → 预期通过(纯类型/接口)。

- [ ] **Step 3: Commit**

```bash
git add server/internal/nacos/types.go server/internal/nacos/querier.go
git commit -m "feat(nacos): NacosQuerier interface + MCP shape types"
```

---

### Task 0.5: 缓存降级包装 + REST 适配实现

**Files:**
- Create: `server/internal/nacos/cache.go` + `cache_test.go`
- Create: `server/internal/nacos/client.go`(REST,照 `REST.md` 填)

- [ ] **Step 1(TDD): 写缓存降级的失败测试** —— `GetMCPServer` 成功时回填缓存,底层失败时回退缓存。

```go
// server/internal/nacos/cache_test.go
package nacos

import (
	"context"
	"errors"
	"testing"
)

type fakeQ struct{ shape MCPServerShape; err error; calls int }
func (f *fakeQ) ListMCPServers(context.Context, string) ([]MCPServerShape, error) { return nil, f.err }
func (f *fakeQ) GetMCPServer(context.Context, string, string, string) (MCPServerShape, error) {
	f.calls++; return f.shape, f.err
}
func (f *fakeQ) RegisterMCPServer(context.Context, string, MCPServerShape) error { return f.err }
func (f *fakeQ) SetMCPLifecycle(context.Context, string, string, string, string) error { return f.err }

func TestCachedQuerier_FallsBackOnError(t *testing.T) {
	f := &fakeQ{shape: MCPServerShape{Name: "voc", Version: "1.0.0"}}
	c := NewCachedQuerier(f)
	// 1st: success → caches
	if _, err := c.GetMCPServer(context.Background(), "ws1", "voc", "stable"); err != nil {
		t.Fatalf("warm: %v", err)
	}
	// 2nd: underlying fails → must return cached, no error
	f.err = errors.New("nacos down")
	got, err := c.GetMCPServer(context.Background(), "ws1", "voc", "stable")
	if err != nil || got.Name != "voc" {
		t.Fatalf("expected cached fallback, got %+v err=%v", got, err)
	}
}
```

- [ ] **Step 2: 跑测试确认失败**

Run: `cd server && go test ./internal/nacos/ -run TestCachedQuerier -v`
Expected: FAIL（`NewCachedQuerier` undefined）。

- [ ] **Step 3: 实现 `CachedQuerier`**

```go
// server/internal/nacos/cache.go
package nacos

import (
	"context"
	"sync"
)

type CachedQuerier struct {
	inner NacosQuerier
	mu    sync.RWMutex
	cache map[string]MCPServerShape // key: ns|name|ref
}

func NewCachedQuerier(inner NacosQuerier) *CachedQuerier {
	return &CachedQuerier{inner: inner, cache: map[string]MCPServerShape{}}
}

func key(ns, name, ref string) string { return ns + "|" + name + "|" + ref }

func (c *CachedQuerier) GetMCPServer(ctx context.Context, ns, name, ref string) (MCPServerShape, error) {
	s, err := c.inner.GetMCPServer(ctx, ns, name, ref)
	if err == nil {
		c.mu.Lock(); c.cache[key(ns, name, ref)] = s; c.mu.Unlock()
		return s, nil
	}
	c.mu.RLock(); cached, ok := c.cache[key(ns, name, ref)]; c.mu.RUnlock()
	if ok { return cached, nil } // degrade to last-known
	return MCPServerShape{}, err
}

func (c *CachedQuerier) ListMCPServers(ctx context.Context, ns string) ([]MCPServerShape, error) {
	return c.inner.ListMCPServers(ctx, ns)
}
func (c *CachedQuerier) RegisterMCPServer(ctx context.Context, ns string, s MCPServerShape) error {
	return c.inner.RegisterMCPServer(ctx, ns, s)
}
func (c *CachedQuerier) SetMCPLifecycle(ctx context.Context, ns, name, ver, lc string) error {
	return c.inner.SetMCPLifecycle(ctx, ns, name, ver, lc)
}
```

- [ ] **Step 4: 跑测试确认通过**

Run: `cd server && go test ./internal/nacos/ -run TestCachedQuerier -v` → PASS。

- [ ] **Step 5: REST 适配 `client.go`**（照 `REST.md` 实测填 path/payload；含 token 登录缓存 +
  context 超时 + 有界重试）。实现 `NacosQuerier` 的四个方法,HTTP 调 Nacos。**结构如下,具体
  URL/JSON 字段按 `REST.md`**:

```go
// server/internal/nacos/client.go (骨架;path/payload 照 REST.md)
package nacos

import (
	"context"
	"net/http"
	"time"
)

type Client struct {
	base   string
	user   string
	pass   string
	http   *http.Client
}

func NewClient(base, user, pass string) *Client {
	return &Client{base: base, user: user, pass: pass, http: &http.Client{Timeout: 5 * time.Second}}
}
// login() 取 accessToken（缓存到过期）；doJSON() 带超时+1 次重试。
// 四个方法照 REST.md 拼 URL + 解析响应 → MCPServerShape。
func (c *Client) GetMCPServer(ctx context.Context, ns, name, ref string) (MCPServerShape, error) { /* REST.md */ return MCPServerShape{}, nil }
func (c *Client) ListMCPServers(ctx context.Context, ns string) ([]MCPServerShape, error) { /* REST.md */ return nil, nil }
func (c *Client) RegisterMCPServer(ctx context.Context, ns string, s MCPServerShape) error { /* REST.md */ return nil }
func (c *Client) SetMCPLifecycle(ctx context.Context, ns, name, ver, lc string) error { /* REST.md */ return nil }
```

- [ ] **Step 6: 集成冒烟**（需 Nacos 起着）：写个 `client_integration_test.go`（`//go:build integration`）
  注册→列→取,断言往返。Run: `go test -tags integration ./internal/nacos/ -run TestClientRoundTrip -v`。

- [ ] **Step 7: Commit**

```bash
git add server/internal/nacos/cache.go server/internal/nacos/cache_test.go server/internal/nacos/client.go server/internal/nacos/client_integration_test.go
git commit -m "feat(nacos): cached querier (degradation) + REST client"
```
