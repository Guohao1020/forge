import { describe, expect, it } from "vitest"
import { formatToolSummary } from "./tool-formatters"

describe("formatToolSummary", () => {
  describe("read_file", () => {
    it("shows path and line count", () => {
      const summary = formatToolSummary(
        "read_file",
        { path: "src/main.go" },
        "     1\tpackage main\n     2\t\n     3\timport \"fmt\"\n",
      )
      expect(summary.icon).toBe("\u{1F50D}")
      expect(summary.label).toBe("src/main.go")
      expect(summary.status).toContain("lines")
    })
  })

  describe("write_file", () => {
    it("shows path and created status", () => {
      const summary = formatToolSummary(
        "write_file",
        { path: "new.txt", content: "hello" },
        "Wrote 1 line(s), 5 byte(s) to new.txt",
      )
      expect(summary.icon).toBe("\u{270F}\u{FE0F}")
      expect(summary.label).toBe("new.txt")
      expect(summary.status).toBe("created")
    })
  })

  describe("edit_file", () => {
    it("parses +X -Y from output", () => {
      const summary = formatToolSummary(
        "edit_file",
        { path: "foo.py" },
        "Replaced in foo.py (+3 -1 line(s))",
      )
      expect(summary.status).toBe("+3 -1")
    })

    it("falls back to 'edited' on unparseable output", () => {
      const summary = formatToolSummary(
        "edit_file",
        { path: "foo.py" },
        "something weird",
      )
      expect(summary.status).toBe("edited")
    })
  })

  describe("glob", () => {
    it("counts matches from newline-separated output", () => {
      const summary = formatToolSummary(
        "glob",
        { pattern: "**/*.go" },
        "src/main.go\nsrc/util/helper.go\n",
      )
      expect(summary.icon).toBe("\u{1F4C1}")
      expect(summary.label).toBe("**/*.go")
      expect(summary.status).toBe("2 matches")
    })

    it("reports 0 matches on 'No matches' output", () => {
      const summary = formatToolSummary(
        "glob",
        { pattern: "*.xyz" },
        "No matches for pattern '*.xyz'",
      )
      expect(summary.status).toBe("0 matches")
    })
  })

  describe("grep", () => {
    it("counts result lines", () => {
      const summary = formatToolSummary(
        "grep",
        { pattern: "TODO" },
        "a.go:12:// TODO fix this\nb.go:5:// TODO refactor\n",
      )
      expect(summary.icon).toBe("\u{1F50E}")
      expect(summary.status).toBe("2 results")
    })
  })

  describe("list_directory", () => {
    it("counts items", () => {
      const summary = formatToolSummary(
        "list_directory",
        { path: "src" },
        "util/\nmain.go\nhandler.go\n",
      )
      expect(summary.icon).toBe("\u{1F4C2}")
      expect(summary.label).toBe("src")
      expect(summary.status).toBe("3 items")
    })

    it("shows . when no path provided", () => {
      const summary = formatToolSummary("list_directory", {}, "a\nb\n")
      expect(summary.label).toBe(".")
    })
  })

  describe("bash", () => {
    it("truncates long commands and parses exit code", () => {
      const longCmd = "go build ./... " + "-tag=".repeat(20)
      const summary = formatToolSummary(
        "bash",
        { command: longCmd },
        "$ " + longCmd + "\nexit code: 0\n\nbuild output",
      )
      expect(summary.icon).toBe("\u{25B6}")
      expect(summary.label.length).toBeLessThanOrEqual(60)
      expect(summary.status).toBe("ok")
    })

    it("shows exit code when non-zero", () => {
      const summary = formatToolSummary(
        "bash",
        { command: "go build" },
        "$ go build\nexit code: 1\n\nerror: ...",
      )
      expect(summary.status).toBe("exit 1")
    })
  })

  describe("set_phase", () => {
    it("has hideCard: true", () => {
      const summary = formatToolSummary(
        "set_phase",
        { phase: "Generate" },
        "Phase set to Generate",
      )
      expect(summary.hideCard).toBe(true)
      expect(summary.label).toContain("Generate")
    })
  })

  describe("unknown tool", () => {
    it("falls through to generic formatter", () => {
      const summary = formatToolSummary("mystery_tool", { x: 1 }, "out")
      expect(summary.icon).toBe("\u{1F6E0}")
      expect(summary.label).toBe("mystery_tool")
      expect(summary.hideCard).toBeUndefined()
    })
  })
})
