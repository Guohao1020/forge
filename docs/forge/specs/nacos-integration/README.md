# Nacos 接入 Multica AI 管理中心 —— 总体方案

> **状态**:设计阶段(2026-06-05)。本目录是 Nacos 集成的**完整、可拆分方案**:一份总览
> + 每个切片一份 spec。**N0+N1 为实现级详细设计;N2–N4 为架构级设计草案**(实施前各自按
> N0+N1 的方式深化:clarifying → approaches → 4 节设计 → 落 spec → writing-plans)。
>
> **安全**:本仓 `origin` 为**公开** GitHub。所有文档中凭证一律占位符
> (`<NACOS_USER>` / `<NACOS_PASSWORD>` / `<ROUTER_BASE_URL>` / `<ROUTER_API_KEY>`);
> 真值只进 env / DB,**永不写入 git**。

**Goal:** 把 [Nacos 3.x](https://nacos.io/docs/latest/manual/user/ai/ai-registry-overview/)
作为 Multica/Forge **AI 管理中心**的治理型注册 / 配置底座——分阶段补上 MCP 中心目录、
LLM provider 注册表、Prompt/Skill/AgentSpec 治理、daemon/runtime 服务发现与配置中心。

**Architecture:** Nacos 跑在 self-host Docker 栈;后端唯一一个 `internal/nacos` 适配包对接
Nacos AI Registry **REST API**(带缓存 + 降级);**前端只跟 Multica 说话**(Multica 套
workspace 鉴权再调 Nacos);Multica DB 存"引用",真相源按缺口大小落在 Nacos(新能力)
或 Multica(load-bearing 既有功能)。

**Tech Stack:** Nacos 3.x · Go 1.26(Chi + sqlc + pgx)· Next.js 16 / React 19 · Docker selfhost。

---

## 0. 背景:为什么 Nacos,补哪个缺口

Multica 本身是 AI-native 平台,**已自带一套"AI 注册"**——但它是 DB 内生、per-workspace、
缺中心目录 / 治理 / 版本化 / 生态互通。各块现状与重叠:

| 能力 | Multica 现状 | 缺口 | Nacos 能补什么 |
|------|-------------|------|----------------|
| MCP server | `agent.mcp_config`(per-agent、**无中心目录、无 UI**) | 大 | 中心编目 + 版本 + 发现(招牌) |
| LLM provider/模型 | `pkg/agent/models.go` 静态目录 + per-agent `custom_env` 散配 | 中 | 集中 provider 注册表 |
| Prompt/Skill/规范 | Forge Standards + Skills(DB) | 中 | 版本化治理 + 发布/灰度 + 多团队共享 |
| daemon/runtime 发现 | `agent_runtime` 表 + 心跳(**load-bearing**) | 小 | 标准化服务发现 + 动态配置 |

核心判断:**按缺口大小分阶段**——缺口大、重叠小的先做(MCP);load-bearing、重叠深的最后做
且只增强不替换(runtime 发现)。

## 1. Nacos AI Registry 是什么(摘要)

治理型的 **AI 资源注册中心**:把 MCP Server、Agent 元数据/端点、Prompt 模板、Skill 包/AgentSpec
当一等公民管,带 **namespace**(多租户)、**version**(latest/stable/canary)、**生命周期**
(draft→review→published→online/offline)、可见性/审批工作流;SDK/REST/MCP 接入,生态组件
(Dify、Spring AI、MCP Router)从它发现资源。核心是 **AI 资源的版本化治理与发现**。

## 2. 四能力 → 五切片(拆分与定序)

| 切片 | 名称 | 依赖 | 重叠/风险 | 真相源取向 | 状态 |
|------|------|------|-----------|-----------|------|
| **N0** | Nacos 基座(容器 + `internal/nacos` 适配包) | — | — | — | [N0+N1](N0-N1-mcp-registry.md) |
| **N1** | MCP Server 中心注册表 | N0 | 最小 | **Nacos 真相源** | [N0+N1](N0-N1-mcp-registry.md) ✅ 实现级 |
| **N2** | LLM provider / 模型注册表 | N0 | 中 | Nacos 真相源(shape)+ Multica 注密 | [N2](N2-provider-model-registry.md) 草案 |
| **N3** | Prompt/Skill/AgentSpec 治理 | N0,(参考 N1) | 中(与 Standards/Skills) | 待定(倾向 Nacos 治理 + Multica 注入) | [N3](N3-prompt-skill-agentspec-governance.md) 草案 |
| **N4** | daemon/runtime 服务发现 + 配置中心 | N0 | 最深(load-bearing) | **只增强/双写,不替换** | [N4](N4-runtime-discovery-config.md) 草案 |

**建议实施顺序**:N0+N1 →(N2 或 N3,看届时痛点)→ N4 最后。每切片各自走完整 brainstorm→spec→plan→实现。

## 3. 贯穿性架构原则(所有切片共用)

1. **前端 → Multica API → Nacos**:前端永不直连 Nacos;Multica 在前面套既有 workspace 鉴权。
2. **多租户 = namespace**:Nacos `namespace = workspace id` + 一个 `shared` 放 org 级公共资源;
   Multica 强制 workspace W 只能解析 `W ns + shared`(对齐"所有查询按 workspace_id 过滤")。
3. **Secret 永不进 Nacos**:Nacos 只存资源 **shape**(env/header 的 KEY 名),真值派发时从 Multica
   侧(agent `custom_env` / workspace 秘钥袋)注入。Nacos 是治理面,不是秘钥库。
4. **降级是硬约束**:`internal/nacos` 适配包是唯一对 Nacos 的出口,每调用带超时/有界重试/
   降级到缓存;**Nacos 故障绝不拖垮 Multica 核心**(目录不可用 ≠ agent 跑不了)。
5. **真相源按缺口定**:Multica 没有的(MCP 目录)让 Nacos 当真相源拿治理;Multica load-bearing
   的(runtime 心跳)坚决只增强不替换。
6. **消费侧零改动优先**:解析在服务端(claim/build task 时),对齐 F1 `InjectStandards` /
   F2 `ResolveChecks`;daemon/CLI 仍收同样的 payload,只是来源变成"解析自 Nacos"。

## 4. 验收哲学(沿用 F0–F5)

TDD + **绕凭证、源码构建栈集成验收**(不依赖活体 agent 跑):起 Nacos 容器 → 经 API 注册资源 →
设 agent 引用 → 调 resolver → 断言解析出的有效 payload。活体(真 agent 收到解析配置)花额度、延后。
守 CLAUDE.md 红线:API 响应过 zod `parseWithFallback`、handler 校验 workspace 归属、迁移可逆。

## 5. 跨切片风险与决议

- **Nacos 存储后端**:dev 用 standalone + 内嵌存储;prod 接外部库(MySQL/PG)、集群、HA —— 单列运维任务,不混进功能切片。
- **Nacos 版本依赖**:AI Registry 需 **3.x**;基座固定 3.x 镜像。
- **与上游 Multica 的关系**:本接入是叠加层,尽量 `internal/nacos` + `mcpresolve` 等独立包 + 一处 claim 钩子,**最小侵入**既有 agent/daemon 代码,便于跟 upstream rebase。
