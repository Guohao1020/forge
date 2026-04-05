"use client";

interface CodeViewerProps {
  content: string;
  language?: string;
  fileName?: string;
}

export function CodeViewer({ content, language, fileName }: CodeViewerProps) {
  if (!content) {
    return (
      <div className="flex items-center justify-center h-full text-muted-foreground/40 text-sm">
        Select a file from the left to view code
      </div>
    );
  }

  const lines = content.split("\n");

  return (
    <div className="h-full flex flex-col">
      {fileName && (
        <div className="flex items-center px-4 py-2 border-b border-border bg-muted/20">
          <span className="text-xs text-muted-foreground font-mono">{fileName}</span>
          {language && (
            <span className="ml-2 text-xs text-muted-foreground/60">{language}</span>
          )}
        </div>
      )}
      <div className="flex-1 overflow-auto">
        <pre className="p-4 text-sm font-mono leading-relaxed">
          <code>
            {lines.map((line, i) => (
              <div key={i} className="flex">
                <span className="inline-block w-10 text-right pr-4 text-muted-foreground/40 select-none shrink-0">
                  {i + 1}
                </span>
                <span className="text-foreground/70 whitespace-pre">{line}</span>
              </div>
            ))}
          </code>
        </pre>
      </div>
    </div>
  );
}
