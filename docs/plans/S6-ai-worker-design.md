# S6 — AI Worker 设计规格书

> **状态**: 设计完成，待实施
> **依赖**: S4 (Temporal + Tasks), S5 (Specs Center)
> **交付**: Python AI Worker + Go 对话 API + 前端 Chat UI

---

## 1. 目标

交付完整的 AI 驱动代码生成流水线：用户输入自然语言需求 → AI 多轮澄清 → 任务拆解 → 代码生成 → AI Review → 修复重试 → 完成。

**S6 交付后用户可以做什么**：
1. 在项目中新建需求任务，用自然语言描述
2. AI 通过对话澄清需求（多轮）
3. 确认后 AI 自动拆解任务、生成代码、审查修复
4. 实时查看每一步的进度和结果
5. Review 不通过时 AI 自动修复重试（最多 3 次）

---

## 2. 架构

```
┌─────────────────────────────────────────────────────────────┐
│ forge-portal (Next.js)                                       │
│  Chat UI ─ SSE ─ Task Progress                               │
└──────────────────┬──────────────────────────────────────────┘
                   │ HTTP + SSE
┌──────────────────▼──────────────────────────────────────────┐
│ forge-core (Go)                                              │
│  Conversation API ─ Task API ─ Temporal Client               │
│  ┌────────────────────────────────┐                          │
│  │ TaskGenerationWorkflow (Go)    │                          │
│  │  step 1: analyze (→ Python)    │                          │
│  │  step 2: plan    (→ Python)    │                          │
│  │  step 3: generate(→ Python)    │                          │
│  │  step 4: review  (→ Python)    │                          │
│  │  step 5: fix loop (→ Python)   │                          │
│  └────────────────────────────────┘                          │
└──────────────────┬──────────────────────────────────────────┘
                   │ Temporal (cross-language activity)
┌──────────────────▼──────────────────────────────────────────┐
│ ai-worker (Python 3.12)                                      │
│  ┌──────────┐  ┌──────────────┐  ┌───────────────────────┐  │
│  │ Model    │  │ Context      │  │ AI Agents             │  │
│  │ Router   │  │ Builder      │  │  analyst / planner    │  │
│  │ Claude   │  │ project +    │  │  coder / reviewer     │  │
│  │ → GPT    │  │ specs +      │  │                       │  │
│  │ → Qwen   │  │ prompts      │  │ Temporal Activities   │  │
│  │ → DeepS  │  │              │  │  analyze / plan /     │  │
│  └──────────┘  └──────────────┘  │  generate / review    │  │
│                                   └───────────────────────┘  │
└──────────────────────────────────────────────────────────────┘
```

---

## 3. Python AI Worker

### 3.1 项目结构

```
ai-worker/
├── pyproject.toml
├── requirements.txt
├── src/
│   ├── __init__.py
│   ├── config.py              # 配置（环境变量）
│   ├── worker.py              # Temporal Worker 入口
│   ├── models/
│   │   ├── __init__.py
│   │   ├── router.py          # 多模型路由器
│   │   └── client.py          # LLM 客户端封装
│   ├── context/
│   │   ├── __init__.py
│   │   └── builder.py         # 上下文构建器
│   ├── agents/
│   │   ├── __init__.py
│   │   ├── analyst.py         # 需求分析 Agent
│   │   ├── planner.py         # 任务规划 Agent
│   │   ├── coder.py           # 代码生成 Agent
│   │   └── reviewer.py        # 代码审查 Agent
│   └── activities/
│       ├── __init__.py
│       ├── analyze.py         # analyze_requirement_activity
│       ├── plan.py            # plan_task_activity
│       ├── generate.py        # generate_code_activity
│       └── review.py          # review_code_activity
```

### 3.2 多模型路由

```python
class Purpose(Enum):
    ANALYZE = "analyze"       # 需求分析（中等难度）
    PLAN = "plan"             # 任务拆解（中等难度）
    GENERATE = "generate"     # 代码生成（高难度）
    REVIEW = "review"         # 代码审查（高难度）

# 模型优先级链（按 purpose 不同可调整）
MODEL_CHAIN = [
    ("anthropic", "claude-sonnet-4-20250514"),   # Primary
    ("openai", "gpt-4o"),                         # Fallback 1
    ("dashscope", "qwen-max"),                    # Fallback 2
    ("deepseek", "deepseek-chat"),                # Fallback 3
]
```

**路由逻辑**：
- 按 MODEL_CHAIN 顺序尝试
- 任一模型成功即返回
- 记录每次调用的 tokens/latency/cost 到 model_calls 表
- 所有模型失败则抛出异常，Temporal 重试

### 3.3 上下文构建器

从 forge-core API 拉取项目上下文，组装成 LLM 的 system prompt：

```python
@dataclass
class ProjectContext:
    project_name: str
    project_description: str
    tech_stack: str
    coding_standards: list[str]       # /api/specs/effective/{projectId}
    prompt_template: PromptTemplate   # /api/specs/prompts?purpose=X
    conversation_history: list[dict]  # 已有对话记录

    def to_system_prompt(self) -> str:
        """组装完整 system prompt"""
```

**Token 预算**: 180k input（Claude 200k - 20k output reserve）

### 3.4 AI Agents

#### Analyst Agent
- **输入**: 用户需求 + 项目上下文 + 对话历史
- **输出**: `{"status": "clarify|confirmed", "questions": [...], "summary": "...", "task_title": "..."}`
- **行为**: 分析需求完整性，不清楚则提出澄清问题，足够清楚则确认

#### Planner Agent
- **输入**: 确认的需求摘要 + 项目上下文
- **输出**: `{"title": "...", "tasks": [{order, title, files, type}], "risk_level": "LOW|MEDIUM|HIGH"}`
- **行为**: 将需求拆解为具体的实现任务列表

#### Coder Agent
- **输入**: 任务列表 + 项目上下文 + 编码规范
- **输出**: `{"files": [{path, content, action: "create|modify"}], "commit_message": "..."}`
- **行为**: 按任务列表生成/修改代码文件

#### Reviewer Agent
- **输入**: 生成的代码 + 编码规范 + review 规则
- **输出**: `{"passed": bool, "score": 0-100, "findings": [{severity, file, line, message, suggestion}]}`
- **行为**: 审查代码的规范合规性、安全性、逻辑正确性

### 3.5 Temporal Activities

每个 Activity 是 `@activity.defn` 装饰的 async 函数：

```python
@activity.defn(name="analyze_requirement")
async def analyze_requirement_activity(input: AnalyzeInput) -> AnalyzeOutput:
    ctx = await ContextBuilder().build(input.project_id, purpose="analyze")
    result = await AnalystAgent(router).run(input.requirement, ctx, input.history)
    return AnalyzeOutput(...)
```

**Activity 超时**: 5 分钟（LLM 调用较慢）
**重试策略**: 3 次, 指数退避 (5s, 10s, 20s)

---

## 4. Go 改造

### 4.1 数据库迁移

```sql
-- conversations: 任务对话记录
CREATE TABLE engine.conversations (
    id BIGSERIAL PRIMARY KEY,
    task_id BIGINT NOT NULL REFERENCES engine.tasks(id),
    role VARCHAR(20) NOT NULL,      -- user | assistant | system
    content TEXT NOT NULL,
    metadata JSONB,                  -- agent 附加数据
    created_at TIMESTAMPTZ DEFAULT NOW()
);

-- model_calls: AI 模型调用记录（成本追踪）
CREATE TABLE engine.model_calls (
    id BIGSERIAL PRIMARY KEY,
    tenant_id BIGINT NOT NULL,
    task_id BIGINT NOT NULL,
    step_type VARCHAR(20),
    model VARCHAR(50) NOT NULL,
    provider VARCHAR(20) NOT NULL,
    purpose VARCHAR(20) NOT NULL,
    input_tokens INT,
    output_tokens INT,
    cost_cents INT,
    latency_ms INT,
    status VARCHAR(20) NOT NULL,
    created_at TIMESTAMPTZ DEFAULT NOW()
);
```

### 4.2 对话 API

```
POST /api/projects/:id/tasks/:taskId/messages     # 发送消息（同步）
GET  /api/projects/:id/tasks/:taskId/messages      # 获取对话历史
POST /api/projects/:id/tasks/:taskId/confirm       # 确认计划，启动流水线
```

### 4.3 真实 Workflow（替换骨架）

```go
func TaskGenerationWorkflow(ctx workflow.Context, input TaskWorkflowInput) error {
    // Step 1: Plan
    updateStatus(ctx, taskID, "PLANNING")
    planOutput := executeAIActivity(ctx, "plan_task_activity", planInput)

    // Step 2: Generate
    updateStatus(ctx, taskID, "GENERATING")
    codeOutput := executeAIActivity(ctx, "generate_code_activity", genInput)

    // Step 3: Review (retry loop, max 3)
    for attempt := 1; attempt <= 3; attempt++ {
        updateStatus(ctx, taskID, "REVIEWING")
        reviewOutput := executeAIActivity(ctx, "review_code_activity", reviewInput)
        if reviewOutput.Passed { break }
        // Fix: regenerate with review feedback
        codeOutput = executeAIActivity(ctx, "generate_code_activity", fixInput)
    }

    // Step 4: Complete (S7 will add GitHub commit)
    updateStatus(ctx, taskID, "COMPLETED")
    return nil
}
```

**Activity Options**:
- AI activities: task queue = "ai-worker", timeout = 5min
- Local activities: task queue = default, timeout = 10s

---

## 5. 前端 Chat UI

### 5.1 用户流程

1. 项目详情页 → 点击"新建需求" → 创建 task (SUBMITTED)
2. 进入对话页面，输入需求描述
3. `POST /messages` → AI 分析，返回澄清问题或确认
4. 多轮对话直到 AI 确认需求
5. 显示确认卡片（需求摘要 + 任务拆解预览）
6. 点击"确认并执行" → `POST /confirm` → Workflow 启动
7. 跳转到任务详情页，实时查看进度（复用 S4 SSE）

### 5.2 组件

```
forge-portal/
├── components/
│   └── chat/
│       ├── chat-panel.tsx          # 对话面板主组件
│       ├── message-bubble.tsx      # 消息气泡（用户/AI）
│       └── confirmation-card.tsx   # 确认卡片（需求摘要 + 执行按钮）
├── lib/
│   └── conversation.ts            # 对话 API 客户端
└── app/(dashboard)/projects/[id]/
    └── tasks/
        └── new/
            └── page.tsx            # 新建需求对话页
```

### 5.3 消息气泡样式

- 用户消息：右对齐，紫色背景 (#8B5CF6/10)
- AI 消息：左对齐，深灰背景 (white/5)，Markdown 渲染（复用 MarkdownPreview）
- 系统消息：居中，小字灰色
- 确认卡片：全宽，带边框，包含需求摘要 + "确认执行"按钮

---

## 6. Phase 1 简化

| 特性 | Phase 1 (S6) | 后续 |
|------|-------------|------|
| Token 流式 | Activity 返回完整结果，非逐 token 流式 | S7+ 可加 |
| 文件生成 | 串行生成 | S7 并行 |
| 外部 Linter | 仅 AI Review | S7 constraint-worker |
| GitHub 提交 | 不提交，仅生成代码 | S7 devops-worker |
| 成本控制 | 记录 model_calls，无硬限 | Phase 2 |
| 风险评估 | 简单规则 (HIGH/MEDIUM/LOW) | Phase 2 AI 评估 |

---

## 7. 环境变量

```env
# AI Worker
ANTHROPIC_API_KEY=sk-ant-...
OPENAI_API_KEY=sk-...
DASHSCOPE_API_KEY=sk-...
DEEPSEEK_API_KEY=sk-...
TEMPORAL_HOST=localhost:7233
FORGE_API_URL=http://localhost:8080
FORGE_API_TOKEN=<internal service token>
```

---

## 8. 验收标准

- [ ] Python AI Worker 启动并连接 Temporal
- [ ] 创建任务 → 对话 → AI 澄清 → 确认 → 自动生成代码
- [ ] 4 级模型降级链工作（任一模型不可用时自动切换）
- [ ] Review 不通过 → 自动修复重试（最多 3 次）
- [ ] model_calls 表记录每次 AI 调用的 token/cost
- [ ] 前端 Chat UI 多轮对话 + 确认卡片 + 进度展示
- [ ] docker-compose 一键启动完整环境
