package project

import (
	"testing"
)

func TestBuiltInTemplates(t *testing.T) {
	templates := BuiltInTemplates()

	if len(templates) < 5 {
		t.Errorf("expected at least 5 templates, got %d", len(templates))
	}

	// All templates should have required fields
	ids := make(map[string]bool)
	for _, tmpl := range templates {
		if tmpl.ID == "" {
			t.Error("template ID should not be empty")
		}
		if ids[tmpl.ID] {
			t.Errorf("duplicate template ID: %s", tmpl.ID)
		}
		ids[tmpl.ID] = true

		if tmpl.Name == "" {
			t.Errorf("template %s: name should not be empty", tmpl.ID)
		}
		if tmpl.Language == "" {
			t.Errorf("template %s: language should not be empty", tmpl.ID)
		}
		if tmpl.Category == "" {
			t.Errorf("template %s: category should not be empty", tmpl.ID)
		}
		if len(tmpl.Frameworks) == 0 {
			t.Errorf("template %s: should have at least one framework", tmpl.ID)
		}
	}
}

func TestBuiltInTemplates_Categories(t *testing.T) {
	templates := BuiltInTemplates()

	categories := make(map[string]int)
	for _, tmpl := range templates {
		categories[tmpl.Category]++
	}

	if categories["backend"] < 2 {
		t.Errorf("expected at least 2 backend templates, got %d", categories["backend"])
	}
	if categories["frontend"] < 1 {
		t.Errorf("expected at least 1 frontend template, got %d", categories["frontend"])
	}
}

func TestBuiltInTemplates_GoAPI(t *testing.T) {
	templates := BuiltInTemplates()

	var found bool
	for _, tmpl := range templates {
		if tmpl.ID == "go-api" {
			found = true
			if tmpl.Language != "go" {
				t.Errorf("go-api template: expected language 'go', got %s", tmpl.Language)
			}
			if tmpl.Category != "backend" {
				t.Errorf("go-api template: expected category 'backend', got %s", tmpl.Category)
			}
			break
		}
	}
	if !found {
		t.Error("go-api template not found")
	}
}
