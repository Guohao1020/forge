"use client"

import { useState } from "react"
import { cn } from "@/lib/utils"
import { FileCode, GitCompare } from "lucide-react"

interface CodeFile {
  path: string
  content: string
  language?: string
}

interface CodePanelProps {
  files: CodeFile[]
  diffContent?: string
  className?: string
}

export function CodePanel({ files, diffContent, className }: CodePanelProps) {
  const [activeTab, setActiveTab] = useState(0)
  const [showDiff, setShowDiff] = useState(false)

  if (files.length === 0 && !diffContent) {
    return (
      <div className={cn("flex flex-col h-full border-l border-[var(--border)]", className)}>
        <div className="flex items-center justify-center h-full text-[var(--text-dim)] text-sm">
          <div className="text-center space-y-2">
            <FileCode className="h-8 w-8 mx-auto opacity-40" />
            <p>No files yet</p>
          </div>
        </div>
      </div>
    )
  }

  const activeFile = files[activeTab]

  return (
    <div className={cn("flex flex-col h-full border-l border-[var(--border)]", className)}>
      {/* Tab bar */}
      <div className="flex items-center border-b border-[var(--border)] bg-[var(--surface)] overflow-x-auto">
        {files.map((file, i) => (
          <button
            key={file.path}
            onClick={() => { setActiveTab(i); setShowDiff(false) }}
            className={cn(
              "px-3 py-1.5 text-xs font-mono whitespace-nowrap border-b-2 transition-colors duration-150",
              i === activeTab && !showDiff
                ? "border-[var(--accent)] text-[var(--text)]"
                : "border-transparent text-[var(--text-dim)] hover:text-[var(--text)]",
            )}
          >
            {file.path.split("/").pop()}
          </button>
        ))}
        {diffContent && (
          <>
            <div className="w-px h-4 bg-[var(--border)] mx-1" />
            <button
              onClick={() => setShowDiff(true)}
              className={cn(
                "px-3 py-1.5 text-xs font-mono whitespace-nowrap border-b-2 flex items-center gap-1 transition-colors duration-150",
                showDiff
                  ? "border-[var(--accent)] text-[var(--text)]"
                  : "border-transparent text-[var(--text-dim)] hover:text-[var(--text)]",
              )}
            >
              <GitCompare className="h-3 w-3" />
              Diff
            </button>
          </>
        )}
      </div>

      {/* Content */}
      <div className="flex-1 overflow-auto">
        {showDiff && diffContent ? (
          <pre className="font-mono text-xs p-3 leading-relaxed">
            {diffContent.split("\n").map((line, i) => (
              <div
                key={i}
                className={cn(
                  "px-2",
                  line.startsWith("+") && !line.startsWith("+++") && "bg-[var(--code-added-bg)] text-[var(--code-added)]",
                  line.startsWith("-") && !line.startsWith("---") && "bg-[var(--code-removed-bg)] text-[var(--code-removed)]",
                )}
              >
                {line}
              </div>
            ))}
          </pre>
        ) : activeFile ? (
          <pre className="font-mono text-xs p-3 leading-relaxed">
            {activeFile.content.split("\n").map((line, i) => (
              <div key={i} className="px-2">
                <span className="text-[var(--text-dim)] select-none inline-block w-8 text-right mr-3">
                  {i + 1}
                </span>
                {line}
              </div>
            ))}
          </pre>
        ) : null}
      </div>
    </div>
  )
}
