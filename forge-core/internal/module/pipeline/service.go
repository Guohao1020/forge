package pipeline

import (
	"context"
	"errors"

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
