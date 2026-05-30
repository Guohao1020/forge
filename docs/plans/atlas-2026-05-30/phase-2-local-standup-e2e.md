## Phase 2 — Local Stand-up & E2E Acceptance

**Goal:** 用 Phase 0 锁定的 dev 路径，在已改造的 forge 仓库（现为 Multica base）上拉起整套服务，
跑通 F0 核心验收闭环，并跑一遍 Multica 自带验证管线。

**Depends-on:** Phase 0（dev 路径）、Phase 1（仓库已是 Multica）
**Unblocks:** Phase 3
**Completion gate:** forge 仓库本地 e2e 闭环跑通（登录 → workspace → daemon 检测 `claude`
→ issue 被 Claude Code agent 执行回报 → 进 review），证据留存；`make check` 不因 F0 改动变红。

> 与 Phase 0 的区别：Phase 0 在 `multica` 克隆上去风险；Phase 2 在 **forge 仓库本体** 上做正式验收。

---

### Task 2.1: 在 forge 仓库准备 env 并拉起服务

- [ ] **Step 1: 准备 env**

在 Phase 0 选定的环境进入 forge 仓库（若用 WSL2 且 clone 在 /mnt/d，建议 `cp -r` 到 WSL2 文件系统）：
```bash
cd <forge-repo-path>
cp .env.example .env
sed -i 's/^MULTICA_DEV_VERIFICATION_CODE=.*/MULTICA_DEV_VERIFICATION_CODE=888888/' .env
```
Expected: `.env` 就绪，确定性登录已开。

- [ ] **Step 2: 拉起 DB + 服务**

Run:
```bash
make db-up
make dev
```
Expected: Postgres 起来、迁移跑完、server(:8080) 与 web 启动。

- [ ] **Step 3: 健康检查**

Run:
```bash
curl -fsS http://localhost:8080/health || curl -fsS http://localhost:8080/
```
Expected: 200 / 健康响应；浏览器打开 web 页面加载正常。

---

### Task 2.2: 跑通 F0 验收闭环

- [ ] **Step 1: 登录 + 建 workspace**

浏览器 web → 邮箱触发验证码 → 输 `888888`（或 server stdout 的码）→ onboarding → 建 workspace。
Expected: 进入 `/<workspaceSlug>` dashboard。

- [ ] **Step 2: 启动 daemon，确认 runtime + claude**

Run:
```bash
make multica ARGS="login"
make daemon
```
web → Settings → Runtimes。
Expected: 本机 runtime active，检测到 `claude` provider。

- [ ] **Step 3: 建 agent + issue 并 assign**

web → Settings → Agents → New Agent（runtime + Claude Code，命名 `forge-smoke`）；
建 issue "create hello.txt with content hi"，assign 给该 agent。
Expected: issue 走 queued → running。

- [ ] **Step 4: 验证执行回报 + 留证**

观察 issue 实时消息流至完成。
Expected: issue 进 in_review/done，消息流含 Claude Code 执行记录。截图存档作 F0 验收证据。

---

### Task 2.3: 跑 Multica 自带验证管线

- [ ] **Step 1: 运行 check**

Run:
```bash
make check
```
Expected: typecheck + TS tests + Go tests + Playwright E2E 跑完。记录结果。
（F0 未改业务代码，应与上游一致；若个别 E2E 因本地环境 flaky，记录但不算 F0 回归。）

- [ ] **Step 2: 记录结果**

把通过/失败概况写进 `_atlas_notes.md`。失败项判断是否本地环境导致（非 F0 改动引入）。

---

### Task 2.4: 停服并记录起停序列

- [ ] **Step 1: 优雅停服**

Run:
```bash
make stop
make db-down   # 仅停容器，不删 volume
```
Expected: 进程与容器停止，DB volume 保留。

- [ ] **Step 2: 记录完整起停序列**

把"准备 env → db-up → dev → login → daemon → 闭环 → stop"的完整命令序列固化进 `_atlas_notes.md`
（Phase 4 转 runbook）。

---

## Phase 2 完成检查

- [ ] forge 仓库本地服务起来且健康
- [ ] F0 验收闭环跑通，截图证据留存
- [ ] `make check` 已跑，结果记录（无 F0 引入的回归）
- [ ] 完整起停序列固化
