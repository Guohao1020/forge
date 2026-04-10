// Package workspace owns the physical code artifact for each project.
//
// It handles:
//   - Cloning repos on first access (see EnsureReady in ensure.go)
//   - Dependency pre-install via RPC to ai-worker
//   - Per-task git worktrees for parallel work
//   - File write helpers for AI-generated code
//
// Directory layout (both on host and inside ai-worker container):
//
//	WORKSPACE_ROOT/
//	  tenant-{tenantId}/
//	    project-{projectId}/
//	      repo/                  <- shared git clone, managed by EnsureReady
//	      tasks/
//	        task-{taskId}/       <- git worktree per task
//
// Callers interact via the Manager struct. Manager is constructed
// with a Config that wires in all dependencies; nil dependencies
// disable the corresponding capability (e.g., a Manager with nil
// stateRepo cannot call EnsureReady but can still use ProjectDir).
//
// PHASE 1A: auth is HTTPS+token via RealGitRunner's internal gitInjectToken.
// Phase 1b rewrites this to SSH deploy keys; see docs/specs/... §2.9.4.
package workspace

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
)

// Config bundles Manager dependencies. Passing a struct avoids a
// 5-parameter NewManager call and makes it clear what's optional
// (nil stateRepo/gitRunner/prepClient/projectLookup all degrade
// gracefully — EnsureReady returns a descriptive error).
type Config struct {
	Root          string        // FORGE_WORKSPACE_ROOT; defaults to /data/forge/workspaces
	StateRepo     *StateRepo    // engine.workspaces DAO; nil disables EnsureReady
	GitRunner     gitRunner     // HTTPS+token git wrapper; typically *RealGitRunner
	PrepClient    prepRunner    // ai-worker /api/workspace/prep client; typically *PrepRunnerAdapter
	ProjectLookup ProjectLookup // project metadata + HTTPS URL + token
}

// Manager handles local git clones and per-task worktrees.
type Manager struct {
	root          string
	stateRepo     *StateRepo
	gitRunner     gitRunner
	prepClient    prepRunner
	projectLookup ProjectLookup
}

// NewManager creates a workspace manager from a Config. If cfg.Root is
// empty, defaults to "/data/forge/workspaces". Nil dependency fields
// are allowed — EnsureReady will return a descriptive error if called
// on a Manager missing any of them.
func NewManager(cfg Config) *Manager {
	root := cfg.Root
	if root == "" {
		root = "/data/forge/workspaces"
	}
	return &Manager{
		root:          root,
		stateRepo:     cfg.StateRepo,
		gitRunner:     cfg.GitRunner,
		prepClient:    cfg.PrepClient,
		projectLookup: cfg.ProjectLookup,
	}
}

// ProjectDir returns the shared repo directory for a project.
func (m *Manager) ProjectDir(tenantID, projectID int64) string {
	return filepath.Join(m.root,
		fmt.Sprintf("tenant-%d", tenantID),
		fmt.Sprintf("project-%d", projectID),
		"repo",
	)
}

// TaskDir returns the worktree directory for a task.
func (m *Manager) TaskDir(tenantID, projectID, taskID int64) string {
	return filepath.Join(m.root,
		fmt.Sprintf("tenant-%d", tenantID),
		fmt.Sprintf("project-%d", projectID),
		"tasks",
		fmt.Sprintf("task-%d", taskID),
	)
}

// CreateWorktree creates a git worktree for a task on a new branch.
// If a worktree already exists at that path, it is removed first.
//
// Unchanged from the pre-A2 Manager — the temporal worker still uses
// worktrees for task-level isolation, and that flow is untouched by
// the Variant B refactor.
func (m *Manager) CreateWorktree(ctx context.Context, tenantID, projectID, taskID int64, branchName string) (string, error) {
	repoDir := m.ProjectDir(tenantID, projectID)
	taskDir := m.TaskDir(tenantID, projectID, taskID)

	// Remove existing worktree if present
	if _, err := os.Stat(taskDir); err == nil {
		slog.Info("workspace: removing existing worktree", "task_id", taskID, "dir", taskDir)
		_ = exec.CommandContext(ctx, "git", "-C", repoDir, "worktree", "remove", "--force", taskDir).Run()
		_ = os.RemoveAll(taskDir)
	}

	if err := os.MkdirAll(filepath.Dir(taskDir), 0755); err != nil {
		return "", fmt.Errorf("create tasks dir: %w", err)
	}

	// Create new branch and worktree
	cmd := exec.CommandContext(ctx, "git", "-C", repoDir, "worktree", "add", "-b", branchName, taskDir)
	if out, err := cmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("create worktree: %s: %w", string(out), err)
	}

	slog.Info("workspace: worktree created", "task_id", taskID, "branch", branchName, "dir", taskDir)
	return taskDir, nil
}

// FileToWrite represents a file to be written to the workspace.
type FileToWrite struct {
	Path    string
	Content string
}

// WriteFiles writes AI-generated files to the task worktree.
func (m *Manager) WriteFiles(taskDir string, files []FileToWrite) error {
	for _, f := range files {
		fullPath := filepath.Join(taskDir, f.Path)
		if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
			return fmt.Errorf("mkdir for %s: %w", f.Path, err)
		}
		if err := os.WriteFile(fullPath, []byte(f.Content), 0644); err != nil {
			return fmt.Errorf("write %s: %w", f.Path, err)
		}
	}
	return nil
}

// CleanupTask removes a task worktree and its branch.
func (m *Manager) CleanupTask(ctx context.Context, tenantID, projectID, taskID int64) error {
	repoDir := m.ProjectDir(tenantID, projectID)
	taskDir := m.TaskDir(tenantID, projectID, taskID)

	_ = exec.CommandContext(ctx, "git", "-C", repoDir, "worktree", "remove", "--force", taskDir).Run()
	return os.RemoveAll(taskDir)
}

// SetLookup wires in the ProjectLookup after Manager construction.
// Needed because projectService depends on Manager.ProjectDir while
// Manager.EnsureReady depends on ProjectLookup — classic chicken-
// and-egg. main.go constructs Manager first (without Lookup), then
// projectService, then SetLookup.
func (m *Manager) SetLookup(lookup ProjectLookup) {
	m.projectLookup = lookup
}
