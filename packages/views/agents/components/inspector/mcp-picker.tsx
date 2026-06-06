"use client";

import { useMemo } from "react";
import { useQuery } from "@tanstack/react-query";
import { useWorkspaceId } from "@multica/core/hooks";
import { api } from "@multica/core/api";
import { mcpServerListOptions } from "@multica/core/workspace/queries";
import type { Agent, MCPRef, MCPServerShape } from "@multica/core/types";
import { Input } from "@multica/ui/components/ui/input";
import { Badge } from "@multica/ui/components/ui/badge";

// missingSecrets returns the secret KEY names a server needs (env_keys +
// header_keys) that are NOT present in the agent's env. Pure + exported so the
// preflight is unit-tested without rendering. An empty agentEnvKeys means
// "unknown" to the caller, but this function still reports every required key —
// the component decides whether to surface the warning.
export function missingSecrets(
  server: MCPServerShape,
  agentEnvKeys: string[],
): string[] {
  const need = [...(server.env_keys ?? []), ...(server.header_keys ?? [])];
  return need.filter((k) => !agentEnvKeys.includes(k));
}

// serverNamespace resolves the namespace an MCPRef should carry for a server:
// the namespace the list endpoint tagged, falling back to the current workspace.
function serverNamespace(server: MCPServerShape, wsId: string): string {
  return server.namespace ?? wsId;
}

function refKey(namespace: string, name: string): string {
  return `${namespace}|${name}`;
}

export function McpRefPicker({
  value,
  agentEnvKeys,
  onChange,
  disabled,
}: {
  value: MCPRef[];
  agentEnvKeys: string[];
  onChange: (refs: MCPRef[]) => void;
  disabled?: boolean;
}) {
  const wsId = useWorkspaceId();
  const { data, isLoading } = useQuery(mcpServerListOptions(wsId));
  const servers = data?.servers ?? [];

  // Index selected refs by namespace|name for O(1) lookup + edits.
  const selected = useMemo(() => {
    const m = new Map<string, MCPRef>();
    for (const r of value) m.set(refKey(r.namespace, r.name), r);
    return m;
  }, [value]);

  const sorted = useMemo(
    () =>
      [...servers]
        .filter((s) => s.lifecycle === "published")
        .sort(
          (a, b) =>
            a.name.localeCompare(b.name) || a.version.localeCompare(b.version),
        ),
    [servers],
  );

  const toggle = (server: MCPServerShape, checked: boolean) => {
    const ns = serverNamespace(server, wsId);
    const key = refKey(ns, server.name);
    const next = new Map(selected);
    if (checked) {
      next.set(key, { namespace: ns, name: server.name, ref: "stable" });
    } else {
      next.delete(key);
    }
    onChange([...next.values()]);
  };

  const setRef = (server: MCPServerShape, ref: string) => {
    const ns = serverNamespace(server, wsId);
    const key = refKey(ns, server.name);
    const existing = selected.get(key);
    if (!existing) return;
    const next = new Map(selected);
    next.set(key, { ...existing, ref });
    onChange([...next.values()]);
  };

  if (isLoading) {
    return <p className="text-xs text-muted-foreground">Loading catalog…</p>;
  }
  if (sorted.length === 0) {
    return (
      <p className="text-xs text-muted-foreground">
        No published MCP servers in the catalog. Register one in the MCP catalog
        first, then reference it here.
      </p>
    );
  }

  return (
    <div className="space-y-2">
      <p className="text-xs text-muted-foreground">
        Reference MCP servers from the catalog. At dispatch the server resolves
        these into the agent&apos;s effective config and injects secrets from the
        agent&apos;s environment.
      </p>
      <ul className="divide-y rounded-md border">
        {sorted.map((s) => {
          const ns = serverNamespace(s, wsId);
          const key = refKey(ns, s.name);
          const ref = selected.get(key);
          const checked = ref !== undefined;
          const missing = checked ? missingSecrets(s, agentEnvKeys) : [];
          return (
            <li key={`${ns}/${s.name}`} className="space-y-1 px-3 py-2">
              <div className="flex items-center gap-2">
                <input
                  type="checkbox"
                  checked={checked}
                  disabled={disabled}
                  aria-label={`select ${s.name}`}
                  onChange={(e) => toggle(s, e.target.checked)}
                />
                <span className="text-sm font-medium">{s.name}</span>
                <Badge variant="outline" className="text-[10px]">
                  {s.transport}
                </Badge>
                {ns === "shared" ? (
                  <Badge variant="secondary" className="text-[10px]">
                    shared
                  </Badge>
                ) : null}
                {checked ? (
                  <Input
                    value={ref?.ref ?? "stable"}
                    disabled={disabled}
                    aria-label={`${s.name} ref`}
                    onChange={(e) => setRef(s, e.target.value)}
                    className="ml-auto h-7 w-28 text-xs"
                    placeholder="stable"
                  />
                ) : (
                  <span className="ml-auto font-mono text-xs text-muted-foreground">
                    {s.version}
                  </span>
                )}
              </div>
              {missing.length > 0 ? (
                <p className="text-xs text-destructive">
                  Needs agent env:{" "}
                  <span className="font-mono">{missing.join(", ")}</span>
                </p>
              ) : null}
            </li>
          );
        })}
      </ul>
    </div>
  );
}

// McpRefSection is the agent-detail mount point: it fetches the agent's env KEY
// names (for the missing-secret preflight) and saves selected refs through the
// standard agent update path. The env fetch is owner/admin only and may 403 for
// some viewers — on failure we fall back to no keys (the picker still works; the
// preflight just can't confirm secrets, which the worst case over-warns).
export function McpRefSection({
  agent,
  onSave,
  disabled,
}: {
  agent: Agent;
  onSave: (updates: { mcp_refs: MCPRef[] }) => Promise<void> | void;
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
    <McpRefPicker
      value={agent.mcp_refs ?? []}
      agentEnvKeys={agentEnvKeys}
      disabled={disabled}
      onChange={(refs) => onSave({ mcp_refs: refs })}
    />
  );
}
