-- S14: Deploy records for K8s deployment framework
CREATE TABLE IF NOT EXISTS pipeline.deploy_records (
    id              BIGSERIAL PRIMARY KEY,
    tenant_id       BIGINT NOT NULL,
    project_id      BIGINT NOT NULL REFERENCES engine.projects(id),
    environment_id  BIGINT NOT NULL REFERENCES pipeline.environments(id),
    artifact_id     BIGINT REFERENCES pipeline.artifacts(id),
    version         VARCHAR(100) NOT NULL,
    status          VARCHAR(20) NOT NULL DEFAULT 'PENDING',  -- PENDING / DEPLOYING / DEPLOYED / FAILED / ROLLED_BACK
    deployed_by     BIGINT NOT NULL,
    started_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    completed_at    TIMESTAMPTZ,
    k8s_manifest    TEXT,
    error_message   TEXT,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_deploy_records_env ON pipeline.deploy_records(environment_id);
CREATE INDEX IF NOT EXISTS idx_deploy_records_project ON pipeline.deploy_records(project_id);
