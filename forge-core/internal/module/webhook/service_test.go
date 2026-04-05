package webhook

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func TestMatchesEvent(t *testing.T) {
	tests := []struct {
		events string
		event  string
		want   bool
	}{
		{"*", "task.completed", true},
		{"task.completed,task.failed", "task.completed", true},
		{"task.completed,task.failed", "task.failed", true},
		{"task.completed,task.failed", "pr.created", false},
		{"task.completed", "task.completed", true},
		{"task.completed", "task.failed", false},
		{"", "anything", false},
	}

	for _, tt := range tests {
		got := matchesEvent(tt.events, tt.event)
		if got != tt.want {
			t.Errorf("matchesEvent(%q, %q) = %v, want %v", tt.events, tt.event, got, tt.want)
		}
	}
}

func TestSplitEvents(t *testing.T) {
	tests := []struct {
		input string
		want  int
	}{
		{"task.completed,task.failed,pr.created", 3},
		{"task.completed", 1},
		{"*", 1},
		{"", 0},
		{"a,b", 2},
	}

	for _, tt := range tests {
		got := splitEvents(tt.input)
		if len(got) != tt.want {
			t.Errorf("splitEvents(%q) = %d items, want %d", tt.input, len(got), tt.want)
		}
	}
}

func TestWebhookPayloadStructure(t *testing.T) {
	p := WebhookPayload{
		Event:     "task.completed",
		Timestamp: "2026-04-05T12:00:00Z",
		ProjectID: 42,
		Data:      map[string]string{"taskId": "1", "title": "Add auth"},
	}

	if p.Event != "task.completed" {
		t.Errorf("expected event task.completed, got %s", p.Event)
	}
	if p.ProjectID != 42 {
		t.Errorf("expected projectId 42, got %d", p.ProjectID)
	}
}

func TestWebhookDelivery_HMAC(t *testing.T) {
	secret := "test-webhook-secret"
	var receivedSig string
	var receivedBody []byte
	var receivedEvent string

	// Mock webhook endpoint
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedSig = r.Header.Get("X-Forge-Signature")
		receivedEvent = r.Header.Get("X-Forge-Event")
		receivedBody, _ = io.ReadAll(r.Body)
		w.WriteHeader(200)
	}))
	defer srv.Close()

	svc := &Service{client: &http.Client{Timeout: 5 * time.Second}}

	payload := WebhookPayload{
		Event:     "task.completed",
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		ProjectID: 1,
		Data:      map[string]string{"taskId": "42"},
	}
	body, _ := json.Marshal(payload)

	svc.deliver(srv.URL, secret, body, 1, "task.completed")

	// Verify signature
	if receivedSig == "" {
		t.Fatal("expected X-Forge-Signature header")
	}

	// Recompute expected signature
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	expected := "sha256=" + hex.EncodeToString(mac.Sum(nil))

	if receivedSig != expected {
		t.Errorf("signature mismatch:\n  got:  %s\n  want: %s", receivedSig, expected)
	}

	if receivedEvent != "task.completed" {
		t.Errorf("expected event task.completed, got %s", receivedEvent)
	}

	if len(receivedBody) == 0 {
		t.Error("expected non-empty body")
	}
}

func TestWebhookDelivery_NoSecret(t *testing.T) {
	var receivedSig string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedSig = r.Header.Get("X-Forge-Signature")
		w.WriteHeader(200)
	}))
	defer srv.Close()

	svc := &Service{client: &http.Client{Timeout: 5 * time.Second}}
	body := []byte(`{"event":"test"}`)

	svc.deliver(srv.URL, "", body, 1, "test")

	if receivedSig != "" {
		t.Error("should not have signature when no secret")
	}
}

func TestWebhookDelivery_ServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
	}))
	defer srv.Close()

	svc := &Service{client: &http.Client{Timeout: 5 * time.Second}}
	// Should not panic on server error
	svc.deliver(srv.URL, "", []byte(`{}`), 1, "test")
}

func TestWebhookDelivery_Unreachable(t *testing.T) {
	svc := &Service{client: &http.Client{Timeout: 1 * time.Second}}
	// Should not panic on unreachable URL
	svc.deliver("http://localhost:19999/nonexistent", "", []byte(`{}`), 1, "test")
}

func TestWebhookDelivery_Headers(t *testing.T) {
	var headers http.Header

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		headers = r.Header
		w.WriteHeader(200)
	}))
	defer srv.Close()

	svc := &Service{client: &http.Client{Timeout: 5 * time.Second}}
	svc.deliver(srv.URL, "", []byte(`{}`), 42, "task.failed")

	if headers.Get("Content-Type") != "application/json" {
		t.Error("expected Content-Type: application/json")
	}
	if headers.Get("X-Forge-Event") != "task.failed" {
		t.Error("expected X-Forge-Event: task.failed")
	}
	delivery := headers.Get("X-Forge-Delivery")
	if delivery == "" {
		t.Error("expected X-Forge-Delivery header")
	}
}

func TestWebhookDelivery_ConcurrentSafe(t *testing.T) {
	var count atomic.Int64

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count.Add(1)
		w.WriteHeader(200)
	}))
	defer srv.Close()

	svc := &Service{client: &http.Client{Timeout: 5 * time.Second}}

	for i := 0; i < 20; i++ {
		go svc.deliver(srv.URL, "secret", []byte(`{"test":true}`), int64(i), "test")
	}

	time.Sleep(2 * time.Second)

	if count.Load() != 20 {
		t.Errorf("expected 20 deliveries, got %d", count.Load())
	}
}
