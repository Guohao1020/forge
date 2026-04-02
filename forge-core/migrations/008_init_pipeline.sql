-- S7: Pipeline schema + Task PR fields
CREATE SCHEMA IF NOT EXISTS pipeline;

-- pipeline.environments — deployment environment tracking
CREATE TABLE IF NOT EXISTS pipeline.environments (
    id              BIGSERIAL PRIMARY KEY,
    tenant_id       BIGINT NOT NULL,
    project_id      BIGINT NOT NULL REFERENCES engine.projects(id),
    name            VARCHAR(100) NOT NULL,
    env_type        VARCHAR(20) NOT NULL,  -- DEV / STAGING / PROD
    status          VARCHAR(20) NOT NULL DEFAULT 'INACTIVE',
    current_version VARCHAR(100),
    config          JSONB NOT NULL DEFAULT '{}',
    last_deploy_at  TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_environments_project ON pipeline.environments(project_id);
CREATE UNIQUE INDEX IF NOT EXISTS idx_environments_project_type ON pipeline.environments(project_id, env_type);

-- Task PR fields
ALTER TABLE engine.tasks ADD COLUMN IF NOT EXISTS pr_number INT;
ALTER TABLE engine.tasks ADD COLUMN IF NOT EXISTS mr_url TEXT;
ALTER TABLE engine.tasks ADD COLUMN IF NOT EXISTS review_score INT;

-- Trigger: auto-create default environments on project creation
CREATE OR REPLACE FUNCTION pipeline.create_default_environments()
RETURNS TRIGGER AS $$
BEGIN
    INSERT INTO pipeline.environments (tenant_id, project_id, name, env_type, status)
    VALUES
        (NEW.tenant_id, NEW.id, 'Development', 'DEV', 'INACTIVE'),
        (NEW.tenant_id, NEW.id, 'Staging', 'STAGING', 'INACTIVE'),
        (NEW.tenant_id, NEW.id, 'Production', 'PROD', 'INACTIVE');
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_create_default_environments ON engine.projects;
CREATE TRIGGER trg_create_default_environments
    AFTER INSERT ON engine.projects
    FOR EACH ROW
    EXECUTE FUNCTION pipeline.create_default_environments();

-- Backfill existing projects
INSERT INTO pipeline.environments (tenant_id, project_id, name, env_type, status)
SELECT p.tenant_id, p.id, 'Development', 'DEV', 'INACTIVE'
FROM engine.projects p
WHERE NOT EXISTS (SELECT 1 FROM pipeline.environments e WHERE e.project_id = p.id AND e.env_type = 'DEV');

INSERT INTO pipeline.environments (tenant_id, project_id, name, env_type, status)
SELECT p.tenant_id, p.id, 'Staging', 'STAGING', 'INACTIVE'
FROM engine.projects p
WHERE NOT EXISTS (SELECT 1 FROM pipeline.environments e WHERE e.project_id = p.id AND e.env_type = 'STAGING');

INSERT INTO pipeline.environments (tenant_id, project_id, name, env_type, status)
SELECT p.tenant_id, p.id, 'Production', 'PROD', 'INACTIVE'
FROM engine.projects p
WHERE NOT EXISTS (SELECT 1 FROM pipeline.environments e WHERE e.project_id = p.id AND e.env_type = 'PROD');
