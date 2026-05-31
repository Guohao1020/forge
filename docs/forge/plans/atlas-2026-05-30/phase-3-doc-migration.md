## Phase 3 — Doc Migration & Archive Reconciliation

**Goal:** 把旧 forge 的价值文档从 `archive/forge-legacy` 迁回新 main，放到不破坏 Multica
文档站（Fumadocs）构建的独立位置；标注被取代的历史计划。

**Depends-on:** Phase 1（archive 分支存在，新 main 是 Multica）
**Unblocks:** Phase 4
**Completion gate:** PRD/technical-design/coding-standards/DESIGN/specs/plans 已迁入新 main 的
独立目录；Multica 文档站构建不受影响；提交并 push。

---

### Task 3.1: 勘定迁移落点（不破坏 Multica docs 构建）

- [ ] **Step 1: 查 Multica docs/ 结构与构建方式**

Run:
```bash
cd <forge-repo-path>
ls docs/
cat docs/package.json 2>/dev/null | head -30 || true
grep -rl "fumadocs\|contentlayer\|mdx" docs/ apps/ | head
```
Expected: 判明 `docs/` 是否被某个 app 当内容源构建（Fumadocs）。结论决定落点。

- [ ] **Step 2: 定落点目录**

决策规则：
- 若 `docs/` 被 Fumadocs 当内容源 → Forge 规划文档放 **顶层 `forge-docs/`**（不进 docs/ 内容树），
  规避 MDX 构建对未知 markdown 报错。
- 若 `docs/` 不参与构建 → 放 `docs/forge/`。

把选定落点写进 `_atlas_notes.md`。下文以 `<DOCDIR>` 指代该目录。

---

### Task 3.2: 从归档分支迁移价值文档

- [ ] **Step 1: 取回文档到落点**

Run（`<DOCDIR>` 替换为 3.1 选定目录）:
```bash
cd <forge-repo-path>
mkdir -p <DOCDIR>
git checkout archive/forge-legacy -- docs/PRD.md docs/technical-design.md docs/product-design.md docs/milestone-plan.md docs/DESIGN.md
git checkout archive/forge-legacy -- docs/references docs/specs docs/plans
# 把取回的内容归拢到 <DOCDIR>（git checkout 会落在原 docs/ 路径，需移动）
```
Expected: 工作树出现旧文档。若落点不是 `docs/`，用 `git mv` 移到 `<DOCDIR>` 下并保持子结构。

- [ ] **Step 2: 移动到落点并核对结构**

Run:
```bash
# 示例（落点 forge-docs/）：
git mv docs/PRD.md docs/technical-design.md docs/product-design.md docs/milestone-plan.md docs/DESIGN.md <DOCDIR>/ 2>/dev/null || true
git mv docs/references docs/specs docs/plans <DOCDIR>/ 2>/dev/null || true
ls -R <DOCDIR> | head -40
```
Expected: `<DOCDIR>` 下含 PRD、technical-design、references/coding-standards.md、
specs/2026-05-30-…、plans/atlas-2026-05-30/ 等。

- [ ] **Step 3: 确认本计划与 spec 已在新 main**

Run:
```bash
ls <DOCDIR>/specs/2026-05-30-forge-on-multica-f0-foundation-design.md
ls <DOCDIR>/plans/atlas-2026-05-30/index.md
```
Expected: 两者都在 —— 计划文档已随迁移回到新 main（与仓库外备份一致）。

---

### Task 3.3: 标注被取代的历史计划

- [ ] **Step 1: 给旧计划加 superseded 抬头**

在 `<DOCDIR>/plans/` 下的旧计划（chronos-2026-04-09、pair_pipeline、S1-S17 等）目录/文件顶部，
加一行：
```markdown
> ⚠️ SUPERSEDED 2026-05-30 — 旧 Go+Python+Temporal 架构的计划，仅作需求/设计参考。
> Forge 已 rebase 到 Multica 底座，见 specs/2026-05-30-forge-on-multica-f0-foundation-design.md。
```
Expected: 历史计划清楚标注已被取代，避免误执行。

- [ ] **Step 2: 验证 Multica 文档站仍能构建（若适用）**

Run（仅当 3.1 判定 docs/ 参与构建）:
```bash
make check    # 或针对 docs app 的 build；以实际命令为准
```
Expected: 文档站构建不因迁入文档报错（迁文档已避开 docs/ 内容树则天然不受影响）。

---

### Task 3.4: 提交迁移

- [ ] **Step 1: 提交**

Run:
```bash
git add <DOCDIR> ARCHIVE.md
git commit -m "docs(forge): migrate legacy PRD/design/specs/plans into <DOCDIR> from archive"
git push origin main
```
Expected: 新 main 含迁移后的 Forge 文档；origin 同步。

---

## Phase 3 完成检查

- [ ] 迁移落点已勘定，不破坏 Multica 文档构建
- [ ] PRD/technical-design/coding-standards/DESIGN/specs/plans 已迁入 `<DOCDIR>`
- [ ] 本 atlas 计划与 F0 spec 确认在新 main
- [ ] 历史计划标注 superseded
- [ ] 已提交并 push
