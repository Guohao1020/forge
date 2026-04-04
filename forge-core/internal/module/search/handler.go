package search

import (
	"context"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shulex/forge/forge-core/internal/pkg/response"
)

// SearchResult represents a single search hit.
type SearchResult struct {
	Type        string `json:"type"` // project, task
	ID          int64  `json:"id"`
	ProjectID   int64  `json:"projectId,omitempty"`
	Title       string `json:"title"`
	Description string `json:"description,omitempty"`
	Status      string `json:"status,omitempty"`
	URL         string `json:"url"` // frontend route
}

// SearchResponse wraps the complete search response.
type SearchResponse struct {
	Query   string         `json:"query"`
	Results []SearchResult `json:"results"`
	Total   int            `json:"total"`
}

type Handler struct {
	db *pgxpool.Pool
}

func NewHandler(db *pgxpool.Pool) *Handler {
	return &Handler{db: db}
}

// GET /api/search?q=keyword
func (h *Handler) Search(c *gin.Context) {
	query := strings.TrimSpace(c.Query("q"))
	if query == "" || len(query) < 2 {
		response.OK(c, SearchResponse{Query: query, Results: []SearchResult{}, Total: 0})
		return
	}

	tid, _ := c.Get("tenant_id")
	tenantID, _ := tid.(int64)

	results, err := h.search(c.Request.Context(), tenantID, query)
	if err != nil {
		slog.Error("search failed", "error", err, "query", query)
		response.Fail(c, http.StatusInternalServerError, "search failed")
		return
	}

	response.OK(c, SearchResponse{
		Query:   query,
		Results: results,
		Total:   len(results),
	})
}

func (h *Handler) search(ctx context.Context, tenantID int64, query string) ([]SearchResult, error) {
	var results []SearchResult
	pattern := "%" + query + "%"

	// Search projects
	rows, err := h.db.Query(ctx,
		`SELECT id, name, COALESCE(description, ''), status
		 FROM engine.projects
		 WHERE tenant_id = $1 AND status != 'ARCHIVED'
		   AND (name ILIKE $2 OR description ILIKE $2)
		 ORDER BY updated_at DESC
		 LIMIT 10`,
		tenantID, pattern,
	)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var r SearchResult
			if err := rows.Scan(&r.ID, &r.Title, &r.Description, &r.Status); err != nil {
				continue
			}
			r.Type = "project"
			r.URL = "/projects/" + strconv.FormatInt(r.ID, 10)
			results = append(results, r)
		}
	}

	// Search tasks
	taskRows, err := h.db.Query(ctx,
		`SELECT t.id, t.project_id, t.title, COALESCE(t.description, ''), t.status
		 FROM engine.tasks t
		 JOIN engine.projects p ON p.id = t.project_id
		 WHERE p.tenant_id = $1 AND p.status != 'ARCHIVED'
		   AND (t.title ILIKE $2 OR t.description ILIKE $2)
		 ORDER BY t.updated_at DESC
		 LIMIT 10`,
		tenantID, pattern,
	)
	if err == nil {
		defer taskRows.Close()
		for taskRows.Next() {
			var r SearchResult
			if err := taskRows.Scan(&r.ID, &r.ProjectID, &r.Title, &r.Description, &r.Status); err != nil {
				continue
			}
			r.Type = "task"
			r.URL = "/projects/" + strconv.FormatInt(r.ProjectID, 10) + "/tasks/" + strconv.FormatInt(r.ID, 10)
			results = append(results, r)
		}
	}

	if results == nil {
		results = []SearchResult{}
	}
	return results, nil
}
