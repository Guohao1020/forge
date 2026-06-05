# Phase 4 — 绕凭证集成验收 + seed + 文档

依赖:Phase 0–3。产出:源码构建栈上"注册→选用→派发解析"端到端实测(不跑活体)+ seed + 文档。

---

### Task 4.1: 绕凭证集成实测(源码构建栈)

**Files:**
- Create: `server/internal/nacos/verify_e2e_test.go`(`//go:build integration`)或一个 `scripts/verify-mcp.sh`

- [ ] **Step 1: 起栈 + Nacos**

```bash
docker compose -f docker-compose.selfhost.yml -f docker-compose.selfhost.build.yml -p forge-build up -d --build
curl -s http://127.0.0.1:8848/nacos/ >/dev/null && echo nacos-up
```

- [ ] **Step 2: 端到端断言(不跑活体 agent)**

```
1. POST /api/mcp-registry/servers 注册一个 stdio server(shared ns,env_keys=[DEMO_KEY],published)
2. PUT /api/agents/{id} 设 mcp_refs=[{namespace:"shared",name:"demo",ref:"stable"}]
   + 给该 agent custom_env 设 DEMO_KEY=xxx(经 owner-only /env 端点)
3. 直接调 mcpresolve.ResolveMCP(或触发一次 task-build 路径)→ 断言有效 mcp_config:
   mcpServers.demo.command 正确、env.DEMO_KEY=xxx(secret 从 custom_env 注入)、tag→具体版本
```

- [ ] **Step 3: 降级实测**:`docker stop forge-build-nacos-1` → 再解析一次 → 断言走缓存仍出配置(不报错);
  起回来恢复。

- [ ] **Step 4: 记录证据**(类似 F 切片:把"注册→设 refs→解析出的有效 mcp_config + 降级走缓存"贴进
  plan 的 DoD 勾选)。

- [ ] **Step 5: Commit** — `git commit -am "test(nacos): creds-free e2e — register->refs->resolve + degradation"`

---

### Task 4.2: seed 一个示例 MCP server

**Files:**
- Modify: `server/cmd/multica`(若有 seed 子命令)或 `scripts/seed-mcp.sh`

- [ ] **Step 1**:往 `shared` namespace seed 一个真实可用的示例(比如把你在用的 `voc-openapi` 编目:
  transport=stdio/sse、env_keys 列出其密钥 KEY 名、published)。**只 seed shape,不 seed 密钥值**。
- [ ] **Step 2**:`go build` / 跑 seed,确认目录里能看到。
- [ ] **Step 3: Commit** — `git commit -am "chore(mcp): seed sample MCP server into shared namespace"`

---

### Task 4.3: 文档 / 记忆

**Files:**
- Modify: `docs/forge/specs/nacos-integration/README.md`(N0+N1 状态 → 已实现;贴验收证据链接)
- Modify: `docs/forge/specs/nacos-integration/N0-N1-mcp-registry.md`(把"待深化"里实测出的 REST schema 决议补上)
- Modify: 项目记忆 `memory/`(加一条 Nacos 接入条目:N0+N1 done,做法 A,接口优先 + secret 分离 + 降级)

- [ ] **Step 1**:更新上述文档,**不写明文凭证**(占位符)。
- [ ] **Step 2: Commit** — `git commit -m "docs(nacos): N0+N1 done — update spec status + memory"`

---

### Task 4.4: 全量门禁

- [ ] **Step 1: `make check`**(typecheck + 单测 + Go 测 + e2e)。
- [ ] **Step 2**:有红就修到全绿。
- [ ] **Step 3**:`finishing-a-development-branch` 收尾(合并/PR 由你定)。

---

## DoD 回填(实测后勾)

- [ ] Nacos 3.x 起得来、`REST.md` 记录实测接口
- [ ] `agent.mcp_refs` 迁移可逆 + sqlc
- [ ] `mcpresolve` 全单测绿(stdio/remote/skip/override/missing-secret/降级)
- [ ] `/api/mcp-registry/*` + 鉴权 + 单测;claim 钩子(refs 空 = 原行为;daemon 零改动)
- [ ] 前端目录 + 选择器 + zod;三包 typecheck 绿
- [ ] 绕凭证 e2e:注册→设 refs→解析出有效 `mcp_config`(secret 注入、tag→版本);停 Nacos→走缓存
