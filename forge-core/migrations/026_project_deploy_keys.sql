-- Per-project SSH deploy keys for git clone auth in the single-agent
-- architecture. One keypair per project. Public key is uploaded to
-- GitHub via POST /repos/{owner}/{repo}/keys (read_only=false for
-- forward compat with future git push from agent). Private key is
-- AES-GCM encrypted with a key derived from FORGE_SECRETS_MASTER_KEY.
--
-- Storage format of private_key_enc:
--   nonce(12 bytes) || ciphertext || tag(16 bytes)
-- See forge-core/internal/secrets/crypto.go for Encrypt/Decrypt.
--
-- github_key_id is the ID returned by GitHub's deploy-key API, used
-- for future rotation (delete old -> upload new).

CREATE TABLE IF NOT EXISTS engine.project_deploy_keys (
    project_id       BIGINT PRIMARY KEY,
    tenant_id        BIGINT NOT NULL,
    public_key       TEXT NOT NULL,
    private_key_enc  BYTEA NOT NULL,
    key_type         TEXT NOT NULL DEFAULT 'ed25519',
    github_key_id    BIGINT,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_project_deploy_keys_tenant
    ON engine.project_deploy_keys(tenant_id);

COMMENT ON TABLE engine.project_deploy_keys IS
    'Per-project SSH deploy keys. Private key is AES-GCM encrypted with key derived from FORGE_SECRETS_MASTER_KEY via HKDF.';

COMMENT ON COLUMN engine.project_deploy_keys.private_key_enc IS
    'Storage format: nonce(12) || ciphertext || tag(16). See forge-core/internal/secrets/crypto.go.';
