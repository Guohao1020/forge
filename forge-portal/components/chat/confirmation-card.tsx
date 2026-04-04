"use client";

import { Button } from "@/components/ui/button";
import { CheckCircle, Edit3, X, Info } from "lucide-react";
import { RiskAlert, Risk } from "./risk-alert";

interface ConfirmationCardProps {
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
  onConfirm: () => void;
  onModify: () => void;
  onCancel: () => void;
  isLoading?: boolean;
}

const riskColors: Record<string, string> = {
  LOW: "bg-green-500/10 text-green-400 border-green-500/20",
  MEDIUM: "bg-yellow-500/10 text-yellow-400 border-yellow-500/20",
  HIGH: "bg-red-500/10 text-red-400 border-red-500/20",
};

export function ConfirmationCard({
  summary,
  taskTitle,
  affectedModules = [],
  riskLevel = "MEDIUM",
  estimatedComplexity,
  risks,
  nonFunctional,
  functionalRequirements,
  acceptanceCriteria,
  outOfScope,
  onConfirm,
  onModify,
  onCancel,
  isLoading = false,
}: ConfirmationCardProps) {
  return (
    <div className="border border-[#8B5CF6]/20 rounded-xl bg-[#8B5CF6]/[0.03] p-5 my-4">
      <div className="flex items-center gap-2 mb-3">
        <CheckCircle className="h-5 w-5 text-green-400" />
        <h3 className="text-sm font-semibold text-white">需求确认</h3>
      </div>

      <h4 className="text-base font-medium text-white mb-2">{taskTitle}</h4>
      <p className="text-sm text-white/60 mb-4 whitespace-pre-wrap">{summary}</p>

      {/* Functional requirements */}
      {functionalRequirements && functionalRequirements.length > 0 && (
        <div className="border border-white/5 rounded-lg bg-white/[0.02] p-3 mb-3">
          <span className="text-xs font-medium text-white/50 block mb-2">功能需求</span>
          <ol className="space-y-1 list-decimal list-inside">
            {functionalRequirements.map((req, idx) => (
              <li key={idx} className="text-xs text-white/60">{req}</li>
            ))}
          </ol>
        </div>
      )}

      {/* Modules & badges */}
      {affectedModules.length > 0 && (
        <div className="flex flex-wrap gap-1.5 mb-3">
          {affectedModules.map((m) => (
            <span
              key={m}
              className="px-2 py-0.5 rounded text-xs bg-[#8B5CF6]/10 text-[#8B5CF6] border border-[#8B5CF6]/20"
            >
              {m}
            </span>
          ))}
        </div>
      )}

      <div className="flex items-center gap-2 mb-4">
        {riskLevel && (
          <span className={`px-2 py-0.5 rounded text-xs border ${riskColors[riskLevel] || riskColors.MEDIUM}`}>
            风险：{riskLevel}
          </span>
        )}
        {estimatedComplexity && (
          <span className="px-2 py-0.5 rounded text-xs bg-white/5 text-white/50 border border-white/10">
            复杂度：{estimatedComplexity}
          </span>
        )}
      </div>

      {/* Acceptance criteria */}
      {acceptanceCriteria && acceptanceCriteria.length > 0 && (
        <div className="border border-emerald-500/10 rounded-lg bg-emerald-500/[0.03] p-3 mb-3">
          <span className="text-xs font-medium text-emerald-400/70 block mb-2">验收标准</span>
          <ol className="space-y-1 list-decimal list-inside">
            {acceptanceCriteria.map((c, idx) => (
              <li key={idx} className="text-xs text-white/60">{c}</li>
            ))}
          </ol>
        </div>
      )}

      {/* Out of scope */}
      {outOfScope && outOfScope.length > 0 && (
        <div className="border border-white/5 rounded-lg bg-white/[0.02] p-3 mb-3">
          <span className="text-xs font-medium text-white/40 block mb-2">不在范围内</span>
          <ul className="space-y-1">
            {outOfScope.map((item, idx) => (
              <li key={idx} className="text-xs text-white/40 pl-4 relative before:content-['—'] before:absolute before:left-0 before:text-white/20">
                {item}
              </li>
            ))}
          </ul>
        </div>
      )}

      {risks && risks.length > 0 && <RiskAlert risks={risks} />}

      {nonFunctional && nonFunctional.length > 0 && (
        <div className="border border-white/5 rounded-lg bg-white/[0.02] p-2.5 mb-4">
          <div className="flex items-center gap-1.5 mb-1.5">
            <Info className="h-3.5 w-3.5 text-[#8B5CF6]/60" />
            <span className="text-xs font-medium text-white/50">非功能需求</span>
          </div>
          <ul className="space-y-1">
            {nonFunctional.map((item, idx) => (
              <li key={idx} className="text-xs text-white/40 pl-5 relative before:content-[''] before:absolute before:left-2 before:top-[7px] before:w-1 before:h-1 before:rounded-full before:bg-white/20">
                {item}
              </li>
            ))}
          </ul>
        </div>
      )}

      <div className="flex gap-2">
        <Button
          onClick={onConfirm}
          disabled={isLoading}
          className="bg-[#8B5CF6] hover:bg-[#7C3AED] text-white"
        >
          <CheckCircle className="h-4 w-4 mr-1.5" />
          {isLoading ? "正在生成方案..." : "确认需求"}
        </Button>
        <Button variant="ghost" onClick={onModify} className="text-white/50">
          <Edit3 className="h-4 w-4 mr-1.5" />
          继续完善
        </Button>
        <Button variant="ghost" onClick={onCancel} className="text-red-400/60 hover:text-red-400">
          <X className="h-4 w-4 mr-1.5" />
          取消
        </Button>
      </div>
    </div>
  );
}
