## Phase 2 — 前端 core 接线 + 快照卡 + zod schema

**Goal:** types + zod schema(parseWithFallback)+ client 5 方法 + queries options + 快照卡 view + web 路由。
**Depends-on:** Phase 0　**Unblocks:** Phase 3
**Completion gate:** 三包 typecheck 绿。

> 镜像 F1–F4b 的 core 接线 + `packages/core/api/schemas.ts` 既有 zod 模式。CLAUDE.md 要求新端点的响应走
> `parseWithFallback(raw, Schema, EMPTY, { endpoint })`（见 `client.ts:415` getMe）。

---

### Task 2.1: types + zod schema

**Files:**
- Create: `packages/core/types/forge-health.ts`
- Modify: `packages/core/types/index.ts`
- Modify: `packages/core/api/schemas.ts`（追加 zod）

- [ ] **Step 1: 类型**

`packages/core/types/forge-health.ts`：
```ts
// Forge F5: Harness health observability.

export interface ForgeHealthCategoryCount { category: string; count: number; }
export interface ForgeHealth {
  standards: ForgeHealthCategoryCount[];
  standards_total: number;
  checks: number;
  review_configs: number;
  scans: number;
  gate: { passed: number; failed: number };
  review: { total: number; completed: number; avg_turnaround_sec: number };
  open_findings: number;
  scan_runs: number;
  fix_prs: { opened: number; merged: number; matched: number };
}
export interface ForgeTrendPoint { date: string; passed?: number; failed?: number; count?: number; }
export interface ForgeHealthTrends { findings: ForgeTrendPoint[]; gate: ForgeTrendPoint[]; fix_prs: ForgeTrendPoint[]; }
export interface ForgeIssueRef { issue_id: string; number: number; title: string; }
export interface ForgeFixPRRef { issue_id: string; number: number; title: string; pr_url: string; }
```

`packages/core/types/index.ts`,在 forge-entropy 导出之后加:
```ts
export type {
  ForgeHealth, ForgeHealthCategoryCount, ForgeTrendPoint, ForgeHealthTrends,
  ForgeIssueRef, ForgeFixPRRef,
} from "./forge-health";
```

- [ ] **Step 2: zod schema**

`packages/core/api/schemas.ts` 末尾追加（`z` 已在该文件 import）：
```ts
// Forge F5: Harness health
export const ForgeHealthSchema = z.object({
  standards: z.array(z.object({ category: z.string(), count: z.number() })),
  standards_total: z.number(),
  checks: z.number(),
  review_configs: z.number(),
  scans: z.number(),
  gate: z.object({ passed: z.number(), failed: z.number() }),
  review: z.object({ total: z.number(), completed: z.number(), avg_turnaround_sec: z.number() }),
  open_findings: z.number(),
  scan_runs: z.number(),
  fix_prs: z.object({ opened: z.number(), merged: z.number(), matched: z.number() }),
});
export const EMPTY_FORGE_HEALTH = {
  standards: [], standards_total: 0, checks: 0, review_configs: 0, scans: 0,
  gate: { passed: 0, failed: 0 },
  review: { total: 0, completed: 0, avg_turnaround_sec: 0 },
  open_findings: 0, scan_runs: 0, fix_prs: { opened: 0, merged: 0, matched: 0 },
};

const ForgeTrendPointSchema = z.object({
  date: z.string(),
  passed: z.number().optional(),
  failed: z.number().optional(),
  count: z.number().optional(),
});
export const ForgeHealthTrendsSchema = z.object({
  findings: z.array(ForgeTrendPointSchema),
  gate: z.array(ForgeTrendPointSchema),
  fix_prs: z.array(ForgeTrendPointSchema),
});
export const EMPTY_FORGE_TRENDS = { findings: [], gate: [], fix_prs: [] };

export const ForgeIssueRefListSchema = z.array(
  z.object({ issue_id: z.string(), number: z.number(), title: z.string() }),
);
export const ForgeFixPRRefListSchema = z.array(
  z.object({ issue_id: z.string(), number: z.number(), title: z.string(), pr_url: z.string() }),
);
```

- [ ] **Step 3: typecheck core**

Run: `cd D:\shulex_work\forge; corepack pnpm --filter "@multica/core" typecheck 2>&1 | Select-Object -Last 5`
Expected: Done。

- [ ] **Step 4: Commit**

```bash
git add packages/core/types/forge-health.ts packages/core/types/index.ts packages/core/api/schemas.ts
git commit -m "feat(forge): F5 health types + zod schemas"
```

---

### Task 2.2: client 方法 + queries options

**Files:**
- Modify: `packages/core/api/client.ts`
- Modify: `packages/core/workspace/queries.ts`

- [ ] **Step 1: client 5 方法**

`packages/core/api/client.ts`:import 块加(在既有 schema import 处)`ForgeHealthSchema, EMPTY_FORGE_HEALTH, ForgeHealthTrendsSchema, EMPTY_FORGE_TRENDS, ForgeIssueRefListSchema, ForgeFixPRRefListSchema`(from "./schemas")+ 类型 `ForgeHealth, ForgeHealthTrends, ForgeIssueRef, ForgeFixPRRef`(from "../types")。
在 `deleteForgeEntropyScan` 之后加:
```ts
  // Forge F5: Harness health observability
  async getForgeHealth(projectId?: string, days = 30): Promise<ForgeHealth> {
    const q = new URLSearchParams({ days: String(days) });
    if (projectId) q.set("project_id", projectId);
    const raw = await this.fetch<unknown>(`/api/forge/health?${q.toString()}`);
    return parseWithFallback(raw, ForgeHealthSchema, EMPTY_FORGE_HEALTH, {
      endpoint: "GET /api/forge/health",
    });
  }

  async getForgeHealthTrends(projectId?: string, days = 30): Promise<ForgeHealthTrends> {
    const q = new URLSearchParams({ days: String(days) });
    if (projectId) q.set("project_id", projectId);
    const raw = await this.fetch<unknown>(`/api/forge/health/trends?${q.toString()}`);
    return parseWithFallback(raw, ForgeHealthTrendsSchema, EMPTY_FORGE_TRENDS, {
      endpoint: "GET /api/forge/health/trends",
    });
  }

  async getForgeHealthFindings(projectId?: string): Promise<ForgeIssueRef[]> {
    const q = projectId ? `?project_id=${encodeURIComponent(projectId)}` : "";
    const raw = await this.fetch<unknown>(`/api/forge/health/findings${q}`);
    return parseWithFallback(raw, ForgeIssueRefListSchema, [], {
      endpoint: "GET /api/forge/health/findings",
    });
  }

  async getForgeHealthGateFailures(projectId?: string, days = 30): Promise<ForgeIssueRef[]> {
    const q = new URLSearchParams({ days: String(days) });
    if (projectId) q.set("project_id", projectId);
    const raw = await this.fetch<unknown>(`/api/forge/health/gate-failures?${q.toString()}`);
    return parseWithFallback(raw, ForgeIssueRefListSchema, [], {
      endpoint: "GET /api/forge/health/gate-failures",
    });
  }

  async getForgeHealthFixPRs(projectId?: string, days = 30): Promise<ForgeFixPRRef[]> {
    const q = new URLSearchParams({ days: String(days) });
    if (projectId) q.set("project_id", projectId);
    const raw = await this.fetch<unknown>(`/api/forge/health/fix-prs?${q.toString()}`);
    return parseWithFallback(raw, ForgeFixPRRefListSchema, [], {
      endpoint: "GET /api/forge/health/fix-prs",
    });
  }
```
> `this.fetch<unknown>` + `parseWithFallback` 完全镜像 `getMe`(client.ts:415)。`parseWithFallback` 已在文件 import。

- [ ] **Step 2: queries options**

`packages/core/workspace/queries.ts`,keys 加:
```ts
  forgeHealth: (wsId: string) => ["workspaces", wsId, "forge-health"] as const,
  forgeHealthTrends: (wsId: string) => ["workspaces", wsId, "forge-health-trends"] as const,
```
options 加:
```ts
export function forgeHealthOptions(wsId: string) {
  return queryOptions({
    queryKey: workspaceKeys.forgeHealth(wsId),
    queryFn: () => api.getForgeHealth(),
  });
}

export function forgeHealthTrendsOptions(wsId: string) {
  return queryOptions({
    queryKey: workspaceKeys.forgeHealthTrends(wsId),
    queryFn: () => api.getForgeHealthTrends(),
  });
}
```

- [ ] **Step 3: typecheck core**

Run: `cd D:\shulex_work\forge; corepack pnpm --filter "@multica/core" typecheck 2>&1 | Select-Object -Last 5`
Expected: Done。

- [ ] **Step 4: Commit**

```bash
git add packages/core/api/client.ts packages/core/workspace/queries.ts
git commit -m "feat(forge): F5 health client methods + query options"
```

---

### Task 2.3: 快照卡 view + web 路由

**Files:**
- Create: `packages/views/forge-health/forge-health-page.tsx`
- Create: `packages/views/forge-health/index.ts`
- Modify: `packages/views/package.json`
- Create: `apps/web/app/[workspaceSlug]/(dashboard)/forge-health/page.tsx`

- [ ] **Step 1: 快照卡 view**（趋势/钻取 Phase 3 再加）

`packages/views/forge-health/forge-health-page.tsx`：
```tsx
"use client";

import { useQuery } from "@tanstack/react-query";
import { useWorkspaceId } from "@multica/core/hooks";
import { forgeHealthOptions } from "@multica/core/workspace/queries";

function rate(num: number, den: number): string {
  if (den <= 0) return "—";
  return `${Math.round((num / den) * 100)}%`;
}

function Card({ label, value, sub }: { label: string; value: string; sub?: string }) {
  return (
    <div className="rounded-md border p-4">
      <div className="text-xs text-muted-foreground">{label}</div>
      <div className="mt-1 text-2xl font-semibold tabular-nums">{value}</div>
      {sub ? <div className="mt-1 text-xs text-muted-foreground">{sub}</div> : null}
    </div>
  );
}

export function ForgeHealthPage() {
  const wsId = useWorkspaceId();
  const { data: h, isLoading } = useQuery(forgeHealthOptions(wsId));

  if (isLoading || !h) {
    return <div className="p-6 text-sm text-muted-foreground">Loading…</div>;
  }

  const gateTotal = h.gate.passed + h.gate.failed;
  const mergeRate = h.fix_prs.matched > 0 ? rate(h.fix_prs.merged, h.fix_prs.opened) : "— · needs GitHub App";

  return (
    <div className="flex flex-1 min-h-0 flex-col gap-6 overflow-y-auto p-6">
      <div>
        <h1 className="text-lg font-semibold">Harness health</h1>
        <p className="text-xs text-muted-foreground">
          What the Forge Harness is doing across this workspace (last 30 days for rates).
        </p>
      </div>

      <section>
        <h2 className="mb-2 text-sm font-medium">Configured</h2>
        <div className="grid grid-cols-2 gap-3 md:grid-cols-4">
          <Card label="Standards (F1)" value={String(h.standards_total)} />
          <Card label="Checks (F2)" value={String(h.checks)} />
          <Card label="Reviewers (F3)" value={String(h.review_configs)} />
          <Card label="Entropy scans (F4)" value={String(h.scans)} />
        </div>
      </section>

      <section>
        <h2 className="mb-2 text-sm font-medium">Activity (last 30 days)</h2>
        <div className="grid grid-cols-2 gap-3 md:grid-cols-4">
          <Card label="Gate pass rate (F2)" value={rate(h.gate.passed, gateTotal)} sub={`${h.gate.passed} pass · ${h.gate.failed} fail`} />
          <Card label="Reviews (F3)" value={String(h.review.total)} sub={`${h.review.completed} completed · ${Math.round(h.review.avg_turnaround_sec / 60)}m avg`} />
          <Card label="Open findings (F4)" value={String(h.open_findings)} sub={`${h.scan_runs} scan runs`} />
          <Card label="Fix PRs (F4b)" value={String(h.fix_prs.opened)} sub={`merge rate ${mergeRate}`} />
        </div>
      </section>
    </div>
  );
}
```

`packages/views/forge-health/index.ts`：
```ts
export { ForgeHealthPage } from "./forge-health-page";
```

- [ ] **Step 2: package export + web 路由**

`packages/views/package.json`,在 `"./forge-entropy"` 之后加:
```json
    "./forge-health": "./forge-health/index.ts",
```

`apps/web/app/[workspaceSlug]/(dashboard)/forge-health/page.tsx`：
```tsx
export { ForgeHealthPage as default } from "@multica/views/forge-health";
```

- [ ] **Step 3: typecheck 三包**

Run: `cd D:\shulex_work\forge; corepack pnpm --filter "@multica/core" --filter "@multica/views" --filter "@multica/web" typecheck 2>&1 | Select-Object -Last 8`
Expected: 三包 Done。

- [ ] **Step 4: Commit**

```bash
git add packages/core/types/forge-health.ts packages/views/forge-health/ packages/views/package.json "apps/web/app/[workspaceSlug]/(dashboard)/forge-health/"
git commit -m "feat(forge): F5 Harness health snapshot view"
```

---

## Phase 2 完成检查
- [ ] types + zod schema + 5 client 方法 + queries options
- [ ] 快照卡 view + web 路由,三包 typecheck 绿
