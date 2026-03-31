package main

import (
	"context"
	"log/slog"
	"os"

	"github.com/shulex/forge/forge-core/internal/config"
	"github.com/shulex/forge/forge-core/internal/module/auth"
	"github.com/shulex/forge/forge-core/internal/module/project"
	"github.com/shulex/forge/forge-core/internal/module/task"
	forgetemporal "github.com/shulex/forge/forge-core/internal/temporal"
	"github.com/shulex/forge/forge-core/internal/pkg/database"
	forgeRedis "github.com/shulex/forge/forge-core/internal/pkg/redis"
	"github.com/shulex/forge/forge-core/internal/router"
)

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo})))

	cfg := config.Load()
	cfg.Validate()
	ctx := context.Background()

	db, err := database.NewPool(ctx, cfg.DatabaseURL)
	if err != nil {
		slog.Error("failed to connect database", "error", err)
		os.Exit(1)
	}
	defer db.Close()

	rdb, err := forgeRedis.NewClient(ctx, cfg.RedisAddr, cfg.RedisPassword)
	if err != nil {
		slog.Error("failed to connect redis", "error", err)
		os.Exit(1)
	}
	defer rdb.Close()

	if err := database.RunMigrations(ctx, db, "migrations"); err != nil {
		slog.Error("failed to run migrations", "error", err)
		os.Exit(1)
	}

	// Auth module
	authRepo := auth.NewRepository(db)
	authService := auth.NewService(authRepo, cfg.JWTSecret, cfg.JWTExpireHours,
		cfg.GitHubClientID, cfg.GitHubClientSecret, cfg.GitHubRedirectURI, cfg.EncryptionKey)
	authHandler := auth.NewHandler(authService)

	// Project module
	projectRepo := project.NewRepository(db)
	projectService := project.NewService(projectRepo)
	projectHandler := project.NewHandler(projectService)

	// Temporal (optional — gracefully skip if unavailable)
	var workflowStarter task.WorkflowStarter
	temporalClient, err := forgetemporal.NewClient(ctx, cfg.TemporalAddress)
	if err != nil {
		slog.Warn("temporal not available, tasks will stay SUBMITTED", "error", err)
	} else {
		defer temporalClient.Close()
		workflowStarter = temporalClient

		_, err := forgetemporal.StartWorker(temporalClient.Inner(), db)
		if err != nil {
			slog.Error("failed to start temporal worker", "error", err)
		}
	}

	// Task module
	sseHub := task.NewSSEHub()
	taskRepo := task.NewRepository(db)
	taskService := task.NewService(taskRepo, workflowStarter)
	taskHandler := task.NewHandler(taskService)
	taskSSE := task.NewSSEHandler(sseHub)

	r := router.Setup(&router.Deps{
		AuthHandler:    authHandler,
		AuthService:    authService,
		ProjectHandler: projectHandler,
		TaskHandler:    taskHandler,
		TaskSSE:        taskSSE,
	})

	slog.Info("forge-core starting", "port", cfg.ServerPort)
	if err := r.Run(":" + cfg.ServerPort); err != nil {
		slog.Error("server failed", "error", err)
		os.Exit(1)
	}
}
