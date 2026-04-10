package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/joho/godotenv"
	"github.com/shulex/forge/forge-core/internal/config"
	"github.com/shulex/forge/forge-core/internal/k8s"
	"go.temporal.io/sdk/client"

	"github.com/shulex/forge/forge-core/internal/module/agent"
	"github.com/shulex/forge/forge-core/internal/module/artifact"
	"github.com/shulex/forge/forge-core/internal/module/cost"
	"github.com/shulex/forge/forge-core/internal/module/entropy"
	"github.com/shulex/forge/forge-core/internal/module/search"
	"github.com/shulex/forge/forge-core/internal/module/settings"
	"github.com/shulex/forge/forge-core/internal/module/webhook"
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

	// Load .env from project root (forge-core/../.env) if present
	for _, p := range []string{".env", "../.env"} {
		if err := godotenv.Load(p); err == nil {
			slog.Info("loaded env file", "path", p)
			break
		}
	}

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

	// Generate service token for ai-worker (100-year expiry, auto-updates .env)
	if serviceToken, err := authService.GenerateServiceToken("ai-worker"); err == nil {
		envPath := "../ai-worker/.env"
		if data, readErr := os.ReadFile(envPath); readErr == nil {
			lines := strings.Split(string(data), "\n")
			found := false
			for i, line := range lines {
				if strings.HasPrefix(line, "FORGE_API_TOKEN=") {
					lines[i] = "FORGE_API_TOKEN=" + serviceToken
					found = true
					break
				}
			}
			if !found {
				lines = append(lines, "FORGE_API_TOKEN="+serviceToken)
			}
			if writeErr := os.WriteFile(envPath, []byte(strings.Join(lines, "\n")), 0644); writeErr == nil {
				slog.Info("service token written to ai-worker/.env")
			} else {
				slog.Warn("failed to write service token to ai-worker/.env", "error", writeErr)
			}
		} else {
			slog.Warn("failed to read ai-worker/.env for service token", "error", readErr)
		}
	} else {
		slog.Warn("failed to generate service token", "error", err)
	}

	// Workspace manager (local git clones + per-task worktrees)
	workspaceMgr := workspace.NewManager(workspace.Config{Root: cfg.WorkspaceRoot})

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
		// Build CodeFetcher from project service for branch-aware file access.
		// Uses system admin userID=1 to access GitHub tokens for API calls.
		const systemUserID int64 = 1
		codeFetcher := &activity.CodeFetcher{
			GetTree: func(ctx context.Context, projectID, tenantID int64, ref string) ([]string, error) {
				return projectService.GetCodeTree(ctx, projectID, tenantID, systemUserID, ref)
			},
			GetFile: func(ctx context.Context, projectID, tenantID int64, path, ref string) (string, error) {
				return projectService.GetCodeFile(ctx, projectID, tenantID, systemUserID, path, ref)
			},
		}
		_, err := forgetemporal.StartWorker(temporalClient.Inner(), db, sseHub,
			authService, activity.NewProjectRepoAdapter(projectRepo), taskRepoForWorker, workspaceMgr, k8sClient, codeFetcher, cfg.ACRRegistry)
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
	convService := conversation.NewService(convRepo, taskRepo, temporalInner, sseHub)
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

	// Wire ScanTrigger to version service for version-scoped scanning
	versionSvc.SetScanTrigger(&scanTriggerAdapter{
		profileSvc: profileSvc,
		entropySvc: entropySvc,
	})

	// Search module
	searchHandler := search.NewHandler(db)

	// Webhook module
	webhookSvc := webhook.NewService(db)
	webhookHandler := webhook.NewHandler(webhookSvc)

	// Settings module
	settingsSvc := settings.NewService(db)
	settingsHandler := settings.NewHandler(settingsSvc)

	// Specs module
	specsRepo := specs.NewRepository(db)
	specsService := specs.NewService(specsRepo, rdb)
	specsHandler := specs.NewHandler(specsService)

	// Agent Terminal handler (OpenHarness). The Repository backs the
	// dual-storage path (PG session/message log); it's optional, so the
	// handler still serves the core Chat/Stream endpoints if db is nil.
	agentSvc := agent.NewService(cfg.AIWorkerURL, workspaceMgr)
	agentRepo := agent.NewRepository(db)
	agentHandler := agent.NewHandler(agentSvc, rdb, agentRepo)

	r := router.Setup(&router.Deps{
		DB:                  db,
		RDB:                 rdb,
		TemporalClient:      temporalInner,
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
		SearchHandler:       searchHandler,
		SettingsHandler:     settingsHandler,
		WebhookHandler:      webhookHandler,
		AgentHandler:        agentHandler,
	})

	// Graceful shutdown
	srv := &http.Server{
		Addr:         ":" + cfg.ServerPort,
		Handler:      r,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 60 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	go func() {
		slog.Info("forge-core starting", "port", cfg.ServerPort)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("server failed", "error", err)
			os.Exit(1)
		}
	}()

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	sig := <-quit
	slog.Info("shutting down", "signal", sig.String())

	// Give in-flight requests 10 seconds to complete
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		slog.Error("forced shutdown", "error", err)
	}

	slog.Info("forge-core stopped")
}

// scanTriggerAdapter implements version.ScanTrigger by delegating to profile and entropy services.
type scanTriggerAdapter struct {
	profileSvc *profile.Service
	entropySvc *entropy.Service
}

func (a *scanTriggerAdapter) TriggerProfileScan(ctx context.Context, projectID, userID int64, branches []string) error {
	_, err := a.profileSvc.TriggerScan(ctx, projectID, userID, nil, branches)
	return err
}

func (a *scanTriggerAdapter) TriggerEntropyScan(ctx context.Context, projectID, tenantID int64, branches []string) error {
	_, err := a.entropySvc.TriggerScan(ctx, projectID, tenantID, branches)
	return err
}
