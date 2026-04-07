"use client"

import { cn } from "@/lib/utils"

interface ThinkingIndicatorProps {
  label?: string
  className?: string
}

/**
 * Three-dot pulsing indicator rendered under the current AI message
 * while the agent is thinking or running a tool. The Python backend
 * emits ThinkingStarted with an optional label (e.g. "Running read_file",
 * "Analyzing project", "Fixing code"); the frontend shows it until the
 * matching ThinkingStopped event arrives.
 *
 * Uses --accent-text for label color so it reads as secondary motion
 * rather than a primary state change.
 */
export function ThinkingIndicator({
  label = "Thinking",
  className,
}: ThinkingIndicatorProps) {
  return (
    <div
      role="status"
      aria-live="polite"
      aria-label={`${label}…`}
      className={cn(
        "flex items-center gap-2 py-1 px-0.5 font-mono text-[11px] italic text-[var(--accent-text)]",
        className,
      )}
    >
      <span className="flex items-center gap-0.5" aria-hidden>
        <span className="w-1 h-1 rounded-full bg-current animate-thinking-dot-1" />
        <span className="w-1 h-1 rounded-full bg-current animate-thinking-dot-2" />
        <span className="w-1 h-1 rounded-full bg-current animate-thinking-dot-3" />
      </span>
      <span>{label}…</span>
    </div>
  )
}
