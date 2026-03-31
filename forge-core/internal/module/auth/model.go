package auth

import "time"

// DB model
type User struct {
	ID           int64      `json:"id"`
	TenantID     int64      `json:"tenant_id"`
	Username     string     `json:"username"`
	Email        *string    `json:"email,omitempty"`
	PasswordHash string     `json:"-"`
	DisplayName  *string    `json:"display_name,omitempty"`
	AvatarURL    *string    `json:"avatar_url,omitempty"`
	Status       string     `json:"status"`
	LastLoginAt  *time.Time `json:"last_login_at,omitempty"`
	CreatedAt    time.Time  `json:"created_at"`
}

type Role struct {
	ID   int64  `json:"id"`
	Code string `json:"code"`
	Name string `json:"name"`
}

// Request DTOs
type LoginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

// Response DTOs
type LoginResponse struct {
	Token     string    `json:"token"`
	ExpiresAt time.Time `json:"expires_at"`
	User      UserInfo  `json:"user"`
}

type UserInfo struct {
	ID          int64  `json:"id"`
	TenantID    int64  `json:"tenant_id"`
	Username    string `json:"username"`
	DisplayName string `json:"display_name"`
	AvatarURL   string `json:"avatar_url"`
	Roles       []Role `json:"roles"`
}
