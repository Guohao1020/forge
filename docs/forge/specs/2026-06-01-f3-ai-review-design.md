# Forge F3 · AI Review 设计

> Design spec — 2026-06-01
>
> **Topic:** coding task 完成后，Forge 自动建一个 review 子任务，assign 给项目配置的
> reviewer agent，复用 parent 的 work_dir；reviewer（Claude Code，经 F1 注入规范）跑
> `git diff` 评审 → findings 作为 review 评论回写 issue。建议性，不阻断。落地 Harness 的
> "AI Review" 层。
>
> **Scope:** 仅 **F3 v1 — 完成即评审、专用 reviewer、建议性评论**。评审门禁（阻断）、auto-fix、
> Squad 路由、PR/GitHub diff、结构化 verdict、多轮/多 reviewer、review 专用规范 category 均
> out of scope。
>
> **Engineering standard:** 硅谷级基建。one code path、Forge 代码隔离在 `forge_` 前缀，最小化
> 对 Multica 上游的侵入（R2）。复用 F1 的 InjectStandards 基础设施。

---

## 0. 背景

F3 是 Forge-on-Multica 路线图第四切片（F0 基座、F1 规范中心、F2 验证门禁已完成）。
F1 注入规范（软约束）、F2 跑命令门禁（硬约束）、F3 让 reviewer agent 读 diff 做 **AI 评审**
（软反馈）。

**核心洞察（来自探查）**：**评审就是另一个 agent 任务。** reviewer 是普通 Multica agent——
它 claim 时**自动走 F1 的 `forge.InjectStandards`**（daemon.go claim 钩子，F1 已实现）。所以
只要 reviewer 是个配好 instructions 的 agent，它就自动拿到评审准则，**零 daemon 改动复用 F1
基础设施**。

**Multica 现状**：
- `squad_member.role` 是自由字段；GitHub PR 表存在但**无 PR-review 概念**。
- `parent_task_id`（agent_task_queue）已存在（用于 retry）；retry 任务复用 session_id/work_dir。
- `TaskService.CompleteTask`（`server/internal/service/task.go:1020`）是触发后续任务的挂点
  （issue 状态由 agent 自管，平台不改）。
- `createAgentComment` 可回写评论；`comment.type` 是自由字段（可用 "review"）。

## 1. 决策链（brainstorming 2026-06-01）

| # | 决策 | 选择 |
|---|------|------|
| 1 | 评审编排 | **完成即评审 + 专用 reviewer agent**（CompleteTask 钩子建 review 子任务） |
| 2 | diff 送达 | **复用 parent work_dir**（reviewer 同机 git diff；单 daemon 场景） |
| 3 | 结果处理 | **建议性**（review 评论，不阻断；硬门禁是 F2 的活） |
| 4 | 评审准则 | **复用 F1 InjectStandards**（reviewer 也是 agent，claim 时自动注入） |

## 2. 核心概念 & 数据流

```
coding task 完成 (TaskService.CompleteTask)
  → forge.MaybeEnqueueReview(ctx, q, completedTask):
      若 task.context 含 forge_review marker → return（防循环）
      若 task.WorkDir == "" → return（无 diff 可评）
      reviewer = ResolveReviewer(workspace, project)；无 → return
      EnqueueReviewTask(reviewer.id, issueID, parent=task.id, workDir=task.WorkDir)
  → review 任务被 reviewer 的 daemon claim → F1 InjectStandards 自动注入规范（零改动）
  → daemon 复用 parent work_dir → reviewer 在同目录
  → reviewer agent: git diff → 按规范评审 → 发 review 评论
```

## 3. 数据模型（sidecar）

```sql
CREATE TABLE forge_review_config (
  id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  workspace_id      UUID NOT NULL REFERENCES workspace(id) ON DELETE CASCADE,
  project_id        UUID REFERENCES project(id) ON DELETE CASCADE,  -- NULL=workspace级
  reviewer_agent_id UUID NOT NULL REFERENCES agent(id) ON DELETE CASCADE,
  enabled           BOOLEAN NOT NULL DEFAULT TRUE,
  created_by        UUID,
  created_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at        TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX uq_forge_review_ws   ON forge_review_config (workspace_id) WHERE project_id IS NULL;
CREATE UNIQUE INDEX uq_forge_review_proj ON forge_review_config (project_id)   WHERE project_id IS NOT NULL;
```
迁移 113（实现时复查最大号）。不碰 Multica 表。

**解析**：project 级 reviewer 覆盖 workspace 级（每 scope 单一 reviewer）。

## 4. 触发 + EnqueueReviewTask（核心）+ 防循环

### 4.1 触发（`server/internal/forge/review.go`）
`MaybeEnqueueReview` 由 `CompleteTask` 调一行。纯逻辑 + repository 注入，便于单测：
- 跳过条件：review 任务自身（context 含 `forge_review:true`）、无 work_dir、无 reviewer 配置、
  无 issue。
- 否则建 review 任务。

### 4.2 EnqueueReviewTask
建 `agent_task_queue` 行：
- `agent_id` = reviewer_agent_id
- `issue_id` = 同 issue
- `parent_task_id` = coding task id
- `work_dir` = coding task 的 work_dir（**需 claim 钩子**：Multica 的 resume 是 per-(agent,issue)，
  reviewer ≠ coder 找不到 coder 的 session，故 claim handler 对 review 任务把
  `PriorWorkDir = task.work_dir` 覆盖一处。**非零 daemon 改动**——这是对原"复用现有机制"的细化）
- `context` = JSON `{"forge_review": true, "review_prompt": "Review the changes (run `git diff`).
  Apply the coding standards. Post your findings as comments."}`
- `status` = queued
→ 入队后由 reviewer 的 runtime daemon 认领。

### 4.3 防循环
review 任务的 context 标 `forge_review:true`；4.1 的触发钩子跳过带此标记的任务完成——评审任务
完成不会再触发评审。

## 5. reviewer 执行（多数复用，+1 处 claim 钩子）

- review 任务 claim 时 → **F1 的 `InjectStandards` 自动注入规范**（reviewer 是 agent，走同样
  claim 钩子，零改动）。
- **work_dir 复用需 claim 钩子**（细化 §4.2）：claim handler 对 review 任务把
  `PriorWorkDir = review 任务的 work_dir`（= coder 的 work_dir）覆盖，reviewer 在同目录。
- reviewer agent（Claude Code）：`git diff` 看改动 → 按注入规范评审 → 发评论。
- 评审行为 = reviewer 自己的 `instructions`（身份，用户配）+ 注入的规范（查什么）。

## 6. 结果（建议性评论）

reviewer 的 findings 作为 issue 评论回写（reviewer agent 用 `multica issue comment` CLI 自发，或
CompleteTask 合成评论）。建议性——不卡 issue 流转。硬门禁是 F2 的职责。

## 7. API + UI

- **API**：`/api/forge/review-config` GET/PUT（按 workspace/project 配 reviewer agent + enabled）。
- **UI**：设置面选 reviewer agent + 开关（轻量；镜像 F1 配置面）。

## 8. Upstream 隔离（R2）

`forge_review_config` 表 · `server/internal/forge/review.go` · `server/internal/service/forge_review.go` ·
`server/internal/handler/forge_review.go` · `/api/forge/review-config` · UI。**侵入点**：
`CompleteTask` 一处 `s.MaybeEnqueueReview(...)` + claim handler 一处 `PriorWorkDir` 钩子 + 路由几行。

## 9. 测试

- **Go 单测（绕凭证）**：`ResolveReviewer`（project 覆盖 workspace）；`MaybeEnqueueReview` 逻辑
  （喂 completed coding task + 配置 reviewer → 断言建 review 任务：agent=reviewer、parent 对、
  marker、work_dir 复用；review 任务自身完成 → 不再触发，防循环）。用 fake querier。
- **⚠️ 活体评审 e2e 需凭证**：reviewer agent 真跑 git diff + 评审 + 评论，**需可用 provider
  凭证**（F3 比 F2 更依赖——评审本质是 LLM agent）。编排（触发→建 review 任务）可单测验证；
  评审输出待凭证。

## 10. 边界（F3 v1 不做）

评审门禁（阻断）· auto-fix · Squad 路由 · PR/GitHub diff（用 work_dir）· 结构化 verdict 解析 ·
多 reviewer/多轮 · review 专用规范 category（复用 F1 规范）· reviewer 跨 runtime（同机约束）。

## 11. 风险

| | 风险 | 缓解 |
|---|---|---|
| R1 | **F3 重度依赖凭证**（评审=LLM agent） | 编排层单测全覆盖；活体评审待凭证。F3 固有，非缺陷 |
| R2 | reviewer 必须同 runtime（work_dir 本地） | v1 单 daemon 场景 OK；多 runtime 留待 diff-capture（已记边界） |
| R3 | 触发循环（review 触发 review） | context `forge_review` marker，钩子跳过 |
| R4 | CompleteTask 钩子致 upstream 冲突 | 一行 `MaybeEnqueueReview` 调用，逻辑在 forge 包 |

> F3 完成后，下一切片 **F4 熵管理 = Autopilot**（定时质量扫描 + 自动修复 issue + 趋势）。
