package profile

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

func (r *Repository) ListByProject(ctx context.Context, projectID int64) ([]ProfileEntry, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, project_id, profile_key, profile_value, version,
		       scanned_at, created_at, updated_at
		FROM engine.project_profiles
		WHERE project_id = $1
		ORDER BY profile_key`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []ProfileEntry
	for rows.Next() {
		var e ProfileEntry
		if err := rows.Scan(
			&e.ID, &e.ProjectID, &e.ProfileKey, &e.ProfileValue, &e.Version,
			&e.ScannedAt, &e.CreatedAt, &e.UpdatedAt,
		); err != nil {
			return nil, err
		}
		entries = append(entries, e)
	}
	return entries, rows.Err()
}

func (r *Repository) GetByKey(ctx context.Context, projectID int64, key string) (*ProfileEntry, error) {
	var e ProfileEntry
	err := r.db.QueryRow(ctx, `
		SELECT id, project_id, profile_key, profile_value, version,
		       scanned_at, created_at, updated_at
		FROM engine.project_profiles
		WHERE project_id = $1 AND profile_key = $2`, projectID, key,
	).Scan(
		&e.ID, &e.ProjectID, &e.ProfileKey, &e.ProfileValue, &e.Version,
		&e.ScannedAt, &e.CreatedAt, &e.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &e, nil
}

func (r *Repository) Upsert(ctx context.Context, entry *ProfileEntry) error {
	return r.db.QueryRow(ctx, `
		INSERT INTO engine.project_profiles (project_id, profile_key, profile_value, version, scanned_at)
		VALUES ($1, $2, $3::jsonb, 1, NOW())
		ON CONFLICT (project_id, profile_key)
		DO UPDATE SET profile_value = $3::jsonb, version = project_profiles.version + 1, scanned_at = NOW(), updated_at = NOW()
		RETURNING id, version, scanned_at, created_at, updated_at`,
		entry.ProjectID, entry.ProfileKey, entry.ProfileValue,
	).Scan(&entry.ID, &entry.Version, &entry.ScannedAt, &entry.CreatedAt, &entry.UpdatedAt)
}
