package conversation

import (
	"testing"
)

func TestRoleConstants(t *testing.T) {
	roles := []string{RoleUser, RoleAssistant, RoleSystem}
	if len(roles) != 3 {
		t.Errorf("expected 3 role constants, got %d", len(roles))
	}

	seen := make(map[string]bool)
	for _, r := range roles {
		if r == "" {
			t.Error("role should not be empty")
		}
		if seen[r] {
			t.Errorf("duplicate role: %s", r)
		}
		seen[r] = true
	}
}

func TestConversationStruct(t *testing.T) {
	conv := Conversation{
		TaskID:  42,
		Role:    RoleUser,
		Content: "Add login feature",
	}
	if conv.Role != "user" {
		t.Errorf("expected role 'user', got %s", conv.Role)
	}
	if conv.Content == "" {
		t.Error("content should not be empty")
	}
}

func TestModelCallStruct(t *testing.T) {
	mc := ModelCall{
		TenantID:     1,
		TaskID:       42,
		Model:        "qwen3-coder-plus",
		Provider:     "dashscope",
		InputTokens:  500,
		OutputTokens: 1000,
		TotalTokens:  1500,
		LatencyMs:    3200,
	}
	if mc.TotalTokens != mc.InputTokens+mc.OutputTokens {
		t.Errorf("total tokens mismatch: %d != %d + %d", mc.TotalTokens, mc.InputTokens, mc.OutputTokens)
	}
}

func TestSendMessageRequest(t *testing.T) {
	req := SendMessageRequest{
		Content: "I want to add a login page",
	}
	if req.Content == "" {
		t.Error("content should not be empty")
	}
}
