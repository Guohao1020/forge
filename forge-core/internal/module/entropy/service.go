package entropy

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"go.temporal.io/sdk/client"

	"github.com/shulex/forge/forge-core/internal/temporal/activity"
)

type Service struct {
	db       *pgxpool.Pool
	temporal client.Client
}

func NewService(db *pgxpool.Pool) *Service {
	return &Service{db: db}
}

func (s *Service) SetTemporalClient(c client.Client) {
	s.temporal = c
}

// GetLatestScan returns the most recent entropy scan for a project.
func (s *Service) GetLatestScan(ctx context.Context, projectID int64) (*EntropyScan, error) {
	var scan EntropyScan
	err := s.db.QueryRow(ctx,
		`SELECT id, project_id, score, issue_count, issues::text, scanned_at
		 FROM engine.entropy_scans
		 WHERE project_id = $1
		 ORDER BY scanned_at DESC LIMIT 1`,
		projectID,
	).Scan(&scan.ID, &scan.ProjectID, &scan.Score, &scan.IssueCount, &scan.Issues, &scan.ScannedAt)
	if err != nil {
		return nil, err
	}
	return &scan, nil
}

// ListScans returns recent scan history for a project.
func (s *Service) ListScans(ctx context.Context, projectID int64, limit int) ([]EntropyScan, error) {
	if limit <= 0 || limit > 50 {
		limit = 20
	}

	rows, err := s.db.Query(ctx,
		`SELECT id, project_id, score, issue_count, issues::text, scanned_at
		 FROM engine.entropy_scans
		 WHERE project_id = $1
		 ORDER BY scanned_at DESC
		 LIMIT $2`,
		projectID, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var scans []EntropyScan
	for rows.Next() {
		var s EntropyScan
		if err := rows.Scan(&s.ID, &s.ProjectID, &s.Score, &s.IssueCount, &s.Issues, &s.ScannedAt); err != nil {
			continue
		}
		scans = append(scans, s)
	}
	if scans == nil {
		scans = []EntropyScan{}
	}
	return scans, nil
}

// GetQualityTrends returns quality score trend data points.
func (s *Service) GetQualityTrends(ctx context.Context, projectID int64, days int) ([]QualityTrend, error) {
	if days <= 0 || days > 90 {
		days = 30
	}

	rows, err := s.db.Query(ctx,
		`SELECT TO_CHAR(scanned_at, 'YYYY-MM-DD') as date, score, issue_count
		 FROM engine.entropy_scans
		 WHERE project_id = $1 AND scanned_at >= NOW() - INTERVAL '1 day' * $2
		 ORDER BY scanned_at ASC`,
		projectID, days,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var trends []QualityTrend
	for rows.Next() {
		var t QualityTrend
		if err := rows.Scan(&t.Date, &t.Score, &t.IssueCount); err != nil {
			continue
		}
		trends = append(trends, t)
	}
	if trends == nil {
		trends = []QualityTrend{}
	}
	return trends, nil
}

// GetConfig returns the entropy scan config for a project.
func (s *Service) GetConfig(ctx context.Context, projectID int64) (*EntropyConfig, error) {
	var cfg EntropyConfig
	err := s.db.QueryRow(ctx,
		`SELECT id, project_id, enabled, schedule, auto_fix, rules::text
		 FROM engine.entropy_configs
		 WHERE project_id = $1`,
		projectID,
	).Scan(&cfg.ID, &cfg.ProjectID, &cfg.Enabled, &cfg.Schedule, &cfg.AutoFix, &cfg.Rules)
	if err != nil {
		// Return default config if none exists
		return &EntropyConfig{
			ProjectID: projectID,
			Enabled:   true,
			Schedule:  "weekly",
			AutoFix:   false,
			Rules:     "[]",
		}, nil
	}
	return &cfg, nil
}

// UpdateConfig creates or updates entropy scan config.
func (s *Service) UpdateConfig(ctx context.Context, projectID, tenantID int64, req *UpdateConfigRequest) error {
	rulesJSON := "[]"
	if len(req.Rules) > 0 {
		data, _ := json.Marshal(req.Rules)
		rulesJSON = string(data)
	}

	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}
	autoFix := false
	if req.AutoFix != nil {
		autoFix = *req.AutoFix
	}
	schedule := "weekly"
	if req.Schedule != "" {
		schedule = req.Schedule
	}

	_, err := s.db.Exec(ctx,
		`INSERT INTO engine.entropy_configs (project_id, tenant_id, enabled, schedule, auto_fix, rules)
		 VALUES ($1, $2, $3, $4, $5, $6::jsonb)
		 ON CONFLICT (project_id) DO UPDATE SET
		   enabled = $3, schedule = $4, auto_fix = $5, rules = $6::jsonb, updated_at = NOW()`,
		projectID, tenantID, enabled, schedule, autoFix, rulesJSON,
	)
	return err
}

// TriggerScan starts entropy scan workflows for one or more branches.
// If branches is empty, scans the default branch (empty string = project default).
func (s *Service) TriggerScan(ctx context.Context, projectID, tenantID int64, branches []string) ([]string, error) {
	if s.temporal == nil {
		return nil, fmt.Errorf("temporal client not configured")
	}

	cfg, _ := s.GetConfig(ctx, projectID)

	var rules []string
	if cfg.Rules != "[]" {
		_ = json.Unmarshal([]byte(cfg.Rules), &rules)
	}

	// Default: scan with empty branch (project default branch)
	if len(branches) == 0 {
		branches = []string{""}
	}

	var workflowIDs []string
	for _, branch := range branches {
		input := activity.EntropyScanInput{
			ProjectID: projectID,
			TenantID:  tenantID,
			Branch:    branch,
			AutoFix:   cfg.AutoFix,
			Rules:     rules,
		}

		wfID := fmt.Sprintf("entropy-scan-%d-%d", projectID, time.Now().UnixNano())
		opts := client.StartWorkflowOptions{
			ID:        wfID,
			TaskQueue: "forge-task-queue",
		}

		run, err := s.temporal.ExecuteWorkflow(ctx, opts, "EntropyScanWorkflow", input)
		if err != nil {
			slog.Error("failed to start entropy scan", "project_id", projectID, "branch", branch, "error", err)
			continue
		}

		slog.Info("entropy scan triggered",
			"project_id", projectID,
			"branch", branch,
			"workflow_id", run.GetID(),
		)
		workflowIDs = append(workflowIDs, run.GetID())
	}

	if len(workflowIDs) == 0 {
		return nil, fmt.Errorf("failed to start any entropy scan workflows")
	}
	return workflowIDs, nil
}
