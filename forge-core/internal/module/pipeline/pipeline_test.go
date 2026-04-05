package pipeline

import (
	"encoding/json"
	"testing"
)

func TestDeployRecordJSON(t *testing.T) {
	dr := DeployRecord{
		ProjectID: 1,
		Version:   "v1.0.0",
		Status:    "DEPLOYED",
	}
	data, _ := json.Marshal(dr)
	var parsed DeployRecord
	json.Unmarshal(data, &parsed)
	if parsed.Version != "v1.0.0" {
		t.Errorf("expected v1.0.0, got %s", parsed.Version)
	}
}

func TestTriggerDeployRequest(t *testing.T) {
	req := TriggerDeployRequest{Version: "v2.0.0"}
	if req.Version == "" {
		t.Error("version should not be empty")
	}
}

func TestEnvironmentListResponse_Empty(t *testing.T) {
	resp := EnvironmentListResponse{Environments: []Environment{}}
	if len(resp.Environments) != 0 {
		t.Error("expected empty")
	}
}
