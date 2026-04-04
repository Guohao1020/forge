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

// TriggerScan starts a Temporal workflow to scan project profile via AI Worker.
func (s *Service) TriggerScan(ctx context.Context, projectID int64, userID int64, keys []string) (string, error) {
	if s.temporalClient == nil {
		return "", fmt.Errorf("Temporal client not available")
	}

	input := map[string]interface{}{
		"project_id": projectID,
		"user_id":    userID,
	}
	if len(keys) > 0 {
		input["keys"] = keys
	}

	workflowID := fmt.Sprintf("profile-scan-%d-%d", projectID, time.Now().Unix())
	opts := client.StartWorkflowOptions{
		ID:        workflowID,
		TaskQueue: "forge-task-queue",
	}

	we, err := s.temporalClient.ExecuteWorkflow(ctx, opts, "ProfileScanWorkflow", input)
	if err != nil {
		slog.Error("failed to start ProfileScanWorkflow", "project_id", projectID, "error", err)
		return "", fmt.Errorf("failed to start profile scan: %w", err)
	}

	slog.Info("ProfileScanWorkflow started", "project_id", projectID, "workflow_id", we.GetID())

	// Fire and forget — workflow runs in background, results saved by Python activity
	return we.GetID(), nil
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
