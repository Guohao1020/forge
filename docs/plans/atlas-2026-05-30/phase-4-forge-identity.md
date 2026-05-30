## Phase 4 — Forge Identity Layer

**Goal:** 在 Multica base 上叠加 Forge 身份：保留 Multica 许可证/归属/前端 LOGO（许可证限制），
重写 README 与根 CLAUDE.md 为 Forge 愿景，做最小前端共标，落正式 Windows dev runbook。

**Depends-on:** Phase 3（文档已迁入新 main）
**Unblocks:** Phase 5
**Completion gate:** LICENSE+NOTICE 保留并注明 Forge 衍生；README/CLAUDE.md 反映 Forge=Multica+Harness；
前端共标保留 Multica 归属；runbook 写好。各项提交。

> 许可证约束（改良 Apache 2.0）：**叠加不替换** —— 不得移除 Multica console 的 LOGO/版权；
> 内部标识符保持 `multica` 以降低 upstream 合并冲突。

---

### Task 4.1: 许可证与归属

**Files:**
- Keep: `LICENSE`（Multica 原文，不改）
- Create/Modify: `NOTICE`

- [ ] **Step 1: 确认 LICENSE 原样保留**

Run:
```bash
cd <forge-repo-path>
head -5 LICENSE
```
Expected: 仍是 Multica 的改良 Apache 2.0（来自 upstream，未被改动）。

- [ ] **Step 2: 写/补 NOTICE 注明 Forge 衍生**

在 `NOTICE` 追加（若无则创建，保留 Multica 既有 NOTICE 内容）：
```
Forge — Harness Engineering layer on top of Multica.
This project is a derivative of Multica (https://github.com/multica-ai/multica),
licensed under Multica's modified Apache License 2.0. All Multica copyright and
license notices are retained. Forge adds an engineering-discipline layer
(specs / constraints / verification / entropy management) above Multica.
```

- [ ] **Step 3: 提交**

Run:
```bash
git add NOTICE
git commit -m "docs(license): retain Multica license; add Forge derivative NOTICE"
```

---

### Task 4.2: 重写 README

**Files:**
- Modify: `README.md`

- [ ] **Step 1: 在顶部加 Forge 区块（保留 Multica 归属链接）**

在 README 顶部插入 Forge 定位段，并保留指向 Multica 的归属：
```markdown
# Forge — Harness Engineering on Multica

Forge 让不懂代码的产品/运营用自然语言描述需求，AI 在 **Harness 环境**
（规范约束 + 机械化验证 + 可观测反馈）中生成生产级代码。

**核心理念：规范即灵魂。**

Forge 以开源平台 [Multica](https://github.com/multica-ai/multica) 为底座
（managed-agents 平台：任务看板、daemon 驱动 CLI 执行、Skills、Squad、Autopilot），
在其上叠加工程纪律层。下方为 Multica 原始 README。

---
```
保留原 Multica README 全文于该区块之下。

- [ ] **Step 2: 提交**

Run:
```bash
git add README.md
git commit -m "docs(readme): add Forge positioning above Multica README"
```

---

### Task 4.3: 重写根 CLAUDE.md

**Files:**
- Modify: `CLAUDE.md`（Multica 自带的根 CLAUDE.md）

- [ ] **Step 1: 合并为 Forge 版**

把根 `CLAUDE.md` 改写为反映新现实，至少含：
- Forge=Multica+Harness 的定位与两层架构说明
- 新技术栈（Go Chi+sqlc / Next.js / Postgres / daemon 驱动 claude；**Temporal/ai-worker 已退役**）
- 文档规范：沿用 Forge 既有的 Plan Directory Convention（`<DOCDIR>/plans/{greek}-{date}/`）与
  spec 落点（`<DOCDIR>/specs/`）
- 语言规范：保留 Forge 全局三层语言约定
- 保留 Multica 原 CLAUDE.md 中仍适用的工程约定（monorepo / sqlc / store 位置等），其余标注更新

- [ ] **Step 2: 提交**

Run:
```bash
git add CLAUDE.md
git commit -m "docs(claude): rewrite root CLAUDE.md for Forge-on-Multica architecture"
```

---

### Task 4.4: 最小前端共标（保留 Multica LOGO/版权）

**Files:**
- Modify: 前端品牌位（如 `apps/web` 的 header/sidebar 品牌组件、`<title>`、favicon 区）

- [ ] **Step 1: 定位品牌渲染点**

Run:
```bash
grep -rin "Multica" apps/web/app apps/web/components packages/ui 2>/dev/null | grep -iE 'title|brand|logo|header' | head
```
Expected: 找到 console 顶部品牌/标题渲染处。

- [ ] **Step 2: 叠加 Forge 标识，不动 Multica 归属**

在品牌位加 Forge 产品名（如标题 `Forge · Harness Engineering on Multica`，或 sidebar 加一行
"Forge layer"），**保留** Multica 的 LOGO 与版权信息（许可证限制 b）。改 `<title>`/metadata 为 Forge。

- [ ] **Step 3: 浏览器验证**

起服务（Phase 2 命令）后浏览器查看 console。
Expected: 顶部同时可见 Forge 产品标识与保留的 Multica LOGO/版权；页面标题为 Forge。截图存档。

- [ ] **Step 4: 提交**

Run:
```bash
git add apps/web
git commit -m "feat(brand): co-brand Forge alongside retained Multica logo/copyright"
```

---

### Task 4.5: 落正式 Windows dev runbook

**Files:**
- Create: `<DOCDIR>/dev-windows.md`

- [ ] **Step 1: 把 `_atlas_notes.md` 的可行路径转成 runbook**

写 `<DOCDIR>/dev-windows.md`：选定的 dev 路径（WSL2/Docker）、先决条件安装、
完整起停命令序列（env → db-up → dev → login → daemon → 闭环 → stop）、踩坑与解法、
确定性登录（`MULTICA_DEV_VERIFICATION_CODE=888888`）。

- [ ] **Step 2: 提交**

Run:
```bash
git add <DOCDIR>/dev-windows.md
git commit -m "docs(dev): Windows local dev runbook for Forge-on-Multica"
git push origin main
```

---

## Phase 4 完成检查

- [ ] LICENSE 原样保留；NOTICE 注明 Forge 衍生
- [ ] README 顶部 Forge 定位 + 保留 Multica 归属
- [ ] 根 CLAUDE.md 重写为 Forge-on-Multica
- [ ] 前端最小共标，保留 Multica LOGO/版权，浏览器验证 + 截图
- [ ] Windows dev runbook 落档
- [ ] 各项已提交并 push
