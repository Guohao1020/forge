"use client";

import { useEffect, useState } from "react";
import { Keyboard } from "lucide-react";

const SHORTCUTS = [
  { keys: ["⌘", "K"], description: "搜索项目和任务" },
  { keys: ["Escape"], description: "关闭搜索/对话框" },
  { keys: ["?"], description: "显示快捷键帮助" },
];

export function KeyboardShortcutsDialog() {
  const [open, setOpen] = useState(false);

  useEffect(() => {
    const handleKeyDown = (e: KeyboardEvent) => {
      // ? key (without modifiers, not in input)
      if (e.key === "?" && !e.metaKey && !e.ctrlKey && !e.altKey) {
        const target = e.target as HTMLElement;
        if (target.tagName === "INPUT" || target.tagName === "TEXTAREA" || target.tagName === "SELECT") {
          return;
        }
        e.preventDefault();
        setOpen((prev) => !prev);
      }
      if (e.key === "Escape" && open) {
        setOpen(false);
      }
    };
    document.addEventListener("keydown", handleKeyDown);
    return () => document.removeEventListener("keydown", handleKeyDown);
  }, [open]);

  if (!open) return null;

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center">
      {/* Backdrop */}
      <div className="absolute inset-0 bg-black/50" onClick={() => setOpen(false)} />

      {/* Dialog */}
      <div className="relative bg-card border border-border rounded-xl shadow-2xl p-6 w-96 max-w-[90vw]">
        <div className="flex items-center gap-2 mb-4">
          <Keyboard className="h-5 w-5 text-primary" />
          <h2 className="text-lg font-semibold text-foreground">��盘快捷键</h2>
        </div>

        <div className="space-y-2">
          {SHORTCUTS.map((shortcut, i) => (
            <div key={i} className="flex items-center justify-between py-1.5">
              <span className="text-sm text-muted-foreground">{shortcut.description}</span>
              <div className="flex items-center gap-1">
                {shortcut.keys.map((key, j) => (
                  <span key={j}>
                    {j > 0 && <span className="text-muted-foreground/40 mx-0.5">+</span>}
                    <kbd className="inline-flex items-center px-2 py-0.5 text-xs bg-muted/50 border border-border rounded text-foreground font-mono">
                      {key}
                    </kbd>
                  </span>
                ))}
              </div>
            </div>
          ))}
        </div>

        <div className="mt-4 pt-3 border-t border-border text-center">
          <p className="text-[10px] text-muted-foreground">按 Escape 或 ? 关闭</p>
        </div>
      </div>
    </div>
  );
}
