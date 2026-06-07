package providerresolve

import (
	"fmt"

	"github.com/multica-ai/multica/server/internal/nacos"
)

// anthropicMapper → Claude env. Mirrors what live execution wired by hand:
// ANTHROPIC_BASE_URL / ANTHROPIC_AUTH_TOKEN / ANTHROPIC_MODEL.
type anthropicMapper struct{}

func (anthropicMapper) Protocol() string { return "anthropic" }

func (anthropicMapper) Map(s nacos.ProviderShape, secrets SecretSource, model string) (map[string]string, []string, []string) {
	var warns []string
	env := map[string]string{"ANTHROPIC_BASE_URL": s.BaseURL}
	v, ok := secrets.Get(s.AuthKey)
	if !ok {
		warns = append(warns, fmt.Sprintf("provider %s: secret %q missing from agent env", s.Name, s.AuthKey))
	}
	env["ANTHROPIC_AUTH_TOKEN"] = v
	if model != "" {
		env["ANTHROPIC_MODEL"] = model
	}
	return env, nil, warns
}
