package auth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	ghAdapter "github.com/shulex/forge/forge-core/internal/adapter/github"
	"golang.org/x/crypto/bcrypt"
)

type Service struct {
	repo              *Repository
	jwtSecret         []byte
	jwtExpire         time.Duration
	githubClientID    string
	githubSecret      string
	githubRedirectURI string
}

func NewService(repo *Repository, jwtSecret string, jwtExpireHours int, githubClientID, githubSecret, githubRedirectURI string) *Service {
	return &Service{
		repo:              repo,
		jwtSecret:         []byte(jwtSecret),
		jwtExpire:         time.Duration(jwtExpireHours) * time.Hour,
		githubClientID:    githubClientID,
		githubSecret:      githubSecret,
		githubRedirectURI: githubRedirectURI,
	}
}

type Claims struct {
	jwt.RegisteredClaims
	UserID   int64  `json:"uid"`
	TenantID int64  `json:"tid"`
	Username string `json:"usr"`
}

func (s *Service) Login(ctx context.Context, req *LoginRequest, ipAddr string) (*LoginResponse, error) {
	const defaultTenantID int64 = 1

	user, err := s.repo.FindUserByUsername(ctx, defaultTenantID, req.Username)
	if err != nil {
		return nil, errors.New("用户名或密码错误")
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
		return nil, errors.New("用户名或密码错误")
	}

	if user.Status != "ACTIVE" {
		return nil, errors.New("用户名或密码错误")
	}

	jti := uuid.New().String()
	expiresAt := time.Now().Add(s.jwtExpire)

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, &Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			ID:        jti,
			ExpiresAt: jwt.NewNumericDate(expiresAt),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
		UserID:   user.ID,
		TenantID: user.TenantID,
		Username: user.Username,
	})

	tokenString, err := token.SignedString(s.jwtSecret)
	if err != nil {
		return nil, fmt.Errorf("sign token: %w", err)
	}

	_ = s.repo.SaveActiveToken(ctx, user.TenantID, user.ID, jti, "SESSION", "", ipAddr, expiresAt)
	_ = s.repo.UpdateLastLogin(ctx, user.ID)

	roles, _ := s.repo.GetUserRoles(ctx, user.ID)

	displayName := ""
	if user.DisplayName != nil {
		displayName = *user.DisplayName
	}
	avatarURL := ""
	if user.AvatarURL != nil {
		avatarURL = *user.AvatarURL
	}

	return &LoginResponse{
		Token:     tokenString,
		ExpiresAt: expiresAt,
		User: UserInfo{
			ID:          user.ID,
			TenantID:    user.TenantID,
			Username:    user.Username,
			DisplayName: displayName,
			AvatarURL:   avatarURL,
			Roles:       roles,
		},
	}, nil
}

func (s *Service) ValidateToken(ctx context.Context, tokenString string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(t *jwt.Token) (interface{}, error) {
		// Reject algorithm confusion attacks
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return s.jwtSecret, nil
	})
	if err != nil {
		return nil, err
	}
	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, errors.New("invalid token")
	}

	active, err := s.repo.IsTokenActive(ctx, claims.ID)
	if err != nil || !active {
		return nil, errors.New("token revoked")
	}

	return claims, nil
}

func (s *Service) Logout(ctx context.Context, jti string) error {
	return s.repo.DeleteActiveToken(ctx, jti)
}

func (s *Service) GetCurrentUser(ctx context.Context, userID int64) (*UserInfo, error) {
	user, err := s.repo.FindUserByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	roles, _ := s.repo.GetUserRoles(ctx, userID)

	displayName := ""
	if user.DisplayName != nil {
		displayName = *user.DisplayName
	}
	avatarURL := ""
	if user.AvatarURL != nil {
		avatarURL = *user.AvatarURL
	}

	return &UserInfo{
		ID:          user.ID,
		TenantID:    user.TenantID,
		Username:    user.Username,
		DisplayName: displayName,
		AvatarURL:   avatarURL,
		Roles:       roles,
	}, nil
}

// GetGitHubAuthorizeURL generates the GitHub OAuth authorization URL.
func (s *Service) GetGitHubAuthorizeURL(state string) string {
	params := url.Values{
		"client_id":    {s.githubClientID},
		"redirect_uri": {s.githubRedirectURI},
		"scope":        {"repo,read:org,read:user,user:email"},
		"state":        {state},
	}
	return "https://github.com/login/oauth/authorize?" + params.Encode()
}

// HandleGitHubCallback exchanges the OAuth code for a token, fetches the GitHub user,
// and saves/updates the identity binding for the current Forge user.
func (s *Service) HandleGitHubCallback(ctx context.Context, userID int64, code string) (*GitHubCallbackResponse, error) {
	tokenResp, err := ghAdapter.ExchangeCode(ctx, s.githubClientID, s.githubSecret, code)
	if err != nil {
		return nil, fmt.Errorf("exchange code: %w", err)
	}

	ghClient := ghAdapter.NewClient(tokenResp.AccessToken)
	ghUser, err := ghClient.GetAuthenticatedUser(ctx)
	if err != nil {
		return nil, fmt.Errorf("get github user: %w", err)
	}

	profileJSON, _ := json.Marshal(ghUser)
	identity := &UserIdentity{
		UserID:      userID,
		Provider:    "github",
		ProviderUID: strconv.FormatInt(ghUser.ID, 10),
		AccessToken: tokenResp.AccessToken,
		Profile:     string(profileJSON),
	}

	if _, err := s.repo.UpsertUserIdentity(ctx, identity); err != nil {
		return nil, fmt.Errorf("save identity: %w", err)
	}

	user, err := s.GetCurrentUser(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("get current user: %w", err)
	}

	return &GitHubCallbackResponse{
		User:     *user,
		Provider: "github",
		GitHubUser: struct {
			Login     string `json:"login"`
			AvatarURL string `json:"avatar_url"`
		}{
			Login:     ghUser.Login,
			AvatarURL: ghUser.AvatarURL,
		},
	}, nil
}

// GetGitHubToken retrieves the stored GitHub access token for a user.
func (s *Service) GetGitHubToken(ctx context.Context, userID int64) (string, error) {
	identity, err := s.repo.FindUserIdentity(ctx, userID, "github")
	if err != nil {
		return "", fmt.Errorf("github not connected: %w", err)
	}
	return identity.AccessToken, nil
}

// HasGitHubConnection checks if a user has a GitHub identity binding.
func (s *Service) HasGitHubConnection(ctx context.Context, userID int64) bool {
	_, err := s.repo.FindUserIdentity(ctx, userID, "github")
	return err == nil
}

// DisconnectGitHub removes the GitHub identity binding for a user.
func (s *Service) DisconnectGitHub(ctx context.Context, userID int64) error {
	return s.repo.DeleteUserIdentity(ctx, userID, "github")
}

// ListGitHubRepos lists all GitHub repositories for the authenticated user.
func (s *Service) ListGitHubRepos(ctx context.Context, userID int64) ([]ghAdapter.Repository, error) {
	token, err := s.GetGitHubToken(ctx, userID)
	if err != nil {
		return nil, err
	}
	client := ghAdapter.NewClient(token)
	return client.ListRepos(ctx)
}
