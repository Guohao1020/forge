package middleware

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

// These tests verify the HTTP-layer behavior of JWTAuth middleware
// (header/query extraction, abort on missing token) without requiring
// a database. JWT validation logic is tested in auth/jwt_test.go.

type failResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func TestJWTAuth_NoToken_Returns401(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	// Pass nil service — middleware should abort before calling ValidateToken
	// because no token is present.
	r.Use(JWTAuth(nil))
	r.GET("/protected", func(c *gin.Context) {
		c.String(200, "ok")
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/protected", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}

	var resp failResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if resp.Code != -1 {
		t.Fatalf("expected error code -1, got %d", resp.Code)
	}
}

func TestJWTAuth_EmptyBearerHeader_Returns401(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(JWTAuth(nil))
	r.GET("/protected", func(c *gin.Context) {
		c.String(200, "ok")
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/protected", nil)
	req.Header.Set("Authorization", "Bearer ")
	r.ServeHTTP(w, req)

	// "Bearer " with empty token should still result in empty tokenString
	// because TrimPrefix("Bearer ", "Bearer ") == ""
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for empty bearer token, got %d", w.Code)
	}
}

func TestJWTAuth_NonBearerScheme_Returns401(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(JWTAuth(nil))
	r.GET("/protected", func(c *gin.Context) {
		c.String(200, "ok")
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/protected", nil)
	req.Header.Set("Authorization", "Basic dXNlcjpwYXNz")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for non-Bearer scheme, got %d", w.Code)
	}
}

func TestJWTAuth_BodyNotReachable_WhenNoToken(t *testing.T) {
	gin.SetMode(gin.TestMode)
	reached := false
	r := gin.New()
	r.Use(JWTAuth(nil))
	r.GET("/protected", func(c *gin.Context) {
		reached = true
		c.String(200, "ok")
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/protected", nil)
	r.ServeHTTP(w, req)

	if reached {
		t.Fatal("handler should not be reached when token is missing")
	}
	_ = w // use w to suppress unused warning
}
