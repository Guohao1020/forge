-- P1: Add tech_stack to projects for language constraint detection
ALTER TABLE engine.projects ADD COLUMN IF NOT EXISTS tech_stack JSONB NOT NULL DEFAULT '{}';
