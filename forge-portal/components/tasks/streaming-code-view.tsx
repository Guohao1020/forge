"use client";

import { useEffect, useRef, useMemo } from "react";
import { Loader2, Check, FileCode2 } from "lucide-react";
import { CodePreviewPanel } from "@/components/code-preview/code-preview-panel";

interface ParsedFile {
  path: string;
  content: string;
  action: string;
  language?: string;
}

interface StreamingCodeViewProps {
  tokens: string;
  isStreaming: boolean;
}

/**
 * Incrementally parse the JSON stream from AI code generation.
 *
 * The AI outputs a JSON object like:
 *   {"files": [{"path":"...","content":"...","action":"create","language":"typescript"}, ...], "commit_message": "..."}
 *
 * During streaming, the JSON is incomplete. We extract completed file objects
 * by finding balanced braces within the "files" array.
 */
function parseFilesFromStream(tokens: string): { files: ParsedFile[]; isComplete: boolean; currentFileName?: string } {
  // Try complete JSON parse first
  try {
    const parsed = JSON.parse(tokens);
    if (parsed?.files && Array.isArray(parsed.files)) {
      return { files: parsed.files, isComplete: true };
    }
    // Might be a different format — return as-is
    return { files: [], isComplete: true };
  } catch {
    // Incomplete JSON — extract what we can
  }

  const files: ParsedFile[] = [];

  // Find the start of the "files" array
  const filesArrayStart = tokens.indexOf('"files"');
  if (filesArrayStart === -1) return { files: [], isComplete: false };

  const bracketStart = tokens.indexOf("[", filesArrayStart);
  if (bracketStart === -1) return { files: [], isComplete: false };

  // Walk through the array content, finding complete {...} objects
  let i = bracketStart + 1;
  let currentFileName: string | undefined;

  while (i < tokens.length) {
    // Skip whitespace and commas
    while (i < tokens.length && (tokens[i] === " " || tokens[i] === "\n" || tokens[i] === "\r" || tokens[i] === "\t" || tokens[i] === ",")) {
      i++;
    }

    if (i >= tokens.length || tokens[i] === "]") break;

    if (tokens[i] === "{") {
      // Find the matching closing brace, accounting for nested braces and strings
      let depth = 0;
      let inString = false;
      let escaped = false;
      const objStart = i;

      for (let j = i; j < tokens.length; j++) {
        const ch = tokens[j];
        if (escaped) {
          escaped = false;
          continue;
        }
        if (ch === "\\") {
          escaped = true;
          continue;
        }
        if (ch === '"' && !escaped) {
          inString = !inString;
          continue;
        }
        if (inString) continue;
        if (ch === "{") depth++;
        if (ch === "}") {
          depth--;
          if (depth === 0) {
            // Found complete object
            const objStr = tokens.slice(objStart, j + 1);
            try {
              const obj = JSON.parse(objStr);
              if (obj.path && obj.content !== undefined) {
                files.push(obj);
              }
            } catch {
              // Malformed object, skip
            }
            i = j + 1;
            break;
          }
        }
      }

      // If depth > 0, the object is incomplete — this is the currently generating file
      if (depth > 0) {
        // Try to extract the path of the file being generated
        const pathMatch = tokens.slice(objStart).match(/"path"\s*:\s*"([^"]+)"/);
        if (pathMatch) {
          currentFileName = pathMatch[1];
        }
        break;
      }
    } else {
      // Unexpected character, advance
      i++;
    }
  }

  return { files, isComplete: false, currentFileName };
}

export function StreamingCodeView({ tokens, isStreaming }: StreamingCodeViewProps) {
  const bottomRef = useRef<HTMLDivElement>(null);

  const { files, isComplete, currentFileName } = useMemo(
    () => parseFilesFromStream(tokens),
    [tokens]
  );

  // Auto-scroll when new files appear
  useEffect(() => {
    bottomRef.current?.scrollIntoView({ behavior: "smooth" });
  }, [files.length]);

  // No files parsed yet — show loading
  if (files.length === 0 && isStreaming) {
    return (
      <div className="rounded-xl border border-border bg-card overflow-hidden">
        <div className="flex items-center gap-2 px-4 py-3 border-b border-border">
          <Loader2 className="h-4 w-4 animate-spin text-primary" />
          <span className="text-sm text-muted-foreground">AI 正在生成代码...</span>
        </div>
        <div className="flex items-center justify-center py-16">
          <div className="text-center space-y-2">
            <FileCode2 className="h-8 w-8 text-muted-foreground/30 mx-auto" />
            <p className="text-sm text-muted-foreground/50">等待代码输出...</p>
          </div>
        </div>
      </div>
    );
  }

  // Files available — show code preview
  return (
    <div className="space-y-3">
      {/* Header */}
      <div className="flex items-center gap-2">
        {isStreaming && !isComplete ? (
          <>
            <Loader2 className="h-4 w-4 animate-spin text-primary" />
            <span className="text-sm text-muted-foreground">
              AI 正在生成代码... ({files.length} 个文件已完成
              {currentFileName && `，正在生成 ${currentFileName}`})
            </span>
          </>
        ) : (
          <>
            <Check className="h-4 w-4 text-emerald-500" />
            <span className="text-sm text-muted-foreground">
              代码生成完成 ({files.length} 个文件)
            </span>
          </>
        )}
      </div>

      {/* Code preview panel — reuse the same component as completed view */}
      {files.length > 0 && (
        <CodePreviewPanel files={files} />
      )}

      <div ref={bottomRef} />
    </div>
  );
}
