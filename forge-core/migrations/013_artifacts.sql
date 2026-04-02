-- S13: Artifact management
CREATE TABLE IF NOT EXISTS pipeline.artifacts (
    id              BIGSERIAL PRIMARY KEY,
    tenant_id       BIGINT NOT NULL,
    project_id      BIGINT NOT NULL REFERENCES engine.projects(id),
    task_id         BIGINT REFERENCES engine.tasks(id),
    name            VARCHAR(200) NOT NULL,
    version         VARCHAR(100) NOT NULL,
    artifact_type   VARCHAR(20) NOT NULL,  -- DOCKER_IMAGE / JAR / BINARY / ARCHIVE
    registry_url    TEXT,                  -- e.g., registry.cn-hangzhou.aliyuncs.com/forge/app:v1.0.0
    size_bytes      BIGINT,
    checksum        VARCHAR(64),           -- SHA256
    metadata        JSONB NOT NULL DEFAULT '{}',
    status          VARCHAR(20) NOT NULL DEFAULT 'BUILDING',  -- BUILDING / READY / FAILED
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_artifacts_project ON pipeline.artifacts(project_id);
CREATE INDEX IF NOT EXISTS idx_artifacts_task ON pipeline.artifacts(task_id);
