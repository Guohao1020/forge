"use client"

import { useEffect, useId, useRef, useState } from "react"
import { cn } from "@/lib/utils"
import { ChevronDown, ChevronRight } from "lucide-react"
import { formatToolInput, formatToolSummary } from "./tool-formatters"

interface ToolExecutionProps {
  toolName: string
  toolInput: Record<string, unknown>
  output?: string
  isError?: boolean
  isLoading?: boolean
}

// Name exported for tests
export { type ToolExecutionProps }

type ToolStatus = "ok" | "err" | "running"

function statusOf(isError?: boolean, isLoading?: boolean): ToolStatus {
  if (isLoading) return "running"
  if (isError) return "err"
  return "ok"
}

// Mockup variant-B-dense.html lines 417-502. Border is ALWAYS --border-primary.
// Status is encoded as a right-side badge, never a border color tint.
const statusClass: Record<ToolStatus, string> = {
  ok: "text-[var(--text-success)] bg-[var(--bg-success)]",
  err: "text-[var(--text-error)] bg-[var(--bg-error)]",
  running: "text-[var(--accent-text)] bg-[var(--accent-subtle)]",
}

const statusLabel: Record<ToolStatus, string> = {
  ok: "ok",
  err: "error",
  running: "running",
}

export function ToolExecution({
  toolName,
  toolInput,
  output,
  isError,
  isLoading,
}: ToolExecutionProps) {
  const [expanded, setExpanded] = useState(false)
  const status = statusOf(isError, isLoading)
  const toolId = useId()
  const toggleRef = useRef<HTMLButtonElement>(null)

  // Keyboard: Esc closes the expanded panel when focus is anywhere inside
  // the region. Restores focus to the toggle so keyboard users don't lose
  // their place in the message list.
  useEffect(() => {
    if (!expanded) return
    function onKey(e: KeyboardEvent) {
      if (e.key === "Escape") {
        e.preventDefault()
        setExpanded(false)
        toggleRef.current?.focus()
      }
    }
    window.addEventListener("keydown", onKey)
    return () => window.removeEventListener("keydown", onKey)
  }, [expanded])

  // Check if this tool opts out of card rendering (e.g. set_phase).
  // The effect of the tool is shown elsewhere (step ribbon for phases).
  // Must be after hooks to satisfy Rules of Hooks.
  const toolSummary = formatToolSummary(toolName, toolInput, output)
  if (toolSummary.hideCard) {
    return null
  }

  return (
    <div
      className="my-1 overflow-hidden rounded border border-[var(--border-primary)] bg-[var(--bg-tool)]"
      role="region"
      aria-label={toolName}
    >
      <button
        ref={toggleRef}
        onClick={() => setExpanded((v) => !v)}
        aria-expanded={expanded}
        aria-controls={toolId}
        className={cn(
          "flex items-center w-full gap-1.5 px-2 py-1 bg-[var(--bg-tertiary)] border-b border-[var(--border-secondary)] transition-colors duration-100",
          "hover:bg-[var(--bg-hover)]",
        )}
      >
        {expanded ? (
          <ChevronDown className="h-3 w-3 shrink-0 text-[var(--text-tertiary)]" />
        ) : (
          <ChevronRight className="h-3 w-3 shrink-0 text-[var(--text-tertiary)]" />
        )}
        <span className="font-mono text-[11px] font-medium text-[var(--text-secondary)]">
          {toolName}
        </span>
        {!expanded && (
          <span className="font-mono text-[10px] text-[var(--text-tertiary)] truncate ml-1 flex-1 text-left">
            {formatToolInput(toolName, toolInput)}
          </span>
        )}
        <span
          className={cn(
            "ml-auto font-mono text-[10px] px-1.5 py-px rounded",
            statusClass[status],
          )}
          aria-label={`Status: ${statusLabel[status]}`}
        >
          {statusLabel[status]}
        </span>
      </button>

      {expanded && (
        <div id={toolId} className="p-2 space-y-2">
          <div>
            <div className="font-mono text-[10px] text-[var(--text-tertiary)] mb-1">
              Input
            </div>
            <pre className="font-mono text-[10px] bg-[var(--bg-code)] rounded p-2 overflow-x-auto text-[var(--text-secondary)]">
              {JSON.stringify(toolInput, null, 2)}
            </pre>
          </div>
          {output && (
            <div>
              <div className="font-mono text-[10px] text-[var(--text-tertiary)] mb-1">
                Output
              </div>
              <pre
                className={cn(
                  "font-mono text-[10px] rounded p-2 overflow-x-auto max-h-[200px] overflow-y-auto",
                  isError
                    ? "bg-[var(--bg-error)] text-[var(--text-error)]"
                    : "bg-[var(--bg-code)] text-[var(--text-secondary)]",
                )}
              >
                {output}
              </pre>
            </div>
          )}
        </div>
      )}
    </div>
  )
}
