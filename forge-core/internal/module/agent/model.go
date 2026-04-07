package agent

import "time"

// ChatRequest is the request body for POST /projects/:id/agent/chat.
type ChatRequest struct {
	SessionID     string `json:"session_id,omitempty"`
	Message       string `json:"message" binding:"required"`
	Model         string `json:"model,omitempty"`
	SystemPrompt  string `json:"system_prompt,omitempty"`
	CorrelationID string `json:"correlation_id,omitempty"`
}

// ChatResponse is the response for POST /projects/:id/agent/chat.
type ChatResponse struct {
	SessionID     string `json:"session_id"`
	Status        string `json:"status"`
	CorrelationID string `json:"correlation_id"`
}

// StreamEvent represents an SSE event from Redis Streams.
type StreamEvent struct {
	Type          string `json:"type"`
	Text          string `json:"text,omitempty"`
	ToolName      string `json:"tool_name,omitempty"`
	ToolInput     string `json:"tool_input,omitempty"`
	Output        string `json:"output,omitempty"`
	IsError       string `json:"is_error,omitempty"`
	Message       string `json:"message,omitempty"`
	InputTokens   string `json:"input_tokens,omitempty"`
	OutputTokens  string `json:"output_tokens,omitempty"`
	Recoverable   string `json:"recoverable,omitempty"`
	CorrelationID string `json:"correlation_id,omitempty"`
}

// AgentSession is a durable agent terminal conversation. One session maps
// to one Redis Stream (agent:stream:{id}) AND one row in
// engine.agent_sessions. Sessions may optionally link to an engine.tasks
// row when the user wants the conversation tied to a Temporal workflow.
type AgentSession struct {
	ID            string     `json:"id"`
	TenantID      int64      `json:"tenant_id"`
	ProjectID     int64      `json:"project_id"`
	TaskID        *int64     `json:"task_id,omitempty"`
	Title         *string    `json:"title,omitempty"`
	CreatedBy     int64      `json:"created_by"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
	Archived      bool       `json:"archived"`
	LastMessageAt *time.Time `json:"last_message_at,omitempty"`
}

// AgentMessage is one durable event from an agent session. The
// `event_type` mirrors the Python stream_events dataclasses
// (text_delta, turn_complete, tool_started, tool_completed, error,
// thinking_started, thinking_stopped, fix_loop_started,
// fix_loop_completed, session_complete). The canonical payload lives
// in `Data` as JSON.
type AgentMessage struct {
	ID            int64     `json:"id"`
	SessionID     string    `json:"session_id"`
	RedisID       *string   `json:"redis_id,omitempty"`
	EventType     string    `json:"event_type"`
	Role          *string   `json:"role,omitempty"`
	Content       *string   `json:"content,omitempty"`
	ToolName      *string   `json:"tool_name,omitempty"`
	Data          []byte    `json:"data"`
	CorrelationID *string   `json:"correlation_id,omitempty"`
	CreatedAt     time.Time `json:"created_at"`
}

// CreateSessionRequest is POST /projects/:id/agent/sessions.
type CreateSessionRequest struct {
	Title  *string `json:"title,omitempty"`
	TaskID *int64  `json:"task_id,omitempty"`
}

// ListSessionsResponse is GET /projects/:id/agent/sessions.
type ListSessionsResponse struct {
	Sessions []AgentSession `json:"sessions"`
	Total    int            `json:"total"`
}

// ListMessagesResponse is GET /projects/:id/agent/sessions/:sid/messages.
type ListMessagesResponse struct {
	Messages []AgentMessage `json:"messages"`
	Total    int            `json:"total"`
}
