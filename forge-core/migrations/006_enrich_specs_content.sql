-- =============================================
-- S5-patch: 完善企业级规范内容
-- 基于 docs/references/coding-standards.md + PRD 2.7
-- =============================================

-- 1. 更新 Java 编码规范（完整企业级）
UPDATE specs.standards SET content = '## Java 编码规范

### 1. 命名规范

#### 1.1 类命名
- 类名使用 UpperCamelCase 风格，禁止拼音与英文混用
- 抽象类以 Abstract / Base 开头
- 异常类以 Exception 结尾
- 测试类以被测类名 + Test 结尾
- 枚举类以 Enum 结尾，枚举值使用 UPPER_SNAKE_CASE

#### 1.2 方法/变量命名
- 方法名使用 lowerCamelCase
- 常量使用 UPPER_SNAKE_CASE，禁止魔法数字
- 布尔类型变量禁止以 is 开头（POJO 中框架序列化会出错）

#### 1.3 领域模型后缀（强制）
| 后缀 | 用途 | 所在包 |
|------|------|--------|
| DO | 数据库实体，与表一一映射 | repository/entity |
| DTO | 服务间传输对象 | service/dto |
| VO | 视图对象，面向前端 | entrance/vo |
| BO | 业务对象，封装业务逻辑 | domain/bo |
| Query | 查询参数对象 | api/query |

#### 1.4 Service/DAO 方法命名
- 获取单个：getXxx
- 获取列表：listXxx
- 计数：countXxx
- 插入：saveXxx / insertXxx
- 删除：removeXxx / deleteXxx
- 修改：updateXxx

### 2. 分层架构

| 层级 | 职责 | 包名 |
|------|------|------|
| Entrance（入口层） | Controller、VO、请求/响应转换 | entrance/ |
| Service（业务层） | 业务逻辑、DTO、Service 接口与实现 | service/ |
| Domain（领域层） | 领域实体、业务规则、BO | domain/ |
| Infrastructure（基础设施层） | Mapper、外部服务调用、DBO | infrastructure/ |
| Common（公共层） | 枚举、常量、异常、工具 | common/ |

**依赖规则**：上层可调下层，禁止反向依赖。Entrance 禁止直接调 Infrastructure。

### 3. 统一响应封装

所有 API 返回值使用 Result<T> 包装：
```java
public class Result<T> {
    private String code;      // "0" = 成功
    private String message;   // 可读消息
    private T data;           // 业务数据
    private long timestamp;   // 时间戳
}
```

### 4. 异常体系

三层异常结构：
- **基础异常**（抽象，携带 ErrorCode + message）
- **BizException**（业务异常：如"任务不存在"、"余额不足"）
- **SysException**（系统异常：如"数据库连接失败"、"RPC 超时"）
- **领域异常**（按业务域细分，继承 BizException）

集中式 ErrorCode 枚举管理所有错误码，禁止在代码中硬编码错误码字符串。
全局异常处理器（@RestControllerAdvice）统一捕获并转换为 Result 格式返回。

### 5. 代码格式

- 大括号风格：K&R（左括号不换行）
- 行宽限制：120 字符
- 缩进：4 空格（禁止 Tab）
- 编码：UTF-8
- 方法长度不超过 80 行
- 类长度不超过 500 行
- 方法参数不超过 5 个，超过则封装为参数对象

### 6. 依赖注入

- **强制构造器注入**（@RequiredArgsConstructor + final 字段）
- **禁止** @Autowired 字段注入
- **禁止** @Autowired 构造器参数注入
- Bean 之间通过接口交互，禁止直接依赖实现类

### 7. 集合与并发

- hashCode 和 equals 必须成对实现
- 使用 Objects.equals() 比较，避免 NPE
- 线程池必须自定义命名（ThreadFactory），禁止直接使用 Executors.newXxx
- SimpleDateFormat 禁止跨线程共享，推荐使用 DateTimeFormatter
- 乐观锁优先于悲观锁
- 锁范围最小化，禁止在循环体内加锁
- ConcurrentHashMap 的 value 禁止为 null

### 8. 日志规范

- 统一使用 SLF4J 门面，禁止直接使用 Log4j/Logback API
- 占位符 {} 方式传参，禁止字符串拼接
- 日志保留 15 天以上
- 禁止重复打印（上层已打的下层不再打）
- 异常日志必须打完整栈：log.error("msg", e) 而非 log.error(e.getMessage())
- WARN 级别：可预期的业务异常
- ERROR 级别：需要人工介入的系统异常

### 9. 注释规范

- 公开 API 必须有 Javadoc（类、接口、公共方法）
- 复杂算法必须有行内注释说明思路
- TODO 格式：// TODO [负责人] [截止日期] 描述
- 禁止注释掉的死代码提交到仓库
- 接口注释说明约定：入参约束、返回值含义、异常场景', version = version + 1, updated_at = NOW()
WHERE tenant_id = 1 AND category = 'JAVA';

-- 2. 更新 SQL 编码规范（完整企业级）
UPDATE specs.standards SET content = '## SQL 编码规范

### 1. 命名规范

#### 1.1 表命名
- 全小写，使用 snake_case
- 使用单数名词（user 而非 users，但 Forge 平台本身因 Go 惯例用复数）
- 禁止使用 MySQL/PostgreSQL 保留字
- 关联表命名：主表_从表（如 user_role）

#### 1.2 字段命名
- 全小写 snake_case
- 布尔字段以 is_ 前缀（is_deleted, is_active）
- 时间字段以 _at 或 gmt_ 前缀（created_at, gmt_modified）
- 金额字段使用 DECIMAL，禁止 FLOAT/DOUBLE

#### 1.3 索引命名
- 普通索引：idx_{table}_{column}
- 唯一索引：uk_{table}_{column}
- 联合索引：idx_{table}_{col1}_{col2}
- 外键（如需）：fk_{table}_{ref_table}

### 2. 必备字段

每张业务表必须包含：
- id: BIGINT / BIGSERIAL 主键（禁止 UUID 做主键，性能差）
- created_at: TIMESTAMPTZ NOT NULL DEFAULT NOW()
- updated_at: TIMESTAMPTZ NOT NULL DEFAULT NOW()
- tenant_id: BIGINT NOT NULL（多租户隔离）

可选但推荐：
- created_by: BIGINT（操作人）
- is_deleted: BOOLEAN DEFAULT FALSE（逻辑删除）
- version: INT DEFAULT 1（乐观锁）

### 3. 查询规范

- **禁止 SELECT ***，必须明确列出字段
- **JOIN 不超过 3 张表**，超过需拆分为多次查询
- 子查询优先改写为 JOIN（性能更可控）
- 大表查询必须命中索引，禁止全表扫描
- LIKE 查询禁止左模糊（%keyword），会导致索引失效
- IN 列表不超过 1000 个元素
- 分页查询必须带 ORDER BY，避免结果不稳定
- 深分页使用 keyset pagination（WHERE id > last_id）替代 OFFSET

### 4. DDL 规范

- 字符串字段指定长度上限（VARCHAR(n)），大文本使用 TEXT
- 枚举值存储为 VARCHAR，不使用数据库 ENUM 类型（迁移不便）
- 外键建议在应用层维护，数据库层面可选
- 每张表必须有主键索引
- 高频查询字段建立索引，但单表索引不超过 5 个
- 联合索引遵循最左前缀原则

### 5. DML 规范

- INSERT 必须指定字段列表，禁止 INSERT INTO t VALUES (...)
- 批量 INSERT 每批不超过 500 条
- UPDATE/DELETE 必须带 WHERE 条件，禁止全表操作
- 大批量数据变更分批执行，避免长事务
- 事务范围最小化，禁止在事务中进行 RPC 调用或文件 IO

### 6. 迁移规范

- 所有 Schema 变更必须通过迁移文件管理
- 迁移文件按序号命名：001_init_auth.sql, 002_init_engine.sql
- 禁止修改已执行的迁移文件
- 新增字段必须有默认值或允许 NULL
- 删除字段前先标记废弃，下一版本再物理删除', version = version + 1, updated_at = NOW()
WHERE tenant_id = 1 AND category = 'SQL';

-- 3. 更新 API 设计规范（完整企业级）
UPDATE specs.standards SET content = '## API 设计规范

### 1. URL 规范

- 使用 RESTful 风格
- 资源名使用复数名词（/api/users, /api/projects）
- URL 路径使用 kebab-case（/api/user-roles）
- 版本号放在 URL path：/api/v1/
- 嵌套资源不超过 2 层：/api/projects/:id/tasks（✓）
- 禁止在 URL 中放动词：/api/getUser（✗）→ GET /api/users/:id（✓）

### 2. HTTP 方法语义

| 方法 | 语义 | 幂等 | 示例 |
|------|------|------|------|
| GET | 查询资源 | ✓ | GET /api/users/:id |
| POST | 创建资源 | ✗ | POST /api/users |
| PUT | 全量更新 | ✓ | PUT /api/users/:id |
| PATCH | 部分更新 | ✓ | PATCH /api/users/:id |
| DELETE | 删除资源 | ✓ | DELETE /api/users/:id |

### 3. 请求规范

- Content-Type: application/json（禁止 form-urlencoded，文件上传除外）
- 查询参数使用 camelCase：?pageSize=20&sortBy=createdAt
- 路径参数使用 :id 占位符
- 请求体字段使用 camelCase
- 分页参数：page（从 1 开始）、pageSize（默认 20，最大 100）
- 排序参数：sortBy=field&sortOrder=asc|desc

### 4. 响应规范

统一使用 Result<T> 包装：
```json
{
  "code": 0,
  "message": "ok",
  "data": { ... }
}
```

列表响应：
```json
{
  "code": 0,
  "message": "ok",
  "data": {
    "items": [...],
    "total": 100,
    "page": 1,
    "pageSize": 20
  }
}
```

- 时间字段使用 ISO 8601 格式：2024-01-15T10:30:00Z
- 金额字段使用字符串或整数（分），避免浮点精度问题
- 空集合返回 []，不返回 null
- 布尔字段使用 true/false，不使用 0/1

### 5. 错误响应

```json
{
  "code": 1001,
  "message": "参数校验失败：邮箱格式不正确",
  "data": null
}
```

错误码分类：
| 范围 | 含义 |
|------|------|
| 0 | 成功 |
| 1xxx | 参数校验错误 |
| 2xxx | 认证错误 |
| 3xxx | 授权错误 |
| 4xxx | 资源错误（不存在/冲突） |
| 5xxx | 系统内部错误 |

### 6. HTTP 状态码映射

| 状态码 | 使用场景 |
|--------|---------|
| 200 | 成功（GET/PUT/PATCH/DELETE） |
| 201 | 创建成功（POST） |
| 400 | 参数校验失败 |
| 401 | 未认证 |
| 403 | 无权限 |
| 404 | 资源不存在 |
| 409 | 资源冲突 |
| 429 | 请求过于频繁 |
| 500 | 服务器内部错误 |

### 7. 安全规范

- 所有接口必须经过认证（白名单除外）
- 敏感操作需二次确认或审批
- 响应中禁止返回密码、密钥等敏感字段
- 启用 CORS 白名单，禁止 Access-Control-Allow-Origin: *
- 添加安全头：X-Content-Type-Options, X-Frame-Options, Referrer-Policy

### 8. 版本管理

- 大版本变更（不兼容）：URL 路径升版（/v1 → /v2）
- 小版本变更（兼容）：新增字段不影响旧客户端
- 废弃 API：先标记 Deprecated，保留 2 个版本周期后移除
- 变更日志：每次 API 变更必须更新 CHANGELOG', version = version + 1, updated_at = NOW()
WHERE tenant_id = 1 AND category = 'API';

-- 4. 更新 Git 工作流规范（完整企业级）
UPDATE specs.standards SET content = '## Git 工作流规范

### 1. 分支策略

| 分支 | 用途 | 保护级别 |
|------|------|---------|
| main | 生产分支，始终可部署 | 保护分支，禁止直接推送 |
| develop | 开发集成分支 | 保护分支，仅接受 MR/PR |
| feature/* | 功能开发分支 | 开发者可自由推送 |
| hotfix/* | 紧急修复分支 | 从 main 拉出 |
| release/* | 发布准备分支 | 预发布验证 |

**分支命名规范**：
- feature/[issue-id]-short-description（如 feature/FORGE-123-add-login）
- hotfix/[issue-id]-short-description
- release/v1.2.0

### 2. 提交规范（Conventional Commits）

格式：`type(scope): subject`

| type | 用途 | 示例 |
|------|------|------|
| feat | 新功能 | feat(auth): add GitHub OAuth login |
| fix | 修复 Bug | fix(task): correct status transition logic |
| refactor | 重构（不改变行为） | refactor(project): extract validation helper |
| test | 测试 | test(specs): add merge inheritance tests |
| docs | 文档 | docs: update API reference |
| chore | 构建/工具 | chore: upgrade Go to 1.22 |
| perf | 性能优化 | perf(query): add index for tenant lookup |
| style | 格式调整 | style: fix indentation in handler.go |
| ci | CI/CD | ci: add lint step to pipeline |

**subject 规范**：
- 使用英文，首字母小写
- 不加句号结尾
- 限制 72 字符以内
- 使用祈使句（add, fix, update，不用 added, fixed, updated）

### 3. Code Review 规范

- **所有合并必须经过 Code Review**
- 至少 1 人 Approve 后才能合并
- CI/CD Pipeline 全部通过后才能合并
- Review 关注点：
  - 逻辑正确性
  - 编码规范合规
  - 安全漏洞
  - 性能隐患
  - 测试覆盖
  - 可维护性

### 4. 合并策略

- feature → develop：Squash Merge（保持 develop 历史干净）
- develop → main：Merge Commit（保留完整历史）
- hotfix → main + develop：Cherry-pick

### 5. 发布流程

1. 从 develop 拉出 release/vX.Y.Z 分支
2. 在 release 分支上进行预发布验证
3. 修复发现的问题（仅 bugfix）
4. 验证通过后合并到 main，打 tag
5. main 合并回 develop
6. 部署 main 到生产环境

### 6. 紧急修复流程

1. 从 main 最新 tag 拉出 hotfix 分支
2. 修复 + 测试
3. Code Review（可简化但不可跳过）
4. 合并到 main，打 patch tag
5. Cherry-pick 回 develop', version = version + 1, updated_at = NOW()
WHERE tenant_id = 1 AND category = 'GIT';

-- 5. 新增：安全编码规范（幂等：跳过已存在的分类）
INSERT INTO specs.standards (tenant_id, name, category, scope, scope_id, content, created_by)
SELECT 1, '安全编码规范', 'SECURITY', 'COMPANY', 0,
'## 安全编码规范

### 1. 认证与授权

- 所有接口必须校验用户身份（白名单接口需在配置中显式声明）
- 基于角色的访问控制（RBAC），权限粒度到接口级别
- Token 有效期不超过 8 小时，Refresh Token 不超过 7 天
- 密码存储必须使用 bcrypt / argon2id，禁止 MD5/SHA 系列
- 登录失败超过 5 次自动锁定账号 30 分钟
- 敏感操作（删除、权限变更）需二次认证或审批

### 2. 输入校验

- **所有外部输入必须校验**（请求参数、Header、Cookie、上传文件）
- 白名单校验优先于黑名单
- 使用强类型绑定 + 校验注解（binding:"required,max=200"）
- 文件上传：校验文件类型（MIME + 扩展名）、限制大小（默认 10MB）
- URL 参数防止路径遍历（../）
- 整数参数校验范围，防止溢出

### 3. SQL 注入防护

- **必须使用参数化查询**（$1/$2 占位符或 NamedArgs）
- 禁止字符串拼接构建 SQL
- 禁止使用 String.format() 构建 SQL
- ORM 动态查询也必须参数化
- 定期使用 SQLMap 或同类工具扫描

### 4. XSS 防护

- 所有用户输入在输出时必须转义
- Content-Type 响应头必须设置正确
- 设置 X-Content-Type-Options: nosniff
- 设置 Content-Security-Policy 限制脚本来源
- 富文本内容使用白名单过滤（如 bluemonday）

### 5. CSRF 防护

- 状态修改请求（POST/PUT/DELETE）必须携带 CSRF Token
- OAuth state 参数使用 crypto/rand 生成，禁止可预测值
- 设置 SameSite Cookie 属性
- 检查 Referer / Origin 头

### 6. 敏感数据处理

- 密码、密钥、Token 禁止出现在日志中
- 响应中禁止返回密码字段（即使是哈希值）
- 数据库中的敏感字段（Token、密钥）必须加密存储（AES-256-GCM）
- 配置文件中的密钥必须通过环境变量注入，禁止硬编码
- 日志中的手机号、邮箱、身份证需脱敏（中间部分用 * 替代）

### 7. HTTP 安全头

必须设置的安全响应头：
```
X-Content-Type-Options: nosniff
X-Frame-Options: DENY
X-XSS-Protection: 0
Referrer-Policy: strict-origin-when-cross-origin
Strict-Transport-Security: max-age=31536000; includeSubDomains (HTTPS环境)
```

### 8. 依赖安全

- 定期扫描第三方依赖漏洞（go mod audit / npm audit）
- 禁止使用已知有 CVE 的版本
- 锁定依赖版本（go.sum / package-lock.json）
- 最小化依赖，避免引入不必要的包

### 9. 错误处理安全

- 生产环境禁止返回堆栈信息给客户端
- 错误消息不应泄露系统内部信息（数据库类型、表名、文件路径）
- 认证失败统一返回"用户名或密码错误"，不区分"用户不存在"和"密码错误"
- 500 错误记录完整日志，但只返回通用提示给客户端', 1
WHERE NOT EXISTS (SELECT 1 FROM specs.standards WHERE tenant_id = 1 AND category = 'SECURITY' AND scope = 'COMPANY');

-- 6. 新增：Redis 使用规范（幂等）
INSERT INTO specs.standards (tenant_id, name, category, scope, scope_id, content, created_by)
SELECT 1, 'Redis 使用规范', 'REDIS', 'COMPANY', 0,
'## Redis 使用规范

### 1. Key 命名规范

- 使用冒号分隔层级：{业务}:{对象}:{ID}
- 示例：user:session:12345, specs:effective:1:42, task:lock:789
- Key 长度不超过 128 字节
- 禁止使用特殊字符（空格、换行）
- 环境前缀（可选）：prod:, dev:, test:

### 2. Value 规范

- String 类型 value 不超过 1MB
- 单个 Hash 字段数不超过 128 个
- List/Set/ZSet 元素数不超过 10000（超过需分片）
- 序列化推荐 JSON（可读性好）或 Protocol Buffers（性能高）
- 禁止存储未序列化的 Java/Go 对象

### 3. TTL（过期时间）规范

- **所有 Key 必须设置 TTL**，禁止永不过期（防止内存泄漏）
- 缓存类：5~30 分钟（根据数据变化频率调整）
- 会话类：8 小时（与 JWT 有效期一致）
- 锁类：30 秒~5 分钟（防止死锁）
- 避免大量 Key 同时过期（TTL 加随机偏移）

### 4. 缓存模式

- **Cache-Aside**（推荐）：先查缓存，miss 则查 DB 回填缓存
- 写入时更新缓存：写 DB 后删除缓存（非更新缓存，防止并发不一致）
- 缓存穿透防护：空值缓存（TTL 较短，如 1 分钟）
- 缓存雪崩防护：TTL 加随机偏移 + 热点数据永不过期（后台刷新）
- 缓存击穿防护：热点 Key 使用互斥锁或 singleflight

### 5. 分布式锁

- 使用 SET key value NX EX（原子操作）
- value 使用唯一标识（UUID），释放锁时需比较
- 锁超时必须设置（防止死锁）
- 业务执行时间可能超过锁超时 → 看门狗续期
- 禁止使用 SETNX + EXPIRE（非原子）

### 6. 禁止事项

- 禁止 KEYS * 命令（生产环境会阻塞）→ 使用 SCAN
- 禁止 FLUSHDB / FLUSHALL
- 禁止存储超过 10MB 的大 Value（Big Key）
- 禁止在 Lua 脚本中执行长时间操作
- 禁止将 Redis 当作持久化存储（数据可能丢失）

### 7. 监控指标

- 内存使用率（不超过 70%）
- 命中率（应大于 95%）
- 慢查询日志（slowlog）
- 连接数
- Key 数量增长趋势', 1
WHERE NOT EXISTS (SELECT 1 FROM specs.standards WHERE tenant_id = 1 AND category = 'REDIS' AND scope = 'COMPANY');

-- 7. 新增：命名规范（幂等）
INSERT INTO specs.standards (tenant_id, name, category, scope, scope_id, content, created_by)
SELECT 1, '通用命名规范', 'NAMING', 'COMPANY', 0,
'## 通用命名规范

### 1. 核心原则

- **名副其实**：名称应完整表达含义，无需注释解释
- **避免误导**：不用 list 命名非 List 类型变量
- **有意义的区分**：productInfo vs productData 无法区分 → 用 productSummary vs productDetail
- **可读可搜索**：禁止单字母变量（循环计数器 i/j/k 除外）

### 2. 项目/仓库命名

- 使用 kebab-case：forge-core, ai-worker, devops-worker
- 清晰表达项目用途
- 禁止使用缩写（除非是广泛认知的：api, sdk, cli）

### 3. 数据库命名

| 对象 | 风格 | 示例 |
|------|------|------|
| Schema | snake_case | auth, engine, specs |
| Table | snake_case 单数/复数 | users, task_steps |
| Column | snake_case | tenant_id, created_at |
| Index | idx_{table}_{cols} | idx_users_tenant_id |
| Unique | uk_{table}_{cols} | uk_users_email |

### 4. API 命名

| 对象 | 风格 | 示例 |
|------|------|------|
| URL Path | kebab-case | /api/review-rules |
| Query Param | camelCase | ?pageSize=20&sortBy=name |
| JSON Field | camelCase | { "tenantId": 1, "createdAt": "..." } |
| Header | Title-Case | X-Request-ID, Authorization |

### 5. 前端命名

| 对象 | 风格 | 示例 |
|------|------|------|
| 组件文件 | kebab-case / PascalCase | create-project-dialog.tsx |
| 组件名 | PascalCase | CreateProjectDialog |
| Hook | camelCase, use 前缀 | useAuth, useTaskStream |
| 常量 | UPPER_SNAKE_CASE | MAX_FILE_SIZE, API_BASE_URL |
| CSS 变量 | kebab-case | --font-geist-sans |
| 事件处理器 | handle 前缀 | handleSubmit, handleDelete |

### 6. 后端命名

| 对象 | 风格 | 示例 |
|------|------|------|
| Go Package | lowercase | specs, auth, middleware |
| Go Struct | PascalCase | TaskService, ReviewRule |
| Go Interface | PascalCase (er 后缀) | Reader, WorkflowStarter |
| Go Function | PascalCase (exported) | NewService, ListStandards |
| Java Package | lowercase.dot | com.shulex.forge.auth |
| Java Class | PascalCase | ProjectController |
| Java Interface | PascalCase (I 前缀可选) | ProjectService |

### 7. 配置命名

| 对象 | 风格 | 示例 |
|------|------|------|
| 环境变量 | UPPER_SNAKE_CASE | DATABASE_URL, JWT_SECRET |
| YAML Key | kebab-case | server-port, redis-addr |
| .env Key | UPPER_SNAKE_CASE | GITHUB_CLIENT_ID |

### 8. 禁止事项

- 禁止拼音命名（dingdan → order）
- 禁止拼音+英文混合（getPingfen → getScore）
- 禁止无意义的数字后缀（handler2, service3）
- 禁止过度缩写（svc 可以, svcrpstr 不行）', 1
WHERE NOT EXISTS (SELECT 1 FROM specs.standards WHERE tenant_id = 1 AND category = 'NAMING' AND scope = 'COMPANY');

-- 8. 新增：Kafka 消息规范（幂等）
INSERT INTO specs.standards (tenant_id, name, category, scope, scope_id, content, created_by)
SELECT 1, 'Kafka 消息规范', 'KAFKA', 'COMPANY', 0,
'## Kafka 消息规范

### 1. Topic 命名

- 格式：{环境}.{业务域}.{事件类型}.{版本}
- 示例：prod.order.created.v1, prod.user.updated.v1
- 全小写，使用点号分隔
- 禁止使用下划线（与 Kafka 内部 metric 冲突）

### 2. 消息格式

统一使用 JSON，必须包含信封字段：
```json
{
  "messageId": "uuid-v4",
  "timestamp": "2024-01-15T10:30:00Z",
  "source": "forge-core",
  "type": "task.status.changed",
  "version": "1.0",
  "tenantId": 1,
  "data": { ... }
}
```

### 3. 分区策略

- 需要顺序保证的消息：使用业务 Key 作为 Partition Key
- 示例：同一任务的状态变更 → 以 taskId 为 Key（保证同一任务在同一分区）
- 不需要顺序的消息：使用 Round-Robin 分区

### 4. 消费者规范

- Consumer Group 命名：{服务名}.{用途}（如 ai-worker.task-processor）
- 消费者必须处理幂等（同一消息可能被重复投递）
- 消费失败：重试 3 次后发送到死信队列（DLQ）
- 处理超时设置合理值（默认 30 秒）
- 禁止在消费逻辑中进行长时间阻塞操作

### 5. 生产者规范

- 发送消息必须设置 Key（用于分区和日志追踪）
- 设置合理的 acks 级别：关键业务 acks=all，通知类 acks=1
- 启用 idempotent producer 防止重复发送
- 消息大小不超过 1MB（建议 < 100KB）
- 大数据量考虑压缩（lz4 推荐）

### 6. Schema 管理

- 消息格式变更必须向后兼容
- 新增字段使用可选类型（nullable）
- 禁止删除已有字段，标记 @Deprecated 后保留 2 个版本
- 重大变更创建新版本 Topic（v1 → v2）

### 7. 监控

- 消费延迟（Consumer Lag）：报警阈值 10000 条
- 生产/消费速率
- 失败消息数量
- 死信队列深度', 1
WHERE NOT EXISTS (SELECT 1 FROM specs.standards WHERE tenant_id = 1 AND category = 'KAFKA' AND scope = 'COMPANY');
