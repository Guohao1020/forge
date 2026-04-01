"use client";

import { Button } from "@/components/ui/button";
import { CheckCircle, Edit3, X } from "lucide-react";

interface ConfirmationCardProps {
  summary: string;
  taskTitle: string;
  affectedModules?: string[];
  riskLevel?: string;
  estimatedComplexity?: string;
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
  onConfirm,
  onModify,
  onCancel,
  isLoading = false,
}: ConfirmationCardProps) {
  return (
    <div className="border border-white/10 rounded-xl bg-white/[0.03] p-5 my-4">
      <div className="flex items-center gap-2 mb-3">
        <CheckCircle className="h-5 w-5 text-green-400" />
        <h3 className="text-sm font-semibold text-white">needs confirmed</h3>
      </div>

      <h4 className="text-base font-medium text-white mb-2">{taskTitle}</h4>
      <p className="text-sm text-white/60 mb-4 whitespace-pre-wrap">{summary}</p>

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
            risk: {riskLevel}
          </span>
        )}
        {estimatedComplexity && (
          <span className="px-2 py-0.5 rounded text-xs bg-white/5 text-white/50 border border-white/10">
            complexity: {estimatedComplexity}
          </span>
        )}
      </div>

      <div className="flex gap-2">
        <Button
          onClick={onConfirm}
          disabled={isLoading}
          className="bg-[#8B5CF6] hover:bg-[#7C3AED] text-white"
        >
          <CheckCircle className="h-4 w-4 mr-1.5" />
          {isLoading ? "executing..." : "confirm & execute"}
        </Button>
        <Button variant="ghost" onClick={onModify} className="text-white/50">
          <Edit3 className="h-4 w-4 mr-1.5" />
          modify
        </Button>
        <Button variant="ghost" onClick={onCancel} className="text-red-400/60 hover:text-red-400">
          <X className="h-4 w-4 mr-1.5" />
          cancel
        </Button>
      </div>
    </div>
  );
}
