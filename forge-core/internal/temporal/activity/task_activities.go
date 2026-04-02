package activity

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/shulex/forge/forge-core/internal/module/task"
)

// TaskWorkflowInput is the input to the TaskWorkflow.
type TaskWorkflowInput struct {
	TaskID      int64  `json:"task_id"`
	TenantID    int64  `json:"tenant_id"`
	ProjectID   int64  `json:"project_id"`
	CreatedBy   int64  `json:"created_by"`
	Requirement string `json:"requirement"`
}

type TaskActivities struct {
	db  *pgxpool.Pool
	sse *task.SSEHub
}

func NewTaskActivities(db *pgxpool.Pool, sse *task.SSEHub) *TaskActivities {
	return &TaskActivities{db: db, sse: sse}
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

// ExecuteStep marks the step as RUNNING (actual work is done by AI activities).
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

	if a.sse != nil {
		a.sse.Broadcast(input.TaskID, task.TaskProgressEvent{
			Type:       "step_progress",
			TaskID:     input.TaskID,
			Status:     input.TaskStatus,
			StepType:   input.StepType,
			StepStatus: "RUNNING",
		})
	}

	slog.Info("step marked running", "task_id", input.TaskID, "step", input.StepType)
	return &StepOutput{TaskID: input.TaskID, StepType: input.StepType, Status: "RUNNING"}, nil
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

	if a.sse != nil {
		a.sse.Broadcast(taskID, task.TaskProgressEvent{
			Type:     "task_complete",
			TaskID:   taskID,
			Status:   "COMPLETED",
			Progress: 100,
		})
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

	if a.sse != nil {
		a.sse.Broadcast(taskID, task.TaskProgressEvent{
			Type:   "task_failed",
			TaskID: taskID,
			Status: "FAILED",
			Data:   map[string]string{"error": errMsg},
		})
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

	if a.sse != nil {
		a.sse.Broadcast(taskID, task.TaskProgressEvent{
			Type:       "step_progress",
			TaskID:     taskID,
			StepType:   stepType,
			StepStatus: "COMPLETED",
		})
	}

	slog.Info("step output saved", "task_id", taskID, "step", stepType)
	return nil
}
