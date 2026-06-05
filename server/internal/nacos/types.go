package nacos

// MCPServerShape is a catalog entry WITHOUT secret values. EnvKeys / HeaderKeys
// name the secrets (e.g. "VOC_API_KEY"); the values are injected later from
// Multica (the agent's custom_env), never stored in Nacos.
type MCPServerShape struct {
	Name       string   `json:"name"`
	Version    string   `json:"version"`
	Transport  string   `json:"transport"` // "stdio" | "sse" | "http"
	Command    string   `json:"command,omitempty"`
	Args       []string `json:"args,omitempty"`
	EnvKeys    []string `json:"env_keys,omitempty"`
	URL        string   `json:"url,omitempty"`
	HeaderKeys []string `json:"header_keys,omitempty"`
	Lifecycle  string   `json:"lifecycle"` // "published" | "offline" | "draft"
	Tools      []string `json:"tools,omitempty"`
}

// MCPRef is what an agent stores (agent.mcp_refs element): a reference into the
// Nacos catalog, not a full config. Ref is a version or a tag ("stable"/"latest").
type MCPRef struct {
	Namespace string `json:"namespace"` // workspace id or "shared"
	Name      string `json:"name"`
	Ref       string `json:"ref"`
}
