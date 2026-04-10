import { render, screen } from "@testing-library/react"
import { describe, expect, it } from "vitest"

import {
  PHASES,
  StepRibbon,
  initialSteps,
  updateStepsForPhase,
  type Step,
} from "./step-ribbon"

describe("StepRibbon", () => {
  it("renders an empty state when steps array is empty", () => {
    render(<StepRibbon steps={[]} />)
    expect(screen.getByText("Ready")).toBeInTheDocument()
  })

  it("renders all 7 phases from initialSteps()", () => {
    render(<StepRibbon steps={initialSteps()} />)
    for (const phase of PHASES) {
      expect(screen.getByText(phase.label)).toBeInTheDocument()
    }
  })

  it("shows the active phase with aria-current='step'", () => {
    const steps: Step[] = initialSteps().map((s) =>
      s.id === "Generate" ? { ...s, status: "active" } : s,
    )
    render(<StepRibbon steps={steps} />)
    const generate = screen.getByLabelText(/Generate: active/)
    expect(generate).toHaveAttribute("aria-current", "step")
  })

  it("shows done phases without aria-current", () => {
    const steps: Step[] = initialSteps().map((s) =>
      s.id === "Analyze" ? { ...s, status: "done" } : s,
    )
    render(<StepRibbon steps={steps} />)
    const analyze = screen.getByLabelText(/Analyze: done/)
    expect(analyze).not.toHaveAttribute("aria-current")
  })

  it("pending phases show their 1-indexed number", () => {
    render(<StepRibbon steps={initialSteps()} />)
    // Analyze is pending, should show "1"
    expect(screen.getByLabelText(/Analyze: pending/)).toHaveTextContent("1")
    // Deploy is pending, should show "7"
    expect(screen.getByLabelText(/Deploy: pending/)).toHaveTextContent("7")
  })

  it("applies the transition-colors utility so state changes animate", () => {
    const steps: Step[] = initialSteps().map((s) =>
      s.id === "Build" ? { ...s, status: "active" } : s,
    )
    render(<StepRibbon steps={steps} />)
    const build = screen.getByLabelText(/Build: active/)
    expect(build.className).toContain("transition-colors")
  })

  it("does not render cycle/maxCycles indicators (pair_pipeline carryover)", () => {
    // A Step type now has no cycle/maxCycles fields. This test is a
    // sentinel: if someone re-adds those fields, StepRibbon should not
    // accept them without compilation error.
    const steps = initialSteps()
    render(<StepRibbon steps={steps} />)
    expect(screen.queryByText(/1\/3/)).not.toBeInTheDocument()
    expect(screen.queryByText(/cycle/i)).not.toBeInTheDocument()
  })
})

describe("updateStepsForPhase", () => {
  it("marks the target phase active and leaves others pending on first call", () => {
    const steps = initialSteps()
    const next = updateStepsForPhase(steps, "Analyze")
    expect(next.find((s) => s.id === "Analyze")?.status).toBe("active")
    expect(next.find((s) => s.id === "Plan")?.status).toBe("pending")
    expect(next.find((s) => s.id === "Deploy")?.status).toBe("pending")
  })

  it("transitions the previous active phase to done", () => {
    const steps = updateStepsForPhase(initialSteps(), "Analyze")
    const next = updateStepsForPhase(steps, "Generate")
    expect(next.find((s) => s.id === "Analyze")?.status).toBe("done")
    expect(next.find((s) => s.id === "Plan")?.status).toBe("pending")
    expect(next.find((s) => s.id === "Generate")?.status).toBe("active")
  })

  it("keeps already-done phases done on backward movement", () => {
    // Sequence: Analyze -> Generate -> Build -> back to Generate
    let steps = updateStepsForPhase(initialSteps(), "Analyze")
    steps = updateStepsForPhase(steps, "Generate")
    steps = updateStepsForPhase(steps, "Build")
    steps = updateStepsForPhase(steps, "Generate")

    expect(steps.find((s) => s.id === "Analyze")?.status).toBe("done")
    expect(steps.find((s) => s.id === "Generate")?.status).toBe("active")
    expect(steps.find((s) => s.id === "Build")?.status).toBe("done")
  })

  it("handles an unknown phase name by leaving everything else alone", () => {
    const steps = updateStepsForPhase(initialSteps(), "Unknown")
    // No phase is active because 'Unknown' doesn't match any id
    expect(steps.filter((s) => s.status === "active")).toHaveLength(0)
  })
})
