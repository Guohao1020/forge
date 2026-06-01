// Package forgeentropy composes the scanner agent's brief for a Forge entropy
// scan: F1 standards + F2 checks + custom focus + an open-findings dedup list.
// Service-free (imports db + forge/standards + forge/checks) so the autopilot
// dispatch path can call it without an import cycle.
package forgeentropy

import (
	"fmt"
	"strings"
)

// FindingLabel marks scanner-filed finding issues so the dedup query can find them.
const FindingLabel = "forge-entropy"

// FindingRef is one already-tracked finding for the dedup list.
type FindingRef struct {
	Number int32
	Title  string
}

// BriefInput is the fully-resolved input to ComposeBrief (no I/O).
type BriefInput struct {
	ScanName      string
	StandardsText string // "" = omit section
	ChecksText    string // "" = omit section
	CustomFocus   string // "" = omit section
	OpenFindings  []FindingRef
}

// ComposeBrief builds the scanner agent's Markdown brief. Pure — unit-testable.
func ComposeBrief(in BriefInput) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# Entropy Scan: %s\n\n", in.ScanName)
	b.WriteString("You are performing a periodic, WHOLE-REPOSITORY quality scan (not a diff review).\n")
	b.WriteString("Survey the entire codebase for accumulated quality entropy and FILE issues for findings.\n")
	b.WriteString("This is advisory — do NOT modify code in this task; only survey and report.\n")

	if in.StandardsText != "" {
		b.WriteString("\n## This project's declared standards (F1)\n")
		b.WriteString(in.StandardsText)
		b.WriteString("\n")
	}
	if in.ChecksText != "" {
		b.WriteString("\n## This project's verification checks (F2)\n")
		b.WriteString(in.ChecksText)
		b.WriteString("\n")
	}
	if in.CustomFocus != "" {
		b.WriteString("\n## Additional focus areas\n")
		b.WriteString(in.CustomFocus)
		b.WriteString("\n")
	}
	if len(in.OpenFindings) > 0 {
		b.WriteString("\n## Already-tracked findings — do NOT re-file these\n")
		for _, f := range in.OpenFindings {
			fmt.Fprintf(&b, "- #%d %s\n", f.Number, f.Title)
		}
		b.WriteString("For each item above that still exists, add a short comment confirming it persists.\n")
		b.WriteString("Only create NEW issues for findings NOT already listed.\n")
	}
	b.WriteString("\n## How to report\n")
	b.WriteString("For each NEW finding, create an issue via the `multica` CLI:\n")
	b.WriteString("- clear title, body with problem + location + suggested fix\n")
	fmt.Fprintf(&b, "- apply the label `%s`\n", FindingLabel)
	b.WriteString("When done, post a summary comment on THIS scan issue.\n")
	return b.String()
}
