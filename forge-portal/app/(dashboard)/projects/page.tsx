"use client";

import { FolderOpen, Plus, GitBranch } from "lucide-react";
import { Button } from "@/components/ui/button";

export default function ProjectsPage() {
  return (
    <div>
      <div className="flex items-center justify-between mb-6">
        <h1 className="text-2xl font-semibold tracking-tight text-[var(--foreground)]">
          项目大厅
        </h1>
        <div className="flex gap-3">
          <Button
            variant="outline"
            className="gap-2 border-[var(--border)] text-[var(--muted-foreground)]"
          >
            <GitBranch size={16} />
            接入代码平台
          </Button>
          <Button
            className="gap-2 bg-[var(--primary)] hover:bg-[#7C3AED] text-white"
            style={{ boxShadow: "0 0 20px rgba(139, 92, 246, 0.3)" }}
          >
            <Plus size={16} />
            创建新项目
          </Button>
        </div>
      </div>

      {/* Empty state */}
      <div className="flex flex-col items-center justify-center py-24 rounded-xl border border-[var(--border)] bg-[var(--card)]">
        <div className="w-16 h-16 rounded-2xl flex items-center justify-center mb-4 bg-[rgba(139,92,246,0.1)]">
          <FolderOpen size={32} className="text-[var(--primary)]" />
        </div>
        <h3 className="text-lg font-medium mb-2 text-[var(--foreground)]">
          还没有项目
        </h3>
        <p className="text-sm mb-6 text-[var(--muted-foreground)]">
          接入代码平台同步已有项目，或创建一个新项目开始
        </p>
        <div className="flex gap-3">
          <Button
            variant="outline"
            className="gap-2 border-[var(--border)] text-[var(--muted-foreground)]"
          >
            <GitBranch size={16} />
            接入代码平台
          </Button>
          <Button className="gap-2 bg-[var(--primary)] hover:bg-[#7C3AED] text-white">
            <Plus size={16} />
            创建新项目
          </Button>
        </div>
      </div>
    </div>
  );
}
