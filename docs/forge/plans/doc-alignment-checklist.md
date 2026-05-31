# 文档对齐清单 -- PRD、技术设计、产品设计更新项

> **日期**: 2026-04-03
> **关联切片**: SP-1 (项目智能接入), SP-2 (AI 方案推荐), SP-3 (多平台制品构建)
> **目的**: 列出 SP-1/SP-2/SP-3 交付后，PRD、技术设计、产品设计、里程碑计划中需要同步更新的确切位置和内容

---

## 1. PRD.md 更新项

### 1.1 [Section 1.4] 核心能力表 -- 新增多平台能力

**当前内容**: 核心能力表列出了 12 项能力（从零孵化、迭代增强、风险前置...到多平台适配），但"多平台适配"仅描述为"支持接入多种代码托管、容器编排、CI/CD 平台"，未体现多项目类型的构建差异。

**需要变更**: 修改"多平台适配"行，拆分为两项：

| 能力 | 说明 |
|------|------|
| 多平台适配 | 支持接入多种代码托管、容器编排、CI/CD 平台，首期对接 GitHub + 阿里云生态 |
| **多平台构建** | **支持 Web 应用、移动应用（Flutter/RN）、桌面应用（Tauri/Electron）、函数库（npm/Go/PyPI）的构建和发布，自动匹配构建模板和分支策略** |

**原因**: SP-3 引入了超越 Docker 镜像的多种制品类型，核心能力表应体现这一能力扩展。

---

### 1.2 [Section 2.7.2] 项目管理 -- 新增 Onboarding 向导描述

**当前内容**: 2.7.2 项目管理表格有 4 行：一键接入、创建新项目、项目画像、项目列表。其中"创建新项目"描述为"选模板快速开始 + 分步引导自定义（代码托管/部署环境/CI/CD 平台）"。

**需要变更**: 将"创建新项目"行更新为：

| 功能 | 说明 |
|------|------|
| **创建新项目** | **4 步引导向导：项目类型选择（Web/移动/桌面/API/库/全栈） → 技术栈选择（含 AI 推荐） → 部署目标 → 团队发布节奏。自动配置分支策略、测试框架、制品类型。保留"快速创建"旁路。** |
| **导入后引导** | **项目导入后显示 Onboarding Checklist：GitHub 连接 → 画像扫描 → 规范配置 → 创建版本 → 首个需求。引导用户完成关键配置步骤。** |

**原因**: SP-1 完全重新设计了项目创建流程，PRD 应反映引导式向导而非旧的"选模板"流程。

---

### 1.3 [Section 2.12] 外部平台适配 -- 新增移动/桌面/库构建适配器

**当前内容**: 2.12.1 适配需求表列出 5 种适配类型：代码托管、容器编排、CI/CD 流水线、可观测性、容器镜像仓库。

**需要变更**: 新增 3 行适配类型：

| 适配类型 | 职责 | 首期接入 | 未来扩展 |
|---------|------|---------|---------|
| **移动应用构建** | **Flutter/RN 编译、签名、Store 提交** | **Fastlane (Android/iOS)** | **App Center, Codemagic** |
| **桌面应用构建** | **跨平台编译、代码签名、分发** | **Tauri Action (GitHub Actions)** | **electron-builder** |
| **库发布** | **版本管理、包发布** | **semantic-release (npm/Go/PyPI)** | **JFrog Artifactory, Nexus** |

**原因**: SP-3 引入了 Docker 之外的构建类型，适配器清单应覆盖所有 Forge 支持的构建目标。

---

### 1.4 [Section 2.17 -- 新增或扩展] 分支策略 per 项目类型

**当前内容**: PRD v5.0 有 2.17 版本管理与多需求并行，描述了版本迭代和冲突检测。分支策略仅在 2.4 (代码托管与 Git 操作) 和 2.5.3 (灰度部署) 中零散提及，无系统性描述。

**需要变更**: 在 2.17 中新增一个子节（或作为 2.17.x）：

```
#### 2.17.x 分支策略 per 项目类型

| 策略 | 适用项目类型 | 分支流程 | 发布方式 |
|------|------------|---------|---------|
| TRUNK_BASED | Web 应用, 后端 API, Monorepo | feature -> main -> auto-deploy | 合并即部署 (K8s rolling update) |
| GITHUB_FLOW | 函数库 (npm/Go/PyPI) | feature -> main -> auto-tag | Tag 触发自动发布到 Registry |
| RELEASE_TRAIN | 移动应用, 桌面应用 | feature -> main -> release/{version} -> Store 提交 | 版本进入 TESTING 阶段自动创建 release 分支 |

移动/桌面应用 Hotfix 流程：
hotfix/{date}/{slug} -> release/{version} + cherry-pick to main

分支策略在项目创建时根据项目类型自动设置，可在项目设置中手动覆盖。
```

**原因**: SP-3 Day 2 定义了三种分支策略，需要在需求文档中有正式定义。

---

### 1.5 [新增 Section 2.19] AI 方案推荐系统

**建议位置**: 在 2.18 (AI Harness Engineering 基座) 之后新增 2.19。

**内容**:

```
### 2.19 AI 方案推荐系统

当 AI 在需求分析、任务规划、技术选型等阶段识别出多种可行方案时，以结构化推荐卡片
呈现给用户，而非单方面选择执行。

#### 2.19.1 推荐卡片

| 功能 | 说明 |
|------|------|
| 结构化方案对比 | 每个方案展示标题、优缺点、风险等级、工时估算、影响文件 |
| AI 推荐标注 | AI 基于项目上下文（API 数量、架构类型、模块复杂度）选出最优方案并说明理由 |
| 上下文因素 | 展示影响推荐决策的项目指标（可展开查看） |
| 用户选择 | 用户点选方案后，AI 按选择的方案继续执行 |

#### 2.19.2 推荐触发场景

| 场景 | 示例 |
|------|------|
| 架构决策 | 新功能放在现有服务 vs 独立服务 |
| 任务拆分策略 | 并行开发 vs 串行开发 |
| 技术选型 | ORM 选择、状态管理方案 |
| 部署策略 | 风险等级模糊时的部署方式选择 |

推荐最多 3 个方案，仅在方案有实质性差异时触发（非琐碎变化）。
```

**原因**: SP-2 是全新功能，PRD 中无对应需求描述。

---

### 1.6 [新增 Section 2.20] 多平台制品构建

**建议位置**: 在 2.19 之后新增 2.20。

**内容**:

```
### 2.20 多平台制品构建

根据项目类型自动匹配构建模板和制品类型，支持多种发布目标。

#### 2.20.1 项目类型 -> 构建模板映射

| 项目类型 | 构建模板 | 制品类型 | 发布目标 |
|---------|---------|---------|---------|
| Web 应用 (Next.js/Nuxt/Vite) | Docker multi-stage | Docker 镜像 | K8s / CDN |
| 移动应用 (Flutter) | Flutter build + Fastlane | AAB + IPA | App Store / Google Play |
| 移动应用 (React Native) | RN build + Fastlane | AAB + IPA | App Store / Google Play |
| 桌面应用 (Tauri) | Cargo + Tauri cross-compile | DMG + EXE + AppImage | GitHub Releases |
| 桌面应用 (Electron) | electron-builder | DMG + EXE + AppImage | GitHub Releases |
| 函数库 (npm) | npm build + publish | npm package | npm Registry |
| 函数库 (Go) | GoReleaser | Go module tag | Go Proxy |
| 函数库 (Python) | Python build | wheel/sdist | PyPI |
| 后端 API (Go/Java/Python) | Docker multi-stage | Docker 镜像 | K8s |

#### 2.20.2 签名与密钥管理

平台特定的签名和凭证（Android keystore、App Store API key、npm token 等）
加密存储在项目设置中，构建时通过 K8s Secret 挂载注入。

构建前自动检查必需密钥是否已配置，缺失时在任务流程中提示用户。
```

**原因**: SP-3 引入的多平台构建是全新能力，需要在 PRD 中定义需求。

---

## 2. technical-design.md 更新项

### 2.1 [Section 3.4] 上下文工程 -- 新增 ContextCache + 并行获取 + context tools

**当前内容**: 3.4 展示了 ContextBuilder 类的 `build_context` 和 `optimize_context` 方法，但描述的是串行全量加载模式。

**需要变更**: 更新为 Harness Engineering 架构中的新模式：

1. 将 `ContextBuilder` 替换为 `ContextCache`（Redis 缓存，一次获取全工作流复用）
2. 添加并行获取描述（coding_standards / project_profile / relevant_code 并行加载）
3. 添加 context tools 说明（Agent 通过工具按需查询，而非全量注入 system prompt）

**变更范围**: 整个 3.4 节的代码示例和说明文本需要重写，对齐 harness-engineering-design.md 中 L1/L2 层的设计。

**原因**: SH-1/SH-2 切片重新设计了上下文获取方式，技术设计文档应反映最新架构。

---

### 2.2 [Section 3.5] 并发冲突处理 -- 新增 VersionOrchestrator + 3 层检测

**当前内容**: 3.5 描述了简单的独立分支 + rebase + 冲突检测流程（5 步）。

**需要变更**: 扩展为 VersionOrchestrator 驱动的 3 层冲突检测：

1. **L1 预标记** (PlannerAgent): 任务规划时标记 touched_files，与同版本其他任务比对
2. **L2 Git 检测** (合并前): 实际 rebase 尝试，检测文件级冲突
3. **L3 语义检测** (AI 分析): 非文件级冲突的语义冲突（如两个任务修改同一接口的不同方法）

添加 VersionOrchestrator 工作流描述：管理版本内多任务的执行顺序、冲突预防、顺序合并。

**原因**: SH-3 切片引入了版本管理和并发协调，3.5 节的简单冲突处理描述已过时。

---

### 2.3 [Section 10] 数据架构 -- 新增 project_versions 表

**当前内容**: Section 10 定义了各 schema 的表结构。engine schema 包含 tasks / task_checkpoints / model_calls 等表。

**需要变更**: 在 engine schema 中新增：

```sql
-- project_versions: 版本管理（迭代/Sprint）
CREATE TABLE engine.project_versions (
    id BIGSERIAL PRIMARY KEY,
    project_id BIGINT NOT NULL REFERENCES projects(id),
    tenant_id BIGINT NOT NULL,
    version VARCHAR(50) NOT NULL,        -- "1.2.0" (SemVer)
    title VARCHAR(200),                  -- "v1.2 — 积分系统上线"
    status VARCHAR(30) NOT NULL DEFAULT 'PLANNING',
    -- Status: PLANNING -> IN_PROGRESS -> TESTING -> RELEASING -> RELEASED
    description TEXT,
    branch_strategy VARCHAR(30),         -- Override project default
    release_branch VARCHAR(200),         -- "release/v1.2.0" (auto-created for RELEASE_TRAIN)
    released_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW(),
    UNIQUE(project_id, version)
);
```

同时在 projects 表中标注新增列：
- `onboarding_status JSONB` (SP-1)

新增独立表：
- `project_secrets` (SP-3)

**原因**: SH-3/SH-4 引入版本模型，SP-1 引入 onboarding 状态，SP-3 引入密钥管理，数据架构应同步更新。

---

### 2.4 [Section 14] 三期工程实施计划 -- 重写 Phase 2 计划

**当前内容**: Section 14 引用了旧的三期工程实施策略，与 milestone-plan.md 中 v5.0/v5.1 的切片结构不完全对齐。

**需要变更**: 更新 Phase 2 描述，明确列出 2a/2b/2c/2d 子阶段及各切片：

```
Phase 2a — 补洞 (3-4 天): SX-1 ~ SX-4
Phase 2b — Harness Engineering (15 天): SH-1 ~ SH-4 + S9'/S10'/S11' + SP-1/SP-2/SP-3
Phase 2c — 基础设施 (5-7 天): Infra-1 + S12 + S13
Phase 2d — 部署闭环 (5-7 天): S14 + S16-RAG + S17
```

SP-1/SP-2/SP-3 归入 Phase 2b，标注依赖关系。

**原因**: milestone-plan.md 已更新为 v5.1 切片结构，技术设计的实施计划应保持一致。

---

### 2.5 [Section 9 -- 新增子节] 多平台构建模板和签名密钥管理

**建议位置**: 在 Section 9 (外部平台适配器技术设计) 中新增 9.8 子节。

**内容摘要**:

```
### 9.8 多平台构建模板引擎

构建模板引擎为不同项目类型提供预置的构建流水线配置。

#### 模板结构
每个模板定义: 构建步骤序列、容器镜像、命令、超时、依赖关系、必需密钥。

#### 模板选择逻辑
基于 ProjectTypeProfile (SP-1 检测结果) 自动匹配:
projectType + deployTarget -> 构建模板 ID

#### 密钥管理
平台特定密钥 (Android keystore, npm token 等) 加密存储在 project_secrets 表，
使用 SOPS+age 加密模式，构建时通过 K8s Secret 卷挂载注入。

#### 构建就绪检查
任务进入 BUILD 步骤前自动检查必需密钥是否已配置，缺失则阻断并提示用户。
```

**原因**: SP-3 的构建模板引擎和密钥管理是新的技术组件，需要在技术设计中有对应描述。

---

## 3. product-design.md 更新项

### 3.1 [Section 3.2] 项目大厅 -- 重新设计项目创建流程

**当前内容**: 3.2.2 "创建新项目" 描述了旧流程：选模板（标准 Java 微服务 / Vue 3 前端 / 等 5 种） → 分步配置（项目名 → 代码托管 → 部署环境 → CI/CD）→ 确认创建。

**需要变更**: 整个 3.2.2 节重写为 SP-1 的引导式向导：

```
#### 3.2.2 创建新项目

**目的**: 通过 4 步引导向导帮助用户做出正确的项目配置选择。

**交互流程**:
1. Step 1 — "你要构建什么？": 6 个大卡片 (Web/移动/桌面/API/库/全栈)
2. Step 2 — 技术栈选择: 基于 Step 1 展示对应选项 (Next.js/Flutter/Tauri/Go...)
3. Step 3 — 部署目标: K8s / Serverless / App Store / Desktop / Registry
4. Step 4 — 团队节奏: 持续部署 / 周发布 / 计划发布
5. 确认 — 配置摘要 + 项目名称/描述 + 创建

自动配置: 分支策略、测试框架、制品类型根据选择自动生成。
快速通道: "快速创建" 链接跳过向导，直接输入名称 + GitHub 同步。
```

同时新增 3.2.3 "导入后引导":

```
#### 3.2.3 导入后引导

**目的**: 引导新导入的项目完成关键配置。

导入完成后，项目详情页顶部显示 Onboarding Checklist:
1. 已连接 GitHub (自动完成)
2. 项目画像扫描 (自动进行)
3. 配置编码规范 (链接到规范中心)
4. 创建第一个版本 (链接到版本管理)
5. 提交第一个需求 (链接到对话页)

Checklist 可最小化、可跳过。全部完成后自动消失。
```

**原因**: SP-1 完全替代了旧的项目创建流程，产品设计应反映新的交互设计。

---

### 3.2 [Section 3.6] Phase 2 新增页面 -- 新增版本管理 + 推荐卡片 + 构建设置

**当前内容**: 3.6 有 3 个子节：3.6.1 代码浏览器 (S15), 3.6.2 项目画像 (S16), 3.6.3 云端预览环境 (S17)。

**需要变更**: 新增 3 个子节：

```
#### 3.6.4 版本管理 (SH-3/SH-4)

**目的**: 管理项目版本迭代，组织多个需求/任务到一个版本中。

页面设计:
- 版本列表: 版本号 + 标题 + 状态 (PLANNING/IN_PROGRESS/TESTING/RELEASED) + 任务数
- 版本详情: 关联任务列表 + 冲突检测面板 + 发布按钮
- 发布流程: TESTING 阶段自动创建 release 分支 (RELEASE_TRAIN 策略)
- 冲突展示: 两个任务修改同一文件时，高亮冲突区域和解决建议

#### 3.6.5 AI 推荐卡片 (SP-2)

**目的**: 在对话中展示 AI 的多方案推荐。

UI 设计:
- 2-3 个方案卡片并排展示
- 每个卡片: 标题 + 优缺点 (绿/红) + 风险徽章 + 工时估算
- AI 推荐方案: 紫色边框 + "AI 推荐" 徽章
- 点击"选择此方案"发送用户选择
- "查看 AI 推荐依据" 可展开查看项目上下文因素
- 选择后卡片锁定，已选方案打钩高亮

#### 3.6.6 构建与部署设置 (SP-3)

**目的**: 项目设置中管理构建模板、分支策略和密钥。

页面设计 (项目设置新 Tab):
- 构建模板: 自动检测的模板名称 + 手动切换选项
- 分支策略: 当前策略 + 切换下拉框
- 必需密钥: 清单式展示每个密钥的配置状态 (已配/未配)
- 密钥配置: 文本输入 (token/password) 或文件上传 (keystore/cert)
- 构建就绪状态: 绿色 "就绪" 或红色 "缺少 N 个密钥"
```

**原因**: SP-2/SP-3/SH-3/SH-4 引入了新的 UI 组件和页面，产品设计应包含其视觉和交互规格。

---

### 3.3 [Section 8 -- 里程碑映射] 更新切片映射

**当前内容**: Section 8 (或最末的里程碑映射章节) 将页面/组件映射到切片 S1~S17。

**需要变更**: 新增 SP-1/SP-2/SP-3 的映射行：

| 页面/组件 | 切片 | 阶段 |
|----------|------|------|
| 项目创建向导 (4 步) | SP-1 | Phase 2b |
| 项目类型检测引擎 | SP-1 | Phase 2b |
| 导入后 Onboarding Checklist | SP-1 | Phase 2b |
| AI 推荐卡片 (对话组件) | SP-2 | Phase 2b |
| 构建模板引擎 | SP-3 | Phase 2b (depends Infra-1) |
| 分支策略配置 | SP-3 | Phase 2b |
| 密钥管理 UI | SP-3 | Phase 2b (depends Infra-1) |

**原因**: 新切片的页面应纳入里程碑映射，便于追踪交付范围。

---

## 4. milestone-plan.md 更新项

### 4.1 新增 SP-1/SP-2/SP-3 到 Phase 2b 切片列表

**当前内容**: Phase 2b 列出 SH-1 ~ SH-4 + S9'/S10'/S11' 共 7 个切片。

**需要变更**: 在 Phase 2b 切片列表中新增：

```
Phase 2b — Harness Engineering（18 天）  ← 基座层 + 版本管理 + 流水线增强 + 项目智能化
├── SH-1 Harness 基座
├── SH-2 上下文工具
├── SH-3 项目协调器
├── SH-4 版本管理 UI
├── S9'  任务拆分增强
├── S10' 测试先行
├── S11' 代码生成增强
├── SP-1 项目智能接入与引导（3 天）        ← NEW
├── SP-2 AI 方案推荐系统（2 天）           ← NEW
└── SP-3 多平台制品构建策略（3 天）        ← NEW (after Infra-1)
```

注意: SP-3 虽归入 Phase 2b，但实际执行需在 Infra-1 之后（可能延到 Phase 2c 初期）。

### 4.2 更新总天数

**当前**: Phase 2b 约 15 天。

**变更后**: Phase 2b 约 18 天 (新增 SP-1 3天 + SP-2 2天 + SP-3 3天，部分可与 SH 切片并行)。

实际并行度:
- SP-1 Day 1-2 (后端检测 + 前端向导) 可与 SH-1/SH-2 并行
- SP-2 依赖 SH-2 (上下文工具) -- 需在 SH-2 之后
- SP-3 依赖 Infra-1 和 SH-3a (版本模型) -- 需在两者之后

净增加约 3-5 天（考虑并行）。

### 4.3 新增切片到状态总览表

在"切片状态总览"表中新增：

| 切片 | 状态 | 所属阶段 |
|------|------|---------|
| SP-1 | NOT STARTED | Phase 2b (项目智能化) |
| SP-2 | NOT STARTED | Phase 2b (AI 推荐) |
| SP-3 | NOT STARTED | Phase 2b → 2c (多平台构建) |

### 4.4 新增依赖关系

在依赖关系图中添加：

```
SP-1（项目智能接入）← 依赖 S3 (GitHub 接入)
  └→ SP-3（多平台制品构建）← 依赖 SP-1 的 ProjectTypeProfile + Infra-1 + SH-3a

SH-2（上下文工具）
  └→ SP-2（AI 方案推荐）← 依赖 SH-2 的 context tools + S8 的需求分析

Infra-1（基础设施）
  └→ SP-3（多平台制品构建）← 构建模板在 K8s Job 中执行
```

### 4.5 新增切片详细说明

在 "各切片详细说明" 段落中，为 SP-1/SP-2/SP-3 各写一段简要说明（50-80 字），格式对齐现有 S8~S17 的说明风格：

```
#### SP-1: 项目智能接入与引导

重新设计项目创建/导入体验：自动检测项目类型（Web/移动/桌面/API/库），
4 步引导式创建向导，导入后 Onboarding Checklist 引导用户完成关键配置。

#### SP-2: AI 方案推荐系统

AI 识别多种可行方案时，输出结构化 RecommendationCard（方案对比 + 上下文感知推荐），
前端渲染为可选卡片，用户选择后 AI 按选定方案执行。

#### SP-3: 多平台制品构建策略

构建模板引擎覆盖 Web/移动/桌面/库 4 大类项目的构建、签名、发布全流程。
分支策略按项目类型区分（Trunk-based / GitHub Flow / Release Train）。密钥加密管理。
```

---

## 5. 更新执行优先级

| 优先级 | 文档 | 更新项 | 理由 |
|--------|------|--------|------|
| P0 | milestone-plan.md | 新增 SP-1/SP-2/SP-3 到 Phase 2b + 依赖关系 | 开发计划入口，必须先更新 |
| P1 | PRD.md | 新增 2.19/2.20 + 更新 1.4/2.7.2/2.12/2.17 | 需求定义，指导实现 |
| P1 | product-design.md | 重写 3.2 + 新增 3.6.4/3.6.5/3.6.6 | UI 实现参考 |
| P2 | technical-design.md | 更新 3.4/3.5/10/14 + 新增 9.8 | 架构文档，开发参考 |

建议在 SP-1 编码开始前完成 P0 + P1 更新，P2 可在编码过程中同步更新。

---

*Checklist version: 1.0 | Author: Claude + Harvey | Date: 2026-04-03*
