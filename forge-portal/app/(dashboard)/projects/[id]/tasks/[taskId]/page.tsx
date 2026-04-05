"use client";

import { useEffect, useState, useCallback, useRef } from "react";
import { useParams, useRouter } from "next/navigation";
import Link from "next/link";
import { ArrowLeft, Wifi, ExternalLink, Globe } from "lucide-react";
import { Badge } from "@/components/ui/badge";
import { StepTimeline } from "@/components/tasks/step-timeline";
import { TaskWorkspace } from "@/components/tasks/task-workspace";
import { ChatPanel } from "@/components/chat/chat-panel";
import { ActionPanel } from "@/components/chat/action-panel";
import { Risk } from "@/components/chat/risk-alert";
import { getTaskDetail, TaskDetail, TaskStep, STATUS_LABELS, STATUS_COLORS } from "@/lib/tasks";
import { useTaskStream, TaskStreamEvent } from "@/lib/use-task-stream";
import { getTaskPreview, PreviewEnvironment } from "@/lib/preview";
import { Conversation, PlanData, sendMessage, confirmPlan, approvePlan, triggerAnalysis, getHistory, cancelTask } from "@/lib/conversation";

const TERMINAL_STATUSES = ["COMPLETED", "FAILED", "CANCELLED"];

const PHASE_LABELS: Record<string, string> = {
  understanding: "初步理解",
  scenario: "场景澄清",
  constraints: "约束确认",
};

// Explicit phase state machine — replaces 5 independent useState variables.
// Every phase has clear entry/exit conditions.
//
//   analyzing → confirming → planning → plan_review → idle
//       ↓            ↓           ↓
//     error        error       error
//
type Phase = "analyzing" | "confirming" | "planning" | "plan_review" | "idle" | "error";

function pickDefaultStep(steps: TaskStep[]): TaskStep | null {
  if (!steps.length) return null;
  const running = steps.find((s) => s.status === "RUNNING");
  if (running) return running;
  const completed = [...steps].reverse().find((s) => s.status === "COMPLETED");
  if (completed) return completed;
  return steps[0];
}

export default function TaskDetailPage() {
  const params = useParams();
  const projectId = params.id as string;
  const taskId = params.taskId as string;
  const router = useRouter();
  const [detail, setDetail] = useState<TaskDetail | null>(null);
  const [loading, setLoading] = useState(true);
  const [selectedStep, setSelectedStep] = useState<TaskStep | null>(null);
  const [previewEnv, setPreviewEnv] = useState<PreviewEnvironment | null>(null);
  const userSelectedRef = useRef(false);

  // Single phase state machine
  const [phase, setPhase] = useState<Phase>("idle");
  const [errorMessage, setErrorMessage] = useState<string>("");

  // Conversation state
  const [messages, setMessages] = useState<Conversation[]>([]);
  const [chatLoading, setChatLoading] = useState(false);
  const [latestRisks, setLatestRisks] = useState<Risk[]>([]);
  const [confirmationData, setConfirmationData] = useState<{
    summary: string;
    taskTitle: string;
    affectedModules?: string[];
    riskLevel?: string;
    estimatedComplexity?: string;
    risks?: Risk[];
    nonFunctional?: string[];
    functionalRequirements?: string[];
    acceptanceCriteria?: string[];
    outOfScope?: string[];
  } | null>(null);
  const [planReviewData, setPlanReviewData] = useState<PlanData | null>(null);
  const [isPlanApproving, setIsPlanApproving] = useState(false);
  const [latestRecommendation, setLatestRecommendation] = useState<{
    options: Array<{ id: string; title: string; pros: string[]; cons: string[]; risk: "LOW" | "MEDIUM" | "HIGH"; recommended: boolean; reason: string }>;
    aiRecommendation: string;
    contextFactors?: string[];
  } | null>(null);
  const [latestOptions, setLatestOptions] = useState<string[]>([]);
  const [latestOptionDetails, setLatestOptionDetails] = useState<Array<{ label: string; reason: string }>>([]);
  const [currentQuestion, setCurrentQuestion] = useState<string>("");
  const [currentUnderstanding, setCurrentUnderstanding] = useState<string>("");
  const [currentPhaseLabel, setCurrentPhaseLabel] = useState<string>("");
  const initialAnalysisTriggered = useRef(false);
  const historyFetchingRef = useRef(false); // Prevent duplicate getHistory calls

  // --- Hooks ---

  const fetchDetail = useCallback(async () => {
    try {
      const data = await getTaskDetail(projectId, taskId);
      setDetail(data);
      if (data.task && ["DEPLOYING", "COMPLETED"].includes(data.task.status)) {
        try {
          const preview = await getTaskPreview(projectId, taskId);
          setPreviewEnv(preview);
        } catch { /* expected */ }
      }
    } catch { /* ignore */ } finally {
      setLoading(false);
    }
  }, [projectId, taskId]);

  const isTerminal = detail?.task ? TERMINAL_STATUSES.includes(detail.task.status) : false;
  const isConversationPhase = detail?.task
    ? ["SUBMITTED", "ANALYZING", "PLANNING"].includes(detail.task.status)
    : false;

  // --- Restore state from conversation history ---
  // Called on mount and on SSE reconnect to recover from missed events.
  const restoreStateFromHistory = useCallback(async () => {
    if (historyFetchingRef.current) return;
    historyFetchingRef.current = true;
    try {
      const msgs = await getHistory(Number(projectId), Number(taskId));
      setMessages(msgs);

      const taskStatus = detail?.task?.status;
      const lastAssistant = [...msgs].reverse().find((m) => m.role === "assistant");
      const meta = lastAssistant?.metadata as Record<string, unknown> | undefined;
      const messageType = meta?.message_type as string | undefined;

      // Strict message_type routing — no fallback guessing
      if (messageType === "plan" || (meta && Array.isArray(meta.tasks) && meta.risk_level)) {
        // Plan message exists
        setPlanReviewData(meta as PlanData);
        setPhase("plan_review");
        setConfirmationData(null);
        setLatestOptions([]);
        setCurrentQuestion("");
        return;
      }

      if (messageType === "plan_error") {
        setPhase("error");
        setErrorMessage((meta?.error as string) || "方案生成失败");
        return;
      }

      if (messageType === "analysis" && meta?.status === "confirmed") {
        // Analysis confirmed — check if we should show confirm card or planning spinner
        if (taskStatus === "ANALYZING") {
          // Still in ANALYZING, show confirmation card
          setConfirmationData({
            summary: (meta.summary as string) || "",
            taskTitle: (meta.task_title as string) || detail?.task?.title || "",
            affectedModules: meta.affected_modules as string[] | undefined,
            estimatedComplexity: meta.estimated_complexity as string | undefined,
            risks: meta.risks as Risk[] | undefined,
            functionalRequirements: meta.functional_requirements as string[] | undefined,
            acceptanceCriteria: meta.acceptance_criteria as string[] | undefined,
            outOfScope: meta.out_of_scope as string[] | undefined,
          });
          setPhase("confirming");
        } else if (taskStatus === "PLANNING") {
          // Already confirmed, plan being generated — check if plan message exists
          const planMsg = msgs.find((m) => {
            const mt = (m.metadata as Record<string, unknown> | undefined)?.message_type;
            return mt === "plan";
          });
          if (planMsg?.metadata) {
            setPlanReviewData(planMsg.metadata as PlanData);
            setPhase("plan_review");
          } else {
            // Plan not yet generated, show planning spinner
            setPhase("planning");
          }
        }
        return;
      }

      if (messageType === "analysis" && meta?.status === "clarify") {
        // Restore clarify state
        const opts = (meta.options || []) as string[];
        if (opts.length > 0) setLatestOptions(opts);
        const details = (meta.option_details || []) as Array<{ label: string; reason: string }>;
        if (details.length > 0) setLatestOptionDetails(details);
        setCurrentQuestion((meta.question as string) || "");
        setCurrentUnderstanding((meta.understanding as string) || "");
        const p = (meta.phase as string) || "";
        setCurrentPhaseLabel(PHASE_LABELS[p] || p);
        setPhase("analyzing");
        return;
      }

      // Determine phase from task status if no useful metadata
      if (taskStatus === "PLANNING") {
        // Check for plan message
        const planMsg = msgs.find((m) => {
          const mt = (m.metadata as Record<string, unknown> | undefined)?.message_type;
          return mt === "plan";
        });
        if (planMsg?.metadata) {
          setPlanReviewData(planMsg.metadata as PlanData);
          setPhase("plan_review");
        } else {
          setPhase("planning");
        }
      } else if (taskStatus && ["SUBMITTED", "ANALYZING"].includes(taskStatus)) {
        setPhase("analyzing");
      } else if (taskStatus && !TERMINAL_STATUSES.includes(taskStatus)) {
        setPhase("idle");
      }

      // Auto-trigger first analysis
      const hasAssistant = msgs.some((m) => m.role === "assistant");
      if (msgs.length >= 1 && !hasAssistant && !initialAnalysisTriggered.current) {
        initialAnalysisTriggered.current = true;
        setChatLoading(true);
        try {
          await triggerAnalysis(Number(projectId), Number(taskId));
        } catch (err) {
          console.error("[Chat] initial analysis error:", err);
          setChatLoading(false);
        }
      }
    } catch {
      // ignore
    } finally {
      historyFetchingRef.current = false;
    }
  }, [projectId, taskId, detail?.task?.status, detail?.task?.title]);

  // --- Event handlers ---

  const applyAnalysisResult = useCallback((meta: Record<string, unknown>) => {
    const status = (meta.status as string) || "clarify";
    if (status === "clarify") {
      const opts = (meta.options || []) as string[];
      setLatestOptions(opts);
      const details = (meta.option_details || []) as Array<{ label: string; reason: string }>;
      setLatestOptionDetails(details);
      setCurrentQuestion((meta.question as string) || "");
      setCurrentUnderstanding((meta.understanding as string) || "");
      const p = (meta.phase as string) || "";
      setCurrentPhaseLabel(PHASE_LABELS[p] || p);
      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      const recs = meta.recommendations as any;
      if (recs?.options?.length > 0) {
        setLatestRecommendation({
          options: recs.options,
          aiRecommendation: recs.ai_recommendation || recs.aiRecommendation || "",
          contextFactors: recs.context_factors || recs.contextFactors,
        });
      } else {
        setLatestRecommendation(null);
      }
      setPhase("analyzing");
    }
    if (status === "confirmed") {
      setLatestOptions([]);
      setCurrentQuestion("");
      setConfirmationData({
        summary: (meta.summary as string) || "",
        taskTitle: (meta.task_title as string) || (meta.taskTitle as string) || detail?.task?.title || "",
        affectedModules: (meta.affected_modules || meta.affectedModules) as string[] | undefined,
        riskLevel: (meta.estimated_complexity || meta.riskLevel) as string | undefined,
        estimatedComplexity: (meta.estimated_complexity || meta.estimatedComplexity) as string | undefined,
        risks: meta.risks as Risk[] | undefined,
        nonFunctional: meta.nonFunctional as string[] | undefined,
        functionalRequirements: meta.functional_requirements as string[] | undefined,
        acceptanceCriteria: meta.acceptance_criteria as string[] | undefined,
        outOfScope: meta.out_of_scope as string[] | undefined,
      });
      if (meta.risks) setLatestRisks(meta.risks as Risk[]);
      setPhase("confirming");
    }
  }, [detail?.task?.title]);

  const handleStreamEvent = useCallback((event: TaskStreamEvent) => {
    if (event.type === "TASK_PROGRESS" || event.type === "STEPS_UPDATE" || event.type === "TASK_COMPLETE" || event.type === "FULL_STATE") {
      fetchDetail();
    }

    // SSE reconnect catch-up: re-fetch state to recover from missed events
    if (event.type === "connected") {
      restoreStateFromHistory();
    }

    if (event.type === "ANALYSIS_COMPLETE") {
      setChatLoading(false);
      getHistory(Number(projectId), Number(taskId)).then((msgs) => {
        setMessages(msgs);
        const lastAssistant = [...msgs].reverse().find((m) => m.role === "assistant");
        if (lastAssistant?.metadata) {
          applyAnalysisResult(lastAssistant.metadata as Record<string, unknown>);
        }
      }).catch((err) => {
        console.error("[Chat] failed to fetch messages after analysis:", err);
      });
    }

    if (event.type === "PLAN_COMPLETE") {
      // Plan generation completed (success or error) — fetch the plan message
      getHistory(Number(projectId), Number(taskId)).then((msgs) => {
        setMessages(msgs);
        if (event.status === "error") {
          setPhase("error");
          setErrorMessage(event.data || "方案生成失败，请重试");
          return;
        }
        // Find the plan message
        const planMsg = [...msgs].reverse().find((m) => {
          const meta = m.metadata as Record<string, unknown> | undefined;
          return meta?.message_type === "plan";
        });
        if (planMsg?.metadata) {
          setPlanReviewData(planMsg.metadata as PlanData);
          setPhase("plan_review");
        }
        fetchDetail();
      }).catch((err) => {
        console.error("[Chat] failed to fetch plan:", err);
        setPhase("error");
        setErrorMessage("获取方案失败，请刷新页面重试");
      });
    }
  }, [fetchDetail, projectId, taskId, applyAnalysisResult, restoreStateFromHistory]);

  const { connected, streamingTokens, isStreaming, analyzeThinking, isAnalyzing, resetAnalyzing } = useTaskStream({
    taskId,
    onEvent: handleStreamEvent,
    enabled: !isTerminal,
  });

  // --- Effects ---

  useEffect(() => { fetchDetail(); }, [fetchDetail]);

  // Load conversation history and restore state on mount
  useEffect(() => {
    if (!isConversationPhase) return;
    restoreStateFromHistory();
  }, [isConversationPhase, restoreStateFromHistory]);

  // Auto-select step
  useEffect(() => {
    if (!detail?.steps) return;
    const steps = detail.steps;
    if (!userSelectedRef.current) {
      setSelectedStep(pickDefaultStep(steps));
    } else {
      setSelectedStep((prev) => {
        if (!prev) return prev;
        return steps.find((s) => s.id === prev.id) || prev;
      });
    }
    const running = steps.find((s) => s.status === "RUNNING");
    if (running) {
      setSelectedStep(running);
      userSelectedRef.current = false;
    }
  }, [detail?.steps]);

  // --- Action handlers ---

  const handleChatSend = useCallback(async (content: string) => {
    const userMsg: Conversation = {
      id: Date.now(),
      taskId: Number(taskId),
      role: "user",
      content,
      createdAt: new Date().toISOString(),
    };
    setMessages((prev) => [...prev, userMsg]);
    setLatestOptions([]);
    setCurrentQuestion("");
    setChatLoading(true);
    setPhase("analyzing");
    try {
      await sendMessage(Number(projectId), Number(taskId), content);
    } catch (err) {
      console.error("[Chat] send error:", err);
      setMessages((prev) => prev.filter((m) => m.id !== userMsg.id));
      setChatLoading(false);
      setPhase("error");
      setErrorMessage(err instanceof Error ? err.message : "发送消息失败");
    }
  }, [projectId, taskId]);

  const handleChatConfirm = useCallback(async () => {
    // Transition: confirming → planning
    setPhase("planning");
    setConfirmationData(null);
    setLatestOptions([]);
    setLatestOptionDetails([]);
    setCurrentQuestion("");
    resetAnalyzing();
    try {
      await confirmPlan(Number(projectId), Number(taskId));
      // Plan result will arrive via SSE PLAN_COMPLETE event
      fetchDetail();
    } catch (err) {
      console.error("[Chat] confirm error:", err);
      setPhase("error");
      setErrorMessage(err instanceof Error ? err.message : "确认方案失败");
    }
  }, [projectId, taskId, fetchDetail, resetAnalyzing]);

  const handleApprovePlan = useCallback(async () => {
    setIsPlanApproving(true);
    try {
      await approvePlan(Number(projectId), Number(taskId));
      setPlanReviewData(null);
      setPhase("idle");
      fetchDetail();
    } catch (err) {
      console.error("[Chat] approve plan error:", err);
      setPhase("error");
      setErrorMessage(err instanceof Error ? err.message : "批准方案失败");
    } finally {
      setIsPlanApproving(false);
    }
  }, [projectId, taskId, fetchDetail]);

  const handleChatModify = useCallback(() => {
    setConfirmationData(null);
    setPlanReviewData(null);
    setPhase("analyzing");
  }, []);

  const handleChatCancel = useCallback(async () => {
    if (!confirm("确定要取消此任务吗？取消后不可恢复。")) return;
    try {
      await cancelTask(Number(projectId), Number(taskId));
      setConfirmationData(null);
      setPlanReviewData(null);
      setPhase("idle");
      router.push(`/projects/${projectId}`);
    } catch (err) {
      console.error("[Chat] cancel error:", err);
      setPhase("error");
      setErrorMessage(err instanceof Error ? err.message : "取消失败");
    }
  }, [router, projectId, taskId]);

  const handleRetry = useCallback(() => {
    setPhase("analyzing");
    setErrorMessage("");
  }, []);

  const handleStepClick = useCallback((step: TaskStep) => {
    userSelectedRef.current = true;
    setSelectedStep(step);
  }, []);

  // --- Render ---

  if (loading) {
    return (
      <div className="flex h-[calc(100vh-64px)]">
        <div className="w-[240px] shrink-0 border-r border-border p-4 space-y-4">
          <div className="h-6 w-32 rounded bg-card animate-pulse" />
          <div className="space-y-3 mt-6">
            {[1, 2, 3, 4].map((i) => (
              <div key={i} className="h-10 rounded bg-card animate-pulse" />
            ))}
          </div>
        </div>
        <div className="flex-1 p-6">
          <div className="h-64 rounded-xl bg-card animate-pulse" />
        </div>
      </div>
    );
  }

  if (!detail) {
    return (
      <div className="flex items-center justify-center h-[calc(100vh-64px)]">
        <p className="text-muted-foreground">任务不存在</p>
      </div>
    );
  }

  const { task, steps } = detail;
  const color = STATUS_COLORS[task.status] || "#8888A0";

  return (
    <div className="flex h-[calc(100vh-64px)]">
      {/* Column 1: Step Timeline */}
      <div className="w-[240px] shrink-0 border-r border-border overflow-y-auto">
        <div className="p-4">
          <Link
            href={`/projects/${projectId}`}
            className="flex items-center gap-1.5 text-xs text-muted-foreground hover:text-foreground transition-colors mb-4"
          >
            <ArrowLeft className="h-3.5 w-3.5" />
            返回任务列表
          </Link>

          <div className="mb-5">
            <h1 className="text-sm font-semibold leading-snug mb-2">
              {task.title || "任务详情"}
            </h1>
            <div className="flex items-center gap-2 flex-wrap">
              <Badge variant="secondary" className="text-[10px]" style={{ color, borderColor: `${color}40` }}>
                {STATUS_LABELS[task.status] || task.status}
              </Badge>
              <span className="text-[10px] text-muted-foreground">#{task.id}</span>
              {task.review_score != null && (
                <span className={`text-[10px] font-mono ${task.review_score >= 90 ? "text-emerald-400" : task.review_score >= 70 ? "text-yellow-400" : "text-red-400"}`}>
                  评分 {task.review_score}
                </span>
              )}
              {task.mr_url && (
                <a href={task.mr_url} target="_blank" rel="noopener noreferrer"
                  className="flex items-center gap-1 text-[10px] text-primary hover:text-primary/80">
                  <ExternalLink size={10} /> PR
                </a>
              )}
              {previewEnv?.previewUrl && previewEnv.status === "READY" && (
                <a href={previewEnv.previewUrl} target="_blank" rel="noopener noreferrer"
                  className="flex items-center gap-1 text-[10px] text-emerald-400 hover:text-emerald-300">
                  <Globe size={10} /> Preview
                </a>
              )}
              {!isTerminal && connected && (
                <span className="flex items-center gap-1 text-[10px] text-emerald-400">
                  <Wifi size={10} /> 实时
                </span>
              )}
            </div>
          </div>

          <div className="mb-2">
            <h2 className="text-xs text-muted-foreground/60 uppercase tracking-wide mb-3">执行步骤</h2>
            <StepTimeline
              steps={steps || []}
              selectedStepId={selectedStep?.id}
              onStepClick={handleStepClick}
              taskTerminal={isTerminal}
            />
          </div>
        </div>
      </div>

      {/* Column 2+3: Content area */}
      {isConversationPhase ? (
        /* --- CONVERSATION PHASE: Chat + Action Panel --- */
        <div className="flex-1 flex min-w-0">
          {/* Column 2: Chat messages */}
          <div className="flex-1 min-w-[300px] border-r border-border">
            <ChatPanel
              messages={messages}
              onSend={handleChatSend}
              isLoading={chatLoading}
              disabled={phase === "confirming" || phase === "plan_review" || phase === "planning"}
              placeholder={
                phase === "confirming" ? "请先确认或修改需求" :
                phase === "plan_review" ? "请先审查方案" :
                phase === "planning" ? "正在生成方案..." :
                undefined
              }
            />
          </div>

          {/* Column 3: Action Panel */}
          <div className="w-[420px] shrink-0 bg-muted/10">
            <ActionPanel
              phase={phase === "error" ? "error" : phase === "confirming" ? "confirming" : phase === "plan_review" ? "plan_review" : phase === "planning" ? "planning" : "analyzing"}
              isLoading={chatLoading}
              latestOptions={latestOptions}
              latestOptionDetails={latestOptionDetails}
              recommendation={latestRecommendation}
              risks={latestRisks}
              analyzeThinking={analyzeThinking}
              isAnalyzing={isAnalyzing}
              currentQuestion={currentQuestion}
              currentUnderstanding={currentUnderstanding}
              currentPhaseLabel={currentPhaseLabel}
              onSelectOption={(opt) => handleChatSend(opt)}
              confirmationData={confirmationData}
              isConfirming={false}
              onConfirm={handleChatConfirm}
              onModify={handleChatModify}
              onCancel={handleChatCancel}
              planReviewData={planReviewData}
              isPlanApproving={isPlanApproving}
              onApprovePlan={handleApprovePlan}
              errorMessage={errorMessage}
              onRetry={handleRetry}
            />
          </div>
        </div>
      ) : (
        /* --- EXECUTION PHASE: Step Workspace --- */
        <div className="flex-1 overflow-y-auto p-6">
          <TaskWorkspace
            selectedStep={selectedStep}
            steps={steps || []}
            requirement={task.requirement}
            streamingTokens={streamingTokens}
            isStreaming={isStreaming}
          />
        </div>
      )}
    </div>
  );
}
