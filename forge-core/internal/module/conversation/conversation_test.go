package conversation

import (
	"encoding/json"
	"strings"
	"sync"
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

// ============================================================
// buildPlanRequirement tests
// ============================================================

func TestBuildPlanRequirement_FullMetadata(t *testing.T) {
	meta := map[string]interface{}{
		"summary": "Implement a web-based calculator supporting basic arithmetic operations.",
		"functional_requirements": []interface{}{
			"Addition and subtraction",
			"Multiplication and division",
			"Clear and backspace",
		},
		"non_functional": map[string]interface{}{
			"performance":   "Response time < 100ms",
			"security":      "No data storage needed",
			"compatibility": "Chrome, Firefox, Safari",
		},
		"acceptance_criteria": []interface{}{
			"1+1=2 displays correctly",
			"Continuous operations work without error",
		},
		"out_of_scope": []interface{}{
			"Scientific calculations",
			"User authentication",
		},
		"affected_modules": []interface{}{
			"frontend",
			"calculator-engine",
		},
		"estimated_complexity": "LOW",
	}

	result := buildPlanRequirement(meta, "original requirement")

	// Verify all sections are present
	if !strings.Contains(result, "## 需求概述") {
		t.Error("missing summary section header")
	}
	if !strings.Contains(result, "Implement a web-based calculator") {
		t.Error("missing summary content")
	}
	if !strings.Contains(result, "## 功能需求") {
		t.Error("missing functional requirements section")
	}
	if !strings.Contains(result, "1. Addition and subtraction") {
		t.Error("missing first functional requirement")
	}
	if !strings.Contains(result, "2. Multiplication and division") {
		t.Error("missing second functional requirement")
	}
	if !strings.Contains(result, "3. Clear and backspace") {
		t.Error("missing third functional requirement")
	}
	if !strings.Contains(result, "## 非功能需求") {
		t.Error("missing non-functional requirements section")
	}
	if !strings.Contains(result, "Response time < 100ms") {
		t.Error("missing performance constraint")
	}
	if !strings.Contains(result, "No data storage needed") {
		t.Error("missing security constraint")
	}
	if !strings.Contains(result, "## 验收标准") {
		t.Error("missing acceptance criteria section")
	}
	if !strings.Contains(result, "1. 1+1=2 displays correctly") {
		t.Error("missing first acceptance criterion")
	}
	if !strings.Contains(result, "## 不在范围内") {
		t.Error("missing out-of-scope section")
	}
	if !strings.Contains(result, "- Scientific calculations") {
		t.Error("missing first out-of-scope item")
	}
	if !strings.Contains(result, "## 影响模块") {
		t.Error("missing affected modules section")
	}
	if !strings.Contains(result, "frontend, calculator-engine") {
		t.Error("missing module names")
	}
	if !strings.Contains(result, "预估复杂度: LOW") {
		t.Error("missing complexity")
	}
	// Should NOT contain fallback text since summary is present
	if strings.Contains(result, "## 原始需求") {
		t.Error("should not contain fallback section when summary exists")
	}
}

func TestBuildPlanRequirement_MissingOptionalFields(t *testing.T) {
	meta := map[string]interface{}{
		"summary": "Simple feature with minimal metadata.",
	}

	result := buildPlanRequirement(meta, "fallback text")

	// Should have summary section
	if !strings.Contains(result, "## 需求概述") {
		t.Error("missing summary section")
	}
	if !strings.Contains(result, "Simple feature") {
		t.Error("missing summary content")
	}

	// Optional sections should NOT be present
	if strings.Contains(result, "## 功能需求") {
		t.Error("should not have functional requirements section when empty")
	}
	if strings.Contains(result, "## 非功能需求") {
		t.Error("should not have non-functional section when missing")
	}
	if strings.Contains(result, "## 验收标准") {
		t.Error("should not have acceptance criteria section when missing")
	}
	if strings.Contains(result, "## 不在范围内") {
		t.Error("should not have out-of-scope section when missing")
	}
	if strings.Contains(result, "## 影响模块") {
		t.Error("should not have affected modules section when missing")
	}
	if strings.Contains(result, "预估复杂度") {
		t.Error("should not have complexity when missing")
	}
}

func TestBuildPlanRequirement_EmptyMetadata(t *testing.T) {
	meta := map[string]interface{}{}
	fallback := "User wants a login page with social auth support"

	result := buildPlanRequirement(meta, fallback)

	// With empty metadata (no summary), should use fallback
	if !strings.Contains(result, "## 原始需求") {
		t.Error("should use fallback section when no summary")
	}
	if !strings.Contains(result, fallback) {
		t.Error("should contain fallback text")
	}
}

func TestBuildPlanRequirement_NilFields(t *testing.T) {
	// Simulate metadata with nil/empty slices — should not panic
	meta := map[string]interface{}{
		"summary":                  "Test summary",
		"functional_requirements":  []interface{}{},
		"non_functional":           map[string]interface{}{},
		"acceptance_criteria":      []interface{}{},
		"out_of_scope":             []interface{}{},
		"affected_modules":         []interface{}{},
		"estimated_complexity":     "",
	}

	result := buildPlanRequirement(meta, "fallback")

	// Should have summary but no empty sections
	if !strings.Contains(result, "## 需求概述") {
		t.Error("missing summary section")
	}
	if !strings.Contains(result, "Test summary") {
		t.Error("missing summary content")
	}

	// Empty slices should not produce section headers
	if strings.Contains(result, "## 功能需求") {
		t.Error("empty functional_requirements should not produce section")
	}
	if strings.Contains(result, "## 验收标准") {
		t.Error("empty acceptance_criteria should not produce section")
	}
	if strings.Contains(result, "## 不在范围内") {
		t.Error("empty out_of_scope should not produce section")
	}
	if strings.Contains(result, "## 影响模块") {
		t.Error("empty affected_modules should not produce section")
	}
	// Empty complexity string should not render
	if strings.Contains(result, "预估复杂度:") {
		t.Error("empty complexity should not render")
	}
}

func TestBuildPlanRequirement_NoSummaryUsesFallback(t *testing.T) {
	meta := map[string]interface{}{
		"functional_requirements": []interface{}{"Login", "Logout"},
		"estimated_complexity":    "MEDIUM",
	}
	fallback := "Build a user authentication system"

	result := buildPlanRequirement(meta, fallback)

	// No summary → fallback section
	if !strings.Contains(result, "## 原始需求") {
		t.Error("should use fallback section")
	}
	if !strings.Contains(result, fallback) {
		t.Error("should contain fallback text")
	}
	// But functional requirements should still be present
	if !strings.Contains(result, "## 功能需求") {
		t.Error("missing functional requirements section")
	}
	if !strings.Contains(result, "1. Login") {
		t.Error("missing first functional requirement")
	}
	if !strings.Contains(result, "2. Logout") {
		t.Error("missing second functional requirement")
	}
	if !strings.Contains(result, "预估复杂度: MEDIUM") {
		t.Error("missing complexity")
	}
}

func TestBuildPlanRequirement_NonFunctionalPartialValues(t *testing.T) {
	meta := map[string]interface{}{
		"summary": "Test",
		"non_functional": map[string]interface{}{
			"performance": "Under 200ms",
			"security":    "",  // empty value — should be skipped
			"other":       "Some constraint",
		},
	}

	result := buildPlanRequirement(meta, "")

	if !strings.Contains(result, "## 非功能需求") {
		t.Error("missing non-functional section")
	}
	if !strings.Contains(result, "Under 200ms") {
		t.Error("missing performance value")
	}
	if !strings.Contains(result, "Some constraint") {
		t.Error("missing other constraint")
	}
	// Empty security value should not appear as a line
	lines := strings.Split(result, "\n")
	for _, line := range lines {
		if strings.Contains(line, "security:") && strings.TrimSpace(strings.Split(line, ":")[1]) == "" {
			t.Error("empty security value should be skipped")
		}
	}
}

func TestBuildPlanRequirement_CompletelyEmptyReturns_Fallback(t *testing.T) {
	// When metadata produces absolutely nothing, return the fallback
	meta := map[string]interface{}{
		"summary": "",
	}
	fallback := "Raw user requirement text"

	result := buildPlanRequirement(meta, fallback)

	// Empty summary triggers fallback path
	if !strings.Contains(result, "## 原始需求") {
		t.Error("should use fallback when summary is empty")
	}
	if !strings.Contains(result, fallback) {
		t.Error("should contain fallback text")
	}
}

func TestBuildPlanRequirement_WrongTypeHandled(t *testing.T) {
	// Fields with wrong types (e.g., non-string in functional_requirements)
	// should be silently skipped without panic
	meta := map[string]interface{}{
		"summary":                 "Test",
		"functional_requirements": []interface{}{"Valid req", 123, nil},
		"acceptance_criteria":     []interface{}{"Criterion 1", true},
		"out_of_scope":            []interface{}{42, "Valid scope"},
		"affected_modules":        []interface{}{"module-a", 999},
	}

	// Should not panic
	result := buildPlanRequirement(meta, "fallback")

	if !strings.Contains(result, "Valid req") {
		t.Error("should include valid string items")
	}
	if !strings.Contains(result, "Criterion 1") {
		t.Error("should include valid acceptance criterion")
	}
	if !strings.Contains(result, "Valid scope") {
		t.Error("should include valid out-of-scope item")
	}
	if !strings.Contains(result, "module-a") {
		t.Error("should include valid module name")
	}
}

// ============================================================
// waitAndBroadcastPlan SSE event format tests
// ============================================================

// mockSSEBroadcaster captures BroadcastRaw calls for test assertions.
type mockSSEBroadcaster struct {
	mu     sync.Mutex
	events []sseCapture
}

type sseCapture struct {
	taskID int64
	data   []byte
}

func (m *mockSSEBroadcaster) BroadcastRaw(taskID int64, data []byte) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.events = append(m.events, sseCapture{taskID: taskID, data: data})
}

func (m *mockSSEBroadcaster) getEvents() []sseCapture {
	m.mu.Lock()
	defer m.mu.Unlock()
	cp := make([]sseCapture, len(m.events))
	copy(cp, m.events)
	return cp
}

func TestPlanCompleteEventFormat(t *testing.T) {
	// Verify the PLAN_COMPLETE SSE event has the expected JSON shape
	// This is the contract the frontend relies on.
	evt := map[string]interface{}{
		"type":    "PLAN_COMPLETE",
		"task_id": int64(42),
		"status":  "plan_review",
	}
	data, err := json.Marshal(evt)
	if err != nil {
		t.Fatal(err)
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatal(err)
	}

	if parsed["type"] != "PLAN_COMPLETE" {
		t.Errorf("expected type PLAN_COMPLETE, got %v", parsed["type"])
	}
	if parsed["status"] != "plan_review" {
		t.Errorf("expected status plan_review, got %v", parsed["status"])
	}
	if parsed["task_id"].(float64) != 42 {
		t.Errorf("expected task_id 42, got %v", parsed["task_id"])
	}
}

func TestPlanCompleteErrorEventFormat(t *testing.T) {
	// Verify error variant of PLAN_COMPLETE
	evt := map[string]interface{}{
		"type":    "PLAN_COMPLETE",
		"task_id": int64(42),
		"status":  "error",
		"data":    "方案生成失败，请重试。错误: timeout",
	}
	data, err := json.Marshal(evt)
	if err != nil {
		t.Fatal(err)
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatal(err)
	}

	if parsed["status"] != "error" {
		t.Errorf("expected status error, got %v", parsed["status"])
	}
	if parsed["data"] == nil || parsed["data"] == "" {
		t.Error("error event should include data field with error message")
	}
}

func TestConfirmPlanResponseContract(t *testing.T) {
	// Verify the async ConfirmPlan response contract:
	// It should return status "planning" with no plan data (plan arrives via SSE).
	resp := PlanConfirmResponse{
		Status: "planning",
	}

	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatal(err)
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatal(err)
	}

	if parsed["status"] != "planning" {
		t.Errorf("expected status planning, got %v", parsed["status"])
	}
	// planData should be null/empty since plan is delivered async
	if parsed["planData"] != nil {
		planData := parsed["planData"]
		if m, ok := planData.(map[string]interface{}); ok && len(m) > 0 {
			t.Error("async ConfirmPlan should not return planData (delivered via SSE)")
		}
	}
}

func TestMockSSEBroadcasterCaptures(t *testing.T) {
	// Verify the mock broadcaster works correctly for testing
	mock := &mockSSEBroadcaster{}
	data := []byte(`{"type":"PLAN_COMPLETE","task_id":1}`)
	mock.BroadcastRaw(1, data)
	mock.BroadcastRaw(1, data)

	events := mock.getEvents()
	if len(events) != 2 {
		t.Errorf("expected 2 events, got %d", len(events))
	}
	if events[0].taskID != 1 {
		t.Errorf("expected taskID 1, got %d", events[0].taskID)
	}
}

func TestFormatPlanForConversation(t *testing.T) {
	plan := map[string]interface{}{
		"title": "实现计算器",
		"tasks": []interface{}{
			map[string]interface{}{
				"order":          float64(1),
				"title":          "创建计算引擎",
				"type":           "BACKEND",
				"estimate_hours": float64(2),
				"description":    "实现基本运算逻辑",
				"files":          []interface{}{"calc.go"},
				"depends_on":     []interface{}{},
			},
		},
		"risk_level":          "LOW",
		"total_estimate_hours": float64(2),
		"parallel_tracks":     float64(1),
		"risk_factors":        []interface{}{"数学精度"},
	}

	result := formatPlanForConversation(plan)

	if !strings.Contains(result, "方案规划：实现计算器") {
		t.Error("missing plan title")
	}
	if !strings.Contains(result, "创建计算引擎") {
		t.Error("missing task title")
	}
	if !strings.Contains(result, "BACKEND") {
		t.Error("missing task type")
	}
	if !strings.Contains(result, "calc.go") {
		t.Error("missing file reference")
	}
	if !strings.Contains(result, "风险等级") {
		t.Error("missing risk level")
	}
	if !strings.Contains(result, "数学精度") {
		t.Error("missing risk factor")
	}
	if !strings.Contains(result, "请审查方案") {
		t.Error("missing review prompt")
	}
}
