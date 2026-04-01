"use client";

import { MarkdownPreview } from "@/components/markdown-preview";

interface MessageBubbleProps {
  role: string;
  content: string;
  createdAt?: string;
}

export function MessageBubble({ role, content, createdAt }: MessageBubbleProps) {
  if (role === "system") {
    return (
      <div className="flex justify-center py-2">
        <span className="text-xs text-white/30">{content}</span>
      </div>
    );
  }

  const isUser = role === "user";

  return (
    <div className={`flex ${isUser ? "justify-end" : "justify-start"} mb-4`}>
      <div
        className={`max-w-[75%] rounded-2xl px-4 py-3 ${
          isUser
            ? "bg-[#8B5CF6]/10 text-white rounded-br-md"
            : "bg-white/5 text-white/80 rounded-bl-md"
        }`}
      >
        {isUser ? (
          <p className="text-sm whitespace-pre-wrap">{content}</p>
        ) : (
          <MarkdownPreview content={content} className="text-sm" />
        )}
        {createdAt && (
          <p className="text-xs text-white/20 mt-2">
            {new Date(createdAt).toLocaleTimeString()}
          </p>
        )}
      </div>
    </div>
  );
}
