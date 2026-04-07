"use client"

import { useState } from "react"
import { useParams } from "next/navigation"
import { ArrowLeft, Search, Upload, BookOpen, Code2, Shield, TestTube, Eye, Brain } from "lucide-react"
import Link from "next/link"
import { ThemeToggle } from "@/components/theme-toggle"
import { cn } from "@/lib/utils"

interface Skill {
  name: string
  description: string
  purpose: string
  source: "forge" | "community" | "project"
  path?: string
}

const purposeIcon: Record<string, React.ReactNode> = {
  analyze: <Brain className="h-4 w-4" />,
  generate: <Code2 className="h-4 w-4" />,
  review: <Eye className="h-4 w-4" />,
  test: <TestTube className="h-4 w-4" />,
  plan: <BookOpen className="h-4 w-4" />,
  profile: <Shield className="h-4 w-4" />,
}

const sourceLabel: Record<string, { label: string; color: string }> = {
  forge: { label: "Built-in", color: "text-[var(--accent)]" },
  community: { label: "Community", color: "text-[var(--success)]" },
  project: { label: "Project", color: "text-[var(--warning)]" },
}

// Default built-in skills
const defaultSkills: Skill[] = [
  { name: "forge:requirement-analysis", description: "Progressive requirement clarification through multi-round dialogue", purpose: "analyze", source: "forge" },
  { name: "forge:code-generation", description: "Generate production code following project conventions", purpose: "generate", source: "forge" },
  { name: "forge:code-review", description: "Review code for quality, security, and standards compliance", purpose: "review", source: "forge" },
  { name: "forge:test-writing", description: "Generate unit and integration tests for code", purpose: "test", source: "forge" },
  { name: "forge:planning", description: "Break down requirements into implementation plans", purpose: "plan", source: "forge" },
  { name: "forge:project-profiling", description: "Analyze project structure, APIs, schemas, and architecture", purpose: "profile", source: "forge" },
]

export default function SkillsMarketplacePage() {
  const params = useParams()
  const projectId = params.id as string
  const [search, setSearch] = useState("")
  const [skills] = useState<Skill[]>(defaultSkills)

  const filtered = skills.filter(
    s => s.name.toLowerCase().includes(search.toLowerCase()) ||
         s.description.toLowerCase().includes(search.toLowerCase()),
  )

  return (
    <div className="flex flex-col h-screen bg-[var(--background)]">
      {/* Header */}
      <header className="flex items-center h-10 px-4 border-b border-[var(--border)] bg-[var(--surface)]">
        <Link
          href={`/projects/${projectId}/agent`}
          className="flex items-center gap-1.5 text-xs text-[var(--text-muted)] hover:text-[var(--text)] transition-colors duration-150"
        >
          <ArrowLeft className="h-3.5 w-3.5" />
          Agent Terminal
        </Link>
        <div className="mx-3 w-px h-4 bg-[var(--border)]" />
        <h1 className="text-xs font-medium">Skills</h1>
        <div className="flex-1" />
        <ThemeToggle />
      </header>

      {/* Toolbar */}
      <div className="flex items-center gap-3 px-4 py-3 border-b border-[var(--border)]">
        <div className="relative flex-1 max-w-md">
          <Search className="absolute left-3 top-1/2 -translate-y-1/2 h-3.5 w-3.5 text-[var(--text-dim)]" />
          <input
            type="text"
            value={search}
            onChange={e => setSearch(e.target.value)}
            placeholder="Search skills..."
            className="w-full pl-9 pr-3 py-1.5 text-sm rounded border border-[var(--border)] bg-[var(--background)] focus:border-[var(--accent)] focus:outline-none transition-colors duration-150"
          />
        </div>
        <button className="inline-flex items-center gap-1.5 px-3 py-1.5 text-xs font-medium rounded border border-[var(--border)] hover:border-[var(--accent)] hover:text-[var(--accent)] transition-colors duration-150">
          <Upload className="h-3.5 w-3.5" />
          Import Skill
        </button>
      </div>

      {/* Skill list */}
      <div className="flex-1 overflow-y-auto p-4">
        <div className="space-y-2 max-w-3xl">
          {filtered.map(skill => {
            const src = sourceLabel[skill.source]
            return (
              <div
                key={skill.name}
                className="flex items-start gap-3 p-3 rounded border border-[var(--border)] hover:border-[var(--accent)]/30 transition-colors duration-150"
              >
                <div className="p-2 rounded bg-[var(--surface)] text-[var(--accent)]">
                  {purposeIcon[skill.purpose] || <Code2 className="h-4 w-4" />}
                </div>
                <div className="flex-1 min-w-0">
                  <div className="flex items-center gap-2">
                    <span className="text-sm font-mono font-medium">{skill.name}</span>
                    <span className={cn("text-[0.65rem] font-medium", src.color)}>
                      {src.label}
                    </span>
                  </div>
                  <p className="text-xs text-[var(--text-muted)] mt-0.5">{skill.description}</p>
                </div>
              </div>
            )
          })}
          {filtered.length === 0 && (
            <div className="text-center py-8 text-sm text-[var(--text-dim)]">
              No skills found matching "{search}"
            </div>
          )}
        </div>
      </div>
    </div>
  )
}
