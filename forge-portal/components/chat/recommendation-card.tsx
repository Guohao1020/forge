"use client";

/**
 * RecommendationCard — Displays AI-generated option recommendations.
 *
 * When AI identifies multiple valid approaches, it presents structured
 * recommendations with pros/cons/risk for each option. The AI-recommended
 * option is highlighted with a purple border.
 *
 * Used in: requirement analysis (architecture choices), task planning
 * (decomposition strategies), deployment (strategy choices).
 */

import { useState } from "react";
import { Check, ChevronDown, ChevronUp, Sparkles } from "lucide-react";

interface RecommendationOption {
  id: string;
  title: string;
  pros: string[];
  cons: string[];
  risk: "LOW" | "MEDIUM" | "HIGH";
  recommended: boolean;
  reason: string;
}

interface RecommendationCardProps {
  options: RecommendationOption[];
  aiRecommendation: string; // ID of the recommended option
  contextFactors?: string[];
  onSelect: (optionId: string) => void;
  disabled?: boolean;
}

const riskColors = {
  LOW: "bg-green-500/10 text-green-400 border-green-500/20",
  MEDIUM: "bg-amber-500/10 text-amber-400 border-amber-500/20",
  HIGH: "bg-red-500/10 text-red-400 border-red-500/20",
};

export function RecommendationCard({
  options,
  aiRecommendation,
  contextFactors,
  onSelect,
  disabled = false,
}: RecommendationCardProps) {
  const [selectedId, setSelectedId] = useState<string | null>(null);
  const [showContext, setShowContext] = useState(false);

  const handleSelect = (id: string) => {
    if (disabled) return;
    setSelectedId(id);
  };

  const handleConfirm = () => {
    if (selectedId && !disabled) {
      const option = options.find((o) => o.id === selectedId);
      if (option) {
        onSelect(`我选择 ${option.title}`);
      }
    }
  };

  return (
    <div className="border border-border rounded-xl bg-muted/30 p-4 my-3">
      <div className="flex items-center gap-2 mb-3">
        <Sparkles className="h-4 w-4 text-accent" />
        <span className="text-xs font-medium text-muted-foreground">AI 方案推荐</span>
      </div>

      {/* Option cards */}
      <div className="grid gap-2" style={{ gridTemplateColumns: `repeat(${Math.min(options.length, 3)}, 1fr)` }}>
        {options.map((opt) => {
          const isRecommended = opt.id === aiRecommendation;
          const isSelected = opt.id === selectedId;

          return (
            <button
              key={opt.id}
              onClick={() => handleSelect(opt.id)}
              disabled={disabled}
              className={`
                text-left p-3 rounded-lg border transition-all relative
                ${isSelected
                  ? "border-accent bg-accent/10"
                  : isRecommended
                  ? "border-accent/40 bg-muted/30"
                  : "border-border bg-muted/20 hover:border-border/80"
                }
                ${disabled ? "opacity-50 cursor-not-allowed" : "cursor-pointer"}
              `}
            >
              {/* AI Recommended badge */}
              {isRecommended && (
                <div className="flex items-center gap-1 mb-2">
                  <Sparkles className="h-3 w-3 text-accent" />
                  <span className="text-[10px] font-medium text-accent">AI 推荐</span>
                </div>
              )}

              {/* Selected check */}
              {isSelected && (
                <div className="absolute top-2 right-2">
                  <Check className="h-4 w-4 text-accent" />
                </div>
              )}

              {/* Title */}
              <p className="text-sm font-medium text-foreground mb-2 pr-5">{opt.title}</p>

              {/* Pros */}
              {opt.pros.length > 0 && (
                <div className="space-y-0.5 mb-2">
                  {opt.pros.map((pro, i) => (
                    <p key={i} className="text-[11px] text-green-400/80 flex items-start gap-1">
                      <span className="text-green-400 mt-0.5 shrink-0">+</span>
                      <span>{pro}</span>
                    </p>
                  ))}
                </div>
              )}

              {/* Cons */}
              {opt.cons.length > 0 && (
                <div className="space-y-0.5 mb-2">
                  {opt.cons.map((con, i) => (
                    <p key={i} className="text-[11px] text-red-400/70 flex items-start gap-1">
                      <span className="text-red-400 mt-0.5 shrink-0">-</span>
                      <span>{con}</span>
                    </p>
                  ))}
                </div>
              )}

              {/* Risk badge */}
              <span className={`inline-block px-1.5 py-0.5 rounded text-[10px] border ${riskColors[opt.risk]}`}>
                风险: {opt.risk}
              </span>

              {/* Recommendation reason */}
              {isRecommended && opt.reason && (
                <p className="text-[10px] text-accent/60 mt-2 italic">
                  {opt.reason}
                </p>
              )}
            </button>
          );
        })}
      </div>

      {/* Context factors (expandable) */}
      {contextFactors && contextFactors.length > 0 && (
        <div className="mt-3">
          <button
            onClick={() => setShowContext(!showContext)}
            className="flex items-center gap-1 text-[10px] text-muted-foreground/60 hover:text-muted-foreground transition-colors"
          >
            {showContext ? <ChevronUp size={12} /> : <ChevronDown size={12} />}
            AI 推荐依据 ({contextFactors.length})
          </button>
          {showContext && (
            <div className="mt-1.5 pl-4 space-y-0.5">
              {contextFactors.map((f, i) => (
                <p key={i} className="text-[10px] text-muted-foreground/50">{f}</p>
              ))}
            </div>
          )}
        </div>
      )}

      {/* Confirm button */}
      {selectedId && (
        <div className="mt-3 flex justify-end">
          <button
            onClick={handleConfirm}
            disabled={disabled}
            className="px-4 py-1.5 bg-accent text-accent-foreground rounded-lg text-xs hover:bg-accent/90 transition-colors disabled:opacity-50"
          >
            按此方案继续
          </button>
        </div>
      )}
    </div>
  );
}
