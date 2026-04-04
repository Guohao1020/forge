package entropy

import (
	"testing"
	"time"
)

func TestEntropyScanDefaults(t *testing.T) {
	scan := EntropyScan{
		ID:        1,
		ProjectID: 10,
		Score:     85,
		ScannedAt: time.Now(),
	}
	if scan.Score < 0 || scan.Score > 100 {
		t.Errorf("score out of range: %d", scan.Score)
	}
}

func TestEntropyConfigDefaults(t *testing.T) {
	cfg := EntropyConfig{
		ProjectID: 1,
		Enabled:   true,
		Schedule:  "weekly",
		AutoFix:   false,
		Rules:     "[]",
	}
	if cfg.Schedule != "weekly" {
		t.Errorf("expected weekly schedule, got %s", cfg.Schedule)
	}
}

func TestQualityTrend(t *testing.T) {
	trend := QualityTrend{
		Date:       "2026-04-05",
		Score:      92,
		IssueCount: 3,
	}
	if trend.Score < 0 || trend.Score > 100 {
		t.Errorf("score out of range: %d", trend.Score)
	}
}

func TestUpdateConfigRequest(t *testing.T) {
	enabled := true
	autoFix := false
	req := UpdateConfigRequest{
		Enabled:  &enabled,
		Schedule: "daily",
		AutoFix:  &autoFix,
		Rules:    []string{"naming", "dead_code"},
	}
	if *req.Enabled != true {
		t.Error("expected enabled=true")
	}
	if len(req.Rules) != 2 {
		t.Errorf("expected 2 rules, got %d", len(req.Rules))
	}
}
