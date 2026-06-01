## Phase 2 — PR 桥（forge_fix_pr.go + CompleteTask 钩子，TDD）

**Goal:** `service/forge_fix_pr.go`:纯 `parseFixPRURL`(TDD)+ `MaybeRecordFixPR`(解析 pr_url → 插 forge_fix_pr + 系统评论,best-effort 幂等);CompleteTask 一处一行钩子。
**Depends-on:** Phase 0（`CreateFixPR` + `forge_fix_pr` 表）　**Unblocks:** Phase 4
**Completion gate:** `parseFixPRURL` 单测绿;`go build ./...` + `go vet ./internal/service/` 通过。

> 接线点:`server/internal/service/task.go` CompleteTask 内 `s.MaybeEnqueueReview(ctx, task)`（line ~1091）之后。
> `result []byte` 是 CompleteTask 的入参（= `json.Marshal(TaskCompleteRequest{pr_url, output, session_id, work_dir})`）。
> `AgentTaskQueue` 无 WorkspaceID → 经 `GetIssue(task.IssueID).WorkspaceID`。完成请求无 branch → `Branch: ""`。

---

### Task 2.1: parseFixPRURL（纯函数，TDD）+ MaybeRecordFixPR

**Files:**
- Create: `server/internal/service/forge_fix_pr.go`
- Create: `server/internal/service/forge_fix_pr_test.go`

- [ ] **Step 1: 写失败测试**

`server/internal/service/forge_fix_pr_test.go`：
```go
package service

import "testing"

func TestParseFixPRURL(t *testing.T) {
	if got := parseFixPRURL([]byte(`{"pr_url":"https://github.com/o/r/pull/1","output":"done"}`)); got != "https://github.com/o/r/pull/1" {
		t.Fatalf("got %q", got)
	}
	if got := parseFixPRURL([]byte(`{"output":"done"}`)); got != "" {
		t.Fatalf("expected empty when absent, got %q", got)
	}
	if got := parseFixPRURL([]byte(`not json`)); got != "" {
		t.Fatalf("expected empty on bad json, got %q", got)
	}
}
```

- [ ] **Step 2: 运行确认失败**

Run: `wsl -d Ubuntu -- bash -lc "cd /mnt/d/shulex_work/forge/server && go test ./internal/service/ -run TestParseFixPRURL 2>&1 | tail -8"`
Expected: 编译失败（`parseFixPRURL` 未定义）。

- [ ] **Step 3: 实现**

`server/internal/service/forge_fix_pr.go`：
```go
package service

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

// parseFixPRURL extracts pr_url from a task completion result blob
// (json.Marshal of TaskCompleteRequest{pr_url, output, session_id, work_dir}).
// Returns "" when absent or unparseable.
func parseFixPRURL(result []byte) string {
	var r struct {
		PrURL string `json:"pr_url"`
	}
	if err := json.Unmarshal(result, &r); err != nil {
		return ""
	}
	return r.PrURL
}

// MaybeRecordFixPR records the PR a coding agent opened against its issue and
// posts a system comment. Best-effort: never blocks CompleteTask. Generic — any
// issue-bound task returning a pr_url is recorded (fills the pr_url dead-end).
// Idempotent via the forge_fix_pr (task_id, pr_url) unique index.
func (s *TaskService) MaybeRecordFixPR(ctx context.Context, task db.AgentTaskQueue, result []byte) {
	if !task.IssueID.Valid {
		return
	}
	prURL := parseFixPRURL(result)
	if prURL == "" {
		return
	}
	issue, err := s.Queries.GetIssue(ctx, task.IssueID)
	if err != nil {
		slog.Warn("forge: record fix PR — get issue failed", "error", err)
		return
	}
	if _, err := s.Queries.CreateFixPR(ctx, db.CreateFixPRParams{
		WorkspaceID: issue.WorkspaceID,
		IssueID:     task.IssueID,
		TaskID:      task.ID,
		PrUrl:       prURL,
		Branch:      "",
	}); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return // already recorded (idempotent) — no duplicate comment
		}
		slog.Warn("forge: record fix PR failed", "error", err)
		return
	}
	s.createAgentComment(ctx, task.IssueID, task.AgentID, "🔧 Fix PR opened: "+prURL, "system", pgtype.UUID{})
}
```

- [ ] **Step 4: 运行确认通过**

Run: `wsl -d Ubuntu -- bash -lc "cd /mnt/d/shulex_work/forge/server && go test ./internal/service/ -run TestParseFixPRURL 2>&1 | tail -6"`
Expected: PASS。

> `CreateFixPR` 因 `ON CONFLICT DO NOTHING` 在重复时返回 `pgx.ErrNoRows` → `errors.Is(err, pgx.ErrNoRows)` 静默跳过、不重复评论。
> `pgx`/`pgtype`/`errors`/`json`/`slog` 在 service 包通常已用;若某 import 报"already imported / unused",按编译提示调整。

- [ ] **Step 5: Commit**

```bash
git add server/internal/service/forge_fix_pr.go server/internal/service/forge_fix_pr_test.go
git commit -m "feat(forge): MaybeRecordFixPR — record agent PR + system comment"
```

---

### Task 2.2: CompleteTask 钩子

**Files:**
- Modify: `server/internal/service/task.go`（CompleteTask 内一行）

- [ ] **Step 1: 插钩子**

在 `server/internal/service/task.go` 的 `CompleteTask` 内,找到:
```go
	s.MaybeEnqueueReview(ctx, task)
```
在其**之后**插入一行:
```go
	// Forge F4b: record any PR the agent opened (e.g. an entropy-scan fix) and
	// comment it on the issue. Best-effort; never blocks completion.
	s.MaybeRecordFixPR(ctx, task, result)
```
（`result` 是 `CompleteTask(ctx, taskID, result []byte, sessionID, workDir string)` 的入参,在此作用域内。）

- [ ] **Step 2: 编译 + vet**

Run: `wsl -d Ubuntu -- bash -lc "cd /mnt/d/shulex_work/forge/server && go build ./... 2>&1 | tail -8 && go vet ./internal/service/ 2>&1 | tail -5 && echo OK"`
Expected: 打印 `OK`,无 build/vet 错误。

- [ ] **Step 3: Commit**

```bash
git add server/internal/service/task.go
git commit -m "feat(forge): record fix PR on task completion"
```

---

## Phase 2 完成检查
- [ ] `parseFixPRURL` 单测绿（有/无/坏 json）
- [ ] `MaybeRecordFixPR` 编译通过（GetIssue 取 workspace、CreateFixPR 幂等、系统评论）
- [ ] CompleteTask 一处钩子，`go build` + vet 通过
