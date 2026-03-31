package temporal

import (
	"log/slog"

	"github.com/jackc/pgx/v5/pgxpool"
	sdkactivity "go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/worker"
	"go.temporal.io/sdk/workflow"

	"github.com/shulex/forge/forge-core/internal/temporal/activity"
	wf "github.com/shulex/forge/forge-core/internal/temporal/workflow"
)

// StartWorker creates and starts a Temporal worker in a goroutine.
func StartWorker(c client.Client, db *pgxpool.Pool) (worker.Worker, error) {
	w := worker.New(c, TaskQueueName, worker.Options{})

	w.RegisterWorkflowWithOptions(wf.TaskWorkflow, workflow.RegisterOptions{
		Name: "TaskWorkflow",
	})

	activities := activity.NewTaskActivities(db)
	w.RegisterActivityWithOptions(activities.ExecuteStep, sdkactivity.RegisterOptions{
		Name: "ExecuteStep",
	})
	w.RegisterActivityWithOptions(activities.CompleteTask, sdkactivity.RegisterOptions{
		Name: "CompleteTask",
	})
	w.RegisterActivityWithOptions(activities.FailTask, sdkactivity.RegisterOptions{
		Name: "FailTask",
	})

	if err := w.Start(); err != nil {
		return nil, err
	}

	slog.Info("temporal worker started", "queue", TaskQueueName)
	return w, nil
}
