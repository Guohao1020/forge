-- Agent Terminal sessions + messages
--
-- Design: Agent sessions are the "chat thread" unit for the interactive
-- Agent Terminal. They're independent of engine.tasks (which are
-- Temporal workflow tasks with their own lifecycle), but a session MAY
-- optionally link to a task when the user wants the conversation to be
-- associated with a specific Temporal workflow run.
--
-- Dual storage pattern: every message is written to Redis Streams (hot
-- buffer for SSE, MAXLEN ~500) AND inserted into PostgreSQL (durable
-- history). Frontend session recovery hydrates from PG first, then
-- subscribes to Redis for new events.

CREATE TABLE IF NOT EXISTS engine.agent_sessions (
    id             UUID PRIMARY KEY,
    tenant_id      BIGINT NOT NULL,
    project_id     BIGINT NOT NULL REFERENCES engine.projects(id) ON DELETE CASCADE,
    task_id        BIGINT REFERENCES engine.tasks(id) ON DELETE SET NULL,
    title          VARCHAR(200),
    created_by     BIGINT NOT NULL REFERENCES auth.users(id),
    created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    archived       BOOLEAN NOT NULL DEFAULT FALSE,
    last_message_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_agent_sessions_project ON engine.agent_sessions(project_id) WHERE archived = FALSE;
CREATE INDEX IF NOT EXISTS idx_agent_sessions_task    ON engine.agent_sessions(task_id)    WHERE task_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_agent_sessions_tenant  ON engine.agent_sessions(tenant_id);
CREATE INDEX IF NOT EXISTS idx_agent_sessions_last_message ON engine.agent_sessions(project_id, last_message_at DESC NULLS LAST);

-- agent_messages stores the durable event log that mirrors Redis Streams.
-- The `event_type` column matches the Python stream_events.py dataclasses
-- (text_delta, turn_complete, tool_started, tool_completed, error,
-- thinking_started, thinking_stopped, fix_loop_started, fix_loop_completed,
-- session_complete). The full event payload lives in `data` as JSONB.
--
-- For chat history rendering, the frontend filters event_type IN
-- ('user_message', 'text_delta', 'turn_complete') and reconstructs the
-- assistant response by concatenating text_deltas into a single message
-- per turn.
CREATE TABLE IF NOT EXISTS engine.agent_messages (
    id             BIGSERIAL PRIMARY KEY,
    session_id     UUID NOT NULL REFERENCES engine.agent_sessions(id) ON DELETE CASCADE,
    redis_id       VARCHAR(40),  -- Redis Stream entry ID for idempotent replay
    event_type     VARCHAR(40) NOT NULL,
    role           VARCHAR(20),  -- user | assistant | system | tool
    content        TEXT,
    tool_name      VARCHAR(100),
    data           JSONB NOT NULL DEFAULT '{}',
    correlation_id VARCHAR(100),
    created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_agent_messages_session_created ON engine.agent_messages(session_id, created_at);
CREATE INDEX IF NOT EXISTS idx_agent_messages_event_type ON engine.agent_messages(event_type);
-- For idempotent writes from the Python worker: if a redis_id is already
-- persisted, skip the insert.
CREATE UNIQUE INDEX IF NOT EXISTS uq_agent_messages_session_redis ON engine.agent_messages(session_id, redis_id) WHERE redis_id IS NOT NULL;

-- Trigger: bump agent_sessions.last_message_at + updated_at when a new
-- message is inserted. Keeps the sidebar list sortable by recent activity
-- without a separate query.
CREATE OR REPLACE FUNCTION engine.touch_agent_session_on_message() RETURNS TRIGGER AS $$
BEGIN
    UPDATE engine.agent_sessions
    SET last_message_at = NEW.created_at,
        updated_at      = NEW.created_at
    WHERE id = NEW.session_id;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_touch_agent_session_on_message ON engine.agent_messages;
CREATE TRIGGER trg_touch_agent_session_on_message
    AFTER INSERT ON engine.agent_messages
    FOR EACH ROW
    EXECUTE FUNCTION engine.touch_agent_session_on_message();
