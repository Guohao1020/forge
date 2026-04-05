"use client";

import { useEffect, useState, useCallback } from "react";
import { useParams } from "next/navigation";
import { toast } from "sonner";
import { Brain, ChevronDown, ChevronRight, RefreshCw, Clock } from "lucide-react";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { listProfiles, triggerScan, type ProfileEntry } from "@/lib/profile";

const DIMENSION_LABELS: Record<string, string> = {
  api_catalog: "API 接口清单",
  db_schema: "数据库结构",
  module_graph: "模块依赖图",
  architecture: "技术架构",
  business_rules: "业务规则",
  coding_habits: "编码习惯",
  quality_trends: "质量趋势",
};

function relativeTime(dateStr: string): string {
  const now = Date.now();
  const then = new Date(dateStr).getTime();
  const diff = now - then;
  const minutes = Math.floor(diff / 60000);
  if (minutes < 1) return "刚刚";
  if (minutes < 60) return `${minutes} 分钟前`;
  const hours = Math.floor(minutes / 60);
  if (hours < 24) return `${hours} 小时前`;
  const days = Math.floor(hours / 24);
  if (days < 30) return `${days} 天前`;
  return new Date(dateStr).toLocaleDateString("zh-CN");
}

function LoadingSkeleton() {
  return (
    <div className="space-y-4 animate-pulse">
      {Array.from({ length: 4 }).map((_, i) => (
        <div
          key={i}
          className="rounded-xl border border-border bg-muted/50 p-5"
        >
          <div className="flex items-center justify-between mb-3">
            <div className="h-5 w-32 bg-muted/50 rounded" />
            <div className="flex items-center gap-2">
              <div className="h-4 w-12 bg-muted/50 rounded" />
              <div className="h-4 w-20 bg-muted/50 rounded" />
            </div>
          </div>
          <div className="h-20 w-full bg-muted/50 rounded" />
        </div>
      ))}
    </div>
  );
}

function EmptyState() {
  return (
    <div className="flex flex-col items-center justify-center py-20 rounded-xl border border-border bg-card">
      <div className="w-12 h-12 rounded-xl flex items-center justify-center mb-3 bg-primary/10">
        <Brain className="h-6 w-6 text-primary" />
      </div>
      <h3 className="text-base font-medium mb-1">暂无画像数据</h3>
      <p className="text-sm text-muted-foreground">
        暂无画像数据，点击「扫描更新」生成
      </p>
    </div>
  );
}

function ProfileCard({
  entry,
  isExpanded,
  onToggle,
}: {
  entry: ProfileEntry;
  isExpanded: boolean;
  onToggle: () => void;
}) {
  const label = DIMENSION_LABELS[entry.profileKey] || entry.profileKey;

  return (
    <div className="rounded-xl border border-border bg-muted/50 hover:bg-muted/70 transition-colors">
      <button
        type="button"
        onClick={onToggle}
        className="w-full flex items-center justify-between p-5 text-left"
      >
        <div className="flex items-center gap-3">
          {isExpanded ? (
            <ChevronDown className="h-4 w-4 text-muted-foreground/60 shrink-0" />
          ) : (
            <ChevronRight className="h-4 w-4 text-muted-foreground/60 shrink-0" />
          )}
          <span className="text-sm font-medium text-foreground/90">{label}</span>
        </div>
        <div className="flex items-center gap-3">
          <Badge
            variant="secondary"
            className="bg-primary/10 text-primary border-primary/20 text-[10px]"
          >
            v{entry.version}
          </Badge>
          <span className="flex items-center gap-1 text-xs text-muted-foreground/60">
            <Clock className="h-3 w-3" />
            {relativeTime(entry.scannedAt)}
          </span>
        </div>
      </button>

      {isExpanded && (
        <div className="px-5 pb-5 pt-0">
          <pre className="text-xs text-muted-foreground bg-muted/50 rounded-lg p-4 overflow-auto max-h-96 font-mono whitespace-pre-wrap">
            {JSON.stringify(entry.profileValue, null, 2)}
          </pre>
        </div>
      )}
    </div>
  );
}

export default function ProfilePage() {
  const params = useParams();
  const projectId = params.id as string;

  const [loading, setLoading] = useState(true);
  const [profiles, setProfiles] = useState<ProfileEntry[]>([]);
  const [scanning, setScanning] = useState(false);
  const [expanded, setExpanded] = useState<Set<string>>(new Set());

  const fetchProfiles = useCallback(async () => {
    try {
      setLoading(true);
      const data = await listProfiles(projectId);
      setProfiles(data);
    } catch (err) {
      console.error("Failed to fetch profiles:", err);
      setProfiles([]);
    } finally {
      setLoading(false);
    }
  }, [projectId]);

  useEffect(() => {
    fetchProfiles();
  }, [fetchProfiles]);

  const handleScan = async () => {
    try {
      setScanning(true);
      await triggerScan(projectId);
      toast.success("扫描任务已提交");
    } catch (err) {
      console.error("Failed to trigger scan:", err);
      toast.error("扫描触发失败");
    } finally {
      setScanning(false);
    }
  };

  const toggleCard = (key: string) => {
    setExpanded((prev) => {
      const next = new Set(prev);
      if (next.has(key)) next.delete(key);
      else next.add(key);
      return next;
    });
  };

  if (loading) {
    return (
      <div>
        <div className="flex items-center justify-between mb-6">
          <h1 className="text-2xl font-semibold tracking-tight">项目画像</h1>
        </div>
        <LoadingSkeleton />
      </div>
    );
  }

  return (
    <div>
      <div className="flex items-center justify-between mb-6">
        <h1 className="text-2xl font-semibold tracking-tight">项目画像</h1>
        <Button
          onClick={handleScan}
          disabled={scanning}
          className="bg-primary hover:bg-primary/90 text-primary-foreground"
        >
          <RefreshCw
            className={`h-4 w-4 mr-2 ${scanning ? "animate-spin" : ""}`}
          />
          扫描更新
        </Button>
      </div>

      {profiles.length === 0 ? (
        <EmptyState />
      ) : (
        <div className="space-y-3">
          {profiles.map((entry) => (
            <ProfileCard
              key={entry.profileKey}
              entry={entry}
              isExpanded={expanded.has(entry.profileKey)}
              onToggle={() => toggleCard(entry.profileKey)}
            />
          ))}
        </div>
      )}
    </div>
  );
}
