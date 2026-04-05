package testresult

import "testing"

func TestStatusConstants(t *testing.T) {
	statuses := []string{StatusPending, StatusRunning, StatusPassed, StatusFailed}
	if len(statuses) != 4 {
		t.Errorf("expected 4 test status constants, got %d", len(statuses))
	}
	for _, s := range statuses {
		if s == "" {
			t.Error("status should not be empty")
		}
	}
}

func TestLayerConstants(t *testing.T) {
	layers := []string{LayerUnit, LayerAPI, LayerIntegration, LayerE2E}
	if len(layers) != 4 {
		t.Errorf("expected 4 test layer constants, got %d", len(layers))
	}
	for _, l := range layers {
		if l == "" {
			t.Error("layer should not be empty")
		}
	}
}

func TestTestResult(t *testing.T) {
	cov := 85.5
	dur := 3200
	tr := TestResult{
		TaskID:      42,
		Layer:       LayerUnit,
		Framework:   "go test",
		TotalCases:  100,
		Passed:      95,
		Failed:      3,
		Skipped:     2,
		CoveragePct: &cov,
		DurationMs:  &dur,
		Status:      StatusFailed,
	}

	if tr.Passed+tr.Failed+tr.Skipped != tr.TotalCases {
		t.Errorf("case counts don't add up: %d + %d + %d != %d",
			tr.Passed, tr.Failed, tr.Skipped, tr.TotalCases)
	}
	if *tr.CoveragePct != 85.5 {
		t.Errorf("expected coverage 85.5, got %.1f", *tr.CoveragePct)
	}
}

func TestCreateTestResultRequest(t *testing.T) {
	req := CreateTestResultRequest{
		TaskID:     1,
		Layer:      LayerUnit,
		Framework:  "pytest",
		TotalCases: 50,
		Passed:     50,
		Status:     StatusPassed,
	}
	if req.Failed != 0 {
		t.Errorf("expected 0 failures, got %d", req.Failed)
	}
}
