package pipeline

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5"
)

type Service struct {
	repo *Repository
}

func NewService(repo *Repository) *Service {
	return &Service{repo: repo}
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
	// TODO: Replace mock with real K8s deployment via Temporal workflow
	// This should:
	// 1. Pull artifact from ACR registry
	// 2. Generate K8s manifest (Deployment + Service + Ingress)
	// 3. Apply manifest to target cluster via kubectl/client-go
	// 4. Wait for rollout completion
	// 5. Update environment current_version and last_deploy_at
	slog.Info("mock deploy triggered", "projectId", projectID, "envId", envID, "version", req.Version)

	now := time.Now()
	record := &DeployRecord{
		TenantID:      tenantID,
		ProjectID:     projectID,
		EnvironmentID: envID,
		ArtifactID:    req.ArtifactID,
		Version:       req.Version,
		Status:        "DEPLOYED", // TODO: Mock — real deploy should start as PENDING, then transition
		DeployedBy:    userID,
		StartedAt:     now,
		CompletedAt:   &now,
	}

	if err := s.repo.CreateDeployRecord(ctx, record); err != nil {
		return nil, err
	}
	return record, nil
}
