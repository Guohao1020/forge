-- 规范表：编码规范基线文档
CREATE TABLE IF NOT EXISTS spec_standard (
    id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    category VARCHAR(64) NOT NULL COMMENT '分类：java/sql/api/security/naming/git',
    title VARCHAR(256) NOT NULL COMMENT '规范标题',
    content TEXT NOT NULL COMMENT '规范内容（Markdown）',
    scope_level VARCHAR(16) NOT NULL DEFAULT 'company' COMMENT '作用域：company/team/project',
    scope_id VARCHAR(64) DEFAULT NULL COMMENT '作用域 ID（团队ID 或 项目ID）',
    sort_order INT NOT NULL DEFAULT 0 COMMENT '排序',
    is_enabled TINYINT(1) NOT NULL DEFAULT 1 COMMENT '是否启用',
    gmt_create DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    gmt_modified DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    INDEX idx_category (category),
    INDEX idx_scope (scope_level, scope_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin COMMENT='编码规范';

-- Prompt 模板表
CREATE TABLE IF NOT EXISTS spec_prompt_template (
    id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    template_key VARCHAR(64) NOT NULL COMMENT '模板标识：requirement-analysis/code-generation/code-review/test-generation/fix-generation/doc-generation',
    name VARCHAR(128) NOT NULL COMMENT '模板名称',
    description VARCHAR(512) DEFAULT NULL COMMENT '模板说明',
    system_prompt TEXT NOT NULL COMMENT 'System Prompt（固定层）',
    standards_injection TEXT DEFAULT NULL COMMENT 'Standards Injection 模板（项目层，变量占位）',
    version INT NOT NULL DEFAULT 1 COMMENT '版本号',
    is_active TINYINT(1) NOT NULL DEFAULT 1 COMMENT '是否为当前活跃版本',
    gmt_create DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    gmt_modified DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    UNIQUE KEY uk_template_key_version (template_key, version),
    INDEX idx_template_key (template_key)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin COMMENT='Prompt 模板';

-- AI Review 规则表
CREATE TABLE IF NOT EXISTS spec_review_rule (
    id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    category VARCHAR(64) NOT NULL COMMENT '分类：coding/security/performance/database/api-compat',
    rule_key VARCHAR(128) NOT NULL COMMENT '规则标识',
    name VARCHAR(256) NOT NULL COMMENT '规则名称',
    description TEXT NOT NULL COMMENT '规则描述（Markdown）',
    severity VARCHAR(16) NOT NULL DEFAULT 'warning' COMMENT '严重程度：error/warning/info',
    is_enabled TINYINT(1) NOT NULL DEFAULT 1 COMMENT '是否启用',
    gmt_create DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    gmt_modified DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    UNIQUE KEY uk_rule_key (rule_key),
    INDEX idx_category (category)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin COMMENT='AI Review 规则';

-- 脚手架模板表
CREATE TABLE IF NOT EXISTS spec_scaffold_template (
    id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    name VARCHAR(128) NOT NULL COMMENT '模板名称：java-microservice/vue3-frontend/java-sdk',
    description VARCHAR(512) DEFAULT NULL COMMENT '模板说明',
    tech_stack VARCHAR(256) DEFAULT NULL COMMENT '技术栈描述',
    template_content TEXT NOT NULL COMMENT '脚手架模板内容（JSON/YAML 格式的文件结构 + 模板代码）',
    is_active TINYINT(1) NOT NULL DEFAULT 1 COMMENT '是否启用',
    gmt_create DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    gmt_modified DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    UNIQUE KEY uk_name (name)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin COMMENT='脚手架模板';
