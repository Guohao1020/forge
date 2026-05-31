# SP-3 -- 多平台制品构建策略

**Duration**: 3 days (after Infra-1)
**Priority**: P2 -- 不同项目类型需要不同的构建/部署流程
**Dependencies**: Infra-1 (K8s 环境), SP-1 (ProjectTypeProfile), SH-3a (版本模型)
**Outputs**: 构建模板引擎 + 分支策略per项目类型 + 签名与密钥管理

---

## 1. Goal

支持不同项目类型的构建和部署全流程：Web 应用构建 Docker 镜像部署到 K8s，移动应用构建 AAB/IPA 发布到 App Store，桌面应用交叉编译发布到 GitHub Releases，函数库发布到 npm/PyPI/Go Modules。每种类型自动配置对应的分支策略和密钥管理。

---

## 2. Current State Analysis

### 2.1 What exists

1. **S13 (制品管理) 设计**: milestone-plan.md 定义了 Docker 构建 + OSS 推送 + SemVer 版本管理，但仅覆盖 Docker 镜像这一种制品类型
2. **Branch naming**: 硬编码 `feature/{date}/{tenant}/{user}/{taskId}-{slug}`，所有项目同一策略
3. **GitHub client** (`forge-core/internal/adapter/github/client.go`): 支持 branch/commit/PR 操作，但无 release/tag 管理
4. **ProjectTypeProfile** (SP-1): 已有 artifactType 和 branchStrategy 字段，但无对应的构建执行逻辑

### 2.2 What is missing

| Gap | Impact |
|-----|--------|
| 只支持 Docker 镜像构建 | 移动/桌面/库项目无法使用 Forge 构建 |
| 无构建模板系统 | 每种项目类型的 CI/CD 配置需要从头编写 |
| 分支策略一刀切 | 移动应用需要 release train，库需要 tag-based release |
| 无签名/密钥管理 | 移动应用签名、npm token 等无处存放 |
| 无 release 管理 | 桌面/库项目需要 GitHub Releases + tag |

---

## 3. Day 1 -- Build Template Engine

### 3.1 Template directory structure

```
forge-core/internal/module/pipeline/templates/
  ├── web-k8s.yaml           # Web app → Docker → K8s
  ├── web-serverless.yaml    # Web app → static build → CDN/OSS
  ├── mobile-flutter.yaml    # Flutter → AAB/IPA → Store
  ├── mobile-rn.yaml         # React Native → AAB/IPA → Store
  ├── desktop-tauri.yaml     # Tauri → cross-compile → GitHub Releases
  ├── desktop-electron.yaml  # Electron → electron-builder → GitHub Releases
  ├── library-npm.yaml       # npm pack → registry publish
  ├── library-go.yaml        # GoReleaser → module tag
  ├── library-pypi.yaml      # Python build → PyPI publish
  └── api-docker.yaml        # Backend API → Docker → K8s
```

### 3.2 Template data model

```go
// forge-core/internal/module/pipeline/template.go (new)

// BuildTemplate defines a reusable build pipeline configuration.
type BuildTemplate struct {
    ID             string            `json:"id"`             // "web-k8s"
    ProjectType    string            `json:"projectType"`    // "web_app"
    DeployTarget   string            `json:"deployTarget"`   // "k8s"
    Name           string            `json:"name"`           // "Web App → Docker → K8s"
    Description    string            `json:"description"`
    Steps          []BuildStep       `json:"steps"`
    Variables      map[string]string `json:"variables"`      // Template variables with defaults
    RequiredSecrets []string         `json:"requiredSecrets"` // ["DOCKER_REGISTRY_URL", "K8S_NAMESPACE"]
}

type BuildStep struct {
    Name    string            `json:"name"`    // "build", "test", "push", "deploy"
    Type    string            `json:"type"`    // "docker_build" | "npm_publish" | "flutter_build" | "go_releaser" ...
    Image   string            `json:"image"`   // Container image for K8s Job execution
    Command []string          `json:"command"` // Build commands
    Env     map[string]string `json:"env"`     // Environment variables
    Timeout string            `json:"timeout"` // "10m"
    DependsOn []string        `json:"dependsOn"` // Previous step names
}
```

### 3.3 Template examples

**web-k8s.yaml** (Web App to Docker to K8s):

```yaml
id: web-k8s
projectType: web_app
deployTarget: k8s
name: "Web App -> Docker -> K8s"
description: "Multi-stage Docker build, push to registry, deploy to K8s"
steps:
  - name: build
    type: docker_build
    image: docker:24-dind
    command:
      - docker build
        --build-arg NODE_ENV=production
        -t {{registry}}/{{project_name}}:{{version}}
        -f Dockerfile .
    timeout: 10m

  - name: push
    type: docker_push
    image: docker:24-dind
    command:
      - docker push {{registry}}/{{project_name}}:{{version}}
    dependsOn: [build]
    timeout: 5m

  - name: deploy
    type: k8s_apply
    image: bitnami/kubectl:latest
    command:
      - kubectl set image deployment/{{project_name}}
        {{project_name}}={{registry}}/{{project_name}}:{{version}}
        -n {{namespace}}
    dependsOn: [push]
    timeout: 5m

variables:
  registry: "registry.cn-hangzhou.aliyuncs.com/forge"
  namespace: "tenant-{{tenant_id}}-{{env}}"
  version: "{{semver}}"
  project_name: "{{project.name}}"

requiredSecrets:
  - DOCKER_REGISTRY_URL
  - DOCKER_REGISTRY_USER
  - DOCKER_REGISTRY_PASSWORD
  - K8S_KUBECONFIG
```

**mobile-flutter.yaml** (Flutter to AAB/IPA):

```yaml
id: mobile-flutter
projectType: mobile_app
deployTarget: app_store
name: "Flutter -> AAB/IPA -> Store"
description: "Build Flutter app for Android (AAB) and iOS (IPA)"
steps:
  - name: build_android
    type: flutter_build
    image: ghcr.io/cirruslabs/flutter:latest
    command:
      - flutter build appbundle
        --release
        --build-number={{build_number}}
        --build-name={{version}}
    timeout: 15m

  - name: build_ios
    type: flutter_build
    image: ghcr.io/cirruslabs/flutter:latest
    command:
      - flutter build ipa
        --release
        --build-number={{build_number}}
        --build-name={{version}}
        --export-options-plist=ios/ExportOptions.plist
    timeout: 15m

  - name: upload_android
    type: store_upload
    command:
      - fastlane android deploy
    dependsOn: [build_android]
    timeout: 10m

  - name: upload_ios
    type: store_upload
    command:
      - fastlane ios deploy
    dependsOn: [build_ios]
    timeout: 10m

variables:
  version: "{{semver}}"
  build_number: "{{auto_increment}}"

requiredSecrets:
  - ANDROID_KEYSTORE
  - ANDROID_KEYSTORE_PASSWORD
  - ANDROID_KEY_ALIAS
  - ANDROID_KEY_PASSWORD
  - APP_STORE_CONNECT_API_KEY
  - APP_STORE_CONNECT_ISSUER_ID
  - IOS_PROVISIONING_PROFILE
```

**library-npm.yaml** (npm Package Publish):

```yaml
id: library-npm
projectType: library
deployTarget: package_registry
name: "npm Package -> Registry"
description: "Build, test, and publish npm package"
steps:
  - name: build
    type: npm_build
    image: node:20-alpine
    command:
      - npm ci
      - npm run build
    timeout: 5m

  - name: test
    type: npm_test
    image: node:20-alpine
    command:
      - npm test
    dependsOn: [build]
    timeout: 5m

  - name: publish
    type: npm_publish
    image: node:20-alpine
    command:
      - npm version {{version}} --no-git-tag-version
      - npm publish --access public
    dependsOn: [test]
    timeout: 3m

variables:
  version: "{{semver}}"

requiredSecrets:
  - NPM_TOKEN
```

**desktop-tauri.yaml** (Tauri Cross-Compile):

```yaml
id: desktop-tauri
projectType: desktop_app
deployTarget: desktop_dist
name: "Tauri -> Cross-Compile -> GitHub Releases"
description: "Build Tauri desktop app for macOS, Windows, Linux; publish to GitHub Releases"
steps:
  - name: build_linux
    type: tauri_build
    image: ghcr.io/nicehash/tauri-builder:latest
    command:
      - cargo tauri build --target x86_64-unknown-linux-gnu
    timeout: 20m

  - name: build_macos
    type: tauri_build
    command:
      - cargo tauri build --target universal-apple-darwin
    timeout: 20m

  - name: build_windows
    type: tauri_build
    command:
      - cargo tauri build --target x86_64-pc-windows-msvc
    timeout: 20m

  - name: release
    type: github_release
    command:
      - gh release create v{{version}}
        target/release/bundle/**/*
        --title "v{{version}}"
        --generate-notes
    dependsOn: [build_linux, build_macos, build_windows]
    timeout: 5m

variables:
  version: "{{semver}}"

requiredSecrets:
  - APPLE_SIGNING_IDENTITY  # optional
  - WINDOWS_SIGNING_CERT    # optional
```

### 3.4 Template selection logic

```go
// SelectTemplate picks the right build template based on ProjectTypeProfile.
func SelectTemplate(profile *project.ProjectTypeProfile) (*BuildTemplate, error) {
    key := fmt.Sprintf("%s-%s", mapProjectTypeToTemplatePrefix(profile.ProjectType), mapDeployTargetToTemplateSuffix(profile.DeployTarget))

    // Direct match
    if tmpl, ok := templates[key]; ok {
        return tmpl, nil
    }

    // Fallback: match by projectType only (use default deployTarget)
    defaults := map[string]string{
        "web_app":     "web-k8s",
        "mobile_app":  "mobile-flutter",
        "desktop_app": "desktop-tauri",
        "backend_api": "api-docker",
        "library":     "library-npm",
        "monorepo":    "web-k8s",  // default; monorepo needs special handling
    }

    if defaultKey, ok := defaults[profile.ProjectType]; ok {
        return templates[defaultKey], nil
    }

    return nil, fmt.Errorf("no build template for project type %s", profile.ProjectType)
}
```

### 3.5 Tasks

| # | Task | File(s) | Est. |
|---|------|---------|------|
| 1.1 | Define BuildTemplate and BuildStep data model | `pipeline/template.go` (new) | 30min |
| 1.2 | Write web-k8s and api-docker templates | `pipeline/templates/` (new) | 45min |
| 1.3 | Write mobile-flutter and mobile-rn templates | `pipeline/templates/` (new) | 45min |
| 1.4 | Write desktop-tauri and desktop-electron templates | `pipeline/templates/` (new) | 30min |
| 1.5 | Write library-npm, library-go, library-pypi templates | `pipeline/templates/` (new) | 45min |
| 1.6 | Write web-serverless template | `pipeline/templates/` (new) | 20min |
| 1.7 | Template loader + SelectTemplate function | `pipeline/template.go` | 1h |
| 1.8 | API endpoint: GET /api/projects/{id}/build-template | `pipeline/handler.go` | 45min |
| 1.9 | Unit tests for template selection | `pipeline/template_test.go` | 45min |

---

## 4. Day 2 -- Branch Strategy Per Project Type

### 4.1 BranchStrategy enum and configuration

```go
// forge-core/internal/module/project/branch_strategy.go (new)

type BranchStrategy string

const (
    BranchTrunkBased  BranchStrategy = "TRUNK_BASED"
    BranchGitHubFlow  BranchStrategy = "GITHUB_FLOW"
    BranchReleaseTrain BranchStrategy = "RELEASE_TRAIN"
)

// BranchConfig defines how branches are named and managed for a project.
type BranchConfig struct {
    Strategy        BranchStrategy `json:"strategy"`
    MainBranch      string         `json:"mainBranch"`      // "main"
    FeaturePattern  string         `json:"featurePattern"`  // "feature/{date}/{tenant}/{user}/{taskId}-{slug}"
    ReleasePattern  string         `json:"releasePattern"`  // "release/{version}" (RELEASE_TRAIN only)
    HotfixPattern   string         `json:"hotfixPattern"`   // "hotfix/{date}/{slug}" (RELEASE_TRAIN only)
    AutoMerge       bool           `json:"autoMerge"`       // Auto-merge low-risk PRs to main
    AutoTag         bool           `json:"autoTag"`         // Auto-create tag on release (GITHUB_FLOW)
    AutoRelease     bool           `json:"autoRelease"`     // Auto-create GitHub Release on tag
}
```

### 4.2 Strategy behavior matrix

| Behavior | TRUNK_BASED | GITHUB_FLOW | RELEASE_TRAIN |
|----------|-------------|-------------|---------------|
| Feature branch | `feature/{date}/.../{taskId}-{slug}` | same | same |
| Merge target | `main` | `main` | `main` |
| Release trigger | merge to main -> auto-deploy | tag creation -> auto-publish | version status TESTING -> create `release/{ver}` |
| Hotfix flow | feature branch -> main | feature branch -> main | `hotfix/{date}/{slug}` -> `release/{ver}` + cherry-pick to main |
| Version tagging | implicit (commit SHA) | `v{major}.{minor}.{patch}` | `v{major}.{minor}.{patch}` |
| Deploy target | K8s rolling update | package registry | App Store / GitHub Releases |

### 4.3 Modify GitHub client for release management

Extend `forge-core/internal/adapter/github/client.go`:

```go
// New methods on Client:

// CreateTag creates a lightweight tag on the given commit SHA.
func (c *Client) CreateTag(ctx context.Context, owner, repo, tag, sha string) error

// CreateRelease creates a GitHub Release with auto-generated notes.
func (c *Client) CreateRelease(ctx context.Context, owner, repo string, opts CreateReleaseOpts) (*Release, error)

// CreateReleaseBranch creates a release/{version} branch from main.
func (c *Client) CreateReleaseBranch(ctx context.Context, owner, repo, version string) error

// CherryPick applies a commit to a target branch (for hotfix flows).
// Note: GitHub API does not natively support cherry-pick. This creates a
// new commit on the target branch with the same changes.
func (c *Client) CherryPick(ctx context.Context, owner, repo, commitSHA, targetBranch string) error

type CreateReleaseOpts struct {
    TagName         string
    TargetCommitish string // branch or commit SHA
    Name            string
    Body            string
    Draft           bool
    Prerelease      bool
    GenerateNotes   bool // Use GitHub auto-generated release notes
}

type Release struct {
    ID        int64  `json:"id"`
    TagName   string `json:"tag_name"`
    Name      string `json:"name"`
    HTMLURL   string `json:"html_url"`
    CreatedAt string `json:"created_at"`
}
```

### 4.4 Auto-create release branches (RELEASE_TRAIN)

When a project version status changes to TESTING (from SH-3a version model):

```go
// In version service or orchestrator:
func (s *Service) onVersionStatusChange(ctx context.Context, projectID int64, version string, newStatus string) error {
    project, _ := s.projectRepo.GetByID(ctx, projectID)
    branchConfig := project.GetBranchConfig()

    if branchConfig.Strategy == BranchReleaseTrain && newStatus == "TESTING" {
        // Auto-create release branch: release/v1.2.0
        releaseBranch := fmt.Sprintf("release/v%s", version)
        err := s.githubClient.CreateReleaseBranch(ctx, owner, repo, version)
        if err != nil {
            return fmt.Errorf("create release branch: %w", err)
        }
        slog.Info("created release branch", "branch", releaseBranch, "version", version)
    }

    return nil
}
```

### 4.5 Hotfix flow for mobile apps in production

```
RELEASE_TRAIN hotfix flow:

1. PM reports critical bug in v1.2.0 (already in App Store)
2. Create task with urgency=critical, target_version=1.2.1
3. AI creates hotfix branch: hotfix/{date}/{slug} FROM release/v1.2.0
4. AI generates fix
5. Fix merged to release/v1.2.0 branch
6. Cherry-pick fix to main (to prevent regression in next version)
7. Rebuild from release/v1.2.0 branch
8. Submit to App Store as v1.2.1
```

### 4.6 Tasks

| # | Task | File(s) | Est. |
|---|------|---------|------|
| 2.1 | Define BranchStrategy enum and BranchConfig model | `branch_strategy.go` (new) | 30min |
| 2.2 | Add GetBranchConfig() to Project (derive from TechStack) | `model.go` | 30min |
| 2.3 | Implement CreateTag, CreateRelease on GitHub client | `adapter/github/client.go` | 1h |
| 2.4 | Implement CreateReleaseBranch on GitHub client | `adapter/github/client.go` | 45min |
| 2.5 | Implement CherryPick on GitHub client | `adapter/github/client.go` | 1h |
| 2.6 | Auto-create release branch on version TESTING (hook) | `project/service.go` or `version/service.go` | 1h |
| 2.7 | Modify branch naming to use BranchConfig patterns | `adapter/github/client.go` (existing branch creation) | 45min |
| 2.8 | Unit tests for branch strategy logic | `branch_strategy_test.go` | 45min |
| 2.9 | API endpoint: GET/PUT /api/projects/{id}/branch-config | `handler.go`, `router.go` | 45min |

---

## 5. Day 3 -- Signing and Secrets Management

### 5.1 Platform-specific secrets model

```go
// forge-core/internal/module/project/secrets.go (new)

// ProjectSecret stores an encrypted platform-specific secret for build/deploy.
type ProjectSecret struct {
    ID         int64     `json:"id"`
    ProjectID  int64     `json:"projectId"`
    TenantID   int64     `json:"tenantId"`
    Category   string    `json:"category"`   // "mobile_android" | "mobile_ios" | "desktop" | "library" | "docker"
    Key        string    `json:"key"`        // "ANDROID_KEYSTORE" | "NPM_TOKEN" | etc.
    Value      string    `json:"-"`          // Encrypted, never exposed via API
    Encrypted  bool      `json:"encrypted"`
    CreatedAt  time.Time `json:"createdAt"`
    UpdatedAt  time.Time `json:"updatedAt"`
}

// SecretRequirement describes a secret needed for a build template.
type SecretRequirement struct {
    Key         string `json:"key"`
    Category    string `json:"category"`
    Description string `json:"description"` // "Android keystore file for signing APK/AAB"
    Required    bool   `json:"required"`    // false = optional (e.g., code signing cert for desktop)
    FileType    bool   `json:"fileType"`    // true = binary file (keystore, cert), false = text (token, password)
    Configured  bool   `json:"configured"`  // true if already set for this project
}
```

### 5.2 Secrets per project type

| Category | Secrets | Required for |
|----------|---------|-------------|
| `mobile_android` | ANDROID_KEYSTORE (file), ANDROID_KEYSTORE_PASSWORD, ANDROID_KEY_ALIAS, ANDROID_KEY_PASSWORD | Flutter/RN Android build |
| `mobile_ios` | APP_STORE_CONNECT_API_KEY, APP_STORE_CONNECT_ISSUER_ID, IOS_PROVISIONING_PROFILE (file) | Flutter/RN iOS build |
| `desktop` | APPLE_SIGNING_IDENTITY (optional), WINDOWS_SIGNING_CERT (optional, file) | Tauri/Electron code signing |
| `library_npm` | NPM_TOKEN | npm publish |
| `library_pypi` | PYPI_TOKEN | PyPI publish |
| `library_go` | (none -- Go modules use git tags, no registry auth) | -- |
| `docker` | DOCKER_REGISTRY_URL, DOCKER_REGISTRY_USER, DOCKER_REGISTRY_PASSWORD | Docker push to ACR |
| `k8s` | K8S_KUBECONFIG (file) | K8s deployment |

### 5.3 Encryption approach

Use existing SOPS+age pattern (aligned with technical-design.md):

```go
// Encrypt before storage:
func EncryptSecret(plaintext string) (string, error) {
    // Use age encryption with platform master key
    // Key stored in K8s Secret / environment variable
    // Ciphertext stored in PostgreSQL
}

// Decrypt at build time:
func DecryptSecret(ciphertext string) (string, error) {
    // Decrypt using age identity
    // Only called by build pipeline (K8s Job)
    // Never returns plaintext to API responses
}
```

### 5.4 Warning system for missing secrets

When a task reaches the DEPLOY/BUILD step, check if all required secrets are configured:

```go
func CheckBuildReadiness(ctx context.Context, projectID int64) []BuildWarning {
    profile := getProjectProfile(projectID)
    template := SelectTemplate(profile)
    secrets := getProjectSecrets(projectID)

    var warnings []BuildWarning
    for _, req := range template.RequiredSecrets {
        if !secretExists(secrets, req) {
            warnings = append(warnings, BuildWarning{
                Level:   "ERROR",
                Message: fmt.Sprintf("缺少 %s — 无法执行 %s", req, template.Name),
                Action:  fmt.Sprintf("前往 项目设置 → 构建与部署 配置 %s", req),
            })
        }
    }
    return warnings
}
```

Warnings surface in:
1. Task workflow: before starting build step, check and surface warning to user
2. Project settings: "Build & Deploy" tab shows checklist of configured/missing secrets
3. Onboarding checklist (SP-1): if project type requires secrets, add a checklist item

### 5.5 Frontend: Build & Deploy settings tab

Add a new tab to project settings page (`forge-portal/app/(dashboard)/projects/[id]/settings/page.tsx`):

```
Project Settings
├── General (existing)
├── Build & Deploy (new)  ← SP-3
│   ├── Build Template: [auto-detected] "Web App -> Docker -> K8s" [change]
│   ├── Branch Strategy: [auto-detected] TRUNK_BASED [change]
│   ├── Required Secrets:
│   │   ├── [check] DOCKER_REGISTRY_URL    [configured]
│   │   ├── [check] DOCKER_REGISTRY_USER   [configured]
│   │   ├── [warn]  DOCKER_REGISTRY_PASSWORD [configure]
│   │   └── [check] K8S_KUBECONFIG          [configured]
│   └── Build History: (link to build logs)
└── Specs (existing)
```

### 5.6 Database migration

```sql
-- Migration: project_secrets table
CREATE TABLE IF NOT EXISTS project_secrets (
    id BIGSERIAL PRIMARY KEY,
    project_id BIGINT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    tenant_id BIGINT NOT NULL,
    category VARCHAR(50) NOT NULL,
    key VARCHAR(100) NOT NULL,
    value TEXT NOT NULL,        -- Encrypted with age
    encrypted BOOLEAN DEFAULT true,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW(),
    UNIQUE(project_id, key)
);

CREATE INDEX idx_project_secrets_project ON project_secrets(project_id);
CREATE INDEX idx_project_secrets_tenant ON project_secrets(tenant_id);
```

### 5.7 API endpoints

| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/projects/{id}/secrets` | List secret requirements + configured status (never returns values) |
| PUT | `/api/projects/{id}/secrets/{key}` | Set/update a secret (encrypted before storage) |
| DELETE | `/api/projects/{id}/secrets/{key}` | Remove a secret |
| GET | `/api/projects/{id}/build-readiness` | Check if all required secrets are configured |

### 5.8 Tasks

| # | Task | File(s) | Est. |
|---|------|---------|------|
| 3.1 | DB migration: project_secrets table | `migrations/` (new) | 20min |
| 3.2 | ProjectSecret model + SecretRequirement | `secrets.go` (new) | 30min |
| 3.3 | Secret repository (CRUD with encryption) | `project/repository.go` | 1h |
| 3.4 | Secret API endpoints (GET/PUT/DELETE) | `project/handler.go`, `router.go` | 1h |
| 3.5 | BuildReadiness checker | `pipeline/readiness.go` (new) | 45min |
| 3.6 | Integration: check readiness before build step in workflow | `temporal/workflow/` | 45min |
| 3.7 | Frontend: Build & Deploy settings tab | `projects/[id]/settings/page.tsx` | 2h |
| 3.8 | Frontend: secret input form (text + file upload) | `components/project/secret-form.tsx` (new) | 1h |
| 3.9 | Manual test: configure secrets for a Flutter project | -- | 30min |

---

## 6. Template Variable Resolution

### 6.1 Variable sources

When a build template is instantiated for a specific task:

```go
// ResolveTemplateVariables fills in template variables from multiple sources.
func ResolveTemplateVariables(tmpl *BuildTemplate, ctx *BuildContext) map[string]string {
    vars := make(map[string]string)

    // 1. Template defaults
    for k, v := range tmpl.Variables {
        vars[k] = v
    }

    // 2. Project-level overrides
    vars["project_name"] = ctx.Project.Name
    vars["tenant_id"] = strconv.FormatInt(ctx.Project.TenantID, 10)

    // 3. Version info
    vars["version"] = ctx.Version        // "1.2.0"
    vars["semver"] = ctx.Version
    vars["build_number"] = ctx.BuildNumber // auto-incremented

    // 4. Environment
    vars["env"] = ctx.Environment         // "dev" | "staging" | "prod"
    vars["namespace"] = fmt.Sprintf("tenant-%d-%s", ctx.Project.TenantID, ctx.Environment)

    // 5. Secrets (injected as env vars at runtime, not embedded in template)
    // Secrets are mounted via K8s Secret volumes, not resolved here

    return vars
}
```

### 6.2 Template override mechanism

Users can override template defaults per project in the Build & Deploy settings:

```go
type BuildConfigOverride struct {
    ProjectID        int64             `json:"projectId"`
    TemplateID       string            `json:"templateId"`       // "web-k8s" (can switch template)
    VariableOverrides map[string]string `json:"variableOverrides"` // Override specific variables
    StepOverrides    []StepOverride    `json:"stepOverrides"`     // Disable/modify specific steps
}

type StepOverride struct {
    StepName string `json:"stepName"`
    Disabled bool   `json:"disabled"` // Skip this step
    Command  string `json:"command"`  // Override command
}
```

---

## 7. Risk & Mitigation

| Risk | Mitigation |
|------|-----------|
| Mobile build requires macOS (iOS) / specific SDKs | Phase 1: document limitation; Phase 2: self-hosted macOS runners or cloud build services |
| Template explosion as more project types are added | Generic template inheritance: base template + per-type overrides |
| Secret leakage in build logs | Strip secrets from all log output; use K8s Secret mount, not env vars in command |
| Tauri cross-compile is complex (3 OS targets) | Start with Linux only; macOS/Windows as stretch goals (needs cross-compile toolchain) |
| User confusion about which secrets to configure | Clear descriptions + links to platform docs for each secret |

---

## 8. Acceptance Criteria

| # | Criterion | Verification |
|---|-----------|-------------|
| 1 | SelectTemplate("web_app", "k8s") returns web-k8s template | Unit test |
| 2 | SelectTemplate("mobile_app", "app_store") returns mobile-flutter | Unit test |
| 3 | Template variables are correctly resolved with project context | Unit test |
| 4 | GitHub client can create tag + release | Integration test with GitHub API |
| 5 | Release branch auto-created when version status = TESTING | Flow test (requires SH-3a) |
| 6 | Secrets stored encrypted, never returned in API GET | API test + DB inspection |
| 7 | Build readiness check surfaces missing secrets | API test |
| 8 | Build & Deploy settings tab shows all required secrets with status | UI test |
| 9 | Cherry-pick from hotfix branch to main works | Integration test |

---

*Plan version: 1.0 | Author: Claude + Harvey | Date: 2026-04-03*
