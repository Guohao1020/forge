package temporal

import (
	"log/slog"

	"github.com/jackc/pgx/v5/pgxpool"
	sdkactivity "go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/worker"
	"go.temporal.io/sdk/workflow"

	"github.com/shulex/forge/forge-core/internal/module/task"
	"github.com/shulex/forge/forge-core/internal/temporal/activity"
	wf "github.com/shulex/forge/forge-core/internal/temporal/workflow"
)

// StartWorker creates and starts a Temporal worker in a goroutine.
func StartWorker(c client.Client, db *pgxpool.Pool, sse *task.SSEHub, authToken activity.AuthTokenProvider, projectProv activity.ProjectProvider, taskPR activity.TaskPRUpdater) (worker.Worker, error) {
	w := worker.New(c, TaskQueueName, worker.Options{})

	w.RegisterWorkflowWithOptions(wf.TaskWorkflow, workflow.RegisterOptions{
		Name: "TaskWorkflow",
	})
	w.RegisterWorkflowWithOptions(wf.AnalyzeRequirementWorkflow, workflow.RegisterOptions{
		Name: "AnalyzeRequirementWorkflow",
	})

	activities := activity.NewTaskActivities(db, sse)
	w.RegisterActivityWithOptions(activities.ExecuteStep, sdkactivity.RegisterOptions{
		Name: "ExecuteStep",
	})
	w.RegisterActivityWithOptions(activities.CompleteTask, sdkactivity.RegisterOptions{
		Name: "CompleteTask",
	})
	w.RegisterActivityWithOptions(activities.FailTask, sdkactivity.RegisterOptions{
		Name: "FailTask",
	})
	w.RegisterActivityWithOptions(activities.UpdateTaskAnalysis, sdkactivity.RegisterOptions{
		Name: "UpdateTaskAnalysis",
	})
	w.RegisterActivityWithOptions(activities.SaveStepOutput, sdkactivity.RegisterOptions{
		Name: "SaveStepOutput",
	})
	w.RegisterActivityWithOptions(activities.SaveTaskNodes, sdkactivity.RegisterOptions{
		Name: "SaveTaskNodes",
	})

	// DevOps activities (GitHub operations)
	devops := activity.NewDevOpsActivities(db, authToken, projectProv, taskPR, sse)
	w.RegisterActivityWithOptions(devops.PushToGitHub, sdkactivity.RegisterOptions{
		Name: "PushToGitHub",
	})
	w.RegisterActivityWithOptions(devops.CreatePullRequest, sdkactivity.RegisterOptions{
		Name: "CreatePullRequest",
	})
	w.RegisterActivityWithOptions(devops.SavePRInfo, sdkactivity.RegisterOptions{
		Name: "SavePRInfo",
	})

	if err := w.Start(); err != nil {
		return nil, err
	}

	slog.Info("temporal worker started", "queue", TaskQueueName)
	return w, nil
}
