package preview

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

func (r *Repository) Create(ctx context.Context, env *PreviewEnvironment) (*PreviewEnvironment, error) {
	err := r.db.QueryRow(ctx, `
		INSERT INTO pipeline.preview_environments
			(tenant_id, project_id, task_id, branch_name, pr_number, preview_url, status, namespace, expires_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		RETURNING id, created_at, updated_at`,
		env.TenantID, env.ProjectID, env.TaskID, env.BranchName, env.PRNumber,
		env.PreviewURL, env.Status, env.Namespace, env.ExpiresAt,
	).Scan(&env.ID, &env.CreatedAt, &env.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return env, nil
}

func (r *Repository) ListByProject(ctx context.Context, projectID int64) ([]PreviewEnvironment, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, tenant_id, project_id, task_id, branch_name, pr_number,
		       preview_url, status, namespace, expires_at, created_at, updated_at
		FROM pipeline.preview_environments
		WHERE project_id = $1 AND status != 'DESTROYED'
		ORDER BY created_at DESC`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var envs []PreviewEnvironment
	for rows.Next() {
		var e PreviewEnvironment
		if err := rows.Scan(
			&e.ID, &e.TenantID, &e.ProjectID, &e.TaskID, &e.BranchName, &e.PRNumber,
			&e.PreviewURL, &e.Status, &e.Namespace, &e.ExpiresAt, &e.CreatedAt, &e.UpdatedAt,
		); err != nil {
			return nil, err
		}
		envs = append(envs, e)
	}
	return envs, rows.Err()
}

func (r *Repository) GetByTaskID(ctx context.Context, taskID int64) (*PreviewEnvironment, error) {
	var e PreviewEnvironment
	err := r.db.QueryRow(ctx, `
		SELECT id, tenant_id, project_id, task_id, branch_name, pr_number,
		       preview_url, status, namespace, expires_at, created_at, updated_at
		FROM pipeline.preview_environments
		WHERE task_id = $1 AND status != 'DESTROYED'
		ORDER BY created_at DESC LIMIT 1`, taskID,
	).Scan(
		&e.ID, &e.TenantID, &e.ProjectID, &e.TaskID, &e.BranchName, &e.PRNumber,
		&e.PreviewURL, &e.Status, &e.Namespace, &e.ExpiresAt, &e.CreatedAt, &e.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &e, nil
}

func (r *Repository) UpdateStatus(ctx context.Context, id int64, status string) error {
	_, err := r.db.Exec(ctx,
		`UPDATE pipeline.preview_environments SET status = $1, updated_at = NOW() WHERE id = $2`,
		status, id)
	return err
}

func (r *Repository) Delete(ctx context.Context, id int64) error {
	_, err := r.db.Exec(ctx,
		`UPDATE pipeline.preview_environments SET status = 'DESTROYED', updated_at = NOW() WHERE id = $1`,
		id)
	return err
}

// CreateFromActivity is used by the Temporal activity to insert a preview record.
func (r *Repository) CreateFromActivity(ctx context.Context, tenantID, projectID, taskID int64, branchName string, prNumber int, previewURL, namespace string, expiresAt time.Time) error {
	_, err := r.db.Exec(ctx,
		`INSERT INTO pipeline.preview_environments
			(tenant_id, project_id, task_id, branch_name, pr_number, preview_url, status, namespace, expires_at)
		 VALUES ($1, $2, $3, $4, $5, $6, 'READY', $7, $8)
		 ON CONFLICT DO NOTHING`,
		tenantID, projectID, taskID, branchName, prNumber, previewURL, namespace, expiresAt)
	return err
}
