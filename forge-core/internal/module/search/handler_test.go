package search

import (
	"testing"
)

func TestSearchResultTypes(t *testing.T) {
	r := SearchResult{
		Type:      "project",
		ID:        1,
		Title:     "Forge Platform",
		Status:    "ACTIVE",
		URL:       "/projects/1",
	}

	if r.Type != "project" {
		t.Errorf("expected type 'project', got %s", r.Type)
	}
	if r.URL != "/projects/1" {
		t.Errorf("expected URL '/projects/1', got %s", r.URL)
	}
}

func TestSearchResultTask(t *testing.T) {
	r := SearchResult{
		Type:      "task",
		ID:        42,
		ProjectID: 5,
		Title:     "Add login feature",
		URL:       "/projects/5/tasks/42",
	}

	if r.Type != "task" {
		t.Errorf("expected type 'task', got %s", r.Type)
	}
	if r.ProjectID != 5 {
		t.Errorf("expected projectId 5, got %d", r.ProjectID)
	}
}

func TestSearchResponseEmpty(t *testing.T) {
	resp := SearchResponse{
		Query:   "nonexistent",
		Results: []SearchResult{},
		Total:   0,
	}

	if resp.Total != 0 {
		t.Errorf("expected 0 results, got %d", resp.Total)
	}
	if len(resp.Results) != 0 {
		t.Errorf("expected empty results, got %d", len(resp.Results))
	}
}
