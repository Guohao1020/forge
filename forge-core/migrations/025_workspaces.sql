-- Workspace state machine for the single-agent Variant B architecture.
--
-- One row per (tenant, project). Lifecycle:
--
--   no row → INSERT status='pending' (with pg_advisory_xact_lock)
--   pending → clone + deps prep ok → UPDATE status='ready'
--   pending → clone or prep fails  → UPDATE status='error', last_error
--   error   → next EnsureReady call wipes dir, re-enters 'pending'
--   ready   → subsequent calls may fetch + reset --hard (row stays 'ready')
--
-- Advisory lock protocol:
--   SELECT pg_advisory_xact_lock(hashtext('workspace:' || tenant || ':' || project))
--   within the transaction that reads/writes this row.

CREATE TABLE IF NOT EXISTS engine.workspaces (
    id              BIGSERIAL PRIMARY KEY,
    tenant_id       BIGINT NOT NULL,
    project_id      BIGINT NOT NULL,
    host_path       TEXT NOT NULL,
    container_path  TEXT NOT NULL,
    status          TEXT NOT NULL CHECK (status IN ('pending', 'ready', 'error')),
    last_synced_at  TIMESTAMPTZ,
    last_error      TEXT,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(tenant_id, project_id)
);

CREATE INDEX IF NOT EXISTS idx_workspaces_tenant_project
    ON engine.workspaces(tenant_id, project_id);

CREATE INDEX IF NOT EXISTS idx_workspaces_status_updated
    ON engine.workspaces(status, updated_at DESC);

COMMENT ON TABLE engine.workspaces IS
    'One row per (tenant, project). Owns the clone lifecycle for the single-agent Variant B architecture. Guarded by pg_advisory_xact_lock on hashtext(workspace:tenant:project).';
