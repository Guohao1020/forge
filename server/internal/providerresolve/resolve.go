// Package providerresolve resolves an agent's provider_ref (a reference into the
// Nacos LLM-provider catalog) into the effective env vars + CLI args passed to
// the daemon/runtime. Pure logic (service-free), tested with a fake
// ProviderQuerier — no live Nacos needed. Mirrors internal/mcpresolve.
package providerresolve

import (
	"context"
	"fmt"

	"github.com/multica-ai/multica/server/internal/nacos"
)

// SecretSource resolves a secret KEY (named in the provider shape) to its value.
// Backed by agent.custom_env. Never comes from Nacos.
type SecretSource interface{ Get(key string) (string, bool) }

type MapSecrets map[string]string

func (m MapSecrets) Get(k string) (string, bool) { v, ok := m[k]; return v, ok }

// Mapper translates a provider shape into the env + args a given runtime needs.
type Mapper interface {
	Protocol() string
	Map(shape nacos.ProviderShape, secrets SecretSource, model string) (env map[string]string, args []string, warnings []string)
}

type Input struct {
	Ref     *nacos.ProviderRef // agent.provider_ref (nil = none)
	Secrets SecretSource       // agent.custom_env
	Model   string             // agent.model
}

type Result struct {
	Env   map[string]string
	Args  []string
	Model string // resolved default when in.Model was empty
}

var mappers = map[string]Mapper{}

func register(m Mapper) { mappers[m.Protocol()] = m }

func init() {
	register(anthropicMapper{})
	register(codexRouterMapper{})
}

// Resolve looks up the provider shape and runs the protocol's mapper. Best-effort:
// a missing/offline provider or unknown protocol is skipped with a warning and an
// empty Result (caller keeps the agent's existing env/args). Never errors on a
// bad ref — only a programming error returns err.
func Resolve(ctx context.Context, q nacos.ProviderQuerier, in Input) (Result, []string, error) {
	if in.Ref == nil {
		return Result{}, nil, nil
	}
	shape, err := q.GetProvider(ctx, in.Ref.Namespace, in.Ref.Name, in.Ref.Ref)
	if err != nil {
		return Result{}, []string{fmt.Sprintf("provider %s/%s@%s skipped: %v", in.Ref.Namespace, in.Ref.Name, in.Ref.Ref, err)}, nil
	}
	if shape.Lifecycle != "published" {
		return Result{}, []string{fmt.Sprintf("provider %s@%s is %q; skipped", shape.Name, shape.Version, shape.Lifecycle)}, nil
	}
	m, ok := mappers[shape.Protocol]
	if !ok {
		return Result{}, []string{fmt.Sprintf("provider %s: unknown protocol %q; skipped", shape.Name, shape.Protocol)}, nil
	}
	model := in.Model
	if model == "" {
		model = defaultModel(shape)
	}
	env, args, warns := m.Map(shape, in.Secrets, model)
	return Result{Env: env, Args: args, Model: model}, warns, nil
}

func defaultModel(s nacos.ProviderShape) string {
	for _, mm := range s.Models {
		if mm.Default {
			return mm.ID
		}
	}
	return ""
}
