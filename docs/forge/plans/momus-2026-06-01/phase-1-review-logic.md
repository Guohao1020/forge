## Phase 1 — 解析 + 触发逻辑（forge/review + service）

**Goal:** `forge/review.go`（ResolveReviewer + IsReviewTask + ShouldEnqueueReview，纯函数 TDD）
+ `service/forge_review.go`（MaybeEnqueueReview 编排，建 review 任务 + 通知 daemon）。

**Depends-on:** Phase 0　**Unblocks:** Phase 2
**Completion gate:** forge 单测绿；`go build ./...` 通过。

---

### Task 1.1: forge/review.go（先写测试）

**Files:**
- Create: `server/internal/forge/review.go`
- Test: `server/internal/forge/review_test.go`

- [ ] **Step 1: 写失败测试**

`server/internal/forge/review_test.go`：
```go
package forge

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

func TestIsReviewTask(t *testing.T) {
	marked, _ := json.Marshal(ForgeReviewContext{Type: ReviewContextType, ForgeReview: true})
	if !IsReviewTask(marked) {
		t.Fatal("marked context should be a review task")
	}
	if IsReviewTask(nil) || IsReviewTask([]byte(`{"type":"quick_create"}`)) {
		t.Fatal("non-review context must not be a review task")
	}
}

func TestShouldEnqueueReview(t *testing.T) {
	if !ShouldEnqueueReview(true, "/w/d", nil) {
		t.Fatal("issue-bound + workdir + non-review should enqueue")
	}
	if ShouldEnqueueReview(false, "/w/d", nil) {
		t.Fatal("no issue → skip")
	}
	if ShouldEnqueueReview(true, "", nil) {
		t.Fatal("no workdir → skip")
	}
	marked, _ := json.Marshal(ForgeReviewContext{ForgeReview: true})
	if ShouldEnqueueReview(true, "/w/d", marked) {
		t.Fatal("review task itself → skip (loop prevention)")
	}
}

type fakeReviewQ struct {
	ws, proj *db.ForgeReviewConfig
}

func (f fakeReviewQ) GetWorkspaceReviewConfig(_ context.Context, _ pgtype.UUID) (db.ForgeReviewConfig, error) {
	if f.ws == nil {
		return db.ForgeReviewConfig{}, errNoRow
	}
	return *f.ws, nil
}
func (f fakeReviewQ) GetProjectReviewConfig(_ context.Context, _ pgtype.UUID) (db.ForgeReviewConfig, error) {
	if f.proj == nil {
		return db.ForgeReviewConfig{}, errNoRow
	}
	return *f.proj, nil
}

var errNoRow = &noRowErr{}

type noRowErr struct{}

func (*noRowErr) Error() string { return "no row" }

var _ ReviewConfigQuerier = fakeReviewQ{}

func agentUUID(b byte) pgtype.UUID { return pgtype.UUID{Bytes: [16]byte{b}, Valid: true} }

func TestResolveReviewer_ProjectOverrides(t *testing.T) {
	wsCfg := &db.ForgeReviewConfig{ReviewerAgentID: agentUUID(1)}
	projCfg := &db.ForgeReviewConfig{ReviewerAgentID: agentUUID(2)}
	got, ok := ResolveReviewer(context.Background(), fakeReviewQ{ws: wsCfg, proj: projCfg}, pgtype.UUID{Valid: true}, pgtype.UUID{Valid: true})
	if !ok || got.Bytes[0] != 2 {
		t.Fatalf("project reviewer should win; got ok=%v id0=%d", ok, got.Bytes[0])
	}
	// no project config → fall back to workspace
	got, ok = ResolveReviewer(context.Background(), fakeReviewQ{ws: wsCfg}, pgtype.UUID{Valid: true}, pgtype.UUID{Valid: true})
	if !ok || got.Bytes[0] != 1 {
		t.Fatalf("workspace reviewer fallback; got ok=%v id0=%d", ok, got.Bytes[0])
	}
	// none → ok=false
	if _, ok := ResolveReviewer(context.Background(), fakeReviewQ{}, pgtype.UUID{Valid: true}, pgtype.UUID{}); ok {
		t.Fatal("no config → ok=false")
	}
}
```

- [ ] **Step 2: 跑测试确认失败**

Run: `wsl -d Ubuntu -- bash -lc "cd /mnt/d/shulex_work/forge/server && go test ./internal/forge/ -run 'Review|ShouldEnqueue' 2>&1 | tail -8"`
Expected: 编译失败（未定义）。

- [ ] **Step 3: 写实现**

`server/internal/forge/review.go`：
```go
package forge

import (
	"context"
	"encoding/json"

	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

// ReviewContextType marks a task's context as a Forge AI-review task.
const ReviewContextType = "forge_review"

// DefaultReviewPrompt guides the reviewer agent.
const DefaultReviewPrompt = "Review the code changes in this working directory: run `git diff` (or `git diff main...HEAD`) to see them. Apply the project coding standards. Post your findings as comments on the issue, then stop."

// ForgeReviewContext is stored in a review task's context (JSONB). The marker
// prevents review loops; the prompt guides the reviewer.
type ForgeReviewContext struct {
	Type         string `json:"type"`
	ForgeReview  bool   `json:"forge_review"`
	ReviewPrompt string `json:"review_prompt"`
	ParentTaskID string `json:"parent_task_id"`
}

// IsReviewTask reports whether a task's context marks it as a forge review task.
func IsReviewTask(contextJSON []byte) bool {
	if len(contextJSON) == 0 {
		return false
	}
	var c ForgeReviewContext
	if err := json.Unmarshal(contextJSON, &c); err != nil {
		return false
	}
	return c.ForgeReview
}

// ShouldEnqueueReview reports whether a just-completed task is eligible for an
// AI review: issue-bound, has a workdir (a diff to review), and is not itself a
// review task (loop prevention).
func ShouldEnqueueReview(issueValid bool, workDir string, contextJSON []byte) bool {
	return issueValid && workDir != "" && !IsReviewTask(contextJSON)
}

// ReviewConfigQuerier is the subset of db methods ResolveReviewer needs.
// *db.Queries satisfies it.
type ReviewConfigQuerier interface {
	GetWorkspaceReviewConfig(ctx context.Context, workspaceID pgtype.UUID) (db.ForgeReviewConfig, error)
	GetProjectReviewConfig(ctx context.Context, projectID pgtype.UUID) (db.ForgeReviewConfig, error)
}

// ResolveReviewer returns the reviewer agent for (workspace, project); a
// project-level config overrides the workspace-level one. ok=false if none.
func ResolveReviewer(ctx context.Context, q ReviewConfigQuerier, workspaceID, projectID pgtype.UUID) (pgtype.UUID, bool) {
	if projectID.Valid {
		if cfg, err := q.GetProjectReviewConfig(ctx, projectID); err == nil {
			return cfg.ReviewerAgentID, true
		}
	}
	if cfg, err := q.GetWorkspaceReviewConfig(ctx, workspaceID); err == nil {
		return cfg.ReviewerAgentID, true
	}
	return pgtype.UUID{}, false
}
```

- [ ] **Step 4: 跑测试确认通过**

Run: `wsl -d Ubuntu -- bash -lc "cd /mnt/d/shulex_work/forge/server && go test ./internal/forge/ -run 'Review|ShouldEnqueue' -v 2>&1 | tail -15"`
Expected: 3 测试 PASS。

- [ ] **Step 5: Commit**

```bash
git add server/internal/forge/review.go server/internal/forge/review_test.go
git commit -m "feat(forge): review resolve + loop-guard logic"
```

---

### Task 1.2: service/forge_review.go（MaybeEnqueueReview 编排）

**Files:**
- Create: `server/internal/service/forge_review.go`

- [ ] **Step 1: 写编排**

`server/internal/service/forge_review.go`：
```go
package service

import (
	"context"
	"encoding/json"
	"log/slog"

	"github.com/multica-ai/multica/server/internal/forge"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/protocol"
	"github.com/multica-ai/multica/server/pkg/util"
)

// MaybeEnqueueReview enqueues an AI review task after a coding task completes,
// when a reviewer is configured for the scope. Best-effort: returns silently on
// any unmet condition. Reuses the coder's workdir so the reviewer can git diff.
func (s *TaskService) MaybeEnqueueReview(ctx context.Context, task db.AgentTaskQueue) {
	if !forge.ShouldEnqueueReview(task.IssueID.Valid, task.WorkDir.String, task.Context) {
		return
	}
	issue, err := s.Queries.GetIssue(ctx, task.IssueID)
	if err != nil {
		return
	}
	reviewerID, ok := forge.ResolveReviewer(ctx, s.Queries, issue.WorkspaceID, issue.ProjectID)
	if !ok {
		return
	}
	reviewer, err := s.Queries.GetAgent(ctx, reviewerID)
	if err != nil || reviewer.ArchivedAt.Valid || !reviewer.RuntimeID.Valid {
		return
	}
	ctxJSON, err := json.Marshal(forge.ForgeReviewContext{
		Type:         forge.ReviewContextType,
		ForgeReview:  true,
		ReviewPrompt: forge.DefaultReviewPrompt,
		ParentTaskID: util.UUIDToString(task.ID),
	})
	if err != nil {
		return
	}
	reviewTask, err := s.Queries.CreateForgeReviewTask(ctx, db.CreateForgeReviewTaskParams{
		AgentID:      reviewerID,
		RuntimeID:    reviewer.RuntimeID,
		IssueID:      task.IssueID,
		ParentTaskID: task.ID,
		WorkDir:      task.WorkDir,
		Context:      ctxJSON,
		Priority:     priorityToInt("high"),
	})
	if err != nil {
		slog.Warn("forge: enqueue review task failed", "error", err)
		return
	}
	slog.Info("forge: review task enqueued",
		"review_task_id", util.UUIDToString(reviewTask.ID),
		"parent_task_id", util.UUIDToString(task.ID),
		"reviewer", util.UUIDToString(reviewerID))
	s.broadcastTaskEvent(ctx, protocol.EventTaskQueued, reviewTask)
	s.NotifyTaskEnqueued(ctx, reviewTask)
}
```
> 实现时确认：`priorityToInt`、`s.broadcastTaskEvent`、`s.NotifyTaskEnqueued`、`util.UUIDToString`、
> `protocol.EventTaskQueued` 在 service 包可用（探查确认）。`issue.ProjectID`/`issue.WorkspaceID`
> 是 pgtype.UUID。`CreateForgeReviewTaskParams` 字段名以 sqlc 生成为准。

- [ ] **Step 2: 编译验证**

Run: `wsl -d Ubuntu -- bash -lc "cd /mnt/d/shulex_work/forge/server && go build ./... 2>&1 | tail -8"`
Expected: 通过。

- [ ] **Step 3: Commit**

```bash
git add server/internal/service/forge_review.go
git commit -m "feat(forge): MaybeEnqueueReview — enqueue review task for reviewer"
```

---

## Phase 1 完成检查
- [ ] forge review 3 单测绿（IsReviewTask / ShouldEnqueueReview / ResolveReviewer）
- [ ] MaybeEnqueueReview 编译通过，`*db.Queries` 满足 `ReviewConfigQuerier`
