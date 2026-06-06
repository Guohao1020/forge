// Package mcpresolve resolves an agent's mcp_refs (references into the Nacos
// catalog) into the effective mcp_config passed to the daemon/CLI. Pure logic
// (service-free), tested with a fake NacosQuerier — no live Nacos needed.
// Mirrors internal/forge/checks and internal/forge/standards.
package mcpresolve

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/multica-ai/multica/server/internal/nacos"
)

// SecretSource resolves a secret KEY (named in the catalog shape) to its value.
// Backed by the agent's custom_env. Never comes from Nacos.
type SecretSource interface{ Get(key string) (string, bool) }

// MapSecrets is a map-backed SecretSource.
type MapSecrets map[string]string

func (m MapSecrets) Get(k string) (string, bool) { v, ok := m[k]; return v, ok }

// Input carries everything ResolveMCP needs from an agent.
type Input struct {
	Refs      []nacos.MCPRef  // from agent.mcp_refs
	InlineMCP json.RawMessage // existing agent.mcp_config: {"mcpServers": {...}}
	Secrets   SecretSource    // from agent.custom_env
}

// ResolveMCP builds the effective mcp_config from catalog refs + inline config.
// Best-effort: a bad ref is skipped with a warning, never fails the whole
// resolve. Returns the JSON, any warnings, and a hard error only on marshal
// failure.
func ResolveMCP(ctx context.Context, q nacos.NacosQuerier, in Input) (json.RawMessage, []string, error) {
	servers := map[string]json.RawMessage{}
	var warnings []string

	for _, ref := range in.Refs {
		shape, err := q.GetMCPServer(ctx, ref.Namespace, ref.Name, ref.Ref)
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("mcp %s/%s@%s skipped: %v", ref.Namespace, ref.Name, ref.Ref, err))
			continue
		}
		if shape.Lifecycle != "published" {
			warnings = append(warnings, fmt.Sprintf("mcp %s@%s is %q; skipped", shape.Name, shape.Version, shape.Lifecycle))
			continue
		}
		entry, w := buildEntry(shape, in.Secrets)
		warnings = append(warnings, w...)
		servers[shape.Name] = entry
	}

	// Inline mcp_config overrides cataloged entries by name (local override).
	if len(in.InlineMCP) > 0 {
		var inline struct {
			McpServers map[string]json.RawMessage `json:"mcpServers"`
		}
		if err := json.Unmarshal(in.InlineMCP, &inline); err == nil {
			for name, cfg := range inline.McpServers {
				servers[name] = cfg
			}
		}
	}

	out, err := json.Marshal(map[string]any{"mcpServers": servers})
	if err != nil {
		return nil, warnings, err
	}
	return out, warnings, nil
}

func buildEntry(s nacos.MCPServerShape, secrets SecretSource) (json.RawMessage, []string) {
	var warnings []string
	m := map[string]any{}
	if s.Transport == "stdio" {
		m["command"] = s.Command
		if len(s.Args) > 0 {
			m["args"] = s.Args
		}
		if len(s.EnvKeys) > 0 {
			env := map[string]string{}
			for _, k := range s.EnvKeys {
				v, ok := secrets.Get(k)
				if !ok {
					warnings = append(warnings, fmt.Sprintf("mcp %s: secret %q missing from agent env", s.Name, k))
				}
				env[k] = v
			}
			m["env"] = env
		}
	} else { // "sse" | "http"
		m["url"] = s.URL
		if len(s.HeaderKeys) > 0 {
			headers := map[string]string{}
			for _, k := range s.HeaderKeys {
				v, ok := secrets.Get(k)
				if !ok {
					warnings = append(warnings, fmt.Sprintf("mcp %s: secret %q missing from agent env", s.Name, k))
				}
				headers[k] = v
			}
			m["headers"] = headers
		}
	}
	b, _ := json.Marshal(m)
	return b, warnings
}
