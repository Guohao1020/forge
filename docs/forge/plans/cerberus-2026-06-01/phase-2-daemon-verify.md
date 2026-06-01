## Phase 2 — daemon 验证（端点 + forge_verify + hook）

**Goal:** daemon 在 agent 会话后拉 checks、在 workdir 跑、失败转 FailTask 路径
（`verification_failed` + 失败详情作 Comment，现有 FailTask 自动回写评论）。

**Depends-on:** Phase 1（`forge.ResolveChecks`）　**Unblocks:** Phase 4
**Completion gate:** `runChecks` 单测（一过一挂）绿；`go build ./...` 通过；侵入面 = handleTask 一处调用 + 一个 daemon 文件 + 一个 handler + 一条路由。

> **简化**：失败回写评论**不需要新钩子**——daemon 把失败详情作 `result.Comment` 走 FailTask，
> 服务端 `FailTask`（task.go:1273）已有 `createAgentComment(errMsg)` 路径会发评论
> （`verification_failed` 不在自动重试列表，故 `retried==nil` → 发）。

---

### Task 2.1: 服务端 daemon 端点（解析 checks）

**Files:**
- Create: `server/internal/handler/forge_daemon.go`
- Modify: `server/cmd/server/router.go`（daemon 路由组，`/api/daemon/tasks/{taskId}/*` 附近）

- [ ] **Step 1: 写 handler（复用 F1 的 taskProjectID helper）**

`server/internal/handler/forge_daemon.go`：
```go
package handler

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/multica-ai/multica/server/internal/forge"
)

type ForgeChecksResponse struct {
	Checks []forge.Check `json:"checks"`
}

// GetTaskForgeChecks resolves verification checks for a claimed task (daemon-auth).
func (h *Handler) GetTaskForgeChecks(w http.ResponseWriter, r *http.Request) {
	taskID := chi.URLParam(r, "taskId")
	_, workspaceID, ok := h.requireDaemonTaskAccessWithWorkspace(w, r, taskID)
	if !ok {
		return
	}
	task, err := h.Queries.GetAgentTask(r.Context(), parseUUID(taskID))
	if err != nil {
		writeError(w, http.StatusNotFound, "task not found")
		return
	}
	projID := h.taskProjectID(r.Context(), task.IssueID) // F1 helper in forge_hook.go
	checks, err := forge.ResolveChecks(r.Context(), h.Queries, parseUUID(workspaceID), projID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to resolve checks")
		return
	}
	writeJSON(w, http.StatusOK, ForgeChecksResponse{Checks: checks})
}
```
> 实现时确认：`requireDaemonTaskAccessWithWorkspace` 返回 `(任务?, workspaceID string, ok bool)`；
> `GetAgentTask` 返回的 task 有 `IssueID pgtype.UUID`。均见探查（daemon.go:1779 / types）。

- [ ] **Step 2: 注册路由**

在 `router.go` 的 daemon 路由组（与 `/api/daemon/tasks/{taskId}/complete` 同组、经 DaemonAuth）加：
```go
r.Get("/api/daemon/tasks/{taskId}/forge-checks", h.GetTaskForgeChecks)
```

- [ ] **Step 3: 编译 + commit**

Run: `wsl -d Ubuntu -- bash -lc "cd /mnt/d/shulex_work/forge/server && go build ./... 2>&1 | tail -8"`
Expected: 通过。
```bash
git add server/internal/handler/forge_daemon.go server/cmd/server/router.go
git commit -m "feat(forge): daemon endpoint GET /api/daemon/tasks/{id}/forge-checks"
```

---

### Task 2.2: daemon 侧 forge_verify.go（先写测试）

**Files:**
- Create: `server/internal/daemon/forge_verify.go`
- Test: `server/internal/daemon/forge_verify_test.go`

- [ ] **Step 1: 写失败测试（纯 runChecks，不依赖 client）**

`server/internal/daemon/forge_verify_test.go`：
```go
package daemon

import (
	"context"
	"log/slog"
	"strings"
	"testing"
)

func TestRunChecks_FailureSummary(t *testing.T) {
	dir := t.TempDir()
	checks := []ForgeCheck{
		{Name: "ok", Command: "exit 0"},
		{Name: "bad", Command: "echo boom; exit 1"},
	}
	got := runChecks(context.Background(), dir, checks, slog.Default())
	if !strings.Contains(got, "bad") || !strings.Contains(got, "boom") {
		t.Fatalf("failure summary must name the failed check + its output; got %q", got)
	}
}

func TestRunChecks_AllPass(t *testing.T) {
	dir := t.TempDir()
	got := runChecks(context.Background(), dir, []ForgeCheck{{Name: "ok", Command: "exit 0"}}, slog.Default())
	if got != "" {
		t.Fatalf("all-pass must yield empty summary; got %q", got)
	}
}
```

- [ ] **Step 2: 跑测试确认失败**

Run: `wsl -d Ubuntu -- bash -lc "cd /mnt/d/shulex_work/forge/server && go test ./internal/daemon/ -run RunChecks 2>&1 | tail -8"`
Expected: 编译失败（`runChecks` / `ForgeCheck` undefined）。

- [ ] **Step 3: 写实现**

`server/internal/daemon/forge_verify.go`：
```go
package daemon

import (
	"context"
	"fmt"
	"log/slog"
	"os/exec"
	"strings"
	"time"
)

// ForgeCheck mirrors the server's forge.Check (name + shell command).
type ForgeCheck struct {
	Name    string `json:"name"`
	Command string `json:"command"`
}

type forgeChecksResult struct {
	Checks []ForgeCheck `json:"checks"`
}

// GetForgeChecks fetches resolved verification checks for a task.
func (c *Client) GetForgeChecks(ctx context.Context, taskID string) (*forgeChecksResult, error) {
	var resp forgeChecksResult
	if err := c.getJSON(ctx, fmt.Sprintf("/api/daemon/tasks/%s/forge-checks", taskID), &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

const (
	forgeCheckTimeout   = 5 * time.Minute
	forgeCheckOutputCap = 4096
)

// runChecks runs each check command in workDir (via `bash -lc`). Returns a
// formatted failure summary; empty string = all passed / nothing to run.
// Pure (no client / Daemon) so it is unit-testable.
func runChecks(ctx context.Context, workDir string, checks []ForgeCheck, log *slog.Logger) string {
	var failures []string
	for _, ch := range checks {
		cctx, cancel := context.WithTimeout(ctx, forgeCheckTimeout)
		cmd := exec.CommandContext(cctx, "bash", "-lc", ch.Command)
		cmd.Dir = workDir
		out, err := cmd.CombinedOutput()
		cancel()
		if err != nil {
			tail := strings.TrimSpace(string(out))
			if len(tail) > forgeCheckOutputCap {
				tail = tail[len(tail)-forgeCheckOutputCap:]
			}
			failures = append(failures,
				fmt.Sprintf("- **%s** (`%s`) failed: %v\n```\n%s\n```", ch.Name, ch.Command, err, tail))
			log.Warn("forge check failed", "check", ch.Name, "error", err)
		} else {
			log.Info("forge check passed", "check", ch.Name)
		}
	}
	if len(failures) == 0 {
		return ""
	}
	return fmt.Sprintf("❌ Verification failed (%d check(s)):\n\n%s", len(failures), strings.Join(failures, "\n\n"))
}

// runForgeChecks fetches the task's checks and runs them in workDir.
// Best-effort: fetch error → "" (no gate), logged.
func (d *Daemon) runForgeChecks(ctx context.Context, taskID, workDir string, log *slog.Logger) string {
	if workDir == "" {
		return ""
	}
	res, err := d.client.GetForgeChecks(ctx, taskID)
	if err != nil {
		log.Warn("forge: fetch checks failed; skipping gate", "error", err)
		return ""
	}
	if res == nil || len(res.Checks) == 0 {
		return ""
	}
	return runChecks(ctx, workDir, res.Checks, log)
}
```
> 注：daemon 跑在 Windows 时需 `bash` 在 PATH（Git-for-Windows 提供）；WSL2/Linux daemon 原生有。
> v1 用 `bash -lc` 取最大命令兼容性。

- [ ] **Step 4: 跑测试确认通过**

Run: `wsl -d Ubuntu -- bash -lc "cd /mnt/d/shulex_work/forge/server && go test ./internal/daemon/ -run RunChecks -v 2>&1 | tail -10"`
Expected: 2 个测试 PASS。

- [ ] **Step 5: Commit**

```bash
git add server/internal/daemon/forge_verify.go server/internal/daemon/forge_verify_test.go
git commit -m "feat(forge): daemon-side verification check runner"
```

---

### Task 2.3: 接入 handleTask 钩子

**Files:**
- Modify: `server/internal/daemon/daemon.go`（`handleTask`，`reportTaskResult` 调用前，约 2216 行）

- [ ] **Step 1: 在 reportTaskResult 前插验证**

把 `handleTask` 末尾的 `d.reportTaskResult(ctx, task.ID, result, taskLog)`（约 2216 行）改为：
```go
	// Forge F2: verification gate. After the agent session ends, run the
	// project's configured checks in the workdir; on failure divert to the
	// fail path (verification_failed) so the work does not pass silently.
	if result.Status == "completed" && task.IssueID != "" {
		if failure := d.runForgeChecks(ctx, task.ID, result.WorkDir, taskLog); failure != "" {
			result.Status = "blocked"
			result.FailureReason = "verification_failed"
			result.Comment = failure
		}
	}

	d.reportTaskResult(ctx, task.ID, result, taskLog)
```
> `result.Status="blocked"` 走 `reportTaskResult` 的 default 分支 → `FailTask(..., result.Comment,
> ..., "verification_failed")`；服务端 FailTask 把 Comment 发成 issue 评论。`task.IssueID` 是
> string（types.go Task struct），非空判断即可。

- [ ] **Step 2: 编译验证**

Run: `wsl -d Ubuntu -- bash -lc "cd /mnt/d/shulex_work/forge/server && go build ./... 2>&1 | tail -8"`
Expected: 通过。

- [ ] **Step 3: Commit**

```bash
git add server/internal/daemon/daemon.go
git commit -m "feat(forge): gate task completion on verification checks at daemon"
```

---

## Phase 2 完成检查
- [ ] `runChecks` 一过一挂 + 全过 2 单测绿
- [ ] daemon 端点 + client `GetForgeChecks` + handleTask 钩子，`go build ./...` 通过
- [ ] 侵入面 = handleTask 一处 + forge_verify.go + forge_daemon.go + 一条路由
