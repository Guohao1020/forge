"use client"

import { useState, type FormEvent } from "react"
import { cn } from "@/lib/utils"

interface ClarificationInputProps {
  question: string
  toolUseId: string
  onSubmit: (toolUseId: string, response: string) => Promise<void>
  disabled?: boolean
  className?: string
}

type State =
  | { kind: "editing" }
  | { kind: "submitting" }
  | { kind: "submitted" }
  | { kind: "error"; message: string }

const MAX_CHARS = 4096

/**
 * Inline form the agent uses to ask the human user a clarifying
 * question. Rendered below the current assistant message when
 * the SSE stream delivers a `clarification_requested` event.
 *
 * Four states:
 * - editing: user is typing a response (default)
 * - submitting: POST /api/sessions/{id}/clarify in flight
 * - submitted: success; waiting for the agent's tool_execution_completed
 *   event to arrive (at which point the parent unmounts this)
 * - error: POST failed; user can edit and retry
 *
 * The `disabled` prop is set by the parent when the whole session
 * is ending (e.g. clarification timeout) -- the form should not
 * accept new input in that case.
 */
export function ClarificationInput({
  question,
  toolUseId,
  onSubmit,
  disabled,
  className,
}: ClarificationInputProps) {
  const [response, setResponse] = useState("")
  const [state, setState] = useState<State>({ kind: "editing" })

  const handleSubmit = async (e: FormEvent) => {
    e.preventDefault()
    if (state.kind === "submitting" || state.kind === "submitted") return
    if (response.trim().length === 0) return
    setState({ kind: "submitting" })
    try {
      await onSubmit(toolUseId, response)
      setState({ kind: "submitted" })
    } catch (err) {
      setState({
        kind: "error",
        message: err instanceof Error ? err.message : "Unknown error",
      })
    }
  }

  const inputDisabled = disabled || state.kind === "submitting" || state.kind === "submitted"

  return (
    <form
      onSubmit={handleSubmit}
      className={cn(
        "border border-[var(--border-primary)] rounded p-3 my-2 bg-[var(--bg-secondary)]",
        className,
      )}
      aria-label="Clarification response"
    >
      <div className="text-[11px] font-medium text-[var(--text-secondary)] mb-1.5">
        The agent is asking:
      </div>
      <div className="text-[12px] text-[var(--text-primary)] mb-2.5 whitespace-pre-wrap">
        {question}
      </div>
      <textarea
        value={response}
        onChange={(e) => setResponse(e.target.value)}
        placeholder="Your response..."
        disabled={inputDisabled}
        maxLength={MAX_CHARS}
        rows={3}
        className={cn(
          "w-full px-2 py-1.5 text-[12px] font-mono rounded",
          "bg-[var(--bg-primary)] border border-[var(--border-primary)]",
          "text-[var(--text-primary)] placeholder:text-[var(--text-tertiary)]",
          "focus:outline-none focus:border-[var(--accent)]",
          "disabled:opacity-60 disabled:cursor-not-allowed",
          "resize-y mb-2",
        )}
        aria-label="Clarification response textarea"
      />
      {state.kind === "error" && (
        <div
          className="text-[11px] text-[var(--text-error)] mb-2"
          role="alert"
        >
          {state.message}
        </div>
      )}
      {state.kind === "submitted" && (
        <div className="text-[11px] text-[var(--text-tertiary)] mb-2">
          Submitted -- waiting for agent...
        </div>
      )}
      <div className="flex items-center justify-between">
        <div className="text-[10px] text-[var(--text-tertiary)]">
          {response.length}/{MAX_CHARS} characters
        </div>
        <button
          type="submit"
          disabled={
            disabled ||
            state.kind === "submitting" ||
            state.kind === "submitted" ||
            response.trim().length === 0
          }
          className={cn(
            "px-2.5 py-1 text-[11px] font-medium rounded",
            "bg-[var(--accent)] text-white",
            "hover:bg-[var(--accent-hover)]",
            "disabled:opacity-50 disabled:cursor-not-allowed",
            "transition-colors duration-100",
          )}
        >
          {state.kind === "submitting" && "Submitting..."}
          {state.kind === "submitted" && "Submitted"}
          {(state.kind === "editing" || state.kind === "error") &&
            "Submit Response"}
        </button>
      </div>
    </form>
  )
}
