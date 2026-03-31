-- Tenants
CREATE TABLE IF NOT EXISTS auth.tenants (
    id            BIGSERIAL PRIMARY KEY,
    name          VARCHAR(100) NOT NULL,
    code          VARCHAR(50) NOT NULL UNIQUE,
    status        VARCHAR(20) NOT NULL DEFAULT 'ACTIVE',
    plan          VARCHAR(20) NOT NULL DEFAULT 'FREE',
    config        JSONB NOT NULL DEFAULT '{}',
    token_budget  BIGINT DEFAULT 0,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Users
CREATE TABLE IF NOT EXISTS auth.users (
    id            BIGSERIAL PRIMARY KEY,
    tenant_id     BIGINT NOT NULL REFERENCES auth.tenants(id),
    username      VARCHAR(100) NOT NULL,
    email         VARCHAR(200),
    password_hash VARCHAR(255),
    display_name  VARCHAR(100),
    avatar_url    TEXT,
    status        VARCHAR(20) NOT NULL DEFAULT 'ACTIVE',
    last_login_at TIMESTAMPTZ,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(tenant_id, username)
);

-- Roles
CREATE TABLE IF NOT EXISTS auth.roles (
    id            BIGSERIAL PRIMARY KEY,
    tenant_id     BIGINT NOT NULL REFERENCES auth.tenants(id),
    name          VARCHAR(100) NOT NULL,
    code          VARCHAR(50) NOT NULL,
    scope         VARCHAR(20) NOT NULL DEFAULT 'PLATFORM',
    description   TEXT,
    is_system     BOOLEAN NOT NULL DEFAULT FALSE,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(tenant_id, code)
);

-- User-Role mapping
CREATE TABLE IF NOT EXISTS auth.user_roles (
    id            BIGSERIAL PRIMARY KEY,
    user_id       BIGINT NOT NULL REFERENCES auth.users(id),
    role_id       BIGINT NOT NULL REFERENCES auth.roles(id),
    scope         VARCHAR(20) NOT NULL DEFAULT 'PLATFORM',
    scope_id      BIGINT NOT NULL DEFAULT 0,
    granted_by    BIGINT REFERENCES auth.users(id),
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(user_id, role_id, scope, scope_id)
);

-- Active tokens
CREATE TABLE IF NOT EXISTS auth.active_tokens (
    id            BIGSERIAL PRIMARY KEY,
    tenant_id     BIGINT NOT NULL REFERENCES auth.tenants(id),
    user_id       BIGINT NOT NULL REFERENCES auth.users(id),
    token_jti     VARCHAR(100) NOT NULL UNIQUE,
    token_type    VARCHAR(20) NOT NULL DEFAULT 'SESSION',
    device_info   VARCHAR(200),
    ip_address    INET,
    expires_at    TIMESTAMPTZ NOT NULL,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Seed: default tenant
INSERT INTO auth.tenants (name, code) VALUES ('Default', 'default')
ON CONFLICT (code) DO NOTHING;

-- Seed: admin user (password: admin123)
INSERT INTO auth.users (tenant_id, username, password_hash, display_name, status)
VALUES (
    (SELECT id FROM auth.tenants WHERE code = 'default'),
    'admin',
    '$2a$10$eV5/MA37clbZQbclWPR2HuvlzkvUyWAkb3oXEGPVm9Wocj7Claeym',
    'Administrator',
    'ACTIVE'
) ON CONFLICT (tenant_id, username) DO NOTHING;

-- Seed: system roles
INSERT INTO auth.roles (tenant_id, name, code, scope, is_system) VALUES
    ((SELECT id FROM auth.tenants WHERE code = 'default'), '平台管理员', 'PLATFORM_ADMIN', 'PLATFORM', TRUE),
    ((SELECT id FROM auth.tenants WHERE code = 'default'), '技术管理者', 'TECH_LEAD', 'PROJECT', TRUE),
    ((SELECT id FROM auth.tenants WHERE code = 'default'), '产品经理', 'PM', 'PROJECT', TRUE)
ON CONFLICT (tenant_id, code) DO NOTHING;

-- Seed: assign admin role
INSERT INTO auth.user_roles (user_id, role_id, scope)
SELECT u.id, r.id, 'PLATFORM'
FROM auth.users u, auth.roles r
WHERE u.username = 'admin' AND r.code = 'PLATFORM_ADMIN'
ON CONFLICT (user_id, role_id, scope, scope_id) DO NOTHING;
