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

// UserIdentity represents an external OAuth identity binding.
type UserIdentity struct {
	ID           int64      `json:"id"`
	UserID       int64      `json:"user_id"`
	Provider     string     `json:"provider"`
	ProviderUID  string     `json:"provider_uid"`
	AccessToken  string     `json:"-"`
	RefreshToken string     `json:"-"`
	TokenExpires *time.Time `json:"token_expires,omitempty"`
	Profile      string     `json:"profile"`
	CreatedAt    time.Time  `json:"created_at"`
}

// GitHubAuthorizeResponse is returned by the authorize endpoint.
type GitHubAuthorizeResponse struct {
	AuthorizeURL string `json:"authorize_url"`
}

// GitHubCallbackRequest is the query parameters from GitHub OAuth callback.
type GitHubCallbackRequest struct {
	Code  string `form:"code" binding:"required"`
	State string `form:"state"`
}

// GitHubCallbackResponse is returned after successful OAuth callback.
type GitHubCallbackResponse struct {
	User       UserInfo `json:"user"`
	Provider   string   `json:"provider"`
	GitHubUser struct {
		Login     string `json:"login"`
		AvatarURL string `json:"avatar_url"`
	} `json:"github_user"`
}
