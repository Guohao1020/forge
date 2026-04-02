package testresult

import "time"

// Test result status constants
const (
	StatusPending = "PENDING"
	StatusRunning = "RUNNING"
	StatusPassed  = "PASSED"
	StatusFailed  = "FAILED"
)

// Test layer constants
const (
	LayerUnit        = "UNIT"
	LayerAPI         = "API"
	LayerIntegration = "INTEGRATION"
	LayerE2E         = "E2E"
)

type TestResult struct {
	ID          int64     `json:"id"`
	TaskID      int64     `json:"taskId"`
	Layer       string    `json:"layer"`
	Framework   string    `json:"framework"`
	TotalCases  int       `json:"totalCases"`
	Passed      int       `json:"passed"`
	Failed      int       `json:"failed"`
	Skipped     int       `json:"skipped"`
	CoveragePct *float64  `json:"coveragePct"`
	DurationMs  *int      `json:"durationMs"`
	Report      string    `json:"report"`
	Status      string    `json:"status"`
	CreatedAt   time.Time `json:"createdAt"`
}

type TestResultListResponse struct {
	Results []TestResult `json:"results"`
}

type CreateTestResultRequest struct {
	TaskID      int64    `json:"taskId"`
	Layer       string   `json:"layer"`
	Framework   string   `json:"framework"`
	TotalCases  int      `json:"totalCases"`
	Passed      int      `json:"passed"`
	Failed      int      `json:"failed"`
	Skipped     int      `json:"skipped"`
	CoveragePct *float64 `json:"coveragePct"`
	DurationMs  *int     `json:"durationMs"`
	Report      string   `json:"report"`
	Status      string   `json:"status"`
}
