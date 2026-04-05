"use client";

import { useMemo } from "react";
import { MarkdownPreview } from "@/components/markdown-preview";

interface MessageBubbleProps {
  role: string;
  content: string;
  createdAt?: string;
  metadata?: Record<string, unknown>;
}

/**
 * Minimal fallback for legacy messages that stored raw JSON as content.
 * New messages are pre-formatted by Python's format_human_response() and
 * should pass through unchanged. Only raw JSON needs conversion.
 */
function formatAIContent(content: string): string {
  const trimmed = content.trim();
  if (!trimmed.startsWith("{")) return content;

  try {
    const data = JSON.parse(trimmed);
    if (!data || typeof data !== "object") return content;
    // Extract readable text from raw JSON fallback
    if (data.content && typeof data.content === "string") return data.content;
    if (data.summary && typeof data.summary === "string") return data.summary;
    if (data.understanding && typeof data.understanding === "string") return data.understanding;
    return content;
  } catch {
    return content;
  }
}

export function MessageBubble({ role, content, createdAt, metadata }: MessageBubbleProps) {
  const displayContent = useMemo(
    () => (role === "assistant" ? formatAIContent(content) : content),
    [role, content]
  );

  if (role === "system") {
    return (
      <div className="flex justify-center py-2">
        <span className="text-xs text-muted-foreground/60">{content}</span>
      </div>
    );
  }

  const isUser = role === "user";

  return (
    <div className={`flex ${isUser ? "justify-end" : "justify-start"} mb-4`}>
      <div
        className={`max-w-[75%] rounded-2xl px-4 py-3 ${
          isUser
            ? "bg-accent/10 text-foreground rounded-br-md"
            : "bg-muted/50 text-foreground/80 rounded-bl-md"
        }`}
      >
        {isUser ? (
          <p className="text-sm whitespace-pre-wrap">{displayContent}</p>
        ) : (
          <MarkdownPreview content={displayContent} className="text-sm" />
        )}
        <div className="flex items-center gap-2 mt-2">
          {!isUser && metadata && typeof metadata.model === "string" && (
            <span className="text-[10px] text-muted-foreground/30 font-mono">
              {metadata.model.replace(/^(claude|gpt|qwen)/i, (m) => m.charAt(0).toUpperCase() + m.slice(1))}
            </span>
          )}
          {createdAt && (
            <span className="text-[10px] text-muted-foreground/30">
              {new Date(createdAt).toLocaleTimeString()}
            </span>
          )}
        </div>
      </div>
    </div>
  );
}
