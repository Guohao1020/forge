package agent

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
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

// TestSubmitMessage_PassesRelativeWorkspacePath_WhenRepoExists asserts
// that when wsManager is non-nil, tenantID is positive, and the resolved
// ProjectDir contains a .git directory, SubmitMessage sends the
// workspace_path as a RELATIVE fragment (per the protocol amendment
// so forge-core on the host and ai-worker in its container can join
// the same fragment against their own FORGE_WORKSPACE_ROOT env var).
func TestSubmitMessage_PassesRelativeWorkspacePath_WhenRepoExists(t *testing.T) {
	// Create a fake workspace root with a .git marker
	root := t.TempDir()
	projectDir := filepath.Join(root, "tenant-1", "project-42", "repo")
	if err := os.MkdirAll(filepath.Join(projectDir, ".git"), 0o755); err != nil {
		t.Fatalf("mkdir .git: %v", err)
	}

	wsManager := workspace.NewManager(workspace.Config{Root: root})
	srv, captured := captureAIWorker(t)

	svc := NewService(srv.URL, wsManager)
	req := ChatRequest{Message: "hello"}

	resp, err := svc.SubmitMessage(context.Background(), 1, 42, req)
	if err != nil {
		t.Fatalf("SubmitMessage: %v", err)
	}
	if resp == nil {
		t.Fatalf("nil resp")
	}

	// The key assertion: workspace_path must be the RELATIVE fragment,
	// not the absolute host path. If this ever becomes absolute,
	// the ai-worker side (which joins with /data/forge/workspaces)
	// will produce a path that doesn't exist inside the container.
	want := "tenant-1/project-42/repo"
	if captured.WorkspacePath != want {
		t.Errorf("WorkspacePath = %q, want %q", captured.WorkspacePath, want)
	}
}

// TestSubmitMessage_EmptyWorkspacePath_WhenRepoMissing asserts the
// fallback: when the project has not been cloned (no .git), we send
// an empty workspace_path so the ai-worker falls back to QueryEngine.
func TestSubmitMessage_EmptyWorkspacePath_WhenRepoMissing(t *testing.T) {
	root := t.TempDir() // empty — no tenant-1/project-42/repo at all

	wsManager := workspace.NewManager(workspace.Config{Root: root})
	srv, captured := captureAIWorker(t)

	svc := NewService(srv.URL, wsManager)
	req := ChatRequest{Message: "hello"}

	_, err := svc.SubmitMessage(context.Background(), 1, 42, req)
	if err != nil {
		t.Fatalf("SubmitMessage: %v", err)
	}

	if captured.WorkspacePath != "" {
		t.Errorf("WorkspacePath = %q, want empty string (project not cloned)", captured.WorkspacePath)
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
	root := t.TempDir()
	// Even if some directory happens to exist at tenant-0/..., we
	// must NOT use it — tenantID=0 is a sentinel meaning "unknown".
	projectDir := filepath.Join(root, "tenant-0", "project-42", "repo")
	_ = os.MkdirAll(filepath.Join(projectDir, ".git"), 0o755)

	wsManager := workspace.NewManager(workspace.Config{Root: root})
	srv, captured := captureAIWorker(t)

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
