## Phase 1 — 前端 types/zod/badge

**Goal:** `ForgeHealth` 类型 + zod schema + EMPTY 加 `score`/`status`/`no_activity`;`forge-health-page` 顶部加健康分 badge。
**Depends-on:** Phase 0　**Unblocks:** Phase 2
**Completion gate:** 三包 typecheck 绿。

> 状态色用既有 status-semantic 色(chat-window 的 `bg-amber-500` 状态点、billing 的 green/red/amber 同款)——
> 状态语义色,非装饰色,符合 CLAUDE.md。

---

### Task 1.1: 类型 + zod + EMPTY

**Files:**
- Modify: `packages/core/types/forge-health.ts`
- Modify: `packages/core/api/schemas.ts`

- [ ] **Step 1: 类型加字段**

`packages/core/types/forge-health.ts`:`ForgeHealth` interface 末尾(`fix_prs` 之后)加:
```ts
  score: number;
  status: string;
  no_activity: boolean;
```

- [ ] **Step 2: zod schema + EMPTY**

`packages/core/api/schemas.ts`:`ForgeHealthSchema` 的 `z.object({...})` 内(`fix_prs` 之后)加:
```ts
  score: z.number(),
  status: z.string(),
  no_activity: z.boolean(),
```
（保持该 object 末尾的 `.loose()` 不变。）

`EMPTY_FORGE_HEALTH` 对象末尾加:
```ts
  score: 0, status: "red", no_activity: true,
```

- [ ] **Step 3: 修 F5 schema 测试 fixture（必做)**

`ForgeHealthSchema` 新增的 3 个字段是**必填**,会让 F5 既有测试 `packages/core/api/schemas.test.ts` 里
`describe("Forge F5 health schemas")` 的 "parses a valid health response" 用例失败(其 `valid` fixture 缺这 3 字段
→ 校验失败 → 回退 EMPTY → `toEqual(valid)` 不匹配)。在该 `valid` 对象里补上:
```ts
  score: 80, status: "green", no_activity: false,
```
（"tolerates unknown server-added fields" 用例基于 `EMPTY_FORGE_HEALTH` + future_field,EMPTY 已含三字段,不受影响;
"malformed" 用例仍回退 EMPTY,不受影响。)
跑 `cd D:\shulex_work\forge; corepack pnpm --filter "@multica/core" exec vitest run api/schemas.test.ts 2>&1 | Select-Object -Last 10` 确认全绿。

- [ ] **Step 4: typecheck core**

Run: `cd D:\shulex_work\forge; corepack pnpm --filter "@multica/core" typecheck 2>&1 | Select-Object -Last 5`
Expected: Done。

- [ ] **Step 5: Commit**

```bash
git add packages/core/types/forge-health.ts packages/core/api/schemas.ts packages/core/api/schemas.test.ts
git commit -m "feat(forge): health score fields in type + zod schema"
```

---

### Task 1.2: 健康分 badge

**Files:**
- Modify: `packages/views/forge-health/forge-health-page.tsx`

- [ ] **Step 1: 加 statusDot 映射**

在 `forge-health-page.tsx` 的 `function rate(...)` 定义**之后**加:
```ts
const statusDot: Record<string, string> = {
  green: "bg-green-500",
  yellow: "bg-amber-500",
  red: "bg-red-500",
};
```

- [ ] **Step 2: header 加 badge**

找到 render 顶部的 header 块:
```tsx
      <div>
        <h1 className="text-lg font-semibold">Harness health</h1>
        <p className="text-xs text-muted-foreground">
          What the Forge Harness is doing across this workspace (last 30 days for activity).
        </p>
      </div>
```
整块替换为:
```tsx
      <div className="flex items-start justify-between gap-4">
        <div>
          <h1 className="text-lg font-semibold">Harness health</h1>
          <p className="text-xs text-muted-foreground">
            What the Forge Harness is doing across this workspace (last 30 days for activity).
          </p>
        </div>
        <div className="flex items-center gap-2 rounded-md border px-3 py-2">
          <span className={`size-2.5 shrink-0 rounded-full ${statusDot[h.status] ?? "bg-muted"}`} />
          <span className="text-2xl font-semibold tabular-nums">{h.score}</span>
          <span className="text-xs text-muted-foreground">
            /100{h.no_activity ? " · configured, no activity yet" : ""}
          </span>
        </div>
      </div>
```

- [ ] **Step 3: typecheck 三包**

Run: `cd D:\shulex_work\forge; corepack pnpm --filter "@multica/core" --filter "@multica/views" --filter "@multica/web" typecheck 2>&1 | Select-Object -Last 8`
Expected: 三包 Done。

- [ ] **Step 4: Commit**

```bash
git add packages/views/forge-health/forge-health-page.tsx
git commit -m "feat(forge): Harness health score badge"
```

---

## Phase 1 完成检查
- [ ] 类型 + zod + EMPTY 加 score/status/no_activity
- [ ] 顶部健康分 badge(数字 + 状态点 + no_activity 注记),三包 typecheck 绿
