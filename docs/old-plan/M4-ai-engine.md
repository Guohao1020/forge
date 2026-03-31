# M4 — AI 引擎 实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 构建 forge-engine AI 引擎，实现从需求输入到代码生成、AI Review、代码提交到 Codeup 分支的完整闭环，包含状态机驱动的任务编排、Kafka 任务通道、Claude 模型接入、两阶段风险评估、Token 用量追踪、三级紧急停止开关。

**Architecture:** forge-engine 作为单体服务（端口 8081），编排层和执行层在同一进程内通过 Kafka 解耦。编排层接收需求、驱动状态机、调度任务步骤；执行层消费 Kafka 消息、构建上下文、调用 Claude API 执行代码生成/Review/修复。跨服务调用 forge-specs（规范查询）和 forge-pipeline（代码托管适配器）通过 HTTP。状态持久化到 MySQL，缓存到 Redis。

**Tech Stack:** Java 17, Spring Boot 3.2, Spring Kafka, OkHttp, JJWT (验证), MyBatis Plus 3.5.5, Flyway, Redis, H2 (test), Kafka (docker)

**M4 轻量版简化说明：**
- 单模型 Claude（不做多模型路由/熔断/fallback）
- 串行代码生成（不做并行 Phase B）
- 简化上下文构建（不做 RAG/向量检索）
- 编排和执行在同一 JVM（不做 Leader 选举）
- 不做 Worker Pool / HPA（单实例执行）

---

## 文件结构总览

```
forge-engine/
├── pom.xml                                              ← 补充依赖
├── src/main/java/com/shulex/forge/engine/
│   ├── ForgeEngineApplication.java                      ← 已有
│   ├── common/
│   │   ├── Result.java                                  ← 统一响应
│   │   ├── ErrorCode.java                               ← 错误码枚举
│   │   ├── BizException.java                            ← 业务异常
│   │   ├── SysException.java                            ← 系统异常
│   │   └── GlobalExceptionHandler.java                  ← 全局异常处理
│   ├── infrastructure/
│   │   ├── entity/
│   │   │   ├── TaskDO.java                              ← 任务主表
│   │   │   ├── TaskStepDO.java                          ← 任务步骤表
│   │   │   ├── ModelCallLogDO.java                      ← 模型调用日志
│   │   │   └── CodeChangeDO.java                        ← 代码变更记录
│   │   ├── mapper/
│   │   │   ├── TaskMapper.java
│   │   │   ├── TaskStepMapper.java
│   │   │   ├── ModelCallLogMapper.java
│   │   │   └── CodeChangeMapper.java
│   │   ├── config/
│   │   │   ├── MyBatisPlusConfig.java                   ← 时间自动填充
│   │   │   └── KafkaConfig.java                         ← Kafka topic 配置
│   │   └── http/
│   │       ├── SpecsClient.java                         ← 调用 forge-specs API
│   │       └── PipelineClient.java                      ← 调用 forge-pipeline 适配器 API
│   ├── orchestration/
│   │   ├── model/
│   │   │   ├── TaskStatus.java                          ← 状态枚举
│   │   │   ├── StepType.java                            ← 步骤类型枚举
│   │   │   ├── StepStatus.java                          ← 步骤状态枚举
│   │   │   ├── RiskLevel.java                           ← 风险等级枚举
│   │   │   └── KillSwitchLevel.java                     ← 紧急停止等级
│   │   ├── statemachine/
│   │   │   └── TaskStateMachine.java                    ← 状态机转换逻辑
│   │   ├── service/
│   │   │   ├── TaskService.java                         ← 任务 CRUD + 状态驱动
│   │   │   ├── TaskDispatcher.java                      ← 步骤调度 + Kafka 发送
│   │   │   ├── RiskAssessor.java                        ← 风险评估（初评+终评）
│   │   │   ├── KillSwitchService.java                   ← 紧急停止管理
│   │   │   └── TokenUsageService.java                   ← Token 用量追踪
│   │   └── listener/
│   │       └── StepResultListener.java                  ← Kafka 消费步骤结果
│   ├── execution/
│   │   ├── model/
│   │   │   ├── StepRequest.java                         ← 步骤执行请求
│   │   │   ├── StepResult.java                          ← 步骤执行结果
│   │   │   └── GeneratedCode.java                       ← 生成的代码文件
│   │   ├── service/
│   │   │   ├── StepExecutor.java                        ← 步骤分发执行
│   │   │   ├── ContextBuilder.java                      ← 上下文构建
│   │   │   ├── CodeGenerator.java                       ← 代码生成
│   │   │   ├── CodeReviewer.java                        ← AI Review
│   │   │   ├── CodeFixer.java                           ← 代码修复
│   │   │   └── CodeCommitter.java                       ← 代码提交到 Codeup
│   │   ├── ai/
│   │   │   ├── ClaudeClient.java                        ← Claude API 客户端
│   │   │   ├── ClaudeConfig.java                        ← Claude 配置
│   │   │   └── AiResponse.java                          ← AI 响应模型
│   │   └── listener/
│   │       └── StepRequestListener.java                 ← Kafka 消费步骤请求
│   └── entrance/
│       ├── controller/
│       │   ├── TaskController.java                      ← 任务 API
│       │   ├── KillSwitchController.java                ← 紧急停止 API
│       │   └── TokenUsageController.java                ← Token 用量查询 API
│       └── vo/
│           ├── CreateTaskRequest.java                   ← 创建任务请求
│           ├── TaskVO.java                              ← 任务视图
│           ├── TaskStepVO.java                          ← 步骤视图
│           └── TokenUsageVO.java                        ← Token 用量视图
├── src/main/resources/
│   ├── application.yml                                  ← 更新配置
│   └── db/migration/
│       └── V1__init_engine_tables.sql                   ← 建表
├── src/test/java/com/shulex/forge/engine/
│   ├── orchestration/
│   │   ├── statemachine/TaskStateMachineTest.java
│   │   └── service/
│   │       ├── TaskServiceTest.java
│   │       ├── RiskAssessorTest.java
│   │       └── KillSwitchServiceTest.java
│   ├── execution/
│   │   ├── ai/ClaudeClientTest.java
│   │   ├── service/
│   │   │   ├── ContextBuilderTest.java
│   │   │   ├── CodeGeneratorTest.java
│   │   │   └── CodeReviewerTest.java
│   └── entrance/
│       └── controller/TaskControllerTest.java
└── src/test/resources/
    ├── application-test.yml
    └── db/test-migration/
        ├── V1__init_engine_tables.sql
        └── V2__seed_test_data.sql
```

---

### Task 1: 补充依赖 + 公共基础设施 + 数据库建表

**Files:**
- Modify: `forge-engine/pom.xml`
- Create: `forge-engine/src/main/java/com/shulex/forge/engine/common/Result.java`
- Create: `forge-engine/src/main/java/com/shulex/forge/engine/common/ErrorCode.java`
- Create: `forge-engine/src/main/java/com/shulex/forge/engine/common/BizException.java`
- Create: `forge-engine/src/main/java/com/shulex/forge/engine/common/SysException.java`
- Create: `forge-engine/src/main/java/com/shulex/forge/engine/common/GlobalExceptionHandler.java`
- Create: `forge-engine/src/main/resources/db/migration/V1__init_engine_tables.sql`

- [ ] **Step 1: 更新 pom.xml 添加依赖**

```xml
<!-- 在 <dependencies> 中追加 -->
<!-- Kafka -->
<dependency>
    <groupId>org.springframework.kafka</groupId>
    <artifactId>spring-kafka</artifactId>
</dependency>
<!-- HTTP Client -->
<dependency>
    <groupId>com.squareup.okhttp3</groupId>
    <artifactId>okhttp</artifactId>
</dependency>
<!-- JSON -->
<dependency>
    <groupId>com.fasterxml.jackson.core</groupId>
    <artifactId>jackson-databind</artifactId>
</dependency>
<dependency>
    <groupId>com.fasterxml.jackson.datatype</groupId>
    <artifactId>jackson-datatype-jsr310</artifactId>
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
    <groupId>org.springframework.kafka</groupId>
    <artifactId>spring-kafka-test</artifactId>
    <scope>test</scope>
</dependency>
<dependency>
    <groupId>com.squareup.okhttp3</groupId>
    <artifactId>mockwebserver</artifactId>
    <scope>test</scope>
</dependency>
```

- [ ] **Step 2: 创建 Result.java**

```java
package com.shulex.forge.engine.common;

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
package com.shulex.forge.engine.common;

import lombok.Getter;
import lombok.AllArgsConstructor;

@Getter
@AllArgsConstructor
public enum ErrorCode {
    TASK_NOT_FOUND(40400, "任务不存在"),
    TASK_INVALID_STATUS(40001, "任务状态不允许此操作"),
    STEP_NOT_FOUND(40401, "步骤不存在"),
    KILL_SWITCH_ACTIVE(40301, "紧急停止已激活，无法创建新任务"),
    INVALID_PARAM(40000, "参数错误"),
    AI_CALL_FAILED(50001, "AI 模型调用失败"),
    CODE_COMMIT_FAILED(50002, "代码提交失败"),
    CONTEXT_BUILD_FAILED(50003, "上下文构建失败"),
    INTERNAL_ERROR(50000, "系统内部错误");

    private final int code;
    private final String message;
}
```

- [ ] **Step 4: 创建 BizException.java**

```java
package com.shulex.forge.engine.common;

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
package com.shulex.forge.engine.common;

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
package com.shulex.forge.engine.common;

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

- [ ] **Step 7: 创建 V1__init_engine_tables.sql**

```sql
-- 任务主表
CREATE TABLE engine_task (
    id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY COMMENT '任务ID',
    tenant_id BIGINT UNSIGNED NOT NULL COMMENT '租户ID',
    user_id BIGINT UNSIGNED NOT NULL COMMENT '用户ID',
    requirement TEXT NOT NULL COMMENT '需求描述',
    task_type VARCHAR(32) NOT NULL DEFAULT 'GENERATE' COMMENT '任务类型: GENERATE/ITERATE',
    status VARCHAR(32) NOT NULL DEFAULT 'SUBMITTED' COMMENT '状态',
    risk_level VARCHAR(16) DEFAULT NULL COMMENT '风险等级: LOW/MEDIUM/HIGH',
    repo_id VARCHAR(128) DEFAULT NULL COMMENT '仓库ID',
    branch_name VARCHAR(128) DEFAULT NULL COMMENT '工作分支',
    mr_id BIGINT DEFAULT NULL COMMENT 'MR ID',
    review_score INT DEFAULT NULL COMMENT 'Review 评分 0-100',
    total_input_tokens BIGINT NOT NULL DEFAULT 0 COMMENT '累计输入 Token',
    total_output_tokens BIGINT NOT NULL DEFAULT 0 COMMENT '累计输出 Token',
    error_message TEXT DEFAULT NULL COMMENT '错误信息',
    gmt_create DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    gmt_modified DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '修改时间',
    INDEX idx_tenant_user (tenant_id, user_id),
    INDEX idx_status (status)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin COMMENT='任务主表';

-- 任务步骤表
CREATE TABLE engine_task_step (
    id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY COMMENT '步骤ID',
    task_id BIGINT UNSIGNED NOT NULL COMMENT '任务ID',
    step_type VARCHAR(32) NOT NULL COMMENT '步骤类型',
    step_order INT NOT NULL COMMENT '步骤序号',
    status VARCHAR(32) NOT NULL DEFAULT 'PENDING' COMMENT '状态',
    input_snapshot TEXT DEFAULT NULL COMMENT '输入快照(JSON)',
    output_snapshot TEXT DEFAULT NULL COMMENT '输出快照(JSON)',
    input_tokens BIGINT NOT NULL DEFAULT 0 COMMENT '输入 Token',
    output_tokens BIGINT NOT NULL DEFAULT 0 COMMENT '输出 Token',
    retry_count INT NOT NULL DEFAULT 0 COMMENT '重试次数',
    error_message TEXT DEFAULT NULL COMMENT '错误信息',
    gmt_create DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    gmt_modified DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '修改时间',
    INDEX idx_task_id (task_id),
    INDEX idx_status (status)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin COMMENT='任务步骤表';

-- 模型调用日志（append-only）
CREATE TABLE engine_model_call_log (
    id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY COMMENT 'ID',
    task_id BIGINT UNSIGNED NOT NULL COMMENT '任务ID',
    step_id BIGINT UNSIGNED DEFAULT NULL COMMENT '步骤ID',
    model_id VARCHAR(64) NOT NULL COMMENT '模型ID',
    purpose VARCHAR(32) NOT NULL COMMENT '用途: ANALYZE/GENERATE/REVIEW/FIX',
    input_tokens BIGINT NOT NULL DEFAULT 0 COMMENT '输入 Token',
    output_tokens BIGINT NOT NULL DEFAULT 0 COMMENT '输出 Token',
    latency_ms BIGINT NOT NULL DEFAULT 0 COMMENT '延迟(ms)',
    is_fallback TINYINT NOT NULL DEFAULT 0 COMMENT '是否降级',
    gmt_create DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    INDEX idx_task_id (task_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin COMMENT='模型调用日志';

-- 代码变更记录
CREATE TABLE engine_code_change (
    id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY COMMENT 'ID',
    task_id BIGINT UNSIGNED NOT NULL COMMENT '任务ID',
    repo_id VARCHAR(128) NOT NULL COMMENT '仓库ID',
    branch_name VARCHAR(128) NOT NULL COMMENT '分支名',
    commit_hash VARCHAR(64) DEFAULT NULL COMMENT 'commit hash',
    file_count INT NOT NULL DEFAULT 0 COMMENT '变更文件数',
    review_score INT DEFAULT NULL COMMENT 'Review 评分',
    mr_id BIGINT DEFAULT NULL COMMENT 'MR ID',
    mr_status VARCHAR(32) DEFAULT NULL COMMENT 'MR 状态',
    gmt_create DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    gmt_modified DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '修改时间',
    INDEX idx_task_id (task_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin COMMENT='代码变更记录';
```

- [ ] **Step 8: 编译验证**

Run: `cd forge-engine && mvn clean compile -q`
Expected: 编译通过

- [ ] **Step 9: Commit**

```bash
git add forge-engine/pom.xml forge-engine/src/main/java/com/shulex/forge/engine/common/ forge-engine/src/main/resources/db/
git commit -m "feat(m4): add forge-engine dependencies, common infrastructure, and schema"
```

---

### Task 2: 实体层 + Mapper + 配置 + 测试基础设施

**Files:**
- Create: `forge-engine/src/main/java/com/shulex/forge/engine/infrastructure/entity/TaskDO.java`
- Create: `forge-engine/src/main/java/com/shulex/forge/engine/infrastructure/entity/TaskStepDO.java`
- Create: `forge-engine/src/main/java/com/shulex/forge/engine/infrastructure/entity/ModelCallLogDO.java`
- Create: `forge-engine/src/main/java/com/shulex/forge/engine/infrastructure/entity/CodeChangeDO.java`
- Create: `forge-engine/src/main/java/com/shulex/forge/engine/infrastructure/mapper/TaskMapper.java`
- Create: `forge-engine/src/main/java/com/shulex/forge/engine/infrastructure/mapper/TaskStepMapper.java`
- Create: `forge-engine/src/main/java/com/shulex/forge/engine/infrastructure/mapper/ModelCallLogMapper.java`
- Create: `forge-engine/src/main/java/com/shulex/forge/engine/infrastructure/mapper/CodeChangeMapper.java`
- Create: `forge-engine/src/main/java/com/shulex/forge/engine/infrastructure/config/MyBatisPlusConfig.java`
- Create: `forge-engine/src/main/java/com/shulex/forge/engine/infrastructure/config/KafkaConfig.java`
- Create: `forge-engine/src/test/resources/application-test.yml`
- Create: `forge-engine/src/test/resources/db/test-migration/V1__init_engine_tables.sql`
- Create: `forge-engine/src/test/resources/db/test-migration/V2__seed_test_data.sql`

- [ ] **Step 1: 创建 TaskDO.java**

```java
package com.shulex.forge.engine.infrastructure.entity;

import com.baomidou.mybatisplus.annotation.*;
import lombok.Data;
import java.time.LocalDateTime;

@Data
@TableName("engine_task")
public class TaskDO {
    @TableId(type = IdType.AUTO)
    private Long id;
    private Long tenantId;
    private Long userId;
    private String requirement;
    private String taskType;
    private String status;
    private String riskLevel;
    private String repoId;
    private String branchName;
    private Long mrId;
    private Integer reviewScore;
    private Long totalInputTokens;
    private Long totalOutputTokens;
    private String errorMessage;
    @TableField(fill = FieldFill.INSERT)
    private LocalDateTime gmtCreate;
    @TableField(fill = FieldFill.INSERT_UPDATE)
    private LocalDateTime gmtModified;
}
```

- [ ] **Step 2: 创建 TaskStepDO.java**

```java
package com.shulex.forge.engine.infrastructure.entity;

import com.baomidou.mybatisplus.annotation.*;
import lombok.Data;
import java.time.LocalDateTime;

@Data
@TableName("engine_task_step")
public class TaskStepDO {
    @TableId(type = IdType.AUTO)
    private Long id;
    private Long taskId;
    private String stepType;
    private Integer stepOrder;
    private String status;
    private String inputSnapshot;
    private String outputSnapshot;
    private Long inputTokens;
    private Long outputTokens;
    private Integer retryCount;
    private String errorMessage;
    @TableField(fill = FieldFill.INSERT)
    private LocalDateTime gmtCreate;
    @TableField(fill = FieldFill.INSERT_UPDATE)
    private LocalDateTime gmtModified;
}
```

- [ ] **Step 3: 创建 ModelCallLogDO.java**

```java
package com.shulex.forge.engine.infrastructure.entity;

import com.baomidou.mybatisplus.annotation.*;
import lombok.Data;
import java.time.LocalDateTime;

@Data
@TableName("engine_model_call_log")
public class ModelCallLogDO {
    @TableId(type = IdType.AUTO)
    private Long id;
    private Long taskId;
    private Long stepId;
    private String modelId;
    private String purpose;
    private Long inputTokens;
    private Long outputTokens;
    private Long latencyMs;
    private Integer isFallback;
    @TableField(fill = FieldFill.INSERT)
    private LocalDateTime gmtCreate;
}
```

- [ ] **Step 4: 创建 CodeChangeDO.java**

```java
package com.shulex.forge.engine.infrastructure.entity;

import com.baomidou.mybatisplus.annotation.*;
import lombok.Data;
import java.time.LocalDateTime;

@Data
@TableName("engine_code_change")
public class CodeChangeDO {
    @TableId(type = IdType.AUTO)
    private Long id;
    private Long taskId;
    private String repoId;
    private String branchName;
    private String commitHash;
    private Integer fileCount;
    private Integer reviewScore;
    private Long mrId;
    private String mrStatus;
    @TableField(fill = FieldFill.INSERT)
    private LocalDateTime gmtCreate;
    @TableField(fill = FieldFill.INSERT_UPDATE)
    private LocalDateTime gmtModified;
}
```

- [ ] **Step 5: 创建 4 个 Mapper 接口**

```java
// TaskMapper.java
package com.shulex.forge.engine.infrastructure.mapper;
import com.baomidou.mybatisplus.core.mapper.BaseMapper;
import com.shulex.forge.engine.infrastructure.entity.TaskDO;
import org.apache.ibatis.annotations.Mapper;
@Mapper
public interface TaskMapper extends BaseMapper<TaskDO> {}

// TaskStepMapper.java
package com.shulex.forge.engine.infrastructure.mapper;
import com.baomidou.mybatisplus.core.mapper.BaseMapper;
import com.shulex.forge.engine.infrastructure.entity.TaskStepDO;
import org.apache.ibatis.annotations.Mapper;
@Mapper
public interface TaskStepMapper extends BaseMapper<TaskStepDO> {}

// ModelCallLogMapper.java
package com.shulex.forge.engine.infrastructure.mapper;
import com.baomidou.mybatisplus.core.mapper.BaseMapper;
import com.shulex.forge.engine.infrastructure.entity.ModelCallLogDO;
import org.apache.ibatis.annotations.Mapper;
@Mapper
public interface ModelCallLogMapper extends BaseMapper<ModelCallLogDO> {}

// CodeChangeMapper.java
package com.shulex.forge.engine.infrastructure.mapper;
import com.baomidou.mybatisplus.core.mapper.BaseMapper;
import com.shulex.forge.engine.infrastructure.entity.CodeChangeDO;
import org.apache.ibatis.annotations.Mapper;
@Mapper
public interface CodeChangeMapper extends BaseMapper<CodeChangeDO> {}
```

- [ ] **Step 6: 创建 MyBatisPlusConfig.java**

```java
package com.shulex.forge.engine.infrastructure.config;

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

- [ ] **Step 7: 创建 KafkaConfig.java**

```java
package com.shulex.forge.engine.infrastructure.config;

import org.apache.kafka.clients.admin.NewTopic;
import org.springframework.context.annotation.Bean;
import org.springframework.context.annotation.Configuration;

@Configuration
public class KafkaConfig {

    public static final String TOPIC_STEP_REQUEST = "forge-engine-step-request";
    public static final String TOPIC_STEP_RESULT = "forge-engine-step-result";

    @Bean
    public NewTopic stepRequestTopic() {
        return new NewTopic(TOPIC_STEP_REQUEST, 3, (short) 1);
    }

    @Bean
    public NewTopic stepResultTopic() {
        return new NewTopic(TOPIC_STEP_RESULT, 3, (short) 1);
    }
}
```

- [ ] **Step 8: 创建 application-test.yml**

```yaml
spring:
  datasource:
    url: jdbc:h2:mem:forge_engine_test;MODE=MYSQL;DB_CLOSE_DELAY=-1
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
  kafka:
    bootstrap-servers: localhost:9092
    consumer:
      auto-offset-reset: earliest
      group-id: forge-engine-test

forge:
  claude:
    api-key: test-api-key
    model: claude-sonnet-4-20250514
    base-url: https://api.anthropic.com
  specs:
    base-url: http://localhost:8084
  pipeline:
    base-url: http://localhost:8083
```

- [ ] **Step 9: 创建 H2 兼容 test-migration/V1__init_engine_tables.sql**

```sql
CREATE TABLE engine_task (
    id BIGINT AUTO_INCREMENT PRIMARY KEY,
    tenant_id BIGINT NOT NULL,
    user_id BIGINT NOT NULL,
    requirement CLOB NOT NULL,
    task_type VARCHAR(32) NOT NULL DEFAULT 'GENERATE',
    status VARCHAR(32) NOT NULL DEFAULT 'SUBMITTED',
    risk_level VARCHAR(16) DEFAULT NULL,
    repo_id VARCHAR(128) DEFAULT NULL,
    branch_name VARCHAR(128) DEFAULT NULL,
    mr_id BIGINT DEFAULT NULL,
    review_score INT DEFAULT NULL,
    total_input_tokens BIGINT NOT NULL DEFAULT 0,
    total_output_tokens BIGINT NOT NULL DEFAULT 0,
    error_message CLOB DEFAULT NULL,
    gmt_create DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    gmt_modified DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX idx_task_tenant_user ON engine_task(tenant_id, user_id);
CREATE INDEX idx_task_status ON engine_task(status);

CREATE TABLE engine_task_step (
    id BIGINT AUTO_INCREMENT PRIMARY KEY,
    task_id BIGINT NOT NULL,
    step_type VARCHAR(32) NOT NULL,
    step_order INT NOT NULL,
    status VARCHAR(32) NOT NULL DEFAULT 'PENDING',
    input_snapshot CLOB DEFAULT NULL,
    output_snapshot CLOB DEFAULT NULL,
    input_tokens BIGINT NOT NULL DEFAULT 0,
    output_tokens BIGINT NOT NULL DEFAULT 0,
    retry_count INT NOT NULL DEFAULT 0,
    error_message CLOB DEFAULT NULL,
    gmt_create DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    gmt_modified DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX idx_step_task_id ON engine_task_step(task_id);

CREATE TABLE engine_model_call_log (
    id BIGINT AUTO_INCREMENT PRIMARY KEY,
    task_id BIGINT NOT NULL,
    step_id BIGINT DEFAULT NULL,
    model_id VARCHAR(64) NOT NULL,
    purpose VARCHAR(32) NOT NULL,
    input_tokens BIGINT NOT NULL DEFAULT 0,
    output_tokens BIGINT NOT NULL DEFAULT 0,
    latency_ms BIGINT NOT NULL DEFAULT 0,
    is_fallback TINYINT NOT NULL DEFAULT 0,
    gmt_create DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX idx_call_log_task ON engine_model_call_log(task_id);

CREATE TABLE engine_code_change (
    id BIGINT AUTO_INCREMENT PRIMARY KEY,
    task_id BIGINT NOT NULL,
    repo_id VARCHAR(128) NOT NULL,
    branch_name VARCHAR(128) NOT NULL,
    commit_hash VARCHAR(64) DEFAULT NULL,
    file_count INT NOT NULL DEFAULT 0,
    review_score INT DEFAULT NULL,
    mr_id BIGINT DEFAULT NULL,
    mr_status VARCHAR(32) DEFAULT NULL,
    gmt_create DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    gmt_modified DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX idx_code_change_task ON engine_code_change(task_id);
```

- [ ] **Step 10: 创建 test-migration/V2__seed_test_data.sql**

```sql
-- 测试任务
INSERT INTO engine_task (tenant_id, user_id, requirement, task_type, status, repo_id)
VALUES (1, 1, '创建一个用户管理服务', 'GENERATE', 'SUBMITTED', 'test-repo-123');
```

- [ ] **Step 11: 编译验证**

Run: `cd forge-engine && mvn clean compile -q`
Expected: 编译通过

- [ ] **Step 12: Commit**

```bash
git add forge-engine/src/
git commit -m "feat(m4): add entity layer, mappers, Kafka config, and test migrations"
```

---

### Task 3: 编排层 — 状态机 + 枚举 + 测试

**Files:**
- Create: `forge-engine/src/main/java/com/shulex/forge/engine/orchestration/model/TaskStatus.java`
- Create: `forge-engine/src/main/java/com/shulex/forge/engine/orchestration/model/StepType.java`
- Create: `forge-engine/src/main/java/com/shulex/forge/engine/orchestration/model/StepStatus.java`
- Create: `forge-engine/src/main/java/com/shulex/forge/engine/orchestration/model/RiskLevel.java`
- Create: `forge-engine/src/main/java/com/shulex/forge/engine/orchestration/model/KillSwitchLevel.java`
- Create: `forge-engine/src/main/java/com/shulex/forge/engine/orchestration/statemachine/TaskStateMachine.java`
- Create: `forge-engine/src/test/java/com/shulex/forge/engine/orchestration/statemachine/TaskStateMachineTest.java`

- [ ] **Step 1: 写 TaskStateMachineTest**

```java
package com.shulex.forge.engine.orchestration.statemachine;

import com.shulex.forge.engine.orchestration.model.TaskStatus;
import org.junit.jupiter.api.Test;
import static org.assertj.core.api.Assertions.assertThat;
import static org.assertj.core.api.Assertions.assertThatThrownBy;

class TaskStateMachineTest {

    @Test
    void transition_submittedToAnalyzing() {
        assertThat(TaskStateMachine.transition(TaskStatus.SUBMITTED, TaskStatus.ANALYZING)).isTrue();
    }

    @Test
    void transition_analyzingToPlanning() {
        assertThat(TaskStateMachine.transition(TaskStatus.ANALYZING, TaskStatus.PLANNING)).isTrue();
    }

    @Test
    void transition_fullHappyPath() {
        TaskStatus[] path = {
            TaskStatus.SUBMITTED, TaskStatus.ANALYZING, TaskStatus.PLANNING,
            TaskStatus.GENERATING, TaskStatus.REVIEWING, TaskStatus.DEPLOYING, TaskStatus.DONE
        };
        for (int i = 0; i < path.length - 1; i++) {
            assertThat(TaskStateMachine.transition(path[i], path[i + 1])).isTrue();
        }
    }

    @Test
    void transition_reviewingToHumanReview() {
        assertThat(TaskStateMachine.transition(TaskStatus.REVIEWING, TaskStatus.HUMAN_REVIEW)).isTrue();
    }

    @Test
    void transition_invalidTransitionReturnsFalse() {
        assertThat(TaskStateMachine.transition(TaskStatus.DONE, TaskStatus.SUBMITTED)).isFalse();
    }

    @Test
    void transition_anyToFailed() {
        assertThat(TaskStateMachine.transition(TaskStatus.GENERATING, TaskStatus.FAILED)).isTrue();
        assertThat(TaskStateMachine.transition(TaskStatus.REVIEWING, TaskStatus.FAILED)).isTrue();
    }

    @Test
    void transition_anyToCancelled() {
        assertThat(TaskStateMachine.transition(TaskStatus.PLANNING, TaskStatus.CANCELLED)).isTrue();
    }
}
```

- [ ] **Step 2: 运行测试确认失败**

Run: `cd forge-engine && mvn test -Dtest=TaskStateMachineTest -pl . 2>&1 | tail -20`
Expected: 编译失败

- [ ] **Step 3: 创建所有枚举**

```java
// TaskStatus.java
package com.shulex.forge.engine.orchestration.model;

public enum TaskStatus {
    SUBMITTED, ANALYZING, PLANNING, GENERATING, REVIEWING,
    HUMAN_REVIEW, APPROVED, REJECTED, DEPLOYING, DONE,
    FAILED, CANCELLED
}

// StepType.java
package com.shulex.forge.engine.orchestration.model;

public enum StepType {
    ANALYZE,           // 需求分析
    PLAN,              // 方案规划
    RISK_ASSESS_INIT,  // 初步风险评估
    GENERATE_CONTRACT, // 契约生成
    GENERATE_CODE,     // 代码生成
    REVIEW,            // AI Review
    FIX,               // 代码修复
    RISK_ASSESS_FINAL, // 终评风险
    COMMIT,            // 代码提交
    CREATE_MR          // 创建 MR
}

// StepStatus.java
package com.shulex.forge.engine.orchestration.model;

public enum StepStatus {
    PENDING, RUNNING, SUCCESS, FAILED, SKIPPED
}

// RiskLevel.java
package com.shulex.forge.engine.orchestration.model;

public enum RiskLevel {
    LOW, MEDIUM, HIGH
}

// KillSwitchLevel.java
package com.shulex.forge.engine.orchestration.model;

public enum KillSwitchLevel {
    NONE,  // 正常
    L1,    // 暂停提交
    L2,    // 冻结引擎
    L3     // 全面停机
}
```

- [ ] **Step 4: 实现 TaskStateMachine**

```java
package com.shulex.forge.engine.orchestration.statemachine;

import com.shulex.forge.engine.orchestration.model.TaskStatus;
import java.util.Map;
import java.util.Set;

public class TaskStateMachine {

    private static final Map<TaskStatus, Set<TaskStatus>> TRANSITIONS = Map.ofEntries(
        Map.entry(TaskStatus.SUBMITTED, Set.of(TaskStatus.ANALYZING, TaskStatus.FAILED, TaskStatus.CANCELLED)),
        Map.entry(TaskStatus.ANALYZING, Set.of(TaskStatus.PLANNING, TaskStatus.FAILED, TaskStatus.CANCELLED)),
        Map.entry(TaskStatus.PLANNING, Set.of(TaskStatus.GENERATING, TaskStatus.FAILED, TaskStatus.CANCELLED)),
        Map.entry(TaskStatus.GENERATING, Set.of(TaskStatus.REVIEWING, TaskStatus.FAILED, TaskStatus.CANCELLED)),
        Map.entry(TaskStatus.REVIEWING, Set.of(TaskStatus.HUMAN_REVIEW, TaskStatus.DEPLOYING, TaskStatus.FAILED, TaskStatus.CANCELLED)),
        Map.entry(TaskStatus.HUMAN_REVIEW, Set.of(TaskStatus.APPROVED, TaskStatus.REJECTED)),
        Map.entry(TaskStatus.APPROVED, Set.of(TaskStatus.DEPLOYING, TaskStatus.FAILED)),
        Map.entry(TaskStatus.REJECTED, Set.of()),
        Map.entry(TaskStatus.DEPLOYING, Set.of(TaskStatus.DONE, TaskStatus.FAILED)),
        Map.entry(TaskStatus.DONE, Set.of()),
        Map.entry(TaskStatus.FAILED, Set.of()),
        Map.entry(TaskStatus.CANCELLED, Set.of())
    );

    public static boolean transition(TaskStatus from, TaskStatus to) {
        Set<TaskStatus> allowed = TRANSITIONS.get(from);
        return allowed != null && allowed.contains(to);
    }

    private TaskStateMachine() {}
}
```

- [ ] **Step 5: 运行测试确认通过**

Run: `cd forge-engine && mvn test -Dtest=TaskStateMachineTest -pl . 2>&1 | tail -15`
Expected: 全部 PASS

- [ ] **Step 6: Commit**

```bash
git add forge-engine/src/
git commit -m "feat(m4): add task state machine, enums, and state transition tests"
```

---

### Task 4: 编排层 — TaskService + KillSwitchService + RiskAssessor + 测试

**Files:**
- Create: `forge-engine/src/main/java/com/shulex/forge/engine/orchestration/service/TaskService.java`
- Create: `forge-engine/src/main/java/com/shulex/forge/engine/orchestration/service/KillSwitchService.java`
- Create: `forge-engine/src/main/java/com/shulex/forge/engine/orchestration/service/RiskAssessor.java`
- Create: `forge-engine/src/main/java/com/shulex/forge/engine/orchestration/service/TokenUsageService.java`
- Create: `forge-engine/src/test/java/com/shulex/forge/engine/orchestration/service/TaskServiceTest.java`
- Create: `forge-engine/src/test/java/com/shulex/forge/engine/orchestration/service/RiskAssessorTest.java`
- Create: `forge-engine/src/test/java/com/shulex/forge/engine/orchestration/service/KillSwitchServiceTest.java`

- [ ] **Step 1: 写 KillSwitchServiceTest**

```java
package com.shulex.forge.engine.orchestration.service;

import com.shulex.forge.engine.orchestration.model.KillSwitchLevel;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;
import org.springframework.data.redis.core.StringRedisTemplate;
import org.springframework.data.redis.core.ValueOperations;

import static org.assertj.core.api.Assertions.assertThat;
import static org.mockito.Mockito.*;

class KillSwitchServiceTest {

    private KillSwitchService killSwitchService;
    private StringRedisTemplate redisTemplate;
    private ValueOperations<String, String> valueOps;

    @BeforeEach
    void setUp() {
        redisTemplate = mock(StringRedisTemplate.class);
        valueOps = mock(ValueOperations.class);
        when(redisTemplate.opsForValue()).thenReturn(valueOps);
        killSwitchService = new KillSwitchService(redisTemplate);
    }

    @Test
    void getLevel_returnsNoneByDefault() {
        when(valueOps.get("forge:killswitch:level")).thenReturn(null);
        assertThat(killSwitchService.getLevel()).isEqualTo(KillSwitchLevel.NONE);
    }

    @Test
    void activate_setsLevel() {
        killSwitchService.activate(KillSwitchLevel.L1);
        verify(valueOps).set("forge:killswitch:level", "L1");
    }

    @Test
    void deactivate_setsNone() {
        killSwitchService.deactivate();
        verify(valueOps).set("forge:killswitch:level", "NONE");
    }

    @Test
    void isNewTaskAllowed_falseWhenL1() {
        when(valueOps.get("forge:killswitch:level")).thenReturn("L1");
        assertThat(killSwitchService.isNewTaskAllowed()).isFalse();
    }

    @Test
    void isExecutionAllowed_falseWhenL2() {
        when(valueOps.get("forge:killswitch:level")).thenReturn("L2");
        assertThat(killSwitchService.isExecutionAllowed()).isFalse();
    }
}
```

- [ ] **Step 2: 实现 KillSwitchService**

```java
package com.shulex.forge.engine.orchestration.service;

import com.shulex.forge.engine.orchestration.model.KillSwitchLevel;
import lombok.extern.slf4j.Slf4j;
import org.springframework.data.redis.core.StringRedisTemplate;
import org.springframework.stereotype.Service;

@Slf4j
@Service
public class KillSwitchService {

    private static final String KEY = "forge:killswitch:level";
    private final StringRedisTemplate redisTemplate;

    public KillSwitchService(StringRedisTemplate redisTemplate) {
        this.redisTemplate = redisTemplate;
    }

    public KillSwitchLevel getLevel() {
        String val = redisTemplate.opsForValue().get(KEY);
        if (val == null) return KillSwitchLevel.NONE;
        try {
            return KillSwitchLevel.valueOf(val);
        } catch (IllegalArgumentException e) {
            return KillSwitchLevel.NONE;
        }
    }

    public void activate(KillSwitchLevel level) {
        redisTemplate.opsForValue().set(KEY, level.name());
        log.warn("紧急停止已激活: level={}", level);
    }

    public void deactivate() {
        redisTemplate.opsForValue().set(KEY, KillSwitchLevel.NONE.name());
        log.info("紧急停止已解除");
    }

    public boolean isNewTaskAllowed() {
        return getLevel() == KillSwitchLevel.NONE;
    }

    public boolean isExecutionAllowed() {
        KillSwitchLevel level = getLevel();
        return level == KillSwitchLevel.NONE;
    }
}
```

- [ ] **Step 3: 写 RiskAssessorTest**

```java
package com.shulex.forge.engine.orchestration.service;

import com.shulex.forge.engine.orchestration.model.RiskLevel;
import org.junit.jupiter.api.Test;

import static org.assertj.core.api.Assertions.assertThat;

class RiskAssessorTest {

    private final RiskAssessor riskAssessor = new RiskAssessor();

    @Test
    void initialAssess_lowRiskForSimpleGeneration() {
        assertThat(riskAssessor.initialAssess("创建一个简单的 CRUD 接口", "GENERATE"))
                .isEqualTo(RiskLevel.LOW);
    }

    @Test
    void initialAssess_highRiskForSecurityRelated() {
        assertThat(riskAssessor.initialAssess("修改支付模块的加密逻辑", "ITERATE"))
                .isEqualTo(RiskLevel.HIGH);
    }

    @Test
    void finalAssess_upgradesWhenReviewScoreLow() {
        assertThat(riskAssessor.finalAssess(RiskLevel.LOW, 85, 12))
                .isEqualTo(RiskLevel.HIGH);
    }

    @Test
    void finalAssess_keepsLowWhenAllGood() {
        assertThat(riskAssessor.finalAssess(RiskLevel.LOW, 95, 3))
                .isEqualTo(RiskLevel.LOW);
    }
}
```

- [ ] **Step 4: 实现 RiskAssessor**

```java
package com.shulex.forge.engine.orchestration.service;

import com.shulex.forge.engine.orchestration.model.RiskLevel;
import org.springframework.stereotype.Service;

import java.util.Set;

@Service
public class RiskAssessor {

    private static final Set<String> HIGH_RISK_KEYWORDS = Set.of(
            "支付", "加密", "权限", "安全", "密码", "token", "secret",
            "payment", "security", "auth", "credential", "DROP", "DELETE"
    );

    public RiskLevel initialAssess(String requirement, String taskType) {
        String lower = requirement.toLowerCase();
        for (String keyword : HIGH_RISK_KEYWORDS) {
            if (lower.contains(keyword.toLowerCase())) {
                return RiskLevel.HIGH;
            }
        }
        return "ITERATE".equals(taskType) ? RiskLevel.MEDIUM : RiskLevel.LOW;
    }

    public RiskLevel finalAssess(RiskLevel initialRisk, int reviewScore, int fileCount) {
        if (reviewScore < 90 || fileCount > 10) {
            return RiskLevel.HIGH;
        }
        if (initialRisk == RiskLevel.HIGH) {
            return RiskLevel.HIGH;
        }
        if (fileCount > 5 || reviewScore < 95) {
            return RiskLevel.MEDIUM;
        }
        return initialRisk;
    }
}
```

- [ ] **Step 5: 写 TaskServiceTest**

```java
package com.shulex.forge.engine.orchestration.service;

import com.shulex.forge.engine.common.BizException;
import com.shulex.forge.engine.infrastructure.entity.TaskDO;
import com.shulex.forge.engine.infrastructure.entity.TaskStepDO;
import com.shulex.forge.engine.infrastructure.mapper.TaskMapper;
import com.shulex.forge.engine.infrastructure.mapper.TaskStepMapper;
import com.shulex.forge.engine.orchestration.model.TaskStatus;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;

import static org.assertj.core.api.Assertions.*;
import static org.mockito.ArgumentMatchers.any;
import static org.mockito.Mockito.*;

class TaskServiceTest {

    private TaskService taskService;
    private TaskMapper taskMapper;
    private TaskStepMapper taskStepMapper;
    private KillSwitchService killSwitchService;
    private RiskAssessor riskAssessor;

    @BeforeEach
    void setUp() {
        taskMapper = mock(TaskMapper.class);
        taskStepMapper = mock(TaskStepMapper.class);
        killSwitchService = mock(KillSwitchService.class);
        riskAssessor = mock(RiskAssessor.class);
        taskService = new TaskService(taskMapper, taskStepMapper, killSwitchService, riskAssessor);
    }

    @Test
    void createTask_insertsAndReturns() {
        when(killSwitchService.isNewTaskAllowed()).thenReturn(true);
        when(taskMapper.insert(any())).thenReturn(1);

        TaskDO task = taskService.createTask(1L, 1L, "创建用户服务", "GENERATE", "repo-123");
        assertThat(task.getStatus()).isEqualTo(TaskStatus.SUBMITTED.name());
        verify(taskMapper).insert(any());
    }

    @Test
    void createTask_throwsWhenKillSwitchActive() {
        when(killSwitchService.isNewTaskAllowed()).thenReturn(false);

        assertThatThrownBy(() -> taskService.createTask(1L, 1L, "test", "GENERATE", "repo"))
                .isInstanceOf(BizException.class);
    }

    @Test
    void transitionStatus_updatesOnValid() {
        TaskDO task = new TaskDO();
        task.setId(1L);
        task.setStatus(TaskStatus.SUBMITTED.name());
        when(taskMapper.selectById(1L)).thenReturn(task);
        when(taskMapper.updateById(any())).thenReturn(1);

        taskService.transitionStatus(1L, TaskStatus.ANALYZING);
        assertThat(task.getStatus()).isEqualTo(TaskStatus.ANALYZING.name());
    }

    @Test
    void transitionStatus_throwsOnInvalid() {
        TaskDO task = new TaskDO();
        task.setId(1L);
        task.setStatus(TaskStatus.DONE.name());
        when(taskMapper.selectById(1L)).thenReturn(task);

        assertThatThrownBy(() -> taskService.transitionStatus(1L, TaskStatus.SUBMITTED))
                .isInstanceOf(BizException.class);
    }
}
```

- [ ] **Step 6: 实现 TaskService**

```java
package com.shulex.forge.engine.orchestration.service;

import com.baomidou.mybatisplus.core.conditions.query.LambdaQueryWrapper;
import com.shulex.forge.engine.common.BizException;
import com.shulex.forge.engine.common.ErrorCode;
import com.shulex.forge.engine.infrastructure.entity.TaskDO;
import com.shulex.forge.engine.infrastructure.entity.TaskStepDO;
import com.shulex.forge.engine.infrastructure.mapper.TaskMapper;
import com.shulex.forge.engine.infrastructure.mapper.TaskStepMapper;
import com.shulex.forge.engine.orchestration.model.TaskStatus;
import com.shulex.forge.engine.orchestration.statemachine.TaskStateMachine;
import lombok.extern.slf4j.Slf4j;
import org.springframework.stereotype.Service;

import java.util.List;

@Slf4j
@Service
public class TaskService {

    private final TaskMapper taskMapper;
    private final TaskStepMapper taskStepMapper;
    private final KillSwitchService killSwitchService;
    private final RiskAssessor riskAssessor;

    public TaskService(TaskMapper taskMapper, TaskStepMapper taskStepMapper,
                       KillSwitchService killSwitchService, RiskAssessor riskAssessor) {
        this.taskMapper = taskMapper;
        this.taskStepMapper = taskStepMapper;
        this.killSwitchService = killSwitchService;
        this.riskAssessor = riskAssessor;
    }

    public TaskDO createTask(Long tenantId, Long userId, String requirement, String taskType, String repoId) {
        if (!killSwitchService.isNewTaskAllowed()) {
            throw new BizException(ErrorCode.KILL_SWITCH_ACTIVE);
        }
        TaskDO task = new TaskDO();
        task.setTenantId(tenantId);
        task.setUserId(userId);
        task.setRequirement(requirement);
        task.setTaskType(taskType);
        task.setStatus(TaskStatus.SUBMITTED.name());
        task.setRepoId(repoId);
        task.setTotalInputTokens(0L);
        task.setTotalOutputTokens(0L);
        taskMapper.insert(task);
        log.info("创建任务: id={}, tenant={}, user={}", task.getId(), tenantId, userId);
        return task;
    }

    public TaskDO getTask(Long taskId) {
        TaskDO task = taskMapper.selectById(taskId);
        if (task == null) {
            throw new BizException(ErrorCode.TASK_NOT_FOUND);
        }
        return task;
    }

    public List<TaskDO> listTasks(Long tenantId, Long userId) {
        return taskMapper.selectList(new LambdaQueryWrapper<TaskDO>()
                .eq(TaskDO::getTenantId, tenantId)
                .eq(TaskDO::getUserId, userId)
                .orderByDesc(TaskDO::getGmtCreate));
    }

    public void transitionStatus(Long taskId, TaskStatus newStatus) {
        TaskDO task = getTask(taskId);
        TaskStatus currentStatus = TaskStatus.valueOf(task.getStatus());
        if (!TaskStateMachine.transition(currentStatus, newStatus)) {
            throw new BizException(ErrorCode.TASK_INVALID_STATUS,
                    "不允许从 " + currentStatus + " 转换到 " + newStatus);
        }
        task.setStatus(newStatus.name());
        taskMapper.updateById(task);
        log.info("任务状态变更: id={}, {} -> {}", taskId, currentStatus, newStatus);
    }

    public void updateTokenUsage(Long taskId, long inputTokens, long outputTokens) {
        TaskDO task = getTask(taskId);
        task.setTotalInputTokens(task.getTotalInputTokens() + inputTokens);
        task.setTotalOutputTokens(task.getTotalOutputTokens() + outputTokens);
        taskMapper.updateById(task);
    }

    public List<TaskStepDO> getSteps(Long taskId) {
        return taskStepMapper.selectList(new LambdaQueryWrapper<TaskStepDO>()
                .eq(TaskStepDO::getTaskId, taskId)
                .orderByAsc(TaskStepDO::getStepOrder));
    }
}
```

- [ ] **Step 7: 实现 TokenUsageService**

```java
package com.shulex.forge.engine.orchestration.service;

import com.shulex.forge.engine.infrastructure.entity.ModelCallLogDO;
import com.shulex.forge.engine.infrastructure.mapper.ModelCallLogMapper;
import lombok.extern.slf4j.Slf4j;
import org.springframework.stereotype.Service;

@Slf4j
@Service
public class TokenUsageService {

    private final ModelCallLogMapper modelCallLogMapper;

    public TokenUsageService(ModelCallLogMapper modelCallLogMapper) {
        this.modelCallLogMapper = modelCallLogMapper;
    }

    public void recordCall(Long taskId, Long stepId, String modelId, String purpose,
                           long inputTokens, long outputTokens, long latencyMs) {
        ModelCallLogDO logDO = new ModelCallLogDO();
        logDO.setTaskId(taskId);
        logDO.setStepId(stepId);
        logDO.setModelId(modelId);
        logDO.setPurpose(purpose);
        logDO.setInputTokens(inputTokens);
        logDO.setOutputTokens(outputTokens);
        logDO.setLatencyMs(latencyMs);
        logDO.setIsFallback(0);
        modelCallLogMapper.insert(logDO);
        log.debug("记录模型调用: task={}, model={}, tokens={}+{}", taskId, modelId, inputTokens, outputTokens);
    }
}
```

- [ ] **Step 8: 运行全部编排层测试**

Run: `cd forge-engine && mvn test -Dtest=TaskStateMachineTest,TaskServiceTest,RiskAssessorTest,KillSwitchServiceTest -pl . 2>&1 | tail -15`
Expected: 全部 PASS

- [ ] **Step 9: Commit**

```bash
git add forge-engine/src/
git commit -m "feat(m4): add task service, kill switch, risk assessor, and token usage tracking"
```

---

### Task 5: 执行层 — Claude 客户端 + 上下文构建 + 测试

**Files:**
- Create: `forge-engine/src/main/java/com/shulex/forge/engine/execution/ai/ClaudeConfig.java`
- Create: `forge-engine/src/main/java/com/shulex/forge/engine/execution/ai/AiResponse.java`
- Create: `forge-engine/src/main/java/com/shulex/forge/engine/execution/ai/ClaudeClient.java`
- Create: `forge-engine/src/main/java/com/shulex/forge/engine/execution/model/GeneratedCode.java`
- Create: `forge-engine/src/main/java/com/shulex/forge/engine/infrastructure/http/SpecsClient.java`
- Create: `forge-engine/src/main/java/com/shulex/forge/engine/infrastructure/http/PipelineClient.java`
- Create: `forge-engine/src/main/java/com/shulex/forge/engine/execution/service/ContextBuilder.java`
- Create: `forge-engine/src/test/java/com/shulex/forge/engine/execution/ai/ClaudeClientTest.java`
- Create: `forge-engine/src/test/java/com/shulex/forge/engine/execution/service/ContextBuilderTest.java`

- [ ] **Step 1: 创建 ClaudeConfig**

```java
package com.shulex.forge.engine.execution.ai;

import lombok.Data;
import org.springframework.boot.context.properties.ConfigurationProperties;
import org.springframework.stereotype.Component;

@Data
@Component
@ConfigurationProperties(prefix = "forge.claude")
public class ClaudeConfig {
    private String apiKey;
    private String model = "claude-sonnet-4-20250514";
    private String baseUrl = "https://api.anthropic.com";
    private int maxTokens = 4096;
}
```

- [ ] **Step 2: 创建 AiResponse**

```java
package com.shulex.forge.engine.execution.ai;

import lombok.AllArgsConstructor;
import lombok.Data;
import lombok.NoArgsConstructor;

@Data
@NoArgsConstructor
@AllArgsConstructor
public class AiResponse {
    private String content;
    private long inputTokens;
    private long outputTokens;
    private String model;
    private String stopReason;
}
```

- [ ] **Step 3: 写 ClaudeClientTest**

```java
package com.shulex.forge.engine.execution.ai;

import okhttp3.mockwebserver.MockResponse;
import okhttp3.mockwebserver.MockWebServer;
import okhttp3.mockwebserver.RecordedRequest;
import org.junit.jupiter.api.AfterEach;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;

import static org.assertj.core.api.Assertions.assertThat;

class ClaudeClientTest {

    private MockWebServer server;
    private ClaudeClient claudeClient;

    @BeforeEach
    void setUp() throws Exception {
        server = new MockWebServer();
        server.start();
        ClaudeConfig config = new ClaudeConfig();
        config.setApiKey("test-key");
        config.setModel("claude-sonnet-4-20250514");
        config.setBaseUrl(server.url("/").toString());
        config.setMaxTokens(4096);
        claudeClient = new ClaudeClient(config);
    }

    @AfterEach
    void tearDown() throws Exception {
        server.shutdown();
    }

    @Test
    void chat_sendsCorrectRequest() throws Exception {
        server.enqueue(new MockResponse()
                .setBody("{\"content\":[{\"type\":\"text\",\"text\":\"Hello\"}],\"model\":\"claude-sonnet-4-20250514\",\"stop_reason\":\"end_turn\",\"usage\":{\"input_tokens\":10,\"output_tokens\":5}}")
                .addHeader("Content-Type", "application/json"));

        AiResponse response = claudeClient.chat("system prompt", "user message");
        assertThat(response.getContent()).isEqualTo("Hello");
        assertThat(response.getInputTokens()).isEqualTo(10);
        assertThat(response.getOutputTokens()).isEqualTo(5);

        RecordedRequest request = server.takeRequest();
        assertThat(request.getHeader("x-api-key")).isEqualTo("test-key");
        assertThat(request.getHeader("anthropic-version")).isEqualTo("2023-06-01");
    }

    @Test
    void chat_handlesLargeResponse() throws Exception {
        String longText = "x".repeat(1000);
        server.enqueue(new MockResponse()
                .setBody("{\"content\":[{\"type\":\"text\",\"text\":\"" + longText + "\"}],\"model\":\"claude-sonnet-4-20250514\",\"stop_reason\":\"end_turn\",\"usage\":{\"input_tokens\":100,\"output_tokens\":200}}")
                .addHeader("Content-Type", "application/json"));

        AiResponse response = claudeClient.chat("sys", "msg");
        assertThat(response.getContent()).hasSize(1000);
    }
}
```

- [ ] **Step 4: 实现 ClaudeClient**

```java
package com.shulex.forge.engine.execution.ai;

import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.ObjectMapper;
import com.fasterxml.jackson.databind.node.ArrayNode;
import com.fasterxml.jackson.databind.node.ObjectNode;
import com.shulex.forge.engine.common.SysException;
import com.shulex.forge.engine.common.ErrorCode;
import lombok.extern.slf4j.Slf4j;
import okhttp3.*;
import org.springframework.stereotype.Component;

import java.io.IOException;
import java.util.concurrent.TimeUnit;

@Slf4j
@Component
public class ClaudeClient {

    private final ClaudeConfig config;
    private final OkHttpClient httpClient;
    private final ObjectMapper objectMapper;

    public ClaudeClient(ClaudeConfig config) {
        this.config = config;
        this.httpClient = new OkHttpClient.Builder()
                .connectTimeout(30, TimeUnit.SECONDS)
                .readTimeout(120, TimeUnit.SECONDS)
                .writeTimeout(30, TimeUnit.SECONDS)
                .build();
        this.objectMapper = new ObjectMapper();
    }

    public AiResponse chat(String systemPrompt, String userMessage) {
        try {
            ObjectNode body = objectMapper.createObjectNode();
            body.put("model", config.getModel());
            body.put("max_tokens", config.getMaxTokens());
            body.put("system", systemPrompt);

            ArrayNode messages = body.putArray("messages");
            ObjectNode userMsg = messages.addObject();
            userMsg.put("role", "user");
            userMsg.put("content", userMessage);

            String url = config.getBaseUrl().endsWith("/")
                    ? config.getBaseUrl() + "v1/messages"
                    : config.getBaseUrl() + "/v1/messages";

            Request request = new Request.Builder()
                    .url(url)
                    .addHeader("x-api-key", config.getApiKey())
                    .addHeader("anthropic-version", "2023-06-01")
                    .addHeader("Content-Type", "application/json")
                    .post(RequestBody.create(objectMapper.writeValueAsString(body),
                            MediaType.parse("application/json")))
                    .build();

            long start = System.currentTimeMillis();
            try (Response response = httpClient.newCall(request).execute()) {
                long latency = System.currentTimeMillis() - start;
                if (!response.isSuccessful()) {
                    String errorBody = response.body() != null ? response.body().string() : "no body";
                    log.error("Claude API 失败: status={}, body={}", response.code(), errorBody);
                    throw new SysException(ErrorCode.AI_CALL_FAILED,
                            new RuntimeException("Claude API 返回 " + response.code()));
                }

                JsonNode root = objectMapper.readTree(response.body().string());
                String content = root.path("content").get(0).path("text").asText();
                long inputTokens = root.path("usage").path("input_tokens").asLong();
                long outputTokens = root.path("usage").path("output_tokens").asLong();
                String model = root.path("model").asText();
                String stopReason = root.path("stop_reason").asText();

                log.debug("Claude 调用完成: model={}, tokens={}+{}, latency={}ms",
                        model, inputTokens, outputTokens, latency);

                return new AiResponse(content, inputTokens, outputTokens, model, stopReason);
            }
        } catch (SysException e) {
            throw e;
        } catch (IOException e) {
            throw new SysException(ErrorCode.AI_CALL_FAILED, e);
        }
    }
}
```

- [ ] **Step 5: 创建 GeneratedCode**

```java
package com.shulex.forge.engine.execution.model;

import lombok.AllArgsConstructor;
import lombok.Data;
import lombok.NoArgsConstructor;

@Data
@NoArgsConstructor
@AllArgsConstructor
public class GeneratedCode {
    private String filePath;
    private String content;
    private String action; // CREATE, MODIFY, DELETE
}
```

- [ ] **Step 6: 创建 SpecsClient + PipelineClient**

```java
// SpecsClient.java
package com.shulex.forge.engine.infrastructure.http;

import com.fasterxml.jackson.core.type.TypeReference;
import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.ObjectMapper;
import lombok.extern.slf4j.Slf4j;
import okhttp3.*;
import org.springframework.beans.factory.annotation.Value;
import org.springframework.stereotype.Component;

import java.io.IOException;
import java.util.List;
import java.util.Map;

@Slf4j
@Component
public class SpecsClient {

    private final OkHttpClient httpClient;
    private final ObjectMapper objectMapper;
    private final String baseUrl;

    public SpecsClient(@Value("${forge.specs.base-url}") String baseUrl) {
        this.baseUrl = baseUrl;
        this.httpClient = new OkHttpClient();
        this.objectMapper = new ObjectMapper();
    }

    public String getPromptTemplate(String templateKey) {
        try {
            Request request = new Request.Builder()
                    .url(baseUrl + "/api/prompts/" + templateKey)
                    .build();
            try (Response response = httpClient.newCall(request).execute()) {
                if (!response.isSuccessful()) return null;
                JsonNode root = objectMapper.readTree(response.body().string());
                return root.path("data").path("systemPrompt").asText(null);
            }
        } catch (IOException e) {
            log.warn("获取 Prompt 模板失败: key={}", templateKey, e);
            return null;
        }
    }

    public String getStandards(String category) {
        try {
            Request request = new Request.Builder()
                    .url(baseUrl + "/api/standards?category=" + category)
                    .build();
            try (Response response = httpClient.newCall(request).execute()) {
                if (!response.isSuccessful()) return "";
                JsonNode root = objectMapper.readTree(response.body().string());
                StringBuilder sb = new StringBuilder();
                for (JsonNode item : root.path("data")) {
                    sb.append("## ").append(item.path("title").asText()).append("\n");
                    sb.append(item.path("content").asText()).append("\n\n");
                }
                return sb.toString();
            }
        } catch (IOException e) {
            log.warn("获取编码规范失败: category={}", category, e);
            return "";
        }
    }

    public String getReviewRules() {
        try {
            Request request = new Request.Builder()
                    .url(baseUrl + "/api/review-rules")
                    .build();
            try (Response response = httpClient.newCall(request).execute()) {
                if (!response.isSuccessful()) return "";
                JsonNode root = objectMapper.readTree(response.body().string());
                StringBuilder sb = new StringBuilder();
                for (JsonNode item : root.path("data")) {
                    sb.append("- [").append(item.path("severity").asText()).append("] ");
                    sb.append(item.path("name").asText()).append(": ");
                    sb.append(item.path("description").asText()).append("\n");
                }
                return sb.toString();
            }
        } catch (IOException e) {
            log.warn("获取 Review 规则失败", e);
            return "";
        }
    }
}
```

```java
// PipelineClient.java
package com.shulex.forge.engine.infrastructure.http;

import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.ObjectMapper;
import com.fasterxml.jackson.databind.node.ArrayNode;
import com.fasterxml.jackson.databind.node.ObjectNode;
import com.shulex.forge.engine.common.ErrorCode;
import com.shulex.forge.engine.common.SysException;
import com.shulex.forge.engine.execution.model.GeneratedCode;
import lombok.extern.slf4j.Slf4j;
import okhttp3.*;
import org.springframework.beans.factory.annotation.Value;
import org.springframework.stereotype.Component;

import java.io.IOException;
import java.util.ArrayList;
import java.util.List;

@Slf4j
@Component
public class PipelineClient {

    private final OkHttpClient httpClient;
    private final ObjectMapper objectMapper;
    private final String baseUrl;

    public PipelineClient(@Value("${forge.pipeline.base-url}") String baseUrl) {
        this.baseUrl = baseUrl;
        this.httpClient = new OkHttpClient();
        this.objectMapper = new ObjectMapper();
    }

    public String getFileContent(String adapterType, String repoId, String filePath, String ref) {
        try {
            String url = baseUrl + "/api/adapters/" + adapterType + "/repos/" + repoId
                    + "/files?path=" + filePath + "&ref=" + ref;
            Request request = new Request.Builder().url(url).build();
            try (Response response = httpClient.newCall(request).execute()) {
                if (!response.isSuccessful()) return null;
                JsonNode root = objectMapper.readTree(response.body().string());
                return root.path("data").path("content").asText(null);
            }
        } catch (IOException e) {
            log.warn("获取文件内容失败: repo={}, path={}", repoId, filePath, e);
            return null;
        }
    }

    public List<String> listRepositoryTree(String adapterType, String repoId, String path, String ref) {
        try {
            String url = baseUrl + "/api/adapters/" + adapterType + "/repos/" + repoId
                    + "/tree?path=" + path + "&ref=" + ref;
            Request request = new Request.Builder().url(url).build();
            try (Response response = httpClient.newCall(request).execute()) {
                if (!response.isSuccessful()) return List.of();
                JsonNode root = objectMapper.readTree(response.body().string());
                List<String> paths = new ArrayList<>();
                for (JsonNode item : root.path("data")) {
                    paths.add(item.path("path").asText());
                }
                return paths;
            }
        } catch (IOException e) {
            log.warn("获取文件树失败: repo={}", repoId, e);
            return List.of();
        }
    }

    public String createBranch(String adapterType, String repoId, String branchName, String ref) {
        try {
            ObjectNode body = objectMapper.createObjectNode();
            body.put("branchName", branchName);
            body.put("ref", ref);
            String url = baseUrl + "/api/adapters/" + adapterType + "/repos/" + repoId + "/branches";
            Request request = new Request.Builder()
                    .url(url)
                    .post(RequestBody.create(objectMapper.writeValueAsString(body),
                            MediaType.parse("application/json")))
                    .build();
            try (Response response = httpClient.newCall(request).execute()) {
                if (!response.isSuccessful()) return null;
                JsonNode root = objectMapper.readTree(response.body().string());
                return root.path("data").path("name").asText(null);
            }
        } catch (IOException e) {
            log.warn("创建分支失败: repo={}, branch={}", repoId, branchName, e);
            return null;
        }
    }

    public String commitFiles(String adapterType, String repoId, String branch,
                              String commitMessage, List<GeneratedCode> files) {
        try {
            ObjectNode body = objectMapper.createObjectNode();
            body.put("branch", branch);
            body.put("commitMessage", commitMessage);
            ArrayNode filesArray = body.putArray("files");
            for (GeneratedCode file : files) {
                ObjectNode f = filesArray.addObject();
                f.put("filePath", file.getFilePath());
                f.put("content", file.getContent());
                f.put("action", file.getAction());
            }
            String url = baseUrl + "/api/adapters/" + adapterType + "/repos/" + repoId + "/commits";
            Request request = new Request.Builder()
                    .url(url)
                    .post(RequestBody.create(objectMapper.writeValueAsString(body),
                            MediaType.parse("application/json")))
                    .build();
            try (Response response = httpClient.newCall(request).execute()) {
                if (!response.isSuccessful()) {
                    throw new SysException(ErrorCode.CODE_COMMIT_FAILED,
                            new RuntimeException("提交失败: " + response.code()));
                }
                JsonNode root = objectMapper.readTree(response.body().string());
                return root.path("data").asText(null);
            }
        } catch (SysException e) {
            throw e;
        } catch (IOException e) {
            throw new SysException(ErrorCode.CODE_COMMIT_FAILED, e);
        }
    }

    public Long createMergeRequest(String adapterType, String repoId,
                                    String sourceBranch, String targetBranch,
                                    String title, String description) {
        try {
            ObjectNode body = objectMapper.createObjectNode();
            body.put("sourceBranch", sourceBranch);
            body.put("targetBranch", targetBranch);
            body.put("title", title);
            body.put("description", description);
            String url = baseUrl + "/api/adapters/" + adapterType + "/repos/" + repoId + "/merge-requests";
            Request request = new Request.Builder()
                    .url(url)
                    .post(RequestBody.create(objectMapper.writeValueAsString(body),
                            MediaType.parse("application/json")))
                    .build();
            try (Response response = httpClient.newCall(request).execute()) {
                if (!response.isSuccessful()) return null;
                JsonNode root = objectMapper.readTree(response.body().string());
                return root.path("data").path("id").asLong();
            }
        } catch (IOException e) {
            log.warn("创建 MR 失败", e);
            return null;
        }
    }
}
```

- [ ] **Step 7: 写 ContextBuilderTest**

```java
package com.shulex.forge.engine.execution.service;

import com.shulex.forge.engine.infrastructure.http.PipelineClient;
import com.shulex.forge.engine.infrastructure.http.SpecsClient;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;

import java.util.List;

import static org.assertj.core.api.Assertions.assertThat;
import static org.mockito.Mockito.*;

class ContextBuilderTest {

    private ContextBuilder contextBuilder;
    private SpecsClient specsClient;
    private PipelineClient pipelineClient;

    @BeforeEach
    void setUp() {
        specsClient = mock(SpecsClient.class);
        pipelineClient = mock(PipelineClient.class);
        contextBuilder = new ContextBuilder(specsClient, pipelineClient);
    }

    @Test
    void buildContext_includesStandards() {
        when(specsClient.getStandards("java")).thenReturn("## Java 规范\n内容");
        when(specsClient.getReviewRules()).thenReturn("- 规则1");
        when(pipelineClient.listRepositoryTree(any(), any(), any(), any())).thenReturn(List.of());

        String context = contextBuilder.buildContext("codeup", "repo-123", "main", "创建用户服务");
        assertThat(context).contains("Java 规范");
    }

    @Test
    void buildContext_includesFileTree() {
        when(specsClient.getStandards("java")).thenReturn("");
        when(specsClient.getReviewRules()).thenReturn("");
        when(pipelineClient.listRepositoryTree("codeup", "repo-123", "/", "main"))
                .thenReturn(List.of("src/main/java/App.java", "pom.xml"));

        String context = contextBuilder.buildContext("codeup", "repo-123", "main", "test");
        assertThat(context).contains("src/main/java/App.java");
    }
}
```

- [ ] **Step 8: 实现 ContextBuilder**

```java
package com.shulex.forge.engine.execution.service;

import com.shulex.forge.engine.infrastructure.http.PipelineClient;
import com.shulex.forge.engine.infrastructure.http.SpecsClient;
import lombok.extern.slf4j.Slf4j;
import org.springframework.stereotype.Service;

import java.util.List;

@Slf4j
@Service
public class ContextBuilder {

    private final SpecsClient specsClient;
    private final PipelineClient pipelineClient;

    public ContextBuilder(SpecsClient specsClient, PipelineClient pipelineClient) {
        this.specsClient = specsClient;
        this.pipelineClient = pipelineClient;
    }

    public String buildContext(String adapterType, String repoId, String ref, String requirement) {
        StringBuilder context = new StringBuilder();

        // 1. 编码规范
        String standards = specsClient.getStandards("java");
        if (standards != null && !standards.isBlank()) {
            context.append("# 编码规范\n\n").append(standards).append("\n\n");
        }

        // 2. Review 规则
        String rules = specsClient.getReviewRules();
        if (rules != null && !rules.isBlank()) {
            context.append("# Review 规则\n\n").append(rules).append("\n\n");
        }

        // 3. 项目文件结构
        List<String> files = pipelineClient.listRepositoryTree(adapterType, repoId, "/", ref);
        if (!files.isEmpty()) {
            context.append("# 项目文件结构\n\n");
            for (String file : files) {
                context.append("- ").append(file).append("\n");
            }
            context.append("\n");
        }

        log.info("上下文构建完成: 长度={}", context.length());
        return context.toString();
    }

    public String buildSystemPrompt(String templateKey) {
        String template = specsClient.getPromptTemplate(templateKey);
        if (template != null) {
            return template;
        }
        return getDefaultSystemPrompt(templateKey);
    }

    private String getDefaultSystemPrompt(String templateKey) {
        return switch (templateKey) {
            case "code-generation" -> "你是一个资深 Java 开发工程师。根据需求和项目上下文生成高质量的生产级代码。"
                    + "输出格式：每个文件用 ```file:路径 ``` 包裹。";
            case "code-review" -> "你是一个代码审查专家。审查代码是否符合编码规范、安全性和最佳实践。"
                    + "输出格式：JSON {\"score\": 0-100, \"issues\": [{\"severity\": \"...\", \"description\": \"...\", \"suggestion\": \"...\"}]}";
            case "code-fix" -> "你是一个代码修复专家。根据 Review 反馈修复代码问题。"
                    + "输出格式：每个文件用 ```file:路径 ``` 包裹。";
            default -> "你是一个 AI 编程助手。";
        };
    }
}
```

- [ ] **Step 9: 运行测试**

Run: `cd forge-engine && mvn test -Dtest=ClaudeClientTest,ContextBuilderTest -pl . 2>&1 | tail -15`
Expected: 全部 PASS

- [ ] **Step 10: Commit**

```bash
git add forge-engine/src/
git commit -m "feat(m4): add Claude client, context builder, and cross-service HTTP clients"
```

---

### Task 6: 执行层 — 代码生成 + Review + 修复 + 提交 + 测试

**Files:**
- Create: `forge-engine/src/main/java/com/shulex/forge/engine/execution/service/CodeGenerator.java`
- Create: `forge-engine/src/main/java/com/shulex/forge/engine/execution/service/CodeReviewer.java`
- Create: `forge-engine/src/main/java/com/shulex/forge/engine/execution/service/CodeFixer.java`
- Create: `forge-engine/src/main/java/com/shulex/forge/engine/execution/service/CodeCommitter.java`
- Create: `forge-engine/src/test/java/com/shulex/forge/engine/execution/service/CodeGeneratorTest.java`
- Create: `forge-engine/src/test/java/com/shulex/forge/engine/execution/service/CodeReviewerTest.java`

- [ ] **Step 1: 写 CodeGeneratorTest**

```java
package com.shulex.forge.engine.execution.service;

import com.shulex.forge.engine.execution.ai.AiResponse;
import com.shulex.forge.engine.execution.ai.ClaudeClient;
import com.shulex.forge.engine.execution.model.GeneratedCode;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;

import java.util.List;

import static org.assertj.core.api.Assertions.assertThat;
import static org.mockito.ArgumentMatchers.any;
import static org.mockito.Mockito.*;

class CodeGeneratorTest {

    private CodeGenerator codeGenerator;
    private ClaudeClient claudeClient;
    private ContextBuilder contextBuilder;

    @BeforeEach
    void setUp() {
        claudeClient = mock(ClaudeClient.class);
        contextBuilder = mock(ContextBuilder.class);
        codeGenerator = new CodeGenerator(claudeClient, contextBuilder);
    }

    @Test
    void generate_parsesFileBlocks() {
        String aiOutput = "```file:src/main/java/User.java\npublic class User {}\n```\n"
                + "```file:src/main/java/UserService.java\npublic class UserService {}\n```";
        when(contextBuilder.buildSystemPrompt("code-generation")).thenReturn("sys prompt");
        when(contextBuilder.buildContext(any(), any(), any(), any())).thenReturn("context");
        when(claudeClient.chat(any(), any()))
                .thenReturn(new AiResponse(aiOutput, 100, 200, "claude", "end_turn"));

        var result = codeGenerator.generate("codeup", "repo", "main", "创建用户服务");
        assertThat(result.getFiles()).hasSize(2);
        assertThat(result.getFiles().get(0).getFilePath()).isEqualTo("src/main/java/User.java");
        assertThat(result.getInputTokens()).isEqualTo(100);
    }

    @Test
    void generate_handlesEmptyResponse() {
        when(contextBuilder.buildSystemPrompt(any())).thenReturn("sys");
        when(contextBuilder.buildContext(any(), any(), any(), any())).thenReturn("ctx");
        when(claudeClient.chat(any(), any()))
                .thenReturn(new AiResponse("No code needed.", 10, 5, "claude", "end_turn"));

        var result = codeGenerator.generate("codeup", "repo", "main", "test");
        assertThat(result.getFiles()).isEmpty();
    }
}
```

- [ ] **Step 2: 实现 CodeGenerator**

```java
package com.shulex.forge.engine.execution.service;

import com.shulex.forge.engine.execution.ai.AiResponse;
import com.shulex.forge.engine.execution.ai.ClaudeClient;
import com.shulex.forge.engine.execution.model.GeneratedCode;
import lombok.AllArgsConstructor;
import lombok.Data;
import lombok.extern.slf4j.Slf4j;
import org.springframework.stereotype.Service;

import java.util.ArrayList;
import java.util.List;
import java.util.regex.Matcher;
import java.util.regex.Pattern;

@Slf4j
@Service
public class CodeGenerator {

    private static final Pattern FILE_BLOCK = Pattern.compile(
            "```file:([^\\n]+)\\n(.*?)```", Pattern.DOTALL);

    private final ClaudeClient claudeClient;
    private final ContextBuilder contextBuilder;

    public CodeGenerator(ClaudeClient claudeClient, ContextBuilder contextBuilder) {
        this.claudeClient = claudeClient;
        this.contextBuilder = contextBuilder;
    }

    public GenerateResult generate(String adapterType, String repoId, String ref, String requirement) {
        String systemPrompt = contextBuilder.buildSystemPrompt("code-generation");
        String context = contextBuilder.buildContext(adapterType, repoId, ref, requirement);
        String userMessage = "# 需求\n\n" + requirement + "\n\n# 项目上下文\n\n" + context;

        AiResponse response = claudeClient.chat(systemPrompt, userMessage);
        List<GeneratedCode> files = parseFiles(response.getContent());

        log.info("代码生成完成: files={}, tokens={}+{}", files.size(),
                response.getInputTokens(), response.getOutputTokens());

        return new GenerateResult(files, response.getInputTokens(), response.getOutputTokens(), response.getContent());
    }

    private List<GeneratedCode> parseFiles(String content) {
        List<GeneratedCode> files = new ArrayList<>();
        Matcher matcher = FILE_BLOCK.matcher(content);
        while (matcher.find()) {
            String filePath = matcher.group(1).trim();
            String fileContent = matcher.group(2).trim();
            files.add(new GeneratedCode(filePath, fileContent, "CREATE"));
        }
        return files;
    }

    @Data
    @AllArgsConstructor
    public static class GenerateResult {
        private List<GeneratedCode> files;
        private long inputTokens;
        private long outputTokens;
        private String rawResponse;
    }
}
```

- [ ] **Step 3: 写 CodeReviewerTest**

```java
package com.shulex.forge.engine.execution.service;

import com.shulex.forge.engine.execution.ai.AiResponse;
import com.shulex.forge.engine.execution.ai.ClaudeClient;
import com.shulex.forge.engine.execution.model.GeneratedCode;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;

import java.util.List;

import static org.assertj.core.api.Assertions.assertThat;
import static org.mockito.ArgumentMatchers.any;
import static org.mockito.Mockito.*;

class CodeReviewerTest {

    private CodeReviewer codeReviewer;
    private ClaudeClient claudeClient;
    private ContextBuilder contextBuilder;

    @BeforeEach
    void setUp() {
        claudeClient = mock(ClaudeClient.class);
        contextBuilder = mock(ContextBuilder.class);
        codeReviewer = new CodeReviewer(claudeClient, contextBuilder);
    }

    @Test
    void review_parsesScoreAndIssues() {
        String aiOutput = "{\"score\": 92, \"issues\": [{\"severity\": \"minor\", \"description\": \"缺少注释\", \"suggestion\": \"添加 Javadoc\"}]}";
        when(contextBuilder.buildSystemPrompt("code-review")).thenReturn("review prompt");
        when(claudeClient.chat(any(), any()))
                .thenReturn(new AiResponse(aiOutput, 50, 30, "claude", "end_turn"));

        List<GeneratedCode> files = List.of(new GeneratedCode("User.java", "code", "CREATE"));
        var result = codeReviewer.review(files);
        assertThat(result.getScore()).isEqualTo(92);
        assertThat(result.getIssues()).hasSize(1);
    }

    @Test
    void review_handlesNonJsonGracefully() {
        when(contextBuilder.buildSystemPrompt("code-review")).thenReturn("prompt");
        when(claudeClient.chat(any(), any()))
                .thenReturn(new AiResponse("The code looks good overall.", 50, 30, "claude", "end_turn"));

        List<GeneratedCode> files = List.of(new GeneratedCode("Test.java", "code", "CREATE"));
        var result = codeReviewer.review(files);
        assertThat(result.getScore()).isEqualTo(80); // default score
    }
}
```

- [ ] **Step 4: 实现 CodeReviewer**

```java
package com.shulex.forge.engine.execution.service;

import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.ObjectMapper;
import com.shulex.forge.engine.execution.ai.AiResponse;
import com.shulex.forge.engine.execution.ai.ClaudeClient;
import com.shulex.forge.engine.execution.model.GeneratedCode;
import lombok.AllArgsConstructor;
import lombok.Data;
import lombok.NoArgsConstructor;
import lombok.extern.slf4j.Slf4j;
import org.springframework.stereotype.Service;

import java.util.ArrayList;
import java.util.List;

@Slf4j
@Service
public class CodeReviewer {

    private final ClaudeClient claudeClient;
    private final ContextBuilder contextBuilder;
    private final ObjectMapper objectMapper = new ObjectMapper();

    public CodeReviewer(ClaudeClient claudeClient, ContextBuilder contextBuilder) {
        this.claudeClient = claudeClient;
        this.contextBuilder = contextBuilder;
    }

    public ReviewResult review(List<GeneratedCode> files) {
        String systemPrompt = contextBuilder.buildSystemPrompt("code-review");
        StringBuilder userMessage = new StringBuilder("请审查以下代码：\n\n");
        for (GeneratedCode file : files) {
            userMessage.append("## ").append(file.getFilePath()).append("\n```\n")
                    .append(file.getContent()).append("\n```\n\n");
        }

        AiResponse response = claudeClient.chat(systemPrompt, userMessage.toString());
        return parseReviewResult(response);
    }

    private ReviewResult parseReviewResult(AiResponse response) {
        try {
            String content = response.getContent().trim();
            // 尝试提取 JSON（可能包裹在 markdown code block 中）
            if (content.contains("```json")) {
                content = content.substring(content.indexOf("```json") + 7);
                content = content.substring(0, content.indexOf("```"));
            } else if (content.contains("```")) {
                content = content.substring(content.indexOf("```") + 3);
                content = content.substring(0, content.indexOf("```"));
            }
            content = content.trim();

            JsonNode root = objectMapper.readTree(content);
            int score = root.path("score").asInt(80);
            List<ReviewIssue> issues = new ArrayList<>();
            for (JsonNode issue : root.path("issues")) {
                issues.add(new ReviewIssue(
                        issue.path("severity").asText("minor"),
                        issue.path("description").asText(),
                        issue.path("suggestion").asText("")
                ));
            }
            return new ReviewResult(score, issues, response.getInputTokens(), response.getOutputTokens());
        } catch (Exception e) {
            log.warn("解析 Review 结果失败，使用默认评分: {}", e.getMessage());
            return new ReviewResult(80, List.of(), response.getInputTokens(), response.getOutputTokens());
        }
    }

    @Data
    @AllArgsConstructor
    public static class ReviewResult {
        private int score;
        private List<ReviewIssue> issues;
        private long inputTokens;
        private long outputTokens;
    }

    @Data
    @NoArgsConstructor
    @AllArgsConstructor
    public static class ReviewIssue {
        private String severity;
        private String description;
        private String suggestion;
    }
}
```

- [ ] **Step 5: 实现 CodeFixer**

```java
package com.shulex.forge.engine.execution.service;

import com.shulex.forge.engine.execution.ai.AiResponse;
import com.shulex.forge.engine.execution.ai.ClaudeClient;
import com.shulex.forge.engine.execution.model.GeneratedCode;
import lombok.extern.slf4j.Slf4j;
import org.springframework.stereotype.Service;

import java.util.List;

@Slf4j
@Service
public class CodeFixer {

    private final ClaudeClient claudeClient;
    private final ContextBuilder contextBuilder;
    private final CodeGenerator codeGenerator;

    public CodeFixer(ClaudeClient claudeClient, ContextBuilder contextBuilder, CodeGenerator codeGenerator) {
        this.claudeClient = claudeClient;
        this.contextBuilder = contextBuilder;
        this.codeGenerator = codeGenerator;
    }

    public CodeGenerator.GenerateResult fix(List<GeneratedCode> originalFiles,
                                             List<CodeReviewer.ReviewIssue> issues) {
        String systemPrompt = contextBuilder.buildSystemPrompt("code-fix");
        StringBuilder userMessage = new StringBuilder("请修复以下代码中的问题：\n\n");

        userMessage.append("# 问题列表\n\n");
        for (CodeReviewer.ReviewIssue issue : issues) {
            userMessage.append("- [").append(issue.getSeverity()).append("] ")
                    .append(issue.getDescription());
            if (issue.getSuggestion() != null && !issue.getSuggestion().isBlank()) {
                userMessage.append(" → ").append(issue.getSuggestion());
            }
            userMessage.append("\n");
        }

        userMessage.append("\n# 原始代码\n\n");
        for (GeneratedCode file : originalFiles) {
            userMessage.append("## ").append(file.getFilePath()).append("\n```\n")
                    .append(file.getContent()).append("\n```\n\n");
        }

        AiResponse response = claudeClient.chat(systemPrompt, userMessage.toString());

        // 复用 CodeGenerator 的解析逻辑
        List<GeneratedCode> fixedFiles = parseFixedFiles(response.getContent(), originalFiles);
        log.info("代码修复完成: fixedFiles={}", fixedFiles.size());
        return new CodeGenerator.GenerateResult(fixedFiles, response.getInputTokens(),
                response.getOutputTokens(), response.getContent());
    }

    private List<GeneratedCode> parseFixedFiles(String content, List<GeneratedCode> originals) {
        // 使用 CodeGenerator 的正则解析
        java.util.regex.Matcher matcher = java.util.regex.Pattern.compile(
                "```file:([^\\n]+)\\n(.*?)```", java.util.regex.Pattern.DOTALL).matcher(content);
        List<GeneratedCode> files = new java.util.ArrayList<>();
        while (matcher.find()) {
            files.add(new GeneratedCode(matcher.group(1).trim(), matcher.group(2).trim(), "MODIFY"));
        }
        return files.isEmpty() ? originals : files;
    }
}
```

- [ ] **Step 6: 实现 CodeCommitter**

```java
package com.shulex.forge.engine.execution.service;

import com.shulex.forge.engine.execution.model.GeneratedCode;
import com.shulex.forge.engine.infrastructure.entity.CodeChangeDO;
import com.shulex.forge.engine.infrastructure.http.PipelineClient;
import com.shulex.forge.engine.infrastructure.mapper.CodeChangeMapper;
import lombok.extern.slf4j.Slf4j;
import org.springframework.stereotype.Service;

import java.util.List;

@Slf4j
@Service
public class CodeCommitter {

    private final PipelineClient pipelineClient;
    private final CodeChangeMapper codeChangeMapper;

    public CodeCommitter(PipelineClient pipelineClient, CodeChangeMapper codeChangeMapper) {
        this.pipelineClient = pipelineClient;
        this.codeChangeMapper = codeChangeMapper;
    }

    public String createBranch(String adapterType, String repoId, Long taskId) {
        String branchName = "ai/task-" + taskId;
        String result = pipelineClient.createBranch(adapterType, repoId, branchName, "main");
        log.info("创建分支: {}", branchName);
        return branchName;
    }

    public String commitCode(String adapterType, String repoId, String branch,
                              Long taskId, List<GeneratedCode> files) {
        String commitMessage = "[forge-ai] task-" + taskId + ": 自动生成代码";
        String commitHash = pipelineClient.commitFiles(adapterType, repoId, branch, commitMessage, files);

        CodeChangeDO change = new CodeChangeDO();
        change.setTaskId(taskId);
        change.setRepoId(repoId);
        change.setBranchName(branch);
        change.setCommitHash(commitHash);
        change.setFileCount(files.size());
        codeChangeMapper.insert(change);

        log.info("代码提交成功: task={}, commit={}, files={}", taskId, commitHash, files.size());
        return commitHash;
    }

    public Long createMergeRequest(String adapterType, String repoId, String branch,
                                    Long taskId, String requirement) {
        String title = "[forge-ai] task-" + taskId;
        String description = "## AI 生成代码\n\n**需求:** " + requirement;
        Long mrId = pipelineClient.createMergeRequest(adapterType, repoId, branch, "main", title, description);

        if (mrId != null) {
            CodeChangeDO change = new CodeChangeDO();
            change.setTaskId(taskId);
            change.setRepoId(repoId);
            change.setBranchName(branch);
            change.setMrId(mrId);
            change.setMrStatus("OPEN");
            change.setFileCount(0);
            codeChangeMapper.insert(change);
        }

        log.info("创建 MR: task={}, mrId={}", taskId, mrId);
        return mrId;
    }
}
```

- [ ] **Step 7: 运行测试**

Run: `cd forge-engine && mvn test -Dtest=CodeGeneratorTest,CodeReviewerTest -pl . 2>&1 | tail -15`
Expected: 全部 PASS

- [ ] **Step 8: Commit**

```bash
git add forge-engine/src/
git commit -m "feat(m4): add code generator, reviewer, fixer, and committer"
```

---

### Task 7: Kafka 任务通道 — 步骤分发 + 消费 + 编排闭环

**Files:**
- Create: `forge-engine/src/main/java/com/shulex/forge/engine/execution/model/StepRequest.java`
- Create: `forge-engine/src/main/java/com/shulex/forge/engine/execution/model/StepResult.java`
- Create: `forge-engine/src/main/java/com/shulex/forge/engine/orchestration/service/TaskDispatcher.java`
- Create: `forge-engine/src/main/java/com/shulex/forge/engine/execution/service/StepExecutor.java`
- Create: `forge-engine/src/main/java/com/shulex/forge/engine/execution/listener/StepRequestListener.java`
- Create: `forge-engine/src/main/java/com/shulex/forge/engine/orchestration/listener/StepResultListener.java`

- [ ] **Step 1: 创建 StepRequest**

```java
package com.shulex.forge.engine.execution.model;

import lombok.AllArgsConstructor;
import lombok.Data;
import lombok.NoArgsConstructor;

@Data
@NoArgsConstructor
@AllArgsConstructor
public class StepRequest {
    private Long taskId;
    private Long stepId;
    private String stepType;
    private String adapterType;
    private String repoId;
    private String branchName;
    private String requirement;
    private String inputData; // JSON extra data
}
```

- [ ] **Step 2: 创建 StepResult**

```java
package com.shulex.forge.engine.execution.model;

import lombok.AllArgsConstructor;
import lombok.Data;
import lombok.NoArgsConstructor;

@Data
@NoArgsConstructor
@AllArgsConstructor
public class StepResult {
    private Long taskId;
    private Long stepId;
    private String stepType;
    private boolean success;
    private String outputData; // JSON
    private long inputTokens;
    private long outputTokens;
    private String errorMessage;
}
```

- [ ] **Step 3: 实现 TaskDispatcher**

```java
package com.shulex.forge.engine.orchestration.service;

import com.fasterxml.jackson.databind.ObjectMapper;
import com.shulex.forge.engine.infrastructure.config.KafkaConfig;
import com.shulex.forge.engine.infrastructure.entity.TaskDO;
import com.shulex.forge.engine.infrastructure.entity.TaskStepDO;
import com.shulex.forge.engine.infrastructure.mapper.TaskStepMapper;
import com.shulex.forge.engine.execution.model.StepRequest;
import com.shulex.forge.engine.orchestration.model.StepStatus;
import com.shulex.forge.engine.orchestration.model.StepType;
import com.shulex.forge.engine.orchestration.model.TaskStatus;
import lombok.extern.slf4j.Slf4j;
import org.springframework.kafka.core.KafkaTemplate;
import org.springframework.stereotype.Service;

import java.util.List;

@Slf4j
@Service
public class TaskDispatcher {

    private final TaskService taskService;
    private final TaskStepMapper taskStepMapper;
    private final KafkaTemplate<String, String> kafkaTemplate;
    private final ObjectMapper objectMapper;

    public TaskDispatcher(TaskService taskService, TaskStepMapper taskStepMapper,
                          KafkaTemplate<String, String> kafkaTemplate) {
        this.taskService = taskService;
        this.taskStepMapper = taskStepMapper;
        this.kafkaTemplate = kafkaTemplate;
        this.objectMapper = new ObjectMapper();
    }

    public void startTask(Long taskId) {
        TaskDO task = taskService.getTask(taskId);
        createSteps(task);
        taskService.transitionStatus(taskId, TaskStatus.ANALYZING);
        dispatchNextStep(taskId);
    }

    public void dispatchNextStep(Long taskId) {
        List<TaskStepDO> steps = taskService.getSteps(taskId);
        TaskDO task = taskService.getTask(taskId);

        for (TaskStepDO step : steps) {
            if (StepStatus.PENDING.name().equals(step.getStatus())) {
                step.setStatus(StepStatus.RUNNING.name());
                taskStepMapper.updateById(step);

                updateTaskStatusForStep(taskId, step.getStepType());

                StepRequest request = new StepRequest();
                request.setTaskId(taskId);
                request.setStepId(step.getId());
                request.setStepType(step.getStepType());
                request.setAdapterType("codeup");
                request.setRepoId(task.getRepoId());
                request.setBranchName(task.getBranchName());
                request.setRequirement(task.getRequirement());

                try {
                    String json = objectMapper.writeValueAsString(request);
                    kafkaTemplate.send(KafkaConfig.TOPIC_STEP_REQUEST, String.valueOf(taskId), json);
                    log.info("派发步骤: task={}, step={}, type={}", taskId, step.getId(), step.getStepType());
                } catch (Exception e) {
                    log.error("派发步骤失败", e);
                }
                return;
            }
        }
        // 所有步骤完成
        log.info("所有步骤已完成: task={}", taskId);
    }

    private void createSteps(TaskDO task) {
        StepType[] stepSequence = {
                StepType.ANALYZE, StepType.PLAN, StepType.RISK_ASSESS_INIT,
                StepType.GENERATE_CONTRACT, StepType.GENERATE_CODE,
                StepType.REVIEW, StepType.RISK_ASSESS_FINAL,
                StepType.COMMIT, StepType.CREATE_MR
        };
        for (int i = 0; i < stepSequence.length; i++) {
            TaskStepDO step = new TaskStepDO();
            step.setTaskId(task.getId());
            step.setStepType(stepSequence[i].name());
            step.setStepOrder(i + 1);
            step.setStatus(StepStatus.PENDING.name());
            step.setInputTokens(0L);
            step.setOutputTokens(0L);
            step.setRetryCount(0);
            taskStepMapper.insert(step);
        }
    }

    private void updateTaskStatusForStep(Long taskId, String stepType) {
        try {
            StepType st = StepType.valueOf(stepType);
            TaskStatus targetStatus = switch (st) {
                case ANALYZE -> TaskStatus.ANALYZING;
                case PLAN, RISK_ASSESS_INIT -> TaskStatus.PLANNING;
                case GENERATE_CONTRACT, GENERATE_CODE -> TaskStatus.GENERATING;
                case REVIEW, RISK_ASSESS_FINAL -> TaskStatus.REVIEWING;
                case COMMIT, CREATE_MR -> TaskStatus.DEPLOYING;
                default -> null;
            };
            if (targetStatus != null) {
                TaskDO task = taskService.getTask(taskId);
                TaskStatus current = TaskStatus.valueOf(task.getStatus());
                if (current != targetStatus && com.shulex.forge.engine.orchestration.statemachine.TaskStateMachine.transition(current, targetStatus)) {
                    taskService.transitionStatus(taskId, targetStatus);
                }
            }
        } catch (Exception e) {
            log.debug("状态更新跳过: {}", e.getMessage());
        }
    }
}
```

- [ ] **Step 4: 实现 StepExecutor**

```java
package com.shulex.forge.engine.execution.service;

import com.fasterxml.jackson.databind.ObjectMapper;
import com.shulex.forge.engine.execution.model.StepRequest;
import com.shulex.forge.engine.execution.model.StepResult;
import com.shulex.forge.engine.orchestration.model.RiskLevel;
import com.shulex.forge.engine.orchestration.model.StepType;
import com.shulex.forge.engine.orchestration.service.RiskAssessor;
import lombok.extern.slf4j.Slf4j;
import org.springframework.stereotype.Service;

import java.util.List;

@Slf4j
@Service
public class StepExecutor {

    private final CodeGenerator codeGenerator;
    private final CodeReviewer codeReviewer;
    private final CodeFixer codeFixer;
    private final CodeCommitter codeCommitter;
    private final ContextBuilder contextBuilder;
    private final RiskAssessor riskAssessor;
    private final com.shulex.forge.engine.execution.ai.ClaudeClient claudeClient;
    private final ObjectMapper objectMapper = new ObjectMapper();

    // 暂存生成的代码文件，按 taskId 隔离，支持并发任务
    private final java.util.concurrent.ConcurrentHashMap<Long, List<com.shulex.forge.engine.execution.model.GeneratedCode>> generatedFilesMap = new java.util.concurrent.ConcurrentHashMap<>();

    public StepExecutor(CodeGenerator codeGenerator, CodeReviewer codeReviewer,
                        CodeFixer codeFixer, CodeCommitter codeCommitter,
                        ContextBuilder contextBuilder, RiskAssessor riskAssessor,
                        com.shulex.forge.engine.execution.ai.ClaudeClient claudeClient) {
        this.codeGenerator = codeGenerator;
        this.codeReviewer = codeReviewer;
        this.codeFixer = codeFixer;
        this.codeCommitter = codeCommitter;
        this.contextBuilder = contextBuilder;
        this.riskAssessor = riskAssessor;
        this.claudeClient = claudeClient;
    }

    public StepResult execute(StepRequest request) {
        StepType stepType = StepType.valueOf(request.getStepType());
        log.info("执行步骤: task={}, step={}, type={}", request.getTaskId(), request.getStepId(), stepType);

        try {
            return switch (stepType) {
                case ANALYZE -> executeAnalyze(request);
                case PLAN -> executePlan(request);
                case RISK_ASSESS_INIT -> executeRiskAssessInit(request);
                case GENERATE_CONTRACT, GENERATE_CODE -> executeGenerate(request);
                case REVIEW -> executeReview(request);
                case RISK_ASSESS_FINAL -> executeRiskAssessFinal(request);
                case COMMIT -> executeCommit(request);
                case CREATE_MR -> executeCreateMR(request);
                case FIX -> executeFix(request);
            };
        } catch (Exception e) {
            log.error("步骤执行失败: task={}, step={}", request.getTaskId(), request.getStepId(), e);
            return new StepResult(request.getTaskId(), request.getStepId(),
                    request.getStepType(), false, null, 0, 0, e.getMessage());
        }
    }

    private StepResult executeAnalyze(StepRequest request) {
        String systemPrompt = contextBuilder.buildSystemPrompt("requirement-analysis");
        if (systemPrompt == null) systemPrompt = "分析以下需求，输出结构化技术任务清单。";
        var response = claudeClient.chat(systemPrompt, request.getRequirement());
        return new StepResult(request.getTaskId(), request.getStepId(),
                request.getStepType(), true, response.getContent(),
                response.getInputTokens(), response.getOutputTokens(), null);
    }

    private StepResult executePlan(StepRequest request) {
        var response = claudeClient.chat("你是技术方案规划师。根据需求分析结果生成实施方案。",
                request.getRequirement());
        return new StepResult(request.getTaskId(), request.getStepId(),
                request.getStepType(), true, response.getContent(),
                response.getInputTokens(), response.getOutputTokens(), null);
    }

    private StepResult executeRiskAssessInit(StepRequest request) {
        RiskLevel risk = riskAssessor.initialAssess(request.getRequirement(), "GENERATE");
        return new StepResult(request.getTaskId(), request.getStepId(),
                request.getStepType(), true, "{\"riskLevel\":\"" + risk.name() + "\"}",
                0, 0, null);
    }

    private StepResult executeGenerate(StepRequest request) {
        var result = codeGenerator.generate(request.getAdapterType(), request.getRepoId(),
                request.getBranchName() != null ? request.getBranchName() : "main",
                request.getRequirement());
        generatedFilesMap.put(request.getTaskId(), result.getFiles());
        String output = result.getFiles().size() + " files generated";
        return new StepResult(request.getTaskId(), request.getStepId(),
                request.getStepType(), true, output,
                result.getInputTokens(), result.getOutputTokens(), null);
    }

    private StepResult executeReview(StepRequest request) {
        var files = generatedFilesMap.get(request.getTaskId());
        if (files == null || files.isEmpty()) {
            return new StepResult(request.getTaskId(), request.getStepId(),
                    request.getStepType(), true, "{\"score\":100,\"issues\":[]}",
                    0, 0, null);
        }
        var result = codeReviewer.review(files);

        // 自动修复：如果评分 < 90 且有问题，尝试修复（最多 3 轮）
        int maxFix = 3;
        int round = 0;
        while (result.getScore() < 90 && !result.getIssues().isEmpty() && round < maxFix) {
            round++;
            log.info("自动修复 round {}: score={}", round, result.getScore());
            var fixResult = codeFixer.fix(files, result.getIssues());
            if (!fixResult.getFiles().isEmpty()) {
                files = fixResult.getFiles();
                generatedFilesMap.put(request.getTaskId(), files);
            }
            result = codeReviewer.review(files);
        }

        try {
            String output = objectMapper.writeValueAsString(result);
            return new StepResult(request.getTaskId(), request.getStepId(),
                    request.getStepType(), true, output,
                    result.getInputTokens(), result.getOutputTokens(), null);
        } catch (Exception e) {
            return new StepResult(request.getTaskId(), request.getStepId(),
                    request.getStepType(), true, "{\"score\":" + result.getScore() + "}",
                    result.getInputTokens(), result.getOutputTokens(), null);
        }
    }

    private StepResult executeRiskAssessFinal(StepRequest request) {
        int score = 80;
        var files = generatedFilesMap.get(request.getTaskId());
        int fileCount = files != null ? files.size() : 0;
        RiskLevel risk = riskAssessor.finalAssess(RiskLevel.LOW, score, fileCount);
        return new StepResult(request.getTaskId(), request.getStepId(),
                request.getStepType(), true, "{\"riskLevel\":\"" + risk.name() + "\",\"score\":" + score + "}",
                0, 0, null);
    }

    private StepResult executeCommit(StepRequest request) {
        var files = generatedFilesMap.get(request.getTaskId());
        if (files == null || files.isEmpty()) {
            return new StepResult(request.getTaskId(), request.getStepId(),
                    request.getStepType(), true, "no files to commit", 0, 0, null);
        }
        String branch = codeCommitter.createBranch(request.getAdapterType(), request.getRepoId(), request.getTaskId());
        String commitHash = codeCommitter.commitCode(request.getAdapterType(), request.getRepoId(),
                branch, request.getTaskId(), files);
        return new StepResult(request.getTaskId(), request.getStepId(),
                request.getStepType(), true,
                "{\"branch\":\"" + branch + "\",\"commit\":\"" + commitHash + "\"}",
                0, 0, null);
    }

    private StepResult executeCreateMR(StepRequest request) {
        String branch = "ai/task-" + request.getTaskId();
        Long mrId = codeCommitter.createMergeRequest(request.getAdapterType(), request.getRepoId(),
                branch, request.getTaskId(), request.getRequirement());
        return new StepResult(request.getTaskId(), request.getStepId(),
                request.getStepType(), true,
                "{\"mrId\":" + mrId + "}",
                0, 0, null);
    }

    private StepResult executeFix(StepRequest request) {
        // Fix 通常在 Review 步骤内自动完成
        return new StepResult(request.getTaskId(), request.getStepId(),
                request.getStepType(), true, "fix embedded in review", 0, 0, null);
    }
}
```

- [ ] **Step 5: 实现 StepRequestListener**

```java
package com.shulex.forge.engine.execution.listener;

import com.fasterxml.jackson.databind.ObjectMapper;
import com.shulex.forge.engine.execution.model.StepRequest;
import com.shulex.forge.engine.execution.model.StepResult;
import com.shulex.forge.engine.execution.service.StepExecutor;
import com.shulex.forge.engine.infrastructure.config.KafkaConfig;
import lombok.extern.slf4j.Slf4j;
import org.springframework.kafka.annotation.KafkaListener;
import org.springframework.kafka.core.KafkaTemplate;
import org.springframework.stereotype.Component;

@Slf4j
@Component
public class StepRequestListener {

    private final StepExecutor stepExecutor;
    private final KafkaTemplate<String, String> kafkaTemplate;
    private final ObjectMapper objectMapper = new ObjectMapper();

    public StepRequestListener(StepExecutor stepExecutor, KafkaTemplate<String, String> kafkaTemplate) {
        this.stepExecutor = stepExecutor;
        this.kafkaTemplate = kafkaTemplate;
    }

    @KafkaListener(topics = KafkaConfig.TOPIC_STEP_REQUEST, groupId = "forge-engine-executor")
    public void onStepRequest(String message) {
        try {
            StepRequest request = objectMapper.readValue(message, StepRequest.class);
            log.info("收到步骤请求: task={}, step={}, type={}",
                    request.getTaskId(), request.getStepId(), request.getStepType());

            StepResult result = stepExecutor.execute(request);

            String resultJson = objectMapper.writeValueAsString(result);
            kafkaTemplate.send(KafkaConfig.TOPIC_STEP_RESULT, String.valueOf(request.getTaskId()), resultJson);
        } catch (Exception e) {
            log.error("处理步骤请求失败", e);
        }
    }
}
```

- [ ] **Step 6: 实现 StepResultListener**

```java
package com.shulex.forge.engine.orchestration.listener;

import com.fasterxml.jackson.databind.ObjectMapper;
import com.shulex.forge.engine.execution.model.StepResult;
import com.shulex.forge.engine.infrastructure.config.KafkaConfig;
import com.shulex.forge.engine.infrastructure.entity.TaskStepDO;
import com.shulex.forge.engine.infrastructure.mapper.TaskStepMapper;
import com.shulex.forge.engine.orchestration.model.StepStatus;
import com.shulex.forge.engine.orchestration.model.TaskStatus;
import com.shulex.forge.engine.orchestration.service.TaskDispatcher;
import com.shulex.forge.engine.orchestration.service.TaskService;
import com.shulex.forge.engine.orchestration.service.TokenUsageService;
import lombok.extern.slf4j.Slf4j;
import org.springframework.kafka.annotation.KafkaListener;
import org.springframework.stereotype.Component;

@Slf4j
@Component
public class StepResultListener {

    private final TaskService taskService;
    private final TaskDispatcher taskDispatcher;
    private final TaskStepMapper taskStepMapper;
    private final TokenUsageService tokenUsageService;
    private final ObjectMapper objectMapper = new ObjectMapper();

    public StepResultListener(TaskService taskService, TaskDispatcher taskDispatcher,
                              TaskStepMapper taskStepMapper, TokenUsageService tokenUsageService) {
        this.taskService = taskService;
        this.taskDispatcher = taskDispatcher;
        this.taskStepMapper = taskStepMapper;
        this.tokenUsageService = tokenUsageService;
    }

    @KafkaListener(topics = KafkaConfig.TOPIC_STEP_RESULT, groupId = "forge-engine-orchestrator")
    public void onStepResult(String message) {
        try {
            StepResult result = objectMapper.readValue(message, StepResult.class);
            log.info("收到步骤结果: task={}, step={}, success={}",
                    result.getTaskId(), result.getStepId(), result.isSuccess());

            TaskStepDO step = taskStepMapper.selectById(result.getStepId());
            if (step == null) return;

            step.setStatus(result.isSuccess() ? StepStatus.SUCCESS.name() : StepStatus.FAILED.name());
            step.setOutputSnapshot(result.getOutputData());
            step.setInputTokens(result.getInputTokens());
            step.setOutputTokens(result.getOutputTokens());
            step.setErrorMessage(result.getErrorMessage());
            taskStepMapper.updateById(step);

            // 记录 Token 用量
            if (result.getInputTokens() > 0 || result.getOutputTokens() > 0) {
                tokenUsageService.recordCall(result.getTaskId(), result.getStepId(),
                        "claude-sonnet-4-20250514", result.getStepType(),
                        result.getInputTokens(), result.getOutputTokens(), 0);
                taskService.updateTokenUsage(result.getTaskId(),
                        result.getInputTokens(), result.getOutputTokens());
            }

            if (result.isSuccess()) {
                taskDispatcher.dispatchNextStep(result.getTaskId());
            } else {
                // 重试逻辑：最多 3 次
                if (step.getRetryCount() < 3) {
                    step.setRetryCount(step.getRetryCount() + 1);
                    step.setStatus(StepStatus.PENDING.name());
                    taskStepMapper.updateById(step);
                    taskDispatcher.dispatchNextStep(result.getTaskId());
                } else {
                    taskService.transitionStatus(result.getTaskId(), TaskStatus.FAILED);
                }
            }
        } catch (Exception e) {
            log.error("处理步骤结果失败", e);
        }
    }
}
```

- [ ] **Step 7: 编译验证**

Run: `cd forge-engine && mvn clean compile -q`
Expected: 编译通过

- [ ] **Step 8: Commit**

```bash
git add forge-engine/src/
git commit -m "feat(m4): add Kafka task channel with step dispatch, execution, and result handling"
```

---

### Task 8: 入口层 — Controller + VO + 测试

**Files:**
- Create: `forge-engine/src/main/java/com/shulex/forge/engine/entrance/vo/CreateTaskRequest.java`
- Create: `forge-engine/src/main/java/com/shulex/forge/engine/entrance/vo/TaskVO.java`
- Create: `forge-engine/src/main/java/com/shulex/forge/engine/entrance/vo/TaskStepVO.java`
- Create: `forge-engine/src/main/java/com/shulex/forge/engine/entrance/vo/TokenUsageVO.java`
- Create: `forge-engine/src/main/java/com/shulex/forge/engine/entrance/controller/TaskController.java`
- Create: `forge-engine/src/main/java/com/shulex/forge/engine/entrance/controller/KillSwitchController.java`
- Create: `forge-engine/src/main/java/com/shulex/forge/engine/entrance/controller/TokenUsageController.java`
- Create: `forge-engine/src/test/java/com/shulex/forge/engine/entrance/controller/TaskControllerTest.java`

- [ ] **Step 1: 创建 VOs**

```java
// CreateTaskRequest.java
package com.shulex.forge.engine.entrance.vo;
import jakarta.validation.constraints.NotBlank;
import jakarta.validation.constraints.NotNull;
import lombok.Data;

@Data
public class CreateTaskRequest {
    @NotNull
    private Long tenantId;
    @NotNull
    private Long userId;
    @NotBlank
    private String requirement;
    private String taskType = "GENERATE";
    @NotBlank
    private String repoId;
}
```

```java
// TaskVO.java
package com.shulex.forge.engine.entrance.vo;
import lombok.Builder;
import lombok.Data;
import java.time.LocalDateTime;

@Data
@Builder
public class TaskVO {
    private Long id;
    private Long tenantId;
    private Long userId;
    private String requirement;
    private String taskType;
    private String status;
    private String riskLevel;
    private String repoId;
    private String branchName;
    private Long mrId;
    private Integer reviewScore;
    private Long totalInputTokens;
    private Long totalOutputTokens;
    private LocalDateTime gmtCreate;
}
```

```java
// TaskStepVO.java
package com.shulex.forge.engine.entrance.vo;
import lombok.Builder;
import lombok.Data;

@Data
@Builder
public class TaskStepVO {
    private Long id;
    private String stepType;
    private Integer stepOrder;
    private String status;
    private Long inputTokens;
    private Long outputTokens;
    private Integer retryCount;
    private String errorMessage;
}
```

```java
// TokenUsageVO.java
package com.shulex.forge.engine.entrance.vo;
import lombok.Builder;
import lombok.Data;

@Data
@Builder
public class TokenUsageVO {
    private Long taskId;
    private Long totalInputTokens;
    private Long totalOutputTokens;
}
```

- [ ] **Step 2: 创建 TaskController**

```java
package com.shulex.forge.engine.entrance.controller;

import com.shulex.forge.engine.common.Result;
import com.shulex.forge.engine.entrance.vo.CreateTaskRequest;
import com.shulex.forge.engine.entrance.vo.TaskStepVO;
import com.shulex.forge.engine.entrance.vo.TaskVO;
import com.shulex.forge.engine.infrastructure.entity.TaskDO;
import com.shulex.forge.engine.orchestration.service.TaskDispatcher;
import com.shulex.forge.engine.orchestration.service.TaskService;
import jakarta.validation.Valid;
import org.springframework.web.bind.annotation.*;

import java.util.List;

@RestController
@RequestMapping("/api/tasks")
public class TaskController {

    private final TaskService taskService;
    private final TaskDispatcher taskDispatcher;

    public TaskController(TaskService taskService, TaskDispatcher taskDispatcher) {
        this.taskService = taskService;
        this.taskDispatcher = taskDispatcher;
    }

    @PostMapping
    public Result<TaskVO> createTask(@Valid @RequestBody CreateTaskRequest request) {
        TaskDO task = taskService.createTask(
                request.getTenantId(), request.getUserId(),
                request.getRequirement(), request.getTaskType(), request.getRepoId());
        taskDispatcher.startTask(task.getId());
        return Result.ok(toVO(task));
    }

    @GetMapping("/{taskId}")
    public Result<TaskVO> getTask(@PathVariable("taskId") Long taskId) {
        return Result.ok(toVO(taskService.getTask(taskId)));
    }

    @GetMapping
    public Result<List<TaskVO>> listTasks(
            @RequestParam("tenantId") Long tenantId,
            @RequestParam("userId") Long userId) {
        return Result.ok(taskService.listTasks(tenantId, userId).stream()
                .map(this::toVO).toList());
    }

    @GetMapping("/{taskId}/steps")
    public Result<List<TaskStepVO>> getSteps(@PathVariable("taskId") Long taskId) {
        return Result.ok(taskService.getSteps(taskId).stream()
                .map(s -> TaskStepVO.builder()
                        .id(s.getId())
                        .stepType(s.getStepType())
                        .stepOrder(s.getStepOrder())
                        .status(s.getStatus())
                        .inputTokens(s.getInputTokens())
                        .outputTokens(s.getOutputTokens())
                        .retryCount(s.getRetryCount())
                        .errorMessage(s.getErrorMessage())
                        .build())
                .toList());
    }

    @PostMapping("/{taskId}/cancel")
    public Result<Void> cancelTask(@PathVariable("taskId") Long taskId) {
        taskService.transitionStatus(taskId, com.shulex.forge.engine.orchestration.model.TaskStatus.CANCELLED);
        return Result.ok(null);
    }

    private TaskVO toVO(TaskDO task) {
        return TaskVO.builder()
                .id(task.getId())
                .tenantId(task.getTenantId())
                .userId(task.getUserId())
                .requirement(task.getRequirement())
                .taskType(task.getTaskType())
                .status(task.getStatus())
                .riskLevel(task.getRiskLevel())
                .repoId(task.getRepoId())
                .branchName(task.getBranchName())
                .mrId(task.getMrId())
                .reviewScore(task.getReviewScore())
                .totalInputTokens(task.getTotalInputTokens())
                .totalOutputTokens(task.getTotalOutputTokens())
                .gmtCreate(task.getGmtCreate())
                .build();
    }
}
```

- [ ] **Step 3: 创建 KillSwitchController**

```java
package com.shulex.forge.engine.entrance.controller;

import com.shulex.forge.engine.common.Result;
import com.shulex.forge.engine.orchestration.model.KillSwitchLevel;
import com.shulex.forge.engine.orchestration.service.KillSwitchService;
import org.springframework.web.bind.annotation.*;

import java.util.Map;

@RestController
@RequestMapping("/api/killswitch")
public class KillSwitchController {

    private final KillSwitchService killSwitchService;

    public KillSwitchController(KillSwitchService killSwitchService) {
        this.killSwitchService = killSwitchService;
    }

    @GetMapping
    public Result<Map<String, String>> getStatus() {
        return Result.ok(Map.of("level", killSwitchService.getLevel().name()));
    }

    @PostMapping("/activate")
    public Result<Void> activate(@RequestParam("level") String level) {
        killSwitchService.activate(KillSwitchLevel.valueOf(level));
        return Result.ok(null);
    }

    @PostMapping("/deactivate")
    public Result<Void> deactivate() {
        killSwitchService.deactivate();
        return Result.ok(null);
    }
}
```

- [ ] **Step 4: 创建 TokenUsageController**

```java
package com.shulex.forge.engine.entrance.controller;

import com.shulex.forge.engine.common.Result;
import com.shulex.forge.engine.entrance.vo.TokenUsageVO;
import com.shulex.forge.engine.infrastructure.entity.TaskDO;
import com.shulex.forge.engine.orchestration.service.TaskService;
import org.springframework.web.bind.annotation.*;

@RestController
@RequestMapping("/api/token-usage")
public class TokenUsageController {

    private final TaskService taskService;

    public TokenUsageController(TaskService taskService) {
        this.taskService = taskService;
    }

    @GetMapping("/{taskId}")
    public Result<TokenUsageVO> getUsage(@PathVariable("taskId") Long taskId) {
        TaskDO task = taskService.getTask(taskId);
        return Result.ok(TokenUsageVO.builder()
                .taskId(task.getId())
                .totalInputTokens(task.getTotalInputTokens())
                .totalOutputTokens(task.getTotalOutputTokens())
                .build());
    }
}
```

- [ ] **Step 5: 写 TaskControllerTest**

```java
package com.shulex.forge.engine.entrance.controller;

import com.fasterxml.jackson.databind.ObjectMapper;
import com.shulex.forge.engine.entrance.vo.CreateTaskRequest;
import com.shulex.forge.engine.infrastructure.entity.TaskDO;
import com.shulex.forge.engine.orchestration.service.TaskDispatcher;
import com.shulex.forge.engine.orchestration.service.TaskService;
import org.junit.jupiter.api.Test;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.boot.test.autoconfigure.web.servlet.AutoConfigureMockMvc;
import org.springframework.boot.test.context.SpringBootTest;
import org.springframework.boot.test.mock.mockito.MockBean;
import org.springframework.data.redis.core.StringRedisTemplate;
import org.springframework.http.MediaType;
import org.springframework.kafka.core.KafkaTemplate;
import org.springframework.test.context.ActiveProfiles;
import org.springframework.test.web.servlet.MockMvc;

import static org.mockito.ArgumentMatchers.*;
import static org.mockito.Mockito.*;
import static org.springframework.test.web.servlet.request.MockMvcRequestBuilders.*;
import static org.springframework.test.web.servlet.result.MockMvcResultMatchers.*;

@SpringBootTest
@AutoConfigureMockMvc
@ActiveProfiles("test")
class TaskControllerTest {

    @Autowired
    private MockMvc mockMvc;
    @Autowired
    private ObjectMapper objectMapper;
    @MockBean
    private TaskService taskService;
    @MockBean
    private TaskDispatcher taskDispatcher;
    @MockBean
    private StringRedisTemplate redisTemplate;
    @MockBean
    private KafkaTemplate<String, String> kafkaTemplate;

    @Test
    void createTask_returns200() throws Exception {
        TaskDO task = new TaskDO();
        task.setId(1L);
        task.setTenantId(1L);
        task.setUserId(1L);
        task.setRequirement("创建用户服务");
        task.setTaskType("GENERATE");
        task.setStatus("SUBMITTED");
        task.setRepoId("repo-123");
        task.setTotalInputTokens(0L);
        task.setTotalOutputTokens(0L);

        when(taskService.createTask(eq(1L), eq(1L), eq("创建用户服务"), eq("GENERATE"), eq("repo-123")))
                .thenReturn(task);

        CreateTaskRequest request = new CreateTaskRequest();
        request.setTenantId(1L);
        request.setUserId(1L);
        request.setRequirement("创建用户服务");
        request.setRepoId("repo-123");

        mockMvc.perform(post("/api/tasks")
                        .contentType(MediaType.APPLICATION_JSON)
                        .content(objectMapper.writeValueAsString(request)))
                .andExpect(status().isOk())
                .andExpect(jsonPath("$.data.id").value(1))
                .andExpect(jsonPath("$.data.status").value("SUBMITTED"));

        verify(taskDispatcher).startTask(1L);
    }

    @Test
    void createTask_returns400OnMissingFields() throws Exception {
        mockMvc.perform(post("/api/tasks")
                        .contentType(MediaType.APPLICATION_JSON)
                        .content("{}"))
                .andExpect(status().isBadRequest());
    }

    @Test
    void getTask_returns200() throws Exception {
        TaskDO task = new TaskDO();
        task.setId(1L);
        task.setTenantId(1L);
        task.setUserId(1L);
        task.setRequirement("test");
        task.setStatus("ANALYZING");
        task.setTotalInputTokens(100L);
        task.setTotalOutputTokens(50L);

        when(taskService.getTask(1L)).thenReturn(task);

        mockMvc.perform(get("/api/tasks/1"))
                .andExpect(status().isOk())
                .andExpect(jsonPath("$.data.status").value("ANALYZING"));
    }
}
```

- [ ] **Step 6: 运行全部测试**

Run: `cd forge-engine && mvn test -pl . 2>&1 | tail -20`
Expected: 全部 PASS

- [ ] **Step 7: Commit**

```bash
git add forge-engine/src/
git commit -m "feat(m4): add task, kill switch, and token usage APIs with tests"
```

---

### Task 9: 应用配置 + ForgeEngineApplicationTest 修复 + 全量验证

**Files:**
- Modify: `forge-engine/src/main/resources/application.yml`
- Modify: `forge-engine/src/test/java/com/shulex/forge/engine/ForgeEngineApplicationTest.java`

- [ ] **Step 1: 更新 application.yml**

```yaml
server:
  port: 8081

spring:
  application:
    name: forge-engine
  datasource:
    url: jdbc:mysql://localhost:3306/forge_engine?useSSL=false&allowPublicKeyRetrieval=true&serverTimezone=Asia/Shanghai
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
  kafka:
    bootstrap-servers: localhost:9094
    consumer:
      auto-offset-reset: earliest
      group-id: forge-engine
    producer:
      key-serializer: org.apache.kafka.common.serialization.StringSerializer
      value-serializer: org.apache.kafka.common.serialization.StringSerializer

forge:
  claude:
    api-key: ${CLAUDE_API_KEY:}
    model: claude-sonnet-4-20250514
    base-url: https://api.anthropic.com
    max-tokens: 4096
  specs:
    base-url: http://localhost:8084
  pipeline:
    base-url: http://localhost:8083

mybatis-plus:
  mapper-locations: classpath*:mapper/**/*.xml
  configuration:
    map-underscore-to-camel-case: true
```

- [ ] **Step 2: 修复 ForgeEngineApplicationTest**

```java
package com.shulex.forge.engine;

import org.junit.jupiter.api.Test;
import org.springframework.boot.test.context.SpringBootTest;
import org.springframework.boot.test.mock.mockito.MockBean;
import org.springframework.data.redis.core.StringRedisTemplate;
import org.springframework.kafka.core.KafkaTemplate;
import org.springframework.test.context.ActiveProfiles;

@SpringBootTest
@ActiveProfiles("test")
class ForgeEngineApplicationTest {

    @MockBean
    private StringRedisTemplate redisTemplate;
    @MockBean
    private KafkaTemplate<String, String> kafkaTemplate;

    @Test
    void contextLoads() {
    }
}
```

- [ ] **Step 3: 运行全部测试**

Run: `cd forge-engine && mvn clean test -pl . 2>&1 | tail -20`
Expected: 全部 PASS

- [ ] **Step 4: Commit**

```bash
git add forge-engine/
git commit -m "feat(m4): update config and fix application test"
```

---

## M4 完成标准

- [ ] forge-engine 编译、测试全部通过
- [ ] 状态机驱动：任务经历 SUBMITTED → ANALYZING → PLANNING → GENERATING → REVIEWING → DEPLOYING → DONE
- [ ] Kafka 任务通道：步骤请求通过 forge-engine-step-request 派发，结果通过 forge-engine-step-result 回报
- [ ] Claude 模型接入：通过 ClaudeClient 调用 Anthropic Messages API
- [ ] 代码生成：ContextBuilder 构建上下文 → CodeGenerator 调用 Claude 生成代码 → 解析 file block
- [ ] AI Review：CodeReviewer 评分 + 问题列表，低分自动修复（最多 3 轮）
- [ ] 代码提交：CodeCommitter 创建分支 → 原子提交 → 创建 MR
- [ ] 两阶段风险评估：RiskAssessor 初评（关键词）+ 终评（评分+文件数）
- [ ] Token 用量追踪：TokenUsageService 记录每次模型调用，TaskService 累计汇总
- [ ] 三级紧急停止：KillSwitchService 支持 L1/L2/L3，L1 阻断新任务
- [ ] REST API：任务 CRUD + 步骤查询 + 取消 + 紧急停止 + Token 用量查询
- [ ] 所有变更已 commit
