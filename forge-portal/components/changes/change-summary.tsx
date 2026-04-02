"use client";

import { Badge } from "@/components/ui/badge";
import { GitCommit, Shield, FileCode, Plus, Minus, ExternalLink } from "lucide-react";

export interface ChangeSummaryProps {
  commitMessage: string;
  reviewScore?: number;
  reviewPassed?: boolean;
  riskLevel?: string;
  filesChanged: number;
  linesAdded: number;
  linesDeleted: number;
  summary?: string;
  mrUrl?: string;
}

const RISK_STYLES: Record<string, string> = {
  LOW: "bg-emerald-500/10 text-emerald-400 border-emerald-500/20",
  MEDIUM: "bg-yellow-500/10 text-yellow-400 border-yellow-500/20",
  HIGH: "bg-red-500/10 text-red-400 border-red-500/20",
};

const RISK_LABELS: Record<string, string> = {
  LOW: "低风险",
  MEDIUM: "中风险",
  HIGH: "高风险",
};

function scoreColor(score: number): string {
  if (score >= 90) return "text-emerald-400";
  if (score >= 70) return "text-yellow-400";
  return "text-red-400";
}


export function ChangeSummary({
  commitMessage,
  reviewScore,
  reviewPassed,
  riskLevel,
  filesChanged,
  linesAdded,
  linesDeleted,
  summary,
  mrUrl,
}: ChangeSummaryProps) {
  return (
    <div className="space-y-4">
      {/* Commit message */}
      <div className="rounded-xl border border-white/10 bg-card p-5">
        <div className="flex items-start gap-3">
          <div className="shrink-0 w-9 h-9 rounded-lg bg-primary/10 flex items-center justify-center mt-0.5">
            <GitCommit className="h-4.5 w-4.5 text-primary" />
          </div>
          <div className="flex-1 min-w-0">
            <p className="text-xs text-white/30 mb-1">提交信息</p>
            <p className="text-base font-medium text-white/90 leading-relaxed">
              {commitMessage}
            </p>
            {mrUrl && (
              <a
                href={mrUrl}
                target="_blank"
                rel="noopener noreferrer"
                className="inline-flex items-center gap-1.5 mt-2 text-xs text-primary hover:text-primary/80 transition-colors"
              >
                <ExternalLink className="h-3.5 w-3.5" />
                在 GitHub 上查看 PR
              </a>
            )}
          </div>
        </div>
      </div>

      {/* Trust indicators row */}
      <div className="grid grid-cols-2 lg:grid-cols-4 gap-3">
        {/* Review score */}
        <div className="rounded-xl border border-white/10 bg-card p-4">
          <p className="text-xs text-white/30 mb-2">Review 评分</p>
          {reviewScore != null ? (
            <div className="flex items-center gap-2">
              <span className={`text-2xl font-bold ${scoreColor(reviewScore)}`}>
                {reviewScore}
              </span>
              <span className="text-sm text-white/30">/100</span>
              {reviewPassed != null && (
                <Badge
                  variant="secondary"
                  className={`ml-auto text-[10px] ${
                    reviewPassed
                      ? "bg-emerald-500/10 text-emerald-400 border-emerald-500/20"
                      : "bg-red-500/10 text-red-400 border-red-500/20"
                  }`}
                >
                  {reviewPassed ? "通过" : "未通过"}
                </Badge>
              )}
            </div>
          ) : (
            <span className="text-sm text-white/20">--</span>
          )}
        </div>

        {/* Risk level */}
        <div className="rounded-xl border border-white/10 bg-card p-4">
          <p className="text-xs text-white/30 mb-2">风险等级</p>
          {riskLevel ? (
            <Badge
              variant="secondary"
              className={RISK_STYLES[riskLevel] || RISK_STYLES.MEDIUM}
            >
              {RISK_LABELS[riskLevel] || riskLevel}
            </Badge>
          ) : (
            <span className="text-sm text-white/20">--</span>
          )}
        </div>

        {/* Files changed */}
        <div className="rounded-xl border border-white/10 bg-card p-4">
          <p className="text-xs text-white/30 mb-2">文件变更</p>
          <div className="flex items-center gap-2">
            <FileCode className="h-4 w-4 text-primary" />
            <span className="text-2xl font-bold text-white/80">{filesChanged}</span>
            <span className="text-sm text-white/30">个文件</span>
          </div>
        </div>

        {/* Lines changed */}
        <div className="rounded-xl border border-white/10 bg-card p-4">
          <p className="text-xs text-white/30 mb-2">代码行数</p>
          <div className="flex items-center gap-3">
            <span className="flex items-center gap-1 text-emerald-400 font-mono font-medium">
              <Plus className="h-3.5 w-3.5" />
              {linesAdded}
            </span>
            <span className="flex items-center gap-1 text-red-400 font-mono font-medium">
              <Minus className="h-3.5 w-3.5" />
              {linesDeleted}
            </span>
          </div>
        </div>
      </div>

      {/* Review summary */}
      {summary && (
        <div className="rounded-xl border border-white/10 bg-card p-5">
          <div className="flex items-start gap-3">
            <div className="shrink-0 w-9 h-9 rounded-lg bg-primary/10 flex items-center justify-center mt-0.5">
              <Shield className="h-4.5 w-4.5 text-primary" />
            </div>
            <div className="flex-1 min-w-0">
              <p className="text-xs text-white/30 mb-1">Review 总结</p>
              <p className="text-sm text-white/60 leading-relaxed whitespace-pre-wrap">
                {summary}
              </p>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}
