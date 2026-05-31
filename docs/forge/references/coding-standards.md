# 编码规范基线

> **来源**: aegis 工程实践沉淀（shulex-coding-standards.md + project-structure.md）
> **用途**: 作为 Forge 规范中心编码规范的基线，AI 生成代码必须遵守

---

## 1. 命名规范

| 类型 | 规则 |
|------|------|
| 类名 | UpperCamelCase，禁止拼音混合 |
| 方法/变量 | lowerCamelCase |
| 常量 | UPPER_SNAKE_CASE |
| 包名 | 全小写，层级用点分隔 |

**领域模型后缀（强制）**：

| 后缀 | 用途 | 所在包 |
|------|------|--------|
| DO | 数据库实体，与表一一映射 | repository/entity |
| DTO | 服务间传输对象 | service/dto |
| VO | 视图对象，面向前端 | entrance/vo |
| BO | 业务对象，封装业务逻辑 | domain/bo |
| Query | 查询参数对象 | api/query |

**Service/DAO 方法命名**：
- 获取单个：getXxx
- 获取列表：listXxx
- 计数：countXxx
- 插入：saveXxx / insertXxx
- 删除：removeXxx / deleteXxx
- 修改：updateXxx

---

## 2. 分层架构

| 层级 | 职责 | 包名 |
|------|------|------|
| Entrance（入口层） | Controller、VO、请求/响应转换 | entrance/ |
| Service（业务层） | 业务逻辑、DTO、Service 接口与实现 | service/ |
| Domain（领域层） | 领域实体、业务规则、BO | domain/ |
| Infrastructure（基础设施层） | Mapper、外部服务调用、DBO | infrastructure/ |
| Common（公共层） | 枚举、常量、异常、工具 | common/ |

---

## 3. 统一响应封装

所有 API 返回值使用 Result<T> 包装：
- code：错误码（"0" = 成功）
- message：可读消息
- data：业务数据
- timestamp：时间戳

---

## 4. 异常体系

三层异常结构：
- 基础异常（抽象，携带 ErrorCode）
- BizException（业务异常，如"任务不存在"）
- SysException（系统异常，如"数据库连接失败"）
- 领域异常（按业务域细分，如 ChannelException、RateLimitException）

集中式 ErrorCode 枚举管理所有错误码，禁止硬编码。

全局异常处理器统一捕获并转换为 Result 格式返回。

---

## 5. 代码格式

- 大括号风格：K&R（左括号不换行）
- 行宽限制：120 字符
- 缩进：4 空格（禁止 Tab）
- 编码：UTF-8
- 依赖注入：@RequiredArgsConstructor 构造器注入，禁止 @Autowired 字段注入

---

## 6. 集合与并发

- hashCode/equals 必须成对实现
- 线程池必须命名，禁止直接 Executors.newXxx
- SimpleDateFormat 禁止跨线程共享
- 乐观锁优先于悲观锁
- 锁范围最小化

---

## 7. 日志规范

- 统一使用 SLF4J 门面
- 占位符 {} 方式传参，禁止字符串拼接
- 保留 15 天以上
- 禁止重复打印（上层已打的下层不再打）
- 异常必须打完整栈

---

## 8. MySQL 规范

- 布尔字段以 is_ 前缀
- 表名全小写、单数、禁止保留字
- 必备字段：id（BIGINT UNSIGNED AUTO_INCREMENT）、gmt_create、gmt_modified
- 索引命名：idx_字段名 / uk_字段名
- 禁止 SELECT *
- 禁止超过 3 表 JOIN

---

## 9. 安全规范

- 所有接口必须校验授权
- 敏感数据必须脱敏
- SQL 参数化防注入
- 用户输入必须校验
- XSS / CSRF 防护
- 核心接口限流

---

## 10. Git 规范

**分支策略**：main / develop / feature / hotfix / release

**Commit 格式**：`type(scope): subject`
- type：feat / fix / refactor / test / docs / chore
- scope：模块名
- subject：简洁描述
