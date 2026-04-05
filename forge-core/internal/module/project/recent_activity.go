package project

import (
	"context"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/shulex/forge/forge-core/internal/pkg/response"
)

// ActivityItem represents a single recent activity entry.
type ActivityItem struct {
	Type        string `json:"type"` // task_created, task_completed, task_failed, pr_created, version_released
	ProjectID   int64  `json:"projectId"`
	ProjectName string `json:"projectName"`
	Title       string `json:"title"`
	Status      string `json:"status,omitempty"`
	TaskID      int64  `json:"taskId,omitempty"`
	Timestamp   string `json:"timestamp"`
}

// GetRecentActivity returns recent activity across all projects for a tenant.
func (r *Repository) GetRecentActivity(ctx context.Context, tenantID int64, limit int) ([]ActivityItem, error) {
	if limit <= 0 || limit > 50 {
		limit = 15
	}

	rows, err := r.db.Query(ctx,
		`SELECT t.id, t.project_id, p.name, t.title, t.status,
		        TO_CHAR(t.updated_at, 'YYYY-MM-DD"T"HH24:MI:SS"Z"')
		 FROM engine.tasks t
		 JOIN engine.projects p ON p.id = t.project_id
		 WHERE p.tenant_id = $1 AND p.status != 'ARCHIVED'
		 ORDER BY t.updated_at DESC
		 LIMIT $2`,
		tenantID, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []ActivityItem
	for rows.Next() {
		var a ActivityItem
		if err := rows.Scan(&a.TaskID, &a.ProjectID, &a.ProjectName, &a.Title, &a.Status, &a.Timestamp); err != nil {
			continue
		}
		switch a.Status {
		case "COMPLETED":
			a.Type = "task_completed"
		case "FAILED":
			a.Type = "task_failed"
		case "RUNNING":
			a.Type = "task_running"
		default:
			a.Type = "task_created"
		}
		items = append(items, a)
	}
	if items == nil {
		items = []ActivityItem{}
	}
	return items, nil
}

func (s *Service) GetRecentActivity(ctx context.Context, tenantID int64, limit int) ([]ActivityItem, error) {
	return s.repo.GetRecentActivity(ctx, tenantID, limit)
}

// GET /api/activity
func (h *Handler) GetRecentActivity(c *gin.Context) {
	_, tenantID := userCtx(c)
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "15"))

	items, err := h.svc.GetRecentActivity(c.Request.Context(), tenantID, limit)
	if err != nil {
		response.Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	response.OK(c, gin.H{"activity": items})
}
