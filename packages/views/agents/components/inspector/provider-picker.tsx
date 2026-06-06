"use client";

import { useMemo } from "react";
import { useQuery } from "@tanstack/react-query";
import { useWorkspaceId } from "@multica/core/hooks";
import { api } from "@multica/core/api";
import { providerListOptions } from "@multica/core/workspace/queries";
import type { Agent, ProviderRef, ProviderShape } from "@multica/core/types";
import { Badge } from "@multica/ui/components/ui/badge";

// providerMissingSecret returns the single auth_key a provider needs that is
// NOT present in the agent's env, or null when the provider needs no secret or
// the agent already supplies it. Pure + exported so the preflight is unit-tested
// without rendering — mirrors mcp-picker's `missingSecrets`, but single-key
// because a provider has exactly one auth_key.
export function providerMissingSecret(
  p: ProviderShape,
  agentEnvKeys: string[],
): string | null {
  return p.auth_key && !agentEnvKeys.includes(p.auth_key) ? p.auth_key : null;
}

// providerNamespace resolves the namespace a ProviderRef should carry for a
// provider: the namespace the list endpoint tagged, falling back to the current
// workspace.
function providerNamespace(p: ProviderShape, wsId: string): string {
  return p.namespace ?? wsId;
}

function isSelected(value: ProviderRef | null, p: ProviderShape, wsId: string) {
  if (!value) return false;
  return value.name === p.name && value.namespace === providerNamespace(p, wsId);
}

export function ProviderPicker({
  value,
  agentEnvKeys,
  onChange,
  disabled,
}: {
  value: ProviderRef | null;
  agentEnvKeys: string[];
  onChange: (ref: ProviderRef | null) => void;
  disabled?: boolean;
}) {
  const wsId = useWorkspaceId();
  const { data, isLoading } = useQuery(providerListOptions(wsId));
  const providers = data?.providers ?? [];

  const sorted = useMemo(
    () =>
      [...providers]
        .filter((p) => p.lifecycle === "published")
        .sort(
          (a, b) =>
            a.name.localeCompare(b.name) || a.version.localeCompare(b.version),
        ),
    [providers],
  );

  const select = (p: ProviderShape) => {
    onChange({ namespace: providerNamespace(p, wsId), name: p.name, ref: "stable" });
  };

  if (isLoading) {
    return <p className="text-xs text-muted-foreground">Loading catalog…</p>;
  }
  if (sorted.length === 0) {
    return (
      <p className="text-xs text-muted-foreground">
        No published LLM providers in the catalog. Register one in the LLM
        provider catalog first, then reference it here.
      </p>
    );
  }

  return (
    <div className="space-y-2">
      <p className="text-xs text-muted-foreground">
        Bind this agent to an LLM provider from the catalog. At dispatch the
        server resolves the provider&apos;s base URL and injects its API key from
        the agent&apos;s environment. The model picker then offers this
        provider&apos;s models.
      </p>
      <ul className="divide-y rounded-md border">
        {/* "None" — fall back to the runtime's own provider. */}
        <li className="px-3 py-2">
          <label className="flex items-center gap-2">
            <input
              type="radio"
              name="provider-ref"
              checked={value === null}
              disabled={disabled}
              aria-label="select none"
              onChange={() => onChange(null)}
            />
            <span className="text-sm font-medium">None</span>
            <span className="ml-auto text-xs text-muted-foreground">
              Use runtime default
            </span>
          </label>
        </li>
        {sorted.map((p) => {
          const ns = providerNamespace(p, wsId);
          const checked = isSelected(value, p, wsId);
          const missing = checked ? providerMissingSecret(p, agentEnvKeys) : null;
          return (
            <li key={`${ns}/${p.name}`} className="space-y-1 px-3 py-2">
              <label className="flex items-center gap-2">
                <input
                  type="radio"
                  name="provider-ref"
                  checked={checked}
                  disabled={disabled}
                  aria-label={`select ${p.name}`}
                  onChange={() => select(p)}
                />
                <span className="text-sm font-medium">{p.name}</span>
                <Badge variant="outline" className="text-[10px]">
                  {p.protocol}
                </Badge>
                {ns === "shared" ? (
                  <Badge variant="secondary" className="text-[10px]">
                    shared
                  </Badge>
                ) : null}
                <span className="ml-auto font-mono text-xs text-muted-foreground">
                  {p.version}
                </span>
              </label>
              {missing ? (
                <p className="text-xs text-destructive">
                  Needs agent env: <span className="font-mono">{missing}</span>
                </p>
              ) : null}
            </li>
          );
        })}
      </ul>
    </div>
  );
}

// ProviderRefSection is the agent-detail mount point: it fetches the agent's env
// KEY names (for the missing-secret preflight) and saves the selected ref
// through the standard agent update path. The env fetch is owner/admin only and
// may 403 for some viewers — on failure we fall back to no keys (the picker
// still works; the preflight just can't confirm the secret, over-warning in the
// worst case). Mirrors mcp-picker's McpRefSection.
export function ProviderRefSection({
  agent,
  onSave,
  disabled,
}: {
  agent: Agent;
  onSave: (updates: { provider_ref: ProviderRef | null }) => Promise<void> | void;
  disabled?: boolean;
}) {
  const { data: env } = useQuery({
    queryKey: ["agents", agent.id, "env"],
    queryFn: () => api.getAgentEnv(agent.id),
    retry: false,
    staleTime: 30_000,
  });
  const agentEnvKeys = env ? Object.keys(env.custom_env) : [];

  return (
    <ProviderPicker
      value={agent.provider_ref ?? null}
      agentEnvKeys={agentEnvKeys}
      disabled={disabled}
      onChange={(ref) => onSave({ provider_ref: ref })}
    />
  );
}
