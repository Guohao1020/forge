"use client";

import { useState } from "react";
import { Badge } from "@/components/ui/badge";
import {
  Collapsible,
  CollapsibleTrigger,
  CollapsibleContent,
} from "@/components/ui/collapsible";
import { CheckCircle2, Circle, ChevronRight, Clock, GitBranch, FileCode2 } from "lucide-react";

interface DagTask {
  order: number;
  title: string;
  description?: string;
  type?: string;
  files?: string[];
  depends_on?: number[];
  estimate_hours?: number;
  requirement_ref?: string;
  status?: string;
}

interface DagTaskListProps {
  tasks: DagTask[];
  totalEstimateHours?: number;
  parallelTracks?: number;
}

const TYPE_STYLES: Record<string, string> = {
  BACKEND: "bg-cyan-500/10 text-cyan-400 border-cyan-500/20",
  FRONTEND: "bg-violet-500/10 text-violet-400 border-violet-500/20",
  SCHEMA: "bg-amber-500/10 text-amber-400 border-amber-500/20",
  CONFIG: "bg-slate-500/10 text-slate-400 border-slate-500/20",
  TEST: "bg-emerald-500/10 text-emerald-400 border-emerald-500/20",
};

function StatusIcon({ status }: { status?: string }) {
  switch (status) {
    case "COMPLETED":
      return <CheckCircle2 className="w-4 h-4 text-emerald-400" />;
    case "RUNNING":
      return <Circle className="w-4 h-4 text-cyan-400 animate-pulse" />;
    case "READY":
      return <Circle className="w-4 h-4 text-violet-400" />;
    case "SKIPPED":
      return <Circle className="w-4 h-4 text-white/20" />;
    default:
      return <Circle className="w-4 h-4 text-white/20" />;
  }
}

export function DagTaskList({ tasks, totalEstimateHours, parallelTracks }: DagTaskListProps) {
  const [expandedNodes, setExpandedNodes] = useState<Set<number>>(new Set());

  const toggleNode = (order: number) => {
    setExpandedNodes((prev) => {
      const next = new Set(prev);
      if (next.has(order)) {
        next.delete(order);
      } else {
        next.add(order);
      }
      return next;
    });
  };

  // Compute stats from tasks if not provided
  const computedTotal =
    totalEstimateHours ??
    tasks.reduce((sum, t) => sum + (t.estimate_hours || 0), 0);

  const computedParallel =
    parallelTracks ??
    (() => {
      const noDeps = tasks.filter(
        (t) => !t.depends_on || t.depends_on.length === 0
      ).length;
      return Math.max(1, noDeps);
    })();

  return (
    <div className="space-y-2">
      {tasks.map((task, idx) => {
        const isLast = idx === tasks.length - 1;
        const hasDetails =
          (task.files && task.files.length > 0) || task.description;
        const isExpanded = expandedNodes.has(task.order);

        return (
          <Collapsible
            key={task.order}
            open={isExpanded}
            onOpenChange={() => hasDetails && toggleNode(task.order)}
          >
            <div className="relative">
              {/* Vertical connector line */}
              {!isLast && (
                <div className="absolute left-[15px] top-[32px] bottom-0 w-px bg-white/10" />
              )}

              {/* Main row */}
              <CollapsibleTrigger
                className={`w-full text-left flex items-start gap-3 p-3 rounded-lg bg-white/[0.02] border border-white/5 hover:border-white/10 transition-colors ${
                  hasDetails ? "cursor-pointer" : "cursor-default"
                }`}
              >
                {/* Order circle */}
                <span className="shrink-0 w-7 h-7 rounded-full bg-primary/10 text-primary text-xs font-medium flex items-center justify-center mt-0.5">
                  {task.order}
                </span>

                {/* Content */}
                <div className="flex-1 min-w-0">
                  <div className="flex items-center gap-2">
                    <p className="text-sm text-white/80 truncate">{task.title}</p>
                    {hasDetails && (
                      <ChevronRight
                        className={`w-3 h-3 text-white/30 transition-transform shrink-0 ${
                          isExpanded ? "rotate-90" : ""
                        }`}
                      />
                    )}
                  </div>

                  {/* Dependency + requirement info */}
                  <div className="flex flex-wrap gap-x-4 gap-y-1 mt-1">
                    {task.depends_on && task.depends_on.length > 0 && (
                      <span className="text-[11px] text-white/30 flex items-center gap-1">
                        <GitBranch className="w-3 h-3" />
                        依赖: [{task.depends_on.join(", ")}]
                      </span>
                    )}
                    {task.requirement_ref && (
                      <span className="text-[11px] text-white/30">
                        需求: {task.requirement_ref}
                      </span>
                    )}
                  </div>
                </div>

                {/* Right side: type badge + estimate + status */}
                <div className="flex items-center gap-2 shrink-0">
                  {task.type && (
                    <Badge
                      variant="secondary"
                      className={`text-[10px] px-1.5 py-0 h-5 ${
                        TYPE_STYLES[task.type] || TYPE_STYLES.BACKEND
                      }`}
                    >
                      {task.type}
                    </Badge>
                  )}
                  {task.estimate_hours != null && (
                    <span className="text-[11px] text-white/30 flex items-center gap-0.5">
                      <Clock className="w-3 h-3" />
                      {task.estimate_hours}h
                    </span>
                  )}
                  <StatusIcon status={task.status} />
                </div>
              </CollapsibleTrigger>

              {/* Expandable details */}
              {hasDetails && (
                <CollapsibleContent className="pl-10 pr-3 pb-1">
                  <div className="pt-2 space-y-2">
                    {task.description && (
                      <p className="text-xs text-white/40 leading-relaxed">
                        {task.description}
                      </p>
                    )}
                    {task.files && task.files.length > 0 && (
                      <div className="space-y-1">
                        <p className="text-[11px] text-white/30 flex items-center gap-1">
                          <FileCode2 className="w-3 h-3" />
                          涉及文件
                        </p>
                        <ul className="space-y-0.5">
                          {task.files.map((file, fi) => (
                            <li
                              key={fi}
                              className="text-[11px] text-white/30 font-mono pl-4"
                            >
                              {file}
                            </li>
                          ))}
                        </ul>
                      </div>
                    )}
                  </div>
                </CollapsibleContent>
              )}
            </div>
          </Collapsible>
        );
      })}

      {/* Bottom stats */}
      <div className="flex items-center gap-4 pt-3 border-t border-white/5">
        <span className="text-xs text-white/40 flex items-center gap-1">
          <Clock className="w-3.5 h-3.5" />
          总工时 {computedTotal} 小时
        </span>
        <span className="text-xs text-white/40 flex items-center gap-1">
          <GitBranch className="w-3.5 h-3.5" />
          可并行 {computedParallel} 条
        </span>
      </div>
    </div>
  );
}
