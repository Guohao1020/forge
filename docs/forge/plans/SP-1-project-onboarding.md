# SP-1 -- 项目智能接入与引导

**Duration**: 3 days
**Priority**: P1 -- 项目创建/导入是用户的第一印象，直接影响留存
**Dependencies**: S3 (GitHub 接入已完成), SX-3 (项目画像扫描已接线)
**Outputs**: 项目类型检测引擎 + 引导式创建向导 + 导入后 Onboarding Checklist

---

## 1. Goal

将项目创建/导入从"填表单"升级为"智能引导"体验。导入已有项目时，自动检测项目类型并生成完整的 ProjectTypeProfile；创建新项目时，通过分步向导引导用户做出正确选择，并根据选择自动配置分支策略、测试框架、制品类型等。导入完成后，展示 Onboarding Checklist 引导用户完成关键配置。

---

## 2. Current State Analysis

### 2.1 What exists

1. **Project model** (`forge-core/internal/module/project/model.go`): `Project` struct 有 `TechStack json.RawMessage` 字段，但仅存简单 JSON，无结构化类型检测
2. **CreateProjectRequest**: 只有 name / description / codePlatform / codeRepoUrl / defaultBranch，无项目类型、部署目标、团队节奏等字段
3. **create-project-dialog.tsx**: 单页对话框，输入名称 + 描述 + 是否同步到 GitHub，无分步引导
4. **ImportRequest**: 批量导入 GitHub 仓库，只记录 fullName / language / htmlURL，不做任何类型检测
5. **Branch naming**: 硬编码 `feature/{date}/{tenant}/{user}/{taskId}-{slug}`，无法按项目类型区分

### 2.2 What is missing

| Gap | Impact |
|-----|--------|
| 无项目类型检测 | 不同类型项目（Web/移动/桌面/库）的构建流程完全不同，当前一刀切 |
| 创建向导太简陋 | 新用户不知道该配什么，容易放弃 |
| 导入后无引导 | 用户导入项目后不知道下一步做什么 |
| 分支策略固定 | 移动应用需要 release train，库需要 tag-based，当前只有一种 |

---

## 3. Day 1 -- 项目类型检测引擎

### 3.1 New file: `forge-core/internal/module/project/detector.go`

**核心函数**:

```go
// DetectProjectType analyzes a repository's file tree to determine project type,
// frameworks, build tools, and recommended configurations.
func DetectProjectType(repoFiles []string) *ProjectTypeProfile {
    // ...
}
```

**ProjectTypeProfile 结构**:

```go
type ProjectTypeProfile struct {
    ProjectType   string   `json:"projectType"`   // web_app | mobile_app | desktop_app | backend_api | library | monorepo
    SubType       string   `json:"subType"`       // nextjs | flutter | tauri | go_api | spring_boot | react_native | electron ...
    Languages     []string `json:"languages"`     // ["TypeScript", "Go", "Dart"]
    Frameworks    []string `json:"frameworks"`    // ["Next.js 15", "Tailwind CSS"]
    BuildTools    []string `json:"buildTools"`    // ["npm", "go build", "flutter build"]
    TestFrameworks []string `json:"testFrameworks"` // ["jest", "go test", "flutter test"]
    DeployTarget  string   `json:"deployTarget"`  // k8s | serverless | app_store | desktop_dist | package_registry
    ArtifactType  string   `json:"artifactType"`  // docker_image | static_site | aab_ipa | dmg_exe | npm_package | go_module
    BranchStrategy string  `json:"branchStrategy"` // TRUNK_BASED | GITHUB_FLOW | RELEASE_TRAIN
    Confidence    string   `json:"confidence"`     // high | medium | low
    Signals       []string `json:"signals"`        // ["found next.config.ts", "found package.json with next dependency"]
}
```

### 3.2 Detection rules (signature files)

检测基于文件树中的特征文件，按优先级匹配：

| Signature Files | ProjectType | SubType | Confidence |
|----------------|-------------|---------|------------|
| `next.config.*` + `package.json` | web_app | nextjs | high |
| `nuxt.config.*` + `package.json` | web_app | nuxt | high |
| `angular.json` | web_app | angular | high |
| `vite.config.*` + no SSR | web_app | vite_spa | high |
| `pubspec.yaml` + `android/` + `ios/` | mobile_app | flutter | high |
| `react-native.config.js` OR `metro.config.*` | mobile_app | react_native | high |
| `src-tauri/` + `tauri.conf.json` | desktop_app | tauri | high |
| `electron-builder.*` OR `electron/` + `package.json` | desktop_app | electron | high |
| `go.mod` + `cmd/` directory | backend_api | go_api | high |
| `pom.xml` + `src/main/java/` | backend_api | spring_boot | medium |
| `build.gradle*` + `src/main/java/` | backend_api | spring_boot | medium |
| `requirements.txt` + `main.py` OR `app.py` | backend_api | fastapi | medium |
| `go.mod` + NO `cmd/` + NO `main.go` in root | library | go_module | medium |
| `package.json` + `tsconfig.json` + NO framework config | library | npm_package | medium |
| `setup.py` OR `pyproject.toml` + NO app entry | library | pypi_package | medium |
| `lerna.json` OR `pnpm-workspace.yaml` OR `turbo.json` | monorepo | mono_mixed | high |

**Confidence rules**:
- `high`: 2+ strong signature files match
- `medium`: 1 strong signature OR 2+ weak signals
- `low`: heuristic only (language file counts, directory patterns)

### 3.3 Auto-derive configurations

基于 ProjectType 自动推导默认配置：

```go
func (p *ProjectTypeProfile) DefaultBranchStrategy() string {
    switch p.ProjectType {
    case "web_app", "backend_api":
        return "TRUNK_BASED"      // feature -> main -> auto-deploy
    case "library":
        return "GITHUB_FLOW"      // feature -> main -> auto-publish tag
    case "mobile_app", "desktop_app":
        return "RELEASE_TRAIN"    // feature -> main -> release/{version} -> store
    case "monorepo":
        return "TRUNK_BASED"
    default:
        return "TRUNK_BASED"
    }
}

func (p *ProjectTypeProfile) DefaultArtifactType() string {
    switch p.SubType {
    case "nextjs", "nuxt", "vite_spa":
        return "docker_image"     // or static_site for pure SPA
    case "flutter":
        return "aab_ipa"
    case "react_native":
        return "aab_ipa"
    case "tauri":
        return "dmg_exe_appimage"
    case "electron":
        return "dmg_exe"
    case "go_api", "spring_boot", "fastapi":
        return "docker_image"
    case "go_module":
        return "go_module"
    case "npm_package":
        return "npm_package"
    case "pypi_package":
        return "pypi_package"
    default:
        return "docker_image"
    }
}
```

### 3.4 Integration with import flow

修改 `forge-core/internal/module/project/service.go` 的 `ImportProjects` 方法：

```
Import flow (current):
1. Validate repos
2. Create project records
3. Return

Import flow (new):
1. Validate repos
2. Create project records
3. For each project, async:
   a. Fetch repo file tree (top 2 levels) via GitHub API
   b. Run DetectProjectType(fileList)
   c. Store result in projects.tech_stack (JSONB)
   d. If confidence == "low", mark project for manual review
4. Return immediately (detection runs in background)
```

### 3.5 Extend Project model

修改 `model.go`，在 `Project` struct 中不新增字段（`TechStack` JSONB 已存在），但定义其结构：

```go
// TechStackData is the structured content of Project.TechStack JSONB field.
// Replaces the previous unstructured tech_stack usage.
type TechStackData struct {
    // Auto-detected profile (from DetectProjectType)
    Profile *ProjectTypeProfile `json:"profile,omitempty"`

    // User-confirmed overrides (from onboarding wizard)
    ProjectType    string `json:"projectType,omitempty"`
    DeployTarget   string `json:"deployTarget,omitempty"`
    BranchStrategy string `json:"branchStrategy,omitempty"`
    TeamCadence    string `json:"teamCadence,omitempty"` // continuous | weekly | planned
}
```

### 3.6 Tasks

| # | Task | File(s) | Est. |
|---|------|---------|------|
| 1.1 | Define ProjectTypeProfile and TechStackData types | `model.go` | 30min |
| 1.2 | Implement DetectProjectType with all rules | `detector.go` (new) | 2h |
| 1.3 | Write unit tests for detector (all project types) | `detector_test.go` (new) | 1.5h |
| 1.4 | Integrate detection into import flow (async) | `service.go` | 1h |
| 1.5 | API endpoint: GET /api/projects/{id}/profile | `handler.go`, `router.go` | 1h |
| 1.6 | Manual test: import a Next.js + Go + Flutter repo | -- | 30min |

---

## 4. Day 2 -- 引导式创建向导 (New Projects)

### 4.1 Redesign `create-project-dialog.tsx`

将单页对话框重构为分步向导组件：

```
Step 1: "你要构建什么？"
  → 6 个大卡片选择：Web 应用 / 移动应用 / 桌面应用 / 后端 API / 函数库 / 全栈项目
  → 每个卡片: 图标 + 标题 + 一句话描述
  → 选择后卡片高亮（紫色边框），"下一步" 可点击

Step 2: "技术栈选择" (conditional on Step 1)
  → Web: Next.js / Nuxt / Vite+React / Vite+Vue / 其他
  → 移动: Flutter / React Native / 其他
  → 桌面: Tauri / Electron / 其他
  → 后端: Go (Gin) / Spring Boot / FastAPI / 其他
  → 库: Go Module / npm Package / PyPI Package / 其他
  → 全栈: Next.js + Go API / Nuxt + Spring Boot / 其他
  → 如果 SP-2 已完成: 显示 AI 推荐卡片 (RecommendationCard)
  → 否则: 热门选项带 "推荐" 徽章 (静态标注)

Step 3: "部署到哪里？"
  → K8s (ACK / 自建)
  → Serverless (阿里云 FC / Vercel)
  → App Store (iOS + Android)
  → Desktop Distribution (GitHub Releases)
  → Package Registry (npm / PyPI / Go Modules)
  → 暂不配置
  → 默认根据 Step 1 + Step 2 自动选中最合适的选项

Step 4: "团队发布节奏"
  → 持续部署: 每次合并即发布 (推荐用于 Web/API)
  → 周发布: 每周固定时间发布 (推荐用于 SaaS)
  → 计划发布: 按版本计划发布 (推荐用于移动/桌面)
  → 默认根据 Step 1 自动选中

Confirm: 配置摘要
  → 显示所有选择的摘要卡片
  → "自动配置项" 区域: 展示将自动设置的分支策略、测试框架、制品类型
  → 项目名称 + 描述输入 (从 Step 1 移到这里)
  → GitHub 同步选项 (保留现有 syncToRemote 逻辑)
  → "创建项目" 按钮
```

### 4.2 Auto-configure based on wizard selections

用户完成向导后，前端构造 `CreateProjectRequest`，后端根据选择自动生成完整配置：

| 用户选择 | 自动配置 |
|---------|---------|
| Web + Next.js + K8s + 持续部署 | branchStrategy=TRUNK_BASED, testFramework=jest, artifactType=docker_image |
| Mobile + Flutter + App Store + 计划发布 | branchStrategy=RELEASE_TRAIN, testFramework=flutter_test, artifactType=aab_ipa |
| Desktop + Tauri + GitHub Releases + 计划发布 | branchStrategy=RELEASE_TRAIN, testFramework=cargo_test+jest, artifactType=dmg_exe_appimage |
| Library + Go Module + Package Registry + 持续部署 | branchStrategy=GITHUB_FLOW, testFramework=go_test, artifactType=go_module |
| Backend + Go + K8s + 周发布 | branchStrategy=TRUNK_BASED, testFramework=go_test, artifactType=docker_image |

### 4.3 Extend `CreateProjectRequest`

```go
type CreateProjectRequest struct {
    // Existing fields
    Name          string `json:"name" binding:"required,min=1,max=200"`
    Description   string `json:"description"`
    CodePlatform  string `json:"codePlatform"`
    CodeRepoURL   string `json:"codeRepoUrl"`
    DefaultBranch string `json:"defaultBranch"`
    AIModel       string `json:"aiModel"`
    RiskThreshold *int   `json:"riskThreshold"`
    AutoMerge     *bool  `json:"autoMerge"`
    SyncToRemote  bool   `json:"syncToRemote"`
    RepoPrivate   bool   `json:"repoPrivate"`
    RepoName      string `json:"repoName"`

    // New wizard fields
    ProjectType    string `json:"projectType"`    // web_app | mobile_app | desktop_app | backend_api | library | monorepo
    SubType        string `json:"subType"`        // nextjs | flutter | tauri | go_api ...
    DeployTarget   string `json:"deployTarget"`   // k8s | serverless | app_store | desktop_dist | package_registry
    TeamCadence    string `json:"teamCadence"`    // continuous | weekly | planned
    BranchStrategy string `json:"branchStrategy"` // TRUNK_BASED | GITHUB_FLOW | RELEASE_TRAIN (auto-derived if empty)
}
```

### 4.4 Frontend component structure

```
create-project-dialog.tsx (refactored)
  ├── ProjectTypeStep.tsx     -- Step 1: 6 project type cards
  ├── TechStackStep.tsx       -- Step 2: conditional tech stack options
  ├── DeployTargetStep.tsx    -- Step 3: deploy target selection
  ├── TeamCadenceStep.tsx     -- Step 4: release cadence
  └── ProjectConfirmStep.tsx  -- Summary + name/description + create button
```

Progress indicator: 顶部 5 个圆点 (1-2-3-4-confirm)，当前步骤紫色高亮，已完成步骤打钩。

### 4.5 Tasks

| # | Task | File(s) | Est. |
|---|------|---------|------|
| 2.1 | Extend CreateProjectRequest with wizard fields | `model.go` | 20min |
| 2.2 | Backend: auto-derive config from wizard input | `service.go` | 1h |
| 2.3 | Frontend: ProjectTypeStep component | `components/project-wizard/project-type-step.tsx` (new) | 1.5h |
| 2.4 | Frontend: TechStackStep component | `components/project-wizard/tech-stack-step.tsx` (new) | 1.5h |
| 2.5 | Frontend: DeployTargetStep + TeamCadenceStep | `components/project-wizard/` (new) | 1h |
| 2.6 | Frontend: ProjectConfirmStep + refactor dialog | `components/create-project-dialog.tsx` | 1h |
| 2.7 | Wire all steps together with state management | `create-project-dialog.tsx` | 1h |
| 2.8 | Manual test: create projects of each type | -- | 30min |

---

## 5. Day 3 -- Post-Import Onboarding Checklist

### 5.1 Checklist design

导入项目后，在项目详情页顶部显示 Onboarding Checklist overlay (dismissible)：

```
┌─────────────────────────────────────────────────────┐
│  项目接入引导                              [最小化]   │
│                                                     │
│  1. [check] 已连接 GitHub                            │
│  2. [spinner] 项目画像扫描中... (预计 30 秒)          │
│  3. [ ] 配置编码规范 → [去配置]                       │
│  4. [ ] 创建第一个版本 → [创建版本]                   │
│  5. [ ] 提交第一个需求 → [开始对话]                   │
│                                                     │
│  完成所有步骤后，你就可以让 AI 帮你写代码了            │
│  ─────────────────────────────────────────────────── │
│  进度: 2/5                              [跳过引导]   │
└─────────────────────────────────────────────────────┘
```

### 5.2 Checklist state management

**Backend**: 新增 `onboarding_status` JSONB 列到 `projects` 表：

```go
type OnboardingStatus struct {
    GithubConnected  bool      `json:"githubConnected"`
    ProfileScanned   bool      `json:"profileScanned"`
    SpecsConfigured  bool      `json:"specsConfigured"`
    FirstVersion     bool      `json:"firstVersion"`
    FirstRequirement bool      `json:"firstRequirement"`
    Dismissed        bool      `json:"dismissed"`        // user clicked "跳过引导"
    CompletedAt      *time.Time `json:"completedAt,omitempty"`
}
```

**状态更新触发**:

| Checklist Item | Trigger | How |
|---------------|---------|-----|
| 已连接 GitHub | 项目有 codePlatform=github + 有效 token | Import 时即为 true |
| 项目画像扫描完成 | DetectProjectType 完成写入 tech_stack | Background goroutine 完成时更新 |
| 配置编码规范 | 项目有至少 1 条 specs 记录 | Specs CRUD 时检查 |
| 创建第一个版本 | 项目有至少 1 条 version 记录 | Version 创建时检查 (SH-3/SH-4 交付后) |
| 提交第一个需求 | 项目有至少 1 条 task 记录 | Task 创建时检查 |

### 5.3 Frontend: Onboarding Checklist component

```
components/project/onboarding-checklist.tsx (new)
  - Renders on project detail pages when onboarding is not completed/dismissed
  - Polls GET /api/projects/{id}/onboarding every 5 seconds during scan
  - Each item: icon (check/spinner/empty) + text + optional action link
  - "跳过引导" button calls PATCH /api/projects/{id}/onboarding { dismissed: true }
  - "最小化" collapses to a small banner at top
  - Fully completed: auto-dismiss with confetti animation (optional, tasteful)
```

### 5.4 API endpoints

| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/projects/{id}/onboarding` | 获取 onboarding checklist 状态 |
| PATCH | `/api/projects/{id}/onboarding` | 更新 (dismiss, manual check-off) |

### 5.5 Database migration

```sql
-- Migration: Add onboarding_status to projects
ALTER TABLE projects ADD COLUMN IF NOT EXISTS onboarding_status JSONB DEFAULT '{}';

-- Backfill existing projects (mark github_connected based on code_platform)
UPDATE projects
SET onboarding_status = jsonb_build_object(
    'githubConnected', code_platform = 'github',
    'profileScanned', tech_stack IS NOT NULL AND tech_stack != '{}' AND tech_stack != 'null',
    'specsConfigured', false,
    'firstVersion', false,
    'firstRequirement', false,
    'dismissed', false
)
WHERE onboarding_status = '{}';
```

### 5.6 Tasks

| # | Task | File(s) | Est. |
|---|------|---------|------|
| 3.1 | DB migration: add onboarding_status column | `migrations/` (new) | 20min |
| 3.2 | Backend: OnboardingStatus model + repo methods | `model.go`, `repository.go` | 45min |
| 3.3 | Backend: GET/PATCH onboarding endpoints | `handler.go`, `router.go` | 45min |
| 3.4 | Backend: auto-update onboarding on import/task/specs events | `service.go` | 1h |
| 3.5 | Frontend: OnboardingChecklist component | `components/project/onboarding-checklist.tsx` (new) | 2h |
| 3.6 | Frontend: integrate into project detail pages | `app/(dashboard)/projects/[id]/layout.tsx` | 30min |
| 3.7 | Manual test: full import-to-onboarding flow | -- | 30min |

---

## 6. Risk & Mitigation

| Risk | Mitigation |
|------|-----------|
| Detection accuracy for uncommon project types | Start with high-confidence rules only; low-confidence results prompt user confirmation |
| Wizard adds friction to quick project creation | Keep "快速创建" link that bypasses wizard (goes straight to name + GitHub sync) |
| Onboarding checklist items depend on unbuilt features (versions) | Items for unbuilt features show "Coming soon" badge and link to nothing until available |
| GitHub API rate limit during file tree fetch for detection | Use conditional requests + cache; only fetch top 2 directory levels |

---

## 7. Acceptance Criteria

| # | Criterion | Verification |
|---|-----------|-------------|
| 1 | Import a Next.js project: profile shows projectType=web_app, subType=nextjs, confidence=high | Import test repo, check GET /api/projects/{id}/profile |
| 2 | Import a Flutter project: profile shows projectType=mobile_app, subType=flutter | Same |
| 3 | Import a Go API project: branchStrategy auto-set to TRUNK_BASED | Check project tech_stack JSONB |
| 4 | New project wizard: 4 steps complete with auto-config summary | UI manual test |
| 5 | Onboarding checklist appears after import, items update as user completes each step | Full flow test |
| 6 | "跳过引导" dismisses checklist permanently for that project | PATCH + reload |
| 7 | Existing projects (pre-migration) work without errors | Verify backfill migration |

---

*Plan version: 1.0 | Author: Claude + Harvey | Date: 2026-04-03*
