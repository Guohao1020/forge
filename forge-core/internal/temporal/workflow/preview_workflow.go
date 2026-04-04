package workflow

import (
	"fmt"
	"time"

	"go.temporal.io/sdk/workflow"
)

// PreviewLifecycleInput starts a preview environment lifecycle.
type PreviewLifecycleInput struct {
	PreviewID  int64  `json:"previewId"`
	TenantID   int64  `json:"tenantId"`
	ProjectID  int64  `json:"projectId"`
	TaskID     int64  `json:"taskId"`
	BranchName string `json:"branchName"`
	ImageURL   string `json:"imageUrl"`
	IdleTimeout time.Duration `json:"idleTimeout"` // default 30 minutes
}

// PreviewLifecycleWorkflow manages the lifecycle of a cloud preview environment.
//
// 1. Creates K8s namespace + deploys service
// 2. Generates preview URL
// 3. Waits for: idle timeout OR destroy signal OR PR merge signal
// 4. Tears down resources
func PreviewLifecycleWorkflow(ctx workflow.Context, input PreviewLifecycleInput) error {
	logger := workflow.GetLogger(ctx)
	logger.Info("PreviewLifecycleWorkflow started",
		"preview_id", input.PreviewID,
		"task_id", input.TaskID,
		"branch", input.BranchName,
	)

	if input.IdleTimeout == 0 {
		input.IdleTimeout = 30 * time.Minute
	}

	localCtx := workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
		StartToCloseTimeout: 2 * time.Minute,
	})

	// Step 1: Create preview resources
	namespace := fmt.Sprintf("preview-%d", input.TaskID)
	previewURL := fmt.Sprintf("%d.preview.forge.example.com", input.TaskID)

	// Update preview record with URL
	_ = workflow.ExecuteActivity(localCtx, "UpdatePreviewStatus",
		input.PreviewID, "CREATING", previewURL, namespace,
	).Get(ctx, nil)

	// TODO: When K8s is available:
	// - kubectl create namespace {namespace}
	// - kubectl apply -f deployment.yaml (generated from GenerateK8sManifests)
	// - kubectl apply -f service.yaml
	// - kubectl apply -f ingress.yaml (with preview URL)
	// For now: just update status to READY

	_ = workflow.ExecuteActivity(localCtx, "UpdatePreviewStatus",
		input.PreviewID, "READY", previewURL, namespace,
	).Get(ctx, nil)

	logger.Info("Preview environment ready",
		"url", previewURL,
		"namespace", namespace,
	)

	// Step 2: Wait for termination signal or idle timeout
	destroyCh := workflow.GetSignalChannel(ctx, "destroy_preview")
	prMergedCh := workflow.GetSignalChannel(ctx, "pr_merged")

	selector := workflow.NewSelector(ctx)

	// Idle timeout
	timerFuture := workflow.NewTimer(ctx, input.IdleTimeout)
	selector.AddFuture(timerFuture, func(f workflow.Future) {
		logger.Info("Preview idle timeout reached, destroying", "preview_id", input.PreviewID)
	})

	// Manual destroy signal
	selector.AddReceive(destroyCh, func(ch workflow.ReceiveChannel, more bool) {
		ch.Receive(ctx, nil)
		logger.Info("Destroy signal received", "preview_id", input.PreviewID)
	})

	// PR merged signal
	selector.AddReceive(prMergedCh, func(ch workflow.ReceiveChannel, more bool) {
		ch.Receive(ctx, nil)
		logger.Info("PR merged signal received, destroying preview", "preview_id", input.PreviewID)
	})

	selector.Select(ctx) // blocks until one of the above triggers

	// Step 3: Tear down
	logger.Info("Destroying preview environment",
		"preview_id", input.PreviewID,
		"namespace", namespace,
	)

	// TODO: When K8s is available:
	// - kubectl delete namespace {namespace}
	// For now: just update status to DESTROYED

	_ = workflow.ExecuteActivity(localCtx, "UpdatePreviewStatus",
		input.PreviewID, "DESTROYED", previewURL, namespace,
	).Get(ctx, nil)

	logger.Info("Preview environment destroyed", "preview_id", input.PreviewID)
	return nil
}
