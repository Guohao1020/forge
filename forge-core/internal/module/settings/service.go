package settings

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shulex/forge/forge-core/internal/pkg/response"
)

// Setting is a key-value pair stored per tenant.
type Setting struct {
	Key       string `json:"key"`
	Value     string `json:"value"`
	Category  string `json:"category"` // general, ai, deploy, notification
	UpdatedAt string `json:"updatedAt,omitempty"`
}

// Defaults returns the platform default settings.
func Defaults() map[string]Setting {
	return map[string]Setting{
		"ai.default_model":      {Key: "ai.default_model", Value: "qwen3-coder-plus", Category: "ai"},
		"ai.fallback_chain":     {Key: "ai.fallback_chain", Value: "claude-sonnet-4,gpt-4o,deepseek-chat", Category: "ai"},
		"ai.max_tokens":         {Key: "ai.max_tokens", Value: "8192", Category: "ai"},
		"ai.temperature":        {Key: "ai.temperature", Value: "0.1", Category: "ai"},
		"deploy.auto_merge":     {Key: "deploy.auto_merge", Value: "false", Category: "deploy"},
		"deploy.require_review": {Key: "deploy.require_review", Value: "true", Category: "deploy"},
		"notify.slack_webhook":  {Key: "notify.slack_webhook", Value: "", Category: "notification"},
		"notify.on_completion":  {Key: "notify.on_completion", Value: "true", Category: "notification"},
		"general.language":      {Key: "general.language", Value: "zh-CN", Category: "general"},
		"general.timezone":      {Key: "general.timezone", Value: "Asia/Shanghai", Category: "general"},
		"entropy.default_schedule": {Key: "entropy.default_schedule", Value: "weekly", Category: "general"},
	}
}

type Service struct {
	db *pgxpool.Pool
}

func NewService(db *pgxpool.Pool) *Service {
	return &Service{db: db}
}

// List returns all settings for a tenant, merged with defaults.
func (s *Service) List(ctx context.Context, tenantID int64) ([]Setting, error) {
	// Start with defaults
	settings := Defaults()

	// Override with stored values
	rows, err := s.db.Query(ctx,
		`SELECT key, value, COALESCE(category, 'general'),
		        TO_CHAR(updated_at, 'YYYY-MM-DD"T"HH24:MI:SS"Z"')
		 FROM engine.platform_settings
		 WHERE tenant_id = $1`,
		tenantID,
	)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var setting Setting
			if err := rows.Scan(&setting.Key, &setting.Value, &setting.Category, &setting.UpdatedAt); err != nil {
				continue
			}
			settings[setting.Key] = setting
		}
	}

	// Convert map to sorted list
	result := make([]Setting, 0, len(settings))
	for _, s := range settings {
		result = append(result, s)
	}
	return result, nil
}

// Get returns a single setting value.
func (s *Service) Get(ctx context.Context, tenantID int64, key string) (string, error) {
	var value string
	err := s.db.QueryRow(ctx,
		`SELECT value FROM engine.platform_settings WHERE tenant_id = $1 AND key = $2`,
		tenantID, key,
	).Scan(&value)
	if err != nil {
		// Return default if not found
		if d, ok := Defaults()[key]; ok {
			return d.Value, nil
		}
		return "", nil
	}
	return value, nil
}

// Set stores a setting value.
func (s *Service) Set(ctx context.Context, tenantID int64, key, value, category string) error {
	if category == "" {
		if d, ok := Defaults()[key]; ok {
			category = d.Category
		} else {
			category = "general"
		}
	}

	_, err := s.db.Exec(ctx,
		`INSERT INTO engine.platform_settings (tenant_id, key, value, category)
		 VALUES ($1, $2, $3, $4)
		 ON CONFLICT (tenant_id, key) DO UPDATE SET value = $3, category = $4, updated_at = NOW()`,
		tenantID, key, value, category,
	)
	return err
}

// Handler for HTTP endpoints
type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

// GET /api/settings
func (h *Handler) List(c *gin.Context) {
	tid, _ := c.Get("tenant_id")
	tenantID, _ := tid.(int64)

	settings, err := h.svc.List(c.Request.Context(), tenantID)
	if err != nil {
		response.Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	response.OK(c, gin.H{"settings": settings})
}

// GET /api/settings/:key
func (h *Handler) Get(c *gin.Context) {
	tid, _ := c.Get("tenant_id")
	tenantID, _ := tid.(int64)
	key := c.Param("key")

	value, err := h.svc.Get(c.Request.Context(), tenantID, key)
	if err != nil {
		response.Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	response.OK(c, gin.H{"key": key, "value": value})
}

// PUT /api/settings/:key
func (h *Handler) Set(c *gin.Context) {
	tid, _ := c.Get("tenant_id")
	tenantID, _ := tid.(int64)
	key := c.Param("key")

	var body struct {
		Value    string `json:"value"`
		Category string `json:"category"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		response.Fail(c, http.StatusBadRequest, "invalid request")
		return
	}

	if err := h.svc.Set(c.Request.Context(), tenantID, key, body.Value, body.Category); err != nil {
		response.Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	response.OK(c, gin.H{"status": "setting_saved"})
}

// PUT /api/settings (bulk update)
func (h *Handler) BulkSet(c *gin.Context) {
	tid, _ := c.Get("tenant_id")
	tenantID, _ := tid.(int64)

	var body map[string]string
	raw, err := c.GetRawData()
	if err != nil {
		response.Fail(c, http.StatusBadRequest, "invalid request")
		return
	}
	if err := json.Unmarshal(raw, &body); err != nil {
		response.Fail(c, http.StatusBadRequest, "invalid JSON")
		return
	}

	count := 0
	for key, value := range body {
		if err := h.svc.Set(c.Request.Context(), tenantID, key, value, ""); err != nil {
			continue
		}
		count++
	}
	response.OK(c, gin.H{"status": "bulk_saved", "count": strconv.Itoa(count)})
}
