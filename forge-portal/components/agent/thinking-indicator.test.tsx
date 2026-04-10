import { describe, it, expect } from "vitest"
import { render, screen } from "@testing-library/react"
import { axe } from "vitest-axe"
import { ThinkingIndicator } from "./thinking-indicator"

describe("ThinkingIndicator", () => {
  it("renders the label text", () => {
    render(<ThinkingIndicator label="Running go build" />)
    expect(screen.getByText("Running go build")).toBeInTheDocument()
  })

  it("has role=status and aria-live=polite for screen readers", () => {
    render(<ThinkingIndicator label="Analyzing" />)
    const node = screen.getByRole("status")
    expect(node).toHaveAttribute("aria-live", "polite")
  })

  it("renders the pulsing dot animation", () => {
    const { container } = render(<ThinkingIndicator label="Working" />)
    const pings = container.querySelectorAll(".animate-ping")
    expect(pings.length).toBe(1)
  })

  it("has no axe violations", async () => {
    const { container } = render(<ThinkingIndicator label="Reviewing code" />)
    const results = await axe(container)
    expect(results).toHaveNoViolations()
  })
})
