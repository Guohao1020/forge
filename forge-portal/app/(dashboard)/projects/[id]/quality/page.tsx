"use client";

import { useEffect, useState } from "react";
import { useParams } from "next/navigation";
import { Shield, Activity, TrendingUp, AlertCircle } from "lucide-react";
import { api } from "@/lib/api";
import { SEVERITY_CONFIG, CATEGORY_CONFIG } from "@/lib/entropy";

interface EntropyScan {
  id: number;
  projectId: number;
  score: number;
  issueCount: number;
  issues: string;
  scannedAt: string;
}

interface EntropyIssue {
  file: string;
  line: number;
  rule: string;
  message: string;
  severity: string;
  category: string;
}

export default function QualityPage() {
  const params = useParams();
  const projectId = params.id as string;

  const [scan, setScan] = useState<EntropyScan | null>(null);
  const [issues, setIssues] = useState<EntropyIssue[]>([]);
  const [history, setHistory] = useState<EntropyScan[]>([]);
  const [loading, setLoading] = useState(true);
  const [scanning, setScanning] = useState(false);

  useEffect(() => {
    Promise.all([
      api.get<{ scan: EntropyScan | null }>(`/projects/${projectId}/entropy/latest`)
        .then((res) => {
          if (res.scan) {
            setScan(res.scan);
            try {
              setIssues(JSON.parse(res.scan.issues));
            } catch {
              setIssues([]);
            }
          }
        })
        .catch(() => {}),
      api.get<{ scans: EntropyScan[] }>(`/projects/${projectId}/entropy/scans?limit=10`)
        .then((res) => setHistory(res.scans))
        .catch(() => {}),
    ]).finally(() => setLoading(false));
  }, [projectId]);

  const handleScan = async () => {
    setScanning(true);
    try {
      await api.post(`/projects/${projectId}/entropy/scan`, {});
    } catch {
      // ignore
    } finally {
      setScanning(false);
    }
  };

  if (loading) {
    return (
      <div className="space-y-4 max-w-4xl">
        {[1, 2, 3].map((i) => (
          <div key={i} className="h-24 rounded-xl bg-white/5 animate-pulse" />
        ))}
      </div>
    );
  }

  return (
    <div className="max-w-4xl space-y-6">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-semibold tracking-tight flex items-center gap-2">
            <Shield className="h-6 w-6 text-primary" />
            代码质量
          </h1>
          <p className="text-sm text-muted-foreground mt-1">基于 Entropy Management 的代码质量跟踪</p>
        </div>
        <button
          onClick={handleScan}
          disabled={scanning}
          className="flex items-center gap-2 px-4 py-2 bg-primary text-white rounded-lg text-sm hover:bg-primary/90 disabled:opacity-50 transition-colors"
        >
          <Activity size={16} />
          {scanning ? "扫描中..." : "运行扫描"}
        </button>
      </div>

      {/* Score overview */}
      {scan ? (
        <div className="grid grid-cols-4 gap-4">
          <ScoreCard
            label="质量分数"
            value={String(scan.score)}
            color={scan.score >= 80 ? "text-green-400" : scan.score >= 60 ? "text-yellow-400" : "text-red-400"}
            large
          />
          <ScoreCard label="问题总数" value={String(scan.issueCount)} />
          <ScoreCard label="扫描文件" value={String(issues.length > 0 ? new Set(issues.map(i => i.file)).size : 0)} />
          <ScoreCard label="上次扫描" value={new Date(scan.scannedAt).toLocaleDateString("zh-CN")} small />
        </div>
      ) : (
        <div className="rounded-xl border border-border bg-card p-8 text-center">
          <TrendingUp className="h-12 w-12 text-muted-foreground mx-auto mb-3 opacity-30" />
          <p className="text-lg text-muted-foreground">尚未运行过质量扫描</p>
          <p className="text-sm text-muted-foreground mt-1">点击「运行扫描」开始首次代码质量检测</p>
        </div>
      )}

      {/* Issue breakdown by category */}
      {issues.length > 0 && (
        <div className="rounded-xl border border-border bg-card p-5 space-y-4">
          <h2 className="text-sm font-medium text-foreground">问题分布</h2>
          <div className="grid grid-cols-5 gap-3">
            {Object.entries(CATEGORY_CONFIG).map(([key, cfg]) => {
              const count = issues.filter(i => i.category === key).length;
              return (
                <div key={key} className="bg-white/5 rounded-lg border border-white/10 px-3 py-2 text-center">
                  <p className="text-lg font-bold text-foreground">{count}</p>
                  <p className="text-[10px] text-muted-foreground">{cfg.label}</p>
                </div>
              );
            })}
          </div>
        </div>
      )}

      {/* Issue list */}
      {issues.length > 0 && (
        <div className="rounded-xl border border-border bg-card p-5 space-y-3">
          <h2 className="text-sm font-medium text-foreground">问题详情</h2>
          <div className="space-y-2">
            {issues.slice(0, 50).map((issue, idx) => {
              const sevCfg = SEVERITY_CONFIG[issue.severity] || SEVERITY_CONFIG.info;
              return (
                <div key={idx} className="flex items-start gap-3 px-3 py-2 rounded-lg bg-white/5 border border-white/5">
                  <span className={`shrink-0 px-1.5 py-0.5 rounded text-[10px] border ${sevCfg.color}`}>
                    {sevCfg.label}
                  </span>
                  <div className="flex-1 min-w-0">
                    <p className="text-sm text-foreground">{issue.message}</p>
                    <p className="text-xs text-muted-foreground mt-0.5">
                      {issue.file}:{issue.line} <span className="text-white/20 mx-1">|</span> {issue.rule}
                    </p>
                  </div>
                </div>
              );
            })}
          </div>
        </div>
      )}

      {/* Scan history */}
      {history.length > 1 && (
        <div className="rounded-xl border border-border bg-card p-5 space-y-3">
          <h2 className="text-sm font-medium text-foreground">扫描历史</h2>
          <div className="space-y-1.5">
            {history.map((s) => (
              <div key={s.id} className="flex items-center justify-between px-3 py-2 rounded-lg bg-white/5">
                <div className="flex items-center gap-3">
                  <span className={`text-lg font-bold ${
                    s.score >= 80 ? "text-green-400" : s.score >= 60 ? "text-yellow-400" : "text-red-400"
                  }`}>
                    {s.score}
                  </span>
                  <span className="text-sm text-muted-foreground">
                    {s.issueCount} 个问题
                  </span>
                </div>
                <span className="text-xs text-muted-foreground">
                  {new Date(s.scannedAt).toLocaleDateString("zh-CN")}
                </span>
              </div>
            ))}
          </div>
        </div>
      )}
    </div>
  );
}

function ScoreCard({
  label,
  value,
  color = "text-foreground",
  large = false,
  small = false,
}: {
  label: string;
  value: string;
  color?: string;
  large?: boolean;
  small?: boolean;
}) {
  return (
    <div className="bg-white/5 rounded-xl border border-white/10 px-4 py-3 text-center">
      <p className="text-[10px] text-muted-foreground uppercase tracking-wider mb-1">{label}</p>
      <p className={`${large ? "text-3xl" : small ? "text-sm mt-1" : "text-2xl"} font-bold ${color}`}>{value}</p>
    </div>
  );
}
