# Eunomia — F4 熵管理 = Autopilot 实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development
> (recommended) or superpowers:executing-plans to implement this plan task-by-task.
> Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 让用户定义周期性全仓熵扫描，扫描派发时 Forge 把 scanner agent 的 brief 由 F1 规范 +
F2 检查 + 自定义聚焦 + 开放发现去重清单运行时合成，复用 Multica Autopilot 当调度/派发引擎，
为发现建建议性 issue。

**Architecture:** `forge_entropy_scan` sidecar 表（workspace→project 配 scanner agent + cron +
F1/F2 开关 + custom_focus + backing autopilot_id）。Forge 在 handler 侧代管一个 backing Autopilot
（schedule 触发=cron、execution_mode=create_issue、assignee=scanner agent）。一处 Forge 钩子在
`service.AutopilotService.dispatchCreateIssue`：反查 `GetEntropyScanByAutopilot`，命中则把
issue.description 覆写为 `forgeentropy.ResolveBrief`（合成 brief）。破环：`checks.go` 迁出为
`internal/forge/checks` 子包（对称已有 `standards/`），使服务端可零环 resolve F1+F2。Forge 隔离在
`forge_` 前缀。

**Tech Stack:** Go 1.26（Chi + sqlc + pgx/v5）· PostgreSQL 17 · Next.js 16 · Multica monorepo。
沿用 F1/F2/F3 的 WSL2 Go + Docker selfhost.build 验证栈。

**Source spec:** [`docs/forge/specs/2026-06-01-f4-entropy-autopilot-design.md`](../../specs/2026-06-01-f4-entropy-autopilot-design.md)

> **复用既有 Autopilot（不改）**：调度器 `cmd/server/autopilot_scheduler.go`（30s 轮询 + 原子 claim）、
> admission gate、run 追踪、失败监控全部复用。注入点 `service/autopilot.go:159`
> （`dispatchCreateIssue` 内 `description := s.buildIssueDescription(...)` 之后）。

---

## 决策链（brainstorming 2026-06-01）
advisory 建 issue · F1+F2 感知合成 brief · 单发现软去重 · Forge 代管 backing Autopilot · create_issue 派发。

## Phase 表

| Phase | 名称 | Depends-on | 状态 | 文件 |
|-------|------|-----------|------|------|
| 0 | 数据层（迁移 114 + sqlc：scan CRUD + GetByAutopilot + 开放发现查询） | — | ☐ | [phase-0-data-layer.md](phase-0-data-layer.md) |
| 1 | 破环子包化 + 合成逻辑（forge/checks 迁移 + forgeentropy，TDD） | Phase 0 | ☐ | [phase-1-compose.md](phase-1-compose.md) |
| 2 | 派发钩子（dispatchCreateIssue 注入合成 brief） | Phase 1 | ☐ | [phase-2-dispatch-hook.md](phase-2-dispatch-hook.md) |
| 3 | API + autopilot 代管 + UI | Phase 0 | ☐ | [phase-3-api-ui.md](phase-3-api-ui.md) |
| 4 | 验收 + 文档 | Phase 2, 3 | ☐ | [phase-4-verify.md](phase-4-verify.md) |

## 完成门禁（F4 DoD）
- [ ] `forge_entropy_scan` 迁移 114 + sqlc（CRUD + `GetEntropyScanByAutopilot` + `ListOpenEntropyFindings`）
- [ ] `checks.go` 迁 `internal/forge/checks` 子包，唯一调用点 `handler/forge_daemon.go` 改通，编译绿
- [ ] `forgeentropy.ComposeBrief` 纯单测（含/不含 standards/checks/custom/去重清单）
- [ ] `dispatchCreateIssue` 一处钩子注入合成 brief，`go build ./...` + vet 通过
- [ ] `/api/forge/entropy-scans` CRUD + 代管 backing autopilot（POST 建/PATCH 改/DELETE 删同步）
- [ ] UI `packages/views/forge-entropy/` + web 路由（三包 typecheck 绿）
- [ ] **绕凭证集成**：API 建 scan → 断言 backing autopilot+schedule 触发；手动触发 autopilot →
  断言 `issue.description` 含合成的 F1/F2/custom 段

## 已知约束
- **⚠️ 活体扫描依赖凭证**：scanner agent 真扫仓 + 经 CLI 建发现 issue 需 provider 凭证（同 F2/F3）。
  编排链（scan→autopilot→派发→合成→落 issue.description）绕凭证可验；活体扫描待凭证。
- scanner agent 必须能访问目标仓库 workdir（execenv 现成机制，复用）。

## 后续
F4b 自愈闭环（扫描 agent 直接修 + 开 PR + 自动过 F2 门禁 + F3 评审）→ F5 可观测闭环。
