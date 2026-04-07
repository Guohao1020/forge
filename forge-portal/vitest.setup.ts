import "@testing-library/jest-dom/vitest"
import { expect, afterEach } from "vitest"
import { cleanup } from "@testing-library/react"
import * as matchers from "vitest-axe/matchers"
import "vitest-axe/extend-expect"

// Extend vitest's expect with vitest-axe's toHaveNoViolations.
expect.extend(matchers)

// Clean up the DOM between tests (jsdom persists otherwise).
afterEach(() => {
  cleanup()
})
