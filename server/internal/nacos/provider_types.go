package nacos

// ProviderShape is an LLM-provider catalog entry WITHOUT secret values.
// AuthKey names the secret (e.g. "ROUTER_API_KEY"); the value is injected later
// from Multica (agent.custom_env), never stored in Nacos. BaseURL is an
// endpoint, not a secret. Namespace is tagged on list (workspace id / "shared").
type ProviderShape struct {
	Name      string          `json:"name"`
	Namespace string          `json:"namespace,omitempty"`
	Version   string          `json:"version"`
	Protocol  string          `json:"protocol"` // "anthropic" | "codex-router"
	BaseURL   string          `json:"base_url"`
	AuthKey   string          `json:"auth_key"`
	WireAPI   string          `json:"wire_api,omitempty"` // codex; default "responses"
	Models    []ProviderModel `json:"models,omitempty"`
	Lifecycle string          `json:"lifecycle"` // "published" | "offline" | "draft"
}

type ProviderModel struct {
	ID      string `json:"id"`
	Label   string `json:"label,omitempty"`
	Default bool   `json:"default,omitempty"`
}

// ProviderRef is what an agent stores (agent.provider_ref). Single, optional.
type ProviderRef struct {
	Namespace string `json:"namespace"`
	Name      string `json:"name"`
	Ref       string `json:"ref"` // version or tag ("stable"/"latest")
}
