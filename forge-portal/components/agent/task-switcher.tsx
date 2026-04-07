"use client"

import { useCallback, useEffect, useState } from "react"
import { cn } from "@/lib/utils"
import { Plus, MessageSquare, Trash2 } from "lucide-react"
import {
  listAgentSessions,
  createAgentSession,
  archiveAgentSession,
  type AgentSession,
} from "@/lib/agent"

interface TaskSwitcherProps {
  projectId: number
  activeSessionId: string | null
  onSessionSelect: (sessionId: string) => void
  className?: string
}

function formatRelativeTime(iso: string | null | undefined): string {
  if (!iso) return ""
  const then = new Date(iso).getTime()
  const now = Date.now()
  const sec = Math.max(1, Math.floor((now - then) / 1000))
  if (sec < 60) return `${sec}s ago`
  const min = Math.floor(sec / 60)
  if (min < 60) return `${min}m ago`
  const hr = Math.floor(min / 60)
  if (hr < 24) return `${hr}h ago`
  const d = Math.floor(hr / 24)
  if (d < 7) return `${d}d ago`
  return new Date(iso).toLocaleDateString()
}

/**
 * Sidebar that lists the project's recent agent sessions. ChatGPT-style
 * left column with a "New session" button at the top. Clicking a session
 * switches the main pane to it; archiving (trash icon on hover) soft
 * deletes.
 */
export function TaskSwitcher({
  projectId,
  activeSessionId,
  onSessionSelect,
  className,
}: TaskSwitcherProps) {
  const [sessions, setSessions] = useState<AgentSession[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)

  const refresh = useCallback(async () => {
    try {
      setLoading(true)
      setError(null)
      const res = await listAgentSessions(projectId)
      setSessions(res.sessions)
    } catch (e) {
      setError(e instanceof Error ? e.message : "Failed to load sessions")
    } finally {
      setLoading(false)
    }
  }, [projectId])

  useEffect(() => {
    refresh()
  }, [refresh])

  async function handleNewSession() {
    try {
      const session = await createAgentSession(projectId, {})
      // Prepend to the list; active selection flips to the new one.
      setSessions((prev) => [session, ...prev])
      onSessionSelect(session.id)
    } catch (e) {
      setError(e instanceof Error ? e.message : "Failed to create session")
    }
  }

  async function handleArchive(sessionId: string, e: React.MouseEvent) {
    e.stopPropagation()
    try {
      await archiveAgentSession(projectId, sessionId)
      setSessions((prev) => prev.filter((s) => s.id !== sessionId))
      // If we archived the active session, clear the selection so the
      // parent can decide what to render.
      if (sessionId === activeSessionId && sessions.length > 1) {
        const next = sessions.find((s) => s.id !== sessionId)
        if (next) onSessionSelect(next.id)
      }
    } catch (e) {
      setError(e instanceof Error ? e.message : "Failed to archive session")
    }
  }

  return (
    <aside
      className={cn(
        "flex flex-col h-full bg-[var(--bg-secondary)] border-r border-[var(--border-primary)] min-w-0",
        className,
      )}
      aria-label="Agent sessions"
    >
      <div className="flex items-center justify-between h-10 px-2 border-b border-[var(--border-primary)] shrink-0">
        <span className="font-mono text-[11px] font-semibold text-[var(--text-primary)]">
          Sessions
        </span>
        <button
          onClick={handleNewSession}
          className="inline-flex items-center gap-1 px-2 h-6 rounded text-[10px] font-mono text-[var(--accent)] hover:bg-[var(--accent-subtle)] transition-colors duration-100"
          aria-label="New session"
        >
          <Plus className="h-3 w-3" />
          New
        </button>
      </div>

      {error && (
        <div
          role="alert"
          className="px-2 py-1 font-mono text-[10px] text-[var(--text-error)] bg-[var(--bg-error)] border-b border-[var(--border-secondary)]"
        >
          {error}
        </div>
      )}

      <div
        className="flex-1 overflow-y-auto"
        role="list"
        aria-label="Session list"
      >
        {loading ? (
          <div className="px-2 py-2 font-mono text-[10px] text-[var(--text-tertiary)]">
            Loading sessions…
          </div>
        ) : sessions.length === 0 ? (
          <div className="px-2 py-2 font-mono text-[10px] text-[var(--text-tertiary)]">
            No sessions yet. Click New to start.
          </div>
        ) : (
          sessions.map((session) => {
            const isActive = session.id === activeSessionId
            const label =
              session.title ||
              (session.task_id ? `TASK-${session.task_id}` : session.id.slice(0, 8))
            return (
              <button
                key={session.id}
                role="listitem"
                onClick={() => onSessionSelect(session.id)}
                className={cn(
                  "group/session flex items-center w-full gap-1.5 px-2 py-1.5 text-left border-l-2 transition-colors duration-100",
                  isActive
                    ? "border-[var(--accent)] bg-[var(--accent-subtle)]"
                    : "border-transparent hover:bg-[var(--bg-hover)]",
                )}
              >
                <MessageSquare
                  className={cn(
                    "h-3 w-3 shrink-0",
                    isActive
                      ? "text-[var(--accent)]"
                      : "text-[var(--text-tertiary)]",
                  )}
                  aria-hidden
                />
                <div className="flex-1 min-w-0">
                  <div
                    className={cn(
                      "font-mono text-[11px] truncate",
                      isActive
                        ? "text-[var(--text-primary)]"
                        : "text-[var(--text-secondary)]",
                    )}
                  >
                    {label}
                  </div>
                  <div className="font-mono text-[9px] text-[var(--text-tertiary)]">
                    {formatRelativeTime(session.last_message_at || session.created_at)}
                    {session.task_id && (
                      <span className="ml-1 text-[var(--accent-text)]">
                        · TASK-{session.task_id}
                      </span>
                    )}
                  </div>
                </div>
                <button
                  onClick={(e) => handleArchive(session.id, e)}
                  className="opacity-0 group-hover/session:opacity-60 hover:opacity-100 transition-opacity p-0.5 rounded hover:bg-[var(--bg-error)] shrink-0"
                  aria-label={`Archive session ${label}`}
                >
                  <Trash2 className="h-3 w-3 text-[var(--text-error)]" />
                </button>
              </button>
            )
          })
        )}
      </div>
    </aside>
  )
}
