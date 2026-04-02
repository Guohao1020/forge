"use client";

import { FileCode2 } from "lucide-react";
import { ShikiCodeViewer } from "@/components/code-preview/shiki-code-viewer";

interface TestCaseListProps {
  files: Array<{ path: string; content: string; language?: string }>;
}

export function TestCaseList({ files }: TestCaseListProps) {
  if (!files.length) {
    return (
      <div className="px-4 py-8 text-center">
        <p className="text-sm text-white/30">暂无测试文件</p>
      </div>
    );
  }

  return (
    <div className="divide-y divide-white/5">
      {files.map((file) => {
        const fileName = file.path.split("/").pop() || file.path;
        return (
          <div key={file.path}>
            <div className="flex items-center gap-2 px-4 py-2 bg-white/[0.02]">
              <FileCode2 size={14} className="text-white/40 shrink-0" />
              <span className="text-xs font-mono text-white/60 truncate">
                {file.path}
              </span>
            </div>
            <div className="max-h-[400px] overflow-auto">
              <ShikiCodeViewer
                content={file.content}
                fileName={fileName}
                language={file.language}
              />
            </div>
          </div>
        );
      })}

      <div className="px-4 py-3 bg-white/[0.01]">
        <p className="text-xs text-white/30 italic">
          Phase 1: 仅展示 AI 生成的测试代码，运行结果将在后续版本支持
        </p>
      </div>
    </div>
  );
}
