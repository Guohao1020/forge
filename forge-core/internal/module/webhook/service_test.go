package webhook

import (
	"testing"
)

func TestMatchesEvent(t *testing.T) {
	tests := []struct {
		events string
		event  string
		want   bool
	}{
		{"*", "task.completed", true},
		{"task.completed,task.failed", "task.completed", true},
		{"task.completed,task.failed", "task.failed", true},
		{"task.completed,task.failed", "pr.created", false},
		{"task.completed", "task.completed", true},
		{"task.completed", "task.failed", false},
		{"", "anything", false},
	}

	for _, tt := range tests {
		got := matchesEvent(tt.events, tt.event)
		if got != tt.want {
			t.Errorf("matchesEvent(%q, %q) = %v, want %v", tt.events, tt.event, got, tt.want)
		}
	}
}

func TestSplitEvents(t *testing.T) {
	tests := []struct {
		input string
		want  int
	}{
		{"task.completed,task.failed,pr.created", 3},
		{"task.completed", 1},
		{"*", 1},
		{"", 0},
		{"a,b", 2},
	}

	for _, tt := range tests {
		got := splitEvents(tt.input)
		if len(got) != tt.want {
			t.Errorf("splitEvents(%q) = %d items, want %d", tt.input, len(got), tt.want)
		}
	}
}

func TestWebhookPayloadStructure(t *testing.T) {
	p := WebhookPayload{
		Event:     "task.completed",
		Timestamp: "2026-04-05T12:00:00Z",
		ProjectID: 42,
		Data:      map[string]string{"taskId": "1", "title": "Add auth"},
	}

	if p.Event != "task.completed" {
		t.Errorf("expected event task.completed, got %s", p.Event)
	}
	if p.ProjectID != 42 {
		t.Errorf("expected projectId 42, got %d", p.ProjectID)
	}
}
