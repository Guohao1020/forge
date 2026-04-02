package activity

import (
	"context"
	"fmt"

	"github.com/shulex/forge/forge-core/internal/module/project"
)

// ProjectRepoAdapter wraps project.Repository to satisfy the ProjectProvider interface.
type ProjectRepoAdapter struct {
	repo *project.Repository
}

// NewProjectRepoAdapter creates a ProjectProvider backed by a project.Repository.
func NewProjectRepoAdapter(repo *project.Repository) *ProjectRepoAdapter {
	return &ProjectRepoAdapter{repo: repo}
}

func (a *ProjectRepoAdapter) GetByID(ctx context.Context, id, tenantID, userID int64) (*ProjectInfo, error) {
	p, err := a.repo.GetByID(ctx, id, tenantID, userID)
	if err != nil {
		return nil, fmt.Errorf("get project %d: %w", id, err)
	}
	return &ProjectInfo{
		CodeRepoURL:   p.CodeRepoURL,
		DefaultBranch: p.DefaultBranch,
	}, nil
}
