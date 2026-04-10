package workspace

import (
	"errors"
	"strings"
	"testing"
)

func TestGitInjectToken_GitHubHTTPS(t *testing.T) {
	got := gitInjectToken("https://github.com/owner/repo.git", "ghp_fake_token")
	want := "https://x-access-token:ghp_fake_token@github.com/owner/repo.git"
	if got != want {
		t.Errorf("gitInjectToken: got %s, want %s", got, want)
	}
}

func TestGitInjectToken_EmptyToken(t *testing.T) {
	got := gitInjectToken("https://github.com/owner/repo.git", "")
	want := "https://github.com/owner/repo.git"
	if got != want {
		t.Errorf("gitInjectToken empty: got %s, want %s", got, want)
	}
}

func TestGitInjectToken_NonHTTPSUnchanged(t *testing.T) {
	// file:// URLs (used by integration tests) should pass through
	// unchanged — there's no https:// prefix to replace.
	got := gitInjectToken("file:///tmp/bare-repo.git", "ghp_token")
	want := "file:///tmp/bare-repo.git"
	if got != want {
		t.Errorf("gitInjectToken file://: got %s, want %s", got, want)
	}
}

func TestClassifyGitError_Auth(t *testing.T) {
	stderr := "fatal: Authentication failed for 'https://github.com/owner/repo.git/'\n"
	err := classifyGitError(errors.New("exit status 128"), stderr)
	var authErr *AuthError
	if !errors.As(err, &authErr) {
		t.Errorf("expected AuthError, got %T: %v", err, err)
	}
}

func TestClassifyGitError_401(t *testing.T) {
	stderr := "remote: The requested URL returned error: 401\n"
	err := classifyGitError(errors.New("exit status 128"), stderr)
	var authErr *AuthError
	if !errors.As(err, &authErr) {
		t.Errorf("expected AuthError for 401, got %T", err)
	}
}

func TestClassifyGitError_Network_UnresolvedHost(t *testing.T) {
	stderr := "fatal: unable to access 'https://github.com/owner/repo.git/': Could not resolve host: github.com\n"
	err := classifyGitError(errors.New("exit status 128"), stderr)
	var netErr *NetworkError
	if !errors.As(err, &netErr) {
		t.Errorf("expected NetworkError for unresolved host, got %T", err)
	}
}

func TestClassifyGitError_Network_Timeout(t *testing.T) {
	stderr := "fatal: unable to access 'https://github.com/owner/repo.git/': Connection timed out\n"
	err := classifyGitError(errors.New("exit status 128"), stderr)
	var netErr *NetworkError
	if !errors.As(err, &netErr) {
		t.Errorf("expected NetworkError for timeout, got %T", err)
	}
}

func TestClassifyGitError_Unknown(t *testing.T) {
	stderr := "fatal: not a git repository\n"
	err := classifyGitError(errors.New("exit status 128"), stderr)
	// Should NOT be auth or network
	var authErr *AuthError
	var netErr *NetworkError
	if errors.As(err, &authErr) || errors.As(err, &netErr) {
		t.Errorf("expected unknown git error, got typed %T", err)
	}
	if !strings.Contains(err.Error(), "not a git repository") {
		t.Errorf("unknown error should include stderr: %v", err)
	}
}
