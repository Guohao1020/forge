"use client"

import { cn } from "@/lib/utils"

// SSE connection state — mockup uses a 3-dot indicator, we use 4 states
// internally but render as 3 colors (connecting+reconnecting share yellow).
export type ConnStatus = "connecting" | "connected" | "reconnecting" | "failed"

export type BuildState = "idle" | "building" | "fixing" | "passed" | "failed"

export interface StatusBarProps {
  connStatus: ConnStatus
  buildState: BuildState
  currentStep?: number
  maxSteps?: number
  branch?: string
  language?: string
  errorCount?: number
  model?: string
  tokenCount: number
  costEstimate: number
  className?: string
}

// Mockup variant-B-dense.html lines 828-874 + 1367-1384. Blue bar in light
// mode (accent background, white-90 text), dark gray in dark mode.
const DOT_COLOR: Record<ConnStatus, string> = {
  connecting: "bg-[#fbbf24]", // yellow
  connected: "bg-[#4ade80]", // green
  reconnecting: "bg-[#fbbf24]", // yellow
  failed: "bg-[#f87171]", // red
}

const CONN_LABEL: Record<ConnStatus, string> = {
  connecting: "Connecting",
  connected: "Connected",
  reconnecting: "Reconnecting",
  failed: "Offline",
}

const BUILD_LABEL: Record<BuildState, string> = {
  idle: "idle",
  building: "building",
  fixing: "fixing",
  passed: "passed",
  failed: "failed",
}

function BuildDotColor(state: BuildState): string {
  if (state === "passed") return "bg-[#4ade80]" // green
  if (state === "failed") return "bg-[#f87171]" // red
  if (state === "building" || state === "fixing") return "bg-[#fbbf24]" // yellow
  return "bg-white/50 dark:bg-[var(--text-tertiary)]"
}

function formatNumber(n: number): string {
  if (n >= 1000) return `${(n / 1000).toFixed(1)}k`
  return String(n)
}

function formatCost(c: number): string {
  return c < 0.01 ? `$${c.toFixed(4)}` : `$${c.toFixed(2)}`
}

export function StatusBar({
  connStatus,
  buildState,
  currentStep,
  maxSteps,
  branch,
  language,
  errorCount = 0,
  model,
  tokenCount,
  costEstimate,
  className,
}: StatusBarProps) {
  // Rendered as a <div> (not <footer>) because <footer> has implicit
  // role="contentinfo" which collides with the explicit role="status"
  // the axe-core rules flag as an invalid combination.
  return (
    <div
      role="status"
      aria-live="polite"
      aria-label="Session status"
      className={cn(
        "flex items-center justify-between h-5 px-2 gap-1 font-mono text-[10px]",
        // Light: accent bg, white-90 text. Dark: bg-secondary, text-secondary,
        // top border. Mockup lines 828-845.
        "bg-[var(--accent)] text-white/90",
        "dark:bg-[var(--bg-secondary)] dark:text-[var(--text-secondary)] dark:border-t dark:border-[var(--border-primary)]",
        className,
      )}
    >
      {/* LEFT — connection + build + step + branch + lang */}
      <div className="flex items-center gap-2 min-w-0">
        <span className="flex items-center gap-1 whitespace-nowrap">
          <span
            className={cn("w-1.5 h-1.5 rounded-full shrink-0", DOT_COLOR[connStatus])}
            aria-hidden
          />
          <span className="sr-only">Connection: {CONN_LABEL[connStatus]}</span>
        </span>
        <span className="flex items-center gap-1 whitespace-nowrap">
          <span
            className={cn("w-1.5 h-1.5 rounded-full shrink-0", BuildDotColor(buildState))}
            aria-hidden
          />
          <span>Build: {BUILD_LABEL[buildState]}</span>
        </span>
        {currentStep != null && maxSteps != null && maxSteps > 0 && (
          <span className="whitespace-nowrap">
            Step {currentStep}/{maxSteps}
          </span>
        )}
        {branch && <span className="truncate max-w-[16ch]">{branch}</span>}
        {language && <span className="whitespace-nowrap">{language}</span>}
      </div>

      {/* RIGHT — errors + model + tokens + cost */}
      <div className="flex items-center gap-2 min-w-0 shrink-0">
        {errorCount > 0 && (
          <span className="whitespace-nowrap">
            {errorCount} {errorCount === 1 ? "error" : "errors"}
          </span>
        )}
        {model && (
          <span className="truncate max-w-[24ch]" title={model}>
            {model}
          </span>
        )}
        <span className="whitespace-nowrap">
          {formatNumber(tokenCount)} tok
        </span>
        <span className="whitespace-nowrap">{formatCost(costEstimate)}</span>
      </div>
    </div>
  )
}
