-- Entropy Management: quality scan results and trend tracking

CREATE TABLE IF NOT EXISTS engine.entropy_scans (
    id            BIGSERIAL PRIMARY KEY,
    project_id    BIGINT NOT NULL REFERENCES engine.projects(id),
    tenant_id     BIGINT NOT NULL,
    score         INT NOT NULL DEFAULT 100, -- 0-100 quality score
    issue_count   INT NOT NULL DEFAULT 0,
    issues        JSONB NOT NULL DEFAULT '[]',
    scanned_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_entropy_scans_project ON engine.entropy_scans(project_id);
CREATE INDEX IF NOT EXISTS idx_entropy_scans_tenant ON engine.entropy_scans(tenant_id);
CREATE INDEX IF NOT EXISTS idx_entropy_scans_time ON engine.entropy_scans(scanned_at DESC);

-- Entropy scan configuration per project
CREATE TABLE IF NOT EXISTS engine.entropy_configs (
    id            BIGSERIAL PRIMARY KEY,
    project_id    BIGINT NOT NULL REFERENCES engine.projects(id) UNIQUE,
    tenant_id     BIGINT NOT NULL,
    enabled       BOOLEAN NOT NULL DEFAULT true,
    schedule      VARCHAR(50) NOT NULL DEFAULT 'weekly', -- daily, weekly, monthly
    auto_fix      BOOLEAN NOT NULL DEFAULT false,
    rules         JSONB NOT NULL DEFAULT '[]', -- which rules to enforce
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
