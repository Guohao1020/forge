# F4 熵管理 = Autopilot · 设计（eunomia）

> Forge-on-Multica 第四切片。把 Harness 的**熵管理层**（定期质量扫描 + 自动建 issue）
> 映射到 Multica 的 **Autopilot** 原语之上。承接 F1 规范中心、F2 验证门禁、F3 AI Review。

**Status:** Approved（2026-06-01 brainstorming）
**Source:** 本 spec 由 brainstorming 决策链驱动，见 §1。
**Plan:** `docs/forge/plans/eunomia-2026-06-01/`（待 writing-plans 产出）。

---

## 1. 决策链（brainstorming 2026-06-01）

| # | 决策 | 选择 |
|---|------|------|
| Q1 | 扫描产出 | **建议性建 issue**（扫描→为每个发现建 issue；自愈/修复留 F4b） |
| Q2 | 扫描维度 | **F1/F2 感知 + 可加自定义**（brief 由 F1 规范 + F2 检查合成，再追加自由聚焦） |
| Q3 | 去重防噪 | **单发现去重 + 注入开放清单**（软去重，无独立去重表） |
| Arch | 与 Autopilot 挂钩 | **方案 A：Forge 全权拥有扫描定义，后台代管 backing Autopilot** |
| 派发模式 | create_issue vs run_only | **create_issue**（issue.description 是 brief 最自然的注入点 + 审计 + 分组） |

**核心洞察**：Multica 的 Autopilot 已经是一个完整的「cron → 建 issue → 派 agent」生产级系统
（调度器 30s 轮询 + 原子 claim、admission gate、run 追踪、失败自动暂停、create_issue/run_only
两模式、issue 模板、squad 解析、完整 CRUD API + UI）。F4 的 Forge 价值**不在重建调度/建 issue**，
而在叠加 **Harness 熵管理语义**：扫什么、brief 如何从 F1+F2 运行时合成、以及去重防噪。

---

## 2. 目标 / 非目标

**目标**
- 让用户为一个 workspace / project 定义**周期性全仓熵扫描**（节奏 = cron）。
- 扫描派发时，自动把 scanner agent 的 brief 合成为：**F1 规范 + F2 检查 + 自定义聚焦 + 开放发现去重清单**。
- scanner agent 全仓巡检，为**新**发现建带 `forge-entropy` 标签的**建议性 issue**，对仍存在的旧发现评论 bump。
- 完整复用 Autopilot 调度/派发，Forge 仅叠加 sidecar 定义 + 一处派发钩子 + backing autopilot 代管。
- 保持 Forge 隔离原则（R2）：`forge_` 前缀、`/api/forge/*`、`packages/views/forge-*`，Multica 改动最小化。

**非目标（本切片不做）**
- **F4b 自愈闭环**：scanner agent 直接修代码 + 开 PR + 自动过 F2 门禁 + F3 评审。建在本切片之上，且需活体凭证。
- 规则引擎 / 严重度分级 / 趋势 dashboard / 跨扫描去重 / profile 过滤维度 —— 后置。
- 不改 Autopilot 的 schema、调度器、admission gate、失败监控（直接复用）。

---

## 3. 当前 Multica Autopilot 事实（设计依据）

> 以下为本设计依赖的既有实现，**不改动**，仅复用。

- **数据**：`autopilot`（`assignee_id` + `assignee_type` ∈ {agent,squad}、`status` ∈ {active,paused,archived}、
  `execution_mode` ∈ {create_issue,run_only}、`issue_title_template`、`project_id` NULL）、
  `autopilot_trigger`（`kind` ∈ {schedule,webhook,api}、`cron_expression`、`timezone`、`next_run_at`、`enabled`）、
  `autopilot_run`（`source`、`status`、`issue_id`、`task_id`）。
- **调度器**：`server/cmd/server/autopilot_scheduler.go` `runAutopilotScheduler`（后台 goroutine，30s 轮询，
  `ClaimDueScheduleTriggers` 原子 claim `next_run_at <= now() AND enabled AND kind='schedule' AND autopilot.status='active'`，
  逐个 `DispatchAutopilot` 后 `advanceNextRun`；崩溃恢复 `recoverLostTriggers`）。
- **派发**：`server/internal/service/autopilot.go` `AutopilotService.DispatchAutopilot(ctx, autopilot, triggerID, source, payload)`
  → admission gate（assignee runtime 在线？否则 `recordSkippedRun`）→ `CreateAutopilotRun`
  → `execution_mode` 路由：`dispatchCreateIssue`（`CreateIssueWithOrigin(origin_type='autopilot', origin_id=autopilot.id)`
  + 插值 `issue_title_template` + 建 description + `EnqueueTaskForIssue`）/ `dispatchRunOnly`。
- **API**：`/api/autopilots` CRUD + `/{id}/triggers` CRUD + `/{id}/trigger`（手动触发，source='manual'）+ `/{id}/runs`。
  Handler `server/internal/handler/autopilot.go`，路由 `server/cmd/server/router.go` ~537–558。
  `AutopilotService.CreateAutopilot` / `CreateAutopilotTrigger` 可服务端编排。
- **UI**：`packages/views/autopilots/`（不复用其界面；F4 自有 Forge 界面）。

---

## 4. 数据模型

### 4.1 迁移 114 — `forge_entropy_scan`

`server/migrations/114_forge_entropy_scan.up.sql`：

```sql
CREATE TABLE forge_entropy_scan (
    id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id      UUID NOT NULL REFERENCES workspace(id) ON DELETE CASCADE,
    project_id        UUID REFERENCES project(id) ON DELETE CASCADE, -- NULL = workspace-level
    name              TEXT NOT NULL,
    scanner_agent_id  UUID NOT NULL REFERENCES agent(id) ON DELETE CASCADE,
    custom_focus      TEXT NOT NULL DEFAULT '',
    include_standards BOOLEAN NOT NULL DEFAULT true,
    include_checks    BOOLEAN NOT NULL DEFAULT true,
    cron_expression   TEXT NOT NULL,
    timezone          TEXT NOT NULL DEFAULT 'UTC',
    enabled           BOOLEAN NOT NULL DEFAULT true,
    autopilot_id      UUID REFERENCES autopilot(id) ON DELETE SET NULL, -- Forge-managed backing autopilot
    created_by        UUID NOT NULL,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_forge_entropy_scan_ws ON forge_entropy_scan(workspace_id);
-- 派发钩子反查：autopilot → scan（判定「这是熵 autopilot」）
CREATE UNIQUE INDEX idx_forge_entropy_scan_autopilot
    ON forge_entropy_scan(autopilot_id) WHERE autopilot_id IS NOT NULL;
```

down：`DROP TABLE forge_entropy_scan;`

**设计要点**
- `autopilot_id` 可空 + `ON DELETE SET NULL`：backing autopilot 若被外部删除，scan 行不悬空；Forge 代管时回写。
- 反查唯一索引保证一个 autopilot 至多对应一个熵 scan。
- **无「发现去重表」**：Q3 软去重，去重清单由「查带 `forge-entropy` 标签的开放 issue」即时得出（§6.3）。
- 不加 `name` 唯一约束（一个 scope 下允许多个 scan，与 F1/F2 多条记录一致）。

### 4.2 sqlc 查询 — `server/pkg/db/queries/forge_entropy.sql`

```sql
-- name: CreateEntropyScan :one
INSERT INTO forge_entropy_scan (workspace_id, project_id, name, scanner_agent_id,
    custom_focus, include_standards, include_checks, cron_expression, timezone, enabled, created_by)
VALUES ($1, sqlc.narg('project_id'), $2, $3, $4, $5, $6, $7, $8, $9, $10)
RETURNING *;

-- name: SetEntropyScanAutopilot :exec
UPDATE forge_entropy_scan SET autopilot_id = $2, updated_at = now() WHERE id = $1;

-- name: GetEntropyScan :one
SELECT * FROM forge_entropy_scan WHERE id = $1 AND workspace_id = $2;

-- name: GetEntropyScanByAutopilot :one
SELECT * FROM forge_entropy_scan WHERE autopilot_id = $1;

-- name: ListEntropyScans :many
SELECT * FROM forge_entropy_scan
WHERE workspace_id = $1
  AND project_id IS NOT DISTINCT FROM sqlc.narg('project_id')
ORDER BY created_at DESC;

-- name: UpdateEntropyScan :one
UPDATE forge_entropy_scan SET
    name = $3, scanner_agent_id = $4, custom_focus = $5, include_standards = $6,
    include_checks = $7, cron_expression = $8, timezone = $9, enabled = $10, updated_at = now()
WHERE id = $1 AND workspace_id = $2
RETURNING *;

-- name: DeleteEntropyScan :one
DELETE FROM forge_entropy_scan WHERE id = $1 AND workspace_id = $2
RETURNING autopilot_id;

-- name: ListOpenEntropyFindings :many
-- 去重清单：当前 scope 下带 forge-entropy 标签、未到终态的 issue。
SELECT i.id, i.identifier, i.title
FROM issue i
JOIN issue_to_label il ON il.issue_id = i.id
JOIN issue_label l ON l.id = il.label_id
WHERE i.workspace_id = $1
  AND (sqlc.narg('project_id')::uuid IS NULL OR i.project_id = sqlc.narg('project_id'))
  AND l.name = 'forge-entropy'
  AND i.status NOT IN ('done', 'cancelled')
ORDER BY i.created_at DESC
LIMIT 100;
```

> `ListOpenEntropyFindings` 的列名（`issue_to_label` / `issue_label` / `issue.status` 终态值）以实际 schema 为准，
> 在 P0 实施时按生成结果校正；若 label join 形态与上不符，退化为按 `issue.title` 前缀或 origin 标记查（见 §6.3 容错）。

---

## 5. 破环：`forge/checks` 子包化

**问题**：F4 的 brief 合成在**服务端**（autopilot 派发路径，`service.AutopilotService`）需要调用
F1 `standards.Resolve` + F2 `ResolveChecks`。但 `ResolveChecks` 在顶层 `internal/forge` 包，而该包的
`inject.go` import 了 `internal/service`（为 `service.AgentSkillData`）。于是 `service → forge → service`
成环——与 F3 当初的环同源。

**事实核查**（2026-06-01）
- `internal/forge/standards/` 已是**独立 service-free 子包**（`standards.Resolve` 可任意调用）。
- `internal/forge/checks.go`（`ResolveChecks` + `Check` + `CheckQuerier`）与 `inject.go` 同属顶层 `forge` 包，
  故导入 `forge` 取 `ResolveChecks` 会连带拉进 service。
- `service` 目前**未** import `internal/forge`（环尚未成形）。
- `ResolveChecks` 唯一调用点：`server/internal/handler/forge_daemon.go`（`forge.ResolveChecks` + `forge.Check`）。

**破环动作（小、对称、照搬已有 `standards/` 结构）**
1. `checks.go` + `checks_test.go` → 移入新子包 `server/internal/forge/checks/`（`package checks`）。
   导出 `checks.ResolveChecks`、`checks.Check`、`checks.CheckQuerier`，签名不变。
2. 更新唯一调用点 `handler/forge_daemon.go`：`forge.ResolveChecks` → `checks.ResolveChecks`、`forge.Check` → `checks.Check`。
3. 顶层 `internal/forge` 移除后仅剩 `inject.go`（继续 import service，是唯一 service-coupled 文件）。

破环后：`standards`（service-free）+ `checks`（service-free）两子包，F4 的 `forgeentropy` 可零环导入两者；
`service → forgeentropy → {db, forge/standards, forge/checks}` 全 service-free，无环。
（此举亦让未来需要服务端 resolve 的切片不再撞环。）

---

## 6. brief 合成 + 数据流

### 6.1 端到端流

```
cron 到点
  → Autopilot scheduler（现成：30s 轮询 + 原子 claim）
  → AutopilotService.DispatchAutopilot → dispatchCreateIssue
  → 【Forge 钩子】scan, err := q.GetEntropyScanByAutopilot(autopilot.id)
       err == nil（命中熵 autopilot）→
         brief := forgeentropy.ResolveBrief(ctx, q, scan)
                    ├─ scan.include_standards ? standards.Resolve(F1) : ""
                    ├─ scan.include_checks    ? checks.ResolveChecks(F2) : ""
                    ├─ scan.custom_focus
                    └─ q.ListOpenEntropyFindings(scope) → 去重清单
         issue.description = brief
  → 建 issue（assignee = scanner agent，origin_type='autopilot'）+ 派 task（现成）
  → scanner agent 读 issue brief → 全仓巡检
       → 经 multica CLI：为【新】发现建带 forge-entropy 标签的 issue；
         对仍存在的旧发现评论 bump；扫描 issue 上发摘要评论 → 完成
```

### 6.2 `forgeentropy` 包（service-free）

`server/internal/forgeentropy/brief.go`：

```go
package forgeentropy

// BriefInput 是 ComposeBrief 的纯输入（已 resolve 完毕）。
type BriefInput struct {
    ScanName     string
    StandardsText string // F1 resolve 结果（core+detail 拼接）；空 = 不含此段
    ChecksText    string // F2 resolve 结果（name + command 列表）；空 = 不含此段
    CustomFocus   string
    OpenFindings  []FindingRef // 去重清单
}

type FindingRef struct {
    Identifier string // e.g. MUL-123
    Title      string
}

// ComposeBrief 纯函数：组装 scanner agent 的扫描 brief（Markdown）。
func ComposeBrief(in BriefInput) string { /* §6.4 模板 */ }

// ResolveBrief：服务端调用入口——resolve F1/F2 + 查开放发现 → ComposeBrief。
// q 为接口（db.Queries 实现），聚合 standards.Querier / checks.CheckQuerier / 发现查询。
func ResolveBrief(ctx context.Context, q Querier, scan db.ForgeEntropyScan) string {
    var in BriefInput
    in.ScanName = scan.Name
    if scan.IncludeStandards {
        if res, err := standards.Resolve(ctx, q, scan.WorkspaceID, scan.ProjectID); err == nil {
            in.StandardsText = res.Core + "\n\n" + res.Detail // best-effort
        }
    }
    if scan.IncludeChecks {
        if cs, err := checks.ResolveChecks(ctx, q, scan.WorkspaceID, scan.ProjectID); err == nil {
            in.ChecksText = formatChecks(cs)
        }
    }
    in.CustomFocus = scan.CustomFocus
    if fs, err := q.ListOpenEntropyFindings(ctx, /* scope */); err == nil {
        in.OpenFindings = toFindingRefs(fs)
    }
    return ComposeBrief(in)
}
```

`Querier` 接口聚合三类查询（`standards.Querier`、`checks.CheckQuerier`、`ListOpenEntropyFindings`），由
`db.Queries` 满足——延续 F1/F3 的接口注入风格，便于单测打桩。

### 6.3 去重标记

- scanner agent 经 CLI 建的发现 issue 不会自带 `origin_type='autopilot'`（那是扫描 issue 的标记）。
  故发现 issue 用一个 **Forge 管理的 workspace 标签 `forge-entropy`** 标识。
- brief 指示 agent：每个新发现 issue 打 `forge-entropy` 标签。
- 去重清单 = `ListOpenEntropyFindings`（该 scope 下带 `forge-entropy` 标签、未到终态的 issue）。
- **容错**：若 label 体系形态与查询假设不符（P0 校正），退化为「按 origin 标记 / 标题前缀」查；
  去重是**软**保证（Q3），清单缺失最坏退化为「可能重复建」，不阻断扫描。
- 标签作用域：单一静态标签 `forge-entropy` + 按 `project_id` 收窄去重清单（不为每个 scan 造动态标签，避免标签膨胀）。

### 6.4 brief 模板（Markdown，注入 issue.description）

```
# Entropy Scan: {ScanName}

You are performing a periodic, WHOLE-REPOSITORY quality scan (not a diff review).
Survey the entire codebase for accumulated quality entropy and FILE issues for findings.
This is advisory — do NOT modify code in this task; only survey and report.

## This project's declared standards (F1)          ← 仅 include_standards
{StandardsText}

## This project's verification checks (F2)          ← 仅 include_checks
{ChecksText}

## Additional focus areas
{CustomFocus}

## Already-tracked findings — do NOT re-file these   ← 仅 OpenFindings 非空
{每行: - {Identifier} {Title}}
For each item above that still exists, add a short comment confirming it persists.
Only create NEW issues for findings NOT already listed.

## How to report
For each NEW finding, create an issue via the `multica` CLI:
- clear title, body with problem + location + suggested fix
- apply the label `forge-entropy`
When done, post a summary comment on THIS scan issue.
```

各段为空时整段省略（含/不含由开关与数据决定）。

---

## 7. 生命周期钩子（两处，都在已有路径）

### 7.1 派发钩子（service 侧，§6.1）
`server/internal/service/autopilot.go` `dispatchCreateIssue` 内一处：建 issue 前 `GetEntropyScanByAutopilot`，
命中则 `issue.description = forgeentropy.ResolveBrief(...)`。best-effort：resolve 失败退化为不含该段，
**绝不阻断派发**（延续 F1 InjectStandards 容错基调）。`service` 新增 import `internal/forgeentropy`（service-free，无环）。

### 7.2 backing autopilot 代管（handler 侧，非 Multica 改动）
`server/internal/handler/forge_entropy.go` 的 CRUD 在事务内同步管理 backing Autopilot：
- **POST** `/api/forge/entropy-scans`：`CreateEntropyScan` → `AutopilotService.CreateAutopilot`
  （title=`Entropy scan: {name}`、`execution_mode='create_issue'`、`assignee_type='agent'`、`assignee_id=scanner_agent`、
  `project_id`）+ `CreateAutopilotTrigger`（kind='schedule'、cron、timezone、enabled）→ `SetEntropyScanAutopilot(autopilot_id)`。
- **PATCH**：`UpdateEntropyScan` + 同步 autopilot（title/assignee/project）与其 schedule 触发（cron/timezone/enabled）。
- **DELETE**：`DeleteEntropyScan`（RETURNING autopilot_id）+ 删该 Autopilot。
- **容错**：autopilot 编排失败 → scan 写入回滚（不留孤儿 scan）；反之 scan 删除后尽力删 autopilot，删失败仅记日志
  （autopilot 孤儿可被 Multica 失败监控/手动清理，不阻断 scan 删除）。

---

## 8. API + UI

### 8.1 API — `/api/forge/entropy-scans`
`server/internal/handler/forge_entropy.go`，路由 `server/cmd/server/router.go`（镜像 F1/F2 forge 路由）：
| 方法 | 路径 | 行为 |
|---|---|---|
| GET | `/api/forge/entropy-scans?project_id=` | 列表（scope 过滤） |
| POST | `/api/forge/entropy-scans` | 建 scan + backing autopilot |
| PATCH | `/api/forge/entropy-scans/{id}` | 改 scan + 同步 autopilot |
| DELETE | `/api/forge/entropy-scans/{id}` | 删 scan + autopilot |

请求体 `ForgeEntropyScanBody{ project_id?, name, scanner_agent_id, custom_focus, include_standards, include_checks, cron_expression, timezone, enabled }`。
UUID 入参一律走 `parseUUIDOrBadRequest`（遵守后端 UUID 解析约定）。

### 8.2 UI — `packages/views/forge-entropy/`
- core 接线（镜像 F1/F2/F3）：`packages/core/types/forge-entropy.ts`（`ForgeEntropyScan`）、
  `client.ts`（list/create/update/deleteForgeEntropyScan）、`workspace/queries.ts`（`forgeEntropyScansOptions`）。
- view：`forge-entropy-page.tsx` —— 扫描列表 + 编辑面（name、scanner agent 下拉走 `agentListOptions` 过滤归档、
  cron 输入、timezone、F1/F2 两个开关、custom_focus 文本域、enabled）。`packages/views/package.json` 加 `./forge-entropy` export。
- web 路由：`apps/web/app/[workspaceSlug]/(dashboard)/forge-entropy/page.tsx`（re-export，按 URL 可达，不加侧边栏入口，与 F1/F2/F3 一致）。

---

## 9. 错误处理（汇总）
- brief 合成 best-effort：任一 resolve / 去重查询失败 → 记日志、退化为缺该段的 brief，不阻断派发。
- backing autopilot 编排失败 → scan 创建回滚；删除侧 autopilot 删失败仅记日志。
- 去重为软保证：清单缺失最坏「可能重复建 issue」，不阻断、不报错。
- scanner agent 跑不动（凭证/离线）→ Autopilot 自带 admission gate `recordSkippedRun`，不堆任务（复用，零新增）。

---

## 10. 测试 / 验收（凭证现实）

> **凭证现实**：与 F2/F3 同源——scanner agent **真扫仓 + 经 CLI 建发现 issue** 需 provider 凭证（agent 真跑）。
> 但 F4 的**编排链**（scan → backing autopilot → 派发 → brief 合成 → 落 issue.description）**全在服务端**，
> 可经「手动触发 autopilot」**绕凭证**端到端验证——比 F2/F3 的绕凭证面更强。

- **纯单测（绕凭证）**
  - `forgeentropy.ComposeBrief` 表驱动：含/不含 standards、含/不含 checks、有/无 custom_focus、有/无去重清单 → 断言各段存在性。
  - `forge/checks` 子包化后，F2 现有 `ResolveChecks` 测随包迁移仍绿。
- **绕凭证集成（源码构建栈）**
  1. API 建 entropy-scan → 断言 `forge_entropy_scan` 行 + backing `autopilot` + schedule `autopilot_trigger`（cron）已建、`autopilot_id` 回写。
  2. **手动触发** `POST /api/autopilots/{backing_id}/trigger` → `DispatchAutopilot` → `dispatchCreateIssue`
     → 命中 Forge 钩子 → 建 issue。**断言 `issue.description` 含合成的 F1 规范段 / F2 检查段 / custom_focus 段**。
  3. PATCH/DELETE → 断言 backing autopilot 同步更新/删除。
- **活体（凭证门，延后）**：scanner agent 真扫仓 + CLI 建带 `forge-entropy` 标签的发现 issue + 去重 bump。标注凭证依赖（同 F2/F3）。

---

## 11. 范围 / 拆分
本切片 = **advisory 熵扫描循环**（扫描 → 建发现 issue），单 spec，体量与 F1/F2/F3 相当：
sidecar 表 + sqlc + 破环子包化 + `forgeentropy` 合成 + 一处派发钩子 + autopilot 代管 + API/UI + 绕凭证验收。

**明确延后**
- **F4b 自愈闭环**：scanner agent 直接修 + 开 PR + 自动过 F2 门禁 + F3 评审（建在本切片之上，需活体凭证）。
- 严重度分级 / 趋势 dashboard / 跨扫描去重 / profile 过滤维度。

## 12. 后续
F4b 自愈闭环 → F5 可观测闭环（熵趋势 / 修复率 / Harness 健康度）。
provider 凭证仍是活体扫描的前置（同 F2 活体门禁 + F3 活体评审）。
