package handler

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

// Forge F2: verification checks CRUD.

type ForgeCheckRequest struct {
	ProjectID string `json:"project_id,omitempty"` // empty = workspace-level
	Name      string `json:"name"`
	Command   string `json:"command"`
	Enabled   *bool  `json:"enabled,omitempty"`
}

type ForgeCheckResponse struct {
	ID          string `json:"id"`
	WorkspaceID string `json:"workspace_id"`
	ProjectID   string `json:"project_id,omitempty"`
	Name        string `json:"name"`
	Command     string `json:"command"`
	Enabled     bool   `json:"enabled"`
}

func forgeCheckToResponse(c db.ForgeCheck) ForgeCheckResponse {
	r := ForgeCheckResponse{
		ID: uuidToString(c.ID), WorkspaceID: uuidToString(c.WorkspaceID),
		Name: c.Name, Command: c.Command, Enabled: c.Enabled,
	}
	if c.ProjectID.Valid {
		r.ProjectID = uuidToString(c.ProjectID)
	}
	return r
}

func (h *Handler) ListForgeChecks(w http.ResponseWriter, r *http.Request) {
	wsID := h.resolveWorkspaceID(r)
	if wsID == "" {
		writeError(w, http.StatusBadRequest, "workspace_id is required")
		return
	}
	rows, err := h.Queries.ListChecksByWorkspace(r.Context(), parseUUID(wsID))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list checks")
		return
	}
	out := make([]ForgeCheckResponse, len(rows))
	for i, c := range rows {
		out[i] = forgeCheckToResponse(c)
	}
	writeJSON(w, http.StatusOK, out)
}

func (h *Handler) CreateForgeCheck(w http.ResponseWriter, r *http.Request) {
	wsID := h.resolveWorkspaceID(r)
	if wsID == "" {
		writeError(w, http.StatusBadRequest, "workspace_id is required")
		return
	}
	userID, ok := requireUserID(w, r)
	if !ok {
		return
	}
	var req ForgeCheckRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Name == "" || req.Command == "" {
		writeError(w, http.StatusBadRequest, "name and command are required")
		return
	}
	// parseAutopilotProjectID validates the project belongs to this workspace
	// (empty project_id → workspace-level, returns the zero UUID).
	projID, ok := h.parseAutopilotProjectID(w, r, &req.ProjectID, parseUUID(wsID))
	if !ok {
		return
	}
	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}
	c, err := h.Queries.CreateForgeCheck(r.Context(), db.CreateForgeCheckParams{
		WorkspaceID: parseUUID(wsID), ProjectID: projID, Name: req.Name,
		Command: req.Command, Enabled: enabled, CreatedBy: parseUUID(userID),
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create check")
		return
	}
	writeJSON(w, http.StatusCreated, forgeCheckToResponse(c))
}

func (h *Handler) loadForgeCheck(w http.ResponseWriter, r *http.Request) (db.ForgeCheck, bool) {
	wsID := h.resolveWorkspaceID(r)
	if wsID == "" {
		writeError(w, http.StatusBadRequest, "workspace_id is required")
		return db.ForgeCheck{}, false
	}
	id, ok := parseUUIDOrBadRequest(w, chi.URLParam(r, "id"), "check id")
	if !ok {
		return db.ForgeCheck{}, false
	}
	c, err := h.Queries.GetForgeCheck(r.Context(), db.GetForgeCheckParams{ID: id, WorkspaceID: parseUUID(wsID)})
	if err != nil {
		writeError(w, http.StatusNotFound, "check not found")
		return db.ForgeCheck{}, false
	}
	return c, true
}

func (h *Handler) GetForgeCheck(w http.ResponseWriter, r *http.Request) {
	c, ok := h.loadForgeCheck(w, r)
	if !ok {
		return
	}
	writeJSON(w, http.StatusOK, forgeCheckToResponse(c))
}

func (h *Handler) UpdateForgeCheck(w http.ResponseWriter, r *http.Request) {
	existing, ok := h.loadForgeCheck(w, r)
	if !ok {
		return
	}
	var req ForgeCheckRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	params := db.UpdateForgeCheckParams{ID: existing.ID, WorkspaceID: existing.WorkspaceID}
	if req.Name != "" {
		params.Name = pgtype.Text{String: req.Name, Valid: true}
	}
	if req.Command != "" {
		params.Command = pgtype.Text{String: req.Command, Valid: true}
	}
	if req.Enabled != nil {
		params.Enabled = pgtype.Bool{Bool: *req.Enabled, Valid: true}
	}
	c, err := h.Queries.UpdateForgeCheck(r.Context(), params)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update check")
		return
	}
	writeJSON(w, http.StatusOK, forgeCheckToResponse(c))
}

func (h *Handler) DeleteForgeCheck(w http.ResponseWriter, r *http.Request) {
	c, ok := h.loadForgeCheck(w, r)
	if !ok {
		return
	}
	if err := h.Queries.DeleteForgeCheck(r.Context(), db.DeleteForgeCheckParams{ID: c.ID, WorkspaceID: c.WorkspaceID}); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to delete check")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
