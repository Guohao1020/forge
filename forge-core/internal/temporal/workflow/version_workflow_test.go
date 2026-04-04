package workflow

import (
	"testing"
)

func TestDetectConflicts_NoOverlap(t *testing.T) {
	active := map[int64]*taskState{
		1: {TaskID: 1, Status: "RUNNING", TouchedFiles: []string{"user/service.go", "user/handler.go"}},
		2: {TaskID: 2, Status: "RUNNING", TouchedFiles: []string{"order/service.go"}},
	}
	newFiles := []string{"payment/service.go", "payment/handler.go"}

	blockers := detectConflicts(newFiles, active)
	if len(blockers) != 0 {
		t.Errorf("expected no blockers, got %v", blockers)
	}
}

func TestDetectConflicts_ExactFileOverlap(t *testing.T) {
	active := map[int64]*taskState{
		1: {TaskID: 1, Status: "RUNNING", TouchedFiles: []string{"user/service.go", "user/handler.go"}},
	}
	newFiles := []string{"user/handler.go", "user/points.go"}

	blockers := detectConflicts(newFiles, active)
	if len(blockers) != 1 || blockers[0] != 1 {
		t.Errorf("expected blocker [1], got %v", blockers)
	}
}

func TestDetectConflicts_CaseInsensitive(t *testing.T) {
	active := map[int64]*taskState{
		1: {TaskID: 1, Status: "RUNNING", TouchedFiles: []string{"User/Service.go"}},
	}
	newFiles := []string{"user/service.go"}

	blockers := detectConflicts(newFiles, active)
	if len(blockers) != 1 {
		t.Errorf("expected case-insensitive match, got %v", blockers)
	}
}

func TestDetectConflicts_WaitingTaskIgnored(t *testing.T) {
	// COMPLETED tasks should not block
	active := map[int64]*taskState{
		1: {TaskID: 1, Status: "COMPLETED", TouchedFiles: []string{"user/service.go"}},
	}
	newFiles := []string{"user/service.go"}

	blockers := detectConflicts(newFiles, active)
	if len(blockers) != 0 {
		t.Errorf("completed tasks should not block, got %v", blockers)
	}
}

func TestDetectConflicts_EmptyFiles(t *testing.T) {
	active := map[int64]*taskState{
		1: {TaskID: 1, Status: "RUNNING", TouchedFiles: []string{"user/service.go"}},
	}

	// New task has no files yet — should not block
	blockers := detectConflicts(nil, active)
	if len(blockers) != 0 {
		t.Errorf("nil files should not block, got %v", blockers)
	}

	blockers = detectConflicts([]string{}, active)
	if len(blockers) != 0 {
		t.Errorf("empty files should not block, got %v", blockers)
	}
}

func TestHasFileOverlap(t *testing.T) {
	tests := []struct {
		a, b []string
		want bool
	}{
		{[]string{"a.go", "b.go"}, []string{"c.go"}, false},
		{[]string{"a.go", "b.go"}, []string{"b.go"}, true},
		{[]string{"A.go"}, []string{"a.go"}, true}, // case insensitive
		{nil, []string{"a.go"}, false},
		{[]string{}, []string{}, false},
	}

	for _, tt := range tests {
		got := hasFileOverlap(tt.a, tt.b)
		if got != tt.want {
			t.Errorf("hasFileOverlap(%v, %v) = %v, want %v", tt.a, tt.b, got, tt.want)
		}
	}
}

func TestHasPackageOverlap(t *testing.T) {
	tests := []struct {
		a, b []string
		want bool
	}{
		// Same directory
		{[]string{"user/service.go"}, []string{"user/handler.go"}, true},
		// Different directories
		{[]string{"user/service.go"}, []string{"order/service.go"}, false},
		// Subdirectory match
		{[]string{"internal/user/service.go"}, []string{"internal/user/handler.go"}, true},
		// Root files
		{[]string{"main.go"}, []string{"config.go"}, true}, // both in "."
	}

	for _, tt := range tests {
		got := hasPackageOverlap(tt.a, tt.b)
		if got != tt.want {
			t.Errorf("hasPackageOverlap(%v, %v) = %v, want %v", tt.a, tt.b, got, tt.want)
		}
	}
}
