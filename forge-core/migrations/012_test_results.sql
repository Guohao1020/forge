CREATE TABLE IF NOT EXISTS engine.test_results (
    id              BIGSERIAL PRIMARY KEY,
    task_id         BIGINT NOT NULL REFERENCES engine.tasks(id),
    layer           VARCHAR(20) NOT NULL,  -- UNIT / API / INTEGRATION / E2E
    framework       VARCHAR(50),           -- go_test / junit / jest / pytest
    total_cases     INT NOT NULL DEFAULT 0,
    passed          INT NOT NULL DEFAULT 0,
    failed          INT NOT NULL DEFAULT 0,
    skipped         INT NOT NULL DEFAULT 0,
    coverage_pct    DECIMAL(5,2),
    duration_ms     INT,
    report          JSONB NOT NULL DEFAULT '{}',  -- detailed results
    status          VARCHAR(20) NOT NULL DEFAULT 'PENDING',  -- PENDING / RUNNING / PASSED / FAILED
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_test_results_task ON engine.test_results(task_id);
