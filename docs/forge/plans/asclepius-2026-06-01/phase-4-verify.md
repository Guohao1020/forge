## Phase 4 — 验收 + 文档

**Goal:** Go 测试 + 三包 typecheck;源码构建确认 auto_fix 往返 + PR 桥(绕凭证);index + 记忆。
**Depends-on:** Phase 1、2、3　**Unblocks:** F5
**Completion gate:** Go 测试 + 三包 typecheck 绿;源码构建起栈;auto_fix scan 往返;forge_fix_pr 录入(+幂等)验证;DoD 勾完。

> **凭证现实**:活体修复需 ① provider 凭证 ② execenv GitHub push auth。PR 桥本身(解析 pr_url → 录入 + 评论)
> 是纯后端,绕凭证可验:把一个带 `pr_url` 的完成请求喂给 issue-bound 任务,或直接验 SQL 幂等。

---

### Task 4.1: 测试 + typecheck

- [ ] **Step 1: Go 测试 + build + vet**

Run:
```bash
wsl -d Ubuntu -- bash -lc "cd /mnt/d/shulex_work/forge/server && go test ./internal/forgeentropy/... ./internal/service/ -run 'TestComposeBrief|TestParseFixPRURL' 2>&1 | tail -10 && go build ./... && go vet ./internal/service/ ./internal/handler/ 2>&1 | tail -5 && echo OK"
```
Expected: `ComposeBrief`(4 测含 auto_fix on/off)+ `TestParseFixPRURL` 绿;build/vet 干净;打印 `OK`。

- [ ] **Step 2: 前端 typecheck**

Run (Windows): `cd D:\shulex_work\forge; corepack pnpm --filter "@multica/core" --filter "@multica/views" --filter "@multica/web" typecheck 2>&1 | Select-Object -Last 8`
Expected: 三包 Done。

---

### Task 4.2: 源码构建 + auto_fix 往返 + PR 桥（绕凭证）

- [ ] **Step 1: 源码构建起栈**

Run (PowerShell)：
```powershell
cd D:\shulex_work\forge
docker compose --env-file D:\shulex_work\multica\.env -f docker-compose.selfhost.yml -f docker-compose.selfhost.build.yml -p forge-build up -d --build backend postgres
```
Expected: health 200(`http://localhost:8081/health`);`forge_fix_pr` 表存在 + `forge_entropy_scan.auto_fix` 列存在
（`'\d forge_fix_pr' | docker exec -i forge-build-postgres-1 psql -U multica -d multica`）。

- [ ] **Step 2: auto_fix scan 往返**

Run (PowerShell；登录沿用 F4:`/auth/send-code` → `/auth/verify-code` code=888888 → Bearer + `X-Workspace-ID`；
`$ws`/`$scanner` 实测时复查 DB)：
```powershell
$base="http://localhost:8081"; $email="harvey@forge.local"; $ws="1379e8af-e0bc-4a01-ae39-0b3beefb5f86"; $scanner="180f52ce-0da0-40a0-93b0-56eb5b4a5e0f"
Invoke-RestMethod "$base/auth/send-code" -Method Post -ContentType application/json -Body (@{email=$email}|ConvertTo-Json) | Out-Null
$login = Invoke-RestMethod "$base/auth/verify-code" -Method Post -ContentType application/json -Body (@{email=$email; code="888888"}|ConvertTo-Json)
$H = @{ Authorization="Bearer $($login.token)"; "X-Workspace-ID"=$ws }
$body = @{ name="autofix"; scanner_agent_id=$scanner; custom_focus="x"; include_standards=$false; include_checks=$false; cron_expression="0 9 * * 1"; timezone="UTC"; enabled=$true; auto_fix=$true } | ConvertTo-Json
$scan = Invoke-RestMethod "$base/api/forge/entropy-scans" -Method Post -Headers $H -ContentType application/json -Body $body
Write-Output "AUTO_FIX_ROUNDTRIP=$($scan.auto_fix)"   # 期望 True
```
Expected: `AUTO_FIX_ROUNDTRIP=True`（auto_fix 经 API 持久化往返）。**留着 `$scan`/`$H` 给 Step 3。**

- [ ] **Step 3: PR 桥 —— 喂带 pr_url 的完成请求（首选路径）**

手动触发该 scan 的 backing autopilot 建出 issue + scanner task,然后以 PAT 调 daemon 完成端点喂 `pr_url`：
```powershell
# 触发 → 建 issue + 入队 scanner task
$run = Invoke-RestMethod "$base/api/autopilots/$($scan.autopilot_id)/trigger" -Method Post -Headers $H
$issueId = $run.issue_id
# 取该 issue 的 task_id
$taskId = ('SELECT id FROM agent_task_queue WHERE issue_id=''' + $issueId + ''' ORDER BY created_at DESC LIMIT 1;' | docker exec -i forge-build-postgres-1 psql -U multica -d multica -t -A).Trim()
Write-Output "issue=$issueId task=$taskId"
# 建一个 PAT 作 daemon 端点鉴权(若 /api/tokens 形态不同,按返回调整;PAT 前缀 mul_)
$pat = Invoke-RestMethod "$base/api/tokens" -Method Post -Headers $H -ContentType application/json -Body (@{name="f4b-verify"}|ConvertTo-Json)
$DH = @{ Authorization="Bearer $($pat.token)" }
# 喂完成请求(带假 pr_url + work_dir),触发 MaybeRecordFixPR
$cbody = @{ pr_url="https://github.com/o/r/pull/999"; work_dir="/tmp/wd"; output="fixed" } | ConvertTo-Json
Invoke-RestMethod "$base/api/daemon/tasks/$taskId/complete" -Method Post -Headers $DH -ContentType application/json -Body $cbody | Out-Null
# 断言 forge_fix_pr 行 + 系统评论
Write-Output "=== forge_fix_pr (expect 1 row, pr_url=.../999) ==="
('SELECT pr_url FROM forge_fix_pr WHERE task_id=''' + $taskId + ''';') | docker exec -i forge-build-postgres-1 psql -U multica -d multica
Write-Output "=== system comment (expect Fix PR opened) ==="
('SELECT type, left(content,40) FROM comment WHERE issue_id=''' + $issueId + ''' AND type=''system'' ORDER BY created_at DESC LIMIT 3;') | docker exec -i forge-build-postgres-1 psql -U multica -d multica
# 幂等:再喂一次 → 仍一行、评论不增
Invoke-RestMethod "$base/api/daemon/tasks/$taskId/complete" -Method Post -Headers $DH -ContentType application/json -Body $cbody | Out-Null
Write-Output "=== forge_fix_pr count after 2nd complete (expect 1) ==="
('SELECT count(*) FROM forge_fix_pr WHERE task_id=''' + $taskId + ''';') | docker exec -i forge-build-postgres-1 psql -U multica -d multica
```
Expected: `forge_fix_pr` 一行 `pr_url=.../999`;issue 上一条 `system` 评论含 `Fix PR opened`;二次完成后仍 1 行(幂等)。
**证明 PR 桥端到端工作(全后端,无需 agent 真开 PR)。**

> **fallback(若 PAT 不被 daemon 端点接受 / task 状态不可完成)**:直接 SQL 验幂等 ——
> `INSERT INTO forge_fix_pr(workspace_id,issue_id,task_id,pr_url) VALUES (<ws>,<issue>,<task>,'u') ON CONFLICT (task_id,pr_url) DO NOTHING;` 跑两次 → `SELECT count(*)` 仍 1。
> 证明唯一索引 + ON CONFLICT 幂等;PR 桥其余路径由 `parseFixPRURL` 单测 + 编译 + 代码评审覆盖。
> 这条 fallback 也是诚实记录:完整 CompleteTask 路径需一个可完成的任务(daemon/PAT 鉴权),
> 与 F2/F3 活体同属凭证/守卫相邻区。

---

### Task 4.3: 活体修复流程（双重凭证就绪后）

> **当前阻塞**:scanner agent 真扫 + 真修 + `gh` 开 PR 需 ① provider 凭证 ② execenv GitHub push auth。

- [ ] **Step（待凭证）**:配 auto_fix scanner agent(execenv 有 repo + GitHub push)→ cron/手动触发 →
  scanner 读含修复段的 brief → 能修的改 + `gh pr create`(body `Closes MUL-N`)+ 修不了建 issue →
  任务完成带 `pr_url` → **PR 桥录入 + 评论** → **F2 门禁 + F3 评审自动跑在 fix diff 上** → 人工合并 PR。
  **断言**:forge_fix_pr 录入;F2 通过/blocked;F3 reviewer 评审评论;PR 上 `Closes` 关键字(若装 GitHub App 则 webhook 合并后自动关 issue)。

---

### Task 4.4: 更新 index + 记忆

- [ ] **Step 1: index 标记完成**

`docs/forge/plans/asclepius-2026-06-01/index.md` Phase 表 5 行改 ✅;DoD 勾完(活体修复标注双重凭证依赖)。

- [ ] **Step 2: 记忆**

`~/.claude/projects/D--shulex-work-forge/memory/forge-on-multica-pivot.md`:加 F4b 段——自愈闭环
（per-scan auto_fix:scanner 顺手修+开 PR;PR 桥 `forge_fix_pr` + CompleteTask 钩子填 pr_url dead-end;
F2/F3 复用零新增;不碰 GitHub-App 耦合的 github_pull_request;建议性人工合并）已落地。绕凭证验:
auto_fix 往返 + forge_fix_pr 录入/幂等;活体修复待双重凭证。并更新 `MEMORY.md` 索引行(F0–F4b done)。

- [ ] **Step 3: 提交**

```bash
git add docs/forge/plans/asclepius-2026-06-01/index.md
git commit -m "docs(plan): mark asclepius F4b complete (PR bridge verified; live fix pending creds)"
```

---

## Phase 4 完成检查
- [ ] Go 测试（ComposeBrief auto_fix + parseFixPRURL）+ 三包 typecheck 绿
- [ ] 源码构建起栈,`forge_fix_pr` 表 + `auto_fix` 列存在
- [ ] auto_fix scan 往返;PR 桥录入 + 系统评论 + 幂等(或 SQL fallback)
- [ ] 活体修复流程记录,标注双重凭证依赖;index + 记忆更新
