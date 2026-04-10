"use client"

import { cn } from "@/lib/utils"
import { Check, Loader2 } from "lucide-react"

/**
 * Step ribbon states for the A2 single-agent phase tracker.
 *
 * - `done`: the agent has passed through this phase (set_phase
 *   fired a later phase after it)
 * - `active`: the agent is currently in this phase (last set_phase
 *   event named this phase)
 * - `pending`: the agent has not yet entered this phase
 *
 * There is no `failed` state in A2. Build failures are rendered
 * as error bash tool cards inside the chat; the step ribbon is
 * purely a progress indicator, not an error display.
 */
export type StepStatus = "done" | "active" | "pending"

export interface Step {
  id: string
  label: string
  status: StepStatus
}

interface StepRibbonProps {
  steps: Step[]
  className?: string
}

// Mockup variant-B-dense.html lines 230-289. No glow, no shadow.
// completed -> --text-success, active -> --accent bg + border.
// Step connector is a 10x1px div, NOT an arrow character.
const statusClass: Record<StepStatus, string> = {
  done: "text-[var(--text-success)]",
  active:
    "bg-[var(--accent-subtle)] text-[var(--accent)] border-[var(--accent)]",
  pending: "text-[var(--text-tertiary)]",
}

function StepIcon({ status, index }: { status: StepStatus; index: number }) {
  const baseCls = "inline-flex items-center justify-center w-3.5 h-3.5 text-[10px]"
  if (status === "done") return <Check className={cn(baseCls, "h-3 w-3")} />
  if (status === "active")
    return <Loader2 className={cn(baseCls, "h-3 w-3 animate-spin")} />
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
        aria-label="Agent workflow phases"
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
      aria-label="Agent workflow phases"
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
            aria-label={`${step.label}: ${step.status}`}
          >
            <StepIcon status={step.status} index={i} />
            <span>{step.label}</span>
          </div>
        </div>
      ))}
    </nav>
  )
}

/**
 * The 7 phases of the A2 workflow ribbon. Exported so callers
 * (page.tsx) can initialize state without duplicating the list.
 * Matches SetPhaseTool's Literal type in ai-worker/src/openharness/
 * tools/phase_tool.py.
 */
export const PHASES: ReadonlyArray<{ id: string; label: string }> = [
  { id: "Analyze", label: "Analyze" },
  { id: "Plan", label: "Plan" },
  { id: "Generate", label: "Generate" },
  { id: "Build", label: "Build" },
  { id: "Test", label: "Test" },
  { id: "Review", label: "Review" },
  { id: "Deploy", label: "Deploy" },
] as const

/**
 * Build an initial 7-step array with all phases in "pending" state.
 * Convenience for page-level state initialization.
 */
export function initialSteps(): Step[] {
  return PHASES.map((p) => ({
    id: p.id,
    label: p.label,
    status: "pending" as const,
  }))
}

/**
 * Compute the next steps array after a phase_changed event.
 *
 * Rules (spec §6.3):
 * - The named phase becomes 'active'
 * - Any phase that was previously 'active' becomes 'done'
 * - Any phase that was previously 'done' stays 'done' (we never
 *   un-complete a phase, even if the agent moves backwards)
 * - Phases the agent has never touched stay 'pending'
 */
export function updateStepsForPhase(
  current: Step[],
  newPhase: string,
): Step[] {
  return current.map((s) => {
    if (s.id === newPhase) {
      return { ...s, status: "active" as const }
    }
    if (s.status === "active") {
      // Previously active but the agent moved — mark as done
      return { ...s, status: "done" as const }
    }
    return s
  })
}
