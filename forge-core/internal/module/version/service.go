package version

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"regexp"
	"strings"

	"github.com/jackc/pgx/v5"
)

// ScanTrigger abstracts scan triggering to avoid circular deps.
type ScanTrigger interface {
	TriggerProfileScan(ctx context.Context, projectID, userID int64, branches []string) error
	TriggerEntropyScan(ctx context.Context, projectID, tenantID int64, branches []string) error
}

type Service struct {
	repo    *Repository
	scanner ScanTrigger
}

func NewService(repo *Repository) *Service {
	return &Service{repo: repo}
}

// SetScanTrigger injects the scan trigger after construction (avoids circular deps).
func (s *Service) SetScanTrigger(st ScanTrigger) {
	s.scanner = st
}

// semverRegex matches patterns like "1.0", "1.2.0", "v1.0", "v2.1.3"
var semverRegex = regexp.MustCompile(`^v?\d+\.\d+(\.\d+)?$`)

// Create creates a new project version.
func (s *Service) Create(ctx context.Context, tenantID, projectID, userID int64, req *CreateVersionRequest) (*ProjectVersion, error) {
	ver := strings.TrimSpace(req.Version)
	if ver == "" {
		return nil, errors.New("version number is required")
	}
	if !semverRegex.MatchString(ver) {
		return nil, fmt.Errorf("invalid version format: %q (expected: v1.2 or 1.2.0)", ver)
	}
	// Normalize: ensure "v" prefix
	if !strings.HasPrefix(ver, "v") {
		ver = "v" + ver
	}

	v := &ProjectVersion{
		TenantID:    tenantID,
		ProjectID:   projectID,
		Version:     ver,
		Status:      StatusPlanning,
		Description: strings.TrimSpace(req.Description),
		CreatedBy:   userID,
	}
	if err := s.repo.Create(ctx, v); err != nil {
		if strings.Contains(err.Error(), "uq_project_version") {
			return nil, fmt.Errorf("version %s already exists for this project", ver)
		}
		return nil, fmt.Errorf("create version: %w", err)
	}
	return v, nil
}

// List returns all versions for a project.
func (s *Service) List(ctx context.Context, tenantID, projectID int64) ([]ProjectVersion, error) {
	return s.repo.ListByProject(ctx, projectID, tenantID)
}

// Get returns a single version with its tasks.
func (s *Service) Get(ctx context.Context, tenantID, versionID int64) (*VersionDetailResponse, error) {
	v, err := s.repo.GetByID(ctx, versionID, tenantID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, errors.New("version not found")
	}
	if err != nil {
		return nil, fmt.Errorf("get version: %w", err)
	}
	tasks, err := s.repo.GetTasksByVersion(ctx, versionID, tenantID)
	if err != nil {
		return nil, fmt.Errorf("get version tasks: %w", err)
	}
	return &VersionDetailResponse{
		Version: *v,
		Tasks:   tasks,
	}, nil
}

// Update modifies a version's description and/or status.
func (s *Service) Update(ctx context.Context, tenantID, versionID int64, req *UpdateVersionRequest) error {
	// Validate status transition
	if req.Status != nil {
		v, err := s.repo.GetByID(ctx, versionID, tenantID)
		if err != nil {
			return fmt.Errorf("version not found: %w", err)
		}
		if err := s.validateStatusTransition(v.Status, *req.Status); err != nil {
			return err
		}
	}
	return s.repo.Update(ctx, versionID, tenantID, req.Description, req.Status)
}

// Release marks a version as released. All tasks must be COMPLETED.
func (s *Service) Release(ctx context.Context, tenantID, versionID int64) (*ProjectVersion, error) {
	v, err := s.repo.GetByID(ctx, versionID, tenantID)
	if err != nil {
		return nil, fmt.Errorf("version not found: %w", err)
	}
	if v.Status != StatusTesting && v.Status != StatusInProgress {
		return nil, fmt.Errorf("cannot release version in %s status (must be TESTING or IN_PROGRESS)", v.Status)
	}

	// Check all tasks are completed
	tasks, err := s.repo.GetTasksByVersion(ctx, versionID, tenantID)
	if err != nil {
		return nil, fmt.Errorf("get tasks: %w", err)
	}
	if len(tasks) == 0 {
		return nil, errors.New("cannot release a version with no tasks")
	}
	for _, t := range tasks {
		if t.Status != "COMPLETED" {
			return nil, fmt.Errorf("cannot release: task %d (%s) is in %s status", t.ID, t.Title, t.Status)
		}
	}

	// Auto-generate changelog from task descriptions
	var changelogParts []string
	for _, t := range tasks {
		title := t.Title
		if title == "" {
			title = fmt.Sprintf("Task #%d", t.ID)
		}
		changelogParts = append(changelogParts, fmt.Sprintf("- %s (#%d)", title, t.ID))
	}
	changelog := strings.Join(changelogParts, "\n")

	// Save changelog to version description if not already set
	if v.Description == "" {
		_ = s.repo.Update(ctx, versionID, tenantID, &changelog, nil)
	}

	// Create git tag
	gitTag := v.Version
	if err := s.repo.Release(ctx, versionID, tenantID, gitTag); err != nil {
		return nil, fmt.Errorf("release version: %w", err)
	}

	// Refetch to return updated version
	return s.repo.GetByID(ctx, versionID, tenantID)
}

// AssignTask assigns a task to a version.
func (s *Service) AssignTask(ctx context.Context, tenantID, taskID, versionID int64) error {
	// Verify version exists
	_, err := s.repo.GetByID(ctx, versionID, tenantID)
	if err != nil {
		return fmt.Errorf("version not found: %w", err)
	}
	return s.repo.AssignTaskToVersion(ctx, taskID, versionID, tenantID)
}

// validateStatusTransition checks that a status transition is valid.
func (s *Service) validateStatusTransition(from, to string) error {
	allowed := map[string][]string{
		StatusPlanning:   {StatusInProgress, StatusCancelled},
		StatusInProgress: {StatusTesting, StatusCancelled},
		StatusTesting:    {StatusReleased, StatusInProgress, StatusCancelled},
		// RELEASED and CANCELLED are terminal
	}
	validTargets, ok := allowed[from]
	if !ok {
		return fmt.Errorf("version in %s status cannot be changed", from)
	}
	for _, valid := range validTargets {
		if to == valid {
			return nil
		}
	}
	return fmt.Errorf("invalid transition: %s -> %s", from, to)
}

// TriggerVersionScan triggers profile + entropy scans for all branches in a version's tasks.
func (s *Service) TriggerVersionScan(ctx context.Context, tenantID, projectID, versionID, userID int64) ([]string, error) {
	if s.scanner == nil {
		return nil, fmt.Errorf("scan trigger not configured")
	}

	tasks, err := s.repo.GetTasksByVersion(ctx, versionID, tenantID)
	if err != nil {
		return nil, fmt.Errorf("get version tasks: %w", err)
	}

	// Collect unique branch names
	branchSet := make(map[string]bool)
	for _, t := range tasks {
		if t.BranchName != "" {
			branchSet[t.BranchName] = true
		}
	}

	if len(branchSet) == 0 {
		return nil, fmt.Errorf("no branches found in version tasks")
	}

	var branches []string
	for b := range branchSet {
		branches = append(branches, b)
	}

	slog.Info("triggering version scan",
		"version_id", versionID,
		"branches", branches,
	)

	// Trigger both profile and entropy scans
	var triggered []string
	if err := s.scanner.TriggerProfileScan(ctx, projectID, userID, branches); err != nil {
		slog.Warn("profile scan trigger failed", "error", err)
	} else {
		triggered = append(triggered, "profile")
	}
	if err := s.scanner.TriggerEntropyScan(ctx, projectID, tenantID, branches); err != nil {
		slog.Warn("entropy scan trigger failed", "error", err)
	} else {
		triggered = append(triggered, "entropy")
	}

	return triggered, nil
}
