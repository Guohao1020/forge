package agent

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
)

// fakeClarifyStore satisfies chatStore for Clarify handler tests.
type fakeClarifyStore struct {
	sessions map[string]*AgentSession
}

func (f *fakeClarifyStore) CreateSession(
	_ context.Context,
	id string, tenantID, projectID, createdBy int64,
	title *string, taskID *int64,
) (*AgentSession, error) {
	return nil, nil
}

func (f *fakeClarifyStore) GetSession(
	_ context.Context,
	sessionID string, projectID int64,
) (*AgentSession, error) {
	s, ok := f.sessions[sessionID]
	if !ok {
		return nil, nil
	}
	return s, nil
}

func (f *fakeClarifyStore) InsertMessage(
	_ context.Context, m *AgentMessage,
) error {
	return nil
}

func setupClarifyRouter(t *testing.T, store chatStore) (*gin.Engine, *miniredis.Miniredis) {
	t.Helper()
	gin.SetMode(gin.TestMode)

	mr, err := miniredis.Run()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(mr.Close)

	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { rdb.Close() })

	h := NewHandlerForTest(nil, rdb, store)
	r := gin.New()
	rg := r.Group("/api")
	// Inject auth context for tests
	rg.Use(func(c *gin.Context) {
		c.Set("tenant_id", int64(1))
		c.Set("user_id", int64(100))
		c.Next()
	})
	h.RegisterRoutes(rg)
	return r, mr
}

func TestClarify_HappyPath_204(t *testing.T) {
	store := &fakeClarifyStore{
		sessions: map[string]*AgentSession{
			"sess-001": {
				ID:        "sess-001",
				TenantID:  1,
				ProjectID: 42,
				CreatedBy: 100,
			},
		},
	}
	r, _ := setupClarifyRouter(t, store)

	body := `{"tool_use_id":"toolu_abc","response":"TypeScript"}`
	req := httptest.NewRequest(
		http.MethodPost,
		"/api/projects/42/agent/sessions/sess-001/clarify",
		strings.NewReader(body),
	)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNoContent {
		t.Errorf("expected 204, got %d: %s", w.Code, w.Body.String())
	}
}

func TestClarify_MissingToolUseID_400(t *testing.T) {
	store := &fakeClarifyStore{
		sessions: map[string]*AgentSession{
			"sess-001": {
				ID: "sess-001", TenantID: 1, ProjectID: 42, CreatedBy: 100,
			},
		},
	}
	r, _ := setupClarifyRouter(t, store)

	body := `{"response":"TypeScript"}`
	req := httptest.NewRequest(
		http.MethodPost,
		"/api/projects/42/agent/sessions/sess-001/clarify",
		strings.NewReader(body),
	)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestClarify_OversizedResponse_400(t *testing.T) {
	store := &fakeClarifyStore{
		sessions: map[string]*AgentSession{
			"sess-001": {
				ID: "sess-001", TenantID: 1, ProjectID: 42, CreatedBy: 100,
			},
		},
	}
	r, _ := setupClarifyRouter(t, store)

	bigResponse := strings.Repeat("x", 4097)
	body, _ := json.Marshal(map[string]string{
		"tool_use_id": "toolu_abc",
		"response":    bigResponse,
	})
	req := httptest.NewRequest(
		http.MethodPost,
		"/api/projects/42/agent/sessions/sess-001/clarify",
		strings.NewReader(string(body)),
	)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestClarify_SessionNotFound_403(t *testing.T) {
	store := &fakeClarifyStore{sessions: map[string]*AgentSession{}}
	r, _ := setupClarifyRouter(t, store)

	body := `{"tool_use_id":"toolu_abc","response":"TypeScript"}`
	req := httptest.NewRequest(
		http.MethodPost,
		"/api/projects/42/agent/sessions/sess-missing/clarify",
		strings.NewReader(body),
	)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	// Returns 403 (not 404) to avoid leaking session existence across tenants
	if w.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d: %s", w.Code, w.Body.String())
	}
}

func TestClarify_TenantMismatch_403(t *testing.T) {
	store := &fakeClarifyStore{
		sessions: map[string]*AgentSession{
			"sess-other": {
				ID:        "sess-other",
				TenantID:  999, // Different tenant
				ProjectID: 42,
				CreatedBy: 200, // Different user
			},
		},
	}
	r, _ := setupClarifyRouter(t, store)

	body := `{"tool_use_id":"toolu_abc","response":"TypeScript"}`
	req := httptest.NewRequest(
		http.MethodPost,
		"/api/projects/42/agent/sessions/sess-other/clarify",
		strings.NewReader(body),
	)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d", w.Code)
	}
}

func TestClarify_ToolUseIDTooLong_400(t *testing.T) {
	store := &fakeClarifyStore{
		sessions: map[string]*AgentSession{
			"sess-001": {
				ID: "sess-001", TenantID: 1, ProjectID: 42, CreatedBy: 100,
			},
		},
	}
	r, _ := setupClarifyRouter(t, store)

	longID := strings.Repeat("a", 129) // > 128 chars
	body, _ := json.Marshal(map[string]string{
		"tool_use_id": longID,
		"response":    "hello",
	})
	req := httptest.NewRequest(
		http.MethodPost,
		"/api/projects/42/agent/sessions/sess-001/clarify",
		strings.NewReader(string(body)),
	)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}
