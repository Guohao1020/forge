"use client";

/**
 * ChatPanel — Simplified message list + input.
 *
 * In the 3-column layout, this only shows conversation messages and the input box.
 * Action cards (confirmation, plan review, options) are rendered in the ActionPanel.
 */

import { useState, useRef, useEffect } from "react";
import { Send, Loader2 } from "lucide-react";
import { Button } from "@/components/ui/button";
import { MessageBubble } from "./message-bubble";
import { Conversation } from "@/lib/conversation";

interface ChatPanelProps {
  messages: Conversation[];
  onSend: (content: string) => Promise<void>;
  isLoading: boolean;
  disabled?: boolean;
  placeholder?: string;
}

export function ChatPanel({
  messages,
  onSend,
  isLoading,
  disabled = false,
  placeholder = "描述你的需求...（Shift+Enter 换行）",
}: ChatPanelProps) {
  const [input, setInput] = useState("");
  const messagesEndRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    messagesEndRef.current?.scrollIntoView({ behavior: "smooth" });
  }, [messages]);

  const handleSubmit = async () => {
    const content = input.trim();
    if (!content || isLoading || disabled) return;
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
          <div className="flex items-center justify-center h-full text-muted-foreground/40 text-sm">
            描述你的需求，AI 会进行分析和拆解
          </div>
        )}
        {messages.map((msg) => (
          <MessageBubble
            key={msg.id}
            role={msg.role}
            content={msg.content}
            createdAt={msg.createdAt}
            metadata={msg.metadata as Record<string, unknown> | undefined}
          />
        ))}
        {isLoading && (
          <div className="flex justify-start mb-4">
            <div className="bg-muted/50 rounded-2xl rounded-bl-md px-4 py-3 max-w-[80%]">
              <div className="flex items-center gap-2 text-muted-foreground text-sm">
                <Loader2 className="h-4 w-4 animate-spin text-accent" />
                <span className="animate-pulse">AI 正在分析...</span>
              </div>
            </div>
          </div>
        )}
        <div ref={messagesEndRef} />
      </div>

      {/* Input */}
      <div className="border-t border-border p-3">
        <div className="flex gap-2">
          <textarea
            value={input}
            onChange={(e) => setInput(e.target.value)}
            onKeyDown={handleKeyDown}
            placeholder={placeholder}
            rows={2}
            className="flex-1 bg-muted/50 border border-border rounded-lg px-4 py-2.5 text-sm text-foreground placeholder:text-muted-foreground/50 resize-none focus:outline-none focus:border-accent/50"
            disabled={isLoading || disabled}
          />
          <Button
            onClick={handleSubmit}
            disabled={!input.trim() || isLoading || disabled}
            className="bg-accent hover:bg-accent/90 text-accent-foreground self-end h-10 w-10 p-0"
          >
            <Send className="h-4 w-4" />
          </Button>
        </div>
      </div>
    </div>
  );
}
