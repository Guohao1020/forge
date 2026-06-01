## Phase 4 — 验收 + 文档

**Goal:** 跑测试 + typecheck；源码构建确认 server 带 F3 启动 + review-config CRUD；记录活体流程
的凭证依赖；更新 index + 记忆。

**Depends-on:** Phase 2、Phase 3　**Unblocks:** F4
**Completion gate:** Go 测试 + 三包 typecheck 绿；源码构建启动 + review-config 往返；F3 DoD 勾完
（活体触发+评审标注凭证依赖）。

> **凭证现实**：F3 比 F2 更依赖凭证。F2 门禁逻辑 `runChecks` 可纯单测；F3 的**完整触发**需要一个
> coding task 真的 completed（需 agent 成功=凭证），**活体评审**需 reviewer agent 真跑（凭证）。
> 可绕凭证验证的是：forge 决策逻辑单测 + review-config CRUD + `MaybeEnqueueReview` 编译。

---

### Task 4.1: 测试 + typecheck

- [ ] **Step 1: Go 测试 + build + vet**

Run:
```bash
wsl -d Ubuntu -- bash -lc "cd /mnt/d/shulex_work/forge/server && go test ./internal/forge/... 2>&1 | tail -8 && go build ./... && go vet ./internal/forge/... ./internal/service/ 2>&1 | tail -5 && echo OK"
```
Expected: forge review/checks/standards 单测全绿；build/vet 干净。

- [ ] **Step 2: 前端 typecheck**

Run (Windows): `cd D:\shulex_work\forge; corepack pnpm --filter "@multica/core" --filter "@multica/views" --filter "@multica/web" typecheck 2>&1 | Select-Object -Last 8`
Expected: 三包 Done。

---

### Task 4.2: 源码构建 + review-config CRUD（绕凭证）

- [ ] **Step 1: 源码构建起栈**

Run:
```powershell
cd D:\shulex_work\forge
docker compose --env-file D:\shulex_work\multica\.env -f docker-compose.selfhost.yml -f docker-compose.selfhost.build.yml -p forge-build up -d --build backend postgres
```
Expected: 重建启动，health 200；`forge_review_config` 表存在
（`docker exec forge-build-postgres-1 psql -U multica -d multica -c "\dt forge_review_config"`）。

- [ ] **Step 2: 经 API 配 reviewer + 确认往返**

Run（PowerShell，复用 F1/F2 的 API 登录拿 $H；reviewer agent id 用已有 agent，如 F0 建的 Claude agent）:
```powershell
# 先 GET 现有 agent 列表拿一个 agent id 作 reviewer
$agents = Invoke-RestMethod "$base/api/agents" -Headers $H
$reviewer = $agents[0].id
$body = @{ reviewer_agent_id=$reviewer; enabled=$true } | ConvertTo-Json
Invoke-RestMethod "$base/api/forge/review-config" -Method Put -Headers $H -ContentType application/json -Body $body
Invoke-RestMethod "$base/api/forge/review-config" -Headers $H   # 断言返回该 reviewer + enabled
```
Expected: PUT 成功；GET 返回配的 reviewer_agent_id + enabled。**证明 review-config 在源码构建
服务器工作。**

---

### Task 4.3: 活体评审流程（凭证就绪后）

> **当前阻塞**：需一个能跑到 completed 的 provider（F0 遗留）+ 一个能跑的 reviewer agent。

- [ ] **Step（待凭证）**：配 reviewer agent → 建 issue 派给 coder agent → coder 完成（completed，
  有 work_dir）→ **触发**：CompleteTask 钩子 `MaybeEnqueueReview` 建 review 任务 → reviewer 的
  daemon claim（复用 coder workdir via PriorWorkDir + F1 注入规范）→ reviewer 跑 `git diff` 评审
  → 发评论。**断言**：issue 上出现 review 任务 + reviewer 的评审评论。
- 防循环反向：reviewer 的 review 任务完成 → 不再触发新 review（context marker）。

> 编排（coder 完成→建 review 任务）的逻辑由 forge 单测 + 编译覆盖；活体串联待凭证。

---

### Task 4.4: 更新 index + 记忆

- [ ] **Step 1: index 标记完成**

`docs/forge/plans/momus-2026-06-01/index.md` Phase 表改 ✅；DoD 勾完（活体标注凭证依赖）。

- [ ] **Step 2: 记忆**

`~/.claude/projects/D--shulex-work-forge/memory/forge-on-multica-pivot.md`：F3 AI review
（完成即评审、专用 reviewer、复用 work_dir、复用 F1 注入）已落地——`forge_review_config` 表、
`forge/review.go` + `service/forge_review.go`、CompleteTask 钩子 + claim PriorWorkDir 钩子、
`/api/forge/review-config` + UI。标注活体评审待凭证。

- [ ] **Step 3: 提交**

```bash
git add docs/forge/plans/momus-2026-06-01/index.md
git commit -m "docs(plan): mark momus F3 complete (orchestration verified; live review pending creds)"
```

---

## Phase 4 完成检查
- [ ] Go 测试（forge review）+ 三包 typecheck 绿
- [ ] 源码构建起栈，`forge_review_config` 表存在，review-config CRUD 往返
- [ ] 活体评审流程记录，标注凭证依赖
- [ ] index + 记忆更新
