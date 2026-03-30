CREATE TABLE identity_tenant (
    id BIGINT AUTO_INCREMENT PRIMARY KEY,
    name VARCHAR(100) NOT NULL,
    status TINYINT NOT NULL DEFAULT 1,
    gmt_create DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    gmt_modified DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE UNIQUE INDEX uk_tenant_name ON identity_tenant(name);

CREATE TABLE identity_user (
    id BIGINT AUTO_INCREMENT PRIMARY KEY,
    tenant_id BIGINT NOT NULL,
    username VARCHAR(64) NOT NULL,
    password_hash VARCHAR(128) NOT NULL,
    email VARCHAR(128) DEFAULT NULL,
    nickname VARCHAR(64) DEFAULT NULL,
    status TINYINT NOT NULL DEFAULT 1,
    gmt_create DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    gmt_modified DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE UNIQUE INDEX uk_tenant_username ON identity_user(tenant_id, username);
CREATE INDEX idx_user_tenant ON identity_user(tenant_id);

CREATE TABLE identity_role (
    id BIGINT AUTO_INCREMENT PRIMARY KEY,
    tenant_id BIGINT NOT NULL,
    role_code VARCHAR(32) NOT NULL,
    role_name VARCHAR(64) NOT NULL,
    gmt_create DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    gmt_modified DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE UNIQUE INDEX uk_role_tenant_code ON identity_role(tenant_id, role_code);

CREATE TABLE identity_user_role (
    id BIGINT AUTO_INCREMENT PRIMARY KEY,
    user_id BIGINT NOT NULL,
    role_id BIGINT NOT NULL,
    gmt_create DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE UNIQUE INDEX uk_user_role ON identity_user_role(user_id, role_id);

CREATE TABLE identity_api_token (
    id BIGINT AUTO_INCREMENT PRIMARY KEY,
    tenant_id BIGINT NOT NULL,
    user_id BIGINT NOT NULL,
    token_name VARCHAR(64) NOT NULL,
    token_hash VARCHAR(128) NOT NULL,
    token_prefix VARCHAR(8) NOT NULL,
    expires_at DATETIME DEFAULT NULL,
    status TINYINT NOT NULL DEFAULT 1,
    gmt_create DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    gmt_modified DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX idx_api_token_tenant_user ON identity_api_token(tenant_id, user_id);
CREATE INDEX idx_api_token_hash ON identity_api_token(token_hash);
