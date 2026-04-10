package workspace

import (
	"context"
	"database/sql"
	"fmt"
)

// ProjectInfo contains the data needed to clone a project's repository.
// Phase 1a shape: HTTPS URL + GitHub PAT (access token).
// Phase 1b will break this: drop AccessToken, rename RepoURL to SSHURL.
type ProjectInfo struct {
	RepoURL     string // e.g. "https://github.com/owner/repo.git"
	AccessToken string // GitHub PAT for HTTPS auth
	Branch      string // default branch to clone
}

// ProjectLookup resolves a (tenantID, projectID) to the repository info
// needed by the workspace module. Implementations may call forge-core's
// project table, an external API, or a test fixture.
type ProjectLookup interface {
	Lookup(ctx context.Context, tenantID, projectID int64) (*ProjectInfo, error)
}

// DBProjectLookup queries the project and user_identities tables to resolve
// repository URL and access token. The query joins project -> user_identities
// via created_by to get the GitHub OAuth token for the project creator.
type DBProjectLookup struct {
	db *sql.DB
}

// NewDBProjectLookup constructs a lookup backed by the forge database.
func NewDBProjectLookup(db *sql.DB) *DBProjectLookup {
	return &DBProjectLookup{db: db}
}

// Lookup resolves a project's clone info: repo URL, default branch, and the
// GitHub access token from the project creator's linked identity.
func (l *DBProjectLookup) Lookup(ctx context.Context, tenantID, projectID int64) (*ProjectInfo, error) {
	const q = `
		SELECT p.code_repo_url, COALESCE(p.default_branch, 'main'),
		       COALESCE(ui.access_token, '')
		FROM engine.projects p
		LEFT JOIN auth.user_identities ui
		       ON ui.user_id = p.created_by AND ui.provider = 'github'
		WHERE p.id = $1 AND p.tenant_id = $2
	`
	var info ProjectInfo
	err := l.db.QueryRowContext(ctx, q, projectID, tenantID).Scan(
		&info.RepoURL, &info.Branch, &info.AccessToken,
	)
	if err != nil {
		return nil, fmt.Errorf("workspace: project lookup (%d, %d): %w", tenantID, projectID, err)
	}
	return &info, nil
}

// StaticProjectLookup is a test fixture that returns a fixed ProjectInfo.
type StaticProjectLookup struct {
	Info *ProjectInfo
	Err  error
}

// Lookup returns the preconfigured ProjectInfo or error.
func (s *StaticProjectLookup) Lookup(_ context.Context, _, _ int64) (*ProjectInfo, error) {
	return s.Info, s.Err
}
