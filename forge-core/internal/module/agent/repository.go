package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Repository is the durable storage layer for agent terminal sessions
// and the dual-written message log. Redis Streams remain the primary
// hot path for SSE delivery; this repository backs the PostgreSQL half
// of the dual-storage pattern.
type Repository struct {
	db *pgxpool.Pool
}

// NewRepository constructs an agent Repository.
func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

// CreateSession inserts a new agent session and returns the populated row.
// The caller must generate the UUID up front so it can be reused as the
// Redis Stream key without a round-trip.
func (r *Repository) CreateSession(
	ctx context.Context,
	id string,
	tenantID int64,
	projectID int64,
	createdBy int64,
	title *string,
	taskID *int64,
) (*AgentSession, error) {
	// Validate the caller-supplied UUID to fail fast on bad IDs.
	if _, err := uuid.Parse(id); err != nil {
		return nil, fmt.Errorf("invalid session id: %w", err)
	}

	var s AgentSession
	err := r.db.QueryRow(ctx, `
		INSERT INTO engine.agent_sessions
			(id, tenant_id, project_id, task_id, title, created_by)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, tenant_id, project_id, task_id, title, created_by,
		          created_at, updated_at, archived, last_message_at
	`, id, tenantID, projectID, taskID, title, createdBy).Scan(
		&s.ID, &s.TenantID, &s.ProjectID, &s.TaskID, &s.Title, &s.CreatedBy,
		&s.CreatedAt, &s.UpdatedAt, &s.Archived, &s.LastMessageAt,
	)
	if err != nil {
		return nil, fmt.Errorf("create agent session: %w", err)
	}
	return &s, nil
}

// GetSession fetches a single session by id, scoped to the project.
func (r *Repository) GetSession(
	ctx context.Context,
	sessionID string,
	projectID int64,
) (*AgentSession, error) {
	var s AgentSession
	err := r.db.QueryRow(ctx, `
		SELECT id, tenant_id, project_id, task_id, title, created_by,
		       created_at, updated_at, archived, last_message_at
		FROM engine.agent_sessions
		WHERE id = $1 AND project_id = $2
	`, sessionID, projectID).Scan(
		&s.ID, &s.TenantID, &s.ProjectID, &s.TaskID, &s.Title, &s.CreatedBy,
		&s.CreatedAt, &s.UpdatedAt, &s.Archived, &s.LastMessageAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get agent session: %w", err)
	}
	return &s, nil
}

// ListSessions returns the non-archived agent sessions for a project,
// sorted by recent activity. Used by the TaskSwitcher sidebar.
func (r *Repository) ListSessions(
	ctx context.Context,
	projectID int64,
	limit int,
) ([]AgentSession, int, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}

	var total int
	if err := r.db.QueryRow(ctx, `
		SELECT COUNT(*)
		FROM engine.agent_sessions
		WHERE project_id = $1 AND archived = FALSE
	`, projectID).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count agent sessions: %w", err)
	}

	rows, err := r.db.Query(ctx, `
		SELECT id, tenant_id, project_id, task_id, title, created_by,
		       created_at, updated_at, archived, last_message_at
		FROM engine.agent_sessions
		WHERE project_id = $1 AND archived = FALSE
		ORDER BY COALESCE(last_message_at, created_at) DESC
		LIMIT $2
	`, projectID, limit)
	if err != nil {
		return nil, 0, fmt.Errorf("list agent sessions: %w", err)
	}
	defer rows.Close()

	out := make([]AgentSession, 0, limit)
	for rows.Next() {
		var s AgentSession
		if err := rows.Scan(
			&s.ID, &s.TenantID, &s.ProjectID, &s.TaskID, &s.Title, &s.CreatedBy,
			&s.CreatedAt, &s.UpdatedAt, &s.Archived, &s.LastMessageAt,
		); err != nil {
			return nil, 0, fmt.Errorf("scan agent session: %w", err)
		}
		out = append(out, s)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	return out, total, nil
}

// ArchiveSession flips the archived flag on a session. Soft delete.
func (r *Repository) ArchiveSession(
	ctx context.Context,
	sessionID string,
	projectID int64,
) error {
	ct, err := r.db.Exec(ctx, `
		UPDATE engine.agent_sessions
		SET archived = TRUE, updated_at = NOW()
		WHERE id = $1 AND project_id = $2
	`, sessionID, projectID)
	if err != nil {
		return fmt.Errorf("archive agent session: %w", err)
	}
	if ct.RowsAffected() == 0 {
		return fmt.Errorf("session not found")
	}
	return nil
}

// UpdateSessionTitle sets a user-visible title on an existing session.
func (r *Repository) UpdateSessionTitle(
	ctx context.Context,
	sessionID string,
	projectID int64,
	title string,
) error {
	ct, err := r.db.Exec(ctx, `
		UPDATE engine.agent_sessions
		SET title = $1, updated_at = NOW()
		WHERE id = $2 AND project_id = $3
	`, title, sessionID, projectID)
	if err != nil {
		return fmt.Errorf("update agent session title: %w", err)
	}
	if ct.RowsAffected() == 0 {
		return fmt.Errorf("session not found")
	}
	return nil
}

// InsertMessage inserts one agent_messages row. Used by the ai-worker
// dual-write path (Redis XADD + PG insert) and by any backfill job. The
// unique index on (session_id, redis_id) makes this idempotent when the
// same redis_id is replayed.
func (r *Repository) InsertMessage(
	ctx context.Context,
	m *AgentMessage,
) error {
	if len(m.Data) == 0 {
		m.Data = []byte("{}")
	}
	if !json.Valid(m.Data) {
		return fmt.Errorf("message data is not valid JSON")
	}

	_, err := r.db.Exec(ctx, `
		INSERT INTO engine.agent_messages
			(session_id, redis_id, event_type, role, content, tool_name, data, correlation_id)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		ON CONFLICT (session_id, redis_id) WHERE redis_id IS NOT NULL DO NOTHING
	`,
		m.SessionID, m.RedisID, m.EventType, m.Role, m.Content, m.ToolName, m.Data, m.CorrelationID,
	)
	if err != nil {
		return fmt.Errorf("insert agent message: %w", err)
	}
	return nil
}

// GetProjectTechStack returns the raw JSONB tech_stack blob for a
// project, used by the suggestions heuristic to derive language-
// appropriate starter prompts. Returns an empty payload on any error
// so the caller can fall back to defaults.
func (r *Repository) GetProjectTechStack(
	ctx context.Context,
	projectID int64,
) ([]byte, error) {
	var raw []byte
	err := r.db.QueryRow(ctx, `
		SELECT COALESCE(tech_stack, '{}'::jsonb)
		FROM engine.projects
		WHERE id = $1
	`, projectID).Scan(&raw)
	if errors.Is(err, pgx.ErrNoRows) {
		return []byte("{}"), nil
	}
	if err != nil {
		return nil, fmt.Errorf("get project tech_stack: %w", err)
	}
	return raw, nil
}

// ListMessages returns durable messages for a session in chronological
// order. The frontend uses this to hydrate the chat on page load before
// subscribing to the Redis Stream for new events.
func (r *Repository) ListMessages(
	ctx context.Context,
	sessionID string,
	limit int,
) ([]AgentMessage, error) {
	if limit <= 0 || limit > 1000 {
		limit = 500
	}

	rows, err := r.db.Query(ctx, `
		SELECT id, session_id, redis_id, event_type, role, content,
		       tool_name, data, correlation_id, created_at
		FROM engine.agent_messages
		WHERE session_id = $1
		ORDER BY created_at ASC, id ASC
		LIMIT $2
	`, sessionID, limit)
	if err != nil {
		return nil, fmt.Errorf("list agent messages: %w", err)
	}
	defer rows.Close()

	out := make([]AgentMessage, 0)
	for rows.Next() {
		var m AgentMessage
		if err := rows.Scan(
			&m.ID, &m.SessionID, &m.RedisID, &m.EventType, &m.Role, &m.Content,
			&m.ToolName, &m.Data, &m.CorrelationID, &m.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan agent message: %w", err)
		}
		out = append(out, m)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}
