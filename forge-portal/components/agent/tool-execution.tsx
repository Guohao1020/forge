"use client"

import { useState } from "react"
import { cn } from "@/lib/utils"
import { ChevronDown, ChevronRight, Check, Loader2, AlertCircle } from "lucide-react"

interface ToolExecutionProps {
  toolName: string
  toolInput: Record<string, unknown>
  output?: string
  isError?: boolean
  isLoading?: boolean
}

export function ToolExecution({ toolName, toolInput, output, isError, isLoading }: ToolExecutionProps) {
  const [expanded, setExpanded] = useState(false)

  const icon = isLoading
    ? <Loader2 className="h-3.5 w-3.5 animate-spin text-[var(--accent)]" />
    : isError
      ? <AlertCircle className="h-3.5 w-3.5 text-[var(--error)]" />
      : <Check className="h-3.5 w-3.5 text-[var(--success)]" />

  return (
    <div className={cn(
      "rounded border text-xs transition-all duration-150",
      isError ? "border-[var(--error)]/30" : "border-[var(--border)]",
    )}>
      <button
        onClick={() => setExpanded(!expanded)}
        className="flex items-center gap-2 w-full px-3 py-2 hover:bg-[var(--hover)] transition-colors duration-150"
      >
        {expanded ? <ChevronDown className="h-3 w-3" /> : <ChevronRight className="h-3 w-3" />}
        {icon}
        <span className="font-mono font-medium">{toolName}</span>
        {!expanded && toolInput && (
          <span className="text-[var(--text-dim)] truncate ml-1">
            {JSON.stringify(toolInput).slice(0, 60)}
          </span>
        )}
      </button>
      {expanded && (
        <div className="px-3 pb-2 space-y-2">
          <div>
            <div className="text-[var(--text-dim)] mb-1">Input:</div>
            <pre className="font-mono text-[0.7rem] bg-[var(--surface)] rounded p-2 overflow-x-auto">
              {JSON.stringify(toolInput, null, 2)}
            </pre>
          </div>
          {output && (
            <div>
              <div className="text-[var(--text-dim)] mb-1">Output:</div>
              <pre className={cn(
                "font-mono text-[0.7rem] rounded p-2 overflow-x-auto max-h-[200px] overflow-y-auto",
                isError ? "bg-[var(--error-bg)] text-[var(--error)]" : "bg-[var(--surface)]",
              )}>
                {output}
              </pre>
            </div>
          )}
        </div>
      )}
    </div>
  )
}
