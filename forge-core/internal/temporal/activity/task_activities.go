package activity

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// TaskWorkflowInput is the input to the TaskWorkflow.
type TaskWorkflowInput struct {
	TaskID    int64 `json:"task_id"`
	TenantID  int64 `json:"tenant_id"`
	ProjectID int64 `json:"project_id"`
}

type TaskActivities struct {
	db *pgxpool.Pool
}

func NewTaskActivities(db *pgxpool.Pool) *TaskActivities {
	return &TaskActivities{db: db}
}

type StepInput struct {
	TaskID     int64  `json:"task_id"`
	StepType   string `json:"step_type"`
	TaskStatus string `json:"task_status"`
	Duration   int    `json:"duration"`
}

type StepOutput struct {
	TaskID   int64  `json:"task_id"`
	StepType string `json:"step_type"`
	Status   string `json:"status"`
}

// ExecuteStep is the generic skeleton activity for all workflow steps.
func (a *TaskActivities) ExecuteStep(ctx context.Context, input StepInput) (*StepOutput, error) {
	slog.Info("step started", "task_id", input.TaskID, "step", input.StepType, "status", input.TaskStatus)

	_, err := a.db.Exec(ctx,
		`UPDATE engine.tasks SET status = $1, updated_at = NOW() WHERE id = $2`,
		input.TaskStatus, input.TaskID,
	)
	if err != nil {
		return nil, fmt.Errorf("update task status: %w", err)
	}

	_, err = a.db.Exec(ctx,
		`UPDATE engine.task_steps SET status = 'RUNNING', started_at = NOW() WHERE task_id = $1 AND step_type = $2`,
		input.TaskID, input.StepType,
	)
	if err != nil {
		return nil, fmt.Errorf("mark step running: %w", err)
	}

	time.Sleep(time.Duration(input.Duration) * time.Second)

	_, err = a.db.Exec(ctx,
		`UPDATE engine.task_steps
		 SET status = 'COMPLETED', completed_at = NOW(),
		     duration_ms = EXTRACT(EPOCH FROM (NOW() - started_at)) * 1000
		 WHERE task_id = $1 AND step_type = $2`,
		input.TaskID, input.StepType,
	)
	if err != nil {
		return nil, fmt.Errorf("mark step completed: %w", err)
	}

	slog.Info("step completed", "task_id", input.TaskID, "step", input.StepType)
	return &StepOutput{TaskID: input.TaskID, StepType: input.StepType, Status: "COMPLETED"}, nil
}

// CompleteTask marks the task as COMPLETED.
func (a *TaskActivities) CompleteTask(ctx context.Context, taskID int64) error {
	_, err := a.db.Exec(ctx,
		`UPDATE engine.tasks SET status = 'COMPLETED', completed_at = NOW(), updated_at = NOW() WHERE id = $1`,
		taskID,
	)
	if err != nil {
		return fmt.Errorf("complete task: %w", err)
	}
	slog.Info("task completed", "task_id", taskID)
	return nil
}

// FailTask marks the task as FAILED.
func (a *TaskActivities) FailTask(ctx context.Context, taskID int64, errMsg string) error {
	_, err := a.db.Exec(ctx,
		`UPDATE engine.tasks SET status = 'FAILED', completed_at = NOW(), updated_at = NOW() WHERE id = $1`,
		taskID,
	)
	if err != nil {
		return fmt.Errorf("fail task: %w", err)
	}
	slog.Error("task failed", "task_id", taskID, "error", errMsg)
	return nil
}

// UpdateTaskAnalysis saves the AI analysis result as JSONB on the task.
func (a *TaskActivities) UpdateTaskAnalysis(ctx context.Context, taskID int64, analysis string) error {
	_, err := a.db.Exec(ctx,
		`UPDATE engine.tasks SET analysis = $2::jsonb, updated_at = NOW() WHERE id = $1`,
		taskID, analysis,
	)
	if err != nil {
		return fmt.Errorf("update task analysis: %w", err)
	}
	slog.Info("task analysis updated", "task_id", taskID)
	return nil
}

// SaveStepOutput saves the output of a workflow step.
func (a *TaskActivities) SaveStepOutput(ctx context.Context, taskID int64, stepType string, output map[string]interface{}) error {
	outputJSON, err := json.Marshal(output)
	if err != nil {
		return fmt.Errorf("marshal step output: %w", err)
	}

	_, err = a.db.Exec(ctx,
		`UPDATE engine.task_steps SET output = $3, status = 'COMPLETED',
		    completed_at = NOW(),
		    duration_ms = CASE WHEN started_at IS NOT NULL
		                       THEN EXTRACT(EPOCH FROM (NOW() - started_at)) * 1000
		                       ELSE duration_ms END
		 WHERE task_id = $1 AND step_type = $2`,
		taskID, stepType, string(outputJSON),
	)
	if err != nil {
		return fmt.Errorf("save step output: %w", err)
	}
	slog.Info("step output saved", "task_id", taskID, "step", stepType)
	return nil
}
