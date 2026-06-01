package handler

import (
	"encoding/json"
	"net/http"

	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

// Forge F3: review config (which agent reviews) per workspace/project.

type ForgeReviewConfigBody struct {
	ProjectID       string `json:"project_id,omitempty"` // empty = workspace-level
	ReviewerAgentID string `json:"reviewer_agent_id"`
	Enabled         bool   `json:"enabled"`
}

func (h *Handler) GetForgeReviewConfig(w http.ResponseWriter, r *http.Request) {
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
	cfg, err := h.Queries.GetReviewConfigByScope(r.Context(), db.GetReviewConfigByScopeParams{
		WorkspaceID: parseUUID(wsID),
		ProjectID:   projParam,
	})
	if err != nil {
		writeJSON(w, http.StatusOK, ForgeReviewConfigBody{}) // none configured
		return
	}
	out := ForgeReviewConfigBody{ReviewerAgentID: uuidToString(cfg.ReviewerAgentID), Enabled: cfg.Enabled}
	if cfg.ProjectID.Valid {
		out.ProjectID = uuidToString(cfg.ProjectID)
	}
	writeJSON(w, http.StatusOK, out)
}

func (h *Handler) PutForgeReviewConfig(w http.ResponseWriter, r *http.Request) {
	wsID := h.resolveWorkspaceID(r)
	if wsID == "" {
		writeError(w, http.StatusBadRequest, "workspace_id is required")
		return
	}
	userID, ok := requireUserID(w, r)
	if !ok {
		return
	}
	var req ForgeReviewConfigBody
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	reviewerID, ok := parseUUIDOrBadRequest(w, req.ReviewerAgentID, "reviewer_agent_id")
	if !ok {
		return
	}
	if req.ProjectID != "" {
		projID, ok := parseUUIDOrBadRequest(w, req.ProjectID, "project_id")
		if !ok {
			return
		}
		cfg, err := h.Queries.UpsertProjectReviewConfig(r.Context(), db.UpsertProjectReviewConfigParams{
			WorkspaceID: parseUUID(wsID), ProjectID: projID, ReviewerAgentID: reviewerID,
			Enabled: req.Enabled, CreatedBy: parseUUID(userID),
		})
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to save review config")
			return
		}
		writeJSON(w, http.StatusOK, ForgeReviewConfigBody{ProjectID: uuidToString(cfg.ProjectID), ReviewerAgentID: uuidToString(cfg.ReviewerAgentID), Enabled: cfg.Enabled})
		return
	}
	cfg, err := h.Queries.UpsertWorkspaceReviewConfig(r.Context(), db.UpsertWorkspaceReviewConfigParams{
		WorkspaceID: parseUUID(wsID), ReviewerAgentID: reviewerID, Enabled: req.Enabled, CreatedBy: parseUUID(userID),
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to save review config")
		return
	}
	writeJSON(w, http.StatusOK, ForgeReviewConfigBody{ReviewerAgentID: uuidToString(cfg.ReviewerAgentID), Enabled: cfg.Enabled})
}
