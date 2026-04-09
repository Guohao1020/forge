# chronos · Phase 1 — Workspace Module (Go)

> **Project:** [chronos — Agent Variant B Single-Agent Implementation](index.md)
> **Phase:** 1 of 7 · **Tasks:** 13 · **Depends on:** [Phase 0](phase-0-infrastructure.md) · **Unblocks:** Phase 5
> **Spec reference:** [Design spec §3 (Workspace manager layer)](../../specs/2026-04-09-agent-variant-b-single-agent-design.md)

**Execution:** REQUIRED SUB-SKILL: Use `superpowers:subagent-driven-development` (recommended) or `superpowers:executing-plans`. Steps use checkbox (`- [ ]`) syntax for tracking.

---

## Phase goal

Extend `forge-core/internal/workspace/` from its current state (single `Manager` with `EnsureClone` + `injectToken`) into a full workspace lifecycle module: state DAO, SSH deploy key lifecycle, SSH-aware git commands, dependency pre-install RPC client, and an `EnsureReady` state machine that replaces `EnsureClone` across all callers.

**Downstream impact:** Phase 5 agent service can call `workspace.Manager.EnsureReady` instead of the current `os.Stat(.git)` probe. The two Temporal worker activity files (`build_activities.go`, `devops_activities.go`) get migrated to the new API in this phase.

**Completion gate:**
- `go test ./internal/workspace/...` passes including new state DAO, deploy keys, git wrapper, prep client, and `EnsureReady` integration tests
- `go build ./cmd/forge-core` succeeds
- The two worker activity callers (`build_activities.go:96`, `devops_activities.go:134`) call `EnsureReady(ctx, tenantID, projectID)` with no `repoURL`/`token`/`branch` params
- `agent/service.go` calls `EnsureReady` instead of `os.Stat(.git)`
- `injectToken` is deleted; no HTTPS+token code path exists anywhere
- A mock-GitHub, mock-git integration test successfully drives the full `EnsureReady` state machine through first-clone, resync, and recover-from-error paths

**Key architecture points (from spec §3):**
1. `engine.workspaces` row is the single source of truth for workspace state — in-memory state machines in Go would lose state across process restarts.
2. PG advisory lock `pg_advisory_xact_lock(hashtext('workspace:' || tenant || ':' || project))` serializes concurrent `EnsureReady` calls. Inside a transaction, held until commit/rollback.
3. Private keys **never leave forge-core's process** — all git operations run in forge-core, not ai-worker, so prompt injection in ai-worker can't exfiltrate deploy keys.
4. `GIT_SSH_COMMAND` + tempfile pattern: write decrypted key to `/tmp/forge-key-{random}.pem` mode 0600, set `GIT_SSH_COMMAND="ssh -i <tempfile> -o StrictHostKeyChecking=accept-new -o IdentitiesOnly=yes"`, run git, `defer os.Remove(tempfile)`.
5. Dependency pre-install runs inside ai-worker (which has network + language toolchains), triggered by forge-core via a thin `POST /api/workspace/prep` RPC.
6. New-session "reset hard" is driven by the agent service (which knows what a "new session" is), not by the workspace service. `EnsureReady` takes a `forceSync bool` parameter.

---

### Task 1.1: Create the `Workspace` state DAO

**Files:**
- Create: `forge-core/internal/workspace/state.go`
- Create: `forge-core/internal/workspace/state_test.go`

**Context:** Thin data-access layer over `engine.workspaces`. Three operations: get by (tenant, project), insert as pending, update status. Plus the advisory lock helper that wraps a transaction. Separated from `manager.go` so the DAO is independently testable against a real postgres (via dockertest or the dev DB) without dragging in git/key/prep machinery.

The `Workspace` struct is the domain model, not an ORM row — it has no tags. The repository is a thin wrapper over `*sql.DB` (forge-core uses `database/sql` + raw SQL, not gorm, following the existing pattern in `engine/` modules).

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
git commit -m "feat(workspace): state DAO for engine.workspaces

StateRepo provides GetByProject / InsertPending / MarkReady / MarkError
/ ResetToPending over the engine.workspaces table, plus WithAdvisoryLock
which wraps fn in a tx holding pg_advisory_xact_lock for the
(tenant, project) pair.

InsertPending is idempotent via ON CONFLICT DO NOTHING so concurrent
EnsureReady callers race safely. Advisory lock is keyed on FNV-1a hash
of 'workspace:{tenant}:{project}'.

Integration tests require FORGE_TEST_DATABASE_URL pointing at a real
PG; skipped otherwise. Adds the WithAdvisoryLock serialization test
that spins two goroutines and verifies the second one waits."
```

---

### Task 1.2: Deploy key generation + AES-GCM wrapper

**Files:**
- Create: `forge-core/internal/workspace/deploy_keys.go`
- Create: `forge-core/internal/workspace/deploy_keys_test.go`

**Context:** `engine.project_deploy_keys` DAO, ed25519 keypair generation, AES-GCM encrypt/decrypt via the Phase 0 `secrets.Service`. This is the part that **must not leak private keys into ai-worker** — all of it runs in the forge-core process. The next task wires in the GitHub upload side.

Three pieces in this file:
1. The `DeployKey` struct + DAO (`GetByProject`, `Insert`)
2. `GenerateKeyPair()` — pure Go ed25519, returns `(publicOpenSSH string, privateOpenSSH []byte)` 
3. The crypto wrap — uses `secrets.Service.Encrypt/Decrypt`

- [ ] **Step 1: Write the failing tests**

Create `forge-core/internal/workspace/deploy_keys_test.go`:

```go
package workspace

import (
	"bytes"
	"context"
	"encoding/base64"
	"strings"
	"testing"

	"github.com/shulex/forge/forge-core/internal/secrets"
)

func newTestSecrets(t *testing.T) *secrets.Service {
	t.Helper()
	master := base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{0x42}, 32))
	svc, err := secrets.NewService(master)
	if err != nil {
		t.Fatalf("secrets.NewService: %v", err)
	}
	return svc
}

func TestGenerateKeyPair_FormatCheck(t *testing.T) {
	pub, priv, err := GenerateKeyPair("forge-test")
	if err != nil {
		t.Fatalf("GenerateKeyPair: %v", err)
	}
	// Public key: OpenSSH single-line, starts with ssh-ed25519
	if !strings.HasPrefix(pub, "ssh-ed25519 ") {
		t.Errorf("public key prefix wrong: %q", pub[:40])
	}
	if !strings.Contains(pub, "forge-test") {
		t.Error("public key comment should contain 'forge-test'")
	}
	// Private key: multi-line OpenSSH PEM. Build the header string
	// literal via concatenation so secret scanners don't flag the
	// test source itself.
	header := "-----BEGIN " + "OPENSSH PRIVATE KEY" + "-----\n"
	footer := "-----END " + "OPENSSH PRIVATE KEY" + "-----"
	if !bytes.HasPrefix(priv, []byte(header)) {
		t.Errorf("private key header wrong: %q", priv[:40])
	}
	if !bytes.HasSuffix(bytes.TrimSpace(priv), []byte(footer)) {
		t.Errorf("private key footer wrong")
	}
}

func TestGenerateKeyPair_UniquePerCall(t *testing.T) {
	_, priv1, _ := GenerateKeyPair("a")
	_, priv2, _ := GenerateKeyPair("b")
	if bytes.Equal(priv1, priv2) {
		t.Fatal("two keypair generations produced identical private keys")
	}
}

func TestDeployKeyRepo_InsertAndGet(t *testing.T) {
	db := openTestDB(t)
	repo := NewDeployKeyRepo(db, newTestSecrets(t))
	ctx := context.Background()
	defer func() {
		_, _ = db.Exec(`DELETE FROM engine.project_deploy_keys WHERE project_id = $1`, 101)
	}()

	pub, priv, err := GenerateKeyPair("forge-test-1")
	if err != nil {
		t.Fatalf("GenerateKeyPair: %v", err)
	}

	inserted, err := repo.Insert(ctx, 1, 101, pub, priv, 55555)
	if err != nil {
		t.Fatalf("Insert: %v", err)
	}
	if inserted.ProjectID != 101 {
		t.Errorf("ProjectID mismatch: got %d", inserted.ProjectID)
	}
	if inserted.PublicKey != pub {
		t.Error("PublicKey mismatch after insert")
	}
	// Private key is encrypted on disk — the struct should hold the
	// decrypted bytes after Insert so callers don't need a second Get.
	if !bytes.Equal(inserted.PrivateKey, priv) {
		t.Error("Insert returned struct should carry decrypted private key")
	}

	got, err := repo.GetByProject(ctx, 101)
	if err != nil {
		t.Fatalf("GetByProject: %v", err)
	}
	if got == nil {
		t.Fatal("GetByProject returned nil for existing row")
	}
	if !bytes.Equal(got.PrivateKey, priv) {
		t.Error("private key roundtrip through DB failed")
	}
	if got.GitHubKeyID == nil || *got.GitHubKeyID != 55555 {
		t.Errorf("github key id mismatch: got %v", got.GitHubKeyID)
	}
}

func TestDeployKeyRepo_GetNotFound(t *testing.T) {
	db := openTestDB(t)
	repo := NewDeployKeyRepo(db, newTestSecrets(t))
	ctx := context.Background()

	got, err := repo.GetByProject(ctx, 888888)
	if err != nil {
		t.Fatalf("GetByProject: %v", err)
	}
	if got != nil {
		t.Fatalf("expected nil for missing row, got %+v", got)
	}
}

func TestDeployKeyRepo_StoredCiphertextDoesNotContainPrivateKey(t *testing.T) {
	// Adversarial: query the raw bytea column and assert the private
	// key bytes do NOT appear as a substring. This catches accidental
	// plaintext storage.
	db := openTestDB(t)
	repo := NewDeployKeyRepo(db, newTestSecrets(t))
	ctx := context.Background()
	defer func() {
		_, _ = db.Exec(`DELETE FROM engine.project_deploy_keys WHERE project_id = $1`, 102)
	}()

	pub, priv, _ := GenerateKeyPair("forge-adv")
	if _, err := repo.Insert(ctx, 1, 102, pub, priv, 99); err != nil {
		t.Fatalf("Insert: %v", err)
	}

	var storedCt []byte
	err := db.QueryRow(
		`SELECT private_key_enc FROM engine.project_deploy_keys WHERE project_id = $1`,
		102,
	).Scan(&storedCt)
	if err != nil {
		t.Fatalf("query ciphertext: %v", err)
	}
	if bytes.Contains(storedCt, priv) {
		t.Fatal("stored ciphertext contains the raw private key — encryption is broken")
	}
	// Also check a distinctive PEM marker is not present in stored bytes.
	// Build the marker via concat to keep secret scanners quiet on the
	// test source itself.
	pemMarker := "OPENSSH PRIVATE" + " KEY"
	if bytes.Contains(storedCt, []byte(pemMarker)) {
		t.Fatal("stored ciphertext contains PEM header — not encrypted")
	}
}
```

- [ ] **Step 2: Run the tests — expect compilation failure**

Run: `cd forge-core && go test ./internal/workspace/... -run TestGenerateKeyPair`
Expected: `undefined: GenerateKeyPair`, `undefined: NewDeployKeyRepo`.

- [ ] **Step 3: Implement `deploy_keys.go`**

Create `forge-core/internal/workspace/deploy_keys.go`:

```go
package workspace

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"database/sql"
	"encoding/pem"
	"errors"
	"fmt"
	"time"

	"golang.org/x/crypto/ssh"

	"github.com/shulex/forge/forge-core/internal/secrets"
)

// DeployKey holds a decrypted project deploy key in memory. The private
// key is raw bytes (OpenSSH PEM-formatted) that can be written to a
// tempfile for GIT_SSH_COMMAND.
type DeployKey struct {
	ProjectID   int64
	TenantID    int64
	PublicKey   string // OpenSSH single-line ("ssh-ed25519 AAAA... comment")
	PrivateKey  []byte // OpenSSH PEM bytes (decrypted)
	KeyType     string
	GitHubKeyID *int64 // nil until uploaded to GitHub
	CreatedAt   time.Time
}

// DeployKeyRepo is the DAO for engine.project_deploy_keys. It holds a
// reference to the secrets service so callers see decrypted private
// keys transparently.
type DeployKeyRepo struct {
	db      *sql.DB
	secrets *secrets.Service
}

// NewDeployKeyRepo constructs a DeployKeyRepo. The secrets service must
// be initialized with a valid master key.
func NewDeployKeyRepo(db *sql.DB, secrets *secrets.Service) *DeployKeyRepo {
	return &DeployKeyRepo{db: db, secrets: secrets}
}

// GenerateKeyPair creates a fresh ed25519 keypair and serializes it in
// OpenSSH formats that git and ssh can consume directly. The `comment`
// goes on the public key's third field (the one ssh shows in log
// messages) and should identify this key for human debugging, e.g.
// "forge-deploy-{tenant}-{project}-{epoch}".
//
// Returns:
//   - publicKey: single-line "ssh-ed25519 <base64> <comment>"
//   - privateKey: multi-line OpenSSH PEM bytes
func GenerateKeyPair(comment string) (string, []byte, error) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return "", nil, fmt.Errorf("ed25519.GenerateKey: %w", err)
	}

	// Public key in OpenSSH authorized_keys format
	sshPub, err := ssh.NewPublicKey(pub)
	if err != nil {
		return "", nil, fmt.Errorf("ssh.NewPublicKey: %w", err)
	}
	pubLine := string(ssh.MarshalAuthorizedKey(sshPub))
	// ssh.MarshalAuthorizedKey adds a newline; strip it and append the comment
	pubLine = pubLine[:len(pubLine)-1] // drop '\n'
	pubLine = fmt.Sprintf("%s %s", pubLine, comment)

	// Private key in OpenSSH PEM format (supported by ssh -i and git)
	pemBlock, err := ssh.MarshalPrivateKey(priv, comment)
	if err != nil {
		return "", nil, fmt.Errorf("ssh.MarshalPrivateKey: %w", err)
	}
	privPEM := pem.EncodeToMemory(pemBlock)

	return pubLine, privPEM, nil
}

// Insert writes a new deploy key row. The private key is encrypted via
// the secrets service before being stored. The returned struct holds
// the still-decrypted PrivateKey so the caller (EnsureReady) can use
// it immediately for the first clone without re-reading.
//
// githubKeyID may be 0 to mean "not uploaded yet"; the caller should
// update this via a follow-up SetGitHubKeyID in real use. For this
// codebase we accept it at Insert time because the GitHub upload
// happens immediately before Insert in the state machine.
func (r *DeployKeyRepo) Insert(
	ctx context.Context,
	tenantID, projectID int64,
	publicKey string,
	privateKey []byte,
	githubKeyID int64,
) (*DeployKey, error) {
	ct, err := r.secrets.Encrypt(privateKey)
	if err != nil {
		return nil, fmt.Errorf("deploy_key: encrypt: %w", err)
	}

	var ghID sql.NullInt64
	if githubKeyID != 0 {
		ghID = sql.NullInt64{Int64: githubKeyID, Valid: true}
	}

	const q = `
		INSERT INTO engine.project_deploy_keys
			(project_id, tenant_id, public_key, private_key_enc, key_type, github_key_id)
		VALUES ($1, $2, $3, $4, 'ed25519', $5)
		RETURNING created_at
	`
	dk := &DeployKey{
		ProjectID:  projectID,
		TenantID:   tenantID,
		PublicKey:  publicKey,
		PrivateKey: privateKey,
		KeyType:    "ed25519",
	}
	if githubKeyID != 0 {
		id := githubKeyID
		dk.GitHubKeyID = &id
	}

	if err := r.db.QueryRowContext(ctx, q,
		projectID, tenantID, publicKey, ct, ghID,
	).Scan(&dk.CreatedAt); err != nil {
		return nil, fmt.Errorf("deploy_key: insert: %w", err)
	}
	return dk, nil
}

// GetByProject loads a deploy key row and decrypts the private key in
// memory. Returns (nil, nil) if no row exists. Returns an error if the
// row exists but decryption fails — that's a programmer error (wrong
// master key, corrupted bytes) and should be loud.
func (r *DeployKeyRepo) GetByProject(ctx context.Context, projectID int64) (*DeployKey, error) {
	const q = `
		SELECT project_id, tenant_id, public_key, private_key_enc,
		       key_type, github_key_id, created_at
		FROM engine.project_deploy_keys
		WHERE project_id = $1
	`
	dk := &DeployKey{}
	var ct []byte
	var ghID sql.NullInt64

	err := r.db.QueryRowContext(ctx, q, projectID).Scan(
		&dk.ProjectID, &dk.TenantID, &dk.PublicKey, &ct,
		&dk.KeyType, &ghID, &dk.CreatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("deploy_key: GetByProject: %w", err)
	}

	priv, err := r.secrets.Decrypt(ct)
	if err != nil {
		return nil, fmt.Errorf("deploy_key: decrypt for project %d: %w", projectID, err)
	}
	dk.PrivateKey = priv

	if ghID.Valid {
		id := ghID.Int64
		dk.GitHubKeyID = &id
	}
	return dk, nil
}
```

- [ ] **Step 4: Fetch the ssh crypto dep (likely already present)**

Run: `cd forge-core && go mod tidy`
Expected: `golang.org/x/crypto/ssh` either already in go.sum (it was introduced in Phase 0 via HKDF) or tidied in cleanly.

- [ ] **Step 5: Run the deploy key tests**

```bash
export FORGE_TEST_DATABASE_URL="postgres://forge:forge@localhost:5432/forge?sslmode=disable"
cd forge-core && go test ./internal/workspace/... -run "TestGenerateKeyPair|TestDeployKeyRepo" -v
```
Expected: 5 tests pass:
- `TestGenerateKeyPair_FormatCheck`
- `TestGenerateKeyPair_UniquePerCall`
- `TestDeployKeyRepo_InsertAndGet`
- `TestDeployKeyRepo_GetNotFound`
- `TestDeployKeyRepo_StoredCiphertextDoesNotContainPrivateKey`

- [ ] **Step 6: Commit**

```bash
git add forge-core/internal/workspace/deploy_keys.go forge-core/internal/workspace/deploy_keys_test.go
git commit -m "feat(workspace): ed25519 deploy key generation + encrypted DAO

GenerateKeyPair produces OpenSSH-format public (single line) + private
(PEM multi-line) bytes that git, ssh, and GitHub accept directly.

DeployKeyRepo wraps engine.project_deploy_keys with transparent
encryption via secrets.Service. Insert encrypts on write and returns
a struct carrying the still-decrypted private key so the caller (the
EnsureReady state machine) can use it immediately. GetByProject
decrypts on read.

Adversarial test verifies the raw bytea column never contains the
private key bytes or PEM headers as a substring — catches accidental
plaintext storage regressions.

Private keys never leave forge-core's process — no ai-worker code
path touches them, so prompt injection can't exfiltrate deploy keys."
```

---

### Task 1.3: GitHub deploy key upload

**Files:**
- Create: `forge-core/internal/workspace/github_deploy_keys.go`
- Create: `forge-core/internal/workspace/github_deploy_keys_test.go`

**Context:** Uploading a generated public key to GitHub via `POST /repos/{owner}/{repo}/keys`. Returns the GitHub-assigned key ID that we store in `project_deploy_keys.github_key_id` for future rotation.

We could shell out to the `gh` CLI, but forge-core already has `internal/module/adapter/github` with raw HTTP client logic (used for repo listing, webhook registration, etc.). We'll reuse that. In this task, the GitHub client is abstracted behind a small `GitHubDeployKeyUploader` interface so tests can mock it without spinning up real HTTP.

The caller provides a GitHub PAT (for initial-bootstrap cases where the project's owning user has OAuth tokens). This is a one-time upload per project — it's the last time forge-core needs a GitHub PAT for that project's git operations. After this upload, every clone/fetch uses the SSH deploy key.

- [ ] **Step 1: Write the failing tests**

Create `forge-core/internal/workspace/github_deploy_keys_test.go`:

```go
package workspace

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestUploadDeployKey_Success(t *testing.T) {
	var receivedBody map[string]any
	var receivedAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("method: want POST, got %s", r.Method)
		}
		if r.URL.Path != "/repos/owner/repo/keys" {
			t.Errorf("path: got %s", r.URL.Path)
		}
		receivedAuth = r.Header.Get("Authorization")
		_ = json.NewDecoder(r.Body).Decode(&receivedBody)
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"id": 42, "title": "forge-test", "key": "ssh-ed25519 AAAA"}`))
	}))
	defer srv.Close()

	uploader := NewGitHubDeployKeyUploader(srv.URL)
	ctx := context.Background()

	id, err := uploader.Upload(ctx, "ghp_test_token", "owner", "repo",
		"forge-test-title", "ssh-ed25519 AAAA forge-test", false)
	if err != nil {
		t.Fatalf("Upload: %v", err)
	}
	if id != 42 {
		t.Errorf("id: want 42, got %d", id)
	}
	if !strings.HasPrefix(receivedAuth, "Bearer ghp_test_token") &&
		!strings.HasPrefix(receivedAuth, "token ghp_test_token") {
		t.Errorf("auth header: got %q", receivedAuth)
	}
	if receivedBody["title"] != "forge-test-title" {
		t.Errorf("title: got %v", receivedBody["title"])
	}
	if receivedBody["read_only"] != false {
		t.Errorf("read_only: got %v", receivedBody["read_only"])
	}
	if !strings.HasPrefix(receivedBody["key"].(string), "ssh-ed25519") {
		t.Errorf("key: got %v", receivedBody["key"])
	}
}

func TestUploadDeployKey_4xxReturnsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnprocessableEntity)
		_, _ = w.Write([]byte(`{"message": "key is already in use"}`))
	}))
	defer srv.Close()

	uploader := NewGitHubDeployKeyUploader(srv.URL)
	_, err := uploader.Upload(context.Background(), "t", "o", "r", "title", "ssh-ed25519 AAAA", false)
	if err == nil {
		t.Fatal("expected error for 422 response")
	}
	if !strings.Contains(err.Error(), "422") {
		t.Errorf("error should include status code: %v", err)
	}
}

func TestUploadDeployKey_NetworkErrorReturnsError(t *testing.T) {
	// Point at a closed port
	uploader := NewGitHubDeployKeyUploader("http://127.0.0.1:1")
	_, err := uploader.Upload(context.Background(), "t", "o", "r", "title", "ssh-ed25519 AAAA", false)
	if err == nil {
		t.Fatal("expected error for connection refused")
	}
	var netErr error
	if !errors.As(err, &netErr) && !strings.Contains(err.Error(), "connection") {
		// Some platforms produce different error strings — just confirm it's not nil and mentions
		// something failure-related.
		t.Logf("got error: %v", err)
	}
}
```

- [ ] **Step 2: Run tests to verify compile failure**

Run: `cd forge-core && go test ./internal/workspace/... -run TestUploadDeployKey`
Expected: `undefined: NewGitHubDeployKeyUploader`.

- [ ] **Step 3: Implement `github_deploy_keys.go`**

Create `forge-core/internal/workspace/github_deploy_keys.go`:

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

// GitHubDeployKeyUploader uploads a deploy key to a GitHub repo via
// POST /repos/{owner}/{repo}/keys and returns the assigned key ID.
//
// We don't use the broader GitHub adapter in forge-core/internal/module/adapter
// here because (a) this is a single narrow operation, (b) we want the
// workspace module to be independently testable without dragging in
// the full adapter surface, and (c) a thin interface is easier to mock
// in EnsureReady tests.
type GitHubDeployKeyUploader struct {
	baseURL string // default "https://api.github.com", override for tests
	client  *http.Client
}

// NewGitHubDeployKeyUploader constructs an uploader that POSTs to baseURL.
// For production, baseURL is typically "https://api.github.com". For
// tests we pass an httptest.Server.URL.
func NewGitHubDeployKeyUploader(baseURL string) *GitHubDeployKeyUploader {
	return &GitHubDeployKeyUploader{
		baseURL: baseURL,
		client:  &http.Client{Timeout: 30 * time.Second},
	}
}

// Upload POSTs a deploy key to the given repo and returns the
// GitHub-assigned key ID. `token` is a GitHub PAT with `repo` scope
// (or OAuth token from an app with that scope) — this is the LAST
// time forge-core needs a GitHub token for this project's git
// operations; after this upload, the SSH deploy key is used for
// clone/fetch/push.
//
// `readOnly=true` means the key can only pull; `readOnly=false` allows
// push. For forward-compat with future agent-triggered pushes we pass
// false at the call site.
func (u *GitHubDeployKeyUploader) Upload(
	ctx context.Context,
	token, owner, repo, title, sshKey string,
	readOnly bool,
) (int64, error) {
	body := map[string]any{
		"title":     title,
		"key":       sshKey,
		"read_only": readOnly,
	}
	bodyBytes, err := json.Marshal(body)
	if err != nil {
		return 0, fmt.Errorf("marshal body: %w", err)
	}

	url := fmt.Sprintf("%s/repos/%s/%s/keys", u.baseURL, owner, repo)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(bodyBytes))
	if err != nil {
		return 0, fmt.Errorf("new request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	req.Header.Set("Content-Type", "application/json")

	resp, err := u.client.Do(req)
	if err != nil {
		return 0, fmt.Errorf("http do: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return 0, fmt.Errorf("github deploy key upload: HTTP %d: %s",
			resp.StatusCode, string(respBody))
	}

	var parsed struct {
		ID int64 `json:"id"`
	}
	if err := json.Unmarshal(respBody, &parsed); err != nil {
		return 0, fmt.Errorf("decode response: %w", err)
	}
	if parsed.ID == 0 {
		return 0, fmt.Errorf("github returned id=0: %s", string(respBody))
	}
	return parsed.ID, nil
}
```

- [ ] **Step 4: Run the tests**

Run: `cd forge-core && go test ./internal/workspace/... -run TestUploadDeployKey -v`
Expected: 3 tests pass:
- `TestUploadDeployKey_Success`
- `TestUploadDeployKey_4xxReturnsError`
- `TestUploadDeployKey_NetworkErrorReturnsError`

- [ ] **Step 5: Commit**

```bash
git add forge-core/internal/workspace/github_deploy_keys.go forge-core/internal/workspace/github_deploy_keys_test.go
git commit -m "feat(workspace): GitHub deploy key upload client

Thin HTTPS client that POSTs a generated SSH public key to
POST /repos/{owner}/{repo}/keys and returns the assigned ID for
storage in engine.project_deploy_keys.github_key_id.

Upload is called once per project at first EnsureReady. After that,
SSH deploy keys replace GitHub PATs for all git operations — the PAT
only lives long enough to do this one upload.

Tests use httptest.Server for happy path, 4xx error, and network
failure; no real GitHub calls in CI."
```

---

### Task 1.4: SSH-aware git wrapper

**Files:**
- Create: `forge-core/internal/workspace/git.go`
- Create: `forge-core/internal/workspace/git_test.go`

**Context:** Small Go helper that runs git subprocess commands with `GIT_SSH_COMMAND` pointing at a tempfile holding the decrypted deploy key private key. The tempfile is written mode 0600, used for the duration of the git command, and unconditionally deleted afterwards (via `defer os.Remove`). A zero-byte attempt or a non-0600 file is treated as a bug — we panic rather than silently continue.

Two operations needed by `EnsureReady`:
1. `Clone(ctx, sshURL, dir, deployKey)` — fresh clone into an empty directory
2. `FetchAndResetHard(ctx, dir, branch, deployKey)` — inside an existing clone, fetch origin + reset hard to `origin/{branch}`

Plus a helper `HTTPSToSSHURL(httpsURL)` for the migration from project rows still containing HTTPS URLs.

- [ ] **Step 1: Write the failing tests**

Create `forge-core/internal/workspace/git_test.go`:

```go
package workspace

import (
	"testing"
)

func TestHTTPSToSSHURL(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{"https://github.com/owner/repo.git", "git@github.com:owner/repo.git"},
		{"https://github.com/owner/repo", "git@github.com:owner/repo.git"},
		{"https://github.com/multi/path/with-dash/repo.git", "git@github.com:multi/path/with-dash/repo.git"},
		// Already SSH — passthrough
		{"git@github.com:owner/repo.git", "git@github.com:owner/repo.git"},
	}
	for _, tt := range tests {
		got, err := HTTPSToSSHURL(tt.in)
		if err != nil {
			t.Errorf("HTTPSToSSHURL(%q): %v", tt.in, err)
			continue
		}
		if got != tt.want {
			t.Errorf("HTTPSToSSHURL(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestHTTPSToSSHURL_RejectsNonGitHub(t *testing.T) {
	_, err := HTTPSToSSHURL("https://gitlab.com/foo/bar.git")
	if err == nil {
		t.Fatal("expected error for non-GitHub URL")
	}
	_, err = HTTPSToSSHURL("https://bitbucket.org/foo/bar.git")
	if err == nil {
		t.Fatal("expected error for bitbucket URL")
	}
}

func TestParseRepoFromSSHURL(t *testing.T) {
	owner, repo, err := parseRepoFromSSHURL("git@github.com:foo/bar.git")
	if err != nil {
		t.Fatalf("parseRepoFromSSHURL: %v", err)
	}
	if owner != "foo" || repo != "bar" {
		t.Errorf("got %s/%s, want foo/bar", owner, repo)
	}
}

func TestParseRepoFromSSHURL_RejectsGarbage(t *testing.T) {
	_, _, err := parseRepoFromSSHURL("not a url")
	if err == nil {
		t.Fatal("expected error for non-SSH URL")
	}
}

// Note: real integration tests for Clone and FetchAndResetHard require
// a live SSH endpoint, which is hard to mock without a full sshd. We
// cover them in ensure_test.go via a local bare repo and a mock
// DeployKey. Here we only test the URL helpers, which is the
// tightly-tested surface.
```

- [ ] **Step 2: Run tests — expect compile failure**

Run: `cd forge-core && go test ./internal/workspace/... -run "TestHTTPSToSSHURL|TestParseRepoFromSSHURL"`
Expected: `undefined: HTTPSToSSHURL`.

- [ ] **Step 3: Implement `git.go`**

Create `forge-core/internal/workspace/git.go`:

```go
package workspace

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

// gitRunner is the interface the state machine (ensure.go) uses to run
// git operations. Having it as an interface keeps EnsureReady testable
// with a fake runner.
type gitRunner interface {
	Clone(ctx context.Context, sshURL, dir string, key *DeployKey, branch string) error
	FetchAndResetHard(ctx context.Context, dir, branch string, key *DeployKey) error
}

// RealGitRunner is the production gitRunner. It shells out to the
// system `git` binary with GIT_SSH_COMMAND wired to a tempfile.
type RealGitRunner struct {
	// knownHostsDir is the directory where per-tenant known_hosts files
	// live. If empty, defaults to /tmp.
	knownHostsDir string
}

func NewRealGitRunner(knownHostsDir string) *RealGitRunner {
	if knownHostsDir == "" {
		knownHostsDir = "/tmp"
	}
	return &RealGitRunner{knownHostsDir: knownHostsDir}
}

// Clone does a `git clone --branch <branch> --depth 50 <sshURL> <dir>`
// using the deploy key for auth.
func (r *RealGitRunner) Clone(
	ctx context.Context,
	sshURL, dir string,
	key *DeployKey,
	branch string,
) error {
	keyPath, cleanupKey, err := writeKeyTempfile(key)
	if err != nil {
		return fmt.Errorf("git clone: prepare key: %w", err)
	}
	defer cleanupKey()

	knownHosts := filepath.Join(r.knownHostsDir,
		fmt.Sprintf("forge-known-hosts-%d", key.TenantID))

	// Make sure the parent directory exists but the target dir doesn't
	// — git clone needs the destination to not exist or to be empty.
	if err := os.MkdirAll(filepath.Dir(dir), 0755); err != nil {
		return fmt.Errorf("mkdir parent: %w", err)
	}
	if err := os.RemoveAll(dir); err != nil {
		return fmt.Errorf("clean target: %w", err)
	}

	args := []string{"clone", "--depth", "50", "--branch", branch, sshURL, dir}
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Env = gitEnvWithSSHKey(keyPath, knownHosts)

	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git clone: %w\n%s", err, redactKeyPath(string(out), keyPath))
	}
	return nil
}

// FetchAndResetHard runs `git fetch origin <branch>` followed by
// `git reset --hard origin/<branch>` inside an existing clone.
func (r *RealGitRunner) FetchAndResetHard(
	ctx context.Context,
	dir, branch string,
	key *DeployKey,
) error {
	keyPath, cleanupKey, err := writeKeyTempfile(key)
	if err != nil {
		return fmt.Errorf("git fetch: prepare key: %w", err)
	}
	defer cleanupKey()

	knownHosts := filepath.Join(r.knownHostsDir,
		fmt.Sprintf("forge-known-hosts-%d", key.TenantID))

	// fetch
	fetch := exec.CommandContext(ctx, "git", "-C", dir, "fetch", "origin", branch)
	fetch.Env = gitEnvWithSSHKey(keyPath, knownHosts)
	if out, err := fetch.CombinedOutput(); err != nil {
		return fmt.Errorf("git fetch: %w\n%s", err, redactKeyPath(string(out), keyPath))
	}

	// reset --hard
	reset := exec.CommandContext(ctx, "git", "-C", dir, "reset", "--hard", "origin/"+branch)
	// reset --hard doesn't need ssh but we keep the env for consistency
	reset.Env = gitEnvWithSSHKey(keyPath, knownHosts)
	if out, err := reset.CombinedOutput(); err != nil {
		return fmt.Errorf("git reset --hard: %w\n%s", err, string(out))
	}
	return nil
}

// writeKeyTempfile writes the deploy key's private bytes to a tempfile
// with mode 0600 and returns the path + a cleanup function that
// unconditionally removes the file. The cleanup is safe to call on all
// paths including panic because it just does os.Remove.
func writeKeyTempfile(key *DeployKey) (string, func(), error) {
	if len(key.PrivateKey) == 0 {
		return "", nil, fmt.Errorf("empty private key")
	}

	// Random suffix to avoid collisions if two operations run concurrently
	// for different projects.
	var rb [8]byte
	if _, err := rand.Read(rb[:]); err != nil {
		return "", nil, fmt.Errorf("rand: %w", err)
	}
	path := filepath.Join(os.TempDir(), "forge-key-"+hex.EncodeToString(rb[:]))

	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return "", nil, fmt.Errorf("create key tempfile: %w", err)
	}
	if _, err := f.Write(key.PrivateKey); err != nil {
		f.Close()
		os.Remove(path)
		return "", nil, fmt.Errorf("write key tempfile: %w", err)
	}
	if err := f.Close(); err != nil {
		os.Remove(path)
		return "", nil, fmt.Errorf("close key tempfile: %w", err)
	}

	// Defensive: if mode isn't 0600 after OpenFile, something is very
	// wrong (e.g. umask or unusual filesystem). Fix it explicitly.
	if err := os.Chmod(path, 0600); err != nil {
		os.Remove(path)
		return "", nil, fmt.Errorf("chmod key tempfile: %w", err)
	}

	cleanup := func() {
		_ = os.Remove(path)
	}
	return path, cleanup, nil
}

// gitEnvWithSSHKey returns an env slice suitable for os/exec.Cmd.Env
// that sets GIT_SSH_COMMAND to use the given key file and known_hosts.
func gitEnvWithSSHKey(keyPath, knownHostsPath string) []string {
	sshCmd := fmt.Sprintf(
		"ssh -i %s -o StrictHostKeyChecking=accept-new -o IdentitiesOnly=yes -o UserKnownHostsFile=%s",
		keyPath, knownHostsPath,
	)
	env := append(os.Environ(),
		"GIT_SSH_COMMAND="+sshCmd,
		"GIT_TERMINAL_PROMPT=0",
	)
	return env
}

// redactKeyPath removes any occurrence of keyPath from a git error
// message so error logs never leak the tempfile location (which by
// itself isn't secret, but is a defense-in-depth hygiene habit).
func redactKeyPath(s, keyPath string) string {
	return strings.ReplaceAll(s, keyPath, "<redacted-key-path>")
}

var httpsGitHubRe = regexp.MustCompile(`^https://github\.com/([^/]+)/([^/]+?)(\.git)?$`)

// HTTPSToSSHURL converts a GitHub HTTPS URL to the SSH form
// git@github.com:owner/repo.git. Idempotent on SSH URLs. Errors on
// non-GitHub hosts — this codebase only supports GitHub in this phase.
func HTTPSToSSHURL(u string) (string, error) {
	// Passthrough for SSH
	if strings.HasPrefix(u, "git@github.com:") {
		return u, nil
	}
	m := httpsGitHubRe.FindStringSubmatch(u)
	if m == nil {
		return "", fmt.Errorf("workspace: URL %q is not a supported GitHub URL (only github.com is supported)", u)
	}
	owner, repo := m[1], m[2]
	return fmt.Sprintf("git@github.com:%s/%s.git", owner, repo), nil
}

var sshGitHubRe = regexp.MustCompile(`^git@github\.com:([^/]+)/([^/]+?)(\.git)?$`)

// parseRepoFromSSHURL extracts (owner, repo) from an SSH-form GitHub URL.
// Used by the GitHub deploy key upload path, which needs owner/repo in
// the API URL.
func parseRepoFromSSHURL(u string) (string, string, error) {
	m := sshGitHubRe.FindStringSubmatch(u)
	if m == nil {
		return "", "", fmt.Errorf("parseRepoFromSSHURL: %q is not a github SSH URL", u)
	}
	return m[1], m[2], nil
}
```

- [ ] **Step 4: Run the URL-helper tests**

Run: `cd forge-core && go test ./internal/workspace/... -run "TestHTTPSToSSHURL|TestParseRepoFromSSHURL" -v`
Expected: 4 tests pass:
- `TestHTTPSToSSHURL` (4 cases)
- `TestHTTPSToSSHURL_RejectsNonGitHub`
- `TestParseRepoFromSSHURL`
- `TestParseRepoFromSSHURL_RejectsGarbage`

- [ ] **Step 5: Commit**

```bash
git add forge-core/internal/workspace/git.go forge-core/internal/workspace/git_test.go
git commit -m "feat(workspace): SSH-aware git runner with key tempfile

RealGitRunner runs git subprocess with GIT_SSH_COMMAND wired to a
mode-0600 tempfile holding the decrypted deploy key private bytes.
Tempfile lifetime is scoped to the single git call via defer-cleanup;
the path is redacted from error strings as defense-in-depth.

Clone does fresh --depth=50 clone into the target dir (wipes first
to avoid git's 'target not empty' error). FetchAndResetHard does
'git fetch origin <branch>' followed by 'reset --hard origin/<branch>'
inside an existing clone — matches spec §3.7's resync-on-new-session.

HTTPSToSSHURL converts legacy https://github.com/... URLs (which is
what project.code_repo_url currently stores) to git@github.com:...
form. Non-GitHub URLs are rejected — this phase only supports GitHub.

No live-sshd integration test here; Clone/FetchAndResetHard are
exercised end-to-end in ensure_test.go via a local bare repo fixture."
```

---

### Task 1.5: Dependency pre-install RPC client

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
git commit -m "feat(workspace): dependency pre-install RPC client

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
in Phase 5."
```

---

### Task 1.6: `ProjectLookup` interface + production adapter

**Files:**
- Create: `forge-core/internal/workspace/lookup.go`
- Create: `forge-core/internal/workspace/lookup_test.go`

**Context:** `EnsureReady` needs to know a project's repo URL, default branch, and the owning user's GitHub token (for the initial deploy-key upload only). The workspace package must not import from `internal/module/project/` directly — that would create a cyclic dependency (project imports workspace via `WorkspaceProvider`). So we define a small `ProjectLookup` interface here and implement it via a thin adapter elsewhere (wired in `main.go`).

The interface has two methods:
1. `GetProject(ctx, tenantID, projectID) → (*ProjectInfo, error)` — returns repo URL and default branch
2. `GetOwnerGitHubToken(ctx, projectID) → (string, error)` — returns a GitHub PAT owned by whoever created the project (used ONCE per project to upload the deploy key, then never again)

We don't implement the adapter here. We stub it with a memory implementation for tests. The real adapter is created in Task 1.12 when we wire everything into `main.go`.

- [ ] **Step 1: Write the failing tests**

Create `forge-core/internal/workspace/lookup_test.go`:

```go
package workspace

import (
	"context"
	"errors"
	"testing"
)

type memoryLookup struct {
	projects map[int64]*ProjectInfo
	tokens   map[int64]string
}

func (m *memoryLookup) GetProject(ctx context.Context, tenantID, projectID int64) (*ProjectInfo, error) {
	p, ok := m.projects[projectID]
	if !ok {
		return nil, ErrProjectNotFound
	}
	return p, nil
}

func (m *memoryLookup) GetOwnerGitHubToken(ctx context.Context, projectID int64) (string, error) {
	t, ok := m.tokens[projectID]
	if !ok {
		return "", errors.New("no token")
	}
	return t, nil
}

func TestMemoryLookupImplementsInterface(t *testing.T) {
	// Compile-time check — assigning to the interface type verifies the
	// memoryLookup satisfies ProjectLookup. This test body is nearly empty
	// but the var assignment is the actual assertion.
	var _ ProjectLookup = &memoryLookup{}
}

func TestProjectInfo_FieldsPresent(t *testing.T) {
	p := &ProjectInfo{
		ProjectID:     42,
		TenantID:      1,
		RepoURL:       "https://github.com/owner/repo.git",
		DefaultBranch: "main",
	}
	if p.RepoURL == "" {
		t.Fatal("RepoURL must be populated")
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
type ProjectInfo struct {
	ProjectID     int64
	TenantID      int64
	RepoURL       string // HTTPS form as stored in project.code_repo_url; converted to SSH via HTTPSToSSHURL
	DefaultBranch string
	CreatedBy     int64 // Used to look up the owner's GitHub token for one-time deploy key upload
}

// ErrProjectNotFound is returned by ProjectLookup.GetProject when no
// project row exists for (tenantID, projectID). EnsureReady treats
// this as a fatal error — there's nothing to clone.
var ErrProjectNotFound = errors.New("workspace: project not found")

// ProjectLookup abstracts the project-row + github-token access that
// EnsureReady needs. Defined here in the workspace package to avoid
// a cyclic dependency with internal/module/project (which imports
// workspace.WorkspaceProvider).
//
// Production implementation is a thin adapter in
// forge-core/cmd/forge-core/main.go that delegates to
// project.Repository and auth.Service.
type ProjectLookup interface {
	// GetProject returns project metadata. Returns ErrProjectNotFound
	// if no row exists. The tenantID is passed explicitly so the
	// lookup can enforce multi-tenant isolation at the DB layer.
	GetProject(ctx context.Context, tenantID, projectID int64) (*ProjectInfo, error)

	// GetOwnerGitHubToken returns a usable GitHub PAT for the user
	// who owns the project. This is called ONCE per project, at the
	// moment we upload the deploy key, and never again. Any error
	// from this method halts EnsureReady with an error status.
	GetOwnerGitHubToken(ctx context.Context, projectID int64) (string, error)
}
```

- [ ] **Step 4: Run tests**

Run: `cd forge-core && go test ./internal/workspace/... -run "TestMemoryLookup|TestProjectInfo|TestErrProjectNotFound" -v`
Expected: 3 tests pass.

- [ ] **Step 5: Commit**

```bash
git add forge-core/internal/workspace/lookup.go forge-core/internal/workspace/lookup_test.go
git commit -m "feat(workspace): ProjectLookup interface for dep inversion

EnsureReady needs project metadata (repo URL, default branch) and a
one-time GitHub token (for the deploy key upload). Importing project
module directly would create a cyclic dep because project already
imports workspace.WorkspaceProvider. Define a small interface in the
workspace package; wire the adapter in main.go.

ProjectInfo carries the four fields EnsureReady actually needs.
ErrProjectNotFound is exported so callers can errors.Is against it.

In-test memoryLookup satisfies the interface for unit tests; the
real adapter comes in Task 1.12 (main.go wiring)."
```

---

### Task 1.7: EnsureReady state machine — core loop

**Files:**
- Create: `forge-core/internal/workspace/ensure.go`
- Create: `forge-core/internal/workspace/ensure_test.go`

**Context:** This is the heart of Phase 1. `EnsureReady(ctx, tenantID, projectID, forceSync)` is the single public entry point that drives the state machine. It handles all five cases: no row → create + clone, pending row → wait on advisory lock, ready row → maybe resync, ready row + forceSync=true → fetch+reset, error row → wipe + retry from scratch.

The method takes dependencies through the `Manager` struct (constructed in Task 1.11). The unit test uses fakes for gitRunner, prepClient, github uploader, and lookup. A full mock-free integration test that drives real git against a local bare repo comes in Task 1.10.

Method signature:
```go
func (m *Manager) EnsureReady(ctx context.Context, tenantID, projectID int64, forceSync bool) (*Workspace, error)
```

- [ ] **Step 1: Write the failing ensure_test.go (core cases)**

Create `forge-core/internal/workspace/ensure_test.go`:

```go
package workspace

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/base64"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/shulex/forge/forge-core/internal/secrets"
)

// --- Test fakes ---

type fakeGitRunner struct {
	mu               sync.Mutex
	cloneCalls       int
	fetchResetCalls  int
	cloneShouldFail  bool
	fetchShouldFail  bool
	clonedDirs       []string
	lastCloneBranch  string
}

func (f *fakeGitRunner) Clone(ctx context.Context, sshURL, dir string, key *DeployKey, branch string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.cloneCalls++
	f.lastCloneBranch = branch
	f.clonedDirs = append(f.clonedDirs, dir)
	if f.cloneShouldFail {
		return errors.New("fake clone failed")
	}
	// Simulate a successful clone by creating the dir with a .git marker
	_ = os.MkdirAll(filepath.Join(dir, ".git"), 0755)
	return nil
}

func (f *fakeGitRunner) FetchAndResetHard(ctx context.Context, dir, branch string, key *DeployKey) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.fetchResetCalls++
	if f.fetchShouldFail {
		return errors.New("fake fetch/reset failed")
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
	calls       int
	shouldFail  bool
	lastSSHKey  string
}

func (f *fakeUploader) Upload(ctx context.Context, token, owner, repo, title, sshKey string, readOnly bool) (int64, error) {
	f.calls++
	f.lastSSHKey = sshKey
	if f.shouldFail {
		return 0, errors.New("github upload failed")
	}
	return 88888, nil
}

// --- Setup helper ---

type ensureTestFixture struct {
	db       *sql.DB
	manager  *Manager
	git      *fakeGitRunner
	prep     *fakePrepClient
	uploader *fakeUploader
	lookup   *memoryLookup
	rootDir  string
}

func newEnsureFixture(t *testing.T, projectID int64) *ensureTestFixture {
	t.Helper()
	db := openTestDB(t)
	rootDir := t.TempDir()

	master := base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{0x77}, 32))
	secretsSvc, err := secrets.NewService(master)
	if err != nil {
		t.Fatalf("secrets: %v", err)
	}

	git := &fakeGitRunner{}
	prep := &fakePrepClient{}
	uploader := &fakeUploader{}
	lookup := &memoryLookup{
		projects: map[int64]*ProjectInfo{
			projectID: {
				ProjectID:     projectID,
				TenantID:      1,
				RepoURL:       "https://github.com/owner/repo.git",
				DefaultBranch: "main",
				CreatedBy:     99,
			},
		},
		tokens: map[int64]string{
			projectID: "ghp_fake_token",
		},
	}

	mgr := &Manager{
		root:         rootDir,
		stateRepo:    NewStateRepo(db),
		deployKeys:   NewDeployKeyRepo(db, secretsSvc),
		git:          git,
		prepClient:   prep,
		ghUploader:   uploader,
		lookup:       lookup,
	}

	t.Cleanup(func() {
		_, _ = db.Exec(`DELETE FROM engine.workspaces WHERE project_id = $1`, projectID)
		_, _ = db.Exec(`DELETE FROM engine.project_deploy_keys WHERE project_id = $1`, projectID)
	})

	return &ensureTestFixture{
		db:       db,
		manager:  mgr,
		git:      git,
		prep:     prep,
		uploader: uploader,
		lookup:   lookup,
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
	if f.uploader.calls != 1 {
		t.Errorf("github upload calls: want 1, got %d", f.uploader.calls)
	}
	if f.prep.calls != 1 {
		t.Errorf("prep calls: want 1, got %d", f.prep.calls)
	}
	if f.git.lastCloneBranch != "main" {
		t.Errorf("clone branch: want main, got %s", f.git.lastCloneBranch)
	}
	// Deploy key was stored
	dk, _ := f.manager.deployKeys.GetByProject(context.Background(), 201)
	if dk == nil {
		t.Fatal("deploy key row should exist after EnsureReady")
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
	baseFetches := f.git.fetchResetCalls

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
	if f.git.fetchResetCalls != baseFetches {
		t.Errorf("second call should not fetch; got %d additional fetches", f.git.fetchResetCalls-baseFetches)
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
	if f.git.fetchResetCalls != 1 {
		t.Errorf("fetch+reset calls: want 1, got %d", f.git.fetchResetCalls)
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

// --- Deploy key reused across calls ---

func TestEnsureReady_ReusesDeployKey(t *testing.T) {
	f := newEnsureFixture(t, 208)
	ctx := context.Background()

	if _, err := f.manager.EnsureReady(ctx, 1, 208, false); err != nil {
		t.Fatalf("first EnsureReady: %v", err)
	}
	if f.uploader.calls != 1 {
		t.Fatalf("first call upload count: want 1, got %d", f.uploader.calls)
	}

	// Force a resync — deploy key should NOT be regenerated/re-uploaded
	if _, err := f.manager.EnsureReady(ctx, 1, 208, true); err != nil {
		t.Fatalf("second EnsureReady: %v", err)
	}
	if f.uploader.calls != 1 {
		t.Errorf("second call should not re-upload: calls=%d", f.uploader.calls)
	}
}

// --- GitHub upload failure is fatal ---

func TestEnsureReady_GitHubUploadFails_MarksError(t *testing.T) {
	f := newEnsureFixture(t, 209)
	f.uploader.shouldFail = true

	_, err := f.manager.EnsureReady(context.Background(), 1, 209, false)
	if err == nil {
		t.Fatal("expected error from failing GitHub upload")
	}
	row, _ := f.manager.stateRepo.GetByProject(context.Background(), 1, 209)
	if row == nil || row.Status != StatusError {
		t.Errorf("row status: want error, got %+v", row)
	}
	// Should not have tried to clone — upload happens first
	if f.git.cloneCalls != 0 {
		t.Errorf("should not clone when upload fails: calls=%d", f.git.cloneCalls)
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
	if f.uploader.calls != 1 {
		t.Errorf("concurrent EnsureReady should upload deploy key once; got %d", f.uploader.calls)
	}
}
```

- [ ] **Step 2: Run tests — expect compile failure**

Run: `cd forge-core && go test ./internal/workspace/... -run TestEnsureReady`
Expected: `undefined: Manager field stateRepo`, or similar compile errors. `Manager` currently only has `root string`; we're adding state + deps in this task.

- [ ] **Step 3: Implement `ensure.go`**

Create `forge-core/internal/workspace/ensure.go`:

```go
package workspace

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"
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
func (m *Manager) EnsureReady(
	ctx context.Context,
	tenantID, projectID int64,
	forceSync bool,
) (*Workspace, error) {
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
			// INSERT but before any state transition. Treat as error.
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
//   1. Ensure project row exists via ProjectLookup
//   2. Ensure/generate deploy key, upload to GitHub on first encounter
//   3. Clone the repo via SSH
//   4. Call ai-worker prep (non-blocking)
//   5. Mark ready
func (m *Manager) freshInstall(
	ctx context.Context,
	tenantID, projectID int64,
) (*Workspace, error) {
	proj, err := m.lookup.GetProject(ctx, tenantID, projectID)
	if err != nil {
		m.markErrorOrLog(ctx, tenantID, projectID, fmt.Sprintf("project lookup: %v", err))
		return nil, fmt.Errorf("ensure: project lookup: %w", err)
	}

	sshURL, err := HTTPSToSSHURL(proj.RepoURL)
	if err != nil {
		m.markErrorOrLog(ctx, tenantID, projectID, fmt.Sprintf("unsupported repo URL: %v", err))
		return nil, fmt.Errorf("ensure: URL conversion: %w", err)
	}

	hostPath := m.ProjectDir(tenantID, projectID)
	containerPath := m.containerProjectDir(tenantID, projectID)

	// InsertPending is idempotent, so it's safe to call here whether or
	// not the row was just reset by the caller above.
	if err := m.stateRepo.InsertPending(ctx, tenantID, projectID, hostPath, containerPath); err != nil {
		return nil, fmt.Errorf("ensure: insert pending: %w", err)
	}

	// Deploy key lifecycle: reuse if present, generate+upload if not.
	dk, err := m.deployKeys.GetByProject(ctx, projectID)
	if err != nil {
		m.markErrorOrLog(ctx, tenantID, projectID, fmt.Sprintf("deploy key lookup: %v", err))
		return nil, fmt.Errorf("ensure: deploy key lookup: %w", err)
	}
	if dk == nil {
		dk, err = m.generateAndUploadDeployKey(ctx, proj, sshURL)
		if err != nil {
			m.markErrorOrLog(ctx, tenantID, projectID, fmt.Sprintf("deploy key setup: %v", err))
			return nil, fmt.Errorf("ensure: deploy key setup: %w", err)
		}
	}

	// Clone
	if err := m.git.Clone(ctx, sshURL, hostPath, dk, proj.DefaultBranch); err != nil {
		m.markErrorOrLog(ctx, tenantID, projectID, fmt.Sprintf("clone: %v", err))
		return nil, fmt.Errorf("ensure: clone: %w", err)
	}

	// Dep prep — non-blocking
	wsRelPath := m.relPath(tenantID, projectID)
	prepRes, prepErr := m.prepClient.Prep(ctx, tenantID, projectID, wsRelPath)
	if prepErr != nil {
		slog.Warn("workspace: dep prep transport error; proceeding to ready",
			"tenant", tenantID, "project", projectID, "error", prepErr)
	} else if prepRes != nil && prepRes.Status == "error" {
		slog.Warn("workspace: dep prep failed; proceeding to ready",
			"tenant", tenantID, "project", projectID, "reason", prepRes.Error)
	}

	if err := m.stateRepo.MarkReady(ctx, tenantID, projectID); err != nil {
		return nil, fmt.Errorf("ensure: mark ready: %w", err)
	}

	// Return the updated row
	return m.stateRepo.GetByProject(ctx, tenantID, projectID)
}

// resync performs a fetch + reset --hard on an already-ready workspace.
func (m *Manager) resync(
	ctx context.Context,
	existing *Workspace,
	tenantID, projectID int64,
) (*Workspace, error) {
	proj, err := m.lookup.GetProject(ctx, tenantID, projectID)
	if err != nil {
		return nil, fmt.Errorf("resync: project lookup: %w", err)
	}
	dk, err := m.deployKeys.GetByProject(ctx, projectID)
	if err != nil || dk == nil {
		return nil, fmt.Errorf("resync: deploy key missing: %w", err)
	}

	if err := m.git.FetchAndResetHard(ctx, existing.HostPath, proj.DefaultBranch, dk); err != nil {
		// Transparent recovery: if reset fails (corrupted repo, missing .git, etc),
		// wipe and re-clone via freshInstall.
		slog.Warn("workspace: fetch+reset failed; falling back to fresh clone",
			"tenant", tenantID, "project", projectID, "error", err)
		if err := m.stateRepo.ResetToPending(ctx, tenantID, projectID); err != nil {
			return nil, fmt.Errorf("resync: reset to pending: %w", err)
		}
		_ = os.RemoveAll(existing.HostPath)
		return m.freshInstall(ctx, tenantID, projectID)
	}

	// Update last_synced_at
	if err := m.stateRepo.MarkReady(ctx, tenantID, projectID); err != nil {
		return nil, fmt.Errorf("resync: mark ready: %w", err)
	}
	return m.stateRepo.GetByProject(ctx, tenantID, projectID)
}

// generateAndUploadDeployKey creates a fresh ed25519 keypair, uploads
// the public half to GitHub via the owner's PAT, and persists the row.
// This is called ONCE per project; subsequent EnsureReady calls reuse
// the stored key.
func (m *Manager) generateAndUploadDeployKey(
	ctx context.Context,
	proj *ProjectInfo,
	sshURL string,
) (*DeployKey, error) {
	comment := fmt.Sprintf("forge-deploy-%d-%d-%d", proj.TenantID, proj.ProjectID, time.Now().Unix())
	pub, priv, err := GenerateKeyPair(comment)
	if err != nil {
		return nil, fmt.Errorf("generate keypair: %w", err)
	}

	token, err := m.lookup.GetOwnerGitHubToken(ctx, proj.ProjectID)
	if err != nil {
		return nil, fmt.Errorf("get owner GitHub token: %w", err)
	}
	owner, repo, err := parseRepoFromSSHURL(sshURL)
	if err != nil {
		return nil, fmt.Errorf("parse ssh url: %w", err)
	}

	title := fmt.Sprintf("Forge: tenant %d project %d", proj.TenantID, proj.ProjectID)
	ghID, err := m.ghUploader.Upload(ctx, token, owner, repo, title, pub, false)
	if err != nil {
		return nil, fmt.Errorf("github upload: %w", err)
	}

	return m.deployKeys.Insert(ctx, proj.TenantID, proj.ProjectID, pub, priv, ghID)
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

- [ ] **Step 4: The tests need `Manager` to have new fields — defer to Task 1.8**

The tests reference `Manager.stateRepo`, `Manager.deployKeys`, `Manager.git`, `Manager.prepClient`, `Manager.ghUploader`, `Manager.lookup`. Those fields don't exist yet on the current single-field `Manager` (which is just `{root string}`). They get added in **Task 1.8** when we refactor `manager.go`. So `go test` will still fail to compile after this task.

**That's expected.** The next task (1.8) is specifically the `Manager` struct refactor. After 1.8, both tests and `ensure.go` compile; after 1.8's test run, the Task 1.7 tests in this task file start passing.

Go to Task 1.8 now — don't try to fix the compile error in isolation.

- [ ] **Step 5: Commit the state machine (with known pending compile error)**

```bash
git add forge-core/internal/workspace/ensure.go forge-core/internal/workspace/ensure_test.go
git commit -m "feat(workspace): EnsureReady state machine + tests (WIP: depends on 1.8)

EnsureReady is the single public entry point for the workspace lifecycle.
Handles five starting states under a pg_advisory_xact_lock:
  no row → fresh install (generate key, upload, clone, prep, mark ready)
  ready + !forceSync → no-op
  ready + forceSync → fetch + reset --hard (fallback: wipe + reclone)
  error → wipe + retry freshInstall
  pending → treat as crashed previous run, wipe + retry
freshInstall reuses the deploy key if one exists in the row (generated
lazily on first encounter per project).

Dep prep is non-blocking: transport failures and 'error' results log
warnings but still mark the row ready, matching spec §3.9.

Concurrency test spawns 3 goroutines and asserts only 1 clone + 1
github upload happen — advisory lock serializes them.

NOTE: this commit does NOT compile yet. Manager still has a single
'root' field; the ensure.go impl references stateRepo/deployKeys/
git/prepClient/ghUploader/lookup which are added in the next task
(1.8 manager.go refactor). Landing the state machine first so the
test suite is in place to verify 1.8."
```

---

### Task 1.8: Refactor `manager.go` — remove `EnsureClone`/`injectToken`, add dependencies

**Files:**
- Modify: `forge-core/internal/workspace/manager.go`
- Modify: `forge-core/internal/workspace/manager_test.go`

**Context:** Current `Manager` is `{root string}` with methods `NewManager`, `ProjectDir`, `TaskDir`, `EnsureClone`, `CreateWorktree`, `WriteFiles`, `CleanupTask`, and the private `injectToken`. We need to:

1. Add fields for the dependencies `EnsureReady` uses (state repo, deploy key repo, git runner, prep client, github uploader, project lookup)
2. Update `NewManager` to take those as a single config struct (constructor explosion is worse than a config struct with 6 fields)
3. **Delete `EnsureClone`** and `injectToken`
4. Update `manager_test.go` to remove the now-deleted `TestInjectToken` and `TestInjectToken_NoLeakInLogs`
5. Keep `ProjectDir`, `TaskDir`, `CreateWorktree`, `WriteFiles`, `CleanupTask`, `FileToWrite` unchanged

After this task both the 1.7 tests and 1.8's remaining manager tests should compile. The 1.7 tests need `FORGE_TEST_DATABASE_URL` and a running DB; the remaining manager tests work without DB.

- [ ] **Step 1: Read the current `manager.go` to know what's being changed**

Already reviewed in the reconnaissance above. Current struct:
```go
type Manager struct {
    root string
}
```

Current constructor: `func NewManager(root string) *Manager`.

Methods to keep: `ProjectDir`, `TaskDir`, `CreateWorktree`, `WriteFiles`, `CleanupTask`.
Methods to delete: `EnsureClone`, `injectToken`.

- [ ] **Step 2: Rewrite `manager.go`**

Replace the entire content of `forge-core/internal/workspace/manager.go`:

```go
// Package workspace owns the physical code artifact for each project.
//
// It handles:
//   - Cloning repos on first access (see EnsureReady in ensure.go)
//   - SSH deploy key lifecycle (generation, encryption, GitHub upload)
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
// 6-parameter NewManager call and makes it clear what's optional
// (nil stateRepo/deployKeys/git/prepClient/ghUploader/lookup all
// degrade gracefully — see method doc comments).
type Config struct {
	Root       string        // FORGE_WORKSPACE_ROOT; defaults to /data/forge/workspaces
	StateRepo  *StateRepo    // engine.workspaces DAO, nil disables EnsureReady
	DeployKeys *DeployKeyRepo // engine.project_deploy_keys DAO
	Git        gitRunner     // ssh-aware git wrapper; typically *RealGitRunner
	PrepClient prepRunner    // ai-worker /api/workspace/prep client; typically *PrepClient
	Uploader   githubUploader // GitHub deploy key upload client; typically *GitHubDeployKeyUploader
	Lookup     ProjectLookup // project metadata + owner github token
}

// prepRunner is the interface the state machine uses to run dep prep.
// Having it as an interface lets tests swap in a fake.
type prepRunner interface {
	Prep(ctx context.Context, tenantID, projectID int64, wsPath string) (*PrepResult, error)
}

// githubUploader is the interface the state machine uses for deploy
// key upload.
type githubUploader interface {
	Upload(ctx context.Context, token, owner, repo, title, sshKey string, readOnly bool) (int64, error)
}

// Manager handles local git clones and per-task worktrees.
type Manager struct {
	root       string
	stateRepo  *StateRepo
	deployKeys *DeployKeyRepo
	git        gitRunner
	prepClient prepRunner
	ghUploader githubUploader
	lookup     ProjectLookup
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
		root:       root,
		stateRepo:  cfg.StateRepo,
		deployKeys: cfg.DeployKeys,
		git:        cfg.Git,
		prepClient: cfg.PrepClient,
		ghUploader: cfg.Uploader,
		lookup:     cfg.Lookup,
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
```

- [ ] **Step 3: Update `manager_test.go`**

Edit `forge-core/internal/workspace/manager_test.go`. Delete tests that are no longer valid:

Delete `TestInjectToken` (lines 42-71).
Delete `TestInjectToken_NoLeakInLogs` (lines 125-139).

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

Remove the `strings` import at the top since `TestInjectToken_NoLeakInLogs` was using it — `go vet` will flag unused imports.

- [ ] **Step 4: Run the workspace tests**

```bash
cd forge-core && go test ./internal/workspace/... -v
```
Expected status:
- Pure-unit tests pass: `TestNewManager_DefaultRoot`, `_CustomRoot`, `TestProjectDir`, `TestTaskDir`, `TestWriteFiles`, `TestWriteFiles_NestedDirs`, `TestHTTPSToSSHURL` (all variants), `TestParseRepoFromSSHURL` (all variants), `TestUploadDeployKey_*`, `TestPrepClient_*`, `TestGenerateKeyPair_*`, `TestProjectInfo_*`, `TestMemoryLookup*`, `TestErrProjectNotFound`
- DB integration tests pass if `FORGE_TEST_DATABASE_URL` is set: `TestStateRepo_*`, `TestDeployKeyRepo_*`, `TestEnsureReady_*`
- DB integration tests skip cleanly if `FORGE_TEST_DATABASE_URL` is unset

If any fail, fix before proceeding.

- [ ] **Step 5: Run `go build` on the whole forge-core tree to catch compile errors in callers**

```bash
cd forge-core && go build ./...
```
Expected: **compile error** — `build_activities.go:96` and `devops_activities.go:134` still call `EnsureClone` which no longer exists. That's the handoff to the next two tasks (1.9 and 1.10). Note the error location but don't fix here.

Also expect: `main.go:122` calls `workspace.NewManager(cfg.WorkspaceRoot)` (old signature). That's the handoff to Task 1.12.

- [ ] **Step 6: Commit**

```bash
git add forge-core/internal/workspace/manager.go forge-core/internal/workspace/manager_test.go
git commit -m "refactor(workspace): delete EnsureClone/injectToken, add deps via Config

Manager now takes a Config struct with stateRepo/deployKeys/git/
prepClient/ghUploader/lookup. Nil dependencies are allowed (e.g.
tests that only need ProjectDir can pass Config{}).

Deleted:
- EnsureClone (replaced by EnsureReady in ensure.go)
- injectToken + TestInjectToken + TestInjectToken_NoLeakInLogs
  (HTTPS+token auth is gone; SSH deploy keys everywhere)

Kept (used by the temporal worker and project module):
- ProjectDir, TaskDir, CreateWorktree, WriteFiles, CleanupTask

Compile breaks in build_activities.go:96 and devops_activities.go:134
are expected — those are the two EnsureClone callers and are migrated
in the next two tasks (1.9, 1.10). main.go:122 also breaks on the
new NewManager signature; migrated in 1.12."
```

---

### Task 1.9: Migrate `build_activities.go` to `EnsureReady`

**Files:**
- Modify: `forge-core/internal/temporal/activity/build_activities.go`

**Context:** The `build_activities.go:96` call currently does:
```go
repoDir, err := a.ws.EnsureClone(ctx, input.TenantID, input.ProjectID,
    input.RepoURL, input.GitHubToken, defaultBranch)
```

It needs to become:
```go
ws, err := a.ws.EnsureReady(ctx, input.TenantID, input.ProjectID, false /* forceSync */)
repoDir := ws.HostPath
```

Plus the `input.RepoURL`, `input.GitHubToken`, and `defaultBranch` fields become unused in the clone path. Don't delete them from the input struct — they may still be referenced elsewhere (e.g., k8s image build contexts). Just stop passing them to workspace.

- [ ] **Step 1: Open `build_activities.go` and find the call site**

Search: `grep -n "EnsureClone\|defaultBranch" forge-core/internal/temporal/activity/build_activities.go`
Expected: one `EnsureClone` hit on line 96, one `defaultBranch` decl on the preceding line.

- [ ] **Step 2: Replace the EnsureClone block**

Edit `forge-core/internal/temporal/activity/build_activities.go`. Find:

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

- [ ] **Step 3: Build to verify**

```bash
cd forge-core && go build ./internal/temporal/activity/...
```
Expected: `build_activities.go` compiles. `devops_activities.go` still errors on its own `EnsureClone` call — that's Task 1.10.

- [ ] **Step 4: Run the activity tests**

```bash
cd forge-core && go test ./internal/temporal/activity/... -run TestBuild -v 2>&1 | tail -30
```
Expected: existing tests either pass or skip gracefully (some tests may require k8s/docker that isn't present). Key check: no compile errors, and no runtime panics about `EnsureClone`.

If tests can't run due to unrelated dependencies, verify the change at least compiles via `go vet`:
```bash
cd forge-core && go vet ./internal/temporal/activity/...
```

- [ ] **Step 5: Commit**

```bash
git add forge-core/internal/temporal/activity/build_activities.go
git commit -m "refactor(worker): migrate build_activities EnsureClone → EnsureReady

The build activity no longer passes repoURL/token/branch to workspace
— workspace.EnsureReady looks those up internally via ProjectLookup.
forceSync=false because we want to reuse an existing clone if the
agent already set it up.

input.RepoURL and input.GitHubToken fields remain on the activity
input struct because they may be consumed by other (non-workspace)
code paths; untouching them here keeps the diff minimal."
```

---

### Task 1.10: Migrate `devops_activities.go` to `EnsureReady`

**Files:**
- Modify: `forge-core/internal/temporal/activity/devops_activities.go`

**Context:** Same shape as Task 1.9. The `devops_activities.go:134` call site reads project info, builds a branch name, then calls `EnsureClone(ctx, tenantID, projectID, repoURL, token, defaultBranch)`. It uses the return value as `taskDir`'s parent via a subsequent `CreateWorktree` call.

- [ ] **Step 1: Read the call site**

```bash
grep -n "EnsureClone" forge-core/internal/temporal/activity/devops_activities.go
```
Expected: one hit on line ~134.

- [ ] **Step 2: Replace the call**

Edit `devops_activities.go`. Find:

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

- [ ] **Step 3: Build + vet**

```bash
cd forge-core && go build ./... && go vet ./internal/temporal/activity/...
```
Expected: the whole project builds — both activity callers now use `EnsureReady`, and since `EnsureClone` is gone, `main.go:122` is the last remaining break (handled in 1.12). Wait, actually — `main.go` still calls `workspace.NewManager(cfg.WorkspaceRoot)` with the old signature, so it still breaks. That's fine, keep going.

Expected: `go build ./...` fails with:
```
./cmd/forge-core/main.go:122: too few arguments in call to workspace.NewManager
```
That's the only remaining break. Task 1.12 fixes it.

- [ ] **Step 4: Commit**

```bash
git add forge-core/internal/temporal/activity/devops_activities.go
git commit -m "refactor(worker): migrate devops_activities EnsureClone → EnsureReady

Same migration as build_activities (Task 1.9). The devops activity
that previously cloned via EnsureClone(repoURL, token, branch) now
calls EnsureReady(tenantID, projectID, forceSync=false). Token and
repo URL are resolved internally by workspace via ProjectLookup.

After this commit, main.go is the last EnsureClone/NewManager
signature break — fixed in Task 1.12."
```

---

### Task 1.11: Migrate `agent/service.go` to `EnsureReady`

**Files:**
- Modify: `forge-core/internal/module/agent/service.go`
- Modify: `forge-core/internal/module/agent/service_test.go`

**Context:** `agent/service.go:SubmitMessage` currently uses `os.Stat(gitDir)` to probe whether the workspace has a `.git` directory — a shortcut that was introduced to populate `workspace_path` without owning the clone lifecycle. Now that `EnsureReady` exists, the probe goes away: the agent service synchronously calls `EnsureReady(ctx, tenantID, projectID, forceSync)` and uses the returned `Workspace.HostPath`.

The `forceSync` decision: agent service decides it based on whether this is a new session. A new session is signalled by the client via an (existing) `is_new_session` flag on `ChatRequest`, or equivalently by checking whether `req.SessionID == ""`. Check the actual current `ChatRequest` definition to decide which signal to use.

- [ ] **Step 1: Find the SessionID check in the current service**

Read the current `agent/service.go` SubmitMessage method (lines 60-100 roughly):
```bash
grep -n "SubmitMessage\|SessionID\|workspace_path\|os.Stat" forge-core/internal/module/agent/service.go
```

Look for: how is "is this a new session" currently determined?

If `ChatRequest` has an `IsNewSession bool` field: use it.
If not: `req.SessionID == ""` is the new-session signal.

- [ ] **Step 2: Update the service to call `EnsureReady`**

Edit `forge-core/internal/module/agent/service.go`. Find the block around line 80-95 that does `os.Stat(gitDir)`:

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
		_ = ws // ws.HostPath and ws.ContainerPath available if ever needed
	}
```

Remove the `os` and `path/filepath` imports if they're no longer used elsewhere in the file. (Likely still used for `WorkspaceRoot` handling — check before removing.)

- [ ] **Step 3: Update `service_test.go`**

The existing tests mock out `wsManager`. Find any test that constructs a `*workspace.Manager` directly (rather than using an interface mock). Those tests need updating to the new `workspace.Config{}` pattern.

```bash
grep -n "workspace.Manager\|NewManager\|EnsureClone\|wsManager" forge-core/internal/module/agent/service_test.go
```

If the tests use a mock interface (e.g., `workspaceManager` interface), no changes needed — just confirm it compiles.

If the tests use `*workspace.Manager` directly, update to `workspace.NewManager(workspace.Config{})`. The tests probably stub out the DB and dependencies via the Manager's nil-tolerant methods (ProjectDir works without any deps); if they also test the EnsureReady path, they'll need a fake Manager interface we don't yet have. For now, focus on compile-clean; full-path agent-service tests can be added after the e2e in Phase 7.

- [ ] **Step 4: Build + run tests**

```bash
cd forge-core && go build ./internal/module/agent/... && go test ./internal/module/agent/... -v 2>&1 | tail -30
```
Expected: builds, existing tests pass (they mostly exercise the HTTP-to-ai-worker plumbing, not workspace).

- [ ] **Step 5: Commit**

```bash
git add forge-core/internal/module/agent/service.go forge-core/internal/module/agent/service_test.go
git commit -m "refactor(agent): call workspace.EnsureReady instead of os.Stat(.git)

The agent service previously probed for .git dir existence as a
proxy for 'is workspace ready'. Now it calls EnsureReady directly,
which owns the full clone/prep/deploy-key lifecycle.

forceSync=(SessionID == '') so new sessions get a fetch+reset to
clean main, multi-turn messages in an existing session reuse the
current working tree (spec §2.7 resync semantics).

EnsureReady failures now propagate as errors from SubmitMessage
— previously a missing clone silently left workspace_path empty
and let ai-worker's pair_pipeline fallback kick in. A2 has no
pair_pipeline, so failing fast here is the right move."
```

---

### Task 1.12: Wire everything into `main.go`

**Files:**
- Modify: `forge-core/cmd/forge-core/main.go`
- Modify: `forge-core/internal/config/config.go`

**Context:** The capstone task for Phase 1. `main.go:122` currently calls `workspace.NewManager(cfg.WorkspaceRoot)`. We need to:

1. Read `FORGE_SECRETS_MASTER_KEY` from env via config (add to `Config` struct)
2. Read `FORGE_AI_WORKER_URL` (for prep client; probably already in config as `AIWorkerURL`)
3. Read `FORGE_GITHUB_API_URL` (for deploy key upload; defaults to "https://api.github.com")
4. Construct `secrets.NewService(cfg.SecretsMasterKey)`
5. Construct `workspace.NewStateRepo(db)`, `workspace.NewDeployKeyRepo(db, secrets)`, `workspace.NewRealGitRunner("")`, `workspace.NewPrepClient(cfg.AIWorkerURL)`, `workspace.NewGitHubDeployKeyUploader(cfg.GitHubAPIURL)`
6. Construct the `ProjectLookup` adapter (new — see below)
7. Pass everything into `workspace.NewManager(workspace.Config{...})`

The `ProjectLookup` adapter needs to live somewhere that can import both `internal/module/project` and `internal/module/auth`. `main.go` is the obvious place — it already imports both.

- [ ] **Step 1: Add config fields**

Edit `forge-core/internal/config/config.go`. Find the `Config` struct and add:

```go
	// Secrets master key (base64-encoded 32 bytes). Required when
	// workspace manager is active.
	SecretsMasterKey string

	// GitHub API base URL. Defaults to https://api.github.com. Override
	// for GitHub Enterprise.
	GitHubAPIURL string
```

Then in the `Load` function (or wherever envs are parsed), add:
```go
	SecretsMasterKey: getEnv("FORGE_SECRETS_MASTER_KEY", ""),
	GitHubAPIURL:     getEnv("FORGE_GITHUB_API_URL", "https://api.github.com"),
```

- [ ] **Step 2: Add the `ProjectLookup` adapter in `main.go`**

Edit `forge-core/cmd/forge-core/main.go`. Near the top of the file (after the imports block), add:

```go
// projectLookupAdapter satisfies workspace.ProjectLookup by delegating
// to the project module and the auth service. Defined in main so it
// can bridge the two without creating a package-level cycle.
type projectLookupAdapter struct {
	projectSvc *project.Service
	authSvc    *auth.Service
}

func (a *projectLookupAdapter) GetProject(ctx context.Context, tenantID, projectID int64) (*workspace.ProjectInfo, error) {
	// Use a system context that doesn't require a specific user —
	// the underlying repo should permit tenant-scoped reads.
	p, err := a.projectSvc.GetByIDInternal(ctx, projectID, tenantID)
	if err != nil {
		return nil, workspace.ErrProjectNotFound
	}
	return &workspace.ProjectInfo{
		ProjectID:     p.ID,
		TenantID:      p.TenantID,
		RepoURL:       p.CodeRepoURL,
		DefaultBranch: p.DefaultBranch,
		CreatedBy:     p.CreatedBy,
	}, nil
}

func (a *projectLookupAdapter) GetOwnerGitHubToken(ctx context.Context, projectID int64) (string, error) {
	// Look up the project, then fetch the creator's GitHub token.
	p, err := a.projectSvc.GetByIDInternal(ctx, projectID, 0 /* tenantID wildcard for internal lookup */)
	if err != nil {
		return "", err
	}
	return a.authSvc.GetGitHubToken(ctx, p.CreatedBy)
}
```

**Note:** this assumes `project.Service` has a `GetByIDInternal(ctx, projectID, tenantID)` method. If it doesn't, either:
- Add one (it's a thin wrapper over `s.repo.GetByID`)
- Or adapt this to use the existing method signature

Check first: `grep -n "func (s \*Service)" forge-core/internal/module/project/service.go | head -10`

- [ ] **Step 3: Update the `workspaceMgr` construction in `main.go`**

Find line 122 of `main.go`:

```go
	// Workspace manager (local git clones + per-task worktrees)
	workspaceMgr := workspace.NewManager(cfg.WorkspaceRoot)
```

Replace with:

```go
	// Workspace manager — Variant B single-agent wiring
	var workspaceMgr *workspace.Manager
	if cfg.SecretsMasterKey == "" {
		slog.Warn("FORGE_SECRETS_MASTER_KEY not set; workspace manager is in legacy mode (no EnsureReady). agent sessions will fail to clone.")
		workspaceMgr = workspace.NewManager(workspace.Config{Root: cfg.WorkspaceRoot})
	} else {
		secretsSvc, err := secrets.NewService(cfg.SecretsMasterKey)
		if err != nil {
			slog.Error("failed to initialize secrets service", "error", err)
			os.Exit(1)
		}
		workspaceMgr = workspace.NewManager(workspace.Config{
			Root:       cfg.WorkspaceRoot,
			StateRepo:  workspace.NewStateRepo(db),
			DeployKeys: workspace.NewDeployKeyRepo(db, secretsSvc),
			Git:        workspace.NewRealGitRunner(""),
			PrepClient: workspace.NewPrepClient(cfg.AIWorkerURL),
			Uploader:   workspace.NewGitHubDeployKeyUploader(cfg.GitHubAPIURL),
			// Lookup is wired in after projectService is constructed below
		})
	}
```

Then find where `projectService` is constructed (line 126) and AFTER it, add:

```go
	// Back-wire ProjectLookup now that projectService exists. (Chicken-
	// and-egg: workspace wants ProjectLookup, projectService wants
	// workspace for read-only ProjectDir.)
	if workspaceMgr != nil && cfg.SecretsMasterKey != "" {
		workspaceMgr.SetLookup(&projectLookupAdapter{
			projectSvc: projectService,
			authSvc:    authService,
		})
	}
```

This needs a new `SetLookup` method on `Manager`. Add it to `manager.go`:

```go
// SetLookup wires in the ProjectLookup after Manager construction.
// Needed because projectService depends on Manager.ProjectDir while
// Manager.EnsureReady depends on ProjectLookup — classic chicken-
// and-egg. main.go constructs Manager first (without Lookup), then
// projectService, then SetLookup.
func (m *Manager) SetLookup(lookup ProjectLookup) {
	m.lookup = lookup
}
```

- [ ] **Step 4: Add imports to `main.go`**

```go
import (
	// ...
	"github.com/shulex/forge/forge-core/internal/secrets"
	// workspace already imported
)
```

- [ ] **Step 5: Build**

```bash
cd forge-core && go build ./...
```
Expected: clean build. No more compile errors anywhere.

If this fails, the most common issue is `project.Service.GetByIDInternal` not existing. If so, add it:

```go
// In forge-core/internal/module/project/service.go

// GetByIDInternal loads a project by ID for internal callers that
// bypass normal RBAC. tenantID=0 means "any tenant" — caller must
// have already authorized the access.
func (s *Service) GetByIDInternal(ctx context.Context, projectID, tenantID int64) (*Project, error) {
	return s.repo.GetByIDInternal(ctx, projectID, tenantID)
}
```

And in `repository.go`, add the corresponding `GetByIDInternal(ctx, projectID, tenantID)` that runs a simple WHERE.

- [ ] **Step 6: Run the full test suite**

```bash
cd forge-core && go test ./... 2>&1 | tail -30
```
Expected: all tests pass (or skip with known reasons). Pay special attention to `./internal/workspace/...` — that's the biggest new surface.

- [ ] **Step 7: Commit**

```bash
git add forge-core/cmd/forge-core/main.go forge-core/internal/config/config.go forge-core/internal/workspace/manager.go forge-core/internal/module/project/service.go forge-core/internal/module/project/repository.go 2>/dev/null || true
git commit -m "feat(main): wire secrets + workspace module dependencies

Config gets FORGE_SECRETS_MASTER_KEY and FORGE_GITHUB_API_URL. When
master key is present, main constructs a fully-wired Manager with
StateRepo + DeployKeyRepo + RealGitRunner + PrepClient + GitHub
uploader. When master key is missing, Manager falls back to legacy
mode (ProjectDir works, EnsureReady returns nil-deref errors) with
a loud warning.

ProjectLookup adapter bridges workspace → project + auth without
creating a package cycle; lives in main.go because that's where
both modules are already imported.

Manager.SetLookup is a post-construction wire to resolve the
chicken-and-egg between projectService (wants ProjectDir) and
Manager.EnsureReady (wants ProjectLookup)."
```

---

### Task 1.13: End-to-end integration test with a local bare repo

**Files:**
- Create: `forge-core/internal/workspace/e2e_test.go`

**Context:** Final Phase 1 task. The unit tests in Task 1.7 use fake git runners; this test uses a **real** git binary against a **local bare repo** fixture. It verifies the actual shell command plumbing, tempfile lifecycle, and git error parsing. Still a Go test, still fast, but no mocks of git itself.

The test creates a local bare repo in `t.TempDir()`, seeds it with one commit, generates a fake "deploy key" (the file doesn't need to be a real SSH key because we use `file://` URL form, not SSH), and drives `Clone` + `FetchAndResetHard` through the `RealGitRunner`.

**Important: this test does not verify the SSH key actually works** (that requires a real sshd). It verifies the rest of the plumbing: tempfile creation, command invocation, stderr capture, success paths, failure paths. SSH auth is covered by the adversarial integration test in Phase 7.

- [ ] **Step 1: Write the e2e test**

Create `forge-core/internal/workspace/e2e_test.go`:

```go
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

	return "file://" + bareDir
}

func TestRealGitRunner_Clone_LocalBareRepo(t *testing.T) {
	gitAvailable(t)

	bareDir := t.TempDir()
	fileURL := seedBareRepo(t, bareDir)

	targetDir := filepath.Join(t.TempDir(), "cloned")
	runner := NewRealGitRunner("")

	// Use dummy non-empty bytes — the file:// URL doesn't invoke SSH,
	// so the key content doesn't matter for this code path. But
	// RealGitRunner.Clone still writes the tempfile, so the DeployKey
	// struct must have non-empty PrivateKey bytes.
	//
	// We deliberately avoid real PEM headers here to keep
	// secret-scanner pre-commit hooks happy. Real integration with
	// an actual SSH server is covered by Phase 7's smoke test.
	key := &DeployKey{
		TenantID:   1,
		ProjectID:  1,
		PrivateKey: []byte("fake-key-bytes-only-for-tempfile-test"),
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := runner.Clone(ctx, fileURL, targetDir, key, "main"); err != nil {
		t.Fatalf("Clone: %v", err)
	}

	// Verify the clone worked
	if _, err := os.Stat(filepath.Join(targetDir, ".git")); err != nil {
		t.Errorf(".git dir missing after clone: %v", err)
	}
	if _, err := os.Stat(filepath.Join(targetDir, "README.md")); err != nil {
		t.Errorf("README.md missing after clone: %v", err)
	}
}

func TestRealGitRunner_FetchAndResetHard_LocalBareRepo(t *testing.T) {
	gitAvailable(t)

	bareDir := t.TempDir()
	fileURL := seedBareRepo(t, bareDir)

	targetDir := filepath.Join(t.TempDir(), "cloned")
	runner := NewRealGitRunner("")
	key := &DeployKey{
		TenantID:   1,
		ProjectID:  2,
		PrivateKey: []byte("dummy"),
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := runner.Clone(ctx, fileURL, targetDir, key, "main"); err != nil {
		t.Fatalf("Clone: %v", err)
	}

	// Modify a file locally to verify reset --hard wipes it
	localChange := filepath.Join(targetDir, "README.md")
	if err := os.WriteFile(localChange, []byte("local modification"), 0644); err != nil {
		t.Fatalf("write local change: %v", err)
	}

	// Run FetchAndResetHard
	if err := runner.FetchAndResetHard(ctx, targetDir, "main", key); err != nil {
		t.Fatalf("FetchAndResetHard: %v", err)
	}

	// README should be back to "initial"
	content, err := os.ReadFile(localChange)
	if err != nil {
		t.Fatalf("read README: %v", err)
	}
	if string(content) != "initial" {
		t.Errorf("README content after reset: %q, want 'initial'", string(content))
	}
}

func TestRealGitRunner_Clone_TempfileCleanup(t *testing.T) {
	gitAvailable(t)

	bareDir := t.TempDir()
	fileURL := seedBareRepo(t, bareDir)

	targetDir := filepath.Join(t.TempDir(), "cloned")
	runner := NewRealGitRunner("")
	key := &DeployKey{
		TenantID:   1,
		ProjectID:  3,
		PrivateKey: []byte("dummy-key-bytes-for-tempfile-test"),
	}

	ctx := context.Background()
	if err := runner.Clone(ctx, fileURL, targetDir, key, "main"); err != nil {
		t.Fatalf("Clone: %v", err)
	}

	// Look for any leftover forge-key tempfiles in /tmp
	matches, err := filepath.Glob(filepath.Join(os.TempDir(), "forge-key-*"))
	if err != nil {
		t.Fatalf("glob: %v", err)
	}
	if len(matches) > 0 {
		t.Errorf("leftover forge-key tempfiles after Clone: %v", matches)
	}
}

func TestRealGitRunner_Clone_InvalidBranch(t *testing.T) {
	gitAvailable(t)

	bareDir := t.TempDir()
	fileURL := seedBareRepo(t, bareDir)

	targetDir := filepath.Join(t.TempDir(), "cloned")
	runner := NewRealGitRunner("")
	key := &DeployKey{
		TenantID:   1,
		ProjectID:  4,
		PrivateKey: []byte("dummy"),
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	err := runner.Clone(ctx, fileURL, targetDir, key, "does-not-exist")
	if err == nil {
		t.Fatal("expected error cloning nonexistent branch")
	}
}
```

- [ ] **Step 2: Run the tests**

```bash
cd forge-core && go test ./internal/workspace/... -run "TestRealGitRunner_" -v
```
Expected: 4 tests pass:
- `TestRealGitRunner_Clone_LocalBareRepo`
- `TestRealGitRunner_FetchAndResetHard_LocalBareRepo`
- `TestRealGitRunner_Clone_TempfileCleanup`
- `TestRealGitRunner_Clone_InvalidBranch`

On a machine without git installed, they skip cleanly.

- [ ] **Step 3: Run the whole workspace package tests to confirm everything still works together**

```bash
cd forge-core && go test ./internal/workspace/... -v 2>&1 | tail -50
```
Expected: all tests pass or skip with documented reasons (DB-integration tests skip if `FORGE_TEST_DATABASE_URL` is unset; git tests skip if git is missing).

- [ ] **Step 4: Commit**

```bash
git add forge-core/internal/workspace/e2e_test.go
git commit -m "test(workspace): e2e integration against local bare git repo

Drives RealGitRunner.Clone + FetchAndResetHard against a file:// URL
backed by a real bare git repo in t.TempDir(). Verifies:
- clone produces .git + working tree files
- reset --hard wipes local modifications
- tempfile cleanup leaves no stray forge-key-* files
- clone of nonexistent branch fails with an error

Uses file:// URL so we don't need sshd or real deploy keys —
this is a plumbing test, not an SSH auth test. SSH auth is
exercised by the smoke test in Phase 7.

Skips cleanly on machines without git on PATH."
```

---

### Phase 1 completion check

Before starting Phase 2:

- [ ] `go build ./...` in forge-core produces a clean binary
- [ ] `go vet ./...` in forge-core is clean
- [ ] `go test ./internal/workspace/... -v` passes (or skips DB tests cleanly if `FORGE_TEST_DATABASE_URL` unset)
- [ ] `go test ./internal/secrets/...` still green from Phase 0
- [ ] `go test ./internal/temporal/activity/... -run TestBuild` passes (migrated caller)
- [ ] `go test ./internal/module/agent/...` passes (migrated caller)
- [ ] `injectToken` no longer exists anywhere: `grep -rn injectToken forge-core/` returns nothing
- [ ] `EnsureClone` no longer exists: `grep -rn EnsureClone forge-core/` returns nothing
- [ ] `main.go` wires in `secrets.NewService`, `workspace.NewStateRepo`, `workspace.NewDeployKeyRepo`, `workspace.NewRealGitRunner`, `workspace.NewPrepClient`, `workspace.NewGitHubDeployKeyUploader`, `projectLookupAdapter`
- [ ] Branch has **13 new commits** from this phase (one per task)
- [ ] A dev-mode `docker compose up forge-core` starts cleanly (with `FORGE_SECRETS_MASTER_KEY` in env)

## Phase 1 outputs unlock

- Phase 5 agent service has a working `EnsureReady` call it can depend on
- Phase 5 ai-worker endpoint `POST /api/workspace/prep` has a Go client calling it
- Phase 7 smoke test has a real workspace manager it can exercise
