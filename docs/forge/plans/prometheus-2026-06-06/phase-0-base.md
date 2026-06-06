# Phase 0 — 配置中心 spike + `ProviderQuerier` 接口/适配/缓存 + `agent.provider_ref`

依赖:无。产出:Nacos 配置中心实测接口记录 + DB 列 + provider 适配包(接口 + 缓存降级 + REST 实现)。
镜像 N1 的 `internal/nacos/{types,querier,cache,client}.go`。

---

### Task 0.1: 探针 — 实测 Nacos 配置中心的 provider REST 接口

**Files:**
- Create: `server/internal/nacos/REST-providers.md`(实测接口记录,供适配实现照抄)

前置:Nacos 已在 `forge-build` 栈跑(N1 已加,`nacos/nacos-server:v3.2.2`,API `127.0.0.1:8848`,
identity header `nacos: nacos`)。

- [ ] **Step 1: 实测 publish / get / list / history**(逐个 `curl`,带 identity header)

```bash
ADDR=http://127.0.0.1:8848; H='nacos: nacos'
# publish(form):dataId/group/namespaceId/content/type=json
curl -s -X POST "$ADDR/nacos/v3/admin/cs/config" -H "$H" \
  --data-urlencode 'dataId=flatkey-router' --data-urlencode 'group=forge-llm-providers' \
  --data-urlencode 'namespaceId=shared' --data-urlencode 'type=json' \
  --data-urlencode 'content={"name":"flatkey-router","version":"1.0.0","protocol":"anthropic","base_url":"<ROUTER_BASE_URL>","auth_key":"ROUTER_API_KEY","lifecycle":"published"}'
# get
curl -s "$ADDR/nacos/v3/admin/cs/config?dataId=flatkey-router&group=forge-llm-providers&namespaceId=shared" -H "$H"
# list by group(确认 path:可能是 /cs/config/list 或带 search=blur 的分页)
curl -s "$ADDR/nacos/v3/admin/cs/config/list?group=forge-llm-providers&namespaceId=shared&pageNo=1&pageSize=100" -H "$H"
# history(当 version 用)
curl -s "$ADDR/nacos/v3/admin/cs/history/list?dataId=flatkey-router&group=forge-llm-providers&namespaceId=shared" -H "$H"
```

- [ ] **Step 2: 把实测的确切 path / 参数名 / 响应信封原样记进 `REST-providers.md`**
  (含:publish 的 form 字段、get 的响应是裸 content 还是带信封、list 的分页字段名 `pageItems`/`configList`、
  history 当 version 的取法、namespaceId 空时是否落 `public`)。**也确认 AI Registry 确无 provider 资源类型**
  → 定稿用配置中心。

- [ ] **Step 3: Commit** — `git add server/internal/nacos/REST-providers.md && git commit -m "docs(nacos): record measured config-center provider REST API"`

> 之后 Task 0.4 的 REST 实现**照 `REST-providers.md` 填**。接口(0.3)与上层(Phase 1/2)不依赖此实测,可并行先做。

---

### Task 0.2: `agent.provider_ref` 迁移 + sqlc

**Files:**
- Create: `server/migrations/117_agent_provider_ref.up.sql` / `.down.sql`
- Modify: `server/pkg/db/queries/agent.sql`(`CreateAgent` / `UpdateAgent` 带 `provider_ref`)

- [ ] **Step 1: 迁移**

```sql
-- 117_agent_provider_ref.up.sql
-- Forge prometheus (N2): single optional reference into the Nacos LLM-provider
-- catalog. NULL = agent uses no provider (today's static/runtime model path);
-- a JSON object = {namespace,name,ref}. Unlike mcp_refs (a list with '[]'
-- default) this is a single nullable ref, so NULL is the clean "unset" state.
ALTER TABLE agent ADD COLUMN provider_ref JSONB;
```
```sql
-- 117_agent_provider_ref.down.sql
ALTER TABLE agent DROP COLUMN IF EXISTS provider_ref;
```

- [ ] **Step 2: 跑迁移可逆(一次性 PG,不碰 dev 库)**

```bash
docker rm -f n2-mig >/dev/null 2>&1
docker run -d --name n2-mig -e POSTGRES_DB=multica -e POSTGRES_USER=multica -e POSTGRES_PASSWORD=multica -p 127.0.0.1:15440:5432 pgvector/pgvector:pg17
sleep 3
export DATABASE_URL="postgres://multica:multica@localhost:15440/multica?sslmode=disable"
cd server && CGO_ENABLED=0 go run ./cmd/migrate up && go run ./cmd/migrate down && go run ./cmd/migrate up
docker exec n2-mig psql -U multica -d multica -tAc "SELECT column_name,data_type FROM information_schema.columns WHERE table_name='agent' AND column_name='provider_ref';"
docker rm -f n2-mig
```
Expected: 末尾 up 后列 `provider_ref | jsonb` 存在;down 干净。

- [ ] **Step 3: `agent.sql` 的 `UpdateAgent` 带 `provider_ref`**(narg COALESCE)

**只改 `UpdateAgent`**(不碰 `CreateAgent`):provider_ref 可空、默认 NULL,创建时省略该列即得 NULL,
设值统一走 UpdateAgent —— 避免动 CreateAgent 的所有调用方。在 `UpdateAgent` 的 SET 里加一行:
`provider_ref = COALESCE(sqlc.narg('provider_ref'), provider_ref),`。然后 `make sqlc`
(或 `"$(go env GOPATH)/bin/sqlc" generate`,sqlc v1.31.1)。

Run: 确认 `db.Agent` 多了 `ProviderRef []byte`、`UpdateAgentParams` 多了 `ProviderRef`。

- [ ] **Step 4: `CGO_ENABLED=0 go build ./...`** → 绿(现有 CreateAgent 调用方不传新字段即 nil → INSERT 写 NULL,符合可空列)。

- [ ] **Step 5: Commit** — `git add server/migrations/117_agent_provider_ref.* server/pkg/db/queries/agent.sql server/pkg/db/generated/ && git commit -m "feat(agent): add provider_ref column + sqlc"`

---

### Task 0.3: `ProviderShape` 类型 + `ProviderQuerier` 接口(纯类型,无外部依赖)

**Files:**
- Create: `server/internal/nacos/provider_types.go`
- Create: `server/internal/nacos/provider_querier.go`

- [ ] **Step 1: 类型 + 接口**

```go
// server/internal/nacos/provider_types.go
package nacos

// ProviderShape is an LLM-provider catalog entry WITHOUT secret values.
// AuthKey names the secret (e.g. "ROUTER_API_KEY"); the value is injected later
// from Multica (agent.custom_env), never stored in Nacos. BaseURL is an
// endpoint, not a secret. Namespace is tagged on list (workspace id / "shared").
type ProviderShape struct {
	Name      string          `json:"name"`
	Namespace string          `json:"namespace,omitempty"`
	Version   string          `json:"version"`
	Protocol  string          `json:"protocol"` // "anthropic" | "codex-router"
	BaseURL   string          `json:"base_url"`
	AuthKey   string          `json:"auth_key"`
	WireAPI   string          `json:"wire_api,omitempty"` // codex; default "responses"
	Models    []ProviderModel `json:"models,omitempty"`
	Lifecycle string          `json:"lifecycle"` // "published" | "offline" | "draft"
}

type ProviderModel struct {
	ID      string `json:"id"`
	Label   string `json:"label,omitempty"`
	Default bool   `json:"default,omitempty"`
}

// ProviderRef is what an agent stores (agent.provider_ref). Single, optional.
type ProviderRef struct {
	Namespace string `json:"namespace"`
	Name      string `json:"name"`
	Ref       string `json:"ref"` // version or tag ("stable"/"latest")
}
```

```go
// server/internal/nacos/provider_querier.go
package nacos

import "context"

// ProviderQuerier is the seam the providerresolve package + handler depend on.
// The real adapter (provider_client.go) implements it over the Nacos config
// center; tests use a fake.
type ProviderQuerier interface {
	ListProviders(ctx context.Context, namespace string) ([]ProviderShape, error)
	GetProvider(ctx context.Context, namespace, name, ref string) (ProviderShape, error)
	RegisterProvider(ctx context.Context, namespace string, p ProviderShape) error
	SetProviderLifecycle(ctx context.Context, namespace, name, version, lifecycle string) error
}
```

- [ ] **Step 2: `CGO_ENABLED=0 go build ./internal/nacos/`** → 通过(纯类型/接口)。
- [ ] **Step 3: Commit** — `git commit -am "feat(nacos): ProviderQuerier interface + provider shape types"`

---

### Task 0.4: 缓存降级包装 + 配置中心 REST 适配

**Files:**
- Create: `server/internal/nacos/provider_cache.go` + `provider_cache_test.go`
- Create: `server/internal/nacos/provider_client.go`(REST,照 `REST-providers.md` 填)
- Create: `server/internal/nacos/provider_client_integration_test.go`(`//go:build integration`)

- [ ] **Step 1(TDD): 缓存降级失败测试**(镜像 N1 `cache_test.go`)

```go
// server/internal/nacos/provider_cache_test.go
package nacos

import (
	"context"
	"errors"
	"testing"
)

type fakePQ struct {
	shape ProviderShape
	err   error
}

func (f *fakePQ) ListProviders(context.Context, string) ([]ProviderShape, error) { return nil, f.err }
func (f *fakePQ) GetProvider(context.Context, string, string, string) (ProviderShape, error) {
	return f.shape, f.err
}
func (f *fakePQ) RegisterProvider(context.Context, string, ProviderShape) error      { return f.err }
func (f *fakePQ) SetProviderLifecycle(context.Context, string, string, string, string) error { return f.err }

func TestCachedProviderQuerier_FallsBackOnError(t *testing.T) {
	f := &fakePQ{shape: ProviderShape{Name: "router", Version: "1.0.0", Protocol: "anthropic"}}
	c := NewCachedProviderQuerier(f)
	if _, err := c.GetProvider(context.Background(), "ws1", "router", "stable"); err != nil {
		t.Fatalf("warm: %v", err)
	}
	f.err = errors.New("nacos down")
	got, err := c.GetProvider(context.Background(), "ws1", "router", "stable")
	if err != nil || got.Name != "router" {
		t.Fatalf("expected cached fallback, got %+v err=%v", got, err)
	}
}
```

- [ ] **Step 2: 跑确认失败** — `cd server && go test ./internal/nacos/ -run TestCachedProviderQuerier -v` → FAIL(`NewCachedProviderQuerier` undefined)。

- [ ] **Step 3: 实现 `CachedProviderQuerier`**(照 N1 `cache.go`,key `ns|name|ref`)

```go
// server/internal/nacos/provider_cache.go
package nacos

import (
	"context"
	"sync"
)

type CachedProviderQuerier struct {
	inner ProviderQuerier
	mu    sync.RWMutex
	cache map[string]ProviderShape
}

func NewCachedProviderQuerier(inner ProviderQuerier) *CachedProviderQuerier {
	return &CachedProviderQuerier{inner: inner, cache: map[string]ProviderShape{}}
}

func providerKey(ns, name, ref string) string { return ns + "|" + name + "|" + ref }

func (c *CachedProviderQuerier) GetProvider(ctx context.Context, ns, name, ref string) (ProviderShape, error) {
	s, err := c.inner.GetProvider(ctx, ns, name, ref)
	if err == nil {
		c.mu.Lock()
		c.cache[providerKey(ns, name, ref)] = s
		c.mu.Unlock()
		return s, nil
	}
	c.mu.RLock()
	cached, ok := c.cache[providerKey(ns, name, ref)]
	c.mu.RUnlock()
	if ok {
		return cached, nil
	}
	return ProviderShape{}, err
}

func (c *CachedProviderQuerier) ListProviders(ctx context.Context, ns string) ([]ProviderShape, error) {
	return c.inner.ListProviders(ctx, ns)
}
func (c *CachedProviderQuerier) RegisterProvider(ctx context.Context, ns string, p ProviderShape) error {
	return c.inner.RegisterProvider(ctx, ns, p)
}
func (c *CachedProviderQuerier) SetProviderLifecycle(ctx context.Context, ns, name, ver, lc string) error {
	return c.inner.SetProviderLifecycle(ctx, ns, name, ver, lc)
}
```

- [ ] **Step 4: 跑确认通过** — `go test ./internal/nacos/ -run TestCachedProviderQuerier -v` → PASS。

- [ ] **Step 5: REST 适配 `provider_client.go`**(照 `REST-providers.md` 填 path/payload；含 identity
  header + context 超时 + 有界重试)。骨架(具体 URL/字段按 `REST-providers.md`):

```go
// server/internal/nacos/provider_client.go  (骨架;path/payload 照 REST-providers.md)
package nacos

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type ProviderClient struct {
	base   string
	idKey  string
	idVal  string
	client *http.Client
}

func NewProviderClient(base, identityKey, identityValue string) *ProviderClient {
	return &ProviderClient{
		base:   strings.TrimRight(base, "/"),
		idKey:  identityKey,
		idVal:  identityValue,
		client: &http.Client{Timeout: 5 * time.Second},
	}
}

const providerGroup = "forge-llm-providers"

// GetProvider: GET /nacos/v3/admin/cs/config?dataId=<name>&group=forge-llm-providers&namespaceId=<ns>
//   → content is ProviderShape JSON. Set shape.Namespace=ns, shape.Version from
//     history/tag per REST-providers.md.
func (c *ProviderClient) GetProvider(ctx context.Context, ns, name, ref string) (ProviderShape, error) { /* REST-providers.md */ return ProviderShape{}, nil }

// ListProviders: GET .../cs/config/list?group=forge-llm-providers&namespaceId=<ns>&pageNo=1&pageSize=500
//   → for each item, parse content → ProviderShape, tag Namespace=ns.
func (c *ProviderClient) ListProviders(ctx context.Context, ns string) ([]ProviderShape, error) { /* REST-providers.md */ return nil, nil }

// RegisterProvider: POST .../cs/config (form: dataId=name, group, namespaceId=ns, type=json,
//   content=json.Marshal(p with lifecycle="published")).
func (c *ProviderClient) RegisterProvider(ctx context.Context, ns string, p ProviderShape) error { /* REST-providers.md */ return nil }

// SetProviderLifecycle: read content, set lifecycle, re-publish (config center has no
//   native lifecycle; it's a content field).
func (c *ProviderClient) SetProviderLifecycle(ctx context.Context, ns, name, version, lifecycle string) error { /* REST-providers.md */ return nil }

func providerForm(p ProviderShape) url.Values { b, _ := json.Marshal(p); _ = b; return url.Values{} } // helper, fill per REST
```

- [ ] **Step 6: 集成冒烟**(需 Nacos 起着)`provider_client_integration_test.go`(`//go:build integration`)
  注册→get→list,断言往返。用唯一 name `fmt.Sprintf("router-it-%d", time.Now().UnixNano())`(若 publish
  是 upsert 就无 409 问题;若 create-only 则唯一 name 规避)。
  Run: `NACOS_TEST_ADDR=http://127.0.0.1:8848 go test -tags integration ./internal/nacos/ -run TestProviderClient -v`。

- [ ] **Step 7: Commit** — `git add server/internal/nacos/provider_*.go && git commit -m "feat(nacos): cached provider querier (degradation) + config-center REST client"`
