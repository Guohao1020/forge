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

// chatStore is the minimal slice of Repository that Handler.Chat and
// Handler.Stream depend on. Defining it as an interface lets unit
// tests inject a fake without standing up a real PostgreSQL —
// *Repository automatically satisfies it because Go interfaces are
// structural. All other endpoints continue to use the concrete
// *Repository field.
//
// GetSession is the same call the dual-storage tenant ownership check
// uses (TASK 6 of agent-base-loop-reduction): it scopes by project_id
// in SQL and the handler additionally checks tenant_id + created_by in
// memory before allowing access. This is data-privacy critical.
type chatStore interface {
	CreateSession(
		ctx context.Context,
		id string,
		tenantID int64,
		projectID int64,
		createdBy int64,
		title *string,
		taskID *int64,
	) (*AgentSession, error)
	GetSession(ctx context.Context, sessionID string, projectID int64) (*AgentSession, error)
	InsertMessage(ctx context.Context, m *AgentMessage) error
}

// Handler handles agent HTTP endpoints.
type Handler struct {
	service *Service
	rdb     *redis.Client
	repo    *Repository
	// chat is the dual-storage hook used by Chat(). It is the same value
	// as repo when wired through NewHandler, but can be overridden in
	// tests via NewHandlerForTest to inject a fake without spinning up
	// PostgreSQL.
	chat chatStore
}

// NewHandler creates a new agent handler. The Repository is optional —
// when nil, the new session/message endpoints return 503 and the
// original Chat/Stream endpoints still work against Redis alone. This
// lets the handler boot before the PG migration has been applied.
func NewHandler(service *Service, rdb *redis.Client, repo *Repository) *Handler {
	h := &Handler{
		service: service,
		rdb:     rdb,
		repo:    repo,
	}
	if repo != nil {
		h.chat = repo
	}
	return h
}

// NewHandlerForTest is a test-only constructor that wires a fake
// chatStore directly. It exists so handler_test.go can validate the
// dual-storage path (TASK 5: persist user_message before ai-worker)
// without a live PostgreSQL.
func NewHandlerForTest(service *Service, rdb *redis.Client, chat chatStore) *Handler {
	return &Handler{
		service: service,
		rdb:     rdb,
		chat:    chat,
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
//
// Durability contract (TASK 5 of agent-base-loop-reduction):
//
//  1. If req.SessionID is empty, allocate a new session row first so PG
//     stays the source of truth and the user_message can satisfy the
//     agent_messages.session_id FK.
//  2. Persist the user_message to PG BEFORE calling ai-worker. PG write
//     failure → 500 (we have not contacted ai-worker yet, so the user can
//     safely retry). ai-worker failure → 502, but the user_message is
//     already durable and will hydrate on the next sidebar load.
//
// Repository is optional: when h.repo is nil (PG not configured / migration
// 024 not applied), Chat falls back to the legacy fire-and-forget path so
// the binary still boots in dev. The dual-storage promise only holds when
// the repository is wired.
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

	// Legacy path: no chat store wired, fall through to fire-and-forget.
	// Preserves dev/boot ergonomics when migration 024 has not run yet.
	// We pass tenantID=0 here as a sentinel meaning "unknown tenant" —
	// SubmitMessage will emit an empty workspace_path so ai-worker uses
	// the legacy QueryEngine chat path. The dual-storage path below
	// passes the real authenticated tenantID.
	if h.chat == nil {
		resp, err := h.service.SubmitMessage(c.Request.Context(), 0, projectID, req)
		if err != nil {
			slog.Error("failed to submit agent message", "error", err, "project_id", projectID)
			c.JSON(http.StatusBadGateway, gin.H{"error": "AI worker unavailable"})
			return
		}
		c.JSON(http.StatusAccepted, resp)
		return
	}

	tenantID, userID, ok := currentUser(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthenticated"})
		return
	}

	ctx := c.Request.Context()

	// Step 1: ensure a session exists AND belongs to the caller.
	//
	// New conversations come in with an empty SessionID — Handler creates
	// the row, which inherently belongs to the caller.
	//
	// Existing conversations carry SessionID — we MUST verify ownership
	// before writing into someone else's session. Plan TASK 6 / G2.
	if req.SessionID == "" {
		newID := uuid.NewString()
		session, err := h.chat.CreateSession(ctx, newID, tenantID, projectID, userID, nil, nil)
		if err != nil {
			slog.Error("create agent session failed", "error", err, "project_id", projectID)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "create session failed"})
			return
		}
		req.SessionID = session.ID
	} else {
		_, status := h.authorizeSessionAccess(ctx, req.SessionID, projectID, tenantID, userID)
		switch status {
		case sessionOK:
			// fall through to step 2
		case sessionForbidden:
			c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
			return
		case sessionLookupFailed:
			c.JSON(http.StatusInternalServerError, gin.H{"error": "session lookup failed"})
			return
		default:
			// sessionMissing should be impossible here because h.chat != nil
			// (we checked at the top of Chat). Treat as forbidden out of paranoia.
			c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
			return
		}
	}

	// Step 2: persist the user_message before contacting ai-worker. If PG
	// write fails we abort with 500 — we have not yet reached out, so the
	// user can safely retry without producing a duplicate downstream call.
	userRole := "user"
	msgContent := req.Message
	if err := h.chat.InsertMessage(ctx, &AgentMessage{
		SessionID: req.SessionID,
		EventType: "user_message",
		Role:      &userRole,
		Content:   &msgContent,
	}); err != nil {
		slog.Error("persist user message failed", "error", err, "session_id", req.SessionID)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "persist message failed"})
		return
	}

	// Step 3: forward to ai-worker. On failure return 502 — the user
	// message is already durable, so the next sidebar load will hydrate it
	// and the user can resend without losing context.
	resp, err := h.service.SubmitMessage(ctx, tenantID, projectID, req)
	if err != nil {
		slog.Error("failed to submit agent message",
			"error", err,
			"project_id", projectID,
			"session_id", req.SessionID,
		)
		c.JSON(http.StatusBadGateway, gin.H{
			"error":      "AI worker unavailable",
			"session_id": req.SessionID,
		})
		return
	}

	// ai-worker echoes its own session_id; on a fresh conversation it
	// should match what we just inserted. Prefer the locally-created id
	// if ai-worker returns blank for any reason.
	if resp.SessionID == "" {
		resp.SessionID = req.SessionID
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
// Security (TASK 6 / G3): when the chat store is wired we MUST verify
// the caller owns the session before subscribing to its Redis stream.
// Without this check, any authenticated user who knows or guesses
// another user's session_id can read their live conversation in real
// time. The check matches Chat()'s ownership rules: tenant_id and
// created_by must both match the caller. We deliberately return 403
// (not 404) to avoid leaking session existence across tenants.
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

	// Tenant ownership gate. Only enforced when chat store is wired —
	// the legacy Redis-only path stays open for dev/boot scenarios.
	if h.chat != nil {
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
		_, status := h.authorizeSessionAccess(c.Request.Context(), sessionID, projectID, tenantID, userID)
		switch status {
		case sessionOK:
			// authorized — fall through to SSE setup
		case sessionForbidden:
			c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
			return
		case sessionLookupFailed:
			c.JSON(http.StatusInternalServerError, gin.H{"error": "session lookup failed"})
			return
		default:
			// sessionMissing should not occur because h.chat != nil; treat as forbidden.
			c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
			return
		}
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

// sessionOwnershipStatus is the result of authorizeSessionAccess.
type sessionOwnershipStatus int

const (
	sessionOK sessionOwnershipStatus = iota
	sessionMissing
	sessionForbidden
	sessionLookupFailed
)

// authorizeSessionAccess loads a session via the chatStore and verifies
// the caller owns it. Returns the session pointer when status==sessionOK.
//
// CRITICAL — TASK 6 of agent-base-loop-reduction: this is the data
// privacy gate for Chat and Stream. The session row is scoped by
// project_id in SQL, but a malicious user could craft a request with
// their own project_id and someone else's session_id. We additionally
// check that session.TenantID matches the caller's tenant AND that
// session.CreatedBy matches the caller's user_id.
//
// We deliberately collapse "session not found in this project" and
// "session belongs to another tenant/user" into the same outcome
// (sessionForbidden → 403). Returning 404 vs 403 differently would
// leak existence information across tenants.
func (h *Handler) authorizeSessionAccess(
	ctx context.Context,
	sessionID string,
	projectID int64,
	tenantID int64,
	userID int64,
) (*AgentSession, sessionOwnershipStatus) {
	if h.chat == nil {
		// No store wired — the caller must decide whether to allow the
		// legacy unsafe path or refuse. We surface this as missing.
		return nil, sessionMissing
	}
	session, err := h.chat.GetSession(ctx, sessionID, projectID)
	if err != nil {
		slog.Error("session lookup failed",
			"error", err,
			"session_id", sessionID,
			"project_id", projectID,
		)
		return nil, sessionLookupFailed
	}
	if session == nil {
		return nil, sessionForbidden
	}
	if session.TenantID != tenantID || session.CreatedBy != userID {
		slog.Warn("cross-tenant session access denied",
			"session_id", sessionID,
			"session_tenant", session.TenantID,
			"session_owner", session.CreatedBy,
			"caller_tenant", tenantID,
			"caller_user", userID,
			"project_id", projectID,
		)
		return nil, sessionForbidden
	}
	return session, sessionOK
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
