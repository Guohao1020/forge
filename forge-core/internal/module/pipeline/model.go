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

type DeployRecord struct {
	ID            int64      `json:"id"`
	TenantID      int64      `json:"tenantId"`
	ProjectID     int64      `json:"projectId"`
	EnvironmentID int64      `json:"environmentId"`
	ArtifactID    *int64     `json:"artifactId,omitempty"`
	Version       string     `json:"version"`
	Status        string     `json:"status"` // PENDING / DEPLOYING / DEPLOYED / FAILED / ROLLED_BACK
	DeployedBy    int64      `json:"deployedBy"`
	StartedAt     time.Time  `json:"startedAt"`
	CompletedAt   *time.Time `json:"completedAt,omitempty"`
	K8sManifest   *string    `json:"k8sManifest,omitempty"`
	ErrorMessage  *string    `json:"errorMessage,omitempty"`
	CreatedAt     time.Time  `json:"createdAt"`
}

type DeployRecordListResponse struct {
	Records []DeployRecord `json:"records"`
}

type TriggerDeployRequest struct {
	Version    string `json:"version" binding:"required"`
	ArtifactID *int64 `json:"artifactId,omitempty"`
}
