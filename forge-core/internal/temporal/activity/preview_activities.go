package activity

import (
	"context"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// PreviewActivities handles preview environment lifecycle operations.
type PreviewActivities struct {
	db *pgxpool.Pool
}

func NewPreviewActivities(db *pgxpool.Pool) *PreviewActivities {
	return &PreviewActivities{db: db}
}

// UpdatePreviewStatus updates the status, URL, and namespace of a preview environment.
func (a *PreviewActivities) UpdatePreviewStatus(ctx context.Context, previewID int64, status, previewURL, namespace string) error {
	slog.Info("UpdatePreviewStatus",
		"preview_id", previewID,
		"status", status,
		"url", previewURL,
	)

	var expiresAt *time.Time
	if status == "READY" {
		t := time.Now().Add(30 * time.Minute)
		expiresAt = &t
	}

	_, err := a.db.Exec(ctx,
		`UPDATE pipeline.preview_environments
		 SET status = $1, preview_url = $2, namespace = $3, expires_at = $4, updated_at = NOW()
		 WHERE id = $5`,
		status, previewURL, namespace, expiresAt, previewID,
	)
	return err
}
