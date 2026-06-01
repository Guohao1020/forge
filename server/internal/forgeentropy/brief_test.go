package forgeentropy

import (
	"strings"
	"testing"
)

func TestComposeBrief_AllSections(t *testing.T) {
	out := ComposeBrief(BriefInput{
		ScanName:      "weekly",
		StandardsText: "Always write tests.",
		ChecksText:    "- lint: `make lint`",
		CustomFocus:   "Check for dead code.",
		OpenFindings:  []FindingRef{{Number: 12, Title: "TODO debt in auth"}},
	})
	for _, want := range []string{
		"# Entropy Scan: weekly",
		"declared standards (F1)",
		"Always write tests.",
		"verification checks (F2)",
		"make lint",
		"Additional focus areas",
		"Check for dead code.",
		"Already-tracked findings",
		"#12 TODO debt in auth",
		"label `forge-entropy`",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("brief missing %q\n---\n%s", want, out)
		}
	}
}

func TestComposeBrief_OmitsEmptySections(t *testing.T) {
	out := ComposeBrief(BriefInput{ScanName: "minimal"})
	for _, absent := range []string{
		"declared standards (F1)",
		"verification checks (F2)",
		"Additional focus areas",
		"Already-tracked findings",
	} {
		if strings.Contains(out, absent) {
			t.Fatalf("brief should omit %q when empty\n---\n%s", absent, out)
		}
	}
	if !strings.Contains(out, "WHOLE-REPOSITORY") || !strings.Contains(out, "How to report") {
		t.Fatalf("brief missing always-on sections\n---\n%s", out)
	}
}
