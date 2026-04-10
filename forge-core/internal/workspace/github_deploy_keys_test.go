package workspace

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

func TestUploadDeployKey_Success(t *testing.T) {
	var receivedBody map[string]any
	var receivedAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method: want POST, got %s", r.Method)
		}
		if r.URL.Path != "/repos/owner/repo/keys" {
			t.Errorf("path: got %s", r.URL.Path)
		}
		receivedAuth = r.Header.Get("Authorization")
		_ = json.NewDecoder(r.Body).Decode(&receivedBody)
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"id": 42, "title": "forge-test", "key": "ssh-ed25519 AAAA"}`))
	}))
	defer srv.Close()

	uploader := NewGitHubDeployKeyUploader(srv.URL)
	ctx := context.Background()

	id, err := uploader.Upload(ctx, "ghp_test_token", "owner", "repo",
		"forge-test-title", "ssh-ed25519 AAAA forge-test", false)
	if err != nil {
		t.Fatalf("Upload: %v", err)
	}
	if id != 42 {
		t.Errorf("id: want 42, got %d", id)
	}
	if !strings.HasPrefix(receivedAuth, "Bearer ghp_test_token") {
		t.Errorf("auth header: got %q", receivedAuth)
	}
	if receivedBody["title"] != "forge-test-title" {
		t.Errorf("title: got %v", receivedBody["title"])
	}
	if receivedBody["read_only"] != false {
		t.Errorf("read_only: got %v (want false -- forge agents may push)", receivedBody["read_only"])
	}
	key, _ := receivedBody["key"].(string)
	if !strings.HasPrefix(key, "ssh-ed25519") {
		t.Errorf("key: got %v", receivedBody["key"])
	}
}

func TestUploadDeployKey_422Idempotent(t *testing.T) {
	// GitHub returns 422 when the same key has already been uploaded.
	// We treat this as success since it means the key is already present.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnprocessableEntity)
		_, _ = w.Write([]byte(`{"message":"Validation Failed","errors":[{"resource":"PublicKey","code":"custom","field":"key","message":"key is already in use"}]}`))
	}))
	defer srv.Close()

	uploader := NewGitHubDeployKeyUploader(srv.URL)
	id, err := uploader.Upload(context.Background(), "t", "o", "r", "title", "ssh-ed25519 AAAA", false)
	if err != nil {
		t.Fatalf("Upload with 422 'already in use' should be idempotent, got err: %v", err)
	}
	if id != 0 {
		t.Errorf("expected id=0 (unknown) for idempotent path, got %d", id)
	}
}

func TestUploadDeployKey_422OtherErrorReturnsError(t *testing.T) {
	// A 422 that's not "key already in use" is still an error.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnprocessableEntity)
		_, _ = w.Write([]byte(`{"message":"Validation Failed","errors":[{"code":"missing_field","field":"key"}]}`))
	}))
	defer srv.Close()

	uploader := NewGitHubDeployKeyUploader(srv.URL)
	_, err := uploader.Upload(context.Background(), "t", "o", "r", "title", "", false)
	if err == nil {
		t.Fatal("expected error for 422 missing_field")
	}
}

func TestUploadDeployKey_401ReturnsAuthFailed(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"message":"Bad credentials"}`))
	}))
	defer srv.Close()

	uploader := NewGitHubDeployKeyUploader(srv.URL)
	_, err := uploader.Upload(context.Background(), "bad", "o", "r", "title", "key", false)
	if err == nil {
		t.Fatal("expected error for 401")
	}
	if !strings.Contains(err.Error(), "401") && !strings.Contains(err.Error(), "auth") {
		t.Errorf("error should mention 401 or auth: %v", err)
	}
}

func TestUploadDeployKey_5xxRetrySucceeds(t *testing.T) {
	// First two calls return 500, third returns 201. Upload should
	// succeed after retries.
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt32(&calls, 1)
		if n < 3 {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte("transient"))
			return
		}
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"id": 99}`))
	}))
	defer srv.Close()

	uploader := NewGitHubDeployKeyUploader(srv.URL)
	id, err := uploader.Upload(context.Background(), "t", "o", "r", "title", "key", false)
	if err != nil {
		t.Fatalf("Upload should succeed after 2 retries: %v", err)
	}
	if id != 99 {
		t.Errorf("id: want 99, got %d", id)
	}
	if atomic.LoadInt32(&calls) != 3 {
		t.Errorf("call count: want 3, got %d", atomic.LoadInt32(&calls))
	}
}

func TestUploadDeployKey_5xxRetryExhausted(t *testing.T) {
	// Every call returns 500. After N attempts, give up with an error.
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = fmt.Fprintf(w, "persistent server failure")
	}))
	defer srv.Close()

	uploader := NewGitHubDeployKeyUploader(srv.URL)
	_, err := uploader.Upload(context.Background(), "t", "o", "r", "title", "key", false)
	if err == nil {
		t.Fatal("expected error after retry exhaustion")
	}
	n := atomic.LoadInt32(&calls)
	if n < 3 {
		t.Errorf("expected at least 3 attempts, got %d", n)
	}
}

func TestUploadDeployKey_NetworkErrorReturnsError(t *testing.T) {
	// Point at a closed port -- connection refused
	uploader := NewGitHubDeployKeyUploader("http://127.0.0.1:1")
	_, err := uploader.Upload(context.Background(), "t", "o", "r", "title", "ssh-ed25519 AAAA", false)
	if err == nil {
		t.Fatal("expected error for connection refused")
	}
}
