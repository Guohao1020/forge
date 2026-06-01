package forge

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

// Check is a resolved verification check the daemon runs in the task workdir.
type Check struct {
	Name    string `json:"name"`
	Command string `json:"command"`
}

// resolveChecks merges workspace + project checks additively (both run).
func resolveChecks(ws, proj []db.ForgeCheck) []Check {
	out := make([]Check, 0, len(ws)+len(proj))
	for _, c := range ws {
		out = append(out, Check{Name: c.Name, Command: c.Command})
	}
	for _, c := range proj {
		out = append(out, Check{Name: c.Name, Command: c.Command})
	}
	return out
}

// CheckQuerier is the subset of generated db methods ResolveChecks needs.
// *db.Queries satisfies it.
type CheckQuerier interface {
	ListWorkspaceChecks(ctx context.Context, workspaceID pgtype.UUID) ([]db.ForgeCheck, error)
	ListProjectChecks(ctx context.Context, projectID pgtype.UUID) ([]db.ForgeCheck, error)
}

// ResolveChecks loads workspace + project checks for a task and merges them.
// projectID may be the zero UUID (no project) — then only workspace checks apply.
func ResolveChecks(ctx context.Context, q CheckQuerier, workspaceID, projectID pgtype.UUID) ([]Check, error) {
	ws, err := q.ListWorkspaceChecks(ctx, workspaceID)
	if err != nil {
		return nil, fmt.Errorf("list workspace checks: %w", err)
	}
	var proj []db.ForgeCheck
	if projectID.Valid {
		proj, err = q.ListProjectChecks(ctx, projectID)
		if err != nil {
			return nil, fmt.Errorf("list project checks: %w", err)
		}
	}
	return resolveChecks(ws, proj), nil
}
