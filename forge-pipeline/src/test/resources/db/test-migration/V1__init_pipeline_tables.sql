CREATE TABLE pipeline_execution (
    id BIGINT AUTO_INCREMENT PRIMARY KEY,
    tenant_id BIGINT NOT NULL,
    repo_id VARCHAR(128) NOT NULL,
    branch VARCHAR(128) NOT NULL,
    pipeline_id VARCHAR(128) DEFAULT NULL,
    run_id VARCHAR(128) DEFAULT NULL,
    project_type VARCHAR(32) NOT NULL DEFAULT 'JAVA_SERVICE',
    status VARCHAR(32) NOT NULL DEFAULT 'PENDING',
    compile_passed TINYINT DEFAULT NULL,
    test_passed TINYINT DEFAULT NULL,
    review_passed TINYINT DEFAULT NULL,
    quality_gate_passed TINYINT DEFAULT NULL,
    log_url CLOB DEFAULT NULL,
    error_message CLOB DEFAULT NULL,
    trigger_type VARCHAR(32) NOT NULL DEFAULT 'WEBHOOK',
    task_id BIGINT DEFAULT NULL,
    gmt_create DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    gmt_modified DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX idx_pe_tenant_repo ON pipeline_execution(tenant_id, repo_id);

CREATE TABLE deployment_record (
    id BIGINT AUTO_INCREMENT PRIMARY KEY,
    tenant_id BIGINT NOT NULL,
    environment_id BIGINT DEFAULT NULL,
    repo_id VARCHAR(128) NOT NULL,
    branch VARCHAR(128) NOT NULL,
    image VARCHAR(256) NOT NULL,
    namespace VARCHAR(128) NOT NULL,
    deployment_name VARCHAR(128) NOT NULL,
    replicas INT NOT NULL DEFAULT 1,
    status VARCHAR(32) NOT NULL DEFAULT 'PENDING',
    error_message CLOB DEFAULT NULL,
    pipeline_execution_id BIGINT DEFAULT NULL,
    gmt_create DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    gmt_modified DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX idx_dr_tenant ON deployment_record(tenant_id);

CREATE TABLE environment (
    id BIGINT AUTO_INCREMENT PRIMARY KEY,
    tenant_id BIGINT NOT NULL,
    name VARCHAR(64) NOT NULL,
    env_type VARCHAR(32) NOT NULL,
    namespace VARCHAR(128) NOT NULL,
    bound_branch VARCHAR(128) DEFAULT NULL,
    status VARCHAR(32) NOT NULL DEFAULT 'ACTIVE',
    auto_destroy_at DATETIME DEFAULT NULL,
    repo_id VARCHAR(128) DEFAULT NULL,
    task_id BIGINT DEFAULT NULL,
    gmt_create DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    gmt_modified DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX idx_env_tenant ON environment(tenant_id);
