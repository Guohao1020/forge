-- 租户表
CREATE TABLE identity_tenant (
    id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY COMMENT '租户ID',
    name VARCHAR(100) NOT NULL COMMENT '租户名称',
    status TINYINT NOT NULL DEFAULT 1 COMMENT '状态: 1=启用, 0=禁用',
    gmt_create DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    gmt_modified DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '修改时间',
    UNIQUE KEY uk_name (name)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin COMMENT='租户表';

-- 用户表
CREATE TABLE identity_user (
    id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY COMMENT '用户ID',
    tenant_id BIGINT UNSIGNED NOT NULL COMMENT '租户ID',
    username VARCHAR(64) NOT NULL COMMENT '用户名',
    password_hash VARCHAR(128) NOT NULL COMMENT '密码哈希(BCrypt)',
    email VARCHAR(128) DEFAULT NULL COMMENT '邮箱',
    nickname VARCHAR(64) DEFAULT NULL COMMENT '昵称',
    status TINYINT NOT NULL DEFAULT 1 COMMENT '状态: 1=启用, 0=禁用',
    gmt_create DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    gmt_modified DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '修改时间',
    UNIQUE KEY uk_tenant_username (tenant_id, username),
    INDEX idx_tenant_id (tenant_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin COMMENT='用户表';

-- 角色表
CREATE TABLE identity_role (
    id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY COMMENT '角色ID',
    tenant_id BIGINT UNSIGNED NOT NULL COMMENT '租户ID',
    role_code VARCHAR(32) NOT NULL COMMENT '角色编码: ADMIN, USER',
    role_name VARCHAR(64) NOT NULL COMMENT '角色名称',
    gmt_create DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    gmt_modified DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '修改时间',
    UNIQUE KEY uk_tenant_code (tenant_id, role_code),
    INDEX idx_tenant_id (tenant_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin COMMENT='角色表';

-- 用户角色绑定表
CREATE TABLE identity_user_role (
    id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY COMMENT 'ID',
    user_id BIGINT UNSIGNED NOT NULL COMMENT '用户ID',
    role_id BIGINT UNSIGNED NOT NULL COMMENT '角色ID',
    gmt_create DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    UNIQUE KEY uk_user_role (user_id, role_id),
    INDEX idx_user_id (user_id),
    INDEX idx_role_id (role_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin COMMENT='用户角色绑定表';

-- API Token 表
CREATE TABLE identity_api_token (
    id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY COMMENT 'ID',
    tenant_id BIGINT UNSIGNED NOT NULL COMMENT '租户ID',
    user_id BIGINT UNSIGNED NOT NULL COMMENT '用户ID',
    token_name VARCHAR(64) NOT NULL COMMENT 'Token 名称',
    token_hash VARCHAR(128) NOT NULL COMMENT 'Token 哈希(SHA-256)',
    token_prefix VARCHAR(8) NOT NULL COMMENT 'Token 前缀(用于展示)',
    expires_at DATETIME DEFAULT NULL COMMENT '过期时间(NULL=永不过期)',
    status TINYINT NOT NULL DEFAULT 1 COMMENT '状态: 1=启用, 0=禁用',
    gmt_create DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    gmt_modified DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '修改时间',
    INDEX idx_tenant_user (tenant_id, user_id),
    INDEX idx_token_hash (token_hash)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin COMMENT='API Token 表';

-- 种子数据: 默认租户
INSERT INTO identity_tenant (name, status) VALUES ('default', 1);

-- 种子数据: 默认角色
INSERT INTO identity_role (tenant_id, role_code, role_name) VALUES
(1, 'ADMIN', '管理员'),
(1, 'USER', '普通用户');

-- 种子数据: 管理员用户 (密码: admin123, BCrypt hash)
INSERT INTO identity_user (tenant_id, username, password_hash, nickname, status) VALUES
(1, 'admin', '$2a$10$bJWB1JoPwQd4H2wbTc.4LeUK1y7fA9fk93.iaLwYnatd.3BxPrMf2', '系统管理员', 1);

-- 种子数据: 管理员角色绑定
INSERT INTO identity_user_role (user_id, role_id) VALUES (1, 1);
