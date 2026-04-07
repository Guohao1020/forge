// Tool input summarizers for Agent Terminal collapsed view.
//
// Mockup variant-B-dense.html uses one-line labels like
// `read_file  ProductService.java • 142 lines` — far more readable than a
// generic `JSON.stringify(toolInput).slice(0, 60)` fallback.
//
// Each formatter takes the tool's input dict and returns a single-line human
// string. Anything that doesn't have a formatter falls back to JSON.stringify
// truncation in tool-execution.tsx. Add new formatters here as tools are added
// to ai-worker/src/openharness/tools/.

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
  // ---- Context tools (ai-worker/src/openharness/tools/context_tools.py) ----
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
    return path ? `${path} • ${lines} lines` : "read file"
  },

  // ---- Expected pair-pipeline tools (will match Stream 4 additions) ----
  read_file: (i) => {
    const path = str(i.path, "")
    const lines = num(i.lines)
    return path ? `${path} • ${lines} lines` : "read file"
  },
  write_file: (i) => {
    const path = str(i.path, "")
    const lines = num(i.lines)
    return path ? `${path} • +${lines} lines` : "write file"
  },
  edit_file: (i) => {
    const path = str(i.path, "")
    const added = num(i.added)
    const removed = num(i.removed)
    return path ? `${path} • +${added}/-${removed}` : "edit file"
  },
  execute_command: (i) => {
    const cmd = str(i.command, "")
    return cmd || "run command"
  },
  list_files: (i) => {
    const path = str(i.path, "")
    const count = num(i.count)
    return path ? `${path} • ${count} items` : "list files"
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
  return raw.length > 60 ? raw.slice(0, 60) + "…" : raw
}
