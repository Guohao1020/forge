# Asclepius — F4b 自愈闭环实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development
> (recommended) or superpowers:executing-plans to implement this plan task-by-task.
> Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 给熵扫描加 per-scan `auto_fix` 开关:开启则 scanner agent 能安全修的直接改 + 用 `gh` 开 PR、
修不了的建 issue;后端把 agent 开的 PR 录入 Forge sidecar 并 link 到 issue + 发系统评论。复用已就绪的
F2 门禁 + F3 评审(零新增)。建议性,人工合并。

**Architecture:** `forge_entropy_scan` 加 `auto_fix` 列 + 新 sidecar `forge_fix_pr`(issue↔pr_url 轻量 link,
幂等)。`forgeentropy.ComposeBrief` 增 `AutoFix` 分支(插修复指令段)。`service/forge_fix_pr.go`
`MaybeRecordFixPR`——CompleteTask 一处一行钩子,从 result 解析 `pr_url` → 插 `forge_fix_pr` + 系统评论
(best-effort、幂等、通用非熵专属,填 `pr_url` dead-end)。**不碰 GitHub-App 耦合的 `github_pull_request`**。
F2/F3 已对 issue-bound+有 workdir 的修复任务自动触发,不写一行 F2/F3 代码。Forge 隔离在 `forge_` 前缀。

**Tech Stack:** Go 1.26（Chi + sqlc + pgx/v5）· PostgreSQL 17 · Next.js 16 · Multica monorepo。
沿用 F1–F4 的 WSL2 Go + Docker selfhost.build 验证栈。

**Source spec:** [`docs/forge/specs/2026-06-01-f4b-self-healing-loop-design.md`](../../specs/2026-06-01-f4b-self-healing-loop-design.md)

> **复用既有(不改)**:F2 门禁 `daemon/daemon.go`(~2219 `completed && IssueID!=""`)、F3 评审
> `service/task.go:1091` `MaybeEnqueueReview`、系统评论 `service/task.go:1924` `createAgentComment`、
> `CompleteTask(ctx, taskID, result []byte, ...)`(result = `json.Marshal(TaskCompleteRequest{pr_url,...})`)。
> 注:`AgentTaskQueue` **无** WorkspaceID 字段 → 经 `GetIssue(task.IssueID).WorkspaceID` 取。
> 完成请求体**无 branch 字段** → `forge_fix_pr.branch` 留默认 ''。

---

## 决策链（brainstorming 2026-06-01）
同一 scanner 顺手修开一个 PR · 后端显式录入(Forge sidecar)· 建议性人工合并 · F2/F3 复用零新增。

## Phase 表

| Phase | 名称 | Depends-on | 状态 | 文件 |
|-------|------|-----------|------|------|
| 0 | 数据层（迁移 115：auto_fix 列 + forge_fix_pr 表 + sqlc） | — | ✅ | [phase-0-data-layer.md](phase-0-data-layer.md) |
| 1 | brief auto_fix 分支（forgeentropy，TDD） | Phase 0 | ✅ | [phase-1-brief.md](phase-1-brief.md) |
| 2 | PR 桥（forge_fix_pr.go + CompleteTask 钩子，TDD） | Phase 0 | ✅ | [phase-2-pr-bridge.md](phase-2-pr-bridge.md) |
| 3 | API + UI（auto_fix 开关透传） | Phase 0 | ✅ | [phase-3-api-ui.md](phase-3-api-ui.md) |
| 4 | 验收 + 文档 | Phase 1, 2, 3 | ✅ | [phase-4-verify.md](phase-4-verify.md) |

## 完成门禁（F4b DoD）
- [x] 迁移 115：`forge_entropy_scan.auto_fix` 列 + `forge_fix_pr` 表 + sqlc（CreateFixPR 幂等 + auto_fix 并入 entropy CRUD）
- [x] `ComposeBrief` `AutoFix` 分支单测(开启含修复段 + 断言 `report the PR URL` / 关闭逐字等于 F4)
- [x] `MaybeRecordFixPR`(service)解析 pr_url → 插 forge_fix_pr + 系统评论,CompleteTask 一处钩子,`go build`+vet 通过;`parseFixPRURL` 单测绿
- [x] `auto_fix` 透传 API(handler Create/Update,response 从 db 行回读)+ UI checkbox(三包 typecheck 绿)
- [x] **绕凭证集成(源码构建栈实测)**:建 auto_fix scan → `auto_fix=true` 往返;手动触发 → issue+task → 以 PAT 调 daemon complete 端点喂 `pr_url` → 断言 forge_fix_pr 行(pr_url=.../999)+ issue `🔧 Fix PR opened` 系统评论;**二次完成 → 仍 1 行 1 评论(幂等)**

## 已知约束
- **⚠️ 活体修复双重凭证门**:scanner agent 真修 + 真开 PR 需 ① provider 凭证(agent 跑)② execenv GitHub push auth(开 PR)。
  brief 合成 + PR 桥 + F2/F3 触发条件全绕凭证可验;活体修复延后。
- close-on-merge 推进 issue 走 GitHub App webhook(若装,凭 `Closes MUL-N`)或人工——不在本切片自包含范围。

## 后续
F5 可观测闭环(熵趋势 / 修复率 / PR 合并率)。auto-merge / F3 approve 裁决 / close-on-merge 自包含 后置。
