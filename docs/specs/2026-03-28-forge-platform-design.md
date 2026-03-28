# Forge Platform — AI 驱动的快速产品孵化平台设计文档

> **日期**: 2026-03-28
> **状态**: Reviewed (Round 2 Approved)
> **作者**: Harvey + Claude

---

## 1. 愿景与目标

### 1.1 核心愿景

让完全不懂代码的产品经理/运营，通过自然语言描述需求，由 AI 基于成熟的企业级规范和脚手架，生成高质量代码，经过自动化审查后稳定发布到生产环境。

### 1.2 目标用户

**A 类用户：产品经理 / 运营**
- 懂业务逻辑但不写代码
- 用自然语言描述需求
- AI 完全生成代码，用户只做业务确认

### 1.3 核心能力

| 能力 | 说明 |
|------|------|
| 从零孵化 | 产品经理说"我要一个优惠券系统"，AI 基于脚手架生成完整新项目 |
| 迭代增强 | 产品经理说"加一个通知渠道"，AI 理解现有架构，在正确位置插入代码 |
| 信任分级 | 低风险全自动（生成 → Review → 合并 → 部署），高风险人工审批兜底 |
| 多入口 | CLI + Web 工作台 + 钉钉/飞书 IM，统一后端引擎 |

### 1.4 核心理念

**程序员的"心法"不在 AI 模型里，在规范中心（forge-specs）里。** AI 只是执行者，规范才是灵魂。小白不需要懂代码，但 AI 必须严格遵守沉淀多年的工程规范。

---

## 2. 已有技术资产

### 2.1 saas-work 技术沉淀

| 项目 | 版本 | 能力 | 复用方式 |
|------|------|------|---------|
| **solar-foundation** | 1.0.5 | 多租户微服务基座（16 个组件：Nacos/Apollo/Redis/RocketMQ/XXL-Job/ShardingSphere 等） | 演进为 forge-foundation，升级多版本兼容 |
| **hunter-saas-permission** | 1.0.5 | RBAC + 注解驱动 + MyBatis 行级数据权限插件 | 核心能力整合进 forge-identity + forge-component-permission |
| **hunter-saas-shadow** | 0.0.1 | 影子流量路由（ThreadLocal 上下文 + 多中间件钩子） | 整合进 forge-component-shadow，支撑 AI 代码的安全验证 |
| **kohinur** | 1.0.16 | 灰度发布平台（11 个插件，支持 Ribbon/Feign/Gateway/Zuul） | 整合进 forge-component-gray + forge-pipeline 灰度部署 |
| **kohinur-pilot-facade** | 4.4.0 | Vue 2 灰度管理 UI | 设计模式参考，forge-portal 用 Vue 3 重写 |
| **beacon（已有经验）** | — | Socket.IO 实时交互网关（server 分发鉴权 + client 业务消费） | 演进为 forge-beacon，增加 SDK starter |

### 2.2 aegis 工程实践沉淀

| 沉淀 | 内容 | 复用方式 |
|------|------|---------|
| 编码规范 | 阿里巴巴 Java + Google Style 完整规约 | 纳入 forge-specs 规范库 |
| 中间件规范 | Redis Key 规范、Kafka Topic 规范、XXL-Job Handler 规范、SQL 规范 | 纳入 forge-specs |
| 熔断模式 | Redis 滑动窗口熔断器 + 多通道 fallback | forge-engine 的多模型熔断复用此模式 |
| 调度模式 | 中心化 TaskDispatcher → Channel Adapter | forge-engine 的 Orchestrator → Worker 复用此模式 |
| DevOps | Docker 多阶段构建 + 多环境配置 + 健康检查 | forge-pipeline 模板基础 |
| 文档体系 | docs/guides/ 标准化文档 + 代码同步更新原则 | forge-specs 文档治理模式 |

---

## 3. 整体架构

### 3.1 架构选型：中心化 Agent 架构 + 高可用保障

选择中心化架构的理由：
1. MVP 阶段最快落地
2. 与 Aegis 调度模式一致，模式复用
3. 风险分级天然支持——中央大脑决定自动还是人工
4. 后续可演进为混合架构

### 3.2 全局架构图

```
┌─────────────────── 接入层 ───────────────────┐
│  forge-portal(Web)  forge-bot(IM)  CLI(本地) │
└──────────────┬───────────────────┬────────────┘
               │ HTTP / WebSocket  │
┌──────────────▼───────────────────▼────────────┐
│           forge-beacon (实时交互网关)           │
│   Socket.IO 长连接 / 鉴权 / 业务隔离           │
└──────────────┬────────────────────────────────┘
               │
┌──────────────▼────────────────────────────────┐
│           forge-identity (统一鉴权中心)         │
│   动态鉴权链 / RBAC+ABAC / Token 管理          │
└──────────────┬────────────────────────────────┘
               │
┌──────────────▼────────────────────────────────┐
│           forge-engine (中央大脑)               │
│                                                │
│  ┌─────────┐  ┌─────────┐  ┌────────────┐    │
│  │ 需求解析 │  │ 任务编排 │  │  风险评估   │    │
│  │ Agent   │  │ 状态机   │  │  决策器    │    │
│  └────┬────┘  └────┬────┘  └─────┬──────┘    │
│       └────────────┼─────────────┘            │
│                    ▼                           │
│  ┌──────────────────────────────────────┐     │
│  │       Worker Pool (弹性伸缩)          │     │
│  │  CodeGen │ Review │ TestGen │ Deploy  │     │
│  └──────────────────────────────────────┘     │
└───────┬──────────┬──────────┬─────────────────┘
        ▼          ▼          ▼
┌──────────┐ ┌─��────────┐ ┌──────────┐
│forge-specs│ │forge-pipe│ │  外部服务  │
│ 规范中心  │ │  line    │ │ Codeup   │
│          │ │ DevOps   │ │ 云效流水线 │
│          │ │ 自动化   │ │ 阿里云ACK │
└──────────┘ └──────────┘ └──────────┘
```

### 3.3 高可用设计

| 风险点 | 解决方案 | 借鉴来源 |
|--------|---------|---------|
| engine 单点 | 多实例部署 + Leader 选举（K8s Lease 对象，与 ACK 原生集成），Leader 做调度编排，Follower 热备秒切 | K8s Controller Manager 选举机制 |
| 任务丢失 | 任务持久化 DB + Redis 双写，重启后从 DB 恢复未完成任务 | Aegis delayed_queue + dead_letter |
| Worker 崩溃 | Worker 心跳注册 + 任务 lease（Redis EXPIRE），超时未完成自动释放给其他 Worker | Celery、Sidekiq |
| 长任务中断 | 任务拆分为多个 checkpoint step，每步持久化中间状态，恢复时从最后 checkpoint 继续 | Airflow TaskInstance、GitHub Actions |
| 流量突增 | Worker Pool K8s HPA，基于队列深度自动扩容 Worker Pod | K8s HPA + KEDA |
| AI 模型故障 | 多模型 fallback 链（Claude → GPT → 通义），超时/限流自动降级 | Aegis circuit breaker + channel fallback |
| 数据一致性 | 编排状态机 + 幂等设计，唯一 traceId，重复执行跳过已完成步骤 | Temporal Workflow、Saga 模式 |
| 网关高可用 | beacon 无状态多实例，Socket.IO Redis Adapter 跨实例广播 | Socket.IO 官方方案 |
| 脑裂防护 | K8s Lease 对象自带 TTL 续约机制，续约失败主动降级，新 Leader 等旧 lease 过期后接管；备选方案：Nacos 选举 | K8s Lease API、Nacos |
| 级联故障 | 服务间调用设置超时 + 熔断 + 舱壁隔离（Bulkhead），防止单个下游拖垮整体 | Resilience4j、Sentinel |
| 数据库故障 | 核心表读写分离，写入走主库，查询走从库；引擎关键状态同时写 Redis 保证短期可用 | MySQL 主从 + Redis 缓存兜底 |

### 3.4 核心状态机（任务生命周期）

```
SUBMITTED → ANALYZING → PLANNING → GENERATING → REVIEWING → TESTING → DEPLOYING → DONE
                                                    │                      │
                                               HUMAN_REVIEW            ROLLBACK
                                                    │
                                              APPROVED / REJECTED
```

- 每个状态转换持久化到 DB + 发送事件到 beacon
- 任何环节失败进入 `FAILED`，可从失败点重试（最多 3 轮，超限升级人工）
- `HUMAN_REVIEW` 是风险分级关键节点——低风险跳过，高风险卡住等人审批

---

## 4. 项目拆分

### 4.1 项目清单

| # | 项目名 | 定位 | 技术栈 |
|---|--------|------|--------|
| 1 | **forge-foundation** | 新一代多版本基座 | Java 17 + SB 3.2（MVP），多版本 Phase 4+ |
| 2 | **forge-identity** | 统一身份认证 + 动态鉴权中心 | Java 17, Spring Boot 3.x |
| 3 | **forge-engine** | AI 引擎核心（中央大脑） | Java 17, Spring Boot 3.x |
| 4 | **forge-pipeline** | DevOps 自动化 | YAML 模板 + Java 服务 |
| 5 | **forge-portal** | Web 工作台 + 代码可视化 | Vue 3 + TypeScript |
| 6 | **forge-bot** | IM 机器人（钉钉/飞书） | Java 17 / Node.js |
| 7 | **forge-beacon** | 实时交互网关 | Node.js, Socket.IO |
| 8 | **forge-specs** | 规范中心 | Markdown + JSON + Java 服务 |

### 4.2 项目依赖关系

```
forge-foundation (基座，所有 Java 项目依赖)
       │
       ├── forge-identity (鉴权，被所有需要认证的服务依赖)
       │       │
       │       ├── forge-engine (核心引擎)
       │       │       │
       │       │       ├── forge-pipeline (DevOps，被 engine 调用)
       │       │       │
       │       │       └── forge-specs (规范，被 engine 消费)
       │       │
       │       ├── forge-portal (Web 前端，调用 engine + identity API)
       │       │
       │       ├── forge-bot (IM 机器人，调用 engine API)
       │       │
       │       └── forge-beacon (实时网关，对接 identity 鉴权)
```

---

## 5. 各项目详细设计

### 5.1 forge-foundation — 新一代多版本基座

#### 5.1.1 设计目标

从 solar-foundation 演进而来，核心升级：
- 支持 Java 8 / 11 / 17 / 21 多版本
- 支持 Spring Boot 2.x / 3.x
- 组件更丰富（新增 beacon SDK、OSS、消息队列双支持）
- 新增的组件补充 solar-foundation 缺失的能力

#### 5.1.2 模块结构

> **MVP 范围（Phase 1~3）**：仅 JDK 17 + Spring Boot 3.2。多版本 BOM 在 Phase 4 按实际需要逐步添加。

```
forge-foundation/
├── forge-bom/                         # 统一依赖版本管理（BOM）
│   ├── forge-bom-jdk17-sb3/          # Java 17 + Spring Boot 3.2.x（MVP 唯一版本）
│   ├── forge-bom-jdk8-sb2/           # Java 8 + Spring Boot 2.6.x（Phase 4+）
│   ├── forge-bom-jdk11-sb2/          # Java 11 + Spring Boot 2.7.x（Phase 4+）
│   └── forge-bom-jdk21-sb3/          # Java 21 + Spring Boot 3.3.x（Phase 4+）
│
├── forge-component/                   # 可复用组件
│   ├── forge-component-common/        # 工具类（无框架依赖，纯 Java）
│   │   ├── 结果封装 Result<T>
│   │   ├── 异常体系 BaseException → BizException / SysException
│   │   ├── 分页 PageQuery / PageResult
│   │   ├── 断言 AssertUtils
│   │   └── JSON / Date / String 工具
│   │
│   ├── forge-component-redis/         # Redis（Redisson + Spring Data Redis）
│   ├── forge-component-nacos/         # 服务发现 + 配置中心
│   ├── forge-component-database/      # MyBatis-Plus + 多数据源 + Flyway
│   ├── forge-component-mq/            # 消息队列抽象层
│   │   ├── forge-mq-rocketmq/        # RocketMQ 实现
│   │   └── forge-mq-kafka/           # Kafka 实现
│   ├── forge-component-xxl-job/       # 分布式调度
│   ├── forge-component-oss/           # 对象存储（阿里云 OSS / MinIO）
│   ├── forge-component-logging/       # 统一日志 + Logstash 对接
│   ├── forge-component-metrics/       # 指标采集（Prometheus / Micrometer）
│   ├── forge-component-gray/          # 灰度发布（复用 Kohinur 核心）
│   ├── forge-component-shadow/        # 影子流量（复用 shadow 核心）
│   ├── forge-component-permission/    # 权限控制（复用 permission 核心）
│   ├── forge-component-beacon/        # 实时通信 SDK（对接 forge-beacon）
│   ├── forge-component-cache/         # 多级缓存（Caffeine L1 + Redis L2）
│   ├── forge-component-idgen/         # 分布式 ID 生成（雪花 / Leaf / UUID）
│   ├── forge-component-ratelimit/     # 限流（Redis 滑动窗口 + Sentinel）
│   ├── forge-component-audit/         # 操作审计日志
│   └── forge-component-i18n/          # 国际化
│
├── forge-starter/                     # 一键启动器
│   ├── forge-starter-service/         # 标准微服务（含常用组件）
│   ├── forge-starter-gateway/         # API 网关服务
│   └── forge-starter-minimal/         # 最小化服务（仅 common + web）
│
└── forge-parent/                      # 父 POM（按版本矩阵）
```

> **MVP 组件范围**：Phase 1 只建 forge-engine 自身需要的 6 个核心组件（common、redis、database、mq-kafka、logging、oss）。其他组件按需添加，不预先构建。

#### 5.1.3 多版本兼容策略（Phase 4+）

| 挑战 | 解决方案 |
|------|---------|
| `javax.*` → `jakarta.*`（SB2 → SB3） | 核心代码用适配层隔离，通过 Maven profile 条件引入不同的 bridge 模块 |
| Spring Cloud 版本差异 | BOM 内锁定对应版本（Hoxton/2021.x/2023.x） |
| 组件 API 差异 | SPI + 适配器模式，对外接口统一 |
| 编译验证 | CI 矩阵，每个 BOM 版本独立跑测试 |

#### 5.1.4 相比 solar-foundation 的补充

| 新增能力 | 说明 | solar 缺失原因 |
|----------|------|---------------|
| 多级缓存 | Caffeine L1 + Redis L2，热数据命中率更高 | solar 只有单层 Redis |
| 分布式 ID | 雪花算法 + Leaf segment，可配置切换 | solar 依赖数据库自增 |
| 限流组件 | Redis 滑动窗口 + Sentinel 降级 | solar 无内置限流 |
| 操作审计 | 注解驱动的操作日志记录 | solar 无审计 |
| 消息队列抽象 | 统一 API，RocketMQ / Kafka 可切换 | solar 只支持 RocketMQ |
| OSS 组件 | 阿里云 OSS / MinIO 双支持 | solar 无 OSS |
| 指标采集 | Micrometer + Prometheus，标准 /metrics 端点 | solar 用阿里内部 metrics |
| 国际化 | 消息码 + 多语言资源管理 | solar 无 i18n |

---

### 5.2 forge-identity — 统一动态鉴权中心

#### 5.2.1 设计目标

一个系统动态控制多种鉴权方式。鉴权方式可插拔，权限规则动态可配，不改代码就能切换和组合。

#### 5.2.2 架构

```
所有入口 (Portal / Bot / CLI / API / Git)
          │
          ▼
┌──────────────────────────────────────────────┐
│            forge-identity                     │
│                                               │
│  ┌─────────────────────────────────────┐     │
│  │     Authentication Gateway          │     │
│  │     (动态鉴权链 — 责任链模式)        │     │
│  │                                     │     │
│  │  插件式鉴权器（运行时可增减）：       │     │
│  │  · OAuth2 / OIDC                    │     │
│  │  · LDAP / Active Directory          │     │
│  │  · API Token / AK-SK 签名           │     │
│  │  · 钉钉扫码 / 飞书扫码              │     │
│  │  · SSO SAML                         │     │
│  │  · SSH Key（Git 操作）              │     │
│  │  · Personal Access Token            │     │
│  │  · OAuth2 Device Flow（CLI）        │     │
│  │  · IM 平台回调验签                   │     │
│  └─────────────────────────────────────┘     │
│                                               │
│  ┌─────────────────────────────────────┐     │
│  │     Authorization Engine            │     │
│  │     (动态权限决策引擎)               │     │
│  │                                     │     │
│  │  · RBAC  — 基于角色（管理员/开发/PM）│     │
│  │  · ABAC  — 基于属性（部门/项目/环境）│     │
│  │  · PBAC  — 基于策略（JSON 规则动态下发）│   │
│  │  · 数据权限 — 行级/列级              │     │
│  │    （复用 permission 的 MyBatis 插件）│     │
│  └─────────────────────────────────────┘     │
│                                               │
│  ┌─────────────────────────────────────┐     │
│  │     Token Service                   │     │
│  │  · JWT 签发 / 刷新 / 吊销            │     │
│  │  · 多端登录管理（同时在线数控制）      │     │
│  │  · Token 黑名单（Redis Set）         │     │
│  └─────────────────────────────────────┘     │
│                                               │
│  ┌─────────────────────────────────────┐     │
│  │     MFA Service                     │     │
│  │  · TOTP（Google Authenticator）      │     │
│  │  · 短信验证码                        │     │
│  │  · 敏感操作二次验证触发器             │     │
│  └─────────────────────────────────────┘     │
└──────────────────────────────────────────────┘
```

#### 5.2.3 动态鉴权链配置

```yaml
# 运行时可热更新（存 DB，缓存 Redis，变更事件广播）
auth-chains:
  portal:
    methods: [dingtalk_scan, password, oauth2_oidc]
    mfa: optional          # 敏感操作时触发
    session_ttl: 8h

  api:
    methods: [api_token, ak_sk_signature]
    rate_limit: true
    ip_whitelist: dynamic

  git:
    methods: [ssh_key, personal_access_token]
    scope: [repo_read, repo_write, repo_admin]

  bot:
    methods: [dingtalk_callback_sign, feishu_callback_sign]
    user_binding: required

  cli:
    methods: [oauth2_device_flow, personal_access_token]
```

#### 5.2.4 权限模型

```
租户 (Tenant)
  └── 组织 (Organization)
       └── 项目空间 (Project Space)
            ├── 代码仓库 (Repository)  ── 读 / 写 / 管理
            ├── 流水线 (Pipeline)      ── 查看 / 触发 / 编辑
            ├── 环境 (Environment)     ── dev / staging / prod 各自独立
            └── AI 任务 (AI Task)      ── 提交 / 审批 / 查看
```

**三层粒度**：
- **平台级**：创建项目空间、管理鉴权配置、系统设置
- **项目级**：推代码、触发部署、审批 AI PR、管理成员
- **操作级**：生产部署、删除仓库等敏感操作需 MFA 二次验证

#### 5.2.5 鉴权器插件化设计

```java
// 鉴权器 SPI 接口
public interface AuthenticationPlugin {
    String type();                          // "oauth2", "dingtalk_scan", "api_token"...
    boolean supports(AuthRequest request);  // 是否能处理此请求
    AuthResult authenticate(AuthRequest request); // 执行鉴权
    int order();                            // 责任链顺序
}

// 运行时动态加载，DB 配置哪些 plugin 启用
// 新增鉴权方式 = 实现接口 + 注册到 DB，不改框架代码
```

---

### 5.3 forge-engine — AI 引擎核心

#### 5.3.1 模块结构

> engine-core 和 engine-worker 是**独立部署**的服务。core 负责编排调度（有状态，Leader 选举），worker 负责实际 AI 调用（无状态，可水平扩缩）。详见第 12 章。

```
forge-engine/
├── engine-api/            # 对外 API 定义（DTO、接口契约、Kafka 消息格式）
├── engine-core/           # 编排服务：状态机 + 风险评估 + 任务调度
│   └── engine-core-server/  # core 的 Spring Boot 启动入口
├── engine-model/          # 多模型路由 + 熔断 + 适配器（共享模块）
├── engine-context/        # 上下文构建器 + 代码索引 + RAG（共享模块）
└── engine-worker/         # 执行服务：CodeGen/Review/TestGen/Deploy Worker
    └── engine-worker-server/  # worker 的 Spring Boot 启动入口
```

#### 5.3.2 多模型路由

```
┌─────────────────────────────────────────┐
│         Model Router (模型路由器)        │
│                                         │
│  任务类型            → 首选模型          │
│  ───────────────     ──────────         │
│  需求分析 / 架构设计  → Claude Opus      │
│  代码生成（复杂）     → Claude Sonnet    │
│  代码生成（简单）     → 通义灵码         │
│  Code Review         → Claude Opus      │
│  测试用例生成         → Claude Sonnet    │
│  文档 / 注释生成      → Claude Haiku     │
│  代码补全 / 小修改    → Copilot / 通义   │
│                                         │
│  ┌──────────────────────────────┐       │
│  │ Circuit Breaker (熔断器)     │       │
│  │                              │       │
│  │ 每个模型独立熔断统计：        │       │
│  │ · 成功率 < 80% → OPEN       │       │
│  │ · 延迟 P99 > 30s → 标记慢    │       │
│  │ · 限流响应 → 不计入失败       │       │
│  │                              │       │
│  │ Fallback 链：                │       │
│  │ Claude Opus → Claude Sonnet  │       │
│  │   → GPT-4 → 通义 → 排队等待  │       │
│  └──────────────────────────────┘       │
└─────────────────────────────────────────┘
```

#### 5.3.3 模型适配器设计

```java
// 统一的模型调用接口
public interface ModelAdapter {
    String modelId();
    ModelResponse generate(ModelRequest request);
    ModelResponse generateStream(ModelRequest request, StreamCallback callback);
    boolean healthCheck();
}

// 各模型实现
// ClaudeAdapter → Anthropic API
// GptAdapter → OpenAI API
// TongyiAdapter → 通义灵码 API
// CopilotAdapter → GitHub Copilot API

// 路由规则可配置（DB + 热缓存）
// 新增模型 = 实现 ModelAdapter + 配置路由规则
```

#### 5.3.4 上下文构建器（Context Builder）

AI 生成高质量代码的关键在于给它足够精准的上下文：

```
┌──────────────── Context Builder ────────────────┐
│                                                  │
│  静态上下文（每个项目固定的）：                     │
│  · 编码规范        ← forge-specs/standards/      │
│  · 项目脚手架结构   ← forge-specs/templates/     │
│  · Review 规则     ← forge-specs/review-rules/   │
│  · Prompt 模板     ← forge-specs/prompts/        │
│                                                  │
│  动态上下文（每次任务动态加载的）：                  │
│  · 相关代码文件     ← Codeup API（基于需求分析    │
│  │                   确定需要读哪些文件）          │
│  · 数据库 Schema   ← 项目 Flyway 迁移文件        │
│  · API 契约        ← 已有 Controller/DTO 定义    │
│  · 最近变更        ← Git log（避免冲突）          │
│  · 用户对话历史     ← 会话存储（连续交互时）       │
│                                                  │
│  上下文优化策略：                                  │
│  · Token 预算管理   — 每个模型有上下文长度限制，    │
│  │                   按优先级裁剪                  │
│  · 代码摘要        — 大文件只送关键签名+注释       │
│  · 增量上下文      — 迭代修改时只送 diff + 周边    │
│  · RAG 检索        — 规范库太大时，向量检索相关段落 │
└──────────────────────────────────────────────────┘
```

#### 5.3.5 风险评估器（Risk Evaluator）

```yaml
# 风险评估规则（可配置，存 DB）
risk-rules:
  auto-merge:  # 低风险 → 全自动
    - type: "config_change"       # 纯配置修改
    - type: "doc_change"          # 文档变更
    - type: "test_change"         # 测试代码
    - type: "new_standalone_api"  # 新增独立接口（不影响现有）
    - type: "style_fix"           # 代码风格修复
    - max_files_changed: 5        # 影响文件数 ≤ 5
    - ai_review_score: ">= 90"   # AI Review 评分 ≥ 90

  human-review:  # 高风险 → 人工审批
    - type: "core_logic_change"   # 修改核心业务逻辑
    - type: "schema_migration"    # 数据库 Schema 变更
    - type: "security_module"     # 涉及权限/支付/安全
    - type: "shared_component"    # 修改共享组件（影响多个服务）
    - type: "infra_change"        # 基础设施变更（K8s/Docker 配置）
    - files_changed: "> 10"       # 影响文件数 > 10
    - ai_review_score: "< 90"    # AI Review 评分 < 90
    - ai_review_has_warning: true # AI Review 有告警
```

#### 5.3.6 端到端流程

```
PM: "给优惠券系统加一个按用户等级发放的功能"

① ANALYZING — 需求解析 (Claude Opus)
   · 理解意图，拆解为技术任务清单
   · 输出：需改哪些模块、新增哪些接口、数据库变更
   · 反馈给 PM：确认理解是否正确（通过 beacon 实时推送）

② PLANNING — 方案规划 (Claude Opus)
   · 加载项目上下文（代码 + Schema + 规范）
   · 生成实施方案（修改哪些文件、新增哪些类）
   · 估算风险等级

③ GENERATING — 代码生成 (Worker Pool 并行)
   · SQL 迁移文件     → Worker A (Claude Sonnet)
   · Java 后端代码    → Worker B (Claude Sonnet)
   · 前端页面         → Worker C (Claude Sonnet)
   · 单元测试         → Worker D (Claude Sonnet)
   · 每个 Worker 生成后 push 到 ai/feature-xxx 分支

④ REVIEWING — AI 审查 (Claude Opus)
   · 编码规范合规检查（对照 forge-specs 规则）
   · 安全扫描（OWASP 规则 + SQL 注入 + XSS）
   · 逻辑一致性（生成的代码是否自洽、接口是否匹配）
   · 输出：评分 + 问题列表 + 修复建议
   · 有问题 → 回到 ③ 自动修复（最多 3 轮）

⑤ TESTING — 自动化测试 (forge-pipeline)
   · 云效流水线：编译 → 单测 → 集成测试 → 镜像构建
   · 测试失败 → 回到 ③ AI 修复（最多 3 轮）
   · 3 轮未通过 → 升级人工

⑥ DEPLOYING — 部署
   · 低风险 → 自动合并 + 自动部署到 staging
   · 高风险 → 创建 MR → 推送审批通知 → 人工审批后部署
   · 生产部署 → 必须人工审批

⑦ DONE — 完成
   · 全程通过 beacon 推送进度到 Portal / 钉钉
   · PM 可在任意环节介入、暂停、取消
```

---

### 5.4 forge-pipeline — DevOps 自动化

#### 5.4.1 模块结构

```
forge-pipeline/
├── pipeline-api/          # 对外 API（触发流水线、查询状态）
├── pipeline-template/     # 流水线 YAML 模板引擎
├── pipeline-quality/      # 质量门禁规则引擎
├── pipeline-deployer/     # ACK 部署编排（Helm Chart 渲染）
├── pipeline-env/          # 临时环境管理（创建/销毁）
└── pipeline-server/       # Spring Boot 启动入口
```

#### 5.4.2 流水线模板引擎

```yaml
# 根据项目类型自动生成云效流水线配置
templates:
  java-microservice:
    stages:
      - name: build
        steps: [mvn clean package -DskipTests]
      - name: test
        steps: [mvn test, jacoco-report]
      - name: code-scan
        steps: [sonarqube-scan, dependency-check]
      - name: image
        steps: [docker build, docker push to ACR]
      - name: deploy
        steps: [helm upgrade --install]

  vue-frontend:
    stages:
      - name: install
        steps: [npm ci]
      - name: build
        steps: [npm run build]
      - name: image
        steps: [docker build nginx, docker push]
      - name: deploy
        steps: [helm upgrade --install]

  sdk-library:
    stages:
      - name: build
        steps: [mvn clean package]
      - name: test
        steps: [mvn test]
      - name: publish
        steps: [mvn deploy to private nexus]
```

#### 5.4.3 环境管理

```
临时环境（AI 分支预览）：
  · AI 推送 ai/feature-xxx 分支 → 自动创建临时 K8s namespace
  · namespace 命名：forge-preview-{task-id}
  · 包含独立的 DB（schema clone）+ Redis + 服务实例
  · MR 合并后 30min 自动销毁
  · 资源配额限制（防止临时环境耗尽集群资源）

固定环境：
  · dev      → develop 分支自动部署
  · staging  → release 分支自动部署
  · prod     → master 分支，人工审批后灰度部署
```

#### 5.4.4 质量门禁

```
编译通过          → 必须
单测覆盖率        → ≥ 配置阈值（默认 60%，可按项目调）
AI Review 通过    → 必须（来自 forge-engine）
安全扫描无高危    → 必须（SonarQube + OWASP Dependency Check）
镜像漏洞扫描      → 无 Critical（Trivy）
API 兼容性       → 不允许破坏性变更（接口删除/改签名）

任一不通过 → 阻断部署 + 通知 forge-engine AI 修复
```

#### 5.4.5 灰度部署（复用 Kohinur）

```
生产部署策略（可配置）：
  · 金丝雀发布  → 先 5% 流量，观察 10min，无异常逐步放量
  · 蓝绿部署   → 新版本完全就绪后切换流量
  · 灰度发布   → 按用户标签/租户/地域分流（Kohinur 能力）

回滚策略：
  · 健康检查失败        → 自动回滚
  · 错误率突增（> 5%）  → 自动回滚 + 告警
  · 人工触发            → 一键回滚到上一版本
```

---

### 5.5 forge-portal — Web 工作台 + 代码可视化

#### 5.5.1 技术选型

```
框架: Vue 3 + TypeScript + Vite
UI 库: Ant Design Vue 4.x
状态管理: Pinia
路由: Vue Router 4
代码编辑器: Monaco Editor（VS Code 同款）
Diff 展示: monaco-diff-editor
语法高亮: Shiki
图表: ECharts
实时通信: Socket.IO Client（对接 forge-beacon）
```

#### 5.5.2 页面结构

| 页面 | 功能 | 数据源 |
|------|------|--------|
| **需求工作台** | 自然语言对话式输入需求，AI 实时回应澄清问题，确认后提交 | forge-engine API + beacon 流式输出 |
| **任务看板** | Kanban 视图展示所有 AI 任务状态，支持筛选/搜索/批量操作 | forge-engine API + beacon 实时更新 |
| **代码浏览器** | 文件树 + Monaco Editor 语法高亮 + 在线查看 | Codeup Git API |
| **AI Diff 预览** | AI 生成的代码变更逐行展示，每段附带 AI 解释（为什么改、改了什么） | forge-engine 输出 |
| **MR 审批** | AI Review 报告 + 风险评分 + 人工审批按钮 + 评论批注 | Codeup MR API + forge-engine |
| **部署看板** | 环境状态总览、流水线进度、灰度比例、一键回滚 | forge-pipeline API |
| **项目管理** | 创建新项目（选脚手架模板 + 填配置）、成员权限管理 | forge-identity + forge-specs |
| **监控大盘** | AI 任务成功率、模型用量、代码生成量、部署频率 | forge-engine metrics |
| **系统设置** | 鉴权方式配置、AI 模型配置、风险规则配置、通知设置 | forge-identity + forge-engine |

#### 5.5.3 代码可视化（Codeup 加壳）

```
不做（Codeup 已有）：          做（体验增强）：
· Git 存储引擎               · AI Diff 智能注释（每段变更附解释）
· 代码搜索                   · 风险标注（高亮可能有问题的代码行）
· CI 构建执行器              · AI 对话式 Code Review（在代码行上提问）
· Webhook 管理               · 分支可视化（AI 分支自动命名 ai/feature-xxx）
                             · 变更影响分析（这次改动影响哪些服务/接口）
```

---

### 5.6 forge-bot — IM 机器人

#### 5.6.1 支持的 IM 平台

| 平台 | 接入方式 | 交互形式 |
|------|---------|---------|
| 钉钉 | 企业内部机器人 + Webhook | 群聊 @forge / 私聊 |
| 飞书 | 自建应用 + 事件订阅 | 群聊 @forge / 私聊 |

#### 5.6.2 交互设计

```
用户：@forge 给优惠券系统加一个按用户等级发放的功能

forge-bot：
┌─────────────────────────────────┐
│ ✅ 收到需求，我理解为：            │
│                                  │
│ 1. 在优惠券发放逻辑中新增         │
│    用户等级判断                   │
│ 2. 不同等级对应不同面额/数量      │
│ 3. 需要新增等级配置表             │
│                                  │
│ 理解正确吗？                      │
│ [确认] [修改] [取消]              │
└─────────────────────────────────┘

用户：确认

forge-bot：
┌─────────────────────────────────┐
│ 🔧 AI 开发任务 #1024            │
│ 状态：代码生成中... (2/4)        │
│ ├ ✅ SQL 迁移文件已生成           │
│ ├ ✅ 后端代码已生成               │
│ ├ ⏳ 前端页面生成中...            │
│ └ ⬜ 单元测试待生成               │
│                                  │
│ [查看代码] [查看详情] [暂停]      │
└─────────────────────────────────┘

（完成后）
forge-bot：
┌─────────────────────────────────┐
│ ✅ 任务 #1024 已完成              │
│                                  │
│ · AI Review 评分: 95/100         │
│ · 风险等级: 低                    │
│ · 已自动合并到 develop           │
│ · staging 部署成功                │
│                                  │
│ [查看代码] [查看部署] [提生产]     │
└─────────────────────────────────┘
```

---

### 5.7 forge-beacon — 实时交互网关

#### 5.7.1 模块结构

```
forge-beacon/
├── beacon-server/             # 网关服务
│   ├── 连接管理              # Socket.IO 连接池 + 心跳
│   ├── 鉴权模块              # 对接 forge-identity
│   ├── namespace 路由         # 按业务隔离
│   ├── 消息分发              # 有状态（绑定用户） + 无状态（广播）
│   └── Redis Adapter         # 跨实例消息广播
│
├── beacon-client-java/        # Java SDK (Spring Boot Starter)
│   ├── @BeaconListener 注解   # 事件监听
│   ├── BeaconTemplate        # 消息发送
│   ├── 自动重连 + ACK 确认
│   └── 断线消息缓冲
│
├── beacon-client-node/        # Node.js SDK
│   └── （同 Java SDK 能力）
│
└── beacon-common/             # 共享定义（消息类型、协议）
```

#### 5.7.2 Namespace 隔离

```
/forge-portal    → Web 工作台的实时推送（任务进度、审批通知）
/forge-bot       → IM 消息桥接（钉钉/飞书消息透传）
/forge-engine    → AI 流式输出（代码生成实时展示）
/forge-pipeline  → 部署进度推送（构建/推送/上线状态）
```

#### 5.7.3 有状态 vs 无状态连接

```
有状态连接（绑定用户会话）：
  · AI 对话 — 需要保持会话上下文，同一用户的消息路由到同一 Worker
  · 管理方式：userId → connectionId 映射存 Redis Hash
  · 断线重连：30s 内重连恢复会话，超时释放

无状态连接（订阅模式）：
  · 任务进度广播 — 订阅 task:{taskId} 频道
  · 系统通知 — 订阅 system:alert 频道
  · 管理方式：Redis Pub/Sub + Socket.IO Room
```

#### 5.7.4 消息类型

| 类型 | 方向 | 说明 |
|------|------|------|
| STREAM_OUTPUT | server → client | AI 生成代码的流式文本 |
| TASK_PROGRESS | server → client | 任务状态机变更 |
| REVIEW_RESULT | server → client | Review 结果通知 |
| DEPLOY_STATUS | server → client | 部署进度 |
| APPROVAL_REQ | server → client | 需要人工审批的请求 |
| SYSTEM_ALERT | server → client | 系统告警 |
| USER_INPUT | client → server | 用户输入（对话、确认、取消） |
| HEARTBEAT | 双向 | 心跳保活 |

---

### 5.8 forge-specs — 规范中心

#### 5.8.1 不只是静态文件，是一个可管理的规范服务

```
forge-specs/
├── standards/                    # 编码规范（AI 的 system prompt）
│   ├── java-coding-standards.md  # 阿里巴巴 Java 规约（来自现有沉淀）
│   ├── sql-standards.md          # SQL 规范
│   ├── redis-standards.md        # Redis 规范
│   ├── kafka-standards.md        # Kafka 规范
│   ├── api-design-standards.md   # RESTful API 设计规范
│   ├── security-standards.md     # 安全编码规范
│   ├── naming-conventions.md     # 命名规范汇总
│   └── git-workflow.md           # Git 分支策略 + 提交规范
│
├── templates/                    # 项目脚手架模板
│   ├── microservice-java/        # Java 微服务骨架
│   │   ├── {{project}}-api/
│   │   ├── {{project}}-common/
│   │   ├── {{project}}-dao/
│   │   ├── {{project}}-core/
│   │   ├── {{project}}-server/
│   │   ├── pom.xml.template
│   │   ├── application.yml.template
│   │   ├── Dockerfile.template
│   │   └── helm-chart/
│   ├── gateway-java/             # API 网关骨架
│   ├── frontend-vue3/            # Vue 3 前端骨架
│   └── library-java/             # Java SDK 骨架
│
├── review-rules/                 # AI Review 规则库
│   ├── alibaba-java.json         # 阿里巴巴规约检查点
│   ├── security.json             # 安全检查点（OWASP Top 10）
│   ├── performance.json          # 性能检查点
│   ├── database.json             # 数据库规范检查点
│   ├── api-compatibility.json    # API 兼容性检查
│   └── custom/                   # 团队自定义规则（可按项目覆盖）
│
├── prompts/                      # AI Prompt 模板
│   ├── requirement-analysis.md   # 需求分析 prompt
│   ├── code-generation.md        # 代码生成 prompt
│   ├── code-review.md            # Code Review prompt
│   ├── test-generation.md        # 测试生成 prompt
│   ├── doc-generation.md         # 文档生成 prompt
│   └── fix-generation.md         # 修复生成 prompt
│
├── specs-service/                # 规范服务（Java）
│   ├── 规范版本管理 API
│   ├── 规范继承解析（公司级 → 团队级 → 项目级）
│   ├── 规范合规检测 API
│   └── 规范效果度量 API
│
└── specs-eval/                   # 规范测试套件
    ├── bad-code-samples/         # 故意违规的代码样本
    ├── good-code-samples/        # 标准的代码样本
    └── eval-runner.sh            # 自动化测试脚本
```

#### 5.8.2 规范继承机制

```
公司级规范（forge-specs 默认）
    │
    ▼ 继承
团队级覆盖（可选，按团队配置）
    │ 例：某团队单测覆盖率要求 80%（高于公司默认 60%）
    ▼ 继承
项目级覆盖（可选，按项目配置）
    │ 例：某项目允许使用特定的第三方库
    ▼
最终生效规范 = merge(公司, 团队, 项目)
```

#### 5.8.3 规范效果度量

```
持续追踪：
  · AI 生成代码的规范合规率（按规则维度统计）
  · AI Review 的准确率（人工复核对比）
  · 低风险自动合并的代码线上故障率
  · 各 Prompt 模板的生成质量评分

目的：
  · 合规率低的规则 → 优化对应的 Prompt 或 Review 规则
  · 准确率低的 Review → 调整 AI 模型选择或 Prompt
  · 故障率高的自动合并 → 收紧风险评估规则
```

---

## 6. MVP 建设路线

### 第一阶段：最小闭环（跑通一个需求从输入到部署）

```
目标：简易 Web 界面输入需求 → AI 生成代码 → 推到 Codeup → 流水线构建部署
目标用户：开发者/架构师验证平台能力（PM 上手需等 Phase 2 体验完善）

项目：
  forge-specs        → 基础规范 + 1 套微服务模板 + 核心 Prompt
  forge-engine       → 编排器 + 单模型（Claude）+ CodeGen + Review Worker
                       engine-core 和 engine-worker 独立部署（Worker 从队列拉取任务）
  forge-pipeline     → 1 套 Java 微服务流水线模板 + 基础质量门禁
  forge-identity(轻) → 服务间认证（Codeup/云效/ACK 的 API Token 管理）
                       + 基础用户认证（账号密码，支撑 Web 界面登录）
  forge-portal(轻)   → 最小 Web 界面：需求输入 + 任务进度（SSE 推送，不依赖 beacon）

验收标准：
  · 用户在简易 Web 界面输入"创建一个用户管理服务"
  · AI 生成完整项目（模块结构 + 数据库 + 接口 + 基础前端）
  · 代码推送到 Codeup，流水线自动构建
  · 成功部署到 dev 环境
  · Token 消耗有记录，可按任务查询
```

### 第二阶段：可用性（让小白真正能用）

```
目标：PM 在网页上提需求 → 实时看到 AI 工作进度 → 审批发布

项目：
  forge-identity  → 增加钉钉扫码 + OAuth2 OIDC + RBAC 权限模型
  forge-portal    → 完整版：需求工作台 + 任务看板 + AI Diff 预览 + MR 审批
                    + 代码浏览器（Codeup 加壳）
                    实时推送从 SSE 升级为 WebSocket（仍内嵌 engine，不依赖 beacon）

验收标准：
  · PM 用钉钉扫码登录 Portal
  · 在需求工作台用自然语言描述需求
  · 实时看到 AI 生成进度
  · 在 AI Diff 预览页查看代码变更 + AI 解释
  · 审批后自动部署
  · 有租户级 Token 预算告警
```

### 第三阶段：扩展性

```
目标：多入口 + 多模型 + 多项目类型 + 实时网关

项目：
  forge-foundation  → JDK 17 + Spring Boot 3.2 基座（仅此版本，多版本延后）
  forge-bot         → 钉钉机器人入口
  forge-engine      → 多模型路由 + 熔断 fallback
  forge-pipeline    → 前端项目模板 + 临时环境管理 + 灰度部署
  forge-beacon      → 独立实时网关（当多入口 fan-out 成为真实需求时引入）

验收标准：
  · 钉钉群里 @forge 也能走完全流程
  · Claude 不可用时自动切换到 GPT
  · 支持孵化 Vue 3 前端项目
  · 新创建的项目基于 forge-foundation 脚手架
```

### 第四阶段：成熟化

```
目标：企业级生产可用

项目：
  全项目深化

重点：
  · forge-identity 完整的动态鉴权链 + MFA + 全部鉴权插件
  · forge-foundation 多版本扩展（按实际需要逐步补 JDK8+SB2、JDK11+SB2）
  · forge-specs 规范继承 + 效果度量
  · forge-pipeline 灰度部署（Kohinur）+ 自动回滚
  · forge-beacon Java SDK（供孵化出的项目集成长连接）
  · forge-portal 监控大盘 + 系统设置
  · 全链路可观测（Prometheus + Grafana + 链路追踪）
```

---

## 7. 技术选型总览

| 维度 | 选型 |
|------|------|
| 后端框架 | Java 17 + Spring Boot 3.2 + Spring Cloud 2023.x（Forge 平台本身） |
| 基座框架 | forge-foundation 多版本（Java 8~21 + SB 2.x/3.x） |
| 前端框架 | Vue 3 + TypeScript + Ant Design Vue + Vite |
| 实时通信 | Socket.IO (Node.js server + 多语言 client SDK) |
| AI 模型 | Claude (Anthropic) + GPT (OpenAI) + 通义灵码 (Alibaba) + Copilot (GitHub) |
| 代码托管 | 云效 Codeup（加壳，不自建 Git） |
| CI/CD | 云效流水线 |
| 容器编排 | 阿里云 ACK (Kubernetes) |
| 镜像仓库 | 阿里云 ACR |
| 数据库 | MySQL 8.0 |
| 缓存 | Redis 7 (Redisson) |
| 消息队列 | Kafka（Forge 平台）/ RocketMQ + Kafka 双支持（基座） |
| 调度 | XXL-Job |
| 服务发现 | Nacos |
| 配置中心 | Nacos（Forge）/ Apollo（兼容老项目） |
| 监控 | Prometheus + Grafana + Micrometer |
| 日志 | ELK (Elasticsearch + Logstash + Kibana) |
| 链路追踪 | SkyWalking / Jaeger |

---

## 8. 项目仓库规划

所有项目在 `D:\shulex_work` 下：

```
D:\shulex_work\
├── saas-work/               # 已有技术沉淀（只读参考）
│   ├── hunter-saas-solar-foundation/
│   ├── hunter-saas-permission/
│   ├── hunter-saas-shadow/
│   ├── Kohinur/
│   └── ...
│
├── shulex-aegis/            # 已有项目（工程实践参考）
│
└── forge/                   # Forge 平台（新建）
    ├── forge-foundation/    # 多版本基座
    ├── forge-identity/      # 统一鉴权
    ├── forge-engine/        # AI 引擎
    ├── forge-pipeline/      # DevOps 自动化
    ├── forge-portal/        # Web 工作台
    ├── forge-bot/           # IM 机器人
    ├── forge-beacon/        # 实时网关
    └── forge-specs/         # 规范中心
```

---

## 9. AI 成本模型与 Token 预算

### 9.1 单次任务 Token 消耗估算

```
典型任务："给优惠券系统加按用户等级发放功能"

① 需求分析    (Opus)    ~  5K input +  2K output =   7K tokens
② 方案规划    (Opus)    ~ 20K input +  5K output =  25K tokens
③ 代码生成 ×4 (Sonnet)  ~ 30K input + 10K output = 160K tokens (4 Worker)
④ AI Review   (Opus)    ~ 40K input +  5K output =  45K tokens
⑤ 修复重试    (Sonnet)  ~ 20K input +  5K output =  25K tokens (如果需要)
───────────────────────────────────────────────────────
单次任务总计：约 200K ~ 300K tokens
复杂任务 + 3 轮重试：可达 500K ~ 800K tokens
```

### 9.2 成本控制策略

| 层级 | 机制 | 说明 |
|------|------|------|
| **任务级** | 预估展示 | 任务提交前展示预估 Token 消耗和费用，用户确认后执行 |
| **任务级** | 硬上限 | 单次任务 Token 上限（默认 1M，可配），超限中止并通知 |
| **租户级** | 月度预算 | 每个租户设置月度 Token 预算，80% 软告警，100% 硬限制 |
| **平台级** | 并发控制 | 全局最大并发 AI 任务数（防止突发消耗），队列排队 |
| **优化级** | 上下文裁剪 | Token 预算管理器按优先级裁剪上下文，优先保留规范和核心代码 |
| **优化级** | 缓存复用 | 相同项目结构的上下文缓存，避免重复构建 |
| **优化级** | 模型降级 | 简单子任务自动用轻量模型（Haiku/通义），降低成本 |

### 9.3 Token 用量追踪

```
每次模型调用记录：
  task_id, step, model_id, input_tokens, output_tokens,
  cost_usd, latency_ms, timestamp

可视化：
  · 按任务查看 Token 消耗明细
  · 按租户查看月度趋势
  · 按模型查看用量分布
  · 成本异常告警（单任务超阈值）
```

---

## 10. 核心数据模型

### 10.1 forge-engine 数据模型

```sql
-- 任务主表
CREATE TABLE forge_task (
    id              BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    task_id         VARCHAR(64)  NOT NULL COMMENT '任务唯一ID',
    tenant_id       VARCHAR(64)  NOT NULL COMMENT '租户ID',
    user_id         BIGINT       NOT NULL COMMENT '提交用户',
    title           VARCHAR(256) NOT NULL COMMENT '需求标题',
    description     TEXT         NOT NULL COMMENT '需求描述（用户原文）',
    task_type       VARCHAR(32)  NOT NULL COMMENT 'NEW_PROJECT / FEATURE / BUGFIX',
    project_id      VARCHAR(64)           COMMENT '目标项目（迭代时必填）',
    status          VARCHAR(32)  NOT NULL COMMENT '状态机状态',
    risk_level      VARCHAR(16)           COMMENT 'LOW / HIGH',
    risk_score      INT                   COMMENT '风险评分 0-100',
    total_tokens    BIGINT       DEFAULT 0 COMMENT '累计 Token 消耗',
    total_cost_usd  DECIMAL(10,4) DEFAULT 0 COMMENT '累计费用（美元）',
    retry_count     INT          DEFAULT 0,
    error_message   TEXT,
    gmt_create      DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
    gmt_modified    DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    PRIMARY KEY (id),
    UNIQUE KEY uk_task_id (task_id),
    INDEX idx_tenant_status (tenant_id, status),
    INDEX idx_user_id (user_id)
) COMMENT '孵化任务主表';

-- 任务步骤表（checkpoint）
CREATE TABLE forge_task_step (
    id              BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    task_id         VARCHAR(64)  NOT NULL,
    step_name       VARCHAR(64)  NOT NULL COMMENT 'ANALYZING/PLANNING/GENERATING/...',
    step_order      INT          NOT NULL,
    status          VARCHAR(32)  NOT NULL COMMENT 'PENDING/RUNNING/SUCCESS/FAILED/SKIPPED',
    worker_id       VARCHAR(64)           COMMENT '执行此步骤的 Worker 实例',
    input_snapshot  JSON                  COMMENT '步骤输入（上下文摘要）',
    output_snapshot JSON                  COMMENT '步骤输出（生成结果摘要）',
    tokens_used     BIGINT       DEFAULT 0,
    started_at      DATETIME,
    completed_at    DATETIME,
    error_message   TEXT,
    gmt_create      DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
    gmt_modified    DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    PRIMARY KEY (id),
    INDEX idx_task_id (task_id)
) COMMENT '任务步骤表（checkpoint 恢复用）';

-- AI 模型调用日志
CREATE TABLE forge_model_call_log (
    id              BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    task_id         VARCHAR(64)  NOT NULL,
    step_name       VARCHAR(64)  NOT NULL,
    model_id        VARCHAR(64)  NOT NULL COMMENT 'claude-opus/claude-sonnet/gpt-4/...',
    purpose         VARCHAR(64)  NOT NULL COMMENT 'requirement_analysis/code_gen/review/...',
    input_tokens    BIGINT       NOT NULL,
    output_tokens   BIGINT       NOT NULL,
    cost_usd        DECIMAL(10,6) NOT NULL,
    latency_ms      BIGINT       NOT NULL,
    is_fallback     TINYINT(1) UNSIGNED DEFAULT 0 COMMENT '是否降级调用',
    is_success      TINYINT(1) UNSIGNED DEFAULT 1,
    error_code      VARCHAR(32),
    gmt_create      DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
    -- 注：append-only 日志表，不做 UPDATE，故省略 gmt_modified
    PRIMARY KEY (id),
    INDEX idx_task_id (task_id),
    INDEX idx_model_date (model_id, gmt_create)
) COMMENT 'AI 模型调用日志（append-only）';

-- 代码变更记录
CREATE TABLE forge_code_change (
    id              BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    task_id         VARCHAR(64)  NOT NULL,
    repo_url        VARCHAR(512) NOT NULL,
    branch_name     VARCHAR(128) NOT NULL,
    commit_hash     VARCHAR(64),
    files_changed   INT          DEFAULT 0,
    lines_added     INT          DEFAULT 0,
    lines_deleted   INT          DEFAULT 0,
    review_score    INT                   COMMENT 'AI Review 评分 0-100',
    review_summary  TEXT                  COMMENT 'AI Review 摘要',
    mr_url          VARCHAR(512)          COMMENT 'Codeup MR 链接',
    mr_status       VARCHAR(32)           COMMENT 'OPEN/MERGED/CLOSED',
    gmt_create      DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
    gmt_modified    DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    PRIMARY KEY (id),
    INDEX idx_task_id (task_id)
) COMMENT '代码变更记录';
```

### 10.2 forge-identity 数据模型

```sql
-- 租户
CREATE TABLE forge_tenant (
    id              BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    tenant_id       VARCHAR(64)  NOT NULL,
    tenant_name     VARCHAR(128) NOT NULL,
    monthly_token_budget BIGINT  DEFAULT 0 COMMENT '月度 Token 预算（0=不限）',
    status          VARCHAR(16)  NOT NULL DEFAULT 'ACTIVE',
    gmt_create      DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
    gmt_modified    DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    PRIMARY KEY (id),
    UNIQUE KEY uk_tenant_id (tenant_id)
) COMMENT '租户表';

-- 用户
CREATE TABLE forge_user (
    id              BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    tenant_id       VARCHAR(64)  NOT NULL,
    username        VARCHAR(64)  NOT NULL,
    display_name    VARCHAR(128),
    email           VARCHAR(128),
    phone           VARCHAR(32),
    password_hash   VARCHAR(256),
    status          VARCHAR(16)  NOT NULL DEFAULT 'ACTIVE',
    last_login_at   DATETIME,
    gmt_create      DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
    gmt_modified    DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    PRIMARY KEY (id),
    UNIQUE KEY uk_tenant_username (tenant_id, username),
    INDEX idx_email (email)
) COMMENT '用户表';

-- 角色
CREATE TABLE forge_role (
    id              BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    tenant_id       VARCHAR(64)  NOT NULL,
    role_code       VARCHAR(64)  NOT NULL COMMENT 'PLATFORM_ADMIN/PROJECT_ADMIN/DEVELOPER/PM',
    role_name       VARCHAR(128) NOT NULL,
    scope           VARCHAR(32)  NOT NULL COMMENT 'PLATFORM/PROJECT',
    gmt_create      DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
    gmt_modified    DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    PRIMARY KEY (id),
    UNIQUE KEY uk_tenant_role (tenant_id, role_code)
) COMMENT '角色表';

-- 用户角色绑定
CREATE TABLE forge_user_role (
    id              BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    user_id         BIGINT       NOT NULL,
    role_id         BIGINT       NOT NULL,
    project_id      VARCHAR(64)           COMMENT '项目级角色时必填',
    gmt_create      DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (id),
    gmt_modified    DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    UNIQUE KEY uk_user_role_project (user_id, role_id, project_id)
) COMMENT '用户角色绑定';

-- 鉴权配置（动态鉴权链）
CREATE TABLE forge_auth_config (
    id              BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    chain_name      VARCHAR(64)  NOT NULL COMMENT 'portal/api/git/bot/cli',
    auth_type       VARCHAR(64)  NOT NULL COMMENT 'password/dingtalk_scan/oauth2_oidc/...',
    sort_order      INT          NOT NULL DEFAULT 0,
    is_enabled      TINYINT(1) UNSIGNED NOT NULL DEFAULT 1,
    config_json     JSON                  COMMENT '鉴权器特有配置',
    gmt_create      DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
    gmt_modified    DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    PRIMARY KEY (id),
    UNIQUE KEY uk_chain_type (chain_name, auth_type)
) COMMENT '动态鉴权链配置';
```

---

## 11. Context Builder 详细设计（现有项目迭代）

### 11.1 代码索引策略

```
项目首次接入 Forge 时：
  ① 克隆 Codeup 仓库到本地索引
  ② 解析项目结构 → 生成项目地图（module 列表、包结构、类清单）
  ③ 对每个 Java 文件提取签名摘要（类名+方法签名+注释，不含方法体）
  ④ 对数据库 Schema（Flyway 文件）生成表结构摘要
  ⑤ 对 API 层（Controller + DTO）生成接口清单
  ⑥ 存储索引到 DB + 向量数据库（用于 RAG 检索）

增量更新：
  · Codeup Webhook 通知代码推送 → 增量更新受影响文件的索引
  · 定时全量刷新（每日凌晨）
```

### 11.2 上下文组装流程

```
需求解析完成后，已知需要修改哪些模块：

① 加载静态上下文
   · 编码规范（forge-specs）
   · Prompt 模板

② 精准加载相关代码（基于需求分析的输出）
   · 直接相关文件 → 完整代码
   · 接口依赖文件 → 仅签名摘要
   · 数据库 Schema → 相关表的 DDL

③ RAG 补充检索
   · 用需求描述做向量检索，补充可能遗漏的相关代码
   · 检索范围限定在同一 module 内

④ Token 预算裁剪
   · 预算 = 模型 context window - 预留输出空间 - 安全余量
   · 超预算时按优先级裁剪：
     P0 规范（不裁） > P1 直接相关代码 > P2 接口签名 > P3 RAG 补充
```

### 11.3 处理上限与降级

| 场景 | 策略 |
|------|------|
| 项目代码量 < 50 文件 | 直接全量加载，不需要索引 |
| 项目代码量 50~500 文件 | 签名索引 + 精准加载 + RAG |
| 项目代码量 > 500 文件 | 仅索引 + RAG，直接加载限定为需求相关的 module |
| 单文件 > 1000 行 | 拆分为签名 + 关键方法体，不完整加载 |
| 上下文仍超限 | 告知用户"需求范围太大"，建议拆分为多个小任务 |

### 11.4 并发冲突处理

```
AI 修改代码期间，人工也在同一项目开发：

① AI 创建独立分支（ai/feature-{taskId}），不直接操作 develop
② AI 推送代码前，先 fetch + rebase 目标分支
③ rebase 冲突 → 标记冲突文件，AI 尝试自动解决（最多 1 次）
④ 自动解决失败 → 标记任务为 CONFLICT，通知人工处理
⑤ MR 合并时由 Codeup 做最终冲突检测

原则：AI 永远不会强制覆盖人工代码。冲突时优先保护人工变更。
```

### 11.5 DB 迁移安全策略

```
AI 生成的 Flyway 迁移文件要求：
  ① 必须同时生成 UP 和 DOWN 迁移（可回滚）
  ② 在临时环境先执行 UP → 验证 → 执行 DOWN → 验证（确认可逆）
  ③ 破坏性操作（DROP TABLE/COLUMN）强制进入 HUMAN_REVIEW
  ④ 大表变更（ALTER TABLE 涉及 > 100 万行估算）标记风险并建议使用 pt-osc
```

---

## 12. forge-engine 拆分为 engine-core + engine-worker

### 12.1 职责分离

```
engine-core（编排服务，有状态）：
  · 接收需求，创建任务
  · 状态机驱动（任务生命周期管理）
  · 风险评估
  · 任务持久化（DB + Redis）
  · Leader 选举（仅 core 需要）
  · 对外 API（Portal/Bot/CLI 调用）

engine-worker（执行服务，无状态，可水平扩缩）：
  · 从任务队列（Kafka/Redis）拉取待执行步骤
  · 调用 AI 模型（模型路由 + 熔断在 Worker 内）
  · 上下文构建
  · 代码生成 / Review / 测试生成
  · 执行完毕后回报 core（状态更新）
  · 天然支持 K8s HPA 按队列深度扩缩
```

### 12.2 通信方式

```
core → worker: Kafka topic (forge.task.step) 派发步骤任务
worker → core: Kafka topic (forge.task.step.result) 回报结果
worker 心跳:   Redis Hash (forge:worker:heartbeat:{workerId}) TTL 30s
```

---

## 13. 风险与应对

| 风险 | 概率 | 影响 | 应对 |
|------|------|------|------|
| AI 生成代码质量不达标 | 高 | 高 | 多轮 Review + 修复循环；规范持续优化；初期收紧自动合并阈值 |
| AI 模型 API 不稳定/限流 | 中 | 高 | 多模型 fallback；请求排队；错峰调度 |
| 小白用户需求描述模糊 | 高 | 中 | AI 对话式澄清；需求模板引导；常见需求快捷入口 |
| 多版本基座兼容性 | 中 | 中 | 先做 JDK17+SB3，逐步补 JDK8+SB2；CI 矩阵验证 |
| 安全风险（AI 生成不安全代码） | 中 | 高 | 专项安全 Review 规则；敏感模块强制人工审批；安全扫描门禁 |
| 云效 API 能力不足 | 低 | 中 | 提前验证关键 API；不足时用 Git 原生操作 + Webhook 补充 |
| Token 费用失控 | 中 | 高 | 任务级预估 + 租户月度预算 + 硬上限中止（见第 9 章） |
| AI 生成不可逆 DB 迁移 | 中 | 高 | 必须生成 UP+DOWN 迁移；临时环境验证可逆性；破坏性操作强制人工审批 |
| 生产事故（AI 代码通过所有门禁但线上异常） | 低 | 极高 | 平台级紧急停止开关（暂停所有 AI 任务 + 阻断部署）；灰度发布 + 自动回滚 |
| 代码上下文窗口不足 | 中 | 中 | 签名摘要 + RAG 检索 + Token 预算裁剪；超大项目建议拆分需求 |
