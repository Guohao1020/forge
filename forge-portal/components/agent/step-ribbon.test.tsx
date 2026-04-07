import { describe, it, expect } from "vitest"
import { render, screen } from "@testing-library/react"
import { axe } from "vitest-axe"
import { StepRibbon, type Step } from "./step-ribbon"

describe("StepRibbon", () => {
  const fixture: Step[] = [
    { id: "analyze", label: "Analyze", status: "done" },
    { id: "plan", label: "Plan", status: "done" },
    { id: "build", label: "Build", status: "active", cycle: 2, maxCycles: 3 },
    { id: "test", label: "Test", status: "pending" },
  ]

  it("renders an empty 'Ready' state when no steps are provided", () => {
    render(<StepRibbon steps={[]} />)
    expect(screen.getByText("Ready")).toBeInTheDocument()
    expect(screen.getByRole("navigation", { name: /task progress/i })).toBeInTheDocument()
  })

  it("renders each step with its label and connectors between", () => {
    render(<StepRibbon steps={fixture} />)
    expect(screen.getByText("Analyze")).toBeInTheDocument()
    expect(screen.getByText("Plan")).toBeInTheDocument()
    expect(screen.getByText("Build")).toBeInTheDocument()
    expect(screen.getByText("Test")).toBeInTheDocument()
  })

  it("marks the active step with aria-current", () => {
    render(<StepRibbon steps={fixture} />)
    const active = screen.getByText("Build").closest("[aria-current='step']")
    expect(active).not.toBeNull()
  })

  it("renders the cycle counter when cycle/maxCycles are set", () => {
    render(<StepRibbon steps={fixture} />)
    expect(screen.getByText("(2/3)")).toBeInTheDocument()
  })

  it("has no axe violations", async () => {
    const { container } = render(<StepRibbon steps={fixture} />)
    const results = await axe(container)
    expect(results).toHaveNoViolations()
  })

  it("empty state has no axe violations", async () => {
    const { container } = render(<StepRibbon steps={[]} />)
    const results = await axe(container)
    expect(results).toHaveNoViolations()
  })
})
