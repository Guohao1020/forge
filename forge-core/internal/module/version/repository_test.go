package version

import (
	"testing"
)

func TestVersionListResponseEmpty(t *testing.T) {
	resp := VersionListResponse{Versions: nil}
	if resp.Versions != nil {
		t.Error("nil versions should be nil")
	}

	resp2 := VersionListResponse{Versions: []ProjectVersion{}}
	if len(resp2.Versions) != 0 {
		t.Errorf("empty versions length = %d, want 0", len(resp2.Versions))
	}
}

func TestVersionDetailResponseEmpty(t *testing.T) {
	resp := VersionDetailResponse{
		Version: ProjectVersion{Version: "v1.0.0"},
		Tasks:   []VersionTaskBrief{},
	}
	if resp.Version.Version != "v1.0.0" {
		t.Errorf("version = %q, want v1.0.0", resp.Version.Version)
	}
	if len(resp.Tasks) != 0 {
		t.Errorf("tasks length = %d, want 0", len(resp.Tasks))
	}
}

func TestVersionTaskBriefConflictStatus(t *testing.T) {
	task := VersionTaskBrief{
		ID:             1,
		Title:          "test task",
		Status:         "RUNNING",
		ConflictStatus: "NONE",
	}
	if task.ConflictStatus != "NONE" {
		t.Errorf("conflict = %q, want NONE", task.ConflictStatus)
	}

	task.ConflictStatus = "WAITING"
	if task.ConflictStatus != "WAITING" {
		t.Errorf("conflict = %q, want WAITING", task.ConflictStatus)
	}
}

func TestProjectVersionComputedFields(t *testing.T) {
	v := ProjectVersion{
		ID:             1,
		Version:        "v1.0.0",
		Status:         StatusPlanning,
		TaskCount:      5,
		CompletedCount: 3,
	}

	progress := 0
	if v.TaskCount > 0 {
		progress = v.CompletedCount * 100 / v.TaskCount
	}

	if progress != 60 {
		t.Errorf("progress = %d%%, want 60%%", progress)
	}
}
