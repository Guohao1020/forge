"use client";

import { useState } from "react";
import { AlertTriangle, ChevronDown, ChevronRight, Shield } from "lucide-react";

export interface Risk {
  level: "HIGH" | "MEDIUM" | "LOW";
  description: string;
  mitigation: string;
}

interface RiskAlertProps {
  risks: Risk[];
}

const levelConfig: Record<string, { bg: string; text: string; border: string; label: string }> = {
  HIGH: { bg: "bg-red-500/10", text: "text-red-400", border: "border-red-500/20", label: "HIGH" },
  MEDIUM: { bg: "bg-yellow-500/10", text: "text-yellow-400", border: "border-yellow-500/20", label: "MEDIUM" },
  LOW: { bg: "bg-green-500/10", text: "text-green-400", border: "border-green-500/20", label: "LOW" },
};

export function RiskAlert({ risks }: RiskAlertProps) {
  const [expanded, setExpanded] = useState(false);

  if (!risks || risks.length === 0) return null;

  const highCount = risks.filter((r) => r.level === "HIGH").length;
  const hasHigh = highCount > 0;

  return (
    <div className="border border-white/10 rounded-lg bg-white/[0.03] my-2 overflow-hidden">
      <button
        onClick={() => setExpanded(!expanded)}
        className="w-full flex items-center gap-2 px-3 py-2 text-sm hover:bg-white/5 transition-colors"
      >
        <AlertTriangle className={`h-4 w-4 ${hasHigh ? "text-red-400" : "text-yellow-400"}`} />
        <span className="text-white/80 font-medium">风险识别</span>
        <span className="px-1.5 py-0.5 rounded text-[10px] font-medium bg-white/10 text-white/60">
          {risks.length}
        </span>
        <span className="flex-1" />
        {expanded ? (
          <ChevronDown className="h-3.5 w-3.5 text-white/30" />
        ) : (
          <ChevronRight className="h-3.5 w-3.5 text-white/30" />
        )}
      </button>

      {expanded && (
        <div className="px-3 pb-3 space-y-2">
          {risks.map((risk, idx) => {
            const config = levelConfig[risk.level] || levelConfig.MEDIUM;
            return (
              <div
                key={idx}
                className="rounded-md border border-white/5 bg-white/[0.02] p-2.5"
              >
                <div className="flex items-start gap-2">
                  <span
                    className={`shrink-0 px-1.5 py-0.5 rounded text-[10px] font-semibold border ${config.bg} ${config.text} ${config.border}`}
                  >
                    {config.label}
                  </span>
                  <p className="text-xs text-white/70 leading-relaxed">
                    {risk.description}
                  </p>
                </div>
                {risk.mitigation && (
                  <div className="flex items-start gap-1.5 mt-1.5 ml-1">
                    <Shield className="h-3 w-3 text-[#8B5CF6] shrink-0 mt-0.5" />
                    <p className="text-xs text-white/50 leading-relaxed">
                      <span className="text-[#8B5CF6]/80 font-medium">规避方案: </span>
                      {risk.mitigation}
                    </p>
                  </div>
                )}
              </div>
            );
          })}
        </div>
      )}
    </div>
  );
}
