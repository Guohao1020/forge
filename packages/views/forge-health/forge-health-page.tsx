"use client";

import { useQuery } from "@tanstack/react-query";
import { useWorkspaceId } from "@multica/core/hooks";
import { forgeHealthOptions } from "@multica/core/workspace/queries";

function rate(num: number, den: number): string {
  if (den <= 0) return "—";
  return `${Math.round((num / den) * 100)}%`;
}

function Card({ label, value, sub }: { label: string; value: string; sub?: string }) {
  return (
    <div className="rounded-md border p-4">
      <div className="text-xs text-muted-foreground">{label}</div>
      <div className="mt-1 text-2xl font-semibold tabular-nums">{value}</div>
      {sub ? <div className="mt-1 text-xs text-muted-foreground">{sub}</div> : null}
    </div>
  );
}

export function ForgeHealthPage() {
  const wsId = useWorkspaceId();
  const { data: h, isLoading } = useQuery(forgeHealthOptions(wsId));

  if (isLoading || !h) {
    return <div className="p-6 text-sm text-muted-foreground">Loading…</div>;
  }

  const gateTotal = h.gate.passed + h.gate.failed;
  const mergeRate = h.fix_prs.matched > 0 ? rate(h.fix_prs.merged, h.fix_prs.opened) : "— · needs GitHub App";

  return (
    <div className="flex flex-1 min-h-0 flex-col gap-6 overflow-y-auto p-6">
      <div>
        <h1 className="text-lg font-semibold">Harness health</h1>
        <p className="text-xs text-muted-foreground">
          What the Forge Harness is doing across this workspace (last 30 days for activity).
        </p>
      </div>

      <section>
        <h2 className="mb-2 text-sm font-medium">Configured</h2>
        <div className="grid grid-cols-2 gap-3 md:grid-cols-4">
          <Card label="Standards (F1)" value={String(h.standards_total)} />
          <Card label="Checks (F2)" value={String(h.checks)} />
          <Card label="Reviewers (F3)" value={String(h.review_configs)} />
          <Card label="Entropy scans (F4)" value={String(h.scans)} />
        </div>
      </section>

      <section>
        <h2 className="mb-2 text-sm font-medium">Activity (last 30 days)</h2>
        <div className="grid grid-cols-2 gap-3 md:grid-cols-4">
          <Card label="Gate pass rate (F2)" value={rate(h.gate.passed, gateTotal)} sub={`${h.gate.passed} pass · ${h.gate.failed} fail`} />
          <Card label="Reviews (F3)" value={String(h.review.total)} sub={`${h.review.completed} completed · ${Math.round(h.review.avg_turnaround_sec / 60)}m avg`} />
          <Card label="Open findings (F4)" value={String(h.open_findings)} sub={`${h.scan_runs} scan runs`} />
          <Card label="Fix PRs (F4b)" value={String(h.fix_prs.opened)} sub={`merge rate ${mergeRate}`} />
        </div>
      </section>
    </div>
  );
}
