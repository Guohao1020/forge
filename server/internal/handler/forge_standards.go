package handler

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

// Forge F1: spec-center Standards CRUD + per-project profile.

type ForgeStandardRequest struct {
	ProjectID     string   `json:"project_id,omitempty"` // empty = workspace-level
	Name          string   `json:"name"`
	Category      string   `json:"category"`
	ProfileTags   []string `json:"profile_tags"`
	CoreContent   string   `json:"core_content"`
	DetailContent string   `json:"detail_content"`
	Enabled       *bool    `json:"enabled,omitempty"`
}

type ForgeStandardResponse struct {
	ID            string   `json:"id"`
	WorkspaceID   string   `json:"workspace_id"`
	ProjectID     string   `json:"project_id,omitempty"`
	Name          string   `json:"name"`
	Category      string   `json:"category"`
	ProfileTags   []string `json:"profile_tags"`
	CoreContent   string   `json:"core_content"`
	DetailContent string   `json:"detail_content"`
	Enabled       bool     `json:"enabled"`
}

func forgeStandardToResponse(s db.ForgeStandard) ForgeStandardResponse {
	r := ForgeStandardResponse{
		ID: uuidToString(s.ID), WorkspaceID: uuidToString(s.WorkspaceID),
		Name: s.Name, Category: s.Category, ProfileTags: s.ProfileTags,
		CoreContent: s.CoreContent, DetailContent: s.DetailContent, Enabled: s.Enabled,
	}
	if s.ProjectID.Valid {
		r.ProjectID = uuidToString(s.ProjectID)
	}
	if r.ProfileTags == nil {
		r.ProfileTags = []string{}
	}
	return r
}

func (h *Handler) ListForgeStandards(w http.ResponseWriter, r *http.Request) {
	wsID := h.resolveWorkspaceID(r)
	if wsID == "" {
		writeError(w, http.StatusBadRequest, "workspace_id is required")
		return
	}
	rows, err := h.Queries.ListStandardsByWorkspace(r.Context(), parseUUID(wsID))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list standards")
		return
	}
	out := make([]ForgeStandardResponse, len(rows))
	for i, s := range rows {
		out[i] = forgeStandardToResponse(s)
	}
	writeJSON(w, http.StatusOK, out)
}

func (h *Handler) CreateForgeStandard(w http.ResponseWriter, r *http.Request) {
	wsID := h.resolveWorkspaceID(r)
	if wsID == "" {
		writeError(w, http.StatusBadRequest, "workspace_id is required")
		return
	}
	userID, ok := requireUserID(w, r)
	if !ok {
		return
	}
	var req ForgeStandardRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Name == "" || req.Category == "" {
		writeError(w, http.StatusBadRequest, "name and category are required")
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
	if req.ProfileTags == nil {
		req.ProfileTags = []string{}
	}
	s, err := h.Queries.CreateForgeStandard(r.Context(), db.CreateForgeStandardParams{
		WorkspaceID: parseUUID(wsID), ProjectID: projID, Name: req.Name, Category: req.Category,
		ProfileTags: req.ProfileTags, CoreContent: req.CoreContent, DetailContent: req.DetailContent,
		Enabled: enabled, CreatedBy: parseUUID(userID),
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create standard")
		return
	}
	writeJSON(w, http.StatusCreated, forgeStandardToResponse(s))
}

func (h *Handler) loadForgeStandard(w http.ResponseWriter, r *http.Request) (db.ForgeStandard, bool) {
	wsID := h.resolveWorkspaceID(r)
	if wsID == "" {
		writeError(w, http.StatusBadRequest, "workspace_id is required")
		return db.ForgeStandard{}, false
	}
	id, ok := parseUUIDOrBadRequest(w, chi.URLParam(r, "id"), "standard id")
	if !ok {
		return db.ForgeStandard{}, false
	}
	s, err := h.Queries.GetForgeStandard(r.Context(), db.GetForgeStandardParams{ID: id, WorkspaceID: parseUUID(wsID)})
	if err != nil {
		writeError(w, http.StatusNotFound, "standard not found")
		return db.ForgeStandard{}, false
	}
	return s, true
}

func (h *Handler) GetForgeStandard(w http.ResponseWriter, r *http.Request) {
	s, ok := h.loadForgeStandard(w, r)
	if !ok {
		return
	}
	writeJSON(w, http.StatusOK, forgeStandardToResponse(s))
}

func (h *Handler) UpdateForgeStandard(w http.ResponseWriter, r *http.Request) {
	existing, ok := h.loadForgeStandard(w, r)
	if !ok {
		return
	}
	var req ForgeStandardRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	params := db.UpdateForgeStandardParams{ID: existing.ID, WorkspaceID: existing.WorkspaceID}
	if req.Name != "" {
		params.Name = pgtype.Text{String: req.Name, Valid: true}
	}
	if req.Category != "" {
		params.Category = pgtype.Text{String: req.Category, Valid: true}
	}
	if req.ProfileTags != nil {
		params.ProfileTags = req.ProfileTags
	}
	params.CoreContent = pgtype.Text{String: req.CoreContent, Valid: true}
	params.DetailContent = pgtype.Text{String: req.DetailContent, Valid: true}
	if req.Enabled != nil {
		params.Enabled = pgtype.Bool{Bool: *req.Enabled, Valid: true}
	}
	s, err := h.Queries.UpdateForgeStandard(r.Context(), params)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update standard")
		return
	}
	writeJSON(w, http.StatusOK, forgeStandardToResponse(s))
}

func (h *Handler) DeleteForgeStandard(w http.ResponseWriter, r *http.Request) {
	s, ok := h.loadForgeStandard(w, r)
	if !ok {
		return
	}
	if err := h.Queries.DeleteForgeStandard(r.Context(), db.DeleteForgeStandardParams{ID: s.ID, WorkspaceID: s.WorkspaceID}); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to delete standard")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ---- project profile ----

type ForgeProfileBody struct {
	Tags []string `json:"tags"`
}

func (h *Handler) GetForgeProjectProfile(w http.ResponseWriter, r *http.Request) {
	wsID := h.resolveWorkspaceID(r)
	if wsID == "" {
		writeError(w, http.StatusBadRequest, "workspace_id is required")
		return
	}
	projID, ok := parseUUIDOrBadRequest(w, chi.URLParam(r, "id"), "project id")
	if !ok {
		return
	}
	// Forge security: the project must belong to this workspace — otherwise a
	// caller could read another workspace's project profile.
	if _, err := h.Queries.GetProjectInWorkspace(r.Context(), db.GetProjectInWorkspaceParams{ID: projID, WorkspaceID: parseUUID(wsID)}); err != nil {
		writeError(w, http.StatusNotFound, "project not found")
		return
	}
	prof, err := h.Queries.GetForgeProjectProfile(r.Context(), projID)
	if err != nil {
		writeJSON(w, http.StatusOK, ForgeProfileBody{Tags: []string{}}) // none yet
		return
	}
	writeJSON(w, http.StatusOK, ForgeProfileBody{Tags: prof.Tags})
}

func (h *Handler) PutForgeProjectProfile(w http.ResponseWriter, r *http.Request) {
	wsID := h.resolveWorkspaceID(r)
	if wsID == "" {
		writeError(w, http.StatusBadRequest, "workspace_id is required")
		return
	}
	projID, ok := parseUUIDOrBadRequest(w, chi.URLParam(r, "id"), "project id")
	if !ok {
		return
	}
	// Forge security: the project must belong to this workspace — otherwise a
	// caller could write another workspace's project profile.
	if _, err := h.Queries.GetProjectInWorkspace(r.Context(), db.GetProjectInWorkspaceParams{ID: projID, WorkspaceID: parseUUID(wsID)}); err != nil {
		writeError(w, http.StatusNotFound, "project not found")
		return
	}
	var req ForgeProfileBody
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Tags == nil {
		req.Tags = []string{}
	}
	prof, err := h.Queries.UpsertForgeProjectProfile(r.Context(), db.UpsertForgeProjectProfileParams{ProjectID: projID, Tags: req.Tags})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to save profile")
		return
	}
	writeJSON(w, http.StatusOK, ForgeProfileBody{Tags: prof.Tags})
}
