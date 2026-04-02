"use client";

import { TaskStep, STATUS_COLORS } from "@/lib/tasks";
import { CheckCircle2, Circle, Loader2, XCircle, SkipForward } from "lucide-react";

interface StepTimelineProps {
  steps: TaskStep[];
  selectedStepId?: number;
  onStepClick?: (step: TaskStep) => void;
}

const stepIcons: Record<string, React.ReactNode> = {
  PENDING: <Circle className="h-4 w-4 text-muted-foreground" />,
  RUNNING: <Loader2 className="h-4 w-4 animate-spin text-primary" />,
  COMPLETED: <CheckCircle2 className="h-4 w-4 text-green-500" />,
  FAILED: <XCircle className="h-4 w-4 text-destructive" />,
  SKIPPED: <SkipForward className="h-4 w-4 text-muted-foreground" />,
};

function getStepSummary(step: TaskStep): string | null {
  if (step.status !== "COMPLETED" || !step.output) return null;
  try {
    const data = JSON.parse(step.output);
    switch (step.step_type) {
      case "PLAN": {
        const count = Array.isArray(data.tasks) ? data.tasks.length : 0;
        return count > 0 ? `拆解为 ${count} 个子任务` : null;
      }
      case "GENERATE": {
        const count = Array.isArray(data.files) ? data.files.length : 0;
        return count > 0 ? `生成 ${count} 个文件` : null;
      }
      case "REVIEW": {
        return data.score != null ? `评分 ${data.score} 分` : null;
      }
      case "TEST_WRITING": {
        const count = data.test_count ?? (Array.isArray(data.test_files) ? data.test_files.length : 0);
        const fw = data.framework ?? "";
        return count > 0 ? `生成 ${count} 个测试用例${fw ? ` (${fw})` : ""}` : null;
      }
      default:
        return null;
    }
  } catch {
    return null;
  }
}

export function StepTimeline({ steps, selectedStepId, onStepClick }: StepTimelineProps) {
  return (
    <div className="space-y-0">
      {steps.map((step, i) => {
        const color = STATUS_COLORS[step.status === "RUNNING" ? "ANALYZING" : step.status] || "#8888A0";
        const isLast = i === steps.length - 1;
        const isSelected = selectedStepId === step.id;
        const isClickable = !!onStepClick;
        const summary = getStepSummary(step);

        return (
          <div
            key={step.id}
            className={`flex gap-3 rounded-lg transition-colors ${
              isClickable ? "cursor-pointer hover:bg-white/[0.03]" : ""
            } ${isSelected ? "bg-white/[0.03] border-l-2 border-primary pl-2" : "pl-[10px]"}`}
            onClick={() => onStepClick?.(step)}
          >
            {/* Timeline line + icon */}
            <div className="flex flex-col items-center">
              <div className="mt-2.5">{stepIcons[step.status] || stepIcons.PENDING}</div>
              {!isLast && (
                <div className="w-px flex-1 min-h-[24px] bg-border" />
              )}
            </div>

            {/* Content */}
            <div className="pb-3 pt-1.5 flex-1 min-w-0">
              <div className="flex items-center gap-2">
                <span className="text-sm font-medium" style={{ color: step.status === "RUNNING" ? color : undefined }}>
                  {step.name}
                </span>
                {step.duration_ms != null && (
                  <span className="text-xs text-muted-foreground">
                    {(step.duration_ms / 1000).toFixed(1)}s
                  </span>
                )}
              </div>
              {summary && (
                <p className="text-xs text-white/40 mt-0.5">{summary}</p>
              )}
              {!summary && step.started_at && (
                <p className="text-xs text-muted-foreground mt-0.5">
                  {new Date(step.started_at).toLocaleTimeString("zh-CN", {
                    hour: "2-digit",
                    minute: "2-digit",
                    second: "2-digit",
                  })}
                  {step.completed_at && (
                    <> → {new Date(step.completed_at).toLocaleTimeString("zh-CN", {
                      hour: "2-digit",
                      minute: "2-digit",
                      second: "2-digit",
                    })}</>
                  )}
                </p>
              )}
            </div>
          </div>
        );
      })}
    </div>
  );
}
