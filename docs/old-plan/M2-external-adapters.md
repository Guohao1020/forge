# M2 — 外部平台适配器 实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 构建外部平台适配器层（forge-pipeline），提供代码托管（Codeup）、容器编排（ACK/K8s）、CI/CD（云效 Flow）三类适配器的统一接口定义和首期实现，为 M4（AI 引擎）和 M5（DevOps 自动化）提供平台无关的外部操作能力。

**Architecture:** forge-pipeline 作为适配器宿主服务，采用 SPI 机制注册适配器实现。每个适配器分为接口层（`adapter/spi/`）和实现层（`adapter/codeup/`、`adapter/ack/`、`adapter/flow/`）。适配器内部封装限流（令牌桶）、重试（指数退避）、缓存（Redis）和凭证管理（Nacos 加密配置）。所有外部 API 调用通过 HTTP Client（OkHttp）封装，测试使用 MockWebServer 模拟外部 API。

**Tech Stack:** Java 17, Spring Boot 3.2, OkHttp 4.12, Resilience4j (限流/重试), Redis (缓存), JUnit 5 + MockWebServer

---

## 文件结构总览

```
forge-pipeline/
├── pom.xml                                              ← 补充依赖
├── src/main/java/com/shulex/forge/pipeline/
│   ├── ForgePipelineApplication.java                    ← 已有
│   ├── common/
│   │   ├── Result.java                                  ← 统一响应（同 forge-specs）
│   │   ├── ErrorCode.java                               ← 错误码枚举
│   │   ├── BizException.java                            ← 业务异常
│   │   ├── SysException.java                            ← 系统异常
│   │   └── GlobalExceptionHandler.java                  ← 全局异常处理
│   ├── adapter/
│   │   ├── spi/                                         ← 适配器 SPI 接口
│   │   │   ├── CodeHostingAdapter.java                  ← 代码托管适配器接口
│   │   │   ├── ContainerOrchestrationAdapter.java       ← 容器编排适配器接口
│   │   │   ├── CiCdAdapter.java                         ← CI/CD 适配器接口
│   │   │   └── AdapterRegistry.java                     ← 适配器注册表
│   │   ├── model/                                       ← 适配器通用模型
│   │   │   ├── FileTreeNode.java                        ← 文件树节点
│   │   │   ├── FileContent.java                         ← 文件内容
│   │   │   ├── CommitFile.java                          ← 提交文件变更
│   │   │   ├── BranchInfo.java                          ← 分支信息
│   │   │   ├── MergeRequestInfo.java                    ← 合并请求信息
│   │   │   ├── MergeRequestCreateRequest.java           ← 创建 MR 请求
│   │   │   ├── PipelineInfo.java                        ← 流水线信息
│   │   │   ├── PipelineRunInfo.java                     ← 流水线运行信息
│   │   │   ├── PipelineCreateRequest.java               ← 创建流水线请求
│   │   │   ├── DeploymentInfo.java                      ← 部署信息
│   │   │   ├── ServiceInfo.java                         ← K8s Service 信息
│   │   │   ├── PodInfo.java                             ← Pod 信息
│   │   │   └── WebhookInfo.java                         ← Webhook 信息
│   │   ├── codeup/                                      ← Codeup 实现
│   │   │   ├── CodeupAdapter.java                       ← Codeup 适配器实现
│   │   │   ├── CodeupClient.java                        ← Codeup HTTP 客户端
│   │   │   └── CodeupConfig.java                        ← Codeup 配置
│   │   ├── ack/                                         ← ACK/K8s 实现
│   │   │   ├── AckAdapter.java                          ← ACK 适配器实现
│   │   │   ├── K8sClientFactory.java                    ← K8s 客户端工厂
│   │   │   └── AckConfig.java                           ← ACK 配置
│   │   └── flow/                                        ← 云效 Flow 实现
│   │       ├── FlowAdapter.java                         ← Flow 适配器实现
│   │       ├── FlowClient.java                          ← Flow HTTP 客户端
│   │       └── FlowConfig.java                          ← Flow 配置
│   ├── infrastructure/
│   │   ├── http/
│   │   │   └── RetryableHttpClient.java                 ← 带重试/限流的 HTTP 客户端
│   │   ├── cache/
│   │   │   └── AdapterCacheService.java                 ← 适配器缓存服务（Redis）
│   │   └── credential/
│   │       └── CredentialService.java                   ← 凭证管理服务
│   └── entrance/
│       └── controller/
│           └── AdapterHealthController.java             ← 适配器健康检查 API
├── src/main/resources/
│   └── application.yml                                  ← 更新配置
├── src/test/java/com/shulex/forge/pipeline/
│   ├── adapter/
│   │   ├── spi/
│   │   │   └── AdapterRegistryTest.java                 ← SPI 注册测试
│   │   ├── codeup/
│   │   │   ├── CodeupAdapterTest.java                   ← Codeup 适配器测试
│   │   │   └── CodeupClientTest.java                    ← Codeup 客户端测试
│   │   ├── ack/
│   │   │   └── AckAdapterTest.java                      ← ACK 适配器测试
│   │   └── flow/
│   │       └── FlowAdapterTest.java                     ← Flow 适配器测试
│   └── infrastructure/
│       ├── http/
│       │   └── RetryableHttpClientTest.java             ← HTTP 客户端测试
│       ├── cache/
│       │   └── AdapterCacheServiceTest.java             ← 缓存服务测试
│       └── credential/
│           └── CredentialServiceTest.java               ← 凭证管理测试
└── src/test/resources/
    ├── application-test.yml                             ← 测试配置
    └── mock/                                            ← Mock API 响应
        ├── codeup-tree-response.json
        ├── codeup-file-response.json
        └── codeup-commit-response.json
```

---

### Task 1: 补充依赖 + 公共基础设施

**Files:**
- Modify: `forge-pipeline/pom.xml`
- Create: `forge-pipeline/src/main/java/com/shulex/forge/pipeline/common/Result.java`
- Create: `forge-pipeline/src/main/java/com/shulex/forge/pipeline/common/ErrorCode.java`
- Create: `forge-pipeline/src/main/java/com/shulex/forge/pipeline/common/BizException.java`
- Create: `forge-pipeline/src/main/java/com/shulex/forge/pipeline/common/SysException.java`
- Create: `forge-pipeline/src/main/java/com/shulex/forge/pipeline/common/GlobalExceptionHandler.java`

- [ ] **Step 1: 更新 pom.xml 添加依赖**

```xml
<!-- 在 <dependencies> 中追加 -->
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
<!-- Resilience4j 限流+重试 -->
<dependency>
    <groupId>io.github.resilience4j</groupId>
    <artifactId>resilience4j-ratelimiter</artifactId>
    <version>2.2.0</version>
</dependency>
<dependency>
    <groupId>io.github.resilience4j</groupId>
    <artifactId>resilience4j-retry</artifactId>
    <version>2.2.0</version>
</dependency>
<!-- Redis -->
<dependency>
    <groupId>org.springframework.boot</groupId>
    <artifactId>spring-boot-starter-data-redis</artifactId>
</dependency>
<!-- K8s Client -->
<dependency>
    <groupId>io.kubernetes</groupId>
    <artifactId>client-java</artifactId>
    <version>20.0.1</version>
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
<!-- Test -->
<dependency>
    <groupId>com.h2database</groupId>
    <artifactId>h2</artifactId>
    <scope>test</scope>
</dependency>
<dependency>
    <groupId>com.squareup.okhttp3</groupId>
    <artifactId>mockwebserver</artifactId>
    <scope>test</scope>
</dependency>
```

Note: OkHttp and jackson-databind versions are managed by Spring Boot BOM. Resilience4j needs explicit version. K8s client-java needs explicit version. mybatis-plus, flyway, mysql-connector versions managed by parent pom.

- [ ] **Step 2: 创建 Result.java**

```java
package com.shulex.forge.pipeline.common;

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
package com.shulex.forge.pipeline.common;

import lombok.Getter;
import lombok.AllArgsConstructor;

@Getter
@AllArgsConstructor
public enum ErrorCode {
    NOT_FOUND(40400, "资源不存在"),
    INVALID_PARAM(40000, "参数错误"),
    ADAPTER_ERROR(50100, "适配器调用失败"),
    RATE_LIMITED(42900, "请求频率超限"),
    CREDENTIAL_ERROR(50200, "凭证错误"),
    INTERNAL_ERROR(50000, "系统内部错误");

    private final int code;
    private final String message;
}
```

- [ ] **Step 4: 创建 BizException.java**

```java
package com.shulex.forge.pipeline.common;

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
package com.shulex.forge.pipeline.common;

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
package com.shulex.forge.pipeline.common;

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

Run: `cd forge-pipeline && mvn clean compile -q`
Expected: 编译通过

- [ ] **Step 8: Commit**

```bash
git add forge-pipeline/pom.xml forge-pipeline/src/main/java/com/shulex/forge/pipeline/common/
git commit -m "feat(m2): add forge-pipeline dependencies and common infrastructure"
```

---

### Task 2: 适配器 SPI 接口定义 + 通用模型

**Files:**
- Create: `forge-pipeline/src/main/java/com/shulex/forge/pipeline/adapter/model/FileTreeNode.java`
- Create: `forge-pipeline/src/main/java/com/shulex/forge/pipeline/adapter/model/FileContent.java`
- Create: `forge-pipeline/src/main/java/com/shulex/forge/pipeline/adapter/model/CommitFile.java`
- Create: `forge-pipeline/src/main/java/com/shulex/forge/pipeline/adapter/model/BranchInfo.java`
- Create: `forge-pipeline/src/main/java/com/shulex/forge/pipeline/adapter/model/MergeRequestInfo.java`
- Create: `forge-pipeline/src/main/java/com/shulex/forge/pipeline/adapter/model/MergeRequestCreateRequest.java`
- Create: `forge-pipeline/src/main/java/com/shulex/forge/pipeline/adapter/model/PipelineInfo.java`
- Create: `forge-pipeline/src/main/java/com/shulex/forge/pipeline/adapter/model/PipelineRunInfo.java`
- Create: `forge-pipeline/src/main/java/com/shulex/forge/pipeline/adapter/model/DeploymentInfo.java`
- Create: `forge-pipeline/src/main/java/com/shulex/forge/pipeline/adapter/model/PodInfo.java`
- Create: `forge-pipeline/src/main/java/com/shulex/forge/pipeline/adapter/spi/CodeHostingAdapter.java`
- Create: `forge-pipeline/src/main/java/com/shulex/forge/pipeline/adapter/spi/ContainerOrchestrationAdapter.java`
- Create: `forge-pipeline/src/main/java/com/shulex/forge/pipeline/adapter/spi/CiCdAdapter.java`
- Create: `forge-pipeline/src/main/java/com/shulex/forge/pipeline/adapter/spi/AdapterRegistry.java`

- [ ] **Step 1: 创建 FileTreeNode.java**

```java
package com.shulex.forge.pipeline.adapter.model;

import lombok.Builder;
import lombok.Data;

@Data
@Builder
public class FileTreeNode {
    private String path;
    private String name;
    private String type; // "tree" or "blob"
    private Long size;
}
```

- [ ] **Step 2: 创建 FileContent.java**

```java
package com.shulex.forge.pipeline.adapter.model;

import lombok.Builder;
import lombok.Data;

@Data
@Builder
public class FileContent {
    private String path;
    private String content;
    private String encoding; // "text" or "base64"
    private String sha;
}
```

- [ ] **Step 3: 创建 CommitFile.java**

```java
package com.shulex.forge.pipeline.adapter.model;

import lombok.Builder;
import lombok.Data;

@Data
@Builder
public class CommitFile {
    private String path;
    private String content;
    private String action; // "create", "update", "delete"
}
```

- [ ] **Step 4: 创建 BranchInfo.java**

```java
package com.shulex.forge.pipeline.adapter.model;

import lombok.Builder;
import lombok.Data;

@Data
@Builder
public class BranchInfo {
    private String name;
    private String commitId;
    private boolean isProtected;
}
```

- [ ] **Step 5: 创建 MergeRequestInfo.java**

```java
package com.shulex.forge.pipeline.adapter.model;

import lombok.Builder;
import lombok.Data;

@Data
@Builder
public class MergeRequestInfo {
    private Long id;
    private String title;
    private String description;
    private String sourceBranch;
    private String targetBranch;
    private String state; // "opened", "merged", "closed"
    private String url;
}
```

- [ ] **Step 6: 创建 MergeRequestCreateRequest.java**

```java
package com.shulex.forge.pipeline.adapter.model;

import lombok.Builder;
import lombok.Data;

@Data
@Builder
public class MergeRequestCreateRequest {
    private String title;
    private String description;
    private String sourceBranch;
    private String targetBranch;
}
```

- [ ] **Step 7: 创建 PipelineInfo.java**

```java
package com.shulex.forge.pipeline.adapter.model;

import lombok.Builder;
import lombok.Data;

@Data
@Builder
public class PipelineInfo {
    private String id;
    private String name;
    private String status; // "active", "inactive"
}
```

- [ ] **Step 8: 创建 PipelineRunInfo.java**

```java
package com.shulex.forge.pipeline.adapter.model;

import lombok.Builder;
import lombok.Data;
import java.time.LocalDateTime;

@Data
@Builder
public class PipelineRunInfo {
    private String runId;
    private String pipelineId;
    private String status; // "running", "success", "failed", "cancelled"
    private String triggerType; // "manual", "push"
    private LocalDateTime startTime;
    private LocalDateTime endTime;
    private String logUrl;
}
```

- [ ] **Step 9: 创建 DeploymentInfo.java**

```java
package com.shulex.forge.pipeline.adapter.model;

import lombok.Builder;
import lombok.Data;

@Data
@Builder
public class DeploymentInfo {
    private String namespace;
    private String name;
    private String image;
    private Integer replicas;
    private Integer availableReplicas;
    private String status; // "available", "progressing", "degraded"
}
```

- [ ] **Step 10: 创建 PodInfo.java**

```java
package com.shulex.forge.pipeline.adapter.model;

import lombok.Builder;
import lombok.Data;
import java.time.LocalDateTime;

@Data
@Builder
public class PodInfo {
    private String namespace;
    private String name;
    private String phase; // "Running", "Pending", "Succeeded", "Failed"
    private String nodeName;
    private LocalDateTime startTime;
}
```

- [ ] **Step 10.1: 创建 WebhookInfo.java**

```java
package com.shulex.forge.pipeline.adapter.model;

import lombok.Builder;
import lombok.Data;

@Data
@Builder
public class WebhookInfo {
    private Long id;
    private String url;
    private boolean active;
    private String secretToken;
    private String events; // "push", "merge_request", etc.
}
```

- [ ] **Step 10.2: 创建 ServiceInfo.java**

```java
package com.shulex.forge.pipeline.adapter.model;

import lombok.Builder;
import lombok.Data;
import java.util.Map;

@Data
@Builder
public class ServiceInfo {
    private String namespace;
    private String name;
    private String type; // "ClusterIP", "NodePort", "LoadBalancer"
    private Map<String, String> selector;
    private Integer port;
    private Integer targetPort;
    private String clusterIp;
}
```

- [ ] **Step 10.3: 创建 PipelineCreateRequest.java**

```java
package com.shulex.forge.pipeline.adapter.model;

import lombok.Builder;
import lombok.Data;

@Data
@Builder
public class PipelineCreateRequest {
    private String name;
    private String repoUrl;
    private String branch;
    private String yamlContent; // 流水线 YAML 定义
}
```

- [ ] **Step 11: 创建 CodeHostingAdapter.java**

```java
package com.shulex.forge.pipeline.adapter.spi;

import com.shulex.forge.pipeline.adapter.model.*;
import java.util.List;

/**
 * 代码托管适配器接口 — 面向能力抽象，不面向厂商 API。
 * 首期实现：Codeup。后续扩展：GitHub, Gitee, GitLab。
 */
public interface CodeHostingAdapter {

    /** 适配器标识 */
    String getType();

    // === 仓库结构读取 ===

    /** 获取文件树 */
    List<FileTreeNode> listRepositoryTree(String repoId, String path, String ref);

    /** 获取文件内容 */
    FileContent getFileContent(String repoId, String filePath, String ref);

    // === 代码提交 ===

    /** 原子提交多文件（核心 API） */
    String createCommitWithMultipleFiles(String repoId, String branch,
                                          String commitMessage, List<CommitFile> files);

    // === 分支管理 ===

    /** 创建分支 */
    BranchInfo createBranch(String repoId, String branchName, String ref);

    /** 删除分支 */
    void deleteBranch(String repoId, String branchName);

    /** 获取分支信息 */
    BranchInfo getBranch(String repoId, String branchName);

    /** 列出所有分支 */
    List<BranchInfo> listBranches(String repoId);

    // === 合并请求管理 ===

    /** 创建合并请求 */
    MergeRequestInfo createMergeRequest(String repoId, MergeRequestCreateRequest request);

    /** 获取合并请求详情 */
    MergeRequestInfo getMergeRequest(String repoId, Long mrId);

    /** 合并 MR */
    void mergeMergeRequest(String repoId, Long mrId);

    /** 关闭 MR */
    void closeMergeRequest(String repoId, Long mrId);

    /** 添加 MR 评论 */
    void addMergeRequestComment(String repoId, Long mrId, String comment);

    // === Webhook 管理 ===

    /** 创建 Webhook */
    WebhookInfo createWebhook(String repoId, String url, String secretToken, String events);

    /** 列出 Webhook */
    List<WebhookInfo> listWebhooks(String repoId);

    /** 删除 Webhook */
    void deleteWebhook(String repoId, Long webhookId);
}
```

- [ ] **Step 12: 创建 ContainerOrchestrationAdapter.java**

```java
package com.shulex.forge.pipeline.adapter.spi;

import com.shulex.forge.pipeline.adapter.model.DeploymentInfo;
import com.shulex.forge.pipeline.adapter.model.PodInfo;
import java.util.List;
import java.util.Map;

/**
 * 容器编排适配器接口。
 * 首期实现：ACK (K8s Standard API)。后续扩展：原生 K8s, TKE, CCE。
 */
public interface ContainerOrchestrationAdapter {

    String getType();

    // === Namespace 管理 ===

    void createNamespace(String namespace, Map<String, String> labels);

    void deleteNamespace(String namespace);

    boolean namespaceExists(String namespace);

    // === Deployment 管理 ===

    DeploymentInfo createOrUpdateDeployment(String namespace, String name, String image,
                                             Integer replicas, Map<String, String> env);

    DeploymentInfo getDeployment(String namespace, String name);

    void deleteDeployment(String namespace, String name);

    void scaleDeployment(String namespace, String name, int replicas);

    // === Pod 查询 ===

    List<PodInfo> listPods(String namespace, Map<String, String> labelSelector);

    String getPodLogs(String namespace, String podName, int tailLines);

    // === Service 管理 ===

    ServiceInfo createOrUpdateService(String namespace, String name, String type,
                                       Map<String, String> selector, int port, int targetPort);

    ServiceInfo getService(String namespace, String name);

    void deleteService(String namespace, String name);

    // === ConfigMap ===

    void createOrUpdateConfigMap(String namespace, String name, Map<String, String> data);

    void deleteConfigMap(String namespace, String name);
}
```

- [ ] **Step 13: 创建 CiCdAdapter.java**

```java
package com.shulex.forge.pipeline.adapter.spi;

import com.shulex.forge.pipeline.adapter.model.PipelineCreateRequest;
import com.shulex.forge.pipeline.adapter.model.PipelineInfo;
import com.shulex.forge.pipeline.adapter.model.PipelineRunInfo;
import java.util.List;

/**
 * CI/CD 适配器接口。
 * 首期实现：云效 Flow。后续扩展：GitHub Actions, Jenkins, GitLab CI。
 */
public interface CiCdAdapter {

    String getType();

    // === 流水线管理 ===

    /** 创建流水线 */
    PipelineInfo createPipeline(String orgId, PipelineCreateRequest request);

    List<PipelineInfo> listPipelines(String orgId);

    PipelineInfo getPipeline(String orgId, String pipelineId);

    // === 流水线触发 ===

    PipelineRunInfo triggerPipeline(String orgId, String pipelineId, String branch);

    // === 状态查询 ===

    PipelineRunInfo getPipelineRun(String orgId, String pipelineId, String runId);

    List<PipelineRunInfo> listPipelineRuns(String orgId, String pipelineId, int limit);

    // === 日志获取 ===

    String getPipelineRunLogs(String orgId, String pipelineId, String runId);
}
```

- [ ] **Step 14: 创建 AdapterRegistry.java**

```java
package com.shulex.forge.pipeline.adapter.spi;

import lombok.extern.slf4j.Slf4j;
import org.springframework.stereotype.Component;

import java.util.List;
import java.util.Map;
import java.util.concurrent.ConcurrentHashMap;

@Slf4j
@Component
public class AdapterRegistry {

    private final Map<String, CodeHostingAdapter> codeHostingAdapters = new ConcurrentHashMap<>();
    private final Map<String, ContainerOrchestrationAdapter> containerAdapters = new ConcurrentHashMap<>();
    private final Map<String, CiCdAdapter> ciCdAdapters = new ConcurrentHashMap<>();

    public AdapterRegistry(List<CodeHostingAdapter> codeHostingList,
                           List<ContainerOrchestrationAdapter> containerList,
                           List<CiCdAdapter> ciCdList) {
        codeHostingList.forEach(a -> {
            codeHostingAdapters.put(a.getType(), a);
            log.info("注册代码托管适配器: {}", a.getType());
        });
        containerList.forEach(a -> {
            containerAdapters.put(a.getType(), a);
            log.info("注册容器编排适配器: {}", a.getType());
        });
        ciCdList.forEach(a -> {
            ciCdAdapters.put(a.getType(), a);
            log.info("注册 CI/CD 适配器: {}", a.getType());
        });
    }

    public CodeHostingAdapter getCodeHostingAdapter(String type) {
        CodeHostingAdapter adapter = codeHostingAdapters.get(type);
        if (adapter == null) {
            throw new IllegalArgumentException("未找到代码托管适配器: " + type);
        }
        return adapter;
    }

    public ContainerOrchestrationAdapter getContainerAdapter(String type) {
        ContainerOrchestrationAdapter adapter = containerAdapters.get(type);
        if (adapter == null) {
            throw new IllegalArgumentException("未找到容器编排适配器: " + type);
        }
        return adapter;
    }

    public CiCdAdapter getCiCdAdapter(String type) {
        CiCdAdapter adapter = ciCdAdapters.get(type);
        if (adapter == null) {
            throw new IllegalArgumentException("未找到 CI/CD 适配器: " + type);
        }
        return adapter;
    }

    public Map<String, List<String>> getRegisteredAdapterTypes() {
        return Map.of(
                "codeHosting", List.copyOf(codeHostingAdapters.keySet()),
                "containerOrchestration", List.copyOf(containerAdapters.keySet()),
                "ciCd", List.copyOf(ciCdAdapters.keySet())
        );
    }
}
```

- [ ] **Step 15: 编译验证**

Run: `cd forge-pipeline && mvn clean compile -q`
Expected: 编译通过

- [ ] **Step 16: Commit**

```bash
git add forge-pipeline/src/main/java/com/shulex/forge/pipeline/adapter/
git commit -m "feat(m2): add adapter SPI interfaces and common models"
```

---

### Task 3: HTTP 基础设施 + 凭证管理

**Files:**
- Create: `forge-pipeline/src/main/java/com/shulex/forge/pipeline/infrastructure/http/RetryableHttpClient.java`
- Create: `forge-pipeline/src/main/java/com/shulex/forge/pipeline/infrastructure/credential/CredentialService.java`
- Create: `forge-pipeline/src/test/resources/application-test.yml`
- Create: `forge-pipeline/src/test/java/com/shulex/forge/pipeline/infrastructure/http/RetryableHttpClientTest.java`

- [ ] **Step 1: 创建测试配置 application-test.yml**

```yaml
spring:
  datasource:
    url: jdbc:h2:mem:forge_pipeline_test;MODE=MYSQL;DB_CLOSE_DELAY=-1
    driver-class-name: org.h2.Driver
    username: sa
    password:
  flyway:
    enabled: false
  data:
    redis:
      host: localhost
      port: 6379

# 适配器测试配置
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
```

- [ ] **Step 2: 写 RetryableHttpClient 失败测试**

```java
package com.shulex.forge.pipeline.infrastructure.http;

import okhttp3.mockwebserver.MockResponse;
import okhttp3.mockwebserver.MockWebServer;
import org.junit.jupiter.api.AfterEach;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;

import java.io.IOException;

import static org.assertj.core.api.Assertions.assertThat;
import static org.assertj.core.api.Assertions.assertThatThrownBy;

class RetryableHttpClientTest {

    private MockWebServer mockServer;
    private RetryableHttpClient client;

    @BeforeEach
    void setUp() throws IOException {
        mockServer = new MockWebServer();
        mockServer.start();
        client = new RetryableHttpClient(3, 100); // maxAttempts=3, retryDelayMs=100
    }

    @AfterEach
    void tearDown() throws IOException {
        mockServer.shutdown();
    }

    @Test
    void get_returnsBody() throws Exception {
        mockServer.enqueue(new MockResponse().setBody("{\"result\":\"ok\"}").setResponseCode(200));

        String result = client.get(mockServer.url("/test").toString(), null);
        assertThat(result).contains("ok");
    }

    @Test
    void get_retriesOnServerError() throws Exception {
        mockServer.enqueue(new MockResponse().setResponseCode(503));
        mockServer.enqueue(new MockResponse().setResponseCode(503));
        mockServer.enqueue(new MockResponse().setBody("{\"ok\":true}").setResponseCode(200));

        String result = client.get(mockServer.url("/retry").toString(), null);
        assertThat(result).contains("ok");
        assertThat(mockServer.getRequestCount()).isEqualTo(3);
    }

    @Test
    void get_throwsAfterMaxRetries() {
        mockServer.enqueue(new MockResponse().setResponseCode(503));
        mockServer.enqueue(new MockResponse().setResponseCode(503));
        mockServer.enqueue(new MockResponse().setResponseCode(503));

        assertThatThrownBy(() -> client.get(mockServer.url("/fail").toString(), null))
                .isInstanceOf(RuntimeException.class);
    }

    @Test
    void post_sendsBodyAndReturnsResult() throws Exception {
        mockServer.enqueue(new MockResponse().setBody("{\"id\":1}").setResponseCode(200));

        String result = client.post(mockServer.url("/create").toString(),
                "{\"name\":\"test\"}", null);
        assertThat(result).contains("id");
    }
}
```

- [ ] **Step 3: 运行测试确认失败**

Run: `cd forge-pipeline && mvn test -Dtest=RetryableHttpClientTest -pl . 2>&1 | tail -20`
Expected: 编译失败 — RetryableHttpClient 不存在

- [ ] **Step 4: 实现 RetryableHttpClient**

```java
package com.shulex.forge.pipeline.infrastructure.http;

import lombok.extern.slf4j.Slf4j;
import okhttp3.*;

import java.io.IOException;
import java.util.Map;
import java.util.concurrent.TimeUnit;

@Slf4j
public class RetryableHttpClient {

    private final OkHttpClient httpClient;
    private final int maxAttempts;
    private final long retryDelayMs;
    private final io.github.resilience4j.ratelimiter.RateLimiter rateLimiter;

    public RetryableHttpClient(int maxAttempts, long retryDelayMs) {
        this.maxAttempts = maxAttempts;
        this.retryDelayMs = retryDelayMs;
        this.rateLimiter = io.github.resilience4j.ratelimiter.RateLimiter.of("http-client",
                io.github.resilience4j.ratelimiter.RateLimiterConfig.custom()
                        .limitForPeriod(50)
                        .limitRefreshPeriod(java.time.Duration.ofSeconds(1))
                        .timeoutDuration(java.time.Duration.ofSeconds(5))
                        .build());
        this.httpClient = new OkHttpClient.Builder()
                .connectTimeout(10, TimeUnit.SECONDS)
                .readTimeout(30, TimeUnit.SECONDS)
                .writeTimeout(30, TimeUnit.SECONDS)
                .build();
    }

    public String get(String url, Map<String, String> headers) {
        Request.Builder builder = new Request.Builder().url(url);
        if (headers != null) {
            headers.forEach(builder::addHeader);
        }
        return executeWithRetry(builder.build());
    }

    public String post(String url, String jsonBody, Map<String, String> headers) {
        RequestBody body = RequestBody.create(jsonBody,
                MediaType.parse("application/json; charset=utf-8"));
        Request.Builder builder = new Request.Builder().url(url).post(body);
        if (headers != null) {
            headers.forEach(builder::addHeader);
        }
        return executeWithRetry(builder.build());
    }

    public String put(String url, String jsonBody, Map<String, String> headers) {
        RequestBody body = RequestBody.create(jsonBody,
                MediaType.parse("application/json; charset=utf-8"));
        Request.Builder builder = new Request.Builder().url(url).put(body);
        if (headers != null) {
            headers.forEach(builder::addHeader);
        }
        return executeWithRetry(builder.build());
    }

    public void delete(String url, Map<String, String> headers) {
        Request.Builder builder = new Request.Builder().url(url).delete();
        if (headers != null) {
            headers.forEach(builder::addHeader);
        }
        executeWithRetry(builder.build());
    }

    private String executeWithRetry(Request request) {
        int attempt = 0;
        while (true) {
            attempt++;
            rateLimiter.acquirePermission();
            try (Response response = httpClient.newCall(request).execute()) {
                if (response.isSuccessful()) {
                    ResponseBody responseBody = response.body();
                    return responseBody != null ? responseBody.string() : "";
                }
                if (response.code() >= 500 && attempt < maxAttempts) {
                    log.warn("请求失败 ({}), 第 {}/{} 次尝试: {} {}",
                            response.code(), attempt, maxAttempts, request.method(), request.url());
                    sleep(retryDelayMs * attempt);
                    continue;
                }
                throw new RuntimeException("HTTP 请求失败: " + response.code()
                        + " " + request.method() + " " + request.url());
            } catch (IOException e) {
                if (attempt >= maxAttempts) {
                    throw new RuntimeException("HTTP 请求异常: " + request.url(), e);
                }
                log.warn("请求 IO 异常, 第 {}/{} 次尝试: {}", attempt, maxAttempts, e.getMessage());
                sleep(retryDelayMs * attempt);
            }
        }
    }

    private void sleep(long ms) {
        try {
            Thread.sleep(ms);
        } catch (InterruptedException e) {
            Thread.currentThread().interrupt();
        }
    }
}
```

- [ ] **Step 5: 运行测试确认通过**

Run: `cd forge-pipeline && mvn test -Dtest=RetryableHttpClientTest -pl . 2>&1 | tail -15`
Expected: 全部 PASS

- [ ] **Step 6: 创建 CredentialService.java**

```java
package com.shulex.forge.pipeline.infrastructure.credential;

import lombok.extern.slf4j.Slf4j;
import org.springframework.core.env.Environment;
import org.springframework.stereotype.Service;

/**
 * 凭证管理服务 — 从 Spring Environment（Nacos 配置）读取加密凭证。
 * 首期使用 Spring Environment 属性读取，后续可扩展为 Nacos 加密配置。
 */
@Slf4j
@Service
public class CredentialService {

    private final Environment environment;

    public CredentialService(Environment environment) {
        this.environment = environment;
    }

    /**
     * 获取凭证值。
     * @param key 凭证键，如 "forge.adapter.codeup.access-token"
     * @return 凭证值
     */
    public String getCredential(String key) {
        String value = environment.getProperty(key);
        if (value == null || value.isBlank()) {
            log.error("凭证未配置: {}", key);
            throw new IllegalStateException("凭证未配置: " + key);
        }
        return value;
    }

    /**
     * 获取凭证值，不存在时返回默认值。
     */
    public String getCredential(String key, String defaultValue) {
        return environment.getProperty(key, defaultValue);
    }
}
```

- [ ] **Step 7: 创建 CredentialServiceTest.java**

```java
package com.shulex.forge.pipeline.infrastructure.credential;

import org.junit.jupiter.api.Test;
import org.springframework.core.env.StandardEnvironment;

import static org.assertj.core.api.Assertions.assertThat;
import static org.assertj.core.api.Assertions.assertThatThrownBy;

class CredentialServiceTest {

    @Test
    void getCredential_throwsWhenMissing() {
        CredentialService service = new CredentialService(new StandardEnvironment());
        assertThatThrownBy(() -> service.getCredential("nonexistent.key"))
                .isInstanceOf(IllegalStateException.class)
                .hasMessageContaining("凭证未配置");
    }

    @Test
    void getCredential_returnsDefaultWhenMissing() {
        CredentialService service = new CredentialService(new StandardEnvironment());
        String result = service.getCredential("nonexistent.key", "default-val");
        assertThat(result).isEqualTo("default-val");
    }
}
```

- [ ] **Step 8: 创建 AdapterCacheService.java**

```java
package com.shulex.forge.pipeline.infrastructure.cache;

import lombok.extern.slf4j.Slf4j;
import org.springframework.data.redis.core.StringRedisTemplate;
import org.springframework.stereotype.Service;

import java.time.Duration;
import java.util.Optional;

/**
 * 适配器缓存服务 — 缓存外部 API 响应以减少调用频率。
 * 使用 Redis 存储，支持 TTL 过期。
 */
@Slf4j
@Service
public class AdapterCacheService {

    private final StringRedisTemplate redisTemplate;
    private static final String KEY_PREFIX = "forge:adapter:cache:";

    public AdapterCacheService(StringRedisTemplate redisTemplate) {
        this.redisTemplate = redisTemplate;
    }

    public Optional<String> get(String key) {
        String value = redisTemplate.opsForValue().get(KEY_PREFIX + key);
        if (value != null) {
            log.debug("缓存命中: {}", key);
        }
        return Optional.ofNullable(value);
    }

    public void put(String key, String value, Duration ttl) {
        redisTemplate.opsForValue().set(KEY_PREFIX + key, value, ttl);
        log.debug("缓存写入: {}, TTL: {}", key, ttl);
    }

    public void evict(String key) {
        redisTemplate.delete(KEY_PREFIX + key);
    }

    public void evictByPrefix(String prefix) {
        var keys = redisTemplate.keys(KEY_PREFIX + prefix + "*");
        if (keys != null && !keys.isEmpty()) {
            redisTemplate.delete(keys);
            log.debug("缓存批量清除: {} 个 key", keys.size());
        }
    }
}
```

- [ ] **Step 9: 运行测试确认通过**

Run: `cd forge-pipeline && mvn test -Dtest=RetryableHttpClientTest,CredentialServiceTest -pl . 2>&1 | tail -15`
Expected: 全部 PASS

- [ ] **Step 10: 编译验证**

Run: `cd forge-pipeline && mvn clean compile -q`
Expected: 编译通过

- [ ] **Step 11: Commit**

```bash
git add forge-pipeline/src/
git commit -m "feat(m2): add retryable HTTP client, credential service, and cache service"
```

---

### Task 4: Codeup 代码托管适配器实现（TDD）

**Files:**
- Create: `forge-pipeline/src/main/java/com/shulex/forge/pipeline/adapter/codeup/CodeupConfig.java`
- Create: `forge-pipeline/src/main/java/com/shulex/forge/pipeline/adapter/codeup/CodeupClient.java`
- Create: `forge-pipeline/src/test/java/com/shulex/forge/pipeline/adapter/codeup/CodeupClientTest.java`
- Create: `forge-pipeline/src/main/java/com/shulex/forge/pipeline/adapter/codeup/CodeupAdapter.java`
- Create: `forge-pipeline/src/test/java/com/shulex/forge/pipeline/adapter/codeup/CodeupAdapterTest.java`
- Create: `forge-pipeline/src/test/resources/mock/codeup-tree-response.json`
- Create: `forge-pipeline/src/test/resources/mock/codeup-file-response.json`
- Create: `forge-pipeline/src/test/resources/mock/codeup-commit-response.json`

- [ ] **Step 1: 创建 Mock JSON 响应文件**

`codeup-tree-response.json`:
```json
{
  "result": [
    {"path": "src/main/java", "name": "java", "type": "tree"},
    {"path": "pom.xml", "name": "pom.xml", "type": "blob", "size": 1024}
  ]
}
```

`codeup-file-response.json`:
```json
{
  "result": {
    "filePath": "pom.xml",
    "content": "PD94bWwgdmVyc2lvbj0iMS4wIj8+",
    "encoding": "base64",
    "blobId": "abc123"
  }
}
```

`codeup-commit-response.json`:
```json
{
  "result": {
    "commitId": "abc123def456",
    "message": "feat: auto-generated code"
  }
}
```

- [ ] **Step 2: 创建 CodeupConfig.java**

```java
package com.shulex.forge.pipeline.adapter.codeup;

import lombok.Data;
import org.springframework.boot.context.properties.ConfigurationProperties;
import org.springframework.stereotype.Component;

@Data
@Component
@ConfigurationProperties(prefix = "forge.adapter.codeup")
public class CodeupConfig {
    private String baseUrl = "https://codeup.aliyun.com";
    private String orgId;
    private String accessToken;
}
```

- [ ] **Step 3: 写 CodeupClient 失败测试**

```java
package com.shulex.forge.pipeline.adapter.codeup;

import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.ObjectMapper;
import okhttp3.mockwebserver.MockResponse;
import okhttp3.mockwebserver.MockWebServer;
import org.junit.jupiter.api.AfterEach;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;
import org.springframework.core.io.ClassPathResource;

import java.io.IOException;
import java.nio.charset.StandardCharsets;

import static org.assertj.core.api.Assertions.assertThat;

class CodeupClientTest {

    private MockWebServer mockServer;
    private CodeupClient client;
    private final ObjectMapper objectMapper = new ObjectMapper();

    @BeforeEach
    void setUp() throws IOException {
        mockServer = new MockWebServer();
        mockServer.start();
        CodeupConfig config = new CodeupConfig();
        config.setBaseUrl(mockServer.url("/").toString());
        config.setOrgId("test-org");
        config.setAccessToken("test-token");
        client = new CodeupClient(config);
    }

    @AfterEach
    void tearDown() throws IOException {
        mockServer.shutdown();
    }

    @Test
    void listRepositoryTree_parsesResponse() throws Exception {
        String body = new ClassPathResource("mock/codeup-tree-response.json")
                .getContentAsString(StandardCharsets.UTF_8);
        mockServer.enqueue(new MockResponse().setBody(body).setResponseCode(200));

        JsonNode result = client.listRepositoryTree("repo-1", "/", "main");
        assertThat(result.isArray()).isTrue();
        assertThat(result.size()).isEqualTo(2);
    }

    @Test
    void getFileBlobs_parsesResponse() throws Exception {
        String body = new ClassPathResource("mock/codeup-file-response.json")
                .getContentAsString(StandardCharsets.UTF_8);
        mockServer.enqueue(new MockResponse().setBody(body).setResponseCode(200));

        JsonNode result = client.getFileBlobs("repo-1", "pom.xml", "main");
        assertThat(result.get("filePath").asText()).isEqualTo("pom.xml");
    }

    @Test
    void createCommitWithMultipleFiles_returnsCommitId() throws Exception {
        String body = new ClassPathResource("mock/codeup-commit-response.json")
                .getContentAsString(StandardCharsets.UTF_8);
        mockServer.enqueue(new MockResponse().setBody(body).setResponseCode(200));

        JsonNode result = client.createCommit("repo-1", "main", "test commit", "[]");
        assertThat(result.get("commitId").asText()).isEqualTo("abc123def456");
    }

    @Test
    void request_sendsAccessTokenHeader() throws Exception {
        mockServer.enqueue(new MockResponse().setBody("{\"result\":[]}").setResponseCode(200));

        client.listRepositoryTree("repo-1", "/", "main");

        var request = mockServer.takeRequest();
        assertThat(request.getHeader("Private-Token")).isEqualTo("test-token");
    }
}
```

- [ ] **Step 4: 运行测试确认失败**

Run: `cd forge-pipeline && mvn test -Dtest=CodeupClientTest -pl . 2>&1 | tail -20`
Expected: 编译失败 — CodeupClient 不存在

- [ ] **Step 5: 实现 CodeupClient**

```java
package com.shulex.forge.pipeline.adapter.codeup;

import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.ObjectMapper;
import com.shulex.forge.pipeline.infrastructure.http.RetryableHttpClient;
import lombok.extern.slf4j.Slf4j;

import java.util.Map;

/**
 * Codeup HTTP 客户端 — 封装所有 Codeup REST API 调用。
 */
@Slf4j
public class CodeupClient {

    private final CodeupConfig config;
    private final RetryableHttpClient httpClient;
    private final ObjectMapper objectMapper;

    public CodeupClient(CodeupConfig config) {
        this.config = config;
        this.httpClient = new RetryableHttpClient(3, 500);
        this.objectMapper = new ObjectMapper();
    }

    private Map<String, String> headers() {
        return Map.of("Private-Token", config.getAccessToken());
    }

    private String baseUrl() {
        String url = config.getBaseUrl();
        return url.endsWith("/") ? url.substring(0, url.length() - 1) : url;
    }

    public JsonNode listRepositoryTree(String repoId, String path, String ref) {
        String url = String.format("%s/api/v4/projects/%s/%s/repository/tree?path=%s&ref=%s",
                baseUrl(), config.getOrgId(), repoId, path, ref);
        String body = httpClient.get(url, headers());
        return parseResult(body);
    }

    public JsonNode getFileBlobs(String repoId, String filePath, String ref) {
        String url = String.format("%s/api/v4/projects/%s/%s/repository/blobs?filePath=%s&ref=%s",
                baseUrl(), config.getOrgId(), repoId, filePath, ref);
        String body = httpClient.get(url, headers());
        return parseResult(body);
    }

    public JsonNode createCommit(String repoId, String branch, String message, String actionsJson) {
        String url = String.format("%s/api/v4/projects/%s/%s/repository/commits",
                baseUrl(), config.getOrgId(), repoId);
        try {
            var payload = objectMapper.createObjectNode()
                    .put("branch", branch)
                    .put("commitMessage", message)
                    .set("actions", objectMapper.readTree(actionsJson));
            String body = httpClient.post(url, objectMapper.writeValueAsString(payload), headers());
            return parseResult(body);
        } catch (Exception e) {
            throw new RuntimeException("构建提交请求失败", e);
        }
    }

    public JsonNode createBranch(String repoId, String branchName, String ref) {
        String url = String.format("%s/api/v4/projects/%s/%s/repository/branches",
                baseUrl(), config.getOrgId(), repoId);
        try {
            var payload = objectMapper.createObjectNode()
                    .put("branchName", branchName)
                    .put("ref", ref);
            String body = httpClient.post(url, objectMapper.writeValueAsString(payload), headers());
            return parseResult(body);
        } catch (Exception e) {
            throw new RuntimeException("构建分支请求失败", e);
        }
    }

    public void deleteBranch(String repoId, String branchName) {
        String url = String.format("%s/api/v4/projects/%s/%s/repository/branches?branchName=%s",
                baseUrl(), config.getOrgId(), repoId, branchName);
        httpClient.delete(url, headers());
    }

    public JsonNode getBranch(String repoId, String branchName) {
        String url = String.format("%s/api/v4/projects/%s/%s/repository/branches/%s",
                baseUrl(), config.getOrgId(), repoId, branchName);
        String body = httpClient.get(url, headers());
        return parseResult(body);
    }

    public JsonNode createMergeRequest(String repoId, String title, String description,
                                        String sourceBranch, String targetBranch) {
        String url = String.format("%s/api/v4/projects/%s/%s/merge_requests",
                baseUrl(), config.getOrgId(), repoId);
        try {
            var payload = objectMapper.createObjectNode()
                    .put("title", title)
                    .put("description", description)
                    .put("sourceBranch", sourceBranch)
                    .put("targetBranch", targetBranch);
            String body = httpClient.post(url, objectMapper.writeValueAsString(payload), headers());
            return parseResult(body);
        } catch (Exception e) {
            throw new RuntimeException("构建 MR 请求失败", e);
        }
    }

    public JsonNode getMergeRequest(String repoId, Long mrId) {
        String url = String.format("%s/api/v4/projects/%s/%s/merge_requests/%d",
                baseUrl(), config.getOrgId(), repoId, mrId);
        String body = httpClient.get(url, headers());
        return parseResult(body);
    }

    public void mergeMergeRequest(String repoId, Long mrId) {
        String url = String.format("%s/api/v4/projects/%s/%s/merge_requests/%d/merge",
                baseUrl(), config.getOrgId(), repoId, mrId);
        httpClient.put(url, "{}", headers());
    }

    public void closeMergeRequest(String repoId, Long mrId) {
        String url = String.format("%s/api/v4/projects/%s/%s/merge_requests/%d",
                baseUrl(), config.getOrgId(), repoId, mrId);
        try {
            var payload = objectMapper.createObjectNode().put("state", "closed");
            httpClient.put(url, objectMapper.writeValueAsString(payload), headers());
        } catch (Exception e) {
            throw new RuntimeException("关闭 MR 失败", e);
        }
    }

    public void addComment(String repoId, Long mrId, String comment) {
        String url = String.format("%s/api/v4/projects/%s/%s/merge_requests/%d/comments",
                baseUrl(), config.getOrgId(), repoId, mrId);
        try {
            var payload = objectMapper.createObjectNode().put("content", comment);
            httpClient.post(url, objectMapper.writeValueAsString(payload), headers());
        } catch (Exception e) {
            throw new RuntimeException("添加 MR 评论失败", e);
        }
    }

    public JsonNode createWebhook(String repoId, String url, String secretToken, String events) {
        String reqUrl = String.format("%s/api/v4/projects/%s/%s/webhooks",
                baseUrl(), config.getOrgId(), repoId);
        try {
            var payload = objectMapper.createObjectNode()
                    .put("url", url)
                    .put("secretToken", secretToken)
                    .put("events", events);
            String body = httpClient.post(reqUrl, objectMapper.writeValueAsString(payload), headers());
            return parseResult(body);
        } catch (Exception e) {
            throw new RuntimeException("创建 Webhook 失败", e);
        }
    }

    public JsonNode listWebhooks(String repoId) {
        String url = String.format("%s/api/v4/projects/%s/%s/webhooks",
                baseUrl(), config.getOrgId(), repoId);
        return parseResult(httpClient.get(url, headers()));
    }

    public void deleteWebhook(String repoId, Long webhookId) {
        String url = String.format("%s/api/v4/projects/%s/%s/webhooks/%d",
                baseUrl(), config.getOrgId(), repoId, webhookId);
        httpClient.delete(url, headers());
    }

    public JsonNode listBranches(String repoId) {
        String url = String.format("%s/api/v4/projects/%s/%s/repository/branches",
                baseUrl(), config.getOrgId(), repoId);
        return parseResult(httpClient.get(url, headers()));
    }

    private JsonNode parseResult(String body) {
        try {
            JsonNode root = objectMapper.readTree(body);
            return root.has("result") ? root.get("result") : root;
        } catch (Exception e) {
            throw new RuntimeException("JSON 解析失败: " + body, e);
        }
    }
}
```

- [ ] **Step 6: 运行测试确认通过**

Run: `cd forge-pipeline && mvn test -Dtest=CodeupClientTest -pl . 2>&1 | tail -15`
Expected: 全部 PASS

- [ ] **Step 7: 写 CodeupAdapter 失败测试**

```java
package com.shulex.forge.pipeline.adapter.codeup;

import com.shulex.forge.pipeline.adapter.model.*;
import okhttp3.mockwebserver.MockResponse;
import okhttp3.mockwebserver.MockWebServer;
import org.junit.jupiter.api.AfterEach;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;
import org.springframework.core.io.ClassPathResource;

import java.io.IOException;
import java.nio.charset.StandardCharsets;
import java.util.List;

import static org.assertj.core.api.Assertions.assertThat;

class CodeupAdapterTest {

    private MockWebServer mockServer;
    private CodeupAdapter adapter;

    @BeforeEach
    void setUp() throws IOException {
        mockServer = new MockWebServer();
        mockServer.start();
        CodeupConfig config = new CodeupConfig();
        config.setBaseUrl(mockServer.url("/").toString());
        config.setOrgId("test-org");
        config.setAccessToken("test-token");
        adapter = new CodeupAdapter(new CodeupClient(config));
    }

    @AfterEach
    void tearDown() throws IOException {
        mockServer.shutdown();
    }

    @Test
    void getType_returnsCcodeup() {
        assertThat(adapter.getType()).isEqualTo("codeup");
    }

    @Test
    void listRepositoryTree_convertsToModel() throws Exception {
        String body = new ClassPathResource("mock/codeup-tree-response.json")
                .getContentAsString(StandardCharsets.UTF_8);
        mockServer.enqueue(new MockResponse().setBody(body).setResponseCode(200));

        List<FileTreeNode> nodes = adapter.listRepositoryTree("repo-1", "/", "main");
        assertThat(nodes).hasSize(2);
        assertThat(nodes.get(0).getType()).isEqualTo("tree");
        assertThat(nodes.get(1).getName()).isEqualTo("pom.xml");
    }

    @Test
    void getFileContent_convertsToModel() throws Exception {
        String body = new ClassPathResource("mock/codeup-file-response.json")
                .getContentAsString(StandardCharsets.UTF_8);
        mockServer.enqueue(new MockResponse().setBody(body).setResponseCode(200));

        FileContent content = adapter.getFileContent("repo-1", "pom.xml", "main");
        assertThat(content.getPath()).isEqualTo("pom.xml");
    }

    @Test
    void createCommitWithMultipleFiles_returnsCommitId() throws Exception {
        String body = new ClassPathResource("mock/codeup-commit-response.json")
                .getContentAsString(StandardCharsets.UTF_8);
        mockServer.enqueue(new MockResponse().setBody(body).setResponseCode(200));

        List<CommitFile> files = List.of(
                CommitFile.builder().path("README.md").content("# Hello").action("create").build()
        );
        String commitId = adapter.createCommitWithMultipleFiles("repo-1", "main", "test", files);
        assertThat(commitId).isEqualTo("abc123def456");
    }
}
```

- [ ] **Step 8: 运行测试确认失败**

Run: `cd forge-pipeline && mvn test -Dtest=CodeupAdapterTest -pl . 2>&1 | tail -20`
Expected: 编译失败 — CodeupAdapter 不存在

- [ ] **Step 9: 实现 CodeupAdapter**

```java
package com.shulex.forge.pipeline.adapter.codeup;

import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.ObjectMapper;
import com.shulex.forge.pipeline.adapter.model.*;
import com.shulex.forge.pipeline.adapter.spi.CodeHostingAdapter;
import lombok.extern.slf4j.Slf4j;
import org.springframework.stereotype.Component;

import java.util.ArrayList;
import java.util.List;

@Slf4j
@Component
public class CodeupAdapter implements CodeHostingAdapter {

    private final CodeupClient client;
    private final ObjectMapper objectMapper;

    public CodeupAdapter(CodeupClient client) {
        this.client = client;
        this.objectMapper = new ObjectMapper();
    }

    @Override
    public String getType() {
        return "codeup";
    }

    @Override
    public List<FileTreeNode> listRepositoryTree(String repoId, String path, String ref) {
        JsonNode result = client.listRepositoryTree(repoId, path, ref);
        List<FileTreeNode> nodes = new ArrayList<>();
        for (JsonNode node : result) {
            nodes.add(FileTreeNode.builder()
                    .path(node.path("path").asText())
                    .name(node.path("name").asText())
                    .type(node.path("type").asText())
                    .size(node.has("size") ? node.get("size").asLong() : null)
                    .build());
        }
        return nodes;
    }

    @Override
    public FileContent getFileContent(String repoId, String filePath, String ref) {
        JsonNode result = client.getFileBlobs(repoId, filePath, ref);
        return FileContent.builder()
                .path(result.path("filePath").asText())
                .content(result.path("content").asText())
                .encoding(result.path("encoding").asText())
                .sha(result.path("blobId").asText())
                .build();
    }

    @Override
    public String createCommitWithMultipleFiles(String repoId, String branch,
                                                 String commitMessage, List<CommitFile> files) {
        try {
            String actionsJson = objectMapper.writeValueAsString(files);
            JsonNode result = client.createCommit(repoId, branch, commitMessage, actionsJson);
            return result.path("commitId").asText();
        } catch (Exception e) {
            throw new RuntimeException("提交文件失败", e);
        }
    }

    @Override
    public BranchInfo createBranch(String repoId, String branchName, String ref) {
        JsonNode result = client.createBranch(repoId, branchName, ref);
        return BranchInfo.builder()
                .name(result.path("name").asText())
                .commitId(result.path("commit").path("id").asText())
                .build();
    }

    @Override
    public void deleteBranch(String repoId, String branchName) {
        client.deleteBranch(repoId, branchName);
    }

    @Override
    public BranchInfo getBranch(String repoId, String branchName) {
        JsonNode result = client.getBranch(repoId, branchName);
        return BranchInfo.builder()
                .name(result.path("name").asText())
                .commitId(result.path("commit").path("id").asText())
                .isProtected(result.path("protected").asBoolean())
                .build();
    }

    @Override
    public List<BranchInfo> listBranches(String repoId) {
        JsonNode result = client.listBranches(repoId);
        List<BranchInfo> branches = new ArrayList<>();
        for (JsonNode node : result) {
            branches.add(BranchInfo.builder()
                    .name(node.path("name").asText())
                    .commitId(node.path("commit").path("id").asText())
                    .isProtected(node.path("protected").asBoolean())
                    .build());
        }
        return branches;
    }

    @Override
    public MergeRequestInfo createMergeRequest(String repoId, MergeRequestCreateRequest request) {
        JsonNode result = client.createMergeRequest(repoId,
                request.getTitle(), request.getDescription(),
                request.getSourceBranch(), request.getTargetBranch());
        return parseMergeRequestInfo(result);
    }

    @Override
    public MergeRequestInfo getMergeRequest(String repoId, Long mrId) {
        JsonNode result = client.getMergeRequest(repoId, mrId);
        return parseMergeRequestInfo(result);
    }

    @Override
    public void mergeMergeRequest(String repoId, Long mrId) {
        client.mergeMergeRequest(repoId, mrId);
    }

    @Override
    public void closeMergeRequest(String repoId, Long mrId) {
        client.closeMergeRequest(repoId, mrId);
    }

    @Override
    public void addMergeRequestComment(String repoId, Long mrId, String comment) {
        client.addComment(repoId, mrId, comment);
    }

    @Override
    public WebhookInfo createWebhook(String repoId, String url, String secretToken, String events) {
        JsonNode result = client.createWebhook(repoId, url, secretToken, events);
        return WebhookInfo.builder()
                .id(result.path("id").asLong())
                .url(result.path("url").asText())
                .active(result.path("active").asBoolean(true))
                .secretToken(secretToken)
                .events(events)
                .build();
    }

    @Override
    public List<WebhookInfo> listWebhooks(String repoId) {
        JsonNode result = client.listWebhooks(repoId);
        List<WebhookInfo> webhooks = new ArrayList<>();
        for (JsonNode node : result) {
            webhooks.add(WebhookInfo.builder()
                    .id(node.path("id").asLong())
                    .url(node.path("url").asText())
                    .active(node.path("active").asBoolean())
                    .build());
        }
        return webhooks;
    }

    @Override
    public void deleteWebhook(String repoId, Long webhookId) {
        client.deleteWebhook(repoId, webhookId);
    }

    private MergeRequestInfo parseMergeRequestInfo(JsonNode node) {
        return MergeRequestInfo.builder()
                .id(node.path("id").asLong())
                .title(node.path("title").asText())
                .description(node.path("description").asText())
                .sourceBranch(node.path("sourceBranch").asText())
                .targetBranch(node.path("targetBranch").asText())
                .state(node.path("state").asText())
                .url(node.path("webUrl").asText())
                .build();
    }
}
```

- [ ] **Step 10: 运行测试确认通过**

Run: `cd forge-pipeline && mvn test -Dtest=CodeupAdapterTest -pl . 2>&1 | tail -15`
Expected: 全部 PASS

- [ ] **Step 11: Commit**

```bash
git add forge-pipeline/src/
git commit -m "feat(m2): add Codeup code hosting adapter with tests"
```

---

### Task 5: ACK 容器编排适配器实现（TDD）

**Files:**
- Create: `forge-pipeline/src/main/java/com/shulex/forge/pipeline/adapter/ack/AckConfig.java`
- Create: `forge-pipeline/src/main/java/com/shulex/forge/pipeline/adapter/ack/K8sClientFactory.java`
- Create: `forge-pipeline/src/main/java/com/shulex/forge/pipeline/adapter/ack/AckAdapter.java`
- Create: `forge-pipeline/src/test/java/com/shulex/forge/pipeline/adapter/ack/AckAdapterTest.java`

- [ ] **Step 1: 创建 AckConfig.java**

```java
package com.shulex.forge.pipeline.adapter.ack;

import lombok.Data;
import org.springframework.boot.context.properties.ConfigurationProperties;
import org.springframework.stereotype.Component;

@Data
@Component
@ConfigurationProperties(prefix = "forge.adapter.ack")
public class AckConfig {
    private String kubeConfigPath;
}
```

- [ ] **Step 2: 创建 K8sClientFactory.java**

```java
package com.shulex.forge.pipeline.adapter.ack;

import io.kubernetes.client.openapi.ApiClient;
import io.kubernetes.client.openapi.apis.AppsV1Api;
import io.kubernetes.client.openapi.apis.CoreV1Api;
import io.kubernetes.client.util.Config;
import lombok.extern.slf4j.Slf4j;
import org.springframework.stereotype.Component;

import java.io.FileReader;
import java.io.IOException;

@Slf4j
@Component
public class K8sClientFactory {

    private final AckConfig ackConfig;

    public K8sClientFactory(AckConfig ackConfig) {
        this.ackConfig = ackConfig;
    }

    public ApiClient createClient() {
        try {
            String path = ackConfig.getKubeConfigPath();
            if (path != null && !path.isBlank()) {
                return Config.fromConfig(new FileReader(path));
            }
            // 尝试从默认位置或集群内部获取
            return Config.defaultClient();
        } catch (IOException e) {
            throw new RuntimeException("无法创建 K8s 客户端", e);
        }
    }

    public CoreV1Api coreV1Api() {
        return new CoreV1Api(createClient());
    }

    public AppsV1Api appsV1Api() {
        return new AppsV1Api(createClient());
    }
}
```

- [ ] **Step 3: 写 AckAdapter 失败测试**

注意：K8s client-java 的测试不适合用 MockWebServer，因为 K8s SDK 有自己的 HTTP 层。这里用单元测试 mock K8sClientFactory。

```java
package com.shulex.forge.pipeline.adapter.ack;

import com.shulex.forge.pipeline.adapter.model.DeploymentInfo;
import com.shulex.forge.pipeline.adapter.model.PodInfo;
import io.kubernetes.client.openapi.ApiException;
import io.kubernetes.client.openapi.apis.AppsV1Api;
import io.kubernetes.client.openapi.apis.CoreV1Api;
import io.kubernetes.client.openapi.models.*;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;
import org.mockito.Mockito;

import java.time.OffsetDateTime;
import java.util.List;
import java.util.Map;

import static org.assertj.core.api.Assertions.assertThat;
import static org.mockito.ArgumentMatchers.*;
import static org.mockito.Mockito.when;
import static org.mockito.Mockito.verify;

class AckAdapterTest {

    private CoreV1Api coreV1Api;
    private AppsV1Api appsV1Api;
    private AckAdapter adapter;

    @BeforeEach
    void setUp() {
        coreV1Api = Mockito.mock(CoreV1Api.class);
        appsV1Api = Mockito.mock(AppsV1Api.class);
        K8sClientFactory factory = Mockito.mock(K8sClientFactory.class);
        when(factory.coreV1Api()).thenReturn(coreV1Api);
        when(factory.appsV1Api()).thenReturn(appsV1Api);
        adapter = new AckAdapter(factory);
    }

    @Test
    void getType_returnsAck() {
        assertThat(adapter.getType()).isEqualTo("ack");
    }

    @Test
    void createNamespace_callsCoreApi() throws Exception {
        when(coreV1Api.createNamespace(any(), isNull(), isNull(), isNull(), isNull()))
                .thenReturn(new V1Namespace());
        adapter.createNamespace("test-ns", Map.of("env", "dev"));
        verify(coreV1Api).createNamespace(any(), isNull(), isNull(), isNull(), isNull());
    }

    @Test
    void namespaceExists_returnsTrueWhenFound() throws Exception {
        when(coreV1Api.readNamespace(eq("test-ns"), isNull()))
                .thenReturn(new V1Namespace());
        assertThat(adapter.namespaceExists("test-ns")).isTrue();
    }

    @Test
    void namespaceExists_returnsFalseOnNotFound() throws Exception {
        when(coreV1Api.readNamespace(eq("missing"), isNull()))
                .thenThrow(new ApiException(404, "not found"));
        assertThat(adapter.namespaceExists("missing")).isFalse();
    }

    @Test
    void getDeployment_convertsToModel() throws Exception {
        V1Deployment k8sDeploy = new V1Deployment()
                .metadata(new V1ObjectMeta().name("my-app").namespace("dev"))
                .spec(new V1DeploymentSpec().replicas(3))
                .status(new V1DeploymentStatus().availableReplicas(3));
        when(appsV1Api.readNamespacedDeployment(eq("my-app"), eq("dev"), isNull()))
                .thenReturn(k8sDeploy);

        DeploymentInfo info = adapter.getDeployment("dev", "my-app");
        assertThat(info.getName()).isEqualTo("my-app");
        assertThat(info.getReplicas()).isEqualTo(3);
        assertThat(info.getAvailableReplicas()).isEqualTo(3);
    }

    @Test
    void listPods_convertsToModel() throws Exception {
        V1PodList podList = new V1PodList().items(List.of(
                new V1Pod()
                        .metadata(new V1ObjectMeta().name("my-app-abc123").namespace("dev"))
                        .status(new V1PodStatus().phase("Running").startTime(OffsetDateTime.now()))
                        .spec(new V1PodSpec().nodeName("node-1"))
        ));
        when(coreV1Api.listNamespacedPod(eq("dev"), isNull(), isNull(), isNull(),
                isNull(), eq("app=my-app"), isNull(), isNull(), isNull(), isNull(), isNull(), isNull()))
                .thenReturn(podList);

        List<PodInfo> pods = adapter.listPods("dev", Map.of("app", "my-app"));
        assertThat(pods).hasSize(1);
        assertThat(pods.get(0).getPhase()).isEqualTo("Running");
    }
}
```

- [ ] **Step 4: 运行测试确认失败**

Run: `cd forge-pipeline && mvn test -Dtest=AckAdapterTest -pl . 2>&1 | tail -20`
Expected: 编译失败 — AckAdapter 不存在

- [ ] **Step 5: 实现 AckAdapter**

```java
package com.shulex.forge.pipeline.adapter.ack;

import com.shulex.forge.pipeline.adapter.model.DeploymentInfo;
import com.shulex.forge.pipeline.adapter.model.PodInfo;
import com.shulex.forge.pipeline.adapter.model.ServiceInfo;
import com.shulex.forge.pipeline.adapter.spi.ContainerOrchestrationAdapter;
import io.kubernetes.client.openapi.ApiException;
import io.kubernetes.client.openapi.apis.AppsV1Api;
import io.kubernetes.client.openapi.apis.CoreV1Api;
import io.kubernetes.client.openapi.models.*;
import lombok.extern.slf4j.Slf4j;
import org.springframework.stereotype.Component;

import java.util.List;
import java.util.Map;
import java.util.stream.Collectors;

@Slf4j
@Component
public class AckAdapter implements ContainerOrchestrationAdapter {

    private final K8sClientFactory clientFactory;

    public AckAdapter(K8sClientFactory clientFactory) {
        this.clientFactory = clientFactory;
    }

    @Override
    public String getType() {
        return "ack";
    }

    @Override
    public void createNamespace(String namespace, Map<String, String> labels) {
        try {
            V1Namespace ns = new V1Namespace()
                    .metadata(new V1ObjectMeta().name(namespace).labels(labels));
            clientFactory.coreV1Api().createNamespace(ns, null, null, null, null);
            log.info("创建 Namespace: {}", namespace);
        } catch (ApiException e) {
            throw new RuntimeException("创建 Namespace 失败: " + namespace, e);
        }
    }

    @Override
    public void deleteNamespace(String namespace) {
        try {
            clientFactory.coreV1Api().deleteNamespace(namespace, null, null, null, null, null, null);
            log.info("删除 Namespace: {}", namespace);
        } catch (ApiException e) {
            throw new RuntimeException("删除 Namespace 失败: " + namespace, e);
        }
    }

    @Override
    public boolean namespaceExists(String namespace) {
        try {
            clientFactory.coreV1Api().readNamespace(namespace, null);
            return true;
        } catch (ApiException e) {
            if (e.getCode() == 404) return false;
            throw new RuntimeException("查询 Namespace 失败: " + namespace, e);
        }
    }

    @Override
    public DeploymentInfo createOrUpdateDeployment(String namespace, String name, String image,
                                                    Integer replicas, Map<String, String> env) {
        try {
            V1Container container = new V1Container()
                    .name(name)
                    .image(image);
            if (env != null) {
                env.forEach((k, v) -> container.addEnvItem(new V1EnvVar().name(k).value(v)));
            }

            V1Deployment deployment = new V1Deployment()
                    .metadata(new V1ObjectMeta().name(name).namespace(namespace))
                    .spec(new V1DeploymentSpec()
                            .replicas(replicas)
                            .selector(new V1LabelSelector().matchLabels(Map.of("app", name)))
                            .template(new V1PodTemplateSpec()
                                    .metadata(new V1ObjectMeta().labels(Map.of("app", name)))
                                    .spec(new V1PodSpec().containers(List.of(container)))));

            V1Deployment result;
            try {
                clientFactory.appsV1Api().readNamespacedDeployment(name, namespace, null);
                result = clientFactory.appsV1Api().replaceNamespacedDeployment(
                        name, namespace, deployment, null, null, null, null);
                log.info("更新 Deployment: {}/{}", namespace, name);
            } catch (ApiException e) {
                if (e.getCode() == 404) {
                    result = clientFactory.appsV1Api().createNamespacedDeployment(
                            namespace, deployment, null, null, null, null);
                    log.info("创建 Deployment: {}/{}", namespace, name);
                } else {
                    throw e;
                }
            }
            return toDeploymentInfo(result);
        } catch (ApiException e) {
            throw new RuntimeException("创建/更新 Deployment 失败: " + namespace + "/" + name, e);
        }
    }

    @Override
    public DeploymentInfo getDeployment(String namespace, String name) {
        try {
            V1Deployment deployment = clientFactory.appsV1Api()
                    .readNamespacedDeployment(name, namespace, null);
            return toDeploymentInfo(deployment);
        } catch (ApiException e) {
            throw new RuntimeException("查询 Deployment 失败: " + namespace + "/" + name, e);
        }
    }

    @Override
    public void deleteDeployment(String namespace, String name) {
        try {
            clientFactory.appsV1Api().deleteNamespacedDeployment(
                    name, namespace, null, null, null, null, null, null);
            log.info("删除 Deployment: {}/{}", namespace, name);
        } catch (ApiException e) {
            throw new RuntimeException("删除 Deployment 失败: " + namespace + "/" + name, e);
        }
    }

    @Override
    public void scaleDeployment(String namespace, String name, int replicas) {
        try {
            V1Deployment existing = clientFactory.appsV1Api()
                    .readNamespacedDeployment(name, namespace, null);
            existing.getSpec().setReplicas(replicas);
            clientFactory.appsV1Api().replaceNamespacedDeployment(
                    name, namespace, existing, null, null, null, null);
            log.info("扩缩 Deployment: {}/{} → {} replicas", namespace, name, replicas);
        } catch (ApiException e) {
            throw new RuntimeException("扩缩 Deployment 失败", e);
        }
    }

    @Override
    public List<PodInfo> listPods(String namespace, Map<String, String> labelSelector) {
        try {
            String selector = labelSelector.entrySet().stream()
                    .map(e -> e.getKey() + "=" + e.getValue())
                    .collect(Collectors.joining(","));
            V1PodList podList = clientFactory.coreV1Api()
                    .listNamespacedPod(namespace, null, null, null, null, selector,
                            null, null, null, null, null, null);
            return podList.getItems().stream().map(this::toPodInfo).toList();
        } catch (ApiException e) {
            throw new RuntimeException("查询 Pod 列表失败", e);
        }
    }

    @Override
    public String getPodLogs(String namespace, String podName, int tailLines) {
        try {
            return clientFactory.coreV1Api()
                    .readNamespacedPodLog(podName, namespace, null, null, null,
                            null, null, null, null, tailLines, null);
        } catch (ApiException e) {
            throw new RuntimeException("获取 Pod 日志失败: " + podName, e);
        }
    }

    @Override
    public ServiceInfo createOrUpdateService(String namespace, String name, String type,
                                              Map<String, String> selector, int port, int targetPort) {
        try {
            V1Service service = new V1Service()
                    .metadata(new V1ObjectMeta().name(name).namespace(namespace))
                    .spec(new V1ServiceSpec()
                            .type(type)
                            .selector(selector)
                            .ports(List.of(new V1ServicePort().port(port).targetPort(new io.kubernetes.client.custom.IntOrString(targetPort)))));
            V1Service result;
            try {
                clientFactory.coreV1Api().readNamespacedService(name, namespace, null);
                result = clientFactory.coreV1Api().replaceNamespacedService(name, namespace, service, null, null, null, null);
                log.info("更新 Service: {}/{}", namespace, name);
            } catch (ApiException e) {
                if (e.getCode() == 404) {
                    result = clientFactory.coreV1Api().createNamespacedService(namespace, service, null, null, null, null);
                    log.info("创建 Service: {}/{}", namespace, name);
                } else {
                    throw e;
                }
            }
            return toServiceInfo(result);
        } catch (ApiException e) {
            throw new RuntimeException("创建/更新 Service 失败: " + namespace + "/" + name, e);
        }
    }

    @Override
    public ServiceInfo getService(String namespace, String name) {
        try {
            V1Service service = clientFactory.coreV1Api().readNamespacedService(name, namespace, null);
            return toServiceInfo(service);
        } catch (ApiException e) {
            throw new RuntimeException("查询 Service 失败: " + namespace + "/" + name, e);
        }
    }

    @Override
    public void deleteService(String namespace, String name) {
        try {
            clientFactory.coreV1Api().deleteNamespacedService(name, namespace, null, null, null, null, null, null);
            log.info("删除 Service: {}/{}", namespace, name);
        } catch (ApiException e) {
            throw new RuntimeException("删除 Service 失败: " + namespace + "/" + name, e);
        }
    }

    @Override
    public void createOrUpdateConfigMap(String namespace, String name, Map<String, String> data) {
        try {
            V1ConfigMap configMap = new V1ConfigMap()
                    .metadata(new V1ObjectMeta().name(name).namespace(namespace))
                    .data(data);
            try {
                clientFactory.coreV1Api().readNamespacedConfigMap(name, namespace, null);
                clientFactory.coreV1Api().replaceNamespacedConfigMap(
                        name, namespace, configMap, null, null, null, null);
            } catch (ApiException e) {
                if (e.getCode() == 404) {
                    clientFactory.coreV1Api().createNamespacedConfigMap(
                            namespace, configMap, null, null, null, null);
                } else {
                    throw e;
                }
            }
        } catch (ApiException e) {
            throw new RuntimeException("创建/更新 ConfigMap 失败", e);
        }
    }

    @Override
    public void deleteConfigMap(String namespace, String name) {
        try {
            clientFactory.coreV1Api().deleteNamespacedConfigMap(
                    name, namespace, null, null, null, null, null, null);
        } catch (ApiException e) {
            throw new RuntimeException("删除 ConfigMap 失败", e);
        }
    }

    private DeploymentInfo toDeploymentInfo(V1Deployment d) {
        V1DeploymentStatus status = d.getStatus();
        String statusStr = "progressing";
        if (status != null && status.getAvailableReplicas() != null
                && status.getAvailableReplicas().equals(d.getSpec().getReplicas())) {
            statusStr = "available";
        }
        String image = "";
        if (d.getSpec().getTemplate().getSpec() != null
                && !d.getSpec().getTemplate().getSpec().getContainers().isEmpty()) {
            image = d.getSpec().getTemplate().getSpec().getContainers().get(0).getImage();
        }
        return DeploymentInfo.builder()
                .namespace(d.getMetadata().getNamespace())
                .name(d.getMetadata().getName())
                .image(image)
                .replicas(d.getSpec().getReplicas())
                .availableReplicas(status != null ? status.getAvailableReplicas() : 0)
                .status(statusStr)
                .build();
    }

    private ServiceInfo toServiceInfo(V1Service s) {
        V1ServiceSpec spec = s.getSpec();
        return ServiceInfo.builder()
                .namespace(s.getMetadata().getNamespace())
                .name(s.getMetadata().getName())
                .type(spec.getType())
                .selector(spec.getSelector())
                .port(spec.getPorts() != null && !spec.getPorts().isEmpty() ? spec.getPorts().get(0).getPort() : null)
                .targetPort(spec.getPorts() != null && !spec.getPorts().isEmpty() && spec.getPorts().get(0).getTargetPort() != null
                        ? spec.getPorts().get(0).getTargetPort().getIntValue() : null)
                .clusterIp(spec.getClusterIP())
                .build();
    }

    private PodInfo toPodInfo(V1Pod pod) {
        return PodInfo.builder()
                .namespace(pod.getMetadata().getNamespace())
                .name(pod.getMetadata().getName())
                .phase(pod.getStatus().getPhase())
                .nodeName(pod.getSpec().getNodeName())
                .startTime(pod.getStatus().getStartTime() != null
                        ? pod.getStatus().getStartTime().toLocalDateTime() : null)
                .build();
    }
}
```

- [ ] **Step 6: 运行测试确认通过**

Run: `cd forge-pipeline && mvn test -Dtest=AckAdapterTest -pl . 2>&1 | tail -15`
Expected: 全部 PASS

- [ ] **Step 7: Commit**

```bash
git add forge-pipeline/src/
git commit -m "feat(m2): add ACK container orchestration adapter with tests"
```

---

### Task 6: 云效 Flow CI/CD 适配器实现（TDD）

**Files:**
- Create: `forge-pipeline/src/main/java/com/shulex/forge/pipeline/adapter/flow/FlowConfig.java`
- Create: `forge-pipeline/src/main/java/com/shulex/forge/pipeline/adapter/flow/FlowClient.java`
- Create: `forge-pipeline/src/main/java/com/shulex/forge/pipeline/adapter/flow/FlowAdapter.java`
- Create: `forge-pipeline/src/test/java/com/shulex/forge/pipeline/adapter/flow/FlowAdapterTest.java`

- [ ] **Step 1: 创建 FlowConfig.java**

```java
package com.shulex.forge.pipeline.adapter.flow;

import lombok.Data;
import org.springframework.boot.context.properties.ConfigurationProperties;
import org.springframework.stereotype.Component;

@Data
@Component
@ConfigurationProperties(prefix = "forge.adapter.flow")
public class FlowConfig {
    private String baseUrl = "https://devops.aliyun.com";
    private String orgId;
    private String accessToken;
}
```

- [ ] **Step 2: 创建 FlowClient.java**

```java
package com.shulex.forge.pipeline.adapter.flow;

import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.ObjectMapper;
import com.shulex.forge.pipeline.infrastructure.http.RetryableHttpClient;

import java.util.Map;

public class FlowClient {

    private final FlowConfig config;
    private final RetryableHttpClient httpClient;
    private final ObjectMapper objectMapper;

    public FlowClient(FlowConfig config) {
        this.config = config;
        this.httpClient = new RetryableHttpClient(3, 500);
        this.objectMapper = new ObjectMapper();
    }

    private Map<String, String> headers() {
        return Map.of("x-devops-token", config.getAccessToken());
    }

    private String baseUrl() {
        String url = config.getBaseUrl();
        return url.endsWith("/") ? url.substring(0, url.length() - 1) : url;
    }

    public JsonNode createPipeline(String orgId, String name, String repoUrl, String branch, String yamlContent) {
        String url = String.format("%s/oapi/v1/flow/pipelines?orgId=%s", baseUrl(), orgId);
        try {
            var om = new ObjectMapper();
            var payload = om.createObjectNode()
                    .put("name", name)
                    .put("repoUrl", repoUrl)
                    .put("branch", branch)
                    .put("yamlContent", yamlContent);
            return parseResult(httpClient.post(url, om.writeValueAsString(payload), headers()));
        } catch (Exception e) {
            throw new RuntimeException("创建流水线失败", e);
        }
    }

    public JsonNode listPipelines(String orgId) {
        String url = String.format("%s/oapi/v1/flow/pipelines?orgId=%s", baseUrl(), orgId);
        return parseResult(httpClient.get(url, headers()));
    }

    public JsonNode getPipeline(String orgId, String pipelineId) {
        String url = String.format("%s/oapi/v1/flow/pipelines/%s?orgId=%s",
                baseUrl(), pipelineId, orgId);
        return parseResult(httpClient.get(url, headers()));
    }

    public JsonNode triggerPipeline(String orgId, String pipelineId, String branch) {
        String url = String.format("%s/oapi/v1/flow/pipelines/%s/runs?orgId=%s",
                baseUrl(), pipelineId, orgId);
        String payload = String.format("{\"branchModeBranchs\":[\"%s\"]}", branch);
        return parseResult(httpClient.post(url, payload, headers()));
    }

    public JsonNode getPipelineRun(String orgId, String pipelineId, String runId) {
        String url = String.format("%s/oapi/v1/flow/pipelines/%s/runs/%s?orgId=%s",
                baseUrl(), pipelineId, runId, orgId);
        return parseResult(httpClient.get(url, headers()));
    }

    public JsonNode listPipelineRuns(String orgId, String pipelineId, int limit) {
        String url = String.format("%s/oapi/v1/flow/pipelines/%s/runs?orgId=%s&maxResults=%d",
                baseUrl(), pipelineId, orgId, limit);
        return parseResult(httpClient.get(url, headers()));
    }

    public String getPipelineRunLogs(String orgId, String pipelineId, String runId) {
        String url = String.format("%s/oapi/v1/flow/pipelines/%s/runs/%s/logs?orgId=%s",
                baseUrl(), pipelineId, runId, orgId);
        return httpClient.get(url, headers());
    }

    private JsonNode parseResult(String body) {
        try {
            JsonNode root = objectMapper.readTree(body);
            return root.has("result") ? root.get("result") : root;
        } catch (Exception e) {
            throw new RuntimeException("JSON 解析失败", e);
        }
    }
}
```

- [ ] **Step 3: 写 FlowAdapter 失败测试**

```java
package com.shulex.forge.pipeline.adapter.flow;

import com.shulex.forge.pipeline.adapter.model.PipelineInfo;
import com.shulex.forge.pipeline.adapter.model.PipelineRunInfo;
import okhttp3.mockwebserver.MockResponse;
import okhttp3.mockwebserver.MockWebServer;
import org.junit.jupiter.api.AfterEach;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;

import java.io.IOException;
import java.util.List;

import static org.assertj.core.api.Assertions.assertThat;

class FlowAdapterTest {

    private MockWebServer mockServer;
    private FlowAdapter adapter;

    @BeforeEach
    void setUp() throws IOException {
        mockServer = new MockWebServer();
        mockServer.start();
        FlowConfig config = new FlowConfig();
        config.setBaseUrl(mockServer.url("/").toString());
        config.setOrgId("test-org");
        config.setAccessToken("test-token");
        adapter = new FlowAdapter(new FlowClient(config));
    }

    @AfterEach
    void tearDown() throws IOException {
        mockServer.shutdown();
    }

    @Test
    void getType_returnsFlow() {
        assertThat(adapter.getType()).isEqualTo("flow");
    }

    @Test
    void listPipelines_convertToModel() {
        mockServer.enqueue(new MockResponse()
                .setBody("{\"result\":[{\"id\":\"p-1\",\"name\":\"build-pipeline\",\"status\":\"active\"}]}")
                .setResponseCode(200));

        List<PipelineInfo> list = adapter.listPipelines("test-org");
        assertThat(list).hasSize(1);
        assertThat(list.get(0).getName()).isEqualTo("build-pipeline");
    }

    @Test
    void triggerPipeline_returnsRunInfo() {
        mockServer.enqueue(new MockResponse()
                .setBody("{\"result\":{\"pipelineRunId\":\"r-1\",\"pipelineId\":\"p-1\",\"status\":\"RUNNING\"}}")
                .setResponseCode(200));

        PipelineRunInfo run = adapter.triggerPipeline("test-org", "p-1", "main");
        assertThat(run.getRunId()).isEqualTo("r-1");
        assertThat(run.getStatus()).isEqualTo("running");
    }

    @Test
    void getPipelineRun_returnsStatus() {
        mockServer.enqueue(new MockResponse()
                .setBody("{\"result\":{\"pipelineRunId\":\"r-1\",\"pipelineId\":\"p-1\",\"status\":\"SUCCESS\"}}")
                .setResponseCode(200));

        PipelineRunInfo run = adapter.getPipelineRun("test-org", "p-1", "r-1");
        assertThat(run.getStatus()).isEqualTo("success");
    }

    @Test
    void getPipelineRunLogs_returnsLogText() {
        mockServer.enqueue(new MockResponse()
                .setBody("Build started...\nCompiling...\nBuild success.")
                .setResponseCode(200));

        String logs = adapter.getPipelineRunLogs("test-org", "p-1", "r-1");
        assertThat(logs).contains("Build success");
    }
}
```

- [ ] **Step 4: 运行测试确认失败**

Run: `cd forge-pipeline && mvn test -Dtest=FlowAdapterTest -pl . 2>&1 | tail -20`
Expected: 编译失败 — FlowAdapter 不存在

- [ ] **Step 5: 实现 FlowAdapter**

```java
package com.shulex.forge.pipeline.adapter.flow;

import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.JsonNode;
import com.shulex.forge.pipeline.adapter.model.PipelineCreateRequest;
import com.shulex.forge.pipeline.adapter.model.PipelineInfo;
import com.shulex.forge.pipeline.adapter.model.PipelineRunInfo;
import com.shulex.forge.pipeline.adapter.spi.CiCdAdapter;
import lombok.extern.slf4j.Slf4j;
import org.springframework.stereotype.Component;

import java.util.ArrayList;
import java.util.List;

@Slf4j
@Component
public class FlowAdapter implements CiCdAdapter {

    private final FlowClient client;

    public FlowAdapter(FlowClient client) {
        this.client = client;
    }

    @Override
    public String getType() {
        return "flow";
    }

    @Override
    public PipelineInfo createPipeline(String orgId, PipelineCreateRequest request) {
        JsonNode node = client.createPipeline(orgId, request.getName(),
                request.getRepoUrl(), request.getBranch(), request.getYamlContent());
        return PipelineInfo.builder()
                .id(node.path("id").asText())
                .name(node.path("name").asText())
                .status("active")
                .build();
    }

    @Override
    public List<PipelineInfo> listPipelines(String orgId) {
        JsonNode result = client.listPipelines(orgId);
        List<PipelineInfo> list = new ArrayList<>();
        for (JsonNode node : result) {
            list.add(PipelineInfo.builder()
                    .id(node.path("id").asText())
                    .name(node.path("name").asText())
                    .status(node.path("status").asText())
                    .build());
        }
        return list;
    }

    @Override
    public PipelineInfo getPipeline(String orgId, String pipelineId) {
        JsonNode node = client.getPipeline(orgId, pipelineId);
        return PipelineInfo.builder()
                .id(node.path("id").asText())
                .name(node.path("name").asText())
                .status(node.path("status").asText())
                .build();
    }

    @Override
    public PipelineRunInfo triggerPipeline(String orgId, String pipelineId, String branch) {
        JsonNode node = client.triggerPipeline(orgId, pipelineId, branch);
        return toRunInfo(node);
    }

    @Override
    public PipelineRunInfo getPipelineRun(String orgId, String pipelineId, String runId) {
        JsonNode node = client.getPipelineRun(orgId, pipelineId, runId);
        return toRunInfo(node);
    }

    @Override
    public List<PipelineRunInfo> listPipelineRuns(String orgId, String pipelineId, int limit) {
        JsonNode result = client.listPipelineRuns(orgId, pipelineId, limit);
        List<PipelineRunInfo> list = new ArrayList<>();
        for (JsonNode node : result) {
            list.add(toRunInfo(node));
        }
        return list;
    }

    @Override
    public String getPipelineRunLogs(String orgId, String pipelineId, String runId) {
        return client.getPipelineRunLogs(orgId, pipelineId, runId);
    }

    private PipelineRunInfo toRunInfo(JsonNode node) {
        return PipelineRunInfo.builder()
                .runId(node.path("pipelineRunId").asText())
                .pipelineId(node.path("pipelineId").asText())
                .status(node.path("status").asText().toLowerCase())
                .build();
    }
}
```

- [ ] **Step 6: 运行测试确认通过**

Run: `cd forge-pipeline && mvn test -Dtest=FlowAdapterTest -pl . 2>&1 | tail -15`
Expected: 全部 PASS

- [ ] **Step 7: Commit**

```bash
git add forge-pipeline/src/
git commit -m "feat(m2): add Cloud Effect Flow CI/CD adapter with tests"
```

---

### Task 7: AdapterRegistry 测试 + 适配器 Bean 注册 + 健康检查 API

**Files:**
- Create: `forge-pipeline/src/main/java/com/shulex/forge/pipeline/adapter/codeup/CodeupBeanConfig.java`
- Create: `forge-pipeline/src/main/java/com/shulex/forge/pipeline/adapter/flow/FlowBeanConfig.java`
- Create: `forge-pipeline/src/main/java/com/shulex/forge/pipeline/entrance/controller/AdapterHealthController.java`
- Create: `forge-pipeline/src/test/java/com/shulex/forge/pipeline/adapter/spi/AdapterRegistryTest.java`

- [ ] **Step 1: 创建 CodeupBeanConfig.java**

```java
package com.shulex.forge.pipeline.adapter.codeup;

import org.springframework.context.annotation.Bean;
import org.springframework.context.annotation.Configuration;

@Configuration
public class CodeupBeanConfig {

    @Bean
    public CodeupClient codeupClient(CodeupConfig config) {
        return new CodeupClient(config);
    }
}
```

- [ ] **Step 2: 创建 FlowBeanConfig.java**

```java
package com.shulex.forge.pipeline.adapter.flow;

import org.springframework.context.annotation.Bean;
import org.springframework.context.annotation.Configuration;

@Configuration
public class FlowBeanConfig {

    @Bean
    public FlowClient flowClient(FlowConfig config) {
        return new FlowClient(config);
    }
}
```

- [ ] **Step 3: 写 AdapterRegistryTest**

```java
package com.shulex.forge.pipeline.adapter.spi;

import com.shulex.forge.pipeline.adapter.model.*;
import org.junit.jupiter.api.Test;

import java.util.List;
import java.util.Map;

import static org.assertj.core.api.Assertions.assertThat;
import static org.assertj.core.api.Assertions.assertThatThrownBy;

class AdapterRegistryTest {

    @Test
    void registersAndRetrievesAdapters() {
        CodeHostingAdapter mockCodeHosting = new CodeHostingAdapter() {
            @Override public String getType() { return "test-code"; }
            @Override public List<FileTreeNode> listRepositoryTree(String r, String p, String ref) { return List.of(); }
            @Override public FileContent getFileContent(String r, String f, String ref) { return null; }
            @Override public String createCommitWithMultipleFiles(String r, String b, String m, List<CommitFile> f) { return ""; }
            @Override public BranchInfo createBranch(String r, String b, String ref) { return null; }
            @Override public void deleteBranch(String r, String b) {}
            @Override public BranchInfo getBranch(String r, String b) { return null; }
            @Override public List<BranchInfo> listBranches(String r) { return List.of(); }
            @Override public MergeRequestInfo createMergeRequest(String r, MergeRequestCreateRequest req) { return null; }
            @Override public MergeRequestInfo getMergeRequest(String r, Long id) { return null; }
            @Override public void mergeMergeRequest(String r, Long id) {}
            @Override public void closeMergeRequest(String r, Long id) {}
            @Override public void addMergeRequestComment(String r, Long id, String c) {}
            @Override public WebhookInfo createWebhook(String r, String u, String s, String e) { return null; }
            @Override public List<WebhookInfo> listWebhooks(String r) { return List.of(); }
            @Override public void deleteWebhook(String r, Long id) {}
        };

        AdapterRegistry registry = new AdapterRegistry(
                List.of(mockCodeHosting), List.of(), List.of());

        assertThat(registry.getCodeHostingAdapter("test-code")).isNotNull();
    }

    @Test
    void throwsOnUnknownAdapter() {
        AdapterRegistry registry = new AdapterRegistry(List.of(), List.of(), List.of());

        assertThatThrownBy(() -> registry.getCodeHostingAdapter("nonexistent"))
                .isInstanceOf(IllegalArgumentException.class);
    }
}
```

- [ ] **Step 4: 运行测试确认通过**

Run: `cd forge-pipeline && mvn test -Dtest=AdapterRegistryTest -pl . 2>&1 | tail -15`
Expected: 全部 PASS

- [ ] **Step 5: 创建 AdapterHealthController.java**

```java
package com.shulex.forge.pipeline.entrance.controller;

import com.shulex.forge.pipeline.adapter.spi.AdapterRegistry;
import com.shulex.forge.pipeline.common.Result;
import org.springframework.web.bind.annotation.GetMapping;
import org.springframework.web.bind.annotation.RequestMapping;
import org.springframework.web.bind.annotation.RestController;

import java.util.Map;

@RestController
@RequestMapping("/api/adapters")
public class AdapterHealthController {

    private final AdapterRegistry adapterRegistry;

    public AdapterHealthController(AdapterRegistry adapterRegistry) {
        this.adapterRegistry = adapterRegistry;
    }

    @GetMapping("/health")
    public Result<Map<String, Object>> health() {
        return Result.ok(Map.of(
                "status", "UP",
                "registeredAdapters", adapterRegistry.getRegisteredAdapterTypes()
        ));
    }
}
```

- [ ] **Step 6: 运行全部测试**

Run: `cd forge-pipeline && mvn test -pl . 2>&1 | tail -15`
Expected: 全部 PASS

- [ ] **Step 7: Commit**

```bash
git add forge-pipeline/src/
git commit -m "feat(m2): add adapter registry, bean config, and health check API"
```

---

### Task 8: 应用配置 + 全量测试 + Docker 重建

**Files:**
- Modify: `forge-pipeline/src/main/resources/application.yml`

- [ ] **Step 1: 更新 application.yml**

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
    enabled: false
  data:
    redis:
      host: localhost
      port: 6379

# 适配器配置（本地开发默认值，生产环境通过 Nacos 覆盖）
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

mybatis-plus:
  mapper-locations: classpath*:mapper/**/*.xml
  configuration:
    map-underscore-to-camel-case: true
```

- [ ] **Step 2: 运行全部测试**

Run: `cd forge-pipeline && mvn clean test -pl . 2>&1 | tail -15`
Expected: 全部 PASS

- [ ] **Step 3: 打包 + Docker 重建**

```bash
cd forge-pipeline && mvn clean package -DskipTests -q
docker compose build forge-pipeline
docker compose up -d forge-pipeline
```

- [ ] **Step 4: 验证健康检查 API**

```bash
sleep 15
curl -s http://localhost:8083/api/adapters/health | head -c 200
```

Expected: `{"code":0,"message":"success","data":{"status":"UP",...}}`

- [ ] **Step 5: Commit（如有调整）**

```bash
git add forge-pipeline/
git commit -m "feat(m2): update config and verify Docker deployment"
```

---

## M2 完成标准

- [ ] forge-pipeline 编译、测试全部通过
- [ ] 3 个适配器 SPI 接口定义完整（CodeHostingAdapter / ContainerOrchestrationAdapter / CiCdAdapter）
- [ ] AdapterRegistry 支持按类型查找适配器，动态返回已注册适配器列表
- [ ] Codeup 适配器实现完整：文件树读取、文件内容获取、原子提交、分支管理、MR 管理、Webhook 管理
- [ ] ACK 适配器实现完整：Namespace/Deployment/Service/Pod/ConfigMap 的 CRUD
- [ ] Flow 适配器实现完整：流水线创建、查询、触发、状态查询、日志获取
- [ ] HTTP 客户端支持重试（指数退避）+ 令牌桶限流（Resilience4j RateLimiter）
- [ ] 适配器缓存服务（Redis）可用于缓存外部 API 响应
- [ ] 凭证管理服务可从 Spring Environment 读取加密配置
- [ ] 所有适配器有单元测试（MockWebServer / Mockito）
- [ ] 健康检查 API：`GET /api/adapters/health`
- [ ] Docker 部署成功，健康检查 API 可通过 APISIX 网关访问
- [ ] 所有变更已 commit
