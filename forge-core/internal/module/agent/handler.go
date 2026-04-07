package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

// Handler handles agent HTTP endpoints.
type Handler struct {
	service *Service
	rdb     *redis.Client
	repo    *Repository
}

// NewHandler creates a new agent handler. The Repository is optional —
// when nil, the new session/message endpoints return 503 and the
// original Chat/Stream endpoints still work against Redis alone. This
// lets the handler boot before the PG migration has been applied.
func NewHandler(service *Service, rdb *redis.Client, repo *Repository) *Handler {
	return &Handler{
		service: service,
		rdb:     rdb,
		repo:    repo,
	}
}

// RegisterRoutes adds agent routes to the router group.
//
//	POST   /projects/:id/agent/chat                     — submit a chat message
//	GET    /projects/:id/agent/stream                   — SSE stream via Redis Streams
//	GET    /projects/:id/agent/sessions                 — list non-archived sessions
//	POST   /projects/:id/agent/sessions                 — create a new session
//	DELETE /projects/:id/agent/sessions/:sid            — archive a session
//	PATCH  /projects/:id/agent/sessions/:sid            — rename a session
//	GET    /projects/:id/agent/sessions/:sid/messages   — list durable messages (history hydration)
//	GET    /projects/:id/agent/suggestions              — empty-state starter prompts
func (h *Handler) RegisterRoutes(rg *gin.RouterGroup) {
	rg.POST("/projects/:id/agent/chat", h.Chat)
	rg.GET("/projects/:id/agent/stream", h.Stream)
	rg.GET("/projects/:id/agent/sessions", h.ListSessions)
	rg.POST("/projects/:id/agent/sessions", h.CreateSession)
	rg.DELETE("/projects/:id/agent/sessions/:sid", h.ArchiveSession)
	rg.PATCH("/projects/:id/agent/sessions/:sid", h.RenameSession)
	rg.GET("/projects/:id/agent/sessions/:sid/messages", h.ListSessionMessages)
	rg.GET("/projects/:id/agent/suggestions", h.Suggestions)
}

// Suggestions handles GET /projects/:id/agent/suggestions.
// Returns 3 language-appropriate starter prompts for the Agent
// Terminal empty state, derived from the project's tech stack. Never
// fails — falls back to language-agnostic defaults on any error so
// the empty state always renders.
func (h *Handler) Suggestions(c *gin.Context) {
	projectID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid project id"})
		return
	}
	resp, _ := h.generateSuggestions(c.Request.Context(), projectID)
	c.JSON(http.StatusOK, resp)
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

// streamEvent is a single Redis Stream entry delivered from the reader
// goroutine to the writer loop.
type streamEvent struct {
	ID     string
	Values map[string]interface{}
}

// Stream handles GET /projects/:id/agent/stream.
// Reads events from Redis Streams and pushes them as SSE.
//
// Query params:
//
//	session_id (required) — which session to stream
//	last_event_id        — for SSE reconnection, resume from this Redis entry ID
//
// Architecture: a dedicated goroutine owns the blocking XREAD loop and sends
// each entry through `events`. The writer loop sits on a `select` over
// ctx.Done, heartbeat ticks, events, and errors, so heartbeats keep firing
// even when XREAD is blocked and the goroutine exits cleanly on client
// disconnect. The earlier `select { default: XREAD }` pattern busy-looped
// at 1 call/sec per connection because default always fired first.
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

	// Heartbeat ticker — fires every 15s so idle connections don't time out.
	heartbeat := time.NewTicker(15 * time.Second)
	defer heartbeat.Stop()

	// events is the bridge between the Redis reader goroutine and the SSE
	// writer loop. errCh carries non-nil Redis errors and terminates the
	// connection. Both channels are closed by the goroutine on exit so the
	// writer loop can detect termination via zero-value receives.
	events := make(chan streamEvent, 16)
	errCh := make(chan error, 1)

	go h.streamReader(ctx, streamKey, lastEventID, events, errCh)

	for {
		select {
		case <-ctx.Done():
			// Client disconnected or server shutdown. The reader goroutine
			// also sees ctx.Done() and exits; we don't need to drain events.
			return

		case <-heartbeat.C:
			if _, err := fmt.Fprintf(c.Writer, ": heartbeat\n\n"); err != nil {
				return
			}
			c.Writer.Flush()

		case ev, ok := <-events:
			if !ok {
				// Reader closed the channel — either ctx cancelled or an
				// unrecoverable error already sent on errCh.
				return
			}
			data := mapToJSON(ev.Values)
			if _, err := fmt.Fprintf(c.Writer, "id: %s\nevent: agent\ndata: %s\n\n", ev.ID, data); err != nil {
				return
			}
			c.Writer.Flush()

		case err := <-errCh:
			if err != nil {
				slog.Error("redis XREAD failed", "error", err, "stream", streamKey)
				fmt.Fprintf(c.Writer, "event: error\ndata: {\"message\":\"stream error\"}\n\n")
				c.Writer.Flush()
			}
			return
		}
	}
}

// streamReader runs in its own goroutine and owns the blocking XREAD loop.
// It sends every Redis Stream entry to `events` and any unrecoverable error
// to `errCh`, then closes both channels so the writer loop can terminate.
func (h *Handler) streamReader(
	ctx context.Context,
	streamKey string,
	lastEventID string,
	events chan<- streamEvent,
	errCh chan<- error,
) {
	defer close(events)
	defer close(errCh)

	for {
		// Respect cancellation before issuing another blocking call.
		if ctx.Err() != nil {
			return
		}

		entries, err := h.rdb.XRead(ctx, &redis.XReadArgs{
			Streams: []string{streamKey, lastEventID},
			Count:   10,
			Block:   1 * time.Second,
		}).Result()

		// XREAD returns redis.Nil when the block interval expires with no
		// new data — this is the normal idle path, not an error. Loop back
		// to the ctx check and issue another blocking read.
		if err == redis.Nil || errors.Is(err, context.DeadlineExceeded) {
			continue
		}
		if err != nil {
			// Context cancellation is expected during shutdown — exit
			// silently without forwarding the error.
			if ctx.Err() != nil {
				return
			}
			select {
			case errCh <- err:
			case <-ctx.Done():
			}
			return
		}

		for _, stream := range entries {
			for _, msg := range stream.Messages {
				lastEventID = msg.ID
				select {
				case events <- streamEvent{ID: msg.ID, Values: msg.Values}:
				case <-ctx.Done():
					return
				}
			}
		}
	}
}

// ---- Session / message endpoints (Stream 4b dual storage) ---------------

// currentUser extracts the caller's auth context from gin. Used by the
// session endpoints to enforce per-user scoping.
func currentUser(c *gin.Context) (tenantID int64, userID int64, ok bool) {
	tID, tOK := c.Get("tenant_id")
	uID, uOK := c.Get("user_id")
	if !tOK || !uOK {
		return 0, 0, false
	}
	return tID.(int64), uID.(int64), true
}

// ListSessions handles GET /projects/:id/agent/sessions.
// Returns non-archived agent sessions for the project, sorted by
// last_message_at DESC for the TaskSwitcher sidebar.
func (h *Handler) ListSessions(c *gin.Context) {
	if h.repo == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "agent storage not configured"})
		return
	}
	projectID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid project id"})
		return
	}
	limit := 50
	if v := c.Query("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			limit = n
		}
	}
	sessions, total, err := h.repo.ListSessions(c.Request.Context(), projectID, limit)
	if err != nil {
		slog.Error("list agent sessions failed", "error", err, "project_id", projectID)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "list sessions failed"})
		return
	}
	c.JSON(http.StatusOK, ListSessionsResponse{Sessions: sessions, Total: total})
}

// CreateSession handles POST /projects/:id/agent/sessions.
func (h *Handler) CreateSession(c *gin.Context) {
	if h.repo == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "agent storage not configured"})
		return
	}
	projectID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid project id"})
		return
	}
	tenantID, userID, ok := currentUser(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthenticated"})
		return
	}

	var req CreateSessionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		// Empty body is fine — title and task_id are both optional.
		if err.Error() != "EOF" {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
	}

	// Frontend never sees the raw UUID until this response, so generate
	// it server-side. The id becomes the Redis Stream key for subsequent
	// chat messages.
	id := uuid.NewString()
	session, err := h.repo.CreateSession(
		c.Request.Context(), id, tenantID, projectID, userID, req.Title, req.TaskID,
	)
	if err != nil {
		slog.Error("create agent session failed", "error", err, "project_id", projectID)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "create session failed"})
		return
	}
	c.JSON(http.StatusCreated, session)
}

// ArchiveSession handles DELETE /projects/:id/agent/sessions/:sid.
// Soft delete — flips the archived flag, preserves the messages.
func (h *Handler) ArchiveSession(c *gin.Context) {
	if h.repo == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "agent storage not configured"})
		return
	}
	projectID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid project id"})
		return
	}
	sessionID := c.Param("sid")
	if err := h.repo.ArchiveSession(c.Request.Context(), sessionID, projectID); err != nil {
		slog.Error("archive agent session failed", "error", err, "session_id", sessionID)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "archived"})
}

// RenameSession handles PATCH /projects/:id/agent/sessions/:sid.
// Body: { "title": "New title" }
func (h *Handler) RenameSession(c *gin.Context) {
	if h.repo == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "agent storage not configured"})
		return
	}
	projectID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid project id"})
		return
	}
	sessionID := c.Param("sid")

	var req struct {
		Title string `json:"title" binding:"required,max=200"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.repo.UpdateSessionTitle(
		c.Request.Context(), sessionID, projectID, req.Title,
	); err != nil {
		slog.Error("rename agent session failed", "error", err, "session_id", sessionID)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "renamed"})
}

// ListSessionMessages handles GET /projects/:id/agent/sessions/:sid/messages.
// Returns the durable message log for history hydration on page load.
func (h *Handler) ListSessionMessages(c *gin.Context) {
	if h.repo == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "agent storage not configured"})
		return
	}
	projectID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid project id"})
		return
	}
	sessionID := c.Param("sid")

	// Validate the session belongs to the project before returning
	// messages, so we can't leak other projects' history.
	session, err := h.repo.GetSession(c.Request.Context(), sessionID, projectID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if session == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "session not found"})
		return
	}

	limit := 500
	if v := c.Query("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			limit = n
		}
	}

	messages, err := h.repo.ListMessages(c.Request.Context(), sessionID, limit)
	if err != nil {
		slog.Error("list agent messages failed", "error", err, "session_id", sessionID)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "list messages failed"})
		return
	}
	c.JSON(http.StatusOK, ListMessagesResponse{Messages: messages, Total: len(messages)})
}

// mapToJSON serializes a Redis Stream entry to a JSON object the browser can parse.
// Values come from XREAD as `interface{}` (in practice strings) and may contain quotes,
// backslashes, or newlines from the LLM output — naive concatenation would produce
// invalid JSON and the EventSource would silently drop the event.
func mapToJSON(m map[string]interface{}) string {
	out := make(map[string]string, len(m))
	for k, v := range m {
		out[k] = fmt.Sprint(v)
	}
	b, err := json.Marshal(out)
	if err != nil {
		// Fall back to a minimal, always-valid envelope on the unlikely error.
		return `{"type":"error","message":"failed to encode event"}`
	}
	return string(b)
}
