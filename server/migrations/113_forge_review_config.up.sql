-- Forge F3: AI review config (sidecar, forge_ prefix). Designates the reviewer
-- agent for a scope. After a coding task completes, Forge enqueues a review
-- task for this agent against the same issue, reusing the coder's workdir.
CREATE TABLE forge_review_config (
    id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id      UUID NOT NULL REFERENCES workspace(id) ON DELETE CASCADE,
    project_id        UUID REFERENCES project(id) ON DELETE CASCADE,
    reviewer_agent_id UUID NOT NULL REFERENCES agent(id) ON DELETE CASCADE,
    enabled           BOOLEAN NOT NULL DEFAULT TRUE,
    created_by        UUID,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX uq_forge_review_ws   ON forge_review_config (workspace_id) WHERE project_id IS NULL;
CREATE UNIQUE INDEX uq_forge_review_proj ON forge_review_config (project_id)   WHERE project_id IS NOT NULL;
