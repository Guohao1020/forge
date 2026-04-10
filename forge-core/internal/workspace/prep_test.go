package workspace

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestPrepClient_HappyPath(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("want POST, got %s", r.Method)
		}
		if r.URL.Path != "/api/workspace/prep" {
			t.Errorf("want /api/workspace/prep, got %s", r.URL.Path)
		}
		var req PrepRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if req.TenantID != 1 || req.ProjectID != 42 {
			t.Errorf("wrong tenant/project: %d/%d", req.TenantID, req.ProjectID)
		}
		json.NewEncoder(w).Encode(PrepResponse{
			Status:   "ok",
			Language: "go",
			Command:  "go mod download",
		})
	}))
	defer srv.Close()

	client := NewPrepClient(srv.URL)
	resp, err := client.Prep(context.Background(), PrepRequest{
		TenantID:      1,
		ProjectID:     42,
		WorkspacePath: "tenant-1/project-42/repo",
	})
	if err != nil {
		t.Fatalf("Prep: %v", err)
	}
	if resp.Status != "ok" {
		t.Errorf("want ok, got %s", resp.Status)
	}
	if resp.Language != "go" {
		t.Errorf("want go, got %s", resp.Language)
	}
}

func TestPrepClient_ServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	client := NewPrepClient(srv.URL)
	_, err := client.Prep(context.Background(), PrepRequest{TenantID: 1, ProjectID: 1, WorkspacePath: "x"})
	if err == nil {
		t.Fatal("expected error for 500 response")
	}
}

func TestPrepClient_PrepSkipped(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(PrepResponse{Status: "skipped"})
	}))
	defer srv.Close()

	client := NewPrepClient(srv.URL)
	resp, err := client.Prep(context.Background(), PrepRequest{TenantID: 1, ProjectID: 1, WorkspacePath: "x"})
	if err != nil {
		t.Fatalf("Prep: %v", err)
	}
	if resp.Status != "skipped" {
		t.Errorf("want skipped, got %s", resp.Status)
	}
}
