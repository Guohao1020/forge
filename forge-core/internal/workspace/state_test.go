package workspace

import (
	"context"
	"database/sql"
	"os"
	"sync"
	"testing"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib" // pgx driver for database/sql
)

// openTestDB returns a *sql.DB connected to the integration test database.
// The test is skipped if FORGE_TEST_DATABASE_URL is not set.
func openTestDB(t *testing.T) *sql.DB {
	t.Helper()
	dsn := os.Getenv("FORGE_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("FORGE_TEST_DATABASE_URL not set; skipping integration test")
	}
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

// cleanupWorkspace deletes the workspace row for the given tenant+project so
// tests leave no residue.
func cleanupWorkspace(t *testing.T, db *sql.DB, tenantID, projectID int64) {
	t.Helper()
	t.Cleanup(func() {
		_, _ = db.Exec("DELETE FROM engine.workspaces WHERE tenant_id = $1 AND project_id = $2",
			tenantID, projectID)
	})
}

func TestStateRepo_GetByProject_NotFound(t *testing.T) {
	db := openTestDB(t)
	repo := NewStateRepo(db)

	// Use IDs unlikely to collide with real data.
	ws, err := repo.GetByProject(context.Background(), 999999, 999999)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ws != nil {
		t.Fatalf("expected nil workspace, got %+v", ws)
	}
}

func TestStateRepo_InsertPendingAndGet(t *testing.T) {
	db := openTestDB(t)
	repo := NewStateRepo(db)

	const tenantID, projectID = 900001, 900001
	cleanupWorkspace(t, db, tenantID, projectID)

	err := repo.InsertPending(context.Background(), tenantID, projectID, "/host/path", "/container/path")
	if err != nil {
		t.Fatalf("InsertPending: %v", err)
	}

	ws, err := repo.GetByProject(context.Background(), tenantID, projectID)
	if err != nil {
		t.Fatalf("GetByProject: %v", err)
	}
	if ws == nil {
		t.Fatal("expected workspace, got nil")
	}
	if ws.TenantID != tenantID {
		t.Errorf("tenant_id: got %d, want %d", ws.TenantID, tenantID)
	}
	if ws.ProjectID != projectID {
		t.Errorf("project_id: got %d, want %d", ws.ProjectID, projectID)
	}
	if ws.HostPath != "/host/path" {
		t.Errorf("host_path: got %q, want %q", ws.HostPath, "/host/path")
	}
	if ws.ContainerPath != "/container/path" {
		t.Errorf("container_path: got %q, want %q", ws.ContainerPath, "/container/path")
	}
	if ws.Status != StatusPending {
		t.Errorf("status: got %q, want %q", ws.Status, StatusPending)
	}
	if ws.LastSyncedAt != nil {
		t.Errorf("last_synced_at: expected nil, got %v", ws.LastSyncedAt)
	}
	if ws.LastError != nil {
		t.Errorf("last_error: expected nil, got %v", ws.LastError)
	}
}

func TestStateRepo_InsertPendingIdempotent(t *testing.T) {
	db := openTestDB(t)
	repo := NewStateRepo(db)

	const tenantID, projectID = 900002, 900002
	cleanupWorkspace(t, db, tenantID, projectID)

	// First insert.
	if err := repo.InsertPending(context.Background(), tenantID, projectID, "/a", "/b"); err != nil {
		t.Fatalf("first InsertPending: %v", err)
	}
	// Second insert with different paths — should be a no-op.
	if err := repo.InsertPending(context.Background(), tenantID, projectID, "/x", "/y"); err != nil {
		t.Fatalf("second InsertPending: %v", err)
	}

	ws, err := repo.GetByProject(context.Background(), tenantID, projectID)
	if err != nil {
		t.Fatalf("GetByProject: %v", err)
	}
	if ws == nil {
		t.Fatal("expected workspace, got nil")
	}
	// Paths should still be from the first insert.
	if ws.HostPath != "/a" {
		t.Errorf("host_path: got %q, want %q (first insert should win)", ws.HostPath, "/a")
	}
}

func TestStateRepo_MarkReady(t *testing.T) {
	db := openTestDB(t)
	repo := NewStateRepo(db)

	const tenantID, projectID = 900003, 900003
	cleanupWorkspace(t, db, tenantID, projectID)

	if err := repo.InsertPending(context.Background(), tenantID, projectID, "/h", "/c"); err != nil {
		t.Fatalf("InsertPending: %v", err)
	}

	if err := repo.MarkReady(context.Background(), tenantID, projectID); err != nil {
		t.Fatalf("MarkReady: %v", err)
	}

	ws, err := repo.GetByProject(context.Background(), tenantID, projectID)
	if err != nil {
		t.Fatalf("GetByProject: %v", err)
	}
	if ws == nil {
		t.Fatal("expected workspace, got nil")
	}
	if ws.Status != StatusReady {
		t.Errorf("status: got %q, want %q", ws.Status, StatusReady)
	}
	if ws.LastSyncedAt == nil {
		t.Error("last_synced_at should be set after MarkReady")
	}
	if ws.LastError != nil {
		t.Errorf("last_error should be nil after MarkReady, got %v", *ws.LastError)
	}
}

func TestStateRepo_MarkError(t *testing.T) {
	db := openTestDB(t)
	repo := NewStateRepo(db)

	const tenantID, projectID = 900004, 900004
	cleanupWorkspace(t, db, tenantID, projectID)

	if err := repo.InsertPending(context.Background(), tenantID, projectID, "/h", "/c"); err != nil {
		t.Fatalf("InsertPending: %v", err)
	}

	const reason = "git clone failed: permission denied"
	if err := repo.MarkError(context.Background(), tenantID, projectID, reason); err != nil {
		t.Fatalf("MarkError: %v", err)
	}

	ws, err := repo.GetByProject(context.Background(), tenantID, projectID)
	if err != nil {
		t.Fatalf("GetByProject: %v", err)
	}
	if ws == nil {
		t.Fatal("expected workspace, got nil")
	}
	if ws.Status != StatusError {
		t.Errorf("status: got %q, want %q", ws.Status, StatusError)
	}
	if ws.LastError == nil || *ws.LastError != reason {
		t.Errorf("last_error: got %v, want %q", ws.LastError, reason)
	}
}

func TestStateRepo_WithAdvisoryLock_Serializes(t *testing.T) {
	db := openTestDB(t)
	repo := NewStateRepo(db)

	const tenantID, projectID = 900005, 900005

	var mu sync.Mutex
	var order []int

	var wg sync.WaitGroup
	wg.Add(2)

	// Goroutine 1: acquires lock, holds 100ms, records "1".
	go func() {
		defer wg.Done()
		err := repo.WithAdvisoryLock(context.Background(), tenantID, projectID, func(_ *sql.Tx) error {
			mu.Lock()
			order = append(order, 1)
			mu.Unlock()
			time.Sleep(100 * time.Millisecond)
			return nil
		})
		if err != nil {
			t.Errorf("goroutine 1: %v", err)
		}
	}()

	// Small delay to ensure goroutine 1 acquires the lock first.
	time.Sleep(10 * time.Millisecond)

	// Goroutine 2: must wait for goroutine 1 to finish, then records "2".
	go func() {
		defer wg.Done()
		err := repo.WithAdvisoryLock(context.Background(), tenantID, projectID, func(_ *sql.Tx) error {
			mu.Lock()
			order = append(order, 2)
			mu.Unlock()
			return nil
		})
		if err != nil {
			t.Errorf("goroutine 2: %v", err)
		}
	}()

	wg.Wait()

	if len(order) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(order))
	}
	if order[0] != 1 || order[1] != 2 {
		t.Errorf("expected execution order [1, 2], got %v", order)
	}
}
