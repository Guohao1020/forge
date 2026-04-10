package workspace

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// gitRunner is the interface the EnsureReady state machine uses for
// git operations. RealGitRunner is the production implementation;
// fakeGitRunner (in ensure_test.go) is the test stub.
//
// The interface is deliberately narrow — three methods — because
// Phase 1a only needs clone, fetch, and reset-hard. Phase 1b may
// rewrite this to carry a *DeployKey argument; the state machine
// callers shield upstream code from that change.
type gitRunner interface {
	Clone(ctx context.Context, hostPath, httpsURL, token, branch string) error
	Fetch(ctx context.Context, hostPath, httpsURL, token string) error
	ResetHard(ctx context.Context, hostPath, branch string) error
}

// RealGitRunner shells out to the system `git` binary via os/exec.
// It builds token-injected HTTPS URLs inline and classifies git
// failures into AuthError / NetworkError / wrapped errors based on
// stderr patterns.
//
// PHASE 1A: uses HTTPS+token via injectToken. Phase 1b rewrites this
// file wholesale to use SSH deploy keys via GIT_SSH_COMMAND and a
// tempfile-managed private key. The temporary HTTPS+token surface is
// intentionally confined to this file per spec §2.9.4.c.
type RealGitRunner struct{}

// NewRealGitRunner constructs a stateless git runner. All arguments
// are passed to each method; the struct holds no per-request state,
// so it's safe to share across goroutines.
func NewRealGitRunner() *RealGitRunner {
	return &RealGitRunner{}
}

// Clone runs `git clone --depth=50 --branch <branch> <injected-url> <hostPath>`.
// The hostPath's parent directory must exist; Clone does not MkdirAll.
// Errors are classified via classifyGitError.
func (r *RealGitRunner) Clone(
	ctx context.Context,
	hostPath, httpsURL, token, branch string,
) error {
	authURL := gitInjectToken(httpsURL, token)
	cmd := exec.CommandContext(ctx, "git", "clone",
		"--depth=50",
		"--branch", branch,
		authURL,
		hostPath,
	)
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return classifyGitError(err, string(out))
	}
	return nil
}

// Fetch runs `git -C <hostPath> fetch <injected-url>`.
// Unlike pull, fetch doesn't try to merge; the caller follows up with
// ResetHard to move the working tree to origin/<branch>. This matches
// the spec §3.7 state machine's resync transition.
func (r *RealGitRunner) Fetch(
	ctx context.Context,
	hostPath, httpsURL, token string,
) error {
	authURL := gitInjectToken(httpsURL, token)
	// Use the injected URL explicitly so we don't depend on the repo's
	// stored `origin` remote — which would also contain a token from
	// the original clone. Being explicit keeps credentials off disk in
	// the git config.
	cmd := exec.CommandContext(ctx, "git", "-C", hostPath, "fetch", authURL)
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return classifyGitError(err, string(out))
	}
	return nil
}

// ResetHard runs `git -C <hostPath> reset --hard FETCH_HEAD`.
// Must be called after a successful Fetch — it moves the working tree
// to whatever the last fetch brought down, discarding any local
// modifications. This is the "reset to clean main" step from spec §2.7.
func (r *RealGitRunner) ResetHard(
	ctx context.Context,
	hostPath, branch string,
) error {
	// FETCH_HEAD is what the preceding Fetch call updated. We could
	// also reset to origin/<branch>, but FETCH_HEAD is more robust
	// against rename/delete of the tracking branch.
	cmd := exec.CommandContext(ctx, "git", "-C", hostPath, "reset", "--hard", "FETCH_HEAD")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return classifyGitError(err, string(out))
	}
	return nil
}

// gitInjectToken converts https://github.com/owner/repo.git to
// https://x-access-token:TOKEN@github.com/owner/repo.git.
// If token is empty, the URL is returned unchanged (used by the
// file:// integration test). If the URL doesn't start with "https://",
// it is also returned unchanged.
//
// PHASE 1A: this helper is the HTTPS+token code path used by
// RealGitRunner. It lives in git.go (not manager.go) so the temporary
// surface is confined to one file — Phase 1b's first task is to delete
// this whole file and replace it with the SSH variant, per spec §2.9.4.c.
//
// Named gitInjectToken to avoid collision with the existing injectToken
// in manager.go. The manager.go version is deleted in Task 1a.5.
//
// Token is never logged (it's never written to slog anywhere in this file).
func gitInjectToken(repoURL, token string) string {
	if token == "" {
		return repoURL
	}
	if !strings.HasPrefix(repoURL, "https://") {
		return repoURL
	}
	return strings.Replace(repoURL, "https://", fmt.Sprintf("https://x-access-token:%s@", token), 1)
}

// AuthError signals a git authentication failure (wrong/missing/
// revoked credentials). The state machine uses errors.As to detect
// this and produces a last_error containing "github_auth_failed" for
// the Phase 1a-only failure mode in spec §3.12.
type AuthError struct {
	stderr string
}

func (e *AuthError) Error() string {
	return "git auth failed: " + firstLine(e.stderr)
}

// NetworkError signals a git network failure (DNS, timeout, 5xx from
// github). Distinguished from auth because retry semantics differ —
// network failures often resolve on their own.
type NetworkError struct {
	stderr string
}

func (e *NetworkError) Error() string {
	return "git network error: " + firstLine(e.stderr)
}

// classifyGitError inspects git stderr and maps it to a typed error
// that the state machine can distinguish via errors.As. Unknown
// failures are wrapped as-is so the caller sees the full stderr.
func classifyGitError(baseErr error, stderr string) error {
	lower := strings.ToLower(stderr)
	switch {
	case strings.Contains(lower, "authentication") ||
		strings.Contains(lower, "401") ||
		strings.Contains(lower, "could not read username") ||
		strings.Contains(lower, "terminal prompts disabled"):
		return &AuthError{stderr: stderr}
	case strings.Contains(lower, "could not resolve host") ||
		strings.Contains(lower, "connection timed out") ||
		strings.Contains(lower, "connection refused") ||
		strings.Contains(lower, "network is unreachable"):
		return &NetworkError{stderr: stderr}
	default:
		return fmt.Errorf("git: %w: %s", baseErr, firstLine(stderr))
	}
}

// firstLine returns the first non-empty line of a string. Used by
// error constructors to keep error messages short — git's stderr is
// often multi-line and the first line is usually the most useful.
func firstLine(s string) string {
	for _, line := range strings.Split(s, "\n") {
		if trimmed := strings.TrimSpace(line); trimmed != "" {
			return trimmed
		}
	}
	return ""
}
