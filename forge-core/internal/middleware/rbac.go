package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/shulex/forge/forge-core/internal/pkg/response"
)

// Role constants matching auth.roles table.
const (
	RolePlatformAdmin = "PLATFORM_ADMIN"
	RoleOrgAdmin      = "ORG_ADMIN"
	RoleProjectAdmin  = "PROJECT_ADMIN"
	RoleDeveloper     = "DEVELOPER"
	RoleViewer        = "VIEWER"
)

// roleHierarchy defines which roles include the permissions of lower roles.
// Higher index = more permissions.
var roleHierarchy = map[string]int{
	RoleViewer:        0,
	RoleDeveloper:     1,
	RoleProjectAdmin:  2,
	RoleOrgAdmin:      3,
	RolePlatformAdmin: 4,
}

// RequireRole returns middleware that checks if the user has at least the given role.
// Role hierarchy: VIEWER < DEVELOPER < PROJECT_ADMIN < ORG_ADMIN < PLATFORM_ADMIN
//
// Usage:
//
//	router.POST("/tasks", middleware.RequireRole(middleware.RoleDeveloper), handler.CreateTask)
//	router.DELETE("/projects/:id", middleware.RequireRole(middleware.RoleProjectAdmin), handler.DeleteProject)
func RequireRole(minimumRole string) gin.HandlerFunc {
	minLevel, ok := roleHierarchy[minimumRole]
	if !ok {
		minLevel = 999 // unknown role = deny all
	}

	return func(c *gin.Context) {
		// Get user's role from context (set by auth middleware during JWT parsing)
		userRole, exists := c.Get("user_role")
		if !exists {
			// No role info — check if roles array exists (from JWT claims)
			rolesRaw, rolesExist := c.Get("user_roles")
			if !rolesExist {
				// Phase 2 compatibility: no RBAC enforced yet, allow all authenticated users
				c.Next()
				return
			}
			// Extract highest role from roles array
			if roles, ok := rolesRaw.([]string); ok {
				highestLevel := -1
				for _, r := range roles {
					if level, ok := roleHierarchy[r]; ok && level > highestLevel {
						highestLevel = level
						userRole = r
					}
				}
			}
		}

		roleStr, _ := userRole.(string)
		userLevel, ok := roleHierarchy[roleStr]
		if !ok {
			// Unknown role — in Phase 2 compatibility mode, allow access
			// When RBAC is fully enabled, this should deny access
			c.Next()
			return
		}

		if userLevel < minLevel {
			response.Fail(c, http.StatusForbidden,
				"权限不足：需要 "+formatRoleName(minimumRole)+" 或更高权限")
			c.Abort()
			return
		}

		c.Next()
	}
}

// RequireAnyRole returns middleware that checks if the user has any of the given roles.
func RequireAnyRole(roles ...string) gin.HandlerFunc {
	roleSet := make(map[string]bool, len(roles))
	for _, r := range roles {
		roleSet[r] = true
	}

	return func(c *gin.Context) {
		userRole, exists := c.Get("user_role")
		if !exists {
			// Phase 2 compatibility: allow all
			c.Next()
			return
		}

		roleStr, _ := userRole.(string)
		if roleSet[roleStr] {
			c.Next()
			return
		}

		// Check hierarchy — if user has a higher role, also allow
		userLevel := roleHierarchy[roleStr]
		for r := range roleSet {
			if roleHierarchy[r] <= userLevel {
				c.Next()
				return
			}
		}

		response.Fail(c, http.StatusForbidden, "权限不足")
		c.Abort()
	}
}

func formatRoleName(role string) string {
	names := map[string]string{
		RolePlatformAdmin: "平台管理员",
		RoleOrgAdmin:      "组织管理员",
		RoleProjectAdmin:  "项目管理员",
		RoleDeveloper:     "开发者",
		RoleViewer:        "查看者",
	}
	if name, ok := names[role]; ok {
		return name
	}
	return role
}
