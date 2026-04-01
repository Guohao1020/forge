package task

import (
	"context"
	"fmt"
	"log/slog"
)

// WorkflowStarter abstracts Temporal client for starting task workflows.
type WorkflowStarter interface {
	StartTaskWorkflow(ctx context.Context, taskID, tenantID, projectID int64) (workflowID string, runID string, err error)
}

type Service struct {
	repo            *Repository
	workflowStarter WorkflowStarter
}

func NewService(repo *Repository, ws WorkflowStarter) *Service {
	return &Service{repo: repo, workflowStarter: ws}
}

func (s *Service) CreateTask(ctx context.Context, tenantID, projectID, userID int64, req *CreateTaskRequest) (*TaskResponse, error) {
	title := req.Title
	if title == "" {
		title = req.Requirement
		runes := []rune(title)
		if len(runes) > 50 {
			title = string(runes[:50]) + "..."
		}
	}

	t := &Task{
		TenantID:    tenantID,
		ProjectID:   projectID,
		Title:       &title,
		Requirement: req.Requirement,
		Source:      SourceWeb,
		Status:      StatusSubmitted,
		CreatedBy:   userID,
	}

	if err := s.repo.Create(ctx, t); err != nil {
		return nil, fmt.Errorf("create task: %w", err)
	}

	if err := s.repo.CreateSteps(ctx, t.ID, AllSteps); err != nil {
		return nil, fmt.Errorf("create steps: %w", err)
	}

	if s.workflowStarter != nil {
		workflowID, runID, err := s.workflowStarter.StartTaskWorkflow(ctx, t.ID, tenantID, projectID)
		if err != nil {
			slog.Error("failed to start workflow", "task_id", t.ID, "error", err)
		} else {
			_ = s.repo.UpdateWorkflowIDs(ctx, t.ID, workflowID, runID)
			t.WorkflowID = &workflowID
			t.WorkflowRunID = &runID
		}
	}

	steps, _ := s.repo.GetStepsByTaskID(ctx, t.ID)
	return &TaskResponse{Task: *t, Steps: steps}, nil
}

func (s *Service) GetTask(ctx context.Context, taskID int64) (*TaskResponse, error) {
	t, err := s.repo.FindByID(ctx, taskID)
	if err != nil {
		return nil, fmt.Errorf("get task: %w", err)
	}
	steps, _ := s.repo.GetStepsByTaskID(ctx, taskID)
	return &TaskResponse{Task: *t, Steps: steps}, nil
}

func (s *Service) ListTasks(ctx context.Context, projectID int64, status string, page, pageSize int) (*TaskListResponse, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	offset := (page - 1) * pageSize

	tasks, total, err := s.repo.ListByProject(ctx, projectID, status, offset, pageSize)
	if err != nil {
		return nil, err
	}
	if tasks == nil {
		tasks = []Task{}
	}
	return &TaskListResponse{Tasks: tasks, Total: total}, nil
}
