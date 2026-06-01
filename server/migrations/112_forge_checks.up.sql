-- Forge F2: verification gate checks (sidecar, forge_ prefix).
-- A check = name + shell command run in the task workdir after the agent
-- session ends. Non-zero exit = failure → task blocked + comment. Scope:
-- workspace-level (project_id NULL) + project-level (additive — all run).
CREATE TABLE forge_check (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL REFERENCES workspace(id) ON DELETE CASCADE,
    project_id   UUID REFERENCES project(id) ON DELETE CASCADE,
    name         TEXT NOT NULL,
    command      TEXT NOT NULL,
    enabled      BOOLEAN NOT NULL DEFAULT TRUE,
    created_by   UUID,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX uq_forge_check_ws
    ON forge_check (workspace_id, name) WHERE project_id IS NULL;
CREATE UNIQUE INDEX uq_forge_check_proj
    ON forge_check (project_id, name) WHERE project_id IS NOT NULL;
CREATE INDEX idx_forge_check_ws ON forge_check (workspace_id);
