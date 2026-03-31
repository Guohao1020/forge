package project

import "time"

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
	CreatedBy     int64      `json:"createdBy"`
	CreatedAt     time.Time  `json:"createdAt"`
	UpdatedAt     time.Time  `json:"updatedAt"`
	Starred       bool       `json:"starred"`
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
}

type UpdateProjectRequest struct {
	Name          *string `json:"name"`
	Description   *string `json:"description"`
	DefaultBranch *string `json:"defaultBranch"`
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
