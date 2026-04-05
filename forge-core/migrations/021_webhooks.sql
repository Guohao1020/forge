-- Webhook endpoints for event notifications

CREATE TABLE IF NOT EXISTS engine.webhooks (
    id          BIGSERIAL PRIMARY KEY,
    project_id  BIGINT NOT NULL REFERENCES engine.projects(id),
    tenant_id   BIGINT NOT NULL,
    url         TEXT NOT NULL,
    secret      VARCHAR(255), -- HMAC signing secret
    events      VARCHAR(500) NOT NULL DEFAULT '*', -- comma-separated event types
    active      BOOLEAN NOT NULL DEFAULT true,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_webhooks_project ON engine.webhooks(project_id);
CREATE INDEX IF NOT EXISTS idx_webhooks_active ON engine.webhooks(project_id, active) WHERE active = true;
