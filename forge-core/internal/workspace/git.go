package workspace

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

// ErrRepoURLUnsupported is returned by HTTPSToSSHURL when the URL does
// not point at github.com. Round 2 only supports GitHub; callers are
// expected to halt with this error rather than fall back.
var ErrRepoURLUnsupported = errors.New("workspace: repo URL is not supported (only github.com is supported)")

// GitRunner is the interface the state machine (ensure.go) uses to run
// git operations. Having it as an interface keeps EnsureReady testable
// with a fake runner (the Phase 1a ensure_test.go fixtures already use
// this pattern).
//
// Phase 1b: Clone and FetchAndResetHard both take a *DeployKey argument
// instead of the Phase 1a (httpsURL, token) pair.
type GitRunner interface {
	Clone(ctx context.Context, sshURL, dir string, key *DeployKey, branch string) error
	FetchAndResetHard(ctx context.Context, dir, branch string, key *DeployKey) error
}

// RealGitRunner is the production GitRunner. It shells out to the
// system `git` binary with GIT_SSH_COMMAND wired to a tempfile holding
// the decrypted deploy key's private bytes.
//
// Note: RealGitRunner does NOT hold the CryptoService or DeployKeyRepo
// directly -- EnsureReady resolves the DeployKey and passes it in. This
// keeps RealGitRunner unaware of storage concerns and testable with
// any DeployKey struct.
type RealGitRunner struct {
	// knownHostsDir is the directory where per-tenant known_hosts files
	// live. If empty, defaults to os.TempDir().
	knownHostsDir string
}

// NewRealGitRunner constructs a RealGitRunner. Pass an empty string for
// knownHostsDir to use os.TempDir() (the default in production since
// known_hosts entries survive only across the lifetime of the tenant
// known_hosts file, not across container restarts).
func NewRealGitRunner(knownHostsDir string) *RealGitRunner {
	if knownHostsDir == "" {
		knownHostsDir = os.TempDir()
	}
	return &RealGitRunner{knownHostsDir: knownHostsDir}
}

// Clone does a `git clone --depth 50 --branch <branch> <sshURL> <dir>`
// using the deploy key for auth. The target directory is wiped first
// so git clone won't refuse on "target not empty".
func (r *RealGitRunner) Clone(
	ctx context.Context,
	sshURL, dir string,
	key *DeployKey,
	branch string,
) error {
	keyPath, cleanup, err := writeKeyTempfile(key)
	if err != nil {
		return fmt.Errorf("git clone: prepare key: %w", err)
	}
	defer cleanup()

	knownHosts := filepath.Join(r.knownHostsDir,
		fmt.Sprintf("forge-known-hosts-%d", key.TenantID))

	// Make sure the parent directory exists and the target doesn't -- git
	// clone requires the destination to not exist or to be empty.
	if err := os.MkdirAll(filepath.Dir(dir), 0o755); err != nil {
		return fmt.Errorf("mkdir parent: %w", err)
	}
	if err := os.RemoveAll(dir); err != nil {
		return fmt.Errorf("clean target: %w", err)
	}

	args := []string{"clone", "--depth", "50", "--branch", branch, sshURL, dir}
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Env = gitEnvWithSSHKey(keyPath, knownHosts)

	out, err := cmd.CombinedOutput()
	if err != nil {
		return classifyGitError(err, redactKeyPath(string(out), keyPath))
	}
	return nil
}

// FetchAndResetHard runs `git fetch origin <branch>` followed by
// `git reset --hard origin/<branch>` inside an existing clone. This is
// the new-session resync path.
func (r *RealGitRunner) FetchAndResetHard(
	ctx context.Context,
	dir, branch string,
	key *DeployKey,
) error {
	keyPath, cleanup, err := writeKeyTempfile(key)
	if err != nil {
		return fmt.Errorf("git fetch: prepare key: %w", err)
	}
	defer cleanup()

	knownHosts := filepath.Join(r.knownHostsDir,
		fmt.Sprintf("forge-known-hosts-%d", key.TenantID))

	// fetch origin <branch>
	fetch := exec.CommandContext(ctx, "git", "-C", dir, "fetch", "origin", branch)
	fetch.Env = gitEnvWithSSHKey(keyPath, knownHosts)
	if out, err := fetch.CombinedOutput(); err != nil {
		return classifyGitError(err, redactKeyPath(string(out), keyPath))
	}

	// reset --hard origin/<branch>
	reset := exec.CommandContext(ctx, "git", "-C", dir, "reset", "--hard", "origin/"+branch)
	reset.Env = gitEnvWithSSHKey(keyPath, knownHosts)
	if out, err := reset.CombinedOutput(); err != nil {
		return fmt.Errorf("git reset --hard: %w\n%s", err, string(out))
	}
	return nil
}

// writeKeyTempfile writes the deploy key's private bytes to a tempfile
// with mode 0600 and returns the path + a cleanup function that
// unconditionally removes the file. The cleanup is safe to call on all
// paths including panic because it just does os.Remove.
//
// The filename has a random suffix so concurrent git operations for
// different projects don't collide.
func writeKeyTempfile(key *DeployKey) (string, func(), error) {
	if len(key.PrivateKey) == 0 {
		return "", nil, fmt.Errorf("workspace: writeKeyTempfile: empty private key for project %d", key.ProjectID)
	}

	var rb [8]byte
	if _, err := rand.Read(rb[:]); err != nil {
		return "", nil, fmt.Errorf("rand: %w", err)
	}
	path := filepath.Join(os.TempDir(), "forge-key-"+hex.EncodeToString(rb[:]))

	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return "", nil, fmt.Errorf("create key tempfile: %w", err)
	}
	if _, err := f.Write(key.PrivateKey); err != nil {
		_ = f.Close()
		_ = os.Remove(path)
		return "", nil, fmt.Errorf("write key tempfile: %w", err)
	}
	if err := f.Close(); err != nil {
		_ = os.Remove(path)
		return "", nil, fmt.Errorf("close key tempfile: %w", err)
	}

	// Defensive: OpenFile with 0600 should yield 0600, but umask or
	// unusual filesystems can interfere. Force it explicitly.
	if err := os.Chmod(path, 0o600); err != nil {
		_ = os.Remove(path)
		return "", nil, fmt.Errorf("chmod key tempfile: %w", err)
	}

	cleanup := func() {
		_ = os.Remove(path)
	}
	return path, cleanup, nil
}

// gitEnvWithSSHKey returns an env slice suitable for os/exec.Cmd.Env
// that sets GIT_SSH_COMMAND to use the given key file and known_hosts.
//
// Flags explained:
//
//	-i <keyPath>                          Use this key, this key only
//	-o StrictHostKeyChecking=accept-new   First-connect: accept and pin;
//	                                      later divergence: reject (MITM resistance)
//	-o IdentitiesOnly=yes                 Don't fall back to other keys in the
//	                                      ssh-agent (deterministic behaviour)
//	-o BatchMode=yes                      Never prompt (so hangs fail loudly)
//	-o UserKnownHostsFile=<per-tenant>    Per-tenant known_hosts isolation:
//	                                      one tenant's MITM cannot poison
//	                                      another tenant's pinning
//
// GIT_TERMINAL_PROMPT=0 also disables git's own prompt for credentials.
func gitEnvWithSSHKey(keyPath, knownHostsPath string) []string {
	sshCmd := fmt.Sprintf(
		"ssh -i %s -o StrictHostKeyChecking=accept-new -o IdentitiesOnly=yes -o BatchMode=yes -o UserKnownHostsFile=%s",
		keyPath, knownHostsPath,
	)
	env := append(os.Environ(),
		"GIT_SSH_COMMAND="+sshCmd,
		"GIT_TERMINAL_PROMPT=0",
	)
	return env
}

// redactKeyPath removes any occurrence of keyPath from a git error
// message so error logs never leak the tempfile location.
func redactKeyPath(s, keyPath string) string {
	return strings.ReplaceAll(s, keyPath, "<redacted-key-path>")
}

var httpsGitHubRe = regexp.MustCompile(`^https://github\.com/([^/]+)/([^/]+?)(\.git)?$`)

// HTTPSToSSHURL converts a GitHub HTTPS URL to the SSH form
// git@github.com:owner/repo.git. Idempotent on SSH URLs. Returns
// ErrRepoURLUnsupported for non-GitHub hosts -- Round 2 only supports
// GitHub.
func HTTPSToSSHURL(u string) (string, error) {
	// Passthrough for SSH
	if strings.HasPrefix(u, "git@github.com:") {
		return u, nil
	}
	m := httpsGitHubRe.FindStringSubmatch(u)
	if m == nil {
		return "", fmt.Errorf("%w: %q", ErrRepoURLUnsupported, u)
	}
	owner, repo := m[1], m[2]
	return fmt.Sprintf("git@github.com:%s/%s.git", owner, repo), nil
}

var sshGitHubRe = regexp.MustCompile(`^git@github\.com:([^/]+)/([^/]+?)(\.git)?$`)

// parseRepoFromSSHURL extracts (owner, repo) from an SSH-form GitHub URL.
// Used by the GitHub deploy key upload path to construct the API URL.
func parseRepoFromSSHURL(u string) (string, string, error) {
	m := sshGitHubRe.FindStringSubmatch(u)
	if m == nil {
		return "", "", fmt.Errorf("parseRepoFromSSHURL: %q is not a github SSH URL", u)
	}
	return m[1], m[2], nil
}

// AuthError signals a git authentication failure (wrong/missing/
// revoked credentials). The state machine uses errors.As to detect
// this and produces a last_error containing an auth-related message.
type AuthError struct {
	stderr string
}

func (e *AuthError) Error() string {
	return "git auth failed: " + firstLine(e.stderr)
}

// NetworkError signals a git network failure (DNS, timeout, 5xx from
// github). Distinguished from auth because retry semantics differ --
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
	case strings.Contains(lower, "permission denied") ||
		strings.Contains(lower, "host key verification failed") ||
		strings.Contains(lower, "no matching host key") ||
		strings.Contains(lower, "authentication") ||
		strings.Contains(lower, "could not read from remote repository"):
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
// error constructors to keep error messages short -- git's stderr is
// often multi-line and the first line is usually the most useful.
func firstLine(s string) string {
	for _, line := range strings.Split(s, "\n") {
		if trimmed := strings.TrimSpace(line); trimmed != "" {
			return trimmed
		}
	}
	return ""
}
