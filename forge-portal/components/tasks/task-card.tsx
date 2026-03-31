"use client";

import Link from "next/link";
import { Badge } from "@/components/ui/badge";
import { Task, STATUS_LABELS, STATUS_COLORS } from "@/lib/tasks";

interface TaskCardProps {
  task: Task;
  projectId: string;
}

export function TaskCard({ task, projectId }: TaskCardProps) {
  const color = STATUS_COLORS[task.status] || "#8888A0";

  return (
    <Link href={`/projects/${projectId}/tasks/${task.id}`} className="block">
      <div className="rounded-lg border border-border bg-card p-3 hover:border-primary/40 transition-colors">
        <div className="flex items-start justify-between gap-2 mb-2">
          <p className="text-sm font-medium text-foreground line-clamp-2 flex-1">
            {task.title || task.requirement}
          </p>
          <Badge
            variant="secondary"
            className="shrink-0 text-xs py-0"
            style={{ color, borderColor: `${color}40` }}
          >
            {STATUS_LABELS[task.status] || task.status}
          </Badge>
        </div>
        <p className="text-xs text-muted-foreground line-clamp-1">
          {task.requirement}
        </p>
        <p className="text-xs text-muted-foreground mt-2">
          {new Date(task.created_at).toLocaleDateString("zh-CN", {
            month: "short",
            day: "numeric",
            hour: "2-digit",
            minute: "2-digit",
          })}
        </p>
      </div>
    </Link>
  );
}
