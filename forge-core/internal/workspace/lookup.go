package workspace

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

// ProjectInfo carries the project metadata that the workspace state
// machine needs. Phase 1b dropped AccessToken and renamed
// RepoURL to SSHURL -- the workspace layer only ever uses SSH URLs
// for git operations now, and the one-time PAT used to upload the
// deploy key is fetched via GetOwnerGitHubToken on that single path.
type ProjectInfo struct {
	ProjectID     int64
	TenantID      int64
	SSHURL        string // git@github.com:owner/repo.git (converted from stored HTTPS)
	DefaultBranch string
	CreatedBy     int64
}

// ProjectLookup abstracts the project-row access that EnsureReady
// needs. Defined in the workspace package to avoid a cyclic dependency
// with internal/module/project (which imports workspace.WorkspaceProvider).
//
// The production implementation is DBProjectLookup below. main.go wires it.
type ProjectLookup interface {
	// Lookup returns project metadata. Returns ErrProjectNotFound
	// if no row exists.
	Lookup(ctx context.Context, tenantID, projectID int64) (*ProjectInfo, error)

	// GetOwnerGitHubToken returns a usable GitHub PAT for the user who
	// owns the project. Called ONCE per project, only when the deploy
	// key is being generated and uploaded. After the first EnsureReady
	// succeeds, this method is never called again for that project.
	GetOwnerGitHubToken(ctx context.Context, projectID int64) (string, error)
}

// ErrProjectNotFound is returned by ProjectLookup implementations when
// the (tenantID, projectID) pair does not match any known project.
var ErrProjectNotFound = errors.New("workspace: project not found")

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

// Lookup resolves a project's clone info. The stored HTTPS URL is
// converted to SSH form via HTTPSToSSHURL -- the workspace layer only
// uses SSH URLs after Phase 1b.
func (l *DBProjectLookup) Lookup(ctx context.Context, tenantID, projectID int64) (*ProjectInfo, error) {
	const q = `
		SELECT p.id, p.tenant_id, p.code_repo_url, COALESCE(p.default_branch, 'main'),
		       p.created_by
		FROM engine.projects p
		WHERE p.id = $1 AND p.tenant_id = $2
	`
	var info ProjectInfo
	var httpsURL string
	err := l.db.QueryRowContext(ctx, q, projectID, tenantID).Scan(
		&info.ProjectID, &info.TenantID, &httpsURL, &info.DefaultBranch,
		&info.CreatedBy,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrProjectNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("workspace: project lookup (%d, %d): %w", tenantID, projectID, err)
	}

	// Convert stored HTTPS URL to SSH form. Non-GitHub URLs return
	// ErrRepoURLUnsupported -- Round 2 only supports GitHub.
	sshURL, err := HTTPSToSSHURL(httpsURL)
	if err != nil {
		return nil, fmt.Errorf("workspace: project %d: %w", projectID, err)
	}
	info.SSHURL = sshURL

	return &info, nil
}

// GetOwnerGitHubToken fetches the PAT for the project's owning user.
// Called only on the one-time deploy-key upload path.
func (l *DBProjectLookup) GetOwnerGitHubToken(ctx context.Context, projectID int64) (string, error) {
	const q = `
		SELECT COALESCE(ui.access_token, '')
		FROM engine.projects p
		LEFT JOIN auth.user_identities ui
		       ON ui.user_id = p.created_by AND ui.provider = 'github'
		WHERE p.id = $1
	`
	var token string
	err := l.db.QueryRowContext(ctx, q, projectID).Scan(&token)
	if errors.Is(err, sql.ErrNoRows) {
		return "", ErrProjectNotFound
	}
	if err != nil {
		return "", fmt.Errorf("workspace: get github token for project %d: %w", projectID, err)
	}
	if token == "" {
		return "", fmt.Errorf("workspace: no github token found for project %d owner", projectID)
	}
	return token, nil
}

// StaticProjectLookup is a test fixture that returns a fixed ProjectInfo.
type StaticProjectLookup struct {
	Info  *ProjectInfo
	Token string
	Err   error
}

// Lookup returns the preconfigured ProjectInfo or error.
func (s *StaticProjectLookup) Lookup(_ context.Context, _, _ int64) (*ProjectInfo, error) {
	return s.Info, s.Err
}

// GetOwnerGitHubToken returns the preconfigured token.
func (s *StaticProjectLookup) GetOwnerGitHubToken(_ context.Context, _ int64) (string, error) {
	if s.Token == "" {
		return "", ErrProjectNotFound
	}
	return s.Token, nil
}

// memoryLookup is an in-memory ProjectLookup keyed by projectID. Used
// by ensure_test.go to wire up test fixtures without a database.
type memoryLookup struct {
	projects  map[int64]*ProjectInfo
	tokens    map[int64]string
	lookupErr error // if set, all lookups return this error
}

// Lookup returns the ProjectInfo for projectID, or ErrProjectNotFound
// if no entry exists (and lookupErr is nil).
func (m *memoryLookup) Lookup(_ context.Context, _, projectID int64) (*ProjectInfo, error) {
	if m.lookupErr != nil {
		return nil, m.lookupErr
	}
	info, ok := m.projects[projectID]
	if !ok {
		return nil, ErrProjectNotFound
	}
	return info, nil
}

// GetOwnerGitHubToken returns the preconfigured token for a project.
func (m *memoryLookup) GetOwnerGitHubToken(_ context.Context, projectID int64) (string, error) {
	t, ok := m.tokens[projectID]
	if !ok {
		return "", ErrProjectNotFound
	}
	return t, nil
}
