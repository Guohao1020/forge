package cost

import "time"

// CostSummary represents aggregated token usage and estimated cost.
type CostSummary struct {
	TenantID     int64   `json:"tenantId"`
	ProjectID    *int64  `json:"projectId,omitempty"` // nil = tenant-wide
	Period       string  `json:"period"`              // "2026-04", "2026-04-05"
	TotalTokens  int64   `json:"totalTokens"`
	InputTokens  int64   `json:"inputTokens"`
	OutputTokens int64   `json:"outputTokens"`
	TotalCalls   int64   `json:"totalCalls"`
	EstimatedUSD float64 `json:"estimatedUsd"`
	ByModel      []ModelCost `json:"byModel"`
}

// ModelCost breaks down usage by LLM model.
type ModelCost struct {
	Provider     string  `json:"provider"`
	Model        string  `json:"model"`
	Calls        int64   `json:"calls"`
	InputTokens  int64   `json:"inputTokens"`
	OutputTokens int64   `json:"outputTokens"`
	EstimatedUSD float64 `json:"estimatedUsd"`
}

// BudgetStatus shows current budget usage for a tenant.
type BudgetStatus struct {
	TenantID      int64   `json:"tenantId"`
	MonthlyBudget int64   `json:"monthlyBudget"` // token limit (0 = unlimited)
	UsedTokens    int64   `json:"usedTokens"`
	RemainingPct  float64 `json:"remainingPct"` // 0.0 to 1.0
	IsExceeded    bool    `json:"isExceeded"`
}

// Pricing per 1M tokens (approximate, updated 2026-04)
var ModelPricing = map[string]struct{ Input, Output float64 }{
	"qwen3-max":            {0.50, 1.50},
	"qwen3-coder-plus":     {0.30, 0.90},
	"claude-sonnet-4":      {3.00, 15.00},
	"gpt-4o":               {2.50, 10.00},
	"deepseek-chat":        {0.14, 0.28},
}

// EstimateCost calculates approximate USD cost from token counts.
func EstimateCost(model string, inputTokens, outputTokens int64) float64 {
	pricing, ok := ModelPricing[model]
	if !ok {
		// Default pricing for unknown models
		pricing = struct{ Input, Output float64 }{1.0, 3.0}
	}
	inputCost := float64(inputTokens) / 1_000_000 * pricing.Input
	outputCost := float64(outputTokens) / 1_000_000 * pricing.Output
	return inputCost + outputCost
}

// CostQuery parameters for querying cost data.
type CostQuery struct {
	TenantID  int64      `json:"tenantId"`
	ProjectID *int64     `json:"projectId,omitempty"`
	StartDate time.Time  `json:"startDate"`
	EndDate   time.Time  `json:"endDate"`
	GroupBy   string     `json:"groupBy"` // "day", "month", "model"
}
