"use client";

import { FlaskConical } from "lucide-react";

export default function TestsPage() {
  return (
    <div>
      <h1 className="text-2xl font-semibold tracking-tight mb-6">测试</h1>
      <div className="flex flex-col items-center justify-center py-20 rounded-xl border border-border bg-card">
        <div className="w-12 h-12 rounded-xl flex items-center justify-center mb-3 bg-primary/10">
          <FlaskConical className="h-6 w-6 text-primary" />
        </div>
        <h3 className="text-base font-medium mb-1">暂无测试记录</h3>
        <p className="text-sm text-muted-foreground">四层自动化测试结果将在此展示</p>
      </div>
    </div>
  );
}
