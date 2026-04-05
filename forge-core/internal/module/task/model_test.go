package task

import (
	"testing"
)

func TestStatusConstants(t *testing.T) {
	statuses := []string{
		StatusSubmitted, StatusAnalyzing, StatusPlanning, StatusTestWriting,
		StatusGenerating, StatusReviewing, StatusTesting, StatusDeploying,
		StatusCompleted, StatusFailed, StatusCancelled,
	}
	if len(statuses) != 11 {
		t.Errorf("expected 11 status constants, got %d", len(statuses))
	}

	// Verify all are non-empty and unique
	seen := make(map[string]bool)
	for _, s := range statuses {
		if s == "" {
			t.Error("status constant should not be empty")
		}
		if seen[s] {
			t.Errorf("duplicate status: %s", s)
		}
		seen[s] = true
	}
}

func TestStepConstants(t *testing.T) {
	steps := []string{
		StepPending, StepRunning, StepCompleted, StepFailed, StepSkipped,
	}
	if len(steps) != 5 {
		t.Errorf("expected 5 step status constants, got %d", len(steps))
	}
}

func TestStepTypeConstants(t *testing.T) {
	types := []string{
		StepTypeAnalyze, StepTypePlan, StepTypeTestWriting,
		StepTypeGenerate, StepTypeReview, StepTypeTest, StepTypeDeploy,
	}
	if len(types) != 7 {
		t.Errorf("expected 7 step type constants, got %d", len(types))
	}
}

func TestAllSteps(t *testing.T) {
	if len(AllSteps) != 7 {
		t.Errorf("expected 7 default steps, got %d", len(AllSteps))
	}

	// First step should be analyze
	if AllSteps[0].StepType != StepTypeAnalyze {
		t.Errorf("first step should be ANALYZE, got %s", AllSteps[0].StepType)
	}

	// Last step should be deploy
	if AllSteps[len(AllSteps)-1].StepType != StepTypeDeploy {
		t.Errorf("last step should be DEPLOY, got %s", AllSteps[len(AllSteps)-1].StepType)
	}

	// All steps should have names
	for _, s := range AllSteps {
		if s.Name == "" || s.StepType == "" {
			t.Error("step should have both name and type")
		}
	}
}

func TestSSEHubBroadcast(t *testing.T) {
	hub := NewSSEHub()

	// Subscribe
	ch := hub.subscribe(42)

	// Broadcast
	hub.Broadcast(42, TaskProgressEvent{
		Type:   "status",
		TaskID: 42,
		Data:   "RUNNING",
	})

	// Should receive the event
	select {
	case data := <-ch:
		if len(data) == 0 {
			t.Error("received empty data")
		}
	default:
		t.Error("expected to receive broadcast")
	}

	// Unsubscribe
	hub.unsubscribe(42, ch)
}

func TestSSEHubNoSubscribers(t *testing.T) {
	hub := NewSSEHub()

	// Broadcasting with no subscribers should not panic
	hub.Broadcast(99, TaskProgressEvent{
		Type:   "status",
		TaskID: 99,
		Data:   "test",
	})
}
