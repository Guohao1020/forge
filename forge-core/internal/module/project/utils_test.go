package project

import (
	"testing"
)

func TestParseOwnerRepo(t *testing.T) {
	tests := []struct {
		url       string
		wantOwner string
		wantRepo  string
	}{
		{"https://github.com/shulex/forge.git", "shulex", "forge"},
		{"https://github.com/shulex/forge", "shulex", "forge"},
		{"https://github.com/owner/my-repo.git", "owner", "my-repo"},
		{"git@github.com:owner/repo.git", "git@github.com:owner", "repo"}, // SSH URL not fully supported
		{"", "", ""},
		{"nopath", "", ""},
	}

	for _, tt := range tests {
		owner, repo := parseOwnerRepo(tt.url)
		if owner != tt.wantOwner || repo != tt.wantRepo {
			t.Errorf("parseOwnerRepo(%q) = (%q, %q), want (%q, %q)",
				tt.url, owner, repo, tt.wantOwner, tt.wantRepo)
		}
	}
}
