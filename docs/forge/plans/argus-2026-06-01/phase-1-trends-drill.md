## Phase 1 — 趋势 + 钻取后端（trend/drill query + 4 端点）

**Goal:** 趋势 query（熵发现/门禁/修复 PR 按天）+ 钻取 query（gate-failures/fix-prs；findings 复用）
+ 4 端点（trends/findings/gate-failures/fix-prs）。
**Depends-on:** Phase 0　**Unblocks:** Phase 3
**Completion gate:** `sqlc generate` + `go build ./...` 通过。

> 追加到既有 `server/pkg/db/queries/forge_health.sql`。日期用 `DATE(col AT TIME ZONE tz)::text AS date`
> （`::text` 让 sqlc 生成 `string`,免去 pgtype.Date 处理）。findings 钻取复用 F4 既有 `ListOpenEntropyFindings`。

---

### Task 1.1: 趋势 + 钻取查询

**Files:**
- Modify: `server/pkg/db/queries/forge_health.sql`（追加）

- [ ] **Step 1: 追加查询**

在 `server/pkg/db/queries/forge_health.sql` 末尾追加：
```sql
-- ---- Trends (date-bucketed, tz-aware) ----

-- name: TrendEntropyFindings :many
SELECT DATE(i.created_at AT TIME ZONE sqlc.arg('tz')::text)::text AS date, COUNT(*)::int AS count
FROM issue i
JOIN issue_to_label il ON il.issue_id = i.id
JOIN issue_label l ON l.id = il.label_id
WHERE i.workspace_id = $1
  AND l.name = 'forge-entropy'
  AND i.created_at >= sqlc.arg('since')::timestamptz
  AND (sqlc.narg('project_id')::uuid IS NULL OR i.project_id = sqlc.narg('project_id'))
GROUP BY DATE(i.created_at AT TIME ZONE sqlc.arg('tz')::text)
ORDER BY DATE(i.created_at AT TIME ZONE sqlc.arg('tz')::text);

-- name: TrendGatePassRate :many
SELECT DATE(atq.created_at AT TIME ZONE sqlc.arg('tz')::text)::text AS date,
    COUNT(*) FILTER (WHERE atq.status = 'completed')::int AS passed,
    COUNT(*) FILTER (WHERE atq.failure_reason = 'verification_failed')::int AS failed
FROM agent_task_queue atq
JOIN agent a ON a.id = atq.agent_id
LEFT JOIN issue i ON i.id = atq.issue_id
WHERE a.workspace_id = $1
  AND atq.issue_id IS NOT NULL
  AND atq.created_at >= sqlc.arg('since')::timestamptz
  AND (sqlc.narg('project_id')::uuid IS NULL OR i.project_id = sqlc.narg('project_id'))
GROUP BY DATE(atq.created_at AT TIME ZONE sqlc.arg('tz')::text)
ORDER BY DATE(atq.created_at AT TIME ZONE sqlc.arg('tz')::text);

-- name: TrendFixPRs :many
SELECT DATE(ffp.created_at AT TIME ZONE sqlc.arg('tz')::text)::text AS date, COUNT(*)::int AS count
FROM forge_fix_pr ffp
JOIN issue i ON i.id = ffp.issue_id
WHERE ffp.workspace_id = $1
  AND ffp.created_at >= sqlc.arg('since')::timestamptz
  AND (sqlc.narg('project_id')::uuid IS NULL OR i.project_id = sqlc.narg('project_id'))
GROUP BY DATE(ffp.created_at AT TIME ZONE sqlc.arg('tz')::text)
ORDER BY DATE(ffp.created_at AT TIME ZONE sqlc.arg('tz')::text);

-- ---- Drill-down lists ----

-- name: ListRecentGateFailures :many
SELECT i.id AS issue_id, i.number, i.title, atq.created_at
FROM agent_task_queue atq
JOIN agent a ON a.id = atq.agent_id
JOIN issue i ON i.id = atq.issue_id
WHERE a.workspace_id = $1
  AND atq.failure_reason = 'verification_failed'
  AND atq.created_at >= sqlc.arg('since')::timestamptz
  AND (sqlc.narg('project_id')::uuid IS NULL OR i.project_id = sqlc.narg('project_id'))
ORDER BY atq.created_at DESC
LIMIT 50;

-- name: ListRecentFixPRs :many
SELECT ffp.id, ffp.issue_id, ffp.pr_url, ffp.created_at, i.number, i.title
FROM forge_fix_pr ffp
JOIN issue i ON i.id = ffp.issue_id
WHERE ffp.workspace_id = $1
  AND ffp.created_at >= sqlc.arg('since')::timestamptz
  AND (sqlc.narg('project_id')::uuid IS NULL OR i.project_id = sqlc.narg('project_id'))
ORDER BY ffp.created_at DESC
LIMIT 50;
```

- [ ] **Step 2: 生成 + 校验**

Run: `wsl -d Ubuntu -- bash -lc "cd /mnt/d/shulex_work/forge/server && sqlc generate && go build ./... 2>&1 | tail -10"`
Expected: 生成 `TrendEntropyFindingsRow{Date string, Count int32}`、`TrendGatePassRateRow{Date string, Passed, Failed int32}`、
`TrendFixPRsRow{Date string, Count int32}`、`ListRecentGateFailuresRow{IssueID pgtype.UUID, Number int32, Title string, CreatedAt pgtype.Timestamptz}`、
`ListRecentFixPRsRow{ID, IssueID pgtype.UUID, PrUrl string, CreatedAt pgtype.Timestamptz, Number int32, Title string}` + 各 Params（含 tz/since/project_id narg）。`go build` 通过。

- [ ] **Step 3: Commit**

```bash
git add server/pkg/db/queries/forge_health.sql server/pkg/db/generated/
git commit -m "feat(forge): F5 trend + drill-down queries"
```

---

### Task 1.2: trends + drill 端点

**Files:**
- Modify: `server/internal/handler/forge_health.go`（追加 handler）
- Modify: `server/cmd/server/router.go`（health 子路由加 4 条）

- [ ] **Step 1: 追加 handler**

在 `server/internal/handler/forge_health.go` 追加（response 结构 + 4 个 handler）：
```go
type ForgeTrendPoint struct {
	Date   string `json:"date"`
	Passed int32  `json:"passed,omitempty"`
	Failed int32  `json:"failed,omitempty"`
	Count  int32  `json:"count,omitempty"`
}

type ForgeHealthTrendsResponse struct {
	Findings []ForgeTrendPoint `json:"findings"`
	Gate     []ForgeTrendPoint `json:"gate"`
	FixPRs   []ForgeTrendPoint `json:"fix_prs"`
}

func (h *Handler) GetForgeHealthTrends(w http.ResponseWriter, r *http.Request) {
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

	out := ForgeHealthTrendsResponse{Findings: []ForgeTrendPoint{}, Gate: []ForgeTrendPoint{}, FixPRs: []ForgeTrendPoint{}}
	if rows, err := h.Queries.TrendEntropyFindings(ctx, db.TrendEntropyFindingsParams{WorkspaceID: ws, ProjectID: projectID, Tz: tz, Since: since}); err == nil {
		for _, row := range rows {
			out.Findings = append(out.Findings, ForgeTrendPoint{Date: row.Date, Count: row.Count})
		}
	}
	if rows, err := h.Queries.TrendGatePassRate(ctx, db.TrendGatePassRateParams{WorkspaceID: ws, ProjectID: projectID, Tz: tz, Since: since}); err == nil {
		for _, row := range rows {
			out.Gate = append(out.Gate, ForgeTrendPoint{Date: row.Date, Passed: row.Passed, Failed: row.Failed})
		}
	}
	if rows, err := h.Queries.TrendFixPRs(ctx, db.TrendFixPRsParams{WorkspaceID: ws, ProjectID: projectID, Tz: tz, Since: since}); err == nil {
		for _, row := range rows {
			out.FixPRs = append(out.FixPRs, ForgeTrendPoint{Date: row.Date, Count: row.Count})
		}
	}
	writeJSON(w, http.StatusOK, out)
}

type ForgeIssueRef struct {
	IssueID string `json:"issue_id"`
	Number  int32  `json:"number"`
	Title   string `json:"title"`
}

func (h *Handler) GetForgeHealthFindings(w http.ResponseWriter, r *http.Request) {
	workspaceID := h.resolveWorkspaceID(r)
	if _, ok := h.workspaceMember(w, r, workspaceID); !ok {
		return
	}
	projectID, ok := parseProjectIDParam(w, r)
	if !ok {
		return
	}
	rows, err := h.Queries.ListOpenEntropyFindings(r.Context(), db.ListOpenEntropyFindingsParams{WorkspaceID: parseUUID(workspaceID), ProjectID: projectID})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list findings")
		return
	}
	out := make([]ForgeIssueRef, 0, len(rows))
	for _, row := range rows {
		out = append(out, ForgeIssueRef{IssueID: uuidToString(row.ID), Number: row.Number, Title: row.Title})
	}
	writeJSON(w, http.StatusOK, out)
}

func (h *Handler) GetForgeHealthGateFailures(w http.ResponseWriter, r *http.Request) {
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
	rows, err := h.Queries.ListRecentGateFailures(r.Context(), db.ListRecentGateFailuresParams{WorkspaceID: parseUUID(workspaceID), ProjectID: projectID, Since: since})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list gate failures")
		return
	}
	out := make([]ForgeIssueRef, 0, len(rows))
	for _, row := range rows {
		out = append(out, ForgeIssueRef{IssueID: uuidToString(row.IssueID), Number: row.Number, Title: row.Title})
	}
	writeJSON(w, http.StatusOK, out)
}

type ForgeFixPRRef struct {
	IssueID string `json:"issue_id"`
	Number  int32  `json:"number"`
	Title   string `json:"title"`
	PrURL   string `json:"pr_url"`
}

func (h *Handler) GetForgeHealthFixPRs(w http.ResponseWriter, r *http.Request) {
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
	rows, err := h.Queries.ListRecentFixPRs(r.Context(), db.ListRecentFixPRsParams{WorkspaceID: parseUUID(workspaceID), ProjectID: projectID, Since: since})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list fix PRs")
		return
	}
	out := make([]ForgeFixPRRef, 0, len(rows))
	for _, row := range rows {
		out = append(out, ForgeFixPRRef{IssueID: uuidToString(row.IssueID), Number: row.Number, Title: row.Title, PrURL: row.PrUrl})
	}
	writeJSON(w, http.StatusOK, out)
}
```
> `TrendEntropyFindingsParams` 等的 `Tz` 字段是 `sqlc.arg('tz')` 生成的(string)。`resolveViewingTZ` 返回 string,直接传。

- [ ] **Step 2: 路由**

把 `server/cmd/server/router.go` 的 health 子路由块改为:
```go
			// Forge F5: Harness health observability.
			r.Route("/api/forge/health", func(r chi.Router) {
				r.Get("/", h.GetForgeHealth)
				r.Get("/trends", h.GetForgeHealthTrends)
				r.Get("/findings", h.GetForgeHealthFindings)
				r.Get("/gate-failures", h.GetForgeHealthGateFailures)
				r.Get("/fix-prs", h.GetForgeHealthFixPRs)
			})
```

- [ ] **Step 3: 编译**

Run: `wsl -d Ubuntu -- bash -lc "cd /mnt/d/shulex_work/forge/server && go build ./... 2>&1 | tail -10 && go vet ./internal/handler/ 2>&1 | tail -5 && echo OK"`
Expected: 打印 `OK`。（若 `TrendEntropyFindingsParams` 的 tz 字段名非 `Tz`,按生成结果改。）

- [ ] **Step 4: Commit**

```bash
git add server/internal/handler/forge_health.go server/cmd/server/router.go
git commit -m "feat(forge): F5 trends + drill-down endpoints"
```

---

## Phase 1 完成检查
- [ ] 趋势 3 query + 钻取 2 query 生成，编译通过
- [ ] `/api/forge/health/{trends,findings,gate-failures,fix-prs}` 4 端点，`go build` + vet 绿
