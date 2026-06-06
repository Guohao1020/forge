# Prometheus — N2 LLM Provider / 模型中心注册表 实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: 用 superpowers:subagent-driven-development(推荐)
> 或 superpowers:executing-plans 逐任务实现。步骤用 checkbox(`- [ ]`)跟踪。

**Goal:** 把 LLM provider(端点 + 协议 + 模型目录 + 密钥 KEY 名)集中进 Nacos 配置中心,agent 按
`provider_ref` 引用;派发时服务端把 provider shape 解析成该 runtime 要的 env(Claude `ANTHROPIC_*`)
或 args(Codex `-c model_providers.*`)+ 从 Multica 注入密钥——daemon 零改动。

**Architecture:** 做法 A(Nacos 真相源 + 派发解析),**接口优先**:`providerresolve` 纯逻辑包依赖
`nacos.ProviderQuerier` 接口(mock 单测,不依赖真 Nacos);唯一对 Nacos 的出口是 `internal/nacos`
的**配置中心**适配(带缓存+降级);前端→Multica→Nacos;secret 留 Multica(`custom_env`),
Nacos 只存 shape。完全镜像 N1(iris)的 `mcpresolve` + claim 钩子。

**Tech Stack:** Nacos 3.x 配置中心 · Go 1.26(Chi/sqlc/pgx)· Next.js 16/React 19 · Docker selfhost · Vitest。

**Source spec:** [`docs/forge/specs/nacos-integration/N2-provider-model-registry.md`](../../specs/nacos-integration/N2-provider-model-registry.md)

**先例(直接照抄结构):** N1 已落地的 `internal/nacos/{client,cache,types}.go`、
`internal/mcpresolve/resolve.go`、`internal/handler/mcp_registry.go`、`daemon.go` claim 钩子、
迁移 116 + `agent.sql` 的 `mcp_refs`、前端 `mcp-catalog` + `inspector/mcp-picker.tsx`。

---

## Phase 表

| Phase | 名称 | Depends-on | 文件 |
|-------|------|-----------|------|
| 0 | 配置中心 spike + `ProviderQuerier` 接口/适配/缓存 + `agent.provider_ref` 迁移117 | — | [phase-0-base.md](phase-0-base.md) |
| 1 | `providerresolve` 解析器 + anthropic/codex 两映射器(纯逻辑,TDD,mock) | 0 | [phase-1-resolve.md](phase-1-resolve.md) |
| 2 | `/api/llm-providers/*` + `resolveAgentProvider` 钩子 + agent provider_ref API | 0,1 | [phase-2-api-hook.md](phase-2-api-hook.md) |
| 3 | 前端:目录页 + provider 选择器 + 模型 picker 收敛 + zod | 2 | [phase-3-frontend.md](phase-3-frontend.md) |
| 4 | 绕凭证集成验收 + seed + 文档 | 0-3 | [phase-4-verify.md](phase-4-verify.md) |

## 接口优先策略(为什么)

Nacos 配置中心的确切 REST(provider 用配置中心存,因 AI Registry 无 provider 类型)未实测。为不让
"外部未知"卡盘:
- **Phase 0 探针任务**:起 Nacos 3.x → `curl` 实测配置中心 publish/get/list-by-group/history 的
  path/payload → 记 `server/internal/nacos/REST-providers.md`。适配实现据此填。
- 其余一切(resolver、两映射器、handler、前端)依赖 `nacos.ProviderQuerier` 接口 + mock,
  **完全 code-complete、可单测**,不等真 Nacos。适配实现是唯一探针驱动的部分。

## 完成门禁(DoD)

- [ ] Nacos 配置中心可 publish/get/list provider config;`REST-providers.md` 记下实测接口
- [ ] `migration 117_agent_provider_ref` 可逆;sqlc 重生成读写 `provider_ref`
- [ ] `nacos.ProviderQuerier` 接口 + 配置中心适配(超时/重试/缓存/降级)
- [ ] `providerresolve.Resolve` 单测全绿(anthropic env / codex args / 缺 secret / offline 跳过 / 未知 protocol / 降级)
- [ ] `/api/llm-providers/*` handler(列/详情/注册/lifecycle)+ workspace 鉴权 + owner/admin 门 + 单测
- [ ] claim 处 `resolveAgentProvider`:provider_ref 空 = 原行为;agent 显式 env/args 覆盖;daemon 零改动
- [ ] 前端目录页 + provider 选择器(单选 + 缺-secret 预检)+ 模型 picker 按 provider_ref 收敛 + zod;三包 typecheck 绿
- [ ] **绕凭证集成实测**:起 Nacos → 注册 provider → 设 agent provider_ref → 调 resolver → 断言
  anthropic 出 `ANTHROPIC_*`(密值注入)/ codex 出 `-c model_providers.*` + 密值 env;停 Nacos → 走缓存

## 非目标(YAGNI,见 spec §1)

OpenAI 直连 / Bedrock / Gemini 映射器(后续按需加);废除 `custom_env`/`custom_args`(共存 + 逃生口);
计费/配额/限流;模型路由策略;动 未绑 `provider_ref` 的 agent 的模型发现链路。

## 分支提醒

N2 spec 已 commit 在 `feat/nacos-n0-n1-mcp-registry`(PR #1)。**N2 实现建议另起干净分支**
(等 PR #1 合从 main 切,或 stacked 分支),别混进 N1 的 PR。
