package temporal

import (
	"log/slog"

	"github.com/jackc/pgx/v5/pgxpool"
	sdkactivity "go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/worker"
	"go.temporal.io/sdk/workflow"

	"github.com/shulex/forge/forge-core/internal/k8s"
	"github.com/shulex/forge/forge-core/internal/module/task"
	"github.com/shulex/forge/forge-core/internal/temporal/activity"
	wf "github.com/shulex/forge/forge-core/internal/temporal/workflow"
	"github.com/shulex/forge/forge-core/internal/workspace"
)

// StartWorker creates and starts a Temporal worker in a goroutine.
func StartWorker(c client.Client, db *pgxpool.Pool, sse *task.SSEHub, authToken activity.AuthTokenProvider, projectProv activity.ProjectProvider, taskPR activity.TaskPRUpdater, ws *workspace.Manager, k8sClient *k8s.Client) (worker.Worker, error) {
	w := worker.New(c, TaskQueueName, worker.Options{})

	w.RegisterWorkflowWithOptions(wf.TaskWorkflow, workflow.RegisterOptions{
		Name: "TaskWorkflow",
	})
	w.RegisterWorkflowWithOptions(wf.PlanOnlyWorkflow, workflow.RegisterOptions{
		Name: "PlanOnlyWorkflow",
	})
	w.RegisterWorkflowWithOptions(wf.TaskExecutionWorkflow, workflow.RegisterOptions{
		Name: "TaskExecutionWorkflow",
	})
	w.RegisterWorkflowWithOptions(wf.AnalyzeRequirementWorkflow, workflow.RegisterOptions{
		Name: "AnalyzeRequirementWorkflow",
	})
	w.RegisterWorkflowWithOptions(wf.ProfileScanWorkflow, workflow.RegisterOptions{
		Name: "ProfileScanWorkflow",
	})

	activities := activity.NewTaskActivities(db, sse, k8sClient)
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
	w.RegisterActivityWithOptions(activities.RunTests, sdkactivity.RegisterOptions{
		Name: "RunTests",
	})

	w.RegisterActivityWithOptions(activities.CreatePreview, sdkactivity.RegisterOptions{
		Name: "CreatePreview",
	})

	// DevOps activities (GitHub operations)
	devops := activity.NewDevOpsActivities(db, authToken, projectProv, taskPR, sse, ws)
	w.RegisterActivityWithOptions(devops.PushToGitHub, sdkactivity.RegisterOptions{
		Name: "PushToGitHub",
	})
	w.RegisterActivityWithOptions(devops.CreatePullRequest, sdkactivity.RegisterOptions{
		Name: "CreatePullRequest",
	})
	w.RegisterActivityWithOptions(devops.SavePRInfo, sdkactivity.RegisterOptions{
		Name: "SavePRInfo",
	})

	// Version orchestrator workflow + activities
	w.RegisterWorkflowWithOptions(wf.VersionOrchestrator, workflow.RegisterOptions{
		Name: "VersionOrchestrator",
	})
	versionActs := activity.NewVersionActivities(db)
	w.RegisterActivityWithOptions(versionActs.UpdateTaskConflict, sdkactivity.RegisterOptions{
		Name: "UpdateTaskConflict",
	})
	w.RegisterActivityWithOptions(versionActs.UpdateVersionStatus, sdkactivity.RegisterOptions{
		Name: "UpdateVersionStatus",
	})
	w.RegisterActivityWithOptions(versionActs.SaveTouchedFiles, sdkactivity.RegisterOptions{
		Name: "SaveTouchedFiles",
	})

	// Build activities (S13 — artifact management)
	buildActs := activity.NewBuildActivities(db)
	w.RegisterActivityWithOptions(buildActs.BuildDockerImage, sdkactivity.RegisterOptions{
		Name: "BuildDockerImage",
	})

	// Deploy activities (S14 — K8s deployment)
	deployActs := activity.NewDeployActivities(db)
	w.RegisterActivityWithOptions(deployActs.GenerateK8sManifests, sdkactivity.RegisterOptions{
		Name: "GenerateK8sManifests",
	})
	w.RegisterActivityWithOptions(deployActs.Rollback, sdkactivity.RegisterOptions{
		Name: "RollbackDeploy",
	})

	// Preview lifecycle (S17 — cloud preview environments)
	w.RegisterWorkflowWithOptions(wf.PreviewLifecycleWorkflow, workflow.RegisterOptions{
		Name: "PreviewLifecycleWorkflow",
	})
	previewActs := activity.NewPreviewActivities(db)
	w.RegisterActivityWithOptions(previewActs.UpdatePreviewStatus, sdkactivity.RegisterOptions{
		Name: "UpdatePreviewStatus",
	})

	if err := w.Start(); err != nil {
		return nil, err
	}

	slog.Info("temporal worker started", "queue", TaskQueueName)
	return w, nil
}
