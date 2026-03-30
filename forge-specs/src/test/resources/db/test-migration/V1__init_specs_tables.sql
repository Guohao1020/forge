-- H2-compatible version of the specs schema (removes ON UPDATE CURRENT_TIMESTAMP and ENGINE clauses)

CREATE TABLE IF NOT EXISTS spec_standard (
    id BIGINT AUTO_INCREMENT PRIMARY KEY,
    category VARCHAR(64) NOT NULL,
    title VARCHAR(256) NOT NULL,
    content TEXT NOT NULL,
    scope_level VARCHAR(16) NOT NULL DEFAULT 'company',
    scope_id VARCHAR(64) DEFAULT NULL,
    sort_order INT NOT NULL DEFAULT 0,
    is_enabled TINYINT(1) NOT NULL DEFAULT 1,
    gmt_create DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    gmt_modified DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX idx_standard_category ON spec_standard (category);
CREATE INDEX idx_standard_scope ON spec_standard (scope_level, scope_id);

CREATE TABLE IF NOT EXISTS spec_prompt_template (
    id BIGINT AUTO_INCREMENT PRIMARY KEY,
    template_key VARCHAR(64) NOT NULL,
    name VARCHAR(128) NOT NULL,
    description VARCHAR(512) DEFAULT NULL,
    system_prompt TEXT NOT NULL,
    standards_injection TEXT DEFAULT NULL,
    version INT NOT NULL DEFAULT 1,
    is_active TINYINT(1) NOT NULL DEFAULT 1,
    gmt_create DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    gmt_modified DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE (template_key, version)
);
CREATE INDEX idx_prompt_template_key ON spec_prompt_template (template_key);

CREATE TABLE IF NOT EXISTS spec_review_rule (
    id BIGINT AUTO_INCREMENT PRIMARY KEY,
    category VARCHAR(64) NOT NULL,
    rule_key VARCHAR(128) NOT NULL,
    name VARCHAR(256) NOT NULL,
    description TEXT NOT NULL,
    severity VARCHAR(16) NOT NULL DEFAULT 'warning',
    is_enabled TINYINT(1) NOT NULL DEFAULT 1,
    gmt_create DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    gmt_modified DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE (rule_key)
);
CREATE INDEX idx_review_rule_category ON spec_review_rule (category);

CREATE TABLE IF NOT EXISTS spec_scaffold_template (
    id BIGINT AUTO_INCREMENT PRIMARY KEY,
    name VARCHAR(128) NOT NULL,
    description VARCHAR(512) DEFAULT NULL,
    tech_stack VARCHAR(256) DEFAULT NULL,
    template_content TEXT NOT NULL,
    is_active TINYINT(1) NOT NULL DEFAULT 1,
    gmt_create DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    gmt_modified DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE (name)
);
