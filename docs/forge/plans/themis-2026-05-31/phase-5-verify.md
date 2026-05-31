## Phase 5 — 验收 + 种子 + 文档

**Goal:** 跑全量测试 + typecheck；构建 fork；种子一个 standard 并**端到端验证注入**
（绕开 provider 凭证）；更新 index 状态 + 记忆。

**Depends-on:** Phase 2（注入）、Phase 4（UI）　**Unblocks:** F2
**Completion gate:** Go 测试 + typecheck 绿；注入在真实 claim 响应里可见；F1 DoD 勾完。

> **绕开凭证的端到端验证**：standards 注入发生在 daemon **claim** 阶段；agent 因凭证失败是在
> claim *之后*。所以"被认领任务的 agent payload 里 instructions 含 core、skills 含
> forge-standards"即证明 F1 端到端打通，**不需要 agent 真正跑成功**。

---

### Task 5.1: 全量测试 + typecheck

- [ ] **Step 1: Go 测试**

Run: `cd <forge-repo>/server && go test ./internal/forge/... ./internal/handler/ -run 'Forge|Standard' 2>&1 | tail -15`
Expected: forge 包单测 + handler 往返测试全绿。

- [ ] **Step 2: 全量 Go + 前端 typecheck**

Run:
```bash
cd <forge-repo>/server && go build ./... && go vet ./internal/forge/... 2>&1 | tail -5
cd <forge-repo> && pnpm --filter @multica/core --filter @multica/views --filter @multica/web typecheck 2>&1 | tail -8
```
Expected: 编译/vet 干净；三个包 typecheck 通过。

---

### Task 5.2: 构建 fork 并起栈（源码构建）

> F0 跑的是上游预构建镜像；F1 有源码改动，需用 build compose 跑本仓库代码。

- [ ] **Step 1: 源码构建并起栈**

Run:
```bash
cd <forge-repo>
docker compose --env-file .env -f docker-compose.selfhost.build.yml -p forge up -d --build 2>&1 | tail -15
```
Expected: backend/web 从源码构建成功；容器 running。迁移 111 自动应用（或手动 `make migrate-up`）。
验证 `curl localhost:8081/health` 200。

> 若 backend 镜像未自动迁移，跑 `make migrate-up`（DATABASE_URL 指向 selfhost postgres）。

---

### Task 5.3: 种子 standard + 端到端注入验证（绕开凭证）

- [ ] **Step 1: 经 API 建一个 workspace 级 standard**

Run（PowerShell，复用 F0 的 API 登录拿 JWT + X-Workspace-Id）:
```powershell
# 见 docs/forge/dev-windows.md 的 API 登录拿 $H
$body = @{ name="rest"; category="api"; profile_tags=@(); core_content="URL 用 kebab-case；响应统一 Result 包装。"; detail_content="## REST 详规\n- 资源名复数\n- 版本放 path /api/v1/" } | ConvertTo-Json
Invoke-RestMethod "$base/api/forge/standards" -Method Post -Headers $H -ContentType application/json -Body $body
Invoke-RestMethod "$base/api/forge/standards" -Headers $H   # 断言列表含该 standard
```
Expected: 201 创建；列表返回含 `rest`。

- [ ] **Step 2: 建 issue 派给 agent，检查 claim 响应里的注入**

建一个 agent + issue（见 F0 流程），让 daemon 认领；从 daemon.log 或经 daemon claim API
观察该任务的 agent payload。
Expected: **task 的 agent instructions 末尾含 "Forge Coding Standards (mandatory)" + core
内容；skills 含 name=forge-standards 的条目**。这证明双层注入端到端生效（agent 后续因凭证
失败不影响本结论）。

> 备选（更确定）：直接对 selfhost DB 跑一个 Go 集成测试，构造 workspace+project+standard，
> 调 `forge.InjectStandards` 走真实 `*db.Queries`，断言 payload。

- [ ] **Step 3: 记录证据**

把 claim 响应/日志片段（含注入的 instructions + forge-standards skill）截图或贴入验收记录。

---

### Task 5.4: 更新 index + 记忆 + 收尾

- [ ] **Step 1: 标记 themis index 完成**

把 `docs/forge/plans/themis-2026-05-31/index.md` 的 Phase 表状态改 ✅，勾完 F1 DoD。

- [ ] **Step 2: 更新项目记忆**

更新 `~/.claude/projects/D--shulex-work-forge/memory/forge-on-multica-pivot.md`：F1 规范中心
（Standards 双层注入）已落地；记录 `forge_standard`/`forge_project_profile` 表、
`server/internal/forge/` 包、唯一侵入点 daemon claim。

- [ ] **Step 3: 最终提交 + push**

```bash
cd <forge-repo>
git add docs/forge/plans/themis-2026-05-31/index.md
git commit -m "docs(plan): mark themis F1 standards complete"
git push origin main
```

---

## Phase 5 完成检查
- [ ] Go 测试 + 三包 typecheck 全绿
- [ ] fork 源码构建起栈成功
- [ ] 端到端注入在真实 claim 响应里可见（绕开凭证）
- [ ] index 标记完成、记忆更新、push
