package workspace

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNewManager_DefaultRoot(t *testing.T) {
	m := NewManager(Config{})
	if m.root != "/data/forge/workspaces" {
		t.Errorf("expected default root, got %s", m.root)
	}
}

func TestNewManager_CustomRoot(t *testing.T) {
	m := NewManager(Config{Root: "/tmp/test-workspaces"})
	if m.root != "/tmp/test-workspaces" {
		t.Errorf("expected /tmp/test-workspaces, got %s", m.root)
	}
}

func TestProjectDir(t *testing.T) {
	m := NewManager(Config{Root: "/data/ws"})
	dir := m.ProjectDir(1, 42)
	expected := filepath.Join("/data/ws", "tenant-1", "project-42", "repo")
	if dir != expected {
		t.Errorf("expected %s, got %s", expected, dir)
	}
}

func TestTaskDir(t *testing.T) {
	m := NewManager(Config{Root: "/data/ws"})
	dir := m.TaskDir(1, 42, 99)
	expected := filepath.Join("/data/ws", "tenant-1", "project-42", "tasks", "task-99")
	if dir != expected {
		t.Errorf("expected %s, got %s", expected, dir)
	}
}

func TestWriteFiles(t *testing.T) {
	tmpDir := t.TempDir()
	m := NewManager(Config{})

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
	m := NewManager(Config{})

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
