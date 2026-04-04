package main

import (
	"context"
	"log/slog"
	"os"

	"github.com/shulex/forge/forge-core/internal/config"
	"github.com/shulex/forge/forge-core/internal/k8s"
	"go.temporal.io/sdk/client"

	"github.com/shulex/forge/forge-core/internal/module/artifact"
	"github.com/shulex/forge/forge-core/internal/module/cost"
	"github.com/shulex/forge/forge-core/internal/module/entropy"
	"github.com/shulex/forge/forge-core/internal/module/auth"
	"github.com/shulex/forge/forge-core/internal/module/conversation"
	"github.com/shulex/forge/forge-core/internal/module/pipeline"
	"github.com/shulex/forge/forge-core/internal/module/preview"
	"github.com/shulex/forge/forge-core/internal/module/profile"
	"github.com/shulex/forge/forge-core/internal/module/project"
	"github.com/shulex/forge/forge-core/internal/module/specs"
	"github.com/shulex/forge/forge-core/internal/module/task"
	"github.com/shulex/forge/forge-core/internal/module/testresult"
	"github.com/shulex/forge/forge-core/internal/module/version"
	forgetemporal "github.com/shulex/forge/forge-core/internal/temporal"
	"github.com/shulex/forge/forge-core/internal/temporal/activity"
	"github.com/shulex/forge/forge-core/internal/workspace"
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

	// K8s client (optional — gracefully skip if unavailable)
	var k8sClient *k8s.Client
	if cfg.KubeconfigPath != "" {
		k8sClient, err = k8s.NewClient(cfg.KubeconfigPath)
		if err != nil {
			slog.Warn("k8s not available, jobs will use mock mode", "error", err)
		}
	}
	// k8sClient may be nil — activities fall back to mock mode

	// Auth module
	authRepo := auth.NewRepository(db)
	authService := auth.NewService(authRepo, cfg.JWTSecret, cfg.JWTExpireHours,
		cfg.GitHubClientID, cfg.GitHubClientSecret, cfg.GitHubRedirectURI, cfg.EncryptionKey)
	authHandler := auth.NewHandler(authService)

	// Workspace manager (local git clones + per-task worktrees)
	workspaceMgr := workspace.NewManager(cfg.WorkspaceRoot)

	// Project module
	projectRepo := project.NewRepository(db)
	projectService := project.NewService(projectRepo, authService, workspaceMgr)
	projectHandler := project.NewHandler(projectService)

	// Task module — SSEHub must be created before Temporal worker
	sseHub := task.NewSSEHub()

	// Temporal (optional — gracefully skip if unavailable)
	var workflowStarter task.WorkflowStarter
	temporalClient, err := forgetemporal.NewClient(ctx, cfg.TemporalAddress)
	if err != nil {
		slog.Warn("temporal not available, tasks will stay SUBMITTED", "error", err)
	} else {
		defer temporalClient.Close()
		workflowStarter = temporalClient

		taskRepoForWorker := task.NewRepository(db)
		_, err := forgetemporal.StartWorker(temporalClient.Inner(), db, sseHub,
			authService, activity.NewProjectRepoAdapter(projectRepo), taskRepoForWorker, workspaceMgr, k8sClient)
		if err != nil {
			slog.Error("failed to start temporal worker", "error", err)
		}
	}
	taskRepo := task.NewRepository(db)
	taskService := task.NewService(taskRepo, workflowStarter)
	taskSSE := task.NewSSEHandler(sseHub, rdb)

	// Conversation module (must be before taskHandler — provides ConversationCreator)
	convRepo := conversation.NewRepository(db)
	var temporalInner client.Client
	if temporalClient != nil {
		temporalInner = temporalClient.Inner()
	}
	convService := conversation.NewService(convRepo, taskRepo, temporalInner)
	convHandler := conversation.NewHandler(convService)

	// Wire Temporal client to project service for auto-profile-scan on import
	if temporalInner != nil {
		projectService.SetTemporalClient(temporalInner)
	}

	taskHandler := task.NewHandler(taskService, convService)

	// Pipeline module
	pipelineRepo := pipeline.NewRepository(db)
	pipelineSvc := pipeline.NewService(pipelineRepo, k8sClient)
	pipelineHandler := pipeline.NewHandler(pipelineSvc)

	// Profile module
	profileRepo := profile.NewRepository(db)
	profileSvc := profile.NewService(profileRepo, temporalInner)
	profileHandler := profile.NewHandler(profileSvc)

	// Test Results module
	testResultRepo := testresult.NewRepository(db)
	testResultSvc := testresult.NewService(testResultRepo)
	testResultHandler := testresult.NewHandler(testResultSvc)

	// Preview module
	previewRepo := preview.NewRepository(db)
	previewSvc := preview.NewService(previewRepo, k8sClient, cfg.K8sNodeIP)
	previewHandler := preview.NewHandler(previewSvc)

	// Artifact module
	artifactRepo := artifact.NewRepository(db)
	artifactSvc := artifact.NewService(artifactRepo)
	artifactHandler := artifact.NewHandler(artifactSvc)

	// Version module
	versionRepo := version.NewRepository(db)
	versionSvc := version.NewService(versionRepo)
	versionHandler := version.NewHandler(versionSvc)

	// Cost module
	costSvc := cost.NewService(db)
	costHandler := cost.NewHandler(costSvc)

	// Entropy module
	entropySvc := entropy.NewService(db)
	if temporalInner != nil {
		entropySvc.SetTemporalClient(temporalInner)
	}
	entropyHandler := entropy.NewHandler(entropySvc)

	// Specs module
	specsRepo := specs.NewRepository(db)
	specsService := specs.NewService(specsRepo, rdb)
	specsHandler := specs.NewHandler(specsService)

	r := router.Setup(&router.Deps{
		AuthHandler:         authHandler,
		AuthService:         authService,
		ProjectHandler:      projectHandler,
		TaskHandler:         taskHandler,
		TaskSSE:             taskSSE,
		ConversationHandler: convHandler,
		SpecsHandler:        specsHandler,
		PipelineHandler:     pipelineHandler,
		PreviewHandler:      previewHandler,
		ProfileHandler:      profileHandler,
		TestResultHandler:   testResultHandler,
		ArtifactHandler:     artifactHandler,
		VersionHandler:      versionHandler,
		CostHandler:         costHandler,
		EntropyHandler:      entropyHandler,
	})

	slog.Info("forge-core starting", "port", cfg.ServerPort)
	if err := r.Run(":" + cfg.ServerPort); err != nil {
		slog.Error("server failed", "error", err)
		os.Exit(1)
	}
}
