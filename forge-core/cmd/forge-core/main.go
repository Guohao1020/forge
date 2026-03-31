package main

import (
	"context"
	"log/slog"
	"os"

	"github.com/shulex/forge/forge-core/internal/config"
	"github.com/shulex/forge/forge-core/internal/module/auth"
	"github.com/shulex/forge/forge-core/internal/module/project"
	"github.com/shulex/forge/forge-core/internal/pkg/database"
	forgeRedis "github.com/shulex/forge/forge-core/internal/pkg/redis"
	"github.com/shulex/forge/forge-core/internal/router"
)

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo})))

	cfg := config.Load()
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
	authService := auth.NewService(authRepo, cfg.JWTSecret, cfg.JWTExpireHours)
	authHandler := auth.NewHandler(authService)

	// Project module
	projectRepo := project.NewRepository(db)
	projectService := project.NewService(projectRepo)
	projectHandler := project.NewHandler(projectService)

	r := router.Setup(&router.Deps{
		AuthHandler:    authHandler,
		AuthService:    authService,
		ProjectHandler: projectHandler,
	})

	slog.Info("forge-core starting", "port", cfg.ServerPort)
	if err := r.Run(":" + cfg.ServerPort); err != nil {
		slog.Error("server failed", "error", err)
		os.Exit(1)
	}
}
