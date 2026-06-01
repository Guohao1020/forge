-- Forge F4b: record the PR a coding agent opened against its issue (idempotent).

-- name: CreateFixPR :one
INSERT INTO forge_fix_pr (workspace_id, issue_id, task_id, pr_url, branch)
VALUES ($1, $2, $3, $4, $5)
ON CONFLICT (task_id, pr_url) DO NOTHING
RETURNING *;

-- name: ListFixPRsByIssue :many
SELECT * FROM forge_fix_pr WHERE issue_id = $1 ORDER BY created_at DESC;
