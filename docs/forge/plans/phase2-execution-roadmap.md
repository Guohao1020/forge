# Phase 2 执行路线图 — 基于现状的优先级排序

> **日期**: 2026-04-02
> **前置**: Phase 1 (S1-S7) 已完成, S8 (P1 需求澄清) 已完成

---

## 现状盘点

### 已完成（可在浏览器中使用）

| 切片 | 状态 | 核心功能 |
|------|------|---------|
| S1 骨架+登录 | ✅ | JWT 登录、Aurora 主题 |
| S2 项目管理 | ✅ | CRUD、收藏、搜索 |
| S3 GitHub 接入 | ✅ | OAuth、同步仓库、导入项目 |
| S4 Temporal+任务 | ✅ | 任务创建、Kanban 看板、SSE 实时 |
| S5 规范中心 | ✅ | 编码规范/Prompt/Review 规则 CRUD |
| S6 AI Worker | ✅ | 需求对话→规划→生成→Review、流式代码输出 |
| S6.1 修复 | ✅ | SSE 接线、数据流修复、JSON 解析 |
| S6.2 前端增强 | ✅ | Shiki 高亮、Monaco Diff、左右分栏工作区 |
| S7 DevOps | ✅ | GitHub 推送分支+创建 PR、环境卡片 |
| S8 需求澄清 | ✅ | 主动追问、风险识别、技术栈检测 |

### 已知问题
- qwen-max JSON 输出不稳定（有时生成空内容）
- 中文编码偶有乱码
- JWT token 有过期时间，ai-worker 需要长期 token
- 项目画像（tech_stack）仅在导入时触发，缺少手动刷新

---

## 执行优先级分析

### 原则
1. **用户价值优先** — 先做用户能立即感知到的改进
2. **依赖链最短** — 优先做不依赖未完成切片的功能
3. **技术风险前置** — 有技术不确定性的先做 PoC
4. **可演示性** — 每个切片完成后都能在浏览器中演示

### 依赖图分析

```
已完成基础：S1-S8 ✅

可立即开始（无前置依赖）：
├── S15 代码浏览与分支管理 ← 只需 S3 (GitHub API) ✅
├── S16 项目画像与 AI 记忆 ← 只需 S3 + S6 ✅
└── S9 任务拆分增强 ← 只需 S8 ✅

需要前置完成：
├── S10 测试先行 ← 需 S9
├── S11 代码生成增强 ← 需 S10
├── S12 自动化测试执行 ← 需 S11
├── S13 制品管理 ← 需 S12
├── S14 K8s 部署 ← 需 S13
└── S17 云端预览 ← 需 S14
```

---

## 推荐执行顺序

### 第一批（立即可做，并行推进）

```
Sprint 1: S15 + S16 并行
         代码浏览    项目画像
         ~8 tasks    ~10 tasks
         ─────────────────────
         预计 2-3 天
```

**S15 代码浏览与分支管理** — 用户价值最直接
- 关联 GitHub 后能浏览仓库代码（文件树 + Shiki 高亮）
- 切换分支查看不同版本
- 查看 AI 创建的 PR
- 复用已有：GitHub adapter (S3/S7), Shiki 组件 (S6.2)
- 主要工作：GitHub Contents API + 前端代码浏览器页面

**S16 项目画像与 AI 记忆** — AI 质量的核心提升
- 自动扫描仓库生成结构化画像（API 清单、DB Schema、模块图）
- 存储为 JSONB + 向量索引
- AI 生成代码时自动加载画像
- 主要工作：扫描逻辑 + DB schema + Context Builder 集成

### 第二批（S15/S16 完成后）

```
Sprint 2: S9
         任务拆分增强
         ~8 tasks
         ─────────
         预计 1-2 天
```

**S9 任务拆分增强** — 流水线核心
- DAG 任务依赖图
- 需求↔任务双向追溯
- 工时估算
- 前端依赖图可视化

### 第三批（测试-生成闭环）

```
Sprint 3: S10 → S11
          测试先行    代码增强
          ~8 tasks    ~8 tasks
          ─────────────────────
          预计 3-4 天
```

**S10 测试先行** — 先于代码生成测试用例
- AI 根据需求生成原生框架测试代码
- 测试用例作为代码生成的输入约束

**S11 代码生成增强** — 约束化代码生成
- 语言约束（tech_stack 强制执行）
- 自动生成 Dockerfile
- Lint 内联检查
- 生成后立即运行测试验证

### 第四批（自动化执行）

```
Sprint 4: S12
          自动化测试执行
          ~7 tasks
          ─────────
          预计 2-3 天
```

**S12 自动化测试执行** — 阻断式门禁
- 在 Docker 容器中运行测试
- 四层顺序执行 + 覆盖率门禁
- 失败自动定位 + AI 修复建议

### 第五批（构建部署链）

```
Sprint 5: S13 → S14 → S17
          制品管理   K8s部署   云端预览
          ~6 tasks   ~8 tasks   ~6 tasks
          ────────────────────────────────
          预计 4-5 天
```

**S13 制品管理** — Docker 构建 + ACR/OSS 推送
**S14 K8s 部署** — ACK 资源生成 + 自动部署
**S17 云端预览** — PR 级临时预览环境

---

## 时间线总览

```
Week 1:  S15 代码浏览 ──────┐
         S16 项目画像 ──────┤ 并行
                            │
Week 2:  S9 任务拆分 ───────┤
         S10 测试先行 ──────┤
                            │
Week 3:  S11 代码生成增强 ──┤
         S12 测试执行 ──────┤
                            │
Week 4:  S13 制品管理 ──────┤
         S14 K8s 部署 ──────┤
         S17 云端预览 ──────┘

总计: ~75 tasks, 预计 4 周
```

---

## 每个切片的具体交付物

### S15: 代码浏览与分支管理 (~8 tasks)

| # | 任务 | 文件 |
|---|------|------|
| 1 | GitHub Contents API (GetTree/GetFile) | `github/client.go` |
| 2 | 代码浏览后端 API | `module/code/handler.go` |
| 3 | 前端代码浏览器页面 | `projects/[id]/code/page.tsx` |
| 4 | 分支列表 + 切换 API | `github/client.go` + 前端 |
| 5 | PR 列表页面 | `projects/[id]/pulls/page.tsx` |
| 6 | PR 详情 + Diff 页面 | `projects/[id]/pulls/[num]/page.tsx` |
| 7 | 左侧导航添加入口 | `project-sidebar.tsx` |
| 8 | 构建 + 测试 | 全量验证 |

### S16: 项目画像与 AI 记忆 (~10 tasks)

| # | 任务 | 文件 |
|---|------|------|
| 1 | 项目画像 DB schema | `migrations/010_project_profile.sql` |
| 2 | 画像扫描 Agent | `ai-worker/src/agents/profiler.py` |
| 3 | API 接口扫描 | 路由/Controller 提取 |
| 4 | DB Schema 扫描 | 迁移文件/模型提取 |
| 5 | 模块依赖图生成 | import/package 分析 |
| 6 | 画像存储 API | `module/profile/handler.go` |
| 7 | Context Builder 集成 | `builder.py` 加载画像 |
| 8 | 画像增量更新 | 代码变更时触发 |
| 9 | 前端画像展示页 | `projects/[id]/profile/page.tsx` |
| 10 | 跨项目关联分析 | 共用依赖检测 |

### S9: 任务拆分增强 (~8 tasks)

| # | 任务 | 文件 |
|---|------|------|
| 1 | PlannerAgent 增强 (DAG) | `agents/planner.py` |
| 2 | 任务依赖模型 | `task/model.go` 扩展 |
| 3 | 需求↔任务映射 | `traceability matrix` |
| 4 | 工时估算逻辑 | `agents/planner.py` |
| 5 | 前端 DAG 可视化 | React Flow/D3 组件 |
| 6 | 任务优先级排序 | 关键路径算法 |
| 7 | 过大/过小任务校准 | Agent 自动拆合 |
| 8 | 构建 + 测试 | 全量验证 |

---

## 关键技术决策

| 决策 | 选型 | 理由 |
|------|------|------|
| DAG 可视化 | React Flow | Next.js 生态首选，交互丰富 |
| 向量索引 | pgvector | 已用 PostgreSQL，无需引入新组件 |
| 画像存储 | JSONB + pgvector | 结构化查询 + 语义搜索 |
| 测试执行 | Docker-in-Docker | 隔离运行环境，安全 |
| 制品存储 | 阿里云 ACR + OSS | 用户已有基础设施 |
| 预览环境 | ACK 临时 namespace | 用户已有 K8s 集群 |
