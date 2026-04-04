package entropy

import "time"

// EntropyScan represents a single code quality scan result.
type EntropyScan struct {
	ID         int64     `json:"id"`
	ProjectID  int64     `json:"projectId"`
	Score      int       `json:"score"`
	IssueCount int       `json:"issueCount"`
	Issues     string    `json:"issues"` // JSON string
	ScannedAt  time.Time `json:"scannedAt"`
}

// EntropyConfig represents scan configuration for a project.
type EntropyConfig struct {
	ID        int64  `json:"id"`
	ProjectID int64  `json:"projectId"`
	Enabled   bool   `json:"enabled"`
	Schedule  string `json:"schedule"` // daily, weekly, monthly
	AutoFix   bool   `json:"autoFix"`
	Rules     string `json:"rules"` // JSON string
}

// QualityTrend represents a time-series data point for quality trends.
type QualityTrend struct {
	Date       string `json:"date"`
	Score      int    `json:"score"`
	IssueCount int    `json:"issueCount"`
}

// UpdateConfigRequest is the API request for updating entropy config.
type UpdateConfigRequest struct {
	Enabled  *bool    `json:"enabled"`
	Schedule string   `json:"schedule"`
	AutoFix  *bool    `json:"autoFix"`
	Rules    []string `json:"rules"`
}
