# TODOS

Deferred items from reviews and retros. Format: one-line what + why + context + dependency.

## UI Alignment

- [ ] **Compare `components/agent/build-card.tsx` against Variant B mockup lines 505-554** — red border, `BUILD FAILED` title, `exit code + duration` meta, multi-line log with `.error-line` red highlight. Not yet inspected in 2026-04-07 eng review; Section 2 was already at 5 decisions and build-card was deferred to avoid scope creep. Pick up alongside the `code-panel` shiki rewrite since both touch code-view styling. **Depends on:** Variant B token rename (Section 2.2 finding, `--text-error`/`--bg-error` must exist).

## Brand Cleanup

- [ ] **Sweep the codebase for "深空指挥中心" purple-brand residue** — the old purple brand (`#8B5CF6`, `#7C3AED`, aurora background, "深空指挥中心" visual language, `accent-glow` shadow effects) was replaced by Variant B blue (`#2563eb` light / `#3b82f6` dark, Cursor/VS Code density). Residue already found in `step-ribbon.tsx:30` (shadow-glow) and `CLAUDE.md` Frontend section. Grep candidates:
  - `grep -ri "8B5CF6\|8b5cf6\|7C3AED\|7c3aed" --include="*.tsx" --include="*.ts" --include="*.css" --include="*.md"`
  - `grep -ri "深空\|指挥中心\|aurora\|forge purple\|Forge Purple" .`
  - `grep -ri "accent-glow\|shadow-glow" --include="*.tsx"`
  Update `CLAUDE.md` Frontend section to replace "Brand color: Forge Purple #8B5CF6" and "深空指挥中心" visual language with Variant B ("Dense Engineering, Cursor/VS Code inspired, blue #2563eb/#3b82f6"). Update `docs/product-design.md` brand sections. **Context:** Variant B was approved in `~/.gstack/projects/voc-shulex-forge/designs/agent-terminal-shotgun-20260406/approved.json` on 2026-04-06; design-doc section on brand shift exists. This is documentation debt, not a bug, but inconsistency risks new code drifting back toward purple.
