"use client";

import { GitCommit } from "lucide-react";

export default function ChangesPage() {
  return (
    <div>
      <h1 className="text-2xl font-semibold tracking-tight mb-6">变更</h1>
      <div className="flex flex-col items-center justify-center py-20 rounded-xl border border-border bg-card">
        <div className="w-12 h-12 rounded-xl flex items-center justify-center mb-3 bg-primary/10">
          <GitCommit className="h-6 w-6 text-primary" />
        </div>
        <h3 className="text-base font-medium mb-1">暂无变更记录</h3>
        <p className="text-sm text-muted-foreground">AI 执行任务后的代码变更将在此展示</p>
      </div>
    </div>
  );
}
