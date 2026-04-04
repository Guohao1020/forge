package workflow

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"path/filepath"
	"strings"
	"time"

	"go.temporal.io/sdk/workflow"
)

// ---------------------------------------------------------------------------
// Signal types
// ---------------------------------------------------------------------------

// NewTaskSignal is sent when a new task is created under a version.
type NewTaskSignal struct {
	TaskID       int64    `json:"taskId"`
	Title        string   `json:"title"`
	TouchedFiles []string `json:"touchedFiles"` // predicted files from PlannerAgent
}

// TaskCompletedSignal is sent when a task finishes (COMPLETED status).
type TaskCompletedSignal struct {
	TaskID int64 `json:"taskId"`
}

// TaskFailedSignal is sent when a task fails.
type TaskFailedSignal struct {
	TaskID int64 `json:"taskId"`
}

// TaskFilesUpdatedSignal is sent after PlannerAgent outputs touched_files.
type TaskFilesUpdatedSignal struct {
	TaskID       int64    `json:"taskId"`
	TouchedFiles []string `json:"touchedFiles"`
}

// ---------------------------------------------------------------------------
// Orchestrator state
// ---------------------------------------------------------------------------

type taskState struct {
	TaskID       int64    `json:"taskId"`
	Title        string   `json:"title"`
	Status       string   `json:"status"` // "RUNNING", "WAITING", "COMPLETED", "FAILED"
	TouchedFiles []string `json:"touchedFiles"`
	BlockedBy    []int64  `json:"blockedBy"`
}

// VersionOrchestratorInput is the input to the VersionOrchestrator workflow.
type VersionOrchestratorInput struct {
	VersionID   int64                 `json:"versionId"`
	TenantID    int64                 `json:"tenantId"`
	ProjectID   int64                 `json:"projectId"`
	ActiveTasks map[int64]*taskState  `json:"activeTasks"` // restored from ContinueAsNew
}

// ---------------------------------------------------------------------------
// VersionOrchestrator workflow
// ---------------------------------------------------------------------------

// VersionOrchestrator coordinates all tasks within a project version.
//
// Lifecycle: version created → tasks arrive via signals → conflict detection →
// tasks execute or wait → all complete → workflow ends.
//
// Uses ContinueAsNew every 50 events to prevent Temporal history bloat.
func VersionOrchestrator(ctx workflow.Context, input VersionOrchestratorInput) error {
	logger := workflow.GetLogger(ctx)
	logger.Info("VersionOrchestrator started",
		"version_id", input.VersionID,
		"active_tasks", len(input.ActiveTasks),
	)

	// Initialize state
	if input.ActiveTasks == nil {
		input.ActiveTasks = make(map[int64]*taskState)
	}

	// Signal channels
	newTaskCh := workflow.GetSignalChannel(ctx, "new_task")
	completedCh := workflow.GetSignalChannel(ctx, "task_completed")
	failedCh := workflow.GetSignalChannel(ctx, "task_failed")
	filesUpdatedCh := workflow.GetSignalChannel(ctx, "task_files_updated")
	cancelCh := workflow.GetSignalChannel(ctx, "cancel_version")

	eventCount := 0

	for {
		selector := workflow.NewSelector(ctx)

		// --- New task arrives ---
		selector.AddReceive(newTaskCh, func(ch workflow.ReceiveChannel, more bool) {
			var signal NewTaskSignal
			ch.Receive(ctx, &signal)
			eventCount++

			logger.Info("New task signal received",
				"task_id", signal.TaskID,
				"title", signal.Title,
				"touched_files", len(signal.TouchedFiles),
			)

			// Detect conflicts with active running/waiting tasks
			blockedBy := detectConflicts(signal.TouchedFiles, input.ActiveTasks)

			if len(blockedBy) == 0 {
				// No conflict — task can proceed
				input.ActiveTasks[signal.TaskID] = &taskState{
					TaskID:       signal.TaskID,
					Title:        signal.Title,
					Status:       "RUNNING",
					TouchedFiles: signal.TouchedFiles,
				}
				logger.Info("Task can proceed (no conflicts)", "task_id", signal.TaskID)
			} else {
				// Has conflicts — task must wait
				input.ActiveTasks[signal.TaskID] = &taskState{
					TaskID:       signal.TaskID,
					Title:        signal.Title,
					Status:       "WAITING",
					TouchedFiles: signal.TouchedFiles,
					BlockedBy:    blockedBy,
				}
				logger.Info("Task blocked by conflicts",
					"task_id", signal.TaskID,
					"blocked_by", blockedBy,
				)
				// Persist conflict status via activity
				persistConflictStatus(ctx, signal.TaskID, input.TenantID, "WAITING", blockedBy)
			}
		})

		// --- Task completed ---
		selector.AddReceive(completedCh, func(ch workflow.ReceiveChannel, more bool) {
			var signal TaskCompletedSignal
			ch.Receive(ctx, &signal)
			eventCount++

			logger.Info("Task completed signal", "task_id", signal.TaskID)
			delete(input.ActiveTasks, signal.TaskID)

			// Unblock waiting tasks
			unblockDependents(ctx, input.ActiveTasks, signal.TaskID, input.TenantID, logger)

			// Check if all tasks are done (no active tasks left)
			if len(input.ActiveTasks) == 0 {
				logger.Info("All tasks completed for version", "version_id", input.VersionID)
				updateVersionStatus(ctx, input.VersionID, input.TenantID, "TESTING")
				// Workflow ends naturally — all tasks done
			}
		})

		// --- Task failed ---
		selector.AddReceive(failedCh, func(ch workflow.ReceiveChannel, more bool) {
			var signal TaskFailedSignal
			ch.Receive(ctx, &signal)
			eventCount++

			logger.Info("Task failed signal", "task_id", signal.TaskID)
			delete(input.ActiveTasks, signal.TaskID)

			// Unblock tasks that were waiting on this failed task
			// (conflict was speculative, let them try)
			unblockDependents(ctx, input.ActiveTasks, signal.TaskID, input.TenantID, logger)
		})

		// --- Task files updated (after planning) ---
		selector.AddReceive(filesUpdatedCh, func(ch workflow.ReceiveChannel, more bool) {
			var signal TaskFilesUpdatedSignal
			ch.Receive(ctx, &signal)
			eventCount++

			if state, ok := input.ActiveTasks[signal.TaskID]; ok {
				state.TouchedFiles = signal.TouchedFiles
				logger.Info("Task files updated", "task_id", signal.TaskID, "files", len(signal.TouchedFiles))
			}
		})

		// --- Cancel version ---
		selector.AddReceive(cancelCh, func(ch workflow.ReceiveChannel, more bool) {
			ch.Receive(ctx, nil)
			logger.Info("Version cancelled", "version_id", input.VersionID)
			updateVersionStatus(ctx, input.VersionID, input.TenantID, "CANCELLED")
		})

		// Add a timeout to prevent the workflow from hanging forever
		// if no signals arrive. Check every 5 minutes.
		timerFuture := workflow.NewTimer(ctx, 5*time.Minute)
		selector.AddFuture(timerFuture, func(f workflow.Future) {
			// Timer fired — check if we should end
			if len(input.ActiveTasks) == 0 {
				logger.Info("No active tasks and idle timeout, ending orchestrator")
			}
		})

		selector.Select(ctx)

		// Check termination conditions
		if len(input.ActiveTasks) == 0 {
			logger.Info("VersionOrchestrator ending (no active tasks)")
			return nil
		}

		// ContinueAsNew to prevent history bloat (every 50 events)
		if eventCount >= 50 {
			logger.Info("ContinueAsNew triggered", "events", eventCount)
			return workflow.NewContinueAsNewError(ctx, VersionOrchestrator, VersionOrchestratorInput{
				VersionID:   input.VersionID,
				TenantID:    input.TenantID,
				ProjectID:   input.ProjectID,
				ActiveTasks: input.ActiveTasks,
			})
		}
	}
}

// ---------------------------------------------------------------------------
// Conflict detection
// ---------------------------------------------------------------------------

// detectConflicts checks if a new task's files overlap with any active task.
// Returns list of blocking task IDs.
func detectConflicts(newFiles []string, activeTasks map[int64]*taskState) []int64 {
	if len(newFiles) == 0 {
		return nil // no file info yet — can't detect conflicts
	}

	var blockers []int64

	for taskID, state := range activeTasks {
		if state.Status != "RUNNING" && state.Status != "WAITING" {
			continue
		}
		if len(state.TouchedFiles) == 0 {
			continue
		}

		// Check exact file overlap
		if hasFileOverlap(newFiles, state.TouchedFiles) {
			blockers = append(blockers, taskID)
			continue
		}

		// Check package-level overlap (same directory = warning)
		if hasPackageOverlap(newFiles, state.TouchedFiles) {
			// Package overlap is a WARNING, not a BLOCK
			// Log it but don't add to blockers
			slog.Warn("Package-level overlap detected",
				"new_files", newFiles,
				"existing_task", taskID,
				"existing_files", state.TouchedFiles,
			)
		}
	}

	return blockers
}

// hasFileOverlap checks if any file path appears in both lists.
func hasFileOverlap(a, b []string) bool {
	set := make(map[string]bool, len(a))
	for _, f := range a {
		set[strings.ToLower(f)] = true
	}
	for _, f := range b {
		if set[strings.ToLower(f)] {
			return true
		}
	}
	return false
}

// hasPackageOverlap checks if any files share the same parent directory.
func hasPackageOverlap(a, b []string) bool {
	dirsA := make(map[string]bool)
	for _, f := range a {
		dirsA[strings.ToLower(filepath.Dir(f))] = true
	}
	for _, f := range b {
		if dirsA[strings.ToLower(filepath.Dir(f))] {
			return true
		}
	}
	return false
}

// ---------------------------------------------------------------------------
// Unblocking
// ---------------------------------------------------------------------------

// unblockDependents removes a completed/failed task from all waiting tasks'
// blockedBy lists. If a task's blockedBy becomes empty, it transitions to RUNNING.
func unblockDependents(
	ctx workflow.Context,
	activeTasks map[int64]*taskState,
	completedTaskID int64,
	tenantID int64,
	logger interface{ Info(string, ...interface{}) },
) {
	for taskID, state := range activeTasks {
		if state.Status != "WAITING" {
			continue
		}

		// Remove the completed task from blockedBy
		newBlockedBy := make([]int64, 0, len(state.BlockedBy))
		for _, blocker := range state.BlockedBy {
			if blocker != completedTaskID {
				newBlockedBy = append(newBlockedBy, blocker)
			}
		}
		state.BlockedBy = newBlockedBy

		if len(state.BlockedBy) == 0 {
			// Unblocked — task can now proceed
			state.Status = "RUNNING"
			logger.Info("Task unblocked", "task_id", taskID, "unblocked_by", completedTaskID)
			persistConflictStatus(ctx, taskID, tenantID, "RESOLVED", nil)
		}
	}
}

// ---------------------------------------------------------------------------
// Activity calls (side effects)
// ---------------------------------------------------------------------------

// persistConflictStatus updates the task's conflict_status and blocked_by in DB.
func persistConflictStatus(ctx workflow.Context, taskID, tenantID int64, status string, blockedBy []int64) {
	actCtx := workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
		StartToCloseTimeout: 10 * time.Second,
	})
	input := map[string]interface{}{
		"task_id":         taskID,
		"tenant_id":       tenantID,
		"conflict_status": status,
		"blocked_by":      blockedBy,
	}
	_ = workflow.ExecuteActivity(actCtx, "UpdateTaskConflict", input).Get(ctx, nil)
}

// updateVersionStatus updates the version status in DB.
func updateVersionStatus(ctx workflow.Context, versionID, tenantID int64, status string) {
	actCtx := workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
		StartToCloseTimeout: 10 * time.Second,
	})
	input := map[string]interface{}{
		"version_id": versionID,
		"tenant_id":  tenantID,
		"status":     status,
	}
	_ = workflow.ExecuteActivity(actCtx, "UpdateVersionStatus", input).Get(ctx, nil)
}

// ---------------------------------------------------------------------------
// Helper: JSON marshal for logging
// ---------------------------------------------------------------------------

func marshalJSON(v interface{}) string {
	b, err := json.Marshal(v)
	if err != nil {
		return fmt.Sprintf("%v", v)
	}
	return string(b)
}
