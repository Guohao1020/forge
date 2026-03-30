# Forge Platform — 技术设计文档

> **版本**: 1.0
> **日期**: 2026-03-29
> **作者**: Harvey + Claude
> **前置文档**: [PRD.md](PRD.md)

---

## 目录

- [1. 架构总览](#1-架构总览)
- [2. AI 引擎技术设计](#2-ai-引擎技术设计)
- [3. 鉴权中心技术设计](#3-鉴权中心技术设计)
- [4. DevOps 自动化技术设计](#4-devops-自动化技术设计)
- [5. Web 工作台技术设计](#5-web-工作台技术设计)
- [6. 实时网关技术设计](#6-实时网关技术设计)
- [7. 外部平台适配器技术设计](#7-外部平台适配器技术设计)
- [8. 自动化测试平台技术设计](#8-自动化测试平台技术设计)
- [9. 产品加速库技术设计](#9-产品加速库技术设计)
- [10. 数据架构](#10-数据架构)
- [11. 高可用设计](#11-高可用设计)
- [12. 部署架构](#12-部署架构)
- [13. 技术选型总览](#13-技术选型总览)
- [附录：引用资料索引](#附录引用资料索引)

---

## 1. 架构总览

### 1.1 架构选型

中心化 Agent 架构 — 一个中央大脑（Engine）接收需求、编排任务、调度 Worker、控制风险。

选型理由：
1. 最快落地
2. 风险分级天然支持 — 中央大脑决定自动还是人工
3. 后续可演进为混合架构

### 1.2 系统全景

```
┌─────────────────── 接入层 ───────────────────┐
│  Web 工作台    IM 机器人(钉钉/飞书)    CLI    │
└──────────────┬───────────────────┬────────────┘
               │ HTTP / WebSocket  │
┌──────────────▼───────────────────▼────────────┐
│          APISIX（统一 API 网关）                │
│   路由 / 鉴权 / 限流 / 灰度 / 负载均衡         │
└──────────────┬────────────────────────────────┘
               │
       ┌───────┼───────────────────────┐
       ▼       ▼                       ▼
┌──────────┐ ┌──────────┐      ┌──────────────┐
│ 鉴权中心  │ │ 实时网关  │      │  AI 引擎     │
│          │ │(长连接)   │      │ (中央大脑)    │
└──────────┘ └──────────┘      │              │
                               │ 编排 + 调度   │
                               │      │       │
                               │  Worker Pool │
                               │ (弹性伸缩)   │
                               └──────┬───────┘
                                      │
                  ┌───────────────┬────┴────────────┐
                  ▼               ▼                  ▼
           ┌──────────┐   ┌──────────┐       ┌───────────────────┐
           │ 规范中心  │   │ DevOps   │       │  外部平台适配器层   │
           │          │   │ 自动化   │       │                   │
           │          │   │          │       │ ┌───────────────┐ │
           └──────────┘   └──────────┘       │ │ 代码托管适配器  │ │
                                             │ │ (Codeup/...)  │ │
                                             │ ├───────────────┤ │
                                             │ │ 容器编排适配器  │ │
                                             │ │ (ACK/...)     │ │
                                             │ ├───────────────┤ │
                                             │ │ CI/CD适配器   │ │
                                             │ │ (云效/...)     │ │
                                             │ └───────────────┘ │
                                             └───────────────────┘
```

### 1.3 子系统清单

| # | 子系统 | 定位 | 技术栈 |
|---|--------|------|--------|
| 1 | **鉴权中心** | 统一身份认证 + 动态鉴权链 + 权限管理 | Java 17, Spring Boot 3.x |
| 2 | **AI 引擎** | 中央大脑：需求解析 → 任务编排 → 代码生成 → 审查 | Java 17, Spring Boot 3.x |
| 3 | **DevOps 自动化** | 流水线模板 + 质量门禁 + 部署编排 + 配置发布 | Java 17 + YAML 模板 |
| 4 | **Web 工作台** | 需求输入 + 任务看板 + 代码预览 + 审批 + 灰度管理 | Vue 3 + TypeScript |
| 5 | **IM 机器人** | 钉钉/飞书入口，对话式需求提交和进度跟踪 | Java 17 |
| 6 | **实时网关** | WebSocket 长连接，AI 流式输出 + 进度推送 | Node.js, Socket.IO |
| 7 | **规范中心** | 编码规范 + 脚手架模板 + Prompt 模板 + Review 规则 | Markdown + JSON + Java 服务 |
| 8 | **产品加速库** | AI 孵化产品的可选组件库 | Java 17, Spring Boot 3.x |

### 1.4 子系统依赖关系

```
AI 引擎（核心）
    │
    ├── 鉴权中心（被所有需要认证的子系统依赖）
    ├── 规范中心（被 AI 引擎消费：编码规范 + Prompt + Review 规则）
    ├── DevOps 自动化（被 AI 引擎调用：触发流水线 + 查询状态 + 配置发布）
    │       └── 依赖外部平台适配器层（CI/CD 适配器 + 容器编排适配器）
    ├── AI 引擎自身也依赖外部平台适配器层（代码托管适配器）
    ├── Web 工作台（前端，调用 AI 引擎 + 鉴权中心 API）
    ├── IM 机器人（调用 AI 引擎 API）
    └── 实时网关（对接鉴权中心做连接鉴权，被 AI 引擎用于推送进度）

外部平台适配器层（横切层，被 AI 引擎和 DevOps 自动化共同依赖）
产品加速库（独立，仅被 AI 孵化出的业务系统引用）
```

**关键原则：各子系统是独立的 Spring Boot 服务，互不强耦合。产品加速库与平台无依赖关系。**

### 1.5 统一网关

APISIX 作为全平台唯一的 API 网关，同时处理：
- **水平流量**（外部）：Web 工作台、IM 机器人、CLI 等客户端请求
- **垂直流量**（内部）：各微服务之间的调用

所有子系统均注册在 APISIX 后方，统一享受鉴权、限流、灰度、可观测等网关层能力。

---

## 2. AI 引擎技术设计

### 2.1 模块拆分

AI 引擎拆分为两个独立部署的服务：

| 模块 | 职责 | 部署特性 |
|------|------|---------|
| **编排服务** | 接收需求、状态机驱动、风险评估、任务调度、对外 API | 有状态（Leader 选举），少量实例 |
| **执行服务** | 从任务队列拉取步骤、调用 AI 模型、上下文构建、代码生成/审查/测试生成 | 无状态，可水平扩缩 |

执行服务是单一服务，按任务类型路由到不同处理逻辑（代码生成/审查/测试/修复），不拆分为独立部署的多个服务。

### 2.2 通信方式

| 链路 | 通道 | 理由 |
|------|------|------|
| 编排服务 → 执行服务（任务派发） | Kafka | 持久化、可回溯、支持消费组 |
| 执行服务 → 编排服务（结果回报） | Kafka | 同上 |
| 执行服务心跳 | Redis TTL | 轻量、TTL 天然支持超时检测 |
| 任务状态缓存 | Redis | 高频读写 |

### 2.3 多模型路由

| 任务类型 | 首选模型 |
|---------|---------|
| 需求分析 / 架构设计 | Claude Opus |
| 代码生成（复杂） | Claude Sonnet |
| 代码生成（简单） | 通义灵码 |
| Code Review | Claude Opus |
| 测试用例生成 | Claude Sonnet |
| 文档 / 注释生成 | Claude Haiku |

每个模型独立熔断统计（成功率 < 80% 触发熔断），Fallback 链：Claude → GPT → 通义 → 排队等待。

模型适配器为插件化设计 — 新增模型只需实现统一接口 + 配置路由规则。

### 2.4 上下文构建

上下文完全通过代码托管适配器远程获取，不做本地克隆，保持执行服务完全无状态。

**静态上下文**（每个项目固定）：
- 编码规范 — 来自规范中心
- 脚手架结构 — 来自规范中心模板库
- Review 规则 — 来自规范中心规则库
- Prompt 模板 — 来自规范中心提示词库

**动态上下文**（每次任务通过代码托管适配器动态加载）：
- 相关代码文件 — 基于需求分析确定
- 数据库 Schema — 项目 Flyway 迁移文件
- API 契约 — 已有 Controller/DTO 定义
- 最近变更 — Git log（避免冲突）
- 用户对话历史 — 会话存储

**上下文优化策略**：
- Token 预算管理：按优先级裁剪
- 代码摘要：大文件只送关键签名 + 注释
- 增量上下文：迭代修改时只送 diff + 周边
- RAG 检索：通过 Elasticsearch kNN 向量检索相关段落

**处理上限与降级**：

| 场景 | 策略 |
|------|------|
| 项目 < 50 文件 | 全量加载 |
| 项目 50~500 文件 | 签名索引 + 精准加载 + RAG |
| 项目 > 500 文件 | 仅索引 + RAG，加载限定为需求相关模块 |
| 单文件 > 1000 行 | 拆分为签名 + 关键方法体 |
| 上下文仍超限 | 告知用户需求范围太大，建议拆分 |

### 2.5 代码生成分阶段

Phase A（串行 — 契约先行）：
- AI 先生成接口契约：DTO 定义、API 接口签名、数据库 Schema 变更

Phase B（并行 — 基于契约实现）：
- 执行服务 A → Java 后端实现
- 执行服务 B → 前端页面
- 执行服务 C → 单元测试
- 每个执行服务生成完毕后通过代码托管适配器原子提交到 ai/feature-xxx 分支

Phase C（串行 — 集成验证）：
- 检查跨文件一致性（接口签名是否匹配、DTO 字段是否对齐）

---

## 3. 鉴权中心技术设计

### 3.1 动态鉴权链（责任链模式）

运行时可动态增减的插件式鉴权器：
- OAuth2 / OIDC
- LDAP / Active Directory
- API Token / AK-SK 签名
- 钉钉扫码 / 飞书扫码
- SSO SAML
- SSH Key（Git 操作）
- Personal Access Token
- OAuth2 Device Flow（CLI）
- IM 平台回调验签

每种接入场景的鉴权链可独立配置，配置存 DB，热更新。

### 3.2 授权引擎

- RBAC — 基于角色（管理员/开发/PM）
- ABAC — 基于属性（部门/项目/环境）
- PBAC — 基于策略（JSON 规则动态下发）
- 数据权限 — 行级/列级（MyBatis 插件自动注入 tenant_id）

### 3.3 Token 服务

- JWT 签发 / 刷新 / 吊销
- 多端登录管理（同时在线数控制）
- Token 黑名单（Redis，带 TTL 自动清理）

### 3.4 MFA 服务

- TOTP（Google Authenticator）
- 短信验证码
- 敏感操作二次验证触发器

---

## 4. DevOps 自动化技术设计

### 4.1 流水线模板

| 项目类型 | 流水线阶段 |
|---------|-----------|
| Java 微服务 | 编译 → 单测 → 代码扫描 → 镜像构建推送 → Helm 部署 |
| Vue 前端 | npm install → build → 镜像构建（nginx）→ Helm 部署 |
| SDK 类库 | 编译 → 单测 → 发布到私有 Nexus |

### 4.2 环境管理

**临时环境**（AI 分支预览）：
- AI 推送 ai/feature-xxx 分支 → 通过容器编排适配器自动创建临时 K8s Namespace
- 包含独立的 DB（schema clone）+ Redis + 服务实例
- MR 合并后 30min 自动销毁，有清理守护进程防止孤儿环境
- 资源配额限制（防止临时环境耗尽集群资源）

**固定环境**：
- dev → develop 分支自动部署
- staging → release 分支自动部署
- prod → master 分支，人工审批后灰度部署

### 4.3 质量门禁工具链

| 检查项 | 工具 |
|--------|------|
| 安全扫描 | SonarQube + OWASP Dependency Check |
| 镜像漏洞扫描 | Trivy |
| AI Review | AI 引擎 code-review Prompt |

### 4.4 灰度部署（APISIX 实现）

灰度发布完全通过 APISIX 网关层实现，业务服务零侵入。方法论参见 [references/gray-release-methodology.md](references/gray-release-methodology.md)。

**三层决策模型**：

| 优先级 | 决策层 | 实现方式 |
|--------|--------|---------|
| 最高 | 环境分区 | APISIX 路由级别隔离 |
| 中 | 规则匹配 | APISIX traffic-split 插件条件匹配（Header/Cookie/参数） |
| 最低 | 比例分流 | APISIX weighted_upstreams 按权重分配 |

灰度规则通过 APISIX Admin API 即时变更，无需重启。

### 4.5 配置自动发布

1. AI 根据项目规范生成配置文件（application.yml 等）
2. DevOps 模块校验配置（命名规范、敏感字段检测、格式正确性）
3. 校验通过后自动发布到 Nacos 对应命名空间和分组
4. 目标服务通过 Nacos 监听机制热加载

---

## 5. Web 工作台技术设计

### 5.1 技术选型

| 维度 | 选型 |
|------|------|
| 框架 | Vue 3.5 + TypeScript |
| 构建 | Vite |
| UI 库 | Ant Design Vue 4.x |
| 状态管理 | Pinia 3.x |
| 路由 | Vue Router 4.x |
| HTTP | Axios |
| 代码编辑器 | Monaco Editor |
| 图表 | ECharts |
| 图标 | Lucide Icons |
| 实时通信 | SSE（Phase 1）→ Socket.IO（Phase 2） |
| 字体 | Geist Sans + Geist Mono |

### 5.2 视觉设计

"深空指挥中心"风格 — 深色模式唯一，品牌紫 #8B5CF6，Aurora 极光背景，毛玻璃弹窗。

详细视觉规范见 [product-design.md](product-design.md) Section 9。

### 5.3 代码可视化

不自建的能力（由代码托管平台提供）：Git 存储引擎、代码搜索、CI 构建执行器、Webhook 管理

做体验增强：
- AI Diff 智能注释（每段变更附解释）
- 风险标注（高亮可能有问题的代码行）
- AI 对话式 Code Review（在代码行上提问）
- AI 分支可视化（自动命名 ai/feature-xxx）
- 变更影响分析（这次改动影响哪些服务/接口）

### 5.4 AI 工作过程实时推送

Phase 1 使用 SSE（Server-Sent Events）实现 AI 工作过程的实时推送：
- AI 代码生成流式输出
- 任务步骤状态变更
- 测试执行进度
- 部署状态更新

SSE 连接由 AI 引擎直接提供，不经过独立的实时网关（Phase 2 升级为 WebSocket 后再独立部署 forge-beacon）。

---

## 6. 实时网关技术设计

### 6.1 业务隔离（Namespace）

| Namespace | 用途 |
|-----------|------|
| /portal | Web 工作台的实时推送 |
| /bot | IM 消息桥接 |
| /engine | AI 流式输出 |
| /pipeline | 部署进度推送 |

### 6.2 连接类型

**有状态连接**（绑定用户会话）：
- AI 对话 — userId → connectionId 映射存 Redis
- 断线 30s 内重连恢复会话，超时释放

**无状态连接**（订阅模式）：
- 任务进度广播 — 订阅 task:{taskId} 频道
- 系统通知 — 订阅 system:alert 频道

### 6.3 消息类型

| 类型 | 方向 | 说明 |
|------|------|------|
| STREAM_OUTPUT | server → client | AI 生成代码的流式文本 |
| TASK_PROGRESS | server → client | 任务状态机变更 |
| REVIEW_RESULT | server → client | Review 结果通知 |
| DEPLOY_STATUS | server → client | 部署进度 |
| APPROVAL_REQ | server → client | 需要人工审批的请求 |
| SYSTEM_ALERT | server → client | 系统告警 |
| KILL_SWITCH | server → client | 紧急停止通知 |
| USER_INPUT | client → server | 用户输入 |
| HEARTBEAT | 双向 | 心跳保活 |

---

## 7. 外部平台适配器技术设计

### 7.1 设计原则

- **面向能力抽象，不面向厂商 API**：接口按业务能力定义（如"原子提交多文件"），而非按厂商 API 结构
- **最小公共能力集**：接口只暴露所有目标平台都能支持的能力；厂商独有能力通过扩展点提供
- **项目级绑定**：每个孵化项目创建时选择平台组合，生命周期内保持一致
- **凭证统一管理**：所有平台凭证统一存储在 Nacos 加密配置中
- **限流与容错由适配器内部封装**：各厂商 API 的限流策略、重试逻辑、缓存策略在实现层处理

### 7.2 适配器能力矩阵

**代码托管适配器**：

| 能力 | 说明 | 首期实现（Codeup） |
|------|------|------|
| 仓库结构读取 | 获取文件树、文件内容、目录结构 | ListRepositoryTree, GetFileBlobs |
| 代码原子提交 | 一次请求提交多个文件变更 | CreateCommitWithMultipleFiles |
| 分支管理 | 创建/删除/查询分支 | CreateBranch |
| 合并请求管理 | 创建/合并/查询/评审 MR | CreateMergeRequest, MergeMergeRequest |
| 提交历史 | 查询 Git log、diff | ListRepositoryCommits |
| Webhook 管理 | 注册/注销事件回调 | Push Hook, MR Hook, Note Hook |

**容器编排适配器**：

| 能力 | 说明 |
|------|------|
| Namespace 管理 | 创建/删除命名空间 |
| 工作负载管理 | Deployment 创建/更新/回滚/扩缩 |
| 服务暴露 | Service + Ingress 管理 |
| 配置管理 | ConfigMap / Secret 的 CRUD |
| Pod 运维 | 查询 Pod 状态、日志、事件 |
| 集群信息 | 节点状态、资源用量查询 |

**CI/CD 流水线适配器**：

| 能力 | 说明 |
|------|------|
| 流水线模板管理 | 创建/更新流水线配置 |
| 流水线触发 | 手动触发 / 代码推送自动触发 |
| 状态查询 | 运行状态、各阶段进度 |
| 日志获取 | 构建日志、测试日志 |
| 产物管理 | 镜像推送状态、制品版本查询 |

### 7.3 适配器注册机制

适配器实现通过 Spring SPI 机制注册，新增平台支持只需实现适配器接口并注册。

### 7.4 限流应对（代码托管适配器）

- 文件内容 Redis 缓存，仅在 Webhook 通知变更时刷新
- 适配器内置令牌桶限流，不同平台独立配置
- 项目结构索引持久化到 DB，避免重复扫描

> 首期 Codeup 适配器 API 详见 [references/codeup-api.md](references/codeup-api.md)
> 首期 ACK 适配器 API 详见 [references/ack-api.md](references/ack-api.md)

---

## 8. 自动化测试平台技术设计

### 8.1 平台选型

主平台：**MeterSphere**（开源持续测试平台，GPLv3，GitHub 13K+ stars）

| 能力 | 说明 |
|------|------|
| 测试管理 | 用例管理、测试计划、缺陷追踪、报告生成 |
| API 测试 | REST API 测试执行，支持 Swagger 导入 |
| 编程接口 | REST API + MCP Server，AI 可直接创建和执行测试 |
| 部署 | Docker，纳入 docker-compose 基础设施 |

### 8.2 四层测试工具链

| 测试层 | 工具 | 触发方式 |
|--------|------|---------|
| 单元测试 | JUnit 5（Java）/ Jest（前端） | Maven/npm 构建自动运行 |
| 接口测试 | MeterSphere（AI 通过 MCP 创建用例） | AI 代码生成完成后触发 |
| 集成测试 | MeterSphere（AI 编排跨服务测试） | 合并前触发 |
| 回归测试 | CI/CD 流水线全量运行 | 合并前自动触发 |

### 8.3 集成方式

```
Forge AI 生成代码
  ├── 生成单元测试代码 → 写入项目 → Maven/npm 构建运行
  ├── 通过 MeterSphere MCP Server 创建接口/集成测试用例
  ├── MeterSphere 执行测试 → 结果通过 Webhook 回传 Forge
  └── Forge 前端展示四层测试结果
```

测试平台通过适配器模式接入，可替换为其他开源测试平台。

---

## 9. 产品加速库技术设计

### 8.1 设计范式

复用已有脚手架的 starter 模式（详见 [references/scaffold-patterns.md](references/scaffold-patterns.md)）：
- 两层结构：parent 模块 + starter jar
- 自动装配机制（spring.factories / AutoConfiguration）
- 条件激活（按需引入，不强制全家桶）
- @EnableXxx 注解一键启用

### 8.2 技术基础设施层

| 组件 | 技术选型 |
|------|---------|
| 统一响应 + 异常 + 工具类 | Result\<T>、ErrorCode、分页、断言 |
| Redis 集成 | Redisson + Spring Data Redis |
| 数据库集成 | MyBatis-Plus + 多数据源 + Flyway |
| 消息队列抽象 | 统一 API，Kafka / RocketMQ 可切换 |
| 分布式调度 | XXL-Job 集成 |
| 对象存储 | 阿里云 OSS / MinIO 双支持 |
| 统一日志 | Logback + Logstash 对接 |
| 指标采集 | Micrometer + Prometheus |

---

## 10. 数据架构

### 10.1 AI 引擎数据

| 数据表 | 用途 | 关键字段 |
|--------|------|---------|
| 任务主表 | 孵化任务全生命周期 | 任务ID、租户ID、用户ID、需求描述、任务类型、状态、风险等级、Token 消耗、费用 |
| 任务步骤表 | checkpoint 恢复用 | 任务ID、步骤名、执行状态、Worker 实例、输入/输出快照、Token 消耗 |
| 模型调用日志 | Token 用量追踪（append-only） | 任务ID、模型ID、用途、input/output tokens、费用、延迟、是否降级 |
| 代码变更记录 | 跟踪 AI 生成的代码变更 | 任务ID、仓库地址、分支名、commit hash、Review 评分、MR 状态 |

### 10.2 鉴权中心数据

| 数据表 | 用途 | 关键字段 |
|--------|------|---------|
| 租户表 | 多租户管理 | 租户ID、名称、月度 Token 预算、状态 |
| 用户表 | 用户账号 | 租户ID、用户名、邮箱、密码哈希、状态 |
| 角色表 | RBAC 角色定义 | 租户ID、角色编码、角色名、作用域（平台/项目） |
| 用户角色绑定 | 用户-角色-项目关联 | 用户ID、角色ID、项目ID |
| 鉴权链配置 | 动态鉴权链 | 租户ID（NULL=全局）、链路名、鉴权类型、排序、启用状态、配置 |

### 10.3 DevOps 数据

| 数据表 | 用途 | 关键字段 |
|--------|------|---------|
| 流水线执行记录 | 流水线运行历史 | 执行ID、任务ID、项目ID、流水线类型、状态、触发方式、开始/结束时间、日志地址 |
| 部署记录 | 部署历史与回滚依据 | 部署ID、项目ID、环境、版本号、镜像地址、部署状态、灰度比例、回滚源 |
| 环境状态 | 临时/固定环境管理 | 环境ID、项目ID、Namespace、环境类型、状态、创建时间、过期时间 |

### 10.4 适配器配置数据

| 数据表 | 用途 | 关键字段 |
|--------|------|---------|
| 平台适配器注册 | 系统可用的外部平台列表 | 适配器ID、适配器类型（代码托管/容器编排/CI/CD）、平台名称、启用状态 |
| 项目平台绑定 | 每个孵化项目绑定的平台组合 | 项目ID、代码托管适配器ID、容器编排适配器ID、CI/CD适配器ID |
| 平台凭证引用 | 项目与 Nacos 凭证的映射 | 项目ID、适配器ID、Nacos 配置 dataId、凭证类型 |

所有业务表带 tenant_id 字段，MyBatis 插件自动注入过滤条件。

---

## 11. 高可用设计

| 风险点 | 解决方案 |
|--------|---------|
| AI 引擎单点 | 多实例部署 + K8s Lease Leader 选举，Leader 做编排，Follower 热备秒切 |
| 任务丢失 | 任务持久化 DB + Redis 双写，重启后从 DB 恢复未完成任务 |
| Worker 崩溃 | Worker 心跳注册（Redis TTL），超时未完成自动释放给其他 Worker |
| 长任务中断 | 任务拆分为多个 checkpoint，每步持久化中间状态，恢复时从最后 checkpoint 继续 |
| 流量突增 | Worker Pool K8s HPA，基于 Kafka 队列深度自动扩容 |
| AI 模型故障 | 多模型 fallback 链（Claude → GPT → 通义），超时/限流自动降级 |
| 数据一致性 | 编排状态机 + 幂等设计，唯一 traceId，重复执行跳过已完成步骤 |
| 网关高可用 | APISIX 多副本 + etcd 集群；实时网关无状态多实例 + Redis Adapter 跨实例广播 |
| 脑裂防护 | K8s Lease TTL 续约，续约失败主动降级 |
| 级联故障 | 服务间超时 + 熔断 + 舱壁隔离 |
| 代码托管平台 API 限流 | 文件内容 Redis 缓存 + Webhook 增量刷新 + 适配器内置令牌桶限流 |

---

## 12. 部署架构

Forge 平台通过容器编排适配器部署，所有服务容器化运行。首期使用阿里云 ACK，后续可切换为原生 K8s 或其他厂商托管集群。

**适配器封装两层操作**：
- K8s 标准 API 层（所有平台通用）：Deployment、Service、Ingress、ConfigMap、Secret、Namespace、Pod 管理
- 厂商扩展 API 层（平台特有，通过扩展点提供）：集群管理、节点池扩缩、组件管理、安全巡检（首期：阿里云 CS API）

APISIX 作为 Ingress Controller 部署在 K8s 集群上，同时处理水平流量（外部）和垂直流量（内部微服务调用）。

### 12.1 配置管理

| 配置类型 | 存储位置 | 说明 |
|---------|---------|------|
| AI 模型 API Key | Nacos 加密配置 | 按模型分组管理 |
| 代码托管平台凭证 | Nacos 加密配置 | 按平台 + 项目分组 |
| CI/CD 平台凭证 | Nacos 加密配置 | 按平台分组 |
| 容器编排平台凭证 | Nacos 加密配置 / RAM 角色绑定 | 首期 ACK 用 RAM 角色无密钥 |
| 数据库 / Redis 密码 | Nacos 加密配置 | 按环境分组 |
| JWT 签名密钥 | Nacos 加密配置 | 定期轮转 |
| 业务应用配置 | Nacos | AI 生成后自动发布 |

---

## 13. 技术选型总览

| 维度 | 选型 |
|------|------|
| 后端框架 | Java 17 + Spring Boot 3.2 + Spring Cloud 2023.x |
| 前端框架 | Vue 3.5 + TypeScript + Vite |
| 前端 UI | Ant Design Vue 4.x |
| 前端状态管理 | Pinia 3.x |
| 前端图标 | Lucide Icons |
| 前端字体 | Geist Sans + Geist Mono |
| API 网关 | APISIX（统一网关，水平+垂直流量） |
| 实时通信 | SSE（Phase 1）→ Socket.IO（Phase 2+） |
| AI 模型 | Claude (Anthropic) + GPT (OpenAI) + 通义灵码 (Alibaba) |
| 代码托管 | 适配器架构（首期：云效 Codeup） |
| CI/CD | 适配器架构（首期：云效 Flow） |
| 容器编排 | 适配器架构（首期：阿里云 ACK） |
| **自动化测试** | **MeterSphere（开源，GPLv3）+ JUnit 5 + Playwright** |
| 镜像仓库 | 阿里云 ACR（首期） |
| 数据库 | MySQL 8.0 |
| 缓存 | Redis 7 (Redisson) |
| 消息队列 | Kafka（平台内部）/ Kafka + RocketMQ 双支持（孵化产品可选） |
| 分布式调度 | XXL-Job |
| 服务发现 + 配置中心 | Nacos |
| 向量检索 | Elasticsearch 8.x kNN |
| 监控 | Prometheus + Grafana + Micrometer |
| 日志 | ELK (Elasticsearch + Logstash + Kibana) |
| 链路追踪 | SkyWalking / Jaeger |

---

## 附录：关联文档

| 文档 | 说明 |
|------|------|
| [PRD.md](PRD.md) | 产品需求文档 |
| [product-design.md](product-design.md) | 产品设计规格书 — 页面详细设计、交互流程、视觉规范 |
| [milestone-plan.md](milestone-plan.md) | 里程碑计划 — 分阶段交付路线图 |

## 附录：引用资料索引

| 文件 | 内容 | 来源 |
|------|------|------|
| [references/coding-standards.md](references/coding-standards.md) | 编码规范基线 | aegis 工程实践 |
| [references/scaffold-patterns.md](references/scaffold-patterns.md) | 脚手架设计范式 | solar-foundation 架构模式 |
| [references/gray-release-methodology.md](references/gray-release-methodology.md) | 灰度发布方法论 | kohinur 灰度发布理念 |
| [references/codeup-api.md](references/codeup-api.md) | Codeup API 能力清单与限流策略 | 阿里云云效文档 |
| [references/ack-api.md](references/ack-api.md) | ACK/K8s API 能力清单与认证方式 | 阿里云 ACK 文档 |
