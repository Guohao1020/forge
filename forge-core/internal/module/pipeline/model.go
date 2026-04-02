package pipeline

import "time"

type Environment struct {
	ID             int64      `json:"id"`
	TenantID       int64      `json:"tenantId"`
	ProjectID      int64      `json:"projectId"`
	Name           string     `json:"name"`
	EnvType        string     `json:"envType"`
	Status         string     `json:"status"`
	CurrentVersion *string    `json:"currentVersion,omitempty"`
	LastDeployAt   *time.Time `json:"lastDeployAt,omitempty"`
	CreatedAt      time.Time  `json:"createdAt"`
	UpdatedAt      time.Time  `json:"updatedAt"`
}

type EnvironmentListResponse struct {
	Environments []Environment `json:"environments"`
}
