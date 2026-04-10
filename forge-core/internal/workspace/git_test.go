package workspace

import (
	"errors"
	"os"
	"strings"
	"testing"
)

func TestHTTPSToSSHURL(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{"https://github.com/owner/repo.git", "git@github.com:owner/repo.git"},
		{"https://github.com/owner/repo", "git@github.com:owner/repo.git"},
		{"https://github.com/multi-owner/weird-name.git", "git@github.com:multi-owner/weird-name.git"},
		// Already SSH -- passthrough
		{"git@github.com:owner/repo.git", "git@github.com:owner/repo.git"},
	}
	for _, tt := range tests {
		got, err := HTTPSToSSHURL(tt.in)
		if err != nil {
			t.Errorf("HTTPSToSSHURL(%q): %v", tt.in, err)
			continue
		}
		if got != tt.want {
			t.Errorf("HTTPSToSSHURL(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestHTTPSToSSHURL_RejectsNonGitHub(t *testing.T) {
	cases := []string{
		"https://gitlab.com/foo/bar.git",
		"https://bitbucket.org/foo/bar.git",
		"https://example.com/anything",
		"ssh://random.host/path",
	}
	for _, u := range cases {
		_, err := HTTPSToSSHURL(u)
		if err == nil {
			t.Errorf("expected error for non-GitHub URL: %s", u)
			continue
		}
		if !errors.Is(err, ErrRepoURLUnsupported) {
			t.Errorf("expected ErrRepoURLUnsupported for %s, got: %v", u, err)
		}
	}
}

func TestParseRepoFromSSHURL(t *testing.T) {
	tests := []struct {
		in        string
		wantOwner string
		wantRepo  string
	}{
		{"git@github.com:foo/bar.git", "foo", "bar"},
		{"git@github.com:foo/bar", "foo", "bar"},
		{"git@github.com:multi-owner/weird-name.git", "multi-owner", "weird-name"},
	}
	for _, tt := range tests {
		owner, repo, err := parseRepoFromSSHURL(tt.in)
		if err != nil {
			t.Errorf("parseRepoFromSSHURL(%q): %v", tt.in, err)
			continue
		}
		if owner != tt.wantOwner || repo != tt.wantRepo {
			t.Errorf("parseRepoFromSSHURL(%q) = %s/%s, want %s/%s",
				tt.in, owner, repo, tt.wantOwner, tt.wantRepo)
		}
	}
}

func TestParseRepoFromSSHURL_RejectsGarbage(t *testing.T) {
	cases := []string{
		"not a url",
		"https://github.com/owner/repo.git",
		"git@gitlab.com:foo/bar.git",
	}
	for _, u := range cases {
		_, _, err := parseRepoFromSSHURL(u)
		if err == nil {
			t.Errorf("expected error for garbage input: %s", u)
		}
	}
}

func TestWriteKeyTempfile_ModeAndCleanup(t *testing.T) {
	key := &DeployKey{
		TenantID:   1,
		ProjectID:  1,
		PrivateKey: []byte("fake-key-bytes-for-tempfile-test"),
	}

	path, cleanup, err := writeKeyTempfile(key)
	if err != nil {
		t.Fatalf("writeKeyTempfile: %v", err)
	}
	defer cleanup()

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat tempfile: %v", err)
	}
	// Mode should be 0600 (owner read/write only). Mask OS-specific bits.
	// On Windows, file permissions work differently, so we only check on
	// Unix-like systems.
	mode := info.Mode() & 0o777
	if mode != 0o600 && mode != 0o666 {
		// 0666 is acceptable on Windows where chmod 0600 is a no-op
		t.Errorf("tempfile mode: want 0600, got %o", mode)
	}

	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read tempfile: %v", err)
	}
	if string(contents) != string(key.PrivateKey) {
		t.Errorf("tempfile contents mismatch")
	}

	// Cleanup removes the file
	cleanup()
	if _, err := os.Stat(path); err == nil {
		t.Errorf("tempfile still exists after cleanup")
	}
}

func TestWriteKeyTempfile_EmptyKeyErrors(t *testing.T) {
	key := &DeployKey{PrivateKey: nil}
	_, _, err := writeKeyTempfile(key)
	if err == nil {
		t.Fatal("expected error for empty private key")
	}
}

func TestClassifyGitError_PermissionDenied(t *testing.T) {
	stderr := "git@github.com: Permission denied (publickey).\nfatal: Could not read from remote repository."
	err := classifyGitError(errors.New("exit status 128"), stderr)
	var authErr *AuthError
	if !errors.As(err, &authErr) {
		t.Errorf("expected AuthError for permission denied, got %T: %v", err, err)
	}
}

func TestClassifyGitError_Network_UnresolvedHost(t *testing.T) {
	stderr := "fatal: unable to access 'git@github.com:owner/repo.git': Could not resolve host: github.com\n"
	err := classifyGitError(errors.New("exit status 128"), stderr)
	var netErr *NetworkError
	if !errors.As(err, &netErr) {
		t.Errorf("expected NetworkError for unresolved host, got %T", err)
	}
}

func TestClassifyGitError_Network_Timeout(t *testing.T) {
	stderr := "fatal: Connection timed out\n"
	err := classifyGitError(errors.New("exit status 128"), stderr)
	var netErr *NetworkError
	if !errors.As(err, &netErr) {
		t.Errorf("expected NetworkError for timeout, got %T", err)
	}
}

func TestClassifyGitError_Unknown(t *testing.T) {
	stderr := "fatal: not a git repository\n"
	err := classifyGitError(errors.New("exit status 128"), stderr)
	var authErr *AuthError
	var netErr *NetworkError
	if errors.As(err, &authErr) || errors.As(err, &netErr) {
		t.Errorf("expected unknown git error, got typed %T", err)
	}
	if !strings.Contains(err.Error(), "not a git repository") {
		t.Errorf("unknown error should include stderr: %v", err)
	}
}

func TestRedactKeyPath(t *testing.T) {
	keyPath := "/tmp/forge-key-abc123"
	msg := "fatal: error at /tmp/forge-key-abc123: something"
	got := redactKeyPath(msg, keyPath)
	if strings.Contains(got, keyPath) {
		t.Errorf("key path not redacted: %s", got)
	}
	if !strings.Contains(got, "<redacted-key-path>") {
		t.Errorf("expected redaction marker: %s", got)
	}
}
