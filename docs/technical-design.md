# Forge Platform — 技术设计文档 v2.2

> **版本**: 2.2
> **日期**: 2026-04-02
> **作者**: Harvey + Claude
> **前置文档**: [PRD.md](PRD.md) | [Product Design](product-design.md) | [Milestone Plan](milestone-plan.md)
> **架构变更**: 从 Java 微服务架构全面重构为 Go + Python + Temporal 架构

---

## 目录

- [1. 架构总览](#1-架构总览)
- [2. Harness 六大组件设计](#2-harness-六大组件设计)
- [3. AI 引擎技术设计](#3-ai-引擎技术设计)
- [4. 鉴权中心技术设计](#4-鉴权中心技术设计)
- [5. 规范中心技术设计](#5-规范中心技术设计)
- [6. DevOps 自动化技术设计](#6-devops-自动化技术设计)
- [7. Web 工作台技术设计](#7-web-工作台技术设计)
- [8. IM 机器人技术设计](#8-im-机器人技术设计)
- [9. 外部平台适配器技术设计](#9-外部平台适配器技术设计)
  - [9.5 CLI 入口设计](#95-cli-入口设计)
  - [9.6 产品加速库 (forge-foundation)](#96-产品加速库-forge-foundation)
  - [9.7 配置自动发布](#97-配置自动发布)
- [10. 数据架构](#10-数据架构)
- [11. 部署架构](#11-部署架构)
- [12. 高可用设计](#12-高可用设计)
- [13. 技术选型总览](#13-技术选型总览)
- [14. 三期工程实施计划](#14-三期工程实施计划)

---

## 1. 架构总览

### 1.1 架构理念

**Harness Engineering 平台** — 不仅是代码生成，而是规范约束 + 机械化验证 + 可观测性闭环的完整 Harness 环境。

核心理念源自 Harness Engineering 三大支柱：
- **Context Engineering** — 规范中心 + 项目画像 + Prompt 模板 + 可观测性数据
- **Architectural Constraints** — Linter 引擎 + 结构测试 + AI Review + 质量门禁
- **Entropy Management** — 定期代码质量扫描 + 自动修复 + 趋势追踪

关键洞察：**Harness 不是给模型加更多工具，而是给模型补上运行边界、恢复机制和验证秩序。**

### 1.2 架构选型

**Temporal 驱动的工作流架构** — 以 Temporal 作为"状态脊梁"，所有 AI 任务都是有状态、可恢复、可观测的 Workflow。

选型理由：
1. Temporal 一个组件解决状态管理、Checkpoint、恢复、超时、重试、可观测性 — 直接覆盖 Harness 六大组件中"状态与 Checkpoint" + "编排协调"
2. 省掉 Kafka、Nacos、自建状态机 — 架构大幅简化
3. Go + Python 混合：Go 做高性能 API 和 DevOps Worker，Python 做 AI 编排（LangGraph 生态最好）
4. 每个 Temporal Activity 天然可观测，自带 retry/timeout/heartbeat

与旧 Java 微服务架构的对比：

| 维度 | 旧架构 (Java) | 新架构 (Go + Python) |
|------|-------------|---------------------|
| 应用服务数 | 6 Java + 1 Node.js = 7 | 1 Go API + 1 Python Worker + 1 Go Bot = 3 |
| 数据库 | 4 MySQL 实例 | 1 PostgreSQL (多 Schema) |
| 消息队列 | Kafka + ZooKeeper | 无 (Temporal 替代) |
| 配置中心 | Nacos | K8s ConfigMap + SOPS 加密 |
| API 网关 | APISIX + etcd | Traefik (统一入口+路由+限流) |
| 状态管理 | 自建状态机 | Temporal (生产级) |
| AI 编排 | 自建 Worker Pool | LangGraph (开源成熟) |
| 代码质量 | 人工 Review | Claude AI Review + Semgrep (AI 替代 SonarQube) |
| 自动化测试 | 手动测试 | Claude 生成测试 + 原生框架运行 (AI 替代 MeterSphere) |
| 搜索 | 无 | PostgreSQL FTS (AI 替代 Elasticsearch) |
| 可观测性 | 基础 Metrics | Grafana + Loki + Prometheus (三期评估 DeepFlow/OTel) |

### 1.3 系统全景

```
用户入口
├── Web 工作台 (Next.js React)
├── Web IDE (code-server / Monaco Editor)
├── IM 机器人 (Go 轻量服务)
└── CLI

        │
   Traefik (TLS 终止 + 负载均衡 + JWT 验证 + 限流 + 灰度路由)
        │
┌───────┴──────────────────────────────────────────┐
│            Go API Server (模块化单体)              │
│  ┌────────────────────────────────────────────┐   │
│  │ Middleware: JWT → tenant_id → RBAC → 限流   │   │
│  ├────────────────────────────────────────────┤   │
│  │ Auth Module     │ 登录/注册/Token/OAuth     │   │
│  │ Project Module  │ 项目管理/适配器绑定        │   │
│  │ Task Module     │ 任务CRUD/SSE推送          │   │
│  │ Specs Module    │ 规范/Prompt/Rule/脚手架    │   │
│  │ Settings Module │ 系统配置/紧急停止          │   │
│  │ Adapter Module  │ GitHub/Codeup/K8s 适配器  │   │
│  │ Billing Module  │ Token追踪/成本预算         │   │
│  └────────────────────────────────────────────┘   │
│  KillSwitch: Redis 分布式标志 (L1/L2/L3)          │
│  SSE/WebSocket: /stream/tasks/{id}                │
└───────┬──────────────────────────────────────────┘
        │
   Temporal Server (状态脊梁)
   ├── TaskWorkflow (主编排: 需求→生成→验证→部署)
   ├── ConstraintWorkflow (Lint→Review)
   ├── DeployWorkflow (Argo CD + 健康检查)
   ├── EntropyWorkflow (定期扫描)
   │
   ├── Interceptors:
   │   ├── KillSwitchInterceptor (检查紧急停止标志)
   │   ├── BudgetInterceptor (检查 Token 预算)
   │   └── AuditInterceptor (记录所有 Activity)
   │
   Workers:
   ├── AI Worker (Python + LangGraph)
   │   ├── Planner Agent (需求分析/任务分解)
   │   ├── Coder Agent (代码生成, 多模型路由)
   │   ├── Reviewer Agent (AI Review + Judge 评分, 替代 SonarQube)
   │   ├── Fixer Agent (自动修复, ≤3 轮)
   │   ├── TestGen Agent (生成单测/API测试/集成测试, 替代 MeterSphere)
   │   └── Constraint Formatter (Lint结果→标准JSON)
   │
   ├── DevOps Worker (Go)
   │   ├── GitActivity (commit/branch/MR via 适配器)
   │   ├── PipelineActivity (GitHub Actions / Argo Workflows)
   │   ├── DeployActivity (Argo CD + Rollouts)
   │   ├── TestRunActivity (调用目标项目原生测试框架)
   │   └── HealthCheckActivity (Prometheus 指标查询)
   │
   └── Constraint Worker (Go)
       ├── LintActivity (语言原生 Linter: golangci-lint/eslint/ruff)
       ├── ScanActivity (Semgrep 安全扫描)
       └── StructuralTestActivity (架构约束验证)

数据层:
├── PostgreSQL (单实例 HA, 多 Schema)
│   ├── auth schema (users, roles, permissions, tokens)
│   ├── engine schema (tasks, steps, checkpoints, model_calls)
│   ├── specs schema (standards, prompts, rules, scaffolds)
│   ├── pipeline schema (environments, deployments, pipelines)
│   ├── billing schema (token_usage, budgets)
│   └── Full-Text Search (tsvector + GIN, 替代 Elasticsearch)
├── Redis Sentinel (会话/Token黑名单/限流/KillSwitch标志)
└── 本地文件系统 / 阿里云 OSS (代码产物/测试报告/备份, 替代 MinIO)

可观测性:
├── Grafana (统一 Dashboard)
├── Loki (日志聚合)
├── Prometheus + Alertmanager (指标 + 告警)
└── Temporal Web UI (工作流可视化)
```

> **精简理念: AI 替代中间件**
>
> 传统平台用独立中间件做测试管理 (MeterSphere)、代码质量 (SonarQube)、搜索 (Elasticsearch) 等。
> Forge 用 Claude/GPT 的语义理解能力替代这些中间件:
> - **测试**: AI 生成测试代码 + 目标项目原生测试框架运行 (go test/pytest/jest)
> - **代码质量**: AI Reviewer Agent 做语义级 Review (理解业务上下文, 比 SonarQube 规则匹配更智能)
> - **搜索**: AI 做语义搜索 + PostgreSQL FTS 做关键词搜索
> - **Lint**: 直接调用语言原生 Linter (golangci-lint/eslint/ruff), 不需要 MegaLinter 包装层
> - **网关**: Traefik 统一承担 (TLS + 路由 + JWT + 限流 + 灰度), 去掉 APISIX + etcd
> - **密钥**: SOPS + age 加密配置文件, K8s Secret, 去掉 Vault
> - **存储**: PostgreSQL + 本地 FS / 阿里云 OSS, 去掉 MinIO

### 1.4 模块职责与通信

| 模块 | 语言 | 通信方式 | 职责 |
|------|------|---------|------|
| forge-core | Go | HTTP/SSE/WebSocket | 统一 API 入口，鉴权，项目管理，任务管理，规范管理，系统配置 |
| ai-worker | Python | Temporal Worker | AI 编排 (LangGraph)，多模型路由，代码生成/Review/修复 |
| devops-worker | Go | Temporal Worker | Git 操作，CI/CD 触发，部署，测试执行，健康检查 |
| constraint-worker | Go | Temporal Worker | Lint 执行，安全扫描，架构约束验证 |
| forge-bot | Go | HTTP (Webhook) | 钉钉/飞书消息接收，任务触发，进度推送 |
| forge-portal | Next.js | HTTP → forge-core | Web 工作台 UI |

通信原则：
- **同步请求**: 前端 → Traefik → forge-core (HTTP REST)
- **异步编排**: forge-core → Temporal → Workers (Temporal Protocol)
- **实时推送**: forge-core → 前端 (SSE/WebSocket)
- **IM 交互**: 钉钉/飞书 → forge-bot → Temporal (Webhook → Temporal Signal)

---

## 2. Harness 六大组件设计

Harness 可以视为 Agent 的操作系统，目标是把模型外面的执行环境补完整，让运行过程具备边界、状态和秩序。

### 2.1 工具接入层 (Tool Access)

**职责：** 统一 API、权限控制和沙箱执行，保证工具使用可控。

```go
// Tool Registry — 每个工具声明元数据
type ToolDefinition struct {
    Name        string            `json:"name"`
    Description string            `json:"description"`
    InputSchema json.RawMessage   `json:"input_schema"`
    OutputSchema json.RawMessage  `json:"output_schema"`
    RiskLevel   RiskLevel         `json:"risk_level"`    // LOW/MEDIUM/HIGH/CRITICAL
    Timeout     time.Duration     `json:"timeout"`
    RequiredPermissions []string  `json:"required_permissions"`
}

type RiskLevel int
const (
    RiskLow      RiskLevel = iota  // 读取操作
    RiskMedium                     // 写入非生产环境
    RiskHigh                       // 写入生产环境
    RiskCritical                   // 不可逆操作 (DB migration, 删除)
)
```

每个工具封装为 Temporal Activity：

| Activity | 风险等级 | 超时 | 重试 |
|----------|---------|------|------|
| CodeHostingRead | LOW | 30s | 3次 |
| CodeHostingWrite | MEDIUM | 60s | 2次 |
| LinterExec | LOW | 5min | 2次 |
| SecurityScan | LOW | 10min | 1次 |
| AIChatCall | MEDIUM | 5min | 2次 |
| DeployDev | MEDIUM | 10min | 1次 |
| DeployProd | CRITICAL | 15min | 0次 (失败即人工) |
| DBMigration | CRITICAL | 5min | 0次 |

**沙箱执行**: HIGH/CRITICAL 风险工具在隔离容器中运行，限制网络/文件系统访问。

### 2.2 状态与 Checkpoint

**职责：** 持久化中间文件、进度和结果，支持跨会话恢复。

Temporal Workflow 天然提供：
- **Activity 级 Checkpoint**: 每个 Activity 完成后自动持久化，崩溃后从最近 Checkpoint 恢复
- **Heartbeat**: 长时间 Activity（代码生成）定期上报心跳，超时自动重新分配
- **Signal**: 外部事件（人工审批、测试结果）注入运行中的工作流

业务层补充：

```
PostgreSQL (engine.task_checkpoints):
├── REQUIREMENT_CONFIRMED  — 需求确认快照
├── CODE_GENERATED         — 生成代码 + Diff
├── REVIEW_PASSED          — Review 结果 + 评分
├── TEST_PASSED            — 四层测试报告
└── DEPLOYED               — 部署版本 + 健康指标

对象存储 (阿里云 OSS / 本地 FS):
├── /tasks/{task_id}/code/      — 生成的代码文件
├── /tasks/{task_id}/diffs/     — Diff 文件
├── /tasks/{task_id}/reports/   — 测试报告
└── /tasks/{task_id}/logs/      — 构建/部署日志
```

**跨会话恢复**: 用户关闭浏览器后重新打开，通过 `task.workflow_id` 查询 Temporal Workflow 状态，前端从最近 Checkpoint 恢复 UI。

### 2.3 规划与分解

**职责：** 把大任务拆成 DAG，再把节点交给子 Agent 或工具执行。

```python
# LangGraph (Python Worker) — Planner Agent
class TaskGraph:
    steps: list[TaskStep]       # DAG 节点列表
    dependencies: dict[str, list[str]]  # 依赖关系

class TaskStep:
    id: str
    name: str
    step_type: StepType         # GENERATE/LINT/TEST/DEPLOY
    input: dict
    risk_level: RiskLevel
    estimated_tokens: int

# Planner 输出 TaskGraph → Temporal Orchestrator 创建 Child Workflows
# 独立步骤并行执行 (Temporal 原生支持)
# 有依赖的步骤按 DAG 顺序串行
# 每个 Child Workflow = 一个可独立恢复的执行单元
```

项目画像自动分析（Context Engineering 核心）：

```python
# 首次接入项目时自动分析
class ProjectProfile:
    tech_stack: list[str]       # ["Java 17", "Spring Boot 3.2", "MyBatis Plus"]
    architecture: str           # "layered-monolith" / "microservice" / "hexagonal"
    db_schema: dict             # 表结构摘要
    dependencies: list[str]     # 关键依赖
    code_style: dict            # 命名规范、缩进风格
    test_framework: str         # "JUnit5" / "Jest" / "Pytest"
    entry_points: list[str]     # API 入口文件列表
```

### 2.4 验证与反馈

**职责：** 用测试、自一致性、评审和 Judge 形成闭环，失败时触发重试或升级。

```
验证管道 (Temporal Workflow 中的 Activity 序列):

Step 1: Format + Lint (语言原生工具)
   ├── 自动格式化: gofmt / prettier / black
   ├── Lint: golangci-lint / eslint / ruff
   ├── 通过 → 继续
   └── 失败 → 返回 AI Agent 修复 (最多 3 轮)

Step 2: Security Scan (Semgrep)
   ├── 安全规则: SQL injection, XSS, hardcoded secrets
   └── 零 HIGH/CRITICAL 违规才放行

Step 3: AI Review (Claude Reviewer Agent, 替代 SonarQube)
   ├── 用不同模型 (或不同 prompt) Review 生成的代码
   ├── 语义级质量分析: 复杂度、重复、设计问题 (AI 理解业务上下文)
   └── Judge 评分: 0-100, 低于阈值打回重做

Step 4: 自动化测试 (AI 生成 + 原生框架运行, 替代 MeterSphere)
   ├── AI TestGen Agent 生成测试代码 (单测 + API 测试 + 集成测试)
   ├── 目标项目原生框架运行: go test / pytest / jest / JUnit
   ├── 覆盖率报告: go test -cover / pytest --cov / jest --coverage
   └── 失败 → AI 分析 stacktrace → 自动修复 (最多 3 轮)

Step 5: Human Review (仅高风险任务)
   ├── 低风险 (score ≥ 90, ≤5 files, 非核心逻辑): 自动合并
   └── 高风险: 推送审批卡片到 Web / IM

反馈闭环:
├── 每次失败的详细信息 (lint 错误, 测试 stacktrace) 结构化传回 AI
├── AI 修复后重新进入验证管道
└── 超过重试上限 → 升级为人工处理 + 通知
```

约束错误标准化格式（Agent 可消费）：

```json
{
  "violations": [
    {
      "type": "LINT",
      "rule": "java/naming/method-name",
      "severity": "ERROR",
      "file": "src/main/java/com/example/UserService.java",
      "line": 42,
      "column": 5,
      "message": "Method name 'GetUser' should be in camelCase",
      "suggestion": "Rename to 'getUser'",
      "doc_link": "https://forge.internal/specs/java-naming#method"
    }
  ]
}
```

### 2.5 编排协调

**职责：** 规划器、执行器、评审器和优化器各自负责不同阶段，通过工作流连起来。

```
主编排 Workflow (Temporal):

SUBMITTED
  → RequirementAnalysis (Planner Agent)
    → 输出: TaskGraph + RiskAssessment
  → [Signal] 用户确认需求理解
PLANNING_CONFIRMED
  → CodeGeneration (Coder Agent, 可并行多文件)
    → 每个文件 = 1 Child Workflow
GENERATED
  → ValidationPipeline (Lint → Analysis → Review → Test)
    → 失败 → FixLoop (最多 3 轮)
VALIDATED
  → [低风险] AutoMerge → Deploy
  → [高风险] [Signal] 等待人工审批
APPROVED
  → Deployment (Argo CD)
    → 健康检查
DEPLOYED
  → RuntimeValidation (Prometheus 指标检查)
    → 异常 → 自动回滚
COMPLETED

每个阶段转换记录到 task_checkpoints, 任何节点可恢复。
```

设计原则（对齐 Harness Engineering 核心理念）：
- **约束不是限制能力，而是稳定性的来源** — 所有 AI 输出必须通过验证管道
- **Checkpoint 应该优先于一次性跑完整个任务** — 每个阶段持久化，崩溃可恢复
- **Human in the Loop 只出现在高风险和不可逆节点** — 低风险全自动
- **小模型负责执行，大模型负责规划或评审** — 成本分层控制

### 2.6 观测与 Guardrails

**职责：** 日志、指标、Tracing 和安全策略共同构成运行护栏。

```
平台自身观测 (Grafana Stack):
├── Temporal Metrics → Grafana Dashboard
│   └── Workflow 延迟/成功率/队列深度/Activity 分布
├── Go API Metrics → Prometheus
│   └── QPS/P99/错误率/活跃连接数
├── Application Logs → Loki
│   └── 结构化日志 (JSON)
└── AI 调用追踪 → 自建
    └── 模型/token 用量/延迟/成本/成功率

AI 生成产品观测 (三期, 评估 DeepFlow 或 Grafana Alloy + OTel):
├── 自动采集 (eBPF 或 OTel auto-instrumentation)
├── Service Map (自动拓扑 + 黄金指标)
├── Distributed Tracing (网关→服务→DB→队列全覆盖)
└── Continuous Profiling (OnCPU/OffCPU/Memory)

Guardrails:
├── Token 预算控制
│   ├── 每任务: Token 预估预执行 + 硬限 (默认 1M tokens)
│   ├── 每租户: 月预算 80% 软警告 + 100% 硬限
│   └── 全局: 并发 AI 任务限制 + 队列
│   └── 实现: Temporal BudgetInterceptor
├── 三级紧急停止 (KillSwitch)
│   ├── L1 暂停提交: 阻止新任务, 在途任务继续
│   ├── L2 冻结引擎: 暂停任务, 阻止代码提交和部署
│   ├── L3 全面停机: 切断所有入口, 停止所有工作负载
│   ├── 自动触发: 3 次连续部署失败 + 错误率 >5% → L1; 1h 内 >3 次回滚 → L2
│   └── 实现: Redis 分布式标志 + Temporal KillSwitchInterceptor
├── 敏感操作审批
│   └── DB migration / 生产部署 / 权限变更 → 强制人工确认
└── 输入/输出过滤
    ├── Prompt injection 检测
    └── PII 脱敏
```

---

## 3. AI 引擎技术设计

### 3.1 架构

AI 引擎分为两层：
- **编排层 (Go API Server)**: 任务管理、状态机、风险评估、Temporal Workflow 触发
- **执行层 (Python Worker)**: LangGraph Agent 编排、LLM 调用、上下文构建

```
forge-core (Go)                    ai-worker (Python)
┌─────────────┐                    ┌─────────────────────┐
│ Task Module │                    │ LangGraph Workflow  │
│ ├── CRUD    │    Temporal        │ ├── Planner Agent   │
│ ├── 状态机   │ ──────────────►   │ ├── Coder Agent     │
│ └── 风险评估 │    Activity       │ ├── Reviewer Agent  │
└─────────────┘                    │ ├── Fixer Agent     │
                                   │ └── Context Builder │
                                   └─────────┬───────────┘
                                             │
                                   ┌─────────▼───────────┐
                                   │ Model Router        │
                                   │ ├── Claude (Opus/   │
                                   │ │   Sonnet/Haiku)   │
                                   │ ├── GPT-4o          │
                                   │ └── 通义千问 Max      │
                                   └─────────────────────┘
```

### 3.2 多模型路由与降级

```python
class ModelRouter:
    """任务感知的模型选择 + 降级链"""

    routing_rules = {
        # 任务类型 → 首选模型 + 降级链
        "ANALYZE":  ["claude-opus-4", "gpt-4o", "qwen-max"],
        "GENERATE": ["claude-sonnet-4", "gpt-4o", "qwen-max"],
        "REVIEW":   ["claude-opus-4", "gpt-4o"],
        "FIX":      ["claude-sonnet-4", "gpt-4o", "qwen-max"],
        "TEST_GEN": ["claude-haiku-4", "gpt-4o-mini", "qwen-plus"],
    }

    # 每个模型有独立熔断器
    # 错误率 > 50% (30s 窗口) → 熔断 → 降级到下一个模型
    # 熔断后 60s 半开状态 → 尝试恢复
```

### 3.3 代码生成流程

三阶段合约优先模式：

```
Phase A: 合约生成 (Contract-First)
├── 输入: 需求分析结果 + 项目画像 + 编码规范
├── 输出: DTO/VO 定义 + API 签名 + DB Schema (Flyway migration)
└── 验证: 合约完整性检查

Phase B: 并行实现 (Parallel Implementation)
├── 基于合约, 并行生成各层代码
├── 每个文件 = 1 Temporal Child Workflow
│   ├── Controller
│   ├── Service
│   ├── Repository/Mapper
│   ├── 配置文件
│   └── 单元测试
└── 并行度: 受 Token 预算和模型并发限制

Phase C: 集成验证 (Integration Verification)
├── 合并所有文件
├── 编译检查
├── 合约一致性验证 (DTO 使用是否和签名匹配)
└── 通过 → 进入验证管道
```

### 3.4 上下文工程 (Context Engineering)

```python
class ContextBuilder:
    """为 AI 调用构建最优上下文"""

    def build_context(self, task, project, purpose):
        context = []

        # 1. 静态上下文 (规范中心)
        context += self.load_coding_standards(project)
        context += self.load_prompt_template(purpose)
        context += self.load_review_rules(project)

        # 2. 动态上下文 (适配器获取)
        context += self.load_project_profile(project)
        context += self.load_relevant_code(project, task)
        context += self.load_db_schema(project)
        context += self.load_api_contracts(project)

        # 3. 运行时上下文 (可观测性数据, Phase 3)
        context += self.load_runtime_metrics(project)

        # 4. Token 预算优化
        context = self.optimize_context(context, token_budget)

        return context

    def optimize_context(self, context, budget):
        """Token 预算内最大化上下文价值"""
        # 优先级: 编码规范 > 相关代码 > DB Schema > API 合约 > 运行时指标
        # 大项目: RAG 检索相关代码片段, 而非全量加载
        # 长任务: 历史对话结构化摘要, 避免 context window 爆炸
```

### 3.5 并发冲突处理

```
原则:
├── AI 永远不强制覆盖人工代码
├── 冲突时优先保留人工变更
└── 自动解决失败 → 标记 CONFLICT → 通知人工

流程:
1. AI 工作在独立分支 (ai/{task_id})
2. 生成完成后尝试 rebase 到目标分支
3. 无冲突 → 创建 MR
4. 有冲突 → 尝试自动解决 (最多 1 次)
5. 自动解决失败 → 标记冲突 → 通知技术负责人
```

### 3.6 数据库迁移安全

```
原则:
├── 所有 migration 必须可逆 (UP + DOWN)
├── 不允许直接删表/删列 (标记弃用 → 下个版本清理)
└── 先在临时环境验证, 再合入主分支

AI 生成 migration 的约束:
├── 检测现有 Flyway 版本号, 生成下一个序号
├── 拦截危险操作: DROP TABLE/DROP COLUMN/TRUNCATE
├── 生成 DOWN 脚本 (回滚用)
└── 临时环境执行验证 → 成功后才允许合入
```

---

## 4. 鉴权中心技术设计

### 4.1 认证体系

支持多种认证方式, 动态鉴权链：

| 认证方式 | 实现期 | 场景 |
|---------|--------|------|
| 账号密码 + JWT | 一期 | Web 登录 |
| OAuth2/OIDC (GitHub/Codeup) | 一期 | 一键接入 + 项目授权 |
| 钉钉扫码 | 二期 | 企业用户快捷登录 |
| 飞书扫码 | 二期 | 企业用户快捷登录 |
| LDAP | 二期 | 企业目录服务对接 |
| SSO SAML | 二期 | 企业 SSO 集成 |
| API Token | 一期 | CLI / API 调用 |
| MFA (TOTP + SMS) | 二期 | 敏感操作二次验证 |

动态鉴权链：

```go
// 运行时可配置, 从 DB 热加载
type AuthChain struct {
    Authenticators []Authenticator  // 按顺序尝试
    MFATriggers    []MFATrigger     // 触发 MFA 的条件
}

type Authenticator interface {
    Type() string
    Authenticate(ctx context.Context, credentials interface{}) (*AuthResult, error)
}
```

### 4.2 授权模型

三层授权：RBAC + ABAC + PBAC

```
RBAC (角色):
├── PLATFORM_ADMIN  — 平台管理员
├── ORG_ADMIN       — 组织管理员
├── PROJECT_ADMIN   — 项目管理员 (技术负责人)
├── DEVELOPER       — 开发者
└── VIEWER          — 只读

ABAC (属性策略):
├── 示例: "只允许 senior 开发者审批生产部署"
└── conditions: {"user.level": "senior", "resource.env": "prod"}

PBAC (策略):
├── 多条件组合, 优先级排序
└── ALLOW/DENY effect
```

权限粒度：

```
平台级: 用户管理, 租户管理, 全局配置
  └── 组织级: 团队管理, 组织规范配置
      └── 项目级: 项目设置, 成员管理, 规范覆盖
          └── 数据级: 行级 (tenant_id) + 列级 (敏感字段)
```

### 4.3 多租户隔离

```
实现方式: 共享表 + tenant_id 隔离

├── 所有业务表含 tenant_id 字段
├── Go Middleware 自动注入 tenant_id (从 JWT 解析)
├── 数据库查询自动追加 WHERE tenant_id = ?
├── 全局默认配置 + 租户级覆盖 (tenants.config JSONB)
└── 大租户未来支持独立部署 (K8s namespace 隔离)
```

### 4.4 Token 管理

```
JWT 策略:
├── Access Token: 15min TTL, 携带 user_id + tenant_id + roles
├── Refresh Token: 7d TTL, 仅用于刷新 Access Token
├── Token 吊销: Redis 黑名单 (token_jti → TTL = 剩余有效期)
├── 多设备管理: 每用户最多 5 个并发设备, 超出踢掉最早设备
└── 敏感操作: 需要重新验证密码或 MFA
```

---

## 5. 规范中心技术设计

### 5.1 内容体系

| 类型 | 内容 | 用途 |
|------|------|------|
| **编码规范** | Java/SQL/Redis/Kafka/API/安全/命名/Git | AI 生成代码时的约束输入 |
| **Prompt 模板** | 需求分析/代码生成/Code Review/测试生成/修复生成/文档生成 | 标准化 AI 调用 |
| **Review 规则** | 编码规范/OWASP Top 10/性能/数据库/API 兼容/自定义 | 验证管道的评判标准 |
| **脚手架模板** | Java 微服务/Vue 前端/API 网关/SDK | 项目初始化模板 |

### 5.2 三级继承

```
公司级 (默认)
  └── 团队级 (覆盖部分规则)
      └── 项目级 (覆盖部分规则)

合并策略:
├── 子级只能覆盖父级的可覆盖项 (override: true)
├── 父级标记 locked: true 的规则不允许子级修改
└── 生效规范 = 公司默认 + 团队覆盖 + 项目覆盖 (逐级合并)

缓存:
├── Redis: specs:effective:{project_id}:{category} → 合并后的生效规范
├── TTL: 10min
└── 变更时主动失效
```

### 5.3 Prompt 模板管理

```
每个 Prompt 模板:
├── system_prompt: 系统指令
├── user_template: 用户消息模板 (含 {{variables}})
├── variables: 变量定义 [{name, type, required}]
├── version: 版本号 (每次修改自增)
└── eval_cases: 评估测试用例

Prompt 变更验证:
├── 修改 Prompt → 自动运行 eval_cases
├── 新版本 score ≥ 旧版本 score → 允许发布
└── score 下降 → 警告, 需要手动确认
```

---

## 6. DevOps 自动化技术设计

### 6.1 四层自动化测试

| 层 | 工具 | 触发时机 | 通过标准 |
|---|------|---------|---------|
| Unit Test | AI 生成 + go test / pytest / jest | 代码生成后立即 | 覆盖率 ≥ 60% |
| API Test | AI 生成 + httptest / supertest | Unit Test 通过后 | 全部用例通过 |
| Integration Test | AI 生成 + 原生框架 | API Test 通过后 | 全部用例通过 |
| Regression Test | CI Pipeline (GitHub Actions / Argo) | MR 合入前 | 全部用例通过 |

测试生成策略 (AI 替代 MeterSphere):
- AI TestGen Agent 分析代码 + 规范 → 生成针对性测试代码
- 测试代码用目标项目原生框架运行，不依赖外部测试平台
- **测试执行环境**: K8s Job 容器（`forge-jobs` namespace），每任务独立容器隔离
- 覆盖率由原生工具采集 (go test -cover / pytest --cov / jest --coverage)
- 测试报告结构化存储到 PostgreSQL + 文件存 OSS/本地 FS
- 测试失败 → AI 分析 stacktrace → 自动修复 (≤3 轮)

> **为什么不用 MeterSphere**: MeterSphere 开源版功能持续缩水，Java 重型应用资源消耗大，
> 且需要人工维护测试用例。Claude 可以直接读懂代码生成测试，比维护独立测试平台更高效。

### 6.2 质量门禁

```
门禁检查点 (按顺序, 任一失败则阻止):

1. COMPILATION     — 编译通过
2. LINT            — 语言原生 Linter 零 ERROR (golangci-lint/eslint/ruff)
3. SECURITY        — Semgrep 零 HIGH/CRITICAL
4. IMAGE_SCAN      — Trivy 镜像漏洞扫描零 CRITICAL
5. COVERAGE        — 单测覆盖率 ≥ 60%
6. AI_REVIEW       — Review 评分 ≥ 项目阈值 (默认 90)
7. API_COMPAT      — API 签名向后兼容
8. TEST_PASS       — 四层测试全部通过
```

### 6.3 环境管理

| 环境类型 | 分支 | 部署方式 | 说明 |
|---------|------|---------|------|
| 临时环境 | ai/{task_id} | 自动创建/销毁 | AI 分支预览, MR 合并后 30min 销毁 |
| dev | develop | 自动部署 | 开发环境 |
| staging | release | 自动部署 | 预发布环境 |
| prod | master/main | 审批 + 灰度 | 生产环境 |

临时环境：
```
创建: AI 分支推送 → 在 forge-jobs namespace 创建 K8s Job
├── DB: PostgreSQL template DB clone
├── Redis: 独立 DB index
├── 应用: 最小副本 (1 replica)
└── URL: {taskId}.preview.forge.example.com

用户代码部署环境: tenant-{id}-{env} namespace
├── dev:     develop 分支合并后自动部署
├── staging: release 分支合并后自动部署
└── prod:    手动审批后部署

销毁: MR 合并 → 30min 后自动清理
└── Guardian CronJob: 扫描过期/孤儿资源
```

### 6.4 灰度发布

```
Argo Rollouts 灰度策略:

canary:
  steps:
  - setWeight: 5      # 5% 流量
  - pause: {duration: 5m}
  - analysis:          # Prometheus 指标检查
      templates:
      - templateName: success-rate
        args:
        - name: threshold
          value: "0.95"  # 成功率 > 95%
  - setWeight: 25
  - pause: {duration: 5m}
  - analysis: ...
  - setWeight: 50
  - pause: {duration: 5m}
  - analysis: ...
  - setWeight: 100

自动回滚触发:
├── 健康检查失败
├── 错误率超过阈值
└── P99 延迟超过基线 200%
```

---

## 7. Web 工作台技术设计

### 7.1 技术栈

| 层 | 选型 | 理由 |
|---|------|------|
| 框架 | Next.js 15 (App Router) | SSR + RSC + 流式渲染 |
| 语言 | TypeScript (strict) | 全量类型安全 |
| 状态管理 | Zustand | 轻量、TS 友好 |
| 服务端状态 | TanStack Query | 缓存/重试/乐观更新 |
| UI 组件库 | shadcn/ui + Radix | 可深度定制暗色主题 |
| 样式 | Tailwind CSS 4 | 原子化 CSS |
| 图标 | Lucide Icons | — |
| 字体 | Geist Sans + Geist Mono | — |
| 实时通信 | SSE → WebSocket | 流式输出 + 进度推送 |
| 图表 | Recharts | React 原生 |
| 代码编辑器 | Monaco Editor (只读) | 内联 Diff 智能注释 |
| Web IDE | code-server (OpenVSCode Server) | 完整代码浏览, "在 IDE 中打开" |
| 表单 | React Hook Form + Zod | 类型安全验证 |
| Markdown | react-markdown + rehype | AI 输出渲染 |
| 国际化 | next-intl | 中文优先, 预留英文 |
| 测试 | Vitest + Playwright | 单测 + E2E |

### 7.2 视觉体系: "深空指挥中心"

```
品牌色: Forge Purple #8B5CF6

深空背景层级:
├── Deep:    #050510  (最深层背景, OLED 黑)
├── Primary: #0F0F1A  (主内容区/卡片底色)
├── Elevated:#1A1A2E  (悬浮面板/弹窗底色)
├── Hover:   #1C1C2E  (悬停态)
└── Border:  #2A2A3E  (边框)

视觉效果:
├── Glassmorphism: bg-white/5 backdrop-blur-xl border border-white/10
├── Aurora 渐变: 背景动画 (品牌紫 + 蓝 + 翠)
├── 呼吸光效: AI 运行中的脉冲动画
└── 卡片悬停: hover 时微亮 + 边框高亮
```

### 7.3 页面结构

```
/login                              — 登录 (Aurora 背景)
/projects                           — 项目大厅 (一键接入 + 项目卡片网格)
/projects/[id]                      — 项目概览
/projects/[id]/tasks                — 任务看板 (三级视图)
/projects/[id]/tasks/new            — 新建任务 (需求对话)
/projects/[id]/tasks/[taskId]       — AI 工作可视化 (时间线 + 实时工作区)
/projects/[id]/tasks/[taskId]/changes — 变更结果 (AI Diff + 注释)
/projects/[id]/tasks/[taskId]/tests   — 测试报告 (四层)
/projects/[id]/tasks/[taskId]/deploy  — 部署状态
/projects/[id]/branches             — 分支管理
/projects/[id]/reviews              — MR 审批列表
/projects/[id]/reviews/[reviewId]   — MR 审批详情
/projects/[id]/environments         — 环境管理
/projects/[id]/quality              — 质量 Dashboard
/projects/[id]/settings             — 项目设置 (规范/适配器/成员)
/specs/*                            — 规范中心 (平台级)
/admin/*                            — 平台管理 (用户/角色/租户/计费/KillSwitch)
/dashboard                          — 全局 Dashboard
```

### 7.4 实时通信

```
Phase 1 (SSE):
├── 端点: GET /api/stream/tasks/{taskId}
├── Go API: Temporal Workflow Query 轮询 → SSE 推送
├── 前端: EventSource + 自动重连 (指数退避, 最大 30s)
└── 事件类型:
    ├── STREAM_OUTPUT   — AI 代码流式输出
    ├── TASK_PROGRESS   — 任务状态变更
    ├── STEP_COMPLETE   — 步骤完成
    ├── REVIEW_RESULT   — Review 结果
    ├── DEPLOY_STATUS   — 部署进度
    ├── APPROVAL_REQ    — 需要人工审批
    ├── KILL_SWITCH     — 紧急停止通知
    └── ERROR           — 错误

Phase 2 (WebSocket):
├── 端点: WS /ws/tasks/{taskId}
├── 双向通信: USER_INPUT (client→server) + 所有事件类型 (server→client)
├── 心跳: 30s 间隔
└── 断线恢复: 重连后从最近 checkpoint 恢复状态
```

### 7.5 性能优化

| 策略 | 实现 |
|------|------|
| SSR + 流式渲染 | Next.js App Router + Suspense boundary, 首屏秒开 |
| RSC | 项目列表、规范内容等静态数据在服务端渲染 |
| 代码分割 | Monaco Editor、Recharts 等大依赖 dynamic import |
| 虚拟滚动 | 任务列表、日志输出使用 @tanstack/react-virtual |
| 乐观更新 | TanStack Query mutate + optimistic update |
| SSE 节流 | AI 流式输出 16ms 合批渲染 (requestAnimationFrame) |
| 静态资源 CDN | Next.js static export + CDN 缓存 |

### 7.6 权限控制 (前端层)

```
路由级: Next.js Middleware
├── 未登录 → /login
├── 无项目权限 → /projects
└── 无管理权限 → 隐藏 /admin

组件级: <PermissionGate permission="task:approve">
└── 根据用户角色动态显示/隐藏操作按钮

数据级: API 层 tenant_id 自动注入, 前端不处理多租户过滤
```

### 7.7 code-server 集成 (Web IDE)

#### 定位

双层代码浏览体验：
- **Monaco Editor（内联）**: 嵌入变更结果页面，展示 Diff + AI 逐行注释，轻量快速
- **code-server（完整 IDE）**: 提供 VS Code Web 版，用户点击"在 IDE 中打开"后在完整 IDE 环境中浏览代码仓库

#### 架构

```
共享 code-server 实例 (Docker 容器)
├── 部署方式: docker-compose 中独立服务
├── 端口: 8443 (HTTPS)
├── 认证: Forge JWT → code-server proxy auth
├── 仓库访问: 通过 GitHub API clone 到临时目录
└── 生命周期: 实例常驻, 工作区按需创建/回收

用户流程:
1. 用户点击 "在 IDE 中打开" 按钮
2. forge-core API 调用:
   ├── 检查工作区是否已存在
   ├── 不存在 → GitHub clone 到 /workspaces/{project_id}/{branch}
   └── 已存在 → 直接返回 URL
3. 前端打开 code-server URL (新标签页或 iframe)
   └── URL: /ide/{project_id}?folder=/workspaces/{project_id}/{branch}
4. 工作区回收: 空闲 30min 后自动清理临时目录
```

#### Docker Compose 配置

```yaml
code-server:
  image: codercom/code-server:latest
  environment:
    - DOCKER_USER=$USER
  volumes:
    - code-workspaces:/workspaces    # 仓库工作区
  ports:
    - "8443:8080"
  restart: unless-stopped
```

#### 安全设计

| 层面 | 措施 |
|------|------|
| 认证 | forge-core 反向代理 code-server, 校验 Forge JWT 后透传 |
| 隔离 | 工作区按 project_id + branch 隔离，用户只能访问自己有权限的项目 |
| 只读 | Phase 1 工作区为只读模式（浏览审查用途），不允许通过 IDE 直接提交 |
| 资源 | code-server 容器设置 CPU/内存限制，防止单用户耗尽资源 |

#### 使用场景

| 场景 | 入口 | 行为 |
|------|------|------|
| 审查 AI 生成代码 | 变更结果页 "在 IDE 中打开" | 打开 AI 分支，定位到变更文件 |
| 浏览项目代码 | 项目详情页 "代码浏览" | 打开默认分支，浏览完整仓库 |
| 对比分支差异 | 任务详情页 "IDE 中查看 Diff" | 打开 AI 分支，VS Code 内置 Diff |

---

## 8. IM 机器人技术设计

### 8.1 支持平台

| 平台 | 接入方式 | 实现期 |
|------|---------|--------|
| 钉钉 | 企业内部机器人 (Webhook + OAuth) | 二期 |
| 飞书 | 自建应用 (Event Subscription + OAuth) | 二期 |

### 8.2 交互流程

```
用户 @forge 或私聊:
1. forge-bot 接收消息 (Webhook)
2. 解析意图:
   ├── 需求提交: @forge + 需求描述
   ├── 查询进度: @forge 查看任务 xxx
   └── 紧急停止: @forge 停止
3. 通过 Temporal Client 触发/查询任务
4. 推送结果:
   ├── 需求确认卡片 (结构化理解 + 确认按钮)
   ├── 进度卡片 (进度条 + 当前步骤 + 耗时)
   ├── 审批卡片 (Review 评分 + 风险 + 批准/拒绝按钮)
   └── 完成通知 (变更摘要 + MR 链接)
```

### 8.3 架构

```
forge-bot (Go 轻量服务):
├── 独立部署 (2 replicas)
├── HTTP Server: 接收 IM Webhook
├── Temporal Client: 触发/查询 Workflow
├── IM SDK: 钉钉 SDK + 飞书 SDK
└── 消息模板: 卡片消息 JSON 模板
```

---

## 9. 外部平台适配器技术设计

### 9.1 适配器抽象

```go
// 代码托管适配器
type CodeHostingAdapter interface {
    // 仓库
    ListRepos(ctx context.Context) ([]Repository, error)
    GetRepo(ctx context.Context, repoID string) (*Repository, error)

    // 文件操作
    GetFileContent(ctx context.Context, repoID, path, ref string) ([]byte, error)
    GetTree(ctx context.Context, repoID, ref string) ([]TreeEntry, error)

    // 分支
    CreateBranch(ctx context.Context, repoID, name, from string) error
    DeleteBranch(ctx context.Context, repoID, name string) error
    ListBranches(ctx context.Context, repoID string) ([]Branch, error)

    // 提交
    CommitFiles(ctx context.Context, repoID, branch, message string, files []FileChange) (*Commit, error)

    // Merge Request
    CreateMR(ctx context.Context, req CreateMRRequest) (*MergeRequest, error)
    MergeMR(ctx context.Context, repoID string, mrID int) error
    GetMRDiff(ctx context.Context, repoID string, mrID int) ([]FileDiff, error)

    // Webhook
    CreateWebhook(ctx context.Context, repoID string, config WebhookConfig) error

    // Git 历史
    GetCommitLog(ctx context.Context, repoID, ref string, limit int) ([]Commit, error)
    GetDiff(ctx context.Context, repoID, from, to string) ([]FileDiff, error)
}

// 容器编排适配器
type ContainerAdapter interface {
    CreateNamespace(ctx context.Context, name string) error
    ApplyManifest(ctx context.Context, namespace string, manifest []byte) error
    GetPodStatus(ctx context.Context, namespace, name string) (*PodStatus, error)
    GetServiceURL(ctx context.Context, namespace, name string) (string, error)
    DeleteNamespace(ctx context.Context, name string) error
}

// CI/CD 适配器
type CICDAdapter interface {
    // 流水线
    CreatePipeline(ctx context.Context, config PipelineConfig) (*Pipeline, error)
    TriggerPipeline(ctx context.Context, pipelineID string, params map[string]string) (*PipelineRun, error)
    GetPipelineStatus(ctx context.Context, runID string) (*PipelineRun, error)
    GetPipelineLogs(ctx context.Context, runID string) (io.Reader, error)
    CancelPipeline(ctx context.Context, runID string) error

    // 产物
    GetArtifacts(ctx context.Context, runID string) ([]Artifact, error)
}
// 首批实现: ArgoWorkflowsAdapter
// 未来扩展: GitHubActionsAdapter, JenkinsAdapter, GitLabCIAdapter
```

### 9.2 首批实现

| 适配器 | 平台 | 实现期 |
|--------|------|--------|
| GitHubAdapter | GitHub API v3/v4 | 一期 |
| CodeupAdapter | 阿里云 Codeup API | 一期 |
| K8sAdapter | Kubernetes API | 一期 |

设计原则：
- 能力导向抽象（不是 vendor API 结构）
- 最小公共能力集（平台无关）
- 每项目绑定一个代码平台（生命周期内一致）
- 统一凭证管理（SOPS + age 加密, K8s Secret 存储）
- 适配器内置限流 + 重试（对上层透明）

---

## 9.5 CLI 入口设计

CLI 作为开发者本地使用的入口, 通过 API Token 鉴权直接调用 forge-core API。

```
forge-cli (Go 编译为单二进制)
├── forge login          → 获取 API Token, 存储到 ~/.forge/credentials
├── forge projects       → 列出项目
├── forge task create    → 提交需求 (交互式或 --requirement 参数)
├── forge task status    → 查看任务状态 (支持 --watch 实时流式)
├── forge task list      → 列出任务
├── forge killswitch     → 触发紧急停止 (需 PLATFORM_ADMIN 权限)
└── forge config         → 管理本地配置

技术实现:
├── Go cobra CLI 框架
├── 鉴权: auth.active_tokens (token_type = 'CLI_TOKEN')
├── 实时输出: SSE 订阅 → 终端流式渲染
└── 分发: GitHub Releases + Homebrew + 直接下载
```

---

## 9.6 产品加速库 (forge-foundation)

AI 孵化产品的可选组件库, 提供开箱即用的基础设施和业务能力。

### 基础设施层

| 组件 | 能力 | 说明 |
|------|------|------|
| forge-foundation-web | 统一响应 + 异常处理 | Result[T], ErrorCode, 分页, 断言工具 |
| forge-foundation-data | 数据库集成 | GORM + 多数据源 + Migration |
| forge-foundation-cache | 缓存集成 | Redis 客户端 + 分布式锁 |
| forge-foundation-mq | 消息队列抽象 | 统一 API, Kafka/RocketMQ/NATS 可切换 |
| forge-foundation-storage | 对象存储 | 阿里云 OSS (S3 兼容接口) |
| forge-foundation-log | 统一日志 | 结构化日志 + Loki 集成 |
| forge-foundation-metrics | 指标采集 | Prometheus metrics (三期可加 OTel 自动采集) |

### 业务能力层 (Phase 2+)

| 组件 | 能力 |
|------|------|
| forge-foundation-auth | 登录/认证 (可复用 forge-core 的 JWT 验证) |
| forge-foundation-notification | 通知中心 (邮件/短信/IM) |

**设计原则**: 所有组件都是 Go module, AI 生成的 Go 项目可通过 `go get` 直接引入。对于 Java/Python 等其他语言的 AI 生成项目, 由 AI 按规范中心的编码规范直接生成等效代码。

---

## 9.7 配置自动发布

AI 生成的业务系统配置（应用配置、环境变量等）需要经过验证后自动发布。

```
配置发布流程 (Temporal Activity):

1. AI 生成配置文件 (application.yaml, .env, etc.)
2. 配置验证:
   ├── 命名规范检查
   ├── 敏感字段检测 (密码/Key → 必须引用 K8s Secret / SOPS 加密)
   ├── 格式校验 (YAML/JSON schema validation)
   └── 值范围检查 (端口/超时/连接数等)
3. 配置写入 K8s ConfigMap / Secret
4. 触发应用热加载 (Pod restart 或 config watcher)
5. 健康检查确认配置生效
```

---

## 10. 数据架构

### 10.1 数据库总览

```
PostgreSQL (单实例 HA, 多 Database):
├── forge_main      → 业务数据 (多 Schema)
│   ├── auth        → 用户/角色/权限/租户
│   ├── engine      → 任务/步骤/检查点/模型调用
│   ├── specs       → 规范/Prompt/规则/脚手架
│   ├── pipeline    → 环境/部署/流水线
│   └── billing     → Token用量/预算
└── forge_temporal  → Temporal 专用 (Visibility + 工作流数据)
```

### 10.2 核心表设计

#### auth Schema

```sql
-- 租户
CREATE TABLE auth.tenants (
    id            BIGSERIAL PRIMARY KEY,
    name          VARCHAR(100) NOT NULL,
    code          VARCHAR(50) NOT NULL UNIQUE,
    status        VARCHAR(20) NOT NULL DEFAULT 'ACTIVE',  -- ACTIVE/SUSPENDED/ARCHIVED
    plan          VARCHAR(20) NOT NULL DEFAULT 'FREE',    -- FREE/PRO/ENTERPRISE
    config        JSONB NOT NULL DEFAULT '{}',
    token_budget  BIGINT DEFAULT 0,                       -- 月 Token 预算 (0=无限)
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- 用户
CREATE TABLE auth.users (
    id            BIGSERIAL PRIMARY KEY,
    tenant_id     BIGINT NOT NULL REFERENCES auth.tenants(id),
    username      VARCHAR(100) NOT NULL,
    email         VARCHAR(200),
    password_hash VARCHAR(255),
    display_name  VARCHAR(100),
    avatar_url    TEXT,
    status        VARCHAR(20) NOT NULL DEFAULT 'ACTIVE',
    mfa_enabled   BOOLEAN NOT NULL DEFAULT FALSE,
    mfa_secret    VARCHAR(100),
    last_login_at TIMESTAMPTZ,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(tenant_id, username)
);

-- 外部身份绑定
CREATE TABLE auth.user_identities (
    id            BIGSERIAL PRIMARY KEY,
    user_id       BIGINT NOT NULL REFERENCES auth.users(id),
    provider      VARCHAR(50) NOT NULL,       -- github/codeup/dingtalk/feishu/ldap/saml
    provider_uid  VARCHAR(200) NOT NULL,
    access_token  TEXT,                       -- SOPS/K8s Secret 加密
    refresh_token TEXT,
    token_expires TIMESTAMPTZ,
    profile       JSONB DEFAULT '{}',
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(provider, provider_uid)
);

-- 组织
CREATE TABLE auth.organizations (
    id            BIGSERIAL PRIMARY KEY,
    tenant_id     BIGINT NOT NULL REFERENCES auth.tenants(id),
    name          VARCHAR(200) NOT NULL,
    code          VARCHAR(100) NOT NULL,
    description   TEXT,
    parent_id     BIGINT REFERENCES auth.organizations(id),  -- 支持多级组织
    status        VARCHAR(20) NOT NULL DEFAULT 'ACTIVE',
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(tenant_id, code)
);

-- 角色
CREATE TABLE auth.roles (
    id            BIGSERIAL PRIMARY KEY,
    tenant_id     BIGINT NOT NULL REFERENCES auth.tenants(id),
    name          VARCHAR(100) NOT NULL,
    code          VARCHAR(50) NOT NULL,
    scope         VARCHAR(20) NOT NULL,       -- PLATFORM/ORG/PROJECT
    description   TEXT,
    is_system     BOOLEAN NOT NULL DEFAULT FALSE,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(tenant_id, code)
);

-- 权限定义
CREATE TABLE auth.permissions (
    id            BIGSERIAL PRIMARY KEY,
    code          VARCHAR(100) NOT NULL UNIQUE,
    name          VARCHAR(100) NOT NULL,
    module        VARCHAR(50) NOT NULL,
    action        VARCHAR(20) NOT NULL,
    description   TEXT
);

-- 角色-权限关联
CREATE TABLE auth.role_permissions (
    role_id       BIGINT NOT NULL REFERENCES auth.roles(id),
    permission_id BIGINT NOT NULL REFERENCES auth.permissions(id),
    PRIMARY KEY (role_id, permission_id)
);

-- 用户-角色关联
CREATE TABLE auth.user_roles (
    id            BIGSERIAL PRIMARY KEY,
    user_id       BIGINT NOT NULL REFERENCES auth.users(id),
    role_id       BIGINT NOT NULL REFERENCES auth.roles(id),
    scope         VARCHAR(20) NOT NULL,
    scope_id      BIGINT,
    granted_by    BIGINT REFERENCES auth.users(id),
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(user_id, role_id, scope, scope_id)
);

-- ABAC 策略
CREATE TABLE auth.policies (
    id            BIGSERIAL PRIMARY KEY,
    tenant_id     BIGINT NOT NULL REFERENCES auth.tenants(id),
    name          VARCHAR(100) NOT NULL,
    description   TEXT,
    effect        VARCHAR(10) NOT NULL,       -- ALLOW/DENY
    conditions    JSONB NOT NULL,
    actions       TEXT[] NOT NULL,
    priority      INT NOT NULL DEFAULT 0,
    enabled       BOOLEAN NOT NULL DEFAULT TRUE,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- 活跃 Token
CREATE TABLE auth.active_tokens (
    id            BIGSERIAL PRIMARY KEY,
    tenant_id     BIGINT NOT NULL REFERENCES auth.tenants(id),
    user_id       BIGINT NOT NULL REFERENCES auth.users(id),
    token_jti     VARCHAR(100) NOT NULL UNIQUE,
    token_type    VARCHAR(20) NOT NULL DEFAULT 'SESSION',  -- SESSION/API_TOKEN/CLI_TOKEN
    device_info   VARCHAR(200),
    ip_address    INET,
    scopes        TEXT[],                                  -- API Token 权限范围
    expires_at    TIMESTAMPTZ NOT NULL,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- 审计日志 (按月分区)
CREATE TABLE auth.audit_logs (
    id            BIGSERIAL PRIMARY KEY,
    tenant_id     BIGINT NOT NULL,
    user_id       BIGINT,
    action        VARCHAR(100) NOT NULL,
    resource_type VARCHAR(50),
    resource_id   VARCHAR(100),
    detail        JSONB,
    ip_address    INET,
    user_agent    TEXT,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
) PARTITION BY RANGE (created_at);
```

#### engine Schema

```sql
-- 项目
CREATE TABLE engine.projects (
    id              BIGSERIAL PRIMARY KEY,
    tenant_id       BIGINT NOT NULL REFERENCES auth.tenants(id),
    name            VARCHAR(200) NOT NULL,
    description     TEXT,
    status          VARCHAR(20) NOT NULL DEFAULT 'ACTIVE',
    code_platform   VARCHAR(50),
    code_repo_url   TEXT,
    code_credential VARCHAR(100),          -- K8s Secret / SOPS 加密路径
    profile         JSONB DEFAULT '{}',    -- 项目画像 (AI 分析后填充)
    profile_version INT DEFAULT 0,
    profile_updated TIMESTAMPTZ,
    default_branch  VARCHAR(100) DEFAULT 'main',
    ai_model        VARCHAR(50),
    risk_threshold  INT DEFAULT 90,
    auto_merge      BOOLEAN DEFAULT TRUE,
    created_by      BIGINT REFERENCES auth.users(id),
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(tenant_id, name)
);

-- 项目收藏
CREATE TABLE engine.project_stars (
    id              BIGSERIAL PRIMARY KEY,
    user_id         BIGINT NOT NULL REFERENCES auth.users(id),
    project_id      BIGINT NOT NULL REFERENCES engine.projects(id),
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(user_id, project_id)
);

-- 任务
CREATE TABLE engine.tasks (
    id              BIGSERIAL PRIMARY KEY,
    tenant_id       BIGINT NOT NULL,
    project_id      BIGINT NOT NULL REFERENCES engine.projects(id),
    title           VARCHAR(500),
    requirement     TEXT NOT NULL,
    source          VARCHAR(20) NOT NULL,  -- WEB/DINGTALK/FEISHU/CLI/API
    status          VARCHAR(30) NOT NULL DEFAULT 'SUBMITTED',
    -- SUBMITTED → ANALYZING → PLANNING → PLAN_CONFIRMED
    -- → GENERATING → REVIEWING → TESTING → DEPLOYING → DEPLOYED
    -- → COMPLETED / FAILED / CANCELLED
    workflow_id     VARCHAR(200),
    workflow_run_id VARCHAR(200),
    analysis        JSONB,
    task_graph      JSONB,
    risk_level      VARCHAR(10),
    risk_factors    JSONB,
    risk_score      INT,
    branch_name     VARCHAR(200),
    mr_url          TEXT,
    files_changed   INT,
    lines_added     INT,
    lines_deleted   INT,
    approved_by     BIGINT REFERENCES auth.users(id),
    approved_at     TIMESTAMPTZ,
    created_by      BIGINT NOT NULL REFERENCES auth.users(id),
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    completed_at    TIMESTAMPTZ
);

-- 任务步骤
CREATE TABLE engine.task_steps (
    id              BIGSERIAL PRIMARY KEY,
    task_id         BIGINT NOT NULL REFERENCES engine.tasks(id),
    name            VARCHAR(200) NOT NULL,
    step_type       VARCHAR(30) NOT NULL,
    status          VARCHAR(20) NOT NULL DEFAULT 'PENDING',
    child_workflow_id VARCHAR(200),
    activity_id     VARCHAR(200),
    input           JSONB,
    output          JSONB,
    error           JSONB,
    attempt         INT NOT NULL DEFAULT 1,
    max_attempts    INT NOT NULL DEFAULT 3,
    depends_on      BIGINT[],
    started_at      TIMESTAMPTZ,
    completed_at    TIMESTAMPTZ,
    duration_ms     BIGINT
);

-- 检查点
CREATE TABLE engine.task_checkpoints (
    id              BIGSERIAL PRIMARY KEY,
    task_id         BIGINT NOT NULL REFERENCES engine.tasks(id),
    step_id         BIGINT REFERENCES engine.task_steps(id),
    checkpoint_type VARCHAR(30) NOT NULL,
    state_snapshot  JSONB NOT NULL,
    artifacts       JSONB,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- AI 模型调用记录 (按月分区, append-only)
CREATE TABLE engine.model_calls (
    id              BIGSERIAL PRIMARY KEY,
    tenant_id       BIGINT NOT NULL,
    task_id         BIGINT REFERENCES engine.tasks(id),
    step_id         BIGINT REFERENCES engine.task_steps(id),
    model           VARCHAR(50) NOT NULL,
    provider        VARCHAR(30) NOT NULL,
    purpose         VARCHAR(30) NOT NULL,
    input_tokens    INT NOT NULL,
    output_tokens   INT NOT NULL,
    total_tokens    INT NOT NULL,
    cost_cents      INT NOT NULL,
    latency_ms      INT NOT NULL,
    status          VARCHAR(10) NOT NULL,
    error_code      VARCHAR(50),
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
) PARTITION BY RANGE (created_at);

-- AI 对话历史
CREATE TABLE engine.conversations (
    id              BIGSERIAL PRIMARY KEY,
    task_id         BIGINT NOT NULL REFERENCES engine.tasks(id),
    role            VARCHAR(20) NOT NULL,
    content         TEXT NOT NULL,
    tool_calls      JSONB,
    tokens_used     INT,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Review 结果
CREATE TABLE engine.review_results (
    id              BIGSERIAL PRIMARY KEY,
    task_id         BIGINT NOT NULL REFERENCES engine.tasks(id),
    step_id         BIGINT REFERENCES engine.task_steps(id),
    review_type     VARCHAR(20) NOT NULL,
    reviewer        VARCHAR(100),
    score           INT,
    passed          BOOLEAN NOT NULL,
    findings        JSONB NOT NULL DEFAULT '[]',
    summary         TEXT,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
```

#### specs Schema

```sql
-- 编码规范
CREATE TABLE specs.standards (
    id              BIGSERIAL PRIMARY KEY,
    tenant_id       BIGINT NOT NULL,
    name            VARCHAR(200) NOT NULL,
    category        VARCHAR(50) NOT NULL,
    scope           VARCHAR(20) NOT NULL,
    scope_id        BIGINT,
    parent_id       BIGINT REFERENCES specs.standards(id),
    content         TEXT NOT NULL,
    version         INT NOT NULL DEFAULT 1,
    status          VARCHAR(20) NOT NULL DEFAULT 'ACTIVE',
    created_by      BIGINT REFERENCES auth.users(id),
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(tenant_id, category, scope, scope_id)
);

-- Prompt 模板
CREATE TABLE specs.prompt_templates (
    id              BIGSERIAL PRIMARY KEY,
    tenant_id       BIGINT NOT NULL,
    name            VARCHAR(200) NOT NULL,
    purpose         VARCHAR(50) NOT NULL,
    system_prompt   TEXT NOT NULL,
    user_template   TEXT NOT NULL,
    variables       JSONB NOT NULL DEFAULT '[]',
    version         INT NOT NULL DEFAULT 1,
    is_default      BOOLEAN NOT NULL DEFAULT FALSE,
    eval_cases      JSONB DEFAULT '[]',
    last_eval_score FLOAT,
    last_eval_at    TIMESTAMPTZ,
    created_by      BIGINT REFERENCES auth.users(id),
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Review 规则
CREATE TABLE specs.review_rules (
    id              BIGSERIAL PRIMARY KEY,
    tenant_id       BIGINT NOT NULL,
    name            VARCHAR(200) NOT NULL,
    category        VARCHAR(50) NOT NULL,
    scope           VARCHAR(20) NOT NULL,
    scope_id        BIGINT,
    rule_type       VARCHAR(20) NOT NULL,
    definition      JSONB NOT NULL,
    severity        VARCHAR(10) NOT NULL,
    auto_fix        BOOLEAN DEFAULT FALSE,
    fix_template    TEXT,
    enabled         BOOLEAN NOT NULL DEFAULT TRUE,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- 脚手架模板
CREATE TABLE specs.scaffold_templates (
    id              BIGSERIAL PRIMARY KEY,
    tenant_id       BIGINT NOT NULL,
    name            VARCHAR(200) NOT NULL,
    project_type    VARCHAR(50) NOT NULL,
    description     TEXT,
    template_repo   TEXT,
    variables       JSONB NOT NULL DEFAULT '[]',
    post_hooks      JSONB DEFAULT '[]',
    version         INT NOT NULL DEFAULT 1,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
```

#### pipeline Schema

```sql
-- 环境定义
CREATE TABLE pipeline.environments (
    id              BIGSERIAL PRIMARY KEY,
    tenant_id       BIGINT NOT NULL,
    project_id      BIGINT NOT NULL REFERENCES engine.projects(id),
    name            VARCHAR(100) NOT NULL,
    env_type        VARCHAR(20) NOT NULL,
    cluster_id      VARCHAR(100),
    namespace       VARCHAR(100),
    auto_deploy     BOOLEAN NOT NULL DEFAULT FALSE,
    requires_approval BOOLEAN NOT NULL DEFAULT FALSE,
    canary_enabled  BOOLEAN NOT NULL DEFAULT FALSE,
    canary_config   JSONB,
    status          VARCHAR(20) NOT NULL DEFAULT 'ACTIVE',
    current_version VARCHAR(100),
    last_deploy_at  TIMESTAMPTZ,
    branch_name     VARCHAR(200),
    expires_at      TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- 部署记录
CREATE TABLE pipeline.deployments (
    id              BIGSERIAL PRIMARY KEY,
    tenant_id       BIGINT NOT NULL,
    project_id      BIGINT NOT NULL,
    environment_id  BIGINT NOT NULL REFERENCES pipeline.environments(id),
    task_id         BIGINT REFERENCES engine.tasks(id),
    version         VARCHAR(100) NOT NULL,
    strategy        VARCHAR(20) NOT NULL,
    status          VARCHAR(20) NOT NULL DEFAULT 'PENDING',
    argo_workflow   VARCHAR(200),
    argo_app        VARCHAR(200),
    argo_rollout    VARCHAR(200),
    build_log_url   TEXT,
    health_check    JSONB,
    rollback_reason TEXT,
    triggered_by    BIGINT REFERENCES auth.users(id),
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    completed_at    TIMESTAMPTZ
);

-- 测试执行记录
CREATE TABLE pipeline.test_executions (
    id              BIGSERIAL PRIMARY KEY,
    tenant_id       BIGINT NOT NULL,
    task_id         BIGINT NOT NULL REFERENCES engine.tasks(id),
    deployment_id   BIGINT REFERENCES pipeline.deployments(id),
    test_layer      VARCHAR(20) NOT NULL,
    tool            VARCHAR(50) NOT NULL,
    status          VARCHAR(20) NOT NULL,
    total_cases     INT,
    passed_cases    INT,
    failed_cases    INT,
    skipped_cases   INT,
    coverage_pct    FLOAT,
    report_url      TEXT,
    failures        JSONB DEFAULT '[]',
    started_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    completed_at    TIMESTAMPTZ,
    duration_ms     BIGINT
);

-- 质量门禁记录
CREATE TABLE pipeline.quality_gates (
    id              BIGSERIAL PRIMARY KEY,
    task_id         BIGINT NOT NULL REFERENCES engine.tasks(id),
    deployment_id   BIGINT REFERENCES pipeline.deployments(id),
    gate_type       VARCHAR(30) NOT NULL,
    passed          BOOLEAN NOT NULL,
    score           INT,
    threshold       INT,
    detail          JSONB,
    checked_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
```

#### billing Schema

```sql
-- 租户月度预算
CREATE TABLE billing.tenant_budgets (
    id              BIGSERIAL PRIMARY KEY,
    tenant_id       BIGINT NOT NULL REFERENCES auth.tenants(id),
    year_month      VARCHAR(7) NOT NULL,
    budget_tokens   BIGINT NOT NULL,
    budget_cents    INT NOT NULL,
    used_tokens     BIGINT NOT NULL DEFAULT 0,
    used_cents      INT NOT NULL DEFAULT 0,
    warning_sent    BOOLEAN DEFAULT FALSE,
    hard_limit_hit  BOOLEAN DEFAULT FALSE,
    UNIQUE(tenant_id, year_month)
);

-- KillSwitch 事件
CREATE TABLE billing.killswitch_events (
    id              BIGSERIAL PRIMARY KEY,
    tenant_id       BIGINT,
    level           VARCHAR(5) NOT NULL,
    action          VARCHAR(10) NOT NULL,
    trigger_type    VARCHAR(20) NOT NULL,
    trigger_reason  TEXT,
    activated_by    BIGINT REFERENCES auth.users(id),
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
```

### 10.3 索引策略

```sql
CREATE INDEX idx_tasks_tenant_status ON engine.tasks(tenant_id, status);
CREATE INDEX idx_tasks_project_status ON engine.tasks(project_id, status);
CREATE INDEX idx_tasks_created_by ON engine.tasks(created_by, created_at DESC);
CREATE INDEX idx_task_steps_task ON engine.task_steps(task_id, status);
CREATE INDEX idx_model_calls_tenant_month ON engine.model_calls(tenant_id, created_at);
CREATE INDEX idx_model_calls_task ON engine.model_calls(task_id);
CREATE INDEX idx_deployments_project_env ON pipeline.deployments(project_id, environment_id, created_at DESC);
CREATE INDEX idx_audit_logs_tenant_time ON auth.audit_logs(tenant_id, created_at DESC);
CREATE INDEX idx_user_roles_user ON auth.user_roles(user_id);
CREATE INDEX idx_review_results_task ON engine.review_results(task_id);
CREATE INDEX idx_tasks_requirement_fts ON engine.tasks USING gin(to_tsvector('simple', requirement));
```

### 10.4 分区策略

```sql
-- 高增长表按月分区, pg_partman 管理
-- engine.model_calls: 在线保留 6 个月, 冷数据导出到阿里云 OSS (Parquet 格式)
-- auth.audit_logs: 在线保留 12 个月
```

### 10.5 Redis 数据结构

```
# 会话管理
session:{user_id}:{jti}                    → JSON    TTL=JWT有效期

# Token 黑名单
token:blacklist:{jti}                      → "1"     TTL=JWT剩余有效期

# KillSwitch
killswitch:global                          → "L0|L1|L2|L3"
killswitch:tenant:{tenant_id}             → "L0|L1|L2|L3"

# 限流
ratelimit:user:{user_id}:min              → counter  TTL=60s
ratelimit:tenant:{tenant_id}:min          → counter  TTL=60s

# 规范缓存
specs:effective:{project_id}:{category}   → JSON     TTL=10min

# 任务实时状态
task:status:{task_id}                     → JSON     TTL=24h

# 项目画像缓存
project:profile:{project_id}              → JSON     TTL=30min

# 全局并发控制
ai:concurrent:count                       → counter
ai:concurrent:limit                       → int
```

### 10.6 搜索设计 (PostgreSQL FTS, 替代 Elasticsearch)

```
项目搜索 — PostgreSQL tsvector + GIN 索引
├── projects 表: name, description, tech_stack 列建 tsvector
├── 支持中文分词: pg_jieba 或 zhparser 扩展
└── 查询: SELECT * FROM projects WHERE tsv @@ plainto_tsquery('关键词')

代码搜索 — AI 语义搜索 + GitHub API
├── 简单关键词: GitHub Code Search API (已接入)
├── 语义搜索: Claude 分析项目结构, 定位相关代码
└── 不需要本地索引代码内容

Temporal Visibility — Temporal 自带 PostgreSQL Visibility Store
├── Temporal Server 原生支持 PG 作为 Visibility 后端
└── 无需额外 Elasticsearch
```

> **为什么不用 Elasticsearch**: Phase 1 数据量级 (百级项目, 千级任务) 用 PG FTS 完全够用,
> ES 集群需要 2GB+ heap + 运维成本高。AI 语义搜索比关键词搜索更准确。后期如需扩展可评估 Meilisearch (Apache 2.0, 更轻量)。

---

## 11. 部署架构

### 11.1 K8s 集群拓扑

```
Internet ──── CLB/NLB ──┬─ CDN (静态资源)
                        │
                        └─ Traefik Ingress Controller (统一入口, JWT/限流/灰度)
                           ├── portal.forge.internal    → Next.js
                           ├── api.forge.internal       → forge-core (Go API)
                           ├── temporal.forge.internal  → Temporal Web UI
                           ├── grafana.forge.internal   → Grafana
                           ├── argo.forge.internal      → Argo CD UI
                           └── ide.forge.internal       → code-server (Web IDE)
```

### 11.2 Namespace 划分

| Namespace | 组件 | 副本数 |
|-----------|------|--------|
| **forge-system** | forge-core + forge-portal + ai-worker + temporal-server + postgresql + redis | 核心平台服务 |
| **forge-jobs** | task-{id}-build / task-{id}-test / task-{id}-deploy (K8s Job, 临时) | 动态 |
| **forge-ingress** | Traefik (3, 统一入口+JWT+限流+灰度) | 3 |
| **forge-workers** | ai-worker (3~20 KEDA) + devops-worker (3~15 KEDA) + constraint-worker (2~10 KEDA) | 8~45 |
| **forge-temporal** | Frontend (3) + History (3) + Matching (3) + Worker (2) + Web UI (1) | 12 |
| **forge-cicd** | Argo Workflows Controller (2) + Server (2) + Argo CD (3+2+1) + Rollouts (1) | 11 |
| **forge-ide** | code-server (1~3 HPA) | 1~3 |
| **forge-observability** | Grafana (2) + Loki (3) + Prometheus (2) + Alertmanager (3) | 10 |
| **forge-data** | PostgreSQL 主从 (1+2) + Redis Sentinel (3) | 6 |
| **tenant-{id}-dev** | 用户业务服务 Pod（开发环境） | 按租户 |
| **tenant-{id}-staging** | 用户业务服务 Pod（预发环境） | 按租户 |
| **tenant-{id}-prod** | 用户业务服务 Pod（生产环境） | 按租户 |

> **精简对比**: 去掉 forge-gateway (APISIX+etcd 6 副本)、forge-quality (SonarQube+MeterSphere 2 副本)、
> forge-secrets (Vault 3 副本)，数据层去掉 Elasticsearch (3) + MinIO (4)。总计减少 **18 个 Pod**。

### 11.3 节点规划

| 节点池 | 数量 | 规格 | 用途 |
|--------|------|------|------|
| Master | 3 | 4C8G | K8s 控制面 |
| App | 3~10 | 8C16G | forge-core, portal, bot, Traefik |
| Worker | 3~20 | 8C32G | AI/DevOps/Constraint Worker (弹性) |
| Data | 3 | 8C32G, SSD | PostgreSQL, Redis |
| Infra | 3 | 8C16G | Temporal, Argo, Grafana, Loki, Prometheus |

**基线 15 节点, 弹性至 39 节点** (Worker 池根据 AI 任务量伸缩)

> 比旧架构减少 Storage 节点池 (4 节点) — MinIO 替换为阿里云 OSS / 本地 FS。

### 11.4 资源配置

```yaml
# forge-core
resources:
  requests: { cpu: 500m, memory: 512Mi }
  limits:   { cpu: "2", memory: 2Gi }
replicas: 3
hpa: { minReplicas: 3, maxReplicas: 10, targetCPU: 70% }
pdb: { minAvailable: 2 }

# ai-worker
resources:
  requests: { cpu: "1", memory: 1Gi }
  limits:   { cpu: "4", memory: 4Gi }
replicas: 3
keda: { trigger: temporal-queue-depth, threshold: 5, min: 3, max: 20 }
terminationGracePeriodSeconds: 300  # 等待当前 Activity 完成

# constraint-worker
resources:
  requests: { cpu: "1", memory: 1Gi }
  limits:   { cpu: "4", memory: 4Gi }  # Lint 吃资源
replicas: 2
keda: { trigger: temporal-queue-depth, threshold: 3, min: 2, max: 10 }
```

### 11.5 网络架构

```
NetworkPolicy 规则:
├── forge-ingress → forge-app (允许)
├── forge-app → forge-data (允许)
├── forge-app → forge-temporal (允许)
├── forge-workers → forge-temporal (允许)
├── forge-workers → forge-data (允许)
├── forge-workers → 外部 API (允许, 通过 Egress: AI API, GitHub API)
├── forge-cicd → forge-target-* (允许)
├── forge-observability → ALL (允许, 采集)
└── 其余 → 默认拒绝

CNI: Calico 或 Cilium
```

### 11.6 存储架构

```
块存储 (K8s PV, StorageClass):
├── SSD: PG data, Redis AOF
└── HDD: Loki chunks

对象存储 (阿里云 OSS, S3 兼容; 本地开发用文件系统):
├── /forge-artifacts    → 代码产物/Diff
├── /forge-test-reports → 测试报告
├── /forge-backups      → PG 备份/WAL
├── /forge-loki         → Loki 日志数据
└── /forge-temp         → 临时文件 (TTL 7d)

备份策略:
├── PostgreSQL: 每日全量 + WAL 持续归档 → 阿里云 OSS (保留 7 日全量 + 30 日 WAL)
├── Redis: AOF + 每日 RDB snapshot
└── 灾难恢复: RPO < 5min, RTO < 30min
```

### 11.7 弹性伸缩

| 组件 | 方式 | 触发指标 | 范围 |
|------|------|---------|------|
| forge-core | HPA | CPU > 70% | 3 → 10 |
| forge-portal | HPA | CPU > 60% | 3 → 8 |
| ai-worker | KEDA | Temporal queue depth > 5 | 3 → 20 |
| devops-worker | KEDA | Temporal queue depth > 3 | 3 → 15 |
| constraint-worker | KEDA | Temporal queue depth > 3 | 2 → 10 |
| Worker 节点池 | Cluster Autoscaler | Pod pending | 3 → 20 |

**KEDA 是关键**: 传统 HPA 基于 CPU/Memory, Worker 负载由 Temporal 队列深度决定, KEDA 直接监听队列精准伸缩。

### 11.8 部署流水线 (GitOps)

```
开发者 push 代码
      │
      ▼
GitHub Actions / Argo Workflows
├── Build: Docker multi-stage build
├── Test: Unit + Integration
├── Scan: Trivy (镜像漏洞扫描)
├── Push: 镜像 → Harbor/ACR
└── Update: Kustomize overlay → Git commit
      │
      ▼
Argo CD (自动同步)
├── dev:     auto-sync, self-heal
├── staging: auto-sync, self-heal
└── prod:    manual sync + Argo Rollouts canary
               ├── 5% → 观察 5min
               ├── 25% → 观察 5min
               ├── 50% → 观察 5min
               └── 100% (或自动回滚)
```

### 11.9 监控告警

| 级别 | 触发条件 | 通知方式 |
|------|---------|---------|
| P0 Critical | PG 主库不可用 / Temporal 全部不可用 / KillSwitch L3 | 电话 + 钉钉/飞书 |
| P1 High | AI Worker 全部不可用 / 队列积压 > 100 / API 5xx > 5% | 钉钉/飞书 + 邮件 |
| P2 Medium | PG 复制延迟 > 10s / 磁盘 > 80% / Token 预算 > 80% | 钉钉/飞书 |
| P3 Low | Pod 重启 > 3次/小时 / 证书过期 < 30天 | 邮件 |

### 11.10 本地开发环境

```yaml
# docker-compose.dev.yml — 精简版, 只需 3 个容器
services:
  postgres:       # 单实例, 包含所有 Schema + FTS
  redis:          # 单实例
  temporal:       # temporal-dev-server (all-in-one, 内置 PG Visibility)

# 开发者启动:
# 1. docker compose -f docker-compose.dev.yml up -d
# 2. go run ./cmd/forge-core          (Go API)
# 3. cd ai-worker && python main.py   (Python Worker)
# 4. cd forge-portal && npm run dev    (Next.js)
# 产物存储: 本地文件系统 (data/ 目录)
```

### 11.11 SaaS 基础设施架构

> 详细设计见 [infra-architecture-design.md](plans/infra-architecture-design.md)

#### 11.11.1 K8s 集群拓扑

单集群多 namespace 架构，通过 namespace 实现平台服务、任务容器、用户业务的隔离：

```
ACK 集群（单集群）
├── forge-system        — 平台核心服务（forge-core, portal, ai-worker, temporal, pg, redis）
├── forge-jobs          — 任务容器（K8s Job: test/build/deploy，临时创建/自动清理）
├── tenant-{id}-dev     — 租户开发环境（用户业务服务 Pod）
├── tenant-{id}-staging — 租户预发环境
└── tenant-{id}-prod    — 租户生产环境
```

#### 11.11.2 任务容器 K8s Job Spec

每个任务步骤（测试/构建/部署）在独立 K8s Job 中执行：

```yaml
apiVersion: batch/v1
kind: Job
metadata:
  name: task-{taskId}-{step}
  namespace: forge-jobs
  labels:
    forge.io/tenant: "{tenantId}"
    forge.io/task: "{taskId}"
    forge.io/step: "test|build|deploy"
spec:
  ttlSecondsAfterFinished: 300  # 完成后 5 分钟清理
  activeDeadlineSeconds: 1800   # 30 分钟超时
  template:
    spec:
      containers:
      - name: worker
        image: forge-task-runner:latest
        resources:
          requests: { cpu: "500m", memory: "1Gi" }
          limits:   { cpu: "2",    memory: "4Gi" }
        env:
        - name: GIT_TOKEN
          valueFrom:
            secretKeyRef: { name: task-{taskId}-creds, key: git-token }
        volumeMounts:
        - name: workspace
          mountPath: /workspace
      volumes:
      - name: workspace
        emptyDir: { sizeLimit: "10Gi" }
      restartPolicy: Never
```

容器内执行流程: git clone → 写入 AI 生成文件（从 Redis 获取）→ lint → test → docker build → git push + 创建 PR。

#### 11.11.3 Git 认证架构

混合模式: GitHub App（平台级）+ OAuth（用户级）。

```
Token 选择决策树:
1. 仓库所属 Org 安装了 Forge GitHub App → Installation Token（最稳定）
2. 操作用户有 OAuth Token → 用户 OAuth Token
3. 都没有 → 报错 "请先关联代码平台"
```

**GitHub App Token 生命周期**:
1. Forge 启动时用 App 私钥签名生成 JWT（10min 有效）
2. 用 JWT 获取 Installation Access Token（1h 有效）
3. Token 缓存在 Redis: `github:installation:{installationId}`
4. 过期前 5 分钟自动刷新
5. 每个 Installation（Org）独立 Token

**GitHub App 权限**:
- Repository: Contents (R/W), Pull requests (R/W), Metadata (R), Webhooks (R/W), Actions (R)
- Organization: Members (R)
- Events: Push, Pull request, Create

**Forge 托管仓库**: `forge-managed/{tenant-slug}-{project-slug}`，GitHub App 安装到 `forge-managed` Org。

#### 11.11.4 多租户资源隔离

```yaml
# 每个租户 namespace 的 ResourceQuota
apiVersion: v1
kind: ResourceQuota
metadata:
  name: tenant-quota
  namespace: tenant-{id}-dev
spec:
  hard:
    requests.cpu: "4"
    requests.memory: "8Gi"
    limits.cpu: "8"
    limits.memory: "16Gi"
    pods: "20"
    services: "10"
    persistentvolumeclaims: "5"
```

NetworkPolicy 规则:
- `forge-jobs` → 外部 API（GitHub, AI API）: 允许
- `forge-jobs` → `forge-system`: 拒绝（任务容器不可访问平台服务）
- `tenant-{id}-*` → `tenant-{id}-*`: 同租户环境间允许
- `tenant-{id}-*` → `tenant-{j}-*`: 跨租户拒绝

#### 11.11.5 数据流: AI 生成 → 部署

```
用户提需求 → AI 分析+规划+生成代码 (ai-worker, 内存中)
  → 代码暂存 Redis: code:task:{taskId} (TTL 1h)
  → 启动 K8s Job: task-{taskId}-build
     ├── 从 Redis 获取生成的代码
     ├── git clone 仓库 + 写入文件 + git commit + push
     ├── 创建 PR
     ├── 运行测试（在容器内）
     ├── 构建 Docker 镜像 → push ACR
     └── 汇报结果到 Forge API
  → Forge 更新任务状态 + SSE 推送前端
  → 审批通过 → 合并 PR → 触发部署
  → 启动 K8s Job: task-{taskId}-deploy
     ├── kubectl apply 到 tenant-{id}-{env}
     └── 健康检查
```

#### 11.11.6 实施阶段

| 阶段 | 内容 | 说明 |
|------|------|------|
| 阶段 1（当前） | GitHub API 模式 | AI 生成代码通过 GitHub API 推送，不需要 K8s Job |
| 阶段 2 | K8s Job 任务容器 | 接入 ACK，测试/构建在 K8s Job 中执行 |
| 阶段 3 | 完整部署流水线 | 用户代码部署到 K8s，多环境管理，PR 预览环境 |
| 阶段 4 | GitHub App | 替代纯 OAuth，Installation Token 自动管理 |

---

## 12. 高可用设计

| 故障场景 | 应对策略 |
|---------|---------|
| 单 Pod 崩溃 | K8s 自动重启 + PDB 保证最少副本 |
| 单 Node 宕机 | Pod 重新调度, PDB 保证服务不中断 |
| AZ 故障 | 节点池跨 3 AZ, Pod 反亲和性跨 AZ |
| PG 主库故障 | CloudNativePG 自动 failover (< 30s) |
| Redis 主节点故障 | Sentinel 自动选举新主 (< 15s) |
| Temporal Server 故障 | 多节点部署, Frontend/History/Matching 独立扩展 |
| AI Worker 中途崩溃 | Temporal Activity heartbeat 超时 → 重分配, 从 Checkpoint 恢复 |
| 外部 API 故障 | 多模型降级链 + 适配器内置重试+熔断 |
| 全集群灾难 | PG WAL → 阿里云 OSS 恢复, RPO < 5min |

失败处理原则 (No Silent Failures):
- AI 代码生成失败: 3 轮自动修复, 超限升级人工
- Pipeline 构建失败: AI 分析日志自动修复 (最多 3 轮)
- 部署失败: 自动回滚一次, 然后通知
- 外部 API 失败: 3 次退避重试, 然后暂停任务
- **所有通知必须包含**: 失败阶段、原因、已完成工作、下一步建议

---

## 13. 技术选型总览

### 13.1 应用层

| 组件 | 技术 | 版本 | 用途 |
|------|------|------|------|
| API Server | Go + Gin | Go 1.22+ | 统一 API 入口 |
| AI Worker | Python + LangGraph | Python 3.12+ | AI 编排 |
| DevOps Worker | Go | Go 1.22+ | CI/CD + 部署 |
| Constraint Worker | Go | Go 1.22+ | Lint + 安全扫描 |
| IM Bot | Go | Go 1.22+ | 钉钉/飞书机器人 |
| Web Portal | Next.js + React | Next.js 15 | Web 工作台 |

### 13.2 基础设施

| 组件 | 技术 | 用途 | 替代说明 |
|------|------|------|---------|
| 工作流引擎 | Temporal | 状态管理/Checkpoint/编排/恢复 | — |
| 统一入口 | Traefik | TLS + 负载均衡 + JWT 验证 + 限流 + 灰度路由 | 替代 APISIX + etcd (减少 2 组件) |
| 数据库 | PostgreSQL (CloudNativePG) | 业务数据 + 全文搜索 (tsvector) | PG FTS 替代 Elasticsearch |
| 缓存 | Redis Sentinel / Valkey | 会话/缓存/KillSwitch | Valkey 为 Redis 的 Linux Foundation fork |
| 对象存储 | 阿里云 OSS (本地: 文件系统) | 产物/报告/备份 | 替代 MinIO (省 4 节点 + AGPL 风险) |
| 密钥管理 | SOPS + age / K8s Secret | API Key/凭证加密 | 替代 Vault (省 3 节点 + BSL 许可风险) |

### 13.3 DevOps

| 组件 | 技术 | 用途 | 替代说明 |
|------|------|------|---------|
| CI Pipeline | GitHub Actions / Argo Workflows | 构建/测试编排 | Phase 1 优先用 GitHub Actions |
| CD GitOps | Argo CD | 声明式部署 | — |
| 灰度发布 | Argo Rollouts | Canary/BlueGreen | — |
| Lint | 语言原生 Linter (golangci-lint/eslint/ruff) | 格式化 + 静态检查 | 替代 MegaLinter (省 2GB+ Docker 镜像) |
| 安全扫描 | Semgrep | 安全规则匹配 | — |
| 代码质量 | Claude Reviewer Agent | 语义级质量分析 + Judge 评分 | AI 替代 SonarQube (省 4GB+ Java 应用) |
| 自动化测试 | Claude TestGen Agent + 原生框架 | 测试生成 + 运行 | AI 替代 MeterSphere (省 Java 重型平台) |

### 13.4 可观测性

| 组件 | 技术 | 用途 |
|------|------|------|
| Dashboard | Grafana | 统一可视化 |
| 日志 | Loki | 日志聚合 |
| 指标 | Prometheus + Alertmanager | 指标采集 + 告警 |
| 工作流可视化 | Temporal Web UI | Workflow 状态/调试 |
| 全栈监控 (三期评估) | DeepFlow (eBPF) 或 Grafana Alloy + OTel | AI 生成产品运行时监控 |

### 13.5 前端

| 组件 | 技术 | 用途 |
|------|------|------|
| 框架 | Next.js 15 (App Router) | SSR + RSC |
| UI | shadcn/ui + Radix | 组件库 |
| 样式 | Tailwind CSS 4 | 原子化 CSS |
| 状态 | Zustand + TanStack Query | 客户端/服务端状态 |
| 图表 | Recharts | 数据可视化 |
| 代码 | Monaco Editor + react-diff-viewer | 代码/Diff 展示 |
| 表单 | React Hook Form + Zod | 表单验证 |
| 字体 | Geist Sans + Geist Mono | — |
| 图标 | Lucide Icons | — |

### 13.6 AI 替代中间件策略

Forge 作为 AI 驱动平台，核心洞察是: **Claude/GPT 的语义理解能力可以替代多个传统中间件**。

#### 替代原则

| 适合中间件的任务 | 适合 AI 的任务 |
|----------------|--------------|
| 确定性、高频、无需语义理解 | 需要语义理解、上下文感知 |
| 格式化 (gofmt/prettier) | 代码 Review (理解业务意图) |
| 安全规则匹配 (Semgrep) | 测试设计 (根据代码逻辑生成针对性测试) |
| 指标采集 (Prometheus) | 代码搜索 (语义级定位) |
| 日志聚合 (Loki) | 质量判断 (复杂度是否合理需要上下文) |

#### 被替代组件清单

| 被替代 | 替代方案 | 节省 |
|--------|---------|------|
| SonarQube | Claude Reviewer Agent | 4GB+ RAM, Java 应用, 社区版无分支分析 |
| MeterSphere | Claude TestGen Agent + 原生测试框架 | Java 重型应用, 开源版功能缩水 |
| Elasticsearch | PostgreSQL FTS + Claude 语义搜索 | 2GB+ heap, SSPL 许可风险 |
| MegaLinter | 语言原生 Linter 直接调用 | 2GB+ Docker 镜像, 50+ Linter 但只需 2~3 个 |
| APISIX + etcd | Traefik (已有) + Gin middleware | 6 Pod (3 APISIX + 3 etcd), 社区活跃度下降 |
| MinIO | 阿里云 OSS / 本地 FS | 4 节点, AGPL 许可风险 |
| HashiCorp Vault | SOPS + age + K8s Secret | 3 节点, BSL 许可 (非开源) |

#### 总计节省

```
旧架构基础设施 Pod 数:  ~75 (19 节点)
新架构基础设施 Pod 数:  ~44 (15 节点)
减少: 31 Pod, 4 节点, 7 个需要独立运维的有状态组件
```

---

## 14. 三期工程实施计划

### 总体验收标准

> 1. 用户通过 Web/IM/CLI 输入需求 → AI 在 Harness 环境中生成生产级代码
> 2. 代码受机械化约束 (原生 Lint + Semgrep + AI Review), 违规自动修复
> 3. AI 生成测试 + 原生框架运行 → 质量门禁放行
> 4. 低风险自动合并部署, 高风险走审批流
> 5. Prometheus + Grafana 监控, 运行指标反馈 AI 迭代 (三期评估 DeepFlow/OTel)
> 6. 代码质量有熵管理, 退化自动修复
> 7. 多模型降级、Token 成本控制、三级紧急停止
> 8. 多租户隔离、完整 RBAC、OAuth/OIDC
> 9. 完整"深空指挥中心"UI + 钉钉/飞书机器人

### 三期是工程依赖顺序, 不是功能裁剪

### 一期: 基座与核心引擎

**交付物**: 完整的 需求→生成→验证→测试→部署 闭环, 所有基础设施生产级部署。

| 模块 | 交付内容 |
|------|---------|
| 基础设施 | PostgreSQL HA (含 FTS), Redis Sentinel, Temporal Server (多节点), Traefik (统一入口), Argo Workflows + Argo CD, Loki + Grafana + Prometheus, 阿里云 OSS |
| Go API Server | Auth (账号密码+JWT+OAuth GitHub/Codeup), Project, Task, Specs (CRUD+三级继承), Adapter (GitHub+Codeup), Settings (KillSwitch L1/L2/L3), Billing (Token追踪+预算) |
| Temporal 编排 | TaskWorkflow 完整主流程, 子工作流并行, 全节点 Checkpoint, Signal (人工审批), 三个 Interceptor |
| AI Worker | Planner/Coder/Reviewer/Fixer Agent, 多模型路由+降级链, 项目画像, RAG, 上下文压缩 |
| DevOps Worker | GitHub+Codeup 全量操作, GitHub Actions/Argo 构建, Argo CD 部署, AI 生成测试+原生框架运行, 临时环境 |
| Constraint Worker | 语言原生 Linter (golangci-lint/eslint/ruff) + Semgrep + Trivy 镜像扫描 + 错误格式化 |
| 前端 | 登录, 项目大厅, 需求对话, 任务看板, AI 工作可视化, 变更结果, 测试报告, 部署环境, WebSocket |
| CLI | forge-cli (Go 单二进制, API Token 鉴权, SSE 实时输出) |
| forge-foundation | 基础设施层组件库 (web/data/cache/storage/log/metrics) |
| 配置发布 | 配置验证 + K8s ConfigMap 写入 + 热加载 + 健康检查 |
| 数据层 | PostgreSQL 全量 Schema (含 organizations 表, FTS 索引), Redis, 阿里云 OSS |

### 二期: 约束闭环与企业能力

**交付物**: Harness 三大支柱完整落地 + 企业级鉴权 + IM 入口。

| 模块 | 交付内容 |
|------|---------|
| 约束引擎 | 架构约束测试, 自定义 Lint 规则管理 API+UI, 规则三级继承, Semgrep 自定义规则 |
| 熵管理 | EntropyWorkflow, 命名/文档/死代码/覆盖率扫描, 自动修复 PR, 质量趋势 |
| 完整鉴权 | OAuth2/OIDC, 钉钉/飞书扫码, LDAP/SSO, MFA, 完整 RBAC 四级, ABAC, 动态鉴权链, 多租户隔离 |
| 成本控制 | 每任务预估+硬限, 每租户月预算, 全局并发限制, 成本报表 |
| IM 机器人 | 钉钉/飞书 Bot (需求提交+进度推送+审批卡片+L1 停止) |
| 前端 | 分支管理, MR 审批, 规范配置 UI, 用户/角色/权限管理 |

### 三期: 可观测闭环与运营成熟

**交付物**: 运行时反馈闭环 + 灰度发布 + 完整运营视图。

| 模块 | 交付内容 |
|------|---------|
| 全栈监控 | 评估 DeepFlow (eBPF) 或 Grafana Alloy + OpenTelemetry, Service Map, 分布式追踪, Profiling |
| 反馈闭环 | 运行时指标 → Temporal → AI 上下文, 部署后健康检查, 异常自动回滚/AI修复 |
| 灰度发布 | Argo Rollouts Canary, 蓝绿, 标签流量切分, 灰度管理 UI |
| 质量 Dashboard | 合规率/覆盖率/复杂度趋势, 模型使用分析 |
| 前端 | 完整"深空指挥中心"视觉, Dashboard, 灰度管理, 变更影响分析, AI 对话 Code Review |
| 平台可观测 | Prometheus + Grafana, Temporal Metrics, AI 调用追踪 |
| 高级适配器 | K8s 多集群, Jenkins/GitLab 适配器, 适配器管理 UI |

### 依赖关系

```
一期 (基座+引擎) ← 没有基座, 上层功能无法运行
      │
      ▼
二期 (约束+企业能力) ← 依赖一期的引擎和适配器
      │
      ▼
三期 (观测+运营) ← 依赖二期的约束和权限体系
```

---

*文档版本: 2.2 | 最后更新: 2026-04-02 | 架构: Go + Python + Temporal | 中间件精简: AI 替代 7 个传统组件*
