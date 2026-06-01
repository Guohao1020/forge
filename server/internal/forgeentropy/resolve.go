package forgeentropy

import (
	"context"
	"log/slog"
	"strings"

	"github.com/multica-ai/multica/server/internal/forge/checks"
	"github.com/multica-ai/multica/server/internal/forge/standards"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

// Querier aggregates the queries ResolveBrief needs. *db.Queries satisfies it.
type Querier interface {
	standards.Querier
	checks.CheckQuerier
	ListOpenEntropyFindings(ctx context.Context, arg db.ListOpenEntropyFindingsParams) ([]db.ListOpenEntropyFindingsRow, error)
}

// ResolveBrief resolves F1 standards + F2 checks + open findings for the scan's
// scope and composes the brief. Best-effort: any resolve/query error degrades
// that section to empty — never blocks dispatch.
func ResolveBrief(ctx context.Context, q Querier, scan db.ForgeEntropyScan) string {
	in := BriefInput{ScanName: scan.Name, CustomFocus: scan.CustomFocus, AutoFix: scan.AutoFix}

	if scan.IncludeStandards {
		if res, err := standards.Resolve(ctx, q, scan.WorkspaceID, scan.ProjectID); err == nil {
			in.StandardsText = strings.TrimSpace(res.Core + "\n\n" + res.Detail)
		} else {
			slog.Warn("forge entropy: resolve standards failed", "error", err)
		}
	}
	if scan.IncludeChecks {
		if cs, err := checks.ResolveChecks(ctx, q, scan.WorkspaceID, scan.ProjectID); err == nil {
			in.ChecksText = formatChecks(cs)
		} else {
			slog.Warn("forge entropy: resolve checks failed", "error", err)
		}
	}
	if fs, err := q.ListOpenEntropyFindings(ctx, db.ListOpenEntropyFindingsParams{
		WorkspaceID: scan.WorkspaceID,
		ProjectID:   scan.ProjectID,
	}); err == nil {
		for _, f := range fs {
			in.OpenFindings = append(in.OpenFindings, FindingRef{Number: f.Number, Title: f.Title})
		}
	} else {
		slog.Warn("forge entropy: list open findings failed", "error", err)
	}
	return ComposeBrief(in)
}

func formatChecks(cs []checks.Check) string {
	var b strings.Builder
	for _, c := range cs {
		b.WriteString("- ")
		b.WriteString(c.Name)
		b.WriteString(": `")
		b.WriteString(c.Command)
		b.WriteString("`\n")
	}
	return strings.TrimRight(b.String(), "\n")
}
