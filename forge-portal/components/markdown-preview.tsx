"use client";

import ReactMarkdown from "react-markdown";
import remarkGfm from "remark-gfm";

interface MarkdownPreviewProps {
  content: string;
  className?: string;
}

export function MarkdownPreview({ content, className = "" }: MarkdownPreviewProps) {
  if (!content?.trim()) {
    return (
      <div className={`flex items-center justify-center h-full text-muted-foreground/40 text-sm ${className}`}>
        在左侧输入 Markdown 内容，这里会实时预览
      </div>
    );
  }

  return (
    <div className={`markdown-preview ${className}`}>
      <ReactMarkdown remarkPlugins={[remarkGfm]}>{content}</ReactMarkdown>
    </div>
  );
}
