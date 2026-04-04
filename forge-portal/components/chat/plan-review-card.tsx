"use client";

import { useState } from "react";
import { Button } from "@/components/ui/button";
import { CheckCircle, Edit3, X, ListTree, Clock, AlertTriangle, Layers, GitBranch } from "lucide-react";
import { DagVisualization } from "@/components/tasks/dag-visualization";

interface PlanTask {
  order: number;
  title: string;
  description?: string;
  type: string;
  files?: string[];
  depends_on?: number[];
  estimate_hours?: number;
  requirement_ref?: string;
}

interface PlanReviewCardProps {
  planData: {
    title?: string;
    tasks?: PlanTask[];
    risk_level?: string;
    risk_factors?: string[];
    total_estimate_hours?: number;
    parallel_tracks?: number;
  };
  onApprove: () => void;
  onRequestChanges: () => void;
  onCancel: () => void;
  isLoading?: boolean;
}

const riskColors: Record<string, string> = {
  LOW: "bg-green-500/10 text-green-400 border-green-500/20",
  MEDIUM: "bg-yellow-500/10 text-yellow-400 border-yellow-500/20",
  HIGH: "bg-red-500/10 text-red-400 border-red-500/20",
};

const typeColors: Record<string, string> = {
  BACKEND: "text-blue-400 bg-blue-500/10 border-blue-500/20",
  FRONTEND: "text-purple-400 bg-purple-500/10 border-purple-500/20",
  SCHEMA: "text-amber-400 bg-amber-500/10 border-amber-500/20",
  CONFIG: "text-gray-400 bg-gray-500/10 border-gray-500/20",
  TEST: "text-emerald-400 bg-emerald-500/10 border-emerald-500/20",
};

export function PlanReviewCard({
  planData,
  onApprove,
  onRequestChanges,
  onCancel,
  isLoading = false,
}: PlanReviewCardProps) {
  const { tasks = [], risk_level, risk_factors = [], total_estimate_hours, parallel_tracks } = planData;
  const [viewMode, setViewMode] = useState<"dag" | "list">("dag");

  return (
    <div className="border border-white/10 rounded-xl bg-white/[0.03] p-5 my-4">
      <div className="flex items-center gap-2 mb-3">
        <ListTree className="h-5 w-5 text-[#8B5CF6]" />
        <h3 className="text-sm font-semibold text-white">方案审查</h3>
      </div>

      {planData.title && (
        <h4 className="text-base font-medium text-white mb-3">{planData.title}</h4>
      )}

      {/* Summary badges */}
      <div className="flex items-center gap-2 flex-wrap mb-4">
        {risk_level && (
          <span className={`px-2 py-0.5 rounded text-xs border ${riskColors[risk_level] || riskColors.MEDIUM}`}>
            风险：{risk_level}
          </span>
        )}
        {total_estimate_hours != null && (
          <span className="flex items-center gap-1 px-2 py-0.5 rounded text-xs bg-white/5 text-white/50 border border-white/10">
            <Clock className="h-3 w-3" />
            {total_estimate_hours}h
          </span>
        )}
        {parallel_tracks != null && (
          <span className="flex items-center gap-1 px-2 py-0.5 rounded text-xs bg-white/5 text-white/50 border border-white/10">
            <Layers className="h-3 w-3" />
            {parallel_tracks} 条并行通道
          </span>
        )}
        <span className="px-2 py-0.5 rounded text-xs bg-white/5 text-white/50 border border-white/10">
          {tasks.length} 个子任务
        </span>
      </div>

      {/* View mode toggle */}
      {tasks.length > 0 && (
        <div className="flex gap-1 mb-3">
          <button
            onClick={() => setViewMode("dag")}
            className={`flex items-center gap-1 px-2 py-1 rounded text-xs transition-colors ${
              viewMode === "dag"
                ? "bg-primary/20 text-primary"
                : "text-white/40 hover:text-white/60"
            }`}
          >
            <GitBranch size={12} />
            依赖图
          </button>
          <button
            onClick={() => setViewMode("list")}
            className={`flex items-center gap-1 px-2 py-1 rounded text-xs transition-colors ${
              viewMode === "list"
                ? "bg-primary/20 text-primary"
                : "text-white/40 hover:text-white/60"
            }`}
          >
            <ListTree size={12} />
            列表
          </button>
        </div>
      )}

      {/* DAG visualization */}
      {tasks.length > 0 && viewMode === "dag" && (
        <div className="mb-4 border border-white/5 rounded-lg bg-white/[0.02] p-3">
          <DagVisualization
            tasks={tasks}
            touchedFiles={(planData as Record<string, unknown>).touched_files as { create?: string[]; modify?: string[] } | undefined}
          />
        </div>
      )}

      {/* Task list view */}
      {tasks.length > 0 && viewMode === "list" && (
        <div className="space-y-2 mb-4">
          {tasks.map((t) => (
            <div
              key={t.order}
              className="border border-white/5 rounded-lg bg-white/[0.02] p-3"
            >
              <div className="flex items-start gap-2">
                <span className="text-xs text-white/30 font-mono mt-0.5 shrink-0 w-5">
                  {t.order}.
                </span>
                <div className="flex-1 min-w-0">
                  <div className="flex items-center gap-2 flex-wrap">
                    <span className="text-sm font-medium text-white">{t.title}</span>
                    <span className={`px-1.5 py-0.5 rounded text-[10px] border ${typeColors[t.type] || "text-white/50 bg-white/5 border-white/10"}`}>
                      {t.type}
                    </span>
                    {t.estimate_hours != null && (
                      <span className="text-[10px] text-white/30">{t.estimate_hours}h</span>
                    )}
                  </div>
                  {t.description && (
                    <p className="text-xs text-white/40 mt-1">{t.description}</p>
                  )}
                  {t.files && t.files.length > 0 && (
                    <div className="flex flex-wrap gap-1 mt-1.5">
                      {t.files.map((f) => (
                        <span key={f} className="text-[10px] text-white/30 font-mono bg-white/5 px-1.5 py-0.5 rounded">
                          {f}
                        </span>
                      ))}
                    </div>
                  )}
                  {t.depends_on && t.depends_on.length > 0 && (
                    <span className="text-[10px] text-white/20 mt-1 block">
                      依赖: {t.depends_on.map((d) => `#${d}`).join(", ")}
                    </span>
                  )}
                </div>
              </div>
            </div>
          ))}
        </div>
      )}

      {/* Risk factors */}
      {risk_factors.length > 0 && (
        <div className="border border-white/5 rounded-lg bg-white/[0.02] p-2.5 mb-4">
          <div className="flex items-center gap-1.5 mb-1.5">
            <AlertTriangle className="h-3.5 w-3.5 text-yellow-500/60" />
            <span className="text-xs font-medium text-white/50">风险因素</span>
          </div>
          <ul className="space-y-1">
            {risk_factors.map((factor, idx) => (
              <li key={idx} className="text-xs text-white/40 pl-5 relative before:content-[''] before:absolute before:left-2 before:top-[7px] before:w-1 before:h-1 before:rounded-full before:bg-white/20">
                {factor}
              </li>
            ))}
          </ul>
        </div>
      )}

      {/* Action buttons */}
      <div className="flex gap-2">
        <Button
          onClick={onApprove}
          disabled={isLoading}
          className="bg-[#8B5CF6] hover:bg-[#7C3AED] text-white"
        >
          <CheckCircle className="h-4 w-4 mr-1.5" />
          {isLoading ? "正在启动..." : "批准方案并执行"}
        </Button>
        <Button variant="ghost" onClick={onRequestChanges} className="text-white/50">
          <Edit3 className="h-4 w-4 mr-1.5" />
          修改方案
        </Button>
        <Button variant="ghost" onClick={onCancel} className="text-red-400/60 hover:text-red-400">
          <X className="h-4 w-4 mr-1.5" />
          取消
        </Button>
      </div>
    </div>
  );
}
