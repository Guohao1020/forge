package conversation

import (
	"encoding/json"
	"testing"
)

func TestConversationJSON(t *testing.T) {
	conv := Conversation{
		TaskID:  42,
		Role:    RoleUser,
		Content: "Add login feature",
	}
	data, err := json.Marshal(conv)
	if err != nil {
		t.Fatal(err)
	}
	var parsed Conversation
	json.Unmarshal(data, &parsed)
	if parsed.Role != "user" {
		t.Errorf("expected user, got %s", parsed.Role)
	}
}

func TestModelCallTokens(t *testing.T) {
	mc := ModelCall{
		InputTokens:  1000,
		OutputTokens: 500,
		TotalTokens:  1500,
	}
	if mc.InputTokens+mc.OutputTokens != mc.TotalTokens {
		t.Error("tokens don't add up")
	}
}

func TestPlanConfirmResponse(t *testing.T) {
	r := PlanConfirmResponse{
		Status: "plan_review",
		PlanData: map[string]interface{}{
			"steps": []string{"step1", "step2"},
		},
	}
	if r.Status != "plan_review" {
		t.Errorf("expected plan_review, got %s", r.Status)
	}
}
