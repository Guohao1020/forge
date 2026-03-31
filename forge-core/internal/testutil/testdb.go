package testutil

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shulex/forge/forge-core/internal/pkg/database"
)

// TestDB returns a pgxpool connected to the dev database and runs migrations.
// It cleans up test data after the test completes.
func TestDB(t *testing.T) *pgxpool.Pool {
	t.Helper()

	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		dbURL = "postgres://forge:forge_dev_2026@localhost:5432/forge_main?sslmode=disable"
	}

	ctx := context.Background()
	pool, err := database.NewPool(ctx, dbURL)
	if err != nil {
		t.Fatalf("connect to test database: %v", err)
	}

	// Run migrations from project root
	migrationsDir := findMigrationsDir()
	if err := database.RunMigrations(ctx, pool, migrationsDir); err != nil {
		pool.Close()
		t.Fatalf("run migrations: %v", err)
	}

	t.Cleanup(func() {
		cleanTestData(ctx, pool)
		pool.Close()
	})

	return pool
}

func findMigrationsDir() string {
	// Try relative paths from different test locations
	candidates := []string{
		"../../migrations",
		"../../../migrations",
		"../../../../migrations",
		"migrations",
	}
	for _, dir := range candidates {
		if _, err := os.Stat(dir); err == nil {
			return dir
		}
	}
	return "migrations"
}

func cleanTestData(ctx context.Context, pool *pgxpool.Pool) {
	// Clean in reverse dependency order, skip seed data
	queries := []string{
		"DELETE FROM engine.project_stars",
		"DELETE FROM engine.projects",
		// Don't delete auth seed data (tenants, users, roles)
		fmt.Sprintf("DELETE FROM auth.users WHERE username != 'admin'"),
	}
	for _, q := range queries {
		pool.Exec(ctx, q)
	}
}
