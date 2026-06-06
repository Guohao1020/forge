package providerresolve

import (
	"fmt"

	"github.com/multica-ai/multica/server/internal/nacos"
)

// codexRouterMapper → Codex custom_args (-c model_providers.<name>.*) + the key
// env var (codex reads the key via env_key). argv tokens are SEPARATE slice
// elements (each "-c" and its "key=value" are distinct args) so the daemon
// forwards them verbatim to the CLI argv.
type codexRouterMapper struct{}

func (codexRouterMapper) Protocol() string { return "codex-router" }

func (codexRouterMapper) Map(s nacos.ProviderShape, secrets SecretSource, _ string) (map[string]string, []string, []string) {
	var warns []string
	wire := s.WireAPI
	if wire == "" {
		wire = "responses"
	}
	args := []string{
		"-c", "model_provider=" + s.Name,
		"-c", "model_providers." + s.Name + ".base_url=" + s.BaseURL,
		"-c", "model_providers." + s.Name + ".wire_api=" + wire,
		"-c", "model_providers." + s.Name + ".env_key=" + s.AuthKey,
	}
	v, ok := secrets.Get(s.AuthKey)
	if !ok {
		warns = append(warns, fmt.Sprintf("provider %s: secret %q missing from agent env", s.Name, s.AuthKey))
	}
	env := map[string]string{s.AuthKey: v}
	return env, args, warns
}
