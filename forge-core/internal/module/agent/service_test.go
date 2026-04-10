package agent

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/shulex/forge/forge-core/internal/workspace"
)

// captureAIWorker returns an httptest server that records the last
// aiRunRequest it received. Useful for asserting what forge-core sent.
func captureAIWorker(t *testing.T) (*httptest.Server, *aiRunRequest) {
	t.Helper()
	captured := &aiRunRequest{}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/run" {
			http.NotFound(w, r)
			return
		}
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, captured)
		resp := aiRunResponse{
			SessionID:     "sid-captured",
			Status:        "accepted",
			CorrelationID: "corr-captured",
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	t.Cleanup(srv.Close)
	return srv, captured
}

// TestSubmitMessage_WorkspaceNotReady_ReturnsError asserts that when
// wsManager is non-nil but partially wired (no stateRepo/gitRunner/
// projectLookup), EnsureReady fails and SubmitMessage propagates the
// error instead of silently sending an empty workspace_path.
//
// This is the new behavior after migrating from os.Stat(.git) to
// EnsureReady — failure is fast and visible, not silent.
func TestSubmitMessage_WorkspaceNotReady_ReturnsError(t *testing.T) {
	root := t.TempDir()

	// Manager with no deps — EnsureReady will return a descriptive error
	wsManager := workspace.NewManager(workspace.Config{Root: root})
	srv, _ := captureAIWorker(t)

	svc := NewService(srv.URL, wsManager)
	req := ChatRequest{Message: "hello"}

	_, err := svc.SubmitMessage(context.Background(), 1, 42, req)
	if err == nil {
		t.Fatal("expected error from SubmitMessage when workspace deps are nil")
	}
}

// TestSubmitMessage_EmptyWorkspacePath_WhenWsManagerNil asserts that
// nil wsManager is tolerated (legacy dev boot, handler_test fixtures)
// and produces an empty workspace_path.
func TestSubmitMessage_EmptyWorkspacePath_WhenWsManagerNil(t *testing.T) {
	srv, captured := captureAIWorker(t)

	svc := NewService(srv.URL, nil)
	req := ChatRequest{Message: "hello"}

	_, err := svc.SubmitMessage(context.Background(), 1, 42, req)
	if err != nil {
		t.Fatalf("SubmitMessage: %v", err)
	}

	if captured.WorkspacePath != "" {
		t.Errorf("WorkspacePath = %q, want empty (nil wsManager)", captured.WorkspacePath)
	}
}

// TestSubmitMessage_EmptyWorkspacePath_WhenTenantZero asserts that
// when the caller does not know the tenant (legacy Chat fallback
// path in handler.go), tenantID=0 produces an empty workspace_path.
// We must NOT synthesize a tenant-0 directory lookup.
func TestSubmitMessage_EmptyWorkspacePath_WhenTenantZero(t *testing.T) {
	srv, captured := captureAIWorker(t)

	// Even with a partially wired manager, tenantID=0 skips EnsureReady
	wsManager := workspace.NewManager(workspace.Config{Root: t.TempDir()})
	svc := NewService(srv.URL, wsManager)
	req := ChatRequest{Message: "hello"}

	_, err := svc.SubmitMessage(context.Background(), 0, 42, req)
	if err != nil {
		t.Fatalf("SubmitMessage: %v", err)
	}

	if captured.WorkspacePath != "" {
		t.Errorf("WorkspacePath = %q, want empty (tenantID=0 sentinel)", captured.WorkspacePath)
	}
}
