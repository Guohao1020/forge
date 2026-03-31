"use client";

import { TaskStep, STATUS_COLORS } from "@/lib/tasks";
import { CheckCircle2, Circle, Loader2, XCircle, SkipForward } from "lucide-react";

interface StepTimelineProps {
  steps: TaskStep[];
}

const stepIcons: Record<string, React.ReactNode> = {
  PENDING: <Circle className="h-4 w-4 text-muted-foreground" />,
  RUNNING: <Loader2 className="h-4 w-4 animate-spin text-primary" />,
  COMPLETED: <CheckCircle2 className="h-4 w-4 text-green-500" />,
  FAILED: <XCircle className="h-4 w-4 text-destructive" />,
  SKIPPED: <SkipForward className="h-4 w-4 text-muted-foreground" />,
};

export function StepTimeline({ steps }: StepTimelineProps) {
  return (
    <div className="space-y-0">
      {steps.map((step, i) => {
        const color = STATUS_COLORS[step.status === "RUNNING" ? "ANALYZING" : step.status] || "#8888A0";
        const isLast = i === steps.length - 1;
        return (
          <div key={step.id} className="flex gap-3">
            {/* Timeline line + icon */}
            <div className="flex flex-col items-center">
              <div className="mt-1">{stepIcons[step.status] || stepIcons.PENDING}</div>
              {!isLast && (
                <div className="w-px flex-1 min-h-[24px] bg-border" />
              )}
            </div>

            {/* Content */}
            <div className="pb-4 flex-1 min-w-0">
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
              {step.started_at && (
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
