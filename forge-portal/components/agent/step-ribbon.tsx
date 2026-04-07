"use client"

import { cn } from "@/lib/utils"
import { Check, Circle, Loader2, X } from "lucide-react"

export type StepStatus = "done" | "active" | "pending" | "failed"

export interface Step {
  id: string
  label: string
  status: StepStatus
  cycle?: number
  maxCycles?: number
}

interface StepRibbonProps {
  steps: Step[]
  className?: string
}

const statusIcon: Record<StepStatus, React.ReactNode> = {
  done: <Check className="h-3 w-3" />,
  active: <Loader2 className="h-3 w-3 animate-spin" />,
  pending: <Circle className="h-3 w-3" />,
  failed: <X className="h-3 w-3" />,
}

const statusClass: Record<StepStatus, string> = {
  done: "bg-[var(--success-bg)] text-[var(--success)] border-[var(--success)]",
  active: "bg-[var(--accent-glow)] text-[var(--accent)] border-[var(--accent)] shadow-[0_0_8px_var(--accent-glow)]",
  pending: "bg-[var(--surface)] text-[var(--text-dim)] border-[var(--border)]",
  failed: "bg-[var(--error-bg)] text-[var(--error)] border-[var(--error)]",
}

export function StepRibbon({ steps, className }: StepRibbonProps) {
  if (steps.length === 0) {
    return (
      <div className={cn("flex items-center gap-2 px-4 py-2 overflow-x-auto", className)}>
        <span className="text-xs text-[var(--text-dim)]">Ready</span>
      </div>
    )
  }

  return (
    <div
      className={cn("flex items-center gap-1.5 px-4 py-2 overflow-x-auto", className)}
      role="list"
      aria-label="Task progress"
    >
      {steps.map((step, i) => (
        <div key={step.id} className="flex items-center gap-1.5">
          {i > 0 && <span className="text-[var(--text-dim)] text-xs">→</span>}
          <div
            className={cn(
              "inline-flex items-center gap-1.5 px-2.5 py-1 rounded border text-xs font-medium whitespace-nowrap transition-all duration-150",
              statusClass[step.status],
            )}
            role="listitem"
            aria-current={step.status === "active" ? "step" : undefined}
          >
            {statusIcon[step.status]}
            <span>{step.label}</span>
            {step.cycle && step.maxCycles && (
              <span className="text-[0.65rem] opacity-70">
                {step.cycle}/{step.maxCycles}
              </span>
            )}
          </div>
        </div>
      ))}
    </div>
  )
}
