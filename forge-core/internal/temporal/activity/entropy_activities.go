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

	"github.com/jackc/pgx/v5/pgxpool"
	"go.temporal.io/sdk/activity"
)

// --- Input/Output Types ---

type EntropyScanInput struct {
	ProjectID int64    `json:"project_id"`
	TenantID  int64    `json:"tenant_id"`
	Rules     []string `json:"rules"` // optional rule filters
	AutoFix   bool     `json:"auto_fix"`
}

type EntropyScanOutput struct {
	ProjectID  int64          `json:"project_id"`
	Score      int            `json:"score"` // 0-100 quality score
	Issues     []EntropyIssue `json:"issues"`
	ScannedAt  string         `json:"scanned_at"`
	FileCount  int            `json:"file_count"`
	Language   string         `json:"language"`
	LintIssues int            `json:"lint_issues"`
	AIIssues   int            `json:"ai_issues"`
	FixPRURL   string         `json:"fix_pr_url,omitempty"`
}

type EntropyIssue struct {
	File        string `json:"file"`
	Line        int    `json:"line"`
	Rule        string `json:"rule"`
	Message     string `json:"message"`
	Severity    string `json:"severity"` // critical, error, warning, info
	Category    string `json:"category"` // naming, dead_code, error_handling, complexity, style
	Suggestion  string `json:"suggestion,omitempty"`
	AutoFixable bool   `json:"auto_fixable"`
}

type FetchProjectFilesInput struct {
	ProjectID int64 `json:"project_id"`
	TenantID  int64 `json:"tenant_id"`
}

type FetchProjectFilesOutput struct {
	Files    []ProjectFile `json:"files"`
	Language string        `json:"language"`
}

type ProjectFile struct {
	Path    string `json:"path"`
	Content string `json:"content"`
}

type EntropyLintInput struct {
	ProjectID int64         `json:"project_id"`
	Language  string        `json:"language"`
	Files     []ProjectFile `json:"files"`
}

type EntropyLintOutput struct {
	Issues []EntropyIssue `json:"issues"`
}

type EntropyAIScanInput struct {
	ProjectID int64         `json:"project_id"`
	Language  string        `json:"language"`
	Files     []ProjectFile `json:"files"`
	Rules     []string      `json:"rules"`
}

type EntropyAIScanOutput struct {
	Issues []EntropyIssue `json:"issues"`
}

type SaveEntropyInput struct {
	ProjectID int64          `json:"project_id"`
	TenantID  int64          `json:"tenant_id"`
	Score     int            `json:"score"`
	Issues    []EntropyIssue `json:"issues"`
	ScannedAt string         `json:"scanned_at"`
}

type AutoFixInput struct {
	ProjectID int64          `json:"project_id"`
	TenantID  int64          `json:"tenant_id"`
	Issues    []EntropyIssue `json:"issues"`
	Language  string         `json:"language"`
}

type AutoFixOutput struct {
	PRURL string `json:"pr_url"`
}

// --- Activities ---

type EntropyActivities struct {
	db *pgxpool.Pool
}

func NewEntropyActivities(db *pgxpool.Pool) *EntropyActivities {
	return &EntropyActivities{db: db}
}

// FetchProjectFiles retrieves source files from the local workspace for scanning.
func (a *EntropyActivities) FetchProjectFiles(ctx context.Context, input FetchProjectFilesInput) (*FetchProjectFilesOutput, error) {
	info := activity.GetInfo(ctx)
	slog.Info("FetchProjectFiles",
		"project_id", input.ProjectID,
		"workflow_id", info.WorkflowExecution.ID,
	)

	// Query project details to find workspace path and language
	var fullName, language string
	err := a.db.QueryRow(ctx,
		`SELECT COALESCE(p.full_name, ''), COALESCE(p.language, 'unknown')
		 FROM engine.projects p WHERE p.id = $1`,
		input.ProjectID,
	).Scan(&fullName, &language)
	if err != nil {
		return nil, fmt.Errorf("query project: %w", err)
	}

	// Try local workspace first
	wsDir := fmt.Sprintf("workspaces/tenant-%d/project-%d/repo", input.TenantID, input.ProjectID)
	files, scanErr := scanLocalFiles(wsDir, language)
	if scanErr != nil || len(files) == 0 {
		slog.Warn("no local files found", "ws_dir", wsDir, "error", scanErr)
		return &FetchProjectFilesOutput{Files: []ProjectFile{}, Language: language}, nil
	}

	return &FetchProjectFilesOutput{
		Files:    files,
		Language: language,
	}, nil
}

// scanLocalFiles reads source files from a workspace directory.
func scanLocalFiles(dir string, language string) ([]ProjectFile, error) {
	var extensions []string
	switch strings.ToLower(language) {
	case "go", "golang":
		extensions = []string{".go"}
	case "javascript", "typescript":
		extensions = []string{".js", ".ts", ".tsx", ".jsx"}
	case "python":
		extensions = []string{".py"}
	case "java":
		extensions = []string{".java"}
	default:
		extensions = []string{".go", ".js", ".ts", ".py", ".java"}
	}

	var files []ProjectFile
	maxFiles := 50   // limit for scan
	maxSize := 50000 // 50KB per file

	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // skip unreadable
		}
		if info.IsDir() {
			base := filepath.Base(path)
			// Skip common non-source directories
			if base == "node_modules" || base == ".git" || base == "vendor" ||
				base == "__pycache__" || base == ".next" || base == "dist" || base == "build" {
				return filepath.SkipDir
			}
			return nil
		}
		if len(files) >= maxFiles {
			return filepath.SkipAll
		}
		if info.Size() > int64(maxSize) {
			return nil
		}

		ext := strings.ToLower(filepath.Ext(path))
		for _, allowed := range extensions {
			if ext == allowed {
				content, readErr := os.ReadFile(path)
				if readErr != nil {
					return nil
				}
				relPath, _ := filepath.Rel(dir, path)
				files = append(files, ProjectFile{
					Path:    filepath.ToSlash(relPath),
					Content: string(content),
				})
				break
			}
		}
		return nil
	})
	return files, err
}

// RunEntropyLint runs linters for entropy scanning (reuses lint infrastructure).
func (a *EntropyActivities) RunEntropyLint(ctx context.Context, input EntropyLintInput) (*EntropyLintOutput, error) {
	slog.Info("RunEntropyLint", "project_id", input.ProjectID, "language", input.Language, "files", len(input.Files))

	// Create temp directory
	tmpDir, err := os.MkdirTemp("", fmt.Sprintf("entropy-lint-%d-", input.ProjectID))
	if err != nil {
		return nil, fmt.Errorf("create temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	// Write files
	for _, f := range input.Files {
		fullPath := filepath.Join(tmpDir, f.Path)
		if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
			continue
		}
		if err := os.WriteFile(fullPath, []byte(f.Content), 0644); err != nil {
			continue
		}
	}

	var issues []EntropyIssue

	// Run appropriate linter
	switch strings.ToLower(input.Language) {
	case "go", "golang":
		issues = runGoEntropyLint(ctx, tmpDir)
	case "javascript", "typescript":
		issues = runJSEntropyLint(ctx, tmpDir)
	case "python":
		issues = runPythonEntropyLint(ctx, tmpDir)
	}

	return &EntropyLintOutput{Issues: issues}, nil
}

func runGoEntropyLint(ctx context.Context, dir string) []EntropyIssue {
	if _, err := exec.LookPath("golangci-lint"); err != nil {
		return nil
	}

	cmd := exec.CommandContext(ctx, "golangci-lint", "run",
		"--out-format=json",
		"--timeout=120s",
		"--enable=errcheck,govet,staticcheck,unused,ineffassign,misspell",
		"./...")
	cmd.Dir = dir
	out, _ := cmd.CombinedOutput()

	return parseLintIssues(out, "golangci-lint")
}

func runJSEntropyLint(ctx context.Context, dir string) []EntropyIssue {
	if _, err := exec.LookPath("npx"); err != nil {
		return nil
	}

	cmd := exec.CommandContext(ctx, "npx", "eslint", "--format=json", ".")
	cmd.Dir = dir
	out, _ := cmd.CombinedOutput()

	return parseESLintIssues(out)
}

func runPythonEntropyLint(ctx context.Context, dir string) []EntropyIssue {
	if _, err := exec.LookPath("ruff"); err != nil {
		return nil
	}

	cmd := exec.CommandContext(ctx, "ruff", "check", "--output-format=json", ".")
	cmd.Dir = dir
	out, _ := cmd.CombinedOutput()

	return parseRuffIssues(out)
}

func parseLintIssues(data []byte, linter string) []EntropyIssue {
	var result struct {
		Issues []struct {
			FromLinter string `json:"FromLinter"`
			Text       string `json:"Text"`
			Pos        struct {
				Filename string `json:"Filename"`
				Line     int    `json:"Line"`
			} `json:"Pos"`
			Severity string `json:"Severity"`
		} `json:"Issues"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return nil
	}

	var issues []EntropyIssue
	for _, i := range result.Issues {
		category := categorizeRule(i.FromLinter)
		severity := "warning"
		if i.Severity == "error" {
			severity = "error"
		}
		issues = append(issues, EntropyIssue{
			File:     i.Pos.Filename,
			Line:     i.Pos.Line,
			Rule:     i.FromLinter,
			Message:  i.Text,
			Severity: severity,
			Category: category,
		})
	}
	return issues
}

func parseESLintIssues(data []byte) []EntropyIssue {
	var results []struct {
		FilePath string `json:"filePath"`
		Messages []struct {
			RuleId   string `json:"ruleId"`
			Message  string `json:"message"`
			Line     int    `json:"line"`
			Severity int    `json:"severity"`
		} `json:"messages"`
	}
	if err := json.Unmarshal(data, &results); err != nil {
		return nil
	}

	var issues []EntropyIssue
	for _, r := range results {
		for _, m := range r.Messages {
			severity := "warning"
			if m.Severity == 2 {
				severity = "error"
			}
			issues = append(issues, EntropyIssue{
				File:     filepath.Base(r.FilePath),
				Line:     m.Line,
				Rule:     m.RuleId,
				Message:  m.Message,
				Severity: severity,
				Category: categorizeRule(m.RuleId),
			})
		}
	}
	return issues
}

func parseRuffIssues(data []byte) []EntropyIssue {
	var results []struct {
		Code     string `json:"code"`
		Message  string `json:"message"`
		Filename string `json:"filename"`
		Location struct {
			Row int `json:"row"`
		} `json:"location"`
	}
	if err := json.Unmarshal(data, &results); err != nil {
		return nil
	}

	var issues []EntropyIssue
	for _, r := range results {
		issues = append(issues, EntropyIssue{
			File:     filepath.Base(r.Filename),
			Line:     r.Location.Row,
			Rule:     r.Code,
			Message:  r.Message,
			Severity: "warning",
			Category: categorizeRule(r.Code),
		})
	}
	return issues
}

// categorizeRule maps linter rules to entropy categories.
func categorizeRule(rule string) string {
	rule = strings.ToLower(rule)
	switch {
	case strings.Contains(rule, "naming") || strings.Contains(rule, "name") ||
		strings.Contains(rule, "misspell") || strings.Contains(rule, "camel"):
		return "naming"
	case strings.Contains(rule, "unused") || strings.Contains(rule, "dead") ||
		strings.Contains(rule, "unreachable"):
		return "dead_code"
	case strings.Contains(rule, "err") || strings.Contains(rule, "error") ||
		strings.Contains(rule, "errcheck"):
		return "error_handling"
	case strings.Contains(rule, "complex") || strings.Contains(rule, "cyclo") ||
		strings.Contains(rule, "cognitive"):
		return "complexity"
	default:
		return "style"
	}
}

// RunEntropyAIScan uses AI to detect deeper code quality issues.
// This is a placeholder that returns empty results — the real implementation
// would call the AI worker for pattern analysis.
func (a *EntropyActivities) RunEntropyAIScan(ctx context.Context, input EntropyAIScanInput) (*EntropyAIScanOutput, error) {
	slog.Info("RunEntropyAIScan", "project_id", input.ProjectID, "language", input.Language, "files", len(input.Files))

	// TODO: Call AI worker for deep pattern analysis
	// For now, return empty results — lint scan covers the basics
	return &EntropyAIScanOutput{Issues: []EntropyIssue{}}, nil
}

// SaveEntropyResults persists scan results to the database.
func (a *EntropyActivities) SaveEntropyResults(ctx context.Context, input SaveEntropyInput) error {
	slog.Info("SaveEntropyResults",
		"project_id", input.ProjectID,
		"score", input.Score,
		"issues", len(input.Issues),
	)

	issuesJSON, err := json.Marshal(input.Issues)
	if err != nil {
		return fmt.Errorf("marshal issues: %w", err)
	}

	_, err = a.db.Exec(ctx,
		`INSERT INTO engine.entropy_scans (project_id, tenant_id, score, issues, scanned_at, issue_count)
		 VALUES ($1, $2, $3, $4, $5, $6)`,
		input.ProjectID, input.TenantID, input.Score, issuesJSON,
		time.Now(), len(input.Issues),
	)
	if err != nil {
		return fmt.Errorf("insert entropy scan: %w", err)
	}

	return nil
}

// CreateAutoFixPR creates a branch with auto-fixes and opens a PR.
// Placeholder — real implementation would use AI worker + GitHub adapter.
func (a *EntropyActivities) CreateAutoFixPR(ctx context.Context, input AutoFixInput) (*AutoFixOutput, error) {
	slog.Info("CreateAutoFixPR",
		"project_id", input.ProjectID,
		"fixable_issues", countFixable(input.Issues),
	)

	// TODO: Implement auto-fix via AI worker + GitHub PR creation
	return &AutoFixOutput{PRURL: ""}, nil
}

func countFixable(issues []EntropyIssue) int {
	count := 0
	for _, i := range issues {
		if i.AutoFixable {
			count++
		}
	}
	return count
}
