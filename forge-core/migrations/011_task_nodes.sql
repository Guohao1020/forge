-- S9: Task decomposition nodes for DAG-based planning
CREATE TABLE IF NOT EXISTS engine.task_nodes (
    id              BIGSERIAL PRIMARY KEY,
    task_id         BIGINT NOT NULL REFERENCES engine.tasks(id),
    node_order      INT NOT NULL,
    title           TEXT NOT NULL,
    description     TEXT,
    node_type       VARCHAR(20) NOT NULL DEFAULT 'BACKEND',  -- BACKEND / FRONTEND / SCHEMA / CONFIG / TEST
    status          VARCHAR(20) NOT NULL DEFAULT 'PENDING',   -- PENDING / READY / RUNNING / COMPLETED / SKIPPED
    depends_on      JSONB NOT NULL DEFAULT '[]',              -- array of node_order integers
    files           JSONB NOT NULL DEFAULT '[]',              -- array of file paths
    estimate_hours  DECIMAL(4,1),
    requirement_ref TEXT,                                     -- traceability: which part of requirement this covers
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_task_nodes_task ON engine.task_nodes(task_id);
CREATE UNIQUE INDEX IF NOT EXISTS idx_task_nodes_order ON engine.task_nodes(task_id, node_order);
