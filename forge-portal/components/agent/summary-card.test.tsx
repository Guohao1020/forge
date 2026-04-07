import { describe, it, expect } from "vitest"
import { render, screen } from "@testing-library/react"
import { axe } from "vitest-axe"
import { SummaryCard } from "./summary-card"

describe("SummaryCard", () => {
  const baseProps = {
    filesCreated: 3,
    filesModified: 1,
    buildStatus: "passed" as const,
    durationMs: 4200,
    tokensTotal: 42180,
    costUsd: 1.24,
  }

  it("renders as a region with Session summary label", () => {
    render(<SummaryCard {...baseProps} />)
    expect(screen.getByRole("region", { name: /session summary/i })).toBeInTheDocument()
  })

  it("shows Done header for passed builds", () => {
    render(<SummaryCard {...baseProps} />)
    expect(screen.getByText("Done")).toBeInTheDocument()
  })

  it("shows Failed header for failed builds", () => {
    render(<SummaryCard {...baseProps} buildStatus="failed" />)
    expect(screen.getByText("Failed")).toBeInTheDocument()
  })

  it("renders the stats grid with all six values", () => {
    render(<SummaryCard {...baseProps} />)
    expect(screen.getByText("3")).toBeInTheDocument() // files created
    expect(screen.getByText("1")).toBeInTheDocument() // files modified
    expect(screen.getAllByText(/passed/i).length).toBeGreaterThan(0)
    expect(screen.getAllByText("4.2s").length).toBeGreaterThan(0)
    expect(screen.getByText("42.2k")).toBeInTheDocument()
    expect(screen.getByText("$1.24")).toBeInTheDocument()
  })

  it("formats sub-$0.01 costs with four decimals", () => {
    render(<SummaryCard {...baseProps} costUsd={0.0034} />)
    expect(screen.getByText("$0.0034")).toBeInTheDocument()
  })

  it("shows n/a for skipped build status", () => {
    render(<SummaryCard {...baseProps} buildStatus="skipped" />)
    expect(screen.getByText("n/a")).toBeInTheDocument()
  })

  it("has no axe violations across all states", async () => {
    for (const buildStatus of ["passed", "failed", "skipped"] as const) {
      const { container, unmount } = render(
        <SummaryCard {...baseProps} buildStatus={buildStatus} />,
      )
      const results = await axe(container)
      expect(results).toHaveNoViolations()
      unmount()
    }
  })
})
