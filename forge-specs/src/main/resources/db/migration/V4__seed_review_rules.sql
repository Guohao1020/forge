-- V4: Seed review rules (15+ rules across coding/security/performance/database)

INSERT INTO spec_review_rule (category, rule_key, name, description, severity, is_enabled) VALUES

-- Coding rules (5)
('coding', 'naming-convention-check', '命名规范检查',
'## 命名规范检查

### 检查内容
验证代码中的类名、方法名、变量名是否符合命名规范。

### 规则
- 类名必须使用 UpperCamelCase：`UserService`、`OrderDO`
- 方法名和局部变量必须使用 lowerCamelCase：`getUserById`、`userName`
- 常量必须全大写加下划线：`MAX_RETRY_COUNT`
- 禁止使用拼音命名：`yongHuMingCheng`（违规）

### 示例

**违规代码：**
```java
public class userService {          // 类名小写开头
    private String UserName;        // 字段名大写开头
    public void GetUser() {}        // 方法名大写开头
    private final int maxcount = 3; // 常量未大写
}
```

**合规代码：**
```java
public class UserService {
    private String userName;
    public void getUser() {}
    private static final int MAX_COUNT = 3;
}
```',
'error', 1),

('coding', 'do-dto-layer-check', 'DO/DTO 分层检查',
'## DO/DTO 分层检查

### 检查内容
验证数据对象的类型与其所在层次是否匹配，防止跨层传递错误类型的对象。

### 规则
- Controller（entrance 层）只能接收/返回 VO 和 DTO
- Service 层方法参数和返回值使用 DTO 或 BO
- Repository/Mapper 层只操作 DO
- 禁止将 DO 对象直接返回给 Controller 或前端

### 示例

**违规代码：**
```java
// Controller 直接返回 DO（违规！）
@GetMapping("/{id}")
public Result<UserDO> getUser(@PathVariable Long id) {
    return Result.ok(userRepository.findById(id));
}
```

**合规代码：**
```java
// Controller 返回 VO
@GetMapping("/{id}")
public Result<UserVO> getUser(@PathVariable Long id) {
    UserVO vo = userService.getById(id); // Service 负责 DO → VO 转换
    return Result.ok(vo);
}
```',
'error', 1),

('coding', 'constructor-injection-check', '构造器注入检查',
'## 构造器注入检查

### 检查内容
检测 Spring Bean 中是否使用了被禁止的字段注入（`@Autowired` 或 `@Inject` 注解在字段上）。

### 规则
- 禁止在字段上使用 `@Autowired`
- 禁止在字段上使用 `@Inject`
- 必须使用构造器注入或 Lombok `@RequiredArgsConstructor`

### 示例

**违规代码：**
```java
@Service
public class OrderService {
    @Autowired
    private UserService userService;    // 违规！

    @Inject
    private OrderRepository repository; // 违规！
}
```

**合规代码：**
```java
@Service
@RequiredArgsConstructor
public class OrderService {
    private final UserService userService;
    private final OrderRepository repository;
}
```',
'error', 1),

('coding', 'result-wrapper-check', 'Result<T> 响应包装检查',
'## Result<T> 响应包装检查

### 检查内容
检测 Controller 层方法的返回类型是否使用了 `Result<T>` 统一包装。

### 规则
- 所有 `@RestController` 中的 `@GetMapping`、`@PostMapping`、`@PutMapping`、`@DeleteMapping`、`@PatchMapping` 方法必须返回 `Result<T>`
- 禁止直接返回业务对象、`ResponseEntity` 或 `void`

### 示例

**违规代码：**
```java
@RestController
public class UserController {
    @GetMapping("/{id}")
    public UserVO getUser(@PathVariable Long id) { // 违规！未包装
        return userService.getById(id);
    }

    @DeleteMapping("/{id}")
    public void deleteUser(@PathVariable Long id) { // 违规！void 未包装
        userService.delete(id);
    }
}
```

**合规代码：**
```java
@GetMapping("/{id}")
public Result<UserVO> getUser(@PathVariable Long id) {
    return Result.ok(userService.getById(id));
}

@DeleteMapping("/{id}")
public Result<Void> deleteUser(@PathVariable Long id) {
    userService.delete(id);
    return Result.ok(null);
}
```',
'error', 1),

('coding', 'slf4j-logging-check', 'SLF4J 日志规范检查',
'## SLF4J 日志规范检查

### 检查内容
检测代码中的日志使用是否符合 SLF4J 规范，禁止字符串拼接和使用 `System.out`。

### 规则
1. 禁止使用 `System.out.println` 或 `System.err.println`
2. 禁止在日志方法中使用字符串 `+` 拼接
3. 必须使用占位符 `{}` 传参
4. Logger 必须声明为 `private static final`

### 示例

**违规代码：**
```java
System.out.println("用户登录: " + userId); // 违规！

log.info("用户登录: " + userId + ", ip: " + ip); // 违规！字符串拼接

Logger logger = LoggerFactory.getLogger(getClass()); // 违规！非 static final
```

**合规代码：**
```java
private static final Logger log = LoggerFactory.getLogger(UserService.class);

log.info("用户登录，userId={}, ip={}", userId, ip);
log.error("数据库查询失败，table={}", tableName, e);
```',
'warning', 1),

-- Security rules (5)
('security', 'sql-injection-check', 'SQL 注入检查',
'## SQL 注入检查

### 检查内容
检测代码中是否存在通过字符串拼接构建 SQL 查询的安全漏洞。

### 风险
SQL 注入可导致数据库数据泄露、篡改甚至删除，是 OWASP Top 10 最高危漏洞之一。

### 检查规则
- 检测 JDBC 中使用 `Statement`（而非 `PreparedStatement`）执行动态 SQL
- 检测 MyBatis XML 中使用 `${}` 而非 `#{}`
- 检测 SQL 字符串中包含变量拼接

### 示例

**违规代码：**
```java
// JDBC 字符串拼接
String sql = "SELECT * FROM user WHERE name = ''" + name + "''";
stmt.executeQuery(sql); // 高危！

// MyBatis ${} 注入
// SELECT * FROM user WHERE name = ''${name}''  高危！
```

**合规代码：**
```java
// JDBC 参数化
PreparedStatement ps = conn.prepareStatement(
    "SELECT id, name FROM user WHERE name = ?");
ps.setString(1, name);

// MyBatis #{} 参数化
// SELECT id, name FROM user WHERE name = #{name}
```',
'error', 1),

('security', 'xss-protection-check', 'XSS 防护检查',
'## XSS 防护检查

### 检查内容
检测用户输入是否在输出到响应时经过了适当的编码或过滤，防止跨站脚本攻击。

### 检查规则
- 检测直接将用户输入写入 HTTP 响应的代码
- 检测富文本字段是否经过白名单过滤
- 检测响应头是否设置了 `X-XSS-Protection` 和 `Content-Security-Policy`

### 示例

**违规代码：**
```java
// 直接输出用户输入到响应（XSS 漏洞）
response.getWriter().write("<p>" + userInput + "</p>"); // 高危！

// 富文本未过滤直接存储和展示
entity.setContent(userHtml); // 违规！应先过滤
```

**合规代码：**
```java
// HTML 编码后输出
String safe = HtmlUtils.htmlEscape(userInput);
response.getWriter().write("<p>" + safe + "</p>");

// 富文本白名单过滤
String safeHtml = Jsoup.clean(userHtml, Whitelist.relaxed());
entity.setContent(safeHtml);
```',
'error', 1),

('security', 'csrf-protection-check', 'CSRF 防护检查',
'## CSRF 防护检查

### 检查内容
检测 Spring Security 配置中是否正确启用了 CSRF 保护，或在关闭时是否有等效的替代方案。

### 检查规则
- 检测是否存在 `csrf().disable()` 且未配置 JWT/Token 认证
- 检测 Cookie 是否缺少 `SameSite` 属性
- 检测状态变更操作是否通过 GET 请求暴露

### 示例

**违规代码（纯 Session 认证时关闭 CSRF）：**
```java
http.csrf().disable(); // 危险！Session 认证下必须开启 CSRF 保护
```

**合规代码（关闭 CSRF 但使用 JWT）：**
```java
http.csrf().disable()  // JWT 无状态认证时可以关闭
    .sessionManagement()
    .sessionCreationPolicy(SessionCreationPolicy.STATELESS)
    .and()
    .addFilterBefore(jwtAuthFilter, UsernamePasswordAuthenticationFilter.class);
```

**合规代码（启用 CSRF Token）：**
```java
http.csrf()
    .csrfTokenRepository(CookieCsrfTokenRepository.withHttpOnlyFalse());
```',
'error', 1),

('security', 'sensitive-data-masking-check', '敏感数据脱敏检查',
'## 敏感数据脱敏检查

### 检查内容
检测接口响应和日志输出中是否包含未脱敏的敏感数据（手机号、身份证、密码等）。

### 检查规则
- VO 类中的手机号、身份证等字段必须标注脱敏注解或手动脱敏
- 日志中禁止打印密码、Token、完整手机号
- 数据库中密码字段不能以明文或 MD5 存储

### 示例

**违规代码：**
```java
// VO 直接暴露手机号（违规！）
public class UserVO {
    private String phone;    // 138xxxxxxxx，未脱敏
    private String password; // 明文密码！
}

// 日志暴露敏感信息（违规！）
log.info("用户注册，phone={}, password={}", phone, password);
```

**合规代码：**
```java
public class UserVO {
    @Desensitize(type = DesensitizeType.PHONE)
    private String phone;    // 输出：138****8888

    @JsonIgnore
    private String password; // 不序列化到 JSON
}

log.info("用户注册，phone={}", PhoneUtils.mask(phone)); // 138****8888
```',
'error', 1),

('security', 'permission-check', '权限校验检查',
'## 权限校验检查

### 检查内容
检测 Controller 接口是否遗漏了权限控制注解，防止未授权访问。

### 检查规则
- 所有非公开接口必须标注 `@PreAuthorize` 或类级别权限注解
- 禁止直接信任客户端传入的用户 ID（应从 SecurityContext 中获取当前用户）
- 资源的 CRUD 操作必须校验资源归属

### 示例

**违规代码：**
```java
// 接口未标注权限注解（任何人可访问）
@GetMapping("/admin/users")
public Result<List<UserVO>> listAllUsers() { // 违规！缺少权限控制
    return Result.ok(userService.listAll());
}

// 直接信任客户端传入的 userId（IDOR 漏洞）
@DeleteMapping("/{userId}/orders/{orderId}")
public Result<Void> deleteOrder(@PathVariable Long userId, ...) {
    // 没有校验 userId 是否是当前登录用户！
}
```

**合规代码：**
```java
@PreAuthorize("hasRole(''ADMIN'')")
@GetMapping("/admin/users")
public Result<List<UserVO>> listAllUsers() { ... }

@DeleteMapping("/orders/{orderId}")
public Result<Void> deleteOrder(@PathVariable Long orderId) {
    Long currentUserId = SecurityContextHolder.getCurrentUserId();
    orderService.deleteByIdAndUserId(orderId, currentUserId);
    return Result.ok(null);
}
```',
'error', 1),

-- Performance rules (3)
('performance', 'n-plus-one-query-check', 'N+1 查询检查',
'## N+1 查询检查

### 检查内容
检测代码中是否存在在循环内执行数据库查询的 N+1 问题，导致大量不必要的数据库往返。

### 检查规则
- 检测循环体内存在数据库查询调用
- 检测 ORM 懒加载触发的 N+1 场景
- 要求批量查询替代逐个查询

### 示例

**违规代码：**
```java
// N+1 问题：每个订单都单独查询用户
List<Order> orders = orderRepository.findAll();
for (Order order : orders) {
    User user = userRepository.findById(order.getUserId()); // N 次查询！
    order.setUserName(user.getName());
}
```

**合规代码：**
```java
// 批量查询：1次查询所有用户
List<Order> orders = orderRepository.findAll();
Set<Long> userIds = orders.stream().map(Order::getUserId).collect(toSet());
Map<Long, User> userMap = userRepository.findAllById(userIds)
    .stream().collect(toMap(User::getId, u -> u));
for (Order order : orders) {
    order.setUserName(userMap.get(order.getUserId()).getName());
}
```',
'warning', 1),

('performance', 'batch-operation-check', '批量操作检查',
'## 批量操作检查

### 检查内容
检测代码中是否在循环内执行单条 INSERT/UPDATE/DELETE，应改用批量操作。

### 检查规则
- 循环内的单条 insert/update/delete 调用必须改为批量操作
- 批量操作单次数量建议控制在 500-1000 条，超出需分批

### 示例

**违规代码：**
```java
// 循环单条插入（性能极差）
for (UserDO user : users) {
    userRepository.save(user); // 每次都是一次 INSERT，N 次网络往返！
}
```

**合规代码：**
```java
// 批量插入
int batchSize = 500;
for (int i = 0; i < users.size(); i += batchSize) {
    List<UserDO> batch = users.subList(i, Math.min(i + batchSize, users.size()));
    userRepository.saveAll(batch); // 一次 INSERT ... VALUES(...),(...)
}
```

**MyBatis XML 批量插入：**
```xml
<insert id="batchInsert">
    INSERT INTO user (name, email) VALUES
    <foreach collection="list" item="item" separator=",">
        (#{item.name}, #{item.email})
    </foreach>
</insert>
```',
'warning', 1),

('performance', 'cache-usage-check', '缓存使用检查',
'## 缓存使用检查

### 检查内容
检测高频查询是否使用了缓存，以及缓存使用是否符合规范（避免缓存穿透、雪崩、击穿）。

### 检查规则
- 频繁读取且变化不频繁的数据（如配置、用户信息）应使用 `@Cacheable`
- 缓存 Key 必须包含业务标识，避免冲突
- 缓存 Value 必须设置过期时间，禁止永不过期
- 对可能不存在的数据要防止缓存穿透（缓存空值）

### 示例

**违规代码：**
```java
// 高频查询未使用缓存
public UserVO getUserById(Long id) {
    return userMapper.findById(id); // 每次都查数据库！
}

// 缓存未设置过期时间（违规！）
redisTemplate.opsForValue().set("user:" + id, user); // 永不过期
```

**合规代码：**
```java
@Cacheable(value = "user", key = "#id", unless = "#result == null")
public UserVO getUserById(Long id) {
    return userMapper.findById(id);
}

// 手动设置缓存，必须加过期时间
redisTemplate.opsForValue().set("user:" + id, user, 30, TimeUnit.MINUTES);

// 防缓存穿透：不存在时缓存空值
if (user == null) {
    redisTemplate.opsForValue().set("user:" + id, NULL_VALUE, 5, TimeUnit.MINUTES);
}
```',
'info', 1),

-- Database rules (2)
('database', 'index-design-check', '索引设计检查',
'## 索引设计检查

### 检查内容
分析表的查询模式，检测索引是否缺失、冗余或设计不合理。

### 检查规则
- WHERE 子句中的高频查询字段必须有索引
- 联合索引列顺序必须符合最左前缀原则和区分度原则
- 单表索引数量不得超过 5 个（过多索引影响写性能）
- 索引命名必须遵循 `idx_` 或 `uk_` 前缀规范
- 禁止在低区分度字段（如 status、gender）上单独建立索引

### 示例

**违规设计：**
```sql
-- 缺少必要索引
SELECT * FROM order WHERE user_id = 1 AND status = 2;
-- 但 order 表只有主键索引，user_id 没有索引！

-- 低区分度单字段索引（浪费）
CREATE INDEX idx_status ON order (status); -- status 只有 3 个值
```

**合规设计：**
```sql
-- 联合索引（user_id 区分度高放左边，status 过滤性好作为第二列）
CREATE INDEX idx_user_id_status ON order (user_id, status);

-- 唯一索引命名规范
CREATE UNIQUE INDEX uk_order_no ON order (order_no);
```',
'warning', 1),

('database', 'slow-query-check', '慢查询检查',
'## 慢查询检查

### 检查内容
检测可能导致慢查询的 SQL 写法，包括索引失效场景和全表扫描风险。

### 检查规则
- WHERE 条件中禁止对索引列使用函数（导致索引失效）
- 禁止在索引列上做隐式类型转换
- LIKE 查询禁止以通配符 `%` 开头（无法使用索引）
- 大数据量分页禁止使用大 OFFSET（改用游标翻页）

### 示例

**违规写法（索引失效）：**
```sql
-- 对索引列使用函数（违规！索引失效）
SELECT * FROM user WHERE DATE(gmt_create) = ''2024-01-01'';

-- LIKE 前缀通配符（违规！全表扫描）
SELECT * FROM user WHERE name LIKE ''%alice%'';

-- 大 OFFSET 深度分页（违规！扫描大量数据后丢弃）
SELECT * FROM user ORDER BY id LIMIT 1000000, 10;
```

**合规写法：**
```sql
-- 范围查询代替函数
SELECT * FROM user WHERE gmt_create >= ''2024-01-01'' AND gmt_create < ''2024-01-02'';

-- 全文索引或搜索引擎处理模糊搜索
-- 游标翻页代替 OFFSET
SELECT * FROM user WHERE id > #{lastId} ORDER BY id LIMIT 10;
```',
'warning', 1);
