package webhook

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shulex/forge/forge-core/internal/pkg/response"
)

// Webhook represents a registered webhook endpoint.
type Webhook struct {
	ID        int64  `json:"id"`
	ProjectID int64  `json:"projectId"`
	TenantID  int64  `json:"tenantId"`
	URL       string `json:"url"`
	Secret    string `json:"secret,omitempty"` // HMAC secret for signing
	Events    string `json:"events"`           // comma-separated: task.completed,task.failed,pr.created
	Active    bool   `json:"active"`
	CreatedAt string `json:"createdAt,omitempty"`
}

// WebhookPayload is the JSON body sent to webhook URLs.
type WebhookPayload struct {
	Event     string      `json:"event"`
	Timestamp string      `json:"timestamp"`
	ProjectID int64       `json:"projectId"`
	Data      interface{} `json:"data"`
}

// CreateWebhookRequest is the API request for registering a webhook.
type CreateWebhookRequest struct {
	URL    string `json:"url" binding:"required"`
	Secret string `json:"secret"`
	Events string `json:"events" binding:"required"` // e.g., "task.completed,task.failed"
}

type Service struct {
	db     *pgxpool.Pool
	client *http.Client
}

func NewService(db *pgxpool.Pool) *Service {
	return &Service{
		db: db,
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// List returns all webhooks for a project.
func (s *Service) List(ctx context.Context, projectID int64) ([]Webhook, error) {
	rows, err := s.db.Query(ctx,
		`SELECT id, project_id, tenant_id, url, COALESCE(secret, ''), events, active,
		        TO_CHAR(created_at, 'YYYY-MM-DD"T"HH24:MI:SS"Z"')
		 FROM engine.webhooks
		 WHERE project_id = $1
		 ORDER BY created_at DESC`,
		projectID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var webhooks []Webhook
	for rows.Next() {
		var w Webhook
		if err := rows.Scan(&w.ID, &w.ProjectID, &w.TenantID, &w.URL, &w.Secret, &w.Events, &w.Active, &w.CreatedAt); err != nil {
			continue
		}
		w.Secret = "" // Never return secret in list
		webhooks = append(webhooks, w)
	}
	if webhooks == nil {
		webhooks = []Webhook{}
	}
	return webhooks, nil
}

// Create registers a new webhook.
func (s *Service) Create(ctx context.Context, projectID, tenantID int64, req *CreateWebhookRequest) (int64, error) {
	var id int64
	err := s.db.QueryRow(ctx,
		`INSERT INTO engine.webhooks (project_id, tenant_id, url, secret, events, active)
		 VALUES ($1, $2, $3, $4, $5, true)
		 RETURNING id`,
		projectID, tenantID, req.URL, req.Secret, req.Events,
	).Scan(&id)
	return id, err
}

// Delete removes a webhook.
func (s *Service) Delete(ctx context.Context, webhookID int64) error {
	_, err := s.db.Exec(ctx, `DELETE FROM engine.webhooks WHERE id = $1`, webhookID)
	return err
}

// Fire sends a webhook event to all matching endpoints for a project.
// Runs asynchronously — errors are logged, not returned.
func (s *Service) Fire(ctx context.Context, projectID int64, event string, data interface{}) {
	rows, err := s.db.Query(ctx,
		`SELECT id, url, COALESCE(secret, ''), events
		 FROM engine.webhooks
		 WHERE project_id = $1 AND active = true`,
		projectID,
	)
	if err != nil {
		slog.Warn("webhook: failed to query webhooks", "error", err)
		return
	}
	defer rows.Close()

	payload := WebhookPayload{
		Event:     event,
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		ProjectID: projectID,
		Data:      data,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return
	}

	for rows.Next() {
		var id int64
		var url, secret, events string
		if err := rows.Scan(&id, &url, &secret, &events); err != nil {
			continue
		}

		// Check if this webhook subscribes to this event
		if !matchesEvent(events, event) {
			continue
		}

		go s.deliver(url, secret, body, id, event)
	}
}

func (s *Service) deliver(url, secret string, body []byte, webhookID int64, event string) {
	req, err := http.NewRequest("POST", url, bytes.NewReader(body))
	if err != nil {
		slog.Warn("webhook: create request failed", "webhook_id", webhookID, "error", err)
		return
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Forge-Event", event)
	req.Header.Set("X-Forge-Delivery", fmt.Sprintf("%d-%d", webhookID, time.Now().UnixMilli()))

	// Sign payload if secret is configured
	if secret != "" {
		mac := hmac.New(sha256.New, []byte(secret))
		mac.Write(body)
		sig := hex.EncodeToString(mac.Sum(nil))
		req.Header.Set("X-Forge-Signature", "sha256="+sig)
	}

	resp, err := s.client.Do(req)
	if err != nil {
		slog.Warn("webhook: delivery failed", "webhook_id", webhookID, "url", url, "error", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		slog.Warn("webhook: endpoint returned error", "webhook_id", webhookID, "status", resp.StatusCode)
	} else {
		slog.Debug("webhook: delivered", "webhook_id", webhookID, "event", event, "status", resp.StatusCode)
	}
}

// matchesEvent checks if a webhook's event list includes the given event.
func matchesEvent(events, event string) bool {
	if events == "*" {
		return true
	}
	for _, e := range splitEvents(events) {
		if e == event || e == "*" {
			return true
		}
	}
	return false
}

func splitEvents(s string) []string {
	var result []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == ',' {
			e := s[start:i]
			if len(e) > 0 {
				result = append(result, e)
			}
			start = i + 1
		}
	}
	if start < len(s) {
		result = append(result, s[start:])
	}
	return result
}

// --- Handlers ---

type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

// GET /api/projects/:id/webhooks
func (h *Handler) List(c *gin.Context) {
	projectID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.Fail(c, http.StatusBadRequest, "invalid project id")
		return
	}
	webhooks, err := h.svc.List(c.Request.Context(), projectID)
	if err != nil {
		response.Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	response.OK(c, gin.H{"webhooks": webhooks})
}

// POST /api/projects/:id/webhooks
func (h *Handler) Create(c *gin.Context) {
	projectID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.Fail(c, http.StatusBadRequest, "invalid project id")
		return
	}
	tid, _ := c.Get("tenant_id")
	tenantID, _ := tid.(int64)

	var req CreateWebhookRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, "invalid request: "+err.Error())
		return
	}

	id, err := h.svc.Create(c.Request.Context(), projectID, tenantID, &req)
	if err != nil {
		response.Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	response.OK(c, gin.H{"id": id})
}

// DELETE /api/projects/:id/webhooks/:webhookId
func (h *Handler) Delete(c *gin.Context) {
	webhookID, err := strconv.ParseInt(c.Param("webhookId"), 10, 64)
	if err != nil {
		response.Fail(c, http.StatusBadRequest, "invalid webhook id")
		return
	}
	if err := h.svc.Delete(c.Request.Context(), webhookID); err != nil {
		response.Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	response.OK(c, gin.H{"status": "deleted"})
}
