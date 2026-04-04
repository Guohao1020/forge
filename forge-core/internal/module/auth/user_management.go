package auth

import (
	"context"
	"fmt"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/shulex/forge/forge-core/internal/pkg/response"
	"golang.org/x/crypto/bcrypt"
)

// --- Models ---

type UserListItem struct {
	ID          int64    `json:"id"`
	Username    string   `json:"username"`
	DisplayName string   `json:"displayName"`
	Roles       []string `json:"roles"`
	Status      string   `json:"status"`
	LastLoginAt *string  `json:"lastLoginAt,omitempty"`
}

type CreateUserRequest struct {
	Username    string `json:"username" binding:"required,min=3,max=50"`
	Password    string `json:"password" binding:"required,min=6"`
	DisplayName string `json:"displayName"`
	Role        string `json:"role"` // DEVELOPER, PROJECT_ADMIN, etc.
}

type UpdateUserRoleRequest struct {
	Role string `json:"role" binding:"required"`
}

// --- Repository Methods ---

func (r *Repository) ListUsers(ctx context.Context, tenantID int64) ([]UserListItem, error) {
	rows, err := r.db.Query(ctx,
		`SELECT u.id, u.username, COALESCE(u.display_name, ''), u.status,
		        COALESCE(TO_CHAR(u.last_login_at, 'YYYY-MM-DD HH24:MI'), '')
		 FROM auth.users u
		 WHERE u.tenant_id = $1
		 ORDER BY u.created_at ASC`,
		tenantID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []UserListItem
	for rows.Next() {
		var u UserListItem
		var lastLogin string
		if err := rows.Scan(&u.ID, &u.Username, &u.DisplayName, &u.Status, &lastLogin); err != nil {
			continue
		}
		if lastLogin != "" {
			u.LastLoginAt = &lastLogin
		}
		// Get roles
		roles, _ := r.GetUserRoles(ctx, u.ID)
		u.Roles = make([]string, 0, len(roles))
		for _, role := range roles {
			u.Roles = append(u.Roles, role.Code)
		}
		users = append(users, u)
	}
	if users == nil {
		users = []UserListItem{}
	}
	return users, nil
}

func (r *Repository) CreateUser(ctx context.Context, tenantID int64, username, passwordHash, displayName string) (int64, error) {
	var id int64
	err := r.db.QueryRow(ctx,
		`INSERT INTO auth.users (tenant_id, username, password_hash, display_name, status)
		 VALUES ($1, $2, $3, $4, 'ACTIVE')
		 RETURNING id`,
		tenantID, username, passwordHash, displayName,
	).Scan(&id)
	return id, err
}

func (r *Repository) AssignRole(ctx context.Context, userID int64, roleCode string) error {
	_, err := r.db.Exec(ctx,
		`INSERT INTO auth.user_roles (user_id, role_id)
		 SELECT $1, r.id FROM auth.roles r WHERE r.code = $2
		 ON CONFLICT DO NOTHING`,
		userID, roleCode,
	)
	return err
}

func (r *Repository) RemoveAllRoles(ctx context.Context, userID int64) error {
	_, err := r.db.Exec(ctx, `DELETE FROM auth.user_roles WHERE user_id = $1`, userID)
	return err
}

// --- Service Methods ---

func (s *Service) ListUsers(ctx context.Context, tenantID int64) ([]UserListItem, error) {
	return s.repo.ListUsers(ctx, tenantID)
}

func (s *Service) CreateUser(ctx context.Context, tenantID int64, req *CreateUserRequest) (int64, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return 0, fmt.Errorf("hash password: %w", err)
	}
	displayName := req.DisplayName
	if displayName == "" {
		displayName = req.Username
	}

	userID, err := s.repo.CreateUser(ctx, tenantID, req.Username, string(hash), displayName)
	if err != nil {
		return 0, fmt.Errorf("create user: %w", err)
	}

	// Assign role (default: DEVELOPER)
	role := req.Role
	if role == "" {
		role = "DEVELOPER" // default role for new users
	}
	if err := s.repo.AssignRole(ctx, userID, role); err != nil {
		return userID, fmt.Errorf("assign role: %w", err)
	}

	return userID, nil
}

func (s *Service) UpdateUserRole(ctx context.Context, userID int64, role string) error {
	if err := s.repo.RemoveAllRoles(ctx, userID); err != nil {
		return err
	}
	return s.repo.AssignRole(ctx, userID, role)
}

// --- Handler Methods ---

func (h *Handler) ListUsers(c *gin.Context) {
	tid, _ := c.Get("tenant_id")
	tenantID, _ := tid.(int64)

	users, err := h.service.ListUsers(c.Request.Context(), tenantID)
	if err != nil {
		response.Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	response.OK(c, gin.H{"users": users})
}

func (h *Handler) CreateUser(c *gin.Context) {
	tid, _ := c.Get("tenant_id")
	tenantID, _ := tid.(int64)

	var req CreateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, "invalid request: "+err.Error())
		return
	}

	userID, err := h.service.CreateUser(c.Request.Context(), tenantID, &req)
	if err != nil {
		response.Fail(c, http.StatusBadRequest, err.Error())
		return
	}
	response.OK(c, gin.H{"id": userID})
}

func (h *Handler) UpdateUserRole(c *gin.Context) {
	userID, err := strconv.ParseInt(c.Param("userId"), 10, 64)
	if err != nil {
		response.Fail(c, http.StatusBadRequest, "invalid user id")
		return
	}

	var req UpdateUserRoleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, "invalid request: "+err.Error())
		return
	}

	if err := h.service.UpdateUserRole(c.Request.Context(), userID, req.Role); err != nil {
		response.Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	response.OK(c, gin.H{"status": "role_updated"})
}
