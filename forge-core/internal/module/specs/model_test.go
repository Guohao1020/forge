package specs

import "testing"

func TestStandardFilter_Defaults(t *testing.T) {
	f := StandardFilter{}
	if f.Page != 0 {
		t.Error("default page should be 0")
	}
	if f.PageSize != 0 {
		t.Error("default page size should be 0")
	}
}

func TestPromptTemplateFilter(t *testing.T) {
	f := PromptTemplateFilter{
		Purpose: "code-generation",
		Page:    1,
	}
	if f.Purpose != "code-generation" {
		t.Errorf("expected purpose code-generation, got %s", f.Purpose)
	}
}

func TestReviewRuleFilter(t *testing.T) {
	f := ReviewRuleFilter{
		Category: "security",
		Severity: "ERROR",
	}
	if f.Category != "security" {
		t.Errorf("expected category security, got %s", f.Category)
	}
}

func TestEffectiveSpecs(t *testing.T) {
	specs := EffectiveSpecs{
		Standards: []*Standard{
			{Name: "Go Style"},
		},
		Rules: []*ReviewRule{
			{Name: "no-console"},
		},
	}
	if len(specs.Standards) != 1 {
		t.Errorf("expected 1 standard, got %d", len(specs.Standards))
	}
	if len(specs.Rules) != 1 {
		t.Errorf("expected 1 rule, got %d", len(specs.Rules))
	}
}

func TestPageResult(t *testing.T) {
	result := PageResult[*Standard]{
		Items: []*Standard{
			{Name: "test"},
		},
		Total: 1,
		Page:  1,
	}
	if result.Total != 1 {
		t.Errorf("expected total 1, got %d", result.Total)
	}
	if len(result.Items) != 1 {
		t.Errorf("expected 1 item, got %d", len(result.Items))
	}
}

func TestScaffoldTemplate(t *testing.T) {
	s := ScaffoldTemplate{
		Name:        "Go API",
		ProjectType: "backend_api",
	}
	if s.Name != "Go API" {
		t.Errorf("expected Go API, got %s", s.Name)
	}
	if s.ProjectType != "backend_api" {
		t.Errorf("expected backend_api, got %s", s.ProjectType)
	}
}
