"use client";

import { useMemo, useState } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { useWorkspaceId } from "@multica/core/hooks";
import { api } from "@multica/core/api";
import {
  providerListOptions,
  workspaceKeys,
} from "@multica/core/workspace/queries";
import type { ProviderShape } from "@multica/core/types";
import { Button } from "@multica/ui/components/ui/button";
import { Input } from "@multica/ui/components/ui/input";
import { Label } from "@multica/ui/components/ui/label";
import { Badge } from "@multica/ui/components/ui/badge";

const SHARED_NS = "shared";

interface RegisterForm {
  name: string;
  version: string;
  protocol: string;
  base_url: string;
  auth_key: string; // KEY name (no secret value)
  wire_api: string;
  shared: boolean; // register into the shared namespace vs this workspace
}

const EMPTY_FORM: RegisterForm = {
  name: "",
  version: "",
  protocol: "anthropic",
  base_url: "",
  auth_key: "",
  wire_api: "",
  shared: false,
};

function formToProvider(form: RegisterForm): ProviderShape {
  const base: ProviderShape = {
    name: form.name.trim(),
    version: form.version.trim(),
    protocol: form.protocol,
    base_url: form.base_url.trim(),
    auth_key: form.auth_key.trim(),
    lifecycle: "published",
  };
  const wireApi = form.wire_api.trim();
  if (wireApi) base.wire_api = wireApi;
  return base;
}

function lifecycleVariant(
  lifecycle: string,
): "default" | "secondary" | "outline" {
  if (lifecycle === "published") return "default";
  if (lifecycle === "offline") return "secondary";
  return "outline";
}

export function LlmProvidersPage() {
  const wsId = useWorkspaceId();
  const qc = useQueryClient();
  const { data, isLoading } = useQuery(providerListOptions(wsId));
  const providers = data?.providers ?? [];

  const [form, setForm] = useState<RegisterForm>(EMPTY_FORM);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState("");

  const invalidate = () =>
    Promise.all([
      qc.invalidateQueries({ queryKey: workspaceKeys.llmProviders(wsId) }),
      // The agent model-picker reads each bound provider under its own
      // ["llm-providers", ns, name, ref] key (model-picker.tsx) — invalidate
      // that tree too so editing a provider's models[] here doesn't leave the
      // inspector showing a stale catalog until its staleTime lapses.
      qc.invalidateQueries({ queryKey: ["llm-providers"] }),
    ]);

  const sorted = useMemo(
    () =>
      [...providers].sort(
        (a, b) =>
          a.name.localeCompare(b.name) || a.version.localeCompare(b.version),
      ),
    [providers],
  );

  const submit = async () => {
    if (!form.name.trim() || !form.version.trim()) {
      setError("name and version are required");
      return;
    }
    if (!form.base_url.trim()) {
      setError("base_url is required");
      return;
    }
    if (!form.auth_key.trim()) {
      setError("auth_key (KEY name) is required");
      return;
    }
    setSaving(true);
    setError("");
    try {
      await api.registerProvider(
        formToProvider(form),
        form.shared ? SHARED_NS : undefined,
      );
      await invalidate();
      setForm(EMPTY_FORM);
    } catch (err) {
      setError(err instanceof Error ? err.message : "failed to register provider");
    } finally {
      setSaving(false);
    }
  };

  const toggleLifecycle = async (p: ProviderShape) => {
    const next = p.lifecycle === "published" ? "offline" : "published";
    try {
      await api.setProviderLifecycle(p.name, p.version, next, p.namespace);
      await invalidate();
    } catch (err) {
      setError(err instanceof Error ? err.message : "failed to update lifecycle");
    }
  };

  return (
    <div className="flex flex-1 min-h-0 flex-col gap-4 p-6">
      <div>
        <h1 className="text-lg font-semibold">LLM provider catalog</h1>
        <p className="text-xs text-muted-foreground">
          The central registry of LLM providers (backed by Nacos). Agents
          reference a provider from here instead of inlining endpoint config; the
          secret value stays in each agent&apos;s environment — the catalog
          records only the KEY name.
        </p>
      </div>

      <div className="grid grid-cols-1 gap-6 md:grid-cols-[1.3fr_1fr] min-h-0 flex-1">
        {/* Catalog list */}
        <div className="min-h-0 overflow-y-auto rounded-md border">
          {isLoading ? (
            <p className="p-4 text-sm text-muted-foreground">Loading…</p>
          ) : sorted.length === 0 ? (
            <p className="p-4 text-sm text-muted-foreground">
              No providers in the catalog yet. If you expected entries here, the
              registry may be unreachable — register a provider or check Nacos.
            </p>
          ) : (
            <ul className="divide-y">
              {sorted.map((p) => (
                <li key={`${p.name}@${p.version}`} className="space-y-1 px-3 py-2.5">
                  <div className="flex items-center justify-between gap-2">
                    <div className="flex min-w-0 items-center gap-2">
                      <span className="truncate text-sm font-medium">{p.name}</span>
                      <span className="font-mono text-xs text-muted-foreground">
                        {p.version}
                      </span>
                      <Badge variant="outline" className="text-[10px]">
                        {p.protocol}
                      </Badge>
                      <Badge
                        variant={lifecycleVariant(p.lifecycle)}
                        className="text-[10px]"
                      >
                        {p.lifecycle}
                      </Badge>
                    </div>
                    <Button
                      variant="ghost"
                      size="sm"
                      onClick={() => toggleLifecycle(p)}
                    >
                      {p.lifecycle === "published" ? "Take offline" : "Publish"}
                    </Button>
                  </div>
                  <div className="truncate font-mono text-xs text-muted-foreground">
                    {[p.base_url, p.wire_api].filter(Boolean).join(" · ")}
                  </div>
                  {p.auth_key ? (
                    <div className="text-xs text-muted-foreground">
                      requires env:{" "}
                      <span className="font-mono">{p.auth_key}</span>
                    </div>
                  ) : null}
                  {p.models && p.models.length > 0 ? (
                    <div className="text-xs text-muted-foreground">
                      models:{" "}
                      {p.models.map((m) => m.label ?? m.id).join(", ")}
                    </div>
                  ) : null}
                </li>
              ))}
            </ul>
          )}
        </div>

        {/* Register form */}
        <div className="min-h-0 overflow-y-auto rounded-md border p-4">
          <h2 className="mb-3 text-sm font-medium">Register a provider</h2>
          <div className="space-y-3">
            <div className="grid grid-cols-2 gap-2">
              <div className="space-y-1.5">
                <Label className="text-xs text-muted-foreground">Name</Label>
                <Input
                  value={form.name}
                  onChange={(e) => setForm({ ...form, name: e.target.value })}
                  placeholder="claude-router"
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
              <Label className="text-xs text-muted-foreground">Protocol</Label>
              <select
                className="h-9 w-full rounded-md border border-input bg-transparent px-3 text-sm"
                value={form.protocol}
                onChange={(e) => setForm({ ...form, protocol: e.target.value })}
              >
                <option value="anthropic">anthropic</option>
                <option value="codex-router">codex-router</option>
              </select>
            </div>

            <div className="space-y-1.5">
              <Label className="text-xs text-muted-foreground">Base URL</Label>
              <Input
                value={form.base_url}
                onChange={(e) => setForm({ ...form, base_url: e.target.value })}
                placeholder="<ROUTER_BASE_URL>"
              />
            </div>

            <div className="space-y-1.5">
              <Label className="text-xs text-muted-foreground">
                Auth key (KEY name — no secret value)
              </Label>
              <Input
                value={form.auth_key}
                onChange={(e) => setForm({ ...form, auth_key: e.target.value })}
                placeholder="ANTHROPIC_AUTH_TOKEN"
              />
            </div>

            <div className="space-y-1.5">
              <Label className="text-xs text-muted-foreground">
                Wire API (optional)
              </Label>
              <Input
                value={form.wire_api}
                onChange={(e) => setForm({ ...form, wire_api: e.target.value })}
                placeholder="responses"
              />
            </div>

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
