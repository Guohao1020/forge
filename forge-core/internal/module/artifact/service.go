package artifact

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

func (s *Service) ListArtifacts(ctx context.Context, projectID int64) ([]Artifact, error) {
	arts, err := s.repo.ListByProject(ctx, projectID)
	if err != nil {
		return nil, err
	}
	if arts == nil {
		arts = []Artifact{}
	}
	return arts, nil
}

func (s *Service) GetArtifact(ctx context.Context, id int64) (*Artifact, error) {
	a, err := s.repo.GetByID(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, errors.New("制品不存在")
	}
	return a, err
}
