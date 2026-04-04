package forgeapi

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"
)

// Client communicates with forge-core API.
type Client struct {
	baseURL    string
	httpClient *http.Client
	token      string // JWT token for authenticated calls
}

func NewClient(baseURL string) *Client {
	return &Client{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// SetToken sets the JWT auth token for API calls.
func (c *Client) SetToken(token string) {
	c.token = token
}

// Login authenticates with forge-core and stores the JWT.
func (c *Client) Login(username, password string) error {
	body := map[string]string{
		"username": username,
		"password": password,
	}
	data, _ := json.Marshal(body)

	resp, err := c.httpClient.Post(c.baseURL+"/api/auth/login", "application/json", bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("login request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("login failed: status %d", resp.StatusCode)
	}

	var result struct {
		Data struct {
			Token string `json:"token"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("parse login response: %w", err)
	}

	c.token = result.Data.Token
	slog.Info("forge-core login successful")
	return nil
}

// CreateTask creates a new task in a project.
func (c *Client) CreateTask(projectID int64, title, description string) (int64, error) {
	body := map[string]string{
		"title":       title,
		"description": description,
	}
	data, _ := json.Marshal(body)

	resp, err := c.doRequest("POST", fmt.Sprintf("/api/projects/%d/tasks", projectID), data)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	var result struct {
		Data struct {
			ID int64 `json:"id"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return 0, fmt.Errorf("parse create task response: %w", err)
	}
	return result.Data.ID, nil
}

// SendMessage sends a message to a task conversation.
func (c *Client) SendMessage(projectID, taskID int64, content string) (string, error) {
	body := map[string]string{
		"content": content,
	}
	data, _ := json.Marshal(body)

	resp, err := c.doRequest("POST", fmt.Sprintf("/api/projects/%d/tasks/%d/messages", projectID, taskID), data)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	return string(respBody), nil
}

// ListProjects lists all projects.
func (c *Client) ListProjects() ([]map[string]interface{}, error) {
	resp, err := c.doRequest("GET", "/api/projects", nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result struct {
		Data struct {
			Projects []map[string]interface{} `json:"projects"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	return result.Data.Projects, nil
}

func (c *Client) doRequest(method, path string, body []byte) (*http.Response, error) {
	var bodyReader io.Reader
	if body != nil {
		bodyReader = bytes.NewReader(body)
	}

	req, err := http.NewRequest(method, c.baseURL+path, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}

	if resp.StatusCode >= 400 {
		respBody, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return nil, fmt.Errorf("API error: status=%d body=%s", resp.StatusCode, string(respBody))
	}

	return resp, nil
}
