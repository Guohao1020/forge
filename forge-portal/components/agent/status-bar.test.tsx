import { describe, it, expect } from "vitest"
import { render, screen } from "@testing-library/react"
import { axe } from "vitest-axe"
import { StatusBar } from "./status-bar"

describe("StatusBar", () => {
  const baseProps = {
    connStatus: "connected" as const,
    buildState: "idle" as const,
    tokenCount: 42180,
    costEstimate: 1.24,
  }

  it("renders with role=status and aria-live=polite", () => {
    render(<StatusBar {...baseProps} />)
    const footer = screen.getByRole("status", { name: /session status/i })
    expect(footer).toHaveAttribute("aria-live", "polite")
  })

  it("formats token count in thousands", () => {
    render(<StatusBar {...baseProps} />)
    expect(screen.getByText("42.2k tok")).toBeInTheDocument()
  })

  it("formats cost with two decimals above $0.01", () => {
    render(<StatusBar {...baseProps} />)
    expect(screen.getByText("$1.24")).toBeInTheDocument()
  })

  it("formats cost with four decimals below $0.01", () => {
    render(<StatusBar {...baseProps} costEstimate={0.0042} />)
    expect(screen.getByText("$0.0042")).toBeInTheDocument()
  })

  it("shows step indicator when currentStep and maxSteps are set", () => {
    render(<StatusBar {...baseProps} currentStep={4} maxSteps={7} />)
    expect(screen.getByText(/Step 4\/7/)).toBeInTheDocument()
  })

  it("shows error count plural", () => {
    render(<StatusBar {...baseProps} errorCount={3} />)
    expect(screen.getByText("3 errors")).toBeInTheDocument()
  })

  it("shows error count singular", () => {
    render(<StatusBar {...baseProps} errorCount={1} />)
    expect(screen.getByText("1 error")).toBeInTheDocument()
  })

  it("announces connection state to screen readers via sr-only", () => {
    render(<StatusBar {...baseProps} connStatus="reconnecting" />)
    expect(screen.getByText(/Connection: Reconnecting/i)).toBeInTheDocument()
  })

  it("has no axe violations", async () => {
    const { container } = render(
      <StatusBar
        {...baseProps}
        currentStep={2}
        maxSteps={5}
        branch="feat/auth"
        language="Java 17"
        errorCount={2}
        model="claude-sonnet-4"
      />,
    )
    const results = await axe(container)
    expect(results).toHaveNoViolations()
  })
})
