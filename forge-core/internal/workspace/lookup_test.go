package workspace

import (
	"context"
	"errors"
	"testing"
)

func TestStaticProjectLookup_ReturnsInfo(t *testing.T) {
	lookup := &StaticProjectLookup{
		Info: &ProjectInfo{
			ProjectID:     42,
			TenantID:      1,
			SSHURL:        "git@github.com:owner/repo.git",
			DefaultBranch: "main",
			CreatedBy:     1,
		},
		Token: "ghp_test",
	}
	info, err := lookup.Lookup(context.Background(), 1, 42)
	if err != nil {
		t.Fatalf("Lookup: %v", err)
	}
	if info.SSHURL != "git@github.com:owner/repo.git" {
		t.Errorf("SSHURL = %q, want %q", info.SSHURL, "git@github.com:owner/repo.git")
	}
	if info.DefaultBranch != "main" {
		t.Errorf("DefaultBranch = %q, want %q", info.DefaultBranch, "main")
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

func TestStaticProjectLookup_GetOwnerGitHubToken(t *testing.T) {
	lookup := &StaticProjectLookup{Token: "ghp_test"}
	tok, err := lookup.GetOwnerGitHubToken(context.Background(), 42)
	if err != nil {
		t.Fatalf("GetOwnerGitHubToken: %v", err)
	}
	if tok != "ghp_test" {
		t.Errorf("token = %q, want %q", tok, "ghp_test")
	}
}

func TestProjectLookupInterface(t *testing.T) {
	// Verify both implementations satisfy the interface at compile time.
	var _ ProjectLookup = &DBProjectLookup{}
	var _ ProjectLookup = &StaticProjectLookup{}
	var _ ProjectLookup = &memoryLookup{}
}
