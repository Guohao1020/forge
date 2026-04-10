/**
 * Tool card formatters for the Variant B agent UI.
 *
 * Maps a tool invocation (name + input + output) to the visual
 * fields the tool card renders. Each formatter is pure — given
 * the same inputs, returns the same ToolSummary.
 *
 * `hideCard: true` signals that the tool should NOT render a
 * tool card at all. Currently only set_phase uses this — the
 * phase change is shown in the step ribbon, so a redundant tool
 * card for every set_phase call would be visual noise.
 */

export interface ToolSummary {
  icon: string
  label: string
  status: string
  hideCard?: boolean
}

function truncate(s: string, maxLen: number): string {
  if (s.length <= maxLen) return s
  return s.slice(0, maxLen - 3) + "..."
}

// Parse line count from read_file output.
// read_file output contains numbered lines like "    1\tpackage main\n..."
function parseLineCount(output?: string): string {
  if (!output) return ""
  const lines = output.split("\n").length
  return `${lines - 1} lines`
}

function parseEditDelta(output?: string): string {
  if (!output) return ""
  // Match "+X -Y line(s)" from EditFileTool output
  const m = output.match(/\+(\d+)\s+-(\d+)/)
  if (!m) return "edited"
  return `+${m[1]} -${m[2]}`
}

function parseMatchCount(output?: string): string {
  if (!output) return ""
  if (output.startsWith("No matches")) return "0 matches"
  const lines = output.split("\n").filter((l) => l.trim() && !l.startsWith("..."))
  return `${lines.length} matches`
}

function parseResultCount(output?: string): string {
  if (!output) return ""
  if (output.startsWith("No matches")) return "0 results"
  const lines = output.split("\n").filter((l) => l.trim() && !l.startsWith("..."))
  return `${lines.length} results`
}

function parseItemCount(output?: string): string {
  if (!output) return ""
  if (output.includes("(empty directory)")) return "0 items"
  const lines = output.split("\n").filter((l) => l.trim() && !l.startsWith("..."))
  return `${lines.length} items`
}

function parseBashExitCode(output?: string): string {
  if (!output) return ""
  const m = output.match(/^exit code:\s*(-?\d+)/m)
  if (!m) return ""
  const code = m[1]
  return code === "0" ? "ok" : `exit ${code}`
}

/**
 * Format a single tool invocation into its card summary.
 *
 * Unknown tool names fall through to a generic formatter so the
 * card still renders with sensible defaults.
 */
export function formatToolSummary(
  name: string,
  input: Record<string, unknown>,
  output?: string,
): ToolSummary {
  switch (name) {
    // --- T2 file tools ---
    case "read_file":
      return {
        icon: "\u{1F50D}",
        label: String(input.path ?? ""),
        status: parseLineCount(output),
      }

    case "write_file":
      return {
        icon: "\u{270F}\u{FE0F}",
        label: String(input.path ?? ""),
        status: "created",
      }

    case "edit_file":
      return {
        icon: "\u{270F}\u{FE0F}",
        label: String(input.path ?? ""),
        status: parseEditDelta(output),
      }

    case "glob":
      return {
        icon: "\u{1F4C1}",
        label: String(input.pattern ?? ""),
        status: parseMatchCount(output),
      }

    case "grep":
      return {
        icon: "\u{1F50E}",
        label: String(input.pattern ?? ""),
        status: parseResultCount(output),
      }

    case "list_directory":
      return {
        icon: "\u{1F4C2}",
        label: String(input.path ?? "."),
        status: parseItemCount(output),
      }

    // --- T2 exec tools ---
    case "bash":
      return {
        icon: "\u{25B6}",
        label: truncate(String(input.command ?? ""), 60),
        status: parseBashExitCode(output),
      }

    case "set_phase":
      return {
        icon: "\u{2192}",
        label: `Phase: ${String(input.phase ?? "")}`,
        status: "",
        hideCard: true,
      }

    // --- Legacy context tools (kept; Phase 3 already registered them) ---
    case "query_api_catalog":
    case "query_db_schema":
    case "query_business_rules":
    case "query_module_graph":
      return {
        icon: "\u{1F4DA}",
        label: String(input.keyword ?? input.table_name ?? input.domain ?? input.module_name ?? name),
        status: output ? `${output.length} chars` : "",
      }

    case "read_project_file":
      return {
        icon: "\u{1F4D6}",
        label: String(input.path ?? ""),
        status: output ? `${output.length} chars` : "",
      }

    // --- Fallback ---
    default:
      return {
        icon: "\u{1F6E0}",
        label: name,
        status: "",
      }
  }
}

// --- Legacy formatToolInput for backward compatibility ---
// tool-execution.tsx still calls this for collapsed summary text.
type ToolInput = Record<string, unknown>

function str(v: unknown, fallback = "?"): string {
  if (v == null) return fallback
  if (typeof v === "string") return v
  if (typeof v === "number" || typeof v === "boolean") return String(v)
  return fallback
}

function num(v: unknown): string {
  if (typeof v === "number") return String(v)
  return "?"
}

export const toolFormatters: Record<string, (input: ToolInput) => string> = {
  // Context tools
  query_api_catalog: (i) => {
    const q = str(i.query, "")
    return q ? `query: ${q}` : "api catalog lookup"
  },
  query_db_schema: (i) => {
    const t = str(i.table, "")
    return t ? `table: ${t}` : "schema lookup"
  },
  query_business_rules: (i) => {
    const topic = str(i.topic, "")
    return topic ? `topic: ${topic}` : "business rules lookup"
  },
  query_module_graph: (i) => {
    const m = str(i.module, "")
    return m ? `module: ${m}` : "module graph lookup"
  },
  read_project_file: (i) => {
    const path = str(i.path, "")
    const lines = num(i.lines)
    return path ? `${path} \u2022 ${lines} lines` : "read file"
  },
  // T2 file tools
  read_file: (i) => {
    const path = str(i.path, "")
    const lines = num(i.lines)
    return path ? `${path} \u2022 ${lines} lines` : "read file"
  },
  write_file: (i) => {
    const path = str(i.path, "")
    return path || "write file"
  },
  edit_file: (i) => {
    const path = str(i.path, "")
    return path || "edit file"
  },
  glob: (i) => {
    const pattern = str(i.pattern, "")
    return pattern || "glob"
  },
  grep: (i) => {
    const pattern = str(i.pattern, "")
    return pattern ? `"${pattern}"` : "search code"
  },
  list_directory: (i) => {
    const path = str(i.path, ".")
    return path
  },
  bash: (i) => {
    const cmd = str(i.command, "")
    return cmd ? truncate(cmd, 60) : "run command"
  },
  set_phase: (i) => {
    return `Phase: ${str(i.phase, "?")}`
  },
  // Legacy
  execute_command: (i) => {
    const cmd = str(i.command, "")
    return cmd || "run command"
  },
  list_files: (i) => {
    const path = str(i.path, "")
    const count = num(i.count)
    return path ? `${path} \u2022 ${count} items` : "list files"
  },
  search_code: (i) => {
    const q = str(i.query, "")
    return q ? `"${q}"` : "search code"
  },
  build_verify: (i) => {
    const cmd = str(i.command, "")
    return cmd ? `build: ${cmd}` : "build verify"
  },
}

export function formatToolInput(
  toolName: string,
  toolInput: ToolInput | null | undefined,
): string {
  if (!toolInput || typeof toolInput !== "object") return ""
  const formatter = toolFormatters[toolName]
  if (formatter) {
    try {
      return formatter(toolInput as ToolInput)
    } catch {
      // fall through to JSON fallback
    }
  }
  const raw = JSON.stringify(toolInput)
  return raw.length > 60 ? raw.slice(0, 60) + "\u2026" : raw
}
