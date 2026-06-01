# F5 可观测闭环 · 设计（argus）

> Forge-on-Multica 收官切片。把 F1–F4b 的 Harness 活动 compute-on-read 聚合成**健康面板**:
> 快照卡 + 趋势图 + 混合钻取。**零新追踪表 / 零迁移** —— 纯聚合既有表。

**Status:** Approved（2026-06-01 brainstorming）
**Plan:** `docs/forge/plans/argus-2026-06-01/`（待 writing-plans 产出）。

---

## 1. 决策链（brainstorming 2026-06-01）

| # | 决策 | 选择 |
|---|------|------|
| Q1 | 形态 | **完整可观测套件**(快照 + 趋势 + 钻取) |
| Q2 | 钻取机制 | **混合**:能链到现有视图就链;没有的(gate-failed task / 评审任务)在健康面内嵌轻量明细表 |
| PR 合并率 | webhook 门处理 | 修复 PR 开启数永远显示;合并率只在 `github_pull_request` 有匹配数据时显示,否则优雅降级「— · 需 GitHub App」 |

**核心洞察**(F5 探明):**F1–F4b 几乎全部指标可由聚合既有表得出,零新追踪**。仅 F4b 合并率依赖
GitHub App webhook(`forge_fix_pr.pr_url ↔ github_pull_request.html_url` 文本 join)。镜像 Multica
既有 `/api/dashboard`(compute-on-read SUM/COUNT + tz-aware `DATE(col AT TIME ZONE tz)` 分桶)。

---

## 2. 目标 / 非目标

**目标**
- 一个 **Forge Harness 健康面板**:聚合 F1–F4b 活动 → 快照卡 + 趋势图 + 钻取明细。
- 让用户一眼看到「Harness 在起什么作用」:配了多少规范/检查/扫描,门禁通过率,评审完成率,
  开放熵发现数 + 趋势,修复 PR 开启/合并。
- **零新追踪表 / 零迁移**(compute-on-read 聚合既有表)。
- Forge 隔离(R2):`/api/forge/health*` + `packages/views/forge-health`,workspace 级 + 可选 project 过滤。

**非目标(本切片不做)**
- composite「Harness 健康总分」(只出原始指标,不加权合成)。
- 阈值告警 / 通知。
- materialized rollup 表(`forge_activity_hourly`)—— 表小,compute-on-read 足够;趋势变慢再加。
- 跨 workspace 聚合视图。
- PR 合并率的「可靠 join」(webhook 成熟前用文本 join + 优雅降级)。

---

## 3. 当前事实（设计依据,不改）

- **既有 dashboard**:`server/internal/handler/dashboard.go`(4 端点:usage/daily、usage/by-agent、
  agent-runtime、runtime/daily)+ `server/pkg/db/queries/task_usage.sql`。**compute-on-read**:
  `COUNT(*) FILTER (WHERE ...)`、`COALESCE(SUM(...),0)::bigint`、`DATE(col AT TIME ZONE sqlc.arg('tz')::text)`、
  `EXTRACT(EPOCH FROM (completed_at - started_at))`。前端 `packages/views/dashboard/components/dashboard-page.tsx`。
- **recharts 3.8.0** 已在 `packages/ui`/`packages/views`/`apps/web`;`packages/views/agents/components/sparkline.tsx` 可复用。
- **可聚合的 Harness 数据**(F5 探明):
  - F1 `forge_standard`(count by category);F2 `forge_check`(count)。
  - F2 门禁失败 = `agent_task_queue` `status` 终态 + `failure_reason='verification_failed'`(daemon.go:2222 写入)。
  - F3 `forge_review_config`(count);评审任务 = `agent_task_queue` `context->>'type'='forge_review'`
    (marker `forgereview.ReviewContextType`);`parent_task_id` 链回 coder;周转 = `completed_at - created_at`。
  - F4 `forge_entropy_scan`(count);开放熵发现 = `issue` JOIN `issue_to_label`/`issue_label` `name='forge-entropy'`
    AND `status NOT IN ('done','cancelled')`(复用 `ListOpenEntropyFindings`);扫描运行 = `autopilot_run` JOIN
    `forge_entropy_scan.autopilot_id`。
  - F4b `forge_fix_pr`(count = PR 开启);合并 = `LEFT JOIN github_pull_request ON html_url = pr_url AND state='merged'`。
- **Forge handler helpers**(已有):`h.resolveWorkspaceID(r)`、`parseUUIDOrBadRequest`、`parseUUID`、`writeJSON`、`writeError`、`uuidToString`。

---

## 4. 数据 / 查询（sqlc,全 compute-on-read）`server/pkg/db/queries/forge_health.sql`

> 全部带 `workspace_id`,可选 `project_id`(narg),时间窗 `since`(narg timestamptz)+ `tz`。
> 拆成多个聚焦 query(非一个巨 query),每个对应一组指标。

**快照类(point-in-time count + window rate)**
- `CountForgeStandards`(by category,GROUP BY category)
- `CountForgeChecks` / `CountForgeReviewConfigs` / `CountForgeEntropyScans`
- `GetGateOutcomes`(window):`COUNT(*) total`,`COUNT(*) FILTER (WHERE failure_reason='verification_failed') failed`
  —— 限定 issue-bound 且终态(`status IN ('completed','failed','blocked')`)的任务。
- `GetReviewOutcomes`(window):review 任务 total / `FILTER (WHERE status='completed') completed` /
  `AVG(EXTRACT(EPOCH FROM (completed_at - created_at)))` 周转。
- `CountOpenEntropyFindings`(复用 ListOpenEntropyFindings 的 COUNT 版)。
- `CountEntropyScanRuns`(window,autopilot_run join)。
- `GetFixPROutcomes`(window):`COUNT(*) opened`,`COUNT(*) FILTER (WHERE gpr.state='merged') merged`
  via `LEFT JOIN github_pull_request gpr ON gpr.workspace_id=ffp.workspace_id AND gpr.html_url=ffp.pr_url`,
  外加 `COUNT(gpr.id) matched`(=0 则合并率「未知/需 webhook」)。

**趋势类(date-bucketed,window)**
- `TrendEntropyFindings`(`DATE(i.created_at AT TIME ZONE tz)` 分桶,COUNT)。
- `TrendGatePassRate`(按天 total/failed,前端算率)。
- `TrendFixPRs`(按天 opened)。

**钻取类(轻量 list,LIMIT)**
- 开放熵发现:复用 `ListOpenEntropyFindings`(返回 id/number/title → 行链到 issue)。
- `ListRecentGateFailures`(window,LIMIT 50:issue_id/identifier/title/failed-task completed_at)。
- `ListRecentFixPRs`(window,LIMIT 50:issue_id/pr_url/created_at → 行链到 PR url / issue)。

---

## 5. 指标清单 + 绕凭证可验性

| 层 | 指标 | 绕凭证(源码构建栈已有数据) |
|---|---|---|
| F1 | 规范数(by category) | ✓ 配置,非零 |
| F2 | 检查数;**门禁通过率**(passed/total) | ✓ 检查数;门禁率有 F4/F4b 留下的完成任务数据 |
| F3 | reviewer 配置数;评审任务数/完成率/平均周转 | ✓ 配置数;评审任务数据视 live 而定 |
| F4 | 扫描数;**开放熵发现数** + 趋势;扫描运行数 | ✓ 扫描数 + 已有熵 issue + autopilot_run |
| F4b | **修复 PR 开启数**;合并率(降级) | ✓ 开启数(=已建的 forge_fix_pr 行);合并率需 webhook |

不做 composite 健康分。

---

## 6. API（镜像 dashboard.go 的 tz/since 参数 + F1–F4b forge handler 风格）

`server/internal/handler/forge_health.go` + 路由(在 forge 路由块):
- `GET /api/forge/health?project_id=&days=N&tz=` → 快照(全部计数 + 窗口率,一个聚合 response)。
- `GET /api/forge/health/trends?project_id=&days=N&tz=` → 三条时间序列。
- `GET /api/forge/health/findings?project_id=` → 开放熵发现列表(钻取)。
- `GET /api/forge/health/gate-failures?project_id=&days=N` → 近期门禁失败任务列表(钻取)。
- `GET /api/forge/health/fix-prs?project_id=&days=N` → 近期修复 PR 列表(钻取)。

`days` 默认 30,`tz` 默认 UTC。所有端点 workspace-scoped(`resolveWorkspaceID`),project_id 可选过滤。

---

## 7. UI `packages/views/forge-health/forge-health-page.tsx`（镜像 `dashboard-page.tsx`）

- **顶部快照卡网格**:按 F1/F2/F3/F4/F4b 分组,每指标一卡(大数字 + 标签 + 关键率,如门禁通过率/评审完成率)。
- **中部趋势图**:recharts `LineChart`/`BarChart`(熵发现趋势、门禁通过率趋势、修复 PR 趋势);可复用 `agents/sparkline.tsx`。
- **混合钻取**:卡可展开 →
  - 开放熵发现 / 修复 PR:行链到 issue(`useNavigation().push` 到 issue)/ 外部 PR url。
  - 门禁失败任务 / 评审任务:健康面内嵌轻量明细表(兑底,无现成视图)。
- core 接线(`types/forge-health.ts`、client 5 方法、queries options)+ web 路由 `forge-health`,镜像前几个 F。
- 不加侧边栏入口(与 F1–F4b 一致,URL 可达)。

---

## 8. 错误处理
- 单指标 query 失败 → 该卡显「—」,不崩整面(各指标独立 fetch 或 response 内各段可空)。
- 合并率:`matched=0`(无 webhook 数据)→ 显「— · 需 GitHub App」。
- 空数据 → 卡显 0 / 趋势空态。tz 默认 UTC;非法 tz → 回退 UTC。
- 前端按 CLAUDE.md「API Response Compatibility」:health response 经 `parseWithFallback` + zod
  （**F5 是只读聚合面、数值驱动 UI,值得上 schema**;比 F1–F4b 的 forge config 端点更该防御,因字段多、可空)。

---

## 9. 测试 / 验收（凭证现实 —— **F5 绕凭证最彻底**）

- **纯单测**:抽出的纯计算(如 pass_rate / merge_rate 计算 + 「matched=0 → 未知」逻辑)单测。
- **绕凭证集成(源码构建栈)**:F4/F4b 验证已在 DB 留下真实数据(配置行、熵 issue、≥1 个 forge_fix_pr 行、
  完成的 task)→ `GET /api/forge/health` → **断言:配置计数非零(规范/检查/扫描);开放熵发现数 ≥ 实际;
  fix PR 开启数 ≥ 1(我们建的那个);门禁/评审计数与 DB 实际一致**;`/trends` 返回 date-bucketed 数组;
  钻取端点返回对应 list。**纯读聚合,无需任何 agent 跑** —— 比 F2/F3/F4/F4b 都更彻底地绕凭证。
- **前端**:三包 typecheck;health response 的 zod schema 喂畸形响应(缺字段/null)测试(CLAUDE.md 要求)。
- **活体**:满负荷真实数字(高门禁/评审/扫描/修复量)需持续 live 活动(凭证);但 F5 的查询 + 面板本身完全绕凭证可验。

---

## 10. 范围 / 拆分
单切片(偏大但连贯):`forge_health.sql`(聚合 + 趋势 + 钻取 query)+ `forge_health.go`(5 端点)+
`forge-health` view(卡 + recharts 趋势 + 钻取)+ core 接线 + zod schema。**无迁移**。保持精简:
无 composite 分、无告警、钻取混合复用。

## 11. 后续
Harness 健康总分 / 阈值告警 / 跨 workspace 视图 / materialized rollup(趋势变慢时)/ PR 合并率可靠 join。
**F5 落地后 Forge 路线图 F0–F5 全闭环**:声明(F1)→ 门禁(F2)→ 评审(F3)→ 扫描(F4)→ 自愈(F4b)→ **观测(F5)**。
