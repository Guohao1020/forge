-- Forge F3: review config + review task creation

-- name: GetWorkspaceReviewConfig :one
SELECT * FROM forge_review_config
WHERE workspace_id = $1 AND project_id IS NULL AND enabled = TRUE;

-- name: GetProjectReviewConfig :one
SELECT * FROM forge_review_config
WHERE project_id = $1 AND enabled = TRUE;

-- name: GetReviewConfigByScope :one
-- For the GET API: workspace-level when project_id arg is NULL, else project.
SELECT * FROM forge_review_config
WHERE workspace_id = $1
  AND project_id IS NOT DISTINCT FROM sqlc.narg('project_id');

-- name: UpsertWorkspaceReviewConfig :one
INSERT INTO forge_review_config (workspace_id, project_id, reviewer_agent_id, enabled, created_by)
VALUES ($1, NULL, $2, $3, $4)
ON CONFLICT (workspace_id) WHERE project_id IS NULL
DO UPDATE SET reviewer_agent_id = EXCLUDED.reviewer_agent_id, enabled = EXCLUDED.enabled, updated_at = now()
RETURNING *;

-- name: UpsertProjectReviewConfig :one
INSERT INTO forge_review_config (workspace_id, project_id, reviewer_agent_id, enabled, created_by)
VALUES ($1, $2, $3, $4, $5)
ON CONFLICT (project_id) WHERE project_id IS NOT NULL
DO UPDATE SET reviewer_agent_id = EXCLUDED.reviewer_agent_id, enabled = EXCLUDED.enabled, updated_at = now()
RETURNING *;

-- name: CreateForgeReviewTask :one
-- Enqueue a review task for the reviewer agent against the same issue, reusing
-- the coder's workdir. The forge_review marker in context prevents review loops.
INSERT INTO agent_task_queue (
    agent_id, runtime_id, issue_id, parent_task_id, work_dir, context, status, priority
)
VALUES ($1, $2, $3, $4, $5, $6, 'queued', $7)
RETURNING *;
