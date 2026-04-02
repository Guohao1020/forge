package project

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"path/filepath"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	ghAdapter "github.com/shulex/forge/forge-core/internal/adapter/github"
)

// AuthTokenProvider retrieves the decrypted GitHub access token for a user.
type AuthTokenProvider interface {
	GetGitHubToken(ctx context.Context, userID int64) (string, error)
}

type Service struct {
	repo    *Repository
	authSvc AuthTokenProvider
}

func NewService(repo *Repository, authSvc AuthTokenProvider) *Service {
	return &Service{repo: repo, authSvc: authSvc}
}

func (s *Service) Create(ctx context.Context, tenantID, userID int64, req *CreateProjectRequest) (*Project, error) {
	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" {
		return nil, errors.New("项目名称不能为空")
	}
	return s.repo.Create(ctx, tenantID, userID, req)
}

func (s *Service) List(ctx context.Context, tenantID, userID int64, q *ListProjectsQuery) (*ListProjectsResponse, error) {
	projects, total, err := s.repo.List(ctx, tenantID, userID, q)
	if err != nil {
		return nil, err
	}
	if projects == nil {
		projects = []*Project{}
	}
	return &ListProjectsResponse{
		Projects: projects,
		Total:    total,
		Page:     q.Page,
		Size:     q.Size,
	}, nil
}

func (s *Service) GetByID(ctx context.Context, id, tenantID, userID int64) (*Project, error) {
	p, err := s.repo.GetByID(ctx, id, tenantID, userID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, errors.New("项目不存在")
	}
	return p, err
}

func (s *Service) Update(ctx context.Context, id, tenantID int64, req *UpdateProjectRequest) (*Project, error) {
	p, err := s.repo.Update(ctx, id, tenantID, req)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, errors.New("项目不存在")
	}
	return p, err
}

func (s *Service) Archive(ctx context.Context, id, tenantID int64) error {
	return s.repo.Archive(ctx, id, tenantID)
}

func (s *Service) Star(ctx context.Context, projectID, tenantID, userID int64) error {
	// Verify project belongs to tenant
	if _, err := s.repo.GetByID(ctx, projectID, tenantID, userID); err != nil {
		return errors.New("项目不存在")
	}
	return s.repo.Star(ctx, projectID, userID)
}

func (s *Service) Unstar(ctx context.Context, projectID, tenantID, userID int64) error {
	return s.repo.Unstar(ctx, projectID, userID)
}

// DetectTechStack scans a GitHub repo and detects its tech stack.
func (s *Service) DetectTechStack(ctx context.Context, projectID, tenantID, userID int64) error {
	project, err := s.repo.GetByID(ctx, projectID, tenantID, userID)
	if err != nil {
		return err
	}
	if project.CodeRepoURL == "" {
		return nil // No repo to scan
	}

	owner, repo := parseOwnerRepo(project.CodeRepoURL)
	if owner == "" || repo == "" {
		return fmt.Errorf("invalid repo URL: %s", project.CodeRepoURL)
	}

	// Get GitHub token
	token, err := s.authSvc.GetGitHubToken(ctx, userID)
	if err != nil || token == "" {
		slog.Warn("no GitHub token for tech stack detection", "user_id", userID)
		return nil
	}

	ghClient := ghAdapter.NewClient(token)

	// Get languages
	languages, err := ghClient.GetRepoLanguages(ctx, owner, repo)
	if err != nil {
		slog.Warn("failed to get repo languages", "error", err)
		languages = map[string]int{}
	}

	// Get file tree for config detection
	files, err := ghClient.GetTree(ctx, owner, repo, project.DefaultBranch)
	if err != nil {
		slog.Warn("failed to get repo tree", "error", err)
		files = []string{}
	}

	// Detect tech stack from files
	techStack := map[string]interface{}{
		"languages":   languages,
		"frameworks":  detectFrameworks(files),
		"detected_at": time.Now().Format(time.RFC3339),
	}

	tsJSON, _ := json.Marshal(techStack)
	return s.repo.UpdateTechStack(ctx, projectID, string(tsJSON))
}

func detectFrameworks(files []string) []string {
	frameworks := []string{}
	for _, f := range files {
		base := filepath.Base(f)
		switch {
		case base == "go.mod":
			frameworks = append(frameworks, "Go")
		case base == "pom.xml":
			frameworks = append(frameworks, "Java/Maven")
		case base == "build.gradle" || base == "build.gradle.kts":
			frameworks = append(frameworks, "Java/Gradle")
		case base == "package.json":
			frameworks = append(frameworks, "Node.js")
		case base == "requirements.txt" || base == "pyproject.toml":
			frameworks = append(frameworks, "Python")
		case base == "Cargo.toml":
			frameworks = append(frameworks, "Rust")
		case base == "Dockerfile":
			frameworks = append(frameworks, "Docker")
		}
	}
	// Deduplicate
	seen := map[string]bool{}
	result := []string{}
	for _, f := range frameworks {
		if !seen[f] {
			seen[f] = true
			result = append(result, f)
		}
	}
	return result
}

// ImportFromGitHub imports selected GitHub repos as Forge projects.
func (s *Service) ImportFromGitHub(ctx context.Context, tenantID, userID int64, req *ImportRequest) (*ImportResponse, error) {
	resp := &ImportResponse{}

	for _, item := range req.Repos {
		brief, err := s.repo.CreateFromImport(ctx, tenantID, userID, &item)
		if err != nil {
			resp.Errors = append(resp.Errors, fmt.Sprintf("%s: %s", item.FullName, err.Error()))
			continue
		}
		if brief == nil {
			resp.Skipped++
			continue
		}
		resp.Imported++
		resp.Projects = append(resp.Projects, *brief)

		// Trigger async tech stack detection for the newly imported project
		go func(pid int64) {
			bgCtx := context.Background()
			if err := s.DetectTechStack(bgCtx, pid, tenantID, userID); err != nil {
				slog.Warn("tech stack detection failed", "project_id", pid, "error", err)
			}
		}(brief.ID)
	}

	if resp.Projects == nil {
		resp.Projects = []ProjectBrief{}
	}

	return resp, nil
}

// ---------------------------------------------------------------------------
// Code browsing
// ---------------------------------------------------------------------------

// getGitHubClient resolves project + authenticated GitHub client.
func (s *Service) getGitHubClient(ctx context.Context, projectID, tenantID, userID int64) (*Project, *ghAdapter.Client, error) {
	p, err := s.repo.GetByID(ctx, projectID, tenantID, userID)
	if err != nil {
		return nil, nil, err
	}
	if p.CodeRepoURL == "" {
		return nil, nil, fmt.Errorf("project has no repo URL")
	}
	token, err := s.authSvc.GetGitHubToken(ctx, userID)
	if err != nil || token == "" {
		return nil, nil, fmt.Errorf("no GitHub token available")
	}
	return p, ghAdapter.NewClient(token), nil
}

// parseOwnerRepo extracts owner and repo from a GitHub URL.
func parseOwnerRepo(rawURL string) (string, string) {
	parts := strings.Split(strings.TrimSuffix(rawURL, ".git"), "/")
	if len(parts) < 2 {
		return "", ""
	}
	return parts[len(parts)-2], parts[len(parts)-1]
}

// GetCodeTree returns the file tree for a given ref (branch/tag/SHA).
func (s *Service) GetCodeTree(ctx context.Context, projectID, tenantID, userID int64, ref string) ([]string, error) {
	p, ghClient, err := s.getGitHubClient(ctx, projectID, tenantID, userID)
	if err != nil {
		return nil, err
	}
	owner, repo := parseOwnerRepo(p.CodeRepoURL)
	if ref == "" {
		ref = p.DefaultBranch
	}
	return ghClient.GetTree(ctx, owner, repo, ref)
}

// GetCodeFile returns file content at a specific path and ref.
func (s *Service) GetCodeFile(ctx context.Context, projectID, tenantID, userID int64, path, ref string) (string, error) {
	p, ghClient, err := s.getGitHubClient(ctx, projectID, tenantID, userID)
	if err != nil {
		return "", err
	}
	owner, repo := parseOwnerRepo(p.CodeRepoURL)
	if ref == "" {
		ref = p.DefaultBranch
	}
	return ghClient.GetFileContent(ctx, owner, repo, path, ref)
}

// ListBranches returns all branches for the project's repo.
func (s *Service) ListBranches(ctx context.Context, projectID, tenantID, userID int64) ([]ghAdapter.Branch, error) {
	p, ghClient, err := s.getGitHubClient(ctx, projectID, tenantID, userID)
	if err != nil {
		return nil, err
	}
	owner, repo := parseOwnerRepo(p.CodeRepoURL)
	return ghClient.ListBranches(ctx, owner, repo)
}

// ListPRs returns pull requests for the project's repo.
func (s *Service) ListPRs(ctx context.Context, projectID, tenantID, userID int64, state string) ([]ghAdapter.PullRequestSummary, error) {
	p, ghClient, err := s.getGitHubClient(ctx, projectID, tenantID, userID)
	if err != nil {
		return nil, err
	}
	owner, repo := parseOwnerRepo(p.CodeRepoURL)
	return ghClient.ListPRs(ctx, owner, repo, state)
}

// GetPRDetail returns the changed files for a pull request.
func (s *Service) GetPRDetail(ctx context.Context, projectID, tenantID, userID int64, prNumber int) ([]ghAdapter.PRFile, error) {
	p, ghClient, err := s.getGitHubClient(ctx, projectID, tenantID, userID)
	if err != nil {
		return nil, err
	}
	owner, repo := parseOwnerRepo(p.CodeRepoURL)
	return ghClient.GetPRFiles(ctx, owner, repo, prNumber)
}
