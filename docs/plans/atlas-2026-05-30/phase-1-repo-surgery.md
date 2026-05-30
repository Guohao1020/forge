## Phase 1 — Repo Surgery

**Goal:** 把 `Guohao1020/forge` 的 `main` 从旧 forge 代码就地改造成 Multica base：归档旧代码、
重置 main 到 `upstream/main`、配置 remote、push。

**Depends-on:** Phase 0（dev 路径已证明可行，再做不可逆改动）
**Unblocks:** Phase 2, Phase 3
**Completion gate:** `main` = Multica base（descends from `upstream/main`），旧代码完整保留在
`archive/forge-legacy`，`upstream` remote 已配，`origin/main` 已 force-push 更新。

> ⚠️ 本 phase 含唯一不可逆动作（force-push origin/main）。Task 1.1 先做仓库外备份，
> 且旧代码会进 `archive/forge-legacy` 分支 —— 双保险，安全可逆。

---

### Task 1.1: 仓库外备份规划文档

> 重置 main 后，`docs/specs/` 与 `docs/plans/` 会离开工作树（进 archive 分支）。
> 执行器需要持续读到本计划 —— 先复制到仓库外。

**Files:**
- Create: `D:\shulex_work\_atlas_planning_backup\`（仓库外）

- [ ] **Step 1: 复制 specs 与 plans 到仓库外**

Run (PowerShell):
```powershell
New-Item -ItemType Directory -Force D:\shulex_work\_atlas_planning_backup
Copy-Item -Recurse -Force D:\shulex_work\forge\docs\specs   D:\shulex_work\_atlas_planning_backup\specs
Copy-Item -Recurse -Force D:\shulex_work\forge\docs\plans   D:\shulex_work\_atlas_planning_backup\plans
```
Expected: 备份目录含 `specs\2026-05-30-forge-on-multica-f0-foundation-design.md` 与
`plans\atlas-2026-05-30\`。

- [ ] **Step 2: 校验备份完整**

Run (PowerShell):
```powershell
Get-ChildItem -Recurse D:\shulex_work\_atlas_planning_backup | Measure-Object | Select-Object Count
```
Expected: Count ≥ 8（spec 1 + plan 7）。

---

### Task 1.2: 配置 upstream remote 并 fetch

**Files:**
- Modify: `D:\shulex_work\forge\.git\config`（git remote 配置）

- [ ] **Step 1: 加 upstream 并 fetch**

Run:
```bash
cd D:/shulex_work/forge
git remote add upstream https://github.com/multica-ai/multica.git
git fetch upstream
```
Expected: fetch 到 `upstream/main`（如已在 Phase 前手动加过 `github`/`upstream`，先 `git remote -v` 核对，避免重复）。

- [ ] **Step 2: 核对 remote 与上游 tip**

Run:
```bash
git remote -v
git log --oneline -1 upstream/main
```
Expected: `upstream` 指向 multica-ai/multica；`origin` 指向 Guohao1020/forge；显示 upstream/main tip。

---

### Task 1.3: 归档旧 forge 代码

- [ ] **Step 1: 当前 main 建归档分支**

Run:
```bash
cd D:/shulex_work/forge
git checkout main
git branch archive/forge-legacy main
```
Expected: 创建 `archive/forge-legacy`，指向当前旧 forge tip（含 spec/plan 提交）。

- [ ] **Step 2: 推归档分支到两个 remote**

Run:
```bash
git push origin archive/forge-legacy
git push codeup archive/forge-legacy
```
Expected: 两个 remote 都有 `archive/forge-legacy`（codeup 若不存在则跳过该行）。

- [ ] **Step 3: 校验归档分支含旧代码**

Run:
```bash
git ls-tree --name-only archive/forge-legacy | grep -E 'forge-core|ai-worker|forge-portal'
```
Expected: 列出 `forge-core` / `ai-worker` / `forge-portal` —— 旧代码确在归档分支。

---

### Task 1.4: 重置 main 到 Multica base

- [ ] **Step 1: 硬重置 main 到 upstream/main**

Run:
```bash
cd D:/shulex_work/forge
git checkout main
git reset --hard upstream/main
```
Expected: `main` 工作树变为 Multica 内容（出现 `server/`、`apps/`、`packages/`、`pnpm-workspace.yaml`）。

- [ ] **Step 2: 校验 main 现在等于 Multica base**

Run:
```bash
git log --oneline -1 main
git log --oneline -1 upstream/main
git ls-files | grep -E '^(server/|apps/|packages/)' | head
```
Expected: main tip == upstream/main tip；工作树是 Multica 树。

---

### Task 1.5: Force-push main 到 origin

- [ ] **Step 1: 安全 force-push**

Run:
```bash
git push --force-with-lease origin main
```
Expected: `origin/main` 更新为 Multica base。`--force-with-lease` 校验远端未被他人改动。
（旧代码已在 `archive/forge-legacy` + codeup，安全。）

- [ ] **Step 2: 校验 origin/main**

Run:
```bash
git fetch origin
git log --oneline -1 origin/main
```
Expected: origin/main == main == upstream/main。

---

### Task 1.6: 标注归档关系（提交到新 main）

> 新 main 是纯 Multica 树，还没有任何 Forge 痕迹。留一个最小的"归档指针"提交，
> 让人能找到旧代码（完整文档迁移在 Phase 3）。

**Files:**
- Create: `D:\shulex_work\forge\ARCHIVE.md`

- [ ] **Step 1: 写归档指针**

写入 `ARCHIVE.md`：
```markdown
# Legacy Forge (pre-Multica)

旧 forge 架构（Go forge-core + Python ai-worker + Temporal + Next.js forge-portal）
已于 2026-05-30 归档到分支 `archive/forge-legacy`（origin 与 codeup 均有）。

本 main 自该日起以 Multica（https://github.com/multica-ai/multica）为新底座（fork/rebase），
Forge 作为其上的 Harness 工程层。详见 F0 设计 spec 与 atlas 计划（Phase 3 迁移后入 docs/）。

取回旧代码：`git checkout archive/forge-legacy`
```

- [ ] **Step 2: 提交**

Run:
```bash
git add ARCHIVE.md
git commit -m "chore(forge): rebase main onto Multica base; archive legacy to archive/forge-legacy"
```
Expected: 新 main 上第一个 Forge 提交（在 Multica base 之上）。

- [ ] **Step 3: push**

Run:
```bash
git push origin main
```
Expected: origin/main 含该提交。

---

## Phase 1 完成检查

- [ ] 规划文档已备份到仓库外 `_atlas_planning_backup`
- [ ] `upstream` remote 配好，`origin`/`codeup` 核对无误
- [ ] 旧代码完整保留在 `archive/forge-legacy`（origin + codeup）
- [ ] `main` = Multica base，descends from `upstream/main`，可 `git merge upstream/main`
- [ ] `origin/main` 已更新；`ARCHIVE.md` 归档指针已提交
