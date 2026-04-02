package preview

import "time"

// PreviewEnvironment represents a PR-level preview environment.
// TODO: When K8s is available, the namespace field will map to a real K8s namespace.
type PreviewEnvironment struct {
	ID         int64      `json:"id"`
	TenantID   int64      `json:"tenantId"`
	ProjectID  int64      `json:"projectId"`
	TaskID     *int64     `json:"taskId,omitempty"`
	BranchName *string    `json:"branchName,omitempty"`
	PRNumber   *int       `json:"prNumber,omitempty"`
	PreviewURL *string    `json:"previewUrl,omitempty"`
	Status     string     `json:"status"`
	Namespace  *string    `json:"namespace,omitempty"`
	ExpiresAt  *time.Time `json:"expiresAt,omitempty"`
	CreatedAt  time.Time  `json:"createdAt"`
	UpdatedAt  time.Time  `json:"updatedAt"`
}

type PreviewListResponse struct {
	Previews []PreviewEnvironment `json:"previews"`
}
