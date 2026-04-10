package workspace

import (
	"context"
	"errors"
	"testing"
)

func TestStaticProjectLookup_ReturnsInfo(t *testing.T) {
	lookup := &StaticProjectLookup{
		Info: &ProjectInfo{
			RepoURL:     "https://github.com/owner/repo.git",
			AccessToken: "ghp_test",
			Branch:      "main",
		},
	}
	info, err := lookup.Lookup(context.Background(), 1, 42)
	if err != nil {
		t.Fatalf("Lookup: %v", err)
	}
	if info.RepoURL != "https://github.com/owner/repo.git" {
		t.Errorf("RepoURL = %q, want %q", info.RepoURL, "https://github.com/owner/repo.git")
	}
	if info.AccessToken != "ghp_test" {
		t.Errorf("AccessToken = %q, want %q", info.AccessToken, "ghp_test")
	}
	if info.Branch != "main" {
		t.Errorf("Branch = %q, want %q", info.Branch, "main")
	}
}

func TestStaticProjectLookup_ReturnsError(t *testing.T) {
	lookup := &StaticProjectLookup{
		Err: errors.New("not found"),
	}
	_, err := lookup.Lookup(context.Background(), 1, 999)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if err.Error() != "not found" {
		t.Errorf("error = %q, want %q", err.Error(), "not found")
	}
}

func TestStaticProjectLookup_NilInfo(t *testing.T) {
	lookup := &StaticProjectLookup{}
	info, err := lookup.Lookup(context.Background(), 1, 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if info != nil {
		t.Errorf("expected nil info, got %+v", info)
	}
}

func TestProjectLookupInterface(t *testing.T) {
	// Verify both implementations satisfy the interface at compile time.
	var _ ProjectLookup = &DBProjectLookup{}
	var _ ProjectLookup = &StaticProjectLookup{}
}
