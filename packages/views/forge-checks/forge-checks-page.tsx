"use client";

import { useMemo, useState } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { useWorkspaceId } from "@multica/core/hooks";
import { api } from "@multica/core/api";
import {
  forgeCheckListOptions,
  workspaceKeys,
} from "@multica/core/workspace/queries";
import type { ForgeCheck, ForgeCheckInput } from "@multica/core/types";
import { Button } from "@multica/ui/components/ui/button";
import { Input } from "@multica/ui/components/ui/input";
import { Textarea } from "@multica/ui/components/ui/textarea";
import { Label } from "@multica/ui/components/ui/label";

const EMPTY: ForgeCheckInput = { name: "", command: "", enabled: true };

export function ForgeChecksPage() {
  const wsId = useWorkspaceId();
  const qc = useQueryClient();
  const { data: checks = [], isLoading } = useQuery(forgeCheckListOptions(wsId));

  const [editingId, setEditingId] = useState<string | null>(null);
  const [form, setForm] = useState<ForgeCheckInput>(EMPTY);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState("");

  const startCreate = () => {
    setEditingId(null);
    setForm(EMPTY);
    setError("");
  };

  const startEdit = (c: ForgeCheck) => {
    setEditingId(c.id);
    setForm({
      project_id: c.project_id,
      name: c.name,
      command: c.command,
      enabled: c.enabled,
    });
    setError("");
  };

  const invalidate = () =>
    qc.invalidateQueries({ queryKey: workspaceKeys.forgeChecks(wsId) });

  const submit = async () => {
    if (!form.name.trim() || !form.command.trim()) {
      setError("name and command are required");
      return;
    }
    setSaving(true);
    setError("");
    try {
      if (editingId) {
        await api.updateForgeCheck(editingId, form);
      } else {
        await api.createForgeCheck(form);
      }
      await invalidate();
      startCreate();
    } catch (err) {
      setError(err instanceof Error ? err.message : "failed to save check");
    } finally {
      setSaving(false);
    }
  };

  const remove = async (id: string) => {
    await api.deleteForgeCheck(id);
    await invalidate();
    if (editingId === id) startCreate();
  };

  const scope = (c: ForgeCheck) => (c.project_id ? "project" : "workspace");

  const sorted = useMemo(
    () => [...checks].sort((a, b) => a.name.localeCompare(b.name)),
    [checks],
  );

  return (
    <div className="flex flex-1 min-h-0 flex-col gap-4 p-6">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-lg font-semibold">Verification checks</h1>
          <p className="text-xs text-muted-foreground">
            Shell commands run in the task workdir after the agent finishes. A
            non-zero exit blocks the task and comments the failure on the issue.
          </p>
        </div>
        <Button size="sm" onClick={startCreate}>
          New check
        </Button>
      </div>

      <div className="grid grid-cols-1 gap-6 md:grid-cols-[1fr_1.2fr] min-h-0 flex-1">
        {/* List */}
        <div className="min-h-0 overflow-y-auto rounded-md border">
          {isLoading ? (
            <p className="p-4 text-sm text-muted-foreground">Loading…</p>
          ) : sorted.length === 0 ? (
            <p className="p-4 text-sm text-muted-foreground">No checks yet.</p>
          ) : (
            <ul className="divide-y">
              {sorted.map((c) => (
                <li
                  key={c.id}
                  className="flex items-center justify-between gap-2 px-3 py-2 hover:bg-accent/40"
                >
                  <button
                    type="button"
                    className="min-w-0 flex-1 text-left"
                    onClick={() => startEdit(c)}
                  >
                    <div className="truncate text-sm font-medium">{c.name}</div>
                    <div className="truncate font-mono text-xs text-muted-foreground">
                      {c.command} · {scope(c)}
                      {c.enabled ? "" : " · disabled"}
                    </div>
                  </button>
                  <Button variant="ghost" size="sm" onClick={() => remove(c.id)}>
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
            {editingId ? "Edit check" : "New check"}
          </h2>
          <div className="space-y-3">
            <div className="space-y-1.5">
              <Label className="text-xs text-muted-foreground">Name</Label>
              <Input
                value={form.name}
                onChange={(e) => setForm({ ...form, name: e.target.value })}
                placeholder="go-build"
              />
            </div>
            <div className="space-y-1.5">
              <Label className="text-xs text-muted-foreground">
                Command (runs in workdir; non-zero exit = fail)
              </Label>
              <Textarea
                rows={3}
                className="font-mono"
                value={form.command}
                onChange={(e) => setForm({ ...form, command: e.target.value })}
                placeholder="go build ./..."
              />
            </div>
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
