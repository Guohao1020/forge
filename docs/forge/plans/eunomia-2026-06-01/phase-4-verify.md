## Phase 4 — 验收 + 文档

**Goal:** Go 测试 + 三包 typecheck；源码构建确认 server 带 F4 启动；**绕凭证**端到端验证编排链
（建 scan → backing autopilot + schedule 触发 → 手动触发 → 断言 `issue.description` 含合成 brief）；
更新 index + 记忆。
**Depends-on:** Phase 2、Phase 3　**Unblocks:** F4b
**Completion gate:** Go 测试 + 三包 typecheck 绿；源码构建启动；scan→autopilot→派发→brief 落 issue 往返；F4 DoD 勾完。

> **凭证现实**：F4 的编排链**全在服务端**（建 scan、建 autopilot、派发建 issue、合成 brief），
> 手动触发即可绕凭证端到端验证——比 F2/F3 更强。**活体扫描**（scanner agent 真扫仓 + CLI 建发现 issue）
> 需 provider 凭证，延后。

---

### Task 4.1: 测试 + typecheck

- [ ] **Step 1: Go 测试 + build + vet**

Run:
```bash
wsl -d Ubuntu -- bash -lc "cd /mnt/d/shulex_work/forge/server && go test ./internal/forge/... ./internal/forgeentropy/... 2>&1 | tail -10 && go build ./... && go vet ./internal/forgeentropy/... ./internal/service/ ./internal/handler/ 2>&1 | tail -5 && echo OK"
```
Expected: `forge/standards`、`forge/checks`、`forgeentropy` 测试全绿；build/vet 干净；打印 `OK`。

- [ ] **Step 2: 前端 typecheck**

Run (Windows): `cd D:\shulex_work\forge; corepack pnpm --filter "@multica/core" --filter "@multica/views" --filter "@multica/web" typecheck 2>&1 | Select-Object -Last 8`
Expected: 三包 Done。

---

### Task 4.2: 源码构建 + 编排链绕凭证验证

- [ ] **Step 1: 源码构建起栈**

Run (PowerShell)：
```powershell
cd D:\shulex_work\forge
docker compose --env-file D:\shulex_work\multica\.env -f docker-compose.selfhost.yml -f docker-compose.selfhost.build.yml -p forge-build up -d --build backend postgres
```
Expected: 重建启动，`http://localhost:8081/health` 200；`forge_entropy_scan` 表存在
（`'\dt forge_entropy_scan' | docker exec -i forge-build-postgres-1 psql -U multica -d multica`）。

- [ ] **Step 2: 登录 + 建 scan + 断言 backing autopilot**

Run (PowerShell；登录沿用 F3 流程：`/auth/send-code` → `/auth/verify-code` code=888888 → Bearer + `X-Workspace-ID`)：
```powershell
$base = "http://localhost:8081"
$email = "harvey@forge.local"
$ws = "1379e8af-e0bc-4a01-ae39-0b3beefb5f86"   # 实测时复查：DB 里取一个有 agent 的 workspace
Invoke-RestMethod "$base/auth/send-code" -Method Post -ContentType application/json -Body (@{email=$email}|ConvertTo-Json) | Out-Null
$login = Invoke-RestMethod "$base/auth/verify-code" -Method Post -ContentType application/json -Body (@{email=$email; code="888888"}|ConvertTo-Json)
$H = @{ Authorization = "Bearer $($login.token)"; "X-Workspace-ID" = $ws }
$agents = Invoke-RestMethod "$base/api/agents?workspace_id=$ws" -Headers $H
$scanner = $agents[0].id
$body = @{ name="weekly"; scanner_agent_id=$scanner; custom_focus="dead code"; include_standards=$true; include_checks=$true; cron_expression="0 9 * * 1"; timezone="UTC"; enabled=$true } | ConvertTo-Json
$scan = Invoke-RestMethod "$base/api/forge/entropy-scans" -Method Post -Headers $H -ContentType application/json -Body $body
Write-Output "SCAN: $($scan | ConvertTo-Json -Compress)"
if (-not $scan.autopilot_id) { Write-Output "FAIL: no backing autopilot"; return }
$ap = Invoke-RestMethod "$base/api/autopilots/$($scan.autopilot_id)" -Headers $H
Write-Output "AUTOPILOT: title=$($ap.title) mode=$($ap.execution_mode) assignee=$($ap.assignee_id)"
```
Expected: `SCAN` 含 `autopilot_id`；该 autopilot `execution_mode=create_issue`、`assignee_id=scanner`、title=`Entropy scan: weekly`。
**证明 Forge 代管 backing autopilot 工作。**

- [ ] **Step 3: 手动触发 → 断言 brief 落 issue.description**

Run (PowerShell，续上)：
```powershell
$run = Invoke-RestMethod "$base/api/autopilots/$($scan.autopilot_id)/trigger" -Method Post -Headers $H
Write-Output "RUN: $($run | ConvertTo-Json -Compress)"
# create_issue 模式：run 关联一个 issue。取 issue 看 description。
$issueId = $run.issue_id
if (-not $issueId) { Write-Output "NOTE: run skipped (agent runtime offline?) — admission gate; brief 合成在 dispatchCreateIssue 内，离线则不建 issue。见下方备注。"; return }
$issue = Invoke-RestMethod "$base/api/issues/$issueId" -Headers $H
$d = $issue.description
$ok = ($d -match "Entropy Scan: weekly") -and ($d -match "WHOLE-REPOSITORY") -and ($d -match "Additional focus areas") -and ($d -match "dead code")
Write-Output ("BRIEF_IN_ISSUE=" + $ok)
```
Expected: `BRIEF_IN_ISSUE=True` —— issue.description 含合成 brief 的标题段 + 全仓说明 + custom_focus（“dead code”）。
若 `include_standards/checks` 对应该 workspace 有 F1/F2 配置，亦应见 “declared standards (F1)” / “verification checks (F2)” 段。
**证明派发钩子把合成 brief 注入 issue.description（编排链端到端，无需 agent 真跑）。**

> **admission gate 备注**：`DispatchAutopilot` 在 assignee runtime 离线时会 `recordSkippedRun`（不建 issue）。
> 若 Step 3 命中 skip，说明 scanner agent 的 runtime 不在线。绕过法：选一个 runtime 在线的 agent 作 scanner，
> 或直接对该 workspace 已有的在线 agent 建 scan。brief 合成逻辑本身已由 `ComposeBrief` 单测 + 编译覆盖；
> 此 Step 验证的是“钩子真的在派发路径触发并落到 issue”。

- [ ] **Step 4: PATCH/DELETE 同步（可选加固）**

Run (PowerShell，续上)：
```powershell
$body2 = @{ name="weekly-2"; scanner_agent_id=$scanner; custom_focus="dead code"; include_standards=$true; include_checks=$true; cron_expression="0 10 * * 1"; timezone="UTC"; enabled=$false } | ConvertTo-Json
Invoke-RestMethod "$base/api/forge/entropy-scans/$($scan.id)" -Method Patch -Headers $H -ContentType application/json -Body $body2 | Out-Null
$ap2 = Invoke-RestMethod "$base/api/autopilots/$($scan.autopilot_id)" -Headers $H
Write-Output "AFTER PATCH: title=$($ap2.title) status=$($ap2.status)"   # 期望 title=Entropy scan: weekly-2, status=paused
Invoke-RestMethod "$base/api/forge/entropy-scans/$($scan.id)" -Method Delete -Headers $H | Out-Null
try { Invoke-RestMethod "$base/api/autopilots/$($scan.autopilot_id)" -Headers $H; Write-Output "FAIL: autopilot still exists" } catch { Write-Output "DELETE_OK: backing autopilot gone" }
```
Expected: PATCH 后 autopilot title=`Entropy scan: weekly-2`、status=`paused`；DELETE 后 backing autopilot 404（`DELETE_OK`）。

---

### Task 4.3: 活体扫描流程（凭证就绪后）

> **当前阻塞**：scanner agent 真跑需 provider 凭证（F0 遗留）+ runtime 在线。

- [ ] **Step（待凭证）**：配在线 scanner agent 建 scan → cron 到点（或手动触发）→ Autopilot 建 “Entropy scan” issue
  （description=合成 brief）→ 派 task → scanner agent 读 brief → 全仓巡检 → 经 `multica` CLI 为新发现建带
  `forge-entropy` 标签的 issue、对开放发现评论 bump → 扫描 issue 发摘要评论 → 完成。
  **断言**：出现带 `forge-entropy` 标签的发现 issue；第二次扫描注入开放清单、不重复建同一发现（软去重）。

> 编排链（scan→autopilot→派发→合成→落 issue.description）由 Task 4.2 绕凭证验证；活体扫描 + 去重待凭证。

---

### Task 4.4: 更新 index + 记忆

- [ ] **Step 1: index 标记完成**

`docs/forge/plans/eunomia-2026-06-01/index.md` Phase 表 5 行改 ✅；DoD 勾完（活体扫描标注凭证依赖）。

- [ ] **Step 2: 记忆**

`~/.claude/projects/D--shulex-work-forge/memory/forge-on-multica-pivot.md`：加 F4 段——熵管理=Autopilot
（周期全仓扫描 → advisory 建 issue）已落地：`forge_entropy_scan` 表 + 破环 `forge/checks` 子包 +
`forgeentropy` brief 合成 + `dispatchCreateIssue` 派发钩子 + handler 侧 backing autopilot 代管 +
`/api/forge/entropy-scans` + UI。绕凭证编排链已验（手动触发断言 brief 落 issue.description）；活体扫描待凭证。
并更新 `MEMORY.md` 索引行（F0–F4 done，next F4b/F5）。

- [ ] **Step 3: 提交**

```bash
git add docs/forge/plans/eunomia-2026-06-01/index.md
git commit -m "docs(plan): mark eunomia F4 complete (orchestration verified; live scan pending creds)"
```

---

## Phase 4 完成检查
- [ ] Go 测试（forge/standards + forge/checks + forgeentropy）+ 三包 typecheck 绿
- [ ] 源码构建起栈，`forge_entropy_scan` 表存在
- [ ] 绕凭证编排链：建 scan → backing autopilot + schedule 触发；手动触发 → `issue.description` 含合成 brief
- [ ] PATCH/DELETE 同步 backing autopilot
- [ ] 活体扫描流程记录，标注凭证依赖；index + 记忆更新
