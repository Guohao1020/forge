package specs

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

// ==================== Standards ====================

func (r *Repository) ListStandards(ctx context.Context, tenantID int64, f StandardFilter) (*PageResult[*Standard], error) {
	where := "WHERE tenant_id = @tenantID AND status = 'ACTIVE'"
	args := pgx.NamedArgs{"tenantID": tenantID}

	if f.Category != "" {
		where += " AND category = @category"
		args["category"] = f.Category
	}
	if f.Scope != "" {
		where += " AND scope = @scope"
		args["scope"] = f.Scope
	}
	if f.ScopeID != nil {
		where += " AND scope_id = @scopeID"
		args["scopeID"] = *f.ScopeID
	}

	var total int64
	countSQL := fmt.Sprintf("SELECT count(*) FROM specs.standards %s", where)
	if err := r.db.QueryRow(ctx, countSQL, args).Scan(&total); err != nil {
		return nil, fmt.Errorf("count standards: %w", err)
	}

	offset := (f.Page - 1) * f.PageSize
	args["limit"] = f.PageSize
	args["offset"] = offset

	query := fmt.Sprintf(`SELECT id, tenant_id, name, category, scope, scope_id, parent_id, content, version, status, created_by, created_at, updated_at
		FROM specs.standards %s ORDER BY created_at DESC LIMIT @limit OFFSET @offset`, where)

	rows, err := r.db.Query(ctx, query, args)
	if err != nil {
		return nil, fmt.Errorf("list standards: %w", err)
	}
	defer rows.Close()

	var items []*Standard
	for rows.Next() {
		s := &Standard{}
		if err := rows.Scan(&s.ID, &s.TenantID, &s.Name, &s.Category, &s.Scope, &s.ScopeID,
			&s.ParentID, &s.Content, &s.Version, &s.Status, &s.CreatedBy, &s.CreatedAt, &s.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan standard: %w", err)
		}
		items = append(items, s)
	}
	if items == nil {
		items = []*Standard{}
	}

	return &PageResult[*Standard]{Items: items, Total: total, Page: f.Page, PageSize: f.PageSize}, nil
}

func (r *Repository) GetStandard(ctx context.Context, tenantID, id int64) (*Standard, error) {
	s := &Standard{}
	err := r.db.QueryRow(ctx, `SELECT id, tenant_id, name, category, scope, scope_id, parent_id, content, version, status, created_by, created_at, updated_at
		FROM specs.standards WHERE id = $1 AND tenant_id = $2 AND status = 'ACTIVE'`, id, tenantID).
		Scan(&s.ID, &s.TenantID, &s.Name, &s.Category, &s.Scope, &s.ScopeID,
			&s.ParentID, &s.Content, &s.Version, &s.Status, &s.CreatedBy, &s.CreatedAt, &s.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("get standard: %w", err)
	}
	return s, nil
}

func (r *Repository) CreateStandard(ctx context.Context, tenantID int64, userID int64, req CreateStandardReq) (*Standard, error) {
	s := &Standard{}
	err := r.db.QueryRow(ctx, `INSERT INTO specs.standards (tenant_id, name, category, scope, scope_id, parent_id, content, created_by)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING id, tenant_id, name, category, scope, scope_id, parent_id, content, version, status, created_by, created_at, updated_at`,
		tenantID, req.Name, req.Category, req.Scope, req.ScopeID, req.ParentID, req.Content, userID).
		Scan(&s.ID, &s.TenantID, &s.Name, &s.Category, &s.Scope, &s.ScopeID,
			&s.ParentID, &s.Content, &s.Version, &s.Status, &s.CreatedBy, &s.CreatedAt, &s.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("create standard: %w", err)
	}
	return s, nil
}

func (r *Repository) UpdateStandard(ctx context.Context, tenantID, id int64, req UpdateStandardReq) (*Standard, error) {
	s := &Standard{}
	err := r.db.QueryRow(ctx, `UPDATE specs.standards SET name = $1, content = $2, version = version + 1, updated_at = NOW()
		WHERE id = $3 AND tenant_id = $4 AND status = 'ACTIVE'
		RETURNING id, tenant_id, name, category, scope, scope_id, parent_id, content, version, status, created_by, created_at, updated_at`,
		req.Name, req.Content, id, tenantID).
		Scan(&s.ID, &s.TenantID, &s.Name, &s.Category, &s.Scope, &s.ScopeID,
			&s.ParentID, &s.Content, &s.Version, &s.Status, &s.CreatedBy, &s.CreatedAt, &s.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("update standard: %w", err)
	}
	return s, nil
}

func (r *Repository) DeleteStandard(ctx context.Context, tenantID, id int64) error {
	_, err := r.db.Exec(ctx, `UPDATE specs.standards SET status = 'DELETED', updated_at = NOW() WHERE id = $1 AND tenant_id = $2`, id, tenantID)
	if err != nil {
		return fmt.Errorf("delete standard: %w", err)
	}
	return nil
}

// GetStandardsByScope retrieves standards for a specific scope (used in inheritance resolution)
func (r *Repository) GetStandardsByScope(ctx context.Context, tenantID int64, scope string, scopeID int64) ([]*Standard, error) {
	rows, err := r.db.Query(ctx, `SELECT id, tenant_id, name, category, scope, scope_id, parent_id, content, version, status, created_by, created_at, updated_at
		FROM specs.standards WHERE tenant_id = $1 AND scope = $2 AND scope_id = $3 AND status = 'ACTIVE'
		ORDER BY category`, tenantID, scope, scopeID)
	if err != nil {
		return nil, fmt.Errorf("get standards by scope: %w", err)
	}
	defer rows.Close()

	var items []*Standard
	for rows.Next() {
		s := &Standard{}
		if err := rows.Scan(&s.ID, &s.TenantID, &s.Name, &s.Category, &s.Scope, &s.ScopeID,
			&s.ParentID, &s.Content, &s.Version, &s.Status, &s.CreatedBy, &s.CreatedAt, &s.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan standard: %w", err)
		}
		items = append(items, s)
	}
	if items == nil {
		items = []*Standard{}
	}
	return items, nil
}

// ==================== Prompt Templates ====================

func (r *Repository) ListPromptTemplates(ctx context.Context, tenantID int64, f PromptTemplateFilter) (*PageResult[*PromptTemplate], error) {
	where := "WHERE tenant_id = @tenantID"
	args := pgx.NamedArgs{"tenantID": tenantID}

	if f.Purpose != "" {
		where += " AND purpose = @purpose"
		args["purpose"] = f.Purpose
	}

	var total int64
	countSQL := fmt.Sprintf("SELECT count(*) FROM specs.prompt_templates %s", where)
	if err := r.db.QueryRow(ctx, countSQL, args).Scan(&total); err != nil {
		return nil, fmt.Errorf("count prompt templates: %w", err)
	}

	offset := (f.Page - 1) * f.PageSize
	args["limit"] = f.PageSize
	args["offset"] = offset

	query := fmt.Sprintf(`SELECT id, tenant_id, name, purpose, system_prompt, user_template, variables, version, is_default, created_by, created_at, updated_at
		FROM specs.prompt_templates %s ORDER BY is_default DESC, created_at DESC LIMIT @limit OFFSET @offset`, where)

	rows, err := r.db.Query(ctx, query, args)
	if err != nil {
		return nil, fmt.Errorf("list prompt templates: %w", err)
	}
	defer rows.Close()

	var items []*PromptTemplate
	for rows.Next() {
		p := &PromptTemplate{}
		var varsJSON []byte
		if err := rows.Scan(&p.ID, &p.TenantID, &p.Name, &p.Purpose, &p.SystemPrompt, &p.UserTemplate,
			&varsJSON, &p.Version, &p.IsDefault, &p.CreatedBy, &p.CreatedAt, &p.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan prompt template: %w", err)
		}
		if err := json.Unmarshal(varsJSON, &p.Variables); err != nil {
			p.Variables = []string{}
		}
		items = append(items, p)
	}
	if items == nil {
		items = []*PromptTemplate{}
	}

	return &PageResult[*PromptTemplate]{Items: items, Total: total, Page: f.Page, PageSize: f.PageSize}, nil
}

func (r *Repository) GetPromptTemplate(ctx context.Context, tenantID, id int64) (*PromptTemplate, error) {
	p := &PromptTemplate{}
	var varsJSON []byte
	err := r.db.QueryRow(ctx, `SELECT id, tenant_id, name, purpose, system_prompt, user_template, variables, version, is_default, created_by, created_at, updated_at
		FROM specs.prompt_templates WHERE id = $1 AND tenant_id = $2`, id, tenantID).
		Scan(&p.ID, &p.TenantID, &p.Name, &p.Purpose, &p.SystemPrompt, &p.UserTemplate,
			&varsJSON, &p.Version, &p.IsDefault, &p.CreatedBy, &p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("get prompt template: %w", err)
	}
	if err := json.Unmarshal(varsJSON, &p.Variables); err != nil {
		p.Variables = []string{}
	}
	return p, nil
}

func (r *Repository) CreatePromptTemplate(ctx context.Context, tenantID, userID int64, req CreatePromptTemplateReq) (*PromptTemplate, error) {
	varsJSON, err := json.Marshal(req.Variables)
	if err != nil {
		return nil, fmt.Errorf("marshal variables: %w", err)
	}

	p := &PromptTemplate{}
	var retVarsJSON []byte
	err = r.db.QueryRow(ctx, `INSERT INTO specs.prompt_templates (tenant_id, name, purpose, system_prompt, user_template, variables, is_default, created_by)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING id, tenant_id, name, purpose, system_prompt, user_template, variables, version, is_default, created_by, created_at, updated_at`,
		tenantID, req.Name, req.Purpose, req.SystemPrompt, req.UserTemplate, varsJSON, req.IsDefault, userID).
		Scan(&p.ID, &p.TenantID, &p.Name, &p.Purpose, &p.SystemPrompt, &p.UserTemplate,
			&retVarsJSON, &p.Version, &p.IsDefault, &p.CreatedBy, &p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("create prompt template: %w", err)
	}
	if err := json.Unmarshal(retVarsJSON, &p.Variables); err != nil {
		p.Variables = []string{}
	}
	return p, nil
}

func (r *Repository) UpdatePromptTemplate(ctx context.Context, tenantID, id int64, req UpdatePromptTemplateReq) (*PromptTemplate, error) {
	varsJSON, err := json.Marshal(req.Variables)
	if err != nil {
		return nil, fmt.Errorf("marshal variables: %w", err)
	}

	p := &PromptTemplate{}
	var retVarsJSON []byte
	err = r.db.QueryRow(ctx, `UPDATE specs.prompt_templates
		SET name = $1, purpose = $2, system_prompt = $3, user_template = $4, variables = $5, is_default = $6, version = version + 1, updated_at = NOW()
		WHERE id = $7 AND tenant_id = $8
		RETURNING id, tenant_id, name, purpose, system_prompt, user_template, variables, version, is_default, created_by, created_at, updated_at`,
		req.Name, req.Purpose, req.SystemPrompt, req.UserTemplate, varsJSON, req.IsDefault, id, tenantID).
		Scan(&p.ID, &p.TenantID, &p.Name, &p.Purpose, &p.SystemPrompt, &p.UserTemplate,
			&retVarsJSON, &p.Version, &p.IsDefault, &p.CreatedBy, &p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("update prompt template: %w", err)
	}
	if err := json.Unmarshal(retVarsJSON, &p.Variables); err != nil {
		p.Variables = []string{}
	}
	return p, nil
}

func (r *Repository) DeletePromptTemplate(ctx context.Context, tenantID, id int64) error {
	_, err := r.db.Exec(ctx, `DELETE FROM specs.prompt_templates WHERE id = $1 AND tenant_id = $2`, id, tenantID)
	if err != nil {
		return fmt.Errorf("delete prompt template: %w", err)
	}
	return nil
}

// ==================== Review Rules ====================

func (r *Repository) ListReviewRules(ctx context.Context, tenantID int64, f ReviewRuleFilter) (*PageResult[*ReviewRule], error) {
	where := "WHERE tenant_id = @tenantID"
	args := pgx.NamedArgs{"tenantID": tenantID}

	if f.Category != "" {
		where += " AND category = @category"
		args["category"] = f.Category
	}
	if f.Severity != "" {
		where += " AND severity = @severity"
		args["severity"] = f.Severity
	}
	if f.Scope != "" {
		where += " AND scope = @scope"
		args["scope"] = f.Scope
	}
	if f.ScopeID != nil {
		where += " AND scope_id = @scopeID"
		args["scopeID"] = *f.ScopeID
	}

	var total int64
	countSQL := fmt.Sprintf("SELECT count(*) FROM specs.review_rules %s", where)
	if err := r.db.QueryRow(ctx, countSQL, args).Scan(&total); err != nil {
		return nil, fmt.Errorf("count review rules: %w", err)
	}

	offset := (f.Page - 1) * f.PageSize
	args["limit"] = f.PageSize
	args["offset"] = offset

	query := fmt.Sprintf(`SELECT id, tenant_id, name, category, scope, scope_id, rule_type, definition, severity, auto_fix, fix_template, enabled, created_by, created_at, updated_at
		FROM specs.review_rules %s ORDER BY severity, created_at DESC LIMIT @limit OFFSET @offset`, where)

	rows, err := r.db.Query(ctx, query, args)
	if err != nil {
		return nil, fmt.Errorf("list review rules: %w", err)
	}
	defer rows.Close()

	var items []*ReviewRule
	for rows.Next() {
		rule := &ReviewRule{}
		var defJSON []byte
		if err := rows.Scan(&rule.ID, &rule.TenantID, &rule.Name, &rule.Category, &rule.Scope, &rule.ScopeID,
			&rule.RuleType, &defJSON, &rule.Severity, &rule.AutoFix, &rule.FixTemplate, &rule.Enabled,
			&rule.CreatedBy, &rule.CreatedAt, &rule.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan review rule: %w", err)
		}
		if err := json.Unmarshal(defJSON, &rule.Definition); err != nil {
			rule.Definition = map[string]interface{}{}
		}
		items = append(items, rule)
	}
	if items == nil {
		items = []*ReviewRule{}
	}

	return &PageResult[*ReviewRule]{Items: items, Total: total, Page: f.Page, PageSize: f.PageSize}, nil
}

func (r *Repository) GetReviewRule(ctx context.Context, tenantID, id int64) (*ReviewRule, error) {
	rule := &ReviewRule{}
	var defJSON []byte
	err := r.db.QueryRow(ctx, `SELECT id, tenant_id, name, category, scope, scope_id, rule_type, definition, severity, auto_fix, fix_template, enabled, created_by, created_at, updated_at
		FROM specs.review_rules WHERE id = $1 AND tenant_id = $2`, id, tenantID).
		Scan(&rule.ID, &rule.TenantID, &rule.Name, &rule.Category, &rule.Scope, &rule.ScopeID,
			&rule.RuleType, &defJSON, &rule.Severity, &rule.AutoFix, &rule.FixTemplate, &rule.Enabled,
			&rule.CreatedBy, &rule.CreatedAt, &rule.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("get review rule: %w", err)
	}
	if err := json.Unmarshal(defJSON, &rule.Definition); err != nil {
		rule.Definition = map[string]interface{}{}
	}
	return rule, nil
}

func (r *Repository) CreateReviewRule(ctx context.Context, tenantID, userID int64, req CreateReviewRuleReq) (*ReviewRule, error) {
	defJSON, err := json.Marshal(req.Definition)
	if err != nil {
		return nil, fmt.Errorf("marshal definition: %w", err)
	}

	rule := &ReviewRule{}
	var retDefJSON []byte
	err = r.db.QueryRow(ctx, `INSERT INTO specs.review_rules (tenant_id, name, category, scope, scope_id, rule_type, definition, severity, auto_fix, fix_template, created_by)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		RETURNING id, tenant_id, name, category, scope, scope_id, rule_type, definition, severity, auto_fix, fix_template, enabled, created_by, created_at, updated_at`,
		tenantID, req.Name, req.Category, req.Scope, req.ScopeID, req.RuleType, defJSON, req.Severity, req.AutoFix, req.FixTemplate, userID).
		Scan(&rule.ID, &rule.TenantID, &rule.Name, &rule.Category, &rule.Scope, &rule.ScopeID,
			&rule.RuleType, &retDefJSON, &rule.Severity, &rule.AutoFix, &rule.FixTemplate, &rule.Enabled,
			&rule.CreatedBy, &rule.CreatedAt, &rule.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("create review rule: %w", err)
	}
	if err := json.Unmarshal(retDefJSON, &rule.Definition); err != nil {
		rule.Definition = map[string]interface{}{}
	}
	return rule, nil
}

func (r *Repository) UpdateReviewRule(ctx context.Context, tenantID, id int64, req UpdateReviewRuleReq) (*ReviewRule, error) {
	defJSON, err := json.Marshal(req.Definition)
	if err != nil {
		return nil, fmt.Errorf("marshal definition: %w", err)
	}

	rule := &ReviewRule{}
	var retDefJSON []byte
	err = r.db.QueryRow(ctx, `UPDATE specs.review_rules
		SET name = $1, category = $2, rule_type = $3, definition = $4, severity = $5, auto_fix = $6, fix_template = $7, updated_at = NOW()
		WHERE id = $8 AND tenant_id = $9
		RETURNING id, tenant_id, name, category, scope, scope_id, rule_type, definition, severity, auto_fix, fix_template, enabled, created_by, created_at, updated_at`,
		req.Name, req.Category, req.RuleType, defJSON, req.Severity, req.AutoFix, req.FixTemplate, id, tenantID).
		Scan(&rule.ID, &rule.TenantID, &rule.Name, &rule.Category, &rule.Scope, &rule.ScopeID,
			&rule.RuleType, &retDefJSON, &rule.Severity, &rule.AutoFix, &rule.FixTemplate, &rule.Enabled,
			&rule.CreatedBy, &rule.CreatedAt, &rule.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("update review rule: %w", err)
	}
	if err := json.Unmarshal(retDefJSON, &rule.Definition); err != nil {
		rule.Definition = map[string]interface{}{}
	}
	return rule, nil
}

func (r *Repository) ToggleReviewRule(ctx context.Context, tenantID, id int64) (*ReviewRule, error) {
	rule := &ReviewRule{}
	var defJSON []byte
	err := r.db.QueryRow(ctx, `UPDATE specs.review_rules SET enabled = NOT enabled, updated_at = NOW()
		WHERE id = $1 AND tenant_id = $2
		RETURNING id, tenant_id, name, category, scope, scope_id, rule_type, definition, severity, auto_fix, fix_template, enabled, created_by, created_at, updated_at`,
		id, tenantID).
		Scan(&rule.ID, &rule.TenantID, &rule.Name, &rule.Category, &rule.Scope, &rule.ScopeID,
			&rule.RuleType, &defJSON, &rule.Severity, &rule.AutoFix, &rule.FixTemplate, &rule.Enabled,
			&rule.CreatedBy, &rule.CreatedAt, &rule.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("toggle review rule: %w", err)
	}
	if err := json.Unmarshal(defJSON, &rule.Definition); err != nil {
		rule.Definition = map[string]interface{}{}
	}
	return rule, nil
}

// GetReviewRulesByScope retrieves review rules for a specific scope (used in inheritance resolution)
func (r *Repository) GetReviewRulesByScope(ctx context.Context, tenantID int64, scope string, scopeID int64) ([]*ReviewRule, error) {
	rows, err := r.db.Query(ctx, `SELECT id, tenant_id, name, category, scope, scope_id, rule_type, definition, severity, auto_fix, fix_template, enabled, created_by, created_at, updated_at
		FROM specs.review_rules WHERE tenant_id = $1 AND scope = $2 AND scope_id = $3 AND enabled = TRUE
		ORDER BY severity, category`, tenantID, scope, scopeID)
	if err != nil {
		return nil, fmt.Errorf("get review rules by scope: %w", err)
	}
	defer rows.Close()

	var items []*ReviewRule
	for rows.Next() {
		rule := &ReviewRule{}
		var defJSON []byte
		if err := rows.Scan(&rule.ID, &rule.TenantID, &rule.Name, &rule.Category, &rule.Scope, &rule.ScopeID,
			&rule.RuleType, &defJSON, &rule.Severity, &rule.AutoFix, &rule.FixTemplate, &rule.Enabled,
			&rule.CreatedBy, &rule.CreatedAt, &rule.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan review rule: %w", err)
		}
		if err := json.Unmarshal(defJSON, &rule.Definition); err != nil {
			rule.Definition = map[string]interface{}{}
		}
		items = append(items, rule)
	}
	if items == nil {
		items = []*ReviewRule{}
	}
	return items, nil
}

// ==================== Scaffold Templates ====================

func (r *Repository) ListScaffoldTemplates(ctx context.Context, tenantID int64, f ScaffoldFilter) (*PageResult[*ScaffoldTemplate], error) {
	where := "WHERE tenant_id = @tenantID"
	args := pgx.NamedArgs{"tenantID": tenantID}

	if f.ProjectType != "" {
		where += " AND project_type = @projectType"
		args["projectType"] = f.ProjectType
	}

	var total int64
	countSQL := fmt.Sprintf("SELECT count(*) FROM specs.scaffold_templates %s", where)
	if err := r.db.QueryRow(ctx, countSQL, args).Scan(&total); err != nil {
		return nil, fmt.Errorf("count scaffold templates: %w", err)
	}

	offset := (f.Page - 1) * f.PageSize
	args["limit"] = f.PageSize
	args["offset"] = offset

	query := fmt.Sprintf(`SELECT id, tenant_id, name, project_type, description, template_repo, variables, post_hooks, version, created_by, created_at, updated_at
		FROM specs.scaffold_templates %s ORDER BY created_at DESC LIMIT @limit OFFSET @offset`, where)

	rows, err := r.db.Query(ctx, query, args)
	if err != nil {
		return nil, fmt.Errorf("list scaffold templates: %w", err)
	}
	defer rows.Close()

	var items []*ScaffoldTemplate
	for rows.Next() {
		st := &ScaffoldTemplate{}
		var varsJSON, hooksJSON []byte
		if err := rows.Scan(&st.ID, &st.TenantID, &st.Name, &st.ProjectType, &st.Description, &st.TemplateRepo,
			&varsJSON, &hooksJSON, &st.Version, &st.CreatedBy, &st.CreatedAt, &st.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan scaffold template: %w", err)
		}
		if err := json.Unmarshal(varsJSON, &st.Variables); err != nil {
			st.Variables = []string{}
		}
		if err := json.Unmarshal(hooksJSON, &st.PostHooks); err != nil {
			st.PostHooks = []string{}
		}
		items = append(items, st)
	}
	if items == nil {
		items = []*ScaffoldTemplate{}
	}

	return &PageResult[*ScaffoldTemplate]{Items: items, Total: total, Page: f.Page, PageSize: f.PageSize}, nil
}

func (r *Repository) GetScaffoldTemplate(ctx context.Context, tenantID, id int64) (*ScaffoldTemplate, error) {
	st := &ScaffoldTemplate{}
	var varsJSON, hooksJSON []byte
	err := r.db.QueryRow(ctx, `SELECT id, tenant_id, name, project_type, description, template_repo, variables, post_hooks, version, created_by, created_at, updated_at
		FROM specs.scaffold_templates WHERE id = $1 AND tenant_id = $2`, id, tenantID).
		Scan(&st.ID, &st.TenantID, &st.Name, &st.ProjectType, &st.Description, &st.TemplateRepo,
			&varsJSON, &hooksJSON, &st.Version, &st.CreatedBy, &st.CreatedAt, &st.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("get scaffold template: %w", err)
	}
	if err := json.Unmarshal(varsJSON, &st.Variables); err != nil {
		st.Variables = []string{}
	}
	if err := json.Unmarshal(hooksJSON, &st.PostHooks); err != nil {
		st.PostHooks = []string{}
	}
	return st, nil
}
