"use client"

import { useState } from "react"
import { cn } from "@/lib/utils"
import { Check, X, Loader2, ChevronDown, ChevronRight } from "lucide-react"

interface BuildCardProps {
  status: "building" | "passed" | "failed"
  command: string
  output?: string
  durationMs?: number
  className?: string
}

export function BuildCard({ status, command, output, durationMs, className }: BuildCardProps) {
  const [expanded, setExpanded] = useState(status === "failed")

  const bgClass = {
    building: "bg-[var(--accent-dim)] border-[var(--accent)]",
    passed: "bg-[var(--success-bg)] border-[var(--success)]",
    failed: "bg-[var(--error-bg)] border-[var(--error)]",
  }[status]

  const icon = {
    building: <Loader2 className="h-4 w-4 animate-spin text-[var(--accent)]" />,
    passed: <Check className="h-4 w-4 text-[var(--success)]" />,
    failed: <X className="h-4 w-4 text-[var(--error)]" />,
  }[status]

  const label = {
    building: "Building...",
    passed: "Build Passed",
    failed: "Build Failed",
  }[status]

  return (
    <div className={cn("rounded border transition-all duration-150", bgClass, className)} role="alert">
      <button
        onClick={() => setExpanded(!expanded)}
        className="flex items-center gap-2 w-full px-3 py-2 text-sm font-medium"
      >
        {expanded ? <ChevronDown className="h-3.5 w-3.5" /> : <ChevronRight className="h-3.5 w-3.5" />}
        {icon}
        <span>{label}</span>
        <span className="font-mono text-xs text-[var(--text-dim)] ml-auto">
          {command}
        </span>
        {durationMs !== undefined && (
          <span className="text-xs text-[var(--text-dim)]">
            {durationMs < 1000 ? `${durationMs}ms` : `${(durationMs / 1000).toFixed(1)}s`}
          </span>
        )}
      </button>
      {expanded && output && (
        <div className="px-3 pb-3">
          <pre className="font-mono text-[0.7rem] bg-[var(--background)] rounded p-3 overflow-x-auto max-h-[300px] overflow-y-auto leading-relaxed">
            {output}
          </pre>
        </div>
      )}
    </div>
  )
}
