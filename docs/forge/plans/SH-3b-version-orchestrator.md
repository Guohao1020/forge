# SH-3b — 版本协调器（VersionOrchestrator + 冲突检测）

## 目标

实现 Temporal 长运行 Workflow，协调同一版本内多个任务的并发执行：

1. **VersionOrchestrator** — 每个版本一个长运行 Workflow，管理任务的注册、完成、失败信号
2. **冲突检测** — 基于 touched_files 的文件级冲突检测 + package 级启发式告警
3. **SSE 集成** — 冲突状态变更实时推送到前端
4. **任务创建流程集成** — 带 version_id 的任务通过 VersionOrchestrator 协调启动

完成后：同一版本内创建 3 个任务，2 个有文件重叠时，后创建的任务自动标记为 BLOCKED，前序任务完成后自动解除阻塞。

## 前置依赖

- **SH-3a 完成** — project_versions 表 + tasks 表扩展字段 + version module CRUD
- Temporal SDK 已集成（forge-core/internal/temporal/）
- SSEHub 已实现（task module）
- 现有 TaskWorkflow / TaskExecutionWorkflow 正常运行

## 工期

3 天

---

## Day 1 — VersionOrchestrator Workflow

### 1.1 新建 `forge-core/internal/temporal/workflow/version_workflow.go`

```go
package workflow

import (
	"fmt"
	"time"

	"go.temporal.io/sdk/workflow"
)

// VersionOrchestratorInput is the input to start a VersionOrchestrator.
type VersionOrchestratorInput struct {
	VersionID int64 `json:"version_id"`
	ProjectID int64 `json:"project_id"`
	TenantID  int64 `json:"tenant_id"`
}

// VersionTaskSignal is sent when a new task is added to the version.
type VersionTaskSignal struct {
	TaskID       int64    `json:"task_id"`
	Title        string   `json:"title"`
	Requirement  string   `json:"requirement"`
	CreatedBy    int64    `json:"created_by"`
	TouchedFiles []string `json:"touched_files"` // May be empty initially, populated after PLAN step
}

// TaskCompletedSignal is sent when a task within the version completes.
type TaskCompletedSignal struct {
	TaskID       int64    `json:"task_id"`
	TouchedFiles []string `json:"touched_files"` // Final list of files modified
	Success      bool     `json:"success"`
}

// TaskFilesUpdatedSignal is sent after the PLAN step populates touched_files.
type TaskFilesUpdatedSignal struct {
	TaskID       int64    `json:"task_id"`
	TouchedFiles []string `json:"touched_files"`
}

// CancelVersionSignal requests cancellation of the entire version.
type CancelVersionSignal struct {
	Reason string `json:"reason"`
}

// Signal channel names
const (
	SignalNewTask          = "new_task"
	SignalTaskCompleted    = "task_completed"
	SignalTaskFilesUpdated = "task_files_updated"
	SignalCancelVersion    = "cancel_version"
)

// trackedTask holds the state of a task within the orchestrator.
type trackedTask struct {
	TaskID         int64
	Title          string
	Requirement    string
	CreatedBy      int64
	TouchedFiles   []string
	Status         string   // "PENDING", "ACTIVE", "COMPLETED", "FAILED", "BLOCKED"
	ConflictStatus string   // "NONE", "WARNING", "BLOCKED"
	BlockedBy      []int64  // Task IDs that block this task
	WorkflowStarted bool
}

// VersionOrchestrator is a long-running Temporal workflow that coordinates
// all tasks within a version. It:
//   - Receives new tasks via signal
//   - Detects file conflicts between concurrent tasks
//   - Blocks conflicting tasks until predecessors complete
//   - Auto-unblocks tasks when their blockers finish
//   - Uses ContinueAsNew every 50 events to avoid history bloat
func VersionOrchestrator(ctx workflow.Context, input VersionOrchestratorInput) error {
	logger := workflow.GetLogger(ctx)
	logger.Info("VersionOrchestrator started", "version_id", input.VersionID)

	// Activity options for DB operations
	actCtx := workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
		StartToCloseTimeout: 30 * time.Second,
		RetryPolicy:         defaultRetryPolicy(),
	})

	// State
	activeTasks := make(map[int64]*trackedTask) // taskID -> state
	eventCount := 0
	cancelled := false

	// Signal channels
	newTaskCh := workflow.GetSignalChannel(ctx, SignalNewTask)
	completedCh := workflow.GetSignalChannel(ctx, SignalTaskCompleted)
	filesUpdatedCh := workflow.GetSignalChannel(ctx, SignalTaskFilesUpdated)
	cancelCh := workflow.GetSignalChannel(ctx, SignalCancelVersion)

	for {
		// ContinueAsNew every 50 events to avoid unbounded history
		if eventCount >= 50 {
			logger.Info("ContinueAsNew triggered", "version_id", input.VersionID, "events", eventCount)
			return workflow.NewContinueAsNewError(ctx, VersionOrchestrator, input)
		}

		// Check if all tasks are done (terminal condition)
		if len(activeTasks) > 0 && allTasksTerminal(activeTasks) {
			logger.Info("all tasks terminal, completing orchestrator", "version_id", input.VersionID)
			// Update version status to indicate readiness for release
			_ = workflow.ExecuteActivity(actCtx, "UpdateVersionStatus",
				input.VersionID, "FROZEN").Get(ctx, nil)
			return nil
		}

		// Wait for any signal
		selector := workflow.NewSelector(ctx)

		// Handle: new task added to version
		selector.AddReceive(newTaskCh, func(ch workflow.ReceiveChannel, more bool) {
			var sig VersionTaskSignal
			ch.Receive(ctx, &sig)
			eventCount++
			logger.Info("new task signal", "task_id", sig.TaskID, "version_id", input.VersionID)

			task := &trackedTask{
				TaskID:       sig.TaskID,
				Title:        sig.Title,
				Requirement:  sig.Requirement,
				CreatedBy:    sig.CreatedBy,
				TouchedFiles: sig.TouchedFiles,
				Status:       "PENDING",
				ConflictStatus: "NONE",
			}
			activeTasks[sig.TaskID] = task

			// Run conflict detection if we have file info
			if len(sig.TouchedFiles) > 0 {
				conflicts := detectConflicts(task, activeTasks)
				applyConflicts(task, conflicts)
			}

			// Start task workflow if not blocked
			if task.ConflictStatus != "BLOCKED" {
				task.Status = "ACTIVE"
				task.WorkflowStarted = true
				startTaskWorkflow(ctx, actCtx, sig, input)
			} else {
				// Persist conflict status
				_ = workflow.ExecuteActivity(actCtx, "UpdateTaskConflictStatus",
					sig.TaskID, task.ConflictStatus, task.BlockedBy).Get(ctx, nil)
				// SSE: notify frontend
				_ = workflow.ExecuteActivity(actCtx, "NotifyConflict",
					input.VersionID, sig.TaskID, task.ConflictStatus, task.BlockedBy, task.TouchedFiles).Get(ctx, nil)
			}
		})

		// Handle: task reports its files after PLAN step
		selector.AddReceive(filesUpdatedCh, func(ch workflow.ReceiveChannel, more bool) {
			var sig TaskFilesUpdatedSignal
			ch.Receive(ctx, &sig)
			eventCount++
			logger.Info("task files updated", "task_id", sig.TaskID)

			if task, ok := activeTasks[sig.TaskID]; ok {
				task.TouchedFiles = sig.TouchedFiles

				// Re-run conflict detection for this task
				conflicts := detectConflicts(task, activeTasks)
				applyConflicts(task, conflicts)

				// Also check if this task NOW blocks other pending tasks
				for _, other := range activeTasks {
					if other.TaskID == sig.TaskID || other.Status == "COMPLETED" || other.Status == "FAILED" {
						continue
					}
					if len(other.TouchedFiles) > 0 {
						otherConflicts := detectConflicts(other, activeTasks)
						applyConflicts(other, otherConflicts)
					}
				}

				// Persist updated conflict status
				_ = workflow.ExecuteActivity(actCtx, "UpdateTaskConflictStatus",
					sig.TaskID, task.ConflictStatus, task.BlockedBy).Get(ctx, nil)
				// Persist touched files
				_ = workflow.ExecuteActivity(actCtx, "UpdateTaskTouchedFiles",
					sig.TaskID, sig.TouchedFiles).Get(ctx, nil)
			}
		})

		// Handle: task completed or failed
		selector.AddReceive(completedCh, func(ch workflow.ReceiveChannel, more bool) {
			var sig TaskCompletedSignal
			ch.Receive(ctx, &sig)
			eventCount++
			logger.Info("task completed signal", "task_id", sig.TaskID, "success", sig.Success)

			if task, ok := activeTasks[sig.TaskID]; ok {
				if sig.Success {
					task.Status = "COMPLETED"
				} else {
					task.Status = "FAILED"
				}
				if len(sig.TouchedFiles) > 0 {
					task.TouchedFiles = sig.TouchedFiles
				}

				// Check if any blocked tasks can be unblocked
				unblocked := unblockDependents(sig.TaskID, activeTasks)
				for _, unblockedTask := range unblocked {
					logger.Info("task unblocked", "task_id", unblockedTask.TaskID, "was_blocked_by", sig.TaskID)
					unblockedTask.Status = "ACTIVE"
					unblockedTask.ConflictStatus = "RESOLVED"

					// Persist unblocked status
					_ = workflow.ExecuteActivity(actCtx, "UpdateTaskConflictStatus",
						unblockedTask.TaskID, "RESOLVED", []int64{}).Get(ctx, nil)

					// Start the unblocked task workflow
					if !unblockedTask.WorkflowStarted {
						unblockedTask.WorkflowStarted = true
						startTaskWorkflow(ctx, actCtx, VersionTaskSignal{
							TaskID:      unblockedTask.TaskID,
							Title:       unblockedTask.Title,
							Requirement: unblockedTask.Requirement,
							CreatedBy:   unblockedTask.CreatedBy,
						}, input)
					}

					// SSE: notify unblocked
					_ = workflow.ExecuteActivity(actCtx, "NotifyConflict",
						input.VersionID, unblockedTask.TaskID, "RESOLVED", []int64{}, unblockedTask.TouchedFiles).Get(ctx, nil)
				}
			}
		})

		// Handle: cancel version
		selector.AddReceive(cancelCh, func(ch workflow.ReceiveChannel, more bool) {
			var sig CancelVersionSignal
			ch.Receive(ctx, &sig)
			eventCount++
			logger.Warn("cancel version signal", "version_id", input.VersionID, "reason", sig.Reason)
			cancelled = true
		})

		// Also add a timer to prevent infinite blocking if no signals
		timerCtx, cancelTimer := workflow.WithCancel(ctx)
		selector.AddReceive(workflow.GetSignalChannel(timerCtx, "__timeout__"), func(ch workflow.ReceiveChannel, more bool) {})
		_ = workflow.NewTimer(timerCtx, 30*time.Minute)

		selector.Select(ctx)
		cancelTimer()

		if cancelled {
			// Cancel all active task workflows
			for _, task := range activeTasks {
				if task.Status == "ACTIVE" || task.Status == "PENDING" {
					task.Status = "CANCELLED"
				}
			}
			_ = workflow.ExecuteActivity(actCtx, "UpdateVersionStatus",
				input.VersionID, "CANCELLED").Get(ctx, nil)
			return nil
		}
	}
}

// startTaskWorkflow launches a child TaskExecutionWorkflow for a task.
func startTaskWorkflow(ctx workflow.Context, actCtx workflow.Context, sig VersionTaskSignal, versionInput VersionOrchestratorInput) {
	childCtx := workflow.WithChildOptions(ctx, workflow.ChildWorkflowOptions{
		WorkflowID: fmt.Sprintf("task-%d-v%d", sig.TaskID, versionInput.VersionID),
	})

	// Start as fire-and-forget child workflow
	workflow.ExecuteChildWorkflow(childCtx, "TaskWorkflow", map[string]interface{}{
		"task_id":     sig.TaskID,
		"tenant_id":   versionInput.TenantID,
		"project_id":  versionInput.ProjectID,
		"created_by":  sig.CreatedBy,
		"requirement": sig.Requirement,
		"title":       sig.Title,
		"version_id":  versionInput.VersionID,
	})
}

// allTasksTerminal returns true if every tracked task is in a terminal state.
func allTasksTerminal(tasks map[int64]*trackedTask) bool {
	for _, t := range tasks {
		if t.Status != "COMPLETED" && t.Status != "FAILED" && t.Status != "CANCELLED" {
			return false
		}
	}
	return true
}

func defaultRetryPolicy() *temporal.RetryPolicy {
	return &temporal.RetryPolicy{
		MaximumAttempts: 3,
	}
}
```

### 1.2 需要额外引入的 import

```go
import (
	"go.temporal.io/sdk/temporal"
)
```

---

## Day 2 — 冲突检测

### 2.1 冲突检测函数（在 `version_workflow.go` 内部）

```go
// conflictResult holds the result of conflict detection for one pair.
type conflictResult struct {
	BlockerTaskID int64
	Level         string // "WARNING" or "BLOCKED"
	OverlapFiles  []string
}

// detectConflicts compares a task's touched_files against all other active tasks.
func detectConflicts(task *trackedTask, allTasks map[int64]*trackedTask) []conflictResult {
	if len(task.TouchedFiles) == 0 {
		return nil
	}

	var results []conflictResult

	for _, other := range allTasks {
		// Skip self, completed, or failed tasks
		if other.TaskID == task.TaskID {
			continue
		}
		if other.Status == "COMPLETED" || other.Status == "FAILED" || other.Status == "CANCELLED" {
			continue
		}
		if len(other.TouchedFiles) == 0 {
			continue
		}

		// Only block against tasks that are actively running (earlier in the version)
		// A task can only be blocked by tasks that were added before it
		if other.TaskID > task.TaskID {
			continue // Only earlier tasks can block later ones
		}

		// Check exact file overlap
		overlapFiles := fileIntersection(task.TouchedFiles, other.TouchedFiles)
		if len(overlapFiles) > 0 {
			results = append(results, conflictResult{
				BlockerTaskID: other.TaskID,
				Level:         "BLOCKED",
				OverlapFiles:  overlapFiles,
			})
			continue
		}

		// Package-level heuristic: same Go package or same directory
		if samePackage(task.TouchedFiles, other.TouchedFiles) {
			results = append(results, conflictResult{
				BlockerTaskID: other.TaskID,
				Level:         "WARNING",
				OverlapFiles:  nil,
			})
		}
	}

	return results
}

// applyConflicts updates the task's conflict status and blocked_by list.
func applyConflicts(task *trackedTask, conflicts []conflictResult) {
	task.BlockedBy = nil
	task.ConflictStatus = "NONE"

	hasBlocked := false
	hasWarning := false

	for _, c := range conflicts {
		if c.Level == "BLOCKED" {
			hasBlocked = true
			task.BlockedBy = append(task.BlockedBy, c.BlockerTaskID)
		} else if c.Level == "WARNING" {
			hasWarning = true
		}
	}

	if hasBlocked {
		task.ConflictStatus = "BLOCKED"
		task.Status = "BLOCKED"
	} else if hasWarning {
		task.ConflictStatus = "WARNING"
	}
}

// unblockDependents checks all blocked tasks and unblocks those
// whose only blocker was the completed task.
func unblockDependents(completedTaskID int64, allTasks map[int64]*trackedTask) []*trackedTask {
	var unblocked []*trackedTask

	for _, task := range allTasks {
		if task.ConflictStatus != "BLOCKED" {
			continue
		}

		// Remove completedTaskID from blocked_by
		newBlockedBy := make([]int64, 0, len(task.BlockedBy))
		for _, bid := range task.BlockedBy {
			if bid != completedTaskID {
				// Check if the blocker is still active
				if blocker, ok := allTasks[bid]; ok {
					if blocker.Status != "COMPLETED" && blocker.Status != "FAILED" && blocker.Status != "CANCELLED" {
						newBlockedBy = append(newBlockedBy, bid)
					}
				}
			}
		}
		task.BlockedBy = newBlockedBy

		// If no more blockers, unblock
		if len(newBlockedBy) == 0 {
			unblocked = append(unblocked, task)
		}
	}

	return unblocked
}

// fileIntersection returns files that appear in both lists.
func fileIntersection(a, b []string) []string {
	set := make(map[string]bool, len(a))
	for _, f := range a {
		set[f] = true
	}
	var overlap []string
	for _, f := range b {
		if set[f] {
			overlap = append(overlap, f)
		}
	}
	return overlap
}

// samePackage returns true if any files from a and b are in the same Go package
// or the same directory.
func samePackage(a, b []string) bool {
	dirsA := make(map[string]bool)
	for _, f := range a {
		dir := parentDir(f)
		if dir != "" {
			dirsA[dir] = true
		}
	}
	for _, f := range b {
		dir := parentDir(f)
		if dir != "" && dirsA[dir] {
			return true
		}
	}
	return false
}

// parentDir extracts the directory portion of a file path.
func parentDir(path string) string {
	for i := len(path) - 1; i >= 0; i-- {
		if path[i] == '/' {
			return path[:i]
		}
	}
	return ""
}
```

### 2.2 新建 `forge-core/internal/temporal/activity/conflict.go`

```go
package activity

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shulex/forge/forge-core/internal/module/task"
)

// ConflictActivities provides DB activities for the VersionOrchestrator.
type ConflictActivities struct {
	db  *pgxpool.Pool
	sse *task.SSEHub
}

func NewConflictActivities(db *pgxpool.Pool, sse *task.SSEHub) *ConflictActivities {
	return &ConflictActivities{db: db, sse: sse}
}

// UpdateVersionStatus updates the version's status in the database.
func (a *ConflictActivities) UpdateVersionStatus(ctx context.Context, versionID int64, status string) error {
	_, err := a.db.Exec(ctx,
		`UPDATE engine.project_versions SET status = $2, updated_at = NOW() WHERE id = $1`,
		versionID, status,
	)
	if err != nil {
		return fmt.Errorf("update version status: %w", err)
	}
	slog.Info("version status updated", "version_id", versionID, "status", status)
	return nil
}

// UpdateTaskConflictStatus updates a task's conflict_status and blocked_by.
func (a *ConflictActivities) UpdateTaskConflictStatus(ctx context.Context, taskID int64, conflictStatus string, blockedBy []int64) error {
	_, err := a.db.Exec(ctx,
		`UPDATE engine.tasks SET conflict_status = $2, blocked_by = $3, updated_at = NOW() WHERE id = $1`,
		taskID, conflictStatus, blockedBy,
	)
	if err != nil {
		return fmt.Errorf("update task conflict: %w", err)
	}
	slog.Info("task conflict updated", "task_id", taskID, "conflict_status", conflictStatus, "blocked_by", blockedBy)
	return nil
}

// UpdateTaskTouchedFiles stores the touched_files for a task.
func (a *ConflictActivities) UpdateTaskTouchedFiles(ctx context.Context, taskID int64, files []string) error {
	_, err := a.db.Exec(ctx,
		`UPDATE engine.tasks SET touched_files = $2, updated_at = NOW() WHERE id = $1`,
		taskID, files,
	)
	if err != nil {
		return fmt.Errorf("update task touched_files: %w", err)
	}
	slog.Info("task touched_files updated", "task_id", taskID, "files_count", len(files))
	return nil
}

// NotifyConflict sends an SSE event for conflict status changes.
func (a *ConflictActivities) NotifyConflict(
	ctx context.Context,
	versionID int64,
	taskID int64,
	conflictStatus string,
	blockedBy []int64,
	touchedFiles []string,
) error {
	if a.sse == nil {
		return nil
	}

	// Build human-readable conflict message
	var message string
	switch conflictStatus {
	case "BLOCKED":
		if len(touchedFiles) > 0 {
			message = fmt.Sprintf("任务 #%d 被阻塞，等待文件冲突解决（冲突文件: %v）", taskID, touchedFiles[:min(3, len(touchedFiles))])
		} else {
			message = fmt.Sprintf("任务 #%d 被阻塞，等待前序任务完成", taskID)
		}
	case "WARNING":
		message = fmt.Sprintf("任务 #%d 检测到潜在冲突（同目录），建议关注", taskID)
	case "RESOLVED":
		message = fmt.Sprintf("任务 #%d 冲突已解决，正在启动", taskID)
	}

	a.sse.Broadcast(taskID, task.TaskProgressEvent{
		Type:   "conflict_status",
		TaskID: taskID,
		Status: conflictStatus,
		Data: map[string]interface{}{
			"version_id":      versionID,
			"conflict_status": conflictStatus,
			"blocked_by":      blockedBy,
			"touched_files":   touchedFiles,
			"message":         message,
		},
	})

	slog.Info("conflict notification sent", "task_id", taskID, "conflict_status", conflictStatus)
	return nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
```

---

## Day 3 — 集成 + SSE + 测试

### 3.1 修改 `forge-core/internal/temporal/worker.go` — 注册新 Workflow + Activities

```go
// 在 StartWorker 函数中添加：

// Version Orchestrator workflow
w.RegisterWorkflowWithOptions(wf.VersionOrchestrator, workflow.RegisterOptions{
    Name: "VersionOrchestrator",
})

// Conflict activities
conflictActs := activity.NewConflictActivities(db, sse)
w.RegisterActivityWithOptions(conflictActs.UpdateVersionStatus, sdkactivity.RegisterOptions{
    Name: "UpdateVersionStatus",
})
w.RegisterActivityWithOptions(conflictActs.UpdateTaskConflictStatus, sdkactivity.RegisterOptions{
    Name: "UpdateTaskConflictStatus",
})
w.RegisterActivityWithOptions(conflictActs.UpdateTaskTouchedFiles, sdkactivity.RegisterOptions{
    Name: "UpdateTaskTouchedFiles",
})
w.RegisterActivityWithOptions(conflictActs.NotifyConflict, sdkactivity.RegisterOptions{
    Name: "NotifyConflict",
})
```

### 3.2 修改任务创建流程 — 带 version_id 的任务走 VersionOrchestrator

修改 `forge-core/internal/module/task/service.go` 的 `CreateTask` 方法：

```go
// 在 CreateTask 方法中，任务入库后：

// If task has a version_id, signal the VersionOrchestrator instead of starting TaskWorkflow directly
if req.VersionID != nil && *req.VersionID > 0 {
    // Ensure VersionOrchestrator is running for this version
    versionWorkflowID := fmt.Sprintf("version-orchestrator-%d", *req.VersionID)

    // Try to signal existing orchestrator
    err := s.temporalClient.SignalWorkflow(ctx,
        versionWorkflowID,
        "",
        "new_task",
        map[string]interface{}{
            "task_id":     task.ID,
            "title":       task.Title,
            "requirement": task.Requirement,
            "created_by":  task.CreatedBy,
        },
    )
    if err != nil {
        // Orchestrator not running yet — start it, then signal
        _, startErr := s.temporalClient.ExecuteWorkflow(ctx,
            client.StartWorkflowOptions{
                ID:        versionWorkflowID,
                TaskQueue: "forge-core",
            },
            "VersionOrchestrator",
            map[string]interface{}{
                "version_id": *req.VersionID,
                "project_id": task.ProjectID,
                "tenant_id":  task.TenantID,
            },
        )
        if startErr != nil {
            slog.Error("failed to start VersionOrchestrator", "error", startErr)
        }

        // Retry signal
        _ = s.temporalClient.SignalWorkflow(ctx,
            versionWorkflowID,
            "",
            "new_task",
            map[string]interface{}{
                "task_id":     task.ID,
                "title":       task.Title,
                "requirement": task.Requirement,
                "created_by":  task.CreatedBy,
            },
        )
    }
    return // Don't start individual TaskWorkflow
}

// Original flow: start TaskWorkflow directly (no version)
// ... existing code ...
```

### 3.3 修改任务完成流程 — 通知 VersionOrchestrator

在 `forge-core/internal/temporal/workflow/task_workflow.go` 的 `TaskWorkflow` 和 `TaskExecutionWorkflow` 末尾，添加版本信号：

```go
// At the end of TaskWorkflow / TaskExecutionWorkflow, before returning:

// If task belongs to a version, signal the VersionOrchestrator
if input.VersionID > 0 {
    versionWorkflowID := fmt.Sprintf("version-orchestrator-%d", input.VersionID)

    // Collect touched files from generate result
    var touchedFiles []string
    if files, ok := generateResult["files"].([]interface{}); ok {
        for _, f := range files {
            if m, ok := f.(map[string]interface{}); ok {
                if path, ok := m["path"].(string); ok {
                    touchedFiles = append(touchedFiles, path)
                }
            }
        }
    }

    _ = workflow.SignalExternalWorkflow(ctx, versionWorkflowID, "", "task_completed",
        map[string]interface{}{
            "task_id":       input.TaskID,
            "touched_files": touchedFiles,
            "success":       true,
        },
    ).Get(ctx, nil)
}
```

对应地，需要扩展 `TaskWorkflowInput` 添加 `VersionID` 字段：

```go
// In activity/task_activities.go:
type TaskWorkflowInput struct {
    TaskID      int64                  `json:"task_id"`
    TenantID    int64                  `json:"tenant_id"`
    ProjectID   int64                  `json:"project_id"`
    CreatedBy   int64                  `json:"created_by"`
    Requirement string                 `json:"requirement"`
    Title       string                 `json:"title"`
    PlanResult  map[string]interface{} `json:"plan_result,omitempty"`
    VersionID   int64                  `json:"version_id,omitempty"` // NEW
}
```

### 3.4 PLAN 步骤后信号 — 更新 touched_files

在 `TaskWorkflow` 的 PLAN 步骤完成后，提取计划中的文件列表并通知 VersionOrchestrator：

```go
// After plan result is saved, extract planned files and signal VersionOrchestrator
if input.VersionID > 0 {
    var plannedFiles []string
    if tasks, ok := planResult["tasks"].([]interface{}); ok {
        for _, t := range tasks {
            if node, ok := t.(map[string]interface{}); ok {
                if files, ok := node["files"].([]interface{}); ok {
                    for _, f := range files {
                        if path, ok := f.(string); ok {
                            plannedFiles = append(plannedFiles, path)
                        }
                    }
                }
            }
        }
    }

    if len(plannedFiles) > 0 {
        versionWorkflowID := fmt.Sprintf("version-orchestrator-%d", input.VersionID)
        _ = workflow.SignalExternalWorkflow(ctx, versionWorkflowID, "", "task_files_updated",
            map[string]interface{}{
                "task_id":       input.TaskID,
                "touched_files": plannedFiles,
            },
        ).Get(ctx, nil)
    }
}
```

### 3.5 修改 `forge-core/internal/module/task/model.go` — CreateTaskRequest 添加 VersionID

```go
type CreateTaskRequest struct {
    Title       string `json:"title" binding:"max=200"`
    Requirement string `json:"requirement" binding:"required,min=1,max=10000"`
    VersionID   *int64 `json:"versionId,omitempty"` // NEW: optional version association
}
```

---

## 数据结构

### VersionOrchestrator State（内存，Temporal Workflow 内部）

```go
activeTasks map[int64]*trackedTask

type trackedTask struct {
    TaskID         int64
    Title          string
    Requirement    string
    CreatedBy      int64
    TouchedFiles   []string
    Status         string    // PENDING / ACTIVE / COMPLETED / FAILED / BLOCKED / CANCELLED
    ConflictStatus string    // NONE / WARNING / BLOCKED / RESOLVED
    BlockedBy      []int64
    WorkflowStarted bool
}
```

### Signal Payloads

| Signal | Payload | 方向 |
|--------|---------|------|
| `new_task` | `{task_id, title, requirement, created_by, touched_files}` | Task Service → Orchestrator |
| `task_files_updated` | `{task_id, touched_files}` | TaskWorkflow (post-PLAN) → Orchestrator |
| `task_completed` | `{task_id, touched_files, success}` | TaskWorkflow (end) → Orchestrator |
| `cancel_version` | `{reason}` | Version Service → Orchestrator |

### SSE Events

```json
{
  "type": "conflict_status",
  "task_id": 42,
  "status": "BLOCKED",
  "data": {
    "version_id": 7,
    "conflict_status": "BLOCKED",
    "blocked_by": [38, 40],
    "touched_files": ["internal/handler/user.go", "internal/service/user.go"],
    "message": "任务 #42 被阻塞，等待文件冲突解决（冲突文件: [internal/handler/user.go, internal/service/user.go]）"
  }
}
```

---

## API 设计

本切片不新增 REST API（版本 CRUD 在 SH-3a 完成）。所有协调通过 Temporal Signal 完成：

| 操作 | 机制 | 触发者 |
|------|------|--------|
| 注册任务 | Signal `new_task` | Task Service (CreateTask) |
| 更新文件列表 | Signal `task_files_updated` | TaskWorkflow (post-PLAN) |
| 任务完成通知 | Signal `task_completed` | TaskWorkflow (end) |
| 取消版本 | Signal `cancel_version` | Version Service (Release/Cancel) |

---

## 验收标准

1. **VersionOrchestrator Workflow**
   - 每个版本启动一个长运行 Workflow（ID 格式: `version-orchestrator-{version_id}`）
   - 接收 new_task 信号后注册任务到内部 map
   - 无冲突的任务立即启动 TaskWorkflow
   - ContinueAsNew 每 50 个事件触发一次（不丢失状态）
   - 所有任务终态后 Workflow 自动完成

2. **冲突检测**
   - 两个任务修改同一文件 → 后者 BLOCKED，前者完成后自动解除
   - 两个任务修改同目录不同文件 → WARNING（不阻塞，仅提示）
   - BLOCKED 任务的 blocked_by 字段正确记录阻塞者 ID
   - 多级阻塞：A→B→C（A 阻塞 B，B 阻塞 C）正确传递

3. **SSE 集成**
   - 冲突检测后发送 `conflict_status` 事件
   - 事件包含 version_id, conflict_status, blocked_by, touched_files, message
   - message 是人类可读的中文描述

4. **任务创建集成**
   - 带 version_id 的任务不直接启动 TaskWorkflow
   - 而是 Signal VersionOrchestrator，由后者决定是否启动
   - 无 version_id 的任务走原有流程（不受影响）

5. **PLAN 步骤集成**
   - PLAN 完成后提取文件列表，Signal VersionOrchestrator 更新 touched_files
   - 触发重新冲突检测

---

## 质量验证

### 集成测试场景

**场景 1：无冲突并行**

```
1. 创建版本 v1.0.0
2. 创建任务 A（version_id=v1.0.0），files: [handler/order.go]
3. 创建任务 B（version_id=v1.0.0），files: [handler/product.go]
4. 预期：A 和 B 同时启动，无冲突
```

**场景 2：文件冲突阻塞**

```
1. 创建版本 v1.0.0
2. 创建任务 A（version_id=v1.0.0），files: [handler/user.go, service/user.go]
3. 创建任务 B（version_id=v1.0.0），files: [handler/user.go]  ← 冲突！
4. 预期：
   - A 立即启动
   - B 标记 BLOCKED, blocked_by=[A.id]
   - SSE 收到 conflict_status 事件
   - A 完成后 → B 自动解除阻塞 → B 启动
   - SSE 收到 conflict_status=RESOLVED 事件
```

**场景 3：目录级告警**

```
1. 创建任务 A，files: [internal/module/user/handler.go]
2. 创建任务 B，files: [internal/module/user/service.go]  ← 同目录
3. 预期：B 标记 WARNING（不阻塞），两个任务同时执行
```

**场景 4：多级阻塞链**

```
1. 创建任务 A，files: [config.go]
2. 创建任务 B，files: [config.go, main.go]  ← 被 A 阻塞
3. 创建任务 C，files: [main.go]  ← 被 B 阻塞
4. 预期：
   - A 执行，B 和 C 等待
   - A 完成 → B 解除阻塞 → B 开始执行
   - B 完成 → C 解除阻塞 → C 开始执行
```

### 单元测试

```go
// internal/temporal/workflow/version_workflow_test.go

func TestFileIntersection(t *testing.T) {
    tests := []struct {
        a, b     []string
        expected []string
    }{
        {[]string{"a.go", "b.go"}, []string{"b.go", "c.go"}, []string{"b.go"}},
        {[]string{"a.go"}, []string{"b.go"}, nil},
        {[]string{}, []string{"a.go"}, nil},
        {[]string{"x/y.go", "x/z.go"}, []string{"x/y.go"}, []string{"x/y.go"}},
    }
    for _, tt := range tests {
        result := fileIntersection(tt.a, tt.b)
        if len(result) != len(tt.expected) {
            t.Errorf("fileIntersection(%v, %v) = %v, want %v", tt.a, tt.b, result, tt.expected)
        }
    }
}

func TestSamePackage(t *testing.T) {
    tests := []struct {
        a, b     []string
        expected bool
    }{
        {[]string{"pkg/user/handler.go"}, []string{"pkg/user/service.go"}, true},
        {[]string{"pkg/user/handler.go"}, []string{"pkg/order/handler.go"}, false},
        {[]string{"main.go"}, []string{"config.go"}, false},  // root level, no parent dir
        {[]string{"internal/a.go"}, []string{"internal/b.go"}, true},
    }
    for _, tt := range tests {
        result := samePackage(tt.a, tt.b)
        if result != tt.expected {
            t.Errorf("samePackage(%v, %v) = %v, want %v", tt.a, tt.b, result, tt.expected)
        }
    }
}

func TestDetectConflicts(t *testing.T) {
    taskA := &trackedTask{TaskID: 1, TouchedFiles: []string{"handler/user.go"}, Status: "ACTIVE"}
    taskB := &trackedTask{TaskID: 2, TouchedFiles: []string{"handler/user.go"}, Status: "PENDING"}

    allTasks := map[int64]*trackedTask{1: taskA, 2: taskB}
    conflicts := detectConflicts(taskB, allTasks)

    if len(conflicts) != 1 {
        t.Fatalf("expected 1 conflict, got %d", len(conflicts))
    }
    if conflicts[0].Level != "BLOCKED" {
        t.Errorf("expected BLOCKED, got %s", conflicts[0].Level)
    }
    if conflicts[0].BlockerTaskID != 1 {
        t.Errorf("expected blocker 1, got %d", conflicts[0].BlockerTaskID)
    }
}

func TestUnblockDependents(t *testing.T) {
    taskA := &trackedTask{TaskID: 1, Status: "COMPLETED"}
    taskB := &trackedTask{TaskID: 2, Status: "BLOCKED", ConflictStatus: "BLOCKED", BlockedBy: []int64{1}}
    taskC := &trackedTask{TaskID: 3, Status: "BLOCKED", ConflictStatus: "BLOCKED", BlockedBy: []int64{1, 2}}

    allTasks := map[int64]*trackedTask{1: taskA, 2: taskB, 3: taskC}
    unblocked := unblockDependents(1, allTasks)

    // taskB should be unblocked (only blocker was 1)
    if len(unblocked) != 1 || unblocked[0].TaskID != 2 {
        t.Errorf("expected taskB unblocked, got %v", unblocked)
    }
    // taskC should still be blocked (still blocked by 2)
    if taskC.ConflictStatus != "BLOCKED" {
        t.Errorf("taskC should still be BLOCKED, got %s", taskC.ConflictStatus)
    }
}

func TestApplyConflicts(t *testing.T) {
    task := &trackedTask{TaskID: 3}
    conflicts := []conflictResult{
        {BlockerTaskID: 1, Level: "BLOCKED", OverlapFiles: []string{"a.go"}},
        {BlockerTaskID: 2, Level: "WARNING"},
    }
    applyConflicts(task, conflicts)
    if task.ConflictStatus != "BLOCKED" {
        t.Errorf("expected BLOCKED, got %s", task.ConflictStatus)
    }
    if len(task.BlockedBy) != 1 || task.BlockedBy[0] != 1 {
        t.Errorf("expected blocked_by=[1], got %v", task.BlockedBy)
    }
}

func TestParentDir(t *testing.T) {
    tests := []struct {
        path     string
        expected string
    }{
        {"internal/handler/user.go", "internal/handler"},
        {"main.go", ""},
        {"a/b/c/d.go", "a/b/c"},
    }
    for _, tt := range tests {
        result := parentDir(tt.path)
        if result != tt.expected {
            t.Errorf("parentDir(%q) = %q, want %q", tt.path, result, tt.expected)
        }
    }
}
```
