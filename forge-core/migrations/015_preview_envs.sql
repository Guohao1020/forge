CREATE TABLE IF NOT EXISTS pipeline.preview_environments (
    id              BIGSERIAL PRIMARY KEY,
    tenant_id       BIGINT NOT NULL,
    project_id      BIGINT NOT NULL REFERENCES engine.projects(id),
    task_id         BIGINT REFERENCES engine.tasks(id),
    branch_name     VARCHAR(200),
    pr_number       INT,
    preview_url     TEXT,
    status          VARCHAR(20) NOT NULL DEFAULT 'CREATING',  -- CREATING / READY / ERROR / DESTROYING / DESTROYED
    namespace       VARCHAR(100),  -- K8s namespace (mock for now)
    expires_at      TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_preview_envs_project ON pipeline.preview_environments(project_id);
CREATE INDEX IF NOT EXISTS idx_preview_envs_task ON pipeline.preview_environments(task_id);
