"use client";

import { useEffect, useState, useCallback } from "react";
import { useParams } from "next/navigation";
import Link from "next/link";
import { ArrowLeft, Wifi, AlertTriangle, AlertCircle, Info } from "lucide-react";
import { Badge } from "@/components/ui/badge";
import { StepTimeline } from "@/components/tasks/step-timeline";
import { CodePreviewPanel } from "@/components/code-preview/code-preview-panel";
import { getTaskDetail, TaskDetail, TaskStep, STATUS_LABELS, STATUS_COLORS } from "@/lib/tasks";
import { useTaskStream, TaskStreamEvent } from "@/lib/use-task-stream";

const TERMINAL_STATUSES = ["COMPLETED", "FAILED"];

const SEVERITY_STYLES: Record<string, string> = {
  ERROR: "bg-red-500/10 text-red-400 border-red-500/20",
  WARNING: "bg-yellow-500/10 text-yellow-400 border-yellow-500/20",
  INFO: "bg-blue-500/10 text-blue-400 border-blue-500/20",
};

const SEVERITY_ICONS: Record<string, React.ReactNode> = {
  ERROR: <AlertCircle className="h-3.5 w-3.5 text-red-400" />,
  WARNING: <AlertTriangle className="h-3.5 w-3.5 text-yellow-400" />,
  INFO: <Info className="h-3.5 w-3.5 text-blue-400" />,
};

interface ReviewFinding {
  severity: string;
  file: string;
  message: string;
  line?: number;
}

interface GenerateOutput {
  files: { path: string; content: string; action: string; language?: string }[];
  commitMessage?: string;
  filesChanged?: number;
  linesAdded?: number;
  linesDeleted?: number;
}

function tryParseOutput<T>(step: TaskStep): T | null {
  if (!step.output) return null;
  try {
    return JSON.parse(step.output) as T;
  } catch {
    return null;
  }
}

export default function TaskDetailPage() {
  const params = useParams();
  const projectId = params.id as string;
  const taskId = params.taskId as string;
  const [detail, setDetail] = useState<TaskDetail | null>(null);
  const [loading, setLoading] = useState(true);

  const fetchDetail = useCallback(async () => {
    try {
      const data = await getTaskDetail(projectId, taskId);
      setDetail(data);
    } catch {
      // ignore
    } finally {
      setLoading(false);
    }
  }, [projectId, taskId]);

  const isTerminal = detail?.task ? TERMINAL_STATUSES.includes(detail.task.status) : false;

  const handleStreamEvent = useCallback((event: TaskStreamEvent) => {
    if (event.type === "TASK_PROGRESS" || event.type === "STEPS_UPDATE" || event.type === "TASK_COMPLETE" || event.type === "FULL_STATE") {
      // Refetch full state on any meaningful event
      fetchDetail();
    }
  }, [fetchDetail]);

  const { connected } = useTaskStream({
    taskId,
    onEvent: handleStreamEvent,
    enabled: !isTerminal,
  });

  useEffect(() => {
    fetchDetail();
  }, [fetchDetail]);

  if (loading) {
    return (
      <div className="space-y-4">
        <div className="h-8 w-48 rounded-lg bg-card animate-pulse" />
        <div className="h-64 rounded-xl bg-card animate-pulse" />
      </div>
    );
  }

  if (!detail) {
    return <p className="text-muted-foreground">任务不存在</p>;
  }

  const { task, steps } = detail;
  const color = STATUS_COLORS[task.status] || "#8888A0";

  return (
    <div className="max-w-4xl">
      <Link
        href={`/projects/${projectId}`}
        className="flex items-center gap-1.5 text-sm text-muted-foreground hover:text-foreground transition-colors mb-4"
      >
        <ArrowLeft className="h-4 w-4" />
        返回任务列表
      </Link>

      {/* Header */}
      <div className="mb-6">
        <div className="flex items-center gap-3 mb-2">
          <h1 className="text-xl font-semibold">{task.title || "任务详情"}</h1>
          <Badge variant="secondary" style={{ color, borderColor: `${color}40` }}>
            {STATUS_LABELS[task.status] || task.status}
          </Badge>
        </div>
        <div className="flex items-center gap-3 text-sm text-muted-foreground">
          <span>#{task.id} · 创建于 {new Date(task.created_at).toLocaleString("zh-CN")}</span>
          {!isTerminal && connected && (
            <span className="flex items-center gap-1 text-xs text-emerald-400">
              <Wifi size={12} />
              实时更新中
            </span>
          )}
        </div>
      </div>

      {/* Requirement */}
      <div className="rounded-xl border border-border bg-card p-5 mb-6">
        <h2 className="text-sm font-medium mb-2">需求描述</h2>
        <p className="text-sm text-foreground whitespace-pre-wrap">{task.requirement}</p>
      </div>

      {/* Step Timeline */}
      <div className="rounded-xl border border-border bg-card p-5">
        <h2 className="text-sm font-medium mb-4">执行步骤</h2>
        <StepTimeline steps={steps || []} />
      </div>

      {/* Code Preview — for completed GENERATE steps */}
      {(steps || [])
        .filter((s) => s.step_type === "GENERATE" && s.status === "COMPLETED" && s.output)
        .map((step) => {
          const output = tryParseOutput<GenerateOutput>(step);
          if (!output?.files?.length) return null;
          return (
            <div key={`gen-${step.id}`} className="mt-6">
              <h2 className="text-sm font-medium mb-3">生成代码预览</h2>
              <CodePreviewPanel
                files={output.files}
                commitMessage={output.commitMessage}
                filesChanged={output.filesChanged}
                linesAdded={output.linesAdded}
                linesDeleted={output.linesDeleted}
              />
            </div>
          );
        })}

      {/* Review Findings — for completed REVIEW steps */}
      {(steps || [])
        .filter((s) => s.step_type === "REVIEW" && s.status === "COMPLETED" && s.output)
        .map((step) => {
          const findings = tryParseOutput<ReviewFinding[]>(step);
          if (!findings?.length) return null;
          return (
            <div key={`rev-${step.id}`} className="rounded-xl border border-border bg-card p-5 mt-6">
              <h2 className="text-sm font-medium mb-3">审查结果</h2>
              <div className="space-y-2">
                {findings.map((f, i) => (
                  <div
                    key={i}
                    className="flex items-start gap-2 p-2 rounded-lg bg-white/[0.02] border border-white/5"
                  >
                    {SEVERITY_ICONS[f.severity] || SEVERITY_ICONS.INFO}
                    <div className="flex-1 min-w-0">
                      <div className="flex items-center gap-2">
                        <span
                          className={`px-1.5 py-0.5 rounded text-[10px] border ${
                            SEVERITY_STYLES[f.severity] || SEVERITY_STYLES.INFO
                          }`}
                        >
                          {f.severity}
                        </span>
                        <span className="text-xs text-white/50 font-mono truncate">
                          {f.file}
                          {f.line != null && `:${f.line}`}
                        </span>
                      </div>
                      <p className="text-sm text-white/70 mt-1">{f.message}</p>
                    </div>
                  </div>
                ))}
              </div>
            </div>
          );
        })}
    </div>
  );
}
