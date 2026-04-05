"use client";

import { Code2 } from "lucide-react";

interface TechStackBadgeProps {
  techStack: {
    languages?: Record<string, number>;
    frameworks?: string[];
  };
}

export function TechStackBadge({ techStack }: TechStackBadgeProps) {
  if (!techStack) return null;

  const { languages, frameworks } = techStack;
  const hasLanguages = languages && Object.keys(languages).length > 0;
  const hasFrameworks = frameworks && frameworks.length > 0;

  if (!hasLanguages && !hasFrameworks) return null;

  return (
    <div className="flex items-center gap-1.5 flex-wrap px-4 py-2 border-b border-border/50">
      <Code2 className="h-3.5 w-3.5 text-muted-foreground/60 shrink-0" />
      <span className="text-[10px] text-muted-foreground/60 uppercase tracking-wide mr-1">技术栈</span>
      {hasLanguages &&
        Object.entries(languages)
          .sort(([, a], [, b]) => b - a)
          .map(([lang, pct]) => (
            <span
              key={lang}
              className="px-1.5 py-0.5 rounded text-[10px] bg-accent/10 text-accent/80 border border-accent/15"
            >
              {lang} {pct}%
            </span>
          ))}
      {hasFrameworks &&
        frameworks.map((fw) => (
          <span
            key={fw}
            className="px-1.5 py-0.5 rounded text-[10px] bg-muted/50 text-muted-foreground border border-border"
          >
            {fw}
          </span>
        ))}
    </div>
  );
}
