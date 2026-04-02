package artifact

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

func (r *Repository) ListByProject(ctx context.Context, projectID int64) ([]Artifact, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, tenant_id, project_id, task_id, name, version,
		       artifact_type, registry_url, size_bytes, checksum,
		       metadata, status, created_at
		FROM pipeline.artifacts
		WHERE project_id = $1
		ORDER BY created_at DESC`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var artifacts []Artifact
	for rows.Next() {
		var a Artifact
		if err := rows.Scan(
			&a.ID, &a.TenantID, &a.ProjectID, &a.TaskID, &a.Name, &a.Version,
			&a.ArtifactType, &a.RegistryURL, &a.SizeBytes, &a.Checksum,
			&a.Metadata, &a.Status, &a.CreatedAt,
		); err != nil {
			return nil, err
		}
		artifacts = append(artifacts, a)
	}
	return artifacts, rows.Err()
}

func (r *Repository) GetByID(ctx context.Context, id int64) (*Artifact, error) {
	var a Artifact
	err := r.db.QueryRow(ctx, `
		SELECT id, tenant_id, project_id, task_id, name, version,
		       artifact_type, registry_url, size_bytes, checksum,
		       metadata, status, created_at
		FROM pipeline.artifacts
		WHERE id = $1`, id,
	).Scan(
		&a.ID, &a.TenantID, &a.ProjectID, &a.TaskID, &a.Name, &a.Version,
		&a.ArtifactType, &a.RegistryURL, &a.SizeBytes, &a.Checksum,
		&a.Metadata, &a.Status, &a.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &a, nil
}
