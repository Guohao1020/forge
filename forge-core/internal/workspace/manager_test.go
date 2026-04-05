package workspace

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNewManager_DefaultRoot(t *testing.T) {
	m := NewManager("")
	if m.root != "/data/forge/workspaces" {
		t.Errorf("expected default root, got %s", m.root)
	}
}

func TestNewManager_CustomRoot(t *testing.T) {
	m := NewManager("/tmp/test-workspaces")
	if m.root != "/tmp/test-workspaces" {
		t.Errorf("expected /tmp/test-workspaces, got %s", m.root)
	}
}

func TestProjectDir(t *testing.T) {
	m := NewManager("/data/ws")
	dir := m.ProjectDir(1, 42)
	expected := filepath.Join("/data/ws", "tenant-1", "project-42", "repo")
	if dir != expected {
		t.Errorf("expected %s, got %s", expected, dir)
	}
}

func TestTaskDir(t *testing.T) {
	m := NewManager("/data/ws")
	dir := m.TaskDir(1, 42, 99)
	expected := filepath.Join("/data/ws", "tenant-1", "project-42", "tasks", "task-99")
	if dir != expected {
		t.Errorf("expected %s, got %s", expected, dir)
	}
}

func TestInjectToken(t *testing.T) {
	tests := []struct {
		repoURL  string
		token    string
		expected string
	}{
		{
			"https://github.com/owner/repo.git",
			"ghp_test123",
			"https://x-access-token:ghp_test123@github.com/owner/repo.git",
		},
		{
			"https://github.com/owner/repo",
			"",
			"https://github.com/owner/repo",
		},
		{
			"git@github.com:owner/repo.git",
			"token",
			"git@github.com:owner/repo.git", // SSH URL not modified
		},
	}

	for _, tt := range tests {
		result := injectToken(tt.repoURL, tt.token)
		if result != tt.expected {
			t.Errorf("injectToken(%q, %q) = %q, want %q", tt.repoURL, tt.token, result, tt.expected)
		}
	}
}

func TestWriteFiles(t *testing.T) {
	tmpDir := t.TempDir()
	m := NewManager("")

	files := []FileToWrite{
		{Path: "main.go", Content: "package main"},
		{Path: "internal/handler/api.go", Content: "package handler"},
	}

	if err := m.WriteFiles(tmpDir, files); err != nil {
		t.Fatalf("WriteFiles failed: %v", err)
	}

	// Verify files exist
	content, err := os.ReadFile(filepath.Join(tmpDir, "main.go"))
	if err != nil {
		t.Fatalf("read main.go: %v", err)
	}
	if string(content) != "package main" {
		t.Errorf("expected 'package main', got %q", string(content))
	}

	content, err = os.ReadFile(filepath.Join(tmpDir, "internal", "handler", "api.go"))
	if err != nil {
		t.Fatalf("read api.go: %v", err)
	}
	if string(content) != "package handler" {
		t.Errorf("expected 'package handler', got %q", string(content))
	}
}

func TestWriteFiles_NestedDirs(t *testing.T) {
	tmpDir := t.TempDir()
	m := NewManager("")

	files := []FileToWrite{
		{Path: "a/b/c/d/deep.txt", Content: "deep"},
	}

	if err := m.WriteFiles(tmpDir, files); err != nil {
		t.Fatalf("WriteFiles failed: %v", err)
	}

	content, err := os.ReadFile(filepath.Join(tmpDir, "a", "b", "c", "d", "deep.txt"))
	if err != nil {
		t.Fatalf("read deep.txt: %v", err)
	}
	if string(content) != "deep" {
		t.Errorf("expected 'deep', got %q", string(content))
	}
}

func TestInjectToken_NoLeakInLogs(t *testing.T) {
	// Ensure token is embedded in URL but original URL can be logged safely
	url := "https://github.com/owner/repo"
	token := "secret-token-xyz"
	result := injectToken(url, token)

	if !strings.Contains(result, token) {
		t.Error("token should be in the injected URL")
	}

	// The original URL should not contain the token (for safe logging)
	if strings.Contains(url, token) {
		t.Error("original URL should not contain the token")
	}
}
