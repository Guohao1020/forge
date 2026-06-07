-- Forge prometheus (N2): single optional reference into the Nacos LLM-provider
-- catalog. NULL = agent uses no provider (today's static/runtime model path);
-- a JSON object = {namespace,name,ref}. Unlike mcp_refs (a list with '[]'
-- default) this is a single nullable ref, so NULL is the clean "unset" state.
ALTER TABLE agent ADD COLUMN provider_ref JSONB;
