package github

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	ghlib "github.com/google/go-github/v63/github"
	"golang.org/x/oauth2"
)

// Client wraps the GitHub API for Forge operations.
type Client struct {
	token  string
	client *ghlib.Client
}

// NewClient creates a GitHub API client with the given access token.
func NewClient(token string) *Client {
	ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: token})
	tc := oauth2.NewClient(context.Background(), ts)
	return &Client{
		token:  token,
		client: ghlib.NewClient(tc),
	}
}

// ExchangeCode exchanges an OAuth authorization code for an access token.
func ExchangeCode(ctx context.Context, clientID, clientSecret, code string) (*OAuthTokenResponse, error) {
	data := url.Values{
		"client_id":     {clientID},
		"client_secret": {clientSecret},
		"code":          {code},
	}

	req, err := http.NewRequestWithContext(ctx, "POST", "https://github.com/login/oauth/access_token", strings.NewReader(data.Encode()))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("exchange code: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	var tokenResp OAuthTokenResponse
	if err := json.Unmarshal(body, &tokenResp); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}

	if tokenResp.Error != "" {
		return nil, fmt.Errorf("github oauth error: %s - %s", tokenResp.Error, tokenResp.ErrorDesc)
	}

	if tokenResp.AccessToken == "" {
		return nil, fmt.Errorf("empty access token in response")
	}

	return &tokenResp, nil
}

// GetAuthenticatedUser returns the authenticated GitHub user.
func (c *Client) GetAuthenticatedUser(ctx context.Context) (*GitHubUser, error) {
	user, resp, err := c.client.Users.Get(ctx, "")
	if err != nil {
		return nil, fmt.Errorf("get authenticated user: %w", err)
	}
	c.logRateLimit(resp)

	return &GitHubUser{
		ID:        user.GetID(),
		Login:     user.GetLogin(),
		Name:      user.GetName(),
		AvatarURL: user.GetAvatarURL(),
		Email:     user.GetEmail(),
	}, nil
}

// ListRepos returns all repositories accessible to the authenticated user.
func (c *Client) ListRepos(ctx context.Context) ([]Repository, error) {
	var allRepos []Repository

	opts := &ghlib.RepositoryListByAuthenticatedUserOptions{
		Sort:      "updated",
		Direction: "desc",
		ListOptions: ghlib.ListOptions{
			PerPage: 100,
		},
	}

	for {
		repos, resp, err := c.client.Repositories.ListByAuthenticatedUser(ctx, opts)
		if err != nil {
			return nil, fmt.Errorf("list repos: %w", err)
		}
		c.logRateLimit(resp)

		for _, r := range repos {
			allRepos = append(allRepos, repoFromGitHub(r))
		}

		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}

	return allRepos, nil
}

// GetRepo returns a single repository by owner and name.
func (c *Client) GetRepo(ctx context.Context, owner, repo string) (*Repository, error) {
	r, resp, err := c.client.Repositories.Get(ctx, owner, repo)
	if err != nil {
		return nil, fmt.Errorf("get repo %s/%s: %w", owner, repo, err)
	}
	c.logRateLimit(resp)

	result := repoFromGitHub(r)
	return &result, nil
}

func (c *Client) logRateLimit(resp *ghlib.Response) {
	if resp == nil || resp.Response == nil {
		return
	}
	remaining := resp.Header.Get("X-RateLimit-Remaining")
	limit := resp.Header.Get("X-RateLimit-Limit")
	resetStr := resp.Header.Get("X-RateLimit-Reset")
	remainingInt, _ := strconv.Atoi(remaining)
	if remainingInt < 100 {
		resetUnix, _ := strconv.ParseInt(resetStr, 10, 64)
		resetTime := time.Unix(resetUnix, 0)
		slog.Warn("github rate limit low",
			"remaining", remaining,
			"limit", limit,
			"reset_at", resetTime.Format(time.RFC3339),
		)
	}
}

func repoFromGitHub(r *ghlib.Repository) Repository {
	return Repository{
		ID:            r.GetID(),
		Owner:         r.GetOwner().GetLogin(),
		Name:          r.GetName(),
		FullName:      r.GetFullName(),
		Description:   r.GetDescription(),
		HTMLURL:       r.GetHTMLURL(),
		CloneURL:      r.GetCloneURL(),
		DefaultBranch: r.GetDefaultBranch(),
		Language:      r.GetLanguage(),
		Private:       r.GetPrivate(),
		Fork:          r.GetFork(),
		StarCount:     r.GetStargazersCount(),
		UpdatedAt:     r.GetUpdatedAt().Time,
	}
}
