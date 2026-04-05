package preview

import "testing"

func TestPreviewEnvironment(t *testing.T) {
	taskID := int64(42)
	branch := "feature/auth"
	url := "http://preview-42.forge.dev"

	pe := PreviewEnvironment{
		ProjectID:  1,
		TaskID:     &taskID,
		BranchName: &branch,
		PreviewURL: &url,
		Status:     "RUNNING",
	}

	if *pe.TaskID != 42 {
		t.Errorf("expected taskID 42, got %d", *pe.TaskID)
	}
	if pe.Status != "RUNNING" {
		t.Errorf("expected status RUNNING, got %s", pe.Status)
	}
	if *pe.PreviewURL != "http://preview-42.forge.dev" {
		t.Errorf("unexpected preview URL: %s", *pe.PreviewURL)
	}
}

func TestPreviewListResponse(t *testing.T) {
	resp := PreviewListResponse{
		Previews: []PreviewEnvironment{
			{ProjectID: 1, Status: "RUNNING"},
			{ProjectID: 1, Status: "STOPPED"},
		},
	}
	if len(resp.Previews) != 2 {
		t.Errorf("expected 2 previews, got %d", len(resp.Previews))
	}
}
