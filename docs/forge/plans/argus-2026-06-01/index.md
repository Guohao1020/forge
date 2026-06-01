# Argus — F5 可观测闭环实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development
> (recommended) or superpowers:executing-plans to implement this plan task-by-task.
> Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 把 F1–F4b 的 Harness 活动 compute-on-read 聚合成 **Forge 健康面板**:快照卡 + 趋势图 + 混合钻取。
**零新追踪表 / 零迁移** —— 纯聚合既有表。

**Architecture:** 新 `server/pkg/db/queries/forge_health.sql`(聚合 + 趋势 + 钻取 query,全 compute-on-read)
+ `server/internal/handler/forge_health.go`(5 个 `/api/forge/health*` 端点,镜像 `dashboard.go` 的
`resolveWorkspaceID`/`parseProjectIDParam`/`resolveViewingTZ`/`parseSinceParamInTZ`)+ `packages/views/forge-health`
(快照卡 + recharts 趋势 + 混合钻取,镜像 `dashboard-page.tsx`)+ core 接线 + **health response zod schema**。

**Tech Stack:** Go 1.26（Chi + sqlc + pgx/v5）· PostgreSQL 17 · Next.js 16 · recharts 3.8.0 · Multica monorepo。
沿用 F1–F4b 的 WSL2 Go + Docker selfhost.build 验证栈。

**Source spec:** [`docs/forge/specs/2026-06-01-f5-observability-design.md`](../../specs/2026-06-01-f5-observability-design.md)

> **复用既有(不改)**:`dashboard.go` 的 handler skeleton（`h.resolveWorkspaceID(r)` → `h.workspaceMember(w,r,ws)`
> → `parseProjectIDParam(w,r)` → `tz := h.resolveViewingTZ(r)` → `since := parseSinceParamInTZ(r, 30, tz)`）;
> 查询风格 `DATE(col AT TIME ZONE sqlc.arg('tz')::text)` / `COUNT(*) FILTER (WHERE ...)::int` /
> `COALESCE(SUM/AVG(...),0)`;`agent_task_queue` 无 workspace_id → `JOIN agent a ON a.id=atq.agent_id WHERE a.workspace_id=$1`
> + `LEFT JOIN issue i ON i.id=atq.issue_id` 做 project narg 过滤。**F5 无迁移**。

---

## 决策链（brainstorming 2026-06-01）
完整套件(快照+趋势+钻取)· 混合钻取(能链就链 + 小面兑底)· PR 合并率优雅降级 · compute-on-read 零新表。

## Phase 表

| Phase | 名称 | Depends-on | 状态 | 文件 |
|-------|------|-----------|------|------|
| 0 | 快照后端（forge_health.sql 聚合 + GetForgeHealth 端点） | — | ✅ | [phase-0-snapshot.md](phase-0-snapshot.md) |
| 1 | 趋势 + 钻取后端（trend/drill query + 4 端点） | Phase 0 | ✅ | [phase-1-trends-drill.md](phase-1-trends-drill.md) |
| 2 | 前端 core 接线 + 快照卡 + zod schema | Phase 0 | ✅ | [phase-2-frontend-snapshot.md](phase-2-frontend-snapshot.md) |
| 3 | 前端趋势图(recharts) + 钻取面板 | Phase 1, 2 | ✅ | [phase-3-frontend-trends-drill.md](phase-3-frontend-trends-drill.md) |
| 4 | 验收 + 文档 | Phase 1, 2, 3 | ✅ | [phase-4-verify.md](phase-4-verify.md) |

## 完成门禁（F5 DoD）
- [x] `forge_health.sql` 9 个快照聚合（配置 COUNT + 门禁/评审/findings/scan-runs/fix-PR 窗口聚合）+ `GetForgeHealth` 端点
- [x] 趋势 3（熵发现/门禁通过/修复 PR 按天）+ 钻取（gate-failures/fix-prs list + findings 复用）+ 4 端点
- [x] 前端 `forge-health` view:快照卡 + recharts 趋势(3 图) + 混合钻取(findings/gate 内嵌、fix-PR 外链) + **health response zod schema**;三包 typecheck 绿 + core 413 单测绿
- [x] **绕凭证集成(源码构建栈实测)**:`GET /api/forge/health` → standards=1/checks=1/reviewers=1/scans=1 配置非零、gate pass=1、review total=1、**fix_prs opened=1 / merged=0 / matched=0(优雅降级)**、scan_runs=1 → `SNAPSHOT_REAL=True`;`/trends` 返回 date-bucketed(gate/fixpr 各 1 天);drill fix-prs=1(含 .../pull/999)。**纯读聚合 F1–F4b 留下的真实数据,无需 agent 跑**

## 已知约束
- **PR 合并率 webhook 门**:`forge_fix_pr.pr_url ↔ github_pull_request.html_url` 文本 join;无 webhook 数据(`matched=0`)→ UI 显「— · 需 GitHub App」。
- **满负荷真实数字**(高门禁/评审/扫描/修复量)需持续 live 活动(凭证);但 F5 查询 + 面板本身**完全绕凭证可验**(纯读聚合)。

## 后续
Harness 健康总分 / 阈值告警 / 跨 workspace / materialized rollup（趋势变慢时）。
**F5 落地 → Forge 路线图 F0–F5 全闭环**:声明→门禁→评审→扫描→自愈→观测。
