import { describe, it, expect } from "vitest"
import { render, screen } from "@testing-library/react"
import { axe } from "vitest-axe"
import { BuildCard } from "./build-card"

describe("BuildCard", () => {
  it("renders building state with command and label", () => {
    render(<BuildCard status="building" command="mvn compile -q" />)
    expect(screen.getByText("Building...")).toBeInTheDocument()
    expect(screen.getByText("mvn compile -q")).toBeInTheDocument()
  })

  it("renders passed state", () => {
    render(<BuildCard status="passed" command="npm run build" durationMs={3420} />)
    expect(screen.getByText("Build Passed")).toBeInTheDocument()
    expect(screen.getByText("3.4s")).toBeInTheDocument()
  })

  it("renders failed state expanded by default with output visible", () => {
    render(
      <BuildCard
        status="failed"
        command="go build ./..."
        output="cannot find package"
        durationMs={850}
      />,
    )
    expect(screen.getByText("Build Failed")).toBeInTheDocument()
    expect(screen.getByText("cannot find package")).toBeInTheDocument()
    expect(screen.getByText("850ms")).toBeInTheDocument()
  })

  it("has no axe violations across all states", async () => {
    for (const status of ["building", "passed", "failed"] as const) {
      const { container, unmount } = render(
        <BuildCard status={status} command="test" output="x" durationMs={100} />,
      )
      const results = await axe(container)
      expect(results).toHaveNoViolations()
      unmount()
    }
  })
})
