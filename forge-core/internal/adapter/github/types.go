package github

import "time"

// Repository represents a GitHub repository.
type Repository struct {
	ID            int64     `json:"id"`
	Owner         string    `json:"owner"`
	Name          string    `json:"name"`
	FullName      string    `json:"full_name"`
	Description   string    `json:"description"`
	HTMLURL       string    `json:"html_url"`
	CloneURL      string    `json:"clone_url"`
	DefaultBranch string    `json:"default_branch"`
	Language      string    `json:"language"`
	Private       bool      `json:"private"`
	Fork          bool      `json:"fork"`
	StarCount     int       `json:"star_count"`
	UpdatedAt     time.Time `json:"updated_at"`
}

// OAuthTokenResponse represents the response from GitHub OAuth token exchange.
type OAuthTokenResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	Scope       string `json:"scope"`
	Error       string `json:"error,omitempty"`
	ErrorDesc   string `json:"error_description,omitempty"`
}

// GitHubUser represents a GitHub user profile.
type GitHubUser struct {
	ID        int64  `json:"id"`
	Login     string `json:"login"`
	Name      string `json:"name"`
	AvatarURL string `json:"avatar_url"`
	Email     string `json:"email"`
}

// FileChange represents a file to commit.
type FileChange struct {
	Path    string `json:"path"`
	Content string `json:"content"`
	Action  string `json:"action"` // "create" / "update" / "delete"
}

// PullRequestInfo represents a created PR.
type PullRequestInfo struct {
	Number  int    `json:"number"`
	HTMLURL string `json:"html_url"`
	Title   string `json:"title"`
	State   string `json:"state"`
	Head    string `json:"head"`
	Base    string `json:"base"`
}

// PRFile represents a changed file in a PR.
type PRFile struct {
	Filename  string `json:"filename"`
	Status    string `json:"status"` // "added" / "modified" / "removed"
	Additions int    `json:"additions"`
	Deletions int    `json:"deletions"`
	Patch     string `json:"patch"`
}

// Branch represents a GitHub branch.
type Branch struct {
	Name      string `json:"name"`
	SHA       string `json:"sha"`
	Protected bool   `json:"protected"`
}

// PullRequestSummary for listing PRs.
type PullRequestSummary struct {
	Number    int    `json:"number"`
	Title     string `json:"title"`
	State     string `json:"state"`
	HTMLURL   string `json:"html_url"`
	Head      string `json:"head"`
	Base      string `json:"base"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
	User      string `json:"user"`
}
