package task

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	goredis "github.com/redis/go-redis/v9"
)

// SSEHub manages SSE connections for task progress streaming.
type SSEHub struct {
	mu      sync.RWMutex
	clients map[int64]map[chan []byte]struct{} // taskID -> set of channels
}

func NewSSEHub() *SSEHub {
	return &SSEHub{
		clients: make(map[int64]map[chan []byte]struct{}),
	}
}

// Broadcast sends an event to all SSE clients watching a specific task.
func (h *SSEHub) Broadcast(taskID int64, event TaskProgressEvent) {
	data, err := json.Marshal(event)
	if err != nil {
		return
	}

	h.mu.RLock()
	defer h.mu.RUnlock()

	if subs, ok := h.clients[taskID]; ok {
		for ch := range subs {
			select {
			case ch <- data:
			default:
				// Skip slow clients
			}
		}
	}
}

func (h *SSEHub) subscribe(taskID int64) chan []byte {
	ch := make(chan []byte, 16)
	h.mu.Lock()
	if h.clients[taskID] == nil {
		h.clients[taskID] = make(map[chan []byte]struct{})
	}
	h.clients[taskID][ch] = struct{}{}
	h.mu.Unlock()
	return ch
}

func (h *SSEHub) unsubscribe(taskID int64, ch chan []byte) {
	h.mu.Lock()
	if subs, ok := h.clients[taskID]; ok {
		delete(subs, ch)
		if len(subs) == 0 {
			delete(h.clients, taskID)
		}
	}
	h.mu.Unlock()
	close(ch)
}

// SSEHandler handles SSE streaming HTTP requests.
type SSEHandler struct {
	hub *SSEHub
	rdb *goredis.Client // Redis client for code streaming subscription
}

func NewSSEHandler(hub *SSEHub, rdb *goredis.Client) *SSEHandler {
	return &SSEHandler{hub: hub, rdb: rdb}
}

// GET /api/stream/tasks/:taskId
func (h *SSEHandler) Stream(c *gin.Context) {
	taskID, err := strconv.ParseInt(c.Param("taskId"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid task id"})
		return
	}

	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no")

	ch := h.hub.subscribe(taskID)
	defer h.hub.unsubscribe(taskID, ch)

	reqCtx := c.Request.Context()
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	// Subscribe to Redis streaming channels (if Redis available)
	var redisCh <-chan *goredis.Message
	var analyzeRedisCh <-chan *goredis.Message
	if h.rdb != nil {
		subCtx, cancelSub := context.WithCancel(reqCtx)
		defer cancelSub()

		// Code generation streaming
		codeChannel := fmt.Sprintf("code:stream:%d", taskID)
		codeSub := h.rdb.Subscribe(subCtx, codeChannel)
		redisCh = codeSub.Channel()
		defer func() {
			if err := codeSub.Close(); err != nil {
				slog.Debug("redis code sub close", "error", err)
			}
		}()

		// Analyze streaming (P0: thinking process)
		analyzeChannel := fmt.Sprintf("analyze:stream:%d", taskID)
		analyzeSub := h.rdb.Subscribe(subCtx, analyzeChannel)
		analyzeRedisCh = analyzeSub.Channel()
		defer func() {
			if err := analyzeSub.Close(); err != nil {
				slog.Debug("redis analyze sub close", "error", err)
			}
		}()
	}

	// Send initial connection event
	fmt.Fprintf(c.Writer, "data: {\"type\":\"connected\",\"task_id\":%d}\n\n", taskID)
	c.Writer.Flush()

	for {
		select {
		case <-reqCtx.Done():
			slog.Debug("sse client disconnected", "task_id", taskID)
			return
		case data := <-ch:
			fmt.Fprintf(c.Writer, "data: %s\n\n", data)
			c.Writer.Flush()
		case msg, ok := <-redisCh:
			if !ok {
				redisCh = nil // channel closed, stop selecting
				continue
			}
			// Forward as a code_token SSE event
			evt := TaskProgressEvent{
				Type:   "code_token",
				TaskID: taskID,
				Data:   msg.Payload,
			}
			evtData, err := json.Marshal(evt)
			if err == nil {
				fmt.Fprintf(c.Writer, "data: %s\n\n", evtData)
				c.Writer.Flush()
			}
		case msg, ok := <-analyzeRedisCh:
			if !ok {
				analyzeRedisCh = nil
				continue
			}
			// Forward as analyze_token SSE event (thinking process)
			evt := TaskProgressEvent{
				Type:   "analyze_token",
				TaskID: taskID,
				Data:   msg.Payload,
			}
			evtData, err := json.Marshal(evt)
			if err == nil {
				fmt.Fprintf(c.Writer, "data: %s\n\n", evtData)
				c.Writer.Flush()
			}
		case <-ticker.C:
			fmt.Fprint(c.Writer, ": keepalive\n\n")
			c.Writer.Flush()
		}
	}
}
