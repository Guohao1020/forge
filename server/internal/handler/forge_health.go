package handler

import (
	"net/http"

	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

// Forge F5: Harness health snapshot. Compute-on-read aggregation over existing
// Forge + Multica tables (no new tracking). Mirrors dashboard.go conventions.

type ForgeHealthCategoryCount struct {
	Category string `json:"category"`
	Count    int32  `json:"count"`
}

type ForgeHealthGate struct {
	Passed int32 `json:"passed"`
	Failed int32 `json:"failed"`
}

type ForgeHealthReview struct {
	Total            int32 `json:"total"`
	Completed        int32 `json:"completed"`
	AvgTurnaroundSec int64 `json:"avg_turnaround_sec"`
}

type ForgeHealthFixPRs struct {
	Opened  int32 `json:"opened"`
	Merged  int32 `json:"merged"`
	Matched int32 `json:"matched"`
}

type ForgeHealthResponse struct {
	Standards      []ForgeHealthCategoryCount `json:"standards"`
	StandardsTotal int32                      `json:"standards_total"`
	Checks         int32                      `json:"checks"`
	ReviewConfigs  int32                      `json:"review_configs"`
	Scans          int32                      `json:"scans"`
	Gate           ForgeHealthGate            `json:"gate"`
	Review         ForgeHealthReview          `json:"review"`
	OpenFindings   int32                      `json:"open_findings"`
	ScanRuns       int32                      `json:"scan_runs"`
	FixPRs         ForgeHealthFixPRs          `json:"fix_prs"`
}

func (h *Handler) GetForgeHealth(w http.ResponseWriter, r *http.Request) {
	workspaceID := h.resolveWorkspaceID(r)
	if _, ok := h.workspaceMember(w, r, workspaceID); !ok {
		return
	}
	projectID, ok := parseProjectIDParam(w, r)
	if !ok {
		return
	}
	tz := h.resolveViewingTZ(r)
	since := parseSinceParamInTZ(r, 30, tz)
	ws := parseUUID(workspaceID)
	ctx := r.Context()

	out := ForgeHealthResponse{Standards: []ForgeHealthCategoryCount{}}

	if rows, err := h.Queries.CountForgeStandardsByCategory(ctx, db.CountForgeStandardsByCategoryParams{WorkspaceID: ws, ProjectID: projectID}); err == nil {
		for _, row := range rows {
			out.Standards = append(out.Standards, ForgeHealthCategoryCount{Category: row.Category, Count: row.Count})
			out.StandardsTotal += row.Count
		}
	}
	out.Checks, _ = h.Queries.CountForgeChecks(ctx, db.CountForgeChecksParams{WorkspaceID: ws, ProjectID: projectID})
	out.ReviewConfigs, _ = h.Queries.CountForgeReviewConfigs(ctx, db.CountForgeReviewConfigsParams{WorkspaceID: ws, ProjectID: projectID})
	out.Scans, _ = h.Queries.CountForgeEntropyScans(ctx, db.CountForgeEntropyScansParams{WorkspaceID: ws, ProjectID: projectID})
	out.OpenFindings, _ = h.Queries.CountOpenEntropyFindings(ctx, db.CountOpenEntropyFindingsParams{WorkspaceID: ws, ProjectID: projectID})

	if g, err := h.Queries.GetForgeGateOutcomes(ctx, db.GetForgeGateOutcomesParams{WorkspaceID: ws, ProjectID: projectID, Since: since}); err == nil {
		out.Gate = ForgeHealthGate{Passed: g.Passed, Failed: g.Failed}
	}
	if rv, err := h.Queries.GetForgeReviewOutcomes(ctx, db.GetForgeReviewOutcomesParams{WorkspaceID: ws, ProjectID: projectID, Since: since}); err == nil {
		out.Review = ForgeHealthReview{Total: rv.Total, Completed: rv.Completed, AvgTurnaroundSec: rv.AvgTurnaroundSec}
	}
	out.ScanRuns, _ = h.Queries.CountForgeEntropyScanRuns(ctx, db.CountForgeEntropyScanRunsParams{WorkspaceID: ws, ProjectID: projectID, Since: since})
	if fp, err := h.Queries.GetForgeFixPROutcomes(ctx, db.GetForgeFixPROutcomesParams{WorkspaceID: ws, ProjectID: projectID, Since: since}); err == nil {
		out.FixPRs = ForgeHealthFixPRs{Opened: fp.Opened, Merged: fp.Merged, Matched: fp.Matched}
	}

	writeJSON(w, http.StatusOK, out)
}
