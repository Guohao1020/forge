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
