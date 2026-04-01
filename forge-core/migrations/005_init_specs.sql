-- =============================================
-- S5: Specs Center — 规范中心
-- =============================================

CREATE SCHEMA IF NOT EXISTS specs;

-- 1. 编码规范
CREATE TABLE IF NOT EXISTS specs.standards (
    id              BIGSERIAL PRIMARY KEY,
    tenant_id       BIGINT NOT NULL,
    name            VARCHAR(200) NOT NULL,
    category        VARCHAR(50) NOT NULL,
    scope           VARCHAR(20) NOT NULL,
    scope_id        BIGINT NOT NULL DEFAULT 0,
    parent_id       BIGINT REFERENCES specs.standards(id),
    content         TEXT NOT NULL,
    version         INT NOT NULL DEFAULT 1,
    status          VARCHAR(20) NOT NULL DEFAULT 'ACTIVE',
    created_by      BIGINT REFERENCES auth.users(id),
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_standards_tenant ON specs.standards(tenant_id);
CREATE INDEX IF NOT EXISTS idx_standards_category ON specs.standards(tenant_id, category);
CREATE INDEX IF NOT EXISTS idx_standards_scope ON specs.standards(tenant_id, scope, scope_id);

-- 2. Prompt 模板
CREATE TABLE IF NOT EXISTS specs.prompt_templates (
    id              BIGSERIAL PRIMARY KEY,
    tenant_id       BIGINT NOT NULL,
    name            VARCHAR(200) NOT NULL,
    purpose         VARCHAR(50) NOT NULL,
    system_prompt   TEXT NOT NULL,
    user_template   TEXT NOT NULL,
    variables       JSONB NOT NULL DEFAULT '[]',
    version         INT NOT NULL DEFAULT 1,
    is_default      BOOLEAN NOT NULL DEFAULT FALSE,
    created_by      BIGINT REFERENCES auth.users(id),
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_prompt_templates_tenant ON specs.prompt_templates(tenant_id);
CREATE INDEX IF NOT EXISTS idx_prompt_templates_purpose ON specs.prompt_templates(tenant_id, purpose);

-- 3. Review 规则
CREATE TABLE IF NOT EXISTS specs.review_rules (
    id              BIGSERIAL PRIMARY KEY,
    tenant_id       BIGINT NOT NULL,
    name            VARCHAR(200) NOT NULL,
    category        VARCHAR(50) NOT NULL,
    scope           VARCHAR(20) NOT NULL DEFAULT 'COMPANY',
    scope_id        BIGINT NOT NULL DEFAULT 0,
    rule_type       VARCHAR(20) NOT NULL,
    definition      JSONB NOT NULL,
    severity        VARCHAR(10) NOT NULL,
    auto_fix        BOOLEAN NOT NULL DEFAULT FALSE,
    fix_template    TEXT,
    enabled         BOOLEAN NOT NULL DEFAULT TRUE,
    created_by      BIGINT REFERENCES auth.users(id),
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_review_rules_tenant ON specs.review_rules(tenant_id);
CREATE INDEX IF NOT EXISTS idx_review_rules_category ON specs.review_rules(tenant_id, category);
CREATE INDEX IF NOT EXISTS idx_review_rules_scope ON specs.review_rules(tenant_id, scope, scope_id);

-- 4. 脚手架模板
CREATE TABLE IF NOT EXISTS specs.scaffold_templates (
    id              BIGSERIAL PRIMARY KEY,
    tenant_id       BIGINT NOT NULL,
    name            VARCHAR(200) NOT NULL,
    project_type    VARCHAR(50) NOT NULL,
    description     TEXT,
    template_repo   TEXT,
    variables       JSONB NOT NULL DEFAULT '[]',
    post_hooks      JSONB NOT NULL DEFAULT '[]',
    version         INT NOT NULL DEFAULT 1,
    created_by      BIGINT REFERENCES auth.users(id),
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_scaffold_templates_tenant ON specs.scaffold_templates(tenant_id);

-- =============================================
-- Seed Data
-- =============================================

-- Default coding standards (company-level, tenant_id=1)
INSERT INTO specs.standards (tenant_id, name, category, scope, scope_id, content, created_by) VALUES
(1, 'Java 编码规范', 'JAVA', 'COMPANY', 0,
'## Java 编码规范

### 命名规范
- 类名使用 UpperCamelCase
- 方法名使用 lowerCamelCase
- 常量使用 UPPER_SNAKE_CASE
- 包名使用全小写

### 代码风格
- 缩进使用 4 空格
- 行宽上限 120 字符
- 方法长度不超过 80 行
- 类长度不超过 500 行

### 异常处理
- 禁止空 catch 块
- 使用自定义业务异常
- 异常信息必须包含上下文

### 注释规范
- 公开 API 必须有 Javadoc
- 复杂逻辑必须有行内注释
- TODO 必须关联 Issue 编号', 1),

(1, 'SQL 编码规范', 'SQL', 'COMPANY', 0,
'## SQL 编码规范

### 命名规范
- 表名使用 snake_case 复数形式
- 字段名使用 snake_case
- 索引命名: idx_{table}_{column}
- 外键命名: fk_{table}_{ref_table}

### 查询规范
- 禁止 SELECT *
- 大表查询必须走索引
- JOIN 不超过 3 张表
- 子查询优先改写为 JOIN

### DDL 规范
- 所有表必须有主键
- 必须有 created_at/updated_at
- 字符串字段指定长度上限
- 大文本使用 TEXT 类型', 1),

(1, 'API 设计规范', 'API', 'COMPANY', 0,
'## API 设计规范

### URL 规范
- 使用 RESTful 风格
- 资源名使用复数名词
- URL 使用 kebab-case
- 版本号放在 URL path: /api/v1/

### 请求/响应
- 统一使用 JSON 格式
- 响应使用 Result[T] 包装
- 分页使用 page/pageSize 参数
- 时间字段使用 ISO 8601

### 错误处理
- 使用标准 HTTP 状态码
- 错误响应包含 code + message
- 业务错误使用 4xx
- 系统错误使用 5xx', 1),

(1, 'Git 工作流规范', 'GIT', 'COMPANY', 0,
'## Git 工作流规范

### 分支规范
- main: 生产分支，保护分支
- develop: 开发分支
- feature/*: 功能分支
- hotfix/*: 紧急修复
- release/*: 发布分支

### 提交规范
- 使用 Conventional Commits
- 格式: type(scope): description
- type: feat/fix/docs/style/refactor/test/chore

### Code Review
- 所有合并必须经过 Review
- 至少一个 Approve
- CI 通过后才能合并', 1);

-- Default prompt templates (tenant_id=1)
INSERT INTO specs.prompt_templates (tenant_id, name, purpose, system_prompt, user_template, variables, is_default, created_by) VALUES
(1, '需求分析模板', 'requirement-analysis',
'你是一名资深产品经理和系统分析师。你的任务是将用户的自然语言需求转化为结构化的需求规格。

## 规范约束
{{coding_standards}}

## 项目上下文
项目名称: {{project_name}}
技术栈: {{tech_stack}}
现有模块: {{existing_modules}}',
'请分析以下需求并输出结构化规格：

## 用户需求
{{user_requirement}}

请输出：
1. 功能点列表（含优先级）
2. 涉及的模块和接口
3. 数据模型变更
4. 非功能需求（性能/安全）
5. 需要澄清的问题',
'["coding_standards", "project_name", "tech_stack", "existing_modules", "user_requirement"]',
true, 1),

(1, '代码生成模板', 'code-generation',
'你是一名高级软件工程师。你的任务是根据需求规格生成生产级代码。

## 编码规范（必须严格遵守）
{{coding_standards}}

## 项目约束
项目名称: {{project_name}}
技术栈: {{tech_stack}}
架构模式: {{architecture_pattern}}

## 现有代码上下文
{{code_context}}',
'请根据以下规格生成代码：

## 需求规格
{{requirement_spec}}

## 要求
1. 严格遵守编码规范
2. 包含完整的错误处理
3. 包含必要的注释
4. 代码可直接运行
5. 包含单元测试',
'["coding_standards", "project_name", "tech_stack", "architecture_pattern", "code_context", "requirement_spec"]',
true, 1),

(1, '代码 Review 模板', 'code-review',
'你是一名严格的 Code Reviewer。你的任务是审查代码质量，找出问题并给出改进建议。

## 编码规范（审查基准）
{{coding_standards}}

## Review 规则
{{review_rules}}',
'请 Review 以下代码变更：

## 变更文件
{{diff_content}}

## Review 维度
1. 编码规范合规性
2. 安全漏洞
3. 性能问题
4. 逻辑正确性
5. 可维护性

请对每个问题给出：
- 严重级别 (ERROR/WARNING/INFO)
- 问题描述
- 改进建议
- 修复代码（如果可以自动修复）',
'["coding_standards", "review_rules", "diff_content"]',
true, 1),

(1, '测试生成模板', 'test-generation',
'你是一名测试工程师。你的任务是为给定代码生成全面的测试用例。

## 编码规范
{{coding_standards}}

## 技术栈
测试框架: {{test_framework}}
Mock 框架: {{mock_framework}}',
'请为以下代码生成测试：

## 源代码
{{source_code}}

## 要求
1. 覆盖正常路径和异常路径
2. 包含边界条件测试
3. Mock 外部依赖
4. 测试命名清晰描述场景
5. 目标覆盖率 > 80%',
'["coding_standards", "test_framework", "mock_framework", "source_code"]',
true, 1),

(1, '修复生成模板', 'fix-generation',
'你是一名高级工程师。你的任务是修复代码中的问题。

## 编码规范
{{coding_standards}}

## 项目上下文
{{code_context}}',
'请修复以下问题：

## 问题描述
{{issue_description}}

## 相关代码
{{related_code}}

## 错误日志
{{error_logs}}

请输出：
1. 根因分析
2. 修复方案
3. 修复后的代码
4. 验证方法',
'["coding_standards", "code_context", "issue_description", "related_code", "error_logs"]',
true, 1),

(1, '文档生成模板', 'doc-generation',
'你是一名技术文档工程师。你的任务是为代码生成清晰、完整的文档。

## 文档规范
- 使用 Markdown 格式
- 包含代码示例
- 面向开发者的技术文档',
'请为以下内容生成文档：

## 源代码/API
{{source_content}}

## 文档类型
{{doc_type}}

请包含：
1. 概述
2. 使用方法
3. 参数说明
4. 代码示例
5. 注意事项',
'["source_content", "doc_type"]',
true, 1);

-- Default review rules (tenant_id=1)
INSERT INTO specs.review_rules (tenant_id, name, category, scope, scope_id, rule_type, definition, severity, auto_fix, fix_template, created_by) VALUES
(1, '禁止空 catch 块', 'CODING', 'COMPANY', 0, 'PATTERN',
'{"pattern": "catch\\\\s*\\\\([^)]*\\\\)\\\\s*\\\\{\\\\s*\\\\}", "language": "java", "description": "catch 块不能为空，必须处理或记录异常"}',
'ERROR', true,
'catch (Exception e) {
    log.error("Unexpected error", e);
    throw new RuntimeException(e);
}', 1),

(1, '禁止 SELECT *', 'DATABASE', 'COMPANY', 0, 'PATTERN',
'{"pattern": "SELECT\\\\s+\\\\*\\\\s+FROM", "language": "sql", "description": "禁止使用 SELECT *，必须明确指定字段"}',
'ERROR', false, NULL, 1),

(1, 'API 响应必须使用 Result 包装', 'API_COMPAT', 'COMPANY', 0, 'AI_CHECK',
'{"check": "verify_response_wrapper", "description": "所有 API 响应必须使用 Result[T] 统一包装", "examples": ["return Result.ok(data)", "return Result.fail(message)"]}',
'WARNING', false, NULL, 1),

(1, '密码不能明文存储', 'SECURITY', 'COMPANY', 0, 'PATTERN',
'{"pattern": "password\\\\s*=\\\\s*[\"'']", "language": "*", "description": "密码不能以明文形式存储在代码中"}',
'ERROR', false, NULL, 1),

(1, 'SQL 注入防护', 'SECURITY', 'COMPANY', 0, 'PATTERN',
'{"pattern": "\".*\\\\+.*\".*execute|String\\\\.format.*execute", "language": "java", "description": "禁止拼接 SQL 字符串，必须使用参数化查询"}',
'ERROR', false, NULL, 1);

-- Default scaffold templates (tenant_id=1)
INSERT INTO specs.scaffold_templates (tenant_id, name, project_type, description, template_repo, variables, post_hooks, created_by) VALUES
(1, 'Java 微服务脚手架', 'JAVA_MICROSERVICE',
'Spring Boot 3 + MyBatis Plus + Redis + RocketMQ 微服务项目模板，包含标准分层架构、统一异常处理、API 文档生成。',
'https://codeup.aliyun.com/shulex/scaffold-java-microservice.git',
'["group_id", "artifact_id", "base_package", "service_port", "db_name"]',
'["mvn clean compile", "mvn spotless:apply"]', 1),

(1, 'Vue 3 前端脚手架', 'VUE_FRONTEND',
'Vue 3 + Vite + TypeScript + Element Plus + Pinia 前端项目模板，包含路由守卫、请求拦截、权限指令。',
'https://codeup.aliyun.com/shulex/scaffold-vue-frontend.git',
'["project_name", "api_base_url", "title"]',
'["npm install", "npm run lint:fix"]', 1),

(1, '全栈项目脚手架', 'FULLSTACK',
'前后端一体化项目模板：Spring Boot 后端 + Vue 3 前端 + Docker Compose 部署配置。',
'https://codeup.aliyun.com/shulex/scaffold-fullstack.git',
'["project_name", "group_id", "base_package", "db_name"]',
'["mvn clean compile", "cd frontend && npm install"]', 1),

(1, 'SDK 项目脚手架', 'SDK',
'Java SDK 项目模板，包含 Maven 发布配置、API 文档生成、版本管理。',
'https://codeup.aliyun.com/shulex/scaffold-sdk.git',
'["group_id", "artifact_id", "sdk_name"]',
'["mvn clean compile"]', 1);
