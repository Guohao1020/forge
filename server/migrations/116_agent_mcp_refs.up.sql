-- Forge iris (N1): agents reference MCP servers in the Nacos AI Registry catalog
-- by storing lightweight refs (namespace/name/version-or-tag) instead of inlining
-- a full mcp_config. At dispatch time the server resolves these refs into an
-- effective mcp_config (secret values injected from the agent's custom_env; the
-- catalog stores shape only). Defaults to '[]' so every existing agent keeps
-- working off its inline mcp_config until refs are added.
ALTER TABLE agent ADD COLUMN mcp_refs JSONB NOT NULL DEFAULT '[]';
