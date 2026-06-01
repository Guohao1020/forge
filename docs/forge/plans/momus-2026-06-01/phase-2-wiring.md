## Phase 2 — 接线（CompleteTask 钩子 + claim PriorWorkDir 钩子）

**Goal:** 把 `MaybeEnqueueReview` 接进 `CompleteTask`（一行）；claim handler 对 review 任务把
`PriorWorkDir` 设为该任务的 work_dir（让 reviewer 复用 coder 的 workdir）。

**Depends-on:** Phase 1　**Unblocks:** Phase 4
**Completion gate:** `go build ./...` 通过；侵入面 = CompleteTask 一行 + claim handler 一处。

> **细化 spec §4.2**：work_dir 复用非零 daemon 改动——Multica 的 resume 是 per-(agent,issue)，
> reviewer ≠ coder 找不到 coder 的 session，故需 claim 处一处 PriorWorkDir 钩子。

---

### Task 2.1: CompleteTask 触发钩子

**Files:**
- Modify: `server/internal/service/task.go`（CompleteTask，task-completed 日志后，约 1087-1097）

- [ ] **Step 1: 插入一行**

在 `CompleteTask` 里 `slog.Info("task completed", ...)`（约 1087 行）之后、issue 评论合成块
（`if task.IssueID.Valid {`，约 1097 行）之前插入：
```go
	// Forge F3: after a coding task completes, enqueue an AI review task for
	// the configured reviewer (best-effort; skips review tasks / no reviewer).
	s.MaybeEnqueueReview(ctx, task)
```
> `task` 是 `db.AgentTaskQueue`（CompleteTask 的局部变量）。`MaybeEnqueueReview` 在
> `service/forge_review.go`（同包）。

- [ ] **Step 2: 编译 + commit**

Run: `wsl -d Ubuntu -- bash -lc "cd /mnt/d/shulex_work/forge/server && go build ./... 2>&1 | tail -8"`
Expected: 通过。
```bash
git add server/internal/service/task.go
git commit -m "feat(forge): trigger AI review on coding task completion"
```

---

### Task 2.2: claim handler — review 任务复用 coder workdir

**Files:**
- Modify: `server/internal/handler/daemon.go`（ClaimTaskByRuntime，PriorWorkDir 解析逻辑后，约 1335-1375）

- [ ] **Step 1: 加 review-task 的 PriorWorkDir 覆盖**

在 claim handler 设置 `resp.PriorWorkDir` 的现有逻辑（issue-bound resume 块，约 1318-1335；
GetLastTaskSession → resp.PriorWorkDir）**之后**插入：
```go
	// Forge F3: a review task reuses the coder's workdir (stored on the review
	// task row) so the reviewer can `git diff` the changes. The per-(agent,issue)
	// resume lookup above won't find it (reviewer != coder), so override here.
	if forge.IsReviewTask(task.Context) && task.WorkDir.Valid {
		resp.PriorWorkDir = task.WorkDir.String
	}
```
> `forge` 已在 daemon.go import（F1 加的）。`task.Context`/`task.WorkDir` 是 db row 字段
> （`task` 在 claim handler 里是 db.AgentTaskQueue；F1/F2 已用 `task.IssueID`/`task.WorkspaceID`）。
> 实现时确认插入点在 resp.PriorWorkDir 赋值之后、resp 最终返回之前；行号以实际为准。

- [ ] **Step 2: 编译 + commit**

Run: `wsl -d Ubuntu -- bash -lc "cd /mnt/d/shulex_work/forge/server && go build ./... 2>&1 | tail -8"`
Expected: 通过（若 `task` 变量在该处不是 db row 而是别的类型，按实际取 Context/WorkDir）。
```bash
git add server/internal/handler/daemon.go
git commit -m "feat(forge): review tasks reuse coder workdir via PriorWorkDir at claim"
```

---

## Phase 2 完成检查
- [ ] CompleteTask 一行 `MaybeEnqueueReview` 钩子
- [ ] claim handler review-task PriorWorkDir 覆盖
- [ ] `go build ./...` 通过；侵入面 = service 一行 + handler 一处
