# Forge Design System — Dense Engineering

**Authoritative source:** `~/.gstack/projects/voc-shulex-forge/designs/agent-terminal-shotgun-20260406/variant-B-dense.html`
**Approved:** 2026-04-06 via design shotgun (`approved.json`)
**Aesthetic:** Cursor / VS Code. 40px header, 12px body, 4px radius, compact IDE density.

This document is the bridge between the static mockup (HTML, 1397 lines) and the
implementation (React, Tailwind 4). When mockup and code disagree, mockup wins —
with the one exception documented under **WCAG corrections** below.

## Brand

- **Light accent:** `#2563eb` (`--accent`) with hover `#1d4ed8`, subtle `#eff6ff`, text `#1e40af`
- **Dark accent:** `#3b82f6` with hover `#60a5fa`, subtle `#1e3a5f`, text `#93c5fd`
- **No purple.** The old "深空指挥中心 / Forge Purple #8B5CF6" brand is retired. Residue in
  `app/(dashboard)/specs/prompts/page.tsx`, `components/forge-logo.tsx`, `lib/tasks.ts`,
  and a few others is tracked in `TODOS.md` under "Brand Cleanup".
- **Logo treatment:** 20×20 square, `--accent` fill, 3px radius, white lightning glyph (mockup line 151-157)

## Typography

- **Body:** Inter, 12px, line-height 1.4 (mockup lines 62, 117-120)
- **Code:** JetBrains Mono, 11px in UI chrome (tool labels, gutters), 10-11px inline code
- **Tailwind class vs mockup token:**

| Mockup | Tailwind | `--text-*` | Use |
|---|---|---|---|
| 9px | `text-tiny` | `--text-tiny` | msg-model badge, code-gutter, meta chrome |
| 10px | `text-meta` | `--text-meta` | msg-time, tool-status, code-breadcrumb |
| 11px | `text-label` | `--text-label` | msg-name, tool-label, step, code-view |
| 12px | `text-body` / default | `--text-body` | msg-text (body copy), headers |

**Do not use `text-sm` (14px) anywhere in Agent Terminal chrome.** 14px breaks the IDE density.
`text-sm` is still fine for login page, settings forms, and high-density reading panels
where 12px would feel cramped.

Body line-height is forced to 1.4 in `@layer base` (`globals.css`). Tailwind's default 1.5
is too airy for the Cursor/VS Code look.

## Colors — Token System

All colors come from CSS variables in `globals.css` under `:root` (light) and `.dark` (dark).
The mockup uses `[data-theme="dark"]`; we map to `.dark` via `ThemeToggle`.

### Backgrounds — 3 layers + semantic bands

| Token | Light | Dark | Use |
|---|---|---|---|
| `--bg-primary` | `#ffffff` | `#1e1e1e` | Page body, code content |
| `--bg-secondary` | `#f8f9fa` | `#252526` | Header, ribbon, sidebar, chat scroll |
| `--bg-tertiary` | `#f1f3f5` | `#2d2d2d` | Tool card header, raised chrome |
| `--bg-input` | `#ffffff` | `#2d2d2d` | Text inputs, search |
| `--bg-hover` | `#f1f3f5` | `#333333` | Button hover, step hover |
| `--bg-active` | `#e9ecef` | `#3c3c3c` | Active nav, pressed buttons |
| `--bg-code` | `#f8f9fa` | `#1e1e1e` | Inline `<code>` background |
| `--bg-tool` | `#f8f9fb` | `#252526` | Tool card body |
| `--bg-error` | `#fff5f5` | `#2d1b1b` | Error banners, bad build |
| `--bg-success` | `#f0fdf4` | `#1b2d1e` | Pass build, completed step |
| `--bg-warning` | `#fffbeb` | `#2d2a1b` | System messages, fix loop |
| `--bg-info` | `#eff6ff` | `#1b2233` | Info banners |
| `--bg-badge` | `#e9ecef` | `#3c3c3c` | Model badge pills |

### Text — 3 visual levels + semantic bands

| Token | Light | Dark | Use |
|---|---|---|---|
| `--text-primary` | `#1a1a1a` | `#cccccc` | Body copy, headers |
| `--text-secondary` | `#495057` | `#9d9d9d` | Labels, secondary info |
| `--text-tertiary` | `#6b7280` *★* | `#9ca3af` *★* | Meta, placeholders, timestamps |
| `--text-inverse` | `#ffffff` | `#1e1e1e` | On accent backgrounds |
| `--text-code` | `#d6336c` | `#ce9178` | Inline code color |
| `--text-link` | `#2563eb` | `#3b82f6` | Hyperlinks, file paths in tool cards |
| `--text-error` | `#c92a2a` | `#f87171` | Errors, fail states |
| `--text-success` | `#2b8a3e` | `#4ade80` | Success, pass states |
| `--text-warning` | `#e67700` | `#fbbf24` | Warnings, fix loop status |

### Borders

| Token | Light | Dark |
|---|---|---|
| `--border-primary` | `rgba(0,0,0,0.12)` | `rgba(255,255,255,0.12)` |
| `--border-secondary` | `rgba(0,0,0,0.08)` | `rgba(255,255,255,0.06)` |
| `--border-focus` | `#2563eb` | `#3b82f6` |

### ★ WCAG corrections

**`--text-tertiary` is darkened vs the mockup.** The mockup specifies `#868e96` on `#ffffff`,
which measures 3.4:1 contrast ratio — below WCAG AA's 4.5:1 threshold for small text. We ship
`#6b7280` (4.6:1) in light mode and `#9ca3af` on `#1e1e1e` (4.9:1) in dark mode. The visual
hierarchy of msg-time and code-gutter is slightly heavier than the mockup; this is an
acceptable trade for accessibility compliance. Do not restore the mockup values.

## Spacing

Tailwind 4 derives fractional spacing utilities from a single `--spacing: 0.25rem` base,
so `p-0.25` (1px), `p-0.5` (2px), `p-0.75` (3px), and `p-1.25` (5px) all work out of the
box without any `@theme` extension. Variant B's 1-5px gaps map cleanly.

| Class | Size | Purpose |
|---|---|---|
| `gap-0.25` / `p-0.25` | 1px | msg-header, tool-status border |
| `gap-0.5` / `p-0.5` | 2px | step-ribbon gap, tool-label spacing |
| `gap-0.75` / `p-0.75` | 3px | tool-body scrollbar track |
| `p-1.25` | 5px | tight card padding |
| `p-1` | 4px | default step padding, tool-header padding |
| `p-2` | 8px | tool-header horizontal, message gap |
| `p-2.5` | 10px | ribbon/header horizontal padding |
| `p-3` | 12px | (avoid — too large for chrome) |

Stock Tailwind sizes cover 90% of the mockup. No custom spacing tokens needed.

## Shell Layout

Agent Terminal is a 4-row CSS Grid (mockup lines 128-135):

```
┌─────────── header      (40px, --bg-secondary) ───────────┐
│ logo  title  task      $1.24/42k  [btn] [btn] [btn] [btn]│
├─────────── ribbon      (40px, --bg-secondary) ───────────┤
│ Analyze → Plan → Generate → Build → Test → Review → Deploy│
├─────────── main        (1fr, 3-col grid 1fr|1px|1fr) ────┤
│        chat panel      │ divider │      code panel       │
├─────────── status bar  (20px, --bg-secondary) ───────────┤
│ ● Build idle  Step 4/7  feat/auth  java  |  5 err  $1.24 │
└───────────────────────────────────────────────────────────┘
```

CSS vars `--header-h`, `--ribbon-h`, `--status-h` expose these heights.
`--input-h: 44px` is the fixed chat textarea row (sticks to chat panel bottom).

## Component Library

### Header

- `h: 40px`, `bg: --bg-secondary`, `border-bottom: 1px solid --border-primary`
- Left cluster: 20×20 logo (accent bg, 3px radius, white lightning) + `Forge Agent` (12px semibold, --text-primary) + 1×14px separator + `TASK-2847` (11px mono, --text-secondary) + task-meta pill (10px mono in --bg-tertiary, 3px radius)
- Right cluster: meta pill `$1.24 / 42k tok` + 26×26 icon buttons (Terminal, Code panel, Theme, Settings). Active button: `--accent-subtle` bg + `--accent` color.

### Step Ribbon

- `h: 40px`, `bg: --bg-secondary`, `gap: 2px`, `overflow-x: auto` (scrollbar hidden)
- Each step: 4×8 padding, 11px medium, `--text-tertiary` default
- States: `completed` → `--text-success`, `active` → `--accent` bg + `--accent-subtle` + border, `error` → `--text-error`
- **Step connector:** `w: 10px, h: 1px, bg: --border-primary`. NOT an arrow `→` character.
- **No glow effects.** The old `shadow-[0_0_8px_var(--accent-glow)]` is removed. Dense Engineering has no glow.

### Messages (`.msg`)

- `gap: 6px` between avatar and body
- Avatar 18×18, 3px radius. Variants: `ai` (accent bg, white F), `user` (--bg-tertiary bg, initial), `sys` (--bg-warning bg, ! icon)
- Message header: `msg-name` (11px semibold primary) + `msg-time` (10px mono tertiary) + optional `msg-model` pill (9px mono in --bg-badge, 2px radius)
- Message text: 12px / 1.4 / `--text-primary`
- Inline code: 11px mono, `--bg-code` bg, 2px radius, `--text-code` color

### Tool Cards (`.tool-card`)

- Border-radius: 4px (--radius). Border always `--border-primary`. **State is never encoded in the border color.**
- Header: `--bg-tertiary` bg with `border-bottom: 1px solid --border-secondary`
- Tool label: 11px mono medium `--text-secondary` + 12×12 tool icon
- **State badge** on right: `.tool-status.ok` (--text-success on --bg-success), `.err` (--text-error on --bg-error), `.running` (--accent-text on --accent-subtle)
- Body: `--bg-tool`, 11px mono `--text-secondary`, `max-height: 120px`, 3px scrollbar thumb
- Linked file paths: `--text-link`, cursor pointer, hover underline

### Build Card

- Border-color = status color (`--text-error` for fail, `--text-success` for pass)
- Header bg: `--bg-error` / `--bg-success`
- Lines below show stdout/stderr. Error lines highlighted with `2px solid --text-error` left border.

### Code Panel

- Tab bar: 4px radius, language icon (colored dot), filename, close × on hover
- Breadcrumb row: 10px mono `--text-tertiary` on `--bg-secondary` with `›` separators (`--border-primary`)
- Gutter: 36px min-width, `--bg-secondary`, right-border `--border-secondary`, 10px mono line numbers, 16.5px line-height
- Code view: 11px mono, 16.5px line-height (rows are exactly 16.5px tall)
- Diff highlights: added `rgba(46,160,67,0.1)`, removed `rgba(248,81,73,0.1)`, error `rgba(201,42,42,0.1)` with 2px left border
- Minimap: 40px wide, `--bg-secondary`, `--border-secondary` left border
- **Syntax highlighting:** Shiki (VS Code grammars) — NOT handcrafted spans. See Stream 2 for rewrite.

### Status Bar

- `h: 20px`, `bg: --bg-secondary`, `border-top: 1px solid --border-primary`
- Left: status dot (`--text-success/warning/error`) + Build state + Step cur/max + branch + language
- Right: errors count + model + tokens + cost + Ln/Col
- All text: 10px mono `--text-tertiary`

## Interactions

- **Transitions:** `0.1s` for background/color (fast, IDE snap). Never 200-300ms; it feels sluggish in dense UIs.
- **Hover states:** background shift to `--bg-hover`, NOT a transform or glow
- **Active states:** `--bg-active` or `--accent-subtle` for navigational items
- **Focus:** `--border-focus` (= `--accent`) outline, 2px
- **Scrollbars:** 4-6px wide, thumb = `--border-primary`, track transparent. On `.chat-scroll`, `.tool-body`, `.code-content`.
- **Empty states:** CLI-style single-line `→ Try: {suggestion}` (not pill buttons)

## Keyboard Navigation

- Tab order: input → send → messages → tools → code tabs → status bar
- `Esc`: close expanded tool card
- `↑/↓`: navigate message focus
- `Cmd/Ctrl+Enter`: submit chat
- `Cmd/Ctrl+\`: toggle code panel
- Focus trap in modal-like expanded tool panels

## Accessibility Floor

- All interactive elements have `aria-label` or visible text
- `aria-live="polite"` on message list (for streaming token deltas)
- `aria-expanded` / `aria-controls` on all expand/collapse toggles
- `role="tablist"` on code panel tabs, `role="tab"` + `aria-selected` on each tab
- `role="status"` on status bar
- WCAG AA contrast: ≥4.5:1 for text, ≥3:1 for UI components. See **WCAG corrections** above.
- Test with `@axe-core/react` in vitest — component tests assert 0 violations.

## Migration Status

Stream 1 (this commit) ships the tokens, fonts, type scale, and this document.
Legacy tokens (`--surface`, `--text-muted`, `--text-dim`, `--accent-glow`, etc.) remain as
aliases pointing at the new namespaced tokens so existing components keep compiling.
Stream 2 sweeps components to use the new tokens directly and retires the aliases.

## Extending This Document

When adding a new component to the Agent Terminal surface:

1. Check if the mockup has it. If yes, copy the CSS verbatim into the component as
   Tailwind utilities or raw CSS custom properties. Reference the mockup line numbers
   in a code comment.
2. If the mockup doesn't have it, design it in the same idiom: 11-12px text, 4px radius,
   `--border-primary` borders, `--bg-secondary` chrome, `--bg-tertiary` raised surfaces,
   `0.1s` transitions, no glows.
3. Add an entry to this document under **Component Library**.
4. If you need a new token, add it to `globals.css` in both light and dark blocks AND
   document it in the token tables above.

When proposing a deviation from the mockup:

1. Write the reason. "Looks better" is not a reason. "WCAG AA fail" or "mobile breaks at
   375px" or "performance regression" is a reason.
2. Document the deviation in the component's section + a new bullet under **WCAG
   corrections** or a similar heading.
3. Never silently ignore the mockup. If you cannot match it, either fix the code or
   update this document with the trade-off.
