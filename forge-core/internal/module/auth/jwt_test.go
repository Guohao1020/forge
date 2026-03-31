package auth

import (
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func TestValidateTokenRejectsNoneAlgorithm(t *testing.T) {
	s := &Service{jwtSecret: []byte("test-secret")}

	token := jwt.NewWithClaims(jwt.SigningMethodNone, &Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			ID:        "test-jti",
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
		},
		UserID: 1, TenantID: 1, Username: "admin",
	})
	tokenString, _ := token.SignedString(jwt.UnsafeAllowNoneSignatureType)

	_, err := s.ValidateToken(t.Context(), tokenString)
	if err == nil {
		t.Fatal("should reject alg:none token")
	}
}

func TestValidateTokenAcceptsHS256(t *testing.T) {
	secret := []byte("test-secret-32-bytes-long-enough")
	s := &Service{
		jwtSecret: secret,
		repo:      nil,
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, &Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			ID:        "test-jti",
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
		},
		UserID: 1, TenantID: 1, Username: "admin",
	})
	tokenString, _ := token.SignedString(secret)

	// Will panic at repo.IsTokenActive (nil repo) — that means JWT parse succeeded
	func() {
		defer func() {
			r := recover()
			if r == nil {
				t.Fatal("expected panic from nil repo, meaning JWT parse succeeded")
			}
		}()
		s.ValidateToken(t.Context(), tokenString)
	}()
}
