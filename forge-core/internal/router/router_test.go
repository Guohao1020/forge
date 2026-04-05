package router

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSetup_HealthEndpoint(t *testing.T) {
	r := Setup(&Deps{})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/health", nil)
	r.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Errorf("GET /health: expected 200, got %d", w.Code)
	}
}

func TestSetup_MetricsEndpoint(t *testing.T) {
	r := Setup(&Deps{})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/metrics", nil)
	r.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Errorf("GET /metrics: expected 200, got %d", w.Code)
	}

	body := w.Body.String()
	if len(body) == 0 {
		t.Error("metrics body should not be empty")
	}
}

func TestSetup_SystemInfoEndpoint(t *testing.T) {
	r := Setup(&Deps{})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/system/info", nil)
	r.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Errorf("GET /api/system/info: expected 200, got %d", w.Code)
	}
}

func TestSetup_LoginEndpoint_NoAuthService(t *testing.T) {
	// Without AuthService, login route should still be registered
	// but will fail at handler level (nil pointer)
	r := Setup(&Deps{})

	routes := r.Routes()
	found := false
	for _, route := range routes {
		if route.Path == "/api/auth/login" && route.Method == "POST" {
			found = true
			break
		}
	}
	if !found {
		t.Error("POST /api/auth/login route should be registered")
	}
}

func TestSetup_CORS_Options(t *testing.T) {
	r := Setup(&Deps{})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("OPTIONS", "/api/auth/login", nil)
	r.ServeHTTP(w, req)

	if w.Code != 204 {
		t.Errorf("OPTIONS should return 204, got %d", w.Code)
	}
}

func TestSetup_SecurityHeaders(t *testing.T) {
	r := Setup(&Deps{})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/health", nil)
	r.ServeHTTP(w, req)

	if w.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Error("missing X-Content-Type-Options header")
	}
	if w.Header().Get("X-Frame-Options") != "DENY" {
		t.Error("missing X-Frame-Options header")
	}
}

func TestSetup_VersionHeader(t *testing.T) {
	r := Setup(&Deps{})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/health", nil)
	r.ServeHTTP(w, req)

	version := w.Header().Get("X-Forge-Version")
	if version == "" {
		t.Error("missing X-Forge-Version header")
	}
}

func TestSetup_RouteCount(t *testing.T) {
	r := Setup(&Deps{})

	routes := r.Routes()
	if len(routes) < 5 {
		t.Errorf("expected at least 5 routes, got %d", len(routes))
	}
}

func TestSetup_RequestID(t *testing.T) {
	r := Setup(&Deps{})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/health", nil)
	r.ServeHTTP(w, req)

	reqID := w.Header().Get("X-Request-ID")
	if reqID == "" {
		t.Error("expected X-Request-ID header")
	}
}

func TestSetup_RequestID_Forwarded(t *testing.T) {
	r := Setup(&Deps{})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/health", nil)
	req.Header.Set("X-Request-ID", "custom-id-123")
	r.ServeHTTP(w, req)

	reqID := w.Header().Get("X-Request-ID")
	if reqID != "custom-id-123" {
		t.Errorf("expected forwarded ID 'custom-id-123', got %s", reqID)
	}
}

func TestSetup_NotFound(t *testing.T) {
	r := Setup(&Deps{})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/nonexistent/path", nil)
	r.ServeHTTP(w, req)

	if w.Code != 404 {
		t.Errorf("expected 404, got %d", w.Code)
	}
}
