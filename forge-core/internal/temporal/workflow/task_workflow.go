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

	// Mark ANALYZE as completed since analysis was done during conversation
	_ = workflow.ExecuteActivity(localCtx, "ExecuteStep", activity.StepInput{
		TaskID: input.TaskID, StepType: "ANALYZE", TaskStatus: "ANALYZING", Duration: 0,
	}).Get(ctx, nil)
	_ = workflow.ExecuteActivity(localCtx, "SaveStepOutput", input.TaskID, "ANALYZE", map[string]interface{}{
		"status": "completed_during_conversation",
	}).Get(ctx, nil)

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

	// Save DAG nodes if plan has tasks
	if tasks, ok := planResult["tasks"].([]interface{}); ok && len(tasks) > 0 {
		taskNodes := make([]map[string]interface{}, 0, len(tasks))
		for _, t := range tasks {
			if node, ok := t.(map[string]interface{}); ok {
				taskNodes = append(taskNodes, node)
			}
		}
		_ = workflow.ExecuteActivity(localCtx, "SaveTaskNodes", input.TaskID, taskNodes).Get(ctx, nil)
	}

	// Save touched_files to task record (for version conflict detection)
	if touchedFiles, ok := planResult["touched_files"].(map[string]interface{}); ok {
		allFiles := make([]string, 0)
		for _, key := range []string{"create", "modify"} {
			if files, ok := touchedFiles[key].([]interface{}); ok {
				for _, f := range files {
					if s, ok := f.(string); ok {
						allFiles = append(allFiles, s)
					}
				}
			}
		}
		if len(allFiles) > 0 {
			_ = workflow.ExecuteActivity(localCtx, "SaveTouchedFiles", input.TaskID, input.TenantID, allFiles).Get(ctx, nil)
		}
	}

	// ---- Step 2: Test Writing (non-blocking) ----
	var testResult map[string]interface{}
	err = workflow.ExecuteActivity(localCtx, "ExecuteStep", activity.StepInput{
		TaskID: input.TaskID, StepType: "TEST_WRITING", TaskStatus: "TEST_WRITING", Duration: 0,
	}).Get(ctx, nil)
	if err != nil {
		logger.Warn("test writing step DB update failed, continuing", "error", err)
	} else {
		err = workflow.ExecuteActivity(aiCtx, "generate_test_cases", map[string]interface{}{
			"task_id":             input.TaskID,
			"tenant_id":           input.TenantID,
			"project_id":          input.ProjectID,
			"plan":                planResult,
			"requirement_summary": input.Requirement,
		}).Get(ctx, &testResult)
		if err != nil {
			logger.Warn("AI test writing failed, continuing without tests", "error", err)
			testResult = nil
		}
		_ = workflow.ExecuteActivity(localCtx, "SaveStepOutput", input.TaskID, "TEST_WRITING", testResult).Get(ctx, nil)
	}

	// ---- Step 3: Generate ----
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
		"test_cases":          testResult,
	}).Get(ctx, &generateResult)
	if err != nil {
		logger.Error("AI generate failed", "error", err)
		_ = workflow.ExecuteActivity(localCtx, "FailTask", input.TaskID, err.Error()).Get(ctx, nil)
		return err
	}

	_ = workflow.ExecuteActivity(localCtx, "SaveStepOutput", input.TaskID, "GENERATE", generateResult).Get(ctx, nil)

	// ---- Step 4: Review loop (max 3 attempts) ----
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

	// ---- Step 5: Test Execution ----
	// TODO(k8s): Currently runs mock tests. Replace with real K8s Job execution when available.
	err = workflow.ExecuteActivity(localCtx, "ExecuteStep", activity.StepInput{
		TaskID: input.TaskID, StepType: "TEST", TaskStatus: "TESTING", Duration: 0,
	}).Get(ctx, nil)
	if err != nil {
		logger.Warn("test step DB update failed, continuing", "error", err)
	} else {
		testInput := testResult
		if testInput == nil {
			testInput = map[string]interface{}{}
		}
		var runTestsOutput activity.RunTestsOutput
		testErr := workflow.ExecuteActivity(localCtx, "RunTests", input.TaskID, testInput).Get(ctx, &runTestsOutput)
		if testErr != nil {
			logger.Warn("test execution failed (non-blocking)", "error", testErr)
		}

		testStepOutput := map[string]interface{}{
			"status":       runTestsOutput.Status,
			"mock":         runTestsOutput.Mock,
			"framework":    runTestsOutput.Framework,
			"total":        runTestsOutput.Total,
			"passed":       runTestsOutput.Passed,
			"failed":       runTestsOutput.Failed,
			"coverage_pct": runTestsOutput.CoveragePct,
			"duration_ms":  runTestsOutput.DurationMs,
		}
		if runTestsOutput.K8sJob != "" {
			testStepOutput["k8s_job"] = runTestsOutput.K8sJob
		}
		if runTestsOutput.Logs != "" {
			testStepOutput["logs"] = runTestsOutput.Logs
		}
		_ = workflow.ExecuteActivity(localCtx, "SaveStepOutput", input.TaskID, "TEST", testStepOutput).Get(ctx, nil)
	}

	// ---- Step 6: Deploy (Push to GitHub) ----
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

			// Determine branch title: prefer plan title, fall back to workflow input title
			branchTitle := input.Title
			if t, ok := planResult["title"].(string); ok && t != "" {
				branchTitle = t
			}

			// Push to GitHub
			var pushResult activity.PushToGitHubOutput
			err = workflow.ExecuteActivity(localCtx, "PushToGitHub", activity.PushToGitHubInput{
				TaskID:        input.TaskID,
				TenantID:      input.TenantID,
				ProjectID:     input.ProjectID,
				CreatedBy:     input.CreatedBy,
				Title:         branchTitle,
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

			// Extract file stats from generate result
			filesChanged := 0
			linesAdded := 0
			linesDeleted := 0
			if fc, ok := generateResult["files_changed"].(float64); ok {
				filesChanged = int(fc)
			}
			if la, ok := generateResult["lines_added"].(float64); ok {
				linesAdded = int(la)
			}
			if ld, ok := generateResult["lines_deleted"].(float64); ok {
				linesDeleted = int(ld)
			}
			if filesChanged == 0 {
				if files, ok := generateResult["files"].([]interface{}); ok {
					filesChanged = len(files)
				}
			}

			_ = workflow.ExecuteActivity(localCtx, "SavePRInfo", activity.SavePRInfoInput{
				TaskID:       input.TaskID,
				PRNumber:     prResult.PRNumber,
				PRURL:        prResult.PRURL,
				ReviewScore:  reviewScore,
				BranchName:   pushResult.BranchName,
				FilesChanged: filesChanged,
				LinesAdded:   linesAdded,
				LinesDeleted: linesDeleted,
			}).Get(ctx, nil)

			_ = workflow.ExecuteActivity(localCtx, "SaveStepOutput", input.TaskID, "DEPLOY", map[string]interface{}{
				"branch_name": pushResult.BranchName,
				"pr_number":   prResult.PRNumber,
				"pr_url":      prResult.PRURL,
			}).Get(ctx, nil)

			// After CreatePullRequest succeeds, create preview environment
			// TODO: Replace mock with real K8s namespace when available
			_ = workflow.ExecuteActivity(localCtx, "CreatePreview", map[string]interface{}{
				"task_id":     input.TaskID,
				"project_id":  input.ProjectID,
				"tenant_id":   input.TenantID,
				"branch_name": pushResult.BranchName,
				"pr_number":   prResult.PRNumber,
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

	// ---- Step 7: Complete ----
	err = workflow.ExecuteActivity(localCtx, "CompleteTask", input.TaskID).Get(ctx, nil)
	if err != nil {
		logger.Error("failed to complete task", "error", err)
		return err
	}

	logger.Info("TaskWorkflow completed", "task_id", input.TaskID)
	return nil
}

// PlanOnlyWorkflow runs only the PLAN step and returns the plan result.
// Used by ConfirmPlan to generate a plan for human review before execution.
func PlanOnlyWorkflow(ctx workflow.Context, input activity.TaskWorkflowInput) (map[string]interface{}, error) {
	logger := workflow.GetLogger(ctx)
	logger.Info("PlanOnlyWorkflow started", "task_id", input.TaskID)

	localCtx := workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
		StartToCloseTimeout: 30 * time.Second,
		RetryPolicy:         &temporal.RetryPolicy{MaximumAttempts: 3},
	})

	aiCtx := workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
		TaskQueue:           "ai-worker",
		StartToCloseTimeout: 5 * time.Minute,
		RetryPolicy: &temporal.RetryPolicy{
			MaximumAttempts:    3,
			InitialInterval:    5 * time.Second,
			BackoffCoefficient: 2.0,
		},
	})

	// Mark PLAN step as RUNNING
	err := workflow.ExecuteActivity(localCtx, "ExecuteStep", activity.StepInput{
		TaskID: input.TaskID, StepType: "PLAN", TaskStatus: "PLANNING", Duration: 0,
	}).Get(ctx, nil)
	if err != nil {
		logger.Error("plan step DB update failed", "error", err)
		return nil, err
	}

	// Call AI planning activity
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
		return nil, err
	}

	// Save plan output
	_ = workflow.ExecuteActivity(localCtx, "SaveStepOutput", input.TaskID, "PLAN", planResult).Get(ctx, nil)

	// Save DAG nodes if plan has tasks
	if tasks, ok := planResult["tasks"].([]interface{}); ok && len(tasks) > 0 {
		taskNodes := make([]map[string]interface{}, 0, len(tasks))
		for _, t := range tasks {
			if node, ok := t.(map[string]interface{}); ok {
				taskNodes = append(taskNodes, node)
			}
		}
		_ = workflow.ExecuteActivity(localCtx, "SaveTaskNodes", input.TaskID, taskNodes).Get(ctx, nil)
	}

	logger.Info("PlanOnlyWorkflow completed", "task_id", input.TaskID)
	return planResult, nil
}

// TaskExecutionWorkflow runs the execution steps after plan approval:
// TEST_WRITING → GENERATE → REVIEW → TEST → DEPLOY
func TaskExecutionWorkflow(ctx workflow.Context, input activity.TaskWorkflowInput) error {
	logger := workflow.GetLogger(ctx)
	logger.Info("TaskExecutionWorkflow started", "task_id", input.TaskID)

	localCtx := workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
		StartToCloseTimeout: 30 * time.Second,
		RetryPolicy:         &temporal.RetryPolicy{MaximumAttempts: 3},
	})

	aiCtx := workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
		TaskQueue:           "ai-worker",
		StartToCloseTimeout: 5 * time.Minute,
		RetryPolicy: &temporal.RetryPolicy{
			MaximumAttempts:    3,
			InitialInterval:    5 * time.Second,
			BackoffCoefficient: 2.0,
		},
	})

	// Load plan result from saved step output (passed via input or fetched)
	planResult := input.PlanResult

	// ---- Step: Test Writing (non-blocking) ----
	var testResult map[string]interface{}
	err := workflow.ExecuteActivity(localCtx, "ExecuteStep", activity.StepInput{
		TaskID: input.TaskID, StepType: "TEST_WRITING", TaskStatus: "TEST_WRITING", Duration: 0,
	}).Get(ctx, nil)
	if err != nil {
		logger.Warn("test writing step DB update failed, continuing", "error", err)
	} else {
		err = workflow.ExecuteActivity(aiCtx, "generate_test_cases", map[string]interface{}{
			"task_id":             input.TaskID,
			"tenant_id":           input.TenantID,
			"project_id":          input.ProjectID,
			"plan":                planResult,
			"requirement_summary": input.Requirement,
		}).Get(ctx, &testResult)
		if err != nil {
			logger.Warn("AI test writing failed, continuing without tests", "error", err)
			testResult = nil
		}
		_ = workflow.ExecuteActivity(localCtx, "SaveStepOutput", input.TaskID, "TEST_WRITING", testResult).Get(ctx, nil)
	}

	// ---- Step: Generate ----
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
		"test_cases":          testResult,
	}).Get(ctx, &generateResult)
	if err != nil {
		logger.Error("AI generate failed", "error", err)
		_ = workflow.ExecuteActivity(localCtx, "FailTask", input.TaskID, err.Error()).Get(ctx, nil)
		return err
	}

	_ = workflow.ExecuteActivity(localCtx, "SaveStepOutput", input.TaskID, "GENERATE", generateResult).Get(ctx, nil)

	// ---- Step: Review loop (max 3 attempts) ----
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

		passed, _ := reviewResult["passed"].(bool)
		if passed {
			break
		}

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

	// ---- Step: Test Execution ----
	err = workflow.ExecuteActivity(localCtx, "ExecuteStep", activity.StepInput{
		TaskID: input.TaskID, StepType: "TEST", TaskStatus: "TESTING", Duration: 0,
	}).Get(ctx, nil)
	if err != nil {
		logger.Warn("test step DB update failed, continuing", "error", err)
	} else {
		testInput := testResult
		if testInput == nil {
			testInput = map[string]interface{}{}
		}
		var runTestsOutput activity.RunTestsOutput
		testErr := workflow.ExecuteActivity(localCtx, "RunTests", input.TaskID, testInput).Get(ctx, &runTestsOutput)
		if testErr != nil {
			logger.Warn("test execution failed (non-blocking)", "error", testErr)
		}

		testStepOutput := map[string]interface{}{
			"status":       runTestsOutput.Status,
			"mock":         runTestsOutput.Mock,
			"framework":    runTestsOutput.Framework,
			"total":        runTestsOutput.Total,
			"passed":       runTestsOutput.Passed,
			"failed":       runTestsOutput.Failed,
			"coverage_pct": runTestsOutput.CoveragePct,
			"duration_ms":  runTestsOutput.DurationMs,
		}
		if runTestsOutput.K8sJob != "" {
			testStepOutput["k8s_job"] = runTestsOutput.K8sJob
		}
		if runTestsOutput.Logs != "" {
			testStepOutput["logs"] = runTestsOutput.Logs
		}
		_ = workflow.ExecuteActivity(localCtx, "SaveStepOutput", input.TaskID, "TEST", testStepOutput).Get(ctx, nil)
	}

	// ---- Step: Deploy ----
	err = workflow.ExecuteActivity(localCtx, "ExecuteStep", activity.StepInput{
		TaskID: input.TaskID, StepType: "DEPLOY", TaskStatus: "DEPLOYING", Duration: 0,
	}).Get(ctx, nil)
	if err != nil {
		logger.Error("deploy step DB update failed", "error", err)
	} else {
		deployErr := func() error {
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

			branchTitle := input.Title
			if t, ok := planResult["title"].(string); ok && t != "" {
				branchTitle = t
			}

			var pushResult activity.PushToGitHubOutput
			err = workflow.ExecuteActivity(localCtx, "PushToGitHub", activity.PushToGitHubInput{
				TaskID:        input.TaskID,
				TenantID:      input.TenantID,
				ProjectID:     input.ProjectID,
				CreatedBy:     input.CreatedBy,
				Title:         branchTitle,
				Files:         generateResult["files"],
				CommitMessage: commitMsg,
			}).Get(ctx, &pushResult)
			if err != nil {
				return err
			}

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

			reviewScore := 0
			if s, ok := reviewResult["score"].(float64); ok {
				reviewScore = int(s)
			}

			// Extract file stats from generate result
			filesChanged := 0
			linesAdded := 0
			linesDeleted := 0
			if fc, ok := generateResult["files_changed"].(float64); ok {
				filesChanged = int(fc)
			}
			if la, ok := generateResult["lines_added"].(float64); ok {
				linesAdded = int(la)
			}
			if ld, ok := generateResult["lines_deleted"].(float64); ok {
				linesDeleted = int(ld)
			}
			if filesChanged == 0 {
				if files, ok := generateResult["files"].([]interface{}); ok {
					filesChanged = len(files)
				}
			}

			_ = workflow.ExecuteActivity(localCtx, "SavePRInfo", activity.SavePRInfoInput{
				TaskID:       input.TaskID,
				PRNumber:     prResult.PRNumber,
				PRURL:        prResult.PRURL,
				ReviewScore:  reviewScore,
				BranchName:   pushResult.BranchName,
				FilesChanged: filesChanged,
				LinesAdded:   linesAdded,
				LinesDeleted: linesDeleted,
			}).Get(ctx, nil)

			_ = workflow.ExecuteActivity(localCtx, "SaveStepOutput", input.TaskID, "DEPLOY", map[string]interface{}{
				"branch_name": pushResult.BranchName,
				"pr_number":   prResult.PRNumber,
				"pr_url":      prResult.PRURL,
			}).Get(ctx, nil)

			_ = workflow.ExecuteActivity(localCtx, "CreatePreview", map[string]interface{}{
				"task_id":     input.TaskID,
				"project_id":  input.ProjectID,
				"tenant_id":   input.TenantID,
				"branch_name": pushResult.BranchName,
				"pr_number":   prResult.PRNumber,
			}).Get(ctx, nil)

			return nil
		}()

		if deployErr != nil {
			logger.Warn("deploy step failed (non-fatal)", "task_id", input.TaskID, "error", deployErr)
			_ = workflow.ExecuteActivity(localCtx, "SaveStepOutput", input.TaskID, "DEPLOY", map[string]interface{}{
				"skipped": true,
				"error":   deployErr.Error(),
			}).Get(ctx, nil)
		}
	}

	// ---- Complete ----
	err = workflow.ExecuteActivity(localCtx, "CompleteTask", input.TaskID).Get(ctx, nil)
	if err != nil {
		logger.Error("failed to complete task", "error", err)
		return err
	}

	logger.Info("TaskExecutionWorkflow completed", "task_id", input.TaskID)
	return nil
}

// ProfileScanWorkflow triggers the Python scan_project_profile activity.
// Called from profile.Service.TriggerScan and auto-triggered on project import.
func ProfileScanWorkflow(ctx workflow.Context, input map[string]interface{}) (map[string]interface{}, error) {
	logger := workflow.GetLogger(ctx)
	logger.Info("ProfileScanWorkflow started", "project_id", input["project_id"])

	aiCtx := workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
		TaskQueue:           "ai-worker",
		StartToCloseTimeout: 5 * time.Minute, // Profile scanning reads many files, needs more time
		HeartbeatTimeout:    30 * time.Second,
		RetryPolicy: &temporal.RetryPolicy{
			MaximumAttempts:    2,
			InitialInterval:    5 * time.Second,
			BackoffCoefficient: 2.0,
		},
	})

	var result map[string]interface{}
	err := workflow.ExecuteActivity(aiCtx, "scan_project_profile", input).Get(ctx, &result)
	if err != nil {
		logger.Error("scan_project_profile activity failed", "error", err)
		return nil, err
	}

	logger.Info("ProfileScanWorkflow completed",
		"dimensions_scanned", result["dimensions_scanned"],
		"dimensions_failed", result["dimensions_failed"],
	)
	return result, nil
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
