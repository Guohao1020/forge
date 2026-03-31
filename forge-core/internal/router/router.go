package router

import (
	"github.com/gin-gonic/gin"
	"github.com/shulex/forge/forge-core/internal/middleware"
	"github.com/shulex/forge/forge-core/internal/module/auth"
	"github.com/shulex/forge/forge-core/internal/module/project"
)

type Deps struct {
	AuthHandler    *auth.Handler
	AuthService    *auth.Service
	ProjectHandler *project.Handler
}

func Setup(deps *Deps) *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(gin.Recovery())
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

			// Projects
			protected.POST("/projects", deps.ProjectHandler.Create)
			protected.GET("/projects", deps.ProjectHandler.List)
			protected.GET("/projects/:id", deps.ProjectHandler.GetByID)
			protected.PUT("/projects/:id", deps.ProjectHandler.Update)
			protected.DELETE("/projects/:id", deps.ProjectHandler.Archive)
			protected.POST("/projects/:id/star", deps.ProjectHandler.Star)
			protected.DELETE("/projects/:id/star", deps.ProjectHandler.Unstar)
		}
	}

	return r
}
