# S5 — 规范中心 (Specs Center)

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 交付平台级"规范中心"，包括编码规范、Prompt 模板、Review 规则、脚手架模板的 CRUD 管理，以及三级继承（公司 -> 团队 -> 项目）的规范解析能力。项目设置页可以覆盖上级规范。

**Architecture:** forge-core 新增 specs 模块（model/repository/service/handler），forge-portal 新增 /specs 页面群和项目设置中的规范覆盖页。

**Tech Stack:** Go 1.22 + Gin + pgx, Next.js 15 (App Router) + TypeScript + Tailwind CSS 4 + shadcn/ui, PostgreSQL 16, Redis 7

**Dependencies:** S1 (auth + login), S2 (project CRUD), S3 (GitHub OAuth), S4 (Temporal + task)

---

## 前置说明

### S5 交付后你可以做什么

1. 在侧边栏点击"规范中心"进入平台级规范管理
2. 在 Standards 标签页 CRUD 编码规范（按分类/作用域筛选）
3. 在 Prompts 标签页 CRUD Prompt 模板（含代码编辑器编辑系统提示词和用户模板）
4. 在 Rules 标签页 CRUD Review 规则（支持启用/禁用开关）
5. 在 Scaffolds 标签页查看脚手架模板（Phase 1 只读）
6. 规范支持三级继承：公司级 -> 团队级 -> 项目级
7. 在项目设置页查看继承的规范，点击"Override"创建项目级副本
8. 有效规范 API 返回解析后的完整规范集，Redis 缓存 10 分钟

---

## 文件结构

### forge-core（新增 specs 模块）

```
forge-core/
├── migrations/
│   └── 005_init_specs.sql              # specs schema DDL + seed data
├── internal/
│   └── module/
│       └── specs/
│           ├── model.go                # 数据模型（Standards, PromptTemplates, ReviewRules, ScaffoldTemplates）
│           ├── repository.go           # 数据库操作（CRUD + 继承查询）
│           ├── service.go              # 业务逻辑（三级继承解析 + Redis 缓存）
│           └── handler.go             # HTTP handlers（所有 /api/specs/* 端点）
```

### forge-portal（新增 specs 页面）

```
forge-portal/
├── app/
│   ├── (dashboard)/
│   │   ├── specs/
│   │   │   ├── page.tsx               # 规范中心主页（重定向到 standards）
│   │   │   ├── layout.tsx             # 规范中心布局（tabs 导航）
│   │   │   ├── standards/
│   │   │   │   └── page.tsx           # 编码规范列表 + 创建/编辑
│   │   │   ├── prompts/
│   │   │   │   └── page.tsx           # Prompt 模板列表 + 创建/编辑
│   │   │   ├── rules/
│   │   │   │   └── page.tsx           # Review 规则列表 + 启用/禁用
│   │   │   └── scaffolds/
│   │   │       └── page.tsx           # 脚手架模板列表（只读）
│   │   └── projects/
│   │       └── [id]/
│   │           └── settings/
│   │               └── specs/
│   │                   └── page.tsx   # 项目级规范覆盖页
│   └── ...
├── lib/
│   └── specs.ts                       # specs API 客户端函数
```

---

## Task 1: 数据库迁移 + 种子数据

**Files:**
- Create: `forge-core/migrations/005_init_specs.sql`

- [ ] **Step 1: 创建 specs schema 迁移文件**

`forge-core/migrations/005_init_specs.sql`：

```sql
-- =============================================
-- S5: Specs Center — 规范中心
-- =============================================

-- 1. 编码规范
CREATE TABLE IF NOT EXISTS specs.standards (
    id              BIGSERIAL PRIMARY KEY,
    tenant_id       BIGINT NOT NULL,
    name            VARCHAR(200) NOT NULL,
    category        VARCHAR(50) NOT NULL,      -- JAVA/SQL/REDIS/KAFKA/API/SECURITY/NAMING/GIT
    scope           VARCHAR(20) NOT NULL,       -- COMPANY/TEAM/PROJECT
    scope_id        BIGINT NOT NULL DEFAULT 0,  -- team_id or project_id (0 for COMPANY)
    parent_id       BIGINT REFERENCES specs.standards(id),
    content         TEXT NOT NULL,
    version         INT NOT NULL DEFAULT 1,
    status          VARCHAR(20) NOT NULL DEFAULT 'ACTIVE',
    created_by      BIGINT REFERENCES auth.users(id),
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_standards_tenant ON specs.standards(tenant_id);
CREATE INDEX idx_standards_category ON specs.standards(tenant_id, category);
CREATE INDEX idx_standards_scope ON specs.standards(tenant_id, scope, scope_id);

-- 2. Prompt 模板
CREATE TABLE IF NOT EXISTS specs.prompt_templates (
    id              BIGSERIAL PRIMARY KEY,
    tenant_id       BIGINT NOT NULL,
    name            VARCHAR(200) NOT NULL,
    purpose         VARCHAR(50) NOT NULL,       -- requirement-analysis/code-generation/code-review/test-generation/fix-generation/doc-generation
    system_prompt   TEXT NOT NULL,
    user_template   TEXT NOT NULL,
    variables       JSONB NOT NULL DEFAULT '[]',
    version         INT NOT NULL DEFAULT 1,
    is_default      BOOLEAN NOT NULL DEFAULT FALSE,
    created_by      BIGINT REFERENCES auth.users(id),
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_prompt_templates_tenant ON specs.prompt_templates(tenant_id);
CREATE INDEX idx_prompt_templates_purpose ON specs.prompt_templates(tenant_id, purpose);

-- 3. Review 规则
CREATE TABLE IF NOT EXISTS specs.review_rules (
    id              BIGSERIAL PRIMARY KEY,
    tenant_id       BIGINT NOT NULL,
    name            VARCHAR(200) NOT NULL,
    category        VARCHAR(50) NOT NULL,       -- CODING/SECURITY/PERFORMANCE/DATABASE/API_COMPAT/CUSTOM
    scope           VARCHAR(20) NOT NULL DEFAULT 'COMPANY',
    scope_id        BIGINT NOT NULL DEFAULT 0,
    rule_type       VARCHAR(20) NOT NULL,       -- PATTERN/AST/AI_CHECK
    definition      JSONB NOT NULL,
    severity        VARCHAR(10) NOT NULL,       -- ERROR/WARNING/INFO
    auto_fix        BOOLEAN NOT NULL DEFAULT FALSE,
    fix_template    TEXT,
    enabled         BOOLEAN NOT NULL DEFAULT TRUE,
    created_by      BIGINT REFERENCES auth.users(id),
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_review_rules_tenant ON specs.review_rules(tenant_id);
CREATE INDEX idx_review_rules_category ON specs.review_rules(tenant_id, category);
CREATE INDEX idx_review_rules_scope ON specs.review_rules(tenant_id, scope, scope_id);

-- 4. 脚手架模板
CREATE TABLE IF NOT EXISTS specs.scaffold_templates (
    id              BIGSERIAL PRIMARY KEY,
    tenant_id       BIGINT NOT NULL,
    name            VARCHAR(200) NOT NULL,
    project_type    VARCHAR(50) NOT NULL,       -- JAVA_MICROSERVICE/VUE_FRONTEND/FULLSTACK/SDK/BLANK
    description     TEXT,
    template_repo   TEXT,
    variables       JSONB NOT NULL DEFAULT '[]',
    post_hooks      JSONB NOT NULL DEFAULT '[]',
    version         INT NOT NULL DEFAULT 1,
    created_by      BIGINT REFERENCES auth.users(id),
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_scaffold_templates_tenant ON specs.scaffold_templates(tenant_id);

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
'catch (Exception e) {\n    log.error("Unexpected error", e);\n    throw new RuntimeException(e);\n}', 1),

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
```

- [ ] **Step 2: 执行迁移**

```bash
# 确保 PostgreSQL 正在运行
docker exec forge-postgres psql -U forge -d forge_main -f /dev/stdin < forge-core/migrations/005_init_specs.sql

# 或者手动连接执行
docker exec -i forge-postgres psql -U forge -d forge_main < forge-core/migrations/005_init_specs.sql
```

- [ ] **Step 3: 验证**

```bash
# 验证表创建
docker exec forge-postgres psql -U forge -d forge_main -c "\dt specs.*"
# 预期: 4 张表 — standards, prompt_templates, review_rules, scaffold_templates

# 验证种子数据
docker exec forge-postgres psql -U forge -d forge_main -c "SELECT count(*) FROM specs.standards;"
# 预期: 4

docker exec forge-postgres psql -U forge -d forge_main -c "SELECT count(*) FROM specs.prompt_templates;"
# 预期: 6

docker exec forge-postgres psql -U forge -d forge_main -c "SELECT count(*) FROM specs.review_rules;"
# 预期: 5

docker exec forge-postgres psql -U forge -d forge_main -c "SELECT count(*) FROM specs.scaffold_templates;"
# 预期: 4
```

- [ ] **Step 4: Commit**

```bash
git add forge-core/migrations/005_init_specs.sql
git commit -m "feat(s5): add specs center database migration and seed data"
```

---

## Task 2: Standards 模块（后端）

**Files:**
- Create: `forge-core/internal/module/specs/model.go`
- Create: `forge-core/internal/module/specs/repository.go`
- Create: `forge-core/internal/module/specs/service.go`
- Create: `forge-core/internal/module/specs/handler.go`
- Modify: `forge-core/internal/router/router.go` (注册 specs 路由)
- Modify: `forge-core/cmd/forge-core/main.go` (初始化 specs 模块)

- [ ] **Step 1: 创建 model.go — 所有 specs 数据模型**

`forge-core/internal/module/specs/model.go`：

```go
package specs

import "time"

// ==================== Standards ====================

type Standard struct {
	ID        int64     `json:"id"`
	TenantID  int64     `json:"tenantId"`
	Name      string    `json:"name"`
	Category  string    `json:"category"`
	Scope     string    `json:"scope"`
	ScopeID   int64     `json:"scopeId"`
	ParentID  *int64    `json:"parentId,omitempty"`
	Content   string    `json:"content"`
	Version   int       `json:"version"`
	Status    string    `json:"status"`
	CreatedBy *int64    `json:"createdBy,omitempty"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

type CreateStandardReq struct {
	Name     string `json:"name" binding:"required,max=200"`
	Category string `json:"category" binding:"required,oneof=JAVA SQL REDIS KAFKA API SECURITY NAMING GIT"`
	Scope    string `json:"scope" binding:"required,oneof=COMPANY TEAM PROJECT"`
	ScopeID  int64  `json:"scopeId"`
	ParentID *int64 `json:"parentId"`
	Content  string `json:"content" binding:"required"`
}

type UpdateStandardReq struct {
	Name    string `json:"name" binding:"required,max=200"`
	Content string `json:"content" binding:"required"`
}

type StandardFilter struct {
	Category string `form:"category"`
	Scope    string `form:"scope"`
	ScopeID  *int64 `form:"scopeId"`
	Status   string `form:"status"`
	Page     int    `form:"page,default=1"`
	PageSize int    `form:"pageSize,default=20"`
}

// ==================== Prompt Templates ====================

type PromptTemplate struct {
	ID           int64     `json:"id"`
	TenantID     int64     `json:"tenantId"`
	Name         string    `json:"name"`
	Purpose      string    `json:"purpose"`
	SystemPrompt string    `json:"systemPrompt"`
	UserTemplate string    `json:"userTemplate"`
	Variables    []string  `json:"variables"`
	Version      int       `json:"version"`
	IsDefault    bool      `json:"isDefault"`
	CreatedBy    *int64    `json:"createdBy,omitempty"`
	CreatedAt    time.Time `json:"createdAt"`
	UpdatedAt    time.Time `json:"updatedAt"`
}

type CreatePromptTemplateReq struct {
	Name         string   `json:"name" binding:"required,max=200"`
	Purpose      string   `json:"purpose" binding:"required,oneof=requirement-analysis code-generation code-review test-generation fix-generation doc-generation"`
	SystemPrompt string   `json:"systemPrompt" binding:"required"`
	UserTemplate string   `json:"userTemplate" binding:"required"`
	Variables    []string `json:"variables"`
	IsDefault    bool     `json:"isDefault"`
}

type UpdatePromptTemplateReq struct {
	Name         string   `json:"name" binding:"required,max=200"`
	Purpose      string   `json:"purpose" binding:"required,oneof=requirement-analysis code-generation code-review test-generation fix-generation doc-generation"`
	SystemPrompt string   `json:"systemPrompt" binding:"required"`
	UserTemplate string   `json:"userTemplate" binding:"required"`
	Variables    []string `json:"variables"`
	IsDefault    bool     `json:"isDefault"`
}

type PromptTemplateFilter struct {
	Purpose  string `form:"purpose"`
	Page     int    `form:"page,default=1"`
	PageSize int    `form:"pageSize,default=20"`
}

// ==================== Review Rules ====================

type ReviewRule struct {
	ID          int64                  `json:"id"`
	TenantID    int64                  `json:"tenantId"`
	Name        string                 `json:"name"`
	Category    string                 `json:"category"`
	Scope       string                 `json:"scope"`
	ScopeID     int64                  `json:"scopeId"`
	RuleType    string                 `json:"ruleType"`
	Definition  map[string]interface{} `json:"definition"`
	Severity    string                 `json:"severity"`
	AutoFix     bool                   `json:"autoFix"`
	FixTemplate *string                `json:"fixTemplate,omitempty"`
	Enabled     bool                   `json:"enabled"`
	CreatedBy   *int64                 `json:"createdBy,omitempty"`
	CreatedAt   time.Time              `json:"createdAt"`
	UpdatedAt   time.Time              `json:"updatedAt"`
}

type CreateReviewRuleReq struct {
	Name        string                 `json:"name" binding:"required,max=200"`
	Category    string                 `json:"category" binding:"required,oneof=CODING SECURITY PERFORMANCE DATABASE API_COMPAT CUSTOM"`
	Scope       string                 `json:"scope" binding:"required,oneof=COMPANY TEAM PROJECT"`
	ScopeID     int64                  `json:"scopeId"`
	RuleType    string                 `json:"ruleType" binding:"required,oneof=PATTERN AST AI_CHECK"`
	Definition  map[string]interface{} `json:"definition" binding:"required"`
	Severity    string                 `json:"severity" binding:"required,oneof=ERROR WARNING INFO"`
	AutoFix     bool                   `json:"autoFix"`
	FixTemplate *string                `json:"fixTemplate"`
}

type UpdateReviewRuleReq struct {
	Name        string                 `json:"name" binding:"required,max=200"`
	Category    string                 `json:"category" binding:"required,oneof=CODING SECURITY PERFORMANCE DATABASE API_COMPAT CUSTOM"`
	RuleType    string                 `json:"ruleType" binding:"required,oneof=PATTERN AST AI_CHECK"`
	Definition  map[string]interface{} `json:"definition" binding:"required"`
	Severity    string                 `json:"severity" binding:"required,oneof=ERROR WARNING INFO"`
	AutoFix     bool                   `json:"autoFix"`
	FixTemplate *string                `json:"fixTemplate"`
}

type ReviewRuleFilter struct {
	Category string `form:"category"`
	Severity string `form:"severity"`
	Scope    string `form:"scope"`
	ScopeID  *int64 `form:"scopeId"`
	Page     int    `form:"page,default=1"`
	PageSize int    `form:"pageSize,default=20"`
}

// ==================== Scaffold Templates ====================

type ScaffoldTemplate struct {
	ID           int64     `json:"id"`
	TenantID     int64     `json:"tenantId"`
	Name         string    `json:"name"`
	ProjectType  string    `json:"projectType"`
	Description  *string   `json:"description,omitempty"`
	TemplateRepo *string   `json:"templateRepo,omitempty"`
	Variables    []string  `json:"variables"`
	PostHooks    []string  `json:"postHooks"`
	Version      int       `json:"version"`
	CreatedBy    *int64    `json:"createdBy,omitempty"`
	CreatedAt    time.Time `json:"createdAt"`
	UpdatedAt    time.Time `json:"updatedAt"`
}

type ScaffoldFilter struct {
	ProjectType string `form:"projectType"`
	Page        int    `form:"page,default=1"`
	PageSize    int    `form:"pageSize,default=20"`
}

// ==================== Effective Specs ====================

type EffectiveSpecs struct {
	Standards []*Standard  `json:"standards"`
	Rules     []*ReviewRule `json:"rules"`
}

// ==================== Pagination ====================

type PageResult[T any] struct {
	Items    []T   `json:"items"`
	Total    int64 `json:"total"`
	Page     int   `json:"page"`
	PageSize int   `json:"pageSize"`
}
```

- [ ] **Step 2: 创建 repository.go — 数据库操作**

`forge-core/internal/module/specs/repository.go`：

```go
package specs

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

// ==================== Standards ====================

func (r *Repository) ListStandards(ctx context.Context, tenantID int64, f StandardFilter) (*PageResult[*Standard], error) {
	where := "WHERE tenant_id = @tenantID AND status = 'ACTIVE'"
	args := pgx.NamedArgs{"tenantID": tenantID}

	if f.Category != "" {
		where += " AND category = @category"
		args["category"] = f.Category
	}
	if f.Scope != "" {
		where += " AND scope = @scope"
		args["scope"] = f.Scope
	}
	if f.ScopeID != nil {
		where += " AND scope_id = @scopeID"
		args["scopeID"] = *f.ScopeID
	}

	var total int64
	countSQL := fmt.Sprintf("SELECT count(*) FROM specs.standards %s", where)
	if err := r.db.QueryRow(ctx, countSQL, args).Scan(&total); err != nil {
		return nil, fmt.Errorf("count standards: %w", err)
	}

	offset := (f.Page - 1) * f.PageSize
	args["limit"] = f.PageSize
	args["offset"] = offset

	query := fmt.Sprintf(`SELECT id, tenant_id, name, category, scope, scope_id, parent_id, content, version, status, created_by, created_at, updated_at
		FROM specs.standards %s ORDER BY created_at DESC LIMIT @limit OFFSET @offset`, where)

	rows, err := r.db.Query(ctx, query, args)
	if err != nil {
		return nil, fmt.Errorf("list standards: %w", err)
	}
	defer rows.Close()

	var items []*Standard
	for rows.Next() {
		s := &Standard{}
		if err := rows.Scan(&s.ID, &s.TenantID, &s.Name, &s.Category, &s.Scope, &s.ScopeID,
			&s.ParentID, &s.Content, &s.Version, &s.Status, &s.CreatedBy, &s.CreatedAt, &s.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan standard: %w", err)
		}
		items = append(items, s)
	}

	return &PageResult[*Standard]{Items: items, Total: total, Page: f.Page, PageSize: f.PageSize}, nil
}

func (r *Repository) GetStandard(ctx context.Context, tenantID, id int64) (*Standard, error) {
	s := &Standard{}
	err := r.db.QueryRow(ctx, `SELECT id, tenant_id, name, category, scope, scope_id, parent_id, content, version, status, created_by, created_at, updated_at
		FROM specs.standards WHERE id = $1 AND tenant_id = $2 AND status = 'ACTIVE'`, id, tenantID).
		Scan(&s.ID, &s.TenantID, &s.Name, &s.Category, &s.Scope, &s.ScopeID,
			&s.ParentID, &s.Content, &s.Version, &s.Status, &s.CreatedBy, &s.CreatedAt, &s.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("get standard: %w", err)
	}
	return s, nil
}

func (r *Repository) CreateStandard(ctx context.Context, tenantID int64, userID int64, req CreateStandardReq) (*Standard, error) {
	s := &Standard{}
	err := r.db.QueryRow(ctx, `INSERT INTO specs.standards (tenant_id, name, category, scope, scope_id, parent_id, content, created_by)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING id, tenant_id, name, category, scope, scope_id, parent_id, content, version, status, created_by, created_at, updated_at`,
		tenantID, req.Name, req.Category, req.Scope, req.ScopeID, req.ParentID, req.Content, userID).
		Scan(&s.ID, &s.TenantID, &s.Name, &s.Category, &s.Scope, &s.ScopeID,
			&s.ParentID, &s.Content, &s.Version, &s.Status, &s.CreatedBy, &s.CreatedAt, &s.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("create standard: %w", err)
	}
	return s, nil
}

func (r *Repository) UpdateStandard(ctx context.Context, tenantID, id int64, req UpdateStandardReq) (*Standard, error) {
	s := &Standard{}
	err := r.db.QueryRow(ctx, `UPDATE specs.standards SET name = $1, content = $2, version = version + 1, updated_at = NOW()
		WHERE id = $3 AND tenant_id = $4 AND status = 'ACTIVE'
		RETURNING id, tenant_id, name, category, scope, scope_id, parent_id, content, version, status, created_by, created_at, updated_at`,
		req.Name, req.Content, id, tenantID).
		Scan(&s.ID, &s.TenantID, &s.Name, &s.Category, &s.Scope, &s.ScopeID,
			&s.ParentID, &s.Content, &s.Version, &s.Status, &s.CreatedBy, &s.CreatedAt, &s.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("update standard: %w", err)
	}
	return s, nil
}

func (r *Repository) DeleteStandard(ctx context.Context, tenantID, id int64) error {
	_, err := r.db.Exec(ctx, `UPDATE specs.standards SET status = 'DELETED', updated_at = NOW() WHERE id = $1 AND tenant_id = $2`, id, tenantID)
	if err != nil {
		return fmt.Errorf("delete standard: %w", err)
	}
	return nil
}

// GetStandardsByScope retrieves standards for a specific scope (used in inheritance resolution)
func (r *Repository) GetStandardsByScope(ctx context.Context, tenantID int64, scope string, scopeID int64) ([]*Standard, error) {
	rows, err := r.db.Query(ctx, `SELECT id, tenant_id, name, category, scope, scope_id, parent_id, content, version, status, created_by, created_at, updated_at
		FROM specs.standards WHERE tenant_id = $1 AND scope = $2 AND scope_id = $3 AND status = 'ACTIVE'
		ORDER BY category`, tenantID, scope, scopeID)
	if err != nil {
		return nil, fmt.Errorf("get standards by scope: %w", err)
	}
	defer rows.Close()

	var items []*Standard
	for rows.Next() {
		s := &Standard{}
		if err := rows.Scan(&s.ID, &s.TenantID, &s.Name, &s.Category, &s.Scope, &s.ScopeID,
			&s.ParentID, &s.Content, &s.Version, &s.Status, &s.CreatedBy, &s.CreatedAt, &s.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan standard: %w", err)
		}
		items = append(items, s)
	}
	return items, nil
}

// ==================== Prompt Templates ====================

func (r *Repository) ListPromptTemplates(ctx context.Context, tenantID int64, f PromptTemplateFilter) (*PageResult[*PromptTemplate], error) {
	where := "WHERE tenant_id = @tenantID"
	args := pgx.NamedArgs{"tenantID": tenantID}

	if f.Purpose != "" {
		where += " AND purpose = @purpose"
		args["purpose"] = f.Purpose
	}

	var total int64
	countSQL := fmt.Sprintf("SELECT count(*) FROM specs.prompt_templates %s", where)
	if err := r.db.QueryRow(ctx, countSQL, args).Scan(&total); err != nil {
		return nil, fmt.Errorf("count prompt templates: %w", err)
	}

	offset := (f.Page - 1) * f.PageSize
	args["limit"] = f.PageSize
	args["offset"] = offset

	query := fmt.Sprintf(`SELECT id, tenant_id, name, purpose, system_prompt, user_template, variables, version, is_default, created_by, created_at, updated_at
		FROM specs.prompt_templates %s ORDER BY is_default DESC, created_at DESC LIMIT @limit OFFSET @offset`, where)

	rows, err := r.db.Query(ctx, query, args)
	if err != nil {
		return nil, fmt.Errorf("list prompt templates: %w", err)
	}
	defer rows.Close()

	var items []*PromptTemplate
	for rows.Next() {
		p := &PromptTemplate{}
		var varsJSON []byte
		if err := rows.Scan(&p.ID, &p.TenantID, &p.Name, &p.Purpose, &p.SystemPrompt, &p.UserTemplate,
			&varsJSON, &p.Version, &p.IsDefault, &p.CreatedBy, &p.CreatedAt, &p.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan prompt template: %w", err)
		}
		if err := json.Unmarshal(varsJSON, &p.Variables); err != nil {
			p.Variables = []string{}
		}
		items = append(items, p)
	}

	return &PageResult[*PromptTemplate]{Items: items, Total: total, Page: f.Page, PageSize: f.PageSize}, nil
}

func (r *Repository) GetPromptTemplate(ctx context.Context, tenantID, id int64) (*PromptTemplate, error) {
	p := &PromptTemplate{}
	var varsJSON []byte
	err := r.db.QueryRow(ctx, `SELECT id, tenant_id, name, purpose, system_prompt, user_template, variables, version, is_default, created_by, created_at, updated_at
		FROM specs.prompt_templates WHERE id = $1 AND tenant_id = $2`, id, tenantID).
		Scan(&p.ID, &p.TenantID, &p.Name, &p.Purpose, &p.SystemPrompt, &p.UserTemplate,
			&varsJSON, &p.Version, &p.IsDefault, &p.CreatedBy, &p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("get prompt template: %w", err)
	}
	if err := json.Unmarshal(varsJSON, &p.Variables); err != nil {
		p.Variables = []string{}
	}
	return p, nil
}

func (r *Repository) CreatePromptTemplate(ctx context.Context, tenantID, userID int64, req CreatePromptTemplateReq) (*PromptTemplate, error) {
	varsJSON, err := json.Marshal(req.Variables)
	if err != nil {
		return nil, fmt.Errorf("marshal variables: %w", err)
	}

	p := &PromptTemplate{}
	var retVarsJSON []byte
	err = r.db.QueryRow(ctx, `INSERT INTO specs.prompt_templates (tenant_id, name, purpose, system_prompt, user_template, variables, is_default, created_by)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING id, tenant_id, name, purpose, system_prompt, user_template, variables, version, is_default, created_by, created_at, updated_at`,
		tenantID, req.Name, req.Purpose, req.SystemPrompt, req.UserTemplate, varsJSON, req.IsDefault, userID).
		Scan(&p.ID, &p.TenantID, &p.Name, &p.Purpose, &p.SystemPrompt, &p.UserTemplate,
			&retVarsJSON, &p.Version, &p.IsDefault, &p.CreatedBy, &p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("create prompt template: %w", err)
	}
	if err := json.Unmarshal(retVarsJSON, &p.Variables); err != nil {
		p.Variables = []string{}
	}
	return p, nil
}

func (r *Repository) UpdatePromptTemplate(ctx context.Context, tenantID, id int64, req UpdatePromptTemplateReq) (*PromptTemplate, error) {
	varsJSON, err := json.Marshal(req.Variables)
	if err != nil {
		return nil, fmt.Errorf("marshal variables: %w", err)
	}

	p := &PromptTemplate{}
	var retVarsJSON []byte
	err = r.db.QueryRow(ctx, `UPDATE specs.prompt_templates
		SET name = $1, purpose = $2, system_prompt = $3, user_template = $4, variables = $5, is_default = $6, version = version + 1, updated_at = NOW()
		WHERE id = $7 AND tenant_id = $8
		RETURNING id, tenant_id, name, purpose, system_prompt, user_template, variables, version, is_default, created_by, created_at, updated_at`,
		req.Name, req.Purpose, req.SystemPrompt, req.UserTemplate, varsJSON, req.IsDefault, id, tenantID).
		Scan(&p.ID, &p.TenantID, &p.Name, &p.Purpose, &p.SystemPrompt, &p.UserTemplate,
			&retVarsJSON, &p.Version, &p.IsDefault, &p.CreatedBy, &p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("update prompt template: %w", err)
	}
	if err := json.Unmarshal(retVarsJSON, &p.Variables); err != nil {
		p.Variables = []string{}
	}
	return p, nil
}

func (r *Repository) DeletePromptTemplate(ctx context.Context, tenantID, id int64) error {
	_, err := r.db.Exec(ctx, `DELETE FROM specs.prompt_templates WHERE id = $1 AND tenant_id = $2`, id, tenantID)
	if err != nil {
		return fmt.Errorf("delete prompt template: %w", err)
	}
	return nil
}

// ==================== Review Rules ====================

func (r *Repository) ListReviewRules(ctx context.Context, tenantID int64, f ReviewRuleFilter) (*PageResult[*ReviewRule], error) {
	where := "WHERE tenant_id = @tenantID"
	args := pgx.NamedArgs{"tenantID": tenantID}

	if f.Category != "" {
		where += " AND category = @category"
		args["category"] = f.Category
	}
	if f.Severity != "" {
		where += " AND severity = @severity"
		args["severity"] = f.Severity
	}
	if f.Scope != "" {
		where += " AND scope = @scope"
		args["scope"] = f.Scope
	}
	if f.ScopeID != nil {
		where += " AND scope_id = @scopeID"
		args["scopeID"] = *f.ScopeID
	}

	var total int64
	countSQL := fmt.Sprintf("SELECT count(*) FROM specs.review_rules %s", where)
	if err := r.db.QueryRow(ctx, countSQL, args).Scan(&total); err != nil {
		return nil, fmt.Errorf("count review rules: %w", err)
	}

	offset := (f.Page - 1) * f.PageSize
	args["limit"] = f.PageSize
	args["offset"] = offset

	query := fmt.Sprintf(`SELECT id, tenant_id, name, category, scope, scope_id, rule_type, definition, severity, auto_fix, fix_template, enabled, created_by, created_at, updated_at
		FROM specs.review_rules %s ORDER BY severity, created_at DESC LIMIT @limit OFFSET @offset`, where)

	rows, err := r.db.Query(ctx, query, args)
	if err != nil {
		return nil, fmt.Errorf("list review rules: %w", err)
	}
	defer rows.Close()

	var items []*ReviewRule
	for rows.Next() {
		rule := &ReviewRule{}
		var defJSON []byte
		if err := rows.Scan(&rule.ID, &rule.TenantID, &rule.Name, &rule.Category, &rule.Scope, &rule.ScopeID,
			&rule.RuleType, &defJSON, &rule.Severity, &rule.AutoFix, &rule.FixTemplate, &rule.Enabled,
			&rule.CreatedBy, &rule.CreatedAt, &rule.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan review rule: %w", err)
		}
		if err := json.Unmarshal(defJSON, &rule.Definition); err != nil {
			rule.Definition = map[string]interface{}{}
		}
		items = append(items, rule)
	}

	return &PageResult[*ReviewRule]{Items: items, Total: total, Page: f.Page, PageSize: f.PageSize}, nil
}

func (r *Repository) GetReviewRule(ctx context.Context, tenantID, id int64) (*ReviewRule, error) {
	rule := &ReviewRule{}
	var defJSON []byte
	err := r.db.QueryRow(ctx, `SELECT id, tenant_id, name, category, scope, scope_id, rule_type, definition, severity, auto_fix, fix_template, enabled, created_by, created_at, updated_at
		FROM specs.review_rules WHERE id = $1 AND tenant_id = $2`, id, tenantID).
		Scan(&rule.ID, &rule.TenantID, &rule.Name, &rule.Category, &rule.Scope, &rule.ScopeID,
			&rule.RuleType, &defJSON, &rule.Severity, &rule.AutoFix, &rule.FixTemplate, &rule.Enabled,
			&rule.CreatedBy, &rule.CreatedAt, &rule.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("get review rule: %w", err)
	}
	if err := json.Unmarshal(defJSON, &rule.Definition); err != nil {
		rule.Definition = map[string]interface{}{}
	}
	return rule, nil
}

func (r *Repository) CreateReviewRule(ctx context.Context, tenantID, userID int64, req CreateReviewRuleReq) (*ReviewRule, error) {
	defJSON, err := json.Marshal(req.Definition)
	if err != nil {
		return nil, fmt.Errorf("marshal definition: %w", err)
	}

	rule := &ReviewRule{}
	var retDefJSON []byte
	err = r.db.QueryRow(ctx, `INSERT INTO specs.review_rules (tenant_id, name, category, scope, scope_id, rule_type, definition, severity, auto_fix, fix_template, created_by)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		RETURNING id, tenant_id, name, category, scope, scope_id, rule_type, definition, severity, auto_fix, fix_template, enabled, created_by, created_at, updated_at`,
		tenantID, req.Name, req.Category, req.Scope, req.ScopeID, req.RuleType, defJSON, req.Severity, req.AutoFix, req.FixTemplate, userID).
		Scan(&rule.ID, &rule.TenantID, &rule.Name, &rule.Category, &rule.Scope, &rule.ScopeID,
			&rule.RuleType, &retDefJSON, &rule.Severity, &rule.AutoFix, &rule.FixTemplate, &rule.Enabled,
			&rule.CreatedBy, &rule.CreatedAt, &rule.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("create review rule: %w", err)
	}
	if err := json.Unmarshal(retDefJSON, &rule.Definition); err != nil {
		rule.Definition = map[string]interface{}{}
	}
	return rule, nil
}

func (r *Repository) UpdateReviewRule(ctx context.Context, tenantID, id int64, req UpdateReviewRuleReq) (*ReviewRule, error) {
	defJSON, err := json.Marshal(req.Definition)
	if err != nil {
		return nil, fmt.Errorf("marshal definition: %w", err)
	}

	rule := &ReviewRule{}
	var retDefJSON []byte
	err = r.db.QueryRow(ctx, `UPDATE specs.review_rules
		SET name = $1, category = $2, rule_type = $3, definition = $4, severity = $5, auto_fix = $6, fix_template = $7, updated_at = NOW()
		WHERE id = $8 AND tenant_id = $9
		RETURNING id, tenant_id, name, category, scope, scope_id, rule_type, definition, severity, auto_fix, fix_template, enabled, created_by, created_at, updated_at`,
		req.Name, req.Category, req.RuleType, defJSON, req.Severity, req.AutoFix, req.FixTemplate, id, tenantID).
		Scan(&rule.ID, &rule.TenantID, &rule.Name, &rule.Category, &rule.Scope, &rule.ScopeID,
			&rule.RuleType, &retDefJSON, &rule.Severity, &rule.AutoFix, &rule.FixTemplate, &rule.Enabled,
			&rule.CreatedBy, &rule.CreatedAt, &rule.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("update review rule: %w", err)
	}
	if err := json.Unmarshal(retDefJSON, &rule.Definition); err != nil {
		rule.Definition = map[string]interface{}{}
	}
	return rule, nil
}

func (r *Repository) ToggleReviewRule(ctx context.Context, tenantID, id int64) (*ReviewRule, error) {
	rule := &ReviewRule{}
	var defJSON []byte
	err := r.db.QueryRow(ctx, `UPDATE specs.review_rules SET enabled = NOT enabled, updated_at = NOW()
		WHERE id = $1 AND tenant_id = $2
		RETURNING id, tenant_id, name, category, scope, scope_id, rule_type, definition, severity, auto_fix, fix_template, enabled, created_by, created_at, updated_at`,
		id, tenantID).
		Scan(&rule.ID, &rule.TenantID, &rule.Name, &rule.Category, &rule.Scope, &rule.ScopeID,
			&rule.RuleType, &defJSON, &rule.Severity, &rule.AutoFix, &rule.FixTemplate, &rule.Enabled,
			&rule.CreatedBy, &rule.CreatedAt, &rule.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("toggle review rule: %w", err)
	}
	if err := json.Unmarshal(defJSON, &rule.Definition); err != nil {
		rule.Definition = map[string]interface{}{}
	}
	return rule, nil
}

// GetReviewRulesByScope retrieves review rules for a specific scope (used in inheritance resolution)
func (r *Repository) GetReviewRulesByScope(ctx context.Context, tenantID int64, scope string, scopeID int64) ([]*ReviewRule, error) {
	rows, err := r.db.Query(ctx, `SELECT id, tenant_id, name, category, scope, scope_id, rule_type, definition, severity, auto_fix, fix_template, enabled, created_by, created_at, updated_at
		FROM specs.review_rules WHERE tenant_id = $1 AND scope = $2 AND scope_id = $3 AND enabled = TRUE
		ORDER BY severity, category`, tenantID, scope, scopeID)
	if err != nil {
		return nil, fmt.Errorf("get review rules by scope: %w", err)
	}
	defer rows.Close()

	var items []*ReviewRule
	for rows.Next() {
		rule := &ReviewRule{}
		var defJSON []byte
		if err := rows.Scan(&rule.ID, &rule.TenantID, &rule.Name, &rule.Category, &rule.Scope, &rule.ScopeID,
			&rule.RuleType, &defJSON, &rule.Severity, &rule.AutoFix, &rule.FixTemplate, &rule.Enabled,
			&rule.CreatedBy, &rule.CreatedAt, &rule.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan review rule: %w", err)
		}
		if err := json.Unmarshal(defJSON, &rule.Definition); err != nil {
			rule.Definition = map[string]interface{}{}
		}
		items = append(items, rule)
	}
	return items, nil
}

// ==================== Scaffold Templates ====================

func (r *Repository) ListScaffoldTemplates(ctx context.Context, tenantID int64, f ScaffoldFilter) (*PageResult[*ScaffoldTemplate], error) {
	where := "WHERE tenant_id = @tenantID"
	args := pgx.NamedArgs{"tenantID": tenantID}

	if f.ProjectType != "" {
		where += " AND project_type = @projectType"
		args["projectType"] = f.ProjectType
	}

	var total int64
	countSQL := fmt.Sprintf("SELECT count(*) FROM specs.scaffold_templates %s", where)
	if err := r.db.QueryRow(ctx, countSQL, args).Scan(&total); err != nil {
		return nil, fmt.Errorf("count scaffold templates: %w", err)
	}

	offset := (f.Page - 1) * f.PageSize
	args["limit"] = f.PageSize
	args["offset"] = offset

	query := fmt.Sprintf(`SELECT id, tenant_id, name, project_type, description, template_repo, variables, post_hooks, version, created_by, created_at, updated_at
		FROM specs.scaffold_templates %s ORDER BY created_at DESC LIMIT @limit OFFSET @offset`, where)

	rows, err := r.db.Query(ctx, query, args)
	if err != nil {
		return nil, fmt.Errorf("list scaffold templates: %w", err)
	}
	defer rows.Close()

	var items []*ScaffoldTemplate
	for rows.Next() {
		st := &ScaffoldTemplate{}
		var varsJSON, hooksJSON []byte
		if err := rows.Scan(&st.ID, &st.TenantID, &st.Name, &st.ProjectType, &st.Description, &st.TemplateRepo,
			&varsJSON, &hooksJSON, &st.Version, &st.CreatedBy, &st.CreatedAt, &st.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan scaffold template: %w", err)
		}
		if err := json.Unmarshal(varsJSON, &st.Variables); err != nil {
			st.Variables = []string{}
		}
		if err := json.Unmarshal(hooksJSON, &st.PostHooks); err != nil {
			st.PostHooks = []string{}
		}
		items = append(items, st)
	}

	return &PageResult[*ScaffoldTemplate]{Items: items, Total: total, Page: f.Page, PageSize: f.PageSize}, nil
}

func (r *Repository) GetScaffoldTemplate(ctx context.Context, tenantID, id int64) (*ScaffoldTemplate, error) {
	st := &ScaffoldTemplate{}
	var varsJSON, hooksJSON []byte
	err := r.db.QueryRow(ctx, `SELECT id, tenant_id, name, project_type, description, template_repo, variables, post_hooks, version, created_by, created_at, updated_at
		FROM specs.scaffold_templates WHERE id = $1 AND tenant_id = $2`, id, tenantID).
		Scan(&st.ID, &st.TenantID, &st.Name, &st.ProjectType, &st.Description, &st.TemplateRepo,
			&varsJSON, &hooksJSON, &st.Version, &st.CreatedBy, &st.CreatedAt, &st.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("get scaffold template: %w", err)
	}
	if err := json.Unmarshal(varsJSON, &st.Variables); err != nil {
		st.Variables = []string{}
	}
	if err := json.Unmarshal(hooksJSON, &st.PostHooks); err != nil {
		st.PostHooks = []string{}
	}
	return st, nil
}
```

- [ ] **Step 3: 创建 service.go — 业务逻辑 + 三级继承解析 + Redis 缓存**

`forge-core/internal/module/specs/service.go`：

```go
package specs

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	effectiveSpecsCacheTTL    = 10 * time.Minute
	effectiveSpecsCachePrefix = "specs:effective:"
)

type Service struct {
	repo  *Repository
	redis *redis.Client
}

func NewService(repo *Repository, redis *redis.Client) *Service {
	return &Service{repo: repo, redis: redis}
}

// ==================== Standards ====================

func (s *Service) ListStandards(ctx context.Context, tenantID int64, f StandardFilter) (*PageResult[*Standard], error) {
	if f.Page < 1 {
		f.Page = 1
	}
	if f.PageSize < 1 || f.PageSize > 100 {
		f.PageSize = 20
	}
	return s.repo.ListStandards(ctx, tenantID, f)
}

func (s *Service) GetStandard(ctx context.Context, tenantID, id int64) (*Standard, error) {
	return s.repo.GetStandard(ctx, tenantID, id)
}

func (s *Service) CreateStandard(ctx context.Context, tenantID, userID int64, req CreateStandardReq) (*Standard, error) {
	result, err := s.repo.CreateStandard(ctx, tenantID, userID, req)
	if err != nil {
		return nil, err
	}
	s.invalidateEffectiveCache(ctx, tenantID)
	return result, nil
}

func (s *Service) UpdateStandard(ctx context.Context, tenantID, id int64, req UpdateStandardReq) (*Standard, error) {
	result, err := s.repo.UpdateStandard(ctx, tenantID, id, req)
	if err != nil {
		return nil, err
	}
	s.invalidateEffectiveCache(ctx, tenantID)
	return result, nil
}

func (s *Service) DeleteStandard(ctx context.Context, tenantID, id int64) error {
	if err := s.repo.DeleteStandard(ctx, tenantID, id); err != nil {
		return err
	}
	s.invalidateEffectiveCache(ctx, tenantID)
	return nil
}

// ==================== Prompt Templates ====================

func (s *Service) ListPromptTemplates(ctx context.Context, tenantID int64, f PromptTemplateFilter) (*PageResult[*PromptTemplate], error) {
	if f.Page < 1 {
		f.Page = 1
	}
	if f.PageSize < 1 || f.PageSize > 100 {
		f.PageSize = 20
	}
	return s.repo.ListPromptTemplates(ctx, tenantID, f)
}

func (s *Service) GetPromptTemplate(ctx context.Context, tenantID, id int64) (*PromptTemplate, error) {
	return s.repo.GetPromptTemplate(ctx, tenantID, id)
}

func (s *Service) CreatePromptTemplate(ctx context.Context, tenantID, userID int64, req CreatePromptTemplateReq) (*PromptTemplate, error) {
	if req.Variables == nil {
		req.Variables = []string{}
	}
	return s.repo.CreatePromptTemplate(ctx, tenantID, userID, req)
}

func (s *Service) UpdatePromptTemplate(ctx context.Context, tenantID, id int64, req UpdatePromptTemplateReq) (*PromptTemplate, error) {
	if req.Variables == nil {
		req.Variables = []string{}
	}
	return s.repo.UpdatePromptTemplate(ctx, tenantID, id, req)
}

func (s *Service) DeletePromptTemplate(ctx context.Context, tenantID, id int64) error {
	return s.repo.DeletePromptTemplate(ctx, tenantID, id)
}

// ==================== Review Rules ====================

func (s *Service) ListReviewRules(ctx context.Context, tenantID int64, f ReviewRuleFilter) (*PageResult[*ReviewRule], error) {
	if f.Page < 1 {
		f.Page = 1
	}
	if f.PageSize < 1 || f.PageSize > 100 {
		f.PageSize = 20
	}
	return s.repo.ListReviewRules(ctx, tenantID, f)
}

func (s *Service) GetReviewRule(ctx context.Context, tenantID, id int64) (*ReviewRule, error) {
	return s.repo.GetReviewRule(ctx, tenantID, id)
}

func (s *Service) CreateReviewRule(ctx context.Context, tenantID, userID int64, req CreateReviewRuleReq) (*ReviewRule, error) {
	result, err := s.repo.CreateReviewRule(ctx, tenantID, userID, req)
	if err != nil {
		return nil, err
	}
	s.invalidateEffectiveCache(ctx, tenantID)
	return result, nil
}

func (s *Service) UpdateReviewRule(ctx context.Context, tenantID, id int64, req UpdateReviewRuleReq) (*ReviewRule, error) {
	result, err := s.repo.UpdateReviewRule(ctx, tenantID, id, req)
	if err != nil {
		return nil, err
	}
	s.invalidateEffectiveCache(ctx, tenantID)
	return result, nil
}

func (s *Service) ToggleReviewRule(ctx context.Context, tenantID, id int64) (*ReviewRule, error) {
	result, err := s.repo.ToggleReviewRule(ctx, tenantID, id)
	if err != nil {
		return nil, err
	}
	s.invalidateEffectiveCache(ctx, tenantID)
	return result, nil
}

// ==================== Scaffold Templates ====================

func (s *Service) ListScaffoldTemplates(ctx context.Context, tenantID int64, f ScaffoldFilter) (*PageResult[*ScaffoldTemplate], error) {
	if f.Page < 1 {
		f.Page = 1
	}
	if f.PageSize < 1 || f.PageSize > 100 {
		f.PageSize = 20
	}
	return s.repo.ListScaffoldTemplates(ctx, tenantID, f)
}

func (s *Service) GetScaffoldTemplate(ctx context.Context, tenantID, id int64) (*ScaffoldTemplate, error) {
	return s.repo.GetScaffoldTemplate(ctx, tenantID, id)
}

// ==================== Effective Specs (Three-Level Inheritance) ====================

// GetEffectiveSpecs resolves the three-level inheritance chain: COMPANY -> TEAM -> PROJECT.
// Project-level specs override team-level, which override company-level.
// The result is cached in Redis with a 10-minute TTL.
func (s *Service) GetEffectiveSpecs(ctx context.Context, tenantID, projectID int64) (*EffectiveSpecs, error) {
	cacheKey := fmt.Sprintf("%s%d:%d", effectiveSpecsCachePrefix, tenantID, projectID)

	// Try cache first
	cached, err := s.redis.Get(ctx, cacheKey).Bytes()
	if err == nil {
		var result EffectiveSpecs
		if json.Unmarshal(cached, &result) == nil {
			return &result, nil
		}
	}

	// Resolve inheritance: company (scope_id=0) -> project (scope_id=projectID)
	// Note: team-level is resolved via parent_id linkage; for Phase 1 we resolve COMPANY + PROJECT scopes directly.
	companyStandards, err := s.repo.GetStandardsByScope(ctx, tenantID, "COMPANY", 0)
	if err != nil {
		return nil, fmt.Errorf("get company standards: %w", err)
	}
	projectStandards, err := s.repo.GetStandardsByScope(ctx, tenantID, "PROJECT", projectID)
	if err != nil {
		return nil, fmt.Errorf("get project standards: %w", err)
	}

	// Merge: project standards override company standards (by category)
	standardsByCategory := make(map[string]*Standard)
	for _, std := range companyStandards {
		standardsByCategory[std.Category] = std
	}
	for _, std := range projectStandards {
		standardsByCategory[std.Category] = std // override
	}
	mergedStandards := make([]*Standard, 0, len(standardsByCategory))
	for _, std := range standardsByCategory {
		mergedStandards = append(mergedStandards, std)
	}

	// Resolve review rules similarly
	companyRules, err := s.repo.GetReviewRulesByScope(ctx, tenantID, "COMPANY", 0)
	if err != nil {
		return nil, fmt.Errorf("get company rules: %w", err)
	}
	projectRules, err := s.repo.GetReviewRulesByScope(ctx, tenantID, "PROJECT", projectID)
	if err != nil {
		return nil, fmt.Errorf("get project rules: %w", err)
	}

	// Merge: project rules override company rules (by name within same category)
	ruleKey := func(r *ReviewRule) string { return r.Category + ":" + r.Name }
	rulesByKey := make(map[string]*ReviewRule)
	for _, rule := range companyRules {
		rulesByKey[ruleKey(rule)] = rule
	}
	for _, rule := range projectRules {
		rulesByKey[ruleKey(rule)] = rule // override
	}
	mergedRules := make([]*ReviewRule, 0, len(rulesByKey))
	for _, rule := range rulesByKey {
		mergedRules = append(mergedRules, rule)
	}

	result := &EffectiveSpecs{
		Standards: mergedStandards,
		Rules:     mergedRules,
	}

	// Cache the result
	if data, err := json.Marshal(result); err == nil {
		if err := s.redis.Set(ctx, cacheKey, data, effectiveSpecsCacheTTL).Err(); err != nil {
			slog.Warn("failed to cache effective specs", "error", err)
		}
	}

	return result, nil
}

// invalidateEffectiveCache removes all effective specs cache entries for a tenant.
func (s *Service) invalidateEffectiveCache(ctx context.Context, tenantID int64) {
	pattern := fmt.Sprintf("%s%d:*", effectiveSpecsCachePrefix, tenantID)
	iter := s.redis.Scan(ctx, 0, pattern, 100).Iterator()
	for iter.Next(ctx) {
		if err := s.redis.Del(ctx, iter.Val()).Err(); err != nil {
			slog.Warn("failed to invalidate cache", "key", iter.Val(), "error", err)
		}
	}
}
```

- [ ] **Step 4: 创建 handler.go — HTTP handlers**

`forge-core/internal/module/specs/handler.go`：

```go
package specs

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/shulex/forge/forge-core/internal/pkg/response"
)

type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

// RegisterRoutes registers all /api/specs/* routes
func (h *Handler) RegisterRoutes(r *gin.RouterGroup) {
	specs := r.Group("/specs")
	{
		// Standards
		standards := specs.Group("/standards")
		standards.GET("", h.ListStandards)
		standards.GET("/:id", h.GetStandard)
		standards.POST("", h.CreateStandard)
		standards.PUT("/:id", h.UpdateStandard)
		standards.DELETE("/:id", h.DeleteStandard)

		// Prompt Templates
		prompts := specs.Group("/prompts")
		prompts.GET("", h.ListPromptTemplates)
		prompts.GET("/:id", h.GetPromptTemplate)
		prompts.POST("", h.CreatePromptTemplate)
		prompts.PUT("/:id", h.UpdatePromptTemplate)
		prompts.DELETE("/:id", h.DeletePromptTemplate)

		// Review Rules
		rules := specs.Group("/rules")
		rules.GET("", h.ListReviewRules)
		rules.GET("/:id", h.GetReviewRule)
		rules.POST("", h.CreateReviewRule)
		rules.PUT("/:id", h.UpdateReviewRule)
		rules.DELETE("/:id", h.ToggleReviewRule)

		// Scaffold Templates (read-only)
		scaffolds := specs.Group("/scaffolds")
		scaffolds.GET("", h.ListScaffoldTemplates)
		scaffolds.GET("/:id", h.GetScaffoldTemplate)

		// Effective specs (resolved inheritance)
		specs.GET("/effective/:projectId", h.GetEffectiveSpecs)
	}
}

// Helper: extract tenant_id from JWT context (set by auth middleware)
func getTenantID(c *gin.Context) int64 {
	if v, ok := c.Get("tenantId"); ok {
		if tid, ok := v.(int64); ok {
			return tid
		}
	}
	return 1 // default tenant
}

// Helper: extract user_id from JWT context
func getUserID(c *gin.Context) int64 {
	if v, ok := c.Get("userId"); ok {
		if uid, ok := v.(int64); ok {
			return uid
		}
	}
	return 0
}

// Helper: parse path param :id as int64
func parseID(c *gin.Context, param string) (int64, bool) {
	idStr := c.Param(param)
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		response.Fail(c, http.StatusBadRequest, "invalid id parameter")
		return 0, false
	}
	return id, true
}

// ==================== Standards Handlers ====================

func (h *Handler) ListStandards(c *gin.Context) {
	tenantID := getTenantID(c)
	var filter StandardFilter
	if err := c.ShouldBindQuery(&filter); err != nil {
		response.Fail(c, http.StatusBadRequest, "invalid query parameters: "+err.Error())
		return
	}
	result, err := h.svc.ListStandards(c.Request.Context(), tenantID, filter)
	if err != nil {
		response.Fail(c, http.StatusInternalServerError, "failed to list standards: "+err.Error())
		return
	}
	response.OK(c, result)
}

func (h *Handler) GetStandard(c *gin.Context) {
	tenantID := getTenantID(c)
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	result, err := h.svc.GetStandard(c.Request.Context(), tenantID, id)
	if err != nil {
		response.Fail(c, http.StatusNotFound, "standard not found")
		return
	}
	response.OK(c, result)
}

func (h *Handler) CreateStandard(c *gin.Context) {
	tenantID := getTenantID(c)
	userID := getUserID(c)
	var req CreateStandardReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, "invalid request: "+err.Error())
		return
	}
	result, err := h.svc.CreateStandard(c.Request.Context(), tenantID, userID, req)
	if err != nil {
		response.Fail(c, http.StatusInternalServerError, "failed to create standard: "+err.Error())
		return
	}
	response.OK(c, result)
}

func (h *Handler) UpdateStandard(c *gin.Context) {
	tenantID := getTenantID(c)
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	var req UpdateStandardReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, "invalid request: "+err.Error())
		return
	}
	result, err := h.svc.UpdateStandard(c.Request.Context(), tenantID, id, req)
	if err != nil {
		response.Fail(c, http.StatusInternalServerError, "failed to update standard: "+err.Error())
		return
	}
	response.OK(c, result)
}

func (h *Handler) DeleteStandard(c *gin.Context) {
	tenantID := getTenantID(c)
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	if err := h.svc.DeleteStandard(c.Request.Context(), tenantID, id); err != nil {
		response.Fail(c, http.StatusInternalServerError, "failed to delete standard: "+err.Error())
		return
	}
	response.OK(c, nil)
}

// ==================== Prompt Templates Handlers ====================

func (h *Handler) ListPromptTemplates(c *gin.Context) {
	tenantID := getTenantID(c)
	var filter PromptTemplateFilter
	if err := c.ShouldBindQuery(&filter); err != nil {
		response.Fail(c, http.StatusBadRequest, "invalid query parameters: "+err.Error())
		return
	}
	result, err := h.svc.ListPromptTemplates(c.Request.Context(), tenantID, filter)
	if err != nil {
		response.Fail(c, http.StatusInternalServerError, "failed to list prompt templates: "+err.Error())
		return
	}
	response.OK(c, result)
}

func (h *Handler) GetPromptTemplate(c *gin.Context) {
	tenantID := getTenantID(c)
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	result, err := h.svc.GetPromptTemplate(c.Request.Context(), tenantID, id)
	if err != nil {
		response.Fail(c, http.StatusNotFound, "prompt template not found")
		return
	}
	response.OK(c, result)
}

func (h *Handler) CreatePromptTemplate(c *gin.Context) {
	tenantID := getTenantID(c)
	userID := getUserID(c)
	var req CreatePromptTemplateReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, "invalid request: "+err.Error())
		return
	}
	result, err := h.svc.CreatePromptTemplate(c.Request.Context(), tenantID, userID, req)
	if err != nil {
		response.Fail(c, http.StatusInternalServerError, "failed to create prompt template: "+err.Error())
		return
	}
	response.OK(c, result)
}

func (h *Handler) UpdatePromptTemplate(c *gin.Context) {
	tenantID := getTenantID(c)
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	var req UpdatePromptTemplateReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, "invalid request: "+err.Error())
		return
	}
	result, err := h.svc.UpdatePromptTemplate(c.Request.Context(), tenantID, id, req)
	if err != nil {
		response.Fail(c, http.StatusInternalServerError, "failed to update prompt template: "+err.Error())
		return
	}
	response.OK(c, result)
}

func (h *Handler) DeletePromptTemplate(c *gin.Context) {
	tenantID := getTenantID(c)
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	if err := h.svc.DeletePromptTemplate(c.Request.Context(), tenantID, id); err != nil {
		response.Fail(c, http.StatusInternalServerError, "failed to delete prompt template: "+err.Error())
		return
	}
	response.OK(c, nil)
}

// ==================== Review Rules Handlers ====================

func (h *Handler) ListReviewRules(c *gin.Context) {
	tenantID := getTenantID(c)
	var filter ReviewRuleFilter
	if err := c.ShouldBindQuery(&filter); err != nil {
		response.Fail(c, http.StatusBadRequest, "invalid query parameters: "+err.Error())
		return
	}
	result, err := h.svc.ListReviewRules(c.Request.Context(), tenantID, filter)
	if err != nil {
		response.Fail(c, http.StatusInternalServerError, "failed to list review rules: "+err.Error())
		return
	}
	response.OK(c, result)
}

func (h *Handler) GetReviewRule(c *gin.Context) {
	tenantID := getTenantID(c)
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	result, err := h.svc.GetReviewRule(c.Request.Context(), tenantID, id)
	if err != nil {
		response.Fail(c, http.StatusNotFound, "review rule not found")
		return
	}
	response.OK(c, result)
}

func (h *Handler) CreateReviewRule(c *gin.Context) {
	tenantID := getTenantID(c)
	userID := getUserID(c)
	var req CreateReviewRuleReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, "invalid request: "+err.Error())
		return
	}
	result, err := h.svc.CreateReviewRule(c.Request.Context(), tenantID, userID, req)
	if err != nil {
		response.Fail(c, http.StatusInternalServerError, "failed to create review rule: "+err.Error())
		return
	}
	response.OK(c, result)
}

func (h *Handler) UpdateReviewRule(c *gin.Context) {
	tenantID := getTenantID(c)
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	var req UpdateReviewRuleReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, "invalid request: "+err.Error())
		return
	}
	result, err := h.svc.UpdateReviewRule(c.Request.Context(), tenantID, id, req)
	if err != nil {
		response.Fail(c, http.StatusInternalServerError, "failed to update review rule: "+err.Error())
		return
	}
	response.OK(c, result)
}

func (h *Handler) ToggleReviewRule(c *gin.Context) {
	tenantID := getTenantID(c)
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	result, err := h.svc.ToggleReviewRule(c.Request.Context(), tenantID, id)
	if err != nil {
		response.Fail(c, http.StatusInternalServerError, "failed to toggle review rule: "+err.Error())
		return
	}
	response.OK(c, result)
}

// ==================== Scaffold Templates Handlers ====================

func (h *Handler) ListScaffoldTemplates(c *gin.Context) {
	tenantID := getTenantID(c)
	var filter ScaffoldFilter
	if err := c.ShouldBindQuery(&filter); err != nil {
		response.Fail(c, http.StatusBadRequest, "invalid query parameters: "+err.Error())
		return
	}
	result, err := h.svc.ListScaffoldTemplates(c.Request.Context(), tenantID, filter)
	if err != nil {
		response.Fail(c, http.StatusInternalServerError, "failed to list scaffold templates: "+err.Error())
		return
	}
	response.OK(c, result)
}

func (h *Handler) GetScaffoldTemplate(c *gin.Context) {
	tenantID := getTenantID(c)
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	result, err := h.svc.GetScaffoldTemplate(c.Request.Context(), tenantID, id)
	if err != nil {
		response.Fail(c, http.StatusNotFound, "scaffold template not found")
		return
	}
	response.OK(c, result)
}

// ==================== Effective Specs Handler ====================

func (h *Handler) GetEffectiveSpecs(c *gin.Context) {
	tenantID := getTenantID(c)
	projectID, ok := parseID(c, "projectId")
	if !ok {
		return
	}
	result, err := h.svc.GetEffectiveSpecs(c.Request.Context(), tenantID, projectID)
	if err != nil {
		response.Fail(c, http.StatusInternalServerError, "failed to get effective specs: "+err.Error())
		return
	}
	response.OK(c, result)
}
```

- [ ] **Step 5: 注册 specs 模块路由**

在 `forge-core/internal/router/router.go` 中添加 specs 路由注册。找到已有的路由注册代码（如 auth、project、task 模块），在其后添加：

```go
// 在 router.go 的 Setup 函数中添加：
// Import specs module
specsRepo := specs.NewRepository(db)
specsSvc := specs.NewService(specsRepo, redisClient)
specsHandler := specs.NewHandler(specsSvc)
specsHandler.RegisterRoutes(api) // api 是 /api group
```

确保 import 路径正确：

```go
import (
    // ... existing imports
    "github.com/shulex/forge/forge-core/internal/module/specs"
)
```

- [ ] **Step 6: 验证**

```bash
cd forge-core && go build ./cmd/forge-core

# 启动服务后测试 API
# 列出编码规范（种子数据）
curl -s -H "Authorization: Bearer $TOKEN" http://localhost:8080/api/specs/standards | jq .
# 预期: 4 条编码规范

# 创建新规范
curl -s -X POST -H "Authorization: Bearer $TOKEN" -H "Content-Type: application/json" \
  -d '{"name":"Redis 使用规范","category":"REDIS","scope":"COMPANY","scopeId":0,"content":"## Redis 规范\n- Key 使用冒号分隔\n- 设置合理 TTL"}' \
  http://localhost:8080/api/specs/standards | jq .
# 预期: 返回新创建的规范

# 列出 Prompt 模板
curl -s -H "Authorization: Bearer $TOKEN" http://localhost:8080/api/specs/prompts | jq .
# 预期: 6 条默认模板

# 列出 Review 规则
curl -s -H "Authorization: Bearer $TOKEN" http://localhost:8080/api/specs/rules | jq .
# 预期: 5 条默认规则

# 列出脚手架模板
curl -s -H "Authorization: Bearer $TOKEN" http://localhost:8080/api/specs/scaffolds | jq .
# 预期: 4 条脚手架模板

# 获取项目有效规范
curl -s -H "Authorization: Bearer $TOKEN" http://localhost:8080/api/specs/effective/1 | jq .
# 预期: 合并后的 standards + rules
```

- [ ] **Step 7: Commit**

```bash
git add forge-core/internal/module/specs/
git commit -m "feat(s5): add specs center backend — standards, prompts, rules, scaffolds with three-level inheritance"
```

---

## Task 3: Specs API 客户端（前端）

**Files:**
- Create: `forge-portal/lib/specs.ts`

- [ ] **Step 1: 创建 specs.ts — API 客户端函数**

`forge-portal/lib/specs.ts`：

```typescript
import { api } from "./api";

// ==================== Types ====================

export interface Standard {
  id: number;
  tenantId: number;
  name: string;
  category: string;
  scope: string;
  scopeId: number;
  parentId?: number;
  content: string;
  version: number;
  status: string;
  createdBy?: number;
  createdAt: string;
  updatedAt: string;
}

export interface PromptTemplate {
  id: number;
  tenantId: number;
  name: string;
  purpose: string;
  systemPrompt: string;
  userTemplate: string;
  variables: string[];
  version: number;
  isDefault: boolean;
  createdBy?: number;
  createdAt: string;
  updatedAt: string;
}

export interface ReviewRule {
  id: number;
  tenantId: number;
  name: string;
  category: string;
  scope: string;
  scopeId: number;
  ruleType: string;
  definition: Record<string, unknown>;
  severity: string;
  autoFix: boolean;
  fixTemplate?: string;
  enabled: boolean;
  createdBy?: number;
  createdAt: string;
  updatedAt: string;
}

export interface ScaffoldTemplate {
  id: number;
  tenantId: number;
  name: string;
  projectType: string;
  description?: string;
  templateRepo?: string;
  variables: string[];
  postHooks: string[];
  version: number;
  createdBy?: number;
  createdAt: string;
  updatedAt: string;
}

export interface PageResult<T> {
  items: T[];
  total: number;
  page: number;
  pageSize: number;
}

export interface EffectiveSpecs {
  standards: Standard[];
  rules: ReviewRule[];
}

// ==================== Standards API ====================

export async function listStandards(params?: {
  category?: string;
  scope?: string;
  scopeId?: number;
  page?: number;
  pageSize?: number;
}): Promise<PageResult<Standard>> {
  const searchParams = new URLSearchParams();
  if (params?.category) searchParams.set("category", params.category);
  if (params?.scope) searchParams.set("scope", params.scope);
  if (params?.scopeId !== undefined) searchParams.set("scopeId", String(params.scopeId));
  if (params?.page) searchParams.set("page", String(params.page));
  if (params?.pageSize) searchParams.set("pageSize", String(params.pageSize));
  const query = searchParams.toString();
  return api.get(`/api/specs/standards${query ? `?${query}` : ""}`);
}

export async function getStandard(id: number): Promise<Standard> {
  return api.get(`/api/specs/standards/${id}`);
}

export async function createStandard(data: {
  name: string;
  category: string;
  scope: string;
  scopeId: number;
  parentId?: number;
  content: string;
}): Promise<Standard> {
  return api.post("/api/specs/standards", data);
}

export async function updateStandard(
  id: number,
  data: { name: string; content: string }
): Promise<Standard> {
  return api.put(`/api/specs/standards/${id}`, data);
}

export async function deleteStandard(id: number): Promise<void> {
  return api.delete(`/api/specs/standards/${id}`);
}

// ==================== Prompt Templates API ====================

export async function listPromptTemplates(params?: {
  purpose?: string;
  page?: number;
  pageSize?: number;
}): Promise<PageResult<PromptTemplate>> {
  const searchParams = new URLSearchParams();
  if (params?.purpose) searchParams.set("purpose", params.purpose);
  if (params?.page) searchParams.set("page", String(params.page));
  if (params?.pageSize) searchParams.set("pageSize", String(params.pageSize));
  const query = searchParams.toString();
  return api.get(`/api/specs/prompts${query ? `?${query}` : ""}`);
}

export async function getPromptTemplate(id: number): Promise<PromptTemplate> {
  return api.get(`/api/specs/prompts/${id}`);
}

export async function createPromptTemplate(data: {
  name: string;
  purpose: string;
  systemPrompt: string;
  userTemplate: string;
  variables: string[];
  isDefault: boolean;
}): Promise<PromptTemplate> {
  return api.post("/api/specs/prompts", data);
}

export async function updatePromptTemplate(
  id: number,
  data: {
    name: string;
    purpose: string;
    systemPrompt: string;
    userTemplate: string;
    variables: string[];
    isDefault: boolean;
  }
): Promise<PromptTemplate> {
  return api.put(`/api/specs/prompts/${id}`, data);
}

export async function deletePromptTemplate(id: number): Promise<void> {
  return api.delete(`/api/specs/prompts/${id}`);
}

// ==================== Review Rules API ====================

export async function listReviewRules(params?: {
  category?: string;
  severity?: string;
  page?: number;
  pageSize?: number;
}): Promise<PageResult<ReviewRule>> {
  const searchParams = new URLSearchParams();
  if (params?.category) searchParams.set("category", params.category);
  if (params?.severity) searchParams.set("severity", params.severity);
  if (params?.page) searchParams.set("page", String(params.page));
  if (params?.pageSize) searchParams.set("pageSize", String(params.pageSize));
  const query = searchParams.toString();
  return api.get(`/api/specs/rules${query ? `?${query}` : ""}`);
}

export async function getReviewRule(id: number): Promise<ReviewRule> {
  return api.get(`/api/specs/rules/${id}`);
}

export async function createReviewRule(data: {
  name: string;
  category: string;
  scope: string;
  scopeId: number;
  ruleType: string;
  definition: Record<string, unknown>;
  severity: string;
  autoFix: boolean;
  fixTemplate?: string;
}): Promise<ReviewRule> {
  return api.post("/api/specs/rules", data);
}

export async function updateReviewRule(
  id: number,
  data: {
    name: string;
    category: string;
    ruleType: string;
    definition: Record<string, unknown>;
    severity: string;
    autoFix: boolean;
    fixTemplate?: string;
  }
): Promise<ReviewRule> {
  return api.put(`/api/specs/rules/${id}`, data);
}

export async function toggleReviewRule(id: number): Promise<ReviewRule> {
  return api.delete(`/api/specs/rules/${id}`);
}

// ==================== Scaffold Templates API ====================

export async function listScaffoldTemplates(params?: {
  projectType?: string;
  page?: number;
  pageSize?: number;
}): Promise<PageResult<ScaffoldTemplate>> {
  const searchParams = new URLSearchParams();
  if (params?.projectType) searchParams.set("projectType", params.projectType);
  if (params?.page) searchParams.set("page", String(params.page));
  if (params?.pageSize) searchParams.set("pageSize", String(params.pageSize));
  const query = searchParams.toString();
  return api.get(`/api/specs/scaffolds${query ? `?${query}` : ""}`);
}

export async function getScaffoldTemplate(id: number): Promise<ScaffoldTemplate> {
  return api.get(`/api/specs/scaffolds/${id}`);
}

// ==================== Effective Specs API ====================

export async function getEffectiveSpecs(projectId: number): Promise<EffectiveSpecs> {
  return api.get(`/api/specs/effective/${projectId}`);
}
```

- [ ] **Step 2: Commit**

```bash
git add forge-portal/lib/specs.ts
git commit -m "feat(s5): add specs center API client for frontend"
```

---

## Task 4: 规范中心前端页面 — Standards

**Files:**
- Create: `forge-portal/app/(dashboard)/specs/layout.tsx`
- Create: `forge-portal/app/(dashboard)/specs/page.tsx`
- Create: `forge-portal/app/(dashboard)/specs/standards/page.tsx`

- [ ] **Step 1: 创建 specs 布局（tabs 导航）**

`forge-portal/app/(dashboard)/specs/layout.tsx`：

```tsx
"use client";

import { usePathname, useRouter } from "next/navigation";
import { BookOpen, MessageSquareCode, ShieldCheck, Boxes } from "lucide-react";
import { cn } from "@/lib/utils";

const tabs = [
  { label: "编码规范", href: "/specs/standards", icon: BookOpen },
  { label: "Prompt 模板", href: "/specs/prompts", icon: MessageSquareCode },
  { label: "Review 规则", href: "/specs/rules", icon: ShieldCheck },
  { label: "脚手架模板", href: "/specs/scaffolds", icon: Boxes },
];

export default function SpecsLayout({ children }: { children: React.ReactNode }) {
  const pathname = usePathname();
  const router = useRouter();

  return (
    <div className="flex flex-col h-full">
      {/* Header */}
      <div className="border-b border-white/10 px-6 pt-6 pb-0">
        <h1 className="text-2xl font-bold text-white mb-1">规范中心</h1>
        <p className="text-sm text-white/50 mb-4">
          管理编码规范、Prompt 模板、Review 规则和脚手架模板
        </p>

        {/* Tab navigation */}
        <div className="flex gap-1">
          {tabs.map((tab) => {
            const isActive = pathname.startsWith(tab.href);
            return (
              <button
                key={tab.href}
                onClick={() => router.push(tab.href)}
                className={cn(
                  "flex items-center gap-2 px-4 py-2.5 text-sm font-medium rounded-t-lg transition-colors",
                  isActive
                    ? "bg-white/10 text-white border-b-2 border-[#8B5CF6]"
                    : "text-white/50 hover:text-white/70 hover:bg-white/5"
                )}
              >
                <tab.icon className="h-4 w-4" />
                {tab.label}
              </button>
            );
          })}
        </div>
      </div>

      {/* Content */}
      <div className="flex-1 overflow-auto p-6">{children}</div>
    </div>
  );
}
```

- [ ] **Step 2: 创建 specs 主页（重定向到 standards）**

`forge-portal/app/(dashboard)/specs/page.tsx`：

```tsx
import { redirect } from "next/navigation";

export default function SpecsPage() {
  redirect("/specs/standards");
}
```

- [ ] **Step 3: 创建 Standards 页面**

`forge-portal/app/(dashboard)/specs/standards/page.tsx`：

```tsx
"use client";

import { useState, useEffect, useCallback } from "react";
import {
  Plus,
  Search,
  Edit2,
  Trash2,
  ChevronLeft,
  ChevronRight,
} from "lucide-react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogFooter,
} from "@/components/ui/dialog";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { Textarea } from "@/components/ui/textarea";
import { Label } from "@/components/ui/label";
import {
  Standard,
  listStandards,
  createStandard,
  updateStandard,
  deleteStandard,
} from "@/lib/specs";

const CATEGORIES = [
  { value: "", label: "全部分类" },
  { value: "JAVA", label: "Java" },
  { value: "SQL", label: "SQL" },
  { value: "REDIS", label: "Redis" },
  { value: "KAFKA", label: "Kafka" },
  { value: "API", label: "API" },
  { value: "SECURITY", label: "安全" },
  { value: "NAMING", label: "命名" },
  { value: "GIT", label: "Git" },
];

const SCOPES = [
  { value: "COMPANY", label: "公司级" },
  { value: "TEAM", label: "团队级" },
  { value: "PROJECT", label: "项目级" },
];

const CATEGORY_COLORS: Record<string, string> = {
  JAVA: "bg-orange-500/10 text-orange-400",
  SQL: "bg-blue-500/10 text-blue-400",
  REDIS: "bg-red-500/10 text-red-400",
  KAFKA: "bg-green-500/10 text-green-400",
  API: "bg-purple-500/10 text-purple-400",
  SECURITY: "bg-yellow-500/10 text-yellow-400",
  NAMING: "bg-cyan-500/10 text-cyan-400",
  GIT: "bg-pink-500/10 text-pink-400",
};

const SCOPE_COLORS: Record<string, string> = {
  COMPANY: "bg-[#8B5CF6]/10 text-[#8B5CF6]",
  TEAM: "bg-blue-500/10 text-blue-400",
  PROJECT: "bg-green-500/10 text-green-400",
};

export default function StandardsPage() {
  const [standards, setStandards] = useState<Standard[]>([]);
  const [total, setTotal] = useState(0);
  const [page, setPage] = useState(1);
  const [category, setCategory] = useState("");
  const [loading, setLoading] = useState(true);

  // Dialog state
  const [dialogOpen, setDialogOpen] = useState(false);
  const [editingStandard, setEditingStandard] = useState<Standard | null>(null);
  const [form, setForm] = useState({
    name: "",
    category: "JAVA",
    scope: "COMPANY",
    scopeId: 0,
    content: "",
  });

  const pageSize = 20;
  const totalPages = Math.ceil(total / pageSize);

  const fetchStandards = useCallback(async () => {
    setLoading(true);
    try {
      const result = await listStandards({
        category: category || undefined,
        page,
        pageSize,
      });
      setStandards(result.items || []);
      setTotal(result.total);
    } catch (err) {
      console.error("Failed to fetch standards:", err);
    } finally {
      setLoading(false);
    }
  }, [category, page]);

  useEffect(() => {
    fetchStandards();
  }, [fetchStandards]);

  const openCreate = () => {
    setEditingStandard(null);
    setForm({ name: "", category: "JAVA", scope: "COMPANY", scopeId: 0, content: "" });
    setDialogOpen(true);
  };

  const openEdit = (std: Standard) => {
    setEditingStandard(std);
    setForm({
      name: std.name,
      category: std.category,
      scope: std.scope,
      scopeId: std.scopeId,
      content: std.content,
    });
    setDialogOpen(true);
  };

  const handleSave = async () => {
    try {
      if (editingStandard) {
        await updateStandard(editingStandard.id, {
          name: form.name,
          content: form.content,
        });
      } else {
        await createStandard(form);
      }
      setDialogOpen(false);
      fetchStandards();
    } catch (err) {
      console.error("Failed to save standard:", err);
    }
  };

  const handleDelete = async (id: number) => {
    if (!confirm("确定要删除此编码规范吗？")) return;
    try {
      await deleteStandard(id);
      fetchStandards();
    } catch (err) {
      console.error("Failed to delete standard:", err);
    }
  };

  return (
    <div className="space-y-4">
      {/* Toolbar */}
      <div className="flex items-center justify-between gap-4">
        <div className="flex items-center gap-3">
          <Select value={category} onValueChange={(v) => { setCategory(v); setPage(1); }}>
            <SelectTrigger className="w-[160px] bg-white/5 border-white/10 text-white">
              <SelectValue placeholder="全部分类" />
            </SelectTrigger>
            <SelectContent>
              {CATEGORIES.map((c) => (
                <SelectItem key={c.value} value={c.value || "all"}>
                  {c.label}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        </div>
        <Button onClick={openCreate} className="bg-[#8B5CF6] hover:bg-[#7C3AED] text-white">
          <Plus className="h-4 w-4 mr-2" />
          新建规范
        </Button>
      </div>

      {/* Table */}
      <div className="bg-white/[0.03] border border-white/10 rounded-lg overflow-hidden">
        <table className="w-full">
          <thead>
            <tr className="border-b border-white/10 text-white/50 text-sm">
              <th className="text-left px-4 py-3 font-medium">名称</th>
              <th className="text-left px-4 py-3 font-medium">分类</th>
              <th className="text-left px-4 py-3 font-medium">作用域</th>
              <th className="text-left px-4 py-3 font-medium">版本</th>
              <th className="text-left px-4 py-3 font-medium">更新时间</th>
              <th className="text-right px-4 py-3 font-medium">操作</th>
            </tr>
          </thead>
          <tbody>
            {loading ? (
              <tr>
                <td colSpan={6} className="text-center py-12 text-white/30">
                  加载中...
                </td>
              </tr>
            ) : standards.length === 0 ? (
              <tr>
                <td colSpan={6} className="text-center py-12 text-white/30">
                  暂无编码规范，点击"新建规范"添加
                </td>
              </tr>
            ) : (
              standards.map((std) => (
                <tr
                  key={std.id}
                  className="border-b border-white/5 hover:bg-white/[0.02] transition-colors"
                >
                  <td className="px-4 py-3 text-white font-medium">{std.name}</td>
                  <td className="px-4 py-3">
                    <span
                      className={`inline-flex px-2 py-0.5 rounded text-xs font-medium ${
                        CATEGORY_COLORS[std.category] || "bg-white/10 text-white/70"
                      }`}
                    >
                      {std.category}
                    </span>
                  </td>
                  <td className="px-4 py-3">
                    <span
                      className={`inline-flex px-2 py-0.5 rounded text-xs font-medium ${
                        SCOPE_COLORS[std.scope] || "bg-white/10 text-white/70"
                      }`}
                    >
                      {SCOPES.find((s) => s.value === std.scope)?.label || std.scope}
                    </span>
                  </td>
                  <td className="px-4 py-3 text-white/50 text-sm">v{std.version}</td>
                  <td className="px-4 py-3 text-white/50 text-sm">
                    {new Date(std.updatedAt).toLocaleDateString("zh-CN")}
                  </td>
                  <td className="px-4 py-3 text-right">
                    <div className="flex items-center justify-end gap-1">
                      <Button
                        variant="ghost"
                        size="icon"
                        className="h-8 w-8 text-white/50 hover:text-white"
                        onClick={() => openEdit(std)}
                      >
                        <Edit2 className="h-4 w-4" />
                      </Button>
                      <Button
                        variant="ghost"
                        size="icon"
                        className="h-8 w-8 text-white/50 hover:text-red-400"
                        onClick={() => handleDelete(std.id)}
                      >
                        <Trash2 className="h-4 w-4" />
                      </Button>
                    </div>
                  </td>
                </tr>
              ))
            )}
          </tbody>
        </table>
      </div>

      {/* Pagination */}
      {totalPages > 1 && (
        <div className="flex items-center justify-between text-sm text-white/50">
          <span>
            共 {total} 条，第 {page}/{totalPages} 页
          </span>
          <div className="flex items-center gap-2">
            <Button
              variant="ghost"
              size="icon"
              className="h-8 w-8"
              disabled={page <= 1}
              onClick={() => setPage((p) => p - 1)}
            >
              <ChevronLeft className="h-4 w-4" />
            </Button>
            <Button
              variant="ghost"
              size="icon"
              className="h-8 w-8"
              disabled={page >= totalPages}
              onClick={() => setPage((p) => p + 1)}
            >
              <ChevronRight className="h-4 w-4" />
            </Button>
          </div>
        </div>
      )}

      {/* Create/Edit Dialog */}
      <Dialog open={dialogOpen} onOpenChange={setDialogOpen}>
        <DialogContent className="bg-[#0A0A12] border-white/10 text-white max-w-2xl">
          <DialogHeader>
            <DialogTitle>
              {editingStandard ? "编辑编码规范" : "新建编码规范"}
            </DialogTitle>
          </DialogHeader>
          <div className="space-y-4 py-4">
            <div className="space-y-2">
              <Label>名称</Label>
              <Input
                value={form.name}
                onChange={(e) => setForm({ ...form, name: e.target.value })}
                placeholder="如：Java 编码规范"
                className="bg-white/5 border-white/10"
              />
            </div>
            {!editingStandard && (
              <div className="grid grid-cols-2 gap-4">
                <div className="space-y-2">
                  <Label>分类</Label>
                  <Select
                    value={form.category}
                    onValueChange={(v) => setForm({ ...form, category: v })}
                  >
                    <SelectTrigger className="bg-white/5 border-white/10">
                      <SelectValue />
                    </SelectTrigger>
                    <SelectContent>
                      {CATEGORIES.filter((c) => c.value).map((c) => (
                        <SelectItem key={c.value} value={c.value}>
                          {c.label}
                        </SelectItem>
                      ))}
                    </SelectContent>
                  </Select>
                </div>
                <div className="space-y-2">
                  <Label>作用域</Label>
                  <Select
                    value={form.scope}
                    onValueChange={(v) => setForm({ ...form, scope: v })}
                  >
                    <SelectTrigger className="bg-white/5 border-white/10">
                      <SelectValue />
                    </SelectTrigger>
                    <SelectContent>
                      {SCOPES.map((s) => (
                        <SelectItem key={s.value} value={s.value}>
                          {s.label}
                        </SelectItem>
                      ))}
                    </SelectContent>
                  </Select>
                </div>
              </div>
            )}
            <div className="space-y-2">
              <Label>规范内容（Markdown）</Label>
              <Textarea
                value={form.content}
                onChange={(e) => setForm({ ...form, content: e.target.value })}
                placeholder="输入编码规范内容，支持 Markdown 格式..."
                className="bg-[#0A0A12] border-white/10 font-mono text-sm min-h-[300px]"
              />
            </div>
          </div>
          <DialogFooter>
            <Button
              variant="ghost"
              onClick={() => setDialogOpen(false)}
              className="text-white/50"
            >
              取消
            </Button>
            <Button
              onClick={handleSave}
              className="bg-[#8B5CF6] hover:bg-[#7C3AED] text-white"
              disabled={!form.name || !form.content}
            >
              {editingStandard ? "保存修改" : "创建"}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  );
}
```

- [ ] **Step 4: 在侧边栏添加"规范中心"入口**

在 Dashboard layout 的侧边栏导航中添加规范中心链接。找到侧边栏导航配置（通常在 `forge-portal/app/(dashboard)/layout.tsx`），添加：

```tsx
// 在 sidebar navigation items 中添加:
{ label: "规范中心", href: "/specs", icon: BookOpen },
```

确保 import `BookOpen` from `lucide-react`。

- [ ] **Step 5: 验证**

```bash
cd forge-portal && npm run build
# 预期: 编译成功，无类型错误

# 本地开发模式
cd forge-portal && npm run dev
# 浏览器打开 http://localhost:3000/specs/standards
# 预期: 看到编码规范列表，种子数据显示正确
# 点击"新建规范"弹出对话框，可创建/编辑/删除
```

- [ ] **Step 6: Commit**

```bash
git add forge-portal/app/\(dashboard\)/specs/ forge-portal/lib/specs.ts
git commit -m "feat(s5): add specs center standards page with CRUD and tab navigation"
```

---

## Task 5: 规范中心前端页面 — Prompts, Rules, Scaffolds

**Files:**
- Create: `forge-portal/app/(dashboard)/specs/prompts/page.tsx`
- Create: `forge-portal/app/(dashboard)/specs/rules/page.tsx`
- Create: `forge-portal/app/(dashboard)/specs/scaffolds/page.tsx`

- [ ] **Step 1: 创建 Prompts 页面**

`forge-portal/app/(dashboard)/specs/prompts/page.tsx`：

```tsx
"use client";

import { useState, useEffect, useCallback } from "react";
import {
  Plus,
  Edit2,
  Trash2,
  ChevronLeft,
  ChevronRight,
  Star,
  Copy,
} from "lucide-react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogFooter,
} from "@/components/ui/dialog";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { Textarea } from "@/components/ui/textarea";
import { Label } from "@/components/ui/label";
import { Switch } from "@/components/ui/switch";
import {
  PromptTemplate,
  listPromptTemplates,
  createPromptTemplate,
  updatePromptTemplate,
  deletePromptTemplate,
} from "@/lib/specs";

const PURPOSES = [
  { value: "", label: "全部用途" },
  { value: "requirement-analysis", label: "需求分析" },
  { value: "code-generation", label: "代码生成" },
  { value: "code-review", label: "代码 Review" },
  { value: "test-generation", label: "测试生成" },
  { value: "fix-generation", label: "修复生成" },
  { value: "doc-generation", label: "文档生成" },
];

const PURPOSE_COLORS: Record<string, string> = {
  "requirement-analysis": "bg-blue-500/10 text-blue-400",
  "code-generation": "bg-green-500/10 text-green-400",
  "code-review": "bg-orange-500/10 text-orange-400",
  "test-generation": "bg-purple-500/10 text-purple-400",
  "fix-generation": "bg-red-500/10 text-red-400",
  "doc-generation": "bg-cyan-500/10 text-cyan-400",
};

const emptyForm = {
  name: "",
  purpose: "code-generation" as string,
  systemPrompt: "",
  userTemplate: "",
  variables: [] as string[],
  isDefault: false,
};

export default function PromptsPage() {
  const [templates, setTemplates] = useState<PromptTemplate[]>([]);
  const [total, setTotal] = useState(0);
  const [page, setPage] = useState(1);
  const [purpose, setPurpose] = useState("");
  const [loading, setLoading] = useState(true);

  const [dialogOpen, setDialogOpen] = useState(false);
  const [editingTemplate, setEditingTemplate] = useState<PromptTemplate | null>(null);
  const [form, setForm] = useState(emptyForm);
  const [variableInput, setVariableInput] = useState("");

  const pageSize = 20;
  const totalPages = Math.ceil(total / pageSize);

  const fetchTemplates = useCallback(async () => {
    setLoading(true);
    try {
      const result = await listPromptTemplates({
        purpose: purpose || undefined,
        page,
        pageSize,
      });
      setTemplates(result.items || []);
      setTotal(result.total);
    } catch (err) {
      console.error("Failed to fetch prompt templates:", err);
    } finally {
      setLoading(false);
    }
  }, [purpose, page]);

  useEffect(() => {
    fetchTemplates();
  }, [fetchTemplates]);

  const openCreate = () => {
    setEditingTemplate(null);
    setForm(emptyForm);
    setDialogOpen(true);
  };

  const openEdit = (tpl: PromptTemplate) => {
    setEditingTemplate(tpl);
    setForm({
      name: tpl.name,
      purpose: tpl.purpose,
      systemPrompt: tpl.systemPrompt,
      userTemplate: tpl.userTemplate,
      variables: tpl.variables || [],
      isDefault: tpl.isDefault,
    });
    setDialogOpen(true);
  };

  const handleSave = async () => {
    try {
      if (editingTemplate) {
        await updatePromptTemplate(editingTemplate.id, form);
      } else {
        await createPromptTemplate(form);
      }
      setDialogOpen(false);
      fetchTemplates();
    } catch (err) {
      console.error("Failed to save prompt template:", err);
    }
  };

  const handleDelete = async (id: number) => {
    if (!confirm("确定要删除此 Prompt 模板吗？")) return;
    try {
      await deletePromptTemplate(id);
      fetchTemplates();
    } catch (err) {
      console.error("Failed to delete prompt template:", err);
    }
  };

  const addVariable = () => {
    const v = variableInput.trim();
    if (v && !form.variables.includes(v)) {
      setForm({ ...form, variables: [...form.variables, v] });
      setVariableInput("");
    }
  };

  const removeVariable = (v: string) => {
    setForm({ ...form, variables: form.variables.filter((x) => x !== v) });
  };

  return (
    <div className="space-y-4">
      {/* Toolbar */}
      <div className="flex items-center justify-between gap-4">
        <Select value={purpose} onValueChange={(v) => { setPurpose(v === "all" ? "" : v); setPage(1); }}>
          <SelectTrigger className="w-[160px] bg-white/5 border-white/10 text-white">
            <SelectValue placeholder="全部用途" />
          </SelectTrigger>
          <SelectContent>
            {PURPOSES.map((p) => (
              <SelectItem key={p.value || "all"} value={p.value || "all"}>
                {p.label}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
        <Button onClick={openCreate} className="bg-[#8B5CF6] hover:bg-[#7C3AED] text-white">
          <Plus className="h-4 w-4 mr-2" />
          新建模板
        </Button>
      </div>

      {/* Table */}
      <div className="bg-white/[0.03] border border-white/10 rounded-lg overflow-hidden">
        <table className="w-full">
          <thead>
            <tr className="border-b border-white/10 text-white/50 text-sm">
              <th className="text-left px-4 py-3 font-medium">名称</th>
              <th className="text-left px-4 py-3 font-medium">用途</th>
              <th className="text-left px-4 py-3 font-medium">变量数</th>
              <th className="text-left px-4 py-3 font-medium">版本</th>
              <th className="text-left px-4 py-3 font-medium">默认</th>
              <th className="text-left px-4 py-3 font-medium">更新时间</th>
              <th className="text-right px-4 py-3 font-medium">操作</th>
            </tr>
          </thead>
          <tbody>
            {loading ? (
              <tr>
                <td colSpan={7} className="text-center py-12 text-white/30">
                  加载中...
                </td>
              </tr>
            ) : templates.length === 0 ? (
              <tr>
                <td colSpan={7} className="text-center py-12 text-white/30">
                  暂无 Prompt 模板
                </td>
              </tr>
            ) : (
              templates.map((tpl) => (
                <tr
                  key={tpl.id}
                  className="border-b border-white/5 hover:bg-white/[0.02] transition-colors"
                >
                  <td className="px-4 py-3 text-white font-medium">{tpl.name}</td>
                  <td className="px-4 py-3">
                    <span
                      className={`inline-flex px-2 py-0.5 rounded text-xs font-medium ${
                        PURPOSE_COLORS[tpl.purpose] || "bg-white/10 text-white/70"
                      }`}
                    >
                      {PURPOSES.find((p) => p.value === tpl.purpose)?.label || tpl.purpose}
                    </span>
                  </td>
                  <td className="px-4 py-3 text-white/50 text-sm">
                    {tpl.variables?.length || 0}
                  </td>
                  <td className="px-4 py-3 text-white/50 text-sm">v{tpl.version}</td>
                  <td className="px-4 py-3">
                    {tpl.isDefault && (
                      <Star className="h-4 w-4 text-yellow-400 fill-yellow-400" />
                    )}
                  </td>
                  <td className="px-4 py-3 text-white/50 text-sm">
                    {new Date(tpl.updatedAt).toLocaleDateString("zh-CN")}
                  </td>
                  <td className="px-4 py-3 text-right">
                    <div className="flex items-center justify-end gap-1">
                      <Button
                        variant="ghost"
                        size="icon"
                        className="h-8 w-8 text-white/50 hover:text-white"
                        onClick={() => openEdit(tpl)}
                      >
                        <Edit2 className="h-4 w-4" />
                      </Button>
                      <Button
                        variant="ghost"
                        size="icon"
                        className="h-8 w-8 text-white/50 hover:text-red-400"
                        onClick={() => handleDelete(tpl.id)}
                      >
                        <Trash2 className="h-4 w-4" />
                      </Button>
                    </div>
                  </td>
                </tr>
              ))
            )}
          </tbody>
        </table>
      </div>

      {/* Pagination */}
      {totalPages > 1 && (
        <div className="flex items-center justify-between text-sm text-white/50">
          <span>共 {total} 条，第 {page}/{totalPages} 页</span>
          <div className="flex items-center gap-2">
            <Button variant="ghost" size="icon" className="h-8 w-8" disabled={page <= 1} onClick={() => setPage((p) => p - 1)}>
              <ChevronLeft className="h-4 w-4" />
            </Button>
            <Button variant="ghost" size="icon" className="h-8 w-8" disabled={page >= totalPages} onClick={() => setPage((p) => p + 1)}>
              <ChevronRight className="h-4 w-4" />
            </Button>
          </div>
        </div>
      )}

      {/* Create/Edit Dialog */}
      <Dialog open={dialogOpen} onOpenChange={setDialogOpen}>
        <DialogContent className="bg-[#0A0A12] border-white/10 text-white max-w-3xl max-h-[90vh] overflow-y-auto">
          <DialogHeader>
            <DialogTitle>
              {editingTemplate ? "编辑 Prompt 模板" : "新建 Prompt 模板"}
            </DialogTitle>
          </DialogHeader>
          <div className="space-y-4 py-4">
            <div className="grid grid-cols-2 gap-4">
              <div className="space-y-2">
                <Label>名称</Label>
                <Input
                  value={form.name}
                  onChange={(e) => setForm({ ...form, name: e.target.value })}
                  placeholder="如：代码生成模板"
                  className="bg-white/5 border-white/10"
                />
              </div>
              <div className="space-y-2">
                <Label>用途</Label>
                <Select value={form.purpose} onValueChange={(v) => setForm({ ...form, purpose: v })}>
                  <SelectTrigger className="bg-white/5 border-white/10">
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    {PURPOSES.filter((p) => p.value).map((p) => (
                      <SelectItem key={p.value} value={p.value}>
                        {p.label}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </div>
            </div>

            <div className="space-y-2">
              <Label>System Prompt</Label>
              <Textarea
                value={form.systemPrompt}
                onChange={(e) => setForm({ ...form, systemPrompt: e.target.value })}
                placeholder="系统提示词，定义 AI 的角色和约束..."
                className="bg-[#0A0A12] border-white/10 font-mono text-sm min-h-[150px]"
              />
            </div>

            <div className="space-y-2">
              <Label>User Template</Label>
              <Textarea
                value={form.userTemplate}
                onChange={(e) => setForm({ ...form, userTemplate: e.target.value })}
                placeholder="用户模板，使用 {{variable}} 插入变量..."
                className="bg-[#0A0A12] border-white/10 font-mono text-sm min-h-[150px]"
              />
            </div>

            <div className="space-y-2">
              <Label>模板变量</Label>
              <div className="flex gap-2">
                <Input
                  value={variableInput}
                  onChange={(e) => setVariableInput(e.target.value)}
                  placeholder="输入变量名，按回车添加"
                  className="bg-white/5 border-white/10"
                  onKeyDown={(e) => {
                    if (e.key === "Enter") {
                      e.preventDefault();
                      addVariable();
                    }
                  }}
                />
                <Button variant="outline" onClick={addVariable} className="border-white/10">
                  添加
                </Button>
              </div>
              {form.variables.length > 0 && (
                <div className="flex flex-wrap gap-2 mt-2">
                  {form.variables.map((v) => (
                    <span
                      key={v}
                      className="inline-flex items-center gap-1 px-2 py-1 rounded bg-[#8B5CF6]/10 text-[#8B5CF6] text-xs font-mono"
                    >
                      {"{{" + v + "}}"}
                      <button
                        onClick={() => removeVariable(v)}
                        className="hover:text-red-400 ml-1"
                      >
                        x
                      </button>
                    </span>
                  ))}
                </div>
              )}
            </div>

            <div className="flex items-center gap-3">
              <Switch
                checked={form.isDefault}
                onCheckedChange={(v) => setForm({ ...form, isDefault: v })}
              />
              <Label>设为默认模板</Label>
            </div>
          </div>
          <DialogFooter>
            <Button variant="ghost" onClick={() => setDialogOpen(false)} className="text-white/50">
              取消
            </Button>
            <Button
              onClick={handleSave}
              className="bg-[#8B5CF6] hover:bg-[#7C3AED] text-white"
              disabled={!form.name || !form.systemPrompt || !form.userTemplate}
            >
              {editingTemplate ? "保存修改" : "创建"}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  );
}
```

- [ ] **Step 2: 创建 Rules 页面**

`forge-portal/app/(dashboard)/specs/rules/page.tsx`：

```tsx
"use client";

import { useState, useEffect, useCallback } from "react";
import {
  Plus,
  Edit2,
  ChevronLeft,
  ChevronRight,
  Zap,
  AlertTriangle,
  Info,
  AlertCircle,
} from "lucide-react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogFooter,
} from "@/components/ui/dialog";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { Textarea } from "@/components/ui/textarea";
import { Label } from "@/components/ui/label";
import { Switch } from "@/components/ui/switch";
import {
  ReviewRule,
  listReviewRules,
  createReviewRule,
  updateReviewRule,
  toggleReviewRule,
} from "@/lib/specs";

const RULE_CATEGORIES = [
  { value: "", label: "全部分类" },
  { value: "CODING", label: "编码" },
  { value: "SECURITY", label: "安全" },
  { value: "PERFORMANCE", label: "性能" },
  { value: "DATABASE", label: "数据库" },
  { value: "API_COMPAT", label: "API 兼容性" },
  { value: "CUSTOM", label: "自定义" },
];

const SEVERITIES = [
  { value: "", label: "全部级别" },
  { value: "ERROR", label: "Error" },
  { value: "WARNING", label: "Warning" },
  { value: "INFO", label: "Info" },
];

const RULE_TYPES = [
  { value: "PATTERN", label: "正则匹配" },
  { value: "AST", label: "AST 分析" },
  { value: "AI_CHECK", label: "AI 检查" },
];

const SEVERITY_CONFIG: Record<string, { icon: typeof AlertCircle; color: string }> = {
  ERROR: { icon: AlertCircle, color: "text-red-400 bg-red-500/10" },
  WARNING: { icon: AlertTriangle, color: "text-yellow-400 bg-yellow-500/10" },
  INFO: { icon: Info, color: "text-blue-400 bg-blue-500/10" },
};

const CATEGORY_COLORS: Record<string, string> = {
  CODING: "bg-purple-500/10 text-purple-400",
  SECURITY: "bg-red-500/10 text-red-400",
  PERFORMANCE: "bg-yellow-500/10 text-yellow-400",
  DATABASE: "bg-blue-500/10 text-blue-400",
  API_COMPAT: "bg-green-500/10 text-green-400",
  CUSTOM: "bg-white/10 text-white/70",
};

const emptyForm = {
  name: "",
  category: "CODING",
  scope: "COMPANY",
  scopeId: 0,
  ruleType: "PATTERN",
  definitionStr: '{"pattern": "", "language": "", "description": ""}',
  severity: "WARNING",
  autoFix: false,
  fixTemplate: "",
};

export default function RulesPage() {
  const [rules, setRules] = useState<ReviewRule[]>([]);
  const [total, setTotal] = useState(0);
  const [page, setPage] = useState(1);
  const [category, setCategory] = useState("");
  const [severity, setSeverity] = useState("");
  const [loading, setLoading] = useState(true);

  const [dialogOpen, setDialogOpen] = useState(false);
  const [editingRule, setEditingRule] = useState<ReviewRule | null>(null);
  const [form, setForm] = useState(emptyForm);

  const pageSize = 20;
  const totalPages = Math.ceil(total / pageSize);

  const fetchRules = useCallback(async () => {
    setLoading(true);
    try {
      const result = await listReviewRules({
        category: category || undefined,
        severity: severity || undefined,
        page,
        pageSize,
      });
      setRules(result.items || []);
      setTotal(result.total);
    } catch (err) {
      console.error("Failed to fetch review rules:", err);
    } finally {
      setLoading(false);
    }
  }, [category, severity, page]);

  useEffect(() => {
    fetchRules();
  }, [fetchRules]);

  const openCreate = () => {
    setEditingRule(null);
    setForm(emptyForm);
    setDialogOpen(true);
  };

  const openEdit = (rule: ReviewRule) => {
    setEditingRule(rule);
    setForm({
      name: rule.name,
      category: rule.category,
      scope: rule.scope,
      scopeId: rule.scopeId,
      ruleType: rule.ruleType,
      definitionStr: JSON.stringify(rule.definition, null, 2),
      severity: rule.severity,
      autoFix: rule.autoFix,
      fixTemplate: rule.fixTemplate || "",
    });
    setDialogOpen(true);
  };

  const handleSave = async () => {
    try {
      let definition: Record<string, unknown>;
      try {
        definition = JSON.parse(form.definitionStr);
      } catch {
        alert("规则定义 JSON 格式错误");
        return;
      }

      const payload = {
        name: form.name,
        category: form.category,
        scope: form.scope,
        scopeId: form.scopeId,
        ruleType: form.ruleType,
        definition,
        severity: form.severity,
        autoFix: form.autoFix,
        fixTemplate: form.fixTemplate || undefined,
      };

      if (editingRule) {
        await updateReviewRule(editingRule.id, payload);
      } else {
        await createReviewRule(payload);
      }
      setDialogOpen(false);
      fetchRules();
    } catch (err) {
      console.error("Failed to save review rule:", err);
    }
  };

  const handleToggle = async (id: number) => {
    try {
      await toggleReviewRule(id);
      fetchRules();
    } catch (err) {
      console.error("Failed to toggle review rule:", err);
    }
  };

  return (
    <div className="space-y-4">
      {/* Toolbar */}
      <div className="flex items-center justify-between gap-4">
        <div className="flex items-center gap-3">
          <Select value={category} onValueChange={(v) => { setCategory(v === "all" ? "" : v); setPage(1); }}>
            <SelectTrigger className="w-[160px] bg-white/5 border-white/10 text-white">
              <SelectValue placeholder="全部分类" />
            </SelectTrigger>
            <SelectContent>
              {RULE_CATEGORIES.map((c) => (
                <SelectItem key={c.value || "all"} value={c.value || "all"}>
                  {c.label}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
          <Select value={severity} onValueChange={(v) => { setSeverity(v === "all" ? "" : v); setPage(1); }}>
            <SelectTrigger className="w-[140px] bg-white/5 border-white/10 text-white">
              <SelectValue placeholder="全部级别" />
            </SelectTrigger>
            <SelectContent>
              {SEVERITIES.map((s) => (
                <SelectItem key={s.value || "all"} value={s.value || "all"}>
                  {s.label}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        </div>
        <Button onClick={openCreate} className="bg-[#8B5CF6] hover:bg-[#7C3AED] text-white">
          <Plus className="h-4 w-4 mr-2" />
          新建规则
        </Button>
      </div>

      {/* Table */}
      <div className="bg-white/[0.03] border border-white/10 rounded-lg overflow-hidden">
        <table className="w-full">
          <thead>
            <tr className="border-b border-white/10 text-white/50 text-sm">
              <th className="text-left px-4 py-3 font-medium">名称</th>
              <th className="text-left px-4 py-3 font-medium">分类</th>
              <th className="text-left px-4 py-3 font-medium">类型</th>
              <th className="text-left px-4 py-3 font-medium">严重级别</th>
              <th className="text-left px-4 py-3 font-medium">自动修复</th>
              <th className="text-left px-4 py-3 font-medium">启用</th>
              <th className="text-right px-4 py-3 font-medium">操作</th>
            </tr>
          </thead>
          <tbody>
            {loading ? (
              <tr>
                <td colSpan={7} className="text-center py-12 text-white/30">
                  加载中...
                </td>
              </tr>
            ) : rules.length === 0 ? (
              <tr>
                <td colSpan={7} className="text-center py-12 text-white/30">
                  暂无 Review 规则
                </td>
              </tr>
            ) : (
              rules.map((rule) => {
                const sevConfig = SEVERITY_CONFIG[rule.severity] || SEVERITY_CONFIG.INFO;
                const SevIcon = sevConfig.icon;
                return (
                  <tr
                    key={rule.id}
                    className="border-b border-white/5 hover:bg-white/[0.02] transition-colors"
                  >
                    <td className="px-4 py-3 text-white font-medium">{rule.name}</td>
                    <td className="px-4 py-3">
                      <span
                        className={`inline-flex px-2 py-0.5 rounded text-xs font-medium ${
                          CATEGORY_COLORS[rule.category] || "bg-white/10 text-white/70"
                        }`}
                      >
                        {RULE_CATEGORIES.find((c) => c.value === rule.category)?.label || rule.category}
                      </span>
                    </td>
                    <td className="px-4 py-3 text-white/50 text-sm">
                      {RULE_TYPES.find((t) => t.value === rule.ruleType)?.label || rule.ruleType}
                    </td>
                    <td className="px-4 py-3">
                      <span className={`inline-flex items-center gap-1 px-2 py-0.5 rounded text-xs font-medium ${sevConfig.color}`}>
                        <SevIcon className="h-3 w-3" />
                        {rule.severity}
                      </span>
                    </td>
                    <td className="px-4 py-3">
                      {rule.autoFix && (
                        <Zap className="h-4 w-4 text-yellow-400" />
                      )}
                    </td>
                    <td className="px-4 py-3">
                      <Switch
                        checked={rule.enabled}
                        onCheckedChange={() => handleToggle(rule.id)}
                      />
                    </td>
                    <td className="px-4 py-3 text-right">
                      <Button
                        variant="ghost"
                        size="icon"
                        className="h-8 w-8 text-white/50 hover:text-white"
                        onClick={() => openEdit(rule)}
                      >
                        <Edit2 className="h-4 w-4" />
                      </Button>
                    </td>
                  </tr>
                );
              })
            )}
          </tbody>
        </table>
      </div>

      {/* Pagination */}
      {totalPages > 1 && (
        <div className="flex items-center justify-between text-sm text-white/50">
          <span>共 {total} 条，第 {page}/{totalPages} 页</span>
          <div className="flex items-center gap-2">
            <Button variant="ghost" size="icon" className="h-8 w-8" disabled={page <= 1} onClick={() => setPage((p) => p - 1)}>
              <ChevronLeft className="h-4 w-4" />
            </Button>
            <Button variant="ghost" size="icon" className="h-8 w-8" disabled={page >= totalPages} onClick={() => setPage((p) => p + 1)}>
              <ChevronRight className="h-4 w-4" />
            </Button>
          </div>
        </div>
      )}

      {/* Create/Edit Dialog */}
      <Dialog open={dialogOpen} onOpenChange={setDialogOpen}>
        <DialogContent className="bg-[#0A0A12] border-white/10 text-white max-w-2xl max-h-[90vh] overflow-y-auto">
          <DialogHeader>
            <DialogTitle>
              {editingRule ? "编辑 Review 规则" : "新建 Review 规则"}
            </DialogTitle>
          </DialogHeader>
          <div className="space-y-4 py-4">
            <div className="space-y-2">
              <Label>规则名称</Label>
              <Input
                value={form.name}
                onChange={(e) => setForm({ ...form, name: e.target.value })}
                placeholder="如：禁止空 catch 块"
                className="bg-white/5 border-white/10"
              />
            </div>
            <div className="grid grid-cols-3 gap-4">
              <div className="space-y-2">
                <Label>分类</Label>
                <Select value={form.category} onValueChange={(v) => setForm({ ...form, category: v })}>
                  <SelectTrigger className="bg-white/5 border-white/10">
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    {RULE_CATEGORIES.filter((c) => c.value).map((c) => (
                      <SelectItem key={c.value} value={c.value}>{c.label}</SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </div>
              <div className="space-y-2">
                <Label>规则类型</Label>
                <Select value={form.ruleType} onValueChange={(v) => setForm({ ...form, ruleType: v })}>
                  <SelectTrigger className="bg-white/5 border-white/10">
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    {RULE_TYPES.map((t) => (
                      <SelectItem key={t.value} value={t.value}>{t.label}</SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </div>
              <div className="space-y-2">
                <Label>严重级别</Label>
                <Select value={form.severity} onValueChange={(v) => setForm({ ...form, severity: v })}>
                  <SelectTrigger className="bg-white/5 border-white/10">
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    {SEVERITIES.filter((s) => s.value).map((s) => (
                      <SelectItem key={s.value} value={s.value}>{s.label}</SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </div>
            </div>
            <div className="space-y-2">
              <Label>规则定义 (JSON)</Label>
              <Textarea
                value={form.definitionStr}
                onChange={(e) => setForm({ ...form, definitionStr: e.target.value })}
                placeholder='{"pattern": "...", "language": "java", "description": "..."}'
                className="bg-[#0A0A12] border-white/10 font-mono text-sm min-h-[120px]"
              />
            </div>
            <div className="flex items-center gap-3">
              <Switch
                checked={form.autoFix}
                onCheckedChange={(v) => setForm({ ...form, autoFix: v })}
              />
              <Label>支持自动修复</Label>
            </div>
            {form.autoFix && (
              <div className="space-y-2">
                <Label>修复模板</Label>
                <Textarea
                  value={form.fixTemplate}
                  onChange={(e) => setForm({ ...form, fixTemplate: e.target.value })}
                  placeholder="自动修复的代码模板..."
                  className="bg-[#0A0A12] border-white/10 font-mono text-sm min-h-[100px]"
                />
              </div>
            )}
          </div>
          <DialogFooter>
            <Button variant="ghost" onClick={() => setDialogOpen(false)} className="text-white/50">
              取消
            </Button>
            <Button
              onClick={handleSave}
              className="bg-[#8B5CF6] hover:bg-[#7C3AED] text-white"
              disabled={!form.name || !form.definitionStr}
            >
              {editingRule ? "保存修改" : "创建"}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  );
}
```

- [ ] **Step 3: 创建 Scaffolds 页面（只读卡片视图）**

`forge-portal/app/(dashboard)/specs/scaffolds/page.tsx`：

```tsx
"use client";

import { useState, useEffect, useCallback } from "react";
import { ExternalLink, Code2, Globe, Layers, Package } from "lucide-react";
import { Button } from "@/components/ui/button";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { ScaffoldTemplate, listScaffoldTemplates } from "@/lib/specs";

const PROJECT_TYPES = [
  { value: "", label: "全部类型" },
  { value: "JAVA_MICROSERVICE", label: "Java 微服务" },
  { value: "VUE_FRONTEND", label: "Vue 前端" },
  { value: "FULLSTACK", label: "全栈" },
  { value: "SDK", label: "SDK" },
  { value: "BLANK", label: "空白项目" },
];

const TYPE_ICONS: Record<string, typeof Code2> = {
  JAVA_MICROSERVICE: Code2,
  VUE_FRONTEND: Globe,
  FULLSTACK: Layers,
  SDK: Package,
  BLANK: Code2,
};

const TYPE_COLORS: Record<string, string> = {
  JAVA_MICROSERVICE: "from-orange-500/20 to-orange-500/5 border-orange-500/20",
  VUE_FRONTEND: "from-green-500/20 to-green-500/5 border-green-500/20",
  FULLSTACK: "from-blue-500/20 to-blue-500/5 border-blue-500/20",
  SDK: "from-purple-500/20 to-purple-500/5 border-purple-500/20",
  BLANK: "from-white/10 to-white/5 border-white/10",
};

export default function ScaffoldsPage() {
  const [scaffolds, setScaffolds] = useState<ScaffoldTemplate[]>([]);
  const [projectType, setProjectType] = useState("");
  const [loading, setLoading] = useState(true);

  const fetchScaffolds = useCallback(async () => {
    setLoading(true);
    try {
      const result = await listScaffoldTemplates({
        projectType: projectType || undefined,
        pageSize: 50,
      });
      setScaffolds(result.items || []);
    } catch (err) {
      console.error("Failed to fetch scaffold templates:", err);
    } finally {
      setLoading(false);
    }
  }, [projectType]);

  useEffect(() => {
    fetchScaffolds();
  }, [fetchScaffolds]);

  return (
    <div className="space-y-4">
      {/* Toolbar */}
      <div className="flex items-center justify-between gap-4">
        <Select value={projectType} onValueChange={(v) => setProjectType(v === "all" ? "" : v)}>
          <SelectTrigger className="w-[160px] bg-white/5 border-white/10 text-white">
            <SelectValue placeholder="全部类型" />
          </SelectTrigger>
          <SelectContent>
            {PROJECT_TYPES.map((t) => (
              <SelectItem key={t.value || "all"} value={t.value || "all"}>
                {t.label}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
        <span className="text-sm text-white/30">Phase 1 仅支持查看，后续版本可编辑</span>
      </div>

      {/* Cards Grid */}
      {loading ? (
        <div className="text-center py-12 text-white/30">加载中...</div>
      ) : scaffolds.length === 0 ? (
        <div className="text-center py-12 text-white/30">暂无脚手架模板</div>
      ) : (
        <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
          {scaffolds.map((scaffold) => {
            const Icon = TYPE_ICONS[scaffold.projectType] || Code2;
            const colors = TYPE_COLORS[scaffold.projectType] || TYPE_COLORS.BLANK;
            return (
              <div
                key={scaffold.id}
                className={`bg-gradient-to-br ${colors} border rounded-lg p-5 space-y-3`}
              >
                <div className="flex items-start justify-between">
                  <div className="flex items-center gap-3">
                    <div className="p-2 rounded-lg bg-white/5">
                      <Icon className="h-5 w-5 text-white/70" />
                    </div>
                    <div>
                      <h3 className="text-white font-medium">{scaffold.name}</h3>
                      <span className="text-xs text-white/40">
                        {PROJECT_TYPES.find((t) => t.value === scaffold.projectType)?.label || scaffold.projectType}
                        {" "}v{scaffold.version}
                      </span>
                    </div>
                  </div>
                  {scaffold.templateRepo && (
                    <a
                      href={scaffold.templateRepo}
                      target="_blank"
                      rel="noopener noreferrer"
                      className="text-white/30 hover:text-white/60 transition-colors"
                    >
                      <ExternalLink className="h-4 w-4" />
                    </a>
                  )}
                </div>

                {scaffold.description && (
                  <p className="text-sm text-white/50 leading-relaxed">
                    {scaffold.description}
                  </p>
                )}

                {scaffold.variables.length > 0 && (
                  <div className="space-y-1">
                    <span className="text-xs text-white/30">模板变量</span>
                    <div className="flex flex-wrap gap-1.5">
                      {scaffold.variables.map((v) => (
                        <span
                          key={v}
                          className="px-2 py-0.5 rounded bg-white/5 text-white/50 text-xs font-mono"
                        >
                          {v}
                        </span>
                      ))}
                    </div>
                  </div>
                )}

                {scaffold.postHooks.length > 0 && (
                  <div className="space-y-1">
                    <span className="text-xs text-white/30">Post Hooks</span>
                    <div className="space-y-1">
                      {scaffold.postHooks.map((hook, i) => (
                        <code
                          key={i}
                          className="block px-2 py-1 rounded bg-black/30 text-white/40 text-xs font-mono"
                        >
                          $ {hook}
                        </code>
                      ))}
                    </div>
                  </div>
                )}
              </div>
            );
          })}
        </div>
      )}
    </div>
  );
}
```

- [ ] **Step 4: 验证**

```bash
cd forge-portal && npm run build
# 预期: 编译成功

# 本地开发
cd forge-portal && npm run dev
# 测试以下页面:
# http://localhost:3000/specs/standards — 编码规范列表
# http://localhost:3000/specs/prompts — Prompt 模板列表（6 条种子数据）
# http://localhost:3000/specs/rules — Review 规则列表（5 条种子数据，可启用/禁用）
# http://localhost:3000/specs/scaffolds — 脚手架模板卡片（4 条种子数据）
```

- [ ] **Step 5: Commit**

```bash
git add forge-portal/app/\(dashboard\)/specs/prompts/ forge-portal/app/\(dashboard\)/specs/rules/ forge-portal/app/\(dashboard\)/specs/scaffolds/
git commit -m "feat(s5): add prompts, rules, and scaffolds pages for specs center"
```

---

## Task 6: 项目级规范覆盖页面

**Files:**
- Create: `forge-portal/app/(dashboard)/projects/[id]/settings/specs/page.tsx`

- [ ] **Step 1: 创建项目规范覆盖页面**

`forge-portal/app/(dashboard)/projects/[id]/settings/specs/page.tsx`：

```tsx
"use client";

import { useState, useEffect, useCallback } from "react";
import { useParams } from "next/navigation";
import {
  Copy,
  ChevronDown,
  ChevronRight,
  BookOpen,
  ShieldCheck,
  Building2,
  FolderGit2,
} from "lucide-react";
import { Button } from "@/components/ui/button";
import {
  Collapsible,
  CollapsibleContent,
  CollapsibleTrigger,
} from "@/components/ui/collapsible";
import {
  Standard,
  ReviewRule,
  EffectiveSpecs,
  getEffectiveSpecs,
  createStandard,
  createReviewRule,
} from "@/lib/specs";

export default function ProjectSpecsPage() {
  const params = useParams();
  const projectId = Number(params.id);

  const [effectiveSpecs, setEffectiveSpecs] = useState<EffectiveSpecs | null>(null);
  const [loading, setLoading] = useState(true);
  const [expandedStandards, setExpandedStandards] = useState<Set<number>>(new Set());
  const [expandedRules, setExpandedRules] = useState<Set<number>>(new Set());

  const fetchSpecs = useCallback(async () => {
    setLoading(true);
    try {
      const result = await getEffectiveSpecs(projectId);
      setEffectiveSpecs(result);
    } catch (err) {
      console.error("Failed to fetch effective specs:", err);
    } finally {
      setLoading(false);
    }
  }, [projectId]);

  useEffect(() => {
    fetchSpecs();
  }, [fetchSpecs]);

  const toggleStandard = (id: number) => {
    setExpandedStandards((prev) => {
      const next = new Set(prev);
      if (next.has(id)) next.delete(id);
      else next.add(id);
      return next;
    });
  };

  const toggleRule = (id: number) => {
    setExpandedRules((prev) => {
      const next = new Set(prev);
      if (next.has(id)) next.delete(id);
      else next.add(id);
      return next;
    });
  };

  const handleOverrideStandard = async (std: Standard) => {
    if (!confirm(`确定要为此项目创建"${std.name}"的覆盖副本吗？`)) return;
    try {
      await createStandard({
        name: std.name,
        category: std.category,
        scope: "PROJECT",
        scopeId: projectId,
        parentId: std.id,
        content: std.content,
      });
      fetchSpecs();
    } catch (err) {
      console.error("Failed to override standard:", err);
    }
  };

  const handleOverrideRule = async (rule: ReviewRule) => {
    if (!confirm(`确定要为此项目创建"${rule.name}"的覆盖副本吗？`)) return;
    try {
      await createReviewRule({
        name: rule.name,
        category: rule.category,
        scope: "PROJECT",
        scopeId: projectId,
        ruleType: rule.ruleType,
        definition: rule.definition,
        severity: rule.severity,
        autoFix: rule.autoFix,
        fixTemplate: rule.fixTemplate || undefined,
      });
      fetchSpecs();
    } catch (err) {
      console.error("Failed to override rule:", err);
    }
  };

  const ScopeIcon = ({ scope }: { scope: string }) =>
    scope === "COMPANY" ? (
      <Building2 className="h-3.5 w-3.5 text-[#8B5CF6]" />
    ) : (
      <FolderGit2 className="h-3.5 w-3.5 text-green-400" />
    );

  if (loading) {
    return (
      <div className="flex items-center justify-center h-64 text-white/30">
        加载中...
      </div>
    );
  }

  return (
    <div className="space-y-8 max-w-4xl">
      <div>
        <h2 className="text-xl font-bold text-white mb-1">项目规范配置</h2>
        <p className="text-sm text-white/50">
          查看此项目的有效规范。公司级规范自动继承，点击"Override"可创建项目级覆盖副本。
        </p>
      </div>

      {/* Standards Section */}
      <div className="space-y-3">
        <div className="flex items-center gap-2 text-white/70">
          <BookOpen className="h-5 w-5" />
          <h3 className="text-lg font-semibold">编码规范</h3>
          <span className="text-sm text-white/30">
            ({effectiveSpecs?.standards?.length || 0})
          </span>
        </div>

        {effectiveSpecs?.standards?.length === 0 ? (
          <div className="text-sm text-white/30 py-4">暂无生效的编码规范</div>
        ) : (
          <div className="space-y-2">
            {effectiveSpecs?.standards?.map((std) => (
              <Collapsible
                key={std.id}
                open={expandedStandards.has(std.id)}
                onOpenChange={() => toggleStandard(std.id)}
              >
                <div className="bg-white/[0.03] border border-white/10 rounded-lg">
                  <CollapsibleTrigger className="flex items-center justify-between w-full px-4 py-3 text-left">
                    <div className="flex items-center gap-3">
                      {expandedStandards.has(std.id) ? (
                        <ChevronDown className="h-4 w-4 text-white/30" />
                      ) : (
                        <ChevronRight className="h-4 w-4 text-white/30" />
                      )}
                      <span className="text-white font-medium">{std.name}</span>
                      <span className="px-2 py-0.5 rounded text-xs font-medium bg-white/10 text-white/50">
                        {std.category}
                      </span>
                      <ScopeIcon scope={std.scope} />
                      <span className="text-xs text-white/30">
                        {std.scope === "COMPANY" ? "公司级" : "项目级"}
                      </span>
                    </div>
                    {std.scope === "COMPANY" && (
                      <Button
                        variant="ghost"
                        size="sm"
                        className="text-[#8B5CF6] hover:text-[#7C3AED] text-xs"
                        onClick={(e) => {
                          e.stopPropagation();
                          handleOverrideStandard(std);
                        }}
                      >
                        <Copy className="h-3 w-3 mr-1" />
                        Override
                      </Button>
                    )}
                  </CollapsibleTrigger>
                  <CollapsibleContent>
                    <div className="px-4 pb-4 border-t border-white/5">
                      <pre className="mt-3 p-4 rounded-lg bg-[#0A0A12] text-white/60 text-sm font-mono whitespace-pre-wrap overflow-auto max-h-[400px]">
                        {std.content}
                      </pre>
                    </div>
                  </CollapsibleContent>
                </div>
              </Collapsible>
            ))}
          </div>
        )}
      </div>

      {/* Rules Section */}
      <div className="space-y-3">
        <div className="flex items-center gap-2 text-white/70">
          <ShieldCheck className="h-5 w-5" />
          <h3 className="text-lg font-semibold">Review 规则</h3>
          <span className="text-sm text-white/30">
            ({effectiveSpecs?.rules?.length || 0})
          </span>
        </div>

        {effectiveSpecs?.rules?.length === 0 ? (
          <div className="text-sm text-white/30 py-4">暂无生效的 Review 规则</div>
        ) : (
          <div className="space-y-2">
            {effectiveSpecs?.rules?.map((rule) => (
              <Collapsible
                key={rule.id}
                open={expandedRules.has(rule.id)}
                onOpenChange={() => toggleRule(rule.id)}
              >
                <div className="bg-white/[0.03] border border-white/10 rounded-lg">
                  <CollapsibleTrigger className="flex items-center justify-between w-full px-4 py-3 text-left">
                    <div className="flex items-center gap-3">
                      {expandedRules.has(rule.id) ? (
                        <ChevronDown className="h-4 w-4 text-white/30" />
                      ) : (
                        <ChevronRight className="h-4 w-4 text-white/30" />
                      )}
                      <span className="text-white font-medium">{rule.name}</span>
                      <span className="px-2 py-0.5 rounded text-xs font-medium bg-white/10 text-white/50">
                        {rule.category}
                      </span>
                      <span
                        className={`px-2 py-0.5 rounded text-xs font-medium ${
                          rule.severity === "ERROR"
                            ? "bg-red-500/10 text-red-400"
                            : rule.severity === "WARNING"
                            ? "bg-yellow-500/10 text-yellow-400"
                            : "bg-blue-500/10 text-blue-400"
                        }`}
                      >
                        {rule.severity}
                      </span>
                      <ScopeIcon scope={rule.scope} />
                    </div>
                    {rule.scope === "COMPANY" && (
                      <Button
                        variant="ghost"
                        size="sm"
                        className="text-[#8B5CF6] hover:text-[#7C3AED] text-xs"
                        onClick={(e) => {
                          e.stopPropagation();
                          handleOverrideRule(rule);
                        }}
                      >
                        <Copy className="h-3 w-3 mr-1" />
                        Override
                      </Button>
                    )}
                  </CollapsibleTrigger>
                  <CollapsibleContent>
                    <div className="px-4 pb-4 border-t border-white/5">
                      <pre className="mt-3 p-4 rounded-lg bg-[#0A0A12] text-white/60 text-sm font-mono whitespace-pre-wrap overflow-auto max-h-[300px]">
                        {JSON.stringify(rule.definition, null, 2)}
                      </pre>
                      {rule.fixTemplate && (
                        <div className="mt-2">
                          <span className="text-xs text-white/30">修复模板:</span>
                          <pre className="mt-1 p-3 rounded-lg bg-[#0A0A12] text-green-400/60 text-sm font-mono whitespace-pre-wrap">
                            {rule.fixTemplate}
                          </pre>
                        </div>
                      )}
                    </div>
                  </CollapsibleContent>
                </div>
              </Collapsible>
            ))}
          </div>
        )}
      </div>
    </div>
  );
}
```

- [ ] **Step 2: 在项目设置导航中添加"规范配置"入口**

在项目设置页的导航菜单中（通常在 `forge-portal/app/(dashboard)/projects/[id]/settings/layout.tsx` 或类似位置），添加规范配置链接：

```tsx
// 在项目设置侧边导航中添加:
{ label: "规范配置", href: `/projects/${id}/settings/specs`, icon: BookOpen },
```

- [ ] **Step 3: 验证**

```bash
cd forge-portal && npm run build
# 预期: 编译成功

# 本地开发
cd forge-portal && npm run dev
# 浏览器打开 http://localhost:3000/projects/1/settings/specs
# 预期:
# - 显示继承的编码规范列表（4 条公司级种子数据）
# - 显示继承的 Review 规则列表（5 条公司级种子数据）
# - 每条规范旁有 Override 按钮（仅公司级显示）
# - 点击 Override 创建项目级副本
# - 展开可查看规范内容和规则定义
```

- [ ] **Step 4: Commit**

```bash
git add forge-portal/app/\(dashboard\)/projects/\[id\]/settings/specs/
git commit -m "feat(s5): add project-level specs override page with inheritance display"
```

---

## 验收标准

全部任务完成后，执行以下端到端验证：

1. **数据库**: 4 张 specs 表已创建，种子数据已插入（4 + 6 + 5 + 4 = 19 条记录）
2. **后端 API**:
   - `GET /api/specs/standards` 返回编码规范列表
   - `POST /api/specs/standards` 可创建新规范
   - `PUT /api/specs/standards/:id` 可更新规范（version 自增）
   - `DELETE /api/specs/standards/:id` 软删除
   - `GET /api/specs/prompts` 返回 6 条默认模板
   - `GET /api/specs/rules` 返回 5 条默认规则
   - `DELETE /api/specs/rules/:id` 切换启用/禁用
   - `GET /api/specs/scaffolds` 返回 4 条脚手架模板
   - `GET /api/specs/effective/:projectId` 返回合并后的有效规范
3. **前端页面**:
   - `/specs` 显示 4 个 tab 页面
   - `/specs/standards` 表格 + 筛选 + 分页 + CRUD 弹窗
   - `/specs/prompts` 表格 + 代码编辑器风格弹窗 + 变量管理
   - `/specs/rules` 表格 + 启用/禁用开关 + 严重级别图标
   - `/specs/scaffolds` 卡片式展示 + 类型图标
   - `/projects/:id/settings/specs` 继承展示 + Override 按钮
4. **缓存**: 有效规范通过 Redis 缓存 10 分钟，规范变更时自动失效
5. **视觉**: 深空指挥中心暗色主题，分类 badge 使用语义色（10% 透明度），代码区域使用 Geist Mono + #0A0A12 背景
