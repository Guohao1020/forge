## Phase 0 — 快照后端（forge_health.sql 聚合 + GetForgeHealth 端点）

**Goal:** `forge_health.sql` 快照聚合 query（配置 COUNT + 门禁/评审/findings/scan-runs/fix-PR 窗口聚合）
+ `GetForgeHealth` 端点 + 路由。**无迁移**。
**Depends-on:** 无　**Unblocks:** Phase 1, 2
**Completion gate:** `sqlc generate` 生成；`go build ./...` 通过。

> Go/sqlc 走 WSL；`git commit` 用原生 Windows git（无 `--no-verify`）。
> handler 镜像 `dashboard.go`:`resolveWorkspaceID`→`workspaceMember`→`parseProjectIDParam`→`resolveViewingTZ`→`parseSinceParamInTZ(r,30,tz)`。
> `agent_task_queue` 无 workspace_id → `JOIN agent a ON a.id=atq.agent_id WHERE a.workspace_id=$1` + `LEFT JOIN issue i` 做 project narg。

---

### Task 0.1: 快照聚合查询

**Files:**
- Create: `server/pkg/db/queries/forge_health.sql`

- [ ] **Step 1: 写查询**

`server/pkg/db/queries/forge_health.sql`：
```sql
-- Forge F5: Harness health snapshot — compute-on-read aggregates over existing
-- tables. No new tracking. project_id narg: NULL = whole workspace; when set,
-- config counts include that project + workspace-level (the resolved set).

-- name: CountForgeStandardsByCategory :many
SELECT category, COUNT(*)::int AS count
FROM forge_standard
WHERE workspace_id = $1 AND enabled = TRUE
  AND (sqlc.narg('project_id')::uuid IS NULL OR project_id = sqlc.narg('project_id') OR project_id IS NULL)
GROUP BY category
ORDER BY category;

-- name: CountForgeChecks :one
SELECT COUNT(*)::int AS count FROM forge_check
WHERE workspace_id = $1 AND enabled = TRUE
  AND (sqlc.narg('project_id')::uuid IS NULL OR project_id = sqlc.narg('project_id') OR project_id IS NULL);

-- name: CountForgeReviewConfigs :one
SELECT COUNT(*)::int AS count FROM forge_review_config
WHERE workspace_id = $1 AND enabled = TRUE
  AND (sqlc.narg('project_id')::uuid IS NULL OR project_id = sqlc.narg('project_id') OR project_id IS NULL);

-- name: CountForgeEntropyScans :one
SELECT COUNT(*)::int AS count FROM forge_entropy_scan
WHERE workspace_id = $1 AND enabled = TRUE
  AND (sqlc.narg('project_id')::uuid IS NULL OR project_id = sqlc.narg('project_id') OR project_id IS NULL);

-- name: GetForgeGateOutcomes :one
-- F2 gate: a verification failure flips the task and stamps
-- failure_reason='verification_failed'. passed = completed tasks that weren't
-- gate-failed; failed = gate-failed. Disjoint; total = passed + failed.
SELECT
    COUNT(*) FILTER (WHERE atq.status = 'completed')::int AS passed,
    COUNT(*) FILTER (WHERE atq.failure_reason = 'verification_failed')::int AS failed
FROM agent_task_queue atq
JOIN agent a ON a.id = atq.agent_id
LEFT JOIN issue i ON i.id = atq.issue_id
WHERE a.workspace_id = $1
  AND atq.issue_id IS NOT NULL
  AND atq.created_at >= sqlc.arg('since')::timestamptz
  AND (sqlc.narg('project_id')::uuid IS NULL OR i.project_id = sqlc.narg('project_id'));

-- name: GetForgeReviewOutcomes :one
-- F3 review tasks carry context->>'type' = 'forge_review'.
SELECT
    COUNT(*)::int AS total,
    COUNT(*) FILTER (WHERE atq.status = 'completed')::int AS completed,
    COALESCE(AVG(EXTRACT(EPOCH FROM (atq.completed_at - atq.created_at)))
        FILTER (WHERE atq.completed_at IS NOT NULL), 0)::bigint AS avg_turnaround_sec
FROM agent_task_queue atq
JOIN agent a ON a.id = atq.agent_id
LEFT JOIN issue i ON i.id = atq.issue_id
WHERE a.workspace_id = $1
  AND atq.context->>'type' = 'forge_review'
  AND atq.created_at >= sqlc.arg('since')::timestamptz
  AND (sqlc.narg('project_id')::uuid IS NULL OR i.project_id = sqlc.narg('project_id'));

-- name: CountOpenEntropyFindings :one
SELECT COUNT(*)::int AS count
FROM issue i
JOIN issue_to_label il ON il.issue_id = i.id
JOIN issue_label l ON l.id = il.label_id
WHERE i.workspace_id = $1
  AND (sqlc.narg('project_id')::uuid IS NULL OR i.project_id = sqlc.narg('project_id'))
  AND l.name = 'forge-entropy'
  AND i.status NOT IN ('done', 'cancelled');

-- name: CountForgeEntropyScanRuns :one
SELECT COUNT(*)::int AS count
FROM autopilot_run ar
JOIN forge_entropy_scan fes ON fes.autopilot_id = ar.autopilot_id
WHERE fes.workspace_id = $1
  AND ar.created_at >= sqlc.arg('since')::timestamptz
  AND (sqlc.narg('project_id')::uuid IS NULL OR fes.project_id = sqlc.narg('project_id'));

-- name: GetForgeFixPROutcomes :one
-- F4b fix PRs. merged via text-join to github_pull_request (GitHub-App webhook
-- populated). matched = how many fix PRs we could join; matched=0 means no
-- webhook data → UI shows merge rate as unknown.
SELECT
    COUNT(*)::int AS opened,
    COUNT(gpr.id) FILTER (WHERE gpr.state = 'merged')::int AS merged,
    COUNT(gpr.id)::int AS matched
FROM forge_fix_pr ffp
LEFT JOIN github_pull_request gpr
    ON gpr.workspace_id = ffp.workspace_id AND gpr.html_url = ffp.pr_url
JOIN issue i ON i.id = ffp.issue_id
WHERE ffp.workspace_id = $1
  AND ffp.created_at >= sqlc.arg('since')::timestamptz
  AND (sqlc.narg('project_id')::uuid IS NULL OR i.project_id = sqlc.narg('project_id'));
```

- [ ] **Step 2: 生成 + 校验**

Run: `wsl -d Ubuntu -- bash -lc "cd /mnt/d/shulex_work/forge/server && sqlc generate && go build ./... 2>&1 | tail -10"`
Expected: 生成 `CountForgeStandardsByCategoryRow{Category string, Count int32}`、`GetForgeGateOutcomesRow{Passed, Failed int32}`、
`GetForgeReviewOutcomesRow{Total, Completed int32, AvgTurnaroundSec int64}`、`GetForgeFixPROutcomesRow{Opened, Merged, Matched int32}`、
+ 各 `*Params{WorkspaceID, ProjectID pgtype.UUID, Since pgtype.Timestamptz}`（含 since 的查询）/`{WorkspaceID, ProjectID}`（纯 count）。
`go build ./...` 通过。

> 若某查询因列名/类型生成报错，按 sqlc 报错校正（已核：`agent_task_queue` 有 `status`/`failure_reason`/`context jsonb`/`created_at`/`completed_at`；
> `autopilot_run` 有 `autopilot_id`/`created_at`；`github_pull_request` 有 `state`/`html_url`/`workspace_id`）。

- [ ] **Step 3: Commit**

```bash
git add server/pkg/db/queries/forge_health.sql server/pkg/db/generated/
git commit -m "feat(forge): F5 snapshot aggregate queries (forge_health)"
```

---

### Task 0.2: GetForgeHealth 端点 + 路由

**Files:**
- Create: `server/internal/handler/forge_health.go`
- Modify: `server/cmd/server/router.go`（forge 路由块）

- [ ] **Step 1: handler**

`server/internal/handler/forge_health.go`：
```go
package handler

import (
	"net/http"

	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

// Forge F5: Harness health snapshot. Compute-on-read aggregation over existing
// Forge + Multica tables (no new tracking). Mirrors dashboard.go conventions.

type ForgeHealthCategoryCount struct {
	Category string `json:"category"`
	Count    int32  `json:"count"`
}

type ForgeHealthGate struct {
	Passed int32 `json:"passed"`
	Failed int32 `json:"failed"`
}

type ForgeHealthReview struct {
	Total            int32 `json:"total"`
	Completed        int32 `json:"completed"`
	AvgTurnaroundSec int64 `json:"avg_turnaround_sec"`
}

type ForgeHealthFixPRs struct {
	Opened  int32 `json:"opened"`
	Merged  int32 `json:"merged"`
	Matched int32 `json:"matched"` // 0 = no github_pull_request data (merge rate unknown)
}

type ForgeHealthResponse struct {
	Standards      []ForgeHealthCategoryCount `json:"standards"`
	StandardsTotal int32                      `json:"standards_total"`
	Checks         int32                      `json:"checks"`
	ReviewConfigs  int32                      `json:"review_configs"`
	Scans          int32                      `json:"scans"`
	Gate           ForgeHealthGate            `json:"gate"`
	Review         ForgeHealthReview          `json:"review"`
	OpenFindings   int32                      `json:"open_findings"`
	ScanRuns       int32                      `json:"scan_runs"`
	FixPRs         ForgeHealthFixPRs          `json:"fix_prs"`
}

func (h *Handler) GetForgeHealth(w http.ResponseWriter, r *http.Request) {
	workspaceID := h.resolveWorkspaceID(r)
	if _, ok := h.workspaceMember(w, r, workspaceID); !ok {
		return
	}
	projectID, ok := parseProjectIDParam(w, r)
	if !ok {
		return
	}
	tz := h.resolveViewingTZ(r)
	since := parseSinceParamInTZ(r, 30, tz)
	ws := parseUUID(workspaceID)
	ctx := r.Context()

	out := ForgeHealthResponse{Standards: []ForgeHealthCategoryCount{}}

	if rows, err := h.Queries.CountForgeStandardsByCategory(ctx, db.CountForgeStandardsByCategoryParams{WorkspaceID: ws, ProjectID: projectID}); err == nil {
		for _, row := range rows {
			out.Standards = append(out.Standards, ForgeHealthCategoryCount{Category: row.Category, Count: row.Count})
			out.StandardsTotal += row.Count
		}
	}
	out.Checks, _ = h.Queries.CountForgeChecks(ctx, db.CountForgeChecksParams{WorkspaceID: ws, ProjectID: projectID})
	out.ReviewConfigs, _ = h.Queries.CountForgeReviewConfigs(ctx, db.CountForgeReviewConfigsParams{WorkspaceID: ws, ProjectID: projectID})
	out.Scans, _ = h.Queries.CountForgeEntropyScans(ctx, db.CountForgeEntropyScansParams{WorkspaceID: ws, ProjectID: projectID})
	out.OpenFindings, _ = h.Queries.CountOpenEntropyFindings(ctx, db.CountOpenEntropyFindingsParams{WorkspaceID: ws, ProjectID: projectID})

	if g, err := h.Queries.GetForgeGateOutcomes(ctx, db.GetForgeGateOutcomesParams{WorkspaceID: ws, ProjectID: projectID, Since: since}); err == nil {
		out.Gate = ForgeHealthGate{Passed: g.Passed, Failed: g.Failed}
	}
	if rv, err := h.Queries.GetForgeReviewOutcomes(ctx, db.GetForgeReviewOutcomesParams{WorkspaceID: ws, ProjectID: projectID, Since: since}); err == nil {
		out.Review = ForgeHealthReview{Total: rv.Total, Completed: rv.Completed, AvgTurnaroundSec: rv.AvgTurnaroundSec}
	}
	out.ScanRuns, _ = h.Queries.CountForgeEntropyScanRuns(ctx, db.CountForgeEntropyScanRunsParams{WorkspaceID: ws, ProjectID: projectID, Since: since})
	if fp, err := h.Queries.GetForgeFixPROutcomes(ctx, db.GetForgeFixPROutcomesParams{WorkspaceID: ws, ProjectID: projectID, Since: since}); err == nil {
		out.FixPRs = ForgeHealthFixPRs{Opened: fp.Opened, Merged: fp.Merged, Matched: fp.Matched}
	}

	writeJSON(w, http.StatusOK, out)
}
```
> 注：`CountForgeChecks` 等纯 count 的 `:one` 直接返回 `(int32, error)`,故 `out.Checks, _ = ...`。
> 含 since 的查询返回 Row struct。各查询 best-effort:出错则该字段留零值,不阻断整面(§8)。
> handler 不直接引用 `pgtype.`(所有 pgtype 值由 `parseUUID`/`parseProjectIDParam`/`parseSinceParamInTZ`
> 返回并经类型推断流入 db params),故**不 import pgtype**。若某处编译要求显式 pgtype,按报错补 import。

- [ ] **Step 2: 路由**

`server/cmd/server/router.go`,在 `/api/forge/entropy-scans` 块之后加:
```go
			// Forge F5: Harness health observability.
			r.Route("/api/forge/health", func(r chi.Router) {
				r.Get("/", h.GetForgeHealth)
			})
```

- [ ] **Step 3: 编译**

Run: `wsl -d Ubuntu -- bash -lc "cd /mnt/d/shulex_work/forge/server && go build ./... 2>&1 | tail -10 && go vet ./internal/handler/ 2>&1 | tail -5 && echo OK"`
Expected: 打印 `OK`。（若 `h.workspaceMember`/`resolveViewingTZ`/`parseSinceParamInTZ` 签名与假设不符,按 `dashboard.go` 实际调用修正——它们就在同包。）

- [ ] **Step 4: Commit**

```bash
git add server/internal/handler/forge_health.go server/cmd/server/router.go
git commit -m "feat(forge): F5 GetForgeHealth snapshot endpoint"
```

---

## Phase 0 完成检查
- [ ] `forge_health.sql` 9 个快照聚合 query 生成，编译通过
- [ ] `GET /api/forge/health` 端点 + 路由，`go build` + vet 绿
