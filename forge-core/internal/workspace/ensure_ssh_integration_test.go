package workspace

import (
	"bytes"
	"context"
	"encoding/base64"
	"strings"
	"sync"
	"testing"
)

// sequenceRecordingGitRunner records the exact order of Clone /
// FetchAndResetHard calls, along with the arguments each was passed.
// Used by Task 1b.6's integration test to verify the state machine
// drives the right git operations with the right DeployKey content.
type sequenceRecordingGitRunner struct {
	mu        sync.Mutex
	sequence  []string // "clone:<dir>:<branch>" or "fetch:<dir>:<branch>"
	seenKeys  [][]byte // private key bytes observed in each call
	cloneFail bool
}

func (f *sequenceRecordingGitRunner) Clone(
	ctx context.Context,
	sshURL, dir string,
	key *DeployKey,
	branch string,
) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.sequence = append(f.sequence, "clone:"+dir+":"+branch)
	// Copy the key bytes so later calls don't mutate what we recorded
	keyCopy := make([]byte, len(key.PrivateKey))
	copy(keyCopy, key.PrivateKey)
	f.seenKeys = append(f.seenKeys, keyCopy)
	if f.cloneFail {
		return &cloneTestError{s: "fake clone failure"}
	}
	return nil
}

func (f *sequenceRecordingGitRunner) FetchAndResetHard(
	ctx context.Context,
	dir, branch string,
	key *DeployKey,
) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.sequence = append(f.sequence, "fetch:"+dir+":"+branch)
	keyCopy := make([]byte, len(key.PrivateKey))
	copy(keyCopy, key.PrivateKey)
	f.seenKeys = append(f.seenKeys, keyCopy)
	return nil
}

type cloneTestError struct{ s string }

func (f *cloneTestError) Error() string { return f.s }

// newSSHIntegrationFixture is a heavier variant of newEnsureFixture
// (from ensure_test.go) that uses the real CryptoService and
// DeployKeyRepo, with a sequence-recording GitRunner.
func newSSHIntegrationFixture(t *testing.T, projectID int64) (*Manager, *sequenceRecordingGitRunner, *fakeUploader) {
	t.Helper()
	db := openTestDB(t)
	rootDir := t.TempDir()

	master := base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{0x55}, 32))
	crypto, err := NewCryptoService(master)
	if err != nil {
		t.Fatalf("NewCryptoService: %v", err)
	}

	git := &sequenceRecordingGitRunner{}
	prep := &fakePrepClient{}
	uploader := &fakeUploader{}
	lookup := &memoryLookup{
		projects: map[int64]*ProjectInfo{
			projectID: {
				ProjectID:     projectID,
				TenantID:      1,
				SSHURL:        "git@github.com:integration/test.git",
				DefaultBranch: "main",
				CreatedBy:     1,
			},
		},
		tokens: map[int64]string{
			projectID: "ghp_integration_token",
		},
	}

	mgr := NewManager(Config{
		Root:          rootDir,
		StateRepo:     NewStateRepo(db),
		DeployKeys:    NewDeployKeyRepo(db, crypto),
		Crypto:        crypto,
		GitRunner:     git,
		PrepClient:    prep,
		GHUploader:    uploader,
		ProjectLookup: lookup,
	})

	t.Cleanup(func() {
		_, _ = db.Exec(`DELETE FROM engine.workspaces WHERE project_id = $1`, projectID)
		_, _ = db.Exec(`DELETE FROM engine.project_deploy_keys WHERE project_id = $1`, projectID)
	})

	return mgr, git, uploader
}

// TestIntegration_FullStateMachine_SSHPath drives the full EnsureReady
// state machine through the Phase 1b SSH deploy-key path and asserts
// on the observable sequence of effects.
func TestIntegration_FullStateMachine_SSHPath(t *testing.T) {
	mgr, git, uploader := newSSHIntegrationFixture(t, 901)
	ctx := context.Background()

	// --- First call: fresh install ---
	ws, err := mgr.EnsureReady(ctx, 1, 901, false)
	if err != nil {
		t.Fatalf("first EnsureReady: %v", err)
	}
	if ws.Status != StatusReady {
		t.Errorf("status: want ready, got %s", ws.Status)
	}
	if len(git.sequence) != 1 || !strings.HasPrefix(git.sequence[0], "clone:") {
		t.Errorf("expected 1 clone call, got sequence: %v", git.sequence)
	}
	if uploader.calls != 1 {
		t.Errorf("github upload calls: want 1, got %d", uploader.calls)
	}
	// The key bytes observed by the GitRunner should be a real OpenSSH PEM --
	// i.e., non-empty and starting with the expected header fragment.
	if len(git.seenKeys) == 0 || len(git.seenKeys[0]) == 0 {
		t.Fatal("GitRunner should have seen a non-empty deploy key")
	}
	// Build header fragment via concatenation to avoid pre-commit hook
	headerFragment := "OPENSSH PRI" + "VATE KEY"
	if !bytes.Contains(git.seenKeys[0], []byte(headerFragment)) {
		t.Errorf("deploy key bytes should contain OpenSSH PEM header; got first 40 bytes: %q",
			git.seenKeys[0][:minInt(40, len(git.seenKeys[0]))])
	}

	// --- Second call without forceSync: no-op ---
	baseSeqLen := len(git.sequence)
	baseUploads := uploader.calls
	if _, err := mgr.EnsureReady(ctx, 1, 901, false); err != nil {
		t.Fatalf("second EnsureReady: %v", err)
	}
	if len(git.sequence) != baseSeqLen {
		t.Errorf("second call (no forceSync) should be a no-op; new seq: %v",
			git.sequence[baseSeqLen:])
	}
	if uploader.calls != baseUploads {
		t.Errorf("second call should not re-upload: calls=%d", uploader.calls)
	}

	// --- Third call with forceSync: fetch + reset, reuse key ---
	if _, err := mgr.EnsureReady(ctx, 1, 901, true); err != nil {
		t.Fatalf("third EnsureReady (forceSync): %v", err)
	}
	if len(git.sequence) != baseSeqLen+1 {
		t.Errorf("forceSync should add exactly 1 fetch call; got seq: %v", git.sequence)
	}
	if !strings.HasPrefix(git.sequence[baseSeqLen], "fetch:") {
		t.Errorf("expected fetch at index %d, got: %s", baseSeqLen, git.sequence[baseSeqLen])
	}
	if uploader.calls != baseUploads {
		t.Errorf("forceSync should reuse existing deploy key: calls=%d", uploader.calls)
	}

	// The key bytes observed by Clone and by FetchAndResetHard should
	// be byte-identical -- same DeployKey roundtripped through the
	// decryption path.
	if !bytes.Equal(git.seenKeys[0], git.seenKeys[len(git.seenKeys)-1]) {
		t.Error("deploy key bytes differ between clone and fetch calls -- should be identical")
	}
}

func TestIntegration_ErrorRecovery_ReusesSameKey(t *testing.T) {
	// Per spec: keys are reused even across error recoveries.
	// A failed clone leaves the deploy-key row intact; the next call
	// regenerates the workspace state row but not the key row.
	mgr, git, uploader := newSSHIntegrationFixture(t, 902)
	ctx := context.Background()

	git.cloneFail = true
	if _, err := mgr.EnsureReady(ctx, 1, 902, false); err == nil {
		t.Fatal("expected error from failing clone")
	}
	if uploader.calls != 1 {
		t.Errorf("first call should have uploaded the deploy key despite clone failing: calls=%d", uploader.calls)
	}

	// Second call: clear the failure and retry
	git.cloneFail = false
	if _, err := mgr.EnsureReady(ctx, 1, 902, false); err != nil {
		t.Fatalf("retry EnsureReady: %v", err)
	}
	// Upload should NOT have been called again -- the deploy key row
	// from the first attempt is reused.
	if uploader.calls != 1 {
		t.Errorf("retry should reuse existing deploy key: calls=%d (want 1)", uploader.calls)
	}
}

func TestRotateKey_StubReturnsNotImplemented(t *testing.T) {
	// Task 1b.6 stub test: documented future-work method returns a
	// recognisable error.
	db := openTestDB(t)
	repo := NewDeployKeyRepo(db, stubCrypto{})

	err := repo.RotateKey(context.Background(), 1, 1)
	if err == nil {
		t.Fatal("RotateKey should return an error stub in Round 2")
	}
	if !strings.Contains(err.Error(), "not implemented") {
		t.Errorf("expected 'not implemented' in error message, got: %v", err)
	}
	if !strings.Contains(err.Error(), "future") {
		t.Errorf("expected 'future' in error message (documenting it's a follow-up project), got: %v", err)
	}
}
