package project

import (
	"context"
	"errors"
	"strings"

	"github.com/jackc/pgx/v5"
)

type Service struct {
	repo *Repository
}

func NewService(repo *Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) Create(ctx context.Context, tenantID, userID int64, req *CreateProjectRequest) (*Project, error) {
	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" {
		return nil, errors.New("项目名称不能为空")
	}
	return s.repo.Create(ctx, tenantID, userID, req)
}

func (s *Service) List(ctx context.Context, tenantID, userID int64, q *ListProjectsQuery) (*ListProjectsResponse, error) {
	projects, total, err := s.repo.List(ctx, tenantID, userID, q)
	if err != nil {
		return nil, err
	}
	if projects == nil {
		projects = []*Project{}
	}
	return &ListProjectsResponse{
		Projects: projects,
		Total:    total,
		Page:     q.Page,
		Size:     q.Size,
	}, nil
}

func (s *Service) GetByID(ctx context.Context, id, tenantID, userID int64) (*Project, error) {
	p, err := s.repo.GetByID(ctx, id, tenantID, userID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, errors.New("项目不存在")
	}
	return p, err
}

func (s *Service) Update(ctx context.Context, id, tenantID int64, req *UpdateProjectRequest) (*Project, error) {
	p, err := s.repo.Update(ctx, id, tenantID, req)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, errors.New("项目不存在")
	}
	return p, err
}

func (s *Service) Archive(ctx context.Context, id, tenantID int64) error {
	return s.repo.Archive(ctx, id, tenantID)
}

func (s *Service) Star(ctx context.Context, projectID, tenantID, userID int64) error {
	// Verify project belongs to tenant
	if _, err := s.repo.GetByID(ctx, projectID, tenantID, userID); err != nil {
		return errors.New("项目不存在")
	}
	return s.repo.Star(ctx, projectID, userID)
}

func (s *Service) Unstar(ctx context.Context, projectID, tenantID, userID int64) error {
	return s.repo.Unstar(ctx, projectID, userID)
}
