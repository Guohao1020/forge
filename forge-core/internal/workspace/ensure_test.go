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
	resetCalls      int
	cloneShouldFail bool
	fetchShouldFail bool
	resetShouldFail bool
	clonedDirs      []string
	lastCloneBranch string
	lastCloneToken  string
}

func (f *fakeGitRunner) Clone(ctx context.Context, hostPath, httpsURL, token, branch string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.cloneCalls++
	f.lastCloneBranch = branch
	f.lastCloneToken = token
	f.clonedDirs = append(f.clonedDirs, hostPath)
	if f.cloneShouldFail {
		return &AuthError{stderr: "fake auth failure"}
	}
	// Simulate a successful clone by creating the dir with a .git marker
	_ = os.MkdirAll(filepath.Join(hostPath, ".git"), 0755)
	return nil
}

func (f *fakeGitRunner) Fetch(ctx context.Context, hostPath, httpsURL, token string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.fetchCalls++
	if f.fetchShouldFail {
		return errors.New("fake fetch failed")
	}
	return nil
}

func (f *fakeGitRunner) ResetHard(ctx context.Context, hostPath, branch string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.resetCalls++
	if f.resetShouldFail {
		return errors.New("fake reset failed")
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

// --- Setup helper ---

type ensureTestFixture struct {
	manager *Manager
	git     *fakeGitRunner
	prep    *fakePrepClient
	lookup  *memoryLookup
	rootDir string
}

func newEnsureFixture(t *testing.T, projectID int64) *ensureTestFixture {
	t.Helper()
	db := openTestDB(t)
	rootDir := t.TempDir()

	git := &fakeGitRunner{}
	prep := &fakePrepClient{}
	lookup := &memoryLookup{
		projects: map[int64]ProjectInfo{
			projectID: {
				RepoURL:     "https://github.com/owner/repo.git",
				AccessToken: "ghp_fake_token",
				Branch:      "main",
			},
		},
	}

	mgr := &Manager{
		root:          rootDir,
		stateRepo:     NewStateRepo(db),
		gitRunner:     git,
		prepClient:    prep,
		projectLookup: lookup,
	}

	t.Cleanup(func() {
		_, _ = db.Exec(`DELETE FROM engine.workspaces WHERE project_id = $1`, projectID)
	})

	return &ensureTestFixture{
		manager: mgr,
		git:     git,
		prep:    prep,
		lookup:  lookup,
		rootDir: rootDir,
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
	if f.git.lastCloneToken != "ghp_fake_token" {
		t.Errorf("clone token: want ghp_fake_token, got %s", f.git.lastCloneToken)
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
	baseResets := f.git.resetCalls

	// Second call without forceSync — should be a no-op
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
	if f.git.resetCalls != baseResets {
		t.Errorf("second call should not reset; got %d additional resets", f.git.resetCalls-baseResets)
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
	if f.git.resetCalls != 1 {
		t.Errorf("reset calls: want 1, got %d", f.git.resetCalls)
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

// --- Phase 1a-specific: GitHub PAT revoked surfaces as auth_failed ---

func TestEnsureReady_GitHubPATRevoked_MarksErrorWithAuthFailed(t *testing.T) {
	// Simulates the spec §3.12 Phase 1a-only row: "GitHub PAT revoked"
	// should produce last_error containing "github_auth_failed" so
	// operators can tell HTTPS+token auth from other clone failures.
	f := newEnsureFixture(t, 211)
	f.git.cloneShouldFail = true // fakeGitRunner returns an AuthError

	_, err := f.manager.EnsureReady(context.Background(), 1, 211, false)
	if err == nil {
		t.Fatal("expected error from revoked PAT")
	}
	row, _ := f.manager.stateRepo.GetByProject(context.Background(), 1, 211)
	if row == nil || row.Status != StatusError {
		t.Fatalf("row status: want error, got %+v", row)
	}
	if row.LastError == nil || !strings.Contains(*row.LastError, "github_auth_failed") {
		t.Errorf("last_error should contain 'github_auth_failed' (Phase 1a PAT revoked path); got %v", row.LastError)
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
