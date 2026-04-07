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
)

// Service handles communication with the Python AI worker.
type Service struct {
	aiWorkerURL string
	httpClient  *http.Client
}

// NewService creates a new agent service.
func NewService(aiWorkerURL string) *Service {
	return &Service{
		aiWorkerURL: aiWorkerURL,
		httpClient: &http.Client{
			Timeout: 10 * time.Second, // Only for the initial POST (fire-and-forget)
		},
	}
}

// aiRunRequest is the request body sent to the Python AI worker.
type aiRunRequest struct {
	SessionID     string `json:"session_id,omitempty"`
	ProjectID     int64  `json:"project_id"`
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
func (s *Service) SubmitMessage(ctx context.Context, projectID int64, req ChatRequest) (*ChatResponse, error) {
	body := aiRunRequest{
		SessionID:     req.SessionID,
		ProjectID:     projectID,
		Message:       req.Message,
		Model:         req.Model,
		SystemPrompt:  req.SystemPrompt,
		CorrelationID: req.CorrelationID,
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
	)

	return &ChatResponse{
		SessionID:     aiResp.SessionID,
		Status:        aiResp.Status,
		CorrelationID: aiResp.CorrelationID,
	}, nil
}
