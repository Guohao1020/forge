package settings

import "testing"

func TestDefaultsHaveAllCategories(t *testing.T) {
	defaults := Defaults()
	categories := make(map[string]bool)
	for _, s := range defaults {
		categories[s.Category] = true
	}

	required := []string{"ai", "deploy", "notification", "general"}
	for _, cat := range required {
		if !categories[cat] {
			t.Errorf("missing category: %s", cat)
		}
	}
}

func TestDefaultsKeysUnique(t *testing.T) {
	defaults := Defaults()
	seen := make(map[string]bool)
	for key := range defaults {
		if seen[key] {
			t.Errorf("duplicate key: %s", key)
		}
		seen[key] = true
	}
}

func TestDefaultsNonEmptyValues(t *testing.T) {
	defaults := Defaults()
	// At least some defaults should have non-empty values
	nonEmpty := 0
	for _, s := range defaults {
		if s.Value != "" {
			nonEmpty++
		}
	}
	if nonEmpty < 5 {
		t.Errorf("expected at least 5 defaults with values, got %d", nonEmpty)
	}
}
