"use client";

import { Rocket } from "lucide-react";

export default function DeployPage() {
  return (
    <div>
      <h1 className="text-2xl font-semibold tracking-tight mb-6">部署</h1>
      <div className="flex flex-col items-center justify-center py-20 rounded-xl border border-border bg-card">
        <div className="w-12 h-12 rounded-xl flex items-center justify-center mb-3 bg-primary/10">
          <Rocket className="h-6 w-6 text-primary" />
        </div>
        <h3 className="text-base font-medium mb-1">暂无部署记录</h3>
        <p className="text-sm text-muted-foreground">CI/CD 流水线执行结果将在此展示</p>
      </div>
    </div>
  );
}
