"use client"

import { useRef, useState, type ReactNode } from "react"
import { cn } from "@/lib/utils"
import { MessageSquare, Code2 } from "lucide-react"

export type MobilePanel = "chat" | "code"

export interface MobilePanelSwitcherProps {
  chat: ReactNode
  code: ReactNode
  className?: string
  initial?: MobilePanel
}

// Horizontal swipe threshold in px — below this is treated as a tap, not a
// swipe. Matches common mobile gesture thresholds.
const SWIPE_THRESHOLD = 50

/**
 * Mobile (<md breakpoint) panel switcher for Agent Terminal. Shows one panel
 * at a time with a bottom segmented control, plus left/right swipe to flip.
 * Mockup does not explicitly spec mobile — we derive the behavior from the
 * plan-design review (D-6.1).
 */
export function MobilePanelSwitcher({
  chat,
  code,
  className,
  initial = "chat",
}: MobilePanelSwitcherProps) {
  const [panel, setPanel] = useState<MobilePanel>(initial)
  const touchStartX = useRef<number | null>(null)

  function onTouchStart(e: React.TouchEvent) {
    touchStartX.current = e.touches[0]?.clientX ?? null
  }

  function onTouchEnd(e: React.TouchEvent) {
    const start = touchStartX.current
    touchStartX.current = null
    if (start == null) return
    const end = e.changedTouches[0]?.clientX ?? start
    const dx = end - start
    if (Math.abs(dx) < SWIPE_THRESHOLD) return
    // Swipe right (dx > 0) → go to chat; swipe left → go to code
    if (dx > 0 && panel === "code") setPanel("chat")
    else if (dx < 0 && panel === "chat") setPanel("code")
  }

  return (
    <div
      className={cn("flex flex-col h-full min-h-0", className)}
      onTouchStart={onTouchStart}
      onTouchEnd={onTouchEnd}
    >
      <div className="flex-1 min-h-0 overflow-hidden">
        <div className={cn("h-full", panel !== "chat" && "hidden")}>{chat}</div>
        <div className={cn("h-full", panel !== "code" && "hidden")}>{code}</div>
      </div>

      {/* Bottom segmented control */}
      <nav
        role="tablist"
        aria-label="Panel"
        className="flex items-center h-10 border-t border-[var(--border-primary)] bg-[var(--bg-secondary)] shrink-0"
      >
        <button
          role="tab"
          aria-selected={panel === "chat"}
          aria-controls="mobile-chat-panel"
          onClick={() => setPanel("chat")}
          className={cn(
            "flex-1 flex items-center justify-center gap-1.5 h-full font-mono text-[11px] transition-colors duration-100",
            panel === "chat"
              ? "text-[var(--accent)] border-t-2 border-[var(--accent)] -mt-px"
              : "text-[var(--text-tertiary)]",
          )}
        >
          <MessageSquare className="h-3.5 w-3.5" />
          Chat
        </button>
        <div
          className="w-px h-5 bg-[var(--border-primary)]"
          aria-hidden
        />
        <button
          role="tab"
          aria-selected={panel === "code"}
          aria-controls="mobile-code-panel"
          onClick={() => setPanel("code")}
          className={cn(
            "flex-1 flex items-center justify-center gap-1.5 h-full font-mono text-[11px] transition-colors duration-100",
            panel === "code"
              ? "text-[var(--accent)] border-t-2 border-[var(--accent)] -mt-px"
              : "text-[var(--text-tertiary)]",
          )}
        >
          <Code2 className="h-3.5 w-3.5" />
          Code
        </button>
      </nav>
    </div>
  )
}
