package main

import (
	"path/filepath"
	"testing"

	"github.com/shulex/forge/forge-core/internal/workspace"
)

// TestWorkspaceWiring_Phase1a verifies that the Phase 1a Manager can be
// constructed with all production dependencies (modulo DB -- that's an
// integration concern). Catches future regressions in the Config shape
// and dependency types without needing a live database.
func TestWorkspaceWiring_Phase1a(t *testing.T) {
	mgr := workspace.NewManager(workspace.Config{
		Root:       "/tmp/test-ws",
		StateRepo:  nil, // nil is allowed; EnsureReady returns an error
		GitRunner:  workspace.NewRealGitRunner(),
		PrepClient: workspace.NewPrepRunnerAdapter(workspace.NewPrepClient("http://127.0.0.1:0")),
	})
	if mgr == nil {
		t.Fatal("NewManager returned nil")
	}
	want := filepath.Join("/tmp/test-ws", "tenant-1", "project-2", "repo")
	if dir := mgr.ProjectDir(1, 2); dir != want {
		t.Errorf("ProjectDir: got %s, want %s", dir, want)
	}
}
