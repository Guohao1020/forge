package github

import (
	"encoding/json"
	"testing"
	"time"
)

func TestRepositoryJSON(t *testing.T) {
	repo := Repository{
		ID:            12345,
		Owner:         "shulex",
		Name:          "forge",
		FullName:      "shulex/forge",
		Description:   "AI platform",
		HTMLURL:       "https://github.com/shulex/forge",
		CloneURL:      "https://github.com/shulex/forge.git",
		DefaultBranch: "main",
		Language:      "Go",
		Private:       true,
		StarCount:     42,
		UpdatedAt:     time.Now(),
	}

	data, err := json.Marshal(repo)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var parsed Repository
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if parsed.Owner != "shulex" {
		t.Errorf("expected owner shulex, got %s", parsed.Owner)
	}
	if parsed.Name != "forge" {
		t.Errorf("expected name forge, got %s", parsed.Name)
	}
	if !parsed.Private {
		t.Error("expected private=true")
	}
}

func TestOAuthTokenResponse(t *testing.T) {
	jsonStr := `{"access_token":"gho_test123","token_type":"bearer","scope":"repo,user"}`
	var resp OAuthTokenResponse
	if err := json.Unmarshal([]byte(jsonStr), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp.AccessToken != "gho_test123" {
		t.Errorf("expected token gho_test123, got %s", resp.AccessToken)
	}
	if resp.TokenType != "bearer" {
		t.Errorf("expected bearer, got %s", resp.TokenType)
	}
}

func TestOAuthTokenResponse_Error(t *testing.T) {
	jsonStr := `{"error":"bad_verification_code","error_description":"The code passed is incorrect"}`
	var resp OAuthTokenResponse
	json.Unmarshal([]byte(jsonStr), &resp)

	if resp.Error != "bad_verification_code" {
		t.Errorf("expected error code, got %s", resp.Error)
	}
}

func TestGitHubUser(t *testing.T) {
	user := GitHubUser{
		ID:    1001,
		Login: "harvey",
		Name:  "Harvey",
		Email: "harvey@example.com",
	}
	if user.Login != "harvey" {
		t.Errorf("expected login harvey, got %s", user.Login)
	}
}

func TestFileChange(t *testing.T) {
	fc := FileChange{
		Path:    "main.go",
		Content: "package main",
		Action:  "create",
	}
	if fc.Action != "create" {
		t.Errorf("expected action create, got %s", fc.Action)
	}
}

func TestPullRequestInfo(t *testing.T) {
	pr := PullRequestInfo{
		Number:  42,
		HTMLURL: "https://github.com/shulex/forge/pull/42",
		Title:   "Add auth feature",
		State:   "open",
		Head:    "feature/auth",
		Base:    "main",
	}
	if pr.Number != 42 {
		t.Errorf("expected PR #42, got %d", pr.Number)
	}
}

func TestBranch(t *testing.T) {
	b := Branch{
		Name:      "main",
		SHA:       "abc123def456",
		Protected: true,
	}
	if !b.Protected {
		t.Error("expected main to be protected")
	}
}

func TestNewClient(t *testing.T) {
	c := NewClient("test-token")
	if c == nil {
		t.Fatal("NewClient should not return nil")
	}
	if c.token != "test-token" {
		t.Errorf("expected token test-token, got %s", c.token)
	}
}
