"use client";

import { useEffect, useState, useCallback } from "react";
import { useParams } from "next/navigation";
import { Rocket, Server, Clock, Play, CheckCircle2, XCircle, Loader2, RotateCcw, Tag } from "lucide-react";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { api } from "@/lib/api";
import { listDeployRecords, triggerDeploy, type DeployRecord } from "@/lib/deploy";

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

const DEPLOY_STATUS_ICON: Record<string, React.ReactNode> = {
  PENDING: <Clock className="h-3 w-3 text-muted-foreground/60" />,
  DEPLOYING: <Loader2 className="h-3 w-3 text-blue-400 animate-spin" />,
  DEPLOYED: <CheckCircle2 className="h-3 w-3 text-emerald-400" />,
  FAILED: <XCircle className="h-3 w-3 text-red-400" />,
  ROLLED_BACK: <RotateCcw className="h-3 w-3 text-yellow-400" />,
};

const DEPLOY_STATUS_LABELS: Record<string, string> = {
  PENDING: "等待中",
  DEPLOYING: "部署中",
  DEPLOYED: "已部署",
  FAILED: "失败",
  ROLLED_BACK: "已回滚",
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
        <div key={i} className="rounded-xl border border-border bg-muted/50 p-5">
          <div className="h-4 w-20 bg-muted rounded mb-4" />
          <div className="h-6 w-32 bg-muted rounded mb-3" />
          <div className="h-3 w-24 bg-muted rounded" />
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

function DeployHistory({ projectId, envId }: { projectId: string; envId: number }) {
  const [records, setRecords] = useState<DeployRecord[]>([]);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    let cancelled = false;
    (async () => {
      try {
        const recs = await listDeployRecords(projectId, envId);
        if (!cancelled) setRecords(recs);
      } catch {
        if (!cancelled) setRecords([]);
      } finally {
        if (!cancelled) setLoading(false);
      }
    })();
    return () => { cancelled = true; };
  }, [projectId, envId]);

  if (loading) {
    return (
      <div className="mt-3 pt-3 border-t border-border/50 animate-pulse">
        <div className="h-3 w-24 bg-muted rounded mb-2" />
        <div className="h-3 w-40 bg-muted rounded" />
      </div>
    );
  }

  if (records.length === 0) {
    return (
      <div className="mt-3 pt-3 border-t border-border/50">
        <p className="text-xs text-muted-foreground/40">暂无部署记录</p>
      </div>
    );
  }

  return (
    <div className="mt-3 pt-3 border-t border-border/50 space-y-2">
      <p className="text-xs text-muted-foreground/60 mb-2">部署历史</p>
      {records.slice(0, 5).map((rec) => (
        <div key={rec.id} className="flex items-center gap-2 text-xs">
          {DEPLOY_STATUS_ICON[rec.status] || <Clock className="h-3 w-3 text-muted-foreground/40" />}
          <code className="font-mono text-muted-foreground">{rec.version}</code>
          <span className="text-muted-foreground/40">
            {DEPLOY_STATUS_LABELS[rec.status] || rec.status}
          </span>
          <span className="text-muted-foreground/30 ml-auto">{formatTime(rec.startedAt)}</span>
        </div>
      ))}
    </div>
  );
}

export default function DeployPage() {
  const params = useParams();
  const projectId = params.id as string;
  const [loading, setLoading] = useState(true);
  const [environments, setEnvironments] = useState<Environment[]>([]);
  const [deploying, setDeploying] = useState<number | null>(null);
  const [refreshKey, setRefreshKey] = useState(0);

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

  const [deployDialogEnv, setDeployDialogEnv] = useState<number | null>(null);
  const [deployVersion, setDeployVersion] = useState("");

  const handleDeploy = async (envId: number) => {
    if (!deployVersion.trim()) return;
    try {
      setDeploying(envId);
      await triggerDeploy(projectId, envId, deployVersion.trim());
      setDeployDialogEnv(null);
      setDeployVersion("");
      setRefreshKey((k) => k + 1);
      await fetchEnvironments();
    } catch (err) {
      console.error("Deploy failed:", err);
    } finally {
      setDeploying(null);
    }
  };

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
          const isDeploying = deploying === env.id;
          return (
            <div
              key={env.id}
              className="rounded-xl border border-border bg-muted/50 p-5 hover:bg-muted/70 transition-colors"
            >
              {/* Header: name + type badge */}
              <div className="flex items-center justify-between mb-4">
                <div className="flex items-center gap-2">
                  <Server className="h-4 w-4 text-muted-foreground/60" />
                  <span className="text-sm font-medium text-foreground/80">
                    {env.name}
                  </span>
                </div>
                <Badge
                  variant="secondary"
                  className={`text-[10px] ${ENV_TYPE_STYLES[env.env_type] || "bg-muted text-muted-foreground border-border"}`}
                >
                  {ENV_TYPE_LABELS[env.env_type] || env.env_type}
                </Badge>
              </div>

              {/* Status indicator */}
              <div className="flex items-center gap-2 mb-3">
                <span
                  className={`w-2 h-2 rounded-full ${isActive ? "bg-emerald-400" : "bg-muted-foreground/30"}`}
                />
                <span
                  className={`text-xs ${isActive ? "text-emerald-400" : "text-muted-foreground/60"}`}
                >
                  {isActive ? "运行中" : "未激活"}
                </span>
              </div>

              {/* Version */}
              <div className="mb-2">
                <p className="text-xs text-muted-foreground/60 mb-1">当前版本</p>
                <p className="text-sm font-mono text-muted-foreground">
                  {env.current_version || "—"}
                </p>
              </div>

              {/* Last deploy time */}
              <div className="flex items-center gap-1.5 text-xs text-muted-foreground/60 mb-3">
                <Clock className="h-3 w-3" />
                <span>
                  {env.last_deploy_at
                    ? `上次部署: ${formatTime(env.last_deploy_at)}`
                    : "暂无部署记录"}
                </span>
              </div>

              {/* Deploy button / dialog */}
              {deployDialogEnv === env.id ? (
                <div className="space-y-2 p-2 rounded-lg border border-primary/30 bg-primary/5">
                  <div className="flex items-center gap-2">
                    <Tag className="h-3 w-3 text-primary" />
                    <span className="text-xs text-muted-foreground">部署版本</span>
                  </div>
                  <input
                    type="text"
                    value={deployVersion}
                    onChange={(e) => setDeployVersion(e.target.value)}
                    placeholder="例如: v1.0.0"
                    className="w-full bg-muted/50 border border-border rounded px-2 py-1 text-xs text-foreground placeholder:text-muted-foreground/50 focus:outline-none focus:border-primary/50"
                    autoFocus
                    onKeyDown={(e) => {
                      if (e.key === "Enter") handleDeploy(env.id);
                      if (e.key === "Escape") { setDeployDialogEnv(null); setDeployVersion(""); }
                    }}
                  />
                  <div className="flex gap-1.5">
                    <Button
                      size="sm"
                      className="flex-1 text-xs h-7"
                      disabled={isDeploying || !deployVersion.trim()}
                      onClick={() => handleDeploy(env.id)}
                    >
                      {isDeploying ? <Loader2 className="h-3 w-3 animate-spin" /> : "确认部署"}
                    </Button>
                    <Button
                      size="sm"
                      variant="ghost"
                      className="text-xs h-7"
                      onClick={() => { setDeployDialogEnv(null); setDeployVersion(""); }}
                    >
                      取消
                    </Button>
                  </div>
                </div>
              ) : (
                <Button
                  size="sm"
                  variant="outline"
                  className="w-full text-xs"
                  onClick={() => setDeployDialogEnv(env.id)}
                >
                  <Play className="h-3 w-3 mr-1.5" />
                  部署
                </Button>
              )}

              {/* Deploy history timeline */}
              <DeployHistory key={refreshKey} projectId={projectId} envId={env.id} />
            </div>
          );
        })}
      </div>
    </div>
  );
}
