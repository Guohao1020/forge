"use client";

import { useEffect, useState, useCallback } from "react";
import { useParams } from "next/navigation";
import { Rocket, Server, Clock } from "lucide-react";
import { Badge } from "@/components/ui/badge";
import { api } from "@/lib/api";

interface Environment {
  id: number;
  project_id: number;
  name: string;
  env_type: string;
  status: string;
  current_version?: string;
  last_deploy_at?: string;
  created_at: string;
  updated_at: string;
}

interface EnvironmentListResult {
  environments: Environment[];
}

const ENV_TYPE_LABELS: Record<string, string> = {
  DEV: "开发",
  STAGING: "预发布",
  PROD: "生产",
};

const ENV_TYPE_STYLES: Record<string, string> = {
  DEV: "bg-blue-500/10 text-blue-400 border-blue-500/20",
  STAGING: "bg-yellow-500/10 text-yellow-400 border-yellow-500/20",
  PROD: "bg-emerald-500/10 text-emerald-400 border-emerald-500/20",
};

function formatTime(dateStr?: string): string {
  if (!dateStr) return "—";
  const d = new Date(dateStr);
  return d.toLocaleString("zh-CN", {
    month: "2-digit",
    day: "2-digit",
    hour: "2-digit",
    minute: "2-digit",
  });
}

function LoadingSkeleton() {
  return (
    <div className="grid grid-cols-1 md:grid-cols-3 gap-4 animate-pulse">
      {Array.from({ length: 3 }).map((_, i) => (
        <div key={i} className="rounded-xl border border-white/10 bg-white/5 p-5">
          <div className="h-4 w-20 bg-white/5 rounded mb-4" />
          <div className="h-6 w-32 bg-white/5 rounded mb-3" />
          <div className="h-3 w-24 bg-white/5 rounded" />
        </div>
      ))}
    </div>
  );
}

function EmptyState() {
  return (
    <div className="flex flex-col items-center justify-center py-20 rounded-xl border border-border bg-card">
      <div className="w-12 h-12 rounded-xl flex items-center justify-center mb-3 bg-primary/10">
        <Rocket className="h-6 w-6 text-primary" />
      </div>
      <h3 className="text-base font-medium mb-1">暂无部署环境</h3>
      <p className="text-sm text-muted-foreground">
        配置项目后部署环境将在此展示
      </p>
    </div>
  );
}

export default function DeployPage() {
  const params = useParams();
  const projectId = params.id as string;
  const [loading, setLoading] = useState(true);
  const [environments, setEnvironments] = useState<Environment[]>([]);

  const fetchEnvironments = useCallback(async () => {
    try {
      setLoading(true);
      const result = await api.get<EnvironmentListResult>(
        `/projects/${projectId}/environments`
      );
      setEnvironments(result.environments || []);
    } catch (err) {
      console.error("Failed to fetch environments:", err);
      setEnvironments([]);
    } finally {
      setLoading(false);
    }
  }, [projectId]);

  useEffect(() => {
    fetchEnvironments();
  }, [fetchEnvironments]);

  if (loading) {
    return (
      <div>
        <h1 className="text-2xl font-semibold tracking-tight mb-6">部署</h1>
        <LoadingSkeleton />
      </div>
    );
  }

  if (environments.length === 0) {
    return (
      <div>
        <h1 className="text-2xl font-semibold tracking-tight mb-6">部署</h1>
        <EmptyState />
      </div>
    );
  }

  return (
    <div>
      <h1 className="text-2xl font-semibold tracking-tight mb-6">部署</h1>

      <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
        {environments.map((env) => {
          const isActive = env.status === "ACTIVE";
          return (
            <div
              key={env.id}
              className="rounded-xl border border-white/10 bg-white/5 p-5 hover:bg-white/[0.07] transition-colors"
            >
              {/* Header: name + type badge */}
              <div className="flex items-center justify-between mb-4">
                <div className="flex items-center gap-2">
                  <Server className="h-4 w-4 text-white/40" />
                  <span className="text-sm font-medium text-white/80">
                    {env.name}
                  </span>
                </div>
                <Badge
                  variant="secondary"
                  className={`text-[10px] ${ENV_TYPE_STYLES[env.env_type] || "bg-white/10 text-white/60 border-white/20"}`}
                >
                  {ENV_TYPE_LABELS[env.env_type] || env.env_type}
                </Badge>
              </div>

              {/* Status indicator */}
              <div className="flex items-center gap-2 mb-3">
                <span
                  className={`w-2 h-2 rounded-full ${isActive ? "bg-emerald-400" : "bg-white/20"}`}
                />
                <span
                  className={`text-xs ${isActive ? "text-emerald-400" : "text-white/30"}`}
                >
                  {isActive ? "运行中" : "未激活"}
                </span>
              </div>

              {/* Version */}
              <div className="mb-2">
                <p className="text-xs text-white/30 mb-1">当前版本</p>
                <p className="text-sm font-mono text-white/60">
                  {env.current_version || "—"}
                </p>
              </div>

              {/* Last deploy time */}
              <div className="flex items-center gap-1.5 text-xs text-white/30">
                <Clock className="h-3 w-3" />
                <span>
                  {env.last_deploy_at
                    ? `上次部署: ${formatTime(env.last_deploy_at)}`
                    : "暂无部署记录"}
                </span>
              </div>
            </div>
          );
        })}
      </div>
    </div>
  );
}
