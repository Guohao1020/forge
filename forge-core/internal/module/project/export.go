package project

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/shulex/forge/forge-core/internal/pkg/response"
)

// ProjectExport contains all project data for backup/export.
type ProjectExport struct {
	ExportedAt string          `json:"exportedAt"`
	Version    string          `json:"version"`
	Project    json.RawMessage `json:"project"`
	Tasks      json.RawMessage `json:"tasks"`
	Versions   json.RawMessage `json:"versions"`
	Settings   json.RawMessage `json:"settings,omitempty"`
}

// ExportProject generates a full JSON export of the project.
func (r *Repository) ExportProject(ctx context.Context, projectID, tenantID int64) (*ProjectExport, error) {
	export := &ProjectExport{
		ExportedAt: time.Now().UTC().Format(time.RFC3339),
		Version:    "1.0",
	}

	// Export project details
	row := r.db.QueryRow(ctx,
		`SELECT row_to_json(p) FROM (
			SELECT id, name, description, status, default_branch,
			       code_platform, code_repo_url, ai_model, risk_threshold,
			       auto_merge, created_at, updated_at
			FROM engine.projects WHERE id = $1 AND tenant_id = $2
		) p`,
		projectID, tenantID,
	)
	var projectJSON []byte
	if err := row.Scan(&projectJSON); err != nil {
		return nil, err
	}
	export.Project = projectJSON

	// Export tasks
	rows, err := r.db.Query(ctx,
		`SELECT json_agg(t) FROM (
			SELECT id, title, description, status, priority,
			       version_id, conflict_status, created_at, updated_at
			FROM engine.tasks WHERE project_id = $1
			ORDER BY created_at DESC
		) t`,
		projectID,
	)
	if err == nil {
		defer rows.Close()
		if rows.Next() {
			var tasksJSON []byte
			if err := rows.Scan(&tasksJSON); err == nil && tasksJSON != nil {
				export.Tasks = tasksJSON
			} else {
				export.Tasks = json.RawMessage("[]")
			}
		}
	}

	// Export versions
	vRows, err := r.db.Query(ctx,
		`SELECT json_agg(v) FROM (
			SELECT id, name, status, description, git_tag,
			       created_at, updated_at
			FROM engine.project_versions WHERE project_id = $1
			ORDER BY created_at DESC
		) v`,
		projectID,
	)
	if err == nil {
		defer vRows.Close()
		if vRows.Next() {
			var versionsJSON []byte
			if err := vRows.Scan(&versionsJSON); err == nil && versionsJSON != nil {
				export.Versions = versionsJSON
			} else {
				export.Versions = json.RawMessage("[]")
			}
		}
	}

	return export, nil
}

func (s *Service) ExportProject(ctx context.Context, projectID, tenantID int64) (*ProjectExport, error) {
	return s.repo.ExportProject(ctx, projectID, tenantID)
}

// GET /api/projects/:id/export
func (h *Handler) ExportProject(c *gin.Context) {
	_, tenantID := userCtx(c)
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.Fail(c, http.StatusBadRequest, "invalid id")
		return
	}

	export, err := h.svc.ExportProject(c.Request.Context(), id, tenantID)
	if err != nil {
		response.Fail(c, http.StatusInternalServerError, err.Error())
		return
	}

	// Set download headers
	c.Header("Content-Disposition", "attachment; filename=project-"+c.Param("id")+"-export.json")
	c.JSON(http.StatusOK, export)
}
