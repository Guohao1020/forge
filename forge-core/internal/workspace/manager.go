package workspace

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Manager handles local git clones and per-task worktrees.
// Directory layout:
//
//	WORKSPACE_ROOT/
//	  tenant-{tenantId}/
//	    project-{projectId}/
//	      repo/                  <- shared git clone
//	      tasks/
//	        task-{taskId}/       <- git worktree per task
type Manager struct {
	root string // FORGE_WORKSPACE_ROOT
}

// NewManager creates a workspace manager rooted at the given path.
// If root is empty, defaults to "/data/forge/workspaces".
func NewManager(root string) *Manager {
	if root == "" {
		root = "/data/forge/workspaces"
	}
	return &Manager{root: root}
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

// EnsureClone clones the repo if not already present, otherwise pulls latest.
// Returns the path to the shared repo directory.
func (m *Manager) EnsureClone(ctx context.Context, tenantID, projectID int64, repoURL, token, defaultBranch string) (string, error) {
	dir := m.ProjectDir(tenantID, projectID)

	if _, err := os.Stat(filepath.Join(dir, ".git")); err == nil {
		// Already cloned — pull latest on default branch
		slog.Info("workspace: pulling latest", "project_id", projectID, "dir", dir)
		cmd := exec.CommandContext(ctx, "git", "-C", dir, "pull", "--ff-only")
		cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
		if out, err := cmd.CombinedOutput(); err != nil {
			slog.Warn("workspace: git pull failed, continuing with existing clone",
				"project_id", projectID, "error", err, "output", string(out))
			// Non-fatal: continue with existing clone
		}
		return dir, nil
	}

	// Clone fresh
	if err := os.MkdirAll(filepath.Dir(dir), 0755); err != nil {
		return "", fmt.Errorf("create parent dir: %w", err)
	}

	authURL := injectToken(repoURL, token)
	slog.Info("workspace: cloning repo", "project_id", projectID, "dir", dir)
	cmd := exec.CommandContext(ctx, "git", "clone", "--depth=50", "--branch", defaultBranch, authURL, dir)
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
	if out, err := cmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("git clone failed: %s: %w", string(out), err)
	}

	return dir, nil
}

// CreateWorktree creates a git worktree for a task on a new branch.
// If a worktree already exists at that path, it is removed first.
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

// injectToken converts https://github.com/owner/repo to
// https://x-access-token:TOKEN@github.com/owner/repo.
// Token is never logged.
func injectToken(repoURL, token string) string {
	if token == "" {
		return repoURL
	}
	return strings.Replace(repoURL, "https://", fmt.Sprintf("https://x-access-token:%s@", token), 1)
}
