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

// --- Deploy Records ---

func (r *Repository) ListDeployRecords(ctx context.Context, environmentID int64) ([]DeployRecord, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, tenant_id, project_id, environment_id, artifact_id, version,
		       status, deployed_by, started_at, completed_at, k8s_manifest,
		       error_message, created_at
		FROM pipeline.deploy_records
		WHERE environment_id = $1
		ORDER BY created_at DESC
		LIMIT 50`, environmentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var records []DeployRecord
	for rows.Next() {
		var d DeployRecord
		if err := rows.Scan(
			&d.ID, &d.TenantID, &d.ProjectID, &d.EnvironmentID, &d.ArtifactID, &d.Version,
			&d.Status, &d.DeployedBy, &d.StartedAt, &d.CompletedAt, &d.K8sManifest,
			&d.ErrorMessage, &d.CreatedAt,
		); err != nil {
			return nil, err
		}
		records = append(records, d)
	}
	return records, rows.Err()
}

func (r *Repository) CreateDeployRecord(ctx context.Context, d *DeployRecord) error {
	return r.db.QueryRow(ctx, `
		INSERT INTO pipeline.deploy_records
			(tenant_id, project_id, environment_id, artifact_id, version, status, deployed_by, started_at, completed_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		RETURNING id, created_at`,
		d.TenantID, d.ProjectID, d.EnvironmentID, d.ArtifactID, d.Version,
		d.Status, d.DeployedBy, d.StartedAt, d.CompletedAt,
	).Scan(&d.ID, &d.CreatedAt)
}

func (r *Repository) FindPreviousDeploy(ctx context.Context, envID, tenantID int64) (version string, artifactID *int64, err error) {
	err = r.db.QueryRow(ctx,
		`SELECT version, artifact_id FROM pipeline.deploy_records
		 WHERE environment_id = $1 AND tenant_id = $2 AND status IN ('DEPLOYED', 'SIMULATED')
		 ORDER BY completed_at DESC
		 OFFSET 1 LIMIT 1`,
		envID, tenantID,
	).Scan(&version, &artifactID)
	return
}

func (r *Repository) CreateRollbackRecord(ctx context.Context, d *DeployRecord) error {
	return r.db.QueryRow(ctx,
		`INSERT INTO pipeline.deploy_records (tenant_id, project_id, environment_id, artifact_id, version, status, deployed_by, started_at)
		 VALUES ($1, $2, $3, $4, $5, 'ROLLED_BACK', $6, NOW())
		 RETURNING id, started_at, created_at`,
		d.TenantID, d.ProjectID, d.EnvironmentID, d.ArtifactID, d.Version, d.DeployedBy,
	).Scan(&d.ID, &d.StartedAt, &d.CreatedAt)
}

func (r *Repository) UpdateEnvironmentVersion(ctx context.Context, envID int64, version string) error {
	_, err := r.db.Exec(ctx,
		`UPDATE pipeline.environments SET current_version = $1, last_deploy_at = NOW() WHERE id = $2`,
		version, envID,
	)
	return err
}

func (r *Repository) UpdateDeployRecord(ctx context.Context, d *DeployRecord) error {
	_, err := r.db.Exec(ctx, `
		UPDATE pipeline.deploy_records
		SET status = $2, completed_at = $3, error_message = $4
		WHERE id = $1`,
		d.ID, d.Status, d.CompletedAt, d.ErrorMessage,
	)
	return err
}
