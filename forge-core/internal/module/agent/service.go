package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/shulex/forge/forge-core/internal/workspace"
)

// Service handles communication with the Python AI worker.
type Service struct {
	aiWorkerURL string
	httpClient  *http.Client
	wsManager   *workspace.Manager // nilable; when nil, workspace_path is always empty
}

// NewService creates a new agent service.
// wsManager may be nil — in that case, workspace_path is always empty
// and the ai-worker falls back to the QueryEngine chat path.
func NewService(aiWorkerURL string, wsManager *workspace.Manager) *Service {
	return &Service{
		aiWorkerURL: aiWorkerURL,
		httpClient: &http.Client{
			Timeout: 10 * time.Second, // Only for the initial POST (fire-and-forget)
		},
		wsManager: wsManager,
	}
}

// aiRunRequest is the request body sent to the Python AI worker.
type aiRunRequest struct {
	SessionID     string `json:"session_id,omitempty"`
	ProjectID     int64  `json:"project_id"`
	WorkspacePath string `json:"workspace_path,omitempty"`
	Message       string `json:"message"`
	Model         string `json:"model,omitempty"`
	SystemPrompt  string `json:"system_prompt,omitempty"`
	CorrelationID string `json:"correlation_id,omitempty"`
}

// aiRunResponse is the response from the Python AI worker.
type aiRunResponse struct {
	SessionID     string `json:"session_id"`
	Status        string `json:"status"`
	CorrelationID string `json:"correlation_id"`
}

// SubmitMessage sends a message to the AI worker (fire-and-forget).
// The AI worker runs the QueryEngine asynchronously and publishes events to Redis.
//
// tenantID is used together with projectID to compute the workspace_path
// fragment that lets ai-worker route to the pair_pipeline when the repo
// has been cloned. tenantID=0 is a sentinel meaning "caller does not
// know the tenant" (legacy fallback path) — workspace_path stays empty
// and ai-worker uses the legacy QueryEngine chat path.
func (s *Service) SubmitMessage(ctx context.Context, tenantID, projectID int64, req ChatRequest) (*ChatResponse, error) {
	slog.Info("agent.session_start",
		"event", "agent.session_start",
		"session_id", req.SessionID,
		"tenant_id", tenantID,
		"project_id", projectID,
	)

	body := aiRunRequest{
		SessionID:     req.SessionID,
		ProjectID:     projectID,
		Message:       req.Message,
		Model:         req.Model,
		SystemPrompt:  req.SystemPrompt,
		CorrelationID: req.CorrelationID,
	}

	// Ensure the workspace is ready before we dispatch to ai-worker.
	// A new session (empty SessionID) triggers a fetch+reset so the
	// agent starts from clean main; otherwise we reuse the existing
	// state so multi-turn edits persist across messages. Per spec §2.7.
	if s.wsManager != nil && tenantID > 0 {
		isNewSession := req.SessionID == ""
		ws, err := s.wsManager.EnsureReady(ctx, tenantID, projectID, isNewSession)
		if err != nil {
			// Workspace setup failed — log warning and continue without
			// workspace_path. ai-worker will use a temp directory (dev
			// fallback). In production, this should be a hard error.
			slog.Warn("workspace not ready, continuing without workspace",
				"error", err,
				"tenant_id", tenantID,
				"project_id", projectID,
			)
		} else {
			body.WorkspacePath = fmt.Sprintf("tenant-%d/project-%d/repo", tenantID, projectID)
			_ = ws // ws.HostPath available if ever needed
		}
	}

	jsonBody, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	url := fmt.Sprintf("%s/api/run", s.aiWorkerURL)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := s.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("call ai-worker: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("ai-worker returned %d: %s", resp.StatusCode, string(respBody))
	}

	var aiResp aiRunResponse
	if err := json.NewDecoder(resp.Body).Decode(&aiResp); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	slog.Info("agent message submitted",
		"session_id", aiResp.SessionID,
		"correlation_id", aiResp.CorrelationID,
		"project_id", projectID,
		"workspace_path", body.WorkspacePath,
	)

	return &ChatResponse{
		SessionID:     aiResp.SessionID,
		Status:        aiResp.Status,
		CorrelationID: aiResp.CorrelationID,
	}, nil
}
