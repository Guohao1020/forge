package project

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	temporalclient "go.temporal.io/sdk/client"

	ghAdapter "github.com/shulex/forge/forge-core/internal/adapter/github"
)

// AuthTokenProvider retrieves the decrypted GitHub access token for a user.
type AuthTokenProvider interface {
	GetGitHubToken(ctx context.Context, userID int64) (string, error)
}

type Service struct {
	repo           *Repository
	authSvc        AuthTokenProvider
	ws             WorkspaceProvider // optional, for local file browsing
	temporalClient temporalclient.Client // optional, for triggering profile scans
}

// WorkspaceProvider reads files from local workspace (clone).
type WorkspaceProvider interface {
	ProjectDir(tenantID, projectID int64) string
}

func NewService(repo *Repository, authSvc AuthTokenProvider, ws WorkspaceProvider) *Service {
	return &Service{repo: repo, authSvc: authSvc, ws: ws}
}

// SetTemporalClient sets the Temporal client for async operations (profile scanning).
// Called from main.go after Temporal initialization.
func (s *Service) SetTemporalClient(tc temporalclient.Client) {
	s.temporalClient = tc
}


func (s *Service) Create(ctx context.Context, tenantID, userID int64, req *CreateProjectRequest) (*Project, error) {
	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" {
		return nil, errors.New("项目名称不能为空")
	}

	wantSync := req.SyncToRemote
	// Clear repo fields — insert DB first to validate name uniqueness
	if wantSync {
		req.CodePlatform = ""
		req.CodeRepoURL = ""
	}

	// 1. Insert project into DB first (validates unique name)
	p, err := s.repo.Create(ctx, tenantID, userID, req)
	if err != nil {
		if strings.Contains(err.Error(), "projects_tenant_id_name_key") {
			return nil, errors.New("项目名称已存在")
		}
		return nil, err
	}

	// 2. Then create GitHub repo and backfill URL
	if wantSync {
		token, tokenErr := s.authSvc.GetGitHubToken(ctx, userID)
		if tokenErr != nil || token == "" {
			// Project created but no GitHub — return with warning
			slog.Warn("project created without GitHub sync — no token", "project_id", p.ID)
			return p, nil
		}
		ghClient := ghAdapter.NewClient(token)
		repoName := strings.TrimSpace(req.RepoName)
		if repoName == "" {
			repoName = slugify(req.Name)
		}
		if repoName == "" {
			repoName = fmt.Sprintf("forge-project-%d", p.ID)
		}
		repo, ghErr := ghClient.CreateRepo(ctx, repoName, req.Description, "", req.RepoPrivate)
		if ghErr != nil {
			slog.Warn("project created but GitHub repo creation failed", "project_id", p.ID, "error", ghErr)
			return p, nil
		}
		// Backfill repo info
		platform := "github"
		repoURL := repo.HTMLURL
		branch := repo.DefaultBranch
		updated, updErr := s.repo.Update(ctx, p.ID, tenantID, &UpdateProjectRequest{
			CodePlatform:  &platform,
			CodeRepoURL:   &repoURL,
			DefaultBranch: &branch,
		})
		if updErr != nil {
			slog.Warn("failed to backfill repo URL", "project_id", p.ID, "error", updErr)
			return p, nil
		}
		slog.Info("auto-created GitHub repo", "project_id", p.ID, "repo", repo.FullName)
		return updated, nil
	}

	return p, nil
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

// SyncProjectToRemote creates a GitHub repo for a project that has no remote repo yet.
func (s *Service) SyncProjectToRemote(ctx context.Context, id, tenantID, userID int64, private bool) (*Project, error) {
	p, err := s.repo.GetByID(ctx, id, tenantID, userID)
	if err != nil {
		return nil, errors.New("项目不存在")
	}
	if p.CodeRepoURL != "" {
		return nil, errors.New("项目已关联远程仓库")
	}

	token, err := s.authSvc.GetGitHubToken(ctx, userID)
	if err != nil || token == "" {
		return nil, errors.New("请先连接 GitHub 账户")
	}
	ghClient := ghAdapter.NewClient(token)
	repoName := slugify(p.Name)
	if repoName == "" {
		repoName = fmt.Sprintf("forge-project-%d", p.ID)
	}
	repo, err := ghClient.CreateRepo(ctx, repoName, p.Description, "", private)
	if err != nil {
		return nil, fmt.Errorf("创建 GitHub 仓库失败: %w", err)
	}

	platform := "github"
	repoURL := repo.HTMLURL
	branch := repo.DefaultBranch
	updated, err := s.repo.Update(ctx, id, tenantID, &UpdateProjectRequest{
		CodePlatform:  &platform,
		CodeRepoURL:   &repoURL,
		DefaultBranch: &branch,
	})
	if err != nil {
		return nil, err
	}

	slog.Info("synced project to GitHub", "project_id", id, "repo", repo.FullName)
	return updated, nil
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

	// Detect tech stack from files (basic frameworks)
	techStack := map[string]interface{}{
		"languages":   languages,
		"frameworks":  detectFrameworks(files),
		"detected_at": time.Now().Format(time.RFC3339),
	}

	// Enhanced project type detection (SP-1)
	profile := DetectProjectType(files, languages)
	techStack = enhanceTechStack(techStack, profile)
	slog.Info("project type detected",
		"project_id", projectID,
		"type", profile.ProjectType,
		"subType", profile.SubType,
		"confidence", profile.Confidence,
		"branchStrategy", profile.BranchStrategy,
	)

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

		// Auto-trigger profile scan for the newly imported project
		if s.temporalClient != nil {
			go func(pid int64) {
				bgCtx := context.Background()
				input := map[string]interface{}{
					"project_id": pid,
					"user_id":    userID,
				}
				opts := temporalclient.StartWorkflowOptions{
					ID:        fmt.Sprintf("profile-scan-%d-%d", pid, time.Now().Unix()),
					TaskQueue: "forge-task-queue",
				}
				we, err := s.temporalClient.ExecuteWorkflow(bgCtx, opts, "ProfileScanWorkflow", input)
				if err != nil {
					slog.Warn("auto profile scan failed to start", "project_id", pid, "error", err)
				} else {
					slog.Info("auto profile scan started on import", "project_id", pid, "workflow_id", we.GetID())
				}
			}(brief.ID)
		}
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

// slugify converts a project name to a valid GitHub repo name.
// Keeps ASCII letters, digits, hyphens; replaces everything else with '-'.
func slugify(name string) string {
	var b strings.Builder
	for _, r := range name {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' {
			b.WriteRune(r)
		} else {
			b.WriteByte('-')
		}
	}
	// Collapse multiple hyphens, trim leading/trailing hyphens
	s := b.String()
	for strings.Contains(s, "--") {
		s = strings.ReplaceAll(s, "--", "-")
	}
	return strings.Trim(s, "-")
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
// Tries local workspace first (fast), falls back to GitHub API.
// Git pull runs async in background — returns cached files immediately.
func (s *Service) GetCodeTree(ctx context.Context, projectID, tenantID, userID int64, ref string) ([]string, error) {
	// Try local workspace first (default branch only)
	if s.ws != nil && (ref == "" || ref == "main" || ref == "master") {
		dir := s.ws.ProjectDir(tenantID, projectID)
		if _, err := os.Stat(filepath.Join(dir, ".git")); err == nil {
			// Async git pull — don't block the request
			go func() {
				pullCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
				defer cancel()
				pullCmd := exec.CommandContext(pullCtx, "git", "-C", dir, "pull", "--ff-only")
				pullCmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
				if out, err := pullCmd.CombinedOutput(); err != nil {
					slog.Debug("background git pull failed", "dir", dir, "error", err, "output", string(out))
				} else {
					slog.Debug("background git pull succeeded", "dir", dir)
				}
			}()

			// Return cached local files immediately
			if files, err := listLocalFiles(dir); err == nil && len(files) > 0 {
				slog.Debug("code tree from local workspace", "project_id", projectID, "files", len(files))
				return files, nil
			}
		}
	}

	// Fallback to GitHub API
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
// Tries local workspace first (fast), falls back to GitHub API.
func (s *Service) GetCodeFile(ctx context.Context, projectID, tenantID, userID int64, path, ref string) (string, error) {
	// Try local workspace first (default branch only)
	if s.ws != nil && (ref == "" || ref == "main" || ref == "master") {
		dir := s.ws.ProjectDir(tenantID, projectID)
		fullPath := filepath.Join(dir, path)
		if data, err := os.ReadFile(fullPath); err == nil {
			slog.Debug("code file from local workspace", "path", path)
			return string(data), nil
		}
	}

	// Fallback to GitHub API
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

// listLocalFiles walks a directory and returns relative file paths.
func listLocalFiles(dir string) ([]string, error) {
	if _, err := os.Stat(dir); err != nil {
		return nil, err
	}
	var files []string
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		// Skip .git directory
		if info.IsDir() && info.Name() == ".git" {
			return filepath.SkipDir
		}
		rel, _ := filepath.Rel(dir, path)
		if rel != "." && rel != "" {
			// Use forward slashes for consistency
			files = append(files, filepath.ToSlash(rel))
		}
		return nil
	})
	return files, err
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

func (s *Service) GetProjectStats(ctx context.Context, projectID int64) (*ProjectStats, error) {
	return s.repo.GetProjectStats(ctx, projectID)
}
