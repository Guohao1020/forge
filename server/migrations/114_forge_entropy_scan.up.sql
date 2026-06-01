-- Forge F4: entropy scan config (sidecar, forge_ prefix). Defines a periodic
-- whole-repo quality scan for a scope. Forge manages a backing Autopilot
-- (schedule trigger = cron, execution_mode = create_issue, assignee = scanner
-- agent); autopilot_id links to it. The dispatch hook reverse-looks-up this row
-- by autopilot_id to compose the scanner agent's brief (F1 + F2 + custom + dedup).
CREATE TABLE forge_entropy_scan (
    id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id      UUID NOT NULL REFERENCES workspace(id) ON DELETE CASCADE,
    project_id        UUID REFERENCES project(id) ON DELETE CASCADE, -- NULL = workspace-level
    name              TEXT NOT NULL,
    scanner_agent_id  UUID NOT NULL REFERENCES agent(id) ON DELETE CASCADE,
    custom_focus      TEXT NOT NULL DEFAULT '',
    include_standards BOOLEAN NOT NULL DEFAULT TRUE,
    include_checks    BOOLEAN NOT NULL DEFAULT TRUE,
    cron_expression   TEXT NOT NULL,
    timezone          TEXT NOT NULL DEFAULT 'UTC',
    enabled           BOOLEAN NOT NULL DEFAULT TRUE,
    autopilot_id      UUID REFERENCES autopilot(id) ON DELETE SET NULL,
    created_by        UUID,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_forge_entropy_scan_ws ON forge_entropy_scan(workspace_id);
CREATE UNIQUE INDEX idx_forge_entropy_scan_autopilot
    ON forge_entropy_scan(autopilot_id) WHERE autopilot_id IS NOT NULL;
