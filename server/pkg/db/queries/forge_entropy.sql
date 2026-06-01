-- Forge F4: entropy scan config CRUD + dispatch-hook reverse lookup + dedup list.

-- name: CreateEntropyScan :one
INSERT INTO forge_entropy_scan (
    workspace_id, project_id, name, scanner_agent_id, custom_focus,
    include_standards, include_checks, cron_expression, timezone, enabled, created_by, auto_fix
) VALUES ($1, sqlc.narg('project_id'), $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
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
    include_checks = $7, cron_expression = $8, timezone = $9, enabled = $10, auto_fix = $11, updated_at = now()
WHERE id = $1 AND workspace_id = $2
RETURNING *;

-- name: DeleteEntropyScan :one
DELETE FROM forge_entropy_scan WHERE id = $1 AND workspace_id = $2
RETURNING autopilot_id;

-- name: ListOpenEntropyFindings :many
-- Dedup list: open (non-terminal) issues carrying the 'forge-entropy' label in
-- this scope. project_id narg NULL = whole workspace.
SELECT i.id, i.number, i.title
FROM issue i
JOIN issue_to_label il ON il.issue_id = i.id
JOIN issue_label l ON l.id = il.label_id
WHERE i.workspace_id = $1
  AND (sqlc.narg('project_id')::uuid IS NULL OR i.project_id = sqlc.narg('project_id'))
  AND l.name = 'forge-entropy'
  AND i.status NOT IN ('done', 'cancelled')
ORDER BY i.created_at DESC
LIMIT 100;
