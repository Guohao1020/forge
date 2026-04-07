// Augment Vitest 4's Matchers interface with vitest-axe's `toHaveNoViolations`.
// vitest-axe's shipped type augmentation targets the legacy Vi.Assertion
// namespace which was removed in Vitest 4+; we re-declare the matcher
// against the current @vitest/expect module.

import "vitest"

declare module "vitest" {
  interface Assertion<T = unknown> {
    toHaveNoViolations(): T
  }
  interface AsymmetricMatchersContaining {
    toHaveNoViolations(): unknown
  }
}

export {}
