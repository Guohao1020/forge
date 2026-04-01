"use client";

import { useState } from "react";
import { FileTree } from "./file-tree";
import { CodeViewer } from "./code-viewer";
import { GitCommit } from "lucide-react";

interface GeneratedFile {
  path: string;
  content: string;
  action: string;
  language?: string;
}

interface CodePreviewPanelProps {
  files: GeneratedFile[];
  commitMessage?: string;
  filesChanged?: number;
  linesAdded?: number;
  linesDeleted?: number;
}

export function CodePreviewPanel({
  files,
  commitMessage,
  filesChanged,
  linesAdded,
  linesDeleted,
}: CodePreviewPanelProps) {
  const [selectedPath, setSelectedPath] = useState(files[0]?.path || "");
  const selectedFile = files.find((f) => f.path === selectedPath);

  const createCount = files.filter((f) => f.action === "create").length;
  const modifyCount = files.filter((f) => f.action === "modify").length;

  return (
    <div className="border border-white/10 rounded-lg bg-[#0A0A12] overflow-hidden">
      <div className="grid grid-cols-[220px_1fr] h-[400px]">
        {/* File tree */}
        <div className="border-r border-white/10 overflow-hidden">
          <div className="px-3 py-2 border-b border-white/10 bg-white/[0.02]">
            <span className="text-xs text-white/40">
              {files.length} files
              {createCount > 0 && <span className="text-green-400 ml-1.5">+{createCount}</span>}
              {modifyCount > 0 && <span className="text-yellow-400 ml-1.5">~{modifyCount}</span>}
            </span>
          </div>
          <FileTree
            files={files.map((f) => ({ path: f.path, action: f.action, language: f.language }))}
            selectedPath={selectedPath}
            onSelect={setSelectedPath}
          />
        </div>

        {/* Code viewer */}
        <CodeViewer
          content={selectedFile?.content || ""}
          language={selectedFile?.language}
          fileName={selectedFile?.path}
        />
      </div>

      {/* Footer */}
      {commitMessage && (
        <div className="border-t border-white/10 px-4 py-2 flex items-center justify-between bg-white/[0.02]">
          <div className="flex items-center gap-2 text-xs text-white/40">
            <GitCommit className="h-3.5 w-3.5" />
            <span className="font-mono">{commitMessage}</span>
          </div>
          <span className="text-xs text-white/30">
            {filesChanged || files.length} files
            {linesAdded ? ` (+${linesAdded})` : ""}
            {linesDeleted ? ` (-${linesDeleted})` : ""}
          </span>
        </div>
      )}
    </div>
  );
}
