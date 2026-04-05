package preview

import (
	"encoding/json"
	"testing"
)

func TestPreviewEnvironmentJSON(t *testing.T) {
	url := "http://preview.forge.dev"
	pe := PreviewEnvironment{
		ProjectID:  1,
		PreviewURL: &url,
		Status:     "RUNNING",
	}
	data, _ := json.Marshal(pe)
	var parsed PreviewEnvironment
	json.Unmarshal(data, &parsed)
	if *parsed.PreviewURL != url {
		t.Errorf("expected %s, got %s", url, *parsed.PreviewURL)
	}
}

func TestPreviewListResponse_Multiple(t *testing.T) {
	resp := PreviewListResponse{
		Previews: []PreviewEnvironment{
			{Status: "RUNNING"},
			{Status: "STOPPED"},
			{Status: "RUNNING"},
		},
	}
	running := 0
	for _, p := range resp.Previews {
		if p.Status == "RUNNING" {
			running++
		}
	}
	if running != 2 {
		t.Errorf("expected 2 running, got %d", running)
	}
}
