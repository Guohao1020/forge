package workspace

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

// --- Test fakes ---

type fakeGitRunner struct {
	mu              sync.Mutex
	cloneCalls      int
	fetchCalls      int
	cloneShouldFail bool
	fetchShouldFail bool
	clonedDirs      []string
	lastCloneBranch string
}

func (f *fakeGitRunner) Clone(ctx context.Context, sshURL, dir string, key *DeployKey, branch string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.cloneCalls++
	f.lastCloneBranch = branch
	f.clonedDirs = append(f.clonedDirs, dir)
	if f.cloneShouldFail {
		return &AuthError{stderr: "fake auth failure"}
	}
	// Simulate a successful clone by creating the dir with a .git marker
	_ = os.MkdirAll(filepath.Join(dir, ".git"), 0755)
	return nil
}

func (f *fakeGitRunner) FetchAndResetHard(ctx context.Context, dir, branch string, key *DeployKey) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.fetchCalls++
	if f.fetchShouldFail {
		return errors.New("fake fetch failed")
	}
	return nil
}

type fakePrepClient struct {
	calls       int
	shouldFail  bool
	shouldError bool
}

func (f *fakePrepClient) Prep(ctx context.Context, tenantID, projectID int64, wsPath string) (*PrepResult, error) {
	f.calls++
	if f.shouldFail {
		return nil, errors.New("fake prep transport failed")
	}
	if f.shouldError {
		return &PrepResult{Status: "error", Error: "install failed"}, nil
	}
	return &PrepResult{Status: "ok", Language: "go", Command: "go mod download"}, nil
}

type fakeUploader struct {
	calls      int
	shouldFail bool
	keyID      int64
}

func (f *fakeUploader) Upload(ctx context.Context, token, owner, repo, title, sshKey string, readOnly bool) (int64, error) {
	f.calls++
	if f.shouldFail {
		return 0, errors.New("fake upload failed: deploy key upload error")
	}
	if f.keyID != 0 {
		return f.keyID, nil
	}
	return 88888, nil
}

// --- Setup helper ---

type ensureTestFixture struct {
	manager  *Manager
	git      *fakeGitRunner
	prep     *fakePrepClient
	lookup   *memoryLookup
	uploader *fakeUploader
	rootDir  string
}

func newEnsureFixture(t *testing.T, projectID int64) *ensureTestFixture {
	t.Helper()
	db := openTestDB(t)
	rootDir := t.TempDir()

	git := &fakeGitRunner{}
	prep := &fakePrepClient{}
	uploader := &fakeUploader{}
	lookup := &memoryLookup{
		projects: map[int64]*ProjectInfo{
			projectID: {
				ProjectID:     projectID,
				TenantID:      1,
				SSHURL:        "git@github.com:owner/repo.git",
				DefaultBranch: "main",
				CreatedBy:     99,
			},
		},
		tokens: map[int64]string{
			projectID: "ghp_fake_token",
		},
	}

	crypto, err := NewCryptoService(testMasterKey())
	if err != nil {
		t.Fatalf("NewCryptoService: %v", err)
	}

	mgr := &Manager{
		root:          rootDir,
		stateRepo:     NewStateRepo(db),
		deployKeys:    NewDeployKeyRepo(db, crypto),
		crypto:        crypto,
		gitRunner:     git,
		prepClient:    prep,
		ghUploader:    uploader,
		projectLookup: lookup,
	}

	t.Cleanup(func() {
		_, _ = db.Exec(`DELETE FROM engine.workspaces WHERE project_id = $1`, projectID)
		_, _ = db.Exec(`DELETE FROM engine.project_deploy_keys WHERE project_id = $1`, projectID)
	})

	return &ensureTestFixture{
		manager:  mgr,
		git:      git,
		prep:     prep,
		lookup:   lookup,
		uploader: uploader,
		rootDir:  rootDir,
	}
}

// --- Happy path: first call on new project ---

func TestEnsureReady_NoRow_ClonesAndMarksReady(t *testing.T) {
	f := newEnsureFixture(t, 201)
	ws, err := f.manager.EnsureReady(context.Background(), 1, 201, false)
	if err != nil {
		t.Fatalf("EnsureReady: %v", err)
	}
	if ws.Status != StatusReady {
		t.Errorf("status: want ready, got %s", ws.Status)
	}
	if f.git.cloneCalls != 1 {
		t.Errorf("clone calls: want 1, got %d", f.git.cloneCalls)
	}
	if f.prep.calls != 1 {
		t.Errorf("prep calls: want 1, got %d", f.prep.calls)
	}
	if f.git.lastCloneBranch != "main" {
		t.Errorf("clone branch: want main, got %s", f.git.lastCloneBranch)
	}
	// Deploy key should have been generated and uploaded
	if f.uploader.calls != 1 {
		t.Errorf("github upload calls: want 1, got %d", f.uploader.calls)
	}
}

// --- Happy path: second call on existing ready row ---

func TestEnsureReady_ReadyRow_NoForceSync_NoOp(t *testing.T) {
	f := newEnsureFixture(t, 202)
	ctx := context.Background()

	// First call sets up the row + clone
	if _, err := f.manager.EnsureReady(ctx, 1, 202, false); err != nil {
		t.Fatalf("first EnsureReady: %v", err)
	}
	baseClones := f.git.cloneCalls
	baseFetches := f.git.fetchCalls

	// Second call without forceSync -- should be a no-op
	ws, err := f.manager.EnsureReady(ctx, 1, 202, false)
	if err != nil {
		t.Fatalf("second EnsureReady: %v", err)
	}
	if ws.Status != StatusReady {
		t.Errorf("status: got %s", ws.Status)
	}
	if f.git.cloneCalls != baseClones {
		t.Errorf("second call should not clone again; got %d additional calls", f.git.cloneCalls-baseClones)
	}
	if f.git.fetchCalls != baseFetches {
		t.Errorf("second call should not fetch; got %d additional fetches", f.git.fetchCalls-baseFetches)
	}
}

// --- forceSync=true triggers fetch+reset on ready row ---

func TestEnsureReady_ReadyRow_ForceSync_FetchesAndResets(t *testing.T) {
	f := newEnsureFixture(t, 203)
	ctx := context.Background()

	if _, err := f.manager.EnsureReady(ctx, 1, 203, false); err != nil {
		t.Fatalf("first EnsureReady: %v", err)
	}

	_, err := f.manager.EnsureReady(ctx, 1, 203, true)
	if err != nil {
		t.Fatalf("second EnsureReady (forceSync): %v", err)
	}
	if f.git.fetchCalls != 1 {
		t.Errorf("fetch calls: want 1, got %d", f.git.fetchCalls)
	}
}

// --- Clone failure transitions to error ---

func TestEnsureReady_CloneFails_MarksError(t *testing.T) {
	f := newEnsureFixture(t, 204)
	f.git.cloneShouldFail = true

	_, err := f.manager.EnsureReady(context.Background(), 1, 204, false)
	if err == nil {
		t.Fatal("expected error from failing clone")
	}
	// Row should be in error state
	row, _ := f.manager.stateRepo.GetByProject(context.Background(), 1, 204)
	if row == nil || row.Status != StatusError {
		t.Errorf("row status: want error, got %+v", row)
	}
	if row.LastError == nil || *row.LastError == "" {
		t.Error("last_error should be populated on clone failure")
	}
}

// --- Error row recovers on next call ---

func TestEnsureReady_ErrorRow_NextCallRetriesFromScratch(t *testing.T) {
	f := newEnsureFixture(t, 205)
	ctx := context.Background()

	// First call fails
	f.git.cloneShouldFail = true
	_, err := f.manager.EnsureReady(ctx, 1, 205, false)
	if err == nil {
		t.Fatal("expected error on first call")
	}

	// Clear the failure switch and retry
	f.git.cloneShouldFail = false
	ws, err := f.manager.EnsureReady(ctx, 1, 205, false)
	if err != nil {
		t.Fatalf("retry EnsureReady: %v", err)
	}
	if ws.Status != StatusReady {
		t.Errorf("status after retry: want ready, got %s", ws.Status)
	}
	if f.git.cloneCalls < 2 {
		t.Errorf("retry should have re-cloned; total clone calls: %d", f.git.cloneCalls)
	}
}

// --- Prep soft-failure does not block ready ---

func TestEnsureReady_PrepErrorDoesNotBlockReady(t *testing.T) {
	f := newEnsureFixture(t, 206)
	f.prep.shouldError = true

	ws, err := f.manager.EnsureReady(context.Background(), 1, 206, false)
	if err != nil {
		t.Fatalf("EnsureReady should not fail on prep error: %v", err)
	}
	if ws.Status != StatusReady {
		t.Errorf("status: want ready, got %s", ws.Status)
	}
}

func TestEnsureReady_PrepTransportFailureDoesNotBlockReady(t *testing.T) {
	f := newEnsureFixture(t, 207)
	f.prep.shouldFail = true

	ws, err := f.manager.EnsureReady(context.Background(), 1, 207, false)
	if err != nil {
		t.Fatalf("EnsureReady should not fail on prep transport error: %v", err)
	}
	if ws.Status != StatusReady {
		t.Errorf("status: want ready, got %s", ws.Status)
	}
}

// --- Concurrent callers serialize via advisory lock ---

func TestEnsureReady_ConcurrentCallers_SingleClone(t *testing.T) {
	f := newEnsureFixture(t, 210)

	var wg sync.WaitGroup
	errs := make([]error, 3)
	for i := 0; i < 3; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			_, err := f.manager.EnsureReady(context.Background(), 1, 210, false)
			errs[idx] = err
		}(i)
	}
	wg.Wait()

	for i, err := range errs {
		if err != nil {
			t.Errorf("goroutine %d: %v", i, err)
		}
	}
	if f.git.cloneCalls != 1 {
		t.Errorf("concurrent EnsureReady should clone once; got %d", f.git.cloneCalls)
	}
}

// --- Project lookup missing: fatal ---

func TestEnsureReady_ProjectNotFound_Fatal(t *testing.T) {
	f := newEnsureFixture(t, 212)
	f.lookup.lookupErr = ErrProjectNotFound

	_, err := f.manager.EnsureReady(context.Background(), 1, 212, false)
	if err == nil {
		t.Fatal("expected error when project lookup fails")
	}
}

// --- Deploy key tests ---

func TestEnsureReady_FirstCall_GeneratesAndUploadsKey(t *testing.T) {
	f := newEnsureFixture(t, 301)
	ctx := context.Background()

	ws, err := f.manager.EnsureReady(ctx, 1, 301, false)
	if err != nil {
		t.Fatalf("EnsureReady: %v", err)
	}
	if ws.Status != StatusReady {
		t.Errorf("status: want ready, got %s", ws.Status)
	}
	// Deploy key was generated and uploaded
	if f.uploader.calls != 1 {
		t.Errorf("github upload calls: want 1, got %d", f.uploader.calls)
	}
	// DB row is present
	dk, err := f.manager.deployKeys.GetByProject(ctx, 1, 301)
	if err != nil {
		t.Fatalf("GetByProject: %v", err)
	}
	if dk == nil {
		t.Fatal("deploy key row should exist after EnsureReady")
	}
	if dk.GitHubKeyID == nil || *dk.GitHubKeyID != 88888 {
		t.Errorf("GitHubKeyID: got %v, want 88888", dk.GitHubKeyID)
	}
}

func TestEnsureReady_ExistingKey_ReusedNotRegenerated(t *testing.T) {
	f := newEnsureFixture(t, 302)
	ctx := context.Background()

	// First call generates + uploads
	if _, err := f.manager.EnsureReady(ctx, 1, 302, false); err != nil {
		t.Fatalf("first EnsureReady: %v", err)
	}
	if f.uploader.calls != 1 {
		t.Fatalf("first call should upload once, got %d", f.uploader.calls)
	}

	// Force resync -- should reuse the stored key, NOT regenerate
	if _, err := f.manager.EnsureReady(ctx, 1, 302, true); err != nil {
		t.Fatalf("second EnsureReady: %v", err)
	}
	if f.uploader.calls != 1 {
		t.Errorf("second call should not re-upload: calls=%d", f.uploader.calls)
	}
}

func TestEnsureReady_DeployKeyUploadFails_SetsErrorStatus(t *testing.T) {
	f := newEnsureFixture(t, 303)
	f.uploader.shouldFail = true

	_, err := f.manager.EnsureReady(context.Background(), 1, 303, false)
	if err == nil {
		t.Fatal("expected error from failing GitHub upload")
	}
	row, _ := f.manager.stateRepo.GetByProject(context.Background(), 1, 303)
	if row == nil || row.Status != StatusError {
		t.Errorf("row status: want error, got %+v", row)
	}
	if row.LastError == nil || !strings.Contains(*row.LastError, "deploy key") {
		t.Errorf("last_error should mention deploy key, got: %v", row.LastError)
	}
	// Should not have tried to clone -- upload happens first
	if f.git.cloneCalls != 0 {
		t.Errorf("should not clone when upload fails: calls=%d", f.git.cloneCalls)
	}
}
