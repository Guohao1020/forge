package card

import (
	"testing"
)

func TestWelcomeCard(t *testing.T) {
	r := NewRenderer()
	msg := r.WelcomeCard()

	if msg.MsgType != "actionCard" {
		t.Errorf("expected actionCard, got %s", msg.MsgType)
	}
	if msg.ActionCard == nil {
		t.Fatal("expected actionCard body")
	}
	if msg.ActionCard.Title == "" {
		t.Error("expected non-empty title")
	}
}

func TestRequirementClarificationCard(t *testing.T) {
	r := NewRenderer()
	msg := r.RequirementClarificationCard(42, "Which approach?", []string{"Option A", "Option B", "Option C"})

	if msg.MsgType != "actionCard" {
		t.Errorf("expected actionCard, got %s", msg.MsgType)
	}
	if len(msg.ActionCard.Btns) != 3 {
		t.Errorf("expected 3 buttons, got %d", len(msg.ActionCard.Btns))
	}
}

func TestPlanSummaryCard(t *testing.T) {
	r := NewRenderer()
	msg := r.PlanSummaryCard(99, "Add login feature", []string{"Create UI", "Add API", "Write tests"})

	if msg.MsgType != "actionCard" {
		t.Errorf("expected actionCard, got %s", msg.MsgType)
	}
	if len(msg.ActionCard.Btns) != 2 {
		t.Errorf("expected 2 buttons (approve/revise), got %d", len(msg.ActionCard.Btns))
	}
}

func TestTaskCompletedCard(t *testing.T) {
	r := NewRenderer()
	msg := r.TaskCompletedCard(1, "Add auth", "https://github.com/test/pr/1", "feature/auth", 5)

	if msg.MsgType != "actionCard" {
		t.Errorf("expected actionCard, got %s", msg.MsgType)
	}
	if len(msg.ActionCard.Btns) != 2 {
		t.Errorf("expected 2 buttons, got %d", len(msg.ActionCard.Btns))
	}
}

func TestTaskProgressCard(t *testing.T) {
	r := NewRenderer()
	msg := r.TaskProgressCard(1, "Build feature", "GENERATE", "Generating code...")

	if msg.MsgType != "markdown" {
		t.Errorf("expected markdown, got %s", msg.MsgType)
	}
	if msg.Markdown == nil || msg.Markdown.Title == "" {
		t.Error("expected non-empty markdown title")
	}
}

func TestErrorCard(t *testing.T) {
	r := NewRenderer()
	msg := r.ErrorCard("something went wrong")

	if msg.MsgType != "markdown" {
		t.Errorf("expected markdown, got %s", msg.MsgType)
	}
}

func TestProjectListCard_Empty(t *testing.T) {
	r := NewRenderer()
	msg := r.ProjectListCard(nil)

	if msg.MsgType != "markdown" {
		t.Errorf("expected markdown, got %s", msg.MsgType)
	}
}

func TestProjectListCard_WithProjects(t *testing.T) {
	r := NewRenderer()
	projects := []map[string]interface{}{
		{"name": "forge", "description": "AI platform"},
		{"name": "bot", "description": ""},
	}
	msg := r.ProjectListCard(projects)

	if msg.MsgType != "markdown" {
		t.Errorf("expected markdown, got %s", msg.MsgType)
	}
}

func TestTruncate(t *testing.T) {
	tests := []struct {
		input  string
		maxLen int
		want   string
	}{
		{"hello", 10, "hello"},
		{"hello world", 5, "hello..."},
		{"", 5, ""},
		{"短", 1, "短"},
		{"你好世界", 2, "你好..."},
	}

	for _, tt := range tests {
		got := truncate(tt.input, tt.maxLen)
		if got != tt.want {
			t.Errorf("truncate(%q, %d) = %q, want %q", tt.input, tt.maxLen, got, tt.want)
		}
	}
}
