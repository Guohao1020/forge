package activity

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"

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

// EnsureDraftVersionInput is the input for the EnsureDraftVersion activity.
type EnsureDraftVersionInput struct {
	TenantID  int64  `json:"tenant_id"`
	ProjectID int64  `json:"project_id"`
	CreatedBy int64  `json:"created_by"`
	TaskType  string `json:"task_type"` // "fix", "feature", "breaking" — from AI analysis
}

// EnsureDraftVersionOutput is the result of EnsureDraftVersion.
type EnsureDraftVersionOutput struct {
	VersionID int64  `json:"version_id"`
	Version   string `json:"version"`
	IsNew     bool   `json:"is_new"`
}

// EnsureDraftVersion finds or creates a draft version for the project.
// Uses semantic versioning: fix->patch, feature->minor, breaking->major.
func (a *VersionActivities) EnsureDraftVersion(ctx context.Context, input EnsureDraftVersionInput) (*EnsureDraftVersionOutput, error) {
	slog.Info("EnsureDraftVersion", "project_id", input.ProjectID, "task_type", input.TaskType)

	// 1. Check for existing draft version (PLANNING or IN_PROGRESS)
	var existingID int64
	var existingVer string
	err := a.db.QueryRow(ctx,
		`SELECT id, version FROM engine.project_versions
		 WHERE project_id = $1 AND tenant_id = $2 AND status IN ('PLANNING', 'IN_PROGRESS')
		 ORDER BY created_at DESC LIMIT 1`,
		input.ProjectID, input.TenantID,
	).Scan(&existingID, &existingVer)
	if err == nil {
		return &EnsureDraftVersionOutput{VersionID: existingID, Version: existingVer, IsNew: false}, nil
	}

	// 2. Find latest released version to determine next version
	var latestVer string
	err = a.db.QueryRow(ctx,
		`SELECT version FROM engine.project_versions
		 WHERE project_id = $1 AND tenant_id = $2 AND status = 'RELEASED'
		 ORDER BY released_at DESC LIMIT 1`,
		input.ProjectID, input.TenantID,
	).Scan(&latestVer)

	var nextVer string
	if err != nil {
		// No versions at all — start at v0.1.0
		nextVer = "v0.1.0"
	} else {
		nextVer = bumpVersion(latestVer, input.TaskType)
	}

	// 3. Create new version (ON CONFLICT handles race condition)
	var newID int64
	err = a.db.QueryRow(ctx,
		`INSERT INTO engine.project_versions (tenant_id, project_id, version, status, description, created_by)
		 VALUES ($1, $2, $3, 'PLANNING', '', $4)
		 ON CONFLICT (project_id, version) DO UPDATE SET updated_at = NOW()
		 RETURNING id`,
		input.TenantID, input.ProjectID, nextVer, input.CreatedBy,
	).Scan(&newID)
	if err != nil {
		return nil, fmt.Errorf("create draft version: %w", err)
	}

	slog.Info("draft version created", "version_id", newID, "version", nextVer, "project_id", input.ProjectID)
	return &EnsureDraftVersionOutput{VersionID: newID, Version: nextVer, IsNew: true}, nil
}

// bumpVersion increments a semantic version based on task type.
// "fix" -> patch (v1.2.3 -> v1.2.4)
// "feature" -> minor (v1.2.3 -> v1.3.0)
// "breaking" -> major (v1.2.3 -> v2.0.0)
// unknown -> patch (safe default)
func bumpVersion(current, taskType string) string {
	ver := strings.TrimPrefix(current, "v")
	parts := strings.Split(ver, ".")

	major, minor, patch := 0, 1, 0
	if len(parts) >= 1 {
		fmt.Sscanf(parts[0], "%d", &major)
	}
	if len(parts) >= 2 {
		fmt.Sscanf(parts[1], "%d", &minor)
	}
	if len(parts) >= 3 {
		fmt.Sscanf(parts[2], "%d", &patch)
	}

	switch strings.ToLower(taskType) {
	case "breaking", "major":
		major++
		minor = 0
		patch = 0
	case "feature", "feat", "minor":
		minor++
		patch = 0
	default: // "fix", "bugfix", "patch", unknown
		patch++
	}

	return fmt.Sprintf("v%d.%d.%d", major, minor, patch)
}

// AssignTaskToVersionInput is the input for the AssignTaskToVersion activity.
type AssignTaskToVersionInput struct {
	TaskID    int64 `json:"task_id"`
	TenantID  int64 `json:"tenant_id"`
	VersionID int64 `json:"version_id"`
}

// CreateReleaseBranchInput is the input for the CreateReleaseBranch activity.
type CreateReleaseBranchInput struct {
	ProjectID int64  `json:"project_id"`
	TenantID  int64  `json:"tenant_id"`
	Version   string `json:"version"` // e.g., "v1.2.0"
}

// CreateReleaseBranch creates a release/vX.Y.Z branch from the default branch.
func (a *VersionActivities) CreateReleaseBranch(ctx context.Context, input CreateReleaseBranchInput) error {
	// Placeholder — actual implementation requires GitHub client access via AuthTokenProvider
	slog.Info("CreateReleaseBranch", "version", input.Version, "project_id", input.ProjectID)
	return nil
}

// AssignTaskToVersion links a task to a version.
func (a *VersionActivities) AssignTaskToVersion(ctx context.Context, input AssignTaskToVersionInput) error {
	slog.Info("AssignTaskToVersion", "task_id", input.TaskID, "version_id", input.VersionID)
	_, err := a.db.Exec(ctx,
		`UPDATE engine.tasks SET version_id = $1 WHERE id = $2 AND tenant_id = $3`,
		input.VersionID, input.TaskID, input.TenantID,
	)
	if err != nil {
		return fmt.Errorf("assign task to version: %w", err)
	}
	return nil
}
