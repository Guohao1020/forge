## Phase 4 — 验收 + 文档

**Goal:** Go build/vet + 三包 typecheck;源码构建栈实测 health/trends/drill 端点(绕凭证);index + 记忆。
**Depends-on:** Phase 1、2、3　**Unblocks:** —(F5 收官)
**Completion gate:** build/vet + typecheck 绿;源码构建 `GET /api/forge/health` 返回真实非零数据;DoD 勾完。

> **F5 绕凭证最彻底**:纯读聚合,无需任何 agent 跑。源码构建栈在 F4/F4b 验证已留下真实数据
> (entropy scan、issue、forge_fix_pr 行、完成 task)。

---

### Task 4.1: build/vet + typecheck

- [ ] **Step 1: Go build + vet**

Run: `wsl -d Ubuntu -- bash -lc "cd /mnt/d/shulex_work/forge/server && go build ./... 2>&1 | tail -5 && go vet ./internal/handler/ 2>&1 | tail -5 && echo OK"`
Expected: 打印 `OK`。

- [ ] **Step 2: 前端 typecheck + 既有单测**

Run: `cd D:\shulex_work\forge; corepack pnpm --filter "@multica/core" --filter "@multica/views" --filter "@multica/web" typecheck 2>&1 | Select-Object -Last 8`
Expected: 三包 Done。
Run: `cd D:\shulex_work\forge; corepack pnpm --filter "@multica/core" test 2>&1 | Select-Object -Last 6`
Expected: core 单测全过(zod schema 新增不破坏既有)。

---

### Task 4.2: 源码构建 + health 端点绕凭证实测

- [ ] **Step 1: 源码构建起栈**

Run (PowerShell)：
```powershell
cd D:\shulex_work\forge
docker compose --env-file D:\shulex_work\multica\.env -f docker-compose.selfhost.yml -f docker-compose.selfhost.build.yml -p forge-build up -d --build backend postgres
```
Expected: health 200(`http://localhost:8081/health`)。

- [ ] **Step 2: GET /api/forge/health + trends + drill,断言真实数据**

Run (PowerShell；登录沿用 F4/F4b:`/auth/send-code` → `/auth/verify-code` code=888888；`$ws` 复查)：
```powershell
$base="http://localhost:8081"; $email="harvey@forge.local"; $ws="1379e8af-e0bc-4a01-ae39-0b3beefb5f86"
Invoke-RestMethod "$base/auth/send-code" -Method Post -ContentType application/json -Body (@{email=$email}|ConvertTo-Json) | Out-Null
$login = Invoke-RestMethod "$base/auth/verify-code" -Method Post -ContentType application/json -Body (@{email=$email; code="888888"}|ConvertTo-Json)
$H = @{ Authorization="Bearer $($login.token)"; "X-Workspace-ID"=$ws }

$health = Invoke-RestMethod "$base/api/forge/health?days=365" -Headers $H
Write-Output "HEALTH: scans=$($health.scans) checks=$($health.checks) standards=$($health.standards_total) open_findings=$($health.open_findings)"
Write-Output "  gate: pass=$($health.gate.passed) fail=$($health.gate.failed)  review: total=$($health.review.total)  fix_prs: opened=$($health.fix_prs.opened) merged=$($health.fix_prs.merged) matched=$($health.fix_prs.matched)"
# 断言:scans>=1 且 fix_prs.opened>=1(F4/F4b 留下的真实数据)
$ok = ($health.scans -ge 1) -and ($health.fix_prs.opened -ge 1)
Write-Output "SNAPSHOT_REAL=$ok"

$trends = Invoke-RestMethod "$base/api/forge/health/trends?days=365" -Headers $H
Write-Output "TRENDS: findings_days=$($trends.findings.Count) gate_days=$($trends.gate.Count) fixpr_days=$($trends.fix_prs.Count)"

$fixprs = Invoke-RestMethod "$base/api/forge/health/fix-prs?days=365" -Headers $H
Write-Output "DRILL fix-prs count=$($fixprs.Count) first=$($fixprs[0].pr_url)"
$gf = Invoke-RestMethod "$base/api/forge/health/gate-failures?days=365" -Headers $H
$fd = Invoke-RestMethod "$base/api/forge/health/findings" -Headers $H
Write-Output "DRILL gate-failures=$($gf.Count) findings=$($fd.Count)"
```
Expected: `SNAPSHOT_REAL=True`(scans≥1 + fix_prs.opened≥1,来自 F4/F4b 留下的真实数据);trends 返回 date-bucketed 数组;
drill fix-prs 至少 1 条(含 F4b 建的 `https://github.com/o/r/pull/999`);gate-failures/findings 端点返回 list。
**证明 F5 健康面板端到端纯读聚合工作,无需任何 agent 跑。**

> 若某计数为 0(如 standards/checks 该 workspace 未配),不算失败 —— 关键断言是 scans≥1 + fix_prs.opened≥1
> + 所有端点 200 返回结构正确。merged/matched 多半为 0(无 GitHub App webhook),符合优雅降级预期。

---

### Task 4.3: 更新 index + 记忆

- [ ] **Step 1: index 标记完成**

`docs/forge/plans/argus-2026-06-01/index.md` Phase 表 5 行改 ✅;DoD 勾完。

- [ ] **Step 2: 记忆**

`~/.claude/projects/D--shulex-work-forge/memory/forge-on-multica-pivot.md`:加 F5 段——可观测闭环
（compute-on-read 聚合 F1–F4b 既有表:快照卡 + recharts 趋势 + 混合钻取;`forge_health.sql` +
`forge_health.go` 5 端点 + `forge-health` view + health response zod;零迁移）已落地。绕凭证实测:
`GET /api/forge/health` 返回真实非零(scans≥1、fix_prs.opened≥1)。标注满负荷数字待 live 活动。
**并更新 `MEMORY.md` 索引行:F0–F5 全闭环完成**。

- [ ] **Step 3: 提交**

```bash
git add docs/forge/plans/argus-2026-06-01/index.md
git commit -m "docs(plan): mark argus F5 complete (health panel verified creds-free)"
```

---

## Phase 4 完成检查
- [ ] Go build/vet + 三包 typecheck + core 单测绿
- [ ] 源码构建 `GET /api/forge/health` 返回真实非零数据(scans≥1 + fix_prs.opened≥1);trends/drill 端点 200
- [ ] index + 记忆更新(F0–F5 全闭环)
