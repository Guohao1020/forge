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
