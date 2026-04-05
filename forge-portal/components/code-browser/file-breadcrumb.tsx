"use client";

import { ChevronRight } from "lucide-react";

interface FileBreadcrumbProps {
  /** Current file path, e.g. "src/main/java/Calculator.java" */
  path: string;
  /** Repository / project name shown as root */
  repoName?: string;
  /** Called when a segment is clicked, receives the path up to that segment */
  onNavigate: (path: string) => void;
}

export function FileBreadcrumb({
  path,
  repoName = "repo",
  onNavigate,
}: FileBreadcrumbProps) {
  const segments = path ? path.split("/") : [];

  return (
    <div className="flex items-center gap-1 px-4 py-2 text-sm border-b border-border bg-muted/20 overflow-x-auto">
      <button
        onClick={() => onNavigate("")}
        className="text-muted-foreground hover:text-accent transition-colors shrink-0"
      >
        {repoName}
      </button>
      {segments.map((segment, i) => {
        const partialPath = segments.slice(0, i + 1).join("/");
        const isLast = i === segments.length - 1;
        return (
          <span key={partialPath} className="flex items-center gap-1 shrink-0">
            <ChevronRight className="h-3 w-3 text-muted-foreground/40" />
            {isLast ? (
              <span className="text-foreground/80 font-medium">{segment}</span>
            ) : (
              <button
                onClick={() => onNavigate(partialPath)}
                className="text-muted-foreground hover:text-accent transition-colors"
              >
                {segment}
              </button>
            )}
          </span>
        );
      })}
    </div>
  );
}
