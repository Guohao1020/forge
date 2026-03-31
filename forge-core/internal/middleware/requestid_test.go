package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestRequestIDMiddleware(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(RequestID())
	r.GET("/test", func(c *gin.Context) {
		rid := c.GetString("request_id")
		c.String(200, rid)
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test", nil)
	r.ServeHTTP(w, req)

	headerVal := w.Header().Get("X-Request-ID")
	if headerVal == "" {
		t.Fatal("X-Request-ID header should be set")
	}

	if w.Body.String() != headerVal {
		t.Fatalf("context request_id %q != header %q", w.Body.String(), headerVal)
	}
}

func TestRequestIDForwardsExisting(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(RequestID())
	r.GET("/test", func(c *gin.Context) {
		c.String(200, c.GetString("request_id"))
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test", nil)
	req.Header.Set("X-Request-ID", "incoming-id-123")
	r.ServeHTTP(w, req)

	if w.Body.String() != "incoming-id-123" {
		t.Fatalf("should forward incoming request ID, got %q", w.Body.String())
	}
}
