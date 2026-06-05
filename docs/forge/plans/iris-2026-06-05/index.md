# Iris — N0+N1 Nacos 基座 + MCP 中心注册表 实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: 用 superpowers:subagent-driven-development(推荐)
> 或 superpowers:executing-plans 逐任务实现。步骤用 checkbox(`- [ ]`)跟踪。

**Goal:** 在 self-host Docker 栈起 Nacos 3.x,用其 AI Registry 当 MCP server 中心目录;agent 只存
引用(`mcp_refs`),任务派发时服务端从 Nacos 解析出有效 `mcp_config` 给 daemon/CLI——daemon 零改动。

**Architecture:** 做法 A(Nacos 真相源 + 派发解析)。**接口优先**:`mcpresolve` 纯逻辑包依赖
`NacosQuerier` 接口(mock 单测,不依赖真 Nacos);唯一对 Nacos 的出口 `internal/nacos` 适配包带
缓存+降级;前端→Multica→Nacos;secret 留 Multica(custom_env),Nacos 只存 shape。

**Tech Stack:** Nacos 3.x · Go 1.26(Chi/sqlc/pgx)· Next.js 16/React 19 · Docker selfhost · Vitest。

**Source spec:** [`docs/forge/specs/nacos-integration/N0-N1-mcp-registry.md`](../../specs/nacos-integration/N0-N1-mcp-registry.md)

---

## Phase 表

| Phase | 名称 | Depends-on | 文件 |
|-------|------|-----------|------|
| 0 | Nacos 基座(容器 + API 探针)+ `agent.mcp_refs` 迁移 + `NacosQuerier` 接口 + 适配实现 | — | [phase-0-base.md](phase-0-base.md) |
| 1 | `mcpresolve` 解析器(纯逻辑,TDD,mock) | 0 | [phase-1-resolve.md](phase-1-resolve.md) |
| 2 | MCP 目录 API + 派发解析钩子 | 0,1 | [phase-2-api-hook.md](phase-2-api-hook.md) |
| 3 | 前端:目录视图 + MCP 选择器 + zod | 2 | [phase-3-frontend.md](phase-3-frontend.md) |
| 4 | 绕凭证集成验收 + 文档/seed | 0-3 | [phase-4-verify.md](phase-4-verify.md) |

## 接口优先策略(为什么)

Nacos 是 greenfield、AI Registry REST schema 未实测。为不让"外部未知"卡住整盘:
- **Phase 0 探针任务**:起 Nacos 3.x 容器 → 用 `curl` 实测 AI Registry 的 MCP 资源 REST 端点/载荷
  → 记进 `internal/nacos/REST.md`。`internal/nacos` 适配实现据此填具体 path/payload。
- 其余一切(resolver、handler、前端)依赖 `NacosQuerier` 接口 + mock,**完全 code-complete、可单测**,
  不等真 Nacos。适配实现是唯一探针驱动的部分。

## 完成门禁(DoD)

- [ ] Nacos 3.x 容器在 `forge-build` 栈起得来、开鉴权;`internal/nacos/REST.md` 记下实测接口
- [ ] `migration NNN_agent_mcp_refs` 可逆;sqlc 重生成读写 `mcp_refs`
- [ ] `NacosQuerier` 接口 + `internal/nacos` 适配实现(超时/重试/缓存/降级)
- [ ] `mcpresolve.ResolveMCP` 单测全绿(refs→config / tag / 缺 ref / 注 secret / 合并内联 / 降级)
- [ ] `/api/mcp-registry/*` handler(列/详情/注册/lifecycle)+ workspace 鉴权 + owner/admin 门 + 单测
- [ ] claim 处一行钩子:有效 `mcp_config = ResolveMCP(...)`;daemon 零改动
- [ ] 前端目录视图 + MCP 选择器(编辑 refs/版本/缺 secret 预检)+ zod `parseWithFallback`;三包 typecheck 绿
- [ ] **绕凭证集成实测**:起 Nacos → API 注册 server → 设 agent refs → 调 resolver → 断言有效
  `mcp_config`(mcpServers 拼对、secret 注入、tag→版本);停 Nacos → 走缓存仍出配置

## 非目标(YAGNI,见 spec §4)

N2/N3/N4;Nacos 审批工作流;canary/百分比路由;secret 写进 Nacos;迁移既有内联 `mcp_config`;
生产级 Nacos(外部库/集群/HA);主动探活 MCP 端点;并进 F5 面板。
