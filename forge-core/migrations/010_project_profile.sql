-- S16: Project profile / AI memory storage
CREATE TABLE IF NOT EXISTS engine.project_profiles (
    id            BIGSERIAL PRIMARY KEY,
    project_id    BIGINT NOT NULL REFERENCES engine.projects(id),
    profile_key   VARCHAR(50) NOT NULL,   -- 'api_catalog', 'db_schema', 'module_graph', 'architecture', 'business_rules', 'coding_habits', 'quality_trends'
    profile_value JSONB NOT NULL DEFAULT '{}',
    version       INT NOT NULL DEFAULT 1,
    scanned_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_project_profiles_key ON engine.project_profiles(project_id, profile_key);
CREATE INDEX IF NOT EXISTS idx_project_profiles_project ON engine.project_profiles(project_id);

COMMENT ON TABLE engine.project_profiles IS '项目画像：AI 分析仓库后的结构化知识存储';
COMMENT ON COLUMN engine.project_profiles.profile_key IS '画像维度：api_catalog/db_schema/module_graph/architecture/business_rules/coding_habits/quality_trends';
