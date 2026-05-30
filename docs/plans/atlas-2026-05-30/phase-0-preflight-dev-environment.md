## Phase 0 — Pre-flight & Dev Environment

**Goal:** 在动 forge 仓库前，用已克隆的 `D:\shulex_work\multica` 证明本地 dev 路径能把
Multica 端到端跑通，锁定 R1（Windows dev）风险，决定 WSL2 还是 Docker 路径。

**Depends-on:** 无（第一个 phase）
**Unblocks:** Phase 1, Phase 2
**Completion gate:** Multica 在本地端到端跑通（登录 + daemon 检测到 `claude` + 一个 issue
被 Claude Code agent 执行回报）；dev 路径已决定并记录在 `D:\shulex_work\_atlas_notes.md`。

> 本 phase 是纯验证 / 决策，不改 forge 仓库、不提交 forge 代码。产出是"哪条 dev 路径可行"
> 的事实结论 + 一份临时笔记（后续 Phase 4 转成正式 runbook）。

---

### Task 0.1: 盘点先决条件

**Files:**
- Create: `D:\shulex_work\_atlas_notes.md`（仓库外临时笔记）

- [ ] **Step 1: 在 PowerShell 检查 Windows 侧工具版本**

Run:
```powershell
wsl -l -v; docker --version; go version; node --version; pnpm --version; claude --version; make --version
```
Expected: 记录每项是否存在及版本。关注：是否有 WSL2 发行版（STATE=Running/Stopped, VERSION=2）、Docker、`claude`。
`make` 在 Windows 原生通常没有 —— 这本身就提示走 WSL2 或 Docker。

- [ ] **Step 2: 若有 WSL2，检查发行版内的工具**

Run:
```powershell
wsl -d Ubuntu -- bash -lc "go version; node --version; pnpm --version; claude --version; make --version; docker --version"
```
Expected: 记录 WSL2 内各工具版本。缺的标出来（Phase 0.2 补装）。

- [ ] **Step 3: 把结论写进笔记**

写入 `D:\shulex_work\_atlas_notes.md`：每个工具在 Windows / WSL2 各自的存在性与版本，
以及初步判断（WSL2 可行 / 需补装 / 退 Docker）。

- [ ] **Step 4: 提交（无 —— 仓库外笔记，不提交）**

本 task 不提交 forge 仓库。

---

### Task 0.2: 决定并准备 dev 路径

Multica 需要：Node 20+、pnpm 10.28+、Go 1.26+、Docker（Postgres 容器）。
两条路径，按 0.1 结论二选一。

- [ ] **Step 1（主路径 WSL2）：在 WSL2 Ubuntu 补齐缺失工具**

按 0.1 缺什么补什么，例如：
```bash
# Go 1.26
wget -qO- https://go.dev/dl/go1.26.0.linux-amd64.tar.gz | sudo tar -C /usr/local -xz
# Node 20 + pnpm
curl -fsSL https://deb.nodesource.com/setup_20.x | sudo -E bash - && sudo apt-get install -y nodejs
sudo npm i -g pnpm@10.28
# claude CLI（若 WSL2 内没有）
npm i -g @anthropic-ai/claude-code
```
Expected: `go version` ≥ 1.26、`node -v` ≥ v20、`pnpm -v` ≥ 10.28、`claude --version` 可用。

- [ ] **Step 2（主路径 WSL2）：确认 WSL2 能用 Docker**

Run（WSL2 内）:
```bash
docker ps
```
Expected: 不报错（Docker Desktop 已开启 WSL2 集成，或 WSL2 内装了 docker engine）。
若失败：在 Docker Desktop 设置里开启该发行版的 WSL integration。

- [ ] **Step 3（备选 Docker 路径）：若 WSL2 不可行，确认自托管 compose 可用**

Run（任意有 Docker 的 shell）:
```bash
cat D:/shulex_work/multica/docker-compose.selfhost.yml
```
Expected: 确认 compose 定义了 server + web + postgres（+ redis）。记下端口。
此路径下平台跑 Docker、daemon 仍需在能访问 `claude` 的宿主上单独跑。

- [ ] **Step 4: 在笔记记录最终选定路径**

写入 `_atlas_notes.md`：选定 WSL2 还是 Docker，及补装了什么。

---

### Task 0.3: 在 multica 克隆上拉起服务

> 用现成的 `D:\shulex_work\multica`（不碰 forge 仓库）验证整套能起来。

- [ ] **Step 1: 准备 env（开启确定性本地登录）**

WSL2 内进入仓库（建议把 clone 放到 WSL2 文件系统以避免 /mnt 性能与权限问题；
可 `cp -r /mnt/d/shulex_work/multica ~/multica`）：
```bash
cd ~/multica   # 或 /mnt/d/shulex_work/multica
cp .env.example .env
# 开启本地确定性验证码登录
sed -i 's/^MULTICA_DEV_VERIFICATION_CODE=.*/MULTICA_DEV_VERIFICATION_CODE=888888/' .env
```
Expected: `.env` 中 `MULTICA_DEV_VERIFICATION_CODE=888888`，`APP_ENV=`（非 production）。

- [ ] **Step 2: 拉起数据库 + 一键 bootstrap**

Run:
```bash
make db-up
make dev
```
Expected: Postgres 容器起来；`make dev` 安装依赖、跑迁移、启动 Go server（:8080）与 web。
留意输出里 server 与 web 的就绪日志。

- [ ] **Step 3: 验证服务健康**

Run:
```bash
curl -fsS http://localhost:8080/health || curl -fsS http://localhost:8080/healthz || curl -fsS http://localhost:8080/
```
Expected: 返回 200 / 健康响应。再浏览器开 web（默认 Next.js 端口，见 `make dev` 输出）确认页面加载。

- [ ] **Step 4: 记录结果**

笔记记下：哪个 `make` 目标起了哪些服务、实际端口、踩到的坑与解法。

---

### Task 0.4: 在 multica 克隆上跑通 e2e 闭环

- [ ] **Step 1: 登录建 workspace**

浏览器打开 web → 用任意邮箱触发验证码 → 输入 `888888`（或看 server stdout 打印的码）登录
→ 完成 onboarding，建一个 workspace。
Expected: 进入 dashboard，URL 形如 `/<workspaceSlug>`。

- [ ] **Step 2: 启动 daemon 并确认 runtime 上线**

Run（WSL2 内，已 `claude` 在 PATH）:
```bash
make multica ARGS="login"      # 浏览器授权，或按 CLI 提示
make daemon                    # 启动本地 daemon
```
然后 web → Settings → Runtimes。
Expected: 看到本机 runtime 为 active，且检测到 `claude` provider。

- [ ] **Step 3: 建 issue → assign 给 Claude Code agent**

web → Settings → Agents → New Agent（选刚上线的 runtime + Claude Code provider，命名如
`forge-smoke`）。再建一个 issue（标题如 "create hello.txt with content hi"），assign 给该 agent。
Expected: issue 被 daemon 认领，状态走 queued → running；agent 卡片显示执行中。

- [ ] **Step 4: 验证执行与回报**

观察 issue 详情的实时消息流。
Expected: agent 跑完，issue 进 in_review/done，消息流里有 Claude Code 的执行记录。
截图保存到 `_atlas_notes.md` 同目录作证据。

> 若此步失败（daemon 起不来 / 检测不到 claude / 执行报路径错），回到 Task 0.2 切换路径或修环境。
> **这是 F0 最核心的去风险点 —— 必须在 Phase 1 不可逆改动前跑通。**

---

### Task 0.5: 锁定 dev 路径决策

- [ ] **Step 1: 在笔记定稿 dev 路径**

`_atlas_notes.md` 写明：最终采用的路径（WSL2 / Docker）、完整起停命令序列、登录方式、
daemon 启动方式、踩坑清单。这份内容 Phase 4 会转成正式 `dev-windows.md` runbook。

- [ ] **Step 2: 标记 Phase 0 完成门禁**

确认：✅ Multica 端到端跑通　✅ dev 路径已定并记录。
本 phase 无 forge 仓库提交。

---

## Phase 0 完成检查

- [ ] 先决条件已盘点，结论入笔记
- [ ] dev 路径已选定并准备好（WSL2 主 / Docker 备）
- [ ] Multica 服务在本地起来且健康
- [ ] e2e 闭环跑通（登录 + daemon 检测 claude + issue 被 agent 执行回报）
- [ ] dev 路径决策锁定在 `_atlas_notes.md`
