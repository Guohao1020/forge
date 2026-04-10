package workspace

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// PrepClient calls ai-worker's /api/workspace/prep to install project
// dependencies inside the ai-worker container. This is non-blocking:
// failures are logged but do not prevent the workspace from becoming ready.
type PrepClient struct {
	baseURL    string
	httpClient *http.Client
}

func NewPrepClient(aiWorkerBaseURL string) *PrepClient {
	return &PrepClient{
		baseURL: aiWorkerBaseURL,
		httpClient: &http.Client{
			Timeout: 120 * time.Second, // dep install can be slow
		},
	}
}

type PrepRequest struct {
	TenantID      int64  `json:"tenant_id"`
	ProjectID     int64  `json:"project_id"`
	WorkspacePath string `json:"workspace_path"`
}

type PrepResponse struct {
	Status   string `json:"status"` // "ok" | "skipped" | "error"
	Language string `json:"language,omitempty"`
	Command  string `json:"command,omitempty"`
	Error    string `json:"error,omitempty"`
}

func (c *PrepClient) Prep(ctx context.Context, req PrepRequest) (*PrepResponse, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("prep: marshal: %w", err)
	}

	url := c.baseURL + "/api/workspace/prep"
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("prep: new request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("prep: http: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("prep: unexpected status %d", resp.StatusCode)
	}

	var result PrepResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("prep: decode response: %w", err)
	}
	return &result, nil
}
