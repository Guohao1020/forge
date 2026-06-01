// Package forgereview holds Forge F3 AI-review pure logic. It is deliberately
// separate from package forge (which imports service for AgentSkillData) so that
// the service package can import this without an import cycle.
package forgereview

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
