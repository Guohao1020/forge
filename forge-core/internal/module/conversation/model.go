package conversation

import (
	"encoding/json"
	"time"
)

// Role constants
const (
	RoleUser      = "user"
	RoleAssistant = "assistant"
	RoleSystem    = "system"
)

type Conversation struct {
	ID        int64            `json:"id"`
	TaskID    int64            `json:"task_id"`
	Role      string           `json:"role"`
	Content   string           `json:"content"`
	Metadata  *json.RawMessage `json:"metadata,omitempty"`
	TokensUsed *int            `json:"tokens_used,omitempty"`
	CreatedAt time.Time        `json:"created_at"`
}

type ModelCall struct {
	ID           int64     `json:"id"`
	TenantID     int64     `json:"tenant_id"`
	TaskID       int64     `json:"task_id"`
	StepType     *string   `json:"step_type,omitempty"`
	Model        string    `json:"model"`
	Provider     string    `json:"provider"`
	Purpose      string    `json:"purpose"`
	InputTokens  int       `json:"input_tokens"`
	OutputTokens int       `json:"output_tokens"`
	TotalTokens  int       `json:"total_tokens"`
	CostCents    int       `json:"cost_cents"`
	LatencyMs    int       `json:"latency_ms"`
	Status       string    `json:"status"`
	ErrorCode    *string   `json:"error_code,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
}

type ReviewResult struct {
	ID         int64            `json:"id"`
	TaskID     int64            `json:"task_id"`
	StepID     *int64           `json:"step_id,omitempty"`
	ReviewType string           `json:"review_type"`
	Score      *int             `json:"score,omitempty"`
	Passed     bool             `json:"passed"`
	Findings   json.RawMessage  `json:"findings"`
	Summary    *string          `json:"summary,omitempty"`
	CreatedAt  time.Time        `json:"created_at"`
}

type SendMessageRequest struct {
	Content string `json:"content" binding:"required,min=1,max=10000"`
}

type SendMessageResponse struct {
	Conversation *Conversation          `json:"conversation"`
	Status       string                 `json:"status"`
	Metadata     map[string]interface{} `json:"metadata"`
}

type ConversationListResponse struct {
	Messages []*Conversation `json:"messages"`
}

type PlanConfirmResponse struct {
	Conversation *Conversation          `json:"conversation"`
	Status       string                 `json:"status"`   // "plan_review"
	PlanData     map[string]interface{} `json:"planData"`
}
