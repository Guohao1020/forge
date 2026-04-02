package artifact

import (
	"encoding/json"
	"time"
)

type Artifact struct {
	ID           int64           `json:"id"`
	TenantID     int64           `json:"tenantId"`
	ProjectID    int64           `json:"projectId"`
	TaskID       *int64          `json:"taskId,omitempty"`
	Name         string          `json:"name"`
	Version      string          `json:"version"`
	ArtifactType string          `json:"artifactType"`
	RegistryURL  *string         `json:"registryUrl,omitempty"`
	SizeBytes    *int64          `json:"sizeBytes,omitempty"`
	Checksum     *string         `json:"checksum,omitempty"`
	Metadata     json.RawMessage `json:"metadata"`
	Status       string          `json:"status"`
	CreatedAt    time.Time       `json:"createdAt"`
}

type ArtifactListResponse struct {
	Artifacts []Artifact `json:"artifacts"`
}
