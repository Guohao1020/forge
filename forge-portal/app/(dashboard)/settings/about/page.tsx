"use client";

import { useEffect, useState } from "react";
import { Info, Server, Database, Wifi, Clock, Shield, Layers } from "lucide-react";

interface SystemInfo {
  version: string;
  go: string;
  platform: string;
  uptime: string;
}

interface HealthStatus {
  status: string;
  database: string;
  redis: string;
  uptime: string;
}

export default function AboutPage() {
  const [sysInfo, setSysInfo] = useState<SystemInfo | null>(null);
  const [health, setHealth] = useState<HealthStatus | null>(null);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    Promise.all([
      fetch("/api/system/info").then(r => r.json()).then(setSysInfo).catch(() => {}),
      fetch("/api/health").then(r => r.json()).then(setHealth).catch(() => {}),
    ]).finally(() => setLoading(false));
  }, []);

  if (loading) {
    return (
      <div className="space-y-4 max-w-lg">
        {[1, 2, 3].map(i => (
          <div key={i} className="h-20 rounded-xl bg-muted/50 animate-pulse" />
        ))}
      </div>
    );
  }

  return (
    <div className="max-w-lg space-y-6">
      <div>
        <h1 className="text-xl font-semibold text-foreground flex items-center gap-2">
          <Info className="h-5 w-5 text-primary" />
          关于 Forge
        </h1>
        <p className="text-sm text-muted-foreground mt-1">平台版本和系统状态</p>
      </div>

      {/* Version info */}
      <div className="rounded-xl border border-border bg-card p-5 space-y-3">
        <h2 className="text-sm font-medium text-foreground">版本信息</h2>
        <div className="grid grid-cols-2 gap-3">
          <InfoItem icon={Layers} label="平台版本" value={sysInfo?.version || "dev"} />
          <InfoItem icon={Server} label="运行时" value={`Go ${sysInfo?.go || "1.26"}`} />
          <InfoItem icon={Clock} label="运行时间" value={sysInfo?.uptime || health?.uptime || "—"} />
          <InfoItem icon={Shield} label="服务" value={sysInfo?.platform || "forge-core"} />
        </div>
      </div>

      {/* Health status */}
      <div className="rounded-xl border border-border bg-card p-5 space-y-3">
        <h2 className="text-sm font-medium text-foreground">服务状态</h2>
        <div className="space-y-2">
          <StatusRow
            icon={Server}
            label="API Server"
            status={health ? "up" : "unknown"}
          />
          <StatusRow
            icon={Database}
            label="PostgreSQL"
            status={health?.database || "unknown"}
          />
          <StatusRow
            icon={Wifi}
            label="Redis"
            status={health?.redis || "unknown"}
          />
        </div>
      </div>

      {/* Platform stats */}
      <div className="rounded-xl border border-border bg-card p-5 space-y-3">
        <h2 className="text-sm font-medium text-foreground">平台规格</h2>
        <div className="grid grid-cols-2 gap-2 text-xs">
          <StatItem label="API 端点" value="~95" />
          <StatItem label="资源组" value="21" />
          <StatItem label="中间件层" value="11" />
          <StatItem label="数据库迁移" value="21" />
          <StatItem label="Go 测试" value="142" />
          <StatItem label="Docker 服务" value="10" />
        </div>
      </div>

      <p className="text-[10px] text-muted-foreground text-center">
        Forge Harness Engineering Platform &copy; 2026 Shulex
      </p>
    </div>
  );
}

function InfoItem({
  icon: Icon,
  label,
  value,
}: {
  icon: React.ComponentType<{ className?: string }>;
  label: string;
  value: string;
}) {
  return (
    <div className="flex items-center gap-2.5 bg-muted/50 rounded-lg px-3 py-2">
      <Icon className="h-4 w-4 text-muted-foreground shrink-0" />
      <div>
        <p className="text-[10px] text-muted-foreground">{label}</p>
        <p className="text-sm text-foreground font-mono">{value}</p>
      </div>
    </div>
  );
}

function StatusRow({
  icon: Icon,
  label,
  status,
}: {
  icon: React.ComponentType<{ className?: string }>;
  label: string;
  status: string;
}) {
  const isUp = status === "up";
  return (
    <div className="flex items-center justify-between px-3 py-2 rounded-lg bg-muted/50">
      <div className="flex items-center gap-2">
        <Icon className="h-4 w-4 text-muted-foreground" />
        <span className="text-sm text-foreground">{label}</span>
      </div>
      <span className={`flex items-center gap-1.5 text-xs ${isUp ? "text-green-400" : "text-yellow-400"}`}>
        <span className={`w-2 h-2 rounded-full ${isUp ? "bg-green-400" : "bg-yellow-400"}`} />
        {isUp ? "正常" : status}
      </span>
    </div>
  );
}

function StatItem({ label, value }: { label: string; value: string }) {
  return (
    <div className="flex items-center justify-between px-3 py-1.5 rounded bg-muted/50">
      <span className="text-muted-foreground">{label}</span>
      <span className="text-foreground font-mono">{value}</span>
    </div>
  );
}
