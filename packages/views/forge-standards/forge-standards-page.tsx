"use client";

import { useMemo, useState } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { useWorkspaceId } from "@multica/core/hooks";
import { api } from "@multica/core/api";
import {
  forgeStandardListOptions,
  workspaceKeys,
} from "@multica/core/workspace/queries";
import type { ForgeStandard, ForgeStandardInput } from "@multica/core/types";
import { Button } from "@multica/ui/components/ui/button";
import { Input } from "@multica/ui/components/ui/input";
import { Textarea } from "@multica/ui/components/ui/textarea";
import { Label } from "@multica/ui/components/ui/label";

const EMPTY: ForgeStandardInput = {
  name: "",
  category: "",
  profile_tags: [],
  core_content: "",
  detail_content: "",
  enabled: true,
};

export function ForgeStandardsPage() {
  const wsId = useWorkspaceId();
  const qc = useQueryClient();
  const { data: standards = [], isLoading } = useQuery(
    forgeStandardListOptions(wsId),
  );

  const [editingId, setEditingId] = useState<string | null>(null);
  const [form, setForm] = useState<ForgeStandardInput>(EMPTY);
  const [tagsText, setTagsText] = useState("");
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState("");

  const startCreate = () => {
    setEditingId(null);
    setForm(EMPTY);
    setTagsText("");
    setError("");
  };

  const startEdit = (s: ForgeStandard) => {
    setEditingId(s.id);
    setForm({
      project_id: s.project_id,
      name: s.name,
      category: s.category,
      profile_tags: s.profile_tags,
      core_content: s.core_content,
      detail_content: s.detail_content,
      enabled: s.enabled,
    });
    setTagsText(s.profile_tags.join(", "));
    setError("");
  };

  const invalidate = () =>
    qc.invalidateQueries({ queryKey: workspaceKeys.forgeStandards(wsId) });

  const submit = async () => {
    if (!form.name.trim() || !form.category.trim()) {
      setError("name and category are required");
      return;
    }
    setSaving(true);
    setError("");
    const payload: ForgeStandardInput = {
      ...form,
      profile_tags: tagsText
        .split(",")
        .map((t) => t.trim())
        .filter(Boolean),
    };
    try {
      if (editingId) {
        await api.updateForgeStandard(editingId, payload);
      } else {
        await api.createForgeStandard(payload);
      }
      await invalidate();
      startCreate();
    } catch (err) {
      setError(err instanceof Error ? err.message : "failed to save standard");
    } finally {
      setSaving(false);
    }
  };

  const remove = async (id: string) => {
    await api.deleteForgeStandard(id);
    await invalidate();
    if (editingId === id) startCreate();
  };

  const scope = (s: ForgeStandard) => (s.project_id ? "project" : "workspace");

  const sorted = useMemo(
    () =>
      [...standards].sort(
        (a, b) =>
          a.category.localeCompare(b.category) || a.name.localeCompare(b.name),
      ),
    [standards],
  );

  return (
    <div className="flex flex-1 min-h-0 flex-col gap-4 p-6">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-lg font-semibold">Standards</h1>
          <p className="text-xs text-muted-foreground">
            Coding standards injected into agents — core rules into instructions,
            detail into an on-demand skill.
          </p>
        </div>
        <Button size="sm" onClick={startCreate}>
          New standard
        </Button>
      </div>

      <div className="grid grid-cols-1 gap-6 md:grid-cols-[1fr_1.2fr] min-h-0 flex-1">
        {/* List */}
        <div className="min-h-0 overflow-y-auto rounded-md border">
          {isLoading ? (
            <p className="p-4 text-sm text-muted-foreground">Loading…</p>
          ) : sorted.length === 0 ? (
            <p className="p-4 text-sm text-muted-foreground">No standards yet.</p>
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
                    <div className="truncate text-xs text-muted-foreground">
                      [{s.category}] · {scope(s)}
                      {s.profile_tags.length
                        ? ` · ${s.profile_tags.join(", ")}`
                        : ""}
                    </div>
                  </button>
                  <Button
                    variant="ghost"
                    size="sm"
                    onClick={() => remove(s.id)}
                  >
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
            {editingId ? "Edit standard" : "New standard"}
          </h2>
          <div className="space-y-3">
            <div className="grid grid-cols-2 gap-3">
              <div className="space-y-1.5">
                <Label className="text-xs text-muted-foreground">Name</Label>
                <Input
                  value={form.name}
                  onChange={(e) => setForm({ ...form, name: e.target.value })}
                  placeholder="rest"
                />
              </div>
              <div className="space-y-1.5">
                <Label className="text-xs text-muted-foreground">Category</Label>
                <Input
                  value={form.category}
                  onChange={(e) =>
                    setForm({ ...form, category: e.target.value })
                  }
                  placeholder="api / sql / naming"
                />
              </div>
            </div>
            <div className="space-y-1.5">
              <Label className="text-xs text-muted-foreground">
                Profile tags (comma-separated; empty = applies to all)
              </Label>
              <Input
                value={tagsText}
                onChange={(e) => setTagsText(e.target.value)}
                placeholder="go, postgres"
              />
            </div>
            <div className="space-y-1.5">
              <Label className="text-xs text-muted-foreground">
                Core (mandatory — into agent instructions)
              </Label>
              <Textarea
                rows={4}
                value={form.core_content}
                onChange={(e) =>
                  setForm({ ...form, core_content: e.target.value })
                }
              />
            </div>
            <div className="space-y-1.5">
              <Label className="text-xs text-muted-foreground">
                Detail (on-demand — into forge-standards skill)
              </Label>
              <Textarea
                rows={6}
                value={form.detail_content}
                onChange={(e) =>
                  setForm({ ...form, detail_content: e.target.value })
                }
              />
            </div>
            {error ? (
              <p className="text-xs text-destructive">{error}</p>
            ) : null}
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
