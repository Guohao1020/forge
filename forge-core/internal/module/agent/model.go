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

// AgentSession tracks an active agent session.
type AgentSession struct {
	ID        string    `json:"id"`
	ProjectID int64     `json:"project_id"`
	CreatedBy int64     `json:"created_by"`
	CreatedAt time.Time `json:"created_at"`
}
