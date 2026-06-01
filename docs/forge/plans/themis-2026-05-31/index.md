# Themis — F1 规范中心（Standards）实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development
> (recommended) or superpowers:executing-plans to implement this plan task-by-task.
> Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 在 Multica 之上加 Forge 规范中心：把分类/分级的编码规范（Standards）解析成
"有效规范"，双层注入 Claude Code agent —— 核心进 instructions（常驻强制），详细编译成
按需 skill。

**Architecture:** Forge 代码全部隔离在 `forge_` 前缀（sidecar 表 / `server/internal/forge/`
包 / `/api/forge/*` 路由 / `packages/views/forge-standards/`）。唯一侵入 Multica 的点是
daemon claim handler 里一行 `forge.InjectStandards(...)`。detail-skill 走 Multica 现有
execenv skill 注入（daemon 零改动）。

**Tech Stack:** Go 1.26（Chi + sqlc + pgx/v5）· PostgreSQL 17 · Next.js 16 + React 19 +
Base UI/shadcn + TanStack Query · Multica monorepo（pnpm + turbo）。

**Source spec:** [`docs/forge/specs/2026-05-31-f1-spec-center-standards-design.md`](../../specs/2026-05-31-f1-spec-center-standards-design.md)

---

## 决策链（brainstorming 2026-05-31）

结构化治理层 · 双层注入（core→instructions / detail→skill）· 只治 Standards ·
workspace→project 两级 · 项目画像驱动过滤。

## Phase 表

| Phase | 名称 | Depends-on | 状态 | 文件 |
|-------|------|-----------|------|------|
| 0 | Build env + 数据层（迁移 + sqlc） | — | ✅ 完成 | [phase-0-data-layer.md](phase-0-data-layer.md) |
| 1 | 解析逻辑（forge/standards，TDD） | Phase 0 | ✅ 完成（4 单测绿） | [phase-1-resolve.md](phase-1-resolve.md) |
| 2 | 双层注入钩子（InjectStandards + claim hook） | Phase 1 | ✅ 完成（2 单测绿 + e2e 证明） | [phase-2-inject.md](phase-2-inject.md) |
| 3 | API（CRUD + project profile） | Phase 0 | ✅ 完成（源码构建服务器往返） | [phase-3-api.md](phase-3-api.md) |
| 4 | UI（forge-standards views + web 路由） | Phase 3 | ☐ 待做（需 pnpm install） | [phase-4-ui.md](phase-4-ui.md) |
| 5 | 验收 + 种子 + 文档 | Phase 2, 4 | ◑ 后端 e2e 已验；UI 验收待 P4 | [phase-5-verify.md](phase-5-verify.md) |

> **后端 F1 端到端验证通过（2026-06-01）。** 源码构建 backend → daemon 认领 →
> `forge.InjectStandards` 触发 → 解析 standard → 写入 agent execenv 的
> `.claude/skills/forge-standards/SKILL.md`（内容为解析后的 standard）。迁移 111 经 migrate
> 工具在全新 DB 按序应用；`/api/forge/standards` 在源码构建服务器往返成功。
> **剩 Phase 4 UI**（前端列表/双栏编辑/profile，需 pnpm install）。

## 关键前置：构建/测试环境

F1 是真代码（Go 编译 + 测试），不同于 F0。需要 Go 工具链跑 `go test` / `make sqlc`。
F0 走的是 Docker selfhost（无 host Go）。**Phase 0 第一步建立构建环境**：WSL2 装 Go 1.26 +
pnpm（用户级免 sudo），或用 `docker-compose.selfhost.build.yml` 在 Docker 内构建。
TDD 迭代推荐 WSL2 原生 Go。

## 完成门禁（F1 DoD）

- [ ] `forge_standard` + `forge_project_profile` 迁移 + sqlc 生成代码
- [ ] `forge.ResolveStandards()` 单测通过（覆盖/过滤/拆分/降级）
- [ ] `forge.InjectStandards()` + daemon claim 钩子；集成测试断言 instructions 含 core、
  Skills 含 forge-standards
- [ ] `/api/forge/standards` CRUD + `/api/forge/projects/{id}/profile` 可用
- [ ] `packages/views/forge-standards/` 列表 + 双栏编辑 + profile 编辑
- [ ] `make test`（Go）通过；前端 typecheck 通过

## 已知约束

- **provider 凭证阻塞**（F0 遗留）：无法用真 agent 端到端验证。F1 用单测 + 集成测试 +
  手动 API 验证（对 selfhost 栈）覆盖；真 agent 跑 standards 注入待凭证就绪。
- **upstream 合并**：唯一侵入点是 daemon.go 一行 + 路由注册几行，隔离在 forge 包。

## 后续

F1 完成后 → **F2 验证门禁**（约束引擎卡任务完成）。
