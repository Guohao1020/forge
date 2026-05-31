# S16 — 项目画像与 AI 记忆 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** AI 自动扫描 GitHub 仓库生成全量项目画像（API 接口、DB Schema、模块依赖、架构图谱、业务规则），存储为结构化 JSONB，在代码生成时自动加载为 AI 上下文，实现跨任务的项目理解积累。

**Architecture:** 新增 ProfilerAgent（Python），通过 GitHub adapter 读取代码 → AI 分析提取结构化知识 → 存储到 PostgreSQL JSONB → ContextBuilder 加载画像到 AI 上下文。Go 端新增 profile 模块提供 CRUD API。

**Tech Stack:** Python 3.12 + Temporal, Go 1.22 + pgx, PostgreSQL JSONB

**Dependencies:** S3 (GitHub adapter), S6 (AI Worker)

---

## File Structure

### Go 后端

```
forge-core/
├── migrations/
│   └── 010_project_profile.sql              # NEW: project_profiles 表
├── internal/module/profile/
│   ├── model.go                             # NEW: ProfileEntry 模型
│   ├── repository.go                        # NEW: CRUD
│   ├── service.go                           # NEW: 画像管理 + 触发扫描
│   └── handler.go                           # NEW: HTTP endpoints
├── internal/router/router.go                # MODIFY: 注册路由
└── cmd/forge-core/main.go                   # MODIFY: 初始化模块
```

### Python AI Worker

```
ai-worker/src/
├── agents/profiler.py                       # NEW: 画像分析 Agent
├── activities/profile.py                    # NEW: 画像扫描 Temporal activity
└── context/builder.py                       # MODIFY: 加载画像到上下文
```

### 前端

```
forge-portal/
├── app/(dashboard)/projects/[id]/
│   └── profile/page.tsx                     # NEW: 项目画像展示页
├── components/project-sidebar.tsx           # MODIFY: 添加 "画像" 导航
└── lib/profile.ts                           # NEW: 画像 API 客户端
```

---

## Task 1: 数据库迁移 — project_profiles 表

**Files:**
- Create: `forge-core/migrations/010_project_profile.sql`

- [ ] **Step 1: 创建迁移文件**

```sql
-- S16: Project profile / AI memory storage
CREATE TABLE IF NOT EXISTS engine.project_profiles (
    id            BIGSERIAL PRIMARY KEY,
    project_id    BIGINT NOT NULL REFERENCES engine.projects(id),
    profile_key   VARCHAR(50) NOT NULL,   -- 'api_catalog', 'db_schema', 'module_graph', 'architecture', 'business_rules', 'coding_habits', 'quality_trends'
    profile_value JSONB NOT NULL DEFAULT '{}',
    version       INT NOT NULL DEFAULT 1,
    scanned_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_project_profiles_key ON engine.project_profiles(project_id, profile_key);
CREATE INDEX IF NOT EXISTS idx_project_profiles_project ON engine.project_profiles(project_id);

COMMENT ON TABLE engine.project_profiles IS '项目画像：AI 分析仓库后的结构化知识存储';
COMMENT ON COLUMN engine.project_profiles.profile_key IS '画像维度：api_catalog/db_schema/module_graph/architecture/business_rules/coding_habits/quality_trends';
```

- [ ] **Step 2: 验证迁移（启动 forge-core 后自动执行）**

```bash
cd forge-core && go build ./cmd/forge-core
```

- [ ] **Step 3: Commit**

```bash
git add forge-core/migrations/010_project_profile.sql
git commit -m "feat(s16): add project_profiles table for AI memory storage"
```

---

## Task 2: Go Profile 模块 — Model + Repository + Service + Handler

**Files:**
- Create: `forge-core/internal/module/profile/model.go`
- Create: `forge-core/internal/module/profile/repository.go`
- Create: `forge-core/internal/module/profile/service.go`
- Create: `forge-core/internal/module/profile/handler.go`
- Modify: `forge-core/internal/router/router.go`
- Modify: `forge-core/cmd/forge-core/main.go`

**重要**: 先读取 `forge-core/internal/module/pipeline/` 的 4 个文件了解模块模式。

- [ ] **Step 1: 创建 model.go**

```go
package profile

import (
	"encoding/json"
	"time"
)

type ProfileEntry struct {
	ID           int64           `json:"id"`
	ProjectID    int64           `json:"projectId"`
	ProfileKey   string          `json:"profileKey"`
	ProfileValue json.RawMessage `json:"profileValue"`
	Version      int             `json:"version"`
	ScannedAt    time.Time       `json:"scannedAt"`
	CreatedAt    time.Time       `json:"createdAt"`
	UpdatedAt    time.Time       `json:"updatedAt"`
}

// 画像维度常量
const (
	KeyAPICatalog    = "api_catalog"
	KeyDBSchema      = "db_schema"
	KeyModuleGraph   = "module_graph"
	KeyArchitecture  = "architecture"
	KeyBusinessRules = "business_rules"
	KeyCodingHabits  = "coding_habits"
	KeyQualityTrends = "quality_trends"
)

type ProfileListResponse struct {
	Profiles []ProfileEntry `json:"profiles"`
}

type ScanRequest struct {
	Keys []string `json:"keys"` // 可选，空=全量扫描
}
```

- [ ] **Step 2: 创建 repository.go**

```go
// ListByProject — 获取项目所有画像维度
func (r *Repository) ListByProject(ctx context.Context, projectID int64) ([]ProfileEntry, error)

// GetByKey — 获取单个维度
func (r *Repository) GetByKey(ctx context.Context, projectID int64, key string) (*ProfileEntry, error)

// Upsert — 创建或更新画像（按 project_id + profile_key 唯一）
func (r *Repository) Upsert(ctx context.Context, entry *ProfileEntry) error
```

Upsert SQL:
```sql
INSERT INTO engine.project_profiles (project_id, profile_key, profile_value, version, scanned_at)
VALUES ($1, $2, $3::jsonb, 1, NOW())
ON CONFLICT (project_id, profile_key)
DO UPDATE SET profile_value = $3::jsonb, version = project_profiles.version + 1, scanned_at = NOW(), updated_at = NOW()
RETURNING id, version, scanned_at, created_at, updated_at
```

- [ ] **Step 3: 创建 service.go**

```go
type Service struct {
	repo    *Repository
	authSvc AuthTokenProvider // GitHub token
}

// ListProfiles — 获取项目所有画像
func (s *Service) ListProfiles(ctx context.Context, projectID int64) ([]ProfileEntry, error)

// GetProfile — 获取单个维度
func (s *Service) GetProfile(ctx context.Context, projectID int64, key string) (*ProfileEntry, error)

// TriggerScan — 触发画像扫描（异步，通过 Temporal）
func (s *Service) TriggerScan(ctx context.Context, projectID, userID int64, keys []string) error
```

- [ ] **Step 4: 创建 handler.go**

3 个端点：
- `GET /api/projects/:id/profiles` — 列出所有画像
- `GET /api/projects/:id/profiles/:key` — 获取单个维度
- `POST /api/projects/:id/profiles/scan` — 触发扫描

- [ ] **Step 5: 注册路由 + 初始化模块**

router.go 添加路由，main.go 初始化 profile 模块。

- [ ] **Step 6: 验证构建**

```bash
cd forge-core && go build ./cmd/forge-core
```

- [ ] **Step 7: Commit**

```bash
git add forge-core/internal/module/profile/ forge-core/internal/router/router.go forge-core/cmd/forge-core/main.go
git commit -m "feat(s16): add profile module with CRUD API and scan trigger"
```

---

## Task 3: Python ProfilerAgent — AI 画像分析

**Files:**
- Create: `ai-worker/src/agents/profiler.py`
- Create: `ai-worker/src/activities/profile.py`
- Modify: `ai-worker/src/worker.py`

- [ ] **Step 1: 创建 ProfilerAgent**

`ai-worker/src/agents/profiler.py`:

```python
PROFILER_SYSTEM_PROMPT = """You are a senior software architect analyzing a codebase. Your task is to extract structured knowledge from source code files.

## Output Format
IMPORTANT: You MUST respond with ONLY a JSON object. No explanations, no markdown.

For api_catalog:
{"endpoints": [{"method": "GET", "path": "/api/users", "handler": "UserController.list", "params": [], "response": "User[]"}]}

For db_schema:
{"tables": [{"name": "users", "columns": [{"name": "id", "type": "BIGINT", "primary": true}], "indexes": [], "relations": []}]}

For module_graph:
{"modules": [{"name": "auth", "path": "internal/module/auth", "depends_on": ["db", "redis"], "exports": ["Service", "Handler"]}]}

For architecture:
{"services": [], "middleware": [], "databases": [], "caches": [], "message_queues": [], "patterns": ["Repository pattern", "DI via constructor"]}

For business_rules:
{"rules": [{"domain": "auth", "rule": "JWT expires after 8 hours", "source": "service.go:45"}]}
"""
```

- [ ] **Step 2: 创建 profile activity**

`ai-worker/src/activities/profile.py`:

```python
@activity.defn(name="scan_project_profile")
async def scan_project_profile_activity(input: ScanProfileInput) -> ScanProfileOutput:
    """
    1. Fetch file tree from forge-core API
    2. Fetch key files content (go.mod, main.go, router.go, models, migrations...)
    3. Feed to ProfilerAgent per dimension
    4. Save results back to forge-core API
    """
```

输入：project_id, user_id, keys（要扫描的维度列表）
输出：每个维度的扫描结果

智能文件选择策略：
- api_catalog → 扫描 router.go, handler.go, controller 文件
- db_schema → 扫描 migrations/, model.go, schema.sql
- module_graph → 扫描 go.mod/package.json + import 语句
- architecture → 扫描 main.go, docker-compose, config 文件
- business_rules → 扫描 service.go, 注释中的规则描述

- [ ] **Step 3: 注册 activity 到 worker.py**

读取 `ai-worker/src/worker.py`，添加 `scan_project_profile_activity` 注册。

- [ ] **Step 4: 验证 imports**

```bash
cd ai-worker && python -c "from src.activities.profile import scan_project_profile_activity; print('OK')"
```

- [ ] **Step 5: Commit**

```bash
git add ai-worker/src/agents/profiler.py ai-worker/src/activities/profile.py ai-worker/src/worker.py
git commit -m "feat(s16): add ProfilerAgent and scan activity for project knowledge extraction"
```

---

## Task 4: ContextBuilder 集成 — AI 上下文加载画像

**Files:**
- Modify: `ai-worker/src/context/builder.py`

- [ ] **Step 1: 读取现有 builder.py**

完整读取 `ai-worker/src/context/builder.py`。

- [ ] **Step 2: 添加画像加载到 build() 方法**

在 `build()` 方法中，新增第 4 个 API 调用：

```python
# Fetch project profiles (API catalog, DB schema, etc.)
try:
    profiles_resp = await self.client.get(
        f"{self.base_url}/api/projects/{project_id}/profiles",
        headers=self.headers,
    )
    if profiles_resp.status_code == 200:
        profiles_data = profiles_resp.json().get("data", {}).get("profiles", [])
        for p in profiles_data:
            key = p.get("profileKey", "")
            value = p.get("profileValue", {})
            if key and value:
                ctx.project_profiles[key] = value
except Exception as e:
    logger.warning(f"Failed to fetch project profiles: {e}")
```

- [ ] **Step 3: 更新 ProjectContext 和 to_system_prompt()**

在 ProjectContext 添加字段：
```python
project_profiles: dict = field(default_factory=dict)  # key → JSONB value
```

在 `to_system_prompt()` 中添加画像注入：
```python
if self.project_profiles:
    profile_parts = []
    for key, value in self.project_profiles.items():
        label = {"api_catalog": "API 接口清单", "db_schema": "数据库结构", "module_graph": "模块依赖图", "architecture": "技术架构", "business_rules": "业务规则"}.get(key, key)
        profile_parts.append(f"### {label}\n{json.dumps(value, ensure_ascii=False, indent=2)}")
    if profile_parts:
        parts.append("## 项目画像（AI 记忆）\n" + "\n\n".join(profile_parts))
```

- [ ] **Step 4: Commit**

```bash
git add ai-worker/src/context/builder.py
git commit -m "feat(s16): load project profiles into AI context for memory-enhanced generation"
```

---

## Task 5: 前端 — 项目画像展示页

**Files:**
- Create: `forge-portal/lib/profile.ts`
- Create: `forge-portal/app/(dashboard)/projects/[id]/profile/page.tsx`
- Modify: `forge-portal/components/project-sidebar.tsx`

- [ ] **Step 1: 创建 API 客户端 lib/profile.ts**

```typescript
import { api } from "./api";

export interface ProfileEntry {
  id: number;
  projectId: number;
  profileKey: string;
  profileValue: Record<string, unknown>;
  version: number;
  scannedAt: string;
}

export async function listProfiles(projectId: number): Promise<ProfileEntry[]> {
  const res = await api.get<{ profiles: ProfileEntry[] }>(`/projects/${projectId}/profiles`);
  return res.profiles || [];
}

export async function triggerScan(projectId: number, keys?: string[]): Promise<void> {
  return api.post(`/projects/${projectId}/profiles/scan`, { keys: keys || [] });
}
```

- [ ] **Step 2: 创建画像展示页**

`app/(dashboard)/projects/[id]/profile/page.tsx`:

布局：
```
┌────────────────────────────────────────────────────┐
│  项目画像                          [扫描更新] 按钮   │
├────────────────────────────────────────────────────┤
│                                                    │
│  ┌── API 接口清单 ──── v3 · 2h ago ─────────────┐ │
│  │  GET /api/users → UserController.list        │ │
│  │  POST /api/users → UserController.create     │ │
│  │  ...                                         │ │
│  └──────────────────────────────────────────────┘ │
│                                                    │
│  ┌── 数据库结构 ──── v2 · 1d ago ───────────────┐ │
│  │  users: id, name, email, created_at          │ │
│  │  orders: id, user_id, amount, status         │ │
│  └──────────────────────────────────────────────┘ │
│                                                    │
│  ┌── 模块依赖图 ──── v1 · 3d ago ───────────────┐ │
│  │  auth → [db, redis, jwt]                     │ │
│  │  api  → [auth, service]                      │ │
│  └──────────────────────────────────────────────┘ │
│                                                    │
│  ┌── 技术架构 ──── v1 · 3d ago ─────────────────┐ │
│  │  Services: Go API, Python AI Worker          │ │
│  │  Databases: PostgreSQL, Redis                │ │
│  │  Patterns: Repository, DI, Temporal          │ │
│  └──────────────────────────────────────────────┘ │
│                                                    │
│  ┌── 业务规则 ──── 暂无数据 ────────────────────┐ │
│  │  点击 "扫描更新" 生成画像                     │ │
│  └──────────────────────────────────────────────┘ │
│                                                    │
└────────────────────────────────────────────────────┘
```

每个维度用可折叠卡片，显示版本号和最后扫描时间。

- [ ] **Step 3: 更新 ProjectSidebar**

添加导航项：
```tsx
{ icon: Brain, label: "画像", href: `/projects/${projectId}/profile` },
```

- [ ] **Step 4: 验证前端构建**

```bash
cd forge-portal && npm run build
```

- [ ] **Step 5: Commit**

```bash
git add forge-portal/
git commit -m "feat(s16): add project profile page with dimension cards and scan trigger"
```

---

## Task 6: 构建验证 + Docker 重建

- [ ] **Step 1: Go 构建**

```bash
cd forge-core && go build ./cmd/forge-core
```

- [ ] **Step 2: 前端构建**

```bash
cd forge-portal && npm run build
```

- [ ] **Step 3: 重建 AI Worker**

```bash
docker compose -f docker-compose.dev.yml up -d --build ai-worker
```

- [ ] **Step 4: 端到端验证清单**

1. 启动所有服务
2. 进入已关联 GitHub 的项目
3. 左侧导航点击 "画像"
4. 点击 "扫描更新" 触发全量扫描
5. 等待扫描完成（查看 ai-worker 日志）
6. 刷新页面看到各维度画像数据
7. 创建新任务 → 检查 AI 是否使用了画像上下文

- [ ] **Step 5: Commit**

```bash
git add -A
git commit -m "feat(s16): complete project profile system with AI memory"
```

---

## 验收标准

- [ ] project_profiles 表创建成功
- [ ] 手动触发扫描 → AI 分析仓库代码 → 生成结构化画像
- [ ] 画像数据通过 API 可查询（7 个维度）
- [ ] ContextBuilder 在代码生成时加载画像
- [ ] 前端画像页展示各维度数据（版本号 + 扫描时间）
- [ ] 增量更新：重新扫描时 version +1
- [ ] `go build` + `npm run build` + ai-worker 重建通过
