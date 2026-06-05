# N0 + N1 — Nacos 基座 + MCP Server 中心注册表(实现级)

> 切片 1。隶属 [Nacos 接入总体方案](README.md)。本切片**实现级详细**,可直接转 writing-plans。

**Goal:** 在 self-host Docker 栈起 Nacos 3.x,用其 AI Registry 当 **MCP server 中心目录**(版本化、
多租户);agent 只存"引用",任务派发时服务端从 Nacos 解析出有效 `mcp_config` 给 daemon/CLI。

**Architecture:** 做法 **A**——Nacos 当 MCP 配置真相源 + 派发时解析。前端→Multica API→Nacos;
Multica DB 加 `agent.mcp_refs`(只存引用);secret 留 Multica;Nacos 故障降级到缓存。

**Tech Stack:** Nacos 3.x · Go(Chi/sqlc/pgx)· Next.js/React · Docker selfhost。

---

## 1. 架构与组件

**N0 — Nacos 基座**
- `docker-compose.selfhost.yml` 加 `nacos` 服务(**Nacos 3.x**,dev standalone + 内嵌存储;
  `MODE=standalone`,开鉴权)。
- 后端新增 `server/internal/nacos`:对 Nacos **AI Registry REST API** 的薄 Go 客户端
  (AI Registry 新、Go SDK 大概率未覆盖 → 走 REST)。
  - 接口:`ListMCPServers(ns) / GetMCPServer(ns,name,ref) / RegisterMCPServer(...) / SetLifecycle(...)`。
  - 内建:服务账号鉴权(`<NACOS_USER>`/`<NACOS_PASSWORD>` 从 env)、超时、有界重试、
    **解析缓存 + 降级**(Nacos 不可达 → 返回 last-known;全程不阻塞)。
  - 配置 env:`NACOS_SERVER_ADDR`、`NACOS_NAMESPACE_PREFIX`(可选)、`NACOS_USERNAME`、`NACOS_PASSWORD`。

**N1 — MCP 中心注册表组件**
- **MCP 目录 API** `server/internal/handler/mcp_registry.go`,路由 `/api/mcp-registry/*`:
  - `GET /api/mcp-registry/servers`(列本 workspace ns + shared)
  - `GET /api/mcp-registry/servers/{name}`(详情 + 版本列表)
  - `POST /api/mcp-registry/servers`(注册;owner/admin)
  - `PUT /api/mcp-registry/servers/{name}/lifecycle`(发布/下线;owner/admin)
  - 全部复用既有 workspace 鉴权中间件 + `resolveWorkspaceID`。
- **DB 迁移**:`ALTER TABLE agent ADD COLUMN mcp_refs JSONB NOT NULL DEFAULT '[]'`(新迁移号)。
  `mcp_refs = [{ "namespace": "<ws|shared>", "name": "voc-openapi", "ref": "stable" }]`。
  原 `mcp_config` **保留**(向后兼容 + 内联/覆盖)。
- **派发时解析器** `server/internal/mcpresolve`(对称 `internal/forge/checks` / `internal/forge/standards`,
  **service-free 纯逻辑包**):
  - `ResolveMCP(ctx, q NacosQuerier, secretSource, agent) (effectiveMCPConfig, error)`。
  - 逻辑:逐 ref → Nacos 取 shape(tag→具体版本)→ 从 `agent.custom_env` 注入 secret 值 →
    拼 `mcpServers` → 与内联 `mcp_config.mcpServers` 合并(**内联按 name 覆盖编目项**)→ 有效配置。
  - 缓存:per `(ns,name,version)` shape 缓存;降级取缓存。
- **接线钩子(一处)**:claim/build task 时(对齐 F1 在 claim 注入、F2 GetForgeChecks),
  把 `ResolveMCP` 的结果作为有效 `mcp_config` 放进 claim 响应。**daemon/CLI 零改动**。
- **前端**:
  - `packages/views/mcp-catalog/`(列表 + 注册框 + 版本/生命周期视图)+ web/desktop 路由。
  - agent inspector 加 **MCP 选择器**:勾选目录里的 server + 钉 ref → 写 `agent.mcp_refs`;
    显示版本、生命周期、**缺 secret 预检告警**。
  - `packages/core` 加 client 方法 + zod schema(`parseWithFallback`)。

## 2. 数据模型 + 数据流

**Nacos 资源(每个 MCP server 一条)**:`namespace`、`name`、`version`(semver)、`tags`、
`transport` + shape(stdio:`{command,args,env_keys[]}` / remote:`{url,header_keys[]}`)、
`tools`(展示用)、`lifecycle`、`visibility`。**只存 shape、不存 secret 值**。

**Multica**:`agent.mcp_refs`(引用列表,用户编辑)、`agent.mcp_config`(保留,内联/覆盖)。

**三阶段流**:
1. **注册**:owner UI → `POST /api/mcp-registry/servers` → 校验 + 写 Nacos(选 ns、版本、shape 无密、
   published)。
2. **选用**:编辑 agent → 选择器列目录 → 勾选 + 钉 ref → `PUT /api/agents/{id}` 写 `mcp_refs`(只存引用)。
3. **派发解析**:claim → `ResolveMCP`:逐 ref 取 shape(缓存)→ 注 secret(custom_env)→ 拼 mcpServers
   → 合并内联 → 有效 `mcp_config` 进 claim 响应 → daemon → CLI `--mcp-config`。

## 3. 错误处理 / 降级 / 多租户 / 鉴权

- **降级**:resolver 缓存 shape;Nacos 不可达用缓存,无缓存跳过该 ref + warn,**不拖垮派发**。
  目录 UI:Nacos 挂 → 横幅 + 读缓存 + 禁写。Multica 暴露 Nacos 连通状态。
- **解析边界**:ref 钉 tag → 派发时解析成版本(tag=跟新/版本=冻结);ref 指向缺失/未发布/下线 →
  跳过 + warn;缺 secret KEY → **选用时预检告警**,别等运行时。
- **多租户**:`namespace = workspace id` + `shared`;Multica 强制 workspace W 只解析 `W + shared`;
  scoping 在 Multica 层(非前端)。
- **鉴权**:后端→Nacos 服务账号(env,creds 只在后端);前端→Multica 复用 workspace 成员鉴权
  (注册/编辑 owner/admin,浏览/选用普通成员)。Nacos 开鉴权不裸跑。
- **失败隔离**:`internal/nacos` 是唯一对 Nacos 出口,超时/重试/降级全包在里面。

## 4. 测试 / 验收 + YAGNI

**测试**:
- Go:`internal/nacos`(mock Nacos 接口,仿 `CheckQuerier`)+ `internal/mcpresolve`(refs→config、tag、
  缺 ref、注 secret、合并、降级)+ `mcp_registry` handler(鉴权/owner-admin/坏输入)。TDD。
- 前端:目录视图 + MCP 选择器(编辑 refs / 版本 / 缺 secret 预检)+ catalog 响应 zod `parseWithFallback`。
- **绕凭证集成验收(源码构建栈)**:起 Nacos → API 注册 server → 设 `mcp_refs` → 调 resolver →
  断言有效 `mcp_config`(mcpServers 拼对、secret 注入、tag 解析)。降级测:停 Nacos → 走缓存仍出配置。
  活体(真任务收 `--mcp-config`)延后。

**YAGNI(切片 1 不做)**:N2/N3/N4;Nacos 审批工作流(只 published/offline);canary/百分比路由
(只 tag + 钉版本);**secret 写进 Nacos(永不)**;迁移/替换既有内联 `mcp_config`;生产级 Nacos
(外部库/集群/HA);从 Nacos 主动探活 MCP 端点;并进 F5 健康面板。

## 5. 文件级清单(转 writing-plans 用)

**新增**
- `docker-compose.selfhost.yml`(+ `selfhost.build.yml`):`nacos` 服务
- `server/internal/nacos/`:client.go(REST)、cache.go、types.go、client_test.go
- `server/internal/mcpresolve/`:resolve.go、resolve_test.go(service-free 纯逻辑)
- `server/internal/handler/mcp_registry.go` + `_test.go`
- `server/migrations/NNN_agent_mcp_refs.up/down.sql`
- `packages/views/mcp-catalog/`(列表/注册/详情)+ `packages/core` client + zod schema
- agent inspector 的 `mcp-picker.tsx`
- web + desktop 路由各一处

**修改(最小侵入)**
- `server/internal/handler/agent.go`:agent 响应/更新带 `mcp_refs`
- claim/build task 处一行钩子:`mcp_config = mcpresolve.ResolveMCP(...)`
- `pkg/db/queries/agent.sql` + sqlc 重生成(`mcp_refs` 读写)
- `server/cmd/server/router.go`:注册 `/api/mcp-registry/*` 路由

## 6. 已决议 & 待深化

**已决**:做法 A(Nacos 真相源 + 派发解析);secret 留 Multica;namespace=workspace+shared;
降级到缓存;消费侧零改动。

**待深化(实施时定)**:Nacos AI Registry REST 的确切 MCP 资源 schema(实测 3.x 版本接口)、
缓存的 DB 快照表是否需要(还是纯内存)、内联 vs 编目的覆盖优先级最终语义、注册到 `shared` 的权限门。
