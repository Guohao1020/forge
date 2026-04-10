package workspace

import (
	"context"
	"database/sql"
	"fmt"
	"hash/fnv"
	"time"
)

// WorkspaceStatus represents the lifecycle state of a workspace.
type WorkspaceStatus string

const (
	StatusPending WorkspaceStatus = "pending"
	StatusReady   WorkspaceStatus = "ready"
	StatusError   WorkspaceStatus = "error"
)

// Workspace is the persistent record for a project's local clone state.
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

// StateRepo provides CRUD operations on the engine.workspaces table.
type StateRepo struct {
	db *sql.DB
}

// NewStateRepo creates a StateRepo backed by the given database connection.
func NewStateRepo(db *sql.DB) *StateRepo {
	return &StateRepo{db: db}
}

// GetByProject returns the workspace row for the given tenant+project, or
// (nil, nil) when no row exists.
func (r *StateRepo) GetByProject(ctx context.Context, tenantID, projectID int64) (*Workspace, error) {
	const q = `
		SELECT id, tenant_id, project_id, host_path, container_path,
		       status, last_synced_at, last_error, created_at, updated_at
		  FROM engine.workspaces
		 WHERE tenant_id = $1 AND project_id = $2`

	w := &Workspace{}
	err := r.db.QueryRowContext(ctx, q, tenantID, projectID).Scan(
		&w.ID, &w.TenantID, &w.ProjectID, &w.HostPath, &w.ContainerPath,
		&w.Status, &w.LastSyncedAt, &w.LastError, &w.CreatedAt, &w.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get workspace by project: %w", err)
	}
	return w, nil
}

// InsertPending inserts a new workspace in "pending" state. If a row for
// (tenant_id, project_id) already exists the call is a no-op (ON CONFLICT DO NOTHING).
func (r *StateRepo) InsertPending(ctx context.Context, tenantID, projectID int64, hostPath, containerPath string) error {
	const q = `
		INSERT INTO engine.workspaces (tenant_id, project_id, host_path, container_path, status)
		VALUES ($1, $2, $3, $4, 'pending')
		ON CONFLICT (tenant_id, project_id) DO NOTHING`

	_, err := r.db.ExecContext(ctx, q, tenantID, projectID, hostPath, containerPath)
	if err != nil {
		return fmt.Errorf("insert pending workspace: %w", err)
	}
	return nil
}

// MarkReady transitions the workspace to "ready", records last_synced_at as
// now(), and clears any previous error.
func (r *StateRepo) MarkReady(ctx context.Context, tenantID, projectID int64) error {
	const q = `
		UPDATE engine.workspaces
		   SET status = 'ready', last_synced_at = now(), last_error = NULL, updated_at = now()
		 WHERE tenant_id = $1 AND project_id = $2`

	_, err := r.db.ExecContext(ctx, q, tenantID, projectID)
	if err != nil {
		return fmt.Errorf("mark workspace ready: %w", err)
	}
	return nil
}

// MarkError transitions the workspace to "error" and records the reason.
func (r *StateRepo) MarkError(ctx context.Context, tenantID, projectID int64, reason string) error {
	const q = `
		UPDATE engine.workspaces
		   SET status = 'error', last_error = $3, updated_at = now()
		 WHERE tenant_id = $1 AND project_id = $2`

	_, err := r.db.ExecContext(ctx, q, tenantID, projectID, reason)
	if err != nil {
		return fmt.Errorf("mark workspace error: %w", err)
	}
	return nil
}

// ResetToPending transitions the workspace back to "pending" and clears the error.
func (r *StateRepo) ResetToPending(ctx context.Context, tenantID, projectID int64) error {
	const q = `
		UPDATE engine.workspaces
		   SET status = 'pending', last_error = NULL, updated_at = now()
		 WHERE tenant_id = $1 AND project_id = $2`

	_, err := r.db.ExecContext(ctx, q, tenantID, projectID)
	if err != nil {
		return fmt.Errorf("reset workspace to pending: %w", err)
	}
	return nil
}

// WithAdvisoryLock executes fn inside a transaction that holds a PostgreSQL
// advisory lock scoped to (tenantID, projectID). The lock is automatically
// released when the transaction commits or rolls back.
func (r *StateRepo) WithAdvisoryLock(ctx context.Context, tenantID, projectID int64, fn func(tx *sql.Tx) error) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx for advisory lock: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck

	key := advisoryLockKey(tenantID, projectID)
	if _, err := tx.ExecContext(ctx, "SELECT pg_advisory_xact_lock($1)", key); err != nil {
		return fmt.Errorf("acquire advisory lock: %w", err)
	}

	if err := fn(tx); err != nil {
		return err
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit advisory lock tx: %w", err)
	}
	return nil
}

// advisoryLockKey produces a deterministic int64 from a (tenantID, projectID)
// pair using FNV-1a over the string "workspace:{tenant}:{project}".
func advisoryLockKey(tenantID, projectID int64) int64 {
	h := fnv.New64a()
	_, _ = fmt.Fprintf(h, "workspace:%d:%d", tenantID, projectID)
	return int64(h.Sum64())
}
