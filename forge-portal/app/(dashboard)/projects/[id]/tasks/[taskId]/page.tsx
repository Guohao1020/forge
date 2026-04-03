"use client";

import { useEffect, useState, useCallback, useRef } from "react";
import { useParams } from "next/navigation";
import Link from "next/link";
import { ArrowLeft, Wifi, ExternalLink, Globe } from "lucide-react";
import { Badge } from "@/components/ui/badge";
import { StepTimeline } from "@/components/tasks/step-timeline";
import { TaskWorkspace } from "@/components/tasks/task-workspace";
import { getTaskDetail, TaskDetail, TaskStep, STATUS_LABELS, STATUS_COLORS } from "@/lib/tasks";
import { useTaskStream, TaskStreamEvent } from "@/lib/use-task-stream";
import { getTaskPreview, PreviewEnvironment } from "@/lib/preview";

const TERMINAL_STATUSES = ["COMPLETED", "FAILED"];

/**
 * Pick the best step to auto-select:
 * 1. First RUNNING step
 * 2. Last COMPLETED step
 * 3. First step
 */
function pickDefaultStep(steps: TaskStep[]): TaskStep | null {
  if (!steps.length) return null;
  const running = steps.find((s) => s.status === "RUNNING");
  if (running) return running;
  const completed = [...steps].reverse().find((s) => s.status === "COMPLETED");
  if (completed) return completed;
  return steps[0];
}

export default function TaskDetailPage() {
  const params = useParams();
  const projectId = params.id as string;
  const taskId = params.taskId as string;
  const [detail, setDetail] = useState<TaskDetail | null>(null);
  const [loading, setLoading] = useState(true);
  const [selectedStep, setSelectedStep] = useState<TaskStep | null>(null);
  const [previewEnv, setPreviewEnv] = useState<PreviewEnvironment | null>(null);
  // Track whether user has manually clicked a step
  const userSelectedRef = useRef(false);

  const fetchDetail = useCallback(async () => {
    try {
      const data = await getTaskDetail(projectId, taskId);
      setDetail(data);
      // Fetch preview environment for this task
      const preview = await getTaskPreview(projectId, taskId);
      setPreviewEnv(preview);
    } catch {
      // ignore
    } finally {
      setLoading(false);
    }
  }, [projectId, taskId]);

  const isTerminal = detail?.task ? TERMINAL_STATUSES.includes(detail.task.status) : false;

  const handleStreamEvent = useCallback((event: TaskStreamEvent) => {
    if (event.type === "TASK_PROGRESS" || event.type === "STEPS_UPDATE" || event.type === "TASK_COMPLETE" || event.type === "FULL_STATE") {
      fetchDetail();
    }
  }, [fetchDetail]);

  const { connected, streamingTokens, isStreaming } = useTaskStream({
    taskId,
    onEvent: handleStreamEvent,
    enabled: !isTerminal,
  });

  useEffect(() => {
    fetchDetail();
  }, [fetchDetail]);

  // Auto-select step when steps update (unless user has manually selected)
  useEffect(() => {
    if (!detail?.steps) return;
    const steps = detail.steps;

    if (!userSelectedRef.current) {
      // Auto-select: pick running or last completed
      const autoStep = pickDefaultStep(steps);
      setSelectedStep(autoStep);
    } else {
      // User has selected — update the step object to latest data but keep the selection
      setSelectedStep((prev) => {
        if (!prev) return prev;
        const updated = steps.find((s) => s.id === prev.id);
        return updated || prev;
      });
    }

    // If a step is RUNNING, auto-switch to it even if user previously selected
    const running = steps.find((s) => s.status === "RUNNING");
    if (running) {
      setSelectedStep(running);
      userSelectedRef.current = false;
    }
  }, [detail?.steps]);

  const handleStepClick = useCallback((step: TaskStep) => {
    userSelectedRef.current = true;
    setSelectedStep(step);
  }, []);

  if (loading) {
    return (
      <div className="flex h-[calc(100vh-64px)]">
        <div className="w-[280px] shrink-0 border-r border-white/10 p-4 space-y-4">
          <div className="h-6 w-32 rounded bg-card animate-pulse" />
          <div className="h-4 w-24 rounded bg-card animate-pulse" />
          <div className="space-y-3 mt-6">
            {[1, 2, 3, 4].map((i) => (
              <div key={i} className="h-10 rounded bg-card animate-pulse" />
            ))}
          </div>
        </div>
        <div className="flex-1 p-6">
          <div className="h-64 rounded-xl bg-card animate-pulse" />
        </div>
      </div>
    );
  }

  if (!detail) {
    return (
      <div className="flex items-center justify-center h-[calc(100vh-64px)]">
        <p className="text-muted-foreground">任务不存在</p>
      </div>
    );
  }

  const { task, steps } = detail;
  const color = STATUS_COLORS[task.status] || "#8888A0";

  return (
    <div className="flex h-[calc(100vh-64px)]">
      {/* Left panel: Timeline */}
      <div className="w-[280px] shrink-0 border-r border-white/10 overflow-y-auto">
        <div className="p-4">
          {/* Back link */}
          <Link
            href={`/projects/${projectId}`}
            className="flex items-center gap-1.5 text-xs text-muted-foreground hover:text-foreground transition-colors mb-4"
          >
            <ArrowLeft className="h-3.5 w-3.5" />
            返回任务列表
          </Link>

          {/* Task header */}
          <div className="mb-5">
            <h1 className="text-sm font-semibold leading-snug mb-2">
              {task.title || "任务详情"}
            </h1>
            <div className="flex items-center gap-2 flex-wrap">
              <Badge variant="secondary" className="text-[10px]" style={{ color, borderColor: `${color}40` }}>
                {STATUS_LABELS[task.status] || task.status}
              </Badge>
              <span className="text-[10px] text-muted-foreground">#{task.id}</span>
              {task.review_score != null && (
                <span className={`text-[10px] font-mono ${task.review_score >= 90 ? "text-emerald-400" : task.review_score >= 70 ? "text-yellow-400" : "text-red-400"}`}>
                  评分 {task.review_score}
                </span>
              )}
              {task.mr_url && (
                <a
                  href={task.mr_url}
                  target="_blank"
                  rel="noopener noreferrer"
                  className="flex items-center gap-1 text-[10px] text-primary hover:text-primary/80 transition-colors"
                >
                  <ExternalLink size={10} />
                  PR
                </a>
              )}
              {previewEnv?.previewUrl && previewEnv.status === "READY" && (
                <a
                  href={previewEnv.previewUrl}
                  target="_blank"
                  rel="noopener noreferrer"
                  className="flex items-center gap-1 text-[10px] text-emerald-400 hover:text-emerald-300 transition-colors"
                >
                  <Globe size={10} />
                  Preview
                </a>
              )}
              {!isTerminal && connected && (
                <span className="flex items-center gap-1 text-[10px] text-emerald-400">
                  <Wifi size={10} />
                  实时
                </span>
              )}
            </div>
          </div>

          {/* Step timeline */}
          <div className="mb-2">
            <h2 className="text-xs text-white/30 uppercase tracking-wide mb-3">执行步骤</h2>
            <StepTimeline
              steps={steps || []}
              selectedStepId={selectedStep?.id}
              onStepClick={handleStepClick}
            />
          </div>
        </div>
      </div>

      {/* Right panel: Workspace */}
      <div className="flex-1 overflow-y-auto p-6">
        <TaskWorkspace
          selectedStep={selectedStep}
          steps={steps || []}
          requirement={task.requirement}
          streamingTokens={streamingTokens}
          isStreaming={isStreaming}
        />
      </div>
    </div>
  );
}
