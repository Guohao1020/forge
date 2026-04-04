package router

import (
	"github.com/gin-gonic/gin"
	"github.com/shulex/forge/forge-core/internal/middleware"
	"github.com/shulex/forge/forge-core/internal/module/artifact"
	"github.com/shulex/forge/forge-core/internal/module/cost"
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

type Deps struct {
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
}

func Setup(deps *Deps) *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(middleware.RequestID())
	r.Use(middleware.CORS())
	r.Use(middleware.MetricsMiddleware())

	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
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
		{
			protected.POST("/auth/logout", deps.AuthHandler.Logout)
			protected.GET("/auth/me", deps.AuthHandler.Me)

			// GitHub OAuth
			protected.GET("/auth/github/authorize", deps.AuthHandler.GitHubAuthorize)
			protected.GET("/auth/github/callback", deps.AuthHandler.GitHubCallback)
			protected.GET("/auth/github/status", deps.AuthHandler.GitHubStatus)
			protected.DELETE("/auth/github/disconnect", deps.AuthHandler.GitHubDisconnect)

			// GitHub repos
			protected.GET("/github/repos", deps.AuthHandler.ListGitHubRepos)

			// Projects
			protected.POST("/projects/import", deps.ProjectHandler.Import)
			protected.POST("/projects", deps.ProjectHandler.Create)
			protected.GET("/projects", deps.ProjectHandler.List)
			protected.GET("/projects/:id", deps.ProjectHandler.GetByID)
			protected.PUT("/projects/:id", deps.ProjectHandler.Update)
			protected.DELETE("/projects/:id", deps.ProjectHandler.Archive)
			protected.POST("/projects/:id/star", deps.ProjectHandler.Star)
			protected.DELETE("/projects/:id/star", deps.ProjectHandler.Unstar)
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

			// Cost Control
			if deps.CostHandler != nil {
				protected.GET("/admin/costs", deps.CostHandler.GetMonthlyCosts)
				protected.GET("/admin/budget", deps.CostHandler.GetBudgetStatus)
				protected.GET("/projects/:id/costs", deps.CostHandler.GetProjectCosts)
			}
		}
	}

	return r
}
