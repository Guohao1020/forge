# Forge F1 · 规范中心（Standards）设计

> Design spec — 2026-05-31
>
> **Topic:** 在 Multica skills 之上重建 Forge 的"规范中心"——把分类/分级的编码规范
> （Standards）解析成"有效规范"，**双层注入**到 Claude Code agent：核心规则常驻
> instructions（强制），详细规则编译成按需 skill。落地"规范即灵魂"。
>
> **Scope:** 仅 **F1 — 规范中心（Standards）**。Review Rules（→F2 验证门禁）、
> Prompt Templates（作废）、Scaffolds（后置）、profile 自动扫描（后置）、跨 workspace
> org 级（不做）均明确 out of scope。
>
> **Engineering standard:** 硅谷级基建。one code path、不 hardcode、不拿正则当安全边界。
> Forge 代码全部隔离在 `forge_` 前缀，最小化对 Multica 上游的侵入（R2）。

---

## 0. 背景

F1 是 Forge-on-Multica 路线图的第二切片（F0 基座已完成，见
`docs/forge/specs/2026-05-30-forge-on-multica-f0-foundation-design.md`）。

**张力**：Multica 已有 Skills 系统（SKILL.md 文件式注入，agent 原生按需发现）。Forge 旧
S5 Spec Center 是为旧自建 agent 设计的（`{{coding_standards}}` 变量塞进 prompt），其注入
机制随旧 agent 作废。F1 要在 Claude Code（自带 skills）之上重建规范治理。

**两个注入杠杆**（来自 F0 对 Multica 的探查）：
- `agent.instructions` → 直接进 system prompt（强制、常驻）。
- skills → 写成 workdir 的 provider 原生文件（`.claude/skills/{name}/SKILL.md`），
  agent 原生发现、渐进式按需加载。

## 1. 决策链（brainstorming 2026-05-31）

| # | 决策 | 选择 |
|---|------|------|
| 1 | 野心 | **结构层**：在 skills 之上重建规范治理（分类/分级 + EffectiveSpecs 解析） |
| 2 | 注入语义 | **双层**：核心进 instructions（常驻强制）+ 详细进自动绑定 skill（按需） |
| 3 | 规范类型 | **只治 Standards**（Review Rules→F2，Prompt Templates 作废，Scaffolds 后置） |
| 4 | 分级 scope | **workspace → project**（2 级，project 覆盖；team/company 无 Multica 归宿，丢弃） |
| 5 | 选择性适用 | **项目画像驱动过滤**（project 手动设 profile 标签，按技术栈过滤适用规范） |

## 2. 核心概念 & 数据流

一个 **standard** = 分类(category) + 画像标签(profile_tags) + **core_content**（强制核心
规则，简短）+ **detail_content**（详细指引）。

任务派发时（daemon claim）解析有效规范并双层注入：

```
issue assign → daemon claim
   → forge.ResolveStandards(workspaceID, projectID)
       = workspace 级 standards（project_id NULL）
         被 project 级（project_id=P）按 (category,name) 覆盖
         ⨯ 按 project profile_tags 过滤（tags 交集非空，或 standard tags 空=全适用）
   → 拆两层：
       core_content   → 合进该任务 brief 的 instructions（常驻强制）
       detail_content → 合成一条 forge-standards skill
   → daemon 写 .claude/skills/forge-standards/SKILL.md（复用现有 execenv 机制）
   → Claude Code：核心规范常驻 system prompt；详细规范按需读取
```

**关键巧思**：detail-skill 走 Multica **现有**的 skill 注入（execenv 已会写 skill 文件，
**daemon 零改动**）；只有 core→instructions 需要一个服务端小钩子（§4）。

## 3. 数据模型

Forge 独立 sidecar 表，前缀 `forge_`，**不碰 Multica 的 `project`/`skill` 表**（upstream
合并干净）：

```sql
CREATE TABLE forge_standard (
  id            UUID PRIMARY KEY,
  workspace_id  UUID NOT NULL REFERENCES workspace(id) ON DELETE CASCADE,
  project_id    UUID REFERENCES project(id) ON DELETE CASCADE,  -- NULL=workspace级, 有值=project级覆盖
  name          TEXT NOT NULL,
  category      TEXT NOT NULL,            -- naming/api/sql/go/testing/...
  profile_tags  TEXT[] NOT NULL DEFAULT '{}',  -- 适用画像；空=全适用
  core_content  TEXT NOT NULL DEFAULT '', -- 强制核心（进 instructions）
  detail_content TEXT NOT NULL DEFAULT '',-- 详细（进 skill）
  enabled       BOOLEAN NOT NULL DEFAULT TRUE,
  created_by    UUID,
  created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
-- 覆盖键唯一性：同 scope 内 (category,name) 唯一
CREATE UNIQUE INDEX uq_forge_standard_ws  ON forge_standard(workspace_id, category, name) WHERE project_id IS NULL;
CREATE UNIQUE INDEX uq_forge_standard_proj ON forge_standard(project_id, category, name)   WHERE project_id IS NOT NULL;
CREATE INDEX idx_forge_standard_ws ON forge_standard(workspace_id);

CREATE TABLE forge_project_profile (
  project_id  UUID PRIMARY KEY REFERENCES project(id) ON DELETE CASCADE,
  tags        TEXT[] NOT NULL DEFAULT '{}',  -- 技术栈画像（手动设）
  updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
```

迁移文件放 Multica 的迁移目录（`server/migrations/`），编号续上游最大号之后（F0 时上游
到 `110_*`，实现时复查实际最大号，如 `111_forge_standards.up.sql`），文件名带 `forge_` 标识。

## 4. 解析 + 双层注入（核心）

### 4.1 解析（`server/internal/forge/standards/resolve.go`）

```
Resolve(ctx, workspaceID, projectID) -> Resolved{ Core string, DetailSkill SkillData }
  1. 取 workspace 级 standards（project_id NULL, enabled）
  2. 取 project 级 standards（project_id=P, enabled），按 (category,name) 覆盖 workspace 级
  3. 取 project profile tags（无则空）
  4. 过滤：保留 standard.profile_tags 为空 OR 与 project tags 有交集 的
  5. 拼 Core = 各 standard.core_content 按 category 分组的 markdown
     拼 DetailSkill = SKILL.md（frontmatter name: forge-standards + 各 detail_content）
```

纯函数 + repository 注入，便于单测。

### 4.2 注入钩子（唯一侵入 Multica 的点）

在 claim 响应构建处（`server/internal/handler/daemon.go` 组装 `TaskAgentData` 的位置）
加**一行**调用：

```go
forge.InjectStandards(ctx, resp.Agent, task.WorkspaceID, task.ProjectID)
```

`InjectStandards`（在 `forge` 包内）：
- 把 `Resolved.Core` 追加进 `resp.Agent.Instructions`（仅本次任务响应，**不写回
  agent.instructions 持久字段**，不覆盖用户编辑）。
- 把 `Resolved.DetailSkill` 追加进 `resp.Agent.Skills`（execenv 现有逻辑自动落盘为
  `.claude/skills/forge-standards/SKILL.md`，**daemon 零改动**）。

幂等、可降级：标准为空 → core 空串、不追加 skill；解析出错 → log warning 并跳过（不阻断任务）。

## 5. API + UI

- **API**（`server/internal/handler/forge_standards.go`，sqlc queries + workspace 中间件隔离）：
  - `GET/POST /api/forge/standards`、`GET/PUT/DELETE /api/forge/standards/{id}`
  - `GET/PUT /api/forge/projects/{id}/profile`
- **UI**（`packages/views/forge-standards/`，沿用 Multica 的 Base UI/shadcn + TanStack Query）：
  - 规范列表（按 category / scope 筛选）
  - core / detail 双栏 markdown 编辑器 + profile_tags 多选
  - project 设置页：profile 标签编辑
- Web 路由 `apps/web/app/[workspaceSlug]/(dashboard)/forge-standards/`（参照 Multica 现有页面接线）。

## 6. Upstream 合并隔离策略（R2）

- 全部 Forge 代码：`forge_` 前缀表 · `server/internal/forge/` 包 · `server/internal/handler/forge_*.go` ·
  `/api/forge/*` 路由 · `packages/views/forge-standards/`。
- **唯一侵入点**：daemon.go claim 处一行 `forge.InjectStandards(...)` + 路由注册几行。
  upstream 改这些文件时仅这几行可能冲突，易解。
- sqlc：Forge 查询放独立 `.sql` 文件（`server/pkg/db/queries/forge_standards.sql`），
  `make sqlc` 一并生成。

## 7. 测试

- **Go 单测**：`resolve_test.go` —— project 覆盖 workspace、profile 过滤（交集/空标签）、
  core/detail 拆分、空规范降级。
- **集成测试**：claim 注入 —— 喂 fixtures（workspace+project standards + profile），
  断言响应 instructions 含 core、Skills 含 forge-standards。
- 沿用 Multica `go test` + 测试 DB fixtures 模式。F1 无前端复杂交互，前端测试最小化。

## 8. 边界（F1 不做）

Review Rules / 验证 / lint（→F2）· Prompt Templates（作废）· Scaffolds（后置）·
profile 自动扫描仓库（手动设）· 跨 workspace org 级 · standard 版本化/审批工作流（YAGNI，后置）。

## 9. 风险

| | 风险 | 缓解 |
|---|---|---|
| R1 | core 规范过长撑大每个 prompt 的 token | core_content 约定简短（强制裁剪）；详细内容放 detail-skill |
| R2 | upstream 改 daemon claim 处致合并冲突 | 侵入点压到一行函数调用 + 逻辑全在 forge 包 |
| R3 | 无法用真 agent 端到端验证（F0 遗留的 provider 凭证阻塞） | 解析/注入逻辑用单测+集成测试覆盖；真 agent 验证待凭证就绪 |
| R4 | project profile 字段 Multica 没有 | sidecar 表 `forge_project_profile`，不动 Multica project 表 |

> F1 完成后，下一切片 **F2 验证门禁**（约束引擎卡任务完成）。
