"use client";

import { Badge } from "@/components/ui/badge";
import { AlertCircle, AlertTriangle, Info, Wrench } from "lucide-react";

interface ReviewFinding {
  severity: string;
  file: string;
  line?: number;
  rule?: string;
  message: string;
  suggestion?: string;
}

interface ReviewReportCardProps {
  reviewOutput: {
    passed: boolean;
    score: number;
    findings: ReviewFinding[];
    summary: string;
  };
}

const SEVERITY_STYLES: Record<string, string> = {
  ERROR: "bg-red-500/10 text-red-400 border-red-500/20",
  WARNING: "bg-yellow-500/10 text-yellow-400 border-yellow-500/20",
  INFO: "bg-blue-500/10 text-blue-400 border-blue-500/20",
};

const SEVERITY_ICONS: Record<string, React.ReactNode> = {
  ERROR: <AlertCircle className="h-3.5 w-3.5 text-red-400 shrink-0" />,
  WARNING: <AlertTriangle className="h-3.5 w-3.5 text-yellow-400 shrink-0" />,
  INFO: <Info className="h-3.5 w-3.5 text-blue-400 shrink-0" />,
};

const SEVERITY_ORDER: Record<string, number> = { ERROR: 0, WARNING: 1, INFO: 2 };

function ScoreRing({ score }: { score: number }) {
  const radius = 36;
  const circumference = 2 * Math.PI * radius;
  const progress = (score / 100) * circumference;
  const color = score >= 80 ? "#10B981" : score >= 60 ? "#F59E0B" : "#EF4444";

  return (
    <div className="relative inline-flex items-center justify-center">
      <svg width="88" height="88" viewBox="0 0 88 88">
        <circle
          cx="44"
          cy="44"
          r={radius}
          fill="none"
          stroke="rgba(0,0,0,0.08)"
          strokeWidth="6"
        />
        <circle
          cx="44"
          cy="44"
          r={radius}
          fill="none"
          stroke={color}
          strokeWidth="6"
          strokeLinecap="round"
          strokeDasharray={circumference}
          strokeDashoffset={circumference - progress}
          transform="rotate(-90 44 44)"
          className="transition-all duration-700"
        />
      </svg>
      <span className="absolute text-2xl font-bold" style={{ color }}>
        {score}
      </span>
    </div>
  );
}

function isLintFinding(f: ReviewFinding): boolean {
  return !!f.rule && f.rule.startsWith("LINT/");
}

function FindingRow({ f }: { f: ReviewFinding }) {
  const lint = isLintFinding(f);
  const icon = lint ? (
    <Wrench className="h-3.5 w-3.5 text-amber-400 shrink-0" />
  ) : (
    SEVERITY_ICONS[f.severity] || SEVERITY_ICONS.INFO
  );

  return (
    <div className="flex items-start gap-2 p-2.5 rounded-lg bg-muted/30 border border-border">
      {icon}
      <div className="flex-1 min-w-0">
        <div className="flex items-center gap-2">
          {lint ? (
            <span className="px-1.5 py-0.5 rounded text-[10px] border bg-amber-500/10 text-amber-400 border-amber-500/20">
              LINT
            </span>
          ) : (
            <span
              className={`px-1.5 py-0.5 rounded text-[10px] border ${
                SEVERITY_STYLES[f.severity] || SEVERITY_STYLES.INFO
              }`}
            >
              {f.severity}
            </span>
          )}
          {f.rule && (
            <span className="text-[10px] text-muted-foreground font-mono">{f.rule}</span>
          )}
          <span className="text-xs text-muted-foreground font-mono truncate">
            {f.file}
            {f.line != null && `:${f.line}`}
          </span>
        </div>
        <p className="text-sm text-foreground/70 mt-1">{f.message}</p>
        {f.suggestion && (
          <p className="text-xs text-muted-foreground mt-1">
            建议: {f.suggestion}
          </p>
        )}
      </div>
    </div>
  );
}

export function ReviewReportCard({ reviewOutput }: ReviewReportCardProps) {
  const { passed, score, findings, summary } = reviewOutput;

  const sortBy = (list: ReviewFinding[]) =>
    [...list].sort(
      (a, b) => (SEVERITY_ORDER[a.severity] ?? 3) - (SEVERITY_ORDER[b.severity] ?? 3)
    );

  const lintFindings = sortBy(findings.filter(isLintFinding));
  const reviewFindings = sortBy(findings.filter((f) => !isLintFinding(f)));

  const errorCount = findings.filter((f) => f.severity === "ERROR").length;
  const warningCount = findings.filter((f) => f.severity === "WARNING").length;

  return (
    <div className="rounded-xl border border-border bg-card p-5 space-y-5">
      {/* Score + Status */}
      <div className="flex items-center gap-5">
        <ScoreRing score={score} />
        <div className="space-y-2">
          <Badge
            variant="secondary"
            className={
              passed
                ? "bg-emerald-500/10 text-emerald-400 border-emerald-500/20"
                : "bg-red-500/10 text-red-400 border-red-500/20"
            }
          >
            {passed ? "审查通过" : "审查未通过"}
          </Badge>
          <div className="flex items-center gap-3 text-xs text-muted-foreground">
            {lintFindings.length > 0 && (
              <span className="text-amber-400 flex items-center gap-1">
                <Wrench className="h-3 w-3" />
                {lintFindings.length} 个 Lint 问题
              </span>
            )}
            {reviewFindings.length > 0 && (
              <span>{reviewFindings.length} 个 Review 问题</span>
            )}
            {errorCount > 0 && (
              <span className="text-red-400">{errorCount} 个错误</span>
            )}
            {warningCount > 0 && (
              <span className="text-yellow-400">{warningCount} 个警告</span>
            )}
          </div>
        </div>
      </div>

      {/* Summary */}
      {summary && (
        <p className="text-sm text-foreground/60">{summary}</p>
      )}

      {/* Lint findings */}
      {lintFindings.length > 0 && (
        <div className="space-y-2">
          <h4 className="text-xs text-muted-foreground uppercase tracking-wide flex items-center gap-1.5">
            <Wrench className="h-3 w-3 text-amber-400/60" />
            Lint 问题
          </h4>
          {lintFindings.map((f, i) => (
            <FindingRow key={`lint-${i}`} f={f} />
          ))}
        </div>
      )}

      {/* Review findings */}
      {reviewFindings.length > 0 && (
        <div className="space-y-2">
          <h4 className="text-xs text-muted-foreground uppercase tracking-wide">Review 问题</h4>
          {reviewFindings.map((f, i) => (
            <FindingRow key={`review-${i}`} f={f} />
          ))}
        </div>
      )}
    </div>
  );
}
