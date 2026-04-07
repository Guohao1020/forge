"use client"

import { useEffect, useRef, useState, useCallback } from "react"
import { cn } from "@/lib/utils"
import { Send, Bot, User, Code2, Eye, AlertTriangle, WifiOff } from "lucide-react"
import { ToolExecution } from "./tool-execution"
import { BuildCard } from "./build-card"

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
  onStepsUpdate?: (steps: Array<{ id: string; label: string; status: string }>) => void
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
  className,
}: AgentChatProps) {
  const [messages, setMessages] = useState<ChatMessage[]>([])
  const [input, setInput] = useState("")
  const [isStreaming, setIsStreaming] = useState(false)
  const [connected, setConnected] = useState(true)
  const [tokenCount, setTokenCount] = useState(0)
  const [costEstimate, setCostEstimate] = useState(0)
  const scrollRef = useRef<HTMLDivElement>(null)
  const inputRef = useRef<HTMLTextAreaElement>(null)
  const eventSourceRef = useRef<EventSource | null>(null)
  const sendingRef = useRef(false)

  // Auto-scroll to bottom on new messages
  useEffect(() => {
    if (scrollRef.current) {
      scrollRef.current.scrollTop = scrollRef.current.scrollHeight
    }
  }, [messages])

  // SSE connection with auto-reconnect
  useEffect(() => {
    if (!sessionId) return

    let lastEventId = ""
    let reconnectTimer: NodeJS.Timeout

    function connect() {
      const url = `/api/projects/${projectId}/agent/stream?session_id=${sessionId}${lastEventId ? `&last_event_id=${lastEventId}` : ""}`
      const es = new EventSource(url)
      eventSourceRef.current = es

      es.onopen = () => setConnected(true)

      es.addEventListener("agent", (e: MessageEvent) => {
        lastEventId = e.lastEventId || lastEventId
        try {
          const data = JSON.parse(e.data)
          handleStreamEvent(data)
        } catch {}
      })

      es.onerror = () => {
        setConnected(false)
        es.close()
        reconnectTimer = setTimeout(connect, 3000)
      }
    }

    connect()

    return () => {
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
      const resp = await fetch(`/api/projects/${projectId}/agent/chat`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          session_id: sessionId,
          message: userMsg.content,
        }),
      })
      const data = await resp.json()
      if (data.session_id && !sessionId) {
        onSessionCreated?.(data.session_id)
      }
    } catch (e) {
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

  return (
    <div className={cn("flex flex-col h-full", className)}>
      {/* Connection banner */}
      {!connected && (
        <div className="flex items-center gap-2 px-4 py-2 bg-[var(--error-bg)] text-[var(--error)] text-xs">
          <WifiOff className="h-3.5 w-3.5" />
          Connection lost, reconnecting...
        </div>
      )}

      {/* Messages */}
      <div ref={scrollRef} className="flex-1 overflow-y-auto px-3 py-2 space-y-3">
        {messages.length === 0 && (
          <div className="flex items-center justify-center h-full">
            <div className="text-center space-y-3 max-w-sm">
              <Bot className="h-8 w-8 mx-auto text-[var(--accent)] opacity-40" />
              <p className="text-xs text-[var(--text-muted)]">
                Describe what you want to build
              </p>
              <div className="flex flex-wrap gap-2 justify-center">
                {["Add user registration", "Fix the login bug", "Write tests for the API"].map(suggestion => (
                  <button
                    key={suggestion}
                    onClick={() => setInput(suggestion)}
                    className="text-xs px-3 py-1.5 rounded border border-[var(--border)] text-[var(--text-muted)] hover:border-[var(--accent)] hover:text-[var(--accent)] transition-colors duration-150"
                  >
                    {suggestion}
                  </button>
                ))}
              </div>
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

      {/* Status bar — 20px dense */}
      <div className="flex items-center justify-between px-2.5 h-5 border-t border-[var(--border)] bg-[var(--surface)] text-[11px] text-[var(--text-dim)] font-mono shrink-0">
        <span>claude-sonnet-4-20250514</span>
        <span>{tokenCount.toLocaleString()} tokens (${costEstimate.toFixed(4)})</span>
      </div>

      {/* Input — compact, IDE-style */}
      <div className="border-t border-[var(--border)] px-2.5 py-2 shrink-0">
        <div className="flex items-end gap-1.5">
          <textarea
            ref={inputRef}
            value={input}
            onChange={e => setInput(e.target.value)}
            onKeyDown={handleKeyDown}
            placeholder={isStreaming ? "AI is thinking..." : "Describe what you want to build..."}
            disabled={isStreaming}
            rows={1}
            className="flex-1 resize-none rounded border border-[var(--border)] bg-[var(--background)] px-2.5 py-1.5 text-xs focus:border-[var(--accent)] focus:outline-none focus:ring-1 focus:ring-[var(--accent)] disabled:opacity-50 transition-colors duration-150"
          />
          <button
            onClick={sendMessage}
            disabled={!input.trim() || isStreaming}
            className="flex items-center justify-center rounded bg-[var(--accent)] text-white p-1.5 hover:bg-[var(--accent-hover)] disabled:opacity-50 transition-colors duration-150"
            aria-label="Send message"
          >
            <Send className="h-3.5 w-3.5" />
          </button>
        </div>
        <div className="text-[10px] text-[var(--text-dim)] mt-0.5">
          {isStreaming ? "" : "⌘+Enter to send"}
        </div>
      </div>
    </div>
  )
}
