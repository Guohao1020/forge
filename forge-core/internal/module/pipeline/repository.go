package pipeline

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

func (r *Repository) ListByProject(ctx context.Context, projectID int64) ([]Environment, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, tenant_id, project_id, name, env_type, status,
		       current_version, last_deploy_at, created_at, updated_at
		FROM pipeline.environments
		WHERE project_id = $1
		ORDER BY CASE env_type
			WHEN 'DEV' THEN 1
			WHEN 'STAGING' THEN 2
			WHEN 'PROD' THEN 3
		END`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var envs []Environment
	for rows.Next() {
		var e Environment
		if err := rows.Scan(
			&e.ID, &e.TenantID, &e.ProjectID, &e.Name, &e.EnvType, &e.Status,
			&e.CurrentVersion, &e.LastDeployAt, &e.CreatedAt, &e.UpdatedAt,
		); err != nil {
			return nil, err
		}
		envs = append(envs, e)
	}
	return envs, rows.Err()
}

func (r *Repository) GetByID(ctx context.Context, id int64) (*Environment, error) {
	var e Environment
	err := r.db.QueryRow(ctx, `
		SELECT id, tenant_id, project_id, name, env_type, status,
		       current_version, last_deploy_at, created_at, updated_at
		FROM pipeline.environments
		WHERE id = $1`, id,
	).Scan(
		&e.ID, &e.TenantID, &e.ProjectID, &e.Name, &e.EnvType, &e.Status,
		&e.CurrentVersion, &e.LastDeployAt, &e.CreatedAt, &e.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &e, nil
}
