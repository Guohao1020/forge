"use client"

import { useEffect, useRef, useState, useCallback } from "react"
import { cn } from "@/lib/utils"
import { Send, Bot, User, Code2, Eye, WifiOff } from "lucide-react"
import { ToolExecution } from "./tool-execution"
import { BuildCard } from "./build-card"
import type { ConnStatus } from "./status-bar"

// Agent avatar types for multi-agent pair pipeline visibility
type AgentRole = "user" | "assistant" | "coder" | "reviewer"

interface ChatMessage {
  id: string
  role: AgentRole
  content: string
  timestamp: number
  tools?: ToolCall[]
  build?: BuildInfo
}

interface ToolCall {
  name: string
  input: Record<string, unknown>
  output?: string
  isError?: boolean
  isLoading?: boolean
}

interface BuildInfo {
  status: "building" | "passed" | "failed"
  command: string
  output?: string
  durationMs?: number
}

interface AgentChatProps {
  projectId: string
  sessionId: string | null
  onSessionCreated?: (id: string) => void
  onCodeFiles?: (files: Array<{ path: string; content: string }>) => void
  onStepsUpdate?: (
    steps: Array<{ id: string; label: string; status: string }>,
  ) => void
  // Stream 3: state lift — page.tsx owns these for StatusBar to consume.
  onConnStatusChange?: (status: ConnStatus) => void
  onStatsUpdate?: (stats: { tokens: number; cost: number }) => void
  className?: string
}

const roleConfig: Record<AgentRole, { icon: React.ReactNode; label: string; color: string }> = {
  user: { icon: <User className="h-3.5 w-3.5" />, label: "You", color: "text-[var(--text)]" },
  assistant: { icon: <Bot className="h-3.5 w-3.5" />, label: "AI", color: "text-[var(--accent)]" },
  coder: { icon: <Code2 className="h-3.5 w-3.5" />, label: "Coder", color: "text-[var(--accent)]" },
  reviewer: { icon: <Eye className="h-3.5 w-3.5" />, label: "Reviewer", color: "text-[var(--code-keyword)]" },
}

export function AgentChat({
  projectId,
  sessionId,
  onSessionCreated,
  onCodeFiles,
  onStepsUpdate,
  onConnStatusChange,
  onStatsUpdate,
  className,
}: AgentChatProps) {
  // Placeholder callbacks for Stream 4 backend integration.
  void onCodeFiles
  void onStepsUpdate
  const [messages, setMessages] = useState<ChatMessage[]>([])
  const [input, setInput] = useState("")
  const [isStreaming, setIsStreaming] = useState(false)
  // 4-state SSE connection enum (Stream 3.3). `connecting` is the initial
  // state until onopen fires. `reconnecting` is while we back off between
  // attempts. `failed` is terminal after repeated reconnect failures.
  const [connStatus, setConnStatus] = useState<ConnStatus>("connecting")
  const retryCountRef = useRef(0)
  const [tokenCount, setTokenCount] = useState(0)
  const [costEstimate, setCostEstimate] = useState(0)
  const scrollRef = useRef<HTMLDivElement>(null)
  const inputRef = useRef<HTMLTextAreaElement>(null)
  const eventSourceRef = useRef<EventSource | null>(null)
  const sendingRef = useRef(false)

  // Notify parent of conn status changes for StatusBar.
  useEffect(() => {
    onConnStatusChange?.(connStatus)
  }, [connStatus, onConnStatusChange])

  // Notify parent of stats updates for StatusBar.
  useEffect(() => {
    onStatsUpdate?.({ tokens: tokenCount, cost: costEstimate })
  }, [tokenCount, costEstimate, onStatsUpdate])

  // Auto-scroll to bottom on new messages
  useEffect(() => {
    if (scrollRef.current) {
      scrollRef.current.scrollTop = scrollRef.current.scrollHeight
    }
  }, [messages])

  // SSE connection with auto-reconnect and 4-state enum.
  // State transitions:
  //   mount → connecting
  //   onopen → connected, reset retry count
  //   onerror + retries < MAX → reconnecting → backoff → connect()
  //   onerror + retries >= MAX → failed (terminal)
  useEffect(() => {
    if (!sessionId) {
      setConnStatus("connecting")
      return
    }

    const MAX_RETRIES = 5
    let lastEventId = ""
    let reconnectTimer: ReturnType<typeof setTimeout>
    let cancelled = false
    retryCountRef.current = 0
    setConnStatus("connecting")

    function connect() {
      if (cancelled) return
      const token = localStorage.getItem("forge_token") || ""
      const url = `/api/projects/${projectId}/agent/stream?session_id=${sessionId}${lastEventId ? `&last_event_id=${lastEventId}` : ""}&token=${token}`
      const es = new EventSource(url)
      eventSourceRef.current = es

      es.onopen = () => {
        if (cancelled) return
        retryCountRef.current = 0
        setConnStatus("connected")
      }

      es.addEventListener("agent", (e: MessageEvent) => {
        lastEventId = e.lastEventId || lastEventId
        try {
          const data = JSON.parse(e.data)
          handleStreamEvent(data)
        } catch {}
      })

      es.onerror = () => {
        if (cancelled) return
        es.close()
        retryCountRef.current += 1
        if (retryCountRef.current >= MAX_RETRIES) {
          setConnStatus("failed")
          return
        }
        setConnStatus("reconnecting")
        // Exponential backoff: 1s, 2s, 4s, 8s, 16s (capped at 16s)
        const delay = Math.min(16000, 1000 * 2 ** (retryCountRef.current - 1))
        reconnectTimer = setTimeout(connect, delay)
      }
    }

    connect()

    return () => {
      cancelled = true
      eventSourceRef.current?.close()
      clearTimeout(reconnectTimer)
    }
  }, [sessionId, projectId])

  function handleStreamEvent(data: Record<string, string>) {
    switch (data.type) {
      case "text_delta":
        setIsStreaming(true)
        setMessages(prev => {
          const last = prev[prev.length - 1]
          if (last && last.role !== "user" && !last.build) {
            return [...prev.slice(0, -1), { ...last, content: last.content + data.text }]
          }
          return [...prev, {
            id: crypto.randomUUID(),
            role: "assistant",
            content: data.text,
            timestamp: Date.now(),
          }]
        })
        break

      case "turn_complete":
        setIsStreaming(false)
        if (data.input_tokens) {
          const inp = parseInt(data.input_tokens) || 0
          const out = parseInt(data.output_tokens) || 0
          setTokenCount(prev => prev + inp + out)
          setCostEstimate(prev => prev + (inp * 0.000003 + out * 0.000015))
        }
        break

      case "tool_started":
        setMessages(prev => {
          const last = prev[prev.length - 1]
          if (last && last.role !== "user") {
            const tools = [...(last.tools || []), {
              name: data.tool_name,
              input: JSON.parse(data.tool_input || "{}"),
              isLoading: true,
            }]
            return [...prev.slice(0, -1), { ...last, tools }]
          }
          return prev
        })
        break

      case "tool_completed":
        setMessages(prev => {
          const last = prev[prev.length - 1]
          if (last?.tools) {
            const tools = last.tools.map(t =>
              t.name === data.tool_name && t.isLoading
                ? { ...t, output: data.output, isError: data.is_error === "true", isLoading: false }
                : t,
            )
            return [...prev.slice(0, -1), { ...last, tools }]
          }
          return prev
        })
        break

      case "error":
        setIsStreaming(false)
        setMessages(prev => [...prev, {
          id: crypto.randomUUID(),
          role: "assistant",
          content: `Error: ${data.message}`,
          timestamp: Date.now(),
        }])
        break
    }
  }

  const sendMessage = useCallback(async () => {
    if (!input.trim() || isStreaming || sendingRef.current) return
    sendingRef.current = true

    const userMsg: ChatMessage = {
      id: crypto.randomUUID(),
      role: "user",
      content: input.trim(),
      timestamp: Date.now(),
    }
    setMessages(prev => [...prev, userMsg])
    setInput("")

    try {
      const token = localStorage.getItem("forge_token")
      const resp = await fetch(`/api/projects/${projectId}/agent/chat`, {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
          ...(token ? { "Authorization": `Bearer ${token}` } : {}),
        },
        body: JSON.stringify({
          session_id: sessionId,
          message: userMsg.content,
        }),
      })
      const data = await resp.json()
      if (data.session_id && !sessionId) {
        onSessionCreated?.(data.session_id)
      }
    } catch {
      setMessages(prev => [...prev, {
        id: crypto.randomUUID(),
        role: "assistant",
        content: "Failed to send message. Check your connection.",
        timestamp: Date.now(),
      }])
    } finally {
      sendingRef.current = false
    }
  }, [input, isStreaming, sessionId, projectId, onSessionCreated])

  function handleKeyDown(e: React.KeyboardEvent) {
    if ((e.metaKey || e.ctrlKey) && e.key === "Enter") {
      e.preventDefault()
      sendMessage()
    }
  }

  // Show a banner only for failed (terminal) or reconnecting (mid-retry).
  // connecting (initial) and connected are silent.
  const showBanner = connStatus === "reconnecting" || connStatus === "failed"
  const bannerText =
    connStatus === "failed"
      ? "Connection lost. Please refresh the page to reconnect."
      : "Connection lost, reconnecting…"

  return (
    <div
      className={cn(
        "flex flex-col h-full bg-[var(--bg-primary)] min-h-0",
        className,
      )}
    >
      {/* Connection banner */}
      {showBanner && (
        <div
          role="alert"
          className={cn(
            "flex items-center gap-2 px-4 py-2 text-[11px] font-mono shrink-0",
            connStatus === "failed"
              ? "bg-[var(--bg-error)] text-[var(--text-error)]"
              : "bg-[var(--bg-warning)] text-[var(--text-warning)]",
          )}
        >
          <WifiOff className="h-3 w-3" />
          {bannerText}
        </div>
      )}

      {/* Messages */}
      <div ref={scrollRef} className="flex-1 overflow-y-auto px-2.5 py-2 space-y-1.5">
        {messages.length === 0 && (
          <div className="flex items-start justify-center pt-8">
            <div className="space-y-2 max-w-sm">
              <p className="font-mono text-[11px] text-[var(--text-tertiary)] mb-3">
                Describe what you want to build, or try:
              </p>
              {[
                "Add user registration with JWT auth",
                "Fix the login bug in feat/auth",
                "Write tests for the API",
              ].map((suggestion) => (
                <button
                  key={suggestion}
                  onClick={() => {
                    setInput(suggestion)
                    inputRef.current?.focus()
                  }}
                  className="block w-full text-left font-mono text-[11px] text-[var(--text-tertiary)] hover:text-[var(--accent)] transition-colors duration-100"
                >
                  <span className="text-[var(--text-tertiary)]">→ Try:</span>{" "}
                  <span>{suggestion}</span>
                </button>
              ))}
            </div>
          </div>
        )}

        {messages.map(msg => {
          const config = roleConfig[msg.role]
          return (
            <div key={msg.id} className="space-y-2 animate-in fade-in slide-in-from-bottom-1 duration-150">
              <div className="flex items-start gap-2">
                <div className={cn("mt-0.5 p-1 rounded", config.color)}>
                  {config.icon}
                </div>
                <div className="flex-1 min-w-0">
                  <div className="flex items-center gap-2 mb-1">
                    <span className={cn("text-xs font-medium", config.color)}>
                      {config.label}
                    </span>
                  </div>
                  {msg.content && (
                    <div className="text-xs leading-relaxed whitespace-pre-wrap">
                      {msg.content}
                      {isStreaming && msg === messages[messages.length - 1] && msg.role !== "user" && (
                        <span className="inline-block w-1.5 h-4 bg-[var(--accent)] ml-0.5 animate-blink" />
                      )}
                    </div>
                  )}
                  {msg.tools?.map((tool, i) => (
                    <div key={i} className="mt-2">
                      <ToolExecution
                        toolName={tool.name}
                        toolInput={tool.input}
                        output={tool.output}
                        isError={tool.isError}
                        isLoading={tool.isLoading}
                      />
                    </div>
                  ))}
                  {msg.build && (
                    <div className="mt-2">
                      <BuildCard
                        status={msg.build.status}
                        command={msg.build.command}
                        output={msg.build.output}
                        durationMs={msg.build.durationMs}
                      />
                    </div>
                  )}
                </div>
              </div>
            </div>
          )
        })}
      </div>

      {/* Input — compact, IDE-style. Textarea auto-grows 1..8 rows.
          Note: status bar is lifted to page.tsx StatusBar component (Stream 3). */}
      <div className="border-t border-[var(--border-primary)] bg-[var(--bg-secondary)] px-2.5 py-2 shrink-0">
        <div className="flex items-end gap-1.5">
          <textarea
            ref={inputRef}
            value={input}
            onChange={(e) => setInput(e.target.value)}
            onKeyDown={handleKeyDown}
            placeholder={
              isStreaming
                ? "AI is thinking..."
                : "Describe what you want to build..."
            }
            disabled={isStreaming}
            rows={Math.min(8, Math.max(1, input.split("\n").length))}
            aria-label="Chat with Forge Agent"
            className="flex-1 resize-none rounded border border-[var(--border-primary)] bg-[var(--bg-input)] px-2 py-1.5 text-[12px] leading-[1.4] font-sans focus:border-[var(--border-focus)] focus:outline-none disabled:opacity-50 transition-colors duration-100"
            style={{ maxHeight: "120px", overflowY: "auto" }}
          />
          <button
            onClick={sendMessage}
            disabled={
              !input.trim() ||
              isStreaming ||
              // Block subsequent sends when the stream isn't live. First-send
              // (sessionId is null) is always allowed — the POST /chat
              // response carries the session id before SSE opens.
              (sessionId != null && connStatus !== "connected")
            }
            className="flex items-center justify-center rounded bg-[var(--accent)] text-[var(--text-inverse)] h-7 w-7 hover:bg-[var(--accent-hover)] disabled:opacity-40 disabled:cursor-not-allowed transition-colors duration-100"
            aria-label="Send message"
          >
            <Send className="h-3 w-3" />
          </button>
        </div>
        <div className="font-mono text-[10px] text-[var(--text-tertiary)] mt-1">
          {isStreaming ? "" : "⌘+Enter to send"}
        </div>
      </div>
    </div>
  )
}
