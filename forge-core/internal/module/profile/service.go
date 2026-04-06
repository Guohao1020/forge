package profile

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5"
	"go.temporal.io/sdk/client"
)

type Service struct {
	repo           *Repository
	temporalClient client.Client
}

func NewService(repo *Repository, temporalClient client.Client) *Service {
	return &Service{repo: repo, temporalClient: temporalClient}
}

// TriggerScan starts Temporal workflows to scan project profile via AI Worker.
// If branches is empty, scans the default branch.
func (s *Service) TriggerScan(ctx context.Context, projectID int64, userID int64, keys []string, branches []string) ([]string, error) {
	if s.temporalClient == nil {
		return nil, fmt.Errorf("Temporal client not available")
	}

	// Default: scan with empty branch (project default)
	if len(branches) == 0 {
		branches = []string{""}
	}

	var workflowIDs []string
	for _, branch := range branches {
		input := map[string]interface{}{
			"project_id": projectID,
			"user_id":    userID,
		}
		if len(keys) > 0 {
			input["keys"] = keys
		}
		if branch != "" {
			input["branch"] = branch
		}

		workflowID := fmt.Sprintf("profile-scan-%d-%d", projectID, time.Now().UnixNano())
		opts := client.StartWorkflowOptions{
			ID:        workflowID,
			TaskQueue: "forge-task-queue",
		}

		we, err := s.temporalClient.ExecuteWorkflow(ctx, opts, "ProfileScanWorkflow", input)
		if err != nil {
			slog.Error("failed to start ProfileScanWorkflow", "project_id", projectID, "branch", branch, "error", err)
			continue
		}

		slog.Info("ProfileScanWorkflow started", "project_id", projectID, "branch", branch, "workflow_id", we.GetID())
		workflowIDs = append(workflowIDs, we.GetID())
	}

	if len(workflowIDs) == 0 {
		return nil, fmt.Errorf("failed to start any profile scan workflows")
	}
	return workflowIDs, nil
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
