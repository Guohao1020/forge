-- 016_project_versions.sql
-- Version management for iterative project development.
-- Each version groups multiple tasks (requirements) that ship together.

CREATE TABLE IF NOT EXISTS engine.project_versions (
    id           BIGSERIAL PRIMARY KEY,
    tenant_id    BIGINT NOT NULL,
    project_id   BIGINT NOT NULL REFERENCES engine.projects(id) ON DELETE CASCADE,
    version      VARCHAR(50) NOT NULL,          -- "1.2.0", "v2.0"
    status       VARCHAR(20) NOT NULL DEFAULT 'PLANNING',
    -- PLANNING:     collecting requirements
    -- IN_PROGRESS:  tasks actively being worked on
    -- TESTING:      all tasks completed, awaiting validation
    -- RELEASED:     git tag created, deployed
    -- CANCELLED:    version abandoned (tasks remain as-is)
    description  TEXT DEFAULT '',
    git_tag      VARCHAR(100),                  -- set on release, e.g. "v1.2.0"
    released_at  TIMESTAMPTZ,
    created_by   BIGINT,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT uq_project_version UNIQUE (project_id, version)
);

CREATE INDEX idx_project_versions_tenant ON engine.project_versions(tenant_id);
CREATE INDEX idx_project_versions_project ON engine.project_versions(project_id);
CREATE INDEX idx_project_versions_status ON engine.project_versions(status);

-- Extend tasks table for version tracking and conflict detection
ALTER TABLE engine.tasks
    ADD COLUMN IF NOT EXISTS version_id       BIGINT REFERENCES engine.project_versions(id),
    ADD COLUMN IF NOT EXISTS conflict_status  VARCHAR(20) DEFAULT 'NONE',
    -- NONE:      no conflict detected
    -- DETECTED:  potential file overlap detected during planning
    -- WAITING:   blocked by another task (files conflict)
    -- RESOLVED:  conflict resolved (blocker completed or manually resolved)
    ADD COLUMN IF NOT EXISTS blocked_by       JSONB DEFAULT '[]',
    -- array of task IDs that block this task
    ADD COLUMN IF NOT EXISTS touched_files    JSONB DEFAULT '[]';
    -- array of file paths this task plans to create/modify (from PlannerAgent)

CREATE INDEX idx_tasks_version ON engine.tasks(version_id) WHERE version_id IS NOT NULL;
CREATE INDEX idx_tasks_conflict ON engine.tasks(conflict_status) WHERE conflict_status != 'NONE';
