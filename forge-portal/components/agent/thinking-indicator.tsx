"use client"

import { cn } from "@/lib/utils"

interface ThinkingIndicatorProps {
  label: string
  className?: string
}

/**
 * A small pulsing indicator shown below a running bash tool card.
 * Phase 6 Task 6.8 repositioned this from chat-bottom to attached
 * to the specific tool card that's executing.
 */
export function ThinkingIndicator({ label, className }: ThinkingIndicatorProps) {
  return (
    <div
      className={cn(
        "flex items-center gap-1.5 mt-1 text-[11px] text-[var(--text-tertiary)]",
        className,
      )}
      role="status"
      aria-live="polite"
    >
      <span className="relative flex h-1.5 w-1.5">
        <span className="animate-ping absolute inline-flex h-full w-full rounded-full bg-[var(--accent)] opacity-75" />
        <span className="relative inline-flex rounded-full h-1.5 w-1.5 bg-[var(--accent)]" />
      </span>
      <span className="font-mono">{label}</span>
    </div>
  )
}
