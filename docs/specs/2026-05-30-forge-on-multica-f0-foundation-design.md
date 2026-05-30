# Forge on Multica · F0 Foundation Design

> Design spec — 2026-05-30
>
> **Topic:** 以开源项目 [Multica](https://github.com/multica-ai/multica)
> 为新底座（fork/rebase）重建 Forge：Multica 提供 managed-agents 平台基础设施，
> Forge 在其上叠加 Harness Engineering 工程纪律层。本 spec 只覆盖 **F0 基座** 切片。
>
> **Scope:** 仅 **F0 — 基座立起来**。后续子项目（F1 规范中心=Skills、F2 验证门禁、
> F3 AI Review、F4 熵管理=Autopilot、F5 可观测闭环）明确 out of scope，各自走独立 spec。
>
> **Engineering standard:** 硅谷级基建标准。不妥协、不 hardcode 特例、不拿正则当安全边界、
> one code path。沿用 Forge 既有的自主执行与工业级完整性要求。

---

## 0. 战略背景

### 0.1 Multica 是什么

Multica 是"开源的 managed agents 平台 —— 把编码 agent 变成真正的队友：分配 issue、
追踪进度、复利化技能"。技术栈与 Forge 高度一致：

| 层 | Multica | Forge（旧） |
|----|---------|------------|
| 前端 | Next.js 16 + React 19 | Next.js 16 + React 19 |
| 后端 | Go（Chi + sqlc + gorilla/websocket） | Go（Gin）+ Python（LangGraph）+ Temporal |
| 数据库 | PostgreSQL 17 | PostgreSQL 多 schema |
| Agent 执行 | 本地 daemon 驱动现成 CLI（Claude Code/Codex/…）在隔离 execenv 里跑 | 自建 Python LangGraph agent loop（ai-worker，14 工具 + bwrap 沙箱） |

### 0.2 核心洞察

把系统拆成两层，Forge 与 Multica 的关系一目了然：

- **第一层 — 平台/编排基础设施**（任务看板、agent 当队友、任务队列、runtime/daemon 执行、
  多租户隔离、实时流、CLI、Squad、Autopilot、Skills）→ **Multica 已做得又全又成熟**，
  远超 Forge 旧状态。
- **第二层 — Harness 工程价值**（规范即灵魂：规范中心、约束引擎/Lint 门禁、AI Review、
  熵管理、四层测试、可观测闭环）→ **Multica 完全没有**。这正是 Forge 的独特价值。

Forge 的 Harness 概念能干净地映射到 Multica 原语上，不是硬塞：

| Forge Harness 概念 | Multica 原语 |
|---|---|
| 规范中心（规范即灵魂） | **Skills**（SKILL.md + 文件，agent 绑定，可复利） |
| 熵管理（定期质量扫描 + 自动修复） | **Autopilot**（cron 定时建 issue 给 agent） |
| 约束引擎 / 四层测试 / 质量门禁 | **任务完成门禁 + 验证 hook**（卡 `/complete`） |
| AI Review | **Squad 里的 Reviewer agent** |
| 项目画像 | workspace / project settings |
| 可观测性 | task messages/usage + 质量指标扩展 |

结论：**Forge 的价值 = 给 Multica 装上工程纪律。**

### 0.3 可行性确认

验证门禁能挂上去——Claude Code 本身有 hooks（SessionStart 等），daemon 的 execenv
在 report complete 前可跑验证，Multica 的 `/api/daemon/tasks/{id}/complete` 端点也能加门禁。
thesis 成立，留待 F2 落地。

---

## 1. 决策链（gating decisions）

本 spec 基于以下 5 个已锁定的决策（brainstorming 2026-05-30）：

| # | 决策 | 选择 | 理由 |
|---|------|------|------|
| D1 | 战略定位 | **以 Multica 为新底座（fork/rebase）** | 最贴合"基于这个项目把 forge 构建起来"；平台层起点高 |
| D2 | Agent 执行模型 | **拥抱 CLI 驱动，退役自建 loop + Temporal** | 单一执行路径（one code path）；Claude Code CLI 当执行器，harness 作纪律层 |
| D3 | 第一切片 | **F0 基座** | 其他所有切片的前置 |
| D4 | 分发模式 | **Forge 开源** | 化解许可证最大限制（对第三方付费 SaaS）；继承改良 Apache 2.0 |
| D5 | 仓库策略 | **就地改造 `Guohao1020/forge`**，旧代码进 archive 分支，加 multica upstream remote | 保持一个 forge 仓库身份；能跟上游 |

### 1.1 许可证约束（来自 D4）

Multica 用改良版 Apache 2.0，两条附加限制：

- **(a) 托管/嵌入服务限制**：未经书面授权，不得对第三方提供托管 SaaS 或嵌入到商业产品。
  *单一组织内部使用（含多 workspace）不受限。* → Forge 开源自托管，此条基本 moot。
- **(b) LOGO/版权限制**：使用 Multica 前端时不得移除/修改其 console 的 LOGO 和版权信息。
  → 品牌只能**叠加 Forge 身份 + 保留 Multica 归属**，不能替换。

---

## 2. F0 目标 & 完成定义（DoD）

把 `Guohao1020/forge` 变成 **Forge-on-Multica 开源底座**：Multica 全功能本地跑通、
Forge 身份叠加、旧架构退役、能跟上游。验收硬标准：

- [ ] **仓库**：`main` = Multica base（descends from `upstream/main`），旧 forge 代码在
  `archive/forge-legacy` 分支，`upstream` remote 配好，force-push 到 origin
- [ ] **本地 e2e 闭环**：平台服务起来 → 登录 → 建 workspace → daemon 连上并检测到本地
  `claude` → 建 issue → assign 给 Claude Code agent → agent 在本地 runtime 执行并回报
  → issue 进 review。**整条 Multica 闭环在用户机器上跑通。**
- [ ] **身份**：README/CLAUDE.md 重写为 Forge 愿景；LICENSE + Multica 归属保留；价值文档迁移
- [ ] **退役**：旧 forge-core / ai-worker / Temporal / constraint-worker 等不再运行（仅存档）
- [ ] **runbook**：Windows 本地开发文档

---

## 3. 仓库改造（精确动作）

```bash
git remote add upstream https://github.com/multica-ai/multica.git
git fetch upstream
git branch archive/forge-legacy main          # 归档旧 forge（当前 566a117）
git push origin archive/forge-legacy
git checkout main
git reset --hard upstream/main                 # main 重置到 Multica，使其可跟上游
git push --force-with-lease origin main        # 旧代码已在 archive 分支，安全
```

之后：

- Forge 改动 = Multica `main` 之上的新 commit。
- 跟进上游 = `git fetch upstream && git merge upstream/main`。
- `codeup` remote 保留不动（旧 forge 仍在那）；archive 分支也推一份到 codeup 做双备份。

> **注意**：force-push `origin/main` 前，旧代码已在 `archive/forge-legacy` 分支 + codeup，
> 安全可逆。

---

## 4. 迁移 vs 归档

- **归档**（留在 `archive/forge-legacy`，不进新 main）：forge-core / ai-worker /
  constraint-worker / devops-worker / forge-bot / forge-portal / forge-foundation 全部旧代码。
- **迁移进新 main**（放独立目录，避免破坏 Multica 的 Fumadocs 文档站构建——具体落点执行时核对，
  候选 `docs/forge/` 或顶层 `planning/`）：
  - PRD.md、technical-design.md、product-design.md、milestone-plan.md
  - references/coding-standards.md（← **F1 规范中心的种子内容**）
  - DESIGN.md
  - docs/specs/（含本 spec）、docs/plans/（chronos 等历史计划，标注 superseded）
- **重写**：根 CLAUDE.md（合并 Multica + Forge，反映新架构与文档规范）、
  README（Forge = Multica + Harness）。

---

## 5. Forge 身份层（受许可证约束）

- **许可证**：保留 Multica LICENSE（改良 Apache 2.0）+ NOTICE，Forge 作为衍生沿用；
  加 README 段说明 Forge 基于 Multica 及归属。
- **前端品牌**：**叠加不替换** —— 保留 Multica console 的 LOGO/版权（限制 b），旁边加
  Forge 产品标识。F0 只做最小共标，重 UI 改造留给后续切片。
- **内部标识符**：**保持 `multica`**（包名 / 模块 / CLI 命令 / 数据库），降低 upstream
  合并冲突。"Forge" 只活在产品 / 文档层 + 共标。
- **CLAUDE.md**：新架构说明 + 沿用既有文档规范（Plan Directory Convention 等）。

---

## 6. 本地开发环境（Windows）—— 首要验证项

用户在 Windows 11，而 Multica 的 `make dev` 栈是 Unix 取向（make + pnpm + Go 1.26 +
Docker，daemon 检测 `claude` CLI；execenv / repocache / GC 是 Unix 取向）。

- **推荐主路径：WSL2 Ubuntu** 跑整个 Multica dev 栈（`make dev` 原生 Linux），`claude`
  CLI 装进 WSL2，daemon 在 WSL2 内检测到它。规避 Windows 路径 / 隔离地雷。
- **备选**：平台服务用 Docker（`docker-compose.selfhost.yml`）+ daemon 原生跑 —— 但
  daemon 目录隔离 / repocache 在 Windows 上有风险。
- **执行第一步就实测哪条路跑得通**，再定稿 runbook（写进迁移后的文档目录）。

---

## 7. 退役旧架构

- 停掉旧 `docker-compose.dev.yml`（Postgres / Redis / Temporal / code-server）——
  Multica 自带 DB / Redis 设置。
- 旧服务二进制 / Python 环境不再构建；Temporal 概念移除。
- 更新项目记忆：标注 A2 / chronos 自建 loop 已被 Multica CLI 驱动取代。

---

## 8. 验收 & 测试

- **手动 e2e**：跑 §2 DoD 的闭环，截图 / 日志为证。
- **Multica 自带 CI + Playwright e2e** 在 fork 上应跑绿（至少不被 F0 改动弄坏）。
- F0 无新业务逻辑，**不写新功能测试**（基座切片）。

---

## 9. 风险

| | 风险 | 缓解 |
|---|---|---|
| R1 | Windows dev（最大） | WSL2 兜底；执行第一步实测 |
| R2 | 未来 upstream 合并冲突 | 保持内部 `multica` 标识符 + Forge 改动隔离在独立文件 / 目录 |
| R3 | 许可证前端归属 | 已按"叠加保留"处理；商业化时再评估 |
| R4 | force-push `origin/main` | 旧代码已在 `archive/forge-legacy` 分支 + codeup，安全 |

---

## 10. 后续子项目（占位，各自走独立 spec）

| 子项目 | 内容 | 映射 Multica |
|---|---|---|
| **F1 规范中心 = Skills** | 编码规范 / 项目画像做成 workspace skills，注入 Claude Code 上下文 | Skills 系统 |
| **F2 验证门禁** | task complete 前跑约束引擎（lint / Semgrep / 架构测试 / 四层测试），不过不放行 | 完成门禁 + hook |
| **F3 AI Review** | Reviewer agent 评审生成代码 | Squad |
| **F4 熵管理** | 定时质量扫描 + 自动修复 issue + 趋势追踪 | Autopilot |
| **F5 可观测闭环** | 质量指标 dashboard + 运行时反馈 | task usage + 扩展 |

> F0 完成后，下一切片预计为 **F1 规范中心 = Skills**（Harness thesis 的第一个价值落点）。
