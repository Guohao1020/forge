"use client"

import { useState } from "react"
import { useParams } from "next/navigation"
import { PanelRightClose, PanelRightOpen, Terminal } from "lucide-react"
import { AgentChat } from "@/components/agent/agent-chat"
import { StepRibbon, type Step } from "@/components/agent/step-ribbon"
import { CodePanel } from "@/components/agent/code-panel"
import { ThemeToggle } from "@/components/theme-toggle"

export default function AgentTerminalPage() {
  const params = useParams()
  const projectId = params.id as string

  const [sessionId, setSessionId] = useState<string | null>(null)
  const [steps, setSteps] = useState<Step[]>([])
  const [codeFiles, setCodeFiles] = useState<Array<{ path: string; content: string }>>([])
  const [diffContent, setDiffContent] = useState<string>()
  const [showCodePanel, setShowCodePanel] = useState(true)

  return (
    <div className="flex flex-col h-screen bg-[var(--background)]">
      {/* Header — 40px, dense engineering style */}
      <header className="flex items-center h-10 px-2.5 border-b border-[var(--border)] bg-[var(--surface)] gap-2">
        <div className="flex items-center gap-1.5">
          <div className="w-5 h-5 rounded bg-[var(--accent)] flex items-center justify-center">
            <Terminal className="h-3 w-3 text-white" />
          </div>
          <span className="text-xs font-semibold tracking-tight">Agent Terminal</span>
        </div>
        {sessionId && (
          <>
            <div className="w-px h-3.5 bg-[var(--border)]" />
            <span className="text-[11px] text-[var(--text-dim)] font-mono">
              {sessionId.slice(0, 8)}
            </span>
          </>
        )}
        <div className="flex-1" />
        <button
          onClick={() => setShowCodePanel(!showCodePanel)}
          className="p-1 text-[var(--text-muted)] hover:text-[var(--text)] hover:bg-[var(--hover)] rounded transition-colors duration-150"
          aria-label={showCodePanel ? "Hide code panel" : "Show code panel"}
        >
          {showCodePanel ? <PanelRightClose className="h-3.5 w-3.5" /> : <PanelRightOpen className="h-3.5 w-3.5" />}
        </button>
        <ThemeToggle />
      </header>

      {/* Step Ribbon — 40px */}
      <StepRibbon
        steps={steps}
        className="border-b border-[var(--border)] bg-[var(--surface)]"
      />

      {/* Main content: Chat (60%) + Code (40%) */}
      <div className="flex flex-1 min-h-0">
        <AgentChat
          projectId={projectId}
          sessionId={sessionId}
          onSessionCreated={setSessionId}
          onCodeFiles={setCodeFiles}
          onStepsUpdate={(s) => setSteps(s as Step[])}
          className={showCodePanel ? "w-3/5" : "w-full"}
        />
        {showCodePanel && (
          <CodePanel
            files={codeFiles}
            diffContent={diffContent}
            className="w-2/5 hidden md:flex"
          />
        )}
      </div>
    </div>
  )
}
