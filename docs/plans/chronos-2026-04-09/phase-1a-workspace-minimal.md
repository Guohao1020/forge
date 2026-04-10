# chronos · Phase 1a — Workspace Minimal (Go + HTTPS+Token)

> **Project:** [chronos — Agent Variant B Single-Agent Implementation](index.md)
> **Phase:** 1a of 9 (Round 2) · **Tasks:** 8 · **Depends on:** [Phase 0](phase-0-infrastructure.md) · **Unblocks:** Phase 5
> **Spec reference:** [Design spec §2.9.4, §3 (Phase 1a scope)](../../specs/2026-04-09-agent-variant-b-single-agent-design.md)

**Execution:** REQUIRED SUB-SKILL: Use `superpowers:subagent-driven-development` (recommended) or `superpowers:executing-plans`. Steps use checkbox (`- [ ]`) syntax for tracking.

---

## Phase goal

Extend `forge-core/internal/workspace/` from its current state (a `Manager` with `EnsureClone` + `injectToken`) into a workspace lifecycle module with:

1. A persisted state DAO over `engine.workspaces`
2. A dependency pre-install RPC client for ai-worker's `/api/workspace/prep`
3. A `ProjectLookup` interface (Phase 1a shape: returns HTTPS URL + GitHub PAT)
4. An `EnsureReady` state machine that drives clone / resync / error-recovery under a PG advisory lock
5. A simplified `RealGitRunner` that uses HTTPS+token auth (retains the existing `injectToken` helper)
6. Caller migrations in `build_activities.go`, `devops_activities.go`, and `agent/service.go`
7. Full `main.go` wiring of the above components

**Explicit scope fence — what is NOT in Phase 1a:**

- **No deploy keys.** No ed25519 generation, no AES-GCM encryption, no GitHub deploy-key upload API, no `DeployKey*` symbols, no `keys.go`, no `project_deploy_keys` table, no key rotation interfaces. All of these move to [Phase 1b](phase-1b-deploy-keys.md).
- **No SSH.** `RealGitRunner` in Phase 1a uses HTTPS URLs with the existing `injectToken` helper — no `GIT_SSH_COMMAND`, no `toSSHURL`, no tempfile-managed private keys.
- **`injectToken` STILL EXISTS after this phase.** It's moved out of `manager.go` into `git.go` where `RealGitRunner` uses it. `manager.go` no longer imports it. Phase 1b deletes it wholesale.
- **`ProjectLookup` returns HTTPS+token shape.** Phase 1b rewrites this as a breaking change per spec §2.9.4.b — the `AccessToken` field will be dropped and `RepoURL` renamed to `SSHURL`.

**Downstream impact:** Phase 5 agent service can call `workspace.Manager.EnsureReady(ctx, tenantID, projectID)` instead of the current `os.Stat(.git)` probe. The two Temporal worker activity files (`build_activities.go`, `devops_activities.go`) get migrated to the new API in this phase.

**Phase 1b gate for public deployment:** Per spec §2.9.4.d, Phase 1b is **not optional** for public deployment. The §3.8 security rationale ("deploy keys live in forge-core for prompt-injection containment") still holds. If Phase 1b is deferred indefinitely, the MVP can ship for solo-dev / internal testing but public deployment is blocked until Phase 1b lands. This phase file is the internal-only milestone; public go-live waits on Phase 1b.

**Completion gate:**

- `go test ./internal/workspace/...` passes including new state DAO, prep client, lookup, and `EnsureReady` integration tests
- `go build ./cmd/forge-core` succeeds
- The two worker activity callers (`build_activities.go:96`, `devops_activities.go:134`) call `EnsureReady(ctx, tenantID, projectID)` with no `repoURL`/`token`/`branch` params
- `agent/service.go` calls `EnsureReady` instead of `os.Stat(.git)`
- `EnsureClone` is deleted from `manager.go`
- `injectToken` STILL EXISTS — but only inside `workspace/git.go` (used by `RealGitRunner`). `manager.go` no longer references it. Phase 1b deletes it.
- No references to `keys.go`, `DeployKey*` symbols, `project_deploy_keys`, `ed25519`, `AES-GCM`, or `GIT_SSH_COMMAND` anywhere in Phase 1a
- A mock-free integration test successfully drives the full `EnsureReady` state machine through first-clone, resync, and recover-from-error paths using a local `git init --bare` fixture

**Key architecture points (from spec §3 as scoped by §2.9.4.a):**

1. `engine.workspaces` row is the single source of truth for workspace state — in-memory state machines in Go would lose state across process restarts.
2. PG advisory lock `pg_advisory_xact_lock(hashtext('workspace:' || tenant || ':' || project))` serializes concurrent `EnsureReady` calls. Inside a transaction, held until commit/rollback.
3. **In Phase 1a, HTTPS+token auth is retained via `injectToken`.** All git invocations flow through `RealGitRunner` methods that build token-injected URLs of the form `https://x-access-token:{token}@github.com/owner/repo.git`. Migration to SSH deploy keys is entirely **Phase 1b**.
4. Dependency pre-install runs inside ai-worker (which has network + language toolchains), triggered by forge-core via a thin `POST /api/workspace/prep` RPC.
5. New-session "reset hard" is driven by the Go agent service (which knows what a "new session" is), not by the workspace service. `EnsureReady` takes a `forceSync bool` parameter.
6. **Public deployment is blocked until Phase 1b lands** per spec §2.9.4.d.

---

### Task 1a.1: Create the `Workspace` state DAO

**Files:**
- Create: `forge-core/internal/workspace/state.go`
- Create: `forge-core/internal/workspace/state_test.go`

**Context:** Thin data-access layer over `engine.workspaces`. Three operations: get by (tenant, project), insert as pending, update status. Plus the advisory lock helper that wraps a transaction. Separated from `manager.go` so the DAO is independently testable against a real postgres (via dockertest or the dev DB) without dragging in git/prep machinery.

The `Workspace` struct is the domain model, not an ORM row — it has no tags. The repository is a thin wrapper over `*sql.DB` (forge-core uses `database/sql` + raw SQL, not gorm, following the existing pattern in `engine/` modules).

**This task carries over verbatim from Round 1 Task 1.1.** The state DAO is auth-independent; it's identical in Phase 1a and Phase 1b. The only thing Phase 1b would add is the parallel `DeployKeyRepo` in a separate file (`keys.go`) — which is out of scope for this task and out of scope for Phase 1a entirely.

- [ ] **Step 1: Write the failing DAO tests**

Create `forge-core/internal/workspace/state_test.go`:

```go
package workspace

import (
	"context"
	"database/sql"
	"os"
	"testing"
	"time"

	_ "github.com/lib/pq"
)

// Integration tests run against a real PG via docker-compose. Skipped
// when DATABASE_URL is not set so `go test ./...` on a bare checkout
// still passes.
func openTestDB(t *testing.T) *sql.DB {
	t.Helper()
	dsn := os.Getenv("FORGE_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("FORGE_TEST_DATABASE_URL not set; skipping integration test")
	}
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.PingContext(context.Background()); err != nil {
		t.Fatalf("ping db: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func cleanupWorkspace(t *testing.T, db *sql.DB, tenantID, projectID int64) {
	t.Helper()
	_, err := db.Exec(
		`DELETE FROM engine.workspaces WHERE tenant_id = $1 AND project_id = $2`,
		tenantID, projectID,
	)
	if err != nil {
		t.Fatalf("cleanup: %v", err)
	}
}

func TestStateRepo_GetByProject_NotFound(t *testing.T) {
	db := openTestDB(t)
	repo := NewStateRepo(db)
	ctx := context.Background()

	ws, err := repo.GetByProject(ctx, 9999, 9999)
	if err != nil {
		t.Fatalf("GetByProject: %v", err)
	}
	if ws != nil {
		t.Fatalf("expected nil for missing row, got %+v", ws)
	}
}

func TestStateRepo_InsertPendingAndGet(t *testing.T) {
	db := openTestDB(t)
	repo := NewStateRepo(db)
	ctx := context.Background()
	defer cleanupWorkspace(t, db, 1, 42)

	err := repo.InsertPending(ctx, 1, 42, "/data/forge/workspaces/tenant-1/project-42/repo", "/data/forge/workspaces/tenant-1/project-42/repo")
	if err != nil {
		t.Fatalf("InsertPending: %v", err)
	}

	ws, err := repo.GetByProject(ctx, 1, 42)
	if err != nil {
		t.Fatalf("GetByProject: %v", err)
	}
	if ws == nil {
		t.Fatal("expected workspace row")
	}
	if ws.Status != StatusPending {
		t.Errorf("status: want pending, got %s", ws.Status)
	}
	if ws.TenantID != 1 || ws.ProjectID != 42 {
		t.Errorf("tenant/project mismatch: got %d/%d", ws.TenantID, ws.ProjectID)
	}
}

func TestStateRepo_InsertPendingIdempotent(t *testing.T) {
	// Two inserts for the same (tenant, project) must not error — the
	// second one is a no-op. This is what ON CONFLICT DO NOTHING gives us,
	// and it's what the state machine relies on during concurrent-caller
	// races.
	db := openTestDB(t)
	repo := NewStateRepo(db)
	ctx := context.Background()
	defer cleanupWorkspace(t, db, 1, 43)

	path := "/data/forge/workspaces/tenant-1/project-43/repo"
	err := repo.InsertPending(ctx, 1, 43, path, path)
	if err != nil {
		t.Fatalf("first InsertPending: %v", err)
	}
	err = repo.InsertPending(ctx, 1, 43, path, path)
	if err != nil {
		t.Fatalf("second InsertPending (should be no-op): %v", err)
	}
}

func TestStateRepo_MarkReady(t *testing.T) {
	db := openTestDB(t)
	repo := NewStateRepo(db)
	ctx := context.Background()
	defer cleanupWorkspace(t, db, 1, 44)

	path := "/data/forge/workspaces/tenant-1/project-44/repo"
	if err := repo.InsertPending(ctx, 1, 44, path, path); err != nil {
		t.Fatalf("InsertPending: %v", err)
	}
	if err := repo.MarkReady(ctx, 1, 44); err != nil {
		t.Fatalf("MarkReady: %v", err)
	}

	ws, _ := repo.GetByProject(ctx, 1, 44)
	if ws.Status != StatusReady {
		t.Errorf("status: want ready, got %s", ws.Status)
	}
	if ws.LastSyncedAt == nil {
		t.Error("LastSyncedAt should be set after MarkReady")
	}
	// Ready row should have last_error cleared
	if ws.LastError != nil && *ws.LastError != "" {
		t.Errorf("LastError should be empty after MarkReady, got %q", *ws.LastError)
	}
}

func TestStateRepo_MarkError(t *testing.T) {
	db := openTestDB(t)
	repo := NewStateRepo(db)
	ctx := context.Background()
	defer cleanupWorkspace(t, db, 1, 45)

	path := "/data/forge/workspaces/tenant-1/project-45/repo"
	if err := repo.InsertPending(ctx, 1, 45, path, path); err != nil {
		t.Fatalf("InsertPending: %v", err)
	}
	if err := repo.MarkError(ctx, 1, 45, "clone failed: network"); err != nil {
		t.Fatalf("MarkError: %v", err)
	}

	ws, _ := repo.GetByProject(ctx, 1, 45)
	if ws.Status != StatusError {
		t.Errorf("status: want error, got %s", ws.Status)
	}
	if ws.LastError == nil || *ws.LastError != "clone failed: network" {
		t.Errorf("LastError mismatch: got %v", ws.LastError)
	}
}

func TestStateRepo_WithAdvisoryLock_Serializes(t *testing.T) {
	// Two goroutines both try to grab the advisory lock. The first one
	// holds it for 100ms; the second must wait and complete after.
	db := openTestDB(t)
	repo := NewStateRepo(db)
	ctx := context.Background()

	var firstDone, secondDone time.Time
	errCh := make(chan error, 2)

	go func() {
		errCh <- repo.WithAdvisoryLock(ctx, 1, 46, func(tx *sql.Tx) error {
			time.Sleep(100 * time.Millisecond)
			firstDone = time.Now()
			return nil
		})
	}()
	// Small delay to ensure the first goroutine grabs the lock first.
	time.Sleep(10 * time.Millisecond)

	go func() {
		errCh <- repo.WithAdvisoryLock(ctx, 1, 46, func(tx *sql.Tx) error {
			secondDone = time.Now()
			return nil
		})
	}()

	for i := 0; i < 2; i++ {
		if err := <-errCh; err != nil {
			t.Fatalf("WithAdvisoryLock: %v", err)
		}
	}

	if !secondDone.After(firstDone) {
		t.Fatalf("second caller finished before first: first=%v second=%v", firstDone, secondDone)
	}
	gap := secondDone.Sub(firstDone)
	if gap < 50*time.Millisecond {
		t.Errorf("second caller did not wait for first (gap=%v)", gap)
	}
}
```

- [ ] **Step 2: Run the tests to confirm compilation failure**

Run: `cd forge-core && go test ./internal/workspace/... -run TestStateRepo`
Expected: `undefined: NewStateRepo`, `undefined: StatusPending`, etc. This is the TDD failure mode — we haven't written `state.go` yet.

- [ ] **Step 3: Implement `state.go`**

Create `forge-core/internal/workspace/state.go`:

```go
package workspace

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"hash/fnv"
	"time"
)

// WorkspaceStatus is the persisted state of a workspace row.
type WorkspaceStatus string

const (
	StatusPending WorkspaceStatus = "pending"
	StatusReady   WorkspaceStatus = "ready"
	StatusError   WorkspaceStatus = "error"
)

// Workspace is the domain model for an engine.workspaces row.
// Fields that can be NULL in the DB are pointers.
type Workspace struct {
	ID            int64
	TenantID      int64
	ProjectID     int64
	HostPath      string
	ContainerPath string
	Status        WorkspaceStatus
	LastSyncedAt  *time.Time
	LastError     *string
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

// StateRepo is the DAO for engine.workspaces rows.
type StateRepo struct {
	db *sql.DB
}

// NewStateRepo constructs a StateRepo over a shared *sql.DB. The repo
// itself holds no per-request state so it's safe to share across
// goroutines.
func NewStateRepo(db *sql.DB) *StateRepo {
	return &StateRepo{db: db}
}

// GetByProject returns the workspace row for (tenantID, projectID) or
// (nil, nil) if no row exists. A nonexistent row is not an error —
// callers distinguish "not found" from "query failed" via the nil check.
func (r *StateRepo) GetByProject(ctx context.Context, tenantID, projectID int64) (*Workspace, error) {
	const q = `
		SELECT id, tenant_id, project_id, host_path, container_path, status,
		       last_synced_at, last_error, created_at, updated_at
		FROM engine.workspaces
		WHERE tenant_id = $1 AND project_id = $2
	`
	ws := &Workspace{}
	err := r.db.QueryRowContext(ctx, q, tenantID, projectID).Scan(
		&ws.ID, &ws.TenantID, &ws.ProjectID, &ws.HostPath, &ws.ContainerPath,
		&ws.Status, &ws.LastSyncedAt, &ws.LastError, &ws.CreatedAt, &ws.UpdatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("workspace: GetByProject: %w", err)
	}
	return ws, nil
}

// InsertPending inserts a new workspace row with status='pending'.
// Uses ON CONFLICT DO NOTHING so a racing caller who finds the row
// already present is a no-op, not an error. The subsequent
// GetByProject call observes the row regardless of who inserted it.
func (r *StateRepo) InsertPending(
	ctx context.Context,
	tenantID, projectID int64,
	hostPath, containerPath string,
) error {
	const q = `
		INSERT INTO engine.workspaces
			(tenant_id, project_id, host_path, container_path, status)
		VALUES ($1, $2, $3, $4, 'pending')
		ON CONFLICT (tenant_id, project_id) DO NOTHING
	`
	_, err := r.db.ExecContext(ctx, q, tenantID, projectID, hostPath, containerPath)
	if err != nil {
		return fmt.Errorf("workspace: InsertPending: %w", err)
	}
	return nil
}

// MarkReady transitions the row to status='ready', sets last_synced_at=now(),
// and clears last_error.
func (r *StateRepo) MarkReady(ctx context.Context, tenantID, projectID int64) error {
	const q = `
		UPDATE engine.workspaces
		SET status = 'ready',
		    last_synced_at = now(),
		    last_error = NULL,
		    updated_at = now()
		WHERE tenant_id = $1 AND project_id = $2
	`
	res, err := r.db.ExecContext(ctx, q, tenantID, projectID)
	if err != nil {
		return fmt.Errorf("workspace: MarkReady: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return fmt.Errorf("workspace: MarkReady: no row for (%d, %d)", tenantID, projectID)
	}
	return nil
}

// MarkError transitions the row to status='error' and records the reason.
func (r *StateRepo) MarkError(ctx context.Context, tenantID, projectID int64, reason string) error {
	const q = `
		UPDATE engine.workspaces
		SET status = 'error',
		    last_error = $3,
		    updated_at = now()
		WHERE tenant_id = $1 AND project_id = $2
	`
	res, err := r.db.ExecContext(ctx, q, tenantID, projectID, reason)
	if err != nil {
		return fmt.Errorf("workspace: MarkError: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return fmt.Errorf("workspace: MarkError: no row for (%d, %d)", tenantID, projectID)
	}
	return nil
}

// ResetToPending wipes last_error and moves status back to 'pending'.
// Used by EnsureReady when recovering from an 'error' row: we want to
// retry from scratch, so the row goes back through the same state
// transitions as a fresh install.
func (r *StateRepo) ResetToPending(ctx context.Context, tenantID, projectID int64) error {
	const q = `
		UPDATE engine.workspaces
		SET status = 'pending',
		    last_error = NULL,
		    updated_at = now()
		WHERE tenant_id = $1 AND project_id = $2
	`
	_, err := r.db.ExecContext(ctx, q, tenantID, projectID)
	if err != nil {
		return fmt.Errorf("workspace: ResetToPending: %w", err)
	}
	return nil
}

// WithAdvisoryLock runs fn inside a transaction that holds a PG
// xact-scoped advisory lock on (tenantID, projectID). Concurrent
// callers block on Acquire, then observe each other's state changes
// serially. The lock is released automatically on transaction commit
// or rollback.
//
// Lock key is a 64-bit hash of "workspace:{tenant}:{project}" so
// different (tenant, project) pairs don't contend.
func (r *StateRepo) WithAdvisoryLock(
	ctx context.Context,
	tenantID, projectID int64,
	fn func(tx *sql.Tx) error,
) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("workspace: begin tx: %w", err)
	}
	// Rollback unless we reach the commit below.
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()

	key := advisoryLockKey(tenantID, projectID)
	if _, err := tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock($1)`, key); err != nil {
		return fmt.Errorf("workspace: acquire advisory lock: %w", err)
	}

	if err := fn(tx); err != nil {
		return err
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("workspace: commit: %w", err)
	}
	committed = true
	return nil
}

// advisoryLockKey derives a stable int64 from (tenantID, projectID)
// that fits pg_advisory_xact_lock(bigint). Uses FNV-1a over the
// canonical string form so it matches spec §3.7's
// hashtext('workspace:{tenant}:{project}') semantically (though we
// compute it in Go rather than relying on PG's hashtext, which has
// its own hash algorithm — doesn't matter, both are deterministic
// and collision-resistant enough for lock keys).
func advisoryLockKey(tenantID, projectID int64) int64 {
	h := fnv.New64a()
	_, _ = fmt.Fprintf(h, "workspace:%d:%d", tenantID, projectID)
	// pg advisory lock wants int64; fnv gives uint64 — convert safely.
	return int64(h.Sum64())
}
```

- [ ] **Step 4: Run the DAO tests**

Prerequisite: dev postgres must be running and `FORGE_TEST_DATABASE_URL` env var must point to it. Example:

```bash
export FORGE_TEST_DATABASE_URL="postgres://forge:forge@localhost:5432/forge?sslmode=disable"
cd forge-core && go test ./internal/workspace/... -run TestStateRepo -v
```

Expected: 6 tests pass — `TestStateRepo_GetByProject_NotFound`, `_InsertPendingAndGet`, `_InsertPendingIdempotent`, `_MarkReady`, `_MarkError`, `_WithAdvisoryLock_Serializes`.

If `FORGE_TEST_DATABASE_URL` is unset the tests skip gracefully (see `openTestDB`); set it via `docker compose exec postgres env | grep PG`, or use the dev DB directly if `docker compose up postgres` is running.

- [ ] **Step 5: Commit**

```bash
git add forge-core/internal/workspace/state.go forge-core/internal/workspace/state_test.go
git commit -m "$(cat <<'EOF'
feat(workspace): state DAO for engine.workspaces

StateRepo provides GetByProject / InsertPending / MarkReady / MarkError
/ ResetToPending over the engine.workspaces table, plus WithAdvisoryLock
which wraps fn in a tx holding pg_advisory_xact_lock for the
(tenant, project) pair.

InsertPending is idempotent via ON CONFLICT DO NOTHING so concurrent
EnsureReady callers race safely. Advisory lock is keyed on FNV-1a hash
of 'workspace:{tenant}:{project}'.

Integration tests require FORGE_TEST_DATABASE_URL pointing at a real
PG; skipped otherwise. Adds the WithAdvisoryLock serialization test
that spins two goroutines and verifies the second one waits.

Part of chronos Phase 1a — the DAO is auth-independent; Phase 1b
adds the parallel DeployKeyRepo in a separate file.
EOF
)"
```

---

### Task 1a.2: Dependency pre-install RPC client

**Files:**
- Create: `forge-core/internal/workspace/prep.go`
- Create: `forge-core/internal/workspace/prep_test.go`

**Context:** Thin HTTP client that POSTs to ai-worker's `/api/workspace/prep` endpoint (which will be implemented in Phase 5). The client lives in forge-core because forge-core is the one that *calls* the prep endpoint — ai-worker runs the actual install commands (since only ai-worker has the language toolchains).

Request body:
```json
{"tenant_id": 1, "project_id": 42, "workspace_path": "tenant-1/project-42/repo"}
```
Response:
```json
{"status": "ok" | "skipped" | "error", "language": "go", "command": "go mod download", "error": "..."}
```

Prep failures **do not block** the state machine — spec §3.9 says deps prep is "non-blocking soft failure". The client returns an error so the caller can log it, but `EnsureReady` will still mark the row ready.

**This task carries over verbatim from Round 1 Task 1.5.** The prep client is auth-independent; it doesn't care whether the workspace was cloned via HTTPS+token or SSH deploy key.

- [ ] **Step 1: Write the failing tests**

Create `forge-core/internal/workspace/prep_test.go`:

```go
package workspace

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestPrepClient_Success(t *testing.T) {
	var receivedBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/workspace/prep" {
			t.Errorf("path: got %s", r.URL.Path)
		}
		_ = json.NewDecoder(r.Body).Decode(&receivedBody)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status": "ok", "language": "go", "command": "go mod download"}`))
	}))
	defer srv.Close()

	c := NewPrepClient(srv.URL)
	result, err := c.Prep(context.Background(), 1, 42, "tenant-1/project-42/repo")
	if err != nil {
		t.Fatalf("Prep: %v", err)
	}
	if result.Status != "ok" {
		t.Errorf("status: got %s", result.Status)
	}
	if result.Language != "go" {
		t.Errorf("language: got %s", result.Language)
	}
	if receivedBody["tenant_id"].(float64) != 1 {
		t.Errorf("tenant_id: got %v", receivedBody["tenant_id"])
	}
	if receivedBody["workspace_path"] != "tenant-1/project-42/repo" {
		t.Errorf("workspace_path: got %v", receivedBody["workspace_path"])
	}
}

func TestPrepClient_Skipped(t *testing.T) {
	// Language detection returned None — ai-worker responds with skipped.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"status": "skipped", "reason": "no language detected"}`))
	}))
	defer srv.Close()

	c := NewPrepClient(srv.URL)
	result, err := c.Prep(context.Background(), 1, 42, "tenant-1/project-42/repo")
	if err != nil {
		t.Fatalf("Prep: %v", err)
	}
	if result.Status != "skipped" {
		t.Errorf("status: got %s", result.Status)
	}
}

func TestPrepClient_5xxReturnsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("ai-worker: prep failed"))
	}))
	defer srv.Close()

	c := NewPrepClient(srv.URL)
	_, err := c.Prep(context.Background(), 1, 42, "tenant-1/project-42/repo")
	if err == nil {
		t.Fatal("expected error for 500 response")
	}
	if !strings.Contains(err.Error(), "500") {
		t.Errorf("error should include status: %v", err)
	}
}
```

- [ ] **Step 2: Run tests — expect compile failure**

Run: `cd forge-core && go test ./internal/workspace/... -run TestPrepClient`

- [ ] **Step 3: Implement `prep.go`**

Create `forge-core/internal/workspace/prep.go`:

```go
package workspace

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// PrepResult is the decoded response from ai-worker's /api/workspace/prep.
type PrepResult struct {
	Status   string `json:"status"` // "ok" | "skipped" | "error"
	Language string `json:"language,omitempty"`
	Command  string `json:"command,omitempty"`
	Error    string `json:"error,omitempty"`
	Reason   string `json:"reason,omitempty"`
}

// PrepClient posts to ai-worker's /api/workspace/prep to trigger
// language-specific dependency pre-install inside a freshly cloned
// workspace. Prep runs in the ai-worker container (which has language
// toolchains + network access, unlike the bwrap bash sandbox), so
// this is an HTTP call from forge-core to ai-worker.
type PrepClient struct {
	baseURL string
	client  *http.Client
}

// NewPrepClient constructs a PrepClient pointing at the ai-worker base URL.
// Typical value in dev: "http://host.docker.internal:8090" if forge-core
// runs on host; "http://forge-ai-worker:8090" if both are in compose.
func NewPrepClient(baseURL string) *PrepClient {
	return &PrepClient{
		baseURL: baseURL,
		// Prep can take a minute or two for Maven, longer for big npm trees.
		client: &http.Client{Timeout: 10 * time.Minute},
	}
}

// Prep triggers dependency pre-install for the given workspace.
// Returns a PrepResult describing what happened. Returns an error on
// transport failure or non-2xx HTTP. A PrepResult with status="error"
// is NOT returned as an error — the caller logs it and continues,
// matching spec §3.9's non-blocking-soft-failure rule.
func (c *PrepClient) Prep(
	ctx context.Context,
	tenantID, projectID int64,
	workspacePath string,
) (*PrepResult, error) {
	body := map[string]any{
		"tenant_id":      tenantID,
		"project_id":     projectID,
		"workspace_path": workspacePath,
	}
	bodyBytes, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("prep: marshal: %w", err)
	}

	url := c.baseURL + "/api/workspace/prep"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("prep: new request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("prep: http: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("prep: HTTP %d: %s", resp.StatusCode, string(respBody))
	}

	var result PrepResult
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("prep: decode: %w", err)
	}
	return &result, nil
}
```

- [ ] **Step 4: Run the tests**

Run: `cd forge-core && go test ./internal/workspace/... -run TestPrepClient -v`
Expected: 3 tests pass.

- [ ] **Step 5: Commit**

```bash
git add forge-core/internal/workspace/prep.go forge-core/internal/workspace/prep_test.go
git commit -m "$(cat <<'EOF'
feat(workspace): dependency pre-install RPC client

PrepClient posts to ai-worker's /api/workspace/prep to trigger
language-specific dependency install in the ai-worker container
(where go/mvn/npm/pip live). Forge-core drives the RPC because
the decision 'should we prep' is part of the EnsureReady state
machine, but the actual commands run in ai-worker because only
ai-worker has the toolchains.

PrepResult has status 'ok' | 'skipped' | 'error'. Transport
failures return Go errors; status='error' is NOT a Go error — it
still returns a PrepResult so EnsureReady can treat it as a
non-blocking soft failure per spec §3.9 and still mark the
workspace ready.

Timeout is generous (10 min) because Maven offline-prep + npm ci
on large trees takes real time. The endpoint itself is implemented
in Phase 5.

Part of chronos Phase 1a; auth-independent (identical in 1a and 1b).
EOF
)"
```

---

### Task 1a.3: `ProjectLookup` interface + production adapter (HTTPS+token shape)

**Files:**
- Create: `forge-core/internal/workspace/lookup.go`
- Create: `forge-core/internal/workspace/lookup_test.go`
- Create: `forge-core/internal/module/project/lookup_adapter.go` (Phase 1a production adapter)

**Context:** `EnsureReady` needs to know a project's repo URL, the owning user's GitHub PAT (for token injection), and the default branch. The workspace package must not import from `internal/module/project/` directly — that would create a cyclic dependency (project imports workspace via `WorkspaceProvider`). So we define a small `ProjectLookup` interface here and implement it via a thin adapter that lives in the project module.

**Phase 1a interface shape** (per spec §2.9.4.b):

```go
type ProjectLookup interface {
    LookupProject(ctx context.Context, tenantID, projectID int64) (ProjectInfo, error)
}

type ProjectInfo struct {
    RepoURL       string  // HTTPS URL (e.g. https://github.com/owner/repo.git)
    AccessToken   string  // GitHub PAT — used by injectToken
    DefaultBranch string
}
```

**Phase 1b rewrite (for reference only; do NOT implement here):** Phase 1b drops `AccessToken`, renames `RepoURL` to `SSHURL`, and updates every caller in a single hard-cutover commit. The interface is only consumed inside forge-core so a hard cutover is cheaper than versioning. This is called out in the commit message so future readers understand it's a known transitional shape.

**Production adapter:** The real project table already has `repo_url` HTTPS columns and a separate token store (user-level GitHub PAT). The adapter queries both and assembles `ProjectInfo`. The adapter is tested against a fake project-module client that returns canned HTTPS+token data.

- [ ] **Step 1: Write the failing tests for the interface + in-memory fake**

Create `forge-core/internal/workspace/lookup_test.go`:

```go
package workspace

import (
	"context"
	"errors"
	"testing"
)

type memoryLookup struct {
	projects map[int64]ProjectInfo
	lookupErr error
}

func (m *memoryLookup) LookupProject(ctx context.Context, tenantID, projectID int64) (ProjectInfo, error) {
	if m.lookupErr != nil {
		return ProjectInfo{}, m.lookupErr
	}
	p, ok := m.projects[projectID]
	if !ok {
		return ProjectInfo{}, ErrProjectNotFound
	}
	return p, nil
}

func TestMemoryLookupImplementsInterface(t *testing.T) {
	// Compile-time check — assigning to the interface type verifies the
	// memoryLookup satisfies ProjectLookup. This test body is nearly empty
	// but the var assignment is the actual assertion.
	var _ ProjectLookup = &memoryLookup{}
}

func TestProjectInfo_FieldsPresent(t *testing.T) {
	p := ProjectInfo{
		RepoURL:       "https://github.com/owner/repo.git",
		AccessToken:   "ghp_fake_token",
		DefaultBranch: "main",
	}
	if p.RepoURL == "" {
		t.Fatal("RepoURL must be populated")
	}
	if p.AccessToken == "" {
		t.Fatal("AccessToken must be populated (Phase 1a shape)")
	}
	if p.DefaultBranch == "" {
		t.Fatal("DefaultBranch must be populated")
	}
}

func TestErrProjectNotFound(t *testing.T) {
	// Make sure we're exporting a sentinel so EnsureReady can use
	// errors.Is to distinguish "project row missing" from generic errors.
	if ErrProjectNotFound == nil {
		t.Fatal("ErrProjectNotFound should be a non-nil sentinel")
	}
	if !errors.Is(ErrProjectNotFound, ErrProjectNotFound) {
		t.Fatal("ErrProjectNotFound should be comparable with errors.Is")
	}
}

func TestMemoryLookup_Returns_ProjectInfo(t *testing.T) {
	lookup := &memoryLookup{
		projects: map[int64]ProjectInfo{
			42: {
				RepoURL:       "https://github.com/owner/repo.git",
				AccessToken:   "ghp_fake_token",
				DefaultBranch: "main",
			},
		},
	}
	got, err := lookup.LookupProject(context.Background(), 1, 42)
	if err != nil {
		t.Fatalf("LookupProject: %v", err)
	}
	if got.RepoURL != "https://github.com/owner/repo.git" {
		t.Errorf("RepoURL: got %s", got.RepoURL)
	}
	if got.AccessToken != "ghp_fake_token" {
		t.Errorf("AccessToken: got %s", got.AccessToken)
	}
	if got.DefaultBranch != "main" {
		t.Errorf("DefaultBranch: got %s", got.DefaultBranch)
	}
}

func TestMemoryLookup_NotFound(t *testing.T) {
	lookup := &memoryLookup{projects: map[int64]ProjectInfo{}}
	_, err := lookup.LookupProject(context.Background(), 1, 9999)
	if !errors.Is(err, ErrProjectNotFound) {
		t.Errorf("expected ErrProjectNotFound, got %v", err)
	}
}
```

- [ ] **Step 2: Run tests — expect compile failure**

Run: `cd forge-core && go test ./internal/workspace/... -run "TestMemoryLookup|TestProjectInfo|TestErrProjectNotFound"`
Expected: `undefined: ProjectLookup`, `undefined: ProjectInfo`, `undefined: ErrProjectNotFound`.

- [ ] **Step 3: Implement `lookup.go`**

Create `forge-core/internal/workspace/lookup.go`:

```go
package workspace

import (
	"context"
	"errors"
)

// ProjectInfo is everything EnsureReady needs to know about a project
// to clone and prep it.
//
// PHASE 1A SHAPE: this struct carries a GitHub PAT for HTTPS+token auth
// because Phase 1a retains the existing injectToken path. Phase 1b will
// rewrite this as a breaking change per spec §2.9.4.b — the AccessToken
// field will be dropped and RepoURL will be renamed to SSHURL. Both
// changes happen in a single hard-cutover commit that migrates every
// caller. See docs/specs/2026-04-09-agent-variant-b-single-agent-design.md
// §2.9.4.b for the rationale.
type ProjectInfo struct {
	RepoURL       string // HTTPS URL, e.g. https://github.com/owner/repo.git
	AccessToken   string // GitHub PAT used by injectToken in Phase 1a; dropped in Phase 1b
	DefaultBranch string
}

// ErrProjectNotFound is returned by ProjectLookup.LookupProject when no
// project row exists for (tenantID, projectID). EnsureReady treats
// this as a fatal error — there's nothing to clone.
var ErrProjectNotFound = errors.New("workspace: project not found")

// ProjectLookup abstracts the project-row + github-token access that
// EnsureReady needs. Defined here in the workspace package to avoid
// a cyclic dependency with internal/module/project (which imports
// workspace.WorkspaceProvider).
//
// The production implementation is a thin adapter in
// forge-core/internal/module/project/lookup_adapter.go that delegates
// to project.Service (for repo URL and default branch) and to auth or
// a token store (for the GitHub PAT).
type ProjectLookup interface {
	// LookupProject returns project metadata. Returns ErrProjectNotFound
	// if no row exists. The tenantID is passed explicitly so the
	// lookup can enforce multi-tenant isolation at the DB layer.
	//
	// Phase 1a shape: populates RepoURL (HTTPS), AccessToken (GitHub PAT),
	// and DefaultBranch. Phase 1b rewrites this signature.
	LookupProject(ctx context.Context, tenantID, projectID int64) (ProjectInfo, error)
}
```

- [ ] **Step 4: Run interface tests**

Run: `cd forge-core && go test ./internal/workspace/... -run "TestMemoryLookup|TestProjectInfo|TestErrProjectNotFound" -v`
Expected: 5 tests pass.

- [ ] **Step 5: Write the production adapter tests**

Create `forge-core/internal/module/project/lookup_adapter_test.go`:

```go
package project

import (
	"context"
	"errors"
	"testing"

	"github.com/shulex/forge/forge-core/internal/workspace"
)

// fakeProjectReader simulates the project.Service.GetByIDInternal path.
type fakeProjectReader struct {
	projects map[int64]*Project
}

func (f *fakeProjectReader) GetByIDInternal(ctx context.Context, projectID, tenantID int64) (*Project, error) {
	p, ok := f.projects[projectID]
	if !ok {
		return nil, ErrProjectNotFound
	}
	if tenantID != 0 && p.TenantID != tenantID {
		return nil, ErrProjectNotFound
	}
	return p, nil
}

// fakeTokenReader simulates the user-level GitHub PAT store.
type fakeTokenReader struct {
	tokens   map[int64]string
	tokenErr error
}

func (f *fakeTokenReader) GetGitHubTokenForUser(ctx context.Context, userID int64) (string, error) {
	if f.tokenErr != nil {
		return "", f.tokenErr
	}
	t, ok := f.tokens[userID]
	if !ok {
		return "", errors.New("no token")
	}
	return t, nil
}

func TestLookupAdapter_HappyPath(t *testing.T) {
	projectReader := &fakeProjectReader{
		projects: map[int64]*Project{
			42: {
				ID:            42,
				TenantID:      1,
				CodeRepoURL:   "https://github.com/owner/repo.git",
				DefaultBranch: "main",
				CreatedBy:     99,
			},
		},
	}
	tokenReader := &fakeTokenReader{
		tokens: map[int64]string{99: "ghp_fake_token"},
	}
	adapter := NewLookupAdapter(projectReader, tokenReader)

	got, err := adapter.LookupProject(context.Background(), 1, 42)
	if err != nil {
		t.Fatalf("LookupProject: %v", err)
	}
	if got.RepoURL != "https://github.com/owner/repo.git" {
		t.Errorf("RepoURL: got %s", got.RepoURL)
	}
	if got.AccessToken != "ghp_fake_token" {
		t.Errorf("AccessToken: got %s", got.AccessToken)
	}
	if got.DefaultBranch != "main" {
		t.Errorf("DefaultBranch: got %s", got.DefaultBranch)
	}
}

func TestLookupAdapter_ProjectNotFound(t *testing.T) {
	projectReader := &fakeProjectReader{projects: map[int64]*Project{}}
	tokenReader := &fakeTokenReader{}
	adapter := NewLookupAdapter(projectReader, tokenReader)

	_, err := adapter.LookupProject(context.Background(), 1, 9999)
	if !errors.Is(err, workspace.ErrProjectNotFound) {
		t.Errorf("expected ErrProjectNotFound, got %v", err)
	}
}

func TestLookupAdapter_TokenError(t *testing.T) {
	projectReader := &fakeProjectReader{
		projects: map[int64]*Project{
			42: {ID: 42, TenantID: 1, CodeRepoURL: "https://github.com/owner/repo.git", DefaultBranch: "main", CreatedBy: 99},
		},
	}
	tokenReader := &fakeTokenReader{tokenErr: errors.New("token store down")}
	adapter := NewLookupAdapter(projectReader, tokenReader)

	_, err := adapter.LookupProject(context.Background(), 1, 42)
	if err == nil {
		t.Fatal("expected error when token reader fails")
	}
}

func TestLookupAdapter_TenantMismatch(t *testing.T) {
	projectReader := &fakeProjectReader{
		projects: map[int64]*Project{
			42: {ID: 42, TenantID: 1, CodeRepoURL: "https://github.com/owner/repo.git", DefaultBranch: "main", CreatedBy: 99},
		},
	}
	tokenReader := &fakeTokenReader{tokens: map[int64]string{99: "ghp_fake_token"}}
	adapter := NewLookupAdapter(projectReader, tokenReader)

	_, err := adapter.LookupProject(context.Background(), 2, 42)
	if !errors.Is(err, workspace.ErrProjectNotFound) {
		t.Errorf("expected ErrProjectNotFound when tenant mismatches, got %v", err)
	}
}
```

- [ ] **Step 6: Implement the adapter**

Create `forge-core/internal/module/project/lookup_adapter.go`:

```go
package project

import (
	"context"
	"errors"
	"fmt"

	"github.com/shulex/forge/forge-core/internal/workspace"
)

// ProjectReader is the subset of project.Service the lookup adapter
// needs. Defined as an interface so tests can supply fakes without
// constructing a full service with a real repository.
type ProjectReader interface {
	GetByIDInternal(ctx context.Context, projectID, tenantID int64) (*Project, error)
}

// TokenReader is the subset of the auth/token store that the lookup
// adapter needs: given a user id (the project creator), return a
// usable GitHub PAT. In Phase 1a this is used for every clone/fetch;
// Phase 1b replaces it with deploy keys and the token reader is
// deleted wholesale along with the adapter rewrite.
type TokenReader interface {
	GetGitHubTokenForUser(ctx context.Context, userID int64) (string, error)
}

// LookupAdapter satisfies workspace.ProjectLookup by wiring together
// a ProjectReader (for repo metadata) and a TokenReader (for the
// owner's GitHub PAT). Lives in the project module because it holds
// references to project-domain types; importing this from main.go
// avoids a package cycle with workspace.
//
// PHASE 1A: returns ProjectInfo with HTTPS RepoURL and GitHub PAT.
// Phase 1b rewrites this as a breaking change per spec §2.9.4.b —
// the token reader dependency is deleted and RepoURL becomes SSHURL.
type LookupAdapter struct {
	projects ProjectReader
	tokens   TokenReader
}

// NewLookupAdapter constructs a new adapter. Both arguments are required.
func NewLookupAdapter(projects ProjectReader, tokens TokenReader) *LookupAdapter {
	return &LookupAdapter{projects: projects, tokens: tokens}
}

// LookupProject implements workspace.ProjectLookup. It reads the
// project row, enforces tenant isolation, fetches the creator's
// GitHub PAT, and assembles the Phase 1a ProjectInfo.
func (a *LookupAdapter) LookupProject(
	ctx context.Context,
	tenantID, projectID int64,
) (workspace.ProjectInfo, error) {
	proj, err := a.projects.GetByIDInternal(ctx, projectID, tenantID)
	if err != nil {
		// Translate project-domain errors into the workspace-domain
		// sentinel so EnsureReady can use errors.Is.
		if errors.Is(err, ErrProjectNotFound) {
			return workspace.ProjectInfo{}, workspace.ErrProjectNotFound
		}
		return workspace.ProjectInfo{}, fmt.Errorf("lookup: get project: %w", err)
	}

	token, err := a.tokens.GetGitHubTokenForUser(ctx, proj.CreatedBy)
	if err != nil {
		return workspace.ProjectInfo{}, fmt.Errorf("lookup: get github token for user %d: %w", proj.CreatedBy, err)
	}

	return workspace.ProjectInfo{
		RepoURL:       proj.CodeRepoURL,
		AccessToken:   token,
		DefaultBranch: proj.DefaultBranch,
	}, nil
}
```

**Note:** this file assumes the project module exports `Project` struct with `ID`, `TenantID`, `CodeRepoURL`, `DefaultBranch`, `CreatedBy` fields, and an `ErrProjectNotFound` sentinel. If any of these are missing, add them in the same commit — they are the canonical project-domain shape the adapter needs. The `GetByIDInternal` method is added in Task 1a.7 (main.go wiring) alongside the `ProjectReader` interface satisfaction.

- [ ] **Step 7: Run the adapter tests**

```bash
cd forge-core && go test ./internal/module/project/... -run TestLookupAdapter -v
```

Expected: 4 tests pass (`_HappyPath`, `_ProjectNotFound`, `_TokenError`, `_TenantMismatch`).

If `Project`, `ErrProjectNotFound`, or `GetByIDInternal` don't exist in the project module, add them before proceeding. These are thin additions to `forge-core/internal/module/project/service.go` + `repository.go`.

- [ ] **Step 8: Commit**

```bash
git add forge-core/internal/workspace/lookup.go forge-core/internal/workspace/lookup_test.go forge-core/internal/module/project/lookup_adapter.go forge-core/internal/module/project/lookup_adapter_test.go
git commit -m "$(cat <<'EOF'
feat(workspace): ProjectLookup interface + HTTPS+token adapter

EnsureReady needs project metadata (repo URL, default branch) and,
in Phase 1a, a GitHub PAT for injectToken. Importing project module
directly would create a cyclic dep because project already imports
workspace.WorkspaceProvider. Define a small interface in the workspace
package; the production adapter lives in internal/module/project.

PHASE 1A HTTPS+TOKEN SHAPE:
  type ProjectInfo struct {
      RepoURL       string  // HTTPS URL
      AccessToken   string  // GitHub PAT
      DefaultBranch string
  }

Phase 1b rewrites this as a breaking change per spec §2.9.4.b —
the AccessToken field is dropped and RepoURL is renamed to SSHURL.
All callers migrate in the same Phase 1b commit. Hard cutover is
safe because ProjectLookup has no external consumers.

The production adapter wraps two interfaces: ProjectReader
(GetByIDInternal) and TokenReader (GetGitHubTokenForUser). Both
are tested with fakes; the real wiring lands in main.go (Task 1a.7).

Part of chronos Phase 1a; will be rewritten in Phase 1b.
EOF
)"
```

---

### Task 1a.4: `EnsureReady` state machine — core loop (HTTPS+token variant)

**Files:**
- Create: `forge-core/internal/workspace/ensure.go`
- Create: `forge-core/internal/workspace/ensure_test.go`
- Create: `forge-core/internal/workspace/git.go` (Phase 1a HTTPS+token variant)

**Context:** This is the heart of Phase 1a. `EnsureReady(ctx, tenantID, projectID, forceSync)` is the single public entry point that drives the state machine. It handles all five cases: no row → create + clone, pending row → wait on advisory lock, ready row → maybe resync, ready row + forceSync=true → fetch+reset, error row → wipe + retry from scratch.

The method takes dependencies through the `Manager` struct (updated in Task 1a.5). The unit test uses fakes for `gitRunner`, `prepClient`, and `lookup`. A full mock-free integration test that drives real git against a local bare repo comes in Task 1a.8.

**Phase 1a changes vs Round 1 Task 1.7:**

- No `deployKeys` field on the fixture or the `Manager`
- No `ghUploader` field on the fixture or the `Manager`
- No `generateAndUploadDeployKey` step in `freshInstall`
- `RealGitRunner` uses HTTPS+token via `injectToken` instead of SSH + `GIT_SSH_COMMAND`
- The `git.go` module exports `RealGitRunner` with three methods: `Clone`, `Fetch`, `ResetHard`
- Error classification inside `git.go` inspects git stderr for "authentication" / "401" (auth), "could not resolve host" / "timeout" (network), otherwise unknown
- Test fixture initializes `memoryLookup` with HTTPS+token data

Method signature:
```go
func (m *Manager) EnsureReady(ctx context.Context, tenantID, projectID int64, forceSync bool) (*Workspace, error)
```

- [ ] **Step 1: Write the failing `git.go` module unit tests**

Create `forge-core/internal/workspace/git_test.go` (or, alternatively, combine with `ensure_test.go` — the integration test in Task 1a.8 is the authoritative coverage, but we want a compile-time smoke test here):

```go
package workspace

import (
	"errors"
	"strings"
	"testing"
)

func TestInjectToken_GitHubHTTPS(t *testing.T) {
	got := injectToken("https://github.com/owner/repo.git", "ghp_fake_token")
	want := "https://x-access-token:ghp_fake_token@github.com/owner/repo.git"
	if got != want {
		t.Errorf("injectToken: got %s, want %s", got, want)
	}
}

func TestInjectToken_EmptyToken(t *testing.T) {
	got := injectToken("https://github.com/owner/repo.git", "")
	want := "https://github.com/owner/repo.git"
	if got != want {
		t.Errorf("injectToken empty: got %s, want %s", got, want)
	}
}

func TestInjectToken_NonHTTPSUnchanged(t *testing.T) {
	// file:// URLs (used by integration tests) should pass through
	// unchanged — there's no https:// prefix to replace.
	got := injectToken("file:///tmp/bare-repo.git", "ghp_token")
	want := "file:///tmp/bare-repo.git"
	if got != want {
		t.Errorf("injectToken file://: got %s, want %s", got, want)
	}
}

func TestClassifyGitError_Auth(t *testing.T) {
	stderr := "fatal: Authentication failed for 'https://github.com/owner/repo.git/'\n"
	err := classifyGitError(errors.New("exit status 128"), stderr)
	var authErr *AuthError
	if !errors.As(err, &authErr) {
		t.Errorf("expected AuthError, got %T: %v", err, err)
	}
}

func TestClassifyGitError_401(t *testing.T) {
	stderr := "remote: The requested URL returned error: 401\n"
	err := classifyGitError(errors.New("exit status 128"), stderr)
	var authErr *AuthError
	if !errors.As(err, &authErr) {
		t.Errorf("expected AuthError for 401, got %T", err)
	}
}

func TestClassifyGitError_Network_UnresolvedHost(t *testing.T) {
	stderr := "fatal: unable to access 'https://github.com/owner/repo.git/': Could not resolve host: github.com\n"
	err := classifyGitError(errors.New("exit status 128"), stderr)
	var netErr *NetworkError
	if !errors.As(err, &netErr) {
		t.Errorf("expected NetworkError for unresolved host, got %T", err)
	}
}

func TestClassifyGitError_Network_Timeout(t *testing.T) {
	stderr := "fatal: unable to access 'https://github.com/owner/repo.git/': Connection timed out\n"
	err := classifyGitError(errors.New("exit status 128"), stderr)
	var netErr *NetworkError
	if !errors.As(err, &netErr) {
		t.Errorf("expected NetworkError for timeout, got %T", err)
	}
}

func TestClassifyGitError_Unknown(t *testing.T) {
	stderr := "fatal: not a git repository\n"
	err := classifyGitError(errors.New("exit status 128"), stderr)
	// Should NOT be auth or network
	var authErr *AuthError
	var netErr *NetworkError
	if errors.As(err, &authErr) || errors.As(err, &netErr) {
		t.Errorf("expected UnknownGitError, got typed %T", err)
	}
	if !strings.Contains(err.Error(), "not a git repository") {
		t.Errorf("unknown error should include stderr: %v", err)
	}
}
```

- [ ] **Step 2: Write the failing ensure_test.go (core cases)**

Create `forge-core/internal/workspace/ensure_test.go`:

```go
package workspace

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

// --- Test fakes ---

type fakeGitRunner struct {
	mu               sync.Mutex
	cloneCalls       int
	fetchCalls       int
	resetCalls       int
	cloneShouldFail  bool
	fetchShouldFail  bool
	resetShouldFail  bool
	clonedDirs       []string
	lastCloneBranch  string
	lastCloneToken   string
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
	db       *sql.DB
	manager  *Manager
	git      *fakeGitRunner
	prep     *fakePrepClient
	lookup   *memoryLookup
	rootDir  string
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
				RepoURL:       "https://github.com/owner/repo.git",
				AccessToken:   "ghp_fake_token",
				DefaultBranch: "main",
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
		db:      db,
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
```

Note: the test imports `strings` for `TestEnsureReady_GitHubPATRevoked_MarksErrorWithAuthFailed`. Add `"strings"` to the import list above.

- [ ] **Step 3: Run tests — expect compile failure**

Run: `cd forge-core && go test ./internal/workspace/... -run "TestEnsureReady|TestInjectToken|TestClassifyGitError"`
Expected: `undefined: Manager field stateRepo/gitRunner/prepClient/projectLookup`, `undefined: injectToken`, `undefined: AuthError`, `undefined: NetworkError`, `undefined: classifyGitError`, `undefined: RealGitRunner`. This is the TDD failure mode — we haven't written the production code yet.

- [ ] **Step 4: Implement `git.go` (Phase 1a HTTPS+token variant)**

Create `forge-core/internal/workspace/git.go`:

```go
package workspace

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// gitRunner is the interface the EnsureReady state machine uses for
// git operations. RealGitRunner is the production implementation;
// fakeGitRunner (in ensure_test.go) is the test stub.
//
// The interface is deliberately narrow — three methods — because
// Phase 1a only needs clone, fetch, and reset-hard. Phase 1b may
// rewrite this to carry a *DeployKey argument; the state machine
// callers shield upstream code from that change.
type gitRunner interface {
	Clone(ctx context.Context, hostPath, httpsURL, token, branch string) error
	Fetch(ctx context.Context, hostPath, httpsURL, token string) error
	ResetHard(ctx context.Context, hostPath, branch string) error
}

// RealGitRunner shells out to the system `git` binary via os/exec.
// It builds token-injected HTTPS URLs inline and classifies git
// failures into AuthError / NetworkError / wrapped errors based on
// stderr patterns.
//
// PHASE 1A: uses HTTPS+token via injectToken. Phase 1b rewrites this
// file wholesale to use SSH deploy keys via GIT_SSH_COMMAND and a
// tempfile-managed private key. The temporary HTTPS+token surface is
// intentionally confined to this file per spec §2.9.4.c.
type RealGitRunner struct{}

// NewRealGitRunner constructs a stateless git runner. All arguments
// are passed to each method; the struct holds no per-request state,
// so it's safe to share across goroutines.
func NewRealGitRunner() *RealGitRunner {
	return &RealGitRunner{}
}

// Clone runs `git clone --depth=50 --branch <branch> <injected-url> <hostPath>`.
// The hostPath's parent directory must exist; Clone does not MkdirAll.
// Errors are classified via classifyGitError.
func (r *RealGitRunner) Clone(
	ctx context.Context,
	hostPath, httpsURL, token, branch string,
) error {
	authURL := injectToken(httpsURL, token)
	cmd := exec.CommandContext(ctx, "git", "clone",
		"--depth=50",
		"--branch", branch,
		authURL,
		hostPath,
	)
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return classifyGitError(err, string(out))
	}
	return nil
}

// Fetch runs `git -C <hostPath> fetch <injected-url>`.
// Unlike pull, fetch doesn't try to merge; the caller follows up with
// ResetHard to move the working tree to origin/<branch>. This matches
// the spec §3.7 state machine's resync transition.
func (r *RealGitRunner) Fetch(
	ctx context.Context,
	hostPath, httpsURL, token string,
) error {
	authURL := injectToken(httpsURL, token)
	// Use the injected URL explicitly so we don't depend on the repo's
	// stored `origin` remote — which would also contain a token from
	// the original clone. Being explicit keeps credentials off disk in
	// the git config.
	cmd := exec.CommandContext(ctx, "git", "-C", hostPath, "fetch", authURL)
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return classifyGitError(err, string(out))
	}
	return nil
}

// ResetHard runs `git -C <hostPath> reset --hard FETCH_HEAD`.
// Must be called after a successful Fetch — it moves the working tree
// to whatever the last fetch brought down, discarding any local
// modifications. This is the "reset to clean main" step from spec §2.7.
func (r *RealGitRunner) ResetHard(
	ctx context.Context,
	hostPath, branch string,
) error {
	// FETCH_HEAD is what the preceding Fetch call updated. We could
	// also reset to origin/<branch>, but FETCH_HEAD is more robust
	// against rename/delete of the tracking branch.
	cmd := exec.CommandContext(ctx, "git", "-C", hostPath, "reset", "--hard", "FETCH_HEAD")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return classifyGitError(err, string(out))
	}
	return nil
}

// injectToken converts https://github.com/owner/repo.git to
// https://x-access-token:TOKEN@github.com/owner/repo.git.
// If token is empty, the URL is returned unchanged (used by the
// file:// integration test). If the URL doesn't start with "https://",
// it is also returned unchanged.
//
// PHASE 1A: this helper is the last remaining HTTPS+token code path.
// It lives in git.go (not manager.go) so the temporary surface is
// confined to one file — Phase 1b's first task is to delete this
// whole file and replace it with the SSH variant, per spec §2.9.4.c.
//
// Token is never logged (it's never written to slog anywhere in this
// file).
func injectToken(repoURL, token string) string {
	if token == "" {
		return repoURL
	}
	if !strings.HasPrefix(repoURL, "https://") {
		return repoURL
	}
	return strings.Replace(repoURL, "https://", fmt.Sprintf("https://x-access-token:%s@", token), 1)
}

// AuthError signals a git authentication failure (wrong/missing/
// revoked credentials). The state machine uses errors.As to detect
// this and produces a last_error containing "github_auth_failed" for
// the Phase 1a-only failure mode in spec §3.12.
type AuthError struct {
	stderr string
}

func (e *AuthError) Error() string {
	return "git auth failed: " + firstLine(e.stderr)
}

// NetworkError signals a git network failure (DNS, timeout, 5xx from
// github). Distinguished from auth because retry semantics differ —
// network failures often resolve on their own.
type NetworkError struct {
	stderr string
}

func (e *NetworkError) Error() string {
	return "git network error: " + firstLine(e.stderr)
}

// classifyGitError inspects git stderr and maps it to a typed error
// that the state machine can distinguish via errors.As. Unknown
// failures are wrapped as-is so the caller sees the full stderr.
func classifyGitError(baseErr error, stderr string) error {
	lower := strings.ToLower(stderr)
	switch {
	case strings.Contains(lower, "authentication") ||
		strings.Contains(lower, "401") ||
		strings.Contains(lower, "could not read username") ||
		strings.Contains(lower, "terminal prompts disabled"):
		return &AuthError{stderr: stderr}
	case strings.Contains(lower, "could not resolve host") ||
		strings.Contains(lower, "connection timed out") ||
		strings.Contains(lower, "connection refused") ||
		strings.Contains(lower, "network is unreachable"):
		return &NetworkError{stderr: stderr}
	default:
		return fmt.Errorf("git: %w: %s", baseErr, firstLine(stderr))
	}
}

// firstLine returns the first non-empty line of a string. Used by
// error constructors to keep error messages short — git's stderr is
// often multi-line and the first line is usually the most useful.
func firstLine(s string) string {
	for _, line := range strings.Split(s, "\n") {
		if trimmed := strings.TrimSpace(line); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

// unused suppression: errors package used in test file
var _ = errors.New
```

- [ ] **Step 5: Implement `ensure.go`**

Create `forge-core/internal/workspace/ensure.go`:

```go
package workspace

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
)

// EnsureReady drives the workspace state machine for (tenantID, projectID).
// It guarantees that on successful return, the workspace row is in
// 'ready' state and the filesystem has a valid clone with dependencies
// pre-installed (or prep was skipped as a non-blocking soft failure).
//
// Behavior by starting state:
//
//   no row          → create row('pending'), clone, prep, mark ready
//   row='pending'   → wait on advisory lock, observe final state
//   row='ready' + forceSync=false → no-op
//   row='ready' + forceSync=true  → fetch + reset --hard
//   row='error'     → wipe dir, reset row to 'pending', clone, prep, mark ready
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
//   1. Look up project metadata via ProjectLookup (HTTPS URL + PAT + branch)
//   2. Insert pending row
//   3. MkdirAll the parent dir
//   4. Clone the repo via RealGitRunner (HTTPS+token)
//   5. Call ai-worker prep (non-blocking)
//   6. Mark ready
//
// PHASE 1A: no deploy-key lifecycle. The entire "generate keypair +
// upload to GitHub" dance from Round 1's Task 1.7 is absent because
// HTTPS+token auth doesn't need it.
func (m *Manager) freshInstall(
	ctx context.Context,
	tenantID, projectID int64,
) (*Workspace, error) {
	proj, err := m.projectLookup.LookupProject(ctx, tenantID, projectID)
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
	if err := m.gitRunner.Clone(ctx, hostPath, proj.RepoURL, proj.AccessToken, proj.DefaultBranch); err != nil {
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
	proj, err := m.projectLookup.LookupProject(ctx, tenantID, projectID)
	if err != nil {
		return nil, fmt.Errorf("resync: project lookup: %w", err)
	}

	if err := m.gitRunner.Fetch(ctx, existing.HostPath, proj.RepoURL, proj.AccessToken); err != nil {
		slog.Warn("workspace: fetch failed; falling back to fresh clone",
			"tenant", tenantID, "project", projectID, "error", err)
		return m.wipeAndReclone(ctx, tenantID, projectID, existing.HostPath)
	}

	if err := m.gitRunner.ResetHard(ctx, existing.HostPath, proj.DefaultBranch); err != nil {
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
//   - AuthError  → "github_auth_failed: <stderr-line>"
//   - NetworkError → "clone failed: network: <stderr-line>"
//   - otherwise  → "clone failed: <err.Error()>"
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
```

Note: `strings` import needed at top of `ensure.go` — add it if Go complains.

- [ ] **Step 6: The tests need `Manager` to have new fields — deferred to Task 1a.5**

The tests reference `Manager.stateRepo`, `Manager.gitRunner`, `Manager.prepClient`, `Manager.projectLookup`. Those fields don't exist yet on the current single-field `Manager`. They get added in **Task 1a.5** when we refactor `manager.go`. So `go test` will still fail to compile after this task.

**That's expected.** The next task (1a.5) is specifically the `Manager` struct refactor. After 1a.5, both tests and `ensure.go` compile; after 1a.5's test run, the Task 1a.4 tests in this task file start passing.

Go to Task 1a.5 now — don't try to fix the compile error in isolation.

- [ ] **Step 7: Commit the state machine (with known pending compile error)**

```bash
git add forge-core/internal/workspace/ensure.go forge-core/internal/workspace/ensure_test.go forge-core/internal/workspace/git.go forge-core/internal/workspace/git_test.go
git commit -m "$(cat <<'EOF'
feat(workspace): EnsureReady state machine + git.go HTTPS+token runner (WIP: depends on 1a.5)

EnsureReady is the single public entry point for the workspace lifecycle.
Handles five starting states under a pg_advisory_xact_lock:
  no row → fresh install (clone + prep + mark ready)
  ready + !forceSync → no-op
  ready + forceSync → fetch + reset --hard (fallback: wipe + reclone)
  error → wipe + retry freshInstall
  pending → treat as crashed previous run, wipe + retry

Dep prep is non-blocking: transport failures and 'error' results log
warnings but still mark the row ready, matching spec §3.9.

git.go implements RealGitRunner with three methods (Clone/Fetch/ResetHard)
that shell out to the system git binary. injectToken builds
https://x-access-token:TOKEN@github.com/... URLs inline. classifyGitError
inspects stderr for 'authentication'/'401' → AuthError, 'could not
resolve host'/'timeout' → NetworkError, otherwise wraps as-is. The
state machine's formatCloneError maps AuthError → 'github_auth_failed'
(Phase 1a-only failure mode per spec §3.12).

PHASE 1A TEMP SURFACE: injectToken + HTTPS+token path live ONLY in
git.go. Phase 1b deletes this file wholesale per spec §2.9.4.c.

Concurrency test spawns 3 goroutines and asserts only 1 clone happens
— advisory lock serializes them.

NOTE: this commit does NOT compile yet. Manager still has a single
'root' field; the ensure.go impl references stateRepo/gitRunner/
prepClient/projectLookup which are added in the next task (1a.5
manager.go refactor). Landing the state machine first so the test
suite is in place to verify 1a.5.
EOF
)"
```

---

### Task 1a.5: Refactor `manager.go` — add `EnsureReady`, keep `injectToken` out of manager

**Files:**
- Modify: `forge-core/internal/workspace/manager.go`
- Modify: `forge-core/internal/workspace/manager_test.go`

**Context:** The current `Manager` struct is `{root string}` with methods `NewManager`, `ProjectDir`, `TaskDir`, `EnsureClone`, `CreateWorktree`, `WriteFiles`, `CleanupTask`, and the private top-level `injectToken`. Phase 1a changes:

1. Add fields for the dependencies `EnsureReady` uses: `stateRepo`, `gitRunner`, `prepClient`, `projectLookup`
2. Update `NewManager` to take those as a single Config struct (constructor explosion is worse than a 5-field Config)
3. **Delete `EnsureClone`** — after its callers migrate in Task 1a.6
4. **Move `injectToken` to `git.go`** — which already has it from Task 1a.4. The manager.go `injectToken` is deleted; the call from `EnsureClone` is deleted along with `EnsureClone` itself. `injectToken` continues to exist, but only inside `git.go` where `RealGitRunner` uses it.
5. Update `manager_test.go` to remove the now-deleted `TestInjectToken` and `TestInjectToken_NoLeakInLogs` from the Round 1 file (if present) — note that `git.go` now has its own `TestInjectToken_*` tests from Task 1a.4
6. Keep `ProjectDir`, `TaskDir`, `CreateWorktree`, `WriteFiles`, `CleanupTask`, `FileToWrite` unchanged
7. Add `EnsureReady` as a method (the body lives in `ensure.go`, but it's a `Manager` method so the struct must exist)

**Important: `EnsureClone` is KEPT in this task**, because the callers haven't been migrated yet. Task 1a.6 migrates callers, then this same commit or a follow-up deletes `EnsureClone`. In this task we add `EnsureReady` alongside `EnsureClone` so the Manager has both methods for the duration of Task 1a.6. Task 1a.6's last step deletes `EnsureClone`.

**Phase 1a vs Round 1 Task 1.8:** Round 1 Task 1.8 deleted `EnsureClone` AND `injectToken` from `manager.go` in a single commit because deploy keys immediately replaced both. Phase 1a keeps `injectToken` in `git.go` (where `RealGitRunner` needs it) but removes it from `manager.go` — after Task 1a.6 migrates callers and deletes `EnsureClone`, `manager.go` no longer references `injectToken` at all. The helper stays in `git.go` until Phase 1b deletes that file wholesale.

- [ ] **Step 1: Read the current `manager.go` to confirm what's changing**

Already reviewed above. Current struct:
```go
type Manager struct {
    root string
}
```

Current constructor: `func NewManager(root string) *Manager`.

Methods to keep: `ProjectDir`, `TaskDir`, `CreateWorktree`, `WriteFiles`, `CleanupTask`.
Methods to keep (temporarily, deleted in Task 1a.6): `EnsureClone`.
Methods to add: `EnsureReady` (body in ensure.go).
Helpers to delete from manager.go (still exists in git.go): `injectToken`.

- [ ] **Step 2: Rewrite `manager.go`**

Replace the entire content of `forge-core/internal/workspace/manager.go`:

```go
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
// PHASE 1A: auth is HTTPS+token via RealGitRunner's internal injectToken.
// Phase 1b rewrites this to SSH deploy keys; see docs/specs/... §2.9.4.
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

// Config bundles Manager dependencies. Passing a struct avoids a
// 5-parameter NewManager call and makes it clear what's optional
// (nil stateRepo/gitRunner/prepClient/projectLookup all degrade
// gracefully — EnsureReady returns a descriptive error).
type Config struct {
	Root          string       // FORGE_WORKSPACE_ROOT; defaults to /data/forge/workspaces
	StateRepo     *StateRepo   // engine.workspaces DAO; nil disables EnsureReady
	GitRunner     gitRunner    // HTTPS+token git wrapper; typically *RealGitRunner
	PrepClient    prepRunner   // ai-worker /api/workspace/prep client; typically *PrepClient
	ProjectLookup ProjectLookup // project metadata + HTTPS URL + token
}

// prepRunner is the interface the state machine uses to run dep prep.
// Having it as an interface lets tests swap in a fake.
type prepRunner interface {
	Prep(ctx context.Context, tenantID, projectID int64, wsPath string) (*PrepResult, error)
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

// EnsureClone is the legacy entry point retained ONLY for the duration
// of Task 1a.6 (caller migration). The body delegates to a small local
// helper that uses git with HTTPS+token via the manager's direct
// exec.Command — NOT through RealGitRunner. This is a stepping stone:
// after Task 1a.6 migrates both callers, this method is deleted and
// only EnsureReady remains.
//
// DEPRECATED: migrate to EnsureReady. Will be removed at the end of Task 1a.6.
func (m *Manager) EnsureClone(
	ctx context.Context,
	tenantID, projectID int64,
	repoURL, token, defaultBranch string,
) (string, error) {
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

	// Token injection is done inline here (not via the git.go helper)
	// because this code path is going away. The git.go helper lives in
	// RealGitRunner's methods and is the long-lived Phase 1a path.
	authURL := repoURL
	if token != "" && strings.HasPrefix(repoURL, "https://") {
		authURL = strings.Replace(repoURL, "https://", fmt.Sprintf("https://x-access-token:%s@", token), 1)
	}

	slog.Info("workspace: cloning repo (legacy EnsureClone)", "project_id", projectID, "dir", dir)
	cmd := exec.CommandContext(ctx, "git", "clone", "--depth=50", "--branch", defaultBranch, authURL, dir)
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
	if out, err := cmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("git clone failed: %s: %w", string(out), err)
	}

	return dir, nil
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
```

- [ ] **Step 3: Update `manager_test.go`**

Edit `forge-core/internal/workspace/manager_test.go`. The existing tests probably call `NewManager("")` with a single string argument. Update them to use the Config form.

If `TestInjectToken` or `TestInjectToken_NoLeakInLogs` exist in this file, delete them — `injectToken` no longer lives in `manager.go`. It's tested via `TestInjectToken_*` in `git_test.go` (added in Task 1a.4).

Update `TestNewManager_DefaultRoot`:
```go
func TestNewManager_DefaultRoot(t *testing.T) {
	m := NewManager(Config{})
	if m.root != "/data/forge/workspaces" {
		t.Errorf("expected default root, got %s", m.root)
	}
}
```

Update `TestNewManager_CustomRoot`:
```go
func TestNewManager_CustomRoot(t *testing.T) {
	m := NewManager(Config{Root: "/tmp/test-workspaces"})
	if m.root != "/tmp/test-workspaces" {
		t.Errorf("expected /tmp/test-workspaces, got %s", m.root)
	}
}
```

Update `TestProjectDir`:
```go
func TestProjectDir(t *testing.T) {
	m := NewManager(Config{Root: "/data/ws"})
	dir := m.ProjectDir(1, 42)
	expected := filepath.Join("/data/ws", "tenant-1", "project-42", "repo")
	if dir != expected {
		t.Errorf("expected %s, got %s", expected, dir)
	}
}
```

Update `TestTaskDir`:
```go
func TestTaskDir(t *testing.T) {
	m := NewManager(Config{Root: "/data/ws"})
	dir := m.TaskDir(1, 42, 99)
	expected := filepath.Join("/data/ws", "tenant-1", "project-42", "tasks", "task-99")
	if dir != expected {
		t.Errorf("expected %s, got %s", expected, dir)
	}
}
```

Update `TestWriteFiles` and `TestWriteFiles_NestedDirs` — both use `NewManager("")` which now requires a Config arg:
```go
m := NewManager(Config{})
```

Remove any imports that become unused (`strings` if `TestInjectToken_NoLeakInLogs` was using it).

- [ ] **Step 4: Run the workspace tests**

```bash
cd forge-core && go test ./internal/workspace/... -v
```
Expected status:
- Pure-unit tests pass: `TestNewManager_DefaultRoot`, `_CustomRoot`, `TestProjectDir`, `TestTaskDir`, `TestWriteFiles`, `TestWriteFiles_NestedDirs`, `TestInjectToken_*` (in git_test.go), `TestClassifyGitError_*` (in git_test.go), `TestPrepClient_*`, `TestProjectInfo_*`, `TestMemoryLookup*`, `TestErrProjectNotFound`
- DB integration tests pass if `FORGE_TEST_DATABASE_URL` is set: `TestStateRepo_*`, `TestEnsureReady_*`
- DB integration tests skip cleanly if `FORGE_TEST_DATABASE_URL` is unset

If any fail, fix before proceeding.

- [ ] **Step 5: Run `go build` on the whole forge-core tree**

```bash
cd forge-core && go build ./...
```
Expected: clean build. `EnsureClone` still exists so `build_activities.go:96` and `devops_activities.go:134` still compile. `NewManager` signature changed to `Config` form, so `main.go:122` breaks. That's the handoff to Task 1a.7.

- [ ] **Step 6: Commit**

```bash
git add forge-core/internal/workspace/manager.go forge-core/internal/workspace/manager_test.go
git commit -m "$(cat <<'EOF'
refactor(workspace): add EnsureReady to Manager; keep injectToken for Phase 1a

Manager now takes a Config struct with stateRepo/gitRunner/prepClient/
projectLookup. Nil dependencies are allowed (e.g. tests that only need
ProjectDir can pass Config{}).

Phase 1a-specific notes:
- EnsureClone is KEPT temporarily. Task 1a.6 migrates the two activity
  callers, then EnsureClone is deleted at the end of that task.
- injectToken is no longer in manager.go. It lives in git.go where
  RealGitRunner uses it. manager.go still has its OWN inline token
  injection inside the legacy EnsureClone body, but that body goes
  away in Task 1a.6.
- TestInjectToken and TestInjectToken_NoLeakInLogs (if present) are
  removed from manager_test.go. injectToken is tested in git_test.go.

Kept (used by the temporal worker and project module):
- ProjectDir, TaskDir, CreateWorktree, WriteFiles, CleanupTask

Added:
- SetLookup (post-construction wire to break the project<->workspace
  cycle; see main.go wiring in Task 1a.7)

Compile breaks in main.go:122 are expected — NewManager signature
changed from (string) to (Config). Fixed in Task 1a.7.
EOF
)"
```

---

### Task 1a.6: Migrate callers to `EnsureReady` and delete `EnsureClone`

**Files:**
- Modify: `forge-core/internal/temporal/activity/build_activities.go`
- Modify: `forge-core/internal/temporal/activity/devops_activities.go`
- Modify: `forge-core/internal/module/agent/service.go`
- Modify: `forge-core/internal/module/agent/service_test.go` (compile-clean updates only)
- Modify: `forge-core/internal/workspace/manager.go` (delete `EnsureClone` at the end)

**Context:** This task merges Round 1 Tasks 1.9, 1.10, and 1.11 into one because the three files are small and cohesive — they're the three call sites that need migrating from `EnsureClone` to `EnsureReady`. At the end of the task, `EnsureClone` is deleted from `manager.go`.

Three migration steps:

1. `build_activities.go:96` — replace `EnsureClone(ctx, tenantID, projectID, repoURL, token, branch)` with `EnsureReady(ctx, tenantID, projectID, false)`. Drop the now-unused `repoURL`/`token`/`branch` locals.
2. `devops_activities.go:134` — same migration.
3. `agent/service.go:SubmitMessage` — replace the `os.Stat(.git)` probe with a call to `workspace.Manager.EnsureReady` before starting the agent. `forceSync = (req.SessionID == "")`.

After all three migrations, `go build ./...` verifies `EnsureClone` is no longer referenced; delete the method from `manager.go`.

- [ ] **Step 1: Migrate `build_activities.go`**

Edit `forge-core/internal/temporal/activity/build_activities.go`. Find the block around line 96:

```go
		// Step 1: Clone/pull repo to workspace
		defaultBranch := "main"
		repoDir, err := a.ws.EnsureClone(ctx, input.TenantID, input.ProjectID,
			input.RepoURL, input.GitHubToken, defaultBranch)
		if err != nil {
			slog.Warn("workspace clone failed", "error", err)
		} else {
```

Replace with:

```go
		// Step 1: Ensure workspace is ready. Don't force a sync — we
		// want to use whatever's already cloned, or clone fresh if
		// nothing is there. Agent sessions drive forceSync explicitly.
		ws, err := a.ws.EnsureReady(ctx, input.TenantID, input.ProjectID, false)
		if err != nil {
			slog.Warn("workspace ensure ready failed", "error", err)
		} else {
			repoDir := ws.HostPath
```

Note the `else {` branch: `repoDir := ws.HostPath` replaces the old `repoDir` binding that came from `EnsureClone`'s first return value.

Check the closing brace of the `else` branch — the existing code has a deeply nested structure; make sure the braces still balance.

- [ ] **Step 2: Migrate `devops_activities.go`**

Edit `forge-core/internal/temporal/activity/devops_activities.go`. Find the call around line 134:

```go
		if _, err := a.ws.EnsureClone(ctx, input.TenantID, input.ProjectID, proj.CodeRepoURL, token, proj.DefaultBranch); err != nil {
			slog.Warn("workspace: clone failed, skipping local copy", "task_id", input.TaskID, "error", err)
		} else {
```

Replace with:

```go
		if _, err := a.ws.EnsureReady(ctx, input.TenantID, input.ProjectID, false); err != nil {
			slog.Warn("workspace: ensure ready failed, skipping local copy", "task_id", input.TaskID, "error", err)
		} else {
```

The `proj.CodeRepoURL` / `token` / `proj.DefaultBranch` arguments are no longer needed in the call — `workspace.EnsureReady` looks them up internally via `ProjectLookup`. Leave the surrounding `proj` variable alone; it's probably still used by other lines in the function.

- [ ] **Step 3: Migrate `agent/service.go`**

Edit `forge-core/internal/module/agent/service.go`. Find the block around line 80-95 (the `os.Stat(gitDir)` probe):

```go
	if s.wsManager != nil && tenantID > 0 {
		absDir := s.wsManager.ProjectDir(tenantID, projectID)
		gitDir := filepath.Join(absDir, ".git")
		if _, err := os.Stat(gitDir); err == nil {
			body.WorkspacePath = fmt.Sprintf("tenant-%d/project-%d/repo", tenantID, projectID)
		} else if !os.IsNotExist(err) {
			slog.Warn("agent service: unexpected stat error on .git dir, treating as missing",
				"tenant_id", tenantID,
				"project_id", projectID,
				"path", gitDir,
				"error", err,
			)
		}
	}
```

Replace with:

```go
	// Ensure the workspace is ready before we dispatch to ai-worker.
	// A new session (empty SessionID) triggers a fetch+reset so the
	// agent starts from clean main; otherwise we reuse the existing
	// state so multi-turn edits persist across messages. Per spec §2.7.
	if s.wsManager != nil && tenantID > 0 {
		isNewSession := req.SessionID == ""
		ws, err := s.wsManager.EnsureReady(ctx, tenantID, projectID, isNewSession)
		if err != nil {
			// Workspace setup failed — return an error to the caller
			// so the agent session fails fast with a visible message.
			return nil, fmt.Errorf("workspace not ready: %w", err)
		}
		body.WorkspacePath = fmt.Sprintf("tenant-%d/project-%d/repo", tenantID, projectID)
		_ = ws // ws.HostPath available if ever needed
	}
```

Remove the `os` and `path/filepath` imports if they're no longer used elsewhere in the file. Check with `grep -n "os\.\|filepath\." forge-core/internal/module/agent/service.go` before removing.

- [ ] **Step 4: Compile check**

```bash
cd forge-core && go build ./...
```
Expected: **compile error** in `main.go:122` because `NewManager` signature changed in Task 1a.5. That's Task 1a.7's job. Verify the error is only in main.go:
```bash
cd forge-core && go build ./... 2>&1 | grep -E "(error|undefined|cannot)"
```

- [ ] **Step 5: Run activity tests**

```bash
cd forge-core && go test ./internal/temporal/activity/... -run "TestBuild|TestDevops" -v 2>&1 | tail -30
```
Expected: existing tests either pass or skip (some require k8s/docker that isn't present). Key check: no compile errors, no runtime panics about `EnsureClone`.

If tests can't run due to unrelated dependencies, verify the change at least compiles via `go vet`:
```bash
cd forge-core && go vet ./internal/temporal/activity/...
```

- [ ] **Step 6: Run agent tests**

```bash
cd forge-core && go test ./internal/module/agent/... -v 2>&1 | tail -30
```
Expected: builds, existing tests pass (they mostly exercise HTTP-to-ai-worker plumbing, not workspace). If any tests construct a `*workspace.Manager` directly with the old signature, update them to `workspace.NewManager(workspace.Config{})`.

- [ ] **Step 7: Delete `EnsureClone` from `manager.go`**

Edit `forge-core/internal/workspace/manager.go`. Delete the `EnsureClone` method (entire function from `// EnsureClone is the legacy entry point...` through its closing `}`). Also delete the `strings` import if it's no longer used (check: `grep -n "strings\." forge-core/internal/workspace/manager.go`).

- [ ] **Step 8: Verify `injectToken` is no longer in `manager.go`**

```bash
grep -n injectToken forge-core/internal/workspace/manager.go
```
Expected: no matches. `injectToken` lives only in `git.go` now.

```bash
grep -rn injectToken forge-core/
```
Expected: only `forge-core/internal/workspace/git.go` and `forge-core/internal/workspace/git_test.go`.

- [ ] **Step 9: Final build**

```bash
cd forge-core && go build ./...
```
Expected: **still broken** in `main.go:122`. That's Task 1a.7. Don't fix here.

```bash
cd forge-core && go build ./... 2>&1 | grep -v "cmd/forge-core/main.go" | grep error
```
Expected: no output (main.go is the only remaining break).

- [ ] **Step 10: Commit**

```bash
git add forge-core/internal/temporal/activity/build_activities.go forge-core/internal/temporal/activity/devops_activities.go forge-core/internal/module/agent/service.go forge-core/internal/workspace/manager.go
git add forge-core/internal/module/agent/service_test.go 2>/dev/null || true
git commit -m "$(cat <<'EOF'
refactor(callers): migrate EnsureClone → EnsureReady; delete legacy path

Three call sites migrated to the new state machine:

1. build_activities.go:96 — build activity drops repoURL/token/branch
   params; workspace.EnsureReady resolves them via ProjectLookup.
2. devops_activities.go:134 — devops activity same migration.
3. agent/service.go — replace os.Stat(.git) probe with
   workspace.EnsureReady(forceSync=SessionID==""). New sessions fetch
   + reset to clean main; multi-turn messages in existing sessions
   reuse working tree (spec §2.7).

After migration, EnsureClone is deleted from manager.go. The helper
injectToken no longer exists in manager.go (it lives in git.go now,
used only by RealGitRunner). Phase 1b will delete git.go wholesale
when it switches to SSH deploy keys.

EnsureReady failures propagate as errors from SubmitMessage —
previously a missing clone silently left workspace_path empty and
let ai-worker's pair_pipeline fallback kick in. A2 has no
pair_pipeline, so failing fast here is the right move.

After this commit, main.go is the last EnsureClone/NewManager
signature break — fixed in Task 1a.7.
EOF
)"
```

---

### Task 1a.7: Wire everything into `main.go`

**Files:**
- Modify: `forge-core/cmd/forge-core/main.go`
- Modify: `forge-core/internal/config/config.go`
- Modify: `forge-core/internal/module/project/service.go` (add `GetByIDInternal`, if missing)
- Modify: `forge-core/internal/module/project/repository.go` (add `GetByIDInternal`, if missing)

**Context:** The capstone task for Phase 1a. `main.go:122` currently calls `workspace.NewManager(cfg.WorkspaceRoot)` (the old single-arg signature). We need to:

1. Read `FORGE_AI_WORKER_URL` (probably already in config as `AIWorkerURL`)
2. Construct `workspace.NewStateRepo(db)`
3. Construct `workspace.NewRealGitRunner()` (Phase 1a: no SSH deps)
4. Construct `workspace.NewPrepClient(cfg.AIWorkerURL)`
5. Construct the `project.NewLookupAdapter(projectReader, tokenReader)` from Task 1a.3
6. Pass everything into `workspace.NewManager(workspace.Config{...})`
7. Back-wire `ProjectLookup` via `SetLookup` after `projectService` is constructed (chicken-and-egg)

**No deploy-key crypto service, no secrets store, no `DeployKeyRepo`, no `github.NewClient` for deploy keys, no `FORGE_SECRETS_MASTER_KEY`, no `FORGE_GITHUB_API_URL`.** Phase 1b adds all of those. Phase 1a is minimal.

The `projectReader` is satisfied by adding a `GetByIDInternal(ctx, projectID, tenantID)` method to `project.Service` (if it doesn't exist). The `tokenReader` is satisfied by a small wrapper in `main.go` that delegates to `auth.Service.GetGitHubToken(userID)` — assuming `auth.Service` has such a method. If it doesn't, add it; if even the auth store doesn't have per-user tokens yet, add a minimal one (this is Phase 1a's one load-bearing config surface).

- [ ] **Step 1: Add config fields**

Edit `forge-core/internal/config/config.go`. Confirm `AIWorkerURL` already exists; if not, add:

```go
	// AIWorkerURL is the base URL for ai-worker (used by the workspace
	// prep client and agent service).
	AIWorkerURL string
```

And in `Load`:
```go
	AIWorkerURL: getEnv("FORGE_AI_WORKER_URL", "http://host.docker.internal:8090"),
```

No new Phase 1a config fields beyond this. `FORGE_SECRETS_MASTER_KEY` and `FORGE_GITHUB_API_URL` are Phase 1b additions.

- [ ] **Step 2: Add `GetByIDInternal` to project module if missing**

Check if `project.Service` has `GetByIDInternal`:

```bash
grep -n "GetByIDInternal" forge-core/internal/module/project/service.go
```

If not, add to `service.go`:

```go
// GetByIDInternal loads a project by ID for internal callers that
// bypass normal RBAC. tenantID=0 means "any tenant" — caller must
// have already authorized the access. Used by the workspace lookup
// adapter which is called from EnsureReady's per-tenant, per-project
// context.
func (s *Service) GetByIDInternal(ctx context.Context, projectID, tenantID int64) (*Project, error) {
	return s.repo.GetByIDInternal(ctx, projectID, tenantID)
}
```

And in `repository.go`, add the corresponding method:

```go
// GetByIDInternal fetches a project row by ID with optional tenant
// filtering. If tenantID is 0, tenant isolation is not enforced
// (caller's responsibility). Returns ErrProjectNotFound if no row
// matches.
func (r *Repository) GetByIDInternal(ctx context.Context, projectID, tenantID int64) (*Project, error) {
	var (
		q    string
		args []any
	)
	if tenantID == 0 {
		q = `SELECT id, tenant_id, code_repo_url, default_branch, created_by FROM engine.projects WHERE id = $1`
		args = []any{projectID}
	} else {
		q = `SELECT id, tenant_id, code_repo_url, default_branch, created_by FROM engine.projects WHERE id = $1 AND tenant_id = $2`
		args = []any{projectID, tenantID}
	}
	p := &Project{}
	err := r.db.QueryRowContext(ctx, q, args...).Scan(&p.ID, &p.TenantID, &p.CodeRepoURL, &p.DefaultBranch, &p.CreatedBy)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrProjectNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("project: GetByIDInternal: %w", err)
	}
	return p, nil
}
```

Adjust the SELECT list to match the actual `engine.projects` schema — if the columns have different names, use the real ones. The important fields for the adapter are: `id`, `tenant_id`, `code_repo_url` (the HTTPS URL), `default_branch`, `created_by`.

- [ ] **Step 3: Add a `githubTokenReader` wrapper in `main.go`**

Edit `forge-core/cmd/forge-core/main.go`. Near the top of the file (after the imports block, before `main()`), add:

```go
// githubTokenReader satisfies project.TokenReader by delegating to
// auth.Service.GetGitHubToken. Defined here in main so it can bridge
// the project and auth modules without creating a package cycle.
//
// PHASE 1A: used by the workspace LookupAdapter to fetch the project
// creator's GitHub PAT. Phase 1b deletes this wrapper along with the
// entire TokenReader interface when SSH deploy keys replace PATs.
type githubTokenReader struct {
	auth *auth.Service
}

func (r *githubTokenReader) GetGitHubTokenForUser(ctx context.Context, userID int64) (string, error) {
	return r.auth.GetGitHubToken(ctx, userID)
}
```

**Note:** this assumes `auth.Service` has `GetGitHubToken(ctx, userID) (string, error)`. Verify:

```bash
grep -n "GetGitHubToken" forge-core/internal/module/auth/
```

If it doesn't exist, add a minimal version that queries whatever user-token table the auth module uses. If no table exists at all, this becomes an opportunity to create one (add a `engine.user_github_tokens` table with `user_id, token, updated_at` columns, plus a service method and a repository). Don't gold-plate — the simplest scheme that compiles and returns an empty string for missing tokens is acceptable for Phase 1a, as long as operators can INSERT a token manually via SQL for their own testing.

- [ ] **Step 4: Update the `workspaceMgr` construction in `main.go`**

Find line 122 of `main.go`:

```go
	// Workspace manager (local git clones + per-task worktrees)
	workspaceMgr := workspace.NewManager(cfg.WorkspaceRoot)
```

Replace with:

```go
	// Workspace manager — chronos Phase 1a (HTTPS+token auth)
	//
	// Phase 1a uses injectToken inside RealGitRunner for git auth.
	// Phase 1b will add a DeployKeyRepo, ed25519 generation, GitHub
	// deploy-key upload, and SSH auth — at which point this block
	// adds a secrets service and several more dependencies. Until
	// then, the wiring is deliberately minimal.
	workspaceMgr := workspace.NewManager(workspace.Config{
		Root:       cfg.WorkspaceRoot,
		StateRepo:  workspace.NewStateRepo(db),
		GitRunner:  workspace.NewRealGitRunner(),
		PrepClient: workspace.NewPrepClient(cfg.AIWorkerURL),
		// ProjectLookup is wired in after projectService is constructed below.
	})
```

Then find where `projectService` is constructed (around line 126) and the authService is available, and AFTER both exist, add:

```go
	// Back-wire ProjectLookup now that projectService and authService
	// both exist. Chicken-and-egg: workspace wants ProjectLookup,
	// projectService wants workspace for read-only ProjectDir.
	workspaceMgr.SetLookup(project.NewLookupAdapter(
		projectService,
		&githubTokenReader{auth: authService},
	))
```

- [ ] **Step 5: Add imports to `main.go`**

```go
import (
	// ... existing imports ...
	"github.com/shulex/forge/forge-core/internal/module/project"
	// workspace already imported
)
```

If `project` was already imported (likely — for `project.NewService`), no new import needed.

- [ ] **Step 6: Build**

```bash
cd forge-core && go build ./...
```
Expected: clean build. No more compile errors anywhere.

Common issues if this fails:

- `project.Service` doesn't satisfy `project.ProjectReader` because `GetByIDInternal` has a different signature → adjust the interface or the method
- `auth.Service.GetGitHubToken` doesn't exist → add it per Step 3
- `Project` struct field names differ (`CodeRepoURL` vs `RepoURL`, etc.) → use whichever name the project module exports
- `NewLookupAdapter` called with wrong argument types → double-check the interface satisfaction

Fix in the same commit.

- [ ] **Step 7: Add a wiring smoke test**

Create `forge-core/cmd/forge-core/wiring_test.go` (or extend an existing smoke test):

```go
package main

import (
	"testing"

	"github.com/shulex/forge/forge-core/internal/workspace"
)

// TestWorkspaceWiring_Phase1a verifies that the Phase 1a Manager can be
// constructed with all production dependencies (modulo DB — that's an
// integration concern). Catches future regressions in the Config shape
// and dependency types without needing a live database.
func TestWorkspaceWiring_Phase1a(t *testing.T) {
	mgr := workspace.NewManager(workspace.Config{
		Root:       "/tmp/test-ws",
		StateRepo:  nil, // nil is allowed; EnsureReady returns an error
		GitRunner:  workspace.NewRealGitRunner(),
		PrepClient: workspace.NewPrepClient("http://127.0.0.1:0"),
	})
	if mgr == nil {
		t.Fatal("NewManager returned nil")
	}
	if dir := mgr.ProjectDir(1, 2); dir != "/tmp/test-ws/tenant-1/project-2/repo" {
		t.Errorf("ProjectDir: got %s", dir)
	}
}
```

Run it:
```bash
cd forge-core && go test ./cmd/forge-core/... -run TestWorkspaceWiring_Phase1a -v
```

- [ ] **Step 8: Run the full test suite**

```bash
cd forge-core && go test ./... 2>&1 | tail -30
```
Expected: all tests pass (or skip with known reasons). Pay special attention to `./internal/workspace/...` — that's the biggest new surface.

- [ ] **Step 9: Commit**

```bash
git add forge-core/cmd/forge-core/main.go forge-core/cmd/forge-core/wiring_test.go forge-core/internal/config/config.go forge-core/internal/module/project/service.go forge-core/internal/module/project/repository.go 2>/dev/null || true
git commit -m "$(cat <<'EOF'
feat(main): wire chronos Phase 1a workspace module (HTTPS+token)

Config gets AIWorkerURL if missing. main constructs a Manager with:
- StateRepo over *sql.DB
- RealGitRunner (HTTPS+token, no SSH deps)
- PrepClient pointing at ai-worker
- ProjectLookup via project.LookupAdapter (back-wired after
  projectService is constructed — chicken-and-egg between
  projectService wanting ProjectDir and Manager wanting ProjectLookup)

projectService gains a GetByIDInternal method that bypasses RBAC for
internal callers like the LookupAdapter. Repository has the matching
GetByIDInternal on the DAO side.

A small githubTokenReader wrapper in main.go bridges project.TokenReader
to auth.Service.GetGitHubToken — this wrapper is deleted in Phase 1b
when SSH deploy keys replace PATs.

Wiring smoke test in cmd/forge-core/wiring_test.go catches future
regressions in the Config shape without needing a live database.

No deploy-key crypto, no FORGE_SECRETS_MASTER_KEY, no GitHub API
client — all Phase 1b additions per spec §2.9.4.a.
EOF
)"
```

---

### Task 1a.8: End-to-end integration test with a local bare repo

**Files:**
- Create: `forge-core/internal/workspace/ensure_integration_test.go`

**Context:** Final Phase 1a task. The unit tests in Task 1a.4 use fake git runners; this test uses a **real** git binary against a **local bare repo** fixture. It drives `EnsureReady` through the full state machine — first-clone, resync, error-recovery — using the HTTPS+token path with a dummy token that the `file://` bare repo accepts (local filesystem access doesn't actually check credentials).

This is the Phase 1a analog of Round 1 Task 1.13. The differences:

- Uses `EnsureReady` end-to-end (not just `RealGitRunner` methods directly)
- Uses `file://` URLs with a dummy token
- Uses a `memoryLookup` that returns `(fileURL, "dummy-token", "main")`
- Adds a Phase 1a-specific test case: project lookup returns an invalid URL → EnsureReady marks `last_error` as `repo_url_unsupported`

**This test does not verify HTTPS auth against a real GitHub server** — that's what the Phase 7 smoke test does. It verifies the rest of the plumbing: state transitions, git command invocation, stderr capture, success paths, failure paths, concurrent-caller paths.

Requires `FORGE_TEST_DATABASE_URL` set; skip otherwise.

- [ ] **Step 1: Write the integration test**

Create `forge-core/internal/workspace/ensure_integration_test.go`:

```go
package workspace

import (
	"context"
	"database/sql"
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
		t.Skip("git binary not available; skipping e2e test")
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

// seedBareRepoWithSecondCommit adds a second commit to the bare repo
// so resync tests can verify fetch + reset picks up new history.
// The working directory for this commit is fresh — we clone the bare
// repo, add a file, push.
func addSecondCommit(t *testing.T, fileURL, bareDir string) {
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

// newIntegrationFixture wires a real Manager with real StateRepo, real
// RealGitRunner, a stub PrepClient (non-network), and a memoryLookup
// pointing at a file:// URL. Uses a dummy token — the file:// URL
// doesn't exercise HTTPS auth, so injectToken's output is benign.
type integrationFixture struct {
	db      *sql.DB
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
				RepoURL:       fileURL,
				AccessToken:   "dummy-token",
				DefaultBranch: "main",
			},
		},
	}

	// Stub prep client — no network. The prep HTTP call is skipped
	// because we pass nil; EnsureReady treats nil prepClient as
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
		db:      db,
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
	addSecondCommit(t, f.fileURL, f.bareDir)

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
				RepoURL:       "file:///nonexistent/bare/repo/path",
				AccessToken:   "dummy-token",
				DefaultBranch: "main",
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

	// Now fix the lookup and retry — should recover
	f.manager.projectLookup = &memoryLookup{
		projects: map[int64]ProjectInfo{
			303: {
				RepoURL:       f.fileURL,
				AccessToken:   "dummy-token",
				DefaultBranch: "main",
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
				RepoURL:       "gopher://example.com/repo",
				AccessToken:   "dummy-token",
				DefaultBranch: "main",
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
	if row.LastError == nil || !contains(*row.LastError, "repo_url_unsupported") {
		t.Errorf("last_error should contain 'repo_url_unsupported'; got %v", row.LastError)
	}
}

// contains is a small helper so we don't pull in strings.Contains in
// a test-only path. (We do have strings in other tests, but this is
// explicit about what we're checking.)
func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
```

- [ ] **Step 2: Run the tests**

```bash
export FORGE_TEST_DATABASE_URL="postgres://forge:forge@localhost:5432/forge?sslmode=disable"
cd forge-core && go test ./internal/workspace/... -run TestEnsureReady_Integration -v
```

Expected: 4 tests pass:
- `TestEnsureReady_Integration_FirstClone`
- `TestEnsureReady_Integration_Resync`
- `TestEnsureReady_Integration_ErrorRecovery`
- `TestEnsureReady_Integration_UnsupportedURL`

On a machine without git or without `FORGE_TEST_DATABASE_URL`, they skip cleanly.

- [ ] **Step 3: Run the whole workspace package tests to confirm everything still works together**

```bash
cd forge-core && go test ./internal/workspace/... -v 2>&1 | tail -60
```
Expected: all tests pass or skip with documented reasons (DB-integration tests skip if `FORGE_TEST_DATABASE_URL` is unset; git tests skip if git is missing).

- [ ] **Step 4: Commit**

```bash
git add forge-core/internal/workspace/ensure_integration_test.go
git commit -m "$(cat <<'EOF'
test(workspace): integration test for EnsureReady (Phase 1a HTTPS+token path)

Drives the full EnsureReady state machine against a real git binary
and a local bare repo fixture. Four scenarios:

1. First clone — EnsureReady creates row, runs git clone, marks ready
2. Resync — forceSync=true fetches second commit, reset --hard wipes
   local modifications, new files appear in the working tree
3. Error recovery — first call fails on nonexistent URL, row goes to
   error state, second call with fixed URL recovers cleanly
4. Unsupported URL — non-https:// non-file:// scheme surfaces as
   repo_url_unsupported in last_error

Uses file:// URLs with a dummy token so the injectToken code path
runs but the file:// protocol bypasses credential checking. Phase 7's
smoke test covers the real HTTPS+GitHub path.

Requires FORGE_TEST_DATABASE_URL + git on PATH; skips cleanly
otherwise.

Part of chronos Phase 1a.
EOF
)"
```

---

### Phase 1a completion check

Before declaring Phase 1a done and unblocking Phase 5:

- [ ] `go build ./...` in forge-core produces a clean binary
- [ ] `go vet ./...` in forge-core is clean
- [ ] `go test ./internal/workspace/... -v` passes (or skips DB tests cleanly if `FORGE_TEST_DATABASE_URL` unset)
- [ ] `go test ./internal/temporal/activity/... -run TestBuild` passes (migrated caller)
- [ ] `go test ./internal/module/agent/...` passes (migrated caller)
- [ ] `go test ./cmd/forge-core/... -run TestWorkspaceWiring_Phase1a` passes
- [ ] `grep -rn "EnsureClone" forge-core/` returns nothing — the legacy method is deleted
- [ ] `grep -rn "injectToken" forge-core/` returns matches ONLY in `internal/workspace/git.go` and `internal/workspace/git_test.go` — the helper still exists but only in `git.go`
- [ ] `grep -rn "DeployKey\|deploy_key\|project_deploy_keys\|ed25519\|AES-GCM\|GIT_SSH_COMMAND\|toSSHURL" forge-core/` returns nothing under `internal/workspace/` — all deploy-key references are absent in Phase 1a
- [ ] `main.go` wires in `workspace.NewStateRepo`, `workspace.NewRealGitRunner`, `workspace.NewPrepClient`, `project.NewLookupAdapter`
- [ ] `main.go` does NOT reference `secrets.NewService`, `workspace.NewDeployKeyRepo`, `workspace.NewGitHubDeployKeyUploader`, `FORGE_SECRETS_MASTER_KEY`, or `FORGE_GITHUB_API_URL`
- [ ] Branch has **8 new commits** from this phase (one per task)
- [ ] A dev-mode `docker compose up forge-core` starts cleanly without any new env var requirements beyond what Phase 0 already demanded

## Phase 1a outputs unlock

**What Phase 5 and downstream phases can now do:**

- **Phase 5 agent service** has a working `workspace.Manager.EnsureReady(ctx, tenantID, projectID, forceSync)` call it can depend on. The old `os.Stat(.git)` probe is gone; sessions either succeed with a ready workspace or fail fast with a visible `"workspace not ready: ..."` error.
- **Phase 5 ai-worker endpoint** `POST /api/workspace/prep` has a Go client in forge-core calling it — Phase 5 adds the handler on the Python side and the existing `PrepClient` starts talking to real code.
- **Phase 7 smoke test** has a real workspace manager it can exercise over a real GitHub repo using HTTPS+token (solo-dev/internal testing path).
- **Worker activity migration** is complete — both `build_activities.go` and `devops_activities.go` use `EnsureReady` instead of the legacy `EnsureClone`. No more `repoURL`/`token`/`branch` plumbing through activity inputs.

**What is still missing (deferred to Phase 1b):**

- **SSH deploy keys.** Phase 1a's HTTPS+token auth works for GitHub PATs but doesn't match the §3.8 prompt-injection containment model. Phase 1b adds ed25519 generation, AES-GCM encryption via a secrets service, GitHub deploy-key upload, `engine.project_deploy_keys` table, and swaps `RealGitRunner` to SSH.
- **`ProjectLookup` breaking change.** Phase 1b rewrites `ProjectInfo` to drop `AccessToken` and rename `RepoURL` to `SSHURL`. This is a hard-cutover within the monorepo — the adapter, the interface, and every caller migrate in one commit. Phase 1a's `ProjectLookup` is the transitional shape.
- **Key rotation.** Out of scope for Phase 1b too — deploy keys are generated once and reused. Rotation is a follow-up project.
- **Public deployment.** Per spec §2.9.4.d, public deployment is blocked until Phase 1b lands. Phase 1a is solo-dev / internal testing only.

**When Phase 1b is complete, the final state:**

- `injectToken` is deleted (Phase 1b's first task deletes `workspace/git.go` wholesale and replaces it with the SSH variant per spec §2.9.4.c)
- `manager.go` is unchanged (the state machine and callers were already migrated in 1a)
- The new `engine.project_deploy_keys` table exists with one row per project
- `secrets.Service` is wired into `main.go`
- `ProjectLookup` returns `(SSHURL, DefaultBranch)` with no token
- The HTTPS+token failure mode row in spec §3.12 (`github_auth_failed`) is deleted and replaced with the deploy-key upload failure row
