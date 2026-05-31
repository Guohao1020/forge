package handler

import (
	"context"

	"github.com/jackc/pgx/v5/pgtype"
)

// taskProjectID derives the project UUID for Forge standards resolution from
// the task's issue. Returns the zero UUID when there is no issue/project.
// Forge-owned helper; keeps the daemon claim hook to a single call site.
func (h *Handler) taskProjectID(ctx context.Context, issueID pgtype.UUID) pgtype.UUID {
	if !issueID.Valid {
		return pgtype.UUID{}
	}
	iss, err := h.Queries.GetIssue(ctx, issueID)
	if err != nil {
		return pgtype.UUID{}
	}
	return iss.ProjectID
}
