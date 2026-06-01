package service

import (
	"context"
	"encoding/json"
	"log/slog"

	"github.com/multica-ai/multica/server/internal/forgereview"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

// MaybeEnqueueReview enqueues an AI review task after a coding task completes,
// when a reviewer is configured for the scope. Best-effort: returns silently on
// any unmet condition. Reuses the coder's workdir so the reviewer can git diff.
func (s *TaskService) MaybeEnqueueReview(ctx context.Context, task db.AgentTaskQueue) {
	if !forgereview.ShouldEnqueueReview(task.IssueID.Valid, task.WorkDir.String, task.Context) {
		return
	}
	issue, err := s.Queries.GetIssue(ctx, task.IssueID)
	if err != nil {
		return
	}
	reviewerID, ok := forgereview.ResolveReviewer(ctx, s.Queries, issue.WorkspaceID, issue.ProjectID)
	if !ok {
		return
	}
	reviewer, err := s.Queries.GetAgent(ctx, reviewerID)
	if err != nil || reviewer.ArchivedAt.Valid || !reviewer.RuntimeID.Valid {
		return
	}
	ctxJSON, err := json.Marshal(forgereview.ForgeReviewContext{
		Type:         forgereview.ReviewContextType,
		ForgeReview:  true,
		ReviewPrompt: forgereview.DefaultReviewPrompt,
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
