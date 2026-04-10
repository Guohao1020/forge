package agent

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

// ClarifyRequest is the request body for POST /projects/:id/agent/sessions/:sid/clarify.
type ClarifyRequest struct {
	ToolUseID string `json:"tool_use_id"`
	Response  string `json:"response"`
}

// clarifyChannelMessage is the JSON message published to the Redis
// return channel agent:return:{session_id}.
type clarifyChannelMessage struct {
	Type      string `json:"type"`
	SessionID string `json:"session_id"`
	ToolUseID string `json:"tool_use_id"`
	Response  string `json:"response"`
}

const (
	maxToolUseIDLen  = 128
	maxResponseBytes = 4096 // 4 KiB
)

// Clarify handles POST /projects/:id/agent/sessions/:sid/clarify.
//
// Publishes a clarification response to the session's Redis return
// channel (agent:return:{session_id}). forge-core publishes WITHOUT
// checking if a clarification is pending — the ai-worker subscriber
// validates and discards stale/invalid messages.
//
// Spec reference: §2.9.2.g.
func (h *Handler) Clarify(c *gin.Context) {
	projectID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid project id"})
		return
	}

	sessionID := c.Param("sid")
	if sessionID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing session id"})
		return
	}

	// Auth
	tenantID, userID, ok := currentUser(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthenticated"})
		return
	}

	// Session ownership check
	if h.chat == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "agent storage not configured"})
		return
	}

	_, status := h.authorizeSessionAccess(c.Request.Context(), sessionID, projectID, tenantID, userID)
	switch status {
	case sessionOK:
		// fall through
	case sessionForbidden:
		c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
		return
	case sessionLookupFailed:
		c.JSON(http.StatusInternalServerError, gin.H{"error": "session lookup failed"})
		return
	default:
		c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
		return
	}

	// Parse and validate request body
	var req ClarifyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}

	if req.ToolUseID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "tool_use_id is required"})
		return
	}
	if len(req.ToolUseID) > maxToolUseIDLen {
		c.JSON(http.StatusBadRequest, gin.H{"error": "tool_use_id exceeds 128 characters"})
		return
	}
	if len(req.Response) > maxResponseBytes {
		c.JSON(http.StatusBadRequest, gin.H{"error": "response exceeds 4 KiB limit"})
		return
	}

	// Build the channel message
	msg := clarifyChannelMessage{
		Type:      "clarification_response",
		SessionID: sessionID,
		ToolUseID: req.ToolUseID,
		Response:  req.Response,
	}
	payload, err := json.Marshal(msg)
	if err != nil {
		slog.Error("failed to marshal clarify message",
			"error", err,
			"session_id", sessionID,
		)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	// Publish to Redis return channel
	channel := "agent:return:" + sessionID
	if err := h.rdb.Publish(c.Request.Context(), channel, payload).Err(); err != nil {
		slog.Error("failed to publish clarify response to Redis",
			"error", err,
			"session_id", sessionID,
			"channel", channel,
		)
		c.JSON(http.StatusBadGateway, gin.H{"error": "failed to publish response"})
		return
	}

	c.Status(http.StatusNoContent)
}
