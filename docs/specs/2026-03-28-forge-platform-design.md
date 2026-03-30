# Forge Platform — AI 驱动的快速产品孵化平台设计文档

> **日期**: 2026-03-28
> **状态**: Round 3 — Decisions Finalized
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

### 1.5 生产级定位

本平台不是 MVP 原型或概念验证，而是面向生产环境的企业级系统。所有设计决策必须满足生产级质量标准，错误处理、监控、安全、高可用不是可选的附加功能，而是核心设计要求。分阶段交付的目的是控制范围和节奏，而非降低质量标准。

---

## 2. 已有技术资产

### 2.1 saas-work 技术沉淀

| 项目 | 版本 | 能力 | 复用方式 |
|------|------|------|---------|
| **solar-foundation** | 1.0.5 | 多租户微服务基座（16 个组件：Nacos/Apollo/Redis/RocketMQ/XXL-Job/ShardingSphere 等） | 脚手架模式复用（两层结构：parent 模块 + starter jar），升级为 JDK17 + SB3，配置中心从 Apollo 迁移到 Nacos，指标采集从 CAT 迁移到 Micrometer + Prometheus |
| **hunter-saas-permission** | 1.0.5 | RBAC + 注解驱动 + MyBatis 行级数据权限插件 | 核心能力整合进 forge-identity；MyBatis 多租户插件模式复用于全平台数据隔离 |
| **hunter-saas-shadow** | 0.0.1 | 影子流量路由（ThreadLocal 上下文 + 多中间件钩子） | 整合进 forge-foundation 的影子流量组件，支撑 AI 代码的安全验证 |
| **kohinur** | 1.0.16 | 灰度发布平台（11 个插件，支持 Ribbon/Feign/Gateway/Zuul） | **已废弃**，灰度发布能力由 APISIX 网关层承接，复用 kohinur 的三层决策模型（环境分区 → 规则匹配 → 比例分流）但实现方式完全不同 |
| **kohinur-pilot-facade** | 4.4.0 | Vue 2 灰度管理 UI | 设计模式参考，灰度管理集成到 forge-portal，通过 APISIX Admin API 操作 |
| **beacon（已有经验）** | — | Socket.IO 实时交互网关（server 分发鉴权 + client 业务消费） | 演进为 forge-beacon，增加 SDK starter |

### 2.2 aegis 工程实践沉淀

| 沉淀 | 内容 | 复用方式 |
|------|------|---------|
| 编码规范 | shulex-coding-standards.md（503 行）+ project-structure.md（686 行），涵盖阿里巴巴 Java + Google Style 完整规约 | 作为 forge-specs 编码规范基线，包括 DO/DTO/VO/BO 领域模型命名、Result<T> 统一响应包装、分层异常体系（BizException / SysException / 领域异常）、集中式 ErrorCode 枚举、@RequiredArgsConstructor 依赖注入等 |
| 中间件规范 | Redis Key 规范、Kafka Topic 规范、XXL-Job Handler 规范、SQL 规范 | 纳入 forge-specs |
| 熔断模式 | Redis 滑动窗口熔断器 + 多通道 fallback | forge-engine 的多模型熔断复用此模式 |
| 调度模式 | 中心化 TaskDispatcher → Channel Adapter | forge-engine 的 Orchestrator → Worker 复用此模式 |
| DevOps | Docker 多阶段构建 + 多环境配置 + 健康检查 | forge-pipeline 模板基础 |
| 文档体系 | docs/guides/ 标准化文档 + 代码同步更新原则 | forge-specs 文档治理模式 |

---

## 3. 整体架构

### 3.1 架构选型：中心化 Agent 架构 + 高可用保障

选择中心化架构的理由：
1. 第一阶段最快落地
2. 与 Aegis 调度模式一致，模式复用
3. 风险分级天然支持——中央大脑决定自动还是人工
4. 后续可演进为混合架构

### 3.2 架构图

#### Phase 1 简化架构

Phase 1 聚焦最小闭环，仅包含核心链路：

```
┌──────────────────────────────────────────────────┐
│               forge-portal (简易 Web)              │
│          需求输入 + 任务进度（SSE 推送）             │
└─────────────────────┬────────────────────────────┘
                      │ HTTP
                      ▼
┌──────────────────────────────────────────────────┐
│                    APISIX                         │
│           统一 API 网关 / 路由 / 鉴权              │
└──────┬──────────────┬────────────────────────────┘
       │              │
       ▼              ▼
┌────────────┐  ┌────────────┐
│forge-engine│  │  forge-    │
│  core +    │  │  identity  │
│  worker    │  │   (轻量)    │
└─────┬──────┘  └────────────┘
      │
      ├──────────────┬──────────────┐
      ▼              ▼              ▼
┌──────────┐  ┌──────────┐  ┌──────────┐
│  Codeup  │  │  云效     │  │ 阿里云   │
│  代码托管 │  │  流水线   │  │ ACK     │
└──────────┘  └──────────┘  └──────────┘
```

#### 目标架构（Phase 4）

```
┌─────────────────── 接入层 ───────────────────┐
│  forge-portal(Web)  forge-bot(IM)  CLI(本地) │
└──────────────┬───────────────────┬────────────┘
               │ HTTP / WebSocket  │
┌──────────────▼───────────────────▼────────────┐
│              APISIX (统一 API 网关)             │
│   路由 / 鉴权 / 限流 / 灰度 / 负载均衡         │
└──────────────┬────────────────────────────────┘
               │
┌──────────────▼────────────────────────────────┐
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
┌──────────┐ ┌──────────┐ ┌──────────┐
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
| 网关高可用 | APISIX 多副本部署 + etcd 集群存储路由配置；beacon 无状态多实例，Socket.IO Redis Adapter 跨实例广播 | APISIX HA 方案 + Socket.IO 官方方案 |
| 脑裂防护 | K8s Lease 对象自带 TTL 续约机制，续约失败主动降级，新 Leader 等旧 lease 过期后接管；备选方案：Nacos 选举 | K8s Lease API、Nacos |
| 级联故障 | 服务间调用设置超时 + 熔断 + 舱壁隔离（Bulkhead），防止单个下游拖垮整体 | Resilience4j、Sentinel |
| 数据库故障 | 核心表读写分离，写入走主库，查询走从库；引擎关键状态同时写 Redis 保证短期可用 | MySQL 主从 + Redis 缓存兜底 |
| Codeup API 限流 | 缓存 + 节流保护，避免频繁调用 API；文件内容缓存到 Redis，仅在 Webhook 通知变更时刷新 | 标准 API 限流防护模式 |

### 3.4 核心状态机（任务生命周期）

```
SUBMITTED → ANALYZING → PLANNING → GENERATING → REVIEWING → TESTING → DEPLOYING → DONE
                                                    │                      │
                                               HUMAN_REVIEW            ROLLBACK
                                                    │
                                              APPROVED / REJECTED
```

- 每个状态转换持久化到 DB + 发送事件到 beacon
- 任何环节失败进入 FAILED，可从失败点重试（最多 3 轮，超限升级人工）
- HUMAN_REVIEW 是风险分级关键节点——低风险跳过，高风险卡住等人审批

---

## 4. 项目拆分

### 4.1 项目清单

| # | 项目名 | 定位 | 技术栈 |
|---|--------|------|--------|
| 1 | **forge-foundation** | AI 孵化产品加速组件库（可选，非平台基座） | Java 17 + SB 3.2 |
| 2 | **forge-identity** | 统一身份认证 + 动态鉴权中心 | Java 17, Spring Boot 3.x（独立 Spring Boot，不依赖 foundation） |
| 3 | **forge-engine** | AI 引擎核心（中央大脑） | Java 17, Spring Boot 3.x（独立 Spring Boot，不依赖 foundation） |
| 4 | **forge-pipeline** | DevOps 自动化 | Java 17, Spring Boot 3.x（独立 Spring Boot，不依赖 foundation） |
| 5 | **forge-portal** | Web 工作台 + 代码可视化 | Vue 3 + TypeScript |
| 6 | **forge-bot** | IM 机器人（钉钉/飞书） | Java 17（统一技术栈，钉钉/飞书 SDK 均有 Java 版本） |
| 7 | **forge-beacon** | 实时交互网关 | Node.js, Socket.IO |
| 8 | **forge-specs** | 规范中心 | Markdown + JSON + Java 服务 |

### 4.2 项目依赖关系

```
Forge 平台自身服务（独立 Spring Boot，互不依赖 foundation）：

forge-identity (鉴权，被所有需要认证的服务依赖)
       │
       ├── forge-engine (核心引擎)
       │       │
       │       ├── forge-pipeline (DevOps，被 engine 调用)
       │       │
       │       └── forge-specs (规范，被 engine 消费)
       │
       ├── forge-portal (Web 前端，调用 engine + identity API)
       │
       ├── forge-bot (IM 机器人，调用 engine API)
       │
       └── forge-beacon (实时网关，对接 identity 鉴权)

AI 孵化产品（可选依赖 foundation）：

forge-foundation (加速组件库，供 AI 孵化的业务系统使用)
       │
       └── AI 孵化的业务系统（优惠券系统、CRM、用户管理等）
```

关键说明：forge-foundation 是为 AI 孵化出的业务系统提供的加速组件库，Forge 平台自身的服务（engine/identity/pipeline）不依赖 foundation，而是各自作为独立的 Spring Boot 应用运行。

---

## 5. 各项目详细设计

### 5.1 forge-foundation — AI 孵化产品加速组件库

#### 5.1.1 设计目标与定位

forge-foundation 是一个**可选的加速组件库**，目标消费者是 AI 孵化出来的业务系统（优惠券系统、CRM、用户管理系统等），而非 Forge 平台自身。其核心价值是让 AI 生成新项目时不必从零搭建基础设施，而是复用经过验证的中间件封装和通用业务模块，避免重复造轮子。

Phase 1 不包含 foundation，Phase 2-3 作为"加速包"引入。

从 solar-foundation 演进而来，升级方向：
- 脚手架模式复用 solar-foundation 的两层结构（parent 模块 + starter jar）
- META-INF/spring.factories 自动配置
- 条件激活注解（按类是否存在、按属性开关）
- 一键启用注解
- 升级到 JDK 17 + Spring Boot 3.2，配置中心从 Apollo 迁移到 Nacos，指标采集从 CAT 迁移到 Micrometer + Prometheus

#### 5.1.2 两层架构

**技术基础设施层**：中间件封装，让 AI 生成的项目无需关心中间件接入细节

| 组件 | 说明 |
|------|------|
| Redis 组件 | Redisson + Spring Data Redis 封装 |
| 消息队列组件 | 统一抽象层，RocketMQ / Kafka 可切换 |
| 数据库组件 | MyBatis-Plus + 多数据源 + Flyway |
| 日志组件 | 统一日志格式 + Logstash 对接 |
| OSS 组件 | 阿里云 OSS / MinIO 双支持 |
| 指标采集组件 | Micrometer + Prometheus |
| 多级缓存组件 | Caffeine L1 + Redis L2 |
| 分布式 ID 组件 | 雪花算法 + Leaf segment |
| 限流组件 | Redis 滑动窗口 + Sentinel |
| 操作审计组件 | 注解驱动的操作日志记录 |
| 国际化组件 | 消息码 + 多语言资源管理 |

**业务能力层**（核心价值 — 避免重复造轮子）：AI 孵化新产品时常见的通用业务模块

| 模块 | 说明 |
|------|------|
| 登录认证模块 | 账号密码、手机验证码、第三方扫码登录 |
| 会员体系模块 | 用户等级、积分、权益管理 |
| 支付模块 | 支付宝/微信支付对接封装 |
| 订单模块 | 通用订单模型、状态机、超时处理 |
| 通知模块 | 短信/邮件/站内信/Push 多通道通知 |
| 权限控制模块 | RBAC + 数据权限（复用 permission 核心） |
| 灰度标记模块 | 读取 APISIX 传递的灰度标签，支撑服务内灰度逻辑 |

#### 5.1.3 多版本兼容策略（Phase 4+）

Phase 1-3 仅支持 JDK 17 + Spring Boot 3.2。Phase 4 按实际需求逐步补充 JDK 8/11/21 版本。多版本兼容的挑战包括 javax 到 jakarta 命名空间迁移、Spring Cloud 版本差异、组件 API 差异等，通过 BOM 版本锁定 + SPI 适配器模式 + CI 矩阵验证解决。

#### 5.1.4 相比 solar-foundation 的补充

| 新增能力 | 说明 | solar 缺失原因 |
|----------|------|---------------|
| 多级缓存 | Caffeine L1 + Redis L2，热数据命中率更高 | solar 只有单层 Redis |
| 分布式 ID | 雪花算法 + Leaf segment，可配置切换 | solar 依赖数据库自增 |
| 限流组件 | Redis 滑动窗口 + Sentinel 降级 | solar 无内置限流 |
| 操作审计 | 注解驱动的操作日志记录 | solar 无审计 |
| 消息队列抽象 | 统一 API，RocketMQ / Kafka 可切换 | solar 只支持 RocketMQ |
| OSS 组件 | 阿里云 OSS / MinIO 双支持 | solar 无 OSS |
| 指标采集 | Micrometer + Prometheus | solar 用阿里内部 metrics |
| 国际化 | 消息码 + 多语言资源管理 | solar 无 i18n |
| 业务能力层 | 登录/会员/支付/订单/通知等通用模块 | solar 无业务层封装 |

---

### 5.2 forge-identity — 统一动态鉴权中心

#### 5.2.1 设计目标

一个系统动态控制多种鉴权方式。鉴权方式可插拔，权限规则动态可配，不改代码就能切换和组合。独立 Spring Boot 应用，不依赖 forge-foundation。

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

鉴权链配置存储在数据库中，运行时可热更新（缓存 Redis，变更事件广播）。每种接入方式（portal、api、git、bot、cli）有独立的鉴权链配置，包括支持的鉴权方法列表、MFA 策略、会话超时、限流规则等。

主要鉴权链：
- **portal**：支持钉钉扫码、密码登录、OAuth2/OIDC，MFA 可选（敏感操作时触发），会话有效期 8 小时
- **api**：支持 API Token、AK/SK 签名，启用限流和动态 IP 白名单
- **git**：支持 SSH Key、Personal Access Token，按仓库读/写/管理分权
- **bot**：支持钉钉/飞书回调验签，要求用户绑定
- **cli**：支持 OAuth2 Device Flow、Personal Access Token

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

鉴权器采用 SPI 插件化设计，每个鉴权器实现统一的认证接口，声明自身的类型标识（如 oauth2、dingtalk_scan、api_token）、请求匹配条件、认证执行逻辑和责任链排序。

运行时根据数据库配置动态加载启用的插件，新增鉴权方式只需实现接口并注册到数据库，无需修改框架代码。

---

### 5.3 forge-engine — AI 引擎核心

#### 5.3.1 模块结构

engine-core 和 engine-worker 是**独立部署**的服务。core 负责编排调度（有状态，Leader 选举），worker 负责实际 AI 调用（无状态，可水平扩缩）。Worker 是单一服务，通过 Kafka 消息头中的任务类型字段路由到不同的处理器（CodeGen/Review/TestGen 等），而非每种类型单独部署。详见第 12 章。

engine 各模块：
- **engine-api**：对外 API 定义（DTO、接口契约、Kafka 消息格式）
- **engine-core**：编排服务 — 状态机 + 风险评估 + 任务调度，含 Spring Boot 启动入口
- **engine-model**：多模型路由 + 熔断 + 适配器（共享模块）
- **engine-context**：上下文构建器 + 代码索引 + RAG（共享模块）
- **engine-worker**：执行服务 — CodeGen/Review/TestGen/Deploy 处理器，含 Spring Boot 启动入口

#### 5.3.2 多模型路由

模型路由器根据任务类型选择最优模型，同时内置熔断和降级机制：

| 任务类型 | 首选模型 |
|---------|---------|
| 需求分析 / 架构设计 | Claude Opus |
| 代码生成（复杂） | Claude Sonnet |
| 代码生成（简单） | 通义灵码 |
| Code Review | Claude Opus |
| 测试用例生成 | Claude Sonnet |
| 文档 / 注释生成 | Claude Haiku |
| 代码补全 / 小修改 | Copilot / 通义 |

**熔断机制**：每个模型独立熔断统计，成功率低于 80% 触发 OPEN 状态，P99 延迟超过 30 秒标记为慢，限流响应不计入失败。Fallback 链路：Claude Opus → Claude Sonnet → GPT-4 → 通义 → 排队等待。

#### 5.3.3 模型适配器设计

统一的模型调用接口，支持同步生成和流式生成两种模式，以及健康检查。各模型（Claude、GPT、通义、Copilot）各自实现适配器。路由规则存储在数据库中并热缓存到 Redis，新增模型只需实现适配器接口并配置路由规则。

#### 5.3.4 上下文构建器（Context Builder）

AI 生成高质量代码的关键在于给它足够精准的上下文。Context Builder 完全通过 Codeup API 远程获取代码，不做本地克隆，保持 Worker 完全无状态。

**静态上下文**（每个项目固定的）：
- 编码规范 — 来自 forge-specs 规范库
- 项目脚手架结构 — 来自 forge-specs 模板库
- Review 规则 — 来自 forge-specs 规则库
- Prompt 模板 — 来自 forge-specs 提示词库

**动态上下文**（每次任务通过 Codeup API 动态加载）：
- 相关代码文件 — 通过 Codeup 的 ListRepositoryTree 确定项目结构，通过 GetFileBlobs 获取具体文件内容，基于需求分析确定需要读哪些文件
- 数据库 Schema — 项目 Flyway 迁移文件（通过 GetFileBlobs 获取）
- API 契约 — 已有 Controller/DTO 定义（通过 GetFileBlobs 获取）
- 最近变更 — Git log（避免冲突）
- 用户对话历史 — 会话存储（连续交互时）

**上下文优化策略**：
- Token 预算管理 — 每个模型有上下文长度限制，按优先级裁剪
- 代码摘要 — 大文件只送关键签名+注释
- 增量上下文 — 迭代修改时只送 diff + 周边
- RAG 检索 — 规范库太大时，向量检索相关段落（使用 Elasticsearch 8.x kNN 向量搜索）

#### 5.3.5 风险评估器（Risk Evaluator）— 两阶段评估

风险评估分两个阶段进行，最终风险等级取两阶段的最大值：

**第一阶段：PLANNING 阶段（初步评估）**

在需求解析完成后，基于需求类型和预估影响范围做初步评估：
- 评估维度：需求类型（新建项目 / 功能迭代 / 缺陷修复）、预估影响模块数、是否涉及安全/支付等敏感模块
- 输出：初步风险等级（LOW / HIGH）

**第二阶段：REVIEWING 阶段（最终评估）**

在代码生成和 AI 审查完成后，基于实际变更做最终评估：
- 评估维度：实际变更文件数和范围、AI Review 评分、是否涉及安全模块、是否有数据库 Schema 变更、是否修改共享组件

**低风险判定条件**（全部满足则为低风险，否则为高风险）：
- 需求类型为纯配置修改、文档变更、测试代码、新增独立接口、代码风格修复之一
- 影响文件数不超过 5 个
- AI Review 评分 90 分以上
- 不涉及核心业务逻辑修改、数据库 Schema 变更、安全/支付模块、共享组件、基础设施变更

**高风险判定条件**（任一满足则为高风险）：
- 修改核心业务逻辑
- 数据库 Schema 变更
- 涉及权限/支付/安全模块
- 修改共享组件（影响多个服务）
- 基础设施变更
- 影响文件数超过 10 个
- AI Review 评分低于 90
- AI Review 存在告警

#### 5.3.6 端到端流程（含并行代码生成协调）

```
PM: "给优惠券系统加一个按用户等级发放的功能"

① ANALYZING — 需求解析 (Claude Opus)
   · 理解意图，拆解为技术任务清单
   · 输出：需改哪些模块、新增哪些接口、数据库变更
   · 反馈给 PM：确认理解是否正确（通过 beacon 实时推送）

② PLANNING — 方案规划 (Claude Opus)
   · 通过 Codeup API 加载项目上下文（代码 + Schema + 规范）
   · 生成实施方案（修改哪些文件、新增哪些类）
   · 第一阶段风险评估（基于需求类型和预估影响范围）

③ GENERATING — 代码生成 (Worker Pool，三阶段协调)

   Phase A（串行）：接口契约生成
   · AI 先生成接口契约：DTO 定义、API 签名、DB Schema
   · 契约作为后续并行生成的基准

   Phase B（并行）：基于契约的实现生成
   · Worker A：SQL 迁移文件 (Claude Sonnet)
   · Worker B：Java 后端代码 (Claude Sonnet)
   · Worker C：前端页面 (Claude Sonnet)
   · Worker D：单元测试 (Claude Sonnet)

   Phase C（串行）：集成验证
   · 检查跨文件一致性（接口签名匹配、引用正确）
   · 所有文件通过 Codeup CreateCommitWithMultipleFiles API 原子提交到 ai/feature-xxx 分支

④ REVIEWING — AI 审查 (Claude Opus)
   · 编码规范合规检查（对照 forge-specs 规则）
   · 安全扫描（OWASP 规则 + SQL 注入 + XSS）
   · 逻辑一致性（生成的代码是否自洽、接口是否匹配）
   · 输出：评分 + 问题列表 + 修复建议
   · 第二阶段风险评估（基于实际文件变更、Review 评分、安全模块涉及情况）
   · 最终风险 = max(第一阶段, 第二阶段)
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

forge-pipeline 作为独立 Spring Boot 服务部署，主要模块包括：
- **pipeline-api**：对外 API（触发流水线、查询状态）
- **pipeline-template**：流水线模板引擎，根据项目类型自动生成云效流水线配置
- **pipeline-quality**：质量门禁规则引擎
- **pipeline-deployer**：ACK 部署编排
- **pipeline-env**：临时环境管理（创建/销毁）
- **pipeline-config**：配置自动发布模块（AI 生成配置 → 校验 → 发布 Nacos）
- **pipeline-server**：Spring Boot 启动入口

#### 5.4.2 流水线模板引擎

根据项目类型自动生成云效流水线配置，支持的项目类型包括：

- **Java 微服务**：编译打包 → 单元测试 + 覆盖率报告 → 代码扫描（SonarQube + 依赖检查）→ Docker 镜像构建推送到 ACR → 部署到 ACK
- **Vue 前端**：依赖安装 → 构建 → Nginx 镜像构建推送 → 部署到 ACK
- **SDK 类库**：编译打包 → 单元测试 → 发布到私有 Nexus

#### 5.4.3 环境管理

**临时环境**（AI 分支预览）：
- AI 推送 ai/feature-xxx 分支后自动创建临时 K8s namespace
- namespace 命名规则：forge-preview-{task-id}
- 包含独立的 DB（schema clone）+ Redis + 服务实例
- MR 合并后 30 分钟自动销毁
- 资源配额限制（防止临时环境耗尽集群资源）

**固定环境**：
- dev — develop 分支自动部署
- staging — release 分支自动部署
- prod — master 分支，人工审批后灰度部署

#### 5.4.4 质量门禁

| 门禁项 | 要求 |
|--------|------|
| 编译 | 必须通过 |
| 单测覆盖率 | 不低于配置阈值（默认 60%，可按项目调整） |
| AI Review | 必须通过（来自 forge-engine） |
| 安全扫描 | 无高危漏洞（SonarQube + OWASP Dependency Check） |
| 镜像漏洞扫描 | 无 Critical 级别漏洞（Trivy） |
| API 兼容性 | 不允许破坏性变更（接口删除/改签名） |

任一不通过则阻断部署并通知 forge-engine AI 修复。

#### 5.4.5 灰度部署（APISIX 网关层）

灰度发布完全通过 APISIX 网关层实现，继承 kohinur 的三层决策模型但零侵入业务服务：

**三层决策优先级**：

| 优先级 | 层级 | APISIX 实现方式 |
|--------|------|----------------|
| 最高 | 环境分区 | APISIX 路由级别隔离，不同环境完全独立路由 |
| 中等 | 规则匹配 | traffic-split 插件的 match.vars 条件，按用户标签/租户/地域等维度匹配 |
| 最低 | 比例分流 | weighted_upstreams 权重分配，按百分比切流 |

**核心优势**：
- 集中在网关层管控，业务服务零代码侵入
- 通过 APISIX Admin API 即时变更灰度规则，无需重启服务
- 灰度管理 UI 集成在 forge-portal 中，通过 APISIX Admin API 操作

**生产部署策略**（可配置）：
- 金丝雀发布 — 先 5% 流量，观察 10 分钟，无异常逐步放量
- 蓝绿部署 — 新版本完全就绪后切换流量
- 灰度发布 — 按用户标签/租户/地域分流

**回滚策略**：
- 健康检查失败 → 自动回滚
- 错误率突增（大于 5%）→ 自动回滚 + 告警
- 人工触发 → 一键回滚到上一版本

#### 5.4.6 配置自动发布

AI 生成的业务系统配置文件需要遵循标准并自动发布到 Nacos：

配置生命周期：AI 生成配置 → 校验是否符合配置规范 → 自动发布到 Nacos → 服务热加载生效

校验规则包括：命名规范检查、敏感字段检测（不允许明文密码）、格式正确性验证、与项目已有配置的兼容性检查。

---

### 5.5 forge-portal — Web 工作台 + 代码可视化

#### 5.5.1 技术选型

| 维度 | 选型 |
|------|------|
| 框架 | Vue 3 + TypeScript + Vite |
| UI 库 | Ant Design Vue 4.x |
| 状态管理 | Pinia |
| 路由 | Vue Router 4 |
| 代码编辑器 | Monaco Editor（VS Code 同款） |
| Diff 展示 | monaco-diff-editor |
| 语法高亮 | Shiki |
| 图表 | ECharts |
| 实时通信 | Socket.IO Client（对接 forge-beacon） |

#### 5.5.2 页面结构

| 页面 | 功能 | 数据源 |
|------|------|--------|
| **需求工作台** | 自然语言对话式输入需求，AI 实时回应澄清问题，确认后提交 | forge-engine API + beacon 流式输出 |
| **任务看板** | Kanban 视图展示所有 AI 任务状态，支持筛选/搜索/批量操作 | forge-engine API + beacon 实时更新 |
| **代码浏览器** | 文件树 + Monaco Editor 语法高亮 + 在线查看 | Codeup Git API |
| **AI Diff 预览** | AI 生成的代码变更逐行展示，每段附带 AI 解释（为什么改、改了什么） | forge-engine 输出 |
| **MR 审批** | AI Review 报告 + 风险评分 + 人工审批按钮 + 评论批注 | Codeup MR API + forge-engine |
| **部署看板** | 环境状态总览、流水线进度、灰度比例（APISIX 配置可视化）、一键回滚 | forge-pipeline API + APISIX Admin API |
| **项目管理** | 创建新项目（选脚手架模板 + 填配置）、成员权限管理 | forge-identity + forge-specs |
| **监控大盘** | AI 任务成功率、模型用量、代码生成量、部署频率 | forge-engine metrics |
| **系统设置** | 鉴权方式配置、AI 模型配置、风险规则配置、通知设置、紧急停止开关管理 | forge-identity + forge-engine |
| **灰度管理** | 灰度规则配置、流量比例调整、灰度状态监控 | APISIX Admin API |

#### 5.5.3 代码可视化（Codeup 加壳）

不做（Codeup 已有）的能力：Git 存储引擎、代码搜索、CI 构建执行器、Webhook 管理。

做（体验增强）的能力：
- AI Diff 智能注释（每段变更附解释）
- 风险标注（高亮可能有问题的代码行）
- AI 对话式 Code Review（在代码行上提问）
- 分支可视化（AI 分支自动命名 ai/feature-xxx）
- 变更影响分析（这次改动影响哪些服务/接口）

---

### 5.6 forge-bot — IM 机器人

#### 5.6.1 技术栈与支持平台

forge-bot 使用 Java 17 实现，与平台其他后端服务统一技术栈。钉钉和飞书 SDK 均有成熟的 Java 版本，无需引入额外技术栈。

| 平台 | 接入方式 | 交互形式 |
|------|---------|---------|
| 钉钉 | 企业内部机器人 + Webhook | 群聊 @forge / 私聊 |
| 飞书 | 自建应用 + 事件订阅 | 群聊 @forge / 私聊 |

#### 5.6.2 交互设计

用户在 IM 中 @forge 提出需求后，机器人的交互流程：
1. 收到需求后，AI 返回结构化的理解确认卡片，列出拆解的任务项，提供"确认/修改/取消"按钮
2. 用户确认后，实时更新任务进度卡片，展示每个步骤的完成状态
3. 任务完成后，推送结果卡片，包含 AI Review 评分、风险等级、合并和部署状态，并提供"查看代码/查看部署/提生产"快捷按钮
4. 紧急停止开关也可通过钉钉/飞书触发（L1 级别）

---

### 5.7 forge-beacon — 实时交互网关

#### 5.7.1 模块结构

- **beacon-server**：网关服务，包含连接管理（Socket.IO 连接池 + 心跳）、鉴权模块（对接 forge-identity）、namespace 路由（按业务隔离）、消息分发（有状态绑定用户 + 无状态广播）、Redis Adapter（跨实例消息广播）
- **beacon-client-java**：Java SDK（Spring Boot Starter），提供注解式事件监听、消息发送模板、自动重连 + ACK 确认、断线消息缓冲
- **beacon-client-node**：Node.js SDK，同 Java SDK 能力
- **beacon-common**：共享定义（消息类型、协议）

#### 5.7.2 Namespace 隔离

| Namespace | 用途 |
|-----------|------|
| /forge-portal | Web 工作台的实时推送（任务进度、审批通知） |
| /forge-bot | IM 消息桥接（钉钉/飞书消息透传） |
| /forge-engine | AI 流式输出（代码生成实时展示） |
| /forge-pipeline | 部署进度推送（构建/推送/上线状态） |

#### 5.7.3 有状态 vs 无状态连接

**有状态连接**（绑定用户会话）：
- AI 对话 — 需要保持会话上下文，同一用户的消息路由到同一 Worker
- 管理方式：userId → connectionId 映射存 Redis Hash
- 断线重连：30 秒内重连恢复会话，超时释放

**无状态连接**（订阅模式）：
- 任务进度广播 — 订阅 task:{taskId} 频道
- 系统通知 — 订阅 system:alert 频道
- 管理方式：Redis Pub/Sub + Socket.IO Room

#### 5.7.4 消息类型

| 类型 | 方向 | 说明 |
|------|------|------|
| STREAM_OUTPUT | server → client | AI 生成代码的流式文本 |
| TASK_PROGRESS | server → client | 任务状态机变更 |
| REVIEW_RESULT | server → client | Review 结果通知 |
| DEPLOY_STATUS | server → client | 部署进度 |
| APPROVAL_REQ | server → client | 需要人工审批的请求 |
| SYSTEM_ALERT | server → client | 系统告警 |
| KILL_SWITCH | server → client | 紧急停止通知 |
| USER_INPUT | client → server | 用户输入（对话、确认、取消） |
| HEARTBEAT | 双向 | 心跳保活 |

---

### 5.8 forge-specs — 规范中心

#### 5.8.1 Prompt 模板体系

Prompt 模板是 AI 代码生成质量的核心保障。所有 prompt 必须在开发前完成设计、评审和 eval 测试。

**Prompt 分层结构**

每个 prompt 调用由四层组成，从上到下叠加：

| 层级 | 内容 | 变化频率 |
|------|------|---------|
| System Prompt（固定层） | AI 角色定义、行为约束、输出格式要求 | 极少变化，版本化管理 |
| Standards Injection（项目层） | 项目对应的编码规范、技术栈约束、命名规则 | 按项目不同注入不同规范集 |
| Context（任务层） | 相关代码文件、DB Schema、API 契约、变更历史 | 每次任务动态构建 |
| User Input（用户层） | 用户的需求描述、澄清回复、修改要求 | 每次交互变化 |

**6 个核心 Prompt 模板的职责边界**

| Prompt 模板 | 触发阶段 | 输入 | 输出 |
|-------------|---------|------|------|
| requirement-analysis | ANALYZING | 用户需求原文 + 项目上下文（已有模块/接口/Schema 概览） | 结构化任务清单（需修改的模块、新增的接口、DB 变更）+ 向用户的澄清问题列表 |
| code-generation | GENERATING | 任务清单 + 相关代码文件 + 编码规范 + 脚手架模板 | 可直接提交的代码文件集合（含文件路径和完整内容） |
| code-review | REVIEWING | 生成的代码 + 编码规范 + Review 规则库 | 评分（0-100）+ 问题列表（每项含位置、严重度、描述）+ 修复建议 |
| test-generation | GENERATING | 业务代码 + 测试规范 | 单元测试代码文件 |
| fix-generation | GENERATING（重试） | Review 反馈的问题列表 + 原始代码 + 编码规范 | 修复后的代码文件 |
| doc-generation | DONE 之后 | 代码文件 + API 定义 | API 文档 + 变更日志 |

**Prompt 质量保障**

每个 prompt 模板必须配套：
- bad-code-samples：故意违规的代码样本，用于验证 AI 能否识别问题
- good-code-samples：标准的代码样本，用于验证 AI 生成质量
- eval 测试用例：自动化的质量评估脚本
- 版本基线：每次修改 prompt 后与上一版本做 A/B 对比，确保质量不退化

#### 5.8.2 编码规范基线

以 aegis 项目的 shulex-coding-standards.md（503 行）和 project-structure.md（686 行）为基线，核心规范要点：

| 规范维度 | 要求 |
|---------|------|
| 技术栈 | Java 17 + Spring Boot 3.2 + MyBatis-Plus |
| 领域模型命名 | DO（数据库映射）、DTO（传输对象）、VO（视图对象）、BO（业务对象）严格区分 |
| 响应封装 | 统一使用 Result<T> 包装所有 API 返回值 |
| 异常体系 | 分层异常：BizException（业务异常）/ SysException（系统异常）/ 领域异常 |
| 错误码 | 集中式 ErrorCode 枚举，禁止散落硬编码 |
| 依赖注入 | 使用 @RequiredArgsConstructor 构造器注入，禁止 @Autowired 字段注入 |

#### 5.8.3 规范文件结构

- **standards/**：编码规范文件（Java 编码规范、SQL 规范、Redis 规范、Kafka 规范、API 设计规范、安全编码规范、命名规范、Git 工作流规范）
- **templates/**：项目脚手架模板（Java 微服务骨架、API 网关骨架、Vue 3 前端骨架、Java SDK 骨架）
- **review-rules/**：AI Review 规则库（阿里巴巴规约检查点、安全检查点 OWASP Top 10、性能检查点、数据库规范检查点、API 兼容性检查、团队自定义规则）
- **prompts/**：6 个 AI Prompt 模板
- **specs-service/**：规范服务（Java），提供规范版本管理 API、规范继承解析（公司级 → 团队级 → 项目级）、规范合规检测 API、规范效果度量 API
- **specs-eval/**：规范测试套件（bad-code-samples、good-code-samples、eval 测试脚本）

#### 5.8.4 规范继承机制

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

#### 5.8.5 规范效果度量

持续追踪以下指标：
- AI 生成代码的规范合规率（按规则维度统计）
- AI Review 的准确率（人工复核对比）
- 低风险自动合并的代码线上故障率
- 各 Prompt 模板的生成质量评分

目的：
- 合规率低的规则 → 优化对应的 Prompt 或 Review 规则
- 准确率低的 Review → 调整 AI 模型选择或 Prompt
- 故障率高的自动合并 → 收紧风险评估规则

---

## 6. 生产级分阶段交付路线

### 第一阶段：核心闭环（跑通一个需求从输入到部署）

**目标**：简易 Web 界面输入需求 → AI 生成代码 → 推到 Codeup → 流水线构建部署

**目标用户**：开发者/架构师验证平台能力（PM 上手需等第二阶段体验完善）

**建设项目**：
- forge-specs — 基础规范（aegis 编码规范基线）+ 1 套微服务模板 + 6 个核心 Prompt 模板（含 eval 测试用例）
- forge-engine — 编排器 + 单模型（Claude）+ CodeGen + Review Worker。engine-core 和 engine-worker 独立部署
- forge-pipeline — 1 套 Java 微服务流水线模板 + 基础质量门禁
- forge-identity（轻量）— 服务间认证（Codeup/云效/ACK 的 API Token 管理）+ 基础用户认证（账号密码）
- forge-portal（轻量）— 最小 Web 界面：需求输入 + 任务进度（SSE 推送，不依赖 beacon）

**不包含**：forge-foundation、forge-bot、forge-beacon

**验收标准**：
- 用户在简易 Web 界面输入"创建一个用户管理服务"
- AI 生成完整项目（模块结构 + 数据库 + 接口 + 基础前端）
- 代码通过 Codeup API 原子提交（CreateCommitWithMultipleFiles）
- 流水线自动构建并成功部署到 dev 环境
- Token 消耗有记录，可按任务查询
- 三级紧急停止开关可用

### 第二阶段：用户体验（让小白真正能用）

**目标**：PM 在网页上提需求 → 实时看到 AI 工作进度 → 审批发布

**建设项目**：
- forge-identity — 增加钉钉扫码 + OAuth2 OIDC + RBAC 权限模型
- forge-portal — 完整版：需求工作台 + 任务看板 + AI Diff 预览 + MR 审批 + 代码浏览器，实时推送从 SSE 升级为 WebSocket（仍内嵌 engine，不依赖 beacon）
- forge-foundation（基础版）— 技术基础设施层组件，作为 AI 孵化新项目的加速包引入

**验收标准**：
- PM 用钉钉扫码登录 Portal
- 在需求工作台用自然语言描述需求
- 实时看到 AI 生成进度
- 在 AI Diff 预览页查看代码变更 + AI 解释
- 审批后自动部署
- 有租户级 Token 预算告警
- AI 孵化的新项目可基于 forge-foundation 加速包生成

### 第三阶段：扩展性

**目标**：多入口 + 多模型 + 多项目类型 + 实时网关

**建设项目**：
- forge-bot — 钉钉/飞书机器人入口（Java 17）
- forge-engine — 多模型路由 + 熔断 fallback
- forge-pipeline — 前端项目模板 + 临时环境管理 + APISIX 灰度部署
- forge-beacon — 独立实时网关（当多入口 fan-out 成为真实需求时引入）
- forge-foundation（完整版）— 业务能力层模块（登录/会员/支付/订单/通知）

**验收标准**：
- 钉钉群里 @forge 也能走完全流程
- Claude 不可用时自动切换到 GPT
- 支持孵化 Vue 3 前端项目
- 新创建的项目基于 forge-foundation 脚手架（含业务能力层）

### 第四阶段：成熟化

**目标**：企业级生产全面成熟

**重点**：
- forge-identity 完整的动态鉴权链 + MFA + 全部鉴权插件
- forge-foundation 多版本扩展（按实际需要逐步补 JDK 8/11/21）
- forge-specs 规范继承 + 效果度量
- forge-pipeline APISIX 灰度部署深化 + 自动回滚
- forge-beacon Java SDK（供孵化出的项目集成长连接）
- forge-portal 监控大盘 + 系统设置
- 全链路可观测（Prometheus + Grafana + 链路追踪）

---

## 7. 技术选型总览

| 维度 | 选型 |
|------|------|
| 后端框架 | Java 17 + Spring Boot 3.2（Forge 平台自身服务，各自独立 Spring Boot 应用） |
| 加速组件库 | forge-foundation（Java 17 + SB 3.2，Phase 2+ 引入，供 AI 孵化产品使用） |
| 前端框架 | Vue 3 + TypeScript + Ant Design Vue + Vite |
| API 网关 | APISIX（统一网关，替代 Spring Cloud Gateway，同时处理外部流量和内部微服务调用） |
| 实时通信 | Socket.IO (Node.js server + 多语言 client SDK) |
| AI 模型 | Claude (Anthropic) + GPT (OpenAI) + 通义灵码 (Alibaba) + Copilot (GitHub) |
| 代码托管 | 云效 Codeup（通过 API 操作，不做本地克隆） |
| CI/CD | 云效流水线 |
| 容器编排 | 阿里云 ACK (Kubernetes)，部署通过 K8s 标准 API + 阿里云 CS API |
| 镜像仓库 | 阿里云 ACR |
| 数据库 | MySQL 8.0 |
| 缓存 | Redis 7 (Redisson) |
| 任务队列 | Kafka 作为任务通道（core → worker 派发、worker → core 结果上报），Redis 作为状态缓存（Worker 心跳 TTL、任务状态高频读取） |
| 调度 | XXL-Job |
| 服务发现 | Nacos |
| 配置中心 | Nacos（所有配置和密钥的统一存储） |
| 向量搜索 | Elasticsearch 8.x kNN（复用 ELK 栈，不引入独立向量数据库） |
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
├── shulex-aegis/            # 已有项目（工程实践参考 + 编码规范基线）
│
└── forge/                   # Forge 平台（新建）
    ├── forge-foundation/    # AI 孵化产品加速组件库（Phase 2+）
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

典型任务："给优惠券系统加按用户等级发放功能"

| 步骤 | 模型 | 预估消耗 |
|------|------|---------|
| 需求分析 | Opus | 约 5K input + 2K output = 7K tokens |
| 方案规划 | Opus | 约 20K input + 5K output = 25K tokens |
| 代码生成（4 个 Worker 并行） | Sonnet | 约 30K input + 10K output x 4 = 160K tokens |
| AI Review | Opus | 约 40K input + 5K output = 45K tokens |
| 修复重试（如需要） | Sonnet | 约 20K input + 5K output = 25K tokens |
| **单次任务总计** | | **约 200K ~ 300K tokens** |
| **复杂任务 + 3 轮重试** | | **可达 500K ~ 800K tokens** |

### 9.2 成本控制策略

| 层级 | 机制 | 说明 |
|------|------|------|
| 任务级 | 预估展示 | 任务提交前展示预估 Token 消耗和费用，用户确认后执行 |
| 任务级 | 硬上限 | 单次任务 Token 上限（默认 1M，可配），超限中止并通知 |
| 租户级 | 月度预算 | 每个租户设置月度 Token 预算，80% 软告警，100% 硬限制 |
| 平台级 | 并发控制 | 全局最大并发 AI 任务数（防止突发消耗），队列排队 |
| 优化级 | 上下文裁剪 | Token 预算管理器按优先级裁剪上下文，优先保留规范和核心代码 |
| 优化级 | 缓存复用 | 相同项目结构的上下文缓存（通过 Codeup API 获取后缓存到 Redis），避免重复请求 |
| 优化级 | 模型降级 | 简单子任务自动用轻量模型（Haiku/通义），降低成本 |

### 9.3 Token 用量追踪

每次模型调用记录：任务 ID、步骤名、模型 ID、输入 Token 数、输出 Token 数、费用（美元）、延迟（毫秒）、时间戳。

可视化看板：
- 按任务查看 Token 消耗明细
- 按租户查看月度趋势
- 按模型查看用量分布
- 成本异常告警（单任务超阈值）

---

## 10. 核心数据模型

所有数据表采用共享表 + tenant_id 字段的多租户隔离方式。MyBatis 插件自动在所有查询中注入 tenant_id 条件（复用 hunter-saas-permission 的行级数据权限插件模式），应用层无需手动添加租户过滤条件。未来如有大租户需求，可升级为 schema 级别隔离。

### 10.1 forge-engine 数据模型

**forge_task（孵化任务主表）**

存储每个 AI 孵化任务的全生命周期信息。核心字段包括：任务唯一 ID、租户 ID、提交用户 ID、需求标题和描述（用户原文）、任务类型（新建项目/功能迭代/缺陷修复）、目标项目 ID（迭代时必填）、状态机状态、初始风险等级和评分、最终风险等级和评分（两阶段评估）、累计 Token 消耗和费用、重试次数、错误信息、创建和修改时间。主要索引：任务 ID 唯一索引、租户+状态联合索引、用户 ID 索引。

**forge_task_step（任务步骤表）**

记录任务的每个执行步骤，用于 checkpoint 恢复。核心字段：任务 ID、步骤名（ANALYZING/PLANNING/GENERATING 等）、步骤序号、步骤状态（PENDING/RUNNING/SUCCESS/FAILED/SKIPPED）、执行此步骤的 Worker 实例 ID、步骤输入快照（上下文摘要，JSON 格式）、步骤输出快照（生成结果摘要，JSON 格式）、消耗 Token 数、开始和完成时间、错误信息。

**forge_model_call_log（AI 模型调用日志）**

追加式日志表，记录每次 AI 模型调用的详细信息。核心字段：任务 ID、步骤名、模型 ID（如 claude-opus/claude-sonnet/gpt-4）、调用目的（需求分析/代码生成/Review 等）、输入和输出 Token 数、费用（美元）、延迟（毫秒）、是否为降级调用、是否成功、错误码。此表只追加不更新。

**forge_code_change（代码变更记录）**

记录 AI 生成的代码变更信息。核心字段：任务 ID、仓库 URL、分支名、提交哈希、变更文件数、新增行数、删除行数、AI Review 评分、Review 摘要、Codeup MR 链接和状态。

### 10.2 forge-identity 数据模型

**forge_tenant（租户表）**

核心字段：租户唯一 ID、租户名称、月度 Token 预算（0 表示不限制）、状态（ACTIVE/SUSPENDED）。

**forge_user（用户表）**

核心字段：租户 ID、用户名（租户内唯一）、显示名、邮箱、手机号、密码哈希、状态、最后登录时间。

**forge_role（角色表）**

核心字段：租户 ID、角色编码（如 PLATFORM_ADMIN/PROJECT_ADMIN/DEVELOPER/PM）、角色名、作用域（PLATFORM/PROJECT）。

**forge_user_role（用户角色绑定表）**

核心字段：用户 ID、角色 ID、项目 ID（项目级角色时必填）。

**forge_auth_config（动态鉴权链配置表）**

核心字段：租户 ID（NULL 表示全局默认配置，非 NULL 表示租户级覆盖）、鉴权链名称（portal/api/git/bot/cli）、鉴权类型（password/dingtalk_scan/oauth2_oidc 等）、排序序号、是否启用、鉴权器特有配置（JSON 格式）。全局配置与租户级覆盖的关系：查询时优先取租户级配置，不存在则回退到全局默认。

---

## 11. Context Builder 详细设计（现有项目迭代）

### 11.1 代码索引策略（基于 Codeup API）

Context Builder 完全通过 Codeup API 远程获取代码，不做本地克隆，保持 Worker 无状态。

**项目首次接入 Forge 时**：
1. 通过 ListRepositoryTree API 获取完整项目结构，生成项目地图（模块列表、包结构、类清单）
2. 通过 GetFileBlobs API 批量获取文件内容
3. 对每个 Java 文件提取签名摘要（类名+方法签名+注释，不含方法体）
4. 对数据库 Schema（Flyway 文件）生成表结构摘要
5. 对 API 层（Controller + DTO）生成接口清单
6. 存储索引到 DB + Elasticsearch（用于 kNN 向量检索）

**增量更新**：
- Codeup Webhook（Push Hook）通知代码推送 → 增量更新受影响文件的索引
- 文件内容缓存到 Redis，仅在 Webhook 通知变更时刷新
- 定时全量刷新（每日凌晨）

### 11.2 上下文组装流程

需求解析完成后，已知需要修改哪些模块：

1. 加载静态上下文：编码规范（forge-specs）、Prompt 模板
2. 精准加载相关代码（基于需求分析的输出，通过 Codeup API 获取）：直接相关文件 → 完整代码；接口依赖文件 → 仅签名摘要；数据库 Schema → 相关表的定义
3. RAG 补充检索：用需求描述在 Elasticsearch 中做 kNN 向量检索，补充可能遗漏的相关代码，检索范围限定在同一模块内
4. Token 预算裁剪：预算 = 模型 context window - 预留输出空间 - 安全余量。超预算时按优先级裁剪：P0 规范（不裁）> P1 直接相关代码 > P2 接口签名 > P3 RAG 补充

### 11.3 处理上限与降级

| 场景 | 策略 |
|------|------|
| 项目代码量 < 50 文件 | 直接通过 Codeup API 全量加载，不需要索引 |
| 项目代码量 50~500 文件 | 签名索引 + 精准加载 + RAG |
| 项目代码量 > 500 文件 | 仅索引 + RAG，直接加载限定为需求相关的模块 |
| 单文件 > 1000 行 | 拆分为签名 + 关键方法体，不完整加载 |
| 上下文仍超限 | 告知用户"需求范围太大"，建议拆分为多个小任务 |

### 11.4 并发冲突处理

AI 修改代码期间，人工也在同一项目开发时的处理策略：

1. AI 通过 Codeup CreateBranch API 创建独立分支（ai/feature-{taskId}），不直接操作 develop
2. AI 提交代码前，先通过 GetFileBlobs 获取目标分支最新状态，确保基于最新代码生成
3. 冲突检测 — 如果目标文件在 AI 分析后被修改，标记冲突文件，AI 尝试自动解决（最多 1 次）
4. 自动解决失败 → 标记任务为 CONFLICT，通知人工处理
5. MR 合并时由 Codeup 做最终冲突检测

原则：AI 永远不会强制覆盖人工代码。冲突时优先保护人工变更。

### 11.5 DB 迁移安全策略

AI 生成的 Flyway 迁移文件要求：
1. 必须同时生成 UP 和 DOWN 迁移（可回滚）
2. 在临时环境先执行 UP → 验证 → 执行 DOWN → 验证（确认可逆）
3. 破坏性操作（DROP TABLE/COLUMN）强制进入 HUMAN_REVIEW
4. 大表变更（ALTER TABLE 涉及超过 100 万行估算）标记风险并建议使用 pt-osc

---

## 12. forge-engine 拆分为 engine-core + engine-worker

### 12.1 职责分离

**engine-core（编排服务，有状态）**：
- 接收需求，创建任务
- 状态机驱动（任务生命周期管理）
- 两阶段风险评估
- 任务持久化（DB + Redis）
- Leader 选举（仅 core 需要）
- 对外 API（Portal/Bot/CLI 调用）

**engine-worker（执行服务，无状态，可水平扩缩）**：
- 单一服务，通过 Kafka 消息头中的任务类型字段路由到不同处理器
- 从 Kafka 任务队列拉取待执行步骤
- 调用 AI 模型（模型路由 + 熔断在 Worker 内）
- 通过 Codeup API 构建上下文（完全无状态，不做本地克隆）
- 代码生成 / Review / 测试生成（不同处理器，同一服务）
- 生成的代码通过 Codeup CreateCommitWithMultipleFiles API 原子提交
- 执行完毕后通过 Kafka 回报 core（状态更新）
- 天然支持 K8s HPA 按队列深度扩缩

### 12.2 通信方式

- **任务派发**：core → worker 通过 Kafka topic (forge.task.step) 派发步骤任务，消息头包含任务类型字段用于路由到对应处理器
- **结果上报**：worker → core 通过 Kafka topic (forge.task.step.result) 回报结果
- **Worker 心跳**：Redis Hash (forge:worker:heartbeat:{workerId})，TTL 30 秒
- **任务状态缓存**：Redis 缓存高频读取的任务状态，减轻数据库压力

---

## 13. 密钥与配置管理

### 13.1 密钥存储策略

所有密钥和项目配置统一存储在 Nacos 加密配置中，不使用独立的 KMS 或 Vault。

| 密钥类型 | 存储位置 | 说明 |
|---------|---------|------|
| AI 模型 API Key（Claude/GPT/通义） | Nacos 加密配置 | 按模型分组管理 |
| Codeup API Token | Nacos 加密配置 | 用于代码读写操作 |
| 云效流水线 Token | Nacos 加密配置 | 用于触发和查询流水线 |
| 数据库密码 | Nacos 加密配置 | 按环境分组 |
| Redis 密码 | Nacos 加密配置 | 按环境分组 |
| ACK/ACR 凭证 | RAM 角色绑定 Pod ServiceAccount | 无密钥方式（keyless），通过 RAM 角色授权 Pod 直接访问阿里云资源 |

### 13.2 配置自动发布生命周期

AI 生成的业务系统配置遵循以下生命周期：

1. AI 根据项目规范生成配置文件
2. forge-pipeline 校验配置是否符合标准（命名规范、敏感字段检测、格式正确性）
3. 校验通过后自动发布到 Nacos 对应的命名空间和分组
4. 目标服务通过 Nacos 监听机制热加载新配置

---

## 14. 三级紧急停止开关

当 AI 生成的代码引发生产问题或发现系统性风险时，提供三级紧急停止机制：

### 14.1 停止级别

| 级别 | 名称 | 影响范围 | 触发方式 | 恢复方式 |
|------|------|---------|---------|---------|
| L1 | 暂停提交 | 阻断新任务提交，已在执行的任务继续完成 | Portal 操作台 / 钉钉命令 / API 调用 | 原路解除即可 |
| L2 | 冻结引擎 | 在执行的任务暂停（保存 checkpoint），阻断代码提交和部署 | Portal 操作台 / API 调用 | 需手动恢复 + 逐任务确认状态后继续 |
| L3 | 全面停机 | 切断 APISIX 路由 + 暂停 Worker Pod + 阻断流水线 | 仅限 API 调用 | 需架构师 + 运维双人确认后逐步恢复 |

### 14.2 自动触发规则

| 触发条件 | 自动执行级别 |
|---------|------------|
| 连续 N 次部署失败且错误率超过 5% | 自动触发 L1（暂停提交） |
| 1 小时内回滚次数超过 3 次 | 自动触发 L2（冻结引擎） |

自动触发后会同时通过 Portal 和钉钉/飞书通知相关人员。

---

## 15. 失败处理与用户体验

原则：**禁止静默失败**，任何环节的失败都必须有明确的用户通知和清晰的交接方案。

### 15.1 失败场景与处理策略

| 失败场景 | 最大重试次数 | 处理策略 | 用户通知 |
|---------|------------|---------|---------|
| AI 代码生成失败 | 3 轮 | AI 自动修复重试，3 轮后放弃 | Portal 卡片展示"AI 无法完成，已转交开发团队"+ 已生成的部分代码 + 失败原因说明 |
| 流水线构建失败 | 3 轮 | AI 先尝试分析错误日志并自动修复，3 轮后放弃 | Portal 卡片 + 错误日志摘要 |
| 部署失败 | 1 次自动回滚 | 自动回滚到上一版本 | Portal 卡片 + 钉钉/飞书通知"部署失败，已自动回滚"+ 部署日志 |
| Codeup API 调用失败 | 3 次 | 指数退避重试 | 超过重试次数后通知用户任务暂停，等待 API 恢复 |

### 15.2 失败信息透出

每次失败通知必须包含：
- 失败发生的具体环节（分析/生成/审查/测试/部署）
- 失败原因的可理解描述（面向非技术用户）
- 已完成的工作成果（部分代码、部分测试等）
- 下一步建议（等待修复/联系开发/拆分需求）

---

## 16. Codeup API 依赖清单

Forge 平台通过 Codeup API 完成所有代码操作，不做本地克隆。以下是核心依赖的 API 清单：

### 16.1 代码读取类

| API | 用途 | 调用方 |
|-----|------|--------|
| ListRepositoryTree | 获取仓库目录树结构，用于项目结构分析 | Context Builder |
| GetFileBlobs | 获取文件内容，用于加载相关代码到上下文 | Context Builder |

### 16.2 代码写入类

| API | 用途 | 调用方 |
|-----|------|--------|
| CreateBranch | 创建 AI 工作分支（ai/feature-{taskId}） | engine-worker |
| CreateCommitWithMultipleFiles | 原子提交多个文件（核心 API，保证生成代码的完整性） | engine-worker |

### 16.3 合并请求类

| API | 用途 | 调用方 |
|-----|------|--------|
| CreateMergeRequest | 创建合并请求，关联 AI 任务 | engine-core |
| MergeMergeRequest | 执行合并（低风险自动合并） | engine-core |

### 16.4 Webhook 事件

| 事件类型 | 用途 |
|---------|------|
| Push Hook | 代码推送事件，触发索引增量更新 |
| Tag Push Hook | 标签创建/删除事件 |
| Note Hook | 评论事件，用于接收人工 Review 反馈 |
| Merge Request Hook | MR 生命周期事件（创建、更新、审批、合并、关闭、重新打开） |

Webhook 安全：支持 Secret Token 验证。Codeup 出站 IP 白名单：47.98.116.130、47.111.186.29。

### 16.5 风险：API 限流

Codeup API 存在调用频率限制。应对策略：
- 文件内容读取结果缓存到 Redis，设置合理过期时间
- Webhook 通知变更时才刷新缓存
- API 调用端实现节流保护（令牌桶限流）
- 批量操作尽量使用 CreateCommitWithMultipleFiles 减少调用次数

---

## 17. ACK 部署架构

### 17.1 Forge 平台部署

Forge 平台自身部署在阿里云 ACK（托管 Kubernetes），所有服务容器化运行。操作 ACK 分为两层 API：

**K8s 标准 API**（通过 KubeConfig + 客户端证书认证）：
- Deployment：各服务的部署描述（engine-core、engine-worker、identity、pipeline、portal、bot、beacon）
- Service：服务发现和负载均衡
- Ingress：外部流量入口（APISIX 作为 Ingress Controller）
- ConfigMap / Secret：非敏感配置和敏感凭证
- Namespace：环境隔离（dev/staging/prod + 临时预览环境）
- Pod / ReplicaSet：扩缩容、滚动更新、日志查询
- 认证方式：从 ACK 控制台获取 KubeConfig，提取客户端证书用于 API 调用；也可使用 K8s 官方 Java SDK（io.kubernetes:client-java）

**阿里云 CS API**（通过 RAM 用户 AccessKey + SDK 2.0 认证）：
- 集群管理：CreateCluster / DeleteCluster / ModifyCluster / UpgradeCluster / DescribeClusterDetail
- 节点池管理：CreateClusterNodePool / ScaleClusterNodePool / ModifyClusterNodePool / RemoveNodePoolNodes
- KubeConfig 凭证管理：DescribeClusterUserKubeconfig / RevokeK8sClusterKubeConfig / UpdateK8sClusterUserConfigExpire
- 组件管理：InstallClusterAddons / UpgradeClusterAddons（如安装 APISIX Ingress Controller）
- 安全与巡检：ScanClusterVuls / RunClusterCheck / CreateClusterDiagnosis
- 编排模板：CreateTemplate / DescribeTemplates（K8s YAML 模板管理）
- 触发器：CreateTrigger / DescribeTrigger（Pod 重部署触发）
- 权限管理：GrantPermissions / UpdateUserPermissions（RAM 用户集群 RBAC 授权）

**forge-pipeline 对 ACK 的使用方式**：
- 日常部署（Deployment 滚动更新、服务扩缩容）：通过 K8s 标准 API
- 临时预览环境（创建/销毁 Namespace）：通过 K8s 标准 API
- 集群级运维（节点扩缩、组件升级、安全巡检）：通过阿里云 CS API

### 17.2 APISIX 部署

APISIX 作为 Ingress Controller 部署在 ACK 集群上，同时处理：
- **水平流量**（外部）：Portal、Bot、CLI 等客户端的 API 请求
- **垂直流量**（内部）：Forge 平台各微服务之间的调用

Forge 平台所有服务（engine/identity/pipeline 等）均注册在 APISIX 后方，统一享受网关层的鉴权、限流、灰度、监控等能力。

---

## 18. 风险与应对

| 风险 | 概率 | 影响 | 应对 |
|------|------|------|------|
| AI 生成代码质量不达标 | 高 | 高 | 多轮 Review + 修复循环；aegis 编码规范基线 + Prompt eval 测试保障；初期收紧自动合并阈值 |
| AI 模型 API 不稳定/限流 | 中 | 高 | 多模型 fallback（Claude → GPT → 通义）；请求排队；错峰调度 |
| Codeup API 限流或不可用 | 中 | 高 | 文件内容 Redis 缓存 + Webhook 增量刷新；API 调用端令牌桶限流；降级为排队等待 |
| 小白用户需求描述模糊 | 高 | 中 | AI 对话式澄清；需求模板引导；常见需求快捷入口 |
| 安全风险（AI 生成不安全代码） | 中 | 高 | 专项安全 Review 规则；涉及安全模块的变更在两阶段风险评估中强制标记为高风险；安全扫描门禁 |
| Token 费用失控 | 中 | 高 | 任务级预估 + 租户月度预算 + 硬上限中止（见第 9 章） |
| AI 生成不可逆 DB 迁移 | 中 | 高 | 必须生成 UP+DOWN 迁移；临时环境验证可逆性；破坏性操作强制人工审批 |
| 生产事故（AI 代码通过所有门禁但线上异常） | 低 | 极高 | 三级紧急停止开关（见第 14 章）；APISIX 灰度发布 + 自动回滚；自动触发规则（连续部署失败 → L1，频繁回滚 → L2） |
| 代码上下文窗口不足 | 中 | 中 | 签名摘要 + ES kNN 向量检索 + Token 预算裁剪；超大项目建议拆分需求 |
| APISIX 网关单点 | 低 | 高 | APISIX 多副本部署 + etcd 集群；健康检查 + 自动故障转移 |
| Nacos 密钥泄露 | 低 | 极高 | Nacos 加密配置 + ACK Pod 网络策略限制访问范围；ACK/ACR 使用 RAM 角色无密钥方式 |
| 多租户数据泄露 | 低 | 极高 | MyBatis 插件自动注入 tenant_id 过滤；全平台统一的租户隔离机制；定期安全审计 |
