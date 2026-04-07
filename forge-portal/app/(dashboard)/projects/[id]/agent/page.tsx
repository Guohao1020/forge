"use client"

import { useState } from "react"
import { useParams } from "next/navigation"
import { ArrowLeft, PanelRightClose, PanelRightOpen } from "lucide-react"
import Link from "next/link"
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
      {/* Header — 40px per design spec */}
      <header className="flex items-center h-10 px-4 border-b border-[var(--border)] bg-[var(--surface)]">
        <Link
          href={`/projects/${projectId}/tasks`}
          className="flex items-center gap-1.5 text-xs text-[var(--text-muted)] hover:text-[var(--text)] transition-colors duration-150"
        >
          <ArrowLeft className="h-3.5 w-3.5" />
          Tasks
        </Link>
        <div className="mx-3 w-px h-4 bg-[var(--border)]" />
        <h1 className="text-xs font-medium">Agent Terminal</h1>
        {sessionId && (
          <span className="ml-2 text-[0.65rem] text-[var(--text-dim)] font-mono">
            #{sessionId.slice(0, 8)}
          </span>
        )}
        <div className="flex-1" />
        <button
          onClick={() => setShowCodePanel(!showCodePanel)}
          className="p-1.5 text-[var(--text-muted)] hover:text-[var(--text)] transition-colors duration-150 mr-1"
          aria-label={showCodePanel ? "Hide code panel" : "Show code panel"}
        >
          {showCodePanel ? <PanelRightClose className="h-4 w-4" /> : <PanelRightOpen className="h-4 w-4" />}
        </button>
        <ThemeToggle />
      </header>

      {/* Step Ribbon */}
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
