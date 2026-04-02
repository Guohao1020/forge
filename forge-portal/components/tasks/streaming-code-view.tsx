"use client";

import { useEffect, useRef, useMemo } from "react";
import { Loader2, Check } from "lucide-react";

interface StreamingCodeViewProps {
  tokens: string;
  isStreaming: boolean;
  language?: string;
}

export function StreamingCodeView({ tokens, isStreaming }: StreamingCodeViewProps) {
  const codeEndRef = useRef<HTMLDivElement>(null);

  // Auto-scroll to bottom as new tokens arrive
  useEffect(() => {
    codeEndRef.current?.scrollIntoView({ behavior: "smooth" });
  }, [tokens]);

  // Calculate line numbers
  const lineNumbers = useMemo(() => {
    if (!tokens) return [1];
    const count = (tokens.match(/\n/g) || []).length + 1;
    return Array.from({ length: count }, (_, i) => i + 1);
  }, [tokens]);

  return (
    <div className="rounded-xl border border-white/10 bg-[#0A0A12] overflow-hidden">
      {/* Header */}
      <div className="flex items-center gap-2 px-4 py-3 border-b border-white/10 bg-white/[0.02]">
        {isStreaming ? (
          <>
            <Loader2 className="h-4 w-4 animate-spin text-primary" />
            <span className="text-sm text-white/70">AI 正在生成代码...</span>
          </>
        ) : (
          <>
            <Check className="h-4 w-4 text-emerald-400" />
            <span className="text-sm text-white/70">代码生成完成</span>
          </>
        )}
      </div>

      {/* Code area */}
      <div className="relative min-h-[200px] max-h-[500px] overflow-y-auto">
        <div className="flex">
          {/* Line numbers */}
          <div className="shrink-0 select-none border-r border-white/5 bg-white/[0.01] px-3 py-4 text-right">
            {lineNumbers.map((n) => (
              <div key={n} className="font-mono text-xs leading-5 text-white/20">
                {n}
              </div>
            ))}
          </div>

          {/* Code content */}
          <pre className="flex-1 p-4 overflow-x-auto">
            <code className="font-mono text-sm leading-5 text-emerald-400 whitespace-pre">
              {tokens}
              {isStreaming && (
                <span className="animate-blink text-emerald-300">|</span>
              )}
            </code>
            <div ref={codeEndRef} />
          </pre>
        </div>
      </div>
    </div>
  );
}
