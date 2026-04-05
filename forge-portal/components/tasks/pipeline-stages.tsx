"use client";

import { CheckCircle2, Loader2, Circle, XCircle } from "lucide-react";

const STAGES = [
  { key: "ANALYZE", label: "分析", color: "text-purple-400" },
  { key: "PLAN", label: "规划", color: "text-indigo-400" },
  { key: "TEST_WRITING", label: "测试", color: "text-cyan-400" },
  { key: "GENERATE", label: "生成", color: "text-blue-400" },
  { key: "LINT", label: "检查", color: "text-yellow-400" },
  { key: "REVIEW", label: "审查", color: "text-orange-400" },
  { key: "TEST", label: "验证", color: "text-green-400" },
  { key: "DEPLOY", label: "部署", color: "text-emerald-400" },
];

interface PipelineStagesProps {
  currentStage?: string;
  completedStages?: string[];
  failedStage?: string;
}

export function PipelineStages({
  currentStage,
  completedStages = [],
  failedStage,
}: PipelineStagesProps) {
  const completedSet = new Set(completedStages);

  return (
    <div className="flex items-center gap-1">
      {STAGES.map((stage, i) => {
        const isCompleted = completedSet.has(stage.key);
        const isCurrent = stage.key === currentStage;
        const isFailed = stage.key === failedStage;
        const isPending = !isCompleted && !isCurrent && !isFailed;

        return (
          <div key={stage.key} className="flex items-center">
            {i > 0 && (
              <div className={`w-4 h-px mx-0.5 ${
                isCompleted ? "bg-green-500/50" : "bg-border"
              }`} />
            )}
            <div className="flex flex-col items-center gap-0.5" title={stage.label}>
              {isCompleted && (
                <CheckCircle2 className="h-4 w-4 text-green-400" />
              )}
              {isCurrent && (
                <Loader2 className={`h-4 w-4 ${stage.color} animate-spin`} />
              )}
              {isFailed && (
                <XCircle className="h-4 w-4 text-red-400" />
              )}
              {isPending && (
                <Circle className="h-4 w-4 text-muted-foreground/40" />
              )}
              <span className={`text-[8px] ${
                isCompleted ? "text-green-400/70" :
                isCurrent ? stage.color :
                isFailed ? "text-red-400/70" :
                "text-muted-foreground/40"
              }`}>
                {stage.label}
              </span>
            </div>
          </div>
        );
      })}
    </div>
  );
}
