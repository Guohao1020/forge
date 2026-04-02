package workflow

import (
	"time"

	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"

	"github.com/shulex/forge/forge-core/internal/temporal/activity"
)

// TaskWorkflow orchestrates the full code generation pipeline.
// AI activities run on "ai-worker" queue (Python), DB updates run on Go queue.
func TaskWorkflow(ctx workflow.Context, input activity.TaskWorkflowInput) error {
	logger := workflow.GetLogger(ctx)
	logger.Info("TaskWorkflow started", "task_id", input.TaskID)

	// Local activity options — Go worker, DB updates
	localCtx := workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
		StartToCloseTimeout: 30 * time.Second,
		RetryPolicy: &temporal.RetryPolicy{
			MaximumAttempts: 3,
		},
	})

	// AI activity options — cross-language, Python worker
	aiCtx := workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
		TaskQueue:           "ai-worker",
		StartToCloseTimeout: 5 * time.Minute,
		RetryPolicy: &temporal.RetryPolicy{
			MaximumAttempts:    3,
			InitialInterval:    5 * time.Second,
			BackoffCoefficient: 2.0,
		},
	})

	// ---- Step 1: Plan ----
	err := workflow.ExecuteActivity(localCtx, "ExecuteStep", activity.StepInput{
		TaskID: input.TaskID, StepType: "PLAN", TaskStatus: "PLANNING", Duration: 0,
	}).Get(ctx, nil)
	if err != nil {
		logger.Error("plan step DB update failed", "error", err)
		_ = workflow.ExecuteActivity(localCtx, "FailTask", input.TaskID, err.Error()).Get(ctx, nil)
		return err
	}

	var planResult map[string]interface{}
	err = workflow.ExecuteActivity(aiCtx, "plan_task", map[string]interface{}{
		"task_id":             input.TaskID,
		"tenant_id":           input.TenantID,
		"project_id":          input.ProjectID,
		"requirement_summary": input.Requirement,
	}).Get(ctx, &planResult)
	if err != nil {
		logger.Error("AI plan failed", "error", err)
		_ = workflow.ExecuteActivity(localCtx, "FailTask", input.TaskID, err.Error()).Get(ctx, nil)
		return err
	}

	// Save plan output locally
	_ = workflow.ExecuteActivity(localCtx, "SaveStepOutput", input.TaskID, "PLAN", planResult).Get(ctx, nil)

	// ---- Step 2: Generate ----
	err = workflow.ExecuteActivity(localCtx, "ExecuteStep", activity.StepInput{
		TaskID: input.TaskID, StepType: "GENERATE", TaskStatus: "GENERATING", Duration: 0,
	}).Get(ctx, nil)
	if err != nil {
		_ = workflow.ExecuteActivity(localCtx, "FailTask", input.TaskID, err.Error()).Get(ctx, nil)
		return err
	}

	var generateResult map[string]interface{}
	err = workflow.ExecuteActivity(aiCtx, "generate_code", map[string]interface{}{
		"task_id":             input.TaskID,
		"tenant_id":           input.TenantID,
		"project_id":          input.ProjectID,
		"requirement_summary": input.Requirement,
		"plan":                planResult,
	}).Get(ctx, &generateResult)
	if err != nil {
		logger.Error("AI generate failed", "error", err)
		_ = workflow.ExecuteActivity(localCtx, "FailTask", input.TaskID, err.Error()).Get(ctx, nil)
		return err
	}

	_ = workflow.ExecuteActivity(localCtx, "SaveStepOutput", input.TaskID, "GENERATE", generateResult).Get(ctx, nil)

	// ---- Step 3: Review loop (max 3 attempts) ----
	err = workflow.ExecuteActivity(localCtx, "ExecuteStep", activity.StepInput{
		TaskID: input.TaskID, StepType: "REVIEW", TaskStatus: "REVIEWING", Duration: 0,
	}).Get(ctx, nil)
	if err != nil {
		_ = workflow.ExecuteActivity(localCtx, "FailTask", input.TaskID, err.Error()).Get(ctx, nil)
		return err
	}

	maxReviewAttempts := 3
	var reviewResult map[string]interface{}
	for attempt := 1; attempt <= maxReviewAttempts; attempt++ {
		err = workflow.ExecuteActivity(aiCtx, "review_code", map[string]interface{}{
			"task_id":    input.TaskID,
			"tenant_id":  input.TenantID,
			"project_id": input.ProjectID,
			"code":       generateResult,
			"attempt":    attempt,
		}).Get(ctx, &reviewResult)
		if err != nil {
			logger.Error("AI review failed", "attempt", attempt, "error", err)
			if attempt == maxReviewAttempts {
				_ = workflow.ExecuteActivity(localCtx, "FailTask", input.TaskID, err.Error()).Get(ctx, nil)
				return err
			}
			continue
		}

		// Check if review passed
		passed, _ := reviewResult["passed"].(bool)
		if passed {
			break
		}

		// If not passed and not last attempt, trigger fix
		if attempt < maxReviewAttempts {
			logger.Info("review not passed, triggering fix", "attempt", attempt)
			err = workflow.ExecuteActivity(aiCtx, "generate_code", map[string]interface{}{
				"task_id":             input.TaskID,
				"tenant_id":           input.TenantID,
				"project_id":          input.ProjectID,
				"requirement_summary": input.Requirement,
				"code":                generateResult,
				"review":              reviewResult,
			}).Get(ctx, &generateResult)
			if err != nil {
				logger.Error("AI fix failed", "attempt", attempt, "error", err)
			}
		}
	}

	_ = workflow.ExecuteActivity(localCtx, "SaveStepOutput", input.TaskID, "REVIEW", reviewResult).Get(ctx, nil)

	// ---- Step 4: Deploy (Push to GitHub) ----
	// This step is best-effort: if GitHub is not connected, we skip gracefully.
	err = workflow.ExecuteActivity(localCtx, "ExecuteStep", activity.StepInput{
		TaskID: input.TaskID, StepType: "DEPLOY", TaskStatus: "DEPLOYING", Duration: 0,
	}).Get(ctx, nil)
	if err != nil {
		logger.Error("deploy step DB update failed", "error", err)
		// Non-fatal: still complete the task
	} else {
		deployErr := func() error {
			// Extract files from generateResult
			var files []map[string]interface{}
			if rawFiles, ok := generateResult["files"]; ok {
				if arr, ok := rawFiles.([]interface{}); ok {
					for _, f := range arr {
						if m, ok := f.(map[string]interface{}); ok {
							files = append(files, m)
						}
					}
				}
			}

			if len(files) == 0 {
				logger.Warn("no files to push, skipping deploy", "task_id", input.TaskID)
				return nil
			}

			commitMsg := ""
			if cm, ok := generateResult["commit_message"].(string); ok {
				commitMsg = cm
			}

			// Push to GitHub
			var pushResult activity.PushToGitHubOutput
			err = workflow.ExecuteActivity(localCtx, "PushToGitHub", activity.PushToGitHubInput{
				TaskID:        input.TaskID,
				TenantID:      input.TenantID,
				ProjectID:     input.ProjectID,
				CreatedBy:     input.CreatedBy,
				Files:         generateResult["files"],
				CommitMessage: commitMsg,
			}).Get(ctx, &pushResult)
			if err != nil {
				return err
			}

			// Create PR
			prTitle := ""
			if t, ok := planResult["title"].(string); ok {
				prTitle = t
			}

			var prResult activity.CreatePROutput
			err = workflow.ExecuteActivity(localCtx, "CreatePullRequest", activity.CreatePRInput{
				TaskID:    input.TaskID,
				TenantID:  input.TenantID,
				ProjectID: input.ProjectID,
				CreatedBy: input.CreatedBy,
				Branch:    pushResult.BranchName,
				Title:     prTitle,
			}).Get(ctx, &prResult)
			if err != nil {
				return err
			}

			// Save PR info
			reviewScore := 0
			if s, ok := reviewResult["score"].(float64); ok {
				reviewScore = int(s)
			}

			_ = workflow.ExecuteActivity(localCtx, "SavePRInfo", activity.SavePRInfoInput{
				TaskID:      input.TaskID,
				PRNumber:    prResult.PRNumber,
				PRURL:       prResult.PRURL,
				ReviewScore: reviewScore,
			}).Get(ctx, nil)

			_ = workflow.ExecuteActivity(localCtx, "SaveStepOutput", input.TaskID, "DEPLOY", map[string]interface{}{
				"branch_name": pushResult.BranchName,
				"pr_number":   prResult.PRNumber,
				"pr_url":      prResult.PRURL,
			}).Get(ctx, nil)

			return nil
		}()

		if deployErr != nil {
			logger.Warn("deploy step failed (non-fatal)", "task_id", input.TaskID, "error", deployErr)
			// Save deploy step as skipped so frontend knows
			_ = workflow.ExecuteActivity(localCtx, "SaveStepOutput", input.TaskID, "DEPLOY", map[string]interface{}{
				"skipped": true,
				"error":   deployErr.Error(),
			}).Get(ctx, nil)
		}
	}

	// ---- Step 5: Complete ----
	err = workflow.ExecuteActivity(localCtx, "CompleteTask", input.TaskID).Get(ctx, nil)
	if err != nil {
		logger.Error("failed to complete task", "error", err)
		return err
	}

	logger.Info("TaskWorkflow completed", "task_id", input.TaskID)
	return nil
}

// AnalyzeRequirementWorkflow is a thin wrapper that calls the Python analyze_requirement activity.
// Used by conversation.SendMessage to synchronously get AI analysis results.
func AnalyzeRequirementWorkflow(ctx workflow.Context, input map[string]interface{}) (map[string]interface{}, error) {
	logger := workflow.GetLogger(ctx)
	logger.Info("AnalyzeRequirementWorkflow started")

	aiCtx := workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
		TaskQueue:           "ai-worker",
		StartToCloseTimeout: 3 * time.Minute,
		RetryPolicy: &temporal.RetryPolicy{
			MaximumAttempts:    2,
			InitialInterval:    3 * time.Second,
			BackoffCoefficient: 2.0,
		},
	})

	var result map[string]interface{}
	err := workflow.ExecuteActivity(aiCtx, "analyze_requirement", input).Get(ctx, &result)
	if err != nil {
		logger.Error("analyze_requirement activity failed", "error", err)
		return nil, err
	}

	return result, nil
}
