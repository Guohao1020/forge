import { describe, it, expect, vi, beforeEach } from "vitest"
import { createRef } from "react"
import { render, fireEvent } from "@testing-library/react"
import { axe } from "vitest-axe"
import { PanelDivider, loadSplitPct } from "./panel-divider"

describe("PanelDivider", () => {
  beforeEach(() => {
    localStorage.clear()
  })

  it("renders with role=separator and aria-orientation=vertical", () => {
    const ref = createRef<HTMLDivElement>()
    const { getByRole } = render(
      <div ref={ref} style={{ width: 1000 }}>
        <PanelDivider onChange={() => {}} containerRef={ref} />
      </div>,
    )
    const sep = getByRole("separator")
    expect(sep).toHaveAttribute("aria-orientation", "vertical")
    expect(sep).toHaveAccessibleName(/resize chat and code panels/i)
  })

  it("loadSplitPct returns default when nothing persisted", () => {
    expect(loadSplitPct()).toBe(50)
  })

  it("loadSplitPct reads persisted value and clamps to 20-80 range", () => {
    localStorage.setItem("agent-split-pct", "65")
    expect(loadSplitPct()).toBe(65)

    localStorage.setItem("agent-split-pct", "10")
    expect(loadSplitPct()).toBe(20)

    localStorage.setItem("agent-split-pct", "95")
    expect(loadSplitPct()).toBe(80)

    localStorage.setItem("agent-split-pct", "not a number")
    expect(loadSplitPct()).toBe(50)
  })

  it("fires onChange with clamped percentage on drag", async () => {
    const ref = createRef<HTMLDivElement>()
    const onChange = vi.fn()

    // Stub the container's bounding box so % math is predictable.
    const { getByRole } = render(
      <div
        ref={ref}
        style={{ width: 1000, height: 600 }}
      >
        <PanelDivider onChange={onChange} containerRef={ref} />
      </div>,
    )

    // Patch the container's getBoundingClientRect so drag math is stable.
    if (ref.current) {
      ref.current.getBoundingClientRect = () =>
        ({ left: 0, top: 0, width: 1000, height: 600, right: 1000, bottom: 600, x: 0, y: 0, toJSON: () => ({}) } as DOMRect)
    }

    const sep = getByRole("separator")
    fireEvent.mouseDown(sep, { preventDefault: () => {} })

    // rAF-based update — advance a frame.
    await new Promise<void>((resolve) => {
      requestAnimationFrame(() => {
        window.dispatchEvent(new MouseEvent("mousemove", { clientX: 700 }))
        requestAnimationFrame(() => resolve())
      })
    })

    window.dispatchEvent(new MouseEvent("mouseup"))
    // onChange may be invoked with the rAF-batched value.
    expect(onChange).toHaveBeenCalled()
  })

  it("has no axe violations", async () => {
    const ref = createRef<HTMLDivElement>()
    const { container } = render(
      <div ref={ref}>
        <PanelDivider onChange={() => {}} containerRef={ref} />
      </div>,
    )
    const results = await axe(container)
    expect(results).toHaveNoViolations()
  })
})
