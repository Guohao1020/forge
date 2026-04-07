"use client"

import { useCallback, useEffect, useRef, useState } from "react"
import { cn } from "@/lib/utils"

const STORAGE_KEY = "agent-split-pct"
const MIN_PCT = 20
const MAX_PCT = 80
const DEFAULT_PCT = 50

export interface PanelDividerProps {
  // Called with a 0-100 percentage for the LEFT panel width
  onChange: (pct: number) => void
  // Element that bounds the drag — usually the parent main row
  containerRef: React.RefObject<HTMLElement | null>
  className?: string
}

/**
 * Load persisted split from localStorage. Must be called from useEffect
 * (client-only) to stay SSR-safe.
 */
export function loadSplitPct(): number {
  if (typeof window === "undefined") return DEFAULT_PCT
  const raw = window.localStorage.getItem(STORAGE_KEY)
  if (!raw) return DEFAULT_PCT
  const n = parseFloat(raw)
  if (!Number.isFinite(n)) return DEFAULT_PCT
  return Math.min(MAX_PCT, Math.max(MIN_PCT, n))
}

/**
 * Draggable 1px divider between the chat and code panels. Mockup lines
 * 296-303. Parent owns the split percentage; divider only dispatches
 * onChange during drag and persists the final value to localStorage.
 */
export function PanelDivider({
  onChange,
  containerRef,
  className,
}: PanelDividerProps) {
  const [dragging, setDragging] = useState(false)
  const frameRef = useRef<number | null>(null)
  const latestPctRef = useRef<number>(DEFAULT_PCT)

  const handleMouseMove = useCallback(
    (e: MouseEvent) => {
      const el = containerRef.current
      if (!el) return
      const rect = el.getBoundingClientRect()
      if (rect.width <= 0) return
      const rawPct = ((e.clientX - rect.left) / rect.width) * 100
      const clamped = Math.min(MAX_PCT, Math.max(MIN_PCT, rawPct))
      latestPctRef.current = clamped
      // Batch with rAF so rapid mousemoves don't thrash React state.
      if (frameRef.current == null) {
        frameRef.current = requestAnimationFrame(() => {
          frameRef.current = null
          onChange(latestPctRef.current)
        })
      }
    },
    [containerRef, onChange],
  )

  const handleMouseUp = useCallback(() => {
    setDragging(false)
    if (frameRef.current != null) {
      cancelAnimationFrame(frameRef.current)
      frameRef.current = null
    }
    // Persist the final value.
    try {
      window.localStorage.setItem(
        STORAGE_KEY,
        String(latestPctRef.current.toFixed(2)),
      )
    } catch {
      // ignore quota/security errors
    }
  }, [])

  useEffect(() => {
    if (!dragging) return
    window.addEventListener("mousemove", handleMouseMove)
    window.addEventListener("mouseup", handleMouseUp)
    // Prevent text selection during drag
    const prevUserSelect = document.body.style.userSelect
    document.body.style.userSelect = "none"
    document.body.style.cursor = "col-resize"
    return () => {
      window.removeEventListener("mousemove", handleMouseMove)
      window.removeEventListener("mouseup", handleMouseUp)
      document.body.style.userSelect = prevUserSelect
      document.body.style.cursor = ""
    }
  }, [dragging, handleMouseMove, handleMouseUp])

  return (
    <div
      role="separator"
      aria-orientation="vertical"
      aria-label="Resize chat and code panels"
      className={cn(
        "w-px h-full shrink-0 cursor-col-resize transition-colors duration-100",
        "bg-[var(--border-primary)] hover:bg-[var(--accent)]",
        dragging && "bg-[var(--accent)]",
        className,
      )}
      onMouseDown={(e) => {
        e.preventDefault()
        setDragging(true)
      }}
    />
  )
}
