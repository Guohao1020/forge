package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
)

// ---- mapToJSON regression tests -------------------------------------------

func TestMapToJSON_SimpleStringMap(t *testing.T) {
	out := mapToJSON(map[string]interface{}{
		"type": "text_delta",
		"text": "hello",
	})

	var decoded map[string]string
	if err := json.Unmarshal([]byte(out), &decoded); err != nil {
		t.Fatalf("output is not valid JSON: %v; got %q", err, out)
	}
	if decoded["type"] != "text_delta" {
		t.Errorf("type mismatch: got %q", decoded["type"])
	}
	if decoded["text"] != "hello" {
		t.Errorf("text mismatch: got %q", decoded["text"])
	}
}

func TestMapToJSON_QuotesAndBackslashes(t *testing.T) {
	// The previous hand-rolled implementation broke on any value containing
	// a double quote or backslash. This is the regression test that pins
	// the encoding/json fix.
	out := mapToJSON(map[string]interface{}{
		"type": "text_delta",
		"text": `hello "world" with \ backslash`,
	})

	var decoded map[string]string
	if err := json.Unmarshal([]byte(out), &decoded); err != nil {
		t.Fatalf("output is not valid JSON: %v; got %q", err, out)
	}
	if decoded["text"] != `hello "world" with \ backslash` {
		t.Errorf("text round-trip failed: got %q", decoded["text"])
	}
}

func TestMapToJSON_Newlines(t *testing.T) {
	out := mapToJSON(map[string]interface{}{
		"type": "code",
		"text": "line 1\nline 2\nline 3",
	})

	var decoded map[string]string
	if err := json.Unmarshal([]byte(out), &decoded); err != nil {
		t.Fatalf("output is not valid JSON: %v; got %q", err, out)
	}
	if decoded["text"] != "line 1\nline 2\nline 3" {
		t.Errorf("text round-trip failed: got %q", decoded["text"])
	}
}

func TestMapToJSON_UnicodeAndEmoji(t *testing.T) {
	out := mapToJSON(map[string]interface{}{
		"text": "你好 🚀 мир",
	})
	var decoded map[string]string
	if err := json.Unmarshal([]byte(out), &decoded); err != nil {
		t.Fatalf("output is not valid JSON: %v; got %q", err, out)
	}
	if decoded["text"] != "你好 🚀 мир" {
		t.Errorf("unicode round-trip failed: got %q", decoded["text"])
	}
}

func TestMapToJSON_EmptyMap(t *testing.T) {
	out := mapToJSON(map[string]interface{}{})
	if out != "{}" {
		t.Errorf("expected {}, got %q", out)
	}
}

// ---- Chat handler tests ---------------------------------------------------

func newTestService(t *testing.T, aiWorker *httptest.Server) *Service {
	t.Helper()
	return NewService(aiWorker.URL)
}

func newChatRouter(h *Handler) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/projects/:id/agent/chat", h.Chat)
	return r
}

func TestChat_InvalidProjectID(t *testing.T) {
	aiWorker := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer aiWorker.Close()

	h := NewHandler(newTestService(t, aiWorker), nil, nil)
	r := newChatRouter(h)

	body := strings.NewReader(`{"message":"hello"}`)
	req := httptest.NewRequest(http.MethodPost, "/projects/not-a-number/agent/chat", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for non-numeric project id, got %d", w.Code)
	}
}

func TestChat_InvalidJSONBody(t *testing.T) {
	aiWorker := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer aiWorker.Close()

	h := NewHandler(newTestService(t, aiWorker), nil, nil)
	r := newChatRouter(h)

	req := httptest.NewRequest(http.MethodPost, "/projects/42/agent/chat", strings.NewReader("{not json"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for malformed JSON, got %d", w.Code)
	}
}

func TestChat_AIWorkerUnavailable(t *testing.T) {
	// Point the service at a closed server so the HTTP call fails.
	closedServer := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	closedServer.Close() // Kill immediately

	h := NewHandler(newTestService(t, closedServer), nil, nil)
	r := newChatRouter(h)

	body := strings.NewReader(`{"message":"hello","session_id":"s1"}`)
	req := httptest.NewRequest(http.MethodPost, "/projects/42/agent/chat", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadGateway {
		t.Errorf("expected 502 when ai-worker is unreachable, got %d", w.Code)
	}
}

func TestChat_AIWorkerReturnsNon200(t *testing.T) {
	aiWorker := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error":"engine crashed"}`))
	}))
	defer aiWorker.Close()

	h := NewHandler(newTestService(t, aiWorker), nil, nil)
	r := newChatRouter(h)

	body := strings.NewReader(`{"message":"hello","session_id":"s1"}`)
	req := httptest.NewRequest(http.MethodPost, "/projects/42/agent/chat", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadGateway {
		t.Errorf("expected 502 when ai-worker returns 500, got %d", w.Code)
	}
}

// ---- Suggestions tests (Stream 4c) ---------------------------------------

func TestNormalizeLanguage_HandlesAliases(t *testing.T) {
	cases := map[string]string{
		"Java":       "java",
		"python":     "python",
		"Py":         "python",
		"Go":         "go",
		"golang":     "go",
		"TypeScript": "typescript",
		"ts":         "typescript",
		"javascript": "javascript",
		"Node":       "javascript",
		"nodejs":     "javascript",
		"rust":       "rust",
		"  Java  ":   "java",
		"c++":        "",
		"":           "",
	}
	for input, want := range cases {
		if got := normalizeLanguage(input); got != want {
			t.Errorf("normalizeLanguage(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestDefaultSuggestionsFallback(t *testing.T) {
	// Handler with nil repo → falls back to defaults without touching DB.
	h := NewHandler(nil, nil, nil)
	resp, err := h.generateSuggestions(t.Context(), 42)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Source != "fallback" {
		t.Errorf("expected fallback source, got %q", resp.Source)
	}
	if len(resp.Suggestions) != 3 {
		t.Errorf("expected 3 default suggestions, got %d", len(resp.Suggestions))
	}
}

func TestLanguageSuggestionsHaveAllFiveLanguages(t *testing.T) {
	wanted := []string{"java", "python", "go", "typescript", "javascript", "rust"}
	for _, lang := range wanted {
		if _, ok := languageSuggestions[lang]; !ok {
			t.Errorf("missing suggestions for language %q", lang)
		}
	}
	for lang, suggestions := range languageSuggestions {
		if len(suggestions) != 3 {
			t.Errorf("language %q has %d suggestions, want 3", lang, len(suggestions))
		}
	}
}

func TestChat_SuccessReturnsSessionID(t *testing.T) {
	aiWorker := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/run" {
			t.Errorf("expected /api/run, got %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"session_id":"abc-123","status":"accepted","correlation_id":"corr-1"}`))
	}))
	defer aiWorker.Close()

	h := NewHandler(newTestService(t, aiWorker), nil, nil)
	r := newChatRouter(h)

	body := strings.NewReader(`{"message":"build me a calculator","project_id":42}`)
	req := httptest.NewRequest(http.MethodPost, "/projects/42/agent/chat", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusAccepted {
		t.Errorf("expected 202, got %d; body=%s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), `"session_id":"abc-123"`) {
		t.Errorf("response body missing session_id: %s", w.Body.String())
	}
}

// ---- Dual-storage Chat tests (TASK 5: persist before ai-worker) -----------

// fakeChatStore is an in-memory chatStore for unit tests.
type fakeChatStore struct {
	createSessionErr error
	insertMessageErr error
	getSessionErr    error

	// preloaded sessions keyed by session_id, returned by GetSession
	// when the SQL filter (project_id) matches. Use seed() to populate.
	sessions map[string]*AgentSession

	createCalls  int
	insertCalls  int
	getCalls     int
	insertedMsgs []*AgentMessage
	createdID    string
}

func (f *fakeChatStore) seed(s *AgentSession) {
	if f.sessions == nil {
		f.sessions = map[string]*AgentSession{}
	}
	f.sessions[s.ID] = s
}

func (f *fakeChatStore) CreateSession(
	_ context.Context,
	id string,
	tenantID int64,
	projectID int64,
	createdBy int64,
	_ *string,
	_ *int64,
) (*AgentSession, error) {
	f.createCalls++
	if f.createSessionErr != nil {
		return nil, f.createSessionErr
	}
	f.createdID = id
	s := &AgentSession{
		ID:        id,
		TenantID:  tenantID,
		ProjectID: projectID,
		CreatedBy: createdBy,
	}
	f.seed(s)
	return s, nil
}

func (f *fakeChatStore) GetSession(_ context.Context, sessionID string, projectID int64) (*AgentSession, error) {
	f.getCalls++
	if f.getSessionErr != nil {
		return nil, f.getSessionErr
	}
	s, ok := f.sessions[sessionID]
	if !ok {
		return nil, nil
	}
	// Mirror the SQL filter on project_id — sessions outside the URL's
	// project must look as if they don't exist.
	if s.ProjectID != 0 && s.ProjectID != projectID {
		return nil, nil
	}
	return s, nil
}

func (f *fakeChatStore) InsertMessage(_ context.Context, m *AgentMessage) error {
	f.insertCalls++
	if f.insertMessageErr != nil {
		return f.insertMessageErr
	}
	f.insertedMsgs = append(f.insertedMsgs, m)
	return nil
}

// newDualStorageChatRouter wires a Handler with a fake chatStore plus an
// auth-middleware shim that injects tenant_id / user_id into the gin
// context (matching what the real auth middleware does in production).
func newDualStorageChatRouter(h *Handler, tenantID, userID int64) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("tenant_id", tenantID)
		c.Set("user_id", userID)
		c.Next()
	})
	r.POST("/projects/:id/agent/chat", h.Chat)
	return r
}

func TestChat_PersistsUserMessageBeforeAIWorker(t *testing.T) {
	// Happy path: ai-worker succeeds. Verify CreateSession + InsertMessage
	// were called and the user_message landed in the fake store.
	aiWorker := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"session_id":"server-sid","status":"accepted","correlation_id":"c1"}`))
	}))
	defer aiWorker.Close()

	store := &fakeChatStore{}
	h := NewHandlerForTest(newTestService(t, aiWorker), nil, store)
	r := newDualStorageChatRouter(h, 1, 7)

	body := strings.NewReader(`{"message":"hello world"}`)
	req := httptest.NewRequest(http.MethodPost, "/projects/42/agent/chat", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d; body=%s", w.Code, w.Body.String())
	}
	if store.createCalls != 1 {
		t.Errorf("expected CreateSession called once, got %d", store.createCalls)
	}
	if store.insertCalls != 1 {
		t.Errorf("expected InsertMessage called once, got %d", store.insertCalls)
	}
	if len(store.insertedMsgs) != 1 {
		t.Fatalf("expected 1 inserted message, got %d", len(store.insertedMsgs))
	}
	msg := store.insertedMsgs[0]
	if msg.EventType != "user_message" {
		t.Errorf("expected event_type=user_message, got %q", msg.EventType)
	}
	if msg.Content == nil || *msg.Content != "hello world" {
		t.Errorf("expected content 'hello world', got %v", msg.Content)
	}
	if msg.Role == nil || *msg.Role != "user" {
		t.Errorf("expected role=user, got %v", msg.Role)
	}
	if msg.SessionID != store.createdID {
		t.Errorf("inserted msg session_id %q != created session id %q",
			msg.SessionID, store.createdID)
	}
}

func TestChat_UserMessageDurableWhenAIWorkerReturns502(t *testing.T) {
	// CRITICAL contract from TASK 5: when ai-worker fails, the user_message
	// MUST already be persisted in PG so the next sidebar load can hydrate
	// it and the user can retry without losing context. Plan G1.
	aiWorker := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error":"engine crashed"}`))
	}))
	defer aiWorker.Close()

	store := &fakeChatStore{}
	h := NewHandlerForTest(newTestService(t, aiWorker), nil, store)
	r := newDualStorageChatRouter(h, 1, 7)

	body := strings.NewReader(`{"message":"build me a calculator"}`)
	req := httptest.NewRequest(http.MethodPost, "/projects/42/agent/chat", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadGateway {
		t.Fatalf("expected 502 when ai-worker fails, got %d; body=%s", w.Code, w.Body.String())
	}
	// The whole point of TASK 5: even on 502, the message survives.
	if len(store.insertedMsgs) != 1 {
		t.Fatalf("expected user_message persisted despite 502, got %d messages",
			len(store.insertedMsgs))
	}
	if *store.insertedMsgs[0].Content != "build me a calculator" {
		t.Errorf("persisted content mismatch: %q", *store.insertedMsgs[0].Content)
	}
	// Response body should still echo the session_id so the client can
	// retry against the same conversation.
	if !strings.Contains(w.Body.String(), `"session_id":"`+store.createdID+`"`) {
		t.Errorf("502 response missing session_id for retry: %s", w.Body.String())
	}
}

func TestChat_PGWriteFailureDoesNotCallAIWorker(t *testing.T) {
	// If we cannot persist the user_message we must NOT contact ai-worker —
	// otherwise the user retries, we double-write to ai-worker, and the
	// LLM bills run twice for one prompt. Return 500, no downstream call.
	aiWorkerHit := false
	aiWorker := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		aiWorkerHit = true
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"session_id":"x","status":"ok"}`))
	}))
	defer aiWorker.Close()

	store := &fakeChatStore{
		insertMessageErr: fmt.Errorf("simulated PG outage"),
	}
	h := NewHandlerForTest(newTestService(t, aiWorker), nil, store)
	r := newDualStorageChatRouter(h, 1, 7)

	body := strings.NewReader(`{"message":"hello"}`)
	req := httptest.NewRequest(http.MethodPost, "/projects/42/agent/chat", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected 500 when PG insert fails, got %d", w.Code)
	}
	if aiWorkerHit {
		t.Errorf("ai-worker should NOT be called when PG persistence fails")
	}
}

func TestChat_RejectsUnauthenticatedWhenStoreWired(t *testing.T) {
	// When the dual-storage path is active we require auth context. The
	// legacy nil-repo path keeps working (covered by other tests) so this
	// only fires on the new code path.
	store := &fakeChatStore{}
	h := NewHandlerForTest(nil, nil, store) // service nil, never reached

	gin.SetMode(gin.TestMode)
	r := gin.New() // No auth middleware → tenant_id/user_id not set
	r.POST("/projects/:id/agent/chat", h.Chat)

	body := strings.NewReader(`{"message":"hello"}`)
	req := httptest.NewRequest(http.MethodPost, "/projects/42/agent/chat", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 without auth context, got %d", w.Code)
	}
	if store.createCalls != 0 || store.insertCalls != 0 {
		t.Errorf("store should not be touched when auth fails")
	}
}

func TestChat_ReusesExistingSessionID(t *testing.T) {
	// When the client supplies session_id (subsequent turns in an existing
	// conversation), Handler must NOT create a new session — only insert
	// the user_message and forward. Tenant ownership check still runs;
	// the session must be seeded with matching tenant/user (1, 7).
	aiWorker := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"session_id":"existing-sid","status":"ok"}`))
	}))
	defer aiWorker.Close()

	store := &fakeChatStore{}
	store.seed(&AgentSession{
		ID:        "existing-sid",
		TenantID:  1,
		ProjectID: 42,
		CreatedBy: 7,
	})
	h := NewHandlerForTest(newTestService(t, aiWorker), nil, store)
	r := newDualStorageChatRouter(h, 1, 7)

	body := strings.NewReader(`{"message":"follow-up","session_id":"existing-sid"}`)
	req := httptest.NewRequest(http.MethodPost, "/projects/42/agent/chat", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d; body=%s", w.Code, w.Body.String())
	}
	if store.createCalls != 0 {
		t.Errorf("expected CreateSession NOT called for follow-up, got %d", store.createCalls)
	}
	if store.insertCalls != 1 {
		t.Errorf("expected InsertMessage called once, got %d", store.insertCalls)
	}
	if store.insertedMsgs[0].SessionID != "existing-sid" {
		t.Errorf("expected message persisted to existing-sid, got %q",
			store.insertedMsgs[0].SessionID)
	}
	if store.getCalls != 1 {
		t.Errorf("expected GetSession called once for ownership check, got %d", store.getCalls)
	}
}

// ---- Cross-tenant ownership tests (TASK 6: G2/G3) -------------------------

func TestChat_RejectsCrossTenantSession(t *testing.T) {
	// Attacker (tenant=2, user=99) tries to write into a session owned
	// by tenant=1. MUST return 403, MUST NOT call ai-worker, MUST NOT
	// insert any message into the victim's session.
	aiWorkerHit := false
	aiWorker := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		aiWorkerHit = true
		w.WriteHeader(http.StatusOK)
	}))
	defer aiWorker.Close()

	store := &fakeChatStore{}
	store.seed(&AgentSession{
		ID:        "victim-sid",
		TenantID:  1, // owned by tenant 1
		ProjectID: 42,
		CreatedBy: 7,
	})
	h := NewHandlerForTest(newTestService(t, aiWorker), nil, store)
	r := newDualStorageChatRouter(h, 2, 99) // attacker tenant 2

	body := strings.NewReader(`{"message":"steal","session_id":"victim-sid"}`)
	req := httptest.NewRequest(http.MethodPost, "/projects/42/agent/chat", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403 cross-tenant, got %d; body=%s", w.Code, w.Body.String())
	}
	if aiWorkerHit {
		t.Errorf("ai-worker MUST NOT be contacted on cross-tenant session")
	}
	if store.insertCalls != 0 {
		t.Errorf("InsertMessage MUST NOT be called for cross-tenant session, got %d calls", store.insertCalls)
	}
}

func TestChat_RejectsCrossUserSameTenant(t *testing.T) {
	// Same tenant, different user. Sessions are per-user — even within
	// a tenant, user A cannot inject messages into user B's session.
	store := &fakeChatStore{}
	store.seed(&AgentSession{
		ID:        "user7-sid",
		TenantID:  1,
		ProjectID: 42,
		CreatedBy: 7, // owned by user 7
	})
	h := NewHandlerForTest(nil, nil, store) // service nil — never reached
	r := newDualStorageChatRouter(h, 1, 8)  // same tenant, different user

	body := strings.NewReader(`{"message":"hi","session_id":"user7-sid"}`)
	req := httptest.NewRequest(http.MethodPost, "/projects/42/agent/chat", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("expected 403 cross-user, got %d; body=%s", w.Code, w.Body.String())
	}
}

func TestChat_NonExistentSessionReturnsForbidden(t *testing.T) {
	// Plan TASK 6 explicit requirement: 404 vs 403 differently would
	// leak existence. Both "session does not exist" and "session belongs
	// to someone else" must return 403.
	store := &fakeChatStore{}
	h := NewHandlerForTest(nil, nil, store)
	r := newDualStorageChatRouter(h, 1, 7)

	body := strings.NewReader(`{"message":"hi","session_id":"does-not-exist"}`)
	req := httptest.NewRequest(http.MethodPost, "/projects/42/agent/chat", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("expected 403 for non-existent session (no existence leak), got %d", w.Code)
	}
}

// ---- Stream cross-tenant tests --------------------------------------------

func newDualStorageStreamRouter(h *Handler, tenantID, userID int64) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("tenant_id", tenantID)
		c.Set("user_id", userID)
		c.Next()
	})
	r.GET("/projects/:id/agent/stream", h.Stream)
	return r
}

func TestStream_RejectsCrossTenantSession(t *testing.T) {
	// G3 — the most dangerous gap. Without this check, an authenticated
	// user can subscribe to ANY session_id and read its live SSE flow.
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})

	// Seed a real Redis stream so we can detect if the handler ever
	// reads from it. If the test passes (403) it should remain unread.
	_, _ = mr.XAdd("agent:stream:victim-sid", "*", []string{"type", "secret_text", "text", "private data"})

	store := &fakeChatStore{}
	store.seed(&AgentSession{
		ID:        "victim-sid",
		TenantID:  1,
		ProjectID: 42,
		CreatedBy: 7,
	})
	h := NewHandlerForTest(nil, rdb, store)
	r := newDualStorageStreamRouter(h, 2, 99) // attacker

	srv := httptest.NewServer(r)
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	req, _ := http.NewRequestWithContext(ctx, http.MethodGet,
		srv.URL+"/projects/42/agent/stream?session_id=victim-sid", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("expected 403 cross-tenant SSE access, got %d", resp.StatusCode)
	}

	// Read body and ensure no leak of the seeded stream content.
	body, _ := io.ReadAll(resp.Body)
	if strings.Contains(string(body), "private data") {
		t.Errorf("SSE response leaked victim stream content: %s", string(body))
	}
	if strings.Contains(string(body), "secret_text") {
		t.Errorf("SSE response leaked victim event type: %s", string(body))
	}
}

func TestStream_RejectsCrossUserSameTenant(t *testing.T) {
	store := &fakeChatStore{}
	store.seed(&AgentSession{
		ID:        "user7-stream",
		TenantID:  1,
		ProjectID: 42,
		CreatedBy: 7,
	})
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	h := NewHandlerForTest(nil, rdb, store)
	r := newDualStorageStreamRouter(h, 1, 8) // same tenant, user 8 instead of 7

	srv := httptest.NewServer(r)
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	req, _ := http.NewRequestWithContext(ctx, http.MethodGet,
		srv.URL+"/projects/42/agent/stream?session_id=user7-stream", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("expected 403 cross-user SSE access, got %d", resp.StatusCode)
	}
}

func TestStream_AllowsOwner(t *testing.T) {
	// Sanity: when the caller IS the owner, the stream proceeds normally.
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	_, _ = mr.XAdd("agent:stream:owned-sid", "*", []string{"type", "text_delta", "text", "ok"})

	store := &fakeChatStore{}
	store.seed(&AgentSession{
		ID:        "owned-sid",
		TenantID:  1,
		ProjectID: 42,
		CreatedBy: 7,
	})
	h := NewHandlerForTest(nil, rdb, store)
	r := newDualStorageStreamRouter(h, 1, 7) // owner

	srv := httptest.NewServer(r)
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	req, _ := http.NewRequestWithContext(ctx, http.MethodGet,
		srv.URL+"/projects/42/agent/stream?session_id=owned-sid", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 for owner, got %d", resp.StatusCode)
	}
	body, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(body), "event: agent") {
		t.Errorf("expected SSE event for owner, got: %s", string(body))
	}
}

func TestStream_LegacyNilStoreStillWorks(t *testing.T) {
	// Backward compat: when h.chat is nil (PG not configured / migration
	// 024 not run), Stream falls back to legacy unauthenticated behavior.
	// This is the dev/boot path; do not regress it.
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	_, _ = mr.XAdd("agent:stream:legacy-sid", "*", []string{"type", "tick"})

	h := NewHandlerForTest(nil, rdb, nil) // nil chat store
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/projects/:id/agent/stream", h.Stream)

	srv := httptest.NewServer(r)
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	req, _ := http.NewRequestWithContext(ctx, http.MethodGet,
		srv.URL+"/projects/42/agent/stream?session_id=legacy-sid", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("legacy nil-store path should still return 200, got %d", resp.StatusCode)
	}
}

// ---- Stream handler integration tests -------------------------------------

// newTestHandler wires a Handler against a miniredis instance and returns a
// gin router scoped to the stream route for httptest consumption.
func newTestHandler(t *testing.T) (*Handler, *miniredis.Miniredis, *redis.Client) {
	t.Helper()
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	h := NewHandler(nil, rdb, nil)
	return h, mr, rdb
}

func newStreamRouter(h *Handler) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/stream", h.Stream)
	return r
}

func TestStream_RequiresSessionID(t *testing.T) {
	h, _, _ := newTestHandler(t)
	r := newStreamRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/stream", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 without session_id, got %d", w.Code)
	}
}

func TestStream_DeliversRedisEntries(t *testing.T) {
	h, mr, _ := newTestHandler(t)

	// Seed Redis with a text_delta event BEFORE the stream opens.
	const sessionID = "test-session-1"
	streamKey := fmt.Sprintf("agent:stream:%s", sessionID)
	if _, err := mr.XAdd(streamKey, "*", []string{
		"type", "text_delta",
		"text", `hello "world"`,
	}); err != nil {
		t.Fatalf("XAdd failed: %v", err)
	}

	r := newStreamRouter(h)

	// Use a real HTTP server so the response is fully buffered for test
	// consumption. Client cancels after 2s so the handler exits cleanly.
	srv := httptest.NewServer(r)
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, srv.URL+"/stream?session_id="+sessionID, nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	// Read until ctx times out or we've seen the event.
	body, _ := io.ReadAll(resp.Body)
	got := string(body)

	if !strings.Contains(got, "event: agent") {
		t.Errorf("response missing 'event: agent' line; got:\n%s", got)
	}
	if !strings.Contains(got, `hello \"world\"`) {
		t.Errorf("response missing escaped text; got:\n%s", got)
	}
}

func TestStream_HeartbeatFiresUnderLoad(t *testing.T) {
	// This test pins the goroutine+channel refactor: the writer loop must
	// be able to fire heartbeats even while the reader goroutine is blocked
	// on XREAD. Prior to the refactor, the `select { default: XREAD }`
	// pattern never let the heartbeat case fire.
	//
	// We can't easily trigger the 15s heartbeat in a unit test without
	// time manipulation, so instead we verify that the handler returns
	// cleanly on ctx cancellation — a proxy for "the select loop is alive
	// and responsive", not stuck inside an uncancelable XREAD.
	h, _, _ := newTestHandler(t)
	r := newStreamRouter(h)
	srv := httptest.NewServer(r)
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, srv.URL+"/stream?session_id=no-data-session", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	start := time.Now()
	_, _ = io.Copy(io.Discard, resp.Body)
	elapsed := time.Since(start)

	// Must exit within ~1s of ctx timeout. If the handler is wedged in the
	// XREAD loop without a responsive select, this will hang >> the 500ms
	// ctx deadline.
	if elapsed > 1500*time.Millisecond {
		t.Errorf("handler took %v to exit after ctx cancel (expected < 1.5s)", elapsed)
	}
}

// countStreamReaders counts goroutines whose stack trace references the
// streamReader function. Direct identification is more robust than
// counting total goroutines because go-redis's connection pool and
// miniredis's per-connection servePeer goroutines inflate the total.
func countStreamReaders() int {
	buf := make([]byte, 1<<18)
	n := runtime.Stack(buf, true)
	trace := string(buf[:n])
	// Each goroutine block is separated by a blank line.
	blocks := strings.Split(trace, "\n\n")
	count := 0
	for _, block := range blocks {
		if strings.Contains(block, ".streamReader(") {
			count++
		}
	}
	return count
}

func TestStream_NoGoroutineLeakOnDisconnect(t *testing.T) {
	// Open N stream connections, cancel them all, verify the handler's
	// reader goroutines drain. The total process goroutine count is
	// unreliable because go-redis's connection pool and miniredis's
	// per-peer workers stay alive for pool lifetime — they're not leaks.
	// Instead we count goroutines whose stack contains streamReader.
	h, mr, _ := newTestHandler(t)
	r := newStreamRouter(h)
	srv := httptest.NewServer(r)
	defer srv.Close()

	client := &http.Client{
		Transport: &http.Transport{DisableKeepAlives: true},
	}
	defer client.CloseIdleConnections()

	const sessionPrefix = "leak-test-"
	for i := 0; i < 10; i++ {
		streamKey := fmt.Sprintf("agent:stream:%s%d", sessionPrefix, i)
		_, _ = mr.XAdd(streamKey, "*", []string{"type", "tick"})
	}

	if pre := countStreamReaders(); pre != 0 {
		t.Fatalf("expected 0 streamReader goroutines before test, got %d", pre)
	}

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
			defer cancel()
			req, _ := http.NewRequestWithContext(
				ctx, http.MethodGet,
				fmt.Sprintf("%s/stream?session_id=%s%d", srv.URL, sessionPrefix, idx),
				nil,
			)
			resp, err := client.Do(req)
			if err != nil {
				return
			}
			_, _ = io.Copy(io.Discard, resp.Body)
			resp.Body.Close()
		}(i)
	}
	wg.Wait()

	// XREAD block timeout is 1s, so readers can take up to 1s after ctx
	// cancel to see ctx.Err() and return. Wait 1.5s for safety.
	time.Sleep(1500 * time.Millisecond)
	runtime.GC()

	if after := countStreamReaders(); after != 0 {
		buf := make([]byte, 1<<16)
		n := runtime.Stack(buf, true)
		t.Errorf("streamReader goroutine leak: %d still alive after client disconnect\n%s",
			after, string(buf[:n]))
	}
}
