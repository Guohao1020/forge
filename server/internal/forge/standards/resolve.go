// Package standards implements Forge's spec-center: resolving categorized,
// scoped, profile-filtered coding standards into a two-layer payload —
// Core (mandatory, appended to agent instructions) and Detail (compiled into
// an on-demand forge-standards skill). Forge-owned; isolated from Multica core.
package standards

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

// SkillName is the fixed name of the auto-generated standards skill.
const SkillName = "forge-standards"

// Resolved is the two-layer injection payload.
type Resolved struct {
	Core   string // markdown appended to agent instructions (mandatory, always-on)
	Detail string // SKILL.md content for the forge-standards skill; "" if none
}

// resolveStandards is the pure core: override workspace standards with project
// standards by (category,name), filter by profile tags, then split into
// Core/Detail. No I/O — fully unit-testable.
func resolveStandards(ws, proj []db.ForgeStandard, projTags []string) Resolved {
	type key struct{ cat, name string }
	merged := map[key]db.ForgeStandard{}
	for _, s := range ws {
		merged[key{s.Category, s.Name}] = s
	}
	for _, s := range proj { // project overrides workspace
		merged[key{s.Category, s.Name}] = s
	}

	tagSet := map[string]bool{}
	for _, t := range projTags {
		tagSet[t] = true
	}
	applies := func(s db.ForgeStandard) bool {
		if len(s.ProfileTags) == 0 {
			return true // empty = applies to all
		}
		for _, t := range s.ProfileTags {
			if tagSet[t] {
				return true
			}
		}
		return false
	}

	var kept []db.ForgeStandard
	for _, s := range merged {
		if applies(s) {
			kept = append(kept, s)
		}
	}
	// Deterministic order: category, then name.
	sort.Slice(kept, func(i, j int) bool {
		if kept[i].Category != kept[j].Category {
			return kept[i].Category < kept[j].Category
		}
		return kept[i].Name < kept[j].Name
	})

	var core, detail strings.Builder
	for _, s := range kept {
		if c := strings.TrimSpace(s.CoreContent); c != "" {
			fmt.Fprintf(&core, "### [%s] %s\n%s\n\n", s.Category, s.Name, c)
		}
		if d := strings.TrimSpace(s.DetailContent); d != "" {
			fmt.Fprintf(&detail, "## [%s] %s\n%s\n\n", s.Category, s.Name, d)
		}
	}

	res := Resolved{Core: strings.TrimSpace(core.String())}
	if detail.Len() > 0 {
		res.Detail = fmt.Sprintf("---\nname: %s\ndescription: Project coding standards resolved by Forge.\n---\n\n# Coding Standards\n\n%s",
			SkillName, strings.TrimSpace(detail.String()))
	}
	return res
}

// Querier is the subset of generated db methods Resolve needs.
// *db.Queries (sqlc-generated) satisfies this interface.
type Querier interface {
	ListWorkspaceStandards(ctx context.Context, workspaceID pgtype.UUID) ([]db.ForgeStandard, error)
	ListProjectStandards(ctx context.Context, projectID pgtype.UUID) ([]db.ForgeStandard, error)
	GetForgeProjectProfile(ctx context.Context, projectID pgtype.UUID) (db.ForgeProjectProfile, error)
}

// Resolve loads standards for (workspaceID, projectID) and returns the two-layer
// payload. projectID may be the zero UUID (no project) — then only workspace
// standards apply and no profile filter is performed.
func Resolve(ctx context.Context, q Querier, workspaceID, projectID pgtype.UUID) (Resolved, error) {
	ws, err := q.ListWorkspaceStandards(ctx, workspaceID)
	if err != nil {
		return Resolved{}, fmt.Errorf("list workspace standards: %w", err)
	}
	var proj []db.ForgeStandard
	var tags []string
	if projectID.Valid {
		proj, err = q.ListProjectStandards(ctx, projectID)
		if err != nil {
			return Resolved{}, fmt.Errorf("list project standards: %w", err)
		}
		if prof, perr := q.GetForgeProjectProfile(ctx, projectID); perr == nil {
			tags = prof.Tags
		} // no profile row → tags nil → only empty-tag standards apply
	}
	return resolveStandards(ws, proj, tags), nil
}
