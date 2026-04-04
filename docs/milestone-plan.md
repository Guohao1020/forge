# Forge Platform — 里程碑计划

> **版本**: 6.0
> **日期**: 2026-04-05
> **前置文档**: [PRD.md](PRD.md) | [technical-design.md](technical-design.md)
> **架构变更**: v3.0 采用垂直切片(Slice)替代水平分层(Milestone)，每个切片前后端一起交付
> **v4.0 变更**: Phase 2 重构为 7 阶段 AI 开发流水线（P1~P7），对应切片 S8~S14
> **v4.1 变更**: 新增 S15 代码浏览与分支管理、S16 项目画像与 AI 记忆、S17 云端预览环境
> **v4.2 变更**: 新增 SaaS 基础设施架构（K8s 集群拓扑、GitHub App、任务容器、多租户隔离），标注基础设施前置依赖
> **v6.0 变更**: 新增 SP-1（项目智能接入）、SP-2（AI 方案推荐）、SP-3（多平台制品策略）；Phase 2b 扩展为 26 天含 11 个切片；完整 20 个执行计划文档编写完成
> **v5.1 变更**: Phase 2b 重设计为 Harness Engineering 架构 — 新增 SH-1~SH-4 切片（Agent Loop、上下文工具、项目协调器、版本管理 UI）；S9/S10/S11 升级为 S9'/S10'/S11'（集成 context tools）；新增版本管理 + 多需求并发协调。详见 [harness-engineering-design.md](plans/harness-engineering-design.md)
> **v5.0 变更**: 战略重规划 — Phase 2 拆分为 2a/2b/2c/2d 四个子阶段，新增 SX-1~SX-4 补洞切片；S15 已基本完成并入验证；执行顺序从"按流水线阶段线性推进"调整为"先补洞 → 再增强 → 最后基础设施"。详见 [phase2-strategic-replan.md](plans/phase2-strategic-replan.md)

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

## Phase 1 — 最小闭环（7 个切片） ✅ 已完成

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

---

## Phase 2 执行策略（v5.0 战略重规划）

> 详细战略分析见 [phase2-strategic-replan.md](plans/phase2-strategic-replan.md)

### 核心洞察

代码审计发现 Phase 1 链路上有关键环节是空的：需求分析返回占位文本（AnalystAgent 未接线）、规范注入未验证、项目画像扫描未触发。**在这些空洞之上继续增强流水线没有意义。**

### 新执行顺序

```
Phase 2a — 补洞（3-4 天）         ← 让核心差异化能力首次生效
├── SX-1 接线需求分析（AnalystAgent → Temporal）
├── SX-2 验证+修复规范注入
├── SX-3 接线项目画像扫描
└── SX-4 端到端质量验证 + S15 收尾

Phase 2b — Harness Engineering + 产品增强（26 天）
├── SH-1 Harness 基座（ContextCache + 并行获取 + Agent Loop + ModelRouter tools）— 3天
├── SH-2 上下文工具（5 个 context tools + ContextToolExecutor + Agent 接入）— 2天
├── SH-3a 版本数据模型 + CRUD API — 2天
├── SH-3b 版本协调器（VersionOrchestrator + 冲突检测 + ContinueAsNew）— 3天
├── SH-4 版本管理 UI（版本列表/详情 + 冲突展示 + 发布按钮）— 2天
├── S9'  任务拆分增强（DAG 可视化 + touched_files + 冲突预标记）— 2天
├── S10' 测试先行（TestWriter + context tools + 测试审批 + 预览 UI）— 2天
├── S11' 代码生成增强（Coder + context tools + Lint + 并行子任务）— 2天
���── SP-1 项目智能接入（类型检测 + 引导式创建 + 接入清单）— 3天
├── SP-2 AI 方案推荐（RecommendationCard + 上下文感知推荐 + 选择 UI）— 2天
└── SP-3 多平台制品策略（构建模板引擎 + 分支策略 + 签名密钥管理）— 3天

Phase 2c — 基础设施（5-7 天）
├── Infra-1 最小 K8s 环境（k3s/Docker）
├── S12 自动化测试执行（K8s Job + 4层测试 + 覆盖率门禁 + AI 修复循环）— 3天
└── S13 制品管理（Docker 构建 + Trivy 扫描 + 语义化版本）— 2天

Phase 2d — 部署闭环（7 天）
├── S14 K8s 部署（资源清单生成 + 自动部署 + 回滚 + 环境管理）— 3天
├── S16 项目画像 RAG 增强（pgvector + 语义搜索 + 增量更新 + 可视化）— 2天
└── S17 云端预览（AI 检测 + Dockerfile 生成 + 临时环境 + 生命周期管理）— 2天
```

### 切片状态总览

| 切片 | 状态 | 所属阶段 |
|------|------|---------|
| S1-S7 | ✅ COMPLETED | Phase 1 |
| S8 | ✅ COMPLETED | Phase 2 |
| SX-1~SX-4 | ✅ COMPLETED | Phase 2a（补洞）— 占位文本修复、规范注入验证、画像扫描接线 |
| SH-1~SH-4, S9', S10', S11' | ✅ COMPLETED | Phase 2b（Harness + 流水线）— ContextCache、Agent Loop、Context Tools、版本管理、DAG 可视化 |
| SP-1, SP-2, SP-3 | ✅ COMPLETED | Phase 2b（产品增强）— 项目类型检测、AI 推荐卡片、多平台设计 |
| Infra-1, S12, S13 | ✅ COMPLETED | Phase 2c（基础设施）— task-runner Dockerfile、RunTests 更新、BuildDockerImage |
| S14, S16-RAG, S17 | ✅ COMPLETED | Phase 2d（部署闭环）— K8s manifest 生成、pgvector 迁移、Preview 生命周期 |
| S15 | ⚠️ ~90% DONE | 在 SX-4 中收尾 |

---

## Phase 2 — AI 开发流水线增强（7 个切片）

**目标**: 将 AI 开发从"生成代码"升级为完整的 7 阶段流水线：需求澄清 → 任务拆分 → 测试先行 → 代码生成 → 自动化测试 → 制品管理 → K8s 部署

**核心理念**: 每个阶段都有明确的质量门禁，上一阶段不通过则不进入下一阶段。测试先行（P3 在 P4 之前），约束驱动生成。

### 全局视图

```
                    Infra（基础设施）
                    K8s 集群 + GitHub App + 任务容器 + 多租户隔离
                         │
S8 → S9 → S10 → S11 → S12 → S13 → S14
需求    任务   测试    代码    自动化   制品    K8s
澄清    拆分   先行    生成    测试     管理    部署
增强    增强   体系    增强    执行

S15          S16          S17
代码浏览      项目画像      云端预览
分支管理      AI 记忆       环境管理
（可与 S8~S14 并行开发，依赖 S3 GitHub 接入）

注: S12/S13/S14/S17 依赖 Infra 基础设施就绪（K8s Job 执行环境）
```

### 切片详情

| 切片 | 标题 | 对应阶段 | 交付后可做什么 | 预估 Tasks | 依赖 |
|------|------|---------|---------------|-----------|------|
| S8 | 需求澄清增强 | P1 | AI 主动提问澄清需求 + 风险识别 + 技术栈推断 + 结构化需求卡片 | ~6 | S6 |
| S9 | 任务拆分增强 | P2 | 需求自动拆分为 DAG 任务图 + 双向追溯 + 工时估算 + 依赖可视化 | ~8 | S8 |
| S10 | 测试先行体系 | P3 | AI 根据需求生成测试用例（先于代码）+ 原生框架单测 + Python E2E 测试 | ~8 | S9 |
| S11 | 代码生成增强 | P4 | 按语言约束生成代码 + Lint 集成 + Dockerfile 生成 + 实时预览 | ~8 | S10 |
| S12 | 自动化测试执行 | P5 | 顺序执行四层测试（单测→集成→E2E→安全）+ 覆盖率门禁 + 阻断式质量门 | ~7 | S11, **Infra** |
| S13 | 制品管理 | P6 | Docker 构建 + OSS 推送 + 版本管理 + 制品清单 | ~6 | S12, **Infra** |
| S14 | K8s 部署 | P7 | 资源清单生成 + ACK 自动部署 + 环境管理 + 部署状态追踪 | ~8 | S13, **Infra** |
| S15 | 代码浏览与分支管理 | — | 仓库文件树 + 语法高亮 + 分支切换 + PR 查看 | ~8 | S3 |
| S16 | 项目画像与 AI 记忆 | — | 全量画像生成 + 结构化存储 + 向量化索引 + 增量更新 | ~10 | S3, S6 |
| S17 | 云端预览环境 | — | AI 项目检测 + Dockerfile 生成 + PR 级预览环境 + 自动回收 | ~6 | S3, S14, **Infra** |

**预估总计**: ~51 个 Task（S8~S14）+ ~24 个 Task（S15~S17）= ~75 个 Task

### 切片依赖关系

```
S8（需求澄清增强）← 基于 S6 的对话能力
 └→ S9（任务拆分增强）← 需要 S8 的结构化需求
     └→ S10（测试先行体系）← 需要 S9 的任务定义
         └→ S11（代码生成增强）← 需要 S10 的测试用例作为约束
             └→ S12（自动化测试执行）← 需要 S11 的代码 + S10 的测试用例
                 └→ S13（制品管理）← 需要 S12 测试通过的代码
                     └→ S14（K8s 部署）← 需要 S13 的制品

S3（GitHub 接入）
 ├→ S15（代码浏览与分支管理）← 需要 S3 的 GitHub OAuth + API
 ├→ S16（项目画像与 AI 记忆）← 需要 S3 的仓库访问 + S6 的 AI 分析能力
 └→ S17（云端预览环境）← 需要 S3 的代码访问 + S14 的 K8s 部署能力

Infra（SaaS 基础设施）← 可与 S8~S11 并行搭建
 ├→ S12（自动化测试执行）← 测试在 K8s Job 容器中运行
 ├→ S13（制品管理）← Docker 构建/推送需要 K8s Job + ACR
 ├→ S14（K8s 部署）← 需要 tenant-{id}-{env} namespace + ResourceQuota
 └→ S17（云端预览环境）← 需要 K8s 集群 + Ingress + 预览 namespace
```

### 各切片详细说明

#### S8: 需求澄清增强（P1）— ✅ 已完成

基于 S6 的对话能力，增强 AI 的需求澄清能力：

- AI 主动提问（而非被动等待），识别需求中的模糊点
- 自动检测潜在风险（安全、性能、兼容性）
- 推断项目技术栈，生成结构化需求确认卡片
- 输出标准化的需求规格文档

#### S9: 任务拆分增强（P2）

将澄清后的需求拆分为可执行的任务 DAG：

- 需求 → 子任务 DAG 自动拆分（有向无环图，支持并行）
- 需求 ↔ 任务双向追溯（traceability matrix）
- AI 工时估算（按任务复杂度 + 历史数据）
- 前端任务依赖图可视化
- 任务粒度自动校准（过大拆分，过小合并）

#### S10: 测试先行体系（P3）

在代码生成之前，先生成测试用例：

- AI 根据需求 + 任务定义生成测试用例
- 单元测试使用原生框架：go test / JUnit / Jest / pytest
- E2E/集成测试统一使用 Python：pytest + Playwright
- 测试用例覆盖正常路径、边界条件、异常场景
- 测试用例作为代码生成的约束输入

#### S11: 代码生成增强（P4）

在测试约束下生成高质量代码：

- 按语言规范约束生成（Java/Go/TypeScript 各自的编码规范）
- 生成后自动运行 Lint（golangci-lint / ESLint / Checkstyle）
- 自动生成 Dockerfile（基于项目技术栈）
- 实时预览生成过程（SSE 流式输出）
- 生成代码必须通过 S10 的测试用例

#### S12: 自动化测试执行（P5）

顺序执行四层测试，任一层失败则阻断：

- 第一层：单元测试（go test / JUnit / Jest）
- 第二层：集成测试（pytest + 服务编排）
- 第三层：E2E 测试（pytest + Playwright）
- 第四层：安全扫描（Semgrep / trivy）
- 覆盖率门禁（可配置阈值，默认 80%）
- 测试结果可视化 + 失败定位

#### S13: 制品管理（P6）

测试通过后构建和管理制品：

- Docker 镜像构建（多阶段构建优化）
- 镜像推送到 OSS / ACR（阿里云容器镜像服务）
- 语义化版本管理（SemVer）
- 制品清单（SBOM）生成
- 制品与任务/需求的关联追溯

#### S14: K8s 部署（P7）

将制品部署到 ACK（阿里云 K8s）：

- 自动生成 K8s 资源清单（Deployment / Service / ConfigMap / Ingress）
- ACK 集群自动部署（kubectl apply / Helm）
- 多环境管理（dev / staging / production）
- 部署状态实时追踪 + 回滚能力
- 基础运维 Agent（健康检查、日志查询、Pod 重启）

#### S15: 代码浏览与分支管理

在 Web 工作台中提供完整的代码浏览体验，无需离开 Forge：

- 仓库文件树导航（左侧树 + 右侧代码查看器）
- 基于 Shiki 的语法高亮（100+ 语言）
- 分支/Tag 切换下拉选择器
- AI 分支（`ai/` 前缀）特殊标记，关联任务信息
- PR 列表和详情查看（Open/Closed/Merged）
- PR Diff 查看（复用 Monaco Diff 组件）
- GitHub API 缓存策略（内存 5min + Redis 30min + Webhook 失效）
- Phase 3 增强：code-server (VS Code Web) 在线编辑集成

#### S16: 项目画像与 AI 记忆

AI 自动扫描项目生成全量结构化知识，作为代码生成的核心上下文：

- API 接口清单自动采集（AST 解析 + AI 补充）
- 数据库 Schema 自动解析（迁移文件 + ORM 模型）
- 模块依赖关系图（AST 静态分析）
- 技术架构图谱（服务间调用 + 中间件使用）
- 业务规则摘要提取（AI 语义分析代码注释和文档）
- 结构化 JSON 存储（PostgreSQL JSONB）
- 向量化索引用于 RAG 检索（pgvector 1536 维嵌入）
- Git Push Webhook 触发增量更新（< 2 分钟）
- 新建任务时自动加载画像上下文
- Phase 3 增强：跨项目关联记忆（共用库识别 + 微服务依赖图 + 变更影响分析）

#### S17: 云端预览环境

PR 级别自动创建临时云端预览环境：

- AI 自动检测项目类型（单体/前后端分离/微服务）和服务依赖
- AI 自动生成 Dockerfile（多阶段构建，按语言适配）
- AI 自动生成 K8s 资源清单（Deployment/Service/ConfigMap/Ingress）
- 预览 URL: `{taskId}.preview.forge.example.com` + 自动 SSL
- 30 分钟空闲自动回收（缩容到 0 副本，再次访问冷启动 < 30s）
- PR 合并/关闭时自动清理所有资源
- Web 工作台预览环境状态 + 日志查看
- Phase 3 增强：多服务编排预览 + 自定义域名 + HPA 自动扩缩容

#### Infra: SaaS 基础设施（前置依赖，可与 S8~S11 并行）

> 详细设计见 [infra-architecture-design.md](plans/infra-architecture-design.md)

K8s 集群搭建和 GitHub App 注册，是 S12（测试执行）、S13（制品管理）、S14（K8s 部署）、S17（云端预览）的前置条件：

- **ACK 集群初始化**: 单集群多 namespace 拓扑（forge-system / forge-jobs / tenant-{id}-{env}）
- **K8s Job 任务容器**: forge-task-runner 镜像构建，Job 模板（CPU/内存限制、TTL、超时）
- **GitHub App 注册**: 创建 Forge GitHub App，配置权限（Contents R/W, PR R/W, Webhooks R/W）
- **GitHub App Token 管理**: JWT 签名 → Installation Token 获取 → Redis 缓存 → 自动刷新
- **多租户 namespace 自动化**: 租户首次部署时自动创建 namespace + ResourceQuota + NetworkPolicy
- **ACR 镜像仓库**: 阿里云容器镜像服务配置，用于存储构建的 Docker 镜像
- **Ingress 路由**: 预览环境通配符域名 `*.preview.forge.example.com` 配置

实施分四阶段:
1. **阶段 1（当前）**: GitHub API 模式，AI 生成代码通过 API 推送，无需 K8s Job
2. **阶段 2**: K8s Job 任务容器，接入 ACK，测试/构建在容器中执行
3. **阶段 3**: 完整部署流水线，用户代码部署到 K8s，多环境管理
4. **阶段 4**: GitHub App 替代纯 OAuth，支持企业仓库，Webhook 事件驱动

### 每个切片的技术栈递增

| 切片 | 新增技术组件 |
|------|-------------|
| S8 | — (增强 S6 的 LangGraph 工作流) |
| S9 | DAG 可视化库（React Flow / dagre） |
| S10 | pytest, Playwright, 多语言测试运行器 |
| S11 | golangci-lint, ESLint, Dockerfile 模板引擎 |
| S12 | Docker Compose（测试编排）, Semgrep, trivy |
| S13 | Docker buildx, OSS SDK, ACR SDK |
| S14 | kubernetes client-go, Helm SDK, ACK OpenAPI |
| S15 | Shiki（语法高亮）, go-github 文件/分支 API 扩展 |
| S16 | pgvector（向量索引）, OpenAI Embeddings API, AST 解析库 |
| S17 | Docker buildx, kubernetes client-go（Namespace 管理）, cert-manager |
| Infra | ACK 集群, GitHub App（JWT + Installation Token）, K8s Job, ResourceQuota, NetworkPolicy, ACR |

### Phase 2 验收标准

Phase 2 全部完成后，用户可以在浏览器中完成以下完整流程：

1. 输入自然语言需求，AI 主动提问澄清，识别风险
2. 确认需求后，AI 自动拆分为 DAG 任务图，展示依赖和工时估算
3. AI 先生成测试用例（单测 + E2E），人工确认测试方案
4. AI 在测试约束下生成代码，实时预览生成过程
5. 四层自动化测试顺序执行，任一层失败自动阻断并定位问题
6. 测试全部通过后，自动构建 Docker 镜像并推送到 OSS/ACR
7. 自动生成 K8s 资源清单，一键部署到 ACK 集群
8. 实时查看部署状态，支持回滚
9. 基础运维操作（健康检查、日志查询、Pod 管理）
10. 在 Web 界面浏览项目仓库代码，切换分支，查看 AI 创建的 PR 和 Diff
11. 查看 AI 自动生成的项目画像（API 清单、数据模型、模块依赖、架构图谱）
12. AI 生成代码后自动创建临时预览环境，通过 URL 直接在浏览器中访问运行效果

### Phase 1 延后项在 Phase 2 中的覆盖

| Phase 1 延后项 | Phase 2 覆盖切片 |
|---------------|-----------------|
| 四层自动化测试执行 | S10 + S12 |
| 质量门禁（Lint/安全扫描/覆盖率） | S11 + S12 |
| 两阶段风险评估 | S8 |
| DB 迁移安全 | S11（代码生成约束） |

### Phase 2 明确不包含（延后到 Phase 3）

| 功能 | 延后原因 | 计划阶段 |
|------|---------|---------|
| 灰度发布（Canary / 蓝绿） | 需要完整可观测性支撑 | Phase 3 |
| 运维 Agent 高级功能（自动扩缩容、故障自愈） | 需要监控数据闭环 | Phase 3 |
| 完整 RBAC + ABAC | 非流水线核心路径 | Phase 3 |
| IM 机器人（钉钉/飞书） | 非流水线核心路径 | Phase 3 |
| Token 预算控制 + 成本报表 | 非流水线核心路径 | Phase 3 |
| 并发冲突处理（rebase + 自动解决） | 复杂度高，优先保证主路径 | Phase 3 |
| code-server 在线编辑集成 | 代码浏览优先，编辑能力后续增强 | Phase 3 |
| 跨项目关联记忆（共用库/微服务依赖图） | 需要单项目画像稳定后扩展 | Phase 3 |
| 预览环境多服务编排 + 自定义域名 | 需要基础预览能力稳定后增强 | Phase 3 |
| 团队编码习惯分析 + 历史架构决策记录 | 画像高级分析，Phase 3 增强 | Phase 3 |

---

## Phase 3 — 可观测闭环与运营成熟（未拆分）

**目标**: 运行时反馈闭环 + 灰度发布 + 企业级运营能力 + 运维 Agent 高级功能

> Phase 3 计划在 Phase 2 完成并验证后再详细拆分为切片。以下为高层概述。

| 模块 | 交付内容 |
|------|---------|
| 运维 Agent 高级功能 | 自动扩缩容、故障自愈、智能告警、根因分析 |
| 可观测性闭环 | Grafana + Loki + Prometheus 全栈监控（评估 DeepFlow/OTel），运行时数据 → AI 上下文反馈 |
| 灰度发布 | Argo Rollouts Canary、蓝绿部署、流量切分、自动回滚 |
| 质量大盘 | 合规率/覆盖率/复杂度趋势 Dashboard，项目健康度评分，团队效率指标 |
| 企业级鉴权 | OAuth2/OIDC、钉钉/飞书扫码、MFA、完整 RBAC + ABAC |
| 成本控制 | Token 预算控制（任务级硬限 + 租户月预算）、成本报表、用量分析 |
| IM 机器人 | 钉钉/飞书 Bot 入口，需求提交 + 状态通知 |
| 熵管理 | EntropyWorkflow，命名/文档/死代码定期扫描，自动修复 PR |
| code-server 在线编辑 | 从代码浏览器一键跳转 VS Code Web 编辑 + 终端 + 环境隔离（S15 增强） |
| 跨项目关联记忆 | 共用库识别 + 微服务依赖图 + 变更影响分析 + 知识复用（S16 增强） |
| 预览环境增强 | 多服务编排预览 + 自定义域名 + HPA 自动扩缩容（S17 增强） |
| 项目画像高级分析 | 团队编码习惯分析 + 历史架构决策记录 + 代码质量趋势（S16 增强） |

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

*文档版本: 4.2 | 最后更新: 2026-04-02 | 架构: Go + Python + Temporal + Next.js + code-server*
