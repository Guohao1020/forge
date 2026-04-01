-- S6: Conversations + Model Calls + Review Results

CREATE TABLE IF NOT EXISTS engine.conversations (
    id              BIGSERIAL PRIMARY KEY,
    task_id         BIGINT NOT NULL REFERENCES engine.tasks(id),
    role            VARCHAR(20) NOT NULL,
    content         TEXT NOT NULL,
    metadata        JSONB,
    tokens_used     INT,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_conversations_task ON engine.conversations(task_id);

CREATE TABLE IF NOT EXISTS engine.model_calls (
    id              BIGSERIAL PRIMARY KEY,
    tenant_id       BIGINT NOT NULL,
    task_id         BIGINT NOT NULL,
    step_type       VARCHAR(20),
    model           VARCHAR(50) NOT NULL,
    provider        VARCHAR(20) NOT NULL,
    purpose         VARCHAR(20) NOT NULL,
    input_tokens    INT NOT NULL DEFAULT 0,
    output_tokens   INT NOT NULL DEFAULT 0,
    total_tokens    INT NOT NULL DEFAULT 0,
    cost_cents      INT NOT NULL DEFAULT 0,
    latency_ms      INT NOT NULL DEFAULT 0,
    status          VARCHAR(10) NOT NULL,
    error_code      VARCHAR(50),
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_model_calls_tenant ON engine.model_calls(tenant_id);
CREATE INDEX IF NOT EXISTS idx_model_calls_task ON engine.model_calls(task_id);

CREATE TABLE IF NOT EXISTS engine.review_results (
    id              BIGSERIAL PRIMARY KEY,
    task_id         BIGINT NOT NULL REFERENCES engine.tasks(id),
    step_id         BIGINT,
    review_type     VARCHAR(20) NOT NULL,
    score           INT,
    passed          BOOLEAN NOT NULL,
    findings        JSONB NOT NULL DEFAULT '[]',
    summary         TEXT,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_review_results_task ON engine.review_results(task_id);

ALTER TABLE engine.tasks ADD COLUMN IF NOT EXISTS analysis JSONB;
ALTER TABLE engine.tasks ADD COLUMN IF NOT EXISTS task_graph JSONB;
ALTER TABLE engine.tasks ADD COLUMN IF NOT EXISTS risk_factors JSONB;
