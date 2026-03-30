# Forge Platform — Developer Guide

## What Is Forge

Forge 是一个 AI 驱动的产品孵化平台。让不懂代码的产品经理/运营通过自然语言描述需求，AI 基于企业级规范生成生产级代码，经自动化审查和四层测试后部署上线。

**核心理念：规范即灵魂。** AI 只是执行者，沉淀多年的工程规范才是代码质量的根本保障。

## Project Structure

```
forge/
├── forge-engine/       # AI 引擎 — 需求解析、任务编排、代码生成、审查（Java, port 8081）
├── forge-identity/     # 鉴权中心 — 认证、授权、JWT、多租户（Java, port 8082）
├── forge-pipeline/     # DevOps 自动化 — 流水线、质量门禁、部署、适配器（Java, port 8083）
├── forge-specs/        # 规范中心 — 编码规范、Prompt 模板、Review 规则（Java, port 8084）
├── forge-bot/          # IM 机器人 — 钉钉/飞书入口（Java, port 8085）
├── forge-portal/       # Web 工作台 — Vue 3 前端（npm, port 5173）
├── forge-beacon/       # 实时网关 — WebSocket 推送（Node.js, port 3001）
├── forge-foundation/   # 产品加速库 — AI 孵化产品的可选组件库（Java multi-module）
├── docker-compose.yml  # 本地基础设施
└── docs/               # 项目文档（见下方）
```

## Build Commands

```bash
# All Java modules
mvn clean compile

# Single module
cd forge-engine && mvn clean compile

# Frontend
cd forge-portal && npm run build

# Real-time gateway
cd forge-beacon && npm run build

# Run tests
cd forge-engine && mvn test
```

## Local Dev Environment

```bash
docker compose up -d    # Start MySQL, Redis, Kafka, Nacos, ES, APISIX, etcd
docker compose down     # Stop all
```

### Service Ports

| Service | Port | Description |
|---------|------|-------------|
| forge-engine | 8081 | AI 引擎 |
| forge-identity | 8082 | 鉴权中心 |
| forge-pipeline | 8083 | DevOps 自动化 |
| forge-specs | 8084 | 规范中心 |
| forge-bot | 8085 | IM 机器人 |
| forge-beacon | 3001 | 实时网关 |
| forge-portal (dev) | 5173 | 前端开发服务器 |
| MySQL | 3306 | 数据库 |
| Redis | 6379 | 缓存（密码：forge_redis_2026）|
| Kafka (external) | 9094 | 消息队列 |
| Nacos | 8848 | 配置中心 |
| Elasticsearch | 9200 | 搜索引擎 |
| APISIX (gateway) | 9080 | API 网关 |
| APISIX (admin) | 9180 | 网关管理 |

### Frontend Dev

前端开发时 Vite 按路径前缀代理到各后端服务端口（不依赖 APISIX）：
- `/api/auth`, `/api/users`, `/api/roles` → localhost:8082 (forge-identity)
- `/api/tasks`, `/api/killswitch`, `/api/token-usage` → localhost:8081 (forge-engine)
- `/api/standards`, `/api/prompts`, `/api/review-rules` → localhost:8084 (forge-specs)
- `/api/pipelines`, `/api/deployments`, `/api/environments`, `/api/webhooks` → localhost:8083 (forge-pipeline)

### Default Credentials

- 管理员账号：`admin` / `admin123`（tenantId=1）

## Coding Standards

See `docs/references/coding-standards.md` for full conventions. Key rules:

### Backend (Java)
- Java 17 + Spring Boot 3.2
- DO/DTO/VO/BO naming convention
- `Result<T>` for all API responses
- Constructor injection only (no `@Autowired` on fields)
- SLF4J with placeholders `log.info("msg: {}", val)`, no string concat
- MyBatis Plus 3.5.5 with MetaObjectHandler for auto-fill timestamps
- Flyway for database migrations (separate MySQL and H2 scripts)
- All business tables must have `tenant_id` for multi-tenant isolation

### Frontend (Vue 3)
- Vue 3.5 + TypeScript + Vite
- Ant Design Vue 4.x (UI library)
- Pinia 3.x (state management)
- Geist Sans + Geist Mono (fonts)
- Lucide Icons (icon library)
- Dark mode only, "深空指挥中心" visual style
- Brand color: Forge Purple #8B5CF6

## Documents

### IMPORTANT: Documentation Rules

**All planning, design, and spec work MUST follow this document structure.** When using any skill (brainstorming, writing-plans, executing-plans, etc.):

1. **Product requirements** go into `docs/PRD.md` — what to build and why
2. **Product design** goes into `docs/product-design.md` — pages, interactions, visual specs
3. **Technical design** goes into `docs/technical-design.md` — architecture, data models, tech choices
4. **Milestone plan** goes into `docs/milestone-plan.md` — delivery roadmap and acceptance criteria
5. **Execution plans** go into `docs/plans/M{n}-{name}.md` — task-level breakdown for each milestone
6. **Reference materials** go into `docs/references/` — coding standards, API docs, methodology docs

**Never create docs outside this structure.** No `docs/superpowers/`, no `docs/specs/`, no scattered design files. All documents cross-reference each other and must stay consistent.

When updating any feature:
- Update PRD if requirements change
- Update product-design if pages/interactions change
- Update technical-design if architecture/tech changes
- Update milestone-plan if scope/deliverables change
- Update the relevant execution plan if tasks change

### Core Documents

| Document | Purpose |
|----------|---------|
| [PRD](docs/PRD.md) | 产品需求文档 — 功能需求、业务规则、非功能需求 |
| [Product Design](docs/product-design.md) | 产品设计规格书 — 页面设计、交互流程、视觉规范、错误状态 |
| [Technical Design](docs/technical-design.md) | 技术设计文档 — 架构、数据模型、高可用、技术选型 |
| [Milestone Plan](docs/milestone-plan.md) | 里程碑计划 — Phase 1~4 分阶段交付路线图 |

### Milestone Execution Plans

| Plan | Status | Description |
|------|--------|-------------|
| [M0 — Project Scaffold](docs/plans/M0-project-scaffold.md) | ✅ Done | 项目骨架 + 基础设施 |
| [M1 — Specs Center](docs/plans/M1-specs-center.md) | ✅ Done | 规范中心 |
| [M2 — External Adapters](docs/plans/M2-external-adapters.md) | ✅ Done | 外部平台适配器 |
| [M3 — Auth Center](docs/plans/M3-auth-center.md) | ✅ Done | 鉴权中心 |
| [M4 — AI Engine](docs/plans/M4-ai-engine.md) | ✅ Done | AI 引擎 |
| [M5 — DevOps Automation](docs/plans/M5-devops-automation.md) | ✅ Done | DevOps 自动化 |
| [M6 — Web Console](docs/plans/M6-web-console.md) | 🔄 Needs Rework | Web 工作台（需按新产品设计重做）|

### References

| Reference | Content |
|-----------|---------|
| [Coding Standards](docs/references/coding-standards.md) | 编码规范基线 |
| [Scaffold Patterns](docs/references/scaffold-patterns.md) | 脚手架设计范式 |
| [Gray Release Methodology](docs/references/gray-release-methodology.md) | 灰度发布方法论 |
| [Codeup API](docs/references/codeup-api.md) | Codeup API 能力清单 |
| [ACK API](docs/references/ack-api.md) | ACK/K8s API 能力清单 |

## Architecture Overview

```
用户（Web / IM / CLI）
        │
        ▼
    APISIX 网关（路由 / 鉴权 / 限流 / 灰度）
        │
   ┌────┼────────────────────┐
   ▼    ▼                    ▼
鉴权中心  AI 引擎（中央大脑）    实时网关
          │ 编排 + 调度          │
          │ Worker Pool         │
          ▼                     │
   ┌──────┼──────────┐         │
   ▼      ▼          ▼         │
规范中心  DevOps    适配器层     │
                   ├ 代码托管    │
                   ├ 容器编排    │
                   ├ CI/CD      │
                   └ 测试平台    │
```

## Current Status

**Phase 1 — 最小闭环**: M0~M5 骨架代码已完成，M6 需按新产品设计重做。

**Phase 1 验收标准**:
> 用户在 Web 界面输入"创建一个用户管理服务" → AI 生成完整项目 → 代码推送到 Codeup → 流水线自动构建 → 四层测试通过 → 成功部署到 dev 环境

**Key Design Decisions**:
- 项目优先导航 — 先选项目再看子页面
- 一键接入 — OAuth 授权后同步全部仓库
- 混合式需求输入 — 自然语言 → AI 澄清 → 确认卡片
- 三级进度视图 — 概览/详情/实时可切换
- 四层自动化测试 — 单测/接口/集成/回归，AI 选择工具
- 分支全自动 — 低风险自动合并，高风险等审批
- MeterSphere — 开源测试平台（API 测试 + 测试管理）
- SSE（Phase 1） — 实时推送，Phase 2 升级 WebSocket
- "深空指挥中心" — 暗色主题 + Forge 紫 #8B5CF6 + Aurora 背景
