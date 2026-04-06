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

// CreateBranch creates a new branch from the given source ref. If the branch
// already exists (HTTP 422), the call is treated as a success (idempotent).
func (c *Client) CreateBranch(ctx context.Context, owner, repo, branchName, fromRef string) error {
	srcRef, resp, err := c.client.Git.GetRef(ctx, owner, repo, "refs/heads/"+fromRef)
	if err != nil {
		return fmt.Errorf("get ref %s: %w", fromRef, err)
	}
	c.logRateLimit(resp)

	newRef := &ghlib.Reference{
		Ref:    ghlib.String("refs/heads/" + branchName),
		Object: &ghlib.GitObject{SHA: srcRef.Object.SHA},
	}

	_, resp, err = c.client.Git.CreateRef(ctx, owner, repo, newRef)
	if err != nil {
		// 422 means the ref already exists — treat as success (idempotent)
		if resp != nil && resp.StatusCode == http.StatusUnprocessableEntity {
			slog.Warn("branch already exists, treating as success",
				"owner", owner, "repo", repo, "branch", branchName)
			return nil
		}
		return fmt.Errorf("create branch %s: %w", branchName, err)
	}
	c.logRateLimit(resp)

	return nil
}

// CommitFiles commits a batch of file changes to a branch using the Git Trees API.
func (c *Client) CommitFiles(ctx context.Context, owner, repo, branch, message string, files []FileChange) error {
	// 1. Get branch ref
	ref, resp, err := c.client.Git.GetRef(ctx, owner, repo, "refs/heads/"+branch)
	if err != nil {
		return fmt.Errorf("get branch ref %s: %w", branch, err)
	}
	c.logRateLimit(resp)

	parentSHA := ref.Object.GetSHA()

	// 2. Get the commit to find the base tree
	parentCommit, resp, err := c.client.Git.GetCommit(ctx, owner, repo, parentSHA)
	if err != nil {
		return fmt.Errorf("get commit %s: %w", parentSHA, err)
	}
	c.logRateLimit(resp)

	baseTreeSHA := parentCommit.Tree.GetSHA()

	// 3. Build tree entries — create blobs for create/update, nil SHA for delete
	var entries []*ghlib.TreeEntry
	for _, f := range files {
		if f.Action == "delete" {
			entries = append(entries, &ghlib.TreeEntry{
				Path: ghlib.String(f.Path),
				Mode: ghlib.String("100644"),
				Type: ghlib.String("blob"),
				// Omitting SHA signals deletion from the tree
			})
			continue
		}

		blob, resp, err := c.client.Git.CreateBlob(ctx, owner, repo, &ghlib.Blob{
			Content:  ghlib.String(f.Content),
			Encoding: ghlib.String("utf-8"),
		})
		if err != nil {
			return fmt.Errorf("create blob for %s: %w", f.Path, err)
		}
		c.logRateLimit(resp)

		entries = append(entries, &ghlib.TreeEntry{
			Path: ghlib.String(f.Path),
			Mode: ghlib.String("100644"),
			Type: ghlib.String("blob"),
			SHA:  blob.SHA,
		})
	}

	// 4. Create new tree
	tree, resp, err := c.client.Git.CreateTree(ctx, owner, repo, baseTreeSHA, entries)
	if err != nil {
		return fmt.Errorf("create tree: %w", err)
	}
	c.logRateLimit(resp)

	// 5. Create commit
	commit, resp, err := c.client.Git.CreateCommit(ctx, owner, repo, &ghlib.Commit{
		Message: ghlib.String(message),
		Tree:    tree,
		Parents: []*ghlib.Commit{{SHA: ghlib.String(parentSHA)}},
	}, nil)
	if err != nil {
		return fmt.Errorf("create commit: %w", err)
	}
	c.logRateLimit(resp)

	// 6. Update branch ref to point to new commit
	ref.Object.SHA = commit.SHA
	_, resp, err = c.client.Git.UpdateRef(ctx, owner, repo, ref, false)
	if err != nil {
		return fmt.Errorf("update ref: %w", err)
	}
	c.logRateLimit(resp)

	return nil
}

// CreatePR creates a pull request and returns its metadata.
func (c *Client) CreatePR(ctx context.Context, owner, repo, title, body, head, base string) (*PullRequestInfo, error) {
	pr, resp, err := c.client.PullRequests.Create(ctx, owner, repo, &ghlib.NewPullRequest{
		Title: ghlib.String(title),
		Body:  ghlib.String(body),
		Head:  ghlib.String(head),
		Base:  ghlib.String(base),
	})
	if err != nil {
		return nil, fmt.Errorf("create PR: %w", err)
	}
	c.logRateLimit(resp)

	return &PullRequestInfo{
		Number:  pr.GetNumber(),
		HTMLURL: pr.GetHTMLURL(),
		Title:   pr.GetTitle(),
		State:   pr.GetState(),
		Head:    pr.GetHead().GetRef(),
		Base:    pr.GetBase().GetRef(),
	}, nil
}

// MergePR merges a pull request using the merge (not squash/rebase) strategy.
func (c *Client) MergePR(ctx context.Context, owner, repo string, prNumber int, commitMessage string) error {
	_, _, err := c.client.PullRequests.Merge(ctx, owner, repo, prNumber, commitMessage, &ghlib.PullRequestOptions{
		MergeMethod: "merge",
	})
	if err != nil {
		return fmt.Errorf("merge PR #%d: %w", prNumber, err)
	}
	slog.Info("PR merged", "owner", owner, "repo", repo, "pr", prNumber)
	return nil
}

// GetPRFiles returns the list of changed files in a pull request.
func (c *Client) GetPRFiles(ctx context.Context, owner, repo string, prNumber int) ([]PRFile, error) {
	files, resp, err := c.client.PullRequests.ListFiles(ctx, owner, repo, prNumber, nil)
	if err != nil {
		return nil, fmt.Errorf("list PR files for #%d: %w", prNumber, err)
	}
	c.logRateLimit(resp)

	result := make([]PRFile, 0, len(files))
	for _, f := range files {
		result = append(result, PRFile{
			Filename:  f.GetFilename(),
			Status:    f.GetStatus(),
			Additions: f.GetAdditions(),
			Deletions: f.GetDeletions(),
			Patch:     f.GetPatch(),
		})
	}

	return result, nil
}

// GetFileContent returns the decoded text content of a file at the given ref.
func (c *Client) GetFileContent(ctx context.Context, owner, repo, path, ref string) (string, error) {
	fileContent, _, resp, err := c.client.Repositories.GetContents(ctx, owner, repo, path, &ghlib.RepositoryContentGetOptions{
		Ref: ref,
	})
	if err != nil {
		return "", fmt.Errorf("get file %s@%s: %w", path, ref, err)
	}
	c.logRateLimit(resp)

	if fileContent == nil {
		return "", fmt.Errorf("path %s is a directory, not a file", path)
	}

	content, err := fileContent.GetContent()
	if err != nil {
		return "", fmt.Errorf("decode content for %s: %w", path, err)
	}

	return content, nil
}

// GetRepoLanguages returns language breakdown (bytes per language).
func (c *Client) GetRepoLanguages(ctx context.Context, owner, repo string) (map[string]int, error) {
	languages, resp, err := c.client.Repositories.ListLanguages(ctx, owner, repo)
	if resp != nil {
		c.logRateLimit(resp)
	}
	if err != nil {
		return nil, fmt.Errorf("list languages: %w", err)
	}
	return languages, nil
}

// GetTree returns repository file tree (used for tech stack detection).
func (c *Client) GetTree(ctx context.Context, owner, repo, ref string) ([]string, error) {
	tree, resp, err := c.client.Git.GetTree(ctx, owner, repo, ref, true)
	if resp != nil {
		c.logRateLimit(resp)
	}
	if err != nil {
		return nil, fmt.Errorf("get tree: %w", err)
	}
	paths := make([]string, 0, len(tree.Entries))
	for _, e := range tree.Entries {
		if e.Path != nil {
			paths = append(paths, *e.Path)
		}
	}
	return paths, nil
}

// ListBranches returns all branches for a repository.
func (c *Client) ListBranches(ctx context.Context, owner, repo string) ([]Branch, error) {
	var allBranches []Branch
	opts := &ghlib.BranchListOptions{ListOptions: ghlib.ListOptions{PerPage: 100}}
	for {
		branches, resp, err := c.client.Repositories.ListBranches(ctx, owner, repo, opts)
		if resp != nil {
			c.logRateLimit(resp)
		}
		if err != nil {
			return nil, fmt.Errorf("list branches: %w", err)
		}
		for _, b := range branches {
			allBranches = append(allBranches, Branch{
				Name:      b.GetName(),
				SHA:       b.GetCommit().GetSHA(),
				Protected: b.GetProtected(),
			})
		}
		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}
	return allBranches, nil
}

// ListPRs returns pull requests for a repository.
func (c *Client) ListPRs(ctx context.Context, owner, repo, state string) ([]PullRequestSummary, error) {
	if state == "" {
		state = "open"
	}
	opts := &ghlib.PullRequestListOptions{
		State:       state,
		Sort:        "updated",
		Direction:   "desc",
		ListOptions: ghlib.ListOptions{PerPage: 30},
	}
	prs, resp, err := c.client.PullRequests.List(ctx, owner, repo, opts)
	if resp != nil {
		c.logRateLimit(resp)
	}
	if err != nil {
		return nil, fmt.Errorf("list PRs: %w", err)
	}
	result := make([]PullRequestSummary, 0, len(prs))
	for _, pr := range prs {
		result = append(result, PullRequestSummary{
			Number:    pr.GetNumber(),
			Title:     pr.GetTitle(),
			State:     pr.GetState(),
			HTMLURL:   pr.GetHTMLURL(),
			Head:      pr.GetHead().GetRef(),
			Base:      pr.GetBase().GetRef(),
			CreatedAt: pr.GetCreatedAt().Format(time.RFC3339),
			UpdatedAt: pr.GetUpdatedAt().Format(time.RFC3339),
			User:      pr.GetUser().GetLogin(),
		})
	}
	return result, nil
}

// CreateRepo creates a new GitHub repository under the authenticated user's account.
// If orgName is non-empty, the repo is created under that organization.
func (c *Client) CreateRepo(ctx context.Context, name, description, orgName string, private bool) (*Repository, error) {
	newRepo := &ghlib.Repository{
		Name:        ghlib.String(name),
		Description: ghlib.String(description),
		Private:     ghlib.Bool(private),
		AutoInit:    ghlib.Bool(true), // initialize with README so the repo is not empty
	}

	var (
		r    *ghlib.Repository
		resp *ghlib.Response
		err  error
	)
	if orgName != "" {
		r, resp, err = c.client.Repositories.Create(ctx, orgName, newRepo)
	} else {
		r, resp, err = c.client.Repositories.Create(ctx, "", newRepo)
	}
	if err != nil {
		return nil, fmt.Errorf("create repo: %w", err)
	}
	c.logRateLimit(resp)

	result := repoFromGitHub(r)
	return &result, nil
}

// DeleteRepo deletes a GitHub repository. Requires admin/delete permissions on the repo.
func (c *Client) DeleteRepo(ctx context.Context, owner, repo string) error {
	resp, err := c.client.Repositories.Delete(ctx, owner, repo)
	if err != nil {
		return fmt.Errorf("delete repo %s/%s: %w", owner, repo, err)
	}
	c.logRateLimit(resp)
	return nil
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
