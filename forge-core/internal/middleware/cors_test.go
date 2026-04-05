package middleware

import (
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestCORS_DefaultOrigin(t *testing.T) {
	gin.SetMode(gin.TestMode)
	os.Unsetenv("CORS_ORIGINS")

	r := gin.New()
	r.Use(CORS())
	r.GET("/test", func(c *gin.Context) { c.String(200, "ok") })

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test", nil)
	req.Header.Set("Origin", "http://localhost:3000")
	r.ServeHTTP(w, req)

	origin := w.Header().Get("Access-Control-Allow-Origin")
	if origin != "http://localhost:3000" {
		t.Errorf("expected localhost:3000, got %s", origin)
	}
}

func TestCORS_OptionsPreflightReturns204(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(CORS())
	r.GET("/test", func(c *gin.Context) { c.String(200, "ok") })

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("OPTIONS", "/test", nil)
	r.ServeHTTP(w, req)

	if w.Code != 204 {
		t.Errorf("expected 204 for OPTIONS, got %d", w.Code)
	}
}

func TestCORS_AllowedHeaders(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(CORS())
	r.GET("/test", func(c *gin.Context) { c.String(200, "ok") })

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test", nil)
	r.ServeHTTP(w, req)

	allowHeaders := w.Header().Get("Access-Control-Allow-Headers")
	if allowHeaders == "" {
		t.Error("expected Allow-Headers to be set")
	}

	exposedHeaders := w.Header().Get("Access-Control-Expose-Headers")
	if exposedHeaders == "" {
		t.Error("expected Expose-Headers to be set")
	}
}

func TestCORS_CustomOrigins(t *testing.T) {
	gin.SetMode(gin.TestMode)
	os.Setenv("CORS_ORIGINS", "https://forge.example.com,https://admin.example.com")
	defer os.Unsetenv("CORS_ORIGINS")

	r := gin.New()
	r.Use(CORS())
	r.GET("/test", func(c *gin.Context) { c.String(200, "ok") })

	// Allowed origin
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test", nil)
	req.Header.Set("Origin", "https://forge.example.com")
	r.ServeHTTP(w, req)

	origin := w.Header().Get("Access-Control-Allow-Origin")
	if origin != "https://forge.example.com" {
		t.Errorf("expected forge.example.com, got %s", origin)
	}
}

func TestCORS_MaxAge(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(CORS())
	r.GET("/test", func(c *gin.Context) { c.String(200, "ok") })

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test", nil)
	r.ServeHTTP(w, req)

	maxAge := w.Header().Get("Access-Control-Max-Age")
	if maxAge != "3600" {
		t.Errorf("expected max-age 3600, got %s", maxAge)
	}
}
