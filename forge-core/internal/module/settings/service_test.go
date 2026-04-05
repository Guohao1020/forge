package settings

import (
	"testing"
)

func TestDefaults(t *testing.T) {
	defaults := Defaults()

	if len(defaults) < 10 {
		t.Errorf("expected at least 10 default settings, got %d", len(defaults))
	}

	// Check key settings exist
	required := []string{
		"ai.default_model",
		"ai.fallback_chain",
		"ai.max_tokens",
		"deploy.auto_merge",
		"general.language",
	}
	for _, key := range required {
		if _, ok := defaults[key]; !ok {
			t.Errorf("missing default setting: %s", key)
		}
	}
}

func TestDefaultValues(t *testing.T) {
	defaults := Defaults()

	tests := []struct {
		key   string
		value string
	}{
		{"ai.default_model", "qwen3-coder-plus"},
		{"general.language", "zh-CN"},
		{"general.timezone", "Asia/Shanghai"},
		{"deploy.auto_merge", "false"},
		{"deploy.require_review", "true"},
	}

	for _, tt := range tests {
		d, ok := defaults[tt.key]
		if !ok {
			t.Errorf("missing key %s", tt.key)
			continue
		}
		if d.Value != tt.value {
			t.Errorf("key %s: expected %q, got %q", tt.key, tt.value, d.Value)
		}
	}
}

func TestDefaultCategories(t *testing.T) {
	defaults := Defaults()

	categories := map[string]int{}
	for _, d := range defaults {
		categories[d.Category]++
	}

	if categories["ai"] < 3 {
		t.Errorf("expected at least 3 AI settings, got %d", categories["ai"])
	}
	if categories["deploy"] < 2 {
		t.Errorf("expected at least 2 deploy settings, got %d", categories["deploy"])
	}
}
