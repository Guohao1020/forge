## Phase 4 — UI（forge-standards views + web 路由）

**Goal:** `packages/views/forge-standards/` 规范列表 + core/detail 双栏编辑 + profile_tags 多选；
project 设置页加 profile 编辑；web 路由接线。镜像 Multica 现有 `packages/views/skills/` 模式。

**Depends-on:** Phase 3（API）　**Unblocks:** Phase 5
**Completion gate:** `pnpm --filter @multica/views typecheck` 通过；页面可列出/编辑 standards。

> UI 严格沿用 Multica 既有约定（Base UI/shadcn、TanStack Query 服务端状态、Zustand 客户端状态、
> `packages/core/api` client）。**先读模板**：`packages/views/skills/`（列表+详情+编辑）与
> `packages/core/api/client.ts`、`packages/core/skills/`（query options 模式）作为一一对应的范本。

---

### Task 4.1: API client + 类型

**Files:**
- Create: `packages/core/types/forge-standard.ts`
- Create: `packages/core/forge-standards/` (query options + api 方法)
- Modify: `packages/core/api/client.ts`（加 forge standards 方法，参照现有 skill 方法）

- [ ] **Step 1: 类型定义**

`packages/core/types/forge-standard.ts`：
```ts
export interface ForgeStandard {
  id: string;
  workspace_id: string;
  project_id?: string;
  name: string;
  category: string;
  profile_tags: string[];
  core_content: string;
  detail_content: string;
  enabled: boolean;
}

export interface ForgeStandardInput {
  project_id?: string;
  name: string;
  category: string;
  profile_tags: string[];
  core_content: string;
  detail_content: string;
  enabled?: boolean;
}

export interface ForgeProjectProfile {
  tags: string[];
}
```

- [ ] **Step 2: api client 方法（镜像 skill 方法 + zod parse）**

参照 `packages/core/api/client.ts` 里 `listSkills/createSkill/...` 的写法，加
`listForgeStandards()`、`createForgeStandard(input)`、`getForgeStandard(id)`、
`updateForgeStandard(id, input)`、`deleteForgeStandard(id)`、`getForgeProjectProfile(projectId)`、
`putForgeProjectProfile(projectId, tags)`。**响应过 zod schema + parseWithFallback**（遵循
CLAUDE.md 的 API Response Compatibility 规则——不裸 `as` cast）。query options 放
`packages/core/forge-standards/`（镜像 `packages/core/skills/` 的 `*Options` 模式）。

- [ ] **Step 3: typecheck + commit**

Run: `pnpm --filter @multica/core typecheck 2>&1 | tail -5`
Expected: 通过。
```bash
git add packages/core/types/forge-standard.ts packages/core/forge-standards/ packages/core/api/
git commit -m "feat(forge): core types + api client for standards"
```

---

### Task 4.2: views — 列表 + 双栏编辑

**Files:**
- Create: `packages/views/forge-standards/forge-standards-page.tsx`
- Create: `packages/views/forge-standards/components/standard-editor.tsx`
- Create: `packages/views/forge-standards/components/standard-list.tsx`

- [ ] **Step 1: 列表页（镜像 skills 列表页）**

`forge-standards-page.tsx`：用 `useQuery(listForgeStandardsOptions(wsId))` 拉列表，
按 `category` / scope（project_id 有无）分组展示；顶部"New Standard"按钮开编辑器。
表格列：name、category、scope（workspace/project）、profile_tags、enabled。
样式用 Base UI/shadcn 组件（参照 `packages/views/skills/` 的表格/卡片）。

- [ ] **Step 2: 双栏编辑器**

`standard-editor.tsx`：表单 name + category（输入/下拉）+ profile_tags（多选 chips）+
**core_content / detail_content 双栏 markdown 编辑**（左 core 右 detail，或上下两块），
提交走 `createForgeStandard`/`updateForgeStandard` mutation（乐观更新 + 失败回滚 + settle
invalidate，遵循 CLAUDE.md 状态管理规则）。

- [ ] **Step 3: typecheck + commit**

Run: `pnpm --filter @multica/views typecheck 2>&1 | tail -5`
Expected: 通过。
```bash
git add packages/views/forge-standards/
git commit -m "feat(forge): standards list + dual-pane editor views"
```

---

### Task 4.3: 接线 web 路由 + project 设置 profile

**Files:**
- Create: `apps/web/app/[workspaceSlug]/(dashboard)/forge-standards/page.tsx`
- Modify: project 设置页（加 profile 标签编辑入口，参照现有 project settings 页）
- Modify: dashboard 侧边栏导航（加"规范中心"入口，参照现有导航项）

- [ ] **Step 1: web 页面（薄 wrapper，渲染共享 view）**

`forge-standards/page.tsx`：
```tsx
import { ForgeStandardsPage } from "@multica/views/forge-standards/forge-standards-page";
export default function Page() {
  return <ForgeStandardsPage />;
}
```
> 参照 `apps/web` 现有 dashboard 子页（如 skills/page.tsx）的 wrapper 写法。

- [ ] **Step 2: 侧边栏 + project 设置 profile 编辑**

在 dashboard 导航加"规范中心"链接到 `/{workspaceSlug}/forge-standards`（镜像现有导航项）。
在 project 设置页加一个 profile_tags 多选块，调用 `getForgeProjectProfile`/`putForgeProjectProfile`。

- [ ] **Step 3: typecheck + commit**

Run: `pnpm --filter @multica/web typecheck 2>&1 | tail -5`
Expected: 通过。
```bash
git add apps/web/app/ packages/views/
git commit -m "feat(forge): wire standards web route + project profile editor + nav"
```

---

## Phase 4 完成检查
- [ ] core 类型 + api client（zod 解析）typecheck 通过
- [ ] 列表 + 双栏编辑 views typecheck 通过
- [ ] web 路由 + 侧边栏 + project profile 接线，`pnpm --filter @multica/web typecheck` 通过
