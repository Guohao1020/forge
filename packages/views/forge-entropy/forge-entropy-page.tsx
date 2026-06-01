"use client";

import { useMemo, useState } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { useWorkspaceId } from "@multica/core/hooks";
import { api } from "@multica/core/api";
import {
  agentListOptions,
  forgeEntropyScanListOptions,
  workspaceKeys,
} from "@multica/core/workspace/queries";
import type { ForgeEntropyScan, ForgeEntropyScanInput } from "@multica/core/types";
import { Button } from "@multica/ui/components/ui/button";
import { Input } from "@multica/ui/components/ui/input";
import { Textarea } from "@multica/ui/components/ui/textarea";
import { Label } from "@multica/ui/components/ui/label";

const EMPTY: ForgeEntropyScanInput = {
  name: "",
  scanner_agent_id: "",
  custom_focus: "",
  include_standards: true,
  include_checks: true,
  cron_expression: "0 9 * * 1",
  timezone: "UTC",
  enabled: true,
  auto_fix: false,
};

export function ForgeEntropyPage() {
  const wsId = useWorkspaceId();
  const qc = useQueryClient();
  const { data: scans = [], isLoading } = useQuery(forgeEntropyScanListOptions(wsId));
  const { data: agents = [] } = useQuery(agentListOptions(wsId));

  const [editingId, setEditingId] = useState<string | null>(null);
  const [form, setForm] = useState<ForgeEntropyScanInput>(EMPTY);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState("");

  const startCreate = () => {
    setEditingId(null);
    setForm(EMPTY);
    setError("");
  };

  const startEdit = (s: ForgeEntropyScan) => {
    setEditingId(s.id);
    setForm({
      project_id: s.project_id,
      name: s.name,
      scanner_agent_id: s.scanner_agent_id,
      custom_focus: s.custom_focus,
      include_standards: s.include_standards,
      include_checks: s.include_checks,
      cron_expression: s.cron_expression,
      timezone: s.timezone,
      enabled: s.enabled,
      auto_fix: s.auto_fix,
    });
    setError("");
  };

  const invalidate = () =>
    qc.invalidateQueries({ queryKey: workspaceKeys.forgeEntropyScans(wsId) });

  const submit = async () => {
    if (!form.name.trim() || !form.cron_expression.trim()) {
      setError("name and cron are required");
      return;
    }
    if (!form.scanner_agent_id) {
      setError("pick a scanner agent");
      return;
    }
    setSaving(true);
    setError("");
    try {
      if (editingId) {
        await api.updateForgeEntropyScan(editingId, form);
      } else {
        await api.createForgeEntropyScan(form);
      }
      await invalidate();
      startCreate();
    } catch (err) {
      setError(err instanceof Error ? err.message : "failed to save scan");
    } finally {
      setSaving(false);
    }
  };

  const remove = async (id: string) => {
    await api.deleteForgeEntropyScan(id);
    await invalidate();
    if (editingId === id) startCreate();
  };

  const sorted = useMemo(
    () => [...scans].sort((a, b) => a.name.localeCompare(b.name)),
    [scans],
  );
  const activeAgents = useMemo(
    () => agents.filter((a) => !a.archived_at),
    [agents],
  );

  return (
    <div className="flex flex-1 min-h-0 flex-col gap-4 p-6">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-lg font-semibold">Entropy scans</h1>
          <p className="text-xs text-muted-foreground">
            A scanner agent periodically surveys the whole repo for quality drift
            and files advisory issues. The brief is composed from this project&apos;s
            standards (F1) + checks (F2) + your focus areas.
          </p>
        </div>
        <Button size="sm" onClick={startCreate}>
          New scan
        </Button>
      </div>

      <div className="grid grid-cols-1 gap-6 md:grid-cols-[1fr_1.2fr] min-h-0 flex-1">
        {/* List */}
        <div className="min-h-0 overflow-y-auto rounded-md border">
          {isLoading ? (
            <p className="p-4 text-sm text-muted-foreground">Loading…</p>
          ) : sorted.length === 0 ? (
            <p className="p-4 text-sm text-muted-foreground">No scans yet.</p>
          ) : (
            <ul className="divide-y">
              {sorted.map((s) => (
                <li
                  key={s.id}
                  className="flex items-center justify-between gap-2 px-3 py-2 hover:bg-accent/40"
                >
                  <button
                    type="button"
                    className="min-w-0 flex-1 text-left"
                    onClick={() => startEdit(s)}
                  >
                    <div className="truncate text-sm font-medium">{s.name}</div>
                    <div className="truncate font-mono text-xs text-muted-foreground">
                      {s.cron_expression} · {s.project_id ? "project" : "workspace"}
                      {s.enabled ? "" : " · disabled"}
                    </div>
                  </button>
                  <Button variant="ghost" size="sm" onClick={() => remove(s.id)}>
                    Delete
                  </Button>
                </li>
              ))}
            </ul>
          )}
        </div>

        {/* Editor */}
        <div className="min-h-0 overflow-y-auto rounded-md border p-4">
          <h2 className="mb-3 text-sm font-medium">
            {editingId ? "Edit scan" : "New scan"}
          </h2>
          <div className="space-y-3">
            <div className="space-y-1.5">
              <Label className="text-xs text-muted-foreground">Name</Label>
              <Input
                value={form.name}
                onChange={(e) => setForm({ ...form, name: e.target.value })}
                placeholder="weekly entropy scan"
              />
            </div>
            <div className="space-y-1.5">
              <Label className="text-xs text-muted-foreground">Scanner agent</Label>
              <select
                className="w-full rounded-md border bg-background px-3 py-2 text-sm"
                value={form.scanner_agent_id}
                onChange={(e) =>
                  setForm({ ...form, scanner_agent_id: e.target.value })
                }
              >
                <option value="">— select an agent —</option>
                {activeAgents.map((a) => (
                  <option key={a.id} value={a.id}>
                    {a.name}
                  </option>
                ))}
              </select>
            </div>
            <div className="grid grid-cols-2 gap-3">
              <div className="space-y-1.5">
                <Label className="text-xs text-muted-foreground">Cron</Label>
                <Input
                  className="font-mono"
                  value={form.cron_expression}
                  onChange={(e) =>
                    setForm({ ...form, cron_expression: e.target.value })
                  }
                  placeholder="0 9 * * 1"
                />
              </div>
              <div className="space-y-1.5">
                <Label className="text-xs text-muted-foreground">Timezone</Label>
                <Input
                  value={form.timezone}
                  onChange={(e) => setForm({ ...form, timezone: e.target.value })}
                  placeholder="UTC"
                />
              </div>
            </div>
            <label className="flex items-center gap-2 text-sm">
              <input
                type="checkbox"
                checked={form.include_standards}
                onChange={(e) =>
                  setForm({ ...form, include_standards: e.target.checked })
                }
              />
              Include project standards (F1) in the brief
            </label>
            <label className="flex items-center gap-2 text-sm">
              <input
                type="checkbox"
                checked={form.include_checks}
                onChange={(e) =>
                  setForm({ ...form, include_checks: e.target.checked })
                }
              />
              Include verification checks (F2) in the brief
            </label>
            <label className="flex items-center gap-2 text-sm">
              <input
                type="checkbox"
                checked={form.auto_fix}
                onChange={(e) =>
                  setForm({ ...form, auto_fix: e.target.checked })
                }
              />
              Let the agent fix what it safely can and open a PR
            </label>
            <div className="space-y-1.5">
              <Label className="text-xs text-muted-foreground">
                Additional focus areas
              </Label>
              <Textarea
                rows={3}
                value={form.custom_focus}
                onChange={(e) =>
                  setForm({ ...form, custom_focus: e.target.value })
                }
                placeholder="dead code, TODO/FIXME accumulation, doc staleness…"
              />
            </div>
            <label className="flex items-center gap-2 text-sm">
              <input
                type="checkbox"
                checked={form.enabled}
                onChange={(e) => setForm({ ...form, enabled: e.target.checked })}
              />
              Enabled
            </label>
            {error ? <p className="text-xs text-destructive">{error}</p> : null}
            <div className="flex justify-end gap-2">
              {editingId ? (
                <Button variant="ghost" size="sm" onClick={startCreate}>
                  Cancel
                </Button>
              ) : null}
              <Button size="sm" onClick={submit} disabled={saving}>
                {editingId ? "Save" : "Create"}
              </Button>
            </div>
          </div>
        </div>
      </div>
    </div>
  );
}
