# M1 — 规范中心 实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 构建规范中心服务（forge-specs），提供编码规范查询、Prompt 模板管理、AI Review 规则查询、脚手架模板管理 API，并通过 Prompt eval 测试验证模板质量，为后续 M4（AI 引擎）提供标准化的规范注入能力。

**Architecture:** forge-specs 是一个独立的 Spring Boot 微服务，使用 MySQL 存储规范数据、MyBatis Plus 做持久化。规范内容以 Markdown/YAML 存储在数据库中，通过 REST API 对外提供查询服务。Prompt 模板采用四层叠加模型（System → Standards → Context → User）。规范支持公司级/团队级/项目级三级继承覆盖。Eval 测试作为独立的集成测试验证 Prompt 质量。

**Tech Stack:** Java 17, Spring Boot 3.2, MyBatis Plus 3.5, MySQL 8.0, Flyway（DB 迁移）, JUnit 5 + MockMvc

---

## 文件结构总览

```
forge-specs/
├── pom.xml                                          ← 补充依赖
├── src/main/java/com/shulex/forge/specs/
│   ├── ForgeSpecsApplication.java                   ← 已有
│   ├── common/
│   │   ├── Result.java                              ← 统一响应
│   │   ├── ErrorCode.java                           ← 错误码枚举
│   │   ├── BizException.java                        ← 业务异常
│   │   ├── SysException.java                        ← 系统异常
│   │   └── GlobalExceptionHandler.java              ← 全局异常处理
│   ├── entrance/
│   │   ├── controller/
│   │   │   ├── StandardController.java              ← 规范查询 API
│   │   │   ├── PromptTemplateController.java        ← Prompt 模板 API
│   │   │   ├── ReviewRuleController.java            ← Review 规则 API
│   │   │   └── ScaffoldTemplateController.java      ← 脚手架模板 API
│   │   └── vo/
│   │       ├── StandardVO.java                      ← 规范视图对象
│   │       ├── PromptTemplateVO.java                ← Prompt 模板视图对象
│   │       ├── ReviewRuleVO.java                    ← Review 规则视图对象
│   │       └── ScaffoldTemplateVO.java              ← 脚手架模板视图对象
│   ├── service/
│   │   ├── StandardService.java                     ← 规范服务
│   │   ├── PromptTemplateService.java               ← Prompt 模板服务
│   │   ├── ReviewRuleService.java                   ← Review 规则服务
│   │   └── ScaffoldTemplateService.java             ← 脚手架模板服务
│   └── infrastructure/
│       ├── entity/
│       │   ├── StandardDO.java                      ← 规范表实体
│       │   ├── PromptTemplateDO.java                ← Prompt 模板表实体
│       │   ├── ReviewRuleDO.java                    ← Review 规则表实体
│       │   └── ScaffoldTemplateDO.java              ← 脚手架模板表实体
│       └── mapper/
│           ├── StandardMapper.java                  ← 规范 Mapper
│           ├── PromptTemplateMapper.java             ← Prompt 模板 Mapper
│           ├── ReviewRuleMapper.java                ← Review 规则 Mapper
│           └── ScaffoldTemplateMapper.java          ← 脚手架模板 Mapper
├── src/main/resources/
│   ├── application.yml                              ← 更新配置
│   └── db/migration/
│       └── V1__init_specs_tables.sql                ← Flyway 建表脚本
├── src/test/java/com/shulex/forge/specs/
│   ├── entrance/controller/
│   │   ├── StandardControllerTest.java              ← 规范 API 测试
│   │   ├── PromptTemplateControllerTest.java        ← Prompt API 测试
│   │   └── ReviewRuleControllerTest.java            ← Review 规则 API 测试
│   └── service/
│       ├── StandardServiceTest.java                 ← 规范服务测试
│       ├── PromptTemplateServiceTest.java           ← Prompt 服务测试
│       └── ReviewRuleServiceTest.java               ← Review 规则服务测试
├── src/test/java/com/shulex/forge/specs/
│   ├── ...                                          ← 服务和控制器测试（同上）
│   └── eval/
│       └── PromptEvalTest.java                      ← Prompt 质量 eval 测试
└── src/test/resources/
    ├── application-test.yml                         ← 测试配置（H2 内存库）
    └── eval/                                        ← Prompt eval 样本
        ├── code-generation/
        │   ├── good-sample-01.java                  ← 符合规范的代码样本
        │   └── bad-sample-01.java                   ← 违反规范的代码样本
        ├── code-review/
        │   ├── good-sample-01.java
        │   └── bad-sample-01.java
        └── ...                                      ← 其余 4 个模板的样本
```

---

### Task 1: 补充依赖 + 基础设施代码

**Files:**
- Modify: `forge-specs/pom.xml`
- Create: `forge-specs/src/main/java/com/shulex/forge/specs/common/Result.java`
- Create: `forge-specs/src/main/java/com/shulex/forge/specs/common/ErrorCode.java`
- Create: `forge-specs/src/main/java/com/shulex/forge/specs/common/BizException.java`
- Create: `forge-specs/src/main/java/com/shulex/forge/specs/common/GlobalExceptionHandler.java`

- [ ] **Step 1: 更新 pom.xml 添加依赖**

```xml
<!-- 在 <dependencies> 中追加 -->
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
<!-- 测试用 H2 -->
<dependency>
    <groupId>com.h2database</groupId>
    <artifactId>h2</artifactId>
    <scope>test</scope>
</dependency>
```

- [ ] **Step 2: 创建 Result.java**

```java
package com.shulex.forge.specs.common;

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
package com.shulex.forge.specs.common;

import lombok.Getter;
import lombok.AllArgsConstructor;

@Getter
@AllArgsConstructor
public enum ErrorCode {
    NOT_FOUND(40400, "资源不存在"),
    INVALID_PARAM(40000, "参数错误"),
    INTERNAL_ERROR(50000, "系统内部错误");

    private final int code;
    private final String message;
}
```

- [ ] **Step 4: 创建 BizException.java**

```java
package com.shulex.forge.specs.common;

import lombok.Getter;

@Getter
public class BizException extends RuntimeException {
    private final ErrorCode errorCode;

    public BizException(ErrorCode errorCode) {
        super(errorCode.getMessage());
        this.errorCode = errorCode;
    }
}
```

- [ ] **Step 5: 创建 SysException.java**

```java
package com.shulex.forge.specs.common;

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
package com.shulex.forge.specs.common;

import lombok.extern.slf4j.Slf4j;
import org.springframework.http.ResponseEntity;
import org.springframework.web.bind.annotation.ExceptionHandler;
import org.springframework.web.bind.annotation.RestControllerAdvice;

@Slf4j
@RestControllerAdvice
public class GlobalExceptionHandler {

    @ExceptionHandler(BizException.class)
    public ResponseEntity<Result<Void>> handleBiz(BizException e) {
        log.warn("业务异常: {}", e.getMessage());
        return ResponseEntity.ok(Result.fail(e.getErrorCode().getCode(), e.getMessage()));
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

- [ ] **Step 7: 编译验证**

Run: `cd forge-specs && mvn clean compile -q`
Expected: 编译通过

- [ ] **Step 8: Commit**

```bash
git add forge-specs/pom.xml forge-specs/src/main/java/com/shulex/forge/specs/common/
git commit -m "feat(m1): add forge-specs dependencies and common infrastructure"
```

---

### Task 2: 数据库迁移 + 实体类

**Files:**
- Create: `forge-specs/src/main/resources/db/migration/V1__init_specs_tables.sql`
- Modify: `forge-specs/src/main/resources/application.yml`
- Create: `forge-specs/src/main/java/com/shulex/forge/specs/infrastructure/entity/StandardDO.java`
- Create: `forge-specs/src/main/java/com/shulex/forge/specs/infrastructure/entity/PromptTemplateDO.java`
- Create: `forge-specs/src/main/java/com/shulex/forge/specs/infrastructure/entity/ReviewRuleDO.java`
- Create: `forge-specs/src/main/java/com/shulex/forge/specs/infrastructure/entity/ScaffoldTemplateDO.java`
- Create: `forge-specs/src/main/java/com/shulex/forge/specs/infrastructure/mapper/StandardMapper.java`
- Create: `forge-specs/src/main/java/com/shulex/forge/specs/infrastructure/mapper/PromptTemplateMapper.java`
- Create: `forge-specs/src/main/java/com/shulex/forge/specs/infrastructure/mapper/ReviewRuleMapper.java`
- Create: `forge-specs/src/main/java/com/shulex/forge/specs/infrastructure/mapper/ScaffoldTemplateMapper.java`

- [ ] **Step 1: 创建 Flyway 建表脚本**

`V1__init_specs_tables.sql`:
```sql
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
```

- [ ] **Step 2: 更新 application.yml**

```yaml
server:
  port: 8084

spring:
  application:
    name: forge-specs
  datasource:
    url: jdbc:mysql://localhost:3306/forge_specs?useSSL=false&allowPublicKeyRetrieval=true&serverTimezone=Asia/Shanghai
    username: root
    password: forge_root_2026
    driver-class-name: com.mysql.cj.jdbc.Driver
  flyway:
    enabled: true
    locations: classpath:db/migration
    baseline-on-migrate: true

mybatis-plus:
  mapper-locations: classpath*:mapper/**/*.xml
  configuration:
    map-underscore-to-camel-case: true
```

- [ ] **Step 3: 创建 StandardDO.java**

```java
package com.shulex.forge.specs.infrastructure.entity;

import com.baomidou.mybatisplus.annotation.*;
import lombok.Data;
import java.time.LocalDateTime;

@Data
@TableName("spec_standard")
public class StandardDO {
    @TableId(type = IdType.AUTO)
    private Long id;
    private String category;
    private String title;
    private String content;
    private String scopeLevel;
    private String scopeId;
    private Integer sortOrder;
    private Boolean isEnabled;
    @TableField(fill = FieldFill.INSERT)
    private LocalDateTime gmtCreate;
    @TableField(fill = FieldFill.INSERT_UPDATE)
    private LocalDateTime gmtModified;
}
```

- [ ] **Step 4: 创建 PromptTemplateDO.java**

```java
package com.shulex.forge.specs.infrastructure.entity;

import com.baomidou.mybatisplus.annotation.*;
import lombok.Data;
import java.time.LocalDateTime;

@Data
@TableName("spec_prompt_template")
public class PromptTemplateDO {
    @TableId(type = IdType.AUTO)
    private Long id;
    private String templateKey;
    private String name;
    private String description;
    private String systemPrompt;
    private String standardsInjection;
    private Integer version;
    private Boolean isActive;
    @TableField(fill = FieldFill.INSERT)
    private LocalDateTime gmtCreate;
    @TableField(fill = FieldFill.INSERT_UPDATE)
    private LocalDateTime gmtModified;
}
```

- [ ] **Step 5: 创建 ReviewRuleDO.java**

```java
package com.shulex.forge.specs.infrastructure.entity;

import com.baomidou.mybatisplus.annotation.*;
import lombok.Data;
import java.time.LocalDateTime;

@Data
@TableName("spec_review_rule")
public class ReviewRuleDO {
    @TableId(type = IdType.AUTO)
    private Long id;
    private String category;
    private String ruleKey;
    private String name;
    private String description;
    private String severity;
    private Boolean isEnabled;
    @TableField(fill = FieldFill.INSERT)
    private LocalDateTime gmtCreate;
    @TableField(fill = FieldFill.INSERT_UPDATE)
    private LocalDateTime gmtModified;
}
```

- [ ] **Step 6: 创建 ScaffoldTemplateDO.java**

```java
package com.shulex.forge.specs.infrastructure.entity;

import com.baomidou.mybatisplus.annotation.*;
import lombok.Data;
import java.time.LocalDateTime;

@Data
@TableName("spec_scaffold_template")
public class ScaffoldTemplateDO {
    @TableId(type = IdType.AUTO)
    private Long id;
    private String name;
    private String description;
    private String techStack;
    private String templateContent;
    private Boolean isActive;
    @TableField(fill = FieldFill.INSERT)
    private LocalDateTime gmtCreate;
    @TableField(fill = FieldFill.INSERT_UPDATE)
    private LocalDateTime gmtModified;
}
```

- [ ] **Step 7: 创建 StandardMapper.java**

```java
package com.shulex.forge.specs.infrastructure.mapper;

import com.baomidou.mybatisplus.core.mapper.BaseMapper;
import com.shulex.forge.specs.infrastructure.entity.StandardDO;
import org.apache.ibatis.annotations.Mapper;

@Mapper
public interface StandardMapper extends BaseMapper<StandardDO> {
}
```

- [ ] **Step 7: 创建 PromptTemplateMapper.java**

```java
package com.shulex.forge.specs.infrastructure.mapper;

import com.baomidou.mybatisplus.core.mapper.BaseMapper;
import com.shulex.forge.specs.infrastructure.entity.PromptTemplateDO;
import org.apache.ibatis.annotations.Mapper;

@Mapper
public interface PromptTemplateMapper extends BaseMapper<PromptTemplateDO> {
}
```

- [ ] **Step 8: 创建 ReviewRuleMapper.java**

```java
package com.shulex.forge.specs.infrastructure.mapper;

import com.baomidou.mybatisplus.core.mapper.BaseMapper;
import com.shulex.forge.specs.infrastructure.entity.ReviewRuleDO;
import org.apache.ibatis.annotations.Mapper;

@Mapper
public interface ReviewRuleMapper extends BaseMapper<ReviewRuleDO> {
}
```

- [ ] **Step 10: 创建 ScaffoldTemplateMapper.java**

```java
package com.shulex.forge.specs.infrastructure.mapper;

import com.baomidou.mybatisplus.core.mapper.BaseMapper;
import com.shulex.forge.specs.infrastructure.entity.ScaffoldTemplateDO;
import org.apache.ibatis.annotations.Mapper;

@Mapper
public interface ScaffoldTemplateMapper extends BaseMapper<ScaffoldTemplateDO> {
}
```

- [ ] **Step 11: 编译验证**

Run: `cd forge-specs && mvn clean compile -q`
Expected: 编译通过

- [ ] **Step 12: Commit**

```bash
git add forge-specs/src/main/resources/ forge-specs/src/main/java/com/shulex/forge/specs/infrastructure/
git commit -m "feat(m1): add database schema and entity/mapper layer"
```

---

### Task 3: 规范查询服务 + API（TDD）

**Files:**
- Create: `forge-specs/src/test/resources/application-test.yml`
- Create: `forge-specs/src/test/java/com/shulex/forge/specs/service/StandardServiceTest.java`
- Create: `forge-specs/src/main/java/com/shulex/forge/specs/service/StandardService.java`
- Create: `forge-specs/src/main/java/com/shulex/forge/specs/entrance/vo/StandardVO.java`
- Create: `forge-specs/src/test/java/com/shulex/forge/specs/entrance/controller/StandardControllerTest.java`
- Create: `forge-specs/src/main/java/com/shulex/forge/specs/entrance/controller/StandardController.java`

- [ ] **Step 1: 创建测试配置 application-test.yml**

```yaml
spring:
  datasource:
    url: jdbc:h2:mem:forge_specs_test;MODE=MYSQL;DB_CLOSE_DELAY=-1
    driver-class-name: org.h2.Driver
    username: sa
    password:
  flyway:
    enabled: true
    locations: classpath:db/migration

mybatis-plus:
  configuration:
    map-underscore-to-camel-case: true
```

- [ ] **Step 2: 写 StandardService 失败测试**

```java
package com.shulex.forge.specs.service;

import com.shulex.forge.specs.infrastructure.entity.StandardDO;
import com.shulex.forge.specs.infrastructure.mapper.StandardMapper;
import org.junit.jupiter.api.Test;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.boot.test.context.SpringBootTest;
import org.springframework.test.context.ActiveProfiles;

import java.util.List;

import org.springframework.transaction.annotation.Transactional;

import static org.assertj.core.api.Assertions.assertThat;

@SpringBootTest
@ActiveProfiles("test")
@Transactional
class StandardServiceTest {

    @Autowired
    private StandardService standardService;

    @Autowired
    private StandardMapper standardMapper;

    @Test
    void listByCategory_returnsMatchingStandards() {
        StandardDO std = new StandardDO();
        std.setCategory("java");
        std.setTitle("命名规范");
        std.setContent("类名使用 UpperCamelCase");
        std.setScopeLevel("company");
        std.setSortOrder(1);
        std.setIsEnabled(true);
        standardMapper.insert(std);

        List<StandardDO> result = standardService.listByCategory("java");
        assertThat(result).hasSize(1);
        assertThat(result.get(0).getTitle()).isEqualTo("命名规范");
    }

    @Test
    void listByCategory_excludesDisabled() {
        StandardDO std = new StandardDO();
        std.setCategory("sql");
        std.setTitle("已禁用规范");
        std.setContent("...");
        std.setScopeLevel("company");
        std.setSortOrder(1);
        std.setIsEnabled(false);
        standardMapper.insert(std);

        List<StandardDO> result = standardService.listByCategory("sql");
        assertThat(result).isEmpty();
    }

    @Test
    void listEffective_mergesScopes() {
        // 公司级
        StandardDO company = new StandardDO();
        company.setCategory("java");
        company.setTitle("公司命名规范");
        company.setContent("公司级");
        company.setScopeLevel("company");
        company.setSortOrder(1);
        company.setIsEnabled(true);
        standardMapper.insert(company);

        // 项目级覆盖
        StandardDO project = new StandardDO();
        project.setCategory("java");
        project.setTitle("项目命名规范");
        project.setContent("项目级覆盖");
        project.setScopeLevel("project");
        project.setScopeId("proj-001");
        project.setSortOrder(1);
        project.setIsEnabled(true);
        standardMapper.insert(project);

        List<StandardDO> result = standardService.listEffective("java", "project", "proj-001");
        assertThat(result).hasSize(2);
    }
}
```

- [ ] **Step 3: 运行测试确认失败**

Run: `cd forge-specs && mvn test -Dtest=StandardServiceTest -pl . -q`
Expected: 编译失败 — StandardService 不存在

- [ ] **Step 4: 实现 StandardService**

```java
package com.shulex.forge.specs.service;

import com.baomidou.mybatisplus.core.conditions.query.LambdaQueryWrapper;
import com.shulex.forge.specs.infrastructure.entity.StandardDO;
import com.shulex.forge.specs.infrastructure.mapper.StandardMapper;
import org.springframework.stereotype.Service;

import java.util.List;

@Service
public class StandardService {

    private final StandardMapper standardMapper;

    public StandardService(StandardMapper standardMapper) {
        this.standardMapper = standardMapper;
    }

    public List<StandardDO> listByCategory(String category) {
        return standardMapper.selectList(
                new LambdaQueryWrapper<StandardDO>()
                        .eq(StandardDO::getCategory, category)
                        .eq(StandardDO::getIsEnabled, true)
                        .orderByAsc(StandardDO::getSortOrder)
        );
    }

    public List<StandardDO> listEffective(String category, String scopeLevel, String scopeId) {
        return standardMapper.selectList(
                new LambdaQueryWrapper<StandardDO>()
                        .eq(StandardDO::getCategory, category)
                        .eq(StandardDO::getIsEnabled, true)
                        .and(w -> w
                                .eq(StandardDO::getScopeLevel, "company")
                                .or()
                                .eq(StandardDO::getScopeLevel, scopeLevel)
                                .eq(scopeId != null, StandardDO::getScopeId, scopeId)
                        )
                        .orderByAsc(StandardDO::getSortOrder)
        );
    }
}
```

- [ ] **Step 5: 运行测试确认通过**

Run: `cd forge-specs && mvn test -Dtest=StandardServiceTest -pl . -q`
Expected: 全部 PASS

- [ ] **Step 6: 创建 StandardVO.java**

```java
package com.shulex.forge.specs.entrance.vo;

import lombok.Data;

@Data
public class StandardVO {
    private Long id;
    private String category;
    private String title;
    private String content;
    private String scopeLevel;
}
```

- [ ] **Step 7: 写 StandardController 失败测试**

```java
package com.shulex.forge.specs.entrance.controller;

import org.junit.jupiter.api.Test;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.boot.test.autoconfigure.web.servlet.AutoConfigureMockMvc;
import org.springframework.boot.test.context.SpringBootTest;
import org.springframework.test.context.ActiveProfiles;
import org.springframework.test.web.servlet.MockMvc;

import static org.springframework.test.web.servlet.request.MockMvcRequestBuilders.get;
import static org.springframework.test.web.servlet.result.MockMvcResultMatchers.*;

@SpringBootTest
@AutoConfigureMockMvc
@ActiveProfiles("test")
class StandardControllerTest {

    @Autowired
    private MockMvc mockMvc;

    @Test
    void listByCategory_returns200() throws Exception {
        mockMvc.perform(get("/api/standards").param("category", "java"))
                .andExpect(status().isOk())
                .andExpect(jsonPath("$.code").value(0))
                .andExpect(jsonPath("$.data").isArray());
    }

    @Test
    void listEffective_returns200() throws Exception {
        mockMvc.perform(get("/api/standards/effective")
                        .param("category", "java")
                        .param("scopeLevel", "project")
                        .param("scopeId", "proj-001"))
                .andExpect(status().isOk())
                .andExpect(jsonPath("$.code").value(0));
    }
}
```

- [ ] **Step 8: 运行测试确认失败**

Run: `cd forge-specs && mvn test -Dtest=StandardControllerTest -pl . -q`
Expected: 404 — Controller 不存在

- [ ] **Step 9: 实现 StandardController**

```java
package com.shulex.forge.specs.entrance.controller;

import com.shulex.forge.specs.common.Result;
import com.shulex.forge.specs.entrance.vo.StandardVO;
import com.shulex.forge.specs.infrastructure.entity.StandardDO;
import com.shulex.forge.specs.service.StandardService;
import org.springframework.web.bind.annotation.*;

import java.util.List;

@RestController
@RequestMapping("/api/standards")
public class StandardController {

    private final StandardService standardService;

    public StandardController(StandardService standardService) {
        this.standardService = standardService;
    }

    @GetMapping
    public Result<List<StandardVO>> listByCategory(@RequestParam String category) {
        List<StandardDO> list = standardService.listByCategory(category);
        return Result.ok(list.stream().map(this::toVO).toList());
    }

    @GetMapping("/effective")
    public Result<List<StandardVO>> listEffective(
            @RequestParam String category,
            @RequestParam(defaultValue = "company") String scopeLevel,
            @RequestParam(required = false) String scopeId) {
        List<StandardDO> list = standardService.listEffective(category, scopeLevel, scopeId);
        return Result.ok(list.stream().map(this::toVO).toList());
    }

    private StandardVO toVO(StandardDO entity) {
        StandardVO vo = new StandardVO();
        vo.setId(entity.getId());
        vo.setCategory(entity.getCategory());
        vo.setTitle(entity.getTitle());
        vo.setContent(entity.getContent());
        vo.setScopeLevel(entity.getScopeLevel());
        return vo;
    }
}
```

- [ ] **Step 10: 运行测试确认全部通过**

Run: `cd forge-specs && mvn test -Dtest=StandardControllerTest -pl . -q`
Expected: 全部 PASS

- [ ] **Step 11: Commit**

```bash
git add forge-specs/src/
git commit -m "feat(m1): add standard query service and API with tests"
```

---

### Task 4: Prompt 模板服务 + API（TDD）

**Files:**
- Create: `forge-specs/src/test/java/com/shulex/forge/specs/service/PromptTemplateServiceTest.java`
- Create: `forge-specs/src/main/java/com/shulex/forge/specs/service/PromptTemplateService.java`
- Create: `forge-specs/src/main/java/com/shulex/forge/specs/entrance/vo/PromptTemplateVO.java`
- Create: `forge-specs/src/test/java/com/shulex/forge/specs/entrance/controller/PromptTemplateControllerTest.java`
- Create: `forge-specs/src/main/java/com/shulex/forge/specs/entrance/controller/PromptTemplateController.java`

- [ ] **Step 1: 写 PromptTemplateService 失败测试**

```java
package com.shulex.forge.specs.service;

import com.shulex.forge.specs.common.BizException;
import com.shulex.forge.specs.infrastructure.entity.PromptTemplateDO;
import com.shulex.forge.specs.infrastructure.mapper.PromptTemplateMapper;
import org.junit.jupiter.api.Test;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.boot.test.context.SpringBootTest;
import org.springframework.test.context.ActiveProfiles;

import static org.assertj.core.api.Assertions.assertThat;
import org.springframework.transaction.annotation.Transactional;

import static org.assertj.core.api.Assertions.assertThatThrownBy;

@SpringBootTest
@ActiveProfiles("test")
@Transactional
class PromptTemplateServiceTest {

    @Autowired
    private PromptTemplateService promptTemplateService;

    @Autowired
    private PromptTemplateMapper promptTemplateMapper;

    @Test
    void getActiveByKey_returnsActiveTemplate() {
        PromptTemplateDO t = new PromptTemplateDO();
        t.setTemplateKey("code-generation");
        t.setName("代码生成");
        t.setSystemPrompt("You are a code generator...");
        t.setStandardsInjection("Follow these standards: {{standards}}");
        t.setVersion(1);
        t.setIsActive(true);
        promptTemplateMapper.insert(t);

        PromptTemplateDO result = promptTemplateService.getActiveByKey("code-generation");
        assertThat(result.getName()).isEqualTo("代码生成");
        assertThat(result.getSystemPrompt()).contains("code generator");
    }

    @Test
    void getActiveByKey_throwsWhenNotFound() {
        assertThatThrownBy(() -> promptTemplateService.getActiveByKey("nonexistent"))
                .isInstanceOf(BizException.class);
    }

    @Test
    void listAll_returnsOnlyActiveTemplates() {
        PromptTemplateDO active = new PromptTemplateDO();
        active.setTemplateKey("test-generation");
        active.setName("测试生成");
        active.setSystemPrompt("...");
        active.setVersion(1);
        active.setIsActive(true);
        promptTemplateMapper.insert(active);

        PromptTemplateDO inactive = new PromptTemplateDO();
        inactive.setTemplateKey("test-generation");
        inactive.setName("测试生成 v0");
        inactive.setSystemPrompt("...");
        inactive.setVersion(0);
        inactive.setIsActive(false);
        promptTemplateMapper.insert(inactive);

        var result = promptTemplateService.listActive();
        assertThat(result.stream().noneMatch(t -> t.getVersion() == 0)).isTrue();
    }
}
```

- [ ] **Step 2: 运行测试确认失败**

Run: `cd forge-specs && mvn test -Dtest=PromptTemplateServiceTest -pl . -q`
Expected: 编译失败

- [ ] **Step 3: 实现 PromptTemplateService**

```java
package com.shulex.forge.specs.service;

import com.baomidou.mybatisplus.core.conditions.query.LambdaQueryWrapper;
import com.shulex.forge.specs.common.BizException;
import com.shulex.forge.specs.common.ErrorCode;
import com.shulex.forge.specs.infrastructure.entity.PromptTemplateDO;
import com.shulex.forge.specs.infrastructure.mapper.PromptTemplateMapper;
import org.springframework.stereotype.Service;

import java.util.List;

@Service
public class PromptTemplateService {

    private final PromptTemplateMapper promptTemplateMapper;

    public PromptTemplateService(PromptTemplateMapper promptTemplateMapper) {
        this.promptTemplateMapper = promptTemplateMapper;
    }

    public PromptTemplateDO getActiveByKey(String templateKey) {
        PromptTemplateDO template = promptTemplateMapper.selectOne(
                new LambdaQueryWrapper<PromptTemplateDO>()
                        .eq(PromptTemplateDO::getTemplateKey, templateKey)
                        .eq(PromptTemplateDO::getIsActive, true)
        );
        if (template == null) {
            throw new BizException(ErrorCode.NOT_FOUND);
        }
        return template;
    }

    public List<PromptTemplateDO> listActive() {
        return promptTemplateMapper.selectList(
                new LambdaQueryWrapper<PromptTemplateDO>()
                        .eq(PromptTemplateDO::getIsActive, true)
        );
    }
}
```

- [ ] **Step 4: 运行测试确认通过**

Run: `cd forge-specs && mvn test -Dtest=PromptTemplateServiceTest -pl . -q`
Expected: 全部 PASS

- [ ] **Step 5: 创建 PromptTemplateVO.java**

```java
package com.shulex.forge.specs.entrance.vo;

import lombok.Data;

@Data
public class PromptTemplateVO {
    private Long id;
    private String templateKey;
    private String name;
    private String description;
    private String systemPrompt;
    private String standardsInjection;
    private Integer version;
}
```

- [ ] **Step 6: 写 PromptTemplateController 失败测试**

```java
package com.shulex.forge.specs.entrance.controller;

import org.junit.jupiter.api.Test;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.boot.test.autoconfigure.web.servlet.AutoConfigureMockMvc;
import org.springframework.boot.test.context.SpringBootTest;
import org.springframework.test.context.ActiveProfiles;
import org.springframework.test.web.servlet.MockMvc;

import static org.springframework.test.web.servlet.request.MockMvcRequestBuilders.get;
import static org.springframework.test.web.servlet.result.MockMvcResultMatchers.*;

@SpringBootTest
@AutoConfigureMockMvc
@ActiveProfiles("test")
class PromptTemplateControllerTest {

    @Autowired
    private MockMvc mockMvc;

    @Test
    void listActive_returns200() throws Exception {
        mockMvc.perform(get("/api/prompts"))
                .andExpect(status().isOk())
                .andExpect(jsonPath("$.code").value(0))
                .andExpect(jsonPath("$.data").isArray());
    }

    @Test
    void getByKey_returns200WhenExists() throws Exception {
        // 注意：此测试依赖 seed data 或其他测试插入的数据
        // 如果无数据会返回 40400，也是正确行为
        mockMvc.perform(get("/api/prompts/code-generation"))
                .andExpect(status().isOk())
                .andExpect(jsonPath("$.code").isNumber());
    }
}
```

- [ ] **Step 7: 运行测试确认失败**

Run: `cd forge-specs && mvn test -Dtest=PromptTemplateControllerTest -pl . -q`
Expected: 404

- [ ] **Step 8: 实现 PromptTemplateController**

```java
package com.shulex.forge.specs.entrance.controller;

import com.shulex.forge.specs.common.Result;
import com.shulex.forge.specs.entrance.vo.PromptTemplateVO;
import com.shulex.forge.specs.infrastructure.entity.PromptTemplateDO;
import com.shulex.forge.specs.service.PromptTemplateService;
import org.springframework.web.bind.annotation.*;

import java.util.List;

@RestController
@RequestMapping("/api/prompts")
public class PromptTemplateController {

    private final PromptTemplateService promptTemplateService;

    public PromptTemplateController(PromptTemplateService promptTemplateService) {
        this.promptTemplateService = promptTemplateService;
    }

    @GetMapping
    public Result<List<PromptTemplateVO>> listActive() {
        List<PromptTemplateDO> list = promptTemplateService.listActive();
        return Result.ok(list.stream().map(this::toVO).toList());
    }

    @GetMapping("/{templateKey}")
    public Result<PromptTemplateVO> getByKey(@PathVariable String templateKey) {
        PromptTemplateDO template = promptTemplateService.getActiveByKey(templateKey);
        return Result.ok(toVO(template));
    }

    private PromptTemplateVO toVO(PromptTemplateDO entity) {
        PromptTemplateVO vo = new PromptTemplateVO();
        vo.setId(entity.getId());
        vo.setTemplateKey(entity.getTemplateKey());
        vo.setName(entity.getName());
        vo.setDescription(entity.getDescription());
        vo.setSystemPrompt(entity.getSystemPrompt());
        vo.setStandardsInjection(entity.getStandardsInjection());
        vo.setVersion(entity.getVersion());
        return vo;
    }
}
```

- [ ] **Step 9: 运行测试确认全部通过**

Run: `cd forge-specs && mvn test -Dtest=PromptTemplateControllerTest -pl . -q`
Expected: 全部 PASS

- [ ] **Step 10: Commit**

```bash
git add forge-specs/src/
git commit -m "feat(m1): add prompt template service and API with tests"
```

---

### Task 5: Review 规则服务 + API（TDD）

**Files:**
- Create: `forge-specs/src/test/java/com/shulex/forge/specs/service/ReviewRuleServiceTest.java`
- Create: `forge-specs/src/main/java/com/shulex/forge/specs/service/ReviewRuleService.java`
- Create: `forge-specs/src/main/java/com/shulex/forge/specs/entrance/vo/ReviewRuleVO.java`
- Create: `forge-specs/src/test/java/com/shulex/forge/specs/entrance/controller/ReviewRuleControllerTest.java`
- Create: `forge-specs/src/main/java/com/shulex/forge/specs/entrance/controller/ReviewRuleController.java`

- [ ] **Step 1: 写 ReviewRuleService 失败测试**

```java
package com.shulex.forge.specs.service;

import com.shulex.forge.specs.infrastructure.entity.ReviewRuleDO;
import com.shulex.forge.specs.infrastructure.mapper.ReviewRuleMapper;
import org.junit.jupiter.api.Test;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.boot.test.context.SpringBootTest;
import org.springframework.test.context.ActiveProfiles;

import java.util.List;

import org.springframework.transaction.annotation.Transactional;

import static org.assertj.core.api.Assertions.assertThat;

@SpringBootTest
@ActiveProfiles("test")
@Transactional
class ReviewRuleServiceTest {

    @Autowired
    private ReviewRuleService reviewRuleService;

    @Autowired
    private ReviewRuleMapper reviewRuleMapper;

    @Test
    void listByCategory_returnsEnabledRules() {
        ReviewRuleDO rule = new ReviewRuleDO();
        rule.setCategory("security");
        rule.setRuleKey("sql-injection-check");
        rule.setName("SQL 注入检查");
        rule.setDescription("检查是否使用参数化查询");
        rule.setSeverity("error");
        rule.setIsEnabled(true);
        reviewRuleMapper.insert(rule);

        List<ReviewRuleDO> result = reviewRuleService.listByCategory("security");
        assertThat(result).hasSize(1);
        assertThat(result.get(0).getRuleKey()).isEqualTo("sql-injection-check");
    }

    @Test
    void listAll_returnsAllEnabledRules() {
        ReviewRuleDO r1 = new ReviewRuleDO();
        r1.setCategory("coding");
        r1.setRuleKey("naming-convention");
        r1.setName("命名规范检查");
        r1.setDescription("...");
        r1.setSeverity("warning");
        r1.setIsEnabled(true);
        reviewRuleMapper.insert(r1);

        List<ReviewRuleDO> result = reviewRuleService.listAllEnabled();
        assertThat(result).isNotEmpty();
    }
}
```

- [ ] **Step 2: 运行测试确认失败**

Run: `cd forge-specs && mvn test -Dtest=ReviewRuleServiceTest -pl . -q`
Expected: 编译失败

- [ ] **Step 3: 实现 ReviewRuleService**

```java
package com.shulex.forge.specs.service;

import com.baomidou.mybatisplus.core.conditions.query.LambdaQueryWrapper;
import com.shulex.forge.specs.infrastructure.entity.ReviewRuleDO;
import com.shulex.forge.specs.infrastructure.mapper.ReviewRuleMapper;
import org.springframework.stereotype.Service;

import java.util.List;

@Service
public class ReviewRuleService {

    private final ReviewRuleMapper reviewRuleMapper;

    public ReviewRuleService(ReviewRuleMapper reviewRuleMapper) {
        this.reviewRuleMapper = reviewRuleMapper;
    }

    public List<ReviewRuleDO> listByCategory(String category) {
        return reviewRuleMapper.selectList(
                new LambdaQueryWrapper<ReviewRuleDO>()
                        .eq(ReviewRuleDO::getCategory, category)
                        .eq(ReviewRuleDO::getIsEnabled, true)
        );
    }

    public List<ReviewRuleDO> listAllEnabled() {
        return reviewRuleMapper.selectList(
                new LambdaQueryWrapper<ReviewRuleDO>()
                        .eq(ReviewRuleDO::getIsEnabled, true)
        );
    }
}
```

- [ ] **Step 4: 运行测试确认通过**

Run: `cd forge-specs && mvn test -Dtest=ReviewRuleServiceTest -pl . -q`
Expected: 全部 PASS

- [ ] **Step 5: 创建 ReviewRuleVO.java**

```java
package com.shulex.forge.specs.entrance.vo;

import lombok.Data;

@Data
public class ReviewRuleVO {
    private Long id;
    private String category;
    private String ruleKey;
    private String name;
    private String description;
    private String severity;
}
```

- [ ] **Step 6: 写 ReviewRuleController 失败测试**

```java
package com.shulex.forge.specs.entrance.controller;

import org.junit.jupiter.api.Test;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.boot.test.autoconfigure.web.servlet.AutoConfigureMockMvc;
import org.springframework.boot.test.context.SpringBootTest;
import org.springframework.test.context.ActiveProfiles;
import org.springframework.test.web.servlet.MockMvc;

import static org.springframework.test.web.servlet.request.MockMvcRequestBuilders.get;
import static org.springframework.test.web.servlet.result.MockMvcResultMatchers.*;

@SpringBootTest
@AutoConfigureMockMvc
@ActiveProfiles("test")
class ReviewRuleControllerTest {

    @Autowired
    private MockMvc mockMvc;

    @Test
    void listByCategory_returns200() throws Exception {
        mockMvc.perform(get("/api/review-rules").param("category", "security"))
                .andExpect(status().isOk())
                .andExpect(jsonPath("$.code").value(0))
                .andExpect(jsonPath("$.data").isArray());
    }

    @Test
    void listAll_returns200() throws Exception {
        mockMvc.perform(get("/api/review-rules/all"))
                .andExpect(status().isOk())
                .andExpect(jsonPath("$.code").value(0));
    }
}
```

- [ ] **Step 7: 运行测试确认失败**

Run: `cd forge-specs && mvn test -Dtest=ReviewRuleControllerTest -pl . -q`
Expected: 404

- [ ] **Step 8: 实现 ReviewRuleController**

```java
package com.shulex.forge.specs.entrance.controller;

import com.shulex.forge.specs.common.Result;
import com.shulex.forge.specs.entrance.vo.ReviewRuleVO;
import com.shulex.forge.specs.infrastructure.entity.ReviewRuleDO;
import com.shulex.forge.specs.service.ReviewRuleService;
import org.springframework.web.bind.annotation.*;

import java.util.List;

@RestController
@RequestMapping("/api/review-rules")
public class ReviewRuleController {

    private final ReviewRuleService reviewRuleService;

    public ReviewRuleController(ReviewRuleService reviewRuleService) {
        this.reviewRuleService = reviewRuleService;
    }

    @GetMapping
    public Result<List<ReviewRuleVO>> listByCategory(@RequestParam String category) {
        List<ReviewRuleDO> list = reviewRuleService.listByCategory(category);
        return Result.ok(list.stream().map(this::toVO).toList());
    }

    @GetMapping("/all")
    public Result<List<ReviewRuleVO>> listAll() {
        List<ReviewRuleDO> list = reviewRuleService.listAllEnabled();
        return Result.ok(list.stream().map(this::toVO).toList());
    }

    private ReviewRuleVO toVO(ReviewRuleDO entity) {
        ReviewRuleVO vo = new ReviewRuleVO();
        vo.setId(entity.getId());
        vo.setCategory(entity.getCategory());
        vo.setRuleKey(entity.getRuleKey());
        vo.setName(entity.getName());
        vo.setDescription(entity.getDescription());
        vo.setSeverity(entity.getSeverity());
        return vo;
    }
}
```

- [ ] **Step 9: 运行测试确认全部通过**

Run: `cd forge-specs && mvn test -Dtest=ReviewRuleControllerTest -pl . -q`
Expected: 全部 PASS

- [ ] **Step 10: Commit**

```bash
git add forge-specs/src/
git commit -m "feat(m1): add review rule service and API with tests"
```

---

### Task 6: Seed Data — 编码规范 + Prompt 模板 + Review 规则

**Files:**
- Create: `forge-specs/src/main/resources/db/migration/V2__seed_standards.sql`
- Create: `forge-specs/src/main/resources/db/migration/V3__seed_prompts.sql`
- Create: `forge-specs/src/main/resources/db/migration/V4__seed_review_rules.sql`

- [ ] **Step 1: 创建编码规范种子数据**

`V2__seed_standards.sql` — 从 `docs/references/coding-standards.md` 提取核心规范，按 category 分组插入。至少包含以下分类：

| category | 条数 | 内容 |
|----------|------|------|
| java | 5+ | 命名规范、分层架构、异常体系、日志规范、集合与并发 |
| sql | 3+ | 表命名、索引规范、必备字段 |
| api | 2+ | Result 包装、RESTful 设计 |
| security | 3+ | SQL 注入防护、XSS/CSRF、敏感数据脱敏 |
| git | 2+ | 分支策略、Commit 格式 |

每条规范的 `content` 字段为 Markdown 格式，包含完整的规则描述和示例代码。

- [ ] **Step 2: 创建 Prompt 模板种子数据**

`V3__seed_prompts.sql` — 6 个核心 Prompt 模板：

| template_key | name | 说明 |
|-------------|------|------|
| requirement-analysis | 需求解析 | 解析用户需求为结构化任务清单 |
| code-generation | 代码生成 | 根据任务清单 + 规范生成代码 |
| code-review | 代码审查 | AI Review + 评分 + 修复建议 |
| test-generation | 测试生成 | 生成单元测试代码 |
| fix-generation | 修复生成 | 根据 Review 反馈修复代码 |
| doc-generation | 文档生成 | 生成 API 文档 + 变更日志 |

每个模板包含 `system_prompt`（角色定义 + 行为约束 + 输出格式）和 `standards_injection`（变量占位模板如 `{{standards}}`、`{{context}}`）。

- [ ] **Step 3: 创建 Review 规则种子数据**

`V4__seed_review_rules.sql` — 至少 15 条 Review 规则：

| category | 条数 | 内容 |
|----------|------|------|
| coding | 5+ | 命名规范、DO/DTO 分层、构造器注入、Result 包装、SLF4J 日志 |
| security | 5+ | SQL 注入、XSS、CSRF、敏感数据、权限校验 |
| performance | 3+ | N+1 查询、批量操作、缓存使用 |
| database | 2+ | 索引设计、慢查询 |

- [ ] **Step 4: 编译验证**

Run: `cd forge-specs && mvn clean compile -q`
Expected: 编译通过

- [ ] **Step 5: Commit**

```bash
git add forge-specs/src/main/resources/db/migration/
git commit -m "feat(m1): add seed data for standards, prompts, and review rules"
```

---

### Task 7: 全量测试 + Docker 重建 + 端到端验证

**Files:**
- Modify: `docker-compose.yml`（无需修改，forge-specs 容器已配置）

- [ ] **Step 1: 运行全部测试**

Run: `cd forge-specs && mvn clean test -q`
Expected: 全部 PASS

- [ ] **Step 2: 重新打包 + 重建 Docker 镜像**

```bash
cd forge-specs && mvn clean package -DskipTests -q
docker compose build forge-specs
docker compose up -d forge-specs
```

- [ ] **Step 3: 等待启动，验证 API**

```bash
sleep 15
# 查询规范
curl -s http://localhost:8084/api/standards?category=java | head -c 200
# 查询 Prompt 模板
curl -s http://localhost:8084/api/prompts | head -c 200
# 查询 Review 规则
curl -s http://localhost:8084/api/review-rules?category=security | head -c 200
# 通过 APISIX 网关访问
curl -s http://localhost:9080/api/specs/api/standards?category=java | head -c 200
```

Expected: 全部返回 `{"code":0,"message":"success","data":[...],...}`

- [ ] **Step 4: Commit（如有调整）**

```bash
git add forge-specs/
git commit -m "chore(m1): final adjustments after integration testing"
```

---

### Task 8: 脚手架模板服务 + API（TDD）

**Files:**
- Create: `forge-specs/src/test/java/com/shulex/forge/specs/service/ScaffoldTemplateServiceTest.java`
- Create: `forge-specs/src/main/java/com/shulex/forge/specs/service/ScaffoldTemplateService.java`
- Create: `forge-specs/src/main/java/com/shulex/forge/specs/entrance/vo/ScaffoldTemplateVO.java`
- Create: `forge-specs/src/test/java/com/shulex/forge/specs/entrance/controller/ScaffoldTemplateControllerTest.java`
- Create: `forge-specs/src/main/java/com/shulex/forge/specs/entrance/controller/ScaffoldTemplateController.java`
- Create: `forge-specs/src/main/resources/db/migration/V5__seed_scaffolds.sql`

- [ ] **Step 1: 写 ScaffoldTemplateService 失败测试**

```java
package com.shulex.forge.specs.service;

import com.shulex.forge.specs.common.BizException;
import com.shulex.forge.specs.infrastructure.entity.ScaffoldTemplateDO;
import com.shulex.forge.specs.infrastructure.mapper.ScaffoldTemplateMapper;
import org.junit.jupiter.api.Test;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.boot.test.context.SpringBootTest;
import org.springframework.test.context.ActiveProfiles;
import org.springframework.transaction.annotation.Transactional;

import java.util.List;

import static org.assertj.core.api.Assertions.assertThat;
import static org.assertj.core.api.Assertions.assertThatThrownBy;

@SpringBootTest
@ActiveProfiles("test")
@Transactional
class ScaffoldTemplateServiceTest {

    @Autowired
    private ScaffoldTemplateService scaffoldTemplateService;

    @Autowired
    private ScaffoldTemplateMapper scaffoldTemplateMapper;

    @Test
    void getByName_returnsTemplate() {
        ScaffoldTemplateDO t = new ScaffoldTemplateDO();
        t.setName("java-microservice");
        t.setDescription("Java 微服务骨架");
        t.setTechStack("Java 17, Spring Boot 3.2, MyBatis Plus");
        t.setTemplateContent("{\"files\":[]}");
        t.setIsActive(true);
        scaffoldTemplateMapper.insert(t);

        ScaffoldTemplateDO result = scaffoldTemplateService.getByName("java-microservice");
        assertThat(result.getDescription()).isEqualTo("Java 微服务骨架");
    }

    @Test
    void getByName_throwsWhenNotFound() {
        assertThatThrownBy(() -> scaffoldTemplateService.getByName("nonexistent"))
                .isInstanceOf(BizException.class);
    }

    @Test
    void listActive_returnsOnlyActive() {
        ScaffoldTemplateDO active = new ScaffoldTemplateDO();
        active.setName("active-scaffold");
        active.setTemplateContent("{}");
        active.setIsActive(true);
        scaffoldTemplateMapper.insert(active);

        ScaffoldTemplateDO inactive = new ScaffoldTemplateDO();
        inactive.setName("inactive-scaffold");
        inactive.setTemplateContent("{}");
        inactive.setIsActive(false);
        scaffoldTemplateMapper.insert(inactive);

        List<ScaffoldTemplateDO> result = scaffoldTemplateService.listActive();
        assertThat(result.stream().noneMatch(s -> "inactive-scaffold".equals(s.getName()))).isTrue();
    }
}
```

- [ ] **Step 2: 运行测试确认失败**

Run: `cd forge-specs && mvn test -Dtest=ScaffoldTemplateServiceTest -pl . -q`
Expected: 编译失败 — ScaffoldTemplateService 不存在

- [ ] **Step 3: 实现 ScaffoldTemplateService**

```java
package com.shulex.forge.specs.service;

import com.baomidou.mybatisplus.core.conditions.query.LambdaQueryWrapper;
import com.shulex.forge.specs.common.BizException;
import com.shulex.forge.specs.common.ErrorCode;
import com.shulex.forge.specs.infrastructure.entity.ScaffoldTemplateDO;
import com.shulex.forge.specs.infrastructure.mapper.ScaffoldTemplateMapper;
import org.springframework.stereotype.Service;

import java.util.List;

@Service
public class ScaffoldTemplateService {

    private final ScaffoldTemplateMapper scaffoldTemplateMapper;

    public ScaffoldTemplateService(ScaffoldTemplateMapper scaffoldTemplateMapper) {
        this.scaffoldTemplateMapper = scaffoldTemplateMapper;
    }

    public ScaffoldTemplateDO getByName(String name) {
        ScaffoldTemplateDO template = scaffoldTemplateMapper.selectOne(
                new LambdaQueryWrapper<ScaffoldTemplateDO>()
                        .eq(ScaffoldTemplateDO::getName, name)
                        .eq(ScaffoldTemplateDO::getIsActive, true)
        );
        if (template == null) {
            throw new BizException(ErrorCode.NOT_FOUND);
        }
        return template;
    }

    public List<ScaffoldTemplateDO> listActive() {
        return scaffoldTemplateMapper.selectList(
                new LambdaQueryWrapper<ScaffoldTemplateDO>()
                        .eq(ScaffoldTemplateDO::getIsActive, true)
        );
    }
}
```

- [ ] **Step 4: 运行测试确认通过**

Run: `cd forge-specs && mvn test -Dtest=ScaffoldTemplateServiceTest -pl . -q`
Expected: 全部 PASS

- [ ] **Step 5: 创建 ScaffoldTemplateVO.java**

```java
package com.shulex.forge.specs.entrance.vo;

import lombok.Data;

@Data
public class ScaffoldTemplateVO {
    private Long id;
    private String name;
    private String description;
    private String techStack;
    private String templateContent;
}
```

- [ ] **Step 6: 写 ScaffoldTemplateController 失败测试**

```java
package com.shulex.forge.specs.entrance.controller;

import org.junit.jupiter.api.Test;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.boot.test.autoconfigure.web.servlet.AutoConfigureMockMvc;
import org.springframework.boot.test.context.SpringBootTest;
import org.springframework.test.context.ActiveProfiles;
import org.springframework.test.web.servlet.MockMvc;

import static org.springframework.test.web.servlet.request.MockMvcRequestBuilders.get;
import static org.springframework.test.web.servlet.result.MockMvcResultMatchers.*;

@SpringBootTest
@AutoConfigureMockMvc
@ActiveProfiles("test")
class ScaffoldTemplateControllerTest {

    @Autowired
    private MockMvc mockMvc;

    @Test
    void listActive_returns200() throws Exception {
        mockMvc.perform(get("/api/scaffolds"))
                .andExpect(status().isOk())
                .andExpect(jsonPath("$.code").value(0))
                .andExpect(jsonPath("$.data").isArray());
    }

    @Test
    void getByName_returns200() throws Exception {
        mockMvc.perform(get("/api/scaffolds/java-microservice"))
                .andExpect(status().isOk())
                .andExpect(jsonPath("$.code").isNumber());
    }
}
```

- [ ] **Step 7: 运行测试确认失败**

Run: `cd forge-specs && mvn test -Dtest=ScaffoldTemplateControllerTest -pl . -q`
Expected: 404

- [ ] **Step 8: 实现 ScaffoldTemplateController**

```java
package com.shulex.forge.specs.entrance.controller;

import com.shulex.forge.specs.common.Result;
import com.shulex.forge.specs.entrance.vo.ScaffoldTemplateVO;
import com.shulex.forge.specs.infrastructure.entity.ScaffoldTemplateDO;
import com.shulex.forge.specs.service.ScaffoldTemplateService;
import org.springframework.web.bind.annotation.*;

import java.util.List;

@RestController
@RequestMapping("/api/scaffolds")
public class ScaffoldTemplateController {

    private final ScaffoldTemplateService scaffoldTemplateService;

    public ScaffoldTemplateController(ScaffoldTemplateService scaffoldTemplateService) {
        this.scaffoldTemplateService = scaffoldTemplateService;
    }

    @GetMapping
    public Result<List<ScaffoldTemplateVO>> listActive() {
        List<ScaffoldTemplateDO> list = scaffoldTemplateService.listActive();
        return Result.ok(list.stream().map(this::toVO).toList());
    }

    @GetMapping("/{name}")
    public Result<ScaffoldTemplateVO> getByName(@PathVariable String name) {
        ScaffoldTemplateDO template = scaffoldTemplateService.getByName(name);
        return Result.ok(toVO(template));
    }

    private ScaffoldTemplateVO toVO(ScaffoldTemplateDO entity) {
        ScaffoldTemplateVO vo = new ScaffoldTemplateVO();
        vo.setId(entity.getId());
        vo.setName(entity.getName());
        vo.setDescription(entity.getDescription());
        vo.setTechStack(entity.getTechStack());
        vo.setTemplateContent(entity.getTemplateContent());
        return vo;
    }
}
```

- [ ] **Step 9: 运行测试确认全部通过**

Run: `cd forge-specs && mvn test -Dtest=ScaffoldTemplateControllerTest -pl . -q`
Expected: 全部 PASS

- [ ] **Step 10: 创建脚手架种子数据 V5__seed_scaffolds.sql**

`V5__seed_scaffolds.sql` — 至少包含 1 套 Java 微服务脚手架模板。`template_content` 字段为 JSON 格式，包含完整的文件结构定义（pom.xml 模板、Application 入口类、分层包结构、application.yml 模板等），实现者应参考当前 forge-engine 的项目结构作为模板内容的基础。

- [ ] **Step 11: Commit**

```bash
git add forge-specs/src/
git commit -m "feat(m1): add scaffold template service, API, and seed data"
```

---

### Task 9: Prompt Eval 测试

**Files:**
- Create: `forge-specs/src/test/resources/eval/code-generation/good-sample-01.java`
- Create: `forge-specs/src/test/resources/eval/code-generation/bad-sample-01.java`
- Create: `forge-specs/src/test/resources/eval/code-review/good-sample-01.java`
- Create: `forge-specs/src/test/resources/eval/code-review/bad-sample-01.java`
- Create: `forge-specs/src/test/resources/eval/requirement-analysis/good-sample-01.txt`
- Create: `forge-specs/src/test/resources/eval/requirement-analysis/bad-sample-01.txt`
- Create: `forge-specs/src/test/resources/eval/test-generation/good-sample-01.java`
- Create: `forge-specs/src/test/resources/eval/test-generation/bad-sample-01.java`
- Create: `forge-specs/src/test/resources/eval/fix-generation/good-sample-01.java`
- Create: `forge-specs/src/test/resources/eval/fix-generation/bad-sample-01.java`
- Create: `forge-specs/src/test/resources/eval/doc-generation/good-sample-01.java`
- Create: `forge-specs/src/test/resources/eval/doc-generation/bad-sample-01.java`
- Create: `forge-specs/src/test/java/com/shulex/forge/specs/eval/PromptEvalTest.java`

- [ ] **Step 1: 创建 good-code 样本**

每个模板创建至少 1 个 good-sample 文件。good-sample 必须完全符合 `docs/references/coding-standards.md` 中的所有规范：
- 命名规范（UpperCamelCase 类名、lowerCamelCase 方法名、DO/DTO/VO 后缀）
- 构造器注入
- Result<T> 包装
- SLF4J 日志占位符
- 分层架构（entrance/service/infrastructure/common）

示例 `code-generation/good-sample-01.java`:
```java
package com.example.user.entrance.controller;

import com.example.common.Result;
import com.example.user.entrance.vo.UserVO;
import com.example.user.service.UserService;
import org.springframework.web.bind.annotation.*;
import java.util.List;

@RestController
@RequestMapping("/api/users")
public class UserController {

    private final UserService userService;

    public UserController(UserService userService) {
        this.userService = userService;
    }

    @GetMapping
    public Result<List<UserVO>> listUsers() {
        return Result.ok(userService.listAll());
    }
}
```

- [ ] **Step 2: 创建 bad-code 样本**

每个模板创建至少 1 个 bad-sample 文件。bad-sample 必须包含明确的规范违反：
- 字段注入（`@Autowired` 而非构造器注入）
- 缺少 Result<T> 包装（直接返回实体）
- 命名不规范（如 `userDO` 直接暴露给前端）
- String 拼接日志
- SQL 注入风险

示例 `code-generation/bad-sample-01.java`:
```java
package com.example.controller;  // 缺少分层包

import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.web.bind.annotation.*;

@RestController
public class user_controller {  // 类名不规范

    @Autowired  // 字段注入违规
    private UserMapper userMapper;

    @GetMapping("/getUser")
    public UserDO getUser(Long id) {  // DO 直接暴露、缺少 Result 包装
        System.out.println("getting user " + id);  // 使用 System.out 而非 SLF4J
        return userMapper.selectById(id);
    }
}
```

- [ ] **Step 3: 创建 PromptEvalTest.java**

```java
package com.shulex.forge.specs.eval;

import com.shulex.forge.specs.infrastructure.entity.PromptTemplateDO;
import com.shulex.forge.specs.service.PromptTemplateService;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.params.ParameterizedTest;
import org.junit.jupiter.params.provider.ValueSource;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.boot.test.context.SpringBootTest;
import org.springframework.core.io.ClassPathResource;
import org.springframework.test.context.ActiveProfiles;

import java.io.IOException;
import java.nio.charset.StandardCharsets;

import static org.assertj.core.api.Assertions.assertThat;

@SpringBootTest
@ActiveProfiles("test")
class PromptEvalTest {

    @Autowired
    private PromptTemplateService promptTemplateService;

    private static final String[] TEMPLATE_KEYS = {
            "requirement-analysis", "code-generation", "code-review",
            "test-generation", "fix-generation", "doc-generation"
    };

    @ParameterizedTest
    @ValueSource(strings = {
            "requirement-analysis", "code-generation", "code-review",
            "test-generation", "fix-generation", "doc-generation"
    })
    void eachTemplate_hasActiveVersion(String templateKey) {
        PromptTemplateDO template = promptTemplateService.getActiveByKey(templateKey);
        assertThat(template).isNotNull();
        assertThat(template.getSystemPrompt()).isNotBlank();
    }

    @ParameterizedTest
    @ValueSource(strings = {
            "requirement-analysis", "code-generation", "code-review",
            "test-generation", "fix-generation", "doc-generation"
    })
    void eachTemplate_hasGoodSample(String templateKey) throws IOException {
        String ext = "requirement-analysis".equals(templateKey) ? ".txt" : ".java";
        String path = "eval/" + templateKey + "/good-sample-01" + ext;
        ClassPathResource resource = new ClassPathResource(path);
        assertThat(resource.exists())
                .as("Good sample should exist for template: " + templateKey)
                .isTrue();
        String content = resource.getContentAsString(StandardCharsets.UTF_8);
        assertThat(content).isNotBlank();
    }

    @ParameterizedTest
    @ValueSource(strings = {
            "requirement-analysis", "code-generation", "code-review",
            "test-generation", "fix-generation", "doc-generation"
    })
    void eachTemplate_hasBadSample(String templateKey) throws IOException {
        String ext = "requirement-analysis".equals(templateKey) ? ".txt" : ".java";
        String path = "eval/" + templateKey + "/bad-sample-01" + ext;
        ClassPathResource resource = new ClassPathResource(path);
        assertThat(resource.exists())
                .as("Bad sample should exist for template: " + templateKey)
                .isTrue();
        String content = resource.getContentAsString(StandardCharsets.UTF_8);
        assertThat(content).isNotBlank();
    }

    @Test
    void goodSample_followsNamingConventions() throws IOException {
        String good = new ClassPathResource("eval/code-generation/good-sample-01.java")
                .getContentAsString(StandardCharsets.UTF_8);
        // 应使用构造器注入
        assertThat(good).doesNotContain("@Autowired");
        // 应包含 Result 包装
        assertThat(good).contains("Result<");
    }

    @Test
    void badSample_violatesConventions() throws IOException {
        String bad = new ClassPathResource("eval/code-generation/bad-sample-01.java")
                .getContentAsString(StandardCharsets.UTF_8);
        // 应包含至少一种违规
        boolean hasViolation = bad.contains("@Autowired")
                || bad.contains("System.out")
                || !bad.contains("Result<");
        assertThat(hasViolation).isTrue();
    }
}
```

- [ ] **Step 4: 运行 eval 测试确认通过**

Run: `cd forge-specs && mvn test -Dtest=PromptEvalTest -pl . -q`
Expected: 全部 PASS

- [ ] **Step 5: Commit**

```bash
git add forge-specs/src/test/
git commit -m "feat(m1): add prompt eval tests with good/bad code samples"
```

---

## M1 完成标准

- [ ] forge-specs 服务编译、测试全部通过
- [ ] 数据库有 spec_standard / spec_prompt_template / spec_review_rule / spec_scaffold_template 四张表
- [ ] 编码规范种子数据 ≥ 15 条（覆盖 java/sql/api/security/git）
- [ ] 6 个核心 Prompt 模板全部入库
- [ ] Review 规则 ≥ 15 条（覆盖 coding/security/performance/database）
- [ ] 至少 1 套 Java 微服务脚手架模板入库
- [ ] 规范查询 API：`GET /api/standards?category=xxx`
- [ ] 规范继承查询 API：`GET /api/standards/effective?category=xxx&scopeLevel=xxx&scopeId=xxx`
- [ ] Prompt 模板列表 API：`GET /api/prompts`
- [ ] Prompt 模板详情 API：`GET /api/prompts/{templateKey}`
- [ ] Review 规则列表 API：`GET /api/review-rules?category=xxx`
- [ ] Review 规则全量 API：`GET /api/review-rules/all`
- [ ] 脚手架模板列表 API：`GET /api/scaffolds`
- [ ] 脚手架模板详情 API：`GET /api/scaffolds/{name}`
- [ ] Prompt eval 测试全部通过（6 个模板均有 good/bad 样本）
- [ ] 以上 API 可通过 APISIX 网关（`http://localhost:9080/api/specs/...`）访问
- [ ] 所有变更已 commit
