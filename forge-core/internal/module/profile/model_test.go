package profile

import (
	"encoding/json"
	"testing"
)

func TestProfileDimensionKeys(t *testing.T) {
	keys := []string{
		KeyAPICatalog, KeyDBSchema, KeyModuleGraph,
		KeyArchitecture, KeyBusinessRules, KeyCodingHabits, KeyQualityTrends,
	}
	if len(keys) != 7 {
		t.Errorf("expected 7 dimension keys, got %d", len(keys))
	}

	seen := make(map[string]bool)
	for _, k := range keys {
		if k == "" {
			t.Error("dimension key should not be empty")
		}
		if seen[k] {
			t.Errorf("duplicate dimension key: %s", k)
		}
		seen[k] = true
	}
}

func TestProfileEntry(t *testing.T) {
	entry := ProfileEntry{
		ProjectID:    42,
		ProfileKey:   KeyAPICatalog,
		ProfileValue: json.RawMessage(`{"endpoints": []}`),
		Version:      1,
	}
	if entry.ProjectID != 42 {
		t.Errorf("expected projectID 42, got %d", entry.ProjectID)
	}
	if entry.ProfileKey != "api_catalog" {
		t.Errorf("expected key api_catalog, got %s", entry.ProfileKey)
	}
}

func TestScanRequest(t *testing.T) {
	req := ScanRequest{
		Keys: []string{KeyAPICatalog, KeyDBSchema},
	}
	if len(req.Keys) != 2 {
		t.Errorf("expected 2 keys, got %d", len(req.Keys))
	}
}

func TestScanRequest_Empty(t *testing.T) {
	req := ScanRequest{}
	if req.Keys != nil {
		t.Error("empty keys should be nil (full scan)")
	}
}
