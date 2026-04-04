"use client";

import { useEffect, useState, useCallback, useRef } from "react";
import { useParams, useRouter } from "next/navigation";
import Link from "next/link";
import { ArrowLeft, Wifi, ExternalLink, Globe } from "lucide-react";
import { Badge } from "@/components/ui/badge";
import { StepTimeline } from "@/components/tasks/step-timeline";
import { TaskWorkspace } from "@/components/tasks/task-workspace";
import { ChatPanel } from "@/components/chat/chat-panel";
import { Risk } from "@/components/chat/risk-alert";
import { getTaskDetail, TaskDetail, TaskStep, STATUS_LABELS, STATUS_COLORS } from "@/lib/tasks";
import { useTaskStream, TaskStreamEvent } from "@/lib/use-task-stream";
import { getTaskPreview, PreviewEnvironment } from "@/lib/preview";
import { Conversation, SendMessageResponse, PlanConfirmResponse, sendMessage, confirmPlan, approvePlan, triggerAnalysis, getHistory, cancelTask } from "@/lib/conversation";

const TERMINAL_STATUSES = ["COMPLETED", "FAILED", "CANCELLED"];

/**
 * Pick the best step to auto-select:
 * 1. First RUNNING step
 * 2. Last COMPLETED step
 * 3. First step
 */
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
  // Track whether user has manually clicked a step
  const userSelectedRef = useRef(false);

  // Conversation state (for SUBMITTED/ANALYZING tasks)
  const [messages, setMessages] = useState<Conversation[]>([]);
  const [chatLoading, setChatLoading] = useState(false);
  const [isConfirming, setIsConfirming] = useState(false);
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
  const [planReviewData, setPlanReviewData] = useState<PlanConfirmResponse["planData"] | null>(null);
  const [isPlanApproving, setIsPlanApproving] = useState(false);
  const [latestRecommendation, setLatestRecommendation] = useState<{
    options: Array<{ id: string; title: string; pros: string[]; cons: string[]; risk: "LOW" | "MEDIUM" | "HIGH"; recommended: boolean; reason: string }>;
    aiRecommendation: string;
    contextFactors?: string[];
  } | null>(null);
  // Clickable options from AI clarify response
  const [latestOptions, setLatestOptions] = useState<string[]>([]);
  // Track whether initial analysis has been triggered
  const initialAnalysisTriggered = useRef(false);

  const fetchDetail = useCallback(async () => {
    try {
      const data = await getTaskDetail(projectId, taskId);
      setDetail(data);
      // Fetch preview environment for this task
      const preview = await getTaskPreview(projectId, taskId);
      setPreviewEnv(preview);
    } catch {
      // ignore
    } finally {
      setLoading(false);
    }
  }, [projectId, taskId]);

  const isTerminal = detail?.task ? TERMINAL_STATUSES.includes(detail.task.status) : false;
  const isConversationPhase = detail?.task
    ? ["SUBMITTED", "ANALYZING", "PLANNING"].includes(detail.task.status)
    : false;

  // Load conversation history and auto-trigger initial analysis
  useEffect(() => {
    if (!isConversationPhase) return;
    getHistory(Number(projectId), Number(taskId))
      .then(async (msgs) => {
        setMessages(msgs);

        // Extract options from the last assistant message's metadata (for page refresh)
        const lastAssistant = [...msgs].reverse().find((m) => m.role === "assistant");
        if (lastAssistant?.metadata) {
          const meta = lastAssistant.metadata as Record<string, unknown>;
          const opts = (meta.options || []) as string[];
          if (opts.length > 0) {
            setLatestOptions(opts);
          }
          // Check if last message was a confirmed status → show confirmation card
          if (meta.status === "confirmed") {
            setConfirmationData({
              summary: (meta.summary as string) || "",
              taskTitle: (meta.task_title as string) || detail?.task?.title || "",
              affectedModules: (meta.affected_modules) as string[] | undefined,
              estimatedComplexity: (meta.estimated_complexity) as string | undefined,
              risks: meta.risks as Risk[] | undefined,
              functionalRequirements: meta.functional_requirements as string[] | undefined,
              acceptanceCriteria: meta.acceptance_criteria as string[] | undefined,
              outOfScope: meta.out_of_scope as string[] | undefined,
            });
          }
        }

        // Auto-trigger analysis if only the initial user message exists and no AI response yet
        const hasAssistant = msgs.some((m) => m.role === "assistant");
        if (msgs.length >= 1 && !hasAssistant && !initialAnalysisTriggered.current) {
          initialAnalysisTriggered.current = true;
          setChatLoading(true);
          try {
            const res = await triggerAnalysis(Number(projectId), Number(taskId));
            setMessages((prev) => [...prev, res.conversation]);
            // Extract options for clickable buttons
            if (res.status === "clarify" && res.metadata) {
              const opts = (res.metadata.options || []) as string[];
              setLatestOptions(opts);
            } else {
              setLatestOptions([]);
            }
            if (res.status === "confirmed" && res.metadata) {
              setLatestOptions([]);
              setConfirmationData({
                summary: (res.metadata.summary as string) || "",
                taskTitle: (res.metadata.task_title as string) || (res.metadata.taskTitle as string) || detail?.task?.title || "",
                affectedModules: (res.metadata.affected_modules || res.metadata.affectedModules) as string[] | undefined,
                riskLevel: (res.metadata.estimated_complexity || res.metadata.riskLevel) as string | undefined,
                estimatedComplexity: (res.metadata.estimated_complexity || res.metadata.estimatedComplexity) as string | undefined,
                risks: res.metadata.risks as Risk[] | undefined,
                nonFunctional: res.metadata.nonFunctional as string[] | undefined,
                functionalRequirements: res.metadata.functional_requirements as string[] | undefined,
                acceptanceCriteria: res.metadata.acceptance_criteria as string[] | undefined,
                outOfScope: res.metadata.out_of_scope as string[] | undefined,
              });
            }
          } catch (err) {
            console.error("[Chat] initial analysis error:", err);
          } finally {
            setChatLoading(false);
          }
        }
      })
      .catch(() => {});
  }, [isConversationPhase, projectId, taskId, detail?.task?.title]);

  const handleChatSend = useCallback(async (content: string) => {
    // Optimistically add user message
    const userMsg: Conversation = {
      id: Date.now(),
      taskId: Number(taskId),
      role: "user",
      content,
      createdAt: new Date().toISOString(),
    };
    setMessages((prev) => [...prev, userMsg]);
    setLatestOptions([]); // Clear previous options while waiting
    setChatLoading(true);

    try {
      const res: SendMessageResponse = await sendMessage(Number(projectId), Number(taskId), content);
      // Add assistant reply
      setMessages((prev) => [...prev, res.conversation]);

      // Extract options for clickable buttons
      if (res.status === "clarify" && res.metadata) {
        const opts = (res.metadata.options || []) as string[];
        setLatestOptions(opts);

        // Check for AI recommendations (SP-2)
        const recs = res.metadata.recommendations as any;
        if (recs && recs.options && recs.options.length > 0) {
          setLatestRecommendation({
            options: recs.options,
            aiRecommendation: recs.ai_recommendation || recs.aiRecommendation || "",
            contextFactors: recs.context_factors || recs.contextFactors,
          });
        } else {
          setLatestRecommendation(null);
        }
      } else {
        setLatestOptions([]);
        setLatestRecommendation(null);
      }

      if (res.status === "confirmed" && res.metadata) {
        setLatestOptions([]);
        setConfirmationData({
          summary: (res.metadata.summary as string) || "",
          taskTitle: (res.metadata.task_title as string) || (res.metadata.taskTitle as string) || detail?.task?.title || "",
          affectedModules: (res.metadata.affected_modules || res.metadata.affectedModules) as string[] | undefined,
          riskLevel: (res.metadata.estimated_complexity || res.metadata.riskLevel) as string | undefined,
          estimatedComplexity: (res.metadata.estimated_complexity || res.metadata.estimatedComplexity) as string | undefined,
          risks: res.metadata.risks as Risk[] | undefined,
          nonFunctional: res.metadata.nonFunctional as string[] | undefined,
          functionalRequirements: res.metadata.functional_requirements as string[] | undefined,
          acceptanceCriteria: res.metadata.acceptance_criteria as string[] | undefined,
          outOfScope: res.metadata.out_of_scope as string[] | undefined,
        });
        if (res.metadata.risks) {
          setLatestRisks(res.metadata.risks as Risk[]);
        }
      }
    } catch (err) {
      console.error("[Chat] send error:", err);
      // Remove optimistic message on error
      setMessages((prev) => prev.filter((m) => m.id !== userMsg.id));
    } finally {
      setChatLoading(false);
    }
  }, [projectId, taskId, detail?.task?.title]);

  const handleChatConfirm = useCallback(async () => {
    setIsConfirming(true);
    try {
      const result = await confirmPlan(Number(projectId), Number(taskId));
      setConfirmationData(null);
      // Add the plan message to conversation
      if (result.conversation) {
        setMessages((prev) => [...prev, result.conversation]);
      }
      // Show plan review card
      if (result.planData) {
        setPlanReviewData(result.planData);
      }
      fetchDetail();
    } catch (err) {
      console.error("[Chat] confirm error:", err);
    } finally {
      setIsConfirming(false);
    }
  }, [projectId, taskId, fetchDetail]);

  const handleApprovePlan = useCallback(async () => {
    setIsPlanApproving(true);
    try {
      await approvePlan(Number(projectId), Number(taskId));
      setPlanReviewData(null);
      fetchDetail();
    } catch (err) {
      console.error("[Chat] approve plan error:", err);
    } finally {
      setIsPlanApproving(false);
    }
  }, [projectId, taskId, fetchDetail]);

  const handleChatModify = useCallback(() => {
    // Clear confirmation or plan review, allow further conversation
    setConfirmationData(null);
    setPlanReviewData(null);
  }, []);

  const handleChatCancel = useCallback(async () => {
    if (!confirm("确定要取消此任务吗？取消后不可恢复。")) return;
    try {
      await cancelTask(Number(projectId), Number(taskId));
      setConfirmationData(null);
      setPlanReviewData(null);
      router.push(`/projects/${projectId}`);
    } catch (err) {
      console.error("[Chat] cancel error:", err);
      alert("取消失败：" + (err instanceof Error ? err.message : "未知错误"));
    }
  }, [router, projectId, taskId]);

  const handleStreamEvent = useCallback((event: TaskStreamEvent) => {
    if (event.type === "TASK_PROGRESS" || event.type === "STEPS_UPDATE" || event.type === "TASK_COMPLETE" || event.type === "FULL_STATE") {
      fetchDetail();
    }
  }, [fetchDetail]);

  const { connected, streamingTokens, isStreaming, analyzeThinking, isAnalyzing } = useTaskStream({
    taskId,
    onEvent: handleStreamEvent,
    enabled: !isTerminal,
  });

  useEffect(() => {
    fetchDetail();
  }, [fetchDetail]);

  // Auto-select step when steps update (unless user has manually selected)
  useEffect(() => {
    if (!detail?.steps) return;
    const steps = detail.steps;

    if (!userSelectedRef.current) {
      // Auto-select: pick running or last completed
      const autoStep = pickDefaultStep(steps);
      setSelectedStep(autoStep);
    } else {
      // User has selected — update the step object to latest data but keep the selection
      setSelectedStep((prev) => {
        if (!prev) return prev;
        const updated = steps.find((s) => s.id === prev.id);
        return updated || prev;
      });
    }

    // If a step is RUNNING, auto-switch to it even if user previously selected
    const running = steps.find((s) => s.status === "RUNNING");
    if (running) {
      setSelectedStep(running);
      userSelectedRef.current = false;
    }
  }, [detail?.steps]);

  const handleStepClick = useCallback((step: TaskStep) => {
    userSelectedRef.current = true;
    setSelectedStep(step);
  }, []);

  if (loading) {
    return (
      <div className="flex h-[calc(100vh-64px)]">
        <div className="w-[280px] shrink-0 border-r border-white/10 p-4 space-y-4">
          <div className="h-6 w-32 rounded bg-card animate-pulse" />
          <div className="h-4 w-24 rounded bg-card animate-pulse" />
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
      {/* Left panel: Timeline */}
      <div className="w-[280px] shrink-0 border-r border-white/10 overflow-y-auto">
        <div className="p-4">
          {/* Back link */}
          <Link
            href={`/projects/${projectId}`}
            className="flex items-center gap-1.5 text-xs text-muted-foreground hover:text-foreground transition-colors mb-4"
          >
            <ArrowLeft className="h-3.5 w-3.5" />
            返回任务列表
          </Link>

          {/* Task header */}
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
                <a
                  href={task.mr_url}
                  target="_blank"
                  rel="noopener noreferrer"
                  className="flex items-center gap-1 text-[10px] text-primary hover:text-primary/80 transition-colors"
                >
                  <ExternalLink size={10} />
                  PR
                </a>
              )}
              {previewEnv?.previewUrl && previewEnv.status === "READY" && (
                <a
                  href={previewEnv.previewUrl}
                  target="_blank"
                  rel="noopener noreferrer"
                  className="flex items-center gap-1 text-[10px] text-emerald-400 hover:text-emerald-300 transition-colors"
                >
                  <Globe size={10} />
                  Preview
                </a>
              )}
              {!isTerminal && connected && (
                <span className="flex items-center gap-1 text-[10px] text-emerald-400">
                  <Wifi size={10} />
                  实时
                </span>
              )}
            </div>
          </div>

          {/* Step timeline */}
          <div className="mb-2">
            <h2 className="text-xs text-white/30 uppercase tracking-wide mb-3">执行步骤</h2>
            <StepTimeline
              steps={steps || []}
              selectedStepId={selectedStep?.id}
              onStepClick={handleStepClick}
              taskTerminal={isTerminal}
            />
          </div>
        </div>
      </div>

      {/* Right panel: Chat or Workspace */}
      {isConversationPhase && !(selectedStep && selectedStep.status === "COMPLETED" && userSelectedRef.current) ? (
        <div className="flex-1 overflow-hidden">
          <ChatPanel
            messages={messages}
            onSend={handleChatSend}
            onConfirm={handleChatConfirm}
            onModify={handleChatModify}
            onCancel={handleChatCancel}
            isLoading={chatLoading}
            confirmationData={confirmationData}
            isConfirming={isConfirming}
            risks={latestRisks}
            planReviewData={planReviewData}
            onApprovePlan={handleApprovePlan}
            isPlanApproving={isPlanApproving}
            latestOptions={latestOptions}
            recommendation={latestRecommendation}
            analyzeThinking={analyzeThinking}
            isAnalyzing={isAnalyzing}
          />
        </div>
      ) : (
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
