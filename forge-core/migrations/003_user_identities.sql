-- User external identity bindings (GitHub, Codeup, etc.)
CREATE TABLE IF NOT EXISTS auth.user_identities (
    id            BIGSERIAL PRIMARY KEY,
    user_id       BIGINT NOT NULL REFERENCES auth.users(id),
    provider      VARCHAR(50) NOT NULL,
    provider_uid  VARCHAR(200) NOT NULL,
    access_token  TEXT,
    refresh_token TEXT,
    token_expires TIMESTAMPTZ,
    profile       JSONB DEFAULT '{}',
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(provider, provider_uid)
);

-- Index for fast lookup by user + provider
CREATE INDEX IF NOT EXISTS idx_user_identities_user_provider
    ON auth.user_identities(user_id, provider);
