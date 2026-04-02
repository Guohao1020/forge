"use client";

import { FileCode, FilePlus, FilePen } from "lucide-react";

export interface ChangeFile {
  path: string;
  action: string;
  language?: string;
  content: string;
  linesCount: number;
}

export interface ChangeFileListProps {
  files: ChangeFile[];
  selectedPath?: string;
  onSelectFile: (path: string) => void;
}

const ACTION_CONFIG: Record<string, { icon: typeof FileCode; color: string; label: string }> = {
  create: { icon: FilePlus, color: "text-emerald-400", label: "新增" },
  modify: { icon: FilePen, color: "text-yellow-400", label: "修改" },
};

function languageBadge(lang?: string) {
  if (!lang) return null;
  return (
    <span className="px-1.5 py-0.5 rounded text-[10px] bg-white/5 text-white/40 border border-white/5 font-mono">
      {lang}
    </span>
  );
}

export function ChangeFileList({ files, selectedPath, onSelectFile }: ChangeFileListProps) {
  if (files.length === 0) {
    return (
      <div className="text-center py-8 text-sm text-white/30">
        暂无变更文件
      </div>
    );
  }

  return (
    <div className="divide-y divide-white/5">
      {files.map((file) => {
        const config = ACTION_CONFIG[file.action] || ACTION_CONFIG.create;
        const Icon = config.icon;
        const isSelected = file.path === selectedPath;

        return (
          <button
            key={file.path}
            onClick={() => onSelectFile(file.path)}
            className={`w-full flex items-center gap-3 px-4 py-3 text-left transition-colors hover:bg-white/[0.03] ${
              isSelected ? "bg-primary/5 border-l-2 border-l-primary" : ""
            }`}
          >
            <Icon className={`h-4 w-4 shrink-0 ${config.color}`} />
            <span className="flex-1 min-w-0 text-sm text-white/70 font-mono truncate">
              {file.path}
            </span>
            {languageBadge(file.language)}
            <span className="text-xs text-white/30 shrink-0">
              {file.linesCount} 行
            </span>
          </button>
        );
      })}
    </div>
  );
}
