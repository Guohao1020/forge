package workflow

import (
	"time"

	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"

	"github.com/shulex/forge/forge-core/internal/temporal/activity"
)

// EntropyScanWorkflow runs periodic code quality scans for a project.
// It detects naming violations, dead code patterns, missing error handling,
// and tracks quality trends over time.
func EntropyScanWorkflow(ctx workflow.Context, input activity.EntropyScanInput) (*activity.EntropyScanOutput, error) {
	logger := workflow.GetLogger(ctx)
	logger.Info("EntropyScanWorkflow started", "project_id", input.ProjectID)

	ao := workflow.ActivityOptions{
		StartToCloseTimeout: 5 * time.Minute,
		RetryPolicy: &temporal.RetryPolicy{
			InitialInterval:    10 * time.Second,
			BackoffCoefficient: 2.0,
			MaximumAttempts:    3,
		},
	}
	ctx = workflow.WithActivityOptions(ctx, ao)

	// Step 1: Fetch project files for scanning
	var fetchOutput activity.FetchProjectFilesOutput
	err := workflow.ExecuteActivity(ctx, "FetchProjectFiles", activity.FetchProjectFilesInput{
		ProjectID: input.ProjectID,
		TenantID:  input.TenantID,
	}).Get(ctx, &fetchOutput)
	if err != nil {
		logger.Error("FetchProjectFiles failed", "error", err)
		return nil, err
	}

	if len(fetchOutput.Files) == 0 {
		logger.Info("no files to scan")
		return &activity.EntropyScanOutput{
			ProjectID: input.ProjectID,
			Score:     100,
			Issues:    []activity.EntropyIssue{},
		}, nil
	}

	// Step 2: Run lint scan on fetched files
	var lintOutput activity.EntropyLintOutput
	err = workflow.ExecuteActivity(ctx, "RunEntropyLint", activity.EntropyLintInput{
		ProjectID: input.ProjectID,
		Language:  fetchOutput.Language,
		Files:     fetchOutput.Files,
	}).Get(ctx, &lintOutput)
	if err != nil {
		logger.Warn("RunEntropyLint failed, continuing with AI scan", "error", err)
		lintOutput = activity.EntropyLintOutput{Issues: []activity.EntropyIssue{}}
	}

	// Step 3: Run AI-powered pattern scan (naming, dead code, missing docs)
	var aiOutput activity.EntropyAIScanOutput
	err = workflow.ExecuteActivity(ctx, "RunEntropyAIScan", activity.EntropyAIScanInput{
		ProjectID: input.ProjectID,
		Language:  fetchOutput.Language,
		Files:     fetchOutput.Files,
		Rules:     input.Rules,
	}).Get(ctx, &aiOutput)
	if err != nil {
		logger.Warn("RunEntropyAIScan failed, using lint results only", "error", err)
		aiOutput = activity.EntropyAIScanOutput{Issues: []activity.EntropyIssue{}}
	}

	// Step 4: Merge results and compute quality score
	allIssues := append(lintOutput.Issues, aiOutput.Issues...)
	score := computeQualityScore(allIssues)

	result := &activity.EntropyScanOutput{
		ProjectID:   input.ProjectID,
		Score:       score,
		Issues:      allIssues,
		ScannedAt:   workflow.Now(ctx).Format(time.RFC3339),
		FileCount:   len(fetchOutput.Files),
		Language:    fetchOutput.Language,
		LintIssues:  len(lintOutput.Issues),
		AIIssues:    len(aiOutput.Issues),
	}

	// Step 5: Save quality trends to DB
	err = workflow.ExecuteActivity(ctx, "SaveEntropyResults", activity.SaveEntropyInput{
		ProjectID: input.ProjectID,
		TenantID:  input.TenantID,
		Score:     score,
		Issues:    allIssues,
		ScannedAt: result.ScannedAt,
	}).Get(ctx, nil)
	if err != nil {
		logger.Warn("SaveEntropyResults failed", "error", err)
	}

	// Step 6: Create auto-fix PR if configured and issues found
	if input.AutoFix && len(allIssues) > 0 {
		var fixOutput activity.AutoFixOutput
		err = workflow.ExecuteActivity(ctx, "CreateAutoFixPR", activity.AutoFixInput{
			ProjectID: input.ProjectID,
			TenantID:  input.TenantID,
			Issues:    allIssues,
			Language:  fetchOutput.Language,
		}).Get(ctx, &fixOutput)
		if err != nil {
			logger.Warn("CreateAutoFixPR failed", "error", err)
		} else {
			result.FixPRURL = fixOutput.PRURL
		}
	}

	logger.Info("EntropyScanWorkflow completed",
		"project_id", input.ProjectID,
		"score", score,
		"issues", len(allIssues),
	)
	return result, nil
}

// computeQualityScore calculates a 0-100 quality score based on issues.
// Starts at 100, deducts points per issue severity.
func computeQualityScore(issues []activity.EntropyIssue) int {
	score := 100
	for _, issue := range issues {
		switch issue.Severity {
		case "critical":
			score -= 10
		case "error":
			score -= 5
		case "warning":
			score -= 2
		case "info":
			score -= 1
		}
	}
	if score < 0 {
		score = 0
	}
	return score
}
