package preview

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

type Service struct {
	repo *Repository
}

func NewService(repo *Repository) *Service {
	return &Service{repo: repo}
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

// CreatePreview generates a mock preview environment.
// TODO: Replace mock URL/namespace with real K8s namespace creation when available.
func (s *Service) CreatePreview(ctx context.Context, tenantID, projectID, taskID int64, branchName string, prNumber int) (*PreviewEnvironment, error) {
	previewURL := fmt.Sprintf("https://%d.preview.forge.example.com", taskID)
	namespace := fmt.Sprintf("preview-%d", taskID)
	expiresAt := time.Now().Add(30 * time.Minute)

	env := &PreviewEnvironment{
		TenantID:   tenantID,
		ProjectID:  projectID,
		TaskID:     &taskID,
		BranchName: &branchName,
		PRNumber:   &prNumber,
		PreviewURL: &previewURL,
		Status:     "READY", // Mock: immediately ready
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
// TODO: When K8s is available, delete the actual namespace here.
func (s *Service) DestroyPreview(ctx context.Context, previewID int64) error {
	return s.repo.Delete(ctx, previewID)
}
