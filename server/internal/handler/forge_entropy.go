package handler

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/service"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

// Forge F4: entropy scan config. Forge owns the scan definition and manages a
// backing Autopilot (schedule trigger = cron, execution_mode = create_issue,
// assignee = scanner agent). The dispatch hook composes the scanner's brief.

type ForgeEntropyScanBody struct {
	ProjectID        string `json:"project_id,omitempty"`
	Name             string `json:"name"`
	ScannerAgentID   string `json:"scanner_agent_id"`
	CustomFocus      string `json:"custom_focus"`
	IncludeStandards bool   `json:"include_standards"`
	IncludeChecks    bool   `json:"include_checks"`
	CronExpression   string `json:"cron_expression"`
	Timezone         string `json:"timezone"`
	Enabled          bool   `json:"enabled"`
	AutoFix          bool   `json:"auto_fix"`
}

type ForgeEntropyScanResponse struct {
	ID               string `json:"id"`
	ProjectID        string `json:"project_id,omitempty"`
	Name             string `json:"name"`
	ScannerAgentID   string `json:"scanner_agent_id"`
	CustomFocus      string `json:"custom_focus"`
	IncludeStandards bool   `json:"include_standards"`
	IncludeChecks    bool   `json:"include_checks"`
	CronExpression   string `json:"cron_expression"`
	Timezone         string `json:"timezone"`
	Enabled          bool   `json:"enabled"`
	AutoFix          bool   `json:"auto_fix"`
	AutopilotID      string `json:"autopilot_id,omitempty"`
}

func entropyScanToResponse(s db.ForgeEntropyScan) ForgeEntropyScanResponse {
	out := ForgeEntropyScanResponse{
		ID: uuidToString(s.ID), Name: s.Name, ScannerAgentID: uuidToString(s.ScannerAgentID),
		CustomFocus: s.CustomFocus, IncludeStandards: s.IncludeStandards, IncludeChecks: s.IncludeChecks,
		CronExpression: s.CronExpression, Timezone: s.Timezone, Enabled: s.Enabled,
		AutoFix: s.AutoFix,
	}
	if s.ProjectID.Valid {
		out.ProjectID = uuidToString(s.ProjectID)
	}
	if s.AutopilotID.Valid {
		out.AutopilotID = uuidToString(s.AutopilotID)
	}
	return out
}

func (h *Handler) ListForgeEntropyScans(w http.ResponseWriter, r *http.Request) {
	wsID := h.resolveWorkspaceID(r)
	if wsID == "" {
		writeError(w, http.StatusBadRequest, "workspace_id is required")
		return
	}
	var projParam pgtype.UUID
	if pid := r.URL.Query().Get("project_id"); pid != "" {
		p, ok := parseUUIDOrBadRequest(w, pid, "project_id")
		if !ok {
			return
		}
		projParam = p
	}
	scans, err := h.Queries.ListEntropyScans(r.Context(), db.ListEntropyScansParams{
		WorkspaceID: parseUUID(wsID), ProjectID: projParam,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list entropy scans")
		return
	}
	out := make([]ForgeEntropyScanResponse, 0, len(scans))
	for _, s := range scans {
		out = append(out, entropyScanToResponse(s))
	}
	writeJSON(w, http.StatusOK, out)
}

func (h *Handler) CreateForgeEntropyScan(w http.ResponseWriter, r *http.Request) {
	wsID := h.resolveWorkspaceID(r)
	if wsID == "" {
		writeError(w, http.StatusBadRequest, "workspace_id is required")
		return
	}
	userID, ok := requireUserID(w, r)
	if !ok {
		return
	}
	var req ForgeEntropyScanBody
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Name == "" || req.CronExpression == "" {
		writeError(w, http.StatusBadRequest, "name and cron_expression are required")
		return
	}
	scannerID, ok := parseUUIDOrBadRequest(w, req.ScannerAgentID, "scanner_agent_id")
	if !ok {
		return
	}
	if !h.validateAutopilotAssignee(w, r, "agent", scannerID, parseUUID(wsID)) {
		return
	}
	projParam, ok := h.parseAutopilotProjectID(w, r, &req.ProjectID, parseUUID(wsID))
	if !ok {
		return
	}
	tz := req.Timezone
	if tz == "" {
		tz = "UTC"
	}
	if err := service.ValidateTimezone(tz); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	nextRun, err := computeNextRun(req.CronExpression, tz)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	// 1. backing autopilot first — a failure here leaves no orphan scan.
	status := "active"
	if !req.Enabled {
		status = "paused"
	}
	autopilot, err := h.Queries.CreateAutopilot(r.Context(), db.CreateAutopilotParams{
		WorkspaceID:   parseUUID(wsID),
		Title:         "Entropy scan: " + req.Name,
		AssigneeType:  "agent",
		AssigneeID:    scannerID,
		Status:        status,
		ExecutionMode: "create_issue",
		CreatedByType: "member",
		CreatedByID:   parseUUID(userID),
		ProjectID:     projParam,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create backing autopilot")
		return
	}
	if _, err := h.Queries.CreateAutopilotTrigger(r.Context(), db.CreateAutopilotTriggerParams{
		AutopilotID:    autopilot.ID,
		Kind:           "schedule",
		Enabled:        true,
		CronExpression: pgtype.Text{String: req.CronExpression, Valid: true},
		Timezone:       pgtype.Text{String: tz, Valid: true},
		NextRunAt:      pgtype.Timestamptz{Time: nextRun, Valid: true},
	}); err != nil {
		_ = h.Queries.DeleteAutopilot(r.Context(), autopilot.ID)
		writeError(w, http.StatusInternalServerError, "failed to create schedule trigger")
		return
	}

	// 2. scan row.
	scan, err := h.Queries.CreateEntropyScan(r.Context(), db.CreateEntropyScanParams{
		WorkspaceID:      parseUUID(wsID),
		ProjectID:        projParam,
		Name:             req.Name,
		ScannerAgentID:   scannerID,
		CustomFocus:      req.CustomFocus,
		IncludeStandards: req.IncludeStandards,
		IncludeChecks:    req.IncludeChecks,
		CronExpression:   req.CronExpression,
		Timezone:         tz,
		Enabled:          req.Enabled,
		AutoFix:          req.AutoFix,
		CreatedBy:        parseUUID(userID),
	})
	if err != nil {
		_ = h.Queries.DeleteAutopilot(r.Context(), autopilot.ID)
		writeError(w, http.StatusInternalServerError, "failed to create entropy scan")
		return
	}
	// 3. link scan -> autopilot.
	if err := h.Queries.SetEntropyScanAutopilot(r.Context(), db.SetEntropyScanAutopilotParams{
		ID: scan.ID, AutopilotID: autopilot.ID,
	}); err != nil {
		_ = h.Queries.DeleteAutopilot(r.Context(), autopilot.ID)
		_, _ = h.Queries.DeleteEntropyScan(r.Context(), db.DeleteEntropyScanParams{ID: scan.ID, WorkspaceID: parseUUID(wsID)})
		writeError(w, http.StatusInternalServerError, "failed to link autopilot")
		return
	}
	scan.AutopilotID = autopilot.ID
	writeJSON(w, http.StatusCreated, entropyScanToResponse(scan))
}

func (h *Handler) UpdateForgeEntropyScan(w http.ResponseWriter, r *http.Request) {
	wsID := h.resolveWorkspaceID(r)
	if wsID == "" {
		writeError(w, http.StatusBadRequest, "workspace_id is required")
		return
	}
	scanID, ok := parseUUIDOrBadRequest(w, chi.URLParam(r, "id"), "id")
	if !ok {
		return
	}
	var req ForgeEntropyScanBody
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Name == "" || req.CronExpression == "" {
		writeError(w, http.StatusBadRequest, "name and cron_expression are required")
		return
	}
	scannerID, ok := parseUUIDOrBadRequest(w, req.ScannerAgentID, "scanner_agent_id")
	if !ok {
		return
	}
	if !h.validateAutopilotAssignee(w, r, "agent", scannerID, parseUUID(wsID)) {
		return
	}
	tz := req.Timezone
	if tz == "" {
		tz = "UTC"
	}
	if err := service.ValidateTimezone(tz); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	nextRun, err := computeNextRun(req.CronExpression, tz)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	scan, err := h.Queries.UpdateEntropyScan(r.Context(), db.UpdateEntropyScanParams{
		ID: scanID, WorkspaceID: parseUUID(wsID),
		Name: req.Name, ScannerAgentID: scannerID, CustomFocus: req.CustomFocus,
		IncludeStandards: req.IncludeStandards, IncludeChecks: req.IncludeChecks,
		CronExpression: req.CronExpression, Timezone: tz, Enabled: req.Enabled,
		AutoFix: req.AutoFix,
	})
	if err != nil {
		writeError(w, http.StatusNotFound, "entropy scan not found")
		return
	}
	// Sync the backing autopilot best-effort — never fail the scan update on it.
	if scan.AutopilotID.Valid {
		h.syncEntropyAutopilot(r.Context(), scan, nextRun)
	}
	writeJSON(w, http.StatusOK, entropyScanToResponse(scan))
}

func (h *Handler) DeleteForgeEntropyScan(w http.ResponseWriter, r *http.Request) {
	wsID := h.resolveWorkspaceID(r)
	if wsID == "" {
		writeError(w, http.StatusBadRequest, "workspace_id is required")
		return
	}
	scanID, ok := parseUUIDOrBadRequest(w, chi.URLParam(r, "id"), "id")
	if !ok {
		return
	}
	autopilotID, err := h.Queries.DeleteEntropyScan(r.Context(), db.DeleteEntropyScanParams{
		ID: scanID, WorkspaceID: parseUUID(wsID),
	})
	if err != nil {
		writeError(w, http.StatusNotFound, "entropy scan not found")
		return
	}
	if autopilotID.Valid {
		if err := h.Queries.DeleteAutopilot(r.Context(), autopilotID); err != nil {
			slog.Warn("forge entropy: delete backing autopilot failed", "error", err)
		}
	}
	w.WriteHeader(http.StatusNoContent)
}

// syncEntropyAutopilot mirrors scan fields onto the backing autopilot + its
// schedule trigger. Best-effort; logs and continues on any error.
func (h *Handler) syncEntropyAutopilot(ctx context.Context, scan db.ForgeEntropyScan, nextRun time.Time) {
	status := "active"
	if !scan.Enabled {
		status = "paused"
	}
	if _, err := h.Queries.UpdateAutopilot(ctx, db.UpdateAutopilotParams{
		ID:           scan.AutopilotID,
		Title:        pgtype.Text{String: "Entropy scan: " + scan.Name, Valid: true},
		AssigneeType: pgtype.Text{String: "agent", Valid: true},
		AssigneeID:   scan.ScannerAgentID,
		Status:       pgtype.Text{String: status, Valid: true},
		ProjectID:    scan.ProjectID,
	}); err != nil {
		slog.Warn("forge entropy: sync autopilot failed", "error", err)
	}
	triggers, err := h.Queries.ListAutopilotTriggers(ctx, scan.AutopilotID)
	if err != nil {
		slog.Warn("forge entropy: list triggers failed", "error", err)
		return
	}
	for _, t := range triggers {
		if t.Kind != "schedule" {
			continue
		}
		if _, err := h.Queries.UpdateAutopilotTrigger(ctx, db.UpdateAutopilotTriggerParams{
			ID:             t.ID,
			Enabled:        pgtype.Bool{Bool: true, Valid: true},
			CronExpression: pgtype.Text{String: scan.CronExpression, Valid: true},
			Timezone:       pgtype.Text{String: scan.Timezone, Valid: true},
			NextRunAt:      pgtype.Timestamptz{Time: nextRun, Valid: true},
		}); err != nil {
			slog.Warn("forge entropy: sync trigger failed", "error", err)
		}
	}
}
