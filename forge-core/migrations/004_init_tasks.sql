-- Task status lifecycle:
-- SUBMITTED → ANALYZING → PLANNING → GENERATING → REVIEWING → TESTING → DEPLOYING → COMPLETED
-- Any step can transition to FAILED

-- Tasks table
CREATE TABLE IF NOT EXISTS engine.tasks (
    id              BIGSERIAL PRIMARY KEY,
    tenant_id       BIGINT NOT NULL,
    project_id      BIGINT NOT NULL REFERENCES engine.projects(id),
    title           VARCHAR(500),
    requirement     TEXT NOT NULL,
    source          VARCHAR(20) NOT NULL DEFAULT 'WEB',
    status          VARCHAR(30) NOT NULL DEFAULT 'SUBMITTED',
    workflow_id     VARCHAR(200),
    workflow_run_id VARCHAR(200),
    risk_level      VARCHAR(10),
    risk_score      INT,
    branch_name     VARCHAR(200),
    files_changed   INT,
    lines_added     INT,
    lines_deleted   INT,
    created_by      BIGINT NOT NULL REFERENCES auth.users(id),
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    completed_at    TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_tasks_project_id ON engine.tasks(project_id);
CREATE INDEX IF NOT EXISTS idx_tasks_tenant_id ON engine.tasks(tenant_id);
CREATE INDEX IF NOT EXISTS idx_tasks_status ON engine.tasks(status);
CREATE INDEX IF NOT EXISTS idx_tasks_workflow_id ON engine.tasks(workflow_id);

-- Task steps table
CREATE TABLE IF NOT EXISTS engine.task_steps (
    id              BIGSERIAL PRIMARY KEY,
    task_id         BIGINT NOT NULL REFERENCES engine.tasks(id) ON DELETE CASCADE,
    name            VARCHAR(200) NOT NULL,
    step_type       VARCHAR(30) NOT NULL,
    status          VARCHAR(20) NOT NULL DEFAULT 'PENDING',
    input           JSONB,
    output          JSONB,
    error           JSONB,
    attempt         INT NOT NULL DEFAULT 1,
    max_attempts    INT NOT NULL DEFAULT 3,
    started_at      TIMESTAMPTZ,
    completed_at    TIMESTAMPTZ,
    duration_ms     BIGINT
);

CREATE INDEX IF NOT EXISTS idx_task_steps_task_id ON engine.task_steps(task_id);
CREATE INDEX IF NOT EXISTS idx_task_steps_status ON engine.task_steps(status);
