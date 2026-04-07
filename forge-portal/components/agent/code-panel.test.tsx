import { describe, it, expect, vi } from "vitest"
import { render, screen } from "@testing-library/react"
import { axe } from "vitest-axe"
import { CodePanel } from "./code-panel"

// Shiki is loaded lazily via dynamic import + WASM. In jsdom we stub it out
// so the test can render without pulling in the oniguruma engine.
vi.mock("shiki", () => ({
  getSingletonHighlighter: vi.fn().mockResolvedValue({
    codeToHtml: (code: string) =>
      `<pre><code>${code
        .split("\n")
        .map((l) => `<span class="line">${l}</span>`)
        .join("")}</code></pre>`,
  }),
}))

describe("CodePanel", () => {
  it("renders empty state when no files", () => {
    render(<CodePanel files={[]} />)
    expect(screen.getByText("No files yet")).toBeInTheDocument()
  })

  it("renders tab bar with one tab per file", () => {
    render(
      <CodePanel
        files={[
          { path: "src/App.tsx", content: "export default function App() {}" },
          { path: "src/index.ts", content: "import App from './App'" },
        ]}
      />,
    )
    const tabs = screen.getAllByRole("tab")
    expect(tabs.length).toBe(2)
    expect(tabs[0]).toHaveTextContent("App.tsx")
    expect(tabs[1]).toHaveTextContent("index.ts")
  })

  it("marks the first tab as selected by default", () => {
    render(
      <CodePanel
        files={[
          { path: "a.ts", content: "a" },
          { path: "b.ts", content: "b" },
        ]}
      />,
    )
    const tabs = screen.getAllByRole("tab")
    expect(tabs[0]).toHaveAttribute("aria-selected", "true")
    expect(tabs[1]).toHaveAttribute("aria-selected", "false")
  })

  it("shows a Diff tab when diffContent is provided", () => {
    render(
      <CodePanel
        files={[{ path: "a.ts", content: "a" }]}
        diffContent="- old\n+ new"
      />,
    )
    expect(screen.getByRole("tab", { name: /diff/i })).toBeInTheDocument()
  })

  it("renders breadcrumb for the active file", () => {
    render(
      <CodePanel
        files={[{ path: "src/main/java/com/app/User.java", content: "class User {}" }]}
      />,
    )
    // Breadcrumb renders each path segment; verify the leaf AND a middle
    // segment both exist. Using getAllByText because the filename also
    // appears in the tab label, so strict getByText is ambiguous.
    expect(screen.getAllByText("User.java").length).toBeGreaterThan(0)
    expect(screen.getByText("com")).toBeInTheDocument()
  })

  it("empty state has no axe violations", async () => {
    const { container } = render(<CodePanel files={[]} />)
    const results = await axe(container)
    expect(results).toHaveNoViolations()
  })

  it("populated state has no axe violations", async () => {
    const { container } = render(
      <CodePanel
        files={[{ path: "a.ts", content: "const x = 1" }]}
        diffContent="diff --git"
      />,
    )
    const results = await axe(container)
    expect(results).toHaveNoViolations()
  })
})
