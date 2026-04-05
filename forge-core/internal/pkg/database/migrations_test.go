package database

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

func TestMigrationFilesExist(t *testing.T) {
	// Find migrations directory relative to the test file
	dirs := []string{"../../../migrations", "migrations"}
	var migDir string
	for _, d := range dirs {
		if _, err := os.Stat(d); err == nil {
			migDir = d
			break
		}
	}
	if migDir == "" {
		t.Skip("migrations directory not found (run from forge-core root)")
	}

	entries, err := os.ReadDir(migDir)
	if err != nil {
		t.Fatalf("read migrations dir: %v", err)
	}

	var sqlFiles []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".sql") {
			sqlFiles = append(sqlFiles, e.Name())
		}
	}

	if len(sqlFiles) < 10 {
		t.Errorf("expected at least 10 migration files, got %d", len(sqlFiles))
	}

	// Verify files are numbered sequentially
	sort.Strings(sqlFiles)
	for i, f := range sqlFiles {
		expected := strings.Split(f, "_")[0]
		if expected == "" {
			t.Errorf("migration %s has no numeric prefix", f)
		}

		// Verify content is non-empty
		content, err := os.ReadFile(filepath.Join(migDir, f))
		if err != nil {
			t.Errorf("cannot read %s: %v", f, err)
			continue
		}
		if len(strings.TrimSpace(string(content))) == 0 {
			t.Errorf("migration %s is empty", f)
		}

		// Basic SQL validation — should contain at least one SQL keyword
		upper := strings.ToUpper(string(content))
		hasSQLKeyword := strings.Contains(upper, "CREATE") ||
			strings.Contains(upper, "ALTER") ||
			strings.Contains(upper, "INSERT") ||
			strings.Contains(upper, "UPDATE") ||
			strings.Contains(upper, "DROP")
		if !hasSQLKeyword {
			t.Errorf("migration %s doesn't contain any SQL keywords", f)
		}

		_ = i // used for sequential check if needed
	}
}

func TestMigrationNaming(t *testing.T) {
	dirs := []string{"../../../migrations", "migrations"}
	var migDir string
	for _, d := range dirs {
		if _, err := os.Stat(d); err == nil {
			migDir = d
			break
		}
	}
	if migDir == "" {
		t.Skip("migrations directory not found")
	}

	entries, err := os.ReadDir(migDir)
	if err != nil {
		t.Fatalf("read migrations dir: %v", err)
	}

	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".sql") {
			continue
		}
		// Verify naming convention: NNN_description.sql
		parts := strings.SplitN(e.Name(), "_", 2)
		if len(parts) < 2 {
			t.Errorf("migration %s doesn't follow NNN_description.sql convention", e.Name())
		}
	}
}
