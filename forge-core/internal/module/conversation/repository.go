package conversation

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

func (r *Repository) Create(ctx context.Context, conv *Conversation) error {
	err := r.db.QueryRow(ctx,
		`INSERT INTO engine.conversations (task_id, role, content, metadata, tokens_used)
		 VALUES ($1, $2, $3, $4, $5)
		 RETURNING id, created_at`,
		conv.TaskID, conv.Role, conv.Content, conv.Metadata, conv.TokensUsed,
	).Scan(&conv.ID, &conv.CreatedAt)
	if err != nil {
		return fmt.Errorf("create conversation: %w", err)
	}
	return nil
}

func (r *Repository) ListByTaskID(ctx context.Context, taskID int64) ([]*Conversation, error) {
	rows, err := r.db.Query(ctx,
		`SELECT id, task_id, role, content, metadata, tokens_used, created_at
		 FROM engine.conversations WHERE task_id = $1 ORDER BY created_at ASC`,
		taskID,
	)
	if err != nil {
		return nil, fmt.Errorf("list conversations: %w", err)
	}
	defer rows.Close()

	var convs []*Conversation
	for rows.Next() {
		c := &Conversation{}
		if err := rows.Scan(&c.ID, &c.TaskID, &c.Role, &c.Content, &c.Metadata, &c.TokensUsed, &c.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan conversation: %w", err)
		}
		convs = append(convs, c)
	}
	return convs, nil
}

func (r *Repository) CreateModelCall(ctx context.Context, call *ModelCall) error {
	err := r.db.QueryRow(ctx,
		`INSERT INTO engine.model_calls (tenant_id, task_id, step_type, model, provider, purpose,
		    input_tokens, output_tokens, total_tokens, cost_cents, latency_ms, status, error_code)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
		 RETURNING id, created_at`,
		call.TenantID, call.TaskID, call.StepType, call.Model, call.Provider, call.Purpose,
		call.InputTokens, call.OutputTokens, call.TotalTokens, call.CostCents,
		call.LatencyMs, call.Status, call.ErrorCode,
	).Scan(&call.ID, &call.CreatedAt)
	if err != nil {
		return fmt.Errorf("create model call: %w", err)
	}
	return nil
}

func (r *Repository) CreateReviewResult(ctx context.Context, rr *ReviewResult) error {
	err := r.db.QueryRow(ctx,
		`INSERT INTO engine.review_results (task_id, step_id, review_type, score, passed, findings, summary)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)
		 RETURNING id, created_at`,
		rr.TaskID, rr.StepID, rr.ReviewType, rr.Score, rr.Passed, rr.Findings, rr.Summary,
	).Scan(&rr.ID, &rr.CreatedAt)
	if err != nil {
		return fmt.Errorf("create review result: %w", err)
	}
	return nil
}
