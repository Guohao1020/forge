package project

import (
	"encoding/json"
	"time"
)

type Project struct {
	ID            int64      `json:"id"`
	TenantID      int64      `json:"tenantId"`
	Name          string     `json:"name"`
	Description   string     `json:"description"`
	Status        string     `json:"status"`
	CodePlatform  string     `json:"codePlatform"`
	CodeRepoURL   string     `json:"codeRepoUrl"`
	DefaultBranch string     `json:"defaultBranch"`
	AIModel       string     `json:"aiModel"`
	RiskThreshold int        `json:"riskThreshold"`
	AutoMerge     bool       `json:"autoMerge"`
	TechStack     json.RawMessage `json:"techStack,omitempty" db:"tech_stack"`
	CreatedBy     int64           `json:"createdBy"`
	CreatedAt     time.Time       `json:"createdAt"`
	UpdatedAt     time.Time       `json:"updatedAt"`
	Starred       bool            `json:"starred"`
}

type CreateProjectRequest struct {
	Name          string `json:"name" binding:"required,min=1,max=200"`
	Description   string `json:"description"`
	CodePlatform  string `json:"codePlatform"`
	CodeRepoURL   string `json:"codeRepoUrl"`
	DefaultBranch string `json:"defaultBranch"`
	AIModel       string `json:"aiModel"`
	RiskThreshold *int   `json:"riskThreshold"`
	AutoMerge     *bool  `json:"autoMerge"`
	SyncToRemote  bool   `json:"syncToRemote"`  // true = auto-create repo on GitHub
	RepoPrivate   bool   `json:"repoPrivate"`   // private repo when syncToRemote
	RepoName      string `json:"repoName"`      // GitHub repo name (ASCII slug)
}

type UpdateProjectRequest struct {
	Name          *string `json:"name"`
	Description   *string `json:"description"`
	DefaultBranch *string `json:"defaultBranch"`
	CodePlatform  *string `json:"codePlatform"`
	CodeRepoURL   *string `json:"codeRepoUrl"`
}

type ListProjectsQuery struct {
	Search  string `form:"search"`
	Starred bool   `form:"starred"`
	Page    int    `form:"page"`
	Size    int    `form:"size"`
}

type ListProjectsResponse struct {
	Projects []*Project `json:"projects"`
	Total    int64      `json:"total"`
	Page     int        `json:"page"`
	Size     int        `json:"size"`
}

// ProjectStats provides an overview of project activity.
type ProjectStats struct {
	TasksByStatus   map[string]int64 `json:"tasksByStatus"`
	TotalTasks      int64            `json:"totalTasks"`
	CompletedTasks  int64            `json:"completedTasks"`
	ActiveVersions  int64            `json:"activeVersions"`
	LastActivity    *string          `json:"lastActivity,omitempty"` // timestamp
	QualityScore    *int             `json:"qualityScore,omitempty"`
}

// ImportRepoItem represents a single GitHub repo to import.
type ImportRepoItem struct {
	FullName      string `json:"full_name" binding:"required"`
	Name          string `json:"name" binding:"required"`
	Description   string `json:"description"`
	HTMLURL       string `json:"html_url" binding:"required"`
	CloneURL      string `json:"clone_url"`
	DefaultBranch string `json:"default_branch"`
	Language      string `json:"language"`
}

// ImportRequest is the request body for POST /api/projects/import.
type ImportRequest struct {
	Repos []ImportRepoItem `json:"repos" binding:"required,min=1"`
}

// ImportResponse contains the result of a batch import.
type ImportResponse struct {
	Imported int            `json:"imported"`
	Skipped  int            `json:"skipped"`
	Projects []ProjectBrief `json:"projects"`
	Errors   []string       `json:"errors,omitempty"`
}

// ProjectBrief is a minimal project representation for import response.
type ProjectBrief struct {
	ID            int64  `json:"id"`
	Name          string `json:"name"`
	CodeRepoURL   string `json:"code_repo_url"`
	DefaultBranch string `json:"default_branch"`
}
