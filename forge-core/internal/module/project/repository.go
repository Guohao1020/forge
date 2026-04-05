package project

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

func (r *Repository) Create(ctx context.Context, tenantID, userID int64, req *CreateProjectRequest) (*Project, error) {
	branch := req.DefaultBranch
	if branch == "" {
		branch = "main"
	}
	riskThreshold := 90
	if req.RiskThreshold != nil {
		riskThreshold = *req.RiskThreshold
	}
	autoMerge := true
	if req.AutoMerge != nil {
		autoMerge = *req.AutoMerge
	}

	var p Project
	err := r.db.QueryRow(ctx, `
		INSERT INTO engine.projects
			(tenant_id, name, description, code_platform, code_repo_url, default_branch, ai_model, risk_threshold, auto_merge, created_by)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)
		RETURNING id, tenant_id, name, COALESCE(description,''), status,
		          COALESCE(code_platform,''), COALESCE(code_repo_url,''),
		          default_branch, COALESCE(ai_model,''),
		          risk_threshold, auto_merge, tech_stack, created_by, created_at, updated_at`,
		tenantID, req.Name, req.Description, req.CodePlatform, req.CodeRepoURL,
		branch, req.AIModel, riskThreshold, autoMerge, userID,
	).Scan(
		&p.ID, &p.TenantID, &p.Name, &p.Description, &p.Status,
		&p.CodePlatform, &p.CodeRepoURL, &p.DefaultBranch, &p.AIModel,
		&p.RiskThreshold, &p.AutoMerge, &p.TechStack, &p.CreatedBy, &p.CreatedAt, &p.UpdatedAt,
	)
	return &p, err
}

func (r *Repository) List(ctx context.Context, tenantID, userID int64, q *ListProjectsQuery) ([]*Project, int64, error) {
	if q.Page <= 0 {
		q.Page = 1
	}
	if q.Size <= 0 || q.Size > 100 {
		q.Size = 20
	}
	offset := (q.Page - 1) * q.Size

	where := "WHERE p.tenant_id = $1 AND p.status = 'ACTIVE'"
	args := []interface{}{tenantID}
	argIdx := 2

	if q.Search != "" {
		where += fmt.Sprintf(" AND (p.name ILIKE $%d OR p.description ILIKE $%d)", argIdx, argIdx)
		args = append(args, "%"+q.Search+"%")
		argIdx++
	}
	if q.Starred {
		where += fmt.Sprintf(" AND EXISTS (SELECT 1 FROM engine.project_stars ps WHERE ps.project_id = p.id AND ps.user_id = $%d)", argIdx)
		args = append(args, userID)
		argIdx++
	}

	var total int64
	err := r.db.QueryRow(ctx, "SELECT COUNT(*) FROM engine.projects p "+where, args...).Scan(&total)
	if err != nil {
		return nil, 0, err
	}

	starredArg := argIdx
	args = append(args, userID, q.Size, offset)

	rows, err := r.db.Query(ctx, fmt.Sprintf(`
		SELECT p.id, p.tenant_id, p.name, COALESCE(p.description,''), p.status,
		       COALESCE(p.code_platform,''), COALESCE(p.code_repo_url,''),
		       p.default_branch, COALESCE(p.ai_model,''),
		       p.risk_threshold, p.auto_merge, p.tech_stack, p.created_by, p.created_at, p.updated_at,
		       EXISTS (SELECT 1 FROM engine.project_stars ps WHERE ps.project_id = p.id AND ps.user_id = $%d) AS starred
		FROM engine.projects p
		%s
		ORDER BY starred DESC, p.updated_at DESC
		LIMIT $%d OFFSET $%d`,
		starredArg, where, starredArg+1, starredArg+2,
	), args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var projects []*Project
	for rows.Next() {
		var p Project
		if err := rows.Scan(
			&p.ID, &p.TenantID, &p.Name, &p.Description, &p.Status,
			&p.CodePlatform, &p.CodeRepoURL, &p.DefaultBranch, &p.AIModel,
			&p.RiskThreshold, &p.AutoMerge, &p.TechStack, &p.CreatedBy, &p.CreatedAt, &p.UpdatedAt,
			&p.Starred,
		); err != nil {
			return nil, 0, err
		}
		projects = append(projects, &p)
	}
	return projects, total, rows.Err()
}

func (r *Repository) GetByID(ctx context.Context, id, tenantID, userID int64) (*Project, error) {
	var p Project
	err := r.db.QueryRow(ctx, `
		SELECT p.id, p.tenant_id, p.name, COALESCE(p.description,''), p.status,
		       COALESCE(p.code_platform,''), COALESCE(p.code_repo_url,''),
		       p.default_branch, COALESCE(p.ai_model,''),
		       p.risk_threshold, p.auto_merge, p.tech_stack, p.created_by, p.created_at, p.updated_at,
		       EXISTS (SELECT 1 FROM engine.project_stars ps WHERE ps.project_id = p.id AND ps.user_id = $3) AS starred
		FROM engine.projects p
		WHERE p.id = $1 AND p.tenant_id = $2 AND p.status != 'ARCHIVED'`,
		id, tenantID, userID,
	).Scan(
		&p.ID, &p.TenantID, &p.Name, &p.Description, &p.Status,
		&p.CodePlatform, &p.CodeRepoURL, &p.DefaultBranch, &p.AIModel,
		&p.RiskThreshold, &p.AutoMerge, &p.TechStack, &p.CreatedBy, &p.CreatedAt, &p.UpdatedAt,
		&p.Starred,
	)
	return &p, err
}

func (r *Repository) Update(ctx context.Context, id, tenantID int64, req *UpdateProjectRequest) (*Project, error) {
	var p Project
	err := r.db.QueryRow(ctx, `
		UPDATE engine.projects SET
			name           = COALESCE($3, name),
			description    = COALESCE($4, description),
			default_branch = COALESCE($5, default_branch),
			code_platform  = COALESCE($6, code_platform),
			code_repo_url  = COALESCE($7, code_repo_url),
			updated_at     = NOW()
		WHERE id = $1 AND tenant_id = $2 AND status != 'ARCHIVED'
		RETURNING id, tenant_id, name, COALESCE(description,''), status,
		          COALESCE(code_platform,''), COALESCE(code_repo_url,''),
		          default_branch, COALESCE(ai_model,''),
		          risk_threshold, auto_merge, created_by, created_at, updated_at`,
		id, tenantID, req.Name, req.Description, req.DefaultBranch,
		req.CodePlatform, req.CodeRepoURL,
	).Scan(
		&p.ID, &p.TenantID, &p.Name, &p.Description, &p.Status,
		&p.CodePlatform, &p.CodeRepoURL, &p.DefaultBranch, &p.AIModel,
		&p.RiskThreshold, &p.AutoMerge, &p.CreatedBy, &p.CreatedAt, &p.UpdatedAt,
	)
	return &p, err
}

func (r *Repository) UpdateTechStack(ctx context.Context, projectID int64, techStack string) error {
	_, err := r.db.Exec(ctx,
		`UPDATE engine.projects SET tech_stack = $2::jsonb, updated_at = NOW() WHERE id = $1`,
		projectID, techStack,
	)
	if err != nil {
		return fmt.Errorf("update tech stack: %w", err)
	}
	return nil
}

// GetByIDIncludingArchived returns a project regardless of status (including ARCHIVED).
// Used before deletion to fetch project info for confirmation and GitHub cleanup.
func (r *Repository) GetByIDIncludingArchived(ctx context.Context, id, tenantID int64) (*Project, error) {
	var p Project
	err := r.db.QueryRow(ctx, `
		SELECT p.id, p.tenant_id, p.name, COALESCE(p.description,''), p.status,
		       COALESCE(p.code_platform,''), COALESCE(p.code_repo_url,''),
		       p.default_branch, COALESCE(p.ai_model,''),
		       p.risk_threshold, p.auto_merge, p.tech_stack, p.created_by, p.created_at, p.updated_at,
		       false AS starred
		FROM engine.projects p
		WHERE p.id = $1 AND p.tenant_id = $2`,
		id, tenantID,
	).Scan(
		&p.ID, &p.TenantID, &p.Name, &p.Description, &p.Status,
		&p.CodePlatform, &p.CodeRepoURL, &p.DefaultBranch, &p.AIModel,
		&p.RiskThreshold, &p.AutoMerge, &p.TechStack, &p.CreatedBy, &p.CreatedAt, &p.UpdatedAt,
		&p.Starred,
	)
	return &p, err
}

// HardDelete permanently removes a project and all its child records (via ON DELETE CASCADE).
func (r *Repository) HardDelete(ctx context.Context, id, tenantID int64) error {
	tag, err := r.db.Exec(ctx,
		`DELETE FROM engine.projects WHERE id = $1 AND tenant_id = $2`,
		id, tenantID,
	)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("project not found")
	}
	return nil
}

func (r *Repository) Archive(ctx context.Context, id, tenantID int64) error {
	tag, err := r.db.Exec(ctx,
		`UPDATE engine.projects SET status = 'ARCHIVED', updated_at = NOW() WHERE id = $1 AND tenant_id = $2`,
		id, tenantID,
	)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("project not found")
	}
	return nil
}

func (r *Repository) Star(ctx context.Context, projectID, userID int64) error {
	_, err := r.db.Exec(ctx,
		`INSERT INTO engine.project_stars (project_id, user_id) VALUES ($1, $2) ON CONFLICT DO NOTHING`,
		projectID, userID,
	)
	return err
}

func (r *Repository) Unstar(ctx context.Context, projectID, userID int64) error {
	_, err := r.db.Exec(ctx,
		`DELETE FROM engine.project_stars WHERE project_id = $1 AND user_id = $2`,
		projectID, userID,
	)
	return err
}

// CreateFromImport creates a project from an imported GitHub repo.
// Returns nil if a project with the same name already exists in the tenant (skip).
func (r *Repository) CreateFromImport(ctx context.Context, tenantID, userID int64, item *ImportRepoItem) (*ProjectBrief, error) {
	branch := item.DefaultBranch
	if branch == "" {
		branch = "main"
	}
	var id int64
	err := r.db.QueryRow(ctx,
		`INSERT INTO engine.projects (tenant_id, name, description, status, code_platform, code_repo_url, default_branch, created_by)
		 VALUES ($1, $2, $3, 'ACTIVE', 'github', $4, $5, $6)
		 ON CONFLICT (tenant_id, name) DO NOTHING
		 RETURNING id`,
		tenantID, item.Name, item.Description, item.HTMLURL, branch, userID,
	).Scan(&id)
	if err != nil {
		if err.Error() == "no rows in result set" {
			return nil, nil
		}
		return nil, fmt.Errorf("create project from import: %w", err)
	}

	return &ProjectBrief{
		ID:            id,
		Name:          item.Name,
		CodeRepoURL:   item.HTMLURL,
		DefaultBranch: branch,
	}, nil
}

// GetProjectStats returns task and version statistics for a project.
func (r *Repository) GetProjectStats(ctx context.Context, projectID int64) (*ProjectStats, error) {
	stats := &ProjectStats{
		TasksByStatus: make(map[string]int64),
	}

	// Task counts by status
	rows, err := r.db.Query(ctx,
		`SELECT COALESCE(status, 'UNKNOWN'), COUNT(*)
		 FROM engine.tasks
		 WHERE project_id = $1
		 GROUP BY status`,
		projectID,
	)
	if err != nil {
		return stats, nil // return empty stats on error
	}
	defer rows.Close()

	for rows.Next() {
		var status string
		var count int64
		if err := rows.Scan(&status, &count); err != nil {
			continue
		}
		stats.TasksByStatus[status] = count
		stats.TotalTasks += count
		if status == "COMPLETED" {
			stats.CompletedTasks = count
		}
	}

	// Active versions count
	r.db.QueryRow(ctx,
		`SELECT COUNT(*) FROM engine.project_versions
		 WHERE project_id = $1 AND status IN ('PLANNING', 'IN_PROGRESS', 'TESTING')`,
		projectID,
	).Scan(&stats.ActiveVersions)

	// Last activity timestamp
	var lastActivity string
	err = r.db.QueryRow(ctx,
		`SELECT COALESCE(TO_CHAR(MAX(updated_at), 'YYYY-MM-DD"T"HH24:MI:SS"Z"'), '')
		 FROM engine.tasks WHERE project_id = $1`,
		projectID,
	).Scan(&lastActivity)
	if err == nil && lastActivity != "" {
		stats.LastActivity = &lastActivity
	}

	// Quality score (latest entropy scan)
	var score int
	err = r.db.QueryRow(ctx,
		`SELECT score FROM engine.entropy_scans
		 WHERE project_id = $1 ORDER BY scanned_at DESC LIMIT 1`,
		projectID,
	).Scan(&score)
	if err == nil {
		stats.QualityScore = &score
	}

	return stats, nil
}
