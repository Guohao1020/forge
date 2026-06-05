# Phase 3 — 前端:目录视图 + MCP 选择器 + zod

依赖:Phase 2(API)。产出:`packages/core` client + zod、`packages/views/mcp-catalog` 目录、
agent inspector 的 MCP 选择器(编辑 `mcp_refs`、版本、缺 secret 预检)。三包 typecheck 绿。

> 守 CLAUDE.md 红线:所有响应过 zod `parseWithFallback`;语义 token、不硬编码颜色;identical 组件进共享包。

---

### Task 3.1: 类型 + zod schema + client(core)

**Files:**
- Create: `packages/core/types/mcp-registry.ts`
- Modify: `packages/core/api/schemas.ts`(加 schema)
- Modify: `packages/core/api/*`(client 方法)
- Create: `packages/core/api/schemas.test.ts` 里加 loose/fallback 测

- [ ] **Step 1: 类型 + zod**

```ts
// packages/core/types/mcp-registry.ts
export interface MCPServerShape {
  name: string; version: string; transport: "stdio" | "sse" | "http";
  command?: string; args?: string[]; env_keys?: string[];
  url?: string; header_keys?: string[];
  lifecycle: "published" | "offline" | "draft"; tools?: string[];
}
export interface MCPRef { namespace: string; name: string; ref: string; }
```

```ts
// packages/core/api/schemas.ts (加)
import { z } from "zod";
export const MCPServerShapeSchema = z.object({
  name: z.string(), version: z.string(),
  transport: z.string(),            // loose: 后端枚举漂移降级
  command: z.string().optional(), args: z.array(z.string()).optional(),
  env_keys: z.array(z.string()).optional(),
  url: z.string().optional(), header_keys: z.array(z.string()).optional(),
  lifecycle: z.string(), tools: z.array(z.string()).optional(),
}).loose();
export const MCPServerListSchema = z.object({ servers: z.array(MCPServerShapeSchema) }).loose();
export const EMPTY_MCP_LIST = { servers: [] };
```

- [ ] **Step 2: client 方法**(走 Multica `/api/mcp-registry/*`,`parseWithFallback(MCPServerListSchema, ..., EMPTY_MCP_LIST)`):
  `listMCPServers(wsId)`、`getMCPServer(wsId,name,ns,ref)`、`registerMCPServer(wsId,ns,server)`、`setMCPLifecycle(...)`。

- [ ] **Step 3: schemas.test.ts 加**:喂缺字段 / 错类型 / `null` 数组 → 断言降级到 `EMPTY_MCP_LIST`,
  不抛进 UI(守住 API 兼容契约)。

- [ ] **Step 4: `pnpm --filter @multica/core exec vitest run api/schemas.test.ts`** → 绿。
- [ ] **Step 5: Commit** — `git commit -am "feat(core): mcp-registry types + zod + client"`

---

### Task 3.2: MCP 选择器(agent inspector)

**Files:**
- Create: `packages/views/agents/components/inspector/mcp-picker.tsx` + 测
- Modify: agent 详情页把选择器接进去(写 `mcp_refs`,经现有 `PUT /api/agents/{id}`)

- [ ] **Step 1: 选择器核心逻辑**:列目录(workspace+shared)、勾选 → 维护 `MCPRef[]`、每项可钉 ref
  (tag/version)、**缺 secret 预检**——对选中 server 的 `env_keys`/`header_keys`,比对该 agent
  `custom_env` 已有的 key,缺的红字提示"需在 agent env 配 `<KEY>`"。

```tsx
// 关键:预检纯函数,单测它
export function missingSecrets(server: MCPServerShape, agentEnvKeys: string[]): string[] {
  const need = [...(server.env_keys ?? []), ...(server.header_keys ?? [])];
  return need.filter((k) => !agentEnvKeys.includes(k));
}
```

- [ ] **Step 2: 选择器测(jsdom)**:`missingSecrets` 单测 + 渲染(勾选→onChange 出正确 `mcp_refs`;
  缺 secret 显示告警)。mock `@multica/core` client。

- [ ] **Step 3: 接进 agent 详情**:选择器值 ↔ `agent.mcp_refs`,保存走现有 agent update。

- [ ] **Step 4: `pnpm --filter @multica/views exec vitest run agents/.../mcp-picker.test.tsx`** → 绿。
- [ ] **Step 5: Commit** — `git commit -am "feat(views): MCP picker on agent inspector (refs + missing-secret preflight)"`

---

### Task 3.3: MCP 目录页(浏览 + 注册)

**Files:**
- Create: `packages/views/mcp-catalog/`(列表页 + 注册框 + 详情/版本)
- Modify: `apps/web` + desktop 路由各加一处(沿用 forge-checks/forge-standards 的接法)

- [ ] **Step 1: 目录列表页**:列 workspace+shared 的 server(名/版本/transport/lifecycle/tools);
  owner/admin 见"注册"按钮。Nacos 不可达 → 横幅 + 读到啥显示啥(client 已降级)。
- [ ] **Step 2: 注册框**:填 name/version/transport/shape(env_keys 等 KEY 名,**不收 secret 值**)→ 调 register。
- [ ] **Step 3: web 路由**(`apps/web/app/[workspaceSlug]/(dashboard)/mcp-catalog/page.tsx`)+ desktop 路由 +
  侧栏入口(对齐 forge-checks)。**identical 组件放共享包**(web/desktop 不复制)。
- [ ] **Step 4: 视图测**(列表渲染、注册框提交、Nacos-down 横幅)。
- [ ] **Step 5: `pnpm typecheck`(三包)** → 绿。
- [ ] **Step 6: Commit** — `git commit -am "feat(views): MCP catalog page (browse + register)"`
