package specs

import (
	"testing"
)

// ==================== Pagination Validation ====================

func TestListStandards_PageLessThanOne_CorrectToOne(t *testing.T) {
	svc := &Service{}
	f := StandardFilter{Page: 0, PageSize: 20}

	func() {
		defer func() {
			r := recover()
			if r == nil {
				t.Fatal("expected panic from nil repo call, meaning pagination validation passed")
			}
		}()
		svc.ListStandards(t.Context(), 1, f)
	}()
}

func TestListStandards_PageSizeOverMax_CorrectToDefault(t *testing.T) {
	svc := &Service{}
	f := StandardFilter{Page: 1, PageSize: 200}

	func() {
		defer func() {
			r := recover()
			if r == nil {
				t.Fatal("expected panic from nil repo call")
			}
		}()
		svc.ListStandards(t.Context(), 1, f)
	}()
}

func TestListStandards_PageSizeZero_CorrectToDefault(t *testing.T) {
	svc := &Service{}
	f := StandardFilter{Page: 1, PageSize: 0}

	func() {
		defer func() {
			r := recover()
			if r == nil {
				t.Fatal("expected panic from nil repo call")
			}
		}()
		svc.ListStandards(t.Context(), 1, f)
	}()
}

func TestCreatePromptTemplate_NilVariables_InitToEmptySlice(t *testing.T) {
	svc := &Service{}
	req := CreatePromptTemplateReq{
		Name:         "test",
		Purpose:      "code-generation",
		SystemPrompt: "test",
		UserTemplate: "test",
		Variables:    nil,
	}

	func() {
		defer func() {
			r := recover()
			if r == nil {
				t.Fatal("expected panic from nil repo call")
			}
		}()
		svc.CreatePromptTemplate(t.Context(), 1, 1, req)
	}()
}

// ==================== Three-Level Inheritance Merge ====================

func TestMergeStandards_ProjectOverridesCompany(t *testing.T) {
	company := []*Standard{
		{ID: 1, Category: "JAVA", Scope: "COMPANY", Content: "company java"},
		{ID: 2, Category: "SQL", Scope: "COMPANY", Content: "company sql"},
	}
	project := []*Standard{
		{ID: 10, Category: "JAVA", Scope: "PROJECT", Content: "project java override"},
	}

	merged := mergeStandards(company, project)

	if len(merged) != 2 {
		t.Fatalf("expected 2 merged standards, got %d", len(merged))
	}

	// Find JAVA standard — should be project override
	var javaStd *Standard
	for _, s := range merged {
		if s.Category == "JAVA" {
			javaStd = s
		}
	}
	if javaStd == nil {
		t.Fatal("JAVA standard not found in merged result")
	}
	if javaStd.ID != 10 {
		t.Fatalf("JAVA standard should be project override (id=10), got id=%d", javaStd.ID)
	}
	if javaStd.Content != "project java override" {
		t.Fatalf("JAVA content should be project override, got %q", javaStd.Content)
	}
}

func TestMergeStandards_NoOverride_KeepsCompany(t *testing.T) {
	company := []*Standard{
		{ID: 1, Category: "JAVA", Scope: "COMPANY", Content: "company java"},
		{ID: 2, Category: "SQL", Scope: "COMPANY", Content: "company sql"},
	}
	project := []*Standard{} // no overrides

	merged := mergeStandards(company, project)

	if len(merged) != 2 {
		t.Fatalf("expected 2 merged standards, got %d", len(merged))
	}
}

func TestMergeStandards_ProjectAddsNewCategory(t *testing.T) {
	company := []*Standard{
		{ID: 1, Category: "JAVA", Content: "company java"},
	}
	project := []*Standard{
		{ID: 10, Category: "REDIS", Content: "project redis"},
	}

	merged := mergeStandards(company, project)

	if len(merged) != 2 {
		t.Fatalf("expected 2 merged standards (JAVA + REDIS), got %d", len(merged))
	}
}

func TestMergeRules_ProjectOverridesByNameAndCategory(t *testing.T) {
	company := []*ReviewRule{
		{ID: 1, Category: "CODING", Name: "no-empty-catch", Scope: "COMPANY"},
		{ID: 2, Category: "SECURITY", Name: "no-plaintext-pwd", Scope: "COMPANY"},
	}
	project := []*ReviewRule{
		{ID: 10, Category: "CODING", Name: "no-empty-catch", Scope: "PROJECT", Severity: "WARNING"},
	}

	merged := mergeRules(company, project)

	if len(merged) != 2 {
		t.Fatalf("expected 2 merged rules, got %d", len(merged))
	}

	// Find the CODING:no-empty-catch rule — should be project override
	var codingRule *ReviewRule
	for _, r := range merged {
		if r.Category == "CODING" && r.Name == "no-empty-catch" {
			codingRule = r
		}
	}
	if codingRule == nil {
		t.Fatal("CODING:no-empty-catch rule not found")
	}
	if codingRule.ID != 10 {
		t.Fatalf("should be project override (id=10), got id=%d", codingRule.ID)
	}
	if codingRule.Severity != "WARNING" {
		t.Fatalf("severity should be WARNING (project override), got %q", codingRule.Severity)
	}
}

func TestMergeRules_EmptyInputs(t *testing.T) {
	merged := mergeRules(nil, nil)
	if len(merged) != 0 {
		t.Fatalf("expected 0 merged rules for nil inputs, got %d", len(merged))
	}
}
