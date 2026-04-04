package cost

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Service struct {
	db *pgxpool.Pool
}

func NewService(db *pgxpool.Pool) *Service {
	return &Service{db: db}
}

// GetMonthlySummary returns token usage and cost for the current month.
func (s *Service) GetMonthlySummary(ctx context.Context, tenantID int64) (*CostSummary, error) {
	now := time.Now()
	startOfMonth := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())

	rows, err := s.db.Query(ctx,
		`SELECT COALESCE(provider, 'unknown'), COALESCE(model, 'unknown'),
		        COUNT(*), COALESCE(SUM(input_tokens), 0), COALESCE(SUM(output_tokens), 0)
		 FROM engine.model_calls
		 WHERE tenant_id = $1 AND created_at >= $2
		 GROUP BY provider, model
		 ORDER BY SUM(input_tokens + output_tokens) DESC`,
		tenantID, startOfMonth,
	)
	if err != nil {
		return nil, fmt.Errorf("query model calls: %w", err)
	}
	defer rows.Close()

	summary := &CostSummary{
		TenantID: tenantID,
		Period:   now.Format("2006-01"),
	}

	for rows.Next() {
		var mc ModelCost
		if err := rows.Scan(&mc.Provider, &mc.Model, &mc.Calls, &mc.InputTokens, &mc.OutputTokens); err != nil {
			continue
		}
		mc.EstimatedUSD = EstimateCost(mc.Model, mc.InputTokens, mc.OutputTokens)
		summary.ByModel = append(summary.ByModel, mc)
		summary.TotalTokens += mc.InputTokens + mc.OutputTokens
		summary.InputTokens += mc.InputTokens
		summary.OutputTokens += mc.OutputTokens
		summary.TotalCalls += mc.Calls
		summary.EstimatedUSD += mc.EstimatedUSD
	}

	if summary.ByModel == nil {
		summary.ByModel = []ModelCost{}
	}

	return summary, nil
}

// GetProjectSummary returns token usage for a specific project in the current month.
func (s *Service) GetProjectSummary(ctx context.Context, tenantID, projectID int64) (*CostSummary, error) {
	now := time.Now()
	startOfMonth := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())

	rows, err := s.db.Query(ctx,
		`SELECT COALESCE(mc.provider, 'unknown'), COALESCE(mc.model, 'unknown'),
		        COUNT(*), COALESCE(SUM(mc.input_tokens), 0), COALESCE(SUM(mc.output_tokens), 0)
		 FROM engine.model_calls mc
		 JOIN engine.tasks t ON mc.task_id = t.id
		 WHERE mc.tenant_id = $1 AND t.project_id = $2 AND mc.created_at >= $3
		 GROUP BY mc.provider, mc.model
		 ORDER BY SUM(mc.input_tokens + mc.output_tokens) DESC`,
		tenantID, projectID, startOfMonth,
	)
	if err != nil {
		return nil, fmt.Errorf("query project costs: %w", err)
	}
	defer rows.Close()

	summary := &CostSummary{
		TenantID:  tenantID,
		ProjectID: &projectID,
		Period:    now.Format("2006-01"),
	}

	for rows.Next() {
		var mc ModelCost
		if err := rows.Scan(&mc.Provider, &mc.Model, &mc.Calls, &mc.InputTokens, &mc.OutputTokens); err != nil {
			continue
		}
		mc.EstimatedUSD = EstimateCost(mc.Model, mc.InputTokens, mc.OutputTokens)
		summary.ByModel = append(summary.ByModel, mc)
		summary.TotalTokens += mc.InputTokens + mc.OutputTokens
		summary.InputTokens += mc.InputTokens
		summary.OutputTokens += mc.OutputTokens
		summary.TotalCalls += mc.Calls
		summary.EstimatedUSD += mc.EstimatedUSD
	}

	if summary.ByModel == nil {
		summary.ByModel = []ModelCost{}
	}

	return summary, nil
}

// GetBudgetStatus checks current month usage against the tenant's budget.
func (s *Service) GetBudgetStatus(ctx context.Context, tenantID int64) (*BudgetStatus, error) {
	// Get budget limit
	var budget int64
	err := s.db.QueryRow(ctx,
		`SELECT COALESCE(token_budget, 0) FROM auth.tenants WHERE id = $1`,
		tenantID,
	).Scan(&budget)
	if err != nil {
		return nil, fmt.Errorf("get tenant budget: %w", err)
	}

	// Get current month usage
	now := time.Now()
	startOfMonth := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
	var used int64
	err = s.db.QueryRow(ctx,
		`SELECT COALESCE(SUM(input_tokens + output_tokens), 0)
		 FROM engine.model_calls
		 WHERE tenant_id = $1 AND created_at >= $2`,
		tenantID, startOfMonth,
	).Scan(&used)
	if err != nil {
		return nil, fmt.Errorf("get usage: %w", err)
	}

	status := &BudgetStatus{
		TenantID:      tenantID,
		MonthlyBudget: budget,
		UsedTokens:    used,
	}

	if budget > 0 {
		status.RemainingPct = 1.0 - float64(used)/float64(budget)
		if status.RemainingPct < 0 {
			status.RemainingPct = 0
		}
		status.IsExceeded = used >= budget
	} else {
		status.RemainingPct = 1.0 // unlimited
	}

	return status, nil
}
