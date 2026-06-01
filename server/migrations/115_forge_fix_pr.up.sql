-- Forge F4b: self-healing loop. Per-scan auto_fix flag enables the scanner agent
-- to fix what it safely can and open a PR. forge_fix_pr records the PR the agent
-- opened against its issue (self-contained; does NOT touch the GitHub-App-coupled
-- github_pull_request table).
ALTER TABLE forge_entropy_scan ADD COLUMN auto_fix BOOLEAN NOT NULL DEFAULT FALSE;

CREATE TABLE forge_fix_pr (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL REFERENCES workspace(id) ON DELETE CASCADE,
    issue_id     UUID NOT NULL REFERENCES issue(id) ON DELETE CASCADE,
    task_id      UUID NOT NULL REFERENCES agent_task_queue(id) ON DELETE CASCADE,
    pr_url       TEXT NOT NULL,
    branch       TEXT NOT NULL DEFAULT '',
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_forge_fix_pr_issue ON forge_fix_pr(issue_id);
CREATE UNIQUE INDEX idx_forge_fix_pr_task_url ON forge_fix_pr(task_id, pr_url);
