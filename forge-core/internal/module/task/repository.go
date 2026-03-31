package task

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

func (r *Repository) Create(ctx context.Context, t *Task) error {
	err := r.db.QueryRow(ctx,
		`INSERT INTO engine.tasks (tenant_id, project_id, title, requirement, source, status, created_by)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)
		 RETURNING id, created_at, updated_at`,
		t.TenantID, t.ProjectID, t.Title, t.Requirement, t.Source, t.Status, t.CreatedBy,
	).Scan(&t.ID, &t.CreatedAt, &t.UpdatedAt)
	if err != nil {
		return fmt.Errorf("create task: %w", err)
	}
	return nil
}

func (r *Repository) FindByID(ctx context.Context, taskID int64) (*Task, error) {
	t := &Task{}
	err := r.db.QueryRow(ctx,
		`SELECT id, tenant_id, project_id, title, requirement, source, status,
		        workflow_id, workflow_run_id, risk_level, risk_score,
		        branch_name, files_changed, lines_added, lines_deleted,
		        created_by, created_at, updated_at, completed_at
		 FROM engine.tasks WHERE id = $1`,
		taskID,
	).Scan(&t.ID, &t.TenantID, &t.ProjectID, &t.Title, &t.Requirement, &t.Source, &t.Status,
		&t.WorkflowID, &t.WorkflowRunID, &t.RiskLevel, &t.RiskScore,
		&t.BranchName, &t.FilesChanged, &t.LinesAdded, &t.LinesDeleted,
		&t.CreatedBy, &t.CreatedAt, &t.UpdatedAt, &t.CompletedAt)
	if err != nil {
		return nil, fmt.Errorf("find task: %w", err)
	}
	return t, nil
}

func (r *Repository) ListByProject(ctx context.Context, projectID int64, status string, offset, limit int) ([]Task, int64, error) {
	countSQL := `SELECT COUNT(*) FROM engine.tasks WHERE project_id = $1`
	args := []interface{}{projectID}
	argIdx := 2

	if status != "" {
		countSQL += fmt.Sprintf(" AND status = $%d", argIdx)
		args = append(args, status)
		argIdx++
	}

	var total int64
	if err := r.db.QueryRow(ctx, countSQL, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count tasks: %w", err)
	}

	listSQL := `SELECT id, tenant_id, project_id, title, requirement, source, status,
	                   workflow_id, workflow_run_id, risk_level, risk_score,
	                   branch_name, files_changed, lines_added, lines_deleted,
	                   created_by, created_at, updated_at, completed_at
	            FROM engine.tasks WHERE project_id = $1`

	listArgs := []interface{}{projectID}
	listArgIdx := 2

	if status != "" {
		listSQL += fmt.Sprintf(" AND status = $%d", listArgIdx)
		listArgs = append(listArgs, status)
		listArgIdx++
	}

	listSQL += " ORDER BY created_at DESC"
	listSQL += fmt.Sprintf(" LIMIT $%d OFFSET $%d", listArgIdx, listArgIdx+1)
	listArgs = append(listArgs, limit, offset)

	rows, err := r.db.Query(ctx, listSQL, listArgs...)
	if err != nil {
		return nil, 0, fmt.Errorf("list tasks: %w", err)
	}
	defer rows.Close()

	var tasks []Task
	for rows.Next() {
		var t Task
		if err := rows.Scan(&t.ID, &t.TenantID, &t.ProjectID, &t.Title, &t.Requirement, &t.Source, &t.Status,
			&t.WorkflowID, &t.WorkflowRunID, &t.RiskLevel, &t.RiskScore,
			&t.BranchName, &t.FilesChanged, &t.LinesAdded, &t.LinesDeleted,
			&t.CreatedBy, &t.CreatedAt, &t.UpdatedAt, &t.CompletedAt); err != nil {
			return nil, 0, fmt.Errorf("scan task: %w", err)
		}
		tasks = append(tasks, t)
	}
	return tasks, total, nil
}

func (r *Repository) UpdateStatus(ctx context.Context, taskID int64, status string) error {
	_, err := r.db.Exec(ctx,
		`UPDATE engine.tasks SET status = $1, updated_at = NOW() WHERE id = $2`,
		status, taskID,
	)
	return err
}

func (r *Repository) UpdateWorkflowIDs(ctx context.Context, taskID int64, workflowID, runID string) error {
	_, err := r.db.Exec(ctx,
		`UPDATE engine.tasks SET workflow_id = $1, workflow_run_id = $2, updated_at = NOW() WHERE id = $3`,
		workflowID, runID, taskID,
	)
	return err
}

func (r *Repository) MarkCompleted(ctx context.Context, taskID int64, status string) error {
	_, err := r.db.Exec(ctx,
		`UPDATE engine.tasks SET status = $1, completed_at = NOW(), updated_at = NOW() WHERE id = $2`,
		status, taskID,
	)
	return err
}

// Task step operations

func (r *Repository) CreateSteps(ctx context.Context, taskID int64, steps []struct {
	Name     string
	StepType string
}) error {
	for _, s := range steps {
		_, err := r.db.Exec(ctx,
			`INSERT INTO engine.task_steps (task_id, name, step_type, status)
			 VALUES ($1, $2, $3, $4)`,
			taskID, s.Name, s.StepType, StepPending,
		)
		if err != nil {
			return fmt.Errorf("create step %s: %w", s.Name, err)
		}
	}
	return nil
}

func (r *Repository) GetStepsByTaskID(ctx context.Context, taskID int64) ([]TaskStep, error) {
	rows, err := r.db.Query(ctx,
		`SELECT id, task_id, name, step_type, status, input, output, error,
		        attempt, max_attempts, started_at, completed_at, duration_ms
		 FROM engine.task_steps WHERE task_id = $1 ORDER BY id`,
		taskID,
	)
	if err != nil {
		return nil, fmt.Errorf("get steps: %w", err)
	}
	defer rows.Close()

	var steps []TaskStep
	for rows.Next() {
		var s TaskStep
		if err := rows.Scan(&s.ID, &s.TaskID, &s.Name, &s.StepType, &s.Status,
			&s.Input, &s.Output, &s.Error,
			&s.Attempt, &s.MaxAttempts, &s.StartedAt, &s.CompletedAt, &s.DurationMs); err != nil {
			return nil, fmt.Errorf("scan step: %w", err)
		}
		steps = append(steps, s)
	}
	return steps, nil
}

func (r *Repository) UpdateStepStatus(ctx context.Context, taskID int64, stepType, status string) error {
	_, err := r.db.Exec(ctx,
		`UPDATE engine.task_steps
		 SET status = $1,
		     started_at = CASE WHEN $1 = 'RUNNING' AND started_at IS NULL THEN NOW() ELSE started_at END,
		     completed_at = CASE WHEN $1 IN ('COMPLETED', 'FAILED', 'SKIPPED') THEN NOW() ELSE completed_at END,
		     duration_ms = CASE WHEN $1 IN ('COMPLETED', 'FAILED', 'SKIPPED') AND started_at IS NOT NULL
		                        THEN EXTRACT(EPOCH FROM (NOW() - started_at)) * 1000
		                        ELSE duration_ms END
		 WHERE task_id = $2 AND step_type = $3`,
		status, taskID, stepType,
	)
	return err
}
