package profile

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/jackc/pgx/v5"
)

type Service struct {
	repo *Repository
}

func NewService(repo *Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) ListProfiles(ctx context.Context, projectID int64) ([]ProfileEntry, error) {
	entries, err := s.repo.ListByProject(ctx, projectID)
	if err != nil {
		return nil, err
	}
	if entries == nil {
		entries = []ProfileEntry{}
	}
	return entries, nil
}

func (s *Service) GetProfile(ctx context.Context, projectID int64, key string) (*ProfileEntry, error) {
	e, err := s.repo.GetByKey(ctx, projectID, key)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, errors.New("profile not found")
	}
	return e, err
}

func (s *Service) SaveProfile(ctx context.Context, projectID int64, key string, value json.RawMessage) (*ProfileEntry, error) {
	entry := &ProfileEntry{
		ProjectID:    projectID,
		ProfileKey:   key,
		ProfileValue: value,
	}
	if err := s.repo.Upsert(ctx, entry); err != nil {
		return nil, err
	}
	return entry, nil
}
