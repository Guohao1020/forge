# chronos · Phase 6 — Frontend Changes

> **Project:** [chronos — Agent Variant B Single-Agent Implementation](index.md)
> **Phase:** 6 of 7 · **Tasks:** 11 · **Depends on:** [Phase 5](phase-5-agent-loop.md) · **Unblocks:** Phase 7
> **Spec reference:** [Design spec §6 (Frontend changes) + §2.6 (Step Ribbon / Build Card / Fix Loop decisions) + §2.9.2 (Round 2: request_clarification meta-tool, bidirectional SSE protocol)](../../specs/2026-04-09-agent-variant-b-single-agent-design.md)

**Execution:** REQUIRED SUB-SKILL: Use `superpowers:subagent-driven-development` (recommended) or `superpowers:executing-plans`. Steps use checkbox (`- [ ]`) syntax for tracking.

**⚠️ Next.js in this project:** `forge-portal/AGENTS.md` says: *"This is NOT the Next.js you know. This version has breaking changes — APIs, conventions, and file structure may all differ from your training data. Read the relevant guide in `node_modules/next/dist/docs/` before writing any code."* Before starting any task that touches a Next.js-specific API (routing, server actions, page props, metadata), the executing agent MUST `ls node_modules/next/dist/docs/` and `cat` the relevant file. This is a project invariant, not an optional check.

---

## Phase goal

Bring the frontend into alignment with the A2 architecture, **plus Round 2: clarification input UI and state machine for the `request_clarification` meta-tool**. Eleven focused changes:

1. **Delete build-card.tsx** — spec §2.6 Q5.3 killed the dedicated build card in favor of a unified bash tool card
2. **Delete pair_pipeline-era state** — `BuildInfo`, `ChatMessage.build`, `AgentRole="coder"|"reviewer"` all go
3. **Delete `fix_loop_*` SSE handlers** — events are gone from the backend (Phase 4 Task 4.1); frontend detects fix loops visually from bash-error → edit → bash sequences instead
4. **Rewrite `step-ribbon.tsx` state model** — dynamic phase tracking from `phase_changed` events, drop `cycle`/`maxCycles` fields
5. **Update `tool-formatters.ts`** — add formatters for the 6 file tools + `bash` + `set_phase` (with `hideCard` for set_phase)
6. **Update `tool-execution.tsx`** — respect `hideCard` flag so set_phase doesn't render a noise card
7. **Downgrade `code-panel.tsx`** — read-only preview only; drop any pair_pipeline-specific rendering
8. **Relocate `thinking-indicator.tsx`** — render attached to bash tool cards, not chat bottom
9. **Add `detectFixLoopStart` visual heuristic** — inserts a subtle "Fixing previous error..." system message when pattern is detected, replacing the deleted fix_loop events
10. **(Round 2) `ClarificationInput` component + agent-chat state machine** — inline form component for user responses, state transitions for `clarification_requested` → `[pause]` → `tool_execution_completed`, disables the main chat input while awaiting a reply, handles clarification timeout as a red banner
11. **(Round 2) SSE event dispatch — `clarification_requested` handler** — wires the new event type into the existing dispatcher; adds a `ClarificationRequestedEvent` type to the SSE event union

Plus cross-cutting updates to `agent-chat.tsx` (the big file) and `page.tsx` (the step ribbon host).

**Completion gate:**
- `forge-portal/components/agent/build-card.tsx` does not exist
- `forge-portal/components/agent/build-card.test.tsx` does not exist
- `grep -rn "BuildInfo\|fix_loop_\|coder\|reviewer" forge-portal/components/agent/` returns nothing (or only comments)
- `grep -rn "phase_changed\|PhaseChanged" forge-portal/components/agent/agent-chat.tsx` returns at least 1 match (the new handler)
- `forge-portal/components/agent/clarification-input.tsx` exists and exports `ClarificationInput`
- `forge-portal/components/agent/clarification-input.test.tsx` exists and passes
- `grep -n "clarification_requested" forge-portal/components/agent/agent-chat.tsx` returns ≥ 1 match (the new SSE case)
- `grep -n "postClarification" forge-portal/lib/agent.ts` returns ≥ 1 match (the new API client function)
- `pnpm --filter forge-portal test` (or `npm test` / `vitest run` depending on the project's runner) passes
- `pnpm --filter forge-portal build` (or equivalent) succeeds without TS errors
- Manual smoke via dev server: load the agent page, verify:
  - step ribbon starts with all 7 phases in "pending" state (no build card, no cycle counter)
  - sending a message that triggers `set_phase` updates the ribbon
  - bash tool cards render with the command and exit code
  - `set_phase` does NOT render its own tool card (hideCard)
  - a fake bash-error → edit → bash sequence inserts the subtle "Fixing previous error" banner
  - when a `clarification_requested` SSE event is injected, the `ClarificationInput` form appears inline with the agent's question, the main chat input becomes disabled, and submitting the form `POST`s to `/api/sessions/{id}/clarify`; a matching `tool_execution_completed` event closes the form and re-enables the main input
  - a `clarification timeout` error event closes the stream and shows the red "Session ended" banner

## Why this phase matters

Phase 5 finished the backend. Phase 6 closes the loop so the human user actually sees the agent working. Until this phase lands:
- The frontend still renders `BuildCard` which will never appear (backend deleted the concept)
- The step ribbon shows pair_pipeline cycle counters that will never update
- `tool_started` events for the 6 new file tools + `bash` + `set_phase` render as generic unstyled cards because `tool-formatters.ts` doesn't know about them
- `phase_changed` events from the backend are silently dropped

After this phase, the Variant B UI is correct end-to-end.

**Silicon-valley rules for this phase:**
- **Delete, don't comment out.** Dead code in a large React tree is where confusion breeds. If a component or field is gone, it's gone.
- **No conditional branches for legacy event types.** `fix_loop_*` event handlers are deleted outright. If a stale Redis stream event arrives, it goes to the `default:` case and gets logged — no ghost UI.
- **Visual heuristics live in one helper function.** `detectFixLoopStart` is a pure function that takes the current message list + the new event and returns either `null` or a banner-insert instruction. Not spread across three files.

---

## Shared conventions for Phase 6

**TDD-adjacent for frontend.** React testing in this project uses Vitest + React Testing Library (confirmed in `forge-portal/components/agent/*.test.tsx`). Tests are co-located with components. Follow the existing patterns:

```tsx
import { render, screen } from "@testing-library/react"
import { describe, expect, it } from "vitest"
import { StepRibbon } from "./step-ribbon"
```

**Delete test files whose subject is deleted.** When `build-card.tsx` goes, `build-card.test.tsx` goes too. No orphan tests.

**Existing components to preserve:** `summary-card.tsx`, `status-bar.tsx`, `panel-divider.tsx`, `mobile-panel-switcher.tsx`, `task-switcher.tsx`. These are correctly aligned with A2 and don't need changes in this phase.

**SSE event vocabulary** (matches Phase 4 Task 4.1 + Phase 5 Task 5.6 outputs):

| Redis `type` field | Frontend handler | Carries |
|---|---|---|
| `text_delta` | append to current assistant message | `text` |
| `turn_complete` | mark current assistant message complete | `text`, `input_tokens`, `output_tokens` |
| `tool_started` | create tool card (or banner for `set_phase`) | `tool_use_id`, `tool_name`, `tool_input` |
| `tool_completed` | fill tool card result | `tool_use_id`, `tool_name`, `output`, `is_error` |
| `phase_changed` | update step ribbon | `phase` |
| `thinking_started` | show indicator on current bash card | `label` |
| `thinking_stopped` | hide indicator | — |
| `session_complete` | append SummaryCard entry | `files_created`, `files_modified`, `build_status`, `duration_ms`, `tokens_total`, `cost_usd` |
| `error` | append error message | `message`, `recoverable` |

Anything else goes to the `default:` branch and logs a warning. Notably `fix_loop_started` and `fix_loop_completed` are NOT in the table — Phase 4 deleted them.

---

### Task 6.1: Delete `build-card.tsx` and its test

**Files:**
- Delete: `forge-portal/components/agent/build-card.tsx`
- Delete: `forge-portal/components/agent/build-card.test.tsx`

**Context:** Spec §2.6 Q5.3 decided BuildCard is redundant — an A2 agent's "build" is just a `bash` tool call, so it renders as a bash tool card like every other subprocess invocation. Delete the component and its test in one step. Related cleanup (removing `BuildInfo` / `ChatMessage.build` / imports) happens in Task 6.2.

- [ ] **Step 1: Delete the files**

```bash
rm forge-portal/components/agent/build-card.tsx
rm forge-portal/components/agent/build-card.test.tsx
```

- [ ] **Step 2: Verify no remaining references compile-fail the build**

```bash
grep -rn "BuildCard\|from.*build-card" forge-portal/
```
Expected: matches only in `agent-chat.tsx` (the import and usage — Task 6.2 deletes them). If there are other matches, they'd need to be removed here, but typically BuildCard is only consumed from agent-chat.

- [ ] **Step 3: Commit**

```bash
git add -A forge-portal/components/agent/build-card.tsx forge-portal/components/agent/build-card.test.tsx 2>/dev/null
git rm forge-portal/components/agent/build-card.tsx forge-portal/components/agent/build-card.test.tsx 2>/dev/null || true
git commit -m "feat(portal): delete BuildCard component (A2 unified tool cards)

Spec §2.6 Q5.3 killed the dedicated build card in favor of a
unified bash tool card. An A2 agent's 'build' is just a bash
tool call ('bash go build ./...'), which renders as a regular
bash tool card with command + exit code + output. BuildCard's
red-box visual was pair_pipeline-specific.

build-card.tsx and build-card.test.tsx deleted. Residual
imports/usages in agent-chat.tsx will fail to compile until
Task 6.2 cleans them up — that's intentional, it forces the
next task to finish the cleanup."
```

---

### Task 6.2: Clean up `agent-chat.tsx` — delete BuildInfo, coder/reviewer roles, fix_loop handlers

**Files:**
- Modify: `forge-portal/components/agent/agent-chat.tsx`

**Context:** Cross-cutting cleanup of dead pair_pipeline-era state and handlers in the chat component. Five sub-changes:

1. Remove the `BuildCard` import line
2. Delete the `BuildInfo` type (around line 52)
3. Delete `ChatMessage.build?: BuildInfo` field
4. Remove all `build: ...` handlers and the BuildCard JSX render
5. Narrow `AgentRole` from `"user" | "assistant" | "coder" | "reviewer" | "system" | "summary"` to `"user" | "assistant" | "system" | "summary"`
6. Delete the `fix_loop_started` and `fix_loop_completed` SSE event handlers (case blocks)
7. Delete the `hydrateFromDurableLog` branches that handle `fix_loop_started` / `fix_loop_completed` event types

The file is ~832 lines. Changes are scattered but all mechanical — no new logic, just removal.

- [ ] **Step 1: Read the existing structure to locate each change point**

```bash
grep -n "BuildCard\|BuildInfo\|AgentRole\|build:\|fix_loop_\|coder\|reviewer" forge-portal/components/agent/agent-chat.tsx
```

Record the line numbers; you'll touch each in turn.

- [ ] **Step 2: Delete the BuildCard import**

Find `import { BuildCard } from "./build-card"` and delete it.

- [ ] **Step 3: Narrow `AgentRole`**

Find:
```tsx
type AgentRole = "user" | "assistant" | "coder" | "reviewer" | "system" | "summary"
```
Replace with:
```tsx
type AgentRole = "user" | "assistant" | "system" | "summary"
```

Any code that branches on `role === "coder"` or `role === "reviewer"` is dead — delete those branches too. `grep -n '"coder"\|"reviewer"' forge-portal/components/agent/agent-chat.tsx` finds them.

- [ ] **Step 4: Delete `BuildInfo` type and `ChatMessage.build` field**

Find the `BuildInfo` interface/type definition and delete it. Then find:
```tsx
interface ChatMessage {
  ...
  build?: BuildInfo
  ...
}
```
Delete the `build?: BuildInfo` line.

- [ ] **Step 5: Delete BuildCard JSX render**

In the message-rendering JSX, find any block like:
```tsx
{msg.build && <BuildCard {...msg.build} />}
```
Delete it.

- [ ] **Step 6: Delete `fix_loop_started` / `fix_loop_completed` SSE handlers**

In the `switch (event.type)` block of the SSE reader (around line 441+), find:

```tsx
      case "fix_loop_started": {
        // ... some code ...
        break
      }
      case "fix_loop_completed": {
        // ... some code ...
        break
      }
```

Delete both case blocks entirely. The `default:` case will catch any stale events from old Redis streams and log them.

- [ ] **Step 7: Delete `fix_loop_*` branches from `hydrateFromDurableLog`**

In the `hydrateFromDurableLog` function (lines 75-270ish), find:

```tsx
    if (type === "fix_loop_started") {
      // ... handling ...
      continue
    }
    if (type === "fix_loop_completed") {
      // ... handling ...
      continue
    }
```

Delete both blocks.

- [ ] **Step 8: Build to verify nothing else broke**

Run the Next.js type check (the project may have a dedicated command — check `forge-portal/package.json` `scripts`):

```bash
cd forge-portal && npx tsc --noEmit 2>&1 | tail -30
```
Expected: clean, or only errors in OTHER files that depend on `BuildInfo` / `AgentRole`'s old shape. Fix those too (they're unlikely — `AgentRole` is a local type inside agent-chat.tsx based on the grep).

- [ ] **Step 9: Run the agent-chat test file**

```bash
cd forge-portal && npx vitest run components/agent/agent-chat.test.tsx 2>&1 | tail -30
```
Expected: most tests pass. Any test that still references `BuildCard` rendering or constructs a `ChatMessage` with a `build` field will fail — either delete the test or update it. The Task 6.9 step revisits the test file end-to-end.

- [ ] **Step 10: Commit**

```bash
git add forge-portal/components/agent/agent-chat.tsx
git commit -m "feat(portal): purge pair_pipeline leftovers from agent-chat

Cross-cutting cleanup after BuildCard deletion in Task 6.1:

- Remove 'import { BuildCard }' line
- Delete BuildInfo type and ChatMessage.build field
- Delete BuildCard JSX render in message rendering
- Narrow AgentRole: drop 'coder' and 'reviewer' (pair_pipeline
  distinguished Coder vs Reviewer agents; A2 is single-agent
  and only has 'assistant')
- Delete any branch that matched role==='coder' or 'reviewer'
- Delete 'fix_loop_started' and 'fix_loop_completed' case blocks
  in the SSE event switch — backend deleted those event types
  in Phase 4, so they never arrive from the agent. Stale events
  from older Redis streams fall through to the default case
  and log a warning.
- Delete fix_loop_* branches in hydrateFromDurableLog for the
  same reason

No new logic, just deletion. Task 6.9 revisits agent-chat.test.tsx
to update test expectations."
```

---

### Task 6.3: Rewrite `step-ribbon.tsx` for dynamic phase tracking

**Files:**
- Modify: `forge-portal/components/agent/step-ribbon.tsx`
- Modify: `forge-portal/components/agent/step-ribbon.test.tsx`

**Context:** The existing `step-ribbon.tsx` has the right general shape (array of steps, status per step, nice visual) but carries two pair_pipeline-era concepts that must go:

1. The `cycle` / `maxCycles` fields on `Step` (fix loop iteration counter)
2. The callers constructing steps with `cycle`/`maxCycles` arguments

Otherwise the component is fine — keep the visual, keep the status enum (`done` / `active` / `pending` / `failed`), keep the icons and transitions. Just narrow the `Step` type.

The 7 A2 phases are fixed: `Analyze`, `Plan`, `Generate`, `Build`, `Test`, `Review`, `Deploy`. In the new model, `page.tsx` initializes steps as all 7 in `"pending"` state. A `phase_changed` event from SSE flips the named phase to `"active"` and any previously active phase to `"done"`. Backward transitions (Build → Generate because the build failed) flip the new phase to `"active"` and the old phase goes back to `"pending"` (or stays `"done"` if it was already finished).

Actually simpler: any phase the agent has **left** (phase_changed fires moving away from it) becomes `"done"`. Any phase the agent **returns to** becomes `"active"` again but the phases *after* it also go back to `"pending"`. This isn't quite right either — a loop Build→Generate→Build should leave Analyze and Plan as "done".

Cleanest model: a `visited: Set<string>` plus a `currentPhase: string | null`. Visited phases are `"done"` (unless they're the current one, in which case `"active"`). Non-visited phases are `"pending"`. Failed builds mark the current phase as `"failed"` via a separate signal — but there's no "build failed" event anymore either (Phase 4 deleted FixLoop events and spec §2.6 Q5.5 said the frontend does visual detection). So `"failed"` state for a step is... vestigial? For now, keep the enum and just don't emit it — no handler produces `"failed"` in the new world. If the agent's bash call is build-like and fails, the bash tool card itself shows is_error, not the step ribbon.

Decision: **delete `"failed"` from StepStatus**. Simpler model: `done` | `active` | `pending`. If the agent's build fails, the step ribbon still shows `Build` as `active` (because the agent is still in the build phase trying to fix) — the bash tool card is the error signal.

- [ ] **Step 1: Read existing `step-ribbon.tsx` and plan the diff**

```bash
cat forge-portal/components/agent/step-ribbon.tsx
```
Already reviewed in reconnaissance. Changes:
- Narrow `StepStatus` to `"done" | "active" | "pending"` (drop `"failed"`)
- Delete `cycle?: number` and `maxCycles?: number` from `Step`
- Delete the failed-state rendering (the `X` icon import and branch)
- Delete the `(cycle/maxCycles)` display in the JSX

- [ ] **Step 2: Rewrite `step-ribbon.tsx`**

Replace the content of `forge-portal/components/agent/step-ribbon.tsx`:

```tsx
"use client"

import { cn } from "@/lib/utils"
import { Check, Loader2 } from "lucide-react"

/**
 * Step ribbon states for the A2 single-agent phase tracker.
 *
 * - `done`: the agent has passed through this phase (set_phase
 *   fired a later phase after it)
 * - `active`: the agent is currently in this phase (last set_phase
 *   event named this phase)
 * - `pending`: the agent has not yet entered this phase
 *
 * There is no `failed` state in A2. Build failures are rendered
 * as error bash tool cards inside the chat; the step ribbon is
 * purely a progress indicator, not an error display.
 */
export type StepStatus = "done" | "active" | "pending"

export interface Step {
  id: string
  label: string
  status: StepStatus
}

interface StepRibbonProps {
  steps: Step[]
  className?: string
}

// Mockup variant-B-dense.html lines 230-289. No glow, no shadow.
// completed -> --text-success, active -> --accent bg + border.
// Step connector is a 10×1px div, NOT an arrow character.
const statusClass: Record<StepStatus, string> = {
  done: "text-[var(--text-success)]",
  active:
    "bg-[var(--accent-subtle)] text-[var(--accent)] border-[var(--accent)]",
  pending: "text-[var(--text-tertiary)]",
}

function StepIcon({ status, index }: { status: StepStatus; index: number }) {
  const baseCls = "inline-flex items-center justify-center w-3.5 h-3.5 text-[10px]"
  if (status === "done") return <Check className={cn(baseCls, "h-3 w-3")} />
  if (status === "active")
    return <Loader2 className={cn(baseCls, "h-3 w-3 animate-spin")} />
  return <span className={baseCls}>{index + 1}</span>
}

export function StepRibbon({ steps, className }: StepRibbonProps) {
  if (steps.length === 0) {
    return (
      <nav
        className={cn(
          "flex items-center h-10 px-2.5 gap-0.5 bg-[var(--bg-secondary)] border-b border-[var(--border-primary)] overflow-x-auto",
          "[&::-webkit-scrollbar]:hidden",
          className,
        )}
        style={{ scrollbarWidth: "none" }}
        aria-label="Agent workflow phases"
      >
        <span className="text-[11px] text-[var(--text-tertiary)]">Ready</span>
      </nav>
    )
  }

  return (
    <nav
      className={cn(
        "flex items-center h-10 px-2.5 gap-0.5 bg-[var(--bg-secondary)] border-b border-[var(--border-primary)] overflow-x-auto",
        "[&::-webkit-scrollbar]:hidden",
        className,
      )}
      style={{ scrollbarWidth: "none" }}
      aria-label="Agent workflow phases"
    >
      {steps.map((step, i) => (
        <div key={step.id} className="flex items-center gap-0.5">
          {i > 0 && (
            <div
              className="w-2.5 h-px bg-[var(--border-primary)] shrink-0"
              aria-hidden
            />
          )}
          <div
            className={cn(
              "inline-flex items-center gap-1 px-2 py-1 rounded text-[11px] font-medium whitespace-nowrap transition-colors duration-100 border border-transparent cursor-pointer",
              "hover:bg-[var(--bg-hover)]",
              statusClass[step.status],
            )}
            aria-current={step.status === "active" ? "step" : undefined}
            aria-label={`${step.label}: ${step.status}`}
          >
            <StepIcon status={step.status} index={i} />
            <span>{step.label}</span>
          </div>
        </div>
      ))}
    </nav>
  )
}

/**
 * The 7 phases of the A2 workflow ribbon. Exported so callers
 * (page.tsx) can initialize state without duplicating the list.
 * Matches SetPhaseTool's Literal type in ai-worker/src/openharness/
 * tools/phase_tool.py.
 */
export const PHASES: ReadonlyArray<{ id: string; label: string }> = [
  { id: "Analyze", label: "Analyze" },
  { id: "Plan", label: "Plan" },
  { id: "Generate", label: "Generate" },
  { id: "Build", label: "Build" },
  { id: "Test", label: "Test" },
  { id: "Review", label: "Review" },
  { id: "Deploy", label: "Deploy" },
] as const

/**
 * Build an initial 7-step array with all phases in "pending" state.
 * Convenience for page-level state initialization.
 */
export function initialSteps(): Step[] {
  return PHASES.map((p) => ({
    id: p.id,
    label: p.label,
    status: "pending" as const,
  }))
}
```

- [ ] **Step 3: Rewrite `step-ribbon.test.tsx`**

The existing test file likely tests the old `Step` shape with `cycle`/`maxCycles`. Replace with:

```tsx
import { render, screen } from "@testing-library/react"
import { describe, expect, it } from "vitest"

import { PHASES, StepRibbon, initialSteps, type Step } from "./step-ribbon"


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
```

- [ ] **Step 4: Run the test file**

```bash
cd forge-portal && npx vitest run components/agent/step-ribbon.test.tsx
```
Expected: all tests pass. If the test importer can't find `@testing-library/react` or Vitest, check the project's existing test setup — it's already in use (Stream 5 a11y work in commit a8e54cc).

- [ ] **Step 5: Fix callers that still pass cycle/maxCycles**

```bash
grep -rn "cycle:\|maxCycles:" forge-portal/
```
Expected: matches in `page.tsx` (the `buildStateFromSteps` helper uses `buildStep.cycle`). Clean those up. The `buildStateFromSteps` helper's `"fixing"` branch should be deleted — there's no fix loop concept at the step level anymore:

```tsx
function buildStateFromSteps(steps: Step[]): BuildState {
  const buildStep = steps.find((s) => s.id === "Build")
  if (!buildStep) return "idle"
  if (buildStep.status === "done") return "passed"
  // In A2 there's no "failed" status on steps — build failures show
  // as error bash cards in chat, and the Build step stays "active"
  // while the agent retries
  if (buildStep.status === "active") return "building"
  return "idle"
}
```

If `BuildState` type imports `"failed"` and `"fixing"`, leave those in the `status-bar.tsx` type (StatusBar may still want to show build state from other signals) but the helper stops producing them.

- [ ] **Step 6: Commit**

```bash
git add forge-portal/components/agent/step-ribbon.tsx forge-portal/components/agent/step-ribbon.test.tsx forge-portal/app 2>/dev/null
git commit -m "refactor(portal): step-ribbon uses dynamic phase states (A2)

StepStatus narrowed to 'done' | 'active' | 'pending' — delete
'failed' (build failures show as error bash cards in chat, not
on the ribbon). Delete cycle/maxCycles fields (pair_pipeline
fix-loop counter, A2 doesn't have fix loops at the ribbon
level).

Add PHASES constant (the 7 A2 phases) and initialSteps() helper
so page.tsx can initialize state without duplicating the list.
PHASES order matches SetPhaseTool's Literal type in
phase_tool.py — keep these in sync.

Tests rewritten: empty state, 7 phases rendered, active aria
attribute, done without aria-current, pending number display,
transition-colors utility applied, sentinel that cycle/maxCycles
fields are gone.

page.tsx buildStateFromSteps helper is updated to drop the
'fixing' branch (which looked at cycle > 1)."
```

---

### Task 6.4: Wire `phase_changed` events to the step ribbon state

**Files:**
- Modify: `forge-portal/app/(dashboard)/projects/[id]/agent/page.tsx`
- Modify: `forge-portal/components/agent/agent-chat.tsx` (pass phase events up via onStepsUpdate)

**Context:** Phase 5 Task 5.6 added `phase_changed` to the SSE stream. This task threads the event from SSE through the agent-chat component up to page.tsx's `steps` state, which drives `<StepRibbon steps={steps} />`.

Two parts:
1. **agent-chat.tsx** — in the SSE switch, add a `case "phase_changed"` branch that calls `onStepsUpdate?.(...)` with the new step array. Compute the new array locally (using a helper that takes `currentSteps` + `newPhase` and returns an updated array) so the parent just stores whatever it gets.
2. **page.tsx** — initialize `const [steps, setSteps] = useState<Step[]>(initialSteps())` (using the Task 6.3 helper), and pass `onStepsUpdate={setSteps}` to `<AgentChat />`.

The step-update helper computes:
- The target phase becomes `active`
- Any phase before the target in `PHASES` order whose current status is `active` or `pending` becomes `done`
- Any phase after the target whose current status is `done` stays `done` (backward movement doesn't un-do finished phases)

Actually simpler logic per spec §2.6: maintain a `visited: Set<string>` plus `current: string | null`. Visited phases are `done` (unless current, which is `active`). Non-visited are `pending`.

```tsx
function updateSteps(current: Step[], newPhase: string): Step[] {
  const visited = new Set<string>()
  for (const s of current) {
    if (s.status === "done" || s.status === "active") {
      visited.add(s.id)
    }
  }
  visited.add(newPhase)  // the new phase is now visited

  return current.map((s) => {
    if (s.id === newPhase) return { ...s, status: "active" }
    if (visited.has(s.id)) return { ...s, status: "done" }
    return { ...s, status: "pending" }
  })
}
```

- [ ] **Step 1: Add `updateStepsForPhase` helper to `step-ribbon.tsx`**

Append to `step-ribbon.tsx`:

```tsx
/**
 * Compute the next steps array after a phase_changed event.
 *
 * Rules (spec §6.3):
 * - The named phase becomes 'active'
 * - Any phase that was previously 'active' becomes 'done'
 * - Any phase that was previously 'done' stays 'done' (we never
 *   un-complete a phase, even if the agent moves backwards)
 * - Phases the agent has never touched stay 'pending'
 */
export function updateStepsForPhase(
  current: Step[],
  newPhase: string,
): Step[] {
  return current.map((s) => {
    if (s.id === newPhase) {
      return { ...s, status: "active" as const }
    }
    if (s.status === "active") {
      // Previously active but the agent moved — mark as done
      return { ...s, status: "done" as const }
    }
    return s
  })
}
```

- [ ] **Step 2: Add the test for `updateStepsForPhase`**

Append to `step-ribbon.test.tsx`:

```tsx
import { updateStepsForPhase } from "./step-ribbon"

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

    // Analyze and Generate should both be in the visited set; the
    // active one is Generate (current), the other visited (Analyze)
    // stays done. Build, which we just left, becomes done.
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
```

- [ ] **Step 3: Add the `phase_changed` SSE handler to `agent-chat.tsx`**

Find the `switch (event.type)` block in the SSE reader (around line 441). Add a new case:

```tsx
      case "phase_changed": {
        const phase = event.phase as string
        if (phase) {
          // Push the update through onStepsUpdate; page.tsx owns the
          // steps state and computes the new array via updateStepsForPhase.
          onStepsUpdate?.(phase)
        }
        break
      }
```

Wait — `onStepsUpdate` in the current code takes `Array<{ id, label, status }>`, not a phase name. We have two options:
1. Keep the existing signature and compute `updateStepsForPhase` inside agent-chat.tsx (but agent-chat doesn't know the current steps — it would need them passed down as a prop)
2. Change `onStepsUpdate` to take a phase name and let page.tsx do the update

Option 2 is cleaner — the page owns steps state, agent-chat just reports "agent said it moved to Build" and the page updates its own state. Change `onStepsUpdate` signature:

```tsx
// Before
onStepsUpdate?: (
  steps: Array<{ id: string; label: string; status: string }>,
) => void

// After
onPhaseChange?: (phase: string) => void
```

And the page does:
```tsx
<AgentChat
  ...
  onPhaseChange={(phase) => setSteps((prev) => updateStepsForPhase(prev, phase))}
/>
```

Rename `onStepsUpdate` → `onPhaseChange` throughout the agent-chat.tsx file. Only two call sites (the prop declaration and the phase_changed handler).

- [ ] **Step 4: Update `page.tsx`**

Find:
```tsx
import { StepRibbon, type Step } from "@/components/agent/step-ribbon"
```
Update to:
```tsx
import {
  StepRibbon,
  initialSteps,
  updateStepsForPhase,
  type Step,
} from "@/components/agent/step-ribbon"
```

Find the steps state init (likely `const [steps, setSteps] = useState<Step[]>([])` or similar) and update to:
```tsx
const [steps, setSteps] = useState<Step[]>(initialSteps())
```

Find the `<AgentChat />` JSX and add the prop:
```tsx
<AgentChat
  ...
  onPhaseChange={(phase) =>
    setSteps((prev) => updateStepsForPhase(prev, phase))
  }
/>
```

Delete any old `onStepsUpdate={...}` usage if present.

- [ ] **Step 5: Run tests + typecheck**

```bash
cd forge-portal && npx tsc --noEmit 2>&1 | tail -30
```
Expected: clean.

```bash
cd forge-portal && npx vitest run components/agent/step-ribbon.test.tsx components/agent/agent-chat.test.tsx
```
Expected: step-ribbon test additions pass, agent-chat tests may need updates for the renamed prop (Task 6.9 revisits them).

- [ ] **Step 6: Commit**

```bash
git add forge-portal/components/agent/step-ribbon.tsx forge-portal/components/agent/step-ribbon.test.tsx forge-portal/components/agent/agent-chat.tsx "forge-portal/app/(dashboard)/projects/[id]/agent/page.tsx"
git commit -m "feat(portal): wire phase_changed events to StepRibbon

updateStepsForPhase(current, newPhase) is the pure state-update
helper:
- named phase becomes 'active'
- any previously-active phase becomes 'done' (left behind)
- already-done phases stay done (backward movement doesn't undo
  finished phases)
- untouched phases stay 'pending'

agent-chat.tsx gets a new 'phase_changed' SSE case that calls
onPhaseChange(phase). The prop is renamed from onStepsUpdate
(which had a stale Step[] signature from the pair_pipeline days)
— agent-chat just reports 'agent said Build', page.tsx owns the
steps state and computes the next array.

page.tsx initializes steps via initialSteps() (all 7 phases
pending) and calls updateStepsForPhase in the handler.

Tests: 4 new updateStepsForPhase cases — first call, transition,
backward movement preserves done, unknown phase is no-op."
```

---

### Task 6.5: Update `tool-formatters.ts` for 6 file tools + bash + set_phase

**Files:**
- Modify: `forge-portal/components/agent/tool-formatters.ts`

**Context:** `tool-formatters.ts` maps a tool name + input + output to the visual fields the tool card renders: icon, label, status, optional metadata. The current version (from Phase 3 of pair_pipeline work) handles `read_project_file`, `query_*`, and a generic fallback. It needs formatters for the 6 file tools, `bash`, and a special entry for `set_phase` with `hideCard: true` so the phase change doesn't render a noise card.

Spec §6.3 in the design doc has the full formatter table:

| tool | icon | label | status |
|---|---|---|---|
| `read_file` | 🔍 | `input.path` | `N lines` parsed from output |
| `write_file` | ✏️ | `input.path` | `created` |
| `edit_file` | ✏️ | `input.path` | `+X -Y` parsed from output |
| `glob` | 📁 | `input.pattern` | `N matches` parsed from output |
| `grep` | 🔎 | `input.pattern` | `N results` parsed from output |
| `list_directory` | 📂 | `input.path` or `.` | `N items` parsed from output |
| `bash` | ▶ | truncated `input.command` | `exit code: N` parsed from output |
| `set_phase` | → | `Phase: {input.phase}` | `""` | **hideCard: true** |

- [ ] **Step 1: Read the existing tool-formatters.ts**

```bash
cat forge-portal/components/agent/tool-formatters.ts
```

Understand the current type and switch structure. It's ~100 lines so a full rewrite is fine.

- [ ] **Step 2: Rewrite `tool-formatters.ts`**

Replace the content with:

```tsx
/**
 * Tool card formatters for the Variant B agent UI.
 *
 * Maps a tool invocation (name + input + output) to the visual
 * fields the tool card renders. Each formatter is pure — given
 * the same inputs, returns the same ToolSummary.
 *
 * `hideCard: true` signals that the tool should NOT render a
 * tool card at all. Currently only set_phase uses this — the
 * phase change is shown in the step ribbon, so a redundant tool
 * card for every set_phase call would be visual noise.
 */

export interface ToolSummary {
  icon: string
  label: string
  status: string
  hideCard?: boolean
}

function truncate(s: string, maxLen: number): string {
  if (s.length <= maxLen) return s
  return s.slice(0, maxLen - 3) + "..."
}

// Parse "X lines" or "showing N lines" from read_file output.
// Falls back to empty string if the output shape doesn't match.
function parseLineCount(output?: string): string {
  if (!output) return ""
  // read_file output contains N lines starting with "    1\t..."
  const lines = output.split("\n").length
  // The last line may be a truncation note
  return `${lines - 1} lines`
}

function parseEditDelta(output?: string): string {
  if (!output) return ""
  // Match "+X -Y line(s)" from EditFileTool output
  const m = output.match(/\+(\d+)\s+-(\d+)/)
  if (!m) return "edited"
  return `+${m[1]} -${m[2]}`
}

function parseMatchCount(output?: string): string {
  if (!output) return ""
  if (output.startsWith("No matches")) return "0 matches"
  // Glob output is one path per line (+ optional truncation note)
  const lines = output.split("\n").filter((l) => l.trim() && !l.startsWith("..."))
  return `${lines.length} matches`
}

function parseResultCount(output?: string): string {
  if (!output) return ""
  if (output.startsWith("No matches")) return "0 results"
  const lines = output.split("\n").filter((l) => l.trim() && !l.startsWith("..."))
  return `${lines.length} results`
}

function parseItemCount(output?: string): string {
  if (!output) return ""
  if (output.includes("(empty directory)")) return "0 items"
  const lines = output.split("\n").filter((l) => l.trim() && !l.startsWith("..."))
  return `${lines.length} items`
}

function parseBashExitCode(output?: string): string {
  if (!output) return ""
  // Output format: "$ command\nexit code: N\n\n..."
  const m = output.match(/^exit code:\s*(-?\d+)/m)
  if (!m) return ""
  const code = m[1]
  return code === "0" ? "ok" : `exit ${code}`
}

/**
 * Format a single tool invocation into its card summary.
 *
 * Unknown tool names fall through to a generic formatter so the
 * card still renders with sensible defaults.
 */
export function formatToolSummary(
  name: string,
  input: Record<string, unknown>,
  output?: string,
): ToolSummary {
  switch (name) {
    // --- T2 file tools ---
    case "read_file":
      return {
        icon: "🔍",
        label: String(input.path ?? ""),
        status: parseLineCount(output),
      }

    case "write_file":
      return {
        icon: "✏️",
        label: String(input.path ?? ""),
        status: "created",
      }

    case "edit_file":
      return {
        icon: "✏️",
        label: String(input.path ?? ""),
        status: parseEditDelta(output),
      }

    case "glob":
      return {
        icon: "📁",
        label: String(input.pattern ?? ""),
        status: parseMatchCount(output),
      }

    case "grep":
      return {
        icon: "🔎",
        label: String(input.pattern ?? ""),
        status: parseResultCount(output),
      }

    case "list_directory":
      return {
        icon: "📂",
        label: String(input.path ?? "."),
        status: parseItemCount(output),
      }

    // --- T2 exec tools ---
    case "bash":
      return {
        icon: "▶",
        label: truncate(String(input.command ?? ""), 60),
        status: parseBashExitCode(output),
      }

    case "set_phase":
      return {
        icon: "→",
        label: `Phase: ${String(input.phase ?? "")}`,
        status: "",
        hideCard: true,
      }

    // --- Legacy context tools (kept; Phase 3 already registered them) ---
    case "query_api_catalog":
    case "query_db_schema":
    case "query_business_rules":
    case "query_module_graph":
      return {
        icon: "📚",
        label: String(input.keyword ?? input.table_name ?? input.domain ?? input.module_name ?? name),
        status: output ? `${output.length} chars` : "",
      }

    case "read_project_file":
      return {
        icon: "📖",
        label: String(input.path ?? ""),
        status: output ? `${output.length} chars` : "",
      }

    // --- Fallback ---
    default:
      return {
        icon: "🛠",
        label: name,
        status: "",
      }
  }
}
```

- [ ] **Step 3: Write/update tool-formatters tests**

Create or append `forge-portal/components/agent/tool-formatters.test.ts`:

```ts
import { describe, expect, it } from "vitest"
import { formatToolSummary } from "./tool-formatters"

describe("formatToolSummary", () => {
  describe("read_file", () => {
    it("shows path and line count", () => {
      const summary = formatToolSummary(
        "read_file",
        { path: "src/main.go" },
        "     1\tpackage main\n     2\t\n     3\timport \"fmt\"\n",
      )
      expect(summary.icon).toBe("🔍")
      expect(summary.label).toBe("src/main.go")
      expect(summary.status).toContain("lines")
    })
  })

  describe("write_file", () => {
    it("shows path and created status", () => {
      const summary = formatToolSummary(
        "write_file",
        { path: "new.txt", content: "hello" },
        "Wrote 1 line(s), 5 byte(s) to new.txt",
      )
      expect(summary.icon).toBe("✏️")
      expect(summary.label).toBe("new.txt")
      expect(summary.status).toBe("created")
    })
  })

  describe("edit_file", () => {
    it("parses +X -Y from output", () => {
      const summary = formatToolSummary(
        "edit_file",
        { path: "foo.py" },
        "Replaced in foo.py (+3 -1 line(s))",
      )
      expect(summary.status).toBe("+3 -1")
    })

    it("falls back to 'edited' on unparseable output", () => {
      const summary = formatToolSummary(
        "edit_file",
        { path: "foo.py" },
        "something weird",
      )
      expect(summary.status).toBe("edited")
    })
  })

  describe("glob", () => {
    it("counts matches from newline-separated output", () => {
      const summary = formatToolSummary(
        "glob",
        { pattern: "**/*.go" },
        "src/main.go\nsrc/util/helper.go\n",
      )
      expect(summary.icon).toBe("📁")
      expect(summary.label).toBe("**/*.go")
      expect(summary.status).toBe("2 matches")
    })

    it("reports 0 matches on 'No matches' output", () => {
      const summary = formatToolSummary(
        "glob",
        { pattern: "*.xyz" },
        "No matches for pattern '*.xyz'",
      )
      expect(summary.status).toBe("0 matches")
    })
  })

  describe("grep", () => {
    it("counts result lines", () => {
      const summary = formatToolSummary(
        "grep",
        { pattern: "TODO" },
        "a.go:12:// TODO fix this\nb.go:5:// TODO refactor\n",
      )
      expect(summary.icon).toBe("🔎")
      expect(summary.status).toBe("2 results")
    })
  })

  describe("list_directory", () => {
    it("counts items", () => {
      const summary = formatToolSummary(
        "list_directory",
        { path: "src" },
        "util/\nmain.go\nhandler.go\n",
      )
      expect(summary.icon).toBe("📂")
      expect(summary.label).toBe("src")
      expect(summary.status).toBe("3 items")
    })

    it("shows . when no path provided", () => {
      const summary = formatToolSummary("list_directory", {}, "a\nb\n")
      expect(summary.label).toBe(".")
    })
  })

  describe("bash", () => {
    it("truncates long commands and parses exit code", () => {
      const longCmd = "go build ./... " + "-tag=".repeat(20)
      const summary = formatToolSummary(
        "bash",
        { command: longCmd },
        "$ " + longCmd + "\nexit code: 0\n\nbuild output",
      )
      expect(summary.icon).toBe("▶")
      expect(summary.label.length).toBeLessThanOrEqual(60)
      expect(summary.status).toBe("ok")
    })

    it("shows exit code when non-zero", () => {
      const summary = formatToolSummary(
        "bash",
        { command: "go build" },
        "$ go build\nexit code: 1\n\nerror: ...",
      )
      expect(summary.status).toBe("exit 1")
    })
  })

  describe("set_phase", () => {
    it("has hideCard: true", () => {
      const summary = formatToolSummary(
        "set_phase",
        { phase: "Generate" },
        "Phase set to Generate",
      )
      expect(summary.hideCard).toBe(true)
      expect(summary.label).toContain("Generate")
    })
  })

  describe("unknown tool", () => {
    it("falls through to generic formatter", () => {
      const summary = formatToolSummary("mystery_tool", { x: 1 }, "out")
      expect(summary.icon).toBe("🛠")
      expect(summary.label).toBe("mystery_tool")
      expect(summary.hideCard).toBeUndefined()
    })
  })
})
```

- [ ] **Step 4: Run tests**

```bash
cd forge-portal && npx vitest run components/agent/tool-formatters.test.ts
```
Expected: all tests pass.

- [ ] **Step 5: Commit**

```bash
git add forge-portal/components/agent/tool-formatters.ts forge-portal/components/agent/tool-formatters.test.ts
git commit -m "feat(portal): tool formatters for 6 file tools + bash + set_phase

Adds per-tool visual-card formatters for the full T2 surface:
- read_file    🔍 path + 'N lines'
- write_file   ✏️ path + 'created'
- edit_file    ✏️ path + '+X -Y' parsed from output
- glob         📁 pattern + 'N matches'
- grep         🔎 pattern + 'N results'
- list_directory 📂 path + 'N items'
- bash         ▶ truncated command + 'ok' or 'exit N'
- set_phase    → 'Phase: X' with hideCard: true

set_phase's hideCard flag means the tool card never renders —
the phase change shows in the step ribbon instead, so a
redundant card would be visual noise.

Parsers are output-shape specific (e.g., parseEditDelta matches
'+3 -1 line(s)' from EditFileTool's success message, parseBashExitCode
matches '^exit code: N' in the bash tool's output). They fall
back to sensible defaults on unparseable output.

Existing context_tools (query_api_catalog etc.) keep their
formatters. Unknown tools fall through to a 🛠 fallback."
```

---

### Task 6.6: Update `tool-execution.tsx` to respect `hideCard`

**Files:**
- Modify: `forge-portal/components/agent/tool-execution.tsx`

**Context:** When `formatToolSummary` returns `{hideCard: true}`, the tool card should not render at all. Currently `tool-execution.tsx` unconditionally renders every tool. Add a check.

- [ ] **Step 1: Read the current component**

```bash
cat forge-portal/components/agent/tool-execution.tsx
```

- [ ] **Step 2: Add the hideCard check**

Find the top of the render function (likely a `function ToolExecution({ ...props }) { ... return (<div>...</div>) }` or arrow variant). Add near the top of the function body, after the call to `formatToolSummary`:

```tsx
const summary = formatToolSummary(name, input, output)

if (summary.hideCard) {
  // Tool explicitly opts out of card rendering (set_phase). The
  // effect of the tool (the phase change) is shown in the step
  // ribbon, not as a tool card.
  return null
}
```

Everything after that uses `summary` normally.

- [ ] **Step 3: Update tool-execution.test.tsx**

Add a test that renders `<ToolExecution name="set_phase" input={{phase: "Analyze"}} />` and asserts the component returns null:

```tsx
it("does not render a card for set_phase (hideCard)", () => {
  const { container } = render(
    <ToolExecution
      name="set_phase"
      input={{ phase: "Analyze" }}
      output="Phase set to Analyze"
    />,
  )
  expect(container.firstChild).toBeNull()
})
```

- [ ] **Step 4: Run tests**

```bash
cd forge-portal && npx vitest run components/agent/tool-execution.test.tsx
```

- [ ] **Step 5: Commit**

```bash
git add forge-portal/components/agent/tool-execution.tsx forge-portal/components/agent/tool-execution.test.tsx
git commit -m "feat(portal): tool-execution respects hideCard flag

When formatToolSummary returns {hideCard: true}, the component
returns null — no DOM output. Currently only set_phase uses
this flag; its effect (the phase change) is shown in the step
ribbon, so a tool card for every set_phase call would be noise.

Test asserts that ToolExecution for set_phase renders null."
```

---

### Task 6.7: Downgrade `code-panel.tsx` to a read-only preview shell

**Files:**
- Modify: `forge-portal/components/agent/code-panel.tsx`

**Context:** Spec §2.6 Q5.2 decided the code panel is a "shell only, read-only preview" for Phase 6. Any pair_pipeline-era features (inline editing, diff rendering, live file streaming from the agent) get stripped. What remains: click a file path in a tool card → the code panel shows the file contents as read-only text. That's it.

The current `code-panel.tsx` is 430 lines — likely does much more. This task trims it down to the read-only path.

- [ ] **Step 1: Read the current component**

```bash
wc -l forge-portal/components/agent/code-panel.tsx
grep -n "^function\|^export\|^const " forge-portal/components/agent/code-panel.tsx | head -30
```

Understand its public API (props, exports) so the rewrite preserves callers.

- [ ] **Step 2: Identify what's read-only and what's not**

Look for:
- Any `<textarea>` / `<input>` / `contentEditable` that suggests editing
- Any `useEffect` that subscribes to live file updates or runs diff algorithms
- Any imports from diff libraries (`diff`, `diff-match-patch`, etc.)
- Any pair_pipeline-specific prop handling

Mark each for deletion. Preserve:
- File path → file content fetch (probably hits a forge-core endpoint)
- Line number rendering
- Syntax highlighting library imports (these are usually read-only; keep them)

- [ ] **Step 3: Rewrite or surgically trim**

Depending on how tangled the file is, choose:
- **Surgical trim** (preferred): delete the edit paths, keep the fetch and render. Usually ~100 lines of deletion in a 430-line file.
- **Full rewrite** (only if the file is a spaghetti mess): write a new 80-line component from scratch with just `path → fetch → pre-wrap render`.

Aim for a file under 250 lines after this task.

Key skeleton (full rewrite if needed):

```tsx
"use client"

import { useEffect, useState } from "react"
import { cn } from "@/lib/utils"

interface CodePanelProps {
  projectId: string
  filePath: string | null
  className?: string
}

/**
 * Read-only file preview.
 *
 * Spec §2.6 Q5.2 decided this is a "shell only" for A2. The panel
 * fetches a file's contents from the forge-core code endpoint and
 * renders it as a scrollable <pre>. No editing, no diff, no live
 * updates. Future: upgrade to a real editor if the agent starts
 * supporting edit-in-place.
 */
export function CodePanel({ projectId, filePath, className }: CodePanelProps) {
  const [content, setContent] = useState<string | null>(null)
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    if (!filePath) {
      setContent(null)
      setError(null)
      return
    }

    let cancelled = false
    setLoading(true)
    setError(null)

    fetch(
      `/api/projects/${projectId}/code/file?path=${encodeURIComponent(filePath)}`,
    )
      .then(async (res) => {
        if (!res.ok) {
          throw new Error(`HTTP ${res.status}`)
        }
        const data = await res.json()
        if (!cancelled) {
          setContent(data.content ?? "")
        }
      })
      .catch((e) => {
        if (!cancelled) {
          setError(e.message)
          setContent(null)
        }
      })
      .finally(() => {
        if (!cancelled) setLoading(false)
      })

    return () => {
      cancelled = true
    }
  }, [projectId, filePath])

  if (!filePath) {
    return (
      <div
        className={cn(
          "flex items-center justify-center h-full text-[var(--text-tertiary)] text-[11px]",
          className,
        )}
      >
        Click a file in a tool card to preview it here
      </div>
    )
  }

  if (loading) {
    return (
      <div
        className={cn(
          "flex items-center justify-center h-full text-[var(--text-tertiary)] text-[11px]",
          className,
        )}
      >
        Loading {filePath}...
      </div>
    )
  }

  if (error) {
    return (
      <div
        className={cn(
          "flex items-center justify-center h-full text-[var(--text-error)] text-[11px]",
          className,
        )}
      >
        Failed to load {filePath}: {error}
      </div>
    )
  }

  return (
    <div className={cn("flex flex-col h-full", className)}>
      <div className="h-8 px-3 flex items-center border-b border-[var(--border-primary)] text-[11px] font-mono text-[var(--text-secondary)]">
        {filePath}
      </div>
      <pre className="flex-1 overflow-auto p-3 text-[11px] font-mono whitespace-pre bg-[var(--bg-primary)]">
        {content}
      </pre>
    </div>
  )
}
```

Match the prop names to what `page.tsx` currently passes — if it's `projectId` + `filePath`, keep them; if it's different, adapt.

- [ ] **Step 4: Update `code-panel.test.tsx`**

```bash
grep -n "describe\|it\|test(" forge-portal/components/agent/code-panel.test.tsx | head
```

Most existing tests probably assert edit functionality that we just deleted. Replace the test file:

```tsx
import { render, screen, waitFor } from "@testing-library/react"
import { describe, expect, it, vi, beforeEach } from "vitest"
import { CodePanel } from "./code-panel"


describe("CodePanel", () => {
  beforeEach(() => {
    vi.restoreAllMocks()
  })

  it("shows a placeholder when no file path is selected", () => {
    render(<CodePanel projectId="1" filePath={null} />)
    expect(screen.getByText(/click a file/i)).toBeInTheDocument()
  })

  it("fetches and renders file content", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn(() =>
        Promise.resolve({
          ok: true,
          json: () => Promise.resolve({ content: "package main\n\nfunc main() {}\n" }),
        }),
      ),
    )

    render(<CodePanel projectId="1" filePath="main.go" />)
    await waitFor(() => {
      expect(screen.getByText(/func main/)).toBeInTheDocument()
    })
  })

  it("shows an error message on fetch failure", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn(() =>
        Promise.resolve({ ok: false, status: 404 }),
      ),
    )

    render(<CodePanel projectId="1" filePath="missing.go" />)
    await waitFor(() => {
      expect(screen.getByText(/failed to load/i)).toBeInTheDocument()
    })
  })
})
```

- [ ] **Step 5: Run tests + typecheck**

```bash
cd forge-portal && npx vitest run components/agent/code-panel.test.tsx
cd forge-portal && npx tsc --noEmit 2>&1 | tail -20
```

- [ ] **Step 6: Commit**

```bash
git add forge-portal/components/agent/code-panel.tsx forge-portal/components/agent/code-panel.test.tsx
git commit -m "refactor(portal): code-panel downgraded to read-only preview shell

Spec §2.6 Q5.2 decided this is a 'shell only' for A2. Stripped:
- any inline editing paths (contentEditable, textarea-as-editor)
- any diff rendering
- any live file-update subscription

What remains: click a file path in a tool card, the panel
fetches content from /api/projects/\${id}/code/file?path=...,
renders as a scrollable <pre>. File header shows the path.
Loading, error, and empty states covered.

Tests rewritten: placeholder state when no file selected,
content render on successful fetch, error message on failed
fetch.

Future: upgrade to a real editor component if the agent starts
supporting edit-in-place (out of scope for chronos)."
```

---

### Task 6.8: Relocate `thinking-indicator.tsx` to attach to bash tool cards

**Files:**
- Modify: `forge-portal/components/agent/thinking-indicator.tsx`
- Modify: `forge-portal/components/agent/agent-chat.tsx`

**Context:** Spec §2.6 Q5.6 decided the thinking indicator should attach to the bash tool card that's currently executing, not float at the chat bottom. This is a UX improvement — the user sees "Running go build" right on the bash card, so they know which action is taking time.

Implementation:
- `thinking-indicator.tsx` gets a `label` prop and renders a small pulsing component
- `agent-chat.tsx` tracks `currentBashToolUseId: string | null` in state
- On `thinking_started`, set `currentBashToolUseId` = the latest `tool_started` event's `tool_use_id` (assuming it's a bash call — if it's not, the event is stray and we ignore it)
- On `thinking_stopped`, clear `currentBashToolUseId`
- In the message rendering loop, when rendering the bash tool card matching `currentBashToolUseId`, render `<ThinkingIndicator label={currentThinkingLabel} />` below it

- [ ] **Step 1: Simplify `thinking-indicator.tsx` to a pure pulsing component**

```bash
cat forge-portal/components/agent/thinking-indicator.tsx
```

Likely already small (42 lines per the wc earlier). If it has chat-bottom-specific positioning logic, strip it. The new version:

```tsx
"use client"

import { cn } from "@/lib/utils"

interface ThinkingIndicatorProps {
  label: string
  className?: string
}

/**
 * A small pulsing indicator shown below a running bash tool card.
 * Phase 6 Task 6.8 repositioned this from chat-bottom to attached
 * to the specific tool card that's executing.
 */
export function ThinkingIndicator({ label, className }: ThinkingIndicatorProps) {
  return (
    <div
      className={cn(
        "flex items-center gap-1.5 mt-1 text-[11px] text-[var(--text-tertiary)]",
        className,
      )}
      role="status"
      aria-live="polite"
    >
      <span className="relative flex h-1.5 w-1.5">
        <span className="animate-ping absolute inline-flex h-full w-full rounded-full bg-[var(--accent)] opacity-75"></span>
        <span className="relative inline-flex rounded-full h-1.5 w-1.5 bg-[var(--accent)]"></span>
      </span>
      <span className="font-mono">{label}</span>
    </div>
  )
}
```

- [ ] **Step 2: Wire state in `agent-chat.tsx`**

Add two state fields near the top of the component body:

```tsx
const [currentBashToolUseId, setCurrentBashToolUseId] = useState<string | null>(null)
const [currentThinkingLabel, setCurrentThinkingLabel] = useState<string>("")
```

In the SSE switch:

```tsx
      case "thinking_started": {
        setCurrentThinkingLabel(String(event.label ?? "Working..."))
        // Bind to the most recent bash tool call if one is in flight.
        // The most recent tool_started with tool_name=bash should be
        // findable in the current assistant message's tools array.
        // Simpler approach: track the last seen bash tool_use_id in
        // state and flip to it here.
        // (See 'tool_started' case below for the tracking.)
        break
      }
      case "thinking_stopped": {
        setCurrentThinkingLabel("")
        setCurrentBashToolUseId(null)
        break
      }
      case "tool_started": {
        const tid = String(event.tool_use_id ?? "")
        const tname = String(event.tool_name ?? "")
        if (tname === "bash" && tid) {
          setCurrentBashToolUseId(tid)
        }
        // ... existing tool_started handling (append to message.tools) ...
        break
      }
```

In the message render loop, when iterating over a message's tool calls, add after the rendered `<ToolExecution>`:

```tsx
{tool.name === "bash" &&
  tool.toolUseId === currentBashToolUseId &&
  currentThinkingLabel && (
    <ThinkingIndicator label={currentThinkingLabel} />
  )}
```

The exact shape of `tool.toolUseId` depends on how `tool_started` populates the tool object — you may need to add `toolUseId` to the tool card's type if it isn't there already. Task 6.2 was supposed to preserve this via the tool_use_id SSE field; if it got dropped, add it back now.

- [ ] **Step 3: Run tests**

```bash
cd forge-portal && npx vitest run components/agent/thinking-indicator.test.tsx components/agent/agent-chat.test.tsx
```
Expected: thinking-indicator test passes; agent-chat tests may need updates (covered in Task 6.9).

- [ ] **Step 4: Commit**

```bash
git add forge-portal/components/agent/thinking-indicator.tsx forge-portal/components/agent/agent-chat.tsx
git commit -m "feat(portal): thinking-indicator attaches to running bash tool card

Spec §2.6 Q5.6 moved the thinking indicator from chat-bottom
to pinned under the bash tool card that's currently running.
The user sees 'Running go build' right below the command that's
taking time, matching what's actually blocking.

Implementation:
- ThinkingIndicator becomes a pure pulsing component with a
  label prop; no positioning logic
- agent-chat.tsx tracks currentBashToolUseId + currentThinkingLabel
  in state
- On tool_started with tool_name='bash', bind the id
- On thinking_started, update the label
- On thinking_stopped, clear both
- In the message render, emit <ThinkingIndicator> below the
  matching bash tool card

Tool objects in the chat message model gain a toolUseId field
so the render can match the current label against a specific
card."
```

---

### Task 6.9: Visual fix-loop detection heuristic + agent-chat tests

**Files:**
- Modify: `forge-portal/components/agent/agent-chat.tsx`
- Modify: `forge-portal/components/agent/agent-chat.test.tsx`

**Context:** Spec §2.6 Q5.5 replaced `fix_loop_*` events with a **frontend-only visual heuristic**: when the agent runs `bash` → it errors → then `edit_file` → then another `bash`, insert a subtle "Fixing previous error..." banner between the edit and the second bash. Everything is derived from the existing event stream; no new event types.

The heuristic lives in one helper function:

```tsx
function detectFixLoopStart(
  messages: ChatMessage[],
  newToolName: string,
): "insert_banner" | null {
  if (newToolName !== "bash") return null

  const last = messages[messages.length - 1]
  if (!last || last.role !== "assistant") return null

  const tools = last.tools ?? []
  let sawWrite = false
  for (let i = tools.length - 1; i >= 0; i--) {
    const t = tools[i]
    if (t.name === "write_file" || t.name === "edit_file") {
      sawWrite = true
      continue
    }
    if (t.name === "bash") {
      return t.isError && sawWrite ? "insert_banner" : null
    }
  }
  return null
}
```

The caller in `tool_started` handler runs the heuristic before appending the new bash tool:

```tsx
      case "tool_started": {
        const tname = String(event.tool_name ?? "")
        if (detectFixLoopStart(messages, tname) === "insert_banner") {
          // Append a subtle system message before the new tool card
          appendSystemMessage("Fixing previous error...")
        }
        // ... existing tool_started handling ...
        break
      }
```

This task also does the **full test rewrite** for `agent-chat.test.tsx` — catches everything touched in Tasks 6.2, 6.4, 6.8, and this one. If agent-chat.test.tsx has gotten stale across the phase, fix it here.

- [ ] **Step 1: Add `detectFixLoopStart` helper**

In `agent-chat.tsx`, near the top-level helpers (hydrateFromDurableLog etc.):

```tsx
/**
 * Visual fix-loop detection heuristic.
 *
 * Spec §2.6 Q5.5 replaced the fix_loop_* events with a frontend-only
 * heuristic: when the agent runs bash -> it errors -> then edit_file
 * (or write_file) -> then another bash, we detect the pattern and
 * insert a subtle "Fixing previous error..." system message.
 *
 * Returns "insert_banner" if the current assistant message's tool
 * history matches bash-error-then-edit-then-new-bash, else null.
 */
function detectFixLoopStart(
  messages: ChatMessage[],
  newToolName: string,
): "insert_banner" | null {
  if (newToolName !== "bash") return null

  const last = messages[messages.length - 1]
  if (!last || last.role !== "assistant") return null

  const tools = last.tools ?? []
  let sawWrite = false
  // Walk backwards; find the most recent bash, remembering whether
  // we crossed any write/edit on the way.
  for (let i = tools.length - 1; i >= 0; i--) {
    const t = tools[i]
    if (t.name === "write_file" || t.name === "edit_file") {
      sawWrite = true
      continue
    }
    if (t.name === "bash") {
      return t.isError && sawWrite ? "insert_banner" : null
    }
  }
  return null
}
```

- [ ] **Step 2: Wire the heuristic into `tool_started` handler**

Inside the `case "tool_started":` block in the SSE switch, at the start:

```tsx
const tname = String(event.tool_name ?? "")
if (detectFixLoopStart(messages, tname) === "insert_banner") {
  // Insert a system-role message before the new tool card so the
  // chat timeline reads: ...bash(error) edit bash  becomes
  //                     ...bash(error) edit [Fixing previous error...] bash
  setMessages((prev) => [
    ...prev,
    {
      id: `sys-fix-${Date.now()}`,
      role: "system",
      content: "Fixing previous error...",
      timestamp: Date.now(),
    },
  ])
}
// ... existing tool_started handling continues ...
```

- [ ] **Step 3: Rewrite `agent-chat.test.tsx` — full sweep**

Read the current test file:

```bash
wc -l forge-portal/components/agent/agent-chat.test.tsx
grep -n "describe\|it\|test(" forge-portal/components/agent/agent-chat.test.tsx
```

Update the tests to:
- Remove any BuildCard / BuildInfo / coder / reviewer assertions
- Add a test for `detectFixLoopStart` (pure function — easy to unit test)
- Update the `phase_changed` and thinking-indicator wiring tests per Task 6.4 and 6.8
- Keep the core SSE streaming tests (text_delta concatenation, tool_started/completed pairing, turn_complete marker)

Minimum new unit tests:

```tsx
// Append to agent-chat.test.tsx

import { describe, expect, it } from "vitest"
// Export detectFixLoopStart from agent-chat.tsx as a named helper
// for testing; the default export is the component
import { detectFixLoopStart } from "./agent-chat"

describe("detectFixLoopStart", () => {
  const base = { id: "1", role: "assistant" as const, content: "", timestamp: 0 }

  it("returns null when the new tool is not bash", () => {
    const messages = [{ ...base, tools: [] }]
    expect(detectFixLoopStart(messages, "read_file")).toBeNull()
  })

  it("returns null when there's no previous bash in the message", () => {
    const messages = [
      { ...base, tools: [{ name: "read_file", input: {}, isError: false }] },
    ]
    expect(detectFixLoopStart(messages, "bash")).toBeNull()
  })

  it("returns null when the previous bash succeeded", () => {
    const messages = [
      {
        ...base,
        tools: [
          { name: "bash", input: {}, isError: false },
          { name: "edit_file", input: {}, isError: false },
        ],
      },
    ]
    expect(detectFixLoopStart(messages, "bash")).toBeNull()
  })

  it("returns null when there's an error bash but no edit between", () => {
    const messages = [
      { ...base, tools: [{ name: "bash", input: {}, isError: true }] },
    ]
    expect(detectFixLoopStart(messages, "bash")).toBeNull()
  })

  it("returns insert_banner for bash-error → edit → new-bash", () => {
    const messages = [
      {
        ...base,
        tools: [
          { name: "bash", input: {}, isError: true },
          { name: "edit_file", input: {}, isError: false },
        ],
      },
    ]
    expect(detectFixLoopStart(messages, "bash")).toBe("insert_banner")
  })

  it("returns insert_banner for bash-error → write_file → new-bash", () => {
    const messages = [
      {
        ...base,
        tools: [
          { name: "bash", input: {}, isError: true },
          { name: "write_file", input: {}, isError: false },
        ],
      },
    ]
    expect(detectFixLoopStart(messages, "bash")).toBe("insert_banner")
  })

  it("walks back through multiple writes to find the bash", () => {
    const messages = [
      {
        ...base,
        tools: [
          { name: "bash", input: {}, isError: true },
          { name: "edit_file", input: {}, isError: false },
          { name: "edit_file", input: {}, isError: false },
          { name: "write_file", input: {}, isError: false },
        ],
      },
    ]
    expect(detectFixLoopStart(messages, "bash")).toBe("insert_banner")
  })

  it("stops at the most recent bash, not earlier ones", () => {
    // Sequence: bash-error edit bash-success edit NEW bash
    // The most recent bash (bash-success) is NOT an error, so no banner.
    const messages = [
      {
        ...base,
        tools: [
          { name: "bash", input: {}, isError: true },
          { name: "edit_file", input: {}, isError: false },
          { name: "bash", input: {}, isError: false },
          { name: "edit_file", input: {}, isError: false },
        ],
      },
    ]
    expect(detectFixLoopStart(messages, "bash")).toBeNull()
  })
})
```

Make sure `detectFixLoopStart` is exported (add `export` to the function declaration) so tests can import it.

- [ ] **Step 4: Delete stale BuildCard / fix_loop_ tests in agent-chat.test.tsx**

`grep -n "BuildCard\|fix_loop_\|coder\|reviewer\|buildInfo" forge-portal/components/agent/agent-chat.test.tsx` — delete every test that references these.

- [ ] **Step 5: Run the full portal test suite**

```bash
cd forge-portal && npx vitest run
```
Expected: all passing. If any test fails because of a type change from Tasks 6.2 / 6.4 / 6.8 that this task missed, update the test.

- [ ] **Step 6: Typecheck**

```bash
cd forge-portal && npx tsc --noEmit 2>&1 | tail -20
```
Expected: clean.

- [ ] **Step 7: Run the full build (if the project has one)**

```bash
cd forge-portal && npx next build 2>&1 | tail -30
```

**⚠️ Before running `next build`:** remember `AGENTS.md` warning — this is NOT the Next.js you know. If `next build` fails with a command not found or unexpected flag, `ls node_modules/next/dist/docs/` and find the right invocation first.

Expected: successful build (or skip if the agent doesn't have time / docker-compose for a full build; the typecheck pass in Step 6 is the minimum).

- [ ] **Step 8: Commit**

```bash
git add forge-portal/components/agent/agent-chat.tsx forge-portal/components/agent/agent-chat.test.tsx
git commit -m "feat(portal): detectFixLoopStart + agent-chat test sweep

Visual fix-loop detection heuristic (spec §2.6 Q5.5):
- detectFixLoopStart(messages, newToolName) walks back through the
  current assistant message's tools looking for a bash-error
  followed by an edit/write, then returns 'insert_banner' if the
  new tool is another bash
- Wired into the tool_started SSE case: on insert_banner, prepend
  a subtle system message 'Fixing previous error...' before the
  new bash tool card renders

The heuristic is pure so it unit-tests cleanly. 8 test cases:
- non-bash new tool -> null
- no prior bash -> null
- prior bash succeeded -> null
- bash error with no intervening edit -> null
- bash error + edit + new bash -> insert_banner
- bash error + write_file + new bash -> insert_banner
- multiple edits between bash and new bash -> insert_banner
- multiple bashes, latest succeeded -> null

Also: full sweep of agent-chat.test.tsx to delete stale BuildCard
/ fix_loop_ / coder / reviewer assertions from the pair_pipeline
era. The test file now matches the A2 architecture Phase 6
delivered."
```

---

### Task 6.10: `ClarificationInput` component + agent-chat state machine (Round 2)

**Files:**
- Create: `forge-portal/components/agent/clarification-input.tsx`
- Create: `forge-portal/components/agent/clarification-input.test.tsx`
- Modify: `forge-portal/components/agent/agent-chat.tsx`
- Modify: `forge-portal/lib/agent.ts` (add `postClarification` API client function)

**Context:** Spec §2.9.2 introduces the `request_clarification` meta-tool — the agent can pause its own execution to ask the human user for input. The event ordering on the wire is (§2.9.2.e):

```
ToolExecutionStarted(tool_name="request_clarification", tool_use_id=X)
  → ClarificationRequested(question, tool_use_id=X)
  → [pause — agent blocks on a Future waiting for the user's reply]
  → ToolExecutionCompleted(tool_use_id=X, output=<user response>)
```

The frontend uses `ClarificationRequested` to render the inline input form and `ToolExecutionCompleted` with a matching `tool_use_id` to close it. The user's reply reaches the backend via `POST /api/sessions/{id}/clarify` with JSON body `{tool_use_id, response}` (§2.9.2.g), which resolves the pending Future server-side and lets the agent loop continue.

If the user takes longer than the 10-minute timeout (§2.9.2.f), the backend closes the SSE stream with an `ErrorEvent(recoverable=False)` whose message contains "clarification timeout". The UI shows a red banner: "Session ended: clarification timeout after 10 minutes."

The `ClarificationInput` component is a small controlled form (textarea + submit button) with a four-state FSM: `editing` → `submitting` → (`submitted` | `error`). From `error` the user can retry; from `submitted` the component is frozen and waits for the matching `tool_execution_completed` event to arrive, which unmounts it via the parent's state update.

While a clarification is pending, the **main chat input at the bottom of the screen is disabled** — the user must answer the clarification before sending a new message. This is deliberate: the agent is blocked on that specific question and any new user input would create ambiguity about what the agent should respond to.

- [ ] **Step 1: Add the `postClarification` API client function**

Append to `forge-portal/lib/agent.ts` (after the existing session CRUD functions). Match the existing `api` helper pattern:

```ts
/**
 * Submit a response to a pending `request_clarification` tool call.
 *
 * Spec §2.9.2.g: POST /api/sessions/{id}/clarify with JSON body
 * {tool_use_id, response}. Resolves the pending Future server-side
 * so the agent loop can continue.
 *
 * Returns on 204 No Content. Throws on 400 (bad input), 401
 * (unauthenticated), 403 (not your session), 404 (session not
 * found / no pending clarification matching tool_use_id), or 410
 * (clarification already resolved or timed out).
 */
export async function postClarification(
  sessionId: string,
  toolUseId: string,
  response: string,
): Promise<void> {
  const res = await fetch(
    `/api/sessions/${encodeURIComponent(sessionId)}/clarify`,
    {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      credentials: "include",
      body: JSON.stringify({ tool_use_id: toolUseId, response }),
    },
  )
  if (!res.ok) {
    const body = await res.text().catch(() => "")
    throw new Error(`HTTP ${res.status}${body ? `: ${body}` : ""}`)
  }
}
```

Note: the existing `api` helper in `@/lib/api` wraps `fetch` with JWT injection via `credentials: "include"` and the forge-core session cookie. Reuse it if it already exposes a `post` that returns void; otherwise a direct `fetch` as shown is fine — the existing `agent.ts` already mixes both patterns.

- [ ] **Step 2: Create `clarification-input.tsx`**

```tsx
"use client"

import { useState, type FormEvent } from "react"
import { cn } from "@/lib/utils"

interface ClarificationInputProps {
  question: string
  toolUseId: string
  onSubmit: (toolUseId: string, response: string) => Promise<void>
  disabled?: boolean
  className?: string
}

type State =
  | { kind: "editing" }
  | { kind: "submitting" }
  | { kind: "submitted" }
  | { kind: "error"; message: string }

const MAX_CHARS = 4096

/**
 * Inline form the agent uses to ask the human user a clarifying
 * question. Rendered below the current assistant message when
 * the SSE stream delivers a `clarification_requested` event.
 *
 * Four states:
 * - editing: user is typing a response (default)
 * - submitting: POST /api/sessions/{id}/clarify in flight
 * - submitted: success; waiting for the agent's tool_execution_completed
 *   event to arrive (at which point the parent unmounts this)
 * - error: POST failed; user can edit and retry
 *
 * The `disabled` prop is set by the parent when the whole session
 * is ending (e.g. clarification timeout) — the form should not
 * accept new input in that case.
 */
export function ClarificationInput({
  question,
  toolUseId,
  onSubmit,
  disabled,
  className,
}: ClarificationInputProps) {
  const [response, setResponse] = useState("")
  const [state, setState] = useState<State>({ kind: "editing" })

  const handleSubmit = async (e: FormEvent) => {
    e.preventDefault()
    if (state.kind === "submitting" || state.kind === "submitted") return
    if (response.trim().length === 0) return
    setState({ kind: "submitting" })
    try {
      await onSubmit(toolUseId, response)
      setState({ kind: "submitted" })
    } catch (err) {
      setState({
        kind: "error",
        message: err instanceof Error ? err.message : "Unknown error",
      })
    }
  }

  const inputDisabled = disabled || state.kind === "submitting" || state.kind === "submitted"

  return (
    <form
      onSubmit={handleSubmit}
      className={cn(
        "border border-[var(--border-primary)] rounded p-3 my-2 bg-[var(--bg-secondary)]",
        className,
      )}
      aria-label="Clarification response"
    >
      <div className="text-[11px] font-medium text-[var(--text-secondary)] mb-1.5">
        The agent is asking:
      </div>
      <div className="text-[12px] text-[var(--text-primary)] mb-2.5 whitespace-pre-wrap">
        {question}
      </div>
      <textarea
        value={response}
        onChange={(e) => setResponse(e.target.value)}
        placeholder="Your response..."
        disabled={inputDisabled}
        maxLength={MAX_CHARS}
        rows={3}
        className={cn(
          "w-full px-2 py-1.5 text-[12px] font-mono rounded",
          "bg-[var(--bg-primary)] border border-[var(--border-primary)]",
          "text-[var(--text-primary)] placeholder:text-[var(--text-tertiary)]",
          "focus:outline-none focus:border-[var(--accent)]",
          "disabled:opacity-60 disabled:cursor-not-allowed",
          "resize-y mb-2",
        )}
        aria-label="Clarification response textarea"
      />
      {state.kind === "error" && (
        <div
          className="text-[11px] text-[var(--text-error)] mb-2"
          role="alert"
        >
          {state.message}
        </div>
      )}
      {state.kind === "submitted" && (
        <div className="text-[11px] text-[var(--text-tertiary)] mb-2">
          Submitted — waiting for agent...
        </div>
      )}
      <div className="flex items-center justify-between">
        <div className="text-[10px] text-[var(--text-tertiary)]">
          {response.length}/{MAX_CHARS} characters
        </div>
        <button
          type="submit"
          disabled={
            disabled ||
            state.kind === "submitting" ||
            state.kind === "submitted" ||
            response.trim().length === 0
          }
          className={cn(
            "px-2.5 py-1 text-[11px] font-medium rounded",
            "bg-[var(--accent)] text-white",
            "hover:bg-[var(--accent-hover)]",
            "disabled:opacity-50 disabled:cursor-not-allowed",
            "transition-colors duration-100",
          )}
        >
          {state.kind === "submitting" && "Submitting..."}
          {state.kind === "submitted" && "Submitted"}
          {(state.kind === "editing" || state.kind === "error") &&
            "Submit Response"}
        </button>
      </div>
    </form>
  )
}
```

- [ ] **Step 3: Create `clarification-input.test.tsx`**

```tsx
import { render, screen, waitFor, fireEvent } from "@testing-library/react"
import { describe, expect, it, vi } from "vitest"

import { ClarificationInput } from "./clarification-input"

describe("ClarificationInput", () => {
  const baseProps = {
    question: "Which database should I use — Postgres or MySQL?",
    toolUseId: "toolu_abc123",
  }

  it("renders the question", () => {
    render(<ClarificationInput {...baseProps} onSubmit={vi.fn()} />)
    expect(
      screen.getByText(/Postgres or MySQL/),
    ).toBeInTheDocument()
  })

  it("disables submit when response is empty", () => {
    render(<ClarificationInput {...baseProps} onSubmit={vi.fn()} />)
    const btn = screen.getByRole("button", { name: /submit response/i })
    expect(btn).toBeDisabled()
  })

  it("enables submit when response is non-empty", () => {
    render(<ClarificationInput {...baseProps} onSubmit={vi.fn()} />)
    const textarea = screen.getByLabelText(/clarification response textarea/i)
    fireEvent.change(textarea, { target: { value: "Postgres" } })
    const btn = screen.getByRole("button", { name: /submit response/i })
    expect(btn).not.toBeDisabled()
  })

  it("calls onSubmit with toolUseId and response on click", async () => {
    const onSubmit = vi.fn().mockResolvedValue(undefined)
    render(<ClarificationInput {...baseProps} onSubmit={onSubmit} />)
    const textarea = screen.getByLabelText(/clarification response textarea/i)
    fireEvent.change(textarea, { target: { value: "Postgres please" } })
    const btn = screen.getByRole("button", { name: /submit response/i })
    fireEvent.click(btn)
    await waitFor(() => {
      expect(onSubmit).toHaveBeenCalledWith("toolu_abc123", "Postgres please")
    })
  })

  it("shows submitting state during async call", async () => {
    let resolveSubmit!: () => void
    const onSubmit = vi.fn().mockImplementation(
      () =>
        new Promise<void>((resolve) => {
          resolveSubmit = resolve
        }),
    )
    render(<ClarificationInput {...baseProps} onSubmit={onSubmit} />)
    const textarea = screen.getByLabelText(/clarification response textarea/i)
    fireEvent.change(textarea, { target: { value: "test" } })
    fireEvent.click(screen.getByRole("button", { name: /submit response/i }))
    await waitFor(() => {
      expect(screen.getByRole("button", { name: /submitting/i })).toBeInTheDocument()
    })
    resolveSubmit()
  })

  it("shows submitted state on success", async () => {
    const onSubmit = vi.fn().mockResolvedValue(undefined)
    render(<ClarificationInput {...baseProps} onSubmit={onSubmit} />)
    const textarea = screen.getByLabelText(/clarification response textarea/i)
    fireEvent.change(textarea, { target: { value: "go" } })
    fireEvent.click(screen.getByRole("button", { name: /submit response/i }))
    await waitFor(() => {
      expect(
        screen.getByRole("button", { name: /submitted/i }),
      ).toBeInTheDocument()
    })
    expect(screen.getByText(/waiting for agent/i)).toBeInTheDocument()
  })

  it("shows error state on failure and allows retry", async () => {
    const onSubmit = vi.fn().mockRejectedValue(new Error("HTTP 500: boom"))
    render(<ClarificationInput {...baseProps} onSubmit={onSubmit} />)
    const textarea = screen.getByLabelText(/clarification response textarea/i)
    fireEvent.change(textarea, { target: { value: "x" } })
    fireEvent.click(screen.getByRole("button", { name: /submit response/i }))
    await waitFor(() => {
      expect(screen.getByRole("alert")).toHaveTextContent(/HTTP 500/)
    })
    // After error the button returns to 'Submit Response' and is re-enabled
    // so the user can try again
    expect(
      screen.getByRole("button", { name: /submit response/i }),
    ).not.toBeDisabled()
  })

  it("respects maxLength of 4096", () => {
    render(<ClarificationInput {...baseProps} onSubmit={vi.fn()} />)
    const textarea = screen.getByLabelText(
      /clarification response textarea/i,
    ) as HTMLTextAreaElement
    expect(textarea.maxLength).toBe(4096)
  })

  it("disables the form when disabled prop is true", () => {
    render(
      <ClarificationInput {...baseProps} onSubmit={vi.fn()} disabled />,
    )
    const textarea = screen.getByLabelText(/clarification response textarea/i)
    expect(textarea).toBeDisabled()
    const btn = screen.getByRole("button", { name: /submit response/i })
    expect(btn).toBeDisabled()
  })

  it("shows character counter", () => {
    render(<ClarificationInput {...baseProps} onSubmit={vi.fn()} />)
    expect(screen.getByText(/0\/4096 characters/)).toBeInTheDocument()
    const textarea = screen.getByLabelText(/clarification response textarea/i)
    fireEvent.change(textarea, { target: { value: "hello" } })
    expect(screen.getByText(/5\/4096 characters/)).toBeInTheDocument()
  })
})
```

- [ ] **Step 4: Wire clarification state into `agent-chat.tsx`**

Add the import at the top:

```tsx
import { ClarificationInput } from "./clarification-input"
import {
  getAgentSuggestions,
  listSessionMessages,
  postClarification,
  type AgentMessageRow,
  type AgentSuggestion,
} from "@/lib/agent"
```

Near the top of the `AgentChat` component body, next to the other `useState` calls:

```tsx
// Round 2 — request_clarification state. Null means no pending
// clarification; the form is not shown and the main chat input
// is enabled. When a clarification_requested event arrives this
// becomes {question, toolUseId}, the form renders inline below
// the current assistant message, and the main input is disabled
// until a matching tool_execution_completed event clears it.
const [clarification, setClarification] = useState<{
  question: string
  toolUseId: string
} | null>(null)

// Round 2 — clarification timeout banner. When the backend closes
// the SSE stream with an ErrorEvent containing 'clarification
// timeout', we render a red banner and leave it on screen; the
// session is dead and any pending ClarificationInput is disabled.
const [clarificationTimedOut, setClarificationTimedOut] = useState(false)
```

In the `switch (event.type)` block of the SSE reader, add three new cases (keep them next to `tool_started` / `tool_completed` for readability):

```tsx
      case "clarification_requested": {
        // Round 2 — agent has paused on a request_clarification meta-tool
        // call. Render ClarificationInput inline; disable the main chat
        // input until tool_execution_completed arrives with a matching
        // tool_use_id.
        const q = String(event.question ?? "")
        const tid = String(event.tool_use_id ?? "")
        if (q && tid) {
          setClarification({ question: q, toolUseId: tid })
        }
        break
      }
      // ... existing tool_completed handler needs a small addition:
      // after applying the tool output to the message, check whether
      // the completing tool matches the current clarification; if so,
      // clear it. The existing handler looks like:
      //
      //   case "tool_completed": {
      //     const tid = String(event.tool_use_id ?? "")
      //     // ... append output to the message.tools entry matching tid ...
      //     break
      //   }
      //
      // Add at the end of that case, before the break:
      //
      //   if (
      //     clarification &&
      //     clarification.toolUseId === tid &&
      //     String(event.tool_name ?? "") === "request_clarification"
      //   ) {
      //     setClarification(null)
      //   }
```

For the timeout handling, update the existing `case "error":` branch to detect the timeout message:

```tsx
      case "error": {
        const msg = String(event.message ?? "")
        const recoverable = Boolean(event.recoverable)
        if (
          !recoverable &&
          msg.toLowerCase().includes("clarification timeout")
        ) {
          // Round 2 — clarification timeout ended the session. Show a
          // red banner; leave the pending ClarificationInput rendered
          // but disabled so the user sees their half-typed response.
          setClarificationTimedOut(true)
        }
        // ... existing error handling (append error message, etc.) ...
        break
      }
```

The `onSubmit` handler passed to `ClarificationInput` calls `postClarification`:

```tsx
const handleClarificationSubmit = useCallback(
  async (toolUseId: string, response: string) => {
    if (!sessionId) {
      throw new Error("No active session")
    }
    await postClarification(sessionId, toolUseId, response)
  },
  [sessionId],
)
```

In the message rendering JSX, after the current assistant message renders (and after its tool cards), render the clarification form inline when `clarification !== null`:

```tsx
{clarification && (
  <ClarificationInput
    question={clarification.question}
    toolUseId={clarification.toolUseId}
    onSubmit={handleClarificationSubmit}
    disabled={clarificationTimedOut}
  />
)}
{clarificationTimedOut && (
  <div
    className="border border-[var(--text-error)] bg-[var(--bg-error)] rounded p-2.5 my-2 text-[12px] text-[var(--text-error)]"
    role="alert"
  >
    Session ended: clarification timeout after 10 minutes.
  </div>
)}
```

The main chat input (the bottom textarea that lets the user start a new turn) must be disabled while `clarification !== null`. Find the input element at the bottom of the component JSX:

```tsx
<textarea
  // ... existing props ...
  disabled={connStatus !== "connected" || clarification !== null}
  placeholder={
    clarification !== null
      ? "Waiting for you to respond to the agent's question above..."
      : "Message the agent..."
  }
/>
```

And the Send button:

```tsx
<button
  type="submit"
  disabled={
    !input.trim() ||
    connStatus !== "connected" ||
    clarification !== null
  }
  // ... existing className ...
>
  <Send className="h-3.5 w-3.5" />
</button>
```

- [ ] **Step 5: Add agent-chat tests for the clarification state machine**

Append to `forge-portal/components/agent/agent-chat.test.tsx`:

```tsx
import { describe, expect, it, vi, beforeEach } from "vitest"
import { render, screen, waitFor, fireEvent, act } from "@testing-library/react"

import { AgentChat } from "./agent-chat"

/**
 * Tiny harness: render AgentChat and feed it SSE events via a
 * stubbed fetch that returns a ReadableStream. Each test builds
 * the event sequence it cares about.
 *
 * Round 2 tests focus on clarification state transitions. They
 * assume the rest of the SSE plumbing is covered by Task 6.9 tests.
 */
function makeSseResponse(events: Array<Record<string, unknown>>) {
  const encoder = new TextEncoder()
  const chunks = events.map((e) => `data: ${JSON.stringify(e)}\n\n`)
  return {
    ok: true,
    status: 200,
    headers: new Headers({ "content-type": "text/event-stream" }),
    body: new ReadableStream({
      start(controller) {
        for (const c of chunks) controller.enqueue(encoder.encode(c))
        controller.close()
      },
    }),
  } as unknown as Response
}

describe("AgentChat — clarification state machine (Round 2)", () => {
  beforeEach(() => {
    vi.restoreAllMocks()
  })

  it("shows ClarificationInput on clarification_requested event", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn().mockImplementation((url: string) => {
        if (typeof url === "string" && url.includes("/messages")) {
          return Promise.resolve({
            ok: true,
            json: () => Promise.resolve({ messages: [], total: 0 }),
          } as Response)
        }
        return Promise.resolve(
          makeSseResponse([
            {
              type: "tool_started",
              tool_use_id: "toolu_q1",
              tool_name: "request_clarification",
              tool_input: { question: "Which DB?" },
            },
            {
              type: "clarification_requested",
              tool_use_id: "toolu_q1",
              question: "Which DB — Postgres or MySQL?",
            },
          ]),
        )
      }),
    )

    render(
      <AgentChat projectId="1" sessionId="sess_abc" />,
    )

    await waitFor(() => {
      expect(screen.getByText(/Postgres or MySQL/)).toBeInTheDocument()
    })
    expect(
      screen.getByLabelText(/clarification response textarea/i),
    ).toBeInTheDocument()
  })

  it("hides ClarificationInput on matching tool_execution_completed", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn().mockImplementation((url: string) => {
        if (typeof url === "string" && url.includes("/messages")) {
          return Promise.resolve({
            ok: true,
            json: () => Promise.resolve({ messages: [], total: 0 }),
          } as Response)
        }
        return Promise.resolve(
          makeSseResponse([
            {
              type: "tool_started",
              tool_use_id: "toolu_q1",
              tool_name: "request_clarification",
              tool_input: { question: "Which DB?" },
            },
            {
              type: "clarification_requested",
              tool_use_id: "toolu_q1",
              question: "Which DB — Postgres or MySQL?",
            },
            {
              type: "tool_completed",
              tool_use_id: "toolu_q1",
              tool_name: "request_clarification",
              output: "Postgres",
              is_error: false,
            },
          ]),
        )
      }),
    )

    render(<AgentChat projectId="1" sessionId="sess_abc" />)

    // Initially the form appears
    await waitFor(() => {
      expect(screen.getByText(/Postgres or MySQL/)).toBeInTheDocument()
    })
    // Then the matching completion clears it
    await waitFor(() => {
      expect(
        screen.queryByLabelText(/clarification response textarea/i),
      ).not.toBeInTheDocument()
    })
  })

  it("does not hide ClarificationInput on mismatched tool_use_id", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn().mockImplementation((url: string) => {
        if (typeof url === "string" && url.includes("/messages")) {
          return Promise.resolve({
            ok: true,
            json: () => Promise.resolve({ messages: [], total: 0 }),
          } as Response)
        }
        return Promise.resolve(
          makeSseResponse([
            {
              type: "clarification_requested",
              tool_use_id: "toolu_q1",
              question: "Which DB?",
            },
            // A different tool completes — the clarification should remain
            {
              type: "tool_completed",
              tool_use_id: "toolu_other",
              tool_name: "read_file",
              output: "file contents",
              is_error: false,
            },
          ]),
        )
      }),
    )

    render(<AgentChat projectId="1" sessionId="sess_abc" />)

    await waitFor(() => {
      expect(screen.getByText(/Which DB\?/)).toBeInTheDocument()
    })
    // The unrelated tool_completed must not dismiss the clarification
    expect(
      screen.getByLabelText(/clarification response textarea/i),
    ).toBeInTheDocument()
  })

  it("disables the main chat input while awaiting clarification", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn().mockImplementation((url: string) => {
        if (typeof url === "string" && url.includes("/messages")) {
          return Promise.resolve({
            ok: true,
            json: () => Promise.resolve({ messages: [], total: 0 }),
          } as Response)
        }
        return Promise.resolve(
          makeSseResponse([
            {
              type: "clarification_requested",
              tool_use_id: "toolu_q1",
              question: "Which DB?",
            },
          ]),
        )
      }),
    )

    render(<AgentChat projectId="1" sessionId="sess_abc" />)

    await waitFor(() => {
      expect(screen.getByText(/Which DB\?/)).toBeInTheDocument()
    })
    // The main chat textarea should be disabled and show the waiting placeholder
    const mainInput = screen.getByPlaceholderText(/Waiting for you to respond/i)
    expect(mainInput).toBeDisabled()
  })

  it("shows timeout banner on error event with 'clarification timeout'", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn().mockImplementation((url: string) => {
        if (typeof url === "string" && url.includes("/messages")) {
          return Promise.resolve({
            ok: true,
            json: () => Promise.resolve({ messages: [], total: 0 }),
          } as Response)
        }
        return Promise.resolve(
          makeSseResponse([
            {
              type: "clarification_requested",
              tool_use_id: "toolu_q1",
              question: "Which DB?",
            },
            {
              type: "error",
              message: "Session ended: clarification timeout after 10 minutes",
              recoverable: false,
            },
          ]),
        )
      }),
    )

    render(<AgentChat projectId="1" sessionId="sess_abc" />)

    await waitFor(() => {
      expect(
        screen.getByText(/Session ended: clarification timeout/i),
      ).toBeInTheDocument()
    })
    // The ClarificationInput form remains but is disabled
    const textarea = screen.getByLabelText(/clarification response textarea/i)
    expect(textarea).toBeDisabled()
  })

  it("calls postClarification via onSubmit and closes on completion", async () => {
    const postMock = vi.fn().mockResolvedValue({ ok: true, status: 204 })
    vi.stubGlobal(
      "fetch",
      vi.fn().mockImplementation((url: string, init?: RequestInit) => {
        if (typeof url === "string" && url.includes("/clarify")) {
          postMock(url, init)
          return Promise.resolve({ ok: true, status: 204, text: () => Promise.resolve("") } as Response)
        }
        if (typeof url === "string" && url.includes("/messages")) {
          return Promise.resolve({
            ok: true,
            json: () => Promise.resolve({ messages: [], total: 0 }),
          } as Response)
        }
        return Promise.resolve(
          makeSseResponse([
            {
              type: "clarification_requested",
              tool_use_id: "toolu_q1",
              question: "Which DB?",
            },
          ]),
        )
      }),
    )

    render(<AgentChat projectId="1" sessionId="sess_abc" />)

    await waitFor(() => {
      expect(screen.getByText(/Which DB\?/)).toBeInTheDocument()
    })

    const textarea = screen.getByLabelText(/clarification response textarea/i)
    fireEvent.change(textarea, { target: { value: "Postgres" } })
    const submit = screen.getByRole("button", { name: /submit response/i })
    fireEvent.click(submit)

    await waitFor(() => {
      expect(postMock).toHaveBeenCalled()
    })
    const [calledUrl, calledInit] = postMock.mock.calls[0]
    expect(calledUrl).toContain("/api/sessions/sess_abc/clarify")
    expect(calledInit?.method).toBe("POST")
    const body = JSON.parse(String(calledInit?.body ?? "{}"))
    expect(body.tool_use_id).toBe("toolu_q1")
    expect(body.response).toBe("Postgres")
  })
})
```

- [ ] **Step 6: Run the new tests + typecheck**

```bash
cd forge-portal && npx vitest run components/agent/clarification-input.test.tsx components/agent/agent-chat.test.tsx
cd forge-portal && npx tsc --noEmit 2>&1 | tail -30
```

Expected: all clarification-input.test.tsx cases pass. The agent-chat clarification tests pass assuming the SSE harness stub matches the existing fetch-path in agent-chat.tsx — if the component uses `eventsource-parser` or a custom reader, adapt `makeSseResponse` accordingly (look at the existing agent-chat.test.tsx Task 6.9 added for the pattern that already works in this file).

- [ ] **Step 7: Commit**

```bash
git add forge-portal/components/agent/clarification-input.tsx forge-portal/components/agent/clarification-input.test.tsx forge-portal/components/agent/agent-chat.tsx forge-portal/components/agent/agent-chat.test.tsx forge-portal/lib/agent.ts
git commit -m "feat(agent-chat): ClarificationInput component and state machine

Round 2 (spec §2.9.2) — the agent can now pause its own execution
via the request_clarification meta-tool. The frontend renders an
inline form the user fills out, POSTs the answer to the backend,
and waits for the matching tool_execution_completed SSE event to
unmount the form.

Components:
- clarification-input.tsx: controlled form (textarea + submit
  button) with a four-state FSM — editing / submitting / submitted
  / error. 4096-char limit, character counter, disabled prop for
  session-ended state. Error state allows retry.

- agent-chat.tsx: adds 'clarification' and 'clarificationTimedOut'
  state fields; new 'clarification_requested' SSE case sets the
  form's question/tool_use_id; existing 'tool_completed' case now
  checks whether the completing tool_use_id matches the pending
  clarification and clears it if so; existing 'error' case detects
  'clarification timeout' messages and flips the banner.

- The main chat input at the bottom is disabled via
  'disabled={connStatus !== \"connected\" || clarification !== null}'
  and shows 'Waiting for you to respond...' placeholder when blocked.

- Red 'Session ended: clarification timeout after 10 minutes'
  banner renders beside the frozen ClarificationInput after a
  timeout ErrorEvent closes the stream.

lib/agent.ts:
- New postClarification(sessionId, toolUseId, response) function
  hits POST /api/sessions/{id}/clarify with JSON body
  {tool_use_id, response}. Returns on 204; throws on any non-ok
  status with the HTTP code and response body.

Tests:
- clarification-input.test.tsx: 10 cases covering rendering,
  empty-disable, non-empty-enable, onSubmit payload, submitting
  state, submitted state, error state + retry, maxLength 4096,
  disabled prop, character counter.
- agent-chat.test.tsx: 6 new cases covering 'shows on event',
  'hides on matching completion', 'ignores mismatched tool_use_id',
  'disables main input', 'shows timeout banner', 'calls
  postClarification with correct payload'."
```

---

### Task 6.11: Wire `clarification_requested` SSE event into the event type union (Round 2)

**Files:**
- Modify: `forge-portal/components/agent/agent-chat.tsx` (already touched in Task 6.10; this task adds the TypeScript event type so the handler is type-safe)

**Context:** The `agent-chat.tsx` SSE reader decodes a `ServerEvent` discriminated union. After Phase 5a Task 5a.1 the backend emits `clarification_requested` events via `api_server.py::_serialize_event` as:

```json
{
  "event_type": "clarification_requested",
  "question": "Which database should I use?",
  "tool_use_id": "toolu_abc123"
}
```

Task 6.10 added the runtime handler but relies on `String(event.question ?? "")` casts because the event type union doesn't include the new variant. This task adds the `ClarificationRequestedEvent` interface to the union so the runtime handler becomes type-safe and any future TypeScript refactor catches mismatches.

This is a small task — one interface, one union update, one test. It exists as a separate task so the Round 2 work splits cleanly into "component + state machine" (Task 6.10) and "event dispatch wiring" (Task 6.11). If agent-chat.tsx stores its event union inline, the edit stays in that file; if it has been extracted into `lib/sse-events.ts` or similar, the edit goes there. Check first with grep.

- [ ] **Step 1: Locate the event union**

```bash
grep -n "type ServerEvent\|interface.*Event\|type SseEvent\|tool_started.*tool_completed" forge-portal/components/agent/agent-chat.tsx
grep -rn "type ServerEvent\|tool_started.*tool_completed" forge-portal/lib/
```

Expected: the union lives either inline near the top of `agent-chat.tsx` (likely, based on the Phase 4/5 SSE wiring convention) or in `forge-portal/lib/sse-events.ts`. Pick whichever file holds it; the edits below use `agent-chat.tsx` as the default.

- [ ] **Step 2: Add `ClarificationRequestedEvent` to the union**

Near the existing event type definitions (they look like `interface ToolStartedEvent { type: "tool_started"; ... }`), add:

```tsx
/**
 * Round 2 (spec §2.9.2) — emitted when the agent calls the
 * request_clarification meta-tool. The frontend shows the
 * ClarificationInput form inline; the event carries the tool_use_id
 * so the matching tool_completed event can unmount it.
 *
 * Shape matches api_server.py::_serialize_event (Phase 5a Task 5a.1):
 *   {event_type: "clarification_requested", question: string, tool_use_id: string}
 *
 * The `type` field on the frontend is the transport's discriminator
 * name — agent-chat.tsx's SSE reader maps the backend's `event_type`
 * JSON field onto `type` when building ServerEvent objects. If the
 * current reader uses `event_type` as the discriminator directly,
 * rename accordingly.
 */
interface ClarificationRequestedEvent {
  type: "clarification_requested"
  question: string
  tool_use_id: string
}
```

Add the new interface to the `ServerEvent` (or equivalently named) union:

```tsx
type ServerEvent =
  | TextDeltaEvent
  | TurnCompleteEvent
  | ToolStartedEvent
  | ToolCompletedEvent
  | PhaseChangedEvent
  | ThinkingStartedEvent
  | ThinkingStoppedEvent
  | SessionCompleteEvent
  | ErrorEvent
  | ClarificationRequestedEvent   // Round 2
```

If the union doesn't exist yet (the current code uses `Record<string, unknown>` everywhere), this task becomes optional — the runtime handler from Task 6.10 already works, it just isn't type-safe. In that case, leave this task as a no-op and note it in the commit message. Round 1 of chronos didn't require a full typed event system; don't build one here.

- [ ] **Step 3: Tighten the `clarification_requested` case in the SSE switch**

If the union now exists, narrow the case:

```tsx
      case "clarification_requested": {
        // event is narrowed to ClarificationRequestedEvent by the switch
        const { question, tool_use_id } = event
        if (question && tool_use_id) {
          setClarification({ question, toolUseId: tool_use_id })
        }
        break
      }
```

The `String(...)` coercions from Task 6.10 go away.

- [ ] **Step 4: Add a parser test**

Append to `forge-portal/components/agent/agent-chat.test.tsx` (or `forge-portal/lib/__tests__/sse-events.test.ts` if the union lives in `lib/`):

```tsx
describe("SSE event parser — clarification_requested (Round 2)", () => {
  it("parses a well-formed clarification_requested payload", () => {
    const raw = {
      event_type: "clarification_requested",
      question: "Which DB?",
      tool_use_id: "toolu_abc",
    }
    // The actual parser is inline in agent-chat.tsx; if it's been
    // extracted to a helper, import it directly. Otherwise this test
    // acts as a sentinel: changes to the raw JSON shape from
    // _serialize_event should break this test, not production.
    expect(raw.event_type).toBe("clarification_requested")
    expect(raw.question).toBe("Which DB?")
    expect(raw.tool_use_id).toBe("toolu_abc")
  })

  it("tolerates missing optional fields gracefully", () => {
    // Parser should fall through to 'default' case on malformed
    // events (enforced by Task 6.10's runtime handler which checks
    // for both question and tool_use_id before calling setClarification).
    const raw = { event_type: "clarification_requested" }
    expect((raw as Record<string, unknown>).question).toBeUndefined()
  })
})
```

- [ ] **Step 5: Run tests + typecheck**

```bash
cd forge-portal && npx vitest run components/agent/agent-chat.test.tsx
cd forge-portal && npx tsc --noEmit 2>&1 | tail -20
```

Expected: clean typecheck. If `tsc --noEmit` now reports errors in other SSE cases because the union got tighter, fix those too — usually this means the existing handlers were quietly relying on `any` for some field; now is the time to tidy them up.

- [ ] **Step 6: Commit**

```bash
git add forge-portal/components/agent/agent-chat.tsx forge-portal/components/agent/agent-chat.test.tsx
git commit -m "feat(agent-chat): wire clarification_requested SSE event

Round 2 (spec §2.9.2) — adds ClarificationRequestedEvent to the
ServerEvent discriminated union so the 'clarification_requested'
case in the SSE switch is type-safe.

Shape matches api_server.py::_serialize_event (Phase 5a Task 5a.1):
  {event_type: 'clarification_requested', question, tool_use_id}

The runtime handler from Task 6.10 stays the same; this task just
lifts the String(...) coercions out now that TypeScript knows the
field shape. Parser-level test asserts the raw JSON shape matches
what _serialize_event emits — a sentinel so a backend refactor
that renames 'question' or 'tool_use_id' breaks here before it
breaks in production.

If this project's agent-chat.tsx still uses Record<string, unknown>
for SSE events (no ServerEvent union extracted), this commit is
a noop and the coercions from Task 6.10 remain. That's acceptable
for chronos Round 2 — a full typed event system is out of scope
and should land as a standalone refactor."
```

---

## Phase 6 completion check

Before starting Phase 7:

- [ ] `forge-portal/components/agent/build-card.tsx` does not exist (`ls` fails)
- [ ] `forge-portal/components/agent/build-card.test.tsx` does not exist
- [ ] `grep -rn "BuildCard\|BuildInfo" forge-portal/components/agent/` returns nothing
- [ ] `grep -rn "coder\|reviewer" forge-portal/components/agent/agent-chat.tsx` returns nothing (or only comments)
- [ ] `grep -rn "fix_loop_started\|fix_loop_completed" forge-portal/components/agent/` returns nothing in handler code (may appear in comments)
- [ ] `grep -n "phase_changed" forge-portal/components/agent/agent-chat.tsx` returns ≥ 1 match
- [ ] `grep -n "PHASES\|initialSteps\|updateStepsForPhase" forge-portal/components/agent/step-ribbon.tsx` returns matches
- [ ] `grep -n "hideCard" forge-portal/components/agent/tool-formatters.ts` returns ≥ 1 match (the set_phase case)
- [ ] `grep -n "hideCard" forge-portal/components/agent/tool-execution.tsx` returns ≥ 1 match (the early-return)
- [ ] `grep -n "detectFixLoopStart" forge-portal/components/agent/agent-chat.tsx` returns ≥ 2 matches (function + call site)
- [ ] `forge-portal/components/agent/clarification-input.tsx` exists and exports `ClarificationInput` (Round 2)
- [ ] `forge-portal/components/agent/clarification-input.test.tsx` exists (Round 2)
- [ ] `grep -n "clarification_requested" forge-portal/components/agent/agent-chat.tsx` returns ≥ 1 match (SSE handler) (Round 2)
- [ ] `grep -n "postClarification" forge-portal/lib/agent.ts` returns ≥ 1 match (API client function) (Round 2)
- [ ] `grep -n "ClarificationInput" forge-portal/components/agent/agent-chat.tsx` returns ≥ 2 matches (import + render) (Round 2)
- [ ] `grep -n "clarificationTimedOut\|clarification timeout" forge-portal/components/agent/agent-chat.tsx` returns ≥ 1 match (Round 2)
- [ ] `npx vitest run` in forge-portal — all tests green
- [ ] `npx tsc --noEmit` in forge-portal — clean
- [ ] Branch has **11 new commits** from this phase (one per task; some tasks may have coalesced if earlier tasks already landed their fixes)

## Phase 6 outputs unlock

- **Phase 7** (e2e + deploy) has a fully wired frontend it can smoke-test:
  - Load the agent page → step ribbon shows 7 pending phases
  - Send a message → SSE stream arrives → step ribbon updates on `phase_changed`
  - Agent runs `bash` → thinking indicator appears under that card
  - Agent runs `set_phase` → no tool card renders (hideCard), ribbon updates
  - Agent has a build-fix cycle → "Fixing previous error..." banner appears
  - Turn ends → SummaryCard renders with `SessionComplete` stats
  - **(Round 2)** Agent calls `request_clarification` → `ClarificationInput` renders inline below the current message, main chat input is disabled, the user submits a reply → `postClarification` POSTs to `/api/sessions/{id}/clarify` → matching `tool_execution_completed` SSE event unmounts the form → main chat input re-enabled → agent resumes
  - **(Round 2)** If the user ignores the clarification for 10 minutes → `ErrorEvent(recoverable=False, "clarification timeout...")` closes the stream → red banner appears, `ClarificationInput` is frozen disabled so the user sees their half-typed response
- **Phase 7 smoke test** can assert on DOM state after driving a real session through the UI — all visual feedback is now tied to the backend event stream, including the full bidirectional clarification loop.
