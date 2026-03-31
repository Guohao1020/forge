package router

import (
	"github.com/gin-gonic/gin"
	"github.com/shulex/forge/forge-core/internal/middleware"
	"github.com/shulex/forge/forge-core/internal/module/auth"
	"github.com/shulex/forge/forge-core/internal/module/project"
	"github.com/shulex/forge/forge-core/internal/module/task"
)

type Deps struct {
	AuthHandler    *auth.Handler
	AuthService    *auth.Service
	ProjectHandler *project.Handler
	TaskHandler    *task.Handler
	TaskSSE        *task.SSEHandler
}

func Setup(deps *Deps) *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(middleware.RequestID())
	r.Use(middleware.CORS())

	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
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

			// Tasks
			if deps.TaskHandler != nil {
				protected.POST("/projects/:id/tasks", deps.TaskHandler.CreateTask)
				protected.GET("/projects/:id/tasks", deps.TaskHandler.ListTasks)
				protected.GET("/projects/:id/tasks/:taskId", deps.TaskHandler.GetTask)
			}

			// SSE
			if deps.TaskSSE != nil {
				protected.GET("/stream/tasks/:taskId", deps.TaskSSE.Stream)
			}
		}
	}

	return r
}
