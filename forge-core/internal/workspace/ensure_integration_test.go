package workspace

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// gitAvailable skips the test if git isn't on PATH.
func gitAvailable(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git binary not available; skipping integration test")
	}
}

// seedBareRepo creates a bare git repo at bareDir with one commit on
// branch "main" and returns the file:// URL.
func seedBareRepo(t *testing.T, bareDir string) string {
	t.Helper()
	workDir := t.TempDir()

	// Initialize work dir
	run := func(dir, name string, args ...string) {
		cmd := exec.Command(name, args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=Forge Test",
			"GIT_AUTHOR_EMAIL=test@forge.local",
			"GIT_COMMITTER_NAME=Forge Test",
			"GIT_COMMITTER_EMAIL=test@forge.local",
		)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("%s %v: %s: %v", name, args, string(out), err)
		}
	}

	run(workDir, "git", "init", "-q", "-b", "main")
	if err := os.WriteFile(filepath.Join(workDir, "README.md"), []byte("initial"), 0644); err != nil {
		t.Fatalf("write README: %v", err)
	}
	run(workDir, "git", "add", "README.md")
	run(workDir, "git", "commit", "-q", "-m", "initial")

	// Initialize bare repo and push
	run(bareDir, "git", "init", "-q", "--bare", "-b", "main")
	run(workDir, "git", "remote", "add", "origin", bareDir)
	run(workDir, "git", "push", "-q", "origin", "main")

	return "file://" + filepath.ToSlash(bareDir)
}

// addSecondCommit adds a second commit to the bare repo so resync tests
// can verify fetch + reset picks up new history. The working directory
// for this commit is fresh -- we clone the bare repo, add a file, push.
func addSecondCommit(t *testing.T, bareDir string) {
	t.Helper()
	workDir := t.TempDir()
	run := func(dir, name string, args ...string) {
		cmd := exec.Command(name, args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=Forge Test",
			"GIT_AUTHOR_EMAIL=test@forge.local",
			"GIT_COMMITTER_NAME=Forge Test",
			"GIT_COMMITTER_EMAIL=test@forge.local",
			"GIT_TERMINAL_PROMPT=0",
		)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("%s %v: %s: %v", name, args, string(out), err)
		}
	}
	run(workDir, "git", "clone", "-q", bareDir, ".")
	if err := os.WriteFile(filepath.Join(workDir, "CHANGES.md"), []byte("second commit"), 0644); err != nil {
		t.Fatalf("write CHANGES: %v", err)
	}
	run(workDir, "git", "add", "CHANGES.md")
	run(workDir, "git", "commit", "-q", "-m", "second")
	run(workDir, "git", "push", "-q", "origin", "main")
}

// integrationFixture wires a real Manager with real StateRepo, real
// RealGitRunner, no prep client (nil), and a memoryLookup pointing at
// a file:// URL. Uses a dummy token -- the file:// URL doesn't exercise
// HTTPS auth, so gitInjectToken's output is benign.
type integrationFixture struct {
	manager *Manager
	bareDir string
	fileURL string
	rootDir string
}

func newIntegrationFixture(t *testing.T, projectID int64) *integrationFixture {
	t.Helper()
	gitAvailable(t)
	db := openTestDB(t)

	bareDir := t.TempDir()
	fileURL := seedBareRepo(t, bareDir)
	rootDir := t.TempDir()

	lookup := &memoryLookup{
		projects: map[int64]ProjectInfo{
			projectID: {
				RepoURL:     fileURL,
				AccessToken: "dummy-token",
				Branch:      "main",
			},
		},
	}

	// Stub prep client -- nil. EnsureReady treats nil prepClient as
	// "skip prep, mark ready".
	mgr := &Manager{
		root:          rootDir,
		stateRepo:     NewStateRepo(db),
		gitRunner:     NewRealGitRunner(),
		prepClient:    nil,
		projectLookup: lookup,
	}

	t.Cleanup(func() {
		_, _ = db.Exec(`DELETE FROM engine.workspaces WHERE project_id = $1`, projectID)
	})

	return &integrationFixture{
		manager: mgr,
		bareDir: bareDir,
		fileURL: fileURL,
		rootDir: rootDir,
	}
}

func TestEnsureReady_Integration_FirstClone(t *testing.T) {
	f := newIntegrationFixture(t, 301)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	ws, err := f.manager.EnsureReady(ctx, 1, 301, false)
	if err != nil {
		t.Fatalf("EnsureReady: %v", err)
	}
	if ws.Status != StatusReady {
		t.Errorf("status: want ready, got %s", ws.Status)
	}

	// Verify the clone landed on disk
	readme := filepath.Join(ws.HostPath, "README.md")
	content, err := os.ReadFile(readme)
	if err != nil {
		t.Fatalf("read README: %v", err)
	}
	if string(content) != "initial" {
		t.Errorf("README content: got %q, want 'initial'", string(content))
	}
}

func TestEnsureReady_Integration_Resync(t *testing.T) {
	f := newIntegrationFixture(t, 302)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// First call: clone
	ws, err := f.manager.EnsureReady(ctx, 1, 302, false)
	if err != nil {
		t.Fatalf("first EnsureReady: %v", err)
	}
	hostPath := ws.HostPath

	// Modify the local clone
	localChange := filepath.Join(hostPath, "README.md")
	if err := os.WriteFile(localChange, []byte("local modification"), 0644); err != nil {
		t.Fatalf("write local change: %v", err)
	}

	// Add a second commit to the bare repo so the fetch has something to pull
	addSecondCommit(t, f.bareDir)

	// Second call with forceSync=true: should fetch + reset, landing
	// on "initial" content (not "local modification") AND introducing
	// the new CHANGES.md file from the bare repo.
	_, err = f.manager.EnsureReady(ctx, 1, 302, true)
	if err != nil {
		t.Fatalf("resync EnsureReady: %v", err)
	}

	// README should be back to "initial"
	content, err := os.ReadFile(localChange)
	if err != nil {
		t.Fatalf("read README after resync: %v", err)
	}
	if string(content) != "initial" {
		t.Errorf("README after resync: got %q, want 'initial'", string(content))
	}

	// CHANGES.md from second commit should now be present
	changes := filepath.Join(hostPath, "CHANGES.md")
	if _, err := os.Stat(changes); err != nil {
		t.Errorf("CHANGES.md missing after resync: %v", err)
	}
}

func TestEnsureReady_Integration_ErrorRecovery(t *testing.T) {
	f := newIntegrationFixture(t, 303)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Force an initial error by pointing lookup at a nonexistent path
	f.manager.projectLookup = &memoryLookup{
		projects: map[int64]ProjectInfo{
			303: {
				RepoURL:     "file:///nonexistent/bare/repo/path",
				AccessToken: "dummy-token",
				Branch:      "main",
			},
		},
	}

	_, err := f.manager.EnsureReady(ctx, 1, 303, false)
	if err == nil {
		t.Fatal("expected error from nonexistent bare repo")
	}

	// Verify the row is in error state
	row, _ := f.manager.stateRepo.GetByProject(ctx, 1, 303)
	if row == nil || row.Status != StatusError {
		t.Fatalf("expected error row, got %+v", row)
	}

	// Now fix the lookup and retry -- should recover
	f.manager.projectLookup = &memoryLookup{
		projects: map[int64]ProjectInfo{
			303: {
				RepoURL:     f.fileURL,
				AccessToken: "dummy-token",
				Branch:      "main",
			},
		},
	}

	ws, err := f.manager.EnsureReady(ctx, 1, 303, false)
	if err != nil {
		t.Fatalf("recovery EnsureReady: %v", err)
	}
	if ws.Status != StatusReady {
		t.Errorf("status after recovery: want ready, got %s", ws.Status)
	}

	// Verify the clone actually landed
	readme := filepath.Join(ws.HostPath, "README.md")
	if _, err := os.Stat(readme); err != nil {
		t.Errorf("README missing after recovery: %v", err)
	}
}

// Phase 1a-specific: unsupported repo URL surfaces as repo_url_unsupported.
func TestEnsureReady_Integration_UnsupportedURL(t *testing.T) {
	f := newIntegrationFixture(t, 304)

	// Swap the lookup with a non-HTTPS, non-file:// URL
	f.manager.projectLookup = &memoryLookup{
		projects: map[int64]ProjectInfo{
			304: {
				RepoURL:     "gopher://example.com/repo",
				AccessToken: "dummy-token",
				Branch:      "main",
			},
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	_, err := f.manager.EnsureReady(ctx, 1, 304, false)
	if err == nil {
		t.Fatal("expected error for unsupported URL scheme")
	}

	row, _ := f.manager.stateRepo.GetByProject(ctx, 1, 304)
	if row == nil || row.Status != StatusError {
		t.Fatalf("expected error row, got %+v", row)
	}
	if row.LastError == nil || !strings.Contains(*row.LastError, "repo_url_unsupported") {
		t.Errorf("last_error should contain 'repo_url_unsupported'; got %v", row.LastError)
	}
}
