package preview

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/shulex/forge/forge-core/internal/k8s"
)

type Service struct {
	repo      *Repository
	k8sClient *k8s.Client // optional — nil means mock mode
	nodeIP    string      // K8s node IP for NodePort URLs
}

func NewService(repo *Repository, k8sClient *k8s.Client, nodeIP string) *Service {
	return &Service{repo: repo, k8sClient: k8sClient, nodeIP: nodeIP}
}

func (s *Service) ListPreviews(ctx context.Context, projectID int64) ([]PreviewEnvironment, error) {
	envs, err := s.repo.ListByProject(ctx, projectID)
	if err != nil {
		return nil, err
	}
	if envs == nil {
		envs = []PreviewEnvironment{}
	}
	return envs, nil
}

func (s *Service) GetPreviewByTaskID(ctx context.Context, taskID int64) (*PreviewEnvironment, error) {
	e, err := s.repo.GetByTaskID(ctx, taskID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil // no preview exists yet — not an error
	}
	return e, err
}

// CreatePreview creates a preview environment for a task.
// When K8s is available, it creates a real namespace and deploys via NodePort.
// Otherwise it falls back to a mock URL.
func (s *Service) CreatePreview(ctx context.Context, tenantID, projectID, taskID int64, branchName string, prNumber int) (*PreviewEnvironment, error) {
	namespace := fmt.Sprintf("preview-%d", taskID)
	expiresAt := time.Now().Add(30 * time.Minute)
	var previewURL string

	if s.k8sClient != nil {
		// Real K8s preview environment
		slog.Info("creating k8s preview environment", "task_id", taskID, "namespace", namespace)

		if err := s.k8sClient.EnsureNamespace(ctx, namespace, map[string]string{
			"app":        "forge",
			"component":  "preview",
			"tenant":     fmt.Sprintf("%d", tenantID),
			"task":       fmt.Sprintf("%d", taskID),
			"managed-by": "forge",
		}); err != nil {
			slog.Warn("k8s preview namespace creation failed, falling back to mock", "error", err)
			previewURL = fmt.Sprintf("https://%d.preview.forge.example.com", taskID)
		} else {
			// Deploy a placeholder service into the preview namespace
			deployName := fmt.Sprintf("preview-%d", taskID)
			// Use a simple nginx image as placeholder until real image is built
			image := "nginx:alpine"

			if depErr := s.k8sClient.ApplyDeployment(ctx, namespace, deployName, image, 80, 1, map[string]string{
				"FORGE_TASK_ID": fmt.Sprintf("%d", taskID),
			}); depErr != nil {
				slog.Warn("k8s preview deployment failed", "error", depErr)
				previewURL = fmt.Sprintf("https://%d.preview.forge.example.com", taskID)
			} else {
				// Create NodePort service to expose the preview
				nodePort, svcErr := s.k8sClient.ApplyNodePortService(ctx, namespace, deployName, 80, 80)
				if svcErr != nil {
					slog.Warn("k8s preview nodeport service failed", "error", svcErr)
					previewURL = fmt.Sprintf("https://%d.preview.forge.example.com", taskID)
				} else {
					nodeIP := s.nodeIP
					if nodeIP == "" {
						nodeIP = "localhost"
					}
					previewURL = fmt.Sprintf("http://%s:%d", nodeIP, nodePort)
					slog.Info("k8s preview environment ready",
						"task_id", taskID,
						"namespace", namespace,
						"url", previewURL,
					)
				}
			}
		}
	} else {
		// Mock mode
		previewURL = fmt.Sprintf("https://%d.preview.forge.example.com", taskID)
	}

	env := &PreviewEnvironment{
		TenantID:   tenantID,
		ProjectID:  projectID,
		TaskID:     &taskID,
		BranchName: &branchName,
		PRNumber:   &prNumber,
		PreviewURL: &previewURL,
		Status:     "READY",
		Namespace:  &namespace,
		ExpiresAt:  &expiresAt,
	}

	created, err := s.repo.Create(ctx, env)
	if err != nil {
		return nil, fmt.Errorf("create preview: %w", err)
	}
	return created, nil
}

// DestroyPreview marks a preview environment as destroyed.
// When K8s is available, also deletes the real namespace.
func (s *Service) DestroyPreview(ctx context.Context, previewID int64) error {
	if s.k8sClient != nil {
		// Try to get the preview to find its namespace before deleting the record
		env, err := s.repo.GetByID(ctx, previewID)
		if err == nil && env.Namespace != nil && *env.Namespace != "" {
			if nsErr := s.k8sClient.DeleteNamespace(ctx, *env.Namespace); nsErr != nil {
				slog.Warn("failed to delete k8s preview namespace", "namespace", *env.Namespace, "error", nsErr)
			} else {
				slog.Info("k8s preview namespace deleted", "namespace", *env.Namespace)
			}
		}
	}
	return s.repo.Delete(ctx, previewID)
}
