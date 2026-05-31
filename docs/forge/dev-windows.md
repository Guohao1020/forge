# Forge 本地开发 runbook（Windows）

> F0 选定路径：**Docker selfhost**（平台跑 Docker，daemon 原生跑）。绕开 WSL2 的
> Go/pnpm/sudo toolchain。验证于 2026-05-30，Windows 11 + Docker Desktop。

## 先决条件

- Docker Desktop（含运行中的引擎）
- `claude` CLI（或其他 provider CLI）—— daemon 会自动检测
- 一个可用的 provider 凭证（见末尾"已知坑"）

## 起平台（预构建镜像，最快）

```powershell
# 仓库根目录
copy .env.example .env
```

编辑 `.env`，本地确定性登录 + 避开端口冲突：

```
APP_ENV=development
MULTICA_DEV_VERIFICATION_CODE=888888
ALLOW_SIGNUP=true
JWT_SECRET=forge-local-dev-secret-change-me
BACKEND_PORT=8081          # 8080 被其他项目占用时改这里
FRONTEND_PORT=3000
FRONTEND_ORIGIN=http://localhost:3000
MULTICA_APP_URL=http://localhost:3000
```

拉起（预构建上游镜像；要跑本仓库源码改用 `docker-compose.selfhost.build.yml`）：

```powershell
docker compose --env-file .env -f docker-compose.selfhost.yml -p forge up -d
docker compose -p forge ps          # 等 postgres healthy、backend/frontend running
```

健康检查：`http://localhost:8081/health` 与 `http://localhost:3000/` 均应 200。
backend 会自动跑完全部迁移。

## 登录 + daemon（两种方式）

### A. 浏览器（标准）
打开 `http://localhost:3000` → 邮箱随便填 → 验证码 `888888` → 建 workspace。
然后：

```powershell
multica setup self-host --server-url http://localhost:8081 --app-url http://localhost:3000
multica daemon start
multica daemon status      # 应看到 Agents: claude, ... ; Workspaces: 1
```

### B. 纯 API / 非交互（自动化）
```powershell
$base="http://localhost:8081"; $email="you@forge.local"
Invoke-RestMethod "$base/auth/send-code"   -Method Post -ContentType application/json -Body (@{email=$email}|ConvertTo-Json)
$r = Invoke-RestMethod "$base/auth/verify-code" -Method Post -ContentType application/json -Body (@{email=$email;code="888888"}|ConvertTo-Json)
$H=@{Authorization="Bearer $($r.token)"}
$ws = Invoke-RestMethod "$base/api/workspaces" -Method Post -Headers $H -ContentType application/json -Body (@{name="Forge";slug="forge"}|ConvertTo-Json)
$pat = Invoke-RestMethod "$base/api/tokens" -Method Post -Headers $H -ContentType application/json -Body (@{name="daemon"}|ConvertTo-Json)
multica config set server_url http://localhost:8081
multica config set app_url http://localhost:3000
multica login --token $pat.token
multica daemon start
```

## 停 / 清理

```powershell
multica daemon stop
docker compose -p forge stop        # 停服务，留数据卷
docker compose -p forge down        # 连同容器删除（卷仍保留，除非加 -v）
```

## 已知坑

- **端口 8080 冲突**：本机其他项目可能占用；用 `.env` 的 `BACKEND_PORT` 改到 8081。前端同源代理到容器内 `backend:8080`，改主机端口不影响浏览器流程。
- **provider 凭证**（F1 前必须解决）：
  - `claude`：org 若禁用 Claude Code headless 订阅访问，需设 `ANTHROPIC_API_KEY`（注入 agent 的 `custom_env` 或 daemon 环境）。
  - `codex`：ChatGPT 账号 + 旧 CLI 版本会报模型不支持；需升级 codex 或用 API key 模式。
- **WSL2 路径**（备选）：若改走 `make dev`，需在 WSL2 装 Go 1.26+ 与 pnpm ≥10.28（用户级安装免 sudo），并注意 WSL→Windows localhost 的 mirrored networking。
