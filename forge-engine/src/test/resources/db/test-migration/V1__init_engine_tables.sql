CREATE TABLE engine_task (
    id BIGINT AUTO_INCREMENT PRIMARY KEY,
    tenant_id BIGINT NOT NULL,
    user_id BIGINT NOT NULL,
    requirement CLOB NOT NULL,
    task_type VARCHAR(32) NOT NULL DEFAULT 'GENERATE',
    status VARCHAR(32) NOT NULL DEFAULT 'SUBMITTED',
    risk_level VARCHAR(16) DEFAULT NULL,
    repo_id VARCHAR(128) DEFAULT NULL,
    branch_name VARCHAR(128) DEFAULT NULL,
    mr_id BIGINT DEFAULT NULL,
    review_score INT DEFAULT NULL,
    total_input_tokens BIGINT NOT NULL DEFAULT 0,
    total_output_tokens BIGINT NOT NULL DEFAULT 0,
    error_message CLOB DEFAULT NULL,
    gmt_create DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    gmt_modified DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX idx_task_tenant_user ON engine_task(tenant_id, user_id);
CREATE INDEX idx_task_status ON engine_task(status);

CREATE TABLE engine_task_step (
    id BIGINT AUTO_INCREMENT PRIMARY KEY,
    task_id BIGINT NOT NULL,
    step_type VARCHAR(32) NOT NULL,
    step_order INT NOT NULL,
    status VARCHAR(32) NOT NULL DEFAULT 'PENDING',
    input_snapshot CLOB DEFAULT NULL,
    output_snapshot CLOB DEFAULT NULL,
    input_tokens BIGINT NOT NULL DEFAULT 0,
    output_tokens BIGINT NOT NULL DEFAULT 0,
    retry_count INT NOT NULL DEFAULT 0,
    error_message CLOB DEFAULT NULL,
    gmt_create DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    gmt_modified DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX idx_step_task_id ON engine_task_step(task_id);

CREATE TABLE engine_model_call_log (
    id BIGINT AUTO_INCREMENT PRIMARY KEY,
    task_id BIGINT NOT NULL,
    step_id BIGINT DEFAULT NULL,
    model_id VARCHAR(64) NOT NULL,
    purpose VARCHAR(32) NOT NULL,
    input_tokens BIGINT NOT NULL DEFAULT 0,
    output_tokens BIGINT NOT NULL DEFAULT 0,
    latency_ms BIGINT NOT NULL DEFAULT 0,
    is_fallback TINYINT NOT NULL DEFAULT 0,
    gmt_create DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX idx_call_log_task ON engine_model_call_log(task_id);

CREATE TABLE engine_code_change (
    id BIGINT AUTO_INCREMENT PRIMARY KEY,
    task_id BIGINT NOT NULL,
    repo_id VARCHAR(128) NOT NULL,
    branch_name VARCHAR(128) NOT NULL,
    commit_hash VARCHAR(64) DEFAULT NULL,
    file_count INT NOT NULL DEFAULT 0,
    review_score INT DEFAULT NULL,
    mr_id BIGINT DEFAULT NULL,
    mr_status VARCHAR(32) DEFAULT NULL,
    gmt_create DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    gmt_modified DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX idx_code_change_task ON engine_code_change(task_id);
