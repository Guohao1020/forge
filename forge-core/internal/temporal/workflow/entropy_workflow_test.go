package workflow

import (
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	sdkactivity "go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/testsuite"

	"github.com/shulex/forge/forge-core/internal/temporal/activity"
)

func registerEntropyActivities(env *testsuite.TestWorkflowEnvironment) {
	ea := activity.NewEntropyActivities(nil)
	env.RegisterActivityWithOptions(ea.FetchProjectFiles, sdkactivity.RegisterOptions{Name: "FetchProjectFiles"})
	env.RegisterActivityWithOptions(ea.RunEntropyLint, sdkactivity.RegisterOptions{Name: "RunEntropyLint"})
	env.RegisterActivityWithOptions(ea.RunEntropyAIScan, sdkactivity.RegisterOptions{Name: "RunEntropyAIScan"})
	env.RegisterActivityWithOptions(ea.SaveEntropyResults, sdkactivity.RegisterOptions{Name: "SaveEntropyResults"})
	env.RegisterActivityWithOptions(ea.CreateAutoFixPR, sdkactivity.RegisterOptions{Name: "CreateAutoFixPR"})
}

func TestEntropyScanWorkflow(t *testing.T) {
	s := testsuite.WorkflowTestSuite{}
	env := s.NewTestWorkflowEnvironment()
	registerEntropyActivities(env)

	// Mock FetchProjectFiles
	env.OnActivity("FetchProjectFiles", mock.Anything, activity.FetchProjectFilesInput{
		ProjectID: 1,
		TenantID:  1,
	}).Return(&activity.FetchProjectFilesOutput{
		Files: []activity.ProjectFile{
			{Path: "main.go", Content: "package main\nfunc main() {}"},
			{Path: "handler.go", Content: "package main\nfunc handler() {}"},
		},
		Language: "go",
	}, nil)

	// Mock RunEntropyLint
	env.OnActivity("RunEntropyLint", mock.Anything, mock.Anything).Return(&activity.EntropyLintOutput{
		Issues: []activity.EntropyIssue{
			{
				File:     "main.go",
				Line:     2,
				Rule:     "errcheck",
				Message:  "unchecked error",
				Severity: "warning",
				Category: "error_handling",
			},
		},
	}, nil)

	// Mock RunEntropyAIScan
	env.OnActivity("RunEntropyAIScan", mock.Anything, mock.Anything).Return(&activity.EntropyAIScanOutput{
		Issues: []activity.EntropyIssue{},
	}, nil)

	// Mock SaveEntropyResults
	env.OnActivity("SaveEntropyResults", mock.Anything, mock.Anything).Return(nil)

	input := activity.EntropyScanInput{
		ProjectID: 1,
		TenantID:  1,
		AutoFix:   false,
	}

	env.ExecuteWorkflow(EntropyScanWorkflow, input)

	require.True(t, env.IsWorkflowCompleted())
	require.NoError(t, env.GetWorkflowError())

	var result activity.EntropyScanOutput
	require.NoError(t, env.GetWorkflowResult(&result))
	require.Equal(t, int64(1), result.ProjectID)
	require.Equal(t, 98, result.Score) // 100 - 2 (1 warning)
	require.Len(t, result.Issues, 1)
	require.Equal(t, "go", result.Language)
	require.Equal(t, 2, result.FileCount)
}

func TestEntropyScanWorkflow_NoFiles(t *testing.T) {
	s := testsuite.WorkflowTestSuite{}
	env := s.NewTestWorkflowEnvironment()
	registerEntropyActivities(env)

	env.OnActivity("FetchProjectFiles", mock.Anything, mock.Anything).Return(&activity.FetchProjectFilesOutput{
		Files:    []activity.ProjectFile{},
		Language: "go",
	}, nil)

	input := activity.EntropyScanInput{
		ProjectID: 2,
		TenantID:  1,
	}

	env.ExecuteWorkflow(EntropyScanWorkflow, input)

	require.True(t, env.IsWorkflowCompleted())
	require.NoError(t, env.GetWorkflowError())

	var result activity.EntropyScanOutput
	require.NoError(t, env.GetWorkflowResult(&result))
	require.Equal(t, 100, result.Score)
	require.Empty(t, result.Issues)
}

func TestEntropyScanWorkflow_WithAutoFix(t *testing.T) {
	s := testsuite.WorkflowTestSuite{}
	env := s.NewTestWorkflowEnvironment()
	registerEntropyActivities(env)

	env.OnActivity("FetchProjectFiles", mock.Anything, mock.Anything).Return(&activity.FetchProjectFilesOutput{
		Files:    []activity.ProjectFile{{Path: "main.go", Content: "package main"}},
		Language: "go",
	}, nil)

	env.OnActivity("RunEntropyLint", mock.Anything, mock.Anything).Return(&activity.EntropyLintOutput{
		Issues: []activity.EntropyIssue{
			{File: "main.go", Line: 1, Rule: "unused", Message: "unused var", Severity: "error", Category: "dead_code"},
		},
	}, nil)

	env.OnActivity("RunEntropyAIScan", mock.Anything, mock.Anything).Return(&activity.EntropyAIScanOutput{
		Issues: []activity.EntropyIssue{},
	}, nil)

	env.OnActivity("SaveEntropyResults", mock.Anything, mock.Anything).Return(nil)

	env.OnActivity("CreateAutoFixPR", mock.Anything, mock.Anything).Return(&activity.AutoFixOutput{
		PRURL: "https://github.com/test/repo/pull/5",
	}, nil)

	input := activity.EntropyScanInput{
		ProjectID: 1,
		TenantID:  1,
		AutoFix:   true,
	}

	env.ExecuteWorkflow(EntropyScanWorkflow, input)

	require.True(t, env.IsWorkflowCompleted())
	require.NoError(t, env.GetWorkflowError())

	var result activity.EntropyScanOutput
	require.NoError(t, env.GetWorkflowResult(&result))
	require.Equal(t, 95, result.Score) // 100 - 5 (1 error)
	require.Equal(t, "https://github.com/test/repo/pull/5", result.FixPRURL)
}

func TestComputeQualityScore(t *testing.T) {
	tests := []struct {
		name   string
		issues []activity.EntropyIssue
		want   int
	}{
		{"no issues", nil, 100},
		{"1 warning", []activity.EntropyIssue{{Severity: "warning"}}, 98},
		{"1 error", []activity.EntropyIssue{{Severity: "error"}}, 95},
		{"1 critical", []activity.EntropyIssue{{Severity: "critical"}}, 90},
		{"1 info", []activity.EntropyIssue{{Severity: "info"}}, 99},
		{"mixed", []activity.EntropyIssue{
			{Severity: "critical"},
			{Severity: "error"},
			{Severity: "warning"},
			{Severity: "info"},
		}, 82},
		{"many issues floor at 0", func() []activity.EntropyIssue {
			issues := make([]activity.EntropyIssue, 20)
			for i := range issues {
				issues[i] = activity.EntropyIssue{Severity: "critical"}
			}
			return issues
		}(), 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := computeQualityScore(tt.issues)
			require.Equal(t, tt.want, got)
		})
	}
}
