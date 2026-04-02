"use client";

import { useState } from "react";
import { ChevronRight } from "lucide-react";
import { TestCaseList } from "./test-case-list";

interface TestLayerCardProps {
  title: string;
  icon: React.ReactNode;
  status: "available" | "coming_soon";
  testFiles?: Array<{ path: string; content: string; language?: string }>;
  passCount?: number;
  failCount?: number;
  coverage?: number;
  defaultOpen?: boolean;
}

export function TestLayerCard({
  title,
  icon,
  status,
  testFiles,
  passCount,
  failCount,
  coverage,
  defaultOpen = false,
}: TestLayerCardProps) {
  const [open, setOpen] = useState(defaultOpen);
  const hasFiles = status === "available" && testFiles && testFiles.length > 0;

  return (
    <div className="rounded-xl border border-white/10 bg-white/[0.02] overflow-hidden">
      <button
        type="button"
        onClick={() => setOpen(!open)}
        className="flex items-center w-full gap-3 px-4 py-3 text-left hover:bg-white/[0.03] transition-colors"
      >
        <ChevronRight
          size={16}
          className={`text-white/40 transition-transform duration-200 shrink-0 ${
            open ? "rotate-90" : ""
          }`}
        />
        <span className="text-white/70 shrink-0">{icon}</span>
        <span className="text-sm font-medium text-white/90">{title}</span>

        {status === "available" ? (
          <span className="ml-2 inline-flex items-center px-2 py-0.5 rounded-full text-xs font-medium bg-emerald-500/10 text-emerald-400 border border-emerald-500/20">
            可用
          </span>
        ) : (
          <span className="ml-2 inline-flex items-center px-2 py-0.5 rounded-full text-xs font-medium bg-white/5 text-white/40 border border-white/10">
            即将上线
          </span>
        )}

        <span className="flex-1" />

        {passCount !== undefined && (
          <span className="text-xs text-emerald-400 mr-2">
            ✅ {passCount} 通过
          </span>
        )}
        {failCount !== undefined && failCount > 0 && (
          <span className="text-xs text-red-400 mr-2">
            ❌ {failCount} 失败
          </span>
        )}
        {hasFiles && (
          <span className="text-xs text-white/30">
            {testFiles.length} 个文件
          </span>
        )}
      </button>

      {open && (
        <div className="border-t border-white/10">
          {coverage !== undefined && (
            <div className="px-4 py-2 border-b border-white/10 bg-white/[0.01]">
              <div className="flex items-center justify-between text-xs mb-1">
                <span className="text-white/50">覆盖率</span>
                <span className="text-white/70">{coverage}%</span>
              </div>
              <div className="h-1.5 rounded-full bg-white/10 overflow-hidden">
                <div
                  className="h-full rounded-full bg-emerald-500 transition-all"
                  style={{ width: `${Math.min(coverage, 100)}%` }}
                />
              </div>
            </div>
          )}

          {status === "available" && hasFiles ? (
            <TestCaseList files={testFiles} />
          ) : status === "coming_soon" ? (
            <div className="px-4 py-8 text-center">
              <p className="text-sm text-white/30">
                此测试层将在后续版本中支持
              </p>
            </div>
          ) : (
            <div className="px-4 py-8 text-center">
              <p className="text-sm text-white/30">暂无测试文件</p>
            </div>
          )}
        </div>
      )}
    </div>
  );
}
