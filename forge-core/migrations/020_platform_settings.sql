-- Platform settings: key-value configuration per tenant

CREATE TABLE IF NOT EXISTS engine.platform_settings (
    id          BIGSERIAL PRIMARY KEY,
    tenant_id   BIGINT NOT NULL,
    key         VARCHAR(100) NOT NULL,
    value       TEXT NOT NULL DEFAULT '',
    category    VARCHAR(50) NOT NULL DEFAULT 'general',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (tenant_id, key)
);

CREATE INDEX IF NOT EXISTS idx_platform_settings_tenant ON engine.platform_settings(tenant_id);
