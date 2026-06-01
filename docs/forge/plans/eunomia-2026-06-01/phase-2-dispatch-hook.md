## Phase 2 — 派发钩子（dispatchCreateIssue 注入合成 brief）

**Goal:** 在 `service.AutopilotService.dispatchCreateIssue` 一处钩子：反查
`GetEntropyScanByAutopilot`，命中熵 autopilot 则把 issue.description 覆写为
`forgeentropy.ResolveBrief` 合成的 brief。
**Depends-on:** Phase 1　**Unblocks:** Phase 4
**Completion gate:** `go build ./...` + `go vet ./internal/service/` 通过；普通 autopilot 行为不变（反查无行 → 不改 description）。

> 注入点：`server/internal/service/autopilot.go:159`，`description := s.buildIssueDescription(...)` 之后、
> `CreateIssueWithOrigin`（line 171）之前。`service → forgeentropy → {forge/standards, forge/checks, db}` 全 service-free，无环。

---

### Task 2.1: 加派发钩子

**Files:**
- Modify: `server/internal/service/autopilot.go`（import + `dispatchCreateIssue` 内一处）

- [ ] **Step 1: 加 import**

在 `server/internal/service/autopilot.go` 的 import 块加：
```go
"github.com/multica-ai/multica/server/internal/forgeentropy"
```
（`pgtype` 已 import，无需新增。）

- [ ] **Step 2: 插入钩子**

在 `dispatchCreateIssue` 内，紧接
```go
	description := s.buildIssueDescription(ap, *run, triggerTimezone)
```
之后插入：
```go
	// Forge F4: if this autopilot backs an entropy scan, override the issue
	// description with the composed scan brief (F1 standards + F2 checks +
	// custom focus + open-findings dedup list). GetEntropyScanByAutopilot
	// returns an error (no rows) for ordinary autopilots, leaving the normal
	// description untouched. ResolveBrief is best-effort and never errors.
	if scan, err := s.Queries.GetEntropyScanByAutopilot(ctx, ap.ID); err == nil {
		description = pgtype.Text{String: forgeentropy.ResolveBrief(ctx, s.Queries, scan), Valid: true}
	}
```

> `s.Queries`（`*db.Queries`）满足 `forgeentropy.Querier`（含 standards/checks/findings 全部方法）。
> brief 读取走非事务 `s.Queries`（只读参考数据，与 issue 写事务独立）；任一读失败在 `ResolveBrief` 内被吞、退化为缺段。

- [ ] **Step 3: 编译 + vet**

Run: `wsl -d Ubuntu -- bash -lc "cd /mnt/d/shulex_work/forge/server && go build ./... 2>&1 | tail -8 && go vet ./internal/service/ 2>&1 | tail -5 && echo OK"`
Expected: build 通过、vet 干净、打印 `OK`。无 import cycle 报错（若报 cycle，说明 Phase 1 的 `checks` 子包化未完成——回查 Task 1.1）。

- [ ] **Step 4: Commit**

```bash
git add server/internal/service/autopilot.go
git commit -m "feat(forge): compose entropy brief into issue at autopilot dispatch"
```

---

## Phase 2 完成检查
- [ ] `dispatchCreateIssue` 一处钩子注入合成 brief，无 import cycle
- [ ] `go build ./...` + `go vet ./internal/service/` 通过
- [ ] 活体行为（scanner agent 读 brief → 扫描 → 建发现 issue）待凭证；编排链由 Phase 4 绕凭证集成验证
