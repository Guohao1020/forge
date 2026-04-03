package activity

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	ghAdapter "github.com/shulex/forge/forge-core/internal/adapter/github"
	"github.com/shulex/forge/forge-core/internal/module/task"
	"github.com/shulex/forge/forge-core/internal/workspace"
)

// AuthTokenProvider retrieves the decrypted GitHub access token for a user.
type AuthTokenProvider interface {
	GetGitHubToken(ctx context.Context, userID int64) (string, error)
}

// ProjectProvider retrieves project information.
type ProjectProvider interface {
	GetByID(ctx context.Context, id, tenantID, userID int64) (*ProjectInfo, error)
}

// ProjectInfo is a minimal projection of project data needed by DevOps activities.
type ProjectInfo struct {
	CodeRepoURL   string
	DefaultBranch string
}

// TaskPRUpdater updates PR metadata on a task.
type TaskPRUpdater interface {
	UpdatePRInfo(ctx context.Context, taskID int64, prNumber int, mrUrl string, reviewScore int) error
}

// DevOpsActivities handles GitHub operations (branch, commit, PR).
type DevOpsActivities struct {
	db          *pgxpool.Pool
	authToken   AuthTokenProvider
	projectProv ProjectProvider
	taskPR      TaskPRUpdater
	sse         *task.SSEHub
	ws          *workspace.Manager // optional — nil means skip local workspace ops
}

// NewDevOpsActivities creates a new DevOpsActivities instance.
func NewDevOpsActivities(db *pgxpool.Pool, auth AuthTokenProvider, proj ProjectProvider, taskPR TaskPRUpdater, sse *task.SSEHub, ws *workspace.Manager) *DevOpsActivities {
	return &DevOpsActivities{
		db:          db,
		authToken:   auth,
		projectProv: proj,
		taskPR:      taskPR,
		sse:         sse,
		ws:          ws,
	}
}

// --- Input/Output types ---

type PushToGitHubInput struct {
	TaskID        int64       `json:"task_id"`
	TenantID      int64       `json:"tenant_id"`
	ProjectID     int64       `json:"project_id"`
	CreatedBy     int64       `json:"created_by"`
	Title         string      `json:"title"`
	Files         interface{} `json:"files"` // []FileChange or []map[string]interface{} from Temporal
	CommitMessage string      `json:"commit_message"`
}

type PushToGitHubOutput struct {
	BranchName string `json:"branch_name"`
}

type CreatePRInput struct {
	TaskID    int64  `json:"task_id"`
	TenantID  int64  `json:"tenant_id"`
	ProjectID int64  `json:"project_id"`
	CreatedBy int64  `json:"created_by"`
	Branch    string `json:"branch_name"`
	Title     string `json:"title"`
}

type CreatePROutput struct {
	PRNumber int    `json:"pr_number"`
	PRURL    string `json:"pr_url"`
}

type SavePRInfoInput struct {
	TaskID      int64  `json:"task_id"`
	PRNumber    int    `json:"pr_number"`
	PRURL       string `json:"pr_url"`
	ReviewScore int    `json:"review_score"`
}

// --- Activities ---

// PushToGitHub creates a branch and commits generated files.
func (a *DevOpsActivities) PushToGitHub(ctx context.Context, input PushToGitHubInput) (*PushToGitHubOutput, error) {
	// Convert files from interface{} (Temporal JSON) to []FileChange
	files, err := toFileChanges(input.Files)
	if err != nil {
		return nil, fmt.Errorf("parse files: %w", err)
	}

	slog.Info("PushToGitHub started", "task_id", input.TaskID, "files", len(files))

	token, err := a.authToken.GetGitHubToken(ctx, input.CreatedBy)
	if err != nil {
		return nil, fmt.Errorf("get github token: %w", err)
	}

	proj, err := a.projectProv.GetByID(ctx, input.ProjectID, input.TenantID, input.CreatedBy)
	if err != nil {
		return nil, fmt.Errorf("get project: %w", err)
	}

	owner, repo, err := parseRepoURL(proj.CodeRepoURL)
	if err != nil {
		return nil, fmt.Errorf("parse repo url: %w", err)
	}

	branchName := generateBranchName(input.TaskID, input.TenantID, input.CreatedBy, input.Title)

	// Sync to local workspace (optional — graceful if git CLI unavailable)
	if a.ws != nil {
		if _, err := a.ws.EnsureClone(ctx, input.TenantID, input.ProjectID, proj.CodeRepoURL, token, proj.DefaultBranch); err != nil {
			slog.Warn("workspace: clone failed, skipping local copy", "task_id", input.TaskID, "error", err)
		} else {
			taskDir, wtErr := a.ws.CreateWorktree(ctx, input.TenantID, input.ProjectID, input.TaskID, branchName)
			if wtErr != nil {
				slog.Warn("workspace: worktree creation failed", "task_id", input.TaskID, "error", wtErr)
			} else {
				// Write files to local worktree
				wsFiles := make([]workspace.FileToWrite, 0, len(files))
				for _, f := range files {
					wsFiles = append(wsFiles, workspace.FileToWrite{Path: f.Path, Content: f.Content})
				}
				if wfErr := a.ws.WriteFiles(taskDir, wsFiles); wfErr != nil {
					slog.Warn("workspace: write files failed", "task_id", input.TaskID, "error", wfErr)
				}
			}
		}
	}

	gh := ghAdapter.NewClient(token)

	// Create branch from default branch
	if err := gh.CreateBranch(ctx, owner, repo, branchName, proj.DefaultBranch); err != nil {
		return nil, fmt.Errorf("create branch: %w", err)
	}

	if a.sse != nil {
		a.sse.Broadcast(input.TaskID, task.TaskProgressEvent{
			Type:   "step_progress",
			TaskID: input.TaskID,
			Status: "DEPLOYING",
			Data:   map[string]string{"detail": "branch created: " + branchName},
		})
	}

	// Commit files
	msg := input.CommitMessage
	if msg == "" {
		msg = fmt.Sprintf("feat: AI generated code for task #%d", input.TaskID)
	}

	if err := gh.CommitFiles(ctx, owner, repo, branchName, msg, files); err != nil {
		return nil, fmt.Errorf("commit files: %w", err)
	}

	if a.sse != nil {
		a.sse.Broadcast(input.TaskID, task.TaskProgressEvent{
			Type:   "step_progress",
			TaskID: input.TaskID,
			Status: "DEPLOYING",
			Data:   map[string]string{"detail": fmt.Sprintf("committed %d files", len(files))},
		})
	}

	slog.Info("PushToGitHub completed", "task_id", input.TaskID, "branch", branchName)
	return &PushToGitHubOutput{BranchName: branchName}, nil
}

// CreatePullRequest creates a PR for the AI-generated code.
func (a *DevOpsActivities) CreatePullRequest(ctx context.Context, input CreatePRInput) (*CreatePROutput, error) {
	slog.Info("CreatePullRequest started", "task_id", input.TaskID, "branch", input.Branch)

	token, err := a.authToken.GetGitHubToken(ctx, input.CreatedBy)
	if err != nil {
		return nil, fmt.Errorf("get github token: %w", err)
	}

	proj, err := a.projectProv.GetByID(ctx, input.ProjectID, input.TenantID, input.CreatedBy)
	if err != nil {
		return nil, fmt.Errorf("get project: %w", err)
	}

	owner, repo, err := parseRepoURL(proj.CodeRepoURL)
	if err != nil {
		return nil, fmt.Errorf("parse repo url: %w", err)
	}

	gh := ghAdapter.NewClient(token)

	title := input.Title
	if title == "" {
		title = fmt.Sprintf("AI: Task #%d", input.TaskID)
	}
	body := fmt.Sprintf("Auto-generated by Forge AI for task #%d.\n\nBranch: `%s`", input.TaskID, input.Branch)

	pr, err := gh.CreatePR(ctx, owner, repo, title, body, input.Branch, proj.DefaultBranch)
	if err != nil {
		return nil, fmt.Errorf("create PR: %w", err)
	}

	if a.sse != nil {
		a.sse.Broadcast(input.TaskID, task.TaskProgressEvent{
			Type:   "step_progress",
			TaskID: input.TaskID,
			Status: "DEPLOYING",
			Data:   map[string]string{"detail": fmt.Sprintf("PR #%d created", pr.Number), "pr_url": pr.HTMLURL},
		})
	}

	slog.Info("CreatePullRequest completed", "task_id", input.TaskID, "pr_number", pr.Number, "pr_url", pr.HTMLURL)
	return &CreatePROutput{PRNumber: pr.Number, PRURL: pr.HTMLURL}, nil
}

// SavePRInfo persists PR metadata on the task record.
func (a *DevOpsActivities) SavePRInfo(ctx context.Context, input SavePRInfoInput) error {
	slog.Info("SavePRInfo", "task_id", input.TaskID, "pr_number", input.PRNumber, "pr_url", input.PRURL)
	return a.taskPR.UpdatePRInfo(ctx, input.TaskID, input.PRNumber, input.PRURL, input.ReviewScore)
}

// --- Helpers ---

// toFileChanges converts an interface{} (from Temporal JSON deserialization) to []FileChange.
func toFileChanges(raw interface{}) ([]ghAdapter.FileChange, error) {
	if raw == nil {
		return nil, nil
	}

	// Re-marshal and unmarshal through JSON to handle any map/slice type
	b, err := json.Marshal(raw)
	if err != nil {
		return nil, fmt.Errorf("marshal files: %w", err)
	}

	var files []ghAdapter.FileChange
	if err := json.Unmarshal(b, &files); err != nil {
		return nil, fmt.Errorf("unmarshal files: %w", err)
	}
	return files, nil
}

// generateBranchName builds a branch name following the convention:
//
//	feature/{YYYYMMDD}/{tenantId}/{userId}/{slug}   — 新功能
//	fix/{YYYYMMDD}/{tenantId}/{userId}/{slug}       — 修复
//	release/{version}                                — 发布
//
// slug = task title, max 15 chars, kebab-case ASCII
//
// Examples:
//
//	feature/20260403/1/1/12-health-check
//	fix/20260403/1/1/15-fix-login-bug
func generateBranchName(taskID int64, tenantID int64, userID int64, title string) string {
	date := time.Now().Format("20060102")
	slug := toSlug(title)
	if slug == "" {
		slug = "feature"
	}
	if len(slug) > 15 {
		slug = slug[:15]
	}
	slug = strings.TrimRight(slug, "-")

	// Detect fix/bug keywords
	prefix := "feature"
	lower := strings.ToLower(title)
	if strings.Contains(lower, "fix") || strings.Contains(lower, "bug") ||
		strings.Contains(lower, "修复") || strings.Contains(lower, "修改") {
		prefix = "fix"
	}

	return fmt.Sprintf("%s/%s/%d/%d/%d-%s", prefix, date, tenantID, userID, taskID, slug)
}

// toSlug converts a string to a URL/branch-safe kebab-case slug.
// Keeps only a-z, 0-9, replaces spaces/underscores/dashes with single dashes,
// and drops Chinese and other non-ASCII characters.
func toSlug(s string) string {
	s = strings.ToLower(s)
	var buf strings.Builder
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			buf.WriteRune(r)
		} else if r == ' ' || r == '_' || r == '-' {
			buf.WriteByte('-')
		}
		// Skip Chinese and other chars
	}
	result := buf.String()
	// Collapse multiple dashes
	for strings.Contains(result, "--") {
		result = strings.ReplaceAll(result, "--", "-")
	}
	return strings.Trim(result, "-")
}

// parseRepoURL extracts owner and repo from a GitHub URL.
// Supports: https://github.com/owner/repo, https://github.com/owner/repo.git
func parseRepoURL(rawURL string) (owner, repo string, err error) {
	rawURL = strings.TrimSuffix(rawURL, ".git")
	rawURL = strings.TrimSuffix(rawURL, "/")

	// Try parsing as URL path segments
	parts := strings.Split(rawURL, "/")
	if len(parts) < 2 {
		return "", "", fmt.Errorf("invalid repo URL: %s", rawURL)
	}

	repo = parts[len(parts)-1]
	owner = parts[len(parts)-2]

	if owner == "" || repo == "" {
		return "", "", fmt.Errorf("could not extract owner/repo from URL: %s", rawURL)
	}

	return owner, repo, nil
}
