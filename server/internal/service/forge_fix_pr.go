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
// Idempotent via the forge_fix_pr (task_id, pr_url) unique index: a duplicate
// CreateFixPR returns pgx.ErrNoRows, which we treat as success (no re-comment).
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
