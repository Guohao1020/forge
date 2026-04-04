"use client";

import { useState, useRef, useEffect } from "react";
import { Send, Loader2 } from "lucide-react";
import { Button } from "@/components/ui/button";
import { MessageBubble } from "./message-bubble";
import { OptionButtons } from "./option-buttons";
import { ConfirmationCard } from "./confirmation-card";
import { PlanReviewCard } from "./plan-review-card";
import { RiskAlert, Risk } from "./risk-alert";
import { Conversation, PlanConfirmResponse } from "@/lib/conversation";

interface ChatPanelProps {
  messages: Conversation[];
  onSend: (content: string) => Promise<void>;
  onConfirm: () => void;
  onModify: () => void;
  onCancel: () => void;
  isLoading: boolean;
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
  risks?: Risk[];
  planReviewData?: PlanConfirmResponse["planData"] | null;
  onApprovePlan?: () => void;
  isPlanApproving?: boolean;
  /** Options from the latest AI clarify response for clickable selection */
  latestOptions?: string[];
}

export function ChatPanel({
  messages,
  onSend,
  onConfirm,
  onModify,
  onCancel,
  isLoading,
  confirmationData,
  isConfirming = false,
  risks = [],
  planReviewData,
  onApprovePlan,
  isPlanApproving = false,
  latestOptions = [],
}: ChatPanelProps) {
  const [input, setInput] = useState("");
  const messagesEndRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    messagesEndRef.current?.scrollIntoView({ behavior: "smooth" });
  }, [messages, confirmationData, planReviewData]);

  const handleSubmit = async () => {
    const content = input.trim();
    if (!content || isLoading) return;
    setInput("");
    await onSend(content);
  };

  const handleKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === "Enter" && !e.shiftKey) {
      e.preventDefault();
      handleSubmit();
    }
  };

  const hasActiveCard = !!confirmationData || !!planReviewData;

  return (
    <div className="flex flex-col h-full">
      {/* Messages */}
      <div className="flex-1 overflow-y-auto p-4 space-y-1">
        {messages.length === 0 && !isLoading && (
          <div className="flex items-center justify-center h-full text-white/20 text-sm">
            描述你的需求，AI 会进行分析和拆解
          </div>
        )}
        {messages.map((msg) => (
          <MessageBubble
            key={msg.id}
            role={msg.role}
            content={msg.content}
            createdAt={msg.createdAt}
          />
        ))}
        {/* Clickable option buttons from AI clarify response */}
        {latestOptions.length > 0 && !isLoading && !confirmationData && !planReviewData && (
          <OptionButtons
            options={latestOptions}
            onSelect={(opt) => onSend(opt)}
            disabled={isLoading}
          />
        )}
        {risks.length > 0 && !confirmationData && !planReviewData && (
          <RiskAlert risks={risks} />
        )}
        {confirmationData && (
          <ConfirmationCard
            {...confirmationData}
            onConfirm={onConfirm}
            onModify={onModify}
            onCancel={onCancel}
            isLoading={isConfirming}
          />
        )}
        {planReviewData && onApprovePlan && (
          <PlanReviewCard
            planData={planReviewData}
            onApprove={onApprovePlan}
            onRequestChanges={onModify}
            onCancel={onCancel}
            isLoading={isPlanApproving}
          />
        )}
        {isLoading && (
          <div className="flex justify-start mb-4">
            <div className="bg-white/5 rounded-2xl rounded-bl-md px-4 py-3 max-w-[80%]">
              <div className="flex items-center gap-2 text-white/50 text-sm">
                <Loader2 className="h-4 w-4 animate-spin text-[#8B5CF6]" />
                <span className="animate-pulse">AI 正在分析你的需求...</span>
              </div>
              <div className="mt-1.5 text-white/30 text-xs">
                通常需要 15-45 秒，取决于需求复杂度
              </div>
            </div>
          </div>
        )}
        <div ref={messagesEndRef} />
      </div>

      {/* Input */}
      <div className="border-t border-white/10 p-4">
        <div className="flex gap-2">
          <textarea
            value={input}
            onChange={(e) => setInput(e.target.value)}
            onKeyDown={handleKeyDown}
            placeholder="描述你的需求...（Shift+Enter 换行）"
            rows={2}
            className="flex-1 bg-white/5 border border-white/10 rounded-lg px-4 py-2.5 text-sm text-white placeholder:text-white/30 resize-none focus:outline-none focus:border-[#8B5CF6]/50"
            disabled={isLoading || hasActiveCard}
          />
          <Button
            onClick={handleSubmit}
            disabled={!input.trim() || isLoading || hasActiveCard}
            className="bg-[#8B5CF6] hover:bg-[#7C3AED] text-white self-end h-10 w-10 p-0"
          >
            <Send className="h-4 w-4" />
          </Button>
        </div>
      </div>
    </div>
  );
}
