package cost

import (
	"math"
	"testing"
)

func TestEstimateCost(t *testing.T) {
	tests := []struct {
		model  string
		input  int64
		output int64
		minUSD float64
		maxUSD float64
	}{
		{"qwen3-max", 1_000_000, 500_000, 1.0, 1.5},
		{"claude-sonnet-4", 100_000, 50_000, 0.9, 1.2},
		{"deepseek-chat", 1_000_000, 1_000_000, 0.3, 0.5},
		{"unknown-model", 1_000_000, 0, 0.5, 1.5},
		{"qwen3-max", 0, 0, 0, 0},
	}

	for _, tt := range tests {
		t.Run(tt.model, func(t *testing.T) {
			cost := EstimateCost(tt.model, tt.input, tt.output)
			if cost < tt.minUSD || cost > tt.maxUSD {
				t.Errorf("EstimateCost(%q, %d, %d) = %.4f, want [%.2f, %.2f]",
					tt.model, tt.input, tt.output, cost, tt.minUSD, tt.maxUSD)
			}
		})
	}
}

func TestEstimateCostZero(t *testing.T) {
	cost := EstimateCost("qwen3-max", 0, 0)
	if cost != 0 {
		t.Errorf("zero tokens should cost $0, got %.6f", cost)
	}
}

func TestEstimateCostPrecision(t *testing.T) {
	// 1M input tokens of qwen3-max at $0.50/M = $0.50
	cost := EstimateCost("qwen3-max", 1_000_000, 0)
	if math.Abs(cost-0.50) > 0.01 {
		t.Errorf("1M input qwen3-max should cost ~$0.50, got $%.4f", cost)
	}

	// 1M output tokens of qwen3-max at $1.50/M = $1.50
	cost = EstimateCost("qwen3-max", 0, 1_000_000)
	if math.Abs(cost-1.50) > 0.01 {
		t.Errorf("1M output qwen3-max should cost ~$1.50, got $%.4f", cost)
	}
}

func TestModelPricingEntries(t *testing.T) {
	// Verify all expected models have pricing
	expected := []string{"qwen3-max", "qwen3-coder-plus", "claude-sonnet-4", "gpt-4o", "deepseek-chat"}
	for _, model := range expected {
		if _, ok := ModelPricing[model]; !ok {
			t.Errorf("missing pricing for model %q", model)
		}
	}
}
