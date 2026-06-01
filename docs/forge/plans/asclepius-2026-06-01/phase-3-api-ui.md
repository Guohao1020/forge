## Phase 3 — API + UI（auto_fix 开关透传）

**Goal:** handler 的 `ForgeEntropyScanBody`/`Response` + Create/Update 透传 `auto_fix`;TS 类型 + view 一个 checkbox。
**Depends-on:** Phase 0（params 已含 `AutoFix`）　**Unblocks:** Phase 4
**Completion gate:** `go build ./...` 通过;三包 typecheck 绿。

> 改的是 F4 既有文件(`forge_entropy.go` 后端、`forge-entropy.ts`/`forge-entropy-page.tsx` 前端)。

---

### Task 3.1: 后端透传 auto_fix

**Files:**
- Modify: `server/internal/handler/forge_entropy.go`

- [ ] **Step 1: body + response 加字段**

`ForgeEntropyScanBody` struct 末尾(`Enabled` 之后)加:
```go
	AutoFix bool `json:"auto_fix"`
```
`ForgeEntropyScanResponse` struct 末尾(`Enabled` 之后、`AutoFixID`/`AutopilotID` 之前任意位置)加:
```go
	AutoFix bool `json:"auto_fix"`
```

- [ ] **Step 2: mapper + Create + Update 透传**

`entropyScanToResponse` 内,给 `out` 字面量加一项(与 `Enabled: s.Enabled` 同处):
```go
		Enabled: s.Enabled, AutoFix: s.AutoFix,
```
（即把原来的 `CronExpression: s.CronExpression, Timezone: s.Timezone, Enabled: s.Enabled,` 那行末补 `AutoFix: s.AutoFix,`。）

`CreateForgeEntropyScan` 内 `db.CreateEntropyScanParams{...}` 字面量,在 `Enabled: req.Enabled,` 旁加:
```go
		Enabled:          req.Enabled,
		AutoFix:          req.AutoFix,
```

`UpdateForgeEntropyScan` 内 `db.UpdateEntropyScanParams{...}` 字面量,在 `Enabled: req.Enabled,` 旁加:
```go
		CronExpression: req.CronExpression, Timezone: tz, Enabled: req.Enabled, AutoFix: req.AutoFix,
```

- [ ] **Step 3: 编译**

Run: `wsl -d Ubuntu -- bash -lc "cd /mnt/d/shulex_work/forge/server && go build ./... 2>&1 | tail -8 && echo OK"`
Expected: 打印 `OK`。

- [ ] **Step 4: Commit**

```bash
git add server/internal/handler/forge_entropy.go
git commit -m "feat(forge): pass auto_fix through entropy-scans API"
```

---

### Task 3.2: 前端 auto_fix（类型 + checkbox）

**Files:**
- Modify: `packages/core/types/forge-entropy.ts`
- Modify: `packages/views/forge-entropy/forge-entropy-page.tsx`

- [ ] **Step 1: 类型加 auto_fix**

`packages/core/types/forge-entropy.ts`:`ForgeEntropyScan` 和 `ForgeEntropyScanInput` **两个** interface 都在 `enabled: boolean;` 之后加:
```ts
  auto_fix: boolean;
```
（`ForgeEntropyScan` 的 `auto_fix` 放在 `enabled` 与 `autopilot_id?` 之间。）

- [ ] **Step 2: view —— EMPTY + startEdit + checkbox**

`packages/views/forge-entropy/forge-entropy-page.tsx`:

`EMPTY` 常量加(在 `enabled: true,` 之后):
```ts
  enabled: true,
  auto_fix: false,
```

`startEdit` 的 `setForm({...})` 加(在 `enabled: s.enabled,` 之后):
```ts
      enabled: s.enabled,
      auto_fix: s.auto_fix,
```

在 "Include verification checks (F2)" 那个 `<label>` 之后、`custom_focus` 的 `<div>` 之前,插入一个 checkbox:
```tsx
            <label className="flex items-center gap-2 text-sm">
              <input
                type="checkbox"
                checked={form.auto_fix}
                onChange={(e) =>
                  setForm({ ...form, auto_fix: e.target.checked })
                }
              />
              Let the agent fix what it safely can and open a PR
            </label>
```

- [ ] **Step 3: typecheck**

Run (PowerShell)：`cd D:\shulex_work\forge; corepack pnpm --filter "@multica/core" --filter "@multica/views" --filter "@multica/web" typecheck 2>&1 | Select-Object -Last 8`
Expected: 三包 Done。

- [ ] **Step 4: Commit**

```bash
git add packages/core/types/forge-entropy.ts packages/views/forge-entropy/forge-entropy-page.tsx
git commit -m "feat(forge): auto_fix toggle in entropy-scan UI"
```

---

## Phase 3 完成检查
- [ ] 后端 `auto_fix` 透传(body/response/Create/Update),`go build` 绿
- [ ] 前端类型 + checkbox,三包 typecheck 绿
