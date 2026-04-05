"use client";

import { useEffect, useState } from "react";
import Link from "next/link";
import { CheckCircle2, XCircle, Loader2, PlusCircle } from "lucide-react";
import { api } from "@/lib/api";

interface ActivityItem {
  type: string;
  projectId: number;
  projectName: string;
  title: string;
  status?: string;
  taskId?: number;
  timestamp: string;
}

const TYPE_CONFIG: Record<string, { icon: typeof CheckCircle2; color: string; label: string }> = {
  task_completed: { icon: CheckCircle2, color: "text-green-400", label: "完成" },
  task_failed: { icon: XCircle, color: "text-red-400", label: "失败" },
  task_running: { icon: Loader2, color: "text-primary", label: "进行中" },
  task_created: { icon: PlusCircle, color: "text-blue-400", label: "新建" },
};

export function ActivityFeed() {
  const [items, setItems] = useState<ActivityItem[]>([]);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    api.get<{ activity: ActivityItem[] }>("/activity?limit=10")
      .then((res) => setItems(res.activity))
      .catch(() => {})
      .finally(() => setLoading(false));
  }, []);

  if (loading) {
    return (
      <div className="space-y-2">
        {[1, 2, 3].map((i) => (
          <div key={i} className="h-12 rounded-lg bg-muted/50 animate-pulse" />
        ))}
      </div>
    );
  }

  if (items.length === 0) {
    return (
      <p className="text-sm text-muted-foreground text-center py-4">暂无活动记录</p>
    );
  }

  return (
    <div className="space-y-1.5">
      {items.map((item, idx) => {
        const cfg = TYPE_CONFIG[item.type] || TYPE_CONFIG.task_created;
        const Icon = cfg.icon;
        const href = item.taskId
          ? `/projects/${item.projectId}/tasks/${item.taskId}`
          : `/projects/${item.projectId}`;

        return (
          <Link
            key={idx}
            href={href}
            className="flex items-start gap-3 px-3 py-2 rounded-lg hover:bg-muted/50 transition-colors group"
          >
            <Icon className={`h-4 w-4 mt-0.5 shrink-0 ${cfg.color}`} />
            <div className="flex-1 min-w-0">
              <p className="text-sm text-foreground truncate group-hover:text-primary transition-colors">
                {item.title}
              </p>
              <div className="flex items-center gap-2 text-[10px] text-muted-foreground mt-0.5">
                <span>{item.projectName}</span>
                <span className="text-muted-foreground/30">|</span>
                <span>{formatRelativeTime(item.timestamp)}</span>
              </div>
            </div>
            <span className={`shrink-0 px-1.5 py-0.5 rounded text-[9px] ${cfg.color} bg-muted/50`}>
              {cfg.label}
            </span>
          </Link>
        );
      })}
    </div>
  );
}

function formatRelativeTime(timestamp: string): string {
  const now = Date.now();
  const then = new Date(timestamp).getTime();
  const diff = now - then;

  const minutes = Math.floor(diff / 60000);
  if (minutes < 1) return "刚刚";
  if (minutes < 60) return `${minutes} 分钟前`;

  const hours = Math.floor(minutes / 60);
  if (hours < 24) return `${hours} 小时前`;

  const days = Math.floor(hours / 24);
  if (days < 30) return `${days} 天前`;

  return new Date(timestamp).toLocaleDateString("zh-CN");
}
