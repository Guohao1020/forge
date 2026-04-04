package activity

import (
	"testing"
)

func TestCategorizeRule(t *testing.T) {
	tests := []struct {
		rule string
		want string
	}{
		{"errcheck", "error_handling"},
		{"Error handling missing", "error_handling"},
		{"unused", "dead_code"},
		{"unreachable code", "dead_code"},
		{"misspell", "naming"},
		{"camelCase", "naming"},
		{"naming convention", "naming"},
		{"cyclomatic complexity", "complexity"},
		{"cognitive complexity", "complexity"},
		{"golint", "style"},
		{"unknown-rule", "style"},
	}

	for _, tt := range tests {
		t.Run(tt.rule, func(t *testing.T) {
			got := categorizeRule(tt.rule)
			if got != tt.want {
				t.Errorf("categorizeRule(%q) = %q, want %q", tt.rule, got, tt.want)
			}
		})
	}
}

func TestScanLocalFiles_NonExistentDir(t *testing.T) {
	files, err := scanLocalFiles("/nonexistent/path/12345", "go")
	if err != nil {
		// Walk returns an error for non-existent dirs on some OSes, that's fine
		return
	}
	if len(files) != 0 {
		t.Errorf("expected 0 files for non-existent dir, got %d", len(files))
	}
}

func TestCountFixable(t *testing.T) {
	issues := []EntropyIssue{
		{AutoFixable: true},
		{AutoFixable: false},
		{AutoFixable: true},
	}
	if got := countFixable(issues); got != 2 {
		t.Errorf("countFixable = %d, want 2", got)
	}
}

func TestParseLintIssues_InvalidJSON(t *testing.T) {
	result := parseLintIssues([]byte("not json"), "test")
	if result != nil {
		t.Errorf("expected nil for invalid JSON, got %v", result)
	}
}

func TestParseESLintIssues_InvalidJSON(t *testing.T) {
	result := parseESLintIssues([]byte("not json"))
	if result != nil {
		t.Errorf("expected nil for invalid JSON, got %v", result)
	}
}

func TestParseRuffIssues_InvalidJSON(t *testing.T) {
	result := parseRuffIssues([]byte("not json"))
	if result != nil {
		t.Errorf("expected nil for invalid JSON, got %v", result)
	}
}

func TestParseLintIssues_ValidJSON(t *testing.T) {
	data := `{"Issues":[{"FromLinter":"errcheck","Text":"unchecked error","Pos":{"Filename":"main.go","Line":10},"Severity":"error"}]}`
	result := parseLintIssues([]byte(data), "golangci-lint")
	if len(result) != 1 {
		t.Fatalf("expected 1 issue, got %d", len(result))
	}
	if result[0].Rule != "errcheck" {
		t.Errorf("expected rule errcheck, got %s", result[0].Rule)
	}
	if result[0].Severity != "error" {
		t.Errorf("expected severity error, got %s", result[0].Severity)
	}
	if result[0].Category != "error_handling" {
		t.Errorf("expected category error_handling, got %s", result[0].Category)
	}
}
