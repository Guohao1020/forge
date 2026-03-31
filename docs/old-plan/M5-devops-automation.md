# M5 — DevOps 自动化 实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 构建 forge-pipeline DevOps 自动化层，实现流水线模板生成、质量门禁检查、K8s 部署编排、临时/固定环境管理、Webhook 事件驱动、失败自动分析的完整闭环。

**Architecture:** forge-pipeline 在已有的 M2 适配器层（CodeHostingAdapter、CiCdAdapter、ContainerOrchestrationAdapter）之上新增业务服务层。流水线模板引擎根据项目类型生成 YAML 配置并调用 CiCdAdapter 创建流水线；质量门禁协调编译/测试/Review 检查结果决定是否放行部署；部署编排调用 ContainerOrchestrationAdapter 管理 K8s 资源；Webhook 监听器接收代码推送事件驱动整个流程。

**Tech Stack:** Java 17, Spring Boot 3.2, MyBatis Plus 3.5.5, Flyway, OkHttp, K8s client-java, Resilience4j, H2 (test)

**M5 轻量版简化说明：**
- 仅支持 Java 微服务流水线模板（不做 Vue 前端 / SDK 类库）
- 质量门禁仅检查编译+测试通过（不做 SonarQube/Trivy/API 兼容性）
- 简化部署（直接调用 ContainerOrchestrationAdapter，不做 Helm Chart 渲染）
- 临时环境仅管理 K8s Namespace（不做 DB schema clone / Redis 独立实例）
- 不做灰度部署（APISIX traffic-split 延后）
- 不做 Nacos 配置自动发布（延后）
- 失败处理仅获取日志 + 调用 AI 引擎分析（不做自动修复代码回推）

---

## 文件结构总览

```
forge-pipeline/
├── pom.xml                                              ← 已有，无需修改
├── src/main/java/com/shulex/forge/pipeline/
│   ├── ForgePipelineApplication.java                    ← 已有
│   ├── common/                                          ← 已有（Result, ErrorCode 等）
│   ├── adapter/                                         ← 已有（M2 完整适配器层）
│   ├── infrastructure/
│   │   ├── http/RetryableHttpClient.java                ← 已有
│   │   ├── cache/AdapterCacheService.java               ← 已有
│   │   ├── credential/CredentialService.java            ← 已有
│   │   ├── entity/
│   │   │   ├── PipelineExecutionDO.java                 ← 新建：流水线执行记录
│   │   │   ├── DeploymentRecordDO.java                  ← 新建：部署记录
│   │   │   └── EnvironmentDO.java                       ← 新建：环境状态
│   │   ├── mapper/
│   │   │   ├── PipelineExecutionMapper.java             ← 新建
│   │   │   ├── DeploymentRecordMapper.java              ← 新建
│   │   │   └── EnvironmentMapper.java                   ← 新建
│   │   └── config/
│   │       └── MyBatisPlusConfig.java                   ← 新建：时间自动填充
│   ├── devops/
│   │   ├── model/
│   │   │   ├── ProjectType.java                         ← 新建：项目类型枚举
│   │   │   ├── PipelineStage.java                       ← 新建：流水线阶段枚举
│   │   │   ├── QualityGateResult.java                   ← 新建：质量门禁结果
│   │   │   ├── EnvironmentType.java                     ← 新建：环境类型枚举
│   │   │   └── DeploymentStatus.java                    ← 新建：部署状态枚举
│   │   └── service/
│   │       ├── PipelineTemplateService.java             ← 新建：流水线模板引擎
│   │       ├── QualityGateService.java                  ← 新建：质量门禁
│   │       ├── DeploymentService.java                   ← 新建：部署编排
│   │       ├── EnvironmentService.java                  ← 新建：环境管理
│   │       ├── WebhookDispatcher.java                   ← 新建：Webhook 事件分发
│   │       └── FailureAnalyzer.java                     ← 新建：失败分析
│   └── entrance/
│       ├── controller/
│       │   ├── AdapterHealthController.java             ← 已有
│       │   ├── PipelineController.java                  ← 新建：流水线 API
│       │   ├── DeploymentController.java                ← 新建：部署 API
│       │   ├── EnvironmentController.java               ← 新建：环境 API
│       │   └── WebhookController.java                   ← 新建：Webhook 接收
│       └── vo/
│           ├── TriggerPipelineRequest.java              ← 新建
│           ├── PipelineExecutionVO.java                 ← 新建
│           ├── DeployRequest.java                       ← 新建
│           ├── DeploymentRecordVO.java                  ← 新建
│           ├── EnvironmentVO.java                       ← 新建
│           └── CreateEnvironmentRequest.java            ← 新建
├── src/main/resources/
│   ├── application.yml                                  ← 修改：启用 Flyway + 添加 engine 配置
│   └── db/migration/
│       └── V1__init_pipeline_tables.sql                 ← 新建：建表
├── src/test/java/com/shulex/forge/pipeline/
│   ├── devops/service/
│   │   ├── PipelineTemplateServiceTest.java             ← 新建
│   │   ├── QualityGateServiceTest.java                  ← 新建
│   │   ├── DeploymentServiceTest.java                   ← 新建
│   │   ├── EnvironmentServiceTest.java                  ← 新建
│   │   └── WebhookDispatcherTest.java                   ← 新建
│   └── entrance/controller/
│       └── PipelineControllerTest.java                  ← 新建
└── src/test/resources/
    ├── application-test.yml                             ← 新建
    └── db/test-migration/
        ├── V1__init_pipeline_tables.sql                 ← 新建（H2）
        └── V2__seed_test_data.sql                       ← 新建
```

---

### Task 1: 数据库 + 实体层 + Mapper + 配置 + 测试基础设施

**Files:**
- Create: `forge-pipeline/src/main/resources/db/migration/V1__init_pipeline_tables.sql`
- Create: `forge-pipeline/src/main/java/com/shulex/forge/pipeline/infrastructure/entity/PipelineExecutionDO.java`
- Create: `forge-pipeline/src/main/java/com/shulex/forge/pipeline/infrastructure/entity/DeploymentRecordDO.java`
- Create: `forge-pipeline/src/main/java/com/shulex/forge/pipeline/infrastructure/entity/EnvironmentDO.java`
- Create: `forge-pipeline/src/main/java/com/shulex/forge/pipeline/infrastructure/mapper/PipelineExecutionMapper.java`
- Create: `forge-pipeline/src/main/java/com/shulex/forge/pipeline/infrastructure/mapper/DeploymentRecordMapper.java`
- Create: `forge-pipeline/src/main/java/com/shulex/forge/pipeline/infrastructure/mapper/EnvironmentMapper.java`
- Create: `forge-pipeline/src/main/java/com/shulex/forge/pipeline/infrastructure/config/MyBatisPlusConfig.java`
- Modify: `forge-pipeline/src/main/resources/application.yml`
- Modify: `forge-pipeline/src/test/resources/application-test.yml`
- Create: `forge-pipeline/src/test/resources/db/test-migration/V1__init_pipeline_tables.sql`
- Create: `forge-pipeline/src/test/resources/db/test-migration/V2__seed_test_data.sql`

- [ ] **Step 1: 创建 V1__init_pipeline_tables.sql（MySQL）**

```sql
-- 流水线执行记录
CREATE TABLE pipeline_execution (
    id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY COMMENT 'ID',
    tenant_id BIGINT UNSIGNED NOT NULL COMMENT '租户ID',
    repo_id VARCHAR(128) NOT NULL COMMENT '仓库ID',
    branch VARCHAR(128) NOT NULL COMMENT '分支',
    pipeline_id VARCHAR(128) DEFAULT NULL COMMENT '外部流水线ID',
    run_id VARCHAR(128) DEFAULT NULL COMMENT '执行ID',
    project_type VARCHAR(32) NOT NULL DEFAULT 'JAVA_SERVICE' COMMENT '项目类型',
    status VARCHAR(32) NOT NULL DEFAULT 'PENDING' COMMENT '状态: PENDING/RUNNING/SUCCESS/FAILED',
    compile_passed TINYINT DEFAULT NULL COMMENT '编译是否通过',
    test_passed TINYINT DEFAULT NULL COMMENT '测试是否通过',
    review_passed TINYINT DEFAULT NULL COMMENT 'Review是否通过',
    quality_gate_passed TINYINT DEFAULT NULL COMMENT '质量门禁总结果',
    log_url TEXT DEFAULT NULL COMMENT '日志URL',
    error_message TEXT DEFAULT NULL COMMENT '错误信息',
    trigger_type VARCHAR(32) NOT NULL DEFAULT 'WEBHOOK' COMMENT '触发类型: WEBHOOK/MANUAL/AI',
    task_id BIGINT DEFAULT NULL COMMENT '关联AI任务ID',
    gmt_create DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    gmt_modified DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '修改时间',
    INDEX idx_tenant_repo (tenant_id, repo_id),
    INDEX idx_status (status)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin COMMENT='流水线执行记录';

-- 部署记录
CREATE TABLE deployment_record (
    id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY COMMENT 'ID',
    tenant_id BIGINT UNSIGNED NOT NULL COMMENT '租户ID',
    environment_id BIGINT UNSIGNED DEFAULT NULL COMMENT '环境ID',
    repo_id VARCHAR(128) NOT NULL COMMENT '仓库ID',
    branch VARCHAR(128) NOT NULL COMMENT '分支',
    image VARCHAR(256) NOT NULL COMMENT '镜像地址',
    namespace VARCHAR(128) NOT NULL COMMENT 'K8s Namespace',
    deployment_name VARCHAR(128) NOT NULL COMMENT '部署名',
    replicas INT NOT NULL DEFAULT 1 COMMENT '副本数',
    status VARCHAR(32) NOT NULL DEFAULT 'PENDING' COMMENT '状态: PENDING/DEPLOYING/RUNNING/FAILED/DESTROYED',
    error_message TEXT DEFAULT NULL COMMENT '错误信息',
    pipeline_execution_id BIGINT DEFAULT NULL COMMENT '关联流水线执行ID',
    gmt_create DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    gmt_modified DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '修改时间',
    INDEX idx_tenant (tenant_id),
    INDEX idx_environment (environment_id),
    INDEX idx_status (status)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin COMMENT='部署记录';

-- 环境状态
CREATE TABLE environment (
    id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY COMMENT 'ID',
    tenant_id BIGINT UNSIGNED NOT NULL COMMENT '租户ID',
    name VARCHAR(64) NOT NULL COMMENT '环境名: dev/staging/prod/temp-xxx',
    env_type VARCHAR(32) NOT NULL COMMENT '类型: FIXED/TEMPORARY',
    namespace VARCHAR(128) NOT NULL COMMENT 'K8s Namespace',
    bound_branch VARCHAR(128) DEFAULT NULL COMMENT '绑定分支',
    status VARCHAR(32) NOT NULL DEFAULT 'ACTIVE' COMMENT '状态: ACTIVE/DESTROYING/DESTROYED',
    auto_destroy_at DATETIME DEFAULT NULL COMMENT '自动销毁时间（临时环境）',
    repo_id VARCHAR(128) DEFAULT NULL COMMENT '关联仓库ID',
    task_id BIGINT DEFAULT NULL COMMENT '关联AI任务ID',
    gmt_create DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    gmt_modified DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '修改时间',
    INDEX idx_tenant (tenant_id),
    INDEX idx_env_type_status (env_type, status)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin COMMENT='环境状态';
```

- [ ] **Step 2: 创建 3 个 Entity**

```java
// PipelineExecutionDO.java
package com.shulex.forge.pipeline.infrastructure.entity;

import com.baomidou.mybatisplus.annotation.*;
import lombok.Data;
import java.time.LocalDateTime;

@Data
@TableName("pipeline_execution")
public class PipelineExecutionDO {
    @TableId(type = IdType.AUTO)
    private Long id;
    private Long tenantId;
    private String repoId;
    private String branch;
    private String pipelineId;
    private String runId;
    private String projectType;
    private String status;
    private Integer compilePassed;
    private Integer testPassed;
    private Integer reviewPassed;
    private Integer qualityGatePassed;
    private String logUrl;
    private String errorMessage;
    private String triggerType;
    private Long taskId;
    @TableField(fill = FieldFill.INSERT)
    private LocalDateTime gmtCreate;
    @TableField(fill = FieldFill.INSERT_UPDATE)
    private LocalDateTime gmtModified;
}
```

```java
// DeploymentRecordDO.java
package com.shulex.forge.pipeline.infrastructure.entity;

import com.baomidou.mybatisplus.annotation.*;
import lombok.Data;
import java.time.LocalDateTime;

@Data
@TableName("deployment_record")
public class DeploymentRecordDO {
    @TableId(type = IdType.AUTO)
    private Long id;
    private Long tenantId;
    private Long environmentId;
    private String repoId;
    private String branch;
    private String image;
    private String namespace;
    private String deploymentName;
    private Integer replicas;
    private String status;
    private String errorMessage;
    private Long pipelineExecutionId;
    @TableField(fill = FieldFill.INSERT)
    private LocalDateTime gmtCreate;
    @TableField(fill = FieldFill.INSERT_UPDATE)
    private LocalDateTime gmtModified;
}
```

```java
// EnvironmentDO.java
package com.shulex.forge.pipeline.infrastructure.entity;

import com.baomidou.mybatisplus.annotation.*;
import lombok.Data;
import java.time.LocalDateTime;

@Data
@TableName("environment")
public class EnvironmentDO {
    @TableId(type = IdType.AUTO)
    private Long id;
    private Long tenantId;
    private String name;
    private String envType;
    private String namespace;
    private String boundBranch;
    private String status;
    private LocalDateTime autoDestroyAt;
    private String repoId;
    private Long taskId;
    @TableField(fill = FieldFill.INSERT)
    private LocalDateTime gmtCreate;
    @TableField(fill = FieldFill.INSERT_UPDATE)
    private LocalDateTime gmtModified;
}
```

- [ ] **Step 3: 创建 3 个 Mapper（各自独立文件）**

```java
// PipelineExecutionMapper.java
package com.shulex.forge.pipeline.infrastructure.mapper;
import com.baomidou.mybatisplus.core.mapper.BaseMapper;
import com.shulex.forge.pipeline.infrastructure.entity.PipelineExecutionDO;
import org.apache.ibatis.annotations.Mapper;
@Mapper
public interface PipelineExecutionMapper extends BaseMapper<PipelineExecutionDO> {}

// DeploymentRecordMapper.java
package com.shulex.forge.pipeline.infrastructure.mapper;
import com.baomidou.mybatisplus.core.mapper.BaseMapper;
import com.shulex.forge.pipeline.infrastructure.entity.DeploymentRecordDO;
import org.apache.ibatis.annotations.Mapper;
@Mapper
public interface DeploymentRecordMapper extends BaseMapper<DeploymentRecordDO> {}

// EnvironmentMapper.java
package com.shulex.forge.pipeline.infrastructure.mapper;
import com.baomidou.mybatisplus.core.mapper.BaseMapper;
import com.shulex.forge.pipeline.infrastructure.entity.EnvironmentDO;
import org.apache.ibatis.annotations.Mapper;
@Mapper
public interface EnvironmentMapper extends BaseMapper<EnvironmentDO> {}
```

- [ ] **Step 4: 创建 MyBatisPlusConfig.java**

```java
package com.shulex.forge.pipeline.infrastructure.config;

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

- [ ] **Step 5: 修改 application.yml — 启用 Flyway + 添加 engine 配置**

修改 `forge-pipeline/src/main/resources/application.yml` 中的 `flyway.enabled` 从 `false` 改为 `true`，添加 `flyway.locations`，添加 `forge.engine.base-url` 配置。

完整文件：

```yaml
server:
  port: 8083

spring:
  application:
    name: forge-pipeline
  datasource:
    url: jdbc:mysql://localhost:3306/forge_pipeline?useSSL=false&allowPublicKeyRetrieval=true&serverTimezone=Asia/Shanghai
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
  adapter:
    codeup:
      base-url: https://codeup.aliyun.com
      org-id: ${CODEUP_ORG_ID:}
      access-token: ${CODEUP_ACCESS_TOKEN:}
    flow:
      base-url: https://devops.aliyun.com
      org-id: ${FLOW_ORG_ID:}
      access-token: ${FLOW_ACCESS_TOKEN:}
    ack:
      kube-config-path: ${KUBE_CONFIG_PATH:}
  engine:
    base-url: http://localhost:8081

mybatis-plus:
  mapper-locations: classpath*:mapper/**/*.xml
  configuration:
    map-underscore-to-camel-case: true
```

- [ ] **Step 6: 修改 application-test.yml — 启用 Flyway + 添加 engine 配置**

修改已有的 `forge-pipeline/src/test/resources/application-test.yml`，保留现有 adapter 配置（base-url: http://localhost:8899）不变，仅修改 flyway 和添加 engine 配置。

将 `flyway.enabled: false` 改为：
```yaml
  flyway:
    enabled: true
    locations: classpath:db/test-migration
```

在 `forge.adapter` 同级添加：
```yaml
  engine:
    base-url: http://localhost:8081
```

最终文件内容：
```yaml
spring:
  datasource:
    url: jdbc:h2:mem:forge_pipeline_test;MODE=MYSQL;DB_CLOSE_DELAY=-1
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
  adapter:
    codeup:
      base-url: http://localhost:8899
      org-id: test-org
    flow:
      base-url: http://localhost:8899
      org-id: test-org
    ack:
      kube-config-path: ""
  engine:
    base-url: http://localhost:8081
```

- [ ] **Step 7: 创建 H2 test-migration/V1__init_pipeline_tables.sql**

```sql
CREATE TABLE pipeline_execution (
    id BIGINT AUTO_INCREMENT PRIMARY KEY,
    tenant_id BIGINT NOT NULL,
    repo_id VARCHAR(128) NOT NULL,
    branch VARCHAR(128) NOT NULL,
    pipeline_id VARCHAR(128) DEFAULT NULL,
    run_id VARCHAR(128) DEFAULT NULL,
    project_type VARCHAR(32) NOT NULL DEFAULT 'JAVA_SERVICE',
    status VARCHAR(32) NOT NULL DEFAULT 'PENDING',
    compile_passed TINYINT DEFAULT NULL,
    test_passed TINYINT DEFAULT NULL,
    review_passed TINYINT DEFAULT NULL,
    quality_gate_passed TINYINT DEFAULT NULL,
    log_url CLOB DEFAULT NULL,
    error_message CLOB DEFAULT NULL,
    trigger_type VARCHAR(32) NOT NULL DEFAULT 'WEBHOOK',
    task_id BIGINT DEFAULT NULL,
    gmt_create DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    gmt_modified DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX idx_pe_tenant_repo ON pipeline_execution(tenant_id, repo_id);

CREATE TABLE deployment_record (
    id BIGINT AUTO_INCREMENT PRIMARY KEY,
    tenant_id BIGINT NOT NULL,
    environment_id BIGINT DEFAULT NULL,
    repo_id VARCHAR(128) NOT NULL,
    branch VARCHAR(128) NOT NULL,
    image VARCHAR(256) NOT NULL,
    namespace VARCHAR(128) NOT NULL,
    deployment_name VARCHAR(128) NOT NULL,
    replicas INT NOT NULL DEFAULT 1,
    status VARCHAR(32) NOT NULL DEFAULT 'PENDING',
    error_message CLOB DEFAULT NULL,
    pipeline_execution_id BIGINT DEFAULT NULL,
    gmt_create DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    gmt_modified DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX idx_dr_tenant ON deployment_record(tenant_id);

CREATE TABLE environment (
    id BIGINT AUTO_INCREMENT PRIMARY KEY,
    tenant_id BIGINT NOT NULL,
    name VARCHAR(64) NOT NULL,
    env_type VARCHAR(32) NOT NULL,
    namespace VARCHAR(128) NOT NULL,
    bound_branch VARCHAR(128) DEFAULT NULL,
    status VARCHAR(32) NOT NULL DEFAULT 'ACTIVE',
    auto_destroy_at DATETIME DEFAULT NULL,
    repo_id VARCHAR(128) DEFAULT NULL,
    task_id BIGINT DEFAULT NULL,
    gmt_create DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    gmt_modified DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX idx_env_tenant ON environment(tenant_id);
```

- [ ] **Step 8: 创建 V2__seed_test_data.sql**

```sql
INSERT INTO environment (tenant_id, name, env_type, namespace, bound_branch, status)
VALUES (1, 'dev', 'FIXED', 'forge-dev', 'develop', 'ACTIVE');

INSERT INTO environment (tenant_id, name, env_type, namespace, bound_branch, status)
VALUES (1, 'staging', 'FIXED', 'forge-staging', 'release', 'ACTIVE');
```

- [ ] **Step 9: 编译验证**

Run: `cd forge-pipeline && mvn clean compile -q`
Expected: 编译通过

- [ ] **Step 10: Commit**

```bash
git add forge-pipeline/src/ forge-pipeline/pom.xml
git commit -m "feat(m5): add pipeline tables, entities, mappers, and test infrastructure"
```

---

### Task 2: 枚举模型 + 流水线模板引擎 + 测试

**Files:**
- Create: `forge-pipeline/src/main/java/com/shulex/forge/pipeline/devops/model/ProjectType.java`
- Create: `forge-pipeline/src/main/java/com/shulex/forge/pipeline/devops/model/PipelineStage.java`
- Create: `forge-pipeline/src/main/java/com/shulex/forge/pipeline/devops/model/QualityGateResult.java`
- Create: `forge-pipeline/src/main/java/com/shulex/forge/pipeline/devops/model/EnvironmentType.java`
- Create: `forge-pipeline/src/main/java/com/shulex/forge/pipeline/devops/model/DeploymentStatus.java`
- Create: `forge-pipeline/src/main/java/com/shulex/forge/pipeline/devops/service/PipelineTemplateService.java`
- Create: `forge-pipeline/src/test/java/com/shulex/forge/pipeline/devops/service/PipelineTemplateServiceTest.java`

- [ ] **Step 1: 创建枚举**

```java
// ProjectType.java
package com.shulex.forge.pipeline.devops.model;
public enum ProjectType {
    JAVA_SERVICE, VUE_FRONTEND, SDK_LIBRARY
}

// PipelineStage.java
package com.shulex.forge.pipeline.devops.model;
public enum PipelineStage {
    COMPILE, UNIT_TEST, CODE_SCAN, IMAGE_BUILD, DEPLOY
}

// EnvironmentType.java
package com.shulex.forge.pipeline.devops.model;
public enum EnvironmentType {
    FIXED, TEMPORARY
}

// DeploymentStatus.java
package com.shulex.forge.pipeline.devops.model;
public enum DeploymentStatus {
    PENDING, DEPLOYING, RUNNING, FAILED, DESTROYED
}
```

```java
// QualityGateResult.java
package com.shulex.forge.pipeline.devops.model;

import lombok.AllArgsConstructor;
import lombok.Builder;
import lombok.Data;
import lombok.NoArgsConstructor;

@Data
@Builder
@NoArgsConstructor
@AllArgsConstructor
public class QualityGateResult {
    private boolean compilePassed;
    private boolean testPassed;
    private boolean reviewPassed;
    private boolean overallPassed;
    private String failureReason;
}
```

- [ ] **Step 2: 写 PipelineTemplateServiceTest**

```java
package com.shulex.forge.pipeline.devops.service;

import com.shulex.forge.pipeline.devops.model.ProjectType;
import org.junit.jupiter.api.Test;
import static org.assertj.core.api.Assertions.assertThat;

class PipelineTemplateServiceTest {

    private final PipelineTemplateService service = new PipelineTemplateService();

    @Test
    void generateTemplate_javaService_containsCompileAndTest() {
        String yaml = service.generateTemplate(ProjectType.JAVA_SERVICE, "forge-engine", "main");
        assertThat(yaml).contains("mvn clean compile");
        assertThat(yaml).contains("mvn test");
        assertThat(yaml).contains("docker build");
    }

    @Test
    void generateTemplate_javaService_containsImagePush() {
        String yaml = service.generateTemplate(ProjectType.JAVA_SERVICE, "forge-engine", "main");
        assertThat(yaml).contains("docker push");
    }

    @Test
    void generateTemplate_javaService_containsProjectName() {
        String yaml = service.generateTemplate(ProjectType.JAVA_SERVICE, "my-service", "develop");
        assertThat(yaml).contains("my-service");
    }
}
```

- [ ] **Step 3: 运行测试确认失败**

Run: `cd forge-pipeline && mvn test -Dtest=PipelineTemplateServiceTest -pl . -o 2>&1 | tail -10`
Expected: 编译失败

- [ ] **Step 4: 实现 PipelineTemplateService**

```java
package com.shulex.forge.pipeline.devops.service;

import com.shulex.forge.pipeline.devops.model.ProjectType;
import lombok.extern.slf4j.Slf4j;
import org.springframework.stereotype.Service;

@Slf4j
@Service
public class PipelineTemplateService {

    public String generateTemplate(ProjectType projectType, String projectName, String branch) {
        return switch (projectType) {
            case JAVA_SERVICE -> generateJavaServiceTemplate(projectName, branch);
            case VUE_FRONTEND -> generateVueFrontendTemplate(projectName, branch);
            case SDK_LIBRARY -> generateSdkLibraryTemplate(projectName, branch);
        };
    }

    private String generateJavaServiceTemplate(String projectName, String branch) {
        return """
                name: %s-pipeline
                trigger:
                  branch: %s
                stages:
                  - name: compile
                    steps:
                      - run: mvn clean compile -q
                  - name: unit-test
                    steps:
                      - run: mvn test
                  - name: image-build
                    steps:
                      - run: docker build -t registry.cn-hangzhou.aliyuncs.com/forge/%s:${BUILD_NUMBER} .
                      - run: docker push registry.cn-hangzhou.aliyuncs.com/forge/%s:${BUILD_NUMBER}
                  - name: deploy
                    steps:
                      - run: echo "deploy triggered"
                """.formatted(projectName, branch, projectName, projectName);
    }

    private String generateVueFrontendTemplate(String projectName, String branch) {
        return """
                name: %s-pipeline
                trigger:
                  branch: %s
                stages:
                  - name: install
                    steps:
                      - run: npm install
                  - name: build
                    steps:
                      - run: npm run build
                  - name: image-build
                    steps:
                      - run: docker build -t registry.cn-hangzhou.aliyuncs.com/forge/%s:${BUILD_NUMBER} .
                      - run: docker push registry.cn-hangzhou.aliyuncs.com/forge/%s:${BUILD_NUMBER}
                """.formatted(projectName, branch, projectName, projectName);
    }

    private String generateSdkLibraryTemplate(String projectName, String branch) {
        return """
                name: %s-pipeline
                trigger:
                  branch: %s
                stages:
                  - name: compile
                    steps:
                      - run: mvn clean compile -q
                  - name: unit-test
                    steps:
                      - run: mvn test
                  - name: publish
                    steps:
                      - run: mvn deploy -DskipTests
                """.formatted(projectName, branch);
    }
}
```

- [ ] **Step 5: 运行测试确认通过**

Run: `cd forge-pipeline && mvn test -Dtest=PipelineTemplateServiceTest -pl . -o 2>&1 | tail -10`
Expected: 全部 PASS

- [ ] **Step 6: Commit**

```bash
git add forge-pipeline/src/
git commit -m "feat(m5): add pipeline template engine and domain enums"
```

---

### Task 3: 质量门禁 + 部署服务 + 测试

**Files:**
- Create: `forge-pipeline/src/main/java/com/shulex/forge/pipeline/devops/service/QualityGateService.java`
- Create: `forge-pipeline/src/main/java/com/shulex/forge/pipeline/devops/service/DeploymentService.java`
- Create: `forge-pipeline/src/test/java/com/shulex/forge/pipeline/devops/service/QualityGateServiceTest.java`
- Create: `forge-pipeline/src/test/java/com/shulex/forge/pipeline/devops/service/DeploymentServiceTest.java`

- [ ] **Step 1: 写 QualityGateServiceTest**

```java
package com.shulex.forge.pipeline.devops.service;

import com.shulex.forge.pipeline.devops.model.QualityGateResult;
import com.shulex.forge.pipeline.infrastructure.entity.PipelineExecutionDO;
import com.shulex.forge.pipeline.infrastructure.mapper.PipelineExecutionMapper;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;

import static org.assertj.core.api.Assertions.assertThat;
import static org.mockito.Mockito.*;

class QualityGateServiceTest {

    private QualityGateService qualityGateService;
    private PipelineExecutionMapper executionMapper;

    @BeforeEach
    void setUp() {
        executionMapper = mock(PipelineExecutionMapper.class);
        qualityGateService = new QualityGateService(executionMapper);
    }

    @Test
    void evaluate_allPassed() {
        QualityGateResult result = qualityGateService.evaluate(true, true, true);
        assertThat(result.isOverallPassed()).isTrue();
    }

    @Test
    void evaluate_compileFailed() {
        QualityGateResult result = qualityGateService.evaluate(false, true, true);
        assertThat(result.isOverallPassed()).isFalse();
        assertThat(result.getFailureReason()).contains("编译");
    }

    @Test
    void evaluate_testFailed() {
        QualityGateResult result = qualityGateService.evaluate(true, false, true);
        assertThat(result.isOverallPassed()).isFalse();
        assertThat(result.getFailureReason()).contains("测试");
    }

    @Test
    void updateExecution_savesGateResult() {
        PipelineExecutionDO exec = new PipelineExecutionDO();
        exec.setId(1L);
        when(executionMapper.selectById(1L)).thenReturn(exec);
        when(executionMapper.updateById(any())).thenReturn(1);

        QualityGateResult result = QualityGateResult.builder()
                .compilePassed(true).testPassed(true).reviewPassed(true).overallPassed(true).build();

        qualityGateService.updateExecution(1L, result);
        verify(executionMapper).updateById(any());
    }
}
```

- [ ] **Step 2: 实现 QualityGateService**

```java
package com.shulex.forge.pipeline.devops.service;

import com.shulex.forge.pipeline.devops.model.QualityGateResult;
import com.shulex.forge.pipeline.infrastructure.entity.PipelineExecutionDO;
import com.shulex.forge.pipeline.infrastructure.mapper.PipelineExecutionMapper;
import lombok.extern.slf4j.Slf4j;
import org.springframework.stereotype.Service;

@Slf4j
@Service
public class QualityGateService {

    private final PipelineExecutionMapper executionMapper;

    public QualityGateService(PipelineExecutionMapper executionMapper) {
        this.executionMapper = executionMapper;
    }

    public QualityGateResult evaluate(boolean compilePassed, boolean testPassed, boolean reviewPassed) {
        StringBuilder failureReason = new StringBuilder();
        if (!compilePassed) failureReason.append("编译未通过; ");
        if (!testPassed) failureReason.append("测试未通过; ");
        if (!reviewPassed) failureReason.append("Review未通过; ");

        boolean overall = compilePassed && testPassed && reviewPassed;

        log.info("质量门禁评估: compile={}, test={}, review={}, overall={}",
                compilePassed, testPassed, reviewPassed, overall);

        return QualityGateResult.builder()
                .compilePassed(compilePassed)
                .testPassed(testPassed)
                .reviewPassed(reviewPassed)
                .overallPassed(overall)
                .failureReason(failureReason.length() > 0 ? failureReason.toString().trim() : null)
                .build();
    }

    public void updateExecution(Long executionId, QualityGateResult result) {
        PipelineExecutionDO exec = executionMapper.selectById(executionId);
        if (exec == null) return;
        exec.setCompilePassed(result.isCompilePassed() ? 1 : 0);
        exec.setTestPassed(result.isTestPassed() ? 1 : 0);
        exec.setReviewPassed(result.isReviewPassed() ? 1 : 0);
        exec.setQualityGatePassed(result.isOverallPassed() ? 1 : 0);
        executionMapper.updateById(exec);
    }
}
```

- [ ] **Step 3: 写 DeploymentServiceTest**

```java
package com.shulex.forge.pipeline.devops.service;

import com.shulex.forge.pipeline.adapter.spi.AdapterRegistry;
import com.shulex.forge.pipeline.adapter.spi.ContainerOrchestrationAdapter;
import com.shulex.forge.pipeline.infrastructure.entity.DeploymentRecordDO;
import com.shulex.forge.pipeline.infrastructure.mapper.DeploymentRecordMapper;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;

import java.util.Map;

import static org.assertj.core.api.Assertions.assertThat;
import static org.mockito.ArgumentMatchers.*;
import static org.mockito.Mockito.*;

class DeploymentServiceTest {

    private DeploymentService deploymentService;
    private DeploymentRecordMapper recordMapper;
    private AdapterRegistry adapterRegistry;
    private ContainerOrchestrationAdapter containerAdapter;

    @BeforeEach
    void setUp() {
        recordMapper = mock(DeploymentRecordMapper.class);
        adapterRegistry = mock(AdapterRegistry.class);
        containerAdapter = mock(ContainerOrchestrationAdapter.class);
        when(adapterRegistry.getContainerAdapter("ack")).thenReturn(containerAdapter);
        deploymentService = new DeploymentService(recordMapper, adapterRegistry);
    }

    @Test
    void deploy_createsDeploymentAndRecord() {
        when(recordMapper.insert(any())).thenReturn(1);

        DeploymentRecordDO record = deploymentService.deploy(
                1L, "forge-dev", "forge-engine",
                "registry.cn-hangzhou.aliyuncs.com/forge/forge-engine:1",
                "repo-123", "main", 1, null);

        assertThat(record.getStatus()).isEqualTo("DEPLOYING");
        verify(containerAdapter).createOrUpdateDeployment(
                eq("forge-dev"), eq("forge-engine"),
                eq("registry.cn-hangzhou.aliyuncs.com/forge/forge-engine:1"),
                eq(1), any());
    }

    @Test
    void deploy_createsServiceForDeployment() {
        when(recordMapper.insert(any())).thenReturn(1);

        deploymentService.deploy(1L, "forge-dev", "forge-engine",
                "img:1", "repo", "main", 1, null);

        verify(containerAdapter).createOrUpdateService(
                eq("forge-dev"), eq("forge-engine"),
                eq("ClusterIP"), any(), eq(8080), eq(8080));
    }
}
```

- [ ] **Step 4: 实现 DeploymentService**

```java
package com.shulex.forge.pipeline.devops.service;

import com.shulex.forge.pipeline.adapter.spi.AdapterRegistry;
import com.shulex.forge.pipeline.adapter.spi.ContainerOrchestrationAdapter;
import com.shulex.forge.pipeline.infrastructure.entity.DeploymentRecordDO;
import com.shulex.forge.pipeline.infrastructure.mapper.DeploymentRecordMapper;
import lombok.extern.slf4j.Slf4j;
import org.springframework.stereotype.Service;

import java.util.Map;

@Slf4j
@Service
public class DeploymentService {

    private final DeploymentRecordMapper recordMapper;
    private final AdapterRegistry adapterRegistry;

    public DeploymentService(DeploymentRecordMapper recordMapper, AdapterRegistry adapterRegistry) {
        this.recordMapper = recordMapper;
        this.adapterRegistry = adapterRegistry;
    }

    public DeploymentRecordDO deploy(Long tenantId, String namespace, String deploymentName,
                                      String image, String repoId, String branch,
                                      int replicas, Long pipelineExecutionId) {
        DeploymentRecordDO record = new DeploymentRecordDO();
        record.setTenantId(tenantId);
        record.setNamespace(namespace);
        record.setDeploymentName(deploymentName);
        record.setImage(image);
        record.setRepoId(repoId);
        record.setBranch(branch);
        record.setReplicas(replicas);
        record.setStatus("DEPLOYING");
        record.setPipelineExecutionId(pipelineExecutionId);
        recordMapper.insert(record);

        try {
            ContainerOrchestrationAdapter adapter = adapterRegistry.getContainerAdapter("ack");
            adapter.createOrUpdateDeployment(namespace, deploymentName, image, replicas,
                    Map.of("APP_NAME", deploymentName));
            adapter.createOrUpdateService(namespace, deploymentName, "ClusterIP",
                    Map.of("app", deploymentName), 8080, 8080);
            record.setStatus("RUNNING");
            log.info("部署成功: namespace={}, name={}", namespace, deploymentName);
        } catch (Exception e) {
            record.setStatus("FAILED");
            record.setErrorMessage(e.getMessage());
            log.error("部署失败: namespace={}, name={}", namespace, deploymentName, e);
        }

        recordMapper.updateById(record);
        return record;
    }
}
```

- [ ] **Step 5: 运行测试**

Run: `cd forge-pipeline && mvn test -Dtest=QualityGateServiceTest,DeploymentServiceTest -pl . -o 2>&1 | tail -10`
Expected: 全部 PASS

- [ ] **Step 6: Commit**

```bash
git add forge-pipeline/src/
git commit -m "feat(m5): add quality gate service and deployment orchestration"
```

---

### Task 4: 环境管理 + 测试

**Files:**
- Create: `forge-pipeline/src/main/java/com/shulex/forge/pipeline/devops/service/EnvironmentService.java`
- Create: `forge-pipeline/src/test/java/com/shulex/forge/pipeline/devops/service/EnvironmentServiceTest.java`

- [ ] **Step 1: 写 EnvironmentServiceTest**

```java
package com.shulex.forge.pipeline.devops.service;

import com.shulex.forge.pipeline.adapter.spi.AdapterRegistry;
import com.shulex.forge.pipeline.adapter.spi.ContainerOrchestrationAdapter;
import com.shulex.forge.pipeline.devops.model.EnvironmentType;
import com.shulex.forge.pipeline.infrastructure.entity.EnvironmentDO;
import com.shulex.forge.pipeline.infrastructure.mapper.EnvironmentMapper;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;

import java.util.Map;

import static org.assertj.core.api.Assertions.assertThat;
import static org.mockito.ArgumentMatchers.*;
import static org.mockito.Mockito.*;

class EnvironmentServiceTest {

    private EnvironmentService environmentService;
    private EnvironmentMapper environmentMapper;
    private AdapterRegistry adapterRegistry;
    private ContainerOrchestrationAdapter containerAdapter;

    @BeforeEach
    void setUp() {
        environmentMapper = mock(EnvironmentMapper.class);
        adapterRegistry = mock(AdapterRegistry.class);
        containerAdapter = mock(ContainerOrchestrationAdapter.class);
        when(adapterRegistry.getContainerAdapter("ack")).thenReturn(containerAdapter);
        environmentService = new EnvironmentService(environmentMapper, adapterRegistry);
    }

    @Test
    void createTemporaryEnvironment_createsNamespace() {
        when(environmentMapper.insert(any())).thenReturn(1);

        EnvironmentDO env = environmentService.createTemporaryEnvironment(
                1L, "repo-123", "ai/feature-login", 100L);

        assertThat(env.getEnvType()).isEqualTo(EnvironmentType.TEMPORARY.name());
        assertThat(env.getNamespace()).startsWith("temp-");
        assertThat(env.getAutoDestroyAt()).isNotNull();
        verify(containerAdapter).createNamespace(eq(env.getNamespace()), any());
    }

    @Test
    void destroyEnvironment_deletesNamespace() {
        EnvironmentDO env = new EnvironmentDO();
        env.setId(1L);
        env.setNamespace("temp-123");
        env.setStatus("ACTIVE");
        when(environmentMapper.selectById(1L)).thenReturn(env);
        when(environmentMapper.updateById(any())).thenReturn(1);

        environmentService.destroyEnvironment(1L);

        verify(containerAdapter).deleteNamespace("temp-123");
        assertThat(env.getStatus()).isEqualTo("DESTROYED");
    }

    @Test
    void findFixedEnvironmentByBranch_returnsDev() {
        EnvironmentDO dev = new EnvironmentDO();
        dev.setName("dev");
        dev.setBoundBranch("develop");
        when(environmentMapper.selectOne(any())).thenReturn(dev);

        EnvironmentDO result = environmentService.findFixedEnvironmentByBranch(1L, "develop");
        assertThat(result.getName()).isEqualTo("dev");
    }
}
```

- [ ] **Step 2: 实现 EnvironmentService**

```java
package com.shulex.forge.pipeline.devops.service;

import com.baomidou.mybatisplus.core.conditions.query.LambdaQueryWrapper;
import com.shulex.forge.pipeline.adapter.spi.AdapterRegistry;
import com.shulex.forge.pipeline.adapter.spi.ContainerOrchestrationAdapter;
import com.shulex.forge.pipeline.devops.model.EnvironmentType;
import com.shulex.forge.pipeline.infrastructure.entity.EnvironmentDO;
import com.shulex.forge.pipeline.infrastructure.mapper.EnvironmentMapper;
import lombok.extern.slf4j.Slf4j;
import org.springframework.stereotype.Service;

import java.time.LocalDateTime;
import java.util.List;
import java.util.Map;

@Slf4j
@Service
public class EnvironmentService {

    private final EnvironmentMapper environmentMapper;
    private final AdapterRegistry adapterRegistry;

    public EnvironmentService(EnvironmentMapper environmentMapper, AdapterRegistry adapterRegistry) {
        this.environmentMapper = environmentMapper;
        this.adapterRegistry = adapterRegistry;
    }

    public EnvironmentDO createTemporaryEnvironment(Long tenantId, String repoId, String branch, Long taskId) {
        String namespace = "temp-" + taskId + "-" + System.currentTimeMillis() % 10000;

        EnvironmentDO env = new EnvironmentDO();
        env.setTenantId(tenantId);
        env.setName("temp-task-" + taskId);
        env.setEnvType(EnvironmentType.TEMPORARY.name());
        env.setNamespace(namespace);
        env.setBoundBranch(branch);
        env.setStatus("ACTIVE");
        env.setAutoDestroyAt(LocalDateTime.now().plusMinutes(30));
        env.setRepoId(repoId);
        env.setTaskId(taskId);
        environmentMapper.insert(env);

        try {
            ContainerOrchestrationAdapter adapter = adapterRegistry.getContainerAdapter("ack");
            adapter.createNamespace(namespace, Map.of(
                    "forge.io/env-type", "temporary",
                    "forge.io/task-id", String.valueOf(taskId)
            ));
            log.info("临时环境创建: namespace={}, task={}", namespace, taskId);
        } catch (Exception e) {
            log.error("创建临时环境失败: namespace={}", namespace, e);
            env.setStatus("FAILED");
            environmentMapper.updateById(env);
        }

        return env;
    }

    public void destroyEnvironment(Long environmentId) {
        EnvironmentDO env = environmentMapper.selectById(environmentId);
        if (env == null) return;

        try {
            ContainerOrchestrationAdapter adapter = adapterRegistry.getContainerAdapter("ack");
            adapter.deleteNamespace(env.getNamespace());
            env.setStatus("DESTROYED");
            log.info("环境已销毁: namespace={}", env.getNamespace());
        } catch (Exception e) {
            env.setStatus("DESTROYING");
            log.error("销毁环境失败: namespace={}", env.getNamespace(), e);
        }
        environmentMapper.updateById(env);
    }

    public EnvironmentDO findFixedEnvironmentByBranch(Long tenantId, String branch) {
        return environmentMapper.selectOne(new LambdaQueryWrapper<EnvironmentDO>()
                .eq(EnvironmentDO::getTenantId, tenantId)
                .eq(EnvironmentDO::getEnvType, EnvironmentType.FIXED.name())
                .eq(EnvironmentDO::getBoundBranch, branch)
                .eq(EnvironmentDO::getStatus, "ACTIVE"));
    }

    public List<EnvironmentDO> listEnvironments(Long tenantId) {
        return environmentMapper.selectList(new LambdaQueryWrapper<EnvironmentDO>()
                .eq(EnvironmentDO::getTenantId, tenantId)
                .orderByDesc(EnvironmentDO::getGmtCreate));
    }

    public List<EnvironmentDO> findExpiredTemporaryEnvironments() {
        return environmentMapper.selectList(new LambdaQueryWrapper<EnvironmentDO>()
                .eq(EnvironmentDO::getEnvType, EnvironmentType.TEMPORARY.name())
                .eq(EnvironmentDO::getStatus, "ACTIVE")
                .le(EnvironmentDO::getAutoDestroyAt, LocalDateTime.now()));
    }
}
```

- [ ] **Step 3: 运行测试**

Run: `cd forge-pipeline && mvn test -Dtest=EnvironmentServiceTest -pl . -o 2>&1 | tail -10`
Expected: 全部 PASS

- [ ] **Step 4: Commit**

```bash
git add forge-pipeline/src/
git commit -m "feat(m5): add environment management service with temp/fixed support"
```

---

### Task 5: Webhook 分发 + 失败分析 + 测试

**Files:**
- Create: `forge-pipeline/src/main/java/com/shulex/forge/pipeline/devops/service/WebhookDispatcher.java`
- Create: `forge-pipeline/src/main/java/com/shulex/forge/pipeline/devops/service/FailureAnalyzer.java`
- Create: `forge-pipeline/src/test/java/com/shulex/forge/pipeline/devops/service/WebhookDispatcherTest.java`

- [ ] **Step 1: 写 WebhookDispatcherTest**

```java
package com.shulex.forge.pipeline.devops.service;

import com.shulex.forge.pipeline.infrastructure.entity.PipelineExecutionDO;
import com.shulex.forge.pipeline.infrastructure.mapper.PipelineExecutionMapper;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;

import static org.assertj.core.api.Assertions.assertThat;
import static org.mockito.ArgumentMatchers.any;
import static org.mockito.Mockito.*;

class WebhookDispatcherTest {

    private WebhookDispatcher webhookDispatcher;
    private PipelineExecutionMapper executionMapper;
    private PipelineTemplateService templateService;
    private EnvironmentService environmentService;
    private DeploymentService deploymentService;
    private QualityGateService qualityGateService;

    @BeforeEach
    void setUp() {
        executionMapper = mock(PipelineExecutionMapper.class);
        templateService = mock(PipelineTemplateService.class);
        environmentService = mock(EnvironmentService.class);
        deploymentService = mock(DeploymentService.class);
        qualityGateService = mock(QualityGateService.class);
        webhookDispatcher = new WebhookDispatcher(executionMapper, templateService,
                environmentService, deploymentService, qualityGateService);
    }

    @Test
    void onPush_createsExecutionRecord() {
        when(executionMapper.insert(any())).thenReturn(1);

        webhookDispatcher.onPush(1L, "repo-123", "main", "WEBHOOK");

        verify(executionMapper).insert(any());
    }

    @Test
    void onPush_aisBranch_triggersTemporaryEnvironment() {
        when(executionMapper.insert(any())).thenReturn(1);

        webhookDispatcher.onPush(1L, "repo-123", "ai/feature-login", "AI");

        verify(environmentService).createTemporaryEnvironment(eq(1L), eq("repo-123"), eq("ai/feature-login"), any());
    }
}
```

- [ ] **Step 2: 实现 WebhookDispatcher**

```java
package com.shulex.forge.pipeline.devops.service;

import com.shulex.forge.pipeline.devops.model.ProjectType;
import com.shulex.forge.pipeline.infrastructure.entity.PipelineExecutionDO;
import com.shulex.forge.pipeline.infrastructure.mapper.PipelineExecutionMapper;
import lombok.extern.slf4j.Slf4j;
import org.springframework.stereotype.Service;

@Slf4j
@Service
public class WebhookDispatcher {

    private final PipelineExecutionMapper executionMapper;
    private final PipelineTemplateService templateService;
    private final EnvironmentService environmentService;
    private final DeploymentService deploymentService;
    private final QualityGateService qualityGateService;

    public WebhookDispatcher(PipelineExecutionMapper executionMapper,
                             PipelineTemplateService templateService,
                             EnvironmentService environmentService,
                             DeploymentService deploymentService,
                             QualityGateService qualityGateService) {
        this.executionMapper = executionMapper;
        this.templateService = templateService;
        this.environmentService = environmentService;
        this.deploymentService = deploymentService;
        this.qualityGateService = qualityGateService;
    }

    public void onPush(Long tenantId, String repoId, String branch, String triggerType) {
        log.info("收到推送事件: tenant={}, repo={}, branch={}", tenantId, repoId, branch);

        PipelineExecutionDO execution = new PipelineExecutionDO();
        execution.setTenantId(tenantId);
        execution.setRepoId(repoId);
        execution.setBranch(branch);
        execution.setProjectType(ProjectType.JAVA_SERVICE.name());
        execution.setStatus("PENDING");
        execution.setTriggerType(triggerType);
        executionMapper.insert(execution);

        // AI 分支 → 创建临时环境
        if (branch.startsWith("ai/")) {
            environmentService.createTemporaryEnvironment(tenantId, repoId, branch, null);
        }

        log.info("流水线执行已创建: id={}", execution.getId());
    }

    public void onMergeRequestMerged(Long tenantId, String repoId, String sourceBranch, String targetBranch) {
        log.info("MR 已合并: source={} -> target={}", sourceBranch, targetBranch);
        // 触发目标分支对应环境的部署
        var env = environmentService.findFixedEnvironmentByBranch(tenantId, targetBranch);
        if (env != null) {
            log.info("触发固定环境部署: env={}, branch={}", env.getName(), targetBranch);
        }
    }
}
```

- [ ] **Step 3: 实现 FailureAnalyzer**

```java
package com.shulex.forge.pipeline.devops.service;

import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.ObjectMapper;
import lombok.extern.slf4j.Slf4j;
import okhttp3.*;
import org.springframework.beans.factory.annotation.Value;
import org.springframework.stereotype.Service;

import java.io.IOException;

@Slf4j
@Service
public class FailureAnalyzer {

    private final OkHttpClient httpClient;
    private final ObjectMapper objectMapper;
    private final String engineBaseUrl;

    public FailureAnalyzer(@Value("${forge.engine.base-url}") String engineBaseUrl) {
        this.httpClient = new OkHttpClient();
        this.objectMapper = new ObjectMapper();
        this.engineBaseUrl = engineBaseUrl;
    }

    public String analyzeLogs(String buildLogs) {
        try {
            String body = objectMapper.writeValueAsString(java.util.Map.of(
                    "tenantId", 1,
                    "userId", 1,
                    "requirement", "分析以下构建失败日志并给出修复建议：\n\n" + buildLogs,
                    "taskType", "ITERATE",
                    "repoId", "analysis-only"
            ));

            Request request = new Request.Builder()
                    .url(engineBaseUrl + "/api/tasks")
                    .post(RequestBody.create(body, MediaType.parse("application/json")))
                    .build();

            try (Response response = httpClient.newCall(request).execute()) {
                if (response.isSuccessful() && response.body() != null) {
                    JsonNode root = objectMapper.readTree(response.body().string());
                    Long taskId = root.path("data").path("id").asLong();
                    log.info("失败分析任务已创建: taskId={}", taskId);
                    return "Analysis task created: " + taskId;
                }
            }
        } catch (IOException e) {
            log.error("失败分析请求失败", e);
        }
        return "Failed to create analysis task";
    }
}
```

- [ ] **Step 4: 运行测试**

Run: `cd forge-pipeline && mvn test -Dtest=WebhookDispatcherTest -pl . -o 2>&1 | tail -10`
Expected: 全部 PASS

- [ ] **Step 5: Commit**

```bash
git add forge-pipeline/src/
git commit -m "feat(m5): add webhook dispatcher and failure analyzer"
```

---

### Task 6: API 入口层 (Controllers + VOs) + 测试

**Files:**
- Create: `forge-pipeline/src/main/java/com/shulex/forge/pipeline/entrance/vo/TriggerPipelineRequest.java`
- Create: `forge-pipeline/src/main/java/com/shulex/forge/pipeline/entrance/vo/PipelineExecutionVO.java`
- Create: `forge-pipeline/src/main/java/com/shulex/forge/pipeline/entrance/vo/DeployRequest.java`
- Create: `forge-pipeline/src/main/java/com/shulex/forge/pipeline/entrance/vo/DeploymentRecordVO.java`
- Create: `forge-pipeline/src/main/java/com/shulex/forge/pipeline/entrance/vo/EnvironmentVO.java`
- Create: `forge-pipeline/src/main/java/com/shulex/forge/pipeline/entrance/vo/CreateEnvironmentRequest.java`
- Create: `forge-pipeline/src/main/java/com/shulex/forge/pipeline/entrance/controller/PipelineController.java`
- Create: `forge-pipeline/src/main/java/com/shulex/forge/pipeline/entrance/controller/DeploymentController.java`
- Create: `forge-pipeline/src/main/java/com/shulex/forge/pipeline/entrance/controller/EnvironmentController.java`
- Create: `forge-pipeline/src/main/java/com/shulex/forge/pipeline/entrance/controller/WebhookController.java`
- Create: `forge-pipeline/src/test/java/com/shulex/forge/pipeline/entrance/controller/PipelineControllerTest.java`

- [ ] **Step 1: 创建 VOs**

```java
// TriggerPipelineRequest.java
package com.shulex.forge.pipeline.entrance.vo;
import jakarta.validation.constraints.NotBlank;
import jakarta.validation.constraints.NotNull;
import lombok.Data;

@Data
public class TriggerPipelineRequest {
    @NotNull
    private Long tenantId;
    @NotBlank
    private String repoId;
    @NotBlank
    private String branch;
    private String projectType = "JAVA_SERVICE";
}
```

```java
// PipelineExecutionVO.java
package com.shulex.forge.pipeline.entrance.vo;
import lombok.Builder;
import lombok.Data;
import java.time.LocalDateTime;

@Data
@Builder
public class PipelineExecutionVO {
    private Long id;
    private String repoId;
    private String branch;
    private String projectType;
    private String status;
    private Boolean compilePassed;
    private Boolean testPassed;
    private Boolean reviewPassed;
    private Boolean qualityGatePassed;
    private String triggerType;
    private LocalDateTime gmtCreate;
}
```

```java
// DeployRequest.java
package com.shulex.forge.pipeline.entrance.vo;
import jakarta.validation.constraints.NotBlank;
import jakarta.validation.constraints.NotNull;
import lombok.Data;

@Data
public class DeployRequest {
    @NotNull
    private Long tenantId;
    @NotBlank
    private String namespace;
    @NotBlank
    private String deploymentName;
    @NotBlank
    private String image;
    @NotBlank
    private String repoId;
    @NotBlank
    private String branch;
    private int replicas = 1;
}
```

```java
// DeploymentRecordVO.java
package com.shulex.forge.pipeline.entrance.vo;
import lombok.Builder;
import lombok.Data;
import java.time.LocalDateTime;

@Data
@Builder
public class DeploymentRecordVO {
    private Long id;
    private String namespace;
    private String deploymentName;
    private String image;
    private String status;
    private String branch;
    private LocalDateTime gmtCreate;
}
```

```java
// EnvironmentVO.java
package com.shulex.forge.pipeline.entrance.vo;
import lombok.Builder;
import lombok.Data;
import java.time.LocalDateTime;

@Data
@Builder
public class EnvironmentVO {
    private Long id;
    private String name;
    private String envType;
    private String namespace;
    private String boundBranch;
    private String status;
    private LocalDateTime autoDestroyAt;
    private LocalDateTime gmtCreate;
}
```

```java
// CreateEnvironmentRequest.java
package com.shulex.forge.pipeline.entrance.vo;
import jakarta.validation.constraints.NotBlank;
import jakarta.validation.constraints.NotNull;
import lombok.Data;

@Data
public class CreateEnvironmentRequest {
    @NotNull
    private Long tenantId;
    @NotBlank
    private String repoId;
    @NotBlank
    private String branch;
    private Long taskId;
}
```

- [ ] **Step 2: 创建 PipelineController**

```java
package com.shulex.forge.pipeline.entrance.controller;

import com.shulex.forge.pipeline.common.Result;
import com.shulex.forge.pipeline.devops.service.WebhookDispatcher;
import com.shulex.forge.pipeline.entrance.vo.PipelineExecutionVO;
import com.shulex.forge.pipeline.entrance.vo.TriggerPipelineRequest;
import com.shulex.forge.pipeline.infrastructure.entity.PipelineExecutionDO;
import com.shulex.forge.pipeline.infrastructure.mapper.PipelineExecutionMapper;
import jakarta.validation.Valid;
import com.baomidou.mybatisplus.core.conditions.query.LambdaQueryWrapper;
import org.springframework.web.bind.annotation.*;

import java.util.List;

@RestController
@RequestMapping("/api/pipelines")
public class PipelineController {

    private final WebhookDispatcher webhookDispatcher;
    private final PipelineExecutionMapper executionMapper;

    public PipelineController(WebhookDispatcher webhookDispatcher,
                              PipelineExecutionMapper executionMapper) {
        this.webhookDispatcher = webhookDispatcher;
        this.executionMapper = executionMapper;
    }

    @PostMapping("/trigger")
    public Result<Void> trigger(@Valid @RequestBody TriggerPipelineRequest request) {
        webhookDispatcher.onPush(request.getTenantId(), request.getRepoId(),
                request.getBranch(), "MANUAL");
        return Result.ok(null);
    }

    @GetMapping
    public Result<List<PipelineExecutionVO>> list(
            @RequestParam("tenantId") Long tenantId,
            @RequestParam("repoId") String repoId) {
        List<PipelineExecutionDO> executions = executionMapper.selectList(
                new LambdaQueryWrapper<PipelineExecutionDO>()
                        .eq(PipelineExecutionDO::getTenantId, tenantId)
                        .eq(PipelineExecutionDO::getRepoId, repoId)
                        .orderByDesc(PipelineExecutionDO::getGmtCreate));
        return Result.ok(executions.stream().map(this::toVO).toList());
    }

    @GetMapping("/{id}")
    public Result<PipelineExecutionVO> get(@PathVariable("id") Long id) {
        PipelineExecutionDO exec = executionMapper.selectById(id);
        if (exec == null) return Result.fail(40400, "执行记录不存在");
        return Result.ok(toVO(exec));
    }

    private PipelineExecutionVO toVO(PipelineExecutionDO exec) {
        return PipelineExecutionVO.builder()
                .id(exec.getId())
                .repoId(exec.getRepoId())
                .branch(exec.getBranch())
                .projectType(exec.getProjectType())
                .status(exec.getStatus())
                .compilePassed(exec.getCompilePassed() != null ? exec.getCompilePassed() == 1 : null)
                .testPassed(exec.getTestPassed() != null ? exec.getTestPassed() == 1 : null)
                .reviewPassed(exec.getReviewPassed() != null ? exec.getReviewPassed() == 1 : null)
                .qualityGatePassed(exec.getQualityGatePassed() != null ? exec.getQualityGatePassed() == 1 : null)
                .triggerType(exec.getTriggerType())
                .gmtCreate(exec.getGmtCreate())
                .build();
    }
}
```

- [ ] **Step 3: 创建 DeploymentController**

```java
package com.shulex.forge.pipeline.entrance.controller;

import com.shulex.forge.pipeline.common.Result;
import com.shulex.forge.pipeline.devops.service.DeploymentService;
import com.shulex.forge.pipeline.entrance.vo.DeployRequest;
import com.shulex.forge.pipeline.entrance.vo.DeploymentRecordVO;
import com.shulex.forge.pipeline.infrastructure.entity.DeploymentRecordDO;
import jakarta.validation.Valid;
import org.springframework.web.bind.annotation.*;

@RestController
@RequestMapping("/api/deployments")
public class DeploymentController {

    private final DeploymentService deploymentService;

    public DeploymentController(DeploymentService deploymentService) {
        this.deploymentService = deploymentService;
    }

    @PostMapping
    public Result<DeploymentRecordVO> deploy(@Valid @RequestBody DeployRequest request) {
        DeploymentRecordDO record = deploymentService.deploy(
                request.getTenantId(), request.getNamespace(), request.getDeploymentName(),
                request.getImage(), request.getRepoId(), request.getBranch(),
                request.getReplicas(), null);
        return Result.ok(toVO(record));
    }

    private DeploymentRecordVO toVO(DeploymentRecordDO record) {
        return DeploymentRecordVO.builder()
                .id(record.getId())
                .namespace(record.getNamespace())
                .deploymentName(record.getDeploymentName())
                .image(record.getImage())
                .status(record.getStatus())
                .branch(record.getBranch())
                .gmtCreate(record.getGmtCreate())
                .build();
    }
}
```

- [ ] **Step 4: 创建 EnvironmentController**

```java
package com.shulex.forge.pipeline.entrance.controller;

import com.shulex.forge.pipeline.common.Result;
import com.shulex.forge.pipeline.devops.service.EnvironmentService;
import com.shulex.forge.pipeline.entrance.vo.CreateEnvironmentRequest;
import com.shulex.forge.pipeline.entrance.vo.EnvironmentVO;
import com.shulex.forge.pipeline.infrastructure.entity.EnvironmentDO;
import jakarta.validation.Valid;
import org.springframework.web.bind.annotation.*;

import java.util.List;

@RestController
@RequestMapping("/api/environments")
public class EnvironmentController {

    private final EnvironmentService environmentService;

    public EnvironmentController(EnvironmentService environmentService) {
        this.environmentService = environmentService;
    }

    @PostMapping("/temporary")
    public Result<EnvironmentVO> createTemporary(@Valid @RequestBody CreateEnvironmentRequest request) {
        EnvironmentDO env = environmentService.createTemporaryEnvironment(
                request.getTenantId(), request.getRepoId(), request.getBranch(), request.getTaskId());
        return Result.ok(toVO(env));
    }

    @DeleteMapping("/{id}")
    public Result<Void> destroy(@PathVariable("id") Long id) {
        environmentService.destroyEnvironment(id);
        return Result.ok(null);
    }

    @GetMapping
    public Result<List<EnvironmentVO>> list(@RequestParam("tenantId") Long tenantId) {
        return Result.ok(environmentService.listEnvironments(tenantId).stream()
                .map(this::toVO).toList());
    }

    private EnvironmentVO toVO(EnvironmentDO env) {
        return EnvironmentVO.builder()
                .id(env.getId())
                .name(env.getName())
                .envType(env.getEnvType())
                .namespace(env.getNamespace())
                .boundBranch(env.getBoundBranch())
                .status(env.getStatus())
                .autoDestroyAt(env.getAutoDestroyAt())
                .gmtCreate(env.getGmtCreate())
                .build();
    }
}
```

- [ ] **Step 5: 创建 WebhookController**

```java
package com.shulex.forge.pipeline.entrance.controller;

import com.fasterxml.jackson.databind.JsonNode;
import com.shulex.forge.pipeline.common.Result;
import com.shulex.forge.pipeline.devops.service.WebhookDispatcher;
import lombok.extern.slf4j.Slf4j;
import org.springframework.web.bind.annotation.*;

@Slf4j
@RestController
@RequestMapping("/api/webhooks")
public class WebhookController {

    private final WebhookDispatcher webhookDispatcher;

    public WebhookController(WebhookDispatcher webhookDispatcher) {
        this.webhookDispatcher = webhookDispatcher;
    }

    @PostMapping("/push")
    public Result<Void> onPush(@RequestBody JsonNode payload) {
        log.info("收到 Webhook push 事件");
        String repoId = payload.path("repository").path("id").asText();
        String branch = payload.path("ref").asText("").replace("refs/heads/", "");
        Long tenantId = payload.path("tenantId").asLong(1L);

        if (!branch.isBlank()) {
            webhookDispatcher.onPush(tenantId, repoId, branch, "WEBHOOK");
        }
        return Result.ok(null);
    }

    @PostMapping("/merge-request")
    public Result<Void> onMergeRequest(@RequestBody JsonNode payload) {
        log.info("收到 Webhook MR 事件");
        String action = payload.path("action").asText();
        if ("merge".equals(action)) {
            String repoId = payload.path("repository").path("id").asText();
            String sourceBranch = payload.path("merge_request").path("source_branch").asText();
            String targetBranch = payload.path("merge_request").path("target_branch").asText();
            Long tenantId = payload.path("tenantId").asLong(1L);

            webhookDispatcher.onMergeRequestMerged(tenantId, repoId, sourceBranch, targetBranch);
        }
        return Result.ok(null);
    }
}
```

- [ ] **Step 6: 写 PipelineControllerTest**

```java
package com.shulex.forge.pipeline.entrance.controller;

import com.fasterxml.jackson.databind.ObjectMapper;
import com.shulex.forge.pipeline.devops.service.WebhookDispatcher;
import com.shulex.forge.pipeline.entrance.vo.TriggerPipelineRequest;
import com.shulex.forge.pipeline.infrastructure.mapper.PipelineExecutionMapper;
import org.junit.jupiter.api.Test;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.boot.test.autoconfigure.web.servlet.AutoConfigureMockMvc;
import org.springframework.boot.test.context.SpringBootTest;
import org.springframework.boot.test.mock.mockito.MockBean;
import org.springframework.data.redis.core.StringRedisTemplate;
import org.springframework.http.MediaType;
import org.springframework.test.context.ActiveProfiles;
import org.springframework.test.web.servlet.MockMvc;

import static org.mockito.Mockito.*;
import static org.springframework.test.web.servlet.request.MockMvcRequestBuilders.*;
import static org.springframework.test.web.servlet.result.MockMvcResultMatchers.*;

@SpringBootTest
@AutoConfigureMockMvc
@ActiveProfiles("test")
class PipelineControllerTest {

    @Autowired
    private MockMvc mockMvc;
    @Autowired
    private ObjectMapper objectMapper;
    @MockBean
    private WebhookDispatcher webhookDispatcher;
    @MockBean
    private PipelineExecutionMapper executionMapper;
    @MockBean
    private StringRedisTemplate redisTemplate;

    @Test
    void trigger_returns200() throws Exception {
        TriggerPipelineRequest request = new TriggerPipelineRequest();
        request.setTenantId(1L);
        request.setRepoId("repo-123");
        request.setBranch("main");

        mockMvc.perform(post("/api/pipelines/trigger")
                        .contentType(MediaType.APPLICATION_JSON)
                        .content(objectMapper.writeValueAsString(request)))
                .andExpect(status().isOk());

        verify(webhookDispatcher).onPush(1L, "repo-123", "main", "MANUAL");
    }

    @Test
    void trigger_returns400OnMissingFields() throws Exception {
        mockMvc.perform(post("/api/pipelines/trigger")
                        .contentType(MediaType.APPLICATION_JSON)
                        .content("{}"))
                .andExpect(status().isBadRequest());
    }
}
```

- [ ] **Step 7: 运行全量测试**

Run: `cd forge-pipeline && mvn clean test -pl . 2>&1 | tail -20`
Expected: 全部 PASS

- [ ] **Step 8: Commit**

```bash
git add forge-pipeline/src/
git commit -m "feat(m5): add pipeline, deployment, environment, and webhook APIs with tests"
```

---

### Task 7: 应用启动测试修复 + APISIX 路由 + 全量验证

**Files:**
- Modify: `forge-pipeline/src/test/java/com/shulex/forge/pipeline/ForgePipelineApplicationTest.java`
- Modify: `docker/apisix/apisix.yaml` (添加新路由)

- [ ] **Step 1: 修复 ForgePipelineApplicationTest — 添加必要 MockBean**

现有 `ForgePipelineApplicationTest` 已有 `@MockBean StringRedisTemplate`，但 M5 新增的 `DeploymentService`、`EnvironmentService` 依赖 `AdapterRegistry`，其内部会初始化真实的 K8s / Codeup / Flow 客户端。需要额外 mock `AdapterRegistry` 以防止 contextLoads 加载真实适配器实例。

```java
package com.shulex.forge.pipeline;

import com.shulex.forge.pipeline.adapter.spi.AdapterRegistry;
import org.junit.jupiter.api.Test;
import org.springframework.boot.test.context.SpringBootTest;
import org.springframework.boot.test.mock.mockito.MockBean;
import org.springframework.data.redis.core.StringRedisTemplate;
import org.springframework.test.context.ActiveProfiles;

@SpringBootTest
@ActiveProfiles("test")
class ForgePipelineApplicationTest {

    @MockBean
    private StringRedisTemplate redisTemplate;

    @MockBean
    private AdapterRegistry adapterRegistry;

    @Test
    void contextLoads() {
    }
}
```

- [ ] **Step 2: 运行全量测试**

Run: `cd forge-pipeline && mvn clean test -pl . 2>&1 | tail -20`
Expected: 全部 PASS

- [ ] **Step 3: 更新 APISIX 路由（增量修改 apisix.yaml）**

在 `docker/apisix/apisix.yaml` 中确认 forge-pipeline 路由已存在（M2 时已添加），确保新的 API 路径（/api/pipelines, /api/deployments, /api/environments, /api/webhooks）被已有的 `/api/` 前缀路由覆盖。无需修改（因为 M2 的路由规则已按 prefix 匹配 /api/ 到 forge-pipeline）。

- [ ] **Step 4: Commit**

```bash
git add forge-pipeline/src/ docker/
git commit -m "feat(m5): fix application test and verify APISIX routing"
```

---

## M5 完成标准

- [ ] forge-pipeline 编译、测试全部通过
- [ ] 流水线模板引擎：根据 Java 微服务类型生成 YAML 配置
- [ ] 质量门禁：编译+测试+Review 三项检查，任一不通过则阻断
- [ ] 部署编排：调用 ContainerOrchestrationAdapter 创建 Deployment + Service
- [ ] 临时环境：创建/销毁 K8s Namespace，30min 自动销毁策略
- [ ] 固定环境：dev/staging/prod 分支绑定，按分支查找环境
- [ ] Webhook 接收：push + merge_request 事件分发
- [ ] 失败分析：获取日志调用 AI 引擎分析
- [ ] REST API：/api/pipelines, /api/deployments, /api/environments, /api/webhooks
