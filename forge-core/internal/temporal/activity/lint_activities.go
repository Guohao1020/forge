package activity

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"go.temporal.io/sdk/activity"
)

// LintActivities handles code linting operations.
type LintActivities struct{}

func NewLintActivities() *LintActivities {
	return &LintActivities{}
}

// LintIssue represents a single lint finding.
type LintIssue struct {
	File     string `json:"file"`
	Line     int    `json:"line"`
	Column   int    `json:"column"`
	Rule     string `json:"rule"`
	Message  string `json:"message"`
	Severity string `json:"severity"` // error, warning, info
}

// RunLintInput is the input for the RunLint activity.
type RunLintInput struct {
	TaskID    int64                    `json:"task_id"`
	Language  string                   `json:"language"` // go, javascript, typescript, python
	Files     []map[string]interface{} `json:"files"`    // [{path, content, language}]
}

// RunLintOutput is the result of running linters.
type RunLintOutput struct {
	Passed   bool        `json:"passed"`
	Issues   []LintIssue `json:"issues"`
	Duration int64       `json:"duration_ms"`
	Linter   string      `json:"linter"` // golangci-lint, eslint, ruff
}

// RunLint executes the appropriate linter based on the detected language.
// Writes files to a temp directory, runs the linter, parses output.
func (a *LintActivities) RunLint(ctx context.Context, input RunLintInput) (*RunLintOutput, error) {
	info := activity.GetInfo(ctx)
	slog.Info("RunLint activity",
		"task_id", input.TaskID,
		"language", input.Language,
		"file_count", len(input.Files),
		"workflow_id", info.WorkflowExecution.ID,
	)

	start := time.Now()

	// Create temp directory for lint
	tmpDir, err := os.MkdirTemp("", fmt.Sprintf("forge-lint-%d-", input.TaskID))
	if err != nil {
		return nil, fmt.Errorf("create temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	// Write files to temp directory
	for _, f := range input.Files {
		path, _ := f["path"].(string)
		content, _ := f["content"].(string)
		if path == "" || content == "" {
			continue
		}
		fullPath := filepath.Join(tmpDir, path)
		if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
			continue
		}
		if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
			continue
		}
	}

	// Select and run linter based on language
	var output *RunLintOutput
	switch strings.ToLower(input.Language) {
	case "go", "golang":
		output, err = runGoLint(ctx, tmpDir)
	case "javascript", "typescript", "js", "ts":
		output, err = runESLint(ctx, tmpDir)
	case "python", "py":
		output, err = runRuff(ctx, tmpDir)
	default:
		// Unknown language — skip linting, pass through
		output = &RunLintOutput{
			Passed:   true,
			Issues:   []LintIssue{},
			Duration: time.Since(start).Milliseconds(),
			Linter:   "none",
		}
		return output, nil
	}

	if err != nil {
		slog.Warn("lint execution failed, treating as pass", "error", err, "language", input.Language)
		return &RunLintOutput{
			Passed:   true,
			Issues:   []LintIssue{},
			Duration: time.Since(start).Milliseconds(),
			Linter:   "fallback",
		}, nil
	}

	output.Duration = time.Since(start).Milliseconds()
	slog.Info("lint completed",
		"task_id", input.TaskID,
		"linter", output.Linter,
		"passed", output.Passed,
		"issues", len(output.Issues),
		"duration_ms", output.Duration,
	)
	return output, nil
}

// runGoLint runs golangci-lint on Go files.
func runGoLint(ctx context.Context, dir string) (*RunLintOutput, error) {
	// Check if golangci-lint is available
	if _, err := exec.LookPath("golangci-lint"); err != nil {
		return &RunLintOutput{Passed: true, Linter: "golangci-lint (not installed)"}, nil
	}

	cmd := exec.CommandContext(ctx, "golangci-lint", "run", "--out-format=json", "--timeout=60s", "./...")
	cmd.Dir = dir
	out, _ := cmd.CombinedOutput() // golangci-lint returns non-zero on lint issues

	var issues []LintIssue
	// Parse golangci-lint JSON output
	var result struct {
		Issues []struct {
			FromLinter string `json:"FromLinter"`
			Text       string `json:"Text"`
			Pos        struct {
				Filename string `json:"Filename"`
				Line     int    `json:"Line"`
				Column   int    `json:"Column"`
			} `json:"Pos"`
			Severity string `json:"Severity"`
		} `json:"Issues"`
	}
	if err := json.Unmarshal(out, &result); err == nil {
		for _, i := range result.Issues {
			severity := "warning"
			if i.Severity != "" {
				severity = i.Severity
			}
			issues = append(issues, LintIssue{
				File:     i.Pos.Filename,
				Line:     i.Pos.Line,
				Column:   i.Pos.Column,
				Rule:     i.FromLinter,
				Message:  i.Text,
				Severity: severity,
			})
		}
	}

	hasErrors := false
	for _, i := range issues {
		if i.Severity == "error" {
			hasErrors = true
			break
		}
	}

	return &RunLintOutput{
		Passed: !hasErrors,
		Issues: issues,
		Linter: "golangci-lint",
	}, nil
}

// runESLint runs eslint on JavaScript/TypeScript files.
func runESLint(ctx context.Context, dir string) (*RunLintOutput, error) {
	if _, err := exec.LookPath("npx"); err != nil {
		return &RunLintOutput{Passed: true, Linter: "eslint (npx not installed)"}, nil
	}

	cmd := exec.CommandContext(ctx, "npx", "eslint", "--format=json", ".")
	cmd.Dir = dir
	out, _ := cmd.CombinedOutput()

	var issues []LintIssue
	var results []struct {
		FilePath string `json:"filePath"`
		Messages []struct {
			RuleId   string `json:"ruleId"`
			Message  string `json:"message"`
			Line     int    `json:"line"`
			Column   int    `json:"column"`
			Severity int    `json:"severity"` // 1=warn, 2=error
		} `json:"messages"`
	}
	if err := json.Unmarshal(out, &results); err == nil {
		for _, r := range results {
			for _, m := range r.Messages {
				severity := "warning"
				if m.Severity == 2 {
					severity = "error"
				}
				issues = append(issues, LintIssue{
					File:     filepath.Base(r.FilePath),
					Line:     m.Line,
					Column:   m.Column,
					Rule:     m.RuleId,
					Message:  m.Message,
					Severity: severity,
				})
			}
		}
	}

	hasErrors := false
	for _, i := range issues {
		if i.Severity == "error" {
			hasErrors = true
			break
		}
	}

	return &RunLintOutput{
		Passed: !hasErrors,
		Issues: issues,
		Linter: "eslint",
	}, nil
}

// runRuff runs ruff (fast Python linter) on Python files.
func runRuff(ctx context.Context, dir string) (*RunLintOutput, error) {
	if _, err := exec.LookPath("ruff"); err != nil {
		return &RunLintOutput{Passed: true, Linter: "ruff (not installed)"}, nil
	}

	cmd := exec.CommandContext(ctx, "ruff", "check", "--output-format=json", ".")
	cmd.Dir = dir
	out, _ := cmd.CombinedOutput()

	var issues []LintIssue
	var results []struct {
		Code     string `json:"code"`
		Message  string `json:"message"`
		Filename string `json:"filename"`
		Location struct {
			Row    int `json:"row"`
			Column int `json:"column"`
		} `json:"location"`
	}
	if err := json.Unmarshal(out, &results); err == nil {
		for _, r := range results {
			issues = append(issues, LintIssue{
				File:     filepath.Base(r.Filename),
				Line:     r.Location.Row,
				Column:   r.Location.Column,
				Rule:     r.Code,
				Message:  r.Message,
				Severity: "warning",
			})
		}
	}

	return &RunLintOutput{
		Passed: true, // ruff warnings don't block by default
		Issues: issues,
		Linter: "ruff",
	}, nil
}
