package profile

import (
	"encoding/json"
	"time"
)

type ProfileEntry struct {
	ID           int64           `json:"id"`
	ProjectID    int64           `json:"projectId"`
	ProfileKey   string          `json:"profileKey"`
	ProfileValue json.RawMessage `json:"profileValue"`
	Version      int             `json:"version"`
	ScannedAt    time.Time       `json:"scannedAt"`
	CreatedAt    time.Time       `json:"createdAt"`
	UpdatedAt    time.Time       `json:"updatedAt"`
}

// Profile dimension keys
const (
	KeyAPICatalog    = "api_catalog"
	KeyDBSchema      = "db_schema"
	KeyModuleGraph   = "module_graph"
	KeyArchitecture  = "architecture"
	KeyBusinessRules = "business_rules"
	KeyCodingHabits  = "coding_habits"
	KeyQualityTrends = "quality_trends"
)

type ProfileListResponse struct {
	Profiles []ProfileEntry `json:"profiles"`
}

type ScanRequest struct {
	Keys     []string `json:"keys"`     // optional, empty = full scan
	Branches []string `json:"branches"` // optional, empty = default branch
}
