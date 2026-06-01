# Cerberus — F2 验证门禁（Verification Gates）实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development
> (recommended) or superpowers:executing-plans to implement this plan task-by-task.
> Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** agent 会话结束后，daemon 在 workdir 跑项目配置的验证命令（lint/test/build）；任一非零
退出 → 任务标失败（`failure_reason=verification_failed`）+ 回写评论，不让坏活静默通过。

**Architecture:** `forge_check` sidecar 表（workspace→project，加法合并）。daemon 在 `handleTask`
的 `reportTaskResult` 前插一处调用 `forgeVerify`：拉 checks（新 daemon 端点）→ 在 workdir 跑 →
失败则把 result 转为 FailTask 路径（`FailureReason=verification_failed` + 失败详情作 Comment，
现有 FailTask 自动回写评论）。Forge 代码隔离在 `forge_` 前缀。

**Tech Stack:** Go 1.26（Chi + sqlc + pgx/v5 + os/exec）· PostgreSQL 17 · Next.js 16 + React 19 ·
Multica monorepo。沿用 F1（themis）已建的 WSL2 Go 工具链 + Docker selfhost.build 验证栈。

**Source spec:** [`docs/forge/specs/2026-06-01-f2-verification-gates-design.md`](../../specs/2026-06-01-f2-verification-gates-design.md)

---

## 决策链（brainstorming 2026-06-01）
daemon 侧 post-session 挂点 · 失败=阻断+评论（v1，auto-fix 后续）· 命令式检查（forge_check，
workspace→project 加法）。

## Phase 表

| Phase | 名称 | Depends-on | 状态 | 文件 |
|-------|------|-----------|------|------|
| 0 | 数据层（迁移 112 + sqlc） | — | ✅ 完成 | [phase-0-data-layer.md](phase-0-data-layer.md) |
| 1 | 解析逻辑（forge/checks，TDD） | Phase 0 | ✅ 完成（2 单测） | [phase-1-resolve.md](phase-1-resolve.md) |
| 2 | daemon 验证（端点 + forge_verify + hook） | Phase 1 | ✅ 完成（runChecks 2 单测 + 编译） | [phase-2-daemon-verify.md](phase-2-daemon-verify.md) |
| 3 | API + UI（checks CRUD + views） | Phase 0 | ✅ 完成（三包 typecheck 绿） | [phase-3-api-ui.md](phase-3-api-ui.md) |
| 4 | 验收 + e2e + 文档 | Phase 2, 3 | ◑ 逻辑全验；活体门禁 e2e 待凭证 | [phase-4-verify.md](phase-4-verify.md) |

> **F2 门禁逻辑完成（2026-06-01）。** ResolveChecks + runChecks 单测绿；daemon 端点 +
> handleTask 钩子编译通过；checks CRUD + UI（三包 typecheck 绿）。**活体门禁 e2e**（真 agent
> 完成→拦截→评论）需可用 provider 凭证，延后（见 spec §8/R4）。

## 与 F1（themis）的关系
Phase 0/1/3 紧密镜像 F1 的对应 phase（migration/sqlc、resolve 纯函数、API/UI 镜像
forge-standards）。**Phase 2 是 F2 独有**——daemon 真的跑命令（F1 daemon 侧零改动）。

## 完成门禁（F2 DoD）
- [ ] `forge_check` 迁移 112 + sqlc
- [ ] `ResolveChecks()` 单测（workspace+project 加法合并）
- [ ] daemon 端点 `GET /api/daemon/tasks/{id}/forge-checks` + `GetForgeChecks` client
- [ ] `forge_verify.go` 跑命令 + handleTask 一处 hook；单测（一过一挂 → 失败详情）
- [ ] `/api/forge/checks` CRUD + `packages/views/forge-checks/`
- [ ] **e2e（绕凭证）**：配 `exit 1` check → issue → daemon 验证失败 → task failed
  （verification_failed）+ issue 上有评论

## 后续
F3 AI Review。auto-fix loop / 规则引擎 / profile 过滤 checks 均后置（见 spec §9）。
