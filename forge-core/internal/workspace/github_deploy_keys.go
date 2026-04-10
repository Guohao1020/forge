package workspace

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// ErrGitHubAuthFailed indicates the PAT used for the deploy-key upload
// is invalid or lacks the admin:public_key scope. Callers should stop
// and surface this to a human operator -- retries will not help.
var ErrGitHubAuthFailed = errors.New("github deploy key upload: auth failed (check PAT scope admin:public_key)")

// githubUploader is the interface the state machine uses for deploy key
// uploads. Having it as an interface lets EnsureReady tests swap in a
// fake uploader without an HTTP server.
type githubUploader interface {
	Upload(ctx context.Context, token, owner, repo, title, sshKey string, readOnly bool) (int64, error)
}

// GitHubDeployKeyUploader uploads a deploy key to a GitHub repo via
// POST /repos/{owner}/{repo}/keys and returns the assigned key ID.
//
// Why not the broader internal/module/adapter/github package:
//   (a) this is a single narrow POST operation
//   (b) the workspace module should be independently testable without
//       dragging in the full adapter surface
//   (c) a thin interface is much easier to mock in EnsureReady unit tests
//
// The uploader handles:
//   - 2xx: success (return GitHub's key ID)
//   - 401/403: return ErrGitHubAuthFailed
//   - 422 "key already in use": treated as success (idempotent -- callers
//     fall back to the stored GitHubKeyID from the DB row if they need
//     the actual ID)
//   - 422 other: error
//   - 5xx: exponential backoff retry, 3 attempts total
//   - network error: wrapped and returned
type GitHubDeployKeyUploader struct {
	baseURL    string
	client     *http.Client
	maxRetries int
}

// NewGitHubDeployKeyUploader constructs an uploader that POSTs to baseURL.
// For production, baseURL is "https://api.github.com". For tests, pass
// an httptest.Server.URL.
func NewGitHubDeployKeyUploader(baseURL string) *GitHubDeployKeyUploader {
	return &GitHubDeployKeyUploader{
		baseURL:    baseURL,
		client:     &http.Client{Timeout: 30 * time.Second},
		maxRetries: 3,
	}
}

// Upload POSTs a deploy key and returns the GitHub-assigned key ID.
// This is called at most once per project (first EnsureReady call).
// `token` is a GitHub PAT with admin:public_key scope.
func (u *GitHubDeployKeyUploader) Upload(
	ctx context.Context,
	token, owner, repo, title, sshKey string,
	readOnly bool,
) (int64, error) {
	body := map[string]any{
		"title":     title,
		"key":       sshKey,
		"read_only": readOnly,
	}
	bodyBytes, err := json.Marshal(body)
	if err != nil {
		return 0, fmt.Errorf("marshal body: %w", err)
	}

	url := fmt.Sprintf("%s/repos/%s/%s/keys", u.baseURL, owner, repo)

	var lastErr error
	for attempt := 1; attempt <= u.maxRetries; attempt++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(bodyBytes))
		if err != nil {
			return 0, fmt.Errorf("new request: %w", err)
		}
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Accept", "application/vnd.github+json")
		req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
		req.Header.Set("Content-Type", "application/json")

		resp, err := u.client.Do(req)
		if err != nil {
			lastErr = fmt.Errorf("http do (attempt %d): %w", attempt, err)
			// Back off before next attempt
			if attempt < u.maxRetries {
				select {
				case <-ctx.Done():
					return 0, ctx.Err()
				case <-time.After(backoffDelay(attempt)):
				}
				continue
			}
			return 0, lastErr
		}

		respBody, _ := io.ReadAll(resp.Body)
		_ = resp.Body.Close()

		// 2xx success
		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			var parsed struct {
				ID int64 `json:"id"`
			}
			if err := json.Unmarshal(respBody, &parsed); err != nil {
				return 0, fmt.Errorf("decode response: %w", err)
			}
			if parsed.ID == 0 {
				return 0, fmt.Errorf("github returned id=0: %s", string(respBody))
			}
			return parsed.ID, nil
		}

		// 401/403 auth failure -- not retryable
		if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
			return 0, fmt.Errorf("%w: HTTP %d: %s", ErrGitHubAuthFailed, resp.StatusCode, string(respBody))
		}

		// 422 -- check for idempotent "already in use"
		if resp.StatusCode == http.StatusUnprocessableEntity {
			if isKeyAlreadyInUse(respBody) {
				// Idempotent success. We don't know the GitHub-assigned ID
				// from this response, but EnsureReady will look it up from
				// the DB row (which was presumably populated on a prior
				// successful upload).
				return 0, nil
			}
			return 0, fmt.Errorf("github deploy key upload: HTTP 422: %s", string(respBody))
		}

		// 5xx -- retryable
		if resp.StatusCode >= 500 && resp.StatusCode < 600 {
			lastErr = fmt.Errorf("github deploy key upload: HTTP %d (attempt %d): %s",
				resp.StatusCode, attempt, string(respBody))
			if attempt < u.maxRetries {
				select {
				case <-ctx.Done():
					return 0, ctx.Err()
				case <-time.After(backoffDelay(attempt)):
				}
				continue
			}
			return 0, lastErr
		}

		// Other 4xx -- not retryable, surface the error
		return 0, fmt.Errorf("github deploy key upload: HTTP %d: %s",
			resp.StatusCode, string(respBody))
	}

	return 0, lastErr
}

// isKeyAlreadyInUse returns true if the 422 response body indicates
// the key is already present on the repo -- GitHub's idempotent case.
func isKeyAlreadyInUse(body []byte) bool {
	// Cheap substring check -- avoids a full unmarshal for the common case.
	return strings.Contains(string(body), "already in use")
}

// backoffDelay returns an exponential backoff delay for the given
// attempt number. attempt=1 -> 500ms, 2 -> 1s, 3 -> 2s.
func backoffDelay(attempt int) time.Duration {
	base := 500 * time.Millisecond
	return base * time.Duration(1<<(attempt-1))
}
