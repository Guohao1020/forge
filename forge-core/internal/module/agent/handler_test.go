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
