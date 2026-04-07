"use client"

import { cn } from "@/lib/utils"
import { CheckCircle2, XCircle, MinusCircle } from "lucide-react"

export type BuildSummaryStatus = "passed" | "failed" | "skipped"

export interface SummaryCardProps {
  filesCreated: number
  filesModified: number
  buildStatus: BuildSummaryStatus
  durationMs: number
  tokensTotal: number
  costUsd: number
  className?: string
}

const STATUS_COLOR: Record<BuildSummaryStatus, string> = {
  passed: "border-[var(--text-success)] bg-[var(--bg-success)]",
  failed: "border-[var(--text-error)] bg-[var(--bg-error)]",
  skipped: "border-[var(--border-primary)] bg-[var(--bg-tool)]",
}

const STATUS_HEADER: Record<BuildSummaryStatus, { label: string; icon: typeof CheckCircle2; color: string }> = {
  passed: { label: "Done", icon: CheckCircle2, color: "text-[var(--text-success)]" },
  failed: { label: "Failed", icon: XCircle, color: "text-[var(--text-error)]" },
  skipped: { label: "Done", icon: MinusCircle, color: "text-[var(--text-tertiary)]" },
}

function formatDuration(ms: number): string {
  if (ms < 1000) return `${ms}ms`
  if (ms < 60_000) return `${(ms / 1000).toFixed(1)}s`
  const min = Math.floor(ms / 60_000)
  const sec = Math.round((ms % 60_000) / 1000)
  return `${min}m ${sec}s`
}

function formatTokens(n: number): string {
  if (n >= 1000) return `${(n / 1000).toFixed(1)}k`
  return String(n)
}

function formatCost(c: number): string {
  if (c < 0.01) return `$${c.toFixed(4)}`
  return `$${c.toFixed(2)}`
}

/**
 * End-of-session summary card. Appended to the message list when the
 * backend emits SessionComplete. Matches the mockup's "done" card
 * pattern: header with status icon + duration, 3x2 stats grid beneath.
 */
export function SummaryCard({
  filesCreated,
  filesModified,
  buildStatus,
  durationMs,
  tokensTotal,
  costUsd,
  className,
}: SummaryCardProps) {
  const status = STATUS_HEADER[buildStatus]
  const StatusIcon = status.icon

  const stats: Array<{ label: string; value: string }> = [
    { label: "Files created", value: String(filesCreated) },
    { label: "Files modified", value: String(filesModified) },
    {
      label: "Build",
      value: buildStatus === "skipped" ? "n/a" : buildStatus,
    },
    { label: "Duration", value: formatDuration(durationMs) },
    { label: "Tokens", value: formatTokens(tokensTotal) },
    { label: "Cost", value: formatCost(costUsd) },
  ]

  return (
    <div
      role="region"
      aria-label="Session summary"
      className={cn(
        "my-2 rounded border overflow-hidden",
        STATUS_COLOR[buildStatus],
        className,
      )}
    >
      <div className="flex items-center gap-1.5 px-2 py-1 border-b border-[var(--border-secondary)] bg-[var(--bg-tertiary)]">
        <StatusIcon className={cn("h-3 w-3 shrink-0", status.color)} />
        <span className={cn("font-mono text-[11px] font-medium", status.color)}>
          {status.label}
        </span>
        <span className="ml-auto font-mono text-[10px] text-[var(--text-tertiary)]">
          {formatDuration(durationMs)}
        </span>
      </div>
      <div className="grid grid-cols-3 gap-px bg-[var(--border-secondary)]">
        {stats.map((s) => (
          <div
            key={s.label}
            className="flex flex-col items-start gap-0.5 px-2 py-1.5 bg-[var(--bg-tool)]"
          >
            <span className="font-mono text-[9px] text-[var(--text-tertiary)] uppercase tracking-wide">
              {s.label}
            </span>
            <span className="font-mono text-[11px] font-medium text-[var(--text-primary)]">
              {s.value}
            </span>
          </div>
        ))}
      </div>
    </div>
  )
}
