## Phase 2 — 验收 + 文档

**Goal:** Go 测试 + build/vet + 三包 typecheck;源码构建栈 `GET /api/forge/health` 的 score/status 与实算一致;index + 记忆。
**Depends-on:** Phase 0、1　**Unblocks:** —
**Completion gate:** 测试/typecheck 绿;`GET /api/forge/health` 返回 score≈80 green(当前真实数据);DoD 勾完。

> **零凭证**:`Score` 纯单测 + 健康端点纯读聚合,无需 agent 跑。

---

### Task 2.1: 测试 + typecheck

- [ ] **Step 1: Go 测试 + build + vet**

Run: `wsl -d Ubuntu -- bash -lc "cd /mnt/d/shulex_work/forge/server && go test ./internal/forgehealth/... 2>&1 | tail -6 && go build ./... && go vet ./internal/handler/ ./internal/forgehealth/ 2>&1 | tail -5 && echo OK"`
Expected: `Score` 测试 PASS;build/vet 干净;打印 `OK`。

- [ ] **Step 2: 前端 typecheck + core 单测**

Run: `cd D:\shulex_work\forge; corepack pnpm --filter "@multica/core" --filter "@multica/views" --filter "@multica/web" typecheck 2>&1 | Select-Object -Last 8`
Expected: 三包 Done。
Run: `cd D:\shulex_work\forge; corepack pnpm --filter "@multica/core" test 2>&1 | Select-Object -Last 5`
Expected: core 单测全过(zod 加字段不破坏既有;若有 F5 schema 测断言精确对象,补 score/status/no_activity)。

---

### Task 2.2: 源码构建 + 端点绕凭证实测

- [ ] **Step 1: 源码构建起栈**

Run (PowerShell):
```powershell
cd D:\shulex_work\forge
docker compose --env-file D:\shulex_work\multica\.env -f docker-compose.selfhost.yml -f docker-compose.selfhost.build.yml -p forge-build up -d --build backend
```
Expected: health 200(`http://localhost:8081/health`)。

- [ ] **Step 2: GET /api/forge/health 断言 score/status**

Run (PowerShell；登录沿用 F5:`/auth/send-code` → `/auth/verify-code` code=888888;`$ws` 复查):
```powershell
$base="http://localhost:8081"; $email="harvey@forge.local"; $ws="1379e8af-e0bc-4a01-ae39-0b3beefb5f86"
Invoke-RestMethod "$base/auth/send-code" -Method Post -ContentType application/json -Body (@{email=$email}|ConvertTo-Json) | Out-Null
$login = Invoke-RestMethod "$base/auth/verify-code" -Method Post -ContentType application/json -Body (@{email=$email; code="888888"}|ConvertTo-Json)
$H = @{ Authorization="Bearer $($login.token)"; "X-Workspace-ID"=$ws }
$h = Invoke-RestMethod "$base/api/forge/health?days=365" -Headers $H
Write-Output "SCORE=$($h.score) STATUS=$($h.status) NO_ACTIVITY=$($h.no_activity)"
Write-Output "  inputs: std=$($h.standards_total) chk=$($h.checks) rev=$($h.review_configs) scan=$($h.scans) gate=$($h.gate.passed)/$($h.gate.failed) review=$($h.review.completed)/$($h.review.total) findings=$($h.open_findings) fixprs=$($h.fix_prs.opened)"
# 断言:有 score(0-100)、status 是 green/yellow/red 之一、且与配置一致(当前全配 → score 不低、非 red)
$valid = ($h.score -ge 0 -and $h.score -le 100) -and ($h.status -in @("green","yellow","red"))
Write-Output "SCORE_VALID=$valid"
```
Expected: `SCORE`/`STATUS`/`NO_ACTIVITY` 返回;当前真实数据(全配 + gate.pass=1 + review.completed=0 + findings=0/fixprs=1)
手算 coverage=1.0、gatePass=1.0、reviewDone=0.0、entropyControl=1.0、qual=0.667 → **score=80 green**;`SCORE_VALID=True`。
**证明健康分端到端纯读聚合工作,无需 agent 跑。**

> 实际 score 取决于 DB 当前计数(评审任务是否完成等可能微调),关键断言是 score∈[0,100]、status 合法、与配置一致(全配不会是 red)。

---

### Task 2.3: 更新 index + 记忆

- [ ] **Step 1: index 标记完成**

`docs/forge/plans/hygieia-2026-06-01/index.md` Phase 表 3 行改 ✅;DoD 勾完。

- [ ] **Step 2: 记忆**

`~/.claude/projects/D--shulex-work-forge/memory/forge-on-multica-pivot.md`:在 F5 段后或「后续」处加一句——
Harness 健康总分(hygieia)已落地:`forgehealth.Score` 纯函数(覆盖感知混合,TDD)+ `GetForgeHealth` 加
`score`/`status`/`no_activity` + 前端顶部 badge;零迁移零凭证;源码构建栈实测 score≈80 green。

- [ ] **Step 3: 提交**

```bash
git add docs/forge/plans/hygieia-2026-06-01/index.md
git commit -m "docs(plan): mark hygieia health-score complete (verified creds-free)"
```

---

## Phase 2 完成检查
- [ ] `Score` 单测 + 三包 typecheck + core 单测绿
- [ ] 源码构建 `GET /api/forge/health` 返回合法 score/status(当前真实数据 ≈80 green)
- [ ] index + 记忆更新
