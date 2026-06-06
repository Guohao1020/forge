# Phase 3 — 前端:provider 目录页 + provider 选择器 + 模型 picker 收敛 + zod

依赖:Phase 2(API)。产出:`packages/core` 类型/zod/client、`packages/views/llm-providers` 目录页、
agent inspector 的 provider 选择器(单选 + 缺-secret 预检)、**模型 picker 按 provider_ref 收敛**。
镜像 N1 前端(`mcp-catalog` + `inspector/mcp-picker.tsx`)。三包 typecheck 绿。

> 守 CLAUDE.md 红线:所有响应过 zod `parseWithFallback`;语义 token、不硬编码颜色;identical 组件进共享包。

---

### Task 3.1: 类型 + zod + client + query(core)

**Files:**
- Create: `packages/core/types/llm-provider.ts`
- Modify: `packages/core/types/index.ts`(barrel)、`packages/core/api/schemas.ts`、
  `packages/core/api/client.ts`、`packages/core/workspace/queries.ts`、`packages/core/api/schemas.test.ts`

- [ ] **Step 1: 类型**

```ts
// packages/core/types/llm-provider.ts
// Forge prometheus (N2): LLM provider catalog (Nacos config center). Shape only —
// auth_key names the secret; the value lives in each agent's env. protocol/
// lifecycle are string (not narrow unions) so backend enum drift downgrades.
export interface ProviderModel { id: string; label?: string; default?: boolean }
export interface ProviderShape {
  name: string;
  namespace?: string;
  version: string;
  protocol: string;      // "anthropic" | "codex-router"
  base_url: string;
  auth_key: string;      // KEY name, never the value
  wire_api?: string;
  models?: ProviderModel[];
  lifecycle: string;     // "published" | "offline" | "draft"
}
export interface ProviderRef { namespace: string; name: string; ref: string }
export interface ProviderList { providers: ProviderShape[] }
```

- [ ] **Step 2: barrel** — `types/index.ts` 加 `export type { ProviderShape, ProviderModel, ProviderRef, ProviderList } from "./llm-provider";`

- [ ] **Step 3: zod**(`schemas.ts`,镜像 `MCPServerListSchema`)

```ts
export const ProviderShapeSchema = z.object({
  name: z.string(),
  namespace: z.string().optional(),
  version: z.string(),
  protocol: z.string(),
  base_url: z.string(),
  auth_key: z.string(),
  wire_api: z.string().optional(),
  models: z.array(z.object({ id: z.string(), label: z.string().optional(), default: z.boolean().optional() }).loose()).optional(),
  lifecycle: z.string(),
}).loose();
export const ProviderListSchema = z.object({ providers: z.array(ProviderShapeSchema) }).loose();
export const EMPTY_PROVIDER_LIST: ProviderList = { providers: [] };
```
(在 `schemas.ts` 顶部 `import type { ... ProviderList }` 加 `ProviderList`。)

- [ ] **Step 4: client**(`client.ts`,镜像 `listMCPServers`)

```ts
async listProviders(): Promise<ProviderList> {
  const raw = await this.fetch<unknown>("/api/llm-providers");
  return parseWithFallback(raw, ProviderListSchema, EMPTY_PROVIDER_LIST, { endpoint: "GET /api/llm-providers" });
}
async getProvider(name: string, namespace?: string, ref?: string): Promise<ProviderShape> {
  const q = new URLSearchParams(); if (namespace) q.set("namespace", namespace); if (ref) q.set("ref", ref);
  const qs = q.toString() ? `?${q.toString()}` : "";
  const raw = await this.fetch<unknown>(`/api/llm-providers/${encodeURIComponent(name)}${qs}`);
  return parseWithFallback(raw, ProviderShapeSchema, { name, version: "", protocol: "", base_url: "", auth_key: "", lifecycle: "offline" }, { endpoint: "GET /api/llm-providers/{name}" });
}
async registerProvider(provider: ProviderShape, namespace?: string): Promise<void> {
  await this.fetch("/api/llm-providers", { method: "POST", body: JSON.stringify({ namespace, provider }) });
}
async setProviderLifecycle(name: string, version: string, lifecycle: string, namespace?: string): Promise<void> {
  const q = namespace ? `?namespace=${encodeURIComponent(namespace)}` : "";
  await this.fetch(`/api/llm-providers/${encodeURIComponent(name)}/lifecycle${q}`, { method: "PUT", body: JSON.stringify({ version, lifecycle }) });
}
```
(`client.ts` 顶部类型 import 加 `ProviderShape, ProviderList`;schema import 加 `ProviderListSchema, ProviderShapeSchema, EMPTY_PROVIDER_LIST`。)

- [ ] **Step 5: query**(`workspace/queries.ts`) — key `llmProviders: (wsId) => ["workspaces", wsId, "llm-providers"]` + `providerListOptions(wsId)` 调 `api.listProviders()`。

- [ ] **Step 6: schema 测**(`schemas.test.ts`,加 ProviderList describe):良构解析 / 未知 protocol 保留 /
  `.loose()` 透传 / `providers` 错类型|null|缺字段 → 降级 `EMPTY_PROVIDER_LIST`。
  注意 `noUncheckedIndexedAccess`:断言用 `parsed.providers[0]?.name`。

- [ ] **Step 7: `pnpm --filter @multica/core exec vitest run api/schemas.test.ts && pnpm --filter @multica/core typecheck`** → 绿。
- [ ] **Step 8: Commit** — `git commit -am "feat(core): llm-provider types + zod + client"`

---

### Task 3.2: provider 目录页 + web 路由

**Files:**
- Create: `packages/views/llm-providers/llm-providers-page.tsx` + `index.ts`
- Create: `apps/web/app/[workspaceSlug]/(dashboard)/llm-providers/page.tsx`
- Modify: `packages/views/package.json`(exports 加 `"./llm-providers": "./llm-providers/index.ts"`)

- [ ] **Step 1: 目录页**(照 `mcp-catalog/mcp-catalog-page.tsx` 改:列 provider 的
  name/version/protocol/base_url/lifecycle/所需 `auth_key`/models;register 表单收
  name/version/protocol(select anthropic/codex-router)/base_url/auth_key(KEY 名)/wire_api/shared 勾选;
  publish/offline 切换)。用 `providerListOptions` + `api.registerProvider` + `api.setProviderLifecycle`,
  invalidate `workspaceKeys.llmProviders(wsId)`。

- [ ] **Step 2: barrel + web 路由**
```ts
// packages/views/llm-providers/index.ts
export { LlmProvidersPage } from "./llm-providers-page";
```
```tsx
// apps/web/app/[workspaceSlug]/(dashboard)/llm-providers/page.tsx
export { LlmProvidersPage as default } from "@multica/views/llm-providers";
```

- [ ] **Step 3: `pnpm --filter @multica/web typecheck`** → 绿(若报找不到模块,确认 `package.json` exports 已加)。
- [ ] **Step 4: Commit** — `git commit -am "feat(views): LLM provider catalog page (browse + register)"`

---

### Task 3.3: provider 选择器 + 模型 picker 收敛(agent inspector)

**Files:**
- Create: `packages/views/agents/components/inspector/provider-picker.tsx` + `provider-picker.test.tsx`
- Modify: agent overview/model-picker 接入点(provider_ref 写 + 模型 picker 收敛)
- Modify: `packages/core/types/agent.ts`(`Agent.provider_ref?: ProviderRef | null`、
  `UpdateAgentRequest.provider_ref?: ProviderRef | null`)

- [ ] **Step 1: agent 类型** — `agent.ts` 顶部 `import type { ProviderRef } from "./llm-provider";`,
  `Agent` + `UpdateAgentRequest` 各加 `provider_ref?: ProviderRef | null`。

- [ ] **Step 2: provider 选择器**(单选,镜像 `mcp-picker.tsx` 但单选 + 缺-secret 对单 `auth_key`)

```tsx
// 关键:缺-secret 纯函数(单测)
export function providerMissingSecret(p: ProviderShape, agentEnvKeys: string[]): string | null {
  return p.auth_key && !agentEnvKeys.includes(p.auth_key) ? p.auth_key : null;
}
// ProviderPicker: useQuery(providerListOptions(wsId)) 列已发布 provider;单选(radio)→
//   onChange({namespace: p.namespace ?? wsId, name: p.name, ref: "stable"});选中显示缺-secret 红字。
//   value: ProviderRef | null。ProviderRefSection 取 agent env keys(api.getAgentEnv)+ 经 agent update 存。
```

- [ ] **Step 3: 选择器测**(jsdom,镜像 `mcp-picker.test.tsx`):`providerMissingSecret` 单测 +
  渲染(选中→onChange 出正确 `ProviderRef`;缺 `auth_key` 显红字;只列 published)。mock `@multica/core/api`
  的 `listProviders` + `@multica/core/hooks` 的 `useWorkspaceId`。

- [ ] **Step 4: 接进 agent 详情** — provider 选择器值 ↔ `agent.provider_ref`,保存走现有 agent update
  (`onUpdate(agent.id, { provider_ref })`)。放进 agent overview(可与 mcp_config tab 同区或新 "Provider" 区)。

- [ ] **Step 5: 模型 picker 收敛** — 找到 agent inspector 的模型 picker(`thinking-prop-row.tsx` 邻近,
  数据源是 runtime 动态发现 `InitiateListModels`)。改:**当 `agent.provider_ref` 存在**时,picker 选项
  改用 `api.getProvider(ref.name, ref.namespace, ref.ref).models[]`(useQuery 缓存),`label`=model.label||id;
  **未绑 ref 时保持今天的 runtime 发现链路不变**。加最小渲染测:有 provider_ref → 选项来自 provider.models。

- [ ] **Step 6: `pnpm --filter @multica/views exec vitest run agents/components/inspector/provider-picker.test.tsx && pnpm typecheck`**
  (core/views/web 三包;docs 包 fumadocs node_modules 缺失是预存环境问题,忽略)→ 绿。
- [ ] **Step 7: Commit** — `git commit -am "feat(views): provider picker + model-picker convergence on provider_ref"`
