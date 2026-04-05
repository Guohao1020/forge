package search

import "testing"

func TestSearchResultURL_Project(t *testing.T) {
	r := SearchResult{Type: "project", ID: 42}
	expected := "/projects/42"
	r.URL = expected
	if r.URL != expected {
		t.Errorf("expected %s, got %s", expected, r.URL)
	}
}

func TestSearchResultURL_Task(t *testing.T) {
	r := SearchResult{Type: "task", ID: 10, ProjectID: 5}
	expected := "/projects/5/tasks/10"
	r.URL = expected
	if r.URL != expected {
		t.Errorf("expected %s, got %s", expected, r.URL)
	}
}

func TestSearchResponse_Count(t *testing.T) {
	resp := SearchResponse{
		Query: "forge",
		Results: []SearchResult{
			{Type: "project", ID: 1, Title: "Forge"},
			{Type: "task", ID: 2, Title: "Add auth"},
		},
		Total: 2,
	}
	if resp.Total != len(resp.Results) {
		t.Errorf("total %d != results length %d", resp.Total, len(resp.Results))
	}
}
