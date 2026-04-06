package activity

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"go.temporal.io/sdk/activity"

	"github.com/shulex/forge/forge-core/internal/workspace"
)

// BuildActivities handles Docker image building and artifact management.
type BuildActivities struct {
	db          *pgxpool.Pool
	ws          *workspace.Manager // workspace for git clone
	registryVPC string             // VPC registry for K8s pod pulls, e.g., "repo-voc-registry-vpc.cn-hangzhou.cr.aliyuncs.com/voc-repo"
	registryPub string             // Public registry for docker push, e.g., "repo-voc-registry.cn-hangzhou.cr.aliyuncs.com/voc-repo"
}

func NewBuildActivities(db *pgxpool.Pool, ws *workspace.Manager, registryVPC, registryPub string) *BuildActivities {
	return &BuildActivities{db: db, ws: ws, registryVPC: registryVPC, registryPub: registryPub}
}

// BuildImageInput is the input for the BuildDockerImage activity.
type BuildImageInput struct {
	TaskID      int64  `json:"task_id"`
	TenantID    int64  `json:"tenant_id"`
	ProjectID   int64  `json:"project_id"`
	CreatedBy   int64  `json:"created_by"`
	RepoURL     string `json:"repo_url"`
	Branch      string `json:"branch"`
	ProjectName string `json:"project_name"`
	Version     string `json:"version"`
	GitHubToken string `json:"github_token"` // for git clone auth
}

// BuildImageOutput is the result of a Docker build.
type BuildImageOutput struct {
	ImageURL    string `json:"image_url"`
	ImageTag    string `json:"image_tag"`
	SizeBytes   int64  `json:"size_bytes"`
	BuildTimeMs int64  `json:"build_time_ms"`
	ArtifactID  int64  `json:"artifact_id"`
}

// BuildDockerImage clones the repo, runs `docker build`, pushes to ACR,
// and saves the artifact record.
func (a *BuildActivities) BuildDockerImage(ctx context.Context, input BuildImageInput) (*BuildImageOutput, error) {
	info := activity.GetInfo(ctx)
	slog.Info("BuildDockerImage",
		"task_id", input.TaskID,
		"project", input.ProjectName,
		"branch", input.Branch,
		"workflow_id", info.WorkflowExecution.ID,
	)

	start := time.Now()

	// Generate image tag
	shortSHA := fmt.Sprintf("%x", time.Now().UnixNano())[:8]
	imageTag := fmt.Sprintf("forge-%s-%s", sanitizeK8sName(input.ProjectName), shortSHA)
	if imageTag == "forge--"+shortSHA { // empty project name (e.g., Chinese only)
		imageTag = fmt.Sprintf("forge-p%d-%s", input.ProjectID, shortSHA)
	}

	vpcRegistry := a.registryVPC
	if vpcRegistry == "" {
		vpcRegistry = "repo-voc-registry-vpc.cn-hangzhou.cr.aliyuncs.com/voc-repo"
	}
	pubRegistry := a.registryPub
	if pubRegistry == "" {
		pubRegistry = "repo-voc-registry.cn-hangzhou.cr.aliyuncs.com/voc-repo"
	}

	// K8s pods pull from VPC registry (fast, free)
	imageVPC := fmt.Sprintf("%s/forge:%s", vpcRegistry, imageTag)
	// Docker push uses public registry
	imagePub := fmt.Sprintf("%s/forge:%s", pubRegistry, imageTag)

	buildType := "placeholder"
	buildSuccess := false

	// Real build: workspace clone + docker build + docker push
	if a.ws != nil && input.RepoURL != "" {
		slog.Info("building image with Docker", "image", imagePub, "branch", input.Branch)

		// Step 1: Clone/pull repo to workspace
		defaultBranch := "main"
		repoDir, err := a.ws.EnsureClone(ctx, input.TenantID, input.ProjectID,
			input.RepoURL, input.GitHubToken, defaultBranch)
		if err != nil {
			slog.Warn("workspace clone failed", "error", err)
		} else {
			// Checkout the correct branch
			checkout := exec.CommandContext(ctx, "git", "checkout", input.Branch)
			checkout.Dir = repoDir
			if out, err := checkout.CombinedOutput(); err != nil {
				slog.Warn("git checkout failed", "branch", input.Branch, "error", err, "output", string(out))
				// Try fetch + checkout
				fetch := exec.CommandContext(ctx, "git", "fetch", "origin", input.Branch)
				fetch.Dir = repoDir
				fetch.CombinedOutput()
				checkout2 := exec.CommandContext(ctx, "git", "checkout", input.Branch)
				checkout2.Dir = repoDir
				checkout2.CombinedOutput()
			}

			// Pull latest
			pull := exec.CommandContext(ctx, "git", "pull", "origin", input.Branch)
			pull.Dir = repoDir
			pull.CombinedOutput()

			// Fix common Dockerfile issues (npm ci requires lock file)
			dockerfilePath := repoDir + "/Dockerfile"
			if dfContent, err := os.ReadFile(dockerfilePath); err == nil {
				fixed := strings.ReplaceAll(string(dfContent), "npm ci", "npm install")
				// Ensure PORT=8080 for K8s compatibility
				if !strings.Contains(fixed, "ENV PORT") {
					fixed = strings.ReplaceAll(fixed, "EXPOSE 3000", "EXPOSE 8080\nENV PORT=8080\nENV HOSTNAME=\"0.0.0.0\"")
				}
				// Use standalone output for Next.js
				if strings.Contains(fixed, "next") && !strings.Contains(fixed, "standalone") {
					fixed = strings.ReplaceAll(fixed, "COPY --from=builder /app/.next ./.next",
						"COPY --from=builder /app/.next/standalone ./\nCOPY --from=builder /app/.next/static ./.next/static")
					fixed = strings.ReplaceAll(fixed, "CMD [\"npm\", \"start\"]", "CMD [\"node\", \"server.js\"]")
				}
				os.WriteFile(dockerfilePath, []byte(fixed), 0644)
			}

			// Step 2: Docker build
			slog.Info("docker build starting", "dir", repoDir, "image", imagePub)
			buildCmd := exec.CommandContext(ctx, "docker", "build", "-t", imagePub, ".")
			buildCmd.Dir = repoDir
			buildOutput, buildErr := buildCmd.CombinedOutput()
			if buildErr != nil {
				slog.Warn("docker build failed", "error", buildErr, "output", string(buildOutput[:min(len(buildOutput), 500)]))
			} else {
				slog.Info("docker build succeeded", "image", imagePub)

				// Step 3: Docker push (public registry)
				slog.Info("docker push starting", "image", imagePub)
				pushCmd := exec.CommandContext(ctx, "docker", "push", imagePub)
				pushOutput, pushErr := pushCmd.CombinedOutput()
				if pushErr != nil {
					slog.Warn("docker push failed", "error", pushErr, "output", string(pushOutput[:min(len(pushOutput), 500)]))
				} else {
					slog.Info("docker push succeeded", "image", imagePub)
					buildType = "docker"
					buildSuccess = true
				}
			}
		}
	}

	buildTimeMs := time.Since(start).Milliseconds()

	metadata := map[string]interface{}{
		"dockerfile":   "Dockerfile",
		"build_type":   buildType,
		"registry_vpc": imageVPC,
		"registry_pub": imagePub,
		"repo_url":     input.RepoURL,
		"branch":       input.Branch,
	}
	metadataJSON, _ := json.Marshal(metadata)

	artifactStatus := "FAILED"
	if buildSuccess {
		artifactStatus = "BUILT"
	}

	var artifactID int64
	err := a.db.QueryRow(ctx,
		`INSERT INTO pipeline.artifacts (tenant_id, project_id, task_id, name, version, artifact_type, registry_url, size_bytes, metadata, status)
		 VALUES ($1, $2, $3, $4, $5, 'DOCKER_IMAGE', $6, $7, $8::jsonb, $9)
		 RETURNING id`,
		input.TenantID, input.ProjectID, input.TaskID,
		input.ProjectName, input.Version,
		imageVPC, 0,
		string(metadataJSON),
		artifactStatus,
	).Scan(&artifactID)
	if err != nil {
		return nil, fmt.Errorf("save artifact: %w", err)
	}

	slog.Info("artifact created",
		"artifact_id", artifactID,
		"image_vpc", imageVPC,
		"image_pub", imagePub,
		"build_type", buildType,
		"success", buildSuccess,
		"build_time_ms", buildTimeMs,
	)

	return &BuildImageOutput{
		ImageURL:    imageVPC, // K8s uses VPC address
		ImageTag:    imageTag,
		SizeBytes:   0,
		BuildTimeMs: buildTimeMs,
		ArtifactID:  artifactID,
	}, nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// sanitizeK8sName is imported from devops_activities.go via package scope
// but we need it here too — it's a simple string cleaner
func sanitizeBuildName(name string) string {
	name = strings.ToLower(name)
	var result []byte
	for _, c := range []byte(name) {
		if (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') || c == '-' {
			result = append(result, c)
		}
	}
	if len(result) > 30 {
		result = result[:30]
	}
	return strings.Trim(string(result), "-")
}
