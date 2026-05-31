# S6 — AI Worker Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Deliver a complete AI-driven code generation pipeline: natural language requirement -> multi-turn conversation -> task planning -> code generation -> AI review -> fix/retry loop, with real-time progress and code preview.

**Architecture:** Python Temporal Worker (4 AI agents + multi-model router) receives cross-language activity calls from Go workflow. Go adds conversation API. Next.js adds Chat UI + code preview.

**Tech Stack:** Python 3.12, Temporal SDK (Python + Go), Anthropic/OpenAI/DashScope/DeepSeek SDKs, Go 1.22 + Gin + pgx, Next.js 15 + React + shadcn/ui

**Dependencies:** S4 (Temporal + Tasks), S5 (Specs Center)

**Design Spec:** `docs/plans/S6-ai-worker-design.md`

---

## File Structure

### Python AI Worker (NEW)

```
ai-worker/
├── pyproject.toml
├── requirements.txt
├── .env.example
├── Dockerfile
├── src/
│   ├── __init__.py
│   ├── config.py                     # Pydantic Settings
│   ├── worker.py                     # Temporal Worker entry
│   ├── models/
│   │   ├── __init__.py
│   │   ├── router.py                # Multi-model router + circuit breaker
│   │   └── client.py                # LLM client wrappers (4 providers)
│   ├── context/
│   │   ├── __init__.py
│   │   └── builder.py               # Context builder (4-layer prompt)
│   ├── agents/
│   │   ├── __init__.py
│   │   ├── base.py                   # Agent base class
│   │   ├── analyst.py                # Requirement analysis
│   │   ├── planner.py                # Task planning
│   │   ├── coder.py                  # Code generation
│   │   └── reviewer.py              # Code review
│   └── activities/
│       ├── __init__.py
│       ├── analyze.py
│       ├── plan.py
│       ├── generate.py
│       └── review.py
└── tests/
    ├── __init__.py
    ├── test_router.py
    ├── test_context_builder.py
    └── test_agents.py
```

### Go Changes

```
forge-core/
├── migrations/007_conversations.sql
├── internal/module/conversation/
│   ├── model.go
│   ├── repository.go
│   ├── service.go
│   └── handler.go
├── internal/temporal/workflow/task_workflow.go   (MODIFY)
├── internal/temporal/activity/task_activities.go (MODIFY)
├── internal/temporal/client.go                   (MODIFY)
├── internal/temporal/worker.go                   (MODIFY)
├── internal/router/router.go                     (MODIFY)
├── internal/config/config.go                     (MODIFY)
└── cmd/forge-core/main.go                        (MODIFY)
```

### Frontend Changes

```
forge-portal/
├── lib/conversation.ts
├── components/chat/
│   ├── chat-panel.tsx
│   ├── message-bubble.tsx
│   └── confirmation-card.tsx
├── components/code-preview/
│   ├── code-preview-panel.tsx
│   ├── file-tree.tsx
│   └── code-viewer.tsx
└── app/(dashboard)/projects/[id]/tasks/
    ├── new/page.tsx
    └── [taskId]/page.tsx              (MODIFY)
```

---

## Task 1: Python 项目骨架 + 配置

**Files:**
- Create: `ai-worker/pyproject.toml`
- Create: `ai-worker/requirements.txt`
- Create: `ai-worker/.env.example`
- Create: `ai-worker/src/__init__.py`
- Create: `ai-worker/src/config.py`
- Create: `ai-worker/src/models/__init__.py`
- Create: `ai-worker/src/context/__init__.py`
- Create: `ai-worker/src/agents/__init__.py`
- Create: `ai-worker/src/activities/__init__.py`
- Create: `ai-worker/tests/__init__.py`

- [ ] **Step 1: 创建目录结构**

```bash
mkdir -p ai-worker/src/models ai-worker/src/context ai-worker/src/agents ai-worker/src/activities ai-worker/tests
touch ai-worker/src/__init__.py ai-worker/src/models/__init__.py ai-worker/src/context/__init__.py ai-worker/src/agents/__init__.py ai-worker/src/activities/__init__.py ai-worker/tests/__init__.py
```

- [ ] **Step 2: 创建 pyproject.toml**

`ai-worker/pyproject.toml`:

```toml
[project]
name = "forge-ai-worker"
version = "0.1.0"
description = "Forge AI Worker — LLM-powered code generation via Temporal"
requires-python = ">=3.12"

[tool.pytest.ini_options]
testpaths = ["tests"]
asyncio_mode = "auto"
```

- [ ] **Step 3: 创建 requirements.txt**

`ai-worker/requirements.txt`:

```
# Temporal
temporalio==1.9.0

# LLM SDKs
anthropic>=0.42.0
openai>=1.60.0
dashscope>=1.20.0

# HTTP
httpx>=0.28.0

# Config
pydantic-settings>=2.7.0

# Testing
pytest>=8.0.0
pytest-asyncio>=0.24.0
```

- [ ] **Step 4: 创建 config.py**

`ai-worker/src/config.py`:

```python
from pydantic_settings import BaseSettings


class Settings(BaseSettings):
    # Temporal
    temporal_host: str = "localhost:7233"
    temporal_namespace: str = "default"
    task_queue: str = "ai-worker"

    # LLM API Keys
    anthropic_api_key: str = ""
    openai_api_key: str = ""
    dashscope_api_key: str = ""
    deepseek_api_key: str = ""

    # Forge Core API
    forge_api_url: str = "http://localhost:8080"
    forge_api_token: str = ""

    # Model defaults
    default_model: str = "claude-sonnet-4-20250514"
    default_provider: str = "anthropic"

    model_config = {"env_file": ".env", "env_file_encoding": "utf-8"}


settings = Settings()
```

- [ ] **Step 5: 创建 .env.example**

`ai-worker/.env.example`:

```env
TEMPORAL_HOST=localhost:7233
TASK_QUEUE=ai-worker
ANTHROPIC_API_KEY=
OPENAI_API_KEY=
DASHSCOPE_API_KEY=
DEEPSEEK_API_KEY=
FORGE_API_URL=http://localhost:8080
FORGE_API_TOKEN=
```

- [ ] **Step 6: 验证 Python 环境**

```bash
cd ai-worker && pip install -r requirements.txt
python -c "from src.config import settings; print(f'OK: temporal={settings.temporal_host}')"
```

预期: `OK: temporal=localhost:7233`

- [ ] **Step 7: Commit**

```bash
git add ai-worker/
git commit -m "feat(s6): add Python AI Worker project skeleton and config"
```

---

## Task 2: 多模型路由器 + LLM 客户端

**Files:**
- Create: `ai-worker/src/models/client.py`
- Create: `ai-worker/src/models/router.py`
- Create: `ai-worker/tests/test_router.py`

- [ ] **Step 1: 创建 LLM 客户端封装**

`ai-worker/src/models/client.py`:

封装 4 个 provider 的调用函数，每个返回统一的 `LLMResponse` dataclass：

```python
@dataclass
class LLMResponse:
    content: str
    model: str
    provider: str
    input_tokens: int
    output_tokens: int
    latency_ms: int
```

4 个 async 函数：
- `call_anthropic(api_key, model, system, messages) -> LLMResponse` — 使用 `anthropic.AsyncAnthropic`
- `call_openai(api_key, model, system, messages) -> LLMResponse` — 使用 `openai.AsyncOpenAI`
- `call_dashscope(api_key, model, system, messages) -> LLMResponse` — 使用 OpenAI SDK + DashScope base_url
- `call_deepseek(api_key, model, system, messages) -> LLMResponse` — 使用 OpenAI SDK + DeepSeek base_url

每个函数记录 latency_ms（time.monotonic 计时）。

导出 `PROVIDER_CALLERS` dispatch dict。

- [ ] **Step 2: 创建多模型路由器**

`ai-worker/src/models/router.py`:

核心类：
- `Purpose` 枚举: ANALYZE, PLAN, GENERATE, REVIEW
- `CircuitBreaker` dataclass: failures/threshold=3/window=30s/recovery=60s
- `ModelRouter` 类:
  - `ROUTING_RULES` 按 Purpose 定义降级链（4 级）
  - `chat(system, messages, purpose)` 按链顺序尝试，跳过熔断的模型
  - 成功 → record_success，失败 → record_failure，全部失败 → raise RuntimeError

- [ ] **Step 3: 写路由器测试**

`ai-worker/tests/test_router.py`:

4 个测试：
1. `test_circuit_breaker_opens_after_threshold` — 3 次失败后熔断
2. `test_circuit_breaker_resets_on_success` — 成功后重置
3. `test_router_returns_first_successful_model` — mock 第一个成功即返回
4. `test_router_falls_back_on_failure` — 第一个失败，fallback 到第二个
5. `test_router_raises_when_all_fail` — 全部失败抛异常

- [ ] **Step 4: 运行测试**

```bash
cd ai-worker && python -m pytest tests/test_router.py -v
```

预期: 5 tests pass

- [ ] **Step 5: Commit**

```bash
git add ai-worker/src/models/ ai-worker/tests/test_router.py
git commit -m "feat(s6): add multi-model router with 4-provider fallback and circuit breaker"
```

---

## Task 3: 上下文构建器

**Files:**
- Create: `ai-worker/src/context/builder.py`
- Create: `ai-worker/tests/test_context_builder.py`

- [ ] **Step 1: 创建上下文构建器**

`ai-worker/src/context/builder.py`:

`ProjectContext` dataclass：
- project_name, project_description, tech_stack
- coding_standards: list[str] — 从 /api/specs/effective/{projectId} 获取
- review_rules: list[dict]
- prompt_template_system, prompt_template_user — 从 /api/specs/prompts?purpose=X 获取
- conversation_history: list[dict]
- `to_system_prompt()` 方法：四层叠加组装

`ContextBuilder` 类：
- 使用 httpx.AsyncClient 调用 forge-core API（带 Bearer token）
- `build(project_id, purpose, conversation_history)` 方法
- 按优先级加载：Prompt 模板 > 编码规范 > 项目画像 > 对话历史
- 容错：任一 API 失败只 log warning，不阻断

- [ ] **Step 2: 写测试**

`ai-worker/tests/test_context_builder.py`:

3 个测试（纯单元测试，不调真实 API）：
1. `test_system_prompt_assembly` — 有完整上下文时组装正确
2. `test_system_prompt_empty_context` — 空上下文返回空字符串
3. `test_system_prompt_standards_only` — 只有编码规范时正确包含

- [ ] **Step 3: 运行测试**

```bash
cd ai-worker && python -m pytest tests/test_context_builder.py -v
```

- [ ] **Step 4: Commit**

```bash
git add ai-worker/src/context/ ai-worker/tests/test_context_builder.py
git commit -m "feat(s6): add context builder with 4-layer prompt assembly"
```

---

## Task 4: 4 个 AI Agent

**Files:**
- Create: `ai-worker/src/agents/base.py`
- Create: `ai-worker/src/agents/analyst.py`
- Create: `ai-worker/src/agents/planner.py`
- Create: `ai-worker/src/agents/coder.py`
- Create: `ai-worker/src/agents/reviewer.py`
- Create: `ai-worker/tests/test_agents.py`

- [ ] **Step 1: 创建 Agent 基类**

`ai-worker/src/agents/base.py`:

`AgentResult` dataclass: content, structured(dict), tokens_used, model, provider, latency_ms

`BaseAgent` 类：
- `__init__(router: ModelRouter)`
- `purpose: Purpose` 类属性（子类覆盖）
- `_build_system_prompt(context)` → 子类覆盖
- `_build_messages(user_input, context)` → 拼接对话历史 + 当前输入
- `run(user_input, context) -> AgentResult` → 调用 router.chat + 解析 JSON
- `_parse_json(text) -> dict` → 三级解析：直接 → 从 ````json` 代码块 → 从 `{...}` 提取

- [ ] **Step 2: 创建 Analyst Agent**

`ai-worker/src/agents/analyst.py`:

- 继承 BaseAgent，purpose = Purpose.ANALYZE
- System prompt 定义 AI 为"资深产品分析师和系统架构师"
- 输出格式：`{"status": "clarify|confirmed", "questions": [...], "summary": "..."}`
- 注入项目上下文到 system prompt

- [ ] **Step 3: 创建 Planner Agent**

`ai-worker/src/agents/planner.py`:

- 继承 BaseAgent，purpose = Purpose.PLAN
- System prompt 定义 AI 为"高级软件架构师"
- 输出格式：`{"title", "tasks": [{order, title, files, type}], "risk_level", "risk_factors"}`

- [ ] **Step 4: 创建 Coder Agent**

`ai-worker/src/agents/coder.py`:

- 继承 BaseAgent，purpose = Purpose.GENERATE
- System prompt 强调**必须严格遵守编码规范**
- 输出格式：`{"files": [{path, content, action, language}], "commit_message", ...}`

- [ ] **Step 5: 创建 Reviewer Agent**

`ai-worker/src/agents/reviewer.py`:

- 继承 BaseAgent，purpose = Purpose.REVIEW
- System prompt 定义 5 个 Review 维度
- 注入 review_rules 到 system prompt
- 输出格式：`{"passed", "score", "findings": [{severity, file, line, message, suggestion}], "fix_instructions"}`
- 通过阈值：score >= 80 且零 ERROR findings

- [ ] **Step 6: 写 Agent 测试**

`ai-worker/tests/test_agents.py`:

6 个测试：
1. `test_parse_json_direct` — 直接 JSON 解析
2. `test_parse_json_from_code_block` — 从 markdown 代码块提取
3. `test_parse_json_fallback_empty` — 非 JSON 返回空 dict
4. `test_analyst_system_prompt_includes_context` — Analyst prompt 包含项目上下文
5. `test_reviewer_injects_review_rules` — Reviewer prompt 注入 review 规则
6. `test_coder_system_prompt_has_standards` — Coder prompt 包含编码规范

- [ ] **Step 7: 运行全部 Python 测试**

```bash
cd ai-worker && python -m pytest tests/ -v
```

预期: 全部通过

- [ ] **Step 8: Commit**

```bash
git add ai-worker/src/agents/ ai-worker/tests/test_agents.py
git commit -m "feat(s6): add 4 AI agents — analyst, planner, coder, reviewer"
```

---

## Task 5: Temporal Activities + Worker 入口

**Files:**
- Create: `ai-worker/src/activities/analyze.py`
- Create: `ai-worker/src/activities/plan.py`
- Create: `ai-worker/src/activities/generate.py`
- Create: `ai-worker/src/activities/review.py`
- Create: `ai-worker/src/worker.py`

- [ ] **Step 1: 创建 analyze activity**

`ai-worker/src/activities/analyze.py`:

- `AnalyzeInput` dataclass: project_id, task_id, requirement, conversation_history
- `AnalyzeOutput` dataclass: status, content, metadata, tokens_used, model, provider, latency_ms
- `@activity.defn(name="analyze_requirement")` 装饰器
- 内部：ContextBuilder.build → AnalystAgent.run → 返回 AnalyzeOutput

- [ ] **Step 2: 创建 plan activity**

`ai-worker/src/activities/plan.py`:

- `PlanInput`: project_id, task_id, requirement_summary
- `PlanOutput`: title, tasks, risk_level, risk_factors, tokens_used, model, provider, latency_ms
- `@activity.defn(name="plan_task")`

- [ ] **Step 3: 创建 generate activity**

`ai-worker/src/activities/generate.py`:

- `GenerateInput`: project_id, task_id, requirement_summary, task_plan, fix_instructions(optional)
- `GenerateOutput`: files, commit_message, files_changed, lines_added, lines_deleted, tokens_used, model, provider, latency_ms
- `@activity.defn(name="generate_code")`
- 如果有 fix_instructions，在 user prompt 中追加修复指令

- [ ] **Step 4: 创建 review activity**

`ai-worker/src/activities/review.py`:

- `ReviewInput`: project_id, task_id, files
- `ReviewOutput`: passed, score, findings, summary, fix_instructions, tokens_used, model, provider, latency_ms
- `@activity.defn(name="review_code")`

- [ ] **Step 5: 创建 Worker 入口**

`ai-worker/src/worker.py`:

```python
async def main():
    client = await Client.connect(settings.temporal_host, namespace=settings.temporal_namespace)
    worker = Worker(
        client,
        task_queue=settings.task_queue,
        activities=[
            analyze_requirement_activity,
            plan_task_activity,
            generate_code_activity,
            review_code_activity,
        ],
    )
    await worker.run()

if __name__ == "__main__":
    asyncio.run(main())
```

- [ ] **Step 6: 验证 Worker imports**

```bash
cd ai-worker && python -c "from src.worker import main; print('Worker imports OK')"
```

- [ ] **Step 7: Commit**

```bash
git add ai-worker/src/activities/ ai-worker/src/worker.py
git commit -m "feat(s6): add Temporal activities and worker entry point"
```

---

## Task 6: Go — DB 迁移 + 对话 API + 真实 Workflow

**Files:**
- Create: `forge-core/migrations/007_conversations.sql`
- Create: `forge-core/internal/module/conversation/model.go`
- Create: `forge-core/internal/module/conversation/repository.go`
- Create: `forge-core/internal/module/conversation/service.go`
- Create: `forge-core/internal/module/conversation/handler.go`
- Modify: `forge-core/internal/temporal/workflow/task_workflow.go`
- Modify: `forge-core/internal/temporal/activity/task_activities.go`
- Modify: `forge-core/internal/temporal/worker.go`
- Modify: `forge-core/internal/router/router.go`
- Modify: `forge-core/cmd/forge-core/main.go`

**重要**: 这是最大的 task。实现者必须先读取以下文件了解现有模式：
- `forge-core/internal/module/task/model.go` — struct 和 JSON tag 风格
- `forge-core/internal/module/task/repository.go` — pgx 查询模式
- `forge-core/internal/module/task/service.go` — 业务逻辑模式
- `forge-core/internal/module/task/handler.go` — Gin handler 模式
- `forge-core/internal/temporal/workflow/task_workflow.go` — 现有骨架 workflow
- `forge-core/internal/temporal/activity/task_activities.go` — 现有 activity 模式
- `forge-core/internal/router/router.go` — 路由注册模式
- `forge-core/cmd/forge-core/main.go` — 模块初始化模式

- [ ] **Step 1: 创建 DB 迁移**

`forge-core/migrations/007_conversations.sql`:

3 张新表：
- `engine.conversations`(id, task_id, role, content, metadata JSONB, tokens_used, created_at) + index
- `engine.model_calls`(id, tenant_id, task_id, step_type, model, provider, purpose, input_tokens, output_tokens, total_tokens, cost_cents, latency_ms, status, error_code, created_at) + indexes
- `engine.review_results`(id, task_id, step_id, review_type, score, passed, findings JSONB, summary, created_at) + index

ALTER tasks 表新增: analysis JSONB, task_graph JSONB, risk_factors JSONB

- [ ] **Step 2: 创建 conversation model.go**

遵循 task/model.go 的 struct 风格：
- `Conversation{ID, TaskID, Role, Content, Metadata, TokensUsed, CreatedAt}`
- `SendMessageRequest{Content string binding:"required,min=1"}`

- [ ] **Step 3: 创建 conversation repository.go**

遵循 task/repository.go 的 pgx 模式：
- `Create(ctx, conv) error`
- `ListByTaskID(ctx, taskID) ([]*Conversation, error)` — ORDER BY created_at ASC
- `CreateModelCall(ctx, call) error`
- `CreateReviewResult(ctx, result) error`

- [ ] **Step 4: 创建 conversation service.go**

```go
type Service struct {
    repo           *Repository
    taskRepo       *task.Repository
    temporalClient *temporal.Client
}
```

方法：
- `SendMessage(ctx, taskID, tenantID, userID, content)` → 保存用户消息 → 调用 analyze_requirement activity → 保存 AI 回复 → 如果 confirmed 更新 task.analysis
- `ConfirmPlan(ctx, taskID, tenantID)` → 校验状态 → 启动 TaskGenerationWorkflow → 保存 workflow_id
- `GetHistory(ctx, taskID)` → 查询对话列表

**关键**: SendMessage 调用 Temporal Activity 是**同步等待**（ExecuteActivity + Get），不是启动 Workflow。

- [ ] **Step 5: 创建 conversation handler.go**

3 个 endpoint：
- `SendMessage` POST — 绑定 JSON body，调用 svc.SendMessage
- `GetHistory` GET — 调用 svc.GetHistory
- `ConfirmPlan` POST — 调用 svc.ConfirmPlan

遵循 task/handler.go 的错误处理模式（response.OK / response.Fail）。

- [ ] **Step 6: 替换骨架 Workflow 为真实 Workflow**

修改 `forge-core/internal/temporal/workflow/task_workflow.go`:

用真实 AI activity 调用替换 mock sleep 循环：
1. Plan → ExecuteActivity("plan_task", queue="ai-worker", timeout=5min)
2. Generate → ExecuteActivity("generate_code", queue="ai-worker")
3. Review loop (max 3) → ExecuteActivity("review_code", queue="ai-worker")
4. Fix: review 失败时用 fix_instructions 重新 generate
5. Complete/Fail

**关键**: AI activities 用 `"ai-worker"` task queue（Python Worker），本地 status update 用默认 Go task queue。

- [ ] **Step 7: 添加新 activities**

在 `task_activities.go` 新增：
- `UpdateTaskAnalysis` — 更新 task.analysis JSONB
- `SaveGeneratedCode` — 保存生成代码到 task_steps.output
- `SaveReviewResult` — 保存 review 结果到 review_results 表

- [ ] **Step 8: 注册 activities 到 worker.go**

在 Temporal Worker 注册新的本地 activities。

- [ ] **Step 9: 注册对话路由到 router.go**

在 protected group 中添加：
```go
protected.POST("/projects/:id/tasks/:taskId/messages", deps.ConversationHandler.SendMessage)
protected.GET("/projects/:id/tasks/:taskId/messages", deps.ConversationHandler.GetHistory)
protected.POST("/projects/:id/tasks/:taskId/confirm", deps.ConversationHandler.ConfirmPlan)
```

- [ ] **Step 10: 在 main.go 中初始化 conversation 模块**

初始化 convRepo → convSvc → convHandler，添加到 router.Deps。

- [ ] **Step 11: 验证构建 + 迁移**

```bash
cd forge-core && go build ./cmd/forge-core
docker exec -i forge-postgres psql -U forge -d forge_main < forge-core/migrations/007_conversations.sql
docker exec forge-postgres psql -U forge -d forge_main -c "\dt engine.conversations; \dt engine.model_calls; \dt engine.review_results;"
```

- [ ] **Step 12: 运行 Go 测试**

```bash
cd forge-core && go test ./...
```

预期: 所有现有测试通过

- [ ] **Step 13: Commit**

```bash
git add forge-core/
git commit -m "feat(s6): add conversation API, model_calls tracking, and real Temporal workflow"
```

---

## Task 7: 前端 — Chat UI + 代码预览

**Files:**
- Create: `forge-portal/lib/conversation.ts`
- Create: `forge-portal/components/chat/chat-panel.tsx`
- Create: `forge-portal/components/chat/message-bubble.tsx`
- Create: `forge-portal/components/chat/confirmation-card.tsx`
- Create: `forge-portal/components/code-preview/code-preview-panel.tsx`
- Create: `forge-portal/components/code-preview/file-tree.tsx`
- Create: `forge-portal/components/code-preview/code-viewer.tsx`
- Create: `forge-portal/app/(dashboard)/projects/[id]/tasks/new/page.tsx`
- Modify: `forge-portal/app/(dashboard)/projects/[id]/tasks/[taskId]/page.tsx`

**重要**: 实现者必须先读取以下文件：
- `forge-portal/lib/api.ts` — API 调用模式
- `forge-portal/lib/specs.ts` — 类型定义模式
- `forge-portal/components/markdown-preview.tsx` — Markdown 渲染（复用）
- `forge-portal/app/(dashboard)/projects/[id]/tasks/[taskId]/page.tsx` — 现有任务详情页
- 遵循深空主题: bg-[#0A0A12], border-white/10, #8B5CF6 紫色

- [ ] **Step 1: 创建对话 API 客户端**

`forge-portal/lib/conversation.ts`:

类型：`Conversation`, `SendMessageResponse`, `ConfirmResponse`
函数：
- `sendMessage(projectId, taskId, content)` → POST /tasks/:taskId/messages
- `getHistory(projectId, taskId)` → GET /tasks/:taskId/messages
- `confirmPlan(projectId, taskId)` → POST /tasks/:taskId/confirm

- [ ] **Step 2: 创建 message-bubble 组件**

`forge-portal/components/chat/message-bubble.tsx`:

- 用户消息：右对齐，#8B5CF6/10 紫色背景
- AI 消息：左对齐，white/5 深灰背景，用 MarkdownPreview 渲染
- 系统消息：居中，小字灰色

- [ ] **Step 3: 创建 confirmation-card 组件**

`forge-portal/components/chat/confirmation-card.tsx`:

AI 确认需求后展示的卡片：
- 需求摘要（Markdown 渲染）
- 影响模块列表（badges）
- 风险等级 badge（LOW=绿, MEDIUM=黄, HIGH=红）
- 任务标题
- 三按钮：[确认执行] 紫色 / [修改需求] ghost / [取消] ghost red

- [ ] **Step 4: 创建 chat-panel 组件**

`forge-portal/components/chat/chat-panel.tsx`:

主对话容器：
- 消息列表（可滚动，新消息自动滚底）
- 底部输入区（textarea + 发送按钮）
- AI 思考中 loading 状态
- status=confirmed 时渲染 confirmation-card

- [ ] **Step 5: 创建需求对话页**

`forge-portal/app/(dashboard)/projects/[id]/tasks/new/page.tsx`:

全页对话 UI：
- 首次发送消息时创建 task（POST /tasks）
- 使用 ChatPanel 组件
- 确认后调 confirmPlan → 跳转到任务详情页

- [ ] **Step 6: 创建代码预览组件**

`forge-portal/components/code-preview/` 下 3 个组件：

**file-tree.tsx**: 左侧文件列表
- 可折叠目录结构
- 文件图标：✚ create（绿色）、✎ modify（黄色）
- 点击选中文件

**code-viewer.tsx**: 右侧代码展示
- `<pre>` + 行号
- Geist Mono 字体
- #0A0A12 深色背景

**code-preview-panel.tsx**: 左右分栏容器
- 左 30% FileTree + 右 70% CodeViewer
- 底部显示 commit message + 文件变更统计

- [ ] **Step 7: 任务详情页添加代码预览**

修改 `forge-portal/app/(dashboard)/projects/[id]/tasks/[taskId]/page.tsx`:

- GENERATING 步骤有 output 时，展示 CodePreviewPanel
- REVIEWING 步骤有 output 时，展示 review findings 列表：
  - 按文件分组
  - 每条 finding: severity badge + file + line + message + suggestion

- [ ] **Step 8: 验证前端构建**

```bash
cd forge-portal && npm run build
```

预期: 构建成功

- [ ] **Step 9: Commit**

```bash
git add forge-portal/
git commit -m "feat(s6): add Chat UI, confirmation card, and code preview components"
```

---

## Task 8: 集成 — Docker Compose + 端到端验证

**Files:**
- Create: `ai-worker/Dockerfile`
- Modify: `docker-compose.dev.yml`
- Modify: `.env.example`

- [ ] **Step 1: 创建 AI Worker Dockerfile**

`ai-worker/Dockerfile`:

```dockerfile
FROM python:3.12-slim
WORKDIR /app
COPY requirements.txt .
RUN pip install --no-cache-dir -r requirements.txt
COPY src/ src/
CMD ["python", "-m", "src.worker"]
```

- [ ] **Step 2: docker-compose 添加 ai-worker 服务**

在 `docker-compose.dev.yml` 中添加：

```yaml
  ai-worker:
    build: ./ai-worker
    container_name: forge-ai-worker
    env_file: ./ai-worker/.env
    depends_on:
      - temporal
    restart: unless-stopped
```

- [ ] **Step 3: 更新 .env.example**

根级 `.env.example` 添加 AI Worker 相关环境变量。

- [ ] **Step 4: 端到端验证清单**

1. `docker compose -f docker-compose.dev.yml up -d` — 所有基础设施启动
2. `cd forge-core && go build ./cmd/forge-core && ./forge-core.exe &` — Go 服务启动
3. `cd ai-worker && python -m src.worker &` — AI Worker 启动
4. `cd forge-portal && npm run dev &` — 前端启动
5. 登录获取 token
6. 创建任务 → 发送对话消息 → AI 回复澄清/确认
7. 确认计划 → Workflow 启动 → plan → generate → review
8. 查看 model_calls 表记录
9. 前端打开 http://localhost:3000/projects/1/tasks/new 验证 Chat UI
10. 查看任务详情页的代码预览

- [ ] **Step 5: Commit**

```bash
git add ai-worker/Dockerfile docker-compose.dev.yml .env.example
git commit -m "feat(s6): add Docker integration and end-to-end setup"
```

---

## 验收标准

- [ ] Python AI Worker 启动并连接 Temporal
- [ ] 多轮对话: 发送消息 → AI 澄清 → 回复 → AI 确认
- [ ] 确认后自动执行: plan → generate → review 完整流水线
- [ ] 4 级模型降级链工作（禁用主 API key 时自动 fallback）
- [ ] Review 不通过 → 自动修复重试（最多 3 轮）
- [ ] model_calls 表记录每次 AI 调用的 model/tokens/cost/latency
- [ ] 前端 Chat UI 多轮对话 + Markdown 渲染 + 确认卡片
- [ ] 任务详情页显示步骤时间线 + 代码预览
- [ ] 代码预览: 文件树导航 + 代码展示 + Review findings
- [ ] SSE 实时推送任务进度
- [ ] `go build ./cmd/forge-core` 编译通过
- [ ] `go test ./...` 所有 Go 测试通过
- [ ] `npm run build` 前端构建通过
- [ ] `python -m pytest` Python 测试通过
