"use client";

import { useState, useRef, useEffect } from "react";
import { Send, Loader2 } from "lucide-react";
import { Button } from "@/components/ui/button";
import { MessageBubble } from "./message-bubble";
import { ConfirmationCard } from "./confirmation-card";
import { Conversation } from "@/lib/conversation";

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
  } | null;
  isConfirming?: boolean;
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
}: ChatPanelProps) {
  const [input, setInput] = useState("");
  const messagesEndRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    messagesEndRef.current?.scrollIntoView({ behavior: "smooth" });
  }, [messages, confirmationData]);

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

  return (
    <div className="flex flex-col h-full">
      {/* Messages */}
      <div className="flex-1 overflow-y-auto p-4 space-y-1">
        {messages.length === 0 && !isLoading && (
          <div className="flex items-center justify-center h-full text-white/20 text-sm">
            Enter your requirement and AI will analyze and decompose it
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
        {confirmationData && (
          <ConfirmationCard
            {...confirmationData}
            onConfirm={onConfirm}
            onModify={onModify}
            onCancel={onCancel}
            isLoading={isConfirming}
          />
        )}
        {isLoading && (
          <div className="flex justify-start mb-4">
            <div className="bg-white/5 rounded-2xl rounded-bl-md px-4 py-3">
              <Loader2 className="h-4 w-4 animate-spin text-[#8B5CF6]" />
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
            placeholder="Describe your requirement... (Shift+Enter for newline)"
            rows={2}
            className="flex-1 bg-white/5 border border-white/10 rounded-lg px-4 py-2.5 text-sm text-white placeholder:text-white/30 resize-none focus:outline-none focus:border-[#8B5CF6]/50"
            disabled={isLoading || !!confirmationData}
          />
          <Button
            onClick={handleSubmit}
            disabled={!input.trim() || isLoading || !!confirmationData}
            className="bg-[#8B5CF6] hover:bg-[#7C3AED] text-white self-end h-10 w-10 p-0"
          >
            <Send className="h-4 w-4" />
          </Button>
        </div>
      </div>
    </div>
  );
}
