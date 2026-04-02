package testresult

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

func (r *Repository) ListByTask(ctx context.Context, taskID int64) ([]TestResult, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, task_id, layer, framework, total_cases, passed, failed, skipped,
		       coverage_pct, duration_ms, report, status, created_at
		FROM engine.test_results
		WHERE task_id = $1
		ORDER BY created_at DESC`, taskID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []TestResult
	for rows.Next() {
		var tr TestResult
		if err := rows.Scan(
			&tr.ID, &tr.TaskID, &tr.Layer, &tr.Framework,
			&tr.TotalCases, &tr.Passed, &tr.Failed, &tr.Skipped,
			&tr.CoveragePct, &tr.DurationMs, &tr.Report, &tr.Status, &tr.CreatedAt,
		); err != nil {
			return nil, err
		}
		results = append(results, tr)
	}
	return results, rows.Err()
}

func (r *Repository) Create(ctx context.Context, req CreateTestResultRequest) (*TestResult, error) {
	var tr TestResult
	err := r.db.QueryRow(ctx, `
		INSERT INTO engine.test_results (task_id, layer, framework, total_cases, passed, failed, skipped,
		                                  coverage_pct, duration_ms, report, status)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		RETURNING id, task_id, layer, framework, total_cases, passed, failed, skipped,
		          coverage_pct, duration_ms, report, status, created_at`,
		req.TaskID, req.Layer, req.Framework, req.TotalCases, req.Passed, req.Failed, req.Skipped,
		req.CoveragePct, req.DurationMs, req.Report, req.Status,
	).Scan(
		&tr.ID, &tr.TaskID, &tr.Layer, &tr.Framework,
		&tr.TotalCases, &tr.Passed, &tr.Failed, &tr.Skipped,
		&tr.CoveragePct, &tr.DurationMs, &tr.Report, &tr.Status, &tr.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &tr, nil
}
