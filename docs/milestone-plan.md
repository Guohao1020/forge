# Forge Platform — 里程碑计划

> **版本**: 3.1
> **日期**: 2026-03-31
> **前置文档**: [PRD.md](PRD.md) | [technical-design.md](technical-design.md)
> **架构变更**: v3.0 采用垂直切片(Slice)替代水平分层(Milestone)，每个切片前后端一起交付

---

## 实施策略

### 团队结构

- **开发**: Claude Code Opus 4.6（AI 编写所有代码）
- **测试与决策**: Harvey（人工测试 + 产品决策）
- **节奏**: 计划确认 → 逐步执行 → 人工测试 → 下一步

### 垂直切片 vs 水平分层

旧计划（v2.0）按技术层划分里程碑（M0 骨架 → M1 规范 → ... → M6 前端），导致长时间无法在浏览器中调试。

新计划按**垂直功能切片**划分，每个切片交付完整的前后端功能：

```
旧（水平分层）：后端全部做完 ─────────────────→ 前端全部做完
新（垂直切片）：S1 登录 → S2 项目 → S3 GitHub → S4 任务 → S5 规范 → S6 AI → S7 部署
               每步都有前端页面，可以在浏览器中操作和调试
```

---

## Phase 1 — 最小闭环（7 个切片）

**目标**: Web 界面输入需求 → AI 生成代码 → 推送到 GitHub → 变更可见 → 质量信息展示

### 全局视图

```
S1 → S2 → S3 → S4 → S5 → S6 → S7
登录  项目  GitHub  任务   规范   AI    DevOps
骨架  管理  接入    看板   中心   引擎  闭环
```

### 切片详情

| 切片 | 标题 | 交付后可做什么 | Tasks | 迁移 |
|------|------|---------------|-------|------|
| [S1](plans/S1-skeleton-and-login.md) | 骨架 + 登录闭环 | 浏览器登录，看到空项目大厅 | 9 | 001 |
| [S2](plans/S2-project-management.md) | 项目管理 + 页面 | 创建/编辑/删除/收藏项目 | 9 | 002 |
| [S3](plans/S3-github-integration.md) | GitHub 接入 | OAuth 授权，同步仓库，导入项目 | 9 | 003 |
| [S4](plans/S4-temporal-and-tasks.md) | Temporal + 任务 | 创建任务，Kanban 看板，SSE 实时更新 | 8 | 004 |
| [S5](plans/S5-specs-center.md) | 规范中心 | CRUD 编码规范/Prompt/Review 规则 | 6 | 005 |
| [S6](plans/S6-ai-worker.md) | AI Worker | 需求对话 → AI 分析/规划/生成/Review | 8 | 006 |
| [S7](plans/S7-devops-and-deployment.md) | DevOps 闭环 | 代码推送 GitHub → PR → Diff（Monaco + code-server IDE）→ 质量信息 | 8 | 007 |

**总计**: 57 个 Task，7 个数据库迁移

### 切片依赖关系

```
S1（骨架 + 登录）
 └→ S2（项目管理）
     └→ S3（GitHub 接入）
         └→ S4（Temporal + 任务）
             ├→ S5（规范中心）
             └→ S6（AI Worker）← 依赖 S5 的规范数据
                 └→ S7（DevOps 闭环）← 依赖 S6 的代码生成 + S3 的 GitHub 适配器
```

### 每个切片的技术栈递增

| 切片 | 新增技术组件 |
|------|-------------|
| S1 | Go/Gin, PostgreSQL, Redis, Next.js 15, shadcn/ui, Tailwind CSS 4 |
| S2 | — (在 S1 基础上扩展) |
| S3 | GitHub OAuth, go-github 库 |
| S4 | Temporal Server, Temporal Go SDK, SSE |
| S5 | Redis 缓存（规范继承解析） |
| S6 | Python 3.12, LangGraph, Anthropic SDK, OpenAI SDK, Temporal Python SDK |
| S7 | Monaco Editor（内联 Diff）, code-server（Web IDE 完整代码浏览） |

### Phase 1 验收标准

Phase 1 全部完成后，用户可以在浏览器中完成以下完整流程：

1. 登录 Forge
2. OAuth 接入 GitHub，导入项目
3. 进入项目，输入自然语言需求
4. AI 多轮对话澄清 → 生成确认卡片 → 确认执行
5. 实时看到 AI 分析、规划、生成代码的过程
6. AI 生成代码推送到 GitHub 分支，创建 PR
7. 查看代码 Diff（Monaco Editor 内联 + "在 IDE 中打开" code-server 完整浏览）
8. 查看 AI Review 评分 + 质量信息
9. 查看测试报告（基础版：AI 生成的单测通过/失败，其他层级 "Coming soon"）
10. 查看部署环境状态（信息展示，无实际 K8s 部署）

### Phase 1 明确不包含（延后到 Phase 2+）

以下功能在 PRD 中有定义，但 Phase 1 有意延后，避免范围膨胀：

| 功能 | PRD 来源 | 延后原因 | 计划阶段 |
|------|---------|---------|---------|
| 两阶段风险评估（规划后初评 + Review 后终评） | 2.1.5 | Phase 1 用简化的单次风险打标 | Phase 2 |
| 并发冲突处理（rebase + 自动解决） | 2.1.6 | Phase 1 每任务独立分支，暂不处理冲突 | Phase 2 |
| DB 迁移安全（UP+DOWN + 破坏性拦截） | 2.1.7 | Phase 1 AI 不生成 DB migration | Phase 2 |
| 四层自动化测试执行（MeterSphere 对接） | 2.3.2 | Phase 1 仅展示 AI 生成的单测结果 | Phase 2 |
| 质量门禁（Lint/安全扫描/覆盖率） | 2.3.3 | Phase 1 仅 AI Review 评分 | Phase 2 |
| Token 预算控制（任务级硬限 + 租户月预算） | 4.4 | Phase 1 不限制 Token 用量 | Phase 2 |
| 紧急停止开关（KillSwitch L1/L2/L3） | 4.2 | Phase 1 可手动取消任务 | Phase 2 |
| IM 机器人（钉钉/飞书） | 2.5 | Phase 1 仅 Web 入口 | Phase 2 |
| 完整 RBAC + ABAC（权限定义/策略） | 2.2.2 | Phase 1 仅角色区分，无细粒度权限 | Phase 2 |
| 审计日志 | 技术设计 10.2 | Phase 1 无审计表 | Phase 2 |
| billing schema（Token 用量/成本报表） | 技术设计 10.1 | Phase 1 不追踪成本 | Phase 2 |

### Phase 1 数据表延后说明

技术设计 10.2 中定义但 Phase 1 不创建的表：

| 表 | 说明 |
|----|------|
| `auth.organizations` | Phase 1 无组织层级 |
| `auth.permissions` / `auth.role_permissions` | Phase 1 无细粒度权限 |
| `auth.policies` | Phase 1 无 ABAC |
| `auth.audit_logs` | Phase 1 无审计 |
| `engine.task_checkpoints` | Phase 1 依赖 Temporal 原生 checkpoint，不做业务层 checkpoint |
| `engine.model_calls` | Phase 1 不追踪 AI 调用成本（S6 可选补充） |
| `billing.*` | Phase 1 无计费 |

---

## Phase 2 — Harness 完整版（未拆分为切片）

> Phase 2 计划在 Phase 1 完成并验证后再详细拆分。以下为高层概述。

**目标**: Harness 三大支柱完整落地 + 企业级鉴权 + IM 入口

| 模块 | 交付内容 |
|------|---------|
| 约束引擎 | SonarQube, 架构约束测试, 自定义规则管理 |
| 熵管理 | EntropyWorkflow, 命名/文档/死代码扫描, 自动修复 PR |
| 完整鉴权 | OAuth2/OIDC, 钉钉/飞书扫码, MFA, 完整 RBAC |
| 成本控制 | 任务预估+硬限, 租户月预算, 成本报表 |
| IM 机器人 | 钉钉/飞书 Bot |
| 前端完善 | 分支管理, MR 审批, 规范配置 UI, 权限管理 |

---

## Phase 3 — 可观测闭环与运营成熟（未拆分）

**目标**: 运行时反馈闭环 + 灰度发布 + 完整运营视图

| 模块 | 交付内容 |
|------|---------|
| DeepFlow | eBPF 零代码全栈监控 |
| 反馈闭环 | DeepFlow → Temporal → AI 上下文 |
| 灰度发布 | Argo Rollouts Canary, 蓝绿 |
| 质量 Dashboard | 合规率/覆盖率/复杂度趋势 |
| 高级适配器 | K8s 多集群, Jenkins/GitLab 适配器 |

---

## 旧里程碑计划（已废弃）

> 以下旧计划仅作为需求参考。新架构的实施按上方垂直切片执行。

| 旧计划 | 状态 | 对应新切片 |
|--------|------|-----------|
| M0 项目骨架 | ⚠️ Superseded | S1 |
| M1 规范中心 | ⚠️ Superseded | S5 |
| M2 外部适配器 | ⚠️ Superseded | S3 + S7 |
| M3 鉴权中心 | ⚠️ Superseded | S1 (轻量) |
| M4 AI 引擎 | ⚠️ Superseded | S6 |
| M5 DevOps 自动化 | ⚠️ Superseded | S7 |
| M6 Web 工作台 | ⚠️ Superseded | S1~S7 (前端穿插) |

---

*文档版本: 3.1 | 最后更新: 2026-03-31 | 架构: Go + Python + Temporal + Next.js + code-server*
