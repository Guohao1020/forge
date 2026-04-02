"use client";

import dynamic from "next/dynamic";
import { X, FileCode } from "lucide-react";

const DiffEditor = dynamic(
  () => import("@monaco-editor/react").then((mod) => mod.DiffEditor),
  {
    ssr: false,
    loading: () => (
      <div className="h-[500px] flex items-center justify-center bg-[#1e1e1e] rounded-b-lg">
        <div className="flex items-center gap-2 text-white/30 text-sm">
          <div className="h-4 w-4 border-2 border-white/20 border-t-primary rounded-full animate-spin" />
          加载编辑器...
        </div>
      </div>
    ),
  }
);

export interface ChangeDiffViewProps {
  fileName: string;
  language: string;
  originalContent: string;
  modifiedContent: string;
  isNewFile?: boolean;
  onClose: () => void;
}

// Map common language identifiers to Monaco language IDs
function monacoLanguage(lang: string): string {
  const map: Record<string, string> = {
    js: "javascript",
    ts: "typescript",
    jsx: "javascript",
    tsx: "typescript",
    py: "python",
    go: "go",
    java: "java",
    sql: "sql",
    yaml: "yaml",
    yml: "yaml",
    json: "json",
    md: "markdown",
    html: "html",
    css: "css",
    scss: "scss",
    sh: "shell",
    bash: "shell",
    xml: "xml",
    toml: "ini",
    dockerfile: "dockerfile",
  };
  return map[lang.toLowerCase()] || lang.toLowerCase();
}

export function ChangeDiffView({
  fileName,
  language,
  originalContent,
  modifiedContent,
  isNewFile,
  onClose,
}: ChangeDiffViewProps) {
  return (
    <div className="rounded-xl border border-white/10 bg-[#0A0A12] overflow-hidden">
      {/* Header bar */}
      <div className="flex items-center justify-between px-4 py-2.5 border-b border-white/10 bg-white/[0.02]">
        <div className="flex items-center gap-2 min-w-0">
          <FileCode className="h-4 w-4 text-primary shrink-0" />
          <span className="text-sm text-white/70 font-mono truncate">{fileName}</span>
          <span className="px-1.5 py-0.5 rounded text-[10px] bg-white/5 text-white/40 border border-white/5 font-mono shrink-0">
            {language}
          </span>
          {isNewFile && (
            <span className="px-1.5 py-0.5 rounded text-[10px] bg-emerald-500/10 text-emerald-400 border border-emerald-500/20 shrink-0">
              AI Generated
            </span>
          )}
        </div>
        <button
          onClick={onClose}
          className="p-1 rounded hover:bg-white/10 transition-colors text-white/40 hover:text-white/70"
        >
          <X className="h-4 w-4" />
        </button>
      </div>

      {/* Diff editor */}
      <DiffEditor
        height={500}
        language={monacoLanguage(language)}
        original={originalContent}
        modified={modifiedContent}
        theme="vs-dark"
        options={{
          readOnly: true,
          renderSideBySide: true,
          minimap: { enabled: false },
          scrollBeyondLastLine: false,
          fontSize: 13,
          lineNumbers: "on",
          folding: true,
          wordWrap: "off",
        }}
      />
    </div>
  );
}
