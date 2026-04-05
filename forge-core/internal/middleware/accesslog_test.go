package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestAccessLog_DoesNotPanic(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(AccessLog())
	r.GET("/test", func(c *gin.Context) {
		c.String(200, "ok")
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test", nil)
	r.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestAccessLog_4xxWarning(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(AccessLog())
	r.GET("/notfound", func(c *gin.Context) {
		c.String(404, "not found")
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/notfound", nil)
	r.ServeHTTP(w, req)

	if w.Code != 404 {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

func TestAccessLog_5xxError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(AccessLog())
	r.GET("/error", func(c *gin.Context) {
		c.String(500, "error")
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/error", nil)
	r.ServeHTTP(w, req)

	if w.Code != 500 {
		t.Errorf("expected 500, got %d", w.Code)
	}
}
