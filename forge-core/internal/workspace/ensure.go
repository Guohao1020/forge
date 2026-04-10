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
)

// EnsureReady drives the workspace state machine for (tenantID, projectID).
// It guarantees that on successful return, the workspace row is in
// 'ready' state and the filesystem has a valid clone with dependencies
// pre-installed (or prep was skipped as a non-blocking soft failure).
//
// Behavior by starting state:
//
//	no row          -> create row('pending'), clone, prep, mark ready
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
// new session (spec §2.7). It MUST NOT fire mid-session.
//
// PHASE 1A: git operations go through gitRunner using HTTPS+token auth.
// The state machine itself is auth-independent; Phase 1b swaps the
// gitRunner constructor without touching this file.
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
			// Already ready, caller didn't ask for sync — no-op
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
			// Previous attempt failed — wipe and retry from scratch.
			if err := m.stateRepo.ResetToPending(ctx, tenantID, projectID); err != nil {
				return fmt.Errorf("ensure: reset to pending: %w", err)
			}
			// Wipe the directory so freshInstall sees a clean slate
			dir := m.ProjectDir(tenantID, projectID)
			if err := os.RemoveAll(dir); err != nil {
				slog.Warn("workspace: failed to wipe error dir", "dir", dir, "error", err)
				// Not fatal — freshInstall's git clone will wipe again via its own RemoveAll
			}
			ws, err := m.freshInstall(ctx, tenantID, projectID)
			if err != nil {
				return err
			}
			finalWS = ws
			return nil

		case existing.Status == StatusPending:
			// A different caller is currently in the middle of an EnsureReady.
			// Under advisory lock this shouldn't be observable — if we hold
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
//  1. Look up project metadata via ProjectLookup (HTTPS URL + PAT + branch)
//  2. Insert pending row
//  3. MkdirAll the parent dir
//  4. Clone the repo via RealGitRunner (HTTPS+token)
//  5. Call ai-worker prep (non-blocking)
//  6. Mark ready
//
// PHASE 1A: no deploy-key lifecycle. The entire "generate keypair +
// upload to GitHub" dance from Round 1's Task 1.7 is absent because
// HTTPS+token auth doesn't need it.
func (m *Manager) freshInstall(
	ctx context.Context,
	tenantID, projectID int64,
) (*Workspace, error) {
	proj, err := m.projectLookup.Lookup(ctx, tenantID, projectID)
	if err != nil {
		m.markErrorOrLog(ctx, tenantID, projectID, fmt.Sprintf("project lookup: %v", err))
		return nil, fmt.Errorf("ensure: project lookup: %w", err)
	}

	// Sanity-check the URL — Phase 1a only supports https:// or file://
	// (the latter for local integration tests). Non-recognizable URLs
	// surface as repo_url_unsupported so operators can see the failure
	// mode without digging through git stderr.
	if !isSupportedRepoURL(proj.RepoURL) {
		m.markErrorOrLog(ctx, tenantID, projectID, fmt.Sprintf("repo_url_unsupported: %s", proj.RepoURL))
		return nil, fmt.Errorf("ensure: unsupported repo URL: %s", proj.RepoURL)
	}

	hostPath := m.ProjectDir(tenantID, projectID)
	containerPath := m.containerProjectDir(tenantID, projectID)

	// InsertPending is idempotent, so it's safe to call here whether or
	// not the row was just reset by the caller above.
	if err := m.stateRepo.InsertPending(ctx, tenantID, projectID, hostPath, containerPath); err != nil {
		return nil, fmt.Errorf("ensure: insert pending: %w", err)
	}

	// Make sure the parent directory exists before clone (clone does
	// not MkdirAll its parent).
	if err := os.MkdirAll(filepath.Dir(hostPath), 0755); err != nil {
		m.markErrorOrLog(ctx, tenantID, projectID, fmt.Sprintf("mkdir parent: %v", err))
		return nil, fmt.Errorf("ensure: mkdir parent: %w", err)
	}

	// Clone via HTTPS+token. Error classification handles auth/network
	// failures so the state machine can produce a meaningful last_error.
	if err := m.gitRunner.Clone(ctx, hostPath, proj.RepoURL, proj.AccessToken, proj.Branch); err != nil {
		reason := formatCloneError(err)
		m.markErrorOrLog(ctx, tenantID, projectID, reason)
		return nil, fmt.Errorf("ensure: clone: %w", err)
	}

	// Dep prep — non-blocking
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

// resync performs a fetch + reset --hard on an already-ready workspace.
// If either step fails, falls back to wipe + re-clone via freshInstall.
//
// PHASE 1A: uses HTTPS+token via gitRunner.Fetch + gitRunner.ResetHard.
func (m *Manager) resync(
	ctx context.Context,
	existing *Workspace,
	tenantID, projectID int64,
) (*Workspace, error) {
	proj, err := m.projectLookup.Lookup(ctx, tenantID, projectID)
	if err != nil {
		return nil, fmt.Errorf("resync: project lookup: %w", err)
	}

	if err := m.gitRunner.Fetch(ctx, existing.HostPath, proj.RepoURL, proj.AccessToken); err != nil {
		slog.Warn("workspace: fetch failed; falling back to fresh clone",
			"tenant", tenantID, "project", projectID, "error", err)
		return m.wipeAndReclone(ctx, tenantID, projectID, existing.HostPath)
	}

	if err := m.gitRunner.ResetHard(ctx, existing.HostPath, proj.Branch); err != nil {
		slog.Warn("workspace: reset failed; falling back to fresh clone",
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
// runs. Keeps the transparent-recovery semantics from spec §3.12.
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
// UPDATE itself fails — at that point there's nothing else we can do,
// the caller will surface the original error regardless.
func (m *Manager) markErrorOrLog(ctx context.Context, tenantID, projectID int64, reason string) {
	if err := m.stateRepo.MarkError(ctx, tenantID, projectID, reason); err != nil {
		slog.Error("workspace: failed to mark row as error",
			"tenant", tenantID, "project", projectID, "reason", reason, "error", err)
	}
}

// formatCloneError turns a gitRunner error into the persisted
// last_error string. Phase 1a's §3.12 failure-mode matrix requires:
//   - AuthError  -> "github_auth_failed: <stderr-line>"
//   - NetworkError -> "clone failed: network: <stderr-line>"
//   - otherwise  -> "clone failed: <err.Error()>"
//
// Phase 1b removes the github_auth_failed branch (PAT usage ends with
// the SSH migration).
func formatCloneError(err error) string {
	var authErr *AuthError
	var netErr *NetworkError
	switch {
	case errors.As(err, &authErr):
		return "github_auth_failed: " + firstLine(authErr.stderr)
	case errors.As(err, &netErr):
		return "clone failed: network: " + firstLine(netErr.stderr)
	default:
		return "clone failed: " + err.Error()
	}
}

// isSupportedRepoURL returns true for URLs Phase 1a can clone.
// Accepts https:// (production) and file:// (integration tests).
// Phase 1b extends this to ssh:// and git@ forms.
func isSupportedRepoURL(url string) bool {
	return strings.HasPrefix(url, "https://") || strings.HasPrefix(url, "file://")
}

// relPath returns the relative workspace path fragment sent to ai-worker
// via the RunRequest.workspace_path field. This is the "Stream 4c
// protocol" format from spec §3.4: "tenant-{N}/project-{N}/repo".
func (m *Manager) relPath(tenantID, projectID int64) string {
	return fmt.Sprintf("tenant-%d/project-%d/repo", tenantID, projectID)
}

// containerProjectDir returns the absolute path as seen inside the
// ai-worker container. For now this is the same structure with a
// hardcoded container root; when forge-core moves into the compose
// network, this becomes more sophisticated.
func (m *Manager) containerProjectDir(tenantID, projectID int64) string {
	return filepath.Join("/data/forge/workspaces",
		fmt.Sprintf("tenant-%d", tenantID),
		fmt.Sprintf("project-%d", projectID),
		"repo",
	)
}
