-- V2: Seed coding standards (15 standards across java/sql/api/security/git)

INSERT INTO spec_standard (category, title, content, scope_level, scope_id, sort_order, is_enabled) VALUES

-- Java standards (5)
('java', 'Java 命名规范',
'## Java 命名规范

### 类名
使用 **UpperCamelCase**（大驼峰）命名所有类。

```java
// 正确
public class UserService {}
public class OrderDO {}
public class QueryResultDTO {}

// 错误
public class userService {}
public class order_do {}
```

### 方法与变量名
使用 **lowerCamelCase**（小驼峰）命名方法和局部变量。

```java
// 正确
String userName = "alice";
public void processOrder() {}

// 错误
String UserName = "alice";
public void ProcessOrder() {}
```

### 分层对象命名后缀
| 层次 | 后缀 | 说明 |
|------|------|------|
| 数据层 | DO | Database Object，与表结构一一对应 |
| 传输层 | DTO | Data Transfer Object，服务间传输 |
| 视图层 | VO | View Object，返回前端的数据 |
| 业务层 | BO | Business Object，业务逻辑对象 |

```java
public class UserDO {}       // 数据库实体
public class UserDTO {}      // 服务间传输
public class UserVO {}       // 前端响应
public class UserBO {}       // 业务对象
```',
'company', NULL, 10, 1),

('java', 'Java 分层架构规范',
'## Java 分层架构规范

项目必须遵循以下五层结构，禁止跨层调用。

```
src/main/java/com/example/
├── entrance/        # 入口层：Controller、消费者、定时任务
├── service/         # 服务层：业务逻辑编排
├── domain/          # 领域层：核心业务规则、领域模型
├── infrastructure/  # 基础设施层：数据库、缓存、MQ、第三方调用
└── common/          # 公共层：常量、枚举、工具类、异常定义
```

### 调用方向
```
entrance → service → domain → infrastructure
                             → common（任意层均可引用）
```

### 禁止行为
- **禁止** entrance 层直接调用 infrastructure 层
- **禁止** domain 层调用 service 层（防止循环依赖）
- **禁止** infrastructure 层调用 domain 层

```java
// 错误：Controller 直接操作 Repository
@RestController
public class UserController {
    @Autowired
    private UserRepository userRepository; // 违规！
}

// 正确：Controller 只调用 Service
@RestController
public class UserController {
    private final UserService userService;
    public UserController(UserService userService) {
        this.userService = userService;
    }
}
```',
'company', NULL, 20, 1),

('java', 'Java 异常体系规范',
'## Java 异常体系规范

### 异常分类
| 异常类 | 说明 | 示例场景 |
|--------|------|----------|
| `BizException` | 业务异常，可预期 | 用户不存在、余额不足 |
| `SysException` | 系统异常，不可预期 | 数据库连接失败、RPC超时 |

### ErrorCode 规范
所有错误码必须定义在 `ErrorCode` 枚举中，格式为 `模块_错误描述`。

```java
public enum ErrorCode {
    USER_NOT_FOUND("USER_001", "用户不存在"),
    ORDER_INSUFFICIENT_BALANCE("ORDER_001", "余额不足"),
    SYS_DB_CONNECTION_FAILED("SYS_001", "数据库连接失败");

    private final String code;
    private final String message;
}
```

### 使用规范
```java
// 正确：抛出业务异常
if (user == null) {
    throw new BizException(ErrorCode.USER_NOT_FOUND);
}

// 正确：捕获后包装为系统异常
try {
    return userRepository.findById(id);
} catch (DataAccessException e) {
    throw new SysException(ErrorCode.SYS_DB_CONNECTION_FAILED, e);
}

// 禁止：直接抛出 RuntimeException
throw new RuntimeException("user not found"); // 违规！
```',
'company', NULL, 30, 1),

('java', 'Java 日志规范',
'## Java 日志规范

### 使用 SLF4J
所有日志必须使用 SLF4J，禁止使用 `System.out.println` 或直接使用 Log4j/Logback API。

```java
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;

public class UserService {
    private static final Logger log = LoggerFactory.getLogger(UserService.class);
}
```

### 使用占位符，禁止字符串拼接
```java
// 正确：使用占位符
log.info("用户登录成功，userId={}, ip={}", userId, clientIp);
log.warn("订单支付超时，orderId={}", orderId);
log.error("数据库查询失败，table={}", tableName, e);

// 错误：字符串拼接（性能差，且日志级别未开启时仍会拼接）
log.info("用户登录成功，userId=" + userId + ", ip=" + clientIp); // 违规！
```

### 日志级别
| 级别 | 使用场景 |
|------|----------|
| ERROR | 系统异常、需要立即处理的错误 |
| WARN  | 业务预警、潜在问题 |
| INFO  | 关键业务节点（入参、出参、分支跳转） |
| DEBUG | 调试信息，生产环境关闭 |

### 禁止行为
- 禁止在 catch 块中吞掉异常（至少打印日志）
- 禁止在循环内大量打印 INFO 日志（改用 DEBUG）',
'company', NULL, 40, 1),

('java', 'Java 依赖注入规范',
'## Java 依赖注入规范

### 强制使用构造器注入，禁止字段注入

```java
// 正确：构造器注入
@Service
public class OrderService {
    private final UserService userService;
    private final OrderRepository orderRepository;

    public OrderService(UserService userService, OrderRepository orderRepository) {
        this.userService = userService;
        this.orderRepository = orderRepository;
    }
}

// 错误：字段注入（@Autowired）
@Service
public class OrderService {
    @Autowired
    private UserService userService; // 违规！

    @Inject
    private OrderRepository orderRepository; // 违规！
}
```

### 构造器注入的优势
1. **不可变性**：依赖可声明为 `final`，防止意外修改
2. **可测试性**：单元测试无需 Spring 容器，直接 `new` 即可
3. **明确依赖**：依赖关系在编译期可见，循环依赖会直接报错

### Lombok @RequiredArgsConstructor
对于字段较多的类，可使用 Lombok 注解简化：

```java
@Service
@RequiredArgsConstructor
public class OrderService {
    private final UserService userService;
    private final OrderRepository orderRepository;
    // Lombok 自动生成构造器
}
```',
'company', NULL, 50, 1),

-- SQL standards (3)
('sql', 'SQL 表命名规范',
'## SQL 表命名规范

### 表名规则
- 全部小写，单词间用下划线分隔
- 使用**单数**形式（`user` 而非 `users`）
- 业务模块前缀（如 `order_`、`user_`）

```sql
-- 正确
CREATE TABLE user_profile (...);
CREATE TABLE order_item (...);
CREATE TABLE product_category (...);

-- 错误
CREATE TABLE UserProfile (...);    -- 包含大写
CREATE TABLE order_items (...);    -- 复数形式
CREATE TABLE t_user (...);         -- 无意义前缀
```

### 必备字段
所有业务表必须包含以下字段：

```sql
CREATE TABLE example_table (
    id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY COMMENT ''主键'',
    -- 业务字段 ...
    gmt_create  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT ''创建时间'',
    gmt_modified DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
                 ON UPDATE CURRENT_TIMESTAMP COMMENT ''修改时间''
);
```

| 字段 | 类型 | 说明 |
|------|------|------|
| `id` | BIGINT UNSIGNED AUTO_INCREMENT | 主键，禁止业务含义 |
| `gmt_create` | DATETIME NOT NULL | 创建时间，自动填充 |
| `gmt_modified` | DATETIME NOT NULL | 修改时间，自动更新 |',
'company', NULL, 60, 1),

('sql', 'SQL 索引规范',
'## SQL 索引规范

### 命名规范
| 类型 | 前缀 | 示例 |
|------|------|------|
| 普通索引 | `idx_` | `idx_user_id`, `idx_status_gmt_create` |
| 唯一索引 | `uk_` | `uk_email`, `uk_order_no` |
| 全文索引 | `ft_` | `ft_content` |

```sql
-- 正确
ALTER TABLE user ADD INDEX idx_email (email);
ALTER TABLE order ADD UNIQUE KEY uk_order_no (order_no);

-- 错误
ALTER TABLE user ADD INDEX email_index (email);  -- 缺少前缀
ALTER TABLE user ADD INDEX (email);              -- 无命名
```

### 索引设计原则
1. **选择性高**的字段优先建立索引（如 user_id, order_no）
2. **联合索引**遵循最左前缀原则，将区分度高的字段放左边
3. 单表索引数量**不超过 5 个**
4. 禁止在频繁更新的字段上建立过多索引

```sql
-- 推荐：联合索引，将高区分度字段放左边
CREATE INDEX idx_user_status_date ON order (user_id, status, gmt_create);

-- 不推荐：单独为低区分度字段建索引
CREATE INDEX idx_status ON order (status); -- status 只有几个值，区分度低
```',
'company', NULL, 70, 1),

('sql', 'SQL 查询规范',
'## SQL 查询规范

### 禁止 SELECT *
必须明确指定查询列，禁止使用 `SELECT *`。

```sql
-- 错误
SELECT * FROM user WHERE id = 1;

-- 正确
SELECT id, name, email, status FROM user WHERE id = 1;
```

**原因：**
- 增加网络传输量
- 阻止覆盖索引优化
- 当表结构变更时可能引入不可预期的字段

### 禁止超过 3 表 JOIN
单条 SQL 最多允许 3 张表 JOIN，超过时应通过应用层拆分查询。

```sql
-- 违规：4 表 JOIN
SELECT u.name, o.order_no, p.name, c.name
FROM user u
JOIN order o ON u.id = o.user_id
JOIN product p ON o.product_id = p.id
JOIN category c ON p.category_id = c.id; -- 超过 3 表！

-- 正确：拆分为两次查询，应用层组合
SELECT u.name, o.order_no, o.product_id FROM user u JOIN order o ON u.id = o.user_id;
-- 再根据 product_id 查询产品和分类信息
```

### 其他规范
- WHERE 条件中禁止对索引列做函数操作（会导致索引失效）
- 分页查询使用游标翻页，避免大 OFFSET
- 批量操作使用 IN，单次 IN 的值不超过 1000 个',
'company', NULL, 80, 1),

-- API standards (2)
('api', 'API Result<T> 统一响应包装',
'## API Result<T> 统一响应包装

### 规范
所有 REST API 响应必须使用 `Result<T>` 统一包装，禁止直接返回业务对象。

```java
// Result 定义
public class Result<T> {
    private boolean success;
    private String code;
    private String message;
    private T data;

    public static <T> Result<T> ok(T data) { ... }
    public static <T> Result<T> fail(String code, String message) { ... }
    public static <T> Result<T> fail(ErrorCode errorCode) { ... }
}
```

### 使用示例
```java
// 正确：统一包装
@GetMapping("/{id}")
public Result<UserVO> getUser(@PathVariable Long id) {
    UserVO user = userService.getById(id);
    return Result.ok(user);
}

@PostMapping
public Result<Void> createUser(@RequestBody @Valid UserCreateDTO dto) {
    userService.create(dto);
    return Result.ok(null);
}

// 错误：直接返回业务对象
@GetMapping("/{id}")
public UserVO getUser(@PathVariable Long id) { // 违规！
    return userService.getById(id);
}
```

### 响应结构示例
```json
{
  "success": true,
  "code": "SUCCESS",
  "message": "操作成功",
  "data": { "id": 1, "name": "Alice" }
}
```',
'company', NULL, 90, 1),

('api', 'RESTful API 设计规范',
'## RESTful API 设计规范

### URL 设计
- 使用**名词**表示资源，使用**复数**形式
- 使用小写字母和连字符 `-`，禁止下划线
- 版本号放在 URL 路径第一段：`/api/v1/`

```
# 正确
GET    /api/v1/users           # 查询用户列表
GET    /api/v1/users/{id}      # 查询单个用户
POST   /api/v1/users           # 创建用户
PUT    /api/v1/users/{id}      # 全量更新用户
PATCH  /api/v1/users/{id}      # 部分更新用户
DELETE /api/v1/users/{id}      # 删除用户

# 错误
GET /api/v1/getUser            # 动词
GET /api/v1/user_list          # 下划线
GET /api/v1/user               # 单数
```

### HTTP 状态码
| 状态码 | 语义 |
|--------|------|
| 200 | 请求成功 |
| 201 | 创建成功 |
| 400 | 请求参数错误 |
| 401 | 未认证 |
| 403 | 无权限 |
| 404 | 资源不存在 |
| 500 | 服务器内部错误 |

### 命名一致性
- 请求体 DTO 命名：`{Resource}CreateDTO`、`{Resource}UpdateDTO`
- 响应体 VO 命名：`{Resource}VO`、`{Resource}PageVO`',
'company', NULL, 100, 1),

-- Security standards (3)
('security', 'SQL 注入防护规范',
'## SQL 注入防护规范

### 强制使用参数化查询
禁止通过字符串拼接构建 SQL，必须使用预编译语句（PreparedStatement）或 ORM 框架的参数绑定。

```java
// 错误：字符串拼接（SQL 注入漏洞！）
String sql = "SELECT * FROM user WHERE name = ''" + userName + "''";
jdbcTemplate.query(sql, ...); // 违规！

// 正确：参数化查询
String sql = "SELECT id, name, email FROM user WHERE name = ?";
jdbcTemplate.query(sql, new Object[]{userName}, ...);
```

### MyBatis 使用规范
```xml
<!-- 正确：#{} 参数化 -->
<select id="findByName" resultType="UserDO">
    SELECT id, name, email FROM user WHERE name = #{name}
</select>

<!-- 错误：${} 字符串替换（SQL 注入风险！）-->
<select id="findByName" resultType="UserDO">
    SELECT id, name, email FROM user WHERE name = ''${name}'' <!-- 违规！ -->
</select>
```

### 动态排序字段白名单
当需要动态指定排序字段时，必须使用白名单校验：

```java
private static final Set<String> ALLOWED_SORT_FIELDS =
    Set.of("id", "gmt_create", "name");

public List<UserDO> list(String sortField) {
    if (!ALLOWED_SORT_FIELDS.contains(sortField)) {
        throw new BizException(ErrorCode.PARAM_INVALID);
    }
    // 此时 sortField 安全，可用于 ORDER BY
}
```',
'company', NULL, 110, 1),

('security', 'XSS 与 CSRF 防护规范',
'## XSS 与 CSRF 防护规范

### XSS 防护

#### 输出编码
所有用户输入在输出到 HTML 时必须进行 HTML 实体编码：

```java
// 使用 HtmlUtils 或 StringEscapeUtils
import org.springframework.web.util.HtmlUtils;

String safe = HtmlUtils.htmlEscape(userInput);
```

#### Content-Security-Policy
在响应头中设置 CSP，限制脚本来源：

```
Content-Security-Policy: default-src ''self''; script-src ''self'' ''nonce-{random}'';
```

#### 富文本处理
对于允许 HTML 的富文本字段，使用白名单过滤（如 jsoup）：

```java
String safe = Jsoup.clean(userHtml, Whitelist.relaxed());
```

### CSRF 防护

#### Spring Security 配置
```java
@Configuration
public class SecurityConfig extends WebSecurityConfigurerAdapter {
    @Override
    protected void configure(HttpSecurity http) throws Exception {
        http.csrf()
            .csrfTokenRepository(CookieCsrfTokenRepository.withHttpOnlyFalse());
    }
}
```

#### SameSite Cookie
```
Set-Cookie: sessionid=xxx; SameSite=Strict; Secure; HttpOnly
```

对于纯 API 后端（前后端分离），可关闭 CSRF 保护，但必须使用 JWT/OAuth2 Token 认证代替。',
'company', NULL, 120, 1),

('security', '敏感数据脱敏规范',
'## 敏感数据脱敏规范

### 敏感数据定义
以下数据属于敏感数据，必须进行脱敏处理：
- 手机号、身份证号、银行卡号
- 密码、密钥、Token
- 邮箱地址（部分场景）

### 接口响应脱敏
在 VO 字段上使用脱敏注解，确保返回前端的数据已脱敏：

```java
public class UserVO {
    private Long id;
    private String name;

    @Desensitize(type = DesensitizeType.PHONE)
    private String phone;       // 138****8888

    @Desensitize(type = DesensitizeType.ID_CARD)
    private String idCard;      // 110***********1234

    @JsonIgnore
    private String password;    // 不返回给前端
}
```

### 日志脱敏
禁止在日志中打印敏感信息：

```java
// 错误
log.info("用户注册，phone={}, password={}", phone, password); // 违规！

// 正确：脱敏后打印
log.info("用户注册，phone={}", PhoneUtils.mask(phone));
```

### 存储加密
密码必须使用 BCrypt 哈希存储，禁止明文或 MD5：

```java
// 正确
String encoded = passwordEncoder.encode(rawPassword);

// 错误
String md5 = DigestUtils.md5Hex(rawPassword); // 违规！
```',
'company', NULL, 130, 1),

-- Git standards (2)
('git', 'Git 分支策略规范',
'## Git 分支策略规范

### 分支类型
| 分支 | 说明 | 生命周期 |
|------|------|----------|
| `main` | 生产环境代码，永远可部署 | 永久 |
| `develop` | 集成分支，下一版本的开发基础 | 永久 |
| `feature/*` | 新功能开发 | 开发期间 |
| `hotfix/*` | 生产环境紧急修复 | 修复期间 |
| `release/*` | 版本发布准备 | 发布期间 |

### 分支命名规范
```
feature/JIRA-123-add-user-login
feature/user-permission-refactor
hotfix/fix-payment-null-pointer
release/v1.2.0
```

### 工作流程
```
develop ─────────────────────────────→ main
    ↑                                      ↑
    └── feature/* (合并回 develop)          │
    └── hotfix/* (同时合并回 develop 和 main)
```

### 规则
- 禁止直接向 `main` 或 `develop` 分支提交代码
- 所有合并必须通过 Pull Request（至少 1 人 Review）
- feature 分支从 `develop` 拉出，合并回 `develop`
- hotfix 分支从 `main` 拉出，合并回 `main` 和 `develop`',
'company', NULL, 140, 1),

('git', 'Git Commit 格式规范',
'## Git Commit 格式规范

### Conventional Commits 格式
```
<type>(<scope>): <subject>

[body]

[footer]
```

### type 类型
| type | 说明 |
|------|------|
| `feat` | 新功能 |
| `fix` | Bug 修复 |
| `docs` | 文档变更 |
| `style` | 代码格式（不影响逻辑） |
| `refactor` | 重构（非新功能，非修复） |
| `test` | 测试相关 |
| `chore` | 构建/工具链变更 |
| `perf` | 性能优化 |

### 示例
```
feat(user): add OAuth2 login support

Implement Google and GitHub OAuth2 login via Spring Security OAuth2 client.
Closes #123

fix(order): prevent duplicate payment on retry

Use idempotency key stored in Redis to detect duplicate requests.
Fixes #456

docs(api): update user API swagger annotations

chore(deps): upgrade Spring Boot from 3.1.0 to 3.2.4
```

### 规则
- subject 使用英文或中文，动词开头，首字母小写
- subject 不超过 72 个字符
- body 说明"为什么"而不是"做了什么"
- 关联 Issue 在 footer 中注明（`Closes #123`、`Fixes #456`）',
'company', NULL, 150, 1);
