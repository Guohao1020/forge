package testresult

import (
	"context"
)

type Service struct {
	repo *Repository
}

func NewService(repo *Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) ListByTask(ctx context.Context, taskID int64) ([]TestResult, error) {
	results, err := s.repo.ListByTask(ctx, taskID)
	if err != nil {
		return nil, err
	}
	if results == nil {
		results = []TestResult{}
	}
	return results, nil
}

func (s *Service) Create(ctx context.Context, req CreateTestResultRequest) (*TestResult, error) {
	if req.Report == "" {
		req.Report = "{}"
	}
	if req.Status == "" {
		req.Status = StatusPending
	}
	return s.repo.Create(ctx, req)
}
