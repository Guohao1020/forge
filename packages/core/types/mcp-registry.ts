// Forge iris (N1): the MCP server catalog, backed server-side by the Nacos AI
// Registry. The catalog stores SHAPE ONLY — env_keys / header_keys name the
// secrets an agent must supply via its custom_env; secret VALUES never live in
// the catalog or cross this type.
//
// transport and lifecycle are typed as `string` (not narrow unions) on purpose:
// they're server-driven and the zod schema parses them leniently so a new
// backend enum value downgrades to a generic render instead of crashing the UI
// (CLAUDE.md "enum drift downgrades, not crashes").
export interface MCPServerShape {
  name: string;
  version: string;
  transport: string; // "stdio" | "sse" | "http"
  command?: string;
  args?: string[];
  env_keys?: string[];
  url?: string;
  header_keys?: string[];
  lifecycle: string; // "published" | "offline" | "draft"
  tools?: string[];
}

// MCPRef is what an agent stores (agent.mcp_refs element): a pointer into the
// catalog. ref is either a concrete version or a moving tag ("stable"/"latest").
export interface MCPRef {
  namespace: string; // workspace id or "shared"
  name: string;
  ref: string;
}

export interface MCPServerList {
  servers: MCPServerShape[];
}
