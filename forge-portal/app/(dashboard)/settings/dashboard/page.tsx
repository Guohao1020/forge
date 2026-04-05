"use client";

import { useEffect, useState } from "react";
import { BarChart3, Users, Zap, Clock, Server, Activity, TrendingUp, DollarSign } from "lucide-react";
import { api } from "@/lib/api";

interface MetricsSnapshot {
  totalRequests: number;
  totalErrors: number;
  avgLatencyMs: number;
  uptime: string;
  ai: {
    totalCalls: number;
    totalTokens: number;
    totalFallback: number;
    avgLatencyMs: number;
    callsByModel: Record<string, number>;
    tokensByModel: Record<string, number>;
  };
  tasks: {
    total: number;
    completed: number;
    failed: number;
    inProgress: number;
  };
  sseActive: number;
}

interface CostSummary {
  totalCalls: number;
  totalTokens: number;
  estimatedCost: number;
  models: { model: string; calls: number; tokens: number; cost: number }[];
}

export default function AdminDashboardPage() {
  const [metrics, setMetrics] = useState<MetricsSnapshot | null>(null);
  const [costs, setCosts] = useState<CostSummary | null>(null);
  const [userCount, setUserCount] = useState<number>(0);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    Promise.all([
      fetch("/api/admin/metrics").then(r => r.json()).then(setMetrics).catch(() => {}),
      api.get<{ summary: CostSummary }>("/admin/costs").then(r => setCosts(r.summary)).catch(() => {}),
      api.get<{ users: { id: number }[] }>("/admin/users").then(r => setUserCount(r.users?.length || 0)).catch(() => {}),
    ]).finally(() => setLoading(false));
  }, []);

  if (loading) {
    return (
      <div className="space-y-4 max-w-5xl">
        {[1, 2, 3].map(i => (
          <div key={i} className="h-32 rounded-xl bg-muted/50 animate-pulse" />
        ))}
      </div>
    );
  }

  return (
    <div className="max-w-5xl space-y-6">
      <div>
        <h1 className="text-xl font-semibold text-foreground flex items-center gap-2">
          <BarChart3 className="h-5 w-5 text-primary" />
          平台仪表盘
        </h1>
        <p className="text-sm text-muted-foreground mt-1">Forge 平台运行状况概览</p>
      </div>

      {/* Top stats row */}
      <div className="grid grid-cols-4 gap-4">
        <StatCard icon={Server} label="服务运行时间" value={metrics?.uptime || "—"} />
        <StatCard icon={Users} label="用户总数" value={String(userCount)} />
        <StatCard icon={Activity} label="API 请求总数" value={formatNumber(metrics?.totalRequests || 0)} />
        <StatCard
          icon={Clock}
          label="平均延迟"
          value={`${(metrics?.avgLatencyMs || 0).toFixed(1)} ms`}
          color={
            (metrics?.avgLatencyMs || 0) < 200 ? "text-green-400" :
            (metrics?.avgLatencyMs || 0) < 1000 ? "text-yellow-400" : "text-red-400"
          }
        />
      </div>

      {/* AI & Task row */}
      <div className="grid grid-cols-2 gap-4">
        {/* AI Performance */}
        <div className="rounded-xl border border-border bg-card p-5 space-y-4">
          <h2 className="text-sm font-medium text-foreground flex items-center gap-2">
            <Zap className="h-4 w-4 text-primary" />
            AI 性能
          </h2>
          <div className="grid grid-cols-2 gap-3">
            <MiniStat label="AI 调用" value={String(metrics?.ai.totalCalls || 0)} />
            <MiniStat label="Token 消耗" value={formatNumber(metrics?.ai.totalTokens || 0)} />
            <MiniStat label="平均延迟" value={`${(metrics?.ai.avgLatencyMs || 0).toFixed(0)} ms`} />
            <MiniStat label="降级次数" value={String(metrics?.ai.totalFallback || 0)} />
          </div>
          {metrics?.ai.callsByModel && Object.keys(metrics.ai.callsByModel).length > 0 && (
            <div className="space-y-1.5">
              <p className="text-[10px] text-muted-foreground uppercase tracking-wider">模型调用分布</p>
              {Object.entries(metrics.ai.callsByModel).map(([model, calls]) => (
                <div key={model} className="flex items-center justify-between text-xs">
                  <span className="text-muted-foreground font-mono">{model}</span>
                  <span className="text-foreground">{calls} 次</span>
                </div>
              ))}
            </div>
          )}
        </div>

        {/* Task Pipeline */}
        <div className="rounded-xl border border-border bg-card p-5 space-y-4">
          <h2 className="text-sm font-medium text-foreground flex items-center gap-2">
            <TrendingUp className="h-4 w-4 text-primary" />
            任务流水线
          </h2>
          <div className="grid grid-cols-2 gap-3">
            <MiniStat label="任务总数" value={String(metrics?.tasks.total || 0)} />
            <MiniStat label="已完成" value={String(metrics?.tasks.completed || 0)} color="text-green-400" />
            <MiniStat label="失败" value={String(metrics?.tasks.failed || 0)} color="text-red-400" />
            <MiniStat label="进行中" value={String(metrics?.tasks.inProgress || 0)} color="text-primary" />
          </div>
          <div className="pt-2">
            <MiniStat label="活跃 SSE 连接" value={String(metrics?.sseActive || 0)} />
          </div>
        </div>
      </div>

      {/* Cost Summary */}
      {costs && (
        <div className="rounded-xl border border-border bg-card p-5 space-y-4">
          <h2 className="text-sm font-medium text-foreground flex items-center gap-2">
            <DollarSign className="h-4 w-4 text-primary" />
            本月成本
          </h2>
          <div className="grid grid-cols-3 gap-4">
            <div className="bg-muted/50 rounded-lg border border-border px-4 py-3 text-center">
              <p className="text-[10px] text-muted-foreground uppercase tracking-wider mb-1">预估费用</p>
              <p className="text-2xl font-bold text-foreground">${costs.estimatedCost.toFixed(2)}</p>
            </div>
            <div className="bg-muted/50 rounded-lg border border-border px-4 py-3 text-center">
              <p className="text-[10px] text-muted-foreground uppercase tracking-wider mb-1">API 调用</p>
              <p className="text-2xl font-bold text-foreground">{formatNumber(costs.totalCalls)}</p>
            </div>
            <div className="bg-muted/50 rounded-lg border border-border px-4 py-3 text-center">
              <p className="text-[10px] text-muted-foreground uppercase tracking-wider mb-1">Token 总量</p>
              <p className="text-2xl font-bold text-foreground">{formatNumber(costs.totalTokens)}</p>
            </div>
          </div>
          {costs.models && costs.models.length > 0 && (
            <div className="space-y-1.5">
              <p className="text-[10px] text-muted-foreground uppercase tracking-wider">模型费用明细</p>
              {costs.models.map((m) => (
                <div key={m.model} className="flex items-center justify-between text-xs bg-muted/50 rounded px-3 py-1.5">
                  <span className="text-muted-foreground font-mono">{m.model}</span>
                  <div className="flex items-center gap-4">
                    <span className="text-muted-foreground">{m.calls} 次</span>
                    <span className="text-muted-foreground">{formatNumber(m.tokens)} tokens</span>
                    <span className="text-foreground font-medium">${m.cost.toFixed(2)}</span>
                  </div>
                </div>
              ))}
            </div>
          )}
        </div>
      )}

      {/* Error rate */}
      {metrics && metrics.totalErrors > 0 && (
        <div className="rounded-xl border border-red-500/30 bg-red-500/5 p-5 space-y-2">
          <h2 className="text-sm font-medium text-red-400">5xx 错误</h2>
          <p className="text-2xl font-bold text-red-400">{metrics.totalErrors}</p>
          <p className="text-xs text-muted-foreground">
            错误率: {((metrics.totalErrors / Math.max(metrics.totalRequests, 1)) * 100).toFixed(2)}%
          </p>
        </div>
      )}
    </div>
  );
}

function StatCard({
  icon: Icon,
  label,
  value,
  color = "text-foreground",
}: {
  icon: React.ComponentType<{ className?: string }>;
  label: string;
  value: string;
  color?: string;
}) {
  return (
    <div className="rounded-xl border border-border bg-card p-4">
      <div className="flex items-center gap-2 mb-2">
        <Icon className="h-4 w-4 text-muted-foreground" />
        <p className="text-[10px] text-muted-foreground uppercase tracking-wider">{label}</p>
      </div>
      <p className={`text-xl font-bold ${color}`}>{value}</p>
    </div>
  );
}

function MiniStat({
  label,
  value,
  color = "text-foreground",
}: {
  label: string;
  value: string;
  color?: string;
}) {
  return (
    <div className="bg-muted/50 rounded-lg px-3 py-2">
      <p className="text-[10px] text-muted-foreground">{label}</p>
      <p className={`text-sm font-semibold ${color}`}>{value}</p>
    </div>
  );
}

function formatNumber(n: number): string {
  if (n >= 1_000_000) return `${(n / 1_000_000).toFixed(1)}M`;
  if (n >= 1_000) return `${(n / 1_000).toFixed(1)}K`;
  return String(n);
}
