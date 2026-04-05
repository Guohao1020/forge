"use client";

import { useEffect, useState } from "react";
import type { Highlighter } from "shiki";

// Module-level cache for the highlighter instance
let highlighterInstance: Highlighter | null = null;
let highlighterPromise: Promise<Highlighter> | null = null;

const SUPPORTED_LANGUAGES = [
  "java",
  "go",
  "python",
  "javascript",
  "typescript",
  "sql",
  "yaml",
  "json",
  "xml",
  "bash",
  "dockerfile",
  "css",
  "html",
] as const;

const EXTENSION_TO_LANGUAGE: Record<string, string> = {
  java: "java",
  go: "go",
  py: "python",
  js: "javascript",
  jsx: "javascript",
  ts: "typescript",
  tsx: "typescript",
  sql: "sql",
  yml: "yaml",
  yaml: "yaml",
  json: "json",
  xml: "xml",
  sh: "bash",
  bash: "bash",
  dockerfile: "dockerfile",
  css: "css",
  html: "html",
  htm: "html",
  md: "markdown",
  rs: "rust",
  rb: "ruby",
  php: "php",
  vue: "vue",
  svelte: "svelte",
};

function detectLanguage(fileName?: string, language?: string): string {
  if (language) return language;
  if (!fileName) return "text";
  const ext = fileName.split(".").pop()?.toLowerCase() || "";
  // Handle Dockerfile (no extension)
  if (fileName.toLowerCase().includes("dockerfile")) return "dockerfile";
  return EXTENSION_TO_LANGUAGE[ext] || "text";
}

async function getHighlighter(): Promise<Highlighter> {
  if (highlighterInstance) return highlighterInstance;
  if (highlighterPromise) return highlighterPromise;

  highlighterPromise = import("shiki").then(async ({ createHighlighter }) => {
    const instance = await createHighlighter({
      themes: ["vitesse-light"],
      langs: [...SUPPORTED_LANGUAGES],
    });
    highlighterInstance = instance;
    return instance;
  });

  return highlighterPromise;
}

interface ShikiCodeViewerProps {
  content: string;
  language?: string;
  fileName?: string;
}

export function ShikiCodeViewer({
  content,
  language,
  fileName,
}: ShikiCodeViewerProps) {
  const [highlightedHtml, setHighlightedHtml] = useState<string>("");
  const [isLoading, setIsLoading] = useState(true);

  const detectedLang = detectLanguage(fileName, language);

  useEffect(() => {
    if (!content) {
      setIsLoading(false);
      return;
    }

    let cancelled = false;
    setIsLoading(true);

    (async () => {
      try {
        const highlighter = await getHighlighter();

        // Load language on demand if not already loaded
        const loadedLangs = highlighter.getLoadedLanguages();
        if (detectedLang !== "text" && !loadedLangs.includes(detectedLang as never)) {
          try {
            await highlighter.loadLanguage(detectedLang as never);
          } catch {
            // If language fails to load, fall back to text
          }
        }

        const langToUse = highlighter.getLoadedLanguages().includes(detectedLang as never)
          ? detectedLang
          : "text";

        const html = highlighter.codeToHtml(content, {
          lang: langToUse,
          theme: "vitesse-light",
        });

        if (!cancelled) {
          setHighlightedHtml(html);
          setIsLoading(false);
        }
      } catch {
        if (!cancelled) {
          setIsLoading(false);
        }
      }
    })();

    return () => {
      cancelled = true;
    };
  }, [content, detectedLang]);

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
          {detectedLang && detectedLang !== "text" && (
            <span className="ml-2 text-xs text-muted-foreground/60">{detectedLang}</span>
          )}
        </div>
      )}
      <div className="flex-1 overflow-auto">
        {isLoading ? (
          /* Plain text fallback while Shiki loads */
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
        ) : (
          <div
            className="shiki-container bg-white text-sm font-mono overflow-auto [&_.shiki]:!bg-transparent [&_pre]:p-4 [&_pre]:leading-relaxed [&_code]:![counter-reset:line] [&_code>.line]:[counter-increment:line] [&_code>.line]:before:content-[counter(line)] [&_code>.line]:before:inline-block [&_code>.line]:before:w-10 [&_code>.line]:before:text-right [&_code>.line]:before:pr-4 [&_code>.line]:before:text-muted-foreground/40 [&_code>.line]:before:select-none [&_code>.line]:before:shrink-0"
            dangerouslySetInnerHTML={{ __html: highlightedHtml }}
          />
        )}
      </div>
    </div>
  );
}
