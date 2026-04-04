package version

import (
	"encoding/json"
	"time"
)

// Version status constants
const (
	StatusPlanning   = "PLANNING"
	StatusInProgress = "IN_PROGRESS"
	StatusTesting    = "TESTING"
	StatusReleased   = "RELEASED"
	StatusCancelled  = "CANCELLED"
)

// ProjectVersion represents a versioned release grouping multiple tasks.
type ProjectVersion struct {
	ID          int64     `json:"id"`
	TenantID    int64     `json:"tenantId"`
	ProjectID   int64     `json:"projectId"`
	Version     string    `json:"version"`
	Status      string    `json:"status"`
	Description string    `json:"description"`
	GitTag      string    `json:"gitTag,omitempty"`
	ReleasedAt  *time.Time `json:"releasedAt,omitempty"`
	CreatedBy   int64     `json:"createdBy"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`

	// Computed fields (not stored in DB, populated by service)
	TaskCount     int `json:"taskCount"`
	CompletedCount int `json:"completedCount"`
}

// CreateVersionRequest is the request body for creating a new version.
type CreateVersionRequest struct {
	Version     string `json:"version" binding:"required"`
	Description string `json:"description"`
}

// UpdateVersionRequest is the request body for updating a version.
type UpdateVersionRequest struct {
	Description *string `json:"description"`
	Status      *string `json:"status"`
}

// VersionListResponse wraps the version list with task progress.
type VersionListResponse struct {
	Versions []ProjectVersion `json:"versions"`
}

// VersionDetailResponse includes the version and its associated tasks.
type VersionDetailResponse struct {
	Version ProjectVersion    `json:"version"`
	Tasks   []VersionTaskBrief `json:"tasks"`
}

// VersionTaskBrief is a summary of a task within a version.
type VersionTaskBrief struct {
	ID             int64            `json:"id"`
	Title          string           `json:"title"`
	Status         string           `json:"status"`
	ConflictStatus string           `json:"conflictStatus"`
	BlockedBy      json.RawMessage  `json:"blockedBy"`
	TouchedFiles   json.RawMessage  `json:"touchedFiles"`
	BranchName     string           `json:"branchName,omitempty"`
	PRNumber       int              `json:"prNumber,omitempty"`
	CreatedAt      time.Time        `json:"createdAt"`
	CompletedAt    *time.Time       `json:"completedAt,omitempty"`
}

// ReleaseRequest is the request body for releasing a version.
type ReleaseRequest struct {
	DeployAfterTag bool `json:"deployAfterTag"` // whether to trigger deployment after tagging
}
