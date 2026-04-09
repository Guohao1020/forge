# Agent Base Loop — SCOPE REDUCTION 执行计划

**Generated:** 2026-04-07 by `/plan-ceo-review`
**Branch:** main
**Mode:** SCOPE_REDUCTION
**Baseline commit:** 30ec4aa
**Estimated effort:** 人 ~5 天 / CC ~3-4h
**Reference:** 完整 review 见 conversation log；CEO plan 在
`~/.gstack/projects/voc-shulex-forge/ceo-plans/2026-04-06-agent-terminal.md`

---

## Why this exists

12 commits 的 Agent Terminal Variant B 已经提交，但 base loop 从未真正端到端跑过：

- **Stream 4 commit 只改了一半。** `forge-core/internal/module/agent/service.go`
  POST 给 ai-worker 的 `/api/run`，但 ai-worker 的 Dockerfile 还在跑
  `python -B -m src.worker`（旧 Temporal worker），FastAPI 入口
  `ai-worker/src/api_server.py` 从未被 uvicorn 启动过。
- **ai-worker 容器无 ports 暴露。** `docker-compose.dev.yml` 里 ai-worker 没写
  `ports: ["8090:8090"]`，host 上的 forge-core 即使重启也连不到。
- **Handler.Chat 不持久化 user_message** —— ai-worker 失败时用户消息丢失。
- **cross-tenant session 漏洞** —— Handler.Chat / Handler.Stream 都不校验
  session 归属，A 用户可以读 B 用户的 SSE 流。
- **Stream 4c 是死代码** —— `pair_pipeline.py` 没调用 `detect_language()`，
  BuildVerifyHook 硬编码 `go build`。
- **7 个 src/agents/\*.py 孤儿** —— 旧 Temporal 时代的 agent 文件，
  cross-model learning（confidence 9/10）已经在 2026-04-06 标注过。

**Goal of this PR:** 一次真实的 LLM → coder → BuildVerify → reviewer → fix
端到端跑通，并以 `pytest -m e2e` 形式固化为回归测试。**只有这一件事**。

任何不在这条关键路径上的工作都推到下一轮（见 §"NOT in scope"）。

---

## Hard rules

1. **每个 TASK 是独立 commit**，单独可 revert。
2. **GATE 不通过就停。** TASK 4 不通过别动 5；TASK 11 不通过别动 12。
3. **TASK 11 (pytest -m e2e 通过) 是这次 PR 的成功标准。** 没通过就不能合。
4. **不要在这次 PR 里加 CEO plan 的 expansion 项**（marketplace、cost UI、
   diff view、multi-agent visibility、live compile streaming）。那是 base loop
   验证通过后下一轮的事。
5. **TASK 6 (cross-tenant check) 不能跳** —— 是数据隐私级别的 critical gap。

---

## TASK 清单（按依赖顺序）

### TASK 1 — 改 ai-worker/Dockerfile：同时跑 worker + uvicorn

**File:** `ai-worker/Dockerfile`

CMD 当前是 `python -B -m src.worker`（只跑 Temporal worker）。需要让容器同时
启动 FastAPI server (`uvicorn src.api_server:app --host 0.0.0.0 --port 8090`)
和 Temporal worker。

**Approach options:**
- (A) supervisord — 标准做法，需要装 supervisor
- (B) honcho/foreman + Procfile — Python 生态友好
- (C) shell wrapper `start.sh` 用 `&` 后台启动两个进程，trap SIGTERM

**Recommendation:** (C) shell wrapper —— 零依赖，Dockerfile 改动最小，
SIGTERM 正确传播保证 docker stop 能干净退出。supervisord 装完镜像会大 ~30MB。

**Acceptance:** 容器启动后 `docker logs forge-ai-worker` 同时看到：
- `INFO: Uvicorn running on http://0.0.0.0:8090`
- `INFO: AI Worker started. Waiting for activities...`

---

### TASK 2 — 改 docker-compose.dev.yml：暴露 ai-worker:8090

**File:** `docker-compose.dev.yml`

ai-worker service 加：
```yaml
ports:
  - "8090:8090"
```

**Acceptance:** `docker compose -f docker-compose.dev.yml config | grep -A 2 ai-worker:` 包含 ports 段。

---

### TASK 3 — 重建 + recreate ai-worker 容器

```bash
cd /d/shulex_work/forge
docker compose -f docker-compose.dev.yml build ai-worker
docker compose -f docker-compose.dev.yml up -d --force-recreate ai-worker
```

**Note:** 上一轮 build 已经把 asyncpg 0.31.0 安装进镜像了（耗时 ~12 分钟，
pip install 724s）。这次只需要 Dockerfile 改动重新打层，应该 <30s。

---

### TASK 4 — 🚪 GATE: ai-worker /health 在 host 8090 可达

```bash
curl -sS http://localhost:8090/health
```

**期望输出:** `{"status":"ok"}` 或类似 200 响应。

**如果失败：**
- 检查 `docker logs forge-ai-worker` 看 uvicorn 是否真的起来了
- 检查 `docker port forge-ai-worker` 是否显示 8090 映射
- 不通过不能继续 TASK 5

---

### TASK 5 — Handler.Chat 持久化 user_message

**File:** `forge-core/internal/module/agent/handler.go` line 75-95

当前实现：
```go
func (h *Handler) Chat(c *gin.Context) {
    // ... parse req ...
    resp, err := h.service.SubmitMessage(c.Request.Context(), projectID, req)
    // ...
}
```

需要改成：**先**写 user_message 到 PG，**再**调 ai-worker。如果 PG 写失败
直接返 500（用户消息丢失是更严重的问题）；如果 ai-worker 失败仍返 502 但
PG 已经有记录，下次 sidebar 加载历史能看到。

```go
// 伪代码
if err := h.repo.AppendMessage(ctx, AppendMessageInput{
    SessionID: req.SessionID,
    Role:      "user",
    Content:   req.Message,
}); err != nil {
    return 500
}
resp, err := h.service.SubmitMessage(...)
```

**Test:** `forge-core/internal/module/agent/handler_test.go` 加 1 个测试：
ai-worker mock 返 502，验证 PG 里 user_message 仍然存在。

---

### TASK 6 — Handler.Chat + Handler.Stream tenant ownership check

**File:** `forge-core/internal/module/agent/handler.go`

**Critical gap G2/G3 - 跨租户越权漏洞。**

当前 handler 只验证 `project_id` 在路径里，不验证 `session_id` 是否属于
当前用户。攻击：B 用户构造 A 用户的 session_id，POST 到自己 project 的
`/agent/chat`，如果 session 校验只看 project 不看 owner，可能能往 A 的
session 写消息。Handler.Stream 同样问题，更严重 —— 可以**读**别人的 SSE。

需要在 Chat 和 Stream 里都加：
```go
session, err := h.repo.GetSession(ctx, req.SessionID)
if err != nil { return 404 }
if session.TenantID != claims.TenantID || session.CreatedBy != claims.UserID {
    return 403  // 不要返 404 暴露存在性
}
```

**Test:** handler_test.go 加 2 个测试：
- Chat：A 用户带 B 的 session_id → 403
- Stream：A 用户订阅 B 的 session → 403

---

### TASK 7 — pair_pipeline.py 接入 Stream 4c language detection

**File:** `ai-worker/src/openharness/engine/pair_pipeline.py`

在 pipeline 入口（`run` 方法或 `__init__`）调 `detect_language(project_dir)`，
把返回的 `build_command` 写进 context dict 传给后续 hook。

```python
# 伪代码
from src.skills.language_profiles import detect_language

class PairPipeline:
    async def run(self, project_dir: str, ...):
        lang_profile = detect_language(project_dir, self.profiles)
        context = {
            "build_command": lang_profile.build_command,
            "language": lang_profile.name,
            ...
        }
        # 后续 BuildVerifyHook 从 context 读
```

**Test:** `ai-worker/tests/test_pair_pipeline.py` 加 1 个测试：
mock detect_language 返回 `{name: "go", build_command: "go build ./..."}`，
验证 context 正确填充。

---

### TASK 8 — BuildVerifyHook 读 context.build_command + go.mod 校验

**File:** `ai-worker/src/openharness/hooks/builtin/build_verify_hook.py`

两个改动：
1. 读 `context.get("build_command", "go build ./...")`，不再硬编码。
2. 在 run go build 之前先校验 `os.path.exists(os.path.join(workspace, "go.mod"))`，
   不存在就发 `BuildPreflightError` 事件，message: "no go.mod found in
   {workspace}, language detection returned {context.language}".

**Test:** 加 2 个测试：
- workspace 无 go.mod → BuildPreflightError，不调用 subprocess
- context.build_command = "cargo build" → 实际跑 cargo（mock subprocess）

---

### TASK 9 — 🚪 GATE: forge-core 重新构建 + 你手动重启

```bash
cd /d/shulex_work/forge/forge-core
go build -o forge-core-new.exe ./cmd/forge-core
```

**然后 Harvey 在他的 IDE / terminal 里手动重启 forge-core。**（PID 39176
现在还在跑旧二进制；CC 不应该 kill host 进程，因为不知道 launcher 怎么 spawn 的。）

**Acceptance:**
```bash
curl -sS -o /dev/null -w "%{http_code}\n" http://localhost:8080/api/projects/1/agent/sessions
# 应该返 401 而不是 404
```

---

### TASK 10 — 创建 e2e 烟测脚本

**File:** `ai-worker/tests/e2e/test_pair_pipeline_real_llm.py`

```python
import os
import pytest
from src.openharness.engine.pair_pipeline import PairPipeline

pytestmark = pytest.mark.e2e

@pytest.mark.asyncio
async def test_real_llm_coder_build_reviewer_loop():
    """端到端：真实 DASHSCOPE 调用 → coder 生成 Go 代码 →
    BuildVerify 实际 go build → reviewer 评审 → 通过."""

    assert os.environ.get("DASHSCOPE_API_KEY"), "set DASHSCOPE_API_KEY"

    pipeline = PairPipeline(
        model="qwen-coder-turbo",  # 或 qwen-max
        max_fix_iterations=3,
    )

    # self-hosting：让 AI 改一个 forge-core 的小 helper
    project_dir = "D:/shulex_work/forge/forge-core"

    result = await pipeline.run(
        project_dir=project_dir,
        prompt="在 internal/util 包里加一个 IsEven(n int) bool 函数,带单元测试",
    )

    assert result.status == "success"
    assert result.build_passed is True
    assert result.iterations <= 3
```

**File:** `ai-worker/pytest.ini` (如果不存在就创建)
```ini
[pytest]
markers =
    e2e: end-to-end tests that hit real external services (LLM, network)
addopts = -m "not e2e"
```

CI 默认跳过；本地手动 `pytest -m e2e ai-worker/tests/e2e/`。

---

### TASK 11 — 🚪 GATE: 跑通 e2e 烟测

```bash
cd /d/shulex_work/forge/ai-worker
pytest -m e2e tests/e2e/test_pair_pipeline_real_llm.py -v
```

**这是整个 reduction 的成功标准。** 必须看到：
1. DASHSCOPE 真实返回 coder 响应
2. fenced block 被解析出 Go 文件
3. BuildVerifyHook 真的调用了 `go build`
4. reviewer 真实评审
5. 最终 result.status == "success"

**如果失败：**
- 看 ai-worker 容器 logs 找具体崩在哪一步
- 看 SSE 事件流（curl 一下 stream endpoint）确认事件类型对不对
- 不通过**不能合 PR**，直接修到通过为止
- 这是最容易暴露 base loop 隐藏 bug 的一步，预留最多调试时间

**典型可能踩的坑：**
- LLM router 没默认 DASHSCOPE，得显式指定
- workspace 路径在 host vs container 视角不一样
- BuildVerify 在容器里跑找不到 host 的 go binary
- DASHSCOPE 返回包含 Markdown 反引号但 fenced block 解析挑剔

---

### TASK 12 — 删除 7 个孤儿 src/agents/*.py

```bash
cd /d/shulex_work/forge
git rm ai-worker/src/agents/analyst.py
git rm ai-worker/src/agents/coder.py
git rm ai-worker/src/agents/planner.py
git rm ai-worker/src/agents/profiler.py
git rm ai-worker/src/agents/reviewer.py
git rm ai-worker/src/agents/test_writer.py
# 注意：base.py 和 __init__.py 先 grep 一下有没有别处 import
grep -rn "from src.agents" ai-worker/src/ ai-worker/tests/ | grep -v __pycache__
```

如果 grep 出 import，先把那些文件的 import 改掉/删掉再 git rm base.py 和 __init__.py。

**Reference:** cross-model learning `unify-agent-loops` (confidence 9/10,
2026-04-06) 已经标注过这些文件应该删。

---

### TASK 13 — 写 docs/technical-design.md 新章节

**File:** `docs/technical-design.md`

加 §"Agent Module Runtime"，内容：

- ai-worker 现在跑两个进程：FastAPI (port 8090) + Temporal worker
- Handler.Chat → POST ai-worker /api/run 同步触发
- 双存储：先 PG (Handler.Chat 持久化 user_message) → ai-worker 内部
  Redis Stream + asyncpg dual write
- Stream 4c language detection 接入点 + BuildVerifyHook context 协议
- e2e 烟测在 `tests/e2e/`，怎么跑

---

### TASK 14 — 更新 TODOS.md

**File:** `TODOS.md`

加 1 个 P0 ticket：

```markdown
## Security (P0 — Marketplace 前置门禁)

- [ ] **Workspace 沙箱隔离 — Marketplace P0 前置门禁**
  AI 生成的代码会在 BuildVerifyHook 里被 `go build` 编译。Go 编译过程
  本身会执行代码（init() 函数 + go:generate 指令），意味着只要 LLM 输出
  包含恶意片段，宿主机就被 RCE。当前 workspace 是宿主机进程的子目录,
  无任何隔离。Marketplace（用户互相分享 skills）会让这变成 supply chain
  attack vector。**必须**在 marketplace 之前解决：workspace 跑在
  docker run / firejail / gVisor 沙箱里。
  Effort: M (人 ~3 天 / CC ~3h)。Priority: P0 但 base loop 之后开工。
```

也把已有的紫色品牌清理列表整理一下（已知 7 个文件，见 conversation log
checkpoint）。

---

### TASK 15 — Commit 一组 + push

```bash
git add ai-worker/Dockerfile docker-compose.dev.yml
git commit -m "fix(ai-worker): start FastAPI alongside Temporal worker, expose 8090"

git add forge-core/internal/module/agent/handler.go forge-core/internal/module/agent/handler_test.go
git commit -m "fix(agent): persist user_message + tenant ownership check (G1/G2/G3)"

git add ai-worker/src/openharness/engine/pair_pipeline.py ai-worker/src/openharness/hooks/builtin/build_verify_hook.py ai-worker/tests/test_pair_pipeline.py
git commit -m "feat(agent): wire Stream 4c language detection into pair pipeline"

git add ai-worker/tests/e2e/ ai-worker/pytest.ini
git commit -m "test(agent): real-LLM e2e smoke test for pair pipeline"

git rm ai-worker/src/agents/analyst.py # ... etc
git commit -m "chore(agent): remove orphaned pre-OpenHarness agent files"

git add docs/technical-design.md TODOS.md
git commit -m "docs: agent module runtime + workspace sandbox P0"

git push origin main
```

---

## NOT in scope（这次 PR 明确不做）

| # | 项 | 为什么推迟 | 推到 |
|---|----|-----------|------|
| 1 | Multi-agent 可见性 UI | 等 base loop 跑通才知道 SSE 事件结构 | 下一轮 |
| 2 | Live compile streaming UI | 等 BuildVerify 真实输出格式确定 | 下一轮 |
| 3 | Cost counter UI | 等真实 DASHSCOPE 调用产生 cost 数据 | 下一轮 |
| 4 | Diff view UI | 等修复循环真实跑过 | 下一轮 |
| 5 | Marketplace | 等 workspace 安全隔离 + 至少 1 个真实用户 | 下下轮 |
| 6 | Skills/Projects/Settings 应用 DESIGN.md | 重要但不在关键路径 | 单独 PR |
| 7 | 紫色品牌残留清理 | 7 文件 grep，1h 单独 PR | 单独 PR |
| 8 | Redis consumer group 改造 | 健壮性优化，不阻塞验证 | 等真实负载 |
| 9 | Agent Terminal 接 engine.tasks | 耦合优化 | 等需求驱动 |
| 10 | Workspace 安全隔离（chroot/docker） | **P0**，但写进 TODOS 单独 PR | Marketplace 前置 |
| 11 | LLM prompt injection 防御 | 等真实 prompt 流稳定 | 单独安全 PR |
| 12 | TaskSwitcher 嵌套 button 重构 | a11y 改进，不阻塞 | 单独 PR |

---

## Failure Modes Registry（必修项）

```
+========================================================================================+
| CODEPATH                | FAILURE MODE                | RESCUED | TEST | USER  | LOG  |
+-------------------------+-----------------------------+---------+------+-------+------+
| Handler.Chat            | cross-tenant session        |    N    |   N  | leak  |  N   | 🔴 TASK 6
| Handler.Stream          | cross-tenant session        |    N    |   N  | leak  |  N   | 🔴 TASK 6
| Handler.Chat            | user_message lost on 502    |    N    |   N  | gone  |  N   | 🔴 TASK 5
| BuildVerifyHook         | no go.mod                   |    N    |   N  | bad   |  N   | ⚠ TASK 8
| BuildVerifyHook         | hardcoded go build          |    N    |   N  | bad   |  N   | ⚠ TASK 7+8
| pair_pipeline           | empty coder response        |    N    |   N  | empty |  N   | ⚠ 推到下一轮
| pair_pipeline           | infinite fix loop           |    ?    |   N  | hang  |  ?   | ⚠ TASK 10 测覆盖
| api_server.run          | LLM rate limit 429          |    ?    |   N  | error |  ?   | ⚠ 推到下一轮
| api_server.run          | LLM refusal                 |    N    |   N  | empty |  N   | ⚠ 推到下一轮
| BuildVerifyHook         | LLM injects malicious code  |    N    |   N  | RCE   |  N   | 🚨 TASK 14 P0
+-------------------------+-----------------------------+---------+------+-------+------+
```

---

## Success Criteria（这个 PR 怎么算成）

1. ✅ TASK 4 通过：`curl http://localhost:8090/health` 返 200
2. ✅ TASK 9 通过：`curl /api/projects/1/agent/sessions` 返 401（不再 404）
3. ✅ TASK 11 通过：`pytest -m e2e` 端到端跑通真实 DASHSCOPE 调用
4. ✅ Handler.Chat / Handler.Stream 都拒绝 cross-tenant session
5. ✅ Handler.Chat 在 ai-worker 502 时 user_message 仍在 PG
6. ✅ Stream 4c 不再是死代码（pair_pipeline 真实调用 detect_language）
7. ✅ 7 个孤儿 agent 文件删除
8. ✅ TODOS.md 包含 workspace sandbox P0 ticket
9. ✅ 所有现有测试仍然通过（95 单元 → 104 单元 + 1 e2e）

---

## What this PR is NOT solving

- 不解决 marketplace 商业模式
- 不解决多 agent 可见性的 UX 问题
- 不解决 cost transparency
- 不解决 workspace RCE（写进 TODOS 但本 PR 不修）
- 不解决 prompt injection
- 不解决 Skills/Projects/Settings 页面的 Variant B 落地

这些都是真实问题，但每一个都依赖 base loop 工作。先证明能跑，再加功能。

---

## Reference

- Conversation log（包含完整 review）：上一次 `/plan-ceo-review` 输出
- 上一轮 CEO plan: `~/.gstack/projects/voc-shulex-forge/ceo-plans/2026-04-06-agent-terminal.md`
- Variant B mockup: `~/.gstack/projects/voc-shulex-forge/designs/agent-terminal-shotgun-20260406/variant-B-dense.html`
- DESIGN.md: `docs/DESIGN.md`
- 上次 checkpoint: `~/.gstack/projects/voc-shulex-forge/checkpoints/20260407-180858-agent-terminal-variant-b-delivered.md`
