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

// metadataPRURL extracts a pr_url the agent pinned into the issue's metadata
// bag via `multica issue metadata set pr_url <url>`. Real agent runs report an
// opened PR by pinning it to metadata (the documented convention), not by
// populating the task-completion result, so this is the channel the bridge
// must read to fire end-to-end. Returns "" when absent or unparseable.
func metadataPRURL(metadata []byte) string {
	if len(metadata) == 0 {
		return ""
	}
	var m struct {
		PrURL string `json:"pr_url"`
	}
	if err := json.Unmarshal(metadata, &m); err != nil {
		return ""
	}
	return m.PrURL
}

// MaybeRecordFixPR records the PR a coding agent opened against its issue and
// posts a system comment. Best-effort: never blocks CompleteTask. Generic — any
// issue-bound task that surfaces a pr_url is recorded (fills the pr_url dead-end).
// Idempotent via the forge_fix_pr (task_id, pr_url) unique index: a duplicate
// CreateFixPR returns pgx.ErrNoRows, which we treat as success (no re-comment).
func (s *TaskService) MaybeRecordFixPR(ctx context.Context, task db.AgentTaskQueue, result []byte) {
	if !task.IssueID.Valid {
		return
	}
	issue, err := s.Queries.GetIssue(ctx, task.IssueID)
	if err != nil {
		slog.Warn("forge: record fix PR — get issue failed", "error", err)
		return
	}
	// Prefer the explicit completion field; fall back to the pr_url the agent
	// pinned into issue metadata. Real agent runs pin to metadata rather than
	// populating result.pr_url, so the metadata fallback is what makes the
	// bridge fire end-to-end (the result-only path was only ever exercised by
	// tests feeding pr_url straight into /complete).
	prURL := parseFixPRURL(result)
	if prURL == "" {
		prURL = metadataPRURL(issue.Metadata)
	}
	if prURL == "" {
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
