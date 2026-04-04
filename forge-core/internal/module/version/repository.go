package version

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

// Create inserts a new project version.
func (r *Repository) Create(ctx context.Context, v *ProjectVersion) error {
	return r.db.QueryRow(ctx,
		`INSERT INTO engine.project_versions (tenant_id, project_id, version, status, description, created_by)
		 VALUES ($1, $2, $3, $4, $5, $6)
		 RETURNING id, created_at, updated_at`,
		v.TenantID, v.ProjectID, v.Version, v.Status, v.Description, v.CreatedBy,
	).Scan(&v.ID, &v.CreatedAt, &v.UpdatedAt)
}

// ListByProject returns all versions for a project, ordered by creation time desc.
func (r *Repository) ListByProject(ctx context.Context, projectID, tenantID int64) ([]ProjectVersion, error) {
	rows, err := r.db.Query(ctx,
		`SELECT v.id, v.tenant_id, v.project_id, v.version, v.status, v.description,
		        COALESCE(v.git_tag, ''), v.released_at, COALESCE(v.created_by, 0), v.created_at, v.updated_at,
		        COALESCE((SELECT COUNT(*) FROM engine.tasks t WHERE t.version_id = v.id), 0) AS task_count,
		        COALESCE((SELECT COUNT(*) FROM engine.tasks t WHERE t.version_id = v.id AND t.status = 'COMPLETED'), 0) AS completed_count
		 FROM engine.project_versions v
		 WHERE v.project_id = $1 AND v.tenant_id = $2
		 ORDER BY v.created_at DESC`,
		projectID, tenantID,
	)
	if err != nil {
		return nil, fmt.Errorf("list versions: %w", err)
	}
	defer rows.Close()

	var versions []ProjectVersion
	for rows.Next() {
		var v ProjectVersion
		if err := rows.Scan(
			&v.ID, &v.TenantID, &v.ProjectID, &v.Version, &v.Status, &v.Description,
			&v.GitTag, &v.ReleasedAt, &v.CreatedBy, &v.CreatedAt, &v.UpdatedAt,
			&v.TaskCount, &v.CompletedCount,
		); err != nil {
			return nil, fmt.Errorf("scan version: %w", err)
		}
		versions = append(versions, v)
	}
	if versions == nil {
		versions = []ProjectVersion{}
	}
	return versions, nil
}

// GetByID returns a single version by ID.
func (r *Repository) GetByID(ctx context.Context, id, tenantID int64) (*ProjectVersion, error) {
	var v ProjectVersion
	err := r.db.QueryRow(ctx,
		`SELECT v.id, v.tenant_id, v.project_id, v.version, v.status, v.description,
		        COALESCE(v.git_tag, ''), v.released_at, COALESCE(v.created_by, 0), v.created_at, v.updated_at,
		        COALESCE((SELECT COUNT(*) FROM engine.tasks t WHERE t.version_id = v.id), 0),
		        COALESCE((SELECT COUNT(*) FROM engine.tasks t WHERE t.version_id = v.id AND t.status = 'COMPLETED'), 0)
		 FROM engine.project_versions v
		 WHERE v.id = $1 AND v.tenant_id = $2`,
		id, tenantID,
	).Scan(
		&v.ID, &v.TenantID, &v.ProjectID, &v.Version, &v.Status, &v.Description,
		&v.GitTag, &v.ReleasedAt, &v.CreatedBy, &v.CreatedAt, &v.UpdatedAt,
		&v.TaskCount, &v.CompletedCount,
	)
	if err != nil {
		return nil, err
	}
	return &v, nil
}

// Update modifies version description and/or status.
func (r *Repository) Update(ctx context.Context, id, tenantID int64, desc *string, status *string) error {
	if desc != nil {
		_, err := r.db.Exec(ctx,
			`UPDATE engine.project_versions SET description = $1, updated_at = NOW() WHERE id = $2 AND tenant_id = $3`,
			*desc, id, tenantID,
		)
		if err != nil {
			return err
		}
	}
	if status != nil {
		_, err := r.db.Exec(ctx,
			`UPDATE engine.project_versions SET status = $1, updated_at = NOW() WHERE id = $2 AND tenant_id = $3`,
			*status, id, tenantID,
		)
		if err != nil {
			return err
		}
	}
	return nil
}

// Release marks a version as released with a git tag.
func (r *Repository) Release(ctx context.Context, id, tenantID int64, gitTag string) error {
	now := time.Now()
	_, err := r.db.Exec(ctx,
		`UPDATE engine.project_versions
		 SET status = $1, git_tag = $2, released_at = $3, updated_at = $3
		 WHERE id = $4 AND tenant_id = $5`,
		StatusReleased, gitTag, now, id, tenantID,
	)
	return err
}

// GetTasksByVersion returns all tasks associated with a version.
func (r *Repository) GetTasksByVersion(ctx context.Context, versionID, tenantID int64) ([]VersionTaskBrief, error) {
	rows, err := r.db.Query(ctx,
		`SELECT id, title, status, COALESCE(conflict_status, 'NONE'),
		        COALESCE(blocked_by, '[]'::jsonb), COALESCE(touched_files, '[]'::jsonb),
		        COALESCE(branch_name, ''), COALESCE(pr_number, 0),
		        created_at, completed_at
		 FROM engine.tasks
		 WHERE version_id = $1 AND tenant_id = $2
		 ORDER BY created_at ASC`,
		versionID, tenantID,
	)
	if err != nil {
		return nil, fmt.Errorf("list version tasks: %w", err)
	}
	defer rows.Close()

	var tasks []VersionTaskBrief
	for rows.Next() {
		var t VersionTaskBrief
		if err := rows.Scan(
			&t.ID, &t.Title, &t.Status, &t.ConflictStatus,
			&t.BlockedBy, &t.TouchedFiles,
			&t.BranchName, &t.PRNumber,
			&t.CreatedAt, &t.CompletedAt,
		); err != nil {
			return nil, fmt.Errorf("scan version task: %w", err)
		}
		tasks = append(tasks, t)
	}
	if tasks == nil {
		tasks = []VersionTaskBrief{}
	}
	return tasks, nil
}

// AssignTaskToVersion sets a task's version_id.
func (r *Repository) AssignTaskToVersion(ctx context.Context, taskID, versionID, tenantID int64) error {
	_, err := r.db.Exec(ctx,
		`UPDATE engine.tasks SET version_id = $1 WHERE id = $2 AND tenant_id = $3`,
		versionID, taskID, tenantID,
	)
	return err
}

// UpdateTaskConflict updates conflict status and blocked_by for a task.
func (r *Repository) UpdateTaskConflict(ctx context.Context, taskID, tenantID int64, conflictStatus string, blockedBy []int64) error {
	blockedJSON, _ := json.Marshal(blockedBy)
	_, err := r.db.Exec(ctx,
		`UPDATE engine.tasks SET conflict_status = $1, blocked_by = $2 WHERE id = $3 AND tenant_id = $4`,
		conflictStatus, blockedJSON, taskID, tenantID,
	)
	return err
}

// UpdateTaskTouchedFiles sets the predicted file list from PlannerAgent.
func (r *Repository) UpdateTaskTouchedFiles(ctx context.Context, taskID, tenantID int64, files []string) error {
	filesJSON, _ := json.Marshal(files)
	_, err := r.db.Exec(ctx,
		`UPDATE engine.tasks SET touched_files = $1 WHERE id = $2 AND tenant_id = $3`,
		filesJSON, taskID, tenantID,
	)
	return err
}

// Ensure pgx.ErrNoRows is usable for callers
var ErrNotFound = pgx.ErrNoRows
