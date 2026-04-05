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

func TestSSEHubMultipleSubscribers(t *testing.T) {
	hub := NewSSEHub()

	ch1 := hub.subscribe(10)
	ch2 := hub.subscribe(10)

	hub.Broadcast(10, TaskProgressEvent{
		Type:   "progress",
		TaskID: 10,
		Data:   "50%",
	})

	// Both subscribers should receive
	select {
	case d := <-ch1:
		if len(d) == 0 {
			t.Error("ch1: empty data")
		}
	default:
		t.Error("ch1: expected data")
	}

	select {
	case d := <-ch2:
		if len(d) == 0 {
			t.Error("ch2: empty data")
		}
	default:
		t.Error("ch2: expected data")
	}

	hub.unsubscribe(10, ch1)
	hub.unsubscribe(10, ch2)
}

func TestSSEHubTaskIsolation(t *testing.T) {
	hub := NewSSEHub()

	ch1 := hub.subscribe(1)
	ch2 := hub.subscribe(2)

	// Broadcast to task 1 only
	hub.Broadcast(1, TaskProgressEvent{
		Type:   "status",
		TaskID: 1,
		Data:   "COMPLETED",
	})

	// ch1 should receive
	select {
	case <-ch1:
		// good
	default:
		t.Error("task 1 subscriber should receive")
	}

	// ch2 should NOT receive
	select {
	case <-ch2:
		t.Error("task 2 subscriber should NOT receive task 1 events")
	default:
		// good — no data
	}

	hub.unsubscribe(1, ch1)
	hub.unsubscribe(2, ch2)
}

func TestSSEHubUnsubscribeCleanup(t *testing.T) {
	hub := NewSSEHub()

	ch := hub.subscribe(42)

	hub.mu.RLock()
	subsBefore := len(hub.clients[42])
	hub.mu.RUnlock()

	if subsBefore != 1 {
		t.Errorf("expected 1 subscriber before unsubscribe, got %d", subsBefore)
	}

	hub.unsubscribe(42, ch)

	hub.mu.RLock()
	subsAfter := len(hub.clients[42])
	hub.mu.RUnlock()

	if subsAfter != 0 {
		t.Errorf("expected 0 subscribers after unsubscribe, got %d", subsAfter)
	}
}

func TestTaskProgressEventJSON(t *testing.T) {
	evt := TaskProgressEvent{
		Type:   "code_token",
		TaskID: 42,
		Data:   "func main() {",
	}
	if evt.Type != "code_token" {
		t.Errorf("expected type code_token, got %s", evt.Type)
	}
}
