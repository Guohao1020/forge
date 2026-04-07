"use client"

import { useEffect, useMemo, useState } from "react"
import { cn } from "@/lib/utils"
import { FileCode, GitCompare, X } from "lucide-react"
import type { BundledLanguage, BundledTheme, Highlighter } from "shiki"

interface CodeFile {
  path: string
  content: string
  language?: string
}

interface CodePanelProps {
  files: CodeFile[]
  diffContent?: string
  errorLines?: number[]
  onClose?: (index: number) => void
  className?: string
}

// Mockup variant-B-dense.html lines 660-820. Shiki gives real VS Code grammar
// syntax highlighting that matches the Cursor/Dense Engineering aesthetic.
// Theme pair: GitHub Light + GitHub Dark (closest to mockup's syntax colors).
const LIGHT_THEME: BundledTheme = "github-light"
const DARK_THEME: BundledTheme = "github-dark"

// Language extension -> Shiki grammar id. Covers the languages the platform
// generates code for; falls back to plain text for unknowns.
const LANG_BY_EXT: Record<string, BundledLanguage> = {
  ts: "typescript",
  tsx: "tsx",
  js: "javascript",
  jsx: "jsx",
  java: "java",
  py: "python",
  go: "go",
  rs: "rust",
  rb: "ruby",
  sh: "bash",
  md: "markdown",
  yml: "yaml",
  yaml: "yaml",
  json: "json",
  html: "html",
  css: "css",
  sql: "sql",
  kt: "kotlin",
}

// Colored dot per language — mockup line 669 uses language-branded circles.
const LANG_DOT: Record<string, string> = {
  ts: "bg-[#3178c6]",
  tsx: "bg-[#3178c6]",
  js: "bg-[#f7df1e]",
  jsx: "bg-[#f7df1e]",
  java: "bg-[#f89820]",
  py: "bg-[#3776ab]",
  go: "bg-[#00add8]",
  rs: "bg-[#dea584]",
  rb: "bg-[#cc342d]",
  sh: "bg-[#4eaa25]",
  md: "bg-[var(--text-tertiary)]",
  yml: "bg-[#cb171e]",
  yaml: "bg-[#cb171e]",
  json: "bg-[#cbcb41]",
  html: "bg-[#e34c26]",
  css: "bg-[#264de4]",
  sql: "bg-[#336791]",
  kt: "bg-[#7f52ff]",
}

function extOf(path: string): string {
  const m = path.match(/\.([^./]+)$/)
  return m ? m[1].toLowerCase() : ""
}

function languageOf(file: CodeFile): BundledLanguage {
  if (file.language && file.language in LANG_BY_EXT) {
    return LANG_BY_EXT[file.language] ?? "text" as BundledLanguage
  }
  const ext = extOf(file.path)
  return LANG_BY_EXT[ext] ?? ("text" as BundledLanguage)
}

// Singleton highlighter — load once, reuse across renders and components.
// Lazy imported via dynamic import so it doesn't bloat the initial bundle.
let highlighterPromise: Promise<Highlighter> | null = null

async function getHighlighter(): Promise<Highlighter> {
  if (!highlighterPromise) {
    highlighterPromise = (async () => {
      const { getSingletonHighlighter } = await import("shiki")
      return getSingletonHighlighter({
        themes: [LIGHT_THEME, DARK_THEME],
        langs: Object.values(LANG_BY_EXT),
      })
    })()
  }
  return highlighterPromise
}

function useDarkMode(): boolean {
  const [dark, setDark] = useState(false)
  useEffect(() => {
    const check = () =>
      setDark(document.documentElement.classList.contains("dark"))
    check()
    const obs = new MutationObserver(check)
    obs.observe(document.documentElement, {
      attributes: true,
      attributeFilter: ["class"],
    })
    return () => obs.disconnect()
  }, [])
  return dark
}

function useHighlighted(
  code: string,
  lang: BundledLanguage,
  dark: boolean,
): string[] {
  const [html, setHtml] = useState<string[]>([])
  useEffect(() => {
    let alive = true
    ;(async () => {
      try {
        const hl = await getHighlighter()
        if (!alive) return
        const rendered = hl.codeToHtml(code, {
          lang,
          theme: dark ? DARK_THEME : LIGHT_THEME,
        })
        // Shiki wraps output in <pre><code>...</code></pre>. Extract the
        // inner code so we can render lines alongside gutter/minimap.
        const match = rendered.match(/<code[^>]*>([\s\S]*?)<\/code>/)
        const inner = match?.[1] ?? rendered
        // Shiki inserts <span class="line">lineHtml</span> per line.
        const lines = inner
          .split(/<span class="line">/)
          .slice(1)
          .map((s) => s.replace(/<\/span>\s*$/, ""))
        if (alive) setHtml(lines)
      } catch {
        // Plain fallback
        if (alive) {
          const escaped = code
            .replace(/&/g, "&amp;")
            .replace(/</g, "&lt;")
            .replace(/>/g, "&gt;")
          setHtml(escaped.split("\n"))
        }
      }
    })()
    return () => {
      alive = false
    }
  }, [code, lang, dark])
  return html
}

function Breadcrumb({ path }: { path: string }) {
  const parts = path.split("/")
  return (
    <div className="flex items-center gap-0.5 px-2.5 py-0.5 font-mono text-[10px] text-[var(--text-tertiary)] bg-[var(--bg-secondary)] border-b border-[var(--border-secondary)]">
      {parts.map((p, i) => (
        <span key={i} className="flex items-center gap-0.5">
          {i > 0 && (
            <span className="text-[var(--border-primary)]" aria-hidden>
              ›
            </span>
          )}
          <span
            className={cn(
              "cursor-pointer hover:text-[var(--text-link)] transition-colors duration-100",
              i === parts.length - 1 && "text-[var(--text-secondary)]",
            )}
          >
            {p}
          </span>
        </span>
      ))}
    </div>
  )
}

function EmptyState() {
  return (
    <div className="flex items-center justify-center h-full bg-[var(--bg-primary)]">
      <div className="text-center space-y-2">
        <FileCode className="h-8 w-8 mx-auto text-[var(--text-tertiary)] opacity-40" />
        <p className="font-mono text-[11px] text-[var(--text-tertiary)]">
          No files yet
        </p>
      </div>
    </div>
  )
}

function CodeView({
  file,
  errorLines,
  dark,
}: {
  file: CodeFile
  errorLines: number[]
  dark: boolean
}) {
  const lang = languageOf(file)
  const lines = useHighlighted(file.content, lang, dark)
  const gutterWidth = useMemo(
    () => Math.max(36, String(lines.length).length * 8 + 20),
    [lines.length],
  )
  const errorSet = useMemo(() => new Set(errorLines), [errorLines])

  return (
    <div className="flex-1 overflow-auto bg-[var(--bg-primary)]">
      <div className="flex min-w-max font-mono text-[11px]">
        {/* Gutter (line numbers) */}
        <div
          className="shrink-0 py-1.5 pl-2.5 pr-2 text-right text-[10px] text-[var(--text-tertiary)] bg-[var(--bg-secondary)] border-r border-[var(--border-secondary)] select-none whitespace-pre"
          style={{ minWidth: `${gutterWidth}px` }}
        >
          {lines.map((_, i) => (
            <div key={i} style={{ height: "16.5px", lineHeight: "16.5px" }}>
              {i + 1}
            </div>
          ))}
        </div>

        {/* Lines */}
        <div className="flex-1 py-1.5 pl-3 pr-4">
          {lines.map((line, i) => {
            const isError = errorSet.has(i + 1)
            return (
              <div
                key={i}
                style={{ height: "16.5px", lineHeight: "16.5px" }}
                className={cn(
                  "whitespace-pre",
                  isError &&
                    "bg-[var(--bg-error)] border-l-2 border-[var(--text-error)] pl-2 -ml-2",
                )}
                // Shiki-rendered HTML is trusted (we control the source path).
                dangerouslySetInnerHTML={{ __html: line || "\u00A0" }}
              />
            )
          })}
        </div>

        {/* Minimap stub — 40px right column, per mockup. Real minimap is
            Stream 5+ polish; for now a subtle gradient placeholder so the
            layout matches the mockup without the viewport navigator. */}
        <div
          className="shrink-0 w-10 bg-[var(--bg-secondary)] border-l border-[var(--border-secondary)]"
          aria-hidden
        />
      </div>
    </div>
  )
}

function DiffView({ diff }: { diff: string }) {
  const lines = diff.split("\n")
  return (
    <div className="flex-1 overflow-auto bg-[var(--bg-primary)]">
      <pre className="font-mono text-[11px] py-1.5">
        {lines.map((line, i) => {
          const isAdd = line.startsWith("+") && !line.startsWith("+++")
          const isRem = line.startsWith("-") && !line.startsWith("---")
          return (
            <div
              key={i}
              style={{ height: "16.5px", lineHeight: "16.5px" }}
              className={cn(
                "whitespace-pre px-3",
                isAdd && "bg-[rgba(46,160,67,0.1)] text-[var(--text-success)]",
                isRem && "bg-[rgba(248,81,73,0.1)] text-[var(--text-error)]",
              )}
            >
              {line || "\u00A0"}
            </div>
          )
        })}
      </pre>
    </div>
  )
}

export function CodePanel({
  files,
  diffContent,
  errorLines = [],
  onClose,
  className,
}: CodePanelProps) {
  const [activeTab, setActiveTab] = useState(0)
  const [showDiff, setShowDiff] = useState(false)
  const dark = useDarkMode()

  // Clamp during render — React 19 prefers derived state over effect-based
  // setState (avoids cascading renders). If `files` shrinks below activeTab,
  // the effective tab is the last valid index.
  const clampedTab =
    files.length === 0 ? 0 : Math.min(activeTab, files.length - 1)

  if (files.length === 0 && !diffContent) {
    return (
      <div
        className={cn(
          "flex flex-col h-full bg-[var(--bg-primary)]",
          className,
        )}
        role="tabpanel"
      >
        <EmptyState />
      </div>
    )
  }

  const activeFile = files[clampedTab]

  return (
    <div
      className={cn(
        "flex flex-col h-full bg-[var(--bg-primary)] min-w-0",
        className,
      )}
    >
      {/* Tab bar — mockup lines 660-720 */}
      <div
        role="tablist"
        aria-label="Open files"
        className="flex items-center h-8 bg-[var(--bg-secondary)] border-b border-[var(--border-primary)] overflow-x-auto shrink-0"
      >
        {files.map((file, i) => {
          const ext = extOf(file.path)
          const filename = file.path.split("/").pop() ?? file.path
          const isActive = i === clampedTab && !showDiff
          return (
            <button
              key={file.path + i}
              role="tab"
              aria-selected={isActive}
              aria-controls="code-tabpanel"
              onClick={() => {
                setActiveTab(i)
                setShowDiff(false)
              }}
              className={cn(
                "group/tab flex items-center gap-1.5 h-full px-2.5 whitespace-nowrap border-r border-[var(--border-secondary)] font-mono text-[11px] transition-colors duration-100",
                isActive
                  ? "bg-[var(--bg-primary)] text-[var(--text-primary)]"
                  : "text-[var(--text-secondary)] hover:bg-[var(--bg-hover)]",
              )}
            >
              <span
                className={cn(
                  "w-2 h-2 rounded-full shrink-0",
                  LANG_DOT[ext] ?? "bg-[var(--text-tertiary)]",
                )}
                aria-hidden
              />
              <span>{filename}</span>
              {onClose && (
                <span
                  role="button"
                  tabIndex={0}
                  onClick={(e) => {
                    e.stopPropagation()
                    onClose(i)
                  }}
                  onKeyDown={(e) => {
                    if (e.key === "Enter" || e.key === " ") {
                      e.preventDefault()
                      e.stopPropagation()
                      onClose(i)
                    }
                  }}
                  aria-label={`Close ${filename}`}
                  className="opacity-0 group-hover/tab:opacity-60 hover:opacity-100 transition-opacity duration-100 p-0.5 rounded hover:bg-[var(--bg-active)]"
                >
                  <X className="h-3 w-3" />
                </span>
              )}
            </button>
          )
        })}
        {diffContent && (
          <button
            role="tab"
            aria-selected={showDiff}
            aria-controls="code-tabpanel"
            onClick={() => setShowDiff(true)}
            className={cn(
              "flex items-center gap-1 h-full px-2.5 whitespace-nowrap border-l border-[var(--border-secondary)] font-mono text-[11px] transition-colors duration-100",
              showDiff
                ? "bg-[var(--bg-primary)] text-[var(--text-primary)]"
                : "text-[var(--text-secondary)] hover:bg-[var(--bg-hover)]",
            )}
          >
            <GitCompare className="h-3 w-3" />
            Diff
          </button>
        )}
      </div>

      {/* Breadcrumb */}
      {activeFile && !showDiff && <Breadcrumb path={activeFile.path} />}

      {/* Content */}
      <div id="code-tabpanel" role="tabpanel" className="flex-1 flex min-h-0">
        {showDiff && diffContent ? (
          <DiffView diff={diffContent} />
        ) : activeFile ? (
          <CodeView
            key={activeFile.path}
            file={activeFile}
            errorLines={errorLines}
            dark={dark}
          />
        ) : (
          <EmptyState />
        )}
      </div>
    </div>
  )
}
