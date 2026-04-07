"use client"

import { cn } from "@/lib/utils"
import { Check, Loader2, X } from "lucide-react"

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

// Mockup variant-B-dense.html lines 230-289. No glow, no shadow.
// completed -> --text-success, active -> --accent bg + border, error -> --text-error.
// Step connector is a 10×1px div, NOT an arrow character.
const statusClass: Record<StepStatus, string> = {
  done: "text-[var(--text-success)]",
  active:
    "bg-[var(--accent-subtle)] text-[var(--accent)] border-[var(--accent)]",
  pending: "text-[var(--text-tertiary)]",
  failed: "text-[var(--text-error)]",
}

function StepIcon({ status, index }: { status: StepStatus; index: number }) {
  const baseCls = "inline-flex items-center justify-center w-3.5 h-3.5 text-[10px]"
  if (status === "done") return <Check className={cn(baseCls, "h-3 w-3")} />
  if (status === "active")
    return <Loader2 className={cn(baseCls, "h-3 w-3 animate-spin")} />
  if (status === "failed") return <X className={cn(baseCls, "h-3 w-3")} />
  return <span className={baseCls}>{index + 1}</span>
}

export function StepRibbon({ steps, className }: StepRibbonProps) {
  if (steps.length === 0) {
    return (
      <nav
        className={cn(
          "flex items-center h-10 px-2.5 gap-0.5 bg-[var(--bg-secondary)] border-b border-[var(--border-primary)] overflow-x-auto",
          "[&::-webkit-scrollbar]:hidden",
          className,
        )}
        style={{ scrollbarWidth: "none" }}
        aria-label="Task progress"
      >
        <span className="text-[11px] text-[var(--text-tertiary)]">Ready</span>
      </nav>
    )
  }

  return (
    <nav
      className={cn(
        "flex items-center h-10 px-2.5 gap-0.5 bg-[var(--bg-secondary)] border-b border-[var(--border-primary)] overflow-x-auto",
        "[&::-webkit-scrollbar]:hidden",
        className,
      )}
      style={{ scrollbarWidth: "none" }}
      aria-label="Task progress"
    >
      {steps.map((step, i) => (
        <div key={step.id} className="flex items-center gap-0.5">
          {i > 0 && (
            <div
              className="w-2.5 h-px bg-[var(--border-primary)] shrink-0"
              aria-hidden
            />
          )}
          <div
            className={cn(
              "inline-flex items-center gap-1 px-2 py-1 rounded text-[11px] font-medium whitespace-nowrap transition-colors duration-100 border border-transparent cursor-pointer",
              "hover:bg-[var(--bg-hover)]",
              statusClass[step.status],
            )}
            aria-current={step.status === "active" ? "step" : undefined}
            aria-label={`${step.label}: ${step.status}${
              step.cycle && step.maxCycles
                ? ` (cycle ${step.cycle} of ${step.maxCycles})`
                : ""
            }`}
          >
            <StepIcon status={step.status} index={i} />
            <span>{step.label}</span>
            {step.cycle && step.maxCycles && (
              <span className="text-[9px] opacity-70 ml-0.5">
                ({step.cycle}/{step.maxCycles})
              </span>
            )}
          </div>
        </div>
      ))}
    </nav>
  )
}
