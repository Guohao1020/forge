# N2 — LLM Provider / 模型 中心注册表(实现级 spec)

> 切片 2。隶属 [Nacos 接入总体方案](README.md)。**实现级**(2026-06-06 由架构草案深化:
> clarifying → approaches → 4 节设计)。沿用 N0+N1 的**做法 A**(Nacos 真相源 + 派发解析 +
> secret 分离 + 缓存降级)。
>
> **安全**:`origin` 公开。本文凭证一律占位符(`<ROUTER_BASE_URL>` / `<ROUTER_API_KEY>`);
> 真值只进 env / Multica DB,**永不写入 git / Nacos**。

**Goal:** 把 LLM provider(端点 + 协议 + 模型目录 + 密钥 KEY 名)集中进 Nacos,agent 按
`provider_ref` 引用;**派发时服务端把 provider shape 解析成该 runtime 要的 env(Claude `ANTHROPIC_*`)
或 args(Codex `-c model_providers.*`)+ 从 Multica 注入密钥**,把活体执行手工散配那套产品化成
"选一个 provider"。

**Architecture:** Nacos **配置中心**存 provider shape(AI Registry 无 provider 资源类型);后端
`internal/nacos` 加 `ProviderQuerier` 适配(缓存 + 降级);`internal/providerresolve` 纯逻辑解析器 +
每协议一个映射器;claim 处一个姊妹钩子合并进 `custom_env`/`custom_args`,**daemon 零改动**;
前端目录页 + provider 选择器 + 模型 picker 按 `provider_ref` 收敛。

**Tech Stack:** Nacos 3.x 配置中心 · Go 1.26(Chi/sqlc/pgx)· Next.js 16 / React 19 · Docker selfhost · Vitest。

---

## 0. 背景 & 缺口

活体执行那轮的真实痛点:每个 agent 在 `custom_env` 重复配 `ANTHROPIC_BASE_URL` /
`ANTHROPIC_AUTH_TOKEN` / `ANTHROPIC_MODEL`(Claude),或在 `custom_args` 重复一长串
`-c model_providers.router.*`(Codex)。多 agent / 多 provider 时散配、易漂移、换 key 要逐 agent 改。
中心 provider 注册表把 provider 的 **shape** 收一处、版本化,agent 只引用。

## 1. 范围决议(brainstorm 定)

- **模型目录:增强** —— provider 携带 `models[]`,当模型目录源(不只连接配置)。
- **picker 收敛:provider_ref** —— agent 绑 `provider_ref` → 模型 picker 只显示该 provider 的
  `models[]`;**未绑 ref 的 agent 保持今天行为**(静态 `pkg/agent/models.go` / runtime 动态发现)。
- **协议范围:Claude + Codex** —— 两个映射器(`anthropic`、`codex-router`),映射器接口预留扩展。

**非目标(YAGNI)**:不做 OpenAI 直连 / Bedrock / Gemini 映射器(后续按需加);不彻底废除
`custom_env`/`custom_args`(共存 + 逃生口);不做计费 / 配额 / 限流;不做模型路由策略(路由器自己的事);
不动未绑 `provider_ref` 的 agent 的模型发现链路。

## 2. Nacos 资源模型(配置中心)

**为什么配置中心**:Nacos **AI Registry 的资源类型是 MCP / Prompt / Agent / AgentSpec / Skill,
没有原生 "LLM provider"**。provider shape 是典型的"版本化配置",落 **Nacos 配置中心(cs/config)**
最自然:每 provider 一个 dataId,配置中心天生带版本历史(当 version/tag),REST 成熟。

**存储约定**:
- `namespaceId` = workspace id(+ `shared`,同 N1 隔离)
- `group` = `forge-llm-providers`
- `dataId` = provider `name`
- `content` = `ProviderShape` JSON(下表)
- 版本 = Nacos 配置历史;`lifecycle` 作 content 字段

**`ProviderShape`(`server/internal/nacos/provider_types.go`,无密钥真值):**
```go
type ProviderShape struct {
    Name      string          `json:"name"`
    Namespace string          `json:"namespace,omitempty"` // list 时标注来源,register/get 不依赖
    Version   string          `json:"version"`
    Protocol  string          `json:"protocol"`  // "anthropic" | "codex-router"(loose:漂移降级)
    BaseURL   string          `json:"base_url"`  // 端点,非密钥
    AuthKey   string          `json:"auth_key"`  // 密钥 KEY 名(如 "ROUTER_API_KEY"),绝不存真值
    WireAPI   string          `json:"wire_api,omitempty"`  // codex 用,默认 "responses"
    Models    []ProviderModel `json:"models,omitempty"`
    Lifecycle string          `json:"lifecycle"` // "published" | "offline" | "draft"
}
type ProviderModel struct {
    ID      string `json:"id"`
    Label   string `json:"label,omitempty"`
    Default bool   `json:"default,omitempty"`
}
type ProviderRef struct { // agent.provider_ref
    Namespace string `json:"namespace"`
    Name      string `json:"name"`
    Ref       string `json:"ref"` // version 或 tag("stable"/"latest")
}
```

**`ProviderQuerier` 接口(`server/internal/nacos/provider_querier.go`)**:
```go
type ProviderQuerier interface {
    ListProviders(ctx context.Context, namespace string) ([]ProviderShape, error)
    GetProvider(ctx context.Context, namespace, name, ref string) (ProviderShape, error)
    RegisterProvider(ctx context.Context, namespace string, p ProviderShape) error
    SetProviderLifecycle(ctx context.Context, namespace, name, version, lifecycle string) error
}
```
- 真适配(`provider_client.go`)走 Nacos 配置中心 REST;`CachedProviderQuerier` 同 N1 包一层降级
  (`GetProvider` 成功回填、底层失败回退暖缓存)。
- **唯一探针驱动**:配置中心确切 REST(publish/get/list-by-group/history)实施时实测,记
  `server/internal/nacos/REST-providers.md`(同 N1 的 `REST.md`)。其余全 mock 可单测。

## 3. 数据层(Multica)

- 迁移 `NNN_agent_provider_ref`(NNN = 117):
  ```sql
  -- up:   ALTER TABLE agent ADD COLUMN provider_ref JSONB;        -- NULL = 不用 provider
  -- down: ALTER TABLE agent DROP COLUMN provider_ref;
  ```
  可空(不像 `mcp_refs` 的 `[]` 默认):provider 是单选,NULL 语义清晰("没绑")。
- `agent.sql` 的 `CreateAgent` / `UpdateAgent` 带 `provider_ref`(narg);`make sqlc` 重生成
  `db.Agent.ProviderRef []byte`。**无新 Multica 表**(Nacos 真相源)。

## 4. 解析层 `internal/providerresolve`(纯逻辑,镜像 `mcpresolve`)

```go
// types.go
type SecretSource interface{ Get(key string) (string, bool) } // agent.custom_env
type MapSecrets map[string]string

type Mapper interface {
    Protocol() string
    // Map 产出该协议 runtime 要的 env 增量 + args 增量(纯函数)。
    Map(shape nacos.ProviderShape, secrets SecretSource, model string) (env map[string]string, args []string, warnings []string)
}

type Input struct {
    Ref     *nacos.ProviderRef // agent.provider_ref(nil = 无)
    Secrets SecretSource       // agent.custom_env
    Model   string             // agent.model(决定 ANTHROPIC_MODEL / 默认模型)
}
type Result struct {
    Env   map[string]string // 合并进 agent.custom_env 的增量
    Args  []string          // 追加到 agent.custom_args 的增量
    Model string            // agent.model 为空时取 provider 默认模型
}

// resolve.go
func Resolve(ctx context.Context, q nacos.ProviderQuerier, in Input) (Result, []string, error)
// 内部:in.Ref==nil → 空 Result;取 shape;lifecycle!=published → 跳过+warn;
// 按 protocol 选 mapper(无 → 跳过+warn);mapper.Map → 填 Result.Env/Args;
// in.Model=="" 时从 shape.Models 取 default 填 Result.Model。
```

**映射器(`anthropic.go` / `codex.go`)产出(实现级)**:

`anthropicMapper`(→ Claude),`shape{base_url, auth_key}` + `model`:
```
env:  ANTHROPIC_BASE_URL  = shape.base_url
      ANTHROPIC_AUTH_TOKEN = secrets.Get(shape.auth_key)   // 缺 → ""+warn
      ANTHROPIC_MODEL      = model                          // model!="" 时
args: (无)
```

`codexRouterMapper`(→ Codex),`shape{name, base_url, auth_key, wire_api}`:
```
args: -c model_provider=<name>
      -c model_providers.<name>.base_url=<base_url>
      -c model_providers.<name>.wire_api=<wire_api|"responses">
      -c model_providers.<name>.env_key=<auth_key>
env:  <auth_key> = secrets.Get(auth_key)   // 缺 → ""+warn;codex 经 env_key 读它
```

> 这正是活体执行手工给 Inj 注 `ANTHROPIC_*`、给 CodexForge 注 `-c model_providers.router.*` 的产品化。

## 5. 派发钩子(claim 处,N1 钩子旁加姊妹)

`server/internal/handler/llm_registry.go`:
```go
func (h *Handler) resolveAgentProvider(
    ctx context.Context, agentID string, providerRef []byte,
    customEnv map[string]string, customArgs []string, model string,
) (mergedEnv map[string]string, mergedArgs []string, resolvedModel string)
```
- `h.Providers==nil`(无 Nacos)或 `providerRef` 空 → 原样返回(no-op)。
- 否则 `providerresolve.Resolve` → 合并:
  - **env**:provider `Result.Env` 先铺,**agent `customEnv` 覆盖同名 key**(逃生口);
  - **args**:provider `Result.Args` 先,agent `customArgs` 后(Codex `-c` 后者赢);
  - **model**:`model` 非空保留;空则用 `Result.Model`。
- warn 用 `slog.Warn` 记。

调用点:`ClaimTaskByRuntime`(`daemon.go`)在 `resolveAgentMcpConfig` 之后、装 `TaskAgentData` 之前:
```go
customEnv, customArgs, model = h.resolveAgentProvider(
    r.Context(), uuidToString(agent.ID), agent.ProviderRef, customEnv, customArgs, model)
```
**daemon 零改动**(还是读 `AgentData.CustomEnv`/`CustomArgs`/`Model`)。

## 6. API(镜像 `/api/mcp-registry/*`)

`server/internal/handler/llm_registry.go` + 路由 `server/cmd/server/router.go`(provider 即集合,
不像 N1 那样再套一层 `/servers`):
- `GET  /api/llm-providers`(列,ws+shared 合并、标注 namespace;成员门)
- `GET  /api/llm-providers/{name}`(成员门)
- `POST /api/llm-providers`(注册,**owner/admin** `requireWorkspaceRole`)
- `PUT  /api/llm-providers/{name}/lifecycle`(owner/admin)
- namespace 只能 `wsID` + `shared`,越界 403;`h.Providers==nil` → 503。
- 装配:`router.go` 在 `NACOS_SERVER_ADDR` 存在时 `h.Providers = nacos.NewCachedProviderQuerier(nacos.NewProviderClient(addr, idKey, idVal))`,否则 nil。

agent `provider_ref` 经现有 `PUT /api/agents/{id}` 读写(同 `mcp_refs`,**非密**):
- `AgentResponse.ProviderRef`(默认 `null`)+ `agentToResponse` 映射;
- `UpdateAgent` 经 rawFields 接收 `provider_ref`(`null` 清空、对象替换、缺省不动);
- `Agent` TS 类型加 `provider_ref?: ProviderRef | null`;`UpdateAgentRequest` 加
  `provider_ref?: ProviderRef | null`(`null` 显式清空)。

## 7. 前端

- `packages/core`:`types/llm-provider.ts`(`ProviderShape`/`ProviderRef`/`ProviderModel`/`ProviderList`)+
  `api/schemas.ts` zod(`.loose()` + string 枚举 + `EMPTY_PROVIDER_LIST`,过 `parseWithFallback`)+
  client(`listProviders`/`registerProvider`/`setProviderLifecycle`)+ `workspace/queries.ts` key+options。
- `packages/views/llm-providers/`:目录页(浏览/注册/lifecycle,沿用 `mcp-catalog` 模板;register
  收 name/version/protocol/base_url/auth_key(KEY 名)/models)。
- `packages/views/agents/.../inspector/provider-picker.tsx`:**单选** provider(`provider_ref`)+ 显示
  `auth_key` 缺-secret 预检(复用 N1 `missingSecrets` 思路,对单 key)。接进 agent overview。
- **模型 picker 收敛**:agent 有 `provider_ref` 时,模型 picker 数据源切到该 provider 的 `models[]`
  (来自 `getProvider`),而非 runtime 动态发现;无 ref → 不变。
- web 路由 `apps/web/.../llm-providers/page.tsx`(沿用 forge 惯例,无侧栏)。

## 8. 错误处理 / 降级 / 优先级 / 不变量

- **降级**:Nacos 配置中心不可达 → `CachedProviderQuerier` 回退暖缓存;解析失败绝不阻塞派发。
  provider 不存在 / `lifecycle!=published` → 跳过+warn,**回落 agent 今天的 env/args**。
- **枚举漂移**:`protocol` 不在 `{anthropic, codex-router}` → 无映射器 → 跳过+warn,不动现有字段。
- **缺 secret**:`auth_key` 不在 `custom_env` → 照常产出 env/args 但密值空 + warn;前端红字预检。
- **优先级(逃生口)**:env agent 显式 key 覆盖 provider;args provider 先 agent 后;model 两级
  (`agent.model` → `provider.default` → daemon 级 `MULTICA_<PROVIDER>_MODEL`)。
- **secret 红线**:配置中心只存 `auth_key` 名,shape 无密值字段;密值只在 Multica `custom_env`。
- **不变量**:`provider_ref` 空 / 无 Nacos → 与今天**字节级等价**;daemon 收到同样字段。

## 9. 测试策略(TDD)

- **映射器单测**(纯):anthropic→env、codex→args+env、缺-secret warn、未知 protocol 跳过。
- **`Resolve` 单测**(mock `ProviderQuerier`):ref→(env,args)、offline/not-found 跳过+warn、
  `CachedProviderQuerier` 降级。
- **handler 测**(一次性 PG,同 `mcp_registry_test`):`/api/llm-providers/*` 鉴权(成员 200/
  非成员 404/未配置 503/非 owner 注册 403/越界 namespace 403);`resolveAgentProvider` 钩子
  (provider_ref→合并正确;空 ref→不变;agent 覆盖赢);迁移 117 up/down/up 可逆。
- **前端**(vitest/jsdom):provider 选择器(选中→provider_ref、缺-secret 预检)、zod 降级、
  模型 picker 收敛。
- **绕凭证 e2e**(`//go:build integration`,真 Nacos 配置中心):注册 provider→设 ref→`Resolve`→
  断言 anthropic 的 `ANTHROPIC_*`(密值从模拟 custom_env 注入)/ codex 的 `-c model_providers.*` +
  密值 env;停 Nacos→走缓存。
- 守 CLAUDE.md 红线:响应过 zod `parseWithFallback`、handler 校验 workspace 归属、迁移可逆。

## 10. Phase 拆分(交 writing-plans)

| Phase | 名称 | 依赖 | 产出 |
|-------|------|------|------|
| P0 | 配置中心 spike + `ProviderQuerier` 接口/适配/缓存 + `agent.provider_ref` 迁移117+sqlc | — | 探针记 `REST-providers.md`;适配 + 接口 + 缓存降级 + DB 列 |
| P1 | `providerresolve` 解析器 + 两映射器(TDD,mock) | P0 | resolver + anthropic/codex 映射器 + 单测全绿 |
| P2 | `/api/llm-providers/*` + 鉴权 + `resolveAgentProvider` 钩子 + agent provider_ref API | P0,P1 | API + 钩子(空 ref=原行为,daemon 零改动)+ 单测 |
| P3 | 前端目录页 + provider 选择器 + 模型 picker 收敛 + zod | P2 | 三包 typecheck 绿 |
| P4 | 绕凭证 e2e + seed 示例 provider + 文档 | P0-3 | e2e 对真 Nacos 跑通 + seed + spec/memory 更新 |

## 11. 依赖 & 决策

- **依赖**:N0(基座)。复用 N1 的 resolver + secret 注入 + 缓存降级 + claim 钩子模式。
- **已决**:配置中心存 provider(非 AI Registry);单 `provider_ref`/agent;映射器每协议一套纯函数;
  picker 按 provider_ref 收敛;agent 显式 env/args 覆盖(逃生口);secret 留 Multica。
- **实施时实测定稿**:Nacos 配置中心 REST 的确切 path/payload(`REST-providers.md`)、配置历史
  当 version 的取法、list-by-group 的分页。
