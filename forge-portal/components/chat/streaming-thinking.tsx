"use client";

/**
 * StreamingThinking — Shows AI thinking process in real-time.
 *
 * Displays character-by-character text as the AI analyzes requirements.
 * Collapses into a summary when analysis is complete.
 *
 * Usage:
 *   <StreamingThinking text={streamingText} isComplete={false} />
 *   After completion: isComplete=true shows collapsed view
 */

import { useState } from "react";
import { ChevronDown, ChevronRight, Brain } from "lucide-react";

interface StreamingThinkingProps {
  text: string;
  isComplete: boolean;
  onCollapse?: () => void;
}

export function StreamingThinking({ text, isComplete, onCollapse }: StreamingThinkingProps) {
  const [expanded, setExpanded] = useState(!isComplete);

  if (!text) return null;

  if (isComplete && !expanded) {
    return (
      <button
        onClick={() => setExpanded(true)}
        className="flex items-center gap-1.5 text-xs text-muted-foreground/40 hover:text-muted-foreground/60 transition-colors mb-2"
      >
        <ChevronRight size={12} />
        <Brain size={12} />
        <span>AI 思考过程（点击展开）</span>
      </button>
    );
  }

  return (
    <div className="mb-3">
      {isComplete && (
        <button
          onClick={() => { setExpanded(false); onCollapse?.(); }}
          className="flex items-center gap-1.5 text-xs text-muted-foreground/40 hover:text-muted-foreground/60 transition-colors mb-1"
        >
          <ChevronDown size={12} />
          <Brain size={12} />
          <span>AI 思考过程</span>
        </button>
      )}
      <div className={`bg-muted/20 border border-border/50 rounded-lg px-3 py-2 ${
        isComplete ? "opacity-50" : ""
      }`}>
        {!isComplete && (
          <div className="flex items-center gap-1.5 mb-1.5">
            <div className="w-1.5 h-1.5 rounded-full bg-purple-400 animate-pulse" />
            <span className="text-[10px] text-purple-400/60 uppercase tracking-wider">思考中</span>
          </div>
        )}
        <p className="text-xs text-muted-foreground/60 font-mono whitespace-pre-wrap leading-relaxed">
          {text}
          {!isComplete && <span className="animate-pulse">▊</span>}
        </p>
      </div>
    </div>
  );
}
