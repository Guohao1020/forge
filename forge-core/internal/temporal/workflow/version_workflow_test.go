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

// --- Task State Management Tests ---

func TestDetectConflicts_MultipleBlockers(t *testing.T) {
	active := map[int64]*taskState{
		1: {TaskID: 1, Status: "RUNNING", TouchedFiles: []string{"shared/auth.go"}},
		2: {TaskID: 2, Status: "RUNNING", TouchedFiles: []string{"shared/auth.go", "shared/config.go"}},
		3: {TaskID: 3, Status: "COMPLETED", TouchedFiles: []string{"shared/auth.go"}}, // completed, should not block
	}
	newFiles := []string{"shared/auth.go"}

	blockers := detectConflicts(newFiles, active)
	if len(blockers) != 2 {
		t.Errorf("expected 2 blockers (tasks 1 and 2), got %v", blockers)
	}
}

func TestDetectConflicts_WaitingTasksAlsoBlock(t *testing.T) {
	active := map[int64]*taskState{
		1: {TaskID: 1, Status: "WAITING", TouchedFiles: []string{"user/service.go"}},
	}
	newFiles := []string{"user/service.go"}

	blockers := detectConflicts(newFiles, active)
	if len(blockers) != 1 {
		t.Errorf("WAITING tasks should also block, got %v", blockers)
	}
}

func TestTaskStateLifecycle(t *testing.T) {
	// Simulate: task added → blocked → unblocked → completed
	active := make(map[int64]*taskState)

	// Task 1 starts running
	active[1] = &taskState{TaskID: 1, Status: "RUNNING", TouchedFiles: []string{"a.go"}}

	// Task 2 arrives with conflict
	blockers := detectConflicts([]string{"a.go"}, active)
	if len(blockers) != 1 {
		t.Fatalf("task 2 should be blocked by task 1")
	}
	active[2] = &taskState{TaskID: 2, Status: "WAITING", TouchedFiles: []string{"a.go"}, BlockedBy: blockers}

	// Task 1 completes
	delete(active, 1)

	// Manually unblock (simulating unblockDependents logic)
	for _, state := range active {
		if state.Status == "WAITING" {
			newBlocked := make([]int64, 0)
			for _, b := range state.BlockedBy {
				if b != 1 { // remove completed task
					newBlocked = append(newBlocked, b)
				}
			}
			state.BlockedBy = newBlocked
			if len(state.BlockedBy) == 0 {
				state.Status = "RUNNING"
			}
		}
	}

	if active[2].Status != "RUNNING" {
		t.Errorf("task 2 should be RUNNING after task 1 completes, got %q", active[2].Status)
	}
}

func TestMultipleBlockersPartialUnblock(t *testing.T) {
	active := map[int64]*taskState{
		1: {TaskID: 1, Status: "RUNNING", TouchedFiles: []string{"a.go"}},
		2: {TaskID: 2, Status: "RUNNING", TouchedFiles: []string{"b.go"}},
		3: {TaskID: 3, Status: "WAITING", TouchedFiles: []string{"a.go", "b.go"}, BlockedBy: []int64{1, 2}},
	}

	// Task 1 completes — task 3 should still be waiting (blocked by 2)
	delete(active, 1)
	for _, state := range active {
		if state.Status == "WAITING" {
			newBlocked := make([]int64, 0)
			for _, b := range state.BlockedBy {
				if b != 1 {
					newBlocked = append(newBlocked, b)
				}
			}
			state.BlockedBy = newBlocked
			if len(state.BlockedBy) == 0 {
				state.Status = "RUNNING"
			}
		}
	}

	if active[3].Status != "WAITING" {
		t.Errorf("task 3 should still be WAITING (blocked by 2), got %q", active[3].Status)
	}
	if len(active[3].BlockedBy) != 1 || active[3].BlockedBy[0] != 2 {
		t.Errorf("task 3 should be blocked by [2], got %v", active[3].BlockedBy)
	}

	// Task 2 completes — task 3 should now be running
	delete(active, 2)
	for _, state := range active {
		if state.Status == "WAITING" {
			newBlocked := make([]int64, 0)
			for _, b := range state.BlockedBy {
				if b != 2 {
					newBlocked = append(newBlocked, b)
				}
			}
			state.BlockedBy = newBlocked
			if len(state.BlockedBy) == 0 {
				state.Status = "RUNNING"
			}
		}
	}

	if active[3].Status != "RUNNING" {
		t.Errorf("task 3 should be RUNNING after both blockers complete, got %q", active[3].Status)
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
