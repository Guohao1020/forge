package activity

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"go.temporal.io/sdk/activity"
)

// BuildActivities handles Docker image building and artifact management.
type BuildActivities struct {
	db *pgxpool.Pool
}

func NewBuildActivities(db *pgxpool.Pool) *BuildActivities {
	return &BuildActivities{db: db}
}

// BuildImageInput is the input for the BuildDockerImage activity.
type BuildImageInput struct {
	TaskID      int64  `json:"task_id"`
	TenantID    int64  `json:"tenant_id"`
	ProjectID   int64  `json:"project_id"`
	RepoURL     string `json:"repo_url"`
	Branch      string `json:"branch"`
	ProjectName string `json:"project_name"`
	Version     string `json:"version"` // Semantic version, e.g., "v1.2.0"
}

// BuildImageOutput is the result of a Docker build.
type BuildImageOutput struct {
	ImageURL    string `json:"image_url"`
	ImageTag    string `json:"image_tag"`
	SizeBytes   int64  `json:"size_bytes"`
	BuildTimeMs int64  `json:"build_time_ms"`
	ArtifactID  int64  `json:"artifact_id"`
}

// BuildDockerImage builds a Docker image from the project's Dockerfile
// and stores the artifact metadata in the database.
//
// In production, this would:
// 1. Clone the repo at the specified branch
// 2. Run `docker build` (or kaniko in K8s)
// 3. Push to ACR/GHCR
// 4. Run Trivy scan for vulnerabilities
// 5. Save artifact record
//
// Currently creates a placeholder artifact record (actual build requires Docker/K8s runtime).
func (a *BuildActivities) BuildDockerImage(ctx context.Context, input BuildImageInput) (*BuildImageOutput, error) {
	info := activity.GetInfo(ctx)
	slog.Info("BuildDockerImage activity",
		"task_id", input.TaskID,
		"project", input.ProjectName,
		"version", input.Version,
		"workflow_id", info.WorkflowExecution.ID,
	)

	start := time.Now()

	// Generate image tag
	shortSHA := fmt.Sprintf("%x", time.Now().UnixNano())[:8]
	imageTag := fmt.Sprintf("%s:%s-%s", input.ProjectName, input.Version, shortSHA)
	imageURL := fmt.Sprintf("registry.cn-hangzhou.aliyuncs.com/forge/%s", imageTag)

	// TODO: When Docker/K8s runtime is available:
	// 1. Create K8s Job with kaniko builder
	// 2. Mount repo at /workspace
	// 3. kaniko --context=/workspace --destination=${imageURL}
	// 4. Wait for job completion
	// 5. Run trivy image ${imageURL} --format json
	// For now: save artifact record with placeholder

	buildTimeMs := time.Since(start).Milliseconds()

	// Build metadata
	metadata := map[string]interface{}{
		"dockerfile":  "Dockerfile",
		"build_args":  map[string]string{},
		"registry":    "acr",
		"trivy_scan":  nil, // TODO: add when Trivy is available
		"build_type":  "placeholder", // Change to "docker" when real build is implemented
		"repo_url":    input.RepoURL,
		"branch":      input.Branch,
	}
	metadataJSON, _ := json.Marshal(metadata)

	// Save artifact record
	var artifactID int64
	err := a.db.QueryRow(ctx,
		`INSERT INTO pipeline.artifacts (tenant_id, project_id, task_id, name, version, artifact_type, registry_url, size_bytes, metadata, status)
		 VALUES ($1, $2, $3, $4, $5, 'DOCKER_IMAGE', $6, $7, $8::jsonb, 'BUILT')
		 RETURNING id`,
		input.TenantID, input.ProjectID, input.TaskID,
		input.ProjectName, input.Version,
		imageURL, 0, // size_bytes = 0 for placeholder
		string(metadataJSON),
	).Scan(&artifactID)
	if err != nil {
		return nil, fmt.Errorf("save artifact: %w", err)
	}

	slog.Info("artifact created",
		"artifact_id", artifactID,
		"image_url", imageURL,
		"task_id", input.TaskID,
	)

	return &BuildImageOutput{
		ImageURL:    imageURL,
		ImageTag:    imageTag,
		SizeBytes:   0,
		BuildTimeMs: buildTimeMs,
		ArtifactID:  artifactID,
	}, nil
}
