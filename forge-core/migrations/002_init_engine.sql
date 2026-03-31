-- Projects
CREATE TABLE IF NOT EXISTS engine.projects (
    id               BIGSERIAL PRIMARY KEY,
    tenant_id        BIGINT NOT NULL REFERENCES auth.tenants(id),
    name             VARCHAR(200) NOT NULL,
    description      TEXT,
    status           VARCHAR(20) NOT NULL DEFAULT 'ACTIVE',
    code_platform    VARCHAR(50),
    code_repo_url    TEXT,
    default_branch   VARCHAR(100) DEFAULT 'main',
    ai_model         VARCHAR(50),
    risk_threshold   INT DEFAULT 90,
    auto_merge       BOOLEAN DEFAULT TRUE,
    created_by       BIGINT REFERENCES auth.users(id),
    created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(tenant_id, name)
);

-- Project Stars
CREATE TABLE IF NOT EXISTS engine.project_stars (
    id         BIGSERIAL PRIMARY KEY,
    user_id    BIGINT NOT NULL REFERENCES auth.users(id),
    project_id BIGINT NOT NULL REFERENCES engine.projects(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(user_id, project_id)
);

CREATE INDEX IF NOT EXISTS idx_projects_tenant_id ON engine.projects(tenant_id);
CREATE INDEX IF NOT EXISTS idx_projects_status ON engine.projects(status);
CREATE INDEX IF NOT EXISTS idx_project_stars_user_id ON engine.project_stars(user_id);
