package pipeline

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
}

func NewService(repo *Repository, k8sClient *k8s.Client) *Service {
	return &Service{repo: repo, k8sClient: k8sClient}
}

func (s *Service) ListEnvironments(ctx context.Context, projectID int64) ([]Environment, error) {
	envs, err := s.repo.ListByProject(ctx, projectID)
	if err != nil {
		return nil, err
	}
	if envs == nil {
		envs = []Environment{}
	}
	return envs, nil
}

func (s *Service) GetEnvironment(ctx context.Context, envID int64) (*Environment, error) {
	e, err := s.repo.GetByID(ctx, envID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, errors.New("环境不存在")
	}
	return e, err
}

// --- Deploy Records ---

func (s *Service) ListDeployRecords(ctx context.Context, environmentID int64) ([]DeployRecord, error) {
	records, err := s.repo.ListDeployRecords(ctx, environmentID)
	if err != nil {
		return nil, err
	}
	if records == nil {
		records = []DeployRecord{}
	}
	return records, nil
}

func (s *Service) TriggerDeploy(ctx context.Context, tenantID, projectID, envID, userID int64, req TriggerDeployRequest) (*DeployRecord, error) {
	// Get the environment to determine env type
	env, err := s.repo.GetByID(ctx, envID)
	if err != nil {
		return nil, fmt.Errorf("get environment: %w", err)
	}

	now := time.Now()
	record := &DeployRecord{
		TenantID:      tenantID,
		ProjectID:     projectID,
		EnvironmentID: envID,
		ArtifactID:    req.ArtifactID,
		Version:       req.Version,
		Status:        "PENDING",
		DeployedBy:    userID,
		StartedAt:     now,
	}

	if err := s.repo.CreateDeployRecord(ctx, record); err != nil {
		return nil, err
	}

	if s.k8sClient != nil {
		// Real K8s deployment
		namespace := fmt.Sprintf("tenant-%d-%s", tenantID, env.EnvType)
		deployName := fmt.Sprintf("project-%d", projectID)
		image := req.Version // Version is expected to be a full image reference when using K8s

		slog.Info("k8s deploy started",
			"namespace", namespace,
			"deployment", deployName,
			"image", image,
			"envType", env.EnvType,
		)

		// Ensure namespace exists
		if nsErr := s.k8sClient.EnsureNamespace(ctx, namespace, map[string]string{
			"app":       "forge",
			"tenant":    fmt.Sprintf("%d", tenantID),
			"env-type":  env.EnvType,
			"managed-by": "forge",
		}); nsErr != nil {
			record.Status = "FAILED"
			errMsg := fmt.Sprintf("create namespace: %v", nsErr)
			record.ErrorMessage = &errMsg
			_ = s.repo.UpdateDeployRecord(ctx, record)
			return record, fmt.Errorf("ensure namespace: %w", nsErr)
		}

		// Apply Deployment
		if depErr := s.k8sClient.ApplyDeployment(ctx, namespace, deployName, image, 8080, 1, nil); depErr != nil {
			record.Status = "FAILED"
			errMsg := fmt.Sprintf("apply deployment: %v", depErr)
			record.ErrorMessage = &errMsg
			_ = s.repo.UpdateDeployRecord(ctx, record)
			return record, fmt.Errorf("apply deployment: %w", depErr)
		}

		// Apply Service
		if svcErr := s.k8sClient.ApplyService(ctx, namespace, deployName, 80, 8080); svcErr != nil {
			record.Status = "FAILED"
			errMsg := fmt.Sprintf("apply service: %v", svcErr)
			record.ErrorMessage = &errMsg
			_ = s.repo.UpdateDeployRecord(ctx, record)
			return record, fmt.Errorf("apply service: %w", svcErr)
		}

		completedAt := time.Now()
		record.Status = "DEPLOYED"
		record.CompletedAt = &completedAt
		if updateErr := s.repo.UpdateDeployRecord(ctx, record); updateErr != nil {
			slog.Warn("failed to update deploy record status", "error", updateErr)
		}

		slog.Info("k8s deploy completed", "namespace", namespace, "deployment", deployName)
	} else {
		// Mock mode — immediately mark as deployed
		slog.Info("mock deploy triggered", "projectId", projectID, "envId", envID, "version", req.Version)
		completedAt := time.Now()
		record.Status = "DEPLOYED"
		record.CompletedAt = &completedAt
		if updateErr := s.repo.UpdateDeployRecord(ctx, record); updateErr != nil {
			slog.Warn("failed to update deploy record status", "error", updateErr)
		}
	}

	return record, nil
}
