"use client";

import { useState } from "react";
import type { ReactElement } from "react";
import { useQuery } from "@tanstack/react-query";
import {
  ResponsiveContainer, LineChart, Line, BarChart, Bar,
  XAxis, YAxis, Tooltip, CartesianGrid,
} from "recharts";
import { useWorkspaceId } from "@multica/core/hooks";
import { api } from "@multica/core/api";
import {
  forgeHealthOptions,
  forgeHealthTrendsOptions,
  workspaceKeys,
} from "@multica/core/workspace/queries";

type Drill = "findings" | "gate" | "fix_prs" | null;

function rate(num: number, den: number): string {
  if (den <= 0) return "—";
  return `${Math.round((num / den) * 100)}%`;
}

function MetricCard({
  label, value, sub, active, onClick,
}: { label: string; value: string; sub?: string; active?: boolean; onClick?: () => void }) {
  return (
    <button
      type="button"
      onClick={onClick}
      className={`rounded-md border p-4 text-left transition-colors ${onClick ? "hover:bg-accent/40" : ""} ${active ? "border-foreground" : ""}`}
    >
      <div className="text-xs text-muted-foreground">{label}</div>
      <div className="mt-1 text-2xl font-semibold tabular-nums">{value}</div>
      {sub ? <div className="mt-1 text-xs text-muted-foreground">{sub}</div> : null}
    </button>
  );
}

function Chart({ title, children }: { title: string; children: ReactElement }) {
  return (
    <div className="rounded-md border p-4">
      <div className="mb-2 text-xs text-muted-foreground">{title}</div>
      <div className="h-40">
        <ResponsiveContainer width="100%" height="100%">{children}</ResponsiveContainer>
      </div>
    </div>
  );
}

function DrillIssues({ title, loading, items }: { title: string; loading: boolean; items: { issue_id: string; number: number; title: string }[] }) {
  return (
    <div>
      <div className="mb-1 text-xs text-muted-foreground">{title}</div>
      {loading ? <p className="text-xs text-muted-foreground">Loading…</p>
        : items.length === 0 ? <p className="text-xs text-muted-foreground">None.</p>
        : (
          <ul className="divide-y">
            {items.map((it) => (
              <li key={it.issue_id} className="py-1.5 text-xs">
                <span className="font-mono text-muted-foreground">#{it.number}</span> {it.title}
              </li>
            ))}
          </ul>
        )}
    </div>
  );
}

export function ForgeHealthPage() {
  const wsId = useWorkspaceId();
  const { data: h, isLoading } = useQuery(forgeHealthOptions(wsId));
  const { data: trends } = useQuery(forgeHealthTrendsOptions(wsId));
  const [drill, setDrill] = useState<Drill>(null);

  const findingsQ = useQuery({
    queryKey: [...workspaceKeys.forgeHealth(wsId), "findings"],
    queryFn: () => api.getForgeHealthFindings(),
    enabled: drill === "findings",
  });
  const gateQ = useQuery({
    queryKey: [...workspaceKeys.forgeHealth(wsId), "gate-failures"],
    queryFn: () => api.getForgeHealthGateFailures(),
    enabled: drill === "gate",
  });
  const fixPRQ = useQuery({
    queryKey: [...workspaceKeys.forgeHealth(wsId), "fix-prs"],
    queryFn: () => api.getForgeHealthFixPRs(),
    enabled: drill === "fix_prs",
  });

  if (isLoading || !h) {
    return <div className="p-6 text-sm text-muted-foreground">Loading…</div>;
  }

  const gateTotal = h.gate.passed + h.gate.failed;
  const mergeRate = h.fix_prs.matched > 0 ? rate(h.fix_prs.merged, h.fix_prs.opened) : "— · needs GitHub App";
  const gateRateSeries = (trends?.gate ?? []).map((p) => ({
    date: p.date,
    rate: (p.passed ?? 0) + (p.failed ?? 0) > 0
      ? Math.round(((p.passed ?? 0) / ((p.passed ?? 0) + (p.failed ?? 0))) * 100)
      : 0,
  }));
  const toggle = (d: Drill) => setDrill((cur) => (cur === d ? null : d));

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
          <MetricCard label="Standards (F1)" value={String(h.standards_total)} />
          <MetricCard label="Checks (F2)" value={String(h.checks)} />
          <MetricCard label="Reviewers (F3)" value={String(h.review_configs)} />
          <MetricCard label="Entropy scans (F4)" value={String(h.scans)} />
        </div>
      </section>

      <section>
        <h2 className="mb-2 text-sm font-medium">Activity (last 30 days) — click to drill in</h2>
        <div className="grid grid-cols-2 gap-3 md:grid-cols-4">
          <MetricCard label="Gate pass rate (F2)" value={rate(h.gate.passed, gateTotal)} sub={`${h.gate.passed} pass · ${h.gate.failed} fail`} active={drill === "gate"} onClick={() => toggle("gate")} />
          <MetricCard label="Reviews (F3)" value={String(h.review.total)} sub={`${h.review.completed} done · ${Math.round(h.review.avg_turnaround_sec / 60)}m avg`} />
          <MetricCard label="Open findings (F4)" value={String(h.open_findings)} sub={`${h.scan_runs} scan runs`} active={drill === "findings"} onClick={() => toggle("findings")} />
          <MetricCard label="Fix PRs (F4b)" value={String(h.fix_prs.opened)} sub={`merge ${mergeRate}`} active={drill === "fix_prs"} onClick={() => toggle("fix_prs")} />
        </div>

        {drill ? (
          <div className="mt-3 rounded-md border p-3 text-sm">
            {drill === "findings" ? (
              <DrillIssues title="Open entropy findings" loading={findingsQ.isLoading} items={findingsQ.data ?? []} />
            ) : null}
            {drill === "gate" ? (
              <DrillIssues title="Recent gate failures" loading={gateQ.isLoading} items={gateQ.data ?? []} />
            ) : null}
            {drill === "fix_prs" ? (
              <div>
                <div className="mb-1 text-xs text-muted-foreground">Recent fix PRs</div>
                {fixPRQ.isLoading ? <p className="text-xs text-muted-foreground">Loading…</p>
                  : (fixPRQ.data ?? []).length === 0 ? <p className="text-xs text-muted-foreground">None.</p>
                  : (
                    <ul className="divide-y">
                      {(fixPRQ.data ?? []).map((p) => (
                        <li key={p.pr_url} className="py-1.5">
                          <a href={p.pr_url} target="_blank" rel="noreferrer" className="font-mono text-xs text-foreground underline">
                            {p.pr_url}
                          </a>
                          <span className="ml-2 text-xs text-muted-foreground">#{p.number} {p.title}</span>
                        </li>
                      ))}
                    </ul>
                  )}
              </div>
            ) : null}
          </div>
        ) : null}
      </section>

      <section>
        <h2 className="mb-2 text-sm font-medium">Trends</h2>
        <div className="grid grid-cols-1 gap-3 md:grid-cols-3">
          <Chart title="Entropy findings / day">
            <LineChart data={trends?.findings ?? []}>
              <CartesianGrid strokeDasharray="3 3" className="stroke-border" />
              <XAxis dataKey="date" tick={{ fontSize: 10 }} />
              <YAxis tick={{ fontSize: 10 }} allowDecimals={false} />
              <Tooltip />
              <Line type="monotone" dataKey="count" stroke="currentColor" dot={false} />
            </LineChart>
          </Chart>
          <Chart title="Gate pass rate % / day">
            <LineChart data={gateRateSeries}>
              <CartesianGrid strokeDasharray="3 3" className="stroke-border" />
              <XAxis dataKey="date" tick={{ fontSize: 10 }} />
              <YAxis domain={[0, 100]} tick={{ fontSize: 10 }} />
              <Tooltip />
              <Line type="monotone" dataKey="rate" stroke="currentColor" dot={false} />
            </LineChart>
          </Chart>
          <Chart title="Fix PRs / day">
            <BarChart data={trends?.fix_prs ?? []}>
              <CartesianGrid strokeDasharray="3 3" className="stroke-border" />
              <XAxis dataKey="date" tick={{ fontSize: 10 }} />
              <YAxis tick={{ fontSize: 10 }} allowDecimals={false} />
              <Tooltip />
              <Bar dataKey="count" fill="currentColor" />
            </BarChart>
          </Chart>
        </div>
      </section>
    </div>
  );
}
