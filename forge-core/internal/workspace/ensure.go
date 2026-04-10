package workspace

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// EnsureReady drives the workspace state machine for (tenantID, projectID).
// It guarantees that on successful return, the workspace row is in
// 'ready' state and the filesystem has a valid clone with dependencies
// pre-installed (or prep was skipped as a non-blocking soft failure).
//
// Behavior by starting state:
//
//	no row          -> create row('pending'), ensure deploy key, clone, prep, mark ready
//	row='pending'   -> wait on advisory lock, observe final state
//	row='ready' + forceSync=false -> no-op
//	row='ready' + forceSync=true  -> fetch + reset --hard
//	row='error'     -> wipe dir, reset row to 'pending', clone, prep, mark ready
//
// Concurrent callers for the same (tenant, project) serialize via
// pg_advisory_xact_lock. The second caller observes whatever state
// the first one left the row in.
//
// forceSync=true is driven by the agent service at the start of a
// new session (spec). It MUST NOT fire mid-session.
//
// PHASE 1B: git operations go through GitRunner using SSH deploy keys.
// Deploy key lifecycle: on first call per project, generate ed25519
// keypair, encrypt+store in DB, upload public key to GitHub, then
// clone. On subsequent calls, reuse the stored key.
func (m *Manager) EnsureReady(
	ctx context.Context,
	tenantID, projectID int64,
	forceSync bool,
) (*Workspace, error) {
	if m.stateRepo == nil || m.gitRunner == nil || m.projectLookup == nil {
		return nil, errors.New("workspace: EnsureReady called on partially-wired Manager (nil stateRepo/gitRunner/projectLookup)")
	}

	var finalWS *Workspace

	err := m.stateRepo.WithAdvisoryLock(ctx, tenantID, projectID, func(tx *sql.Tx) error {
		// Step 1: observe current state.
		existing, err := m.stateRepo.GetByProject(ctx, tenantID, projectID)
		if err != nil {
			return fmt.Errorf("ensure: get state: %w", err)
		}

		// Step 2: decide the action based on starting state.
		switch {
		case existing == nil:
			// Fresh install
			ws, err := m.freshInstall(ctx, tenantID, projectID)
			if err != nil {
				return err
			}
			finalWS = ws
			return nil

		case existing.Status == StatusReady && !forceSync:
			// Already ready, caller didn't ask for sync -- no-op
			finalWS = existing
			return nil

		case existing.Status == StatusReady && forceSync:
			// Resync: fetch + reset --hard
			ws, err := m.resync(ctx, existing, tenantID, projectID)
			if err != nil {
				return err
			}
			finalWS = ws
			return nil

		case existing.Status == StatusError:
			// Previous attempt failed -- wipe and retry from scratch.
			if err := m.stateRepo.ResetToPending(ctx, tenantID, projectID); err != nil {
				return fmt.Errorf("ensure: reset to pending: %w", err)
			}
			// Wipe the directory so freshInstall sees a clean slate
			dir := m.ProjectDir(tenantID, projectID)
			if err := os.RemoveAll(dir); err != nil {
				slog.Warn("workspace: failed to wipe error dir", "dir", dir, "error", err)
			}
			ws, err := m.freshInstall(ctx, tenantID, projectID)
			if err != nil {
				return err
			}
			finalWS = ws
			return nil

		case existing.Status == StatusPending:
			// A different caller is currently in the middle of an EnsureReady.
			// Under advisory lock this shouldn't be observable -- if we hold
			// the lock and see pending, it means a previous run crashed after
			// INSERT but before any state transition. Treat as crashed.
			slog.Warn("workspace: observed pending row under lock; treating as crashed run",
				"tenant", tenantID, "project", projectID)
			if err := m.stateRepo.ResetToPending(ctx, tenantID, projectID); err != nil {
				return fmt.Errorf("ensure: reset crashed pending: %w", err)
			}
			dir := m.ProjectDir(tenantID, projectID)
			_ = os.RemoveAll(dir)
			ws, err := m.freshInstall(ctx, tenantID, projectID)
			if err != nil {
				return err
			}
			finalWS = ws
			return nil
		}

		return fmt.Errorf("ensure: unreachable state: %+v", existing)
	})

	if err != nil {
		return nil, err
	}
	return finalWS, nil
}

// freshInstall performs the full "never been here before" flow:
//  1. Look up project metadata via ProjectLookup (SSHURL + branch)
//  2. Ensure deploy key exists (generate + upload if needed)
//  3. Insert pending row
//  4. MkdirAll the parent dir
//  5. Clone the repo via RealGitRunner (SSH deploy key)
//  6. Call ai-worker prep (non-blocking)
//  7. Mark ready
func (m *Manager) freshInstall(
	ctx context.Context,
	tenantID, projectID int64,
) (*Workspace, error) {
	proj, err := m.projectLookup.Lookup(ctx, tenantID, projectID)
	if err != nil {
		m.markErrorOrLog(ctx, tenantID, projectID, fmt.Sprintf("project lookup: %v", err))
		return nil, fmt.Errorf("ensure: project lookup: %w", err)
	}

	// Ensure deploy key exists: on first call per project, generate +
	// upload + store. On subsequent calls (including error recovery),
	// reuse the stored key.
	dk, err := m.ensureDeployKey(ctx, proj)
	if err != nil {
		m.markErrorOrLog(ctx, tenantID, projectID, fmt.Sprintf("deploy key: %v", err))
		return nil, fmt.Errorf("ensure: deploy key: %w", err)
	}

	hostPath := m.ProjectDir(tenantID, projectID)
	containerPath := m.containerProjectDir(tenantID, projectID)

	// InsertPending is idempotent, so it's safe to call here whether or
	// not the row was just reset by the caller above.
	if err := m.stateRepo.InsertPending(ctx, tenantID, projectID, hostPath, containerPath); err != nil {
		return nil, fmt.Errorf("ensure: insert pending: %w", err)
	}

	// Make sure the parent directory exists before clone
	if err := os.MkdirAll(filepath.Dir(hostPath), 0755); err != nil {
		m.markErrorOrLog(ctx, tenantID, projectID, fmt.Sprintf("mkdir parent: %v", err))
		return nil, fmt.Errorf("ensure: mkdir parent: %w", err)
	}

	// Clone via SSH deploy key
	if err := m.gitRunner.Clone(ctx, proj.SSHURL, hostPath, dk, proj.DefaultBranch); err != nil {
		reason := formatCloneError(err)
		m.markErrorOrLog(ctx, tenantID, projectID, reason)
		return nil, fmt.Errorf("ensure: clone: %w", err)
	}

	// Dep prep -- non-blocking
	wsRelPath := m.relPath(tenantID, projectID)
	if m.prepClient != nil {
		prepRes, prepErr := m.prepClient.Prep(ctx, tenantID, projectID, wsRelPath)
		if prepErr != nil {
			slog.Warn("workspace: dep prep transport error; proceeding to ready",
				"tenant", tenantID, "project", projectID, "error", prepErr)
		} else if prepRes != nil && prepRes.Status == "error" {
			slog.Warn("workspace: dep prep failed; proceeding to ready",
				"tenant", tenantID, "project", projectID, "reason", prepRes.Error)
		}
	}

	if err := m.stateRepo.MarkReady(ctx, tenantID, projectID); err != nil {
		return nil, fmt.Errorf("ensure: mark ready: %w", err)
	}

	// Return the updated row
	return m.stateRepo.GetByProject(ctx, tenantID, projectID)
}

// ensureDeployKey checks the DB for an existing deploy key for the project.
// If none exists, generates a new ed25519 keypair, uploads the public key
// to GitHub, and stores the encrypted private key in the DB.
func (m *Manager) ensureDeployKey(
	ctx context.Context,
	proj *ProjectInfo,
) (*DeployKey, error) {
	// Check for existing key
	if m.deployKeys != nil {
		existing, err := m.deployKeys.GetByProject(ctx, proj.TenantID, proj.ProjectID)
		if err != nil {
			return nil, fmt.Errorf("lookup existing deploy key: %w", err)
		}
		if existing != nil {
			return existing, nil
		}
	}

	// No existing key -- generate, upload, and store
	return m.generateAndUploadDeployKey(ctx, proj)
}

// generateAndUploadDeployKey creates a new ed25519 keypair, uploads the
// public key to GitHub, and stores the encrypted private key in the DB.
func (m *Manager) generateAndUploadDeployKey(
	ctx context.Context,
	proj *ProjectInfo,
) (*DeployKey, error) {
	if m.deployKeys == nil || m.ghUploader == nil {
		return nil, errors.New("deploy key generation requires deployKeys DAO and ghUploader")
	}

	comment := fmt.Sprintf("forge-deploy-%d-%d-%d", proj.TenantID, proj.ProjectID, time.Now().Unix())
	pub, priv, err := GenerateKeyPair(comment)
	if err != nil {
		return nil, fmt.Errorf("generate keypair: %w", err)
	}

	token, err := m.projectLookup.GetOwnerGitHubToken(ctx, proj.ProjectID)
	if err != nil {
		return nil, fmt.Errorf("get owner GitHub token: %w", err)
	}

	owner, repo, err := parseRepoFromSSHURL(proj.SSHURL)
	if err != nil {
		return nil, fmt.Errorf("parse ssh url: %w", err)
	}

	title := fmt.Sprintf("Forge: tenant %d project %d", proj.TenantID, proj.ProjectID)
	ghID, err := m.ghUploader.Upload(ctx, token, owner, repo, title, pub, false)
	if err != nil {
		return nil, fmt.Errorf("github deploy key upload: %w", err)
	}

	// Upsert into the DAO -- replaces any stale row.
	if err := m.deployKeys.UpsertKey(ctx, proj.TenantID, proj.ProjectID, pub, priv, ghID); err != nil {
		return nil, fmt.Errorf("deploy key upsert: %w", err)
	}

	// Re-read to return a populated struct with CreatedAt etc.
	return m.deployKeys.GetByProject(ctx, proj.TenantID, proj.ProjectID)
}

// resync performs a fetch + reset --hard on an already-ready workspace.
// If either step fails, falls back to wipe + re-clone via freshInstall.
func (m *Manager) resync(
	ctx context.Context,
	existing *Workspace,
	tenantID, projectID int64,
) (*Workspace, error) {
	proj, err := m.projectLookup.Lookup(ctx, tenantID, projectID)
	if err != nil {
		return nil, fmt.Errorf("resync: project lookup: %w", err)
	}

	// Resolve the deploy key for fetch
	dk, err := m.ensureDeployKey(ctx, proj)
	if err != nil {
		return nil, fmt.Errorf("resync: deploy key: %w", err)
	}

	if err := m.gitRunner.FetchAndResetHard(ctx, existing.HostPath, proj.DefaultBranch, dk); err != nil {
		slog.Warn("workspace: fetch+reset failed; falling back to fresh clone",
			"tenant", tenantID, "project", projectID, "error", err)
		return m.wipeAndReclone(ctx, tenantID, projectID, existing.HostPath)
	}

	// Update last_synced_at
	if err := m.stateRepo.MarkReady(ctx, tenantID, projectID); err != nil {
		return nil, fmt.Errorf("resync: mark ready: %w", err)
	}
	return m.stateRepo.GetByProject(ctx, tenantID, projectID)
}

// wipeAndReclone is the fall-back used when resync's fetch or reset
// fails: state goes back to pending, directory is wiped, freshInstall
// runs.
func (m *Manager) wipeAndReclone(
	ctx context.Context,
	tenantID, projectID int64,
	hostPath string,
) (*Workspace, error) {
	if err := m.stateRepo.ResetToPending(ctx, tenantID, projectID); err != nil {
		return nil, fmt.Errorf("wipe: reset to pending: %w", err)
	}
	_ = os.RemoveAll(hostPath)
	return m.freshInstall(ctx, tenantID, projectID)
}

// markErrorOrLog updates the row to 'error' with reason. Logs if the
// UPDATE itself fails.
func (m *Manager) markErrorOrLog(ctx context.Context, tenantID, projectID int64, reason string) {
	if err := m.stateRepo.MarkError(ctx, tenantID, projectID, reason); err != nil {
		slog.Error("workspace: failed to mark row as error",
			"tenant", tenantID, "project", projectID, "reason", reason, "error", err)
	}
}

// formatCloneError turns a GitRunner error into the persisted
// last_error string.
func formatCloneError(err error) string {
	var authErr *AuthError
	var netErr *NetworkError
	switch {
	case errors.As(err, &authErr):
		return "deploy_key_auth_failed: " + firstLine(authErr.stderr)
	case errors.As(err, &netErr):
		return "clone failed: network: " + firstLine(netErr.stderr)
	default:
		return "clone failed: " + err.Error()
	}
}

// isSupportedRepoURL returns true for URLs that can be cloned.
// Accepts git@ SSH (production), https:// GitHub (converted by
// HTTPSToSSHURL), and file:// (integration tests).
func isSupportedRepoURL(url string) bool {
	return strings.HasPrefix(url, "git@") ||
		strings.HasPrefix(url, "https://github.com/") ||
		strings.HasPrefix(url, "file://")
}

// relPath returns the relative workspace path fragment sent to ai-worker
// via the RunRequest.workspace_path field.
func (m *Manager) relPath(tenantID, projectID int64) string {
	return fmt.Sprintf("tenant-%d/project-%d/repo", tenantID, projectID)
}

// containerProjectDir returns the absolute path as seen inside the
// ai-worker container.
func (m *Manager) containerProjectDir(tenantID, projectID int64) string {
	return filepath.Join("/data/forge/workspaces",
		fmt.Sprintf("tenant-%d", tenantID),
		fmt.Sprintf("project-%d", projectID),
		"repo",
	)
}
