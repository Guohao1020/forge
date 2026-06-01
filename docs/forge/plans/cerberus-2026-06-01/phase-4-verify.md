## Phase 4 — 验收 + 文档

**Goal:** 跑全量测试 + typecheck；源码构建确认 server 带 F2 启动；记录门禁逻辑验证 + 标注
活体 e2e 的凭证依赖；更新 index + 记忆。

**Depends-on:** Phase 2、Phase 3　**Unblocks:** F3
**Completion gate:** Go 测试 + 三包 typecheck 绿；源码构建启动；F2 DoD 勾完（活体门禁 e2e 标注
凭证依赖）。

> **关键修正（vs spec §8/R4）**：F2 门禁只在 agent **成功完成**后触发，**不能**绕开 provider
> 凭证验证（不同于 F1 在 claim 阶段注入）。门禁**逻辑**由单测全覆盖；**活体门禁**（真 agent
> 完成→拦截→评论）需可用 provider 凭证，延后到凭证就绪。

---

### Task 4.1: 全量测试 + typecheck

- [ ] **Step 1: Go 测试**

Run: `wsl -d Ubuntu -- bash -lc "cd /mnt/d/shulex_work/forge/server && go test ./internal/forge/... ./internal/daemon/ -run 'Check|Resolve|RunChecks' 2>&1 | tail -15"`
Expected: ResolveChecks（2）+ runChecks（2）全绿。

- [ ] **Step 2: 全量 build + vet + 前端 typecheck**

Run:
```bash
wsl -d Ubuntu -- bash -lc "cd /mnt/d/shulex_work/forge/server && go build ./... && go vet ./internal/forge/... ./internal/daemon/ 2>&1 | tail -5"
```
```powershell
cd D:\shulex_work\forge; corepack pnpm --filter "@multica/core" --filter "@multica/views" --filter "@multica/web" typecheck 2>&1 | Select-Object -Last 8
```
Expected: 编译/vet 干净；三包 typecheck Done。

---

### Task 4.2: 源码构建 + 端点 resolve 验证（绕凭证可做的部分）

- [ ] **Step 1: 源码构建起栈**

Run（沿用 F1 验证用的 forge-build 栈；迁移 112 随 backend 启动应用）:
```powershell
cd D:\shulex_work\forge
docker compose --env-file D:\shulex_work\multica\.env -f docker-compose.selfhost.yml -f docker-compose.selfhost.build.yml -p forge-build up -d --build backend postgres
```
Expected: 镜像重建、backend 启动、`curl localhost:8081/health` 200。确认 `forge_check` 表存在
（`docker exec forge-build-postgres-1 psql -U multica -d multica -c "\dt forge_check"`）。

- [ ] **Step 2: 经 API 配置一个 check + 确认 CRUD**

Run（PowerShell，复用 F1 的 API 登录拿 JWT + X-Workspace-Id；见 docs/forge/dev-windows.md）:
```powershell
# $H = @{ Authorization="Bearer <jwt>"; "X-Workspace-Id"=<wsid> }
$body = @{ name="reject-all"; command="exit 1" } | ConvertTo-Json
Invoke-RestMethod "$base/api/forge/checks" -Method Post -Headers $H -ContentType application/json -Body $body
Invoke-RestMethod "$base/api/forge/checks" -Headers $H   # 断言列表含 reject-all
```
Expected: 201 创建；列表返回含 `reject-all`/`exit 1`。**证明 checks CRUD 在源码构建服务器工作。**

> daemon 端点 `GET /api/daemon/tasks/{id}/forge-checks` 需 claim 出的 task token，手动复现成本高；
> 其 resolve 逻辑由 ResolveChecks 单测覆盖，endpoint 由 build 编译保证。

---

### Task 4.3: 活体门禁 e2e（凭证就绪后）

> **当前阻塞**：需一个能跑到 completed 的 provider（F0 遗留凭证问题）。凭证就绪后按此验证：

- [ ] **Step（待凭证）**：配 workspace check `exit 1` → 建 issue 派给可用 agent →
  daemon 跑完 agent（completed）→ **门禁触发**：daemon 在 workdir 跑 `exit 1` → 失败 →
  task 走 FailTask(`verification_failed`) → **断言**：task 状态 failed + failure_reason
  verification_failed + issue 上有 "❌ Verification failed" 评论。
- 反向：把 check 改成 `exit 0` → 同流程 → task 正常 completed（门禁放行）。

---

### Task 4.4: 更新 index + 记忆 + 收尾

- [ ] **Step 1: 标记 cerberus index 完成**

`docs/forge/plans/cerberus-2026-06-01/index.md` Phase 表改 ✅；DoD 勾完（活体 e2e 标注凭证依赖）。

- [ ] **Step 2: 更新记忆**

`~/.claude/projects/D--shulex-work-forge/memory/forge-on-multica-pivot.md`：F2 验证门禁
（daemon 侧命令门禁）已落地——`forge_check` 表、`server/internal/forge/checks.go`、
daemon `forge_verify.go` + handleTask 钩子、`/api/forge/checks` + UI。标注活体门禁 e2e 待凭证。

- [ ] **Step 3: 提交 + push（合并分支由 finishing-a-development-branch 决定）**

```bash
git add docs/forge/plans/cerberus-2026-06-01/index.md
git commit -m "docs(plan): mark cerberus F2 complete (gate logic verified; live e2e pending creds)"
```

---

## Phase 4 完成检查
- [ ] Go 测试（ResolveChecks + runChecks）+ 三包 typecheck 绿
- [ ] 源码构建起栈，`forge_check` 表存在，checks CRUD 往返成功
- [ ] 活体门禁 e2e 步骤记录，标注凭证依赖
- [ ] index + 记忆更新
