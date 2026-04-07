import { describe, it, expect } from "vitest"
import { render, screen } from "@testing-library/react"
import { axe } from "vitest-axe"
import { ThinkingIndicator } from "./thinking-indicator"

describe("ThinkingIndicator", () => {
  it("renders default label", () => {
    render(<ThinkingIndicator />)
    expect(screen.getByText("Thinking…")).toBeInTheDocument()
  })

  it("renders custom label", () => {
    render(<ThinkingIndicator label="Running read_file" />)
    expect(screen.getByText("Running read_file…")).toBeInTheDocument()
  })

  it("has role=status and aria-live=polite for screen readers", () => {
    render(<ThinkingIndicator label="Analyzing" />)
    const node = screen.getByRole("status", { name: /analyzing/i })
    expect(node).toHaveAttribute("aria-live", "polite")
  })

  it("has no axe violations", async () => {
    const { container } = render(<ThinkingIndicator label="Reviewing code" />)
    const results = await axe(container)
    expect(results).toHaveNoViolations()
  })
})
