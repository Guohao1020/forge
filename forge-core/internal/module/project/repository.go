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
		          risk_threshold, auto_merge, created_by, created_at, updated_at`,
		tenantID, req.Name, req.Description, req.CodePlatform, req.CodeRepoURL,
		branch, req.AIModel, riskThreshold, autoMerge, userID,
	).Scan(
		&p.ID, &p.TenantID, &p.Name, &p.Description, &p.Status,
		&p.CodePlatform, &p.CodeRepoURL, &p.DefaultBranch, &p.AIModel,
		&p.RiskThreshold, &p.AutoMerge, &p.CreatedBy, &p.CreatedAt, &p.UpdatedAt,
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
		       p.risk_threshold, p.auto_merge, p.created_by, p.created_at, p.updated_at,
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
			&p.RiskThreshold, &p.AutoMerge, &p.CreatedBy, &p.CreatedAt, &p.UpdatedAt,
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
		       p.risk_threshold, p.auto_merge, p.created_by, p.created_at, p.updated_at,
		       EXISTS (SELECT 1 FROM engine.project_stars ps WHERE ps.project_id = p.id AND ps.user_id = $3) AS starred
		FROM engine.projects p
		WHERE p.id = $1 AND p.tenant_id = $2 AND p.status != 'ARCHIVED'`,
		id, tenantID, userID,
	).Scan(
		&p.ID, &p.TenantID, &p.Name, &p.Description, &p.Status,
		&p.CodePlatform, &p.CodeRepoURL, &p.DefaultBranch, &p.AIModel,
		&p.RiskThreshold, &p.AutoMerge, &p.CreatedBy, &p.CreatedAt, &p.UpdatedAt,
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
			updated_at     = NOW()
		WHERE id = $1 AND tenant_id = $2 AND status != 'ARCHIVED'
		RETURNING id, tenant_id, name, COALESCE(description,''), status,
		          COALESCE(code_platform,''), COALESCE(code_repo_url,''),
		          default_branch, COALESCE(ai_model,''),
		          risk_threshold, auto_merge, created_by, created_at, updated_at`,
		id, tenantID, req.Name, req.Description, req.DefaultBranch,
	).Scan(
		&p.ID, &p.TenantID, &p.Name, &p.Description, &p.Status,
		&p.CodePlatform, &p.CodeRepoURL, &p.DefaultBranch, &p.AIModel,
		&p.RiskThreshold, &p.AutoMerge, &p.CreatedBy, &p.CreatedAt, &p.UpdatedAt,
	)
	return &p, err
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
