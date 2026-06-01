# Momus — F3 AI Review 实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development
> (recommended) or superpowers:executing-plans to implement this plan task-by-task.
> Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** coding task 完成后，Forge 自动建一个 review 子任务给项目配置的 reviewer agent，复用
parent 的 work_dir；reviewer（经 F1 注入规范）跑 `git diff` 评审 → 评论。建议性。

**Architecture:** `forge_review_config` sidecar 表（workspace→project 配 reviewer）。
`TaskService.CompleteTask` 一处钩子 `MaybeEnqueueReview` → 建 review 任务（新 query
`CreateForgeReviewTask`，带 parent_task_id + work_dir + forge_review marker 防循环）。
claim handler 一处钩子：review 任务把 `PriorWorkDir` 设为该任务的 work_dir（让 reviewer 复用
coder 的 workdir）。reviewer claim 时自动走 F1 的 InjectStandards（零改动）。Forge 隔离在
`forge_` 前缀。

**Tech Stack:** Go 1.26（Chi + sqlc + pgx/v5）· PostgreSQL 17 · Next.js 16 · Multica monorepo。
沿用 F1/F2 的 WSL2 Go + Docker selfhost.build 验证栈。

**Source spec:** [`docs/forge/specs/2026-06-01-f3-ai-review-design.md`](../../specs/2026-06-01-f3-ai-review-design.md)

> **对 spec §4.2 的细化**：work_dir 复用**非零 daemon 改动**——Multica 的 resume 是
> per-(agent,issue)，F3 要跨 agent 把 coder 的 workdir 给 reviewer，需 claim handler 一处
> `PriorWorkDir` 钩子（见 Phase 2）。spec 已据此修订。

---

## 决策链（brainstorming 2026-06-01）
完成即评审 + 专用 reviewer agent · 复用 parent work_dir · 建议性评论 · 复用 F1 InjectStandards。

## Phase 表

| Phase | 名称 | Depends-on | 状态 | 文件 |
|-------|------|-----------|------|------|
| 0 | 数据层（迁移 113 + sqlc + review-task query） | — | ☐ | [phase-0-data-layer.md](phase-0-data-layer.md) |
| 1 | 解析 + 触发逻辑（forge/review + service） | Phase 0 | ☐ | [phase-1-review-logic.md](phase-1-review-logic.md) |
| 2 | 接线（CompleteTask 钩子 + claim PriorWorkDir 钩子） | Phase 1 | ☐ | [phase-2-wiring.md](phase-2-wiring.md) |
| 3 | API + UI（review-config CRUD + 设置面） | Phase 0 | ☐ | [phase-3-api-ui.md](phase-3-api-ui.md) |
| 4 | 验收 + 文档 | Phase 2, 3 | ☐ | [phase-4-verify.md](phase-4-verify.md) |

## 完成门禁（F3 DoD）
- [ ] `forge_review_config` 迁移 113 + sqlc + `CreateForgeReviewTask` query
- [ ] `ResolveReviewer` + `IsReviewTask` + `ShouldEnqueueReview` 单测
- [ ] `MaybeEnqueueReview`（service）建 review 任务（agent=reviewer、parent、work_dir、marker）
- [ ] CompleteTask 一处钩子 + claim PriorWorkDir 钩子，编译通过
- [ ] `/api/forge/review-config` GET/PUT + UI（三包 typecheck 绿）
- [ ] **编排单测验证**（绕凭证）：completed coding task + reviewer 配置 → 断言建 review 任务；
  review 任务完成 → 不再触发（防循环）

## 已知约束
- **⚠️ F3 重度依赖凭证**：活体评审（reviewer 真跑 git diff + 评审 + 评论）需 provider 凭证。
  编排层单测全覆盖；活体待凭证（见 spec §11/R1）。
- reviewer 必须同 runtime（work_dir 本地）——单 daemon 场景 OK。

## 后续
F4 熵管理=Autopilot。评审门禁 / auto-fix / Squad 路由 / PR diff 后置。
