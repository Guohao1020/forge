package router

import (
	"context"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	goredis "github.com/redis/go-redis/v9"
	"github.com/shulex/forge/forge-core/internal/middleware"
	"github.com/shulex/forge/forge-core/internal/module/artifact"
	"github.com/shulex/forge/forge-core/internal/module/cost"
	"github.com/shulex/forge/forge-core/internal/module/entropy"
	"github.com/shulex/forge/forge-core/internal/module/search"
	"github.com/shulex/forge/forge-core/internal/module/settings"
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
)

var routerStartTime = time.Now()

type Deps struct {
	DB  *pgxpool.Pool
	RDB *goredis.Client

	AuthHandler    *auth.Handler
	AuthService    *auth.Service
	ProjectHandler *project.Handler
	TaskHandler         *task.Handler
	TaskSSE             *task.SSEHandler
	ConversationHandler *conversation.Handler
	SpecsHandler        *specs.Handler
	PipelineHandler     *pipeline.Handler
	PreviewHandler      *preview.Handler
	ProfileHandler      *profile.Handler
	TestResultHandler   *testresult.Handler
	ArtifactHandler     *artifact.Handler
	VersionHandler      *version.Handler
	CostHandler         *cost.Handler
	EntropyHandler      *entropy.Handler
	SearchHandler       *search.Handler
	SettingsHandler     *settings.Handler
}

func Setup(deps *Deps) *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(middleware.RequestID())
	r.Use(middleware.CORS())
	r.Use(middleware.AccessLog())
	r.Use(middleware.Timeout(30 * time.Second))
	r.Use(middleware.MetricsMiddleware())

	r.GET("/health", func(c *gin.Context) {
		health := gin.H{"status": "ok"}

		// Check database
		if deps.DB != nil {
			ctx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Second)
			defer cancel()
			if err := deps.DB.Ping(ctx); err != nil {
				health["database"] = "down"
				health["status"] = "degraded"
			} else {
				health["database"] = "up"
			}
		}

		// Check Redis
		if deps.RDB != nil {
			ctx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Second)
			defer cancel()
			if err := deps.RDB.Ping(ctx).Err(); err != nil {
				health["redis"] = "down"
				health["status"] = "degraded"
			} else {
				health["redis"] = "up"
			}
		}

		health["uptime"] = time.Since(routerStartTime).Truncate(time.Second).String()

		status := 200
		if health["status"] == "degraded" {
			status = 503
		}
		c.JSON(status, health)
	})

	// Prometheus metrics endpoint (no auth required)
	r.GET("/metrics", func(c *gin.Context) {
		c.Data(200, "text/plain; charset=utf-8", []byte(middleware.PrometheusFormat()))
	})

	// JSON metrics endpoint (for admin dashboard)
	r.GET("/api/admin/metrics", func(c *gin.Context) {
		c.JSON(200, middleware.GetMetrics())
	})

	api := r.Group("/api")
	{
		// Public routes
		api.POST("/auth/login", deps.AuthHandler.Login)

		// Protected routes
		protected := api.Group("")
		protected.Use(middleware.JWTAuth(deps.AuthService))
		protected.Use(middleware.RateLimitMiddleware())
		{
			protected.POST("/auth/logout", deps.AuthHandler.Logout)
			protected.GET("/auth/me", deps.AuthHandler.Me)
			protected.PUT("/auth/password", deps.AuthHandler.ChangePassword)

			// Global search
			if deps.SearchHandler != nil {
				protected.GET("/search", deps.SearchHandler.Search)
			}

			// Recent activity feed
			protected.GET("/activity", deps.ProjectHandler.GetRecentActivity)

			// Platform settings
			if deps.SettingsHandler != nil {
				protected.GET("/settings", deps.SettingsHandler.List)
				protected.GET("/settings/:key", deps.SettingsHandler.Get)
				protected.PUT("/settings/:key", middleware.RequireRole(middleware.RolePlatformAdmin), deps.SettingsHandler.Set)
				protected.PUT("/settings", middleware.RequireRole(middleware.RolePlatformAdmin), deps.SettingsHandler.BulkSet)
			}

			// GitHub OAuth
			protected.GET("/auth/github/authorize", deps.AuthHandler.GitHubAuthorize)
			protected.GET("/auth/github/callback", deps.AuthHandler.GitHubCallback)
			protected.GET("/auth/github/status", deps.AuthHandler.GitHubStatus)
			protected.DELETE("/auth/github/disconnect", deps.AuthHandler.GitHubDisconnect)

			// GitHub repos
			protected.GET("/github/repos", deps.AuthHandler.ListGitHubRepos)

			// Projects (read: any auth user, write: DEVELOPER+, admin: PROJECT_ADMIN+)
			protected.POST("/projects/import", middleware.RequireRole(middleware.RoleDeveloper), deps.ProjectHandler.Import)
			protected.POST("/projects", middleware.RequireRole(middleware.RoleDeveloper), deps.ProjectHandler.Create)
			protected.GET("/projects", deps.ProjectHandler.List)
			protected.GET("/projects/:id", deps.ProjectHandler.GetByID)
			protected.PUT("/projects/:id", middleware.RequireRole(middleware.RoleProjectAdmin), deps.ProjectHandler.Update)
			protected.DELETE("/projects/:id", middleware.RequireRole(middleware.RoleProjectAdmin), deps.ProjectHandler.Archive)
			protected.POST("/projects/:id/star", deps.ProjectHandler.Star)
			protected.DELETE("/projects/:id/star", deps.ProjectHandler.Unstar)
			protected.GET("/projects/:id/stats", deps.ProjectHandler.GetStats)
			protected.POST("/projects/:id/sync", deps.ProjectHandler.SyncToRemote)
			protected.POST("/projects/:id/detect", deps.ProjectHandler.DetectTechStack)

			// Code browsing
			protected.GET("/projects/:id/code/tree", deps.ProjectHandler.GetCodeTree)
			protected.GET("/projects/:id/code/file", deps.ProjectHandler.GetCodeFile)
			protected.GET("/projects/:id/code/branches", deps.ProjectHandler.ListBranches)
			protected.GET("/projects/:id/code/prs", deps.ProjectHandler.ListPRs)
			protected.GET("/projects/:id/code/prs/:prNumber", deps.ProjectHandler.GetPRDetail)

			// Tasks
			if deps.TaskHandler != nil {
				protected.POST("/projects/:id/tasks", deps.TaskHandler.CreateTask)
				protected.GET("/projects/:id/tasks", deps.TaskHandler.ListTasks)
				protected.GET("/projects/:id/tasks/:taskId", deps.TaskHandler.GetTask)
				protected.GET("/projects/:id/tasks/:taskId/nodes", deps.TaskHandler.ListTaskNodes)
				protected.POST("/projects/:id/tasks/:taskId/cancel", deps.TaskHandler.CancelTask)
			}

			// Test Results
			if deps.TestResultHandler != nil {
				protected.GET("/projects/:id/tasks/:taskId/test-results", deps.TestResultHandler.ListTestResults)
				protected.POST("/projects/:id/tasks/:taskId/test-results", deps.TestResultHandler.CreateTestResult)
			}

			// Conversations
			if deps.ConversationHandler != nil {
				protected.POST("/projects/:id/tasks/:taskId/messages", deps.ConversationHandler.SendMessage)
				protected.GET("/projects/:id/tasks/:taskId/messages", deps.ConversationHandler.GetHistory)
				protected.POST("/projects/:id/tasks/:taskId/analyze", deps.ConversationHandler.TriggerAnalysis)
				protected.POST("/projects/:id/tasks/:taskId/confirm", deps.ConversationHandler.ConfirmPlan)
				protected.POST("/projects/:id/tasks/:taskId/approve-plan", deps.ConversationHandler.ApprovePlan)
			}

			// SSE
			if deps.TaskSSE != nil {
				protected.GET("/stream/tasks/:taskId", deps.TaskSSE.Stream)
			}

			// Preview Environments
			if deps.PreviewHandler != nil {
				protected.GET("/projects/:id/previews", deps.PreviewHandler.ListPreviews)
				protected.GET("/projects/:id/tasks/:taskId/preview", deps.PreviewHandler.GetPreviewByTask)
				protected.POST("/projects/:id/tasks/:taskId/preview", deps.PreviewHandler.CreatePreview)
				protected.DELETE("/projects/:id/previews/:previewId", deps.PreviewHandler.DestroyPreview)
			}

			// Pipeline / Environments + Deploy Records
			if deps.PipelineHandler != nil {
				protected.GET("/projects/:id/environments", deps.PipelineHandler.ListEnvironments)
				protected.GET("/projects/:id/environments/:envId", deps.PipelineHandler.GetEnvironment)
				protected.GET("/projects/:id/environments/:envId/deploys", deps.PipelineHandler.ListDeployRecords)
				protected.POST("/projects/:id/environments/:envId/deploy", deps.PipelineHandler.TriggerDeploy)
			}

			// Artifacts
			if deps.ArtifactHandler != nil {
				protected.GET("/projects/:id/artifacts", deps.ArtifactHandler.ListArtifacts)
				protected.GET("/projects/:id/artifacts/:artifactId", deps.ArtifactHandler.GetArtifact)
			}

			// Profile
			if deps.ProfileHandler != nil {
				protected.GET("/projects/:id/profiles", deps.ProfileHandler.ListProfiles)
				protected.GET("/projects/:id/profiles/:key", deps.ProfileHandler.GetProfile)
				protected.PUT("/projects/:id/profiles/:key", deps.ProfileHandler.SaveProfile)
				protected.POST("/projects/:id/profiles/scan", deps.ProfileHandler.TriggerScan)
			}

			// Version Management
			if deps.VersionHandler != nil {
				protected.POST("/projects/:id/versions", deps.VersionHandler.Create)
				protected.GET("/projects/:id/versions", deps.VersionHandler.List)
				protected.GET("/projects/:id/versions/:vid", deps.VersionHandler.Get)
				protected.PUT("/projects/:id/versions/:vid", deps.VersionHandler.Update)
				protected.POST("/projects/:id/versions/:vid/release", deps.VersionHandler.Release)
			}

			// Specs Center
			if deps.SpecsHandler != nil {
				specsGroup := protected.Group("/specs")
				{
					// Standards
					standards := specsGroup.Group("/standards")
					standards.GET("", deps.SpecsHandler.ListStandards)
					standards.GET("/:id", deps.SpecsHandler.GetStandard)
					standards.POST("", deps.SpecsHandler.CreateStandard)
					standards.PUT("/:id", deps.SpecsHandler.UpdateStandard)
					standards.DELETE("/:id", deps.SpecsHandler.DeleteStandard)

					// Prompt Templates
					prompts := specsGroup.Group("/prompts")
					prompts.GET("", deps.SpecsHandler.ListPromptTemplates)
					prompts.GET("/:id", deps.SpecsHandler.GetPromptTemplate)
					prompts.POST("", deps.SpecsHandler.CreatePromptTemplate)
					prompts.PUT("/:id", deps.SpecsHandler.UpdatePromptTemplate)
					prompts.DELETE("/:id", deps.SpecsHandler.DeletePromptTemplate)

					// Review Rules
					rules := specsGroup.Group("/rules")
					rules.GET("", deps.SpecsHandler.ListReviewRules)
					rules.GET("/:id", deps.SpecsHandler.GetReviewRule)
					rules.POST("", deps.SpecsHandler.CreateReviewRule)
					rules.PUT("/:id", deps.SpecsHandler.UpdateReviewRule)
					rules.DELETE("/:id", deps.SpecsHandler.ToggleReviewRule)

					// Scaffold Templates (read-only)
					scaffolds := specsGroup.Group("/scaffolds")
					scaffolds.GET("", deps.SpecsHandler.ListScaffoldTemplates)
					scaffolds.GET("/:id", deps.SpecsHandler.GetScaffoldTemplate)

					// Effective specs (resolved inheritance)
					specsGroup.GET("/effective/:projectId", deps.SpecsHandler.GetEffectiveSpecs)
				}
			}

			// User Management (PLATFORM_ADMIN only)
			protected.GET("/admin/users", middleware.RequireRole(middleware.RolePlatformAdmin), deps.AuthHandler.ListUsers)
			protected.POST("/admin/users", middleware.RequireRole(middleware.RolePlatformAdmin), deps.AuthHandler.CreateUser)
			protected.PUT("/admin/users/:userId/role", middleware.RequireRole(middleware.RolePlatformAdmin), deps.AuthHandler.UpdateUserRole)

			// Cost Control (admin-only for tenant costs, project-level for project members)
			if deps.CostHandler != nil {
				protected.GET("/admin/costs", middleware.RequireRole(middleware.RolePlatformAdmin), deps.CostHandler.GetMonthlyCosts)
				protected.GET("/admin/budget", middleware.RequireRole(middleware.RolePlatformAdmin), deps.CostHandler.GetBudgetStatus)
				protected.GET("/projects/:id/costs", middleware.RequireRole(middleware.RoleProjectAdmin), deps.CostHandler.GetProjectCosts)
			}

			// Entropy Management (code quality scans)
			if deps.EntropyHandler != nil {
				protected.GET("/projects/:id/entropy/latest", deps.EntropyHandler.GetLatestScan)
				protected.GET("/projects/:id/entropy/scans", deps.EntropyHandler.ListScans)
				protected.GET("/projects/:id/entropy/trends", deps.EntropyHandler.GetTrends)
				protected.GET("/projects/:id/entropy/config", deps.EntropyHandler.GetConfig)
				protected.PUT("/projects/:id/entropy/config", middleware.RequireRole(middleware.RoleProjectAdmin), deps.EntropyHandler.UpdateConfig)
				protected.POST("/projects/:id/entropy/scan", deps.EntropyHandler.TriggerScan)
			}
		}
	}

	return r
}
