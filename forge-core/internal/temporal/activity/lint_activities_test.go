package activity

import (
	"testing"
)

func TestLintIssueStruct(t *testing.T) {
	issue := LintIssue{
		File:     "main.go",
		Line:     42,
		Column:   10,
		Rule:     "errcheck",
		Message:  "Error return value not checked",
		Severity: "error",
	}

	if issue.File != "main.go" {
		t.Errorf("File = %q, want main.go", issue.File)
	}
	if issue.Severity != "error" {
		t.Errorf("Severity = %q, want error", issue.Severity)
	}
}

func TestRunLintInputDefaults(t *testing.T) {
	input := RunLintInput{
		TaskID:   1,
		Language: "go",
		Files:    nil,
	}

	if input.Language != "go" {
		t.Errorf("Language = %q, want go", input.Language)
	}
	if input.Files != nil {
		t.Errorf("Files should be nil by default")
	}
}

func TestRunLintOutputPassed(t *testing.T) {
	output := RunLintOutput{
		Passed:   true,
		Issues:   []LintIssue{},
		Duration: 150,
		Linter:   "golangci-lint",
	}

	if !output.Passed {
		t.Error("expected Passed=true")
	}
	if len(output.Issues) != 0 {
		t.Errorf("expected 0 issues, got %d", len(output.Issues))
	}
}

func TestRunLintOutputFailed(t *testing.T) {
	output := RunLintOutput{
		Passed: false,
		Issues: []LintIssue{
			{File: "main.go", Line: 10, Rule: "errcheck", Message: "unchecked error", Severity: "error"},
			{File: "main.go", Line: 20, Rule: "unused", Message: "unused variable", Severity: "warning"},
		},
		Linter: "golangci-lint",
	}

	if output.Passed {
		t.Error("expected Passed=false")
	}
	if len(output.Issues) != 2 {
		t.Errorf("expected 2 issues, got %d", len(output.Issues))
	}

	errorCount := 0
	for _, i := range output.Issues {
		if i.Severity == "error" {
			errorCount++
		}
	}
	if errorCount != 1 {
		t.Errorf("expected 1 error-severity issue, got %d", errorCount)
	}
}
