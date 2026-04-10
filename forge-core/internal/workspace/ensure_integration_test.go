package workspace

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
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
// can verify fetch + reset picks up new history.
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

// fileGitRunner is a test GitRunner that uses file:// URLs without SSH.
// It shells out to git without GIT_SSH_COMMAND because file:// protocol
// doesn't need SSH auth. This lets the integration test exercise the
// full state machine without requiring an SSH server.
type fileGitRunner struct{}

func (f *fileGitRunner) Clone(ctx context.Context, sshURL, dir string, key *DeployKey, branch string) error {
	_ = os.MkdirAll(filepath.Dir(dir), 0o755)
	_ = os.RemoveAll(dir)
	cmd := exec.CommandContext(ctx, "git", "clone", "--depth=50", "--branch", branch, sshURL, dir)
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return classifyGitError(err, string(out))
	}
	return nil
}

func (f *fileGitRunner) FetchAndResetHard(ctx context.Context, dir, branch string, key *DeployKey) error {
	fetch := exec.CommandContext(ctx, "git", "-C", dir, "fetch", "origin", branch)
	fetch.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
	if out, err := fetch.CombinedOutput(); err != nil {
		return classifyGitError(err, string(out))
	}
	reset := exec.CommandContext(ctx, "git", "-C", dir, "reset", "--hard", "origin/"+branch)
	if out, err := reset.CombinedOutput(); err != nil {
		return classifyGitError(err, string(out))
	}
	return nil
}

// integrationFixture wires a real Manager with real StateRepo, a
// fileGitRunner (no SSH), no prep client, and a memoryLookup pointing
// at a file:// URL.
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

	crypto, err := NewCryptoService(testMasterKey())
	if err != nil {
		t.Fatalf("NewCryptoService: %v", err)
	}

	lookup := &memoryLookup{
		projects: map[int64]*ProjectInfo{
			projectID: {
				ProjectID:     projectID,
				TenantID:      1,
				SSHURL:        fileURL, // file:// URL for local testing
				DefaultBranch: "main",
				CreatedBy:     1,
			},
		},
		tokens: map[int64]string{
			projectID: "ghp_integration_token",
		},
	}

	// For file:// integration tests, pre-seed a deploy key row so
	// ensureDeployKey finds it and doesn't try to do a real GitHub upload.
	deployKeyRepo := NewDeployKeyRepo(db, crypto)
	if err := deployKeyRepo.UpsertKey(context.Background(), 1, projectID, "ssh-ed25519 AAAA integration-test", []byte("fake-key-for-file-protocol"), 12345); err != nil {
		t.Fatalf("seed deploy key: %v", err)
	}

	mgr := &Manager{
		root:          rootDir,
		stateRepo:     NewStateRepo(db),
		deployKeys:    deployKeyRepo,
		crypto:        crypto,
		gitRunner:     &fileGitRunner{},
		prepClient:    nil, // skip prep
		ghUploader:    &fakeUploader{},
		projectLookup: lookup,
	}

	t.Cleanup(func() {
		_, _ = db.Exec(`DELETE FROM engine.workspaces WHERE project_id = $1`, projectID)
		_, _ = db.Exec(`DELETE FROM engine.project_deploy_keys WHERE project_id = $1`, projectID)
	})

	return &integrationFixture{
		manager: mgr,
		bareDir: bareDir,
		fileURL: fileURL,
		rootDir: rootDir,
	}
}

func TestEnsureReady_Integration_FirstClone(t *testing.T) {
	f := newIntegrationFixture(t, 501)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	ws, err := f.manager.EnsureReady(ctx, 1, 501, false)
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
	f := newIntegrationFixture(t, 502)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// First call: clone
	ws, err := f.manager.EnsureReady(ctx, 1, 502, false)
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

	// Second call with forceSync=true: should fetch + reset
	_, err = f.manager.EnsureReady(ctx, 1, 502, true)
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
	f := newIntegrationFixture(t, 503)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Force an initial error by pointing lookup at a nonexistent path
	f.manager.projectLookup = &memoryLookup{
		projects: map[int64]*ProjectInfo{
			503: {
				ProjectID:     503,
				TenantID:      1,
				SSHURL:        "file:///nonexistent/bare/repo/path",
				DefaultBranch: "main",
				CreatedBy:     1,
			},
		},
		tokens: map[int64]string{
			503: "ghp_integration_token",
		},
	}

	_, err := f.manager.EnsureReady(ctx, 1, 503, false)
	if err == nil {
		t.Fatal("expected error from nonexistent bare repo")
	}

	// Verify the row is in error state
	row, _ := f.manager.stateRepo.GetByProject(ctx, 1, 503)
	if row == nil || row.Status != StatusError {
		t.Fatalf("expected error row, got %+v", row)
	}

	// Now fix the lookup and retry -- should recover
	f.manager.projectLookup = &memoryLookup{
		projects: map[int64]*ProjectInfo{
			503: {
				ProjectID:     503,
				TenantID:      1,
				SSHURL:        f.fileURL,
				DefaultBranch: "main",
				CreatedBy:     1,
			},
		},
		tokens: map[int64]string{
			503: "ghp_integration_token",
		},
	}

	ws, err := f.manager.EnsureReady(ctx, 1, 503, false)
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
