import { describe, it, expect } from "vitest"
import { render, screen, fireEvent } from "@testing-library/react"
import { axe } from "vitest-axe"
import { ToolExecution } from "./tool-execution"

describe("ToolExecution", () => {
  const baseProps = {
    toolName: "read_file",
    toolInput: { path: "src/App.tsx", lines: 142 },
  }

  it("renders with role=region and tool name label", () => {
    render(<ToolExecution {...baseProps} />)
    expect(screen.getByRole("region", { name: "read_file" })).toBeInTheDocument()
  })

  it("renders the formatted input summary in collapsed view", () => {
    render(<ToolExecution {...baseProps} />)
    expect(screen.getByText(/src\/App.tsx.*142.*lines/)).toBeInTheDocument()
  })

  it("shows ok badge when no error or loading", () => {
    render(<ToolExecution {...baseProps} />)
    expect(screen.getByText("ok")).toBeInTheDocument()
  })

  it("shows running badge when isLoading", () => {
    render(<ToolExecution {...baseProps} isLoading />)
    expect(screen.getByText("running")).toBeInTheDocument()
  })

  it("shows error badge when isError", () => {
    render(<ToolExecution {...baseProps} isError output="file not found" />)
    expect(screen.getByText("error")).toBeInTheDocument()
  })

  it("expands on click and shows input/output sections", () => {
    render(<ToolExecution {...baseProps} output="file contents" />)
    const toggle = screen.getByRole("button", { expanded: false })
    fireEvent.click(toggle)
    expect(screen.getByText("Input")).toBeInTheDocument()
    expect(screen.getByText("Output")).toBeInTheDocument()
    expect(screen.getByText("file contents")).toBeInTheDocument()
  })

  it("collapses on Esc key when expanded", () => {
    render(<ToolExecution {...baseProps} output="x" />)
    const toggle = screen.getByRole("button", { expanded: false })
    fireEvent.click(toggle)
    expect(toggle).toHaveAttribute("aria-expanded", "true")

    fireEvent.keyDown(window, { key: "Escape" })
    expect(toggle).toHaveAttribute("aria-expanded", "false")
  })

  it("toggle has aria-expanded and aria-controls wired", () => {
    render(<ToolExecution {...baseProps} />)
    const toggle = screen.getByRole("button")
    expect(toggle).toHaveAttribute("aria-expanded", "false")
    expect(toggle).toHaveAttribute("aria-controls")
  })

  it("has no axe violations in each state", async () => {
    for (const extra of [{}, { isLoading: true }, { isError: true, output: "err" }]) {
      const { container, unmount } = render(
        <ToolExecution {...baseProps} {...extra} />,
      )
      const results = await axe(container)
      expect(results).toHaveNoViolations()
      unmount()
    }
  })

  it("does not render a card for set_phase (hideCard)", () => {
    const { container } = render(
      <ToolExecution
        toolName="set_phase"
        toolInput={{ phase: "Analyze" }}
        output="Phase set to Analyze"
      />,
    )
    expect(container.firstChild).toBeNull()
  })
})
