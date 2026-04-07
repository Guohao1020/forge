"use client"

import { useEffect, useRef, useState } from "react"
import { useParams } from "next/navigation"
import { Settings, Terminal } from "lucide-react"
import { AgentChat } from "@/components/agent/agent-chat"
import { StepRibbon, type Step } from "@/components/agent/step-ribbon"
import { CodePanel } from "@/components/agent/code-panel"
import {
  StatusBar,
  type BuildState,
  type ConnStatus,
} from "@/components/agent/status-bar"
import {
  PanelDivider,
  loadSplitPct,
} from "@/components/agent/panel-divider"
import { MobilePanelSwitcher } from "@/components/agent/mobile-panel-switcher"
import { ThemeToggle } from "@/components/theme-toggle"

// Derive build state from the step list for the status bar.
function buildStateFromSteps(steps: Step[]): BuildState {
  const buildStep = steps.find((s) => s.id.toLowerCase().includes("build"))
  if (!buildStep) return "idle"
  if (buildStep.status === "done") return "passed"
  if (buildStep.status === "failed") return "failed"
  if (buildStep.status === "active") {
    return buildStep.cycle && buildStep.cycle > 1 ? "fixing" : "building"
  }
  return "idle"
}

// Track window width via a media query listener (client-only).
function useIsMobile(): boolean {
  const [isMobile, setIsMobile] = useState(false)
  useEffect(() => {
    const mq = window.matchMedia("(max-width: 767px)")
    const update = () => setIsMobile(mq.matches)
    update()
    mq.addEventListener("change", update)
    return () => mq.removeEventListener("change", update)
  }, [])
  return isMobile
}

export default function AgentTerminalPage() {
  const params = useParams()
  const projectId = params.id as string

  // ---- Session + agent state ----
  const [sessionId, setSessionId] = useState<string | null>(null)
  const [steps, setSteps] = useState<Step[]>([])
  const [codeFiles, setCodeFiles] = useState<
    Array<{ path: string; content: string }>
  >([])
  const [diffContent] = useState<string | undefined>(undefined)

  // ---- Lifted from AgentChat (Stream 3) for StatusBar consumption ----
  const [connStatus, setConnStatus] = useState<ConnStatus>("connecting")
  const [stats, setStats] = useState({ tokens: 0, cost: 0 })

  // ---- Layout state ----
  // Lazy initializer reads localStorage only on the client; SSR gets the
  // default via typeof-window check inside loadSplitPct().
  const [splitPct, setSplitPct] = useState<number>(() => loadSplitPct())
  const mainRef = useRef<HTMLDivElement>(null)
  const isMobile = useIsMobile()

  const activeStep = steps.find((s) => s.status === "active")
  const activeStepIndex = activeStep
    ? steps.findIndex((s) => s.id === activeStep.id) + 1
    : undefined
  const buildState = buildStateFromSteps(steps)

  const chatPanel = (
    <AgentChat
      projectId={projectId}
      sessionId={sessionId}
      onSessionCreated={setSessionId}
      onCodeFiles={setCodeFiles}
      onStepsUpdate={(s) => setSteps(s as Step[])}
      onConnStatusChange={setConnStatus}
      onStatsUpdate={setStats}
    />
  )

  const codePanelEl = (
    <CodePanel files={codeFiles} diffContent={diffContent} />
  )

  return (
    <div
      className="h-screen w-full grid overflow-hidden bg-[var(--bg-primary)] text-[var(--text-primary)]"
      style={{
        // Mockup variant-B-dense.html lines 129-135: 40px header, 40px ribbon,
        // 1fr main, 20px status. The status-h token lives in :root.
        gridTemplateRows: "40px 40px 1fr 20px",
      }}
    >
      {/* ---- ROW 1: HEADER (40px) ---- */}
      <header
        role="banner"
        className="flex items-center h-10 px-2.5 gap-2 bg-[var(--bg-secondary)] border-b border-[var(--border-primary)]"
      >
        <div className="flex items-center gap-2 min-w-0">
          <div className="w-5 h-5 rounded-[3px] bg-[var(--accent)] flex items-center justify-center shrink-0">
            <Terminal className="h-3 w-3 text-white" />
          </div>
          <span className="text-[12px] font-semibold tracking-tight text-[var(--text-primary)]">
            Forge Agent
          </span>
          {sessionId && (
            <>
              <span
                className="w-px h-3.5 bg-[var(--border-primary)] shrink-0"
                aria-hidden
              />
              <span className="font-mono text-[11px] text-[var(--text-secondary)] truncate">
                {sessionId.slice(0, 8)}
              </span>
            </>
          )}
        </div>
        <div className="flex-1" />
        <div className="flex items-center gap-1 shrink-0">
          <span className="font-mono text-[10px] text-[var(--text-tertiary)] px-1.5 py-0.5 bg-[var(--bg-tertiary)] rounded-[3px] mr-1">
            ${stats.cost.toFixed(2)} / {(stats.tokens / 1000).toFixed(1)}k tok
          </span>
          <ThemeToggle />
          <button
            className="inline-flex items-center justify-center w-[26px] h-[26px] rounded text-[var(--text-secondary)] hover:bg-[var(--bg-hover)] hover:text-[var(--text-primary)] transition-colors duration-100"
            aria-label="Settings"
          >
            <Settings className="h-3.5 w-3.5" />
          </button>
        </div>
      </header>

      {/* ---- ROW 2: STEP RIBBON (40px) ---- */}
      <StepRibbon steps={steps} />

      {/* ---- ROW 3: MAIN (chat + divider + code) ---- */}
      <main
        ref={mainRef}
        id="agent-main"
        className="min-h-0 overflow-hidden"
        style={{
          // Desktop only — mobile uses MobilePanelSwitcher below. Grid is
          // {splitPct}% | 1px divider | {100-splitPct}%. Inline style because
          // Tailwind arbitrary value classes can't hold dynamic numeric values.
          display: isMobile ? "block" : "grid",
          gridTemplateColumns: `${splitPct}% 1px minmax(0, 1fr)`,
        }}
      >
        {isMobile ? (
          <MobilePanelSwitcher chat={chatPanel} code={codePanelEl} />
        ) : (
          <>
            <div className="min-w-0 overflow-hidden">{chatPanel}</div>
            <PanelDivider onChange={setSplitPct} containerRef={mainRef} />
            <div className="min-w-0 overflow-hidden">{codePanelEl}</div>
          </>
        )}
      </main>

      {/* ---- ROW 4: STATUS BAR (20px) ---- */}
      <StatusBar
        connStatus={connStatus}
        buildState={buildState}
        currentStep={activeStepIndex}
        maxSteps={steps.length}
        model="claude-sonnet-4"
        tokenCount={stats.tokens}
        costEstimate={stats.cost}
      />
    </div>
  )
}
