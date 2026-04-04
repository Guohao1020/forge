package activity

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/jackc/pgx/v5/pgxpool"
	"go.temporal.io/sdk/activity"
)

// VersionActivities contains activities for the VersionOrchestrator workflow.
type VersionActivities struct {
	db *pgxpool.Pool
}

func NewVersionActivities(db *pgxpool.Pool) *VersionActivities {
	return &VersionActivities{db: db}
}

// UpdateTaskConflictInput is the input for UpdateTaskConflict activity.
type UpdateTaskConflictInput struct {
	TaskID         int64   `json:"task_id"`
	TenantID       int64   `json:"tenant_id"`
	ConflictStatus string  `json:"conflict_status"`
	BlockedBy      []int64 `json:"blocked_by"`
}

// UpdateTaskConflict updates the conflict_status and blocked_by fields for a task.
func (a *VersionActivities) UpdateTaskConflict(ctx context.Context, input UpdateTaskConflictInput) error {
	info := activity.GetInfo(ctx)
	slog.Info("UpdateTaskConflict activity",
		"task_id", input.TaskID,
		"conflict_status", input.ConflictStatus,
		"workflow_id", info.WorkflowExecution.ID,
	)

	blockedJSON, _ := json.Marshal(input.BlockedBy)
	if input.BlockedBy == nil {
		blockedJSON = []byte("[]")
	}

	_, err := a.db.Exec(ctx,
		`UPDATE engine.tasks
		 SET conflict_status = $1, blocked_by = $2
		 WHERE id = $3 AND tenant_id = $4`,
		input.ConflictStatus, blockedJSON, input.TaskID, input.TenantID,
	)
	if err != nil {
		return fmt.Errorf("update task conflict: %w", err)
	}
	return nil
}

// UpdateVersionStatusInput is the input for UpdateVersionStatus activity.
type UpdateVersionStatusInput struct {
	VersionID int64  `json:"version_id"`
	TenantID  int64  `json:"tenant_id"`
	Status    string `json:"status"`
}

// UpdateVersionStatus updates the status of a project version.
func (a *VersionActivities) UpdateVersionStatus(ctx context.Context, input UpdateVersionStatusInput) error {
	info := activity.GetInfo(ctx)
	slog.Info("UpdateVersionStatus activity",
		"version_id", input.VersionID,
		"status", input.Status,
		"workflow_id", info.WorkflowExecution.ID,
	)

	_, err := a.db.Exec(ctx,
		`UPDATE engine.project_versions
		 SET status = $1, updated_at = NOW()
		 WHERE id = $2 AND tenant_id = $3`,
		input.Status, input.VersionID, input.TenantID,
	)
	if err != nil {
		return fmt.Errorf("update version status: %w", err)
	}
	return nil
}

// SaveTouchedFiles stores the predicted file list (from PlannerAgent) to the task record.
func (a *VersionActivities) SaveTouchedFiles(ctx context.Context, taskID, tenantID int64, files []string) error {
	filesJSON, _ := json.Marshal(files)
	_, err := a.db.Exec(ctx,
		`UPDATE engine.tasks SET touched_files = $1 WHERE id = $2 AND tenant_id = $3`,
		filesJSON, taskID, tenantID,
	)
	if err != nil {
		return fmt.Errorf("save touched files: %w", err)
	}
	slog.Info("SaveTouchedFiles", "task_id", taskID, "file_count", len(files))
	return nil
}
