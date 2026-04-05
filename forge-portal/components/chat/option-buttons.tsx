"use client";

import { useState } from "react";
import { Send, Check } from "lucide-react";
import { Button } from "@/components/ui/button";

export interface OptionDetail {
  label: string;
  reason: string;
}

interface OptionButtonsProps {
  options: string[];
  optionDetails?: OptionDetail[];
  onSelect: (selectedOptions: string) => void;
  disabled?: boolean;
}

/**
 * Multi-select option buttons for AI clarification questions.
 * User selects one or more options → clicks confirm → sends combined selection.
 * Supports both single-click quick-select and multi-select + confirm.
 */
export function OptionButtons({ options, optionDetails, onSelect, disabled = false }: OptionButtonsProps) {
  const [selected, setSelected] = useState<Set<number>>(new Set());
  const [sent, setSent] = useState(false);
  const labels = "ABCDEFGH";

  const toggleOption = (idx: number) => {
    if (disabled || sent) return;
    setSelected((prev) => {
      const next = new Set(prev);
      if (next.has(idx)) {
        next.delete(idx);
      } else {
        next.add(idx);
      }
      return next;
    });
  };

  const handleConfirm = () => {
    if (selected.size === 0 || disabled || sent) return;
    setSent(true);
    const selectedTexts = Array.from(selected)
      .sort()
      .map((i) => options[i])
      .join("；");
    onSelect(selectedTexts);
  };

  return (
    <div className="my-3 ml-1">
      <div className="flex flex-col gap-2">
        {options.map((opt, i) => {
          const label = i < labels.length ? labels[i] : `${i + 1}`;
          const isSelected = selected.has(i);

          return (
            <button
              key={i}
              onClick={() => toggleOption(i)}
              disabled={disabled || sent}
              className={`
                group flex items-start gap-3 text-left px-4 py-3
                rounded-xl border transition-all duration-200
                ${isSelected
                  ? "bg-accent/15 border-accent/40 text-foreground"
                  : sent
                    ? "bg-muted/20 border-border/50 text-muted-foreground/40 cursor-not-allowed"
                    : "bg-muted/30 border-border text-muted-foreground hover:bg-accent/10 hover:border-accent/30 hover:text-foreground cursor-pointer"
                }
              `}
            >
              <span className={`
                shrink-0 w-6 h-6 rounded-md flex items-center justify-center text-xs font-semibold transition-all
                ${isSelected
                  ? "bg-accent text-accent-foreground"
                  : "bg-muted text-muted-foreground group-hover:bg-accent/30 group-hover:text-foreground/80"
                }
              `}>
                {isSelected ? <Check className="h-3.5 w-3.5" /> : label}
              </span>
              <div className="flex flex-col gap-0.5">
                <span className="text-sm leading-relaxed">{opt}</span>
                {optionDetails?.[i]?.reason && (
                  <span className="text-xs text-muted-foreground/60 leading-relaxed">{optionDetails[i].reason}</span>
                )}
              </div>
            </button>
          );
        })}
      </div>

      {/* Confirm button — visible when any option is selected */}
      {selected.size > 0 && !sent && (
        <div className="mt-3 flex items-center gap-2">
          <Button
            onClick={handleConfirm}
            disabled={disabled}
            size="sm"
            className="bg-accent hover:bg-accent/90 text-accent-foreground"
          >
            <Send className="h-3.5 w-3.5 mr-1.5" />
            确认选择{selected.size > 1 ? `（${selected.size}项）` : ""}
          </Button>
          <span className="text-xs text-muted-foreground/60">可多选</span>
        </div>
      )}
    </div>
  );
}
