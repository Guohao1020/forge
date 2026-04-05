"use client";

/**
 * ActionPanel — Right-side panel showing the current step's actionable content.
 *
 * Phase-based rendering using explicit state machine:
 *   analyzing   → show current AI question with options
 *   confirming  → show the ConfirmationCard
 *   planning    → show plan generation progress
 *   plan_review → show PlanReviewCard
 *   error       → show error message with retry button
 *   idle        → show default state
 */

import { Loader2, Brain, CheckCircle2, ListTree, Sparkles, AlertCircle, RefreshCw } from "lucide-react";
import { Button } from "@/components/ui/button";
import { ConfirmationCard } from "./confirmation-card";
import { PlanReviewCard } from "./plan-review-card";
import { OptionButtons, OptionDetail } from "./option-buttons";
import { RecommendationCard } from "./recommendation-card";
import { RiskAlert, Risk } from "./risk-alert";
import { StreamingThinking } from "./streaming-thinking";
import { PlanData } from "@/lib/conversation";

interface RecommendationData {
  options: Array<{ id: string; title: string; pros: string[]; cons: string[]; risk: "LOW" | "MEDIUM" | "HIGH"; recommended: boolean; reason: string }>;
  aiRecommendation: string;
  contextFactors?: string[];
}

interface ActionPanelProps {
  // Current phase
  phase: "analyzing" | "confirming" | "planning" | "plan_review" | "error" | "idle";
  // Analysis phase
  isLoading?: boolean;
  latestOptions?: string[];
  latestOptionDetails?: OptionDetail[];
  recommendation?: RecommendationData | null;
  risks?: Risk[];
  analyzeThinking?: string;
  isAnalyzing?: boolean;
  currentQuestion?: string;
  currentUnderstanding?: string;
  currentPhaseLabel?: string;
  onSelectOption?: (opt: string) => void;
  // Confirmation phase
  confirmationData?: {
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
  } | null;
  isConfirming?: boolean;
  onConfirm?: () => void;
  onModify?: () => void;
  onCancel?: () => void;
  // Plan review phase
  planReviewData?: PlanData | null;
  isPlanApproving?: boolean;
  onApprovePlan?: () => void;
  // Error phase
  errorMessage?: string;
  onRetry?: () => void;
}

export function ActionPanel({
  phase,
  isLoading = false,
  latestOptions = [],
  latestOptionDetails,
  recommendation,
  risks = [],
  analyzeThinking = "",
  isAnalyzing = false,
  currentQuestion,
  currentUnderstanding,
  currentPhaseLabel,
  onSelectOption,
  confirmationData,
  isConfirming = false,
  onConfirm,
  onModify,
  onCancel,
  planReviewData,
  isPlanApproving = false,
  onApprovePlan,
  errorMessage,
  onRetry,
}: ActionPanelProps) {
  // Error phase
  if (phase === "error") {
    return (
      <div className="h-full flex items-center justify-center p-6">
        <div className="text-center space-y-4 max-w-sm">
          <div className="mx-auto w-12 h-12 rounded-xl bg-red-500/10 flex items-center justify-center">
            <AlertCircle className="h-6 w-6 text-red-400" />
          </div>
          <div>
            <h3 className="text-sm font-medium text-white mb-1">操作失败</h3>
            <p className="text-xs text-white/50 leading-relaxed">
              {errorMessage || "发生未知错误，请重试"}
            </p>
          </div>
          {onRetry && (
            <Button
              onClick={onRetry}
              variant="ghost"
              className="text-[#8B5CF6] hover:text-[#8B5CF6]/80 hover:bg-[#8B5CF6]/10"
            >
              <RefreshCw className="h-4 w-4 mr-2" />
              重试
            </Button>
          )}
          {onCancel && (
            <Button
              onClick={onCancel}
              variant="ghost"
              className="text-white/40 hover:text-white/60 text-xs"
            >
              取消任务
            </Button>
          )}
        </div>
      </div>
    );
  }

  // Confirmation phase
  if (phase === "confirming" && confirmationData && onConfirm && onModify && onCancel) {
    return (
      <div className="h-full overflow-y-auto p-4">
        <ConfirmationCard
          {...confirmationData}
          onConfirm={onConfirm}
          onModify={onModify}
          onCancel={onCancel}
          isLoading={isConfirming}
        />
      </div>
    );
  }

  // Plan review phase
  if (phase === "plan_review" && planReviewData && onApprovePlan && onModify && onCancel) {
    return (
      <div className="h-full overflow-y-auto p-4">
        <PlanReviewCard
          planData={planReviewData}
          onApprove={onApprovePlan}
          onRequestChanges={onModify}
          onCancel={onCancel}
          isLoading={isPlanApproving}
        />
      </div>
    );
  }

  // Planning phase (generating plan, show progress)
  if (phase === "planning") {
    return (
      <div className="h-full flex items-center justify-center p-6">
        <div className="text-center space-y-4 max-w-sm">
          <div className="mx-auto w-12 h-12 rounded-xl bg-[#8B5CF6]/10 flex items-center justify-center">
            <ListTree className="h-6 w-6 text-[#8B5CF6] animate-pulse" />
          </div>
          <div>
            <h3 className="text-sm font-medium text-white mb-1">正在生成方案</h3>
            <p className="text-xs text-white/40 leading-relaxed">
              AI 正在分析项目结构、读取代码文件，并生成实施方案...
            </p>
          </div>
          <div className="space-y-2 text-left">
            <ProgressStep label="分析项目结构" done />
            <ProgressStep label="读取模块依赖" done />
            <ProgressStep label="生成任务拆解" active />
            <ProgressStep label="评估风险因素" />
          </div>
        </div>
      </div>
    );
  }

  // Analysis phase — show current question and options
  if (phase === "analyzing") {
    return (
      <div className="h-full overflow-y-auto p-4 space-y-4">
        {/* AI thinking stream */}
        {analyzeThinking && (
          <StreamingThinking
            text={analyzeThinking}
            isComplete={!isAnalyzing}
          />
        )}

        {/* Loading state */}
        {isLoading && !isAnalyzing && !currentQuestion && (
          <div className="flex items-center justify-center h-full">
            <div className="text-center space-y-3">
              <div className="mx-auto w-10 h-10 rounded-xl bg-[#8B5CF6]/10 flex items-center justify-center">
                <Brain className="h-5 w-5 text-[#8B5CF6] animate-pulse" />
              </div>
              <div>
                <p className="text-sm text-white/50">AI 正在分析需求...</p>
                <p className="text-xs text-white/30 mt-1">通常需要 15-45 秒</p>
              </div>
            </div>
          </div>
        )}

        {/* Current understanding + question */}
        {currentUnderstanding && !isLoading && (
          <div className="bg-white/[0.03] border border-white/10 rounded-xl p-4 space-y-3">
            {currentPhaseLabel && (
              <div className="flex items-center gap-2">
                <Sparkles className="h-3.5 w-3.5 text-[#8B5CF6]" />
                <span className="text-[10px] text-[#8B5CF6] uppercase tracking-wider font-medium">
                  {currentPhaseLabel}
                </span>
              </div>
            )}
            <p className="text-sm text-white/70 leading-relaxed">{currentUnderstanding}</p>
            {currentQuestion && (
              <p className="text-sm font-medium text-white">{currentQuestion}</p>
            )}
          </div>
        )}

        {/* Recommendation cards */}
        {recommendation && !isLoading && onSelectOption && (
          <RecommendationCard
            options={recommendation.options}
            aiRecommendation={recommendation.aiRecommendation}
            contextFactors={recommendation.contextFactors}
            onSelect={onSelectOption}
            disabled={isLoading}
          />
        )}

        {/* Option buttons */}
        {latestOptions.length > 0 && !isLoading && !recommendation && onSelectOption && (
          <OptionButtons
            options={latestOptions}
            optionDetails={latestOptionDetails}
            onSelect={onSelectOption}
            disabled={isLoading}
          />
        )}

        {/* Risk alerts */}
        {risks.length > 0 && (
          <RiskAlert risks={risks} />
        )}

        {/* Idle state when no question and not loading */}
        {!currentQuestion && !isLoading && !analyzeThinking && latestOptions.length === 0 && (
          <div className="flex items-center justify-center h-full">
            <div className="text-center space-y-2">
              <Brain className="h-8 w-8 text-white/10 mx-auto" />
              <p className="text-sm text-white/20">等待 AI 分析结果</p>
            </div>
          </div>
        )}
      </div>
    );
  }

  // Idle/default
  return (
    <div className="h-full flex items-center justify-center">
      <div className="text-center space-y-2">
        <CheckCircle2 className="h-8 w-8 text-white/10 mx-auto" />
        <p className="text-sm text-white/20">选择左侧步骤查看详情</p>
      </div>
    </div>
  );
}

function ProgressStep({ label, done, active }: { label: string; done?: boolean; active?: boolean }) {
  return (
    <div className="flex items-center gap-2.5">
      {done ? (
        <CheckCircle2 className="h-4 w-4 text-emerald-400 shrink-0" />
      ) : active ? (
        <Loader2 className="h-4 w-4 text-[#8B5CF6] animate-spin shrink-0" />
      ) : (
        <div className="h-4 w-4 rounded-full border border-white/20 shrink-0" />
      )}
      <span className={`text-xs ${done ? "text-white/50" : active ? "text-white" : "text-white/30"}`}>
        {label}
      </span>
    </div>
  );
}
