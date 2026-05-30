<div align="center">

# ⚒️ Forge

### AI 驱动的 Harness Engineering 平台

**用自然语言描述需求，产出生产级代码 —— 经澄清、测试先行、Review，自动部署上线。**

<br/>

[![Go](https://img.shields.io/badge/Go-1.26-00ADD8?logo=go&logoColor=white)](https://go.dev)
[![Python](https://img.shields.io/badge/Python-3.12-3776AB?logo=python&logoColor=white)](https://www.python.org)
[![Next.js](https://img.shields.io/badge/Next.js-16-000000?logo=nextdotjs&logoColor=white)](https://nextjs.org)
[![React](https://img.shields.io/badge/React-19-61DAFB?logo=react&logoColor=black)](https://react.dev)
[![Temporal](https://img.shields.io/badge/Temporal-Workflows-000000?logo=temporal&logoColor=white)](https://temporal.io)
[![PostgreSQL](https://img.shields.io/badge/PostgreSQL-16-4169E1?logo=postgresql&logoColor=white)](https://www.postgresql.org)

[![Tests](https://img.shields.io/badge/tests-745%20passing-3fb950)](#测试)
[![Status](https://img.shields.io/badge/status-production--grade-2563eb)](#路线图)
[![Architecture](https://img.shields.io/badge/agent-A2%20单agent循环-8957e5)](#a2-agent)

[English](README.md) · **中文**

</div>

---

> ### *规范即灵魂*
> AI 只是执行者，沉淀多年的工程规范才是代码质量的根本保障。

Forge 让一个**完全不懂代码**的产品经理/运营，用一句自然语言描述需求 —— *"我要一个优惠券系统"*、*"加一个通知渠道"* —— 由 AI Agent 在 **Harness（约束环境）** 中端到端把它建出来。这个约束环境由三样东西构成：版本化的工程规范、机械化的验证、可观测的反馈。它不是原型生成器，而是一条**生产级流水线**。

## 它如何工作

一切始于有人敲下一句话的那一刻。

Agent **不会**立刻动手写代码。它先退一步，问清楚你到底想要什么 —— 在动任何一个文件之前，主动暴露模糊点、识别技术风险、消除不确定性。当存在多种合理实现方案时，它不会替你拍脑袋：它把方案铺成结构化的卡片 —— 利弊对比、风险评级，以及一条**基于你项目自身上下文**的推荐理由 —— 让你点选。

需求一旦锁定，一个 **Cursor 风格的单 Agent** 接管全程，跑一个连续的 tool-use 循环。它先写测试。它用资深工程师惯用的那套文件工具去读你已有的代码 —— glob、grep、read、edit。它查询你项目的"活画像" —— API 目录、数据库 schema、模块依赖图、业务规则 —— 让生成的代码**贴合**架构而非与之对抗。它在一个被锁死的沙箱里执行命令，看着它失败，自我修复，再跑一遍。每一步都通过 `set_phase` 工具对外播报，前端因此能展示一条实时的阶段进度条，精确显示 Agent 此刻走到哪：澄清 → 测试先行 → 生成 → Review → 测试 → 部署。

**规范中心**里的每一条规范 —— Java、SQL、Redis、命名、安全 —— 都会在每次运行时被**机械化注入**到 Agent 的上下文里。Agent 不可能"忘记"一条约定，因为约定写在 prompt 里，而不是某个人的记忆里。

活干完后，低风险变更一路直通 Review、合并、部署；高风险变更则停下来等人工审批。无论哪条路径，Agent 吐出的每一个 token 都被实时流式推送、并持久落库 —— 你既能看着它思考，也能事后回放。

这就是系统的核心：**规范约束它，验证证明它，可观测性展示它。**

## 架构

```
                       用户  (Web · IM · CLI)
                              │
                  Traefik  (TLS · JWT · 限流 · 灰度)        ← 生产网关
                              │
        ┌─────────────────────┴─────────────────────┐
        │            forge-core  (Go :8080)          │   模块化单体, 18 模块
        │  auth · project · task · specs · pipeline  │   ~99 endpoints / 22 组
        │  profile · version · cost · entropy · …     │
        └─────────────────────┬─────────────────────┘
                              │
            ┌─────────────────┼──────────────────────┐
            ▼                 ▼                        ▼
   ai-worker (Py :8090)   Temporal (:7233)     forge-bot (Go :8085)
   ── A2 单 Agent ──       有状态 workflow        钉钉 · 6 卡片模板
   • QueryEngine 循环      (版本编排、熵扫描)
     (≤25 轮 tool 调用)
   • 15 件工具的工具带
   • bwrap 沙箱  (无网络, namespace 隔离)
   • LRU 会话缓存 + hook + 权限守卫
   • 事件双写 → Redis Streams + Postgres
   • ModelRouter: qwen3 → Claude → GPT → DeepSeek
                 (熔断降级)
            │
            ▼
   forge-portal (Next.js :3000)        可观测性
   29+ 页面 · 87 组件                   Grafana · Prometheus · Loki · Promtail
```

> **AI 替代中间件**：Claude/qwen 替代 SonarQube（代码质量）、MeterSphere（测试平台）、Elasticsearch（搜索）。详见[技术设计文档](docs/technical-design.md) §13.6。

## A2 Agent

Forge 的心脏是**一个自主 Agent**，而不是一条层层交接的刚性流水线。（早期的六 Agent 流水线 *pair_pipeline* 已废弃。）

| 能力 | 含义 |
|------|------|
| **单 Agent tool-use 循环** | 一个 `QueryEngine` 会话以 Cursor 的方式，通过最多 25 轮"推理 + 工具调用"驱动整个任务。 |
| **15 件工具的工具带** | 6 个文件工具（`read` · `write` · `edit` · `glob` · `grep` · `ls`）、沙箱化 `bash`、`set_phase`、5 个项目上下文查询工具、2 个人机交互工具（`clarify` · `request_review`）。 |
| **bwrap 沙箱** | 每条 shell 命令都跑在 Bubblewrap 的 `--unshare-all` 隔离里 —— 无网络、workspace 是**唯一**可读写挂载、环境变量白名单、输出上限、超时、进程组 kill。 |
| **实时 + 持久事件** | 每个事件双写到 Redis Streams（SSE 热缓冲）**与** Postgres `agent_messages`（可回放历史）。会话先从 PG 水合，再订阅 Redis。 |
| **自愈式模型路由** | `ModelRouter` 按 qwen3 → Claude → GPT → DeepSeek 降级，带熔断器（3 次失败即打开，30s 窗口，60s 恢复）。 |
| **安全护栏** | 每次工具调用都经过权限校验 + hook 注册表包裹；`LRUSessionCache` 约束内存占用。 |

## 快速开始

```bash
# 前置：Docker, Go 1.26+, Python 3.12+, Node.js 20+

# 一条命令：基础设施 → migrations → ai-worker 镜像 → forge-core → 健康检查
bash scripts/dev-deploy.sh

# 前端（另开一个终端）
cd forge-portal && npm run dev
```

然后打开工作台：

| | 地址 | 凭据 |
|---|-----|------|
| **Web 工作台** | http://localhost:3000 | `admin` / `admin123` |
| API | http://localhost:8080 | — |
| 健康检查 | http://localhost:8080/health | — |
| Grafana | http://localhost:3001 | `admin` / `forge_grafana_2026` |
| Temporal UI | http://localhost:8233 | — |

<details>
<summary>手动步骤（脚本失败时）</summary>

```bash
# 1. 基础设施
docker compose -f docker-compose.dev.yml up -d postgres redis temporal

# 2. Migrations（幂等）
docker compose -f docker-compose.dev.yml exec -T postgres \
  psql -U forge -d forge_main < forge-core/migrations/025_workspaces.sql

# 3. 重建并重启 ai-worker（代码在镜像里，不是 volume 挂载）
docker compose -f docker-compose.dev.yml build --no-cache ai-worker
docker compose -f docker-compose.dev.yml up -d --force-recreate ai-worker

# 4. forge-core
cd forge-core && go build ./cmd/forge-core
FORGE_SECRETS_MASTER_KEY=$(python -c "import base64,os; print(base64.b64encode(os.urandom(32)).decode())") ./forge-core &
```

各种坑（master-key 轮换、workspace 降级）见 [CLAUDE.md](CLAUDE.md)。
</details>

## 服务

| 服务 | 技术栈 | 端口 | 说明 |
|------|--------|------|------|
| **forge-core** | Go · Gin · PostgreSQL | 8080 | 统一 API —— ~99 endpoints, 22 资源组, 18 模块 |
| **ai-worker** | Python · FastAPI · LangGraph · Temporal | 8090 | A2 单 Agent（QueryEngine + 15 工具 + bwrap 沙箱） |
| **forge-portal** | Next.js 16 · React 19 · shadcn/ui · Tailwind 4 | 3000 | Web 工作台 —— 29+ 页面, 87 组件 |
| **forge-bot** | Go · Gin | 8085 | IM 机器人 —— 钉钉 webhook, 6 卡片模板 |
| **Temporal** | — | 7233 | 有状态工作流引擎（UI :8233） |
| **Grafana** | — | 3001 | 3 个看板（健康度、AI 性能、任务） |
| **Prometheus** | — | 9090 | 指标采集 |
| **Loki + Promtail** | — | 3100 | 日志聚合 |

## 理念

Forge 建立在 **Harness Engineering**（由 OpenAI 提出）之上 —— 这是一门"设计 AI Agent 运行**环境**、使其在规模化场景下保持可靠"的工程学。三大支柱：

- **Context Engineering** —— Agent 只能看到仓库里的东西。规范、项目画像、Prompt 模板都被版本化并机械化注入，绝不依赖记忆。
- **Architectural Constraints** —— 约定靠**强制执行**，而非靠文档自觉。Linter、结构测试、AI Review、质量门禁，让"做错"变得很难。
- **Entropy Management** —— AI 会忠实复制模式，包括坏模式。持续的质量扫描、自动修复、趋势追踪，对抗代码熵增。

以及四条工作原则：

- **规范即灵魂** —— Agent 永远服从规范中心。
- **测试先行** —— 先写测试再写代码，使用目标项目的原生框架。
- **风险前置** —— 模糊点和技术风险在需求阶段解决，而不是在生产环境。
- **证据胜于断言** —— 在机械化验证点头之前，没有什么是"做完了"。

## 测试

```bash
make test          # 全量：Go + Python + TypeScript + ESLint
make test-go       # 404 个 Go 测试（390 core + 14 bot），18 个 package
make test-python   # 341 个 Python 测试，47 个文件
make bench         # 22 个 Go benchmark
make smoke-test    # API 端点冒烟测试
make coverage      # Go 覆盖率报告
```

## 文档

| 文档 | 说明 |
|------|------|
| [PRD](docs/PRD.md) | 产品需求 —— 愿景、20 个功能模块、业务规则 |
| [技术设计](docs/technical-design.md) | 架构、Harness Engineering、数据模型、三期工程计划 |
| [产品设计](docs/product-design.md) | UI/UX 规格、页面设计、"Dense Engineering" 视觉系统 |
| [里程碑计划](docs/milestone-plan.md) | Phase 1–3 交付路线图 |
| [Harness 设计](docs/plans/harness-engineering-design.md) | L1/L2/L3 架构（ContextCache, Tools, Orchestrator） |
| [编码规范](docs/references/coding-standards.md) | 注入给 Agent 的规范基线 |
| [CHANGELOG](CHANGELOG.md) · [CONTRIBUTING](CONTRIBUTING.md) · [CLAUDE.md](CLAUDE.md) | 发布记录 · 协作流程 · 开发者指南 |

## 路线图

Forge 按**生产级企业系统**交付，分三期工程推进 —— 不是 MVP。分期是为了控制范围和节奏，绝不降低质量标准。

- **一期 —— 基座与核心引擎** ✅ —— 基础设施、Harness 六大组件、AI 引擎完整版、外部适配器。
- **二期 —— 约束闭环与企业能力** ✅ —— 约束引擎、熵管理、完整鉴权、成本控制、IM 机器人。
- **三期 —— 可观测闭环与运营成熟** ✅ —— 全栈监控、运行时反馈、灰度发布、质量看板。
- **A2 —— 单 Agent 重构** ✅ —— Cursor 风格 Agent + 统一 tool-use 循环、bwrap 沙箱、SSH deploy key、实时/持久会话采集。*（pair_pipeline 已废弃。）*

## 平台数据

```
后端 API:        ~99 endpoints · 22 资源组 · 18 模块
Go 测试:         404 (390 core + 14 bot) + 22 benchmark
Python 测试:     341 个, 分布在 47 个文件
前端:            29+ 页面 · 87 组件
Agent 工具:      15  (6 文件 · bash · set_phase · 5 上下文 · 2 交互)
Migrations:      26
Docker 服务:     10  (基础设施 + 可观测性)
发布 Tag:        46  (最新 v1.1.3)
```

<div align="center">
<br/>

**⚒️ Forge** —— *规范即灵魂*

</div>
