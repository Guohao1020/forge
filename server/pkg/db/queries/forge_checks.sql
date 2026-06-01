-- Forge F2: verification checks

-- name: ListWorkspaceChecks :many
-- Workspace-level checks (project_id IS NULL), enabled only.
SELECT * FROM forge_check
WHERE workspace_id = $1 AND project_id IS NULL AND enabled = TRUE
ORDER BY name;

-- name: ListProjectChecks :many
-- Project-level checks for a project, enabled only.
SELECT * FROM forge_check
WHERE project_id = $1 AND enabled = TRUE
ORDER BY name;

-- name: ListChecksByWorkspace :many
-- All checks in a workspace (both scopes), for the list API.
SELECT * FROM forge_check
WHERE workspace_id = $1
ORDER BY project_id NULLS FIRST, name;

-- name: GetForgeCheck :one
SELECT * FROM forge_check WHERE id = $1 AND workspace_id = $2;

-- name: CreateForgeCheck :one
INSERT INTO forge_check (workspace_id, project_id, name, command, enabled, created_by)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;

-- name: UpdateForgeCheck :one
UPDATE forge_check SET
    name = COALESCE(sqlc.narg('name'), name),
    command = COALESCE(sqlc.narg('command'), command),
    enabled = COALESCE(sqlc.narg('enabled'), enabled),
    updated_at = now()
WHERE id = $1 AND workspace_id = $2
RETURNING *;

-- name: DeleteForgeCheck :exec
DELETE FROM forge_check WHERE id = $1 AND workspace_id = $2;
