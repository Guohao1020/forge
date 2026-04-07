# Forge Platform — Developer Guide

## What Is Forge

Forge 是一个 **AI 驱动的 Harness Engineering 平台**。让不懂代码的产品经理/运营通过自然语言描述需求，AI 在 Harness 环境（规范约束 + 机械化验证 + 可观测性反馈）中生成生产级代码，经四层测试后部署上线。

**核心理念：规范即灵魂。** AI 只是执行者，沉淀多年的工程规范才是代码质量的根本保障。

**Harness Engineering 三大支柱**：
- **Context Engineering** — 规范中心 + 项目画像 + Prompt 模板 + 可观测性数据
- **Architectural Constraints** — Linter 引擎 + 结构测试 + AI Review + 质量门禁
- **Entropy Management** — 定期代码质量扫描 + 自动修复 + 趋势追踪

## Project Structure

```
forge/
├── forge-core/         # 统一 API Server — 鉴权、项目管理、任务管理、规范、配置（Go, port 8080）
├── ai-worker/          # AI Worker — LangGraph 编排、多模型路由、代码生成/Review/修复（Python）
├── devops-worker/      # DevOps Worker — Git 操作、CI/CD 触发、部署、测试（Go）
├── constraint-worker/  # Constraint Worker — Lint 执行、安全扫描、架构约束验证（Go）
├── forge-bot/          # IM 机器人 — 钉钉/飞书入口（Go, port 8085）
├── forge-portal/       # Web 工作台 — Next.js React 前端（npm, port 3000）
├── forge-foundation/   # 产品加速库 — AI 孵化产品的可选组件库（Go multi-module）
├── docker-compose.yml  # 本地基础设施
└── docs/               # 项目文档（见下方）
```

## Build Commands

```bash
# Go API Server
cd forge-core && go build ./cmd/forge-core

# AI Worker
cd ai-worker && pip install -r requirements.txt

# DevOps Worker
cd devops-worker && go build ./cmd/devops-worker

# Constraint Worker
cd constraint-worker && go build ./cmd/constraint-worker

# Frontend
cd forge-portal && npm run build

# Run tests
cd forge-core && go test ./...
cd ai-worker && pytest
```

## Local Dev Environment

```bash
docker compose -f docker-compose.dev.yml up -d    # Start PostgreSQL, Redis, Temporal
docker compose -f docker-compose.dev.yml down      # Stop all
```

### Service Ports

| Service | Port | Description |
|---------|------|-------------|
| forge-core | 8080 | 统一 API Server (Go) |
| forge-bot | 8085 | IM 机器人 (Go) |
| forge-portal (dev) | 3000 | 前端开发服务器 (Next.js) |
| PostgreSQL | 5432 | 数据库 |
| Redis | 6379 | 缓存（密码：forge_redis_2026）|
| Temporal | 7233 | 工作流引擎 |
| Temporal Web UI | 8233 | 工作流可视化 |
| code-server | 8443 | Web IDE (VS Code Web) |

### Frontend Dev

前端开发时 Next.js 按路径前缀代理到 forge-core：
- `/api/*` → localhost:8080 (forge-core，统一 API 入口)

### Default Credentials

- 管理员账号：`admin` / `admin123`（tenantId=1）

## Coding Standards

See `docs/references/coding-standards.md` for full conventions. Key rules:

### Backend (Go)
- Go 1.22+ with Gin framework
- Modular monolith: forge-core 内按 Module 划分（auth/project/task/specs/adapter/billing/settings）
- `Result[T]` generic wrapper for all API responses
- Constructor injection via dependency struct
- Structured logging with `slog` (JSON format)
- PostgreSQL with multi-schema isolation (auth/engine/specs/pipeline/billing)
- All business tables must have `tenant_id` for multi-tenant isolation

### AI Worker (Python)
- Python 3.12+ with LangGraph
- Temporal Worker SDK for activity execution
- Multi-model routing: Claude → GPT → 通义 (fallback chain)
- Context optimization with token budget management

### Frontend (React)
- Next.js 16 (App Router) + React 19 + TypeScript
- shadcn/ui + Radix UI (component library)
- Zustand (client state) + TanStack Query (server state)
- Tailwind CSS 4 (CSS-first config via `@theme` in `globals.css`)
- Inter + JetBrains Mono (fonts)
- Lucide Icons (icon library)
- Light + dark theme via `.dark` class on `<html>` (user-toggled, localStorage persisted)
- **Brand:** Variant B "Dense Engineering" — Cursor/VS Code aesthetic, 12px/1.4 body,
  4px radius, compact IDE density. Accent `#2563eb` light / `#3b82f6` dark.
- **Design system:** `docs/DESIGN.md` is authoritative; the mockup lives at
  `~/.gstack/projects/voc-shulex-forge/designs/agent-terminal-shotgun-20260406/variant-B-dense.html`

### AI 生成代码的编码规范（规范中心管理）
- AI 为目标项目生成的代码遵循规范中心的编码规范（如 Java/SQL/Redis 等）
- 这些规范通过 Prompt 模板注入 AI 上下文，不是 Forge 平台本身的编码规范
- 详见 `docs/references/coding-standards.md`

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

### Milestone Execution Plans (旧 Java 架构, 已废弃)

| Plan | Status | Description |
|------|--------|-------------|
| [M0 — Project Scaffold](docs/plans/M0-project-scaffold.md) | ⚠️ Superseded | 旧 Java 项目骨架 |
| [M1 — Specs Center](docs/plans/M1-specs-center.md) | ⚠️ Superseded | 旧 Java 规范中心 |
| [M2 — External Adapters](docs/plans/M2-external-adapters.md) | ⚠️ Superseded | 旧 Java 适配器 |
| [M3 — Auth Center](docs/plans/M3-auth-center.md) | ⚠️ Superseded | 旧 Java 鉴权中心 |
| [M4 — AI Engine](docs/plans/M4-ai-engine.md) | ⚠️ Superseded | 旧 Java AI 引擎 |
| [M5 — DevOps Automation](docs/plans/M5-devops-automation.md) | ⚠️ Superseded | 旧 Java DevOps |
| [M6 — Web Console](docs/plans/M6-web-console.md) | ⚠️ Superseded | 旧 Vue 3 Web 工作台 |

> **注意**: 以上旧里程碑计划仅作为需求参考。新架构 (Go + Python + Temporal) 的实施计划见 [技术设计文档](docs/technical-design.md) 第 14 节三期工程实施计划。

### References

| Reference | Content |
|-----------|---------|
| [Harness Engineering Research](docs/references/harness-engineering-research.md) | Harness Engineering + DeepFlow 调研报告 |
| [Coding Standards](docs/references/coding-standards.md) | 编码规范基线 |
| [Scaffold Patterns](docs/references/scaffold-patterns.md) | 脚手架设计范式 |
| [Gray Release Methodology](docs/references/gray-release-methodology.md) | 灰度发布方法论 |
| [Codeup API](docs/references/codeup-api.md) | Codeup API 能力清单 |
| [ACK API](docs/references/ack-api.md) | ACK/K8s API 能力清单 |

## Architecture Overview

```
用户（Web / IM / CLI）
        │
   Traefik（TLS + 路由 + JWT + 限流 + 灰度）
        │
   forge-core（Go 模块化单体）
   ├── Auth / Project / Task / Specs / Adapter / Billing / Settings
        │
   Temporal Server（状态脊梁）
        │
   ┌────┼──────────────┬──────────────┐
   ▼    ▼              ▼              ▼
AI Worker  DevOps Worker  Constraint Worker
(Python    (Go)            (Go)
LangGraph) Argo CD         golangci-lint/eslint
Claude/GPT GitHub Actions  Semgrep
           GitHub/Codeup

   ──── 可观测性层 ────
   Grafana + Loki + Prometheus
```

> **AI 替代中间件**: Claude 替代 SonarQube (代码质量)、MeterSphere (测试平台)、
> Elasticsearch (搜索)。详见技术设计文档 §13.6。

## Current Status

**架构重构中**: 从 Java 微服务架构全面重构为 Go + Python + Temporal 架构。旧的 M0~M5 Java 骨架代码将被替换。

**三期工程实施**（不是 MVP，是生产级企业版的工程依赖顺序）:
- **一期 — 基座与核心引擎**: 基础设施 + Harness 六大组件 + AI 引擎完整版 + 适配器
- **二期 — 约束闭环与企业能力**: 约束引擎 + 熵管理 + 完整鉴权 + 成本控制 + IM 机器人
- **三期 — 可观测闭环与运营成熟**: 全栈监控 (评估 DeepFlow/OTel) + 运行时反馈 + 灰度发布 + 质量 Dashboard

**Key Design Decisions**:
- **Temporal 驱动** — 所有 AI 任务都是有状态、可恢复、可观测的 Workflow
- **Go 模块化单体** — 替代 6 个 Java 微服务，一个二进制部署
- **LangGraph (Python)** — AI 编排，多模型路由 + 降级链
- **Harness Engineering 平台** — 规范约束 + 机械化验证 + 可观测性闭环
- 项目优先导航 — 先选项目再看子页面
- 一键接入 — OAuth 授权后同步全部仓库
- 混合式需求输入 — 自然语言 → AI 澄清 → 确认卡片
- 四层自动化测试 — AI 生成测试 + 原生框架运行（替代 MeterSphere）
- 分支全自动 — 低风险自动合并，高风险等审批
- Grafana + Loki + Prometheus — 平台可观测性（三期评估 DeepFlow/OTel 用于 AI 生成产品）
- "Dense Engineering" — Cursor/VS Code aesthetic, 12px IDE density, blue accent
  (`#2563eb` light / `#3b82f6` dark), approved Variant B mockup, see `docs/DESIGN.md`
