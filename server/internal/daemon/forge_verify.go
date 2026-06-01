package daemon

import (
	"context"
	"fmt"
	"log/slog"
	"os/exec"
	"strings"
	"time"
)

// ForgeCheck mirrors the server's forge.Check (name + shell command).
type ForgeCheck struct {
	Name    string `json:"name"`
	Command string `json:"command"`
}

type forgeChecksResult struct {
	Checks []ForgeCheck `json:"checks"`
}

// GetForgeChecks fetches resolved verification checks for a task.
func (c *Client) GetForgeChecks(ctx context.Context, taskID string) (*forgeChecksResult, error) {
	var resp forgeChecksResult
	if err := c.getJSON(ctx, fmt.Sprintf("/api/daemon/tasks/%s/forge-checks", taskID), &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

const (
	forgeCheckTimeout   = 5 * time.Minute
	forgeCheckOutputCap = 4096
)

// runChecks runs each check command in workDir (via `bash -lc`). Returns a
// formatted failure summary; empty string = all passed / nothing to run.
// Pure (no client / Daemon) so it is unit-testable.
func runChecks(ctx context.Context, workDir string, checks []ForgeCheck, log *slog.Logger) string {
	var failures []string
	for _, ch := range checks {
		cctx, cancel := context.WithTimeout(ctx, forgeCheckTimeout)
		cmd := exec.CommandContext(cctx, "bash", "-lc", ch.Command)
		cmd.Dir = workDir
		out, err := cmd.CombinedOutput()
		cancel()
		if err != nil {
			tail := strings.TrimSpace(string(out))
			if len(tail) > forgeCheckOutputCap {
				tail = tail[len(tail)-forgeCheckOutputCap:]
			}
			failures = append(failures,
				fmt.Sprintf("- **%s** (`%s`) failed: %v\n```\n%s\n```", ch.Name, ch.Command, err, tail))
			log.Warn("forge check failed", "check", ch.Name, "error", err)
		} else {
			log.Info("forge check passed", "check", ch.Name)
		}
	}
	if len(failures) == 0 {
		return ""
	}
	return fmt.Sprintf("❌ Verification failed (%d check(s)):\n\n%s", len(failures), strings.Join(failures, "\n\n"))
}

// runForgeChecks fetches the task's checks and runs them in workDir.
// Best-effort: fetch error → "" (no gate), logged.
func (d *Daemon) runForgeChecks(ctx context.Context, taskID, workDir string, log *slog.Logger) string {
	if workDir == "" {
		return ""
	}
	res, err := d.client.GetForgeChecks(ctx, taskID)
	if err != nil {
		log.Warn("forge: fetch checks failed; skipping gate", "error", err)
		return ""
	}
	if res == nil || len(res.Checks) == 0 {
		return ""
	}
	return runChecks(ctx, workDir, res.Checks, log)
}
