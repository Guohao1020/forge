# Forge F2 · 验证门禁（Verification Gates）设计

> Design spec — 2026-06-01
>
> **Topic:** 在 Multica 上加 Forge 的"验证门禁"——agent 会话结束后，daemon 在 workdir 跑
> 项目配置的验证命令（lint/test/build）；任一失败则任务阻断 + 回写评论，不让"坏活"静默通过。
> 落地 Harness 的"约束验证"层。
>
> **Scope:** 仅 **F2 v1 — 命令式验证门禁（阻断 + 评论）**。auto-fix loop、规则引擎
> （PATTERN/AST）、AI review check（→F3）、profile 过滤、in-session Stop hook 均 out of scope。
>
> **Engineering standard:** 硅谷级基建。one code path、不 hardcode、Forge 代码隔离在 `forge_`
> 前缀，最小化对 Multica 上游的侵入（R2）。

---

## 0. 背景

F2 是 Forge-on-Multica 路线图的第三切片（F0 基座、F1 规范中心已完成）。F1 把"规范"注入
agent；F2 把"验证"卡在任务完成前——规范是软约束（注入上下文），门禁是硬约束（不过不放行）。

**Multica 完成模型**（来自探查）：
- `TaskService.CompleteTask` 服务端**不改 issue 状态**——"Issue status is NOT changed here —
  the agent manages it via the CLI"（`server/internal/service/task.go:1013`）。agent 自己用
  `multica issue status` 流转 issue；平台的 complete 只标记 task。
- **没有任何 checks/verification 概念**——F2 从零建。
- 最干净的验证挂点：**daemon 在 agent 会话结束后、`reportTaskResult` 前**
  （`server/internal/daemon/daemon.go:2205-2216`），workdir 里有 repo，能跑 lint/test/build。
- Multica **不写** `.claude/settings.json`（只写 skills + issue_context），所以 Claude 原生
  Stop hook 需另加注入——v1 不走这条，走 daemon 侧。

## 1. 决策链（brainstorming 2026-06-01）

| # | 决策 | 选择 |
|---|------|------|
| 1 | 门禁挂点 | **daemon 侧 post-session**（workdir 跑验证，agent 会话后、报告完成前） |
| 2 | 失败处理 | **阻断 + 评论**（v1；auto-fix loop 后续） |
| 3 | 检查定义 | **项目配置的验证命令**（forge_check sidecar 表，workspace→project；非规则引擎） |

## 2. 核心概念 & 数据流

一个 **forge_check** = 名称 + shell 命令 + scope（workspace/project）。

```
agent 会话结束（result.Status=="completed", task 有 issue）
  → daemon: GET /api/daemon/tasks/{id}/forge-checks  → [{name, command}]
  → daemon: 逐条在 result.WorkDir 跑命令（超时 + 输出截断），收集失败（name + exit + 输出尾部）
  → 有失败：FailTask(failure_reason="verification_failed", errorMsg=失败清单)
              → 服务端 forge 钩子：createAgentComment 回写 "❌ Verification failed: …"
              → task → blocked（issue 不前进）
  → 全过：CompleteTask 正常完成
```

**与 F1 的区别**：F1 daemon 侧零改动（skill 走现有 execenv 机制）；F2 需 daemon **真的跑命令**，
是新 daemon 代码——F2 唯一比 F1 多的侵入面，隔离在 `server/internal/daemon/forge_verify.go`
+ `handleTask` 一处调用。

## 3. 数据模型（sidecar，类比 F1）

```sql
CREATE TABLE forge_check (
  id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  workspace_id UUID NOT NULL REFERENCES workspace(id) ON DELETE CASCADE,
  project_id   UUID REFERENCES project(id) ON DELETE CASCADE,  -- NULL=workspace级
  name         TEXT NOT NULL,
  command      TEXT NOT NULL,            -- 在 workdir 根跑；非零退出=失败
  enabled      BOOLEAN NOT NULL DEFAULT TRUE,
  created_by   UUID,
  created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX uq_forge_check_ws   ON forge_check (workspace_id, name) WHERE project_id IS NULL;
CREATE UNIQUE INDEX uq_forge_check_proj ON forge_check (project_id, name)   WHERE project_id IS NOT NULL;
CREATE INDEX idx_forge_check_ws ON forge_check (workspace_id);
```
迁移 112（实现时复查最大号）。不碰 Multica 表。

**解析（加法）**：resolved = workspace 级（project_id NULL, enabled）∪ project 级
（project_id=P, enabled）。与 F1 的"覆盖"不同——checks 是加法（都要跑）。

## 4. 解析 + daemon 侧验证（核心）

### 4.1 解析（`server/internal/forge/checks.go`）
```
ResolveChecks(ctx, q, workspaceID, projectID) -> []ForgeCheck
  = ListWorkspaceChecks(workspaceID) ∪ (projectID.Valid ? ListProjectChecks(projectID) : nil)
```
纯 + repository 包装，便于单测（类比 F1 Resolve）。

### 4.2 daemon 端点
`GET /api/daemon/tasks/{taskId}/forge-checks`（DaemonAuth）：由 task 的 issue→project 解析 checks，
返回 `[{name, command}]`。隔离在 `server/internal/handler/forge_daemon.go`。

### 4.3 daemon 执行（`server/internal/daemon/forge_verify.go`）
在 `handleTask` 的 `reportTaskResult` 前插一处调用：
```go
if result.Status == "completed" && task.IssueID != "" {
    if failure := forgeVerify(ctx, d.client, task, result.WorkDir); failure != "" {
        result.Status = "blocked"
        result.FailureReason = "verification_failed"
        result.Comment = failure
    }
}
```
`forgeVerify`：拉 checks → 逐条 `exec.CommandContext`（cwd=workDir，带超时如 5min，stdout/stderr
合并、尾部截断如 4KB）→ 任一非零即返回格式化失败清单（含通过/失败的 check 名）。

## 5. 失败处理（阻断 + 评论）

服务端在 task 失败路径（`FailTask` / `reportTaskResult` 的 fail 分支）加 forge 钩子：当
`failure_reason=="verification_failed"` 且 task 有 issue → `s.createAgentComment(issueID, agentID,
"❌ Verification failed:\n"+detail, ...)`。task → blocked，issue 不前进。重新触发时重跑。

## 6. API + UI

- **API**：`/api/forge/checks` CRUD（镜像 `/api/forge/standards`）+ daemon `forge-checks` 端点。
- **UI**：`packages/views/forge-checks/`——列表 + name/command 编辑 + scope（workspace/project）
  （镜像 F1 forge-standards UI）。

## 7. Upstream 隔离（R2）

`forge_check` 表 · `server/internal/forge/checks.go` · `server/internal/handler/forge_checks.go`
+ `forge_daemon.go` · `server/internal/daemon/forge_verify.go` · `/api/forge/*` +
`/api/daemon/tasks/{id}/forge-checks` · `packages/views/forge-checks/`。
**侵入点**：handleTask 一处调用 + FailTask 一处钩子 + 路由注册几行。

## 8. 测试

- **Go 单测**：`ResolveChecks`（workspace+project 加法合并）；`forgeVerify`（mock 一过一挂的命令，
  断言返回 blocked + 失败详情 + 命令尾部输出）。
- **端到端（绕开 provider 凭证）**：配一个 `bash -c "exit 1"` 的 workspace check → 建 issue →
  daemon 跑完 agent（即便 agent 因凭证失败也行——验证在其后）→ 断言 task blocked +
  failure_reason=verification_failed + issue 上有评论。**与 F1 一样可绕凭证验证**。

## 9. 边界（F2 v1 不做）

auto-fix loop（resume 自修复）· 规则引擎（PATTERN/AST/AI_CHECK）· AI review check（→F3）·
profile 过滤 checks（复用 F1 profile，后续）· 每-check 自定义 cwd · in-session Stop hook ·
并行跑 checks（v1 串行）。

## 10. 风险

| | 风险 | 缓解 |
|---|---|---|
| R1 | daemon 跑任意命令的安全 | 与 agent 同 workdir / 同信任域（用户机器上用户自己的项目命令）；超时 + 输出截断 |
| R2 | daemon 侧新代码致 upstream 合并冲突 | 隔离在 forge_verify.go + handleTask 一处调用 |
| R3 | 验证拖慢完成 | 命令带超时；无 checks 则跳过（no-op）；只 gate 成功完成的 issue-bound 任务 |
| R4 | 无真 agent 端到端（凭证遗留） | 验证在 agent 会话后由 daemon 跑，**可绕凭证**用 `exit 1` check 证明 |

> F2 完成后，下一切片 **F3 AI Review**（Reviewer agent / Squad 评审生成代码）。
