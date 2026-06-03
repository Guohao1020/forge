# Forge 活体执行 Runbook & 关键发现（2026-06-03）

> **安全前置**：本仓库 `origin` 是**公开** GitHub 仓库。本文档中所有凭证一律用占位符
> （`<ROUTER_BASE_URL>` / `<ROUTER_API_KEY>`）。真实的 key 只存在于 **DB 侧 agent.custom_env**，
> **永不**写入任何 git 跟踪文件。`~/.codex/auth.json`、旧 `.env` 里的 `GITHUB_CLIENT_SECRET`
> 全程未碰、未提交。

## 0. 背景

F0–F5 全部切片此前都是**绕凭证 / 源码构建栈**验证的——平台编排逻辑（Go 后端 + DB）跑通，但
agent 从未真正调用过 LLM。本次接入真实 provider 凭证（用户提供的一个 Anthropic + OpenAI 双兼容
的 new-api 路由器），把**活体执行**端到端打通,并在 Codex + 真实代码 + 真门禁下验证了整条
Harness 闭环。

执行分两层，理解这一点是所有调试的基础：

| 层 | 跑什么 | 需要 provider 凭证? |
|----|--------|----------------------|
| **平台编排** | daemon 认领任务、注入规范、跑门禁、入队评审、回写 DB | 否（F0–F5 早已验证） |
| **agent 活体执行** | coding CLI（Claude Code / Codex）在沙箱里调 LLM 出活 | **是**（本次接入） |

"runtime online"（daemon 连上）≠ "agent can run"（CLI 能认证到 LLM）。

## 1. 路由器

- 一个 new-api 风格的代理（`_type: newapi_channel_conn`），**同时**暴露 Anthropic
  (`/v1/messages`) 与 OpenAI (`/v1/chat/completions`、`/v1/responses`) 端点。
- `GET /v1/models` 列出 claude / gemini / gpt 系列。`owned_by: "codex"` 的 `gpt-5.x` 可用。
- 后端是 **ChatGPT 账号通道**：`-codex` 后缀模型（`gpt-5.3-codex` 等）会被拒
  （`"not supported when using Codex with a ChatGPT account"`），用通用 `gpt-5.5` 即可。
- 占位：base URL = `<ROUTER_BASE_URL>`，bearer key = `<ROUTER_API_KEY>`。

## 2. Claude agent（Inj）接入

daemon 在 `daemon.go` 把 `agent.custom_env` 注入 CLI 环境（`isBlockedEnvKey` 只挡
`MULTICA_*` / `HOME` / `PATH` / `CODEX_HOME` 等，**不挡** `ANTHROPIC_*` / `CLAUDE_CONFIG_DIR`）。

`custom_env` 设：

```
ANTHROPIC_BASE_URL          <ROUTER_BASE_URL>
ANTHROPIC_AUTH_TOKEN        <ROUTER_API_KEY>
ANTHROPIC_MODEL             claude-sonnet-4-5
ANTHROPIC_SMALL_FAST_MODEL  claude-haiku-4-5
CLAUDE_CONFIG_DIR           C:\Users\86157\.forge-agent-claude
```

**关键坑——OAuth 遮蔽**：`claude -p` 直连路由器报 `401 Invalid token`，但同一 key 用 `curl`
打 `/v1/messages` 能过认证 → 说明 token 有效，是 Claude Code **优先用了本机登录态的 OAuth 凭证**
（订阅登录），把 OAuth bearer 发给路由器被拒。

**修复**：给 Claude Code 指定一个**隔离、无 OAuth** 的 `CLAUDE_CONFIG_DIR`（拷一份初始化过、
不含 `.credentials.json` 的配置目录），它就回退用 `ANTHROPIC_AUTH_TOKEN`。daemon 继承用户 `HOME`，
所以这条必须走 `custom_env` 注入到 per-task 环境。

## 3. Codex agent（CodexForge）接入

Codex 走 OpenAI 协议，配置机制与 Claude 完全不同：

- daemon 的 `execenv/codex_home.go` 把 per-task `CODEX_HOME` 从共享 `~/.codex/`
  **symlink `auth.json`、复制 `config.toml`**。`CODEX_HOME` 在 `isBlockedEnvKey` 里，不能注入。
- 因此 provider 走 **per-agent `custom_args`**（`-c` 覆盖），**完全不碰 `~/.codex`**：

```
-c model_provider=router
-c model_providers.router.base_url="<ROUTER_BASE_URL>/v1"
-c model_providers.router.env_key="ROUTER_API_KEY"
-c model_providers.router.name="router"
```

key 走 `custom_env` 的 `ROUTER_API_KEY`，`agent.model = gpt-5.5`。

- **Codex 的 `env_key` 机制天然避开 OAuth 遮蔽**：选中自定义 provider 后用 `env_key` 指向的
  环境变量做 bearer，不读 `auth.json`。比 Claude 还省一步。
- **`wire_api="chat"` 在新版 Codex 已删除**（报错让你改 `responses`）。路由器的
  `/v1/responses` 要求 `stream:true`（Codex 本就流式 SSE），直连验证能正确吐 `output_text`。
  所以**用默认 `responses`，不要写 `wire_api`**。
- `agent.custom_args` / `custom_env` 都是 `jsonb`；daemon 在 `daemon.go` 把
  `agent.Model → opts.Model`、`agent.CustomArgs → opts.CustomArgs`，backend
  (`pkg/agent/codex.go`) 把 custom_args 追加到 `codex app-server` 后面（只挡 `--listen`）。

## 4. 真实 gate→review 闭环

要让 F2 门禁 / F3 评审在**真实代码**上跑，需要给项目绑一个仓库工作区：

- `project_resource` 的 `local_directory` 是 **in-place**（agent 直接在该目录改，不是 worktree
  副本）。**所以绝不能指向真实 checkout** —— 用一份隔离克隆并删掉 remote：

  ```bash
  git clone --depth 1 "file:///D:/shulex_work/forge" D:\forge-harness-demo
  git -C D:\forge-harness-demo remote remove origin   # 杜绝任何 push
  ```

- `resource_ref = {"local_path":"D:\\forge-harness-demo","daemon_id":"<daemon-id>"}`，
  挂到一个 `project` 上；issue 落在该 project，coder 任务的 workdir 即被 daemon 解析为该克隆。
- 验证过的闭环：**F1 规范注入（AGENTS.md 被写入 `[api] rest` 标准）→ Coder 改 `score.go`
  → F2 门禁 `go vet` 真跑（daemon 日志 `INF forge check passed`）→ F3 评审 `git diff` 后回帖**。

## 5. 关键坑 & 修复（全部踩过）

1. **Stale daemon（F2 从没真跑）**：部署在 `~/.multica/bin/multica.exe` 的二进制停在 5/29，
   早于 F2 源码（6/1）—— 二进制里搜不到 `forge-checks` / `verification_failed`。F1/F3 能跑是因为
   它们在 **server 端**（server 从源码 `go run`，始终最新）；F2 门禁在 **daemon 端**，老 daemon
   根本不 fetch checks，任务静默判过。
   **修复**：当前源码重编译 + 重启 daemon：

   ```powershell
   & "C:\Program Files\Go\bin\go.exe" build -o <tmp>\multica-new.exe ./cmd/multica   # 在 server/ 下
   multica daemon stop ; copy <tmp>\multica-new.exe ~\.multica\bin\multica.exe ; multica daemon start
   ```

   daemon_id 与三个 runtime ID **重启后保持稳定**（持久化在 `~/.multica`），无需重新绑定 agent。
   旧二进制备份 `multica.exe.bak-20260603`。

2. **F2 check 解析是 additive**（`forge/checks/resolve.go`：workspace + project 检查都跑）。
   种子里有个 `reject-all`（`command = exit 1`）测试检查，会让门禁恒挂——**必须禁用**它，
   再加真检查。

3. **go.mod 在 `server/` 不在仓库根**：门禁命令要 `cd server && go vet ./internal/forgehealth/`。
   `forgehealth` 只 `import "math"`（零外部依赖），所以 `go vet` 无需下载、又快又稳。
   门禁经 `bash -lc` 在 workdir 跑 —— 该 `bash` 是 **WSL**（`/mnt/d/...` 视角）。

4. **Codex Windows elevated 沙箱弹 UAC**：新 daemon 给 codex 写了 `sandbox_mode = "workspace-write"`，
   叠加用户 `~/.codex` 的 `[windows] sandbox = "elevated"`，Windows 下要拉起需管理员授权的
   elevated 助手 → 每个 codex 任务弹 UAC，不点就把第一个 `exec_command` 卡到 10 分钟 inactivity 超时。
   （旧 daemon 不写 workspace-write，所以以前不弹。）
   **修复**：给 agent 加 `-c sandbox_mode="danger-full-access"`（per-agent，**不碰 `~/.codex`**）——
   覆盖掉 daemon 的沙箱块，不再拉 elevated 助手，免弹框。权衡：Forge 的 codex 任务变无沙箱
   （用户级全权，仍低于点"允许"授予的管理员级；且只在隔离克隆里跑、brief 有界）。

## 6. 复现清单（关键坐标）

- workspace `1379e8af-…`，owner `harvey@forge.local`（user `f6ae7312-…`）
- daemon_id `019e798d-…`，runtimes：claude `1f8974e1-…` / codex `5ee21316-…` / openclaw `33702ac1-…`
- agents：Inj（Claude）`180f52ce-…`；CodexForge `ca700162-…`
- 隔离克隆 `D:\forge-harness-demo`（depth 1，无 remote）
- 隔离 Claude 配置 `C:\Users\86157\.forge-agent-claude`
- DB：容器 `forge-build-postgres-1`，`psql -U multica -d multica`

> 凭证不在此列：`<ROUTER_BASE_URL>` / `<ROUTER_API_KEY>` 仅存于对应 agent 的 `custom_env`
> （owner-only 审计端点 `PUT /api/agents/{id}/env` 的存储位）。
