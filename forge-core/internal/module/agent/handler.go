package agent

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
)

// Handler handles agent HTTP endpoints.
type Handler struct {
	service *Service
	rdb     *redis.Client
}

// NewHandler creates a new agent handler.
func NewHandler(service *Service, rdb *redis.Client) *Handler {
	return &Handler{
		service: service,
		rdb:     rdb,
	}
}

// RegisterRoutes adds agent routes to the router group.
//
//	POST /projects/:id/agent/chat   — submit a chat message (fire-and-forget)
//	GET  /projects/:id/agent/stream — SSE stream via Redis Streams
func (h *Handler) RegisterRoutes(rg *gin.RouterGroup) {
	rg.POST("/projects/:id/agent/chat", h.Chat)
	rg.GET("/projects/:id/agent/stream", h.Stream)
}

// Chat handles POST /projects/:id/agent/chat.
// Forwards the message to the AI worker and returns 202 Accepted.
func (h *Handler) Chat(c *gin.Context) {
	projectID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid project id"})
		return
	}

	var req ChatRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	resp, err := h.service.SubmitMessage(c.Request.Context(), projectID, req)
	if err != nil {
		slog.Error("failed to submit agent message", "error", err, "project_id", projectID)
		c.JSON(http.StatusBadGateway, gin.H{"error": "AI worker unavailable"})
		return
	}

	c.JSON(http.StatusAccepted, resp)
}

// Stream handles GET /projects/:id/agent/stream.
// Reads events from Redis Streams and pushes them as SSE.
//
// Query params:
//
//	session_id (required) — which session to stream
//	last_event_id        — for SSE reconnection, resume from this Redis entry ID
func (h *Handler) Stream(c *gin.Context) {
	sessionID := c.Query("session_id")
	if sessionID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "session_id required"})
		return
	}

	lastEventID := c.Query("last_event_id")
	if lastEventID == "" {
		lastEventID = "0" // Read from beginning
	}

	streamKey := fmt.Sprintf("agent:stream:%s", sessionID)

	// SSE headers
	c.Writer.Header().Set("Content-Type", "text/event-stream")
	c.Writer.Header().Set("Cache-Control", "no-cache")
	c.Writer.Header().Set("Connection", "keep-alive")
	c.Writer.Header().Set("X-Accel-Buffering", "no")
	c.Writer.Flush()

	ctx := c.Request.Context()

	// Heartbeat ticker
	heartbeat := time.NewTicker(15 * time.Second)
	defer heartbeat.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-heartbeat.C:
			fmt.Fprintf(c.Writer, ": heartbeat\n\n")
			c.Writer.Flush()
		default:
			// Read from Redis Stream with 1s block timeout
			entries, err := h.rdb.XRead(ctx, &redis.XReadArgs{
				Streams: []string{streamKey, lastEventID},
				Count:   10,
				Block:   1 * time.Second,
			}).Result()

			if err == redis.Nil || err == context.DeadlineExceeded {
				continue
			}
			if err != nil {
				slog.Error("redis XREAD failed", "error", err, "stream", streamKey)
				fmt.Fprintf(c.Writer, "event: error\ndata: {\"message\":\"stream error\"}\n\n")
				c.Writer.Flush()
				return
			}

			for _, stream := range entries {
				for _, msg := range stream.Messages {
					lastEventID = msg.ID
					data := mapToJSON(msg.Values)
					fmt.Fprintf(c.Writer, "id: %s\nevent: agent\ndata: %s\n\n", msg.ID, data)
					c.Writer.Flush()
				}
			}
		}
	}
}

func mapToJSON(m map[string]interface{}) string {
	// Simple JSON serialization for flat string maps
	result := "{"
	first := true
	for k, v := range m {
		if !first {
			result += ","
		}
		result += fmt.Sprintf(`"%s":"%s"`, k, v)
		first = false
	}
	result += "}"
	return result
}
