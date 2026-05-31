-- Forge F1: spec-center Standards (sidecar tables, forge_ prefix).
-- A standard = categorized + profile-tagged coding convention with a mandatory
-- core part (injected into agent instructions) and a detailed part (compiled
-- into an on-demand skill). Scope: workspace-level (project_id NULL) overridden
-- by project-level. Forge-owned; does not touch Multica's project/skill tables.

CREATE TABLE forge_standard (
    id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id   UUID NOT NULL REFERENCES workspace(id) ON DELETE CASCADE,
    project_id     UUID REFERENCES project(id) ON DELETE CASCADE,
    name           TEXT NOT NULL,
    category       TEXT NOT NULL,
    profile_tags   TEXT[] NOT NULL DEFAULT '{}',
    core_content   TEXT NOT NULL DEFAULT '',
    detail_content TEXT NOT NULL DEFAULT '',
    enabled        BOOLEAN NOT NULL DEFAULT TRUE,
    created_by     UUID,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Override key uniqueness: (category,name) unique within each scope.
CREATE UNIQUE INDEX uq_forge_standard_ws
    ON forge_standard (workspace_id, category, name) WHERE project_id IS NULL;
CREATE UNIQUE INDEX uq_forge_standard_proj
    ON forge_standard (project_id, category, name) WHERE project_id IS NOT NULL;
CREATE INDEX idx_forge_standard_ws ON forge_standard (workspace_id);

CREATE TABLE forge_project_profile (
    project_id   UUID PRIMARY KEY REFERENCES project(id) ON DELETE CASCADE,
    tags         TEXT[] NOT NULL DEFAULT '{}',
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);
