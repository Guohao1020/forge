package workflow

import (
	"time"

	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"

	"github.com/shulex/forge/forge-core/internal/temporal/activity"
)

var workflowSteps = []activity.StepInput{
	{StepType: "ANALYZE", TaskStatus: "ANALYZING", Duration: 2},
	{StepType: "PLAN", TaskStatus: "PLANNING", Duration: 2},
	{StepType: "GENERATE", TaskStatus: "GENERATING", Duration: 3},
	{StepType: "REVIEW", TaskStatus: "REVIEWING", Duration: 2},
	{StepType: "TEST", TaskStatus: "TESTING", Duration: 2},
	{StepType: "DEPLOY", TaskStatus: "DEPLOYING", Duration: 2},
}

// TaskWorkflow is the skeleton workflow that transitions through all steps.
func TaskWorkflow(ctx workflow.Context, input activity.TaskWorkflowInput) error {
	logger := workflow.GetLogger(ctx)
	logger.Info("TaskWorkflow started", "task_id", input.TaskID)

	actCtx := workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
		StartToCloseTimeout: 60 * time.Second,
		RetryPolicy: &temporal.RetryPolicy{
			MaximumAttempts: 3,
		},
	})

	for _, step := range workflowSteps {
		stepInput := activity.StepInput{
			TaskID:     input.TaskID,
			StepType:   step.StepType,
			TaskStatus: step.TaskStatus,
			Duration:   step.Duration,
		}

		var result activity.StepOutput
		err := workflow.ExecuteActivity(actCtx, "ExecuteStep", stepInput).Get(ctx, &result)
		if err != nil {
			logger.Error("step failed", "task_id", input.TaskID, "step", step.StepType, "error", err)
			_ = workflow.ExecuteActivity(actCtx, "FailTask", input.TaskID, err.Error()).Get(ctx, nil)
			return err
		}
	}

	err := workflow.ExecuteActivity(actCtx, "CompleteTask", input.TaskID).Get(ctx, nil)
	if err != nil {
		logger.Error("failed to complete task", "task_id", input.TaskID, "error", err)
		return err
	}

	logger.Info("TaskWorkflow completed", "task_id", input.TaskID)
	return nil
}
