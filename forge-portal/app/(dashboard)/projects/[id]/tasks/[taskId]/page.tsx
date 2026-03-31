"use client";

import { useEffect, useState, useCallback } from "react";
import { useParams } from "next/navigation";
import Link from "next/link";
import { ArrowLeft } from "lucide-react";
import { Badge } from "@/components/ui/badge";
import { StepTimeline } from "@/components/tasks/step-timeline";
import { getTaskDetail, TaskDetail, STATUS_LABELS, STATUS_COLORS } from "@/lib/tasks";

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

  useEffect(() => {
    fetchDetail();
    const interval = setInterval(fetchDetail, 3000);
    return () => clearInterval(interval);
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
    <div className="max-w-2xl">
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
        <p className="text-sm text-muted-foreground">#{task.id} · 创建于 {new Date(task.created_at).toLocaleString("zh-CN")}</p>
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
    </div>
  );
}
