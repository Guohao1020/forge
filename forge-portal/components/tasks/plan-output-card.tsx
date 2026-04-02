"use client";

import { Badge } from "@/components/ui/badge";
import { DagTaskList } from "./dag-task-list";
import { Clock, Container, GitBranch } from "lucide-react";

interface PlanTask {
  order: number;
  title: string;
  files?: string[];
  type?: string;
  description?: string;
  depends_on?: number[];
  estimate_hours?: number;
  requirement_ref?: string;
  status?: string;
}

interface PlanOutputCardProps {
  planOutput: {
    title?: string;
    tasks?: PlanTask[];
    risk_level?: string;
    risk_factors?: string[];
    total_estimate_hours?: number;
    parallel_tracks?: number;
  };
}

const RISK_STYLES: Record<string, string> = {
  LOW: "bg-emerald-500/10 text-emerald-400 border-emerald-500/20",
  MEDIUM: "bg-yellow-500/10 text-yellow-400 border-yellow-500/20",
  HIGH: "bg-red-500/10 text-red-400 border-red-500/20",
};

function hasDagData(tasks: PlanTask[]): boolean {
  return tasks.some(
    (t) => t.depends_on !== undefined && Array.isArray(t.depends_on)
  );
}

export function PlanOutputCard({ planOutput }: PlanOutputCardProps) {
  const { title, tasks, risk_level, risk_factors, total_estimate_hours, parallel_tracks } = planOutput;

  const isDag = tasks && tasks.length > 0 && hasDagData(tasks);

  const hasDocker = tasks?.some(
    (t) =>
      t.type === "CONFIG" && t.files?.some((f) => f.toLowerCase().includes("dockerfile"))
      || t.files?.some((f) => f.toLowerCase().includes("dockerfile"))
  ) ?? false;

  return (
    <div className="rounded-xl border border-white/10 bg-card p-5 space-y-4">
      {/* Header */}
      <div className="flex items-center justify-between">
        <h3 className="text-sm font-medium">实施方案</h3>
        <div className="flex items-center gap-2">
          {hasDocker && (
            <Badge
              variant="secondary"
              className="bg-sky-500/10 text-sky-400 border-sky-500/20 flex items-center gap-1"
            >
              <Container className="h-3 w-3" />
              Docker
            </Badge>
          )}
          {/* Estimate + parallel stats in header for DAG mode */}
          {isDag && total_estimate_hours != null && (
            <span className="text-[11px] text-white/40 flex items-center gap-1">
              <Clock className="w-3 h-3" />
              {total_estimate_hours}h
            </span>
          )}
          {isDag && parallel_tracks != null && (
            <span className="text-[11px] text-white/40 flex items-center gap-1">
              <GitBranch className="w-3 h-3" />
              {parallel_tracks} 并行
            </span>
          )}
          {risk_level && (
            <Badge
              variant="secondary"
              className={RISK_STYLES[risk_level] || RISK_STYLES.MEDIUM}
            >
              {risk_level === "LOW" ? "低风险" : risk_level === "MEDIUM" ? "中风险" : "高风险"}
            </Badge>
          )}
        </div>
      </div>

      {/* Title */}
      {title && (
        <p className="text-sm text-white/70">{title}</p>
      )}

      {/* Task list: DAG or simple */}
      {tasks && tasks.length > 0 && (
        isDag ? (
          <DagTaskList
            tasks={tasks}
            totalEstimateHours={total_estimate_hours}
            parallelTracks={parallel_tracks}
          />
        ) : (
          /* Legacy simple list (no depends_on) */
          <div className="space-y-2">
            {tasks.map((task) => (
              <div
                key={task.order}
                className="flex items-start gap-3 p-3 rounded-lg bg-white/[0.02] border border-white/5"
              >
                <span className="shrink-0 w-6 h-6 rounded-full bg-primary/10 text-primary text-xs font-medium flex items-center justify-center mt-0.5">
                  {task.order}
                </span>
                <div className="flex-1 min-w-0">
                  <p className="text-sm text-white/80">{task.title}</p>
                  {task.files && task.files.length > 0 && (
                    <p className="text-xs text-white/40 mt-1">
                      涉及 {task.files.length} 个文件
                    </p>
                  )}
                </div>
                {task.type && (
                  <span className="text-[10px] text-white/30 uppercase tracking-wide">
                    {task.type}
                  </span>
                )}
              </div>
            ))}
          </div>
        )
      )}

      {/* Risk factors */}
      {risk_factors && risk_factors.length > 0 && (
        <div className="pt-2 border-t border-white/5">
          <p className="text-xs text-white/30 mb-1.5">风险因素</p>
          <ul className="space-y-1">
            {risk_factors.map((factor, i) => (
              <li key={i} className="text-xs text-white/40 flex items-start gap-1.5">
                <span className="text-white/20 mt-0.5">•</span>
                {factor}
              </li>
            ))}
          </ul>
        </div>
      )}
    </div>
  );
}
