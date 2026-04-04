package version

import (
	"testing"
)

// TestCreateVersionRequest validates the request binding rules.
func TestCreateVersionRequestValidation(t *testing.T) {
	tests := []struct {
		name    string
		req     CreateVersionRequest
		wantErr bool
	}{
		{"valid version", CreateVersionRequest{Version: "v1.0.0", Description: "test"}, false},
		{"valid without desc", CreateVersionRequest{Version: "v1.0"}, false},
		{"empty version", CreateVersionRequest{Version: ""}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hasErr := tt.req.Version == ""
			if hasErr != tt.wantErr {
				t.Errorf("validation = %v, want %v", hasErr, tt.wantErr)
			}
		})
	}
}

// TestVersionStatusConstants verifies status string values.
func TestVersionStatusConstants(t *testing.T) {
	statuses := []string{StatusPlanning, StatusInProgress, StatusTesting, StatusReleased, StatusCancelled}
	expected := []string{"PLANNING", "IN_PROGRESS", "TESTING", "RELEASED", "CANCELLED"}

	for i, s := range statuses {
		if s != expected[i] {
			t.Errorf("status[%d] = %q, want %q", i, s, expected[i])
		}
	}
}

// TestVersionModelJSON verifies JSON field names.
func TestVersionModelJSON(t *testing.T) {
	v := ProjectVersion{
		ID:        1,
		TenantID:  1,
		ProjectID: 10,
		Version:   "v1.0.0",
		Status:    StatusPlanning,
	}

	if v.Version != "v1.0.0" {
		t.Errorf("Version = %q, want v1.0.0", v.Version)
	}
	if v.Status != "PLANNING" {
		t.Errorf("Status = %q, want PLANNING", v.Status)
	}
}

// TestReleaseRequestDefaults verifies default values.
func TestReleaseRequestDefaults(t *testing.T) {
	req := ReleaseRequest{}
	if req.DeployAfterTag {
		t.Error("DeployAfterTag should default to false")
	}
}
