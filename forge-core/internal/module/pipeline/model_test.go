package pipeline

import "testing"

func TestEnvironmentStruct(t *testing.T) {
	env := Environment{
		ProjectID: 1,
		Name:      "staging",
		EnvType:   "STAGING",
		Status:    "ACTIVE",
	}
	if env.Name != "staging" {
		t.Errorf("expected name staging, got %s", env.Name)
	}
}

func TestDeployRecord(t *testing.T) {
	dr := DeployRecord{
		ProjectID:     1,
		EnvironmentID: 2,
		Version:       "v1.0.0",
		Status:        "DEPLOYED",
		DeployedBy:    1,
	}
	if dr.Version != "v1.0.0" {
		t.Errorf("expected version v1.0.0, got %s", dr.Version)
	}
	if dr.Status != "DEPLOYED" {
		t.Errorf("expected status DEPLOYED, got %s", dr.Status)
	}
}

func TestDeployStatuses(t *testing.T) {
	validStatuses := []string{"PENDING", "DEPLOYING", "DEPLOYED", "FAILED", "ROLLED_BACK"}
	for _, s := range validStatuses {
		if s == "" {
			t.Error("status should not be empty")
		}
	}
	if len(validStatuses) != 5 {
		t.Errorf("expected 5 deploy statuses, got %d", len(validStatuses))
	}
}
