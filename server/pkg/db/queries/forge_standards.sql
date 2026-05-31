-- Forge F1: standards + project profile

-- name: ListWorkspaceStandards :many
-- Workspace-level standards (project_id IS NULL), enabled only. Used by resolve.
SELECT * FROM forge_standard
WHERE workspace_id = $1 AND project_id IS NULL AND enabled = TRUE
ORDER BY category, name;

-- name: ListProjectStandards :many
-- Project-level overrides for a specific project, enabled only.
SELECT * FROM forge_standard
WHERE project_id = $1 AND enabled = TRUE
ORDER BY category, name;

-- name: ListStandardsByWorkspace :many
-- All standards in a workspace (both scopes), for the list API.
SELECT * FROM forge_standard
WHERE workspace_id = $1
ORDER BY project_id NULLS FIRST, category, name;

-- name: GetForgeStandard :one
SELECT * FROM forge_standard WHERE id = $1 AND workspace_id = $2;

-- name: CreateForgeStandard :one
INSERT INTO forge_standard
    (workspace_id, project_id, name, category, profile_tags, core_content, detail_content, enabled, created_by)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
RETURNING *;

-- name: UpdateForgeStandard :one
UPDATE forge_standard SET
    name = COALESCE(sqlc.narg('name'), name),
    category = COALESCE(sqlc.narg('category'), category),
    profile_tags = COALESCE(sqlc.narg('profile_tags'), profile_tags),
    core_content = COALESCE(sqlc.narg('core_content'), core_content),
    detail_content = COALESCE(sqlc.narg('detail_content'), detail_content),
    enabled = COALESCE(sqlc.narg('enabled'), enabled),
    updated_at = now()
WHERE id = $1 AND workspace_id = $2
RETURNING *;

-- name: DeleteForgeStandard :exec
DELETE FROM forge_standard WHERE id = $1 AND workspace_id = $2;

-- name: GetForgeProjectProfile :one
SELECT * FROM forge_project_profile WHERE project_id = $1;

-- name: UpsertForgeProjectProfile :one
INSERT INTO forge_project_profile (project_id, tags, updated_at)
VALUES ($1, $2, now())
ON CONFLICT (project_id) DO UPDATE SET tags = EXCLUDED.tags, updated_at = now()
RETURNING *;
