"use client";

import { useEffect, useState } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { useWorkspaceId } from "@multica/core/hooks";
import { api } from "@multica/core/api";
import {
  agentListOptions,
  forgeReviewConfigOptions,
  workspaceKeys,
} from "@multica/core/workspace/queries";
import { Button } from "@multica/ui/components/ui/button";
import { Label } from "@multica/ui/components/ui/label";

export function ForgeReviewPage() {
  const wsId = useWorkspaceId();
  const qc = useQueryClient();
  const { data: config } = useQuery(forgeReviewConfigOptions(wsId));
  const { data: agents = [] } = useQuery(agentListOptions(wsId));

  const [reviewerId, setReviewerId] = useState("");
  const [enabled, setEnabled] = useState(true);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState("");
  const [saved, setSaved] = useState(false);

  useEffect(() => {
    if (config) {
      setReviewerId(config.reviewer_agent_id ?? "");
      setEnabled(config.enabled ?? true);
    }
  }, [config]);

  const submit = async () => {
    if (!reviewerId) {
      setError("pick a reviewer agent");
      return;
    }
    setSaving(true);
    setError("");
    setSaved(false);
    try {
      await api.putForgeReviewConfig({ reviewer_agent_id: reviewerId, enabled });
      await qc.invalidateQueries({ queryKey: workspaceKeys.forgeReviewConfig(wsId) });
      setSaved(true);
    } catch (err) {
      setError(err instanceof Error ? err.message : "failed to save");
    } finally {
      setSaving(false);
    }
  };

  return (
    <div className="flex flex-1 min-h-0 flex-col gap-4 p-6">
      <div>
        <h1 className="text-lg font-semibold">AI Review</h1>
        <p className="text-xs text-muted-foreground">
          After a coding task completes, this agent automatically reviews the
          changes (git diff) and posts findings as comments.
        </p>
      </div>

      <div className="max-w-md space-y-4 rounded-md border p-4">
        <div className="space-y-1.5">
          <Label className="text-xs text-muted-foreground">Reviewer agent</Label>
          <select
            className="w-full rounded-md border bg-background px-3 py-2 text-sm"
            value={reviewerId}
            onChange={(e) => {
              setReviewerId(e.target.value);
              setSaved(false);
            }}
          >
            <option value="">— select an agent —</option>
            {agents
              .filter((a) => !a.archived_at)
              .map((a) => (
              <option key={a.id} value={a.id}>
                {a.name}
              </option>
            ))}
          </select>
        </div>
        <label className="flex items-center gap-2 text-sm">
          <input
            type="checkbox"
            checked={enabled}
            onChange={(e) => {
              setEnabled(e.target.checked);
              setSaved(false);
            }}
          />
          Enabled
        </label>
        {error ? <p className="text-xs text-destructive">{error}</p> : null}
        {saved ? <p className="text-xs text-muted-foreground">Saved.</p> : null}
        <div className="flex justify-end">
          <Button size="sm" onClick={submit} disabled={saving}>
            Save
          </Button>
        </div>
      </div>
    </div>
  );
}
