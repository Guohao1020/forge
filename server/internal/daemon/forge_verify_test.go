package daemon

import (
	"context"
	"log/slog"
	"strings"
	"testing"
)

func TestRunChecks_FailureSummary(t *testing.T) {
	dir := t.TempDir()
	checks := []ForgeCheck{
		{Name: "ok", Command: "exit 0"},
		{Name: "bad", Command: "echo boom; exit 1"},
	}
	got := runChecks(context.Background(), dir, checks, slog.Default())
	if !strings.Contains(got, "bad") || !strings.Contains(got, "boom") {
		t.Fatalf("failure summary must name the failed check + its output; got %q", got)
	}
}

func TestRunChecks_AllPass(t *testing.T) {
	dir := t.TempDir()
	got := runChecks(context.Background(), dir, []ForgeCheck{{Name: "ok", Command: "exit 0"}}, slog.Default())
	if got != "" {
		t.Fatalf("all-pass must yield empty summary; got %q", got)
	}
}
