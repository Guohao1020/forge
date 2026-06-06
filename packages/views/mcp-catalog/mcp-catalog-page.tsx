"use client";

import { useMemo, useState } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { useWorkspaceId } from "@multica/core/hooks";
import { api } from "@multica/core/api";
import {
  mcpServerListOptions,
  workspaceKeys,
} from "@multica/core/workspace/queries";
import type { MCPServerShape } from "@multica/core/types";
import { Button } from "@multica/ui/components/ui/button";
import { Input } from "@multica/ui/components/ui/input";
import { Label } from "@multica/ui/components/ui/label";
import { Badge } from "@multica/ui/components/ui/badge";

const SHARED_NS = "shared";

interface RegisterForm {
  name: string;
  version: string;
  transport: string;
  command: string;
  args: string; // space-separated
  env_keys: string; // comma-separated KEY names (no secret values)
  url: string;
  header_keys: string; // comma-separated
  shared: boolean; // register into the shared namespace vs this workspace
}

const EMPTY_FORM: RegisterForm = {
  name: "",
  version: "",
  transport: "stdio",
  command: "",
  args: "",
  env_keys: "",
  url: "",
  header_keys: "",
  shared: false,
};

// splitList turns "A, B C" style input into a trimmed, non-empty token list.
function splitList(raw: string): string[] {
  return raw
    .split(/[\s,]+/)
    .map((s) => s.trim())
    .filter(Boolean);
}

function formToServer(form: RegisterForm): MCPServerShape {
  const base: MCPServerShape = {
    name: form.name.trim(),
    version: form.version.trim(),
    transport: form.transport,
    lifecycle: "published",
  };
  if (form.transport === "stdio") {
    base.command = form.command.trim();
    base.args = splitList(form.args);
    base.env_keys = splitList(form.env_keys);
  } else {
    base.url = form.url.trim();
    base.header_keys = splitList(form.header_keys);
  }
  return base;
}

function lifecycleVariant(
  lifecycle: string,
): "default" | "secondary" | "outline" {
  if (lifecycle === "published") return "default";
  if (lifecycle === "offline") return "secondary";
  return "outline";
}

export function McpCatalogPage() {
  const wsId = useWorkspaceId();
  const qc = useQueryClient();
  const { data, isLoading } = useQuery(mcpServerListOptions(wsId));
  const servers = data?.servers ?? [];

  const [form, setForm] = useState<RegisterForm>(EMPTY_FORM);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState("");

  const invalidate = () =>
    qc.invalidateQueries({ queryKey: workspaceKeys.mcpRegistry(wsId) });

  const sorted = useMemo(
    () =>
      [...servers].sort(
        (a, b) =>
          a.name.localeCompare(b.name) || a.version.localeCompare(b.version),
      ),
    [servers],
  );

  const submit = async () => {
    if (!form.name.trim() || !form.version.trim()) {
      setError("name and version are required");
      return;
    }
    if (form.transport === "stdio" && !form.command.trim()) {
      setError("stdio servers require a command");
      return;
    }
    if (form.transport !== "stdio" && !form.url.trim()) {
      setError("remote servers require a url");
      return;
    }
    setSaving(true);
    setError("");
    try {
      await api.registerMCPServer(
        formToServer(form),
        form.shared ? SHARED_NS : undefined,
      );
      await invalidate();
      setForm(EMPTY_FORM);
    } catch (err) {
      setError(err instanceof Error ? err.message : "failed to register server");
    } finally {
      setSaving(false);
    }
  };

  const toggleLifecycle = async (s: MCPServerShape) => {
    const next = s.lifecycle === "published" ? "offline" : "published";
    try {
      await api.setMCPLifecycle(s.name, s.version, next);
      await invalidate();
    } catch (err) {
      setError(err instanceof Error ? err.message : "failed to update lifecycle");
    }
  };

  const secretKeys = (s: MCPServerShape): string[] => [
    ...(s.env_keys ?? []),
    ...(s.header_keys ?? []),
  ];

  return (
    <div className="flex flex-1 min-h-0 flex-col gap-4 p-6">
      <div>
        <h1 className="text-lg font-semibold">MCP server catalog</h1>
        <p className="text-xs text-muted-foreground">
          The central registry of MCP servers (backed by Nacos). Agents reference
          servers from here instead of inlining config; secret values stay in each
          agent&apos;s environment — the catalog records only the KEY names.
        </p>
      </div>

      <div className="grid grid-cols-1 gap-6 md:grid-cols-[1.3fr_1fr] min-h-0 flex-1">
        {/* Catalog list */}
        <div className="min-h-0 overflow-y-auto rounded-md border">
          {isLoading ? (
            <p className="p-4 text-sm text-muted-foreground">Loading…</p>
          ) : sorted.length === 0 ? (
            <p className="p-4 text-sm text-muted-foreground">
              No servers in the catalog yet. If you expected entries here, the
              registry may be unreachable — register a server or check Nacos.
            </p>
          ) : (
            <ul className="divide-y">
              {sorted.map((s) => (
                <li key={`${s.name}@${s.version}`} className="space-y-1 px-3 py-2.5">
                  <div className="flex items-center justify-between gap-2">
                    <div className="flex min-w-0 items-center gap-2">
                      <span className="truncate text-sm font-medium">{s.name}</span>
                      <span className="font-mono text-xs text-muted-foreground">
                        {s.version}
                      </span>
                      <Badge variant="outline" className="text-[10px]">
                        {s.transport}
                      </Badge>
                      <Badge
                        variant={lifecycleVariant(s.lifecycle)}
                        className="text-[10px]"
                      >
                        {s.lifecycle}
                      </Badge>
                    </div>
                    <Button
                      variant="ghost"
                      size="sm"
                      onClick={() => toggleLifecycle(s)}
                    >
                      {s.lifecycle === "published" ? "Take offline" : "Publish"}
                    </Button>
                  </div>
                  {s.transport === "stdio" ? (
                    <div className="truncate font-mono text-xs text-muted-foreground">
                      {[s.command, ...(s.args ?? [])].filter(Boolean).join(" ")}
                    </div>
                  ) : (
                    <div className="truncate font-mono text-xs text-muted-foreground">
                      {s.url}
                    </div>
                  )}
                  {secretKeys(s).length > 0 ? (
                    <div className="text-xs text-muted-foreground">
                      requires env:{" "}
                      <span className="font-mono">{secretKeys(s).join(", ")}</span>
                    </div>
                  ) : null}
                  {s.tools && s.tools.length > 0 ? (
                    <div className="text-xs text-muted-foreground">
                      tools: {s.tools.join(", ")}
                    </div>
                  ) : null}
                </li>
              ))}
            </ul>
          )}
        </div>

        {/* Register form */}
        <div className="min-h-0 overflow-y-auto rounded-md border p-4">
          <h2 className="mb-3 text-sm font-medium">Register a server</h2>
          <div className="space-y-3">
            <div className="grid grid-cols-2 gap-2">
              <div className="space-y-1.5">
                <Label className="text-xs text-muted-foreground">Name</Label>
                <Input
                  value={form.name}
                  onChange={(e) => setForm({ ...form, name: e.target.value })}
                  placeholder="voc"
                />
              </div>
              <div className="space-y-1.5">
                <Label className="text-xs text-muted-foreground">Version</Label>
                <Input
                  value={form.version}
                  onChange={(e) => setForm({ ...form, version: e.target.value })}
                  placeholder="1.0.0"
                />
              </div>
            </div>

            <div className="space-y-1.5">
              <Label className="text-xs text-muted-foreground">Transport</Label>
              <select
                className="h-9 w-full rounded-md border border-input bg-transparent px-3 text-sm"
                value={form.transport}
                onChange={(e) => setForm({ ...form, transport: e.target.value })}
              >
                <option value="stdio">stdio</option>
                <option value="sse">sse</option>
                <option value="http">http</option>
              </select>
            </div>

            {form.transport === "stdio" ? (
              <>
                <div className="space-y-1.5">
                  <Label className="text-xs text-muted-foreground">Command</Label>
                  <Input
                    value={form.command}
                    onChange={(e) => setForm({ ...form, command: e.target.value })}
                    placeholder="voc-mcp"
                  />
                </div>
                <div className="space-y-1.5">
                  <Label className="text-xs text-muted-foreground">
                    Args (space-separated)
                  </Label>
                  <Input
                    value={form.args}
                    onChange={(e) => setForm({ ...form, args: e.target.value })}
                    placeholder="--port 8080"
                  />
                </div>
                <div className="space-y-1.5">
                  <Label className="text-xs text-muted-foreground">
                    Env keys (comma-separated KEY names — no secret values)
                  </Label>
                  <Input
                    value={form.env_keys}
                    onChange={(e) => setForm({ ...form, env_keys: e.target.value })}
                    placeholder="VOC_API_KEY"
                  />
                </div>
              </>
            ) : (
              <>
                <div className="space-y-1.5">
                  <Label className="text-xs text-muted-foreground">URL</Label>
                  <Input
                    value={form.url}
                    onChange={(e) => setForm({ ...form, url: e.target.value })}
                    placeholder="https://mcp.example.com/sse"
                  />
                </div>
                <div className="space-y-1.5">
                  <Label className="text-xs text-muted-foreground">
                    Header keys (comma-separated KEY names — no secret values)
                  </Label>
                  <Input
                    value={form.header_keys}
                    onChange={(e) =>
                      setForm({ ...form, header_keys: e.target.value })
                    }
                    placeholder="Authorization"
                  />
                </div>
              </>
            )}

            <label className="flex items-center gap-2 text-xs text-muted-foreground">
              <input
                type="checkbox"
                checked={form.shared}
                onChange={(e) => setForm({ ...form, shared: e.target.checked })}
              />
              Register into the shared namespace (visible to all workspaces)
            </label>

            {error ? <p className="text-xs text-destructive">{error}</p> : null}
            <div className="flex justify-end">
              <Button size="sm" onClick={submit} disabled={saving}>
                {saving ? "Registering…" : "Register"}
              </Button>
            </div>
          </div>
        </div>
      </div>
    </div>
  );
}
