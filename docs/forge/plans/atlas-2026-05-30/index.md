# Atlas — F0 Foundation Plan (Forge on Multica)

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development
> (recommended) or superpowers:executing-plans to implement this plan task-by-task.
> Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 把 `Guohao1020/forge` 就地改造成 **Forge-on-Multica 开源底座**：以 Multica
为新 base（fork/rebase），本地全功能跑通，Forge 身份叠加，旧架构退役，能跟上游。

**Architecture:** Multica 提供 managed-agents 平台基础设施（任务看板、daemon 驱动 CLI 执行、
Skills、Squad、Autopilot、多租户、实时流）；Forge 后续在其上叠加 Harness 工程层。F0 只负责
让这个 base 立起来、跑起来、归 Forge 所有，不碰任何 harness 功能。

**Tech Stack:** Go 1.26（Chi + sqlc）· Next.js 16 + React 19 · PostgreSQL 17 · Redis ·
pnpm + turbo monorepo · 本地 daemon 驱动 `claude` CLI · 改良 Apache 2.0 许可。

**Source spec:** [`docs/specs/2026-05-30-forge-on-multica-f0-foundation-design.md`](../../specs/2026-05-30-forge-on-multica-f0-foundation-design.md)

---

## 决策链（来自 brainstorming 2026-05-30）

| # | 决策 | 选择 |
|---|------|------|
| D1 | 战略定位 | 以 Multica 为新底座（fork/rebase） |
| D2 | Agent 执行模型 | 拥抱 CLI 驱动，退役自建 ai-worker loop + Temporal |
| D3 | 第一切片 | F0 基座 |
| D4 | 分发模式 | Forge 开源（继承改良 Apache 2.0 + 保留 Multica 归属） |
| D5 | 仓库策略 | 就地改造 `Guohao1020/forge`，旧代码进 `archive/forge-legacy`，加 multica upstream remote |

---

## Phase 表

| Phase | 名称 | 任务数 | Depends-on | 状态 | 文件 |
|-------|------|--------|-----------|------|------|
| 0 | Pre-flight & Dev Environment | 5 | — | ☐ 未开始 | [phase-0-preflight-dev-environment.md](phase-0-preflight-dev-environment.md) |
| 1 | Repo Surgery | 6 | Phase 0 | ☐ 未开始 | [phase-1-repo-surgery.md](phase-1-repo-surgery.md) |
| 2 | Local Stand-up & E2E Acceptance | 4 | Phase 0, 1 | ☐ 未开始 | [phase-2-local-standup-e2e.md](phase-2-local-standup-e2e.md) |
| 3 | Doc Migration & Archive Reconciliation | 4 | Phase 1 | ☐ 未开始 | [phase-3-doc-migration.md](phase-3-doc-migration.md) |
| 4 | Forge Identity Layer | 5 | Phase 3 | ☐ 未开始 | [phase-4-forge-identity.md](phase-4-forge-identity.md) |
| 5 | Decommission & Final Acceptance | 4 | Phase 2, 4 | ☐ 未开始 | [phase-5-decommission-and-acceptance.md](phase-5-decommission-and-acceptance.md) |

---

## 关键执行顺序说明

- **Phase 0 先于一切不可逆动作**：在动 forge 仓库前，先用已克隆的 `D:\shulex_work\multica`
  证明 dev 路径（WSL2 vs Docker）能把 Multica 端到端跑通，锁定 R1（Windows dev）风险。
- **Phase 1 是唯一不可逆步骤**（force-push）：执行前把 `docs/specs/` 与 `docs/plans/` 备份到
  仓库外，且旧代码已进 `archive/forge-legacy`——双保险。
- **Phase 2 复用 Phase 0 锁定的 dev 路径**在 forge 仓库上跑 F0 核心验收闭环。
- Phase 3/4 把价值文档迁回新 main、叠加 Forge 身份。
- Phase 5 退役旧栈 + 更新记忆 + 逐条勾 DoD。

## 完成门禁（F0 DoD，来自 spec §2）

- [ ] 仓库：`main` = Multica base（descends from `upstream/main`），旧代码在
  `archive/forge-legacy`，`upstream` remote 配好，已 push
- [ ] 本地 e2e 闭环：登录 → 建 workspace → daemon 检测到 `claude` → 建 issue → assign
  Claude Code agent → 执行回报 → issue 进 review
- [ ] 身份：README/CLAUDE.md 重写；LICENSE + Multica 归属保留；价值文档迁移
- [ ] 退役：旧 forge-core/ai-worker/Temporal/constraint-worker 不再运行（仅存档）
- [ ] runbook：Windows 本地开发文档已写

## 后续切片（各自走独立 spec→plan）

F1 规范中心=Skills · F2 验证门禁 · F3 AI Review · F4 熵管理=Autopilot · F5 可观测闭环
