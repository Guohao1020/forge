# M3 — 鉴权中心（轻量版）实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 构建 forge-identity 鉴权中心轻量版，提供账号密码登录、JWT 签发/刷新/吊销、API Token 管理、基础 RBAC（管理员/普通用户）、多租户 tenant_id 隔离，并对接 APISIX 鉴权插件，使所有受保护 API 可验证身份。

**Architecture:** forge-identity 作为独立鉴权微服务（端口 8082），使用 Spring Security 处理认证流程，JJWT 签发 JWT，Redis 存储 Token 黑名单。APISIX 通过 `forward-auth` 插件将每次请求转发到 forge-identity 的 `/auth/verify` 端点验证 Token，验证通过后将 `X-User-Id`/`X-Tenant-Id`/`X-Username` 响应头转发给上游服务。多租户通过服务层方法参数手动传递 `tenant_id` 进行查询过滤（轻量版暂不使用 MyBatis Plus TenantLineInnerInterceptor）。

**Tech Stack:** Java 17, Spring Boot 3.2, Spring Security 6, JJWT 0.12, MyBatis Plus 3.5.5, Redis, Flyway, H2 (test), BCrypt

---

## 文件结构总览

```
forge-identity/
├── pom.xml                                              ← 补充依赖
├── src/main/java/com/shulex/forge/identity/
│   ├── ForgeIdentityApplication.java                    ← 已有
│   ├── common/
│   │   ├── Result.java                                  ← 统一响应
│   │   ├── ErrorCode.java                               ← 错误码枚举
│   │   ├── BizException.java                            ← 业务异常
│   │   ├── SysException.java                            ← 系统异常
│   │   └── GlobalExceptionHandler.java                  ← 全局异常处理
│   ├── infrastructure/
│   │   ├── entity/
│   │   │   ├── TenantDO.java                            ← 租户实体
│   │   │   ├── UserDO.java                              ← 用户实体
│   │   │   ├── RoleDO.java                              ← 角色实体
│   │   │   └── UserRoleDO.java                          ← 用户角色绑定实体
│   │   ├── mapper/
│   │   │   ├── TenantMapper.java                        ← 租户 Mapper
│   │   │   ├── UserMapper.java                          ← 用户 Mapper
│   │   │   ├── RoleMapper.java                          ← 角色 Mapper
│   │   │   └── UserRoleMapper.java                      ← 用户角色绑定 Mapper
│   │   └── config/
│   │       ├── MyBatisPlusConfig.java                   ← MetaObjectHandler + 租户拦截器
│   │       └── SecurityConfig.java                      ← Spring Security 配置
│   ├── service/
│   │   ├── AuthService.java                             ← 认证服务（登录/登出/刷新）
│   │   ├── UserService.java                             ← 用户管理服务
│   │   ├── TokenService.java                            ← JWT 签发/验证/吊销
│   │   └── ApiTokenService.java                         ← API Token 管理
│   └── entrance/
│       ├── controller/
│       │   ├── AuthController.java                      ← 认证 API（登录/登出/刷新/验证）
│       │   ├── UserController.java                      ← 用户管理 API
│       │   └── ApiTokenController.java                  ← API Token 管理 API
│       ├── vo/
│       │   ├── LoginRequest.java                        ← 登录请求
│       │   ├── LoginResponse.java                       ← 登录响应（含 Token）
│       │   ├── RefreshRequest.java                      ← 刷新请求
│       │   ├── UserVO.java                              ← 用户视图
│       │   ├── CreateUserRequest.java                   ← 创建用户请求
│       │   └── ApiTokenVO.java                          ← API Token 视图
│       └── filter/
│           └── JwtAuthFilter.java                       ← JWT 认证过滤器
├── src/main/resources/
│   ├── application.yml                                  ← 更新配置
│   └── db/migration/
│       └── V1__init_identity_tables.sql                 ← 建表 + 种子数据
├── src/test/java/com/shulex/forge/identity/
│   ├── service/
│   │   ├── TokenServiceTest.java                        ← JWT 服务测试
│   │   ├── AuthServiceTest.java                         ← 认证服务测试
│   │   ├── UserServiceTest.java                         ← 用户管理测试
│   │   └── ApiTokenServiceTest.java                     ← API Token 服务测试
│   └── entrance/
│       └── controller/
│           ├── AuthControllerTest.java                  ← 认证 API 测试
│           └── UserControllerTest.java                  ← 用户管理 API 测试
└── src/test/resources/
    ├── application-test.yml                             ← 测试配置
    └── db/test-migration/
        ├── V1__init_identity_tables.sql                 ← H2 兼容建表
        └── V2__seed_test_data.sql                       ← 测试种子数据
```

---

### Task 1: 补充依赖 + 公共基础设施 + 数据库建表

**Files:**
- Modify: `forge-identity/pom.xml`
- Create: `forge-identity/src/main/java/com/shulex/forge/identity/common/Result.java`
- Create: `forge-identity/src/main/java/com/shulex/forge/identity/common/ErrorCode.java`
- Create: `forge-identity/src/main/java/com/shulex/forge/identity/common/BizException.java`
- Create: `forge-identity/src/main/java/com/shulex/forge/identity/common/SysException.java`
- Create: `forge-identity/src/main/java/com/shulex/forge/identity/common/GlobalExceptionHandler.java`
- Create: `forge-identity/src/main/resources/db/migration/V1__init_identity_tables.sql`

- [ ] **Step 1: 更新 pom.xml 添加依赖**

```xml
<!-- 在 <dependencies> 中追加 -->
<!-- Spring Security -->
<dependency>
    <groupId>org.springframework.boot</groupId>
    <artifactId>spring-boot-starter-security</artifactId>
</dependency>
<!-- JWT -->
<dependency>
    <groupId>io.jsonwebtoken</groupId>
    <artifactId>jjwt-api</artifactId>
    <version>0.12.6</version>
</dependency>
<dependency>
    <groupId>io.jsonwebtoken</groupId>
    <artifactId>jjwt-impl</artifactId>
    <version>0.12.6</version>
    <scope>runtime</scope>
</dependency>
<dependency>
    <groupId>io.jsonwebtoken</groupId>
    <artifactId>jjwt-jackson</artifactId>
    <version>0.12.6</version>
    <scope>runtime</scope>
</dependency>
<!-- Redis -->
<dependency>
    <groupId>org.springframework.boot</groupId>
    <artifactId>spring-boot-starter-data-redis</artifactId>
</dependency>
<!-- DB -->
<dependency>
    <groupId>com.baomidou</groupId>
    <artifactId>mybatis-plus-spring-boot3-starter</artifactId>
</dependency>
<dependency>
    <groupId>com.mysql</groupId>
    <artifactId>mysql-connector-j</artifactId>
    <scope>runtime</scope>
</dependency>
<dependency>
    <groupId>org.flywaydb</groupId>
    <artifactId>flyway-core</artifactId>
</dependency>
<dependency>
    <groupId>org.flywaydb</groupId>
    <artifactId>flyway-mysql</artifactId>
</dependency>
<!-- Validation -->
<dependency>
    <groupId>org.springframework.boot</groupId>
    <artifactId>spring-boot-starter-validation</artifactId>
</dependency>
<!-- Test -->
<dependency>
    <groupId>com.h2database</groupId>
    <artifactId>h2</artifactId>
    <scope>test</scope>
</dependency>
<dependency>
    <groupId>org.springframework.security</groupId>
    <artifactId>spring-security-test</artifactId>
    <scope>test</scope>
</dependency>
```

Note: mybatis-plus, flyway, mysql-connector versions managed by parent pom. Spring Security, Redis, Validation versions managed by Spring Boot BOM.

- [ ] **Step 2: 创建 Result.java**

```java
package com.shulex.forge.identity.common;

import lombok.AllArgsConstructor;
import lombok.Data;
import lombok.NoArgsConstructor;

@Data
@NoArgsConstructor
@AllArgsConstructor
public class Result<T> {
    private int code;
    private String message;
    private T data;
    private long timestamp;

    public static <T> Result<T> ok(T data) {
        return new Result<>(0, "success", data, System.currentTimeMillis());
    }

    public static <T> Result<T> fail(int code, String message) {
        return new Result<>(code, message, null, System.currentTimeMillis());
    }
}
```

- [ ] **Step 3: 创建 ErrorCode.java**

```java
package com.shulex.forge.identity.common;

import lombok.Getter;
import lombok.AllArgsConstructor;

@Getter
@AllArgsConstructor
public enum ErrorCode {
    INVALID_CREDENTIALS(40100, "用户名或密码错误"),
    TOKEN_EXPIRED(40101, "Token 已过期"),
    TOKEN_INVALID(40102, "Token 无效"),
    TOKEN_REVOKED(40103, "Token 已吊销"),
    UNAUTHORIZED(40104, "未认证"),
    FORBIDDEN(40300, "无权限"),
    USER_NOT_FOUND(40400, "用户不存在"),
    USER_DISABLED(40301, "用户已禁用"),
    USER_EXISTS(40901, "用户名已存在"),
    TENANT_NOT_FOUND(40402, "租户不存在"),
    INVALID_PARAM(40000, "参数错误"),
    INTERNAL_ERROR(50000, "系统内部错误");

    private final int code;
    private final String message;
}
```

- [ ] **Step 4: 创建 BizException.java**

```java
package com.shulex.forge.identity.common;

import lombok.Getter;

@Getter
public class BizException extends RuntimeException {
    private final ErrorCode errorCode;

    public BizException(ErrorCode errorCode) {
        super(errorCode.getMessage());
        this.errorCode = errorCode;
    }

    public BizException(ErrorCode errorCode, String detail) {
        super(detail);
        this.errorCode = errorCode;
    }
}
```

- [ ] **Step 5: 创建 SysException.java**

```java
package com.shulex.forge.identity.common;

import lombok.Getter;

@Getter
public class SysException extends RuntimeException {
    private final ErrorCode errorCode;

    public SysException(ErrorCode errorCode, Throwable cause) {
        super(errorCode.getMessage(), cause);
        this.errorCode = errorCode;
    }
}
```

- [ ] **Step 6: 创建 GlobalExceptionHandler.java**

```java
package com.shulex.forge.identity.common;

import lombok.extern.slf4j.Slf4j;
import org.springframework.http.ResponseEntity;
import org.springframework.web.bind.MethodArgumentNotValidException;
import org.springframework.web.bind.annotation.ExceptionHandler;
import org.springframework.web.bind.annotation.RestControllerAdvice;

@Slf4j
@RestControllerAdvice
public class GlobalExceptionHandler {

    @ExceptionHandler(BizException.class)
    public ResponseEntity<Result<Void>> handleBiz(BizException e) {
        log.warn("业务异常: {}", e.getMessage());
        int httpStatus = e.getErrorCode().getCode() / 100;
        return ResponseEntity.status(httpStatus)
                .body(Result.fail(e.getErrorCode().getCode(), e.getMessage()));
    }

    @ExceptionHandler(MethodArgumentNotValidException.class)
    public ResponseEntity<Result<Void>> handleValidation(MethodArgumentNotValidException e) {
        String message = e.getBindingResult().getFieldErrors().stream()
                .map(f -> f.getField() + " " + f.getDefaultMessage())
                .reduce((a, b) -> a + "; " + b)
                .orElse("参数错误");
        log.warn("参数校验失败: {}", message);
        return ResponseEntity.badRequest()
                .body(Result.fail(ErrorCode.INVALID_PARAM.getCode(), message));
    }

    @ExceptionHandler(SysException.class)
    public ResponseEntity<Result<Void>> handleSys(SysException e) {
        log.error("系统异常: {}", e.getMessage(), e.getCause());
        return ResponseEntity.internalServerError()
                .body(Result.fail(e.getErrorCode().getCode(), e.getMessage()));
    }

    @ExceptionHandler(Exception.class)
    public ResponseEntity<Result<Void>> handleUnknown(Exception e) {
        log.error("系统异常", e);
        return ResponseEntity.internalServerError()
                .body(Result.fail(ErrorCode.INTERNAL_ERROR.getCode(), ErrorCode.INTERNAL_ERROR.getMessage()));
    }
}
```

- [ ] **Step 7: 创建 V1__init_identity_tables.sql**

```sql
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
(1, 'admin', '$2a$10$N.zmdr9k7uOCQb376NoUnuTJ8iAt6Z5EHsM8lE9lBOsl7iKTVKIUi', '系统管理员', 1);

-- 种子数据: 管理员角色绑定
INSERT INTO identity_user_role (user_id, role_id) VALUES (1, 1);
```

- [ ] **Step 8: 编译验证**

Run: `cd forge-identity && mvn clean compile -q`
Expected: 编译通过

- [ ] **Step 9: Commit**

```bash
git add forge-identity/pom.xml forge-identity/src/main/java/com/shulex/forge/identity/common/ forge-identity/src/main/resources/db/
git commit -m "feat(m3): add forge-identity dependencies, common infrastructure, and schema"
```

---

### Task 2: 实体层 + Mapper + MyBatis 配置

**Files:**
- Create: `forge-identity/src/main/java/com/shulex/forge/identity/infrastructure/entity/TenantDO.java`
- Create: `forge-identity/src/main/java/com/shulex/forge/identity/infrastructure/entity/UserDO.java`
- Create: `forge-identity/src/main/java/com/shulex/forge/identity/infrastructure/entity/RoleDO.java`
- Create: `forge-identity/src/main/java/com/shulex/forge/identity/infrastructure/entity/UserRoleDO.java`
- Create: `forge-identity/src/main/java/com/shulex/forge/identity/infrastructure/entity/ApiTokenDO.java`
- Create: `forge-identity/src/main/java/com/shulex/forge/identity/infrastructure/mapper/TenantMapper.java`
- Create: `forge-identity/src/main/java/com/shulex/forge/identity/infrastructure/mapper/UserMapper.java`
- Create: `forge-identity/src/main/java/com/shulex/forge/identity/infrastructure/mapper/RoleMapper.java`
- Create: `forge-identity/src/main/java/com/shulex/forge/identity/infrastructure/mapper/UserRoleMapper.java`
- Create: `forge-identity/src/main/java/com/shulex/forge/identity/infrastructure/mapper/ApiTokenMapper.java`
- Create: `forge-identity/src/main/java/com/shulex/forge/identity/infrastructure/config/MyBatisPlusConfig.java`
- Create: `forge-identity/src/test/resources/application-test.yml`
- Create: `forge-identity/src/test/resources/db/test-migration/V1__init_identity_tables.sql`
- Create: `forge-identity/src/test/resources/db/test-migration/V2__seed_test_data.sql`

- [ ] **Step 1: 创建 TenantDO.java**

```java
package com.shulex.forge.identity.infrastructure.entity;

import com.baomidou.mybatisplus.annotation.*;
import lombok.Data;
import java.time.LocalDateTime;

@Data
@TableName("identity_tenant")
public class TenantDO {
    @TableId(type = IdType.AUTO)
    private Long id;
    private String name;
    private Integer status;
    @TableField(fill = FieldFill.INSERT)
    private LocalDateTime gmtCreate;
    @TableField(fill = FieldFill.INSERT_UPDATE)
    private LocalDateTime gmtModified;
}
```

- [ ] **Step 2: 创建 UserDO.java**

```java
package com.shulex.forge.identity.infrastructure.entity;

import com.baomidou.mybatisplus.annotation.*;
import lombok.Data;
import java.time.LocalDateTime;

@Data
@TableName("identity_user")
public class UserDO {
    @TableId(type = IdType.AUTO)
    private Long id;
    private Long tenantId;
    private String username;
    private String passwordHash;
    private String email;
    private String nickname;
    private Integer status;
    @TableField(fill = FieldFill.INSERT)
    private LocalDateTime gmtCreate;
    @TableField(fill = FieldFill.INSERT_UPDATE)
    private LocalDateTime gmtModified;
}
```

- [ ] **Step 3: 创建 RoleDO.java**

```java
package com.shulex.forge.identity.infrastructure.entity;

import com.baomidou.mybatisplus.annotation.*;
import lombok.Data;
import java.time.LocalDateTime;

@Data
@TableName("identity_role")
public class RoleDO {
    @TableId(type = IdType.AUTO)
    private Long id;
    private Long tenantId;
    private String roleCode;
    private String roleName;
    @TableField(fill = FieldFill.INSERT)
    private LocalDateTime gmtCreate;
    @TableField(fill = FieldFill.INSERT_UPDATE)
    private LocalDateTime gmtModified;
}
```

- [ ] **Step 4: 创建 UserRoleDO.java**

```java
package com.shulex.forge.identity.infrastructure.entity;

import com.baomidou.mybatisplus.annotation.*;
import lombok.Data;
import java.time.LocalDateTime;

@Data
@TableName("identity_user_role")
public class UserRoleDO {
    @TableId(type = IdType.AUTO)
    private Long id;
    private Long userId;
    private Long roleId;
    @TableField(fill = FieldFill.INSERT)
    private LocalDateTime gmtCreate;
}
```

- [ ] **Step 5: 创建 ApiTokenDO.java**

```java
package com.shulex.forge.identity.infrastructure.entity;

import com.baomidou.mybatisplus.annotation.*;
import lombok.Data;
import java.time.LocalDateTime;

@Data
@TableName("identity_api_token")
public class ApiTokenDO {
    @TableId(type = IdType.AUTO)
    private Long id;
    private Long tenantId;
    private Long userId;
    private String tokenName;
    private String tokenHash;
    private String tokenPrefix;
    private LocalDateTime expiresAt;
    private Integer status;
    @TableField(fill = FieldFill.INSERT)
    private LocalDateTime gmtCreate;
    @TableField(fill = FieldFill.INSERT_UPDATE)
    private LocalDateTime gmtModified;
}
```

- [ ] **Step 6: 创建 Mapper 接口（5 个文件）**

```java
// TenantMapper.java
package com.shulex.forge.identity.infrastructure.mapper;

import com.baomidou.mybatisplus.core.mapper.BaseMapper;
import com.shulex.forge.identity.infrastructure.entity.TenantDO;
import org.apache.ibatis.annotations.Mapper;

@Mapper
public interface TenantMapper extends BaseMapper<TenantDO> {}
```

```java
// UserMapper.java
package com.shulex.forge.identity.infrastructure.mapper;

import com.baomidou.mybatisplus.core.mapper.BaseMapper;
import com.shulex.forge.identity.infrastructure.entity.UserDO;
import org.apache.ibatis.annotations.Mapper;

@Mapper
public interface UserMapper extends BaseMapper<UserDO> {}
```

```java
// RoleMapper.java
package com.shulex.forge.identity.infrastructure.mapper;

import com.baomidou.mybatisplus.core.mapper.BaseMapper;
import com.shulex.forge.identity.infrastructure.entity.RoleDO;
import org.apache.ibatis.annotations.Mapper;

@Mapper
public interface RoleMapper extends BaseMapper<RoleDO> {}
```

```java
// UserRoleMapper.java
package com.shulex.forge.identity.infrastructure.mapper;

import com.baomidou.mybatisplus.core.mapper.BaseMapper;
import com.shulex.forge.identity.infrastructure.entity.UserRoleDO;
import org.apache.ibatis.annotations.Mapper;

@Mapper
public interface UserRoleMapper extends BaseMapper<UserRoleDO> {}
```

```java
// ApiTokenMapper.java
package com.shulex.forge.identity.infrastructure.mapper;

import com.baomidou.mybatisplus.core.mapper.BaseMapper;
import com.shulex.forge.identity.infrastructure.entity.ApiTokenDO;
import org.apache.ibatis.annotations.Mapper;

@Mapper
public interface ApiTokenMapper extends BaseMapper<ApiTokenDO> {}
```

- [ ] **Step 7: 创建 MyBatisPlusConfig.java**

```java
package com.shulex.forge.identity.infrastructure.config;

import com.baomidou.mybatisplus.core.handlers.MetaObjectHandler;
import lombok.extern.slf4j.Slf4j;
import org.apache.ibatis.reflection.MetaObject;
import org.springframework.stereotype.Component;

import java.time.LocalDateTime;

@Slf4j
@Component
public class MyBatisPlusConfig implements MetaObjectHandler {

    @Override
    public void insertFill(MetaObject metaObject) {
        this.strictInsertFill(metaObject, "gmtCreate", LocalDateTime::now, LocalDateTime.class);
        this.strictInsertFill(metaObject, "gmtModified", LocalDateTime::now, LocalDateTime.class);
    }

    @Override
    public void updateFill(MetaObject metaObject) {
        this.strictUpdateFill(metaObject, "gmtModified", LocalDateTime::now, LocalDateTime.class);
    }
}
```

- [ ] **Step 8: 创建 application-test.yml**

```yaml
spring:
  datasource:
    url: jdbc:h2:mem:forge_identity_test;MODE=MYSQL;DB_CLOSE_DELAY=-1
    driver-class-name: org.h2.Driver
    username: sa
    password:
  flyway:
    enabled: true
    locations: classpath:db/test-migration
  data:
    redis:
      host: localhost
      port: 6379

forge:
  jwt:
    secret: test-secret-key-for-unit-tests-must-be-at-least-256-bits-long-enough
    access-token-expire-minutes: 30
    refresh-token-expire-minutes: 10080
```

- [ ] **Step 9: 创建 H2 兼容建表 test-migration/V1__init_identity_tables.sql**

```sql
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
```

- [ ] **Step 10: 创建 test-migration/V2__seed_test_data.sql**

```sql
-- 测试租户
INSERT INTO identity_tenant (name, status) VALUES ('test-tenant', 1);

-- 测试角色
INSERT INTO identity_role (tenant_id, role_code, role_name) VALUES
(1, 'ADMIN', '管理员'),
(1, 'USER', '普通用户');

-- 测试管理员 (密码: admin123)
INSERT INTO identity_user (tenant_id, username, password_hash, nickname, status) VALUES
(1, 'admin', '$2a$10$N.zmdr9k7uOCQb376NoUnuTJ8iAt6Z5EHsM8lE9lBOsl7iKTVKIUi', '测试管理员', 1);

-- 测试普通用户 (密码: user123)
INSERT INTO identity_user (tenant_id, username, password_hash, nickname, status) VALUES
(1, 'testuser', '$2a$10$dXJ3SW6G7P50lGmMQoeVhOOoMZCFe0AvhldPOzdQQJXpSGgR4k5Ge', '测试用户', 1);

-- 角色绑定
INSERT INTO identity_user_role (user_id, role_id) VALUES (1, 1);
INSERT INTO identity_user_role (user_id, role_id) VALUES (2, 2);
```

- [ ] **Step 11: 编译验证**

Run: `cd forge-identity && mvn clean compile -q`
Expected: 编译通过

- [ ] **Step 12: Commit**

```bash
git add forge-identity/src/
git commit -m "feat(m3): add entity layer, mappers, and database migrations"
```

---

### Task 3: TokenService（JWT 签发/验证/吊销）+ 测试

**Files:**
- Create: `forge-identity/src/main/java/com/shulex/forge/identity/service/TokenService.java`
- Create: `forge-identity/src/test/java/com/shulex/forge/identity/service/TokenServiceTest.java`

- [ ] **Step 1: 写 TokenService 失败测试**

```java
package com.shulex.forge.identity.service;

import com.shulex.forge.identity.common.BizException;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;
import org.springframework.data.redis.core.StringRedisTemplate;
import org.springframework.data.redis.core.ValueOperations;

import java.util.List;

import static org.assertj.core.api.Assertions.assertThat;
import static org.assertj.core.api.Assertions.assertThatThrownBy;
import static org.mockito.ArgumentMatchers.*;
import static org.mockito.Mockito.*;

class TokenServiceTest {

    private TokenService tokenService;
    private StringRedisTemplate redisTemplate;
    private ValueOperations<String, String> valueOps;

    @BeforeEach
    void setUp() {
        redisTemplate = mock(StringRedisTemplate.class);
        valueOps = mock(ValueOperations.class);
        when(redisTemplate.opsForValue()).thenReturn(valueOps);
        tokenService = new TokenService(
                "test-secret-key-for-unit-tests-must-be-at-least-256-bits-long-enough",
                30, 10080, redisTemplate);
    }

    @Test
    void generateAccessToken_containsExpectedClaims() {
        String token = tokenService.generateAccessToken(1L, 100L, "admin", List.of("ADMIN"));
        var claims = tokenService.parseToken(token);
        assertThat(claims.get("userId", Long.class)).isEqualTo(1L);
        assertThat(claims.get("tenantId", Long.class)).isEqualTo(100L);
        assertThat(claims.getSubject()).isEqualTo("admin");
    }

    @Test
    void generateRefreshToken_isValid() {
        String token = tokenService.generateRefreshToken(1L, 100L, "admin");
        var claims = tokenService.parseToken(token);
        assertThat(claims.get("type", String.class)).isEqualTo("refresh");
    }

    @Test
    void revokeToken_addsToBlacklist() {
        String token = tokenService.generateAccessToken(1L, 100L, "admin", List.of("ADMIN"));
        tokenService.revokeToken(token);
        verify(valueOps).set(startsWith("forge:token:blacklist:"), eq("revoked"), any());
    }

    @Test
    void validateToken_throwsWhenRevoked() {
        when(redisTemplate.hasKey(anyString())).thenReturn(true);
        String token = tokenService.generateAccessToken(1L, 100L, "admin", List.of("ADMIN"));
        assertThatThrownBy(() -> tokenService.validateToken(token))
                .isInstanceOf(BizException.class);
    }

    @Test
    void parseToken_throwsOnInvalidToken() {
        assertThatThrownBy(() -> tokenService.parseToken("invalid.token.here"))
                .isInstanceOf(BizException.class);
    }
}
```

- [ ] **Step 2: 运行测试确认失败**

Run: `cd forge-identity && mvn test -Dtest=TokenServiceTest -pl . 2>&1 | tail -20`
Expected: 编译失败 — TokenService 不存在

- [ ] **Step 3: 实现 TokenService**

```java
package com.shulex.forge.identity.service;

import com.shulex.forge.identity.common.BizException;
import com.shulex.forge.identity.common.ErrorCode;
import io.jsonwebtoken.*;
import io.jsonwebtoken.security.Keys;
import lombok.extern.slf4j.Slf4j;
import org.springframework.beans.factory.annotation.Value;
import org.springframework.data.redis.core.StringRedisTemplate;
import org.springframework.stereotype.Service;

import javax.crypto.SecretKey;
import java.nio.charset.StandardCharsets;
import java.time.Duration;
import java.util.Date;
import java.util.List;
import java.util.UUID;

@Slf4j
@Service
public class TokenService {

    private final SecretKey secretKey;
    private final long accessTokenExpireMinutes;
    private final long refreshTokenExpireMinutes;
    private final StringRedisTemplate redisTemplate;
    private static final String BLACKLIST_PREFIX = "forge:token:blacklist:";

    public TokenService(
            @Value("${forge.jwt.secret}") String secret,
            @Value("${forge.jwt.access-token-expire-minutes}") long accessTokenExpireMinutes,
            @Value("${forge.jwt.refresh-token-expire-minutes}") long refreshTokenExpireMinutes,
            StringRedisTemplate redisTemplate) {
        this.secretKey = Keys.hmacShaKeyFor(secret.getBytes(StandardCharsets.UTF_8));
        this.accessTokenExpireMinutes = accessTokenExpireMinutes;
        this.refreshTokenExpireMinutes = refreshTokenExpireMinutes;
        this.redisTemplate = redisTemplate;
    }

    public String generateAccessToken(Long userId, Long tenantId, String username, List<String> roles) {
        Date now = new Date();
        Date expiry = new Date(now.getTime() + accessTokenExpireMinutes * 60 * 1000);
        return Jwts.builder()
                .id(UUID.randomUUID().toString())
                .subject(username)
                .claim("userId", userId)
                .claim("tenantId", tenantId)
                .claim("roles", roles)
                .claim("type", "access")
                .issuedAt(now)
                .expiration(expiry)
                .signWith(secretKey)
                .compact();
    }

    public String generateRefreshToken(Long userId, Long tenantId, String username) {
        Date now = new Date();
        Date expiry = new Date(now.getTime() + refreshTokenExpireMinutes * 60 * 1000);
        return Jwts.builder()
                .id(UUID.randomUUID().toString())
                .subject(username)
                .claim("userId", userId)
                .claim("tenantId", tenantId)
                .claim("type", "refresh")
                .issuedAt(now)
                .expiration(expiry)
                .signWith(secretKey)
                .compact();
    }

    public Claims parseToken(String token) {
        try {
            return Jwts.parser()
                    .verifyWith(secretKey)
                    .build()
                    .parseSignedClaims(token)
                    .getPayload();
        } catch (ExpiredJwtException e) {
            throw new BizException(ErrorCode.TOKEN_EXPIRED);
        } catch (JwtException e) {
            throw new BizException(ErrorCode.TOKEN_INVALID);
        }
    }

    public Claims validateToken(String token) {
        Claims claims = parseToken(token);
        String jti = claims.getId();
        String blacklistKey = BLACKLIST_PREFIX + (jti != null ? jti : token.hashCode());
        if (Boolean.TRUE.equals(redisTemplate.hasKey(blacklistKey))) {
            throw new BizException(ErrorCode.TOKEN_REVOKED);
        }
        return claims;
    }

    public void revokeToken(String token) {
        try {
            Claims claims = parseToken(token);
            String jti = claims.getId();
            String blacklistKey = BLACKLIST_PREFIX + (jti != null ? jti : token.hashCode());
            long ttlMs = claims.getExpiration().getTime() - System.currentTimeMillis();
            if (ttlMs > 0) {
                redisTemplate.opsForValue().set(blacklistKey, "revoked", Duration.ofMillis(ttlMs));
                log.info("Token 已吊销: user={}", claims.getSubject());
            }
        } catch (BizException e) {
            log.debug("吊销已过期 Token，忽略: {}", e.getMessage());
        }
    }
}
```

- [ ] **Step 4: 运行测试确认通过**

Run: `cd forge-identity && mvn test -Dtest=TokenServiceTest -pl . 2>&1 | tail -15`
Expected: 全部 PASS

- [ ] **Step 5: Commit**

```bash
git add forge-identity/src/
git commit -m "feat(m3): add JWT token service with issue, validate, and revoke"
```

---

### Task 4: AuthService（登录/登出/刷新）+ UserService + 测试

**Files:**
- Create: `forge-identity/src/main/java/com/shulex/forge/identity/service/AuthService.java`
- Create: `forge-identity/src/main/java/com/shulex/forge/identity/service/UserService.java`
- Create: `forge-identity/src/test/java/com/shulex/forge/identity/service/AuthServiceTest.java`
- Create: `forge-identity/src/test/java/com/shulex/forge/identity/service/UserServiceTest.java`

- [ ] **Step 1: 写 AuthService 失败测试**

```java
package com.shulex.forge.identity.service;

import com.shulex.forge.identity.common.BizException;
import com.shulex.forge.identity.infrastructure.entity.RoleDO;
import com.shulex.forge.identity.infrastructure.entity.UserDO;
import com.shulex.forge.identity.infrastructure.entity.UserRoleDO;
import com.shulex.forge.identity.infrastructure.mapper.RoleMapper;
import com.shulex.forge.identity.infrastructure.mapper.UserMapper;
import com.shulex.forge.identity.infrastructure.mapper.UserRoleMapper;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;
import org.springframework.security.crypto.bcrypt.BCryptPasswordEncoder;
import org.springframework.security.crypto.password.PasswordEncoder;

import java.util.List;

import static org.assertj.core.api.Assertions.assertThat;
import static org.assertj.core.api.Assertions.assertThatThrownBy;
import static org.mockito.ArgumentMatchers.any;
import static org.mockito.Mockito.*;

class AuthServiceTest {

    private AuthService authService;
    private UserMapper userMapper;
    private UserRoleMapper userRoleMapper;
    private RoleMapper roleMapper;
    private TokenService tokenService;
    private PasswordEncoder passwordEncoder;

    @BeforeEach
    void setUp() {
        userMapper = mock(UserMapper.class);
        userRoleMapper = mock(UserRoleMapper.class);
        roleMapper = mock(RoleMapper.class);
        tokenService = mock(TokenService.class);
        passwordEncoder = new BCryptPasswordEncoder();
        authService = new AuthService(userMapper, userRoleMapper, roleMapper, tokenService, passwordEncoder);
    }

    @Test
    void login_returnsTokensOnSuccess() {
        UserDO user = new UserDO();
        user.setId(1L);
        user.setTenantId(100L);
        user.setUsername("admin");
        user.setPasswordHash(passwordEncoder.encode("admin123"));
        user.setStatus(1);

        when(userMapper.selectOne(any())).thenReturn(user);

        UserRoleDO ur = new UserRoleDO();
        ur.setRoleId(1L);
        when(userRoleMapper.selectList(any())).thenReturn(List.of(ur));

        RoleDO role = new RoleDO();
        role.setRoleCode("ADMIN");
        when(roleMapper.selectById(1L)).thenReturn(role);

        when(tokenService.generateAccessToken(eq(1L), eq(100L), eq("admin"), any()))
                .thenReturn("access-token");
        when(tokenService.generateRefreshToken(1L, 100L, "admin"))
                .thenReturn("refresh-token");

        var result = authService.login(100L, "admin", "admin123");
        assertThat(result.getAccessToken()).isEqualTo("access-token");
        assertThat(result.getRefreshToken()).isEqualTo("refresh-token");
    }

    @Test
    void login_throwsOnWrongPassword() {
        UserDO user = new UserDO();
        user.setPasswordHash(passwordEncoder.encode("admin123"));
        user.setStatus(1);
        when(userMapper.selectOne(any())).thenReturn(user);

        assertThatThrownBy(() -> authService.login(100L, "admin", "wrong"))
                .isInstanceOf(BizException.class);
    }

    @Test
    void login_throwsOnUserNotFound() {
        when(userMapper.selectOne(any())).thenReturn(null);

        assertThatThrownBy(() -> authService.login(100L, "nobody", "pass"))
                .isInstanceOf(BizException.class);
    }

    @Test
    void login_throwsOnDisabledUser() {
        UserDO user = new UserDO();
        user.setStatus(0);
        when(userMapper.selectOne(any())).thenReturn(user);

        assertThatThrownBy(() -> authService.login(100L, "admin", "admin123"))
                .isInstanceOf(BizException.class);
    }
}
```

- [ ] **Step 2: 运行测试确认失败**

Run: `cd forge-identity && mvn test -Dtest=AuthServiceTest -pl . 2>&1 | tail -20`
Expected: 编译失败 — AuthService 不存在

- [ ] **Step 3: 创建 LoginResponse VO**

```java
package com.shulex.forge.identity.entrance.vo;

import lombok.Builder;
import lombok.Data;

@Data
@Builder
public class LoginResponse {
    private String accessToken;
    private String refreshToken;
    private Long userId;
    private String username;
    private java.util.List<String> roles;
}
```

- [ ] **Step 4: 实现 AuthService**

```java
package com.shulex.forge.identity.service;

import com.baomidou.mybatisplus.core.conditions.query.LambdaQueryWrapper;
import com.shulex.forge.identity.common.BizException;
import com.shulex.forge.identity.common.ErrorCode;
import com.shulex.forge.identity.entrance.vo.LoginResponse;
import com.shulex.forge.identity.infrastructure.entity.RoleDO;
import com.shulex.forge.identity.infrastructure.entity.UserDO;
import com.shulex.forge.identity.infrastructure.entity.UserRoleDO;
import com.shulex.forge.identity.infrastructure.mapper.RoleMapper;
import com.shulex.forge.identity.infrastructure.mapper.UserMapper;
import com.shulex.forge.identity.infrastructure.mapper.UserRoleMapper;
import lombok.extern.slf4j.Slf4j;
import org.springframework.security.crypto.password.PasswordEncoder;
import org.springframework.stereotype.Service;

import java.util.List;

@Slf4j
@Service
public class AuthService {

    private final UserMapper userMapper;
    private final UserRoleMapper userRoleMapper;
    private final RoleMapper roleMapper;
    private final TokenService tokenService;
    private final PasswordEncoder passwordEncoder;

    public AuthService(UserMapper userMapper, UserRoleMapper userRoleMapper,
                       RoleMapper roleMapper, TokenService tokenService,
                       PasswordEncoder passwordEncoder) {
        this.userMapper = userMapper;
        this.userRoleMapper = userRoleMapper;
        this.roleMapper = roleMapper;
        this.tokenService = tokenService;
        this.passwordEncoder = passwordEncoder;
    }

    public LoginResponse login(Long tenantId, String username, String password) {
        UserDO user = userMapper.selectOne(new LambdaQueryWrapper<UserDO>()
                .eq(UserDO::getTenantId, tenantId)
                .eq(UserDO::getUsername, username));
        if (user == null) {
            throw new BizException(ErrorCode.INVALID_CREDENTIALS);
        }
        if (user.getStatus() == 0) {
            throw new BizException(ErrorCode.USER_DISABLED);
        }
        if (!passwordEncoder.matches(password, user.getPasswordHash())) {
            throw new BizException(ErrorCode.INVALID_CREDENTIALS);
        }

        List<String> roles = getUserRoles(user.getId());
        String accessToken = tokenService.generateAccessToken(user.getId(), tenantId, username, roles);
        String refreshToken = tokenService.generateRefreshToken(user.getId(), tenantId, username);

        log.info("用户登录成功: tenant={}, user={}", tenantId, username);
        return LoginResponse.builder()
                .accessToken(accessToken)
                .refreshToken(refreshToken)
                .userId(user.getId())
                .username(username)
                .roles(roles)
                .build();
    }

    public LoginResponse refresh(String refreshToken) {
        var claims = tokenService.validateToken(refreshToken);
        if (!"refresh".equals(claims.get("type", String.class))) {
            throw new BizException(ErrorCode.TOKEN_INVALID, "非 Refresh Token");
        }
        Long userId = claims.get("userId", Long.class);
        Long tenantId = claims.get("tenantId", Long.class);
        String username = claims.getSubject();

        List<String> roles = getUserRoles(userId);
        String newAccessToken = tokenService.generateAccessToken(userId, tenantId, username, roles);
        String newRefreshToken = tokenService.generateRefreshToken(userId, tenantId, username);

        tokenService.revokeToken(refreshToken);

        return LoginResponse.builder()
                .accessToken(newAccessToken)
                .refreshToken(newRefreshToken)
                .userId(userId)
                .username(username)
                .roles(roles)
                .build();
    }

    public void logout(String accessToken) {
        tokenService.revokeToken(accessToken);
    }

    private List<String> getUserRoles(Long userId) {
        List<UserRoleDO> userRoles = userRoleMapper.selectList(
                new LambdaQueryWrapper<UserRoleDO>().eq(UserRoleDO::getUserId, userId));
        return userRoles.stream()
                .map(ur -> roleMapper.selectById(ur.getRoleId()))
                .map(RoleDO::getRoleCode)
                .toList();
    }
}
```

- [ ] **Step 5: 运行测试确认通过**

Run: `cd forge-identity && mvn test -Dtest=AuthServiceTest -pl . 2>&1 | tail -15`
Expected: 全部 PASS

- [ ] **Step 6: 写 UserService 失败测试**

```java
package com.shulex.forge.identity.service;

import com.shulex.forge.identity.common.BizException;
import com.shulex.forge.identity.infrastructure.entity.RoleDO;
import com.shulex.forge.identity.infrastructure.entity.UserDO;
import com.shulex.forge.identity.infrastructure.entity.UserRoleDO;
import com.shulex.forge.identity.infrastructure.mapper.RoleMapper;
import com.shulex.forge.identity.infrastructure.mapper.UserMapper;
import com.shulex.forge.identity.infrastructure.mapper.UserRoleMapper;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;
import org.springframework.security.crypto.bcrypt.BCryptPasswordEncoder;

import static org.assertj.core.api.Assertions.assertThat;
import static org.assertj.core.api.Assertions.assertThatThrownBy;
import static org.mockito.ArgumentMatchers.any;
import static org.mockito.Mockito.*;

class UserServiceTest {

    private UserService userService;
    private UserMapper userMapper;
    private UserRoleMapper userRoleMapper;
    private RoleMapper roleMapper;

    @BeforeEach
    void setUp() {
        userMapper = mock(UserMapper.class);
        userRoleMapper = mock(UserRoleMapper.class);
        roleMapper = mock(RoleMapper.class);
        userService = new UserService(userMapper, userRoleMapper, roleMapper, new BCryptPasswordEncoder());
    }

    @Test
    void createUser_insertsUserAndAssignsRole() {
        when(userMapper.selectOne(any())).thenReturn(null);
        when(userMapper.insert(any())).thenReturn(1);

        RoleDO role = new RoleDO();
        role.setId(2L);
        when(roleMapper.selectOne(any())).thenReturn(role);
        when(userRoleMapper.insert(any())).thenReturn(1);

        UserDO created = userService.createUser(100L, "newuser", "password123", "New User", "ADMIN");
        assertThat(created.getUsername()).isEqualTo("newuser");
        verify(userMapper).insert(any());
        verify(userRoleMapper).insert(any());
    }

    @Test
    void createUser_throwsOnDuplicate() {
        when(userMapper.selectOne(any())).thenReturn(new UserDO());

        assertThatThrownBy(() -> userService.createUser(100L, "existing", "pass", "name", "USER"))
                .isInstanceOf(BizException.class);
    }

    @Test
    void getUserById_returnsUser() {
        UserDO user = new UserDO();
        user.setId(1L);
        user.setUsername("admin");
        when(userMapper.selectById(1L)).thenReturn(user);

        UserDO result = userService.getUserById(1L);
        assertThat(result.getUsername()).isEqualTo("admin");
    }

    @Test
    void getUserById_throwsOnNotFound() {
        when(userMapper.selectById(999L)).thenReturn(null);
        assertThatThrownBy(() -> userService.getUserById(999L))
                .isInstanceOf(BizException.class);
    }
}
```

- [ ] **Step 7: 实现 UserService**

```java
package com.shulex.forge.identity.service;

import com.baomidou.mybatisplus.core.conditions.query.LambdaQueryWrapper;
import com.shulex.forge.identity.common.BizException;
import com.shulex.forge.identity.common.ErrorCode;
import com.shulex.forge.identity.infrastructure.entity.RoleDO;
import com.shulex.forge.identity.infrastructure.entity.UserDO;
import com.shulex.forge.identity.infrastructure.entity.UserRoleDO;
import com.shulex.forge.identity.infrastructure.mapper.RoleMapper;
import com.shulex.forge.identity.infrastructure.mapper.UserMapper;
import com.shulex.forge.identity.infrastructure.mapper.UserRoleMapper;
import lombok.extern.slf4j.Slf4j;
import org.springframework.security.crypto.password.PasswordEncoder;
import org.springframework.stereotype.Service;

import java.util.List;

@Slf4j
@Service
public class UserService {

    private final UserMapper userMapper;
    private final UserRoleMapper userRoleMapper;
    private final RoleMapper roleMapper;
    private final PasswordEncoder passwordEncoder;

    public UserService(UserMapper userMapper, UserRoleMapper userRoleMapper,
                       RoleMapper roleMapper, PasswordEncoder passwordEncoder) {
        this.userMapper = userMapper;
        this.userRoleMapper = userRoleMapper;
        this.roleMapper = roleMapper;
        this.passwordEncoder = passwordEncoder;
    }

    public UserDO createUser(Long tenantId, String username, String password, String nickname, String roleCode) {
        UserDO existing = userMapper.selectOne(new LambdaQueryWrapper<UserDO>()
                .eq(UserDO::getTenantId, tenantId)
                .eq(UserDO::getUsername, username));
        if (existing != null) {
            throw new BizException(ErrorCode.USER_EXISTS);
        }

        UserDO user = new UserDO();
        user.setTenantId(tenantId);
        user.setUsername(username);
        user.setPasswordHash(passwordEncoder.encode(password));
        user.setNickname(nickname);
        user.setStatus(1);
        userMapper.insert(user);

        if (roleCode != null) {
            RoleDO role = roleMapper.selectOne(new LambdaQueryWrapper<RoleDO>()
                    .eq(RoleDO::getTenantId, tenantId)
                    .eq(RoleDO::getRoleCode, roleCode));
            if (role != null) {
                UserRoleDO userRole = new UserRoleDO();
                userRole.setUserId(user.getId());
                userRole.setRoleId(role.getId());
                userRoleMapper.insert(userRole);
            }
        }

        log.info("创建用户: tenant={}, user={}, role={}", tenantId, username, roleCode);
        return user;
    }

    public UserDO getUserById(Long userId) {
        UserDO user = userMapper.selectById(userId);
        if (user == null) {
            throw new BizException(ErrorCode.USER_NOT_FOUND);
        }
        return user;
    }

    public List<UserDO> listUsers(Long tenantId) {
        return userMapper.selectList(new LambdaQueryWrapper<UserDO>()
                .eq(UserDO::getTenantId, tenantId)
                .orderByAsc(UserDO::getId));
    }
}
```

- [ ] **Step 8: 运行全部服务层测试**

Run: `cd forge-identity && mvn test -Dtest=AuthServiceTest,UserServiceTest,TokenServiceTest -pl . 2>&1 | tail -15`
Expected: 全部 PASS

- [ ] **Step 9: Commit**

```bash
git add forge-identity/src/
git commit -m "feat(m3): add auth service, user service, and login/logout logic"
```

---

### Task 5: Spring Security 配置 + JWT 过滤器

**Files:**
- Create: `forge-identity/src/main/java/com/shulex/forge/identity/infrastructure/config/SecurityConfig.java`
- Create: `forge-identity/src/main/java/com/shulex/forge/identity/entrance/filter/JwtAuthFilter.java`

- [ ] **Step 1: 创建 JwtAuthFilter.java**

```java
package com.shulex.forge.identity.entrance.filter;

import com.shulex.forge.identity.common.BizException;
import com.shulex.forge.identity.service.TokenService;
import io.jsonwebtoken.Claims;
import jakarta.servlet.FilterChain;
import jakarta.servlet.ServletException;
import jakarta.servlet.http.HttpServletRequest;
import jakarta.servlet.http.HttpServletResponse;
import lombok.extern.slf4j.Slf4j;
import org.springframework.security.authentication.UsernamePasswordAuthenticationToken;
import org.springframework.security.core.authority.SimpleGrantedAuthority;
import org.springframework.security.core.context.SecurityContextHolder;
import org.springframework.web.filter.OncePerRequestFilter;

import java.io.IOException;
import java.util.List;

@Slf4j
public class JwtAuthFilter extends OncePerRequestFilter {

    private final TokenService tokenService;

    public JwtAuthFilter(TokenService tokenService) {
        this.tokenService = tokenService;
    }

    @Override
    protected void doFilterInternal(HttpServletRequest request, HttpServletResponse response,
                                     FilterChain filterChain) throws ServletException, IOException {
        String header = request.getHeader("Authorization");
        if (header != null && header.startsWith("Bearer ")) {
            String token = header.substring(7);
            try {
                Claims claims = tokenService.validateToken(token);
                String username = claims.getSubject();
                @SuppressWarnings("unchecked")
                List<String> roles = claims.get("roles", List.class);
                List<SimpleGrantedAuthority> authorities = roles != null
                        ? roles.stream().map(r -> new SimpleGrantedAuthority("ROLE_" + r)).toList()
                        : List.of();

                var auth = new UsernamePasswordAuthenticationToken(username, null, authorities);
                auth.setDetails(claims);
                SecurityContextHolder.getContext().setAuthentication(auth);
            } catch (BizException e) {
                log.debug("Token 验证失败: {}", e.getMessage());
            }
        }
        filterChain.doFilter(request, response);
    }
}
```

- [ ] **Step 2: 创建 SecurityConfig.java**

```java
package com.shulex.forge.identity.infrastructure.config;

import com.shulex.forge.identity.entrance.filter.JwtAuthFilter;
import com.shulex.forge.identity.service.TokenService;
import org.springframework.context.annotation.Bean;
import org.springframework.context.annotation.Configuration;
import org.springframework.security.config.annotation.web.builders.HttpSecurity;
import org.springframework.security.config.annotation.web.configuration.EnableWebSecurity;
import org.springframework.security.config.http.SessionCreationPolicy;
import org.springframework.security.crypto.bcrypt.BCryptPasswordEncoder;
import org.springframework.security.crypto.password.PasswordEncoder;
import org.springframework.security.web.SecurityFilterChain;
import org.springframework.security.web.authentication.UsernamePasswordAuthenticationFilter;

@Configuration
@EnableWebSecurity
public class SecurityConfig {

    @Bean
    public PasswordEncoder passwordEncoder() {
        return new BCryptPasswordEncoder();
    }

    @Bean
    public JwtAuthFilter jwtAuthFilter(TokenService tokenService) {
        return new JwtAuthFilter(tokenService);
    }

    @Bean
    public SecurityFilterChain filterChain(HttpSecurity http, JwtAuthFilter jwtAuthFilter) throws Exception {
        http
                .csrf(csrf -> csrf.disable())
                .sessionManagement(session -> session.sessionCreationPolicy(SessionCreationPolicy.STATELESS))
                .authorizeHttpRequests(auth -> auth
                        .requestMatchers("/api/auth/login", "/api/auth/refresh", "/api/auth/verify").permitAll()
                        .requestMatchers("/api/users/**").hasRole("ADMIN")
                        .anyRequest().authenticated()
                )
                .addFilterBefore(jwtAuthFilter, UsernamePasswordAuthenticationFilter.class);
        return http.build();
    }
}
```

- [ ] **Step 3: 编译验证**

Run: `cd forge-identity && mvn clean compile -q`
Expected: 编译通过

- [ ] **Step 4: Commit**

```bash
git add forge-identity/src/
git commit -m "feat(m3): add Spring Security config and JWT auth filter"
```

---

### Task 6: 认证 API（AuthController）+ 测试

**Files:**
- Create: `forge-identity/src/main/java/com/shulex/forge/identity/entrance/vo/LoginRequest.java`
- Create: `forge-identity/src/main/java/com/shulex/forge/identity/entrance/vo/RefreshRequest.java`
- Create: `forge-identity/src/main/java/com/shulex/forge/identity/entrance/controller/AuthController.java`
- Create: `forge-identity/src/test/java/com/shulex/forge/identity/entrance/controller/AuthControllerTest.java`

- [ ] **Step 1: 创建 LoginRequest.java**

```java
package com.shulex.forge.identity.entrance.vo;

import jakarta.validation.constraints.NotBlank;
import jakarta.validation.constraints.NotNull;
import lombok.Data;

@Data
public class LoginRequest {
    @NotNull
    private Long tenantId;
    @NotBlank
    private String username;
    @NotBlank
    private String password;
}
```

- [ ] **Step 2: 创建 RefreshRequest.java**

```java
package com.shulex.forge.identity.entrance.vo;

import jakarta.validation.constraints.NotBlank;
import lombok.Data;

@Data
public class RefreshRequest {
    @NotBlank
    private String refreshToken;
}
```

- [ ] **Step 3: 创建 AuthController.java**

```java
package com.shulex.forge.identity.entrance.controller;

import com.shulex.forge.identity.common.Result;
import com.shulex.forge.identity.entrance.vo.LoginRequest;
import com.shulex.forge.identity.entrance.vo.LoginResponse;
import com.shulex.forge.identity.entrance.vo.RefreshRequest;
import com.shulex.forge.identity.service.AuthService;
import com.shulex.forge.identity.service.TokenService;
import io.jsonwebtoken.Claims;
import jakarta.validation.Valid;
import org.springframework.http.ResponseEntity;
import org.springframework.web.bind.annotation.*;

import java.util.Map;

@RestController
@RequestMapping("/api/auth")
public class AuthController {

    private final AuthService authService;
    private final TokenService tokenService;

    public AuthController(AuthService authService, TokenService tokenService) {
        this.authService = authService;
        this.tokenService = tokenService;
    }

    @PostMapping("/login")
    public Result<LoginResponse> login(@Valid @RequestBody LoginRequest request) {
        LoginResponse response = authService.login(
                request.getTenantId(), request.getUsername(), request.getPassword());
        return Result.ok(response);
    }

    @PostMapping("/logout")
    public Result<Void> logout(@RequestHeader("Authorization") String authHeader) {
        String token = authHeader.replace("Bearer ", "");
        authService.logout(token);
        return Result.ok(null);
    }

    @PostMapping("/refresh")
    public Result<LoginResponse> refresh(@Valid @RequestBody RefreshRequest request) {
        LoginResponse response = authService.refresh(request.getRefreshToken());
        return Result.ok(response);
    }

    @GetMapping("/verify")
    public ResponseEntity<Map<String, Object>> verify(
            @RequestHeader(value = "Authorization", required = false) String authHeader) {
        if (authHeader == null || !authHeader.startsWith("Bearer ")) {
            return ResponseEntity.status(401).body(Map.of("authenticated", false));
        }
        try {
            String token = authHeader.substring(7);
            Claims claims = tokenService.validateToken(token);
            return ResponseEntity.ok()
                    .header("X-User-Id", String.valueOf(claims.get("userId")))
                    .header("X-Tenant-Id", String.valueOf(claims.get("tenantId")))
                    .header("X-Username", claims.getSubject())
                    .body(Map.of(
                            "authenticated", true,
                            "userId", claims.get("userId"),
                            "tenantId", claims.get("tenantId"),
                            "username", claims.getSubject(),
                            "roles", claims.get("roles")
                    ));
        } catch (Exception e) {
            return ResponseEntity.status(401).body(Map.of("authenticated", false));
        }
    }
}
```

- [ ] **Step 4: 写 AuthController 集成测试**

```java
package com.shulex.forge.identity.entrance.controller;

import com.fasterxml.jackson.databind.ObjectMapper;
import com.shulex.forge.identity.entrance.vo.LoginRequest;
import com.shulex.forge.identity.entrance.vo.LoginResponse;
import com.shulex.forge.identity.service.AuthService;
import com.shulex.forge.identity.service.TokenService;
import org.junit.jupiter.api.Test;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.boot.test.autoconfigure.web.servlet.AutoConfigureMockMvc;
import org.springframework.boot.test.context.SpringBootTest;
import org.springframework.boot.test.mock.mockbean.MockBean;
import org.springframework.data.redis.core.StringRedisTemplate;
import org.springframework.http.MediaType;
import org.springframework.test.context.ActiveProfiles;
import org.springframework.test.web.servlet.MockMvc;

import java.util.List;

import static org.mockito.ArgumentMatchers.*;
import static org.mockito.Mockito.when;
import static org.springframework.test.web.servlet.request.MockMvcRequestBuilders.get;
import static org.springframework.test.web.servlet.request.MockMvcRequestBuilders.post;
import static org.springframework.test.web.servlet.result.MockMvcResultMatchers.jsonPath;
import static org.springframework.test.web.servlet.result.MockMvcResultMatchers.status;

@SpringBootTest
@AutoConfigureMockMvc
@ActiveProfiles("test")
class AuthControllerTest {

    @Autowired
    private MockMvc mockMvc;
    @Autowired
    private ObjectMapper objectMapper;
    @MockBean
    private AuthService authService;
    @MockBean
    private StringRedisTemplate redisTemplate;

    @Test
    void login_returns200WithTokens() throws Exception {
        LoginResponse response = LoginResponse.builder()
                .accessToken("access-token")
                .refreshToken("refresh-token")
                .userId(1L)
                .username("admin")
                .roles(List.of("ADMIN"))
                .build();
        when(authService.login(eq(1L), eq("admin"), eq("admin123"))).thenReturn(response);

        LoginRequest request = new LoginRequest();
        request.setTenantId(1L);
        request.setUsername("admin");
        request.setPassword("admin123");

        mockMvc.perform(post("/api/auth/login")
                        .contentType(MediaType.APPLICATION_JSON)
                        .content(objectMapper.writeValueAsString(request)))
                .andExpect(status().isOk())
                .andExpect(jsonPath("$.data.accessToken").value("access-token"));
    }

    @Test
    void login_returns400OnMissingFields() throws Exception {
        mockMvc.perform(post("/api/auth/login")
                        .contentType(MediaType.APPLICATION_JSON)
                        .content("{}"))
                .andExpect(status().isBadRequest());
    }

    @Test
    void verify_returns401WithoutToken() throws Exception {
        mockMvc.perform(get("/api/auth/verify"))
                .andExpect(status().isUnauthorized());
    }
}
```

- [ ] **Step 5: 运行测试**

Run: `cd forge-identity && mvn test -Dtest=AuthControllerTest -pl . 2>&1 | tail -20`
Expected: 全部 PASS

- [ ] **Step 6: Commit**

```bash
git add forge-identity/src/
git commit -m "feat(m3): add auth controller with login, logout, refresh, and verify APIs"
```

---

### Task 7: 用户管理 API + API Token 管理

**Files:**
- Create: `forge-identity/src/main/java/com/shulex/forge/identity/entrance/vo/UserVO.java`
- Create: `forge-identity/src/main/java/com/shulex/forge/identity/entrance/vo/CreateUserRequest.java`
- Create: `forge-identity/src/main/java/com/shulex/forge/identity/entrance/vo/ApiTokenVO.java`
- Create: `forge-identity/src/main/java/com/shulex/forge/identity/service/ApiTokenService.java`
- Create: `forge-identity/src/test/java/com/shulex/forge/identity/service/ApiTokenServiceTest.java`
- Create: `forge-identity/src/main/java/com/shulex/forge/identity/entrance/controller/UserController.java`
- Create: `forge-identity/src/main/java/com/shulex/forge/identity/entrance/controller/ApiTokenController.java`

- [ ] **Step 1: 创建 UserVO.java**

```java
package com.shulex.forge.identity.entrance.vo;

import lombok.Builder;
import lombok.Data;

@Data
@Builder
public class UserVO {
    private Long id;
    private Long tenantId;
    private String username;
    private String nickname;
    private String email;
    private Integer status;
}
```

- [ ] **Step 2: 创建 CreateUserRequest.java**

```java
package com.shulex.forge.identity.entrance.vo;

import jakarta.validation.constraints.NotBlank;
import jakarta.validation.constraints.NotNull;
import lombok.Data;

@Data
public class CreateUserRequest {
    @NotNull
    private Long tenantId;
    @NotBlank
    private String username;
    @NotBlank
    private String password;
    private String nickname;
    private String roleCode;
}
```

- [ ] **Step 3: 创建 ApiTokenVO.java**

```java
package com.shulex.forge.identity.entrance.vo;

import lombok.Builder;
import lombok.Data;
import java.time.LocalDateTime;

@Data
@Builder
public class ApiTokenVO {
    private Long id;
    private String tokenName;
    private String tokenPrefix;
    private LocalDateTime expiresAt;
    private Integer status;
    private String rawToken; // 仅创建时返回，列表查询时为 null
}
```

- [ ] **Step 4: 创建 ApiTokenService.java**

```java
package com.shulex.forge.identity.service;

import com.baomidou.mybatisplus.core.conditions.query.LambdaQueryWrapper;
import com.shulex.forge.identity.entrance.vo.ApiTokenVO;
import com.shulex.forge.identity.infrastructure.entity.ApiTokenDO;
import com.shulex.forge.identity.infrastructure.mapper.ApiTokenMapper;
import lombok.extern.slf4j.Slf4j;
import org.springframework.stereotype.Service;

import java.nio.charset.StandardCharsets;
import java.security.MessageDigest;
import java.security.NoSuchAlgorithmException;
import java.time.LocalDateTime;
import java.util.HexFormat;
import java.util.List;
import java.util.UUID;

@Slf4j
@Service
public class ApiTokenService {

    private final ApiTokenMapper apiTokenMapper;

    public ApiTokenService(ApiTokenMapper apiTokenMapper) {
        this.apiTokenMapper = apiTokenMapper;
    }

    public ApiTokenVO createToken(Long tenantId, Long userId, String tokenName, LocalDateTime expiresAt) {
        String rawToken = "forge_" + UUID.randomUUID().toString().replace("-", "");
        String tokenHash = sha256(rawToken);
        String tokenPrefix = rawToken.substring(0, 12);

        ApiTokenDO token = new ApiTokenDO();
        token.setTenantId(tenantId);
        token.setUserId(userId);
        token.setTokenName(tokenName);
        token.setTokenHash(tokenHash);
        token.setTokenPrefix(tokenPrefix);
        token.setExpiresAt(expiresAt);
        token.setStatus(1);
        apiTokenMapper.insert(token);

        log.info("创建 API Token: tenant={}, user={}, name={}", tenantId, userId, tokenName);
        return ApiTokenVO.builder()
                .id(token.getId())
                .tokenName(tokenName)
                .tokenPrefix(tokenPrefix)
                .expiresAt(expiresAt)
                .status(1)
                .rawToken(rawToken)
                .build();
    }

    public List<ApiTokenVO> listTokens(Long tenantId, Long userId) {
        List<ApiTokenDO> tokens = apiTokenMapper.selectList(new LambdaQueryWrapper<ApiTokenDO>()
                .eq(ApiTokenDO::getTenantId, tenantId)
                .eq(ApiTokenDO::getUserId, userId)
                .orderByDesc(ApiTokenDO::getGmtCreate));
        return tokens.stream().map(t -> ApiTokenVO.builder()
                .id(t.getId())
                .tokenName(t.getTokenName())
                .tokenPrefix(t.getTokenPrefix())
                .expiresAt(t.getExpiresAt())
                .status(t.getStatus())
                .build()).toList();
    }

    public void revokeToken(Long tokenId) {
        ApiTokenDO token = apiTokenMapper.selectById(tokenId);
        if (token != null) {
            token.setStatus(0);
            apiTokenMapper.updateById(token);
            log.info("吊销 API Token: id={}", tokenId);
        }
    }

    public ApiTokenDO validateApiToken(String rawToken) {
        String tokenHash = sha256(rawToken);
        ApiTokenDO token = apiTokenMapper.selectOne(new LambdaQueryWrapper<ApiTokenDO>()
                .eq(ApiTokenDO::getTokenHash, tokenHash)
                .eq(ApiTokenDO::getStatus, 1));
        if (token == null) return null;
        if (token.getExpiresAt() != null && token.getExpiresAt().isBefore(LocalDateTime.now())) {
            return null;
        }
        return token;
    }

    private String sha256(String input) {
        try {
            MessageDigest digest = MessageDigest.getInstance("SHA-256");
            byte[] hash = digest.digest(input.getBytes(StandardCharsets.UTF_8));
            return HexFormat.of().formatHex(hash);
        } catch (NoSuchAlgorithmException e) {
            throw new RuntimeException("SHA-256 不可用", e);
        }
    }
}
```

- [ ] **Step 5: 创建 ApiTokenServiceTest.java**

```java
package com.shulex.forge.identity.service;

import com.shulex.forge.identity.infrastructure.entity.ApiTokenDO;
import com.shulex.forge.identity.infrastructure.mapper.ApiTokenMapper;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;

import static org.assertj.core.api.Assertions.assertThat;
import static org.mockito.ArgumentMatchers.any;
import static org.mockito.Mockito.*;

class ApiTokenServiceTest {

    private ApiTokenService apiTokenService;
    private ApiTokenMapper apiTokenMapper;

    @BeforeEach
    void setUp() {
        apiTokenMapper = mock(ApiTokenMapper.class);
        apiTokenService = new ApiTokenService(apiTokenMapper);
    }

    @Test
    void createToken_generatesForgePrefix() {
        when(apiTokenMapper.insert(any())).thenReturn(1);

        var result = apiTokenService.createToken(1L, 1L, "test-token", null);
        assertThat(result.getRawToken()).startsWith("forge_");
        assertThat(result.getTokenPrefix()).hasSize(12);
        assertThat(result.getTokenName()).isEqualTo("test-token");
        verify(apiTokenMapper).insert(any());
    }

    @Test
    void validateApiToken_returnsTokenOnMatch() {
        ApiTokenDO token = new ApiTokenDO();
        token.setId(1L);
        token.setStatus(1);
        when(apiTokenMapper.selectOne(any())).thenReturn(token);

        // First create a token to get the raw value
        when(apiTokenMapper.insert(any())).thenReturn(1);
        var created = apiTokenService.createToken(1L, 1L, "test", null);

        // Validate using the raw token
        ApiTokenDO result = apiTokenService.validateApiToken(created.getRawToken());
        assertThat(result).isNotNull();
    }

    @Test
    void validateApiToken_returnsNullOnNoMatch() {
        when(apiTokenMapper.selectOne(any())).thenReturn(null);
        assertThat(apiTokenService.validateApiToken("forge_invalid")).isNull();
    }

    @Test
    void revokeToken_setsStatusToZero() {
        ApiTokenDO token = new ApiTokenDO();
        token.setId(1L);
        token.setStatus(1);
        when(apiTokenMapper.selectById(1L)).thenReturn(token);
        when(apiTokenMapper.updateById(any())).thenReturn(1);

        apiTokenService.revokeToken(1L);
        assertThat(token.getStatus()).isEqualTo(0);
        verify(apiTokenMapper).updateById(token);
    }
}
```

- [ ] **Step 6: 创建 UserController.java**

```java
package com.shulex.forge.identity.entrance.controller;

import com.shulex.forge.identity.common.Result;
import com.shulex.forge.identity.entrance.vo.CreateUserRequest;
import com.shulex.forge.identity.entrance.vo.UserVO;
import com.shulex.forge.identity.infrastructure.entity.UserDO;
import com.shulex.forge.identity.service.UserService;
import jakarta.validation.Valid;
import org.springframework.web.bind.annotation.*;

import java.util.List;

@RestController
@RequestMapping("/api/users")
public class UserController {

    private final UserService userService;

    public UserController(UserService userService) {
        this.userService = userService;
    }

    @PostMapping
    public Result<UserVO> createUser(@Valid @RequestBody CreateUserRequest request) {
        UserDO user = userService.createUser(
                request.getTenantId(), request.getUsername(),
                request.getPassword(), request.getNickname(), request.getRoleCode());
        return Result.ok(toVO(user));
    }

    @GetMapping
    public Result<List<UserVO>> listUsers(@RequestParam(value = "tenantId") Long tenantId) {
        List<UserDO> users = userService.listUsers(tenantId);
        return Result.ok(users.stream().map(this::toVO).toList());
    }

    private UserVO toVO(UserDO user) {
        return UserVO.builder()
                .id(user.getId())
                .tenantId(user.getTenantId())
                .username(user.getUsername())
                .nickname(user.getNickname())
                .email(user.getEmail())
                .status(user.getStatus())
                .build();
    }
}
```

- [ ] **Step 7: 创建 ApiTokenController.java**

```java
package com.shulex.forge.identity.entrance.controller;

import com.shulex.forge.identity.common.Result;
import com.shulex.forge.identity.entrance.vo.ApiTokenVO;
import com.shulex.forge.identity.service.ApiTokenService;
import io.jsonwebtoken.Claims;
import org.springframework.security.core.Authentication;
import org.springframework.web.bind.annotation.*;

import java.time.LocalDateTime;
import java.util.List;

@RestController
@RequestMapping("/api/tokens")
public class ApiTokenController {

    private final ApiTokenService apiTokenService;

    public ApiTokenController(ApiTokenService apiTokenService) {
        this.apiTokenService = apiTokenService;
    }

    @PostMapping
    public Result<ApiTokenVO> createToken(
            @RequestParam(value = "name") String name,
            @RequestParam(value = "expireDays", required = false) Integer expireDays,
            Authentication authentication) {
        Claims claims = (Claims) authentication.getDetails();
        Long userId = claims.get("userId", Long.class);
        Long tenantId = claims.get("tenantId", Long.class);
        LocalDateTime expiresAt = expireDays != null
                ? LocalDateTime.now().plusDays(expireDays) : null;
        return Result.ok(apiTokenService.createToken(tenantId, userId, name, expiresAt));
    }

    @GetMapping
    public Result<List<ApiTokenVO>> listTokens(Authentication authentication) {
        Claims claims = (Claims) authentication.getDetails();
        Long userId = claims.get("userId", Long.class);
        Long tenantId = claims.get("tenantId", Long.class);
        return Result.ok(apiTokenService.listTokens(tenantId, userId));
    }

    @DeleteMapping("/{tokenId}")
    public Result<Void> revokeToken(@PathVariable("tokenId") Long tokenId) {
        apiTokenService.revokeToken(tokenId);
        return Result.ok(null);
    }
}
```

- [ ] **Step 8: 编译验证**

Run: `cd forge-identity && mvn clean compile -q`
Expected: 编译通过

- [ ] **Step 9: 运行全部测试**

Run: `cd forge-identity && mvn test -pl . 2>&1 | tail -20`
Expected: 全部 PASS

- [ ] **Step 10: Commit**

```bash
git add forge-identity/src/
git commit -m "feat(m3): add user management API and API token service with tests"
```

---

### Task 8: 应用配置 + APISIX 鉴权对接 + Docker 验证

**Files:**
- Modify: `forge-identity/src/main/resources/application.yml`
- Modify: `docker/apisix/apisix.yaml`

- [ ] **Step 1: 更新 application.yml**

```yaml
server:
  port: 8082

spring:
  application:
    name: forge-identity
  datasource:
    url: jdbc:mysql://localhost:3306/forge_identity?useSSL=false&allowPublicKeyRetrieval=true&serverTimezone=Asia/Shanghai
    username: root
    password: forge_root_2026
    driver-class-name: com.mysql.cj.jdbc.Driver
  flyway:
    enabled: true
    locations: classpath:db/migration
  data:
    redis:
      host: localhost
      port: 6379

forge:
  jwt:
    secret: ${JWT_SECRET:forge-platform-jwt-secret-key-2026-must-be-at-least-256-bits}
    access-token-expire-minutes: 30
    refresh-token-expire-minutes: 10080

mybatis-plus:
  mapper-locations: classpath*:mapper/**/*.xml
  configuration:
    map-underscore-to-camel-case: true
```

- [ ] **Step 2: 更新 APISIX 配置 — 添加 forward-auth 鉴权**

在 `docker/apisix/apisix.yaml` 中，仅修改 `forge-pipeline` 路由，添加 `forward-auth` 插件。保留所有现有路由（engine、identity、specs、bot、ws）不变。

修改 forge-pipeline 路由段，将：

```yaml
  - uri: /api/pipeline/*
    upstream:
      type: roundrobin
      nodes:
        "forge-pipeline:8083": 1
    plugins:
      proxy-rewrite:
        regex_uri: ["^/api/pipeline/(.*)", "/$1"]
```

改为：

```yaml
  - uri: /api/pipeline/*
    upstream:
      type: roundrobin
      nodes:
        "forge-pipeline:8083": 1
    plugins:
      proxy-rewrite:
        regex_uri: ["^/api/pipeline/(.*)", "/$1"]
      forward-auth:
        uri: "http://forge-identity:8082/api/auth/verify"
        request_headers: ["Authorization"]
        upstream_headers: ["X-User-Id", "X-Tenant-Id", "X-Username"]
```

注意：
- `forward-auth` 仅添加到需要鉴权的路由（pipeline），identity 和 specs 路由不需要鉴权
- `/api/auth/verify` 端点返回 200 表示通过鉴权（并通过 X-User-Id 等响应头传递用户信息给上游），返回 401 表示拒绝
- 不要删除或修改其他现有路由（engine、identity、specs、bot、ws）

- [ ] **Step 3: 运行全部测试**

Run: `cd forge-identity && mvn clean test -pl . 2>&1 | tail -20`
Expected: 全部 PASS

- [ ] **Step 4: 打包 + Docker 重建**

```bash
cd forge-identity && mvn clean package -DskipTests -q
docker compose build forge-identity
docker compose up -d forge-identity
```

- [ ] **Step 5: 验证登录 API**

```bash
sleep 15
curl -s -X POST http://localhost:8082/api/auth/login \
  -H "Content-Type: application/json" \
  -d '{"tenantId":1,"username":"admin","password":"admin123"}' | head -c 500
```

Expected: `{"code":0,"message":"success","data":{"accessToken":"eyJ...","refreshToken":"eyJ...","userId":1,...}}`

- [ ] **Step 6: 验证 Token 验证 API**

```bash
# 用上一步返回的 accessToken
TOKEN=$(curl -s -X POST http://localhost:8082/api/auth/login \
  -H "Content-Type: application/json" \
  -d '{"tenantId":1,"username":"admin","password":"admin123"}' | python3 -c "import sys,json; print(json.load(sys.stdin)['data']['accessToken'])")

curl -s http://localhost:8082/api/auth/verify -H "Authorization: Bearer $TOKEN" | head -c 300
```

Expected: `{"authenticated":true,"userId":1,"tenantId":1,"username":"admin","roles":["ADMIN"]}`

- [ ] **Step 7: 验证无 Token 被拒绝**

```bash
curl -s -w "\nHTTP_STATUS: %{http_code}\n" http://localhost:8082/api/auth/verify
```

Expected: HTTP_STATUS: 401, body: `{"authenticated":false}`

- [ ] **Step 8: Commit**

```bash
git add forge-identity/src/main/resources/application.yml docker/apisix/apisix.yaml
git commit -m "feat(m3): update config, APISIX forward-auth integration, and Docker deployment"
```

---

## M3 完成标准

- [ ] forge-identity 编译、测试全部通过
- [ ] 用户认证：账号密码登录成功返回 JWT
- [ ] JWT 签发：accessToken + refreshToken 双 Token 模式
- [ ] JWT 刷新：用 refreshToken 换新 Token 对
- [ ] JWT 吊销：logout 后 Token 加入 Redis 黑名单
- [ ] API Token：创建、列表、吊销 API Token（服务间认证）
- [ ] 基础 RBAC：ADMIN 角色可管理用户，USER 角色仅可操作自有资源
- [ ] 多租户：tenant_id 字段贯穿用户、角色、Token 全链路
- [ ] APISIX 鉴权：forward-auth 插件调用 `/api/auth/verify` 验证请求
- [ ] 验收场景：登录获取 Token → 带 Token 调用受保护 API → 无 Token 被拒绝
- [ ] Docker 部署成功
- [ ] 所有变更已 commit
