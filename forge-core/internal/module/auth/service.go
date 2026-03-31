package auth

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

type Service struct {
	repo      *Repository
	jwtSecret []byte
	jwtExpire time.Duration
}

func NewService(repo *Repository, jwtSecret string, jwtExpireHours int) *Service {
	return &Service{
		repo:      repo,
		jwtSecret: []byte(jwtSecret),
		jwtExpire: time.Duration(jwtExpireHours) * time.Hour,
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
